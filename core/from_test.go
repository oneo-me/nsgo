package core

import (
	"sync"
	"testing"
)

func TestFromFollowsAnOutsideSource(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	var mu sync.Mutex
	var set func(int)
	stopped := false
	owner := h.root.Child()
	get := From(owner, 0, func(s func(int)) func() {
		mu.Lock()
		set = s
		mu.Unlock()
		return func() { stopped = true }
	})
	var seen []int
	owner.Effect(func() { seen = append(seen, get()) })

	// Values from another goroutine before the graph takes them coalesce to the latest.
	done := make(chan struct{})
	go func() {
		mu.Lock()
		s := set
		mu.Unlock()
		s(1)
		s(2)
		s(3)
		close(done)
	}()
	<-done
	h.waitPost()
	h.drain()
	if get() != 3 {
		t.Fatalf("value = %d, want 3", get())
	}
	// An equal value changes nothing.
	set(3)
	h.waitPost()
	h.drain()
	if len(seen) != 2 || seen[0] != 0 || seen[1] != 3 {
		t.Fatalf("seen = %v", seen)
	}

	owner.Dispose()
	if !stopped {
		t.Fatal("the owner's disposal did not stop the subscription")
	}
	set(4)
	h.waitPost()
	h.drain()
	if get() != 3 {
		t.Fatalf("a value after the owner went reached it: %d", get())
	}
}

func TestFromFuncNotifiesSlices(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	var set func([]string)
	get := FromFunc(h.root, nil, nil, func(s func([]string)) func() { set = s; return nil })
	runs := 0
	h.root.Effect(func() { get(); runs++ })
	set([]string{"a"})
	h.waitPost()
	h.drain()
	set([]string{"a"})
	h.waitPost()
	h.drain()
	if runs != 3 || len(get()) != 1 {
		t.Fatalf("runs = %d, value = %v", runs, get())
	}
}

func TestFromNeedsAHost(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("From without a host did not panic")
		}
	}()
	From(NewRoot(RootOptions{}), 0, func(func(int)) func() { return nil })
}

func TestFromTakesTheValueHandedAsItSubscribes(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	var set func(string)
	get := From(h.root, "initial", func(s func(string)) func() {
		set = s
		s("current")
		return nil
	})
	if got := get(); got != "current" {
		t.Fatalf("value = %q before any post, want the value handed as it subscribed", got)
	}
	h.mu.Lock()
	posted := len(h.posts)
	h.mu.Unlock()
	if posted != 0 {
		t.Fatalf("%d posts for a value taken at once", posted)
	}
	// Later values go through Post as before.
	set("later")
	if get() != "current" {
		t.Fatal("a later value skipped Post")
	}
	h.waitPost()
	h.drain()
	if get() != "later" {
		t.Fatalf("value = %q", get())
	}
}
