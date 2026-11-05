package ui

import "github.com/oneo-me/nsgo/core"

// Press is a pressable that sinks a little under the finger and springs
// back; the spring runs on the platform, only press in and out reach the
// engine.
type Press struct {
	OnPress, OnLongPress func()
	Disabled             func() bool
	// Scale is how far it shrinks (default 0.97).
	Scale float64
	// StyleFunc merges over Style, as a View's does.
	Style     core.Style
	StyleFunc func() core.Style
	// Role is a button's by default.
	Role core.Role
	// LabelFunc replaces Label, as a View's does.
	Label     string
	LabelFunc func() string
	Selected  func() bool
	HitSlop   core.Points
	TestID    string
	Children  []core.Node
}

// Render implements core.Node.
func (p Press) Render(cx *core.Scope) core.Node {
	s := core.NewAnimatedValue(cx, 1)
	scale := p.Scale
	if scale == 0 {
		scale = 0.97
	}
	role := p.Role
	if role == 0 {
		role = core.RoleButton
	}
	var onPress, onLongPress func(core.PressEvent)
	if p.OnPress != nil {
		onPress = func(core.PressEvent) { p.OnPress() }
	}
	if p.OnLongPress != nil {
		onLongPress = func(core.PressEvent) { p.OnLongPress() }
	}
	style := p.Style
	style.Animated = &core.AnimatedStyle{Scale: s}
	style.Disabled = &core.Style{Opacity: core.Alpha(0.4)}
	return core.View{OnPress: onPress, OnLongPress: onLongPress, Disabled: p.Disabled, Selected: p.Selected,
		OnPressDown: func(core.PressEvent) { Spring(cx, s, scale, pressSpring) },
		OnPressUp:   func(core.PressEvent) { Spring(cx, s, 1, pressSpring) },
		Role:        role, Label: p.Label, LabelFunc: p.LabelFunc, HitSlop: core.All(p.HitSlop), TestID: p.TestID,
		Style: style, StyleFunc: p.StyleFunc, Children: p.Children}
}
