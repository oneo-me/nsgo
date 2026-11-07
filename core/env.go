package core

import (
	"strings"
	"sync/atomic"
	"time"
)

// Env is the environment as signals, one per part, so a binding runs
// again only when a part it read changes.
type Env struct {
	window               *Signal[Size]
	safe, titleBar       *Signal[Edges]
	maximized            *Signal[bool]
	fullScreen           *Signal[bool]
	windowFrame          *Signal[WindowFrame]
	keyboard             *Signal[Points]
	keyboardDuration     *Signal[time.Duration]
	dark, reduce, reader *Signal[bool]
	screenColor          *Signal[bool]
	fontScale, ratio     *Signal[float32]
	// locale and systemRTL are the platform's or the host's; forced is
	// the locale Scope.SetLocale forces.
	locale, forced *Signal[string]
	systemRTL      *Signal[bool]
	language       *Memo[string]
	rtl            *Memo[bool]
	os             *Signal[OS]
	version        *Signal[OSVersion]
	idiom          *Signal[Idiom]
	appearance     *Signal[Appearance]
	// systemDark is the system's appearance, which a forced one hides.
	systemDark bool
	material   *Signal[WindowMaterial]
	appleKeys  *Signal[bool]
	tokens     *Signal[hostColors]
	// headers are the system header's heights, which only the tree reads.
	headers [2]Points
}

// newEnv makes an environment owned by cx. A window's shares what the
// app sets for all of them with the first's, app.
func newEnv(cx *Scope, app *Env) *Env {
	e := &Env{
		window:           NewSignal(cx, Size{}),
		safe:             NewSignal(cx, Edges{}),
		titleBar:         NewSignal(cx, Edges{}),
		maximized:        NewSignal(cx, false),
		fullScreen:       NewSignal(cx, false),
		windowFrame:      NewSignal(cx, WindowFrame{}),
		keyboard:         NewSignal(cx, Points(0)),
		keyboardDuration: NewSignal(cx, 250*time.Millisecond),
		dark:             NewSignal(cx, false),
		reduce:           NewSignal(cx, false),
		systemRTL:        NewSignal(cx, false),
		reader:           NewSignal(cx, false),
		screenColor:      NewSignal(cx, false),
		fontScale:        NewSignal(cx, float32(1)),
		ratio:            NewSignal(cx, float32(1)),
		locale:           NewSignal(cx, ""),
		forced:           NewSignal(cx, ""),
		os:               NewSignal(cx, OS("")),
		version:          NewSignal(cx, OSVersion{}),
		idiom:            NewSignal(cx, IdiomPhone),
		appearance:       NewSignal(cx, AppearanceSystem),
		appleKeys:        NewSignal(cx, false),
		material:         NewSignal(cx, WindowMaterialNone),
		tokens:           NewSignalFunc(cx, hostColors{}, nil),
	}
	if app != nil {
		e.forced, e.appearance, e.tokens = app.forced, app.appearance, app.tokens
	}
	e.language = NewMemo(cx, func() string {
		locale := e.Locale()
		if match := languageMatch.Load(); match != nil {
			return (*match)(locale)
		}
		return ""
	})
	// The app lays out in the direction of the language it shows, else of
	// the locale it forces, else as the system does.
	e.rtl = NewMemo(cx, func() bool {
		if lang := e.language.Get(); lang != "" {
			return rightToLeft(lang)
		}
		if forced := e.forced.Get(); forced != "" {
			return rightToLeft(forced)
		}
		return e.systemRTL.Get()
	})
	return e
}

// Env returns the environment of the window cx is in: its size, scale
// and safe area are the window's, and what the app forces is every
// window's.
func (cx *Scope) Env() *Env {
	rt := cx.rt
	if rt.env == nil {
		root := cx
		for root.parent != nil {
			root = root.parent
		}
		rt.env = newEnv(root, nil)
	}
	if t := cx.surf; t != nil && t != rt.tree {
		if t.env == nil {
			t.env = newEnv(t.root, rt.env)
		}
		return t.env
	}
	return rt.env
}

func (e *Env) set(v Environment) {
	if v.FontScale <= 0 {
		v.FontScale = 1
	}
	if v.PixelRatio <= 0 {
		v.PixelRatio = 1
	}
	if v.KeyboardDuration <= 0 {
		v.KeyboardDuration = 250 * time.Millisecond
	}
	e.safe.Set(v.SafeArea)
	e.titleBar.Set(v.TitleBar)
	e.maximized.Set(v.Maximized)
	e.fullScreen.Set(v.FullScreen)
	e.windowFrame.Set(v.WindowFrame)
	e.keyboard.Set(v.KeyboardHeight)
	e.keyboardDuration.Set(v.KeyboardDuration)
	e.systemDark = v.Dark
	e.dark.Set(e.shownDark())
	e.reduce.Set(v.ReduceMotion)
	e.systemRTL.Set(v.RTL)
	e.reader.Set(v.ScreenReader)
	e.screenColor.Set(v.ScreenColor)
	e.fontScale.Set(v.FontScale)
	e.ratio.Set(v.PixelRatio)
	e.locale.Set(v.Locale)
	e.os.Set(v.OS)
	e.version.Set(v.OSVersion)
	e.appleKeys.Set(v.OS == OSMacOS || v.OS == OSIOS || v.AppleKeyboard)
	e.idiom.Set(v.Idiom)
	e.material.Set(v.WindowMaterial)
	e.headers = [2]Points{v.HeaderHeight, max(v.LargeHeaderHeight, v.HeaderHeight)}
}

// Window returns the surface's size.
func (e *Env) Window() Size { return e.window.Get() }

// SafeArea returns the insets system UI covers on each edge.
func (e *Env) SafeArea() Edges { return e.safe.Get() }

// KeyboardHeight returns how much of the bottom the keyboard covers.
func (e *Env) KeyboardHeight() Points { return e.keyboard.Get() }

// TitleBar returns what a hidden desktop title bar covers: the core's
// part of desktop.TitleBar, which apps use.
func (e *Env) TitleBar() Edges { return e.titleBar.Get() }

// Maximized reports whether the window is maximized or zoomed: the
// core's part of desktop.Maximized, which apps use.
func (e *Env) Maximized() bool { return e.maximized.Get() }

// FullScreen reports whether the window is in full screen: the core's
// part of desktop.FullScreen, which apps use.
func (e *Env) FullScreen() bool { return e.fullScreen.Get() }

// WindowFrame returns where the window is and how large: the core's part
// of desktop.Frame, which apps use.
func (e *Env) WindowFrame() WindowFrame { return e.windowFrame.Get() }

// KeyboardDuration returns how long the keyboard takes to move.
func (e *Env) KeyboardDuration() time.Duration { return e.keyboardDuration.Get() }

// shownDark is whether the app shows dark: as SetAppearance forces it,
// else as the system's appearance is.
func (e *Env) shownDark() bool {
	switch e.appearance.GetUntracked() {
	case AppearanceLight:
		return false
	case AppearanceDark:
		return true
	}
	return e.systemDark
}

// Dark reports whether the app shows dark: the appearance Scope.SetAppearance
// forces, else the system's.
func (e *Env) Dark() bool { return e.dark.Get() }

// Appearance returns the appearance Scope.SetAppearance forces, or
// AppearanceSystem.
func (e *Env) Appearance() Appearance { return e.appearance.Get() }

// FontScale returns the user's text size multiplier.
func (e *Env) FontScale() float32 { return e.fontScale.Get() }

// ReduceMotion reports whether the user asked for less motion.
func (e *Env) ReduceMotion() bool { return e.reduce.Get() }

// ScreenReader reports whether a screen reader is running.
func (e *Env) ScreenReader() bool { return e.reader.Get() }

// CanPickScreenColor reports whether Scope.PickScreenColor works here:
// macOS and browsers with an EyeDropper.
func (e *Env) CanPickScreenColor() bool { return e.screenColor.Get() }

// RTL reports whether the app lays out right to left: as the language it
// shows is written (Language), else the locale it forces, else as the
// system lays out.
func (e *Env) RTL() bool { return e.rtl.Get() }

// PixelRatio returns device pixels per point.
func (e *Env) PixelRatio() float32 { return e.ratio.Get() }

// Locale returns the locale the app runs in, a BCP 47 tag: the one
// Scope.SetLocale forces, else the embedding host's, else the user's.
func (e *Env) Locale() string {
	if forced := e.forced.Get(); forced != "" {
		return forced
	}
	return e.locale.Get()
}

// Language returns the language the app shows: of its catalogs'
// languages, the one that suits Locale best, as SetLanguageMatch says;
// "" for an app without catalogs. Package i18n's Language, which apps use,
// reads it.
func (e *Env) Language() string { return e.language.Get() }

// shownLanguage is the language the platform says its menus in: the one
// the app shows, else the locale it runs in.
func (t *Tree) shownLanguage() string {
	e := t.root.Env()
	if lang := e.language.GetUntracked(); lang != "" {
		return lang
	}
	if forced := e.forced.GetUntracked(); forced != "" {
		return forced
	}
	return e.locale.GetUntracked()
}

var languageMatch atomic.Pointer[func(locale string) string]

// SetLanguageMatch sets how the language the app shows follows its
// locale: package i18n's SetApp sets its catalog's match, nil none. It is
// set as the program initializes, for every app it runs.
func SetLanguageMatch(match func(locale string) string) {
	if match == nil {
		languageMatch.Store(nil)
		return
	}
	languageMatch.Store(&match)
}

// SetLocale makes the app run in locale, a BCP 47 tag, whatever the
// user's or the host's: the language it shows, its formats and its
// direction follow. "" follows theirs again. The app keeps the choice for
// later launches itself, as in its storage.
func (cx *Scope) SetLocale(locale string) {
	cx.Env().forced.Set(locale)
	if t := cx.tree(); t != nil {
		t.changed = true
	}
}

// rightToLeft reports whether lang, a BCP 47 tag, is written right to
// left: by its script when it names one, else by its language.
func rightToLeft(lang string) bool {
	parts := strings.Split(strings.ReplaceAll(lang, "_", "-"), "-")
	for _, p := range parts[1:] {
		if len(p) == 4 {
			switch strings.ToLower(p) {
			case "arab", "hebr", "syrc", "thaa", "nkoo", "adlm", "rohg", "mand", "samr", "mend", "yezi":
				return true
			}
			return false
		}
	}
	switch strings.ToLower(parts[0]) {
	case "ar", "arc", "ckb", "dv", "fa", "he", "iw", "ks", "ps", "sd", "syr", "ug", "ur", "yi", "lrc", "mzn", "glk":
		return true
	}
	return false
}

// OS returns the operating system.
func (e *Env) OS() OS { return e.os.Get() }

// OSVersion returns the system's version: Android's API level as its
// major, zero on the web.
func (e *Env) OSVersion() OSVersion { return e.version.Get() }

// Idiom returns the kind of device.
func (e *Env) Idiom() Idiom { return e.idiom.Get() }

// AppleKeys reports whether shortcuts follow Apple's conventions: "mod"
// is Command and labels use ⌘.
func (e *Env) AppleKeys() bool { return e.appleKeys.Get() }

// WindowMaterial returns the material behind the window's content, or
// WindowMaterialNone when the window is opaque: the core's part of
// desktop.ShowsMaterial, which apps use.
func (e *Env) WindowMaterial() WindowMaterial { return e.material.Get() }

// Desktop reports whether the device is a desktop.
func (e *Env) Desktop() bool { return e.idiom.Get() == IdiomDesktop }

// insetsKey is provided by screens and modals: the insets their content
// keeps clear of, below their headers.
var insetsKey = NewKey[func() Edges]("insets", nil)

// Insets returns what content under cx must keep clear of: the insets of
// the nearest screen or modal, or the surface's safe area.
func Insets(cx *Scope) Edges { return InsetsFunc(cx)() }

// InsetsFunc returns what reads Insets of cx's place, found now and
// tracked as it is called, wherever that is.
func InsetsFunc(cx *Scope) func() Edges {
	if f := insetsKey.Use(cx); f != nil {
		return f
	}
	env := cx.Env()
	return env.SafeArea
}
