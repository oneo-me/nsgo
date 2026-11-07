package plot

import "github.com/oneo-me/nsgo/core"

// keyStep is where a key moves a plot's hover. gpui-base's plot has no
// keyboard of its own and WAI-ARIA has no pattern for charts, so the keys
// are the few a row picker takes (a slider's Left, Right, Home and End, a
// listbox's Escape). Over a grid of cells they are a data grid's. Left and
// Right follow the screen on a reversed x scale; Home and End, and screen
// readers' increment and decrement, keep the rows' order.
type keyStep uint8

const (
	stepPrevious keyStep = iota
	stepNext
	stepFirst
	stepLast
	stepUp
	stepDown
)

// mirrored is the step the other arrow takes.
func (s keyStep) mirrored() keyStep {
	switch s {
	case stepPrevious:
		return stepNext
	case stepNext:
		return stepPrevious
	}
	return s
}

// stepRow is the row step moves the hover to from from (-1: none), among
// count rows of which those placed can be hovered. With no row hovered, the
// first key starts at the first row or the last; at either end the hover
// stays.
func stepRow(count int, placed func(int) bool, from int, step keyStep) (int, bool) {
	first := func() (int, bool) {
		for i := range count {
			if placed(i) {
				return i, true
			}
		}
		return 0, false
	}
	last := func() (int, bool) {
		for i := count - 1; i >= 0; i-- {
			if placed(i) {
				return i, true
			}
		}
		return 0, false
	}
	here := func() bool { return from >= 0 && from < count && placed(from) }
	switch {
	case step == stepFirst || step == stepNext && from < 0:
		return first()
	case step == stepLast || step == stepPrevious && from < 0:
		return last()
	case step == stepUp || step == stepDown:
		return from, here()
	case step == stepNext:
		for j := from + 1; j < count; j++ {
			if placed(j) {
				return j, true
			}
		}
		if here() {
			return from, true
		}
		// A row the rows no longer hold moves to the nearest end.
		return last()
	}
	for j := min(from, count) - 1; j >= 0; j-- {
		if placed(j) {
			return j, true
		}
	}
	if here() {
		return from, true
	}
	return first()
}

// bindKeys binds the plot's keys while active, a focus of the plot, so they
// never take a key from the rest of the app: each moves the hover to the
// row to finds for its step, Left's and Right's swapped while mirrored, Up
// and Down only while the rows are a grid (elsewhere they go on to what
// they do around the plot, a scroll), and Escape, go's clear, only while a
// row is hovered (elsewhere it goes on to close a dialog).
func bindKeys(active, mirrored, grid func() bool, hovered *core.Signal[int], to func(keyStep) (int, bool),
	goTo func(int, bool), clear func()) core.Node {
	arrow := func(step keyStep) func() {
		return func() {
			if mirrored() && !grid() {
				goTo(to(step.mirrored()))
				return
			}
			goTo(to(step))
		}
	}
	return core.Group{
		core.Show{When: active, Then: func(cx *core.Scope) core.Node {
			cx.OnKeyWith("left", core.KeyOptions{Repeat: true}, arrow(stepPrevious))
			cx.OnKeyWith("right", core.KeyOptions{Repeat: true}, arrow(stepNext))
			cx.OnKey("home", func() { goTo(to(stepFirst)) })
			cx.OnKey("end", func() { goTo(to(stepLast)) })
			return nil
		}},
		core.Show{When: func() bool { return active() && grid() }, Then: func(cx *core.Scope) core.Node {
			cx.OnKeyWith("up", core.KeyOptions{Repeat: true}, func() { goTo(to(stepUp)) })
			cx.OnKeyWith("down", core.KeyOptions{Repeat: true}, func() { goTo(to(stepDown)) })
			return nil
		}},
		core.Show{When: func() bool { return active() && hovered.Get() >= 0 }, Then: func(cx *core.Scope) core.Node {
			cx.OnKey("escape", clear)
			return nil
		}},
	}
}
