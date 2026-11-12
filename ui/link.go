package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// Link is text in the accent that goes somewhere when pressed, underlined
// under the pointer: base.Link's opening, by the system or by OnOpen.
type Link struct {
	Text     string
	Href     string
	OpenWith base.LinkOpenWith
	Disabled func() bool
	// OnOpen hears it open its Href, before OnPress.
	OnOpen  func(href string)
	OnPress func()
	// Label is what assistive technology reads, Text when empty.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (l Link) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	label := l.Text
	if l.Label != "" {
		label = l.Label
	}
	return base.Link{Href: l.Href, OpenWith: l.OpenWith, Disabled: l.Disabled, OnOpen: l.OnOpen, Label: label, TestID: l.TestID,
		OnPress: func(core.PressEvent) {
			if l.OnPress != nil {
				l.OnPress()
			}
		},
		Style: core.Style{Group: "link", AlignSelf: core.AlignStart, CornerRadius: RadiusSmall, Disabled: Dimmed()}.Merge(l.Style),
		StyleFunc: func() core.Style {
			return core.Style{FocusVisible: FocusRing(cx, 2)}
		},
		Children: []core.Node{core.Text{Content: l.Text, Style: TypeBody,
			StyleFunc: func() core.Style {
				return core.Style{Color: colors.Accent(), GroupStyle: &core.GroupStyle{Name: "link",
					Hover: &core.Style{TextDecoration: core.TextDecorationUnderline}}}
			}}}}
}
