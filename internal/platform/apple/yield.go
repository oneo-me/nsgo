//go:build darwin

package apple

import (
	"runtime"
	"time"

	"github.com/ebitengine/purego/objc"
)

// An app's main goroutine lives in the platform's callbacks and never
// passes through Go's scheduler by itself. Go's monitor thread, finding it
// on one P for 10 ms (forcePreemptNS in runtime/proc.go), takes the P
// whenever the thread is back in the platform and then polls every 20 µs
// for a while: about 3,000 wakeups a second while an app animates.

const (
	// yieldWithin is how far apart frames may be and still pass the main
	// goroutine through the scheduler often enough by themselves.
	yieldWithin = 9 * time.Millisecond
	// yieldEvery is how often the timer does it between frames further
	// apart, leaving room for a late timer.
	yieldEvery = 8 * time.Millisecond
	// yieldLinger is how long the timer outlives the last frame that
	// needed it. It then neither stops and starts with every other frame
	// when two windows tick in one refresh, nor starts on the first frame
	// after a pause, nor needs a frame clock to tell it that it stopped.
	yieldLinger = 100 * time.Millisecond
)

// yieldClock decides from the frames' times whether the timer runs.
type yieldClock struct {
	last time.Time // the last frame
	slow time.Time // the last frame that followed another by more than yieldWithin
}

// frame records a frame at now and reports whether the timer runs after
// it.
func (c *yieldClock) frame(now time.Time) bool {
	if gap := now.Sub(c.last); gap > yieldWithin && gap < yieldLinger {
		c.slow = now
	}
	c.last = now
	return now.Sub(c.slow) < yieldLinger
}

// idle reports whether the frames stopped.
func (c *yieldClock) idle(now time.Time) bool { return now.Sub(c.last) >= yieldLinger }

var (
	yields      yieldClock
	yieldTimer  objc.ID
	yieldTarget objc.ID
	selYield    = sel("nsgoYield:")
)

// YieldFrame passes the main goroutine through Go's scheduler. A frame
// clock calls it once a tick, on the main thread, which is enough for
// frames up to yieldWithin apart; between slower ones, a 60 Hz display's or
// a slowed animation's, a timer yields every yieldEvery until frames come
// that close again or stop.
func YieldFrame() {
	now := time.Now()
	if yieldTimer == 0 {
		runtime.Gosched()
	}
	if !yields.frame(now) {
		stopYielding()
		return
	}
	if yieldTimer != 0 {
		return
	}
	if yieldTarget == 0 {
		class := RegisterClass("NsgoYield", Class("NSObject"), nil, []objc.MethodDef{{Cmd: selYield, Fn: yieldTick}})
		yieldTarget = objc.ID(class).Send(sel("new"))
	}
	// In the common modes, so it keeps running while a menu tracks or a
	// scroll view scrolls.
	yieldTimer = objc.ID(Class("NSTimer")).Send(sel("timerWithTimeInterval:target:selector:userInfo:repeats:"),
		yieldEvery.Seconds(), yieldTarget, selYield, objc.ID(0), true).Send(sel("retain"))
	objc.ID(Class("NSRunLoop")).Send(sel("mainRunLoop")).Send(sel("addTimer:forMode:"), yieldTimer, CommonModes())
}

// yieldTick is the timer's pass through the scheduler.
func yieldTick(objc.ID, objc.SEL, objc.ID) {
	runtime.Gosched()
	if yields.idle(time.Now()) {
		stopYielding()
	}
}

func stopYielding() {
	if yieldTimer != 0 {
		yieldTimer.Send(sel("invalidate"))
		yieldTimer.Send(sel("release"))
		yieldTimer = 0
	}
}
