package ui

import (
	"slices"
	"strconv"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// AccordionItem is one section of an Accordion.
type AccordionItem struct {
	Text     string
	Content  core.Node
	Disabled func() bool
	TestID   string
}

// Accordion is a bordered box of sections divided by hairlines, as
// ui's, on base.Accordion: each title a heading over a button, each
// panel a region that springs open. The app keeps which are open: one at a
// time, or any.
type Accordion struct {
	Items        []AccordionItem
	Open         func(index int) bool
	OnOpenChange func(index int, open bool)
	TestID       string
	Style        core.Style
}

// Render implements core.Node.
func (a Accordion) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	open := func() []string {
		var out []string
		for i := range a.Items {
			if a.Open != nil && a.Open(i) {
				out = append(out, strconv.Itoa(i))
			}
		}
		return out
	}
	// The app says what opens: the accordion reports each change as one,
	// so every item is free to stay open.
	change := func(next []string) {
		now := core.Untracked(cx, open)
		for i := range a.Items {
			id := strconv.Itoa(i)
			if was, is := slices.Contains(now, id), slices.Contains(next, id); was != is && a.OnOpenChange != nil {
				a.OnOpenChange(i, is)
			}
		}
	}
	reveal := func() bool { return true }
	face := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(10, 12),
		CornerRadius: RadiusControl, Disabled: Dimmed()}
	var sections []core.Node
	for i, item := range a.Items {
		turn := disclosureArrow(cx, func() bool { return a.Open != nil && a.Open(i) })
		section := base.AccordionItem{Value: strconv.Itoa(i), Disabled: item.Disabled, TestID: item.TestID,
			Children: []core.Node{
				base.AccordionHeader{Children: []core.Node{base.AccordionTrigger{Label: item.Text, Style: face,
					StyleFunc: func() core.Style { return core.Style{Color: colors.Text(), Focus: FocusRing(cx, -4)} },
					Children:  []core.Node{line(item.Text, TypeLabel.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), 0), turn}}}},
				base.AccordionPanel{Label: item.Text, Reveal: reveal,
					Children: []core.Node{core.View{Style: core.Style{Padding: core.Edges{Left: 12, Right: 12, Bottom: 12}, Gap: 8}, Children: []core.Node{item.Content}}}},
			}}
		if i > 0 {
			section.Style = core.Style{Borders: core.Edges{Top: 1}}
			section.StyleFunc = func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Border()}} }
		}
		sections = append(sections, section)
	}
	return base.Accordion{Value: open, Multiple: func() bool { return true }, OnChange: change, TestID: a.TestID,
		StyleFunc: func() core.Style {
			return core.Style{CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, BorderColor: colors.Border()}.Merge(a.Style)
		}, Children: sections}
}
