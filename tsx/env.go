package tsx

import (
	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
)

// The environment (NSH's env.rs): each value an accessor over core's
// environment, which runs again what reads it when it changes.

func (r *runtime) envAccessor(get func(e *core.Env) any) goja.Value {
	return r.accessor(func() goja.Value { return r.vm.ToValue(get(r.cx.Env())) })
}

var envExports = []export{
	{"windowSize", func(r *runtime) goja.Value {
		return r.accessor(func() goja.Value { return r.size(r.cx.Env().Window()) })
	}},
	{"colorScheme", func(r *runtime) goja.Value {
		return r.envAccessor(func(e *core.Env) any {
			if e.Dark() {
				return "dark"
			}
			return "light"
		})
	}},
	fn("setColorScheme", func(r *runtime, call goja.FunctionCall) goja.Value {
		a := map[string]core.Appearance{"light": core.AppearanceLight, "dark": core.AppearanceDark}[stringOf(arg(call, 0))]
		r.cx.SetAppearance(a)
		return goja.Undefined()
	}),
	{"fontScale", func(r *runtime) goja.Value {
		return r.envAccessor(func(e *core.Env) any { return js32(e.FontScale()) })
	}},
	{"pixelRatio", func(r *runtime) goja.Value {
		return r.envAccessor(func(e *core.Env) any { return js32(e.PixelRatio()) })
	}},
	{"locale", func(r *runtime) goja.Value { return r.envAccessor(func(e *core.Env) any { return e.Locale() }) }},
	{"reduceMotion", func(r *runtime) goja.Value {
		return r.envAccessor(func(e *core.Env) any { return e.ReduceMotion() })
	}},
	{"screenReaderEnabled", func(r *runtime) goja.Value {
		return r.envAccessor(func(e *core.Env) any { return e.ScreenReader() })
	}},
	// The insets of the nearest page or dialog, or the safe area.
	fn("safeAreaInsets", func(r *runtime, _ goja.FunctionCall) goja.Value {
		insets := core.InsetsFunc(r.cx)
		return r.accessor(func() goja.Value { return r.edgesValue(insets()) })
	}),
	{"platform", (*runtime).platform},
}

// platform is the platform the app runs on: { os, version, idiom,
// catalyst, select }.
func (r *runtime) platform() goja.Value {
	e := r.cx.Env()
	var os core.OS
	var version, idiom string
	core.Untracked(r.cx, func() bool {
		os, version = e.OS(), e.OSVersion().String()
		idiom = map[core.Idiom]string{core.IdiomPhone: "phone", core.IdiomTablet: "pad", core.IdiomDesktop: "desktop",
			core.IdiomMac: "mac", core.IdiomTV: "tv"}[e.Idiom()]
		return true
	})
	o := r.vm.NewObject()
	_ = o.Set("os", string(os))
	_ = o.Set("version", version)
	_ = o.Set("idiom", idiom)
	_ = o.Set("catalyst", false)
	// select({ ios, android, native, desktop, default }): the value for
	// this platform.
	_ = o.Set("select", r.function(func(call goja.FunctionCall) goja.Value {
		spec := arg(call, 0).ToObject(r.vm)
		keys := []string{string(os)}
		if os == core.OSIOS || os == core.OSAndroid {
			keys = append(keys, "native")
		}
		if idiom == "desktop" {
			keys = append(keys, "desktop")
		}
		for _, k := range append(keys, "default") {
			if v := spec.Get(k); v != nil && !goja.IsUndefined(v) {
				return v
			}
		}
		return goja.Undefined()
	}))
	return o
}

func (r *runtime) edgesValue(e core.Edges) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("top", js32(e.Top))
	_ = o.Set("right", js32(e.Right))
	_ = o.Set("bottom", js32(e.Bottom))
	_ = o.Set("left", js32(e.Left))
	return o
}
