package tsx

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
)

// Animation (NSH's animated.rs): AnimatedNode and AnimatedValue are
// classes over core's animated nodes, Animated composes core's
// animations, which the platform runs on its frame clock with no JS per
// frame, and Easing and LayoutAnimation describe curves as data, as NSH's
// do.

// animatedRef is what a JS animated node holds.
type animatedRef struct {
	node  core.AnimatedNode
	value *core.AnimatedValue // set for an AnimatedValue
}

// compositeRef is what a JS CompositeAnimation holds.
type compositeRef struct {
	anim func() *core.Animation
	run  *core.AnimationRun
}

// animClasses are the VM's animation classes.
type animClasses struct {
	key                 *goja.Symbol
	nodeProto, valProto *goja.Object
	nodeClass, valClass *goja.Object
	compProto           *goja.Object
}

func (r *runtime) installAnimated() {
	a := &r.anim
	a.key = goja.NewSymbol("nsgo.animated")
	vm := r.vm

	a.nodeClass = vm.ToValue(func(call goja.ConstructorCall) *goja.Object { return nil }).(*goja.Object)
	a.nodeProto = a.nodeClass.Get("prototype").(*goja.Object)
	a.valClass = vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		v := core.NewAnimatedValue(r.cx, call.Argument(0).ToFloat())
		if !defined(call.Argument(0)) {
			v = core.NewAnimatedValue(r.cx, 0)
		}
		_ = call.This.SetSymbol(a.key, &animatedRef{node: v, value: v})
		return nil
	}).(*goja.Object)
	a.valProto = a.valClass.Get("prototype").(*goja.Object)
	_ = a.valProto.SetPrototype(a.nodeProto)
	_ = a.valClass.SetPrototype(a.nodeClass)

	method := func(proto *goja.Object, name string, f func(this *animatedRef, call goja.FunctionCall) goja.Value) {
		_ = proto.Set(name, r.function(func(call goja.FunctionCall) goja.Value {
			ref := r.animatedRefOf(call.This)
			if ref == nil {
				panic(vm.NewTypeError("nsgo: %s of something that is not an animated node", name))
			}
			return f(ref, call)
		}))
	}
	method(a.nodeProto, "interpolate", func(this *animatedRef, call goja.FunctionCall) goja.Value {
		return r.wrapNode(core.Interpolate(this.node, r.interpolation(arg(call, 0))))
	})
	method(a.nodeProto, "diffClamp", func(this *animatedRef, call goja.FunctionCall) goja.Value {
		return r.diffClamp(this.node, call.Argument(0), call.Argument(1), call.Argument(2))
	})
	for name, op := range map[string]func(a, b core.AnimatedNode) core.AnimatedNode{
		"add": core.Add, "sub": core.Sub, "mul": core.Mul, "div": core.Div, "mod": core.Mod} {
		method(a.nodeProto, name, func(this *animatedRef, call goja.FunctionCall) goja.Value {
			return r.wrapNode(op(this.node, r.operand(arg(call, 0))))
		})
	}

	value := func(name string, f func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value) {
		method(a.valProto, name, func(this *animatedRef, call goja.FunctionCall) goja.Value {
			if this.value == nil {
				panic(vm.NewTypeError("nsgo: %s of an animated node that is not a value", name))
			}
			return f(this.value, call)
		})
	}
	value("set", func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value {
		v.Set(arg(call, 0).ToFloat())
		return goja.Undefined()
	})
	value("stop", func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value {
		v.Stop()
		return r.readValue(v)
	})
	value("getValue", func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value { return r.readValue(v) })
	value("setOffset", func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value {
		v.SetOffset(arg(call, 0).ToFloat())
		return goja.Undefined()
	})
	value("flattenOffset", func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value {
		v.FlattenOffset()
		return goja.Undefined()
	})
	value("extractOffset", func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value {
		v.ExtractOffset()
		return goja.Undefined()
	})
	value("timing", func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value {
		return r.play(core.Timing(v, arg(call, 0).ToFloat(), r.timingConfig(arg(call, 1))))
	})
	value("spring", func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value {
		return r.play(core.Spring(v, arg(call, 0).ToFloat(), r.springConfig(arg(call, 1))))
	})
	value("decay", func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value {
		return r.play(core.Decay(v, r.decayConfig(arg(call, 0))))
	})
	value("keyframes", func(v *core.AnimatedValue, call goja.FunctionCall) goja.Value {
		return r.play(core.ValueKeyframes(v, r.keyframeTiming(arg(call, 1)), r.keyframeStops(arg(call, 0))...))
	})

	a.compProto = vm.NewObject()
	_ = a.compProto.Set("start", r.function(func(call goja.FunctionCall) goja.Value {
		c := r.compositeOf(call.This)
		cb := arg(call, 0)
		p, resolve, _ := vm.NewPromise()
		if c.run != nil {
			c.run.Stop()
		}
		c.run = r.cx.StartAnimation(c.anim(), func(finished bool) {
			r.inJS(func() {
				if isFunction(cb) {
					o := vm.NewObject()
					_ = o.Set("finished", finished)
					r.call(cb, o)
				}
				_ = resolve(finished)
			})
		})
		return vm.ToValue(p)
	}))
	_ = a.compProto.Set("stop", r.function(func(call goja.FunctionCall) goja.Value {
		if c := r.compositeOf(call.This); c.run != nil {
			c.run.Stop()
		}
		return goja.Undefined()
	}))
	_ = a.compProto.Set("reset", r.function(func(call goja.FunctionCall) goja.Value {
		if c := r.compositeOf(call.This); c.run != nil {
			c.run.Reset()
		}
		return goja.Undefined()
	}))
}

// animatedRefOf returns what v holds when it is an animated node.
func (r *runtime) animatedRefOf(v goja.Value) *animatedRef {
	o, ok := v.(*goja.Object)
	if !ok {
		return nil
	}
	v = o.GetSymbol(r.anim.key)
	if v == nil {
		return nil
	}
	ref, _ := v.Export().(*animatedRef)
	return ref
}

func (r *runtime) animatedNodeOf(v goja.Value) core.AnimatedNode {
	if ref := r.animatedRefOf(v); ref != nil {
		return ref.node
	}
	return nil
}

func (r *runtime) animatedValueOf(v goja.Value) *core.AnimatedValue {
	if ref := r.animatedRefOf(v); ref != nil {
		return ref.value
	}
	return nil
}

// wrapNode makes a JS AnimatedNode of n.
func (r *runtime) wrapNode(n core.AnimatedNode) *goja.Object {
	o := r.vm.NewObject()
	_ = o.SetPrototype(r.anim.nodeProto)
	_ = o.SetSymbol(r.anim.key, &animatedRef{node: n})
	return o
}

// wrapValue makes a JS AnimatedValue of v.
func (r *runtime) wrapValue(v *core.AnimatedValue) *goja.Object {
	o := r.vm.NewObject()
	_ = o.SetPrototype(r.anim.valProto)
	_ = o.SetSymbol(r.anim.key, &animatedRef{node: v, value: v})
	return o
}

// operand is a node or a number.
func (r *runtime) operand(v goja.Value) core.AnimatedNode {
	if n := r.animatedNodeOf(v); n != nil {
		return n
	}
	return core.Constant(v.ToFloat())
}

func (r *runtime) diffClamp(n core.AnimatedNode, lo, hi, options goja.Value) goja.Value {
	if r.options(options).bool("snap", false) {
		return r.wrapNode(core.SnappingDiffClamp(n, lo.ToFloat(), hi.ToFloat()))
	}
	return r.wrapNode(core.DiffClamp(n, lo.ToFloat(), hi.ToFloat()))
}

// readValue is a promise of v's value on the platform.
func (r *runtime) readValue(v *core.AnimatedValue) goja.Value {
	p, resolve, _ := r.vm.NewPromise()
	v.Read(r.cx, func(x float64) { r.inJS(func() { _ = resolve(x) }) })
	return r.vm.ToValue(p)
}

// play runs a on the platform: a promise of whether it finished.
func (r *runtime) play(a *core.Animation) goja.Value {
	p, resolve, _ := r.vm.NewPromise()
	r.cx.StartAnimation(a, func(finished bool) { r.inJS(func() { _ = resolve(finished) }) })
	return r.vm.ToValue(p)
}

// inJS runs fn as a call into JS, so the promise jobs it queues run as it
// returns.
func (r *runtime) inJS(fn func()) {
	r.call(r.function(func(goja.FunctionCall) goja.Value {
		fn()
		return goja.Undefined()
	}))
}

// composite makes a CompositeAnimation of what anim makes, made again at
// each start.
func (r *runtime) composite(anim func() *core.Animation) *goja.Object {
	o := r.vm.NewObject()
	_ = o.SetPrototype(r.anim.compProto)
	_ = o.SetSymbol(r.anim.key, &compositeRef{anim: anim})
	return o
}

func (r *runtime) compositeOf(v goja.Value) *compositeRef {
	if o, ok := v.(*goja.Object); ok {
		if s := o.GetSymbol(r.anim.key); s != nil {
			if c, ok := s.Export().(*compositeRef); ok {
				return c
			}
		}
	}
	panic(r.vm.NewTypeError("nsgo: not an animation"))
}

// composites reads a list of CompositeAnimations.
func (r *runtime) composites(v goja.Value) []*compositeRef {
	items := r.arrayItems(v)
	out := make([]*compositeRef, len(items))
	for i, it := range items {
		out[i] = r.compositeOf(it)
	}
	return out
}

func anims(cs []*compositeRef) []*core.Animation {
	out := make([]*core.Animation, len(cs))
	for i, c := range cs {
		out[i] = c.anim()
	}
	return out
}

func millis(v float64) time.Duration { return time.Duration(v * float64(time.Millisecond)) }

func (r *runtime) timingConfig(v goja.Value) core.TimingConfig {
	o := r.options(v)
	c := core.TimingConfig{Duration: millis(o.float("duration", 500)), Delay: millis(o.float("delay", 0))}
	if e, ok := r.easing(o.get("easing")); ok {
		c.Easing = e
	} else {
		c.Easing = easingNames["easeInOut"]
	}
	return c
}

func (r *runtime) springConfig(v goja.Value) core.SpringConfig {
	o := r.options(v)
	return core.SpringConfig{Stiffness: o.float("stiffness", 0), Damping: o.float("damping", 0),
		Mass: o.float("mass", 0), Velocity: o.float("velocity", 0)}
}

func (r *runtime) decayConfig(v goja.Value) core.DecayConfig {
	o := r.options(v)
	c := core.DecayConfig{Velocity: o.float("velocity", 0), Deceleration: o.float("deceleration", 0),
		Bounce: o.float("bounce", 0)}
	if b := r.arrayItems(o.get("clamp")); len(b) == 2 {
		c.Clamp, c.Min, c.Max = true, b[0].ToFloat(), b[1].ToFloat()
	}
	return c
}

// easingNames are the easings by name (NSH's NAMED).
var easingNames = map[string]core.Easing{
	"linear":    {},
	"ease":      core.BezierEasing(core.Curve{X1: 0.25, Y1: 0.1, X2: 0.25, Y2: 1}),
	"easeIn":    core.BezierEasing(core.EaseIn),
	"easeOut":   core.BezierEasing(core.EaseOut),
	"easeInOut": core.BezierEasing(core.EaseInOut),
}

// easingCurves are the curves of `{ curve, param, mode }` with their
// parameter's default.
var easingCurves = map[string]struct {
	kind  core.EasingKind
	param float64
}{
	"quad": {core.EasingQuad, 0}, "cubic": {core.EasingCubic, 0}, "poly": {core.EasingPoly, 2},
	"sin": {core.EasingSin, 0}, "circle": {core.EasingCircle, 0}, "exp": {core.EasingExp, 0},
	"elastic": {core.EasingElastic, 1}, "back": {core.EasingBack, 1.70158}, "bounce": {core.EasingBounce, 0},
}

var stepPositions = map[string]core.StepPosition{"jump-start": core.JumpStart, "start": core.JumpStart,
	"jump-end": core.JumpEnd, "end": core.JumpEnd, "jump-none": core.JumpNone, "jump-both": core.JumpBoth}

// easing decodes a name, [x1, y1, x2, y2], { curve, param, mode } or
// { steps, position }; false for anything else.
func (r *runtime) easing(v goja.Value) (core.Easing, bool) {
	if !defined(v) {
		return core.Easing{}, false
	}
	if items := r.arrayItems(v); items != nil {
		p := func(i int) float32 {
			if i < len(items) {
				return float32(items[i].ToFloat())
			}
			return 0
		}
		return core.BezierEasing(core.Curve{X1: p(0), Y1: p(1), X2: p(2), Y2: p(3)}), true
	}
	if o, ok := v.(*goja.Object); ok {
		op := opts{o}
		if op.has("steps") {
			n := int(min(max(op.float("steps", 1), 1), 10000))
			return core.StepsEasing(n, stepPositions[op.str("position", "jump-end")]), true
		}
		spec, ok := easingCurves[op.str("curve", "")]
		if !ok {
			return core.Easing{}, false
		}
		mode := map[string]core.EaseMode{"out": core.EaseModeOut, "inOut": core.EaseModeInOut}[op.str("mode", "in")]
		return core.Easing{Kind: spec.kind, Mode: mode, Param: op.float("param", spec.param)}, true
	}
	e, ok := easingNames[v.String()]
	return e, ok
}

// interpolation decodes interpolate's config.
func (r *runtime) interpolation(v goja.Value) core.Interpolation {
	o := r.options(v)
	var in core.Interpolation
	for _, x := range r.arrayItems(o.get("inputRange")) {
		in.Input = append(in.Input, x.ToFloat())
	}
	for _, y := range r.arrayItems(o.get("outputRange")) {
		if s, ok := y.Export().(string); ok {
			if c, err := core.ParseColor(s); err == nil {
				in.Colors = append(in.Colors, c)
				continue
			}
			in.Output = append(in.Output, angleDegrees(s))
			continue
		}
		in.Output = append(in.Output, y.ToFloat())
	}
	ex := map[string]core.Extrapolate{"extend": core.ExtrapolateExtend, "clamp": core.ExtrapolateClamp,
		"identity": core.ExtrapolateIdentity}
	in.Extrapolate = ex[o.str("extrapolate", "extend")]
	in.ExtrapolateLeft, in.ExtrapolateRight = ex[o.str("extrapolateLeft", "")], ex[o.str("extrapolateRight", "")]
	if e, ok := r.easing(o.get("easing")); ok {
		in.Easing = e
	}
	return in
}

// angleDegrees reads an angle ("45deg", "0.5turn", "1rad", "100grad") or a
// number in degrees.
func angleDegrees(s string) float64 {
	s = strings.TrimSpace(s)
	for _, u := range []struct {
		suffix string
		scale  float64
	}{{"deg", 1}, {"grad", 0.9}, {"rad", 180 / math.Pi}, {"turn", 360}} {
		if n, ok := strings.CutSuffix(s, u.suffix); ok {
			f, _ := strconv.ParseFloat(n, 64)
			return f * u.scale
		}
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// keyframeValue reads a keyframe's value: a number or an angle.
func keyframeValue(v goja.Value) (float64, bool) {
	if !defined(v) {
		return 0, false
	}
	if s, ok := v.Export().(string); ok {
		return angleDegrees(s), true
	}
	f := v.ToFloat()
	return f, !math.IsNaN(f) && !math.IsInf(f, 0)
}

// keyframeStops reads keyframes: numbers spread evenly, or { offset?,
// value, easing? }.
func (r *runtime) keyframeStops(v goja.Value) []core.KeyframeStop {
	items := r.arrayItems(v)
	var out []core.KeyframeStop
	for i, it := range items {
		var at float64
		if len(items) > 1 {
			at = float64(i) / float64(len(items)-1)
		} else {
			at = 1
		}
		if o, ok := it.(*goja.Object); ok {
			op := opts{o}
			x, ok := keyframeValue(op.get("value"))
			if !ok {
				continue
			}
			s := core.StopAt(op.float("offset", at), x)
			if e, ok := r.easing(op.get("easing")); ok {
				s = s.Eased(e)
			}
			out = append(out, s)
			continue
		}
		if x, ok := keyframeValue(it); ok {
			out = append(out, core.StopAt(at, x))
		}
	}
	return out
}

// keyframeTiming reads { duration, delay, easing, iterations, direction },
// with CSS's defaults.
func (r *runtime) keyframeTiming(v goja.Value) core.KeyframeTiming {
	o := r.options(v)
	t := core.KeyframeTiming{Duration: millis(max(o.float("duration", 300), 0)), Delay: millis(max(o.float("delay", 0), 0))}
	if e, ok := r.easing(o.get("easing")); ok {
		t.Easing = e
	} else {
		t.Easing = easingNames["ease"]
	}
	switch it := o.get("iterations"); {
	case !defined(it):
	case it.String() == "infinite" || math.IsInf(it.ToFloat(), 1):
		t.Iterations = core.Forever
	default:
		t.Iterations = int(math.Round(max(it.ToFloat(), 0)))
	}
	t.Direction = map[string]core.PlaybackDirection{"reverse": core.PlayReverse, "alternate": core.PlayAlternate,
		"alternate-reverse": core.PlayAlternateReverse}[o.str("direction", "normal")]
	return t
}

// motionChannels are the channels a motion moves, by name.
var motionChannels = map[string]core.MotionChannel{"opacity": core.MotionOpacity,
	"translateX": core.MotionTranslateX, "translateY": core.MotionTranslateY, "rotate": core.MotionRotate,
	"rotateX": core.MotionRotateX, "rotateY": core.MotionRotateY, "scale": core.MotionScale,
	"scaleX": core.MotionScaleX, "scaleY": core.MotionScaleY}

// channelOrder is the channels in NSH's order.
var channelOrder = []string{"opacity", "translateX", "translateY", "rotate", "rotateX", "rotateY", "scale",
	"scaleX", "scaleY"}

// motionSpec reads a preset's name or { keyframes, ...timing }, keyframes
// being channels' keyframes or a list of frames of channels.
func (r *runtime) motionSpec(v goja.Value) (core.MotionSpec, bool) {
	if !defined(v) {
		return core.MotionSpec{}, false
	}
	if s, ok := v.Export().(string); ok {
		for _, p := range core.MotionPresets() {
			if p.Name() == s {
				return p.Spec(), true
			}
		}
		return core.MotionSpec{}, false
	}
	o, ok := v.(*goja.Object)
	if !ok {
		return core.MotionSpec{}, false
	}
	spec := core.MotionSpec{Timing: r.keyframeTiming(o)}
	frames := o.Get("keyframes")
	if items := r.arrayItems(frames); items != nil {
		for _, name := range channelOrder {
			var stops []core.KeyframeStop
			for i, it := range items {
				f, ok := it.(*goja.Object)
				if !ok {
					continue
				}
				x, ok := keyframeValue(f.Get(name))
				if !ok {
					continue
				}
				at := 1.0
				if len(items) > 1 {
					at = float64(i) / float64(len(items)-1)
				}
				s := core.StopAt(opts{f}.float("offset", at), x)
				if e, ok := r.easing(f.Get("easing")); ok {
					s = s.Eased(e)
				}
				stops = append(stops, s)
			}
			if len(stops) > 0 {
				spec.Tracks = append(spec.Tracks, core.MotionTrack{Channel: motionChannels[name], Stops: stops})
			}
		}
	} else if f, ok := frames.(*goja.Object); ok {
		for _, name := range f.Keys() {
			c, ok := motionChannels[name]
			if !ok {
				continue
			}
			if stops := r.keyframeStops(f.Get(name)); len(stops) > 0 {
				spec.Tracks = append(spec.Tracks, core.MotionTrack{Channel: c, Stops: stops})
			}
		}
	} else {
		return core.MotionSpec{}, false
	}
	return spec, true
}

// withTiming lays play options' timing over spec's.
func (r *runtime) withTiming(spec core.MotionSpec, options goja.Value) core.MotionSpec {
	o := r.options(options)
	if o.o == nil {
		return spec
	}
	t := r.keyframeTiming(options)
	if o.has("duration") {
		spec.Timing.Duration = t.Duration
	}
	if o.has("delay") {
		spec.Timing.Delay = t.Delay
	}
	if o.has("easing") {
		spec.Timing.Easing = t.Easing
	}
	if o.has("iterations") {
		spec.Timing.Iterations = t.Iterations
	}
	if o.has("direction") {
		spec.Timing.Direction = t.Direction
	}
	return spec
}

// animatedExports are the animation API of nsgo.
var animatedExports = []export{
	{"AnimatedNode", func(r *runtime) goja.Value { return r.anim.nodeClass }},
	{"AnimatedValue", func(r *runtime) goja.Value { return r.anim.valClass }},
	fn("createAnimatedValue", func(r *runtime, call goja.FunctionCall) goja.Value {
		x := 0.0
		if defined(arg(call, 0)) {
			x = arg(call, 0).ToFloat()
		}
		return r.wrapValue(core.NewAnimatedValue(r.cx, x))
	}),
	fn("createMotion", (*runtime).createMotion),
	{"motions", (*runtime).motions},
	{"Animated", (*runtime).animatedModule},
	{"Easing", (*runtime).easingModule},
	{"LayoutAnimation", (*runtime).layoutAnimationModule},
}

func (r *runtime) animatedModule() goja.Value {
	m := r.vm.NewObject()
	set := func(name string, f func(call goja.FunctionCall) goja.Value) { _ = m.Set(name, r.function(f)) }
	value := func(v goja.Value) *core.AnimatedValue {
		x := r.animatedValueOf(v)
		if x == nil {
			panic(r.vm.NewTypeError("nsgo: not an AnimatedValue"))
		}
		return x
	}
	set("timing", func(call goja.FunctionCall) goja.Value {
		v, cfg := value(arg(call, 0)), arg(call, 1)
		to, c := r.options(cfg).float("toValue", 0), r.timingConfig(cfg)
		return r.composite(func() *core.Animation { return core.Timing(v, to, c) })
	})
	set("spring", func(call goja.FunctionCall) goja.Value {
		v, cfg := value(arg(call, 0)), arg(call, 1)
		to, c := r.options(cfg).float("toValue", 0), r.springConfig(cfg)
		return r.composite(func() *core.Animation { return core.Spring(v, to, c) })
	})
	set("decay", func(call goja.FunctionCall) goja.Value {
		v, c := value(arg(call, 0)), r.decayConfig(arg(call, 1))
		return r.composite(func() *core.Animation { return core.Decay(v, c) })
	})
	set("keyframes", func(call goja.FunctionCall) goja.Value {
		v, cfg := value(arg(call, 0)), arg(call, 1)
		t, stops := r.keyframeTiming(cfg), r.keyframeStops(r.options(cfg).get("keyframes"))
		return r.composite(func() *core.Animation { return core.ValueKeyframes(v, t, stops...) })
	})
	set("sequence", func(call goja.FunctionCall) goja.Value {
		cs := r.composites(arg(call, 0))
		return r.composite(func() *core.Animation { return core.Sequence(anims(cs)...) })
	})
	set("parallel", func(call goja.FunctionCall) goja.Value {
		cs := r.composites(arg(call, 0))
		return r.composite(func() *core.Animation { return core.Parallel(anims(cs)...) })
	})
	set("stagger", func(call goja.FunctionCall) goja.Value {
		d, cs := millis(arg(call, 0).ToFloat()), r.composites(arg(call, 1))
		from := r.options(arg(call, 2)).get("from")
		return r.composite(func() *core.Animation {
			steps := anims(cs)
			start := 0.0
			switch {
			case !defined(from) || from.String() == "first":
				return core.Stagger(d, steps...)
			case from.String() == "last":
				start = float64(len(steps) - 1)
			case from.String() == "center":
				start = float64(len(steps)-1) / 2
			default:
				start = max(from.ToFloat(), 0)
			}
			out := make([]*core.Animation, len(steps))
			for i, s := range steps {
				out[i] = core.Sequence(core.Delay(time.Duration(math.Abs(float64(i)-start)*float64(d))), s)
			}
			return core.Parallel(out...)
		})
	})
	set("delay", func(call goja.FunctionCall) goja.Value {
		d := millis(arg(call, 0).ToFloat())
		return r.composite(func() *core.Animation { return core.Delay(d) })
	})
	set("loop", func(call goja.FunctionCall) goja.Value {
		c, o := r.compositeOf(arg(call, 0)), r.options(arg(call, 1))
		n := -1
		if it := o.float("iterations", -1); it >= 0 {
			n = int(it)
		}
		if !o.bool("resetBeforeIteration", true) {
			return r.composite(func() *core.Animation { return core.LoopOn(c.anim(), n) })
		}
		return r.composite(func() *core.Animation { return core.Loop(c.anim(), n) })
	})
	for name, op := range map[string]func(a, b core.AnimatedNode) core.AnimatedNode{
		"add": core.Add, "subtract": core.Sub, "multiply": core.Mul, "divide": core.Div, "modulo": core.Mod} {
		set(name, func(call goja.FunctionCall) goja.Value {
			return r.wrapNode(op(r.operand(arg(call, 0)), r.operand(arg(call, 1))))
		})
	}
	set("diffClamp", func(call goja.FunctionCall) goja.Value {
		return r.diffClamp(r.operand(arg(call, 0)), call.Argument(1), call.Argument(2), call.Argument(3))
	})
	set("sampleSpring", func(call goja.FunctionCall) goja.Value {
		o := r.options(arg(call, 0))
		return r.vm.ToValue(core.SampleSpring(o.float("from", 0), o.float("toValue", 0), r.springConfig(arg(call, 0))))
	})
	set("sampleDecay", func(call goja.FunctionCall) goja.Value {
		o := r.options(arg(call, 0))
		return r.vm.ToValue(core.SampleDecay(o.float("from", 0), r.decayConfig(arg(call, 0))))
	})
	return m
}

func (r *runtime) easingModule() goja.Value {
	m := r.vm.NewObject()
	for _, name := range []string{"linear", "ease"} {
		_ = m.Set(name, name)
	}
	curve := func(name string, param any) *goja.Object {
		o := r.vm.NewObject()
		_ = o.Set("curve", name)
		if param != nil {
			_ = o.Set("param", param)
		}
		_ = o.Set("mode", "in")
		return o
	}
	for _, name := range []string{"quad", "cubic", "sin", "circle", "exp", "bounce"} {
		_ = m.Set(name, curve(name, nil))
	}
	for _, name := range []string{"poly", "elastic", "back"} {
		def := easingCurves[name].param
		_ = m.Set(name, r.function(func(call goja.FunctionCall) goja.Value {
			p := def
			if defined(arg(call, 0)) {
				p = arg(call, 0).ToFloat()
			}
			return curve(name, p)
		}))
	}
	_ = m.Set("bezier", r.function(func(call goja.FunctionCall) goja.Value {
		return r.vm.NewArray(arg(call, 0), arg(call, 1), arg(call, 2), arg(call, 3))
	}))
	_ = m.Set("steps", r.function(func(call goja.FunctionCall) goja.Value {
		o := r.vm.NewObject()
		_ = o.Set("steps", arg(call, 0))
		if defined(arg(call, 1)) {
			_ = o.Set("position", arg(call, 1))
		}
		return o
	}))
	for _, mode := range []string{"in", "out", "inOut"} {
		_ = m.Set(mode, r.function(func(call goja.FunctionCall) goja.Value {
			e := arg(call, 0)
			if items := r.arrayItems(e); items != nil && mode == "out" && len(items) == 4 {
				f := func(i int) float64 { return 1 - items[i].ToFloat() }
				return r.vm.NewArray(f(2), f(3), f(0), f(1))
			}
			if o, ok := e.(*goja.Object); ok && r.arrayItems(e) == nil {
				c := r.vm.NewObject()
				for _, k := range o.Keys() {
					_ = c.Set(k, o.Get(k))
				}
				_ = c.Set("mode", mode)
				return c
			}
			if mode == "out" {
				switch e.String() {
				case "easeIn":
					return r.vm.ToValue("easeOut")
				case "easeOut":
					return r.vm.ToValue("easeIn")
				}
			}
			return e
		}))
	}
	_ = m.Set("sample", r.function(func(call goja.FunctionCall) goja.Value {
		e, _ := r.easing(arg(call, 0))
		return r.vm.ToValue(core.SampleEasing(e, int(arg(call, 1).ToInteger())))
	}))
	return m
}

var layoutCurves = map[string]core.LayoutCurve{"easeInEaseOut": core.LayoutEaseInEaseOut,
	"spring": core.LayoutSpring, "linear": core.LayoutLinear, "easeIn": core.LayoutEaseIn,
	"easeOut": core.LayoutEaseOut, "keyboard": core.LayoutKeyboard}

var layoutProperties = map[string]core.LayoutProperty{"opacity": core.LayoutOpacity,
	"scaleXY": core.LayoutScale, "scale": core.LayoutScale}

// layoutAnimation decodes { duration, create, update, delete }.
func (r *runtime) layoutAnimation(v goja.Value) core.LayoutAnimation {
	o := r.options(v)
	duration := o.float("duration", 0)
	phase := func(k string) *core.LayoutTransition {
		p := r.options(o.get(k))
		if p.o == nil {
			return nil
		}
		return &core.LayoutTransition{
			Curve:    layoutCurves[p.str("type", "easeInEaseOut")],
			Damping:  float32(p.float("springDamping", 0)),
			Property: layoutProperties[p.str("property", "opacity")],
			Duration: millis(p.float("duration", duration)),
			Delay:    millis(p.float("delay", 0)),
		}
	}
	return core.LayoutAnimation{Create: phase("create"), Update: phase("update"), Delete: phase("delete")}
}

// layoutPreset is { duration, create, update, delete } of NSH's presets.
func (r *runtime) layoutPreset(duration float64, kind, property string) *goja.Object {
	phase := func(withProperty bool) *goja.Object {
		p := r.vm.NewObject()
		_ = p.Set("type", kind)
		if withProperty {
			_ = p.Set("property", property)
		}
		if kind == "spring" {
			_ = p.Set("springDamping", 0.7)
		}
		return p
	}
	o := r.vm.NewObject()
	_ = o.Set("duration", duration)
	_ = o.Set("create", phase(true))
	_ = o.Set("update", phase(false))
	_ = o.Set("delete", phase(true))
	return o
}

func (r *runtime) layoutAnimationModule() goja.Value {
	m := r.vm.NewObject()
	_ = m.Set("configureNext", r.function(func(call goja.FunctionCall) goja.Value {
		r.cx.AnimateLayout(r.layoutAnimation(arg(call, 0)))
		if end := arg(call, 1); isFunction(end) {
			d := r.options(arg(call, 0)).float("duration", 0)
			r.cx.AfterFunc(millis(d), func() { r.call(end) })
		}
		return goja.Undefined()
	}))
	_ = m.Set("create", r.function(func(call goja.FunctionCall) goja.Value {
		kind, property := "easeInEaseOut", "opacity"
		if defined(arg(call, 1)) {
			kind = arg(call, 1).String()
		}
		if defined(arg(call, 2)) {
			property = arg(call, 2).String()
		}
		return r.layoutPreset(arg(call, 0).ToFloat(), kind, property)
	}))
	names := func(list ...string) *goja.Object {
		o := r.vm.NewObject()
		for _, n := range list {
			_ = o.Set(n, n)
		}
		return o
	}
	_ = m.Set("Types", names("easeInEaseOut", "spring", "linear", "easeIn", "easeOut", "keyboard"))
	_ = m.Set("Properties", names("opacity", "scaleXY"))
	presets := map[string]*goja.Object{
		"easeInEaseOut": r.layoutPreset(300, "easeInEaseOut", "opacity"),
		"linear":        r.layoutPreset(500, "linear", "opacity"),
		"spring":        r.layoutPreset(700, "spring", "opacity"),
	}
	p := r.vm.NewObject()
	for _, name := range []string{"easeInEaseOut", "linear", "spring"} {
		_ = p.Set(name, presets[name])
		preset := presets[name]
		_ = m.Set(name, r.function(func(call goja.FunctionCall) goja.Value {
			return r.call(m.Get("configureNext"), preset, arg(call, 0))
		}))
	}
	_ = m.Set("Presets", p)
	return m
}

// motions are the presets as data: { kind, keyframes, ...timing }.
func (r *runtime) motions() goja.Value {
	m := r.vm.NewObject()
	kinds := map[core.MotionKind]string{core.MotionEnter: "enter", core.MotionExit: "exit",
		core.MotionAttention: "attention", core.MotionLoop: "loop"}
	for _, p := range core.MotionPresets() {
		spec := p.Spec()
		o := r.vm.NewObject()
		_ = o.Set("kind", kinds[p.Kind()])
		frames := r.vm.NewObject()
		for _, t := range spec.Tracks {
			for name, c := range motionChannels {
				if c == t.Channel {
					values := make([]any, len(t.Stops))
					for i, s := range t.Stops {
						values[i] = s.Value
					}
					_ = frames.Set(name, values)
				}
			}
		}
		_ = o.Set("keyframes", frames)
		_ = o.Set("duration", spec.Timing.Duration.Milliseconds())
		if spec.Timing.Iterations == core.Forever {
			_ = o.Set("iterations", "infinite")
		}
		_ = m.Set(p.Name(), o)
	}
	_ = m.Get("fadeIn") // presets are read-only data
	return m
}

// createMotion is `createMotion({ initial, enter, ...play options })`.
func (r *runtime) createMotion(call goja.FunctionCall) goja.Value {
	opt := arg(call, 0)
	o := r.options(opt)
	var enter *core.MotionSpec
	if s, ok := r.motionSpec(o.get("enter")); ok {
		s = r.withTiming(s, opt)
		enter = &s
	}
	m := core.NewMotion(r.cx, enter)
	if init := r.options(o.get("initial")); init.o != nil {
		for _, k := range init.o.Keys() {
			if c, ok := motionChannels[k]; ok {
				m.Value(c).Set(init.float(k, 0))
			}
		}
	}
	obj := r.vm.NewObject()
	values := map[string]*goja.Object{}
	for _, name := range channelOrder {
		values[name] = r.wrapValue(m.Value(motionChannels[name]))
		_ = obj.Set(name, values[name])
	}
	scale := func(axis string) goja.Value {
		return r.wrapNode(core.Mul(m.Value(core.MotionScale), m.Value(motionChannels[axis])))
	}
	step := func(k string, v goja.Value) *goja.Object {
		s := r.vm.NewObject()
		_ = s.Set(k, v)
		return s
	}
	transform := r.vm.NewArray(step("translateX", values["translateX"]), step("translateY", values["translateY"]),
		step("rotate", values["rotate"]), step("rotateX", values["rotateX"]), step("rotateY", values["rotateY"]),
		step("scaleX", scale("scaleX")), step("scaleY", scale("scaleY")))
	style := r.vm.NewObject()
	_ = style.Set("opacity", values["opacity"])
	_ = style.Set("transform", transform)
	_ = obj.Set("style", style)
	_ = obj.Set("transform", transform)
	var run *core.AnimationRun
	spec := func(v, options goja.Value) core.MotionSpec {
		s, ok := r.motionSpec(v)
		if !ok {
			panic(r.vm.NewTypeError("nsgo: no motion %s", v))
		}
		return r.withTiming(s, options)
	}
	reduced := func(options goja.Value) bool {
		return !r.options(options).bool("ignoreReduceMotion", o.bool("ignoreReduceMotion", false)) &&
			core.Untracked(r.cx, func() bool { return r.cx.Env().ReduceMotion() })
	}
	_ = obj.Set("play", r.function(func(call goja.FunctionCall) goja.Value {
		s, options := spec(arg(call, 0), arg(call, 1)), arg(call, 1)
		if run != nil {
			run.Stop()
		}
		p, resolve, _ := r.vm.NewPromise()
		run = r.cx.StartAnimation(m.Animation(s, reduced(options)), func(finished bool) {
			r.inJS(func() { _ = resolve(finished) })
		})
		return r.vm.ToValue(p)
	}))
	_ = obj.Set("animation", r.function(func(call goja.FunctionCall) goja.Value {
		s, options := spec(arg(call, 0), arg(call, 1)), arg(call, 1)
		return r.composite(func() *core.Animation { return m.Animation(s, reduced(options)) })
	}))
	_ = obj.Set("stop", r.function(func(goja.FunctionCall) goja.Value {
		if run != nil {
			run.Stop()
		}
		return goja.Undefined()
	}))
	_ = obj.Set("reset", r.function(func(goja.FunctionCall) goja.Value {
		if run != nil {
			run.Stop()
		}
		for _, name := range channelOrder {
			v := 0.0
			if strings.HasPrefix(name, "scale") || name == "opacity" {
				v = 1
			}
			m.Value(motionChannels[name]).Set(v)
		}
		return goja.Undefined()
	}))
	return obj
}
