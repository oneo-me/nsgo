package core

import (
	"fmt"
	"slices"
)

// Collection holds items by key, in order, each in a signal of its own, as a Solid store holds an array: a computation that reads one item follows that item alone, and one that reads the order follows only insertions, removals and moves. A list's rows read their own items, so changing one item runs one row where a signal of the whole slice runs them all.
//
// Like a signal's, its methods belong to the graph's goroutine.
type Collection[K comparable, T any] struct {
	owner *Scope
	key   func(T) K
	equal func(a, b T) bool
	// order is never changed in place, so a slice Keys returned stays as it was.
	order *Signal[[]K]
	items map[K]*Signal[collectionItem[T]]
}

type collectionItem[T any] struct {
	v  T
	ok bool // false once the item is deleted, for readers still holding its signal
}

// NewCollection returns a collection of items, keyed by key; an item set equal to the one held changes nothing. Two items with the same key panic.
func NewCollection[K, T comparable](cx *Scope, key func(T) K, items []T) *Collection[K, T] {
	return NewCollectionFunc(cx, key, func(a, b T) bool { return a == b }, items)
}

// NewCollectionFunc returns a collection as NewCollection does that compares items with equal; with a nil equal every item set notifies its readers, as items holding slices or maps need.
func NewCollectionFunc[K comparable, T any](cx *Scope, key func(T) K, equal func(a, b T) bool, items []T) *Collection[K, T] {
	c := &Collection[K, T]{owner: cx.current(), key: key, equal: equal, items: map[K]*Signal[collectionItem[T]]{}}
	order := make([]K, 0, len(items))
	for _, it := range items {
		k := key(it)
		if _, dup := c.items[k]; dup {
			panic(fmt.Sprintf("core: a collection holds two items keyed %v", k))
		}
		c.items[k] = c.newItem(it)
		order = append(order, k)
	}
	c.order = NewSignalFunc(cx, order, nil)
	return c
}

func (c *Collection[K, T]) newItem(v T) *Signal[collectionItem[T]] {
	var equal func(a, b collectionItem[T]) bool
	if c.equal != nil {
		equal = func(a, b collectionItem[T]) bool { return a.ok == b.ok && c.equal(a.v, b.v) }
	}
	return NewSignalFunc(c.owner, collectionItem[T]{v: v, ok: true}, equal)
}

// Len returns the number of items, following the order.
func (c *Collection[K, T]) Len() int { return len(c.order.Get()) }

// KeyAt returns the key of the item at i, following the order.
func (c *Collection[K, T]) KeyAt(i int) K { return c.order.Get()[i] }

// Keys returns the keys in order, following the order. The slice is the collection's: it does not change, and must not be changed.
func (c *Collection[K, T]) Keys() []K { return c.order.Get() }

// Get returns the item keyed k, following that item alone, and whether the collection holds one; without one it follows the order, to see one added.
func (c *Collection[K, T]) Get(k K) (T, bool) {
	if s := c.items[k]; s != nil {
		it := s.Get()
		return it.v, it.ok
	}
	c.order.Get()
	var zero T
	return zero, false
}

// Items returns the items in order, following the order and every item.
func (c *Collection[K, T]) Items() []T {
	keys := c.order.Get()
	out := make([]T, len(keys))
	for i, k := range keys {
		out[i] = c.items[k].Get().v
	}
	return out
}

// Set replaces the item with item's key, or adds item at the end.
func (c *Collection[K, T]) Set(item T) {
	k := c.key(item)
	if s := c.items[k]; s != nil {
		s.Set(collectionItem[T]{v: item, ok: true})
		return
	}
	c.owner.Batch(func() {
		c.items[k] = c.newItem(item)
		c.order.Set(append(slices.Clip(c.order.GetUntracked()), k))
	})
}

// Update replaces the item keyed k with what fn returns for it, and reports whether the collection held one. fn must keep the item's key.
func (c *Collection[K, T]) Update(k K, fn func(T) T) bool {
	s := c.items[k]
	if s == nil {
		return false
	}
	v := fn(s.GetUntracked().v)
	if nk := c.key(v); nk != k {
		panic(fmt.Sprintf("core: Collection.Update changed an item's key from %v to %v", k, nk))
	}
	s.Set(collectionItem[T]{v: v, ok: true})
	return true
}

// Delete removes the item keyed k, and reports whether the collection held one.
func (c *Collection[K, T]) Delete(k K) bool {
	s := c.items[k]
	if s == nil {
		return false
	}
	c.owner.Batch(func() {
		delete(c.items, k)
		s.Set(collectionItem[T]{v: s.GetUntracked().v})
		c.order.Set(slices.DeleteFunc(slices.Clone(c.order.GetUntracked()), func(x K) bool { return x == k }))
	})
	return true
}

// Replace makes the collection hold items, in their order, as Solid's reconcile does: an item whose key it held replaces that item, notifying its readers only when it differs, items with new keys are added, the others removed, and the order notifies only when it changes. Two items with the same key panic.
func (c *Collection[K, T]) Replace(items []T) {
	order := make([]K, 0, len(items))
	seen := make(map[K]bool, len(items))
	for _, it := range items {
		k := c.key(it)
		if seen[k] {
			panic(fmt.Sprintf("core: a collection holds two items keyed %v", k))
		}
		seen[k] = true
		order = append(order, k)
	}
	c.owner.Batch(func() {
		for k, s := range c.items {
			if !seen[k] {
				delete(c.items, k)
				s.Set(collectionItem[T]{v: s.GetUntracked().v})
			}
		}
		for i, it := range items {
			if s := c.items[order[i]]; s != nil {
				s.Set(collectionItem[T]{v: it, ok: true})
			} else {
				c.items[order[i]] = c.newItem(it)
			}
		}
		if !slices.Equal(order, c.order.GetUntracked()) {
			c.order.Set(order)
		}
	})
}
