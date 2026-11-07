package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// SidebarItem is a destination of a Sidebar.
type SidebarItem struct {
	// ID is what Selected and OnSelect name the item by.
	ID     string
	Text   string
	Icon   func() *core.SVG
	TestID string
}

// SidebarSection is a group of a Sidebar's items under an optional
// title.
type SidebarSection struct {
	Title string
	Items []SidebarItem
	// Actions end the title's row; Note follows the items, for a section
	// that has none yet.
	Actions core.Node
	Note    core.Node
	TestID  string
}

// Sidebar is grouped navigation: the sections' items, the Selected one on
// a faint fill of the text's color, icons in the accent.
// It scrolls and leaves its background to the shell.
//
// It is one stop of Tab, as a macOS source list is: while it has the
// keyboard focus, the selected item is ringed, Up and Down select the item
// before and after, Home and End the first and the last, scrolled into
// view.
type Sidebar struct {
	Sections []SidebarSection
	Selected func() string
	OnSelect func(id string)
	TestID   string
}

// Render implements core.Node.
func (s Sidebar) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	const height core.Points = 32
	var sections []base.SidebarSection
	for _, sec := range s.Sections {
		section := base.SidebarSection{TestID: sec.TestID, Style: core.Style{Gap: 4}}
		if sec.Title != "" {
			title := Text(sec.Title, 11, Bold, colors.Muted)
			title.Role, title.MaxLines = core.RoleHeader, 1
			title.Style.Padding = core.Edges{Top: 4, Right: 8, Bottom: 2, Left: 8}
			section.Header = title
			if sec.Actions != nil {
				title.Style.FlexGrow, title.Style.FlexShrink = 1, 1
				section.Header = core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Edges{Right: 4}}, Children: []core.Node{title, sec.Actions}}
			}
		}
		for _, it := range sec.Items {
			label := Text(it.Text, 13, Regular, colors.Text)
			label.MaxLines, label.Style.FlexGrow, label.Style.FlexShrink = 1, 1, 1
			section.Items = append(section.Items, base.SidebarItem{ID: it.ID, Label: it.Text, TestID: it.TestID,
				Children: []core.Node{Icon(it.Icon, 16, colors.Accent), label}})
		}
		if sec.Note != nil {
			section.Footer = core.View{Style: core.Style{Padding: core.Symmetric(4, 8)}, Children: []core.Node{sec.Note}}
		}
		sections = append(sections, section)
	}
	return base.Sidebar{Sections: sections, Selected: s.Selected, OnSelect: s.OnSelect, TestID: s.TestID, Style: FillRest,
		ContentStyle: core.Style{Padding: core.Edges{Right: 12, Bottom: 12, Left: 12}, Gap: 12},
		ItemStyle: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(height),
			Padding: core.Symmetric(0, 8), CornerRadius: 8, Cursor: core.CursorDefault},
		ItemStyleFunc: func() core.Style {
			return core.Style{Selected: &core.Style{Background: Alpha(colors.Text(), 0.1)}, Focus: FocusRing(cx, -2)}
		}}
}
