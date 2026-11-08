package tsx

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/tsx/internal/js"
	"github.com/oneo-me/nsgo/tsx/pkg"

	"github.com/oneo-me/nsgo/core"
)

// The sandbox (NSH's sandbox.rs and access.rs): what a bundle may use and
// reach, set by the Go code that runs it, never by JS. A bundle holds only
// the app's code, so nothing in JS is trusted to keep them. Core checks
// the capabilities where requests are made (core.Access), the runtime
// where only it makes them: fetch, the clipboard, the pickers, speech.

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
	// Nodes is how many elements the bundle may hold at once, each of
	// which may hold a native view; past it the bundle stops.
	Nodes int
	// Timers is how many timers and frame callbacks may wait at once, and
	// Requests how many fetches may be under way: one more fails.
	Timers, Requests int
	// Body is the most bytes a request's or a response's body may hold.
	Body int
	// Storage is the most bytes its partition of the app's storage may
	// hold (Sandbox.Storage).
	Storage int
}

// DefaultLimits are NSH's: 10 seconds to load, 2 per call, 1000 frames,
// hardened, 200 000 nodes; and 10 000 timers, 32 requests, 64 MiB
// bodies and 1 MiB of storage.
var DefaultLimits = Limits{Load: 10 * time.Second, Call: 2 * time.Second, Stack: 1000, Harden: true,
	Nodes: 200_000, Timers: 10_000, Requests: 32, Body: 64 << 20, Storage: 1 << 20}

// ErrBudget is what a call cut at its time budget fails with.
var ErrBudget = errors.New("tsx: the script ran past its time budget")

// ParseLimits reads limits as nsgo dev -limits writes them: DefaultLimits
// with the fields spec names set, as "nodes=1000,call=500ms". Its keys
// are load, call, stack, nodes, timers, requests, body and storage;
// "default" or "" is DefaultLimits.
func ParseLimits(spec string) (Limits, error) {
	l := DefaultLimits
	if spec == "" || spec == "default" {
		return l, nil
	}
	for _, kv := range strings.Split(spec, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok {
			return l, fmt.Errorf("tsx: limit %q is not key=value", kv)
		}
		var err error
		switch k {
		case "load", "call":
			var d time.Duration
			if d, err = time.ParseDuration(v); err == nil {
				if k == "load" {
					l.Load = d
				} else {
					l.Call = d
				}
			}
		case "stack", "nodes", "timers", "requests", "body", "storage":
			var n int
			if n, err = strconv.Atoi(v); err == nil && n < 0 {
				err = errors.New("is negative")
			}
			*map[string]*int{"stack": &l.Stack, "nodes": &l.Nodes, "timers": &l.Timers, "requests": &l.Requests,
				"body": &l.Body, "storage": &l.Storage}[k] = n
		default:
			return l, fmt.Errorf("tsx: no limit %q: load, call, stack, nodes, timers, requests, body or storage", k)
		}
		if err != nil {
			return l, fmt.Errorf("tsx: limit %s: %v", k, err)
		}
	}
	return l, nil
}

// Sandbox is what a bundle runs within: its limits, what it may reach and
// import, the host it talks to and the storage it keeps.
type Sandbox struct {
	// Limits are what it may use, DefaultLimits when nil.
	Limits *Limits
	// Capabilities are what its code may reach (a package's own); nil
	// reaches what the app's code does. Grant, when set, narrows them to
	// what it also allows: what the host gives this view.
	Capabilities, Grant *pkg.Capabilities
	// Modules are what it may import beyond SandboxModules: a module
	// ("myapp"), one export ("nsgo/base.Terminal"), a native component
	// ("native:MapView"), or "*" for everything.
	Modules []string
	// Host is the channel its host module reaches, for the Go app to
	// answer its calls and send it events; nil gives it none.
	Host *core.Host
	// Storage names its partition of the app's storage, which no one
	// else's code sees; "" leaves it the app's storage.
	Storage string
}

// SandboxModules are the modules a sandboxed bundle imports without
// Sandbox.Modules naming them: NSGo's own, with the Go components
// registered into them but for those registered Unsandboxed, and none of
// the Go functions registered with RegisterFunctions.
var SandboxModules = []string{"nsgo", "nsgo:rt", "nsgo/base", "nsgo/base/plot", "nsgo/ui", "nsgo/ui/chat", "nsgo/mobile"}

// Sandboxed returns the app of a bundle, as App does, run within s.
func Sandboxed(name, src string, s Sandbox) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		inner := cx.Child()
		var root core.Node
		inner.Run(func() { root = s.start(inner, name, src).root.elem })
		return root
	}
}

// start runs src in a new runtime under cx, panicking with what it
// throws as it loads.
func (s Sandbox) start(cx *core.Scope, name, src string) *runtime {
	limits := DefaultLimits
	if s.Limits != nil {
		limits = *s.Limits
	}
	for _, c := range []*pkg.Capabilities{s.Capabilities, s.Grant} {
		if c != nil {
			cx.ProvideAccess(access{c})
		}
	}
	cx.ProvideHost(s.Host)
	if s.Storage != "" {
		cx.ProvideStorage(s.Storage, limits.Storage)
	}
	r := newRuntime(cx)
	r.limits = limits
	r.sandbox = &sandboxed{caps: s.Capabilities != nil || s.Grant != nil, modules: s.Modules}
	if limits.Stack > 0 {
		r.vm.SetMaxCallStackSize(limits.Stack)
	}
	if limits.Harden {
		r.harden()
	}
	r.within(limits.Load, func() {
		if _, err := r.vm.RunScript(name, src); err != nil {
			panic(r.failure(err))
		}
	})
	return r
}

// failure is what a call into JS failing with err fails with: why the
// runtime stopped when it did, whatever JS made of it.
func (r *runtime) failure(err error) error {
	if stopped := r.stoppedErr(); stopped != nil {
		return stopped
	}
	return err
}

// access is capabilities as core.Access.
type access struct{ c *pkg.Capabilities }

func (a access) Allows(req core.AccessRequest) bool {
	return a.c.Allows(req.Permission, req.Resource, req.Method)
}

func (a access) WebHosts() []string { return a.c.WebHosts() }

// sandboxed is a sandboxed runtime's state.
type sandboxed struct {
	// caps says capabilities hold it, so the runtime checks its own
	// requests.
	caps    bool
	modules []string
	// nodes, timers and requests are what it holds now.
	nodes, timers, requests int
	// stopped is why it stopped: no JS runs after.
	stopped error
}

// importsModule reports whether the bundle may import module: one of
// SandboxModules, or one Modules names, whole or by an export.
func (s *sandboxed) importsModule(module string) bool {
	if slices.Contains(SandboxModules, module) {
		return true
	}
	return slices.ContainsFunc(s.modules, func(m string) bool {
		return m == "*" || m == module || strings.HasPrefix(m, module+".")
	})
}

// importsExport reports whether the bundle may import an export of
// module that is not sandboxed: one Modules names, or any of a module it
// names that is not one of SandboxModules.
func (s *sandboxed) importsExport(module, name string) bool {
	return slices.ContainsFunc(s.modules, func(m string) bool {
		return m == "*" || m == module+"."+name || m == module && !slices.Contains(SandboxModules, module)
	})
}

// importsNative reports whether the bundle may show the embedding app's
// native component name.
func (s *sandboxed) importsNative(name string) bool {
	return slices.Contains(s.modules, "*") || slices.Contains(s.modules, "native:"+name)
}

// checkAccess is the refusal of req, for the requests the runtime makes.
func (r *runtime) checkAccess(req core.AccessRequest) error {
	if r.sandbox == nil || !r.sandbox.caps {
		return nil
	}
	return r.cx.CheckAccess(req)
}

// quota is the failure of a quota of what used up.
func quota(what string, limit int) error {
	return &core.Error{Code: core.CodeQuotaExceeded, Resource: what,
		Message: fmt.Sprintf("tsx: the sandbox's quota of %d %s is used up", limit, what)}
}

// stop stops the runtime for err: the JS running is cut and none runs
// after, while the views stay as they are. What runs it panics with err
// into the nearest core.Catch.
func (r *runtime) stop(err error) {
	if stopped := r.sandbox.stopped; stopped != nil {
		if js.Browser {
			panic(r.goError(stopped))
		}
		return
	}
	r.sandbox.stopped = err
	for _, stop := range r.timers.stop {
		stop()
	}
	clear(r.timers.stop)
	r.vm.Interrupt(err)
	if js.Browser {
		// The page's engine cannot be interrupted: a throw unwinds it.
		panic(r.goError(err))
	}
}

// stoppedErr is why the runtime stopped, nil while it runs.
func (r *runtime) stoppedErr() error {
	if r.sandbox == nil {
		return nil
	}
	return r.sandbox.stopped
}

// newCoreElement makes a core element owned by cx, counted against the node
// quota.
func (r *runtime) newCoreElement(cx *core.Scope, props core.Node) *core.Element {
	if s := r.sandbox; s != nil && r.limits.Nodes > 0 {
		if s.nodes >= r.limits.Nodes || s.stopped != nil {
			r.stop(quota("nodes", r.limits.Nodes))
		}
		s.nodes++
		cx.Cleanup(func() { s.nodes-- })
	}
	return core.NewElement(cx, props)
}

// within runs fn, a call into JS from Go, cut at budget when it is the
// outermost. A stopped runtime runs nothing.
func (r *runtime) within(budget time.Duration, fn func()) {
	if r.stoppedErr() != nil {
		return
	}
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
