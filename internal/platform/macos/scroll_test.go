//go:build darwin && !ios

package macos

import (
	"testing"
	"time"
)

func TestALongScrollCommitsOnlyWhenItEnds(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	var rested time.Duration
	// Ten seconds of input or glide: never mid-scroll.
	for at := 1; at < 10_000; at += 16 {
		if commitDue(ms(at), ms(at)+commitDelay, false, true, &rested) {
			t.Fatalf("committed mid-scroll at %d ms", at)
		}
	}
	if commitDue(ms(10_000), ms(10_100), true, false, &rested) {
		t.Fatal("committed while the offset still moved")
	}
	if !commitDue(ms(10_200), ms(10_100), true, false, &rested) {
		t.Fatal("did not commit at rest")
	}
}

func TestARunningAnimationHoldsACommitOnlyBriefly(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	var rested time.Duration
	if commitDue(ms(1000), ms(900), true, true, &rested) {
		t.Fatal("committed while the frame clock runs")
	}
	if commitDue(ms(1000)+restWait-ms(32), ms(900), true, true, &rested) {
		t.Fatal("committed before the wait ran out")
	}
	if !commitDue(ms(1000)+restWait, ms(900), true, true, &rested) {
		t.Fatal("an endless animation held the commit back")
	}
	// Input again starts the wait over.
	rested = ms(1)
	if commitDue(ms(5000), ms(4900), false, true, &rested) || rested != 0 {
		t.Fatal("input did not start the wait over")
	}
}

func TestAScrollPutsThePickOffUntilItSettles(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	// Ten seconds of steps, each pushing the deadline on: never mid-scroll.
	for at := 0; at < 10_000; at += 16 {
		if repickIn(ms(at+16), ms(at)+repickDelay, false) == 0 {
			t.Fatalf("picked mid-scroll at %d ms", at)
		}
	}
	last := ms(10_000) + repickDelay
	if got := repickIn(ms(10_050), last, false); got != ms(70) {
		t.Fatalf("waits %v 50 ms after the last step, want 70ms", got)
	}
	if repickIn(last, last, false) != 0 {
		t.Fatal("did not pick when the scroll settled")
	}
	if repickIn(last, last, true) == 0 {
		t.Fatal("picked while a batch applied")
	}
}

func TestAScrollUnderWayNeedsNoRouting(t *testing.T) {
	v := &view{id: 1, sc: &scrollState{}}
	s := &surface{views: []*view{nil, v}}
	gone := &view{id: 1, sc: &scrollState{}}
	check := func(name string, phase, momentum uint, want bool) {
		t.Helper()
		if got := s.wheelContinues(phase, momentum); got != want {
			t.Errorf("%s: wheelContinues = %v, want %v", name, got, want)
		}
	}
	check("a change with no gesture", phaseChanged, 0, false)
	check("momentum with no view", 0, phaseChanged, false)

	s.wheels.gesture = &wheelGesture{}
	check("a gesture before it finds its view", phaseChanged, 0, false)
	check("a beginning", phaseBegan, 0, false)
	check("the end of a gesture without a view", 0|phaseEnded, 0, true)

	s.wheels.gesture = &wheelGesture{view: v, held: true}
	check("resting fingers choose their view", phaseChanged, 0, false)
	check("fingers lifted without scrolling", phaseCancelled, 0, true)

	s.wheels.gesture = &wheelGesture{view: v}
	check("a change", phaseChanged, 0, true)
	check("an end", phaseEnded, 0, true)
	s.wheels.gesture = &wheelGesture{view: gone}
	check("a change on an unmounted view", phaseChanged, 0, false)

	s.wheels.gesture, s.wheels.momentum = nil, v
	check("momentum", 0, phaseChanged, true)
	check("the end of momentum", 0, phaseEnded, true)
	s.wheels.momentum = gone
	check("momentum on an unmounted view", 0, phaseChanged, false)
}
