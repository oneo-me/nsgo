package datatable

import (
	"math"
	"slices"
	"sort"
)

// The pure parts of DataTable: one axis's scaled scroll range, the
// columns' order and widths, what a key moves the selection to, and the
// window of rows or columns made.

// maxExtent is the longest a scroll view's content is made: platforms keep
// offsets and sizes in 32-bit floats, so past it the offset is scaled to
// the content's.
const maxExtent = 3_000_000.0

// tableAxis is one axis of a table: the content's and the viewport's
// lengths.
type tableAxis struct{ total, viewport float64 }

// extent is the length of the box the scroll view holds.
func (a tableAxis) extent() float64 { return math.Min(a.total, maxExtent) }

func (a tableAxis) scaled() bool { return a.total > maxExtent }

// ratio is content points per scroll point.
func (a tableAxis) ratio() float64 {
	if !a.scaled() {
		return 1
	}
	return math.Max(a.total-a.viewport, 0) / math.Max(a.extent()-a.viewport, 1)
}

// contentOffset is the content offset a scroll offset stands for.
func (a tableAxis) contentOffset(scroll float64) float64 {
	return clampF(scroll*a.ratio(), 0, math.Max(a.total-a.viewport, 0))
}

// scrollOffset is the scroll offset showing content at the start.
func (a tableAxis) scrollOffset(content float64) float64 {
	return clampF(content/a.ratio(), 0, math.Max(a.extent()-a.viewport, 0))
}

// scrollPoints is scrollOffset as the 32-bit float platforms keep, the
// first that does not stop short of content: far down a scaled axis one
// step of that float is several content points.
func (a tableAxis) scrollPoints(content float64) float32 {
	top := float32(math.Max(a.extent()-a.viewport, 0))
	at := min(float32(a.scrollOffset(content)), top)
	for at < top && a.contentOffset(float64(at)) < content-1e-6 {
		at = min(math.Nextafter32(at, float32(math.Inf(1))), top)
	}
	return at
}

// reveal is the content offset that brings start to end into view from
// offset from, moving the least.
func (a tableAxis) reveal(from, start, end float64) float64 {
	switch {
	case start < from || end-start > a.viewport:
		return start
	case end > from+a.viewport:
		return end - a.viewport
	}
	return from
}

func clampF(v, lo, hi float64) float64 { return math.Max(lo, math.Min(v, hi)) }

// columnSpec is a column's widths and whether it is fixed, at the left
// or with right at the right.
type columnSpec struct {
	width, min, max float32
	fixed, right    bool
}

// group is where the column goes: 0 the fixed ones at the left, 1 the
// scrolling ones, 2 the fixed ones at the right.
func (s columnSpec) group() int {
	switch {
	case s.fixed && s.right:
		return 2
	case s.fixed:
		return 0
	}
	return 1
}

func (s columnSpec) clamp(w float32) float32 { return max(s.min, min(w, max(s.max, s.min))) }

// tableColumns is a table's columns as shown: their order (fixed ones
// at the left first, those at the right last), widths and the prefix sums of the widths, so finding the column
// at an x is a binary search over a million columns. Columns are named by
// their definition index (def) or their place as shown (display).
type tableColumns struct {
	specs  []columnSpec
	widths []float32
	order  []int // def at each display index
	shown  []int // display index of each def
	starts []float64
	// fixed and right count the columns fixed at the left and the right.
	fixed, right int
}

func newTableColumns(specs []columnSpec) *tableColumns {
	c := &tableColumns{specs: specs, widths: make([]float32, len(specs)), order: make([]int, len(specs))}
	for i, s := range specs {
		c.widths[i] = s.clamp(s.width)
		c.order[i] = i
		switch s.group() {
		case 0:
			c.fixed++
		case 2:
			c.right++
		}
	}
	sort.SliceStable(c.order, func(a, b int) bool { return specs[c.order[a]].group() < specs[c.order[b]].group() })
	c.reindex()
	c.rebuild(0)
	return c
}

// updated is the columns of specs, keeping the widths a user dragged and
// the order they moved to for the columns that stay; oldOf gives a new
// column's old def, or -1.
func (c *tableColumns) updated(specs []columnSpec, oldOf func(def int) int) *tableColumns {
	next := newTableColumns(specs)
	position := make([]int, len(specs))
	for def, s := range specs {
		position[def] = math.MaxInt
		old := oldOf(def)
		if old < 0 || old >= len(c.widths) {
			continue
		}
		if c.specs[old].width == s.width {
			next.widths[def] = s.clamp(c.widths[old])
		}
		position[def] = c.shown[old]
	}
	sort.SliceStable(next.order, func(a, b int) bool {
		da, db := next.order[a], next.order[b]
		if ga, gb := specs[da].group(), specs[db].group(); ga != gb {
			return ga < gb
		}
		if position[da] != position[db] {
			return position[da] < position[db]
		}
		return da < db
	})
	next.reindex()
	next.rebuild(0)
	return next
}

func (c *tableColumns) reindex() {
	c.shown = make([]int, len(c.order))
	for display, def := range c.order {
		c.shown[def] = display
	}
}

func (c *tableColumns) rebuild(from int) {
	if len(c.starts) != len(c.order)+1 {
		c.starts = make([]float64, len(c.order)+1)
		from = 0
	}
	x := c.starts[from]
	for display := from; display < len(c.order); display++ {
		c.starts[display] = x
		x += float64(c.widths[c.order[display]])
	}
	c.starts[len(c.order)] = x
}

func (c *tableColumns) len() int                    { return len(c.order) }
func (c *tableColumns) def(display int) int         { return c.order[display] }
func (c *tableColumns) displayOf(def int) int       { return c.shown[def] }
func (c *tableColumns) width(def int) float32       { return c.widths[def] }
func (c *tableColumns) start(display int) float64   { return c.starts[display] }
func (c *tableColumns) displayWidth(d int) float32  { return c.widths[c.order[d]] }
func (c *tableColumns) total() float64              { return c.starts[len(c.order)] }
func (c *tableColumns) fixedWidth() float64         { return c.starts[c.fixed] }
func (c *tableColumns) scrollEnd() int              { return len(c.order) - c.right }
func (c *tableColumns) rightWidth() float64         { return c.total() - c.starts[c.scrollEnd()] }
func (c *tableColumns) scrollWidth() float64        { return c.starts[c.scrollEnd()] - c.fixedWidth() }
func (c *tableColumns) scrolls(display int) bool    { return display >= c.fixed && display < c.scrollEnd() }
func (c *tableColumns) hasDef(def int) bool         { return def >= 0 && def < len(c.shown) }
func (c *tableColumns) startOf(display int) float64 { return c.starts[min(display, len(c.order))] }

// displayAt is the column under x in the viewport, view wide, the
// scrolling columns scrolled by scrollX; -1 for none.
func (c *tableColumns) displayAt(x, scrollX, view float64) int {
	right := c.fixedWidth() + view
	switch {
	case c.right > 0 && x >= right:
		x = c.starts[c.scrollEnd()] + x - right
	case x >= c.fixedWidth():
		x += scrollX
		if x >= c.starts[c.scrollEnd()] {
			return -1
		}
	}
	return c.displayAtTable(x)
}

// displayAtTable is the column under x in the unscrolled table.
func (c *tableColumns) displayAtTable(x float64) int {
	if x < 0 || x >= c.total() {
		return -1
	}
	n := len(c.order)
	return sort.Search(n, func(i int) bool { return c.starts[i] > x }) - 1
}

// visible is the display indexes of the scrolling columns from to to, in
// table coordinates.
func (c *tableColumns) visible(from, to float64) (int, int) {
	n := c.scrollEnd()
	if c.fixed >= n || to <= from || from >= c.starts[n] {
		return n, n
	}
	first := max(sort.Search(n, func(i int) bool { return c.starts[i] > from })-1, c.fixed)
	end := max(sort.Search(n, func(i int) bool { return c.starts[i] >= to }), first)
	return first, min(end, n)
}

// setWidth sets def's width within its limits; it reports a change.
func (c *tableColumns) setWidth(def int, w float32) bool {
	w = c.specs[def].clamp(w)
	if c.widths[def] == w {
		return false
	}
	c.widths[def] = w
	c.rebuild(c.shown[def])
	return true
}

// groupOf is the group the column at display is in (columnSpec.group).
func (c *tableColumns) groupOf(display int) int {
	switch {
	case display < c.fixed:
		return 0
	case display >= c.scrollEnd():
		return 2
	}
	return 1
}

// groupRange is the display indexes of display's group, first to last.
func (c *tableColumns) groupRange(display int) (int, int) {
	switch c.groupOf(display) {
	case 0:
		return 0, max(c.fixed-1, 0)
	case 2:
		return c.scrollEnd(), len(c.order) - 1
	}
	return c.fixed, max(c.scrollEnd()-1, c.fixed)
}

// moveTo moves the column at display from to display to, within the
// columns fixed at either side or the others; it reports a move.
func (c *tableColumns) moveTo(from, to int) bool {
	n := len(c.order)
	if from < 0 || to < 0 || from >= n || to >= n || from == to || c.groupOf(from) != c.groupOf(to) {
		return false
	}
	def := c.order[from]
	c.order = slices.Delete(c.order, from, from+1)
	c.order = slices.Insert(c.order, to, def)
	c.reindex()
	c.rebuild(min(from, to))
	return true
}

// tableSel is a table's selection: nothing, a row, a column (a def) or a
// cell.
type tableSel struct {
	kind     selKind
	row, col int
}

type selKind uint8

const (
	selNone selKind = iota
	selRow
	selColumn
	selCell
)

func (s tableSel) rowOf() (int, bool) { return s.row, s.kind == selRow || s.kind == selCell }
func (s tableSel) colOf() (int, bool) { return s.col, s.kind == selColumn || s.kind == selCell }

// selectable is what a table selects.
type selectable struct{ rows, columns, cells bool }

func (s tableSel) fits(a selectable) bool {
	switch s.kind {
	case selRow:
		return a.rows
	case selColumn:
		return a.columns
	case selCell:
		return a.cells
	}
	return true
}

// clampSel is s if the table selects its kind and has its row and column.
func clampSel(s tableSel, a selectable, rows, columns int) tableSel {
	r, hasRow := s.rowOf()
	c, hasCol := s.colOf()
	if !s.fits(a) || hasRow && r >= rows || hasCol && c >= columns {
		return tableSel{}
	}
	return s
}

type tableMove uint8

const (
	moveUp tableMove = iota
	moveDown
	moveLeft
	moveRight
	movePageUp
	movePageDown
	moveFirst
	moveLast
)

// tableNav is what a key moves the selection to: over rows, over the
// selectable columns in display order, or over cells.
type tableNav struct {
	sel     selectable
	rows    int
	columns []int // selectable defs, in display order
	page    int
	wrap    bool
}

func (n tableNav) step(from tableSel, m tableMove) (tableSel, bool) {
	vertical := m == moveUp || m == moveDown || m == movePageUp || m == movePageDown
	horizontal := m == moveLeft || m == moveRight
	switch {
	case from.kind == selRow && n.sel.rows && !horizontal:
		return n.stepRow(from, m)
	case from.kind == selColumn && n.sel.columns && !vertical:
		return n.stepColumn(from, m)
	case n.sel.cells:
		return n.stepCell(from, m)
	case n.sel.rows && !horizontal:
		return n.stepRow(from, m)
	case n.sel.columns && !vertical:
		return n.stepColumn(from, m)
	}
	return tableSel{}, false
}

func (n tableNav) stepRow(from tableSel, m tableMove) (tableSel, bool) {
	if n.rows == 0 {
		return tableSel{}, false
	}
	row := -1
	if from.kind == selRow {
		row = min(from.row, n.rows-1)
	}
	to, ok := n.vertical(row, m)
	return tableSel{kind: selRow, row: to}, ok
}

func (n tableNav) stepColumn(from tableSel, m tableMove) (tableSel, bool) {
	at := -1
	if from.kind == selColumn {
		at = slices.Index(n.columns, from.col)
	}
	to, ok := n.horizontal(at, m)
	if !ok {
		return tableSel{}, false
	}
	return tableSel{kind: selColumn, col: n.columns[to]}, true
}

func (n tableNav) stepCell(from tableSel, m tableMove) (tableSel, bool) {
	if n.rows == 0 || len(n.columns) == 0 {
		return tableSel{}, false
	}
	row, at := -1, -1
	switch from.kind {
	case selCell:
		row, at = min(from.row, n.rows-1), slices.Index(n.columns, from.col)
	case selRow:
		row = min(from.row, n.rows-1)
	case selColumn:
		at = slices.Index(n.columns, from.col)
	default:
		return tableSel{kind: selCell, row: 0, col: n.columns[0]}, true
	}
	switch m {
	case moveLeft, moveRight, moveFirst, moveLast:
		to, ok := n.horizontal(at, m)
		return tableSel{kind: selCell, row: max(row, 0), col: n.columns[to]}, ok
	}
	to, ok := n.vertical(row, m)
	return tableSel{kind: selCell, row: to, col: n.columns[max(at, 0)]}, ok
}

// vertical is where row (-1 for none) goes.
func (n tableNav) vertical(row int, m tableMove) (int, bool) {
	last := n.rows - 1
	if row < 0 {
		if m == moveLast {
			return last, true
		}
		return 0, true
	}
	page := max(n.page, 1)
	switch m {
	case moveUp:
		if row > 0 {
			return row - 1, true
		}
		if n.wrap {
			return last, true
		}
		return row, true
	case moveDown:
		if row < last {
			return row + 1, true
		}
		if n.wrap {
			return 0, true
		}
		return row, true
	case movePageUp:
		return max(row-page, 0), true
	case movePageDown:
		return min(row+page, last), true
	case moveFirst:
		return 0, true
	case moveLast:
		return last, true
	}
	return 0, false
}

// horizontal is where the column at index at of the selectable ones (-1
// for none) goes.
func (n tableNav) horizontal(at int, m tableMove) (int, bool) {
	if len(n.columns) == 0 {
		return 0, false
	}
	last := len(n.columns) - 1
	if at < 0 {
		if m == moveLast {
			return last, true
		}
		return 0, true
	}
	switch m {
	case moveLeft:
		if at > 0 {
			return at - 1, true
		}
		if n.wrap {
			return last, true
		}
		return at, true
	case moveRight:
		if at < last {
			return at + 1, true
		}
		if n.wrap {
			return 0, true
		}
		return at, true
	case moveFirst:
		return 0, true
	case moveLast:
		return last, true
	}
	return 0, false
}

// slotItem is the item slot shows of the window start to end over count
// slots: the one whose index leaves slot divided by count; -1 for none.
// A window that moves by k items gives k slots other items, so no view is
// made or dropped while scrolling.
func slotItem(slot, count, start, end int) int {
	if count == 0 {
		return -1
	}
	item := start + (slot+count-start%count)%count
	if item >= end {
		return -1
	}
	return item
}
