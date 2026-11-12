package base

import (
	"slices"

	"github.com/nsgo-dev/nsgo/core"
)

// Toggle is a button that is on or off (bold, a favorite, a view option),
// alone or in a ToggleGroup. It holds no value: a press reports the next
// one with OnChange and it shows what Pressed returns. Style.Pressed and
// GroupStyle.Pressed style it and its content while it is on.
type Toggle struct {
	Pressed  func() bool
	Disabled func() bool
	// OnChange receives the value a press asks for, the opposite of
	// Pressed's.
	OnChange  func(pressed bool)
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	Hint      string
	TestID    string
	// HitSlop and HitSlopFunc are how far outside it presses count.
	HitSlop     core.Edges
	HitSlopFunc func() core.Edges
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t Toggle) Render(cx *core.Scope) core.Node {
	v := core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Role: core.RoleButton, Label: t.Label, Hint: t.Hint, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		HitSlop: t.HitSlop, HitSlopFunc: t.HitSlopFunc, Disabled: t.Disabled, Pressed: func() bool { return is(t.Pressed) },
		OnPress: func(core.PressEvent) {
			if t.OnChange != nil {
				t.OnChange(!core.Untracked(cx, func() bool { return is(t.Pressed) }))
			}
		},
		Children: t.Children}
	return barItem(cx, v, func() bool { return is(t.Pressed) })
}

// ToggleGroup is the toolbar around a set of Toggles: assistive technology
// hears one toolbar, the keyboard focus one Tab stop, at the first pressed
// toggle, else the first enabled one. The arrows move the focus around the
// ends, Home and End to the first and last, without pressing, and Space
// and Return press. It holds no value; the app keeps which are pressed,
// any number of them or exactly one. Inside a Toolbar it is one of the
// toolbar's groups, whose stop it shares.
type ToggleGroup struct {
	Disabled func() bool
	// Label names it for assistive technology.
	Label     string
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t ToggleGroup) Render(cx *core.Scope) core.Node {
	role, children := core.RoleGroup, t.Children
	if outer := barKey.Use(cx); outer != nil {
		barKey.Provide(cx, nestedBar(outer, t.Disabled))
	} else {
		role = core.RoleToolbar
		group := newRoving(cx, true)
		barKey.Provide(cx, &bar{roving: group, disabled: t.Disabled})
		children = append(slices.Clip(children), group.keys())
	}
	return core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Role: role, Label: t.Label, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Children: children}
}
