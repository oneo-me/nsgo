//go:build linux && !android

package linux

import "runtime"

// An app's main goroutine lives in GTK's callbacks and never passes
// through Go's scheduler by itself. Go's monitor thread, finding its P on
// one scheduling round for 10 ms (forcePreemptNS in runtime/proc.go),
// takes the P whenever the thread is back in GTK and then polls every
// 20 µs: about 3,000 wakeups a second while an app animates. Each tick
// yields, but the frame clock ticks 16 ms apart, so while a window's clock
// runs a timer passes the goroutine through the scheduler every
// yieldMillis as well: either alone left about 2,400 a second of a list
// scrolled every frame, both about 1,000 (2026-11-13).
const yieldMillis = 8

var (
	yielders int    // the windows whose clocks run
	yieldID  uint32 // the timer, while one does
)

// startYielding counts a window whose clock started.
func startYielding() {
	yielders++
	if yieldID == 0 {
		yieldID = gTimeoutAdd(yieldMillis, cbYield, 0)
	}
}

// stopYielding counts a window whose clock stopped.
func stopYielding() {
	yielders--
	if yielders == 0 && yieldID != 0 {
		gSourceRemove(yieldID)
		yieldID = 0
	}
}

// yieldCallback is the timer's pass through the scheduler.
func yieldCallback(uintptr) int32 {
	runtime.Gosched()
	return 1
}
