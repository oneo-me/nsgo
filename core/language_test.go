package core_test

import (
	"strings"
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// languageApp shows the locale, the language and the direction, and two
// boxes in a row, which a press forces into locale.
func languageApp(locale string) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		e := cx.Env()
		return core.View{Children: []core.Node{
			core.Text{TestID: "env", ContentFunc: func() string {
				dir := "ltr"
				if e.RTL() {
					dir = "rtl"
				}
				return e.Locale() + " " + e.Language() + " " + dir
			}},
			core.View{TestID: "force", OnPress: func(core.PressEvent) { cx.SetLocale(locale) }, Style: core.Style{Height: core.Pt(20)}},
			core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
				core.View{TestID: "first", Style: core.Style{Width: core.Pt(10), Height: core.Pt(10)}},
				core.View{TestID: "second", Style: core.Style{Width: core.Pt(10), Height: core.Pt(10)}},
			}},
		}}
	}
}

func TestDirectionWithoutCatalogs(t *testing.T) {
	app := nsgotest.Start(t, languageApp("ar"), nsgotest.Environment(core.Environment{Locale: "he-IL", RTL: true}))
	if got := app.Find("id:env").Text; got != "he-IL  rtl" {
		t.Errorf("the system's: %q", got)
	}
	app.SetEnvironment(core.Environment{Locale: "en-US"})
	if got := app.Find("id:env").Text; got != "en-US  ltr" {
		t.Errorf("the system's: %q", got)
	}
	// A forced locale lays out as it is written.
	app.Press("id:force")
	if got := app.Find("id:env").Text; got != "ar  rtl" {
		t.Errorf("forced: %q", got)
	}
	if first, second := app.Find("id:first").Frame, app.Find("id:second").Frame; first.Origin.X <= second.Origin.X {
		t.Errorf("forced Arabic lays the row out left to right: %v, %v", first, second)
	}
}

func TestPlatformHearsLanguage(t *testing.T) {
	app := nsgotest.Start(t, languageApp("zh-Hans"), nsgotest.Environment(core.Environment{Locale: "en-US"}))
	if app.Language != "en-US" {
		t.Errorf("the system's: %q", app.Language)
	}
	app.Press("id:force")
	if app.Language != "zh-Hans" {
		t.Errorf("forced: %q", app.Language)
	}
}

func TestDirectionFollowsLanguage(t *testing.T) {
	core.SetLanguageMatch(func(locale string) string {
		if strings.HasPrefix(locale, "ar") {
			return "ar"
		}
		return "en"
	})
	defer core.SetLanguageMatch(nil)
	// The system is Hebrew, which the app does not show: it shows English,
	// left to right.
	app := nsgotest.Start(t, languageApp("ar-EG"), nsgotest.Environment(core.Environment{Locale: "he-IL", RTL: true}))
	if got := app.Find("id:env").Text; got != "he-IL en ltr" {
		t.Errorf("Hebrew system: %q", got)
	}
	app.Press("id:force")
	if got := app.Find("id:env").Text; got != "ar-EG ar rtl" {
		t.Errorf("forced Arabic: %q", got)
	}
	if first, second := app.Find("id:first").Frame, app.Find("id:second").Frame; first.Origin.X <= second.Origin.X {
		t.Errorf("Arabic lays the row out left to right: %v, %v", first, second)
	}
}
