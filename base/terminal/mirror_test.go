package terminal

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/oneo-me/nsgo/base/terminal/internal/vt"
)

// mirrored is a terminal and another showing its Mirror's frames.
type mirrored struct {
	t      *testing.T
	src    *Terminal
	dst    *Terminal
	m      *Mirror
	frames int
	// pruned lets the other terminal keep rows the source's scrollback,
	// at its limit, let go of: its rows end with the source's.
	pruned bool
}

func newMirrored(t *testing.T, cols, rows int, o Options) *mirrored {
	t.Helper()
	o.Conn = newPipe()
	src, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	dst, err := New(Options{Conn: newPipe()})
	if err != nil {
		t.Fatal(err)
	}
	src.Resize(cols, rows)
	dst.Resize(cols, rows)
	m, err := src.NewMirror()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close(); src.Close(); dst.Close() })
	x := &mirrored{t: t, src: src, dst: dst, m: m}
	x.frame("start")
	return x
}

// feed writes output to the source, then shows the frame and checks it.
func (x *mirrored) feed(label string, out string) {
	x.t.Helper()
	x.src.Feed([]byte(out))
	x.frame(label)
}

func (x *mirrored) resize(cols, rows int) {
	x.t.Helper()
	x.src.Resize(cols, rows)
	x.dst.Resize(cols, rows)
	x.frame(fmt.Sprintf("resize %dx%d", cols, rows))
}

func (x *mirrored) frame(label string) {
	x.t.Helper()
	f, due := x.m.Frame()
	if due {
		return // a synchronized update holds it back: the next frame shows
	}
	x.dst.Feed(f)
	x.frames++
	x.same(label)
}

// same checks that the other terminal shows what the source does: every
// cell of its screen and scrollback, as the cell's text, style, colors,
// width and link, every row's wrap, the cursor and the modes.
func (x *mirrored) same(label string) {
	x.t.Helper()
	a, b := x.src, x.dst
	a.mu.Lock()
	b.mu.Lock()
	defer a.mu.Unlock()
	defer b.mu.Unlock()
	ta, tb := a.term, b.term
	fail := func(format string, args ...any) {
		x.t.Helper()
		x.t.Fatalf("%s (frame %d): %s", label, x.frames, fmt.Sprintf(format, args...))
	}
	if ta.AltScreen() != tb.AltScreen() {
		fail("alt screen %v, mirrored %v", ta.AltScreen(), tb.AltScreen())
	}
	ca, ra := ta.Size()
	if cb, rb := tb.Size(); ca != cb || ra != rb {
		fail("size %dx%d, mirrored %dx%d", ca, ra, cb, rb)
	}
	if sa, sb := ta.ScrollbackRows(), tb.ScrollbackRows(); !ta.AltScreen() && sa != sb && !(x.pruned && sb > sa) {
		fail("scrollback %d rows, mirrored %d", sa, sb)
	}
	rowsA, rowsB := cellRows(ta), cellRows(tb)
	if x.pruned && len(rowsB) > len(rowsA) {
		rowsB = rowsB[len(rowsB)-len(rowsA):]
	}
	if len(rowsA) != len(rowsB) {
		fail("%d rows, mirrored %d", len(rowsA), len(rowsB))
	}
	for y := range rowsA {
		if rowsA[y].cells != rowsB[y].cells {
			fail("row %d (screen row %d) is\n  %s\nmirrored\n  %s", y, y-(len(rowsA)-ra), rowsA[y].cells, rowsB[y].cells)
		}
		// The screen's last row wraps into nothing: the other terminal
		// cannot wrap its own without scrolling, and nothing follows to
		// join, until the screen scrolls and the mirror writes it again.
		if rowsA[y].wrap != rowsB[y].wrap && y != len(rowsA)-1 {
			fail("row %d wraps %v, mirrored %v", y, rowsA[y].wrap, rowsB[y].wrap)
		}
	}
	xa, ya := ta.Cursor()
	if xb, yb := tb.Cursor(); xa != xb || ya != yb {
		fail("cursor %d,%d, mirrored %d,%d", xa, ya, xb, yb)
	}
	for _, md := range append(mirroredModes, vt.ModeCursorVisible) {
		if ta.Mode(md) != tb.Mode(md) {
			fail("mode %d is %v, mirrored %v", md, ta.Mode(md), tb.Mode(md))
		}
	}
	if ta.KittyKeyboardFlags() != tb.KittyKeyboardFlags() {
		fail("kitty keyboard flags %d, mirrored %d", ta.KittyKeyboardFlags(), tb.KittyKeyboardFlags())
	}
	if ta.ModifyOtherKeys() != tb.ModifyOtherKeys() {
		fail("modifyOtherKeys %v, mirrored %v", ta.ModifyOtherKeys(), tb.ModifyOtherKeys())
	}
	if ta.MouseShape() != tb.MouseShape() {
		fail("pointer shape %d, mirrored %d", ta.MouseShape(), tb.MouseShape())
	}
}

// cellRow is a row as its cells show: their text, styles, colors, widths
// and links, written out, and whether it wraps.
type cellRow struct {
	cells string
	wrap  bool
}

// cellRows reads every row of the scrollback and the screen, cell by cell,
// independently of what the mirror encodes: an empty cell and a plain
// space show the same, and so do a wide character's spacer at a row's end
// and a blank.
func cellRows(term *vt.Terminal) []cellRow {
	rs, err := vt.NewRenderState()
	if err != nil {
		panic(err)
	}
	defer rs.Free()
	_, rows := term.Size()
	total := rows
	if !term.AltScreen() {
		total += term.ScrollbackRows()
	}
	var out []cellRow
	for y := 0; y < total; {
		term.ScrollToRow(y)
		rs.Update(term)
		off := int(term.Scrollbar().Offset)
		rs.Rows()
		for i := 0; rs.NextRow(); i++ {
			if off+i != len(out) || len(out) >= total {
				continue
			}
			cells := rs.RowCells()
			var b strings.Builder
			for x, c := range cells {
				text := string(c.Codepoint)
				if c.Grapheme {
					text = string(rs.CellGraphemes(x, nil))
				}
				if c.Codepoint == 0 || c.Wide == vt.SpacerHead {
					text = " "
				}
				if c.Wide == vt.SpacerTail {
					text = ""
				}
				st := ""
				if c.Style != 0 {
					st = rs.CellStyle(x).SGR()
				}
				if c.HasBackground && c.Codepoint == 0 {
					// A cell a program filled without text shows as a
					// blank in its background.
					if c.Palette {
						st = vt.Style{BG: vt.Color{Kind: 1, Index: c.Background.R}}.SGR()
					} else {
						st = vt.Style{BG: vt.Color{Kind: 2, RGB: c.Background}}.SGR()
					}
				}
				link := ""
				if c.Hyperlink {
					if ref, ok := term.CellAt(x, i); ok {
						link = term.Hyperlink(ref)
					}
				}
				wide := c.Wide
				if wide == vt.SpacerHead {
					wide = vt.Narrow
				}
				if text == " " && st == "\x1b[0m" {
					st = ""
				}
				fmt.Fprintf(&b, "[%s|%q|%d|%s]", text, st, wide, link)
			}
			out = append(out, cellRow{cells: strings.TrimRight(trimBlankCells(b.String()), ""), wrap: rs.RowWrapped()})
		}
		next := off + rows
		if next <= y {
			break
		}
		y = next
	}
	term.ScrollToBottom()
	return out
}

// trimBlankCells drops the blank cells at a row's end, which show nothing.
func trimBlankCells(s string) string {
	for strings.HasSuffix(s, `[ |""|0|]`) {
		s = strings.TrimSuffix(s, `[ |""|0|]`)
	}
	return s
}

func lines(from, to int, prefix string) string {
	var b strings.Builder
	for i := from; i < to; i++ {
		fmt.Fprintf(&b, "%s%d\r\n", prefix, i)
	}
	return b.String()
}

func TestMirrorScrolling(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 20, 6, Options{})
	x.feed("a few lines", lines(0, 3, "line "))
	x.feed("past the screen", lines(3, 9, "line "))
	x.feed("a screen and more at once", lines(9, 40, "row "))
	x.feed("one more", "last\r\n")
	x.feed("styles", "\x1b[1;31mred\x1b[0m \x1b[48;5;4mbg\x1b[0m \x1b[38;2;10;20;30mrgb\x1b[0m\r\n")
	x.feed("a link", "\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\ after\r\n")
	x.feed("wide characters", "你好世界 😄 ok\r\n")
	x.feed("a wrapped line", strings.Repeat("abcdefghij", 5)+"\r\n")
	x.feed("a wrapped line scrolling off", lines(0, 8, "after "))
	x.feed("a wide character at the edge", strings.Repeat("x", 19)+"世"+"\r\n")
	x.feed("a prompt without a newline", "$ ")
}

// The terminal showing the frames keeps the cursor its options make
// while programs leave the cursor alone, and takes one a program asks
// for, until the program resets it.
// The keyboard's modes go to the other terminal as the program sets them,
// and a pointer shape (OSC 22) does not turn modifyOtherKeys on there.
func TestMirrorKeyboardModes(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 20, 6, Options{})
	x.feed("a pointer shape", "\x1b]22;pointer\x1b\\")
	x.feed("modifyOtherKeys", "\x1b[>4;2m")
	x.feed("kitty keyboard flags", "\x1b[>1u")
	x.feed("modifyOtherKeys off", "\x1b[>4;1m\x1b[<u")
	// The modes the keys' encoding takes from the terminal: Backspace's,
	// NumLock's keypad and Alt's ESC.
	x.feed("keys' modes", "\x1b[?67h\x1b[?1035h\x1b[?1036l\x1b[?1039l")
	for _, md := range []vt.Mode{67, 1035, 1036, 1039} {
		if a, b := x.src.term.Mode(md), x.dst.term.Mode(md); a != b {
			t.Errorf("mode %d is %v, mirrored %v", md, a, b)
		}
	}
	x.feed("keys' modes back", "\x1b[?67l\x1b[?1035l\x1b[?1036h\x1b[?1039h")
}

func TestMirrorCursorStyle(t *testing.T) {
	loadLib(t)
	src, err := New(Options{Conn: newPipe()})
	if err != nil {
		t.Fatal(err)
	}
	dst, err := New(Options{Conn: newPipe(), Cursor: CursorBar})
	if err != nil {
		t.Fatal(err)
	}
	m, err := src.NewMirror()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close(); src.Close(); dst.Close() })
	step := func(label, out string, style vt.CursorStyle, blink bool) {
		t.Helper()
		src.Feed([]byte(out))
		f, _ := m.Frame()
		dst.Feed(f)
		dst.mu.Lock()
		dst.rs.Update(dst.term)
		c := dst.rs.Cursor()
		dst.mu.Unlock()
		if c.Style != style || c.Blinking != blink {
			t.Errorf("%s: the cursor is style %d, blinking %v; want %d, %v", label, c.Style, c.Blinking, style, blink)
		}
	}
	step("at the start", "$ ", vt.CursorBar, false)
	step("a program's underline", "\x1b[3 q", vt.CursorUnderline, true)
	step("a program's block", "\x1b[1 q", vt.CursorBlock, true)
	step("reset", "\x1b[0 q", vt.CursorBar, false)
}

// Empty cells after a styled run stay empty: Claude Code's mascot, whose
// eyes are a black background, showed a black square where the cells
// after it took the background (libghostty's formatter wrote them as
// spaces in the run's style).
func TestMirrorGaps(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 40, 5, Options{})
	x.feed("the mascot", "\x1b[38;2;215;119;87m \u2590\x1b[48;2;0;0;0m\u259b\u2588\u2588\u2588\u259b\u2588\x1b[12G\x1b[39m\x1b[49m\x1b[1mClaude\x1b[19GCode\x1b[22m\r\r\n")
	x.feed("a background, then text further on", "\x1b[44mblue\x1b[0m\x1b[20Gplain\r\n")
	x.feed("an erase in a background", "\x1b[41m\x1b[K\x1b[0m\x1b[10Gafter\r\n")
}

func TestMirrorEdits(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 20, 6, Options{})
	x.feed("fill", lines(0, 10, "l"))
	x.feed("overwrite a row", "\x1b[2;1Hchanged\x1b[K")
	x.feed("insert lines", "\x1b[3;1H\x1b[2L")
	x.feed("delete lines", "\x1b[2;1H\x1b[1M")
	x.feed("a scrolling region", "\x1b[2;4r\x1b[4;1H"+lines(0, 5, "in region "))
	x.feed("the region reset", "\x1b[r\x1b[6;1H")
	x.feed("clear the screen", "\x1b[2J\x1b[H")
	x.feed("after the clear", lines(0, 3, "new "))
	x.feed("clear the scrollback", "\x1b[3J")
	x.feed("after clearing the scrollback", lines(0, 9, "more "))
	x.feed("modes", "\x1b[?1h\x1b[?2004h\x1b[?1000h\x1b[?1006h\x1b[?1004h\x1b[>1u\x1b[?25l")
	x.feed("modes off", "\x1b[?1l\x1b[?2004l\x1b[?1000l\x1b[?1006l\x1b[<u\x1b[?25h")
}

func TestMirrorAltScreen(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 20, 6, Options{})
	x.feed("history", lines(0, 10, "shell "))
	x.feed("enter the alternate screen", "\x1b[?1049h\x1b[H\x1b[2Jfull screen\x1b[3;3Hthere")
	x.feed("draw on it", "\x1b[5;1H"+strings.Repeat("-", 20))
	x.feed("leave it", "\x1b[?1049l")
	x.feed("after", "$ ls\r\n")
}

// A terminal that takes a Mirror's frames from while a full-screen program
// shows gets the main screen and its scrollback once the program ends.
func TestMirrorResetInAltScreen(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 20, 5, Options{})
	x.feed("history", lines(0, 50, "x"))
	x.feed("a full-screen program", "\x1b[?1049hfull screen")
	x.m.Reset()
	other, err := New(Options{Conn: newPipe()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { other.Close() })
	other.Resize(20, 5)
	x.dst = other
	x.frame("another terminal")
	x.feed("the program ends", "\x1b[?1049l")
}

func TestMirrorResize(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 20, 6, Options{})
	x.feed("history", lines(0, 12, "line "))
	x.feed("a long line", strings.Repeat("0123456789", 4)+"\r\n$ ")
	x.resize(30, 8)
	x.feed("after growing", "more\r\n")
	x.resize(12, 5)
	x.feed("after shrinking", lines(0, 3, "s"))
}

// The server's terminal clears as a window's ⌘K asks (Options.OnClear):
// the window shows the prompt at the top, and goes on showing what the
// shell draws there, through resizes.
func TestMirrorClearScreen(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 20, 6, Options{})
	x.feed("history", lines(0, 12, "line ")+"~/dev\r\n$ ")
	if !x.src.ClearScreen() {
		t.Fatal("ClearScreen did not clear")
	}
	x.frame("cleared")
	if got := strings.TrimSpace(x.dst.Text()); got != "$" {
		t.Fatalf("the window shows %q after the clear", got)
	}
	x.feed("typing", "ls")
	x.feed("the line repainted", "\r\x1b[K$ ls -la")
	x.resize(30, 8)
	x.resize(12, 5)
	if got := strings.TrimSpace(x.dst.Text()); got != "$ ls -la" {
		t.Errorf("the window shows %q after resizes", got)
	}
	x.feed("output after", "\r\n"+lines(0, 9, "out "))
	x.src.ClearScrollback()
	x.frame("scrollback cleared")
}

func TestMirrorScrollbackLimit(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 20, 5, Options{Scrollback: 1})
	for i := range 40 {
		x.feed(fmt.Sprintf("burst %d", i), lines(i*100, i*100+60, "x"))
	}
}

// A scrollback at its limit lets go of its oldest rows as it takes new
// ones: the other terminal keeps its own, without starting over, whether
// a frame scrolls a row or more than a screen.
func TestMirrorScrollbackPruned(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 40, 5, Options{Scrollback: 64 << 10})
	x.pruned = true
	drops, n := 0, 0
	prev := 0
	for i := range 400 {
		count := 1
		if i%2 == 1 {
			count = 23 // more than a screen
		}
		x.src.Feed([]byte(lines(n, n+count, "line abcdefghijklmnopqrstuvwxyz ")))
		n += count
		f, due := x.m.Frame()
		if due {
			t.Fatal("no frame")
		}
		if strings.Contains(string(f), "\x1b[3J") {
			t.Fatalf("frame %d clears the other terminal's scrollback: %q", i, f[:min(len(f), 80)])
		}
		x.dst.Feed(f)
		x.frames++
		x.src.mu.Lock()
		sb := x.src.term.ScrollbackRows()
		x.src.mu.Unlock()
		if sb < prev {
			drops++
		}
		if sb < prev || i%25 == 0 {
			// Reading every row of both terminals takes long: at the
			// frames that let go of rows, and now and then.
			x.same(fmt.Sprintf("frame %d", i))
		}
		prev = sb
	}
	if drops < 2 {
		t.Fatalf("the scrollback let go of rows %d times, want at least 2", drops)
	}
	// A clear still clears the other terminal's scrollback.
	x.pruned = false
	x.feed("clear", "\x1b[3J")
	x.feed("after the clear", lines(0, 12, "after "))
}

// Claude Code's output under a screen of history, fed in uneven pieces, as
// it reaches a server: the frames keep the cursor where it is, on its
// input line, not a row above.
func TestMirrorClaudeCode(t *testing.T) {
	loadLib(t)
	raw, err := os.ReadFile("testdata/claude-code.vt")
	if err != nil {
		t.Skip(err)
	}
	x := newMirrored(t, 120, 36, Options{})
	x.feed("history", lines(1, 101, ""))
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; len(raw) > 0; i++ {
		n := min(len(raw), 1+r.IntN(400))
		x.feed(fmt.Sprintf("piece %d", i), string(raw[:n]))
		raw = raw[n:]
	}
}

// Thousands of random edits, of every kind the mirror follows, each frame
// checked.
func TestMirrorRandom(t *testing.T) {
	loadLib(t)
	for seed := uint64(1); seed <= 3; seed++ {
		r := rand.New(rand.NewPCG(seed, 7))
		x := newMirrored(t, 16, 6, Options{Scrollback: 2})
		words := []string{"a", "hello", "世界", "😄", "\x1b[1;32mgreen\x1b[0m", "\x1b[48;5;3m  \x1b[0m", "\x1b]8;;https://x.y\x1b\\ln\x1b]8;;\x1b\\", strings.Repeat("w", 21)}
		ops := []func() string{
			func() string { return words[r.IntN(len(words))] },
			func() string { return "\r\n" },
			func() string { return lines(0, 1+r.IntN(20), "r") },
			func() string { return fmt.Sprintf("\x1b[%d;%dH", 1+r.IntN(6), 1+r.IntN(16)) },
			func() string { return []string{"\x1b[K", "\x1b[1K", "\x1b[2K", "\x1b[J", "\x1b[1J"}[r.IntN(5)] },
			func() string { return fmt.Sprintf("\x1b[%dL", 1+r.IntN(3)) },
			func() string { return fmt.Sprintf("\x1b[%dM", 1+r.IntN(3)) },
			func() string { return fmt.Sprintf("\x1b[%dS", 1+r.IntN(3)) },
			func() string { return fmt.Sprintf("\x1b[%dT", 1+r.IntN(3)) },
			func() string { return []string{"\x1b[2;5r", "\x1b[r"}[r.IntN(2)] },
			func() string { return []string{"\x1b[?1049h", "\x1b[?1049l"}[r.IntN(2)] },
			func() string {
				return []string{"\x1b[?25l", "\x1b[?25h", "\x1b[?2004h", "\x1b[?1000h\x1b[?1006h", "\x1b[?1000l"}[r.IntN(5)]
			},
			func() string { return "\x1bM" }, // reverse index
		}
		for i := range 1000 {
			var b strings.Builder
			for range 1 + r.IntN(4) {
				b.WriteString(ops[r.IntN(len(ops))]())
			}
			x.feed(fmt.Sprintf("seed %d op %d %q", seed, i, b.String()), b.String())
		}
	}
}

// Two windows mirror one terminal: each of them shows it all, though their
// reads take turns at the terminal.
func TestMirrorTwoWindows(t *testing.T) {
	loadLib(t)
	x := newMirrored(t, 30, 6, Options{})
	other, err := New(Options{Conn: newPipe()})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	other.Resize(30, 6)
	m2, err := x.src.NewMirror()
	if err != nil {
		t.Fatal(err)
	}
	defer m2.Close()
	second := &mirrored{t: t, src: x.src, dst: other, m: m2}
	second.frame("start")
	raw, _ := os.ReadFile("testdata/claude-code.vt")
	steps := []string{lines(0, 4, "a"), "\x1b[2;1Hedit\x1b[K", lines(0, 12, "b"), "\x1b[44mbg\x1b[0m\x1b[20Gx", string(raw)}
	for i, out := range steps {
		x.feed(fmt.Sprintf("step %d, the first window", i), out)
		second.frame(fmt.Sprintf("step %d, the second window", i))
	}
}

// History is what shows, down to the cursor, with the cursor under it: a
// program's interface below the cursor, as an agent's status bar, goes.
func TestHistory(t *testing.T) {
	loadLib(t)
	src, _ := New(Options{Conn: newPipe()})
	defer src.Close()
	src.Resize(20, 6)
	src.Feed([]byte(lines(0, 10, "line ") + strings.Repeat("abcdefghij", 3) + "\r\n\x1b[1;31mred\x1b[0m\r\n> input\r\nstatus bar\x1b[1A\x1b[3G"))
	h := src.History()
	dst, _ := New(Options{Conn: newPipe()})
	defer dst.Close()
	dst.Resize(20, 6)
	dst.Feed(h)
	text := dst.Text()
	for _, want := range []string{"line 0", "line 9", strings.Repeat("abcdefghij", 3), "red", "> input"} {
		if !strings.Contains(text, want) {
			t.Errorf("history lost %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "status bar") {
		t.Errorf("history kept what was below the cursor:\n%s", text)
	}
	dst.mu.Lock()
	x, y := dst.term.Cursor()
	row, _ := dst.term.RowText(y)
	dst.mu.Unlock()
	if x != 0 || strings.TrimSpace(string(row)) != "" {
		t.Errorf("the cursor is at %d on %q, not at the start of a row of its own", x, string(row))
	}
	// Soft wraps stay soft: the long line is one line.
	if !strings.Contains(text, strings.Repeat("abcdefghij", 3)+"\n") {
		t.Errorf("the wrapped line did not stay one line:\n%s", text)
	}
	alt, _ := New(Options{Conn: newPipe()})
	defer alt.Close()
	alt.Feed([]byte("\x1b[?1049hfull screen"))
	if h := alt.History(); len(h) != 0 {
		t.Errorf("history of a full-screen program: %q", h)
	}
}

// The colors programs set (OSC 4, 10, 11, 12) show in the other terminal,
// as on the source's, and their reset gives back the other terminal's
// own: a TUI that sets the terminal's background, or a shell script its
// palette, showed none of it.
func TestMirrorColors(t *testing.T) {
	loadLib(t)
	src, err := New(Options{Conn: newPipe()})
	if err != nil {
		t.Fatal(err)
	}
	dark := DarkTheme()
	dst, err := New(Options{Conn: newPipe()})
	if err != nil {
		t.Fatal(err)
	}
	m, err := src.NewMirror()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close(); src.Close(); dst.Close() })
	src.useTheme(LightTheme())
	dst.useTheme(dark)
	step := func(label, out string, want vt.TermColors) {
		t.Helper()
		src.Feed([]byte(out))
		f, _ := m.Frame()
		dst.Feed(f)
		dst.mu.Lock()
		got := dst.term.Colors(false)
		dst.mu.Unlock()
		if got.Foreground != want.Foreground || got.Background != want.Background || got.HasCursor != want.HasCursor || got.Cursor != want.Cursor || got.Palette[1] != want.Palette[1] || got.Palette[200] != want.Palette[200] {
			t.Errorf("%s: fg %v bg %v cursor %v palette 1 %v, 200 %v; want %v %v %v %v %v", label,
				got.Foreground, got.Background, got.Cursor, got.Palette[1], got.Palette[200],
				want.Foreground, want.Background, want.Cursor, want.Palette[1], want.Palette[200])
		}
	}
	own := dst.term.Colors(true)
	step("at the start", "", own)
	set := own
	set.Foreground, set.Background, set.Cursor, set.HasCursor = vt.RGB{R: 0x10, G: 0x20, B: 0x30}, vt.RGB{R: 0xfe, G: 0xdc, B: 0xba}, vt.RGB{R: 0xff}, true
	set.Palette[1], set.Palette[200] = vt.RGB{B: 0xff}, vt.RGB{G: 0x80}
	step("set", "\x1b]10;#102030\x07\x1b]11;#fedcba\x07\x1b]12;#ff0000\x07\x1b]4;1;#0000ff;200;#008000\x07", set)
	step("reset", "\x1b]110\x07\x1b]111\x07\x1b]112\x07\x1b]104\x07", own)
}

// The other terminal sends the mouse's events in the format the terminal
// would, whatever order the program set and reset their modes in: the
// last of a setting's modes picks it, and the others stay set.
func TestMirrorMouseOrder(t *testing.T) {
	loadLib(t)
	for _, modes := range []string{
		"\x1b[?1000h\x1b[?1002h\x1b[?1003h\x1b[?1015h\x1b[?1006h",
		"\x1b[?1003h\x1b[?1000h\x1b[?1006h\x1b[?1015h",
		"\x1b[?1002h\x1b[?1003h\x1b[?1003l",
		"\x1b[?1000h\x1b[?1016h\x1b[?1006h",
		"\x1b[?1000h\x1b[?1006h\x1b[?1016h",
		"\x1b[?1000h\x1b[?1015h\x1b[?1005h",
		"\x1b[?1000h\x1b[?1005h\x1b[?1006h\x1b[?1006l",
		"\x1b[?9h\x1b[?1000h\x1b[?9h\x1b[?1006h",
	} {
		src, err := New(Options{Conn: newPipe()})
		if err != nil {
			t.Fatal(err)
		}
		dst, err := New(Options{Conn: newPipe()})
		if err != nil {
			t.Fatal(err)
		}
		m, err := src.NewMirror()
		if err != nil {
			t.Fatal(err)
		}
		src.Feed([]byte(modes))
		f, _ := m.Frame()
		dst.Feed(f)
		sends := func(term *vt.Terminal) string {
			var out []string
			for _, ev := range []struct {
				pressed bool
				action  vt.MouseAction
				button  int
				x       float32
			}{
				{false, vt.MouseMotion, vt.ButtonNone, 5},
				{false, vt.MousePress, vt.ButtonLeft, 1505},
				{true, vt.MouseMotion, vt.ButtonLeft, 15},
				{false, vt.MouseRelease, vt.ButtonLeft, 25},
			} {
				e, err := vt.NewMouseEncoder()
				if err != nil {
					t.Fatal(err)
				}
				e.Sync(term, 3000, 100, 10, 10, 0, 0, ev.pressed)
				out = append(out, string(e.Encode(ev.action, ev.button, 0, ev.x, 5)))
				e.Free()
			}
			return fmt.Sprintf("%q", out)
		}
		if got, want := sends(dst.term), sends(src.term); got != want {
			t.Errorf("%q: the other terminal sends %s, want %s", modes, got, want)
		}
		m.Close()
		src.Close()
		dst.Close()
	}
}

// pipe is a Conn that reads nothing until closed and keeps what it is sent.
type pipe struct {
	closed chan struct{}
	once   sync.Once
	mu     sync.Mutex
	sent   []byte
}

func newPipe() *pipe { return &pipe{closed: make(chan struct{})} }

func (p *pipe) Read([]byte) (int, error) {
	<-p.closed
	return 0, os.ErrClosed
}

func (p *pipe) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.sent = append(p.sent, b...)
	p.mu.Unlock()
	return len(b), nil
}

func (p *pipe) Close() error {
	p.once.Do(func() { close(p.closed) })
	return nil
}

// loadLib loads libghostty-vt, skipping the test where it cannot.
func loadLib(t *testing.T) {
	t.Helper()
	if err := Load(); err != nil {
		t.Skip(err)
	}
}
