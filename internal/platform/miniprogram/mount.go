package miniprogram

import (
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/webstyle"
)

// The mounter turns each batch into the runtime's operations and the
// user's touches back into events. It is plain Go over a host, the logic
// layer's or a test's, so everything but the bridge is tested on any
// machine.
//
// The operations, which nsgo/tree.js applies to the tree the page draws:
//
//	c  id kind               create a view: view, text, input, scroll, image, canvas, modal
//	root id                  the view the page shows
//	i  parent id before      insert id below sibling before (null: on top)
//	r  id                    detach a view, keeping it
//	d  id                    destroy a view
//	f  id x y w h            its frame, in its parent's padding box
//	s  id css                its style as inline CSS
//	a  id css                what its animated values draw over its style
//	tr id css                the transition of its frame's next change
//	ka id frames             its keyframe animation (null: none)
//	t  id text               a text's runs: {x, y, s: [css], r: [[text, x, y, h, style]]}
//	e  id bits               the touches it reports (touch* constants)
//	sc id config             a scroll view's: {h, hide, rf, rt}
//	ss id w h                a scroll view's content size
//	so id x y animated       scroll to an offset
//	im id src mode           an image's source and mode
//	fd id config             a text field's configuration
//	iv id text               a text field's value
//	fo id focused            focus or blur a text field
//	sel id start end         a text field's selection, in UTF-16 units
//	cv id url                what a canvas drew, as an image
//	bk id on                 whether back goes through the app (id 0)
//
// What no view shows (alerts, the clipboard, titles) goes to the host.

// The touches a view reports (op e).
const (
	touchPress      = 1 << iota // taps: EventPress
	touchPressInOut             // EventPressDown and EventPressUp
	touchLongPress              // EventLongPress
	touchFollow                 // every touch, for pans, pinches and EventTouch
	touchCapture                // and the scroll views around it scroll no more
)

// host is what the mounter needs of the mini program.
type host interface {
	// submit hands the runtime a batch of operations (JSON).
	submit(ops []byte)
	// dispatch hands the engine an event.
	dispatch(ev driver.Event)
	// service carries out an item no view shows: an alert, the
	// clipboard, the title.
	service(it *driver.MountItem)
	// imageURL is what an <image> loads for src.
	imageURL(src driver.ImageSource) string
	// paint draws a canvas's ops at size points, its pixel ratio's
	// pixels, and returns the picture as a URL; "" while an image it
	// draws loads, which paints it again (repaint).
	paint(id driver.NodeID, ops []driver.DrawOp, size driver.Size) string
	// tick asks for a call of mounter.tick at the next frame.
	tick()
}

// view is the mounter's record of one view.
type view struct {
	id       driver.NodeID
	kind     driver.ViewKind
	parent   *view
	children []*view
	frame    driver.Bounds
	framed   bool
	// transition is set while a layout transition moves the frame.
	transition bool
	style      driver.ViewStyle
	css        string // as last sent
	animated   driver.AnimatedValues
	acss       string // as last sent
	// shift is a sticky cell's from its frame.
	shift  driver.Point
	events driver.EventMask
	props  driver.ViewProps
	layout *TextLayout
	text   *driver.TextContent
	// A scroll view's offset, the scrolls it applied and sticky cells.
	offset  driver.Point
	applied uint32
	sticky  []driver.StickyCell
	// A text field's edits by the user and the value it shows.
	edits uint32
	value string
	// A canvas's drawing.
	drawing *driver.Drawing
	modal   driver.ModalAnimation
	shown   bool // a modal was presented
}

// border returns the width of each side of v's border.
func (v *view) border() driver.Edges {
	if v.style.Borders != (driver.Edges{}) {
		return v.style.Borders
	}
	return driver.All(v.style.BorderWidth)
}

type mounter struct {
	h        host
	views    []*view
	root     driver.NodeID
	o        ops
	animator *driver.Animator
	// ticking is set while a frame is asked for.
	ticking     bool
	frameWanted bool
	// frame is called when a frame the engine asked for begins.
	frame func()
	// back is whether back goes through the app, as last sent.
	back    bool
	reduce  bool // the environment reduces motion
	touches touches
	// repaint are canvases whose drawing waits for an image.
	repaint map[driver.NodeID]bool
}

func newMounter(h host) *mounter {
	m := &mounter{h: h, repaint: map[driver.NodeID]bool{}}
	m.animator = driver.NewAnimator(h.dispatch)
	return m
}

func (m *mounter) view(id driver.NodeID) *view {
	if int(id) < len(m.views) {
		return m.views[id]
	}
	return nil
}

// kindName is the kind a view is drawn as.
func kindName(k driver.ViewKind) string {
	switch k {
	case driver.KindText:
		return "text"
	case driver.KindInput:
		return "input"
	case driver.KindScroll:
		return "scroll"
	case driver.KindImage:
		return "image"
	case driver.KindCanvas:
		return "canvas"
	case driver.KindModal:
		return "modal"
	}
	return "view"
}

// mount applies a batch and hands its operations to the host.
func (m *mounter) mount(b *driver.MountBatch) {
	m.o.reset()
	var lb *driver.LayoutAnimation
	if b.LayoutAnimation != nil && !m.reduce {
		lb = b.LayoutAnimation
	}
	for i := range b.Items {
		m.item(b, &b.Items[i], lb)
	}
	m.applyAnimated()
	m.flush()
	if m.animator.Running() {
		m.startTicking()
	}
}

// flush hands what was written to the host.
func (m *mounter) flush() {
	if !m.o.empty() {
		m.h.submit(m.o.bytes())
		m.o.reset()
	}
}

func (m *mounter) item(b *driver.MountBatch, it *driver.MountItem, lb *driver.LayoutAnimation) {
	switch it.Kind {
	case driver.ItemAnimation:
		m.animator.Apply(it.Animation)
		return
	case driver.ItemCreate:
		for len(m.views) <= int(it.ID) {
			m.views = append(m.views, nil)
		}
		v := &view{id: it.ID, kind: it.View, style: driver.DefaultViewStyle}
		m.views[it.ID] = v
		m.o.op("c", it.ID).str(kindName(it.View))
		if it.ID == b.Root {
			m.root = it.ID
			m.o.op("root", it.ID)
		}
		return
	case driver.ItemHistory:
		if on := it.Count > 0; on != m.back {
			m.back = on
			m.o.op("bk", 0).boolean(on)
		}
		return
	case driver.ItemAppearance, driver.ItemDirection:
		m.h.service(it)
		return
	}
	v := m.view(it.ID)
	if v == nil {
		if it.ID == 0 {
			m.h.service(it)
		}
		return
	}
	switch it.Kind {
	case driver.ItemInsert:
		parent := m.view(it.Parent)
		if parent == nil {
			return
		}
		m.detach(v)
		at := len(parent.children)
		if before := m.view(it.Before); before != nil {
			for k, c := range parent.children {
				if c == before {
					at = k
					break
				}
			}
		}
		parent.children = append(parent.children[:at], append([]*view{v}, parent.children[at:]...)...)
		v.parent = parent
		m.o.op("i", parent.id).num(float64(v.id))
		if before := m.view(it.Before); before != nil && before.parent == parent {
			m.o.num(float64(before.id))
		} else {
			m.o.null()
		}
		// Placed in its new parent's padding box, styled by its props.
		if v.framed {
			m.sendFrame(v)
		}
		m.sendStyle(v)
		if v.kind == driver.KindModal && !v.shown {
			v.shown = true
			m.present(v)
		}
		if lb != nil && lb.Create != nil {
			m.appear(v, lb.Create)
		}
	case driver.ItemRemove:
		m.detach(v)
		m.o.op("r", v.id)
	case driver.ItemDelete:
		m.detach(v)
		for _, c := range v.children {
			c.parent = nil
		}
		m.views[it.ID] = nil
		m.animator.Forget(v.id)
		delete(m.repaint, v.id)
		m.touches.forget(v)
		m.o.op("d", v.id)
	case driver.ItemFrame:
		resized := v.frame.Size != it.Frame.Size
		v.frame, v.framed = it.Frame, true
		tr := it.Transition
		if lb != nil && lb.Update != nil {
			tr = lb.Update
		}
		switch {
		case tr != nil && tr.Duration > 0 && !m.reduce:
			v.transition = true
			m.o.op("tr", v.id).str(layoutTransitionCSS(tr))
		case v.transition:
			v.transition = false
			m.o.op("tr", v.id).str("")
		}
		m.sendFrame(v)
		if resized {
			// Gradients are resolved against the box.
			if e := v.style.Effects; e != nil && (len(e.Gradients) > 0 || e.Mask != nil) {
				m.sendStyle(v)
			}
			if v.kind == driver.KindCanvas {
				m.paint(v)
			}
		}
	case driver.ItemViewStyle:
		old := v.border()
		anim := v.style.Animation
		v.style = *it.ViewStyle
		m.sendStyle(v)
		if v.border() != old {
			for _, c := range v.children {
				if c.framed {
					m.sendFrame(c)
				}
			}
			if v.kind == driver.KindCanvas {
				m.paint(v)
			}
		}
		if v.style.Animation != anim {
			m.keyframes(v)
		}
		if v.animated.Set != 0 {
			m.animator.Touch(v.id)
		}
	case driver.ItemViewProps:
		boxNone := v.props.PointerEvents == driver.PointerEventsBoxNone
		v.props = *it.ViewProps
		m.sendStyle(v)
		if boxNone != (v.props.PointerEvents == driver.PointerEventsBoxNone) {
			for _, c := range v.children {
				m.sendStyle(c)
			}
		}
		m.sendEvents(v)
	case driver.ItemAccessibilityLabel:
		v.props.Label = it.Value
	case driver.ItemEvents:
		v.events = it.Events
		m.sendEvents(v)
	case driver.ItemText:
		v.text = it.Text
		v.layout, _ = it.Text.Layout.(*TextLayout)
		m.sendText(v)
	case driver.ItemInput:
		m.sendInput(v, it.Input)
	case driver.ItemInputValue:
		// A value the app set before the user's latest edit is stale; one
		// the field shows is not sent again, which would move the caret.
		if it.Count < v.edits || it.Value == v.value {
			return
		}
		v.value = it.Value
		m.o.op("iv", v.id).str(it.Value)
	case driver.ItemFocus:
		m.o.op("fo", v.id).boolean(true)
	case driver.ItemBlur:
		m.o.op("fo", v.id).boolean(false)
	case driver.ItemSelection:
		m.o.op("sel", v.id).num(float64(it.Selection.Start)).num(float64(it.Selection.End))
	case driver.ItemScrollConfig:
		var cfg object
		cfg.boolean("h", it.Scroll.Horizontal).boolean("hide", it.Scroll.HideIndicator).
			boolean("rf", it.Scroll.Refreshable).boolean("rt", it.Scroll.Refreshing).
			boolean("off", it.Scroll.Behavior.NoScroll).boolean("bounce", !it.Scroll.Behavior.NoBounces).
			boolean("paging", it.Scroll.Behavior.Paging)
		m.o.op("sc", v.id).raw(cfg.bytes())
	case driver.ItemScrollContent:
		m.o.op("ss", v.id).pt(it.Size.Width).pt(it.Size.Height)
	case driver.ItemScrollTo:
		v.applied++
		m.o.op("so", v.id).pt(it.Point.X).pt(it.Point.Y).boolean(it.Animated)
	case driver.ItemScrollBy:
		v.applied++
		to := driver.Point{X: v.offset.X + it.Point.X, Y: v.offset.Y + it.Point.Y}
		m.o.op("so", v.id).pt(to.X).pt(to.Y).boolean(false)
	case driver.ItemStickyCells:
		for _, c := range v.sticky {
			if cell := m.view(c.Cell); cell != nil && cell.shift != (driver.Point{}) {
				cell.shift = driver.Point{}
				m.animatedStyle(cell)
			}
		}
		v.sticky = it.Sticky
		m.stick(v)
	case driver.ItemImage:
		mode := "aspectFill"
		switch it.Image.ResizeMode {
		case driver.Contain, driver.ScaleDown:
			mode = "aspectFit"
		case driver.Stretch:
			mode = "scaleToFill"
		case driver.Center:
			mode = "center"
		}
		src := ""
		if it.Image.Source.URI != "" || len(it.Image.Source.Data) > 0 {
			src = m.h.imageURL(it.Image.Source)
		}
		m.o.op("im", v.id).str(src).str(mode)
	case driver.ItemCanvas:
		v.drawing = it.Drawing
		var slots []driver.DrawSlot
		if it.Drawing != nil {
			slots = it.Drawing.Slots
		}
		m.animator.BindCanvas(v.id, slots)
		m.paint(v)
	case driver.ItemModal:
		v.modal = it.Modal.Animation
	default:
		m.h.service(it)
	}
}

// detach takes v out of its parent's children.
func (m *mounter) detach(v *view) {
	p := v.parent
	if p == nil {
		return
	}
	for k, c := range p.children {
		if c == v {
			p.children = append(p.children[:k], p.children[k+1:]...)
			break
		}
	}
	v.parent = nil
}

// sendFrame sends v's frame in its parent's padding box, which CSS
// places absolute children in.
func (m *mounter) sendFrame(v *view) {
	f := v.frame
	if p := v.parent; p != nil && v.kind != driver.KindModal {
		b := p.border()
		f.Origin.X -= b.Left
		f.Origin.Y -= b.Top
	}
	m.o.op("f", v.id).pt(f.Origin.X).pt(f.Origin.Y).pt(f.Size.Width).pt(f.Size.Height)
}

func (m *mounter) sendStyle(v *view) {
	if c := viewCSS(v, v.parent); c != v.css {
		v.css = c
		m.o.op("s", v.id).str(c)
	}
}

// animatedStyle sends what v's animated values draw, when it changed.
func (m *mounter) animatedStyle(v *view) {
	if c := animatedCSS(v); c != v.acss {
		v.acss = c
		m.o.op("a", v.id).str(c)
	}
}

func (m *mounter) sendEvents(v *view) {
	ev := v.events
	if v.props.PointerEvents == driver.PointerEventsNone {
		ev = 0
	}
	bits := 0
	if ev.Has(driver.EventPress) {
		bits |= touchPress
	}
	if ev.Has(driver.EventPressDown) || ev.Has(driver.EventPressUp) {
		bits |= touchPressInOut
	}
	if ev.Has(driver.EventLongPress) {
		bits |= touchLongPress
	}
	if ev.Has(driver.EventPan) || ev.Has(driver.EventPinch) || ev.Has(driver.EventRotation) ||
		ev.Has(driver.EventTouch) || m.animator.PanBound(v.id) {
		bits |= touchFollow
		// A pan that fails on vertical drags leaves them to the scroll
		// views around it; others keep the drags they start.
		pa := v.props.PanActivation
		if (ev.Has(driver.EventPan) || m.animator.PanBound(v.id)) && pa.FailOffsetY == 0 && pa.ActiveOffsetX == 0 {
			bits |= touchCapture
		}
	}
	m.o.op("e", v.id).num(float64(bits))
}

// sendText sends a text's runs, in its view's padding box.
func (m *mounter) sendText(v *view) {
	l := v.layout
	if l == nil {
		m.o.op("t", v.id).null()
		return
	}
	var t object
	off := v.text.Content.Origin
	b := v.border()
	t.num("x", float64(off.X-b.Left)).num("y", float64(off.Y-b.Top))
	var styles []byte
	styles = append(styles, '[')
	for k, s := range l.styles {
		if k > 0 {
			styles = append(styles, ',')
		}
		styles = appendString(styles, s)
	}
	t.raw("s", append(styles, ']'))
	runs := []byte{'['}
	n := 0
	for _, r := range l.runs {
		if r.text == "" {
			continue
		}
		if n > 0 {
			runs = append(runs, ',')
		}
		n++
		runs = append(runs, '[')
		runs = appendString(runs, r.text)
		for _, x := range []driver.Points{r.x, r.y, r.height} {
			runs = append(runs, ',')
			runs = appendNum(runs, float64(x))
		}
		runs = append(runs, ',')
		runs = appendNum(runs, float64(r.style))
		runs = append(runs, ']')
	}
	t.raw("r", append(runs, ']'))
	if s := v.text.Shadow; s.Color != 0 {
		t.str("sh", webstyle.TextShadow(s))
	}
	m.o.op("t", v.id).raw(t.bytes())
}

// sendInput configures a text field: the <input> or <textarea> the
// render layer shows in its content box.
func (m *mounter) sendInput(v *view, c *driver.InputConfig) {
	var j object
	css := fontCSS(c.Style.Font) + "color:" + webstyle.Color(c.Style.Color) + ";"
	if c.CaretColor != 0 {
		css += "caret-color:" + webstyle.Color(c.CaretColor) + ";"
	}
	switch c.Style.Align {
	case driver.TextAlignCenter:
		css += "text-align:center;"
	case driver.TextAlignEnd, driver.TextAlignRight:
		css += "text-align:right;"
	}
	kind := "text"
	switch c.KeyboardType {
	case driver.KeyboardNumber, driver.KeyboardPhone:
		kind = "number"
	case driver.KeyboardDecimal:
		kind = "digit"
	}
	confirm := "done"
	switch c.ReturnKey {
	case driver.ReturnKeyGo:
		confirm = "go"
	case driver.ReturnKeyNext:
		confirm = "next"
	case driver.ReturnKeySearch:
		confirm = "search"
	case driver.ReturnKeySend:
		confirm = "send"
	}
	ph := c.PlaceholderColor
	if ph == 0 {
		ph = c.Style.Color&0xffffff00 | (c.Style.Color&0xff)/2
	}
	maxLength := -1
	if c.MaxLength > 0 {
		maxLength = c.MaxLength
	}
	b := v.border()
	submit := c.Submit()
	j.str("css", css).str("ph", c.Placeholder).str("phs", "color:"+webstyle.Color(ph)+";"+fontCSS(c.Style.Font)).
		boolean("pw", c.Masked).str("type", kind).boolean("ml", c.Multiline).num("max", float64(maxLength)).
		boolean("off", c.Disabled || c.Readonly).str("confirm", confirm).
		boolean("hold", submit == driver.SubmitBehaviorSubmit).
		boolean("nl", c.Multiline && submit == driver.SubmitBehaviorNewline).
		num("x", float64(c.Content.Origin.X-b.Left)).num("y", float64(c.Content.Origin.Y-b.Top)).
		num("w", float64(c.Content.Size.Width)).num("h", float64(c.Content.Size.Height))
	m.o.op("fd", v.id).raw(j.bytes())
}

// present animates a modal in as it is first inserted; reduced motion
// fades what would slide.
func (m *mounter) present(v *view) {
	anim := v.modal
	if anim == driver.ModalSlide && m.reduce {
		anim = driver.ModalFade
	}
	switch anim {
	case driver.ModalSlide:
		m.o.op("ka", v.id).raw([]byte(`{"d":` + ms(driver.ModalSlide.Duration()) +
			`,"n":1,"f":[{"offset":0,"translateY":"100%","ease":"cubic-bezier(0.2,0.8,0.2,1)"},{"offset":1,"translateY":"0%"}]}`))
	case driver.ModalFade:
		m.o.op("ka", v.id).raw([]byte(`{"d":` + ms(driver.ModalFade.Duration()) +
			`,"n":1,"f":[{"offset":0,"opacity":0,"ease":"ease-out"},{"offset":1,"opacity":1}]}`))
	}
}

// appear fades a view the batch inserts in, as its layout animation's
// Create says.
func (m *mounter) appear(v *view, tr *driver.LayoutTransition) {
	if tr.Duration <= 0 {
		return
	}
	from := `"opacity":0`
	if tr.Property == driver.LayoutScale {
		from = `"scale":[0,0]`
	}
	m.o.op("ka", v.id).raw([]byte(`{"d":` + ms(tr.Duration) + `,"n":1,"f":[{"offset":0,` + from +
		`,"ease":"ease-out"},{"offset":1}]}`))
}

// keyframes runs v's keyframe animation in the render layer, which keeps
// it going with no work here.
func (m *mounter) keyframes(v *view) {
	a := v.style.Animation
	d, frames := a.Cycle()
	if d <= 0 || m.reduce {
		m.o.op("ka", v.id).null()
		return
	}
	var j object
	j.num("d", float64(d.Milliseconds())).num("n", float64(a.Iterations)).num("delay", float64(a.Delay.Milliseconds()))
	list := []byte{'['}
	for k, f := range frames {
		if k > 0 {
			list = append(list, ',')
		}
		var o object
		o.num("offset", float64(f.Offset)).num("opacity", float64(f.Opacity)).num("scale", float64(f.Scale)).
			num("rotate", float64(f.Rotate)).str("translateY", formatPercent(f.TranslateY))
		if f.Easing != (driver.Curve{}) {
			o.str("ease", webstyle.Easing(f.Easing))
		}
		list = append(list, o.bytes()...)
	}
	j.raw("f", append(list, ']'))
	m.o.op("ka", v.id).raw(j.bytes())
}

func ms(d time.Duration) string { return string(appendNum(nil, float64(d.Milliseconds()))) }

func formatPercent(f float32) string { return string(appendNum(nil, float64(f)*100)) + "%" }

// paint draws a canvas's drawing, with its animated arguments, inside
// its border.
func (m *mounter) paint(v *view) {
	if v.drawing == nil || !v.framed {
		if v.drawing == nil {
			m.o.op("cv", v.id).str("")
		}
		return
	}
	b := v.border()
	size := driver.Size{Width: max(v.frame.Size.Width-b.Left-b.Right, 0), Height: max(v.frame.Size.Height-b.Top-b.Bottom, 0)}
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	url := m.h.paint(v.id, m.animator.CanvasOps(v.id, v.drawing), size)
	if url == "" {
		m.repaint[v.id] = true
		return
	}
	delete(m.repaint, v.id)
	m.o.op("cv", v.id).str(url)
}

// repaintWaiting paints the canvases that waited for an image again.
func (m *mounter) repaintWaiting() {
	for id := range m.repaint {
		if v := m.view(id); v != nil {
			m.paint(v)
		} else {
			delete(m.repaint, id)
		}
	}
	m.flush()
}

// applyAnimated writes the animated values that changed.
func (m *mounter) applyAnimated() {
	m.animator.Changed(func(id driver.NodeID, vals driver.AnimatedValues) {
		if v := m.view(id); v != nil {
			v.animated = vals
			m.animatedStyle(v)
		}
	})
	m.animator.ChangedCanvases(func(id driver.NodeID) {
		if v := m.view(id); v != nil && v.kind == driver.KindCanvas {
			m.paint(v)
		}
	})
}

// stick moves scroll view v's sticky cells for its offset.
func (m *mounter) stick(v *view) {
	for _, c := range v.sticky {
		cell := m.view(c.Cell)
		if cell == nil {
			continue
		}
		var shift driver.Point
		if v.scrollHorizontal() {
			shift.X = c.StickyShift(v.offset.X)
		} else {
			shift.Y = c.StickyShift(v.offset.Y)
		}
		if shift != cell.shift {
			cell.shift = shift
			m.animatedStyle(cell)
		}
	}
}

// scrollHorizontal reports whether a scroll view's sticky cells move
// across: its content is wider than it and no taller.
func (v *view) scrollHorizontal() bool {
	return v.offset.X != 0 && v.offset.Y == 0
}

func (m *mounter) startTicking() {
	if !m.ticking {
		m.ticking = true
		m.h.tick()
	}
}

// requestFrame has the next frame call the engine's frame callbacks.
func (m *mounter) requestFrame() {
	m.frameWanted = true
	m.startTicking()
}

// tick runs one frame at now, a monotonic time.
func (m *mounter) tick(now time.Duration) {
	m.ticking = false
	m.animator.Step(now)
	m.applyAnimated()
	m.flush()
	if m.frameWanted {
		m.frameWanted = false
		if m.frame != nil {
			m.frame()
		}
	}
	if m.animator.Running() || m.frameWanted {
		m.startTicking()
	}
}

// origin returns where v's top left is on the page: its frames' origins
// less the offsets of the scroll views it is in.
func (m *mounter) origin(v *view) driver.Point {
	var p driver.Point
	for c := v; c != nil; c = c.parent {
		p.X += c.frame.Origin.X + c.shift.X
		p.Y += c.frame.Origin.Y + c.shift.Y
		if c.parent != nil && c.parent.kind == driver.KindScroll {
			p.X -= c.parent.offset.X
			p.Y -= c.parent.offset.Y
		}
		if c.kind == driver.KindModal {
			break
		}
	}
	return p
}

// pointer returns an event of kind for v at page point page.
func (m *mounter) pointer(v *view, kind driver.EventKind, page driver.Point) driver.Event {
	o := m.origin(v)
	return driver.Event{Kind: kind, ID: v.id, Page: page, Count: 1,
		Point: driver.Point{X: page.X - o.X, Y: page.Y - o.Y}}
}

// send dispatches ev for v when v is mounted and asked for its kind.
func (m *mounter) send(v *view, ev driver.Event) {
	if m.view(v.id) != v || !v.events.Has(ev.Kind) {
		return
	}
	ev.ID = v.id
	if m.animator.Gesture(&ev) {
		m.applyAnimated()
		m.flush()
		if m.animator.Running() {
			m.startTicking()
		}
	}
	m.h.dispatch(ev)
}

// press reports a tap, a long press or a press going down or up on view
// id at page point page.
func (m *mounter) press(id driver.NodeID, kind driver.EventKind, page driver.Point) {
	v := m.view(id)
	if v == nil || v.props.Disabled && kind == driver.EventPress {
		return
	}
	ev := m.pointer(v, kind, page)
	if (kind == driver.EventPress || kind == driver.EventLongPress) && v.layout != nil && v.text != nil {
		// A text's links take the presses on them.
		p := driver.Point{X: ev.Point.X - v.text.Content.Origin.X, Y: ev.Point.Y - v.text.Content.Origin.Y}
		ev.Index = -1
		if c := v.layout.HitTest(p); c.Offset >= 0 {
			ev.Index = driver.UTF16Offset(v.text.Text, c.Offset)
		}
		if !driver.PressesAt(v.text.Spans, ev.Index) {
			return
		}
	}
	m.send(v, ev)
}

// scrolled reports that scroll view id scrolled to offset.
func (m *mounter) scrolled(id driver.NodeID, offset driver.Point) {
	v := m.view(id)
	if v == nil {
		return
	}
	v.offset = offset
	if m.animator.Scroll(v.id, offset) {
		m.applyAnimated()
		if m.animator.Running() {
			m.startTicking()
		}
	}
	m.stick(v)
	m.flush()
	m.send(v, driver.Event{Kind: driver.EventScroll, Point: offset, Count: v.applied})
}

// scrollPhase reports a drag of scroll view id starting or ending, or
// its scrolling coming to rest.
func (m *mounter) scrollPhase(id driver.NodeID, kind driver.EventKind) {
	if v := m.view(id); v != nil {
		if kind == driver.EventScrollEnd {
			m.animator.Settle(v.id)
		}
		m.send(v, driver.Event{Kind: kind, Point: v.offset, Count: v.applied})
	}
}

// edited reports the user changing text field id's text.
func (m *mounter) edited(id driver.NodeID, text string) {
	if v := m.view(id); v != nil {
		v.edits++
		v.value = text
		m.send(v, driver.Event{Kind: driver.EventChangeText, Text: text, Count: v.edits})
	}
}

// field reports a text field's focus, blur or submit.
func (m *mounter) field(id driver.NodeID, kind driver.EventKind, text string) {
	if v := m.view(id); v != nil {
		m.send(v, driver.Event{Kind: kind, Text: text})
	}
}
