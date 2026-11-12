//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// Gestures, as NSH's touch.rs gates pans: a view's pan is a recognizer of
// its own that activates once the finger passes its PanActivation and
// fails once it passes a fail offset first. The pans of the scroll views
// around it wait for it to fail, so a drag it fails still scrolls them;
// once it activates, UIKit cancels the touches of the views below,
// which ends their presses. A pan that hands off (PanScrollHandoff, a
// bottom sheet's) recognizes with the pan of the scroll view inside its
// view where the finger went down, as NSH's pan_scroll_handoff: the drag
// scrolls it back to its top first, and what goes on past that moves the
// pan while the scroll view stays pinned there. Pinches and rotations are
// UIKit's recognizers, which recognize together.

// UIGestureRecognizerState values.
const (
	statePossible = iota
	stateBegan
	stateChanged
	stateEnded
	stateCancelled
	stateFailed
)

// panGate is the Go side of a view's pan recognizer.
type panGate struct {
	view     *view
	finger   objc.ID // the touch followed, retained
	start    apple.CGPoint
	at       driver.Point
	velocity driver.Point
	time     float64
	// scroll is the scroll view a handing off pan's finger went down in,
	// retained, and offset how far it was from its top; consuming is set
	// while the drag is past that and moves the pan.
	scroll    objc.ID
	offset    driver.Points
	consuming bool
}

// gates maps pan recognizers to their Go side; handoffs the scroll views
// handing a drag off to the pans of the views around them.
var (
	gates    = map[objc.ID]*panGate{}
	handoffs = map[objc.ID]*panGate{}
)

var (
	selState, selSetState = sel("state"), sel("setState:")
	selTimestamp          = sel("timestamp")
	selView               = sel("view")
	selPinchAction        = sel("nsgoPinch:")
	selRotateAction       = sel("nsgoRotate:")
	selHoverAction        = sel("nsgoHover:")
	selWheelAction        = sel("nsgoWheel:")
	classPanGate          objc.Class
)

func registerGestures() {
	classPanGate = register("NsgoPanGate", apple.Class("UIGestureRecognizer"), nil, []objc.MethodDef{
		{Cmd: sel("touchesBegan:withEvent:"), Fn: gateBegan},
		{Cmd: sel("touchesMoved:withEvent:"), Fn: gateMoved},
		{Cmd: sel("touchesEnded:withEvent:"), Fn: func(self objc.ID, _ objc.SEL, touches, _ objc.ID) {
			gateLifted(self, touches, stateEnded)
		}},
		{Cmd: sel("touchesCancelled:withEvent:"), Fn: func(self objc.ID, _ objc.SEL, touches, _ objc.ID) {
			gateLifted(self, touches, stateCancelled)
		}},
		{Cmd: sel("reset"), Fn: func(self objc.ID, cmd objc.SEL) {
			apple.Super(self, cmd)
			if g := gates[self]; g != nil {
				if g.finger != 0 {
					g.finger.Send(selRelease)
					g.finger = 0
				}
				g.endHandoff()
			}
		}},
		// The pans of the scroll views around the view wait for the gate.
		{Cmd: sel("shouldBeRequiredToFailByGestureRecognizer:"), Fn: gateHoldsBack},
		// Recognizers on the same view (pinch, rotation) go on with it, and
		// so does the pan of a scroll view handing off to it.
		{Cmd: sel("canPreventGestureRecognizer:"), Fn: func(self objc.ID, _ objc.SEL, other objc.ID) bool {
			return self.Send(selView) != other.Send(selView) && !gates[self].sharesWith(other)
		}},
		{Cmd: sel("canBePreventedByGestureRecognizer:"), Fn: func(self objc.ID, cmd objc.SEL, other objc.ID) bool {
			return !gates[self].sharesWith(other) && apple.SendSuper[bool](self, cmd, other)
		}},
	})
}

// handOff finds, for a view whose pan hands off (PanScrollHandoff), the
// scroll view inside it that touch went down in and that scrolls
// vertically: the drag first scrolls it back to its top.
func (g *panGate) handOff(touch objc.ID) {
	g.endHandoff()
	if !g.view.handoff {
		return
	}
	scrollClass := apple.Class("UIScrollView")
	for n := touch.Send(selView); n != 0 && n != g.view.native; n = n.Send(selSuperview) {
		if !objc.Send[bool](n, selIsKindOf, scrollClass) || !objc.Send[bool](n, sel("isScrollEnabled")) {
			continue
		}
		size := objc.Send[apple.CGSize](n, sel("contentSize"))
		if size.Height <= objc.Send[apple.CGRect](n, selBounds).Size.Height && !objc.Send[bool](n, sel("alwaysBounceVertical")) {
			continue
		}
		in := objc.Send[edgeInsets](n, sel("adjustedContentInset"))
		g.scroll = n.Send(sel("retain"))
		g.offset = driver.Points(max(objc.Send[apple.CGPoint](n, selContentOffset).Y+in.Top, 0))
		handoffs[g.scroll] = g
		return
	}
}

func (g *panGate) endHandoff() {
	if g.scroll != 0 {
		delete(handoffs, g.scroll)
		g.scroll.Send(selRelease)
		g.scroll, g.consuming = 0, false
	}
}

// sharesWith reports whether other is the pan of the scroll view the
// gate shares a drag with.
func (g *panGate) sharesWith(other objc.ID) bool {
	return g != nil && g.scroll != 0 && g.scroll.Send(selPanRecognizer) == other
}

// pinTop keeps a scroll view handing a drag off at its top while the
// drag moves the pan instead, so it neither scrolls nor bounces under the
// moving view. It reports whether it did.
func pinTop(scroll objc.ID) bool {
	g := handoffs[scroll]
	if g == nil || !g.consuming {
		return false
	}
	at := objc.Send[apple.CGPoint](scroll, selContentOffset)
	top := -objc.Send[edgeInsets](scroll, sel("adjustedContentInset")).Top
	if at.Y != top {
		scroll.Send(sel("setContentOffset:"), apple.CGPoint{X: at.X, Y: top})
	}
	return true
}

// gestureMethods are the views' methods for their pinch and rotation
// recognizers, whose target and delegate they are.
func gestureMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{Cmd: selPinchAction, Fn: func(self objc.ID, _ objc.SEL, r objc.ID) {
			transformed(self, r, driver.EventPinch, sel("scale"))
		}},
		{Cmd: selRotateAction, Fn: func(self objc.ID, _ objc.SEL, r objc.ID) {
			transformed(self, r, driver.EventRotation, sel("rotation"))
		}},
		{Cmd: selHoverAction, Fn: hovered},
		{Cmd: selWheelAction, Fn: wheeled},
		{Cmd: sel("gestureRecognizer:shouldRecognizeSimultaneouslyWithGestureRecognizer:"),
			Fn: func(self objc.ID, _ objc.SEL, r, other objc.ID) bool { return other.Send(selView) == self }},
	}
}

// setGestures adds and removes v's recognizers for the gestures m follows.
func (v *view) setGestures(m driver.EventMask) {
	toggle := func(want bool, r *objc.ID, make func() objc.ID) {
		switch {
		case want && *r == 0:
			if *r = make(); *r == 0 {
				return
			}
			v.native.Send(sel("addGestureRecognizer:"), *r)
			(*r).Send(selRelease)
		case !want && *r != 0:
			v.native.Send(sel("removeGestureRecognizer:"), *r)
			delete(gates, *r)
			*r = 0
		}
	}
	newRecognizer := func(class objc.Class, action objc.SEL) objc.ID {
		if class == 0 {
			// tvOS has no pinch, rotation or hover recognizers.
			return 0
		}
		target, a := v.native, action
		if action == 0 {
			target = 0
		}
		return objc.ID(class).Send(selAlloc).Send(sel("initWithTarget:action:"), target, a)
	}
	toggle(m.Has(driver.EventPan), &v.pan, func() objc.ID {
		r := newRecognizer(classPanGate, 0)
		// The views below keep their touches until the pan activates.
		r.Send(sel("setDelaysTouchesEnded:"), false)
		gates[r] = &panGate{view: v}
		return r
	})
	toggle(m.Has(driver.EventPinch), &v.pinch, func() objc.ID {
		r := newRecognizer(apple.Class("UIPinchGestureRecognizer"), selPinchAction)
		r.Send(selSetDeleg, v.native)
		return r
	})
	toggle(m.Has(driver.EventRotation), &v.rotation, func() objc.ID {
		r := newRecognizer(apple.Class("UIRotationGestureRecognizer"), selRotateAction)
		r.Send(selSetDeleg, v.native)
		return r
	})
	// UIScrollTypeMask: continuous for a trackpad, discrete for a wheel.
	scrollOnly := func(mask int) func() objc.ID {
		return func() objc.ID {
			r := newRecognizer(apple.Class("UIPanGestureRecognizer"), selWheelAction)
			r.Send(sel("setAllowedScrollTypesMask:"), mask)
			r.Send(sel("setAllowedTouchTypes:"), objc.ID(apple.Class("NSArray")).Send(sel("array")))
			r.Send(selSetDeleg, v.native)
			return r
		}
	}
	toggle(m.Has(driver.EventWheel), &v.scrolls, scrollOnly(1))
	toggle(m.Has(driver.EventWheel), &v.notches, scrollOnly(2))
	hover := m.Has(driver.EventMouseEnter) || m.Has(driver.EventMouseLeave) || m.Has(driver.EventMouseMove)
	toggle(hover, &v.hover, func() objc.ID {
		return newRecognizer(apple.Class("UIHoverGestureRecognizer"), selHoverAction)
	})
}

// hovered follows a pointer over a view, an iPad's trackpad or mouse or a
// pencil held above the screen, as the desktops' hovers: it comes in,
// moves and goes out.
func hovered(self objc.ID, _ objc.SEL, r objc.ID) {
	v := views[self]
	if v == nil {
		return
	}
	at := objc.Send[apple.CGPoint](r, selLocationInView, self)
	page := objc.Send[apple.CGPoint](r, selLocationInView, v.surface.native)
	ev := driver.Event{Point: driver.Point{X: driver.Points(at.X), Y: driver.Points(at.Y)},
		Page: driver.Point{X: driver.Points(page.X), Y: driver.Points(page.Y)}}
	report := func(kind driver.EventKind) {
		ev.Kind = kind
		v.report(ev)
	}
	switch objc.Send[int](r, selState) {
	case stateBegan:
		report(driver.EventMouseEnter)
		fallthrough
	case stateChanged:
		report(driver.EventMouseMove)
	case stateEnded, stateCancelled:
		report(driver.EventMouseLeave)
	}
}

func gateBegan(self objc.ID, _ objc.SEL, touches, _ objc.ID) {
	g := gates[self]
	if g == nil || g.finger != 0 {
		return
	}
	touch := touches.Send(selAnyObject)
	g.finger = touch.Send(sel("retain"))
	g.start = objc.Send[apple.CGPoint](touch, selLocationInView, objc.ID(0))
	g.at, g.velocity = driver.Point{}, driver.Point{}
	g.time = objc.Send[float64](touch, selTimestamp)
	g.handOff(touch)
}

func gateMoved(self objc.ID, _ objc.SEL, _, _ objc.ID) {
	g := gates[self]
	if g == nil || g.finger == 0 {
		return
	}
	state := objc.Send[int](self, selState)
	if state != statePossible && state != stateBegan && state != stateChanged {
		return
	}
	at := objc.Send[apple.CGPoint](g.finger, selLocationInView, objc.ID(0))
	d := driver.Point{X: driver.Points(at.X - g.start.X), Y: driver.Points(at.Y - g.start.Y)}
	now := objc.Send[float64](g.finger, selTimestamp)
	if dt := now - g.time; dt > 0 {
		inst := driver.Point{X: (d.X - g.at.X) / driver.Points(dt), Y: (d.Y - g.at.Y) / driver.Points(dt)}
		// Smoothed, so one uneven sample does not throw a flick.
		g.velocity = driver.Point{X: inst.X*0.7 + g.velocity.X*0.3, Y: inst.Y*0.7 + g.velocity.Y*0.3}
	}
	g.at, g.time = d, now
	phase := driver.GestureChanged
	if state == statePossible {
		switch g.view.activation.Decide(d) {
		case 1:
			phase = driver.GestureBegan
			self.Send(selSetState, stateBegan)
		case -1:
			self.Send(selSetState, stateFailed)
			return
		default:
			return
		}
	} else {
		self.Send(selSetState, stateChanged)
	}
	g.view.report(driver.Event{Kind: driver.EventPan, Phase: phase, Point: g.handedOff(d), Velocity: g.velocity})
}

// handedOff is how far drag d moves the pan: past what the scroll view it
// shares the drag with scrolls back first, never the other way.
func (g *panGate) handedOff(d driver.Point) driver.Point {
	if g.scroll == 0 {
		return d
	}
	g.consuming = d.Y > g.offset
	pinTop(g.scroll)
	d.Y = max(d.Y-g.offset, 0)
	return d
}

func gateLifted(self, touches objc.ID, how int) {
	g := gates[self]
	if g == nil || g.finger == 0 {
		return
	}
	followed := false
	apple.ForEach(touches, func(t objc.ID) { followed = followed || t == g.finger })
	if !followed {
		return
	}
	state := objc.Send[int](self, selState)
	if state != stateBegan && state != stateChanged {
		self.Send(selSetState, stateFailed)
		return
	}
	self.Send(selSetState, how)
	phase, velocity := driver.GestureEnded, g.velocity
	if how == stateCancelled {
		phase = driver.GestureCancelled
	}
	// A drag that stopped before the finger lifted is no flick.
	if objc.Send[float64](g.finger, selTimestamp)-g.time > 0.05 {
		velocity = driver.Point{}
	}
	at := g.at
	if g.scroll != 0 {
		at.Y = max(at.Y-g.offset, 0)
	}
	g.view.report(driver.Event{Kind: driver.EventPan, Phase: phase, Point: at, Velocity: velocity})
}

// gateHoldsBack reports whether other is the pan of a scroll view around
// the gate's view.
func gateHoldsBack(self objc.ID, _ objc.SEL, other objc.ID) bool {
	view, scroll := self.Send(selView), other.Send(selView)
	if view == 0 || scroll == 0 || !objc.Send[bool](scroll, sel("isKindOfClass:"), apple.Class("UIScrollView")) {
		return false
	}
	return scroll.Send(sel("panGestureRecognizer")) == other && objc.Send[bool](view, sel("isDescendantOfView:"), scroll)
}

// transformed reports a pinch's scale or a rotation's angle (value)
// as its recognizer r moves.
func transformed(self, r objc.ID, kind driver.EventKind, value objc.SEL) {
	v := views[self]
	if v == nil {
		return
	}
	var phase driver.GesturePhase
	switch objc.Send[int](r, selState) {
	case stateBegan:
		phase = driver.GestureBegan
	case stateChanged:
		phase = driver.GestureChanged
	case stateEnded:
		phase = driver.GestureEnded
	case stateCancelled, stateFailed:
		phase = driver.GestureCancelled
	default:
		return
	}
	ev := driver.Event{Kind: kind, Phase: phase, Value: float32(objc.Send[float64](r, value)),
		Velocity: driver.Point{X: driver.Points(objc.Send[float64](r, sel("velocity")))}}
	if kind == driver.EventPinch {
		at := objc.Send[apple.CGPoint](r, selLocationInView, self)
		ev.Point = driver.Point{X: driver.Points(at.X), Y: driver.Points(at.Y)}
	}
	v.report(ev)
}

// wheeled follows a scroll-only pan, a trackpad's two fingers or a mouse
// wheel's turns, as wheel events: the translation's steps move the
// content, and the fingers let go at their velocity, which no momentum
// of UIKit's follows.
func wheeled(self objc.ID, _ objc.SEL, r objc.ID) {
	v := views[self]
	if v == nil {
		return
	}
	at := objc.Send[apple.CGPoint](r, selLocationInView, self)
	t := objc.Send[apple.CGPoint](r, sel("translationInView:"), self)
	r.Send(sel("setTranslation:inView:"), apple.CGPoint{}, self)
	ev := driver.Event{Kind: driver.EventWheel, Point: driver.Point{X: driver.Points(at.X), Y: driver.Points(at.Y)},
		Delta: driver.Point{X: driver.Points(t.X), Y: driver.Points(t.Y)}, Precise: r == v.scrolls,
		Modifiers: apple.Modifiers(objc.Send[uint](r, sel("modifierFlags")))}
	switch objc.Send[int](r, selState) {
	case stateBegan:
		ev.Phase = driver.GestureBegan
		if !ev.Precise {
			ev.Phase = driver.GestureChanged
		}
	case stateChanged:
		ev.Phase = driver.GestureChanged
	case stateEnded, stateCancelled:
		if !ev.Precise {
			return
		}
		vel := objc.Send[apple.CGPoint](r, sel("velocityInView:"), self)
		ev.Phase, ev.Velocity = driver.GestureEnded, driver.Point{X: driver.Points(vel.X), Y: driver.Points(vel.Y)}
	default:
		return
	}
	v.report(ev)
}
