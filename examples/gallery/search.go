package main

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// searchKey opens the demos' search, in either layout.
const searchKey = "mod+k"

// commandInputSource is the macOS input source the palettes search in, as
// demos are found by their English names in every language.
const commandInputSource = "com.apple.keylayout.ABC"

// searchItems are the demos as commands, under their groups: an English
// name, the group and what a demo shows find it in any language.
func searchItems(tr locales.Messages) []base.CommandItem {
	var items []base.CommandItem
	for _, t := range tabs {
		for _, grp := range t.groups {
			title := grp.title(tr)
			for _, d := range grp.demos {
				items = append(items, base.CommandItem{ID: d.name, Label: d.title(tr), Description: d.detail(tr), Group: title, Icon: d.icon(),
					Keywords: []string{d.name, title, d.detail(tr)}})
			}
		}
	}
	return items
}

// searchDialog finds a demo and goes to it.
func (g *gallery) searchDialog(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	items := searchItems(tr)
	cx.OnKey(searchKey, func() { g.searchOpen.Set(true) })
	return ui.CommandDialog{TestID: "search-dialog", Open: g.searchOpen.Get, OnOpenChange: func(open bool, _ base.Reason) { g.searchOpen.Set(open) },
		Command: ui.Command{Items: func() []base.CommandItem { return items }, Placeholder: tr.AppSearchDemos(), SearchLabel: tr.AppSearchDemos(),
			Label: tr.AppSearchDemos(), Empty: tr.PickerNothingFound(), InputSource: commandInputSource,
			OnSelect: func(name string) {
				g.searchOpen.Set(false)
				g.open(name)
			}}}
}

// searchButton is the sidebar's way to searchDialog, with the key beside it.
func (g *gallery) searchButton(cx *core.Scope, tr locales.Messages) core.Node {
	colors := ui.ColorsOf(cx)
	label := tr.AppSearchDemos()
	return core.View{TestID: "search", Role: core.RoleButton, Label: label, OnPress: func(core.PressEvent) { g.searchOpen.Set(true) },
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(32), Padding: core.Symmetric(0, 8), CornerRadius: 8,
			Margin: core.Edges{Right: 12, Bottom: 8, Left: 12}},
		StyleFunc: func() core.Style { return core.Style{Background: ui.Alpha(colors.Text(), 0.06)} },
		Children: []core.Node{
			ui.Icon(lucide.Search, 16, colors.Muted),
			core.Text{Content: label, MaxLines: 1, Style: ui.TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), StyleFunc: ui.Colored(colors.Muted)},
			core.Text{Content: cx.KeyLabel(searchKey), MaxLines: 1, Style: ui.TypeCaption, StyleFunc: ui.Colored(colors.Muted)},
		}}
}
