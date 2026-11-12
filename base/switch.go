package base

import "github.com/nsgo-dev/nsgo/core"

// Switch is on or off and takes effect at once, as a setting. Its parts
// are a track and a thumb inside it, then its children, the label.
type Switch struct {
	Checked  func() bool
	Disabled func() bool
	// OnChange receives the value a press asks for, the opposite of
	// Checked's.
	OnChange func(checked bool)

	Class          string
	Style          core.Style
	StyleFunc      func() core.Style
	TrackClass     string
	TrackStyle     core.Style
	TrackStyleFunc func() core.Style
	ThumbClass     string
	ThumbStyle     core.Style
	ThumbStyleFunc func() core.Style
	Children       []core.Node
	Label          string
	Hint           string
	TestID         string
	// HitSlop is how far outside the switch presses count.
	HitSlop core.Edges
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (s Switch) Render(*core.Scope) core.Node {
	shows := func() core.PartStates { return core.PartStates{Checked: is(s.Checked), Disabled: is(s.Disabled)} }
	thumb := core.View{Class: s.ThumbClass, Style: s.ThumbStyle, StyleFunc: s.ThumbStyleFunc, PartStates: shows}
	track := core.View{Class: s.TrackClass, Style: s.TrackStyle, StyleFunc: s.TrackStyleFunc, PartStates: shows,
		Children: []core.Node{thumb}}
	return core.View{Class: s.Class, Style: s.Style, StyleFunc: s.StyleFunc, Role: core.RoleSwitch, Label: s.Label, Hint: s.Hint, TestID: s.TestID, OnLayout: s.OnLayout, Tooltip: s.Tooltip, HitSlop: s.HitSlop,
		Disabled: s.Disabled, Checked: s.Checked,
		OnPress: func(core.PressEvent) {
			if s.OnChange != nil {
				s.OnChange(!is(s.Checked))
			}
		},
		Children: append([]core.Node{track}, s.Children...)}
}
