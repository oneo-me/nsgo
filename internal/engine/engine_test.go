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
