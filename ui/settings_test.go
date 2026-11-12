package ui

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

var settingsCatalog = []SettingsCategory{
	{ID: "general", Title: "General", Icon: lucide.Settings, Groups: []SettingsGroup{
		{Title: "Startup", Items: []SettingsItem{
			{ID: "login", Title: "Open at login"},
			{ID: "restore", Title: "Restore windows", Description: "Reopen what was open"},
		}},
	}},
	{ID: "appearance", Title: "Appearance", Icon: lucide.Sun, Groups: []SettingsGroup{
		{Items: []SettingsItem{{ID: "theme", Title: "Theme", Keywords: []string{"dark", "light"}}}},
		{Title: "Text", Items: []SettingsItem{{ID: "font", Title: "Font size"}}},
	}},
}

// TestSearchSettings needs every term somewhere in an item or its titles.
func TestSearchSettings(t *testing.T) {
	cases := map[string][]SettingsResult{
		"":                {{0, 0, 0}, {0, 0, 1}, {1, 0, 0}, {1, 1, 0}},
		"DARK":            {{1, 0, 0}},
		"startup reopen":  {{0, 0, 1}},
		"appearance text": {{1, 1, 0}},
		"nothing":         nil,
	}
	for q, want := range cases {
		if got := SearchSettings(settingsCatalog, q); !slices.Equal(got, want) {
			t.Errorf("SearchSettings(%q) = %v, want %v", q, got, want)
		}
	}
}

// TestSettings shows the selected category's rows, the rows a query
// finds across categories, and a category again once one is chosen.
func TestSettings(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var query *core.Signal[string]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			query = core.NewSignal(cx, "")
			narrow := e.Idiom != core.IdiomDesktop
			return core.View{Style: core.Style{Height: core.Pt(600)}, Children: []core.Node{
				Settings{TestID: "settings", Categories: settingsCatalog, Query: query, Narrow: func() bool { return narrow },
					Title: "Settings", SearchLabel: "Search settings", ResultsTitle: "Results", EmptyText: "No settings found"}}}
		})
		app.Find("settings-general-login")
		if app.Exists("settings-appearance-theme") || !app.Find("settings-nav-general").Props.Selected {
			t.Fatal("the first category is not the one shown")
		}
		app.Press("settings-nav-appearance")
		app.Find("settings-appearance-font")
		app.Type("settings-search", "o")
		for _, id := range []string{"general-login", "general-restore", "appearance-font"} {
			app.Find("settings-" + id)
		}
		app.Type("settings-search", "pen at log")
		if !app.Exists("settings-general-login") || app.Exists("settings-general-restore") {
			t.Fatal("the search shows other than the row it finds")
		}
		app.Type("settings-search", "zz")
		app.Find("No settings found")
		app.Press("settings-nav-general")
		if query.Get() != "" || !app.Exists("settings-general-restore") {
			t.Fatal("choosing a category kept the search")
		}
	})
}

// TestSettingsHideHeading leaves the title to the host's header.
func TestSettingsHideHeading(t *testing.T) {
	app, _ := shell(t, desktopEnv, func(cx *core.Scope) core.Node {
		return Settings{Categories: settingsCatalog, Title: "Preferences", HideHeading: true, HideSearch: true, HideSidebar: true}
	})
	app.Find("Open at login")
	if app.Exists("Preferences") {
		t.Fatal("the heading shows")
	}
}
