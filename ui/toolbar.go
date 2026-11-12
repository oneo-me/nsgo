package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// Toolbar is a bar of controls, on a quiet inset panel: base.Toolbar's one
// Tab stop whose arrows move between its IconButtons, ToggleButtons and
// Links.
type Toolbar struct {
	Axis     base.Axis
	Disabled func() bool
	Children []core.Node
	Label    string
	TestID   string
	Style    core.Style
}

// Render implements core.Node.
func (t Toolbar) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	return base.Toolbar{Axis: t.Axis, Disabled: t.Disabled, Label: t.Label, TestID: t.TestID, Children: t.Children,
		Style: core.Style{AlignItems: core.AlignCenter, AlignSelf: core.AlignStart, Gap: 2, Padding: core.All(2), CornerRadius: RadiusControl + 2},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Inset()}.Merge(t.Style)
		}}
}

// ToolbarGroup is a run of related items in a Toolbar, read as one unit.
type ToolbarGroup struct {
	Disabled func() bool
	Children []core.Node
	Label    string
	TestID   string
}

// Render implements core.Node.
func (t ToolbarGroup) Render(*core.Scope) core.Node {
	return base.ToolbarGroup{Disabled: t.Disabled, Label: t.Label, TestID: t.TestID, Children: t.Children,
		Style: core.Style{AlignItems: core.AlignCenter, Gap: 2}}
}

// ToolbarSeparator is a hairline between runs of a Toolbar's items.
type ToolbarSeparator struct {
	TestID string
}

// Render implements core.Node.
func (t ToolbarSeparator) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	return base.ToolbarSeparator{TestID: t.TestID,
		Style:     core.Style{Width: core.Pt(1), Height: core.Pt(16), Margin: core.Symmetric(0, 4)},
		StyleFunc: Filled(colors.Line)}
}
