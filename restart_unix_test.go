//go:build (darwin && !ios) || (linux && !android)

package nsgo

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

func TestRestartProgram(t *testing.T) {
	exe := "/Applications/App.app/Contents/MacOS/App"
	want := []string{exe, "--nsgo-lang=de", "file.txt"}
	if runtime.GOOS == "darwin" {
		want = []string{"/usr/bin/open", "-n", "/Applications/App.app", "--args", "--nsgo-lang=de", "file.txt"}
	}
	if got := restartProgram(exe, []string{"--nsgo-lang=de", "file.txt"}); !slices.Equal(got, want) {
		t.Errorf("a bundle restarts with %q, want %q", got, want)
	}
	if got := restartProgram("/usr/local/bin/app", nil); !slices.Equal(got, []string{"/usr/local/bin/app"}) {
		t.Errorf("an executable restarts with %q", got)
	}
}

func TestRestartWaitsForTheApp(t *testing.T) {
	app := exec.Command("sleep", "0.5")
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan time.Time, 1)
	go func() { _ = app.Wait(); exited <- time.Now() }()
	started := filepath.Join(t.TempDir(), "started")
	cmd := restartCommand(app.Process.Pid, []string{"/bin/sh", "-c", `touch "$0"`, started})
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(started)
	if err != nil {
		t.Fatal(err)
	}
	if end := <-exited; info.ModTime().Before(end.Add(-50 * time.Millisecond)) {
		t.Errorf("the app started again at %v, before the old one exited at %v", info.ModTime(), end)
	}
}
