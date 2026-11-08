package datatable

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/base/internal/window"
	"github.com/oneo-me/nsgo/core"
)

const (
	// tableColumnWidth, tableMinWidth and tableMaxWidth are a column's
	// width and limits unless it gives its own.
	tableColumnWidth = 120
	tableMinWidth    = 20
	tableMaxWidth    = 1200
	// sortWidth is the room a sortable title keeps for its sort button.
	sortWidth = 20
	// fitRows is the rows around the view a column's fit measures, at most.
	fitRows = 500
	// copyRows is the rows a selected column's copy takes, at most.
	copyRows = 100_000
	// moveSlop is how far a title's drag goes before it moves its column.
	moveSlop = 4
	// handleDoublePress is the longest between two presses of a resize
	// handle that fit its column.
	handleDoublePress = 400 * time.Millisecond
	// rowChunk and columnChunk are the rows and columns a scaled table
	// places its rows and columns relative to.
	rowChunk    = 1024
	columnChunk = 256
)

// Align is where a column's cells and title put their content.
type Align uint8

const (
	AlignStartColumn Align = iota
	AlignCenterColumn
	AlignEndColumn
)

// Column is a column of a DataTable.
type Column struct {
	// Key names the column among the others: the name events and the ref
	// use.
	Key string
	// Title is its title's text; Key when empty.
	Title string
	// Width is its width (120 when 0); MinWidth and MaxWidth bound what a
	// drag or a fit makes it (20 and 1200 when 0).
	Width, MinWidth, MaxWidth core.Points
	// FitPadding is room for the app's padding, icons or badges, added to
	// the widest title or cell text when fitting the column.
	FitPadding core.Points
	Align      Align
	// Sortable gives its title a sort button that asks for a sort.
	Sortable bool
	// Fixed keeps it at the left while the other columns scroll under it;
	// fixed columns lead, in their order.
	Fixed bool
	// NoResize and NoMove keep a drag of its edge or title from resizing
	// or moving it; NoSelect keeps it and its cells out of selection (a
	// column of buttons).
	NoResize, NoMove, NoSelect bool
	// CellStyle styles its cells after the Cell part, HeaderStyle its
	// title after the HeaderCell part, and PositiveStyle and NegativeStyle
	// its cells that hold a rise or a fall: a number above or below zero,
	// or a text starting with + or -. Their classes go under them.
	CellClass, HeaderClass       string
	PositiveClass, NegativeClass string
	CellStyle, HeaderStyle       core.Style
	PositiveStyle, NegativeStyle core.Style
}

func (c Column) title() string {
	if c.Title != "" {
		return c.Title
	}
	return c.Key
}

func (c Column) spec() columnSpec {
	or := func(v core.Points, d float32) float32 {
		if v == 0 {
			return d
		}
		return float32(v)
	}
	lo := max(or(c.MinWidth, tableMinWidth), 0)
	return columnSpec{width: or(c.Width, tableColumnWidth), min: lo, max: max(or(c.MaxWidth, tableMaxWidth), lo), fixed: c.Fixed}
}

// ColumnGroup is a label over Span columns (1 when 0), or over
// Span groups of the level below.
type ColumnGroup struct {
	Label string
	Span  int
}

// Sort is what the data is sorted by: a column's key and the
// direction. The zero value is unsorted.
type Sort struct {
	Key        string
	Descending bool
}

// Selection is what a DataTable selects: a row, a column (its
// index in Columns), or both for a cell; -1 for neither. NoTableSelection
// selects nothing.
type Selection struct{ Row, Column int }

// NoTableSelection selects nothing.
var NoTableSelection = Selection{-1, -1}

func (s Selection) sel() tableSel {
	switch {
	case s.Row >= 0 && s.Column >= 0:
		return tableSel{kind: selCell, row: s.Row, col: s.Column}
	case s.Row >= 0:
		return tableSel{kind: selRow, row: s.Row}
	case s.Column >= 0:
		return tableSel{kind: selColumn, col: s.Column}
	}
	return tableSel{}
}

func (s tableSel) public() Selection {
	out := NoTableSelection
	if r, ok := s.rowOf(); ok {
		out.Row = r
	}
	if c, ok := s.colOf(); ok {
		out.Column = c
	}
	return out
}

// Press is a press of a row: the row, the column under it (-1 on
// a row number), the clicks in a row and the modifier keys.
type Press struct {
	Row, Column int
	Clicks      int
	Modifiers   core.Modifiers
}

// ContextMenu is where a context menu was asked for: the row and
// column under the pointer (-1 for none: a title has no row, a row number
// no column) and the point in the root view to open the app's Menu at.
type ContextMenu struct {
	Row, Column int
	At          core.Point
}

// ColumnMove is a column the user moved: its key, from and to its
// places as shown, and every column's key in the order shown.
type ColumnMove struct {
	Key      string
	From, To int
	Order    []string
}

// Cell is what RenderCell draws: a cell, its text as the table
// would show it, and whether it, or its row, is selected.
type Cell struct {
	Row, Column int
	Key, Text   string
	Selected    bool
}

// HeaderCell is what RenderHeaderCell draws: a column's title,
// its sort and whether the column is selected.
type HeaderCell struct {
	Column     int
	Key, Title string
	Sorted     bool
	Descending bool
	Selected   bool
}

// Dump is the titles and the cells' texts, as shown.
type Dump struct {
	Headers []string
	Rows    [][]string
}

// Styles are the styles of a DataTable's parts, which supply its
// whole look; a state's part merges over the part it follows.
type Styles struct {
	// Header is the header across the table; GroupCell a group label;
	// HeaderCell every title, HeaderContent the row of its label and sort
	// button, and HeaderCellDragging, HeaderCellSorted and
	// HeaderCellSelected the dragged, sorted and selected column's title.
	Header, GroupCell, HeaderCell, HeaderContent             core.Style
	HeaderCellDragging, HeaderCellSorted, HeaderCellSelected core.Style
	// SortIndicator is a sortable column's sort button; SortAscending and
	// SortDescending its chevrons, whose Selected state is the sorted
	// direction's.
	SortIndicator, SortAscending, SortDescending core.Style
	// ResizeHandle is the line at a column's trailing edge a drag resizes;
	// MoveIndicator the line where a dragged column will drop.
	ResizeHandle, MoveIndicator core.Style
	// Row is every body row; RowOdd the odd ones (Stripe), RowHover the one
	// under the pointer, RowSelected the selected one and RowActive the
	// one a context menu was asked for where cells do not select.
	Row, RowOdd, RowHover, RowSelected, RowActive core.Style
	// Cell is every body cell; CellSelected the selected one, CellActive
	// the one a context menu was asked for, CellColumnSelected those of the
	// selected column.
	Cell, CellSelected, CellActive, CellColumnSelected core.Style
	// RowHeader is a row number's cell and RowHeaderCorner the corner
	// above them; FixedEdge the line at the fixed columns' trailing edge;
	// Empty the box of the Empty node.
	RowHeader, RowHeaderCorner, FixedEdge, Empty core.Style
}

// Classes are the Tailwind classes of a DataTable's parts, named
// as Styles names them, each under its part's style; a state's
// part follows the part it follows.
type Classes struct {
	// Root is the table's box, under its Class.
	Root                                                     string
	Header, GroupCell, HeaderCell, HeaderContent             string
	HeaderCellDragging, HeaderCellSorted, HeaderCellSelected string
	SortIndicator, SortAscending, SortDescending             string
	ResizeHandle, MoveIndicator                              string
	Row, RowOdd, RowHover, RowSelected, RowActive            string
	Cell, CellSelected, CellActive, CellColumnSelected       string
	RowHeader, RowHeaderCorner, FixedEdge, Empty             string
}

// DataTable shows rows and columns as a spreadsheet does and scrolls both
// ways over any number of either, as gpui-component's: only the cells in
// view and a reach around them exist as views, made once and given other
// rows and columns as it scrolls. Rows, columns or cells select with the
// pointer and the keyboard; columns resize with a drag of their edge (a
// double press fits them), move with a drag of their title, stay at the
// left when Fixed and ask for a sort. It has no look of its own: Styles
// gives every part its padding, colors and borders.
//
// The rows are the app's: Text gives a cell's text, read where the cells
// in view show it, so a change of what it reads updates those cells only;
// OnVisibleRowsChange tells the app which rows to have ready. Sorting is
// the app's too: OnSortChange asks, and Sort shows what the app did.
// Past a few million points of content the scroll range is scaled, so a
// million rows still scroll to every row.
type DataTable struct {
	Columns func() []Column
	// RowCount is how many rows there are; Text the text of a row's cell
	// in a column, by its index in Columns.
	RowCount func() int
	Text     func(row, column int) string
	// Data holds the rows from DataStart, read by the columns' keys, in
	// place of Text: rows outside it show empty until it has them, and
	// RowCount, when nil, is where it ends.
	Data      func() *Rows
	DataStart func() int
	// RowStyle is a row's own look, after the row's parts, and RowClass
	// its classes, after the row's parts' classes.
	RowClass func(row int) string
	RowStyle func(row int) core.Style
	// RowHeight is a body row's height (32 when 0); HeaderHeight a title
	// row's (RowHeight when 0).
	RowHeight, HeaderHeight core.Points
	// Groups are labels above the columns, the lowest level first.
	Groups func() [][]ColumnGroup
	Sort   func() Sort
	// What presses and keys select: rows and columns unless NoRowSelect
	// and NoColumnSelect, cells while CellSelect returns true. A press with
	// Shift or the command key held selects a row unless
	// NoSelectOnModifierPress, for an app keeping its own selection of
	// several rows with OnRowPress.
	NoRowSelect, NoColumnSelect bool
	CellSelect                  func() bool
	NoSelectOnModifierPress     bool
	// Selection, when set, is the selection; the table keeps its own
	// otherwise.
	Selection func() Selection
	// NoLoopSelection stops the arrow keys at the ends.
	NoLoopSelection bool
	// RowHeader shows a column of row numbers at the left, which selects
	// rows; it shows by default with CellSelect, unless NoRowHeader.
	RowHeader, NoRowHeader bool
	// Stripe gives the odd rows the RowOdd part while it returns true.
	Stripe func() bool
	// NoResizeColumns and NoMoveColumns keep every column from being
	// resized or moved.
	NoResizeColumns, NoMoveColumns bool
	// Loading shows LoadingView over the rows and holds OnLoadMore, which
	// fires once per number of rows as the rows in view come within
	// LoadMoreThreshold (20 when 0) of the last while HasMore.
	Loading           func() bool
	HasMore           func() bool
	LoadMoreThreshold int
	// FontFamily and FontSize (14 when 0) are the cells' font, which
	// fitting a column measures in.
	FontFamily string
	FontSize   core.Points
	// NoScrollbar hides the scroll bars; NoVerticalScrollbar and
	// NoHorizontalScrollbar one of them. NoBounce stops the bounce at the
	// ends.
	NoScrollbar, NoVerticalScrollbar, NoHorizontalScrollbar bool
	NoBounce                                                bool
	// Bar draws the scroll bars, over the app's (Scope.SetScrollbars).
	Bar core.ScrollbarMaker
	// RenderCell and RenderHeaderCell draw the cells and the titles in
	// place of their texts, once per slot, the cell it shows changing as
	// the table scrolls.
	RenderCell       func(cx *core.Scope, cell func() Cell) core.Node
	RenderHeaderCell func(cx *core.Scope, cell func() HeaderCell) core.Node
	// Empty shows when there are no rows; LoadingView while Loading.
	Empty, LoadingView core.Node
	Classes            Classes
	Styles             Styles
	StylesFunc         func() Styles
	// Label is what screen readers call the table, a grid of column
	// headers and cells.
	Label     string
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Ref       *Ref
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node

	OnSelectionChange func(Selection)
	OnRowPress        func(Press)
	// OnDoublePress receives a row, a cell or a title pressed twice: its
	// row and column, -1 for none.
	OnDoublePress func(row, column int)
	OnContextMenu func(ContextMenu)
	// OnSortChange receives the next sort of the cycle unsorted, ascending,
	// descending as a sort button is pressed; the zero value clears it.
	OnSortChange func(Sort)
	// OnColumnResize receives a column's width at every step of a drag
	// and when a fit sets it.
	OnColumnResize func(key string, width core.Points)
	OnColumnMove   func(ColumnMove)
	// OnVisibleRowsChange and OnVisibleColumnsChange receive the rows and
	// the scrolling columns (display indexes) in view, end excluded.
	OnVisibleRowsChange    func(start, end int)
	OnVisibleColumnsChange func(start, end int)
	OnLoadMore             func()
}

// Ref is a DataTable's methods, bound once it mounts.
type Ref struct{ t *dataTable }

// ScrollToRow scrolls row into view.
func (r *Ref) ScrollToRow(row int, animated bool) {
	if r.t != nil {
		r.t.scrollToRow(row, animated)
	}
}

// ScrollToColumn scrolls the column key into view.
func (r *Ref) ScrollToColumn(key string, animated bool) {
	if r.t != nil {
		r.t.scrollToColumn(key, animated)
	}
}

// Selection returns what is selected.
func (r *Ref) Selection() Selection {
	if r.t == nil {
		return NoTableSelection
	}
	return r.t.st.sel.public()
}

// Select selects s and brings it into view, without OnSelectionChange.
func (r *Ref) Select(s Selection) {
	if r.t != nil {
		r.t.selectPublic(s, false)
	}
}

// VisibleRows returns the rows in view, end excluded.
func (r *Ref) VisibleRows() (start, end int) {
	if r.t == nil {
		return 0, 0
	}
	r.t.cx.Untrack(func() { start, end = r.t.visibleRows() })
	return
}

// VisibleColumns returns the scrolling columns in view, as display
// indexes, end excluded.
func (r *Ref) VisibleColumns() (start, end int) {
	if r.t == nil {
		return 0, 0
	}
	r.t.cx.Untrack(func() { start, end = r.t.visibleCols() })
	return
}

// ColumnWidths returns the widths now, in the order of Columns.
func (r *Ref) ColumnWidths() []core.Points {
	if r.t == nil {
		return nil
	}
	out := make([]core.Points, len(r.t.st.columns.widths))
	for i, w := range r.t.st.columns.widths {
		out[i] = core.Points(w)
	}
	return out
}

// SetColumnWidth sets the column key's width, within its limits.
func (r *Ref) SetColumnWidth(key string, width core.Points) {
	if r.t != nil {
		if def := r.t.defOf(key); def >= 0 {
			r.t.setWidth(def, float32(width), false)
		}
	}
}

// FitColumn sets the column key as wide as its widest text and title
// need, among the rows in view and up to 500 more each side, and returns
// the width; false for no such column.
func (r *Ref) FitColumn(key string) (core.Points, bool) {
	if r.t == nil {
		return 0, false
	}
	def := r.t.defOf(key)
	if def < 0 {
		return 0, false
	}
	return core.Points(r.t.fit(def)), true
}

// Dump returns the titles and the texts of rows start to end, as shown.
func (r *Ref) Dump(start, end int) Dump {
	if r.t == nil {
		return Dump{}
	}
	return r.t.dump(start, end)
}

// Copy puts the selection's text on the clipboard: a row's cells with tabs
// between, a column's with line breaks, a cell's text.
func (r *Ref) Copy() {
	if r.t != nil {
		r.t.copySelection()
	}
}

// Focus gives the table the keyboard, and Blur takes it back.
func (r *Ref) Focus() {
	if r.t != nil {
		r.t.focused.Set(true)
	}
}

// Blur takes the keyboard from the table.
func (r *Ref) Blur() {
	if r.t != nil {
		r.t.focused.Set(false)
	}
}

// MenuTarget returns what the last context menu was asked for at: a
// cell, a row or a column, as a selection names them; false when no mark
// is shown.
func (r *Ref) MenuTarget() (Selection, bool) {
	if r.t == nil || !r.t.st.active {
		return NoTableSelection, false
	}
	return Selection{Row: r.t.st.activeRow, Column: r.t.st.activeCol}, true
}

// ClearMenuTarget drops the context menu's mark, as the app's menu closes.
func (r *Ref) ClearMenuTarget() {
	if r.t != nil && r.t.st.active {
		r.t.st.active = false
		r.t.bumpData()
	}
}

// tableState is a table's state that is not a signal: bindings follow it
// through the revisions.
type tableState struct {
	columns *tableColumns
	defs    []Column
	// toned is whether each def has styles or classes for rises and falls.
	toned []bool
	sel   tableSel
	// active marks the row and column (-1 for none) a context menu was
	// asked for.
	active               bool
	activeRow, activeCol int
	digitFont            core.Font
	digit                float32
	reportedRows         [2]int
	reportedCols         [2]int
	reported             bool
	loadMoreAt           int
}

// tableDrag is a title's drag moving its column.
type tableDrag struct {
	from, to int
	moving   bool
}

// tableGeo is what layout and the windows need, read at one moment.
type tableGeo struct {
	rowHeight, header, lead, fixed float32
	y, x                           tableAxis
	// cx and cy are the content offsets.
	cx, cy float64
}

type dataTable struct {
	p  DataTable
	cx *core.Scope
	st *tableState
	// layoutRev counts changes of the columns, their widths and order;
	// dataRev of what is drawn over the data (selection, marks).
	layoutRev, dataRev *core.Signal[int]
	scrollX, scrollY   *core.Signal[core.Points]
	viewport           *core.Signal[core.Size]
	hover              *core.Signal[int]
	focused            *core.Signal[bool]
	drag               *core.Signal[tableDrag]
	// sizes is the geometry without the offsets, so what reads only sizes
	// does not run again at each scroll.
	sizes            *core.Memo[tableGeo]
	vscroll, hscroll *core.Ref
	stripRef         *core.Ref
	colsWin          *window.Rows
	resizing         int
	resizeFrom       float32
	handleTap        int
	handleTapAt      time.Time
	// hlead is which of the horizontal strip (true) and the horizontal
	// scroll view moved last.
	hlead tableLead
	// follow moves the body with the scroll views on the main thread
	// (datatable_view.go).
	follow tableFollow
}

func (t *dataTable) dataStart() int {
	if t.p.DataStart == nil {
		return 0
	}
	return max(t.p.DataStart(), 0)
}

func newDataTable(cx *core.Scope, p DataTable) *dataTable {
	t := &dataTable{p: p, cx: cx, st: &tableState{columns: newTableColumns(nil), loadMoreAt: -1},
		layoutRev: core.NewSignal(cx, 0), dataRev: core.NewSignal(cx, 0),
		scrollX: core.NewSignal(cx, core.Points(0)), scrollY: core.NewSignal(cx, core.Points(0)),
		viewport: core.NewSignal(cx, core.Size{}), hover: core.NewSignal(cx, -1), focused: core.NewSignal(cx, false),
		drag: core.NewSignal(cx, tableDrag{from: -1}), vscroll: &core.Ref{}, hscroll: &core.Ref{},
		stripRef: &core.Ref{},
		colsWin:  window.New(1, 3), resizing: -1, handleTap: -1}
	if p.Ref != nil {
		p.Ref.t = t
	}
	t.syncColumns()
	t.sizes = core.NewMemo(cx, t.measureSizes)
	t.follow = newTableFollow(cx, t)
	t.syncSelection()
	t.report()
	return t
}

// styles are the parts' styles, read where they are applied.
func (t *dataTable) styles() *Styles {
	if t.p.StylesFunc != nil {
		s := t.p.StylesFunc()
		return &s
	}
	return &t.p.Styles
}

func (t *dataTable) bumpLayout() { t.layoutRev.Update(func(r int) int { return r + 1 }) }
func (t *dataTable) bumpData()   { t.dataRev.Update(func(r int) int { return r + 1 }) }

// syncColumns follows the app's columns, keeping the widths a user dragged
// and the order they moved to for the columns that stay.
func (t *dataTable) syncColumns() {
	t.cx.Effect(func() {
		var defs []Column
		if t.p.Columns != nil {
			defs = t.p.Columns()
		}
		t.cx.Untrack(func() {
			st := t.st
			old := st.defs
			index := map[string]int{}
			for i, d := range old {
				index[d.Key] = i
			}
			specs := make([]columnSpec, len(defs))
			for i, d := range defs {
				specs[i] = d.spec()
			}
			oldOf := func(def int) int {
				if i, ok := index[defs[def].Key]; ok {
					return i
				}
				return -1
			}
			st.columns = st.columns.updated(specs, oldOf)
			// A selected column stays the column of its key.
			kept := func(c int) int {
				if c < 0 || c >= len(old) {
					return -1
				}
				return slices.IndexFunc(defs, func(d Column) bool { return d.Key == old[c].Key })
			}
			if c, ok := st.sel.colOf(); ok {
				if k := kept(c); k < 0 {
					st.sel = tableSel{}
				} else {
					st.sel.col = k
				}
			}
			st.defs = defs
			st.toned = make([]bool, len(defs))
			for i, c := range defs {
				st.toned[i] = !c.PositiveStyle.IsZero() || !c.NegativeStyle.IsZero() || c.PositiveClass != "" || c.NegativeClass != ""
			}
			t.bumpLayout()
			t.bumpData()
		})
	})
}

// syncSelection follows the app's selection and keeps the selection
// within the rows and columns there are and of a kind the table selects.
func (t *dataTable) syncSelection() {
	if t.p.Selection != nil {
		t.cx.Effect(func() {
			given := t.p.Selection()
			t.cx.Untrack(func() {
				t.st.sel = given.sel()
				t.bumpData()
			})
		})
	}
	t.cx.Effect(func() {
		rows := t.rowCount()
		t.layoutRev.Get()
		t.cx.Untrack(func() {
			next := clampSel(t.st.sel, t.selectable(), rows, t.st.columns.len())
			if next != t.st.sel {
				t.st.sel = next
				t.bumpData()
			}
		})
	})
}

// selectable is what the table selects (tracked).
func (t *dataTable) selectable() selectable {
	return selectable{rows: !t.p.NoRowSelect, columns: !t.p.NoColumnSelect, cells: t.cellSelect()}
}

func (t *dataTable) rowHeaderShown() bool {
	return !t.p.NoRowHeader && (t.p.RowHeader || t.cellSelect())
}

func (t *dataTable) cellSelect() bool { return t.p.CellSelect != nil && t.p.CellSelect() }

func (t *dataTable) rowCount() int {
	if t.p.RowCount == nil && t.p.Data != nil {
		rows := t.p.Data()
		if rows == nil {
			return 0
		}
		return t.dataStart() + rows.Len()
	}
	if t.p.RowCount == nil {
		return 0
	}
	return max(t.p.RowCount(), 0)
}

// cellText is the text of row's cell in column def (tracked).
func (t *dataTable) cellText(row, def int) string {
	if t.p.Data != nil && t.p.Text == nil && def >= 0 && def < len(t.st.defs) {
		if rows := t.p.Data(); rows != nil {
			return rows.Text(row-t.dataStart(), t.st.defs[def].Key)
		}
		return ""
	}
	if t.p.Text == nil || row < 0 || def < 0 || def >= len(t.st.defs) {
		return ""
	}
	return t.p.Text(row, def)
}

// cellSign is +1 for a rise, -1 for a fall and 0 otherwise: the text's
// sign, or the number's.
func cellSign(text string) int {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return 0
	case text[0] == '+':
		return 1
	case text[0] == '-' || strings.HasPrefix(text, "−"):
		return -1
	}
	if v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSuffix(text, "%"), ",", ""), 64); err == nil {
		switch {
		case v > 0:
			return 1
		case v < 0:
			return -1
		}
	}
	return 0
}

func (t *dataTable) title(def int) string {
	if def < 0 || def >= len(t.st.defs) {
		return ""
	}
	return t.st.defs[def].title()
}

func (t *dataTable) defOf(key string) int {
	return slices.IndexFunc(t.st.defs, func(d Column) bool { return d.Key == key })
}

func (t *dataTable) rowHeight() float32 {
	if t.p.RowHeight <= 0 {
		return 32
	}
	return max(float32(t.p.RowHeight), 1)
}

func (t *dataTable) headerRowHeight() float32 {
	if t.p.HeaderHeight <= 0 {
		return t.rowHeight()
	}
	return max(float32(t.p.HeaderHeight), 1)
}

func (t *dataTable) headerLevels() int {
	if t.p.Groups == nil {
		return 1
	}
	return len(t.p.Groups()) + 1
}

func (t *dataTable) font() core.Font {
	size := t.p.FontSize
	if size == 0 {
		size = 14
	}
	return core.Font{Family: t.p.FontFamily, Size: size}
}

// leadWidth is the row numbers' width (tracked): enough for the largest.
func (t *dataTable) leadWidth() float32 {
	if !t.rowHeaderShown() {
		return 0
	}
	digits := len(strconv.Itoa(max(t.rowCount(), 1)))
	font := t.font()
	if t.st.digit == 0 || t.st.digitFont != font {
		t.st.digitFont, t.st.digit = font, t.cx.MeasureText("0000000000", font).Width/10
	}
	return max(t.st.digit*float32(digits), 40)
}

// geo is the sizes and offsets now (tracked).
func (t *dataTable) geo() tableGeo {
	g := t.sizes.Get()
	g.cx = g.x.contentOffset(float64(t.scrollX.Get()))
	g.cy = g.y.contentOffset(float64(t.scrollY.Get()))
	return g
}

// measureSizes is the geometry without the offsets.
func (t *dataTable) measureSizes() tableGeo {
	t.layoutRev.Get()
	size := t.viewport.Get()
	w, h := max(float32(size.Width), 0), max(float32(size.Height), 0)
	rh := t.rowHeight()
	g := tableGeo{rowHeight: rh, header: t.headerRowHeight() * float32(t.headerLevels()), lead: t.leadWidth(),
		fixed: float32(t.st.columns.fixedWidth())}
	g.y = tableAxis{float64(t.rowCount()) * float64(rh), float64(max(h-g.header, 0))}
	g.x = tableAxis{t.st.columns.scrollWidth(), float64(max(w-g.lead-g.fixed, 0))}
	return g
}

// visibleRows is the rows in view (tracked).
func (t *dataTable) visibleRows() (int, int) {
	g := t.geo()
	rows := t.rowCount()
	rh := float64(g.rowHeight)
	first := min(int(math.Max(math.Floor(g.cy/rh), 0)), rows)
	last := min(int(math.Max(math.Ceil((g.cy+g.y.viewport)/rh), 0)), rows)
	return first, max(last, first)
}

// visibleCols is the display indexes of the scrolling columns in view
// (tracked).
func (t *dataTable) visibleCols() (int, int) {
	g := t.geo()
	from := t.st.columns.fixedWidth() + g.cx
	return t.st.columns.visible(from, from+g.x.viewport)
}

// scrollStart is where the column at display starts from the scrolling
// columns' start.
func (t *dataTable) scrollStart(display int) float64 {
	return t.st.columns.startOf(display) - t.st.columns.fixedWidth()
}

// report tells the app of the rows and columns in view, and of nearing the
// last row.
func (t *dataTable) report() {
	t.cx.Effect(func() {
		rs, re := t.visibleRows()
		cs, ce := t.visibleCols()
		total := t.rowCount()
		loading := t.p.Loading != nil && t.p.Loading()
		hasMore := t.p.HasMore != nil && t.p.HasMore()
		threshold := t.p.LoadMoreThreshold
		if threshold == 0 {
			threshold = 20
		}
		t.cx.Untrack(func() {
			st := t.st
			rowsChanged := !st.reported || st.reportedRows != [2]int{rs, re}
			colsChanged := !st.reported || st.reportedCols != [2]int{cs, ce}
			st.reported, st.reportedRows, st.reportedCols = true, [2]int{rs, re}, [2]int{cs, ce}
			load := hasMore && !loading && total > 0 && re+threshold >= total && st.loadMoreAt != total
			if load {
				st.loadMoreAt = total
			}
			if rowsChanged && t.p.OnVisibleRowsChange != nil {
				t.p.OnVisibleRowsChange(rs, re)
			}
			if colsChanged && t.p.OnVisibleColumnsChange != nil {
				t.p.OnVisibleColumnsChange(cs, ce)
			}
			if load && t.p.OnLoadMore != nil {
				t.p.OnLoadMore()
			}
		})
	})
}

// selection is the selection (tracked through the data revision).
func (t *dataTable) selection() tableSel {
	t.dataRev.Get()
	return t.st.sel
}

func (t *dataTable) colSelectable(def int) bool {
	return def < 0 || def >= len(t.st.defs) || !t.st.defs[def].NoSelect
}

func (t *dataTable) setSelection(next tableSel, report bool) {
	if t.st.sel == next {
		return
	}
	t.st.sel = next
	t.bumpData()
	if report && t.p.OnSelectionChange != nil {
		t.p.OnSelectionChange(next.public())
	}
}

func (t *dataTable) selectPublic(s Selection, report bool) {
	rows := core.Untracked(t.cx, t.rowCount)
	t.setSelection(clampSel(s.sel(), t.selectable(), rows, t.st.columns.len()), report)
	t.revealSelection()
}

func (t *dataTable) selectableColumns() []int {
	var out []int
	for _, def := range t.st.columns.order {
		if t.colSelectable(def) {
			out = append(out, def)
		}
	}
	return out
}

func (t *dataTable) untrackedGeo() tableGeo { return core.Untracked(t.cx, t.geo) }

// keyMove moves the selection by a key.
func (t *dataTable) keyMove(m tableMove) {
	g := t.untrackedGeo()
	nav := tableNav{sel: t.selectable(), rows: core.Untracked(t.cx, t.rowCount), columns: t.selectableColumns(),
		page: max(int(g.y.viewport/float64(g.rowHeight)), 1), wrap: !t.p.NoLoopSelection}
	if next, ok := nav.step(t.st.sel, m); ok {
		t.setSelection(next, true)
		t.revealSelection()
	}
}

// keys binds the table's keys while it has the keyboard.
func (t *dataTable) keys(cx *core.Scope) {
	moves := []struct {
		key    string
		m      tableMove
		repeat bool
	}{{"up", moveUp, true}, {"down", moveDown, true}, {"left", moveLeft, true}, {"right", moveRight, true},
		{"shift+tab", moveLeft, false}, {"tab", moveRight, false}, {"pageup", movePageUp, true},
		{"pagedown", movePageDown, true}, {"home", moveFirst, true}, {"end", moveLast, true}}
	for _, k := range moves {
		cx.OnKeyWith(k.key, core.KeyOptions{When: core.KeyNotEditing, Repeat: k.repeat}, func() { t.keyMove(k.m) })
	}
	// Escape clears the selection, and with nothing selected gives up the
	// keyboard.
	cx.OnKey("escape", func() {
		if t.st.sel.kind == selNone {
			t.focused.Set(false)
		} else {
			t.setSelection(tableSel{}, true)
		}
	})
	cx.OnKeyWith("mod+c", core.KeyOptions{When: core.KeyNotEditing}, t.copySelection)
}

func (t *dataTable) revealSelection() {
	if r, ok := t.st.sel.rowOf(); ok {
		t.revealRow(r)
	}
	if c, ok := t.st.sel.colOf(); ok {
		t.revealColumn(c)
	}
}

func (t *dataTable) revealRow(row int) {
	g := t.untrackedGeo()
	top := float64(row) * float64(g.rowHeight)
	if to := g.y.reveal(g.cy, top, top+float64(g.rowHeight)); to != g.cy {
		t.vscroll.ScrollTo(core.Point{Y: core.Points(g.y.scrollPoints(to))}, false)
	}
}

func (t *dataTable) revealColumn(def int) {
	g := t.untrackedGeo()
	c := t.st.columns
	if !c.hasDef(def) {
		return
	}
	display := c.displayOf(def)
	if display < c.fixed {
		return
	}
	start := c.start(display) - c.fixedWidth()
	if to := g.x.reveal(g.cx, start, start+float64(c.displayWidth(display))); to != g.cx {
		t.hscroll.ScrollTo(core.Point{X: core.Points(g.x.scrollPoints(to))}, false)
	}
}

func (t *dataTable) scrollToRow(row int, animated bool) {
	g := t.untrackedGeo()
	rows := core.Untracked(t.cx, t.rowCount)
	if rows == 0 {
		return
	}
	top := float64(min(max(row, 0), rows-1)) * float64(g.rowHeight)
	t.vscroll.ScrollTo(core.Point{Y: core.Points(g.y.scrollPoints(top))}, animated)
}

func (t *dataTable) scrollToColumn(key string, animated bool) {
	def := t.defOf(key)
	c := t.st.columns
	if !c.hasDef(def) || c.displayOf(def) < c.fixed {
		return
	}
	g := t.untrackedGeo()
	start := c.start(c.displayOf(def)) - c.fixedWidth()
	t.hscroll.ScrollTo(core.Point{X: core.Points(g.x.scrollPoints(start))}, animated)
}

// tableHit is what a press landed on: a row and a column (-1 for none),
// or a row number.
type tableHit struct {
	row, col int
	lead     bool
}

// hitAt is what is at p in the hit layer's scrolled content.
func (t *dataTable) hitAt(p core.Point) tableHit {
	g := t.untrackedGeo()
	vx := float64(p.X - t.scrollX.GetUntracked())
	vy := float64(p.Y - t.scrollY.GetUntracked())
	h := tableHit{row: -1, col: -1}
	if at := math.Floor((g.cy + vy) / float64(g.rowHeight)); at >= 0 && int(at) < core.Untracked(t.cx, t.rowCount) {
		h.row = int(at)
	}
	if vx < float64(g.lead) {
		h.lead = true
		return h
	}
	if d := t.st.columns.displayAt(vx-float64(g.lead), g.cx); d >= 0 {
		h.col = t.st.columns.def(d)
	}
	return h
}

// selectsOn reports whether a press with mods selects a row.
func (t *dataTable) selectsOn(mods core.Modifiers) bool {
	secondary := mods.Control
	if core.Untracked(t.cx, t.cx.Env().AppleKeys) {
		secondary = mods.Platform
	}
	return !t.p.NoSelectOnModifierPress || !(secondary || mods.Shift)
}

// press is a press of hit, as gpui-component's table takes it: a cell when
// cells select (the selected cell again, without row numbers, its row); a
// row when rows select, of a press of its number or of a row whose cells
// do not select.
func (t *dataTable) press(h tableHit, clicks int, mods core.Modifiers) {
	t.focused.Set(true)
	t.st.active = false
	if h.row < 0 {
		t.bumpData()
		return
	}
	a := t.selectable()
	col := -1
	if !h.lead {
		col = h.col
	}
	var next tableSel
	ok := false
	switch {
	case col >= 0 && a.cells && !t.colSelectable(col):
	case col >= 0 && a.cells:
		again := t.st.sel == tableSel{kind: selCell, row: h.row, col: col}
		if !t.rowHeaderShown() && a.rows && again && clicks != 2 && t.selectsOn(mods) {
			next, ok = tableSel{kind: selRow, row: h.row}, true
		} else {
			next, ok = tableSel{kind: selCell, row: h.row, col: col}, true
		}
	case a.rows && t.selectsOn(mods):
		next, ok = tableSel{kind: selRow, row: h.row}, true
	}
	if ok {
		t.setSelection(next, true)
	}
	t.bumpData()
	if t.p.OnRowPress != nil {
		t.p.OnRowPress(Press{Row: h.row, Column: col, Clicks: clicks, Modifiers: mods})
	}
	if clicks == 2 && t.p.OnDoublePress != nil {
		t.p.OnDoublePress(h.row, col)
	}
}

// contextMenu marks what hit names as the menu's and tells the app.
func (t *dataTable) contextMenu(h tableHit, at core.Point) {
	t.focused.Set(true)
	col := h.col
	if h.lead {
		col = -1
	}
	// Where rows select and cells do not, the row is the menu's.
	mark := col
	if h.row >= 0 && !core.Untracked(t.cx, t.cellSelect) {
		mark = -1
	}
	t.st.active, t.st.activeRow, t.st.activeCol = true, h.row, mark
	t.bumpData()
	if t.p.OnContextMenu != nil {
		t.p.OnContextMenu(ContextMenu{Row: h.row, Column: col, At: at})
	}
}

func (t *dataTable) hoverAt(p core.Point) {
	if row := t.hitAt(p).row; t.hover.GetUntracked() != row {
		t.hover.Set(row)
	}
}

// headerPress selects the column def when columns select (sorting is the
// sort button's).
func (t *dataTable) headerPress(def, clicks int) {
	t.focused.Set(true)
	if !t.p.NoColumnSelect && t.colSelectable(def) {
		t.setSelection(tableSel{kind: selColumn, col: def}, true)
	}
	if clicks == 2 && t.p.OnDoublePress != nil {
		t.p.OnDoublePress(-1, def)
	}
}

// sortNext asks for the next sort of column def.
func (t *dataTable) sortNext(def int) {
	if def < 0 || def >= len(t.st.defs) || t.p.OnSortChange == nil {
		return
	}
	key := t.st.defs[def].Key
	var current Sort
	if t.p.Sort != nil {
		current = core.Untracked(t.cx, t.p.Sort)
	}
	next := Sort{Key: key}
	if current.Key == key {
		if current.Descending {
			next = Sort{}
		} else {
			next.Descending = true
		}
	}
	t.p.OnSortChange(next)
}

func (t *dataTable) resizable(def int) bool {
	return !t.p.NoResizeColumns && def < len(t.st.defs) && !t.st.defs[def].NoResize
}

func (t *dataTable) movable(def int) bool {
	return !t.p.NoMoveColumns && def < len(t.st.defs) && !t.st.defs[def].NoMove
}

func (t *dataTable) resizeStart(def int) { t.resizing, t.resizeFrom = def, t.st.columns.width(def) }

func (t *dataTable) resizeMove(offset core.Points) {
	if t.resizing < 0 {
		return
	}
	def := t.resizing
	t.setWidth(def, t.resizeFrom+float32(offset), true)
	// The edge being dragged stays in view, the table scrolling as it
	// passes the viewport's end.
	c := t.st.columns
	display := c.displayOf(def)
	if display < c.fixed {
		return
	}
	g := t.untrackedGeo()
	end := c.start(display) - c.fixedWidth() + float64(c.displayWidth(display))
	if end > g.cx+g.x.viewport {
		if to := g.x.reveal(g.cx, end-1, end); to != g.cx {
			t.hscroll.ScrollTo(core.Point{X: core.Points(g.x.scrollPoints(to))}, false)
		}
	}
}

// resizeEnd ends a drag at offset; a press that did not move the handle,
// twice in a row, fits the column.
func (t *dataTable) resizeEnd(offset core.Points) {
	def := t.resizing
	t.resizing = -1
	if def < 0 {
		return
	}
	if math.Abs(float64(offset)) > moveSlop {
		t.handleTap = -1
		return
	}
	now := t.cx.Now()
	if t.handleTap == def && now.Sub(t.handleTapAt) < handleDoublePress {
		t.handleTap = -1
		t.fit(def)
		return
	}
	t.handleTap, t.handleTapAt = def, now
}

func (t *dataTable) setWidth(def int, w float32, report bool) float32 {
	if def < 0 || def >= len(t.st.defs) {
		return 0
	}
	if t.st.columns.setWidth(def, w) {
		t.bumpLayout()
		if report && t.p.OnColumnResize != nil {
			t.p.OnColumnResize(t.st.defs[def].Key, core.Points(t.st.columns.width(def)))
		}
	}
	return t.st.columns.width(def)
}

// fit sets column def as wide as its widest text and title need.
func (t *dataTable) fit(def int) float32 {
	if def < 0 || def >= len(t.st.defs) {
		return 0
	}
	font := t.font()
	rows := core.Untracked(t.cx, t.rowCount)
	var vs, ve int
	t.cx.Untrack(func() { vs, ve = t.visibleRows() })
	column := t.st.defs[def]
	widest := t.cx.MeasureText(column.title(), font).Width
	if column.Sortable {
		widest += sortWidth
	}
	t.cx.Untrack(func() {
		for row := max(vs-fitRows, 0); row < min(ve+fitRows, rows); row++ {
			if text := t.cellText(row, def); text != "" {
				widest = max(widest, t.cx.MeasureText(text, font).Width)
			}
		}
	})
	return t.setWidth(def, float32(math.Ceil(float64(widest+max(float32(column.FitPadding), 0)))), true)
}

func (t *dataTable) moveStart(display int) {
	t.drag.Set(tableDrag{from: display, to: display})
}

// moveTo has a title's drag at x in the table's viewport, travelled
// points from where it went down.
func (t *dataTable) moveTo(x, travelled float32) {
	d := t.drag.GetUntracked()
	if d.from < 0 {
		return
	}
	g := t.untrackedGeo()
	c := t.st.columns
	tx := float64(x - g.lead)
	var at int
	if tx < float64(g.fixed) {
		at = c.displayAt(math.Max(tx, 0), 0)
	} else {
		at = c.displayAt(tx, g.cx)
	}
	lo, hi := c.fixed, c.len()-1
	if d.from < c.fixed {
		lo, hi = 0, max(c.fixed-1, 0)
	}
	if at < 0 {
		if tx < 0 {
			at = lo
		} else {
			at = hi
		}
	}
	next := tableDrag{from: d.from, to: max(lo, min(at, hi)), moving: d.moving || math.Abs(float64(travelled)) > moveSlop}
	if next != d {
		t.drag.Set(next)
	}
}

func (t *dataTable) moveEnd(drop bool) {
	d := t.drag.GetUntracked()
	t.drag.Set(tableDrag{from: -1})
	if !drop || d.from < 0 || !d.moving || d.from == d.to {
		return
	}
	c := t.st.columns
	key := t.st.defs[c.def(d.from)].Key
	if !c.moveTo(d.from, d.to) {
		return
	}
	order := make([]string, c.len())
	for i, def := range c.order {
		order[i] = t.st.defs[def].Key
	}
	t.bumpLayout()
	if t.p.OnColumnMove != nil {
		t.p.OnColumnMove(ColumnMove{Key: key, From: d.from, To: d.to, Order: order})
	}
}

// dump is the titles and the texts of rows start to end (all for end < 0).
func (t *dataTable) dump(start, end int) Dump {
	rows := core.Untracked(t.cx, t.rowCount)
	if end < 0 || end > rows {
		end = rows
	}
	var out Dump
	t.cx.Untrack(func() {
		for _, def := range t.st.columns.order {
			out.Headers = append(out.Headers, t.title(def))
		}
		for row := max(start, 0); row < end; row++ {
			cells := make([]string, 0, len(t.st.columns.order))
			for _, def := range t.st.columns.order {
				cells = append(cells, t.cellText(row, def))
			}
			out.Rows = append(out.Rows, cells)
		}
	})
	return out
}

// selectionText is the selection's text: a row's cells with tabs, a
// column's with line breaks, a cell's.
func (t *dataTable) selectionText() (string, bool) {
	s := t.st.sel
	var b strings.Builder
	t.cx.Untrack(func() {
		switch s.kind {
		case selCell:
			b.WriteString(t.cellText(s.row, s.col))
		case selRow:
			for i, def := range t.st.columns.order {
				if i > 0 {
					b.WriteByte('\t')
				}
				b.WriteString(t.cellText(s.row, def))
			}
		case selColumn:
			for row := range min(t.rowCount(), copyRows) {
				if row > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(t.cellText(row, s.col))
			}
		}
	})
	return b.String(), s.kind != selNone
}

func (t *dataTable) copySelection() {
	if text, ok := t.selectionText(); ok {
		t.cx.SetClipboard(text)
	}
}
