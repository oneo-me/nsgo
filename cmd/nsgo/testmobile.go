package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// iosTestRunner installs the test build in sys's simulator (-device's,
// else the booted one, else the newest iPhone or Apple TV) and launches
// it for each test, its output streamed back.
func iosTestRunner(ctx context.Context, p *project, sys *appleOS, dir string, opts testOptions) (targetRunner, error) {
	app, err := buildIOSApp(ctx, p, sys, dir, true)
	if err != nil {
		return targetRunner{}, err
	}
	udid, err := bootSimulator(ctx, sys, opts.device)
	if err != nil {
		return targetRunner{}, err
	}
	if err := run(ctx, "xcrun", "simctl", "install", udid, app); err != nil {
		return targetRunner{}, err
	}
	return targetRunner{
		run: func(ctx context.Context, args, env []string, out io.Writer) (int, error) {
			cmd := exec.CommandContext(ctx, "xcrun", append([]string{"simctl", "launch", "--console-pty",
				"--terminate-running-process", udid, p.ID}, args...)...)
			cmd.Env = os.Environ()
			for _, kv := range env {
				cmd.Env = append(cmd.Env, "SIMCTL_CHILD_"+kv)
			}
			cmd.Stdout, cmd.Stderr = out, out
			err := cmd.Run()
			if exit, ok := err.(*exec.ExitError); ok {
				return exit.ExitCode(), nil
			}
			if ctx.Err() != nil {
				exec.Command("xcrun", "simctl", "terminate", udid, p.ID).Run()
				return -1, fmt.Errorf("the test took over %v", testTimeout)
			}
			return 0, err
		},
		close: func() { exec.Command("xcrun", "simctl", "terminate", udid, p.ID).Run() },
	}, nil
}

// androidTestRunner installs the test build on the device (-device's,
// else ANDROID_SERIAL's, else the only one connected) and starts its
// activity for each test, the test's arguments and environment in the
// intent; the app's output comes back through the device's log.
func androidTestRunner(ctx context.Context, p *project, dir string, opts testOptions) (targetRunner, error) {
	d, err := adbDevice(ctx, opts.device)
	if err != nil {
		return targetRunner{}, err
	}
	goarch, err := d.goarch(ctx)
	if err != nil {
		return targetRunner{}, err
	}
	apk, err := buildAndroidAPK(ctx, p, dir, goarch)
	if err != nil {
		return targetRunner{}, err
	}
	if err := d.run(ctx, "install", "-r", apk); err != nil {
		return targetRunner{}, err
	}
	stop := func() { d.run(context.Background(), "shell", "am", "force-stop", p.ID) }
	return targetRunner{
		run: func(ctx context.Context, args, env []string, out io.Writer) (int, error) {
			stop()
			d.run(ctx, "logcat", "-c")
			token := strconv.FormatInt(time.Now().UnixNano(), 36)
			envMap := map[string]string{"NSGO_TEST_RUN": token}
			for _, kv := range env {
				k, v, _ := strings.Cut(kv, "=")
				envMap[k] = v
			}
			test, _ := json.Marshal(map[string]any{"args": args, "env": envMap})
			cmd := exec.CommandContext(ctx, d.adb, "-s", d.serial, "logcat", "-v", "raw", "-s", "nsgo:I")
			logs, err := cmd.StdoutPipe()
			if err != nil {
				return -1, err
			}
			if err := cmd.Start(); err != nil {
				return -1, err
			}
			defer func() {
				cmd.Process.Kill()
				cmd.Wait()
				stop()
			}()
			if err := d.run(ctx, "shell", "am", "start", "-S", "-n", p.ID+androidComponent,
				"--es", "nsgo.test", base64.StdEncoding.EncodeToString(test)); err != nil {
				return -1, err
			}
			sc := bufio.NewScanner(logs)
			sc.Buffer(make([]byte, 1<<16), 1<<22)
			started := false
			for sc.Scan() {
				line := sc.Text()
				switch {
				case line == "nsgo-test-start "+token:
					started = true
				case !started:
				case strings.HasPrefix(line, "nsgo-test-exit "+token+" "):
					code, _ := strconv.Atoi(strings.TrimPrefix(line, "nsgo-test-exit "+token+" "))
					return code, nil
				default:
					io.WriteString(out, line+"\n")
				}
			}
			if ctx.Err() != nil {
				return -1, fmt.Errorf("the test took over %v", testTimeout)
			}
			return -1, errors.New("the device's log ended")
		},
		close: stop,
	}, nil
}
