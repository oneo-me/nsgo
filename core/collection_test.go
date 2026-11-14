package core

import (
	"slices"
	"strconv"
	"testing"
)

type task struct {
	id   int
	text string
	done bool
}

func taskID(t task) int { return t.id }

func TestCollectionReadersFollowTheirOwnItem(t *testing.T) {
	cx := NewRoot(RootOptions{})
	c := NewCollection(cx, taskID, []task{{1, "a", false}, {2, "b", false}, {3, "c", false}})
	runs := map[int]int{}
	for _, id := range []int{1, 2, 3} {
		cx.Child().renderEffect(func() { c.Get(id); runs[id]++ })
	}
	orderRuns := 0
	cx.renderEffect(func() { c.Len(); orderRuns++ })

	c.Update(2, func(t task) task { t.done = true; return t })
	if runs[1] != 1 || runs[2] != 2 || runs[3] != 1 || orderRuns != 1 {
		t.Fatalf("runs = %v, order runs = %d", runs, orderRuns)
	}
	// An equal item changes nothing.
	c.Set(task{2, "b", true})
	if runs[2] != 2 {
		t.Fatalf("an equal item ran its reader: %v", runs)
	}
	if v, ok := c.Get(2); !ok || !v.done {
		t.Fatalf("item 2 = %+v %v", v, ok)
	}

	c.Set(task{4, "d", false})
	if orderRuns != 2 || !slices.Equal(c.Keys(), []int{1, 2, 3, 4}) || runs[1] != 1 {
		t.Fatalf("keys = %v, order runs = %d, runs = %v", c.Keys(), orderRuns, runs)
	}
	if !c.Delete(1) || c.Delete(1) {
		t.Fatal("Delete reported wrong")
	}
	if runs[1] != 2 || orderRuns != 3 {
		t.Fatalf("a deleted item's reader did not run: %v", runs)
	}
	if _, ok := c.Get(1); ok {
		t.Fatal("a deleted item is still there")
	}
	if c.Update(1, func(t task) task { return t }) {
		t.Fatal("Update of a missing item reported true")
	}
}

func TestCollectionMissingItemFollowsTheOrder(t *testing.T) {
	cx := NewRoot(RootOptions{})
	c := NewCollection(cx, taskID, nil)
	var seen []bool
	cx.renderEffect(func() { _, ok := c.Get(7); seen = append(seen, ok) })
	c.Set(task{id: 7})
	if !slices.Equal(seen, []bool{false, true}) {
		t.Fatalf("seen = %v", seen)
	}
}

func TestCollectionReplaceNotifiesOnlyWhatChanged(t *testing.T) {
	cx := NewRoot(RootOptions{})
	c := NewCollection(cx, taskID, []task{{1, "a", false}, {2, "b", false}, {3, "c", false}})
	keys := c.Keys()
	runs := map[int]int{}
	for _, id := range []int{1, 2, 3} {
		cx.Child().renderEffect(func() { c.Get(id); runs[id]++ })
	}
	orderRuns := 0
	cx.renderEffect(func() { c.Keys(); orderRuns++ })

	// Item 2 changes; the order holds.
	c.Replace([]task{{1, "a", false}, {2, "B", false}, {3, "c", false}})
	if runs[1] != 1 || runs[2] != 2 || runs[3] != 1 || orderRuns != 1 {
		t.Fatalf("runs = %v, order runs = %d", runs, orderRuns)
	}
	// A move, a removal and an addition.
	c.Replace([]task{{3, "c", false}, {1, "a", false}, {5, "e", false}})
	if !slices.Equal(c.Keys(), []int{3, 1, 5}) || orderRuns != 2 || runs[2] != 3 || runs[1] != 1 {
		t.Fatalf("keys = %v, runs = %v, order runs = %d", c.Keys(), runs, orderRuns)
	}
	if got := c.Items(); len(got) != 3 || got[2].text != "e" {
		t.Fatalf("items = %v", got)
	}
	// What Keys returned before stays as it was.
	if !slices.Equal(keys, []int{1, 2, 3}) {
		t.Fatalf("an old Keys slice changed: %v", keys)
	}
}

func TestCollectionFuncWithoutEqualNotifiesEverySet(t *testing.T) {
	cx := NewRoot(RootOptions{})
	type tagged struct {
		id   int
		tags []string
	}
	c := NewCollectionFunc(cx, func(t tagged) int { return t.id }, nil, []tagged{{id: 1}})
	runs := 0
	cx.renderEffect(func() { c.Get(1); runs++ })
	c.Set(tagged{id: 1})
	c.Replace([]tagged{{id: 1}})
	if runs != 3 {
		t.Fatalf("runs = %d", runs)
	}
}

func TestCollectionPanicsOnDuplicateAndChangedKeys(t *testing.T) {
	cx := NewRoot(RootOptions{})
	panics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s did not panic", name)
			}
		}()
		fn()
	}
	panics("NewCollection", func() { NewCollection(cx, taskID, []task{{id: 1}, {id: 1}}) })
	c := NewCollection(cx, taskID, []task{{id: 1}})
	panics("Replace", func() { c.Replace([]task{{id: 2}, {id: 2}}) })
	panics("Update", func() { c.Update(1, func(t task) task { t.id = 9; return t }) })
}

func TestCollectionRunsOneListRowPerChange(t *testing.T) {
	var c *Collection[int, task]
	builds := map[int]int{}
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		items := make([]task, 50)
		for i := range items {
			items[i] = task{id: i, text: "task " + strconv.Itoa(i)}
		}
		c = NewCollection(cx, taskID, items)
		return List{Count: c.Len, Key: func(i int) any { return c.KeyAt(i) }, Style: Style{FlexGrow: 1},
			Row: func(cx *Scope, row ListRow) Node {
				return Text{Style: Style{Height: Pt(30)}, ContentFunc: func() string {
					id := c.KeyAt(row.Index())
					it, _ := c.Get(id)
					builds[id]++
					if it.done {
						return it.text + " done"
					}
					return it.text
				}}
			}}
	})
	// Measured rows may widen the window the estimate mounted: let it settle first.
	for range 3 {
		h.commit()
		h.paced()
	}
	before := map[int]int{}
	for k, v := range builds {
		before[k] = v
	}
	if len(before) < 5 {
		t.Fatalf("only %d rows built", len(before))
	}
	c.Update(3, func(t task) task { t.done = true; return t })
	h.commit()
	for id, n := range builds {
		want := before[id]
		if id == 3 {
			want++
		}
		if n != want {
			t.Fatalf("row %d ran %d times, want %d", id, n, want)
		}
	}
}
