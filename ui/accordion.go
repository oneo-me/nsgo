package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// AccordionItem is one section of an Accordion.
type AccordionItem struct {
	Text     string
	Content  core.Node
	Disabled func() bool
	TestID   string
}

// Accordion is a card of Collapsibles divided by hairlines. The app keeps
// which are open: one at a time, or any.
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
	var sections []core.Node
	for i, item := range a.Items {
		c := Collapsible{Text: item.Text, Disabled: item.Disabled,
			Open: func() bool { return a.Open != nil && a.Open(i) },
			OnOpenChange: func(open bool) {
				if a.OnOpenChange != nil {
					a.OnOpenChange(i, open)
				}
			}}
		section := base.Collapsible{Open: c.Open, TestID: item.TestID,
			Content:  core.View{Style: core.Style{Padding: core.Edges{Left: 14, Right: 14, Bottom: 14}}, Children: []core.Node{item.Content}},
			Children: []core.Node{collapsibleTrigger(cx, c, core.Style{Padding: core.Symmetric(8, 14), CornerRadius: RadiusCard - 1})}}
		if i > 0 {
			section.Style = core.Style{Borders: core.Edges{Top: 1}}
			section.StyleFunc = func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}} }
		}
		sections = append(sections, section)
	}
	return core.View{TestID: a.TestID, StyleFunc: func() core.Style { return CardStyle(cx).Merge(a.Style) }, Children: sections}
}
