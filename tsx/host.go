package tsx

import (
	"github.com/dop251/goja"
)

// The host channel (NSH's host_channel.rs): host reaches the app that
// embeds this one through core's channel ([embedding]): calls answered
// by its handlers, posts, events and the parameters it started the app
// with. Values cross as JSON.
//
// [embedding]: docs/architecture/embedding.md

func (r *runtime) host() goja.Value {
	o := r.vm.NewObject()
	set := func(name string, f func(call goja.FunctionCall) goja.Value) { _ = o.Set(name, r.function(f)) }
	set("available", func(goja.FunctionCall) goja.Value { return r.vm.ToValue(r.cx.HasHost()) })
	_ = o.DefineAccessorProperty("params", r.function(func(goja.FunctionCall) goja.Value {
		return r.vm.ToValue(r.cx.HostParams())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	set("call", func(call goja.FunctionCall) goja.Value {
		p, resolve, reject := r.vm.NewPromise()
		name, data := arg(call, 0).String(), exportJSON(arg(call, 1))
		done := func(result any, err error) {
			r.inJS(func() {
				if err != nil {
					_ = reject(r.vm.NewGoError(err))
					return
				}
				_ = resolve(r.vm.ToValue(result))
			})
		}
		if t := r.options(arg(call, 2)).get("timeout"); defined(t) {
			if t.ToFloat() < 0 {
				_ = reject(r.vm.NewTypeError("nsgo: host.call's timeout is negative"))
				return r.vm.ToValue(p)
			}
			r.cx.CallHostTimeout(name, data, millis(t.ToFloat()), done)
		} else {
			r.cx.CallHost(name, data, done)
		}
		return r.vm.ToValue(p)
	})
	set("post", func(call goja.FunctionCall) goja.Value {
		r.cx.PostHost(arg(call, 0).String(), exportJSON(arg(call, 1)))
		return goja.Undefined()
	})
	set("on", func(call goja.FunctionCall) goja.Value {
		name, listener := arg(call, 0).String(), arg(call, 1)
		owner := r.cx.Child()
		owner.Run(func() {
			owner.OnHostEvent(name, func(data any) { r.call(listener, r.vm.ToValue(data)) })
		})
		remove := r.function(func(goja.FunctionCall) goja.Value {
			owner.Dispose()
			return goja.Undefined()
		})
		_ = remove.Set("remove", remove)
		return remove
	})
	return o
}

// exportJSON is a JS value as the host channel's JSON takes it.
func exportJSON(v goja.Value) any {
	if !defined(v) {
		return nil
	}
	return v.Export()
}
