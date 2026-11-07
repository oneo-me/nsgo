package base

import (
	"time"

	"github.com/oneo-me/nsgo/core"
)

// holdLinear moves a hold's fill at one speed both ways.
var holdLinear = core.BezierEasing(core.Linear)

// HoldToConfirm is a button for what a stray press must not trigger: held
// down for Duration, 1.2 s when 0, a fill goes from 0 to 1, and once it is
// full OnConfirm runs and assistive technology hears ConfirmedLabel.
// Letting go early rewinds the fill. The keyboard and assistive technology,
// which cannot hold, start it with a press and cancel it with another or
// by moving the focus away. Confirmed is the app's: true holds it full, and
// back to false rewinds it. It draws nothing of the fill: Content gets it
// on the platform's frames.
type HoldToConfirm struct {
	Duration time.Duration
	// Rewind is how long an empty-to-full fill takes to drain, 400 ms when
	// 0.
	Rewind    time.Duration
	Confirmed func() bool
	OnConfirm func()
	// ConfirmedLabel is announced as the hold confirms.
	ConfirmedLabel string
	// OnHoldChange hears a hold start and end, for press feedback.
	OnHoldChange func(holding bool)
	// Content draws the button from the fill, a value from 0 to 1.
	Content   func(cx *core.Scope, fill *core.AnimatedValue) []core.Node
	Disabled  func() bool
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Label     string
	LabelFunc func() string
	Hint      string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (h HoldToConfirm) Render(cx *core.Scope) core.Node {
	duration, rewind := h.Duration, h.Rewind
	if duration <= 0 {
		duration = 1200 * time.Millisecond
	}
	if rewind <= 0 {
		rewind = 400 * time.Millisecond
	}
	confirmed := func() bool { return is(h.Confirmed) }
	holding := core.NewSignal(cx, false)
	setHolding := func(on bool) {
		holding.Set(on)
		if h.OnHoldChange != nil {
			h.OnHoldChange(on)
		}
	}
	// The fill moves linearly both ways, so where it is follows from when
	// its last run started, where from and toward what.
	var runStart time.Time
	var runFor time.Duration
	runFrom, runTo := 0.0, 0.0
	if core.Untracked(cx, confirmed) {
		runFrom, runTo = 1, 1
	}
	fill := core.NewAnimatedValue(cx, runTo)
	at := func() float64 {
		if runFor <= 0 {
			return runTo
		}
		p := min(float64(cx.Now().Sub(runStart))/float64(runFor), 1)
		return runFrom + (runTo-runFrom)*p
	}
	run := func(to float64, d time.Duration, done func(bool)) {
		runFrom, runTo, runStart, runFor = at(), to, cx.Now(), d
		if d <= 0 {
			fill.Set(to)
			return
		}
		cx.StartAnimation(core.Timing(fill, to, core.TimingConfig{Duration: d, Easing: holdLinear}), done)
	}
	begin := func() {
		if core.Untracked(cx, confirmed) || core.Untracked(cx, func() bool { return is(h.Disabled) }) || holding.GetUntracked() {
			return
		}
		setHolding(true)
		run(1, time.Duration((1-at())*float64(duration)), func(finished bool) {
			if !finished || !holding.GetUntracked() {
				return
			}
			setHolding(false)
			if h.ConfirmedLabel != "" {
				cx.Announce(h.ConfirmedLabel)
			}
			if h.OnConfirm != nil {
				h.OnConfirm()
			}
		})
	}
	release := func() {
		if !holding.GetUntracked() {
			return
		}
		setHolding(false)
		run(0, time.Duration(at()*float64(rewind)), nil)
	}
	cx.Effect(func() {
		on := confirmed()
		if core.Untracked(cx, holding.Get) {
			return
		}
		if on && at() < 1 {
			run(1, 0, nil)
		} else if !on && at() > 0 {
			run(0, rewind, nil)
		}
	})
	var children []core.Node
	if h.Content != nil {
		children = h.Content(cx, fill)
	}
	// upAt is when a pointer's press last ended.
	var upAt time.Time
	return core.View{Class: h.Class, Style: h.Style, StyleFunc: h.StyleFunc, Role: core.RoleButton, Label: h.Label, LabelFunc: h.LabelFunc, Hint: h.Hint, TestID: h.TestID,
		OnLayout: h.OnLayout, Tooltip: h.Tooltip, Disabled: h.Disabled,
		OnPressDown: func(core.PressEvent) { begin() },
		OnPressUp: func(core.PressEvent) {
			upAt = cx.Now()
			release()
		},
		OnPress: func(core.PressEvent) {
			// A pointer's press may end in a press too, right after it lets
			// go; only the keyboard's and assistive technology's start or
			// cancel a hold here.
			if !upAt.IsZero() && cx.Now().Sub(upAt) < 250*time.Millisecond {
				upAt = time.Time{}
				return
			}
			if holding.GetUntracked() {
				release()
			} else {
				begin()
			}
		},
		OnBlur: release, Children: children}
}
