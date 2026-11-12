package base

import (
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// revealTime is how long a reveal takes: as long as gpui-component's
// critically damped spring with a 180 ms response takes to settle.
const revealTime = 265 * time.Millisecond

// revealTransition springs the frames a reveal moves, critically damped,
// as a height should not overshoot.
var revealTransition = core.LayoutTransition{Curve: core.LayoutSpring, Damping: 1, Duration: revealTime}

// revealFor is how long revealing height takes: revealTime, longer for a
// taller height, as it travels further, up to twice as long.
func revealFor(height core.Points) time.Duration {
	return min(revealTime+time.Duration(height)*time.Millisecond/4, 2*revealTime)
}

// Collapsible is a region whose Content expands and collapses: a "show
// more", a section of settings. Its children show either way, among them
// the app's trigger that flips Open (a Button with Expanded). Content is
// made the first time it opens and kept while closed, out of layout and
// hidden from assistive technology, so what is inside keeps its state.
type Collapsible struct {
	Open func() bool
	// Reveal opens and closes it with a clipped height that springs, in a
	// region that is there while closed too, zero high, so a gap of the
	// collapsible's comes before it, and what follows moves with it. The
	// content comes into the tree first, so the spring takes longer the
	// taller it is, and stays until the region has closed. With reduced
	// motion it comes and goes at once. Read as it is made.
	Reveal    func() bool
	Content   core.Node
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (c Collapsible) Render(cx *core.Scope) core.Node {
	opened := core.NewMemo(cx, func() bool { return is(c.Open) })
	var content core.Node
	if core.Untracked(cx, func() bool { return is(c.Reveal) }) {
		held := core.NewHolder(cx, func(*core.Scope) core.Node { return c.Content })
		shown, mounted, measured := c.reveal(cx, opened.Get)
		content = core.View{Style: core.Style{Overflow: core.Hidden},
			StyleFunc: func() core.Style {
				if shown() {
					return core.Style{}
				}
				return core.Style{Height: core.Pt(0)}
			},
			Children: []core.Node{core.Show{When: mounted, Then: func(*core.Scope) core.Node {
				return core.View{OnLayout: measured, Children: []core.Node{held.Place()}}
			}}}}
	} else {
		made := false
		content = core.Show{When: func() bool { made = made || opened.Get(); return made }, Then: func(*core.Scope) core.Node {
			return core.View{
				StyleFunc: func() core.Style {
					if opened.Get() {
						return core.Style{}
					}
					return core.Style{Display: core.DisplayNone}
				},
				Children: []core.Node{c.Content}}
		}}
	}
	return core.View{Class: c.Class, Style: c.Style, StyleFunc: c.StyleFunc, TestID: c.TestID, OnLayout: c.OnLayout, Tooltip: c.Tooltip,
		Children: append(append([]core.Node(nil), c.Children...), content)}
}

// reveal follows open for Reveal's region: whether the region shows open,
// whether the content is in the tree, and what the content's layout
// reports to. Opening puts the content in first and opens the region once
// it is measured, springing for as long as its height takes; closing
// springs as long and takes the content out after.
func (c Collapsible) reveal(cx *core.Scope, open func() bool) (shown, mounted func() bool, measured func(core.Bounds)) {
	now := core.Untracked(cx, open)
	isShown, isMounted := core.NewSignal(cx, now), core.NewSignal(cx, now)
	env := cx.Env()
	var height core.Points
	// waiting is an opening the content's first layout is to finish.
	waiting := false
	var stop func() bool
	animated := func() bool { return is(c.Reveal) && !env.ReduceMotion() }
	show := func(on bool) {
		if animated() {
			spring := revealTransition
			spring.Duration = revealFor(height)
			cx.AnimateLayout(core.LayoutAnimation{Update: &spring})
		}
		isShown.Set(on)
	}
	cx.Effect(func() {
		on := open()
		cx.Untrack(func() {
			if on == (isShown.Get() || waiting) {
				return
			}
			if stop != nil {
				stop()
				stop = nil
			}
			waiting = false
			switch {
			case on && !isMounted.Get() && animated():
				waiting = true
				isMounted.Set(true)
			case on:
				isMounted.Set(true)
				show(true)
			case animated():
				show(false)
				stop = cx.AfterFunc(revealFor(height), func() { isMounted.Set(false) })
			default:
				show(false)
				isMounted.Set(false)
			}
		})
	})
	measured = func(b core.Bounds) {
		height = b.Size.Height
		if waiting {
			waiting = false
			show(true)
		}
	}
	return isShown.Get, isMounted.Get, measured
}

// followOpen follows open for a region whose content comes and goes,
// revealed while reveal holds and motion is not reduced: each change
// animates the commit that makes it, and what it returns says whether the
// content is in the tree, turning off with open, or once a closing reveal
// has had its time, as the platform runs it and reports no end.
func followOpen(cx *core.Scope, open, reveal func() bool) func() bool {
	shown := core.Untracked(cx, open)
	mounted := core.NewSignal(cx, shown)
	env := cx.Env()
	var stop func() bool
	cx.Effect(func() {
		now := open()
		cx.Untrack(func() {
			if now == shown {
				return
			}
			shown = now
			if stop != nil {
				stop()
				stop = nil
			}
			animated := is(reveal) && !env.ReduceMotion()
			if animated {
				cx.AnimateLayout(core.LayoutAnimation{Update: &revealTransition})
			}
			if now || !animated {
				mounted.Set(now)
				return
			}
			stop = cx.AfterFunc(revealTime, func() { mounted.Set(false) })
		})
	})
	return mounted.Get
}
