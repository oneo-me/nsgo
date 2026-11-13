//go:build js

// Package js is the JavaScript engine package tsx runs apps on: goja on
// every target but the web (goja.go), where the page's own engine runs
// them. Its API is goja's, so the runtime is written once; here it is the
// part tsx uses, over syscall/js, each Runtime a realm of its own, an
// iframe of the page's origin.
//
// Values cross as goja's do: primitives are copied into Go, objects are
// handles, Go functions and objects are JS functions and objects calling
// back through one function, and what JS throws comes back as an
// *Exception. A Go panic in a Go function JS called goes on in the Go code
// that called JS.
package js

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	sjs "syscall/js"
	"time"

	"github.com/go-sourcemap/sourcemap"

	"github.com/nsgo-dev/nsgo/tsx/websrc"
)

// Browser reports whether apps run on the page's engine.
const Browser = true

// Callable is a JS function as Go calls it.
type Callable func(this Value, args ...Value) (Value, error)

// Constructor is a JS constructor as Go calls it with new.
type Constructor func(newTarget *Object, args ...Value) (*Object, error)

// Flag is a property attribute's setting.
type Flag int

const (
	FLAG_NOT_SET Flag = iota
	FLAG_FALSE
	FLAG_TRUE
)

func (f Flag) js(def bool) bool {
	if f == FLAG_NOT_SET {
		return def
	}
	return f == FLAG_TRUE
}

// FieldNameMapper names Go fields and methods in JS.
type FieldNameMapper interface {
	FieldName(t reflect.Type, f reflect.StructField) string
	MethodName(t reflect.Type, m reflect.Method) string
}

type uncapMapper struct{}

func (uncapMapper) FieldName(_ reflect.Type, f reflect.StructField) string { return uncap(f.Name) }
func (uncapMapper) MethodName(_ reflect.Type, m reflect.Method) string     { return uncap(m.Name) }

func uncap(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// UncapFieldNameMapper names fields and methods with a lowercase first
// letter, as goja's does.
func UncapFieldNameMapper() FieldNameMapper { return uncapMapper{} }

// Program is a script compiled for RunProgram: on the page's engine, its
// source.
type Program struct{ name, src string }

// MustCompile returns the program of src.
func MustCompile(name, src string, _ bool) *Program { return &Program{name: name, src: src} }

// timeProgram patches Date to read the app's clock.
var timeProgram = MustCompile(websrc.Time.Name, websrc.Time.Source, true)

// FunctionCall is a call of a Go function from JS.
type FunctionCall struct {
	This      Value
	Arguments []Value
}

// Argument returns argument i, undefined when missing.
func (f FunctionCall) Argument(i int) Value {
	if i < len(f.Arguments) {
		return f.Arguments[i]
	}
	return Undefined()
}

// ConstructorCall is a call of a Go constructor with new.
type ConstructorCall struct {
	This      *Object
	Arguments []Value
	NewTarget *Object
}

// Argument returns argument i, undefined when missing.
func (f ConstructorCall) Argument(i int) Value {
	if i < len(f.Arguments) {
		return f.Arguments[i]
	}
	return Undefined()
}

// Exception is what JS threw.
type Exception struct {
	val   Value
	stack string
}

// Value is the value thrown.
func (e *Exception) Value() Value { return e.val }

func (e *Exception) Error() string {
	if e.stack != "" {
		return e.stack
	}
	return e.val.String()
}

func (e *Exception) String() string { return e.Error() }

// Unwrap is the Go error a GoError carries.
func (e *Exception) Unwrap() error {
	if o, ok := e.val.(*Object); ok {
		if err, ok := o.Export().(error); ok {
			return err
		}
	}
	return nil
}

// InterruptedError and StackOverflowError are goja's; the page's engine
// cannot be interrupted and reports overflows as RangeErrors.
type InterruptedError struct{ Exception }
type StackOverflowError struct{ Exception }

// PromiseState is a promise's state.
type PromiseState int

const (
	PromiseStatePending PromiseState = iota
	PromiseStateFulfilled
	PromiseStateRejected
)

// Promise is a JS promise. Its state follows the promise's reactions from
// its first reading, so it changes only once the page's engine ran them.
type Promise struct {
	obj     *Object
	state   PromiseState
	result  Value
	tracked bool
}

func (p *Promise) State() PromiseState { p.obj.r.track(p); return p.state }
func (p *Promise) Result() Value       { p.obj.r.track(p); return p.result }

// ArrayBuffer is a JS ArrayBuffer's bytes, copied.
type ArrayBuffer struct{ data []byte }

func (a ArrayBuffer) Bytes() []byte { return a.data }

// Symbol is a JS symbol.
type Symbol struct {
	v    sjs.Value
	desc string
}

// NewSymbol returns a new symbol described as desc.
func NewSymbol(desc string) *Symbol {
	return &Symbol{v: sjs.Global().Get("Symbol").Invoke(desc), desc: desc}
}

// Runtime is one realm: an iframe's global object, with the functions
// Go reaches it through.
type Runtime struct {
	frame, global sjs.Value
	h             helpers
	mapper        FieldNameMapper
	now           func() time.Time
	// funcs are the Go functions JS holds, by ID; objects the Go values,
	// and protos the prototypes of Go types.
	funcs   map[int]func(this, target sjs.Value, args []sjs.Value) sjs.Value
	objects map[int]any
	protos  map[reflect.Type]sjs.Value
	next    int
	// depth counts the calls into JS under way; entry runs what JS calls
	// when none is, as from a promise's reaction or a browser event.
	depth int
	entry func(func())
	// panics are Go panics JS is unwinding, by ID, and panicked the one
	// to go on with once the call into JS returns.
	panics   map[int]any
	panicked any
	closed   bool
	// maps are the source maps of the scripts run, by name, which the
	// stacks of exceptions are read through, as goja reads them.
	maps map[string]*sourcemap.Consumer

	dispatch, release sjs.Func
}

// helpers are the realm's functions the runtime calls.
type helpers struct {
	obj                                           sjs.Value
	throw, missing, goKey, panicKey               sjs.Value
	setThrown, fn, ctor, get, call, construct     sjs.Value
	defineData, defineAccessor, goObject, goID    sjs.Value
	newPromise, track, run, kind, str, num        sjs.Value
	className, keys, array, typeError, error      sjs.Value
	toObject, setProto, getProto, set, json, desc sjs.Value
	del, bytes, buffer, date, dateValue           sjs.Value
}

// factory returns the realm's helper factory: the page's own when it
// shipped one, else the source compiled here.
func (r *Runtime) factory() sjs.Value {
	if f := r.program(websrc.ProgramsGlobal, websrc.Helper.Name); f.Truthy() {
		return f.Invoke()
	}
	return r.global.Call("eval", websrc.Helper.Source)
}

// program returns what the page's files registered as name on the
// global object registry, or undefined when they registered none.
func (r *Runtime) program(registry, name string) sjs.Value {
	reg := r.global.Get(registry)
	if !reg.Truthy() {
		return sjs.Undefined()
	}
	return reg.Get(name)
}

// realmTaken is whether a runtime took the realm the page made.
var realmTaken atomic.Bool

// New returns a runtime in a realm of its own: the page's, when it made
// one for the app (nsgo.js, the loader, names it in NSGO_REALM), else one
// this makes. The first runtime alone takes the page's: a sandbox's, as
// tsx.View runs a package in, is a realm apart from the app's.
func New() *Runtime {
	frame := sjs.Undefined()
	if name := os.Getenv("NSGO_REALM"); name != "" && !realmTaken.Swap(true) {
		frame = sjs.Global().Get(name)
	}
	if !frame.Truthy() {
		doc := sjs.Global().Get("document")
		frame = doc.Call("createElement", "iframe")
		frame.Call("setAttribute", "aria-hidden", "true")
		frame.Call("setAttribute", "tabindex", "-1")
		frame.Get("style").Set("display", "none")
		doc.Get("documentElement").Call("appendChild", frame)
	}
	r := &Runtime{
		frame: frame, global: frame.Get("contentWindow"), mapper: uncapMapper{},
		funcs: map[int]func(this, target sjs.Value, args []sjs.Value) sjs.Value{}, objects: map[int]any{},
		protos: map[reflect.Type]sjs.Value{}, panics: map[int]any{}, maps: map[string]*sourcemap.Consumer{},
	}
	r.dispatch = sjs.FuncOf(r.call)
	r.release = sjs.FuncOf(func(_ sjs.Value, args []sjs.Value) any {
		id := args[0].Int()
		delete(r.funcs, id)
		delete(r.objects, id)
		return nil
	})
	o := r.factory().Invoke(r.dispatch, r.release)
	g := func(k string) sjs.Value { return o.Get(k) }
	r.h = helpers{
		obj: o, throw: g("throw"), missing: g("missing"), goKey: g("goKey"), panicKey: g("panicKey"),
		setThrown: g("setThrown"), fn: g("fn"), ctor: g("ctor"), get: g("get"), call: g("call"), construct: g("construct"),
		defineData: g("defineData"), defineAccessor: g("defineAccessor"), goObject: g("goObject"), goID: g("goID"),
		newPromise: g("newPromise"), track: g("track"), run: g("run"), kind: g("kind"), str: g("str"), num: g("num"),
		className: g("className"), keys: g("keys"), array: g("array"), typeError: g("typeError"), error: g("error"),
		toObject: g("toObject"), setProto: g("setProto"), getProto: g("getProto"), set: g("set"), json: g("json"), desc: g("desc"),
		del: g("del"), bytes: g("bytes"), buffer: g("buffer"), date: g("date"), dateValue: g("dateValue"),
	}
	return r
}

// Close removes the realm: what it still holds stops calling Go.
func (r *Runtime) Close() {
	if r.closed {
		return
	}
	r.closed = true
	r.frame.Call("remove")
	r.funcs, r.objects, r.protos = map[int]func(this, target sjs.Value, args []sjs.Value) sjs.Value{}, map[int]any{}, map[reflect.Type]sjs.Value{}
}

// SetEntry sets what runs a call of a Go function that JS makes when Go
// is not calling JS, as a promise's reactions and the browser's events
// do: run is the call, which panics with what the Go function panicked
// with, and entry must recover it.
func (r *Runtime) SetEntry(entry func(run func())) { r.entry = entry }

func (r *Runtime) SetFieldNameMapper(m FieldNameMapper) { r.mapper = m }
func (r *Runtime) SetMaxCallStackSize(int)              {}
func (r *Runtime) SetAsyncContextTracker(any)           {}
func (r *Runtime) Interrupt(any)                        {}
func (r *Runtime) ClearInterrupt()                      {}

// SetTimeSource has the realm's Date read now.
func (r *Runtime) SetTimeSource(now func() time.Time) {
	r.now = now
	nowMS := r.function(func(FunctionCall) Value {
		return floatValue(float64(r.now().UnixNano()) / 1e6)
	})
	patch, err := r.RunProgram(timeProgram)
	if err != nil {
		panic(err)
	}
	f, _ := AssertFunction(patch)
	if _, err := f(Undefined(), nowMS); err != nil {
		panic(err)
	}
}

// call is what every Go function JS holds calls: its ID, this, new.target
// and the arguments.
func (r *Runtime) call(_ sjs.Value, args []sjs.Value) any {
	fn := r.funcs[args[0].Int()]
	if fn == nil {
		r.h.setThrown.Invoke(r.h.typeError.Invoke("the app has stopped"))
		return r.h.throw
	}
	if r.depth > 0 || r.entry == nil {
		return r.guarded(fn, args[1], args[2], args[3:])
	}
	var out sjs.Value
	r.entry(func() {
		out = r.guarded(fn, args[1], args[2], args[3:])
		if p := r.panicked; p != nil {
			// No Go code called JS: the entry goes on with it.
			r.panicked = nil
			clear(r.panics)
			panic(p)
		}
	})
	return out
}

// guarded runs fn, turning what it panics with into what JS throws.
func (r *Runtime) guarded(fn func(this, target sjs.Value, args []sjs.Value) sjs.Value, this, target sjs.Value, args []sjs.Value) (out sjs.Value) {
	defer func() {
		p := recover()
		if p == nil {
			return
		}
		switch v := p.(type) {
		case *Exception:
			r.h.setThrown.Invoke(v.val.js())
		case Value:
			r.h.setThrown.Invoke(v.js())
		default:
			// A Go panic: JS unwinds, and the Go code that called JS
			// goes on with it, with where it began.
			if err, ok := p.(runtime.Error); ok {
				p = &stackError{err, debug.Stack()}
			}
			r.next++
			r.panics[r.next] = p
			marker := r.h.error.Invoke(fmt.Sprint(p))
			marker.Set("name", "GoPanic")
			r.h.defineData.Invoke(marker, r.h.panicKey, r.next, false, false, false)
			r.panicked = p
			r.h.setThrown.Invoke(marker)
		}
		out = r.h.throw
	}()
	return fn(this, target, args)
}

// stackError is a Go runtime error with the stack it began on, which
// the panic going on past JS's frames no longer shows.
type stackError struct {
	err   runtime.Error
	stack []byte
}

func (e *stackError) Error() string { return e.err.Error() + "\n" + string(e.stack) }
func (e *stackError) Unwrap() error { return e.err }

// enter and leave count a call into JS; leave goes on with a Go panic JS
// unwound.
func (r *Runtime) enter() { r.depth++ }

func (r *Runtime) leave() {
	r.depth--
	if p := r.panicked; p != nil {
		// JS caught it.
		r.panicked = nil
		clear(r.panics)
		panic(p)
	}
}

// try runs a call into JS, returning what it threw.
func (r *Runtime) try(f func() sjs.Value) (v sjs.Value, err error) {
	r.enter()
	defer r.leave()
	defer func() {
		p := recover()
		if p == nil {
			return
		}
		je, ok := p.(sjs.Error)
		if !ok {
			panic(p)
		}
		thrown := je.Value
		if thrown.Type() == sjs.TypeObject {
			if id := r.symbolGet(thrown, r.h.panicKey); id.Type() == sjs.TypeNumber {
				gp := r.panics[id.Int()]
				delete(r.panics, id.Int())
				if gp != nil {
					r.panicked = nil
					panic(gp)
				}
			}
		}
		ex := &Exception{val: r.wrap(thrown)}
		if thrown.Type() == sjs.TypeObject {
			if s := thrown.Get("stack"); s.Type() == sjs.TypeString {
				ex.stack = r.mapStack(s.String())
			}
		}
		err = ex
	}()
	return f(), nil
}

func (r *Runtime) symbolGet(o, key sjs.Value) sjs.Value {
	v := r.h.get.Invoke(o, key)
	if v.Equal(r.h.missing) {
		return sjs.Undefined()
	}
	return v
}

// function makes a JS function of a Go one.
func (r *Runtime) function(fn func(FunctionCall) Value) *Object {
	return r.newFunc(func(this, _ sjs.Value, args []sjs.Value) sjs.Value {
		return jsOf(fn(FunctionCall{This: r.wrap(this), Arguments: r.wrapAll(args)}))
	}, false)
}

func (r *Runtime) newFunc(fn func(this, target sjs.Value, args []sjs.Value) sjs.Value, ctor bool) *Object {
	r.next++
	r.funcs[r.next] = fn
	make := r.h.fn
	if ctor {
		make = r.h.ctor
	}
	return &Object{r: r, v: make.Invoke(r.next), fn: true}
}

const inlineMap = "//# sourceMappingURL=data:application/json;base64,"

// position is a script's position in a stack.
var position = regexp.MustCompile(`([^\s()]+):(\d+):(\d+)`)

// mapStack writes the positions of a stack in scripts with source maps
// as their sources'.
func (r *Runtime) mapStack(stack string) string {
	// The frames of the runtime's own calls say nothing of the app.
	lines := strings.Split(stack, "\n")
	kept := lines[:0]
	for _, l := range lines {
		if !strings.Contains(l, "wasm_exec") && strings.TrimSpace(l) != "at eval (<anonymous>)" {
			kept = append(kept, l)
		}
	}
	stack = strings.Join(kept, "\n")
	if len(r.maps) == 0 {
		return stack
	}
	return position.ReplaceAllStringFunc(stack, func(at string) string {
		m := position.FindStringSubmatch(at)
		sm := r.maps[m[1]]
		if sm == nil {
			return at
		}
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		src, _, l, c, ok := sm.Source(line, col-1)
		if !ok {
			return at
		}
		return fmt.Sprintf("%s:%d:%d", src, l, c+1)
	})
}

// RunScript runs a script in the realm's global scope: the page's copy of
// it when it shipped one, else src. An empty src is one the page ships,
// as a release web build's are (docs/architecture/tsx.md).
func (r *Runtime) RunScript(name, src string) (Value, error) {
	if src == "" {
		f := r.program(websrc.ScriptsGlobal, name)
		if !f.Truthy() {
			return nil, fmt.Errorf("nsgo: %s is not on the page; serve the app with the loader nsgo build -target web writes", name)
		}
		return r.callProgram(f)
	}
	return r.runSource(name, src)
}

func (r *Runtime) RunString(src string) (Value, error) { return r.runSource("<eval>", src) }

// runSource evaluates src in the realm's global scope, named name in
// stacks.
func (r *Runtime) runSource(name, src string) (Value, error) {
	if i := strings.LastIndex(src, inlineMap); i >= 0 {
		data, _, _ := strings.Cut(src[i+len(inlineMap):], "\n")
		if b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data)); err == nil {
			if sm, err := sourcemap.Parse(name, b); err == nil {
				r.maps[name] = sm
			}
		}
	}
	v, err := r.try(func() sjs.Value { return r.h.run.Invoke(src + "\n//# sourceURL=" + name + "\n") })
	if err != nil {
		return nil, err
	}
	return r.wrap(v), nil
}

// RunProgram runs the program p, returning what it evaluates to: the
// page's copy of it when it shipped one, else p's source.
func (r *Runtime) RunProgram(p *Program) (Value, error) {
	if f := r.program(websrc.ProgramsGlobal, p.name); f.Truthy() {
		return r.callProgram(f)
	}
	return r.RunScript(p.name, p.src)
}

// callProgram runs what a shipped file registered: a function whose call
// makes the program (RunProgram) or runs the script (RunScript).
func (r *Runtime) callProgram(f sjs.Value) (Value, error) {
	v, err := r.try(func() sjs.Value { return f.Invoke() })
	if err != nil {
		return nil, err
	}
	return r.wrap(v), nil
}

// GlobalObject is the realm's global object.
func (r *Runtime) GlobalObject() *Object { return &Object{r: r, v: r.global} }

func (r *Runtime) Set(name string, v any) error { return r.GlobalObject().Set(name, v) }

func (r *Runtime) Get(name string) Value { return r.GlobalObject().Get(name) }

func (r *Runtime) NewObject() *Object {
	return &Object{r: r, v: r.global.Get("Object").New()}
}

func (r *Runtime) NewArray(items ...any) *Object {
	args := make([]any, len(items))
	for i, it := range items {
		args[i] = r.ToValue(it).js()
	}
	return &Object{r: r, v: r.h.array.Invoke(args...)}
}

func (r *Runtime) NewTypeError(format string, args ...any) *Object {
	return &Object{r: r, v: r.h.typeError.Invoke(fmt.Sprintf(format, args...))}
}

// NewGoError makes an Error of err, which exports as err.
func (r *Runtime) NewGoError(err error) *Object {
	o := &Object{r: r, v: r.h.error.Invoke(err.Error())}
	r.next++
	r.objects[r.next] = err
	r.h.defineData.Invoke(o.v, r.h.goKey, r.next, false, false, false)
	o.v.Set("name", "GoError")
	return o
}

// NewPromise returns a pending promise and its resolving functions.
func (r *Runtime) NewPromise() (*Promise, func(any) error, func(any) error) {
	trio := r.h.newPromise.Invoke()
	p := &Promise{obj: &Object{r: r, v: trio.Index(0)}}
	resolve, reject := trio.Index(1), trio.Index(2)
	settle := func(fn sjs.Value) func(any) error {
		return func(v any) error {
			_, err := r.try(func() sjs.Value { return fn.Invoke(r.ToValue(v).js()) })
			return err
		}
	}
	return p, settle(resolve), settle(reject)
}

// track follows p's state from its first reading: following it from its
// making would handle its rejection, which the page then reports nowhere.
func (r *Runtime) track(p *Promise) {
	if p.tracked {
		return
	}
	p.tracked = true
	var cb sjs.Func
	cb = sjs.FuncOf(func(_ sjs.Value, args []sjs.Value) any {
		cb.Release()
		if args[0].Int() == 1 {
			p.state = PromiseStateFulfilled
		} else {
			p.state = PromiseStateRejected
		}
		p.result = r.wrap(args[1])
		return nil
	})
	r.h.track.Invoke(p.obj.v, cb)
}

func (r *Runtime) NewArrayBuffer(data []byte) ArrayBuffer { return ArrayBuffer{data: data} }

// New calls a constructor with new.
func (r *Runtime) New(ctor Value, args ...Value) (*Object, error) {
	c, ok := ctor.(*Object)
	if !ok {
		return nil, &Exception{val: &Object{r: r, v: r.h.typeError.Invoke("not a constructor")}}
	}
	in := append([]any{c.v, sjs.Undefined()}, jsAll(args)...)
	v, err := r.try(func() sjs.Value { return r.h.construct.Invoke(in...) })
	if err != nil {
		return nil, err
	}
	return &Object{r: r, v: v}, nil
}

// ExportTo converts v to the Go value target points at.
func (r *Runtime) ExportTo(v Value, target any) error {
	t := reflect.ValueOf(target)
	if t.Kind() != reflect.Pointer || t.IsNil() {
		return errors.New("js: ExportTo needs a non-nil pointer")
	}
	out, err := r.exportAs(v, t.Elem().Type())
	if err != nil {
		return err
	}
	t.Elem().Set(out)
	return nil
}

// AssertFunction returns v as a function Go calls.
func AssertFunction(v Value) (Callable, bool) {
	o, ok := v.(*Object)
	if !ok || !o.isFunction() {
		return nil, false
	}
	r := o.r
	return func(this Value, args ...Value) (Value, error) {
		in := append([]any{o.v, jsOf(this)}, jsAll(args)...)
		res, err := r.try(func() sjs.Value { return r.h.call.Invoke(in...) })
		if err != nil {
			return nil, err
		}
		return r.wrap(res), nil
	}, true
}

// AssertConstructor returns v as a constructor Go calls.
func AssertConstructor(v Value) (Constructor, bool) {
	o, ok := v.(*Object)
	if !ok || !o.isFunction() {
		return nil, false
	}
	r := o.r
	return func(newTarget *Object, args ...Value) (*Object, error) {
		target := sjs.Undefined()
		if newTarget != nil {
			target = newTarget.v
		}
		in := append([]any{o.v, target}, jsAll(args)...)
		res, err := r.try(func() sjs.Value { return r.h.construct.Invoke(in...) })
		if err != nil {
			return nil, err
		}
		return &Object{r: r, v: res}, nil
	}, true
}

func jsOf(v Value) sjs.Value {
	if v == nil {
		return sjs.Undefined()
	}
	return v.js()
}

func jsAll(vs []Value) []any {
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = jsOf(v)
	}
	return out
}

// numberString writes f as JS's Number.prototype.toString does.
func numberString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	a := math.Abs(f)
	if a >= 1e-6 && a < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	mant, exp, _ := strings.Cut(s, "e")
	sign := exp[0]
	exp = strings.TrimLeft(exp[1:], "0")
	return mant + "e" + string(sign) + exp
}
