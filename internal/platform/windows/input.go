//go:build windows

package windows

// Text inputs, as NSH's text_input.rs: an editor of the backend's own,
// laid out with DirectWrite and drawn with Direct2D in the input's view at
// the content box the core laid out (inside padding and border). There
// are no child windows, so the editor clips, fades and transforms with its
// view, joins the composition's layers, and whatever is drawn after it
// covers it.
//
// The view draws its decoration like any other; the editor draws the
// selection, the text or the placeholder, the IME's composition
// (underlined, at the caret) and the caret, which blinks at the system's
// rate on a window timer while a field has the focus of the active
// window. The editing itself (text, selection, caret stops, words, undo)
// is editModel (textedit.go); this file turns pointer, key, character and
// IME messages into its operations and its changes into events.
//
// Every user edit increments the field's edit count and reports
// EventChangeText with it; a value from the engine based on an older count
// is stale and dropped, so fast typing is never overwritten. Values from
// the engine report nothing. While the IME composes, a value waits for the
// composition to end.
//
// One field of the window has the focus. The window's own focus is
// followed apart from it: an inactive window keeps its focused field, with
// no caret, and gets it back when activated. The IME is enabled only while
// an editable, unmasked field has the focus (ImmAssociateContextEx), so
// typing elsewhere never opens a composition with nowhere to go. A field
// under the newest modal gives up the focus while the modal shows, and
// gets it back when the modal goes; Tab moves the focus between the fields
// within the newest modal. Return submits; the IME takes the keys it uses
// while it composes (they come as VK_PROCESSKEY), so a return that commits
// a composition submits nothing. Escape blurs the field.

import (
	"math"
	"slices"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf16"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/field"
)

var (
	imm32 = syscall.NewLazyDLL("imm32.dll")

	immAssociateContextEx    = imm32.NewProc("ImmAssociateContextEx")
	immGetContext            = imm32.NewProc("ImmGetContext")
	immReleaseContext        = imm32.NewProc("ImmReleaseContext")
	immNotifyIME             = imm32.NewProc("ImmNotifyIME")
	immSetCompositionWindow  = imm32.NewProc("ImmSetCompositionWindow")
	immSetCandidateWindow    = imm32.NewProc("ImmSetCandidateWindow")
	immGetCompositionStringW = imm32.NewProc("ImmGetCompositionStringW")

	getCaretBlinkTime          = user32.NewProc("GetCaretBlinkTime")
	isClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	trackPopupMenuEx           = user32.NewProc("TrackPopupMenuEx")
	clientToScreen             = user32.NewProc("ClientToScreen")
)

const (
	wmSetFocus            = 0x0007
	wmKillFocus           = 0x0008
	wmImeStartComposition = 0x010D
	wmImeEndComposition   = 0x010E
	wmImeComposition      = 0x010F
	wmImeSetContext       = 0x0281

	gcsCompStr        = 0x0008
	gcsCompAttr       = 0x0010
	gcsCursorPos      = 0x0080
	gcsResultStr      = 0x0800
	iscShowUIComposit = 0x80000000 // ISC_SHOWUICOMPOSITIONWINDOW
	niCompositionStr  = 0x0015
	cpsComplete       = 0x0001
	cpsCancel         = 0x0004
	iaceDefault       = 0x0010
	cfsPoint          = 0x0002
	cfsExclude        = 0x0080
	attrTargetConv    = 1 // ATTR_TARGET_CONVERTED
	attrTargetNotConv = 3 // ATTR_TARGET_NOTCONVERTED

	vkBack    = 0x08
	vkTab     = 0x09
	vkHome    = 0x24
	vkEnd     = 0x23
	vkRight   = 0x27
	vkUp      = 0x26
	vkDown    = 0x28
	vkInsert  = 0x2D
	vkDelete  = 0x2E
	mkShift   = 0x0004
	idcIBeam  = 32513
	smCxClick = 36 // SM_CXDOUBLECLK
	smCyClick = 37 // SM_CYDOUBLECLK

	spiGetCaretWidth = 0x2006
	tpmReturnCmd     = 0x0100
	tpmRightButton   = 0x0002

	// caretTimer blinks the focused field's caret.
	caretTimer = 3

	// bullet is what a masked field shows for each character, as UIKit's
	// and Android's.
	bullet = 0x2022
)

// The layout methods of IDWriteTextLayout this file adds to d2d.go's.
const (
	layoutSetUnderline     = 36
	layoutSetStrikethrough = 37
)

// selectionTint is the selection's color, iOS's system blue at 30%.
const selectionTint driver.Color = 0x007aff4d

// dragKind is what a drag that started with a press selects by.
type dragKind uint8

const (
	dragNone dragKind = iota
	// dragChar selects characters from where it was pressed.
	dragChar
	// dragWord selects whole words from the one double-clicked.
	dragWord
	// dragAll selects everything, after a triple click.
	dragAll
)

// compositionForm is COMPOSITIONFORM.
type compositionForm struct {
	style uint32
	pos   point
	area  rect
}

// candidateForm is CANDIDATEFORM.
type candidateForm struct {
	index, style uint32
	pos          point
	area         rect
}

// textInput is an input view's editor.
type textInput struct {
	w      *window
	v      *view
	model  editModel
	config driver.InputConfig
	// fs reports the edits, selections, keys and compositions, and holds
	// a value that comes while the IME composes until it ends.
	fs field.State
	// focused is set while the field has the window's focus, active while
	// the window has the keyboard, caretOn in the caret's blink.
	focused, active, caretOn bool
	// scroll is how far the text is scrolled to the left, or a multiline
	// field's up.
	scroll float32
	// What the press selecting now selects by, and the word it started on.
	drag                 dragKind
	dragStart, dragEnd   int
	width, height        float32 // the content box's size
	format, layout, hint *comObject
	// world is where the view was drawn last, for the pointer and the IME.
	world matrix
	drawn bool
}

func newTextInput(w *window, v *view) *textInput {
	in := &textInput{w: w, v: v, active: true, caretOn: true}
	in.fs = field.State{ID: v.id, Dispatch: func(ev driver.Event) {
		if !v.deleted && w.engine != nil {
			w.engine.Dispatch(ev)
		}
	}}
	return in
}

func (in *textInput) multiline() bool { return in.config.Multiline }

func (in *textInput) editable() bool { return !in.config.Readonly && !in.config.Disabled }

func (in *textInput) masked() bool { return in.config.Masked && !in.config.Multiline }

// release lets the editor's DirectWrite objects go.
func (in *textInput) release() { releaseAll(&in.layout, &in.hint, &in.format) }

// relayout drops the shown text's layout: the text changed what it shows.
func (in *textInput) relayout() { releaseAll(&in.layout) }

func (in *textInput) setSize(width, height float32) {
	if width != in.width || height != in.height {
		in.width, in.height = width, height
		releaseAll(&in.layout, &in.hint)
	}
}

// textFormat returns the format of the config's font.
func (in *textInput) textFormat() *comObject {
	if in.format == nil {
		in.format = textFormat(in.config.Style.Font)
	}
	return in.format
}

// makeLayout lays out units aligned as align, on one unwrapped line, or
// in lines wrapped at the field's width when multiline; it returns nil
// when DirectWrite fails.
func (in *textInput) makeLayout(units []uint16, align uintptr) *comObject {
	format := in.textFormat()
	if format == nil {
		return nil
	}
	n := len(units)
	if n == 0 {
		units = []uint16{0}
	}
	var layout *comObject
	if failed(dwriteFactory().callF(dwCreateTextLayout, 0b11000, uintptr(unsafe.Pointer(&units[0])), uintptr(n),
		format.ptr(), f32(1e6), f32(1e6), uintptr(unsafe.Pointer(&layout)))) {
		return nil
	}
	if in.multiline() {
		layout.call(formatSetWordWrapping, dwWrap)
		layout.callF(layoutSetMaxWidth, 1, f32(max(in.width, 1)))
	} else {
		layout.call(formatSetWordWrapping, dwNoWrap)
	}
	layout.call(formatSetTextAlignment, align)
	s := in.config.Style
	if t := typography(s.Font.Variant); t != nil && n > 0 {
		layout.callRange(layoutSetTypography, 0, 0, uint32(n), t.ptr())
	}
	if ls := float32(s.LetterSpacing); ls != 0 && n > 0 {
		if l1 := layout.query(&iidDWriteTextLayout1); l1 != nil {
			l1.callRange(layoutSetCharacterSpacing, 0b111, 0, uint32(n), f32(0), f32(ls), f32(0))
			l1.release()
		}
	}
	if s.Decoration&driver.TextDecorationUnderline != 0 && n > 0 {
		layout.callRange(layoutSetUnderline, 0, 0, uint32(n), 1)
	}
	if s.Decoration&driver.TextDecorationLineThrough != 0 && n > 0 {
		layout.callRange(layoutSetStrikethrough, 0, 0, uint32(n), 1)
	}
	return layout
}

// textLayout returns the shown text, laid out.
func (in *textInput) textLayout() *comObject {
	if in.layout == nil {
		var units []uint16
		if in.masked() {
			for range in.model.text {
				units = append(units, bullet)
			}
		} else {
			units = utf16.Encode([]rune(in.model.display()))
		}
		align := uintptr(dwAlignLeading)
		if in.multiline() {
			// Lines align in the layout; one line aligns in origin.
			align = dwAlignment(in.config.Style.Align)
		}
		in.layout = in.makeLayout(units, align)
	}
	return in.layout
}

// placeholderLayout returns the placeholder laid out in the field's
// width, trimmed with an ellipsis, or nil.
func (in *textInput) placeholderLayout() *comObject {
	if in.hint == nil && in.config.Placeholder != "" {
		in.hint = in.makeLayout(utf16.Encode([]rune(in.config.Placeholder)), dwAlignment(in.config.Style.Align))
		if in.hint != nil && !in.multiline() {
			in.hint.callF(layoutSetMaxWidth, 1, f32(max(in.width, 1)))
			var sign *comObject
			dwriteFactory().call(dwCreateEllipsisTrimmingSign, in.hint.ptr(), uintptr(unsafe.Pointer(&sign)))
			trim := trimming{granularity: dwTrimmingCharacter}
			in.hint.call(formatSetTrimming, uintptr(unsafe.Pointer(&trim)), sign.ptr())
			sign.release()
		}
	}
	return in.hint
}

func layoutMetrics(layout *comObject) textMetrics {
	var m textMetrics
	layout.call(layoutGetMetrics, uintptr(unsafe.Pointer(&m)))
	return m
}

// xAlign is where the text goes across the field: 0 left, 1 right.
func xAlign(a driver.TextAlign) float32 {
	switch a {
	case driver.TextAlignCenter:
		return 0.5
	case driver.TextAlignEnd:
		return 1
	}
	return 0
}

func dwAlignment(a driver.TextAlign) uintptr {
	switch a {
	case driver.TextAlignCenter:
		return dwAlignCenter
	case driver.TextAlignEnd:
		return dwAlignTrailing
	}
	return dwAlignLeading
}

// origin returns where the layout's origin goes in the content box: the
// line is centered vertically, aligned while it fits and scrolled when it
// does not.
func (in *textInput) origin(layout *comObject) (x, y float32) {
	m := layoutMetrics(layout)
	if in.multiline() {
		// Lines start at the top, scrolled up as far as they overflow.
		in.scroll = min(max(in.scroll, 0), max(m.height-in.height, 0))
		return 0, -in.scroll
	}
	textW := m.widthWithTrailing
	caret := caretWidth()
	y = (in.height - m.height) / 2
	if textW+caret <= in.width {
		in.scroll = 0
		x = max(min((in.width-textW)*xAlign(in.config.Style.Align), in.width-textW-caret), 0)
		return x, y
	}
	in.scroll = min(max(in.scroll, 0), textW+caret-in.width)
	return -in.scroll, y
}

// caretAt returns the caret's x, top and height at shown position pos,
// in the layout's coordinates.
func caretAt(layout *comObject, pos uint32) (x, y, h float32) {
	var m hitTestMetrics
	layout.call(layoutHitTestTextPosition, uintptr(pos), 0, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)),
		uintptr(unsafe.Pointer(&m)))
	return x, y, m.height
}

// caretRect returns the caret's x, top and height in the content box.
func (in *textInput) caretRect() (x, y, h float32, ok bool) {
	layout := in.textLayout()
	if layout == nil {
		return 0, 0, 0, false
	}
	ox, oy := in.origin(layout)
	x, y, h = caretAt(layout, in.model.displayCaret(in.masked()))
	return ox + x, oy + y, h, true
}

// hit returns the text offset at x, y in the content box.
func (in *textInput) hit(x, y float32) int {
	layout := in.textLayout()
	if layout == nil {
		return 0
	}
	ox, oy := in.origin(layout)
	var trailing, inside int32
	var m hitTestMetrics
	layout.callF(layoutHitTestPoint, 0b11, f32(x-ox), f32(y-oy), uintptr(unsafe.Pointer(&trailing)),
		uintptr(unsafe.Pointer(&inside)), uintptr(unsafe.Pointer(&m)))
	pos := m.position
	if trailing != 0 {
		pos += m.length
	}
	return in.model.textAt(pos, in.masked())
}

// lineHit returns the text offset on the caret's line moved by lines: at
// the caret's x, or the line's end or start when lines is 0.
func (in *textInput) lineHit(end bool, lines int) int {
	layout := in.textLayout()
	if layout == nil {
		return in.model.caret
	}
	ox, oy := in.origin(layout)
	x, y, h := caretAt(layout, in.model.displayCaret(in.masked()))
	switch {
	case lines != 0:
		y += float32(lines) * h
	case end:
		x = 1e6
	default:
		x = -1e6
	}
	if lines < 0 && y+h/2 < 0 {
		return 0
	}
	if m := layoutMetrics(layout); lines > 0 && y+h/2 > m.height {
		return len(in.model.text)
	}
	return in.hit(x+ox, y+h/2+oy)
}

// reveal scrolls the text so that the caret shows.
func (in *textInput) reveal() {
	layout := in.textLayout()
	if layout == nil {
		return
	}
	x, y, h := caretAt(layout, in.model.displayCaret(in.masked()))
	if in.multiline() {
		if y < in.scroll {
			in.scroll = y
		} else if y+h > in.scroll+in.height {
			in.scroll = y + h - in.height
		}
	} else if x < in.scroll {
		in.scroll = x
	} else if caret := caretWidth(); x+caret > in.scroll+in.width {
		in.scroll = x + caret - in.width
	}
	in.origin(layout) // clamps it to the text
	in.caretMoved()
}

// wakeCaret shows the caret for a whole blink: typing and moving never
// leave it hidden.
func (in *textInput) wakeCaret() {
	in.caretOn = true
	in.w.restartBlink()
}

// hitRects returns the boxes of shown units from..from+n, moved by ox, oy.
func hitRects(layout *comObject, from, n uint32, ox, oy float32) []hitTestMetrics {
	if n == 0 {
		return nil
	}
	buf := make([]hitTestMetrics, 8)
	for {
		var count uint32
		hr := layout.callF(layoutHitTestTextRange, 0b1100, uintptr(from), uintptr(n), f32(ox), f32(oy),
			uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&count)))
		if int(count) > len(buf) {
			buf = make([]hitTestMetrics, count)
			continue
		}
		if failed(hr) {
			return nil
		}
		return buf[:count]
	}
}

// draw paints the editor in its view, whose transform is set to world.
func (in *textInput) draw(p *painter, world matrix) {
	in.world, in.drawn = world, true
	c := in.config.Content
	in.setSize(float32(c.Size.Width), float32(c.Size.Height))
	if in.width <= 0 || in.height <= 0 {
		return
	}
	layout := in.textLayout()
	if layout == nil {
		return
	}
	cx, cy := float32(c.Origin.X), float32(c.Origin.Y)
	// One line may be taller than its box: it stays centered and the
	// view's bounds clip it. Several lines scroll inside the box.
	clip := rectF{cx, cy - 1e4, cx + in.width, cy + 1e4}
	if in.multiline() {
		clip.top, clip.bottom = cy, cy+in.height
	}
	p.pushClipRect(clip)
	defer p.pop()
	ox, oy := in.origin(layout)
	ox, oy = ox+cx, oy+cy
	m := &in.model
	masked := in.masked()
	color := in.config.Style.Color

	if in.focused && m.hasSelection() {
		s, e := m.selection()
		from, to := m.toDisplay(s, masked), m.toDisplay(e, masked)
		for _, r := range hitRects(layout, from, to-from, ox, oy) {
			p.fillRect(rectF{r.left, r.top, r.left + max(r.width, 4), r.top + r.height}, in.selectionColor())
		}
	}

	if m.text == "" && !m.composing() {
		if hint := in.placeholderLayout(); hint != nil {
			y := cy + (in.height-layoutMetrics(hint).height)/2
			if in.multiline() {
				y = cy
			}
			p.dc.callPoint(rtDrawTextLayout, cx, y, hint.ptr(), p.solid(in.placeholderColor()).ptr(), d2dDrawTextEnableColorFont)
		}
	} else {
		p.dc.callPoint(rtDrawTextLayout, ox, oy, layout.ptr(), p.solid(color).ptr(), d2dDrawTextEnableColorFont)
	}

	if comp := m.composition; m.composing() {
		start := m.toDisplay(m.caret, masked)
		thin := 1 / max(p.scale, 1)
		underline := func(from, n uint32, width float32) {
			for _, r := range hitRects(layout, from, n, ox, oy) {
				y := r.top + r.height - width/2 - thin
				p.fillRect(rectF{r.left + thin, y - width/2, r.left + r.width - thin, y + width/2}, color)
			}
		}
		underline(start, uint32(utf16Len(comp.text)), max(thin, 1))
		if comp.hasTarget {
			from := start + uint32(utf16Len(comp.text[:comp.targetStart]))
			underline(from, uint32(utf16Len(comp.text[comp.targetStart:comp.targetEnd])), 2)
		}
	}

	if in.focused && in.active && in.caretOn && in.editable() && !in.config.HideCaret && (!m.hasSelection() || m.composing()) {
		x, y, h := caretAt(layout, m.displayCaret(masked))
		// On whole pixels, so that the caret stays sharp.
		x = float32(math.Round(float64((ox+x)*p.scale))) / p.scale
		p.fillRect(rectF{x, oy + y, x + caretWidth(), oy + y + h}, in.caretColor())
	}
}

// local returns client point at in the content box's points, through
// the transform the view was drawn with.
func (in *textInput) local(at driver.Point) (x, y float32) {
	world := in.world
	if !in.drawn {
		b, _ := in.w.surfaceBounds(in.v)
		world = translation(float64(b.Origin.X), float64(b.Origin.Y))
	}
	lx, ly := world.invert().apply(float64(at.X), float64(at.Y))
	c := in.config.Content.Origin
	return float32(lx) - float32(c.X), float32(ly) - float32(c.Y)
}

var caretPixels = func() func() float32 {
	var width float32
	return func() float32 {
		if width == 0 {
			px := uint32(1)
			systemParametersInfoW.Call(spiGetCaretWidth, 0, uintptr(unsafe.Pointer(&px)), 0)
			width = float32(min(max(px, 1), 20))
		}
		return width
	}
}()

// caretWidth is the caret's width: the system's setting (Ease of
// Access), in points at 100%.
func caretWidth() float32 { return caretPixels() }

// --- Focus -------------------------------------------------------------------

// focusedField returns the field with the focus. One the app removed
// loses the focus quietly.
func (w *window) focusedField() *textInput {
	in := w.focus
	if in == nil || !in.v.deleted {
		return in
	}
	in.focused = false
	w.focus = nil
	w.updateIME()
	w.updateBlink()
	return nil
}

// setFocus moves the focus to in, or nowhere, reporting the blur of the
// field that had it, then the focus of in.
func (w *window) setFocus(in *textInput) {
	old := w.focusedField()
	if old == in {
		return
	}
	w.restore = nil
	if old != nil {
		w.notifyComposition(cpsCancel)
		old.focused = false
		old.drag = dragNone
		old.model.composition = nil
		old.relayout()
		old.compose()
		w.focus = nil
		if w.selecting == old {
			w.selecting = nil
		}
		w.report(old.v, driver.Event{Kind: driver.EventBlur, ID: old.v.id})
	}
	if in != nil {
		w.focus = in
		in.focused = true
		in.active = !w.inactive
		in.wakeCaret()
		in.reveal()
		w.report(in.v, driver.Event{Kind: driver.EventFocus, ID: in.v.id})
		w.uiaFocusChanged(in.v)
		// The scroll views around bring the field into view.
		w.revealView(in.v)
	}
	w.updateIME()
	w.updateBlink()
	w.placeIME()
	w.invalidate()
}

// blurAll takes the focus from whatever field has it.
func (w *window) blurAll() { w.setFocus(nil) }

// focusable reports whether in may take the focus.
func (in *textInput) focusable() bool { return !in.config.Disabled && !in.v.deleted }

// tabTo focuses in, reached with Tab: as in Windows' dialogs, its text
// is selected.
func (w *window) tabTo(in *textInput) {
	w.setFocus(in)
	if in.focused {
		in.model.selectAll()
		in.afterEdit(false)
	}
}

// tab moves the focus to the next field, or the previous one with Shift,
// in tree order within the newest modal, and reports whether there was
// one.
func (w *window) tab(back bool) bool {
	scope := w.focusScope()
	if scope == nil {
		return false
	}
	var fields []*textInput
	var collect func(v *view)
	collect = func(v *view) {
		if v.style.Hidden {
			return
		}
		if v.field != nil && v.field.focusable() {
			fields = append(fields, v.field)
		}
		for _, c := range v.children {
			collect(c)
		}
	}
	collect(scope)
	if len(fields) == 0 {
		return false
	}
	next := 0
	if back {
		next = len(fields) - 1
	}
	if i := slices.Index(fields, w.focusedField()); i >= 0 {
		if back {
			next = (i + len(fields) - 1) % len(fields)
		} else {
			next = (i + 1) % len(fields)
		}
	}
	w.tabTo(fields[next])
	return true
}

// focusScope returns where the focus may go: the newest modal shown, else
// the root.
func (w *window) focusScope() *view {
	root := w.view(w.root)
	if root == nil {
		return nil
	}
	scope := root
	var find func(v *view)
	find = func(v *view) {
		if v.style.Hidden || v.style.Opacity <= 0 {
			return
		}
		if v.kind == driver.KindModal {
			scope = v
		}
		for _, c := range v.children {
			find(c)
		}
	}
	find(root)
	return scope
}

// shown reports whether v is in the root's tree and nothing above it is
// hidden.
func (w *window) shown(v *view) bool {
	for ; v != nil; v = v.parent {
		if v.style.Hidden {
			return false
		}
		if v.id == w.root && v == w.view(w.root) {
			return true
		}
	}
	return false
}

func isAncestor(a, v *view) bool {
	for ; v != nil; v = v.parent {
		if v == a {
			return true
		}
	}
	return false
}

// checkFocus runs after each batch: a field that no longer shows loses
// the focus, one a modal covers gives it up until the modal goes, and
// then takes it back.
func (w *window) checkFocus() {
	in := w.focusedField()
	scope := w.focusScope()
	switch {
	case in != nil && !w.shown(in.v):
		w.setFocus(nil)
	case in != nil && scope != nil && !isAncestor(scope, in.v):
		w.setFocus(nil)
		w.restore = in
	case in == nil && w.restore != nil:
		r := w.restore
		if r.v.deleted || !r.focusable() {
			w.restore = nil
		} else if scope != nil && isAncestor(scope, r.v) && w.shown(r.v) {
			w.setFocus(r)
		}
	}
}

// revealView scrolls the scroll views around v so that it shows.
func (w *window) revealView(v *view) {
	b := v.frame
	for p := v.parent; p != nil; p = p.parent {
		if p.kind == driver.KindScroll {
			off := p.offset
			if p.scroll.Horizontal {
				off.X = revealAxis(off.X, p.frame.Size.Width, b.Origin.X, b.Size.Width)
			} else {
				off.Y = revealAxis(off.Y, p.frame.Size.Height, b.Origin.Y, b.Size.Height)
			}
			p.scrolling = nil
			w.scrollTo(p, off)
		}
		b.Origin.X += p.frame.Origin.X - p.offset.X
		b.Origin.Y += p.frame.Origin.Y - p.offset.Y
	}
}

// revealAxis returns the offset that shows from..from+size in a viewport
// of length at offset.
func revealAxis(offset, length, from, size driver.Points) driver.Points {
	switch {
	case from < offset:
		return from
	case from+size > offset+length:
		return min(from, from+size-length)
	}
	return offset
}

// windowFocus follows the window getting or losing the keyboard: the
// focused field keeps the focus, without a caret while the window is
// inactive.
func (w *window) windowFocus(focused bool) {
	w.inactive = !focused
	if in := w.focusedField(); in != nil {
		in.active = focused
		in.drag = dragNone
		in.caretOn = true
		w.invalidate()
	}
	if focused {
		w.updateIME()
		w.placeIME()
	}
	w.updateBlink()
}

// --- The caret's blinking ----------------------------------------------------

func blinkPeriod() (uint32, bool) {
	ms, _, _ := getCaretBlinkTime.Call()
	return uint32(ms), ms != 0 && uint32(ms) != math.MaxUint32
}

// updateBlink starts or stops the timer as the caret shows or not.
func (w *window) updateBlink() {
	wanted := w.focus != nil && !w.inactive
	if wanted && !w.blinking {
		w.restartBlink()
	} else if !wanted && w.blinking {
		killTimer.Call(w.hwnd, caretTimer)
		w.blinking = false
	}
}

// restartBlink starts the timer over, so a caret just shown stays for a
// period.
func (w *window) restartBlink() {
	ms, ok := blinkPeriod()
	if !ok || w.focus == nil || w.inactive || w.hwnd == 0 {
		return
	}
	setTimer.Call(w.hwnd, caretTimer, uintptr(ms), 0)
	w.blinking = true
}

func (w *window) blink() {
	in := w.focusedField()
	if in == nil || w.inactive {
		w.updateBlink()
		return
	}
	in.caretOn = !in.caretOn
	w.invalidate()
}

// --- The IME -----------------------------------------------------------------

// updateIME enables the IME while an editable, unmasked field has the
// focus.
func (w *window) updateIME() {
	if w.hwnd == 0 {
		return
	}
	var flags uintptr
	if in := w.focus; in != nil && in.editable() && !in.masked() {
		flags = iaceDefault
	}
	// A null context without flags detaches the IME; IACE_DEFAULT
	// restores the window's own.
	immAssociateContextEx.Call(w.hwnd, 0, flags)
}

// withContext runs fn with the window's input context.
func (w *window) withContext(fn func(himc uintptr)) {
	himc, _, _ := immGetContext.Call(w.hwnd)
	if himc == 0 {
		return
	}
	fn(himc)
	immReleaseContext.Call(w.hwnd, himc)
}

// notifyComposition tells the IME to complete or cancel what it composes.
func (w *window) notifyComposition(action uintptr) {
	if w.hwnd != 0 {
		w.withContext(func(himc uintptr) { immNotifyIME.Call(himc, niCompositionStr, action, 0) })
	}
}

// placeIME moves the IME's composition and candidate windows to the
// focused field's caret, in window pixels.
func (w *window) placeIME() {
	in := w.focusedField()
	if in == nil || w.hwnd == 0 {
		return
	}
	x, y, h, ok := in.caretRect()
	if !ok {
		return
	}
	world := in.world
	if !in.drawn {
		b, _ := w.surfaceBounds(in.v)
		world = translation(float64(b.Origin.X), float64(b.Origin.Y))
	}
	c := in.config.Content.Origin
	px := func(lx, ly float32) point {
		gx, gy := world.apply(float64(lx)+float64(c.X), float64(ly)+float64(c.Y))
		return point{int32(math.Round(gx * w.scale)), int32(math.Round(gy * w.scale))}
	}
	top, bottom := px(x, y), px(x, y+h)
	w.withContext(func(himc uintptr) {
		comp := compositionForm{style: cfsPoint, pos: top}
		// The candidates go below the caret, never over it.
		cand := candidateForm{style: cfsExclude, pos: bottom, area: rect{top.X, top.Y, top.X + 1, bottom.Y}}
		immSetCompositionWindow.Call(himc, uintptr(unsafe.Pointer(&comp)))
		immSetCandidateWindow.Call(himc, uintptr(unsafe.Pointer(&cand)))
	})
}

// compositionUnits returns a composition string of the context
// (GCS_COMPSTR, GCS_RESULTSTR, GCS_COMPATTR) as bytes.
func compositionBytes(himc, which uintptr) []byte {
	n, _, _ := immGetCompositionStringW.Call(himc, which, 0, 0)
	if int32(n) <= 0 {
		return nil
	}
	buf := make([]byte, int32(n))
	got, _, _ := immGetCompositionStringW.Call(himc, which, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return buf[:max(min(int(int32(got)), len(buf)), 0)]
}

func compositionString(himc, which uintptr) string {
	b := compositionBytes(himc, which)
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(units))
}

// readComposition returns what the IME composes: its text, caret and
// converted clause.
func readComposition(himc uintptr) *imeText {
	text := compositionString(himc, gcsCompStr)
	c := &imeText{text: text, cursor: len(text)}
	if cursor, _, _ := immGetCompositionStringW.Call(himc, gcsCursorPos, 0, 0); int32(cursor) >= 0 {
		c.cursor = utf16ToByte(text, uint32(int32(cursor)))
	}
	if s, e, ok := targetClause(compositionBytes(himc, gcsCompAttr)); ok {
		c.targetStart, c.targetEnd, c.hasTarget = utf16ToByte(text, s), utf16ToByte(text, e), true
	}
	return c
}

// targetClause returns the clause being converted, in UTF-16 units, from
// a composition's attributes.
func targetClause(attrs []byte) (start, end uint32, ok bool) {
	target := func(a byte) bool { return a == attrTargetConv || a == attrTargetNotConv }
	i := slices.IndexFunc(attrs, target)
	if i < 0 {
		return 0, 0, false
	}
	j := i
	for j < len(attrs) && target(attrs[j]) {
		j++
	}
	return uint32(i), uint32(j), true
}

// ime handles an IME message for the focused field, reporting false for
// default handling.
func (w *window) ime(message uint32, wParam, lParam uintptr) (uintptr, bool) {
	in := w.focusedField()
	if in == nil {
		return 0, false
	}
	switch message {
	case wmImeSetContext:
		// The composition is drawn in the field, not in the IME's window.
		r, _, _ := defWindowProcW.Call(w.hwnd, uintptr(message), wParam, lParam&^iscShowUIComposit)
		return r, true
	case wmImeStartComposition:
		// What is composed replaces the selection.
		if in.editable() && in.model.hasSelection() {
			in.afterEdit(in.model.delete(unitChar, false))
		}
		in.model.composition = &imeText{}
		w.placeIME()
		return 0, true
	case wmImeComposition:
		var result string
		var comp *imeText
		w.withContext(func(himc uintptr) {
			if lParam&gcsResultStr != 0 {
				result = compositionString(himc, gcsResultStr)
			}
			if lParam&gcsCompStr != 0 {
				comp = readComposition(himc)
			}
		})
		if result != "" {
			// A commit ends the composition it came from, even when the IME
			// starts the next one in the same message.
			in.model.composition = &imeText{}
			in.relayout()
			in.insert(result)
		}
		if comp == nil || comp.text == "" {
			comp = &imeText{}
		}
		in.model.composition = comp
		in.relayout()
		in.wakeCaret()
		in.reveal()
		in.compose()
		w.invalidate()
		w.placeIME()
		return 0, true
	case wmImeEndComposition:
		in.model.composition = nil
		in.relayout()
		in.reveal()
		in.compose()
		w.invalidate()
		return 0, true
	}
	return 0, false
}

// --- Edits and events ----------------------------------------------------------

// afterEdit follows a user edit (changed: the text changed) or a caret
// movement: it reports the text with a new edit count and the selection,
// and shows the caret.
func (in *textInput) afterEdit(changed bool) {
	w := in.w
	if changed {
		in.relayout()
		in.fs.Edited(in.model.text)
	}
	in.reportSelection()
	in.wakeCaret()
	in.reveal()
	w.invalidate()
	w.placeIME()
}

// reportSelection reports the selection in UTF-16 units, if it moved.
func (in *textInput) reportSelection() {
	if in.fs.Events.Has(driver.EventSelectionChange) {
		in.fs.Selected(in.selectionUnits())
	}
}

// selectionUnits is the selection in UTF-16 units.
func (in *textInput) selectionUnits() driver.TextRange {
	s, e := in.model.selection()
	return driver.TextRange{Start: driver.UTF16Units(in.model.text, s), End: driver.UTF16Units(in.model.text, e)}
}

// insert inserts typed, pasted or composed text, cut to maxLength.
func (in *textInput) insert(text string) {
	if !in.editable() {
		return
	}
	s, e := in.model.selection()
	if kept, ok, _ := in.fs.Insert(in.model.text, driver.UTF16Len(in.model.text[s:e]), text); ok {
		in.afterEdit(in.model.insert(kept))
	}
}

// selectRange is ItemSelection: start to end in UTF-16 units, clamped.
func (in *textInput) selectRange(r driver.TextRange) {
	t := in.model.text
	in.model.selectRange(driver.UTF16Offset(t, max(r.Start, 0)), driver.UTF16Offset(t, max(r.End, r.Start)))
	in.fs.SetSelection(in.selectionUnits())
	if in.focused {
		in.wakeCaret()
		in.reveal()
		in.w.placeIME()
	}
	in.w.invalidate()
}

// compose follows the IME's composition once the model has it, applying
// a value the app set while it lasted.
func (in *textInput) compose() {
	m := &in.model
	text := ""
	if m.composing() {
		text = m.composition.text
	}
	if value, apply := in.fs.Compose(m.composing(), text, m.text); apply {
		in.setText(value)
	}
}

// selectionColor, placeholderColor and caretColor are the config's, or
// else iOS's blue at 30%, the text color at half strength and the text
// color.
func (in *textInput) selectionColor() driver.Color {
	if c := in.config.SelectionColor; c != 0 {
		return c.Faded(0.3)
	}
	return selectionTint
}

func (in *textInput) placeholderColor() driver.Color {
	if c := in.config.PlaceholderColor; c != 0 {
		return c
	}
	return in.config.Style.Color.Faded(0.5)
}

func (in *textInput) caretColor() driver.Color {
	if c := in.config.SelectionColor; c != 0 {
		return c
	}
	return in.config.Style.Color
}

// configure applies an input's look and behavior.
func (in *textInput) configure(c *driver.InputConfig) {
	old := in.config
	if old.Style.Font != c.Style.Font {
		releaseAll(&in.format)
	}
	if old.Style != c.Style || old.Masked != c.Masked || old.Multiline != c.Multiline {
		in.relayout()
		in.scroll = 0
	}
	releaseAll(&in.hint)
	in.config = *c
	in.fs.Config = *c
	in.setSize(float32(c.Content.Size.Width), float32(c.Content.Size.Height))
	if in.focused {
		if c.Disabled {
			in.w.setFocus(nil)
		} else if old.Readonly != c.Readonly || old.Masked != c.Masked {
			in.w.updateIME()
		}
	}
}

// setValue applies the engine's text unless the user edited it after
// edit count; while the IME composes it waits for the composition's end.
func (in *textInput) setValue(text string, count uint32) {
	if in.fs.Value(text, count) {
		in.setText(text)
	}
}

func (in *textInput) setText(text string) {
	if in.model.text == text {
		return
	}
	// A composition the IME opened with nothing in it yet stays open (a
	// click or a return still completes it).
	open := in.model.composition != nil
	in.model.setText(text)
	if open {
		in.model.composition = &imeText{}
	}
	in.relayout()
	if in.focused {
		in.reveal()
	}
}

// --- Pointer ---------------------------------------------------------------

// countClicks returns how many presses in a row this one at lParam makes,
// by the system's double-click time and distance.
func (w *window) countClicks(lParam uintptr) int {
	t, _, _ := getMessageTime.Call()
	maxMs, _, _ := getDoubleClickTime.Call()
	cx, _, _ := getSystemMetrics.Call(smCxClick)
	cy, _, _ := getSystemMetrics.Call(smCyClick)
	p := point{int32(int16(lParam & 0xffff)), int32(int16(lParam >> 16 & 0xffff))}
	c := &w.clicks
	near := abs32(p.X-c.at.X) <= int32(cx)/2 && abs32(p.Y-c.at.Y) <= int32(cy)/2
	if near && c.count > 0 && uint32(t)-c.time <= uint32(maxMs) {
		c.count++
	} else {
		c.count = 1
	}
	c.time, c.at = uint32(t), p
	return c.count
}

// fieldAt returns the input under client point at that may take the
// pointer.
func (w *window) fieldAt(at driver.Point) *textInput {
	root := w.view(w.root)
	if root == nil {
		return nil
	}
	if v := pick(root, at); v != nil && v.field != nil && v.field.focusable() {
		return v.field
	}
	return nil
}

// fieldDown starts a press at at on the field under it, if any: the field
// takes the focus and places the caret, or selects a word on a double
// click and everything on a triple. It reports whether a field took the
// press, which then starts no press or pan.
func (w *window) fieldDown(at driver.Point, clicks int, shift bool) bool {
	in := w.fieldAt(at)
	if in == nil {
		return false
	}
	if in.model.composition != nil {
		// What is composed is kept, where it was typed.
		w.notifyComposition(cpsComplete)
	}
	w.setFocus(in)
	pos := in.hit(in.local(at))
	m := &in.model
	switch {
	case clicks <= 1:
		m.moveTo(pos, shift)
		in.drag = dragChar
	case clicks == 2 && !in.masked():
		// A double click in a password selects it all: words would tell
		// where its spaces are.
		s, e := m.wordAt(pos)
		m.selectRange(s, e)
		in.drag, in.dragStart, in.dragEnd = dragWord, s, e
	default:
		m.selectAll()
		in.drag, in.dragStart, in.dragEnd = dragAll, 0, len(m.text)
	}
	w.selecting = in
	setCapture.Call(w.hwnd)
	in.afterEdit(false)
	return true
}

// fieldMove extends the selection of the field being pressed to at.
func (w *window) fieldMove(at driver.Point) {
	in := w.selecting
	if in == nil || in.drag == dragNone {
		return
	}
	pos := in.hit(in.local(at))
	m := &in.model
	switch in.drag {
	case dragChar:
		m.moveTo(pos, true)
	default:
		ws, we := 0, len(m.text)
		if in.drag == dragWord {
			ws, we = m.wordAt(pos)
		}
		if pos < in.dragStart {
			m.selectRange(in.dragEnd, ws)
		} else {
			m.selectRange(in.dragStart, max(we, in.dragEnd))
		}
	}
	in.afterEdit(false)
}

// fieldUp ends the press selecting in a field, reporting whether there
// was one.
func (w *window) fieldUp() bool {
	in := w.selecting
	if in == nil {
		return false
	}
	in.drag = dragNone
	w.selecting = nil
	releaseCapture.Call()
	return true
}

// fieldCursor shows the I-beam over a field, reporting whether it did.
func (w *window) fieldCursor() bool {
	var at point
	getCursorPos.Call(uintptr(unsafe.Pointer(&at)))
	screenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&at)))
	if w.selecting == nil && w.fieldAt(w.point(at.X, at.Y)) == nil {
		return false
	}
	ibeam, _, _ := loadCursorW.Call(0, idcIBeam)
	setCursor.Call(ibeam)
	return true
}

// --- Keys --------------------------------------------------------------------

// fieldChar inserts a character typed (WM_CHAR) into the focused field;
// control characters are keys, handled by fieldKey.
func (w *window) fieldChar(unit uint16) {
	if utf16.IsSurrogate(rune(unit)) && unit < 0xDC00 {
		w.surrogate = unit
		return
	}
	units := []uint16{unit}
	if unit >= 0xDC00 && unit < 0xE000 {
		units = []uint16{w.surrogate, unit}
		w.surrogate = 0
	}
	in := w.focusedField()
	if in == nil {
		return
	}
	var text []rune
	for _, r := range utf16.Decode(units) {
		if !unicode.IsControl(r) {
			text = append(text, r)
		}
	}
	if len(text) > 0 {
		in.insert(string(text))
	}
}

// fieldKey handles a key down for the focused field, reporting whether it
// did; the modifiers are read from the keyboard's state.
func (w *window) fieldKey(vk uintptr) bool {
	return w.editKey(vk, keyDown(vkControl), keyDown(vkShift), keyDown(vkMenu))
}

// editKey is fieldKey with the modifiers given: the menu's and the
// context menu's edit items send their keys here.
func (w *window) editKey(vk uintptr, ctrl, shift, alt bool) bool {
	in := w.focusedField()
	if in == nil {
		return false
	}
	m := &in.model
	unit := unitChar
	if ctrl {
		unit = unitWord
	}
	moved := func(f func()) bool {
		f()
		in.afterEdit(false)
		return true
	}
	switch {
	case (vk == vkLeft || vk == vkRight) && !alt:
		// Alt+Left goes back, from a field too.
		return moved(func() { m.step(unit, vk == vkRight, shift) })
	case vk == vkHome || vk == vkEnd:
		to := 0
		if vk == vkEnd {
			to = len(m.text)
		}
		if in.multiline() && !ctrl {
			// The caret's line's start or end.
			to = in.lineHit(vk == vkEnd, 0)
		}
		return moved(func() { m.moveTo(to, shift) })
	case (vk == vkUp || vk == vkDown) && in.multiline():
		lines := 1
		if vk == vkUp {
			lines = -1
		}
		return moved(func() { m.moveTo(in.lineHit(false, lines), shift) })
	case vk == vkBack || vk == vkDelete:
		if !in.editable() {
			return true
		}
		if vk == vkBack {
			in.fs.Key("Backspace")
		}
		if vk == vkDelete && shift && !ctrl {
			// Shift+Delete cuts, as in every Windows editor.
			w.cut(in)
			return true
		}
		in.afterEdit(m.delete(unit, vk == vkDelete))
		return true
	case vk == vkReturn:
		if m.composing() {
			// The IME takes the return that commits a composition; one that
			// comes anyway only commits it.
			w.notifyComposition(cpsComplete)
			return true
		}
		// Return does what SubmitBehavior says; Shift or Alt with it break
		// a multiline field's line whatever it says.
		insert, blur := in.fs.Return(m.text, shift || alt)
		switch {
		case insert:
			in.insert("\n")
		case blur:
			w.setFocus(nil)
		}
		return true
	case ctrl && !alt:
		switch {
		case vk == 'A':
			return moved(m.selectAll)
		case vk == 'C', vk == vkInsert && !shift:
			w.copySelection(in)
		case vk == 'X':
			if in.editable() {
				w.cut(in)
			}
		case vk == 'V':
			if in.editable() {
				w.paste(in)
			}
		case vk == 'Z' && !shift:
			if in.editable() && m.undo() {
				in.afterEdit(true)
			}
		case vk == 'Y', vk == 'Z':
			if in.editable() && m.redo() {
				in.afterEdit(true)
			}
		default:
			return false
		}
		return true
	case vk == vkInsert && shift && !ctrl:
		if in.editable() {
			w.paste(in)
		}
		return true
	}
	return false
}

// copySelection copies the selection, never a password's.
func (w *window) copySelection(in *textInput) {
	if !in.masked() && in.model.hasSelection() {
		w.setClipboard(in.model.selected())
	}
}

func (w *window) cut(in *textInput) {
	if in.masked() || !in.model.hasSelection() {
		return
	}
	w.copySelection(in)
	in.afterEdit(in.model.delete(unitChar, false))
}

func (w *window) paste(in *textInput) {
	if !w.pasteEvent(in) {
		return
	}
	text := pasted(w.clipboard())
	if in.multiline() {
		text = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(w.clipboard())
	}
	if text != "" {
		in.insert(text)
	}
}

// --- The context menu ----------------------------------------------------------

// fieldMenu shows Windows' edit menu for in at client pixel x, y: the
// field takes the focus, keeping its selection, and each item is the
// field's own key.
func (w *window) fieldMenu(in *textInput, x, y int32) {
	if w.focusedField() != in {
		w.setFocus(in)
	}
	m := &in.model
	editable, masked, selected := in.editable(), in.masked(), m.hasSelection()
	s, e := m.selection()
	clip, _, _ := isClipboardFormatAvailable.Call(cfUnicodeText)
	if clip == 0 && in.fs.Events.Has(driver.EventPaste) {
		// A field that hears pastes takes images and files too.
		hdrop, _, _ := isClipboardFormatAvailable.Call(cfHDrop)
		dib, _, _ := isClipboardFormatAvailable.Call(cfDIB)
		clip = hdrop | dib
	}
	type item struct {
		label   string
		enabled bool
		vk      uintptr
		ctrl    bool
	}
	items := []item{
		{driver.MenuUndo.Label() + "\tCtrl+Z", editable && m.canUndo(), 'Z', true},
		{},
		{driver.MenuCut.Label() + "\tCtrl+X", editable && selected && !masked, 'X', true},
		{driver.MenuCopy.Label() + "\tCtrl+C", selected && !masked, 'C', true},
		{driver.MenuPaste.Label() + "\tCtrl+V", editable && clip != 0, 'V', true},
		{"Delete\tDel", editable && selected, vkDelete, false},
		{},
		{driver.MenuSelectAll.Label() + "\tCtrl+A", m.text != "" && !(s == 0 && e == len(m.text)), 'A', true},
	}
	menu, _, _ := createPopupMenu.Call()
	if menu == 0 {
		return
	}
	defer destroyMenu.Call(menu)
	for i, it := range items {
		if it.label == "" {
			appendMenuW.Call(menu, mfSeparator, 0, 0)
			continue
		}
		var flags uintptr = mfString
		if !it.enabled {
			flags |= mfGrayed
		}
		appendMenuW.Call(menu, flags, uintptr(i+1), uintptr(unsafe.Pointer(wide(it.label))))
	}
	at := point{x, y}
	clientToScreen.Call(w.hwnd, uintptr(unsafe.Pointer(&at)))
	chosen, _, _ := trackPopupMenuEx.Call(menu, tpmReturnCmd|tpmRightButton, uintptr(at.X), uintptr(at.Y), w.hwnd, 0)
	if i := int(chosen) - 1; i >= 0 && i < len(items) && w.focusedField() == in {
		w.editKey(items[i].vk, items[i].ctrl, false, false)
	}
}

// fieldContextMenu shows the edit menu of the field at client point at,
// or of the focused field at its caret when at is nil (the menu key or
// Shift+F10), and reports whether there was a field.
func (w *window) fieldContextMenu(at *driver.Point) bool {
	var in *textInput
	var x, y int32
	if at != nil {
		root := w.view(w.root)
		if root == nil {
			return false
		}
		// The innermost view listening to the menu keeps it; a field takes
		// it otherwise.
		for v := pick(root, *at); v != nil; v = v.parent {
			if v.events.Has(driver.EventContextMenu) {
				return false
			}
			if v.field != nil {
				in = v.field
				break
			}
		}
		if in == nil || !in.focusable() {
			return false
		}
		x, y = w.px(at.X), w.px(at.Y)
	} else {
		if in = w.focusedField(); in == nil {
			return false
		}
		cx, cy, h, _ := in.caretRect()
		world := in.world
		c := in.config.Content.Origin
		gx, gy := world.apply(float64(cx)+float64(c.X), float64(cy+h)+float64(c.Y))
		x, y = int32(math.Round(gx*w.scale)), int32(math.Round(gy*w.scale))
	}
	w.fieldMenu(in, x, y)
	return true
}

// invalidate has the window drawn again.
func (w *window) invalidate() { w.redrawAll() }
