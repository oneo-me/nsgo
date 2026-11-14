package core

import (
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"sync/atomic"
	"time"

	"github.com/nsgo-dev/nsgo/internal/devtools"
	"github.com/nsgo-dev/nsgo/internal/driver"
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
	// surf is the tree of the window the scope is in, inherited by what
	// it owns; nil leaves it the graph's first.
	surf *Tree
	// park, set on a list cell's scope, holds back what runs under it while the cell is spare.
	park     *parking
	disposed bool
}

// parking holds back the computations under a list cell's scope while the cell is spare, as its index may name a row that is gone or another row's, and runs them when the cell shows a row again: its effects, and its memos read from outside it, for their readers. guard, the list's computation that frees and binds its cells, is brought up to date before anything under the scope runs, so a row's binding never runs with an index its list has not settled.
type parking struct {
	guard    *computation
	parked   bool
	deferred []*computation
}

// park holds back what runs under p's scope.
func (rt *runtime) park(p *parking) {
	if !p.parked {
		p.parked = true
		rt.parked++
	}
}

// unpark lets what p held back run, in the running batch or the next.
func (rt *runtime) unpark(p *parking) {
	if p.parked {
		p.parked = false
		rt.parked--
	}
	for _, c := range p.deferred {
		if !c.disposed && c.state != clean {
			rt.enqueue(c)
		}
	}
	clear(p.deferred)
	p.deferred = p.deferred[:0]
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
	// RequestFrame tells the host that the app waits for the next display
	// frame: it calls Scope.RunFrame when that frame begins, and is not
	// asked again until then. It runs on the graph's goroutine, so it must
	// not block. A host without display frames leaves it nil, and the
	// callbacks wait.
	RequestFrame func()
	// Storage keeps the app's entries; a store in memory when nil.
	Storage *Store
	// DataDir and CacheDir are the app's directories, "" where the target
	// has none.
	DataDir, CacheDir string
	// Host is the channel to the host app that embeds the app: its
	// parameters, events, handlers and URL handler. The graph attaches to
	// it until its root is disposed.
	Host *Host
	// Windows, set where the platform opens windows, are the main
	// window's options (SetMainView): the first tree is then the app's
	// own surface, which shows no view.
	Windows *WindowOptions
}

// NewRoot returns the root scope of a new graph.
func NewRoot(opts RootOptions) *Scope {
	if opts.Clock == nil {
		opts.Clock = SystemClock{}
	}
	rt := &runtime{post: opts.Post, clock: opts.Clock, report: opts.Report,
		store: opts.Storage, dataDir: opts.DataDir, cacheDir: opts.CacheDir,
		requestFrame: opts.RequestFrame, windows: opts.Windows}
	rt.root = &Scope{rt: rt}
	if devtools.Enabled {
		rt.dev = newDevState()
	}
	if opts.Host != nil {
		rt.attachHost(opts.Host)
	}
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
	// renderAt and userAt are the next in each queue flush runs.
	renderAt, userAt int
	runs             int
	post             func(func())
	clock            Clock
	report           func(error)

	// The callbacks waiting for the next display frame, and whether the
	// host has been asked for it.
	frames         []*frameCallback
	frameRequested bool
	requestFrame   func()
	env            *Env
	tree           *Tree
	root           *Scope
	store          *Store
	storage        *storageWatch
	dataDir        string
	cacheDir       string
	host           *hostApp
	// restricted says an owner was given an Access (ProvideAccess), so
	// requests look for one.
	restricted bool
	// storageScoped says an owner was given a partition (ProvideStorage).
	storageScoped bool
	// hostScoped says an owner was given a host (ProvideHost).
	hostScoped bool
	// parked counts the parked scopes, so a memo looks for one above it only while there are any.
	parked int
	// refusals are the capabilities a refusal was logged of.
	refusals map[string]bool
	// speech is the speech session running, one per engine.
	speech *SpeechSession
	// causes are what ran since a tree last committed, while driver.TraceCauses is set.
	causes causes
	// dev is what DevTools read, while devtools.Enabled.
	dev *devState
	// quits are the OnQuit handlers, oldest first.
	quits []*quitHandler
	// quitRequests are the handlers for user-initiated quit requests.
	quitRequests []*quitRequestHandler
	// notificationPresses hear notifications pressed, and pressesPending
	// keeps those pressed before any did.
	notificationPresses []*notificationPressHandler
	pressesPending      []string
	// windows are the main window's options where the platform opens
	// windows (RootOptions.Windows); surfaces the trees of the windows,
	// panels and tray views opened, keyed those opened with a key.
	windows   *WindowOptions
	surfaces  []*Tree
	surfaceID uint32
	keyed     map[string]*Window
	// shutdown says when the app ends; main is its main window, ownMain
	// the one SetMainView opened to show mainView.
	shutdown      ShutdownMode
	main, ownMain *Window
	mainView      *Signal[func(cx *Scope) Node]
	reopens       []*reopenHandler
	// opens hear the files the system opens with the app; openPending
	// are those that came before any did.
	opens       []*openFilesHandler
	openPending []string
	trays       map[uint32]*Tray
	trayID      uint32
	hotKeys     []*hotKey
	hotKeyID    uint32
	// theme is DefaultTheme's, made on first use; subtreeAppearance
	// whether any scope has forced one.
	theme             *Theme
	subtreeAppearance bool
}

// tree is the tree of the surface cx is in: its window's, or the
// graph's first.
func (cx *Scope) tree() *Tree {
	if cx.surf != nil {
		return cx.surf
	}
	return cx.rt.tree
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
	if child.surf == nil {
		child.surf = cx.surf
	}
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
	if cx.rt.dev != nil {
		cx.rt.dev.forget(cx)
	}
	if cx == cx.rt.root {
		cx.rt.detachHost()
	}
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
	if cx.rt != nil && cx.rt.dev != nil {
		cx.rt.dev.forget(cx)
	}
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
	// The call is posted when the time comes: stopping it before it runs
	// cancels it there too, as a timer an earlier one clears.
	var stopped, ran atomic.Bool
	clockStop := cx.rt.clock.AfterFunc(d, func() {
		o.Post(func() {
			if !stopped.Load() {
				ran.Store(true)
				fn()
			}
		})
	})
	stop = func() bool {
		clockStop()
		return !stopped.Swap(true) && !ran.Load()
	}
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

// PauseTracking stops subscribing the running computation to what is
// read until resume is called: for code that runs within a computation
// without belonging to it, as a promise's continuation the tsx package's
// VM runs as a JS call returns.
func (cx *Scope) PauseTracking() (resume func()) {
	rt := cx.rt
	prev := rt.observer
	rt.observer = nil
	return func() { rt.observer = prev }
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
	c := newComputation(cx.current(), userEffect, nil)
	c.effect = fn
	c.state = dirty
	cx.rt.enqueue(c)
	cx.rt.maybeFlush()
	return c.Dispose
}

// renderEffect runs fn now and again before the app's effects whenever
// what it read changes. Element bindings are render effects.
func (cx *Scope) renderEffect(fn func()) {
	c := newComputation(cx.current(), renderEffect, nil)
	c.effect = fn
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
	rt.raise(o, &PanicError{Value: r, Stack: debug.Stack()})
}

// raise sends err to the nearest Catch above o, or to the host's Report without one.
func (rt *runtime) raise(o *Scope, err error) {
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
	readers() *subscribers
	refresh() // brings a memo up to date
}

type computation struct {
	Scope
	fn     func() (changed bool)
	effect func() // an effect's, which changes nothing it is read for, in place of fn
	kind   compKind
	state  compState
	queued bool
	// sources are what c read, each with c's index in the source's
	// readers, so unlinking takes constant time however many read it.
	sources []sourceLink
	subs    *subscribers // a memo's readers
}

func newComputation(owner *Scope, kind compKind, fn func() bool) *computation {
	c := &computation{fn: fn, kind: kind}
	c.comp = c
	owner.adopt(&c.Scope)
	if c.rt.dev != nil {
		c.rt.dev.computation(c)
	}
	return c
}

func (c *computation) unlink() {
	for _, s := range c.sources {
		s.src.readers().remove(s.at)
	}
	clear(c.sources)
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
			r.c.mark(check)
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
			s.src.refresh()
			if c.state == dirty {
				break
			}
		}
	}
	if c.state == dirty {
		// A memo in a spare cell, read from outside it, keeps its value until the cell shows a row again, and runs then for its readers.
		if c.subs != nil && c.rt.parked > 0 {
			if p := c.parent.heldBack(); p != nil {
				if !slices.Contains(p.deferred, c) {
					p.deferred = append(p.deferred, c)
				}
				return
			}
		}
		c.run()
		return
	}
	c.state = clean
}

// heldBack returns the parking of s or the nearest owner above it that is parked, or nil.
func (s *Scope) heldBack() *parking {
	for ; s != nil; s = s.parent {
		if s.park != nil && s.park.parked {
			return s.park
		}
	}
	return nil
}

// run recomputes c. Writes during the run are batched, and a write to
// something c already read makes it run again.
func (c *computation) run() {
	if driver.TraceCauses {
		c.rt.noteRun(c.kind)
	}
	var began time.Time
	if c.rt.dev != nil {
		began = time.Now()
	}
	c.disposeOwned()
	c.unlink()
	c.state = clean
	rt := c.rt
	prevOwner, prevObs := rt.owner, rt.observer
	rt.owner, rt.observer = &c.Scope, c
	rt.batch++
	changed := false
	rt.guard(&c.Scope, func() {
		if c.effect != nil {
			c.effect()
		} else {
			changed = c.fn()
		}
	})
	// Its own time: what its writes run next, in maybeFlush, is theirs.
	if rt.dev != nil {
		rt.dev.ran(c, time.Since(began))
	}
	rt.batch--
	rt.owner, rt.observer = prevOwner, prevObs
	if changed && c.subs != nil {
		for _, r := range c.subs.list {
			r.c.mark(dirty)
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
		// Taken by index, so the queues keep what they grew to.
		switch {
		case rt.renderAt < len(rt.render):
			c = rt.render[rt.renderAt]
			rt.render[rt.renderAt] = nil
			rt.renderAt++
		case rt.userAt < len(rt.user):
			c = rt.user[rt.userAt]
			rt.user[rt.userAt] = nil
			rt.userAt++
		default:
			rt.render, rt.user, rt.renderAt, rt.userAt = rt.render[:0], rt.user[:0], 0, 0
			rt.runs = 0
			return
		}
		c.queued = false
		if rt.runs++; rt.runs > maxRuns {
			rt.render, rt.user, rt.renderAt, rt.userAt = nil, nil, 0, 0
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
	slow := false
	for s := c.parent; s != nil && !slow; s = s.parent {
		slow = s.comp != nil && s.comp.state != clean || s.park != nil && (s.park.parked || s.park.guard != nil && s.park.guard.state != clean)
	}
	if !slow {
		c.update()
		return
	}
	// Outermost first: stale owners run before what they own, and a list's guard before its cells. A computation in a cell the guard leaves spare waits with the cell.
	var path []*Scope
	for s := c.parent; s != nil; s = s.parent {
		if s.comp != nil && s.comp.state != clean || s.park != nil {
			path = append(path, s)
		}
	}
	for i := len(path) - 1; i >= 0; i-- {
		s := path[i]
		if p := s.park; p != nil {
			if p.guard != nil && p.guard.state != clean {
				p.guard.update()
			}
			if p.parked {
				if !c.disposed && c.state != clean {
					p.deferred = append(p.deferred, c)
				}
				return
			}
		}
		if s.comp != nil && s.comp.state != clean {
			s.comp.update()
		}
	}
	c.update()
}

// subscribers are a source's readers, each with the source's index in
// the reader's sources.
type subscribers struct {
	list []readerLink
}

// sourceLink is a source a computation read and its index in the
// source's readers; readerLink a computation reading a source and the
// source's index in the computation's sources. One slice of pairs a side
// keeps the graph to half the objects of a slice of each.
type (
	sourceLink struct {
		src source
		at  int
	}
	readerLink struct {
		c  *computation
		at int
	}
)

// remove takes out the reader at i, moving the last one into its place.
func (s *subscribers) remove(i int) {
	last := len(s.list) - 1
	if i != last {
		moved := s.list[last]
		s.list[i] = moved
		moved.c.sources[moved.at].at = i
	}
	s.list[last] = readerLink{}
	s.list = s.list[:last]
}

func (rt *runtime) track(src source, subs *subscribers) {
	c := rt.observer
	if c == nil {
		return
	}
	for _, s := range c.sources {
		if s.src == src {
			return
		}
	}
	c.sources = append(c.sources, sourceLink{src, len(subs.list)})
	subs.list = append(subs.list, readerLink{c, len(c.sources) - 1})
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
	s := &Signal[T]{rt: cx.rt, value: v, equal: func(a, b T) bool { return a == b }}
	if cx.rt.dev != nil {
		devSignal(cx.devOwner(), s)
	}
	return s
}

// NewSignalFunc returns a signal holding v that compares values with
// equal; with a nil equal every Set notifies, as slices and maps need.
func NewSignalFunc[T any](cx *Scope, v T, equal func(a, b T) bool) *Signal[T] {
	s := &Signal[T]{rt: cx.rt, value: v, equal: equal}
	if cx.rt.dev != nil {
		devSignal(cx.devOwner(), s)
	}
	return s
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
	if driver.TraceCauses {
		rt.noteWrite()
	}
	rt.batch++
	for _, r := range s.subs.list {
		r.c.mark(dirty)
	}
	rt.batch--
	rt.maybeFlush()
}

// Update sets the value fn returns for the current one.
func (s *Signal[T]) Update(fn func(T) T) { s.Set(fn(s.value)) }

func (s *Signal[T]) readers() *subscribers { return &s.subs }
func (s *Signal[T]) refresh()              {}

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
	if d := m.c.rt.dev; d != nil {
		d.memo(m.c, func() string { return fmt.Sprint(any(m.value)) })
	}
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

func (m *Memo[T]) readers() *subscribers { return m.c.subs }
func (m *Memo[T]) refresh() {
	if m.c.state != clean {
		m.c.update()
	}
}
