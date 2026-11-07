package base

import (
	"math"
	"strconv"
	"strings"
)

// The rules of a number in a range with a step, as NSH's numeric.rs, which
// Slider and NumberInput follow: where a value snaps to, how far a key
// moves it, where it sits along a control and how it reads as text, and
// the rules of a number typed as text.

// numberScale is how values spread along a control.
type numberScale uint8

const (
	linearScale numberScale = iota
	logScale
)

const (
	// pageShare is the share of a range a page key moves a value by.
	pageShare = 0.1
	// freeStep is the share of a range a key moves a value by with no
	// step.
	freeStep = 0.01
)

// numberRange is a range made valid whatever it was given: a non-finite
// min is 0, a max below min is min, a step that is not positive is none,
// and a logarithmic scale needs min > 0, else it is linear.
type numberRange struct {
	min, max, step float64
	scale          numberScale
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func newNumberRange(lo, hi, step float64, scale numberScale) numberRange {
	if !isFinite(lo) {
		lo = 0
	}
	if !isFinite(hi) || hi < lo {
		hi = lo
	}
	if !isFinite(step) || step <= 0 {
		step = 0
	}
	if scale == logScale && !(lo > 0 && hi > lo) {
		scale = linearScale
	}
	return numberRange{lo, hi, step, scale}
}

// clamp is v held to the range; NaN is min.
func (r numberRange) clamp(v float64) float64 {
	if math.IsNaN(v) {
		return r.min
	}
	return math.Min(math.Max(v, r.min), r.max)
}

// snap is the value nearest v the range takes: min, min plus whole steps,
// or max, so the last step may be shorter. Steps count from min, and the
// result carries no more decimals than the step and min.
func (r numberRange) snap(v float64) float64 {
	v = r.clamp(v)
	if r.step == 0 {
		return v
	}
	lo := r.min + math.Floor((v-r.min)/r.step)*r.step
	hi := math.Min(lo+r.step, r.max)
	nearest := hi
	if v-lo < hi-v {
		nearest = lo
	}
	return r.clamp(r.round(nearest))
}

func (r numberRange) round(v float64) float64 {
	scale := math.Pow(10, float64(max(decimals(r.step), decimals(r.min))))
	return math.Round(v*scale) / scale
}

// fraction is where v sits along the range, from 0 at min to 1 at max.
func (r numberRange) fraction(v float64) float64 {
	v = r.clamp(v)
	if r.max <= r.min {
		return 0
	}
	f := (v - r.min) / (r.max - r.min)
	if r.scale == logScale {
		f = math.Log(v/r.min) / math.Log(r.max/r.min)
	}
	return math.Min(math.Max(f, 0), 1)
}

// valueAt is the value at fraction of the way along the range.
func (r numberRange) valueAt(fraction float64) float64 {
	if math.IsNaN(fraction) {
		fraction = 0
	}
	f := math.Min(math.Max(fraction, 0), 1)
	if r.scale == logScale {
		return r.clamp(r.min * math.Pow(r.max/r.min, f))
	}
	return r.clamp(r.min + (r.max-r.min)*f)
}

// stepBy is v moved by steps and snapped: an arrow key, a screen reader's
// increment. From a value off the grid the first step lands on it, in its
// direction.
func (r numberRange) stepBy(v, steps float64) float64 {
	step := r.step
	if step == 0 {
		step = (r.max - r.min) * freeStep
	}
	from := r.clamp(v)
	next := from + steps*step
	if r.step == 0 {
		return r.clamp(next)
	}
	grid := r.snap(from)
	if math.Abs(grid-from) > step*1e-9 && (steps > 0 && grid > from || steps < 0 && grid < from) {
		return grid
	}
	return r.snap(next)
}

// page is v moved a tenth of the way along the range, on its scale, by at
// least a step: Page Up and Page Down.
func (r numberRange) page(v, direction float64) float64 {
	from := r.clamp(v)
	next := r.snap(r.valueAt(r.fraction(from) + direction*pageShare))
	if next == r.snap(from) {
		return r.stepBy(from, direction)
	}
	return next
}

// format is v as text with the decimals of the step and min and no
// trailing zeros: what screen readers say.
func (r numberRange) format(v float64) string {
	text := strconv.FormatFloat(v, 'f', max(decimals(r.step), decimals(r.min)), 64)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	}
	if text == "-0" {
		return "0"
	}
	return text
}

// decimals is how many decimals x has, at most 10.
func decimals(x float64) int {
	if !isFinite(x) {
		return 0
	}
	v, d := math.Abs(x), 0
	for d < 10 && math.Abs(v-math.Round(v)) > 1e-9*math.Max(v, 1) {
		v *= 10
		d++
	}
	return d
}

// normalizeNumberText makes a number's other spellings ASCII: full-width
// digits, signs and dot, the minus sign and the ideographic full stop, and
// reads a lone comma, what many keyboards' decimal key types, as the
// point while the text has none.
func normalizeNumberText(text string) string {
	ascii := strings.Map(func(c rune) rune {
		switch {
		case c >= '０' && c <= '９':
			return c - '０' + '0'
		case c == '＋':
			return '+'
		case c == '－' || c == '−':
			return '-'
		case c == '．' || c == '。':
			return '.'
		case c == '，':
			return ','
		}
		return c
	}, text)
	if !strings.Contains(ascii, ".") && strings.Count(ascii, ",") == 1 {
		return strings.Replace(ascii, ",", ".", 1)
	}
	return ascii
}

// isNumberText reports whether text is a number or on the way to one:
// digits with at most one point and one sign, the sign first.
func isNumberText(text string) bool {
	whole, fraction, dotted := strings.Cut(text, ".")
	if dotted && strings.Contains(fraction, ".") {
		return false
	}
	digits := func(s string) bool {
		return strings.IndexFunc(s, func(c rune) bool { return c < '0' || c > '9' }) < 0
	}
	return digits(strings.TrimLeft(whole[:min(len(whole), 1)], "+-")+whole[min(len(whole), 1):]) && digits(fraction)
}

// parseNumber is the number text reads as, ok false for the empty text
// and those on the way to one (-, .).
func parseNumber(text string) (float64, bool) {
	text = strings.TrimSpace(text)
	if !isNumberText(text) || strings.Trim(text, "+-.") == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(text, "."), 64)
	if err != nil || !isFinite(v) {
		return 0, false
	}
	return v, true
}

// numberText is v as a number input shows a value it did not type: the
// shortest text that reads back as v.
func numberText(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// timesStep is step times n, with the step's decimals and no float noise.
func timesStep(step, n float64) float64 {
	scale := math.Pow(10, float64(decimals(step)))
	return math.Round(step*scale*n) / scale
}

// clampTo is v held to lo and hi, either of which may be nil.
func clampTo(v float64, lo, hi *float64) float64 {
	if lo != nil {
		v = math.Max(v, *lo)
	}
	if hi != nil {
		v = math.Min(v, *hi)
	}
	return v
}

// fractionDigits is how many digits follow text's point.
func fractionDigits(text string) int {
	_, f, _ := strings.Cut(text, ".")
	return len(f)
}

// stepText is the text a step up or down makes of text, held to lo and hi:
// an empty text, or one on the way to a number, counts as 0, and the result
// keeps the decimals of the text, the step and a bound it was held to. ok
// is false when the step would not move the value the way it goes.
func stepText(text string, up bool, step float64, lo, hi *float64) (string, bool) {
	current, known := parseNumber(text)
	next := current - step
	if up {
		next = current + step
	}
	short := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	digits := max(fractionDigits(strings.TrimSpace(text)), fractionDigits(short(step)))
	if lo != nil && next < *lo {
		next, digits = *lo, max(digits, fractionDigits(short(*lo)))
	}
	if hi != nil && next > *hi {
		next, digits = *hi, max(digits, fractionDigits(short(*hi)))
	}
	if known && (up && next <= current || !up && next >= current) {
		return "", false
	}
	out := strconv.FormatFloat(next, 'f', digits, 64)
	if zero, ok := strings.CutPrefix(out, "-"); ok && strings.Trim(zero, "0.") == "" {
		return zero, true
	}
	return out, true
}
