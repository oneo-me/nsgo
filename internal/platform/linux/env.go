//go:build linux && !android

package linux

import (
	"os"
	"strings"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
)

const (
	gtkTextDirLTR = 1
	gtkTextDirRTL = 2
	// gtk-xft-dpi is in 1024ths of a point per inch.
	xftDPIUnit = 1024
)

// environment reads what GTK and the session say about where the app
// runs. There is no software keyboard, so its inset stays 0.
func (w *window) environment() driver.Environment {
	dark, reduce, dpi := false, false, int32(96*xftDPIUnit)
	theme := ""
	if settings := gtkSettingsGetDefault(); settings != 0 {
		if colorScheme() {
			dark = settingInt(settings, "gtk-interface-color-scheme") == schemeDark
		} else {
			dark = settingBool(settings, "gtk-application-prefer-dark-theme")
		}
		reduce = !settingBool(settings, "gtk-enable-animations")
		dpi = settingInt(settings, "gtk-xft-dpi")
		theme = settingString(settings, "gtk-theme-name")
	}
	if strings.Contains(strings.ToLower(theme), "dark") || strings.Contains(strings.ToLower(os.Getenv("GTK_THEME")), "dark") {
		dark = true
	}
	scale := 1.0
	if surface := gtkNativeGetSurface(gtkWidgetGetNative(w.surface)); surface != 0 {
		scale = gdkSurfaceGetScale(surface)
	} else if f := gtkWidgetGetScaleFactor(w.surface); f > 0 {
		scale = float64(f)
	}
	if scale <= 0 {
		scale = 1
	}
	fontScale := float32(1)
	if dpi > 0 {
		fontScale = float32(float64(dpi) / float64(96*xftDPIUnit))
	}
	locale := sessionLocale()
	return driver.Environment{
		Dark:         dark,
		FontScale:    fontScale,
		ReduceMotion: reduce,
		RTL:          systemDirection() == gtkTextDirRTL || rightToLeft(locale),
		PixelRatio:   float32(scale),
		Locale:       locale,
		OS:           driver.OSLinux,
		OSVersion:    osVersion(),
		Idiom:        driver.IdiomDesktop,
		TitleBar:     w.titleBarArea(),
		Maximized:    w.win != 0 && gtkWindowIsMaximized(w.win) != 0,
	}
}

// updateEnvironment reads the environment again and reports it when it
// changed.
func (w *window) updateEnvironment() {
	env := w.environment()
	if env == w.env {
		return
	}
	w.env = env
	if w.engine != nil {
		w.engine.SetEnvironment(env)
	}
}

// settingBool reads a gboolean property of settings.
func settingBool(settings uintptr, name string) bool {
	var v int32
	gObjectGet(settings, uintptr(unsafe.Pointer(cstr(name))), uintptr(unsafe.Pointer(&v)), 0)
	return v != 0
}

// settingInt reads an int property of settings.
func settingInt(settings uintptr, name string) int32 {
	var v int32
	gObjectGet(settings, uintptr(unsafe.Pointer(cstr(name))), uintptr(unsafe.Pointer(&v)), 0)
	return v
}

// settingString reads a string property of settings; the copy GTK makes
// is freed.
func settingString(settings uintptr, name string) string {
	var p uintptr
	gObjectGet(settings, uintptr(unsafe.Pointer(cstr(name))), uintptr(unsafe.Pointer(&p)), 0)
	s := goString(p)
	if p != 0 {
		gFree(p)
	}
	return s
}

// settingsChangedCallback rereads the environment after a GTK setting
// changed.
func settingsChangedCallback(settings, pspec, data uintptr) {
	if w := windowOf(data); w != nil && !w.closing {
		w.updateEnvironment()
	}
}

// windowStateCallback rereads the environment after the window is
// maximized, restored or made full screen, by the app or the window
// manager.
func windowStateCallback(window, pspec, data uintptr) {
	w := windowOf(data)
	if w == nil || w.closing {
		return
	}
	w.chrome.fullScreen = gtkWindowIsFullscreen(w.win) != 0
	w.updateEnvironment()
}

// sessionLocale returns the user's locale as a BCP 47 tag, such as
// "en-US".
func sessionLocale() string {
	locale := ""
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			locale = v
			break
		}
	}
	// Drop the encoding and modifier: "de_DE.UTF-8@euro" is "de-DE".
	if i := strings.IndexAny(locale, ".@"); i >= 0 {
		locale = locale[:i]
	}
	return strings.ReplaceAll(locale, "_", "-")
}

// rightToLeft reports whether locale's language reads right to left.
func rightToLeft(locale string) bool {
	language, _, _ := strings.Cut(strings.ToLower(locale), "-")
	switch language {
	case "ar", "arc", "ckb", "dv", "fa", "he", "ks", "ku", "ps", "sd", "ug", "ur", "yi":
		return true
	}
	return false
}
