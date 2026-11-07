package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Submenu is an item that opens more: beside it on desktops, in place of
// the menu's items on touch screens, under a row of Back, the app's
// label for returning.
type Submenu struct {
	Text     string
	Icon     func() *core.SVG
	Back     string
	Children []core.Node
	Disabled func() bool
	TestID   string
}

// Render implements core.Node.
func (s Submenu) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	style, look := menuRow(cx)
	chevron := menuIconSize(cx) - 2
	trigger := append(menuLeading(cx, s.Icon, nil),
		core.Text{Content: s.Text, MaxLines: 1, Style: TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), StyleFunc: Colored(colors.Text)},
		Icon(lucide.ChevronRight, chevron, colors.Muted))
	return base.Submenu{Disabled: s.Disabled, Style: style, StyleFunc: look, Trigger: core.Group(trigger),
		Back: core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8},
			Children: []core.Node{Icon(lucide.ChevronLeft, chevron, colors.Muted), core.Text{Content: s.Back, MaxLines: 1, Style: TypeLabel,
				StyleFunc: Colored(colors.Muted)}}},
		BackStyle: style, Children: s.Children, Label: s.Text, TestID: s.TestID}
}
