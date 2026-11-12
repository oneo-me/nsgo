package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// parents applies a batch's inserts, removes and deletes to a model of
// each view's parent, failing on an item for a view the model lacks.
func parents(t *testing.T, model map[NodeID]NodeID, b *driver.MountBatch) {
	t.Helper()
	for _, it := range b.Items {
		switch it.Kind {
		case driver.ItemCreate:
			model[it.ID] = 0
		case driver.ItemInsert:
			if _, ok := model[it.ID]; !ok {
				t.Fatalf("insert of %d, which does not exist", it.ID)
			}
			model[it.ID] = it.Parent
		case driver.ItemRemove:
			model[it.ID] = 0
		case driver.ItemDelete:
			for id, p := range model {
				if p == it.ID {
					t.Fatalf("delete of %d while %d is in it", it.ID, id)
				}
			}
			delete(model, it.ID)
		}
	}
}

func TestHolderMovesItsContent(t *testing.T) {
	var where *Signal[int]
	var count *Signal[int]
	var held *Holder
	builds := 0
	h := newHost(t, Size{Width: 100, Height: 200}, func(cx *Scope) Node {
		where = NewSignal(cx, 0)
		held = NewHolder(cx, func(cx *Scope) Node {
			builds++
			count = NewSignal(cx, 0)
			return Text{TestID: "content", ContentFunc: func() string { return string(rune('0' + count.Get())) }}
		})
		side := func(i int) Node {
			return Show{When: func() bool { return where.Get() == i }, Then: func(cx *Scope) Node {
				return View{TestID: string(rune('a' + i)), Children: []Node{held.Place()}}
			}}
		}
		return View{Children: []Node{side(0), side(1)}}
	})
	model := map[NodeID]NodeID{}
	parents(t, model, h.commit())
	content := h.find("content")
	if model[content.id] != h.find("a").id {
		t.Fatal("content is not in the first place")
	}
	count.Set(3)

	// The place moves to a view after the one it leaves, which goes.
	where.Set(1)
	b := h.commit()
	parents(t, model, b)
	if builds != 1 || h.find("content") != content || count.GetUntracked() != 3 {
		t.Fatalf("content built %d times, kept %v", builds, h.find("content") == content)
	}
	if model[content.id] != h.find("b").id {
		t.Fatal("content did not move to the second place")
	}
	moved, placed := -1, -1
	for i, it := range b.Items {
		switch {
		case it.ID == content.id && it.Kind == driver.ItemRemove:
			t.Error("a move removed the content before inserting it")
		case it.ID == content.id && it.Kind == driver.ItemInsert:
			moved = i
		case it.ID == h.find("b").id && it.Kind == driver.ItemInsert:
			placed = i
		}
	}
	if moved < placed {
		t.Error("the content went into its new view before that view was in place")
	}

	// And back, to a view before it.
	where.Set(0)
	parents(t, model, h.commit())
	if model[content.id] != h.find("a").id || h.find("content") != content {
		t.Fatal("content did not move back")
	}

	// No place: it waits out of the tree, still built.
	where.Set(2)
	parents(t, model, h.commit())
	if p, ok := model[content.id]; !ok || p != 0 {
		t.Fatalf("content without a place: in %d, exists %v", p, ok)
	}
	where.Set(1)
	b = h.commit()
	parents(t, model, b)
	if builds != 1 || model[content.id] != h.find("b").id || kinds(b)[driver.ItemCreate] != 1 {
		t.Fatalf("placed again: built %d times, kinds %v", builds, kinds(b))
	}

	held.Dispose()
	parents(t, model, h.commit())
	if _, ok := model[content.id]; ok {
		t.Error("disposed content is still mounted")
	}
}

func TestHolderShowsInItsNewestPlace(t *testing.T) {
	var second *Signal[bool]
	var held *Holder
	h := newHost(t, Size{Width: 100, Height: 200}, func(cx *Scope) Node {
		second = NewSignal(cx, false)
		held = NewHolder(cx, func(cx *Scope) Node { return View{TestID: "content"} })
		return View{Children: []Node{
			View{TestID: "first", Children: []Node{held.Place()}},
			View{TestID: "second", Children: []Node{Show{When: second.Get, Then: func(cx *Scope) Node { return held.Place() }}}},
		}}
	})
	model := map[NodeID]NodeID{}
	parents(t, model, h.commit())
	content := h.find("content").id
	second.Set(true)
	b := h.commit()
	parents(t, model, b)
	if model[content] != h.find("second").id {
		t.Fatal("content is not in the newest place")
	}
	if k := kinds(b); k[driver.ItemRemove] != 0 || k[driver.ItemInsert] != 1 {
		t.Errorf("move kinds = %v, want the content's insert only", k)
	}
	second.Set(false)
	parents(t, model, h.commit())
	if model[content] != h.find("first").id {
		t.Fatal("content did not go back to the place before")
	}
}

func TestHolderHidesOverlaysOutOfTheTree(t *testing.T) {
	var shown *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 200}, func(cx *Scope) Node {
		shown = NewSignal(cx, true)
		held := NewHolder(cx, func(cx *Scope) Node {
			return Overlay{TestID: "popup", Children: []Node{View{Style: Style{Width: Pt(20), Height: Pt(20)}}}}
		})
		return Show{When: shown.Get, Then: func(cx *Scope) Node { return held.Place() }}
	})
	h.commit()
	if h.find("popup").frame.Size.Width != 20 {
		t.Fatal("overlay not laid out")
	}
	shown.Set(false)
	h.commit()
	if p := h.find("popup"); !p.hidden || p.frame != (Bounds{}) {
		t.Errorf("overlay of content out of the tree: hidden %v, frame %v", p.hidden, p.frame)
	}
}

func TestHolderMovesBetweenSiblings(t *testing.T) {
	var second *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 200}, func(cx *Scope) Node {
		second = NewSignal(cx, false)
		held := NewHolder(cx, func(cx *Scope) Node { return View{TestID: "content"} })
		return View{Children: []Node{
			View{TestID: "first", Children: []Node{held.Place()}},
			View{TestID: "second", Children: []Node{
				View{TestID: "x"},
				Show{When: second.Get, Then: func(cx *Scope) Node { return Group{held.Place(), View{TestID: "y"}} }},
				View{TestID: "z"},
			}},
		}}
	})
	order := map[NodeID][]NodeID{}
	apply := func(b *driver.MountBatch) {
		for _, it := range b.Items {
			if it.Kind != driver.ItemInsert && it.Kind != driver.ItemRemove && it.Kind != driver.ItemDelete {
				continue
			}
			for p, kids := range order {
				order[p] = slices.DeleteFunc(kids, func(id NodeID) bool { return id == it.ID })
			}
			if it.Kind == driver.ItemInsert {
				kids := order[it.Parent]
				at := slices.Index(kids, it.Before)
				if at < 0 {
					at = len(kids)
				}
				order[it.Parent] = slices.Insert(kids, at, it.ID)
			}
		}
	}
	apply(h.commit())
	second.Set(true)
	apply(h.commit())
	ids := func(names ...string) (out []NodeID) {
		for _, n := range names {
			out = append(out, h.find(n).id)
		}
		return out
	}
	if got, want := order[h.find("second").id], ids("x", "content", "y", "z"); !slices.Equal(got, want) {
		t.Errorf("second view's children %v, want %v", got, want)
	}
	if got := order[h.find("first").id]; len(got) != 0 {
		t.Errorf("first view keeps %v", got)
	}
}
