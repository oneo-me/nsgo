package core

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// recorder is a storage backend that loads kept and records changes.
type recorder struct {
	kept    map[string]string
	err     error
	mu      sync.Mutex
	changes []StorageChange
}

func (r *recorder) Load() (map[string]string, error) { return r.kept, r.err }

func (r *recorder) Write(c StorageChange) {
	r.mu.Lock()
	r.changes = append(r.changes, c)
	r.mu.Unlock()
}

// graph is a root whose posts run when drained.
type graph struct {
	root  *Scope
	posts []func()
}

func newGraph(store *Store) *graph {
	g := &graph{}
	g.root = NewRoot(RootOptions{Post: func(fn func()) { g.posts = append(g.posts, fn) }, Storage: store})
	return g
}

func (g *graph) drain() {
	for len(g.posts) > 0 {
		p := g.posts
		g.posts = nil
		for _, fn := range p {
			fn()
		}
	}
}

func TestStorageLoadsOnceAndWritesChanges(t *testing.T) {
	b := &recorder{kept: map[string]string{"a": "1"}}
	s := newGraph(NewStore(b)).root.Storage()
	if v, ok := s.GetItem("a"); !ok || v != "1" {
		t.Fatalf("a = %q, %v", v, ok)
	}
	if err := s.SetItem("b", "2"); err != nil {
		t.Fatal(err)
	}
	s.SetItem("b", "2")
	s.RemoveItem("a")
	s.RemoveItem("missing")
	if k := s.Keys(); !slices.Equal(k, []string{"b"}) {
		t.Fatalf("keys = %v", k)
	}
	s.Clear()
	s.Clear()
	want := []StorageChange{{Op: driver.StorageSet, Key: "b", Value: "2"}, {Op: driver.StorageRemove, Key: "a"}, {Op: driver.StorageClear}}
	if !slices.Equal(b.changes, want) {
		t.Fatalf("changes = %v, want %v", b.changes, want)
	}
}

func TestStorageQuota(t *testing.T) {
	b := &recorder{kept: map[string]string{"k": strings.Repeat("v", StorageQuota-1)}}
	s := newGraph(NewStore(b)).root.Storage()
	// What was loaded counts.
	var full *StorageFullError
	if err := s.SetItem("a", "b"); !errors.As(err, &full) || full.Bytes != StorageQuota+2 {
		t.Fatalf("err = %v", err)
	}
	if _, ok := s.GetItem("a"); ok {
		t.Fatal("a refused value was kept")
	}
	// Replacing a value counts the new one only.
	if err := s.SetItem("k", strings.Repeat("z", StorageQuota-1)); err != nil {
		t.Fatal(err)
	}
	s.RemoveItem("k")
	// "键" is three bytes, "😀" four.
	if err := s.SetItem("键", strings.Repeat("😀", (StorageQuota-3)/4)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetItem("x", strings.Repeat("y", 4)); err == nil {
		t.Fatal("the quota counted runes, not bytes")
	}
}

func TestStorageFailedLoadStartsEmpty(t *testing.T) {
	s := newGraph(NewStore(&recorder{err: errors.New("unreadable")})).root.Storage()
	if _, ok := s.GetItem("a"); ok {
		t.Fatal("a has a value")
	}
	s.SetItem("a", "1")
	if v, _ := s.GetItem("a"); v != "1" {
		t.Fatalf("a = %q", v)
	}
}

func TestStorageSignalsChanges(t *testing.T) {
	store := NewStore(nil)
	a, b := newGraph(store), newGraph(store)
	var seen []string
	var keys int
	a.root.Effect(func() {
		v, _ := a.root.Storage().GetItem("draft")
		seen = append(seen, v)
	})
	a.root.Effect(func() {
		a.root.Storage().Keys()
		keys++
	})
	sa, sb := a.root.Storage(), b.root.Storage()

	// The graph's own change reaches its effects at once.
	sa.SetItem("draft", "one")
	// Another graph's comes through a post.
	sb.SetItem("draft", "two")
	if seen[len(seen)-1] != "one" {
		t.Fatalf("seen = %v before the post ran", seen)
	}
	a.drain()
	// A change to another key leaves the draft's readers alone.
	sb.SetItem("other", "x")
	a.drain()
	sb.Clear()
	a.drain()
	if want := []string{"", "one", "two", ""}; !slices.Equal(seen, want) {
		t.Fatalf("seen = %v, want %v", seen, want)
	}
	if keys != 5 {
		t.Fatalf("keys read %d times, want 5", keys)
	}

	// A disposed graph stops watching.
	a.root.Dispose()
	sb.SetItem("draft", "three")
	if len(a.posts) != 0 {
		t.Fatal("a disposed graph was told of a change")
	}
}

func TestStorageInMemoryByDefault(t *testing.T) {
	a, b := newGraph(nil), newGraph(nil)
	a.root.Storage().SetItem("draft", "saved")
	if _, ok := b.root.Storage().GetItem("draft"); ok {
		t.Fatal("graphs without a store shared entries")
	}
}
