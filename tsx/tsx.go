// Package tsx runs TSX apps: bundles of what tsx/bundler emits, on the
// goja JavaScript engine, or the page's own on the web (internal/js),
// rendered by core as Go apps are. It is a port
// of NSH's nsh-js: reactive.go of reactive.rs, dom.go of dom.rs,
// protocol.go of protocol.rs and modules.go of modules.rs, so a change
// there ports here (docs/architecture/tsx.md).
//
// A bundle is a script: imports of NSGo's modules (nsgo, nsgo/base,
// nsgo:rt) are calls of require.
package tsx

import (
	"fmt"
	"log"
	"time"

	"github.com/oneo-me/nsgo/tsx/internal/js"

	"github.com/oneo-me/nsgo/core"
)

// App returns the app of a bundle, for nsgo.Run and nsgotest.Start: it
// runs the bundle, and what the bundle mounts is the app. name names the
// bundle in stacks.
func App(name, src string) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		r := newRuntime(cx)
		if _, err := r.vm.RunScript(name, src); err != nil {
			panic(err)
		}
		return r.root.elem
	}
}

// SplitApp returns the app of a split bundle (bundler.CompileSplit), as
// App does: an import() of one of its chunks calls chunk, on a goroutine
// of its own, for the chunk's script by its key, and runs it.
func SplitApp(name, src string, chunk func(key string) (string, error)) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		r := newRuntime(cx)
		r.chunk = chunk
		if _, err := r.vm.RunScript(name, src); err != nil {
			panic(err)
		}
		return r.root.elem
	}
}

// Hot returns the app of a hot bundle (bundler.CompileHot), as App
// does, and what runs a newer one in the same VM: the modules whose code
// changed run again and their components' instances render again, while
// the rest of the app keeps its state. update runs on the engine
// goroutine, as from Scope.Post.
func Hot(name, src string) (app func(cx *core.Scope) core.Node, update func(src string)) {
	var r *runtime
	app = func(cx *core.Scope) core.Node {
		r = newRuntime(cx)
		if _, err := r.vm.RunScript(name, src); err != nil {
			panic(err)
		}
		return r.root.elem
	}
	update = func(src string) {
		if r == nil {
			return
		}
		r.cx.Run(func() {
			r.within(r.limits.Load, func() {
				if _, err := r.vm.RunScript(name, src); err != nil {
					panic(err)
				}
			})
		})
	}
	return app, update
}

// runtime is one VM and what NSH keeps per context: the module table,
// the graph's scope and the renderer's root.
type runtime struct {
	vm      *js.Runtime
	cx      *core.Scope
	root    *node
	modules map[string]*js.Object
	// warned holds the warnings given, each given once.
	warned map[string]bool
	timers timers
	anim   animClasses
	// assets are the data: URLs of the asset: URLs a bundle ships.
	assets map[string]string
	// blobs are the blob: URLs URL.createObjectURL made, until revoked.
	blobs   map[string]blobEntry
	blobSeq int
	// canvasClasses are Path2D and CanvasGradient.
	canvasClasses canvasClasses
	// platforms are the platform objects (ios={{…}}) that apply here.
	platforms []string
	// web is what web.js gives back; base is the URL relative URLs
	// resolve against.
	web  *js.Object
	base string
	// limits are the sandbox's; depth counts the calls into JS under way,
	// the outermost of which has the time budget.
	limits Limits
	depth  int
	// sandbox is a sandboxed runtime's state (Sandbox), nil for the
	// app's own code.
	sandbox *sandboxed
	// origin is when performance.now() and frames' times count from.
	origin time.Time
	// classes are the Go classes made in this VM; posting counts the
	// async calls converting their arguments.
	classes vmClasses
	posting int
	// chunk loads a split bundle's chunks (SplitApp).
	chunk func(key string) (string, error)
	// committing is set while a commit is asked for after the page's
	// engine called Go (engine_browser.go).
	committing bool
}

// maxCallStack is how deep JS calls nest in any app.
const maxCallStack = 10000

// continuations runs promise reactions untracked (goja's
// AsyncContextTracker).
type continuations struct {
	cx     *core.Scope
	resume []func()
}

func (c *continuations) Grab() any { return nil }

func (c *continuations) Resumed(any) { c.resume = append(c.resume, c.cx.PauseTracking()) }

func (c *continuations) Exited() {
	n := len(c.resume) - 1
	c.resume[n]()
	c.resume = c.resume[:n]
}

func newRuntime(cx *core.Scope) *runtime {
	r := &runtime{vm: js.New(), cx: cx, modules: map[string]*js.Object{}, warned: map[string]bool{}, origin: cx.Now()}
	r.root = &node{r: r, kind: kindRegion, elem: core.NewElement(cx, nil)}
	r.vm.SetFieldNameMapper(js.UncapFieldNameMapper())
	// A runaway recursion throws a RangeError rather than overflowing the
	// goroutine's stack, which would end the process; Sandboxed's limits
	// set it lower.
	r.vm.SetMaxCallStackSize(maxCallStack)
	// Date reads the app's clock, a test's manual one included.
	r.vm.SetTimeSource(cx.WallTime)
	// Promise continuations run as the outermost JS call returns, which
	// may be an effect's: what they read is not the effect's, as in NSH,
	// whose VM runs them after the effect.
	r.vm.SetAsyncContextTracker(&continuations{cx: cx})
	r.attachEngine()
	r.vm.Set("require", r.require)
	r.installGlobals()
	r.installTimers()
	r.installAnimated()
	r.installCanvas()
	r.base = "asset:/"
	r.installWeb()
	return r
}

// require returns a module's exports, made on first use.
func (r *runtime) require(name string) js.Value {
	if m, ok := r.modules[name]; ok {
		return m
	}
	exports, ok := moduleTable[name]
	if more, has := registeredExports(name); has {
		exports, ok = append(append([]export(nil), exports...), more...), true
	}
	if !ok {
		panic(r.vm.NewTypeError("nsgo: no module %q", name))
	}
	if r.sandbox != nil && !r.sandbox.importsModule(name) {
		panic(r.vm.NewTypeError("nsgo: module %q is not given to this sandbox (Sandbox.Modules)", name))
	}
	m := r.vm.NewObject()
	for _, e := range exports {
		if r.sandbox != nil && isUnsandboxed(name, e.name) && !r.sandbox.importsExport(name, e.name) {
			continue
		}
		_ = m.Set(e.name, e.make(r))
	}
	r.addStatics(name, m)
	r.modules[name] = m
	return m
}

// warn logs a warning once.
func (r *runtime) warn(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	if !r.warned[s] {
		r.warned[s] = true
		log.Print("nsgo: " + s)
	}
}

// call calls fn, panicking with what it throws: back into JS when JS
// called the Go code calling it, else to core, which reports it or
// shows the nearest boundary.
func (r *runtime) call(fn js.Value, args ...js.Value) js.Value {
	f, ok := js.AssertFunction(fn)
	if !ok {
		panic(r.vm.NewTypeError("nsgo: %s is not a function", fn))
	}
	v := js.Undefined()
	var err error
	r.within(r.limits.Call, func() { v, err = f(js.Undefined(), args...) })
	if err != nil {
		panic(r.failure(err))
	}
	return v
}

// function makes a JS function of fn.
func (r *runtime) function(fn func(call js.FunctionCall) js.Value) *js.Object {
	return r.vm.ToValue(fn).(*js.Object)
}

func isFunction(v js.Value) bool {
	_, ok := js.AssertFunction(v)
	return ok
}

// arg returns argument i, undefined when missing.
func arg(call js.FunctionCall, i int) js.Value { return call.Argument(i) }
