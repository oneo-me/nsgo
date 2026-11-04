//go:build js && wasm

package web

import (
	"math"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Wheels and touchpads scroll as NSH's do (nsh.js's wheel): the page moves
// a scroll view by the event's delta itself, rather than leaving it to the
// browser, whose step differs with the display's scale (a wheel step moved
// the DataTable twice as far as NSH's at 2x). A scroll goes to the
// innermost scroll view along its axis that can move that way (a vertical
// wheel scrolls a horizontal view when no vertical one can) and stays on it
// until events stop for wheelLockGap. Notches glide, touchpads move with
// the fingers. Views that snap are left to the browser's CSS snapping.

const (
	wheelLockGap = 200 // ms between the events of one scroll
	glideSettle  = 200 // ms without input before a glide's scroll ends
)

// wheelLock is the scroll view a wheel scroll goes to, and when its last
// event came.
type wheelLock struct {
	v *view
	t float64
}

// glide moves a scroll view to where the wheel sends it a part of the way
// each frame.
type glide struct {
	to    [2]float64 // scrollLeft and scrollTop it goes to
	at    [2]float64 // where it is, unrounded
	last  float64
	frame js.Value // the animation frame requested, or undefined
	timer js.Value // the settle timeout, or undefined
	step  js.Func
	end   js.Func
}

// wheelScroll scrolls the view under a wheel or touchpad event; Ctrl's
// are pinches (wheelZoom).
func (s *surface) wheelScroll(e js.Value) {
	if e.Get("ctrlKey").Bool() || e.Get("defaultPrevented").Bool() {
		return
	}
	unit := 1.0
	switch e.Get("deltaMode").Int() {
	case 1: // lines
		unit = 40
	case 2: // pages
		unit = float64(s.size().Height)
	}
	dx, dy := e.Get("deltaX").Float()*unit, e.Get("deltaY").Float()*unit
	if e.Get("shiftKey").Bool() && dx == 0 {
		dx, dy = dy, 0
	}
	t := now()
	g := &s.gestures
	lock := g.scroll
	if lock == nil || t-lock.t > wheelLockGap || s.view(lock.v.id) != lock.v {
		g.scroll = nil
		v := s.wheelTarget(e.Get("target"), dx, dy)
		if v == nil || v.snaps() {
			return // the page around, or the browser's snapping
		}
		lock = &wheelLock{v: v}
		g.scroll = lock
	}
	e.Call("preventDefault")
	lock.t = t
	v := lock.v
	if v.sc.config.Horizontal {
		if dx == 0 {
			dx = dy
		}
		dy = 0
	} else {
		dx = 0
	}
	d := dx + dy
	s.glide(v, dx, dy, e.Get("deltaMode").Int() != 0 || math.Abs(d) >= 40)
}

// wheelTarget is the scroll view around el that a scroll of (dx, dy)
// moves.
func (s *surface) wheelTarget(el js.Value, dx, dy float64) *view {
	var chain []*view
	for ; el.Truthy() && !el.Equal(s.host); el = el.Get("parentElement") {
		if v := s.viewOf(el); v != nil && v.kind == driver.KindScroll && !v.sc.config.Behavior.NoScroll {
			chain = append(chain, v)
		}
	}
	horizontal := math.Abs(dx) > math.Abs(dy)
	d := dy
	if horizontal {
		d = dx
	}
	for _, v := range chain {
		if v.sc.config.Horizontal == horizontal && v.canScroll(d) {
			return v
		}
	}
	if !horizontal {
		for _, v := range chain {
			if v.sc.config.Horizontal && v.canScroll(dy) {
				return v
			}
		}
	}
	return nil
}

// canScroll reports whether scroll view v can move by d along its axis.
func (v *view) canScroll(d float64) bool {
	pos, size, seen := "scrollTop", "scrollHeight", "clientHeight"
	if v.sc.config.Horizontal {
		pos, size, seen = "scrollLeft", "scrollWidth", "clientWidth"
	}
	at := v.el.Get(pos).Float()
	end := v.el.Get(size).Float() - v.el.Get(seen).Float()
	return d < 0 && at > 0.5 || d > 0 && at < end-0.5
}

// snaps reports whether scroll view v stops at snap points.
func (v *view) snaps() bool { return len(v.sc.markers) > 0 }

// glide moves v by (dx, dy), gliding there when smooth, and ends its
// scroll once no more input comes for glideSettle.
func (s *surface) glide(v *view, dx, dy float64, smooth bool) {
	sc := v.sc
	a := sc.glide
	if a == nil {
		a = &glide{to: [2]float64{v.el.Get("scrollLeft").Float(), v.el.Get("scrollTop").Float()}}
		sc.glide = a
		sc.driven++
		a.step = js.FuncOf(func(_ js.Value, args []js.Value) any {
			s.glideFrame(v, a, args[0].Float())
			return nil
		})
		a.end = js.FuncOf(func(js.Value, []js.Value) any {
			a.timer = js.Undefined()
			s.settleGlide(v, a)
			return nil
		})
	}
	a.to[0] = min(max(a.to[0]+dx, 0), v.el.Get("scrollWidth").Float()-v.el.Get("clientWidth").Float())
	a.to[1] = min(max(a.to[1]+dy, 0), v.el.Get("scrollHeight").Float()-v.el.Get("clientHeight").Float())
	switch {
	case !smooth:
		if a.frame.Truthy() {
			js.Global().Call("cancelAnimationFrame", a.frame)
			a.frame = js.Undefined()
		}
		v.el.Set("scrollLeft", a.to[0])
		v.el.Set("scrollTop", a.to[1])
	case !a.frame.Truthy():
		a.last = now()
		// Where the glide is: browsers round the offsets they are given.
		a.at = [2]float64{v.el.Get("scrollLeft").Float(), v.el.Get("scrollTop").Float()}
		a.frame = js.Global().Call("requestAnimationFrame", a.step)
	}
	if a.timer.Truthy() {
		js.Global().Call("clearTimeout", a.timer)
	}
	a.timer = js.Global().Call("setTimeout", a.end, glideSettle)
}

// glideFrame moves a glide a quarter of the way left each 60 Hz frame.
func (s *surface) glideFrame(v *view, a *glide, t float64) {
	k := 1 - math.Pow(0.75, (t-a.last)/16.7)
	a.last = t
	x, y := a.at[0]+(a.to[0]-a.at[0])*k, a.at[1]+(a.to[1]-a.at[1])*k
	done := math.Abs(a.to[0]-x) < 0.5 && math.Abs(a.to[1]-y) < 0.5
	if done {
		x, y = a.to[0], a.to[1]
	}
	a.at = [2]float64{x, y}
	v.el.Set("scrollLeft", x)
	v.el.Set("scrollTop", y)
	a.frame = js.Undefined()
	if !done {
		a.frame = js.Global().Call("requestAnimationFrame", a.step)
		return
	}
	s.settleGlide(v, a)
}

// settleGlide ends v's scroll once its glide is still and no input
// comes.
func (s *surface) settleGlide(v *view, a *glide) {
	if a.frame.Truthy() || a.timer.Truthy() || v.sc.glide != a {
		return
	}
	s.dropGlide(v)
	s.ownEnd(v)
}

// dropGlide stops v's glide where it is.
func (s *surface) dropGlide(v *view) {
	a := v.sc.glide
	if a == nil {
		return
	}
	if a.frame.Truthy() {
		js.Global().Call("cancelAnimationFrame", a.frame)
	}
	if a.timer.Truthy() {
		js.Global().Call("clearTimeout", a.timer)
	}
	a.step.Release()
	a.end.Release()
	v.sc.glide = nil
	v.sc.driven--
}

// stopGlide stops v's glide for a scroll command of the core's to
// offset to, as the native backends interrupt theirs: left running, its
// next frame would put the offset back. The browser's scrollend after
// the command's scroll ends the scroll; a command that leaves the view
// where it is moves nothing, so its end is sent here.
func (s *surface) stopGlide(v *view, to [2]float64) {
	if v.sc.glide == nil {
		return
	}
	s.dropGlide(v)
	stays := func(to, at, end float64) bool { return min(max(to, 0), end) == at }
	if stays(to[0], v.el.Get("scrollLeft").Float(), v.el.Get("scrollWidth").Float()-v.el.Get("clientWidth").Float()) &&
		stays(to[1], v.el.Get("scrollTop").Float(), v.el.Get("scrollHeight").Float()-v.el.Get("clientHeight").Float()) {
		s.ownEnd(v)
	}
}

// ownEnd ends a scroll driven here; the browser's scrollend that may
// follow at the same offset is not another.
func (s *surface) ownEnd(v *view) {
	v.sc.ownEnd = &[2]float64{v.el.Get("scrollLeft").Float(), v.el.Get("scrollTop").Float()}
	s.scrollEvent(v, driver.EventScrollEnd)
}
