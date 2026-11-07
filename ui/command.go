package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Command is a command palette: a bare search field over the app's
// commands under their sections, the first match lit and Return running
// it, with each command's shortcut at its row's end. Put it in a Dialog,
// or anywhere with a height of its own.
type Command struct {
	Items       func() []base.CommandItem
	Placeholder string
	// Empty shows while none matches.
	Empty string
	// Ref's Focus, Highlight and MatchedCount are the palette's.
	Ref       *base.CommandRef
	AutoFocus bool
	OnSelect  func(id string)
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
	empty := c.Empty
	if empty == "" {
		empty = locales.ListNothingFound(cx)
	}
	group, groupLook := listGroupStyle(cx)
	return base.Command{Items: c.Items, Placeholder: c.Placeholder, AutoFocus: c.AutoFocus, Ref: c.Ref, OnSelect: c.OnSelect, OnCancel: c.OnCancel,
		Label: c.Label, TestID: c.TestID, Style: core.Style{MinHeight: core.Pt(0)}.Merge(c.Style),
		SearchBarStyle:     core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(10, 12), Borders: core.Edges{Bottom: 1}},
		SearchBarStyleFunc: func() core.Style { return core.Style{BorderColor: colors.Line()} },
		SearchPrefix:       Icon(lucide.Search, iconSize, colors.Muted),
		SearchField: func(in core.Input) core.Node {
			in.Style = in.Style.Merge(core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 15, Padding: core.All(0), BorderWidth: 0, Background: core.Transparent})
			in.StyleFunc = func() core.Style {
				return core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), CaretColor: colors.Accent(), SelectionColor: cx.Palette().Selection}
			}
			return in
		},
		ListStyle: core.Style{FlexGrow: 1, Padding: core.All(8)},
		ItemStyle: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(10, 10), CornerRadius: RadiusCard, Disabled: Dimmed()},
		ItemStyleFunc: func() core.Style {
			return core.Style{Focus: &core.Style{Background: Alpha(colors.Text(), 0.1)}, Checked: &core.Style{Background: Alpha(colors.Text(), 0.04)}}
		},
		GroupStyle: group, GroupStyleFunc: groupLook, SeparatorStyle: core.Style{Height: core.Pt(8)},
		Empty: emptyList(cx, empty),
		RenderItem: func(e base.CommandEntry) core.Node {
			children := []core.Node{core.Text{Content: e.Label, MaxLines: 1, Style: TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), StyleFunc: Colored(colors.Text)}}
			if e.ShortcutLabel != "" {
				children = append(children, core.Text{Content: e.ShortcutLabel, MaxLines: 1, Style: TypeCaption, StyleFunc: Colored(colors.Muted)})
			}
			return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, FlexGrow: 1}, Children: children}
		}}
}
