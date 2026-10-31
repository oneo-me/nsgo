//go:build js && wasm

package web

import (
	"math"
	"strconv"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Scroll views, as NSH's web backend scrolls them: the browser scrolls,
// CSS scroll snapping stops at the snap points core.ScrollBehavior lists,
// overscroll-behavior turns bounces off, and the content inset is room
// around the content. Touch drags bracket the drag events, and the
// browser's glide or snap after a drag the momentum events; a scroll bar
// drag is a drag too. A touch pulling a refreshable view down past its top
// refreshes it, and a spinner, or the app's own indicator, keeps room
// above the content while it refreshes. A zoomable view scales its content element, pinched with two
// fingers or Ctrl and the wheel. DecelerationRate and the keyboard props
// are the browser's, so they do nothing here.

const (
	momentumWait = 150 // ms after a drag a scroll is its glide
)

const scrollCSS = `@keyframes nsgo-spin { to { transform: rotate(360deg) } }
.nsgo-spinner { position: absolute; left: 50%; width: 22px; height: 22px; margin-left: -11px; box-sizing: border-box;
  border: 2.5px solid rgba(128,128,128,.3); border-top-color: rgba(128,128,128,.9); border-radius: 50%;
  animation: nsgo-spin .8s linear infinite; pointer-events: none; }
.nsgo-snap { position: absolute; width: 1px; height: 1px; pointer-events: none; scroll-snap-align: start; }`

// scrollState is what a scroll view keeps beside its element.
type scrollState struct {
	// edges are the bands of its sides' edge effects.
	edges   [4]js.Value
	config  driver.ScrollConfig
	spacer  js.Value // sizes what scrolls: the content, zoomed, and insets
	markers []js.Value
	spinner js.Value
	zoom    float64
	// pull moves the content down: a touch pulling past the top, or the
	// spinner's room while refreshing.
	pull       float64
	refreshing bool
	// A touch drag and the glide after it.
	touchY, touchAt         float64
	touching, dragging, bar bool
	momentum                bool
	released                float64
	pulling                 bool
	// A two-finger pinch: its distance and zoom as it began, and the
	// content point under its middle.
	pinchDist, pinchZoom float64
	pinchAt              [2]float64
	pinchMid             [2]float64
	// applied counts the core's ItemScrollTo and ItemScrollBy items,
	// reported with each offset; smooth is where a smooth scroll of the
	// core's in flight goes, in scrollLeft and scrollTop; tracks the
	// sticky cells by ID.
	applied uint32
	smooth  *[2]float64
	tracks  map[driver.NodeID]*view
}

// newScroll sets scroll view v up.
func (s *surface) newScroll(v *view) {
	if !s.scrollCSS {
		s.scrollCSS = true
		el := s.doc.Call("createElement", "style")
		el.Set("textContent", scrollCSS)
		s.doc.Get("head").Call("appendChild", el)
	}
	v.sc = &scrollState{zoom: 1}
	v.sc.spacer = s.element("div")
	setStyle(v.sc.spacer, map[string]string{"pointer-events": "none", "visibility": "hidden"})
	v.el.Call("appendChild", v.sc.spacer)
	setStyle(v.content, map[string]string{"transform-origin": "0 0"})
	v.listeners.on(v.el, "scroll", func(js.Value) { s.scrolled(v) })
	v.listeners.on(v.el, "scrollend", func(js.Value) { s.scrollEnded(v) })
	v.listeners.on(v.el, "touchstart", func(e js.Value) { s.scrollTouchStart(v, e) })
	v.listeners.on(v.el, "touchmove", func(e js.Value) { s.scrollTouchMove(v, e) })
	v.listeners.on(v.el, "touchend", func(e js.Value) { s.scrollTouchEnd(v, e) })
	v.listeners.on(v.el, "touchcancel", func(e js.Value) { s.scrollTouchEnd(v, e) })
	v.listeners.on(v.el, "pointerdown", func(e js.Value) { s.scrollBarDown(v, e) })
	v.listeners.on(v.el, "wheel", func(e js.Value) { s.scrollWheelZoom(v, e) })
	// The user takes over a smooth scroll of the core's.
	for _, kind := range []string{"wheel", "pointerdown", "touchstart", "keydown"} {
		v.listeners.on(v.el, kind, func(js.Value) { v.sc.smooth = nil })
	}
	v.listeners.on(s.doc, "pointerup", func(js.Value) {
		if v.sc.bar && s.view(v.id) == v {
			v.sc.bar = false
			s.scrollEvent(v, driver.EventScrollDragEnd)
			v.sc.released = now()
		}
	})
}

func now() float64 { return js.Global().Get("performance").Call("now").Float() }

// configureScroll applies a scroll view's configuration.
func (s *surface) configureScroll(v *view, c *driver.ScrollConfig) {
	sc := v.sc
	old := sc.config
	sc.config = *c
	b := &c.Behavior
	x, y := "hidden", "auto"
	switch {
	case b.NoScroll:
		y = "hidden"
	case c.Zooms():
		x = "auto"
	case c.Horizontal:
		x, y = y, x
	}
	bar := "auto"
	if c.HideIndicator {
		bar = "none"
	}
	overscroll := ""
	if b.NoBounces {
		overscroll = "none"
	}
	touch := ""
	if c.Zooms() {
		touch = "pan-x pan-y" // pinches are the view's
	}
	setStyle(v.el, map[string]string{"overflow-x": x, "overflow-y": y, "scrollbar-width": bar,
		"overscroll-behavior": overscroll, "touch-action": touch})
	if !c.Zooms() {
		sc.zoom = 1
	} else {
		sc.zoom = min(max(sc.zoom, float64(c.MinZoomScale)), float64(c.MaxZoomScale))
	}
	s.setRefreshing(v, c.Refreshing && !c.Horizontal)
	v.placeContent()
	if c.Zooms() && c.ZoomScale != old.ZoomScale {
		s.zoomCommand(v, &driver.ZoomRequest{Scale: c.ZoomScale}, true)
	}
	s.placeEdges(v)
}

// placeContent sizes a scroll view's content. Children are framed from
// the view's origin, but the browser scrolls inside its border; the
// content sits inside the inset, zoomed, pulled down and centered as the
// view says, and the spacer gives the browser the extent to scroll.
func (v *view) placeContent() {
	b, sc := v.border, v.sc
	in := sc.config.Behavior.ContentInset
	z := sc.zoom
	w, h := float64(v.scroll.Width)*z, float64(v.scroll.Height)*z
	var ox, oy float64
	if sc.config.CenterContent {
		ox = max((float64(v.size.Width-in.Left-in.Right)-w)/2, 0)
		oy = max((float64(v.size.Height-in.Top-in.Bottom)-h)/2, 0)
	}
	transform := ""
	if z != 1 {
		transform = "scale(" + strconv.FormatFloat(z, 'g', -1, 64) + ")"
	}
	setStyle(v.content, map[string]string{
		"left": px(in.Left - b.Left + driver.Points(ox)), "top": px(in.Top - b.Top + driver.Points(oy+sc.pull)),
		"width": px(v.scroll.Width - b.Left), "height": px(v.scroll.Height - b.Top), "transform": transform,
	})
	setStyle(sc.spacer, map[string]string{
		"width":  px(driver.Points(w+ox) + in.Left + in.Right - b.Left),
		"height": px(driver.Points(h+oy+sc.pull) + in.Top + in.Bottom - b.Top),
	})
	if sc.spinner.Truthy() {
		setStyle(sc.spinner, map[string]string{"top": px((driver.RefreshSpace-22)/2 - b.Top)})
	}
	v.snapMarkers()
}

// snapMarkers marks the view's snap points for CSS scroll snapping.
func (v *view) snapMarkers() {
	sc := v.sc
	for _, m := range sc.markers {
		m.Call("remove")
	}
	sc.markers = sc.markers[:0]
	b := &sc.config.Behavior
	snap := ""
	viewport, content, lead := v.size.Height, v.scroll.Height+b.ContentInset.Bottom, b.ContentInset.Top
	if sc.config.Horizontal {
		viewport, content, lead = v.size.Width, v.scroll.Width+b.ContentInset.Right, b.ContentInset.Left
	}
	if points := b.SnapPoints(viewport, content); len(points) > 0 && !sc.config.Zooms() {
		snap = "y mandatory"
		if sc.config.Horizontal {
			snap = "x mandatory"
		}
		stop := b.Paging || b.SnapToInterval > 0 || b.DisableIntervalMomentum
		for _, p := range points {
			m := v.el.Get("ownerDocument").Call("createElement", "div")
			m.Set("className", "nsgo-snap")
			at := px(p + lead - v.border.Top)
			style := map[string]string{"top": at, "left": "0"}
			if sc.config.Horizontal {
				style = map[string]string{"left": px(p + lead - v.border.Left), "top": "0"}
			}
			if stop {
				style["scroll-snap-stop"] = "always"
			}
			setStyle(m, style)
			v.el.Call("appendChild", m)
			sc.markers = append(sc.markers, m)
		}
	}
	setStyle(v.el, map[string]string{"scroll-snap-type": snap})
}

// offset returns the view's offset as UIKit's contentOffset: past the
// leading inset, and negative while pulled down.
func (v *view) offset() driver.Point {
	in := v.sc.config.Behavior.ContentInset
	return driver.Point{
		X: driver.Points(v.el.Get("scrollLeft").Float()) - in.Left,
		Y: driver.Points(v.el.Get("scrollTop").Float()-v.sc.pull) - in.Top,
	}
}

// scrollTo scrolls the view so its offset is p; the browser clamps it.
func (v *view) scrollTo(p driver.Point, animated bool) {
	behavior := "auto"
	if animated {
		behavior = "smooth"
	}
	in := v.sc.config.Behavior.ContentInset
	to := [2]float64{float64(p.X + in.Left), float64(p.Y+in.Top) + v.sc.pull}
	v.sc.smooth = nil
	if animated {
		v.sc.smooth = &to
	}
	v.el.Call("scrollTo", map[string]any{"left": to[0], "top": to[1], "behavior": behavior})
}

// scrollBy moves the offset by d: the core keeps the visible content in
// place as rows above it change size. A smooth scroll of the core's in
// flight moves with it, as on the native backends: setting the offset
// would stop it.
func (v *view) scrollBy(d driver.Point) {
	if sm := v.sc.smooth; sm != nil {
		sm[0] += float64(d.X)
		sm[1] += float64(d.Y)
		v.el.Call("scrollTo", map[string]any{"left": sm[0], "top": sm[1], "behavior": "smooth"})
		return
	}
	v.el.Set("scrollLeft", v.el.Get("scrollLeft").Float()+float64(d.X))
	v.el.Set("scrollTop", v.el.Get("scrollTop").Float()+float64(d.Y))
}

// clipOverflow is overflow: clip where the browser has it, which clips
// as hidden does but lets sticky cells inside reach the scroll view
// around.
var clipOverflow = func() func() string {
	var value string
	return func() string {
		if value == "" {
			value = "hidden"
			if css := js.Global().Get("CSS"); css.Truthy() && css.Call("supports", "overflow", "clip").Bool() {
				value = "clip"
			}
		}
		return value
	}
}()

// scrollEvent reports kind on scroll view v at its offset and zoom.
func (s *surface) scrollEvent(v *view, kind driver.EventKind) {
	if kind == driver.EventScrollDragEnd || kind == driver.EventMomentumEnd {
		// At rest, unless a glide follows, which scrolls on.
		s.animator.Settle(v.id)
		if s.animator.Running() {
			s.startTicking()
		}
	}
	e := driver.Event{Kind: kind, Point: v.offset(), Count: v.sc.applied}
	if v.sc.config.Zooms() {
		e.Value = float32(v.sc.zoom)
	}
	s.send(v, e)
}

func (s *surface) scrolled(v *view) {
	sc := v.sc
	offset := v.offset()
	scrolled := s.animator.Scroll(v.id, offset)
	if s.pulled(v) || scrolled {
		s.applyAnimated()
	}
	if sc.touching && !sc.dragging {
		sc.dragging = true
		s.scrollEvent(v, driver.EventScrollDragStart)
	}
	// Still moving after the finger or the bar let go: a glide or a snap.
	if !sc.touching && !sc.bar && !sc.momentum && sc.released > 0 && now()-sc.released < momentumWait {
		sc.momentum = true
		s.scrollEvent(v, driver.EventMomentumStart)
	}
	s.scrollEvent(v, driver.EventScroll)
}

func (s *surface) scrollEnded(v *view) {
	sc := v.sc
	sc.smooth = nil
	if sc.touching || sc.bar {
		return
	}
	sc.released = 0
	if sc.momentum {
		sc.momentum = false
		s.scrollEvent(v, driver.EventMomentumEnd)
	}
}

// scrollBarDown follows a mouse press on the view's scroll bar, outside
// its content's client area, as a drag.
func (s *surface) scrollBarDown(v *view, e js.Value) {
	if e.Get("pointerType").String() != "mouse" || !e.Get("target").Equal(v.el) || v.sc.config.Behavior.NoScroll {
		return
	}
	x, y := e.Get("offsetX").Float(), e.Get("offsetY").Float()
	if x < v.el.Get("clientWidth").Float() && y < v.el.Get("clientHeight").Float() {
		return
	}
	s.interruptMomentum(v)
	v.sc.bar = true
	s.scrollEvent(v, driver.EventScrollDragStart)
}

func (s *surface) interruptMomentum(v *view) {
	if v.sc.momentum {
		v.sc.momentum = false
		s.scrollEvent(v, driver.EventMomentumEnd)
	}
	v.sc.released = 0
}

func touchDistance(e js.Value) (dist, mx, my float64) {
	t := e.Get("touches")
	a, b := t.Index(0), t.Index(1)
	ax, ay, bx, by := a.Get("clientX").Float(), a.Get("clientY").Float(), b.Get("clientX").Float(), b.Get("clientY").Float()
	return math.Hypot(bx-ax, by-ay), (ax + bx) / 2, (ay + by) / 2
}

func (s *surface) scrollTouchStart(v *view, e js.Value) {
	sc := v.sc
	touches := e.Get("touches").Length()
	if touches == 2 && sc.config.Zooms() && !sc.config.NoPinchZoom {
		d, mx, my := touchDistance(e)
		fx, fy := v.localPoint(mx, my)
		sc.pinchDist, sc.pinchZoom = max(d, 1), sc.zoom
		sc.pinchMid = [2]float64{fx, fy}
		sc.pinchAt[0], sc.pinchAt[1] = v.contentPoint(fx, fy)
		return
	}
	if touches != 1 || sc.config.Behavior.NoScroll {
		return
	}
	s.interruptMomentum(v)
	sc.touching = true
	sc.touchY = e.Get("touches").Index(0).Get("clientY").Float()
	sc.pulling = false
}

func (s *surface) scrollTouchMove(v *view, e js.Value) {
	sc := v.sc
	if sc.pinchDist > 0 && e.Get("touches").Length() == 2 {
		e.Call("preventDefault")
		d, _, _ := touchDistance(e)
		scale := v.limitZoom(sc.pinchZoom*d/sc.pinchDist, true)
		s.zoomContent(v, scale, sc.pinchAt[0], sc.pinchAt[1], sc.pinchMid[0], sc.pinchMid[1])
		return
	}
	c := &sc.config
	if !sc.touching || !c.Refreshable || c.Horizontal || sc.refreshing {
		return
	}
	y := e.Get("touches").Index(0).Get("clientY").Float()
	dy := y - sc.touchY
	if !sc.pulling && (dy <= 0 || v.el.Get("scrollTop").Float() > 0.5) {
		sc.touchY = y
		return
	}
	// Past the top, the content follows the finger down, rubber-banded.
	e.Call("preventDefault")
	sc.pulling = true
	dim := max(float64(v.size.Height), 1)
	sc.pull = max((1-1/(dy*0.55/dim+1))*dim, 0)
	if !sc.dragging {
		sc.dragging = true
		s.scrollEvent(v, driver.EventScrollDragStart)
	}
	v.placeContent()
	if s.pulled(v) {
		s.applyAnimated()
	}
	s.scrollEvent(v, driver.EventScroll)
}

// pulled sets the value bound to v's pull to how far its content is
// pulled past its top, the room it keeps while refreshing included, and
// reports whether it changed.
func (s *surface) pulled(v *view) bool {
	if v.sc.config.Horizontal || !s.animator.Pulls(v.id) {
		return false
	}
	pull := max(v.sc.pull-v.el.Get("scrollTop").Float(), 0)
	return s.animator.Pull(v.id, driver.Points(pull))
}

func (s *surface) scrollTouchEnd(v *view, e js.Value) {
	sc := v.sc
	if sc.pinchDist > 0 {
		if e.Get("touches").Length() < 2 {
			sc.pinchDist = 0
			s.zoomEnded(v)
		}
		return
	}
	if !sc.touching || e.Get("touches").Length() > 0 {
		return
	}
	sc.touching = false
	if sc.dragging {
		sc.dragging = false
		s.scrollEvent(v, driver.EventScrollDragEnd)
		sc.released = now()
	}
	if sc.pulling {
		sc.pulling = false
		if sc.pull >= float64(sc.config.Behavior.RefreshThreshold()) && sc.config.Refreshable {
			// Shown right away; the next config says whether the app
			// keeps it.
			s.scrollEvent(v, driver.EventRefresh)
			s.setRefreshing(v, true)
		} else {
			sc.pull = 0
			v.placeContent()
			if s.pulled(v) {
				s.applyAnimated()
			}
			s.scrollEvent(v, driver.EventScroll)
		}
	}
}

// setRefreshing shows the spinner, or the app's indicator, and its room
// above the content, or takes them away.
func (s *surface) setRefreshing(v *view, on bool) {
	sc := v.sc
	room := float64(sc.config.Behavior.RefreshInset())
	own := sc.config.Behavior.RefreshIndicator > 0
	if on == sc.refreshing && !(on && (sc.pull != room || own == sc.spinner.Truthy())) {
		return
	}
	sc.refreshing = on
	if on && own && sc.spinner.Truthy() {
		sc.spinner.Call("remove")
		sc.spinner = js.Value{}
	}
	if on {
		if !own && !sc.spinner.Truthy() {
			sc.spinner = s.doc.Call("createElement", "div")
			sc.spinner.Set("className", "nsgo-spinner")
			v.el.Call("insertBefore", sc.spinner, v.el.Get("firstChild"))
		}
		atTop := v.el.Get("scrollTop").Float() <= 0.5
		sc.pull = room
		v.placeContent()
		if atTop {
			v.el.Set("scrollTop", 0)
		}
	} else {
		if sc.spinner.Truthy() {
			sc.spinner.Call("remove")
			sc.spinner = js.Value{}
		}
		sc.pull = 0
		v.placeContent()
	}
	if s.pulled(v) {
		s.applyAnimated()
	}
	s.scrollEvent(v, driver.EventScroll)
}

// localPoint returns client point x, y in the view's viewport.
func (v *view) localPoint(x, y float64) (float64, float64) {
	r := v.el.Call("getBoundingClientRect")
	return x - r.Get("left").Float(), y - r.Get("top").Float()
}

// contentPoint returns the content point (unzoomed) at viewport point
// fx, fy.
func (v *view) contentPoint(fx, fy float64) (float64, float64) {
	in := v.sc.config.Behavior.ContentInset
	z := v.sc.zoom
	return (v.el.Get("scrollLeft").Float() + fx - float64(in.Left)) / z,
		(v.el.Get("scrollTop").Float() + fy - float64(in.Top) - v.sc.pull) / z
}

// limitZoom keeps scale within the view's limits, past them by a rubber
// band while pinching unless NoBouncesZoom.
func (v *view) limitZoom(scale float64, rubber bool) float64 {
	c := &v.sc.config
	lo, hi := float64(c.MinZoomScale), float64(c.MaxZoomScale)
	switch {
	case scale > hi && rubber && !c.NoBouncesZoom:
		return hi * math.Pow(scale/hi, 0.3)
	case scale > hi:
		return hi
	case scale < lo && rubber && !c.NoBouncesZoom:
		return lo * math.Pow(scale/lo, 0.3)
	case scale < lo:
		return lo
	}
	return scale
}

// zoomContent zooms v to scale with content point cx, cy at viewport
// point fx, fy.
func (s *surface) zoomContent(v *view, scale, cx, cy, fx, fy float64) {
	v.sc.zoom = scale
	v.placeContent()
	in := v.sc.config.Behavior.ContentInset
	v.el.Set("scrollLeft", cx*scale+float64(in.Left)-fx)
	v.el.Set("scrollTop", cy*scale+float64(in.Top)+v.sc.pull-fy)
	s.scrollEvent(v, driver.EventScroll)
}

// zoomEnded springs a pinch back within the limits.
func (s *surface) zoomEnded(v *view) {
	if target := v.limitZoom(v.sc.zoom, false); math.Abs(target-v.sc.zoom) > 1e-3 {
		s.zoomCommand(v, &driver.ZoomRequest{Scale: float32(target)}, true)
	}
}

// zoomCommand zooms v to r.Scale with content point r.Center, or the one
// in the middle now, in the middle of the viewport.
func (s *surface) zoomCommand(v *view, r *driver.ZoomRequest, animated bool) {
	if v.sc == nil || !v.sc.config.Zooms() {
		return
	}
	target := v.limitZoom(float64(r.Scale), false)
	fx, fy := float64(v.size.Width)/2, float64(v.size.Height)/2
	px, py := v.contentPoint(fx, fy)
	cx, cy := px, py
	if r.Center != nil {
		cx, cy = float64(r.Center.X), float64(r.Center.Y)
	}
	if !animated {
		s.zoomContent(v, target, cx, cy, fx, fy)
		return
	}
	z0, start := v.sc.zoom, now()
	var step js.Func
	step = js.FuncOf(func(_ js.Value, args []js.Value) any {
		t := min((args[0].Float()-start)/250, 1)
		k := 1 - math.Pow(1-t, 3)
		if s.view(v.id) == v {
			s.zoomContent(v, z0+(target-z0)*k, px+(cx-px)*k, py+(cy-py)*k, fx, fy)
		}
		if t < 1 && s.view(v.id) == v {
			js.Global().Call("requestAnimationFrame", step)
		} else {
			step.Release()
		}
		return nil
	})
	js.Global().Call("requestAnimationFrame", step)
}

// scrollWheelZoom zooms a zoomable view with Ctrl and the wheel, as
// browsers send trackpad pinches.
func (s *surface) scrollWheelZoom(v *view, e js.Value) {
	c := &v.sc.config
	if !e.Get("ctrlKey").Bool() || !c.Zooms() || c.NoPinchZoom {
		return
	}
	e.Call("preventDefault")
	e.Call("stopPropagation")
	fx, fy := v.localPoint(e.Get("clientX").Float(), e.Get("clientY").Float())
	cx, cy := v.contentPoint(fx, fy)
	s.zoomContent(v, v.limitZoom(v.sc.zoom*math.Exp(-e.Get("deltaY").Float()/100), false), cx, cy, fx, fy)
}
