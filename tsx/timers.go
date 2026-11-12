package tsx

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"
)

// The globals NSH's runtime gives every VM (timers.rs, console.rs):
// timers on the engine goroutine through Scope.AfterFunc, and console
// through the log package.

// timers are the VM's pending timers by ID.
type timers struct {
	next int64
	stop map[int64]func() bool
}

// timerQuota throws when a sandbox's waiting timers, or frame callbacks,
// number its quota.
func (r *runtime) timerQuota(waiting int) {
	if r.sandbox != nil && r.limits.Timers > 0 && waiting >= r.limits.Timers {
		panic(r.goError(quota("timers", r.limits.Timers)))
	}
}

func (r *runtime) installTimers() {
	r.timers.stop = map[int64]func() bool{}
	set := func(repeat bool) func(js.FunctionCall) js.Value {
		return func(call js.FunctionCall) js.Value {
			fn := arg(call, 0)
			d := time.Duration(max(arg(call, 1).ToFloat(), 0) * float64(time.Millisecond))
			args := call.Arguments[min(2, len(call.Arguments)):]
			r.timerQuota(len(r.timers.stop))
			r.timers.next++
			id := r.timers.next
			// An interval keeps its cadence: each run is due an interval
			// after the last was due, however late that one ran.
			due := r.cx.Now()
			var schedule func()
			schedule = func() {
				due = due.Add(d)
				// Timers belong to the app, not to what code runs when one
				// is set.
				r.cx.Run(func() {
					r.timers.stop[id] = r.cx.AfterFunc(max(due.Sub(r.cx.Now()), 0), func() {
						if repeat {
							schedule()
						} else {
							delete(r.timers.stop, id)
						}
						r.call(fn, args...)
					})
				})
			}
			schedule()
			return r.vm.ToValue(id)
		}
	}
	clear := func(call js.FunctionCall) js.Value {
		id := arg(call, 0).ToInteger()
		if stop := r.timers.stop[id]; stop != nil {
			stop()
			delete(r.timers.stop, id)
		}
		return js.Undefined()
	}
	_ = r.vm.Set("setTimeout", set(false))
	_ = r.vm.Set("setInterval", set(true))
	_ = r.vm.Set("clearTimeout", clear)
	_ = r.vm.Set("clearInterval", clear)
	frames := map[int64]func(){}
	_ = r.vm.Set("requestAnimationFrame", r.function(func(call js.FunctionCall) js.Value {
		fn := arg(call, 0)
		r.timerQuota(len(frames))
		r.timers.next++
		id := r.timers.next
		r.cx.Run(func() {
			// The frame's time, on performance.now()'s clock.
			frames[id] = r.cx.RequestAnimationFrame(func(at time.Time) {
				delete(frames, id)
				r.call(fn, r.vm.ToValue(r.millis(at)))
			})
		})
		return r.vm.ToValue(id)
	}))
	_ = r.vm.Set("cancelAnimationFrame", r.function(func(call js.FunctionCall) js.Value {
		id := arg(call, 0).ToInteger()
		if stop := frames[id]; stop != nil {
			stop()
			delete(frames, id)
		}
		return js.Undefined()
	}))
	_ = r.vm.Set("queueMicrotask", r.function(func(call js.FunctionCall) js.Value {
		fn := arg(call, 0)
		p, resolve, _ := r.vm.NewPromise()
		_ = resolve(js.Undefined())
		then, _ := js.AssertFunction(r.vm.ToValue(p).ToObject(r.vm).Get("then"))
		_, _ = then(r.vm.ToValue(p), fn)
		return js.Undefined()
	}))
	// reportError reports an exception as an uncaught one, after the
	// turn: the host shows it while the app runs on.
	_ = r.vm.Set("reportError", r.function(func(call js.FunctionCall) js.Value {
		thrown := arg(call, 0)
		r.cx.Post(func() { panic(thrown) })
		return js.Undefined()
	}))

	console := r.vm.NewObject()
	for _, level := range []string{"log", "info", "debug", "warn", "error"} {
		_ = console.Set(level, r.function(func(call js.FunctionCall) js.Value {
			parts := make([]string, len(call.Arguments))
			for i, a := range call.Arguments {
				parts[i] = r.display(a)
			}
			log.Printf("js %s: %s", level, strings.Join(parts, " "))
			return js.Undefined()
		}))
	}
	_ = r.vm.Set("console", console)
}

// millis is at as performance.now() reads it: milliseconds since the
// VM's start, in 1024ths, which a float holds exactly, so the time
// between two readings is exact too (a second of frames is 1000).
func (r *runtime) millis(at time.Time) float64 {
	return math.Round(float64(at.Sub(r.origin))/float64(time.Millisecond)*1024) / 1024
}

// display is how console shows a value: strings as they are, objects as
// JSON where they have one.
func (r *runtime) display(v js.Value) string {
	if o, ok := v.(*js.Object); ok && !isFunction(o) && o.ClassName() != "Error" {
		if b, err := o.MarshalJSON(); err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(v)
}
