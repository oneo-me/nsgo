package cldr

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// Style is the length of a date or a time as CLDR's patterns write it.
type Style uint8

const (
	NoStyle Style = iota
	ShortStyle
	MediumStyle
	LongStyle
	FullStyle
)

var styleNames = [...]string{"", "short", "medium", "long", "full"}

// DateOptions are how FormatDate writes a time: its date, its time or
// both by style, or else the fields of Skeleton, CLDR's letters in any
// order ("yMMMd", "jm"), in the order and form the locale writes them.
// Neither is "yMd".
type DateOptions struct {
	Date, Time Style
	Skeleton   string
}

type formatterKey struct {
	locale string
	o      DateOptions
}

var formatters sync.Map // formatterKey → *formatter

// formatter writes times by one pattern in one locale.
type formatter struct {
	data  localeData
	lang  string
	items []item
}

// FormatDate writes t, in its location, in the Gregorian calendar, in the
// way of locale: its patterns, names and digits. Time zones are written as
// offsets from GMT in the locale's format.
func FormatDate(locale string, t time.Time, o DateOptions) string {
	key := formatterKey{locale, o}
	f, ok := formatters.Load(key)
	if !ok {
		i := find(locale)
		d := dataAt(i)
		lang, _, _ := strings.Cut(tables().names[i], "-")
		f, _ = formatters.LoadOrStore(key, &formatter{data: d, lang: lang, items: parsePattern(pattern(d, o))})
	}
	return f.(*formatter).format(t)
}

// Pattern returns the CLDR pattern FormatDate writes for locale and o, as
// "MMM d, y".
func Pattern(locale string, o DateOptions) string { return pattern(dataAt(find(locale)), o) }

// pattern is the pattern of o in d.
func pattern(d localeData, o DateOptions) string {
	switch {
	case o.Date != NoStyle && o.Time != NoStyle:
		glue := d["AT."+styleNames[o.Date]]
		if glue == "" {
			glue = d["DT."+styleNames[o.Date]]
		}
		return strings.NewReplacer("{1}", d["D."+styleNames[o.Date]], "{0}", d["T."+styleNames[o.Time]]).Replace(glue)
	case o.Date != NoStyle:
		return d["D."+styleNames[o.Date]]
	case o.Time != NoStyle:
		return d["T."+styleNames[o.Time]]
	}
	skel := o.Skeleton
	if skel == "" {
		skel = "yMd"
	}
	return bestPattern(d, skel)
}

// item is a pattern's literal text, or a field: a letter repeated n times.
type item struct {
	text   string
	letter byte
	n      int
}

// parsePattern splits a CLDR date pattern into its fields and text; text
// in apostrophes is literal, and two apostrophes are one.
func parsePattern(p string) []item {
	var items []item
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			items = append(items, item{text: text.String()})
			text.Reset()
		}
	}
	for i := 0; i < len(p); {
		c := p[i]
		switch {
		case c == '\'':
			if i+1 < len(p) && p[i+1] == '\'' {
				text.WriteByte('\'')
				i += 2
				continue
			}
			end := strings.IndexByte(p[i+1:], '\'')
			if end < 0 {
				end = len(p) - i - 1
			}
			text.WriteString(strings.ReplaceAll(p[i+1:i+1+end], "''", "'"))
			i += end + 2
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
			flush()
			n := 1
			for i+n < len(p) && p[i+n] == c {
				n++
			}
			items = append(items, item{letter: c, n: n})
			i += n
		default:
			text.WriteByte(c)
			i++
		}
	}
	flush()
	return items
}

func (f *formatter) format(t time.Time) string {
	var b strings.Builder
	for _, it := range f.items {
		if it.letter == 0 {
			b.WriteString(it.text)
		} else {
			f.field(&b, it.letter, it.n, t)
		}
	}
	// Browsers write CLDR's narrow no-break and thin spaces (1:07 PM) as
	// spaces, which pages expect; so does this.
	return spaces.Replace(b.String())
}

var spaces = strings.NewReplacer("\u202f", " ", "\u2009", " ")

// num writes v padded with zeros to n digits, in the locale's digits.
func (f *formatter) num(b *strings.Builder, v, n int) {
	s := strconv.Itoa(v)
	for len(s) < n {
		s = "0" + s
	}
	digits := []rune(f.data["N"])
	if len(digits) != 10 || digits[0] == '0' {
		b.WriteString(s)
		return
	}
	for _, c := range s {
		if '0' <= c && c <= '9' {
			b.WriteRune(digits[c-'0'])
		} else {
			b.WriteRune(c)
		}
	}
}

// name writes the name under key, with the widths to try after it when
// the locale has none of that width.
func (f *formatter) name(b *strings.Builder, keys ...string) {
	for _, k := range keys {
		if v, ok := f.data[k]; ok {
			b.WriteString(v)
			return
		}
	}
}

// width is a text field's width by its letter count: 3 abbreviated
// (1 to 3), 4 wide, 5 narrow, 6 short.
func width(n int) string {
	switch {
	case n <= 3:
		return "3"
	case n >= 6:
		return "6"
	}
	return strconv.Itoa(n)
}

func (f *formatter) field(b *strings.Builder, c byte, n int, t time.Time) {
	switch c {
	case 'G':
		era := "1"
		if t.Year() <= 0 {
			era = "0"
		}
		f.name(b, "G"+width(n)+"."+era, "G3."+era)
	case 'y', 'Y', 'u':
		y := t.Year()
		if c == 'Y' {
			y, _ = t.ISOWeek()
		}
		if n == 2 {
			f.num(b, y%100, 2)
		} else {
			f.num(b, y, n)
		}
	case 'M', 'L':
		m := strconv.Itoa(int(t.Month()))
		if n <= 2 {
			f.num(b, int(t.Month()), n)
			return
		}
		ctx, other := "f", "s"
		if c == 'L' {
			ctx, other = "s", "f"
		}
		w := width(n)
		f.name(b, "M"+ctx+w+"."+m, "M"+other+w+"."+m, "Mf3."+m)
	case 'd':
		f.num(b, t.Day(), n)
	case 'D':
		f.num(b, t.YearDay(), n)
	case 'E', 'c', 'e':
		if c != 'E' && n <= 2 {
			f.num(b, int(t.Weekday())+1, n)
			return
		}
		ctx, other := "f", "s"
		if c == 'c' {
			ctx, other = "s", "f"
		}
		w, d := width(n), strconv.Itoa(int(t.Weekday()))
		f.name(b, "E"+ctx+w+"."+d, "E"+other+w+"."+d, "Ef3."+d)
	case 'a', 'b':
		w := width(n)
		if c == 'b' && t.Minute() == 0 && t.Second() == 0 {
			switch t.Hour() {
			case 0:
				if _, ok := f.data["a"+w+".midnight"]; ok {
					f.name(b, "a"+w+".midnight")
					return
				}
			case 12:
				if _, ok := f.data["a"+w+".noon"]; ok {
					f.name(b, "a"+w+".noon")
					return
				}
			}
		}
		f.ampm(b, w, t)
	case 'B':
		w := width(n)
		if p := f.dayPeriod(t); p != "" {
			if _, ok := f.data["a"+w+"."+p]; ok {
				f.name(b, "a"+w+"."+p)
				return
			}
		}
		f.ampm(b, w, t)
	case 'h':
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		f.num(b, h, n)
	case 'H':
		f.num(b, t.Hour(), n)
	case 'K':
		f.num(b, t.Hour()%12, n)
	case 'k':
		h := t.Hour()
		if h == 0 {
			h = 24
		}
		f.num(b, h, n)
	case 'm':
		f.num(b, t.Minute(), n)
	case 's':
		f.num(b, t.Second(), n)
	case 'S':
		frac := strconv.Itoa(t.Nanosecond() + 1e9)[1:]
		for len(frac) < n {
			frac += "0"
		}
		v, _ := strconv.Atoi(frac[:n])
		f.num(b, v, n)
	case 'z', 'O', 'v', 'V', 'Z', 'X', 'x':
		short := (c == 'z' && n < 4) || (c == 'O' && n < 4) || c == 'v' && n < 4
		f.zone(b, t, short)
	}
}

func (f *formatter) ampm(b *strings.Builder, w string, t time.Time) {
	p := "am"
	if t.Hour() >= 12 {
		p = "pm"
	}
	f.name(b, "a"+w+"."+p, "a3."+p)
}

// dayPeriod returns the flexible day period of t in the locale's
// language by CLDR's rules, as "morning1", or "".
func (f *formatter) dayPeriod(t time.Time) string {
	minute := t.Hour()*60 + t.Minute()
	clock := func(s string) int {
		h, m, _ := strings.Cut(s, ":")
		hh, _ := strconv.Atoi(h)
		mm, _ := strconv.Atoi(m)
		return hh*60 + mm
	}
	for _, line := range strings.Split(periodRules[f.lang], "\n") {
		parts := strings.Fields(line)
		if len(parts) != 3 {
			continue
		}
		from, before := clock(parts[1]), clock(parts[2])
		if from < before && from <= minute && minute < before ||
			from > before && (minute >= from || minute < before) {
			return parts[0]
		}
	}
	return ""
}

// zone writes t's offset from UTC in the locale's GMT format: short
// ("GMT+8", "GMT+5:30") or long ("GMT+08:00"), and the zero offset as the
// locale writes it ("GMT").
func (f *formatter) zone(b *strings.Builder, t time.Time, short bool) {
	_, off := t.Zone()
	if off == 0 {
		b.WriteString(f.data["Zz"])
		return
	}
	pos, neg, _ := strings.Cut(f.data["Zh"], ";")
	p := pos
	if off < 0 {
		p, off = neg, -off
	}
	h, m := off/3600, off/60%60
	if short {
		p = strings.Replace(p, "HH", "H", 1)
		if m == 0 {
			if i := strings.Index(p, "mm"); i > 0 {
				p = p[:i-1] + p[i+2:]
			}
		}
	}
	var v strings.Builder
	for _, it := range parsePattern(p) {
		switch it.letter {
		case 0:
			v.WriteString(it.text)
		case 'H':
			f.num(&v, h, it.n)
		case 'm':
			f.num(&v, m, it.n)
		}
	}
	b.WriteString(strings.Replace(f.data["Zg"], "{0}", v.String(), 1))
}
