// Package engine runs an app on a goroutine of its own and hands the
// platform one MountBatch per commit.
package engine

import (
	"log"
	"os"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
)

// SharedMounter is a Platform that applies each batch within Mount, on
// the engine goroutine, as the web does: its batches are the tree's own
// items, not copies (core.Tree.CommitShared).
type SharedMounter interface {
	Platform
	MountsAtOnce()
}

// Platform is what a backend gives the engine.
type Platform interface {
	driver.TextEngine
	// Mount receives each commit on the engine goroutine. The platform
	// applies it on its main thread, in one hop, without waiting.
	Mount(batch *driver.MountBatch)
	// RequestFrame asks for the next display frame. The platform calls
	// Frame when that frame begins, once for each request, and only then.
	// It is called from the engine goroutine, so it must not block. A
	// platform with no display frame leaves it empty, and the app's frame
	// callbacks wait.
	RequestFrame()
}

// observed is a platform whose batches a test host sees first. With a
// clipboard of the test's, the app's clipboard items go to it instead:
// reads are answered there, and the platform never sees them.
type observed struct {
	Platform
	observe   func(*driver.MountBatch)
	clipboard *driver.MemoryClipboard
	dispatch  func(driver.Event)
}

func (o observed) Mount(b *driver.MountBatch) {
	o.observe(b)
	if o.clipboard != nil {
		var kept []driver.MountItem
		for i := range b.Items {
			ev, answer, taken := o.clipboard.Take(&b.Items[i])
			switch {
			case answer:
				o.dispatch(ev)
			case !taken:
				kept = append(kept, b.Items[i])
			}
		}
		if len(kept) != len(b.Items) {
			// The test reads the batch it observed: the platform gets a
			// copy.
			c := *b
			c.Items = kept
			b = &c
		}
	}
	o.Platform.Mount(b)
}

// App builds the root's content once.
type App = func(cx *core.Scope) core.Node

// Options configure an engine.
type Options struct {
	// Size is the surface's size at start.
	Size driver.Size
	// Environment is what the platform knows at start.
	Environment driver.Environment
	// Clock runs the app's timers; the system's when nil.
	Clock core.Clock
	// Storage keeps the app's entries; in memory for the engine's life
	// when nil. Platforms pass OpenFileStorage in the data directory.
	Storage *core.Store
	// DataDir and CacheDir are the app's directories, made when the
	// engine starts; "" where the target has none.
	DataDir, CacheDir string
	// Appearance is the appearance the app starts in, forced as
	// Scope.SetAppearance forces it; the system's by default.
	Appearance driver.Appearance
	// Host is what an embedding host gives the view the app runs in.
	Host HostOptions
	// Windows, set where the platform opens windows, are the main
	// window's options (core.RootOptions.Windows): the first surface is
	// the app's own, which shows no view, and the platform attaches each
	// surface the app opens.
	Windows *WindowOptions
	// Perf collects this app's commit timings; nil measures nothing.
	Perf *PerfMonitor
	// Thread runs first on the OS thread the engine keeps, before the
	// app. The engine turns input into frames, as latency-critical as the
	// main thread: a platform whose scheduler would put it on slower
	// cores raises its priority here.
	Thread func()
	// Observe sees each batch before the platform does, and Event each
	// event the platform reports, as a test host follows the app; Pin
	// turns each environment the platform reports into the app's. None
	// may block.
	Observe func(*driver.MountBatch)
	Event   func(driver.Event)
	Pin     func(driver.Environment) driver.Environment
	// Clipboard, with Observe, is the clipboard of a test's own, which
	// the app's clipboard commands use instead of the platform's.
	Clipboard *driver.MemoryClipboard
	// LogPerf logs Perf's readings once a second while the engine runs.
	LogPerf bool
}

// Host, Tokens and WindowOptions are core's, for backends.
type (
	WindowOptions = core.WindowOptions
	Host          = core.Host
	HostCall      = core.HostCall
	Tokens        = core.Tokens
)

// NewHost returns a new channel to an embedding host (core.NewHost).
func NewHost() *Host { return core.NewHost() }

// HostOptions are what an embedding host gives a view's engine.
type HostOptions struct {
	// Channel is the channel between the host and the app
	// (core.RootOptions.Host).
	Channel *Host
	// Settings are the host's settings of the view.
	Settings *HostSettings
	// OnError hears each failure the app does not catch, after the log,
	// on the engine goroutine; it must not block.
	OnError func(error)
	// DataDir is the app's data directory, its storage's, instead of the
	// system's for the app's ID.
	DataDir string
}

// HostSettings are an embedding host's settings of one view: kept until
// the view's engine starts, which has them from its first frame, and sent
// to it after. Only the platform's main thread uses them.
type HostSettings struct {
	Environment HostEnvironment
	Tokens      Tokens
	engine      *Engine
}

// SetEnvironment sets what the host sets over the platform's environment.
func (h *HostSettings) SetEnvironment(env HostEnvironment) {
	h.Environment = env
	if h.engine != nil {
		h.engine.SetHostEnvironment(env)
	}
}

// SetTokens sets the values the host themes the app with.
func (h *HostSettings) SetTokens(t Tokens) {
	h.Tokens = t
	if h.engine != nil {
		h.engine.SetTokens(t)
	}
}

// HostEnvironment is what an embedding host sets over the platform's
// environment for its view; zero values follow the platform's.
type HostEnvironment struct {
	// Locale is a BCP 47 tag ("de-CH") the app formats for instead of
	// the system's language; the layout direction stays the system's.
	Locale string
	// FontScale is the text size multiplier, standing in for the
	// system's.
	FontScale float32
	// Appearance light or dark stands in for the system's; an appearance
	// the app forces still wins over it.
	Appearance driver.Appearance
}

// over returns env with what h sets.
func (h HostEnvironment) over(env driver.Environment) driver.Environment {
	if h.Locale != "" {
		env.Locale = h.Locale
	}
	if h.FontScale > 0 {
		env.FontScale = h.FontScale
	}
	switch h.Appearance {
	case driver.AppearanceLight:
		env.Dark = false
	case driver.AppearanceDark:
		env.Dark = true
	}
	return env
}

// Engine is one running app: one reactive graph, one goroutine, and a
// tree per surface. An Engine value is one surface's: Start returns the
// first's, and Attach another's, which the app opened. Its methods are
// safe to call from any goroutine and never block, so a platform's main
// thread can call them.
type Engine struct {
	*runner
	surface uint32
}

// runner is what an app's surfaces share.
type runner struct {
	mu     sync.Mutex
	event  func(driver.Event)
	pin    func(driver.Environment) driver.Environment
	raw    driver.Environment // the first surface's last, before pinning
	host   HostEnvironment
	queue  []message
	wake   chan struct{}
	done   chan struct{}
	exited chan struct{}
	once   sync.Once
}

type message struct {
	// surface is the surface a resize, an environment, a press or an
	// event is for.
	surface uint32
	frame   bool
	event   driver.Event
	resize  *driver.Size
	// resized are closed once the commit after the resize is mounted.
	resized []chan struct{}
	env     *driver.Environment
	at      *driver.Event // a press to hit test
	tokens  *core.Tokens
	fn      func()
	fonts   bool
	// attach is the platform of an opened surface; detach ends it.
	attach Platform
	detach bool
}

// Trace, when set, hears each commit's batch, before the platform, and
// how long the commit took, on the engine goroutine of every engine the
// process starts from then on; it must not block. nsgo dev -trace's
// builds set it, as their dev client starts.
var Trace func(b *driver.MountBatch, took time.Duration)

// Start runs app on a new goroutine, its first surface on p.
func Start(p Platform, app App, opts Options) *Engine {
	e := &Engine{runner: &runner{
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
		exited: make(chan struct{}),
		event:  opts.Event,
		pin:    opts.Pin,
	}}
	var tokens Tokens
	if hs := opts.Host.Settings; hs != nil {
		e.host, tokens = hs.Environment, hs.Tokens
		hs.engine = e
	}
	if dir := opts.Host.DataDir; dir != "" && opts.Observe == nil {
		opts.DataDir = dir
		opts.Storage = OpenFileStorage(dir)
	}
	e.raw = opts.Environment
	opts.Environment = e.effective(opts.Environment)
	if observe := opts.Observe; observe != nil {
		p = observed{p, observe, opts.Clipboard, e.Dispatch}
	}
	go e.run(p, app, opts, tokens)
	if opts.LogPerf && opts.Perf != nil {
		go e.logPerf(opts.Perf)
	}
	return e
}

// logPerf logs m's readings once a second until the engine closes.
func (e *Engine) logPerf(m *PerfMonitor) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-e.done:
			return
		case <-tick.C:
			log.Print("nsgo: perf ", m.Snapshot())
		}
	}
}

// Attach attaches platform p to the surface the app opened as id
// (driver.ItemOpenSurface) and returns the surface's Engine, which p
// reports to as the first surface's platform does: its size and
// environment first. Closing it detaches p.
func (e *Engine) Attach(id uint32, p Platform) *Engine {
	e.push(message{surface: id, attach: p})
	return &Engine{runner: e.runner, surface: id}
}

// surface is an attached surface: its tree and platform.
type surface struct {
	tree   *core.Tree
	p      Platform
	commit func() *driver.MountBatch
}

// destroyed is the batch that tells an opened surface's platform that its
// tree is gone.
func destroyed() *driver.MountBatch {
	return &driver.MountBatch{Items: []driver.MountItem{{Kind: driver.ItemWindow,
		Window: &driver.WindowCommand{Action: driver.WindowDestroy}}}}
}

func (e *Engine) run(p Platform, app App, opts Options, tokens Tokens) {
	defer close(e.exited)
	// Text engines such as Android's need the same OS thread each call.
	runtime.LockOSThread()
	if opts.Thread != nil {
		opts.Thread()
	}
	for _, dir := range []string{opts.DataDir, opts.CacheDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("nsgo: the app's directory could not be made: %v", err)
		}
	}
	// surfaces are the attached ones by ID, the first's 0; frameAsked is
	// set while a display frame was asked for.
	surfaces := map[uint32]*surface{}
	frameAsked := false
	requestFrame := func() {
		frameAsked = true
		// A window's display ticks; the first surface may have none.
		for _, s := range surfaces {
			if s.tree.ID() != 0 {
				s.p.RequestFrame()
				return
			}
		}
		p.RequestFrame()
	}
	root := core.NewRoot(core.RootOptions{
		Post:  func(fn func()) { e.push(message{fn: fn}) },
		Clock: opts.Clock,
		Report: func(err error) {
			log.Printf("nsgo: the app failed: %v", err)
			if opts.Host.OnError != nil {
				opts.Host.OnError(err)
			}
		},
		// The platform owns the display frame: the engine only asks.
		RequestFrame: requestFrame,
		Storage:      opts.Storage,
		DataDir:      opts.DataDir,
		CacheDir:     opts.CacheDir,
		Host:         opts.Host.Channel,
		Windows:      opts.Windows,
	})
	defer root.Dispose()
	tree := core.NewTree(root, p)
	tree.Resize(opts.Size)
	tree.SetEnvironment(opts.Environment)
	tree.SetAppearance(opts.Appearance)
	tree.SetTokens(tokens)
	perf, trace := opts.Perf, Trace
	// attach makes t's surface p's. A test host's observer may keep a
	// batch past the next commit.
	attach := func(t *core.Tree, p Platform) *surface {
		s := &surface{tree: t, p: p, commit: t.Commit}
		if _, ok := p.(SharedMounter); ok && opts.Observe == nil {
			s.commit = t.CommitShared
		}
		surfaces[t.ID()] = s
		return s
	}
	attach(tree, p)
	// commitOne applies what changed in s and records what it cost when a
	// monitor measures or a trace hears; start is when the wake-up that
	// led here began.
	commitOne := func(s *surface, start time.Time) {
		if perf == nil && trace == nil {
			if b := s.commit(); b != nil {
				s.p.Mount(b)
			}
			return
		}
		coreStart := time.Now()
		b := s.commit()
		core := time.Since(coreStart)
		if b != nil && trace != nil {
			trace(b, core)
		}
		if perf == nil {
			if b != nil {
				s.p.Mount(b)
			}
			return
		}
		if b != nil {
			app := time.Since(start) - core
			if app < 0 {
				app = 0
			}
			b.Sent = time.Now()
			perf.RecordCommit(CommitTiming{App: app, Core: core, Items: len(b.Items)})
			s.p.Mount(b)
		}
	}
	// commit commits every attached surface, the first first, and ends
	// the opened ones that closed.
	commit := func(start time.Time) {
		commitOne(surfaces[0], start)
		for _, t := range slices.Clone(tree.Surfaces()) {
			s := surfaces[t.ID()]
			switch {
			case t.Closed():
				t.Forget()
				if s != nil {
					delete(surfaces, t.ID())
					s.p.Mount(destroyed())
				}
			case s != nil:
				commitOne(s, start)
			}
		}
		if perf != nil {
			perf.RecordBusy(time.Since(start))
		}
	}
	var start time.Time
	if perf != nil {
		start = time.Now()
	}
	tree.Mount(app)
	commit(start)
	for {
		select {
		case <-e.wake:
		case <-e.done:
			return
		}
		if perf != nil {
			start = time.Now()
		}
		var resized []chan struct{}
		for _, m := range e.take() {
			switch {
			case m.frame:
				frameAsked = false
				root.RunFrame()
				continue
			case m.fn != nil:
				m.fn()
				continue
			case m.tokens != nil:
				tree.SetTokens(*m.tokens)
				continue
			case m.fonts:
				tree.FontsChanged()
				for _, t := range tree.Surfaces() {
					t.FontsChanged()
				}
				continue
			case m.attach != nil:
				i := slices.IndexFunc(tree.Surfaces(), func(t *core.Tree) bool { return t.ID() == m.surface })
				if i < 0 || tree.Surfaces()[i].Closed() {
					// It closed before its platform attached it.
					m.attach.Mount(destroyed())
					continue
				}
				attach(tree.Surfaces()[i], m.attach)
				if frameAsked {
					m.attach.RequestFrame()
				}
				continue
			}
			s := surfaces[m.surface]
			if s == nil {
				continue
			}
			t := s.tree
			switch {
			case m.detach:
				if m.surface != 0 {
					delete(surfaces, m.surface)
				}
			case m.resize != nil:
				t.Resize(*m.resize)
				resized = append(resized, m.resized...)
			case m.env != nil:
				t.SetEnvironment(*m.env)
			case m.at != nil:
				t.PressAt(*m.at)
			default:
				t.Dispatch(m.event)
			}
		}
		commit(start)
		for _, done := range resized {
			close(done)
		}
	}
}

// push queues m and wakes the engine. A mergeable event replaces one of
// the same kind for the same view still queued, a resize the last resize
// of the same surface, and a frame request the one still queued, so
// bursts cost one update each. A wheel's turns add up into the one queued.
func (e *Engine) push(m message) {
	e.mu.Lock()
	merged := false
	for i := range e.queue {
		q := &e.queue[i]
		same := q.surface == m.surface
		switch {
		case m.resize != nil && q.resize != nil && same:
			// The commit at the newest size answers the waits for both.
			m.resized = append(q.resized, m.resized...)
			*q = m
			merged = true
		case same && m.event.Kind == driver.EventWheel && q.event.Kind == driver.EventWheel && q.event.ID == m.event.ID &&
			m.event.Phase == driver.GestureChanged && q.event.Phase == driver.GestureChanged &&
			m.event.Precise == q.event.Precise && m.event.Modifiers == q.event.Modifiers:
			// Turns add up, where other moves replace each other.
			m.event.Delta.X += q.event.Delta.X
			m.event.Delta.Y += q.event.Delta.Y
			*q = m
			merged = true
		case m.frame && q.frame,
			m.env != nil && q.env != nil && same,
			m.tokens != nil && q.tokens != nil,
			m.fonts && q.fonts,
			same && m.event.Mergeable() && q.event.Mergeable() && q.event.Kind == m.event.Kind && q.event.ID == m.event.ID:
			*q = m
			merged = true
		}
		if merged {
			break
		}
	}
	if !merged {
		e.queue = append(e.queue, m)
	}
	e.mu.Unlock()
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *Engine) take() []message {
	e.mu.Lock()
	defer e.mu.Unlock()
	q := e.queue
	e.queue = nil
	return q
}

// Dispatch reports a platform event.
func (e *Engine) Dispatch(ev driver.Event) {
	if e.event != nil && e.surface == 0 {
		e.event(ev)
	}
	e.push(message{surface: e.surface, event: ev})
}

// Sync returns once the engine ran what was posted to it before, as a
// test host waits for the timers a step of its clock fired.
func (e *Engine) Sync() {
	done := make(chan struct{})
	e.push(message{fn: func() { close(done) }})
	select {
	case <-done:
	case <-e.exited:
	}
}

// Frame reports that the display frame the app asked for has begun. The
// engine runs the callbacks waiting for it on its goroutine, then commits.
func (e *Engine) Frame() { e.push(message{frame: true}) }

// Resize sets the surface size.
func (e *Engine) Resize(size driver.Size) { e.push(message{surface: e.surface, resize: &size}) }

// ResizeAndWait sets the surface size and returns a channel closed once
// the commit laid out at it is mounted: a platform that shows a new size
// at once waits on it, briefly.
func (e *Engine) ResizeAndWait(size driver.Size) <-chan struct{} {
	done := make(chan struct{})
	e.push(message{surface: e.surface, resize: &size, resized: []chan struct{}{done}})
	return done
}

// SetEnvironment reports what the platform knows about where the app
// runs; only the latest report queued is applied.
func (e *Engine) SetEnvironment(env driver.Environment) {
	if e.surface == 0 {
		e.mu.Lock()
		e.raw = env
		e.mu.Unlock()
	}
	env = e.effective(env)
	e.push(message{surface: e.surface, env: &env})
}

// effective is the environment the app sees for the platform's env: the
// host's settings over it, then a test's pin.
func (e *Engine) effective(env driver.Environment) driver.Environment {
	e.mu.Lock()
	env = e.host.over(env)
	e.mu.Unlock()
	if e.pin != nil {
		env = e.pin(env)
	}
	return env
}

// Repin reports the platform's last environment again, for a pin that
// changed.
func (e *Engine) Repin() {
	e.mu.Lock()
	env := e.raw
	e.mu.Unlock()
	e.SetEnvironment(env)
}

// SetHostEnvironment sets what an embedding host sets over the
// platform's environment, from now on.
func (e *Engine) SetHostEnvironment(h HostEnvironment) {
	e.mu.Lock()
	e.host = h
	e.mu.Unlock()
	e.Repin()
}

// SetTokens gives the app an embedding host's tokens (core.Tokens).
func (e *Engine) SetTokens(t core.Tokens) { e.push(message{tokens: &t}) }

// PressAt reports a press at ev.Page in surface points, with its
// Modifiers and click Count, for platforms that draw their own views and
// leave hit testing to the engine.
func (e *Engine) PressAt(ev driver.Event) { e.push(message{surface: e.surface, at: &ev}) }

// FontsChanged prepares the app's text again, for fonts the platform
// loaded after they were registered (driver.FontRegistrar).
func (e *Engine) FontsChanged() { e.push(message{fonts: true}) }

// Close stops the app, disposing its graph, and waits for the engine
// goroutine to exit; an opened surface's Engine only detaches its
// platform. It must not be called from the engine goroutine.
func (e *Engine) Close() {
	if e.surface != 0 {
		e.push(message{surface: e.surface, detach: true})
		return
	}
	e.once.Do(func() { close(e.done) })
	<-e.exited
}
