package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// ids returns the test IDs of the native children n last sent, in order.
func (h *host) ids(n *node) []string {
	var out []string
	for _, c := range n.sent.children {
		out = append(out, c.props.TestID)
	}
	return out
}

// has reports whether a node with testID is in the tree's table.
func (h *host) has(testID string) bool {
	for _, n := range h.tree.nodes {
		if n != nil && !n.deleted && n.props.TestID == testID {
			return true
		}
	}
	return false
}

func TestElementsBuiltOneChangeAtATime(t *testing.T) {
	var box, a, b *Element
	var branch *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		root := NewElement(cx, nil)
		box = NewElement(cx, View{TestID: "box"})
		a = NewElement(cx, View{TestID: "a", Style: Style{Height: Pt(10)}})
		branch = cx.Child()
		branch.Run(func() { b = NewElement(cx, Text{TestID: "b", Content: "hi"}) })
		root.Insert(box, nil)
		box.Insert(a, nil)
		box.Insert(b, a)
		return root
	})
	h.commit()
	if got := h.ids(h.find("box")); !slices.Equal(got, []string{"b", "a"}) {
		t.Fatalf("children %q, want b before a", got)
	}
	// Moved and set again, the text keeps its view.
	id := h.find("b").id
	box.Insert(b, nil)
	b.Set(Text{TestID: "b", Content: "bye"})
	batch := h.commit()
	if got := h.ids(h.find("box")); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("children %q after the move, want a before b", got)
	}
	if n := h.find("b"); n.id != id || n.text.content != "bye" {
		t.Errorf("the text is view %d with %q, want view %d with bye", n.id, n.text.content, id)
	}
	for _, it := range batch.Items {
		if it.Kind == driver.ItemCreate || it.Kind == driver.ItemDelete {
			t.Errorf("the move sent %v", it.Kind)
		}
	}
	// Taken out, it stays until its owner goes.
	box.Remove(b)
	h.commit()
	if got := h.ids(h.find("box")); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("children %q after the removal, want a", got)
	}
	if !h.has("b") {
		t.Fatal("a removed element is gone before its owner")
	}
	box.Insert(b, nil)
	h.commit()
	branch.Dispose()
	h.commit()
	if h.has("b") || !slices.Equal(h.ids(h.find("box")), []string{"a"}) {
		t.Errorf("the element outlived its owner: children %q", h.ids(h.find("box")))
	}
}

func TestAnElementMountedByGoLeavesWithItsRegion(t *testing.T) {
	var shown *Signal[bool]
	var el *Element
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		shown = NewSignal(cx, true)
		el = NewElement(cx, View{TestID: "el"})
		return View{TestID: "box", Children: []Node{Show{When: shown.Get, Then: func(*Scope) Node { return el }}}}
	})
	h.commit()
	id := h.find("el").id
	shown.Set(false)
	h.commit()
	if got := h.ids(h.find("box")); len(got) != 0 || !h.has("el") {
		t.Fatalf("hidden, the box shows %q and the element is kept: %v", got, h.has("el"))
	}
	shown.Set(true)
	h.commit()
	if n := h.find("el"); n.id != id {
		t.Errorf("shown again, the element is not the same view")
	}
}
