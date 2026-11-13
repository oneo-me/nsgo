package core

import (
	"strconv"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// bigRows is a 300-point scroll view of 20 rows 50 points high, each a
// column of 30 views, and 40 views beside it, outside any scroll view.
func bigRows(t *testing.T, ref *Ref) *host {
	return newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		rows := make([]Node, 20)
		for i := range rows {
			cells := make([]Node, 30)
			for j := range cells {
				v := View{Style: Style{Height: Pt(1)}, TestID: "cell " + strconv.Itoa(i) + " " + strconv.Itoa(j)}
				if i == 19 && j == 29 {
					v.Ref = ref
				}
				cells[j] = v
			}
			rows[i] = View{Style: Style{Height: Pt(50), FlexShrink: 0}, TestID: "row " + strconv.Itoa(i), Children: cells}
		}
		beside := make([]Node, 40)
		for i := range beside {
			beside[i] = View{Style: Style{Width: Pt(1), Height: Pt(1)}, TestID: "beside " + strconv.Itoa(i)}
		}
		return View{Style: Style{FlexGrow: 1}, Children: []Node{
			View{Style: Style{Direction: Row}, Children: beside},
			Scroll{Style: Style{Height: Pt(299)}, Children: rows},
		}}
	})
}

func created(b *driver.MountBatch) int { return kinds(b)[driver.ItemCreate] }

func TestViewsOutOfSightAreMadeOverLaterFrames(t *testing.T) {
	h := bigRows(t, nil)
	first := created(h.commit())
	// The views beside it are made whatever the count; in the scroll view,
	// the rows that show (0 to 5, as it sits under a 1-point row) with
	// every view in them.
	for i := range 40 {
		if !h.find("beside " + strconv.Itoa(i)).created {
			t.Fatalf("view %d outside the scroll view waits", i)
		}
	}
	for i := range 6 {
		for j := range 30 {
			if n := h.find("cell " + strconv.Itoa(i) + " " + strconv.Itoa(j)); !n.created {
				t.Fatalf("cell %d %d, in sight, waits", i, j)
			}
		}
	}
	if h.find("cell 19 29").created || !h.tree.Pacing() {
		t.Fatal("views out of sight were all made at once")
	}
	if first >= 20*31+40 {
		t.Fatalf("the first commit made %d views", first)
	}
	total := first
	for h.tree.Pacing() {
		h.frame()
		n := created(h.commit())
		if n > paceViews {
			t.Fatalf("a later commit made %d views", n)
		}
		total += n
	}
	// The root, the row beside the scroll view, it and its content too.
	if want := 20*31 + 40 + 4; total != want {
		t.Fatalf("made %d views, want %d", total, want)
	}
	if !h.find("cell 19 29").created {
		t.Fatal("the last view waits")
	}
}

func TestACommandMakesTheViewItNames(t *testing.T) {
	ref := &Ref{}
	h := bigRows(t, ref)
	h.commit()
	if h.find("cell 19 29").created {
		t.Fatal("the last view is made at once")
	}
	ref.Focus()
	b := h.commit()
	if !h.find("cell 19 29").created || !h.find("row 19").created {
		t.Fatal("the focused view waits")
	}
	for _, it := range b.Items {
		if it.Kind == driver.ItemFocus && it.ID != h.find("cell 19 29").id {
			t.Fatalf("focus went to %d", it.ID)
		}
	}
}

func TestViewPacingTurnedOffMakesEveryView(t *testing.T) {
	h := bigRows(t, nil)
	h.tree.SetViewPacing(false)
	if n := created(h.commit()); n != 20*31+40+4 {
		t.Fatalf("made %d views", n)
	}
	h = bigRows(t, nil)
	h.commit()
	h.tree.SetViewPacing(false)
	h.commit()
	if !h.find("cell 19 29").created {
		t.Fatal("a waiting view waits with pacing off")
	}
}

func TestAScrollMakesWhatComesIntoSightAtOnce(t *testing.T) {
	h := bigRows(t, nil)
	h.commit()
	scroll := h.find("row 0").nativeParent()
	h.tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: scroll.id, Point: Point{Y: 700}})
	b := h.commit()
	for i := 14; i < 20; i++ {
		for j := range 30 {
			if !h.find("cell " + strconv.Itoa(i) + " " + strconv.Itoa(j)).created {
				t.Fatalf("cell %d %d, scrolled into sight, waits", i, j)
			}
		}
	}
	// The rows into sight with their views, and the budget of the others.
	if n := created(b); n > 6*31+paceViews {
		t.Fatalf("the scroll made %d views", n)
	}
	// Nearest first: the rows just above the viewport before those further.
	if h.tree.Pacing() && h.find("row 6").created && !h.find("row 13").created {
		t.Fatal("a row far from the viewport was made before a near one")
	}
	h.paced()
	if !h.find("cell 7 29").created {
		t.Fatal("a row out of sight waits for good")
	}
}
