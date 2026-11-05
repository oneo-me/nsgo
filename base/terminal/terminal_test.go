package terminal

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base/terminal/internal/vt"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// newTerminal makes a terminal of opts, skipping where none runs.
func newTerminal(t *testing.T, opts Options) *Terminal {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("runs /bin/sh")
	}
	term, err := New(opts)
	if err != nil {
		if !vt.Supported {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { term.Close() })
	return term
}

func TestViewRunsShell(t *testing.T) {
	term := newTerminal(t, Options{Command: []string{"/bin/sh"}, Env: []string{"PS1=$ "}})
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{Terminal: term, Label: "Terminal", AutoFocus: true, Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}}
	}, nsgotest.Size(640, 400))
	app.WaitUntil(5*time.Second, func() bool { return strings.Contains(term.Text(), "$") })
	if cols, rows := term.Size(); cols < 40 || rows < 10 {
		t.Errorf("the view sized the screen %d×%d", cols, rows)
	}
	app.Type("label:Terminal", "echo hel")
	app.Type("label:Terminal", "lo")
	app.Key("enter")
	app.WaitUntil(5*time.Second, func() bool { return strings.Contains(term.Text(), "\nhello") })

	// Control keys reach the program as their control characters.
	app.Type("label:Terminal", "sleep 30")
	app.Key("enter")
	time.Sleep(500 * time.Millisecond) // for sh to start sleep
	app.Settle()
	app.Key("ctrl+c")
	app.Type("label:Terminal", "echo done")
	app.Key("enter")
	app.WaitUntil(5*time.Second, func() bool { return strings.Contains(term.Text(), "\ndone") })
}

func TestFeedAndExit(t *testing.T) {
	exited := make(chan int, 1)
	term := newTerminal(t, Options{Command: []string{"/bin/sh", "-c", "exit 3"}, OnExit: func(code int) { exited <- code }})
	term.Resize(40, 10)
	select {
	case code := <-exited:
		if code != 3 {
			t.Errorf("exit code %d, want 3", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the program never exited")
	}
	<-term.Done()
	term.Feed([]byte("\r\n\x1b[1mbold\x1b[0m"))
	if text := term.Text(); !strings.Contains(text, "[Process exited with code 3]") || !strings.Contains(text, "bold") {
		t.Errorf("text %q", text)
	}
}
