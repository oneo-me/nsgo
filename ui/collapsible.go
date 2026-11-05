package ui

import (
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Collapsible is a titled region whose Content the trigger row shows and
// hides; the content is made the first time it opens and kept after.
type Collapsible struct {
	Open         func() bool
	OnOpenChange func(open bool)
	Text         string
	// Trigger replaces Text in the trigger row, before its chevron.
	Trigger  []core.Node
	Content  core.Node
	Disabled func() bool
	TestID   string
	Style    core.Style
}

// Render implements core.Node.
func (c Collapsible) Render(cx *core.Scope) core.Node {
	return base.Collapsible{Open: c.Open, Content: core.View{Style: core.Style{Padding: core.Edges{Top: 4, Bottom: 8}},
		Children: []core.Node{c.Content}}, Style: c.Style, TestID: c.TestID,
		Children: []core.Node{collapsibleTrigger(cx, c, core.Style{Padding: core.Symmetric(0, 8)})}}
}

// collapsibleTrigger is the row that opens and closes c: its title and a
// chevron that turns as it opens.
func collapsibleTrigger(cx *core.Scope, c Collapsible, s core.Style) core.Node {
	colors := ColorsOf(cx)
	title := c.Trigger
	if title == nil {
		title = []core.Node{line(c.Text, TypeLabel.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), 0)}
	}
	chevron := core.Style{FlexShrink: 0}
	if !ReduceMotion(cx) {
		chevron.Transition = core.Transition{Duration: 160 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionTransform}
	}
	turn := core.View{Style: chevron, StyleFunc: func() core.Style {
		if is(c.Open) {
			return core.Style{Transform: core.Transform{Rotate: 180}}
		}
		return core.Style{}
	}, Children: []core.Node{Icon(lucide.ChevronDown, iconSize, colors.Muted)}}
	return base.Button{Expanded: func() bool { return is(c.Open) }, Disabled: c.Disabled, Label: c.Text,
		OnPress: func(core.PressEvent) {
			if c.OnOpenChange != nil {
				c.OnOpenChange(!core.Untracked(cx, func() bool { return is(c.Open) }))
			}
		},
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(ButtonHeight + 4),
			CornerRadius: RadiusControl, Transition: quick, Disabled: Dimmed()}.Merge(s),
		StyleFunc: func() core.Style {
			return core.Style{Color: colors.Text(), Hover: &core.Style{Background: colors.Hover()},
				Active: &core.Style{Background: colors.Pressed()}, Focus: FocusRing(cx, -2)}
		},
		Children: append(append([]core.Node(nil), title...), turn)}
}
