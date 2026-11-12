package tsx

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/core"
)

// The services of nsgo (NSH's runtime registry): the keyboard, key
// bindings, the clipboard, storage, the photo picker, opening URLs,
// speech, accessibility, images, the environment's insets, design tokens,
// resources and native components, each on core's API of the same
// meaning. What answers later is a promise core's callback settles.

var serviceExports = []export{
	{"Keyboard", func(r *runtime) js.Value {
		o := r.vm.NewObject()
		_ = o.Set("dismiss", r.function(func(js.FunctionCall) js.Value {
			r.cx.DismissKeyboard()
			return js.Undefined()
		}))
		return o
	}},
	{"keymap", (*runtime).keymap},
	{"Clipboard", (*runtime).clipboard},
	{"Storage", (*runtime).storage},
	{"Picker", (*runtime).picker},
	{"Speech", (*runtime).speech},
	fn("openUrl", func(r *runtime, call js.FunctionCall) js.Value {
		p, resolve, reject := r.vm.NewPromise()
		u := arg(call, 0).String()
		if err := r.checkAccess(core.AccessRequest{Permission: core.AccessOpenURL, Resource: u}); err != nil {
			_ = reject(r.goError(err))
			return r.vm.ToValue(p)
		}
		r.cx.OpenURL(u, func(opened bool) { r.inJS(func() { _ = resolve(opened) }) })
		return r.vm.ToValue(p)
	}),
	fn("announce", func(r *runtime, call js.FunctionCall) js.Value {
		r.cx.Announce(arg(call, 0).String())
		return js.Undefined()
	}),
	fn("setAccessibilityFocus", func(r *runtime, call js.FunctionCall) js.Value {
		if n := r.nodeOf(arg(call, 0)); n != nil && n.ref != nil {
			n.ref.SetAccessibilityFocus()
		}
		return js.Undefined()
	}),
	{"safeArea", func(r *runtime) js.Value {
		return r.accessor(func() js.Value { return r.edgesValue(r.cx.Env().SafeArea()) })
	}},
	{"keyboardFrame", func(r *runtime) js.Value {
		return r.accessor(func() js.Value {
			e := r.cx.Env()
			o := r.vm.NewObject()
			_ = o.Set("height", js32(e.KeyboardHeight()))
			_ = o.Set("duration", float64(e.KeyboardDuration().Milliseconds()))
			return o
		})
	}},
	fn("token", (*runtime).token),
	fn("resource", (*runtime).resource),
	fn("nativeComponent", (*runtime).nativeComponent),
	fn("nativeModule", (*runtime).nativeModule),
	{"host", (*runtime).host},
}

// hexColor writes c as #rrggbbaa.
func hexColor(c core.Color) string {
	cr, cg, cb, ca := c.Components()
	b := func(f float64) int { return int(f*255 + 0.5) }
	return fmt.Sprintf("#%02x%02x%02x%02x", b(cr), b(cg), b(cb), b(ca))
}

// keymap is keymap: bind(keystroke, handler, options) or bind({ keystroke:
// handler }, options), each binding going with the owner it was made
// under or by the function bind returns; label(keystroke) is the
// platform's way of writing it.
func (r *runtime) keymap() js.Value {
	o := r.vm.NewObject()
	_ = o.Set("bind", r.function(func(call js.FunctionCall) js.Value {
		bindings := map[string]js.Value{}
		var order []string
		options := arg(call, 2)
		if m, ok := arg(call, 0).(*js.Object); ok && !isFunction(m) {
			for _, k := range m.Keys() {
				bindings[k], order = m.Get(k), append(order, k)
			}
			options = arg(call, 1)
		} else {
			k := arg(call, 0).String()
			bindings[k], order = arg(call, 1), []string{k}
		}
		opt := r.options(options)
		ko := core.KeyOptions{Repeat: opt.bool("repeat", false),
			When: map[string]core.KeyWhen{"always": core.KeyAlways, "notEditing": core.KeyNotEditing}[opt.str("when", "")]}
		owner := r.cx.Child()
		owner.Run(func() {
			for _, k := range order {
				h, key := bindings[k], k
				owner.OnKeyWith(key, ko, func() {
					e := r.vm.NewObject()
					_ = e.Set("keystroke", key)
					r.call(h, e)
				})
			}
		})
		return r.function(func(js.FunctionCall) js.Value {
			owner.Dispose()
			return js.Undefined()
		})
	}))
	_ = o.Set("label", r.function(func(call js.FunctionCall) js.Value {
		return r.vm.ToValue(r.cx.KeyLabel(arg(call, 0).String()))
	}))
	return o
}

// refused is a promise rejected with the refusal of permission, nil
// when it is granted.
func (r *runtime) refused(permission string) js.Value {
	err := r.checkAccess(core.AccessRequest{Permission: permission})
	if err == nil {
		return nil
	}
	p, _, reject := r.vm.NewPromise()
	_ = reject(r.goError(err))
	return r.vm.ToValue(p)
}

// promise returns a promise and a function settling it on the engine
// goroutine with what make gives.
func (r *runtime) promise() (*js.Promise, func(v func() js.Value)) {
	p, resolve, _ := r.vm.NewPromise()
	return p, func(v func() js.Value) { r.inJS(func() { _ = resolve(v()) }) }
}

func (r *runtime) clipboard() js.Value {
	o := r.vm.NewObject()
	set := func(name string, f func(call js.FunctionCall) js.Value) {
		_ = o.Set(name, r.function(func(call js.FunctionCall) js.Value {
			if p := r.refused(core.AccessClipboard); p != nil {
				return p
			}
			return f(call)
		}))
	}
	set("writeText", func(call js.FunctionCall) js.Value {
		r.cx.SetClipboard(arg(call, 0).String())
		p, done := r.promise()
		done(js.Undefined)
		return r.vm.ToValue(p)
	})
	set("readText", func(js.FunctionCall) js.Value {
		p, done := r.promise()
		r.cx.ReadClipboard(func(text string) { done(func() js.Value { return r.vm.ToValue(text) }) })
		return r.vm.ToValue(p)
	})
	set("hasText", func(js.FunctionCall) js.Value {
		p, done := r.promise()
		r.cx.HasClipboardText(func(has bool) { done(func() js.Value { return r.vm.ToValue(has) }) })
		return r.vm.ToValue(p)
	})
	set("writeImage", func(call js.FunctionCall) js.Value {
		p, resolve, reject := r.vm.NewPromise()
		data, err := imageBytes(arg(call, 0).String())
		if err != nil {
			_ = reject(r.goError(err))
			return r.vm.ToValue(p)
		}
		r.cx.SetClipboardImage(data)
		_ = resolve(js.Undefined())
		return r.vm.ToValue(p)
	})
	set("readImage", func(js.FunctionCall) js.Value {
		p, resolve, reject := r.vm.NewPromise()
		r.cx.ReadClipboardImage(func(img core.ClipboardImage, ok bool, err error) {
			r.inJS(func() {
				if err != nil {
					_ = reject(r.goError(err))
					return
				}
				if !ok {
					_ = resolve(js.Null())
					return
				}
				o := r.vm.NewObject()
				_ = o.Set("url", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(img.PNG))
				_ = o.Set("size", r.size(img.Size))
				_ = resolve(o)
			})
		})
		return r.vm.ToValue(p)
	})
	set("hasImage", func(js.FunctionCall) js.Value {
		p, done := r.promise()
		r.cx.HasClipboardImage(func(has bool) { done(func() js.Value { return r.vm.ToValue(has) }) })
		return r.vm.ToValue(p)
	})
	set("readFiles", func(js.FunctionCall) js.Value {
		p, done := r.promise()
		r.cx.ReadClipboardFiles(func(uris []string) { done(func() js.Value { return r.vm.ToValue(uris) }) })
		return r.vm.ToValue(p)
	})
	return o
}

// imageBytes reads a data: URL's bytes.
func imageBytes(src string) ([]byte, error) {
	meta, data, ok := strings.Cut(strings.TrimPrefix(src, "data:"), ",")
	if !ok || !strings.HasPrefix(src, "data:") {
		return nil, fmt.Errorf("nsgo: Clipboard.writeImage takes a data: URL")
	}
	if strings.HasSuffix(meta, ";base64") {
		return base64.StdEncoding.DecodeString(data)
	}
	return []byte(data), nil
}

func (r *runtime) storage() js.Value {
	o := r.vm.NewObject()
	s := r.cx.Storage()
	set := func(name string, f func(call js.FunctionCall) (js.Value, error)) {
		_ = o.Set(name, r.function(func(call js.FunctionCall) js.Value {
			p, resolve, reject := r.vm.NewPromise()
			v, err := f(call)
			if err != nil {
				_ = reject(r.goError(err))
			} else {
				_ = resolve(v)
			}
			return r.vm.ToValue(p)
		}))
	}
	set("getItem", func(call js.FunctionCall) (js.Value, error) {
		v, ok := s.GetItem(arg(call, 0).String())
		if !ok {
			return js.Null(), nil
		}
		return r.vm.ToValue(v), nil
	})
	set("setItem", func(call js.FunctionCall) (js.Value, error) {
		return js.Undefined(), s.SetItem(arg(call, 0).String(), arg(call, 1).String())
	})
	set("removeItem", func(call js.FunctionCall) (js.Value, error) {
		s.RemoveItem(arg(call, 0).String())
		return js.Undefined(), nil
	})
	set("keys", func(js.FunctionCall) (js.Value, error) { return r.vm.ToValue(s.Keys()), nil })
	set("clear", func(js.FunctionCall) (js.Value, error) {
		s.Clear()
		return js.Undefined(), nil
	})
	return o
}

func (r *runtime) picker() js.Value {
	o := r.vm.NewObject()
	_ = o.Set("pickImages", r.function(func(call js.FunctionCall) js.Value {
		if p := r.refused(core.AccessPicker); p != nil {
			return p
		}
		op := r.options(arg(call, 0))
		opts := core.PickOptions{Multiple: op.bool("multiple", false), Limit: int(op.float("limit", 0)),
			MaxDimension: int(op.float("maxDimension", 0)), Quality: float32(op.float("quality", 0))}
		if op.str("format", "") == "current" {
			opts.Format = core.PickCurrent
		}
		p, done := r.promise()
		r.cx.PickImages(opts, func(files []core.PickedFile) {
			done(func() js.Value {
				out := make([]any, len(files))
				for i, f := range files {
					out[i] = r.pickedImage(f)
				}
				return r.vm.NewArray(out...)
			})
		})
		return r.vm.ToValue(p)
	}))
	_ = o.Set("canTakePhoto", r.function(func(js.FunctionCall) js.Value {
		p, resolve, _ := r.vm.NewPromise()
		_ = resolve(core.Untracked(r.cx, r.cx.Env().CanTakePhoto))
		return r.vm.ToValue(p)
	}))
	_ = o.Set("takePhoto", r.function(func(call js.FunctionCall) js.Value {
		if p := r.refused(core.AccessPicker); p != nil {
			return p
		}
		op := r.options(arg(call, 0))
		opts := core.PhotoOptions{MaxDimension: int(op.float("maxDimension", 0)), Quality: float32(op.float("quality", 0))}
		p, resolve, reject := r.vm.NewPromise()
		r.cx.TakePhoto(opts, func(photo *core.PickedFile, err error) {
			r.inJS(func() {
				switch {
				case err != nil:
					_ = reject(r.goError(err))
				case photo == nil:
					_ = resolve(js.Null())
				default:
					_ = resolve(r.pickedImage(*photo))
				}
			})
		})
		return r.vm.ToValue(p)
	}))
	return o
}

// pickedImage is f as a PickedImage: a File and its size.
func (r *runtime) pickedImage(f core.PickedFile) *js.Object {
	o := r.vm.NewObject()
	_ = o.Set("file", r.newFile(f.Data, f.Name, f.MIME))
	_ = o.Set("size", r.size(f.Size))
	return o
}

// imageModule adds Image's functions to the Image component.
func (r *runtime) imageModule() js.Value {
	img := r.hostComponent("image").(*js.Object)
	_ = img.Set("getSize", r.function(func(call js.FunctionCall) js.Value {
		p, resolve, reject := r.vm.NewPromise()
		r.cx.ImageSize(r.imageSource(arg(call, 0)), func(px core.Size, err error) {
			r.inJS(func() {
				if err != nil {
					_ = reject(r.goError(err))
					return
				}
				_ = resolve(r.size(px))
			})
		})
		return r.vm.ToValue(p)
	}))
	_ = img.Set("prefetch", r.function(func(call js.FunctionCall) js.Value {
		p, done := r.promise()
		r.cx.PrefetchImage(r.imageSource(arg(call, 0)), func(ok bool) { done(func() js.Value { return r.vm.ToValue(ok) }) })
		return r.vm.ToValue(p)
	}))
	return img
}

// resource is resource([source,] fetcher, options): an accessor of what
// fetcher resolves with, loaded again when source changes, with
// loading, error, state, refetch and mutate (NSH's resource.rs).
func (r *runtime) resource(call js.FunctionCall) js.Value {
	source, fetcher, options := js.Value(nil), arg(call, 0), arg(call, 1)
	if isFunction(arg(call, 1)) {
		source, fetcher, options = arg(call, 0), arg(call, 1), arg(call, 2)
	}
	opt := r.options(options)
	initial := opt.get("initialValue")
	if initial == nil {
		initial = js.Undefined()
	}
	value := core.NewSignalFunc(r.cx, initial, strictEquals)
	state := core.NewSignal(r.cx, "unresolved")
	if defined(initial) {
		state.Set("ready")
	}
	errv := core.NewSignalFunc[js.Value](r.cx, js.Undefined(), strictEquals)
	var run int
	var abort js.Value
	load := func(src js.Value, refetching bool) {
		run++
		mine := run
		if abort != nil {
			r.call2(abort.ToObject(r.vm).Get("abort"), abort)
		}
		ctl, _ := r.vm.New(r.vm.Get("AbortController"))
		abort = ctl
		info := r.vm.NewObject()
		_ = info.Set("signal", ctl.Get("signal"))
		_ = info.Set("refetching", refetching)
		// A load over a value refreshes it; without one it is pending.
		if defined(value.GetUntracked()) {
			state.Set("refreshing")
		} else {
			state.Set("pending")
		}
		settle := func(v js.Value, failed bool) {
			if mine != run {
				return
			}
			// A load that ended is not aborted by the next.
			if abort == ctl {
				abort = nil
			}
			r.cx.Batch(func() {
				if failed {
					errv.Set(v)
					state.Set("errored")
					return
				}
				errv.Set(js.Undefined())
				value.Set(v)
				state.Set("ready")
			})
		}
		var res js.Value
		var thrown js.Value
		func() {
			defer func() {
				if x := recover(); x != nil {
					if ex, ok := x.(*js.Exception); ok {
						thrown = ex.Value()
						return
					}
					panic(x)
				}
			}()
			if src == nil {
				res = r.call(fetcher, info)
			} else {
				res = r.call(fetcher, src, info)
			}
		}()
		if thrown != nil {
			settle(thrown, true)
			return
		}
		if p, ok := res.Export().(*js.Promise); ok {
			then := r.vm.ToValue(p).ToObject(r.vm).Get("then")
			r.call2(then, r.vm.ToValue(p),
				r.function(func(c js.FunctionCall) js.Value { settle(arg(c, 0), false); return js.Undefined() }),
				r.function(func(c js.FunctionCall) js.Value { settle(arg(c, 0), true); return js.Undefined() }))
			return
		}
		settle(res, false)
	}
	var current js.Value
	if source == nil {
		core.Untracked(r.cx, func() bool { load(nil, false); return true })
	} else {
		r.cx.RenderEffect(func() {
			src := source
			if isFunction(source) {
				src = r.call(source)
			}
			current = src
			if !defined(src) || (isBool(src) && !src.ToBoolean()) {
				return
			}
			core.Untracked(r.cx, func() bool { load(src, false); return true })
		})
	}
	// It reads the latest value, failed or not, as NSH's does: error()
	// says why a load failed.
	read := r.accessor(func() js.Value { return value.Get() })
	_ = read.Set("loading", r.function(func(js.FunctionCall) js.Value {
		s := state.Get()
		return r.vm.ToValue(s == "pending" || s == "refreshing")
	}))
	_ = read.Set("error", r.function(func(js.FunctionCall) js.Value { return errv.Get() }))
	_ = read.Set("state", r.function(func(js.FunctionCall) js.Value { return r.vm.ToValue(state.Get()) }))
	_ = read.Set("refetch", r.function(func(js.FunctionCall) js.Value {
		if source == nil {
			load(nil, true)
		} else if defined(current) {
			load(current, true)
		}
		return js.Undefined()
	}))
	// The owner's end aborts the load still running.
	r.cx.Cleanup(func() {
		run++
		if abort != nil {
			ctl := abort
			abort = nil
			r.call2(ctl.ToObject(r.vm).Get("abort"), ctl)
		}
	})
	_ = read.Set("mutate", r.function(func(c js.FunctionCall) js.Value {
		v := arg(c, 0)
		run++
		r.cx.Batch(func() {
			value.Set(v)
			errv.Set(js.Undefined())
			state.Set("ready")
		})
		return v
	}))
	return read
}

// call2 calls fn with this.
func (r *runtime) call2(fn, this js.Value, args ...js.Value) js.Value {
	f, ok := js.AssertFunction(fn)
	if !ok {
		panic(r.vm.NewTypeError("nsgo: %s is not a function", fn))
	}
	v, err := f(this, args...)
	if err != nil {
		panic(err)
	}
	return v
}

// nativeModule is nativeModule(specifier): the exports of the module
// specifier names, NSGo's own or one the app's Go code registered
// (Register), the object an import gives, which the app types itself.
// Without one it throws a ReferenceError naming it.
func (r *runtime) nativeModule(call js.FunctionCall) js.Value {
	name := arg(call, 0).String()
	_, builtIn := moduleTable[name]
	if _, ok := registeredExports(name); !ok && !builtIn {
		ref, _ := r.vm.RunString("ReferenceError")
		ctor, _ := js.AssertConstructor(ref)
		e, _ := ctor(nil, r.vm.ToValue(fmt.Sprintf("nativeModule: no module %q is registered", name)))
		panic(e)
	}
	return r.require(name)
}

// nativeComponent is nativeComponent(name, { props, events }): a
// component showing the native component the embedding app registered
// under name, its props sent as they change and its events given to
// on<Event> handlers.
func (r *runtime) nativeComponent(call js.FunctionCall) js.Value {
	name := arg(call, 0).String()
	if r.sandbox != nil && !r.sandbox.importsNative(name) {
		panic(r.vm.NewTypeError("nsgo: native component %q is not given to this sandbox (Sandbox.Modules)", name))
	}
	spec := r.options(arg(call, 1))
	var names, events []string
	for _, p := range r.arrayItems(spec.get("props")) {
		names = append(names, p.String())
	}
	for _, e := range r.arrayItems(spec.get("events")) {
		events = append(events, e.String())
	}
	f := func(p *props) core.Node {
		n := core.Native{
			Component: name,
			PropsFunc: func() map[string]any {
				m := map[string]any{}
				for _, k := range names {
					if v := p.get(k); defined(v) {
						if a := r.animatedNodeOf(v); a != nil {
							m[k] = a
						} else {
							m[k] = v.Export()
						}
					}
				}
				return m
			},
			StyleFunc: p.style,
			OnPress:   p.onPress("onPress"),
			OnLayout:  p.onLayout(),
			Ref:       p.n.ref,
			Label:     p.str("accessibilityLabel"),
			TestID:    p.str("testID"),
		}
		n.OnEvent = func(event string, data any) {
			for _, e := range events {
				if e == event {
					if h := p.handler("on" + strings.ToUpper(e[:1]) + e[1:]); h != nil {
						h(r.vm.ToValue(data))
					}
				}
			}
		}
		return n
	}
	return r.faceComponent(name, f)
}

// registerFont is `registerFont(base64)`, which a compiled font import
// calls as its module runs: the font registered with the platform before
// the app mounts, so its first frame shows it (Scope.RegisterFont).
func (r *runtime) registerFont(call js.FunctionCall) js.Value {
	data, err := base64.StdEncoding.DecodeString(arg(call, 0).String())
	if err == nil {
		err = r.cx.RegisterFont(data)
	}
	if err != nil {
		r.warn("a font did not register: %v", err)
	}
	return js.Undefined()
}

// registerAsset is `registerAsset(url, dataURL)`, which a compiled image
// import calls as its module runs: what its asset: URL reads, for the
// images and fetches that name it.
func (r *runtime) registerAsset(call js.FunctionCall) js.Value {
	if r.assets == nil {
		r.assets = map[string]string{}
	}
	r.assets[arg(call, 0).String()] = arg(call, 1).String()
	return js.Undefined()
}

// assetData is what the platform loads for an image's url: the bytes a
// bundle ships for an asset: URL.
func (r *runtime) assetData(url string) string {
	if data, ok := r.assets[url]; ok {
		return data
	}
	return url
}

// setup is `setup({ theme, tailwind })`, which compiled bundles call
// first: the theme file and the Tailwind theme nsgo.yaml gives, the
// app's.
func (r *runtime) setup(call js.FunctionCall) js.Value {
	o := r.options(arg(call, 0))
	if sb := o.get("statusBar"); defined(sb) {
		r.cx.SetStatusBar(r.statusBar(sb))
	}
	theme, tw := o.str("theme", ""), o.str("tailwind", "")
	if theme == "" && tw == "" {
		return js.Undefined()
	}
	t := r.cx.Theme().Clone()
	if theme != "" {
		var err error
		if t, err = core.ParseTheme([]byte(theme)); err != nil {
			panic(r.goError(err))
		}
	}
	if tw != "" {
		var err error
		if t.Tailwind, err = core.ParseTailwind([]byte(tw)); err != nil {
			panic(r.goError(err))
		}
	}
	r.cx.SetTheme(t)
	return js.Undefined()
}

// token is `token(name)`: the design token name of the theme in the
// appearance shown, followed as it changes: a color as #rrggbbaa, a
// length as points, else its text; undefined for none.
func (r *runtime) token(call js.FunctionCall) js.Value {
	name := strings.TrimPrefix(arg(call, 0).String(), "--")
	p := r.cx.Palette()
	if c, ok := p.Color(name); ok {
		return r.vm.ToValue(hexColor(c))
	}
	v, ok := r.cx.Theme().Tokens[name]
	if !ok {
		return js.Undefined()
	}
	if l, ok := cssLength(v); ok {
		return r.vm.ToValue(js32(l))
	}
	return r.vm.ToValue(v)
}
