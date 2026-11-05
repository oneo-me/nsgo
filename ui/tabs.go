package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TabsVariant is a tab list's look.
type TabsVariant uint8

const (
	// TabsUnderline underlines the selected tab on a line under the list:
	// the default, for sections of a page.
	TabsUnderline TabsVariant = iota
	// TabsSegmented raises the selected tab on a sunken track, for views
	// of the same content.
	TabsSegmented
)

// TabItem is one tab of Tabs.
type TabItem struct {
	Text     string
	Icon     func() *core.SVG
	Disabled func() bool
	TestID   string
}

// Tabs is a tab list: the app keeps which tab is Selected, by index, and
// shows its content beside the list.
type Tabs struct {
	Variant  TabsVariant
	Items    []TabItem
	Selected func() int
	// OnChange receives the index of a pressed tab other than the
	// selected one.
	OnChange func(index int)
	// Label names the list for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (t Tabs) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	list := core.Style{Direction: core.Row, AlignItems: core.AlignStretch}
	tab := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
		Cursor: core.CursorPointer, Transition: quick, Disabled: &core.Style{Cursor: core.CursorDefault}}
	segmented := t.Variant == TabsSegmented
	if segmented {
		list.CornerRadius, list.Padding, list.Gap, list.BorderWidth = RadiusControl, core.All(2), 2, 1
		tab.Height, tab.Padding, tab.CornerRadius, tab.BorderWidth = core.Pt(ButtonHeight-4), core.Symmetric(0, 12), RadiusControl-2, 1
	} else {
		list.Gap, list.Borders = 16, core.Edges{Bottom: 1}
		tab.Height, tab.Padding = core.Pt(ButtonHeight+4), core.Symmetric(0, 4)
		tab.Borders, tab.Margin = core.Edges{Bottom: 2}, core.Edges{Bottom: -1}
	}
	tabLook := func() core.Style {
		s := core.Style{Color: colors.Muted(), Hover: &core.Style{Color: colors.Text()}, Focus: FocusRing(cx, -2),
			Disabled: &core.Style{Color: colors.Faint()}}
		if segmented {
			s.BorderColor = transparent
			s.Selected = &core.Style{Background: colors.Raised(), BorderColor: colors.Border(), Shadows: FinOf(cx).Raised().Shadows,
				Color: colors.Text()}
		} else {
			s.BorderColors = core.EdgeColors{Bottom: transparent}
			s.Selected = &core.Style{BorderColors: core.EdgeColors{Bottom: colors.Accent()}, Color: colors.Text()}
		}
		return s
	}
	selected := func() int {
		if t.Selected == nil {
			return 0
		}
		return t.Selected()
	}
	var tabs []core.Node
	for i, item := range t.Items {
		var children []core.Node
		if item.Icon != nil {
			children = append(children, Icon(item.Icon, iconSize, Inherited))
		}
		if item.Text != "" {
			children = append(children, line(item.Text, TypeLabel, 1))
		}
		tabs = append(tabs, base.Tab{Selected: func() bool { return selected() == i }, Disabled: item.Disabled,
			OnPress: func(core.PressEvent) {
				if t.OnChange != nil && core.Untracked(cx, selected) != i {
					t.OnChange(i)
				}
			},
			Style: tab, StyleFunc: tabLook, Children: children, Label: item.Text, TestID: item.TestID})
	}
	return base.Tabs{Style: list, StyleFunc: func() core.Style {
		if segmented {
			return core.Style{Background: colors.Inset(), BorderColor: colors.Line()}.Merge(t.Style)
		}
		return core.Style{BorderColors: core.EdgeColors{Bottom: colors.Line()}}.Merge(t.Style)
	}, Children: tabs, Label: t.Label, TestID: t.TestID}
}
