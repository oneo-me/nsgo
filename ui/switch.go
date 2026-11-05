package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Switch is an on and off switch in Onevra's look: base.Switch, the knob
// sliding across and the track changing color on the platform. Label is
// what assistive technology reads.
type Switch struct {
	Checked  func() bool
	OnChange func(bool)
	Label    string
	// Small is the compact switch, for a row of chips.
	Small bool
}

// Render implements core.Node.
func (s Switch) Render(cx *core.Scope) core.Node {
	look := NewSwitchLook(cx, s.Checked, s.Small)
	// More to press on every side, without taking more room.
	return base.Switch{Checked: s.Checked, OnChange: s.OnChange, Label: s.Label,
		Style:          core.Style{Margin: core.All(-8), Padding: core.All(8), Cursor: core.CursorPointer},
		TrackStyleFunc: look.Track, ThumbStyleFunc: look.Thumb}
}

// SwitchLook is Onevra's switch as the styles of base.Switch's parts: a
// track that turns to the accent and a capsule knob that slides across,
// both on the platform.
type SwitchLook struct {
	Track, Thumb func() core.Style
}

// NewSwitchLook makes a switch's look, 40 × 20 or, small, 30 × 16 for
// dense rows. on is the value the app owns; the knob follows it.
func NewSwitchLook(cx *core.Scope, on func() bool, small bool) SwitchLook {
	colors := ColorsOf(cx)
	width, height, pad := core.Points(40), core.Points(20), core.Points(2)
	if small {
		width, height = 30, 16
	}
	knobH := height - 2*pad
	knobW := knobH * 1.3
	start := 0.0
	shown := core.Untracked(cx, on)
	if shown {
		start = 1
	}
	t := core.NewAnimatedValue(cx, start)
	x := Lerp(t, 0, float64(width-2*pad-knobW))
	env := cx.Env()
	cx.Effect(func() {
		next := on()
		if next == shown {
			return
		}
		shown = next
		to := 0.0
		if next {
			to = 1
		}
		// Reduced motion: the knob and the track change in place.
		if env.ReduceMotion() {
			t.Set(to)
		} else {
			Timing(cx, t, to, switchTiming)
		}
	})
	track := func() core.Style {
		return core.Style{Width: core.Pt(width), Height: core.Pt(height), CornerRadius: height / 2, Padding: core.All(pad), Background: colors.Track(),
			Animated: &core.AnimatedStyle{Background: core.Interpolate(t, core.Interpolation{Input: []float64{0, 1}, Colors: []core.Color{colors.Track(), colors.AccentFill()}})}}
	}
	return SwitchLook{Track: track, Thumb: func() core.Style {
		return core.Style{Width: core.Pt(knobW), Height: core.Pt(knobH), CornerRadius: knobH / 2, Background: colors.Knob(),
			Shadows: FinOf(cx).Knob().Shadows, Animated: &core.AnimatedStyle{TranslateX: x}}
	}}
}
