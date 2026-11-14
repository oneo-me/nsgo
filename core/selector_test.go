package core

import "testing"

func TestSelectorRunsOnlyTheRowsThatChange(t *testing.T) {
	cx := NewRoot(RootOptions{})
	selected := NewSignal(cx, 2)
	is := NewSelector(cx, selected.Get)
	runs := make([]int, 5)
	states := make([]bool, 5)
	rows := make([]*Scope, 5)
	for i := range 5 {
		rows[i] = cx.Child()
		rows[i].renderEffect(func() {
			runs[i]++
			states[i] = is(i)
		})
	}
	if !states[2] || states[1] || runs[2] != 1 {
		t.Fatalf("states = %v, runs = %v", states, runs)
	}

	selected.Set(4)
	want := []int{1, 1, 2, 1, 2}
	for i := range want {
		if runs[i] != want[i] {
			t.Fatalf("runs = %v, want %v", runs, want)
		}
	}
	if states[2] || !states[4] {
		t.Fatalf("states = %v", states)
	}
	// Outside a computation it answers without subscribing.
	if !is(4) || is(2) {
		t.Fatal("an untracked read is wrong")
	}

	// Keys nothing reads leave the selector, and a row built again reads the current value.
	rows[4].Dispose()
	selected.Set(0)
	if runs[4] != 2 || runs[0] != 2 || !states[0] {
		t.Fatalf("runs = %v, states = %v", runs, states)
	}
	again := cx.Child()
	var on bool
	again.renderEffect(func() { on = is(0) })
	if !on {
		t.Fatal("a new reader of the selected key reads false")
	}
}

func TestSelectorForgetsKeysNobodyReads(t *testing.T) {
	cx := NewRoot(RootOptions{})
	selected := NewSignal(cx, 0)
	s := newSelector(cx, selected.Get)
	rows := make([]*Scope, 100)
	for i := range rows {
		rows[i] = cx.Child()
		rows[i].renderEffect(func() { s.is(i % 10) })
	}
	if len(s.keys) != 10 || s.keys[3].readers != 10 {
		t.Fatalf("keys = %d", len(s.keys))
	}
	// A reader running again keeps one subscription, not one more per run.
	selected.Set(3)
	selected.Set(0)
	if s.keys[3].readers != 10 || s.keys[0].readers != 10 {
		t.Fatalf("readers = %d, %d", s.keys[3].readers, s.keys[0].readers)
	}
	for _, row := range rows {
		row.Dispose()
	}
	if len(s.keys) != 0 {
		t.Fatalf("%d keys outlived their readers", len(s.keys))
	}
}
