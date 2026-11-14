package core

import (
	"context"
	"runtime/debug"
)

// ResourceState is where a Resource is (NSH's ResourceState).
type ResourceState uint8

const (
	// ResourceUnresolved has nothing loaded and nothing loading: its source is not set.
	ResourceUnresolved ResourceState = iota
	// ResourcePending is loading, with no value yet.
	ResourcePending
	// ResourceReady has a value, and nothing loading.
	ResourceReady
	// ResourceRefreshing is loading again, with the last value kept.
	ResourceRefreshing
	// ResourceErrored says the last load failed.
	ResourceErrored
)

// String returns its name in TSX: "unresolved", "pending", "ready", "refreshing" or "errored".
func (s ResourceState) String() string {
	switch s {
	case ResourcePending:
		return "pending"
	case ResourceReady:
		return "ready"
	case ResourceRefreshing:
		return "refreshing"
	case ResourceErrored:
		return "errored"
	}
	return "unresolved"
}

// Loading reports whether a load runs.
func (s ResourceState) Loading() bool { return s == ResourcePending || s == ResourceRefreshing }

// ResourceInfo is what a load is told besides its source.
type ResourceInfo struct {
	// Refetching says Refetch started the load: the source did not change.
	Refetching bool
}

// Resource is a value loaded on another goroutine from a source, loaded again when the source changes or on Refetch, as NSH's resource and Solid's createResource are. Its value, error and state are signals: bindings that read them follow its loads. Only the latest load counts: a newer one cancels the context of the one running, and so does the owner's disposal.
//
// Like a signal's, its methods belong to the graph's goroutine.
type Resource[T any] struct {
	owner      *Scope
	value      *Signal[resourceValue[T]]
	err        *Signal[error]
	state      *Signal[ResourceState]
	trigger    *Signal[uint64]
	refetching bool
	runs       resourceRuns
}

type resourceValue[T any] struct {
	v  T
	ok bool
}

// NewResource returns a resource that loads fetch(ctx, s) whenever source gives a new s with ok true, and on Refetch. source is tracked and runs on the graph's goroutine; with ok false the resource loads nothing, as for a user not signed in yet, and a nil source loads once with S's zero value. fetch runs on a goroutine of its own, so it may block on the network or the disk; ctx is cancelled when a newer load starts or the owner is disposed, and the result of a load that no longer counts is dropped. A panic in fetch fails the load with a *PanicError and goes to the nearest Catch, as a panic in app code does.
//
// Its results reach the graph through Post, so the graph needs a host: an engine or a test driver.
func NewResource[S, T any](cx *Scope, source func() (S, bool), fetch func(ctx context.Context, source S, info ResourceInfo) (T, error)) *Resource[T] {
	return newResource(cx, resourceValue[T]{}, source, fetch)
}

// NewResourceWithValue returns a resource as NewResource does that holds value until its first load ends, which is a refresh (TSX's initialValue).
func NewResourceWithValue[S, T any](cx *Scope, value T, source func() (S, bool), fetch func(ctx context.Context, source S, info ResourceInfo) (T, error)) *Resource[T] {
	return newResource(cx, resourceValue[T]{v: value, ok: true}, source, fetch)
}

func newResource[S, T any](cx *Scope, initial resourceValue[T], source func() (S, bool), fetch func(context.Context, S, ResourceInfo) (T, error)) *Resource[T] {
	if cx.rt.post == nil {
		panic("core: NewResource needs a host: start the scope's graph with an engine or a test driver")
	}
	r := &Resource[T]{owner: cx.current()}
	if initial.ok {
		r.runs = resourceRuns{hasValue: true, state: ResourceReady}
	}
	r.value = NewSignalFunc(cx, initial, nil)
	r.err = NewSignalFunc[error](cx, nil, nil)
	r.state = NewSignal(cx, r.runs.state)
	r.trigger = NewSignal(cx, uint64(0))
	// A render effect, so what a load starts with is there before the bindings run. Each run is the effect's owner again, which cancels the context of the load the previous run started.
	cx.renderEffect(func() {
		r.trigger.Get()
		var src S
		ok := true
		if source != nil {
			src, ok = source()
		}
		cx.Untrack(func() {
			info := ResourceInfo{Refetching: r.refetching}
			r.refetching = false
			if !ok {
				r.state.Set(r.runs.skip())
				return
			}
			run, state := r.runs.start()
			r.state.Set(state)
			go r.load(cx.Context(), run, func(ctx context.Context) (T, error) { return fetch(ctx, src, info) })
		})
	})
	return r
}

// load runs a load on its own goroutine and hands its end to the graph.
func (r *Resource[T]) load(ctx context.Context, run uint64, fetch func(context.Context) (T, error)) {
	var v T
	var err error
	var failure *PanicError
	func() {
		defer func() {
			if x := recover(); x != nil {
				failure = &PanicError{Value: x, Stack: debug.Stack()}
			}
		}()
		v, err = fetch(ctx)
	}()
	// Post drops it once the owner is disposed.
	r.owner.Post(func() {
		if failure != nil {
			err = failure
			defer r.owner.rt.raise(r.owner, failure)
		}
		if err != nil {
			if state, ok := r.runs.reject(run); ok {
				r.err.Set(err)
				r.state.Set(state)
			}
			return
		}
		if state, ok := r.runs.resolve(run); ok {
			r.value.Set(resourceValue[T]{v: v, ok: true})
			r.err.Set(nil)
			r.state.Set(state)
		}
	})
}

// Get returns the latest value, kept while loading again and after a failure, with ok false before the first; it subscribes the running computation, and has a Suspense around it wait while the first value loads.
func (r *Resource[T]) Get() (v T, ok bool) {
	suspend(r, r.owner)
	rv := r.value.Get()
	return rv.v, rv.ok
}

// loadingFirst reports whether it loads with no value yet, for the Suspense around a computation that read it.
func (r *Resource[T]) loadingFirst() bool { return r.state.Get() == ResourcePending }

// Err returns the last load's error, nil once a load gave a value; it subscribes the running computation.
func (r *Resource[T]) Err() error { return r.err.Get() }

// State returns where the resource is; it subscribes the running computation.
func (r *Resource[T]) State() ResourceState { return r.state.Get() }

// Loading reports whether a load runs; it subscribes the running computation.
func (r *Resource[T]) Loading() bool { return r.state.Get().Loading() }

// Refetch loads the resource again from the same source, as a retry or a pull to refresh does, keeping the value meanwhile. A resource whose source is not set stays as it is.
func (r *Resource[T]) Refetch() {
	r.refetching = true
	r.trigger.Update(func(n uint64) uint64 { return n + 1 })
}

// Mutate sets the value with no load, as an optimistic update does, and clears the last error; a load still running replaces it when it ends.
func (r *Resource[T]) Mutate(v T) {
	state := r.runs.mutate()
	r.owner.Batch(func() {
		r.value.Set(resourceValue[T]{v: v, ok: true})
		r.err.Set(nil)
		r.state.Set(state)
	})
}

// resourceRuns are a resource's loads: which is the latest, whether it runs, and whether a value was ever loaded or set (NSH's Runs).
type resourceRuns struct {
	current  uint64
	running  bool
	hasValue bool
	state    ResourceState
}

// start begins a load that supersedes any running one, and returns it with the resource's state from now on.
func (rs *resourceRuns) start() (uint64, ResourceState) {
	rs.current++
	rs.running = true
	rs.state = ResourcePending
	if rs.hasValue {
		rs.state = ResourceRefreshing
	}
	return rs.current, rs.state
}

// skip says the source is not set: nothing loads, and a running load no longer counts.
func (rs *resourceRuns) skip() ResourceState {
	rs.current++
	rs.running = false
	rs.state = ResourceUnresolved
	if rs.hasValue {
		rs.state = ResourceReady
	}
	return rs.state
}

// resolve says run gave a value, and returns the new state; ok is false when run no longer counts.
func (rs *resourceRuns) resolve(run uint64) (ResourceState, bool) {
	if !rs.end(run) {
		return rs.state, false
	}
	rs.hasValue = true
	rs.state = ResourceReady
	return rs.state, true
}

// reject says run failed, and returns the new state, the value kept; ok is false when run no longer counts.
func (rs *resourceRuns) reject(run uint64) (ResourceState, bool) {
	if !rs.end(run) {
		return rs.state, false
	}
	rs.state = ResourceErrored
	return rs.state, true
}

// mutate says the value was set directly: a running load goes on and replaces it when it ends.
func (rs *resourceRuns) mutate() ResourceState {
	rs.hasValue = true
	if !rs.running {
		rs.state = ResourceReady
	}
	return rs.state
}

func (rs *resourceRuns) end(run uint64) bool {
	if !rs.running || run != rs.current {
		return false
	}
	rs.running = false
	return true
}
