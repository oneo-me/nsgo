package tsx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	goruntime "runtime"
	"slices"
	"strings"
	"sync"
	"weak"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/core"
)

// Go classes and calls in TSX (NSH's classes.rs and call.rs, on
// docs/dev/rust-classes.md): a Go type a TSX app constructs with new and
// uses through its methods, getters, setters and statics, its instances
// keeping their state in Go; and how JS calls Go, a class's members and
// registered functions alike.
//
//   - A call whose Go function takes a context.Context first is async: it
//     runs on a goroutine of its own and JS gets a promise, settled on the
//     engine goroutine. The context ends with the app. Every other call
//     runs in the JS call and returns its value.
//   - A trailing error result throws, or rejects; a panic does the same.
//   - A JS function given to a synchronous call runs in place when Go
//     calls it. One given to an async call runs on the engine goroutine:
//     a func without results is posted, NSH's Callback::post, and one
//     with results waits for them, and for a promise it returns to
//     settle, NSH's call_async.

// Class says how RegisterClass shows a Go type T to TSX.
type Class[T any] struct {
	// New, a func returning a *T and optionally an error, is the
	// constructor. Without it JS gets instances only from members and
	// functions that return them.
	New any
	// Getters are methods of *T without parameters read as properties,
	// in Go's names; a method SetX beside the getter X sets X.
	Getters []string
	// Statics are functions on the class, by their JS names.
	Statics map[string]any
}

// RegisterClass makes *T the class name, importable from module in TSX:
// its exported methods are the class's methods, under their names in
// lower camel case, but for its getters and their setters, and Close,
// which dispose() calls. JS holds a *T for each instance, the same
// object while it lives, and dispose() lets it go, after which the
// object throws. A member's or function's parameter of type *T takes an
// instance and a result of type *T gives one. Register classes before an
// app starts.
func RegisterClass[T any](module, name string, c Class[T]) {
	t := reflect.TypeFor[*T]()
	def := &classDef{name: name, module: module, t: t, getters: map[string]reflect.Method{}, setters: map[string]reflect.Method{},
		key: func(p any) any { return weak.Make(p.(*T)) }}
	if c.New != nil {
		def.ctor = reflect.ValueOf(c.New)
		if def.ctor.Kind() != reflect.Func || def.ctor.Type().NumOut() == 0 || def.ctor.Type().Out(0) != t {
			panic(fmt.Sprintf("tsx: RegisterClass: %s.New does not return a %v", name, t))
		}
	}
	for _, g := range c.Getters {
		m, ok := t.MethodByName(g)
		if !ok || m.Type.NumIn() != 1 || m.Type.NumOut() == 0 {
			panic(fmt.Sprintf("tsx: RegisterClass: %s.%s is no getter", name, g))
		}
		def.getters[g] = m
		if s, ok := t.MethodByName("Set" + g); ok {
			def.setters[g] = s
		}
	}
	for i := range t.NumMethod() {
		m := t.Method(i)
		if _, ok := def.getters[m.Name]; ok || m.Name == "Close" {
			continue
		}
		if g, ok := strings.CutPrefix(m.Name, "Set"); ok && def.setters[g].Name != "" {
			continue
		}
		def.methods = append(def.methods, m)
	}
	for k, fn := range c.Statics {
		v := reflect.ValueOf(fn)
		if v.Kind() != reflect.Func {
			panic(fmt.Sprintf("tsx: RegisterClass: %s.%s is no func", name, k))
		}
		def.statics = append(def.statics, namedFunc{k, v})
	}
	slices.SortFunc(def.statics, func(a, b namedFunc) int { return strings.Compare(a.name, b.name) })
	registryMu.Lock()
	defer registryMu.Unlock()
	classes[t] = def
	registeredClasses[module] = append(registeredClasses[module], def)
	registered[module] = append(registered[module], export{name, func(r *runtime) js.Value { return r.class(def).ctor }})
}

// classDef is a registered class.
type classDef struct {
	name, module string
	t            reflect.Type // *T
	ctor         reflect.Value
	methods      []reflect.Method
	getters      map[string]reflect.Method
	setters      map[string]reflect.Method
	statics      []namedFunc
	// key is an instance's key in a VM's table of objects, which does not
	// keep it.
	key func(any) any
}

type namedFunc struct {
	name string
	fn   reflect.Value
}

var (
	// classes are the registered classes by their pointer types.
	classes = map[reflect.Type]*classDef{}
	// registeredClasses are each module's, for its declarations.
	registeredClasses = map[string][]*classDef{}
)

func classOf(t reflect.Type) *classDef {
	registryMu.Lock()
	defer registryMu.Unlock()
	return classes[t]
}

// classObj is a class in one VM.
type classObj struct {
	ctor, proto *js.Object
}

// vmClasses are a VM's classes and the objects of their instances.
type vmClasses struct {
	made map[*classDef]*classObj
	slot *js.Symbol
	mu   sync.Mutex
	objs map[any]weak.Pointer[js.Object]
}

// class is def in this VM, made on first use.
func (r *runtime) class(def *classDef) *classObj {
	if c, ok := r.classes.made[def]; ok {
		return c
	}
	if r.classes.made == nil {
		r.classes.made = map[*classDef]*classObj{}
		r.classes.objs = map[any]weak.Pointer[js.Object]{}
		r.classes.slot = js.NewSymbol("nsgo.instance")
	}
	ctor := r.vm.ToValue(func(call js.ConstructorCall) *js.Object {
		if !def.ctor.IsValid() {
			panic(r.vm.NewTypeError("%s has no constructor: Go makes its instances", def.name))
		}
		ft := def.ctor.Type()
		in := r.goArgs(ft, 0, call.Arguments)
		out := r.guard(def.name, func() []reflect.Value { return callIn(def.ctor, in) })
		if err := trailingErr(ft, out); err != nil {
			panic(r.goError(err))
		}
		if out[0].IsNil() {
			panic(r.vm.NewTypeError("%s: the constructor gave no instance", def.name))
		}
		r.bind(def, call.This, out[0])
		return call.This
	}).(*js.Object)
	proto := ctor.Get("prototype").(*js.Object)
	c := &classObj{ctor: ctor, proto: proto}
	r.classes.made[def] = c
	for _, m := range def.methods {
		name := def.name + "." + lowerCamel(m.Name)
		_ = proto.Set(lowerCamel(m.Name), r.function(func(call js.FunctionCall) js.Value {
			return r.callGo(name, r.receiver(def, call.This, name).Method(m.Index), call.Arguments)
		}))
	}
	getters := make([]string, 0, len(def.getters))
	for g := range def.getters {
		getters = append(getters, g)
	}
	slices.Sort(getters)
	for _, g := range getters {
		name, get := def.name+"."+lowerCamel(g), def.getters[g]
		var setter js.Value
		if s, ok := def.setters[g]; ok {
			setter = r.function(func(call js.FunctionCall) js.Value {
				return r.callGo(name, r.receiver(def, call.This, name).Method(s.Index), call.Arguments)
			})
		}
		_ = proto.DefineAccessorProperty(lowerCamel(g), r.function(func(call js.FunctionCall) js.Value {
			return r.callGo(name, r.receiver(def, call.This, name).Method(get.Index), nil)
		}), setter, js.FLAG_FALSE, js.FLAG_TRUE)
	}
	dispose := r.function(func(call js.FunctionCall) js.Value {
		o, ok := call.This.(*js.Object)
		if !ok {
			return js.Undefined()
		}
		if p, ok := o.GetSymbol(r.classes.slot).Export().(*goInstance); ok {
			_ = o.SetSymbol(r.classes.slot, js.Undefined())
			r.classes.mu.Lock()
			delete(r.classes.objs, def.key(p.v.Interface()))
			r.classes.mu.Unlock()
			if c, ok := p.v.Interface().(io.Closer); ok {
				if err := c.Close(); err != nil {
					panic(r.goError(err))
				}
			}
		}
		return js.Undefined()
	})
	_ = proto.Set("dispose", dispose)
	// esbuild's lowering of using reads Symbol.dispose, else this.
	sym := r.vm.GlobalObject().Get("Symbol").ToObject(r.vm)
	disposeKey := sym.Get("dispose")
	if disposeKey == nil || js.IsUndefined(disposeKey) {
		disposeKey = r.call(sym.Get("for"), r.vm.ToValue("Symbol.dispose"))
	}
	_ = proto.DefineDataPropertySymbol(disposeKey.(*js.Symbol), dispose, js.FLAG_TRUE, js.FLAG_FALSE, js.FLAG_TRUE)
	for _, s := range def.statics {
		name := def.name + "." + s.name
		_ = ctor.Set(s.name, r.function(func(call js.FunctionCall) js.Value {
			return r.callGo(name, s.fn, call.Arguments)
		}))
	}
	return c
}

// goInstance is what an object's slot holds: a *T, as a value goja does
// not reflect on.
type goInstance struct{ v reflect.Value }

// bind makes o the object of instance v.
func (r *runtime) bind(def *classDef, o *js.Object, v reflect.Value) {
	_ = o.DefineDataPropertySymbol(r.classes.slot, r.vm.ToValue(&goInstance{v}), js.FLAG_TRUE, js.FLAG_FALSE, js.FLAG_FALSE)
	key := def.key(v.Interface())
	r.classes.mu.Lock()
	r.classes.objs[key] = weak.Make(o)
	r.classes.mu.Unlock()
	goruntime.AddCleanup(o, func(key any) {
		r.classes.mu.Lock()
		if w, ok := r.classes.objs[key]; ok && w.Value() == nil {
			delete(r.classes.objs, key)
		}
		r.classes.mu.Unlock()
	}, key)
}

// instanceObject is v's object, the one it has while that lives.
func (r *runtime) instanceObject(def *classDef, v reflect.Value) js.Value {
	if v.IsNil() {
		return js.Null()
	}
	c := r.class(def)
	r.classes.mu.Lock()
	w, ok := r.classes.objs[def.key(v.Interface())]
	r.classes.mu.Unlock()
	if ok {
		if o := w.Value(); o != nil {
			return o
		}
	}
	o := r.vm.NewObject()
	_ = o.SetPrototype(c.proto)
	r.bind(def, o, v)
	return o
}

// instanceOf is the *T v holds, or a TypeError naming what.
func (r *runtime) instanceOf(def *classDef, v js.Value, what string) reflect.Value {
	if o, ok := v.(*js.Object); ok && r.classes.slot != nil {
		if s := o.GetSymbol(r.classes.slot); s != nil {
			if p, ok := s.Export().(*goInstance); ok && p.v.Type() == def.t {
				return p.v
			}
			if s == js.Undefined() && o.Get("dispose") != nil {
				panic(r.vm.NewTypeError("%s: %s is disposed", what, def.name))
			}
		}
	}
	panic(r.vm.NewTypeError("%s: expected a %s, got %s", what, def.name, describe(v)))
}

// receiver is the instance a member was called on.
func (r *runtime) receiver(def *classDef, this js.Value, member string) reflect.Value {
	return r.instanceOf(def, this, member+": this")
}

func describe(v js.Value) string {
	if !defined(v) {
		return v.String()
	}
	if o, ok := v.(*js.Object); ok {
		if c, ok := o.Get("constructor").(*js.Object); ok {
			if n := c.Get("name"); defined(n) && n.String() != "" && n.String() != "Object" {
				return "a " + n.String()
			}
		}
		return "an object"
	}
	return v.ExportType().String() + " " + v.String()
}

var contextType = reflect.TypeFor[context.Context]()

// isAsync reports whether a Go call, its parameters from the first'th,
// is async: one taking a context first.
func isAsync(t reflect.Type, first int) bool { return t.NumIn() > first && t.In(first) == contextType }

// goError is err as JS's Error, of its message.
func (r *runtime) goError(err error) *js.Object {
	ctor := "Error"
	var named *JSError
	if errors.As(err, &named) {
		switch named.Name {
		case "TypeError", "RangeError", "SyntaxError", "ReferenceError":
			ctor = named.Name
		}
	}
	o, _ := r.vm.New(r.vm.Get(ctor), r.vm.ToValue(err.Error()))
	if code := core.ErrorCode(err); code != "" {
		_ = o.Set("code", code)
	}
	var coded *core.Error
	if errors.As(err, &coded) && coded.Permission != "" {
		_ = o.Set("permission", coded.Permission)
		_ = o.Set("resource", coded.Resource)
	}
	return o
}

// JSError is an error JS throws as the built-in error Name names, a
// TypeError or a RangeError, rather than an Error.
type JSError struct{ Name, Message string }

func (e *JSError) Error() string { return e.Message }

// callGo calls fn, named name in errors, with JS's arguments: in the JS
// call, or on a goroutine for a promise when fn is async.
func (r *runtime) callGo(name string, fn reflect.Value, args []js.Value) js.Value {
	ft := fn.Type()
	if !isAsync(ft, 0) {
		in := r.goArgs(ft, 0, args)
		out := r.guard(name, func() []reflect.Value { return callIn(fn, in) })
		if err := trailingErr(ft, out); err != nil {
			panic(r.goError(err))
		}
		return r.goResult(ft, out)
	}
	// Arguments convert now, as a synchronous call's do; the functions
	// among them call back through the engine.
	r.posting++
	in := r.goArgs(ft, 1, args)
	r.posting--
	p, resolve, reject := r.vm.NewPromise()
	var ctx context.Context
	r.cx.Run(func() { ctx = r.cx.Context() })
	in[0] = reflect.ValueOf(ctx)
	go func() {
		var out []reflect.Value
		var failed any
		func() {
			defer func() { failed = recover() }()
			out = callIn(fn, in)
		}()
		r.cx.Post(func() {
			r.inJS(func() {
				switch {
				case failed != nil:
					_ = reject(r.goError(fmt.Errorf("%s: %v", name, failed)))
				case trailingErr(ft, out) != nil:
					_ = reject(r.goError(trailingErr(ft, out)))
				default:
					_ = resolve(r.goResult(ft, out))
				}
			})
		})
	}()
	return r.vm.ToValue(p)
}

// guard runs a synchronous call, throwing what it panics with as an
// Error, as NSH catches a Rust panic; JS's own exceptions pass.
func (r *runtime) guard(name string, call func() []reflect.Value) []reflect.Value {
	defer func() {
		if p := recover(); p != nil {
			switch p.(type) {
			case *js.Object, *js.Exception, *js.InterruptedError, *js.StackOverflowError:
				panic(p)
			}
			panic(r.goError(fmt.Errorf("%s: %v", name, p)))
		}
	}()
	return call()
}

// callIn calls fn with what goArgs made, whose last is the variadic
// parameters' slice when fn takes them.
func callIn(fn reflect.Value, in []reflect.Value) []reflect.Value {
	if fn.Type().IsVariadic() {
		return fn.CallSlice(in)
	}
	return fn.Call(in)
}

// goArgs converts JS's arguments to ft's parameters from the first'th.
func (r *runtime) goArgs(ft reflect.Type, first int, args []js.Value) []reflect.Value {
	in := make([]reflect.Value, ft.NumIn())
	if first < len(in) && ft.In(first) == scopeType {
		// A leading scope is the app's, as a component's is its own.
		in[first] = reflect.ValueOf(r.cx)
		first++
	}
	for i := first; i < len(in); i++ {
		t := ft.In(i)
		j := i - first
		if ft.IsVariadic() && i == len(in)-1 {
			rest := reflect.MakeSlice(t, 0, len(args)-j)
			for ; j < len(args); j++ {
				rest = reflect.Append(rest, r.decode(args[j], t.Elem()))
			}
			in[i] = rest
			continue
		}
		v := js.Undefined()
		if j < len(args) {
			v = args[j]
		}
		in[i] = r.decode(v, t)
	}
	return in
}

// trailingErr is a call's error result, nil without one.
func trailingErr(ft reflect.Type, out []reflect.Value) error {
	if n := len(out); n > 0 && ft.Out(n-1) == errorType && !out[n-1].IsNil() {
		return out[n-1].Interface().(error)
	}
	return nil
}

// goResult is what JS gets of a call's results: undefined for none, the
// trailing error left out.
func (r *runtime) goResult(ft reflect.Type, out []reflect.Value) js.Value {
	if n := len(out); n > 0 && ft.Out(n-1) == errorType {
		out = out[:n-1]
	}
	if len(out) == 0 {
		return js.Undefined()
	}
	return r.results(out, ft, nil)
}

// callback is a JS function given to an async call as a Go func of type
// t: run on the engine goroutine, waited for when it has results.
func (r *runtime) callback(fn js.Value, t reflect.Type) reflect.Value {
	var ctx context.Context
	r.cx.Run(func() { ctx = r.cx.Context() })
	return reflect.MakeFunc(t, func(in []reflect.Value) []reflect.Value {
		outs := make([]reflect.Value, t.NumOut())
		for i := range outs {
			outs[i] = reflect.Zero(t.Out(i))
		}
		if t.NumOut() == 0 {
			r.cx.Post(func() { r.inJS(func() { r.call(fn, r.encodeAll(in)...) }) })
			return outs
		}
		done := make(chan []reflect.Value, 1)
		settle := func(v js.Value, err error) {
			res := slices.Clone(outs)
			for i := range res {
				if t.Out(i) == errorType {
					if err != nil {
						res[i] = reflect.ValueOf(&err).Elem()
					}
				} else if err == nil {
					res[i] = r.decode(v, t.Out(i))
				}
			}
			done <- res
		}
		r.cx.Post(func() {
			r.inJS(func() {
				f, _ := js.AssertFunction(fn)
				v, err := f(js.Undefined(), r.encodeAll(in)...)
				if err != nil {
					v = js.Undefined()
				} else if _, ok := v.Export().(*js.Promise); ok {
					then, _ := js.AssertFunction(v.ToObject(r.vm).Get("then"))
					_, _ = then(v, r.function(func(c js.FunctionCall) js.Value {
						settle(c.Argument(0), nil)
						return js.Undefined()
					}), r.function(func(c js.FunctionCall) js.Value {
						settle(nil, jsError(c.Argument(0)))
						return js.Undefined()
					}))
					return
				}
				if ex, ok := err.(*js.Exception); ok {
					err = jsError(ex.Value())
				}
				settle(v, err)
			})
		})
		select {
		case res := <-done:
			return res
		case <-ctx.Done():
			return outs
		}
	})
}

func (r *runtime) encodeAll(in []reflect.Value) []js.Value {
	args := make([]js.Value, len(in))
	for i, a := range in {
		args[i] = r.encode(a)
	}
	return args
}

// jsError is what JS threw or rejected with as a Go error.
func jsError(v js.Value) error {
	if o, ok := v.(*js.Object); ok {
		if m := o.Get("message"); defined(m) {
			return fmt.Errorf("%s", m.String())
		}
	}
	return fmt.Errorf("%s", v.String())
}
