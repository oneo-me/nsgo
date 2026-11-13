package datatable

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/base/locales"
	"github.com/nsgo-dev/nsgo/core"
)

// The table's view, in layers:
//
//   - the body, taking no pointer: the cells of the rows and columns in
//     view and a reach around them, placed where the table last scrolled
//     to, in three regions: the row numbers, the fixed columns (which move
//     with the rows only) and the scrolling columns;
//   - the hit layer over it: a vertical scroll view of a horizontal one of
//     an empty box of the content's size, scaled past a few million
//     points. The platforms scroll it, its box takes presses, hovers and
//     the context menu, and it draws nothing but its scroll bar, which so
//     lies over the rows' backgrounds;
//   - the header above them: group labels and titles in the same regions,
//     the columns' resize handles and the drop line of a column moved;
//   - on desktops, a horizontal scroll bar over the body's bottom edge
//     that moves the table's horizontal scroll view, whose own bar lies at
//     the end of a content as tall as all the rows.

const (
	// stripHeight is the desktops' horizontal scroll bar band.
	stripHeight = 12
	// dropLine is the drop line's width.
	dropLine = 2
	// stripEcho is how long after a move of one of the two horizontal
	// scroll views the other's moves are its echoes.
	stripEcho = 250 * time.Millisecond
)

// Render implements core.Node.
func (p DataTable) Render(cx *core.Scope) core.Node {
	t := newDataTable(cx, p)
	font := t.font()
	root := core.Style{Overflow: core.Hidden, FontSize: font.Size, FontFamily: font.Family}.Merge(p.Style)
	return core.View{
		Role: core.RoleGrid, Label: p.Label, TestID: p.TestID, OnLayout: p.OnLayout, Tooltip: p.Tooltip, Class: parts.JoinClasses(p.Classes.Root, p.Class), Style: root, StyleFunc: p.StyleFunc,
		Children: []core.Node{core.View{
			// The frame is the padding box: the layers lie in it, and its
			// size is the viewport's.
			Style:    fill0(),
			OnLayout: func(b core.Bounds) { t.viewport.Set(b.Size) },
			Children: []core.Node{
				t.body(),
				t.hitLayer(),
				t.header(),
				t.strip(),
				t.overlays(),
				core.Show{When: t.focused.Get, Then: func(cx *core.Scope) core.Node {
					t.keys(cx)
					return core.Group{}
				}},
			},
		}},
	}
}

// fill0 is an absolute view over its parent.
func fill0() core.Style {
	z := core.Pt(0)
	return core.Style{Position: core.Absolute, Left: z, Top: z, Right: z, Bottom: z}
}

// px is a length of v points.
func px[T float32 | float64 | int](v T) core.Length { return core.Pt(core.Points(v)) }

// merge merges styles in order, later ones winning.
func merge(styles ...core.Style) core.Style {
	var out core.Style
	for _, s := range styles {
		out = out.Merge(s)
	}
	return out
}

// contentWidth is the hit layer's box width, whole columns included.
func (t *dataTable) contentWidth() float32 {
	g := t.sizes.Get()
	return g.lead + g.fixed + float32(g.x.extent()) + g.right
}

// below is an absolute view under the header.
func (t *dataTable) below() core.Style {
	z := core.Pt(0)
	return core.Style{Position: core.Absolute, Left: z, Right: z, Bottom: z, Top: px(t.sizes.Get().header)}
}

func (t *dataTable) hitLayer() core.Node {
	p := t.p
	behavior := core.ScrollBehavior{NoBounces: p.NoBounce}
	pressed := func(e core.PressEvent) { t.press(t.hitAt(e.Position), e.Clicks, e.Modifiers) }
	box := core.View{
		StyleFunc: func() core.Style {
			return core.Style{MinWidth: core.Pct(100), Width: px(t.contentWidth()), Height: px(t.sizes.Get().y.extent())}
		},
		OnPress: pressed,
		// Tab stops at the table here: its keys follow.
		OnFocus: func() { t.focused.Set(true) },
		OnBlur:  func() { t.focused.Set(false) },
		OnMouseMove: func(e core.MouseEvent) {
			if e.PressedButton == core.MouseButtonNone {
				t.hoverAt(e.Position)
			}
		},
		OnMouseLeave:  func(core.MouseEvent) { t.hover.Set(-1) },
		OnContextMenu: func(e core.ContextMenuEvent) { t.contextMenu(t.hitAt(e.Position), e.PagePosition) },
		TestID:        "data-table-hit",
	}
	return core.Scroll{
		Ref: t.vscroll, TestID: "data-table-scroll", Behavior: behavior, OffsetValues: core.AnimatedXY{Y: t.follow.offY},
		HideIndicator: p.NoScrollbar || p.NoVerticalScrollbar, Bar: p.Bar,
		StyleFunc: t.below,
		OnScroll:  func(e core.ScrollEvent) { t.scrollY.Set(e.Offset.Y) },
		Children: []core.Node{core.View{
			Style:     core.Style{Direction: core.Row, AlignItems: core.AlignStretch},
			StyleFunc: func() core.Style { return core.Style{Height: px(t.sizes.Get().y.extent())} },
			Children: []core.Node{core.Scroll{
				Horizontal: true, Ref: t.hscroll, HideIndicator: true, Behavior: behavior,
				OffsetValues: core.AnimatedXY{X: t.follow.offX},
				Style:        core.Style{FlexGrow: 1},
				OnScroll: func(e core.ScrollEvent) {
					t.scrollX.Set(e.Offset.X)
					// The desktops' bar follows, unless it leads.
					if t.hlead.take(false) {
						t.stripRef.ScrollTo(core.Point{X: e.Offset.X}, false)
					}
				},
				Children: []core.Node{box},
			}},
		}},
	}
}

// tableLead is which side of one axis moved last, so its follower's
// scroll events, which come after the moves that caused them, are echoes
// for a moment after each of the leader's moves and are not sent back.
type tableLead struct {
	side *bool
	at   time.Time
}

// take reports whether side leads: it moved last, or the other side has
// not moved for stripEcho.
func (l *tableLead) take(side bool) bool {
	now := time.Now()
	if l.side != nil && *l.side != side && now.Sub(l.at) < stripEcho {
		return false
	}
	l.side, l.at = &side, now
	return true
}

// strip is the desktops' horizontal scroll bar: a band over the body's
// bottom edge whose own scroll view the bar belongs to, moving the
// table's.
func (t *dataTable) strip() core.Node {
	p := t.p
	return core.Show{
		When: func() bool {
			g := t.sizes.Get()
			return t.cx.Env().Desktop() && !p.NoScrollbar && !p.NoHorizontalScrollbar && g.x.extent() > g.x.viewport
		},
		Then: func(cx *core.Scope) core.Node {
			z := core.Pt(0)
			return core.Scroll{
				Horizontal: true, Ref: t.stripRef, TestID: "data-table-scrollbar", Bar: p.Bar,
				Style: core.Style{Position: core.Absolute, Left: z, Right: z, Bottom: z, Height: px(stripHeight)},
				OnScroll: func(e core.ScrollEvent) {
					if t.hlead.take(true) && math.Abs(float64(e.Offset.X-t.scrollX.GetUntracked())) > 0.5 {
						t.hscroll.ScrollTo(core.Point{X: e.Offset.X}, false)
					}
				},
				Children: []core.Node{core.View{StyleFunc: func() core.Style {
					return core.Style{Height: px(1), Width: px(t.contentWidth())}
				}}},
			}
		},
	}
}

// tableSlots are views made once and kept while the table scrolls, each
// showing the item of the window whose index leaves its slot number when
// divided by their count, the most the window has held.
type tableSlots struct {
	count  *core.Memo[int]
	window *core.Memo[[2]int]
}

func newTableSlots(cx *core.Scope, window func() [2]int) tableSlots {
	w := core.NewMemo(cx, window)
	most := 0
	count := core.NewMemo(cx, func() int {
		r := w.Get()
		most = max(most, r[1]-r[0])
		return most
	})
	return tableSlots{count: count, window: w}
}

func (s tableSlots) list() []int {
	out := make([]int, s.count.Get())
	for i := range out {
		out[i] = i
	}
	return out
}

func (s tableSlots) item(cx *core.Scope, slot int) *core.Memo[int] {
	return core.NewMemo(cx, func() int {
		w := s.window.Get()
		return slotItem(slot, s.count.Get(), w[0], w[1])
	})
}

// tableWindows are what the body's and the header's regions share: the
// windows of rows and columns made and where they are placed from.
type tableWindows struct {
	rowSlots, colSlots tableSlots
	// scrollCols, fixedCols and rightCols are the defs of the scrolling
	// columns made and of those fixed at the left and the right.
	scrollCols, fixedCols, rightCols *core.Memo[[]int]
	// rowOrigin is the first row of the chunk rows are placed from;
	// colOrigin the display index of the first scrolling column of its
	// chunk, whence the scrolling columns are placed; span how far the made
	// scrolling columns reach from it.
	rowOrigin, colOrigin *core.Memo[int]
	span                 *core.Memo[float32]
	// shownCols are the scrolling columns' display indexes in view and one
	// each way, whose cells show their texts; the other cells made keep
	// what they showed, hidden.
	shownCols *core.Signal[[2]int]
}

func (t *dataTable) windows(cx *core.Scope) tableWindows {
	w := tableWindows{}
	// What the scroll offsets decide is set by one effect, as signals,
	// rather than read by memos: a signal that does not change reaches no
	// one, while a memo of the offsets has the reactive graph check every
	// cell under it at each scroll frame. Only the region boxes follow the
	// offsets themselves.
	rows := core.NewSignal(cx, [2]int{})
	cols := core.NewSignalFunc(cx, []int(nil), slices.Equal)
	rowOrigin, colOrigin := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
	w.shownCols = core.NewSignal(cx, [2]int{})
	cx.Effect(func() {
		vs, ve := t.visibleRows()
		// As many rows as the viewport can show at any offset, so the
		// window keeps its length as a scroll moves a row in and out at
		// each edge and no slot is hidden and shown again.
		if g := t.geo(); g.rowHeight > 0 {
			ve = max(ve, min(vs+int(math.Ceil(g.y.viewport/float64(g.rowHeight)))+1, t.rowCount()))
		}
		rows.Set(rowWindow(vs, ve, t.rowCount()))
		rowOrigin.Set(vs / rowChunk * rowChunk)
		cvs, cve := t.visibleCols()
		c := t.st.columns
		colOrigin.Set(max(cvs/columnChunk*columnChunk, c.fixed))
		w.shownCols.Set([2]int{max(cvs-1, c.fixed), min(cve+1, c.scrollEnd())})
		var defs []int
		if cvs < cve {
			cs, ce := t.colsWin.Span(cvs, cve, c.scrollEnd())
			for d := max(cs, c.fixed); d < min(ce, c.scrollEnd()); d++ {
				defs = append(defs, c.def(d))
			}
		}
		cols.Set(defs)
	})
	w.rowSlots = newTableSlots(cx, rows.Get)
	w.scrollCols = core.NewMemoFunc(cx, cols.Get, slices.Equal)
	w.colSlots = newTableSlots(cx, func() [2]int {
		defs := w.scrollCols.Get()
		if len(defs) == 0 {
			return [2]int{}
		}
		first := t.st.columns.displayOf(defs[0])
		return [2]int{first, first + len(defs)}
	})
	w.fixedCols = core.NewMemoFunc(cx, func() []int {
		t.layoutRev.Get()
		return slices.Clone(t.st.columns.order[:t.st.columns.fixed])
	}, slices.Equal)
	w.rightCols = core.NewMemoFunc(cx, func() []int {
		t.layoutRev.Get()
		return slices.Clone(t.st.columns.order[t.st.columns.scrollEnd():])
	}, slices.Equal)
	w.rowOrigin = core.NewMemo(cx, rowOrigin.Get)
	w.colOrigin = core.NewMemo(cx, colOrigin.Get)
	w.span = core.NewMemo(cx, func() float32 {
		t.layoutRev.Get()
		c := t.st.columns
		from := c.startOf(w.colOrigin.Get())
		end := 0.0
		for _, def := range w.scrollCols.Get() {
			d := c.displayOf(def)
			end = math.Max(end, c.start(d)+float64(c.displayWidth(d)))
		}
		return float32(math.Max(end-from, 0))
	})
	return w
}

// rowWindow is the rows made with rows vs to ve in view, of count: half a
// viewport each way, as many more on one side as the other end cuts, so
// the window keeps its length and moves with every row that scrolls.
//
// NSH's window reaches a viewport and a half ahead and moves only near its
// edges, so a scroll inside it commits nothing. The cells here move with
// the scroll views on the main thread (tableFollow), so a commit per row
// costs nothing to see, and rows made ahead were work thrown away: a fast
// scroll replaced the whole window every frame, its rows ahead gone
// before they showed, and a slow one sent the window's next viewport at
// once.
func rowWindow(vs, ve, count int) [2]int {
	if ve <= vs {
		return [2]int{vs, min(vs+16, count)}
	}
	reach := max((ve-vs)/2, 4)
	s, e := vs-reach, ve+reach
	if s < 0 {
		e -= s
		s = 0
	}
	if e > count {
		s = max(s-(e-count), 0)
		e = count
	}
	return [2]int{s, e}
}

// tableRegion is which of the body's regions a row or a title is in.
type tableRegion uint8

const (
	regionLead tableRegion = iota
	regionFixed
	regionScroll
	// regionRight holds the columns fixed at the right.
	regionRight
)

// regionShown reports, tracked, whether the region has columns, or the
// row numbers.
func (t *dataTable) regionShown(which tableRegion) bool {
	switch which {
	case regionLead:
		return t.rowHeaderShown()
	case regionFixed:
		return t.layoutRev.Get() > 0 && t.st.columns.fixed > 0
	case regionRight:
		return t.layoutRev.Get() > 0 && t.st.columns.right > 0
	}
	return true
}

// regionOrigin is where the region's columns are placed from, in the
// unscrolled table.
func (t *dataTable) regionOrigin(which tableRegion, w tableWindows) float64 {
	c := t.st.columns
	switch which {
	case regionScroll:
		return c.startOf(w.colOrigin.Get())
	case regionRight:
		return c.startOf(c.scrollEnd())
	}
	return 0
}

// regionCols is the defs of the columns the region shows.
func (w tableWindows) regionCols(which tableRegion) *core.Memo[[]int] {
	switch which {
	case regionFixed:
		return w.fixedCols
	case regionRight:
		return w.rightCols
	}
	return w.scrollCols
}

func (t *dataTable) body() core.Node {
	return core.Func(func(cx *core.Scope) core.Node {
		w := t.windows(cx)
		t.follow.track(cx, t, w)
		return core.View{
			StyleFunc:     func() core.Style { return t.below().Merge(core.Style{Overflow: core.Hidden}) },
			PointerEvents: core.PointerEventsNone,
			Children: []core.Node{
				t.region(regionScroll, w),
				t.edge(false),
				t.rightEdge(false),
				t.region(regionFixed, w),
				t.region(regionRight, w),
				t.region(regionLead, w),
			},
		}
	})
}

// edge is the line at the fixed columns' trailing edge.
func (t *dataTable) edge(inRow bool) core.Node {
	return core.Show{
		When: func() bool { return t.layoutRev.Get() > 0 && t.st.columns.fixed > 0 },
		Then: func(cx *core.Scope) core.Node {
			z := core.Pt(0)
			return core.View{Class: t.p.Classes.FixedEdge, StyleFunc: func() core.Style {
				g := t.sizes.Get()
				x := g.lead + g.fixed - 1
				if inRow {
					x = g.fixed - 1
				}
				return core.Style{Position: core.Absolute, Top: z, Bottom: z, Width: px(1), Left: px(x)}.Merge(t.styles().FixedEdge)
			}}
		},
	}
}

// rightEdge is the line at the leading edge of the columns fixed at the
// right.
func (t *dataTable) rightEdge(inRow bool) core.Node {
	return core.Show{
		When: func() bool { return t.layoutRev.Get() > 0 && t.st.columns.right > 0 },
		Then: func(cx *core.Scope) core.Node {
			z := core.Pt(0)
			return core.View{Class: t.p.Classes.FixedEdge, StyleFunc: func() core.Style {
				g := t.sizes.Get()
				x := g.lead + g.fixed + float32(g.x.viewport)
				if inRow {
					x = 0
				}
				return core.Style{Position: core.Absolute, Top: z, Bottom: z, Width: px(1), Left: px(x)}.Merge(t.styles().FixedEdge)
			}}
		},
	}
}

// regionBox is a region's clipped box, from its left edge.
// Its size is in points, so the rows moving inside it lay out it alone
// (layout.Boundary).
func (t *dataTable) regionBox(which tableRegion) core.Style {
	g := t.sizes.Get()
	z := core.Pt(0)
	s := core.Style{Position: core.Absolute, Top: z, Height: px(g.y.viewport), Overflow: core.Hidden}
	switch which {
	case regionLead:
		s.Left, s.Width = z, px(g.lead)
	case regionFixed:
		s.Left, s.Width = px(g.lead), px(g.fixed)
	case regionRight:
		s.Left, s.Width = px(g.lead+g.fixed+float32(g.x.viewport)), px(g.right)
	default:
		s.Left, s.Width = px(g.lead+g.fixed), px(g.x.viewport)
	}
	return s
}

// scrollOrigin is where the scrolling region's columns are placed from,
// scrolled, on whole device pixels as layout rounds frames: off them the
// cells' texts would round to another pixel as it changes, and every
// cell's frame would be sent again at each scroll frame.
func (t *dataTable) scrollOrigin(w tableWindows) float32 {
	g := t.geo()
	return float32(t.onPixel(t.scrollStart(w.colOrigin.Get()) - g.cx))
}

// rowsTop is where the rows' chunk is placed from in the body, scrolled,
// on whole device pixels, as scrollOrigin is for the columns.
func (t *dataTable) rowsTop(w tableWindows) float64 {
	g := t.geo()
	return t.onPixel(float64(w.rowOrigin.Get())*float64(g.rowHeight) - g.cy)
}

// onPixel rounds points to the surface's device pixels.
func (t *dataTable) onPixel(points float64) float64 {
	ratio := float64(max(t.cx.Env().PixelRatio(), 1e-6))
	return math.Round(points*ratio) / ratio
}

// tableFollow moves the body with the scroll views in the frame they
// move, on the platform's main thread: a commit places the rows and
// columns where the table had scrolled to then, and each region's
// translation is how far the scroll views went since (shiftX, shiftY), so
// the cells move in step with the content whatever a commit costs and
// however late it lands. A commit sets the bases (baseX, baseY) with the
// places it lays out, so the two never show apart, and at rest the
// translation is what rounding the places to whole points left. The
// translation stops where the rows made and the columns shown end (madeX, madeY):
// a jump past them, a scroll bar dragged across a million rows, waits
// for the commit that brings the cells there rather than showing none.
type tableFollow struct {
	// offX and offY follow the scroll views' offsets.
	offX, offY *core.AnimatedValue
	// baseX and baseY are how far the scroll views were in the content
	// at the last commit, as its places have it.
	baseX, baseY *core.AnimatedValue
	// madeX and madeY are the content offsets the cells made cover, from
	// the first to the last at which they fill the view.
	madeX, madeY *core.Signal[[2]float64]
	// shiftX and shiftY are the regions' translations: the base less the
	// content offset the scroll views are at now.
	shiftX, shiftY *core.Memo[core.AnimatedNode]
}

func newTableFollow(cx *core.Scope, t *dataTable) tableFollow {
	f := tableFollow{offX: core.NewAnimatedValue(cx, 0), offY: core.NewAnimatedValue(cx, 0),
		baseX: core.NewAnimatedValue(cx, 0), baseY: core.NewAnimatedValue(cx, 0),
		madeX: core.NewSignal(cx, [2]float64{}), madeY: core.NewSignal(cx, [2]float64{})}
	clamp := func(in core.AnimatedNode, lo, hi float64) core.AnimatedNode {
		if hi <= lo {
			return core.Interpolate(in, core.Interpolation{Input: []float64{lo, lo + 1}, Output: []float64{lo, lo},
				Extrapolate: core.ExtrapolateClamp})
		}
		return core.Interpolate(in, core.Interpolation{Input: []float64{lo, hi}, Output: []float64{lo, hi},
			Extrapolate: core.ExtrapolateClamp})
	}
	// The content offset of a scroll offset, as tableAxis.contentOffset
	// has it: scaled past maxExtent, within the content, then within the
	// cells made.
	content := func(off *core.AnimatedValue, a tableAxis, made [2]float64) core.AnimatedNode {
		scroll, max := a.extent()-a.viewport, a.total-a.viewport
		var at core.AnimatedNode
		if scroll <= 0 || max <= 0 {
			at = clamp(off, 0, 0)
		} else {
			at = core.Interpolate(off, core.Interpolation{Input: []float64{0, scroll}, Output: []float64{0, max},
				Extrapolate: core.ExtrapolateClamp})
		}
		return clamp(at, made[0], made[1])
	}
	f.shiftY = core.NewMemoFunc(cx, func() core.AnimatedNode {
		return core.Sub(f.baseY, content(f.offY, t.sizes.Get().y, f.madeY.Get()))
	}, nil)
	f.shiftX = core.NewMemoFunc(cx, func() core.AnimatedNode {
		return core.Sub(f.baseX, content(f.offX, t.sizes.Get().x, f.madeX.Get()))
	}, nil)
	return f
}

// track sets the bases as the commit places the regions.
func (f tableFollow) track(cx *core.Scope, t *dataTable, w tableWindows) {
	cx.Effect(func() {
		f.baseY.Set(float64(w.rowOrigin.Get())*float64(t.geo().rowHeight) - t.rowsTop(w))
		f.baseX.Set(t.scrollStart(w.colOrigin.Get()) - float64(t.scrollOrigin(w)))
	})
	// The cells that show their texts are those it may reach.
	cx.Effect(func() {
		g := t.sizes.Get()
		rh := float64(g.rowHeight)
		rows := w.rowSlots.window.Get()
		f.madeY.Set([2]float64{float64(rows[0]) * rh, float64(rows[1])*rh - g.y.viewport})
		t.layoutRev.Get()
		cols := w.shownCols.Get()
		f.madeX.Set([2]float64{t.scrollStart(cols[0]), t.scrollStart(cols[1]) - g.x.viewport})
	})
}

func (t *dataTable) region(which tableRegion, w tableWindows) core.Node {
	return core.Show{When: func() bool { return t.regionShown(which) }, Then: func(cx *core.Scope) core.Node {
		return core.View{
			StyleFunc: func() core.Style { return t.regionBox(which) },
			Children: []core.Node{core.View{
				StyleFunc: func() core.Style {
					s := core.Style{Position: core.Absolute, Top: px(t.rowsTop(w)), Left: px(0),
						Animated: &core.AnimatedStyle{TranslateY: t.follow.shiftY.Get()}}
					if which == regionScroll {
						s.Left = px(t.scrollOrigin(w))
						s.Animated.TranslateX = t.follow.shiftX.Get()
					}
					return s
				},
				Children: []core.Node{core.Each(w.rowSlots.list, func(slot int) int { return slot },
					func(cx *core.Scope, slot func() int, _ func() int) core.Node {
						return t.bodyRow(cx, w.rowSlots.item(cx, core.Untracked(cx, slot)), which, w)
					})},
			}},
		}
	}}
}

// rowParts is a row's style and classes from its parts and its own
// RowStyle and RowClass.
func (t *dataTable) rowParts(cx *core.Scope, row *core.Memo[int]) (func() core.Style, func() string) {
	hovered := core.NewMemo(cx, func() bool { r := row.Get(); return r >= 0 && t.hover.Get() == r })
	selected := core.NewMemo(cx, func() bool { return t.selection() == tableSel{kind: selRow, row: row.Get()} })
	active := core.NewMemo(cx, func() bool {
		t.dataRev.Get()
		r := row.Get()
		return r >= 0 && t.st.active && t.st.activeRow == r && t.st.activeCol < 0
	})
	odd := func(r int) bool { return t.p.Stripe != nil && t.p.Stripe() && r%2 == 1 }
	style := func() core.Style {
		r := row.Get()
		ts := t.styles()
		out := ts.Row
		if odd(r) {
			out = merge(out, ts.RowOdd)
		}
		if hovered.Get() {
			out = merge(out, ts.RowHover)
		}
		if selected.Get() {
			out = merge(out, ts.RowSelected)
		}
		if active.Get() {
			out = merge(out, ts.RowActive)
		}
		if r >= 0 && t.p.RowStyle != nil {
			out = merge(out, t.p.RowStyle(r))
		}
		return out
	}
	class := func() string {
		r := row.Get()
		tc := &t.p.Classes
		out := tc.Row
		if odd(r) {
			out = parts.JoinClasses(out, tc.RowOdd)
		}
		if hovered.Get() {
			out = parts.JoinClasses(out, tc.RowHover)
		}
		if selected.Get() {
			out = parts.JoinClasses(out, tc.RowSelected)
		}
		if active.Get() {
			out = parts.JoinClasses(out, tc.RowActive)
		}
		if r >= 0 && t.p.RowClass != nil {
			out = parts.JoinClasses(out, t.p.RowClass(r))
		}
		return out
	}
	return style, class
}

// pressRegion is the hit a press at x in a row of which region names.
func (t *dataTable) rowHit(row int, which tableRegion, x core.Points, w tableWindows) tableHit {
	h := tableHit{row: row, col: -1, lead: which == regionLead}
	c := t.st.columns
	d := -1
	switch which {
	case regionFixed:
		d = c.displayAtTable(float64(x))
	case regionScroll:
		d = c.displayAtTable(c.startOf(w.colOrigin.GetUntracked()) + float64(x))
	case regionRight:
		d = c.displayAtTable(c.startOf(c.scrollEnd()) + float64(x))
	}
	if d >= 0 {
		h.col = c.def(d)
	}
	return h
}

func (t *dataTable) bodyRow(cx *core.Scope, row *core.Memo[int], which tableRegion, w tableWindows) core.Node {
	parts, classes := t.rowParts(cx, row)
	selected := core.NewMemo(cx, func() bool { return t.selection() == tableSel{kind: selRow, row: row.Get()} })
	var children []core.Node
	switch which {
	case regionLead:
		children = []core.Node{t.rowNumber(row)}
	case regionFixed:
		// Above the row's background, below its cells: a selected cell's
		// trailing border covers the fixed columns' edge.
		children = []core.Node{t.edge(true), core.Each(w.fixedCols.Get, func(def int) int { return def },
			func(cx *core.Scope, def func() int, _ func() int) core.Node {
				d := core.Untracked(cx, def)
				return t.cell(cx, row, core.NewMemo(cx, func() int { return d }), which, w)
			})}
	case regionRight:
		children = []core.Node{t.rightEdge(true), core.Each(w.rightCols.Get, func(def int) int { return def },
			func(cx *core.Scope, def func() int, _ func() int) core.Node {
				d := core.Untracked(cx, def)
				return t.cell(cx, row, core.NewMemo(cx, func() int { return d }), which, w)
			})}
	default:
		children = []core.Node{core.Each(w.colSlots.list, func(slot int) int { return slot },
			func(cx *core.Scope, slot func() int, _ func() int) core.Node {
				display := w.colSlots.item(cx, core.Untracked(cx, slot))
				def := core.NewMemo(cx, func() int {
					d := display.Get()
					t.layoutRev.Get()
					if d < 0 || d >= t.st.columns.len() {
						return -1
					}
					return t.st.columns.def(d)
				})
				return t.cell(cx, row, def, which, w)
			})}
	}
	return core.View{
		ClassFunc: classes,
		StyleFunc: func() core.Style {
			r := row.Get()
			s := core.Style{Position: core.Absolute, Left: px(0), Height: px(t.rowHeight()),
				Top: px(float32(r-w.rowOrigin.Get()) * t.rowHeight())}
			if r < 0 {
				s.Display = core.DisplayNone
			}
			switch which {
			case regionLead:
				s.Width = px(t.sizes.Get().lead)
			case regionFixed:
				s.Width = px(t.sizes.Get().fixed)
			case regionRight:
				s.Width = px(t.sizes.Get().right)
			default:
				s.Width = px(w.span.Get())
			}
			return s.Merge(parts())
		},
		Role:     core.RoleRow,
		Selected: selected.Get,
		// The table is one stop of Tab, its hit layer.
		NoFocus: true,
		// The views of the cells take presses from assistive technology
		// and tests; a pointer reaches the hit layer.
		OnPress: func(e core.PressEvent) {
			if r := row.GetUntracked(); r >= 0 {
				t.press(t.rowHit(r, which, e.Position.X, w), max(e.Clicks, 1), e.Modifiers)
			}
		},
		// A row's cells ask for their own menus.
		OnContextMenu: func(e core.ContextMenuEvent) {
			if r := row.GetUntracked(); r >= 0 {
				t.contextMenu(tableHit{row: r, col: -1, lead: which == regionLead}, e.PagePosition)
			}
		},
		Children: children,
	}
}

// rowNumber is a row's number, in the row numbers' column.
func (t *dataTable) rowNumber(row *core.Memo[int]) core.Node {
	return core.View{
		Class:     t.p.Classes.RowHeader,
		Style:     fill0().Merge(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}),
		StyleFunc: func() core.Style { return t.styles().RowHeader },
		Children: []core.Node{core.Text{MaxLines: 1, ContentFunc: func() string {
			if r := row.Get(); r >= 0 {
				return fmt.Sprint(r + 1)
			}
			return ""
		}}},
	}
}

// column is def's definition, the zero column for none.
func (t *dataTable) column(def int) Column {
	if def < 0 || def >= len(t.st.defs) {
		return Column{}
	}
	return t.st.defs[def]
}

func justify(a Align) core.Justify {
	switch a {
	case AlignCenterColumn:
		return core.JustifyCenter
	case AlignEndColumn:
		return core.JustifyEnd
	}
	return core.JustifyStart
}

// cell is a cell of row and column def. It reads its text while its
// column is among those shown, and keeps the last one, hidden, outside
// them: a horizontal window shift gives texts to the columns near the
// view.
func (t *dataTable) cell(cx *core.Scope, row, def *core.Memo[int], which tableRegion, w tableWindows) core.Node {
	colShown := core.NewMemo(cx, func() bool {
		if which != regionScroll {
			return true
		}
		d := def.Get()
		t.layoutRev.Get()
		c, b := t.st.columns, w.shownCols.Get()
		return c.hasDef(d) && c.displayOf(d) >= b[0] && c.displayOf(d) < b[1]
	})
	type flags struct{ selected, columnSelected, active bool }
	marks := core.NewMemo(cx, func() flags {
		t.dataRev.Get()
		r, d := row.Get(), def.Get()
		if r < 0 || d < 0 {
			return flags{}
		}
		sel := t.st.sel
		return flags{selected: sel == tableSel{kind: selCell, row: r, col: d},
			columnSelected: sel == tableSel{kind: selColumn, col: d},
			active:         t.st.active && t.st.activeRow == r && t.st.activeCol == d}
	})
	// The text and its sign are memos: a tick that changes some texts
	// restyles only the cells whose sign changed.
	var last string
	text := core.NewMemo(cx, func() string {
		r, d := row.Get(), def.Get()
		if r >= 0 && d >= 0 && colShown.Get() {
			last = t.cellText(r, d)
		}
		return last
	}).Get
	sign := core.NewMemo(cx, func() int {
		t.layoutRev.Get()
		if d := def.Get(); d < 0 || d >= len(t.st.toned) || !t.st.toned[d] {
			return 0
		}
		return cellSign(text())
	})
	// The text fills the cell and aligns in it, so its box stays as a
	// new text takes it: a window shift sends the cells' texts, not their
	// frames too, as when the text's own width placed it.
	var content core.Node = core.Text{ContentFunc: text, MaxLines: 1, StyleFunc: func() core.Style {
		t.layoutRev.Get()
		align := core.TextAlignStart
		switch t.column(def.Get()).Align {
		case AlignCenterColumn:
			align = core.TextAlignCenter
		case AlignEndColumn:
			align = core.TextAlignEnd
		}
		return core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: px(0), MinWidth: px(0), TextAlign: align}
	}}
	if t.p.RenderCell != nil {
		content = core.View{Style: core.Style{FlexGrow: 1, MinWidth: px(0)}, Children: []core.Node{t.p.RenderCell(cx, func() Cell {
			r, d := row.Get(), def.Get()
			t.dataRev.Get()
			sel := t.st.sel
			return Cell{Row: r, Column: d, Key: t.column(d).Key, Text: text(),
				Selected: sel == tableSel{kind: selRow, row: r} || sel == tableSel{kind: selCell, row: r, col: d}}
		})}}
	}
	return core.View{
		OnContextMenu: func(e core.ContextMenuEvent) {
			if r, d := row.GetUntracked(), def.GetUntracked(); r >= 0 && d >= 0 {
				t.contextMenu(tableHit{row: r, col: d}, e.PagePosition)
			}
		},
		Role:     core.RoleCell,
		Selected: func() bool { f := marks.Get(); return f.selected || f.columnSelected },
		ClassFunc: func() string {
			t.layoutRev.Get()
			col := t.column(def.Get())
			f := marks.Get()
			tc := &t.p.Classes
			out := tc.Cell
			if f.columnSelected {
				out = parts.JoinClasses(out, tc.CellColumnSelected)
			}
			if f.selected {
				out = parts.JoinClasses(out, tc.CellSelected)
			}
			if f.active {
				out = parts.JoinClasses(out, tc.CellActive)
			}
			out = parts.JoinClasses(out, col.CellClass)
			switch sign.Get() {
			case 1:
				out = parts.JoinClasses(out, col.PositiveClass)
			case -1:
				out = parts.JoinClasses(out, col.NegativeClass)
			}
			return out
		},
		StyleFunc: func() core.Style {
			t.layoutRev.Get()
			d := def.Get()
			c := t.st.columns
			z := core.Pt(0)
			st := core.Style{Position: core.Absolute, Top: z, Bottom: z, Direction: core.Row, AlignItems: core.AlignCenter,
				Overflow: core.Hidden}
			if d < 0 || !c.hasDef(d) {
				st.Display = core.DisplayNone
				return st
			}
			if !colShown.Get() {
				st.Visibility = core.VisibilityHidden
			}
			origin := t.regionOrigin(which, w)
			col := t.column(d)
			st.Left, st.Width = px(c.start(c.displayOf(d))-origin), px(c.width(d))
			st.JustifyContent = justify(col.Align)
			f := marks.Get()
			ts := t.styles()
			st = merge(st, ts.Cell)
			if f.columnSelected {
				st = merge(st, ts.CellColumnSelected)
			}
			if f.selected {
				st = merge(st, ts.CellSelected)
			}
			if f.active {
				st = merge(st, ts.CellActive)
			}
			st = merge(st, col.CellStyle)
			switch sign.Get() {
			case 1:
				st = merge(st, col.PositiveStyle)
			case -1:
				st = merge(st, col.NegativeStyle)
			}
			return st
		},
		Children: []core.Node{content},
	}
}

// groupCell is a label over the columns from to to, placed by display
// index.
type groupCell struct {
	level    int
	label    string
	from, to int
}

// groupCells are the groups of every level the region shows: in the fixed
// region those starting among the fixed columns, in the scrolling one
// those overlapping the made columns.
func (t *dataTable) groupCells(scroll bool, window []int, origin int) []groupCell {
	if t.p.Groups == nil {
		return nil
	}
	c := t.st.columns
	fixed, n := c.fixed, c.len()
	first, last := -1, -1
	for _, def := range window {
		d := c.displayOf(def)
		if first < 0 {
			first = d
		}
		last = max(last, d)
	}
	var out []groupCell
	for level, groups := range t.p.Groups() {
		at := 0
		for _, g := range groups {
			from, to := at, min(at+max(g.Span, 1), n)
			at = to
			if from >= n {
				break
			}
			keep := from < fixed
			if scroll {
				low := origin
				if first >= 0 {
					low = min(first, origin)
				}
				keep = to > max(fixed, low) && last >= 0 && from <= last
			}
			if keep {
				out = append(out, groupCell{level, g.Label, from, to})
			}
		}
	}
	return out
}

func (t *dataTable) header() core.Node {
	return core.Func(func(cx *core.Scope) core.Node {
		w := t.windows(cx)
		z := core.Pt(0)
		return core.View{
			Class: t.p.Classes.Header,
			StyleFunc: func() core.Style {
				return merge(core.Style{Position: core.Absolute, Top: z, Left: z, Right: z, Height: px(t.sizes.Get().header),
					Overflow: core.Hidden}, t.styles().Header)
			},
			Children: []core.Node{
				t.headerRegion(regionScroll, w),
				t.headerRegion(regionFixed, w),
				t.headerRegion(regionRight, w),
				core.Show{When: t.rowHeaderShown, Then: func(cx *core.Scope) core.Node {
					return core.View{Class: t.p.Classes.RowHeaderCorner, StyleFunc: func() core.Style {
						return merge(core.Style{Position: core.Absolute, Top: z, Bottom: z, Left: z, Width: px(t.sizes.Get().lead)},
							t.styles().RowHeaderCorner)
					}}
				}},
				t.moveLine(),
			},
		}
	})
}

func (t *dataTable) headerRowHeightNow() float32 {
	return t.sizes.Get().header / float32(t.headerLevels())
}

func (t *dataTable) headerRegion(which tableRegion, w tableWindows) core.Node {
	return core.Show{When: func() bool { return t.regionShown(which) }, Then: func(cx *core.Scope) core.Node {
		groups := core.NewMemoFunc(cx, func() []groupCell {
			t.layoutRev.Get()
			if which == regionRight {
				return nil
			}
			return t.groupCells(which == regionScroll, w.scrollCols.Get(), w.colOrigin.Get())
		}, slices.Equal)
		list := w.regionCols(which)
		z := core.Pt(0)
		return core.View{
			StyleFunc: func() core.Style {
				g := t.sizes.Get()
				s := core.Style{Position: core.Absolute, Top: z, Bottom: z, Overflow: core.Hidden, Left: px(g.lead)}
				switch which {
				case regionScroll:
					s.Left, s.Width = px(g.lead+g.fixed), px(g.x.viewport)
				case regionRight:
					s.Left, s.Width = px(g.lead+g.fixed+float32(g.x.viewport)), px(g.right)
				default:
					s.Width = px(g.fixed)
				}
				return s
			},
			Children: []core.Node{core.View{
				StyleFunc: func() core.Style {
					s := core.Style{Position: core.Absolute, Top: z, Bottom: z, Left: z}
					if which == regionScroll {
						s.Left = px(t.scrollOrigin(w))
						s.Animated = &core.AnimatedStyle{TranslateX: t.follow.shiftX.Get()}
					}
					return s
				},
				Children: []core.Node{
					core.Each(groups.Get, func(g groupCell) [2]int { return [2]int{g.level, g.from} },
						func(cx *core.Scope, g func() groupCell, _ func() int) core.Node {
							return t.groupCell(core.Untracked(cx, g), which, w)
						}),
					core.Each(list.Get, func(def int) int { return def },
						func(cx *core.Scope, def func() int, _ func() int) core.Node {
							return t.titleCell(cx, core.Untracked(cx, def), which, w)
						}),
				},
			}},
		}
	}}
}

func (t *dataTable) groupCell(g groupCell, which tableRegion, w tableWindows) core.Node {
	return core.View{
		Class: t.p.Classes.GroupCell,
		StyleFunc: func() core.Style {
			t.layoutRev.Get()
			levels := t.headerLevels()
			h := t.headerRowHeightNow()
			c := t.st.columns
			origin := 0.0
			if which == regionScroll {
				origin = c.startOf(w.colOrigin.Get())
			}
			// The lowest level first: it is the one above the titles.
			top := h * float32(levels-2-min(g.level, levels-2))
			return merge(core.Style{Position: core.Absolute, Top: px(top), Height: px(h),
				Left: px(c.startOf(g.from) - origin), Width: px(c.startOf(g.to) - c.startOf(g.from)),
				Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				Overflow: core.Hidden}, t.styles().GroupCell)
		},
		Children: []core.Node{core.Text{Content: g.label, MaxLines: 1}},
	}
}

func (t *dataTable) titleCell(cx *core.Scope, def int, which tableRegion, w tableWindows) core.Node {
	col := t.column(def)
	sorted := core.NewMemo(cx, func() Sort {
		if t.p.Sort == nil {
			return Sort{}
		}
		if sort := t.p.Sort(); sort.Key == col.Key && col.Key != "" {
			return sort
		}
		return Sort{}
	})
	selected := core.NewMemo(cx, func() bool { return t.selection() == tableSel{kind: selColumn, col: def} })
	dragging := core.NewMemo(cx, func() bool {
		d := t.drag.Get()
		return d.moving && t.st.columns.hasDef(def) && t.st.columns.displayOf(def) == d.from
	})
	moved := false
	responder := &core.Responder{
		Keep: true,
		Claims: []core.TouchClaim{{OnMove: true, Ask: func(e core.ResponderEvent) bool {
			dx, dy := math.Abs(float64(e.Gesture.Delta.X)), math.Abs(float64(e.Gesture.Delta.Y))
			// A drag from the resize handle's band resizes.
			start := float32(e.Position.X - e.Gesture.Delta.X)
			if t.resizing == def || t.resizable(def) && start > t.st.columns.width(def)-float32(parts.HandleBand) {
				return false
			}
			return dx > moveSlop && dx > dy && t.movable(def)
		}}},
		OnGrant: func(core.ResponderEvent) {
			moved = true
			if t.st.columns.hasDef(def) {
				t.moveStart(t.st.columns.displayOf(def))
			}
		},
		OnMove: func(e core.ResponderEvent) {
			g := t.untrackedGeo()
			at := t.viewX(t.st.columns.displayOf(def), g)
			t.moveTo(at+float32(e.Position.X), float32(e.Gesture.Delta.X))
		},
		OnRelease:   func(core.ResponderEvent) { t.moveEnd(true) },
		OnTerminate: func(core.ResponderEvent) { t.moveEnd(false) },
	}
	var label core.Node
	if t.p.RenderHeaderCell != nil {
		label = core.View{Style: core.Style{FlexGrow: 1, MinWidth: px(0)}, Children: []core.Node{t.p.RenderHeaderCell(cx, func() HeaderCell {
			sort := sorted.Get()
			return HeaderCell{Column: def, Key: col.Key, Title: t.title(def), Sorted: sort.Key != "",
				Descending: sort.Descending, Selected: selected.Get()}
		})}}
	} else {
		align := core.TextAlignStart
		switch col.Align {
		case AlignCenterColumn:
			align = core.TextAlignCenter
		case AlignEndColumn:
			align = core.TextAlignEnd
		}
		// The title takes the room the sort button leaves, aligned as the
		// column's cells.
		label = core.Text{Content: col.title(), MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: px(0), TextAlign: align}}
	}
	content := []core.Node{label}
	if col.Sortable {
		content = append(content, t.sortButton(def, sorted))
	}
	children := []core.Node{core.View{
		Class:     t.p.Classes.HeaderContent,
		Style:     core.Style{FlexGrow: 1, Direction: core.Row, AlignItems: core.AlignCenter, MinWidth: px(0)},
		StyleFunc: func() core.Style { return t.styles().HeaderContent },
		Children:  content,
	}}
	if t.resizable(def) {
		children = append(children, base.ResizeHandle{
			Axis: base.Horizontal, Edge: base.HandleTrailing, TestID: "resize:" + col.Key,
			OnDragStart: func() { t.resizeStart(def) },
			OnDragMove:  t.resizeMove,
			OnDragEnd:   t.resizeEnd,
			Class:       t.p.Classes.ResizeHandle,
			Style:       core.Style{Width: px(1)},
			StyleFunc:   func() core.Style { return t.styles().ResizeHandle },
		})
	}
	return core.View{
		Role:     core.RoleColumnHeader,
		Selected: selected.Get,
		NoFocus:  true,
		ClassFunc: func() string {
			tc := &t.p.Classes
			out := tc.HeaderCell
			if sorted.Get().Key != "" {
				out = parts.JoinClasses(out, tc.HeaderCellSorted)
			}
			if selected.Get() {
				out = parts.JoinClasses(out, tc.HeaderCellSelected)
			}
			if dragging.Get() {
				out = parts.JoinClasses(out, tc.HeaderCellDragging)
			}
			return parts.JoinClasses(out, col.HeaderClass)
		},
		StyleFunc: func() core.Style {
			t.layoutRev.Get()
			c := t.st.columns
			if !c.hasDef(def) {
				return core.Style{Display: core.DisplayNone}
			}
			levels := t.headerLevels()
			h := t.headerRowHeightNow()
			origin := 0.0
			if which == regionScroll {
				origin = c.startOf(w.colOrigin.Get())
			}
			st := core.Style{Position: core.Absolute, Top: px(h * float32(levels-1)), Height: px(h),
				Left: px(c.start(c.displayOf(def)) - origin), Width: px(c.width(def)), Direction: core.Row,
				AlignItems: core.AlignCenter, Overflow: core.Hidden, JustifyContent: justify(col.Align)}
			ts := t.styles()
			st = merge(st, ts.HeaderCell)
			if sorted.Get().Key != "" {
				st = merge(st, ts.HeaderCellSorted)
			}
			if selected.Get() {
				st = merge(st, ts.HeaderCellSelected)
			}
			if dragging.Get() {
				st = merge(st, ts.HeaderCellDragging)
			}
			return merge(st, col.HeaderStyle)
		},
		OnPress: func(e core.PressEvent) {
			// A drag that moved the column is not a press.
			if moved {
				moved = false
				return
			}
			t.headerPress(def, max(e.Clicks, 1))
		},
		OnContextMenu: func(e core.ContextMenuEvent) { t.contextMenu(tableHit{row: -1, col: def}, e.PagePosition) },
		Responder:     responder,
		Children:      children,
	}
}

// sortButton is the button at the end of a sortable title, which asks for
// the next sort; the app styles it and its two chevrons.
func (t *dataTable) sortButton(def int, sorted *core.Memo[Sort]) core.Node {
	const k = float32(12.0 / 24.0)
	chevron := func(pts [6]float32, descending bool, class string, style func() core.Style) core.Node {
		return core.View{
			Class: class, Style: fill0(), StyleFunc: style,
			Selected: func() bool {
				sort := sorted.Get()
				return sort.Key != "" && sort.Descending == descending
			},
			PointerEvents:             core.PointerEventsNone,
			ImportantForAccessibility: core.ImportantNoHideDescendants,
			Children: []core.Node{core.Canvas{Style: fill0(), Draw: func(d *core.Drawing, _ core.Size) {
				d.StrokeFromView(core.PaintCurrentColor).SetLineWidth(2*k).SetLineCap(core.CapRound).SetLineJoin(core.JoinRound).
					BeginPath().MoveTo(pts[0]*k, pts[1]*k).LineTo(pts[2]*k, pts[3]*k).LineTo(pts[4]*k, pts[5]*k).Stroke()
			}}},
		}
	}
	return core.View{
		Role: core.RoleButton, LabelFunc: func() string { return locales.SortBy(t.cx, t.title(def)) }, TestID: "sort:" + t.column(def).Key, NoFocus: true,
		Style: core.Style{FlexShrink: 0, Width: px(sortWidth), Height: px(sortWidth), AlignItems: core.AlignCenter,
			JustifyContent: core.JustifyCenter},
		Class:     t.p.Classes.SortIndicator,
		StyleFunc: func() core.Style { return t.styles().SortIndicator },
		OnPress:   func(core.PressEvent) { t.sortNext(def) },
		Children: []core.Node{core.View{Style: core.Style{Width: px(12), Height: px(12)}, Children: []core.Node{
			chevron([6]float32{7, 9, 12, 4, 17, 9}, false, t.p.Classes.SortAscending,
				func() core.Style { return t.styles().SortAscending }),
			chevron([6]float32{7, 15, 12, 20, 17, 15}, true, t.p.Classes.SortDescending,
				func() core.Style { return t.styles().SortDescending }),
		}}},
	}
}

// moveLine is the line where a dragged column will drop.
func (t *dataTable) moveLine() core.Node {
	return core.Show{When: func() bool { return t.drag.Get().moving }, Then: func(cx *core.Scope) core.Node {
		z := core.Pt(0)
		return core.View{Class: t.p.Classes.MoveIndicator, StyleFunc: func() core.Style {
			d := t.drag.Get()
			t.layoutRev.Get()
			g := t.geo()
			c := t.st.columns
			// At the target's trailing edge when moving right.
			display := d.to
			if d.to > d.from {
				display++
			}
			x := t.viewX(display, g)
			if display > c.fixed && display <= c.scrollEnd() {
				x = max(x, g.lead+g.fixed)
			}
			return merge(core.Style{Position: core.Absolute, Top: z, Bottom: z, Width: px(dropLine), Left: px(x - dropLine/2)},
				t.styles().MoveIndicator)
		}}
	}}
}

// overlays are Empty while there are no rows and LoadingView while
// loading, over the rows.
func (t *dataTable) overlays() core.Node {
	loading := func() bool { return t.p.Loading != nil && t.p.Loading() }
	var children []core.Node
	if t.p.Empty != nil {
		children = append(children, core.Show{When: func() bool { return t.rowCount() == 0 && !loading() },
			Then: func(cx *core.Scope) core.Node {
				return core.View{Class: t.p.Classes.Empty,
					Style:     merge(fill0(), core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}),
					StyleFunc: func() core.Style { return t.styles().Empty }, Children: []core.Node{t.p.Empty}}
			}})
	}
	if t.p.LoadingView != nil {
		children = append(children, core.Show{When: loading, Then: func(cx *core.Scope) core.Node {
			return core.View{Style: fill0(), Children: []core.Node{t.p.LoadingView}}
		}})
	}
	return core.View{StyleFunc: t.below, PointerEvents: core.PointerEventsBoxNone, Children: children}
}
