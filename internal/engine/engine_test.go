package engine

import (
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
)

// recorder is a platform that keeps the batches it is handed.
type recorder struct {
	core.ApproxTextEngine
	mu      sync.Mutex
	batches []*driver.MountBatch
	mounted chan struct{}
	// frames receives a value for each display frame the app asks for.
	frames chan struct{}
}

func (r *recorder) Mount(b *driver.MountBatch) {
	r.mu.Lock()
	r.batches = append(r.batches, b)
	r.mu.Unlock()
	select {
	case r.mounted <- struct{}{}:
	default:
	}
}

// RequestFrame implements Platform: the app waits for a display frame.
func (r *recorder) RequestFrame() {
	if r.frames == nil {
		return
	}
	select {
	case r.frames <- struct{}{}:
	default:
	}
}

// texts returns the latest text of every text view.
func (r *recorder) texts() map[driver.NodeID]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[driver.NodeID]string{}
	for _, b := range r.batches {
		for _, it := range b.Items {
			if it.Kind == driver.ItemText {
				out[it.ID] = it.Text.Text
			}
		}
	}
	return out
}

func TestDispatchNeverBlocksAndMerges(t *testing.T) {
	p := &recorder{mounted: make(chan struct{}, 1)}
	var presses int
	var button driver.NodeID
	e := Start(p, func(cx *core.Scope) core.Node {
		count := core.NewSignal(cx, 0)
		return core.View{Children: []core.Node{
			core.View{OnPress: func(core.PressEvent) { presses++; count.Set(presses) }, TestID: "button"},
			core.Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) }},
		}}
	}, Options{Size: driver.Size{Width: 100, Height: 100}})
	defer e.Close()
	<-p.mounted
	p.mu.Lock()
	for _, b := range p.batches {
		for _, it := range b.Items {
			if it.Kind == driver.ItemViewProps && it.ViewProps.TestID == "button" {
				button = it.ID
			}
		}
	}
	p.mu.Unlock()

	// Far more than any fixed buffer, from a goroutine standing in for the
	// main thread: none of these may block.
	done := make(chan struct{})
	go func() {
		for range 5000 {
			e.Dispatch(driver.Event{Kind: driver.EventPress, ID: button})
			e.Resize(driver.Size{Width: 100, Height: 100})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Dispatch blocked")
	}
	deadline := time.After(5 * time.Second)
	for {
		found := false
		for _, s := range p.texts() {
			found = found || s == "5000"
		}
		if found {
			return
		}
		select {
		case <-p.mounted:
		case <-deadline:
			t.Fatalf("texts = %v, want 5000 presses counted", p.texts())
		}
	}
}

func TestCloseDisposesTheApp(t *testing.T) {
	p := &recorder{mounted: make(chan struct{}, 1)}
	cleaned := make(chan struct{})
	e := Start(p, func(cx *core.Scope) core.Node {
		cx.Cleanup(func() { close(cleaned) })
		return core.View{}
	}, Options{})
	<-p.mounted
	e.Close()
	select {
	case <-cleaned:
	default:
		t.Fatal("Close did not dispose the app")
	}
}

// The app's frame callbacks run when the platform reports the frame it
// was asked for, and a callback that waits again asks for another.
func TestFramesFollowThePlatform(t *testing.T) {
	p := &recorder{mounted: make(chan struct{}, 1), frames: make(chan struct{}, 4)}
	seen := make(chan time.Time, 4)
	e := Start(p, func(cx *core.Scope) core.Node {
		var next func(at time.Time)
		next = func(at time.Time) {
			seen <- at
			cx.RequestAnimationFrame(next)
		}
		cx.RequestAnimationFrame(next)
		return core.View{}
	}, Options{})
	defer e.Close()
	<-p.mounted

	select {
	case <-p.frames:
	case <-time.After(5 * time.Second):
		t.Fatal("the app asked for no display frame")
	}
	e.Frame()
	select {
	case at := <-seen:
		if at.IsZero() {
			t.Fatal("the frame had no time")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the frame did not run the app's callback")
	}
	select {
	case <-p.frames:
	case <-time.After(5 * time.Second):
		t.Fatal("the callback waiting for the next frame did not ask for it")
	}
}

// TestATestsClipboardIsItsOwn sets and reads the clipboard of an app a
// test observes: its own memory answers, and the platform sees nothing.
func TestATestsClipboardIsItsOwn(t *testing.T) {
	p := &recorder{mounted: make(chan struct{}, 1)}
	read := make(chan string, 1)
	var opts Options
	opts.UseTest(testhostOptions(func(*driver.MountBatch) {}), &TestPin{})
	opts.Size = driver.Size{Width: 100, Height: 100}
	e := Start(p, func(cx *core.Scope) core.Node {
		cx.SetClipboard("secret")
		cx.Post(func() { cx.ReadClipboard(func(s string) { read <- s }) })
		return core.View{}
	}, opts)
	defer e.Close()
	select {
	case s := <-read:
		if s != "secret" {
			t.Errorf("read %q, want the text set", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the read was not answered")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, b := range p.batches {
		for _, it := range b.Items {
			if it.Kind == driver.ItemSetClipboard || it.Kind == driver.ItemReadClipboard {
				t.Errorf("the platform got %v", it.Kind)
			}
		}
	}
}

// Trace hears each commit's batch before the platform does.
func TestTraceHearsEachCommit(t *testing.T) {
	type heard struct {
		items   int
		mounted bool
	}
	p := &recorder{mounted: make(chan struct{}, 1)}
	traced := make(chan heard, 4)
	Trace = func(b *driver.MountBatch, took time.Duration) {
		p.mu.Lock()
		mounted := slices.Contains(p.batches, b)
		p.mu.Unlock()
		traced <- heard{len(b.Items), mounted}
	}
	defer func() { Trace = nil }()
	e := Start(p, func(cx *core.Scope) core.Node { return core.Text{Content: "hi"} }, Options{})
	defer e.Close()
	<-p.mounted
	if h := <-traced; h.items == 0 || h.mounted {
		t.Fatalf("the trace heard %+v", h)
	}
}

// items returns the items of kind the recorder was handed.
func (r *recorder) items(kind driver.ItemKind) []driver.MountItem {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []driver.MountItem
	for _, b := range r.batches {
		for _, it := range b.Items {
			if it.Kind == kind {
				out = append(out, it)
			}
		}
	}
	return out
}

// wait waits until cond holds of r.
func (r *recorder) wait(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		select {
		case <-r.mounted:
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestAttachOpenedSurfaces(t *testing.T) {
	app := &recorder{mounted: make(chan struct{}, 1)}
	var count *core.Signal[int]
	e := Start(app, core.AppRoot(func(cx *core.Scope) {
		count = core.NewSignal(cx, 1)
		cx.SetMainView(func(cx *core.Scope) core.Node {
			return core.Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) }}
		})
	}), Options{Windows: &core.WindowOptions{Width: 300, Height: 200}})
	defer e.Close()
	app.wait(t, "the main window to open", func() bool { return len(app.items(driver.ItemOpenSurface)) == 1 })
	cfg := app.items(driver.ItemOpenSurface)[0].Surface
	if cfg.ID != 1 || cfg.Width != 300 {
		t.Fatalf("opened %+v", cfg)
	}
	win := &recorder{mounted: make(chan struct{}, 1)}
	we := e.Attach(cfg.ID, win)
	we.Resize(driver.Size{Width: 300, Height: 200})
	win.wait(t, "the window's view", func() bool { return slices.Contains(slices.Collect(values(win.texts())), "1") })
	done := make(chan struct{})
	e.push(message{fn: func() { count.Set(2); close(done) }})
	<-done
	win.wait(t, "a shared signal to reach the window", func() bool {
		return slices.Contains(slices.Collect(values(win.texts())), "2")
	})
	we.Dispatch(driver.Event{Kind: driver.EventWindowClose})
	win.wait(t, "the window to be destroyed", func() bool {
		for _, it := range win.items(driver.ItemWindow) {
			if it.Window.Action == driver.WindowDestroy {
				return true
			}
		}
		return false
	})
	app.wait(t, "the app to quit with its last window", func() bool { return len(app.items(driver.ItemQuit)) == 1 })
}

// values returns the values of m.
func values(m map[driver.NodeID]string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for _, v := range m {
			if !yield(v) {
				return
			}
		}
	}
}

// TestWheelTurnsAddUp merges a view's queued wheel turns by adding their
// deltas, where other moves replace each other, and keeps a touchpad's
// apart from a wheel's.
func TestWheelTurnsAddUp(t *testing.T) {
	e := &Engine{runner: &runner{}}
	turn := func(dy driver.Points, precise bool) {
		e.push(message{event: driver.Event{Kind: driver.EventWheel, ID: 7, Phase: driver.GestureChanged, Delta: driver.Point{Y: dy}, Precise: precise}})
	}
	turn(10, false)
	turn(15, false)
	turn(4, true)
	q := e.take()
	if len(q) != 2 || q[0].event.Delta.Y != 25 || q[1].event.Delta.Y != 4 {
		t.Fatalf("queued %+v", q)
	}
}
