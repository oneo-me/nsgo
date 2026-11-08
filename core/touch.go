package core

import (
	"slices"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Touches and the touch responder, as NSH's core runs them for Rust
// components (core/touch.rs). The platform reports each finger on the
// surface with EventTouch; the core finds the view under it from the
// frames it laid out and the views around that view that listen, which
// hear the touches from the finger outwards. Then the responder is
// negotiated among them by React Native's rules: captures from the
// outermost in, then from the finger outwards; once there is a
// responder only the views around it are asked, and it may keep the
// touches.

// TouchPhase is what happened to the fingers of a TouchEvent.
type TouchPhase uint8

const (
	// TouchStart is fingers going down (the changed ones).
	TouchStart TouchPhase = iota + 1
	// TouchMove is fingers moving.
	TouchMove
	// TouchEnd is fingers lifting (the changed ones).
	TouchEnd
	// TouchCancel is the system or a native gesture, such as a scroll
	// view, taking the touches.
	TouchCancel
)

// TouchEvent is the fingers on the surface as one of them changes. An
// end or a cancel lists the fingers that lifted too.
type TouchEvent struct {
	Phase TouchPhase
	// Time is since an arbitrary origin.
	Time    time.Duration
	Touches []Touch
	stopped *bool
}

// StopPropagation keeps the views after this one on the path from
// hearing the event.
func (e TouchEvent) StopPropagation() {
	if e.stopped != nil {
		*e.stopped = true
	}
}

// PanGesture follows the centroid of the fingers down, since the first
// went down or since the responder was granted.
type PanGesture struct {
	// Position is the centroid in the surface, Origin where it was as the
	// gesture began, and Delta how far it moved since.
	Position, Origin, Delta Point
	// Velocity is the centroid's, in points per second, smoothed over a
	// few moves.
	Velocity Point
	// Touches counts the fingers down.
	Touches int

	last     Point
	lastTime time.Duration
	lastN    int
}

// centroid returns the centroid of the fingers down during e and their
// number.
func centroid(e *TouchEvent) (Point, int) {
	lifted := e.Phase == TouchEnd || e.Phase == TouchCancel
	var c Point
	n := 0
	for _, t := range e.Touches {
		if !lifted || !t.Changed {
			c.X += t.PagePosition.X
			c.Y += t.PagePosition.Y
			n++
		}
	}
	d := Points(max(n, 1))
	return Point{X: c.X / d, Y: c.Y / d}, n
}

// beginGesture starts a gesture at the fingers of e.
func beginGesture(e *TouchEvent) PanGesture {
	c, n := centroid(e)
	return PanGesture{Position: c, Origin: c, Touches: n, last: c, lastTime: e.Time, lastN: n}
}

// update follows the fingers to e. A finger going down or lifting moves
// the centroid without moving the gesture.
func (g *PanGesture) update(e *TouchEvent) {
	c, n := centroid(e)
	if e.Time == g.lastTime && n == g.lastN {
		return
	}
	g.Touches = n
	if n == g.lastN {
		dt := float32(max(e.Time-g.lastTime, time.Millisecond).Seconds())
		dx, dy := c.X-g.last.X, c.Y-g.last.Y
		g.Delta.X += dx
		g.Delta.Y += dy
		g.Velocity = Point{X: g.Velocity.X*0.3 + dx/Points(dt)*0.7, Y: g.Velocity.Y*0.3 + dy/Points(dt)*0.7}
		g.Position = Point{X: g.Position.X + dx, Y: g.Position.Y + dy}
	}
	g.last, g.lastTime, g.lastN = c, e.Time, n
}

// ResponderEvent is what a responder or a claim hears: the finger the
// event is about and the gesture.
type ResponderEvent struct {
	// Position is in the view hearing it, PagePosition in the surface.
	Position     Point
	PagePosition Point
	Time         time.Duration
	Gesture      PanGesture
}

// TouchClaim is when a view asks to become the touch responder.
type TouchClaim struct {
	// OnMove claims as a finger moves (onMoveShouldSetResponder) rather
	// than as one goes down (onStartShouldSetResponder).
	OnMove bool
	// Capture asks the view before the views inside it.
	Capture bool
	// Ask decides with the gesture whether to claim; nil claims.
	Ask func(ResponderEvent) bool
	// LetNative lets the scroll views around go on taking the touches;
	// by default a responder stops them.
	LetNative bool
	// OnPan claims as the platform activates the view's pan by its
	// PanActivation, deciding on the main thread so the scroll views
	// around share the touch until then. A view with such a claim is the
	// responder exactly while its pan is active: its other claims are not
	// asked.
	OnPan bool
}

// panClaim returns r's claim on its pan, if it has one.
func (r *Responder) panClaim() (TouchClaim, bool) {
	if r != nil {
		for _, c := range r.Claims {
			if c.OnPan {
				return c, true
			}
		}
	}
	return TouchClaim{}, false
}

// Responder makes a view take part in the touch responder, React
// Native's: it claims the touches as its Claims say, and as the
// responder hears where they go.
type Responder struct {
	// Claims are asked in order.
	Claims []TouchClaim
	// Keep refuses to hand the touches to a view that claims them later;
	// OnTerminationRequest, when set, is asked instead each time one does.
	Keep                 bool
	OnTerminationRequest func(ResponderEvent) bool
	// OnGrant: the view became the responder. OnReject: it claimed, but
	// the responder kept the touches.
	OnGrant, OnReject func(ResponderEvent)
	// BlockNative, when set, decides after OnGrant whether the scroll
	// views around stop taking the touches, over the claim's LetNative.
	BlockNative func(ResponderEvent) bool
	// OnStart, OnMove and OnEnd follow the fingers while it is the
	// responder.
	OnStart, OnMove, OnEnd func(ResponderEvent)
	// OnRelease: the last finger lifted. OnTerminate: another view or the
	// system took the touches.
	OnRelease, OnTerminate func(ResponderEvent)
}

// touchState is the touches the views hear and their responder.
type touchState struct {
	// paths are the listening views around each finger down, innermost
	// first.
	paths     map[int][]*node
	responder *node
	gesture   PanGesture
	// blocking is the responder whose scroll views ItemBlockNative last
	// stopped.
	blocking *node
	// last is the last touches, and about the finger they were about,
	// for a pan the platform activates between them.
	last  TouchEvent
	about Touch
}

// hearsTouches reports whether n listens to touches.
func (n *node) hearsTouches() bool {
	return n.h.touch != nil || n.h.touchCapture != nil || n.responder != nil || n.tip != nil
}

// alive reports whether n is still mounted.
func (n *node) alive() bool { return n != nil && !n.deleted && !n.owner.disposed }

// touchPhase is the TouchPhase of a platform's report.
func touchPhase(p GesturePhase) TouchPhase {
	switch p {
	case GestureBegan:
		return TouchStart
	case GestureChanged:
		return TouchMove
	case GestureEnded:
		return TouchEnd
	}
	return TouchCancel
}

// touched handles EventTouch: the views' OnTouch on the finger's path, the
// negotiation, the responder's events.
func (t *Tree) touched(ev driver.Event) {
	s := &t.touches
	e := TouchEvent{Phase: touchPhase(ev.Phase), Time: ev.Time, Touches: ev.Touches}
	lifted := e.Phase == TouchEnd || e.Phase == TouchCancel
	if e.Phase == TouchStart {
		if s.paths == nil {
			s.paths = map[int][]*node{}
		}
		for _, p := range e.Touches {
			if !p.Changed {
				continue
			}
			delete(s.paths, p.ID)
			path := t.touchPath(p.PagePosition)
			if n := t.node(ev.ID); n != nil {
				// A platform that names the view a finger went down on.
				path = t.pathFrom(n)
			}
			if len(path) > 0 {
				s.paths[p.ID] = path
			}
		}
	}
	down := 0
	for _, p := range e.Touches {
		if !lifted || !p.Changed {
			down++
		}
	}
	switch {
	case e.Phase == TouchStart && down == 1:
		s.gesture = beginGesture(&e)
	case e.Phase == TouchStart || e.Phase == TouchMove:
		s.gesture.update(&e)
	}
	i := slices.IndexFunc(e.Touches, func(p Touch) bool { return p.Changed })
	if i < 0 {
		return
	}
	about := e.Touches[i]
	s.last, s.about = e, about
	path := s.paths[about.ID]
	defer t.forgetLifted(&e)
	if len(path) == 0 && s.responder == nil {
		return
	}
	// A finger down hides the tooltips of the views it is on.
	if e.Phase == TouchStart {
		for _, n := range path {
			if n.tip != nil && n.alive() {
				t.within(n.owner, n.tip.hide)
			}
		}
	}
	// Capture from the outermost view in, then bubble out.
	stopped := false
	hear := func(n *node, f func(TouchEvent)) {
		if f != nil && n.alive() && !stopped {
			local := t.localTouches(n, &e)
			local.stopped = &stopped
			t.within(n.owner, func() { f(local) })
		}
	}
	for i := len(path) - 1; i >= 0; i-- {
		hear(path[i], path[i].h.touchCapture)
	}
	for _, n := range path {
		hear(n, n.h.touch)
	}
	switch e.Phase {
	case TouchStart:
		t.negotiate(false, path, about, &e)
	case TouchMove:
		t.negotiate(true, path, about, &e)
	}
	r := s.responder
	if r == nil {
		return
	}
	if e.Phase == TouchCancel {
		t.respond(r, r.responder.OnTerminate, about, &e)
		t.releaseResponder()
		return
	}
	s.gesture.update(&e)
	pick := map[TouchPhase]func(ResponderEvent){TouchStart: r.responder.OnStart, TouchMove: r.responder.OnMove,
		TouchEnd: r.responder.OnEnd}[e.Phase]
	t.respond(r, pick, about, &e)
	if e.Phase == TouchEnd && down == 0 {
		t.releaseResponder()
		t.respond(r, r.responder.OnRelease, about, &e)
	}
}

// touchPath returns the views that listen to touches around the view
// under p, in surface points, innermost first.
func (t *Tree) touchPath(p Point) []*node {
	id, _ := hitWith(t.view, p, func(*node) bool { return true })
	return t.pathFrom(t.node(id))
}

// pathFrom is the views from n out that hear touches.
func (t *Tree) pathFrom(n *node) []*node {
	var path []*node
	for ; n != nil; n = n.nativeParent() {
		if n.hearsTouches() {
			path = append(path, n)
		}
	}
	return path
}

// localTouches returns e with its positions in n.
func (t *Tree) localTouches(n *node, e *TouchEvent) TouchEvent {
	o := t.surfaceBounds(n).Origin
	local := *e
	local.Touches = slices.Clone(e.Touches)
	for i := range local.Touches {
		p := &local.Touches[i]
		p.Position = Point{X: p.PagePosition.X - o.X, Y: p.PagePosition.Y - o.Y}
	}
	return local
}

// forgetLifted drops the paths of the fingers that lifted in e.
func (t *Tree) forgetLifted(e *TouchEvent) {
	if e.Phase != TouchEnd && e.Phase != TouchCancel {
		return
	}
	for _, p := range e.Touches {
		if p.Changed {
			delete(t.touches.paths, p.ID)
		}
	}
}

// responderEvent returns what n hears about finger about.
func (t *Tree) responderEvent(n *node, about Touch, e *TouchEvent) ResponderEvent {
	o := t.surfaceBounds(n).Origin
	return ResponderEvent{Position: Point{X: about.PagePosition.X - o.X, Y: about.PagePosition.Y - o.Y},
		PagePosition: about.PagePosition, Time: e.Time, Gesture: t.touches.gesture}
}

// respond runs fn, one of n's responder handlers, about finger about.
func (t *Tree) respond(n *node, fn func(ResponderEvent), about Touch, e *TouchEvent) {
	if fn != nil && n.alive() {
		ev := t.responderEvent(n, about, e)
		t.within(n.owner, func() { fn(ev) })
	}
}

// keeps reports whether responder r refuses to hand the touches over:
// it Keeps them, or its OnTerminationRequest says no.
func (t *Tree) keeps(r *node, about Touch, e *TouchEvent) bool {
	if r.responder.Keep {
		return true
	}
	ask := r.responder.OnTerminationRequest
	if ask == nil {
		return false
	}
	ev := t.responderEvent(r, about, e)
	allow := true
	t.within(r.owner, func() { allow = ask(ev) })
	return !allow
}

// negotiate asks the views of path whether one claims the touches as they
// start or move: the views around both the finger and the responder, the
// responder left out; captures from the outermost in, then the others
// from the finger outwards.
func (t *Tree) negotiate(move bool, path []*node, about Touch, e *TouchEvent) {
	r := t.touches.responder
	if !r.alive() {
		r = nil
	}
	asked := path
	if r != nil {
		from := 0
		for from < len(path) && !r.within(path[from]) {
			from++
		}
		if from < len(path) && path[from] == r {
			from++
		}
		asked = path[from:]
	}
	var wanting *node
	block := false
	for i := len(asked) - 1; i >= 0 && wanting == nil; i-- {
		if b, ok := t.claims(asked[i], move, true, about, e); ok {
			wanting, block = asked[i], b
		}
	}
	for i := 0; i < len(asked) && wanting == nil; i++ {
		if b, ok := t.claims(asked[i], move, false, about, e); ok {
			wanting, block = asked[i], b
		}
	}
	if wanting == nil || wanting == r {
		return
	}
	if r != nil {
		if t.keeps(r, about, e) {
			t.respond(wanting, wanting.responder.OnReject, about, e)
			return
		}
		t.respond(r, r.responder.OnTerminate, about, e)
	}
	t.grant(wanting, block, about, e)
}

// panActivated makes n, whose pan the platform activated, the responder
// when it claims on its pan.
func (t *Tree) panActivated(n *node) {
	s := &t.touches
	c, ok := n.responder.panClaim()
	if !ok || !n.alive() || n.props.Disabled || s.responder == n || len(s.last.Touches) == 0 {
		return
	}
	if r := s.responder; r.alive() {
		if t.keeps(r, s.about, &s.last) {
			t.respond(n, n.responder.OnReject, s.about, &s.last)
			return
		}
		t.respond(r, r.responder.OnTerminate, s.about, &s.last)
	}
	t.grant(n, !c.LetNative, s.about, &s.last)
}

// within reports whether n is anc or inside it.
func (n *node) within(anc *node) bool {
	for p := n; p != nil; p = p.nativeParent() {
		if p == anc {
			return true
		}
	}
	return false
}

// claims reports whether n claims the touches as they start or move,
// asking its claims of that kind in order, and whether it stops the
// scroll views around.
func (t *Tree) claims(n *node, move, capture bool, about Touch, e *TouchEvent) (block, ok bool) {
	if n.responder == nil || !n.alive() || n.props.Disabled {
		return false, false
	}
	if _, gated := n.responder.panClaim(); gated {
		return false, false
	}
	for _, c := range n.responder.Claims {
		if c.OnMove != move || c.Capture != capture {
			continue
		}
		claimed := c.Ask == nil
		if !claimed {
			ev := t.responderEvent(n, about, e)
			t.within(n.owner, func() { claimed = c.Ask(ev) })
		}
		if claimed {
			return !c.LetNative, true
		}
	}
	return false, false
}

// grant makes n the responder; the gesture starts again there. It stays
// pressed, for its Active style, until the touches end.
func (t *Tree) grant(n *node, block bool, about Touch, e *TouchEvent) {
	s := &t.touches
	if s.responder.alive() {
		t.setState(s.responder, statePressed, false)
	}
	s.responder = n
	s.gesture = beginGesture(e)
	t.setState(n, statePressed, true)
	t.respond(n, n.responder.OnGrant, about, e)
	if f := n.responder.BlockNative; f != nil && n.alive() {
		ev := t.responderEvent(n, about, e)
		t.within(n.owner, func() { block = f(ev) })
	}
	if block {
		t.blockNative(n)
	} else {
		t.blockNative(nil)
	}
}

func (t *Tree) releaseResponder() {
	s := &t.touches
	if s.responder.alive() {
		t.setState(s.responder, statePressed, false)
	}
	s.responder = nil
	t.blockNative(nil)
}

// blockNative stops the scroll views around n from taking the touches,
// or lets them again with nil.
func (t *Tree) blockNative(n *node) {
	if t.touches.blocking == n {
		return
	}
	t.touches.blocking = n
	it := driver.MountItem{Kind: driver.ItemBlockNative}
	if n != nil {
		it.ID = n.id
	}
	t.commands = append(t.commands, it)
	t.changed = true
}

// Gestures the platform recognizes on a view and reports as they happen:
// drags (pans), pinches and rotations. Moves are merged per engine turn,
// so a handler sees the latest state, never a backlog.

// PanEvent is a drag: its translation since it began and its velocity,
// in points and points per second.
type PanEvent struct {
	Phase       GesturePhase
	Translation Point
	Velocity    Point
	// Values are the PanValues as the pan begins and ends: a drag may grab
	// them mid animation, which a snap after it starts from.
	Values Point
}

// GestureEvent is a pinch or a rotation: its amount since it began and
// how fast that changes per second. A pinch's amount is its scale, 1 when
// it begins; a rotation's is an angle in radians, clockwise.
type GestureEvent struct {
	Phase    GesturePhase
	Value    float32
	Velocity float32
	// Position is the gesture's center in the view, where the fingers
	// are or the pointer over a touchpad is; a rotation's is the origin.
	Position Point
}

// WheelEvent is a turn of the mouse wheel or a scroll on a touchpad over a
// view (View.OnWheel). Delta is how far it moves the content, positive
// right and down: a wheel's notch counts its lines at 40 points on macOS,
// iOS and the web, 33 on Windows and Linux, as browsers scroll about 100
// points a notch.
type WheelEvent struct {
	// Position is the pointer's place in the view.
	Position  Point
	Delta     Point
	Modifiers Modifiers
	// Touchpad marks a touch surface's scroll, a Magic Mouse's included:
	// what a canvas pans by where a wheel's notches zoom it.
	Touchpad bool
	// Phase is a touchpad's gesture: GestureBegan as the fingers start,
	// GestureChanged as they move, GestureEnded as they lift. Where the
	// system carries the content on after them, as macOS and the web do,
	// its momentum reports more GestureChanged and a GestureEnded of its
	// own. A wheel's notches are GestureChanged.
	Phase GesturePhase
	// Velocity, on GestureEnded, is how fast the fingers moved the content
	// as they lifted, in points a second, where the platform's momentum
	// does not follow them; the view carries it on itself (Decay).
	Velocity Point
}
