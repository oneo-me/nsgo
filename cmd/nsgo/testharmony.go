package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/cmd/nsgo/harmony"
)

// harmonyTestRunner builds the tests as the HAP's module and starts it on
// the device hdc reaches, reading what the tests print from hilog: the
// module's console reaches it through the page's console.
func harmonyTestRunner(ctx context.Context, p *project, dir string, opts testOptions) (targetRunner, error) {
	hdc, err := exec.LookPath("hdc")
	if err != nil {
		return targetRunner{}, fmt.Errorf("the harmonyos target needs hdc to install the HAP")
	}
	site := filepath.Join(dir, "web")
	if err := buildHarmony(ctx, p, site); err != nil {
		return targetRunner{}, err
	}
	device := []string(nil)
	if opts.device != "" {
		device = []string{"-t", opts.device}
	}
	return targetRunner{
		run: func(ctx context.Context, args, env []string, out io.Writer) (int, error) {
			cfg, err := json.Marshal(struct {
				Args []string          `json:"args"`
				Env  map[string]string `json:"env"`
			}{args, envValues(env)})
			if err != nil {
				return -1, err
			}
			if err := os.WriteFile(filepath.Join(site, "test.json"), cfg, 0o644); err != nil {
				return -1, err
			}
			code := p.BuildNumber
			if code == 0 {
				code = 1
			}
			hap, err := harmony.Package(dir, site, p.Name, p.ID, p.Version, code)
			if err != nil {
				return -1, err
			}
			bundle, err := harmony.BundleOf(hap)
			if err != nil {
				return -1, err
			}
			// The tests replace the HAP under the running app: stop it first, or
			// the old process keeps a page that no longer loads and aa start
			// brings that back, where the tests never run.
			hdcAt(ctx, hdc, device, "shell", "aa", "force-stop", bundle)
			if _, err := hdcAt(ctx, hdc, device, "install", "-r", hap); err != nil {
				return -1, err
			}
			hdcAt(ctx, hdc, device, "shell", "hilog", "-r")
			if _, err := hdcAt(ctx, hdc, device, "shell", "aa", "start", "-a", "EntryAbility", "-b", bundle); err != nil {
				return -1, err
			}
			exitCode, testErr := readHarmonyTest(ctx, hdc, device, out)
			// The test HAP leaves a black window in front when its binary
			// exits. Stop it so a later run of the app's own HAP is visible.
			hdcAt(ctx, hdc, device, "shell", "aa", "force-stop", bundle)
			return exitCode, testErr
		},
		close: func() {},
	}, nil
}

func hdcAt(ctx context.Context, hdc string, device []string, rest ...string) (string, error) {
	full := append(append([]string{}, device...), rest...)
	out, err := exec.CommandContext(ctx, hdc, full...).CombinedOutput()
	return string(out), err
}

func envValues(env []string) map[string]string {
	out := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		// The test host opens the app in the page, so the tests run on it;
		// the tool runs the one test it names.
		if k == "NSGO_TEST_RUN" {
			continue
		}
		out[k] = v
	}
	return out
}

// readHarmonyTest prints what the test says until it reports, and returns
// its code.
func readHarmonyTest(ctx context.Context, hdc string, device []string, out io.Writer) (int, error) {
	const (
		prefix = "NSGO harmony page: "
		// startWait is how long the page may take to print its first line: a
		// page that never starts prints nothing, and waiting forever hides
		// that as a test run that hangs.
		startWait = 2 * time.Minute
	)
	seen := map[string]bool{}
	started := time.Now()
	for {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
		if len(seen) == 0 && time.Since(started) > startWait {
			return -1, fmt.Errorf("the tests printed nothing in %s: the app's page did not start", startWait)
		}
		data, _ := hdcAt(ctx, hdc, device, "shell",
			"hilog -x | grep -F '"+prefix+"'")
		for _, line := range strings.Split(data, "\n") {
			i := strings.Index(line, prefix)
			if i < 0 {
				continue
			}
			msg := line[i+len(prefix):]
			if seen[msg] {
				continue
			}
			seen[msg] = true
			fmt.Fprintln(out, msg)
			switch {
			case strings.HasPrefix(msg, "--- PASS:"):
				return 0, nil
			case strings.HasPrefix(msg, "--- FAIL:"):
				return 1, nil
			}
		}
	}
}
