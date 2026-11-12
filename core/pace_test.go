package core

import (
	"slices"
	"strconv"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// pacedList is a list of 30-point rows in a 300-point viewport.
func pacedList(t *testing.T) (*host, *node) {
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		return List{Count: func() int { return 1000 }, EstimatedSize: 30, Style: Style{FlexGrow: 1}, TestID: "list",
			Row: func(cx *Scope, row ListRow) Node {
				return Text{ContentFunc: func() string { return strconv.Itoa(row.Index()) }, Style: Style{Height: Pt(30)}}
			}}
	})
	h.commit()
	return h, h.find("list")
}

// whole reports whether ln bound its whole window.
func whole(ln *listNode) bool {
	first, last, _, _ := ln.st.window()
	got := bound(ln)
	return len(got) == last-first+1 && got[0] == first && got[len(got)-1] == last
}

func bound(ln *listNode) []int {
	var rows []int
	for i := range ln.byIndex {
		rows = append(rows, i)
	}
	slices.Sort(rows)
	return rows
}

func TestAListBindsItsViewportFirstAndItsOverscanOnLaterFrames(t *testing.T) {
	h, list := pacedList(t)
	ln := list.list
	// The viewport, rows 0 to 9, and the first of the overscan below it.
	if got := bound(ln); len(got) != 10+paceCells || got[len(got)-1] != 9+paceCells {
		t.Fatalf("first commit bound %v", got)
	}
	if !h.tree.Pacing() {
		t.Fatal("the rest of the overscan waits for no frame")
	}
	h.paced()
	if !whole(ln) || h.tree.Pacing() {
		t.Fatalf("after its frames the list bound %v", bound(ln))
	}

	// A step binds the viewport and two steps ahead at once.
	h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: list.id, Point: Point{Y: 150}})
	h.commit()
	for i := 5; i <= 24; i++ {
		if _, ok := ln.byIndex[i]; !ok {
			t.Errorf("row %d, in the viewport or two steps ahead, is not bound", i)
		}
	}
	h.paced()

	// A jump binds its viewport, the overscan ahead of it first.
	h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: list.id, Point: Point{Y: 9000}})
	h.commit()
	got := bound(ln)
	want := []int{300, 301, 302, 303, 304, 305, 306, 307, 308, 309, 310, 311, 312, 313}
	if !slices.Equal(got, want) {
		t.Errorf("after a jump the list bound %v, want the viewport and the rows below it %v", got, want)
	}
	h.paced()
	if !whole(ln) {
		t.Errorf("after its frames the list bound %v", bound(ln))
	}
}
