package core

import (
	"slices"
	"strconv"
	"testing"
)

// A list's rows read a slice by index in their bindings, as the tutorials' do: rows that go must not run with an index past the slice's end.
func TestListRowsNeverReadPastTheirItems(t *testing.T) {
	for _, n := range []int{3, 50} {
		for _, keyed := range []bool{false, true} {
			var items *Signal[[]string]
			h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
				all := make([]string, n)
				for i := range all {
					all[i] = "item " + strconv.Itoa(i)
				}
				items = NewSignalFunc(cx, all, nil)
				l := List{Count: func() int { return len(items.Get()) }, Style: Style{FlexGrow: 1},
					Row: func(cx *Scope, row ListRow) Node {
						return Text{Style: Style{Height: Pt(30)}, ContentFunc: func() string { return items.Get()[row.Index()] }}
					}}
				if keyed {
					l.Key = func(i int) any { return items.Get()[i] }
				}
				return l
			})
			h.commit()
			h.paced()
			// Shrink twice, so the list's own binding has run again and subscribed after the rows.
			for _, keep := range []int{n - 1, n / 2, 1, 0} {
				items.Set(items.GetUntracked()[:keep])
				h.commit()
			}
			items.Set([]string{"a", "b"})
			h.commit()
			if len(h.errs) != 0 {
				t.Fatalf("%d items, keyed %v: %v", n, keyed, h.errs[0])
			}
		}
	}
}

func TestListSpareCellsWaitUntilTheyShowARow(t *testing.T) {
	var items *Signal[[]string]
	runs := map[string]int{}
	effects := 0
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		items = NewSignalFunc(cx, []string{"a", "b", "c"}, nil)
		return List{Count: func() int { return len(items.Get()) }, Style: Style{FlexGrow: 1},
			Row: func(cx *Scope, row ListRow) Node {
				cx.Effect(func() { items.Get(); effects++ })
				return Text{Style: Style{Height: Pt(30)}, ContentFunc: func() string {
					s := items.Get()[row.Index()]
					runs[s]++
					return s
				}}
			}}
	})
	h.commit()
	items.Set([]string{"a"})
	h.commit()
	before := effects
	// The spare cells of b and c run nothing as the items change.
	items.Set([]string{"x"})
	h.commit()
	if runs["x"] != 1 || effects != before+1 {
		t.Fatalf("runs = %v, effects %d after %d", runs, effects, before)
	}
	// Growing again, a spare cell shows its new row and runs once with its index.
	items.Set([]string{"x", "y", "z"})
	h.commit()
	if runs["y"] != 1 || runs["z"] != 1 {
		t.Fatalf("runs = %v", runs)
	}
}

func TestListReusedCellsRunOnceWithTheirRowAndCount(t *testing.T) {
	var items *Signal[[]string]
	type seen struct {
		item  string
		count int
	}
	var runs []seen
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		items = NewSignalFunc(cx, []string{"a", "b", "c"}, nil)
		return List{Count: func() int { return len(items.Get()) }, Style: Style{FlexGrow: 1},
			Row: func(cx *Scope, row ListRow) Node {
				return Text{Style: Style{Height: Pt(30)}, ContentFunc: func() string {
					s := items.Get()[row.Index()]
					runs = append(runs, seen{s, row.Count()})
					return s
				}}
			}}
	})
	h.commit()
	items.Set([]string{"a"})
	h.commit()
	runs = nil
	// The spare cells come back for rows 1 to 3 of four: each runs once, already knowing the count.
	items.Set([]string{"a", "x", "y", "z"})
	h.commit()
	n := map[string]int{}
	for _, r := range runs {
		if r.item != "a" && r.count != 4 {
			t.Fatalf("%q ran with count %d: %v", r.item, r.count, runs)
		}
		n[r.item]++
	}
	if n["x"] != 1 || n["y"] != 1 || n["z"] != 1 {
		t.Fatalf("runs = %v", runs)
	}
}

func TestListSpareCellsMemosKeepTheirValueForReadersOutside(t *testing.T) {
	var items *Signal[[]string]
	var memos []*Memo[string]
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		items = NewSignalFunc(cx, []string{"a", "b", "c"}, nil)
		return List{Count: func() int { return len(items.Get()) }, Style: Style{FlexGrow: 1},
			Row: func(cx *Scope, row ListRow) Node {
				m := NewMemo(cx, func() string { return items.Get()[row.Index()] })
				memos = append(memos, m)
				return Text{Style: Style{Height: Pt(30)}, ContentFunc: m.Get}
			}}
	})
	h.commit()
	var seen []string
	h.root.Effect(func() {
		seen = seen[:0]
		for _, m := range memos {
			seen = append(seen, m.Get())
		}
	})
	items.Set([]string{"x"})
	h.commit()
	if len(h.errs) != 0 {
		t.Fatalf("an outside read of a spare cell's memo: %v", h.errs[0])
	}
	if !slices.Contains(seen, "x") || !slices.Contains(seen, "b") {
		t.Fatalf("seen = %v, want the shown row's new value and a spare cell's last", seen)
	}
	// Shown again, the cell computes its memo and its readers outside follow.
	items.Set([]string{"x", "y"})
	h.commit()
	if !slices.Contains(seen, "y") || len(h.errs) != 0 {
		t.Fatalf("seen = %v, errors %v", seen, h.errs)
	}
}

// A memo in a row that only readers outside the cell read: shown again, the cell computes it for them.
func TestListSpareCellsMemosReadOnlyOutsideFollowTheirNewRow(t *testing.T) {
	var items *Signal[[]string]
	var memos []*Memo[string]
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		items = NewSignalFunc(cx, []string{"a", "b", "c"}, nil)
		return List{Count: func() int { return len(items.Get()) }, Style: Style{FlexGrow: 1},
			Row: func(cx *Scope, row ListRow) Node {
				memos = append(memos, NewMemo(cx, func() string { return items.Get()[row.Index()] }))
				return Text{Style: Style{Height: Pt(30)}, Content: "row"}
			}}
	})
	h.commit()
	var seen []string
	h.root.Effect(func() {
		seen = seen[:0]
		for _, m := range memos {
			seen = append(seen, m.Get())
		}
	})
	items.Set([]string{"x"})
	h.commit()
	items.Set([]string{"x", "y", "z"})
	h.commit()
	if len(h.errs) != 0 {
		t.Fatalf("errors %v", h.errs)
	}
	if !slices.Contains(seen, "y") || !slices.Contains(seen, "z") {
		t.Fatalf("seen = %v, want the reused cells' new rows", seen)
	}
}
