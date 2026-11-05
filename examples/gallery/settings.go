package main

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"
)

// settingsButton opens the gallery's settings, at the start of the
// sidebar's footer and in the index's header.
func (g *gallery) settingsButton(cx *core.Scope) core.Node {
	return ui.IconButton{Icon: lucide.Settings, Label: locales.Settings(cx), Size: 30, TestID: "settings",
		OnPress: func() { g.settingsOpen.Set(true) }}
}

// barSettingsButton is the settings button in the system's bar, the icon
// alone, as barThemeToggle is.
func (g *gallery) barSettingsButton(cx *core.Scope) core.Node {
	return ui.Press{OnPress: func() { g.settingsOpen.Set(true) }, Scale: 0.92, Label: locales.Settings(cx), Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(34), Height: core.Pt(34), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{ui.Icon(lucide.Settings, 18, ui.ColorsOf(cx).Text)}},
	}}
}

// settingsDialog is the gallery's settings (ui.Settings) in a dialog over
// the window, narrow on a phone's width.
func (g *gallery) settingsDialog(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	env := cx.Env()
	width := func() core.Points { return min(800, env.Window().Width*0.92) }
	categories := []ui.SettingsCategory{{ID: "general", Title: locales.SettingsGeneral(cx), Icon: lucide.Settings,
		Groups: []ui.SettingsGroup{{ID: "language", Items: []ui.SettingsItem{{
			ID: "language", Title: locales.Language(cx), Description: locales.SettingsLanguageDetail(cx),
			Keywords: append([]string{"language", languageName(cx, "")}, languageNames()...),
			Control:  g.languageSelect(cx),
		}}}}}}
	close := func() { g.settingsOpen.Set(false) }
	return base.Dialog{Open: g.settingsOpen.Get, Animation: core.ModalFade, Label: locales.Settings(cx), TestID: "settings-dialog",
		OnOpenChange:  func(open bool, _ base.Reason) { g.settingsOpen.Set(open) },
		BackdropStyle: core.Style{Background: ui.Black(0.4)},
		PopupStyleFunc: func() core.Style {
			return ui.CardStyle(cx).Merge(core.Style{Width: core.Pt(width()), Height: core.Pt(min(560, env.Window().Height*0.85)),
				Overflow: core.Hidden, Color: colors.Text()})
		},
		Children: []core.Node{ui.Settings{Categories: categories, Category: g.settingsCategory, Query: g.settingsQuery,
			Narrow: func() bool { return width() < 600 }, TestID: "settings-surface",
			Title: locales.Settings(cx), SearchLabel: locales.SettingsSearch(cx), SearchPlaceholder: locales.SettingsSearchPlaceholder(cx),
			ResultsTitle: locales.SettingsResults(cx), EmptyText: locales.SettingsEmpty(cx),
			OnClose: close, CloseLabel: locales.SettingsClose(cx)}},
	}
}

// languageNames are the catalogs' languages named in themselves, for the
// language row's search.
func languageNames() []string {
	var names []string
	for _, lang := range locales.Catalog.Languages() {
		names = append(names, locales.In(lang).LanguageName())
	}
	return names
}
