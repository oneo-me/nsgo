//go:build js && wasm

// Package web is NSGo's browser backend: Go compiled to WebAssembly, one
// absolutely positioned element per view. Text is measured with the
// browser's own layout in a hidden element styled as the one that draws
// it, so measuring and drawing agree. The browser loads and draws images
// from their URIs, and alerts are dialogs drawn over the page.
//
// In WebAssembly the engine goroutine shares the page's only thread, so
// text is measured and batches are applied right where the engine asks.
package web

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall/js"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/engine"
	"github.com/oneo-me/nsgo/internal/testhost"
)

// Options configure the page.
type Options struct {
	// Mount is the CSS selector of the element the app fills; the body
	// when empty.
	Mount string
	// ID names the app's entries in localStorage.
	ID string
	// Components are the native components Native elements name.
	Components driver.NativeComponents
	// Fonts are font files under the app's resources, registered before
	// the first view.
	Fonts []string
	// Appearance is the appearance the app starts in.
	Appearance driver.Appearance
}

const fontFamily = `system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`

// monospaceFamily is the CSS of core.FontMonospace.
const monospaceFamily = `ui-monospace, SFMono-Regular, Menlo, Consolas, monospace`

// cssFamily returns the CSS font-family of a family name.
func cssFamily(family string) string {
	switch family {
	case "":
		return fontFamily
	case driver.FontMonospace:
		return monospaceFamily
	}
	return strconv.Quote(family) + ", " + fontFamily
}

// longPressDelay is how long a press is held to be a long press, in
// milliseconds.
const longPressDelay = 500

// clicks counts consecutive presses, which pointer events do not: one
// within 500 ms and 4 points of the last is its next click, as browsers
// count a mouse's.
type clicks struct {
	at   driver.Point
	last time.Time
	n    int
}

// down returns the count of a press at p, in surface points.
func (c *clicks) down(p driver.Point) int {
	now := time.Now()
	dx, dy := p.X-c.at.X, p.Y-c.at.Y
	if c.n > 0 && now.Sub(c.last) <= 500*time.Millisecond && dx*dx+dy*dy <= 16 {
		c.n++
	} else {
		c.n = 1
	}
	c.at, c.last = p, now
	return c.n
}

type surface struct {
	doc       js.Value
	host      js.Value
	root      *view
	measure   js.Value      // a hidden element text is measured in
	parked    js.Value      // a hidden, inert element removed views wait in
	measuring *preparedText // the text measure holds
	metrics   js.Value      // a canvas's 2D context, for fonts' metrics
	engine    *engine.Engine
	views     []*view // index is NodeID
	press     press
	touch     fingers
	covers    []covered // what the modals made inert, oldest first
	history   history
	// whole is set when the app fills the page; an app in an element of
	// it takes the page's keys only after a press in it, pressedIn, or
	// while the focus is in it, as another app on the page has them
	// otherwise.
	whole, pressedIn bool
	clicks           clicks
	timer            js.Value // the pending long press
	longPress        js.Func
	outside          []*view // views that report presses outside them
	alerts           []*alert
	keys             []driver.KeyBinding // the keymap
	live             js.Value            // the region announcements are read from
	watch            watch
	env              driver.Environment // as last sent
	gestures         gestures
	// canvasImages are the images canvases draw, by ImageSource.Key.
	canvasImages map[string]*canvasImage
	// stickyBound are the sticky cells whose insets follow animated
	// values.
	stickyBound map[*view]bool
	// selectionCSS is whether the ::selection rule is in the page.
	selectionCSS bool
	scrollCSS    bool
	stickyCSS    bool
	// fieldCSS is whether fieldCSS is in the page.
	fieldCSS bool
	// pointerCSS is whether pointerCSS was added to the page.
	pointerCSS bool

	animator *driver.Animator
	frame    js.Func // the animation frame callback
	ticking  bool
	// frameWanted is set while the app waits for a display frame.
	frameWanted bool
	components  driver.NativeComponents
	unknown     map[string]bool // component names warned about
	drag        *view           // the view files are dragged over
	// shading are the shader views to draw once the batch or the frame
	// is applied.
	shading []*view
	// mouse is where the mouse last moved over the host, in client
	// coordinates, while mouseIn.
	mouse   driver.Point
	mouseIn bool
}

// view is the element of one NodeID.
type view struct {
	id        driver.NodeID
	kind      driver.ViewKind
	el        js.Value // framed and styled by the view's items
	content   js.Value // where children go: el, or a scroll view's content
	inner     js.Value // a text view's text, a text field's <input> or a <canvas>
	events    driver.EventMask
	input     *webInput    // a text field's
	border    driver.Edges // the width of each side's border
	size      driver.Size  // the frame's
	scroll    driver.Size  // a scroll view's content size
	sc        *scrollState
	layout    *textLayout
	image     *image
	drawing   *driver.Drawing // a canvas's
	gl        *glShader       // a shader view's
	web       *webFrame       // a web view's
	video     *video          // a video's
	modal     driver.ModalAnimation
	presented bool // a modal was inserted once
	hover     bool // pointerenter and pointerleave are listened to
	moves     bool // pointermove is
	// keyFocused is whether the view has the keyboard focus it reported.
	keyFocused bool
	// panActivation decides when a drag on the view becomes its pan.
	panActivation driver.PanActivation
	listeners     listeners
	// A text's selection props, and whether a view is a selection area.
	selectable driver.Selectable
	selColor   driver.Color
	area       bool
	disabled   bool
	// props are the view's last props; grouped hides its children from
	// screen readers, and a11yFocus keeps it focusable for them.
	props     driver.ViewProps
	grouped   bool
	a11yFocus bool

	// style is the view's own, which its animated values draw over;
	// layoutAnim the layout animation moving it.
	style driver.ViewStyle
	// keyframes is the keyframe animation the Web Animations in running
	// play.
	keyframes  driver.KeyframeAnimation
	running    []js.Value
	animated   driver.AnimatedValues
	framed     bool
	layoutAnim js.Value

	component      driver.NativeComponent // a native view's
	componentTried bool

	// frame is the view's frame; a sticky cell's track holds it in its
	// list, stickyIn, along sticky's range.
	frame     driver.Bounds
	track     js.Value
	trackPage bool // in a list in its scroll view's viewport
	stickyIn  *view
	sticky    driver.StickyCell
}

// listeners are event listeners added until their owner goes.
type listeners []listener

type listener struct {
	target js.Value
	name   string
	fn     js.Func
}

// on listens to name on target until the listeners are released.
func (ls *listeners) on(target js.Value, name string, fn func(e js.Value)) {
	f := js.FuncOf(func(_ js.Value, args []js.Value) any {
		fn(args[0])
		return nil
	})
	*ls = append(*ls, listener{target, name, f})
	target.Call("addEventListener", name, f)
}

// release removes the listeners first, so no event queued for them can
// call a released function.
func (ls *listeners) release() {
	for _, l := range *ls {
		l.target.Call("removeEventListener", l.name, l.fn)
		l.fn.Release()
	}
	*ls = nil
}

// press is what the pointer last pressed in the page.
type press struct {
	active  bool
	pointer int
	view    *view        // the pressable view it landed in, if any
	done    bool         // the pointer went up or the press was canceled
	cancel  bool         // the press was canceled or held as a long press
	at      driver.Event // where it went down, and how
}

// Run fills the page with app. Call it from main; it does not return.
func Run(app engine.App, opts Options) {
	open(app, opts, nil)
	select {}
}

// open mounts app in the page and starts its engine; test, when set, is
// what a test host opens it with.
func open(app engine.App, opts Options, test *testOpen) *surface {
	loadResourceNames()
	registerFonts(opts.Fonts)
	doc := js.Global().Get("document")
	host := doc.Get("body")
	if opts.Mount != "" {
		host = doc.Call("querySelector", opts.Mount)
	}
	setStyle(host, map[string]string{"position": "relative", "overflow": "hidden"})
	if doc.Call("getElementById", "nsgo-base").IsNull() {
		base := doc.Call("createElement", "style")
		base.Set("id", "nsgo-base")
		base.Set("textContent", baseCSS)
		doc.Get("head").Call("appendChild", base)
	}
	s := &surface{doc: doc, host: host, components: opts.Components, unknown: map[string]bool{}}
	s.animator = driver.NewAnimator(func(e driver.Event) { s.engine.Dispatch(e) })
	// Inside the host, measured text inherits what drawn text does.
	s.measure = doc.Call("createElement", "div")
	setStyle(s.measure, textBase)
	setStyle(s.measure, map[string]string{
		"position": "absolute", "visibility": "hidden", "left": "-10000px", "top": "0",
		"width": "max-content",
	})
	host.Call("appendChild", s.measure)
	s.parked = doc.Call("createElement", "div")
	setStyle(s.parked, map[string]string{
		"position": "absolute", "visibility": "hidden", "left": "-10000px", "top": "0",
		"width": "0", "height": "0", "overflow": "hidden",
	})
	s.parked.Set("inert", true)
	setAttr(s.parked, "aria-hidden", "true")
	host.Call("appendChild", s.parked)
	s.listen()
	s.whole = opts.Mount == ""
	var ls listeners // for the page's life, as listen's
	s.listenHistory(&ls, opts.Mount == "" && test == nil)
	s.watchStart(opts.Mount == "")

	s.env = s.environment()
	s.setAppearance(opts.Appearance)
	eopts := engine.Options{
		Size: s.size(), Environment: s.env, Appearance: opts.Appearance,
		Storage: engine.NewStorage(newLocalStorage(opts.ID)),
	}
	if test != nil {
		eopts.UseTest(test.options, &test.pin)
		// The page's size, which the test sets, is the surface's.
		eopts.Size = s.size()
	}
	s.engine = engine.Start(s, app, eopts)
	// The host can change size without the window doing so.
	observer := js.Global().Get("ResizeObserver").New(js.FuncOf(func(js.Value, []js.Value) any {
		s.engine.Resize(s.size())
		return nil
	}))
	observer.Call("observe", host)
	s.watchChanges()
	return s
}

// testOpen is what a test host opens the app with: the test's options and
// the environment it pins.
type testOpen struct {
	options testhost.Options
	pin     engine.TestPin
}

func (s *surface) size() driver.Size {
	return driver.Size{
		Width:  driver.Points(s.host.Get("clientWidth").Float()),
		Height: driver.Points(s.host.Get("clientHeight").Float()),
	}
}

func (s *surface) view(id driver.NodeID) *view {
	if int(id) < len(s.views) {
		return s.views[id]
	}
	return nil
}

// send reports ev for v if v is still mounted and asked for ev's kind.
func (s *surface) send(v *view, ev driver.Event) {
	if s.view(v.id) != v || !v.events.Has(ev.Kind) {
		return
	}
	ev.ID = v.id
	if s.animator.Gesture(&ev) {
		s.applyAnimated()
	}
	s.engine.Dispatch(ev)
}

func setStyle(el js.Value, props map[string]string) {
	style := el.Get("style")
	for k, v := range props {
		style.Call("setProperty", k, v)
	}
}

// setAttr sets an attribute, or removes it when value is empty.
func setAttr(el js.Value, name, value string) {
	if value == "" {
		el.Call("removeAttribute", name)
	} else {
		el.Call("setAttribute", name, value)
	}
}

func px(p driver.Points) string { return fmt.Sprintf("%gpx", float32(p)) }

func cssColor(c driver.Color) string {
	r, g, b, a := c.Components()
	return fmt.Sprintf("rgba(%d,%d,%d,%g)", int(r*255+0.5), int(g*255+0.5), int(b*255+0.5), a)
}

// fillCSS is the CSS of a background or border color, "" for none.
func fillCSS(c driver.Color) string {
	if c == 0 {
		return ""
	}
	return cssColor(c)
}

// opacityCSS is the CSS of an opacity, "" for opaque.
func opacityCSS(o float32) string {
	if o == 1 {
		return ""
	}
	return fmt.Sprint(o)
}

func frameStyle(f driver.Bounds) map[string]string {
	return map[string]string{
		"left": px(f.Origin.X), "top": px(f.Origin.Y),
		"width": px(f.Size.Width), "height": px(f.Size.Height),
	}
}

// Text.

// textBase is how both the measuring element and drawn text break lines.
var textBase = map[string]string{"white-space": "pre-wrap", "overflow-wrap": "break-word"}

// fontStyle is the CSS of a resolved text style.
func fontStyle(t driver.TextStyle) map[string]string {
	family := cssFamily(t.Font.Family)
	italic := "normal"
	if t.Font.Italic {
		italic = "italic"
	}
	lineHeight := "normal"
	if t.LineHeight > 0 {
		lineHeight = px(t.LineHeight)
	}
	align := "start"
	switch t.Align {
	case driver.TextAlignCenter:
		align = "center"
	case driver.TextAlignEnd:
		align = "end"
	case driver.TextAlignJustify:
		align = "justify"
	}
	var lines []string
	if t.Decoration&driver.TextDecorationUnderline != 0 {
		lines = append(lines, "underline")
	}
	if t.Decoration&driver.TextDecorationLineThrough != 0 {
		lines = append(lines, "line-through")
	}
	decoration := "none"
	if len(lines) > 0 {
		decoration = strings.Join(lines, " ")
	}
	decorationColor := "currentcolor"
	if t.DecorationColor != 0 {
		decorationColor = cssColor(t.DecorationColor)
	}
	thickness, offset := "auto", "auto"
	if t.DecorationThickness > 0 {
		thickness = px(t.DecorationThickness)
	}
	if o, ok := t.UnderlineOffset.Points(); ok {
		offset = px(o)
	}
	direction := "ltr"
	if t.RTL {
		direction = "rtl"
	}
	caps, numeric := "normal", "normal"
	if t.Font.Variant&driver.FontSmallCaps != 0 {
		caps = "small-caps"
	}
	var nums []string
	for _, n := range numericVariants {
		if t.Font.Variant&n.bit != 0 {
			nums = append(nums, n.css)
		}
	}
	if len(nums) > 0 {
		numeric = strings.Join(nums, " ")
	}
	return map[string]string{
		"font-family": family, "font-size": px(t.Font.Size), "font-weight": fmt.Sprint(t.Font.Weight),
		"font-style": italic, "line-height": lineHeight, "letter-spacing": px(t.LetterSpacing),
		"color": cssColor(t.Color), "text-align": align, "text-decoration-line": decoration,
		"text-decoration-style": decorationStyles[t.DecorationStyle], "text-decoration-color": decorationColor,
		"text-decoration-thickness": thickness, "text-underline-offset": offset,
		"font-variant-caps": caps, "font-variant-numeric": numeric, "direction": direction,
	}
}

// numericVariants are the CSS of font-variant-numeric's values.
var numericVariants = [...]struct {
	bit driver.FontVariant
	css string
}{
	{driver.FontOldstyleNums, "oldstyle-nums"}, {driver.FontLiningNums, "lining-nums"},
	{driver.FontTabularNums, "tabular-nums"}, {driver.FontProportionalNums, "proportional-nums"},
}

// decorationStyles are the CSS of the decoration styles.
var decorationStyles = map[driver.TextDecorationStyle]string{
	0: "solid", driver.DecorationSolid: "solid", driver.DecorationDouble: "double", driver.DecorationDotted: "dotted",
	driver.DecorationDashed: "dashed", driver.DecorationWavy: "wavy",
}

// textShadow is the CSS of a text shadow.
func textShadow(s driver.TextShadow) string {
	if s.Color == 0 {
		return "none"
	}
	return fmt.Sprintf("%s %s %s %s", px(s.Offset.X), px(s.Offset.Y), px(s.Radius), cssColor(s.Color))
}

// richText is text with the CSS that lays it out: one style for the
// whole and one per span, and its attachments' placeholders.
type richText struct {
	text        string
	style       map[string]string
	spans       []span
	attachments []driver.TextAttachment
	// drawn is CSS over style for the element that draws the text: a
	// clipped text is measured as clamped and drawn cut by its box.
	drawn    map[string]string
	maxLines int
}

// span is a styled run of the text, from byte start to byte end.
type span struct {
	start, end int
	style      map[string]string
	link       bool
}

// preparedText is text styled for the browser to lay out.
type preparedText struct {
	s *surface
	richText
	// cut is the text, kept when it ellipsizes at its head or middle,
	// which CSS cannot: Layout cuts it, its last line's rest when it has
	// several (rest).
	cut  *driver.AttributedText
	rest bool
}

// textLayout is what the element that draws text needs; the browser lays
// it out at the size the engine measured.
type textLayout struct {
	richText
	// boxes are where the attachments' placeholders landed, in the
	// content box; shown is false for those the line limit cut off.
	boxes []attachmentBox
	s     *surface
	width driver.Points
	// carets hit-tests the lines, measured when first asked.
	carets driver.LazyCarets
}

// HitTest implements driver.TextHitTester.
func (l *textLayout) HitTest(p driver.Point) driver.Caret { return l.carets.HitTest(p) }

// CaretRect implements driver.TextHitTester.
func (l *textLayout) CaretRect(c driver.Caret) (driver.Bounds, bool) { return l.carets.CaretRect(c) }

// RangeRects implements driver.TextHitTester.
func (l *textLayout) RangeRects(start, end int, out []driver.Bounds) []driver.Bounds {
	return l.carets.RangeRects(start, end, out)
}

// caretLines lays the text out again in the measuring element at its
// width and reads each character's box with a range: its rows are the
// lines, the boxes their clusters, right to left in the scripts written
// so (neutral characters take the direction of the text).
func (l *textLayout) caretLines() driver.CaretLayout {
	p := &preparedText{s: l.s, richText: l.richText}
	m := p.hold()
	defer p.Release()
	setStyle(m, map[string]string{"width": px(l.width), "max-width": "none"})
	defer setStyle(m, map[string]string{"width": "max-content"})
	origin := m.Call("getBoundingClientRect")
	left, top := origin.Get("left").Float(), origin.Get("top").Float()
	rtl := js.Global().Call("getComputedStyle", m).Get("direction").String() == "rtl"
	r := l.s.doc.Call("createRange")
	var out driver.CaretLayout
	var bottom float64
	unit := 0 // the UTF-16 offset of the node walked
	add := func(start, end int, b js.Value, right bool) {
		x0, y0 := b.Get("left").Float()-left, b.Get("top").Float()-top
		x1, y1 := x0+b.Get("width").Float(), y0+b.Get("height").Float()
		if y1 <= y0 {
			return
		}
		if k := len(out); k == 0 || y0 >= bottom-0.5 {
			if k > 0 {
				out[k-1].End = start
			}
			out = append(out, driver.CaretLine{Start: start, RTL: rtl,
				Box: driver.Bounds{Origin: driver.Point{X: driver.Points(x0), Y: driver.Points(y0)},
					Size: driver.Size{Width: driver.Points(x1 - x0), Height: driver.Points(y1 - y0)}}})
			bottom = y1
		}
		cl := &out[len(out)-1]
		x := min(float64(cl.Box.Origin.X), x0)
		cl.Box.Size.Width = driver.Points(max(float64(cl.Box.Origin.X+cl.Box.Size.Width), x1) - x)
		cl.Box.Origin.X = driver.Points(x)
		cl.Clusters = append(cl.Clusters, driver.CaretCluster{Start: start, End: end, Left: driver.Points(x0),
			Right: driver.Points(x1), RTL: right})
		bottom = max(bottom, y1)
	}
	var walk func(n js.Value)
	walk = func(n js.Value) {
		switch n.Get("nodeType").Int() {
		case 3: // text
			u := 0
			for _, c := range n.Get("data").String() {
				units := len(utf16.Encode([]rune{c}))
				if c != '\n' && c != '\r' {
					r.Call("setStart", n, u)
					r.Call("setEnd", n, u+units)
					add(unit+u, unit+u+units, r.Call("getBoundingClientRect"), writtenRTL(c, rtl))
				}
				u += units
			}
			unit += u
		case 1: // element
			if n.Call("hasAttribute", "data-attachment").Bool() {
				add(unit, unit+1, n.Call("getBoundingClientRect"), rtl)
				unit++
				return
			}
			for c := n.Get("firstChild"); !c.IsNull(); c = c.Get("nextSibling") {
				walk(c)
			}
		}
	}
	if l.text != "" {
		walk(m)
	}
	if len(out) == 0 {
		out = append(out, driver.CaretLine{Box: driver.Bounds{Size: driver.Size{
			Height: driver.Points(origin.Get("height").Float())}}})
	}
	out[len(out)-1].End = unit
	if l.maxLines > 0 && len(out) > l.maxLines {
		out = out[:l.maxLines]
		out[len(out)-1].End = unit
	}
	return out
}

// writtenRTL says whether c runs right to left: a letter of a script
// written so, or a character of no direction in a text that is.
func writtenRTL(c rune, rtl bool) bool {
	switch {
	case unicode.In(c, unicode.Hebrew, unicode.Arabic, unicode.Syriac, unicode.Thaana, unicode.Nko):
		return true
	case unicode.IsLetter(c) || unicode.IsDigit(c):
		return false
	}
	return rtl
}

type attachmentBox struct {
	box   driver.Bounds
	shown bool
}

// Release implements core.TextLayout.
func (*textLayout) Release() {}

// Attachment implements core.AttachmentLayout.
func (l *textLayout) Attachment(i int) (driver.Bounds, bool) {
	if i < 0 || i >= len(l.boxes) {
		return driver.Bounds{}, false
	}
	return l.boxes[i].box, l.boxes[i].shown
}

// Prepare implements core.TextEngine.
func (s *surface) Prepare(t *driver.AttributedText) driver.PreparedText {
	style := fontStyle(t.Style)
	// Every property is set, so the measuring element keeps none of the
	// last text's.
	style["display"], style["-webkit-box-orient"], style["-webkit-line-clamp"], style["overflow"] =
		"block", "", "", "visible"
	style["text-shadow"] = textShadow(t.Shadow)
	var drawn map[string]string
	if t.MaxLines > 0 {
		style["display"], style["-webkit-box-orient"], style["-webkit-line-clamp"], style["overflow"] =
			"-webkit-box", "vertical", strconv.Itoa(t.MaxLines), "hidden"
		if t.Ellipsize == driver.EllipsizeClip {
			drawn = map[string]string{"display": "block", "-webkit-box-orient": "", "-webkit-line-clamp": ""}
		}
	}
	var spans []span
	at := 0
	for _, sp := range t.Spans {
		start, end := max(sp.Start, at), min(sp.End, len(t.Text))
		if start >= end {
			continue
		}
		// Alignment is the paragraph's.
		css := fontStyle(sp.Style)
		delete(css, "text-align")
		// A background is the span's inline box: from its font's ascent to
		// its descent, a rounded box on every line, changing no line.
		if bg := sp.Background; bg.Color != 0 {
			css["background-color"] = cssColor(bg.Color)
			css["border-radius"] = px(bg.Radius)
			css["box-decoration-break"], css["-webkit-box-decoration-break"] = "clone", "clone"
		}
		spans = append(spans, span{start, end, css, sp.Link})
		at = end
	}
	p := &preparedText{s: s, richText: richText{text: t.Text, style: style, spans: spans,
		attachments: t.Attachments, drawn: drawn, maxLines: t.MaxLines}}
	if t.Ellipsize == driver.EllipsizeHead || t.Ellipsize == driver.EllipsizeMiddle {
		switch {
		case t.MaxLines == 1 && !strings.ContainsAny(t.Text, "\n\r"):
			c := *t
			p.cut = &c
		case t.MaxLines > 1:
			c := *t
			p.cut, p.rest = &c, true
		}
	}
	return p
}

// verticalAligns are the CSS of the vertical aligns, which core defines
// as CSS's vertical-align of a replaced inline box against the font
// around it.
var verticalAligns = map[driver.VerticalAlign]string{
	driver.VerticalAlignTextTop:    "text-top",
	driver.VerticalAlignTextBottom: "text-bottom",
	driver.VerticalAlignMiddle:     "middle",
}

// appendText appends the text from byte start to end to el, an empty
// inline block of its size in place of each attachment's character: the
// line holds it as core.TextAttachment.Extent says, and its view is drawn
// over it.
func (r *richText) appendText(doc, el js.Value, start, end int) {
	for i, a := range r.attachments {
		if a.Offset < start || a.Offset >= end {
			continue
		}
		if a.Offset > start {
			el.Call("append", r.text[start:a.Offset])
		}
		align := verticalAligns[a.Align]
		if align == "" {
			align = "baseline"
		}
		ph := doc.Call("createElement", "span")
		ph.Call("setAttribute", "data-attachment", i)
		setStyle(ph, map[string]string{
			"display": "inline-block", "vertical-align": align,
			"width": px(a.Size.Width), "height": px(a.Size.Height),
			"user-select": "none", "-webkit-user-select": "none",
		})
		el.Call("appendChild", ph)
		start = a.Offset + len(string(driver.AttachmentRune))
	}
	if start < end {
		el.Call("append", r.text[start:end])
	}
}

// fill puts the text in el: as it is, or as a <span> per span carrying
// its byte offset, so a press can tell the character it hit. Links take
// keyboard focus when interactive.
func (r *richText) fill(doc, el js.Value, interactive bool) {
	if len(r.spans) == 0 && len(r.attachments) == 0 {
		el.Set("textContent", r.text)
		return
	}
	el.Set("textContent", "")
	at := 0
	for _, sp := range r.spans {
		if sp.start > at {
			r.appendText(doc, el, at, sp.start)
		}
		e := doc.Call("createElement", "span")
		setStyle(e, sp.style)
		r.appendText(doc, e, sp.start, sp.end)
		e.Call("setAttribute", "data-start", sp.start)
		if sp.link && interactive {
			e.Call("setAttribute", "role", "link")
			e.Call("setAttribute", "tabindex", "0")
			setStyle(e, map[string]string{"cursor": "pointer"})
		}
		el.Call("appendChild", e)
		at = sp.end
	}
	if at < len(r.text) {
		r.appendText(doc, el, at, len(r.text))
	}
}

// Measure implements core.PreparedText with the browser's layout.
func (p *preparedText) Measure(maxWidth driver.Points) driver.Size {
	m := p.hold()
	limit := "none"
	if !maxWidth.IsAuto() {
		limit = px(maxWidth)
	}
	m.Get("style").Call("setProperty", "max-width", limit)
	r := m.Call("getBoundingClientRect")
	w := r.Get("width").Float()
	if p.text == "" {
		w = 0
	}
	return driver.Size{Width: driver.Points(w), Height: driver.Points(r.Get("height").Float())}
}

// unclamped returns the width of p on one line.
func (p *preparedText) unclamped() driver.Points {
	m := p.hold()
	m.Get("style").Call("setProperty", "max-width", "none")
	return driver.Points(m.Call("getBoundingClientRect").Get("width").Float())
}

// TextLines implements core.LineLayout: the text laid out again in the
// measuring element at its width, its lines the rows its characters'
// boxes fall in, their metrics those of the font each starts in.
func (l *textLayout) TextLines() []driver.TextLine {
	if l.text == "" {
		return nil
	}
	p := &preparedText{s: l.s, richText: l.richText}
	m := p.hold()
	defer p.Release()
	setStyle(m, map[string]string{"width": px(l.width), "max-width": "none"})
	defer setStyle(m, map[string]string{"width": "max-content"})
	origin := m.Call("getBoundingClientRect")
	left, top := origin.Get("left").Float(), origin.Get("top").Float()
	r := l.s.doc.Call("createRange")
	var lines []driver.TextLine
	var bottom float64
	at := 0 // the byte offset of the node walked
	var walk func(n js.Value)
	walk = func(n js.Value) {
		switch n.Get("nodeType").Int() {
		case 3: // text
			s := n.Get("data").String()
			u := 0
			for i, c := range s {
				units := len(utf16.Encode([]rune{c}))
				r.Call("setStart", n, u)
				r.Call("setEnd", n, u+units)
				u += units
				b := r.Call("getBoundingClientRect")
				x0, y0 := b.Get("left").Float()-left, b.Get("top").Float()-top
				x1, y1 := x0+b.Get("width").Float(), y0+b.Get("height").Float()
				if y1 <= y0 {
					continue
				}
				if k := len(lines); k == 0 || y0 >= bottom-0.5 {
					font := n.Get("parentElement").Call("closest", "span, div")
					lines = append(lines, l.s.lineAt(font, at+i, x0, y0, x1-x0, y1-y0))
					bottom = y1
				} else {
					last := &lines[k-1]
					x := min(float64(last.Bounds.Origin.X), x0)
					right := max(float64(last.Bounds.Origin.X+last.Bounds.Size.Width), x1)
					last.Bounds.Origin.X, last.Bounds.Size.Width = driver.Points(x), driver.Points(right-x)
					bottom = max(bottom, y1)
				}
				lines[len(lines)-1].End = at + i + len(string(c))
			}
			at += len(s)
		case 1: // element
			if n.Call("hasAttribute", "data-attachment").Bool() {
				at += len(string(driver.AttachmentRune))
				return
			}
			for c := n.Get("firstChild"); !c.IsNull(); c = c.Get("nextSibling") {
				walk(c)
			}
		}
	}
	walk(m)
	if l.maxLines > 0 && len(lines) > l.maxLines {
		lines = lines[:l.maxLines]
		lines[len(lines)-1].End = len(l.text)
	}
	return lines
}

// lineAt starts a line at byte offset i, its first character's box
// x, y, w, h, with the metrics of el's font, from a canvas.
func (s *surface) lineAt(el js.Value, i int, x, y, w, h float64) driver.TextLine {
	if s.metrics.IsUndefined() || s.metrics.IsNull() {
		s.metrics = s.doc.Call("createElement", "canvas").Call("getContext", "2d")
	}
	cs := js.Global().Call("getComputedStyle", el)
	s.metrics.Set("font", cs.Get("fontStyle").String()+" "+cs.Get("fontWeight").String()+" "+
		cs.Get("fontSize").String()+" "+cs.Get("fontFamily").String())
	capital := s.metrics.Call("measureText", "H")
	small := s.metrics.Call("measureText", "x")
	return driver.TextLine{Start: i, End: i,
		Bounds:    driver.Bounds{Origin: driver.Point{X: driver.Points(x), Y: driver.Points(y)}, Size: driver.Size{Width: driver.Points(w), Height: driver.Points(h)}},
		Ascender:  driver.Points(capital.Get("fontBoundingBoxAscent").Float()),
		Descender: driver.Points(capital.Get("fontBoundingBoxDescent").Float()),
		CapHeight: driver.Points(capital.Get("actualBoundingBoxAscent").Float()),
		XHeight:   driver.Points(small.Get("actualBoundingBoxAscent").Float()),
	}
}

// hold puts p in the measuring element and returns it.
func (p *preparedText) hold() js.Value {
	m := p.s.measure
	if p.s.measuring != p {
		p.s.measuring = p
		setStyle(m, p.style)
		if p.text == "" {
			m.Set("textContent", "​") // an empty text keeps a line's height
		} else {
			p.fill(p.s.doc, m, false)
		}
	}
	return m
}

// Layout implements core.PreparedText. Where the text has attachments,
// it lays the text out in the measuring element at size's width, as the
// element drawing it will, and reads where their placeholders landed.
func (p *preparedText) Layout(size driver.Size) driver.TextLayout {
	// The most characters that fit around the ellipsis, as a text of
	// their own.
	fits := func(c *driver.AttributedText) bool {
		q := p.s.Prepare(c).(*preparedText)
		defer q.Release()
		return q.unclamped() <= size.Width+0.01
	}
	var cut driver.AttributedText
	switch {
	case p.cut == nil:
	case p.rest:
		// The lines before the last as they wrap, the rest cut.
		whole := *p.cut
		whole.MaxLines = 0
		q := p.s.Prepare(&whole).(*preparedText)
		lines := q.Layout(size).(*textLayout).TextLines()
		q.Release()
		if len(lines) > p.cut.MaxLines {
			cut = p.cut.CutRest(lines[p.cut.MaxLines-1].Start, fits)
		}
	case p.unclamped() > size.Width+0.01:
		cut = p.cut.CutToFit(fits)
	}
	if cut.Text != "" {
		q := p.s.Prepare(&cut).(*preparedText)
		defer q.Release()
		l := q.Layout(size).(*textLayout)
		// Attachments by the app's indices: those cut out are hidden.
		if len(p.cut.Attachments) > 0 {
			boxes := make([]attachmentBox, len(p.cut.Attachments))
			for k, o := range cut.Origin {
				if k < len(l.boxes) && o < len(boxes) {
					boxes[o] = l.boxes[k]
				}
			}
			l.boxes = boxes
		}
		return l
	}
	l := &textLayout{richText: p.richText, s: p.s, width: size.Width}
	l.carets.Set(l.caretLines)
	if len(p.attachments) == 0 {
		return l
	}
	m := p.hold()
	setStyle(m, map[string]string{"width": px(size.Width), "max-width": "none"})
	r := m.Call("getBoundingClientRect")
	left, top := r.Get("left").Float(), r.Get("top").Float()
	// A line limit hides the lines below the element's height.
	bottom := top + r.Get("height").Float()
	l.boxes = make([]attachmentBox, len(p.attachments))
	phs := m.Call("querySelectorAll", "[data-attachment]")
	for i := range phs.Length() {
		ph := phs.Index(i)
		k, err := strconv.Atoi(ph.Call("getAttribute", "data-attachment").String())
		if err != nil || k >= len(l.boxes) {
			continue
		}
		b := ph.Call("getBoundingClientRect")
		y := b.Get("top").Float()
		l.boxes[k] = attachmentBox{
			box: driver.Bounds{
				Origin: driver.Point{X: driver.Points(b.Get("left").Float() - left), Y: driver.Points(y - top)},
				Size:   p.attachments[k].Size,
			},
			shown: y+b.Get("height").Float() <= bottom+0.5,
		}
	}
	setStyle(m, map[string]string{"width": "max-content"})
	return l
}

// Release implements core.PreparedText.
func (p *preparedText) Release() {
	if p.s.measuring == p {
		p.s.measuring = nil
	}
}

// Mounting.

// Mount implements engine.Platform; the page has one thread, so it
// applies the batch right away.
func (s *surface) Mount(b *driver.MountBatch) {
	var lb *layoutBatch
	if b.LayoutAnimation != nil && !s.env.ReduceMotion {
		lb = &layoutBatch{anim: b.LayoutAnimation, created: map[driver.NodeID]bool{}}
	}
	for i := range b.Items {
		it := &b.Items[i]
		switch it.Kind {
		case driver.ItemAnimation:
			s.animator.Apply(it.Animation)
			continue
		case driver.ItemCreate:
			v := s.create(it.ID, it.View)
			if lb != nil {
				lb.created[it.ID] = true
			}
			if it.ID == b.Root {
				s.root = v
				s.host.Call("appendChild", v.el)
			}
			continue
		case driver.ItemAlert:
			s.showAlert(it.Alert)
			continue
		case driver.ItemDismissKeyboard:
			if el := s.doc.Get("activeElement"); editing(el) {
				el.Call("blur")
			}
			continue
		case driver.ItemSetClipboard:
			s.setClipboard(it.Value)
			continue
		case driver.ItemReadClipboard:
			s.readClipboard(it)
			continue
		case driver.ItemSetClipboardImage:
			if data, ok := it.Data.([]byte); ok {
				s.setClipboardImage(data)
			}
			continue
		case driver.ItemOpenURL:
			s.openURL(it.Value, it.Count)
			continue
		case driver.ItemPickImages:
			s.pickImages(*it.Pick, it.Count)
			continue
		case driver.ItemAppearance:
			s.setAppearance(it.Appearance)
			continue
		case driver.ItemQuit, driver.ItemPopupMenu:
			// No toasts; a page neither quits nor shows native menus, and
			// the core never asks it to.
			continue
		case driver.ItemImageInfo:
			s.imageInfo(it)
			continue
		case driver.ItemAnnounce:
			s.announce(it.Value)
			continue
		case driver.ItemKeymap:
			s.setKeymap(it.Keys)
			continue
		case driver.ItemTitle:
			if s.whole {
				s.doc.Set("title", it.Value)
			}
			continue
		case driver.ItemStatusBar, driver.ItemBackUnhandled, driver.ItemBackPreview, driver.ItemMenu:
			// A page has no status bar nor menu bar, and the browser's
			// back is its own.
			continue
		case driver.ItemHistory:
			s.setHistory(int(it.Count))
			continue
		case driver.ItemWindow:
			s.windowCommand(it.Window)
			continue
		case driver.ItemBlockNative:
			s.touch.blocked = it.ID != 0
			continue
		}
		v := s.view(it.ID)
		if v == nil {
			continue
		}
		switch it.Kind {
		case driver.ItemInsert:
			parent := s.view(it.Parent)
			if parent == nil {
				continue
			}
			ref := js.Null()
			if before := s.view(it.Before); before != nil && before.outer().Get("parentNode").Equal(parent.content) {
				ref = before.outer()
			}
			if el := v.outer(); canMove() && el.Get("isConnected").Bool() && parent.content.Get("isConnected").Bool() {
				// One step keeps what removing it loses: scroll offsets,
				// an iframe's page, the focus.
				parent.content.Call("moveBefore", el, ref)
			} else {
				parent.content.Call("insertBefore", el, ref)
			}
			b := driver.Edges{}
			if parent.content.Equal(parent.el) {
				b = parent.border
			}
			unborder(v.el, b)
			if parent.grouped || v.el.Call("hasAttribute", "aria-hidden").Bool() {
				hidden := parent.grouped || v.props.ElementsHidden && v.kind != driver.KindInput
				setAttr(v.el, "aria-hidden", map[bool]string{true: "true"}[hidden])
			}
			// A text's children are its attachments, which copying leaves
			// out.
			v.el.Get("classList").Call("toggle", "nsgo-attachment", parent.kind == driver.KindText)
			if v.kind == driver.KindModal && !v.presented {
				v.presented = true
				s.present(v)
				s.cover(v)
			}
			s.appear(v, lb, it.Parent)
		case driver.ItemRemove:
			if el := v.outer(); canMove() && el.Get("isConnected").Bool() {
				// Parked, not taken out of the page, so its scroll offsets and
				// an iframe's page are there when it is inserted again.
				s.parked.Call("moveBefore", el, js.Null())
			} else {
				el.Call("remove")
			}
		case driver.ItemDelete:
			// Forgotten first, so events its removal fires are not sent.
			s.views[it.ID] = nil
			if v.kind == driver.KindModal {
				defer s.uncover(v) // once the batch is in
			}
			if v.track.Truthy() {
				v.track.Call("remove")
			}
			if !s.vanish(v, lb) {
				v.el.Call("remove")
			}
			s.animator.Forget(v.id)
			if s.drag == v {
				s.drag = nil
			}
			if v.component != nil {
				v.component.Close()
			}
			if v.layout != nil {
				v.layout.Release()
			}
			if v.image != nil {
				v.image.release()
			}
			if v.gl != nil {
				v.gl.release()
			}
			if v.web != nil {
				v.web.closePort()
			}
			if v.video != nil {
				v.video.release()
			}
			s.setOutside(v, false)
			v.listeners.release()
			v.stopKeyframes()
			if v.input != nil {
				v.input.listeners.release()
			}
		case driver.ItemFrame:
			v.frame = it.Frame
			if v.track.Truthy() {
				s.placeTrack(v)
			} else {
				s.moveFrame(v, it.Frame, lb)
				setStyle(v.el, frameStyle(it.Frame))
			}
			v.framed = true
			if v.size != it.Frame.Size {
				v.size = it.Frame.Size
				if v.image != nil {
					v.image.draw(v.size)
				}
				if v.kind == driver.KindCanvas {
					s.paint(v)
				}
				if v.kind == driver.KindShader {
					s.shadeLater(v)
				}
				if v.kind == driver.KindScroll {
					v.placeContent() // centering and snap points follow the size
				}
				s.redrawGradients(v)
			}
		case driver.ItemViewStyle:
			v.style = *it.ViewStyle
			s.setViewStyle(v, it.ViewStyle)
			if v.animated.Set != 0 {
				s.animator.Touch(v.id)
			}
		case driver.ItemNative:
			s.configureNative(v, it.Native)
		case driver.ItemNativeCommand:
			if v.component != nil {
				v.component.Command(it.Value, it.Data)
			}
		case driver.ItemText:
			s.setText(v, it.Text)
		case driver.ItemInput:
			s.setInput(v, it.Input)
		case driver.ItemImage:
			s.setImage(v, it.Image)
		case driver.ItemWeb:
			s.setWeb(v, it.Web)
		case driver.ItemWebCommand:
			s.webCommand(v, it.WebCommand, it.Value)
		case driver.ItemVideo:
			if v.video != nil {
				s.setVideo(v, it.Video)
			}
		case driver.ItemVideoSeek:
			if v.video != nil {
				v.video.seek(it.Time)
			}
		case driver.ItemModal:
			v.modal = it.Modal.Animation
		case driver.ItemCanvas:
			if v.kind == driver.KindCanvas {
				s.setDrawing(v, it.Drawing)
			}
		case driver.ItemShader:
			if v.gl != nil {
				s.setShader(v, it.Shader)
			}
		case driver.ItemScrollTo:
			if v.kind == driver.KindScroll {
				// The browser clamps the offset and reports the scroll.
				v.sc.applied++
				s.interruptMomentum(v)
				v.scrollTo(it.Point, it.Animated)
			}
		case driver.ItemScrollBy:
			if v.kind == driver.KindScroll {
				v.sc.applied++
				v.scrollBy(it.Point)
			}
		case driver.ItemStickyCells:
			if v.kind == driver.KindScroll {
				s.setSticky(v, it.Sticky)
			}
		case driver.ItemZoomTo:
			if v.kind == driver.KindScroll {
				s.zoomCommand(v, it.Zoom, it.Animated)
			}
		case driver.ItemInputValue:
			if v.input != nil {
				s.inputValue(v, it.Value, it.Count)
			}
		case driver.ItemSelection:
			if v.input != nil {
				s.selectInput(v, it.Selection)
			}
		case driver.ItemEvents:
			s.setEvents(v, it.Events)
		case driver.ItemAccessibilityLabel:
			v.props.Label = it.Value
			el := v.el
			if v.kind == driver.KindInput {
				el = v.inner
			}
			setAttr(el, "aria-label", it.Value)
		case driver.ItemAccessibilityFocus:
			v.focusAccessibility()
		case driver.ItemViewProps:
			v.disabled = it.ViewProps.Disabled
			v.props = *it.ViewProps
			setViewProps(v, it.ViewProps)
			s.setPointer(v, it.ViewProps)
			if v.input != nil {
				v.input.props = it.ViewProps
			}
			s.setSelection(v, it.ViewProps)
			if v.panActivation != it.ViewProps.PanActivation {
				v.panActivation = it.ViewProps.PanActivation
				s.setGestureStyle(v)
			}
		case driver.ItemScrollConfig:
			if v.kind == driver.KindScroll {
				s.configureScroll(v, it.Scroll)
			}
		case driver.ItemScrollContent:
			v.scroll = it.Size
			v.placeContent()
			for _, c := range v.sc.tracks {
				s.placeTrack(c)
			}
		case driver.ItemFocus:
			if v.kind == driver.KindInput {
				v.inner.Call("focus")
			}
		case driver.ItemBlur:
			if v.kind == driver.KindInput {
				v.inner.Call("blur")
			}
		}
	}
	s.applyAnimated()
	s.changedShaders()
	if s.animator.Running() || len(s.shading) > 0 {
		s.startTicking()
	}
}

// RequestFrame implements engine.Platform: the app waits for the next
// display frame. The page runs everything on one thread, so no hop is
// needed here.
func (s *surface) RequestFrame() {
	s.frameWanted = true
	s.startTicking()
}

// baseCSS places the elements the surface makes (nsgo-el) where their
// frames say, and starts views (nsgo-view) without a border or the
// browser's focus ring, so their styles only write what they draw.
const baseCSS = `.nsgo-el { position: absolute; box-sizing: border-box; margin: 0; left: 0; top: 0; }
.nsgo-view { border: 0 solid transparent; outline: none; }`

func (s *surface) element(tag string) js.Value {
	el := s.doc.Call("createElement", tag)
	el.Set("className", "nsgo-el")
	return el
}

func (s *surface) create(id driver.NodeID, kind driver.ViewKind) *view {
	for len(s.views) <= int(id) {
		s.views = append(s.views, nil)
	}
	v := &view{id: id, kind: kind, el: s.element("div")}
	v.el.Get("classList").Call("add", "nsgo-view")
	v.el.Set("nsgoID", int(id))
	v.content = v.el
	switch kind {
	case driver.KindText:
		v.inner = s.element("div")
		v.inner.Get("classList").Call("add", "nsgo-text")
		setStyle(v.inner, textBase)
		v.el.Call("appendChild", v.inner)
	case driver.KindInput:
		s.createInput(v)
	case driver.KindImage:
		s.createImage(v)
	case driver.KindWeb:
		s.createWeb(v)
	case driver.KindVideo:
		s.createVideo(v)
	case driver.KindModal:
		v.el.Call("setAttribute", "aria-modal", "true")
	case driver.KindCanvas:
		v.inner = s.element("canvas")
		setStyle(v.inner, map[string]string{"width": "100%", "height": "100%"})
		v.el.Call("appendChild", v.inner)
	case driver.KindShader:
		s.createShader(v)
	case driver.KindScroll:
		setStyle(v.el, map[string]string{"overflow": "auto"})
		v.content = s.element("div")
		v.el.Call("appendChild", v.content)
		s.newScroll(v)
	}
	s.views[id] = v
	return v
}

func (s *surface) setViewStyle(v *view, vs *driver.ViewStyle) {
	style := decorCSS(vs, v.size)
	style["background-color"] = fillCSS(vs.Background)
	style["opacity"] = opacityCSS(vs.Opacity)
	style["display"] = ""
	style["transform"] = transformCSS(vs.Transform)
	style["transition"] = transitionCSS(vs.Transition)
	if vs.Hidden {
		style["display"] = "none"
	}
	border := driver.All(vs.BorderWidth)
	colors := driver.AllColors(vs.BorderColor)
	if vs.Borders != (driver.Edges{}) {
		border, colors = vs.Borders, vs.BorderColors
	}
	for _, side := range []struct {
		name  string
		width driver.Points
		color driver.Color
	}{
		{"top", border.Top, colors.Top}, {"right", border.Right, colors.Right},
		{"bottom", border.Bottom, colors.Bottom}, {"left", border.Left, colors.Left},
	} {
		style["border-"+side.name+"-style"], style["border-"+side.name+"-width"] = "", ""
		if ls := lineStyle(vs.BorderStyle); ls != "solid" {
			style["border-"+side.name+"-style"] = ls
		}
		if side.width != 0 {
			style["border-"+side.name+"-width"] = px(side.width)
		}
		style["border-"+side.name+"-color"] = fillCSS(side.color)
	}
	if im := v.image; im != nil && im.tint != vs.TintColor {
		im.tint = vs.TintColor
		im.draw(v.size)
	}
	if border != v.border {
		v.border = border
		switch v.kind {
		case driver.KindScroll:
			v.placeContent()
		case driver.KindCanvas:
			s.paint(v)
		case driver.KindShader:
			s.shadeLater(v)
		}
		if v.kind != driver.KindScroll {
			kids := v.el.Get("children")
			for i := range kids.Length() {
				el := kids.Index(i)
				if !el.Get("nsgoID").IsUndefined() || el.Equal(v.inner) && (v.kind == driver.KindText || v.kind == driver.KindInput) {
					unborder(el, border)
				}
			}
		}
	}
	if v.kind != driver.KindScroll {
		style["overflow"] = ""
		if vs.Clip {
			style["overflow"] = clipOverflow()
		}
	}
	setStyle(v.el, style)
	s.setCursor(v)
	s.setKeyframes(v, vs.Animation)
}

// setCursor shows v's cursor, or a pointer over a pressable view.
func (s *surface) setCursor(v *view) {
	cursor := ""
	if v.style.Cursor != driver.CursorAuto {
		cursor = v.style.Cursor.Name()
	} else if v.pressable() {
		cursor = driver.CursorPointer.Name()
	}
	setStyle(v.el, map[string]string{"cursor": cursor})
}

// unborder places el, framed from its parent view's origin, where the
// frame says: CSS places it inside the parent's border b.
func unborder(el js.Value, b driver.Edges) {
	setStyle(el, map[string]string{"margin-left": px(-b.Left), "margin-top": px(-b.Top)})
}

// transformCSS is t around the view's transform origin, in its order:
// perspective, translate, rotate, rotateX, rotateY, skew, then scale.
func transformCSS(t driver.Transform) string {
	if t == (driver.Transform{}) {
		return ""
	}
	sx, sy := t.Scales()
	var b strings.Builder
	if t.Perspective > 0 {
		fmt.Fprintf(&b, "perspective(%s) ", px(t.Perspective))
	}
	fmt.Fprintf(&b, "translate(%s, %s) rotate(%gdeg)", px(t.TranslateX), px(t.TranslateY), t.Rotate)
	if !t.Flat() {
		fmt.Fprintf(&b, " rotateX(%gdeg) rotateY(%gdeg)", t.RotateX, t.RotateY)
	}
	if t.SkewX != 0 || t.SkewY != 0 {
		fmt.Fprintf(&b, " skew(%gdeg, %gdeg)", t.SkewX, t.SkewY)
	}
	fmt.Fprintf(&b, " scale(%g, %g)", sx, sy)
	return b.String()
}

// transitionProperties are the CSS properties of each transition
// property.
var transitionProperties = []struct {
	property driver.TransitionProperty
	css      []string
}{
	{driver.TransitionOpacity, []string{"opacity"}},
	{driver.TransitionBackground, []string{"background-color"}},
	{driver.TransitionTransform, []string{"transform"}},
	{driver.TransitionBorder, []string{"border-width", "border-color", "border-radius", "outline-color", "outline-width", "outline-offset"}},
	{driver.TransitionShadow, []string{"box-shadow"}},
}

// transitionCSS animates the listed properties from what the view shows
// to what the same style change sets.
func transitionCSS(t driver.Transition) string {
	if t.Duration <= 0 {
		return ""
	}
	easing := "linear"
	if c := t.Easing; c != (driver.Curve{}) {
		easing = fmt.Sprintf("cubic-bezier(%g, %g, %g, %g)", c.X1, c.Y1, c.X2, c.Y2)
	}
	timing := fmt.Sprintf(" %gms %s %gms", float64(t.Duration)/1e6, easing, float64(t.Delay)/1e6)
	var parts []string
	for _, p := range transitionProperties {
		if t.Properties == 0 || t.Properties&p.property != 0 {
			for _, name := range p.css {
				parts = append(parts, name+timing)
			}
		}
	}
	return strings.Join(parts, ", ")
}

// present animates a modal in as it is first inserted; reduced motion
// fades what would slide.
func (s *surface) present(v *view) {
	anim := v.modal
	if anim == driver.ModalSlide && s.env.ReduceMotion {
		anim = driver.ModalFade
	}
	switch anim {
	case driver.ModalSlide:
		animate(v.el, map[string]any{"transform": "translateY(100%)"}, 300, "cubic-bezier(0.2, 0.8, 0.2, 1)")
	case driver.ModalFade:
		animate(v.el, map[string]any{"opacity": 0}, 200, "ease-out")
	}
}

// animate runs el from the keyframe from to its own style. The animation
// leaves nothing behind, so the style items set stays in charge.
func animate(el js.Value, from map[string]any, ms int, easing string) {
	from["offset"] = 0
	el.Call("animate", []any{from}, map[string]any{"duration": ms, "easing": easing})
}

func (s *surface) setText(v *view, t *driver.TextContent) {
	if v.kind != driver.KindText {
		return
	}
	l, _ := t.Layout.(*textLayout)
	if v.layout != nil && v.layout != l {
		v.layout.Release()
	}
	v.layout = l
	if l == nil {
		v.inner.Set("textContent", "")
		return
	}
	setStyle(v.inner, l.style)
	if l.drawn != nil {
		setStyle(v.inner, l.drawn)
	}
	setStyle(v.inner, frameStyle(t.Content))
	l.fill(s.doc, v.inner, true)
}

// Events and accessibility.

var roles = map[driver.Role]string{
	driver.RoleButton:       "button",
	driver.RoleLink:         "link",
	driver.RoleHeader:       "heading",
	driver.RoleImage:        "img",
	driver.RoleCheckbox:     "checkbox",
	driver.RoleSwitch:       "switch",
	driver.RoleRadio:        "radio",
	driver.RoleTab:          "tab",
	driver.RoleRadioGroup:   "radiogroup",
	driver.RoleTabList:      "tablist",
	driver.RoleMenu:         "menu",
	driver.RoleMenuItem:     "menuitem",
	driver.RoleDialog:       "dialog",
	driver.RoleAlert:        "alert",
	driver.RoleAdjustable:   "slider",
	driver.RoleProgressBar:  "progressbar",
	driver.RoleTable:        "table",
	driver.RoleGrid:         "grid",
	driver.RoleRowGroup:     "rowgroup",
	driver.RoleRow:          "row",
	driver.RoleColumnHeader: "columnheader",
	driver.RoleCell:         "cell",
}

// colorSchemes are CSS's color-scheme of each appearance, inherited for
// the system's.
var colorSchemes = [...]string{driver.AppearanceLight: "light", driver.AppearanceDark: "dark"}

var liveRegions = [...]string{driver.LiveRegionPolite: "polite", driver.LiveRegionAssertive: "assertive"}

// actionKeys are the keys that perform a focused view's standard actions,
// as ARIA's sliders and dialogs take them.
var actionKeys = map[string]string{
	"ArrowUp": driver.ActionIncrement, "ArrowRight": driver.ActionIncrement,
	"ArrowDown": driver.ActionDecrement, "ArrowLeft": driver.ActionDecrement,
	"Escape": driver.ActionEscape, "Enter": driver.ActionActivate, " ": driver.ActionActivate,
}

// setViewProps maps props to ARIA. A text field's go on its <input>,
// whose role is its own.
func setViewProps(v *view, p *driver.ViewProps) {
	el := v.el
	if v.kind == driver.KindInput {
		el = v.inner
	}
	flag := func(on bool) string {
		if on {
			return "true"
		}
		return ""
	}
	checked := flag(p.Checked)
	switch p.Role {
	case driver.RoleCheckbox, driver.RoleSwitch, driver.RoleRadio:
		checked = strconv.FormatBool(p.Checked)
		if p.Indeterminate {
			checked = "mixed"
		}
	}
	role := roles[p.Role]
	grouped := p.Accessible == driver.AccessibleYes && v.kind != driver.KindText && v.kind != driver.KindInput
	if role == "" && grouped {
		role = "group"
	}
	if p.Accessible == driver.AccessibleNo && p.Role == driver.RoleNone {
		role = "presentation"
	}
	setAttr(el, "role", role)
	setAttr(el, "aria-label", p.Label)
	setAttr(el, "aria-description", p.Hint)
	setAttr(el, "aria-disabled", flag(p.Disabled))
	setAttr(el, "aria-selected", flag(p.Selected))
	setAttr(el, "aria-checked", checked)
	expanded := ""
	if p.Expandable {
		expanded = strconv.FormatBool(p.Expanded)
	}
	setAttr(el, "aria-expanded", expanded)
	var lo, hi, now string
	if val := p.Value; val.IsRange() {
		f := func(x float32) string { return strconv.FormatFloat(float64(x), 'g', -1, 32) }
		lo, hi, now = f(val.Min), f(val.Max), f(val.Now)
	}
	setAttr(el, "aria-valuemin", lo)
	setAttr(el, "aria-valuemax", hi)
	setAttr(el, "aria-valuenow", now)
	setAttr(el, "aria-valuetext", p.Value.Text)
	setAttr(el, "aria-live", liveRegions[p.LiveRegion])
	setAttr(el, "aria-hidden", flag(p.ElementsHidden))
	setAttr(el, "data-testid", p.TestID)
	v.el.Get("style").Set("colorScheme", colorSchemes[p.Appearance])
	if grouped != v.grouped {
		// A group's children are read through its label alone.
		v.grouped = grouped
		for c := v.content.Get("firstElementChild"); c.Truthy(); c = c.Get("nextElementSibling") {
			setAttr(c, "aria-hidden", flag(grouped))
		}
	}
	v.setTabIndex()
}

// pointerCSS gives pointer events and hit slop their CSS: none takes the
// subtree out, box-none the view alone, box-only its children; a slop is
// a pseudo-element around the view, whose events are the view's.
const pointerCSS = `.nsgo-pe-none, .nsgo-pe-none * { pointer-events: none !important; }
.nsgo-pe-box-none { pointer-events: none; }
:where(.nsgo-pe-box-none > *) { pointer-events: auto; }
.nsgo-pe-box-only * { pointer-events: none !important; }
.nsgo-slop::before { content: ""; position: absolute; top: var(--nsgo-slop-top); right: var(--nsgo-slop-right);
  bottom: var(--nsgo-slop-bottom); left: var(--nsgo-slop-left); }`

var pointerClasses = map[driver.PointerEvents]string{
	driver.PointerEventsNone: "nsgo-pe-none", driver.PointerEventsBoxNone: "nsgo-pe-box-none",
	driver.PointerEventsBoxOnly: "nsgo-pe-box-only",
}

// setPointer applies v's pointer events and hit slop. The browser finds
// children outside their parent itself, so no area is needed.
func (s *surface) setPointer(v *view, p *driver.ViewProps) {
	slop := p.HitSlop != (driver.Edges{})
	if !s.pointerCSS {
		if p.PointerEvents == driver.PointerEventsAuto && !slop {
			return
		}
		s.pointerCSS = true
		el := s.doc.Call("createElement", "style")
		el.Set("textContent", pointerCSS)
		s.doc.Get("head").Call("appendChild", el)
	}
	classes := v.el.Get("classList")
	for pe, name := range pointerClasses {
		classes.Call("toggle", name, pe == p.PointerEvents)
	}
	classes.Call("toggle", "nsgo-slop", slop)
	if slop {
		px := func(p driver.Points) string { return strconv.FormatFloat(float64(-p), 'f', -1, 32) + "px" }
		setStyle(v.el, map[string]string{"--nsgo-slop-top": px(p.HitSlop.Top), "--nsgo-slop-right": px(p.HitSlop.Right),
			"--nsgo-slop-bottom": px(p.HitSlop.Bottom), "--nsgo-slop-left": px(p.HitSlop.Left)})
	}
}

// setTabIndex makes pressable views and views with keyboard actions
// Tab stops, and a view the screen reader was moved to focusable.
func (v *view) setTabIndex() {
	if v.kind == driver.KindInput {
		return
	}
	tabIndex := ""
	switch {
	case v.props.ElementsHidden || v.props.Disabled:
	case v.pressable() && !v.props.NoFocus || v.events.Has(driver.EventAccessibilityAction) && v.hasKeyAction():
		tabIndex = "0"
	case v.a11yFocus:
		tabIndex = "-1"
	}
	setAttr(v.el, "tabindex", tabIndex)
	// A focused view that stops being a stop lets the focus go.
	if tabIndex == "" && v.el.Equal(v.el.Get("ownerDocument").Get("activeElement")) {
		v.el.Call("blur")
	}
}

func (v *view) hasKeyAction() bool {
	for _, a := range v.props.ActionList() {
		for _, name := range actionKeys {
			if a.Name == name {
				return true
			}
		}
	}
	return false
}

// focusAccessibility moves the focus, which screen readers follow, to v.
func (v *view) focusAccessibility() {
	el := v.el
	if v.kind == driver.KindInput {
		el = v.inner
	} else if !el.Call("hasAttribute", "tabindex").Bool() {
		v.a11yFocus = true
		v.setTabIndex()
	}
	el.Call("focus")
}

// pressable reports whether v captures presses on its descendants.
func (v *view) pressable() bool {
	m := v.events
	return m.Has(driver.EventPress) || m.Has(driver.EventLongPress) ||
		m.Has(driver.EventPressDown) || m.Has(driver.EventPressUp)
}

func (s *surface) setEvents(v *view, m driver.EventMask) {
	v.events = m
	if v.input != nil {
		v.input.fs.Events = m
	}
	s.setOutside(v, m.Has(driver.EventPressOutside))
	s.setCursor(v)
	if v.kind == driver.KindInput {
		return
	}
	// Pressable views take keyboard focus and keep the page's own text
	// selection and tap highlight out of presses.
	style := map[string]string{
		"user-select": "", "-webkit-user-select": "",
		"-webkit-touch-callout": "", "-webkit-tap-highlight-color": "",
	}
	if v.pressable() {
		style = map[string]string{
			"user-select": "none", "-webkit-user-select": "none",
			"-webkit-touch-callout": "none", "-webkit-tap-highlight-color": "transparent",
		}
	}
	setStyle(v.el, style)
	v.setTabIndex()
	s.setGestureStyle(v)
	if !v.hover && (m.Has(driver.EventHoverIn) || m.Has(driver.EventHoverOut)) {
		v.hover = true
		v.listeners.on(v.el, "pointerenter", func(js.Value) { s.send(v, driver.Event{Kind: driver.EventHoverIn}) })
		v.listeners.on(v.el, "pointerleave", func(js.Value) { s.send(v, driver.Event{Kind: driver.EventHoverOut}) })
	}
	// Moves bubble, so each view under the pointer hears them, innermost
	// first. Touches and pens never hover.
	if !v.moves && (m.Has(driver.EventHoverMove) || m.Has(driver.EventMouseMove)) {
		v.moves = true
		v.listeners.on(v.el, "pointermove", func(e js.Value) {
			if e.Get("pointerType").String() != "mouse" {
				return
			}
			if e.Get("buttons").Int() == 0 && v.events.Has(driver.EventHoverMove) {
				s.send(v, s.pointer(v, driver.EventHoverMove, e))
			}
			if v.events.Has(driver.EventMouseMove) {
				ev := s.pointer(v, driver.EventMouseMove, e)
				ev.Page, ev.Modifiers = driver.Point{}, driver.Modifiers{}
				s.send(v, ev)
			}
		})
	}
}

// listenModifiers follows the mouse over the host and reports a modifier
// key pressed or released while it rests as a hover move to where it is
// with the new modifiers, so a view hears Control come and go under a
// still mouse.
func (s *surface) listenModifiers(ls *listeners) {
	ls.on(s.host, "pointermove", func(e js.Value) {
		if e.Get("pointerType").String() == "mouse" {
			s.mouse = driver.Point{X: driver.Points(e.Get("clientX").Float()), Y: driver.Points(e.Get("clientY").Float())}
			s.mouseIn = true
		}
	})
	ls.on(s.host, "pointerleave", func(e js.Value) {
		if e.Get("pointerType").String() == "mouse" {
			s.mouseIn = false
		}
	})
	changed := func(e js.Value) {
		switch e.Get("key").String() {
		case "Control", "Alt", "Shift", "Meta":
		default:
			return
		}
		if !s.mouseIn || e.Get("repeat").Bool() {
			return
		}
		el := s.doc.Call("elementFromPoint", float64(s.mouse.X), float64(s.mouse.Y))
		for ; el.Truthy() && !el.Equal(s.host); el = el.Get("parentElement") {
			if v := s.viewOf(el); v != nil && v.el.Equal(el) && v.events.Has(driver.EventHoverMove) {
				s.send(v, s.pointerAt(v, driver.EventHoverMove, float64(s.mouse.X), float64(s.mouse.Y), e))
			}
		}
	}
	ls.on(s.doc, "keydown", changed)
	ls.on(s.doc, "keyup", changed)
}

// pointer returns where e's pointer is in v and in v's root view, the
// modal around it or the surface, and the modifier keys held.
func (s *surface) pointer(v *view, kind driver.EventKind, e js.Value) driver.Event {
	return s.pointerAt(v, kind, e.Get("clientX").Float(), e.Get("clientY").Float(), e)
}

// pointerAt is pointer for the client point (x, y), with the modifier keys
// e holds.
func (s *surface) pointerAt(v *view, kind driver.EventKind, x, y float64, e js.Value) driver.Event {
	r := v.el.Call("getBoundingClientRect")
	root := s.host
	for el := v.el; el.Truthy() && !el.Equal(s.host); el = el.Get("parentElement") {
		if m := s.viewOf(el); m != nil && m.kind == driver.KindModal {
			root = el
			break
		}
	}
	rr := root.Call("getBoundingClientRect")
	return driver.Event{Kind: kind,
		Point: driver.Point{X: driver.Points(x - r.Get("left").Float()), Y: driver.Points(y - r.Get("top").Float())},
		Page: driver.Point{X: driver.Points(x - rr.Get("left").Float() - root.Get("clientLeft").Float()),
			Y: driver.Points(y - rr.Get("top").Float() - root.Get("clientTop").Float())},
		Modifiers: driver.Modifiers{Control: e.Get("ctrlKey").Bool(), Alt: e.Get("altKey").Bool(),
			Shift: e.Get("shiftKey").Bool(), Platform: e.Get("metaKey").Bool()},
	}
}

// pressed sends kind about the press in progress, with where it began.
func (s *surface) pressed(v *view, kind driver.EventKind) {
	ev := s.press.at
	ev.Kind = kind
	s.send(v, ev)
}

// setGestureStyle keeps the browser's touch scrolling and zooming and
// its text selection out of a view's gestures.
func (s *surface) setGestureStyle(v *view) {
	if v.kind == driver.KindInput {
		return
	}
	style := map[string]string{"touch-action": v.touchAction()}
	if v.hasGestures() {
		style["user-select"], style["-webkit-user-select"], style["-webkit-touch-callout"] = "none", "none", "none"
	} else if !v.pressable() {
		style["user-select"], style["-webkit-user-select"], style["-webkit-touch-callout"] = "", "", ""
	}
	setStyle(v.el, style)
}

// windowCommand carries out what a page can of a window's commands: full
// screen and closing a window the page opened.
func (s *surface) windowCommand(c *driver.WindowCommand) {
	switch c.Action {
	case driver.WindowToggleFullScreen:
		if s.doc.Get("fullscreenElement").Truthy() || s.doc.Get("webkitFullscreenElement").Truthy() {
			call(s.doc, "exitFullscreen", "webkitExitFullscreen")
		} else {
			call(s.doc.Get("documentElement"), "requestFullscreen", "webkitRequestFullscreen")
		}
	case driver.WindowClose:
		js.Global().Call("close")
	}
}

// call calls the first of methods el has, ignoring a promise's rejection,
// such as a browser refusing full screen without a user gesture.
func call(el js.Value, methods ...string) {
	for _, m := range methods {
		if el.Get(m).Type() != js.TypeFunction {
			continue
		}
		if r := el.Call(m); r.Type() == js.TypeObject && r.Get("catch").Type() == js.TypeFunction {
			r.Call("catch", ignore)
		}
		return
	}
}

// canMove reports whether the browser moves an element in one step
// (Element.moveBefore).
var canMove = sync.OnceValue(func() bool {
	return js.Global().Get("Element").Get("prototype").Get("moveBefore").Type() == js.TypeFunction
})

// ignore is a promise rejection handler doing nothing.
var ignore = js.FuncOf(func(js.Value, []js.Value) any { return nil })

// listen follows presses on the whole page: the nearest pressable view
// takes a press, unless a text field holds it first. The document's
// listeners see presses outside views and Escape anywhere.
func (s *surface) listen() {
	var ls listeners
	ls.on(s.host, "pointerdown", s.pointerDown)
	ls.on(s.host, "pointerup", s.pointerUp)
	ls.on(s.host, "pointercancel", s.pointerUp)
	ls.on(s.host, "click", s.click)
	ls.on(s.host, "keydown", s.keyDown)
	ls.on(s.host, "contextmenu", s.contextMenu)
	ls.on(s.doc, "copy", s.copySelection)
	ls.on(s.doc, "keydown", s.escape)
	s.listenModifiers(&ls)
	s.listenGestures(&ls)
	s.listenTouches(&ls)
	s.listenFocus(&ls)
	s.listenFiles(&ls)
	// Captured, so shortcuts come before what has focus.
	s.doc.Call("addEventListener", "keydown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if s.owns() {
			s.shortcut(args[0])
		}
		return nil
	}), true)
	// Captured, so it sees presses that views stop from bubbling.
	s.doc.Call("addEventListener", "pointerdown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		s.pressedIn = s.host.Call("contains", args[0].Get("target")).Bool()
		s.pressOutside(args[0])
		return nil
	}), true)
	s.longPress = js.FuncOf(func(js.Value, []js.Value) any {
		s.timer = js.Undefined()
		if p := &s.press; p.view != nil && !p.done {
			p.cancel = true
			s.pressed(p.view, driver.EventLongPress)
		}
		return nil
	})
}

// target returns the view that takes a press on el: the nearest text
// field or pressable view holding it.
func (s *surface) target(el js.Value) *view {
	for ; el.Truthy() && !el.Equal(s.host); el = el.Get("parentElement") {
		if v := s.viewOf(el); v != nil && (v.kind == driver.KindInput || v.pressable()) {
			return v
		}
	}
	return nil
}

// viewOf returns the view whose element el is, if any.
func (s *surface) viewOf(el js.Value) *view {
	id := el.Get("nsgoID")
	if id.IsUndefined() {
		return nil
	}
	if v := s.view(driver.NodeID(id.Int())); v != nil && v.el.Equal(el) {
		return v
	}
	return nil
}

// textIndex returns the byte offset of the character of text view v at
// e's pointer, or -1 when e is not on a span.
func (s *surface) textIndex(v *view, e js.Value) int {
	sp := e.Get("target").Call("closest", "[data-start]")
	if v.kind != driver.KindText || !sp.Truthy() || !v.inner.Call("contains", sp).Bool() {
		return -1
	}
	start, _ := strconv.Atoi(sp.Call("getAttribute", "data-start").String())
	node := sp.Get("firstChild")
	text := sp.Get("textContent").String()
	x, y := e.Get("clientX").Float(), e.Get("clientY").Float()
	// The caret nearest the pointer is before or after the character.
	var off js.Value
	switch {
	case s.doc.Get("caretPositionFromPoint").Truthy():
		if p := s.doc.Call("caretPositionFromPoint", x, y); p.Truthy() && p.Get("offsetNode").Equal(node) {
			off = p.Get("offset")
		}
	case s.doc.Get("caretRangeFromPoint").Truthy():
		if r := s.doc.Call("caretRangeFromPoint", x, y); r.Truthy() && r.Get("startContainer").Equal(node) {
			off = r.Get("startOffset")
		}
	}
	if off.Type() != js.TypeNumber {
		return start
	}
	// Browsers count UTF-16 units.
	i, n := off.Int(), len(utf16.Encode([]rune(text)))
	if i >= n {
		i = n - 1
	} else if i > 0 {
		r := s.doc.Call("createRange")
		r.Call("setStart", node, i-1)
		r.Call("setEnd", node, i)
		if b := r.Call("getBoundingClientRect"); x >= b.Get("left").Float() && x <= b.Get("right").Float() {
			i--
		}
	}
	units := 0
	for b, r := range text {
		if units >= i {
			return start + b
		}
		units += utf16.RuneLen(r)
	}
	return start
}

func (s *surface) stopLongPress() {
	if s.timer.Truthy() {
		js.Global().Call("clearTimeout", s.timer)
		s.timer = js.Undefined()
	}
}

func (s *surface) pointerDown(e js.Value) {
	s.stopLongPress()
	if e.Get("button").Int() != 0 {
		s.press = press{}
		return
	}
	s.press = press{active: true, pointer: e.Get("pointerId").Int()}
	el := e.Get("target")
	v := s.target(el)
	if v == nil {
		return
	}
	if v.kind == driver.KindInput {
		// The field's padding focuses it, as an input's own would.
		if !el.Equal(v.inner) {
			e.Call("preventDefault")
			v.inner.Call("focus")
		}
		return
	}
	s.press.view = v
	s.press.at = s.pointer(v, driver.EventPressDown, e)
	s.press.at.Count = uint32(s.clicks.down(s.press.at.Page))
	s.pressed(v, driver.EventPressDown)
	if v.events.Has(driver.EventLongPress) {
		delay := longPressDelay
		if d := v.props.DelayLongPress; d > 0 {
			delay = int(d.Milliseconds())
		}
		s.timer = js.Global().Call("setTimeout", s.longPress, delay)
	}
}

func (s *surface) pointerUp(e js.Value) {
	p := &s.press
	if p.view == nil || p.done || e.Get("pointerId").Int() != p.pointer {
		return
	}
	s.stopLongPress()
	p.done = true
	if e.Get("type").String() == "pointercancel" {
		p.cancel = true
	}
	s.pressed(p.view, driver.EventPressUp)
}

// click completes a press. Clicks also come from assistive technology,
// without a pointer.
func (s *surface) click(e js.Value) {
	p := s.press
	s.press = press{}
	if s.canvasNodePress(e.Get("target")) {
		return
	}
	v := s.target(e.Get("target"))
	if v == nil || v.kind == driver.KindInput {
		return
	}
	// A pointer's click counts only on the view it went down in.
	if p.active && (p.view != v || p.cancel) {
		return
	}
	ev := driver.Event{Kind: driver.EventPress}
	if p.active {
		ev = p.at
		ev.Kind = driver.EventPress
	}
	ev.Index = s.textIndex(v, e)
	s.send(v, ev)
}

// setOutside adds v to the views that report presses outside them, or
// takes it out.
func (s *surface) setOutside(v *view, on bool) {
	i := slices.Index(s.outside, v)
	switch {
	case on && i < 0:
		s.outside = append(s.outside, v)
	case !on && i >= 0:
		s.outside = slices.Delete(s.outside, i, i+1)
	}
}

// pressOutside reports a press in the root view to each view it lands
// outside of, before anything handles the press.
func (s *surface) pressOutside(e js.Value) {
	target := e.Get("target")
	if len(s.outside) == 0 || s.root == nil || !s.root.el.Call("contains", target).Bool() {
		return
	}
	r := s.host.Call("getBoundingClientRect")
	at := driver.Point{
		X: driver.Points(e.Get("clientX").Float() - r.Get("left").Float() - s.host.Get("clientLeft").Float()),
		Y: driver.Points(e.Get("clientY").Float() - r.Get("top").Float() - s.host.Get("clientTop").Float()),
	}
	// The engine handles the events after this returns, so the views
	// reporting cannot change meanwhile.
	for _, v := range s.outside {
		if v.el.Get("isConnected").Bool() && !v.el.Call("contains", target).Bool() {
			s.send(v, driver.Event{Kind: driver.EventPressOutside, Point: at})
		}
	}
}

// escape cancels the topmost alert, or clears a text selection, or
// reports back unless a text field takes Escape. It also gives Select All
// to text selections.
func (s *surface) escape(e js.Value) {
	if !s.owns() {
		return
	}
	if len(s.alerts) == 0 && s.selectKey(e) {
		return
	}
	if e.Get("key").String() != "Escape" || e.Get("isComposing").Bool() || e.Get("defaultPrevented").Bool() {
		return
	}
	if n := len(s.alerts); n > 0 {
		e.Call("preventDefault")
		if a := s.alerts[n-1]; a.cancel >= 0 || a.cancelable {
			s.answer(a, a.cancel)
		}
		return
	}
	if !editing(s.doc.Get("activeElement")) {
		s.engine.Dispatch(driver.Event{Kind: driver.EventBack})
	}
}

// owns reports whether the page's keys are the app's: it fills the page,
// or the last press or the focus is in its element.
func (s *surface) owns() bool {
	return s.whole || s.pressedIn || s.host.Call("contains", s.doc.Get("activeElement")).Bool()
}

// editing reports whether el takes typed text.
func editing(el js.Value) bool {
	if !el.Truthy() {
		return false
	}
	switch el.Get("tagName").String() {
	case "INPUT", "TEXTAREA", "SELECT":
		return true
	}
	return el.Get("isContentEditable").Bool()
}

// contextMenu reports a secondary click to the nearest view asking for
// it, instead of the browser's menu; a text field keeps its own.
func (s *surface) contextMenu(e js.Value) {
	for el := e.Get("target"); el.Truthy() && !el.Equal(s.host); el = el.Get("parentElement") {
		v := s.viewOf(el)
		switch {
		case v == nil:
			continue
		case v.kind == driver.KindInput:
			return
		case v.events.Has(driver.EventContextMenu):
			e.Call("preventDefault")
			s.send(v, s.pointer(v, driver.EventContextMenu, e))
			return
		}
	}
}

// keyDown presses a focused pressable view with Enter or Space.
func (s *surface) keyDown(e js.Value) {
	k := e.Get("key").String()
	el := e.Get("target")
	v := s.target(el)
	if v == nil || v.kind == driver.KindInput {
		return
	}
	if name := actionKeys[k]; name != "" && v.el.Equal(el) && v.props.HasAction(name) &&
		v.events.Has(driver.EventAccessibilityAction) && !e.Get("isComposing").Bool() {
		e.Call("preventDefault")
		e.Call("stopPropagation")
		s.send(v, driver.Event{Kind: driver.EventAccessibilityAction, Text: name})
		return
	}
	if k != "Enter" && k != " " {
		return
	}
	index := -1
	switch {
	case el.Get("tagName").String() == "SPAN" && el.Call("hasAttribute", "data-start").Bool():
		// A focused link of a text.
		index, _ = strconv.Atoi(el.Call("getAttribute", "data-start").String())
	case !v.el.Equal(el):
		return
	}
	e.Call("preventDefault")
	if e.Get("repeat").Bool() {
		return // a held key presses once
	}
	s.send(v, driver.Event{Kind: driver.EventPressDown})
	s.send(v, driver.Event{Kind: driver.EventPressUp})
	s.send(v, driver.Event{Kind: driver.EventPress, Index: index})
}
