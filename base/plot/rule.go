package plot

import (
	"math"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

// Rule is a series of a Plot drawn as a straight line per row along the
// value axis, as Vega-Lite's and Observable Plot's rule mark: from Base to
// the row's value, in the middle of the row's band on a band scale (across
// the plot on a band YScale), at the row's place along the x axis otherwise.
// A candle is a Bar from its open to its close over a Rule from its low to
// its high; a box plot's whiskers, error bars and lollipops' stems are
// rules. It strokes with its Stroke (inherited from the plot), the text
// color where none is set or with Color each line's category's. Its values
// and base count in the value scale's domain when the plot has none. With
// the plot's appear, its lines grow out of their start, one after another
// with Stagger; while the pointer is over the plot, the lines of other rows
// fade by HoverDim. It shows no hover dot.
type Rule struct {
	// Value is where each line ends; the plot's Y without. A row without a
	// value has no line.
	Value func() []float64
	// Base is where each line starts, 0 without it, and Bases a start per
	// row in its place, as a Bar's. They count in the scale's domain.
	Base  func() float64
	Bases func() []float64
	// Scale is the y scale its values go along; lines across the plot put
	// their values along x.
	Scale ValueScale
	// Stagger and HoverDim are a Bar's.
	Stagger, HoverDim float32
	// Color is the category of each line, which the plot's ColorScale gives
	// a color: its Stroke where nothing sets one.
	Color     func() []string
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	TestID    string
}

// ruleStem is a line as drawn: where its row is along the axis of the rows,
// and where its two ends go along the value axis.
type ruleStem struct {
	row          int
	at, from, to float32
}

// ruleGeometry is a Rule's lines, as drawn.
type ruleGeometry struct {
	across bool
	step   float32
	rows   int
	stems  []ruleStem
}

// spacing is the smallest distance between two of places, 0 with fewer than
// two.
func spacing(places []float32) float32 {
	closest := float32(math.Inf(1))
	for i := 1; i < len(places); i++ {
		if d := float32(math.Abs(float64(places[i] - places[i-1]))); !math.IsNaN(float64(d)) && d > 0 {
			closest = min(closest, d)
		}
	}
	if math.IsInf(float64(closest), 1) {
		return 0
	}
	return closest
}

// Render implements core.Node.
func (r Rule) Render(cx *core.Scope) core.Node {
	own := parts.Of(r.Class, r.Style, r.StyleFunc)
	plot := plotOf(cx, "Rule")
	if plot == nil {
		return markLayer(own, r.TestID)
	}
	nan := math.NaN()
	// Each row's start and end, NaN where it has either none.
	spans := core.NewMemoFunc(cx, func() [][2]float64 {
		values := plot.y
		if r.Value != nil {
			values = r.Value
		}
		v := values()
		var bases []float64
		if r.Bases != nil {
			bases = r.Bases()
		}
		base := 0.0
		if r.Base != nil {
			base = r.Base()
		}
		out := make([][2]float64, len(v))
		for row, end := range v {
			b := base
			if r.Bases != nil {
				b = nan
				if row < len(bases) {
					b = bases[row]
				}
			}
			out[row] = [2]float64{nan, nan}
			if isFinite(end) && isFinite(b) {
				out[row] = [2]float64{b, end}
			}
		}
		return out
	}, nil)
	along := func() axis {
		if plot.bandAxis.Get() == axisY {
			return axisX
		}
		return axisOf(r.Scale)
	}
	plot.valueMark(cx, core.NewMemo(cx, func() plotExtent {
		var e plotExtent
		for _, s := range spans.Get() {
			e = e.with(s[0]).with(s[1])
		}
		if r.Bases == nil {
			floor := 0.0
			if r.Base != nil {
				floor = r.Base()
			}
			e = e.with(floor)
		}
		return e
	}), along)
	geometry := core.NewMemoFunc(cx, func() *ruleGeometry {
		if !plot.sized() {
			return nil
		}
		xMap, yMap := plot.xMap.Get(), plot.yMap.Get()
		valuesY := plot.yMapOf(r.Scale)
		n := plot.rows()
		centers := func(b bandMap) []float32 {
			out := make([]float32, n)
			for i := range out {
				at, ok := b.center(i)
				if !ok {
					at = float32(nan)
				}
				out[i] = at
			}
			return out
		}
		var places []float32
		var step float32
		valueMap, across := valuesY, false
		if bx, ok := xMap.band(); ok {
			places, step = centers(bx), bx.stepSize()
		} else if by, ok := yMap.band(); ok {
			places, step, valueMap, across = centers(by), by.stepSize(), xMap, true
		} else {
			places = plot.xs.Get()
			step = spacing(places)
		}
		g := &ruleGeometry{across: across, step: step, rows: n}
		for row, s := range spans.Get() {
			if row >= len(places) || math.IsNaN(float64(places[row])) {
				continue
			}
			from, ok1 := valueMap.tick(s[0])
			to, ok2 := valueMap.tick(s[1])
			if ok1 && ok2 {
				g.stems = append(g.stems, ruleStem{row, places[row], from, to})
			}
		}
		return g
	}, nil)
	return markLayer(own, r.TestID, markCanvas(plot, room, part{}, func(d *core.Drawing, plot *plotState, _ float32) {
		d.LineWidthFromView().LineDashFromView()
		var g *ruleGeometry
		if plot != nil {
			g = geometry.Get()
		}
		if g == nil {
			d.StrokeFromView(core.PaintCurrentColor)
			return
		}
		appearing := plot.appearing.Get()
		var colors []core.Color
		hasColors := false
		if r.Color != nil {
			colors, hasColors = plot.categoryColors(r.Color())
		}
		if !hasColors {
			d.StrokeFromView(core.PaintCurrentColor)
		}
		for _, stem := range g.stems {
			if alpha, ok := plot.rowOpacity(r.HoverDim, g.across, stem.at, g.step); ok {
				setAlpha(d, alpha)
			}
			if hasColors {
				var c core.Color
				if stem.row < len(colors) {
					c = colors[stem.row]
				}
				d.StrokeFromView(paintOf(c, core.PaintCurrentColor))
			}
			// Growing from its start while the plot appears; through the
			// moving scales' transform while a domain moves.
			end := constNum(stem.to)
			if appearing {
				end = nodeNum(plot.grown(stem.row, g.rows, r.Stagger)).mul(constNum(stem.to - stem.from)).add(constNum(stem.from))
			}
			at, from := constNum(stem.at), constNum(stem.from)
			scale := r.Scale
			if g.across {
				scale = noScale
			}
			d.BeginPath()
			plot.traced(d, true, scale, func() {
				if g.across {
					moveTo(d, from, at)
					lineTo(d, end, at)
				} else {
					moveTo(d, at, from)
					lineTo(d, at, end)
				}
			})
			d.Stroke()
		}
	}))
}
