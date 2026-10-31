package core

import (
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

const (
	KeyboardDismissNone KeyboardDismissMode = iota
	// KeyboardDismissOnDrag dismisses it as a drag starts.
	KeyboardDismissOnDrag
	// KeyboardDismissInteractive follows the finger dragging down into
	// it (iOS; on drag elsewhere).
	KeyboardDismissInteractive
)

// ScrollEvent is where a scroll view is: what its scroll, drag and
// momentum events report.
type ScrollEvent struct {
	// Offset is the content's offset, UIKit's contentOffset: negative
	// past the start, as ContentInset allows.
	Offset Point
	// ContentSize is the content's size, zoomed.
	ContentSize Size
	// Size is the scroll view's.
	Size Size
	// ZoomScale is how far the content is zoomed; 1 unzoomed.
	ZoomScale float32
}

// scrollSpec is what Scroll and List give a scroll view, beside its
// direction and indicator.
type scrollSpec struct {
	behavior      ScrollBehavior
	behaviorFunc  func() ScrollBehavior
	offset        Point // the initial one
	refreshing    func() bool
	onRefresh     func()
	onScroll      func(ScrollEvent)
	throttle      time.Duration
	onDragStart   func(ScrollEvent)
	onDragEnd     func(ScrollEvent)
	onMomentum    func(ScrollEvent)
	onMomentumEnd func(ScrollEvent)
	// Zooming, for Scroll only.
	minZoom, maxZoom, zoom float32
	zoomFunc               func() float32
	noBouncesZoom, noPinch bool
	centerContent          bool
}

// bindScrollSpec configures scroll view n from s: its ScrollConfig, kept
// up to date, its handlers and its initial offset.
func (t *Tree) bindScrollSpec(cx *Scope, n *node, s scrollSpec) {
	sc := n.scroll
	c := &sc.config
	c.Behavior = s.behavior
	c.Refreshable = s.onRefresh != nil
	c.MinZoomScale = zoomOr(s.minZoom, 1)
	c.MaxZoomScale = max(zoomOr(s.maxZoom, 1), c.MinZoomScale)
	c.ZoomScale = c.ClampZoom(zoomOr(s.zoom, 1))
	c.NoBouncesZoom, c.NoPinchZoom, c.CenterContent = s.noBouncesZoom, s.noPinch, s.centerContent
	sc.zoom = c.ZoomScale
	sc.on = s
	if f := s.behaviorFunc; f != nil {
		cx.renderEffect(func() {
			b := f()
			if !b.Equal(&sc.config.Behavior) {
				sc.config.Behavior = b
				t.touch(n)
			}
		})
	}
	if f := s.refreshing; f != nil {
		cx.renderEffect(func() {
			if r := f(); r != sc.config.Refreshing {
				sc.config.Refreshing = r
				t.touch(n)
			}
		})
	}
	if f := s.zoomFunc; f != nil {
		cx.renderEffect(func() {
			if z := sc.config.ClampZoom(zoomOr(f(), 1)); z != sc.config.ZoomScale {
				sc.config.ZoomScale = z
				t.touch(n)
			}
		})
	}
	if s.offset != (Point{}) {
		t.scrollTo(n, s.offset, false)
	}
}

func zoomOr(z, or float32) float32 {
	if z > 0 && z == z {
		return z
	}
	return or
}

// reportScroll hands scroll view n's offset p to its OnScroll, at most
// once per its throttle: an offset that comes sooner waits, and the
// latest one is reported once the throttle has passed since the last.
func (t *Tree) reportScroll(n *node, p Point) {
	sc := n.scroll
	d := sc.on.throttle
	if d <= 0 {
		t.within(n.owner, func() { n.h.scroll(p) })
		return
	}
	sc.latest = p
	if sc.waiting {
		return
	}
	clock := n.owner.rt.clock
	now := clock.Now()
	if wait := sc.reported.Add(d).Sub(now); !sc.reported.IsZero() && wait > 0 {
		sc.waiting = true
		clock.AfterFunc(wait, func() {
			n.owner.Post(func() {
				sc.waiting, sc.reported = false, clock.Now()
				if n.h.scroll != nil {
					n.h.scroll(sc.latest)
				}
			})
		})
		return
	}
	sc.reported = now
	t.within(n.owner, func() { n.h.scroll(p) })
}

// scrollEvent is where scroll view n is, with its offset at p.
func (n *node) scrollEvent(p Point) ScrollEvent {
	sc := n.scroll
	return ScrollEvent{Offset: p, ZoomScale: sc.zoom, Size: n.frame.Size,
		ContentSize: Size{Width: sc.content.Width * Points(sc.zoom), Height: sc.content.Height * Points(sc.zoom)}}
}

// dispatchScroll handles scroll view n's scroll events.
func (t *Tree) dispatchScroll(n *node, ev driver.Event) {
	sc := n.scroll
	if ev.Kind != driver.EventRefresh && ev.Value > 0 {
		sc.zoom = ev.Value
	}
	call := func(fn func(ScrollEvent)) {
		if fn != nil {
			e := n.scrollEvent(ev.Point)
			t.within(n.owner, func() { fn(e) })
		}
	}
	switch ev.Kind {
	case driver.EventScrollDragStart:
		call(sc.on.onDragStart)
	case driver.EventScrollDragEnd:
		call(sc.on.onDragEnd)
	case driver.EventMomentumStart:
		call(sc.on.onMomentum)
	case driver.EventMomentumEnd:
		call(sc.on.onMomentumEnd)
	case driver.EventRefresh:
		// The platform shows its spinner as the pull ends; the next
		// config tells it whether the app keeps it.
		sc.resend = true
		t.mark(n)
		if sc.on.onRefresh != nil {
			t.within(n.owner, sc.on.onRefresh)
		}
	}
}

func (sc *scrollNode) events() driver.EventMask {
	var m driver.EventMask
	bit := func(k driver.EventKind, on bool) {
		if on {
			m |= 1 << k
		}
	}
	// Every scroll view reports its offset, so the core's hit tests and
	// touches find what shows under a point, as NSH's core keeps them.
	bit(driver.EventScroll, true)
	bit(driver.EventScrollDragStart, sc.on.onDragStart != nil)
	bit(driver.EventScrollDragEnd, sc.on.onDragEnd != nil)
	bit(driver.EventMomentumStart, sc.on.onMomentum != nil)
	bit(driver.EventMomentumEnd, sc.on.onMomentumEnd != nil)
	bit(driver.EventRefresh, sc.on.onRefresh != nil)
	return m
}
