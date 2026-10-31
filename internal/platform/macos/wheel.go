//go:build darwin && !ios

package macos

import (
	"slices"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/scrollbar"
)

// Wheel and trackpad events reach the surface through the responder
// chain. AppKit latches them to the view a scroll began on, past where
// the pointer moved since, so the surface finds the scroll views under
// the pointer itself, picks the one they scroll, as NSH's macOS input
// does, and keeps a scroll on its view. Presses on a scroll view's
// indicator drag it, and a press that stops a snap is no press.

// wheelGesture is a trackpad scroll gesture under way.
type wheelGesture struct {
	// view is the scroll view it moves, none until a delta finds one.
	view *view
	// total is how far it moved, for its velocity.
	total    driver.Point
	velocity scrollbar.Velocity
	// held is set while it began with fingers resting and has not
	// scrolled yet.
	held bool
}

// wheels is what a surface's wheel events are doing.
type wheels struct {
	gesture *wheelGesture
	// momentum is the view the system's momentum after a gesture goes to.
	momentum *view
	// latch is the view mouse wheel notches go to while they come within
	// wheelLatch of last along the same axis and the pointer is over it.
	latch      *view
	horizontal bool
	last       time.Duration
	// chain is the scroll views an event is over, innermost first.
	chain []*view
}

// alive reports whether v is a scroll view still mounted.
func (s *surface) alive(v *view) bool {
	return v != nil && v.sc != nil && int(v.id) < len(s.views) && s.views[v.id] == v
}

// scrollWheel routes a wheel or trackpad event, which reaches any of
// the surface's views, to the scroll views under the pointer.
func scrollWheel(self objc.ID, cmd objc.SEL, event objc.ID) {
	v := views[self]
	if v == nil || v.surface == nil || v.surface.engine == nil {
		apple.Super(self, cmd, event)
		return
	}
	s := v.surface
	defer s.measuring(true)()
	s.wheels.chain = s.scrollsAt(event)
	if !s.wheelEvent(event) {
		// What no view takes goes to the host's views around the surface.
		if next := s.native.Send(sel("nextResponder")); next != 0 {
			next.Send(cmd, event)
		}
	}
}

// wheelStarts reports whether a wheel event looks for a view to scroll,
// which AppKit's hit testing finds: a gesture beginning or a wheel no
// view under the pointer latches.
func (s *surface) wheelStarts(event objc.ID) bool {
	phase, momentum := objc.Send[uint](event, sel("phase")), objc.Send[uint](event, sel("momentumPhase"))
	switch {
	case momentum != 0:
		return false
	case phase != 0:
		return phase&(phaseBegan|phaseMayBegin) != 0
	}
	latch := s.wheels.latch
	return !s.alive(latch) || eventTime(event)-s.wheels.last >= wheelLatch || !slices.Contains(s.scrollsAt(event), latch)
}

// wheelEvent follows a wheel or trackpad event over the chain's views,
// and reports whether one took it.
func (s *surface) wheelEvent(event objc.ID) bool {
	// AppKit's deltas move the content; ours move the offset.
	dx := -driver.Points(objc.Send[float64](event, sel("scrollingDeltaX")))
	dy := -driver.Points(objc.Send[float64](event, sel("scrollingDeltaY")))
	phase, momentum := objc.Send[uint](event, sel("phase")), objc.Send[uint](event, sel("momentumPhase"))
	at := eventTime(event)
	switch {
	case momentum != 0:
		s.momentumEvent(momentum, dx, dy, at)
		return true
	case phase == 0:
		if !objc.Send[bool](event, sel("hasPreciseScrollingDeltas")) {
			dx, dy = dx*wheelLine, dy*wheelLine // a mouse's lines
		}
		return s.wheelStep(dx, dy, at)
	}
	s.gestureEvent(phase, dx, dy, at)
	return true
}

// scrollTarget returns the innermost scroll view of the chain that can
// move by (dx, dy): nested views fall through to outer ones at their
// edges. Views along the wheel's axis take it, so a vertical wheel over
// a carousel scrolls the page around it, as in browsers; horizontal views
// take vertical wheels only where nothing around scrolls vertically.
func (s *surface) scrollTarget(dx, dy driver.Points) *view {
	horizontal := abs(dx) > abs(dy)
	along := false
	for _, v := range s.wheels.chain {
		along = along || v.along(horizontal)
	}
	for _, v := range s.wheels.chain {
		if (!along || v.along(horizontal)) && v.canScroll(dx, dy) {
			return v
		}
	}
	return nil
}

// gestureTarget returns the view a trackpad gesture by (dx, dy) moves:
// scrollTarget's, else, at the edges, the innermost along the gesture
// that bounces, which stretches as on macOS.
func (s *surface) gestureTarget(dx, dy driver.Points) *view {
	if v := s.scrollTarget(dx, dy); v != nil {
		return v
	}
	for _, v := range s.wheels.chain {
		if v.canBounce(dx, dy) {
			return v
		}
	}
	return nil
}

// wheelStep follows a mouse wheel's notch, or a precise scroll outside a
// gesture, of (dx, dy) points, and reports whether a view took it. One
// scroll stays on its view while the pointer is over it, as in browsers:
// an inner view passing under the pointer does not take it over, and its
// view reaching an edge does not hand the rest to an outer one.
func (s *surface) wheelStep(dx, dy driver.Points, at time.Duration) bool {
	if dx == 0 && dy == 0 {
		return true
	}
	w := &s.wheels
	horizontal := abs(dx) > abs(dy)
	v := w.latch
	if !s.alive(v) || w.horizontal != horizontal || at-w.last >= wheelLatch || !slices.Contains(w.chain, v) {
		v = s.scrollTarget(dx, dy)
	}
	if v == nil {
		w.latch = nil
		return false
	}
	w.latch, w.horizontal, w.last = v, horizontal, at
	v.wheelBy(dx, dy, at)
	return true
}

// gestureEvent follows a trackpad gesture's event in phase.
func (s *surface) gestureEvent(phase uint, dx, dy driver.Points, at time.Duration) {
	w := &s.wheels
	if phase&phaseMayBegin != 0 {
		// Fingers resting on the trackpad stop what moves; lifting them
		// without scrolling ends that, as phaseCancelled.
		s.endMomentum()
		s.endGesture(false)
		if len(w.chain) > 0 {
			v := w.chain[0]
			v.wheelBegin()
			w.gesture = &wheelGesture{view: v, held: true}
			w.gesture.velocity.Add(at, driver.Point{})
		}
		return
	}
	if phase&phaseBegan != 0 {
		s.endMomentum()
		if w.gesture == nil || !w.gesture.held {
			s.endGesture(false)
			w.gesture = &wheelGesture{}
			w.gesture.velocity.Add(at, driver.Point{})
		}
	}
	if phase&(phaseBegan|phaseChanged) != 0 {
		g := w.gesture
		if g == nil {
			s.wheelStep(dx, dy, at) // changes without a beginning: synthesized
			return
		}
		if dx == 0 && dy == 0 {
			return
		}
		if g.held {
			// After resting fingers the scroll goes where it points: the
			// view they rested on keeps it only if it scrolls that way.
			if s.gestureTarget(dx, dy) == g.view {
				g.held = false
			} else {
				s.endGesture(false)
				g = &wheelGesture{}
				g.velocity.Add(at, driver.Point{})
				w.gesture = g
			}
		}
		if !s.alive(g.view) {
			g.view = s.gestureTarget(dx, dy)
			if g.view == nil {
				return
			}
			g.view.wheelBegin()
		}
		g.total.X, g.total.Y = g.total.X+dx, g.total.Y+dy
		g.velocity.Add(at, g.total)
		g.view.wheelBy(dx, dy, at)
	}
	if phase&(phaseEnded|phaseCancelled) != 0 {
		if g := w.gesture; g != nil {
			g.velocity.Add(at, g.total)
		}
		if v := s.endGesture(true); v != nil {
			w.momentum = v
		}
	}
}

// endGesture ends the trackpad gesture under way, let go at its velocity
// when released, else at rest, and returns the view it moved.
func (s *surface) endGesture(released bool) *view {
	g := s.wheels.gesture
	s.wheels.gesture = nil
	if g == nil || !s.alive(g.view) {
		return nil
	}
	var vx, vy float32
	if released {
		vx, vy = g.velocity.Speed()
	}
	g.view.wheelEnd(vx, vy)
	return g.view
}

// momentumEvent follows the system's momentum after a gesture, which goes
// to the gesture's view.
func (s *surface) momentumEvent(phase uint, dx, dy driver.Points, at time.Duration) {
	v := s.wheels.momentum
	if !s.alive(v) {
		return
	}
	if phase&phaseBegan != 0 && !v.momentumBegin() {
		s.wheels.momentum = nil // it springs back or snaps instead
		return
	}
	if phase&(phaseBegan|phaseChanged) != 0 && (dx != 0 || dy != 0) {
		v.momentumBy(dx, dy, at)
	}
	if phase&(phaseEnded|phaseCancelled) != 0 {
		s.endMomentum()
	}
}

// endMomentum ends the system's momentum on its view: it ended, or the
// fingers touched the trackpad again.
func (s *surface) endMomentum() {
	v := s.wheels.momentum
	s.wheels.momentum = nil
	if s.alive(v) {
		v.momentumEnd()
	}
}

// Presses on scroll views.

// thumbDrag is the mouse dragging a scroll view's indicator.
type thumbDrag struct {
	view *view
	from driver.Point // where it was grabbed, in the view
}

// scrollsAt returns the scroll views under a mouse event, innermost first.
func (s *surface) scrollsAt(event objc.ID) []*view {
	up := s.native.Send(selSuperview)
	if up == 0 {
		return nil
	}
	loc := objc.Send[apple.CGPoint](event, selLocationInWindow)
	if event.Send(selWindow) == 0 {
		// A test's wheel event, made in no window, is on the screen.
		loc = objc.Send[apple.CGPoint](s.window, sel("convertPointFromScreen:"), loc)
	}
	p := objc.Send[apple.CGPoint](up, selConvertPoint, loc, objc.ID(0))
	rawHit = true
	hit := s.native.Send(sel("hitTest:"), p)
	rawHit = false
	var out []*view
	for n := hit; n != 0 && n != up; n = n.Send(selSuperview) {
		if v := views[n]; v != nil && v.native == n && s.alive(v) {
			out = append(out, v)
		}
	}
	return out
}

// pressScrolls follows a left mouse down: a press on a scroll view's
// indicator drags it, and a press that stops a snap after a release is
// no press, as a finger catching a glide on iOS. It reports whether it
// took the press, and the release that follows.
func (s *surface) pressScrolls(event objc.ID) bool {
	s.caught = false
	scrolls := s.scrollsAt(event)
	if len(scrolls) == 0 {
		return false
	}
	loc := objc.Send[apple.CGPoint](event, selLocationInWindow)
	for _, v := range scrolls {
		p := objc.Send[apple.CGPoint](v.native, selConvertPoint, loc, objc.ID(0))
		at := driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}
		if v.grabsThumb(at) {
			s.thumb = &thumbDrag{view: v, from: at}
			v.thumbBegin()
			return true
		}
	}
	for _, v := range scrolls {
		s.caught = v.catch() || s.caught
	}
	return s.caught
}

// dragScrolls follows a left mouse drag of a press pressScrolls took.
func (s *surface) dragScrolls(event objc.ID) bool {
	if d := s.thumb; d != nil {
		if s.alive(d.view) {
			loc := objc.Send[apple.CGPoint](event, selLocationInWindow)
			p := objc.Send[apple.CGPoint](d.view.native, selConvertPoint, loc, objc.ID(0))
			d.view.thumbMove(driver.Point{X: driver.Points(p.X) - d.from.X, Y: driver.Points(p.Y) - d.from.Y})
		}
		return true
	}
	return s.caught
}

// releaseScrolls follows the left mouse up of a press pressScrolls took.
func (s *surface) releaseScrolls() bool {
	if d := s.thumb; d != nil {
		s.thumb = nil
		if s.alive(d.view) {
			d.view.thumbEnd()
		}
		return true
	}
	caught := s.caught
	s.caught = false
	return caught
}

// forgetScroll drops what the surface's input keeps of scroll view v,
// which goes away.
func (s *surface) forgetScroll(v *view) {
	delete(s.armed, v)
	for i := range v.sc.edges {
		if v.sc.edges[i].holder != 0 {
			v.sc.edges[i].paint.Release()
		}
	}
	w := &s.wheels
	if w.gesture != nil && w.gesture.view == v {
		w.gesture.view = nil
	}
	if w.momentum == v {
		w.momentum = nil
	}
	if w.latch == v {
		w.latch = nil
	}
	if s.thumb != nil && s.thumb.view == v {
		s.thumb = nil
	}
}
