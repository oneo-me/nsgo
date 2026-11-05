package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// ToggleButton is Onevra's toggle button, 28 pt tall: a quiet bordered
// face while off, filled with the accent while On, Text with an optional
// leading Icon or the Icon alone. A press asks OnChange for the other
// state; assistive technology hears it as a selected button. Label is
// what it reads, Text when empty.
type ToggleButton struct {
	Text     string
	Icon     func() *core.SVG
	Label    string
	On       func() bool
	OnChange func(on bool)
	Disabled func() bool
	TestID   string
}

// Render implements core.Node.
func (b ToggleButton) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	label := b.Label
	if label == "" {
		label = b.Text
	}
	ink := func() core.Color {
		if is(b.On) {
			return colors.OnAccent()
		}
		return colors.Text()
	}
	var children []core.Node
	if b.Icon != nil {
		children = append(children, Icon(b.Icon, 14, ink))
	}
	if b.Text != "" || b.Icon == nil {
		t := Text(b.Text, 13, Regular, ink)
		t.MaxLines = 1
		children = append(children, t)
	}
	shape := core.Style{Height: core.Pt(ButtonHeight), Padding: core.Symmetric(0, 14)}
	if b.Text == "" && b.Icon != nil {
		shape = core.Style{Width: core.Pt(ButtonHeight), Height: core.Pt(ButtonHeight)}
	}
	face := func() core.Style {
		s := shape.Merge(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
			CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, Transition: quick})
		if is(b.On) {
			p := cx.Palette()
			s.Background, s.BorderColor = p.Accent, p.Accent
			s.GroupStyle = &core.GroupStyle{Hover: &core.Style{Background: p.AccentHover, BorderColor: p.AccentHover},
				Active: &core.Style{Background: p.AccentPressed, BorderColor: p.AccentPressed}}
			return s
		}
		s.Background, s.BorderColor = transparent, colors.Border()
		s.GroupStyle = &core.GroupStyle{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}}
		return s
	}
	return base.Button{Label: label, TestID: b.TestID, Selected: b.On, Disabled: b.Disabled,
		OnPress: func(core.PressEvent) {
			if b.OnChange != nil {
				b.OnChange(!is(b.On))
			}
		},
		Style:     core.Style{Group: "button", CornerRadius: RadiusControl, Disabled: Dimmed()}.Merge(slop(cx, ButtonHeight)),
		StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		Children:  []core.Node{core.View{StyleFunc: face, Children: children}}}
}
