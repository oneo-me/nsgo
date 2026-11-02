package driver

import (
	"math"
	"time"
)

// Interpolation maps a node through piecewise-linear ranges.
type Interpolation struct {
	// Input is increasing; Output (numbers) or Colors has one entry for
	// each.
	Input  []float64
	Output []float64
	Colors []Color
	// ExtrapolateLeft and ExtrapolateRight say what happens outside
	// Input; Extrapolate sets both.
	Extrapolate, ExtrapolateLeft, ExtrapolateRight Extrapolate
	// Easing curves each segment.
	Easing Easing
}

// Extrapolate is what an interpolation does outside its input range; 0
// clamps.
type Extrapolate uint8

const (
	// ExtrapolateClamp keeps the first or last output.
	ExtrapolateClamp Extrapolate = iota + 1
	// ExtrapolateExtend continues the first or last segment.
	ExtrapolateExtend
	// ExtrapolateIdentity returns the input unchanged.
	ExtrapolateIdentity
)

// MathOp is an arithmetic node's operation.
type MathOp uint8

const (
	MathAdd MathOp = iota + 1
	MathSub
	MathMul
	// MathDiv is 0 when b is 0.
	MathDiv
	// MathMod is never negative for a positive b.
	MathMod
)

// Operand is an arithmetic node's operand: another node, or Const when
// Node is 0.
type Operand struct {
	Node  uint32
	Const float64
}

// AnimatedProp is a view property an animated node drives.
type AnimatedProp uint8

const (
	PropOpacity AnimatedProp = iota
	PropTranslateX
	PropTranslateY
	PropScale
	PropScaleX
	PropScaleY
	PropRotate
	PropBackground
	PropBorderColor
	PropShadowColor
	PropShadowRadius
	// PropShadowOpacity multiplies the shadow color's alpha.
	PropShadowOpacity
	// PropTextColor colors a text's whole content, its spans included.
	PropTextColor
	// PropRotateX and PropRotateY turn the view out of its plane, in
	// degrees.
	PropRotateX
	PropRotateY
	// PropCount is the number of animated props.
	PropCount
)

// Easing is how a timing animation or an interpolation segment
// progresses: a cubic Bézier, or one of React Native's curves. Its zero
// value is linear.
type Easing struct {
	Kind EasingKind
	// Mode turns a curve around; Bézier curves ignore it.
	Mode EaseMode
	// Bezier is EasingBezier's curve.
	Bezier Curve
	// Param is Poly's exponent, Elastic's bounciness and Back's
	// overshoot; 0 takes React Native's default. It is Steps's count.
	Param float64
	// Steps is where EasingSteps jumps, as CSS's steps().
	Steps StepPosition
}

// StepPosition is where a steps easing jumps, as CSS's step positions.
type StepPosition uint8

const (
	// JumpEnd holds each step to its end: steps(n, end), the default.
	JumpEnd StepPosition = iota
	// JumpStart jumps at each step's start.
	JumpStart
	// JumpNone holds the first and the last value a step each.
	JumpNone
	// JumpBoth jumps at the start and the end.
	JumpBoth
)

// EasingKind is an easing's shape.
type EasingKind uint8

const (
	EasingBezier EasingKind = iota
	EasingQuad
	EasingCubic
	EasingPoly
	EasingSin
	EasingCircle
	EasingExp
	EasingElastic
	EasingBack
	EasingBounce
	// EasingSteps jumps through Param equal steps, as CSS's steps().
	EasingSteps
)

// EaseMode applies a curve as is, mirrored, or both halves.
type EaseMode uint8

const (
	EaseModeIn EaseMode = iota
	EaseModeOut
	EaseModeInOut
)

// Animation is a description of how values move: Timing, Spring and
// Decay move one value, and Sequence, Parallel, Stagger, Delay and Loop
// compose them. StartAnimation runs it on the platform.
type Animation struct {
	Kind AnimationKind
	// Value is the node a Timing, Spring or Decay moves; To where Timing
	// and Spring take it.
	Value  uint32
	To     float64
	Timing TimingConfig
	Spring SpringConfig
	Decay  DecayConfig
	// Steps are a composite's animations.
	Steps []*Animation
	// Delay is Delay's wait and Stagger's interval.
	Delay time.Duration
	// Iterations is Loop's count; 0 repeats forever. NoReset starts each
	// iteration where the last ended, rather than with its values back
	// at their start.
	Iterations int
	NoReset    bool
}

// AnimationKind is what an Animation does.
type AnimationKind uint8

const (
	AnimationTiming AnimationKind = iota + 1
	AnimationSpring
	AnimationDecay
	AnimationSequence
	AnimationParallel
	AnimationStagger
	AnimationDelay
	AnimationLoop
)

// TimingConfig is a timing animation's: Duration (500 ms when 0) along
// Easing, after Delay.
type TimingConfig struct {
	Duration time.Duration
	Easing   Easing
	Delay    time.Duration
}

// SpringConfig is a damped spring's: Stiffness 100, Damping 10 and Mass 1
// when 0. Velocity, in units per second, replaces the value's current
// velocity when it is not 0.
type SpringConfig struct {
	Stiffness, Damping, Mass float64
	Velocity                 float64
}

// DecayConfig slows a value from Velocity (units per second; the value's
// current velocity when 0) by Deceleration per millisecond (0.998 when
// 0). With Clamp it stays within Min and Max: it stops at a bound, or
// bounces off it keeping Bounce of its speed.
type DecayConfig struct {
	Velocity     float64
	Deceleration float64
	Clamp        bool
	Min, Max     float64
	Bounce       float64
}

// AnimationOpKind is a change to the animation graph.
type AnimationOpKind uint8

const (
	// AnimCreateValue makes value Node at Value.
	AnimCreateValue AnimationOpKind = iota + 1
	// AnimCreateInterpolate makes Node map Input through Interpolation.
	AnimCreateInterpolate
	// AnimCreateMath makes Node combine A and B with Math.
	AnimCreateMath
	// AnimCreateDiffClamp makes Node follow Input's changes within Min
	// and Max; with Snap it settles at its nearer bound when the scroll
	// view its input follows comes to rest.
	AnimCreateDiffClamp
	// AnimSet, AnimSetOffset, AnimFlattenOffset, AnimExtractOffset and
	// AnimStopValue change value Node.
	AnimSet
	AnimSetOffset
	AnimFlattenOffset
	AnimExtractOffset
	AnimStopValue
	// AnimRead answers ID with value Node's value in an EventAnimation.
	AnimRead
	// AnimStart runs Animation as ID; its end is an EventAnimation.
	AnimStart
	// AnimStop ends run ID early.
	AnimStop
	// AnimBind makes Node drive Prop of View; Node 0 unbinds it.
	AnimBind
	// AnimBindPan has drags on View set values X and Y to their
	// translation, and AnimBindScroll has scroll view View set them to its
	// offset.
	AnimBindPan
	AnimBindScroll
	// AnimRelease drops Node.
	AnimRelease
	// AnimBindPinch has pinches on View scale value Node, and
	// AnimBindRotation has rotations on View turn it, in radians.
	AnimBindPinch
	AnimBindRotation
	// AnimReset ends run ID unfinished, if it runs, and puts the values
	// it moves back where they were when it started.
	AnimReset
	// AnimForgetRun drops what the animator keeps of run ID for a reset.
	AnimForgetRun
	// AnimBindPull has vertical scroll view View set value Node to how
	// far it is pulled past its top: 0 at rest, the room kept above the
	// content while it refreshes.
	AnimBindPull
)

// AnimationOp is one change to the animation graph.
type AnimationOp struct {
	Kind          AnimationOpKind
	Node          uint32
	Value         float64
	Input         uint32
	Interpolation *Interpolation
	Math          MathOp
	A, B          Operand
	Min, Max      float64
	// Snap makes a diff clamp settle at a bound when its scroll view
	// rests.
	Snap bool
	// ID names a run or a read.
	ID        uint32
	Animation *Animation
	View      NodeID
	Prop      AnimatedProp
	X, Y      uint32
}

// LayoutAnimation animates the frame changes, insertions and removals of
// one commit on the platform, as React Native's LayoutAnimation does.
// Create animates the views the commit inserts, Update those whose frame
// changes, Delete those it removes; a nil phase leaves its changes
// instant.
type LayoutAnimation struct {
	Create, Update, Delete *LayoutTransition
}

// LayoutTransition is how one kind of change animates.
type LayoutTransition struct {
	Curve LayoutCurve
	// Damping is LayoutSpring's damping ratio, from 0 to 1.
	Damping float32
	// Bezier is LayoutBezier's curve.
	Bezier Curve
	// Property is what appearing and disappearing views animate.
	Property LayoutProperty
	Duration time.Duration
	Delay    time.Duration
}

// LayoutCurve is how a layout change progresses.
type LayoutCurve uint8

const (
	LayoutLinear LayoutCurve = iota
	LayoutEaseIn
	LayoutEaseOut
	LayoutEaseInEaseOut
	// LayoutSpring springs with Damping, settling within the duration.
	LayoutSpring
	// LayoutKeyboard is the software keyboard's own curve, timed from
	// when the keyboard started to move where the platform knows it.
	LayoutKeyboard
	// LayoutBezier follows Bezier, a style transition's easing.
	LayoutBezier
)

// KeyboardCurve is UIKit's keyboard curve, as near as a cubic Bézier
// comes; Android's runtime uses the IME's interpolator instead.
var KeyboardCurve = Curve{0.38, 0.7, 0.125, 1}

// LayoutProperty is what appearing and disappearing views animate.
type LayoutProperty uint8

const (
	// LayoutOpacity fades them.
	LayoutOpacity LayoutProperty = iota
	// LayoutScale grows and shrinks them from their center.
	LayoutScale
)

// At returns the transition's progress at elapsed time since the change,
// and whether it is done. A spring may overshoot 1.
func (tr *LayoutTransition) At(elapsed time.Duration) (float64, bool) {
	x := 1.0
	if tr.Duration > 0 {
		x = float64(elapsed-tr.Delay) / float64(tr.Duration)
	}
	if x <= 0 {
		return 0, false
	}
	if x >= 1 {
		return 1, true
	}
	switch tr.Curve {
	case LayoutEaseIn:
		return EaseIn.At(x), false
	case LayoutEaseOut:
		return EaseOut.At(x), false
	case LayoutEaseInEaseOut:
		return EaseInOut.At(x), false
	case LayoutSpring:
		return layoutSpring(float64(tr.Damping), x), false
	case LayoutKeyboard:
		return KeyboardCurve.At(x), false
	case LayoutBezier:
		return tr.Bezier.At(x), false
	}
	return x, false
}

// End is when the transition is done.
func (tr *LayoutTransition) End() time.Duration { return tr.Delay + tr.Duration }

// layoutSpring is a damped spring from 0 to 1 at x, the fraction of the
// duration, stiff enough to settle by the end, as UIKit fits
// usingSpringWithDamping into a duration (NSH's transition.rs).
func layoutSpring(damping, x float64) float64 {
	const settle = 6.907755 // ln(1000): the envelope falls to 1/1000
	zeta := min(max(damping, 0.05), 1)
	if zeta > 0.999 {
		w := 9.233
		return 1 - (1+w*x)*math.Exp(-w*x)
	}
	w := settle / zeta
	wd := w * math.Sqrt(1-zeta*zeta)
	env := math.Exp(-zeta * w * x)
	return 1 - env*(math.Cos(wd*x)+zeta*w/wd*math.Sin(wd*x))
}

// LerpBounds interpolates a frame from a to b at progress p, never to a
// negative size.
func LerpBounds(a, b Bounds, p float64) Bounds {
	l := func(x, y Points) Points { return x + Points(float64(y-x)*p) }
	return Bounds{
		Origin: Point{l(a.Origin.X, b.Origin.X), l(a.Origin.Y, b.Origin.Y)},
		Size:   Size{max(l(a.Size.Width, b.Size.Width), 0), max(l(a.Size.Height, b.Size.Height), 0)},
	}
}
