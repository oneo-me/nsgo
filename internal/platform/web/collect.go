//go:build js && wasm

package web

import (
	"os"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"syscall/js"
)

// Go's collector runs on the page's one thread: once a cycle starts, its
// idle marker finishes it before control returns to the page, so the
// cycle's marking lands in the task of the frame that started it, and
// the two together went over 50 ms. The page collects in a task of its
// own instead: the automatic trigger is raised out of the way, and once
// the app has allocated what GOGC=100 would collect at, a collection is
// queued for when the page is idle, or after collectWait at the latest.
// GOGC in the environment keeps the runtime's own pacing.

const (
	// collectPercent is the automatic trigger, which only a burst of
	// allocation the idle collections cannot keep up with reaches.
	collectPercent = 400
	// collectWait is how long a queued collection waits for the page to
	// be idle, in ms.
	collectWait = 500
)

// collector queues the page's collections.
var collector struct {
	on      bool
	queued  bool
	samples [2]metrics.Sample // allocated and live bytes
	since   uint64            // the allocations at the last collection
	run     js.Func
}

func init() {
	if os.Getenv("GOGC") != "" {
		return
	}
	collector.on = true
	debug.SetGCPercent(collectPercent)
	collector.samples[0].Name = "/gc/heap/allocs:bytes"
	collector.samples[1].Name = "/gc/heap/live:bytes"
	collector.run = js.FuncOf(func(js.Value, []js.Value) any {
		runtime.GC()
		metrics.Read(collector.samples[:1])
		collector.since = collector.samples[0].Value.Uint64()
		collector.queued = false
		return nil
	})
}

// collectLater queues a collection once the app has allocated as much as
// the heap holds since the last one, and 4 MB at least, as GOGC=100's
// smallest heap.
func collectLater() {
	c := &collector
	if !c.on || c.queued {
		return
	}
	metrics.Read(c.samples[:])
	if c.samples[0].Value.Uint64()-c.since < max(c.samples[1].Value.Uint64(), 4<<20) {
		return
	}
	c.queued = true
	if idle := js.Global().Get("requestIdleCallback"); idle.Truthy() {
		idle.Invoke(c.run, map[string]any{"timeout": collectWait})
	} else {
		js.Global().Call("setTimeout", c.run, 0)
	}
}
