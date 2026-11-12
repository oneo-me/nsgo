package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// Switch is an on and off switch in ui's look: base.Switch, the knob
// sliding across and the track changing color on the platform. Label is
// what assistive technology reads.
type Switch struct {
	Checked  func() bool
	OnChange func(bool)
	Label    string
	Size     Size
	Variant  SwitchVariant
	TestID   string
}

// SwitchVariant is a Switch's look.
type SwitchVariant uint8

const (
	// SwitchDefault is ui's own look. It is the default.
	SwitchDefault SwitchVariant = iota
	// SwitchSystem is the platform's own switch where it has one, macOS's
	// NSSwitch and iOS's UISwitch (core.SystemSwitch), sized as the
	// system's; elsewhere it is ui's look.
	SwitchSystem
)

// Render implements core.Node.
func (s Switch) Render(cx *core.Scope) core.Node {
	if s.Variant == SwitchSystem {
		return core.Show{When: func() bool { return core.SystemSwitchAvailable(cx) }, Else: s.own,
			Then: func(*core.Scope) core.Node {
				return core.SystemSwitch{Checked: s.Checked, OnChange: s.OnChange, Label: s.Label, TestID: s.TestID,
					Size: sized(s.Size, "", "small", "large")}
			}}
	}
	return s.own(cx)
}

// own is the switch in ui's look.
func (s Switch) own(cx *core.Scope) core.Node {
	look := NewSwitchLook(cx, s.Checked, s.Size)
	// More to press on every side, without taking more room.
	return base.Switch{Checked: s.Checked, OnChange: s.OnChange, Label: s.Label, TestID: s.TestID,
		Style:          core.Style{Margin: core.All(-8), Padding: core.All(8)},
		TrackStyleFunc: look.Track, ThumbStyleFunc: look.Thumb}
}

// SwitchLook is ui's switch as the styles of base.Switch's parts: a
// track that turns to the accent and a capsule knob that slides across,
// both on the platform.
type SwitchLook struct {
	Track, Thumb func() core.Style
}

// NewSwitchLook makes a switch's look: 40 × 20, 30 × 16 small and 48 × 24
// large. on is the value the app owns; the knob follows it.
func NewSwitchLook(cx *core.Scope, on func() bool, size Size) SwitchLook {
	colors := ColorsOf(cx)
	width, height := sized[core.Points](size, 40, 30, 48), sized[core.Points](size, 20, 16, 24)
	const pad core.Points = 2
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
	// A memo, since the effect's next run stops the slide it started and on
	// may read what changes without changing its value, as an app's revision.
	value := core.NewMemo(cx, on)
	cx.Effect(func() {
		next := value.Get()
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
