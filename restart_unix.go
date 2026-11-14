//go:build (darwin && !ios) || (linux && !android)

package nsgo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

// restart starts the program again with args once this process has
// exited, through a shell that waits for it at most 10 seconds: a macOS
// app in a bundle through Launch Services, as the user opens it.
func restart(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return restartCommand(os.Getpid(), restartProgram(exe, args)).Start()
}

// restartCommand runs program once process pid has exited, or 10 seconds
// on, in a process group of its own.
func restartCommand(pid int, program []string) *exec.Cmd {
	const script = `i=0; while kill -0 "$0" 2>/dev/null && [ $i -lt 100 ]; do sleep 0.1; i=$((i+1)); done; exec "$@"`
	cmd := exec.Command("/bin/sh", append([]string{"-c", script, fmt.Sprint(pid)}, program...)...)
	cmd.Env = append(os.Environ(), restartGuardEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// restartProgram is the command line starting exe with args again.
func restartProgram(exe string, args []string) []string {
	if bundle := appBundle(exe); runtime.GOOS == "darwin" && bundle != "" {
		program := []string{"/usr/bin/open", "-n", bundle}
		if len(args) > 0 {
			program = append(append(program, "--args"), args...)
		}
		return program
	}
	return append([]string{exe}, args...)
}

// appBundle is the .app bundle whose executable exe is, or "".
func appBundle(exe string) string {
	macos := filepath.Dir(exe)
	contents := filepath.Dir(macos)
	bundle := filepath.Dir(contents)
	if filepath.Base(macos) != "MacOS" || filepath.Base(contents) != "Contents" || filepath.Ext(bundle) != ".app" {
		return ""
	}
	return bundle
}

// awaitRestarted does nothing: restart's shell waits instead.
func awaitRestarted() { markRestarted() }
