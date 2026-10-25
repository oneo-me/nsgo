package core

import (
	"strconv"
	"testing"
)

// A first commit sends every view, so Tree.items grows to the whole tree's
// worth of items. A steady commit sends a handful. Keeping the peak pins
// megabytes for a long list, so the buffer has to give it back.
func TestCommitBufferDropsTheFirstCommitPeak(t *testing.T) {
	var bump *Signal[int]
	h := newHost(t, Size{Width: 560, Height: 1000}, func(cx *Scope) Node {
		bump = NewSignal(cx, 0)
		rows := make([]Node, 0, 2000)
		for range 2000 {
			rows = append(rows, View{Style: Style{Height: Pt(4)}})
		}
		return View{Children: []Node{
			Text{ContentFunc: func() string { return strconv.Itoa(bump.Get()) }, TestID: "n"},
			View{Children: rows},
		}}
	})
	h.commit()
	peak := cap(h.tree.items)
	if peak < 4096 {
		t.Fatalf("the first commit queued %d items; this test needs a peak over the 4096 floor", peak)
	}
	bump.Update(func(n int) int { return n + 1 })
	h.commit()
	if got := cap(h.tree.items); got >= peak {
		t.Fatalf("the commit buffer kept its peak: cap %d -> %d", peak, got)
	}
}

// A StyleFunc runs on every render effect. Its result must update one
// allocation per node rather than allocate a Style (976 B) each time, and
// a node without one must not hold a Style at all.
func TestStyleFuncKeepsOneStyle(t *testing.T) {
	var bump *Signal[int]
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		bump = NewSignal(cx, 0)
		return View{Children: []Node{
			Text{TestID: "n", Content: "x", StyleFunc: func() Style {
				return Style{FontSize: Points(12 + bump.Get())}
			}},
			Text{TestID: "plain", Content: "y"},
		}}
	})
	h.commit()
	n := h.find("n")
	if n.dyn == nil {
		t.Fatal("the StyleFunc's style is not kept")
	}
	first := n.dyn
	bump.Update(func(v int) int { return v + 1 })
	h.commit()
	if n.dyn != first {
		t.Fatal("the render effect allocated a new style instead of updating the one it keeps")
	}
	if got := n.dyn.FontSize; got != 13 {
		t.Fatalf("font size = %v, want 13", got)
	}
	if plain := h.find("plain"); plain.dyn != nil {
		t.Fatal("a node without a StyleFunc holds a style")
	}
}
