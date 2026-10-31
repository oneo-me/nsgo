package core

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// newEmbedded is newHost with a host channel, as an embedding host runs
// its app.
func newEmbedded(t *testing.T, ch *Host, app func(cx *Scope) Node) *host {
	h := &host{t: t, clock: NewManualClock(time.Unix(0, 0))}
	h.root = NewRoot(RootOptions{
		Post: func(fn func()) {
			h.mu.Lock()
			h.posts = append(h.posts, fn)
			h.mu.Unlock()
		},
		Clock:  h.clock,
		Report: func(err error) { h.errs = append(h.errs, err) },
		Host:   ch,
	})
	h.tree = NewTree(h.root, ApproxTextEngine{})
	h.tree.Resize(Size{Width: 100, Height: 100})
	h.tree.Mount(app)
	h.drain()
	return h
}

func TestHostParamsAndEvents(t *testing.T) {
	ch := NewHost()
	if err := ch.SetParams(map[string]any{"user": "Ada", "n": 2}); err != nil {
		t.Fatal(err)
	}
	// Sent before the app starts: kept for it.
	ch.Send("message", "early")
	var params map[string]any
	var heard []any
	var late []any
	var cx *Scope
	h := newEmbedded(t, ch, func(c *Scope) Node {
		cx = c
		params = c.HostParams()
		c.OnHostEvent("message", func(data any) { heard = append(heard, data) })
		return View{}
	})
	if params["user"] != "Ada" || params["n"] != float64(2) {
		t.Fatalf("params = %v", params)
	}
	ch.Send("message", map[string]any{"text": "hi"})
	// Nobody listens to "later" yet: it waits for the first listener.
	ch.Send("later", 1)
	ch.Send("later", 2)
	h.drain()
	if len(heard) != 2 || heard[0] != "early" || heard[1].(map[string]any)["text"] != "hi" {
		t.Fatalf("heard = %v", heard)
	}
	cx.OnHostEvent("later", func(data any) { late = append(late, data) })
	if len(late) != 0 {
		t.Fatal("kept events reached the listener before its turn ended")
	}
	h.drain()
	if !slices.Equal(late, []any{float64(1), float64(2)}) {
		t.Fatalf("late = %v", late)
	}
	if err := ch.Send("bad", func() {}); err == nil {
		t.Fatal("a function was sent")
	}

	// Stopped, the app leaves what is still queued to the next one.
	ch.Send("message", "queued")
	h.root.Dispose()
	ch.Send("message", "between")
	heard = nil
	newEmbedded(t, ch, func(c *Scope) Node {
		c.OnHostEvent("message", func(data any) { heard = append(heard, data) })
		return View{}
	}).drain()
	if !slices.Equal(heard, []any{"queued", "between"}) {
		t.Fatalf("the next app heard %v", heard)
	}
}

func TestHostCalls(t *testing.T) {
	ch := NewHost()
	var later *HostCall
	var posted []any
	ch.Handle("time", func(c *HostCall) { c.Resolve(map[string]any{"at": c.Data}) })
	ch.Handle("slow", func(c *HostCall) { later = c })
	ch.Handle("fail", func(c *HostCall) { c.Reject(errors.New("no")) })
	ch.Handle("panic", func(c *HostCall) { panic("boom") })
	ch.Handle("log", func(c *HostCall) {
		if !c.IsPost() {
			t.Error("a post is a call")
		}
		posted = append(posted, c.Data)
	})
	var cx *Scope
	h := newEmbedded(t, ch, func(c *Scope) Node { cx = c; return View{} })
	type answer struct {
		v   any
		err error
	}
	got := map[string]answer{}
	call := func(name string, data any) {
		cx.CallHost(name, data, func(v any, err error) { got[name] = answer{v, err} })
	}
	call("time", 5)
	call("fail", nil)
	call("panic", nil)
	call("missing", nil)
	call("slow", nil)
	cx.PostHost("log", "a")
	cx.PostHost("nobody", "b")
	h.drain()
	if got["time"].v.(map[string]any)["at"] != float64(5) {
		t.Fatalf("time = %+v", got["time"])
	}
	if got["fail"].err == nil || got["panic"].err == nil || !errors.Is(got["missing"].err, ErrNoHostHandler) {
		t.Fatalf("answers = %+v", got)
	}
	if !slices.Equal(posted, []any{"a"}) {
		t.Fatalf("posted = %v", posted)
	}
	if _, ok := got["slow"]; ok {
		t.Fatal("slow answered before the host did")
	}
	go later.Resolve("done")
	for deadline := time.Now().Add(time.Second); got["slow"].v == nil && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
		h.drain()
	}
	if got["slow"].v != "done" {
		t.Fatalf("slow = %+v", got["slow"])
	}
	later.Resolve("again") // only the first answer counts

	// Unanswered, a call fails after its timeout.
	call("slow", nil)
	h.clock.Advance(HostCallTimeout)
	h.drain()
	if !errors.Is(got["slow"].err, ErrHostTimeout) {
		t.Fatalf("slow = %+v", got["slow"])
	}
}

func TestHostDefaultHandler(t *testing.T) {
	ch := NewHost()
	var names []string
	ch.Handle("own", func(c *HostCall) { c.Resolve("own") })
	ch.HandleDefault(func(c *HostCall) {
		names = append(names, c.Name)
		c.Resolve("default")
	})
	var cx *Scope
	h := newEmbedded(t, ch, func(c *Scope) Node { cx = c; return View{} })
	got := map[string]any{}
	for _, name := range []string{"own", "any.name"} {
		cx.CallHost(name, nil, func(v any, err error) { got[name] = v })
	}
	cx.PostHost("posted", nil)
	h.drain()
	if got["own"] != "own" || got["any.name"] != "default" || !slices.Equal(names, []string{"any.name", "posted"}) {
		t.Fatalf("answers %v, default heard %v", got, names)
	}
	ch.HandleDefault(nil)
	var err error
	cx.CallHost("any.name", nil, func(_ any, e error) { err = e })
	h.drain()
	if !errors.Is(err, ErrNoHostHandler) {
		t.Fatalf("without a default: %v", err)
	}
}

func TestHostTokens(t *testing.T) {
	var accent, inset Color
	h := newEmbedded(t, NewHost(), func(c *Scope) Node {
		c.Effect(func() { p := c.Palette(); accent, inset = p.Accent, p.Colors["inset"] })
		return View{}
	})
	h.tree.SetTokens(Tokens{Values: map[string]string{"accent": "#e11d48", "inset": "#eeeeee", "bad": "nope"},
		Dark: map[string]string{"accent": "#fb7185"}})
	h.drain()
	if accent != RGB(0xe11d48) || inset != RGB(0xeeeeee) {
		t.Fatalf("accent = %v, inset = %v", accent, inset)
	}
	h.tree.SetEnvironment(Environment{Dark: true})
	h.drain()
	if accent != RGB(0xfb7185) {
		t.Fatalf("dark accent = %v", accent)
	}
}
