package ui

import (
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// CollapsibleVariant is where a Collapsible's arrow sits beside its title.
type CollapsibleVariant uint8

const (
	// CollapsibleArrowRight follows the title. It is the default.
	CollapsibleArrowRight CollapsibleVariant = iota
	// CollapsibleArrowLeft leads the title, and the content lines up under
	// the title.
	CollapsibleArrowLeft
)

// Collapsible is a titled region whose Content the trigger shows and hides
// with a spring: a title beside an arrow, as wide as they are.
type Collapsible struct {
	Open         func() bool
	OnOpenChange func(open bool)
	Variant      CollapsibleVariant
	Text         string
	// Trigger replaces Text in the trigger, beside its arrow.
	Trigger  []core.Node
	Content  core.Node
	Disabled func() bool
	TestID   string
	Style    core.Style
}

// Render implements core.Node.
func (c Collapsible) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	title := c.Trigger
	if title == nil {
		title = []core.Node{line(c.Text, TypeBody.Merge(core.Style{FlexShrink: 1}), 1)}
	}
	turn := disclosureArrow(cx, func() bool { return is(c.Open) })
	children := append(append([]core.Node(nil), title...), turn)
	indent := core.Points(2)
	if c.Variant == CollapsibleArrowLeft {
		children = append([]core.Node{turn}, title...)
		indent = 22
	}
	return base.Collapsible{Open: c.Open, Reveal: func() bool { return true }, Style: c.Style, TestID: c.TestID,
		Content: core.View{Style: core.Style{Padding: core.Edges{Top: 4, Bottom: 4, Left: indent}, Gap: 8}, Children: []core.Node{c.Content}},
		Children: []core.Node{base.Button{Expanded: func() bool { return is(c.Open) }, Disabled: c.Disabled, Label: c.Text,
			OnPress: func(core.PressEvent) {
				if c.OnOpenChange != nil {
					c.OnOpenChange(!core.Untracked(cx, func() bool { return is(c.Open) }))
				}
			},
			Style: core.Style{Direction: core.Row, AlignSelf: core.AlignStart, AlignItems: core.AlignCenter, Gap: 4,
				Padding: core.Edges{Top: 4, Right: 8, Bottom: 4, Left: 2}, CornerRadius: RadiusControl, Disabled: Dimmed()},
			StyleFunc: func() core.Style { return core.Style{Color: colors.Text(), Focus: FocusRing(cx, 2)} }, Children: children}}}
}

// disclosureArrow is the arrow of what open says is open: it points along
// the reading direction, and turns down as it opens, in the time a
// disclosure takes to open.
func disclosureArrow(cx *core.Scope, open func() bool) core.Node {
	env := cx.Env()
	icon := IconOf(func() *core.SVG {
		if env.RTL() {
			return lucide.ChevronLeft()
		}
		return lucide.ChevronRight()
	}, iconSize, ColorsOf(cx).Muted)
	s := core.Style{FlexShrink: 0}
	if !ReduceMotion(cx) {
		s.Transition = core.Transition{Duration: 200 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionTransform}
	}
	return core.View{Style: s, StyleFunc: func() core.Style {
		var degrees float32
		switch {
		case !open():
		case env.RTL():
			degrees = -90
		default:
			degrees = 90
		}
		return core.Style{Transform: core.Transform{Rotate: degrees}}
	}, Children: []core.Node{icon}}
}
