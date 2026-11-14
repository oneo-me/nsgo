package core

import "testing"

func TestUpdateHighlightsOutlineWhatACommitChanged(t *testing.T) {
	var label *Signal[string]
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		label = NewSignal(cx, "a")
		return Group{
			View{Style: Style{Padding: All(10), Gap: 4}, Children: []Node{
				Text{ContentFunc: label.Get, TestID: "label", Style: Style{Height: Pt(20)}},
				Text{Content: "static", TestID: "static", Style: Style{Height: Pt(20)}},
			}},
			UpdateHighlights{},
		}
	})
	boxes := func() []highlightBox { return h.tree.highlights.boxes.GetUntracked() }
	settle := func() {
		for range 4 {
			h.commit()
			h.drain()
		}
	}
	// The first commit made every view: they show, then go.
	settle()
	if len(boxes()) == 0 {
		t.Fatal("the first commit's views were not outlined")
	}
	h.clock.Advance(highlightFor)
	settle()
	if n := len(boxes()); n != 0 {
		t.Fatalf("%d outlines outlived highlightFor", n)
	}

	label.Set("b")
	settle()
	want := h.tree.surfaceBounds(h.find("label"))
	if bs := boxes(); len(bs) != 1 || bs[0].bounds != want {
		t.Fatalf("outlines = %+v, want one at the label's %+v", bs, want)
	}
	// Drawing and removing outlines are commits of the highlights' own views, which they leave out: nothing outlines again.
	h.clock.Advance(highlightFor)
	settle()
	h.clock.Advance(highlightFor)
	settle()
	if n := len(boxes()); n != 0 {
		t.Fatalf("the highlights outlined their own views: %d", n)
	}
	h.mu.Lock()
	pending := len(h.posts)
	h.mu.Unlock()
	if pending != 0 {
		t.Fatalf("%d posts after the highlights settled", pending)
	}
}

func TestUpdateHighlightsLeaveWithTheirOwner(t *testing.T) {
	var shown *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		shown = NewSignal(cx, true)
		return Show{When: shown.Get, Then: func(*Scope) Node { return UpdateHighlights{} }}
	})
	h.commit()
	if h.tree.highlights == nil {
		t.Fatal("the highlights did not mount")
	}
	shown.Set(false)
	h.commit()
	if h.tree.highlights != nil {
		t.Fatal("the tree kept highlights that went")
	}
}
