package js

import (
	"errors"
	"strings"
	"testing"
)

// The engines' shared behavior: what tsx relies on, on goja and on the
// page's engine alike.

func newRuntime(t *testing.T) *Runtime {
	t.Helper()
	r := New()
	r.SetFieldNameMapper(UncapFieldNameMapper())
	if c, ok := any(r).(interface{ Close() }); ok {
		t.Cleanup(c.Close)
	}
	return r
}

func run(t *testing.T, r *Runtime, src string) Value {
	t.Helper()
	v, err := r.RunString(src)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRealm(t *testing.T) {
	r := newRuntime(t)
	got := run(t, r, "typeof document + ' ' + (globalThis.frameElement ? frameElement.tagName : 'none')").String()
	want := "undefined none"
	if Browser {
		want = "object IFRAME"
	}
	if got != want {
		t.Errorf("the realm is %q, want %q", got, want)
	}
	// Each runtime is a realm of its own.
	run(t, r, "globalThis.mark = 1")
	if v := run(t, newRuntime(t), "typeof mark").String(); v != "undefined" {
		t.Errorf("another runtime sees the mark: %s", v)
	}
}

func TestExport(t *testing.T) {
	r := newRuntime(t)
	cases := map[string]any{
		"3":         int64(3),
		"1.5":       1.5,
		"'a'":       "a",
		"true":      true,
		"null":      nil,
		"undefined": nil,
	}
	for src, want := range cases {
		if got := run(t, r, src).Export(); got != want {
			t.Errorf("%s exports %#v, want %#v", src, got, want)
		}
	}
	arr, ok := run(t, r, "[1, 'b', [2]]").Export().([]any)
	if !ok || len(arr) != 3 || arr[0] != int64(1) || arr[1] != "b" {
		t.Errorf("an array exports %#v", arr)
	}
	m, ok := run(t, r, "({ a: 1, b: { c: 'd' } })").Export().(map[string]any)
	if !ok || m["a"] != int64(1) || m["b"].(map[string]any)["c"] != "d" {
		t.Errorf("an object exports %#v", m)
	}
	if _, ok := run(t, r, "Promise.resolve(1)").Export().(*Promise); !ok {
		t.Error("a promise does not export as one")
	}
	if b, ok := run(t, r, "new Uint8Array([1, 2]).buffer").Export().(ArrayBuffer); !ok || string(b.Bytes()) != "\x01\x02" {
		t.Errorf("an ArrayBuffer exports %#v", b)
	}
}

func TestNumbersAsJS(t *testing.T) {
	r := newRuntime(t)
	for src, want := range map[string]string{"0.1 + 0.2": "0.30000000000000004", "1e21": "1e+21", "1/3": "0.3333333333333333",
		"-0": "0", "2 ** 53": "9007199254740992", "1e-7": "1e-7", "NaN": "NaN"} {
		if got := run(t, r, src).String(); got != want {
			t.Errorf("%s is %q, want %q", src, got, want)
		}
	}
}

type point struct{ X, Y float64 }

func (p *point) Sum() float64          { return p.X + p.Y }
func (p *point) Scale(by float64) bool { p.X *= by; p.Y *= by; return true }

func TestGoValues(t *testing.T) {
	r := newRuntime(t)
	p := &point{X: 1, Y: 2}
	r.Set("p", p)
	if v := run(t, r, "p.scale(2); p.sum() + p.x").ToFloat(); v != 8 {
		t.Errorf("the struct's methods and fields read %v", v)
	}
	if got := r.Get("p").Export(); got != p {
		t.Errorf("the struct exports %#v", got)
	}
	r.Set("join", func(sep string, parts ...string) string { return strings.Join(parts, sep) })
	if v := run(t, r, "join('-', 'a', 'b', 'c')").String(); v != "a-b-c" {
		t.Errorf("a variadic function returns %q", v)
	}
	r.Set("fail", func() (int, error) { return 0, errors.New("no") })
	if v := run(t, r, "try { fail(); 'ran' } catch (e) { e.message }").String(); !strings.Contains(v, "no") {
		t.Errorf("a Go error throws %q", v)
	}
	var by struct {
		Text string
		Uris []string
	}
	if err := r.ExportTo(run(t, r, "({ text: 'hi', uris: ['a'] })"), &by); err != nil || by.Text != "hi" || len(by.Uris) != 1 {
		t.Errorf("ExportTo gives %+v, %v", by, err)
	}
}

func TestCalls(t *testing.T) {
	r := newRuntime(t)
	r.Set("twice", func(call FunctionCall) Value {
		f, _ := AssertFunction(call.Argument(0))
		v, err := f(Undefined(), r.ToValue(21))
		if err != nil {
			panic(err)
		}
		return r.ToValue(v.ToInteger() * 2)
	})
	if v := run(t, r, "twice((n) => n)").ToInteger(); v != 42 {
		t.Errorf("a call back into JS gives %d", v)
	}
	// What JS throws comes back as an Exception, its value the thrown one.
	f, _ := AssertFunction(run(t, r, "(function () { throw new TypeError('bad'); })"))
	_, err := f(Undefined())
	var ex *Exception
	if !errors.As(err, &ex) || !strings.Contains(ex.Value().String(), "TypeError: bad") {
		t.Errorf("a throw comes back as %v", err)
	}
	// A JS error Go throws is JS's to catch.
	r.Set("refuse", func(FunctionCall) Value { panic(r.NewTypeError("refused")) })
	if v := run(t, r, "try { refuse() } catch (e) { e instanceof TypeError && e.message }").String(); v != "refused" {
		t.Errorf("a thrown TypeError reads %q", v)
	}
	// A constructor made in Go.
	r.Set("Box", func(call ConstructorCall) *Object {
		_ = call.This.Set("value", call.Argument(0))
		return nil
	})
	if v := run(t, r, "new Box(5).value").ToInteger(); v != 5 {
		t.Errorf("a Go constructor's object holds %d", v)
	}
}

func TestGoPanicsReachGo(t *testing.T) {
	r := newRuntime(t)
	r.Set("explode", func(FunctionCall) Value { panic("boom") })
	f, _ := AssertFunction(run(t, r, "(function () { try { explode(); } catch (e) {} return 1; })"))
	defer func() {
		if p := recover(); p != "boom" {
			t.Errorf("the panic came back as %v", p)
		}
	}()
	_, _ = f(Undefined())
	t.Error("the panic did not reach Go")
}

func TestObjects(t *testing.T) {
	r := newRuntime(t)
	o := r.NewObject()
	_ = o.Set("a", 1)
	if o.Get("missing") != nil {
		t.Error("a missing property is not nil")
	}
	if v := o.Get("a"); v == nil || v.ToInteger() != 1 {
		t.Errorf("a property reads %v", v)
	}
	s := NewSymbol("key")
	_ = o.SetSymbol(s, "hidden")
	if v := o.GetSymbol(s); v == nil || v.String() != "hidden" {
		t.Errorf("a symbol's property reads %v", v)
	}
	if keys := o.Keys(); len(keys) != 1 || keys[0] != "a" {
		t.Errorf("the keys are %q", keys)
	}
	getter := r.ToValue(func(FunctionCall) Value { return r.ToValue("got") })
	_ = o.DefineAccessorProperty("g", getter, nil, FLAG_TRUE, FLAG_TRUE)
	r.Set("o", o)
	if v := run(t, r, "o.g").String(); v != "got" {
		t.Errorf("an accessor reads %q", v)
	}
	if cn := run(t, r, "[]").(*Object).ClassName(); cn != "Array" {
		t.Errorf("an array's class is %q", cn)
	}
}
