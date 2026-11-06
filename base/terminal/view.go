package terminal

import (
	"math"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/base/terminal/internal/vt"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/i18n"
)

// View shows a terminal and takes its keyboard. Its size sets the
// terminal's, in cells of its font; a press gives it the keyboard focus.
// A terminal shows in one view at a time.
//
// The screen is a canvas. A vertical Scroll over it is the hit layer:
// the wheel, the touchpad, a finger and the scroll bar move through the
// scrollback, and a press and a drag select (two presses a word, three a
// line, Shift extends), a long press of a finger a word. An invisible
// text input at the cursor, the sink, takes typed text and input methods;
// the keys that type no text are bound while it has the focus. Copy is
// Command+C on Apple platforms and Control+Shift+C elsewhere; paste
// Command+V, or Control+Shift+V; Ref gives them to the app's menus.
//
// Links, programs' hyperlinks (OSC 8) and URLs printed as text, underline
// faintly under the mouse, fully with a hand with Command held, or Control
// off Apple platforms, when a click opens them with Scope.OpenURL.
//
// On touch screens a bar of the keys software keyboards lack shows over
// the keyboard while the view has the focus, and the grid keeps to what
// the keyboard leaves of the view.
type View struct {
	Terminal *Terminal
	// Font is the terminal's font: the platform's fixed-pitch font at 13
	// points when its family and size are unset. Bold and italic text
	// take its variants.
	Font core.Font
	// Theme colors the terminal in light appearances, and DarkTheme in
	// dark ones; LightTheme and DarkTheme when nil.
	Theme, DarkTheme *Theme
	// Padding is the room around the grid: 6 points across and 4 down
	// when zero, none when negative.
	Padding Padding
	// AutoFocus gives the view the keyboard focus as it mounts.
	AutoFocus bool
	// Transparent leaves the terminal's default background undrawn, so
	// what is behind the view shows through.
	Transparent bool
	// Class is Tailwind CSS classes, as core.View's, under Style.
	Class string
	Style core.Style
	Label string
	// TestID names the view, and with a suffix its parts for tests: the
	// screen "-screen", the key bar's keys "-key-" and the key's name.
	TestID  string
	OnFocus func()
	OnBlur  func()
	// OnSizeChange receives the size of the screen in cells as the view
	// sets it.
	OnSizeChange func(cols, rows int)
	// Ref, when set, is bound to the view as it mounts.
	Ref *Ref

	// KeyBarKeys are the keys of the bar over a touch screen's keyboard,
	// as core names them ("escape", "ctrl", "left") or characters ("|"):
	// DefaultKeyBar when nil. NoKeyBar shows none.
	KeyBarKeys []string
	NoKeyBar   bool
	// KeyBarClass and KeyBarStyle style the bar, a horizontal Scroll;
	// KeyClass and KeyStyle each key, with KeyOnClass and KeyOnStyle over
	// them while Control or Alt is latched for the next key. A part's
	// StyleFunc merges over its Style and follows the signals it reads.
	KeyBarClass     string
	KeyBarStyle     core.Style
	KeyBarStyleFunc func() core.Style
	KeyClass        string
	KeyStyle        core.Style
	KeyStyleFunc    func() core.Style
	KeyOnClass      string
	KeyOnStyle      core.Style
	KeyOnStyleFunc  func() core.Style
}

// Padding is the room around a terminal's grid, in points (View.Padding).
type Padding struct{ X, Y core.Points }

// Render implements core.Node.
func (p View) Render(cx *core.Scope) core.Node {
	v := &view{p: p, t: p.Terminal, cx: cx,
		rev: core.NewSignal(cx, 0), sink: core.NewSignal(cx, ""), composing: core.NewSignal(cx, ""),
		focused: core.NewSignal(cx, false), content: core.NewSignal(cx, core.Points(0)),
		caret: core.NewSignal(cx, core.Point{}), sinkRef: &core.Ref{}, scrollRef: &core.Ref{},
		selected: core.NewSignal(cx, false), ctrlOn: core.NewSignal(cx, false), altOn: core.NewSignal(cx, false),
		linkOn: core.NewSignal(cx, false),
		bounds: core.NewSignal(cx, core.Bounds{}), rootRef: &core.Ref{}}
	if p.Ref != nil {
		p.Ref.v = v
	}
	v.cellW, v.cellH = 8, 16
	if v.t != nil {
		draw := func() {
			if !v.pending.Swap(true) {
				cx.Post(func() {
					v.pending.Store(false)
					v.rev.Update(func(n int) int { return n + 1 })
				})
			}
		}
		v.t.draw.Store(&draw)
		cx.Cleanup(func() {
			v.t.draw.CompareAndSwap(&draw, nil)
			v.t.mu.Lock()
			if v.gesture != nil {
				v.gesture.Free(v.t.term)
			}
			if v.keys != nil {
				v.keys.Free()
			}
			v.t.mu.Unlock()
		})
	}
	style := p.Style
	if style.Position == 0 {
		style.Position = core.Relative
	}
	style.Overflow = core.Hidden
	fill := core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)}
	return core.View{
		Class: p.Class, Style: style, TestID: p.TestID, Ref: v.rootRef,
		OnLayout: func(core.Bounds) {
			if b, ok := v.rootRef.Bounds(); ok {
				v.bounds.Set(b)
			}
		},
		Children: []core.Node{
			// The grid and the key bar keep above a software keyboard.
			core.View{StyleFunc: func() core.Style {
				return core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(v.keyboardOverlap())}
			}, Children: []core.Node{
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), Position: core.Relative, Overflow: core.Hidden}, Children: []core.Node{
					core.Canvas{Style: fill, Draw: v.draw},
					core.Scroll{
						Ref: v.scrollRef, Style: fill, TestID: v.testID("scroll"), OnScroll: v.scrolled, Behavior: keepKeyboard,
						Children: []core.Node{core.View{
							StyleFunc: func() core.Style {
								cursor := core.CursorText
								if v.linkOn.Get() {
									cursor = core.CursorPointer
								}
								return core.Style{Height: core.Pt(v.content.Get()), Cursor: cursor}
							},
							NoFocus:      true,
							TestID:       v.testID("screen"),
							OnPressDown:  v.pressDown,
							OnPressUp:    v.pressUp,
							OnMouseMove:  v.mouseMove,
							OnMouseLeave: v.mouseLeave,
							OnLongPress:  v.longPress,
							// A scroll view holds its content's touches back
							// 150 ms: the word is selected before a context
							// menu's hold of 400 ms opens it.
							DelayLongPress: 200 * time.Millisecond,
						}},
					},
					v.sinkView(),
				}},
				v.keyBar(),
			}},
			// Keys go to an input method while it composes.
			core.Show{When: func() bool { return v.focused.Get() && v.composing.Get() == "" }, Then: func(cx *core.Scope) core.Node {
				v.bindKeys(cx)
				return nil
			}},
		},
	}
}

// view is a View's state, on the engine's goroutine.
type view struct {
	// searched is the status OnSearch was last told; matches are the
	// search's spans the viewport shows, in matchBuf's memory.
	searched SearchStatus
	matches  []matchSpan
	matchBuf []vt.Selection

	p  View
	t  *Terminal
	cx *core.Scope
	// rev changes when the terminal asks for a frame; pending is a frame
	// asked for and not yet posted.
	rev     *core.Signal[int]
	pending atomic.Bool
	// sink is what the sink holds, composing what an input method
	// composes in it, and sent the sink's text the program has.
	sink, composing *core.Signal[string]
	sent            string
	focused         *core.Signal[bool]
	// content is the scroll layer's height, caret the sink's place.
	content            *core.Signal[core.Points]
	caret              *core.Signal[core.Point]
	sinkRef, scrollRef *core.Ref
	// selected says whether text is selected (Ref.Selected); ctrlOn and
	// altOn are Control and Alt latched on the key bar for the next key.
	selected, ctrlOn, altOn *core.Signal[bool]
	// bounds is the view's frame in the surface, for the keyboard.
	bounds  *core.Signal[core.Bounds]
	rootRef *core.Ref
	// link is the link under the mouse, hover the cell of the viewport the
	// mouse is over while hovered, armed that the key opening links is
	// held, linkOn that the mouse shows a hand as a press would open link,
	// and pressedLink the URL a press on a link opens as it ends there.
	link        link
	hover       [2]int
	hovered     bool
	armed       bool
	linkOn      *core.Signal[bool]
	pressedLink string

	// The grid as the last drawing laid it out, in points.
	cellW, cellH, padX, padY float32
	cols, rows               int
	reported                 [2]int // the size OnSizeChange last had
	ratio                    float32
	// scrollY is the scroll layer's offset; setY the one the view last
	// scrolled it to, whose report it ignores.
	scrollY, setY float64

	// Guarded by the terminal's mu.
	keys      *vt.KeyEncoder
	gesture   *vt.Gesture
	selecting bool
	extending bool
	// lastMouse is where the pointer was in the scroll layer's content
	// while it selects.
	lastMouse core.Point
}

// font returns the font of a variant: bold, italic.
func (v *view) font(bold, italic bool) core.Font {
	f := v.p.Font
	if f.Family == "" {
		f.Family = core.FontMonospace
	}
	if f.Size == 0 {
		f.Size = 13
	}
	if bold {
		f.Weight = core.FontWeightBold
	}
	f.Italic = f.Italic || italic
	return f
}

func (v *view) theme(dark bool) *Theme {
	if dark {
		if v.p.DarkTheme != nil {
			return v.p.DarkTheme
		}
		return defaultTheme(true)
	}
	if v.p.Theme != nil {
		return v.p.Theme
	}
	return defaultTheme(false)
}

// layout sizes the grid for the canvas's size.
func (v *view) layout(d *core.Drawing, size core.Size) {
	f := v.font(false, false)
	m := d.TextMetrics("M", f)
	v.cellW = max(d.MeasureText(strings.Repeat("M", 16), f)/16, 1)
	v.cellH = max(float32(math.Ceil(float64(max(m.LineHeight, m.Ascent+m.Descent)))), 1)
	v.padX, v.padY = 6, 4
	if p := v.p.Padding; p != (Padding{}) {
		v.padX, v.padY = max(float32(p.X), 0), max(float32(p.Y), 0)
	}
	v.cols = max(int((float32(size.Width)-2*v.padX)/v.cellW), 1)
	v.rows = max(int((float32(size.Height)-2*v.padY)/v.cellH), 1)
}

// draw draws the terminal's screen as the render state has it.
func (v *view) draw(d *core.Drawing, size core.Size) {
	v.rev.Get()
	cx := v.cx
	dark := cx.Env().Dark()
	v.ratio = max(cx.Env().PixelRatio(), 1)
	th := v.theme(dark)
	focused := v.focused.Get()
	composing := v.composing.Get()
	v.layout(d, size)
	t := v.t
	if t == nil {
		d.SetFillColor(th.Background).FillRect(0, 0, float32(size.Width), float32(size.Height))
		return
	}
	t.mu.Lock()
	if t.term == nil {
		t.mu.Unlock()
		d.SetFillColor(th.Background).FillRect(0, 0, float32(size.Width), float32(size.Height))
		return
	}
	t.useTheme(th)
	t.lang = i18n.Language(cx)
	t.viewResize(gridSize{v.cols, v.rows, int(v.cellW * v.ratio), int(v.cellH * v.ratio)})
	// A synchronized update shows when it ends, or after a second.
	if t.held && time.Since(t.heldSince) > time.Second {
		t.term.SetMode(vt.ModeSyncOutput, false)
		t.held = false
	}
	if !t.held {
		t.rs.Update(t.term)
		// Output and scrolling move what is under the mouse.
		v.findLink()
	}
	sb := t.term.Scrollbar()
	if cols, rows := v.cols, v.rows; v.p.OnSizeChange != nil && [2]int{cols, rows} != v.reported {
		v.reported = [2]int{cols, rows}
		cx.Post(func() { v.p.OnSizeChange(cols, rows) })
	}
	searching := t.find.drive(t.term)
	searched := t.find.status
	cols, rows := t.term.Size()
	v.matches, v.matchBuf = t.viewportMatches(v.matches, v.matchBuf, cols, rows)
	fr := frame{v: v, d: d, rs: t.rs, th: th, focused: focused, composing: composing}
	fr.paint(size)
	fr.paintMatches()
	t.mu.Unlock()
	if searched != v.searched {
		v.searched = searched
		if on := t.opts.OnSearch; on != nil {
			cx.Post(func() { on(searched) })
		}
	}
	if searching {
		cx.Post(func() { v.rev.Update(func(n int) int { return n + 1 }) })
	}
	if sel := fr.anySel; sel != v.selected.GetUntracked() {
		cx.Post(func() { v.selected.Set(sel) })
	}
	v.follow(sb, size)
}

// paintMatches marks the matches of the search the viewport shows.
func (f *frame) paintMatches() {
	v, th := f.v, f.th
	or := func(c, fallback core.Color) core.Color {
		if c != 0 {
			return c
		}
		return fallback
	}
	match := or(th.Search, core.RGB(0xffe082)).Faded(0.45)
	selected := or(th.SearchSelected, core.RGB(0xf2a57e)).Faded(0.6)
	for _, m := range v.matches {
		c := match
		if m.selected {
			c = selected
		}
		f.d.SetFillColor(c).FillRect(v.padX+float32(m.x0)*v.cellW, v.padY+float32(m.y)*v.cellH, float32(m.x1-m.x0+1)*v.cellW, v.cellH)
	}
}

// follow sizes the scroll layer for the scrollback and brings it to the
// terminal's viewport, after the commit that draws.
func (v *view) follow(sb vt.Scrollbar, size core.Size) {
	content := size.Height + core.Points(float32(sb.Total-min(sb.Len, sb.Total))*v.cellH)
	v.cx.Post(func() {
		v.content.Set(content)
		v.followRow()
	})
}

// followRow moves the scroll layer to the row of the terminal's viewport.
// It reads the terminal, as the user may have scrolled since the frame. A
// layer on that row stays where it is: it rests between rows as the user
// scrolls it, and moving it would stop the gesture and its momentum.
func (v *view) followRow() {
	t := v.t
	t.mu.Lock()
	if t.term == nil {
		t.mu.Unlock()
		return
	}
	row := t.term.Scrollbar().Offset
	t.mu.Unlock()
	if int(math.Round(v.scrollY/float64(v.cellH))) == int(row) {
		return
	}
	y := float64(row) * float64(v.cellH)
	v.setY, v.scrollY = y, y
	v.scrollRef.ScrollTo(core.Point{Y: core.Points(y)}, false)
}

// scrolled moves the terminal's viewport with the scroll layer.
func (v *view) scrolled(e core.ScrollEvent) {
	y := float64(e.Offset.Y)
	v.scrollY = y
	if math.Abs(y-v.setY) < 0.5 || v.t == nil {
		return
	}
	v.setY = math.NaN()
	row := int(math.Round(y / float64(v.cellH)))
	t := v.t
	t.mu.Lock()
	if t.term != nil && uint64(row) != t.term.Scrollbar().Offset {
		t.term.ScrollToRow(row)
		if v.selecting {
			v.dragTo(v.lastMouse)
		}
	}
	t.mu.Unlock()
	t.redraw()
}

// frame draws one frame.
type frame struct {
	v       *view
	d       *core.Drawing
	rs      *vt.RenderState
	th      *Theme
	colors  vt.Colors
	focused bool
	anySel  bool
	// composing is what an input method composes.
	composing string
}

// resolved is a cell's style with its colors resolved.
type resolved struct {
	fg, bg, ul         core.Color
	hasBg, hasUl       bool
	bold, italic       bool
	inverse, invisible bool
	underline          int
	strike, overline   bool
}

func (f *frame) resolve(s vt.Style) resolved {
	pal := &f.colors.Palette
	col := func(c vt.Color, def vt.RGB) (core.Color, bool) {
		switch c.Kind {
		case 1:
			return color(pal[c.Index]), true
		case 2:
			return color(c.RGB), true
		}
		return color(def), false
	}
	var r resolved
	r.fg, _ = col(s.FG, f.colors.Foreground)
	r.bg, r.hasBg = col(s.BG, f.colors.Background)
	r.ul, r.hasUl = col(s.UnderlineColor, f.colors.Foreground)
	if s.Inverse {
		r.fg, r.bg, r.hasBg, r.inverse = r.bg, r.fg, true, true
	}
	if s.Faint {
		r.fg = r.fg.Faded(0.5)
	}
	r.bold, r.italic = s.Bold, s.Italic
	r.invisible, r.underline, r.strike, r.overline = s.Invisible, s.Underline, s.Strikethrough, s.Overline
	return r
}

// run is text of one style in consecutive cells.
type run struct {
	x            int
	text         []rune
	fg           core.Color
	bold, italic bool
}

func (f *frame) paint(size core.Size) {
	v, d, rs := f.v, f.d, f.rs
	f.colors = rs.Colors()
	cw, ch := v.cellW, v.cellH
	ox, oy := v.padX, v.padY
	m := d.TextMetrics("M", v.font(false, false))
	base := (ch-(m.Ascent+m.Descent))/2 + m.Ascent
	bg := color(f.colors.Background)
	if !v.p.Transparent {
		d.SetFillColor(bg).FillRect(0, 0, float32(size.Width), float32(size.Height))
	}
	selection := f.th.Selection
	if selection == 0 {
		selection = color(f.colors.Foreground).Faded(0.3)
	}
	cur := rs.Cursor()
	cursorColor := color(f.colors.Foreground)
	if f.colors.HasCursor {
		cursorColor = color(f.colors.Cursor)
	}
	cursorText := f.th.CursorText
	if cursorText == 0 {
		cursorText = bg
	}
	var cursorRun run
	def := resolved{fg: color(f.colors.Foreground), bg: bg}
	cols, _ := rs.Size()
	var buf []rune
	rs.Rows()
	for y := 0; rs.NextRow(); y++ {
		ry := oy + float32(y)*ch
		cells := rs.RowCells()
		selA, selB, sel := rs.RowSelection()
		linkA, linkB, linked := v.link.cells(y)
		styles := map[uint16]resolved{}
		styleOf := func(c vt.Cell, x int) resolved {
			if c.Style == 0 {
				return def
			}
			if r, ok := styles[c.Style]; ok {
				return r
			}
			r := f.resolve(rs.CellStyle(x))
			styles[c.Style] = r
			return r
		}
		// Backgrounds, then the selection, then the text over them.
		var runs []run
		var cur0 *run
		type deco struct {
			x0, x1, kind int
			c            core.Color
		}
		var decos []deco
		addDeco := func(x0, x1, kind int, c core.Color) {
			if n := len(decos); n > 0 && decos[n-1].x1 == x0 && decos[n-1].kind == kind && decos[n-1].c == c {
				decos[n-1].x1 = x1
				return
			}
			decos = append(decos, deco{x0, x1, kind, c})
		}
		spanX, spanEnd, spanC := 0, 0, core.Color(0)
		flushSpan := func() {
			if spanEnd > spanX {
				d.SetFillColor(spanC).FillRect(ox+float32(spanX)*cw, ry, float32(spanEnd-spanX)*cw, ch)
			}
			spanEnd = spanX
		}
		for x := 0; x < len(cells) && x < cols; x++ {
			c := cells[x]
			if c.Wide == vt.SpacerTail || c.Wide == vt.SpacerHead {
				continue
			}
			w := 1
			if c.Wide == vt.WideChar {
				w = 2
			}
			st := styleOf(c, x)
			if c.HasBackground {
				b := c.Background
				if c.Palette {
					b = f.colors.Palette[c.Background.R]
				}
				st.bg, st.hasBg = color(b), true
			}
			if st.hasBg && st.bg != bg {
				if spanEnd == x && spanC == st.bg {
					spanEnd = x + w
				} else {
					flushSpan()
					spanX, spanEnd, spanC = x, x+w, st.bg
				}
			}
			if sel && x >= selA && x <= selB && f.th.SelectionText != 0 {
				st.fg = f.th.SelectionText
			}
			if st.underline != 0 {
				ul := st.fg
				if st.hasUl {
					ul = st.ul
				}
				addDeco(x, x+w, st.underline, ul)
			} else if linked && x >= linkA && x <= linkB {
				ul := st.fg
				if !v.armed {
					ul = ul.Faded(0.5)
				}
				addDeco(x, x+w, vt.UnderlineSingle, ul)
			}
			if st.strike {
				addDeco(x, x+w, decoStrike, st.fg)
			}
			if st.overline {
				addDeco(x, x+w, decoOverline, st.fg)
			}
			buf = buf[:0]
			if c.Codepoint != 0 && !st.invisible {
				buf = append(buf, c.Codepoint)
				if c.Grapheme {
					buf = rs.CellGraphemes(x, buf)
				}
			}
			if cur.Visible && cur.InView && cur.Y == y && cur.X == x {
				cursorRun = run{x: x, text: append([]rune(nil), buf...), bold: st.bold, italic: st.italic}
			}
			if len(buf) == 0 || buf[0] == ' ' {
				continue
			}
			// ASCII joins a run of its style; anything else, which a
			// fallback font may draw wider, stands in its own cells.
			ascii := len(buf) == 1 && buf[0] < 0x7f && w == 1
			if cur0 != nil && ascii && cur0.fg == st.fg && cur0.bold == st.bold && cur0.italic == st.italic {
				for len(cur0.text) < x-cur0.x {
					cur0.text = append(cur0.text, ' ')
				}
				cur0.text = append(cur0.text, buf[0])
				continue
			}
			runs = append(runs, run{x: x, text: append([]rune(nil), buf...), fg: st.fg, bold: st.bold, italic: st.italic})
			cur0 = nil
			if ascii {
				cur0 = &runs[len(runs)-1]
			}
		}
		flushSpan()
		if sel {
			f.anySel = true
			d.SetFillColor(selection).FillRect(ox+float32(selA)*cw, ry, float32(selB-selA+1)*cw, ch)
		}
		for _, r := range runs {
			d.SetFillColor(r.fg).FillText(string(r.text), ox+float32(r.x)*cw, ry+base, v.font(r.bold, r.italic))
		}
		for _, dc := range decos {
			x0, x1 := ox+float32(dc.x0)*cw, ox+float32(dc.x1)*cw
			d.SetFillColor(dc.c)
			switch dc.kind {
			case decoStrike:
				d.FillRect(x0, ry+base-m.Ascent*0.3, x1-x0, 1)
			case decoOverline:
				d.FillRect(x0, ry, x1-x0, 1)
			case vt.UnderlineDouble:
				d.FillRect(x0, ry+base+1, x1-x0, 1).FillRect(x0, ry+base+3, x1-x0, 1)
			default:
				d.FillRect(x0, ry+base+1, x1-x0, 1)
			}
		}
	}
	if cur.Visible && cur.InView {
		x, y := ox+float32(cur.X)*cw, oy+float32(cur.Y)*ch
		d.SetFillColor(cursorColor)
		switch {
		case !f.focused:
			d.SetStrokeColor(cursorColor).SetLineWidth(1).StrokeRect(x+0.5, y+0.5, cw-1, ch-1)
		case cur.Style == vt.CursorBar:
			d.FillRect(x, y, max(1, cw/8), ch)
		case cur.Style == vt.CursorUnderline:
			d.FillRect(x, y+ch-2, cw, 2)
		default:
			d.FillRect(x, y, cw, ch)
			if len(cursorRun.text) > 0 && cursorRun.text[0] != ' ' {
				d.SetFillColor(cursorText).FillText(string(cursorRun.text), x, y+base, v.font(cursorRun.bold, cursorRun.italic))
			}
		}
		v.cx.Post(func() { v.caret.Set(core.Point{X: core.Points(x), Y: core.Points(y)}) })
		// What an input method composes shows at the cursor, over the
		// cells it will take: the sink shows nothing.
		if text := f.composing; text != "" {
			font := v.font(false, false)
			w := d.MeasureText(text, font)
			d.SetFillColor(bg).FillRect(x, y, w, ch)
			d.SetFillColor(cursorColor).FillText(text, x, y+base, font)
			d.FillRect(x, y+ch-1, w, 1)
		}
	}
}

const (
	decoStrike   = 10
	decoOverline = 11
)

// sinkView is the input that takes typing, at the cursor, invisible.
func (v *view) sinkView() core.Node {
	// Not RGBA(0, 0), which is the unset color: the text would show.
	clear := core.RGBA(0xffffff, 0)
	return core.Input{
		Ref:            v.sinkRef,
		Label:          v.p.Label,
		AutoFocus:      v.p.AutoFocus,
		Value:          v.sink.Get,
		HideCaret:      true,
		NoAutoCorrect:  true,
		AutoCapitalize: core.AutoCapitalizeNone,
		PasteBehavior:  core.PasteReport,
		OnPaste: func(p core.PasteEvent) {
			if p.Text != "" && v.t != nil {
				v.t.Paste(p.Text)
			}
		},
		OnChange: v.sinkChanged,
		// A software keyboard's Return and Backspace reach no key binding:
		// Return submits, keeping the keyboard, and Backspace in an empty
		// sink changes nothing.
		SubmitBehavior: core.SubmitBehaviorSubmit,
		OnSubmit: func(string) {
			v.sent = ""
			v.sink.Set("")
			v.key(v.latched() + "enter")
		},
		OnKeyPress: func(key string) {
			if key == "Backspace" && v.sink.GetUntracked() == "" {
				v.key(v.latched() + "backspace")
			}
		},
		OnCompositionChange: func(c core.Composition) {
			if c.Composing {
				v.composing.Set(c.Text)
				return
			}
			v.composing.Set("")
			v.flushSink()
		},
		OnFocus: func() {
			v.focused.Set(true)
			v.focusReport(true)
			if v.p.OnFocus != nil {
				v.p.OnFocus()
			}
		},
		OnBlur: func() {
			v.focused.Set(false)
			v.focusReport(false)
			if v.p.OnBlur != nil {
				v.p.OnBlur()
			}
		},
		StyleFunc: func() core.Style {
			c := v.caret.Get()
			return core.Style{Position: core.Absolute, Left: core.Pt(c.X), Top: core.Pt(c.Y),
				Width: core.Pt(1), Height: core.Pt(core.Points(v.cellH)), Color: clear, Background: clear,
				SelectionColor: clear, FontSize: v.font(false, false).Size, Padding: core.Edges{}, BorderWidth: 0}
		},
	}
}

// sinkChanged takes what the sink holds now. Platforms report the text an
// input method commits before the composition's end, which sends it.
func (v *view) sinkChanged(text string) {
	v.sink.Set(text)
	if v.composing.GetUntracked() == "" {
		v.flushSink()
	}
}

// flushSink sends how the sink differs from what was sent, as backspaces
// and typed text, as autocorrection and input methods replace their text.
func (v *view) flushSink() {
	text, held := v.sink.GetUntracked(), v.sent
	common := 0
	for common < len(held) && common < len(text) && held[common] == text[common] {
		common++
	}
	for common > 0 && common < len(held) && !utf8.RuneStart(held[common]) {
		common--
	}
	for range utf8.RuneCountInString(held[common:]) {
		v.key("backspace")
	}
	v.typed(text[common:])
	v.sent = text
	if len(text) > 256 {
		v.sent = ""
		v.sink.Set("")
	}
}

// typed sends text typed or committed by an input method: a character as
// its key, for programs that ask for keys (the Kitty keyboard protocol),
// and more as text.
func (v *view) typed(text string) {
	if text == "" || v.t == nil {
		return
	}
	r, size := utf8.DecodeRuneInString(text)
	latched := v.latched()
	if size == len(text) {
		if k, unshifted, shift := keyOfRune(r); k != vt.KeyUnidentified {
			var mods vt.Mods
			if shift {
				mods = vt.ModShift
			}
			if latched != "" {
				// Control or Alt from the key bar: the key with them.
				_, _, m := parseKey(latched + "a")
				v.send(vt.KeyEvent{Action: vt.KeyPress, Key: k, Mods: mods | m, Text: text, Unshifted: unshifted})
				return
			}
			v.send(vt.KeyEvent{Action: vt.KeyPress, Key: k, Mods: mods, Consumed: mods, Text: text, Unshifted: unshifted})
			return
		}
	}
	t := v.t
	t.mu.Lock()
	if t.term != nil {
		t.term.ScrollToBottom()
		t.term.SetSelection(nil)
	}
	t.mu.Unlock()
	t.Send([]byte(text))
	t.redraw()
}

// key sends a bound key, as core names it.
func (v *view) key(name string) {
	k, r, mods := parseKey(name)
	var text string
	if r > ' ' || r == ' ' {
		text = string(r)
		if mods&vt.ModShift != 0 {
			text = strings.ToUpper(text)
		}
	}
	v.send(vt.KeyEvent{Action: vt.KeyPress, Key: k, Mods: mods, Text: text, Unshifted: r})
}

// send encodes a key as the terminal's modes ask and sends it, scrolling
// to the bottom and clearing the selection as Ghostty does.
func (v *view) send(k vt.KeyEvent) {
	t := v.t
	if t == nil {
		return
	}
	t.mu.Lock()
	if t.term == nil {
		t.mu.Unlock()
		return
	}
	if v.keys == nil {
		var err error
		if v.keys, err = vt.NewKeyEncoder(); err != nil {
			t.mu.Unlock()
			return
		}
	}
	v.keys.Sync(t.term, false)
	if b := v.keys.Encode(k); len(b) > 0 {
		t.in.push(append([]byte(nil), b...))
		t.term.ScrollToBottom()
		t.term.SetSelection(nil)
	}
	t.mu.Unlock()
	t.redraw()
}

// focusReport tells a program that asked (mode 1004) of the focus.
func (v *view) focusReport(gained bool) {
	t := v.t
	if t == nil {
		return
	}
	t.mu.Lock()
	if t.term != nil && t.term.Mode(vt.ModeFocusEvent) {
		t.in.push(vt.EncodeFocus(gained))
	}
	t.mu.Unlock()
	t.redraw()
}

// bindKeys binds the keys the sink must not take while the view has the
// focus: those typing no text, Control's and, but on Apple platforms,
// where Option types characters, Alt's; and copy and paste.
func (v *view) bindKeys(cx *core.Scope) {
	apple := core.Untracked(cx, cx.Env().AppleKeys)
	bind := func(name string, fn func()) {
		cx.OnKeyWith(name, core.KeyOptions{When: core.KeyAlways, Repeat: true}, fn)
	}
	sendKey := func(name string) { bind(name, func() { v.key(name) }) }
	for _, k := range namedKeys {
		for m := range 8 {
			name := k
			if m&4 != 0 {
				name = "alt+" + name
			}
			if m&2 != 0 {
				name = "ctrl+" + name
			}
			if m&1 != 0 {
				name = "shift+" + name
			}
			sendKey(name)
		}
	}
	for _, k := range textKeys {
		sendKey("ctrl+" + k)
		sendKey("ctrl+shift+" + k)
		if !apple {
			sendKey("alt+" + k)
			sendKey("alt+shift+" + k)
		}
	}
	scroll := func(name string, rows func() int) {
		bind(name, func() {
			if t := v.t; t != nil {
				t.mu.Lock()
				if t.term != nil && !t.term.AltScreen() {
					t.term.ScrollBy(rows())
				}
				t.mu.Unlock()
				t.redraw()
			}
		})
	}
	scroll("shift+pageup", func() int { return -v.rows })
	scroll("shift+pagedown", func() int { return v.rows })
	if apple {
		bind("meta+c", v.copy)
		bind("meta+a", v.selectAll)
		return
	}
	bind("ctrl+shift+c", v.copy)
	bind("ctrl+shift+a", v.selectAll)
	bind("ctrl+shift+v", v.paste)
}

// copy puts the selection on the clipboard.
func (v *view) copy() {
	t := v.t
	if t == nil {
		return
	}
	t.mu.Lock()
	var text string
	if t.term != nil {
		text, _ = t.term.SelectionText()
	}
	t.mu.Unlock()
	if text != "" {
		v.cx.SetClipboard(text)
	}
}

// selectAll selects the scrollback and the screen.
func (v *view) selectAll() {
	t := v.t
	if t == nil {
		return
	}
	t.mu.Lock()
	if t.term != nil {
		if s, ok := t.term.SelectAll(); ok {
			t.term.SetSelection(&s)
		}
	}
	t.mu.Unlock()
	t.redraw()
}

// cellAt returns the cell under a point of the scroll layer's content, and
// the point in the grid's pixels.
func (v *view) cellAt(p core.Point) (col, row int, px, py float64) {
	x := float32(p.X) - v.padX
	y := float32(float64(p.Y)-v.scrollY) - v.padY
	col = max(0, min(int(math.Floor(float64(x/v.cellW))), v.cols-1))
	row = max(0, min(int(math.Floor(float64(y/v.cellH))), v.rows-1))
	return col, row, float64(x * v.ratio), float64(y * v.ratio)
}

func (v *view) pressDown(e core.PressEvent) {
	v.sinkRef.Focus()
	t := v.t
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.redraw()
	defer t.mu.Unlock()
	if t.term == nil {
		return
	}
	col, row, px, py := v.cellAt(e.Position)
	if v.opensLinks(e.Modifiers) {
		if l := linkAt(t.term, col, row); l.url != "" {
			v.pressedLink = l.url
			return
		}
	}
	v.hovered = false
	v.findLink()
	if !v.ensureGesture() {
		return
	}
	ref, ok := t.term.CellAt(col, row)
	if !ok {
		return
	}
	v.selecting, v.lastMouse = true, e.Position
	v.extending = e.Modifiers.Shift && t.term.ExtendSelection(ref)
	if v.extending {
		v.gesture.Reset(t.term)
		return
	}
	if s, ok := v.gesture.Press(t.term, ref, px, py, uint64(time.Now().UnixNano())); ok {
		t.term.SetSelection(&s)
	} else {
		t.term.SetSelection(nil)
	}
}

func (v *view) mouseMove(e core.MouseEvent) {
	t := v.t
	if t == nil {
		return
	}
	if v.selecting {
		v.lastMouse = e.Position
		t.mu.Lock()
		v.dragTo(e.Position)
		t.mu.Unlock()
		t.redraw()
		return
	}
	col, row, _, _ := v.cellAt(e.Position)
	hover, armed := [2]int{col, row}, v.opensLinks(e.Modifiers)
	if v.hovered && hover == v.hover && armed == v.armed {
		return
	}
	draw := armed != v.armed && v.link.url != ""
	v.hovered, v.hover, v.armed = true, hover, armed
	t.mu.Lock()
	if t.term != nil && v.findLink() {
		draw = true
	}
	t.mu.Unlock()
	if draw {
		t.redraw()
	}
}

func (v *view) mouseLeave(core.MouseEvent) {
	t := v.t
	if t == nil {
		return
	}
	v.hovered = false
	t.mu.Lock()
	draw := v.findLink()
	t.mu.Unlock()
	if draw {
		t.redraw()
	}
}

// opensLinks reports whether held has the key that opens links.
func (v *view) opensLinks(held core.Modifiers) bool {
	if core.Untracked(v.cx, v.cx.Env().AppleKeys) {
		return held.Platform
	}
	return held.Control
}

// findLink finds the link under the mouse, and reports whether it changed;
// t.mu is held and the terminal is running while the mouse hovers.
func (v *view) findLink() bool {
	var l link
	if v.hovered {
		l = linkAt(v.t.term, v.hover[0], v.hover[1])
	}
	if on := l.url != "" && v.armed; on != v.linkOn.GetUntracked() {
		v.cx.Post(func() { v.linkOn.Set(on) })
	}
	if l.same(v.link) {
		return false
	}
	v.link = l
	return true
}

// dragTo extends the selection the press started to p; t.mu is held.
func (v *view) dragTo(p core.Point) {
	t := v.t
	if t.term == nil || v.gesture == nil {
		return
	}
	col, row, px, py := v.cellAt(p)
	ref, ok := t.term.CellAt(col, row)
	if !ok {
		return
	}
	if v.extending {
		t.term.ExtendSelection(ref)
		return
	}
	geo := vt.Geometry{Columns: v.cols, CellWidth: int(v.cellW * v.ratio), Height: int(float32(v.rows) * v.cellH * v.ratio)}
	if s, ok := v.gesture.Drag(t.term, ref, px, py, geo, false); ok {
		t.term.SetSelection(&s)
	}
}

func (v *view) pressUp(e core.PressEvent) {
	t := v.t
	if url := v.pressedLink; url != "" && t != nil {
		v.pressedLink = ""
		col, row, _, _ := v.cellAt(e.Position)
		t.mu.Lock()
		same := t.term != nil && linkAt(t.term, col, row).url == url
		t.mu.Unlock()
		if same {
			v.cx.OpenURL(url, nil)
		}
		return
	}
	if !v.selecting || t == nil {
		return
	}
	t.mu.Lock()
	v.selecting = false
	if t.term != nil && v.gesture != nil {
		if v.extending {
			v.extending = false
		} else {
			col, row, _, _ := v.cellAt(e.Position)
			if ref, ok := t.term.CellAt(col, row); ok {
				v.gesture.Release(t.term, &ref)
			} else {
				v.gesture.Release(t.term, nil)
			}
		}
	}
	t.mu.Unlock()
	t.redraw()
}
