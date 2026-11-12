package android

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// animatorBatch is ops applied after the animator steps to at.
type animatorBatch struct {
	at  time.Duration
	ops []driver.AnimationOp
}

// animatorScenario exercises every node kind, easing family, driver and
// composite, an interruption, a read and slowed runs.
func animatorScenario() []animatorBatch {
	const (
		a, b, c, d, e, f, g = 1, 2, 3, 4, 5, 6, 7
	)
	val := func(id uint32, v float64) driver.AnimationOp {
		return driver.AnimationOp{Kind: driver.AnimCreateValue, Node: id, Value: v}
	}
	interp := func(id, in uint32, r driver.Interpolation) driver.AnimationOp {
		return driver.AnimationOp{Kind: driver.AnimCreateInterpolate, Node: id, Input: in, Interpolation: &r}
	}
	arith := func(id uint32, op driver.MathOp, x uint32, k float64) driver.AnimationOp {
		return driver.AnimationOp{Kind: driver.AnimCreateMath, Node: id, Math: op, A: driver.Operand{Node: x}, B: driver.Operand{Const: k}}
	}
	bind := func(view driver.NodeID, p driver.AnimatedProp, node uint32) driver.AnimationOp {
		return driver.AnimationOp{Kind: driver.AnimBind, View: view, Prop: p, Node: node}
	}
	timing := func(v uint32, to float64, ms int, e driver.Easing) *driver.Animation {
		return &driver.Animation{Kind: driver.AnimationTiming, Value: v, To: to,
			Timing: driver.TimingConfig{Duration: time.Duration(ms) * time.Millisecond, Easing: e}}
	}
	slowed := func(an *driver.Animation, rate float64) *driver.Animation {
		an.FrameRate = rate
		return an
	}
	start := func(id uint32, an *driver.Animation) driver.AnimationOp {
		return driver.AnimationOp{Kind: driver.AnimStart, ID: id, Animation: an}
	}
	red, green, blue := driver.Color(0xff0000ff), driver.Color(0x00ff0080), driver.Color(0x0000ffff)
	first := []driver.AnimationOp{
		val(a, 0), val(b, 0), val(c, 0), val(d, 0), val(e, 0), val(f, 0), val(g, 0),
		interp(18, g, driver.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 100},
			Easing: driver.Easing{Kind: driver.EasingSteps, Param: 4, Steps: driver.JumpStart}}),
		interp(10, a, driver.Interpolation{Input: []float64{0, 50, 100}, Colors: []driver.Color{red, green, blue}}),
		interp(11, b, driver.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 300},
			ExtrapolateRight: driver.ExtrapolateExtend, Easing: driver.Easing{Kind: driver.EasingBack, Mode: driver.EaseModeOut}}),
		arith(12, driver.MathMul, a, 0.01),
		arith(13, driver.MathMod, c, 37),
		{Kind: driver.AnimCreateDiffClamp, Node: 14, Input: c, Min: 0, Max: 60},
		{Kind: driver.AnimCreateRubberBand, Node: 19, Input: c, Min: 0, Max: 60, Value: 100},
		interp(15, d, driver.Interpolation{Input: []float64{0.2, 0.8}, Output: []float64{0, 1},
			Extrapolate: driver.ExtrapolateIdentity, ExtrapolateRight: driver.ExtrapolateClamp,
			Easing: driver.Easing{Kind: driver.EasingCircle, Mode: driver.EaseModeInOut}}),
		interp(16, e, driver.Interpolation{Input: []float64{0, 10}, Output: []float64{1, 0.5},
			Easing: driver.Easing{Kind: driver.EasingSin, Mode: driver.EaseModeInOut}}),
		arith(17, driver.MathDiv, f, -4),
		bind(1, driver.PropOpacity, 12), bind(1, driver.PropTranslateX, a), bind(1, driver.PropBackground, 10),
		bind(2, driver.PropScale, 11), bind(2, driver.PropRotate, 13), bind(2, driver.PropTranslateY, 14),
		bind(3, driver.PropTranslateX, d), bind(3, driver.PropScaleX, 16), bind(3, driver.PropScaleY, 17),
		bind(3, driver.PropShadowRadius, c), bind(3, driver.PropTranslateY, 19), bind(3, driver.PropBorderColor, 10), bind(4, driver.PropOpacity, 15),
		bind(4, driver.PropRotateX, 18), bind(4, driver.PropRotateY, g), bind(4, driver.PropShadowOpacity, 12),
		bind(4, driver.PropTextColor, 10),
		start(6, timing(g, 1, 800, driver.Easing{})),
		start(1, &driver.Animation{Kind: driver.AnimationParallel, Steps: []*driver.Animation{
			timing(a, 100, 1000, core.BezierEasing(driver.EaseInOut)),
			{Kind: driver.AnimationSpring, Value: b, To: 1, Spring: driver.SpringConfig{Stiffness: 100, Damping: 10, Mass: 1}},
		}}),
		start(2, &driver.Animation{Kind: driver.AnimationDecay, Value: c,
			Decay: driver.DecayConfig{Velocity: 1000, Deceleration: 0.998, Clamp: true, Min: -50, Max: 200, Bounce: 0.5}}),
		start(3, &driver.Animation{Kind: driver.AnimationLoop, Iterations: 3, Steps: []*driver.Animation{
			{Kind: driver.AnimationSequence, Steps: []*driver.Animation{
				timing(d, 1, 300, driver.Easing{Kind: driver.EasingBounce, Mode: driver.EaseModeOut}),
				{Kind: driver.AnimationDelay, Delay: 50 * time.Millisecond},
				timing(d, 0, 200, driver.Easing{Kind: driver.EasingElastic, Mode: driver.EaseModeInOut, Param: 2}),
			}},
		}}),
		start(4, &driver.Animation{Kind: driver.AnimationStagger, Delay: 100 * time.Millisecond, Steps: []*driver.Animation{
			timing(e, 10, 250, driver.Easing{Kind: driver.EasingPoly, Param: 3}),
			timing(f, -10, 250, driver.Easing{Kind: driver.EasingExp, Mode: driver.EaseModeOut}),
			{Kind: driver.AnimationSpring, Value: e, To: 4, Spring: driver.SpringConfig{Stiffness: 300, Damping: 30, Mass: 2, Velocity: 5}},
		}}),
	}
	return []animatorBatch{
		{at: time.Second, ops: first},
		// A Set interrupts run 1's timing, ending the run unfinished.
		{at: 1600 * time.Millisecond, ops: []driver.AnimationOp{
			{Kind: driver.AnimSet, Node: a, Value: 30},
			{Kind: driver.AnimSetOffset, Node: a, Value: 5},
			{Kind: driver.AnimRead, Node: a, ID: 9},
			start(5, timing(a, 80, 400, driver.Easing{Kind: driver.EasingCubic, Mode: driver.EaseModeOut})),
			start(7, slowed(timing(f, 5, 2000, driver.Easing{Kind: driver.EasingSteps, Param: 3, Steps: driver.JumpNone}), 20)),
			// A slowed run moves only on its own frames.
			start(8, slowed(timing(e, 0, 1500, driver.Easing{}), 12)),
		}},
		// Resets put a finished run's values and a running one's back.
		{at: 2300 * time.Millisecond, ops: []driver.AnimationOp{
			{Kind: driver.AnimFlattenOffset, Node: a},
			{Kind: driver.AnimStop, ID: 3},
			{Kind: driver.AnimRelease, Node: 13},
			{Kind: driver.AnimReset, ID: 6},
			{Kind: driver.AnimReset, ID: 7},
			{Kind: driver.AnimForgetRun, ID: 6},
		}},
	}
}

// animatorSteps are the frame times, at an uneven rate.
func animatorSteps() []time.Duration {
	var out []time.Duration
	t := time.Second
	for i := 0; t < 4*time.Second; i++ {
		out = append(out, t)
		t += time.Duration(12+i*7%11) * time.Millisecond
	}
	return out
}

// runCoreAnimator samples core.Animator as the Java check does.
func runCoreAnimator(batches []animatorBatch, steps []time.Duration) []string {
	var out, reports []string
	a := driver.NewAnimator(func(ev driver.Event) {
		reports = append(reports, fmt.Sprintf("report %d %v %v", ev.Count, ev.Value, ev.Finished))
	})
	next := 0
	for _, t := range steps {
		a.Step(t)
		for next < len(batches) && batches[next].at <= t {
			a.Apply(batches[next].ops)
			next++
		}
		sort.Strings(reports)
		out = append(out, fmt.Sprintf("step %d", t))
		out = append(out, reports...)
		reports = reports[:0]
		for view := driver.NodeID(1); view <= 4; view++ {
			v, _ := a.Values(view)
			out = append(out, fmt.Sprintf("view %d %d %v %v %v %v %v %v %v %v %s %s %s %v %v %v %s", view, v.Set,
				v.Opacity, v.TranslateX, v.TranslateY, v.Scale, v.ScaleX, v.ScaleY, v.Rotate, v.ShadowRadius,
				channels(v.Background), channels(v.BorderColor), channels(v.ShadowColor), v.ShadowOpacity, v.RotateX,
				v.RotateY, channels(v.TextColor)))
		}
	}
	return out
}

func channels(c driver.Color) string {
	return fmt.Sprintf("%d %d %d %d", c>>24, c>>16&0xff, c>>8&0xff, c&0xff)
}

// TestAnimatorMatchesJava runs the Java runtime's Animator on the JVM
// with the ops android.go encodes and compares what it samples with
// core.Animator's. It needs a JDK on PATH.
func TestAnimatorMatchesJava(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil || exec.Command(javac, "-version").Run() != nil {
		t.Skip("no JDK")
	}
	batches, steps := animatorScenario(), animatorSteps()
	w := &writer{}
	w.u32(uint32(len(batches)))
	for _, b := range batches {
		w.u64(uint64(b.at))
		writeAnimation(w, b.ops)
	}
	w.u32(uint32(len(steps)))
	for _, s := range steps {
		w.u64(uint64(s))
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "ops")
	if err := os.WriteFile(input, w.buf, 0o644); err != nil {
		t.Fatal(err)
	}
	build := exec.Command(javac, "-d", dir, "java/dev/oneo/nsgo/Animator.java", "java/dev/oneo/nsgo/Protocol.java",
		"testdata/AnimatorCheck.java")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	out, err := exec.Command("java", "-cp", dir, "dev.oneo.nsgo.AnimatorCheck", input).Output()
	if err != nil {
		t.Fatalf("java: %v", err)
	}
	want := runCoreAnimator(batches, steps)
	got := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(got) != len(want) {
		t.Fatalf("Java printed %d lines, core %d", len(got), len(want))
	}
	moving := 0
	for i := range want {
		if !sameLine(got[i], want[i]) {
			t.Fatalf("line %d differs:\n java %s\n core %s", i, got[i], want[i])
		}
		if i > 0 && want[i] != want[max(i-5, 0)] {
			moving++
		}
	}
	if moving < len(want)/4 {
		t.Fatalf("the scenario hardly moves: %d of %d lines change", moving, len(want))
	}
	t.Logf("%d lines over %d steps match", len(want), len(steps))
}

// sameLine compares two lines word by word, numbers within float32
// rounding and color channels within 1.
func sameLine(a, b string) bool {
	x, y := strings.Fields(a), strings.Fields(b)
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] == y[i] {
			continue
		}
		p, err1 := strconv.ParseFloat(x[i], 64)
		q, err2 := strconv.ParseFloat(y[i], 64)
		if err1 != nil || err2 != nil || math.Abs(p-q) > max(1e-3, 1e-4*math.Abs(q)) && math.Abs(p-q) > 1 {
			return false
		}
		if math.Abs(p-q) > max(1e-3, 1e-4*math.Abs(q)) && !isChannel(i, x) {
			return false
		}
	}
	return true
}

// isChannel reports whether word i of a view line is a color channel:
// the background, border and shadow colors', then the text color's after
// the shadow opacity and the rotations.
func isChannel(i int, words []string) bool {
	return words[0] == "view" && (i >= 11 && i < 23 || i >= 26)
}
