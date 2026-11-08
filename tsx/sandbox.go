package tsx

import (
	"errors"
	"time"

	"github.com/oneo-me/nsgo/tsx/internal/js"

	"github.com/oneo-me/nsgo/core"
)

// The sandbox's limits (NSH's sandbox.rs): what a bundle may use, set by
// the Go code that runs it, never by JS. A bundle holds only the app's
// code, so nothing in JS is trusted to keep them. Capabilities, which
// say what requests a module may make, are not ported yet.

// Limits are what a bundle may use. Zero fields leave that unlimited.
type Limits struct {
	// Load is how long the bundle's script may run as it loads, and Call
	// how long each call into JS may run (a handler, an effect, a
	// render): past it, the call is cut with ErrBudget.
	Load, Call time.Duration
	// Stack is how deep JS calls may nest.
	Stack int
	// Harden deletes eval, has the Function constructors throw and
	// freezes the built-in prototypes, so code cannot be made from
	// strings or the shared objects changed.
	Harden bool
}

// DefaultLimits are NSH's: 10 seconds to load, 2 per call, 1000 frames,
// hardened.
var DefaultLimits = Limits{Load: 10 * time.Second, Call: 2 * time.Second, Stack: 1000, Harden: true}

// ErrBudget is what a call cut at its time budget fails with.
var ErrBudget = errors.New("tsx: the script ran past its time budget")

// Sandboxed returns the app of a bundle, as App does, run within limits.
func Sandboxed(name, src string, limits Limits) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		r := newRuntime(cx)
		r.limits = limits
		if limits.Stack > 0 {
			r.vm.SetMaxCallStackSize(limits.Stack)
		}
		if limits.Harden {
			r.harden()
		}
		r.within(limits.Load, func() {
			if _, err := r.vm.RunScript(name, src); err != nil {
				panic(err)
			}
		})
		return r.root.elem
	}
}

// within runs fn, a call into JS from Go, cut at budget when it is the
// outermost.
func (r *runtime) within(budget time.Duration, fn func()) {
	if budget <= 0 || r.depth > 0 {
		r.depth++
		defer func() { r.depth-- }()
		fn()
		return
	}
	r.depth++
	timer := time.AfterFunc(budget, func() { r.vm.Interrupt(ErrBudget) })
	defer func() {
		timer.Stop()
		r.vm.ClearInterrupt()
		r.depth--
	}()
	fn()
}

// hardenSource closes what makes code from strings and freezes the
// shared prototypes.
const hardenSource = `(function (g) {
	"use strict";
	const refuse = function () { throw new EvalError("nsgo: code is not made from strings here"); };
	for (const make of [function () {}, async function () {}, function* () {}]) {
		const F = make.constructor;
		Object.defineProperty(F.prototype, "constructor", { value: refuse });
	}
	delete g.eval;
	g.Function = refuse;
	for (const C of [Object, Array, Function, String, Number, Boolean, Symbol, Promise, Map, Set, WeakMap,
		WeakSet, Date, RegExp, Error, TypeError, RangeError, SyntaxError, ArrayBuffer, Uint8Array, JSON, Math, Reflect]) {
		Object.freeze(C);
		if (C.prototype) Object.freeze(C.prototype);
	}
})`

var hardenProgram = js.MustCompile("harden.js", hardenSource, true)

func (r *runtime) harden() {
	v, err := r.vm.RunProgram(hardenProgram)
	if err != nil {
		panic(err)
	}
	f, _ := js.AssertFunction(v)
	if _, err := f(js.Undefined(), r.vm.GlobalObject()); err != nil {
		panic(err)
	}
}
