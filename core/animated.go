package core

import (
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Animated values follow NSH's native animation graph (React Native's
// Animated with its native driver): the app builds values, derived nodes
// and animations once, and the platform evaluates them every frame on its
// main thread, so a running animation costs the engine nothing per frame.
// The graph travels in ItemAnimation; each backend runs an Animator.

// AnimatedNode is a value of the animation graph: an AnimatedValue or a
// node derived from others.
type AnimatedNode interface {
	animated() *animNode
	// AnimatedNodeID is the node's ID in the platform's graph, which a
	// Drawing binds arguments to.
	AnimatedNodeID() uint32
}

type animNode struct {
	t     *Tree
	id    uint32
	color bool // it outputs a color
}

func (n *animNode) animated() *animNode { return n }

// AnimatedNodeID implements AnimatedNode.
func (n *animNode) AnimatedNodeID() uint32 { return n.id }

// AnimatedValue is a number the platform animates. Bind it to a view
// through Style.Animated, or derive other nodes from it.
type AnimatedValue struct{ animNode }

// AnimatedXY are the values a drag or a scroll view moves.
type AnimatedXY struct{ X, Y *AnimatedValue }

// NewAnimatedValue returns a value starting at v, released with the
// current owner.
func NewAnimatedValue(cx *Scope, v float64) *AnimatedValue {
	t := cx.tree()
	a := &AnimatedValue{animNode{t: t, id: t.anim.node()}}
	t.anim.add(driver.AnimationOp{Kind: driver.AnimCreateValue, Node: a.id, Value: v})
	t.anim.own(cx, &a.animNode)
	return a
}

// Set jumps the value to v, stopping its animation.
func (v *AnimatedValue) Set(x float64) { v.op(driver.AnimationOp{Kind: driver.AnimSet, Value: x}) }

// SetOffset sets what is added to the value, as a drag's start.
func (v *AnimatedValue) SetOffset(x float64) {
	v.op(driver.AnimationOp{Kind: driver.AnimSetOffset, Value: x})
}

// FlattenOffset adds the offset into the value and resets it to 0.
func (v *AnimatedValue) FlattenOffset() { v.op(driver.AnimationOp{Kind: driver.AnimFlattenOffset}) }

// ExtractOffset moves the value into the offset and resets it to 0.
func (v *AnimatedValue) ExtractOffset() { v.op(driver.AnimationOp{Kind: driver.AnimExtractOffset}) }

// Stop stops the value's animation where it is.
func (v *AnimatedValue) Stop() { v.op(driver.AnimationOp{Kind: driver.AnimStopValue}) }

// Read asks the platform for the value, with its offset, and passes it to
// fn under the current owner.
func (v *AnimatedValue) Read(cx *Scope, fn func(float64)) {
	id := v.t.anim.wait(cx, func(e driver.Event) { fn(float64(e.Value)) })
	v.op(driver.AnimationOp{Kind: driver.AnimRead, ID: id})
}

func (v *AnimatedValue) op(o driver.AnimationOp) {
	o.Node = v.id
	v.t.anim.add(o)
}

// Interpolate returns a node that maps in through r. Its output is a
// color when r has Colors.
func Interpolate(in AnimatedNode, r Interpolation) AnimatedNode {
	src := in.animated()
	n := src.t.anim.derive(src, driver.AnimationOp{Kind: driver.AnimCreateInterpolate, Input: src.id, Interpolation: &r})
	n.animated().color = len(r.Colors) > 0
	return n
}

// DiffClamp follows the changes of in but stays within min and max, as a
// header that hides as content scrolls up and shows as it scrolls down.
func DiffClamp(in AnimatedNode, min, max float64) AnimatedNode {
	src := in.animated()
	return src.t.anim.derive(src, driver.AnimationOp{Kind: driver.AnimCreateDiffClamp, Input: src.id, Min: min, Max: max})
}

// SnappingDiffClamp is DiffClamp that settles at its nearer bound in 200
// ms, easing out, when the scroll view whose offset its input follows
// comes to rest, so a header is never left half hidden. It settles at max
// only once its input reached max; scrolling again ends the settling
// where it is.
func SnappingDiffClamp(in AnimatedNode, min, max float64) AnimatedNode {
	src := in.animated()
	return src.t.anim.derive(src, driver.AnimationOp{Kind: driver.AnimCreateDiffClamp, Input: src.id, Min: min,
		Max: max, Snap: true})
}

// Resist follows in but resists it past min and max as
// RubberBandClamp does, on the frame clock: bound to a view's translation
// with in a value PanValues drags, it slows the drag at its ends with no
// commit per move. in keeps the unresisted value, which a pan's end
// reports to snap back from.
func Resist(in AnimatedNode, min, max, dimension float64) AnimatedNode {
	src := in.animated()
	return src.t.anim.derive(src, driver.AnimationOp{Kind: driver.AnimCreateRubberBand, Input: src.id, Min: min,
		Max: max, Value: dimension})
}

// Constant is a fixed operand for Add, Sub, Mul, Div and Mod.
type Constant float64

func (Constant) animated() *animNode { return nil }

// AnimatedNodeID implements AnimatedNode: a constant is no node.
func (Constant) AnimatedNodeID() uint32 { return 0 }

func operand(n AnimatedNode) (driver.Operand, *animNode) {
	if c, ok := n.(Constant); ok {
		return driver.Operand{Const: float64(c)}, nil
	}
	a := n.animated()
	return driver.Operand{Node: a.id}, a
}

func mathNode(op driver.MathOp, a, b AnimatedNode) AnimatedNode {
	oa, na := operand(a)
	ob, nb := operand(b)
	src := na
	if src == nil {
		src = nb
	}
	if src == nil {
		panic("core: an animated math node needs a node operand")
	}
	return src.t.anim.derive(src, driver.AnimationOp{Kind: driver.AnimCreateMath, Math: op, A: oa, B: ob})
}

// Add returns a + b. One of them may be a Constant; so for Sub, Mul, Div
// and Mod.
func Add(a, b AnimatedNode) AnimatedNode { return mathNode(driver.MathAdd, a, b) }
func Sub(a, b AnimatedNode) AnimatedNode { return mathNode(driver.MathSub, a, b) }
func Mul(a, b AnimatedNode) AnimatedNode { return mathNode(driver.MathMul, a, b) }
func Div(a, b AnimatedNode) AnimatedNode { return mathNode(driver.MathDiv, a, b) }
func Mod(a, b AnimatedNode) AnimatedNode { return mathNode(driver.MathMod, a, b) }

// AnimatedStyle binds a view's properties to animated nodes, over its
// style. The platform writes them every frame they change.
type AnimatedStyle struct {
	Opacity                AnimatedNode
	TranslateX, TranslateY AnimatedNode
	Scale, ScaleX, ScaleY  AnimatedNode
	// Rotate is in degrees, clockwise.
	Rotate AnimatedNode
	// Background, BorderColor and ShadowColor take color
	// interpolations.
	Background, BorderColor, ShadowColor AnimatedNode
	ShadowRadius                         AnimatedNode
	// ShadowOpacity multiplies the shadow color's alpha.
	ShadowOpacity AnimatedNode
	// TextColor colors a text's whole content, its spans included.
	TextColor AnimatedNode
	// RotateX and RotateY turn the view out of its plane, in degrees.
	RotateX, RotateY AnimatedNode
	// StickyInset is how far a sticky view pins from its scroll view's
	// start, in place of its Top (Left across).
	StickyInset AnimatedNode
}

func (s *AnimatedStyle) nodes() [driver.PropCount]uint32 {
	var out [driver.PropCount]uint32
	if s == nil {
		return out
	}
	for i, n := range [...]AnimatedNode{s.Opacity, s.TranslateX, s.TranslateY, s.Scale, s.ScaleX, s.ScaleY,
		s.Rotate, s.Background, s.BorderColor, s.ShadowColor, s.ShadowRadius, s.ShadowOpacity, s.TextColor,
		s.RotateX, s.RotateY} {
		if n != nil {
			if a := n.animated(); a != nil {
				out[i] = a.id
			}
		}
	}
	return out
}

// BezierEasing returns the easing of c.
func BezierEasing(c Curve) Easing { return Easing{Bezier: c} }

// Timing moves v to to over c.Duration.
func Timing(v *AnimatedValue, to float64, c TimingConfig) *Animation {
	if c.Duration == 0 {
		c.Duration = 500 * time.Millisecond
	}
	return &Animation{Kind: AnimationTiming, Value: v.id, To: to, Timing: c}
}

// Spring moves v to to on a damped spring.
func Spring(v *AnimatedValue, to float64, c SpringConfig) *Animation {
	return &Animation{Kind: AnimationSpring, Value: v.id, To: to, Spring: springDefaults(c)}
}

// Decay slows v down from a velocity, as a fling.
func Decay(v *AnimatedValue, c DecayConfig) *Animation {
	return &Animation{Kind: AnimationDecay, Value: v.id, Decay: decayDefaults(c)}
}

// Sequence runs steps one after another, each starting when the one
// before ends.
func Sequence(steps ...*Animation) *Animation {
	return &Animation{Kind: AnimationSequence, Steps: steps}
}

// Parallel runs steps together and ends when all have.
func Parallel(steps ...*Animation) *Animation {
	return &Animation{Kind: AnimationParallel, Steps: steps}
}

// Stagger runs steps together, each starting interval after the one
// before.
func Stagger(interval time.Duration, steps ...*Animation) *Animation {
	return &Animation{Kind: AnimationStagger, Steps: steps, Delay: interval}
}

// Delay waits for d, as a step of a composite.
func Delay(d time.Duration) *Animation { return &Animation{Kind: AnimationDelay, Delay: d} }

// Loop runs a again each time it ends, with its values back at their
// start, iterations times; forever when 0. LoopOn starts each iteration
// where the last ended.
func Loop(a *Animation, iterations int) *Animation {
	return &Animation{Kind: AnimationLoop, Steps: []*Animation{a}, Iterations: iterations}
}

// LoopOn is Loop without putting the values back before each iteration.
func LoopOn(a *Animation, iterations int) *Animation {
	l := Loop(a, iterations)
	l.NoReset = true
	return l
}

// AtFrameRate has a move its values at most rate times a second, on the
// animation StartAnimation runs, and returns it. What loops for as long as
// work lasts, such as a pulsing dot, wants this: a compositor redraws at
// the rate anything on screen moves, however small, and at 120 Hz that
// costs the system far more than the animation shows. Reserve the display's
// full rate for what follows a finger or travels across the window.
func AtFrameRate(a *Animation, rate float64) *Animation {
	a.FrameRate = rate
	return a
}

// AnimationRun is an animation StartAnimation runs on the platform.
type AnimationRun struct {
	t     *Tree
	id    uint32
	ended bool
}

// Stop ends the run where its values are, unfinished.
func (r *AnimationRun) Stop() {
	if !r.ended {
		r.ended = true
		r.t.anim.add(driver.AnimationOp{Kind: driver.AnimStop, ID: r.id})
	}
}

// Reset ends the run, unfinished if it still runs, and puts the values it
// moves back where they were as it started.
func (r *AnimationRun) Reset() {
	r.ended = true
	r.t.anim.add(driver.AnimationOp{Kind: driver.AnimReset, ID: r.id})
}

// StartAnimation runs a on the platform and calls done, if not nil,
// under the current owner with whether it ran to its end, once. Stopping
// or resetting it, a value's Set or Stop, or another animation of one of
// its values ends it early. The owner going stops it.
func (cx *Scope) StartAnimation(a *Animation, done func(finished bool)) *AnimationRun {
	t := cx.tree()
	r := &AnimationRun{t: t}
	reported := false
	r.id = t.anim.wait(cx, func(e driver.Event) {
		r.ended = true
		if done != nil && !reported {
			reported = true
			done(e.Finished)
		}
	})
	t.anim.add(driver.AnimationOp{Kind: driver.AnimStart, ID: r.id, Animation: a})
	cx.current().addCleanup(func() {
		r.Stop()
		t.anim.add(driver.AnimationOp{Kind: driver.AnimForgetRun, ID: r.id})
	})
	return r
}

// SampleEasing returns e at n evenly spaced fractions from 0 to 1, both
// included, as the platform evaluates it: for drawing the curve.
func SampleEasing(e Easing, n int) []float64 { return driver.SampleEasing(e, n) }

// SampleSpring returns where Spring takes a value from from to to, 60
// times a second until it rests, from the platforms' own code.
func SampleSpring(from, to float64, c SpringConfig) []float64 {
	return driver.SampleSpring(from, to, springDefaults(c))
}

// SampleDecay returns where Decay takes a value from from, as
// SampleSpring does.
func SampleDecay(from float64, c DecayConfig) []float64 {
	return driver.SampleDecay(from, decayDefaults(c))
}

func springDefaults(c SpringConfig) SpringConfig {
	if c.Stiffness <= 0 {
		c.Stiffness = 100
	}
	if c.Damping <= 0 {
		c.Damping = 10
	}
	if c.Mass <= 0 {
		c.Mass = 1
	}
	return c
}

// ProjectDecay returns where Decay with c brings a value from from to
// rest: where a fling released at c.Velocity lands, to snap from before
// it starts. A clamped one stops at the bound it passes, whatever its
// bounce.
func ProjectDecay(from float64, c DecayConfig) float64 {
	c = decayDefaults(c)
	// Decay's speed falls as exp(-(1-Deceleration)·t), t in
	// milliseconds, so it covers its speed over that rate in all.
	to := from + c.Velocity/1000/(1-c.Deceleration)
	if c.Clamp {
		to = min(max(to, c.Min), c.Max)
	}
	return to
}

// RubberBandClamp resists x past lo and hi as a scroll view's edge
// resists a drag, with UIKit's constant: past a bound it moves less the
// farther it goes, never more than dimension, the size of what moves.
// Resist is the node that does it on the frame clock.
func RubberBandClamp(x, lo, hi, dimension float64) float64 {
	return driver.RubberBand(x, lo, hi, dimension)
}

func decayDefaults(c DecayConfig) DecayConfig {
	if c.Deceleration <= 0 || c.Deceleration >= 1 {
		c.Deceleration = 0.998
	}
	return c
}

// animations is the engine's side of the graph: node and request IDs,
// the ops for the next batch and those waiting for an answer.
type animations struct {
	t               *Tree
	nodes, requests uint32
	ops             []driver.AnimationOp
	waits           map[uint32]animWait
}

type animWait struct {
	owner *Scope
	fn    func(driver.Event)
}

func (a *animations) node() uint32 { a.nodes++; return a.nodes }

func (a *animations) add(o driver.AnimationOp) {
	a.ops = append(a.ops, o)
	a.t.changed = true
}

// own releases n when cx goes.
func (a *animations) own(cx *Scope, n *animNode) {
	cx.current().addCleanup(func() { a.add(driver.AnimationOp{Kind: driver.AnimRelease, Node: n.id}) })
}

// derive makes a node with op from src, owned by the running owner, else
// by the graph's root.
func (a *animations) derive(src *animNode, op driver.AnimationOp) AnimatedNode {
	t := src.t
	n := &animNode{t: t, id: a.node()}
	op.Node = n.id
	a.add(op)
	owner := t.rt.owner
	if owner == nil {
		owner = t.root
	}
	a.own(owner, n)
	return n
}

// wait registers fn for the EventAnimation answering a new request.
func (a *animations) wait(cx *Scope, fn func(driver.Event)) uint32 {
	a.requests++
	if a.waits == nil {
		a.waits = map[uint32]animWait{}
	}
	a.waits[a.requests] = animWait{owner: cx.current(), fn: fn}
	return a.requests
}

// animationEvent answers a request.
func (t *Tree) animationEvent(ev driver.Event) {
	w, ok := t.anim.waits[ev.Count]
	if !ok {
		return
	}
	delete(t.anim.waits, ev.Count)
	if !w.owner.Disposed() {
		t.within(w.owner, func() { w.fn(ev) })
	}
}

// emitAnimation sends the views' bindings and the graph's changes.
func (t *Tree) emitAnimation() {
	if len(t.anim.ops) == 0 {
		return
	}
	t.items = append(t.items, driver.MountItem{Kind: driver.ItemAnimation, Animation: t.anim.ops})
	t.anim.ops = nil
}

// viewAnimation is what a view has bound.
type viewAnimation struct {
	sent [driver.PropCount]uint32
}

// bindAnimated sends the changes to n's bound properties.
func (t *Tree) bindAnimated(n *node) {
	want := n.style.Animated.nodes()
	if n.anim == nil {
		if want == ([driver.PropCount]uint32{}) {
			return
		}
		n.anim = &viewAnimation{}
	}
	for p, id := range want {
		if id != n.anim.sent[p] {
			n.anim.sent[p] = id
			t.anim.add(driver.AnimationOp{Kind: driver.AnimBind, View: n.id, Prop: driver.AnimatedProp(p), Node: id})
		}
	}
}

func (t *Tree) bindPan(cx *Scope, n *node, xy AnimatedXY) {
	if xy.X == nil && xy.Y == nil {
		return
	}
	t.anim.add(driver.AnimationOp{Kind: driver.AnimBindPan, View: n.id, X: xy.idX(), Y: xy.idY()})
}

// bindGestureValue binds value v to view n's pinches or rotations.
func (t *Tree) bindGestureValue(n *node, kind driver.AnimationOpKind, v *AnimatedValue) {
	if v != nil {
		t.anim.add(driver.AnimationOp{Kind: kind, View: n.id, Node: v.id})
	}
}

func (t *Tree) bindScroll(cx *Scope, n *node, xy AnimatedXY, pull *AnimatedValue) {
	if pull != nil {
		t.anim.add(driver.AnimationOp{Kind: driver.AnimBindPull, View: n.id, Node: pull.id})
	}
	if xy.X == nil && xy.Y == nil {
		return
	}
	t.anim.add(driver.AnimationOp{Kind: driver.AnimBindScroll, View: n.id, X: xy.idX(), Y: xy.idY()})
}

// boundValues are the animated values a view's gestures and scrolling
// set.
type boundValues struct {
	pan, scroll           AnimatedXY
	pinch, rotation, pull *AnimatedValue
}

// rebind binds the values an element's props give view n, sending only
// what changed since its last props: 0 unbinds.
func (t *Tree) rebind(n *node, b boundValues) {
	old := n.bound
	if old == nil {
		old = &boundValues{}
	}
	id := func(v *AnimatedValue) uint32 {
		if v == nil {
			return 0
		}
		return v.id
	}
	if b.pan != old.pan {
		t.anim.add(driver.AnimationOp{Kind: driver.AnimBindPan, View: n.id, X: b.pan.idX(), Y: b.pan.idY()})
	}
	if b.pinch != old.pinch {
		t.anim.add(driver.AnimationOp{Kind: driver.AnimBindPinch, View: n.id, Node: id(b.pinch)})
	}
	if b.rotation != old.rotation {
		t.anim.add(driver.AnimationOp{Kind: driver.AnimBindRotation, View: n.id, Node: id(b.rotation)})
	}
	if b.pull != old.pull {
		t.anim.add(driver.AnimationOp{Kind: driver.AnimBindPull, View: n.id, Node: id(b.pull)})
	}
	if b.scroll != old.scroll {
		t.anim.add(driver.AnimationOp{Kind: driver.AnimBindScroll, View: n.id, X: b.scroll.idX(), Y: b.scroll.idY()})
	}
	if b == (boundValues{}) {
		n.bound = nil
	} else {
		n.bound = &b
	}
}

func (xy AnimatedXY) idX() uint32 {
	if xy.X == nil {
		return 0
	}
	return xy.X.id
}

func (xy AnimatedXY) idY() uint32 {
	if xy.Y == nil {
		return 0
	}
	return xy.Y.id
}

func allLayout(tr driver.LayoutTransition) LayoutAnimation {
	return LayoutAnimation{Create: &tr, Update: &tr, Delete: &tr}
}

// React Native's presets.
var (
	// LayoutEaseInEaseOutPreset eases every change in and out over 300 ms.
	LayoutEaseInEaseOutPreset = allLayout(driver.LayoutTransition{Curve: driver.LayoutEaseInEaseOut, Duration: 300 * time.Millisecond})
	// LayoutLinearPreset moves every change at constant speed over 500 ms.
	LayoutLinearPreset = allLayout(driver.LayoutTransition{Curve: driver.LayoutLinear, Duration: 500 * time.Millisecond})
	// LayoutSpringPreset springs frames (damping 0.4) over 700 ms and fades
	// views linearly.
	LayoutSpringPreset = LayoutAnimation{
		Create: &driver.LayoutTransition{Curve: driver.LayoutLinear, Duration: 700 * time.Millisecond},
		Update: &driver.LayoutTransition{Curve: driver.LayoutSpring, Damping: 0.4, Duration: 700 * time.Millisecond},
		Delete: &driver.LayoutTransition{Curve: driver.LayoutLinear, Duration: 700 * time.Millisecond},
	}
)

// AnimateLayout animates the changes of the next commit that has any
// with a: views move and resize, and appear and go, on the platform.
func (cx *Scope) AnimateLayout(a LayoutAnimation) {
	t := cx.tree()
	t.layoutAnim = &a
	t.changed = true
}
