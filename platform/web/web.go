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
	"syscall/js"
	"unicode/utf16"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/engine"
)

// Options configure the page.
type Options struct {
	// Mount is the CSS selector of the element the app fills; the body
	// when empty.
	Mount string
	// ID names the app's entries in localStorage.
	ID string
	// Components are the native components Native elements name.
	Components core.NativeComponents
}

const fontFamily = `system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`

// monospaceFamily is the CSS of core.FontMonospace.
const monospaceFamily = `ui-monospace, SFMono-Regular, Menlo, Consolas, monospace`

// cssFamily returns the CSS font-family of a family name.
func cssFamily(family string) string {
	switch family {
	case "":
		return fontFamily
	case core.FontMonospace:
		return monospaceFamily
	}
	return strconv.Quote(family) + ", " + fontFamily
}

// longPressDelay is how long a press is held to be a long press, in
// milliseconds.
const longPressDelay = 500

type surface struct {
	doc       js.Value
	host      js.Value
	root      *view
	measure   js.Value      // a hidden element text is measured in
	measuring *preparedText // the text measure holds
	engine    *engine.Engine
	views     []*view // index is NodeID
	press     press
	timer     js.Value // the pending long press
	longPress js.Func
	outside   []*view // views that report presses outside them
	alerts    []*alert
	keys      map[string]bool // the keymap
	live      js.Value        // the region announcements are read from
	watch     watch
	env       core.Environment // as last sent
	gestures  gestures
	// selectionCSS is whether the ::selection rule is in the page.
	selectionCSS bool

	animator   *core.Animator
	frame      js.Func // the animation frame callback
	ticking    bool
	components core.NativeComponents
	unknown    map[string]bool // component names warned about
	drag       *view           // the view files are dragged over
}

// view is the element of one NodeID.
type view struct {
	id        core.NodeID
	kind      core.ViewKind
	el        js.Value // framed and styled by the view's items
	content   js.Value // where children go: el, or a scroll view's content
	inner     js.Value // a text view's text, a text field's <input> or a <canvas>
	events    core.EventMask
	edits     uint32     // the user's edits of a text field
	border    core.Edges // the width of each side's border
	size      core.Size  // the frame's
	scroll    core.Size  // a scroll view's content size
	layout    *textLayout
	image     *image
	drawing   *core.Drawing // a canvas's
	web       *webFrame     // a web view's
	video     *video        // a video's
	modal     core.ModalAnimation
	presented bool // a modal was inserted once
	hover     bool // pointerenter and pointerleave are listened to
	// panActivation decides when a drag on the view becomes its pan.
	panActivation core.PanActivation
	listeners     listeners
	// A text's selection props, and whether a view is a selection area.
	selectable core.Selectable
	selColor   core.Color
	area       bool
	disabled   bool

	// style is the view's own, which its animated values draw over;
	// layoutAnim the layout animation moving it.
	style      core.ViewStyle
	animated   core.AnimatedValues
	framed     bool
	layoutAnim js.Value

	component      core.NativeComponent // a native view's
	componentTried bool
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
	view    *view // the pressable view it landed in, if any
	done    bool  // the pointer went up or the press was canceled
	cancel  bool  // the press was canceled or held as a long press
}

// Run fills the page with app. Call it from main; it does not return.
func Run(app engine.App, opts Options) {
	doc := js.Global().Get("document")
	host := doc.Get("body")
	if opts.Mount != "" {
		host = doc.Call("querySelector", opts.Mount)
	}
	setStyle(host, map[string]string{"position": "relative", "overflow": "hidden"})
	s := &surface{doc: doc, host: host, components: opts.Components, unknown: map[string]bool{}}
	s.animator = core.NewAnimator(func(e core.Event) { s.engine.Dispatch(e) })
	// Inside the host, measured text inherits what drawn text does.
	s.measure = doc.Call("createElement", "div")
	setStyle(s.measure, textBase)
	setStyle(s.measure, map[string]string{
		"position": "absolute", "visibility": "hidden", "left": "-10000px", "top": "0",
		"width": "max-content",
	})
	host.Call("appendChild", s.measure)
	s.listen()
	s.watchStart(opts.Mount == "")

	s.env = s.environment()
	s.engine = engine.Start(s, app, engine.Options{
		Size: s.size(), Environment: s.env,
		Storage: core.NewStore(newLocalStorage(opts.ID)),
	})
	// The host can change size without the window doing so.
	observer := js.Global().Get("ResizeObserver").New(js.FuncOf(func(js.Value, []js.Value) any {
		s.engine.Resize(s.size())
		return nil
	}))
	observer.Call("observe", host)
	s.watchChanges()
	select {}
}

func (s *surface) size() core.Size {
	return core.Size{
		Width:  core.Points(s.host.Get("clientWidth").Float()),
		Height: core.Points(s.host.Get("clientHeight").Float()),
	}
}

func (s *surface) view(id core.NodeID) *view {
	if int(id) < len(s.views) {
		return s.views[id]
	}
	return nil
}

// send reports ev for v if v is still mounted and asked for ev's kind.
func (s *surface) send(v *view, ev core.Event) {
	if s.view(v.id) != v || !v.events.Has(ev.Kind) {
		return
	}
	ev.ID = v.id
	if ev.Kind == core.EventPan && s.animator.Pan(v.id, ev.Point) {
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

func px(p core.Points) string { return fmt.Sprintf("%gpx", float32(p)) }

func cssColor(c core.Color) string {
	r, g, b, a := c.Components()
	return fmt.Sprintf("rgba(%d,%d,%d,%g)", int(r*255+0.5), int(g*255+0.5), int(b*255+0.5), a)
}

func frameStyle(f core.Bounds) map[string]string {
	return map[string]string{
		"left": px(f.Origin.X), "top": px(f.Origin.Y),
		"width": px(f.Size.Width), "height": px(f.Size.Height),
	}
}

// Text.

// textBase is how both the measuring element and drawn text break lines.
var textBase = map[string]string{"white-space": "pre-wrap", "overflow-wrap": "break-word"}

// fontStyle is the CSS of a resolved text style.
func fontStyle(t core.TextStyle) map[string]string {
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
	case core.TextAlignCenter:
		align = "center"
	case core.TextAlignEnd:
		align = "end"
	}
	var lines []string
	if t.Decoration&core.TextDecorationUnderline != 0 {
		lines = append(lines, "underline")
	}
	if t.Decoration&core.TextDecorationLineThrough != 0 {
		lines = append(lines, "line-through")
	}
	decoration := "none"
	if len(lines) > 0 {
		decoration = strings.Join(lines, " ")
	}
	return map[string]string{
		"font-family": family, "font-size": px(t.Font.Size), "font-weight": fmt.Sprint(t.Font.Weight),
		"font-style": italic, "line-height": lineHeight, "letter-spacing": px(t.LetterSpacing),
		"color": cssColor(t.Color), "text-align": align, "text-decoration-line": decoration,
	}
}

// richText is text with the CSS that lays it out: one style for the
// whole and one per span, and its attachments' placeholders.
type richText struct {
	text        string
	style       map[string]string
	spans       []span
	attachments []core.TextAttachment
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
}

// textLayout is what the element that draws text needs; the browser lays
// it out at the size the engine measured.
type textLayout struct {
	richText
	// boxes are where the attachments' placeholders landed, in the
	// content box; shown is false for those the line limit cut off.
	boxes []attachmentBox
}

type attachmentBox struct {
	box   core.Bounds
	shown bool
}

// Release implements core.TextLayout.
func (*textLayout) Release() {}

// Attachment implements core.AttachmentLayout.
func (l *textLayout) Attachment(i int) (core.Bounds, bool) {
	if i < 0 || i >= len(l.boxes) {
		return core.Bounds{}, false
	}
	return l.boxes[i].box, l.boxes[i].shown
}

// Prepare implements core.TextEngine.
func (s *surface) Prepare(t *core.AttributedText) core.PreparedText {
	style := fontStyle(t.Style)
	// Every property is set, so the measuring element keeps none of the
	// last text's.
	style["display"], style["-webkit-box-orient"], style["-webkit-line-clamp"], style["overflow"] =
		"block", "", "", "visible"
	if t.MaxLines > 0 {
		style["display"], style["-webkit-box-orient"], style["-webkit-line-clamp"], style["overflow"] =
			"-webkit-box", "vertical", strconv.Itoa(t.MaxLines), "hidden"
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
	return &preparedText{s: s, richText: richText{text: t.Text, style: style, spans: spans,
		attachments: t.Attachments}}
}

// verticalAligns are the CSS of the vertical aligns, which core defines
// as CSS's vertical-align of a replaced inline box against the font
// around it.
var verticalAligns = map[core.VerticalAlign]string{
	core.VerticalAlignTextTop:    "text-top",
	core.VerticalAlignTextBottom: "text-bottom",
	core.VerticalAlignMiddle:     "middle",
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
		start = a.Offset + len(string(core.AttachmentRune))
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
func (p *preparedText) Measure(maxWidth core.Points) core.Size {
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
	return core.Size{Width: core.Points(w), Height: core.Points(r.Get("height").Float())}
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
func (p *preparedText) Layout(size core.Size) core.TextLayout {
	l := &textLayout{richText: p.richText}
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
			box: core.Bounds{
				Origin: core.Point{X: core.Points(b.Get("left").Float() - left), Y: core.Points(y - top)},
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
// Animate implements engine.Platform. This backend has no display link yet:
// apps drive their own frames with a timer, which costs the wakeups a
// platform-driven frame would not. See the macOS backend for the shape.
func (s *surface) Animate(on bool) {}

func (s *surface) Mount(b *core.MountBatch) {
	var lb *layoutBatch
	if b.LayoutAnimation != nil && !s.env.ReduceMotion {
		lb = &layoutBatch{anim: b.LayoutAnimation, created: map[core.NodeID]bool{}}
	}
	for i := range b.Items {
		it := &b.Items[i]
		switch it.Kind {
		case core.ItemAnimation:
			s.animator.Apply(it.Animation)
			continue
		case core.ItemCreate:
			v := s.create(it.ID, it.View)
			if lb != nil {
				lb.created[it.ID] = true
			}
			if it.ID == b.Root {
				s.root = v
				s.host.Call("appendChild", v.el)
			}
			continue
		case core.ItemAlert:
			s.showAlert(it.Alert)
			continue
		case core.ItemDismissKeyboard:
			if el := s.doc.Get("activeElement"); editing(el) {
				el.Call("blur")
			}
			continue
		case core.ItemSetClipboard:
			s.setClipboard(it.Value)
			continue
		case core.ItemReadClipboard:
			s.readClipboard(it.Count)
			continue
		case core.ItemAnnounce:
			s.announce(it.Value)
			continue
		case core.ItemKeymap:
			s.setKeymap(it.Keys)
			continue
		case core.ItemTitle:
			s.doc.Set("title", it.Value)
			continue
		case core.ItemStatusBar, core.ItemBackUnhandled, core.ItemMenu:
			// A page has no status bar nor menu bar, and the browser's
			// back is its own.
			continue
		case core.ItemWindow:
			s.windowCommand(it.Window)
			continue
		}
		v := s.view(it.ID)
		if v == nil {
			continue
		}
		switch it.Kind {
		case core.ItemInsert:
			parent := s.view(it.Parent)
			if parent == nil {
				continue
			}
			if before := s.view(it.Before); before != nil {
				parent.content.Call("insertBefore", v.el, before.el)
			} else {
				parent.content.Call("appendChild", v.el)
			}
			b := core.Edges{}
			if parent.content.Equal(parent.el) {
				b = parent.border
			}
			unborder(v.el, b)
			// A text's children are its attachments, which copying leaves
			// out.
			v.el.Get("classList").Call("toggle", "nsgo-attachment", parent.kind == core.KindText)
			if v.kind == core.KindModal && !v.presented {
				v.presented = true
				s.present(v)
			}
			s.appear(v, lb, it.Parent)
		case core.ItemRemove:
			v.el.Call("remove")
		case core.ItemDelete:
			// Forgotten first, so events its removal fires are not sent.
			s.views[it.ID] = nil
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
			if v.web != nil {
				v.web.closePort()
			}
			if v.video != nil {
				v.video.release()
			}
			s.setOutside(v, false)
			v.listeners.release()
		case core.ItemFrame:
			s.moveFrame(v, it.Frame, lb)
			v.framed = true
			setStyle(v.el, frameStyle(it.Frame))
			if v.size != it.Frame.Size {
				v.size = it.Frame.Size
				if v.image != nil && v.image.config.ResizeMode == core.Center {
					v.image.draw(v.size)
				}
				if v.kind == core.KindCanvas {
					s.paint(v)
				}
			}
		case core.ItemViewStyle:
			v.style = *it.ViewStyle
			s.setViewStyle(v, it.ViewStyle)
			if v.animated.Set != 0 {
				s.animator.Touch(v.id)
			}
		case core.ItemNative:
			s.configureNative(v, it.Native)
		case core.ItemNativeCommand:
			if v.component != nil {
				v.component.Command(it.Value, it.Data)
			}
		case core.ItemText:
			s.setText(v, it.Text)
		case core.ItemInput:
			s.setInput(v, it.Input)
		case core.ItemImage:
			s.setImage(v, it.Image)
		case core.ItemWeb:
			s.setWeb(v, it.Web)
		case core.ItemWebCommand:
			s.webCommand(v, it.WebCommand, it.Value)
		case core.ItemVideo:
			if v.video != nil {
				s.setVideo(v, it.Video)
			}
		case core.ItemVideoSeek:
			if v.video != nil {
				v.video.seek(it.Time)
			}
		case core.ItemModal:
			v.modal = it.Modal.Animation
		case core.ItemCanvas:
			if v.kind == core.KindCanvas {
				v.drawing = it.Drawing
				s.paint(v)
			}
		case core.ItemScrollTo:
			if v.kind == core.KindScroll {
				behavior := "auto"
				if it.Animated {
					behavior = "smooth"
				}
				// The browser clamps the offset and reports the scroll.
				v.el.Call("scrollTo", map[string]any{
					"left": float64(it.Point.X), "top": float64(it.Point.Y), "behavior": behavior,
				})
			}
		case core.ItemInputValue:
			// Setting the text the field shows would move the caret.
			if v.kind == core.KindInput && v.edits <= it.Count && v.inner.Get("value").String() != it.Value {
				v.inner.Set("value", it.Value)
			}
		case core.ItemEvents:
			s.setEvents(v, it.Events)
		case core.ItemViewProps:
			v.disabled = it.ViewProps.Disabled
			setViewProps(v, it.ViewProps)
			s.setSelection(v, it.ViewProps)
			if v.panActivation != it.ViewProps.PanActivation {
				v.panActivation = it.ViewProps.PanActivation
				s.setGestureStyle(v)
			}
		case core.ItemScrollConfig:
			x, y := "hidden", "auto"
			if it.Scroll.Horizontal {
				x, y = y, x
			}
			bar := "auto"
			if it.Scroll.HideIndicator {
				bar = "none"
			}
			setStyle(v.el, map[string]string{"overflow-x": x, "overflow-y": y, "scrollbar-width": bar})
		case core.ItemScrollContent:
			v.scroll = it.Size
			v.placeContent()
		case core.ItemFocus:
			if v.kind == core.KindInput {
				v.inner.Call("focus")
			}
		case core.ItemBlur:
			if v.kind == core.KindInput {
				v.inner.Call("blur")
			}
		}
	}
	s.applyAnimated()
	if s.animator.Running() {
		s.startTicking()
	}
}

func (s *surface) element(tag string) js.Value {
	el := s.doc.Call("createElement", tag)
	setStyle(el, map[string]string{
		"position": "absolute", "box-sizing": "border-box", "margin": "0", "left": "0", "top": "0",
	})
	return el
}

func (s *surface) create(id core.NodeID, kind core.ViewKind) *view {
	for len(s.views) <= int(id) {
		s.views = append(s.views, nil)
	}
	v := &view{id: id, kind: kind, el: s.element("div")}
	v.el.Set("nsgoID", int(id))
	v.content = v.el
	switch kind {
	case core.KindText:
		v.inner = s.element("div")
		v.inner.Set("className", "nsgo-text")
		setStyle(v.inner, textBase)
		v.el.Call("appendChild", v.inner)
	case core.KindInput:
		s.createInput(v)
	case core.KindImage:
		s.createImage(v)
	case core.KindWeb:
		s.createWeb(v)
	case core.KindVideo:
		s.createVideo(v)
	case core.KindModal:
		v.el.Call("setAttribute", "aria-modal", "true")
	case core.KindCanvas:
		v.inner = s.element("canvas")
		setStyle(v.inner, map[string]string{"width": "100%", "height": "100%"})
		v.el.Call("appendChild", v.inner)
	case core.KindScroll:
		setStyle(v.el, map[string]string{"overflow": "auto"})
		v.content = s.element("div")
		v.el.Call("appendChild", v.content)
		v.listeners.on(v.el, "scroll", func(js.Value) {
			offset := core.Point{
				X: core.Points(v.el.Get("scrollLeft").Float()),
				Y: core.Points(v.el.Get("scrollTop").Float()),
			}
			if s.animator.Scroll(v.id, offset) {
				s.applyAnimated()
			}
			s.send(v, core.Event{Kind: core.EventScroll, Point: offset})
		})
	}
	s.views[id] = v
	return v
}

// placeContent sizes a scroll view's content. Children are framed from
// the view's origin, but the browser scrolls inside its border.
func (v *view) placeContent() {
	b := v.border
	setStyle(v.content, map[string]string{
		"left": px(-b.Left), "top": px(-b.Top), "width": px(v.scroll.Width - b.Left), "height": px(v.scroll.Height - b.Top),
	})
}

func (s *surface) setViewStyle(v *view, vs *core.ViewStyle) {
	style := map[string]string{
		"background-color": cssColor(vs.Background),
		"border-radius":    px(vs.CornerRadius),
		"opacity":          fmt.Sprint(vs.Opacity),
		"box-shadow":       "none",
		"display":          "block",
		"transform":        transformCSS(vs.Transform),
		"transition":       transitionCSS(vs.Transition),
	}
	if vs.ShadowColor != 0 {
		style["box-shadow"] = fmt.Sprintf("%s %s %s %s",
			px(vs.ShadowOffset.X), px(vs.ShadowOffset.Y), px(vs.ShadowRadius), cssColor(vs.ShadowColor))
	}
	if vs.Hidden {
		style["display"] = "none"
	}
	border := core.All(vs.BorderWidth)
	colors := core.AllColors(vs.BorderColor)
	if vs.Borders != (core.Edges{}) {
		border, colors = vs.Borders, vs.BorderColors
	}
	for _, side := range []struct {
		name  string
		width core.Points
		color core.Color
	}{
		{"top", border.Top, colors.Top}, {"right", border.Right, colors.Right},
		{"bottom", border.Bottom, colors.Bottom}, {"left", border.Left, colors.Left},
	} {
		style["border-"+side.name+"-style"] = "solid"
		style["border-"+side.name+"-width"] = px(side.width)
		style["border-"+side.name+"-color"] = cssColor(side.color)
	}
	if im := v.image; im != nil && im.tint != vs.TintColor {
		im.tint = vs.TintColor
		im.draw(v.size)
	}
	if border != v.border {
		v.border = border
		switch v.kind {
		case core.KindScroll:
			v.placeContent()
		case core.KindCanvas:
			s.paint(v)
		}
		if v.kind != core.KindScroll {
			kids := v.el.Get("children")
			for i := range kids.Length() {
				el := kids.Index(i)
				if !el.Get("nsgoID").IsUndefined() || el.Equal(v.inner) && (v.kind == core.KindText || v.kind == core.KindInput) {
					unborder(el, border)
				}
			}
		}
	}
	if v.kind != core.KindScroll {
		style["overflow"] = "visible"
		if vs.Clip {
			style["overflow"] = "hidden"
		}
	}
	setStyle(v.el, style)
}

// unborder places el, framed from its parent view's origin, where the
// frame says: CSS places it inside the parent's border b.
func unborder(el js.Value, b core.Edges) {
	setStyle(el, map[string]string{"margin-left": px(-b.Left), "margin-top": px(-b.Top)})
}

// transformCSS is t around the view's center, which is CSS's default
// origin: translate, then rotate, then scale.
func transformCSS(t core.Transform) string {
	if t == (core.Transform{}) {
		return "none"
	}
	sx, sy := t.Scales()
	return fmt.Sprintf("translate(%s, %s) rotate(%gdeg) scale(%g, %g)",
		px(t.TranslateX), px(t.TranslateY), t.Rotate, sx, sy)
}

// transitionProperties are the CSS properties of each transition
// property.
var transitionProperties = []struct {
	property core.TransitionProperty
	css      []string
}{
	{core.TransitionOpacity, []string{"opacity"}},
	{core.TransitionBackground, []string{"background-color"}},
	{core.TransitionTransform, []string{"transform"}},
	{core.TransitionBorder, []string{"border-width", "border-color", "border-radius"}},
	{core.TransitionShadow, []string{"box-shadow"}},
}

// transitionCSS animates the listed properties from what the view shows
// to what the same style change sets.
func transitionCSS(t core.Transition) string {
	if t.Duration <= 0 {
		return "none"
	}
	easing := "linear"
	if c := t.Easing; c != (core.Curve{}) {
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
	if anim == core.ModalSlide && s.env.ReduceMotion {
		anim = core.ModalFade
	}
	switch anim {
	case core.ModalSlide:
		animate(v.el, map[string]any{"transform": "translateY(100%)"}, 300, "cubic-bezier(0.2, 0.8, 0.2, 1)")
	case core.ModalFade:
		animate(v.el, map[string]any{"opacity": 0}, 200, "ease-out")
	}
}

// animate runs el from the keyframe from to its own style. The animation
// leaves nothing behind, so the style items set stays in charge.
func animate(el js.Value, from map[string]any, ms int, easing string) {
	from["offset"] = 0
	el.Call("animate", []any{from}, map[string]any{"duration": ms, "easing": easing})
}

func (s *surface) setText(v *view, t *core.TextContent) {
	if v.kind != core.KindText {
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
	setStyle(v.inner, frameStyle(t.Content))
	l.fill(s.doc, v.inner, true)
}

// Text fields.

var inputModes = map[core.KeyboardType]string{
	core.KeyboardEmail:   "email",
	core.KeyboardNumber:  "numeric",
	core.KeyboardDecimal: "decimal",
	core.KeyboardPhone:   "tel",
	core.KeyboardURL:     "url",
}

// createInput puts an <input> in v's element, which draws the field's
// background and border.
func (s *surface) createInput(v *view) {
	in := s.element("input")
	in.Set("type", "text")
	setStyle(in, map[string]string{
		"padding": "0", "border": "0", "outline": "none", "background": "transparent",
	})
	v.inner = in
	v.el.Call("appendChild", in)
	v.listeners.on(in, "input", func(js.Value) {
		v.edits++
		s.send(v, core.Event{Kind: core.EventChangeText, Text: in.Get("value").String(), Count: v.edits})
	})
	v.listeners.on(in, "keydown", func(e js.Value) {
		if e.Get("key").String() == "Enter" && !e.Get("isComposing").Bool() {
			s.send(v, core.Event{Kind: core.EventSubmitEditing, Text: in.Get("value").String()})
		}
	})
	v.listeners.on(in, "focus", func(js.Value) { s.send(v, core.Event{Kind: core.EventFocus}) })
	v.listeners.on(in, "blur", func(js.Value) { s.send(v, core.Event{Kind: core.EventBlur}) })
}

func (s *surface) setInput(v *view, c *core.InputConfig) {
	if v.kind != core.KindInput {
		return
	}
	in := v.inner
	setStyle(in, fontStyle(c.Style))
	setStyle(in, frameStyle(c.Content))
	kind := "text"
	if c.Masked {
		kind = "password"
	}
	if in.Get("type").String() != kind {
		in.Set("type", kind)
	}
	in.Set("placeholder", c.Placeholder)
	in.Set("readOnly", c.Readonly)
	in.Set("disabled", c.Disabled)
	setAttr(in, "inputmode", inputModes[c.KeyboardType])
}

// Events and accessibility.

var roles = map[core.Role]string{
	core.RoleButton:     "button",
	core.RoleLink:       "link",
	core.RoleHeader:     "heading",
	core.RoleImage:      "img",
	core.RoleCheckbox:   "checkbox",
	core.RoleSwitch:     "switch",
	core.RoleRadio:      "radio",
	core.RoleTab:        "tab",
	core.RoleRadioGroup: "radiogroup",
	core.RoleTabList:    "tablist",
	core.RoleMenu:       "menu",
	core.RoleMenuItem:   "menuitem",
	core.RoleDialog:     "dialog",
	core.RoleAlert:      "alert",
}

// setViewProps maps props to ARIA. A text field's go on its <input>,
// whose role is its own.
func setViewProps(v *view, p *core.ViewProps) {
	el := v.el
	if v.kind == core.KindInput {
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
	case core.RoleCheckbox, core.RoleSwitch, core.RoleRadio:
		checked = strconv.FormatBool(p.Checked)
		if p.Indeterminate {
			checked = "mixed"
		}
	}
	setAttr(el, "role", roles[p.Role])
	setAttr(el, "aria-label", p.Label)
	setAttr(el, "aria-disabled", flag(p.Disabled))
	setAttr(el, "aria-selected", flag(p.Selected))
	setAttr(el, "aria-checked", checked)
	setAttr(el, "data-testid", p.TestID)
}

// pressable reports whether v captures presses on its descendants.
func (v *view) pressable() bool {
	m := v.events
	return m.Has(core.EventPress) || m.Has(core.EventLongPress) ||
		m.Has(core.EventPressDown) || m.Has(core.EventPressUp)
}

func (s *surface) setEvents(v *view, m core.EventMask) {
	v.events = m
	s.setOutside(v, m.Has(core.EventPressOutside))
	if v.kind == core.KindInput {
		return
	}
	// Pressable views take keyboard focus and keep the page's own text
	// selection and tap highlight out of presses.
	style := map[string]string{
		"cursor": "", "user-select": "", "-webkit-user-select": "",
		"-webkit-touch-callout": "", "-webkit-tap-highlight-color": "",
	}
	tabIndex := ""
	if v.pressable() {
		style = map[string]string{
			"cursor": "pointer", "user-select": "none", "-webkit-user-select": "none",
			"-webkit-touch-callout": "none", "-webkit-tap-highlight-color": "transparent",
		}
		tabIndex = "0"
	}
	setStyle(v.el, style)
	setAttr(v.el, "tabindex", tabIndex)
	s.setGestureStyle(v)
	if !v.hover && (m.Has(core.EventHoverIn) || m.Has(core.EventHoverOut)) {
		v.hover = true
		v.listeners.on(v.el, "pointerenter", func(js.Value) { s.send(v, core.Event{Kind: core.EventHoverIn}) })
		v.listeners.on(v.el, "pointerleave", func(js.Value) { s.send(v, core.Event{Kind: core.EventHoverOut}) })
	}
}

// setGestureStyle keeps the browser's touch scrolling and zooming and
// its text selection out of a view's gestures.
func (s *surface) setGestureStyle(v *view) {
	if v.kind == core.KindInput {
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
func (s *surface) windowCommand(c *core.WindowCommand) {
	switch c.Action {
	case core.WindowToggleFullScreen:
		if s.doc.Get("fullscreenElement").Truthy() || s.doc.Get("webkitFullscreenElement").Truthy() {
			call(s.doc, "exitFullscreen", "webkitExitFullscreen")
		} else {
			call(s.doc.Get("documentElement"), "requestFullscreen", "webkitRequestFullscreen")
		}
	case core.WindowClose:
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
	s.listenGestures(&ls)
	s.listenFiles(&ls)
	// Captured, so shortcuts come before what has focus.
	s.doc.Call("addEventListener", "keydown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		s.shortcut(args[0])
		return nil
	}), true)
	// Captured, so it sees presses that views stop from bubbling.
	s.doc.Call("addEventListener", "pointerdown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		s.pressOutside(args[0])
		return nil
	}), true)
	s.longPress = js.FuncOf(func(js.Value, []js.Value) any {
		s.timer = js.Undefined()
		if p := &s.press; p.view != nil && !p.done {
			p.cancel = true
			s.send(p.view, core.Event{Kind: core.EventLongPress})
		}
		return nil
	})
}

// target returns the view that takes a press on el: the nearest text
// field or pressable view holding it.
func (s *surface) target(el js.Value) *view {
	for ; el.Truthy() && !el.Equal(s.host); el = el.Get("parentElement") {
		if v := s.viewOf(el); v != nil && (v.kind == core.KindInput || v.pressable()) {
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
	if v := s.view(core.NodeID(id.Int())); v != nil && v.el.Equal(el) {
		return v
	}
	return nil
}

// textIndex returns the byte offset of the character of text view v at
// e's pointer, or -1 when e is not on a span.
func (s *surface) textIndex(v *view, e js.Value) int {
	sp := e.Get("target").Call("closest", "[data-start]")
	if v.kind != core.KindText || !sp.Truthy() || !v.inner.Call("contains", sp).Bool() {
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
	if v.kind == core.KindInput {
		// The field's padding focuses it, as an input's own would.
		if !el.Equal(v.inner) {
			e.Call("preventDefault")
			v.inner.Call("focus")
		}
		return
	}
	s.press.view = v
	s.send(v, core.Event{Kind: core.EventPressDown})
	if v.events.Has(core.EventLongPress) {
		s.timer = js.Global().Call("setTimeout", s.longPress, longPressDelay)
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
	s.send(p.view, core.Event{Kind: core.EventPressUp})
}

// click completes a press. Clicks also come from assistive technology,
// without a pointer.
func (s *surface) click(e js.Value) {
	p := s.press
	s.press = press{}
	v := s.target(e.Get("target"))
	if v == nil || v.kind == core.KindInput {
		return
	}
	// A pointer's click counts only on the view it went down in.
	if p.active && (p.view != v || p.cancel) {
		return
	}
	s.send(v, core.Event{Kind: core.EventPress, Index: s.textIndex(v, e)})
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
	at := core.Point{
		X: core.Points(e.Get("clientX").Float() - r.Get("left").Float() - s.host.Get("clientLeft").Float()),
		Y: core.Points(e.Get("clientY").Float() - r.Get("top").Float() - s.host.Get("clientTop").Float()),
	}
	// The engine handles the events after this returns, so the views
	// reporting cannot change meanwhile.
	for _, v := range s.outside {
		if v.el.Get("isConnected").Bool() && !v.el.Call("contains", target).Bool() {
			s.send(v, core.Event{Kind: core.EventPressOutside, Point: at})
		}
	}
}

// escape cancels the topmost alert, or clears a text selection, or
// reports back unless a text field takes Escape. It also gives Select All
// to text selections.
func (s *surface) escape(e js.Value) {
	if len(s.alerts) == 0 && s.selectKey(e) {
		return
	}
	if e.Get("key").String() != "Escape" || e.Get("isComposing").Bool() || e.Get("defaultPrevented").Bool() {
		return
	}
	if n := len(s.alerts); n > 0 {
		e.Call("preventDefault")
		s.answer(s.alerts[n-1], s.alerts[n-1].cancel)
		return
	}
	if !editing(s.doc.Get("activeElement")) {
		s.engine.Dispatch(core.Event{Kind: core.EventBack})
	}
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
		case v.kind == core.KindInput:
			return
		case v.events.Has(core.EventContextMenu):
			e.Call("preventDefault")
			r := v.el.Call("getBoundingClientRect")
			s.send(v, core.Event{Kind: core.EventContextMenu, Point: core.Point{
				X: core.Points(e.Get("clientX").Float() - r.Get("left").Float()),
				Y: core.Points(e.Get("clientY").Float() - r.Get("top").Float()),
			}})
			return
		}
	}
}

// keyDown presses a focused pressable view with Enter or Space.
func (s *surface) keyDown(e js.Value) {
	if k := e.Get("key").String(); k != "Enter" && k != " " {
		return
	}
	el := e.Get("target")
	v := s.target(el)
	if v == nil || v.kind == core.KindInput {
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
	s.send(v, core.Event{Kind: core.EventPress, Index: index})
}
