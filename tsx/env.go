package tsx

import (
	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/core"
)

// The environment (NSH's env.rs): each value an accessor over core's
// environment, which runs again what reads it when it changes.

func (r *runtime) envAccessor(get func(e *core.Env) any) js.Value {
	return r.accessor(func() js.Value { return r.vm.ToValue(get(r.cx.Env())) })
}

var envExports = []export{
	{"windowSize", func(r *runtime) js.Value {
		return r.accessor(func() js.Value { return r.size(r.cx.Env().Window()) })
	}},
	{"colorScheme", func(r *runtime) js.Value {
		return r.envAccessor(func(e *core.Env) any {
			if e.Dark() {
				return "dark"
			}
			return "light"
		})
	}},
	fn("setColorScheme", func(r *runtime, call js.FunctionCall) js.Value {
		a := map[string]core.Appearance{"light": core.AppearanceLight, "dark": core.AppearanceDark}[stringOf(arg(call, 0))]
		r.cx.SetAppearance(a)
		return js.Undefined()
	}),
	{"fontScale", func(r *runtime) js.Value {
		return r.envAccessor(func(e *core.Env) any { return js32(e.FontScale()) })
	}},
	{"pixelRatio", func(r *runtime) js.Value {
		return r.envAccessor(func(e *core.Env) any { return js32(e.PixelRatio()) })
	}},
	{"locale", func(r *runtime) js.Value { return r.envAccessor(func(e *core.Env) any { return e.Locale() }) }},
	{"reduceMotion", func(r *runtime) js.Value {
		return r.envAccessor(func(e *core.Env) any { return e.ReduceMotion() })
	}},
	{"lifecycle", func(r *runtime) js.Value {
		return r.envAccessor(func(e *core.Env) any { return e.Lifecycle().String() })
	}},
	{"screenReaderEnabled", func(r *runtime) js.Value {
		return r.envAccessor(func(e *core.Env) any { return e.ScreenReader() })
	}},
	// The insets of the nearest page or dialog, or the safe area.
	fn("safeAreaInsets", func(r *runtime, _ js.FunctionCall) js.Value {
		insets := core.InsetsFunc(r.cx)
		return r.accessor(func() js.Value { return r.edgesValue(insets()) })
	}),
	{"platform", (*runtime).platform},
	fn("appInfo", func(r *runtime, _ js.FunctionCall) js.Value { return r.appInfo() }),
}

// appInfo is the app's identity (core.App): { id, name, version, build,
// development }, null for what nsgo.yaml leaves out.
func (r *runtime) appInfo() js.Value {
	a := core.App()
	o := r.vm.NewObject()
	for _, f := range []struct{ name, value string }{{"id", a.ID}, {"name", a.Name}, {"version", a.Version}, {"build", a.Build}} {
		v := js.Null()
		if f.value != "" {
			v = r.vm.ToValue(f.value)
		}
		_ = o.Set(f.name, v)
	}
	_ = o.Set("development", a.Development)
	return o
}

// platform is the platform the app runs on: { os, version, idiom,
// catalyst, select }.
func (r *runtime) platform() js.Value {
	e := r.cx.Env()
	var os core.OS
	var version, idiom string
	core.Untracked(r.cx, func() bool {
		os, version = e.OS(), e.OSVersion().String()
		idiom = map[core.Idiom]string{core.IdiomPhone: "phone", core.IdiomTablet: "pad", core.IdiomDesktop: "desktop",
			core.IdiomMac: "mac", core.IdiomTV: "tv", core.IdiomWatch: "watch"}[e.Idiom()]
		return true
	})
	o := r.vm.NewObject()
	_ = o.Set("os", string(os))
	_ = o.Set("version", version)
	_ = o.Set("idiom", idiom)
	_ = o.Set("catalyst", false)
	// select({ ios, android, native, desktop, default }): the value for
	// this platform.
	_ = o.Set("select", r.function(func(call js.FunctionCall) js.Value {
		spec := arg(call, 0).ToObject(r.vm)
		keys := []string{string(os)}
		if os == core.OSIOS || os == core.OSAndroid {
			keys = append(keys, "native")
		}
		if idiom == "desktop" {
			keys = append(keys, "desktop")
		}
		for _, k := range append(keys, "default") {
			if v := spec.Get(k); v != nil && !js.IsUndefined(v) {
				return v
			}
		}
		return js.Undefined()
	}))
	return o
}

func (r *runtime) edgesValue(e core.Edges) *js.Object {
	o := r.vm.NewObject()
	_ = o.Set("top", js32(e.Top))
	_ = o.Set("right", js32(e.Right))
	_ = o.Set("bottom", js32(e.Bottom))
	_ = o.Set("left", js32(e.Left))
	return o
}
