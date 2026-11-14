package core

import (
	"context"
	"slices"
	"sync/atomic"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// QuitTimeout is how long the engine waits for the work of the OnQuit
// handlers as the app quits, at most, as GPUI's shutdown timeout. The
// platform's main thread waits for the engine meanwhile, so it stays
// short.
const QuitTimeout = 200 * time.Millisecond

type quitHandler struct {
	owner *Scope
	fn    func(ctx context.Context)
}

// QuitRequest is a user-initiated request to end the app. A handler may
// cancel it, or defer it while it asks the user what to do.
type QuitRequest struct {
	canceled bool
	deferred bool
	answered atomic.Bool
	answer   func(bool)
}

// Cancel keeps the app running. It has an effect only while the request
// handler is running.
func (q *QuitRequest) Cancel() {
	if q != nil && !q.deferred {
		q.canceled = true
	}
}

// Defer keeps the platform's quit request pending and returns a one-shot
// answer. quit true proceeds with quitting; false cancels it. The returned
// function is safe to call from another goroutine.
func (q *QuitRequest) Defer() func(quit bool) {
	if q == nil || q.canceled || q.deferred || q.answer == nil {
		return func(bool) {}
	}
	q.deferred = true
	return func(quit bool) {
		if q.answered.Swap(true) {
			return
		}
		q.answer(quit)
	}
}

type quitRequestHandler struct {
	owner *Scope
	fn    func(*QuitRequest)
}

// OnQuitRequest hears a user-initiated request to quit, such as Cmd+Q or
// a tray menu's Quit. Handlers run newest first. A handler that does not
// cancel or defer lets the request continue to the next handler; when all
// handlers do so, the app quits. Scope.Quit and Restart remain immediate.
func (cx *Scope) OnQuitRequest(fn func(*QuitRequest)) {
	rt := cx.rt
	h := &quitRequestHandler{owner: cx.current(), fn: fn}
	rt.quitRequests = append(rt.quitRequests, h)
	h.owner.addCleanup(func() {
		rt.quitRequests = slices.DeleteFunc(rt.quitRequests, func(o *quitRequestHandler) bool { return o == h })
	})
}

func (rt *runtime) quitRequest(id uint32) {
	answer := func(quit bool) {
		t := rt.tree
		if t == nil {
			return
		}
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemQuitResponse, Count: id, On: quit})
		t.changed = true
	}
	for i := len(rt.quitRequests) - 1; i >= 0; i-- {
		h := rt.quitRequests[i]
		if h.owner.disposed {
			continue
		}
		q := &QuitRequest{answer: func(quit bool) {
			if rt.post != nil {
				rt.post(func() { answer(quit) })
			} else {
				answer(quit)
			}
		}}
		rt.guard(h.owner, func() { h.fn(q) })
		switch {
		case q.canceled:
			answer(false)
			return
		case q.deferred:
			return
		}
	}
	answer(true)
}

// OnQuit runs fn, while the current owner lives, as the app quits,
// however it stops: its own Quit, the last window closing, the Dock's or
// the system's quit, the runtime ending on Android, a host removing its
// view. A page hiding on the web and a watchOS app going to the
// background run it too, as the system may end either unheard, and keep
// it for the next time in case the app shows again. fn cleans up, such
// as saving a draft or closing a connection; it cannot keep the app
// running. The handlers run on the engine goroutine, oldest first, with
// one context that ends QuitTimeout after the first starts: a handler
// may wait on it for its own goroutines' work, and the engine ends once
// they return or the time is up.
func (cx *Scope) OnQuit(fn func(ctx context.Context)) {
	rt := cx.rt
	h := &quitHandler{owner: cx.current(), fn: fn}
	rt.quits = append(rt.quits, h)
	h.owner.addCleanup(func() { rt.quits = slices.DeleteFunc(rt.quits, func(o *quitHandler) bool { return o == h }) })
}

// Quitting runs the OnQuit handlers of the graph whose root cx is, for
// the engine as it stops, and reports whether there were any. Each runs
// once: a second call runs none.
func (cx *Scope) Quitting() bool {
	return cx.quitting(false)
}

// Hiding runs the OnQuit handlers as Quitting does but keeps them, for an
// app the system hides and may end without a word: a page the browser
// keeps in its back-forward cache, or a watchOS app in the background.
// It may show the app again, which runs them again at its next hide or
// quit.
func (cx *Scope) Hiding() bool {
	return cx.quitting(true)
}

func (cx *Scope) quitting(keep bool) bool {
	rt := cx.rt
	hs := slices.Clone(rt.quits)
	if len(hs) == 0 {
		return false
	}
	if !keep {
		rt.quits = nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), QuitTimeout)
	defer cancel()
	for _, h := range hs {
		if h.owner.disposed || ctx.Err() != nil {
			continue
		}
		rt.guard(h.owner, func() { h.fn(ctx) })
	}
	return true
}
