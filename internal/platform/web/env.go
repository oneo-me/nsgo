//go:build js && wasm

package web

import (
	"math"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// watch is what the page's environment is read from. The page cannot
// tell whether a screen reader runs, so ScreenReader stays false.
type watch struct {
	dark, reduce, desktop js.Value // media query lists
	resolution            js.Value // matches ratio only
	ratio                 float64
	// probe is padded by the safe area and as wide as the root font size.
	probe   js.Value
	changed js.Func
	// settings are what Intl says of settingsFor's hours and weeks.
	settingsFor string
	settings    driver.LocaleSettings
}

// rtlLanguages are the languages written right to left.
var rtlLanguages = map[string]bool{
	"ar": true, "arc": true, "ckb": true, "dv": true, "fa": true, "he": true, "iw": true,
	"ks": true, "ps": true, "sd": true, "ug": true, "ur": true, "yi": true,
}

// watchStart prepares what the environment is read from. A page the app
// fills extends under notches and system bars, which only then report
// their safe area.
func (s *surface) watchStart(page bool) {
	g := js.Global()
	if meta := s.doc.Call("querySelector", `meta[name="viewport"]`); page && meta.Truthy() {
		content := meta.Call("getAttribute", "content")
		if content.Type() == js.TypeString && !strings.Contains(content.String(), "viewport-fit") {
			meta.Call("setAttribute", "content", content.String()+", viewport-fit=cover")
		}
	}
	w := &s.watch
	w.dark = g.Call("matchMedia", "(prefers-color-scheme: dark)")
	w.reduce = g.Call("matchMedia", "(prefers-reduced-motion: reduce)")
	w.desktop = g.Call("matchMedia", "(hover: hover) and (pointer: fine) and (min-width: 600px)")
	w.probe = s.doc.Call("createElement", "div")
	setStyle(w.probe, map[string]string{
		"position": "fixed", "left": "0", "top": "0", "visibility": "hidden", "pointer-events": "none",
		"box-sizing": "content-box", "width": "1rem", "height": "0",
		"padding": "env(safe-area-inset-top, 0px) env(safe-area-inset-right, 0px) " +
			"env(safe-area-inset-bottom, 0px) env(safe-area-inset-left, 0px)",
	})
	s.host.Call("appendChild", w.probe)
}

// watchChanges sends the environment again whenever a part of it may
// have changed.
func (s *surface) watchChanges() {
	g, w := js.Global(), &s.watch
	w.changed = js.FuncOf(func(js.Value, []js.Value) any {
		s.updateEnv()
		return nil
	})
	for _, q := range []js.Value{w.dark, w.reduce, w.desktop} {
		q.Call("addEventListener", "change", w.changed)
	}
	s.watchRatio(g.Get("devicePixelRatio").Float())
	g.Call("addEventListener", "resize", w.changed)
	g.Call("addEventListener", "languagechange", w.changed)
	// The page's visibility and focus are the app's lifecycle.
	g.Call("addEventListener", "focus", w.changed)
	g.Call("addEventListener", "blur", w.changed)
	s.doc.Call("addEventListener", "visibilitychange", w.changed)
	// A page that hides is the app quitting, unless the browser keeps it
	// in its back-forward cache to show again, where it may also discard
	// it unheard: the OnQuit handlers run then too, kept for the next.
	g.Call("addEventListener", "pagehide", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if args[0].Get("persisted").Bool() {
			s.engine.Hide()
		} else {
			s.engine.Close()
		}
		return nil
	}))
	if vv := g.Get("visualViewport"); vv.Truthy() {
		vv.Call("addEventListener", "resize", w.changed)
		vv.Call("addEventListener", "scroll", w.changed)
	}
	// The probe's box follows the safe area and the root font size.
	ro := g.Get("ResizeObserver").New(w.changed)
	ro.Call("observe", w.probe, map[string]any{"box": "border-box"})
}

// watchRatio follows the pixel ratio from ratio on, as a resolution
// query matches one ratio only.
func (s *surface) watchRatio(ratio float64) {
	g, w := js.Global(), &s.watch
	if w.resolution.Truthy() {
		w.resolution.Call("removeEventListener", "change", w.changed)
	}
	w.ratio = ratio
	q := "(resolution: " + strconv.FormatFloat(ratio, 'g', -1, 64) + "dppx)"
	w.resolution = g.Call("matchMedia", q)
	w.resolution.Call("addEventListener", "change", w.changed)
}

func (s *surface) updateEnv() {
	if ratio := js.Global().Get("devicePixelRatio").Float(); ratio != s.watch.ratio {
		s.watchRatio(ratio)
	}
	if env := s.environment(); !env.Equal(s.env) {
		repaint := env.PixelRatio != s.env.PixelRatio
		s.env = env
		s.engine.SetEnvironment(env)
		for _, v := range s.views {
			if repaint && v != nil && v.kind == driver.KindCanvas {
				s.paint(v)
			}
			if repaint && v != nil && v.gl != nil {
				s.shadeLater(v)
			}
		}
	}
}

// environment reads the page's environment.
func (s *surface) environment() driver.Environment {
	g, w := js.Global(), &s.watch
	css := g.Call("getComputedStyle", w.probe)
	length := func(name string) driver.Points {
		f := g.Call("parseFloat", css.Get(name)).Float()
		if math.IsNaN(f) {
			return 0
		}
		return driver.Points(f)
	}
	env := driver.Environment{
		ScreenColor: js.Global().Get("EyeDropper").Truthy(),
		// Browsers on touch screens open the camera for an input's capture.
		Camera: g.Call("matchMedia", "(pointer: coarse)").Get("matches").Bool(),
		SafeArea: driver.Edges{
			Top: length("paddingTop"), Right: length("paddingRight"),
			Bottom: length("paddingBottom"), Left: length("paddingLeft"),
		},
		Dark:          w.dark.Get("matches").Bool(),
		ReduceMotion:  w.reduce.Get("matches").Bool(),
		FontScale:     float32(length("width") / 16),
		PixelRatio:    float32(g.Get("devicePixelRatio").Float()),
		OS:            driver.OSWeb,
		HasWebView:    true,
		AppleKeyboard: apple,
		Idiom:         driver.IdiomPhone,
	}
	if w.desktop.Get("matches").Bool() {
		env.Idiom = driver.IdiomDesktop
	}
	switch {
	case s.doc.Get("visibilityState").String() == "hidden":
		env.Lifecycle = driver.LifecycleBackground
	case !s.doc.Call("hasFocus").Bool():
		env.Lifecycle = driver.LifecycleInactive
	}
	if lang := g.Get("navigator").Get("language"); lang.Type() == js.TypeString {
		env.Locale = lang.String()
		primary, _, _ := strings.Cut(strings.ToLower(env.Locale), "-")
		env.RTL = rtlLanguages[primary]
		env.LocaleSettings = s.localeSettings(env.Locale)
	}
	// The keyboard shrinks the visual viewport from the bottom; so does
	// a pinch zoom, which is told apart by its scale.
	if vv := g.Get("visualViewport"); vv.Truthy() && math.Abs(vv.Get("scale").Float()-1) < 0.01 {
		h := g.Get("innerHeight").Float() - vv.Get("height").Float() - vv.Get("offsetTop").Float()
		env.KeyboardHeight = driver.Points(math.Max(0, math.Round(h)))
	}
	return env
}

// localeSettings is what the browser's Intl says of locale's hours and
// weeks, read once per locale: its hour cycle, which some browsers take
// from the system's 24-hour time, and its first day of the week.
func (s *surface) localeSettings(locale string) driver.LocaleSettings {
	if s.watch.settingsFor == locale {
		return s.watch.settings
	}
	var out driver.LocaleSettings
	intl := js.Global().Get("Intl")
	if intl.Truthy() {
		opts := intl.Get("DateTimeFormat").New(locale, map[string]any{"hour": "numeric"}).Call("resolvedOptions")
		if hc := opts.Get("hourCycle"); hc.Type() == js.TypeString {
			out.HourCycle = hc.String()
		}
		if loc := intl.Get("Locale"); loc.Truthy() {
			l := loc.New(locale)
			info := l.Get("weekInfo")
			if get := l.Get("getWeekInfo"); get.Type() == js.TypeFunction {
				info = l.Call("getWeekInfo")
			}
			if info.Type() == js.TypeObject {
				if d := info.Get("firstDay"); d.Type() == js.TypeNumber {
					out.FirstDayOfWeek = d.Int()
				}
			}
		}
	}
	s.watch.settingsFor, s.watch.settings = locale, out
	return out
}
