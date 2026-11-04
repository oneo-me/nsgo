package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// ShellTab is a top-level section of a TabShell.
type ShellTab struct {
	Text string
	Icon IconName
	// Content is the section, such as a base.Nav's stack of pages. It is
	// built when the tab is first selected and kept while others show.
	Content func(cx *core.Scope) core.Node
	TestID  string
}

// TabShell is an app's top-level sections over a drawn tab bar, which
// keeps clear of the bottom safe area. The Selected tab's content fills
// the rest; Selected is the app's, set from OnChange. The bar hides
// while the software keyboard shows, so content padded by
// core.EnvKeyboard meets the keyboard without an offset.
type TabShell struct {
	Tabs     []ShellTab
	Selected func() int
	OnChange func(index int)
	// Label names the bar for assistive technology.
	Label  string
	TestID string
}

// Render implements core.Node.
func (s TabShell) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	selected := func() int {
		if s.Selected == nil {
			return 0
		}
		return s.Selected()
	}
	change := func(i int) {
		if s.OnChange != nil {
			s.OnChange(i)
		}
	}
	var sections []core.Node
	for i, tab := range s.Tabs {
		// A section is built when first selected, then hidden while others
		// show, keeping its state.
		seen := false
		sections = append(sections, core.View{TestID: tab.TestID, Children: []core.Node{core.Show{
			When: func() bool { seen = seen || selected() == i; return seen },
			Then: func(cx *core.Scope) core.Node {
				if tab.Content == nil {
					return nil
				}
				return tab.Content(cx)
			}}},
			StyleFunc: func() core.Style {
				s := fillRest
				if selected() != i {
					s.Display = core.DisplayNone
				}
				return s
			}})
	}
	env := cx.Env()
	return core.View{TestID: s.TestID, Style: fillRest, StyleFunc: func() core.Style {
		return core.Style{Background: colors.Background(), Color: colors.Text()}
	}, Children: []core.Node{
		core.View{Style: fillRest, Children: sections},
		core.View{Children: []core.Node{s.bar(cx, selected, change)}, StyleFunc: func() core.Style {
			if env.KeyboardHeight() > 0 {
				return core.Style{Display: core.DisplayNone}
			}
			return core.Style{}
		}},
	}}
}

// bar is the tab bar: the tabs' icons over their text, the selected
// one in the accent.
func (s TabShell) bar(cx *core.Scope, selected func() int, change func(int)) core.Node {
	colors := ColorsOf(cx)
	var tabs []core.Node
	for i, tab := range s.Tabs {
		var content []core.Node
		if tab.Icon != "" {
			content = append(content, Icon(tab.Icon, 22, inherited))
		}
		content = append(content, line(tab.Text, typeCaption.Merge(core.Style{FontWeight: Medium}), 1))
		tabs = append(tabs, base.Button{Role: core.RoleTab, Label: tab.Text, Selected: func() bool { return selected() == i },
			OnPress: func(core.PressEvent) { change(i) },
			Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), AlignItems: core.AlignCenter,
				JustifyContent: core.JustifyCenter, Gap: 2, MinHeight: core.Pt(rowHeight(cx) + 4), CornerRadius: RadiusControl,
				Cursor: core.CursorPointer, Transition: quick},
			StyleFunc: func() core.Style {
				return core.Style{Color: colors.Muted(), Hover: &core.Style{Color: colors.Text()}, Active: &core.Style{Background: colors.Pressed()},
					Selected: &core.Style{Color: colors.Accent()}, Focus: FocusRing(cx, -2)}
			},
			Children: content})
	}
	return core.View{Style: core.Style{Direction: core.Row, Padding: core.Symmetric(4, 8), Borders: core.Edges{Top: 1},
		PaddingEnv: core.EnvEdges{Right: core.EnvSafeArea, Bottom: core.EnvSafeArea, Left: core.EnvSafeArea}},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Surface(), BorderColors: core.EdgeColors{Top: colors.Line()}}
		}, Children: []core.Node{
			core.View{Role: core.RoleTabList, Label: s.Label, TestID: "tab-bar",
				Style: core.Style{Direction: core.Row, FlexGrow: 1, Gap: 4}, Children: tabs},
		}}
}
