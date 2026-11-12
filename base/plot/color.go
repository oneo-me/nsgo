package plot

import (
	"math"
	"slices"

	"github.com/nsgo-dev/nsgo/core"
)

// colorKind is what a ColorScale colors.
type colorKind uint8

const (
	colorOrdinal colorKind = iota
	colorSequential
	colorDiverging
)

// ColorScale is the colors of a Plot's marks: a color per category, or the
// colors of numbers along a ramp, as d3's scaleOrdinal, scaleSequential and
// scaleDiverging. A mark's Color names each row's category; a Cell's Value
// and a Treemap's ColorValue are the numbers of a ramp. The zero value
// colors nothing.
type ColorScale struct {
	kind       colorKind
	categories []string
	colors     []core.Color
	unknown    core.Color
	domain     [2]float64
	hasDomain  bool
	pivot      float64
}

// OrdinalColors gives domain's categories colors in order, repeating them
// over a longer domain. A category outside the domain takes Unknown's
// color; without one it joins the domain the first time a mark meets it
// (marks in order, rows in order) and takes the next color, as d3's
// implicit domain.
func OrdinalColors(domain []string, colors ...core.Color) ColorScale {
	return ColorScale{kind: colorOrdinal, categories: unique(domain), colors: colors}
}

// SequentialColors places numbers along a ramp of colors, from the low end
// of the domain to the high one, the colors spread evenly and mixed in
// OKLab; values outside the domain take its ends.
func SequentialColors(colors ...core.Color) ColorScale {
	return ColorScale{kind: colorSequential, colors: colors}
}

// DivergingColors is SequentialColors about a pivot, 0 until Pivot says
// otherwise: the values below it take the ramp's first half and those above
// its second, so the pivot takes the middle color. Without a domain it is
// the values' extent made even on both sides of the pivot.
func DivergingColors(colors ...core.Color) ColorScale {
	return ColorScale{kind: colorDiverging, colors: colors}
}

// Over returns s with the ramp's ends at lo and hi, in place of the
// extent of the values it colors.
func (s ColorScale) Over(lo, hi float64) ColorScale {
	s.domain, s.hasDomain = [2]float64{min(lo, hi), max(lo, hi)}, true
	return s
}

// Pivot returns s with its ramp's middle at p.
func (s ColorScale) Pivot(p float64) ColorScale {
	s.pivot = p
	return s
}

// Unknown returns s with c for a category outside its domain or a row
// without a number.
func (s ColorScale) Unknown(c core.Color) ColorScale {
	s.unknown = c
	return s
}

func (s ColorScale) equal(o ColorScale) bool {
	return s.kind == o.kind && s.unknown == o.unknown && s.domain == o.domain && s.hasDomain == o.hasDomain &&
		s.pivot == o.pivot && slices.Equal(s.categories, o.categories) && slices.Equal(s.colors, o.colors)
}

// colorOf is the color of category, 0 for none: its place in the domain
// and then among met, the categories outside the domain met so far, where
// it goes when new; unknown instead when set. An empty category has none.
func (s ColorScale) colorOf(met *[]string, category string) core.Color {
	if category == "" || len(s.colors) == 0 {
		return 0
	}
	at := func(i int) core.Color { return s.colors[i%len(s.colors)] }
	if i := slices.Index(s.categories, category); i >= 0 {
		return at(i)
	}
	if s.unknown != 0 {
		return s.unknown
	}
	i := slices.Index(*met, category)
	if i < 0 {
		*met = append(*met, category)
		i = len(*met) - 1
	}
	return at(len(s.categories) + i)
}

// spread is the domain numbers are spread over, given the extent of the
// values it colors.
func (s ColorScale) spread(extent plotExtent) ([2]float64, bool) {
	switch {
	case s.kind == colorOrdinal:
		return [2]float64{}, false
	case s.hasDomain:
		return s.domain, true
	case !extent.ok:
		return [2]float64{}, false
	case s.kind == colorDiverging:
		r := max(math.Abs(extent.lo-s.pivot), math.Abs(extent.hi-s.pivot))
		return [2]float64{s.pivot - r, s.pivot + r}, true
	}
	return extent.ends(), true
}

// shade is the color of value over domain, from spread: where it lies
// along the ramp. unknown for no value.
func (s ColorScale) shade(value float64, domain [2]float64) core.Color {
	if s.kind == colorOrdinal {
		return 0
	}
	if !isFinite(value) {
		return s.unknown
	}
	lo, hi := domain[0], domain[1]
	var t float64
	if s.kind == colorDiverging {
		side := func(from, to float64) float64 {
			if span := to - from; span != 0 && isFinite(span) {
				return (value - from) / span
			}
			return 0
		}
		if value < s.pivot {
			t = (1 - side(s.pivot, lo)) * 0.5
		} else {
			t = 0.5 + side(s.pivot, hi)*0.5
		}
	} else if span := hi - lo; span != 0 && isFinite(span) {
		t = (value - lo) / span
	} else {
		t = 0.5
	}
	return ramp(s.colors, float32(min(max(t, 0), 1)))
}

// ramp is the color at t, 0 to 1, along colors, spread evenly over it and
// mixed in OKLab, which is perceptually even with no gray dip between two
// hues.
func ramp(colors []core.Color, t float32) core.Color {
	switch len(colors) {
	case 0:
		return 0
	case 1:
		return colors[0]
	}
	at := t * float32(len(colors)-1)
	i := min(int(at), len(colors)-2)
	// On a stop, its color as the app wrote it.
	switch f := at - float32(i); {
	case f <= 0:
		return colors[i]
	case f >= 1:
		return colors[i+1]
	default:
		return mixOKLab(colors[i], colors[i+1], f)
	}
}

// oklab is a color in OKLab (Björn Ottosson's matrices), with its alpha.
type oklab struct{ l, a, b, alpha float64 }

func toOKLab(c core.Color) oklab {
	r, g, b, alpha := c.Components()
	linear := func(c float64) float64 {
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	r, g, b = linear(r), linear(g), linear(b)
	l := math.Cbrt(0.41222147*r + 0.53633255*g + 0.051445995*b)
	m := math.Cbrt(0.2119035*r + 0.6806995*g + 0.10739696*b)
	s := math.Cbrt(0.08830246*r + 0.28171885*g + 0.6299787*b)
	return oklab{0.21045426*l + 0.7936178*m - 0.004072047*s, 1.9779985*l - 2.4285922*m + 0.4505937*s,
		0.025904037*l + 0.78277177*m - 0.80867577*s, alpha}
}

func (c oklab) color() core.Color {
	l := c.l + 0.39633778*c.a + 0.21580376*c.b
	m := c.l - 0.105561346*c.a - 0.06385417*c.b
	s := c.l - 0.08948418*c.a - 1.2914855*c.b
	l, m, s = l*l*l, m*m*m, s*s*s
	r := 4.0767417*l - 3.3077116*m + 0.23096994*s
	g := -1.268438*l + 2.6097574*m - 0.34131938*s
	b := -0.0041960863*l - 0.7034186*m + 1.7076147*s
	channel := func(c float64) uint32 {
		if c <= 0.0031308 {
			c *= 12.92
		} else {
			c = 1.055*math.Pow(c, 1/2.4) - 0.055
		}
		return uint32(min(max(c, 0), 1)*255 + 0.5)
	}
	return core.Color(channel(r)<<24 | channel(g)<<16 | channel(b)<<8 | uint32(min(max(c.alpha, 0), 1)*255+0.5))
}

// mixOKLab is the color t of the way from a to b.
func mixOKLab(a, b core.Color, t float32) core.Color {
	p, q, f := toOKLab(a), toOKLab(b), float64(t)
	return oklab{p.l + (q.l-p.l)*f, p.a + (q.a-p.a)*f, p.b + (q.b-p.b)*f, p.alpha + (q.alpha-p.alpha)*f}.color()
}

// isDark reports whether light text reads better on c.
func isDark(c core.Color) bool { return toOKLab(c).l < 0.65 }
