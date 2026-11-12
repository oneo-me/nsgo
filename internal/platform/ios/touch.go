//go:build ios

package ios

import (
	"slices"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// Touches for the views that listen to them: a tracker on the surface
// sees every finger, as the views do, and reports it as core.EventTouch
// without recognizing anything. A scroll view that starts dragging
// cancels them, and the touch responder disables the pans of the scroll
// views around it.

var classTracker objc.Class

// fingers are the touches down, in order, with the ids reported.
type fingers struct {
	down     []objc.ID
	ids      []int
	next     int
	touching bool
	blocked  []objc.ID // the pan recognizers the responder disabled
}

func registerTouches() {
	report := func(phase driver.GesturePhase) func(objc.ID, objc.SEL, objc.ID, objc.ID) {
		return func(self objc.ID, cmd objc.SEL, touches, event objc.ID) {
			apple.Super(self, cmd, touches, event)
			if v := views[self.Send(sel("view"))]; v != nil {
				v.surface.touched(phase, touches)
			}
			if phase == driver.GestureEnded || phase == driver.GestureCancelled {
				failLifted(self, event)
			}
		}
	}
	no := func(objc.ID, objc.SEL, objc.ID) bool { return false }
	classTracker = register("NsgoTouchTracker", apple.Class("UIGestureRecognizer"), nil, []objc.MethodDef{
		{Cmd: sel("touchesBegan:withEvent:"), Fn: report(driver.GestureBegan)},
		{Cmd: sel("touchesMoved:withEvent:"), Fn: report(driver.GestureChanged)},
		{Cmd: sel("touchesEnded:withEvent:"), Fn: report(driver.GestureEnded)},
		{Cmd: sel("touchesCancelled:withEvent:"), Fn: report(driver.GestureCancelled)},
		// It recognizes nothing, so it holds no other recognizer back.
		{Cmd: sel("canPreventGestureRecognizer:"), Fn: no},
		{Cmd: sel("canBePreventedByGestureRecognizer:"), Fn: no},
	})
}

// failLifted fails the tracker once the last of its fingers lifts, so
// UIKit resets it. Left possible, it keeps tracking the touches' event,
// and iOS 27 then resets none of the recognizers that share a gesture
// subgraph with it: a scroll view's pan stays failed and never scrolls
// again.
func failLifted(tracker, event objc.ID) {
	down := false
	apple.ForEach(event.Send(sel("touchesForGestureRecognizer:"), tracker), func(t objc.ID) {
		if p := objc.Send[int](t, sel("phase")); p != touchEnded && p != touchCancelled {
			down = true
		}
	})
	if !down {
		tracker.Send(selSetState, stateFailed)
	}
}

// UITouchPhase values.
const (
	touchEnded     = 3
	touchCancelled = 4
)

// addTracker puts the tracker on the surface.
func (s *surface) addTracker() {
	t := objc.ID(classTracker).Send(selAlloc).Send(sel("initWithTarget:action:"), objc.ID(0), objc.SEL(0))
	t.Send(sel("setCancelsTouchesInView:"), false)
	t.Send(sel("setDelaysTouchesEnded:"), false)
	s.native.Send(sel("addGestureRecognizer:"), t)
	t.Send(selRelease)
}

// touched reports the touches of phase that changed, with the others
// down, while a view listens.
func (s *surface) touched(phase driver.GesturePhase, touches objc.ID) {
	f := &s.fingers
	if s.engine == nil {
		return
	}
	if phase == driver.GestureBegan && len(f.down) == 0 {
		f.touching = s.listensTouches()
	}
	var changed []objc.ID
	apple.ForEach(touches, func(t objc.ID) {
		changed = append(changed, t)
		if phase == driver.GestureBegan && !slices.Contains(f.down, t) {
			f.down = append(f.down, t)
			f.ids = append(f.ids, f.next)
			f.next++
		}
	})
	// The fingers are followed while nothing is reported too, after a
	// scroll view took them, so the next touch asks again.
	if f.touching {
		var out []driver.Touch
		var at float64
		for i, t := range f.down {
			p := objc.Send[apple.CGPoint](t, selLocationInView, s.native)
			at = objc.Send[float64](t, sel("timestamp"))
			out = append(out, driver.Touch{ID: f.ids[i], PagePosition: driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)},
				Changed: slices.Contains(changed, t)})
		}
		s.engine.Dispatch(driver.Event{Kind: driver.EventTouch, Phase: phase, Touches: out,
			Time: time.Duration(at * float64(time.Second))})
	}
	if phase == driver.GestureEnded || phase == driver.GestureCancelled {
		for _, t := range changed {
			if i := slices.Index(f.down, t); i >= 0 {
				f.down = slices.Delete(f.down, i, i+1)
				f.ids = slices.Delete(f.ids, i, i+1)
			}
		}
	}
}

// touchesTaken cancels the touches a scroll view started dragging with.
func (s *surface) touchesTaken() {
	f := &s.fingers
	if !f.touching || len(f.blocked) > 0 || len(f.down) == 0 {
		return
	}
	var out []driver.Touch
	for i, t := range f.down {
		p := objc.Send[apple.CGPoint](t, selLocationInView, s.native)
		out = append(out, driver.Touch{ID: f.ids[i], PagePosition: driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)},
			Changed: true})
	}
	f.touching = false
	s.engine.Dispatch(driver.Event{Kind: driver.EventTouch, Phase: driver.GestureCancelled, Touches: out})
}

// blockNative disables the pans of the scroll views around v, the touch
// responder, or enables them again with nil.
func (s *surface) blockNative(v *view) {
	f := &s.fingers
	for _, pan := range f.blocked {
		pan.Send(sel("setEnabled:"), true)
		pan.Send(selRelease)
	}
	f.blocked = nil
	if v == nil {
		return
	}
	for n := v.native.Send(selSuperview); n != 0; n = n.Send(selSuperview) {
		if objc.Send[bool](n, sel("isKindOfClass:"), apple.Class("UIScrollView")) {
			pan := n.Send(sel("panGestureRecognizer"))
			pan.Send(sel("setEnabled:"), false)
			f.blocked = append(f.blocked, pan.Send(sel("retain")))
		}
	}
}

// listensTouches reports whether a view listens to touches.
func (s *surface) listensTouches() bool {
	for _, v := range s.views {
		if v != nil && v.events.Has(driver.EventTouch) {
			return true
		}
	}
	return false
}
