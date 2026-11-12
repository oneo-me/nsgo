package plot

import (
	"math"
	"slices"

	"github.com/nsgo-dev/nsgo/core"
)

// scaleKind is what a Scale places.
type scaleKind uint8

const (
	scalePoint scaleKind = iota
	scaleLinear
	scaleBand
)

// Scale is how an axis of a Plot places values, as gpui-base's scales and
// d3-scale's do: a point scale places the rows in order, evenly from one
// edge to the other (one row in the middle), a linear scale places a
// number in proportion to where it lies in its domain, and a band scale
// gives each row, or each category of its domain, a band of the axis for
// the bars and cells drawn in it. The zero value is a point scale.
type Scale struct {
	kind      scaleKind
	domain    [2]float64
	hasDomain bool
	reverse   bool
	band      Band
}

// PointScale places the rows in order.
func PointScale() Scale { return Scale{} }

// LinearScale places values in proportion over the extent of domain's two
// values, in either order; without them, over the extent of what the plot
// draws.
func LinearScale(domain ...float64) Scale {
	s := Scale{kind: scaleLinear}
	if len(domain) == 2 && isFinite(domain[0]) && isFinite(domain[1]) {
		s.domain, s.hasDomain = [2]float64{domain[0], domain[1]}, true
	}
	return s
}

// BandScale gives each row a band of the axis, or each category of band's
// Domain.
func BandScale(band Band) Scale { return Scale{kind: scaleBand, band: band} }

// Reversed returns s running from its other edge: the first row or the
// domain's smallest value at the right edge of the x axis, and on the y
// axis the domain's smallest value at the top edge (bars grow down from
// it) and the first row at the bottom edge.
func (s Scale) Reversed() Scale {
	s.reverse = true
	return s
}

// Band says how a band scale lays its bands out, as d3's scaleBand: gaps
// are shares of a band's step.
type Band struct {
	// Domain is the categories its bands stand for, in order: a Cell goes
	// in the band of its row's category and an Axis labels each band with
	// its category; one written twice keeps its first place. Without one,
	// the categories the plot's cells read along the axis, in the order
	// met, and without cells a band per row.
	Domain []string
	// PaddingInner is the gap between neighbouring bands, 0 to 1, and
	// PaddingOuter the gap before the first and after the last.
	PaddingInner, PaddingOuter float32
	// MaxBandWidth is the widest a band may be: a few bands across a wide
	// plot stay narrow, each in the middle of its place. 0 sets none.
	MaxBandWidth core.Points
	// BandCount is the bands the axis is laid out for when more than the
	// rows: the rows take the leading ones and keep their width and place
	// as rows come.
	BandCount int
	// GroupPadding is the gap between the bars side by side in a band, a
	// share of each bar's place.
	GroupPadding float32
}

// ValueScale is the scale along the y axis a mark places its values on.
type ValueScale uint8

const (
	// ScaleY is the plot's YScale.
	ScaleY ValueScale = iota
	// ScaleY2 is its second one, Y2Scale: a dual-axis chart's right-hand
	// scale.
	ScaleY2
	// noScale names no y scale (traced).
	noScale
)

// axis is an axis of a plot's scales.
type axis uint8

const (
	axisX axis = iota
	axisY
	axisY2
)

func axisOf(s ValueScale) axis {
	if s == ScaleY2 {
		return axisY2
	}
	return axisY
}

// linearMap maps a domain onto a range in proportion, as d3's scaleLinear:
// start to rng[0], start+diff to rng[1]. step, when not 0, is the step its
// ticks take when it is aligned with another scale.
type linearMap struct {
	start, diff float64
	rng         [2]float32
	step        float64
}

func newLinear(domain [2]float64, rng [2]float32) linearMap {
	lo, hi := min(domain[0], domain[1]), max(domain[0], domain[1])
	return linearMap{start: lo, diff: hi - lo, rng: rng}
}

func alignedLinear(a aligned, rng [2]float32) linearMap {
	m := newLinear(a.domain, rng)
	m.step = a.step
	return m
}

// tick is where v goes; a domain of one value puts it in the middle, as d3
// does.
func (l linearMap) tick(v float64) (float32, bool) {
	if !isFinite(v) {
		return 0, false
	}
	ratio := 0.5
	if l.diff != 0 {
		ratio = (v - l.start) / l.diff
	}
	return l.rng[0] + float32(ratio)*(l.rng[1]-l.rng[0]), true
}

func (l linearMap) ends() [2]float64 { return [2]float64{l.start, l.start + l.diff} }

// aligned is a domain laid out in whole steps (nice ones: 1, 2 or 5 times a
// power of ten), whose ticks fall on the same places as another's.
type aligned struct {
	domain [2]float64
	step   float64
}

// minAlignedDivisions is the fewest divisions aligned scales take when they
// may: a grid of fewer lines tells little.
const minAlignedDivisions = 3

// align lays the extents a and b of two scales out in nice steps with the
// same number of divisions, at most limit, so that their ticks fall on the
// same places, as ECharts' alignTicks: when both hold zero, zero falls on
// the same place too; otherwise a domain gains its extra divisions away
// from zero when it starts or ends there and on both sides when not. Of the
// choices, those of at least minAlignedDivisions come first, then those
// whose data fill the most of both domains, then those of more divisions.
func align(a, b [2]float64, limit int) (aligned, aligned, bool) {
	limit = max(limit, 1)
	valid := func(e [2]float64) bool { return isFinite(e[0]) && isFinite(e[1]) && e[1] > e[0] }
	if !valid(a) || !valid(b) {
		return aligned{}, aligned{}, false
	}
	holdsZero := func(e [2]float64) bool { return e[0] <= 0 && e[1] >= 0 }
	bothZero := holdsZero(a) && holdsZero(b)
	dense := min(minAlignedDivisions, limit)
	var bestA, bestB aligned
	found, wasDense, bestFill, bestDivisions := false, false, 0.0, 0
	ca, cb := niceSteps(a, limit), niceSteps(b, limit)
	for _, sa := range ca {
		for _, sb := range cb {
			var la, ha, lb, hb, divisions int64
			if bothZero {
				below, above := max(-sa.lo, -sb.lo), max(sa.hi, sb.hi)
				la, ha, lb, hb, divisions = -below, above, -below, above, below+above
			} else {
				divisions = max(sa.hi-sa.lo, sb.hi-sb.lo)
				la, ha = widen(sa.lo, sa.hi, divisions)
				lb, hb = widen(sb.lo, sb.hi, divisions)
			}
			if divisions == 0 || int(divisions) > limit {
				continue
			}
			fill := (a[1]-a[0])/(float64(divisions)*sa.step) + (b[1]-b[0])/(float64(divisions)*sb.step)
			isDense := int(divisions) >= dense
			better := !found || isDense && !wasDense ||
				isDense == wasDense && (fill > bestFill+1e-9 || math.Abs(fill-bestFill) <= 1e-9 && int(divisions) > bestDivisions)
			if better {
				found, wasDense, bestFill, bestDivisions = true, isDense, fill, int(divisions)
				bestA = aligned{[2]float64{float64(la) * sa.step, float64(ha) * sa.step}, sa.step}
				bestB = aligned{[2]float64{float64(lb) * sb.step, float64(hb) * sb.step}, sb.step}
			}
		}
	}
	return bestA, bestB, found
}

// niceStep is a step and the whole steps an extent's ends round out to.
type niceStep struct {
	step   float64
	lo, hi int64
}

// niceSteps are the nice steps that lay extent out in at most limit
// divisions.
func niceSteps(extent [2]float64, limit int) []niceStep {
	// Float division's slack (0.3 / 0.1 = 2.9999…) adds no division.
	const eps = 1e-9
	least := (extent[1] - extent[0]) / float64(limit)
	base := math.Pow(10, math.Floor(math.Log10(least)))
	var out []niceStep
	for k := range 4 {
		for _, factor := range []float64{1, 2, 5} {
			step := base * math.Pow(10, float64(k)) * factor
			l, h := int64(math.Floor(extent[0]/step+eps)), int64(math.Ceil(extent[1]/step-eps))
			seen := slices.ContainsFunc(out, func(s niceStep) bool { return s.step == step })
			if h-l >= 1 && int(h-l) <= limit && !seen {
				out = append(out, niceStep{step, l, h})
			}
		}
	}
	return out
}

// widen takes the whole steps lo to hi to divisions: up from a domain that
// starts at zero, down to one that ends there, evenly round others.
func widen(lo, hi, divisions int64) (int64, int64) {
	extra := divisions - (hi - lo)
	switch {
	case lo == 0:
		return lo, hi + extra
	case hi == 0:
		return lo - extra, hi
	}
	return lo - extra/2, hi + (extra - extra/2)
}

// bandMap is bands laid out across a range as d3's scaleBand lays them out:
// len bands one step apart, paddingInner of a step between neighbours and
// paddingOuter of a step at each end, the whole in the middle of the range;
// each band at most maxWidth wide, in the middle of its place.
type bandMap struct {
	len                        int
	start, diff                float32
	paddingInner, paddingOuter float32
	maxWidth                   float32
	groupPadding               float32
	reverse                    bool
}

// newBands is rows bands, or BandCount when more, across rng.
func newBands(rows int, rng [2]float32, options Band, reverse bool) bandMap {
	return bandMap{len: max(rows, options.BandCount), start: min(rng[0], rng[1]),
		diff:         float32(math.Abs(float64(rng[1] - rng[0]))),
		paddingInner: min(max(options.PaddingInner, 0), 1), paddingOuter: max(options.PaddingOuter, 0),
		maxWidth: float32(max(options.MaxBandWidth, 0)), groupPadding: min(max(options.GroupPadding, 0), 1), reverse: reverse}
}

// bandGroup is count bands across width with padding between them and none
// at the ends: the places of the bars side by side in a band.
func bandGroup(count int, width, padding float32) bandMap {
	b := bandMap{len: count, diff: width}
	// One bar fills the band: the gap is between bars.
	if count > 1 {
		b.paddingInner = padding
	}
	return b
}

// unit is the same bands over a range from 0 to 1 and no widest band: where
// they go as shares of the axis, whatever its length.
func (b bandMap) unit() bandMap {
	b.start, b.diff, b.maxWidth = 0, 1, 0
	return b
}

// groups is the places of count bars side by side in each band.
func (b bandMap) groups(count int) bandMap { return bandGroup(count, b.width(), b.groupPadding) }

// stepSize is the distance between the starts of two neighbouring bands.
func (b bandMap) stepSize() float32 {
	if b.len == 0 {
		return 0
	}
	return b.diff / max(float32(b.len)-b.paddingInner+2*b.paddingOuter, 1)
}

func (b bandMap) fullWidth() float32 { return b.stepSize() * (1 - b.paddingInner) }

// width is a band's width: its step less the inner padding, and no wider
// than maxWidth.
func (b bandMap) width() float32 {
	if w := b.fullWidth(); b.maxWidth <= 0 || w < b.maxWidth {
		return w
	}
	return b.maxWidth
}

// startOf is where band index starts, a band narrowed by maxWidth in the
// middle of its place; from the end of the range when reversed.
func (b bandMap) startOf(index int) (float32, bool) {
	if index < 0 || index >= b.len {
		return 0, false
	}
	place := index
	if b.reverse {
		place = b.len - 1 - index
	}
	step := b.stepSize()
	first := b.start + (b.diff-step*(float32(b.len)-b.paddingInner))/2
	return first + float32(place)*step + (b.fullWidth()-b.width())/2, true
}

// center is the middle of band index.
func (b bandMap) center(index int) (float32, bool) {
	s, ok := b.startOf(index)
	return s + b.width()/2, ok
}

// tick is where v goes: the middle of band v, and in proportion between
// the middles of two bands.
func (b bandMap) tick(v float64) (float32, bool) {
	if !isFinite(v) {
		return 0, false
	}
	first, ok := b.center(0)
	if !ok {
		return 0, false
	}
	step := b.stepSize()
	if b.reverse {
		step = -step
	}
	return first + float32(v)*step, true
}

// axisKind is what an axisMap places.
type axisKind uint8

const (
	axisPoint axisKind = iota
	axisLinear
	axisBand
)

// axisMap places values along an axis of a plot, laid out for its size:
// row indices evenly over the range on a point scale, values in proportion
// on a linear one, the middle of each band on a band one.
type axisMap struct {
	kind   axisKind
	len    int
	rng    [2]float32
	linear linearMap
	bands  bandMap
}

func (m axisMap) tick(v float64) (float32, bool) {
	switch m.kind {
	case axisLinear:
		return m.linear.tick(v)
	case axisBand:
		return m.bands.tick(v)
	}
	switch {
	case m.len == 0 || !isFinite(v):
		return 0, false
	case m.len == 1:
		return (m.rng[0] + m.rng[1]) / 2, true
	}
	return m.rng[0] + float32(v)*(m.rng[1]-m.rng[0])/float32(m.len-1), true
}

// ticks is where n rows go: by values on a linear scale, else by index.
func (m axisMap) ticks(n int, values []float64) []float32 {
	if m.kind == axisLinear && values != nil {
		n = len(values)
	}
	out := make([]float32, n)
	for i := range out {
		v := float64(i)
		if m.kind == axisLinear && values != nil {
			v = values[i]
		}
		at, ok := m.tick(v)
		if !ok {
			at = float32(math.NaN())
		}
		out[i] = at
	}
	return out
}

// band reports the bands of a band scale.
func (m axisMap) band() (bandMap, bool) { return m.bands, m.kind == axisBand }

// backward reports whether its places run back along the axis, toward
// smaller coordinates as the rows or values grow.
func (m axisMap) backward() bool {
	switch m.kind {
	case axisLinear:
		return m.linear.rng[1] < m.linear.rng[0]
	case axisBand:
		return m.bands.reverse
	}
	return m.rng[1] < m.rng[0]
}

// plotExtent is the extent of some values, ok when any is finite.
type plotExtent struct {
	lo, hi float64
	ok     bool
}

func (e plotExtent) with(v float64) plotExtent {
	switch {
	case !isFinite(v):
		return e
	case !e.ok:
		return plotExtent{v, v, true}
	}
	return plotExtent{min(e.lo, v), max(e.hi, v), true}
}

func (e plotExtent) join(o plotExtent) plotExtent {
	if !o.ok {
		return e
	}
	return e.with(o.lo).with(o.hi)
}

func (e plotExtent) ends() [2]float64 { return [2]float64{e.lo, e.hi} }

func extentOf(values []float64) plotExtent {
	var e plotExtent
	for _, v := range values {
		e = e.with(v)
	}
	return e
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// unique is domain without the categories written again.
func unique(domain []string) []string {
	var out []string
	for _, c := range domain {
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}
