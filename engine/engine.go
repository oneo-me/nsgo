// Package engine runs an app on a goroutine of its own and hands the
// platform one MountBatch per commit.
package engine

import (
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/oneo-me/nsgo/core"
)

// Platform is what a backend gives the engine.
type Platform interface {
	core.TextEngine
	// Mount receives each commit on the engine goroutine. The platform
	// applies it on its main thread, in one hop, without waiting.
	Mount(batch *core.MountBatch)
	// Animate starts or stops display frames. The engine turns it on while
	// someone is registered for frames and off when the last one leaves,
	// so an idle app holds no display link and takes no wakeups.
	Animate(on bool)
}

// App builds the root's content once.
type App = func(cx *core.Scope) core.Node

// Options configure an engine.
type Options struct {
	// Size is the surface's size at start.
	Size core.Size
	// Environment is what the platform knows at start.
	Environment core.Environment
	// Clock runs the app's timers; the system's when nil.
	Clock core.Clock
	// Logger receives failures the app does not catch; slog's default
	// when nil.
	Logger *slog.Logger
	// Storage keeps the app's entries; in memory for the engine's life
	// when nil. Platforms pass OpenFileStorage in the data directory.
	Storage *core.Store
	// DataDir and CacheDir are the app's directories, made when the
	// engine starts; "" where the target has none.
	DataDir, CacheDir string
}

// Engine is one running app: one tree, one reactive graph, one goroutine.
// Its methods are safe to call from any goroutine and never block, so a
// platform's main thread can call them.
type Engine struct {
	mu    sync.Mutex
	queue []message
	// wantsFrame mirrors the last Animate call, for platforms that build
	// their display link late.
	wantsFrame atomic.Bool
	wake       chan struct{}
	done       chan struct{}
	exited     chan struct{}
	once       sync.Once
}

type message struct {
	frame  bool
	event  core.Event
	resize *core.Size
	env    *core.Environment
	at     *core.Point
	fn     func()
}

// Start runs app on a new goroutine.
func Start(p Platform, app App, opts Options) *Engine {
	e := &Engine{
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
		exited: make(chan struct{}),
	}
	go e.run(p, app, opts)
	return e
}

func (e *Engine) run(p Platform, app App, opts Options) {
	defer close(e.exited)
	// Text engines such as Android's need the same OS thread each call.
	runtime.LockOSThread()
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	for _, dir := range []string{opts.DataDir, opts.CacheDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			logger.Error("nsgo: the app's directory could not be made", "err", err)
		}
	}
	root := core.NewRoot(core.RootOptions{
		Post:  func(fn func()) { e.push(message{fn: fn}) },
		Clock: opts.Clock,
		Report: func(err error) {
			logger.Error("nsgo: the app failed", "err", err)
		},
		Storage:  opts.Storage,
		DataDir:  opts.DataDir,
		CacheDir: opts.CacheDir,
		// Called from any goroutine: the platform owns the display link.
		Animate: func(on bool) {
			e.wantsFrame.Store(on)
			p.Animate(on)
		},
	})
	defer root.Dispose()
	tree := core.NewTree(root, p)
	tree.Resize(opts.Size)
	tree.SetEnvironment(opts.Environment)
	tree.Mount(app)
	for {
		if b := tree.Commit(); b != nil {
			p.Mount(b)
		}
		select {
		case <-e.wake:
		case <-e.done:
			return
		}
		for _, m := range e.take() {
			switch {
			case m.frame:
				root.RunFrames()
			case m.fn != nil:
				m.fn()
			case m.resize != nil:
				tree.Resize(*m.resize)
			case m.env != nil:
				tree.SetEnvironment(*m.env)
			case m.at != nil:
				if id := tree.HitTest(*m.at); id != 0 {
					tree.Dispatch(core.Event{Kind: core.EventPress, ID: id})
				}
			default:
				tree.Dispatch(m.event)
			}
		}
	}
}

// push queues m and wakes the engine. A mergeable event replaces one of
// the same kind for the same view still queued, and a resize the last
// resize, so bursts cost one update each.
func (e *Engine) push(m message) {
	e.mu.Lock()
	merged := false
	for i := range e.queue {
		q := &e.queue[i]
		switch {
		case m.resize != nil && q.resize != nil,
			m.env != nil && q.env != nil,
			m.event.Mergeable() && q.event.Mergeable() && q.event.Kind == m.event.Kind && q.event.ID == m.event.ID:
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
func (e *Engine) Dispatch(ev core.Event) { e.push(message{event: ev}) }

// WantsFrames reports whether the app asked for display frames. A platform
// that builds its link lazily asks this when the first commit arrives.
func (e *Engine) WantsFrames() bool { return e.wantsFrame.Load() }

// Frame reports a display frame from the platform's display link. It runs
// the app's frame callbacks on the engine goroutine and commits once.
func (e *Engine) Frame() { e.push(message{frame: true}) }

// Resize sets the surface size.
func (e *Engine) Resize(size core.Size) { e.push(message{resize: &size}) }

// SetEnvironment reports what the platform knows about where the app
// runs; only the latest report queued is applied.
func (e *Engine) SetEnvironment(env core.Environment) { e.push(message{env: &env}) }

// PressAt reports a press at p in surface points, for platforms that
// draw their own views and leave hit testing to the engine.
func (e *Engine) PressAt(p core.Point) { e.push(message{at: &p}) }

// Close stops the app, disposing its graph, and waits for the engine
// goroutine to exit. It must not be called from the engine goroutine.
func (e *Engine) Close() {
	e.once.Do(func() { close(e.done) })
	<-e.exited
}
