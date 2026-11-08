//go:build js

package js

import (
	"fmt"
	"reflect"
	sjs "syscall/js"
	"time"
)

var (
	valueType     = reflect.TypeFor[Value]()
	objectType    = reflect.TypeFor[*Object]()
	errorType     = reflect.TypeFor[error]()
	callType      = reflect.TypeFor[FunctionCall]()
	anyType       = reflect.TypeFor[any]()
	timeType      = reflect.TypeFor[time.Time]()
	arrayBufType  = reflect.TypeFor[ArrayBuffer]()
	stringMapType = reflect.TypeFor[map[string]any]()
)

// ToValue makes a JS value of a Go one, as goja does: primitives as
// themselves, functions as JS functions converting their arguments and
// results, slices and maps as arrays and objects of their items, and
// other values as objects whose properties are their fields and
// methods, exported back as the value.
func (r *Runtime) ToValue(i any) Value {
	switch v := i.(type) {
	case nil:
		return nullValue{}
	case Value:
		if o, ok := v.(*Object); ok && o == nil {
			return nullValue{}
		}
		return v
	case *Promise:
		if v == nil {
			return nullValue{}
		}
		return v.obj
	case ArrayBuffer:
		u := r.h.bytes.Invoke(len(v.data))
		sjs.CopyBytesToJS(pageBytes(u), v.data)
		return &Object{r: r, v: u.Get("buffer")}
	case *ArrayBuffer:
		return r.ToValue(*v)
	case string:
		return stringValue(v)
	case bool:
		return boolValue(v)
	case int:
		return floatValue(v)
	case int64:
		return floatValue(v)
	case int32:
		return floatValue(v)
	case uint32:
		return floatValue(v)
	case float64:
		return floatValue(v)
	case float32:
		return floatValue(v)
	case func(FunctionCall) Value:
		return r.function(v)
	case func(ConstructorCall) *Object:
		return r.newFunc(func(this, target sjs.Value, args []sjs.Value) sjs.Value {
			if target.IsUndefined() {
				panic(r.NewTypeError("Class constructor cannot be invoked without 'new'"))
			}
			res := v(ConstructorCall{This: &Object{r: r, v: this}, NewTarget: &Object{r: r, v: target, fn: true}, Arguments: r.wrapAll(args)})
			if res == nil {
				return sjs.Undefined()
			}
			return res.v
		}, true)
	}
	return r.reflectValue(reflect.ValueOf(i))
}

func (r *Runtime) reflectValue(v reflect.Value) Value {
	if v.IsValid() && v.CanInterface() {
		switch x := v.Interface().(type) {
		case Value, *Promise, ArrayBuffer, *ArrayBuffer, func(FunctionCall) Value, func(ConstructorCall) *Object:
			return r.ToValue(x)
		}
	}
	switch v.Kind() {
	case reflect.Invalid:
		return nullValue{}
	case reflect.Bool:
		return boolValue(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return floatValue(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return floatValue(v.Uint())
	case reflect.Float32, reflect.Float64:
		return floatValue(v.Float())
	case reflect.String:
		return stringValue(v.String())
	case reflect.Func:
		if v.IsNil() {
			return nullValue{}
		}
		return r.reflectFunc(v)
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return nullValue{}
		}
		items := make([]any, v.Len())
		for i := range items {
			items[i] = r.reflectValue(v.Index(i)).js()
		}
		return &Object{r: r, v: r.h.array.Invoke(items...)}
	case reflect.Map:
		if v.IsNil() {
			return nullValue{}
		}
		o := r.NewObject()
		it := v.MapRange()
		for it.Next() {
			o.v.Set(fmt.Sprint(it.Key().Interface()), r.reflectValue(it.Value()).js())
		}
		return o
	case reflect.Interface:
		if v.IsNil() {
			return nullValue{}
		}
		return r.ToValue(v.Elem().Interface())
	case reflect.Pointer:
		if v.IsNil() {
			return nullValue{}
		}
	}
	if v.Type() == timeType {
		t := v.Interface().(time.Time)
		return &Object{r: r, v: r.h.date.Invoke(float64(t.UnixNano()) / 1e6)}
	}
	return r.goObject(v)
}

// goObject makes an object of a Go value: its prototype holds the type's
// methods and fields, and the object its ID.
func (r *Runtime) goObject(v reflect.Value) *Object {
	proto := r.proto(v.Type())
	r.next++
	id := r.next
	r.objects[id] = v.Interface()
	return &Object{r: r, v: r.h.goObject.Invoke(proto, id)}
}

func (r *Runtime) receiver(this sjs.Value) (reflect.Value, bool) {
	id := r.h.goID.Invoke(this)
	if id.Type() != sjs.TypeNumber {
		return reflect.Value{}, false
	}
	v, ok := r.objects[id.Int()]
	if !ok {
		return reflect.Value{}, false
	}
	return reflect.ValueOf(v), true
}

func (r *Runtime) proto(t reflect.Type) sjs.Value {
	if p, ok := r.protos[t]; ok {
		return p
	}
	p := r.global.Get("Object").New()
	r.protos[t] = p
	for i := range t.NumMethod() {
		m := t.Method(i)
		name := r.mapper.MethodName(t, m)
		if name == "" {
			continue
		}
		index := i
		f := r.newFunc(func(this, _ sjs.Value, args []sjs.Value) sjs.Value {
			recv, ok := r.receiver(this)
			if !ok {
				panic(r.NewTypeError("%s called on another object", name))
			}
			return r.callReflect(recv.Method(index), this, args)
		}, false)
		p.Set(name, f.v)
	}
	st := t
	if st.Kind() == reflect.Pointer {
		st = st.Elem()
	}
	if st.Kind() == reflect.Struct {
		for _, f := range reflect.VisibleFields(st) {
			if !f.IsExported() || f.Anonymous {
				continue
			}
			name := r.mapper.FieldName(st, f)
			if name == "" {
				continue
			}
			index := f.Index
			field := func(this sjs.Value) reflect.Value {
				recv, ok := r.receiver(this)
				if !ok {
					panic(r.NewTypeError("%s read on another object", name))
				}
				if recv.Kind() == reflect.Pointer {
					recv = recv.Elem()
				}
				return recv.FieldByIndex(index)
			}
			get := r.newFunc(func(this, _ sjs.Value, _ []sjs.Value) sjs.Value {
				return r.reflectValue(field(this)).js()
			}, false)
			set := r.newFunc(func(this, _ sjs.Value, args []sjs.Value) sjs.Value {
				fv := field(this)
				if !fv.CanSet() || len(args) == 0 {
					return sjs.Undefined()
				}
				x, err := r.exportAs(r.wrap(args[0]), fv.Type())
				if err != nil {
					panic(r.NewTypeError("%s: %v", name, err))
				}
				fv.Set(x)
				return sjs.Undefined()
			}, false)
			r.h.defineAccessor.Invoke(p, name, get.v, set.v, true, true)
		}
	}
	return p
}

// reflectFunc makes a JS function of a Go function of any type.
func (r *Runtime) reflectFunc(fn reflect.Value) *Object {
	return r.newFunc(func(this, _ sjs.Value, args []sjs.Value) sjs.Value {
		return r.callReflect(fn, this, args)
	}, false)
}

// callReflect calls fn with JS's arguments converted to its parameters,
// and makes its results JS's: none undefined, one itself, and a trailing
// error thrown.
func (r *Runtime) callReflect(fn reflect.Value, this sjs.Value, args []sjs.Value) sjs.Value {
	ft := fn.Type()
	if ft.NumIn() == 1 && ft.In(0) == callType && !ft.IsVariadic() {
		out := fn.Call([]reflect.Value{reflect.ValueOf(FunctionCall{This: r.wrap(this), Arguments: r.wrapAll(args)})})
		return r.results(out)
	}
	n := ft.NumIn()
	in := make([]reflect.Value, 0, max(n, len(args)))
	for i := range n {
		t := ft.In(i)
		if ft.IsVariadic() && i == n-1 {
			et := t.Elem()
			for _, a := range args[min(i, len(args)):] {
				x, err := r.exportAs(r.wrap(a), et)
				if err != nil {
					panic(r.NewTypeError("argument %d: %v", i, err))
				}
				in = append(in, x)
			}
			break
		}
		var a Value = undefinedValue{}
		if i < len(args) {
			a = r.wrap(args[i])
		}
		x, err := r.exportAs(a, t)
		if err != nil {
			panic(r.NewTypeError("argument %d: %v", i, err))
		}
		in = append(in, x)
	}
	return r.results(fn.Call(in))
}

func (r *Runtime) results(out []reflect.Value) sjs.Value {
	if n := len(out); n > 0 && out[n-1].Type() == errorType {
		if err, _ := out[n-1].Interface().(error); err != nil {
			panic(r.NewGoError(err))
		}
		out = out[:n-1]
	}
	switch len(out) {
	case 0:
		return sjs.Undefined()
	case 1:
		if out[0].Type() == valueType && out[0].IsNil() {
			return sjs.Undefined()
		}
		return r.reflectValue(out[0]).js()
	}
	items := make([]any, len(out))
	for i, o := range out {
		items[i] = r.reflectValue(o).js()
	}
	return r.h.array.Invoke(items...)
}

// exportAs converts v to a Go value of type t, as goja's ExportTo does.
func (r *Runtime) exportAs(v Value, t reflect.Type) (reflect.Value, error) {
	if v == nil {
		v = undefinedValue{}
	}
	switch t {
	case valueType:
		return reflect.ValueOf(&v).Elem(), nil
	case objectType:
		if o, ok := v.(*Object); ok {
			return reflect.ValueOf(o), nil
		}
		if IsUndefined(v) || IsNull(v) {
			return reflect.Zero(t), nil
		}
		return reflect.ValueOf(v.ToObject(r)), nil
	case anyType:
		e := v.Export()
		if e == nil {
			return reflect.Zero(t), nil
		}
		return reflect.ValueOf(e), nil
	case callType:
		return reflect.Value{}, fmt.Errorf("cannot convert to %v", t)
	}
	if IsUndefined(v) || IsNull(v) {
		return reflect.Zero(t), nil
	}
	if e := v.Export(); e != nil {
		if ev := reflect.ValueOf(e); ev.Type().AssignableTo(t) {
			if _, plain := e.(map[string]any); !plain || t == stringMapType || t.Kind() == reflect.Interface {
				out := reflect.New(t).Elem()
				out.Set(ev)
				return out, nil
			}
		}
	}
	out := reflect.New(t).Elem()
	switch t.Kind() {
	case reflect.Bool:
		out.SetBool(v.ToBoolean())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		out.SetInt(v.ToInteger())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		out.SetUint(uint64(v.ToInteger()))
	case reflect.Float32, reflect.Float64:
		out.SetFloat(v.ToFloat())
	case reflect.String:
		out.SetString(v.String())
	case reflect.Slice:
		o, ok := v.(*Object)
		if !ok {
			return out, fmt.Errorf("cannot convert %s to %v", v, t)
		}
		n := int(o.Get("length").ToInteger())
		s := reflect.MakeSlice(t, n, n)
		for i := range n {
			item := r.wrap(o.v.Index(i))
			x, err := r.exportAs(item, t.Elem())
			if err != nil {
				return out, err
			}
			s.Index(i).Set(x)
		}
		out.Set(s)
	case reflect.Map:
		o, ok := v.(*Object)
		if !ok {
			return out, fmt.Errorf("cannot convert %s to %v", v, t)
		}
		m := reflect.MakeMap(t)
		for _, k := range o.Keys() {
			kv, err := r.exportAs(stringValue(k), t.Key())
			if err != nil {
				return out, err
			}
			x, err := r.exportAs(o.Get(k), t.Elem())
			if err != nil {
				return out, err
			}
			m.SetMapIndex(kv, x)
		}
		out.Set(m)
	case reflect.Struct:
		o, ok := v.(*Object)
		if !ok {
			return out, fmt.Errorf("cannot convert %s to %v", v, t)
		}
		for _, f := range reflect.VisibleFields(t) {
			if !f.IsExported() || f.Anonymous {
				continue
			}
			fv := o.Get(r.mapper.FieldName(t, f))
			if fv == nil {
				continue
			}
			x, err := r.exportAs(fv, f.Type)
			if err != nil {
				return out, err
			}
			out.FieldByIndex(f.Index).Set(x)
		}
	case reflect.Pointer:
		x, err := r.exportAs(v, t.Elem())
		if err != nil {
			return out, err
		}
		p := reflect.New(t.Elem())
		p.Elem().Set(x)
		out.Set(p)
	case reflect.Func:
		f, ok := AssertFunction(v)
		if !ok {
			return out, fmt.Errorf("cannot convert %s to %v", v, t)
		}
		out.Set(reflect.MakeFunc(t, func(in []reflect.Value) []reflect.Value {
			args := make([]Value, len(in))
			for i, a := range in {
				args[i] = r.reflectValue(a)
			}
			res, err := f(undefinedValue{}, args...)
			if err != nil {
				panic(err)
			}
			outs := make([]reflect.Value, t.NumOut())
			for i := range outs {
				outs[i] = reflect.Zero(t.Out(i))
			}
			if t.NumOut() > 0 {
				if x, err := r.exportAs(res, t.Out(0)); err == nil {
					outs[0] = x
				}
			}
			return outs
		}))
	case reflect.Interface:
		e := v.Export()
		if e != nil && reflect.TypeOf(e).Implements(t) {
			out.Set(reflect.ValueOf(e))
		} else if e != nil {
			return out, fmt.Errorf("cannot convert %s to %v", v, t)
		}
	default:
		return out, fmt.Errorf("cannot convert %s to %v", v, t)
	}
	return out, nil
}
