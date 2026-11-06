package terminal

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/oneo-me/nsgo/base/terminal/internal/vt"
)

// Mirror frames what a terminal shows for another terminal to show, as a
// server that keeps a session's terminal sends its clients: Frame returns
// the VT sequences that bring a terminal showing the frames before to what
// this one shows now. The other terminal, as one whose Conn reads the
// frames, then shows this one's screen and scrollback cell for cell, with
// its cursor and the modes that change what it sends (the cursor keys',
// the keyboard protocols', the mouse's, bracketed paste), and never lays
// out a program's output itself: it cannot come to show something else,
// as a terminal fed the program's output from a snapshot can.
//
// Frames draw rows where they are, and rows that scroll off pass through
// the other terminal's screen in order, so that its scrollback is this
// one's: rows this one's limit lets go of stay there, under its own
// limit. Soft wraps stay soft. The first frame, and a frame after the
// screen took another size, start the other terminal over. A frame does
// not come while a program holds a synchronized update (mode 2026), for up
// to a second, as the view's frames do not: Frame then reports that one
// is due.
//
// Programs' writes to the clipboard (OSC 52) go on to the other terminal
// of each Mirror.
//
// Frames read the terminal as its view does, which takes the terminal's
// record of what changed since it last read it: a terminal a view shows
// too would not repaint all that changed. A Mirror is for a terminal no
// view shows, as a server's; many Mirrors of one terminal are fine.
type Mirror struct {
	t      *Terminal
	closed bool
	// started tells that a frame went; cols and rows are the size it
	// drew, alt that it showed the alternate screen.
	started    bool
	cols, rows int
	alt        bool
	// ids are the identities of the rows the last frame showed, which
	// rows keep as they scroll; sent is how each was drawn, and main the
	// main screen's while the alternate screen shows.
	ids        []vt.RowID
	sent, main [][]byte
	// scrollback is how many rows the main screen's scrollback held at the
	// last frame that showed the main screen.
	scrollback int
	// modes, colors, cursor and title are what the last frame set of them.
	modes, colors, cursor, title string
	// imgs are the images sent of the main and the alternate screens, and
	// placement the last id of a placement made.
	imgs      [2]mirroredImages
	placement uint32
	// clipOut is what programs copied since the last frame, which each
	// Mirror passes on.
	clipOut []string
}

// NewMirror returns a Mirror of the terminal, whose first frame starts the
// other terminal over. Close frees it.
func (t *Terminal) NewMirror() (*Mirror, error) {
	if !vt.Loaded() {
		return nil, errors.New("terminal: libghostty-vt is not loaded")
	}
	m := &Mirror{t: t}
	t.mu.Lock()
	t.mirrors = append(t.mirrors, m)
	t.mu.Unlock()
	return m, nil
}

// Close frees the mirror.
func (m *Mirror) Close() {
	t := m.t
	t.mu.Lock()
	defer t.mu.Unlock()
	m.closed = true
	m.clipOut = nil
	t.mirrors = slices.DeleteFunc(t.mirrors, func(o *Mirror) bool { return o == m })
}

// Reset has the next frame start the other terminal over, as when another
// terminal shows the frames from now.
func (m *Mirror) Reset() {
	m.t.mu.Lock()
	m.started = false
	m.t.mu.Unlock()
}

// Frame returns what brings the other terminal to what this one shows,
// nil when it shows it already; due reports that a frame is due later,
// as one a synchronized update holds back.
func (m *Mirror) Frame() (frame []byte, due bool) {
	t := m.t
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term == nil || m.closed {
		return nil, false
	}
	if t.term.Mode(vt.ModeSyncOutput) && t.held && time.Since(t.heldSince) < time.Second {
		return nil, true
	}
	var out bytes.Buffer
	m.frame(&out)
	return out.Bytes(), false
}

func (m *Mirror) frame(out *bytes.Buffer) {
	t := m.t
	term := t.term
	cols, rows := term.Size()
	alt := term.AltScreen()
	scrollback := term.ScrollbackRows()
	if !term.AtBottom() {
		term.ScrollToBottom()
	}
	// A render state of its own, each time, copies the terminal whole: one
	// kept would copy the rows the terminal marked changed, and the marks
	// go to whatever render state reads them first, another Mirror's.
	rs, err := vt.NewRenderState()
	if err != nil {
		return
	}
	defer rs.Free()
	rs.Update(term)
	ids := make([]vt.RowID, 0, rows)
	screen := make([]line, 0, rows)
	rs.Rows()
	for y := 0; rs.NextRow(); y++ {
		ids = append(ids, rs.RowID())
		screen = append(screen, readLine(term, rs, y))
	}
	top := scrollback // the screen's first row, from the top of the scrollback
	if alt {
		top = 0
	}
	switch {
	case !m.started || cols != m.cols || rows != m.rows:
		m.restart(out, screen, alt, top)
	case alt != m.alt && !alt && scrollback != m.scrollback:
		// Rows went to the main screen's scrollback before its program
		// took the alternate screen, which hid them: start over.
		m.restart(out, screen, alt, top)
	case alt != m.alt:
		if alt {
			out.WriteString("\x1b[?1049h")
			m.main, m.sent = m.sent, make([][]byte, rows)
		} else {
			out.WriteString("\x1b[?1049l")
			m.sent, m.main = m.main, nil
		}
		m.redraw(out, screen)
	default:
		scrolled := 0
		if !alt {
			scrolled = m.scrolled(ids, scrollback)
		}
		switch {
		case scrolled < 0: // lost track: start over
			m.restart(out, screen, alt, top)
		case scrolled > 0:
			// Rows went to the scrollback: they pass through the other
			// screen's top in order, the screen's rows after them. A
			// scrollback that holds fewer rows was cleared when none of
			// those before them are left: one at its limit lets go of its
			// oldest rows instead, which the other terminal keeps, its own
			// limit taking them.
			if scrollback < m.scrollback && scrollback <= scrolled {
				out.WriteString("\x1b[3J")
			}
			m.write(out, append(m.lines(top-scrolled, top), screen...))
		default:
			if scrollback < m.scrollback {
				out.WriteString("\x1b[3J") // the scrollback was cleared
			}
			m.redraw(out, screen)
		}
	}
	m.started, m.cols, m.rows, m.alt, m.ids = true, cols, rows, alt, ids
	if !alt {
		m.scrollback = scrollback
	}
	m.images(out, term, alt, top, rows)
	m.state(out, rs.Cursor())
}

// scrolled returns how many rows scrolled off the screen since the last
// frame, which ids are the rows of now, or -1 when it cannot tell.
func (m *Mirror) scrolled(ids []vt.RowID, scrollback int) int {
	if len(ids) == 0 || len(m.ids) == 0 {
		return -1
	}
	k := -1
	for i, id := range m.ids {
		if id == ids[0] {
			k = i
			break
		}
	}
	grew := scrollback - m.scrollback
	switch {
	case k == 0 && grew > 0:
		// The screen's rows kept their places, the scrollback grew under
		// a full screen's worth of rows: more than a screen scrolled.
		return grew
	case k >= 0:
		return k
	}
	// More than a screen scrolled: as many rows as are below the last
	// frame's first row, now in the scrollback. The scrollback's growth
	// tells it too, but for one at its limit, which lets go of rows as it
	// takes others.
	if n := scrolledPast(m.t.term, m.ids[0], scrollback); n > 0 {
		return n
	}
	if grew > 0 {
		return grew
	}
	return -1
}

// scrolledPast returns how many rows of the scrollback, which holds
// scrollback rows, are below the row id, from the bottom up, a screen's
// worth at a time, or -1 when it holds no such row. It leaves the viewport
// at the bottom.
func scrolledPast(term *vt.Terminal, id vt.RowID, scrollback int) int {
	_, rows := term.Size()
	defer term.ScrollToBottom()
	for y := scrollback; y > 0; {
		y = max(y-rows, 0)
		term.ScrollToRow(y)
		// A render state of its own, as readLines reads.
		rs, err := vt.NewRenderState()
		if err != nil {
			return -1
		}
		rs.Update(term)
		off := int(term.Scrollbar().Offset)
		found := -1
		rs.Rows()
		for i := 0; rs.NextRow(); i++ {
			if r := off + i; r < scrollback && found < 0 && rs.RowID() == id {
				found = scrollback - r
			}
		}
		rs.Free()
		if found >= 0 {
			return found
		}
		y = min(y, off)
	}
	return -1
}

// restart starts the other terminal over: its scrollback and screen, then
// this one's, row by row.
func (m *Mirror) restart(out *bytes.Buffer, screen []line, alt bool, top int) {
	// Both screens' images go too, which frames send again: those sent by
	// their ids, as deleting the placements that show (d=A) leaves those
	// of the scrollback, which the other terminal keeps over its screen
	// once the scrollback is cleared under them.
	const noImages = "\x1b_Ga=d,d=A,q=2\x1b\\"
	out.WriteString("\x1b[?1049h" + noImages)
	m.deleteImages(out, &m.imgs[1])
	out.WriteString("\x1b[?1049l" + noImages)
	m.deleteImages(out, &m.imgs[0])
	out.WriteString("\x1b[0m\x1b[H\x1b[2J\x1b[3J")
	m.modes, m.cursor = "", ""
	m.imgs = [2]mirroredImages{}
	m.sent = make([][]byte, len(screen))
	m.main = nil
	if alt {
		// The main screen of a full-screen program's terminal is not to be
		// read: the other one's shows from when the program ends, which
		// starts over again, as the other's main screen and scrollback are
		// empty now.
		m.scrollback = -1
		out.WriteString("\x1b[?1049h")
		m.redraw(out, screen)
		return
	}
	m.write(out, append(m.lines(0, top), screen...))
}

// lines reads the rows of the scrollback from up to to, counted from its
// top.
func (m *Mirror) lines(from, to int) []line { return readLines(m.t.term, from, to) }

// readLines reads rows from up to to, counted from the top of the
// scrollback, a screen's worth at a time, through the viewport, which it
// leaves at the bottom; each with a render state of its own (see frame).
func readLines(term *vt.Terminal, from, to int) []line {
	out := make([]line, 0, max(0, to-from))
	if to <= from {
		return out
	}
	_, rows := term.Size()
	for y := from; y < to; {
		term.ScrollToRow(y)
		rs, err := vt.NewRenderState()
		if err != nil {
			break
		}
		rs.Update(term)
		off := int(term.Scrollbar().Offset)
		rs.Rows()
		for i := 0; rs.NextRow(); i++ {
			if r := off + i; r == from+len(out) && r < to {
				out = append(out, readLine(term, rs, i))
			}
		}
		rs.Free()
		next := off + rows
		if next <= y {
			break
		}
		y = next
	}
	term.ScrollToBottom()
	for len(out) < to-from {
		out = append(out, line{}) // not to be read: drawn empty
	}
	return out
}

// write draws lines one after the other from the other screen's top, so
// that those before the screen's, its last rows, go to its scrollback, as
// they went to this one's.
func (m *Mirror) write(out *bytes.Buffer, lines []line) {
	rows := len(m.sent)
	screen := len(lines) - rows
	out.WriteString("\x1b[H")
	for i, l := range lines {
		last := i == len(lines)-1
		var next *line
		if !last {
			next = &lines[i+1]
		}
		k := m.row(out, l, next, i > 0 && lines[i-1].wrap)
		if i >= screen {
			m.sent[i-screen] = k
		}
		if !last && !l.wrap {
			out.WriteString("\r\n")
		}
	}
}

// redraw draws again the rows of the screen that changed, and those of the
// soft-wrapped lines they are in, from their lines' first rows.
func (m *Mirror) redraw(out *bytes.Buffer, screen []line) {
	rows := len(screen)
	if len(m.sent) != rows {
		m.sent = make([][]byte, rows)
	}
	for y := 0; y < rows; y++ {
		if m.sent[y] != nil && bytes.Equal(screen[y].key(), m.sent[y]) {
			continue
		}
		// From the first row of its line, which a soft wrap continues.
		s := y
		for s > 0 && screen[s-1].wrap {
			s--
		}
		fmt.Fprintf(out, "\x1b[%dH", s+1)
		for first := s; s < rows; s++ {
			var next *line
			if s+1 < rows && screen[s].wrap {
				next = &screen[s+1]
			}
			m.sent[s] = m.row(out, screen[s], next, s > first)
			if next == nil {
				break
			}
		}
		y = s
	}
}

// row draws a line from the cursor's column 0: in full, clearing what was
// after it, or, soft wrapped into next, out to its last column for the
// other terminal to wrap it too. A row is cleared before it is drawn, but
// for one a soft wrap continues, cont, which starts with the cursor still
// at the end of the row before, waiting to wrap, which a clear would clear
// instead; with nothing on it, a blank wraps there first, and the clear
// after takes it. It returns what identifies how it drew it.
func (m *Mirror) row(out *bytes.Buffer, l line, next *line, cont bool) []byte {
	cols := m.t.size.cols
	if c, _ := m.t.term.Size(); c > 0 {
		cols = c
	}
	out.WriteString("\x1b[0m")
	switch {
	case !cont:
		// Cleared whole, which also undoes a wrap it had.
		out.WriteString("\x1b[2K")
	case len(l.vt) == 0:
		// The blank wraps the row before into this one, then goes.
		out.WriteString(" \r")
	}
	out.Write(l.vt)
	out.WriteString("\x1b[0m")
	if l.wrap && next != nil {
		// Out to the last column, which the next character wraps from;
		// short of it when a wide character did not fit there, for the
		// other terminal to wrap it too.
		to := cols
		if l.spacerHead && next.startsWide {
			to--
		}
		if l.end < to {
			out.Write(bytes.Repeat([]byte{' '}, to-l.end))
		}
	} else if l.end < cols {
		out.WriteString("\x1b[K")
	}
	return l.key()
}

// line is a row of a terminal as a Mirror draws it.
type line struct {
	// vt is its cells, from the first to the last that holds something,
	// as VT sequences, each cell in its own style: an empty cell in none.
	vt []byte
	// end is how many columns hold something: text, the second half of a
	// wide character, a style or a background.
	end int
	// wrap tells that it soft-wraps into the next row; spacerHead that a
	// wide character that did not fit at its end went on to the next row;
	// startsWide that it starts with a wide character.
	wrap, spacerHead, startsWide bool
}

// key identifies how the line shows.
func (l line) key() []byte {
	w := byte(0)
	if l.wrap {
		w = 1
	}
	return append([]byte{w}, l.vt...)
}

// readLine reads the row the render state's row iterator is on, row y of
// the viewport.
func readLine(term *vt.Terminal, rs *vt.RenderState, y int) line {
	cells := rs.RowCells()
	l := line{wrap: rs.RowWrapped()}
	for x, c := range cells {
		if c.Codepoint != 0 || c.Grapheme || c.HasBackground || c.Style != 0 || c.Wide == vt.SpacerTail {
			l.end = x + 1
		}
	}
	if n := len(cells); n > 0 {
		l.spacerHead = cells[n-1].Wide == vt.SpacerHead
		l.startsWide = cells[0].Wide == vt.WideChar
	}
	var b bytes.Buffer
	sgr, link := "\x1b[0m", ""
	styles := map[uint16]string{}
	var buf []rune
	for x := 0; x < l.end; x++ {
		c := cells[x]
		if c.Wide == vt.SpacerTail || c.Wide == vt.SpacerHead {
			continue
		}
		want := "\x1b[0m"
		switch {
		case c.HasBackground && c.Codepoint == 0 && !c.Grapheme:
			// A cell without text that a program filled.
			bg := vt.Color{Kind: 2, RGB: c.Background}
			if c.Palette {
				bg = vt.Color{Kind: 1, Index: c.Background.R}
			}
			want = vt.Style{BG: bg}.SGR()
		case c.Style != 0:
			s, ok := styles[c.Style]
			if !ok {
				s = rs.CellStyle(x).SGR()
				styles[c.Style] = s
			}
			want = s
		}
		if want != sgr {
			b.WriteString(want)
			sgr = want
		}
		uri := ""
		if c.Hyperlink {
			if ref, ok := term.CellAt(x, y); ok {
				uri = term.Hyperlink(ref)
			}
		}
		if uri != link {
			fmt.Fprintf(&b, "\x1b]8;;%s\x1b\\", uri)
			link = uri
		}
		buf = buf[:0]
		if c.Grapheme {
			buf = rs.CellGraphemes(x, buf)
		}
		switch {
		case len(buf) > 0:
			for _, r := range buf {
				b.WriteRune(r)
			}
		case c.Codepoint != 0:
			b.WriteRune(c.Codepoint)
		default:
			b.WriteByte(' ')
		}
	}
	if link != "" {
		b.WriteString("\x1b]8;;\x1b\\")
	}
	if sgr != "\x1b[0m" {
		b.WriteString("\x1b[0m")
	}
	l.vt = b.Bytes()
	return l
}

// mirroredModes are the modes the other terminal takes: those that change
// what it sends, as the keys and the mouse, and how it draws. Those off
// go first: the mouse's events and format are one setting each, which
// setting one of their modes picks and resetting any sets back to none or
// X10's, as Ghostty does.
var mirroredModes = []vt.Mode{
	vt.ModeCursorKeys, 66, // DECCKM, DECNKM (the keypad's)
	67, 1035, 1036, 1039, // DECBKM (Backspace's), NumLock's keypad, Alt's ESC
	vt.ModeFocusEvent, vt.ModeAltScroll, vt.ModeBracketedPaste, vt.ModeGraphemes,
	9, 1000, 1002, 1003, 1005, 1006, 1015, 1016, // the mouse's
	5, // DECSCNM, the screen in reverse video
}

// mouseEvents and mouseFormats are the mouse's modes of each of its two
// settings.
var (
	mouseEvents  = []vt.Mode{9, 1000, 1002, 1003}
	mouseFormats = []vt.Mode{1005, 1006, 1015, 1016}
)

// mouseLast orders the modes on so that the mouse's in effect come last,
// for the other terminal to take the events and the format the terminal
// sends: the last of a setting's modes set picks it, and its others stay
// set, so that crossterm's 1015 then 1006, as herdr sets them, are SGR's
// and not URxvt's. off is a mode to reset after, when the modes on are
// none in effect: a reset of one picks none or X10's again, and leaves the
// others set. A mouse encoder tells which are in effect by what it sends.
func mouseLast(term *vt.Terminal, on []vt.Mode) (_ []vt.Mode, off vt.Mode) {
	var events, formats int
	for _, md := range on {
		if slices.Contains(mouseEvents, md) {
			events++
		} else if slices.Contains(mouseFormats, md) {
			formats++
		}
	}
	if events == 0 {
		return on, 0 // no events, of which the format is the format
	}
	e, err := vt.NewMouseEncoder()
	if err != nil {
		return on, 0
	}
	defer e.Free()
	// Cells of 10 by 10 pixels, 300 of them across: X10's coordinates go
	// up to 223, past which UTF-8's take two bytes.
	sends := func(pressed bool, action vt.MouseAction, button int, x float32) []byte {
		e.Sync(term, 3000, 100, 10, 10, 0, 0, pressed)
		return e.Encode(action, button, 0, x, 5)
	}
	var event vt.Mode
	switch {
	case len(sends(false, vt.MouseMotion, vt.ButtonNone, 5)) > 0:
		event = 1003
	case len(sends(true, vt.MouseMotion, vt.ButtonLeft, 15)) > 0:
		event = 1002
	case len(sends(false, vt.MouseRelease, vt.ButtonLeft, 5)) > 0:
		event = 1000
	case len(sends(false, vt.MousePress, vt.ButtonLeft, 5)) > 0:
		event = 9
	}
	on = moveLast(on, event, mouseEvents)
	if event == 0 {
		return on, mouseEventsOn(on)
	}
	if formats == 0 {
		return on, 0
	}
	var format vt.Mode
	switch b := sends(false, vt.MousePress, vt.ButtonLeft, 1505); {
	case bytes.HasPrefix(b, []byte("\x1b[<")):
		format = 1006
		if bytes.Contains(b, []byte(";1505;")) {
			format = 1016
		}
	case bytes.HasPrefix(b, []byte("\x1b[M")):
		if len(b) > 6 {
			format = 1005
		}
	case len(b) > 0:
		format = 1015
	}
	on = moveLast(on, format, mouseFormats)
	if format == 0 {
		for _, md := range on {
			if slices.Contains(mouseFormats, md) {
				off = md
			}
		}
	}
	return on, off
}

// moveLast moves md to the end of the modes on, when it's on, or else
// none of the setting's modes.
func moveLast(on []vt.Mode, md vt.Mode, setting []vt.Mode) []vt.Mode {
	if md == 0 || !slices.Contains(on, md) {
		return on
	}
	on = slices.DeleteFunc(slices.Clone(on), func(o vt.Mode) bool { return o == md })
	return append(on, md)
}

// mouseEventsOn returns a mouse's event mode among those on.
func mouseEventsOn(on []vt.Mode) vt.Mode {
	for _, md := range on {
		if slices.Contains(mouseEvents, md) {
			return md
		}
	}
	return 0
}

// programColors returns the sequences that set the colors programs set
// (OSC 4, 10, 11, 12) and reset the others, which the other terminal's
// own defaults then color.
func programColors(term *vt.Terminal) string {
	set, def := term.Colors(false), term.Colors(true)
	var b bytes.Buffer
	osc := func(code string, c vt.RGB) {
		fmt.Fprintf(&b, "\x1b]%s;rgb:%02x/%02x/%02x\x1b\\", code, c.R, c.G, c.B)
	}
	for _, c := range []struct {
		code      string
		set, def  vt.RGB
		has, dhas bool
	}{
		{"10", set.Foreground, def.Foreground, set.HasForeground, def.HasForeground},
		{"11", set.Background, def.Background, set.HasBackground, def.HasBackground},
		{"12", set.Cursor, def.Cursor, set.HasCursor, def.HasCursor},
	} {
		if c.has && (!c.dhas || c.set != c.def) {
			osc(c.code, c.set)
		} else {
			fmt.Fprintf(&b, "\x1b]1%s\x1b\\", c.code)
		}
	}
	b.WriteString("\x1b]104\x1b\\")
	for i := range set.Palette {
		if set.Palette[i] != def.Palette[i] {
			osc("4;"+strconv.Itoa(i), set.Palette[i])
		}
	}
	return b.String()
}

// state sets the cursor, the modes and the title, where they changed, and
// passes on programs' uses of the clipboard.
func (m *Mirror) state(out *bytes.Buffer, c vt.Cursor) {
	t := m.t
	term := t.term
	var modes bytes.Buffer
	var on []vt.Mode
	for _, md := range mirroredModes {
		if term.Mode(md) {
			on = append(on, md)
		} else {
			fmt.Fprintf(&modes, "\x1b[?%dl", md)
		}
	}
	on, off := mouseLast(term, on)
	for _, md := range on {
		fmt.Fprintf(&modes, "\x1b[?%dh", md)
	}
	if off != 0 {
		fmt.Fprintf(&modes, "\x1b[?%dl", off)
	}
	mok := 1
	if term.ModifyOtherKeys() {
		mok = 2
	}
	fmt.Fprintf(&modes, "\x1b[=%d;1u\x1b[>4;%dm", term.KittyKeyboardFlags(), mok)
	if shape := term.MouseShape(); shape >= 0 && shape < len(shapeNames) {
		fmt.Fprintf(&modes, "\x1b]22;%s\x1b\\", shapeNames[shape])
	}
	if s := modes.String(); s != m.modes {
		out.WriteString(s)
		m.modes = s
	}
	if colors := programColors(term); colors != m.colors {
		out.WriteString(colors)
		m.colors = colors
	}
	if title := term.Title(); title != m.title {
		fmt.Fprintf(out, "\x1b]2;%s\x1b\\", title)
		m.title = title
	}
	for _, text := range m.clipOut {
		fmt.Fprintf(out, "\x1b]52;c;%s\x1b\\", base64.StdEncoding.EncodeToString([]byte(text)))
	}
	m.clipOut = nil
	x, y := term.Cursor()
	var cur bytes.Buffer
	// A cursor as this terminal's options make it, which programs left
	// alone or reset, is the default of the other terminal too (DECSCUSR
	// 0), which takes its own options': a client's Cursor shows.
	shape := 0
	if c.Style != vtCursor(t.opts.Cursor) || c.Blinking {
		shape = map[vt.CursorStyle]int{vt.CursorBlock: 2, vt.CursorUnderline: 4, vt.CursorBar: 6}[c.Style]
		if shape == 0 {
			shape = 2
		}
		if c.Blinking {
			shape--
		}
	}
	visible := "l"
	if c.Visible {
		visible = "h"
	}
	fmt.Fprintf(&cur, "\x1b[0m\x1b[%d;%dH\x1b[?25%s\x1b[%d q", y+1, x+1, visible, shape)
	// After rows, the cursor goes back, though it did not move.
	if s := cur.String(); s != m.cursor || out.Len() > 0 {
		out.WriteString(s)
		m.cursor = s
	}
}

// shapeNames are the pointer shapes of OSC 22, in Ghostty's order.
var shapeNames = []string{"default", "context-menu", "help", "pointer", "progress", "wait", "cell", "crosshair",
	"text", "vertical-text", "alias", "copy", "move", "no-drop", "not-allowed", "grab", "grabbing", "all-scroll",
	"col-resize", "row-resize", "n-resize", "e-resize", "s-resize", "w-resize", "ne-resize", "nw-resize",
	"se-resize", "sw-resize", "ew-resize", "ns-resize", "nesw-resize", "nwse-resize", "zoom-in", "zoom-out"}

// History returns what the terminal shows as rows of VT sequences, for a
// terminal that takes the session over, as after the program that wrote
// them ended: the scrollback and the screen down to the cursor's row, or
// to the last row with something on it when that is above, without the
// cursor and the modes. Fed to a terminal of the same width, it leaves the
// cursor below what it shows, for the next program to start under it, not
// over it. A full-screen program's screen has no history: History returns
// nothing while the alternate screen shows.
func (t *Terminal) History() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	term := t.term
	if term == nil || term.AltScreen() {
		return nil
	}
	cols, rows := term.Size()
	sb := term.ScrollbackRows()
	lines := readLines(term, 0, sb+rows)
	_, y := term.Cursor()
	end := min(sb+y, len(lines)-1) // the last line, down to the cursor's
	for end >= 0 && lines[end].end == 0 {
		end--
	}
	var out bytes.Buffer
	for i, l := range lines[:end+1] {
		out.Write(l.vt)
		out.WriteString("\x1b[0m")
		if l.wrap && i < end {
			// Soft wrapped: on to the last column, for the next row to wrap.
			to := cols
			if l.spacerHead && lines[i+1].startsWide {
				to--
			}
			if l.end < to {
				out.Write(bytes.Repeat([]byte{' '}, to-l.end))
			}
			continue
		}
		out.WriteString("\r\n")
	}
	return out.Bytes()
}
