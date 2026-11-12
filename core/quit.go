package core

import (
	"context"
	"slices"
	"time"
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

// OnQuit runs fn, while the current owner lives, as the app quits,
// however it stops: its own Quit, the last window closing, the Dock's or
// the system's quit, the runtime ending on Android, a host removing its
// view. fn cleans up, such as saving a draft or closing a connection; it
// cannot keep the app running. The handlers run on the engine goroutine,
// oldest first, with one context that ends QuitTimeout after the first
// starts: a handler may wait on it for its own goroutines' work, and the
// engine ends once they return or the time is up.
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
	rt := cx.rt
	hs := rt.quits
	if len(hs) == 0 {
		return false
	}
	rt.quits = nil
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
