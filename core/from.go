package core

import "sync"

// From returns an accessor of the latest value a source outside the graph delivers, as Solid's from does: a subscription to a store, a socket or a file watcher. subscribe runs once, untracked, with set, which any goroutine may call; it returns stop, which runs when the current owner is disposed. Values that arrive before the graph takes them coalesce, the latest winning, and a value equal to the one held changes nothing.
//
// Its values reach the graph through Post, so the graph needs a host: an engine or a test driver.
func From[T comparable](cx *Scope, initial T, subscribe func(set func(T)) (stop func())) func() T {
	return FromFunc(cx, initial, func(a, b T) bool { return a == b }, subscribe)
}

// FromFunc returns an accessor as From does that compares values with equal; with a nil equal every value notifies, as slices and maps need.
func FromFunc[T any](cx *Scope, initial T, equal func(a, b T) bool, subscribe func(set func(T)) (stop func())) func() T {
	if cx.rt.post == nil {
		panic("core: From needs a host: start the scope's graph with an engine or a test driver")
	}
	o := cx.current()
	s := NewSignalFunc(cx, initial, equal)
	var mu sync.Mutex
	var latest T
	pending := false
	set := func(v T) {
		mu.Lock()
		latest = v
		post := !pending
		pending = true
		mu.Unlock()
		if post {
			o.Post(func() {
				mu.Lock()
				v := latest
				var zero T
				latest, pending = zero, false
				mu.Unlock()
				s.Set(v)
			})
		}
	}
	var stop func()
	cx.Untrack(func() { stop = subscribe(set) })
	if stop != nil {
		o.addCleanup(stop)
	}
	return s.Get
}
