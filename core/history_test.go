package core

import (
	"slices"
	"testing"
	"time"
)

func TestHistoryUndoesAndRedoes(t *testing.T) {
	cx := NewRoot(RootOptions{})
	h := NewHistory(cx, "", HistoryOptions{})
	var can [][2]bool
	cx.renderEffect(func() { can = append(can, [2]bool{h.CanUndo(), h.CanRedo()}) })
	h.Set("a")
	h.Set("ab")
	h.Set("ab") // equal: no step
	if !h.Undo() || h.Get() != "a" || !h.CanRedo() {
		t.Fatalf("after undo = %q", h.Get())
	}
	if !h.Undo() || h.Get() != "" || h.Undo() || h.CanUndo() {
		t.Fatalf("after undoing all = %q", h.Get())
	}
	if !h.Redo() || h.Get() != "a" || !h.Redo() || h.Get() != "ab" || h.Redo() {
		t.Fatalf("after redo = %q", h.Get())
	}
	// A change after an undo drops what was undone.
	h.Undo()
	h.Set("ac")
	if h.CanRedo() || !h.Undo() || h.Get() != "a" {
		t.Fatalf("after a new change = %q", h.Get())
	}
	if last := can[len(can)-1]; last != [2]bool{true, true} {
		t.Fatalf("can = %v", can)
	}
	h.Clear()
	if h.CanUndo() || h.CanRedo() || h.Get() != "a" {
		t.Fatalf("after clear = %q %v %v", h.Get(), h.CanUndo(), h.CanRedo())
	}
}

func TestHistoryMergesChangesWithinMerge(t *testing.T) {
	clock := NewManualClock(time.Unix(0, 0))
	cx := NewRoot(RootOptions{Clock: clock})
	h := NewHistory(cx, "", HistoryOptions{Merge: time.Second})
	for _, s := range []string{"h", "he", "hel"} {
		h.Set(s)
		clock.Advance(300 * time.Millisecond)
	}
	clock.Advance(2 * time.Second)
	h.Set("hel ")
	h.Set("hel w")
	h.Break()
	h.Set("hel wo")
	var undone []string
	for h.Undo() {
		undone = append(undone, h.Get())
	}
	if want := []string{"hel w", "hel", ""}; !slices.Equal(undone, want) {
		t.Fatalf("undone through %q, want %q", undone, want)
	}
	// An undo ends the step, so typing after it starts another.
	h.Redo()
	h.Set("x")
	if !h.Undo() || h.Get() != "hel" {
		t.Fatalf("after undo = %q", h.Get())
	}
}

func TestHistoryGroupIsOneStep(t *testing.T) {
	cx := NewRoot(RootOptions{})
	h := NewHistoryFunc(cx, []int{}, nil, HistoryOptions{})
	h.Set([]int{1})
	h.Group(func() {
		h.Update(func(v []int) []int { return append(slices.Clone(v), 2) })
		h.Group(func() { h.Update(func(v []int) []int { return append(slices.Clone(v), 3) }) })
	})
	h.Set([]int{1, 2, 3, 4})
	h.Undo()
	if got := h.Get(); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("after one undo = %v", got)
	}
	h.Undo()
	if got := h.Get(); !slices.Equal(got, []int{1}) {
		t.Fatalf("the group was not one step: %v", got)
	}
}

func TestHistoryKeepsItsLimit(t *testing.T) {
	cx := NewRoot(RootOptions{})
	h := NewHistory(cx, 0, HistoryOptions{Limit: 3})
	for i := 1; i <= 10; i++ {
		h.Set(i)
	}
	n := 0
	for h.Undo() {
		n++
	}
	if n != 3 || h.Get() != 7 {
		t.Fatalf("undid %d steps to %d", n, h.Get())
	}
	all := NewHistory(cx, 0, HistoryOptions{Limit: -1})
	for i := 1; i <= 150; i++ {
		all.Set(i)
	}
	n = 0
	for all.Undo() {
		n++
	}
	if n != 150 {
		t.Fatalf("a history without a limit undid %d", n)
	}
}
