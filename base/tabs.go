package base

import "github.com/oneo-me/nsgo/core"

// Tab is one tab of a tab list, in Tabs with the others. The app keeps
// which is selected and shows its content beside the list.
type Tab struct {
	Selected func() bool
	Disabled func() bool
	// OnPress runs on every press, the selected tab's included.
	OnPress   func(core.PressEvent)
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	Hint      string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t Tab) Render(*core.Scope) core.Node {
	press := t.OnPress
	if press == nil {
		press = func(core.PressEvent) {}
	}
	return core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Role: core.RoleTab, Label: t.Label, Hint: t.Hint, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Disabled: t.Disabled, Selected: t.Selected, OnPress: press, Children: t.Children}
}

// Tabs is the tab list around a set of Tabs, which assistive technology
// hears as one list. It holds no value and no content.
type Tabs struct {
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
func (t Tabs) Render(*core.Scope) core.Node {
	return core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Role: core.RoleTabList, Label: t.Label, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Children: t.Children}
}
