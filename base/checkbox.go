package base

import "github.com/oneo-me/nsgo/core"

// Checkbox is checked, unchecked or indeterminate: an option, an item of a
// selection, a "select all". Its parts are the indicator, the box holding
// the app's Indicator mark, then its children, the label assistive
// technology reads.
type Checkbox struct {
	Checked func() bool
	// Indeterminate is neither checked nor unchecked, over Checked while
	// it holds; a press then asks for checked.
	Indeterminate func() bool
	Disabled      func() bool
	// OnChange receives the value a press asks for: true from unchecked
	// and indeterminate, false from checked. The checkbox shows what
	// Checked returns.
	OnChange func(checked bool)
	// Role is what it is for assistive technology; RoleCheckbox when 0.
	Role core.Role

	Class              string
	Style              core.Style
	StyleFunc          func() core.Style
	IndicatorClass     string
	IndicatorStyle     core.Style
	IndicatorStyleFunc func() core.Style
	// Indicator is the mark inside the indicator, which the app shows by
	// the value it owns.
	Indicator core.Node
	Children  []core.Node
	Label     string
	Hint      string
	TestID    string
	// HitSlop is how far outside the checkbox presses count.
	HitSlop core.Edges
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (c Checkbox) Render(*core.Scope) core.Node {
	role := c.Role
	if role == core.RoleNone {
		role = core.RoleCheckbox
	}
	// Indeterminate wins over checked, as gpui-base's CheckboxState holds
	// one of the three.
	shows := func() core.PartStates {
		mixed := is(c.Indeterminate)
		return core.PartStates{Checked: is(c.Checked) && !mixed, Indeterminate: mixed, Disabled: is(c.Disabled)}
	}
	var mark []core.Node
	if c.Indicator != nil {
		mark = []core.Node{c.Indicator}
	}
	return core.View{Class: c.Class, Style: c.Style, StyleFunc: c.StyleFunc, Role: role, Label: c.Label, Hint: c.Hint,
		TestID: c.TestID, OnLayout: c.OnLayout, Tooltip: c.Tooltip, HitSlop: c.HitSlop, Disabled: c.Disabled, Indeterminate: c.Indeterminate,
		Checked: func() bool { return is(c.Checked) && !is(c.Indeterminate) },
		OnPress: func(core.PressEvent) {
			if c.OnChange != nil {
				c.OnChange(is(c.Indeterminate) || !is(c.Checked))
			}
		},
		Children: append([]core.Node{core.View{Class: c.IndicatorClass, Style: c.IndicatorStyle, StyleFunc: c.IndicatorStyleFunc,
			PartStates: shows, Children: mark}}, c.Children...)}
}
