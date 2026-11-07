package commitgraph

import (
	"math"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
)

// The graph's size by default, shared so the rows of a list line up.
const (
	// RowHeight is a row's height, which the lines span.
	RowHeight core.Points = 26
	// LaneWidth is a column's width.
	LaneWidth core.Points = 12
)

const (
	// mergeSize is a merge button's diameter.
	mergeSize = 18
	// inset is the distance from the graph's edge to its first and last
	// columns' centers, which leaves room for a collapsed merge's ring.
	inset = 10
)

// Graph draws one commit's row of the revision graph: lines cross the row
// and curve into and out of the commit, a ring in its line's color. Its
// width is its own row's columns, so a busy stretch of history does not push
// every subject away from its graph; give each row of a list one Graph and
// no gap between them, so the lines meet.
//
// A commit the History knows as a merge is a disc with a chevron, pointing
// down while expanded and right, inside a ring, while collapsed, and a
// button that toggles it. What the graph cuts out of itself shows the row
// behind it, selected or not.
type Graph struct {
	History *History
	// Item is the row's commit, read as a signal.
	Item func() Item
	// Height is the row's, RowHeight when 0, and Lane a column's width,
	// LaneWidth when 0.
	Height, Lane core.Points
	// Color returns a line's color, the text's when nil.
	Color func(line int) core.Color
	// Class, Style and StyleFunc are the graph's.
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	// MergeClass, MergeStyle and MergeStyleFunc are the merge button's,
	// such as its hover.
	MergeClass     string
	MergeStyle     core.Style
	MergeStyleFunc func() core.Style
}

// Render implements core.Node.
func (g Graph) Render(cx *core.Scope) core.Node {
	height, lane := g.Height, g.Lane
	if height == 0 {
		height = RowHeight
	}
	if lane == 0 {
		lane = LaneWidth
	}
	env := cx.Env()
	item := g.Item
	if item == nil {
		item = func() Item { return Item{} }
	}
	merge := func() bool {
		i := item()
		return i.Merge && i.Row.Column >= 0
	}
	label := func() string {
		if g.History.Collapsed(item().Hash) {
			return locales.ExpandMerge(cx)
		}
		return locales.CollapseMerge(cx)
	}
	return core.View{Class: g.Class, Style: core.Style{Height: core.Pt(height), FlexShrink: 0, Overflow: core.Hidden}.Merge(g.Style),
		StyleFunc: func() core.Style {
			s := core.Style{Width: core.Pt(core.Points(max(item().Row.Columns-1, 0))*lane + 2*inset)}
			if g.StyleFunc != nil {
				s = s.Merge(g.StyleFunc())
			}
			return s
		},
		Children: []core.Node{
			core.Canvas{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)},
				Draw: func(d *core.Drawing, size core.Size) {
					i := item()
					collapsed := merge() && g.History.Collapsed(i.Hash)
					painter{d: d, size: size, lane: float32(lane), rtl: env.RTL(), color: g.Color}.row(i.Row, merge(), collapsed)
				}},
			core.Show{When: merge, Then: func(cx *core.Scope) core.Node {
				return base.Button{LabelFunc: label, NoFocus: true, Expanded: func() bool { return !g.History.Collapsed(item().Hash) },
					OnPress: func(core.PressEvent) { g.History.Toggle(core.Untracked(cx, item).Hash) },
					Class:   g.MergeClass, Style: g.MergeStyle,
					StyleFunc: func() core.Style {
						at := core.Pt(inset + core.Points(item().Row.Column)*lane - mergeSize/2)
						s := core.Style{Position: core.Absolute, Top: core.Pt((height - mergeSize) / 2), Left: at, Width: core.Pt(mergeSize), Height: core.Pt(mergeSize), CornerRadius: mergeSize / 2}
						if g.MergeStyleFunc != nil {
							s = s.Merge(g.MergeStyleFunc())
						}
						return s
					}}
			}},
		}}
}

// painter draws a graph's row into a drawing.
type painter struct {
	d     *core.Drawing
	size  core.Size
	lane  float32
	rtl   bool
	color func(line int) core.Color
}

// x is the center of column, which right to left counts from the right.
func (p painter) x(column int) float32 {
	x := inset + float32(column)*p.lane
	if p.rtl {
		return float32(p.size.Width) - x
	}
	return x
}

// paint has the drawing fill and stroke with line's color.
func (p painter) paint(line int) {
	if p.color == nil {
		p.d.FillFromView(core.PaintCurrentColor).StrokeFromView(core.PaintCurrentColor)
		return
	}
	c := p.color(line)
	p.d.SetFillColor(c).SetStrokeColor(c)
}

// row draws the lines of row and its commit's node.
func (p painter) row(row Row, merge, collapsed bool) {
	d, height := p.d, float32(p.size.Height)
	top, mid, bottom := float32(0), height/2, height
	d.SetLineWidth(1.5)
	for _, line := range row.Lines {
		p.paint(line.ID)
		d.BeginPath()
		switch {
		case line.Top < 0:
			p.bend(p.x(line.Column), mid, p.x(line.Bottom), bottom, p.x(line.Bottom), mid)
		case line.Bottom < 0:
			p.bend(p.x(line.Top), top, p.x(line.Column), mid, p.x(line.Top), mid)
		default:
			p.join(p.x(line.Top), top, p.x(line.Bottom), bottom)
		}
		d.Stroke()
	}
	if row.Column < 0 {
		return
	}
	p.paint(row.Line)
	p.node(p.x(row.Column), mid, merge, collapsed)
}

// join is a straight segment within a column, or a curve between two, so
// branches and merges read as smooth joins.
func (p painter) join(fromX, fromY, toX, toY float32) {
	p.d.MoveTo(fromX, fromY)
	if fromX == toX {
		p.d.LineTo(toX, toY)
		return
	}
	p.d.CubicTo(fromX, toY-5, toX, fromY+5, toX, toY)
}

// bend is a line that starts or ends at a commit: across columns it bends
// through the corner, so it meets the commit's side at its middle rather
// than curving out from under it.
func (p painter) bend(fromX, fromY, toX, toY, cornerX, cornerY float32) {
	if fromX == toX {
		p.join(fromX, fromY, toX, toY)
		return
	}
	p.d.MoveTo(fromX, fromY).QuadTo(cornerX, cornerY, toX, toY)
}

// disc fills a circle.
func (p painter) disc(x, y, r float32) {
	p.d.BeginPath().Arc(x, y, r, 0, 2*math.Pi, false).Fill()
}

// clear cuts what draw paints out of the graph, showing the row behind it.
func (p painter) clear(draw func()) {
	opaque := core.RGB(0)
	p.d.Save().SetComposite(core.CompositeDestinationOut).SetFillColor(opaque).SetStrokeColor(opaque)
	draw()
	p.d.Restore()
}

// node draws a commit: a ring, or a merge's disc with a chevron cut out of
// it, in a ring of its own for the commits a collapsed merge holds.
func (p painter) node(x, y float32, merge, collapsed bool) {
	d := p.d
	if !merge {
		p.disc(x, y, 4)
		p.clear(func() { p.disc(x, y, 1.75) })
		return
	}
	radius := float32(6.5)
	if collapsed {
		radius = 5.5
		p.clear(func() { p.disc(x, y, 9.5) })
		d.SetLineWidth(1.25).BeginPath().Arc(x, y, 8.5, 0, 2*math.Pi, false).Stroke()
	}
	p.disc(x, y, radius)
	p.clear(func() {
		d.SetLineWidth(1.5).BeginPath()
		if collapsed {
			side := float32(1)
			if p.rtl {
				side = -1
			}
			d.MoveTo(x-side, y-2.5).LineTo(x+1.5*side, y).LineTo(x-side, y+2.5)
		} else {
			d.MoveTo(x-2.75, y-1).LineTo(x, y+1.75).LineTo(x+2.75, y-1)
		}
		d.Stroke()
	})
}
