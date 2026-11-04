package cldr

import (
	"slices"
	"strings"
)

// A skeleton names the fields of a date or time, as "yMMMd", and the
// locale's availableFormats give the patterns of the skeletons it writes.
// bestPattern finds the one closest to a skeleton and fits it: the
// closest has the skeleton's fields, then fields of the same kind (text
// or numbers), then of the nearest widths; a skeleton of both a date and
// a time the locale has no pattern for is a date and a time glued
// together by its date-time pattern.

// Field types, by letter.
const (
	typeEra = iota
	typeYear
	typeMonth
	typeWeekday
	typeDay
	typePeriod
	typeHour
	typeMinute
	typeSecond
	typeZone
	numTypes
)

var fieldTypes = map[byte]int{
	'G': typeEra, 'y': typeYear, 'Y': typeYear, 'u': typeYear,
	'M': typeMonth, 'L': typeMonth, 'E': typeWeekday, 'c': typeWeekday, 'e': typeWeekday,
	'd': typeDay, 'a': typePeriod, 'b': typePeriod, 'B': typePeriod,
	'h': typeHour, 'H': typeHour, 'K': typeHour, 'k': typeHour,
	'm': typeMinute, 's': typeSecond,
	'z': typeZone, 'v': typeZone, 'O': typeZone, 'V': typeZone,
}

// field is one field of a skeleton.
type field struct {
	letter byte
	n      int
}

// fields is a skeleton's fields by type; a zero letter is absent.
type fields [numTypes]field

// parseSkeleton reads a skeleton's fields; 12 hours imply a period,
// which availableFormats' skeletons leave out.
func parseSkeleton(s string) fields {
	var fs fields
	for _, it := range parsePattern(s) {
		if t, ok := fieldTypes[it.letter]; ok {
			fs[t] = field{it.letter, it.n}
		}
	}
	if fs[typeHour].twelve() && !fs.has(typePeriod) {
		fs[typePeriod] = field{'a', 1}
	}
	return fs
}

// numeric reports whether a field writes a number rather than a name.
func (f field) numeric() bool {
	switch f.letter {
	case 'M', 'L', 'c', 'e':
		return f.n <= 2
	case 'G', 'E', 'a', 'b', 'B', 'z', 'v', 'O', 'V':
		return false
	}
	return true
}

// twelve reports whether an hour field counts 12 hours.
func (f field) twelve() bool { return f.letter == 'h' || f.letter == 'K' }

func (fs *fields) has(t int) bool { return fs[t].letter != 0 }

func (fs *fields) date() bool {
	return fs.has(typeEra) || fs.has(typeYear) || fs.has(typeMonth) || fs.has(typeWeekday) || fs.has(typeDay)
}

func (fs *fields) time() bool {
	return fs.has(typeHour) || fs.has(typeMinute) || fs.has(typeSecond) || fs.has(typeZone)
}

// distance is how far a pattern's skeleton is from the one asked for:
// each field it lacks, each it has besides, each of another kind, and the
// difference of widths, in that order of weight.
func distance(want, have *fields) (d, missing int) {
	for t := range numTypes {
		w, h := want[t], have[t]
		switch {
		case w.letter == 0 && h.letter == 0:
		case h.letter == 0:
			d += 0x1000
			missing++
		case w.letter == 0:
			d += 0x10000
		case w.numeric() != h.numeric(), t == typeHour && w.twelve() != h.twelve(), t == typePeriod && w.letter != h.letter:
			d += 0x100
		default:
			d += min(abs(w.n-h.n), 0xff)
		}
	}
	return d, missing
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// hourLetter is the locale's preferred hour letter, its short time's.
func hourLetter(d localeData) byte {
	for _, it := range parsePattern(d["T.short"]) {
		switch it.letter {
		case 'h', 'H', 'K', 'k':
			return it.letter
		}
	}
	return 'H'
}

// bestPattern returns d's pattern for skeleton.
func bestPattern(d localeData, skeleton string) string {
	// The locale's own hour, as j, takes the skeleton's width.
	hour := hourLetter(d)
	local := strings.ContainsAny(skeleton, "jJC"+string(hour))
	skeleton = strings.Map(func(r rune) rune {
		if r == 'j' || r == 'J' || r == 'C' {
			return rune(hour)
		}
		return r
	}, skeleton)
	want := parseSkeleton(skeleton)
	if p, missing := closest(d, &want, local); missing == 0 || !(want.date() && want.time()) {
		return p
	}
	var date, tm fields
	for t := range numTypes {
		if t <= typeDay {
			date[t] = want[t]
		} else {
			tm[t] = want[t]
		}
	}
	dp, _ := closest(d, &date, local)
	tp, _ := closest(d, &tm, local)
	style := "short"
	switch {
	case date[typeMonth].n >= 4 && date.has(typeWeekday):
		style = "full"
	case date[typeMonth].n >= 4:
		style = "long"
	case date[typeMonth].n == 3:
		style = "medium"
	}
	glue := d["AT."+style]
	if glue == "" {
		glue = d["DT."+style]
	}
	return strings.NewReplacer("{1}", dp, "{0}", tp).Replace(glue)
}

// closest returns the pattern of d's skeleton nearest want, fitted to
// it, and how many of want's fields it lacks.
func closest(d localeData, want *fields, localHour bool) (string, int) {
	var keys []string
	for k := range d {
		if strings.HasPrefix(k, "S.") {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	best, bestD, bestMissing := "", -1, 0
	var bestHave fields
	for _, k := range keys {
		have := parseSkeleton(k[2:])
		dist, missing := distance(want, &have)
		if bestD < 0 || dist < bestD {
			best, bestD, bestMissing, bestHave = d[k], dist, missing, have
		}
	}
	return fit(best, want, &bestHave, localHour), bestMissing
}

// fit fits a pattern, of the skeleton have, to the skeleton want, as
// ICU's pattern generator does: a field whose width have already has
// stays as the pattern writes it; otherwise a name takes the width asked
// for and a number the wider, while a number the pattern writes for a
// name asked for stays a number (2026年1月, not 2026年一月). Minutes and
// seconds keep the pattern's widths. The locale's hour (j) takes the
// width asked for; an hour of a cycle asked for (h, H) its letter.
func fit(pattern string, want, have *fields, localHour bool) string {
	var b strings.Builder
	for _, it := range parsePattern(pattern) {
		if it.letter == 0 {
			b.WriteString(quote(it.text))
			continue
		}
		letter, n := it.letter, it.n
		t, ok := fieldTypes[letter]
		if ok && want.has(t) {
			w, f := want[t], field{letter, n}
			switch {
			case t == typeHour:
				if localHour {
					n = w.n
				} else if w.twelve() == f.twelve() {
					letter = w.letter
				}
			case t == typeMinute, t == typeSecond, w == have[t]:
			case !f.numeric() && !w.numeric():
				n = w.n
			case f.numeric() && w.numeric() && w.n > n:
				n = w.n
			}
		}
		b.WriteString(strings.Repeat(string(letter), n))
	}
	return b.String()
}

// quote quotes text that holds letters or apostrophes for a pattern.
func quote(s string) string {
	if !strings.ContainsFunc(s, func(r rune) bool {
		return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || r == '\''
	}) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
