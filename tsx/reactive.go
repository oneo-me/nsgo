package tsx

import (
	"github.com/oneo-me/nsgo/tsx/internal/js"

	"github.com/oneo-me/nsgo/core"
)

// The reactive graph JS runs on (NSH's reactive.rs): core's graph with
// goja values. Reading a signal or a memo is a call of a function over
// it; writes run the graph in Go, and only the app's closures run in JS.

// signal is `signal(value, options)`: a function reading the value, with
// set and update.
func (r *runtime) signal(call js.FunctionCall) js.Value {
	s := core.NewSignalFunc(r.cx, arg(call, 0), r.equals(arg(call, 1)))
	read := r.function(func(js.FunctionCall) js.Value { return s.Get() })
	_ = read.Set("set", r.function(func(call js.FunctionCall) js.Value {
		v := arg(call, 0)
		s.Set(v)
		return v
	}))
	_ = read.Set("update", r.function(func(call js.FunctionCall) js.Value {
		v := r.call(arg(call, 0), s.GetUntracked())
		s.Set(v)
		return v
	}))
	return read
}

// computed is `computed(fn, options)`: a function reading what fn
// computes from its previous value, again when what it read changes.
func (r *runtime) computed(call js.FunctionCall) js.Value {
	fn := arg(call, 0)
	prev := js.Undefined()
	m := core.NewMemoFunc(r.cx, func() js.Value {
		prev = r.call(fn, prev)
		return prev
	}, r.equals(arg(call, 1)))
	return r.accessor(m.Get)
}

// effect is `effect(fn)`: fn after rendering and again when what it read
// changes; a function fn returns is its cleanup.
func (r *runtime) effect(call js.FunctionCall) js.Value {
	fn := arg(call, 0)
	r.cx.Effect(func() {
		if c := r.call(fn); isFunction(c) {
			r.cx.Cleanup(func() { r.call(c) })
		}
	})
	return js.Undefined()
}

// batch is `batch(fn)`: fn's writes applied together at its end.
func (r *runtime) batch(call js.FunctionCall) js.Value {
	var v js.Value
	r.cx.Batch(func() { v = r.call(arg(call, 0)) })
	return v
}

// untracked is `untracked(fn)`: fn without tracking what it reads.
func (r *runtime) untracked(call js.FunctionCall) js.Value {
	return core.Untracked(r.cx, func() js.Value { return r.call(arg(call, 0)) })
}

// cleanup is `cleanup(fn)`: fn before the current owner runs again or
// goes.
func (r *runtime) cleanup(call js.FunctionCall) js.Value {
	fn := arg(call, 0)
	r.cx.Cleanup(func() { r.call(fn) })
	return js.Undefined()
}

// accessor makes a JS function returning get().
func (r *runtime) accessor(get func() js.Value) *js.Object {
	return r.function(func(js.FunctionCall) js.Value { return get() })
}

// equals is the comparison options name (`{ equals }`): ===, none with
// false, else the app's function.
func (r *runtime) equals(options js.Value) func(a, b js.Value) bool {
	if o, ok := options.(*js.Object); ok {
		switch eq := o.Get("equals"); {
		case eq == nil || js.IsUndefined(eq):
		case isFunction(eq):
			return func(a, b js.Value) bool { return r.call(eq, a, b).ToBoolean() }
		case !eq.ToBoolean():
			return nil
		}
	}
	return strictEquals
}

func strictEquals(a, b js.Value) bool { return a.StrictEquals(b) }
