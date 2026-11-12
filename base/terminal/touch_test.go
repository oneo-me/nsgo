package terminal

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// recordingConn is a remote session: what it is given to show, and what
// the terminal sends it.
type recordingConn struct {
	*io.PipeReader
	show *io.PipeWriter
	mu   sync.Mutex
	sent bytes.Buffer
}

func newRecordingConn() *recordingConn {
	r, w := io.Pipe()
	return &recordingConn{PipeReader: r, show: w}
}

func (c *recordingConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sent.Write(p)
}

// take returns what was sent since it last did.
func (c *recordingConn) take() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sent.String()
	c.sent.Reset()
	return s
}

func (c *recordingConn) Close() error { return c.show.Close() }

var phone = core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US"}

// TestTouch checks a phone's remote session: the key bar's keys and
// latched Control, a long press selecting a word, and the grid keeping
// above the keyboard.
func TestTouch(t *testing.T) {
	conn := newRecordingConn()
	term := newTerminal(t, Options{Conn: conn})
	ref := &Ref{}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{Terminal: term, Label: "Terminal", TestID: "term", AutoFocus: true, Ref: ref,
			Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}}
	}, nsgotest.Size(390, 700), nsgotest.Environment(phone))
	go conn.show.Write([]byte(strings.Repeat("word ", 400)))
	app.WaitUntil(5*time.Second, func() bool { return strings.Contains(term.Text(), "word word") })
	_, rows := term.Size()

	sent := func(want string) {
		t.Helper()
		var got string
		app.WaitUntil(2*time.Second, func() bool { got += conn.take(); return got == want })
	}
	app.Press("id:term-key-escape")
	sent("\x1b")
	app.Press("id:term-key-up")
	sent("\x1b[A")
	app.Press("id:term-key-ctrl")
	app.Type("label:Terminal", "c")
	sent("\x03")
	app.Type("label:Terminal", "c")
	sent("c")
	app.Press("id:term-key-|")
	sent("|")
	// A software keyboard's Return, which keeps the focus, and Backspace,
	// which with nothing in the field changes nothing.
	app.Type("label:Terminal", "ls")
	app.Submit("label:Terminal")
	sent("ls\r")
	app.Find("id:term-key-escape")
	app.KeyPress("label:Terminal", "Backspace")
	sent("\x7f")

	if ref.Selected() {
		t.Fatal("selected before the long press")
	}
	app.LongPress("id:term-screen")
	app.WaitUntil(2*time.Second, ref.Selected)

	kb := phone
	kb.KeyboardHeight = 300
	app.SetEnvironment(kb)
	app.WaitUntil(2*time.Second, func() bool { _, r := term.Size(); return r < rows-10 })
	_, after := term.Size()
	t.Logf("rows %d, then %d over the keyboard", rows, after)
	app.Find("id:term-key-escape")
}

// TestComposition sends what an input method commits, once, and nothing
// it composes, as a Chinese keyboard's pinyin.
func TestComposition(t *testing.T) {
	conn := newRecordingConn()
	term := newTerminal(t, Options{Conn: conn})
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{Terminal: term, Label: "Terminal", AutoFocus: true, Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}}
	}, nsgotest.Size(390, 700), nsgotest.Environment(phone))
	app.WaitUntil(2*time.Second, func() bool { c, _ := term.Size(); return c != 80 })
	var got string
	sent := func(want string) {
		t.Helper()
		app.WaitUntil(2*time.Second, func() bool { got += conn.take(); return got == want })
	}
	app.Type("label:Terminal", "a")
	sent("a")
	app.Compose("label:Terminal", "ni")
	app.Compose("label:Terminal", "nihao")
	// Sends go on a goroutine: what composing wrongly sent would come now.
	time.Sleep(50 * time.Millisecond)
	if got += conn.take(); got != "a" {
		t.Errorf("sent %q while composing, want a", got)
	}
	app.Type("label:Terminal", "你好")
	app.Compose("label:Terminal", "")
	sent("a你好")
	app.Type("label:Terminal", "b")
	sent("a你好b")
}
