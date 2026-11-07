package plot

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// AxisPlacement is the edge of a Plot an Axis runs along, as d3's
// axisBottom, axisTop, axisLeft and axisRight: the bottom and top edges
// carry the x scale's ticks, the left and right ones the y scale's.
type AxisPlacement uint8

const (
	AxisBottom AxisPlacement = iota
	AxisTop
	AxisLeft
	AxisRight
)

func (a AxisPlacement) alongX() bool { return a == AxisBottom || a == AxisTop }

// AxisLabelPlacement is which side of its edge an Axis lays its labels on.
type AxisLabelPlacement uint8

const (
	// LabelsOutside puts them past the plot's edge, in the room the app
	// leaves around the plot, as d3's axes sit in a chart's margins: they
	// never cover a mark.
	LabelsOutside AxisLabelPlacement = iota
	// LabelsInside puts them over the plot, along the inside of its edge,
	// so the plot keeps its whole size.
	LabelsInside
)

// Axis is an axis of a Plot, as gpui-base's PlotAxis and d3's axes: along
// one edge, ticks at values of the scale the edge carries (d3's nice values
// on a linear scale, rows on a point or band one), each with its label,
// laid out natively by the plot's layout: centered on its tick along the
// edge (along the x scale's, the first and last moved inward so no label
// passes the plot's ends; beside the y scale's, the plot keeping what a
// label near its top or bottom edge would pass it by inside it, so every
// label sits on its tick within the plot), and past the edge or along its
// inside. Its parts are the app's to style (the labels, the tick marks and
// the line), and only the labels show unless the app styles the others. It
// follows the plot's size and scales, and takes no pointer; screen readers
// hear the plot, not its labels.
type Axis struct {
	Placement      AxisPlacement
	LabelPlacement AxisLabelPlacement
	// Scale is the y scale a left or right axis carries; a bottom or top
	// axis carries the x scale.
	Scale ValueScale
	// Ticks is about how many ticks it takes, as d3's axis.ticks: on a
	// linear scale d3's nice values inside the domain (10 without), on a
	// point or band scale this many rows spread evenly from the first to
	// the last (every row without).
	Ticks int
	// TickValues is the values of its ticks, in place of Ticks: values of
	// the scale, row indices on a point or band scale. Those outside the
	// scale's domain have no place and no tick.
	TickValues func() []float64
	// Format is the text of each tick, as d3's axis.tickFormat: without it,
	// the category of a band on a band scale of categories, else d3's
	// default, the value with the decimals the step between ticks needs,
	// thousands grouped.
	Format func(value float64, index int) string
	// LabelStyle styles its labels: a text per tick, its font, color and
	// padding (its distance from the edge) the app's.
	LabelClass     string
	LabelStyle     core.Style
	LabelStyleFunc func() core.Style
	// TickStyle styles its tick marks: a view per tick from the edge toward
	// its label, centered on the tick; none without a class or style.
	TickClass     string
	TickStyle     core.Style
	TickStyleFunc func() core.Style
	// LineStyle styles its line: a view along the whole edge on its labels'
	// side; none without a class or style.
	LineClass     string
	LineStyle     core.Style
	LineStyleFunc func() core.Style
}

// shareEpsilon is how far a label's share of the range moves before the
// insets follow it: far above float32's rounding of a share recomputed over
// a new range, far below a share any tick moves by.
const shareEpsilon = 1e-4

// labelMeasure is a value by a label's index.
type labelMeasure struct {
	i int
	v float32
}

// labelInsets is how far in from a plot height tall its y scale's range
// must start and end for labels of heights, centered on ticks at shares of
// the range from its top, to stay inside it: a label h tall at share f sits
// at top + f*(height-top-bottom), which must be h/2 from either edge. The
// two insets lean on each other, so they are found in a few rounds; neither
// takes more than a third of the plot.
func labelInsets(heights, shares []labelMeasure, height float32) [2]float32 {
	var top, bottom float32
	most := height / 3
	type pair struct{ f, half float32 }
	// A label not yet laid out takes the tallest one's height: an inset that
	// fell away for it would give the aligned scales other steps, and those
	// other labels, without end.
	var tallest float32
	for _, h := range heights {
		tallest = max(tallest, h.v)
	}
	pairs := make([]pair, 0, len(shares))
	for _, s := range shares {
		height := tallest
		for _, h := range heights {
			if h.i == s.i {
				height = h.v
				break
			}
		}
		pairs = append(pairs, pair{min(max(s.v, 0), 1), height / 2})
	}
	for range 4 {
		top = 0
		for _, p := range pairs {
			if p.f < 1 {
				top = max(top, (p.half-p.f*(height-bottom))/(1-p.f))
			}
		}
		top = min(top, most)
		bottom = 0
		for _, p := range pairs {
			if p.f > 0 {
				bottom = max(bottom, (p.half-(1-p.f)*(height-top))/p.f)
			}
		}
		bottom = min(bottom, most)
	}
	return [2]float32{top, bottom}
}

// axisLabel is a tick and its text.
type axisLabel struct {
	value float64
	text  string
}

// axisSide is an axis's place on the plot, laying out its parts.
type axisSide struct {
	placement AxisPlacement
	labels    AxisLabelPlacement
	size      core.Size
}

func (s axisSide) extent() float32 {
	if s.placement.alongX() {
		return float32(s.size.Width)
	}
	return float32(s.size.Height)
}

func (s axisSide) edge() float32 {
	switch s.placement {
	case AxisBottom:
		return float32(s.size.Height)
	case AxisRight:
		return float32(s.size.Width)
	}
	return 0
}

// grow lays out from the edge toward the labels' side.
func (s axisSide) grow() core.Justify {
	outward := s.placement == AxisBottom || s.placement == AxisRight
	if outward == (s.labels == LabelsOutside) {
		return core.JustifyStart
	}
	return core.JustifyEnd
}

func pts(v float32) core.Length { return core.Pt(core.Points(v)) }

// line is the line's box: no thickness along the whole edge, the app's view
// growing from it toward the labels and stretched along it.
func (s axisSide) line() core.Style {
	if s.placement.alongX() {
		return core.Style{Direction: core.Column, Left: pts(0), Top: pts(s.edge()), Width: pts(s.extent()), Height: pts(0), JustifyContent: s.grow()}
	}
	return core.Style{Direction: core.Row, Top: pts(0), Left: pts(s.edge()), Height: pts(s.extent()), Width: pts(0), JustifyContent: s.grow()}
}

// tick is a tick mark's anchor: a point on the edge at at, the app's view
// centered on it and growing toward the labels.
func (s axisSide) tick(at float32, ok bool) core.Style {
	if !ok {
		return core.Style{Display: core.DisplayNone}
	}
	if s.placement.alongX() {
		return core.Style{Direction: core.Column, Left: pts(at), Top: pts(s.edge()), JustifyContent: s.grow()}
	}
	return core.Style{Direction: core.Row, Top: pts(at), Left: pts(s.edge()), JustifyContent: s.grow()}
}

// anchor is a label's anchor: room around the tick at at, as far on either
// side so the label centers on the tick in it, past the edge or inside it.
// Along the x scale's edge the room lies within the plot's ends; beside the
// y scale's, outside, it reaches the plot's height either way, so every
// label sits on its tick. There it is a row with no width, the label's
// window overflowing it away from the plot or into it; inside, the room
// runs from the top edge down to the tick, the label at its bottom.
func (s axisSide) anchor(at float32, ok bool) core.Style {
	if !ok {
		return core.Style{Display: core.DisplayNone}
	}
	room := max(min(at, s.extent()-at), 0)
	edge := s.edge()
	if s.placement.alongX() {
		out := core.Style{Direction: core.Row, Left: pts(at - room), Width: pts(2 * room)}
		switch {
		case s.placement == AxisBottom && s.labels == LabelsOutside, s.placement == AxisTop && s.labels == LabelsInside:
			out.Top = pts(edge)
		case s.placement == AxisTop:
			out.Bottom = pts(float32(s.size.Height))
		default:
			out.Bottom = pts(0)
		}
		return out
	}
	out := core.Style{Direction: core.Row, Left: pts(edge), Width: pts(0), JustifyContent: s.grow()}
	if s.labels == LabelsOutside {
		room = max(s.extent(), room)
		out.Top, out.Height = pts(at-room), pts(2*room)
	} else {
		out.Top, out.Height = pts(0), pts(at)
	}
	return out
}

// window is a label's window in its anchor: along the x scale's edge it
// fills the anchor; beside the y scale's it is as wide as the label and as
// tall as the anchor. Its label goes toward the plot's end the tick is
// nearer.
func (s axisSide) window(at float32, ok bool) core.Style {
	nearStart := ok && at <= s.extent()/2
	toTick := s.labels == LabelsInside && !s.placement.alongX()
	out := core.Style{Direction: core.Column, JustifyContent: core.JustifyEnd}
	if s.placement.alongX() {
		out = core.Style{FlexGrow: 1, MinWidth: pts(0), Direction: core.Row, JustifyContent: core.JustifyEnd}
	}
	if nearStart || toTick {
		out.JustifyContent = core.JustifyStart
	}
	return out
}

// label is the label's box: centered in its window while it fits.
func (s axisSide) label() core.Style {
	switch {
	case s.placement.alongX():
		return core.Style{AutoMargin: core.SideLeft | core.SideRight}
	case s.labels == LabelsOutside:
		return core.Style{AutoMargin: core.SideTop | core.SideBottom}
	}
	return core.Style{AutoMargin: core.SideTop}
}

// Render implements core.Node.
func (a Axis) Render(cx *core.Scope) core.Node {
	layer := core.Style{Position: core.Absolute, Left: pts(0), Top: pts(0), Right: pts(0), Bottom: pts(0)}
	plot := plotOf(cx, "Axis")
	if plot == nil {
		return core.View{Style: layer, PointerEvents: core.PointerEventsNone}
	}
	alongX := a.Placement.alongX()
	mapOf := func() axisMap {
		if alongX {
			return plot.xMap.Get()
		}
		return plot.yMapOf(a.Scale)
	}
	// The categories of the scale's bands, which its labels write.
	categories := func() []string {
		switch {
		case alongX:
			return plot.xCategories.Get()
		case a.Scale == ScaleY:
			return plot.yCategories.Get()
		}
		return nil
	}
	// The ticks follow the scale's domain, or its rows or categories, not
	// the plot's size: a resize moves the labels without writing them again.
	scale := core.NewMemo(cx, func() scaleTicks {
		rows := plot.rows()
		if c := categories(); c != nil {
			rows = len(c)
		}
		return ticksOf(mapOf(), rows)
	})
	ticks := core.NewMemoFunc(cx, func() tickSet {
		var values []float64
		if a.TickValues != nil {
			values = a.TickValues()
		}
		return scale.Get().ticks(a.Ticks, values, a.TickValues != nil)
	}, func(x, y tickSet) bool { return x.precision == y.precision && slices.Equal(x.values, y.values) })
	labels := core.NewMemoFunc(cx, func() []axisLabel {
		t := ticks.Get()
		c := categories()
		out := make([]axisLabel, len(t.values))
		for i, v := range t.values {
			text := ""
			switch {
			case a.Format != nil:
				text = a.Format(v, i)
			case c != nil && v == math.Trunc(v) && v >= 0 && int(v) < len(c):
				text = c[int(v)]
			default:
				text = defaultTickText(v, t.precision)
			}
			out[i] = axisLabel{v, text}
		}
		return out
	}, slices.Equal)
	// Beside the y scale's edge, outside, a label on its tick reaches half
	// its height above and below it: the plot keeps what would pass its top
	// and bottom edges inside them, from each label's height and where its
	// tick lies along the scale.
	heights := core.NewSignalFunc(cx, []labelMeasure(nil), nil)
	shares := core.NewSignalFunc(cx, []labelMeasure(nil), nil)
	if !alongX && a.LabelPlacement == LabelsOutside {
		cx.Effect(func() {
			m := mapOf()
			var now []labelMeasure
			if m.kind == axisLinear {
				l, h := m.linear.rng[0], m.linear.rng[1]
				top, span := min(l, h), float32(math.Abs(float64(l-h)))
				for i, label := range labels.Get() {
					// Only a label on the scale.
					at, ok := m.linear.tick(label.value)
					if !ok || span <= 0 {
						continue
					}
					if share := (at - top) / span; share >= -1e-3 && share <= 1+1e-3 {
						now = append(now, labelMeasure{i, share})
					}
				}
			}
			// A share does not change with the inset it feeds, but
			// recomputed over the inset's new range it differs in its last
			// bits: compared exactly, each change would move the inset
			// again, without end.
			if was := shares.GetUntracked(); !slices.EqualFunc(was, now, func(x, y labelMeasure) bool {
				return x.i == y.i && math.Abs(float64(x.v-y.v)) <= shareEpsilon
			}) {
				shares.Set(now)
			}
		})
		plot.addInset(cx, core.NewMemo(cx, func() [2]float32 {
			return labelInsets(heights.Get(), shares.Get(), float32(plot.size.Get().Height))
		}))
	}
	side := func() axisSide { return axisSide{a.Placement, a.LabelPlacement, plot.size.Get()} }
	// Where tick i is along the edge.
	at := func(i int) (float32, bool) {
		all := labels.Get()
		if i >= len(all) {
			return 0, false
		}
		return mapOf().tick(all[i].value)
	}
	indices := func() []int {
		n := len(labels.Get())
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	id := func(i int) int { return i }
	tween := plot.xTween
	if !alongX {
		tween = plot.yTweenOf(a.Scale)
	}
	// While the axis's domain glides, tick i moves with the marks from where
	// the layout puts it to where the domain shown puts it, as d3's axis
	// transitions.
	glide := func(cx *core.Scope, i func() int) *core.AnimatedStyle {
		place := core.NewAnimatedValue(cx, 0)
		cx.Effect(func() {
			if t, ok := at(i()); ok && tween.moving.Get() {
				place.Set(float64(t))
			}
		})
		moved := tween.moved(place)
		if alongX {
			return &core.AnimatedStyle{TranslateX: moved}
		}
		return &core.AnimatedStyle{TranslateY: moved}
	}
	line := parts.Of(a.LineClass, a.LineStyle, a.LineStyleFunc)
	tick := parts.Of(a.TickClass, a.TickStyle, a.TickStyleFunc)
	sized := plot.sized
	var children []core.Node
	if line.Set() {
		children = append(children, core.Show{When: sized, Then: func(*core.Scope) core.Node {
			return core.View{Style: core.Style{Position: core.Absolute}, StyleFunc: func() core.Style { return side().line() },
				Children: []core.Node{core.View{Class: line.Class, Style: line.Style, StyleFunc: line.Fn}}}
		}})
	}
	if tick.Set() {
		children = append(children, core.Show{When: sized, Then: func(*core.Scope) core.Node {
			return core.Each(indices, id, func(cx *core.Scope, item func() int, _ func() int) core.Node {
				return core.View{
					Style: core.Style{Position: core.Absolute, Width: pts(0), Height: pts(0), AlignItems: core.AlignCenter, Animated: glide(cx, item)},
					StyleFunc: func() core.Style {
						t, ok := at(item())
						return side().tick(t, ok)
					},
					Children: []core.Node{core.View{Class: tick.Class, Style: tick.Style, StyleFunc: tick.Fn}},
				}
			})
		}})
	}
	children = append(children, core.Show{When: sized, Then: func(*core.Scope) core.Node {
		return core.Each(indices, id, func(cx *core.Scope, item func() int, _ func() int) core.Node {
			i := item()
			cx.Cleanup(func() {
				heights.Update(func(all []labelMeasure) []labelMeasure {
					return slices.DeleteFunc(slices.Clone(all), func(h labelMeasure) bool { return h.i == i })
				})
			})
			text := func() string {
				if all := labels.Get(); i < len(all) {
					return all[i].text
				}
				return ""
			}
			// The anchor spans the room around the tick, the window inside
			// it aligns the label toward the tick's end of the plot, and
			// the label's own automatic margins center it on the tick while
			// it fits: they are none when it is wider than its room, so it
			// starts at the plot's end it is near rather than passing it.
			return core.View{
				Style: core.Style{Position: core.Absolute, Animated: glide(cx, item)},
				StyleFunc: func() core.Style {
					t, ok := at(i)
					return side().anchor(t, ok)
				},
				Children: []core.Node{core.View{
					StyleFunc: func() core.Style {
						t, ok := at(i)
						return side().window(t, ok)
					},
					Children: []core.Node{core.View{
						StyleFunc: func() core.Style { return side().label() },
						OnLayout: func(b core.Bounds) {
							h := float32(b.Size.Height)
							if !slices.Contains(heights.GetUntracked(), labelMeasure{i, h}) {
								heights.Update(func(all []labelMeasure) []labelMeasure {
									all = slices.DeleteFunc(slices.Clone(all), func(m labelMeasure) bool { return m.i == i })
									return append(all, labelMeasure{i, h})
								})
							}
						},
						Children: []core.Node{core.Text{ContentFunc: text, Class: a.LabelClass, Style: a.LabelStyle, StyleFunc: a.LabelStyleFunc}},
					}},
				}},
			}
		})
	}})
	return core.View{Style: layer, PointerEvents: core.PointerEventsNone, Children: children}
}

// defaultTicks is the ticks of an Axis when the app gives no Ticks, as
// d3's scale.ticks().
const defaultTicks = 10

// tickSet is the values an axis ticks and the decimals its default texts
// write.
type tickSet struct {
	values    []float64
	precision int
}

// scaleTicks is what a scale's ticks follow: its domain and the step of an
// aligned scale, or how many places it lays out and how many rows there are.
type scaleTicks struct {
	linear       bool
	domain       [2]float64
	step         float64
	places, rows int
}

func ticksOf(m axisMap, rows int) scaleTicks {
	switch m.kind {
	case axisLinear:
		return scaleTicks{linear: true, domain: m.linear.ends(), step: m.linear.step}
	case axisBand:
		return scaleTicks{places: m.bands.len, rows: rows}
	}
	return scaleTicks{places: m.len, rows: m.len}
}

// tickValues is the values of the ticks of m for a plot of rows rows: about
// count, or its default ticks without one, the step of an aligned scale's.
func tickValues(m axisMap, rows, count int) []float64 {
	return ticksOf(m, rows).ticks(count, nil, false).values
}

// ticks is about count nice values in the domain, or count of the rows
// spread evenly from the first to the last, or the app's values within the
// domain.
func (s scaleTicks) ticks(count int, values []float64, given bool) tickSet {
	switch {
	case s.linear && s.step != 0 && count == 0 && !given:
		// Aligned with another scale: a tick at every step, as that one's.
		lo, hi := s.domain[0], s.domain[1]
		n := int(max(math.Round((hi-lo)/s.step), 0))
		out := make([]float64, n+1)
		for i := range out {
			out[i] = lo + float64(i)*s.step
		}
		return tickSet{out, stepPrecision(s.step)}
	case s.linear:
		lo, hi := s.domain[0], s.domain[1]
		c := float64(count)
		if count == 0 {
			c = defaultTicks
		}
		if !given {
			return tickSet{linearTicks(lo, hi, c), tickPrecision(lo, hi, c)}
		}
		slack := (hi - lo) * 1e-9
		var out []float64
		for _, v := range values {
			if v >= lo-slack && v <= hi+slack {
				out = append(out, v)
			}
		}
		return tickSet{out, tickPrecision(lo, hi, c)}
	case given:
		var out []float64
		for _, v := range values {
			if v >= 0 && v <= float64(s.places-1) {
				out = append(out, v)
			}
		}
		return tickSet{values: out}
	}
	var out []float64
	for _, i := range spread(s.places, count) {
		if i < s.rows {
			out = append(out, float64(i))
		}
	}
	return tickSet{values: out}
}

// spread is count of places spread evenly from the first to the last, both
// included; every one without a count, or with as many.
func spread(places, count int) []int {
	switch {
	case places == 0:
		return nil
	case count == 1:
		return []int{0}
	}
	var out []int
	if count > 1 && count < places {
		for k := range count {
			out = append(out, int(jsRound(float64(k)*float64(places-1)/float64(count-1))))
		}
		return out
	}
	for i := range places {
		out = append(out, i)
	}
	return out
}

// d3's thresholds between the steps of 1, 2, 5 and 10 times a power of ten.
const (
	e10 = 7.0710678118654755
	e5  = 3.1622776601683795
	e2  = math.Sqrt2
)

// jsRound is JS's Math.round: halves up.
func jsRound(x float64) float64 { return math.Floor(x + 0.5) }

// tickSpec is d3's tickSpec for start below stop: the first and last tick
// as multiples of the increment inc, which is the step, or for a step below
// 1 its inverse, negated, so the ticks are i / -inc, exact.
func tickSpec(start, stop, count float64) (i1, i2, inc float64) {
	step := (stop - start) / max(count, 0)
	power := math.Floor(math.Log10(step))
	err := step / math.Pow(10, power)
	factor := 1.0
	switch {
	case err >= e10:
		factor = 10
	case err >= e5:
		factor = 5
	case err >= e2:
		factor = 2
	}
	if power < 0 {
		n := math.Pow(10, -power) / factor
		i1, i2 = jsRound(start*n), jsRound(stop*n)
		if i1/n < start {
			i1++
		}
		if i2/n > stop {
			i2--
		}
		inc = -n
	} else {
		n := math.Pow(10, power) * factor
		i1, i2 = jsRound(start/n), jsRound(stop/n)
		if i1*n < start {
			i1++
		}
		if i2*n > stop {
			i2--
		}
		inc = n
	}
	if i2 < i1 && count >= 0.5 && count < 2 {
		return tickSpec(start, stop, count*2)
	}
	return i1, i2, inc
}

// linearTicks is d3's ticks(lo, hi, count): about count values of 1, 2 or
// 5 times a power of ten inside lo to hi, from the lowest.
func linearTicks(lo, hi, count float64) []float64 {
	if math.IsNaN(count) || count <= 0 || !isFinite(lo) || !isFinite(hi) {
		return nil
	}
	if lo == hi {
		return []float64{lo}
	}
	i1, i2, inc := tickSpec(lo, hi, count)
	if math.IsNaN(i1) || math.IsNaN(i2) || i2 < i1 {
		return nil
	}
	out := make([]float64, int(i2-i1)+1)
	for i := range out {
		k := i1 + float64(i)
		if inc < 0 {
			out[i] = k / -inc
		} else {
			out[i] = k * inc
		}
	}
	return out
}

// tickPrecision is the decimals d3's default tick text writes: as many as
// the step needs. A domain of one value writes what the value needs, up to
// six.
func tickPrecision(lo, hi, count float64) int {
	if lo == hi || math.IsNaN(count) || count <= 0 {
		for p := range 6 {
			scale := math.Pow(10, float64(p))
			if math.Abs(math.Round(lo*scale)/scale-lo) < 1e-9 {
				return p
			}
		}
		return 6
	}
	if _, _, inc := tickSpec(lo, hi, count); inc < 0 {
		return int(max(math.Ceil(math.Log10(-inc)), 0))
	}
	return 0
}

// stepPrecision is the decimals a tick step apart needs, up to six.
func stepPrecision(step float64) int {
	for p := range 6 {
		scaled := step * math.Pow(10, float64(p))
		if math.Abs(scaled-math.Round(scaled)) < 1e-6*max(math.Abs(scaled), 1) {
			return p
		}
	}
	return 6
}

// defaultTickText is value as d3's default tick format writes it:
// precision decimals, thousands grouped with commas, a minus sign before a
// value below zero that does not round to zero.
func defaultTickText(value float64, precision int) string {
	if !isFinite(value) {
		return ""
	}
	scale := math.Pow(10, float64(precision))
	fixed := strconv.FormatFloat(math.Round(math.Abs(value)*scale)/scale, 'f', precision, 64)
	whole, fraction, dotted := strings.Cut(fixed, ".")
	var b strings.Builder
	if value < 0 && strings.Trim(fixed, "0.") != "" {
		b.WriteRune('−')
	}
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if dotted {
		fmt.Fprintf(&b, ".%s", fraction)
	}
	return b.String()
}
