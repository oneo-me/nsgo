package main

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"
)

// settingsButton opens the gallery's settings, at the start of the
// sidebar's footer.
func (g *gallery) settingsButton(cx *core.Scope) core.Node {
	return ui.IconButton{Icon: lucide.Settings, Label: locales.Settings(cx), Size: 30, TestID: "settings",
		OnPress: func() { g.settingsOpen.Set(true) }}
}

// settingsCategories are the gallery's settings.
func (g *gallery) settingsCategories(cx *core.Scope) []ui.SettingsCategory {
	return []ui.SettingsCategory{{ID: "general", Title: locales.SettingsGeneral(cx), Icon: lucide.Settings,
		Groups: []ui.SettingsGroup{{ID: "general", Items: []ui.SettingsItem{{
			ID: "language", Title: locales.Language(cx), Description: locales.SettingsLanguageDetail(cx),
			Keywords: append([]string{"language", languageName(cx, "")}, languageNames()...),
			Control:  g.languageSelect(cx),
		}, {
			ID: "theme", Title: locales.Theme(cx), Description: locales.SettingsThemeDetail(cx),
			Keywords: []string{"theme", "appearance", "dark", "light", locales.ThemeSystem(cx), locales.ThemeLight(cx), locales.ThemeDark(cx)},
			Control:  themeSelect(cx),
		}}}}}}
}

// settingsDialog is the gallery's settings (ui.Settings) in a dialog over
// the desktop layout, narrow on a small window's width.
func (g *gallery) settingsDialog(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	env := cx.Env()
	width := func() core.Points { return min(800, env.Window().Width*0.92) }
	close := func() { g.settingsOpen.Set(false) }
	return base.Dialog{Open: g.settingsOpen.Get, Animation: core.ModalFade, Label: locales.Settings(cx), TestID: "settings-dialog",
		OnOpenChange:  func(open bool, _ base.Reason) { g.settingsOpen.Set(open) },
		BackdropStyle: core.Style{Background: ui.Black(0.4)},
		PopupStyleFunc: func() core.Style {
			return ui.CardStyle(cx).Merge(core.Style{Width: core.Pt(width()), Height: core.Pt(min(560, env.Window().Height*0.85)),
				Overflow: core.Hidden, Color: colors.Text()})
		},
		Children: []core.Node{ui.Settings{Categories: g.settingsCategories(cx), Category: g.settingsCategory, Query: g.settingsQuery,
			Narrow: func() bool { return width() < 600 }, TestID: "settings-surface",
			Title: locales.Settings(cx), SearchLabel: locales.SettingsSearch(cx), SearchPlaceholder: locales.SettingsSearchPlaceholder(cx),
			ResultsTitle: locales.SettingsResults(cx), EmptyText: locales.SettingsEmpty(cx),
			OnClose: close, CloseLabel: locales.SettingsClose(cx)}},
	}
}

// settingsPage is the settings as the narrow layout's tab, under its
// header: its rows alone.
func (g *gallery) settingsPage(cx *core.Scope) core.Node {
	return ui.Settings{Categories: g.settingsCategories(cx), Narrow: func() bool { return true }, TestID: "settings-surface",
		HideHeading: true, HideSearch: true, HideSidebar: true, EmptyText: locales.SettingsEmpty(cx)}
}

// themeKey keeps the appearance chosen in the settings or with the theme
// toggle; without it the gallery follows the system's.
const themeKey = "theme"

// rememberAppearance shows the appearance chosen last time.
func rememberAppearance(cx *core.Scope) {
	switch theme, _ := cx.Storage().GetItem(themeKey); theme {
	case "light":
		cx.SetAppearance(core.AppearanceLight)
	case "dark":
		cx.SetAppearance(core.AppearanceDark)
	}
}

// setAppearance shows a, core.AppearanceSystem the system's, and keeps the
// choice.
func setAppearance(cx *core.Scope, a core.Appearance) {
	cx.SetAppearance(a)
	switch a {
	case core.AppearanceLight:
		cx.Storage().SetItem(themeKey, "light")
	case core.AppearanceDark:
		cx.Storage().SetItem(themeKey, "dark")
	default:
		cx.Storage().RemoveItem(themeKey)
	}
}

// themeSelect chooses the appearance: the system's, light or dark.
func themeSelect(cx *core.Scope) core.Node {
	return ui.Select[core.Appearance]{Options: []ui.Option[core.Appearance]{
		{Value: core.AppearanceSystem, Label: locales.ThemeSystem(cx)},
		{Value: core.AppearanceLight, Label: locales.ThemeLight(cx)},
		{Value: core.AppearanceDark, Label: locales.ThemeDark(cx)},
	}, Value: cx.Env().Appearance, OnChange: func(a core.Appearance) { setAppearance(cx, a) },
		Label: locales.Theme(cx), TestID: "theme", Style: core.Style{Width: core.Pt(140)}}
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
