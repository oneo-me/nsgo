package driver

import (
	"slices"
	"testing"
)

// ltrLine is a line of 10-point characters from start to end, at y.
func ltrLine(start, end int, y Points) CaretLine {
	l := CaretLine{Start: start, End: end, Box: Bounds{Origin: Point{Y: y}, Size: Size{Width: Points(end-start) * 10, Height: 20}}}
	for i := start; i < end; i++ {
		x := Points(i-start) * 10
		l.Clusters = append(l.Clusters, CaretCluster{Start: i, End: i + 1, Left: x, Right: x + 10})
	}
	return l
}

func rangeBox(x, y, w Points) Bounds {
	return Bounds{Origin: Point{X: x, Y: y}, Size: Size{Width: w, Height: 20}}
}

func TestCaretLayoutsHitTestByLineAndAffinity(t *testing.T) {
	// "abcd" wrapped after its 4th character, then "efgh".
	cl := CaretLayout{ltrLine(0, 4, 0), ltrLine(4, 8, 20)}
	if c := cl.HitTest(Point{X: 70, Y: 5}); c != (Caret{4, Upstream}) {
		t.Fatalf("past the wrapped line's end: %+v, want its end upstream", c)
	}
	if r, _ := cl.CaretRect(Caret{4, Upstream}); r != rangeBox(40, 0, 0) {
		t.Fatalf("upstream caret %v, want the first line's end", r)
	}
	if r, _ := cl.CaretRect(Caret{4, Downstream}); r != rangeBox(0, 20, 0) {
		t.Fatalf("downstream caret %v, want the second line's start", r)
	}
	if c := cl.HitTest(Point{X: 16, Y: 90}); c != (Caret{Offset: 6}) {
		t.Fatalf("below the text: %+v, want the nearest edge on the last line", c)
	}
	got := cl.RangeRects(2, 6, nil)
	if want := []Bounds{rangeBox(20, 0, 20), rangeBox(0, 20, 20)}; !slices.Equal(got, want) {
		t.Fatalf("range boxes %v, want %v", got, want)
	}
	if got := cl.RangeRects(3, 3, nil); len(got) != 0 {
		t.Fatalf("an empty range covers %v", got)
	}
	if _, ok := (CaretLayout{}).CaretRect(Caret{}); ok {
		t.Fatal("a layout without lines has a caret")
	}
}

func TestCaretLayoutsSplitRangesByDirection(t *testing.T) {
	// "ab" then "CD" right to left: D at 20..30, C at 30..40.
	l := CaretLine{Start: 0, End: 4, Box: rangeBox(0, 0, 40), Clusters: []CaretCluster{
		{Start: 0, End: 1, Left: 0, Right: 10}, {Start: 1, End: 2, Left: 10, Right: 20},
		{Start: 2, End: 3, Left: 30, Right: 40, RTL: true}, {Start: 3, End: 4, Left: 20, Right: 30, RTL: true},
	}}
	cl := CaretLayout{l}
	if got, want := cl.RangeRects(1, 3, nil), []Bounds{rangeBox(10, 0, 10), rangeBox(30, 0, 10)}; !slices.Equal(got, want) {
		t.Fatalf("a range across the change of direction covers %v, want %v", got, want)
	}
	if c := cl.HitTest(Point{X: 38, Y: 5}); c.Offset != 2 {
		t.Fatalf("near C's start edge: %+v, want 2", c)
	}
	if r, _ := cl.CaretRect(Caret{Offset: 2}); r.Origin.X != 40 {
		t.Fatalf("downstream caret before C at %v, want its right edge", r.Origin.X)
	}
	if r, _ := cl.CaretRect(Caret{2, Upstream}); r.Origin.X != 20 {
		t.Fatalf("upstream caret after b at %v, want b's right edge", r.Origin.X)
	}
}
