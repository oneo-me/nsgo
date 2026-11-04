// Package cldr formats dates and times by the Unicode CLDR's data, as ICU
// and Intl write them, and transforms messages for the pseudo locales,
// for package i18n. tables.go holds the data of the locales of CLDR's
// modern coverage, generated from a pinned release of cldr-json by go
// generate (internal/gen).
package cldr

//go:generate go run ./internal/gen

import (
	"compress/flate"
	"io"
	"strings"
	"sync"

	"golang.org/x/text/language"
)

// tables are the generated tables, inflated on first use.
var tables = sync.OnceValue(func() (t struct {
	names   []string
	records []string
}) {
	t.names = strings.Split(localeNames, " ")
	data, err := io.ReadAll(flate.NewReader(strings.NewReader(packedLocales)))
	if err != nil {
		panic("cldr: the tables do not inflate: " + err.Error())
	}
	t.records = strings.Split(string(data), "\x1d")
	return t
})

// localeData is a locale's entries, its parents' included (see
// internal/gen for the keys).
type localeData map[string]string

var resolved sync.Map // int → localeData

// dataAt returns the data of the locale at index i of the tables.
func dataAt(i int) localeData {
	if d, ok := resolved.Load(i); ok {
		return d.(localeData)
	}
	d := localeData{}
	if p := localeParents[i]; p >= 0 {
		for k, v := range dataAt(int(p)) {
			d[k] = v
		}
	}
	if rec := tables().records[i]; rec != "" {
		for _, f := range strings.Split(rec, "\x1e") {
			k, v, _ := strings.Cut(f, "\x1f")
			d[k] = v
		}
	}
	resolved.Store(i, d)
	return d
}

var found sync.Map // string → int

// find returns the index of the locale whose data suits locale: its own,
// else the nearest with its language, script and region (zh-Hant for
// zh-TW, en-AU for en-AU), else English.
func find(locale string) int {
	if i, ok := found.Load(locale); ok {
		return i.(int)
	}
	i := search(locale)
	found.Store(locale, i)
	return i
}

func search(locale string) int {
	names := tables().names
	at := func(name string) int {
		lo, hi := 0, len(names)
		for lo < hi {
			m := (lo + hi) / 2
			if names[m] < name {
				lo = m + 1
			} else {
				hi = m
			}
		}
		if lo < len(names) && names[lo] == name {
			return lo
		}
		return -1
	}
	t, err := language.Parse(strings.ReplaceAll(locale, "_", "-"))
	if err == nil {
		base, _ := t.Base()
		script, _ := t.Script()
		region, rconf := t.Region()
		b, s := base.String(), script.String()
		likely, _ := language.Make(b).Script()
		var candidates []string
		if rconf == language.Exact {
			candidates = append(candidates, b+"-"+s+"-"+region.String())
			if likely == script {
				candidates = append(candidates, b+"-"+region.String())
			}
		}
		candidates = append(candidates, b+"-"+s)
		if likely == script {
			candidates = append(candidates, b)
		}
		for _, c := range candidates {
			if i := at(c); i >= 0 {
				return i
			}
		}
	}
	return at("en")
}
