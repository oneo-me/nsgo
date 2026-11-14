package core_test

import (
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The writes are the app's here, so this test is outside core, whose own frames a write's site skips.
func TestCommitsRecordWhatTheyFollowFrom(t *testing.T) {
	driver.TraceCauses = true
	defer func() { driver.TraceCauses = false }()
	root := core.NewRoot(core.RootOptions{Post: func(func()) {}, Clock: core.NewManualClock(time.Unix(0, 0))})
	tree := core.NewTree(root, core.ApproxTextEngine{})
	tree.Resize(core.Size{Width: 100, Height: 100})
	var count *core.Signal[int]
	tree.Mount(func(cx *core.Scope) core.Node {
		count = core.NewSignal(cx, 0)
		return core.Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) }}
	})
	tree.Commit()

	_, _, line, _ := runtime.Caller(0)
	count.Set(1)
	count.Set(2)
	count.Update(func(n int) int { return n + 1 })
	b := tree.Commit()
	if b == nil || b.Causes == nil {
		t.Fatal("the commit recorded no causes")
	}
	c := b.Causes
	if len(c.Writes) != 3 {
		t.Fatalf("writes = %+v, want one per line", c.Writes)
	}
	for i, w := range c.Writes {
		if want := "core/causes_test.go:" + strconv.Itoa(line+1+i); w.Site != want || w.Count != 1 {
			t.Errorf("write %d = %+v, want %s", i, w, want)
		}
	}
	// Each write outside a batch ran the text's binding again: what the trace shows to batch.
	if c.Renders != 3 || c.Effects != 0 {
		t.Errorf("ran %d bindings and %d effects, want the text's binding once a write", c.Renders, c.Effects)
	}

	// What the next commit follows from starts anew, and writes in one batch run the binding once.
	root.Batch(func() {
		count.Set(10)
		count.Set(11)
	})
	if c := tree.Commit().Causes; len(c.Writes) != 2 || c.Renders != 1 {
		t.Errorf("the next commit's causes = %+v", c)
	}
}

func TestCommitsRecordNothingUnlessTraced(t *testing.T) {
	root := core.NewRoot(core.RootOptions{Post: func(func()) {}})
	tree := core.NewTree(root, core.ApproxTextEngine{})
	tree.Resize(core.Size{Width: 100, Height: 100})
	var count *core.Signal[int]
	tree.Mount(func(cx *core.Scope) core.Node {
		count = core.NewSignal(cx, 0)
		return core.Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) }}
	})
	tree.Commit()
	count.Set(1)
	if b := tree.Commit(); b.Causes != nil {
		t.Fatalf("an untraced commit recorded %+v", b.Causes)
	}
}
