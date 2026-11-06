package driver

import (
	"math"
	"testing"
	"time"
)

func TestAnimatorSpringDecaySequenceAndLoop(t *testing.T) {
	a := NewAnimator(func(Event) {})
	a.Apply([]AnimationOp{{Kind: AnimCreateValue, Node: 1}, {Kind: AnimCreateValue, Node: 2}})
	spring := &Animation{Kind: AnimationSpring, Value: 1, To: 10, Spring: SpringConfig{Stiffness: 100, Damping: 10, Mass: 1}}
	a.Apply([]AnimationOp{{Kind: AnimStart, ID: 1, Animation: spring}})
	over := false
	for i := range 400 {
		a.Step(time.Duration(i) * 16 * time.Millisecond)
		if a.nodes[1].value > 10.01 {
			over = true
		}
	}
	if !over || a.Running() || a.nodes[1].value != 10 {
		t.Fatalf("spring: overshot %v running %v value %v", over, a.Running(), a.nodes[1].value)
	}

	decay := &Animation{Kind: AnimationDecay, Value: 2, Decay: DecayConfig{Velocity: 1000, Deceleration: 0.998, Clamp: true, Min: 0, Max: 300}}
	a.Apply([]AnimationOp{{Kind: AnimStart, ID: 2, Animation: decay}})
	for i := range 200 {
		a.Step(time.Duration(i) * 16 * time.Millisecond)
	}
	if v := a.nodes[2].value; v != 300 || a.Running() {
		t.Fatalf("decay stopped at %v, want its bound", v)
	}

	// A sequence starts each step at the exact end of the one before, and
	// a loop puts its values back.
	a.Apply([]AnimationOp{{Kind: AnimSet, Node: 1, Value: 0}})
	seq := &Animation{Kind: AnimationLoop, Iterations: 2, Steps: []*Animation{{Kind: AnimationSequence, Steps: []*Animation{
		{Kind: AnimationTiming, Value: 1, To: 1, Timing: TimingConfig{Duration: 100 * time.Millisecond}},
		{Kind: AnimationTiming, Value: 1, To: 3, Timing: TimingConfig{Duration: 100 * time.Millisecond}},
	}}}}
	var done []Event
	a.report = func(e Event) { done = append(done, e) }
	a.Apply([]AnimationOp{{Kind: AnimStart, ID: 3, Animation: seq}})
	a.Step(0)
	a.Step(150 * time.Millisecond)
	if v := a.nodes[1].value; math.Abs(v-2) > 1e-9 {
		t.Fatalf("sequence at 150ms: %v, want 2", v)
	}
	a.Step(250 * time.Millisecond) // into the second iteration, from 0
	if v := a.nodes[1].value; math.Abs(v-0.5) > 1e-9 {
		t.Fatalf("loop at 250ms: %v, want 0.5", v)
	}
	a.Step(400 * time.Millisecond)
	if len(done) != 1 || !done[0].Finished || done[0].Count != 3 || a.nodes[1].value != 3 {
		t.Fatalf("loop ended %+v at %v", done, a.nodes[1].value)
	}

	// Setting a value ends its run unfinished.
	done = nil
	a.Apply([]AnimationOp{{Kind: AnimStart, ID: 4, Animation: &Animation{Kind: AnimationTiming, Value: 1, To: 0,
		Timing: TimingConfig{Duration: time.Second}}}})
	a.Step(time.Second)
	a.Apply([]AnimationOp{{Kind: AnimSet, Node: 1, Value: 5}})
	if len(done) != 1 || done[0].Finished || a.Running() {
		t.Fatalf("interrupted %+v", done)
	}
}

func TestALoopWithoutResetGoesOnFromWhereItEnded(t *testing.T) {
	a := NewAnimator(func(Event) {})
	a.Apply([]AnimationOp{{Kind: AnimCreateValue, Node: 1}})
	// A timing to 1 from wherever the value is: a second iteration that
	// starts at 1 stays there.
	loop := &Animation{Kind: AnimationLoop, Iterations: 2, NoReset: true, Steps: []*Animation{
		{Kind: AnimationTiming, Value: 1, To: 1, Timing: TimingConfig{Duration: 100 * time.Millisecond}}}}
	a.Apply([]AnimationOp{{Kind: AnimStart, ID: 1, Animation: loop}})
	a.Step(0)
	a.Step(150 * time.Millisecond)
	if v := a.nodes[1].value; math.Abs(v-1) > 1e-9 {
		t.Fatalf("second iteration at %v, want it to go on from 1", v)
	}
}

func TestInterpolationExtrapolates(t *testing.T) {
	r := Interpolation{Input: []float64{0, 1}, Output: []float64{0, 10}, Extrapolate: ExtrapolateExtend,
		ExtrapolateLeft: ExtrapolateClamp}
	if v, _ := r.At(-1); v != 0 {
		t.Fatalf("left %v, want clamped", v)
	}
	if v, _ := r.At(2); v != 20 {
		t.Fatalf("right %v, want extended", v)
	}
	if v, _ := (&Interpolation{Input: []float64{0, 1}, Output: []float64{0, 10}}).At(2); v != 10 {
		t.Fatalf("default %v, want clamped", v)
	}
}

func TestAnimatorPanAndMath(t *testing.T) {
	a := NewAnimator(func(Event) {})
	a.Apply([]AnimationOp{
		{Kind: AnimCreateValue, Node: 1},
		{Kind: AnimCreateMath, Node: 2, Math: MathMul, A: Operand{Node: 1}, B: Operand{Const: 2}},
		{Kind: AnimCreateDiffClamp, Node: 3, Input: 1, Min: 0, Max: 50},
		{Kind: AnimBindPan, View: 7, X: 1},
		{Kind: AnimBind, View: 7, Prop: PropTranslateX, Node: 2},
		{Kind: AnimBind, View: 7, Prop: PropTranslateY, Node: 3},
	})
	for i, x := range []Points{30, 100, 80} {
		phase := GestureChanged
		if i == 0 {
			phase = GestureBegan
		}
		if !a.Gesture(&Event{Kind: EventPan, ID: 7, Phase: phase, Point: Point{X: x}}) {
			t.Fatal("pan not bound")
		}
		a.Values(7)
	}
	v, _ := a.Values(7)
	if v.TranslateX != 160 || v.TranslateY != 30 {
		t.Fatalf("values %+v, want 160 and a diff clamp of 30", v)
	}
	vs := v.Apply(ViewStyle{Opacity: 1, Transform: Transform{Rotate: 9}})
	if vs.Transform.TranslateX != 160 || vs.Transform.Rotate != 9 || vs.Opacity != 1 {
		t.Fatalf("applied %+v", vs)
	}
}

func TestGesturesMoveTheirValuesFromWhereTheyWere(t *testing.T) {
	a := NewAnimator(func(Event) {})
	a.Apply([]AnimationOp{
		{Kind: AnimCreateValue, Node: 1, Value: 40},
		{Kind: AnimCreateValue, Node: 2, Value: 2},
		{Kind: AnimCreateValue, Node: 3, Value: 0.5},
		{Kind: AnimBindPan, View: 7, X: 1},
		{Kind: AnimBindPinch, View: 7, Node: 2},
		{Kind: AnimBindRotation, View: 7, Node: 3},
	})
	began := Event{Kind: EventPan, ID: 7, Phase: GestureBegan, Point: Point{X: 10}}
	a.Gesture(&began)
	if began.Values.X != 50 {
		t.Fatalf("pan start values = %v, want 40 grabbed plus 10", began.Values)
	}
	ended := Event{Kind: EventPan, ID: 7, Phase: GestureEnded, Point: Point{X: -15}}
	a.Gesture(&ended)
	if ended.Values.X != 25 {
		t.Fatalf("pan end values = %v, want 25", ended.Values)
	}
	a.Gesture(&Event{Kind: EventPinch, ID: 7, Phase: GestureBegan, Value: 1})
	a.Gesture(&Event{Kind: EventPinch, ID: 7, Phase: GestureChanged, Value: 1.5})
	a.Gesture(&Event{Kind: EventRotation, ID: 7, Phase: GestureBegan, Value: 0})
	a.Gesture(&Event{Kind: EventRotation, ID: 7, Phase: GestureChanged, Value: 0.25})
	if s, r := a.value(2).value, a.value(3).value; s != 3 || r != 0.75 {
		t.Fatalf("pinch value %v, rotation value %v; want 3 and 0.75", s, r)
	}
	if a.Gesture(&Event{Kind: EventPinch, ID: 8, Phase: GestureBegan}) {
		t.Fatal("a view with no value bound moved one")
	}
}

// A shader view's bound lanes follow their nodes, and the view is
// reported only when a lane it draws changed.
func TestAnimatorShaderLanes(t *testing.T) {
	a := NewAnimator(func(Event) {})
	// The lanes bind before the batch's ItemAnimation makes the nodes.
	a.BindShader(5, []ShaderNode{{Lane: 1, Node: 1}, {Lane: 2, Node: 3}})
	a.Apply([]AnimationOp{{Kind: AnimCreateValue, Node: 1, Value: 2}, {Kind: AnimCreateValue, Node: 2},
		{Kind: AnimCreateMath, Node: 3, Math: MathMul, A: Operand{Node: 1}, B: Operand{Const: 10}}})
	changed := func() (views []NodeID) {
		a.ChangedShaders(func(v NodeID) { views = append(views, v) })
		return views
	}
	if got := changed(); len(got) != 1 || got[0] != 5 {
		t.Fatalf("after binding: %v changed, want view 5", got)
	}
	lanes := []float64{7, 0, 0}
	a.ShaderLanes(5, lanes)
	if lanes[0] != 7 || lanes[1] != 2 || lanes[2] != 20 {
		t.Fatalf("lanes = %v", lanes)
	}
	// A value the shader does not read changes nothing it draws.
	a.Apply([]AnimationOp{{Kind: AnimSet, Node: 2, Value: 9}})
	if got := changed(); len(got) != 0 {
		t.Fatalf("an unbound value reported %v", got)
	}
	a.Apply([]AnimationOp{{Kind: AnimStart, ID: 1, Animation: &Animation{Kind: AnimationTiming, Value: 1, To: 4,
		Timing: TimingConfig{Duration: 100 * time.Millisecond}}}})
	a.Step(0)
	a.Step(50 * time.Millisecond)
	if got := changed(); len(got) != 1 {
		t.Fatalf("a running animation reported %v", got)
	}
	a.ShaderLanes(5, lanes)
	if lanes[1] != 3 || lanes[2] != 30 {
		t.Fatalf("mid-animation lanes = %v", lanes)
	}
	a.Forget(5)
	a.Step(100 * time.Millisecond)
	if got := changed(); len(got) != 0 {
		t.Fatalf("a forgotten view reported %v", got)
	}
}

func TestStepsEasingsAndReversal(t *testing.T) {
	steps := func(n float64, p StepPosition) Easing { return Easing{Kind: EasingSteps, Param: n, Steps: p} }
	for _, c := range []struct {
		e    Easing
		x    float64
		want float64
	}{
		{steps(4, JumpEnd), 0.3, 0.25}, {steps(4, JumpEnd), 1, 1}, {steps(4, JumpStart), 0, 0.25},
		{steps(4, JumpStart), 0.3, 0.5}, {steps(3, JumpNone), 0.5, 0.5}, {steps(3, JumpBoth), 0, 0.25},
	} {
		if got := c.e.At(c.x); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%+v at %v = %v, want %v", c.e, c.x, got, c.want)
		}
	}
	in := Easing{Kind: EasingCubic}
	if r := in.Reversed(); r.Mode != EaseModeOut || math.Abs(r.At(0.3)-(1-in.At(0.7))) > 1e-9 {
		t.Errorf("reversed cubic = %+v", r)
	}
	b := Easing{Bezier: EaseIn}
	if r := b.Reversed(); math.Abs(r.At(0.25)-(1-b.At(0.75))) > 1e-3 {
		t.Errorf("reversed ease-in at 0.25 = %v, want %v", r.At(0.25), 1-b.At(0.75))
	}
}

func TestResetsAndSnappingClamps(t *testing.T) {
	a := NewAnimator(func(Event) {})
	a.Apply([]AnimationOp{{Kind: AnimCreateValue, Node: 1, Value: 5},
		{Kind: AnimStart, ID: 1, Animation: &Animation{Kind: AnimationTiming, Value: 1, To: 10,
			Timing: TimingConfig{Duration: time.Second}}}})
	a.Step(0)
	a.Step(500 * time.Millisecond)
	a.Apply([]AnimationOp{{Kind: AnimReset, ID: 1}})
	if v, _, _ := a.Value(1); v != 5 || a.Running() {
		t.Fatalf("reset left %v, running %v", v, a.Running())
	}

	a.Apply([]AnimationOp{{Kind: AnimCreateValue, Node: 2},
		{Kind: AnimCreateDiffClamp, Node: 3, Input: 2, Min: 0, Max: 40, Snap: true},
		{Kind: AnimBindScroll, View: 9, Y: 2}})
	a.Value(3)
	a.Scroll(9, Point{Y: 25})
	if v, _, _ := a.Value(3); v != 25 {
		t.Fatalf("clamp at %v", v)
	}
	a.Settle(9)
	a.Step(time.Second)
	a.Step(time.Second + 100*time.Millisecond)
	mid, _, _ := a.Value(3)
	a.Step(2 * time.Second)
	if end, _, _ := a.Value(3); mid <= 0 || mid >= 25 || end != 0 || a.Running() {
		t.Fatalf("settled through %v to %v, want toward 0 short of the max", mid, end)
	}
}

func TestSamplersRunTheDrivers(t *testing.T) {
	s := SampleSpring(0, 1, SpringConfig{Stiffness: 100, Damping: 10, Mass: 1})
	if s[0] != 0 || s[len(s)-1] != 1 || len(s) < 30 {
		t.Fatalf("spring samples %d, from %v to %v", len(s), s[0], s[len(s)-1])
	}
	over := false
	for _, v := range s {
		over = over || v > 1
	}
	if !over {
		t.Fatal("an underdamped spring should overshoot")
	}
	d := SampleDecay(0, DecayConfig{Velocity: 1000, Deceleration: 0.998})
	if last := d[len(d)-1]; math.Abs(last-500) > 5 {
		t.Fatalf("decay ends at %v, want near 500", last)
	}
	if e := SampleEasing(Easing{Bezier: Ease}, 5); len(e) != 5 || e[0] != 0 || e[4] != 1 {
		t.Fatalf("easing samples %v", e)
	}
}

func TestLayoutBezierFollowsItsCurve(t *testing.T) {
	tr := Transition{Duration: 200 * time.Millisecond, Delay: 100 * time.Millisecond, Easing: EaseOut}.Layout()
	if p, done := tr.At(50 * time.Millisecond); p != 0 || done {
		t.Errorf("in the delay: %v %v", p, done)
	}
	if p, _ := tr.At(200 * time.Millisecond); math.Abs(p-EaseOut.At(0.5)) > 1e-9 {
		t.Errorf("halfway %v, want %v", p, EaseOut.At(0.5))
	}
	if p, done := tr.At(300 * time.Millisecond); p != 1 || !done {
		t.Errorf("end %v %v", p, done)
	}
}

func TestAFrameRateSlowsARunAndItsClock(t *testing.T) {
	a := NewAnimator(func(Event) {})
	a.Apply([]AnimationOp{{Kind: AnimCreateValue, Node: 1}, {Kind: AnimCreateValue, Node: 2}})
	move := func(id uint32, rate float64) {
		a.Apply([]AnimationOp{{Kind: AnimStart, ID: uint32(id), Animation: &Animation{Kind: AnimationTiming, Value: id, To: 1,
			Timing: TimingConfig{Duration: time.Second}, FrameRate: rate}}})
	}
	move(1, 30)
	if r := a.FrameRate(); r != 30 {
		t.Fatalf("a slowed run's rate = %v", r)
	}
	// A 120 Hz clock: the run moves on every fourth frame, not on the
	// eighth, though the frames land a hair before its rate.
	changes, last := 0, -1.0
	for i := range 60 {
		a.Step(time.Duration(i) * time.Second / 120)
		if v := a.nodes[1].value; v != last {
			changes, last = changes+1, v
		}
	}
	if changes != 15 {
		t.Fatalf("a 30 Hz run changed %d times in half a second of 120 Hz frames", changes)
	}
	// A run that wants every frame takes the clock back to full speed.
	move(2, 0)
	if r := a.FrameRate(); r != 0 {
		t.Fatalf("the rate with an unslowed run = %v", r)
	}
	// It still ends where it would have.
	for i := 60; i < 140; i++ {
		a.Step(time.Duration(i) * time.Second / 120)
	}
	if a.nodes[1].value != 1 {
		t.Fatalf("a slowed run ended at %v", a.nodes[1].value)
	}
}
