package tsxtest

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"path"
	"strings"
	"time"

	"github.com/dop251/goja"
	"go.yaml.in/yaml/v3"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/tsx/bundler"
)

// testHost is the embedding app a test plays for an app (open's host, or
// the page's host module of nsgo.yaml's web.host): its channel, and the
// calls and posts the app sent the names the test handles.
type testHost struct {
	h            *core.Host
	calls, posts []map[string]any
}

// playHost is open's host: the params the app reads and the handlers of
// its calls and posts, each a value it answers with or a function of the
// data whose result, or the promise of it, answers. A call of a name not
// listed fails; a post of one is dropped.
func (r *runner) playHost(o *goja.Object, app func() *nsgotest.App) *testHost {
	th := &testHost{h: core.NewHost()}
	if params, ok := o.Get("params").(*goja.Object); ok {
		_ = th.h.SetParams(params.Export().(map[string]any))
	}
	handlers, ok := o.Get("handlers").(*goja.Object)
	if !ok {
		return th
	}
	for _, name := range handlers.Keys() {
		hv := handlers.Get(name)
		th.h.Handle(name, func(c *core.HostCall) {
			msg := map[string]any{"name": c.Name, "data": c.Data}
			if c.IsPost() {
				th.posts = append(th.posts, msg)
			} else {
				th.calls = append(th.calls, msg)
			}
			fn, ok := goja.AssertFunction(hv)
			if !ok {
				c.Resolve(hv.Export())
				return
			}
			res, err := fn(goja.Undefined(), r.vm.ToValue(c.Data))
			if err != nil {
				c.Reject(errors.New(message(err)))
				return
			}
			settle(res, app(), func(v goja.Value, err error) {
				if err != nil {
					c.Reject(err)
					return
				}
				c.Resolve(v.Export())
			})
		})
	}
	return th
}

// settle has done hear v, or what the promise v is settles with: at
// once when it is settled, else as the app settles later, polling it.
func settle(v goja.Value, app *nsgotest.App, done func(goja.Value, error)) {
	p, ok := v.Export().(*goja.Promise)
	if !ok {
		done(v, nil)
		return
	}
	switch p.State() {
	case goja.PromiseStatePending:
		app.Post(func() { settle(v, app, done) })
	case goja.PromiseStateRejected:
		done(nil, errors.New(message(p.Result())))
	default:
		done(p.Result(), nil)
	}
}

// message is what a JS error says: an Error's message, else the value.
func message(v any) string {
	if ex, ok := v.(*goja.Exception); ok {
		v = ex.Value()
	}
	if o, ok := v.(*goja.Object); ok {
		if m := o.Get("message"); m != nil && !goja.IsUndefined(m) {
			return m.String()
		}
	}
	if gv, ok := v.(goja.Value); ok {
		return gv.String()
	}
	return fmt.Sprint(v)
}

// webHost is the page's host module nsgo.yaml beside entry names
// (web.host), relative to fsys; "" without one.
func webHost(fsys fs.FS, entry string) (string, error) {
	dir := path.Dir(entry)
	data, err := fs.ReadFile(fsys, path.Join(dir, "nsgo.yaml"))
	if err != nil {
		return "", nil
	}
	var config struct {
		Web struct {
			Host string `yaml:"host"`
		} `yaml:"web"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("nsgo.yaml: %w", err)
	}
	if config.Web.Host == "" {
		return "", nil
	}
	return path.Join(dir, config.Web.Host), nil
}

// pageHost runs the page's host module at file for app, as its page
// would before the app starts: a VM of its own runs it, and what its
// createHost(player) gives answers the app's calls and posts through h,
// a post going to its post when it has one. player.hostEvent sends the
// app events; what it sends before the app runs reaches the app as it
// starts. The VM has console and timers on the app's clock; imports do
// not resolve, as a headless run cannot load the page's.
func pageHost(fsys fs.FS, file string, a *nsgotest.App, h *core.Host) error {
	src, err := fs.ReadFile(fsys, file)
	if err != nil {
		return fmt.Errorf("nsgo.yaml web.host: %w", err)
	}
	code, err := bundler.Transform(file, string(src))
	if err != nil {
		return fmt.Errorf("nsgo.yaml web.host: %w", err)
	}
	vm := goja.New()
	module, exports := vm.NewObject(), vm.NewObject()
	_ = module.Set("exports", exports)
	_ = vm.Set("module", module)
	_ = vm.Set("exports", exports)
	_ = vm.Set("require", func(name string) goja.Value {
		panic(vm.NewTypeError("nsgo.yaml web.host: %s cannot import %q in a headless run", file, name))
	})
	console := vm.NewObject()
	for _, level := range []string{"log", "info", "debug", "warn", "error"} {
		_ = console.Set(level, func(call goja.FunctionCall) goja.Value {
			parts := make([]string, len(call.Arguments))
			for i, v := range call.Arguments {
				parts[i] = v.String()
			}
			log.Printf("js %s: %s", level, strings.Join(parts, " "))
			return goja.Undefined()
		})
	}
	_ = vm.Set("console", console)
	installTimers(vm, a.Clock)
	if _, err := vm.RunScript(file, code); err != nil {
		return fmt.Errorf("nsgo.yaml web.host: %w", err)
	}
	create, ok := goja.AssertFunction(module.Get("exports").ToObject(vm).Get("createHost"))
	if !ok {
		return fmt.Errorf("nsgo.yaml web.host: %s exports no createHost", file)
	}
	player := vm.NewObject()
	_ = player.Set("hostEvent", func(name string, payload goja.Value) {
		var data any
		if payload != nil && !goja.IsUndefined(payload) {
			data = payload.Export()
		}
		if err := h.Send(name, data); err != nil {
			panic(vm.NewTypeError("the event %s's payload is not JSON", name))
		}
	})
	res, err := create(goja.Undefined(), player)
	if err != nil {
		return fmt.Errorf("nsgo.yaml web.host: createHost: %w", err)
	}
	if p, ok := res.Export().(*goja.Promise); ok {
		if p.State() != goja.PromiseStateFulfilled {
			return fmt.Errorf("nsgo.yaml web.host: createHost's promise did not resolve: %v", p.Result())
		}
		res = p.Result()
	}
	host, _ := res.(*goja.Object)
	if host == nil {
		return errors.New("the app's host could not be loaded")
	}
	call, ok := goja.AssertFunction(host.Get("call"))
	if !ok {
		return errors.New("the app's host could not be loaded")
	}
	post, hasPost := goja.AssertFunction(host.Get("post"))
	h.HandleDefault(func(c *core.HostCall) {
		fn := call
		if c.IsPost() && hasPost {
			fn = post
		}
		v, err := fn(host, vm.ToValue(c.Name), vm.ToValue(c.Data))
		if err != nil {
			c.Reject(errors.New(message(err)))
			return
		}
		settle(v, a, func(v goja.Value, err error) {
			if err != nil {
				c.Reject(err)
				return
			}
			c.Resolve(v.Export())
		})
	})
	return nil
}

// installTimers gives vm setTimeout, setInterval and their clear* on
// clock: they run as the test moves the app's time.
func installTimers(vm *goja.Runtime, clock *core.ManualClock) {
	stops := map[int64]func() bool{}
	var next int64
	set := func(repeat bool) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			fn, ok := goja.AssertFunction(call.Argument(0))
			if !ok {
				panic(vm.NewTypeError("a timer runs a function"))
			}
			d := time.Duration(max(call.Argument(1).ToFloat(), 0) * float64(time.Millisecond))
			if repeat {
				d = max(d, time.Millisecond) // as a page clamps them
			}
			args := call.Arguments[min(2, len(call.Arguments)):]
			next++
			id := next
			var schedule func()
			schedule = func() {
				stops[id] = clock.AfterFunc(d, func() {
					if repeat {
						schedule()
					} else {
						delete(stops, id)
					}
					if _, err := fn(goja.Undefined(), args...); err != nil {
						log.Printf("nsgo.yaml web.host: a timer failed: %v", err)
					}
				})
			}
			schedule()
			return vm.ToValue(id)
		}
	}
	clear := func(id int64) {
		if stop := stops[id]; stop != nil {
			stop()
			delete(stops, id)
		}
	}
	_ = vm.Set("setTimeout", set(false))
	_ = vm.Set("setInterval", set(true))
	_ = vm.Set("clearTimeout", clear)
	_ = vm.Set("clearInterval", clear)
}
