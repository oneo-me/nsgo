package tsx

import (
	"math"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"
	"golang.org/x/text/collate"
	"golang.org/x/text/currency"
	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

// Intl (NSH's web/intl): numbers and plural rules by CLDR's data through
// golang.org/x/text, strings compared by its collation, and dates in
// English names. Number's, Date's and String's locale methods use them.

// locale is the locale an Intl call names, else the environment's.
func (r *runtime) locale(v js.Value) language.Tag {
	name := ""
	if items := r.arrayItems(v); len(items) > 0 {
		name = items[0].String()
	} else if defined(v) {
		name = v.String()
	}
	if name == "" {
		name = r.envLocale()
	}
	tag, err := language.Parse(name)
	if err != nil {
		return language.AmericanEnglish
	}
	return tag
}

func (r *runtime) envLocale() string {
	var l string
	r.cx.Untrack(func() { l = r.cx.Env().Locale() })
	if l == "" {
		return "en-US"
	}
	return l
}

// formatNumber formats x as Intl.NumberFormat's options say.
func (r *runtime) formatNumber(x float64, tag language.Tag, o opts) string {
	if math.IsNaN(x) {
		return "NaN"
	}
	if math.IsInf(x, 0) {
		if x > 0 {
			return "∞"
		}
		return "-∞"
	}
	p := message.NewPrinter(tag)
	style := o.str("style", "decimal")
	minFrac := int(o.float("minimumFractionDigits", 0))
	maxFrac := int(o.float("maximumFractionDigits", -1))
	if maxFrac < 0 {
		switch style {
		case "currency":
			maxFrac = max(2, minFrac)
			if !o.has("minimumFractionDigits") {
				minFrac = 2
			}
		case "percent":
			maxFrac = max(0, minFrac)
		default:
			maxFrac = max(3, minFrac)
		}
	}
	if o.str("notation", "standard") == "compact" {
		return r.compact(x, p, o)
	}
	value := x
	if style == "percent" {
		value *= 100
	}
	var numOpts []number.Option
	numOpts = append(numOpts, number.MinFractionDigits(minFrac), number.MaxFractionDigits(maxFrac))
	if v := o.get("useGrouping"); defined(v) && !v.ToBoolean() {
		numOpts = append(numOpts, number.NoSeparator())
	}
	if mi := int(o.float("minimumIntegerDigits", 0)); mi > 0 {
		numOpts = append(numOpts, number.MinIntegerDigits(mi))
	}
	sign := ""
	switch o.str("signDisplay", "auto") {
	case "always":
		if value >= 0 {
			sign = "+"
		}
	case "exceptZero":
		if value > 0 {
			sign = "+"
		}
	case "never":
		value = math.Abs(value)
	}
	s := p.Sprint(number.Decimal(value, numOpts...))
	switch style {
	case "percent":
		s += "%"
	case "currency":
		unit, err := currency.ParseISO(o.str("currency", "USD"))
		if err != nil {
			return sign + s
		}
		sym := p.Sprint(currency.Symbol(unit))
		if strings.HasPrefix(s, "-") {
			return "-" + sym + s[1:]
		}
		return sign + sym + s
	case "unit":
		return sign + s + " " + o.str("unit", "")
	}
	return sign + s
}

// compact formats x as 1.2K, 34M.
func (r *runtime) compact(x float64, p *message.Printer, o opts) string {
	abs := math.Abs(x)
	for _, u := range []struct {
		div    float64
		suffix string
	}{{1e12, "T"}, {1e9, "B"}, {1e6, "M"}, {1e3, "K"}} {
		if abs >= u.div {
			v := x / u.div
			// Two significant digits, as browsers write them.
			frac := 0
			if math.Abs(v) < 10 {
				frac = 1
			}
			return p.Sprint(number.Decimal(v, number.MaxFractionDigits(frac))) + u.suffix
		}
	}
	return p.Sprint(number.Decimal(x, number.MaxFractionDigits(0)))
}

var months = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September",
	"October", "November", "December"}

var weekdays = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// formatDate formats t as Intl.DateTimeFormat's options say, in English.
func (r *runtime) formatDate(t time.Time, o opts, kind string) string {
	if tz := o.str("timeZone", ""); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			t = t.In(loc)
		}
	}
	keys := []string{"weekday", "year", "month", "day", "hour", "minute", "second"}
	any := false
	for _, k := range keys {
		any = any || o.has(k)
	}
	if ds := o.str("dateStyle", ""); ds != "" {
		any = true
		switch ds {
		case "full":
			o = r.withOpts(o, "weekday", "long", "year", "numeric", "month", "long", "day", "numeric")
		case "long":
			o = r.withOpts(o, "year", "numeric", "month", "long", "day", "numeric")
		case "medium":
			o = r.withOpts(o, "year", "numeric", "month", "short", "day", "numeric")
		default:
			o = r.withOpts(o, "year", "2-digit", "month", "numeric", "day", "numeric")
		}
	}
	if ts := o.str("timeStyle", ""); ts != "" {
		any = true
		o = r.withOpts(o, "hour", "numeric", "minute", "2-digit")
		if ts != "short" {
			o = r.withOpts(o, "second", "2-digit")
		}
	}
	if !any {
		switch kind {
		case "date":
			o = r.withOpts(o, "year", "numeric", "month", "numeric", "day", "numeric")
		case "time":
			o = r.withOpts(o, "hour", "numeric", "minute", "2-digit", "second", "2-digit")
		default:
			o = r.withOpts(o, "year", "numeric", "month", "numeric", "day", "numeric", "hour", "numeric",
				"minute", "2-digit", "second", "2-digit")
		}
	}
	two := func(n int, style string) string {
		if style == "2-digit" && n < 10 {
			return "0" + itoa(n)
		}
		return itoa(n)
	}
	var date []string
	month := o.str("month", "")
	switch month {
	case "long", "short", "narrow":
		name := months[t.Month()-1]
		if month == "short" {
			name = name[:3]
		} else if month == "narrow" {
			name = name[:1]
		}
		d := name
		if o.has("day") {
			d += " " + two(t.Day(), o.str("day", ""))
		}
		if o.has("year") {
			d += ", " + yearOf(t, o.str("year", ""))
		}
		date = append(date, d)
	case "":
		if o.has("day") {
			date = append(date, two(t.Day(), o.str("day", "")))
		}
		if o.has("year") {
			date = append(date, yearOf(t, o.str("year", "")))
		}
	default:
		parts := []string{two(int(t.Month()), month)}
		if o.has("day") {
			parts = append(parts, two(t.Day(), o.str("day", "")))
		}
		if o.has("year") {
			parts = append(parts, yearOf(t, o.str("year", "")))
		}
		date = append(date, strings.Join(parts, "/"))
	}
	out := strings.Join(date, " ")
	if w := o.str("weekday", ""); w != "" {
		name := weekdays[t.Weekday()]
		if w != "long" {
			name = name[:3]
		}
		if out != "" {
			out = name + ", " + out
		} else {
			out = name
		}
	}
	if o.has("hour") {
		h12 := o.bool("hour12", true)
		h := t.Hour()
		suffix := ""
		if h12 {
			suffix = " AM"
			if h >= 12 {
				suffix = " PM"
			}
			h %= 12
			if h == 0 {
				h = 12
			}
		}
		tm := two(h, o.str("hour", ""))
		if !h12 && h < 10 {
			tm = "0" + itoa(h)
		}
		if o.has("minute") {
			tm += ":" + two(t.Minute(), "2-digit")
		}
		if o.has("second") {
			tm += ":" + two(t.Second(), "2-digit")
		}
		tm += suffix
		if out != "" {
			out += ", " + tm
		} else {
			out = tm
		}
	}
	return out
}

func yearOf(t time.Time, style string) string {
	if style == "2-digit" {
		y := t.Year() % 100
		if y < 10 {
			return "0" + itoa(y)
		}
		return itoa(y)
	}
	return itoa(t.Year())
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// withOpts lays key-value pairs over o's options.
func (r *runtime) withOpts(o opts, kv ...string) opts {
	m := map[string]string{}
	if o.o != nil {
		for _, k := range o.o.Keys() {
			m[k] = o.o.Get(k).String()
		}
	}
	for i := 0; i+1 < len(kv); i += 2 {
		if _, ok := m[kv[i]]; !ok {
			m[kv[i]] = kv[i+1]
		}
	}
	out := r.vm.NewObject()
	for k, v := range m {
		switch v {
		case "true":
			_ = out.Set(k, true)
		case "false":
			_ = out.Set(k, false)
		default:
			_ = out.Set(k, v)
		}
	}
	return opts{out}
}

func (r *runtime) intlNatives() *js.Object {
	n := r.vm.NewObject()
	set := func(name string, f func(call js.FunctionCall) js.Value) { _ = n.Set(name, r.function(f)) }
	set("number", func(call js.FunctionCall) js.Value {
		return r.vm.ToValue(r.formatNumber(arg(call, 0).ToFloat(), r.locale(arg(call, 1)), r.options(arg(call, 2))))
	})
	set("date", func(call js.FunctionCall) js.Value {
		ms := arg(call, 0).ToFloat()
		t := time.UnixMilli(int64(ms)).In(time.Local)
		return r.vm.ToValue(r.formatDate(t, r.options(arg(call, 2)), stringOf(arg(call, 3))))
	})
	set("plural", func(call js.FunctionCall) js.Value {
		tag, x := r.locale(arg(call, 1)), arg(call, 0).ToFloat()
		rules := plural.Cardinal
		if r.options(arg(call, 2)).str("type", "cardinal") == "ordinal" {
			rules = plural.Ordinal
		}
		i := int(math.Abs(x))
		frac := 0
		if x != math.Trunc(x) {
			frac = 1
		}
		form, ok := map[plural.Form]string{plural.Zero: "zero", plural.One: "one", plural.Two: "two",
			plural.Few: "few", plural.Many: "many"}[rules.MatchPlural(tag, i, frac, frac, 0, 0)]
		if !ok {
			form = "other"
		}
		return r.vm.ToValue(form)
	})
	set("compare", func(call js.FunctionCall) js.Value {
		o := r.options(arg(call, 3))
		var copts []collate.Option
		if o.str("sensitivity", "") == "base" || o.str("sensitivity", "") == "accent" {
			copts = append(copts, collate.IgnoreCase)
		}
		if o.bool("numeric", false) {
			copts = append(copts, collate.Numeric)
		}
		c := collate.New(r.locale(arg(call, 2)), copts...)
		return r.vm.ToValue(c.CompareString(arg(call, 0).String(), arg(call, 1).String()))
	})
	set("locale", func(call js.FunctionCall) js.Value { return r.vm.ToValue(r.locale(arg(call, 0)).String()) })
	return n
}
