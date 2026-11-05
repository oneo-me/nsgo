package pty

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// readAll reads the output of p until the process exits and the output
// ends.
func readAll(t *testing.T, p *PTY) string {
	t.Helper()
	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&out, p)
		close(done)
	}()
	select {
	case <-p.Exited():
	case <-time.After(waitGuard):
		t.Fatal("the process did not exit")
	}
	// What the process wrote before it exited is still to read.
	time.AfterFunc(200*time.Millisecond, func() { p.Close() })
	<-done
	return out.String()
}

func TestStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	p, err := Start(Options{Path: "/bin/sh", Args: []string{"sh", "-c", "echo hello; stty size; [ -t 0 ] && echo tty; exit 3"}, Cols: 100, Rows: 30, Env: []string{"TERM=dumb"}})
	if err != nil {
		t.Fatal(err)
	}
	out := readAll(t, p)
	for _, want := range []string{"hello\r\n", "30 100\r\n", "tty\r\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
	if code, err := p.Wait(); code != 3 || err != nil {
		t.Errorf("exit = %d, %v", code, err)
	}
}

func TestResizeAndInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	p, err := Start(Options{Path: "/bin/sh", Args: []string{"sh", "-c", "read line; echo got $line; stty size"}, Env: []string{"TERM=dumb"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(120, 40, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Write([]byte("typed\r")); err != nil {
		t.Fatal(err)
	}
	out := readAll(t, p)
	for _, want := range []string{"got typed\r\n", "40 120\r\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
}

func TestClose(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	p, err := Start(Options{Path: "/bin/sh", Args: []string{"sh", "-c", "sleep 30"}})
	if err != nil {
		t.Fatal(err)
	}
	// Closing the terminal hangs up the process.
	p.Close()
	select {
	case <-p.Exited():
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("the process outlived its terminal")
	}
}

func TestConPTY(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("ConPTY")
	}
	cmd := os.Getenv("COMSPEC")
	p, err := Start(Options{Path: cmd, Args: []string{"cmd", "/c", "echo hello& exit /b 3"}, Cols: 100, Rows: 30})
	if err != nil {
		t.Fatal(err)
	}
	if out := readAll(t, p); !strings.Contains(out, "hello") {
		t.Errorf("output %q lacks hello", out)
	}
	if code, err := p.Wait(); code != 3 || err != nil {
		t.Errorf("exit = %d, %v", code, err)
	}
}

// waitGuard is how long a test waits for what should happen.
const waitGuard = time.Minute

// waitUntil fails t unless cond becomes true within waitGuard.
func waitUntil(t testing.TB, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitGuard)
	for pause := time.Millisecond; !cond(); pause = min(2*pause, 20*time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s never happened (waited %v)", what, waitGuard)
		}
		time.Sleep(pause)
	}
}
