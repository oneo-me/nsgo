package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Command is a command palette as Raycast's: a bare search
// field over the app's commands, ranked as they match and under their
// sections, the first one lit and Return running it. A row shows an icon,
// a label with a description, and the item's Trailing and shortcut at its
// end. Put it in a CommandDialog, or anywhere with a height of its own.
type Command struct {
	Items func() []base.CommandItem
	// Placeholder and SearchLabel are the field's, Empty what shows while
	// none matches, LoadingText while Loading: all have words of their own.
	Placeholder, SearchLabel, Empty, LoadingText string
	// Query makes the query the app's, nil for the field's own, from
	// DefaultQuery.
	Query         func() *string
	DefaultQuery  string
	OnQueryChange func(query string)
	// Filter replaces base.FilterCommands, and NoFilter leaves the items to
	// the app, as a search it answers itself.
	Filter   func(items []base.CommandItem, query string) []base.CommandItem
	NoFilter bool
	// ExpandOnSearch shows the field alone until the query holds more than
	// spaces, then the Height of Style.
	ExpandOnSearch bool
	// Loading takes the rows' place with LoadingText, Error with what it
	// says, while it says something.
	Loading func() bool
	Error   func() string
	// InputSource and KeepInputSource are base.Command's.
	InputSource     string
	KeepInputSource bool
	// Ref's Focus, Highlight and MatchedCount are the palette's.
	Ref               *base.CommandRef
	AutoFocus         bool
	OnSelect          func(id string)
	OnHighlightChange func(highlighted *string)
	// OnCancel hears Escape with no query.
	OnCancel func()
	// Label names the list for assistive technology.
	Label  string
	TestID string
	// Style merges over the palette's.
	Style core.Style
}

// Render implements core.Node.
func (c Command) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	placeholder, searchLabel, empty, loading, label := c.Placeholder, c.SearchLabel, c.Empty, c.LoadingText, c.Label
	if placeholder == "" {
		placeholder = locales.CommandSearchPlaceholder(cx)
	}
	if searchLabel == "" {
		searchLabel = locales.CommandSearch(cx)
	}
	if empty == "" {
		empty = locales.CommandNoMatches(cx)
	}
	if loading == "" {
		loading = locales.CommandLoading(cx)
	}
	if label == "" {
		label = locales.CommandList(cx)
	}
	style, expanded := core.Style{MinHeight: core.Pt(0)}.Merge(c.Style), core.Style{}
	if c.ExpandOnSearch {
		style.Height, expanded.Height = core.Length{}, style.Height
	}
	group, groupLook := listGroupStyle(cx)
	group.Padding = core.Symmetric(8, 10)
	return base.Command{Items: c.Items, Query: c.Query, DefaultQuery: c.DefaultQuery, OnQueryChange: c.OnQueryChange, Filter: c.Filter, NoFilter: c.NoFilter,
		Placeholder: placeholder, SearchLabel: searchLabel, AutoFocus: c.AutoFocus, Ref: c.Ref, OnSelect: c.OnSelect, OnHighlightChange: c.OnHighlightChange,
		OnCancel: c.OnCancel, ExpandOnSearch: c.ExpandOnSearch, ExpandedStyle: expanded, Loading: c.Loading, Error: c.Error,
		InputSource: c.InputSource, KeepInputSource: c.KeepInputSource, Label: label, TestID: c.TestID, Style: style,
		SearchBarStyle: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.All(18), FlexShrink: 0},
		SearchPrefix:   Icon(lucide.Search, 20, colors.Muted),
		SearchField: func(in core.Input) core.Node {
			in.Style = in.Style.Merge(core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 15, Padding: core.All(0), BorderWidth: 0, Background: core.Transparent})
			in.StyleFunc = func() core.Style {
				return core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), CaretColor: colors.Accent(), SelectionColor: cx.Palette().Selection}
			}
			return in
		},
		ListStyle: core.Style{FlexGrow: 1, Padding: core.Edges{Right: 8, Bottom: 12, Left: 8}},
		ItemStyle: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.All(10), CornerRadius: RadiusCard, Disabled: Dimmed()},
		ItemStyleFunc: func() core.Style {
			return core.Style{Focus: &core.Style{Background: Alpha(colors.Text(), 0.1)}, Checked: &core.Style{Background: Alpha(colors.Text(), 0.04)}}
		},
		GroupStyle: group, GroupStyleFunc: groupLook, NoSeparator: true,
		Empty: commandMessage(cx, empty), Pending: commandMessage(cx, loading),
		RenderError: func(message string) core.Node { return commandMessage(cx, message) },
		RenderItem:  func(e base.CommandEntry) core.Node { return commandRow(cx, e) }}
}

// commandMessage is what shows in a palette's rows' place.
func commandMessage(cx *core.Scope, text string) core.Node {
	return core.View{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Padding: core.All(16), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		Children: []core.Node{core.Text{Content: text, LiveRegion: core.LiveRegionPolite, Style: TypeBody.Merge(core.Style{TextAlign: core.TextAlignCenter}),
			StyleFunc: Colored(ColorsOf(cx).Muted)}}}
}

// commandRow draws what a palette's row shows.
func commandRow(cx *core.Scope, e base.CommandEntry) core.Node {
	colors := ColorsOf(cx)
	var children []core.Node
	switch {
	case e.Leading != nil:
		children = append(children, core.View{Style: core.Style{Width: core.Pt(20), Height: core.Pt(20), FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{e.Leading}})
	case e.Icon != nil:
		children = append(children, core.Icon{SVG: e.Icon, Style: core.Style{Width: core.Pt(20), Height: core.Pt(20), FlexShrink: 0}, StyleFunc: Colored(colors.Text)})
	}
	text := []core.Node{core.Text{Content: e.Label, MaxLines: 1, Style: TypeBody, StyleFunc: Colored(colors.Text)}}
	if e.Description != "" {
		text = append(text, core.Text{Content: e.Description, MaxLines: 1, Style: TypeCaption, StyleFunc: Colored(colors.Muted)})
	}
	children = append(children, core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, Children: text})
	if e.Trailing != nil {
		children = append(children, e.Trailing)
	}
	if e.ShortcutLabel != "" {
		children = append(children, core.Text{Content: e.ShortcutLabel, MaxLines: 1, Style: TypeCaption.Merge(core.Style{FlexShrink: 0}), StyleFunc: Colored(colors.Muted)})
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, Children: children}
}
