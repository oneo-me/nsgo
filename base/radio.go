package base

import (
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// Radio is one option of a set the user picks exactly one of, in a
// RadioGroup with the others. Its children are the app's indicator and
// label; a GroupStyle under Style.Group follows the radio's Checked.
type Radio struct {
	Checked  func() bool
	Disabled func() bool
	// OnChange receives true when an unchecked radio is pressed: the app
	// checks it and unchecks the others. A checked radio reports nothing,
	// as a radio cannot uncheck itself.
	OnChange  func(checked bool)
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	Hint      string
	TestID    string
	// HitSlop is how far outside the radio presses count.
	HitSlop core.Edges
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (r Radio) Render(cx *core.Scope) core.Node {
	check := func() {
		if r.OnChange != nil && !is(r.Checked) {
			r.OnChange(true)
		}
	}
	return rovingView(cx, core.View{Class: r.Class, Style: r.Style, StyleFunc: r.StyleFunc, Role: core.RoleRadio, Label: r.Label, Hint: r.Hint, TestID: r.TestID, OnLayout: r.OnLayout, Tooltip: r.Tooltip, HitSlop: r.HitSlop,
		// Checked and selected both: some screen readers read one, some
		// the other, as gpui-base's Radio states toggled and selected.
		Disabled: r.Disabled, Checked: r.Checked, Selected: r.Checked,
		OnPress:  func(core.PressEvent) { check() },
		Children: r.Children}, r.Checked, check)
}

// RadioGroup holds a set of Radios, which assistive technology hears as
// one set and the keyboard focus as one Tab stop: the checked radio, else
// the first enabled one. Up and Left, Down and Right move to the previous
// and next enabled radio, around the ends, and check it. It holds no
// value: each radio is told whether it is checked.
type RadioGroup struct {
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
func (g RadioGroup) Render(cx *core.Scope) core.Node {
	group := newRoving(cx, false)
	return core.View{Class: g.Class, Style: g.Style, StyleFunc: g.StyleFunc, Role: core.RoleRadioGroup, Label: g.Label, TestID: g.TestID, OnLayout: g.OnLayout, Tooltip: g.Tooltip,
		Children: append(slices.Clip(g.Children), group.keys())}
}
