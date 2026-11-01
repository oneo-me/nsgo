package base_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

func tableColumns(n int) []base.DataTableColumn {
	cols := make([]base.DataTableColumn, n)
	for i := range cols {
		cols[i] = base.DataTableColumn{Key: fmt.Sprint("c", i), Title: fmt.Sprint("Col ", i), Width: 100}
	}
	cols[0].Fixed = true
	cols[1].Sortable = true
	return cols
}

func TestDataTableMakesOnlyTheCellsInView(t *testing.T) {
	var (
		sel     []base.DataTableSelection
		sorts   []base.DataTableSort
		rows    [2]int
		resized []string
		ref     base.DataTableRef
	)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.DataTable{
			TestID:              "table",
			Label:               "Quotes",
			Ref:                 &ref,
			Columns:             func() []base.DataTableColumn { return tableColumns(60) },
			RowCount:            func() int { return 1_000_000 },
			Text:                func(row, col int) string { return fmt.Sprintf("R%dC%d", row, col) },
			Style:               core.Style{Width: core.Pt(400), Height: core.Pt(332)},
			Styles:              base.DataTableStyles{RowSelected: core.Style{Background: core.RGB(0x0000ff)}},
			OnSelectionChange:   func(s base.DataTableSelection) { sel = append(sel, s) },
			OnSortChange:        func(s base.DataTableSort) { sorts = append(sorts, s) },
			OnVisibleRowsChange: func(s, e int) { rows = [2]int{s, e} },
			OnColumnResize:      func(key string, w core.Points) { resized = append(resized, fmt.Sprint(key, " ", w)) },
		}
	}, nsgotest.Size(400, 400))
	if v := app.Find("id:table"); v.Props.Role != core.RoleGrid || v.Props.Label != "Quotes" {
		t.Fatalf("table role %v label %q, want a grid", v.Props.Role, v.Props.Label)
	}
	cells := len(app.Query("role:cell"))
	if cells == 0 || cells > 400 {
		t.Fatalf("%d cells made, want the window's only", cells)
	}
	// 300 points under a 32-point header: rows 0 to 9 in view.
	if rows != [2]int{0, 10} {
		t.Errorf("visible rows %v, want 0 to 10", rows)
	}
	app.Find("text:R0C0")
	app.Press("text:R3C2")
	if len(sel) != 1 || sel[0] != (base.DataTableSelection{Row: 3, Column: -1}) {
		t.Fatalf("selection %v, want row 3", sel)
	}
	app.Key("down")
	if sel[len(sel)-1].Row != 4 {
		t.Errorf("down selected %v, want row 4", sel[len(sel)-1])
	}
	// Far down a million rows: the rows there show, scaled.
	ref.ScrollToRow(777_777, false)
	app.Settle()
	app.Find("text:R777777C0")
	if rows[0] != 777_777 {
		t.Errorf("visible rows %v after scrolling to 777777", rows)
	}
	app.Press("id:sort:c1")
	app.Press("id:sort:c1")
	if !slices.Equal(sorts, []base.DataTableSort{{Key: "c1"}, {Key: "c1"}}) {
		t.Errorf("sorts %v: the app keeps the sort, so each press asks for ascending", sorts)
	}
	app.Drag("id:resize:c1", core.Point{X: 30}, core.Point{X: 60})
	if len(resized) == 0 || resized[len(resized)-1] != "c1 160" {
		t.Errorf("resized %v, want c1 at 160", resized)
	}
	if d := ref.Dump(0, 2); len(d.Rows) != 2 || d.Headers[0] != "Col 0" || d.Rows[1][1] != "R1C1" {
		t.Errorf("dump %+v", d)
	}
}

func TestDataTableSelectsCellsMovesColumnsAndCopies(t *testing.T) {
	var (
		moves []base.DataTableColumnMove
		menus []base.DataTableContextMenu
		sel   base.DataTableSelection
	)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.DataTable{
			TestID:     "table",
			CellSelect: func() bool { return true },
			Columns: func() []base.DataTableColumn {
				return []base.DataTableColumn{{Key: "a", Title: "A"}, {Key: "b", Title: "B"}, {Key: "c", Title: "C"}}
			},
			Groups: func() [][]base.DataTableColumnGroup {
				return [][]base.DataTableColumnGroup{{{Label: "Both", Span: 2}, {Label: "One"}}}
			},
			RowCount:          func() int { return 5 },
			Text:              func(row, col int) string { return fmt.Sprintf("%c%d", 'a'+col, row) },
			Style:             core.Style{Width: core.Pt(500), Height: core.Pt(300)},
			OnSelectionChange: func(s base.DataTableSelection) { sel = s },
			OnColumnMove:      func(m base.DataTableColumnMove) { moves = append(moves, m) },
			OnContextMenu:     func(m base.DataTableContextMenu) { menus = append(menus, m) },
		}
	}, nsgotest.Size(500, 300))
	app.Find("text:Both")
	// Row numbers show with cell selection.
	app.Find("text:5")
	app.Press("text:b2")
	if sel != (base.DataTableSelection{Row: 2, Column: 1}) {
		t.Fatalf("selection %v, want the cell b2", sel)
	}
	app.Key("right")
	app.Key("mod+c")
	if app.Clipboard != "c2" {
		t.Errorf("copied %q, want c2", app.Clipboard)
	}
	app.Drag("text:A", core.Point{X: 40}, core.Point{X: 160})
	if len(moves) != 1 || moves[0].Key != "a" || !slices.Equal(moves[0].Order, []string{"b", "a", "c"}) {
		t.Errorf("moves %+v, want a after b", moves)
	}
	app.RightClick("text:c4")
	if len(menus) != 1 || menus[0].Row != 4 || menus[0].Column != 2 {
		t.Errorf("menus %+v, want c4's", menus)
	}
}

// TestDataTableReadsTheCellsNearTheView scrolls and ticks a table whose
// texts change: the rows made are those in view and half a viewport
// around, so a scroll reads the rows coming near the view and a tick the
// cells near it.
func TestDataTableReadsTheCellsNearTheView(t *testing.T) {
	var (
		tick  *core.Signal[int]
		reads int
		rows  [2]int
	)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		tick = core.NewSignal(cx, 0)
		return base.DataTable{
			TestID:   "table",
			Columns:  func() []base.DataTableColumn { return tableColumns(60) },
			RowCount: func() int { return 1_000_000 },
			Text: func(row, col int) string {
				reads++
				return fmt.Sprintf("R%dC%d·%d", row, col, tick.Get())
			},
			Style:               core.Style{Width: core.Pt(400), Height: core.Pt(332)},
			OnVisibleRowsChange: func(s, e int) { rows = [2]int{s, e} },
		}
	}, nsgotest.Size(400, 400))
	// 10 rows of 4 or 5 columns in view, one more each way, and 5 rows
	// made each way. A million rows scale the scroll view's offsets: 6
	// points scroll a row or two.
	app.Scroll("id:data-table-scroll", 60)
	from := rows[0]
	reads = 0
	app.Scroll("id:data-table-scroll", 6)
	if reads == 0 || reads > 3*7 {
		t.Errorf("a scroll of a row or two read %d texts, want the rows coming near the view's", reads)
	}
	reads = 0
	tick.Set(1)
	app.Settle()
	if reads == 0 || reads > 22*7 {
		t.Errorf("a tick read %d texts, want those near the view", reads)
	}
	if n := len(app.Query(fmt.Sprintf("text:R%dC1·1", from+40))); n != 0 {
		t.Errorf("row %d, past the rows made, shows %d texts", from+40, n)
	}
	app.Find(fmt.Sprintf("text:R%dC1·1", rows[0]+1))
	app.Scroll("id:data-table-scroll", 3000)
	if rows[0] < 10 {
		t.Fatalf("visible rows %v after scrolling", rows)
	}
	app.Find(fmt.Sprintf("text:R%dC1·1", rows[0]+1))
}

// TestDataTableScrollsByDrag scrolls the table's own scroll view, under
// the body, which takes no pointer.
func TestDataTableScrollsByDrag(t *testing.T) {
	var ref base.DataTableRef
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return base.DataTable{
			TestID:   "table",
			Ref:      &ref,
			Columns:  func() []base.DataTableColumn { return tableColumns(20) },
			RowCount: func() int { return 10_000 },
			Text:     func(row, col int) string { return fmt.Sprintf("R%dC%d", row, col) },
			Style:    core.Style{Width: core.Pt(400), Height: core.Pt(332)},
		}
	}, nsgotest.Size(400, 400), nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	if got := app.Find("id:data-table-vscrollbar").Frame.Size.Width; got != 12 {
		t.Fatalf("the vertical bar band is %v wide, want 12", got)
	}
	from, _ := ref.VisibleRows()
	app.Scroll("id:data-table-scroll", 120)
	app.Settle()
	if to, _ := ref.VisibleRows(); to <= from {
		t.Fatalf("visible rows %v after scrolling, want past %v", to, from)
	}
}
