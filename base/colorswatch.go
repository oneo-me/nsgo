package base

import "github.com/oneo-me/nsgo/core"

// ColorSwatch shows a color: Color, a preset a press picks in the picker
// around it and reports with OnPress, or the picker's own without one.
// Its Selected style holds while it is the picker's color.
type ColorSwatch struct {
	Color    func() core.Color
	Disabled func() bool
	// CheckerboardColor shows squares CheckerboardSize wide, 4 when 0.
	CheckerboardColor core.Color
	CheckerboardSize  core.Points
	OnPress           func(color core.Color)
	Children          []core.Node
	Class             string
	Style             core.Style
	StyleFunc         func() core.Style
	Label             string
	TestID            string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (s ColorSwatch) Render(cx *core.Scope) core.Node {
	picker := colorKey.Use(cx)
	own := func() core.Color {
		if s.Color != nil {
			return s.Color()
		}
		return 0
	}
	shown := func() core.Color {
		if c := own(); c != 0 {
			return c
		}
		if picker != nil {
			return picker.model.Get().color()
		}
		return 0x000000ff
	}
	label := func() string {
		if s.Label != "" {
			return s.Label
		}
		return colorModelOf(shown()).format(ColorFormatHex)
	}
	children := append([]core.Node{checkerboard(s.CheckerboardColor, cmpPoints(s.CheckerboardSize, 4)),
		core.View{Style: colorFill.Merge(core.Style{PointerEvents: core.PointerEventsNone}), NoFocus: true,
			StyleFunc: func() core.Style { return core.Style{Background: core.SetColor(shown())} }}}, s.Children...)
	return core.View{Class: s.Class, Style: core.Style{OverflowX: core.Hidden, OverflowY: core.Hidden}.Merge(s.Style), StyleFunc: s.StyleFunc, Disabled: s.Disabled,
		Accessible: true, Role: core.RoleButton, LabelFunc: label, TestID: s.TestID, OnLayout: s.OnLayout, Tooltip: s.Tooltip,
		Selected: func() bool { c := own(); return c != 0 && picker != nil && c == picker.model.Get().color() },
		OnPress: func(core.PressEvent) {
			c := core.Untracked(cx, own)
			if c == 0 {
				return
			}
			if picker != nil && !core.Untracked(cx, picker.isDisabled) {
				picker.set(picker.now().follow(c))
				picker.end()
			}
			if s.OnPress != nil {
				s.OnPress(c)
			}
		},
		Children: children}
}
