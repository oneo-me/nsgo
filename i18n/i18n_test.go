package i18n_test

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/i18n"
	"github.com/oneo-me/nsgo/nsgotest"
)

func TestMatch(t *testing.T) {
	c := i18n.NewCatalog("en", "de", "zh-Hans", "zh-Hant", "en-GB")
	for locale, want := range map[string]string{
		"":           "en",
		"en-US":      "en",
		"de-CH":      "de",
		"zh-CN":      "zh-Hans",
		"zh-TW":      "zh-Hant",
		"zh-HK":      "zh-Hant",
		"en-AU":      "en-GB",
		"ja":         "en",
		"not a tag!": "en",
	} {
		if got := c.Match(locale); got != want {
			t.Errorf("Match(%q) = %q, want %q", locale, got, want)
		}
	}
	if got := c.Match("fr-CA"); got != "en" {
		t.Errorf("Match(fr-CA) = %q before fr is added", got)
	}
	c.Add("fr")
	if got := c.Match("fr-CA"); got != "fr" {
		t.Errorf("Match(fr-CA) = %q after fr is added", got)
	}
}

func TestParent(t *testing.T) {
	for lang, want := range map[string]string{
		"en-AU": "en-001", "en-001": "en", "en": "", "zh-TW": "zh-Hant", "zh-Hant": "", "pt-AO": "pt-PT", "de-CH": "de",
	} {
		if got := i18n.Parent(lang); got != want {
			t.Errorf("Parent(%q) = %q, want %q", lang, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	for _, c := range []struct {
		lang    string
		n       float64
		ordinal bool
		want    i18n.Category
	}{
		{"en", 1, false, i18n.One},
		{"en", 1.0004, false, i18n.One},
		{"en", 1.5, false, i18n.Other},
		{"en", 0, false, i18n.Other},
		{"zh-Hans", 1, false, i18n.Other},
		{"ru", 3, false, i18n.Few},
		{"ru", 5, false, i18n.Many},
		{"ar", 0, false, i18n.Zero},
		{"en", 22, true, i18n.Two},
		{"en", 103, true, i18n.Few},
		{"en", 11, true, i18n.Other},
	} {
		got := i18n.Cardinal(c.lang, c.n)
		if c.ordinal {
			got = i18n.Ordinal(c.lang, c.n)
		}
		if got != c.want {
			t.Errorf("%s %v (ordinal %v) = %v, want %v", c.lang, c.n, c.ordinal, got, c.want)
		}
	}
}

func TestFormat(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{i18n.FormatNumber("en", 1234.5), "1,234.5"},
		{i18n.FormatNumber("de", 1234.5), "1.234,5"},
		{i18n.FormatNumber("en", 1.0004), "1"},
		{i18n.FormatInteger("en", 1234.4), "1,234"},
		{i18n.FormatPercent("en", 0.25), "25%"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestLanguage(t *testing.T) {
	var lang func() string
	app := func(cx *core.Scope) core.Node {
		lang = func() string { return i18n.Language(cx) }
		return core.Text{TestID: "lang", ContentFunc: lang}
	}
	a := nsgotest.Start(t, app, nsgotest.Environment(core.Environment{Locale: "zh-TW"}))
	if got := lang(); got != "en" {
		t.Errorf("without an app catalog: %q, want en", got)
	}
	i18n.SetApp(i18n.NewCatalog("en", "zh-Hant"))
	defer i18n.SetApp(nil)
	a.SetEnvironment(core.Environment{Locale: "zh-HK"})
	if got := a.Find("id:lang").Text; got != "zh-Hant" {
		t.Errorf("zh-HK shows %q, want zh-Hant", got)
	}
	a.SetEnvironment(core.Environment{Locale: "fr"})
	if got := a.Find("id:lang").Text; got != "en" {
		t.Errorf("fr shows %q, want en", got)
	}
}
