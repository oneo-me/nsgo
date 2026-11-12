package vt

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/base/terminal/internal/library"
)

// loadLib loads the terminal's libghostty-vt, or the one $NSGO_GHOSTTY_VT
// names.
func loadLib(t *testing.T) {
	t.Helper()
	if !Supported {
		t.Skip("no libghostty-vt here")
	}
	path, err := library.Find()
	if err != nil {
		t.Fatal(err)
	}
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
}

func rowText(r *RenderState) []string {
	var rows []string
	r.Rows()
	for r.NextRow() {
		var b strings.Builder
		for _, c := range r.RowCells() {
			if c.Codepoint == 0 {
				b.WriteByte(' ')
			} else if c.Wide != SpacerTail {
				b.WriteRune(c.Codepoint)
			}
		}
		rows = append(rows, strings.TrimRight(b.String(), " "))
	}
	return rows
}

func TestTerminal(t *testing.T) {
	loadLib(t)
	var replies []string
	var titled bool
	term, err := NewTerminal(20, 5, Effects{
		WritePTY:     func(p []byte) { replies = append(replies, string(p)) },
		TitleChanged: func() { titled = true },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Free()
	term.Write([]byte("Hello, \x1b[1;32mworld\x1b[0m!\r\nsecond line\x1b]2;the title\x07\x1b[6n"))
	if want := []string{"\x1b[2;12R"}; strings.Join(replies, "") != want[0] {
		t.Errorf("replies = %q, want %q", replies, want)
	}
	if !titled || term.Title() != "the title" {
		t.Errorf("title = %q (changed %v)", term.Title(), titled)
	}
	r, err := NewRenderState()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Free()
	if err := r.Update(term); err != nil {
		t.Fatal(err)
	}
	if cols, rows := r.Size(); cols != 20 || rows != 5 {
		t.Errorf("size = %d×%d", cols, rows)
	}
	if got := rowText(r); got[0] != "Hello, world!" || got[1] != "second line" {
		t.Errorf("rows = %q", got)
	}
	r.Rows()
	r.NextRow()
	cells := r.RowCells()
	if cells[7].Style == 0 {
		t.Fatal("w has the default style")
	}
	s := r.CellStyle(7)
	if !s.Bold || s.FG != (Color{Kind: 1, Index: 2}) {
		t.Errorf("style of w = %+v", s)
	}
	if got := term.Text(); got != "Hello, world!\nsecond line" {
		t.Errorf("text = %q", got)
	}
	if c := r.Cursor(); !c.InView || c.X != 11 || c.Y != 1 {
		t.Errorf("cursor = %+v", c)
	}
}

func TestWideAndGraphemes(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(10, 2, Effects{})
	defer term.Free()
	term.Write([]byte("\x1b[?2027h世👍🏽"))
	r, _ := NewRenderState()
	defer r.Free()
	r.Update(term)
	r.Rows()
	r.NextRow()
	cells := r.RowCells()
	if cells[0].Codepoint != '世' || cells[0].Wide != WideChar || cells[1].Wide != SpacerTail {
		t.Errorf("cells = %+v", cells[:2])
	}
	if !cells[2].Grapheme {
		t.Fatalf("cell 2 = %+v", cells[2])
	}
	if got := string(r.CellGraphemes(2, nil)); got != "👍🏽" {
		t.Errorf("graphemes = %q", got)
	}
}

// The rows of a shell's prompt (OSC 133): where it starts, and those its
// input wraps onto, which the terminal and a render state tell alike.
func TestRowPrompt(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(10, 5, Effects{})
	defer term.Free()
	term.Write([]byte("out\r\n\x1b]133;A\a$ \x1b]133;B\aecho hello world"))
	want := []struct {
		prompt RowPrompt
		wrap   bool
	}{{NoPrompt, false}, {PromptStart, true}, {PromptContinuation, false}, {NoPrompt, false}}
	r, _ := NewRenderState()
	defer r.Free()
	r.Update(term)
	r.Rows()
	for y, w := range want {
		r.NextRow()
		if p, wrap := term.Row(y); p != w.prompt || wrap != w.wrap {
			t.Errorf("row %d is %d, wrapping %v; want %d, %v", y, p, wrap, w.prompt, w.wrap)
		}
		if p := r.RowPrompt(); p != w.prompt {
			t.Errorf("render state's row %d is %d, want %d", y, p, w.prompt)
		}
	}
	if p, _ := term.Row(99); p != NoPrompt {
		t.Errorf("row 99 is %d", p)
	}
}

func TestEncoders(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(80, 24, Effects{})
	defer term.Free()
	k, err := NewKeyEncoder()
	if err != nil {
		t.Fatal(err)
	}
	defer k.Free()
	k.Sync(term, false)
	for _, c := range []struct {
		ev   KeyEvent
		want string
	}{
		{KeyEvent{Action: KeyPress, Key: KeyEnter}, "\r"},
		{KeyEvent{Action: KeyPress, Key: KeyC, Mods: ModCtrl, Unshifted: 'c'}, "\x03"},
		{KeyEvent{Action: KeyPress, Key: KeyArrowUp}, "\x1b[A"},
		{KeyEvent{Action: KeyPress, Key: KeyA, Text: "a", Unshifted: 'a'}, "a"},
		{KeyEvent{Action: KeyPress, Key: KeyA, Mods: ModShift, Consumed: ModShift, Text: "A", Unshifted: 'a'}, "A"},
		{KeyEvent{Action: KeyRelease, Key: KeyA, Unshifted: 'a'}, ""},
		{KeyEvent{Action: KeyPress, Key: KeyEscape}, "\x1b"},
		{KeyEvent{Action: KeyPress, Key: KeyF1}, "\x1bOP"},
	} {
		if got := string(k.Encode(c.ev)); got != c.want {
			t.Errorf("Encode(%+v) = %q, want %q", c.ev, got, c.want)
		}
	}
	term.Write([]byte("\x1b[?1h"))
	k.Sync(term, false)
	if got := string(k.Encode(KeyEvent{Action: KeyPress, Key: KeyArrowUp})); got != "\x1bOA" {
		t.Errorf("up in cursor key mode = %q", got)
	}

	m, err := NewMouseEncoder()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Free()
	m.Sync(term, 800, 480, 10, 20, 0, 0, false)
	if got := m.Encode(MousePress, ButtonLeft, 0, 5, 5); got != nil {
		t.Errorf("press without tracking = %q", got)
	}
	term.Write([]byte("\x1b[?1000h\x1b[?1006h"))
	m.Sync(term, 800, 480, 10, 20, 0, 0, false)
	if got := string(m.Encode(MousePress, ButtonLeft, 0, 25, 45)); got != "\x1b[<0;3;3M" {
		t.Errorf("press = %q", got)
	}
	if got := string(m.Encode(MouseRelease, ButtonLeft, 0, 25, 45)); got != "\x1b[<0;3;3m" {
		t.Errorf("release = %q", got)
	}

	if got := string(EncodeFocus(true)); got != "\x1b[I" {
		t.Errorf("focus = %q", got)
	}
	if got := string(EncodePaste("a\nb", false)); got != "a\rb" {
		t.Errorf("paste = %q", got)
	}
	if got := string(EncodePaste("a\nb", true)); got != "\x1b[200~a\nb\x1b[201~" {
		t.Errorf("bracketed paste = %q", got)
	}
	if PasteIsSafe("rm -rf /\n") || !PasteIsSafe("ls") {
		t.Error("PasteIsSafe")
	}
}

func TestSelection(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(20, 3, Effects{})
	defer term.Free()
	term.Write([]byte("hello world\r\nsecond"))
	g, err := NewGesture(500e6, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Free(term)
	geo := Geometry{Columns: 20, CellWidth: 10, Height: 60}
	a, _ := term.CellAt(0, 0)
	g.Press(term, a, 2, 5, 1)
	b, _ := term.CellAt(4, 0)
	s, ok := g.Drag(term, b, 46, 5, geo, false)
	if !ok {
		t.Fatal("no selection")
	}
	term.SetSelection(&s)
	g.Release(term, &b)
	if text, ok := term.SelectionText(); !ok || text != "hello" {
		t.Errorf("selection = %q, %v", text, ok)
	}
	// A double click selects the word.
	c, _ := term.CellAt(7, 0)
	g.Press(term, c, 72, 5, 600e6)
	g.Release(term, &c)
	s, ok = g.Press(term, c, 72, 5, 700e6)
	if !ok {
		t.Fatal("double click selected nothing")
	}
	term.SetSelection(&s)
	if text, _ := term.SelectionText(); text != "world" {
		t.Errorf("word = %q", text)
	}
	all, _ := term.SelectAll()
	term.SetSelection(&all)
	if text, _ := term.SelectionText(); text != "hello world\nsecond" {
		t.Errorf("all = %q", text)
	}
	term.SetSelection(nil)
	if _, ok := term.SelectionText(); ok {
		t.Error("selection after clearing it")
	}
}

func TestScrollback(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(10, 3, Effects{})
	defer term.Free()
	for i := range 10 {
		term.Write([]byte(strings.Repeat(string(rune('0'+i)), 3) + "\r\n"))
	}
	sb := term.Scrollbar()
	if sb.Total != 11 || sb.Len != 3 || sb.Offset != 8 || !term.AtBottom() {
		t.Errorf("scrollbar = %+v", sb)
	}
	term.ScrollBy(-2)
	if term.AtBottom() || term.Scrollbar().Offset != 6 {
		t.Errorf("after scrolling up: %+v", term.Scrollbar())
	}
	r, _ := NewRenderState()
	defer r.Free()
	r.Update(term)
	if got := rowText(r); got[0] != "666" {
		t.Errorf("rows = %q", got)
	}
	term.ScrollToBottom()
	if !term.AtBottom() {
		t.Error("not at the bottom")
	}
	term.Resize(5, 3, 10, 20)
	if cols, rows := term.Size(); cols != 5 || rows != 3 {
		t.Errorf("size = %d×%d", cols, rows)
	}
}

// The keyboard's modes a Mirror passes on: modifyOtherKeys follows CSI > 4
// alone, not the pointer shape (OSC 22), and the Kitty keyboard flags
// their own sequences.
func TestKeyboardModes(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(20, 3, Effects{})
	defer term.Free()
	if term.ModifyOtherKeys() {
		t.Error("modifyOtherKeys on in a new terminal")
	}
	term.Write([]byte("\x1b]22;pointer\x1b\\"))
	if term.MouseShape() != 3 || term.ModifyOtherKeys() {
		t.Errorf("after OSC 22: shape %d, modifyOtherKeys %v", term.MouseShape(), term.ModifyOtherKeys())
	}
	term.Write([]byte("text\x1b[>4;2m"))
	if !term.ModifyOtherKeys() {
		t.Error("modifyOtherKeys off after CSI > 4 ; 2 m")
	}
	term.Write([]byte("\x1b[>4;1m"))
	if term.ModifyOtherKeys() {
		t.Error("modifyOtherKeys on after CSI > 4 ; 1 m")
	}
	term.Write([]byte("\x1b[>5u"))
	if f := term.KittyKeyboardFlags(); f != 5 {
		t.Errorf("kitty keyboard flags %d, want 5", f)
	}
	// A sequence split across writes sets it, with the Kitty protocol on
	// as well; a reset of the key formats, and of the terminal, clears it.
	term.Write([]byte("\x1b[>4"))
	term.Write([]byte(";2m"))
	if !term.ModifyOtherKeys() {
		t.Error("modifyOtherKeys off after a split CSI > 4 ; 2 m")
	}
	for _, reset := range []string{"\x1b[>m", "\x1b[>4n", "\x1bc"} {
		term.Write([]byte("\x1b[>4;2m" + reset))
		if term.ModifyOtherKeys() {
			t.Errorf("modifyOtherKeys on after %q", reset)
		}
	}
	term.Write([]byte("\x1b[>4;2m"))
	term.Reset()
	if term.ModifyOtherKeys() {
		t.Error("modifyOtherKeys on after Reset")
	}
}

// A search finds text in the scrollback and on the screen, ignoring the
// case of ASCII letters; selecting goes from the newest match to older
// ones, scrolling to them, and the viewport's matches are where it shows.
func TestSearch(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(20, 3, Effects{})
	defer term.Free()
	for i := range 10 {
		line := "line"
		if i%3 == 0 {
			line = "an Error here"
		}
		term.Write([]byte(line + "\r\n"))
	}
	s, err := NewSearch(term)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Free()
	run := func() {
		for range 1000 {
			switch s.Tick() {
			case SearchComplete:
				return
			case SearchFeedRequired:
				s.Feed()
			}
		}
		t.Fatal("the search never completed")
	}
	s.SetText("error")
	run()
	if n := s.Total(); n != 4 {
		t.Fatalf("total = %d, want 4", n)
	}
	if _, ok := s.Selected(); ok {
		t.Error("a match is selected before any was")
	}
	text := func() string {
		m, ok := s.SelectedMatch()
		if !ok {
			return ""
		}
		term.SetSelection(&m)
		got, _ := term.SelectionText()
		term.SetSelection(nil)
		return got
	}
	if !s.Select(false) {
		t.Fatal("nothing to select")
	}
	if i, _ := s.Selected(); i != 0 || text() != "Error" {
		t.Errorf("selected %d %q", i, text())
	}
	// The next, row 6, is above the screen: selecting it scrolls nothing,
	// and its row is where to scroll.
	s.Select(false)
	m, _ := s.SelectedMatch()
	a, _ := m.Ends()
	row, ok := term.ScreenRow(a)
	if i, _ := s.Selected(); i != 1 || !term.AtBottom() || !ok || row != 6 {
		t.Errorf("selected %d at row %d (%v), at the bottom: %v", i, row, ok, term.AtBottom())
	}
	if _, _, shown := term.ViewportPoint(a); shown {
		t.Error("a match above the viewport shows")
	}
	term.ScrollToRow(row)
	s.Feed()
	var shown int
	for _, m := range s.ViewportMatches(nil) {
		a, b := m.Ends()
		x0, y0, ok0 := term.ViewportPoint(a)
		x1, y1, ok1 := term.ViewportPoint(b)
		if ok0 && ok1 && y0 < 3 {
			shown++
			if x0 != 3 || x1 != 7 || y0 != y1 {
				t.Errorf("match at %d,%d–%d,%d", x0, y0, x1, y1)
			}
		}
	}
	if shown != 1 {
		t.Errorf("%d matches show, want 1", shown)
	}
	s.Select(true)
	s.Select(true)
	if i, _ := s.Selected(); i != 3 {
		t.Errorf("selected %d after wrapping back, want 3", i)
	}
	s.SetText("")
	run()
	if s.Total() != 0 {
		t.Errorf("total = %d with no text", s.Total())
	}
}

// A paste with mode 5522 set tells the program what the clipboard holds;
// the program's read with the event's password is granted and served in
// chunks; a terminal that does not answer a read stays silent.
func TestPasteEvent(t *testing.T) {
	loadLib(t)
	png := bytes.Repeat([]byte("\x89PNG\r\n\x1a\n0123456789"), 500) // over two chunks
	var out bytes.Buffer
	term, err := NewTerminal(80, 24, Effects{
		WritePTY: func(p []byte) { out.Write(p) },
		ClipboardRead: func(req ClipboardRead) (ClipboardReply, ClipboardAnswer) {
			if !req.Granted {
				return ClipboardReply{}, ClipboardRefused
			}
			return ClipboardReply{Contents: map[string][]byte{"image/png": png, "text/plain": []byte("hi")}}, ClipboardAnswered
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Free()
	if term.PasteEvent([]string{"text/plain", "image/png"}) {
		t.Fatal("an event without the mode")
	}
	term.Write([]byte("\x1b[?5522h"))
	if !term.PasteEvent([]string{"text/plain", "image/png"}) {
		t.Fatal("no event with the mode")
	}
	event := out.String()
	out.Reset()
	// ESC]5522;type=read:status=OK:pw=<b64>ESC\ ... DATA ... DONE
	_, after, ok := strings.Cut(event, ":pw=")
	if !ok || !strings.Contains(event, "status=DATA:mime=Lg==") || !strings.HasSuffix(event, "status=DONE:pw="+after[:strings.Index(after, "\x1b")]+"\x1b\\") {
		t.Fatalf("event = %q", event)
	}
	pw := after[:strings.Index(after, "\x1b")]
	listing, _ := base64.StdEncoding.DecodeString(event[strings.Index(event, "Lg==:pw="+pw+";")+len("Lg==:pw="+pw+";") : strings.Index(event, "\x1b\\\x1b]5522;type=read:status=DONE")])
	if string(listing) != "text/plain image/png\n" {
		t.Errorf("listing = %q", listing)
	}

	// The program reads the PNG with the password.
	name := base64.StdEncoding.EncodeToString([]byte("Paste event"))
	mime := base64.StdEncoding.EncodeToString([]byte("image/png"))
	term.Write([]byte("\x1b]5522;type=read:pw=" + pw + ":name=" + name + ";" + mime + "\x1b\\"))
	reply := out.String()
	out.Reset()
	if !strings.HasPrefix(reply, "\x1b]5522;type=read:status=OK\x1b\\") || !strings.HasSuffix(reply, "\x1b]5522;type=read:status=DONE\x1b\\") {
		t.Fatalf("reply = %.80q…", reply)
	}
	var got []byte
	for _, pkt := range strings.Split(reply, "\x1b\\") {
		if _, data, ok := strings.Cut(pkt, "status=DATA:mime="+mime+";"); ok {
			b, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, b...)
		}
	}
	if !bytes.Equal(got, png) {
		t.Errorf("read %d bytes, want %d", len(got), len(png))
	}

	// The password is used up: the same read again is not granted, and
	// unanswered, which leaves the program nothing at all.
	term.Write([]byte("\x1b]5522;type=read:pw=" + pw + ":name=" + name + ";" + mime + "\x1b\\"))
	if out.Len() != 0 {
		t.Errorf("an unanswered read wrote %q", out.String())
	}
	// A later query is answered as usual.
	term.Write([]byte("\x1b[6n"))
	if out.String() != "\x1b[1;1R" {
		t.Errorf("after silence, the cursor report = %q", out.String())
	}
	// A read that writes no refusal (OSC 52's) does not silence the next.
	term.Write([]byte("\x1b]52;c;?\x1b\\"))
	if term.silent {
		t.Error("an OSC 52 read left the terminal silent")
	}
}

// A deferred read (ClipboardDeferred) holds the library's refusal back,
// and AnswerClipboard answers it, allowed or denied, with the selector,
// terminator and refusal the library wrote.
func TestDeferredClipboardRead(t *testing.T) {
	loadLib(t)
	var out bytes.Buffer
	var deferred *DeferredClipboardRead
	term, err := NewTerminal(80, 24, Effects{
		WritePTY: func(p []byte) { out.Write(p) },
		ClipboardRead: func(req ClipboardRead) (ClipboardReply, ClipboardAnswer) {
			deferred = &DeferredClipboardRead{}
			return ClipboardReply{Deferred: deferred}, ClipboardDeferred
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Free()
	text := map[string][]byte{"text/plain": []byte("hello")}
	for _, c := range []struct {
		name     string
		read     string
		allow    bool
		contents map[string][]byte
		want     string
	}{
		{"allow c", "\x1b]52;c;?\x1b\\", true, text, "\x1b]52;c;aGVsbG8=\x1b\\"},
		{"deny c", "\x1b]52;c;?\x1b\\", false, nil, "\x1b]52;c;\x1b\\"},
		{"deny s bel", "\x1b]52;s;?\x07", false, nil, "\x1b]52;s;\x07"},
		{"allow p", "\x1b]52;p;?\x1b\\", true, text, "\x1b]52;p;aGVsbG8=\x1b\\"},
		{"allow empty", "\x1b]52;c;?\x1b\\", true, nil, "\x1b]52;c;\x1b\\"},
	} {
		out.Reset()
		deferred = nil
		term.Write([]byte(c.read))
		if out.Len() != 0 {
			t.Fatalf("%s: a deferred read wrote %q before it was answered", c.name, out.String())
		}
		if deferred == nil {
			t.Fatalf("%s: the read was not deferred", c.name)
		}
		term.AnswerClipboard(deferred, c.allow, c.contents, nil)
		if got := out.String(); got != c.want {
			t.Errorf("%s: answered %q, want %q", c.name, got, c.want)
		}
	}
	// A nil handle answers nothing.
	out.Reset()
	term.AnswerClipboard(nil, true, text, nil)
	if out.Len() != 0 {
		t.Errorf("a nil deferred read wrote %q", out.String())
	}
}

// A deferred Kitty protocol read is answered exactly as the library would
// answer a reply: compare a read answered synchronously with the same read
// deferred and answered through AnswerClipboard, for an id, a listing,
// chunks, and a denial.
func TestDeferredClipboardReadKitty(t *testing.T) {
	loadLib(t)
	png := bytes.Repeat([]byte("\x89PNG\r\n\x1a\n0123456789"), 300) // over one 4096-byte chunk
	contents := map[string][]byte{"image/png": png, "text/plain": []byte("hi")}
	available := []string{"text/plain", "image/png"}
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

	want := func(t *testing.T, read string, req func(ClipboardRead) (ClipboardReply, ClipboardAnswer)) string {
		t.Helper()
		var out bytes.Buffer
		term, err := NewTerminal(80, 24, Effects{
			WritePTY:      func(p []byte) { out.Write(p) },
			ClipboardRead: req,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer term.Free()
		term.Write([]byte(read))
		return out.String()
	}
	for _, c := range []struct {
		name  string
		read  string
		reply ClipboardReply
		deny  bool
	}{
		{"plain", "\x1b]5522;type=read;" + b64("text/plain") + "\x1b\\", ClipboardReply{Contents: contents}, false},
		{"id", "\x1b]5522;type=read:id=abc.123;" + b64("text/plain") + "\x1b\\", ClipboardReply{Contents: contents}, false},
		{"listing", "\x1b]5522;type=read:id=9;" + b64("text/plain image/png .") + "\x1b\\", ClipboardReply{Contents: contents, Available: available}, false},
		{"listing only", "\x1b]5522;type=read;Lg==\x1b\\", ClipboardReply{Available: available}, false},
		{"primary", "\x1b]5522;type=read:loc=primary:id=x;dGV4dC9wbGFpbg==\x1b\\", ClipboardReply{Contents: contents}, false},
		{"bel", "\x1b]5522;type=read:id=7;" + b64("text/plain") + "\x07", ClipboardReply{Contents: contents}, false},
		{"deny", "\x1b]5522;type=read:id=d;" + b64("text/plain") + "\x1b\\", ClipboardReply{}, true},
	} {
		sync := want(t, c.read, func(req ClipboardRead) (ClipboardReply, ClipboardAnswer) {
			return c.reply, ClipboardAnswered
		})
		var deferred *DeferredClipboardRead
		out := &bytes.Buffer{}
		term, err := NewTerminal(80, 24, Effects{
			WritePTY: func(p []byte) { out.Write(p) },
			ClipboardRead: func(req ClipboardRead) (ClipboardReply, ClipboardAnswer) {
				deferred = &DeferredClipboardRead{}
				return ClipboardReply{Deferred: deferred}, ClipboardDeferred
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		term.Write([]byte(c.read))
		if out.Len() != 0 {
			t.Fatalf("%s: a deferred read wrote %q before it was answered", c.name, out.String())
		}
		term.AnswerClipboard(deferred, !c.deny, c.reply.Contents, c.reply.Available)
		term.Free()
		if got := out.String(); got != sync {
			t.Errorf("%s: deferred %q, synchronous %q", c.name, got, sync)
		}
		if sync == "" {
			t.Errorf("%s: the library answered nothing", c.name)
		}
	}

	// A listing the clipboard has nothing for is still sent, as the
	// library's own encoder writes it.
	var listing *DeferredClipboardRead
	out := &bytes.Buffer{}
	term, err := NewTerminal(80, 24, Effects{
		WritePTY: func(p []byte) { out.Write(p) },
		ClipboardRead: func(req ClipboardRead) (ClipboardReply, ClipboardAnswer) {
			listing = &DeferredClipboardRead{}
			return ClipboardReply{Deferred: listing}, ClipboardDeferred
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	term.Write([]byte("\x1b]5522;type=read;Lg==\x1b\\"))
	term.AnswerClipboard(listing, true, nil, nil)
	term.Free()
	wantEmpty := "\x1b]5522;type=read:status=OK\x1b\\" +
		"\x1b]5522;type=read:status=DATA:mime=Lg==\x1b\\" +
		"\x1b]5522;type=read:status=DONE\x1b\\"
	if got := out.String(); got != wantEmpty {
		t.Errorf("an empty listing: %q, want %q", got, wantEmpty)
	}
}
