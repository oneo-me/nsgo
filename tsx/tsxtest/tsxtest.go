// Package tsxtest runs a TSX app's tests, its *.test.tsx files, as Go
// tests (NSH's nsh/test): each file imports nsgo/test, whose open starts
// the app headless, or on a platform under nsgo test -target, through
// nsgotest, and whose App acts as a user does. nsgo test adds a Go test
// calling Run to the app's package.
package tsxtest

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/tsx"
	"github.com/oneo-me/nsgo/tsx/bundler"
	"github.com/oneo-me/nsgo/tsx/compiler"
)

//go:embed test.js
var testSource string

// Run runs the tests of every *.test.tsx and *.test.ts file in fsys, a
// subtest per file and per test in it.
func Run(t *testing.T, fsys fs.FS) {
	var files []string
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (strings.HasPrefix(d.Name(), ".") && p != "." || d.Name() == "node_modules") {
			return fs.SkipDir
		}
		if !d.IsDir() && (strings.HasSuffix(p, ".test.tsx") || strings.HasSuffix(p, ".test.ts")) {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		t.Skip("no *.test.tsx files")
	}
	for _, f := range files {
		t.Run(f, func(t *testing.T) { runFile(t, fsys, f) })
	}
}

// idle is how long RunSteps lets the app run before its steps: long
// enough for what it does as it settles to show in a trace, as NSH's
// nsh test <entry> waits.
const idle = 1200

// RunSteps runs steps, the body of an async test function given app, the
// app at entry in fsys opened on the network after it ran a while: what
// nsgo test <entry> runs with the steps of its standard input.
func RunSteps(t *testing.T, fsys fs.FS, entry, steps string) {
	file := path.Join(path.Dir(entry), path.Base(entry)+".steps.test.ts")
	src := fmt.Sprintf(`import { expect, open, test } from "nsgo/test";

test("steps", async () => {
const app = await open(%q, { network: "live" });
await app.wait(%d);
%s
});
`, "./"+path.Base(entry), idle, steps)
	runFile(t, withFile{fsys, file, []byte(src)}, file)
}

// withFile is an fs.FS with one more file.
type withFile struct {
	fs.FS
	name string
	data []byte
}

func (w withFile) Open(name string) (fs.File, error) {
	if name == w.name {
		return fstest.MapFS{name: {Data: w.data}}.Open(name)
	}
	return w.FS.Open(name)
}

// runner is one test file's VM.
type runner struct {
	vm   *goja.Runtime
	fsys fs.FS
	dir  string
	t    *testing.T
	reg  *goja.Object
	// apps are the apps the tests opened, whose failures outside a step
	// fail the test running.
	apps []*appTB
}

// A stepFailure is what a step that cannot be taken panics with, for
// the step's promise to reject with.
type stepFailure struct{ msg string }

// appTB is the testing.TB an app runs with: what fails a step panics with
// a stepFailure, and what the app reports failed waits for the next step,
// which rejects with it, or for the test's end.
type appTB struct {
	testing.TB
	failed []string
}

func (t *appTB) Fatalf(format string, args ...any) { panic(stepFailure{fmt.Sprintf(format, args...)}) }
func (t *appTB) Fatal(args ...any)                 { panic(stepFailure{fmt.Sprint(args...)}) }
func (t *appTB) FailNow()                          { panic(stepFailure{"the step failed"}) }
func (t *appTB) Errorf(format string, args ...any) {
	t.failed = append(t.failed, fmt.Sprintf(format, args...))
}
func (t *appTB) Error(args ...any) { t.failed = append(t.failed, fmt.Sprint(args...)) }

// try runs a step, returning why it failed: what it panicked with, or
// what the app reported failed meanwhile.
func (t *appTB) try(step func()) (err error) {
	defer func() {
		if v := recover(); v != nil {
			f, ok := v.(stepFailure)
			if !ok {
				panic(v)
			}
			err = errors.New(f.msg)
		}
		if err == nil && len(t.failed) > 0 {
			err = errors.New(strings.Join(t.failed, "\n"))
		}
		t.failed = nil
	}()
	step()
	return nil
}

func runFile(t *testing.T, fsys fs.FS, file string) {
	code, err := bundler.Compile(fsys, file)
	if err != nil {
		t.Fatal(err)
	}
	r := &runner{vm: goja.New(), fsys: fsys, dir: path.Dir(file), t: t}
	r.vm.SetFieldNameMapper(goja.UncapFieldNameMapper())
	install, err := r.vm.RunString(testSource)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := goja.AssertFunction(install)
	natives := r.vm.NewObject()
	_ = natives.Set("open", r.open)
	reg, err := f(goja.Undefined(), natives)
	if err != nil {
		t.Fatal(err)
	}
	r.reg = reg.ToObject(r.vm)
	module := r.reg.Get("module")
	_ = r.vm.Set("require", func(name string) goja.Value {
		if name == "nsgo/test" {
			return module
		}
		panic(r.vm.NewTypeError("a test file imports nsgo/test and the app's own files, not %q", name))
	})
	console := r.vm.NewObject()
	_ = console.Set("log", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			parts[i] = a.String()
		}
		r.t.Log(strings.Join(parts, " "))
		return goja.Undefined()
	})
	_ = r.vm.Set("console", console)
	// The web's base64 functions, which tests write fixtures with.
	_ = r.vm.Set("atob", func(s string) string {
		b, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(strings.Join(strings.Fields(s), ""), "="))
		if err != nil {
			panic(r.vm.NewTypeError("atob: %v", err))
		}
		return latin1(b)
	})
	_ = r.vm.Set("btoa", func(s string) string {
		b := make([]byte, 0, len(s))
		for _, c := range s {
			if c > 0xff {
				panic(r.vm.NewTypeError("btoa: a character past Latin-1"))
			}
			b = append(b, byte(c))
		}
		return base64.StdEncoding.EncodeToString(b)
	})
	if _, err := r.vm.RunScript(file, code); err != nil {
		t.Fatal(err)
	}
	var tests []struct {
		Name string
		Run  bool
	}
	call := func(name string, args ...goja.Value) goja.Value {
		fn, _ := goja.AssertFunction(r.reg.Get(name))
		v, err := fn(goja.Undefined(), args...)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if err := r.vm.ExportTo(call("tests"), &tests); err != nil {
		t.Fatal(err)
	}
	for i, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			if !tc.Run {
				t.Skip()
			}
			r.t = t
			p, _ := call("run", r.vm.ToValue(i)).Export().(*goja.Promise)
			for _, a := range r.apps {
				for _, f := range a.failed {
					t.Error(f)
				}
				a.failed = nil
			}
			switch {
			case p == nil:
			case p.State() == goja.PromiseStateRejected:
				t.Fatal(describe(p.Result()))
			case p.State() == goja.PromiseStatePending:
				t.Fatal("the test did not finish: it waits for something that never comes")
			}
		})
	}
}

// latin1 is b read as Latin-1, a character per byte, as atob gives it.
func latin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

// jsError is err as a JS Error, with the stack of the test's call.
func (r *runner) jsError(err error) *goja.Object {
	ctor, _ := goja.AssertConstructor(r.vm.Get("Error"))
	o, cerr := ctor(nil, r.vm.ToValue(err.Error()))
	if cerr != nil {
		return r.vm.NewGoError(err)
	}
	return o
}

// describe is what a test threw, with its stack.
func describe(v goja.Value) string {
	if o, ok := v.(*goja.Object); ok {
		if s := o.Get("stack"); s != nil && !goja.IsUndefined(s) {
			return s.String()
		}
	}
	return v.String()
}

// open is open(entry, options): the app entry names, relative to the test
// file, started for the test.
func (r *runner) open(call goja.FunctionCall) goja.Value {
	entry := path.Join(r.dir, call.Argument(0).String())
	o, _ := call.Argument(1).(*goja.Object)
	opt := func(k string) goja.Value {
		if o == nil {
			return nil
		}
		v := o.Get(k)
		if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
			return nil
		}
		return v
	}
	var opts []nsgotest.Option
	if s := opt("storage"); s != nil {
		var m map[string]string
		_ = r.vm.ExportTo(s, &m)
		opts = append(opts, nsgotest.Storage(m))
	}
	if w, h := opt("width"), opt("height"); w != nil && h != nil {
		opts = append(opts, nsgotest.Size(driver.Points(w.ToFloat()), driver.Points(h.ToFloat())))
	}
	env := driver.Environment{Locale: "en-US", FontScale: 1, PixelRatio: 2, OS: driver.OSIOS, Idiom: driver.IdiomPhone}
	envSet := false
	if v := opt("colorScheme"); v != nil {
		env.Dark, envSet = v.String() == "dark", true
	}
	if v := opt("locale"); v != nil {
		env.Locale, envSet = v.String(), true
	}
	if v := opt("reducedMotion"); v != nil {
		env.ReduceMotion, envSet = v.ToBoolean(), true
	}
	if v := opt("platform"); v != nil {
		env.OS, envSet = driver.OS(v.String()), true
		if env.OS != driver.OSIOS && env.OS != driver.OSAndroid {
			env.Idiom = driver.IdiomDesktop
		}
	}
	if v := opt("safeArea"); v != nil {
		var e struct{ Top, Right, Bottom, Left float64 }
		_ = r.vm.ExportTo(v, &e)
		env.SafeArea = driver.Edges{Top: driver.Points(e.Top), Right: driver.Points(e.Right),
			Bottom: driver.Points(e.Bottom), Left: driver.Points(e.Left)}
		envSet = true
	}
	if envSet {
		opts = append(opts, nsgotest.Environment(env))
	}
	// The embedding app: the test's, else the page's host module.
	tb := &appTB{TB: r.t}
	var app *nsgotest.App
	var th *testHost
	if v := opt("host"); v != nil {
		th = r.playHost(v.ToObject(r.vm), func() *nsgotest.App { return app })
		opts = append(opts, nsgotest.Host(th.h))
	} else if nsgotest.Target() == "" {
		module, err := webHost(r.fsys, entry)
		if err != nil {
			panic(r.jsError(err))
		}
		if module != "" {
			th = &testHost{h: core.NewHost()}
			opts = append(opts, nsgotest.Host(th.h), func(a *nsgotest.App) {
				if err := pageHost(r.fsys, module, a, th.h); err != nil {
					tb.Fatalf("%v", err)
				}
			})
		}
	}
	if c, ok := opt("clock").(*goja.Object); ok {
		if at := c.Get("time"); at != nil && !goja.IsUndefined(at) && !goja.IsNull(at) {
			wall := time.UnixMilli(at.ToInteger())
			if d, ok := at.Export().(time.Time); ok {
				wall = d
			}
			opts = append(opts, func(a *nsgotest.App) { a.Clock.SetWallTime(wall) })
		}
	}
	if v := opt("clipboard"); v != nil {
		text := v.String()
		opts = append(opts, func(a *nsgotest.App) { a.Clipboard = text })
	}
	net := &network{r: r}
	opts = append(opts, func(a *nsgotest.App) { net.app = a })
	if v := opt("network"); v == nil || v.String() != "live" {
		net.stubbed = true
		if v != nil {
			routes := v.ToObject(r.vm)
			for _, k := range routes.Keys() {
				rt := route(k)
				if fn, ok := goja.AssertFunction(routes.Get(k)); ok {
					rt.fn = fn
				} else {
					res, err := r.stub(routes.Get(k))
					if err != nil {
						panic(r.vm.NewTypeError("nsgo/test: network[%q]: %v", k, err))
					}
					rt.res = res
				}
				net.routes = append(net.routes, rt)
			}
		}
	}
	r.apps = append(r.apps, tb)
	p, resolve, reject := r.vm.NewPromise()
	if err := tb.try(func() { app = nsgotest.Start(tb, tsx.WithFetcher(compiler.App(r.fsys, entry), net.fetch), opts...) }); err != nil {
		_ = reject(r.jsError(err))
	} else {
		_ = resolve(r.app(app, tb, env, net, th, opt("clock") != nil))
	}
	return r.vm.ToValue(p)
}

// A stubResponse is what a route of open's network answers with.
type stubResponse struct {
	status  int
	headers http.Header
	body    []byte
	err     string
	delay   time.Duration
}

// stub reads a StubResponse: { status, headers, body, error, delay }, a
// body that is no string sent as JSON.
func (r *runner) stub(v goja.Value) (stubResponse, error) {
	o, ok := v.(*goja.Object)
	if _, fn := goja.AssertFunction(v); !ok || fn {
		return stubResponse{}, errors.New("not a response object")
	}
	res := stubResponse{status: 200, headers: http.Header{}}
	get := func(k string) goja.Value {
		if v := o.Get(k); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
			return v
		}
		return nil
	}
	if v := get("status"); v != nil {
		res.status = int(v.ToInteger())
	}
	if v := get("headers"); v != nil {
		h := v.ToObject(r.vm)
		for _, k := range h.Keys() {
			res.headers.Set(k, h.Get(k).String())
		}
	}
	if v := get("body"); v != nil {
		if s, ok := v.Export().(string); ok {
			res.body = []byte(s)
		} else {
			j, err := json.Marshal(v.Export())
			if err != nil {
				return stubResponse{}, fmt.Errorf("body: %w", err)
			}
			res.body = j
			if res.headers.Get("Content-Type") == "" {
				res.headers.Set("Content-Type", "application/json")
			}
		}
	}
	if v := get("error"); v != nil {
		res.err = v.String()
	}
	if v := get("delay"); v != nil {
		res.delay = time.Duration(max(v.ToFloat(), 0) * float64(time.Millisecond))
	}
	return res, nil
}

// A stubRoute answers the requests of a method whose URLs match: whole,
// the query included, * matching any text. It answers with res, or what
// fn, a function of the test's, returns for the request.
type stubRoute struct {
	method string
	url    *regexp.Regexp
	res    stubResponse
	fn     goja.Callable
}

// route reads a route's key, "<METHOD> <URL>", a bare URL a GET.
func route(key string) stubRoute {
	method, url := "GET", strings.TrimSpace(key)
	if m, u, ok := strings.Cut(url, " "); ok && m != "" && strings.Trim(m, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") == "" {
		method, url = strings.ToUpper(m), strings.TrimSpace(u)
	}
	parts := strings.Split(url, "*")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return stubRoute{method: method, url: regexp.MustCompile("^" + strings.Join(parts, ".*") + "$")}
}

// network is an app's stand-in network: its routes, the first that
// matches answering and a request none matches failing, or the network
// itself ("live"); it records the requests either gets.
type network struct {
	r        *runner
	app      *nsgotest.App
	stubbed  bool
	routes   []stubRoute
	mu       sync.Mutex
	requests []map[string]any
}

// answer is what a route's function returns for rec, which runs on the
// test's goroutine as a step or wait settles, its VM's: a response, or a
// promise of one, which the settles after look at until it is settled.
func (n *network) answer(fn goja.Callable, rec map[string]any, ctx context.Context) (stubResponse, error) {
	type result struct {
		res stubResponse
		err error
	}
	done := make(chan result, 1)
	var check func(p *goja.Promise)
	check = func(p *goja.Promise) {
		switch p.State() {
		case goja.PromiseStatePending:
			n.app.Post(func() { check(p) })
		case goja.PromiseStateRejected:
			done <- result{err: fmt.Errorf("nsgo/test: the route failed: %v", p.Result())}
		default:
			res, err := n.r.stub(p.Result())
			done <- result{res, err}
		}
	}
	n.app.Post(func() {
		v, err := fn(goja.Undefined(), n.r.vm.ToValue(rec))
		if err != nil {
			done <- result{err: err}
			return
		}
		if p, ok := v.Export().(*goja.Promise); ok {
			check(p)
			return
		}
		res, err := n.r.stub(v)
		done <- result{res, err}
	})
	select {
	case got := <-done:
		return got.res, got.err
	case <-ctx.Done():
		return stubResponse{}, ctx.Err()
	}
}

func (n *network) fetch(req *http.Request) (*http.Response, error) {
	rec := map[string]any{"method": req.Method, "url": req.URL.String(), "body": nil}
	headers := map[string]any{}
	for k := range req.Header {
		headers[strings.ToLower(k)] = req.Header.Get(k)
	}
	rec["headers"] = headers
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
		rec["body"] = string(body)
	}
	n.mu.Lock()
	n.requests = append(n.requests, rec)
	n.mu.Unlock()
	if !n.stubbed {
		return http.DefaultTransport.RoundTrip(req)
	}
	for _, rt := range n.routes {
		if rt.method != req.Method || !rt.url.MatchString(req.URL.String()) {
			continue
		}
		if rt.fn != nil {
			res, err := n.answer(rt.fn, rec, req.Context())
			if err != nil {
				return nil, err
			}
			rt.res = res
		}
		if rt.res.delay > 0 {
			select {
			case <-time.After(rt.res.delay):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}
		if rt.res.err != "" {
			return nil, errors.New(rt.res.err)
		}
		return &http.Response{
			StatusCode: rt.res.status, Status: fmt.Sprintf("%d %s", rt.res.status, http.StatusText(rt.res.status)),
			Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
			Header: rt.res.headers.Clone(), Body: io.NopCloser(bytes.NewReader(rt.res.body)),
			ContentLength: int64(len(rt.res.body)), Request: req,
		}, nil
	}
	return nil, fmt.Errorf("nsgo/test: no route of open's network for %s %s", req.Method, req.URL)
}

// selector is NSH's selector as nsgotest reads it: text:Save, a whole
// text, is text:"Save", and role:button[Save] is role:button "Save".
var roleName = regexp.MustCompile(`^role:([a-z]+)\[(.*)\]$`)

func selector(s string) string {
	parts := strings.Split(s, " >> ")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if m := roleName.FindStringSubmatch(p); m != nil {
			name := m[2]
			if !strings.HasPrefix(name, "/") {
				name = `"` + name + `"`
			}
			parts[i] = "role:" + m[1] + " " + name
		} else if t, ok := strings.CutPrefix(p, "text:"); ok && !strings.HasPrefix(t, "/") {
			parts[i] = `text:"` + t + `"`
		}
	}
	return strings.Join(parts, " >> ")
}

// app is a JS App over an nsgotest app.
func (r *runner) app(a *nsgotest.App, tb *appTB, env driver.Environment, net *network, th *testHost, manual bool) *goja.Object {
	o := r.vm.NewObject()
	// until settles until cond holds, failing after timeout of real time.
	// Without open's clock the app's time passes meanwhile, a frame at a
	// time, as the system's clock that NSH's app runs on then moves.
	until := func(timeout time.Duration, cond func() bool) {
		if manual || nsgotest.Target() != "" {
			a.WaitUntil(timeout, cond)
			return
		}
		deadline := time.Now().Add(timeout)
		for {
			a.Settle()
			if cond() {
				return
			}
			if time.Now().After(deadline) {
				tb.Fatalf("nsgotest: the condition did not hold after %v\n%s", timeout, a.Dump())
			}
			a.Advance(driver.FrameInterval)
			time.Sleep(time.Millisecond)
		}
	}
	// step runs f as a step: a promise that resolves once the app is
	// quiet, or rejects with why the step could not be taken or what the
	// app threw.
	step := func(f func()) goja.Value {
		p, resolve, reject := r.vm.NewPromise()
		if err := tb.try(f); err != nil {
			_ = reject(r.jsError(err))
		} else {
			_ = resolve(goja.Undefined())
		}
		return r.vm.ToValue(p)
	}
	act := func(name string, f func(call goja.FunctionCall)) {
		_ = o.Set(name, func(call goja.FunctionCall) goja.Value { return step(func() { f(call) }) })
	}
	// read reads the app now, throwing what fails.
	read := func(name string, f func(call goja.FunctionCall) any) {
		_ = o.Set(name, func(call goja.FunctionCall) goja.Value {
			var v any
			if err := tb.try(func() { v = f(call) }); err != nil {
				panic(r.jsError(err))
			}
			return r.vm.ToValue(v)
		})
	}
	sel := func(call goja.FunctionCall) string { return selector(call.Argument(0).String()) }
	ms := func(v goja.Value) time.Duration { return time.Duration(v.ToFloat() * float64(time.Millisecond)) }
	act("press", func(c goja.FunctionCall) { a.Press(sel(c)) })
	act("longPress", func(c goja.FunctionCall) { a.LongPress(sel(c)) })
	act("type", func(c goja.FunctionCall) { a.Type(sel(c), c.Argument(1).String()) })
	act("submit", func(c goja.FunctionCall) { a.Submit(sel(c)) })
	act("clear", func(c goja.FunctionCall) { a.Clear(sel(c)) })
	act("scroll", func(c goja.FunctionCall) { a.Scroll(sel(c), driver.Points(c.Argument(1).ToFloat())) })
	act("drag", func(c goja.FunctionCall) {
		var by struct{ X, Y float64 }
		_ = r.vm.ExportTo(c.Argument(1), &by)
		a.Drag(sel(c), driver.Point{X: driver.Points(by.X), Y: driver.Points(by.Y)})
	})
	act("rightClick", func(c goja.FunctionCall) { a.RightClick(sel(c)) })
	act("hover", func(c goja.FunctionCall) { a.Hover(sel(c)) })
	act("chooseMenuItem", func(c goja.FunctionCall) { a.ChoosePopup(strings.Split(c.Argument(0).String(), " > ")...) })
	act("closeMenu", func(goja.FunctionCall) { a.ClosePopup() })
	act("back", func(goja.FunctionCall) { a.Back() })
	act("wait", func(c goja.FunctionCall) { a.Advance(ms(c.Argument(0))) })
	act("paste", func(c goja.FunctionCall) {
		var pc struct {
			Text string
			Uris []string
		}
		_ = r.vm.ExportTo(c.Argument(1), &pc)
		a.Paste(sel(c), driver.PasteEvent{Text: pc.Text, URIs: pc.Uris})
	})
	timeout := func(v goja.Value) time.Duration {
		if o, ok := v.(*goja.Object); ok {
			if t := o.Get("timeout"); t != nil && !goja.IsUndefined(t) {
				return ms(t)
			}
		}
		return 5 * time.Second
	}
	act("waitFor", func(c goja.FunctionCall) {
		state := ""
		if o, ok := c.Argument(1).(*goja.Object); ok {
			state = fmt.Sprint(o.Get("state"))
		}
		if state == "gone" {
			until(timeout(c.Argument(1)), func() bool { return !a.Exists(sel(c)) })
		} else {
			until(timeout(c.Argument(1)), func() bool { return a.Exists(sel(c)) })
		}
	})
	act("waitUntil", func(c goja.FunctionCall) {
		cond, _ := goja.AssertFunction(c.Argument(0))
		until(timeout(c.Argument(1)), func() bool {
			v, err := cond(goja.Undefined())
			return err == nil && v.ToBoolean()
		})
	})
	act("setViewport", func(c goja.FunctionCall) {
		a.Resize(driver.Points(c.Argument(0).ToFloat()), driver.Points(c.Argument(1).ToFloat()))
	})
	act("setColorScheme", func(c goja.FunctionCall) {
		env.Dark = c.Argument(0).String() == "dark"
		a.SetEnvironment(env)
	})
	act("screenshot", func(c goja.FunctionCall) {
		name := "screenshot"
		if v := c.Argument(0); !goja.IsUndefined(v) && !goja.IsNull(v) {
			name = strings.TrimSuffix(path.Base(v.String()), ".png")
		}
		a.Screenshot(name)
	})
	read("count", func(c goja.FunctionCall) any { return len(a.Query(sel(c))) })
	read("shows", func(c goja.FunctionCall) any { return a.Exists(sel(c)) })
	read("enabled", func(c goja.FunctionCall) any { return a.Enabled(sel(c)) })
	read("texts", func(goja.FunctionCall) any { return a.Texts() })
	read("value", func(c goja.FunctionCall) any {
		if !a.Exists(sel(c)) {
			return nil
		}
		return a.Find(sel(c)).Text
	})
	read("storage", func(goja.FunctionCall) any {
		s, out := a.Storage(), map[string]any{}
		for _, k := range s.Keys() {
			v, _ := s.GetItem(k)
			out[k] = v
		}
		return out
	})
	read("dialogs", func(goja.FunctionCall) any {
		out := []map[string]any{}
		for _, d := range a.Alerts() {
			out = append(out, dialog(d))
		}
		return out
	})
	read("answer", func(c goja.FunctionCall) any {
		a.Answer(int(c.Argument(0).ToInteger()))
		return nil
	})
	read("requests", func(goja.FunctionCall) any {
		net.mu.Lock()
		defer net.mu.Unlock()
		return append([]map[string]any{}, net.requests...)
	})
	read("openedUrls", func(goja.FunctionCall) any { return append([]string{}, a.OpenedURLs...) })
	read("clipboard", func(goja.FunctionCall) any {
		if a.Clipboard == "" {
			return nil
		}
		return a.Clipboard
	})
	read("picks", func(goja.FunctionCall) any {
		out := []map[string]any{}
		for _, p := range a.Picks {
			out = append(out, pick(p))
		}
		return out
	})
	read("menu", func(goja.FunctionCall) any {
		if a.PopupMenu == nil {
			return nil
		}
		return menuEntries(a.PopupMenu, "", false, []map[string]any{})
	})
	read("spanStyle", func(c goja.FunctionCall) any {
		sp := a.Span(sel(c))
		s := sp.Style
		hex := func(c driver.Color) string { return fmt.Sprintf("#%08x", uint32(c)) }
		out := map[string]any{"color": hex(s.Color), "fontSize": float64(s.Font.Size), "fontWeight": float64(s.Font.Weight),
			"italic": s.Font.Italic, "underline": s.Decoration&driver.TextDecorationUnderline != 0,
			"lineThrough": s.Decoration&driver.TextDecorationLineThrough != 0, "backgroundColor": nil}
		if bg := sp.Background.Color; bg != 0 {
			out["backgroundColor"] = hex(bg)
		}
		return out
	})
	read("onDialog", func(c goja.FunctionCall) any {
		h, ok := goja.AssertFunction(c.Argument(0))
		if !ok {
			a.OnAlert = nil
			return nil
		}
		a.OnAlert = func(d driver.AlertConfig) int {
			res, err := h(goja.Undefined(), r.vm.ToValue(dialog(d)))
			if err != nil {
				panic(err)
			}
			if res == nil || goja.IsNull(res) || goja.IsUndefined(res) {
				return -1
			}
			for i, b := range d.Actions {
				if b.Label == res.String() {
					return i
				}
			}
			panic(r.vm.NewTypeError("nsgo/test: the dialog %q has no button %q", d.Title, res.String()))
		}
		return nil
	})
	read("onPick", func(c goja.FunctionCall) any {
		h, ok := goja.AssertFunction(c.Argument(0))
		if !ok {
			a.PickImages = nil
			return nil
		}
		a.PickImages = func(p driver.PickRequest) []driver.PickedFile {
			res, err := h(goja.Undefined(), r.vm.ToValue(pick(p)))
			if err != nil {
				panic(err)
			}
			if res == nil || goja.IsNull(res) || goja.IsUndefined(res) {
				return nil
			}
			var files []driver.PickedFile
			list := res.ToObject(r.vm)
			for i := range int(list.Get("length").ToInteger()) {
				f := list.Get(strconv.Itoa(i)).ToObject(r.vm)
				pf := driver.PickedFile{Name: f.Get("name").String(), Data: r.bytes(f.Get("data"))}
				if t := f.Get("type"); t != nil && !goja.IsUndefined(t) && !goja.IsNull(t) {
					pf.MIME = t.String()
				} else {
					pf.MIME = mime.TypeByExtension(path.Ext(pf.Name))
				}
				files = append(files, pf)
			}
			return files
		}
		return nil
	})
	keyboard := r.vm.NewObject()
	_ = keyboard.Set("press", func(c goja.FunctionCall) goja.Value {
		return step(func() { a.Key(c.Argument(0).String()) })
	})
	_ = o.Set("keyboard", keyboard)
	clock := r.vm.NewObject()
	_ = clock.Set("runFor", func(c goja.FunctionCall) goja.Value {
		return step(func() { a.Advance(ms(c.Argument(0))) })
	})
	_ = clock.Set("fastForward", func(c goja.FunctionCall) goja.Value {
		return step(func() { a.FastForward(ms(c.Argument(0))) })
	})
	_ = clock.Set("setSystemTime", func(c goja.FunctionCall) goja.Value {
		at := c.Argument(0)
		if d, ok := at.Export().(time.Time); ok {
			a.Clock.SetWallTime(d)
		} else {
			a.Clock.SetWallTime(time.UnixMilli(at.ToInteger()))
		}
		return goja.Undefined()
	})
	_ = o.Set("clock", clock)
	// host is the embedding app the test plays: its events to the app,
	// and the calls and posts of the names open's host handles.
	host := r.vm.NewObject()
	_ = host.Set("send", func(c goja.FunctionCall) goja.Value {
		return step(func() {
			if th == nil {
				tb.Fatalf("nsgo/test: the app has no host: open it with one (open's host)")
			}
			if err := th.h.Send(c.Argument(0).String(), exportData(c.Argument(1))); err != nil {
				tb.Fatalf("nsgo/test: host.send: %v", err)
			}
			a.Settle()
		})
	})
	messages := func(list func() []map[string]any) func(goja.FunctionCall) goja.Value {
		return func(goja.FunctionCall) goja.Value {
			out := []map[string]any{}
			if th != nil {
				out = append(out, list()...)
			}
			return r.vm.ToValue(out)
		}
	}
	_ = host.Set("calls", messages(func() []map[string]any { return th.calls }))
	_ = host.Set("posts", messages(func() []map[string]any { return th.posts }))
	_ = o.Set("host", host)
	return o
}

// exportData is a value a test sends the app: undefined is null.
func exportData(v goja.Value) any {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	return v.Export()
}

// dialog is a Dialog as a test reads it.
func dialog(d driver.AlertConfig) map[string]any {
	buttons := []string{}
	for _, b := range d.Actions {
		buttons = append(buttons, b.Label)
	}
	return map[string]any{"title": d.Title, "message": d.Message, "buttons": buttons,
		"cancelable": d.Cancelable, "sheet": d.Sheet}
}

// pick is a Pick as a test reads it.
func pick(p driver.PickRequest) map[string]any {
	o := map[string]any{"multiple": p.Limit != 1, "limit": p.Limit, "maxDimension": nil, "quality": nil, "format": "compatible"}
	if p.MaxDimension > 0 {
		o["maxDimension"] = p.MaxDimension
	}
	if p.Quality > 0 {
		o["quality"] = float64(p.Quality)
	}
	if p.Format == driver.PickCurrent {
		o["format"] = "current"
	}
	return o
}

// menuEntries are a context menu's actions, a submenu's after its label
// ("Share > Mail"), disabled with a disabled submenu around them.
func menuEntries(items []driver.MenuEntry, prefix string, disabled bool, out []map[string]any) []map[string]any {
	for _, e := range items {
		if e.Separator {
			continue
		}
		if len(e.Items) > 0 {
			out = menuEntries(e.Items, prefix+e.Label+" > ", disabled || e.Disabled, out)
			continue
		}
		m := map[string]any{"label": prefix + e.Label, "disabled": disabled || e.Disabled}
		if e.Checkable {
			m["checked"] = e.Checked
		}
		out = append(out, m)
	}
	return out
}

// bytes reads a Uint8Array, an ArrayBuffer, an array of bytes or a string.
func (r *runner) bytes(v goja.Value) []byte {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	switch x := v.Export().(type) {
	case goja.ArrayBuffer:
		return x.Bytes()
	case []byte:
		return x
	case string:
		return []byte(x)
	}
	o := v.ToObject(r.vm)
	if buf, ok := o.Get("buffer").Export().(goja.ArrayBuffer); ok {
		off, n := o.Get("byteOffset").ToInteger(), o.Get("byteLength").ToInteger()
		return buf.Bytes()[off : off+n]
	}
	out := make([]byte, o.Get("length").ToInteger())
	for i := range out {
		out[i] = byte(o.Get(strconv.Itoa(i)).ToInteger())
	}
	return out
}
