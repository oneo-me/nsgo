package i18n

import (
	"math"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

// Category is a number's plural category in a language, by CLDR's
// rules: one and other in English, other alone in Chinese, six in Arabic.
type Category uint8

const (
	Other Category = iota
	Zero
	One
	Two
	Few
	Many
)

// String returns the category's name in a message: "one", "other", ...
func (c Category) String() string {
	return [...]string{"other", "zero", "one", "two", "few", "many"}[c]
}

// Cardinal returns the plural category of n in lang, as FormatNumber
// writes it: 1.0004 is written 1, whose category in English is one.
func Cardinal(lang string, n float64) Category { return category(plural.Cardinal, lang, n) }

// Ordinal returns the ordinal category of n in lang: one, two, few and
// other in English (1st, 2nd, 3rd, 4th).
func Ordinal(lang string, n float64) Category { return category(plural.Ordinal, lang, n) }

func category(rules *plural.Rules, lang string, n float64) Category {
	s := strconv.FormatFloat(math.Abs(n), 'f', 3, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	whole, frac, _ := strings.Cut(s, ".")
	i, err := strconv.Atoi(whole)
	if err != nil {
		// Beyond int's range: a million's multiple, as the rules see it.
		i = 1_000_000_000
	}
	f, _ := strconv.Atoi(frac)
	v := len(frac)
	return Category(rules.MatchPlural(tag(lang), i, v, v, f, f))
}

var tags, printers sync.Map // string → language.Tag, *message.Printer

func tag(lang string) language.Tag {
	if t, ok := tags.Load(lang); ok {
		return t.(language.Tag)
	}
	t := language.Make(lang)
	tags.Store(lang, t)
	return t
}

func printer(lang string) *message.Printer {
	if p, ok := printers.Load(lang); ok {
		return p.(*message.Printer)
	}
	p := message.NewPrinter(tag(lang))
	printers.Store(lang, p)
	return p
}

// FormatNumber writes n in lang's way, with up to three fraction digits
// and grouped: 1,234.5 in English, 1.234,5 in German.
func FormatNumber(lang string, n float64) string { return printer(lang).Sprint(number.Decimal(n)) }

// FormatInteger writes n rounded to an integer in lang's way.
func FormatInteger(lang string, n float64) string {
	return printer(lang).Sprint(number.Decimal(n, number.MaxFractionDigits(0)))
}

// FormatPercent writes n, a fraction, as a percentage in lang's way: 25%
// in English for 0.25, 25 % in German.
func FormatPercent(lang string, n float64) string { return printer(lang).Sprint(number.Percent(n)) }
