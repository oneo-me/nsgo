// Package i18n says an app's messages in the language it shows: what the
// code nsgo writes from a package's catalogs (locales/*.yaml) runs on.
// Apps call that generated code, and Language for the language itself.
//
// A catalog's languages are its own and those other packages supply, as
// an app supplies German to NSGo's base. The app's catalog decides which
// language the app shows; every other package's messages follow it.
package i18n

import (
	"sync"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/cldr"
	"golang.org/x/text/language"
)

// Catalog is the languages a package's messages are written in: its
// source language, first, its translations, and the languages other
// packages supply. The code nsgo writes makes one per package.
type Catalog struct {
	mu      sync.RWMutex
	langs   []string
	matcher language.Matcher
	matched map[string]string
}

// NewCatalog returns the languages of a catalog written in source.
func NewCatalog(source string, translations ...string) *Catalog {
	return &Catalog{langs: append([]string{source}, translations...)}
}

// Source returns the language the catalog is written in.
func (c *Catalog) Source() string { return c.langs[0] }

// Languages returns the catalog's languages, the source's first.
func (c *Catalog) Languages() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string(nil), c.langs...)
}

// Add adds a language another package supplies the messages of.
func (c *Catalog) Add(lang string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range c.langs {
		if l == lang {
			return
		}
	}
	c.langs = append(c.langs, lang)
	c.matcher, c.matched = nil, nil
}

// Match returns the catalog's language that suits locale best, as
// zh-Hant suits zh-TW and de suits de-CH, or the source language when
// none does. A pseudo locale (IsPseudo) suits every catalog.
func (c *Catalog) Match(locale string) string {
	if IsPseudo(locale) {
		return locale
	}
	c.mu.RLock()
	lang, ok := c.matched[locale]
	c.mu.RUnlock()
	if ok {
		return lang
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.matcher == nil {
		tags := make([]language.Tag, len(c.langs))
		for i, l := range c.langs {
			tags[i] = language.Make(l)
		}
		c.matcher, c.matched = language.NewMatcher(tags), map[string]string{}
	}
	lang = c.langs[0]
	if t, err := language.Parse(locale); err == nil {
		if _, i, conf := c.matcher.Match(t); conf >= language.High {
			lang = c.langs[i]
		}
	}
	c.matched[locale] = lang
	return lang
}

// SetApp makes c the app's catalog: the languages it shows
// (core.SetLanguageMatch), nil none. The code nsgo writes for the catalogs
// of nsgo.yaml's i18n calls it.
func SetApp(c *Catalog) {
	if c == nil {
		core.SetLanguageMatch(nil)
		return
	}
	core.SetLanguageMatch(c.Match)
}

// Language returns the language the app shows under cx
// (core.Env.Language): of its catalog's languages, the one that suits the
// locale it runs in best, else its source language; English for an app
// without a catalog. Read in a binding, it follows the language as it
// changes, as Scope.SetLocale or the user changes it.
func Language(cx *core.Scope) string {
	if lang := cx.Env().Language(); lang != "" {
		return lang
	}
	return "en"
}

var parents sync.Map // string → string

// Parent returns the language a message missing in lang is looked up in
// next, by CLDR's parent locales: en-001 for en-AU, en for en-001,
// zh-Hant for zh-TW; "" after the language alone.
func Parent(lang string) string {
	if p, ok := parents.Load(lang); ok {
		return p.(string)
	}
	p := ""
	if t, err := language.Parse(lang); err == nil {
		if t = t.Parent(); !t.IsRoot() {
			p = t.String()
		}
	}
	parents.Store(lang, p)
	return p
}

// The pseudo locales, which test an app's layouts before any translation
// exists, as Android's do: every catalog says its source language's
// messages transformed in them.
const (
	// PseudoAccents is en-XA: letters accented, messages a third longer
	// and bracketed, "[Šöŕţ ƀý name one]", so text that is not in a
	// catalog, is cut, or is pieced together shows.
	PseudoAccents = cldr.PseudoAccents
	// PseudoBidi is ar-XB: each word shown right to left, and the app
	// laid out right to left, so what does not mirror shows.
	PseudoBidi = cldr.PseudoBidi
)

// IsPseudo reports whether lang is a pseudo locale, en-XA or ar-XB.
func IsPseudo(lang string) bool { return lang == PseudoAccents || lang == PseudoBidi }

// Pseudo transforms s, a message in the source language whose arguments
// are marked (Arg), as the pseudo locale lang shows it. The code nsgo
// writes calls it.
func Pseudo(lang, s string) string { return cldr.Pseudolocalize(lang, s) }

// Arg marks what an argument wrote in a message Pseudo transforms, which
// keeps it as written. The code nsgo writes calls it.
func Arg(s string) string { return cldr.ArgStart + s + cldr.ArgEnd }

// Texts are a catalog's messages without arguments in one language, by
// index: their text one after another, each as long in bytes as lens
// says, 0 where the language lacks one. The code nsgo writes holds a
// language's as one string, which the linker writes as it is, rather
// than a string for each, whose header the program must fix up.
type Texts struct {
	text string
	lens []uint16
	once sync.Once
	ends []uint32
}

// NewTexts returns the texts in text, as long as lens says.
func NewTexts(text string, lens []uint16) *Texts { return &Texts{text: text, lens: lens} }

// At returns the message at i, "" where the language lacks it.
func (t *Texts) At(i int) string {
	t.once.Do(func() {
		t.ends = make([]uint32, len(t.lens)+1)
		for j, n := range t.lens {
			t.ends[j+1] = t.ends[j] + uint32(n)
		}
	})
	return t.text[t.ends[i]:t.ends[i+1]]
}
