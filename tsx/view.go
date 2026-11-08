package tsx

import (
	"errors"
	"fmt"

	"github.com/oneo-me/nsgo/tsx/internal/js"
	"github.com/oneo-me/nsgo/tsx/pkg"

	"github.com/oneo-me/nsgo/core"
)

// View runs the app of a package a pkg.Trust verified where it is placed
// in a Go app, sandboxed: the app's own screens delivered anew, or a
// plugin. The package's capabilities hold it, narrowed by Grant; it keeps
// a partition of the app's storage of its app ID's, and its host module
// reaches Host alone. A package that throws as it loads, or stops at a
// quota, panics, as any component does; place it under a core.Catch to
// show something else.
//
//	p, err := trust.Verify(data) // off the engine's goroutine
//	...
//	core.Catch{Try: tsx.View{Package: p}.Render, Fallback: failed}
type View struct {
	Package *pkg.Package
	// Sandbox's Capabilities are the package's and its Storage its app
	// ID when unset.
	Sandbox
}

// Render implements core.Node.
func (v View) Render(cx *core.Scope) core.Node {
	if v.Package == nil {
		panic("tsx: a View needs a package")
	}
	return Sandboxed(v.Package.App.ID, v.Package.Source(), v.Sandbox.of(v.Package))(cx)
}

// of is s for package p.
func (s Sandbox) of(p *pkg.Package) Sandbox {
	if s.Capabilities == nil {
		s.Capabilities = &p.Capabilities
	}
	if s.Storage == "" {
		s.Storage = p.App.ID
	}
	return s
}

// Plugin is a package that gives the Go app functions rather than a view:
// what its entry module exports, which Call calls. It runs sandboxed as
// View does, under the scope that loaded it, until Close or that scope's
// end.
//
//	// search.tsx
//	export async function search(query: string) { ... }
//
//	plugin, err := tsx.Load(cx, p, tsx.Sandbox{})
//	plugin.Call("search", []any{"query"}, func(result any, err error) { ... })
type Plugin struct {
	r       *runtime
	cx      *core.Scope
	exports *js.Object
}

// ErrNoExport is the failure of a call of a name the plugin does not
// export as a function.
var ErrNoExport = errors.New("tsx: the plugin exports no such function")

// Load runs package p as a plugin under cx, within s as View runs it, and
// returns it once its entry module ran, or what that threw. Call it on
// the engine's goroutine.
func Load(cx *core.Scope, p *pkg.Package, s Sandbox) (plugin *Plugin, err error) {
	inner := cx.Child()
	defer func() {
		if x := recover(); x != nil {
			inner.Dispose()
			if e, ok := x.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("tsx: the plugin failed as it loaded: %v", x)
			}
		}
	}()
	var r *runtime
	inner.Run(func() { r = s.of(p).start(inner, p.App.ID, p.Source()) })
	plugin = &Plugin{r: r, cx: inner}
	plugin.exports, _ = r.vm.Get(exportsGlobal).(*js.Object)
	return plugin, nil
}

// exportsGlobal is where a bundle leaves its entry module's exports
// (bundler.Compile).
const exportsGlobal = "__nsgo_exports"

// Exports are the names of the functions the plugin exports.
func (p *Plugin) Exports() []string {
	if p.exports == nil {
		return nil
	}
	var out []string
	for _, k := range p.exports.Keys() {
		if isFunction(p.exports.Get(k)) {
			out = append(out, k)
		}
	}
	return out
}

// Call calls the plugin's export name with args, values JSON encodes,
// and has done hear what it returns, or what its promise resolves with,
// as JSON decodes it, on the engine's goroutine; err is what it threw or
// rejected with, ErrNoExport, or why the plugin stopped. Call it on the
// engine's goroutine.
func (p *Plugin) Call(name string, args []any, done func(result any, err error)) {
	r := p.r
	if err := r.stoppedErr(); err != nil || p.cx.Disposed() {
		if err == nil {
			err = errors.New("tsx: the plugin was closed")
		}
		done(nil, err)
		return
	}
	var fn js.Value
	if p.exports != nil {
		fn = p.exports.Get(name)
	}
	if !isFunction(fn) {
		done(nil, fmt.Errorf("%w: %q", ErrNoExport, name))
		return
	}
	var once bool
	finish := func(v any, err error) {
		if !once {
			once = true
			done(v, err)
		}
	}
	p.cx.Run(func() {
		defer func() {
			if x := recover(); x != nil {
				err, ok := x.(error)
				if !ok {
					err = fmt.Errorf("%v", x)
				}
				finish(nil, err)
			}
		}()
		in := make([]js.Value, len(args))
		for i, a := range args {
			in[i] = r.vm.ToValue(a)
		}
		v := r.call(fn, in...)
		if _, ok := v.Export().(*js.Promise); !ok {
			finish(exportJSON(v), nil)
			return
		}
		then, _ := js.AssertFunction(v.(*js.Object).Get("then"))
		onResolve := r.function(func(call js.FunctionCall) js.Value {
			finish(exportJSON(arg(call, 0)), nil)
			return js.Undefined()
		})
		onReject := r.function(func(call js.FunctionCall) js.Value {
			finish(nil, r.rejection(arg(call, 0)))
			return js.Undefined()
		})
		r.within(r.limits.Call, func() {
			if _, err := then(v, onResolve, onReject); err != nil {
				panic(err)
			}
		})
	})
}

// Close stops the plugin and frees what it holds.
func (p *Plugin) Close() { p.cx.Dispose() }

// rejection is what JS threw as a Go error: its message, with its code.
func (r *runtime) rejection(v js.Value) error {
	o, ok := v.(*js.Object)
	if !ok {
		return fmt.Errorf("%v", exportJSON(v))
	}
	msg := o.Get("message")
	if !defined(msg) {
		return fmt.Errorf("%v", v.String())
	}
	if code := o.Get("code"); defined(code) {
		return &core.Error{Code: code.String(), Message: msg.String()}
	}
	return errors.New(msg.String())
}
