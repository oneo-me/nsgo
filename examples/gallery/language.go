package main

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/i18n"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"
)

// languageKey keeps the language chosen in the gallery's settings;
// without it the gallery follows the system's.
const languageKey = "language"

// rememberLanguage shows the language chosen last time.
func rememberLanguage(cx *core.Scope, g *gallery) {
	if lang, ok := cx.Storage().GetItem(languageKey); ok && lang != "" {
		g.language.Set(lang)
		cx.SetLocale(lang)
	}
}

// setLanguage shows lang, "" the system's, and keeps the choice.
func (g *gallery) setLanguage(cx *core.Scope, lang string) {
	g.language.Set(lang)
	cx.SetLocale(lang)
	if lang == "" {
		cx.Storage().RemoveItem(languageKey)
	} else {
		cx.Storage().SetItem(languageKey, lang)
	}
}

// languageName is lang named in itself, "" the system's choice in the
// language shown.
func languageName(cx *core.Scope, lang string) string {
	if lang == "" {
		return locales.LanguageSystem(cx)
	}
	return locales.In(lang).LanguageName()
}

// languageSelect chooses the language the gallery shows: the system's,
// one of its catalogs', each named in itself, or a pseudo locale. Its button shows the
// choice; base's components follow it, and Arabic lays the gallery out
// right to left.
func (g *gallery) languageSelect(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	l := newMenuLook(cx)
	item := func(lang string) core.Node {
		return l.item(cx, itemProps{label: label(languageName(cx, lang)), onSelect: func() { g.setLanguage(cx, lang) }, mark: func() mark {
			if g.language.Get() == lang {
				return markCheck
			}
			return markNone
		}})
	}
	items := []core.Node{item(""), separator(cx)}
	for _, lang := range locales.Catalog.Languages() {
		items = append(items, item(lang))
	}
	// The pseudo locales, which show untranslated, cut and unmirrored
	// text.
	items = append(items, separator(cx), item(i18n.PseudoAccents), item(i18n.PseudoBidi))
	name := ui.Textf(func() string { return languageName(cx, g.language.Get()) }, 13, ui.Medium, colors.Text)
	name.MaxLines = 1
	return base.Menu{Align: core.AlignEnd, Offset: 4, Margin: 8, PopupStyleFunc: l.surface(cx), BackdropStyle: l.backdrop(),
		Label: locales.Language(cx), TestID: "language",
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Height: core.Pt(32), MinWidth: core.Pt(140),
			Padding: core.Symmetric(0, 10), CornerRadius: 8, BorderWidth: 1, Cursor: core.CursorPointer},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Raised(), BorderColor: colors.Border(), Hover: &core.Style{Background: colors.Hover()},
				Active: &core.Style{Background: colors.Pressed()}, Focus: ui.FocusRing(cx, -2)}
		},
		Trigger: core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, FlexGrow: 1}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, Children: []core.Node{name}},
			ui.Icon(lucide.ChevronDown, 14, colors.Faint),
		}},
		Children: items}
}
