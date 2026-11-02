package main

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

// languageKey keeps the language chosen in the gallery's menu; without
// it the gallery follows the system's.
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

// languageMenu chooses the language the gallery shows: the system's, or
// one of its catalogs', each named in itself. The gallery's own texts
// stay English but the menu's; base's follow, and Arabic lays it out
// right to left.
func (g *gallery) languageMenu(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	l := newMenuLook(cx)
	item := func(name func() string, lang string) core.Node {
		return l.item(cx, itemProps{label: name, onSelect: func() { g.setLanguage(cx, lang) }, mark: func() mark {
			if g.language.Get() == lang {
				return markCheck
			}
			return markNone
		}})
	}
	items := []core.Node{
		core.Text{ContentFunc: func() string { return locales.Language(cx) }, Style: core.Style{FontSize: 11, FontWeight: ui.Semibold,
			LetterSpacing: 0.6, TextTransform: core.TextTransformUppercase, Padding: core.Edges{Top: 8, Right: 8, Bottom: 4, Left: 8}},
			StyleFunc: ui.Colored(colors.Faint)},
		item(func() string { return locales.LanguageSystem(cx) }, ""),
		separator(cx),
	}
	for _, lang := range locales.Catalog.Languages() {
		items = append(items, item(label(locales.In(lang).LanguageName()), lang))
	}
	// The trigger's label is a string: the menu is made again in a new
	// language.
	return core.Dynamic(func(cx *core.Scope) core.Node {
		return base.Menu{Align: core.AlignEnd, Offset: 4, Margin: 8, PopupStyleFunc: l.surface(cx), BackdropStyle: l.backdrop(),
			Label: locales.Language(cx), TestID: "language",
			Style: core.Style{Width: core.Pt(30), Height: core.Pt(30), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				CornerRadius: 8, Cursor: core.CursorPointer},
			StyleFunc: func() core.Style {
				return core.Style{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}, Focus: ui.FocusRing(cx, -2)}
			},
			Trigger:  ui.Icon(ui.IconGlobe, 18, colors.Muted),
			Children: items}
	})
}
