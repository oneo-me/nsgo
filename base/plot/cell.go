package plot

import (
	"cmp"
	"math"
	"slices"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// cellEntry is a cell the plot knows: the categories it names along each
// axis, which make the bands of a band scale without a domain, and its
// values, which the ramps of the plot's ColorScale spread over.
type cellEntry struct {
	x, y   func() []string
	values func() []float64
}

// categories is what its rows name along a, none along the second y scale.
func (c *cellEntry) categories(a axis) ([]string, bool) {
	switch a {
	case axisX:
		return c.x(), c.x != nil
	case axisY:
		return c.y(), c.y != nil
	}
	return nil, false
}

// cellSpot is a row's cell as the app sets it: its box as shares of the
// plot's width and height, x NaN for no cell, so a new size moves nothing
// and new bands move the cells, and its color, 0 for the mark's own Fill.
type cellSpot struct {
	x, y, w, h float32
	color      core.Color
}

func (s cellSpot) shown() bool { return !math.IsNaN(float64(s.x)) && !math.IsNaN(float64(s.y)) }

// scaled is the same box shrunk to t of its size about its middle.
func (s cellSpot) scaled(t float32) cellSpot {
	s.x, s.y = s.x+s.w*(1-t)/2, s.y+s.h*(1-t)/2
	s.w, s.h = s.w*t, s.h*t
	return s
}

// cellPose is the cells as the app sets them, one per row.
type cellPose []cellSpot

func (p cellPose) same(o cellPose) bool { return slices.Equal(p, o) }

func (p cellPose) spot(i int) cellSpot {
	if i < len(p) {
		return p[i]
	}
	nan := float32(math.NaN())
	return cellSpot{x: nan, y: nan}
}

func (p cellPose) lerp(to cellPose, t float32) cellPose {
	out := make(cellPose, max(len(p), len(to)))
	mix := func(a, b float32) float32 { return a + (b-a)*t }
	for i := range out {
		a, b := p.spot(i), to.spot(i)
		switch {
		case a.shown() && b.shown():
			c := b.color
			if a.color != 0 && b.color != 0 {
				c = mixOKLab(a.color, b.color, t)
			}
			out[i] = cellSpot{mix(a.x, b.x), mix(a.y, b.y), mix(a.w, b.w), mix(a.h, b.h), c}
		case b.shown():
			out[i] = b.scaled(t)
		case a.shown():
			out[i] = a.scaled(1 - t)
		default:
			out[i] = a
		}
	}
	return out
}

// cellPlaced is a cell as drawn at rest: its row and its box, in points.
type cellPlaced struct {
	row    int
	bounds core.Bounds
}

func (c cellPlaced) center() (float32, float32) {
	b := c.bounds
	return float32(b.Origin.X + b.Size.Width/2), float32(b.Origin.Y + b.Size.Height/2)
}

// cellSpan is the span along an axis of length points of a band at start
// that is width wide, shares of the axis, narrowed to limit points in the
// middle of its place.
func cellSpan(start, width, length, limit float32) (float32, float32) {
	at, w := start*length, width*length
	if limit > 0 && w > limit {
		return at + (w-limit)/2, limit
	}
	return at, w
}

// placedCells is the boxes of the shown cells of p on a plot of size, the
// bands no wider than limits along each axis.
func placedCells(p cellPose, size core.Size, limits [2]float32) []cellPlaced {
	out := make([]cellPlaced, 0, len(p))
	for row, s := range p {
		if !s.shown() {
			continue
		}
		x, w := cellSpan(s.x, s.w, float32(size.Width), limits[0])
		y, h := cellSpan(s.y, s.h, float32(size.Height), limits[1])
		out = append(out, cellPlaced{row, core.Bounds{Origin: core.Point{X: core.Points(x), Y: core.Points(y)},
			Size: core.Size{Width: core.Points(w), Height: core.Points(h)}}})
	}
	return out
}

// boxDistance is the distance from at to the box, 0 inside.
func boxDistance(b core.Bounds, at core.Point) float32 {
	dx := max(float64(b.Origin.X-at.X), float64(at.X-b.Origin.X-b.Size.Width), 0)
	dy := max(float64(b.Origin.Y-at.Y), float64(at.Y-b.Origin.Y-b.Size.Height), 0)
	return float32(math.Hypot(dx, dy))
}

// cellAt is the cell under at, or the nearest within reach of it, so the
// hover stays on the grid as the pointer crosses the gaps.
func cellAt(cells []cellPlaced, at core.Point, reach float32) (int, bool) {
	found, best := -1, float32(math.Inf(1))
	for _, c := range cells {
		if d := boxDistance(c.bounds, at); d <= reach && d < best {
			found, best = c.row, d
		}
	}
	return found, found >= 0
}

// cellStep is the cell the keys step to from from across cells on the
// screen, as WAI-ARIA's data grid: the arrows to the nearest cell on their
// side in the same line or column, Home and End to the first and last cell
// of the line; from none, the top left cell, the bottom right one for Left,
// Up and End. At an edge the hover stays.
func cellStep(cells []cellPlaced, from int, step keyStep) (int, bool) {
	if len(cells) == 0 {
		return 0, false
	}
	order := func(a, b cellPlaced) int {
		ax, ay := a.center()
		bx, by := b.center()
		return cmp.Or(cmp.Compare(ay, by), cmp.Compare(ax, bx))
	}
	i := slices.IndexFunc(cells, func(c cellPlaced) bool { return c.row == from })
	if i < 0 {
		if step == stepPrevious || step == stepUp || step == stepLast {
			return slices.MaxFunc(cells, order).row, true
		}
		return slices.MinFunc(cells, order).row, true
	}
	here := cells[i]
	cx, cy := here.center()
	halfW, halfH := max(float32(here.bounds.Size.Width)/2, 0.5), max(float32(here.bounds.Size.Height)/2, 0.5)
	found, best := -1, float32(0)
	consider := func(row int, along float32, better func(a, b float32) bool) {
		if found < 0 || better(along, best) {
			found, best = row, along
		}
	}
	less := func(a, b float32) bool { return a < b }
	more := func(a, b float32) bool { return a > b }
	for _, c := range cells {
		x, y := c.center()
		inLine := float32(math.Abs(float64(y-cy))) < halfH
		inColumn := float32(math.Abs(float64(x-cx))) < halfW
		switch {
		case step == stepNext && inLine && x > cx:
			consider(c.row, x, less)
		case step == stepPrevious && inLine && x < cx:
			consider(c.row, x, more)
		case step == stepDown && inColumn && y > cy:
			consider(c.row, y, less)
		case step == stepUp && inColumn && y < cy:
			consider(c.row, y, more)
		case step == stepFirst && inLine:
			consider(c.row, x, less)
		case step == stepLast && inLine:
			consider(c.row, x, more)
		}
	}
	if found < 0 {
		return here.row, true
	}
	return found, true
}

// Cell is a series of a Plot drawn as a rectangle per row, as Observable
// Plot's cell and Vega-Lite's rect over two band scales: heatmaps,
// correlation matrices, calendars of monthly returns. It goes in the band of
// its X category along the plot's x scale and of its Y category along its y
// scale, both band scales (the cells name their bands where a scale has no
// domain), filling the two bands, so the scales' paddings are the gaps
// between and around the cells. Its color is its row's of the plot's
// ColorScale: of its Value along a ramp, of its Color category, or its Fill
// where the app sets one, the text color where nothing does. Label writes a
// text in each cell, shown where it fits. The pointer finds the cell under
// it, or the nearest across a gap, which the plot reports and marks with the
// hover cell; the keys step across the grid and screen readers through the
// rows. With the plot's appear, its cells grow from their middles, one after
// another with Stagger; with Motion.Update a cell's color glides to its new
// one, new rows' cells grow in, those of rows that are gone shrink away and
// cells move to new bands; while one is hovered, the others fade by HoverDim.
type Cell struct {
	// X and Y are the column and line of each row: the row's cell goes in
	// the band of that category. A row without one has no cell.
	X, Y func() []string
	// Value is the number of each cell, which a ramp of the plot's
	// ColorScale makes its color; a row without one then has no cell, or the
	// scale's Unknown color.
	Value func() []float64
	// Color is the category of each cell, which an ordinal ColorScale gives
	// a color.
	Color func() []string
	// Corners rounds its cells' corners; square by default.
	Corners Corners
	// Stagger and HoverDim are a Bar's.
	Stagger, HoverDim float32
	// Label is the text in each cell, centered, shown only where its
	// laid-out size fits inside the cell. LabelStyle styles it, and
	// DarkLabelStyle merges over it on a cell whose color is dark, where
	// light text reads better, as DarkLabelClass follows LabelClass.
	Label          func() []string
	LabelClass     string
	LabelStyle     core.Style
	LabelStyleFunc func() core.Style
	DarkLabelClass string
	DarkLabelStyle core.Style
	// HoverCellStyle styles a view as big as the cell under the pointer or
	// the keys, over it; none without a class or style.
	HoverCellClass     string
	HoverCellStyle     core.Style
	HoverCellStyleFunc func() core.Style
	Class              string
	Style              core.Style
	StyleFunc          func() core.Style
	TestID             string
}

// categoryIndices is the index of each of texts among categories, -1 for
// none.
func categoryIndices(texts, categories []string) []int {
	out := make([]int, len(texts))
	for i, t := range texts {
		out[i] = -1
		if t != "" {
			out[i] = slices.Index(categories, t)
		}
	}
	return out
}

// Render implements core.Node.
func (c Cell) Render(cx *core.Scope) core.Node {
	own := parts.Of(c.Class, c.Style, c.StyleFunc)
	plot := plotOf(cx, "Cell")
	if plot == nil {
		return markLayer(own, c.TestID)
	}
	empty := func() []string { return nil }
	xs, ys := c.X, c.Y
	if xs == nil {
		xs = empty
	}
	if ys == nil {
		ys = empty
	}
	values := func() []float64 {
		if c.Value == nil {
			return nil
		}
		return c.Value()
	}
	// The plot's bands are made of the cells' categories.
	entry := &cellEntry{x: xs, y: ys, values: values}
	plot.cells.Update(func(all []*cellEntry) []*cellEntry { return append(slices.Clip(all), entry) })
	cx.Cleanup(func() {
		plot.cells.Update(func(all []*cellEntry) []*cellEntry {
			return slices.DeleteFunc(slices.Clone(all), func(o *cellEntry) bool { return o == entry })
		})
	})
	moves := newMoves(cx, plot, false, func() (cellPose, bool) {
		xb, okX := plot.xMap.Get().band()
		yb, okY := plot.yMap.Get().band()
		xc, yc := plot.xCategories.Get(), plot.yCategories.Get()
		if !okX || !okY || xc == nil || yc == nil {
			return nil, true
		}
		xb, yb = xb.unit(), yb.unit()
		xOf, yOf := categoryIndices(xs(), xc), categoryIndices(ys(), yc)
		var categories, shades []core.Color
		if c.Color != nil {
			categories, _ = plot.categoryColors(c.Color())
		}
		ramp := plot.isRamp()
		if c.Value != nil {
			shades, _ = plot.shades(values(), plot.cellExtent.Get())
		}
		nan := float32(math.NaN())
		out := make(cellPose, min(len(xOf), len(yOf)))
		for row := range out {
			out[row] = cellSpot{x: nan, y: nan}
			x, ok1 := xb.startOf(xOf[row])
			y, ok2 := yb.startOf(yOf[row])
			if !ok1 || !ok2 {
				continue
			}
			var color core.Color
			if row < len(categories) {
				color = categories[row]
			}
			if color == 0 && row < len(shades) {
				color = shades[row]
			}
			if ramp && color == 0 {
				continue
			}
			out[row] = cellSpot{x, y, xb.width(), yb.width(), color}
		}
		return out, true
	})
	limits := func() [2]float32 {
		var out [2]float32
		if b, ok := plot.xMap.Get().band(); ok {
			out[0] = b.maxWidth
		}
		if b, ok := plot.yMap.Get().band(); ok {
			out[1] = b.maxWidth
		}
		return out
	}
	// Where the cells are at rest: what the pointer finds, where the hover
	// cell and the labels go.
	rest := core.NewMemoFunc(cx, func() []cellPlaced {
		if !plot.sized() {
			return nil
		}
		return placedCells(moves.now().to, plot.size.Get(), limits())
	}, nil)
	// The gap between neighbouring cells, which the pointer crosses without
	// leaving the grid.
	reachOf := func() float32 {
		gap := func(m axisMap) float32 {
			if b, ok := m.band(); ok {
				return max(b.stepSize()-b.width(), 0)
			}
			return 0
		}
		return max(gap(plot.xMap.GetUntracked()), gap(plot.yMap.GetUntracked())) + 1
	}
	find := func(cells []cellPlaced, row int) (cellPlaced, bool) {
		i := slices.IndexFunc(cells, func(c cellPlaced) bool { return c.row == row })
		if i < 0 {
			return cellPlaced{}, false
		}
		return cells[i], true
	}
	plot.addHit(cx, &plotHit{
		at: func(at core.Point) (int, bool) { return cellAt(rest.GetUntracked(), at, reachOf()) },
		center: func(row int) (core.Point, bool) {
			p, ok := find(rest.GetUntracked(), row)
			x, y := p.center()
			return core.Point{X: core.Points(x), Y: core.Points(y)}, ok
		},
		grid: func(from int, step keyStep) (int, bool) { return cellStep(rest.GetUntracked(), from, step) },
	})
	hoveredAt := plot.hoveredIndex(cx)
	dim := min(max(c.HoverDim, 0), 1)
	if plot.noHover {
		dim = 0
	}
	canvas := markCanvas(plot, room, part{}, func(d *core.Drawing, plot *plotState, _ float32) {
		d.StrokeFromView(core.PaintNone).LineWidthFromView().LineDashFromView()
		if plot == nil {
			return
		}
		m := moves.now()
		size, limit := plot.size.Get(), limits()
		count := len(m.to)
		if m.moving {
			count = max(count, len(m.from))
		}
		appearing := plot.appearing.Get()
		box := func(s cellSpot) [4]float32 {
			x, w := cellSpan(s.x, s.w, float32(size.Width), limit[0])
			y, h := cellSpan(s.y, s.h, float32(size.Height), limit[1])
			return [4]float32{x, y, w, h}
		}
		paint := func(c core.Color) core.Paint { return paintOf(c, core.PaintCurrentColor) }
		for row := range count {
			target := m.to.spot(row)
			var rect [4]num
			var old core.Color
			fades := false
			var q core.AnimatedNode
			if !m.moving {
				if !target.shown() {
					continue
				}
				b := box(target)
				rect = [4]num{constNum(b[0]), constNum(b[1]), constNum(b[2]), constNum(b[3])}
			} else {
				source := m.from.spot(row)
				q = moves.progress
				a, b := source, target
				switch {
				case source.shown() && target.shown():
				case target.shown():
					a = target.scaled(0)
				case source.shown():
					b = source.scaled(0)
				default:
					continue
				}
				ra, rb := box(a), box(b)
				for i := range rect {
					rect[i] = lerpNum(q, ra[i], rb[i])
				}
				if source.shown() && target.shown() && a.color != b.color {
					old, fades = a.color, true
				}
			}
			color := target.color
			if !target.shown() && m.moving {
				color = m.from.spot(row).color
			}
			if appearing {
				g := nodeNum(plot.grown(row, count, c.Stagger))
				left := g.mul(constNum(-0.5)).add(constNum(0.5))
				rect = [4]num{rect[0].add(rect[2].mul(left)), rect[1].add(rect[3].mul(left)), rect[2].mul(g), rect[3].mul(g)}
			}
			alpha, dims := num{}, dim > 0
			if dims {
				alpha = plot.indexOpacity(hoveredAt, dim, row)
			}
			if fades {
				// A cross-fade: the old color at what is left of the move,
				// the new one added at what has gone, so a cell is old*(1-t)
				// + new*t, its opacity too: translucent colors stay as
				// translucent as they are, where one over the other would
				// thicken.
				left := nodeNum(moves.left())
				if dims {
					left = alpha.mul(left)
				}
				setAlpha(d, left)
				d.FillFromView(paint(old)).BeginPath()
				rectPath(d, rect[0], rect[1], rect[2], rect[3], c.Corners)
				d.Fill()
				d.SetComposite(core.CompositeLighter)
			}
			switch {
			case dims && fades:
				setAlpha(d, alpha.mul(nodeNum(q)))
			case dims:
				setAlpha(d, alpha)
			case fades:
				setAlpha(d, nodeNum(q))
			default:
				d.SetAlpha(1)
			}
			d.FillFromView(paint(color)).BeginPath()
			rectPath(d, rect[0], rect[1], rect[2], rect[3], c.Corners)
			d.Fill()
			if fades {
				d.SetComposite(core.CompositeSourceOver)
			}
			d.Stroke()
		}
	})
	children := []core.Node{canvas}
	if c.Label != nil {
		children = append(children, core.Each(func() []int {
			cells := rest.Get()
			out := make([]int, len(cells))
			for i, p := range cells {
				out[i] = p.row
			}
			return out
		}, func(row int) int { return row }, func(cx *core.Scope, item func() int, _ func() int) core.Node {
			row := item()
			opacity := plot.grown(row, core.Untracked(cx, plot.rows), c.Stagger)
			if dim > 0 {
				opacity = core.Mul(opacity, plot.indexOpacity(hoveredAt, dim, row).n)
			}
			bounds := func() (core.Bounds, bool) {
				p, ok := find(rest.Get(), row)
				return p.bounds, ok
			}
			dark := func() bool {
				s := moves.now().to.spot(row)
				return s.color != 0 && isDark(s.color)
			}
			return c.cellLabel(cx, row, bounds, dark, opacity)
		}))
	}
	if hover := parts.Of(c.HoverCellClass, c.HoverCellStyle, c.HoverCellStyleFunc); !plot.noHover && hover.Set() {
		h := plot.hoverPlace(cx, func(row int) (core.Bounds, bool) {
			p, ok := find(rest.Get(), row)
			return p.bounds, ok
		})
		children = append(children, h.box(plot, hover.Class, hover.Style, hover.Fn))
	}
	return markLayer(own, c.TestID, children...)
}

// cellLabel is the label of a cell: a point with no size at its middle, the
// text around it at its own size on one line, shown while that size fits
// inside the cell.
func (c Cell) cellLabel(cx *core.Scope, row int, bounds func() (core.Bounds, bool), dark func() bool, opacity core.AnimatedNode) core.Node {
	measured := core.NewSignal(cx, core.Size{})
	fits := core.NewMemo(cx, func() bool {
		b, ok := bounds()
		m := measured.Get()
		return ok && m.Width > 0 && m.Width <= b.Size.Width && m.Height <= b.Size.Height
	})
	return core.View{
		Style: core.Style{Position: core.Absolute, Width: pts(0), Height: pts(0), Direction: core.Row, AlignItems: core.AlignCenter,
			JustifyContent: core.JustifyCenter, Animated: &core.AnimatedStyle{Opacity: opacity}},
		StyleFunc: func() core.Style {
			b, ok := bounds()
			if !ok {
				return core.Style{Display: core.DisplayNone}
			}
			return core.Style{Left: core.Pt(b.Origin.X + b.Size.Width/2), Top: core.Pt(b.Origin.Y + b.Size.Height/2)}
		},
		PointerEvents: core.PointerEventsNone,
		Children: []core.Node{core.Text{
			ContentFunc: func() string { return textAt(c.Label, row) },
			OnLayout:    measure(measured),
			Class:       c.LabelClass,
			ClassFunc: func() string {
				if dark() {
					return c.DarkLabelClass
				}
				return ""
			},
			Style: c.LabelStyle,
			StyleFunc: func() core.Style {
				var s core.Style
				if c.LabelStyleFunc != nil {
					s = c.LabelStyleFunc()
				}
				if dark() {
					s = s.Merge(c.DarkLabelStyle)
				}
				if !fits.Get() {
					s = s.Merge(core.Style{Opacity: core.Alpha(0)})
				}
				return s
			},
		}},
	}
}
