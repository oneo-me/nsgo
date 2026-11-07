package base

import (
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// Toolbar is a bar of controls: assistive technology hears one toolbar,
// each control still an element of its own, and the keyboard focus one Tab
// stop, the item it last left, else the first enabled one. The arrows
// along Axis move the focus around the ends, Home and End to the first and
// last, without pressing, and Space and Return press. Its items are the
// Buttons, Toggles and Links inside it, those of its ToolbarGroups and
// ToggleGroups too; other controls stay stops of their own and keep their
// arrows. It lays its children out along Axis and has no look.
type Toolbar struct {
	// Axis is the way its items run, read as it is made.
	Axis Axis
	// Disabled disables every item, whatever its own.
	Disabled  func() bool
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// axisStyle lays children out along a.
func axisStyle(a Axis, s core.Style) core.Style {
	if a == Horizontal {
		return core.Style{Direction: core.Row}.Merge(s)
	}
	return s
}

// Render implements core.Node.
func (t Toolbar) Render(cx *core.Scope) core.Node {
	group := newRoving(cx, true)
	group.toolbar, group.axis, group.hasAxis = true, t.Axis, true
	barKey.Provide(cx, &bar{roving: group, disabled: t.Disabled, axis: t.Axis, toolbar: true})
	return core.View{Class: t.Class, Style: axisStyle(t.Axis, t.Style), StyleFunc: t.StyleFunc, Role: core.RoleToolbar, Label: t.Label, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Children: append(slices.Clip(t.Children), group.keys())}
}

// ToolbarGroup is a run of related items in a Toolbar (undo and redo):
// assistive technology hears the group and its label; its items are the
// toolbar's, and it lays them out along the toolbar's Axis.
type ToolbarGroup struct {
	// Disabled disables every item of the group, whatever its own.
	Disabled  func() bool
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t ToolbarGroup) Render(cx *core.Scope) core.Node {
	style := t.Style
	if outer := barKey.Use(cx); outer != nil {
		barKey.Provide(cx, nestedBar(outer, t.Disabled))
		if outer.toolbar {
			style = axisStyle(outer.axis, style)
		}
	}
	return core.View{Class: t.Class, Style: style, StyleFunc: t.StyleFunc, Role: core.RoleGroup, Label: t.Label, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Children: t.Children}
}

// ToolbarSeparator is a line between runs of a Toolbar's items, a view the
// app styles, never a stop of the keyboard focus.
type ToolbarSeparator struct {
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	TestID    string
}

// Render implements core.Node.
func (t ToolbarSeparator) Render(*core.Scope) core.Node {
	return core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, TestID: t.TestID, NoFocus: true}
}
