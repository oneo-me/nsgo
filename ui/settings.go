package ui

import (
	"strings"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Settings is Onevra's settings surface: categories in a sidebar beside
// the selected one's groups of rows, and a search through every row. It
// fills its parent, which gives it a finite height and its background,
// and can sit in a dialog or a window's pane.

// SettingsCategory is a destination of Settings' sidebar. IDs are
// nonempty, unique and stable across languages; consecutive categories
// of a Section are listed under its title.
type SettingsCategory struct {
	ID, Title, Section, Description string
	Icon                            func() *core.SVG
	Groups                          []SettingsGroup
	// Placeholder, set on a category with no page, stands for it in the
	// sidebar: a note that it is empty, not a destination.
	Placeholder core.Node
}

// SettingsGroup is related rows under an optional Title, with Actions at
// the end of its heading. Actions are not searched.
type SettingsGroup struct {
	ID, Title string
	Items     []SettingsItem
	Actions   []core.Node
}

// SettingsItem is a row: Title and Description with an optional Leading
// before them and the Control after, or below them when Stacked (wide
// controls) or Settings is Narrow. Content, when set, replaces the row
// with the app's own; Title, Description and Keywords are what search
// matches either way.
type SettingsItem struct {
	ID, Title, Description string
	Keywords               []string
	Leading, Control       core.Node
	Stacked                bool
	Content                core.Node
}

// SettingsResult is an item by its indices in the categories searched.
type SettingsResult struct{ Category, Group, Item int }

// SearchSettings returns the items, in order, whose category, group,
// title, description and keywords hold every whitespace-separated term
// of query, in any case: every item for an empty query.
func SearchSettings(categories []SettingsCategory, query string) []SettingsResult {
	terms := strings.Fields(strings.ToLower(query))
	var results []SettingsResult
	for ci, category := range categories {
		for gi, group := range category.Groups {
			for ii, item := range group.Items {
				haystack := strings.ToLower(strings.Join([]string{category.Title, category.Section, category.Description,
					group.Title, item.Title, item.Description, strings.Join(item.Keywords, " ")}, " "))
				match := true
				for _, term := range terms {
					if !strings.Contains(haystack, term) {
						match = false
						break
					}
				}
				if match {
					results = append(results, SettingsResult{ci, gi, ii})
				}
			}
		}
	}
	return results
}

// Settings shows Categories. Category and Query are the app's state,
// its own where nil: an empty or unknown category shows the first, and
// choosing one clears the query. Narrow, the host's say from the width it
// gives, swaps the sidebar for a row of the categories above the page
// and stacks each row's control below its text.
//
// The texts are the app's: Title heads the sidebar and narrow pages,
// SearchLabel and SearchPlaceholder name the search field, ResultsTitle
// heads the results and EmptyText says there are none. OnClose, when
// set, adds a close button named CloseLabel. HideSearch drops the search
// field; HideSidebar the navigation, showing every category on one page;
// HideHeading the page's heading, for a host whose header says it.
type Settings struct {
	Categories              []SettingsCategory
	Category, Query         *core.Signal[string]
	Narrow                  func() bool
	Title, SearchLabel      string
	SearchPlaceholder       string
	ResultsTitle, EmptyText string
	OnClose                 func()
	CloseLabel              string
	HideSearch, HideSidebar bool
	HideHeading             bool
	// HeaderActions end the shown page's title; SectionActions the heading
	// of a section of the sidebar, given its title.
	HeaderActions  func(cx *core.Scope) core.Node
	SectionActions func(cx *core.Scope, section string) core.Node
	TestID         string
}

// Render implements core.Node.
func (s Settings) Render(cx *core.Scope) core.Node {
	if s.Category == nil {
		s.Category = core.NewSignal(cx, "")
	}
	if s.Query == nil {
		s.Query = core.NewSignal(cx, "")
	}
	narrow := s.Narrow
	if narrow == nil {
		narrow = func() bool { return false }
	}
	return core.Dynamic(func(cx *core.Scope) core.Node {
		n := narrow()
		return core.Func(func(cx *core.Scope) core.Node { return s.layout(cx, n) })
	})
}

// selected is the index of the category shown, -1 when there is none.
func (s Settings) selected() int {
	id := s.Category.Get()
	for i, c := range s.Categories {
		if c.ID == id {
			return i
		}
	}
	if len(s.Categories) == 0 {
		return -1
	}
	return 0
}

// query is what to search for, "" without the search field.
func (s Settings) query() string {
	if s.HideSearch {
		return ""
	}
	return strings.TrimSpace(s.Query.Get())
}

func (s Settings) choose(id string) {
	s.Category.Set(id)
	s.Query.Set("")
}

func (s Settings) layout(cx *core.Scope, narrow bool) core.Node {
	colors := ColorsOf(cx)
	var page []core.Node
	if !s.HideHeading {
		page = append(page, s.heading(cx, func() string {
			switch {
			case s.query() != "":
				return s.ResultsTitle
			case narrow || s.HideSidebar:
				return s.Title
			}
			if i := s.selected(); i >= 0 {
				return s.Categories[i].Title
			}
			return s.Title
		}, true))
	}
	if narrow {
		var bar []core.Node
		if !s.HideSidebar {
			bar = append(bar, s.chips(cx))
		}
		if !s.HideSearch {
			bar = append(bar, s.search(cx))
		}
		if bar != nil {
			page = append(page, core.View{Style: core.Style{Gap: 8, Padding: core.Edges{Right: 12, Bottom: 12, Left: 12}}, Children: bar})
		}
	} else if s.HideSidebar && !s.HideSearch {
		page = append(page, core.View{Style: core.Style{Padding: core.Edges{Right: 12, Bottom: 12, Left: 12}}, Children: []core.Node{s.search(cx)}})
	}
	page = append(page, core.Scroll{Style: FillRest, Children: []core.Node{core.Dynamic(func(cx *core.Scope) core.Node {
		results, selected, query := SearchSettings(s.Categories, s.query()), s.selected(), s.query()
		return core.Func(func(cx *core.Scope) core.Node { return s.rows(cx, narrow, results, selected, query != "") })
	})}})

	children := []core.Node{core.View{Style: FillRest, Children: page}}
	if !narrow && !s.HideSidebar {
		side := []core.Node{s.heading(cx, func() string { return s.Title }, false)}
		if !s.HideSearch {
			side = append(side, core.View{Style: core.Style{Padding: core.Edges{Right: 12, Bottom: 16, Left: 12}}, Children: []core.Node{s.search(cx)}})
		}
		side = append(side, core.View{Style: FillRest, Children: []core.Node{s.sidebar(cx)}})
		children = append([]core.Node{
			core.View{TestID: "settings-sidebar", Style: core.Style{Width: core.Pt(216), FlexShrink: 0},
				StyleFunc: func() core.Style { return core.Style{Background: Alpha(colors.Text(), 0.035)} }, Children: side},
			core.View{Style: core.Style{Width: core.Pt(1), FlexShrink: 0}, StyleFunc: Filled(colors.Border)},
		}, children...)
	}
	return core.View{TestID: s.TestID, Style: FillRest.Merge(core.Style{Direction: core.Row}), Children: children}
}

// heading is a pane's title, with the close button where close says.
func (s Settings) heading(cx *core.Scope, title func() string, close bool) core.Node {
	colors := ColorsOf(cx)
	text := Textf(title, 15, Semibold, colors.Text)
	text.Role, text.MaxLines, text.Style.FlexGrow, text.Style.FlexShrink = core.RoleHeader, 1, 1, 1
	children := []core.Node{text}
	if close && s.HeaderActions != nil {
		children = append(children, s.HeaderActions(cx))
	}
	if close && s.OnClose != nil {
		children = append(children, IconButton{Icon: lucide.X, Label: s.CloseLabel, OnPress: s.OnClose, TestID: "settings-close"})
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(56), FlexShrink: 0,
		Padding: core.Symmetric(12, 16)}, Children: children}
}

// search is the search field, with a magnifier at its start.
func (s Settings) search(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return FinOf(cx).Field().Over(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Height: core.Pt(32),
			Padding: core.Symmetric(0, 10), CornerRadius: 8, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1, BorderColor: colors.FieldBorder(), Background: colors.Field()})
	}, Children: []core.Node{
		Icon(lucide.Search, 14, colors.Faint),
		base.Input{Value: s.Query.Get, OnChange: s.Query.Set, Placeholder: s.SearchPlaceholder, Label: s.SearchLabel, TestID: "settings-search",
			Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: 13},
			StyleFunc: func() core.Style {
				return core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Accent()}
			}},
	}}
}

// sidebar lists the categories, consecutive ones of a section under its
// title.
func (s Settings) sidebar(cx *core.Scope) core.Node {
	var sections []SidebarSection
	for i, c := range s.Categories {
		if i == 0 || c.Section != s.Categories[i-1].Section {
			section := SidebarSection{Title: c.Section}
			if s.SectionActions != nil {
				section.Actions = s.SectionActions(cx, c.Section)
			}
			sections = append(sections, section)
		}
		last := &sections[len(sections)-1]
		if c.Placeholder != nil {
			last.Note = c.Placeholder
			continue
		}
		last.Items = append(last.Items, SidebarItem{ID: c.ID, Text: c.Title, Icon: c.Icon, TestID: "settings-nav-" + c.ID})
	}
	return Sidebar{Sections: sections, OnSelect: s.choose, Selected: func() string {
		if i := s.selected(); i >= 0 {
			return s.Categories[i].ID
		}
		return ""
	}}
}

// chips are the narrow layout's categories, a row that scrolls across.
func (s Settings) chips(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var chips []core.Node
	for i, c := range s.Categories {
		on := func() bool { return s.selected() == i }
		chips = append(chips, Press{OnPress: func() { s.choose(c.ID) }, Label: c.Title, Selected: on, TestID: "settings-nav-" + c.ID,
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Height: core.Pt(32), Padding: core.Symmetric(0, 12),
				CornerRadius: 16, BorderWidth: 1},
			StyleFunc: func() core.Style {
				if on() {
					return core.Style{Background: Alpha(colors.Text(), 0.1), BorderColor: transparent}
				}
				return core.Style{BorderColor: colors.Border()}
			},
			Children: []core.Node{Icon(c.Icon, 14, colors.Accent), Text(c.Title, 13, Medium, colors.Text)}})
	}
	return core.Scroll{Horizontal: true, HideIndicator: true, Style: core.Style{FlexGrow: 0},
		Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, Gap: 6}, Children: chips}}}
}

// rows are the page: the selected category's rows, every category's
// without navigation, or the results while searching.
func (s Settings) rows(cx *core.Scope, narrow bool, results []SettingsResult, selected int, searching bool) core.Node {
	colors := ColorsOf(cx)
	all := searching || s.HideSidebar
	var nodes []core.Node
	lastCategory, lastGroup := -1, -1
	for _, r := range results {
		if !all && r.Category != selected {
			continue
		}
		category := s.Categories[r.Category]
		group := category.Groups[r.Group]
		item := group.Items[r.Item]
		if r.Category != lastCategory {
			if searching || all && len(s.Categories) > 1 {
				title := Text(category.Title, 15, Semibold, colors.Text)
				title.Role = core.RoleHeader
				nodes = append(nodes, title)
			}
			if category.Description != "" {
				nodes = append(nodes, Text(category.Description, 13, Regular, colors.Muted))
			}
		}
		if (r.Category != lastCategory || r.Group != lastGroup) && (group.Title != "" || group.Actions != nil) {
			title := Text(group.Title, 13, Semibold, colors.Muted)
			title.Style.FlexGrow, title.Style.FlexShrink = 1, 1
			nodes = append(nodes, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12,
				Padding: core.Edges{Bottom: 8}, Borders: core.Edges{Bottom: 1}},
				StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Bottom: colors.Line()}} },
				Children:  append([]core.Node{title}, group.Actions...)})
		}
		lastCategory, lastGroup = r.Category, r.Group
		testID := "settings-" + category.ID + "-" + item.ID
		if item.Content != nil {
			nodes = append(nodes, core.View{TestID: testID, Children: []core.Node{item.Content}})
			continue
		}
		nodes = append(nodes, s.row(cx, item, narrow || item.Stacked, testID))
	}
	if len(nodes) == 0 {
		nodes = append(nodes, Text(s.EmptyText, 14, Regular, colors.Muted))
	}
	return core.View{Style: core.Style{Padding: core.All(20), Gap: 20}, Children: nodes}
}

// row is an item's title and description, its Leading before them and
// its Control after them, or below them when stacked.
func (s Settings) row(cx *core.Scope, item SettingsItem, stacked bool, testID string) core.Node {
	colors := ColorsOf(cx)
	var texts []core.Node
	if item.Title != "" {
		texts = append(texts, Text(item.Title, 14, Medium, colors.Text))
	}
	if item.Description != "" {
		d := Text(item.Description, 13, Regular, colors.Muted)
		d.Style.LineHeight = 18
		texts = append(texts, d)
	}
	var text core.Node = core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 4}, Children: texts}
	if item.Leading != nil {
		text = core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)},
			Children: []core.Node{core.View{Style: core.Style{FlexShrink: 0}, Children: []core.Node{item.Leading}}, text}}
	}
	children := []core.Node{text}
	if item.Control != nil {
		children = append(children, core.View{Style: core.Style{FlexShrink: 0}, Children: []core.Node{item.Control}})
	}
	style := core.Style{Gap: 12}
	if !stacked {
		style.Direction, style.AlignItems = core.Row, core.AlignCenter
	}
	return core.View{TestID: testID, Style: style, Children: children}
}
