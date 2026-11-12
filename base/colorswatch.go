package base

import "github.com/nsgo-dev/nsgo/core"

// ColorSwatch shows a color: Color, or the picker's around it without one.
type ColorSwatch struct {
	Color func() core.Color
	// CheckerboardColor shows squares CheckerboardSize wide, 4 when 0.
	CheckerboardColor core.Color
	CheckerboardSize  core.Points
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
	shown := func() core.Color {
		switch {
		case s.Color != nil:
			return s.Color()
		case picker != nil:
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
	children := append([]core.Node{Checkerboard(s.CheckerboardColor, cmpPoints(s.CheckerboardSize, 4)),
		core.View{Style: colorFill.Merge(core.Style{PointerEvents: core.PointerEventsNone}), NoFocus: true,
			StyleFunc: func() core.Style { return core.Style{Background: core.SetColor(shown())} }}}, s.Children...)
	return core.View{Class: s.Class, Style: core.Style{OverflowX: core.Hidden, OverflowY: core.Hidden}.Merge(s.Style), StyleFunc: s.StyleFunc,
		Accessible: true, Role: core.RoleImage, LabelFunc: label, TestID: s.TestID, OnLayout: s.OnLayout, Tooltip: s.Tooltip, Children: children}
}
