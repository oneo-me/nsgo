package richeditor

import (
	"fmt"
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/core"
)

const (
	bandWidth   = 6
	rowMinimum  = 16
	gripLength  = 18
	gripBreadth = 6
)

// tableDragState is a drag of a table's handle: the size a column or row is
// resized to, or where a moved one lands.
type tableDragState struct {
	table  *block
	col    bool
	index  int
	size   float64
	moving bool
	to     int
}

func (o TableOptions) orDefault() TableOptions {
	if o.CellMinWidth == 0 {
		o.CellMinWidth = 25
	}
	return o
}

func tableOptions(b *block) TableOptions {
	o, _ := b.typ.Spec.Extra["options"].(TableOptions)
	return o.orDefault()
}

// gridTracks are the table's columns: as wide as the cells say, at least the
// least a cell takes, else a share; a resize in progress is previewed.
func (v *rv) gridTracks(b *block) (core.GridTracks, core.GridTracks, bool) {
	n := b.node.Get()
	drag := v.tableDrag.Get()
	widths := ColumnWidths(n)
	o := tableOptions(b)
	if drag.table == b && drag.col && !drag.moving {
		widths[drag.index] = drag.size
	}
	cols := make([]string, len(widths))
	fixed := true
	for i, w := range widths {
		if w > 0 {
			cols[i] = fmt.Sprintf("%gpx", max(w, o.CellMinWidth))
		} else {
			cols[i] = fmt.Sprintf("minmax(%gpx, 1fr)", o.CellMinWidth)
			fixed = false
		}
	}
	rows := make([]string, n.ChildCount())
	for i := range rows {
		h := n.Child(i).AttrNum("height")
		if drag.table == b && !drag.col && !drag.moving && drag.index == i {
			h = drag.size
		}
		rows[i] = "auto"
		if h > 0 {
			rows[i] = fmt.Sprintf("minmax(%gpx, auto)", h)
		}
	}
	return core.GridTracks(strings.Join(cols, " ")), core.GridTracks(strings.Join(rows, " ")), fixed
}

func (v *rv) tableView(cx *core.Scope, b *block, look func() core.Style, class func() string) core.Node {
	cells := core.Each(b.kids.Get, func(k *block) int { return k.id }, func(cx *core.Scope, k func() *block, _ func() int) core.Node {
		return v.cellView(cx, k())
	})
	return core.View{Ref: b.ref, ClassFunc: class,
		OnLayout: func(core.Bounds) { v.geo.Update(func(i int) int { return i + 1 }) },
		StyleFunc: func() core.Style {
			cols, rows, fixed := v.gridTracks(b)
			st := core.Style{Display: core.DisplayGrid, GridTemplateColumns: cols, GridTemplateRows: rows}
			if fixed {
				st.AlignSelf = core.AlignStart
			}
			return st.Merge(look())
		},
		Children: []core.Node{cells}}
}

func (v *rv) cellView(cx *core.Scope, b *block) core.Node {
	selected := func() bool {
		v.rev.Get()
		c, ok := v.s.State().Selection.(*CellSelection)
		if !ok {
			return false
		}
		start := c.AnchorCell.Start(-1)
		m := TableMapOf(c.AnchorCell.Node(-1))
		if b.pos < start || b.pos >= c.AnchorCell.End(-1) {
			return false
		}
		return slices.Contains(m.CellsInRect(m.RectBetween(c.AnchorCell.Pos-start, c.HeadCell.Pos-start)), b.pos-start)
	}
	return core.View{Ref: b.ref,
		OnLayout: func(core.Bounds) { v.geo.Update(func(i int) int { return i + 1 }) },
		ClassFunc: func() string {
			_, c := v.nodeLook(b, v.looks.Get())
			return c
		},
		StyleFunc: func() core.Style {
			st, _ := v.nodeLook(b, v.looks.Get())
			r := b.rect.Get()
			return core.Style{Position: core.Relative, GridColumn: core.GridLines{Start: core.GridLine(r.Left + 1), End: core.GridLine(r.Right + 1)},
				GridRow: core.GridLines{Start: core.GridLine(r.Top + 1), End: core.GridLine(r.Bottom + 1)}}.Merge(st)
		},
		Children: []core.Node{v.kidViews(b), core.Show{When: selected, Then: func(cx *core.Scope) core.Node {
			z := core.Pt(0)
			return core.View{Class: v.p.Classes.SelectedCell, TestID: "rich-editor-selected-cell", PointerEvents: core.PointerEventsNone,
				StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Left: z, Top: z, Right: z, Bottom: z}.Merge(v.looks.Get().SelectedCell)
				}}
		}}}}
}

// tableGeometry is where a table's lines are in the content box.
type tableGeometry struct {
	box Rect2
	// cols and rows are each one's far edge and start.
	cols, rows         []float64
	colStart, rowStart []float64
}

func (v *rv) geometryOf(b *block) (tableGeometry, bool) {
	box, ok := v.boxOf(b)
	if !ok {
		return tableGeometry{}, false
	}
	m := TableMapOf(b.nowNode)
	g := tableGeometry{box: box, cols: make([]float64, m.Width), rows: make([]float64, m.Height), colStart: make([]float64, m.Width), rowStart: make([]float64, m.Height)}
	for i := range g.colStart {
		g.colStart[i], g.cols[i] = box.X, box.X
	}
	for i := range g.rowStart {
		g.rowStart[i], g.rows[i] = box.Y, box.Y
	}
	kids := b.kids.GetUntracked()
	for i, c := range m.Cells() {
		if i >= len(kids) {
			break
		}
		r, ok := v.boxOf(kids[i])
		if !ok {
			return tableGeometry{}, false
		}
		g.cols[c.Rect.Right-1] = max(g.cols[c.Rect.Right-1], r.X+r.Width)
		g.rows[c.Rect.Bottom-1] = max(g.rows[c.Rect.Bottom-1], r.Y+r.Height)
		g.colStart[c.Rect.Left] = max(g.colStart[c.Rect.Left], r.X)
		g.rowStart[c.Rect.Top] = max(g.rowStart[c.Rect.Top], r.Y)
	}
	return g, true
}

func at(x, y, w, h float64) core.Style {
	return core.Style{Position: core.Absolute, Left: core.Pt(core.Points(x)), Top: core.Pt(core.Points(y)), Width: core.Pt(core.Points(w)), Height: core.Pt(core.Points(h))}
}

// tableControls draws, over the content, each table's handles while the
// document can be edited: a band on each column's and row's far edge on
// desktops, and on the table the selection is in a grip over each column and
// beside each row, which a press selects and, on desktops, a drag moves.
func (v *rv) tableControls() core.Node {
	return core.Each(v.tableList.Get, func(b *block) int { return b.id }, func(cx *core.Scope, b func() *block, _ func() int) core.Node {
		return v.tableHandles(cx, b())
	})
}

func (v *rv) tableHandles(cx *core.Scope, b *block) core.Node {
	o := tableOptions(b)
	geometry := core.NewMemoFunc(cx, func() tableGeometry {
		v.geo.Get()
		v.rev.Get()
		g, _ := v.geometryOf(b)
		return g
	}, func(a, b tableGeometry) bool {
		return a.box == b.box && slices.Equal(a.cols, b.cols) && slices.Equal(a.rows, b.rows)
	})
	columns := func() []int { return indexes(len(geometry.Get().cols)) }
	rows := func() []int { return indexes(len(geometry.Get().rows)) }
	active := core.NewMemo(cx, func() bool {
		v.rev.Get()
		if !v.s.Editable() {
			return false
		}
		from := v.s.State().Selection.From()
		return from > b.pos && from < b.pos+b.nowNode.NodeSize()
	})
	desktop := core.Untracked(cx, cx.Env().Desktop)
	var kids []core.Node
	if desktop && !o.NotResizable {
		last := o.LastColumnNotResizable
		kids = append(kids,
			core.Each(columns, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
				if last && i() == len(geometry.GetUntracked().cols)-1 {
					return core.Group{}
				}
				return v.resizeBand(cx, b, geometry, true, i)
			}),
			core.Each(rows, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
				return v.resizeBand(cx, b, geometry, false, i)
			}))
	}
	kids = append(kids, core.Show{When: active.Get, Then: func(cx *core.Scope) core.Node {
		return core.Group{
			core.Each(columns, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
				return v.grip(cx, b, geometry, true, i, desktop)
			}),
			core.Each(rows, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
				return v.grip(cx, b, geometry, false, i, desktop)
			}),
			v.dropLine(b, geometry),
		}
	}})
	return core.Group(kids)
}

func indexes(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func (v *rv) resizeBand(cx *core.Scope, b *block, geometry *core.Memo[tableGeometry], col bool, i func() int) core.Node {
	o := tableOptions(b)
	name := func() string { return fmt.Sprintf("%d:%v:%d", b.id, col, i()) }
	shown := func() bool {
		d := v.tableDrag.Get()
		return v.hovered.Get() == name() || d.table == b && d.col == col && d.index == i() && !d.moving
	}
	start := 0.0
	var origin core.Point
	size := func(g tableGeometry) float64 {
		if col {
			return g.cols[i()] - g.colStart[i()]
		}
		return g.rows[i()] - g.rowStart[i()]
	}
	cursor := core.CursorColResize
	if !col {
		cursor = core.CursorRowResize
	}
	return core.View{NoFocus: true, TestID: "rich-editor-table-band",
		OnMouseEnter: func(core.MouseEvent) { v.hovered.Set(name()) },
		OnMouseLeave: func(core.MouseEvent) { v.hovered.Set("") },
		StyleFunc: func() core.Style {
			g := geometry.Get()
			if i() >= len(g.cols) && col || i() >= len(g.rows) && !col {
				return core.Style{Display: core.DisplayNone}
			}
			if col {
				return at(g.cols[i()]-bandWidth/2, g.box.Y, bandWidth, g.box.Height).Merge(core.Style{Cursor: cursor})
			}
			return at(g.box.X, g.rows[i()]-bandWidth/2, g.box.Width, bandWidth).Merge(core.Style{Cursor: cursor})
		},
		// A press on the band stays there, off the text under it.
		OnPressDown: func(core.PressEvent) {},
		Responder: &core.Responder{Claims: []core.TouchClaim{{OnMove: true}}, Keep: true,
			OnGrant: func(e core.ResponderEvent) {
				origin = e.PagePosition
				start = size(geometry.GetUntracked())
				v.tableDrag.Set(tableDragState{table: b, col: col, index: i(), size: start})
			},
			OnMove: func(e core.ResponderEvent) {
				d := float64(e.PagePosition.X - origin.X)
				least := o.CellMinWidth
				if !col {
					d, least = float64(e.PagePosition.Y-origin.Y), rowMinimum
				}
				v.tableDrag.Set(tableDragState{table: b, col: col, index: i(), size: max(start+d, least)})
			},
			OnRelease: func(core.ResponderEvent) {
				d := v.tableDrag.GetUntracked()
				v.tableDrag.Set(tableDragState{})
				if d.table != b {
					return
				}
				if col {
					v.s.Run(SetColumnWidth(d.index, d.size, b.pos))
				} else {
					v.s.Run(SetRowHeight(d.index, d.size, b.pos))
				}
			},
			OnTerminate: func(core.ResponderEvent) { v.tableDrag.Set(tableDragState{}) }},
		Children: []core.Node{core.Show{When: shown, Then: func(cx *core.Scope) core.Node {
			z := core.Pt(0)
			line := core.Style{Position: core.Absolute, Left: core.Pt(bandWidth/2 - 1), Top: z, Bottom: z, Width: core.Pt(2)}
			if !col {
				line = core.Style{Position: core.Absolute, Top: core.Pt(bandWidth/2 - 1), Left: z, Right: z, Height: core.Pt(2)}
			}
			return core.View{Class: v.p.Classes.TableResizeHandle, PointerEvents: core.PointerEventsNone,
				StyleFunc: func() core.Style { return line.Merge(v.looks.Get().TableResizeHandle) }}
		}}}}
}

// lineAt is the column or row a drag at p is over.
func lineAt(g tableGeometry, col bool, p float64) int {
	ends := g.rows
	if col {
		ends = g.cols
	}
	for i, e := range ends {
		if p < e {
			return i
		}
	}
	return len(ends) - 1
}

func (v *rv) grip(cx *core.Scope, b *block, geometry *core.Memo[tableGeometry], col bool, i func() int, desktop bool) core.Node {
	pressAt := 0.0
	var origin core.Point
	selectLine := func() {
		st := v.s.State()
		m := TableMapOf(b.nowNode)
		start := b.pos + 1
		if col {
			r := st.Doc.Resolve(start + m.Map[i()])
			v.s.Dispatch(st.Tr().SetSelection(ColSelection(r, r)))
		} else {
			r := st.Doc.Resolve(start + m.Map[i()*m.Width])
			v.s.Dispatch(st.Tr().SetSelection(RowSelection(r, r)))
		}
	}
	grip := core.View{NoFocus: true, TestID: "rich-editor-table-grip", Class: v.p.Classes.TableGrip,
		OnPress: func(core.PressEvent) { selectLine() },
		StyleFunc: func() core.Style {
			g := geometry.Get()
			if i() >= len(g.cols) && col || i() >= len(g.rows) && !col {
				return core.Style{Display: core.DisplayNone}
			}
			var st core.Style
			if col {
				st = at((g.colStart[i()]+g.cols[i()]-gripLength)/2, g.box.Y-gripBreadth/2, gripLength, gripBreadth)
			} else {
				st = at(g.box.X-gripBreadth/2, (g.rowStart[i()]+g.rows[i()]-gripLength)/2, gripBreadth, gripLength)
			}
			return st.Merge(v.looks.Get().TableGrip)
		}}
	if desktop {
		grip.Responder = &core.Responder{Claims: []core.TouchClaim{{OnMove: true}}, Keep: true,
			OnGrant: func(e core.ResponderEvent) {
				origin = e.PagePosition
				g := geometry.GetUntracked()
				if col {
					pressAt = (g.colStart[i()] + g.cols[i()]) / 2
				} else {
					pressAt = (g.rowStart[i()] + g.rows[i()]) / 2
				}
				v.tableDrag.Set(tableDragState{table: b, col: col, index: i(), moving: true, to: i()})
			},
			OnMove: func(e core.ResponderEvent) {
				d := float64(e.PagePosition.X - origin.X)
				if !col {
					d = float64(e.PagePosition.Y - origin.Y)
				}
				v.tableDrag.Set(tableDragState{table: b, col: col, index: i(), moving: true, to: lineAt(geometry.GetUntracked(), col, pressAt+d)})
			},
			OnRelease: func(core.ResponderEvent) {
				d := v.tableDrag.GetUntracked()
				v.tableDrag.Set(tableDragState{})
				if d.table != b || !d.moving || d.to == d.index {
					return
				}
				if col {
					v.s.Run(MoveTableColumn(d.index, d.to, b.pos))
				} else {
					v.s.Run(MoveTableRow(d.index, d.to, b.pos))
				}
			},
			OnTerminate: func(core.ResponderEvent) { v.tableDrag.Set(tableDragState{}) }}
	}
	return grip
}

func (v *rv) dropLine(b *block, geometry *core.Memo[tableGeometry]) core.Node {
	shown := func() bool {
		d := v.tableDrag.Get()
		return d.table == b && d.moving && d.to != d.index
	}
	return core.Show{When: shown, Then: func(cx *core.Scope) core.Node {
		return core.View{Class: v.p.Classes.TableDropIndicator, TestID: "rich-editor-table-drop", PointerEvents: core.PointerEventsNone,
			StyleFunc: func() core.Style {
				d := v.tableDrag.Get()
				g := geometry.Get()
				if d.col && d.to < len(g.cols) {
					x := g.colStart[d.to]
					if d.to > d.index {
						x = g.cols[d.to]
					}
					return at(x-1, g.box.Y, 2, g.box.Height).Merge(v.looks.Get().TableDropIndicator)
				}
				if !d.col && d.to < len(g.rows) {
					y := g.rowStart[d.to]
					if d.to > d.index {
						y = g.rows[d.to]
					}
					return at(g.box.X, y-1, g.box.Width, 2).Merge(v.looks.Get().TableDropIndicator)
				}
				return core.Style{Display: core.DisplayNone}
			}}
	}}
}
