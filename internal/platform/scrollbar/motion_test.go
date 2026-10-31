package scrollbar

import (
	"math"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestDragsRubberBandPastTheEdges(t *testing.T) {
	if got := Dragged(50, 0, 100, 200, true); got != 50 {
		t.Fatalf("inside the limits moved to %v", got)
	}
	over := Dragged(-100, 0, 100, 200, true)
	if over >= 0 || over <= -100 {
		t.Fatalf("100 past the start shows at %v, want a part of it", over)
	}
	if further := Dragged(-200, 0, 100, 200, true); further >= over || -further-(-over) >= 100 {
		t.Fatalf("pulling further gives less: %v after %v", further, over)
	}
	if got := Dragged(300, 0, 100, 200, false); got != 100 {
		t.Fatalf("without bounces stopped at %v, want the end", got)
	}
}

func TestAGlideBouncesAtTheEdgeItHits(t *testing.T) {
	// It leaves the edge at the speed it hit it, overshoots and comes back.
	near, _ := Bounce(2, time.Millisecond)
	if near < 1.9 || near > 2 {
		t.Fatalf("a millisecond after hitting at 2 points/ms it is %v past", near)
	}
	peak, back := Bounce(2, bounceTime)
	if back || peak < 50 || peak > 70 {
		t.Fatalf("at its peak it is %v past (%v)", peak, back)
	}
	if fast, _ := Bounce(20, bounceTime); fast > 100 {
		t.Fatalf("a fast hit overshoots %v", fast)
	}
	if up, _ := Bounce(-2, bounceTime); up != -peak {
		t.Fatalf("a bounce at the start is %v, want %v", up, -peak)
	}
	if over, back := Bounce(2, 12*bounceTime); !back || over > 0.5 {
		t.Fatalf("not back after twelve time constants: %v", over)
	}
}

func TestSnapsMatchTheRelease(t *testing.T) {
	if got := SnapDuration(0.2, 1); got != 0 {
		t.Fatalf("nothing to move took %v", got)
	}
	if got := SnapDuration(100, -1); got != Animation {
		t.Fatalf("against the release took %v", got)
	}
	// Ease out starts at 3 × 100 / 300 = 1 point per millisecond.
	if got := SnapDuration(100, 1); got != 300*time.Millisecond {
		t.Fatalf("a move at the release's speed took %v", got)
	}
	if got := SnapDuration(10, 5); got != 180*time.Millisecond {
		t.Fatalf("a short fast move took %v, want the shortest", got)
	}
	if EaseOut(0) != 0 || EaseOut(1) != 1 || EaseOut(2) != 1 || EaseOut(0.5) <= 0.5 {
		t.Fatal("ease out does not ease out")
	}
}

func TestTheIndicatorShowsWhatIsSeen(t *testing.T) {
	if _, _, ok := Indicator(200, 200, 0); ok {
		t.Fatal("content that fits has an indicator")
	}
	start, length, ok := Indicator(200, 800, 0)
	if !ok || start != IndicatorMargin || length != 49 {
		t.Fatalf("at the start: %v long at %v", length, start)
	}
	if start, length, _ := Indicator(200, 800, 600); start+length != 200-IndicatorMargin {
		t.Fatalf("at the end it ends at %v", start+length)
	}
	if _, pulled, _ := Indicator(200, 800, -40); pulled >= length {
		t.Fatalf("pulled past the start it is %v long, was %v", pulled, length)
	}
	if _, long, _ := Indicator(200, 1e6, 0); long != indicatorMin {
		t.Fatalf("on long content it is %v long", long)
	}
	if IndicatorAlpha(IndicatorHold/2) != 1 || IndicatorAlpha(IndicatorHold+IndicatorFade) != 0 {
		t.Fatal("the indicator does not fade after it holds")
	}
}

func TestResamplingMovesEvenlyBetweenEvents(t *testing.T) {
	ms := func(n float64) time.Duration { return time.Duration(n * float64(time.Millisecond)) }
	r := Resampler{Interval: ms(8)}
	// Events every 8 ms, 10 points each, from 0.
	for i := 1; i <= 4; i++ {
		r.Push(ms(float64(8*i)), driver.Points(10*i), 0)
	}
	if math.Abs(float64(r.Interval-ms(8))) > float64(time.Microsecond) {
		t.Fatalf("the interval drifted to %v", r.Interval)
	}
	// The first event starts its interval from what showed.
	if got, _ := r.Sample(ms(4)); math.Abs(float64(got-5)) > 1e-3 {
		t.Fatalf("halfway to the first event: %v", got)
	}
	// Frames at other times interpolate.
	if got, _ := r.Sample(ms(13)); math.Abs(float64(got-16.25)) > 1e-3 {
		t.Fatalf("at 13 ms: %v", got)
	}
	if got, past := r.Sample(ms(29)); past || math.Abs(float64(got-36.25)) > 1e-3 {
		t.Fatalf("at 29 ms: %v past=%v", got, past)
	}
	if got, past := r.Sample(ms(40)); !past || got != 40 {
		t.Fatalf("after the last event: %v past=%v", got, past)
	}
	if goal, _ := r.Goal(); goal != 40 {
		t.Fatalf("the goal is %v", goal)
	}
	// The core moving the content moves what waits.
	r.Push(ms(48), 50, 0)
	r.Shift(100)
	if goal, _ := r.Goal(); goal != 150 {
		t.Fatalf("shifted goal is %v", goal)
	}
}

func TestAPauseStopsTheVelocity(t *testing.T) {
	var v Velocity
	for i := range 10 {
		v.Add(time.Duration(i)*8*time.Millisecond, driver.Point{Y: driver.Points(16 * i)})
	}
	if _, y := v.Speed(); math.Abs(float64(y-2)) > 1e-3 {
		t.Fatalf("16 points every 8 ms is %v per ms", y)
	}
	v.Add(500*time.Millisecond, driver.Point{Y: 144})
	if _, y := v.Speed(); y != 0 {
		t.Fatalf("after a pause: %v", y)
	}
}

func TestRevealingShowsTheSpanWithAMargin(t *testing.T) {
	if got := RevealDelta(100, 140, 400); got != 0 {
		t.Fatalf("a span in view moved %v", got)
	}
	if got := RevealDelta(-50, -10, 400); got != -66 {
		t.Fatalf("a span above moved %v, want -66", got)
	}
	if got := RevealDelta(420, 460, 400); got != 76 {
		t.Fatalf("a span below moved %v, want 76", got)
	}
	// Too long to fit: its start shows.
	if got := RevealDelta(300, 900, 400); got != 284 {
		t.Fatalf("a long span moved %v, want 284", got)
	}
}
