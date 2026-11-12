package ui

import (
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// Progress is a bar of how far a task is, Value from 0 to 100, or with
// Indeterminate one under way, a segment sweeping across it.
type Progress struct {
	Value         func() float32
	Indeterminate func() bool
	// Label names the task for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (p Progress) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	const height core.Points = 6
	bar := func() core.Style {
		return core.Style{Height: core.Pt(height), CornerRadius: RadiusFull, Background: colors.AccentFill()}
	}
	still := ReduceMotion(cx)
	var sweep *core.Signal[bool]
	if p.Indeterminate != nil && !still {
		sweep = core.NewSignal(cx, false)
		const period = 900 * time.Millisecond
		var tick func()
		tick = func() {
			if is(p.Indeterminate) {
				sweep.Set(!sweep.GetUntracked())
			}
			cx.AfterFunc(period, tick)
		}
		cx.AfterFunc(0, tick)
	}
	return base.Progress{Value: p.Value, Indeterminate: p.Indeterminate, Label: p.Label, TestID: p.TestID, Style: p.Style,
		TrackStyle:     core.Style{Height: core.Pt(height), CornerRadius: RadiusFull, BorderWidth: 0, Overflow: core.Hidden},
		TrackStyleFunc: Filled(colors.Inset),
		IndicatorStyleFunc: func() core.Style {
			if !is(p.Indeterminate) {
				return bar().Merge(core.Style{Transition: core.Transition{Duration: 200 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionLayout}})
			}
			if still {
				return bar().Merge(core.Style{Width: core.Pct(100), Animation: core.KeyframeAnimation{Keyframes: core.KeyframesPulse}})
			}
			s := bar().Merge(core.Style{Width: core.Pct(100.0 / 3),
				Transition: core.Transition{Duration: 900 * time.Millisecond, Easing: core.EaseInOut, Properties: core.TransitionTransform}})
			if sweep.Get() {
				s.Transform.TranslateXPercent = 200
			}
			return s
		}}
}
