package datatable

import (
	"github.com/oneo-me/nsgo/base/internal/window"
	"math"
	"slices"
	"testing"
)

func TestTableAxisScalesHugeContent(t *testing.T) {
	small := tableAxis{10_000, 500}
	if small.scaled() || small.contentOffset(1234) != 1234 || small.scrollPoints(1234.5) != 1234.5 {
		t.Fatal("small content is scaled")
	}
	huge := tableAxis{240_000_000, 600}
	if !huge.scaled() || huge.extent() != maxExtent {
		t.Fatal("huge content is not scaled")
	}
	if end := huge.contentOffset(maxExtent - 600); math.Abs(end-(240_000_000-600)) > 1 {
		t.Fatalf("the scroll view's end is %v, want the content's", end)
	}
	// A million rows of 32 points: scroll points never stop short of a row.
	rows := tableAxis{32_000_000, 540}
	for _, row := range []int{1, 7, 499_999, 500_000, 777_777, 999_983} {
		top := float64(row) * 32
		at := rows.contentOffset(float64(rows.scrollPoints(top)))
		if at < top-1e-6 || at >= top+32 {
			t.Errorf("row %d: scrolled to %v", row, at)
		}
	}
	a := tableAxis{1000, 100}
	for _, c := range [][4]float64{{50, 60, 90, 50}, {50, 20, 40, 20}, {50, 140, 170, 70}, {50, 60, 400, 60}} {
		if got := a.reveal(c[0], c[1], c[2]); got != c[3] {
			t.Errorf("reveal(%v) = %v, want %v", c[:3], got, c[3])
		}
	}
}

func spec(w float32, fixed bool) columnSpec {
	return columnSpec{width: w, min: 20, max: 400, fixed: fixed}
}

func TestTableColumns(t *testing.T) {
	c := newTableColumns([]columnSpec{spec(100, false), spec(60, true), spec(80, false), spec(120, false)})
	if !slices.Equal(c.order, []int{1, 0, 2, 3}) || c.fixedWidth() != 60 || c.total() != 360 || c.start(2) != 160 {
		t.Fatalf("order %v, fixed %v, total %v: fixed columns lead", c.order, c.fixedWidth(), c.total())
	}
	for _, x := range [][3]float64{{30, 0, 0}, {30, 90, 0}, {70, 0, 1}, {100, 90, 2}, {400, 0, -1}} {
		if got := c.displayAt(x[0], x[1]); got != int(x[2]) {
			t.Errorf("displayAt(%v, %v) = %d, want %v", x[0], x[1], got, x[2])
		}
	}
	for _, v := range [][4]float64{{60, 150, 1, 2}, {100, 200, 1, 3}, {0, 1000, 1, 4}, {500, 600, 4, 4}} {
		if a, b := c.visible(v[0], v[1]); a != int(v[2]) || b != int(v[3]) {
			t.Errorf("visible(%v, %v) = %d..%d", v[0], v[1], a, b)
		}
	}
	if !c.setWidth(0, 1000) || c.width(0) != 400 || c.start(2) != 460 || c.setWidth(0, 400) {
		t.Errorf("resizing clamps and moves the columns after it")
	}
	m := newTableColumns([]columnSpec{spec(10, true), spec(10, true), spec(30, false), spec(40, false), spec(50, false)})
	if m.moveTo(0, 3) || !m.moveTo(0, 1) || !m.moveTo(2, 4) || !slices.Equal(m.order, []int{1, 0, 3, 4, 2}) {
		t.Errorf("columns move within their zone: %v", m.order)
	}
	c.moveTo(1, 3)
	before := slices.Clone(c.order)
	next := c.updated([]columnSpec{spec(100, false), spec(60, true), spec(80, false), spec(120, false)}, func(d int) int { return d })
	if !slices.Equal(next.order, before) {
		t.Errorf("an update lost the order the user moved: %v, want %v", next.order, before)
	}
	big := newTableColumns(slices.Repeat([]columnSpec{spec(100, false)}, 1_000_000))
	if a, b := big.visible(55_000_050, 55_000_350); a != 550_000 || b != 550_004 {
		t.Errorf("a million columns: visible %d..%d", a, b)
	}
}

func TestTableNav(t *testing.T) {
	rows := tableNav{sel: selectable{rows: true}, rows: 10, columns: []int{2, 0, 1}, page: 4}
	row := func(r int) tableSel { return tableSel{kind: selRow, row: r} }
	check := func(n tableNav, from tableSel, m tableMove, want tableSel) {
		t.Helper()
		if got, _ := n.step(from, m); got != want {
			t.Errorf("step(%v, %v) = %v, want %v", from, m, got, want)
		}
	}
	check(rows, tableSel{}, moveDown, row(0))
	check(rows, row(3), moveUp, row(2))
	check(rows, row(0), moveUp, row(0))
	check(rows, row(3), movePageDown, row(7))
	check(rows, row(8), movePageDown, row(9))
	check(rows, row(5), moveLast, row(9))
	rows.wrap = true
	check(rows, row(0), moveUp, row(9))
	cols := tableNav{sel: selectable{columns: true}, rows: 10, columns: []int{2, 0, 1}}
	check(cols, tableSel{}, moveRight, tableSel{kind: selColumn, col: 2})
	check(cols, tableSel{kind: selColumn, col: 2}, moveRight, tableSel{kind: selColumn, col: 0})
	cells := tableNav{sel: selectable{cells: true}, rows: 10, columns: []int{2, 0, 1}}
	check(cells, tableSel{}, moveDown, tableSel{kind: selCell, row: 0, col: 2})
	check(cells, tableSel{kind: selCell, row: 3, col: 0}, moveRight, tableSel{kind: selCell, row: 3, col: 1})
	check(cells, tableSel{kind: selCell, row: 3, col: 0}, moveDown, tableSel{kind: selCell, row: 4, col: 0})
}

func TestRowWindowStaysWhileItCovers(t *testing.T) {
	w := window.New(4, 16)
	for _, c := range [][5]int{{100, 140, 1000, 80, 200}, {110, 150, 1000, 80, 200}, {141, 181, 1000, 121, 241},
		{100, 140, 1000, 40, 160}, {0, 40, 1000, 0, 120}, {960, 1000, 1000, 880, 1000}} {
		if a, b := w.Span(c[0], c[1], c[2]); a != c[3] || b != c[4] {
			t.Errorf("rows(%d..%d) = %d..%d, want %d..%d", c[0], c[1], a, b, c[3], c[4])
		}
	}
	// Slots: a window moved by k items gives k slots other items.
	if slotItem(0, 4, 2, 6) != 4 || slotItem(2, 4, 2, 6) != 2 || slotItem(1, 4, 2, 5) != -1 {
		t.Error("slots do not keep their items")
	}
}
