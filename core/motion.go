package core

import (
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Keyframes and motions follow NSH's animation::motion: data the core
// turns into Sequence, Loop and Timing trees, which the platform's
// animator runs as any composite, so no platform has code of its own for
// them.

// KeyframeStop is where a value is at a point of a keyframe animation:
// Stops spreads values evenly, StopAt places one, and Eased curves the
// segment that starts at it.
type KeyframeStop struct {
	Value  float64
	offset float64
	placed bool
	easing *Easing
}

// Stops returns values as keyframe stops spread evenly through an
// iteration, the first at its start and the last at its end.
func Stops(values ...float64) []KeyframeStop {
	out := make([]KeyframeStop, len(values))
	for i, v := range values {
		out[i] = KeyframeStop{Value: v}
	}
	return out
}

// StopAt returns value at offset, from 0 to 1, of an iteration.
func StopAt(offset, value float64) KeyframeStop {
	return KeyframeStop{Value: value, offset: offset, placed: true}
}

// Offset returns where s is in an iteration, from 0 to 1, and whether it
// was placed there rather than spread evenly.
func (s KeyframeStop) Offset() (float64, bool) { return s.offset, s.placed }

// Eased returns s with e curving the segment from it to the next stop.
func (s KeyframeStop) Eased(e Easing) KeyframeStop {
	s.easing = &e
	return s
}

// PlaybackDirection is which way the iterations of keyframes play, as
// CSS's animation-direction.
type PlaybackDirection uint8

const (
	PlayNormal PlaybackDirection = iota
	PlayReverse
	PlayAlternate
	PlayAlternateReverse
)

// reversed reports whether iteration i plays backwards.
func (d PlaybackDirection) reversed(i int) bool {
	switch d {
	case PlayReverse:
		return true
	case PlayAlternate:
		return i%2 == 1
	case PlayAlternateReverse:
		return i%2 == 0
	}
	return false
}

// Forever is the Iterations of keyframes that repeat until stopped.
const Forever = -1

// KeyframeTiming is how keyframes play, as CSS's animation-duration,
// -delay, -timing-function, -iteration-count and -direction: Duration (300
// ms when 0) an iteration, Easing for the segments without one of their
// own, Iterations (once when 0, Forever) and the Direction they play in,
// after Delay. During the delay the value already holds the first
// keyframe, as CSS's animation-fill-mode backwards does.
type KeyframeTiming struct {
	Duration   time.Duration
	Delay      time.Duration
	Easing     Easing
	Iterations int
	Direction  PlaybackDirection
}

// resolvedStop is a stop with its offset and easing settled.
type resolvedStop struct {
	offset, value float64
	easing        *Easing
}

// resolveStops places stops as the Web Animations API does: the first at
// 0 and the last at 1 by default (a single one at the end), the ones
// without an offset spread evenly between their neighbors, and an offset
// below the one before held there.
func resolveStops(stops []KeyframeStop) []resolvedStop {
	n := len(stops)
	offsets := make([]float64, n)
	placed := make([]bool, n)
	for i, s := range stops {
		if s.placed && !math.IsNaN(s.offset) && !math.IsInf(s.offset, 0) {
			offsets[i], placed[i] = min(max(s.offset, 0), 1), true
		}
	}
	if n == 1 && !placed[0] {
		offsets[0], placed[0] = 1, true
	} else if n > 1 {
		if !placed[0] {
			offsets[0], placed[0] = 0, true
		}
		if !placed[n-1] {
			offsets[n-1], placed[n-1] = 1, true
		}
	}
	for i := 0; i < n; i++ {
		if placed[i] {
			continue
		}
		start, end := i-1, i
		for !placed[end] {
			end++
		}
		a, b := offsets[start], offsets[end]
		for k := i; k < end; k++ {
			offsets[k] = a + (b-a)*float64(k-start)/float64(end-start)
		}
		i = end
	}
	out := make([]resolvedStop, n)
	last := 0.0
	for i, s := range stops {
		o := max(offsets[i], last)
		last = o
		out[i] = resolvedStop{o, s.Value, s.easing}
	}
	return out
}

// jump moves v to to at once, as a step that ends in the frame it starts.
func jump(v *AnimatedValue, to float64) *Animation {
	return &Animation{Kind: AnimationTiming, Value: v.id, To: to}
}

// iteration is one pass over stops, forwards or backwards: a jump to the
// first stop, or a timing to it from where the value is when it is not at
// 0, then a timing per segment.
func iteration(v *AnimatedValue, stops []resolvedStop, t KeyframeTiming, backwards bool) *Animation {
	d := t.Duration
	type oriented struct {
		offset, value float64
		easing        Easing
	}
	var frames []oriented
	if backwards {
		for i := len(stops) - 1; i >= 0; i-- {
			e := t.Easing
			if i > 0 {
				e = easingOr(stops[i-1].easing, t.Easing).Reversed()
			}
			frames = append(frames, oriented{1 - stops[i].offset, stops[i].value, e})
		}
	} else {
		for _, s := range stops {
			frames = append(frames, oriented{s.offset, s.value, easingOr(s.easing, t.Easing)})
		}
	}
	timing := func(to float64, fraction float64, e Easing) *Animation {
		return &Animation{Kind: AnimationTiming, Value: v.id, To: to,
			Timing: TimingConfig{Duration: time.Duration(fraction * float64(d)), Easing: e}}
	}
	if len(frames) == 0 {
		return Sequence()
	}
	steps := []*Animation{timing(frames[0].value, frames[0].offset, t.Easing)}
	for i := 1; i < len(frames); i++ {
		steps = append(steps, timing(frames[i].value, frames[i].offset-frames[i-1].offset, frames[i-1].easing))
	}
	if last := frames[len(frames)-1].offset; last < 1 {
		steps = append(steps, Delay(time.Duration((1-last)*float64(d))))
	}
	return Sequence(steps...)
}

func easingOr(e *Easing, def Easing) Easing {
	if e != nil {
		return *e
	}
	return def
}

// ValueKeyframes moves v through stops as t says, as CSS keyframes and
// the Web Animations API do: each iteration starts at its first stop, so
// a loop that ends where it starts never jumps, and the value stays where
// the last iteration leaves it.
func ValueKeyframes(v *AnimatedValue, t KeyframeTiming, stops ...KeyframeStop) *Animation {
	if t.Duration <= 0 {
		t.Duration = 300 * time.Millisecond
	}
	frames := resolveStops(stops)
	pass := func(i int) *Animation { return iteration(v, frames, t, t.Direction.reversed(i)) }
	alternates := t.Direction == PlayAlternate || t.Direction == PlayAlternateReverse
	var body *Animation
	switch n := t.Iterations; {
	case n == 0 || n == 1:
		body = pass(0)
	case !alternates && n < 0:
		body = Loop(pass(0), 0)
	case !alternates:
		body = Loop(pass(0), n)
	case n < 0:
		body = Loop(Sequence(pass(0), pass(1)), 0)
	default:
		body = Loop(Sequence(pass(0), pass(1)), n/2)
		if n%2 == 1 {
			body = Sequence(body, pass(0))
		}
	}
	if t.Delay <= 0 {
		return body
	}
	steps := []*Animation{}
	if len(frames) > 0 {
		first := frames[0]
		if t.Direction.reversed(0) {
			last := frames[len(frames)-1]
			first = resolvedStop{1 - last.offset, last.value, nil}
		}
		if first.offset == 0 {
			steps = append(steps, jump(v, first.value))
		}
	}
	return Sequence(append(steps, Delay(t.Delay), body)...)
}

// keyframeEnds returns where stops played as t says start and end.
func keyframeEnds(stops []KeyframeStop, t KeyframeTiming) (start, end float64, ok bool) {
	if len(stops) == 0 {
		return 0, 0, false
	}
	first, last := stops[0].Value, stops[len(stops)-1].Value
	at := func(backwards bool) float64 {
		if backwards {
			return first
		}
		return last
	}
	start = first
	if t.Direction.reversed(0) {
		start = last
	}
	switch n := t.Iterations; {
	case n < 0:
		end = at(t.Direction.reversed(0))
	case n == 0:
		end = at(t.Direction.reversed(0))
	default:
		end = at(t.Direction.reversed(n - 1))
	}
	return start, end, true
}

// MotionChannel is what a motion moves: the opacity of a view and the
// parts of its transform.
type MotionChannel uint8

const (
	MotionOpacity MotionChannel = iota
	MotionTranslateX
	MotionTranslateY
	MotionRotate
	MotionRotateX
	MotionRotateY
	MotionScale
	MotionScaleX
	MotionScaleY
	motionChannels
)

// identity is the channel's value at rest.
func (c MotionChannel) identity() float64 {
	switch c {
	case MotionOpacity, MotionScale, MotionScaleX, MotionScaleY:
		return 1
	}
	return 0
}

// MotionTrack is a channel through keyframe stops.
type MotionTrack struct {
	Channel MotionChannel
	Stops   []KeyframeStop
}

// MotionSpec is several channels of a view through keyframes with one
// timing: a preset's (MotionPreset.Spec) or the app's own.
type MotionSpec struct {
	Tracks []MotionTrack
	Timing KeyframeTiming
}

// Motion moves a view's opacity and transform through MotionSpecs: bind
// its Style, then Play. It has no look of its own: it only moves what the
// app styled.
type Motion struct {
	cx     *Scope
	values [motionChannels]*AnimatedValue
	style  AnimatedStyle
}

// NewMotion returns a motion with every channel at rest, or where enter
// starts, which it then plays: an entrance shows its first frame, never
// the view at rest. The motion's values go with the current owner.
func NewMotion(cx *Scope, enter *MotionSpec) *Motion {
	m := &Motion{cx: cx}
	for c := range motionChannels {
		m.values[c] = NewAnimatedValue(cx, MotionChannel(c).identity())
	}
	if enter != nil {
		for _, tr := range enter.Tracks {
			if start, _, ok := keyframeEnds(tr.Stops, enter.Timing); ok && tr.Channel < motionChannels {
				m.values[tr.Channel].Set(start)
			}
		}
	}
	v := m.values
	m.style = AnimatedStyle{
		Opacity: v[MotionOpacity], TranslateX: v[MotionTranslateX], TranslateY: v[MotionTranslateY],
		Rotate: v[MotionRotate], RotateX: v[MotionRotateX], RotateY: v[MotionRotateY],
		// Scale multiplies each axis's own.
		ScaleX: Mul(v[MotionScale], v[MotionScaleX]), ScaleY: Mul(v[MotionScale], v[MotionScaleY]),
	}
	if enter != nil {
		m.Play(*enter, nil)
	}
	return m
}

// Style returns what binds the motion to a view: its Style.Animated.
func (m *Motion) Style() *AnimatedStyle { return &m.style }

// Value returns the value of channel c, to derive other nodes from.
func (m *Motion) Value(c MotionChannel) *AnimatedValue { return m.values[c] }

// Animation returns spec as an animation of the motion's values, not
// started, to compose: Parallel over its tracks. With reduced, each
// channel lands where it ends at once.
func (m *Motion) Animation(spec MotionSpec, reduced bool) *Animation {
	var steps []*Animation
	for _, tr := range spec.Tracks {
		if tr.Channel >= motionChannels {
			continue
		}
		v := m.values[tr.Channel]
		if reduced {
			if _, end, ok := keyframeEnds(tr.Stops, spec.Timing); ok {
				steps = append(steps, jump(v, end))
			}
			continue
		}
		steps = append(steps, ValueKeyframes(v, spec.Timing, tr.Stops...))
	}
	return Parallel(steps...)
}

// Play runs spec on the motion's values, landing where it ends at once
// when the user asked for reduced motion, and calls done as
// StartAnimation does.
func (m *Motion) Play(spec MotionSpec, done func(finished bool)) *AnimationRun {
	reduced := m.cx.Env().reduce.GetUntracked()
	return m.cx.StartAnimation(m.Animation(spec, reduced), done)
}

// Exit plays spec as the motion's view leaves the Presence branch it is
// in (Scope.OnExit), which keeps the view until spec ends: a sheet or a
// toast built with an exit leaves with it, whatever removes it.
func (m *Motion) Exit(spec MotionSpec) {
	m.cx.OnExit(func(done func()) { m.Play(spec, func(bool) { done() }) })
}

// MotionKind is what a preset is for.
type MotionKind uint8

const (
	// MotionEnter brings a view in: it starts hidden or away and ends at
	// rest.
	MotionEnter MotionKind = iota
	// MotionExit takes a view out: it starts at rest and ends hidden or
	// away.
	MotionExit
	// MotionAttention draws the eye: it starts and ends at rest.
	MotionAttention
	// MotionLoop repeats until stopped.
	MotionLoop
)

// MotionPreset is a ready-made motion, as NSH's: entrances, exits,
// attention seekers and loops. Distances are in points, sized for
// controls and cards.
type MotionPreset uint8

const (
	FadeIn MotionPreset = iota
	FadeInUp
	FadeInDown
	FadeInLeft
	FadeInRight
	ZoomIn
	PopIn
	BounceIn
	SlideInUp
	SlideInDown
	FlipInX
	FlipInY
	FadeOut
	FadeOutUp
	FadeOutDown
	FadeOutLeft
	FadeOutRight
	ZoomOut
	PopOut
	SlideOutDown
	SlideOutUp
	Shake
	HeadShake
	Wiggle
	Swing
	Pulse
	Heartbeat
	RubberBand
	Jello
	Tada
	Jump
	Flash
	Spin
	Float
	Breathe
	Sway
	Bob
	Glow
)

// presetDef is a preset as data.
type presetDef struct {
	name       string
	kind       MotionKind
	ms         int
	easing     Easing
	iterations int
	direction  PlaybackDirection
	tracks     []presetTrack
}

type presetTrack struct {
	channel MotionChannel
	stops   [][2]float64 // offset, value
}

var (
	outExpo   = BezierEasing(Curve{X1: 0.16, Y1: 1, X2: 0.3, Y2: 1})
	inQuart   = BezierEasing(Curve{X1: 0.5, X2: 0.75})
	outBack   = BezierEasing(Curve{X1: 0.34, Y1: 1.56, X2: 0.64, Y2: 1})
	inOutSine = BezierEasing(Curve{X1: 0.37, X2: 0.63, Y2: 1})
	easeOut   = BezierEasing(driver.EaseOut)
	easeIn    = BezierEasing(driver.EaseIn)
	easeInOut = BezierEasing(driver.EaseInOut)
	linear    = BezierEasing(driver.Linear)
)

func track(c MotionChannel, stops ...[2]float64) presetTrack { return presetTrack{c, stops} }

// presets are NSH's PRESETS, in MotionPreset's order.
var presets = [...]presetDef{
	FadeIn:      {"fadeIn", MotionEnter, 300, easeOut, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 0}, [2]float64{1, 1})}},
	FadeInUp:    {"fadeInUp", MotionEnter, 450, outExpo, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 0}, [2]float64{1, 1}), track(MotionTranslateY, [2]float64{0, 24}, [2]float64{1, 0})}},
	FadeInDown:  {"fadeInDown", MotionEnter, 450, outExpo, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 0}, [2]float64{1, 1}), track(MotionTranslateY, [2]float64{0, -24}, [2]float64{1, 0})}},
	FadeInLeft:  {"fadeInLeft", MotionEnter, 450, outExpo, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 0}, [2]float64{1, 1}), track(MotionTranslateX, [2]float64{0, -24}, [2]float64{1, 0})}},
	FadeInRight: {"fadeInRight", MotionEnter, 450, outExpo, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 0}, [2]float64{1, 1}), track(MotionTranslateX, [2]float64{0, 24}, [2]float64{1, 0})}},
	ZoomIn:      {"zoomIn", MotionEnter, 350, outExpo, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 0}, [2]float64{1, 1}), track(MotionScale, [2]float64{0, 0.9}, [2]float64{1, 1})}},
	PopIn:       {"popIn", MotionEnter, 450, outBack, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 0}, [2]float64{0.4, 1}, [2]float64{1, 1}), track(MotionScale, [2]float64{0, 0.5}, [2]float64{1, 1})}},
	BounceIn: {"bounceIn", MotionEnter, 650, BezierEasing(Curve{X1: 0.215, Y1: 0.61, X2: 0.355, Y2: 1}), 1, PlayNormal, []presetTrack{
		track(MotionOpacity, [2]float64{0, 0}, [2]float64{0.6, 1}, [2]float64{1, 1}),
		track(MotionScale, [2]float64{0, 0.3}, [2]float64{0.2, 1.1}, [2]float64{0.4, 0.9}, [2]float64{0.6, 1.03}, [2]float64{0.8, 0.97}, [2]float64{1, 1})}},
	SlideInUp:   {"slideInUp", MotionEnter, 400, outExpo, 1, PlayNormal, []presetTrack{track(MotionTranslateY, [2]float64{0, 64}, [2]float64{1, 0})}},
	SlideInDown: {"slideInDown", MotionEnter, 400, outExpo, 1, PlayNormal, []presetTrack{track(MotionTranslateY, [2]float64{0, -64}, [2]float64{1, 0})}},
	FlipInX: {"flipInX", MotionEnter, 600, easeOut, 1, PlayNormal, []presetTrack{
		track(MotionOpacity, [2]float64{0, 0}, [2]float64{0.6, 1}, [2]float64{1, 1}),
		track(MotionRotateX, [2]float64{0, 90}, [2]float64{0.4, -20}, [2]float64{0.6, 10}, [2]float64{0.8, -5}, [2]float64{1, 0})}},
	FlipInY: {"flipInY", MotionEnter, 600, easeOut, 1, PlayNormal, []presetTrack{
		track(MotionOpacity, [2]float64{0, 0}, [2]float64{0.6, 1}, [2]float64{1, 1}),
		track(MotionRotateY, [2]float64{0, 90}, [2]float64{0.4, -20}, [2]float64{0.6, 10}, [2]float64{0.8, -5}, [2]float64{1, 0})}},
	FadeOut:      {"fadeOut", MotionExit, 250, easeIn, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 1}, [2]float64{1, 0})}},
	FadeOutUp:    {"fadeOutUp", MotionExit, 300, inQuart, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 1}, [2]float64{1, 0}), track(MotionTranslateY, [2]float64{0, 0}, [2]float64{1, -24})}},
	FadeOutDown:  {"fadeOutDown", MotionExit, 300, inQuart, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 1}, [2]float64{1, 0}), track(MotionTranslateY, [2]float64{0, 0}, [2]float64{1, 24})}},
	FadeOutLeft:  {"fadeOutLeft", MotionExit, 300, inQuart, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 1}, [2]float64{1, 0}), track(MotionTranslateX, [2]float64{0, 0}, [2]float64{1, -24})}},
	FadeOutRight: {"fadeOutRight", MotionExit, 300, inQuart, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 1}, [2]float64{1, 0}), track(MotionTranslateX, [2]float64{0, 0}, [2]float64{1, 24})}},
	ZoomOut:      {"zoomOut", MotionExit, 250, inQuart, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 1}, [2]float64{1, 0}), track(MotionScale, [2]float64{0, 1}, [2]float64{1, 0.9})}},
	PopOut: {"popOut", MotionExit, 300, easeIn, 1, PlayNormal, []presetTrack{
		track(MotionOpacity, [2]float64{0, 1}, [2]float64{0.3, 1}, [2]float64{1, 0}),
		track(MotionScale, [2]float64{0, 1}, [2]float64{0.3, 1.08}, [2]float64{1, 0.5})}},
	SlideOutDown: {"slideOutDown", MotionExit, 300, inQuart, 1, PlayNormal, []presetTrack{track(MotionTranslateY, [2]float64{0, 0}, [2]float64{1, 64})}},
	SlideOutUp:   {"slideOutUp", MotionExit, 300, inQuart, 1, PlayNormal, []presetTrack{track(MotionTranslateY, [2]float64{0, 0}, [2]float64{1, -64})}},
	Shake: {"shake", MotionAttention, 500, easeInOut, 1, PlayNormal, []presetTrack{
		track(MotionTranslateX, [2]float64{0, 0}, [2]float64{0.1, -8}, [2]float64{0.3, 8}, [2]float64{0.5, -8}, [2]float64{0.7, 8}, [2]float64{0.9, -4}, [2]float64{1, 0})}},
	HeadShake: {"headShake", MotionAttention, 800, easeInOut, 1, PlayNormal, []presetTrack{
		track(MotionTranslateX, [2]float64{0, 0}, [2]float64{0.065, -6}, [2]float64{0.185, 5}, [2]float64{0.315, -3}, [2]float64{0.435, 2}, [2]float64{0.5, 0}, [2]float64{1, 0}),
		track(MotionRotateY, [2]float64{0, 0}, [2]float64{0.065, -9}, [2]float64{0.185, 7}, [2]float64{0.315, -5}, [2]float64{0.435, 3}, [2]float64{0.5, 0}, [2]float64{1, 0})}},
	Wiggle: {"wiggle", MotionAttention, 600, easeInOut, 1, PlayNormal, []presetTrack{
		track(MotionRotate, [2]float64{0, 0}, [2]float64{0.15, -6}, [2]float64{0.3, 6}, [2]float64{0.45, -4}, [2]float64{0.6, 4}, [2]float64{0.75, -2}, [2]float64{1, 0})}},
	Swing: {"swing", MotionAttention, 900, easeInOut, 1, PlayNormal, []presetTrack{
		track(MotionRotate, [2]float64{0, 0}, [2]float64{0.2, 15}, [2]float64{0.4, -10}, [2]float64{0.6, 5}, [2]float64{0.8, -5}, [2]float64{1, 0})}},
	Pulse: {"pulse", MotionAttention, 500, easeInOut, 1, PlayNormal, []presetTrack{track(MotionScale, [2]float64{0, 1}, [2]float64{0.5, 1.06}, [2]float64{1, 1})}},
	Heartbeat: {"heartbeat", MotionAttention, 900, easeInOut, 1, PlayNormal, []presetTrack{
		track(MotionScale, [2]float64{0, 1}, [2]float64{0.14, 1.25}, [2]float64{0.28, 1}, [2]float64{0.42, 1.25}, [2]float64{0.7, 1}, [2]float64{1, 1})}},
	RubberBand: {"rubberBand", MotionAttention, 800, easeInOut, 1, PlayNormal, []presetTrack{
		track(MotionScaleX, [2]float64{0, 1}, [2]float64{0.3, 1.25}, [2]float64{0.4, 0.75}, [2]float64{0.5, 1.15}, [2]float64{0.65, 0.95}, [2]float64{0.75, 1.05}, [2]float64{1, 1}),
		track(MotionScaleY, [2]float64{0, 1}, [2]float64{0.3, 0.75}, [2]float64{0.4, 1.25}, [2]float64{0.5, 0.85}, [2]float64{0.65, 1.05}, [2]float64{0.75, 0.95}, [2]float64{1, 1})}},
	Jello: {"jello", MotionAttention, 800, easeInOut, 1, PlayNormal, []presetTrack{
		track(MotionScaleX, [2]float64{0, 1}, [2]float64{0.2, 1.12}, [2]float64{0.4, 0.92}, [2]float64{0.6, 1.05}, [2]float64{0.8, 0.98}, [2]float64{1, 1}),
		track(MotionScaleY, [2]float64{0, 1}, [2]float64{0.2, 0.88}, [2]float64{0.4, 1.08}, [2]float64{0.6, 0.95}, [2]float64{0.8, 1.02}, [2]float64{1, 1})}},
	Tada: {"tada", MotionAttention, 900, easeInOut, 1, PlayNormal, []presetTrack{
		track(MotionScale, [2]float64{0, 1}, [2]float64{0.1, 0.9}, [2]float64{0.2, 0.9}, [2]float64{0.3, 1.1}, [2]float64{0.9, 1.1}, [2]float64{1, 1}),
		track(MotionRotate, [2]float64{0, 0}, [2]float64{0.1, -3}, [2]float64{0.2, -3}, [2]float64{0.3, 3}, [2]float64{0.4, -3}, [2]float64{0.5, 3}, [2]float64{0.6, -3}, [2]float64{0.7, 3}, [2]float64{0.8, -3}, [2]float64{0.9, 3}, [2]float64{1, 0})}},
	Jump: {"jump", MotionAttention, 700, easeInOut, 1, PlayNormal, []presetTrack{
		track(MotionTranslateY, [2]float64{0, 0}, [2]float64{0.4, -18}, [2]float64{0.55, -18}, [2]float64{0.8, 0}, [2]float64{1, 0}),
		track(MotionScaleY, [2]float64{0, 1}, [2]float64{0.15, 0.9}, [2]float64{0.4, 1.05}, [2]float64{0.8, 0.92}, [2]float64{1, 1})}},
	Flash:   {"flash", MotionAttention, 800, linear, 1, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 1}, [2]float64{0.25, 0}, [2]float64{0.5, 1}, [2]float64{0.75, 0}, [2]float64{1, 1})}},
	Spin:    {"spin", MotionLoop, 1000, linear, Forever, PlayNormal, []presetTrack{track(MotionRotate, [2]float64{0, 0}, [2]float64{1, 360})}},
	Float:   {"float", MotionLoop, 1500, inOutSine, Forever, PlayAlternate, []presetTrack{track(MotionTranslateY, [2]float64{0, 0}, [2]float64{1, -8})}},
	Breathe: {"breathe", MotionLoop, 1800, inOutSine, Forever, PlayAlternate, []presetTrack{track(MotionScale, [2]float64{0, 1}, [2]float64{1, 1.05}), track(MotionOpacity, [2]float64{0, 1}, [2]float64{1, 0.8})}},
	Sway:    {"sway", MotionLoop, 1600, inOutSine, Forever, PlayAlternate, []presetTrack{track(MotionRotate, [2]float64{0, -4}, [2]float64{1, 4})}},
	Bob:     {"bob", MotionLoop, 1000, linear, Forever, PlayNormal, []presetTrack{track(MotionTranslateY, [2]float64{0, 0}, [2]float64{0.5, -12}, [2]float64{1, 0})}},
	Glow:    {"glow", MotionLoop, 2000, BezierEasing(Curve{X1: 0.4, X2: 0.6, Y2: 1}), Forever, PlayNormal, []presetTrack{track(MotionOpacity, [2]float64{0, 1}, [2]float64{0.5, 0.5}, [2]float64{1, 1})}},
}

// MotionPresets lists every preset, in order.
func MotionPresets() []MotionPreset {
	out := make([]MotionPreset, len(presets))
	for i := range out {
		out[i] = MotionPreset(i)
	}
	return out
}

// Name returns the preset's name, as NSH's JS names it: "fadeIn", "shake".
func (p MotionPreset) Name() string {
	if int(p) < len(presets) {
		return presets[p].name
	}
	return ""
}

// Kind returns what the preset is for.
func (p MotionPreset) Kind() MotionKind {
	if int(p) < len(presets) {
		return presets[p].kind
	}
	return MotionAttention
}

// Spec returns the preset's motion, to play or change.
func (p MotionPreset) Spec() MotionSpec {
	if int(p) >= len(presets) {
		return MotionSpec{}
	}
	d := &presets[p]
	spec := MotionSpec{Timing: KeyframeTiming{Duration: time.Duration(d.ms) * time.Millisecond, Easing: d.easing,
		Iterations: d.iterations, Direction: d.direction}}
	for _, tr := range d.tracks {
		stops := make([]KeyframeStop, len(tr.stops))
		for i, s := range tr.stops {
			stops[i] = StopAt(s[0], s[1])
		}
		spec.Tracks = append(spec.Tracks, MotionTrack{Channel: tr.channel, Stops: stops})
	}
	return spec
}

// StepsEasing returns CSS's steps(count, position): count equal jumps.
func StepsEasing(count int, position StepPosition) Easing {
	return Easing{Kind: driver.EasingSteps, Param: float64(count), Steps: position}
}
