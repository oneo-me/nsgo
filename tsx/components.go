package tsx

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/core"
)

// The rest of NSH's renderer (dom.rs): refs, spreads, the primitives as
// components, ErrorBoundary and context.

// ref is `ref(target, value, name)`: a signal is set to the value, a
// node or a component's own ref, and a function called with it,
// reporting true; a name means the target is a binding that cannot be
// assigned, so it must be one of those.
func (r *runtime) ref(call js.FunctionCall) js.Value {
	target, name := arg(call, 0), arg(call, 2)
	return r.vm.ToValue(r.applyRefValue(target, arg(call, 1), stringOf(name)))
}

// applyRef gives target node n: a signal's set, or a call.
func (r *runtime) applyRef(target js.Value, n *node, name string) bool {
	return r.applyRefValue(target, n.value(), name)
}

// applyRefValue gives target v: a signal's set, or a call.
func (r *runtime) applyRefValue(target, v js.Value, name string) bool {
	if o, ok := target.(*js.Object); ok && isFunction(o) {
		if set := o.Get("set"); isFunction(set) {
			r.call(set, v)
		} else {
			r.call(o, v)
		}
		return true
	}
	if name != "" {
		panic(r.vm.NewTypeError("nsgo: ref={%s}: %s holds no signal or function to give the node", name, name))
	}
	return false
}

// spread is `spread(node, props)`: each prop set, followed as it
// changes.
func (r *runtime) spread(call js.FunctionCall) js.Value {
	r.spreadProps(r.mustNode(arg(call, 0)), arg(call, 1), "")
	return js.Undefined()
}

// spreadProps sets props' props on n but skip, again when what they read
// changes; an unchanged value is not set again.
func (r *runtime) spreadProps(n *node, props js.Value, skip string) {
	o, ok := props.(*js.Object)
	if !ok {
		return
	}
	prev := map[string]js.Value{}
	r.cx.RenderEffect(func() {
		for _, k := range o.Keys() {
			if k == skip {
				continue
			}
			v := o.Get(k)
			if old, ok := prev[k]; ok && old.StrictEquals(v) {
				continue
			}
			prev[k] = v
			r.setProp(n, k, v)
		}
	})
}

// hostComponent is a primitive as a component (`const C = c ? View :
// Text; <C />`): its element with the props spread and the children
// inserted.
func (r *runtime) hostComponent(tag string) js.Value {
	return r.function(func(call js.FunctionCall) js.Value {
		n := r.newElement(tag)
		props := arg(call, 0)
		r.spreadProps(n, props, "children")
		if o, ok := props.(*js.Object); ok {
			r.insertValue(n, r.accessor(func() js.Value { return o.Get("children") }), nil)
		}
		return n.value()
	})
}

// errorBoundary is `ErrorBoundary({ fallback, children })`: its children,
// or the fallback once something under them threw (core's Catch);
// fallback(error, reset) is called with what was thrown.
func (r *runtime) errorBoundary(call js.FunctionCall) js.Value {
	props := arg(call, 0).ToObject(r.vm)
	n := &node{r: r, kind: kindRegion, elem: r.newCoreElement(r.cx, nil)}
	region := func(value js.Value) core.Node {
		inner := &node{r: r, kind: kindRegion, elem: r.newCoreElement(r.cx, nil)}
		r.insertValue(inner, value, nil)
		return inner.elem
	}
	n.elem.Mount(core.Catch{
		Try: func(*core.Scope) core.Node {
			return region(r.accessor(func() js.Value { return props.Get("children") }))
		},
		Fallback: func(_ *core.Scope, err error, reset func()) core.Node {
			fb := props.Get("fallback")
			if isFunction(fb) {
				fb = r.call(fb, r.thrown(err), r.function(func(js.FunctionCall) js.Value {
					reset()
					return js.Undefined()
				}))
			}
			return region(fb)
		},
	})
	return n.value()
}

// thrown is what JS threw that err carries, else err's message as an
// Error.
func (r *runtime) thrown(err error) js.Value {
	var ex *js.Exception
	if errors.As(err, &ex) {
		return ex.Value()
	}
	var p *core.PanicError
	if errors.As(err, &p) {
		if v, ok := p.Value.(js.Value); ok {
			return v
		}
	}
	return r.goError(err)
}

// contextDefault keys a context's default, which a hot update replaces
// in the context it keeps.
var contextDefault = js.NewSymbol("nsgo.contextDefault")

// context is `context(default)`: a function reading the nearest value
// provided, with a Provider component (`<Ctx.Provider value>`).
func (r *runtime) context(call js.FunctionCall) js.Value {
	key := core.NewKey[js.Value]("context", nil)
	var ctx *js.Object
	ctx = r.function(func(js.FunctionCall) js.Value {
		if v := key.Use(r.cx); v != nil {
			return v
		}
		return ctx.GetSymbol(contextDefault)
	})
	_ = ctx.DefineDataPropertySymbol(contextDefault, arg(call, 0), js.FLAG_TRUE, js.FLAG_FALSE, js.FLAG_FALSE)
	_ = ctx.Set("Provider", r.function(func(call js.FunctionCall) js.Value {
		props := arg(call, 0).ToObject(r.vm)
		owner := r.cx.Child()
		var out js.Value
		owner.Run(func() {
			key.Provide(owner, props.Get("value"))
			// The children are made under the value, again as they change.
			m := core.NewMemoFunc(owner, func() js.Value { return props.Get("children") }, nil)
			out = r.accessor(m.Get)
		})
		return out
	}))
	return ctx
}

// keepContext is `keepContext(kept, made)`, for hot updates: kept, the
// context its module made first, takes the default of made, the one it
// made again, and stays the context, so providers and readers stay
// connected.
func (r *runtime) keepContext(call js.FunctionCall) js.Value {
	kept, made := arg(call, 0).ToObject(r.vm), arg(call, 1).ToObject(r.vm)
	_ = kept.DefineDataPropertySymbol(contextDefault, made.GetSymbol(contextDefault), js.FLAG_TRUE, js.FLAG_FALSE, js.FLAG_FALSE)
	return kept
}

// loadChunk is `loadChunk(key)`, for a split bundle's import(): a promise
// settled once the chunk of that key ran, which defines its modules.
func (r *runtime) loadChunk(call js.FunctionCall) js.Value {
	key := arg(call, 0).String()
	p, resolve, reject := r.vm.NewPromise()
	if r.chunk == nil {
		_ = reject(r.vm.NewTypeError("nsgo: the app has no chunk %s", key))
		return r.vm.ToValue(p)
	}
	go func() {
		code, err := r.chunk(key)
		r.cx.Post(func() {
			r.inJS(func() {
				if err == nil {
					_, err = r.vm.RunScript(strings.TrimPrefix(key, "app:"), code)
				}
				if err != nil {
					_ = reject(r.goError(fmt.Errorf("nsgo: loading %s: %w", key, err)))
					return
				}
				_ = resolve(js.Undefined())
			})
		})
	}()
	return r.vm.ToValue(p)
}

// splitSource is `split(props, keys)`: [picked, rest], getters kept.
const splitSource = `(function (props, keys) {
	const picked = {}, rest = {};
	const all = Object.getOwnPropertyDescriptors(props);
	for (const k of Object.keys(all)) Object.defineProperty(keys.includes(k) ? picked : rest, k, all[k]);
	return [picked, rest];
})`
