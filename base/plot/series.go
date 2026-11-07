package plot

import (
	"log"
	"math"
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// strokeRoom is how far a mark's canvas reaches past the plot's edges for
// its strokes: a stroke centered on an edge is half outside the plot, and
// a canvas draws nothing outside itself. Strokes up to twice this wide draw
// whole.
const strokeRoom = 8

// plotMark is a mark the plot knows: the extent of its values and the axis
// they go along, its points, where its hover dot is and how the app styles
// it; bars have no dot and no points.
type plotMark struct {
	key        int
	extent     *core.Memo[plotExtent]
	axis       func() axis
	points     *core.Memo[[]core.Point]
	dotX, dotY *core.AnimatedValue
	dot        *part
}

// near is the mark's point nearest to x along the x axis.
func (m *plotMark) near(x float32) (core.Point, bool) {
	if m.points == nil {
		return core.Point{}, false
	}
	var best core.Point
	ok, d := false, math.Inf(1)
	for _, q := range m.points.GetUntracked() {
		if e := math.Abs(float64(float32(q.X) - x)); e < d {
			best, ok, d = q, true, e
		}
	}
	return best, ok
}

// valueMark adds a mark with no points and no dot.
func (s *plotState) valueMark(cx *core.Scope, extent *core.Memo[plotExtent], along func() axis) *plotMark {
	m := &plotMark{extent: extent, axis: along, dotX: core.NewAnimatedValue(cx, 0), dotY: core.NewAnimatedValue(cx, 0)}
	s.add(cx, m)
	return m
}

// plotSeries is what a mark draws: its points in the plot's space, the
// rows without a value skipped, as gpui-base's shapes skip them, and for a
// band the points of its bottom edge, one under each.
type plotSeries struct {
	points *core.Memo[[]core.Point]
	lows   *core.Memo[[]core.Point]
	ys     func() []float64
	low    func() []float64
	scale  ValueScale
}

// newSeries is the series of the values y (the plot's without), placed
// along the x axis where the plot's rows go, or by x on rows of its own,
// along the y scale scale names, its bottom edge low (nil for none).
func newSeries(cx *core.Scope, plot *plotState, x, y, low func() []float64, scale ValueScale) plotSeries {
	ys := func() []float64 {
		if y != nil {
			return y()
		}
		return plot.y()
	}
	xs := func() []float32 {
		if x == nil {
			return plot.xs.Get()
		}
		own := x()
		return plot.xMap.Get().ticks(len(own), own)
	}
	type edges struct{ top, bottom []core.Point }
	all := core.NewMemoFunc(cx, func() edges {
		at, values, yMap := xs(), ys(), plot.yMapOf(scale)
		var lows []float64
		if low != nil {
			lows = low()
		}
		e := edges{top: make([]core.Point, 0, len(values))}
		for i, v := range values {
			if i >= len(at) {
				break
			}
			py, ok := yMap.tick(v)
			if !ok || math.IsNaN(float64(at[i])) {
				continue
			}
			if low != nil {
				if i >= len(lows) {
					continue
				}
				y0, ok := yMap.tick(lows[i])
				if !ok {
					continue
				}
				e.bottom = append(e.bottom, core.Point{X: core.Points(at[i]), Y: core.Points(y0)})
			}
			e.top = append(e.top, core.Point{X: core.Points(at[i]), Y: core.Points(py)})
		}
		return e
	}, func(a, b edges) bool { return slices.Equal(a.top, b.top) && slices.Equal(a.bottom, b.bottom) })
	return plotSeries{
		points: core.NewMemoFunc(cx, func() []core.Point { return all.Get().top }, slices.Equal),
		lows:   core.NewMemoFunc(cx, func() []core.Point { return all.Get().bottom }, slices.Equal),
		ys:     ys, low: low, scale: scale,
	}
}

// join adds the series to plot while the mark lives: its values, its bottom
// edge's and also's in its y scale's domain, and a hover dot.
func (s plotSeries) join(cx *core.Scope, plot *plotState, also func() (float64, bool), dot part) {
	extent := core.NewMemo(cx, func() plotExtent {
		e := extentOf(s.ys())
		if s.low != nil {
			e = e.join(extentOf(s.low()))
		}
		if v, ok := also(); ok {
			e = e.with(v)
		}
		return e
	})
	plot.add(cx, &plotMark{extent: extent, axis: func() axis { return axisOf(s.scale) }, points: s.points,
		dotX: core.NewAnimatedValue(cx, 0), dotY: core.NewAnimatedValue(cx, 0), dot: &dot})
}

// plotOf is the plot a mark is in, nil outside any.
func plotOf(cx *core.Scope, mark string) *plotState {
	p := plotKey.Use(cx)
	if p == nil {
		log.Printf("base: %s draws only inside a Plot", mark)
	}
	return p
}

// seriesColor is the color the color scale gives a series of rows: its
// first row's that has one, 0 for none.
func seriesColor(plot *plotState, categories func() []string) core.Color {
	if plot == nil || categories == nil {
		return 0
	}
	colors, _ := plot.categoryColors(categories())
	for _, c := range colors {
		if c != 0 {
			return c
		}
	}
	return 0
}

// paintOf is the paint of a mark's shape where nothing sets one: c, the
// color its category takes, in place of the text color; initial otherwise.
func paintOf(c core.Color, initial core.Paint) core.Paint {
	if c != 0 && initial == core.PaintCurrentColor {
		return core.PaintColor(c)
	}
	return initial
}

// markLayer is a mark's view: it fills the plot, takes no pointer (the
// plot follows it) and holds the mark's paint style.
func markLayer(p part, testID string, children ...core.Node) core.Node {
	return core.View{
		TestID:        testID,
		Class:         p.Class,
		Style:         core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)}.Merge(p.Style),
		StyleFunc:     p.Fn,
		PointerEvents: core.PointerEventsNone,
		Children:      children,
	}
}

// markCanvas is a mark's canvas: the plot's area and bleed() points around
// it, for half its strokes and its curves' swing past the plot's edges,
// its drawing in the plot's coordinates. Before the plot's first layout
// draw gets a nil plot. own is the canvas's own part, over its place.
func markCanvas(plot *plotState, bleed func() float32, own part,
	draw func(d *core.Drawing, plot *plotState, bleed float32)) core.Canvas {
	return core.Canvas{
		Class: own.Class,
		Style: core.Style{Position: core.Absolute},
		StyleFunc: func() core.Style {
			b := core.Pt(core.Points(-bleed()))
			s := core.Style{Left: b, Top: b, Right: b, Bottom: b}.Merge(own.Style)
			if own.Fn != nil {
				s = s.Merge(own.Fn())
			}
			return s
		},
		Draw: func(d *core.Drawing, _ core.Size) {
			if plot == nil || !plot.sized() {
				draw(d, nil, 0)
				return
			}
			b := bleed()
			d.Translate(b, b)
			draw(d, plot, b)
		},
	}
}

// room is the bleed of a mark that keeps to the plot.
func room() float32 { return strokeRoom }

// nothing fills with its view's background nowhere: a canvas that takes a
// background draws this from its first drawing on, so its view never draws
// the background as a box.
func nothing(d *core.Drawing) { d.BeginPath().FillBackgroundFromView(core.Bounds{}) }

// grown is how far row of rows has grown while the plot appears, the rows
// starting one after another over stagger of the appear.
func (s *plotState) grown(row, rows int, stagger float32) core.AnimatedNode {
	return s.appear.staggered(row, rows, stagger)
}

// rowOpacity is the opacity of a row's marks at center along the axis of the
// rows (across: down the y axis), rows step apart: dim less away from the
// hover's row, by the hover's distance in rows, as far as the hover has
// faded in. ok is false where nothing dims.
func (s *plotState) rowOpacity(dim float32, across bool, center, step float32) (num, bool) {
	dim = min(max(dim, 0), 1)
	if dim <= 0 || step <= 0 || s.noHover {
		return num{}, false
	}
	at := s.crossX
	if across {
		at = s.crossY
	}
	c, st, f := float64(center), float64(step), float64(dim)
	away := core.Interpolate(at, core.Interpolation{Input: []float64{c - st, c, c + st}, Output: []float64{f, 0, f}})
	return nodeNum(core.Sub(core.Constant(1), core.Mul(away, s.fade))), true
}

// indexOpacity is the opacity of row's mark, dim less away from the hovered
// row, whose index at holds: the rows' order is no path to glide along.
func (s *plotState) indexOpacity(at *core.AnimatedValue, dim float32, row int) num {
	c, f := float64(row), float64(dim)
	away := core.Interpolate(at, core.Interpolation{Input: []float64{c - 1, c, c + 1}, Output: []float64{f, 0, f}})
	return nodeNum(core.Sub(core.Constant(1), core.Mul(away, s.fade)))
}

// hoveredIndex follows the hovered row on an animated value, set at once.
func (s *plotState) hoveredIndex(cx *core.Scope) *core.AnimatedValue {
	at := core.NewAnimatedValue(cx, -2)
	cx.Effect(func() {
		if row := s.hovered.Get(); row >= 0 {
			at.Set(float64(row))
		}
	})
	return at
}

// hoverPlace is where the view over what the pointer or the keys are on goes: its
// place follows the row at once, gliding on the pointer spring when the keys
// step, and it shows while the row is one of this mark's. find places a row.
type hoverPlace struct {
	x, y, mine *core.AnimatedValue
	bounds     *core.Signal[core.Bounds]
}

func (s *plotState) hoverPlace(cx *core.Scope, find func(row int) (core.Bounds, bool)) hoverPlace {
	h := hoverPlace{x: core.NewAnimatedValue(cx, 0), y: core.NewAnimatedValue(cx, 0), mine: core.NewAnimatedValue(cx, 0),
		bounds: core.NewSignal(cx, core.Bounds{})}
	on := -1
	cx.Effect(func() {
		row := s.hovered.Get()
		found, ok := core.Bounds{}, false
		if row >= 0 {
			found, ok = find(row)
		}
		if !ok {
			if row >= 0 {
				h.mine.Set(0)
			}
			on = -1
			return
		}
		h.mine.Set(1)
		glide := on >= 0 && on != row && s.keyed && !s.still()
		on = row
		s.springTo(h.x, float32(found.Origin.X), glide)
		s.springTo(h.y, float32(found.Origin.Y), glide)
		h.bounds.Set(found)
	})
	return h
}

// box is the hover's box, as big as what it marks, over it; it takes no
// pointer.
func (h hoverPlace) box(s *plotState, class string, style core.Style, fn func() core.Style) core.Node {
	inner := core.View{Class: class, Style: core.Style{FlexGrow: 1}.Merge(style), StyleFunc: fn}
	return core.View{
		Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0),
			Animated: &core.AnimatedStyle{TranslateX: h.x, TranslateY: h.y, Opacity: core.Mul(s.fade, h.mine)}},
		StyleFunc: func() core.Style {
			b := h.bounds.Get()
			return core.Style{Width: core.Pt(b.Size.Width), Height: core.Pt(b.Size.Height)}
		},
		PointerEvents: core.PointerEventsNone,
		Children:      []core.Node{inner},
	}
}

// measure keeps a view's size in into.
func measure(into *core.Signal[core.Size]) func(core.Bounds) {
	return func(b core.Bounds) { into.Set(b.Size) }
}

// textAt is the text of texts at i.
func textAt(texts func() []string, i int) string {
	if all := texts(); i >= 0 && i < len(all) {
		return all[i]
	}
	return ""
}
