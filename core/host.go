package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// Host is the channel between an app and the host app that embeds it,
// without a view between them: launch parameters, events the host sends,
// and the host's functions the app calls. A host makes one for each view
// and keeps it across the apps the view runs, one at a time
// (RootOptions.Host). Its methods are safe to call from any goroutine.
//
// Values cross it as JSON does, so iOS and Android hosts see the same:
// what Send, SetParams and HostCall.Resolve take is encoded and decoded
// again, maps becoming map[string]any and numbers float64.
type Host struct {
	mu       sync.Mutex
	params   map[string]any
	handlers map[string]func(*HostCall)
	fallback func(*HostCall)
	openURL  func(url string, answer func(handled bool))
	// queue holds the events sent and not yet delivered: taken by the
	// running app's next turn, or kept for the next app while none runs.
	queue []hostEvent
	app   *hostApp
}

// Tokens are colors an embedding host themes the app with, by the role
// names of Palette.Color ("accent", "inset"), written as CSS writes them.
// They go over every theme's palette (Scope.Palette): Values in both
// appearances, and Dark's over them where a subtree shows dark. A value
// that is no color is dropped.
type Tokens struct {
	Values, Dark map[string]string
}

// SetTokens gives the app the host's tokens, from now on: what reads
// Env.Token runs again.
func (t *Tree) SetTokens(tokens Tokens) {
	t.root.Env().tokens.Set(parseTokens(tokens))
}

// HostInbox is how many events a Host or an app keeps for later: those
// sent while no app runs, and those of names no listener hears yet. Past
// it the oldest are dropped.
const HostInbox = 256

// HostCallTimeout is how long CallHost waits for the host's answer.
const HostCallTimeout = 30 * time.Second

// maxHostCalls is how many calls an app may have waiting at once.
const maxHostCalls = 1024

// ErrNoHostHandler is the error a call to a name the host handles no
// longer, or never did, fails with.
var ErrNoHostHandler = errors.New("core: the host has no handler")

// ErrHostTimeout is the error a call the host did not answer in time
// fails with.
var ErrHostTimeout = errors.New("core: the host did not answer in time")

type hostEvent struct {
	name string
	data any
}

// NewHost returns a channel with no parameters or handlers.
func NewHost() *Host { return &Host{handlers: map[string]func(*HostCall){}} }

// SetParams sets the parameters the next app to start reads with
// Scope.HostParams, fixed for its life. Later requests are events.
func (h *Host) SetParams(params map[string]any) error {
	v, err := hostValue(params)
	if err != nil {
		return err
	}
	m, _ := v.(map[string]any)
	h.mu.Lock()
	h.params = m
	h.mu.Unlock()
	return nil
}

// Send sends the app an event, which its OnHostEvent listeners of name
// hear on its goroutine. It never waits. While no app runs, or none of
// its listeners hears name yet, the event is kept for later, at most
// HostInbox of them. It fails only for data JSON cannot encode.
func (h *Host) Send(name string, data any) error {
	v, err := hostValue(data)
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.queue = keepEvent(h.queue, hostEvent{name, v}, "host")
	app := h.app
	wake := app != nil && len(h.queue) == 1
	h.mu.Unlock()
	if wake {
		app.rt.post(func() { app.drain() })
	}
	return nil
}

// keepEvent appends ev to q, dropping the oldest past HostInbox.
func keepEvent(q []hostEvent, ev hostEvent, where string) []hostEvent {
	if len(q) >= HostInbox {
		log.Printf("nsgo: the %s inbox is full: event %q is dropped", where, q[0].name)
		q = q[1:]
	}
	return append(q, ev)
}

// Handle has fn answer the app's calls and posts of name (Scope.CallHost,
// Scope.PostHost), replacing the handler it had. fn runs on the app's
// goroutine, so it must not block: it answers the call once, then or
// later from any goroutine.
func (h *Host) Handle(name string, fn func(call *HostCall)) {
	h.mu.Lock()
	h.handlers[name] = fn
	h.mu.Unlock()
}

// RemoveHandler removes the handler of name: calls of it fail at once.
func (h *Host) RemoveHandler(name string) {
	h.mu.Lock()
	delete(h.handlers, name)
	h.mu.Unlock()
}

// HandleDefault has fn answer the calls and posts of every name without a
// handler of its own, as Handle's handlers do, for a host that looks
// names up itself, such as a web page's. nil removes it.
func (h *Host) HandleDefault(fn func(call *HostCall)) {
	h.mu.Lock()
	h.fallback = fn
	h.mu.Unlock()
}

// HandleOpenURL has fn see the URLs the app opens (Scope.OpenURL) before
// the platform does, once they pass CheckURL, so the host routes its own
// links through its own router. fn runs on the app's goroutine and must
// not block; it calls answer once, from any goroutine: true takes the
// URL, and the app hears it opened; false hands it to the platform. nil
// removes the handler.
func (h *Host) HandleOpenURL(fn func(url string, answer func(handled bool))) {
	h.mu.Lock()
	h.openURL = fn
	h.mu.Unlock()
}

func (h *Host) handler(name string) func(*HostCall) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if fn := h.handlers[name]; fn != nil {
		return fn
	}
	return h.fallback
}

// HostCall is one call or post of the app's to a host's handler.
type HostCall struct {
	// Name is the handler's name; Data what the app sent, as JSON
	// decodes it.
	Name string
	Data any
	post bool

	once  sync.Once
	reply func(result any, err error)
}

// IsPost reports whether the app posted rather than called: nothing hears
// the answer.
func (c *HostCall) IsPost() bool { return c.post }

// Resolve answers the call with result. Only the first answer counts.
func (c *HostCall) Resolve(result any) {
	v, err := hostValue(result)
	c.answer(v, err)
}

// Reject fails the call with err.
func (c *HostCall) Reject(err error) {
	if err == nil {
		err = errors.New("core: the host rejected the call")
	}
	c.answer(nil, err)
}

func (c *HostCall) answer(result any, err error) {
	c.once.Do(func() {
		if c.reply != nil {
			c.reply(result, err)
		}
	})
}

// hostValue returns v as JSON decodes it: a deep copy of plain values.
func hostValue(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("core: a host value is JSON: %w", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// hostApp is the running app's end of a Host, on its graph's goroutine.
type hostApp struct {
	h      *Host
	rt     *runtime
	params map[string]any
	// listeners hear events by name; kept are those no listener heard.
	listeners map[string][]*hostListener
	kept      []hostEvent
	calls     int
}

type hostListener struct{ fn func(data any) }

// attachHost makes rt the app h serves, taking the events kept for it.
func (rt *runtime) attachHost(h *Host) {
	a := &hostApp{h: h, rt: rt, listeners: map[string][]*hostListener{}}
	h.mu.Lock()
	a.params = h.params
	h.app = a
	pending := len(h.queue) > 0
	h.mu.Unlock()
	if a.params == nil {
		a.params = map[string]any{}
	}
	rt.host = a
	if pending {
		rt.post(func() { a.drain() })
	}
}

// detachHost leaves h's events still queued for the next app.
func (rt *runtime) detachHost() {
	a := rt.host
	if a == nil {
		return
	}
	a.h.mu.Lock()
	if a.h.app == a {
		a.h.app = nil
	}
	a.h.mu.Unlock()
	rt.host = nil
}

// drain delivers the events queued since the last drain.
func (a *hostApp) drain() {
	a.h.mu.Lock()
	if a.h.app != a {
		a.h.mu.Unlock()
		return
	}
	q := a.h.queue
	a.h.queue = nil
	a.h.mu.Unlock()
	for _, ev := range q {
		a.deliver(ev)
	}
}

// deliver runs ev's listeners, or keeps it until one of its name comes.
func (a *hostApp) deliver(ev hostEvent) {
	ls := a.listeners[ev.name]
	if len(ls) == 0 {
		a.kept = keepEvent(a.kept, ev, "app's host")
		return
	}
	for _, l := range append([]*hostListener(nil), ls...) {
		a.rt.run(a.rt.root, func() { l.fn(ev.data) })
	}
}

// HostParams returns the parameters the embedding host gave the app
// before it started (Host.SetParams): an empty map without a host or
// any.
func (cx *Scope) HostParams() map[string]any {
	if a := cx.rt.host; a != nil {
		return a.params
	}
	return map[string]any{}
}

// OnHostEvent calls fn with each event of name the embedding host sends
// (Host.Send), until the current owner is disposed. Events sent before
// the app had a listener of name are kept for it: the first listener
// hears them, in order, once the code adding it has run.
func (cx *Scope) OnHostEvent(name string, fn func(data any)) {
	a := cx.rt.host
	if a == nil {
		return
	}
	o := cx.current()
	l := &hostListener{fn: func(data any) { cx.rt.run(o, func() { fn(data) }) }}
	first := len(a.listeners[name]) == 0
	a.listeners[name] = append(a.listeners[name], l)
	o.addCleanup(func() {
		ls := a.listeners[name]
		for i, x := range ls {
			if x == l {
				a.listeners[name] = append(ls[:i:i], ls[i+1:]...)
				break
			}
		}
		if len(a.listeners[name]) == 0 {
			delete(a.listeners, name)
		}
	})
	if !first {
		return
	}
	// What was kept for name goes once the turn that added the listener
	// is over, so listeners added together all hear it.
	cx.rt.post(func() {
		var keep []hostEvent
		var mine []hostEvent
		for _, ev := range a.kept {
			if ev.name == name {
				mine = append(mine, ev)
			} else {
				keep = append(keep, ev)
			}
		}
		a.kept = keep
		for _, ev := range mine {
			a.deliver(ev)
		}
	})
}

// CallHost calls the embedding host's handler of name with data (any
// value JSON encodes) and has done hear its answer on the app's
// goroutine, unless the current owner is disposed first. It fails at once
// without a host or a handler of name, and after HostCallTimeout without
// an answer.
func (cx *Scope) CallHost(name string, data any, done func(result any, err error)) {
	cx.CallHostTimeout(name, data, HostCallTimeout, done)
}

// CallHostTimeout is CallHost waiting timeout for the answer; 0 waits
// without a deadline.
func (cx *Scope) CallHostTimeout(name string, data any, timeout time.Duration, done func(result any, err error)) {
	o, rt := cx.current(), cx.rt
	finish := func(result any, err error) {
		if done != nil {
			rt.post(func() {
				if !o.disposed {
					rt.run(o, func() { done(result, err) })
				}
			})
		}
	}
	a := rt.host
	if a == nil {
		finish(nil, fmt.Errorf("%w: no host embeds the app", ErrNoHostHandler))
		return
	}
	fn := a.h.handler(name)
	if fn == nil {
		finish(nil, fmt.Errorf("%w of %q", ErrNoHostHandler, name))
		return
	}
	if a.calls >= maxHostCalls {
		finish(nil, fmt.Errorf("core: %d host calls are waiting already", maxHostCalls))
		return
	}
	v, err := hostValue(data)
	if err != nil {
		finish(nil, err)
		return
	}
	a.calls++
	var stop func() bool
	call := &HostCall{Name: name, Data: v}
	call.reply = func(result any, err error) {
		rt.post(func() {
			a.calls--
			if stop != nil {
				stop()
			}
		})
		finish(result, err)
	}
	if timeout > 0 {
		stop = rt.clock.AfterFunc(timeout, func() { call.answer(nil, fmt.Errorf("%w (%s, %v)", ErrHostTimeout, name, timeout)) })
	}
	a.handle(fn, call)
}

// PostHost sends data to the embedding host's handler of name, which has
// nothing to answer. Without a host or a handler it is dropped.
func (cx *Scope) PostHost(name string, data any) {
	a := cx.rt.host
	if a == nil {
		return
	}
	fn := a.h.handler(name)
	v, err := hostValue(data)
	if fn == nil || err != nil {
		return
	}
	a.handle(fn, &HostCall{Name: name, Data: v, post: true})
}

// handle runs a host's handler, failing the call when it panics.
func (a *hostApp) handle(fn func(*HostCall), call *HostCall) {
	defer func() {
		if r := recover(); r != nil {
			call.Reject(fmt.Errorf("core: the host's handler of %q panicked: %v", call.Name, r))
		}
	}()
	fn(call)
}

// openURL offers u to the host's handler, which answer hears on the
// graph's goroutine; it reports false when the host has none.
func (a *hostApp) openURL(u string, answer func(handled bool)) bool {
	if a == nil {
		return false
	}
	a.h.mu.Lock()
	fn := a.h.openURL
	a.h.mu.Unlock()
	if fn == nil {
		return false
	}
	var once sync.Once
	reply := func(handled bool) {
		once.Do(func() { a.rt.post(func() { answer(handled) }) })
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("nsgo: the host's URL handler panicked: %v", r)
			reply(false)
		}
	}()
	fn(u, reply)
	return true
}
