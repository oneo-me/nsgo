package main

import (
	"os"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// searchKey opens the demos' search, in either layout.
const searchKey = "mod+k"

// commandInputSource is the Latin input source the palettes search in on
// each desktop, as demos are found by their English names in every
// language: macOS's ABC layout, Windows' US English layout, and the US
// keyboard of Fcitx 5 or IBus on Linux.
func commandInputSource(cx *core.Scope) string {
	switch cx.Env().OS() {
	case core.OSMacOS:
		return "com.apple.keylayout.ABC"
	case core.OSWindows:
		return "00000409"
	case core.OSLinux:
		if os.Getenv("GTK_IM_MODULE") == "ibus" || os.Getenv("XMODIFIERS") == "@im=ibus" {
			return "xkb:us::eng"
		}
		return "keyboard-us"
	}
	return ""
}

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
			Label: tr.AppSearchDemos(), Empty: tr.PickerNothingFound(), InputSource: commandInputSource(cx),
			OnSelect: func(name string) {
				g.searchOpen.Set(false)
				g.open(name)
			}}}
}

// searchButton is the sidebar's way to searchDialog, with the key beside it.
func (g *gallery) searchButton(tr locales.Messages) core.Node {
	return core.View{Style: core.Style{Padding: core.Edges{Right: 10, Bottom: 8, Left: 10}}, Children: []core.Node{
		ui.SidebarButton{TestID: "search", Variant: ui.SidebarButtonField, Icon: lucide.Search, Text: tr.AppSearchDemos(), Shortcut: searchKey,
			OnPress: func() { g.searchOpen.Set(true) }},
	}}
}
