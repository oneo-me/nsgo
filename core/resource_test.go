package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

// pendingLoad is a load a test fetcher waits in until the test ends it.
type pendingLoad struct {
	ctx   context.Context
	src   int
	info  ResourceInfo
	reply chan loadReply
}

type loadReply struct {
	v     string
	err   error
	panic any
}

// loads returns a fetcher that hands each load to the test, and next, which waits for the next one.
func loads(t *testing.T) (fetch func(context.Context, int, ResourceInfo) (string, error), next func() *pendingLoad, none func()) {
	t.Helper()
	calls := make(chan *pendingLoad, 8)
	fetch = func(ctx context.Context, src int, info ResourceInfo) (string, error) {
		l := &pendingLoad{ctx: ctx, src: src, info: info, reply: make(chan loadReply)}
		calls <- l
		r := <-l.reply
		if r.panic != nil {
			panic(r.panic)
		}
		return r.v, r.err
	}
	next = func() *pendingLoad {
		t.Helper()
		select {
		case l := <-calls:
			return l
		case <-time.After(5 * time.Second):
			t.Fatal("no load started")
			return nil
		}
	}
	none = func() {
		t.Helper()
		select {
		case l := <-calls:
			t.Fatalf("a load of %d started", l.src)
		case <-time.After(20 * time.Millisecond):
		}
	}
	return fetch, next, none
}

// end ends l with r and runs what it posts.
func (h *host) end(l *pendingLoad, r loadReply) {
	l.reply <- r
	h.waitPost()
	h.drain()
}

func cancelled(t *testing.T, ctx context.Context) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the load's context was not cancelled")
	}
}

func wantResource(t *testing.T, r *Resource[string], state ResourceState, value string, hasValue bool, err error) {
	t.Helper()
	v, ok := r.Get()
	if r.State() != state || v != value || ok != hasValue || !errors.Is(r.Err(), err) {
		t.Fatalf("resource = %v %q %v %v, want %v %q %v %v", r.State(), v, ok, r.Err(), state, value, hasValue, err)
	}
}

func TestResourceLoadsAndFollowsItsSource(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	fetch, next, _ := loads(t)
	id := NewSignal(h.root, 1)
	r := NewResource(h.root, func() (int, bool) { return id.Get(), true }, fetch)
	var states []ResourceState
	h.root.Effect(func() { states = append(states, r.State()) })

	first := next()
	if first.src != 1 || first.info.Refetching {
		t.Fatalf("first load = %d %+v", first.src, first.info)
	}
	wantResource(t, r, ResourcePending, "", false, nil)
	if !r.Loading() {
		t.Fatal("a pending resource is not loading")
	}
	h.end(first, loadReply{v: "one"})
	wantResource(t, r, ResourceReady, "one", true, nil)

	// A new source supersedes the load still running: its context goes, and its late result changes nothing.
	id.Set(2)
	slow := next()
	wantResource(t, r, ResourceRefreshing, "one", true, nil)
	id.Set(3)
	cancelled(t, slow.ctx)
	fast := next()
	if fast.src != 3 {
		t.Fatalf("second load = %d", fast.src)
	}
	h.end(fast, loadReply{v: "three"})
	h.end(slow, loadReply{v: "two"})
	wantResource(t, r, ResourceReady, "three", true, nil)
	want := []ResourceState{ResourcePending, ResourceReady, ResourceRefreshing, ResourceReady}
	if len(states) != len(want) {
		t.Fatalf("states = %v, want %v", states, want)
	}
	for i := range want {
		if states[i] != want[i] {
			t.Fatalf("states = %v, want %v", states, want)
		}
	}
}

func TestResourceFailsKeepingItsValueAndRefetches(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	fetch, next, _ := loads(t)
	r := NewResourceWithValue(h.root, "cached", func() (int, bool) { return 7, true }, fetch)
	first := next()
	wantResource(t, r, ResourceRefreshing, "cached", true, nil)
	offline := errors.New("offline")
	h.end(first, loadReply{err: offline})
	wantResource(t, r, ResourceErrored, "cached", true, offline)

	r.Refetch()
	retry := next()
	if retry.src != 7 || !retry.info.Refetching {
		t.Fatalf("retry = %d %+v", retry.src, retry.info)
	}
	wantResource(t, r, ResourceRefreshing, "cached", true, offline)
	h.end(retry, loadReply{v: "fresh"})
	wantResource(t, r, ResourceReady, "fresh", true, nil)
}

func TestResourceWithoutASourceLoadsNothing(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	fetch, next, none := loads(t)
	user := NewSignal(h.root, 0)
	r := NewResource(h.root, func() (int, bool) { return user.Get(), user.Get() != 0 }, fetch)
	none()
	wantResource(t, r, ResourceUnresolved, "", false, nil)
	r.Refetch()
	none()

	user.Set(5)
	l := next()
	wantResource(t, r, ResourcePending, "", false, nil)
	// The source going unset leaves the load behind.
	user.Set(0)
	cancelled(t, l.ctx)
	wantResource(t, r, ResourceUnresolved, "", false, nil)
	h.end(l, loadReply{v: "late"})
	wantResource(t, r, ResourceUnresolved, "", false, nil)
}

func TestResourceWithANilSourceLoadsOnce(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	fetch, next, none := loads(t)
	r := NewResource[int](h.root, nil, fetch)
	first := next()
	if first.src != 0 || first.info.Refetching {
		t.Fatalf("first load = %d %+v", first.src, first.info)
	}
	h.end(first, loadReply{v: "a"})
	none()
	r.Refetch()
	again := next()
	if !again.info.Refetching {
		t.Fatal("a refetch was not told so")
	}
	h.end(again, loadReply{v: "b"})
	wantResource(t, r, ResourceReady, "b", true, nil)
}

func TestResourceMutateDuringALoad(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	fetch, next, _ := loads(t)
	r := NewResource[int](h.root, nil, fetch)
	l := next()
	r.Mutate("optimistic")
	wantResource(t, r, ResourcePending, "optimistic", true, nil)
	h.end(l, loadReply{v: "server"})
	wantResource(t, r, ResourceReady, "server", true, nil)

	r.Refetch()
	failed := next()
	h.end(failed, loadReply{err: errors.New("down")})
	r.Mutate("local")
	wantResource(t, r, ResourceReady, "local", true, nil)
}

func TestResourceStopsWithItsOwner(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	fetch, next, _ := loads(t)
	owner := h.root.Child()
	r := NewResource[int](owner, nil, fetch)
	l := next()
	owner.Dispose()
	cancelled(t, l.ctx)
	h.end(l, loadReply{v: "late"})
	wantResource(t, r, ResourcePending, "", false, nil)
}

func TestResourcePanicFailsTheLoadAndReachesCatch(t *testing.T) {
	fetch, next, _ := loads(t)
	var r *Resource[string]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		return Catch{
			Try: func(cx *Scope) Node {
				r = NewResource[int](cx, nil, fetch)
				return Text{Content: "loading", TestID: "loading"}
			},
			Fallback: func(cx *Scope, err error, _ func()) Node {
				return Text{Content: err.Error()[:len("panic: boom")], TestID: "fallback"}
			},
		}
	})
	h.commit()
	h.end(next(), loadReply{panic: "boom"})
	h.commit()
	if got := h.find("fallback").text.content; got != "panic: boom" {
		t.Fatalf("fallback = %q", got)
	}
	var p *PanicError
	if !errors.As(r.Err(), &p) || p.Value != "boom" || len(p.Stack) == 0 || r.State() != ResourceErrored {
		t.Fatalf("resource = %v %v", r.State(), r.Err())
	}
	if len(h.errs) != 0 {
		t.Fatalf("reported %v", h.errs)
	}
}

func TestResourcePanicWithoutCatchIsReported(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, nil)
	fetch, next, _ := loads(t)
	NewResource[int](h.root, nil, fetch)
	h.end(next(), loadReply{panic: "boom"})
	var p *PanicError
	if len(h.errs) != 1 || !errors.As(h.errs[0], &p) || p.Value != "boom" {
		t.Fatalf("reported %v", h.errs)
	}
}

func TestResourceNeedsAHost(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a resource without a host did not panic")
		}
	}()
	NewResource[int](NewRoot(RootOptions{}), nil, func(context.Context, int, ResourceInfo) (string, error) { return "", nil })
}

func TestResourceStateNames(t *testing.T) {
	for s, want := range map[ResourceState]string{
		ResourceUnresolved: "unresolved", ResourcePending: "pending", ResourceReady: "ready",
		ResourceRefreshing: "refreshing", ResourceErrored: "errored",
	} {
		if s.String() != want {
			t.Errorf("%d = %q, want %q", s, s, want)
		}
	}
}
