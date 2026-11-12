package i18n

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nsgo-dev/nsgo/internal/cldr"

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

// FormatNumberSkeleton writes n in lang's way by an ICU number skeleton,
// as a message's {n, number, ::.00} does: of its stems percent, integer,
// the fraction digits (.00 two, .0# one or two) and group-off.
func FormatNumberSkeleton(lang string, n float64, skeleton string) string {
	var opts []number.Option
	percent := false
	for _, stem := range strings.Fields(skeleton) {
		switch {
		case stem == "percent":
			percent = true
		case stem == "integer", stem == "precision-integer":
			opts = append(opts, number.MaxFractionDigits(0))
		case stem == "group-off":
			opts = append(opts, number.NoSeparator())
		case strings.HasPrefix(stem, "."):
			opts = append(opts, number.MinFractionDigits(strings.Count(stem, "0")), number.MaxFractionDigits(len(stem)-1))
		}
	}
	if percent {
		return printer(lang).Sprint(number.Percent(n, opts...))
	}
	return printer(lang).Sprint(number.Decimal(n, opts...))
}

// Style is the length of a date or a time as CLDR's patterns write it.
type Style = cldr.Style

const (
	// NoStyle leaves the date, or the time, out.
	NoStyle = cldr.NoStyle
	// ShortStyle is numeric: "1/5/26", "1:07 PM".
	ShortStyle = cldr.ShortStyle
	// MediumStyle abbreviates: "Jan 5, 2026", "1:07:09 PM".
	MediumStyle = cldr.MediumStyle
	// LongStyle writes the month out: "January 5, 2026".
	LongStyle = cldr.LongStyle
	// FullStyle writes everything: "Monday, January 5, 2026".
	FullStyle = cldr.FullStyle
)

// DateOptions are how FormatDate writes a time: its date, its time or
// both by style, or else the fields of a skeleton of CLDR's letters
// ("yMMMd", "jm") in the order and form the language writes them.
type DateOptions = cldr.DateOptions

// FormatDate writes t, in its location, in lang's way, in the Gregorian
// calendar, by CLDR's patterns and names: "Jan 5, 2026", "2026年1月5日".
// Time zones are written as offsets from GMT.
func FormatDate(lang string, t time.Time, o DateOptions) string { return cldr.FormatDate(lang, t, o) }
