package core

import (
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
