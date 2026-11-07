package plot

import (
	"log"
	"math"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// barEntry is a bar a plot knows, for laying out its stacks and groups.
type barEntry struct {
	key    int
	stack  string
	scale  ValueScale
	base   func() float64
	bases  func() []float64
	values func() []float64
}

// barLayout is where a plot's bars go: how many stand side by side in a
// band, and each bar's place among them and its segments.
type barLayout struct {
	slots int
	bars  []barPlace
}

// barPlace is a bar's place in a band, its segment in each row (the values
// it spans, NaN where it has none), the value its stack grows from in each
// row, and its stack's constant base, which counts in the domain without
// rows too.
type barPlace struct {
	key      int
	slot     int
	segments [][2]float64
	origins  []float64
	floor    float64
	hasFloor bool
}

func (b barPlace) extent() plotExtent {
	var e plotExtent
	for _, s := range b.segments {
		e = e.with(s[0]).with(s[1])
	}
	for _, o := range b.origins {
		e = e.with(o)
	}
	if b.hasFloor {
		e = e.with(b.floor)
	}
	return e
}

func (l *barLayout) of(key int) (barPlace, bool) {
	if l != nil {
		for _, b := range l.bars {
			if b.key == key {
				return b, true
			}
		}
	}
	return barPlace{}, false
}

// stackID is a stack: a stack of each y scale is its own.
type stackID struct {
	scale ValueScale
	name  string
}

// barPile is a stack's state while the layout piles its bars.
type barPile struct {
	id       stackID
	ends     []float64
	origins  []float64
	floor    float64
	hasFloor bool
}

// layOutBars is the layout of bars, in order, as gpui-base's Stack: each
// stack's rows start at its first bar's base, each bar's segment from where
// the one before it ends, a missing value adding nothing; a row without a
// base has no segment.
func layOutBars(all []*barEntry) *barLayout {
	// A slot per bar outside any stack, one per stack.
	slots := 0
	slotOf := map[stackID]int{}
	var piles []*barPile
	nan := math.NaN()
	places := make([]barPlace, 0, len(all))
	for _, bar := range all {
		values, floor, hasFloor := bar.values(), 0.0, bar.bases == nil
		if bar.base != nil {
			floor = bar.base()
		}
		var bases []float64
		if bar.bases != nil {
			bases = bar.bases()
		}
		baseAt := func(row int) float64 {
			if bar.bases == nil {
				return floor
			}
			if row < len(bases) {
				return bases[row]
			}
			return nan
		}
		if bar.stack == "" {
			origins := make([]float64, len(values))
			segments := make([][2]float64, len(values))
			for i, v := range values {
				origins[i] = baseAt(i)
				if isFinite(v) && isFinite(origins[i]) {
					segments[i] = [2]float64{origins[i], v}
				} else {
					segments[i] = [2]float64{nan, nan}
				}
			}
			places = append(places, barPlace{bar.key, slots, segments, origins, floor, hasFloor})
			slots++
			continue
		}
		id := stackID{bar.scale, bar.stack}
		slot, ok := slotOf[id]
		if !ok {
			slot = slots
			slotOf[id] = slot
			slots++
		}
		var pile *barPile
		for _, p := range piles {
			if p.id == id {
				pile = p
				break
			}
		}
		if pile == nil {
			// The stack's first bar: its bases are the stack's.
			origins := make([]float64, len(values))
			for i := range values {
				origins[i] = baseAt(i)
			}
			pile = &barPile{id, append([]float64(nil), origins...), origins, floor, hasFloor}
			piles = append(piles, pile)
		}
		// Rows past the first bar's: the stack's constant base.
		for len(pile.ends) < len(values) {
			o := nan
			if pile.hasFloor {
				o = pile.floor
			}
			pile.ends = append(pile.ends, o)
			pile.origins = append(pile.origins, o)
		}
		segments := make([][2]float64, len(values))
		for i, v := range values {
			lo := pile.ends[i]
			if !isFinite(v) || !isFinite(lo) {
				segments[i] = [2]float64{nan, nan}
				continue
			}
			pile.ends[i] = lo + v
			segments[i] = [2]float64{lo, lo + v}
		}
		places = append(places, barPlace{bar.key, slot, segments, append([]float64(nil), pile.origins[:len(values)]...), pile.floor, pile.hasFloor})
	}
	return &barLayout{slots, places}
}

func newBarLayout(cx *core.Scope, bars *core.Signal[[]*barEntry]) *core.Memo[*barLayout] {
	return core.NewMemoFunc(cx, func() *barLayout { return layOutBars(bars.Get()) }, nil)
}

// barRect is a bar as drawn: where it starts across its band and how thick
// it is, the middle of its row's band, where its two values go along the
// value axis, and where its stack's base in its row goes: it grows from
// there as it appears.
type barRect struct {
	row                  int
	start, thick, center float32
	from, to, origin     float32
}

// span is its box along the value axis.
func (b barRect) span() (float32, float32) {
	return min(b.from, b.to), float32(math.Abs(float64(b.to - b.from)))
}

// barGeometry is a Bar's bars, as drawn: across the plot when the bands go
// down the y axis, axis the scale its values go along, step the distance
// between the middles of two bands, rows the rows the bands are laid out for.
type barGeometry struct {
	across bool
	axis   axis
	step   float32
	rows   int
	bars   []barRect
}

// Bar is a series of a Plot drawn as bars, as gpui-base's Bar: one per row
// with a value, in the row's band of the plot's band scale (XScale for
// upright bars, YScale for bars across), from Base to the row's value on
// the other scale. Bars with the same Stack pile up; the others stand side
// by side in each band. It fills with its Fill (inherited from the plot),
// the text color where none is set or with Color each bar's category's,
// over its background (a gradient per bar; Fill is then none where none is
// set), its corners rounded by Corners. Its values and base count in the
// value scale's domain when the plot has none. With the plot's appear, its
// bars grow out of their base, one after another with Stagger; while the
// pointer is over the plot, the bars of other rows fade by HoverDim; both on
// the platform's frame clock, with no mark drawn again. Label writes a text
// at each bar's end. It shows no hover dot.
type Bar struct {
	// Value is what each bar shows; the plot's Y without. A row without a
	// value has no bar.
	Value func() []float64
	// Base is the value its bars grow from on the plot's value scale, 0
	// without it, so a negative value grows the other way. A stack grows
	// from its first bar's, row by row. It counts in the scale's domain.
	Base func() float64
	// Bases is a base per row, in place of Base: each bar is then a range
	// from its row's base to its value (a waterfall's step, a candle's body,
	// a task's span), a row without one having no bar.
	Bases func() []float64
	// Stack is the stack it joins: the bars of a plot with the same Stack
	// pile up in order, each row's starting where the one before it ends.
	// Bars in different stacks, or in none, stand side by side in each band.
	Stack string
	// Scale is the y scale its upright bars' values go along; a stack piles
	// only the bars on its scale. Bars across put their values along x.
	Scale ValueScale
	// Corners rounds its bars' corners; square by default.
	Corners Corners
	// Stagger is the share of the plot's appear, 0 to 0.95, over which its
	// bars start, row by row, each growing for the rest of it.
	Stagger float32
	// HoverDim is how much its bars away from the row under the pointer
	// fade, as a share of their opacity: handed from bar to bar as the hover
	// glides, with the hover's fade.
	HoverDim float32
	// Color is the category of each bar, which the plot's ColorScale gives
	// a color: the bar's Fill where nothing sets one.
	Color func() []string
	// Label is the text beside each bar, one per row, at its value end:
	// above an upright bar, below one that grows down, after a bar across
	// and before one that grows back. LabelStyle styles it: its size, color
	// and padding, its distance from the bar, are the app's.
	Label          func() []string
	LabelClass     string
	LabelStyle     core.Style
	LabelStyleFunc func() core.Style
	// BackgroundStyle's Background and Gradients fill each bar's box, shown
	// only inside the bar, from its base to its tip, as an Area's.
	BackgroundClass     string
	BackgroundStyle     core.Style
	BackgroundStyleFunc func() core.Style
	Class               string
	Style               core.Style
	StyleFunc           func() core.Style
	TestID              string
}

// Render implements core.Node.
func (b Bar) Render(cx *core.Scope) core.Node {
	own := parts.Of(b.Class, b.Style, b.StyleFunc)
	plot := plotOf(cx, "Bar")
	if plot == nil {
		return markLayer(own, b.TestID)
	}
	values := func() []float64 {
		if b.Value != nil {
			return b.Value()
		}
		return plot.y()
	}
	entry := &barEntry{key: plot.nextKey(), stack: b.Stack, scale: b.Scale, base: b.Base, bases: b.Bases, values: values}
	key := entry.key
	layout := plot.barLayout
	along := func() axis {
		if plot.bandAxis.Get() == axisY {
			return axisX
		}
		return axisOf(b.Scale)
	}
	plot.valueMark(cx, core.NewMemo(cx, func() plotExtent {
		place, _ := layout.Get().of(key)
		return place.extent()
	}), along)
	addEntry(cx, plot.bars, entry)
	warned := false
	geometry := core.NewMemoFunc(cx, func() *barGeometry {
		if !plot.sized() {
			return nil
		}
		xMap, yMap := plot.xMap.Get(), plot.yMap.Get()
		var band bandMap
		var valueMap axisMap
		var on axis
		if bx, ok := xMap.band(); ok {
			band, valueMap, on = bx, plot.yMapOf(b.Scale), axisOf(b.Scale)
		} else if by, ok := yMap.band(); ok {
			band, valueMap, on = by, xMap, axisX
		} else {
			if !warned {
				warned = true
				log.Printf("base: Bar draws on a band scale: XScale or YScale of BandScale")
			}
			return nil
		}
		l := layout.Get()
		place, ok := l.of(key)
		if !ok {
			return nil
		}
		group := band.groups(l.slots)
		g := &barGeometry{across: on == axisX, axis: on, step: band.stepSize(), rows: band.len}
		for row, s := range place.segments {
			if !isFinite(s[0]) || !isFinite(s[1]) || s[0] == s[1] {
				continue
			}
			start, ok1 := band.startOf(row)
			slot, ok2 := group.startOf(place.slot)
			center, ok3 := band.center(row)
			from, ok4 := valueMap.tick(s[0])
			to, ok5 := valueMap.tick(s[1])
			if row >= len(place.origins) {
				continue
			}
			origin, ok6 := valueMap.tick(place.origins[row])
			if ok1 && ok2 && ok3 && ok4 && ok5 && ok6 {
				g.bars = append(g.bars, barRect{row, start + slot, group.width(), center, from, to, origin})
			}
		}
		return g
	}, nil)
	// Records the bars, each its own path, growing while the plot appears
	// and moving with the value axis's domain.
	draw := func(d *core.Drawing, g *barGeometry, fill func(barRect)) {
		appearing := plot.appearing.Get()
		tween := plot.tweenOf(g.axis)
		moving := tween.moving.Get()
		for _, bar := range g.bars {
			if alpha, ok := plot.rowOpacity(b.HoverDim, g.across, bar.center, g.step); ok {
				setAlpha(d, alpha)
			}
			lo, length := bar.span()
			at, size := constNum(lo), constNum(length)
			if appearing {
				// Scaled about the stack's base, so a stack grows whole.
				q := nodeNum(plot.grown(bar.row, g.rows, b.Stagger))
				at = q.mul(constNum(lo - bar.origin)).add(constNum(bar.origin))
				size = q.mul(constNum(length))
			}
			if moving {
				at, size = tween.shownNum(at), tween.stretchedNum(size)
			}
			start, thick := constNum(bar.start), constNum(bar.thick)
			d.BeginPath()
			if g.across {
				rectPath(d, at, start, size, thick, b.Corners)
			} else {
				rectPath(d, start, at, thick, size, b.Corners)
			}
			fill(bar)
		}
	}
	background := parts.Of(b.BackgroundClass, b.BackgroundStyle, b.BackgroundStyleFunc)
	hasBackground := background.Set()
	var children []core.Node
	if hasBackground {
		children = append(children, markCanvas(plot, room, background, func(d *core.Drawing, _ *plotState, _ float32) {
			g := geometry.Get()
			if g == nil || len(g.bars) == 0 {
				nothing(d)
				return
			}
			draw(d, g, func(bar barRect) {
				// The bar's own box: a gradient runs from its base to its
				// tip, whatever its length.
				lo, length := bar.span()
				area := core.Bounds{Origin: core.Point{X: core.Points(bar.start), Y: core.Points(lo)},
					Size: core.Size{Width: core.Points(bar.thick), Height: core.Points(length)}}
				if g.across {
					area = core.Bounds{Origin: core.Point{X: core.Points(lo), Y: core.Points(bar.start)},
						Size: core.Size{Width: core.Points(length), Height: core.Points(bar.thick)}}
				}
				d.FillBackgroundFromView(area)
			})
		}))
	}
	children = append(children, markCanvas(plot, room, part{}, func(d *core.Drawing, plot *plotState, _ float32) {
		initial := core.PaintCurrentColor
		if hasBackground {
			initial = core.PaintNone
		}
		g := geometry.Get()
		if g == nil {
			d.FillFromView(initial)
			return
		}
		var colors []core.Color
		hasColors := false
		if b.Color != nil {
			colors, hasColors = plot.categoryColors(b.Color())
		}
		if !hasColors {
			d.FillFromView(initial)
			draw(d, g, func(barRect) { d.Fill() })
			return
		}
		// Each bar its row's color.
		draw(d, g, func(bar barRect) {
			var c core.Color
			if bar.row < len(colors) {
				c = colors[bar.row]
			}
			d.FillFromView(paintOf(c, initial))
			d.Fill()
		})
	}))
	if b.Label != nil {
		// A label per row with a bar, made again when the rows the appear
		// staggers over or the bars' direction change.
		type labelKey struct {
			row, rows int
			axis      axis
		}
		children = append(children, core.Each(func() []labelKey {
			g := geometry.Get()
			if g == nil {
				return nil
			}
			out := make([]labelKey, len(g.bars))
			for i, bar := range g.bars {
				out[i] = labelKey{bar.row, g.rows, g.axis}
			}
			return out
		}, func(k labelKey) labelKey { return k }, func(cx *core.Scope, item func() labelKey, _ func() int) core.Node {
			k := item()
			return b.barLabel(cx, plot, geometry, k.row, k.rows, k.axis)
		}))
	}
	return markLayer(own, b.TestID, children...)
}

// addEntry adds e to the list while the current owner lives.
func addEntry(cx *core.Scope, bars *core.Signal[[]*barEntry], e *barEntry) {
	bars.Update(func(all []*barEntry) []*barEntry { return append(all[:len(all):len(all)], e) })
	cx.Cleanup(func() {
		bars.Update(func(all []*barEntry) []*barEntry {
			out := make([]*barEntry, 0, len(all))
			for _, o := range all {
				if o != e {
					out = append(out, o)
				}
			}
			return out
		})
	})
}

// barLabel is the label of the bar in row: a point with no size at the
// bar's value end, the text on the side away from the bar, centered across
// it; it rides the bar's end and fades in as the bar grows. It takes no
// pointer.
func (b Bar) barLabel(cx *core.Scope, plot *plotState, geometry *core.Memo[*barGeometry], row, rows int, on axis) core.Node {
	bar := func() (barRect, bool, bool) {
		g := geometry.Get()
		if g != nil {
			for _, r := range g.bars {
				if r.row == row {
					return r, g.across, true
				}
			}
		}
		return barRect{}, false, false
	}
	// While the bars grow, the label sits back toward the base by what is
	// left to grow: (origin - end) * (1 - grown).
	back, end := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	cx.Effect(func() {
		if r, _, ok := bar(); ok {
			back.Set(float64(r.origin - r.to))
			end.Set(float64(r.to))
		}
	})
	grown := plot.grown(row, rows, b.Stagger)
	ride := core.Mul(back, core.Sub(core.Constant(1), grown))
	// And it moves with the value axis's domain, where its bar's end goes.
	tween := plot.tweenOf(on)
	moved := core.Add(ride, tween.moved(core.Add(end, ride)))
	animated := &core.AnimatedStyle{Opacity: grown, TranslateY: moved}
	if on == axisX {
		animated = &core.AnimatedStyle{Opacity: grown, TranslateX: moved}
	}
	return core.View{
		Style: core.Style{Position: core.Absolute, Width: pts(0), Height: pts(0), Direction: core.Row, Animated: animated},
		StyleFunc: func() core.Style {
			r, across, ok := bar()
			if !ok {
				return core.Style{}
			}
			middle := r.start + r.thick/2
			if across {
				s := core.Style{Left: pts(r.to), Top: pts(middle), AlignItems: core.AlignCenter, JustifyContent: core.JustifyStart}
				if r.to < r.from {
					s.JustifyContent = core.JustifyEnd
				}
				return s
			}
			s := core.Style{Left: pts(middle), Top: pts(r.to), JustifyContent: core.JustifyCenter, AlignItems: core.AlignEnd}
			if r.to > r.from {
				s.AlignItems = core.AlignStart
			}
			return s
		},
		PointerEvents: core.PointerEventsNone,
		Children: []core.Node{core.Text{ContentFunc: func() string { return textAt(b.Label, row) }, Class: b.LabelClass,
			Style: b.LabelStyle, StyleFunc: b.LabelStyleFunc}},
	}
}
