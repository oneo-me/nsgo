package core

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
)

// The reactive graph, scheduled as Solid's: signals hold values, memos
// cache what they compute, effects run again when what they read changes.
// Bindings of elements run before the app's effects. Every computation
// and every scope owns what is created while it runs, and disposing an
// owner disposes all of it.
//
// One graph belongs to one engine and runs on its goroutine, so nothing
// here locks. Other goroutines reach it with Scope.Post.

// Scope owns what is created under it: memos, effects, cleanups, child
// scopes and the views of its elements. Components receive one as cx.
//
// Go has no goroutine-local state, so cx is how code finds its graph.
// While the graph runs code, what it creates belongs to the innermost
// running owner, whichever cx the code captured.
type Scope struct {
	rt       *runtime
	parent   *Scope
	first    *Scope // owned scopes, oldest first
	last     *Scope
	prev     *Scope
	next     *Scope
	cleanups []func()
	values   map[any]any
	catch    func(error)
	ctx      context.Context
	cancel   context.CancelFunc
	comp     *computation // set when the scope is a computation's
	disposed bool
}

// RootOptions connect a graph to its host, an engine or a test driver.
type RootOptions struct {
	// Post runs fn on the graph's goroutine later. It must be safe to
	// call from any goroutine and must not block.
	Post func(fn func())
	// Clock schedules timers; the system's when nil.
	Clock Clock
	// Report receives failures no Catch handles; they are dropped when nil.
	Report func(error)
	// Animate tells the host whether the app still wants display frames.
	// The host starts its display link on true and stops it on false, so an
	// idle app costs no wakeups. It must be safe to call from any goroutine.
	Animate func(on bool)
	// Storage keeps the app's entries; a store in memory when nil.
	Storage *Store
	// DataDir and CacheDir are the app's directories, "" where the target
	// has none.
	DataDir, CacheDir string
}

// NewRoot returns the root scope of a new graph.
func NewRoot(opts RootOptions) *Scope {
	if opts.Clock == nil {
		opts.Clock = SystemClock{}
	}
	rt := &runtime{post: opts.Post, clock: opts.Clock, report: opts.Report,
		store: opts.Storage, dataDir: opts.DataDir, cacheDir: opts.CacheDir,
		animate: opts.Animate}
	rt.root = &Scope{rt: rt}
	return rt.root
}

const maxRuns = 1_000_000

type runtime struct {
	owner    *Scope       // the owner of what running code creates
	observer *computation // the computation reads subscribe
	batch    int
	flushing bool
	render   []*computation
	user     []*computation
	runs     int
	post     func(func())
	clock    Clock
	report   func(error)
	env      *Env
	tree     *Tree
	root     *Scope
	store    *Store
	storage  *storageWatch
	dataDir  string
	cacheDir string

	animate   func(bool)
	frames    []*frameCb // per-frame callbacks; false unregisters
	animating bool
}

// frameCb is one registered per-frame callback. Identity is the pointer:
// comparing func values for equality is not reliable in Go.
type frameCb struct {
	fn func() bool
}

// OnFrame runs fn once per display frame until it returns false or stop is
// called. The host's display link runs while any callback is registered,
// so an app with none gets no frames at all: that is what keeps a static
// screen from waking the process sixty times a second.
func (cx *Scope) OnFrame(fn func() bool) (stop func()) {
	rt := cx.rt
	cb := &frameCb{fn: fn}
	rt.frames = append(rt.frames, cb)
	rt.wantFrames(true)
	stopped := false
	return func() {
		if stopped {
			return
		}
		stopped = true
		rt.removeFrame(cb)
	}
}

// removeFrame drops cb and tells the host when the last one goes.
func (rt *runtime) removeFrame(cb *frameCb) {
	for i, f := range rt.frames {
		if f == cb {
			rt.frames = append(rt.frames[:i], rt.frames[i+1:]...)
			break
		}
	}
	rt.wantFrames(len(rt.frames) > 0)
}

// WantsFrames reports whether anything is registered for display frames.
// Hosts ask before building a display link late, after the first commit.
func (cx *Scope) WantsFrames() bool { return cx.rt.animating }

// RunFrames runs the frame callbacks and reports whether any is left.
// The engine calls it once per display frame, on the graph's goroutine.
func (cx *Scope) RunFrames() bool { return cx.rt.runFrames() }

func (rt *runtime) runFrames() bool {
	live := rt.frames[:0]
	for _, cb := range rt.frames {
		if cb.fn() {
			live = append(live, cb)
		}
	}
	rt.frames = live
	// A callback that returned false must release the host's display link,
	// or a screen that stopped animating keeps a link running for nothing.
	rt.wantFrames(len(live) > 0)
	return len(live) > 0
}

// wantFrames tells the host whether frames are needed, once per change.
func (rt *runtime) wantFrames(on bool) {
	if rt.animating == on {
		return
	}
	rt.animating = on
	if rt.animate != nil {
		rt.animate(on)
	}
}

// current is the scope that owns what code using cx creates now.
func (cx *Scope) current() *Scope {
	if o := cx.rt.owner; o != nil {
		return o
	}
	return cx
}

func (cx *Scope) adopt(child *Scope) {
	child.rt = cx.rt
	child.parent = cx
	if cx.disposed {
		child.disposed = true
		return
	}
	child.prev = cx.last
	if cx.last != nil {
		cx.last.next = child
	} else {
		cx.first = child
	}
	cx.last = child
}

// Child returns a new scope owned by the current one.
func (cx *Scope) Child() *Scope {
	s := &Scope{}
	cx.current().adopt(s)
	return s
}

// Dispose disposes everything the scope owns, runs its cleanups and
// cancels its Context.
func (cx *Scope) Dispose() {
	if cx.disposed {
		return
	}
	cx.disposeOwned()
	cx.disposed = true
	if cx.comp != nil {
		cx.comp.unlink()
	}
	if p := cx.parent; p != nil {
		if cx.prev != nil {
			cx.prev.next = cx.next
		} else if p.first == cx {
			p.first = cx.next
		}
		if cx.next != nil {
			cx.next.prev = cx.prev
		} else if p.last == cx {
			p.last = cx.prev
		}
		cx.prev, cx.next = nil, nil
	}
}

// Disposed reports whether the scope has been disposed.
func (cx *Scope) Disposed() bool { return cx.disposed }

func (cx *Scope) disposeOwned() {
	for c := cx.last; c != nil; {
		prev := c.prev
		c.prev, c.next = nil, nil
		c.parent = nil
		c.disposeDetached()
		c = prev
	}
	cx.first, cx.last = nil, nil
	for i := len(cx.cleanups) - 1; i >= 0; i-- {
		cx.rt.guard(cx, cx.cleanups[i])
	}
	cx.cleanups = nil
	cx.values = nil
	if cx.cancel != nil {
		cx.cancel()
		cx.ctx, cx.cancel = nil, nil
	}
}

func (cx *Scope) disposeDetached() {
	cx.disposeOwned()
	cx.disposed = true
	if cx.comp != nil {
		cx.comp.unlink()
	}
}

// Cleanup registers fn to run when the current owner is disposed or runs
// again, last registered first, as testing.T.Cleanup does.
func (cx *Scope) Cleanup(fn func()) { cx.current().addCleanup(fn) }

func (o *Scope) addCleanup(fn func()) {
	if o.disposed {
		o.rt.guard(o, fn)
		return
	}
	o.cleanups = append(o.cleanups, fn)
}

// Context returns a context cancelled when the current owner is disposed
// or runs again; pass it to work the owner starts on other goroutines.
func (cx *Scope) Context() context.Context { return cx.current().context() }

func (o *Scope) context() context.Context {
	if o.ctx == nil {
		parent := context.Background()
		if o.parent != nil {
			parent = o.parent.context()
		}
		o.ctx, o.cancel = context.WithCancel(parent)
		if o.disposed {
			o.cancel()
		}
	}
	return o.ctx
}

// Post runs fn later on the graph's goroutine, under cx and in one
// batch. It is safe to call from any goroutine; fn does not run if cx is
// disposed first.
func (cx *Scope) Post(fn func()) {
	o, rt := cx, cx.rt
	if rt.post == nil {
		panic("core: Post needs a host: start the scope's graph with an engine or a test driver")
	}
	rt.post(func() {
		if !o.disposed {
			rt.run(o, fn)
		}
	})
}

// AfterFunc runs fn on the graph's goroutine after d, unless the current
// owner is disposed first or stop is called.
func (cx *Scope) AfterFunc(d time.Duration, fn func()) (stop func() bool) {
	o := cx.current()
	stop = cx.rt.clock.AfterFunc(d, func() { o.Post(fn) })
	o.addCleanup(func() { stop() })
	return stop
}

// Untrack runs fn without subscribing the running computation to what
// it reads.
func (cx *Scope) Untrack(fn func()) {
	rt := cx.rt
	prev := rt.observer
	rt.observer = nil
	defer func() { rt.observer = prev }()
	fn()
}

// Untracked returns fn's result without subscribing the running
// computation to what it reads.
func Untracked[T any](cx *Scope, fn func() T) (v T) {
	cx.Untrack(func() { v = fn() })
	return v
}

// Batch runs fn and then the effects its writes scheduled, once each.
func (cx *Scope) Batch(fn func()) {
	rt := cx.rt
	rt.batch++
	defer func() {
		rt.batch--
		if rt.batch == 0 && !rt.flushing {
			rt.flush()
		}
	}()
	fn()
}

// Effect runs fn after the current batch and again whenever what it read
// changes, until the current owner is disposed or stop is called.
func (cx *Scope) Effect(fn func()) (stop func()) {
	c := newComputation(cx.current(), userEffect, func() bool { fn(); return false })
	c.state = dirty
	cx.rt.enqueue(c)
	cx.rt.maybeFlush()
	return c.Dispose
}

// renderEffect runs fn now and again before the app's effects whenever
// what it read changes. Element bindings are render effects.
func (cx *Scope) renderEffect(fn func()) {
	c := newComputation(cx.current(), renderEffect, func() bool { fn(); return false })
	c.run()
}

// run runs fn under owner o, untracked, in a batch, catching panics.
func (rt *runtime) run(o *Scope, fn func()) {
	prevOwner, prevObs := rt.owner, rt.observer
	rt.owner, rt.observer = o, nil
	defer func() { rt.owner, rt.observer = prevOwner, prevObs }()
	rt.batch++
	rt.guard(o, fn)
	rt.batch--
	if rt.batch == 0 && !rt.flushing {
		rt.flush()
	}
}

// guard runs fn and sends a panic to the nearest Catch above o.
func (rt *runtime) guard(o *Scope, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			rt.fail(o, r)
		}
	}()
	fn()
}

// PanicError is a panic recovered from app code.
type PanicError struct {
	Value any
	Stack []byte
}

func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v\n\n%s", e.Value, e.Stack) }

// Unwrap returns the panic's value when it is an error.
func (e *PanicError) Unwrap() error {
	err, _ := e.Value.(error)
	return err
}

func (rt *runtime) fail(o *Scope, r any) {
	err := &PanicError{Value: r, Stack: debug.Stack()}
	for s := o; s != nil; s = s.parent {
		if s.catch != nil && !s.disposed {
			s.catch(err)
			return
		}
	}
	if rt.report != nil {
		rt.report(err)
	}
}

// Key is a context key: a value an owner provides to everything below it.
// Declare one as a package-level variable.
type Key[T any] struct {
	name string
	def  T
}

// NewKey returns a key whose Use returns def where nothing provides one.
func NewKey[T any](name string, def T) *Key[T] { return &Key[T]{name: name, def: def} }

func (k *Key[T]) String() string { return k.name }

// Provide makes v the key's value for the current owner and below.
func (k *Key[T]) Provide(cx *Scope, v T) {
	o := cx.current()
	if o.values == nil {
		o.values = map[any]any{}
	}
	o.values[k] = v
}

// Use returns the value the nearest owner provides, or the default.
func (k *Key[T]) Use(cx *Scope) T {
	for s := cx.current(); s != nil; s = s.parent {
		if v, ok := s.values[k]; ok {
			return v.(T)
		}
	}
	return k.def
}

// Computations.

type compKind uint8

const (
	memoKind compKind = iota
	renderEffect
	userEffect
)

type compState uint8

const (
	clean compState = iota
	check           // a memo it reads may have changed
	dirty           // something it reads changed
)

type source interface {
	unsubscribe(c *computation)
	refresh() // brings a memo up to date
}

type computation struct {
	Scope
	fn      func() (changed bool)
	kind    compKind
	state   compState
	queued  bool
	sources []source
	subs    *subscribers // a memo's readers
}

func newComputation(owner *Scope, kind compKind, fn func() bool) *computation {
	c := &computation{fn: fn, kind: kind}
	c.comp = c
	owner.adopt(&c.Scope)
	return c
}

func (c *computation) unlink() {
	for _, s := range c.sources {
		s.unsubscribe(c)
	}
	c.sources = c.sources[:0]
}

func (c *computation) mark(s compState) {
	if c.state >= s || c.disposed {
		return
	}
	wasClean := c.state == clean
	c.state = s
	if !wasClean {
		return
	}
	if c.subs != nil {
		for _, r := range c.subs.list {
			r.mark(check)
		}
		return
	}
	c.rt.enqueue(c)
}

// update recomputes c if a source really changed.
func (c *computation) update() {
	if c.disposed {
		return
	}
	if c.state == check {
		for _, s := range c.sources {
			s.refresh()
			if c.state == dirty {
				break
			}
		}
	}
	if c.state == dirty {
		c.run()
		return
	}
	c.state = clean
}

// run recomputes c. Writes during the run are batched, and a write to
// something c already read makes it run again.
func (c *computation) run() {
	c.disposeOwned()
	c.unlink()
	c.state = clean
	rt := c.rt
	prevOwner, prevObs := rt.owner, rt.observer
	rt.owner, rt.observer = &c.Scope, c
	rt.batch++
	changed := false
	rt.guard(&c.Scope, func() { changed = c.fn() })
	rt.batch--
	rt.owner, rt.observer = prevOwner, prevObs
	if changed && c.subs != nil {
		for _, r := range c.subs.list {
			r.mark(dirty)
		}
	}
	rt.maybeFlush()
}

func (rt *runtime) enqueue(c *computation) {
	if c.queued {
		return
	}
	c.queued = true
	if c.kind == userEffect {
		rt.user = append(rt.user, c)
	} else {
		rt.render = append(rt.render, c)
	}
}

func (rt *runtime) maybeFlush() {
	if rt.batch == 0 && !rt.flushing {
		rt.flush()
	}
}

func (rt *runtime) flush() {
	rt.flushing = true
	defer func() { rt.flushing = false }()
	for {
		var c *computation
		switch {
		case len(rt.render) > 0:
			c, rt.render = rt.render[0], rt.render[1:]
		case len(rt.user) > 0:
			c, rt.user = rt.user[0], rt.user[1:]
		default:
			rt.render, rt.user = rt.render[:0], rt.user[:0]
			rt.runs = 0
			return
		}
		c.queued = false
		if rt.runs++; rt.runs > maxRuns {
			rt.render, rt.user = nil, nil
			rt.runs = 0
			rt.fail(&c.Scope, fmt.Errorf("core: effects ran %d times without settling", maxRuns))
			return
		}
		c.runTop()
	}
}

// runTop brings c's stale owners up to date first, outermost first, so a
// branch about to be disposed does not run.
func (c *computation) runTop() {
	var chain []*computation
	for s := c.parent; s != nil; s = s.parent {
		if s.comp != nil && s.comp.state != clean {
			chain = append(chain, s.comp)
		}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		chain[i].update()
	}
	c.update()
}

type subscribers struct{ list []*computation }

func (s *subscribers) add(c *computation) { s.list = append(s.list, c) }

func (s *subscribers) remove(c *computation) {
	for i, r := range s.list {
		if r == c {
			last := len(s.list) - 1
			s.list[i] = s.list[last]
			s.list[last] = nil
			s.list = s.list[:last]
			return
		}
	}
}

func (rt *runtime) track(src source, subs *subscribers) {
	c := rt.observer
	if c == nil {
		return
	}
	for _, s := range c.sources {
		if s == src {
			return
		}
	}
	c.sources = append(c.sources, src)
	subs.add(c)
}

// Signal is a value computations subscribe to by reading it.
type Signal[T any] struct {
	rt    *runtime
	value T
	equal func(a, b T) bool
	subs  subscribers
}

// NewSignal returns a signal holding v. Setting an equal value does
// nothing.
func NewSignal[T comparable](cx *Scope, v T) *Signal[T] {
	return &Signal[T]{rt: cx.rt, value: v, equal: func(a, b T) bool { return a == b }}
}

// NewSignalFunc returns a signal holding v that compares values with
// equal; with a nil equal every Set notifies, as slices and maps need.
func NewSignalFunc[T any](cx *Scope, v T, equal func(a, b T) bool) *Signal[T] {
	return &Signal[T]{rt: cx.rt, value: v, equal: equal}
}

// Get returns the value and subscribes the running computation to it.
func (s *Signal[T]) Get() T {
	s.rt.track(s, &s.subs)
	return s.value
}

// GetUntracked returns the value without subscribing.
func (s *Signal[T]) GetUntracked() T { return s.value }

// Set stores v and schedules the computations that read the signal.
func (s *Signal[T]) Set(v T) {
	if s.equal != nil && s.equal(s.value, v) {
		return
	}
	s.value = v
	rt := s.rt
	rt.batch++
	for _, c := range s.subs.list {
		c.mark(dirty)
	}
	rt.batch--
	rt.maybeFlush()
}

// Update sets the value fn returns for the current one.
func (s *Signal[T]) Update(fn func(T) T) { s.Set(fn(s.value)) }

func (s *Signal[T]) unsubscribe(c *computation) { s.subs.remove(c) }
func (s *Signal[T]) refresh()                   {}

// Memo caches a computed value and notifies its readers only when the
// value changes.
type Memo[T any] struct {
	c     *computation
	value T
	equal func(a, b T) bool
}

// NewMemo returns a memo of fn, computed now and again when what fn read
// changes.
func NewMemo[T comparable](cx *Scope, fn func() T) *Memo[T] {
	return newMemo(cx, fn, func(a, b T) bool { return a == b })
}

// NewMemoFunc returns a memo that compares values with equal; with a nil
// equal every computation notifies.
func NewMemoFunc[T any](cx *Scope, fn func() T, equal func(a, b T) bool) *Memo[T] {
	return newMemo(cx, fn, equal)
}

func newMemo[T any](cx *Scope, fn func() T, equal func(a, b T) bool) *Memo[T] {
	m := &Memo[T]{equal: equal}
	first := true
	m.c = newComputation(cx.current(), memoKind, func() bool {
		v := fn()
		if !first && m.equal != nil && m.equal(m.value, v) {
			return false
		}
		first = false
		m.value = v
		return true
	})
	m.c.subs = &subscribers{}
	m.c.run()
	return m
}

// Get returns the value, recomputing it first if needed, and subscribes
// the running computation to it.
func (m *Memo[T]) Get() T {
	m.refresh()
	m.c.rt.track(m, m.c.subs)
	return m.value
}

// GetUntracked returns the up-to-date value without subscribing.
func (m *Memo[T]) GetUntracked() T {
	m.refresh()
	return m.value
}

func (m *Memo[T]) unsubscribe(c *computation) { m.c.subs.remove(c) }
func (m *Memo[T]) refresh() {
	if m.c.state != clean {
		m.c.update()
	}
}
