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

func TestFindURL(t *testing.T) {
	for _, c := range []struct{ text, at, want string }{
		{"  Local   http://localhost:4321/nsgo/  ", "4321", "http://localhost:4321/nsgo/"},
		{"see https://a.b/c.", "a.b", "https://a.b/c"},
		{"(see https://a.b/c_(d)), then", "c_", "https://a.b/c_(d)"},
		{"[docs](https://a.b/c)", "https", "https://a.b/c"},
		{"访问https://a.b/路径，然后", "路径", "https://a.b/路径"},
		{"│ https://a.b│c", "a.b", "https://a.b"},
		{"HTTPS://A.B and mailto:a@b.c", "a@b", "mailto:a@b.c"},
		{"http://[::1]:3000/x", "::1", "http://[::1]:3000/x"},
		{"xhttp://a.b", "a.b", ""},
		{"https://a.b/c.", "b/c.", "https://a.b/c"},
		{"nothing here", "here", ""},
	} {
		start, end, ok := findURL(c.text, strings.Index(c.text, c.at))
		if got := c.text[start:end]; ok != (c.want != "") || got != c.want {
			t.Errorf("findURL(%q, %q) = %q", c.text, c.at, got)
		}
	}
	text := "https://a.b/c."
	if _, _, ok := findURL(text, len(text)-1); ok {
		t.Error("the period after a URL is not in it")
	}
}

func TestViewOpensLinks(t *testing.T) {
	term := newTerminal(t, Options{Command: []string{"/bin/sh", "-c", "exit 0"}})
	<-term.Done()
	term.Feed([]byte("\x1b[2J\x1b[H"))
	ref := &Ref{}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{Terminal: term, Ref: ref, TestID: "terminal", Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}}
	}, nsgotest.Size(320, 240))
	app.WaitUntil(5*time.Second, func() bool { c, _ := term.Size(); return c > 0 && ref.v.cols > 0 })
	cols := ref.v.cols
	// A URL over two rows, and a program's hyperlink under them.
	url := "https://example.com/" + strings.Repeat("x", cols)
	term.Feed([]byte("\x1b[H" + url + "\r\n\x1b]8;;https://example.org\x1b\\named\x1b]8;;\x1b\\ plain\r\n"))
	app.Settle()

	v := ref.v
	at := func(col, row int) core.Point {
		return core.Point{X: core.Points(v.padX + (float32(col)+0.5)*v.cellW), Y: core.Points(float64(v.padY+(float32(row)+0.5)*v.cellH) + v.scrollY)}
	}
	var mods core.Modifiers
	mods.Control, mods.Platform = true, true
	move := func(col, row int, held core.Modifiers) {
		app.Post(func() { v.mouseMove(core.MouseEvent{Position: at(col, row), Modifiers: held}) })
		app.Settle()
	}
	click := func(col, row int, held core.Modifiers) {
		app.Post(func() {
			e := core.PressEvent{Position: at(col, row), Modifiers: held, Clicks: 1}
			v.pressDown(e)
			v.pressUp(e)
		})
		app.Settle()
	}

	// Hovering shows the link, whichever row of it, without arming it.
	move(1, 1, core.Modifiers{})
	want := []linkSpan{{0, 0, cols - 1}, {1, 0, len(url) - cols - 1}}
	if v.link.url != url || len(v.link.spans) != 2 || v.link.spans[0] != want[0] || v.link.spans[1] != want[1] {
		t.Errorf("hovered %+v", v.link)
	}
	if v.linkOn.GetUntracked() {
		t.Error("a hand without the key")
	}
	// The key alone, with no move, shows the hand.
	move(1, 1, mods)
	if !v.linkOn.GetUntracked() {
		t.Error("no hand with the key")
	}
	move(0, 2, mods)
	if v.link.url != "https://example.org" || len(v.link.spans) != 1 || v.link.spans[0] != (linkSpan{2, 0, 4}) {
		t.Errorf("hyperlink %+v", v.link)
	}
	// Past a link, nothing shows or opens.
	move(7, 2, mods)
	if v.link.url != "" || v.linkOn.GetUntracked() {
		t.Errorf("no link there: %+v", v.link)
	}

	click(1, 0, core.Modifiers{})
	if len(app.OpenedURLs) != 0 {
		t.Errorf("a plain click opened %v", app.OpenedURLs)
	}
	click(1, 0, mods)
	click(2, 2, mods)
	click(7, 2, mods)
	if got := strings.Join(app.OpenedURLs, " "); got != url+" https://example.org" {
		t.Errorf("opened %q", got)
	}
	// A press that leaves the link opens nothing.
	app.Post(func() {
		v.pressDown(core.PressEvent{Position: at(1, 0), Modifiers: mods, Clicks: 1})
		v.pressUp(core.PressEvent{Position: at(7, 2), Modifiers: mods, Clicks: 1})
	})
	app.Settle()
	if len(app.OpenedURLs) != 2 {
		t.Errorf("opened %v", app.OpenedURLs)
	}

	// Leaving the view hides the link.
	move(1, 0, core.Modifiers{})
	app.Post(func() { v.mouseLeave(core.MouseEvent{}) })
	app.Settle()
	if v.link.url != "" {
		t.Errorf("left %+v", v.link)
	}
}

func TestLinkPastViewport(t *testing.T) {
	term := newTerminal(t, Options{Command: []string{"/bin/sh", "-c", "exit 0"}})
	<-term.Done()
	term.Resize(20, 4)
	url := "https://example.com/" + strings.Repeat("x", 25)
	term.Feed([]byte("\x1b[2J\x1b[H" + url + strings.Repeat("\r\nmore", 8)))
	term.mu.Lock()
	defer term.mu.Unlock()
	// The URL's middle row at the top of the viewport.
	y, ok := 0, false
	for r := range term.term.ScrollbackRows() + 4 {
		if text, _ := term.term.ScreenRowText(r); strings.HasPrefix(string(text), "https:") {
			y, ok = r, true
			break
		}
	}
	if !ok {
		t.Fatal("the URL is not in the scrollback")
	}
	term.term.ScrollToRow(y + 1)
	l := linkAt(term.term, 3, 0)
	if l.url != url || len(l.spans) != 2 || l.spans[0] != (linkSpan{0, 0, 19}) || l.spans[1] != (linkSpan{1, 0, 4}) {
		t.Errorf("link %+v", l)
	}
}

// The scroll layer stays between rows while the user scrolls it, which a
// move would stop, and goes to the row of a viewport that moved otherwise.
func TestViewKeepsAScrollGoing(t *testing.T) {
	term := newTerminal(t, Options{Command: []string{"/bin/sh", "-c", "exit 0"}})
	<-term.Done()
	term.Feed([]byte(strings.Repeat("line\r\n", 200)))
	ref := &Ref{}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{Terminal: term, Ref: ref, TestID: "terminal", Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}}
	}, nsgotest.Size(320, 240))
	app.WaitUntil(5*time.Second, func() bool { return ref.v.cols > 0 && ref.v.content.GetUntracked() > 400 })
	app.Settle()

	v := ref.v
	viewportRow := func() int {
		term.mu.Lock()
		defer term.mu.Unlock()
		return int(term.term.Scrollbar().Offset)
	}
	cell := core.Points(v.cellH)
	applied := app.Find("id:terminal-scroll").Applied
	at := 10*cell + cell/4
	app.ScrollTo("id:terminal-scroll", core.Point{Y: at})
	app.WaitUntil(5*time.Second, func() bool { return viewportRow() == 10 })
	app.Settle()
	if layer := app.Find("id:terminal-scroll"); layer.Applied != applied || layer.Offset.Y != at {
		t.Errorf("the scroll moved: %d moves since %d, offset %v, want %v", layer.Applied, applied, layer.Offset.Y, at)
	}

	term.mu.Lock()
	term.term.ScrollToRow(20)
	term.mu.Unlock()
	term.redraw()
	app.Settle()
	if y := app.Find("id:terminal-scroll").Offset.Y; y != 20*cell {
		t.Errorf("the layer at %v, want the viewport's %v", y, 20*cell)
	}
}
