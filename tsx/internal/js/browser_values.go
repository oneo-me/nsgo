//go:build js

package js

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	sjs "syscall/js"
	"time"
)

// Value is a JS value: a primitive copied into Go, or an *Object.
type Value interface {
	Export() any
	ExportType() reflect.Type
	ToInteger() int64
	ToFloat() float64
	ToNumber() Value
	ToString() Value
	String() string
	ToBoolean() bool
	ToObject(*Runtime) *Object
	SameAs(Value) bool
	Equals(Value) bool
	StrictEquals(Value) bool
	js() sjs.Value
}

type (
	undefinedValue struct{}
	nullValue      struct{}
	boolValue      bool
	floatValue     float64
	stringValue    string
)

func Undefined() Value { return undefinedValue{} }
func Null() Value      { return nullValue{} }

func IsUndefined(v Value) bool { _, ok := v.(undefinedValue); return ok }
func IsNull(v Value) bool      { _, ok := v.(nullValue); return ok }

func (undefinedValue) Export() any              { return nil }
func (undefinedValue) ExportType() reflect.Type { return nil }
func (undefinedValue) ToInteger() int64         { return 0 }
func (undefinedValue) ToFloat() float64         { return math.NaN() }
func (v undefinedValue) ToNumber() Value        { return floatValue(math.NaN()) }
func (undefinedValue) ToString() Value          { return stringValue("undefined") }
func (undefinedValue) String() string           { return "undefined" }
func (undefinedValue) ToBoolean() bool          { return false }
func (undefinedValue) ToObject(r *Runtime) *Object {
	panic(r.NewTypeError("Cannot convert undefined to object"))
}
func (undefinedValue) SameAs(o Value) bool       { return IsUndefined(o) }
func (undefinedValue) Equals(o Value) bool       { return IsUndefined(o) || IsNull(o) }
func (undefinedValue) StrictEquals(o Value) bool { return IsUndefined(o) }
func (undefinedValue) js() sjs.Value             { return sjs.Undefined() }

func (nullValue) Export() any                 { return nil }
func (nullValue) ExportType() reflect.Type    { return nil }
func (nullValue) ToInteger() int64            { return 0 }
func (nullValue) ToFloat() float64            { return 0 }
func (nullValue) ToNumber() Value             { return floatValue(0) }
func (nullValue) ToString() Value             { return stringValue("null") }
func (nullValue) String() string              { return "null" }
func (nullValue) ToBoolean() bool             { return false }
func (nullValue) ToObject(r *Runtime) *Object { panic(r.NewTypeError("Cannot convert null to object")) }
func (nullValue) SameAs(o Value) bool         { return IsNull(o) }
func (nullValue) Equals(o Value) bool         { return IsUndefined(o) || IsNull(o) }
func (nullValue) StrictEquals(o Value) bool   { return IsNull(o) }
func (nullValue) js() sjs.Value               { return sjs.Null() }

func (b boolValue) Export() any            { return bool(b) }
func (boolValue) ExportType() reflect.Type { return reflect.TypeFor[bool]() }
func (b boolValue) ToInteger() int64       { return int64(b.ToFloat()) }
func (b boolValue) ToFloat() float64 {
	if b {
		return 1
	}
	return 0
}
func (b boolValue) ToNumber() Value { return floatValue(b.ToFloat()) }
func (b boolValue) ToString() Value { return stringValue(b.String()) }
func (b boolValue) String() string  { return strconv.FormatBool(bool(b)) }
func (b boolValue) ToBoolean() bool { return bool(b) }
func (b boolValue) ToObject(r *Runtime) *Object {
	return &Object{r: r, v: r.h.toObject.Invoke(bool(b))}
}
func (b boolValue) SameAs(o Value) bool       { return b.StrictEquals(o) }
func (b boolValue) Equals(o Value) bool       { return b.StrictEquals(o) || looseNumber(b, o) }
func (b boolValue) StrictEquals(o Value) bool { ob, ok := o.(boolValue); return ok && ob == b }
func (b boolValue) js() sjs.Value             { return sjs.ValueOf(bool(b)) }

// Export gives an integer as int64, as goja does, else float64.
func (f floatValue) Export() any {
	v := float64(f)
	if v == math.Trunc(v) && math.Abs(v) < 1<<53 && !(v == 0 && math.Signbit(v)) {
		return int64(v)
	}
	return v
}
func (f floatValue) ExportType() reflect.Type { return reflect.TypeOf(f.Export()) }
func (f floatValue) ToInteger() int64 {
	v := float64(f)
	switch {
	case math.IsNaN(v):
		return 0
	case v >= math.MaxInt64:
		return math.MaxInt64
	case v <= math.MinInt64:
		return math.MinInt64
	}
	return int64(v)
}
func (f floatValue) ToFloat() float64 { return float64(f) }
func (f floatValue) ToNumber() Value  { return f }
func (f floatValue) ToString() Value  { return stringValue(f.String()) }
func (f floatValue) String() string   { return numberString(float64(f)) }
func (f floatValue) ToBoolean() bool  { return f != 0 && !math.IsNaN(float64(f)) }
func (f floatValue) ToObject(r *Runtime) *Object {
	return &Object{r: r, v: r.h.toObject.Invoke(float64(f))}
}
func (f floatValue) SameAs(o Value) bool {
	of, ok := o.(floatValue)
	if !ok {
		return false
	}
	if math.IsNaN(float64(f)) {
		return math.IsNaN(float64(of))
	}
	return f == of && math.Signbit(float64(f)) == math.Signbit(float64(of))
}
func (f floatValue) Equals(o Value) bool       { return f.StrictEquals(o) || looseNumber(f, o) }
func (f floatValue) StrictEquals(o Value) bool { of, ok := o.(floatValue); return ok && of == f }
func (f floatValue) js() sjs.Value             { return sjs.ValueOf(float64(f)) }

func (s stringValue) Export() any            { return string(s) }
func (stringValue) ExportType() reflect.Type { return reflect.TypeFor[string]() }
func (s stringValue) ToInteger() int64       { return floatValue(s.ToFloat()).ToInteger() }
func (s stringValue) ToFloat() float64       { return stringNumber(string(s)) }
func (s stringValue) ToNumber() Value        { return floatValue(s.ToFloat()) }
func (s stringValue) ToString() Value        { return s }
func (s stringValue) String() string         { return string(s) }
func (s stringValue) ToBoolean() bool        { return s != "" }
func (s stringValue) ToObject(r *Runtime) *Object {
	return &Object{r: r, v: r.h.toObject.Invoke(string(s))}
}
func (s stringValue) SameAs(o Value) bool       { return s.StrictEquals(o) }
func (s stringValue) Equals(o Value) bool       { return s.StrictEquals(o) || looseNumber(s, o) }
func (s stringValue) StrictEquals(o Value) bool { os, ok := o.(stringValue); return ok && os == s }
func (s stringValue) js() sjs.Value             { return sjs.ValueOf(string(s)) }

func looseNumber(a, b Value) bool {
	switch b.(type) {
	case boolValue, floatValue, stringValue:
		return a.ToFloat() == b.ToFloat()
	}
	return false
}

// stringNumber is JS's ToNumber of a string.
func stringNumber(s string) float64 {
	s = strings.TrimSpace(s)
	switch s {
	case "":
		return 0
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if len(s) > 2 && s[0] == '0' {
		base := 0
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			if n, err := strconv.ParseUint(s[2:], base, 64); err == nil {
				return float64(n)
			}
			return math.NaN()
		}
	}
	if strings.ContainsAny(s, "_xXpPnN") || strings.HasPrefix(s, "0x") {
		return math.NaN()
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil && !strings.Contains(err.Error(), "range") {
		return math.NaN()
	}
	return f
}

func (s *Symbol) Export() any                 { return s }
func (s *Symbol) ExportType() reflect.Type    { return reflect.TypeOf(s) }
func (s *Symbol) ToInteger() int64            { return 0 }
func (s *Symbol) ToFloat() float64            { return math.NaN() }
func (s *Symbol) ToNumber() Value             { return floatValue(math.NaN()) }
func (s *Symbol) ToString() Value             { return stringValue(s.String()) }
func (s *Symbol) String() string              { return "Symbol(" + s.desc + ")" }
func (s *Symbol) ToBoolean() bool             { return true }
func (s *Symbol) ToObject(r *Runtime) *Object { return &Object{r: r, v: r.h.toObject.Invoke(s.v)} }
func (s *Symbol) SameAs(o Value) bool         { return s.StrictEquals(o) }
func (s *Symbol) Equals(o Value) bool         { return s.StrictEquals(o) }
func (s *Symbol) StrictEquals(o Value) bool   { os, ok := o.(*Symbol); return ok && os.v.Equal(s.v) }
func (s *Symbol) js() sjs.Value               { return s.v }

// Object is a JS object or function.
type Object struct {
	r  *Runtime
	v  sjs.Value
	fn bool
}

func (o *Object) js() sjs.Value { return o.v }

func (o *Object) isFunction() bool { return o.fn || o.v.Type() == sjs.TypeFunction }

func (o *Object) ToObject(*Runtime) *Object { return o }
func (o *Object) ToBoolean() bool           { return true }
func (o *Object) ToFloat() float64 {
	var f float64
	_, err := o.r.try(func() sjs.Value { f = o.r.h.num.Invoke(o.v).Float(); return sjs.Undefined() })
	if err != nil {
		panic(err)
	}
	return f
}
func (o *Object) ToInteger() int64 { return floatValue(o.ToFloat()).ToInteger() }
func (o *Object) ToNumber() Value  { return floatValue(o.ToFloat()) }
func (o *Object) ToString() Value  { return stringValue(o.String()) }
func (o *Object) String() string {
	var s string
	_, _ = o.r.try(func() sjs.Value { s = o.r.h.str.Invoke(o.v).String(); return sjs.Undefined() })
	return s
}
func (o *Object) SameAs(v Value) bool       { return o.StrictEquals(v) }
func (o *Object) Equals(v Value) bool       { return o.StrictEquals(v) }
func (o *Object) StrictEquals(v Value) bool { ov, ok := v.(*Object); return ok && ov.v.Equal(o.v) }

// MarshalJSON writes the object as JSON.stringify does.
func (o *Object) MarshalJSON() ([]byte, error) {
	var s sjs.Value
	if _, err := o.r.try(func() sjs.Value { s = o.r.h.json.Invoke(o.v); return s }); err != nil {
		return nil, err
	}
	if s.Type() != sjs.TypeString {
		return []byte("null"), nil
	}
	return []byte(s.String()), nil
}

// Get returns the property name, nil when the object has none.
func (o *Object) Get(name string) Value {
	var v sjs.Value
	_, err := o.r.try(func() sjs.Value { v = o.r.h.get.Invoke(o.v, name); return v })
	if err != nil {
		panic(err)
	}
	if v.Equal(o.r.h.missing) {
		return nil
	}
	return o.r.wrap(v)
}

func (o *Object) Set(name string, v any) error {
	val := o.r.ToValue(v).js()
	_, err := o.r.try(func() sjs.Value { o.v.Set(name, val); return sjs.Undefined() })
	return err
}

func (o *Object) Delete(name string) bool {
	var ok bool
	_, _ = o.r.try(func() sjs.Value { ok = o.r.h.del.Invoke(o.v, name).Bool(); return sjs.Undefined() })
	return ok
}

// Keys are the object's own enumerable string keys.
func (o *Object) Keys() []string {
	var ks sjs.Value
	if _, err := o.r.try(func() sjs.Value { ks = o.r.h.keys.Invoke(o.v); return ks }); err != nil {
		panic(err)
	}
	out := make([]string, ks.Length())
	for i := range out {
		out[i] = ks.Index(i).String()
	}
	return out
}

func (o *Object) GetSymbol(s *Symbol) Value {
	if s == nil {
		return nil
	}
	v := o.r.h.get.Invoke(o.v, s.v)
	if v.Equal(o.r.h.missing) {
		return nil
	}
	return o.r.wrap(v)
}

func (o *Object) SetSymbol(s *Symbol, v any) error {
	val := o.r.ToValue(v).js()
	_, err := o.r.try(func() sjs.Value { return o.r.h.set.Invoke(o.v, s.v, val) })
	return err
}

func (o *Object) DefineDataProperty(name string, v Value, writable, configurable, enumerable Flag) error {
	_, err := o.r.try(func() sjs.Value {
		return o.r.h.defineData.Invoke(o.v, name, jsOf(v), writable.js(false), configurable.js(false), enumerable.js(false))
	})
	return err
}

func (o *Object) DefineDataPropertySymbol(s *Symbol, v Value, writable, configurable, enumerable Flag) error {
	_, err := o.r.try(func() sjs.Value {
		return o.r.h.defineData.Invoke(o.v, s.v, jsOf(v), writable.js(false), configurable.js(false), enumerable.js(false))
	})
	return err
}

func (o *Object) DefineAccessorProperty(name string, getter, setter Value, configurable, enumerable Flag) error {
	_, err := o.r.try(func() sjs.Value {
		return o.r.h.defineAccessor.Invoke(o.v, name, jsOf(getter), jsOf(setter), configurable.js(false), enumerable.js(false))
	})
	return err
}

func (o *Object) Prototype() *Object {
	p := o.r.h.getProto.Invoke(o.v)
	if p.IsNull() {
		return nil
	}
	return &Object{r: o.r, v: p}
}

func (o *Object) SetPrototype(p *Object) error {
	proto := sjs.Null()
	if p != nil {
		proto = p.v
	}
	_, err := o.r.try(func() sjs.Value { return o.r.h.setProto.Invoke(o.v, proto) })
	return err
}

// ClassName is the object's [[Class]], as Object.prototype.toString
// names it.
func (o *Object) ClassName() string { return o.r.h.className.Invoke(o.v).String() }

// Export gives the object as goja does: a Go value as it is, an array as
// []any, a function as func(FunctionCall) Value, a promise as *Promise,
// a date as time.Time, and other objects as map[string]any.
func (o *Object) Export() any { return o.r.export(o, 0) }

func (o *Object) ExportType() reflect.Type {
	if o.isFunction() {
		return reflect.TypeFor[func(FunctionCall) Value]()
	}
	switch o.r.h.kind.Invoke(o.v).Int() {
	case 2:
		return reflect.TypeFor[[]any]()
	case 3:
		return reflect.TypeFor[*Promise]()
	case 4:
		return reflect.TypeFor[ArrayBuffer]()
	case 5:
		return reflect.TypeFor[time.Time]()
	case 6:
		return reflect.TypeOf(o.Export())
	}
	return reflect.TypeFor[map[string]any]()
}

func (r *Runtime) export(o *Object, depth int) any {
	if o.isFunction() {
		if id := r.h.goID.Invoke(o.v); id.Type() == sjs.TypeNumber {
			if v, ok := r.objects[id.Int()]; ok {
				return v
			}
		}
		f, _ := AssertFunction(o)
		return func(call FunctionCall) Value {
			v, err := f(call.This, call.Arguments...)
			if err != nil {
				panic(err)
			}
			return v
		}
	}
	switch r.h.kind.Invoke(o.v).Int() {
	case 2:
		n := o.v.Length()
		out := make([]any, n)
		for i := range n {
			out[i] = r.exportDeep(r.wrap(o.v.Index(i)), depth)
		}
		return out
	case 3:
		return &Promise{obj: o}
	case 4:
		u := pageBytes(r.h.buffer.Invoke(o.v))
		b := make([]byte, u.Length())
		sjs.CopyBytesToGo(b, u)
		return ArrayBuffer{data: b}
	case 5:
		ms := r.h.dateValue.Invoke(o.v).Float()
		return time.UnixMilli(int64(ms)).Add(time.Duration(math.Mod(ms, 1) * 1e6))
	case 6, 7:
		if id := r.h.goID.Invoke(o.v); id.Type() == sjs.TypeNumber {
			if v, ok := r.objects[id.Int()]; ok {
				return v
			}
		}
	}
	m := map[string]any{}
	for _, k := range o.Keys() {
		m[k] = r.exportDeep(o.Get(k), depth)
	}
	return m
}

func (r *Runtime) exportDeep(v Value, depth int) any {
	if v == nil {
		return nil
	}
	if o, ok := v.(*Object); ok {
		if depth > 64 {
			return nil
		}
		return r.export(o, depth+1)
	}
	return v.Export()
}

// pageBytes views a realm's Uint8Array as the page's, which syscall/js
// copies through.
func pageBytes(u sjs.Value) sjs.Value {
	return sjs.Global().Get("Uint8Array").New(u.Get("buffer"), u.Get("byteOffset"), u.Get("byteLength"))
}

// wrap makes a Value of a JS value.
func (r *Runtime) wrap(v sjs.Value) Value {
	switch v.Type() {
	case sjs.TypeUndefined:
		return undefinedValue{}
	case sjs.TypeNull:
		return nullValue{}
	case sjs.TypeBoolean:
		return boolValue(v.Bool())
	case sjs.TypeNumber:
		return floatValue(v.Float())
	case sjs.TypeString:
		return stringValue(v.String())
	case sjs.TypeSymbol:
		return &Symbol{v: v, desc: r.h.desc.Invoke(v).String()}
	case sjs.TypeFunction:
		return &Object{r: r, v: v, fn: true}
	}
	return &Object{r: r, v: v}
}

func (r *Runtime) wrapAll(vs []sjs.Value) []Value {
	out := make([]Value, len(vs))
	for i, v := range vs {
		out[i] = r.wrap(v)
	}
	return out
}
