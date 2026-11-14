//go:build linux && !android

package linux

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
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
	var frame driver.WindowFrame
	if w.win != 0 {
		var width, height int32
		gtkWindowGetDefaultSize(w.win, &width, &height)
		frame.Width, frame.Height = driver.Points(width), driver.Points(height)
	}
	return driver.Environment{
		Dark:           dark,
		FontScale:      fontScale,
		ReduceMotion:   reduce,
		RTL:            systemDirection() == gtkTextDirRTL || rightToLeft(locale),
		PixelRatio:     float32(scale),
		Locale:         locale,
		LocaleSettings: localeSettings(),
		Displays:       displays(),
		OS:             driver.OSLinux,
		HasWebView:     true,
		OSVersion:      osVersion(),
		Idiom:          driver.IdiomDesktop,
		TitleBar:       w.titleBarArea(),
		Maximized:      w.win != 0 && gtkWindowIsMaximized(w.win) != 0,
		FullScreen:     w.chrome.fullScreen,
		WindowFrame:    frame,
		Lifecycle:      w.lifecycle(),
	}
}

// displays reads GDK's monitors in the same logical coordinate space as
// WindowFrame. GTK exposes no primary-monitor flag, so the first monitor is
// the primary one, matching GTK's ordering.
func displays() []driver.DisplayInfo {
	display := gdkDisplayGetDefault()
	if display == 0 {
		return nil
	}
	model := gdkDisplayGetMonitors(display)
	if model == 0 {
		return nil
	}
	watchDisplays(model)
	count := gListModelGetNItems(model)
	out := make([]driver.DisplayInfo, 0, count)
	for i := uint32(0); i < count; i++ {
		monitor := gListModelGetItem(model, i)
		if monitor == 0 {
			continue
		}
		watchMonitor(monitor)
		var frame gdkMonitorRectangle
		gdkMonitorGetGeometry(monitor, &frame)
		// Wayland reports no work area, so there the whole monitor is the visible one.
		visible := frame
		if gdkX11MonitorGetWorkarea != nil && gTypeIsA(monitor, gdkX11MonitorGetType()) != 0 {
			gdkX11MonitorGetWorkarea(monitor, &visible)
		}
		// The fractional scale, as PixelRatio's: the scale factor is its ceiling.
		var scale float32
		if gdkMonitorGetScale != nil {
			scale = float32(gdkMonitorGetScale(monitor))
		} else {
			scale = float32(gdkMonitorGetScaleFactor(monitor))
		}
		if scale <= 0 {
			scale = 1
		}
		point := func(r gdkMonitorRectangle) driver.Bounds {
			return driver.Bounds{Origin: driver.Point{X: driver.Points(float32(r.X)), Y: driver.Points(float32(r.Y))}, Size: driver.Size{Width: driver.Points(float32(r.Width)), Height: driver.Points(float32(r.Height))}}
		}
		// The connector, as DP-1 or HDMI-A-1, names the display across runs; the object's address only while it is connected.
		id := ""
		if c := gdkMonitorGetConnector(monitor); c != 0 {
			id = goString(c)
		}
		if id == "" {
			id = fmt.Sprintf("%x", monitor)
		}
		out = append(out, driver.DisplayInfo{ID: id, Frame: point(frame), VisibleFrame: point(visible), Scale: scale, Main: i == 0})
		gObjectUnref(monitor)
	}
	return out
}

var watchedDisplays uintptr

func watchDisplays(model uintptr) {
	if watchedDisplays == model {
		return
	}
	watchedDisplays = model
	gSignalConnect(model, "items-changed", cbDisplaysChanged, 0)
}

// watchMonitor rereads the displays when monitor moves, resizes or changes its scale, which GDK reports on the monitor rather than as a change to the model's items. The mark it leaves on the monitor goes with it.
func watchMonitor(monitor uintptr) {
	key := cstr("nsgo-displays-watched")
	if gObjectGetData(monitor, key) != 0 {
		return
	}
	gObjectSetData(monitor, key, 1)
	for _, signal := range []string{"notify::geometry", "notify::scale", "notify::scale-factor", "invalidate"} {
		gSignalConnect(monitor, signal, cbDisplaysChanged, 0)
	}
}

func displaysChangedCallback(_, _, _, _ uintptr) {
	windowHandles.Range(func(_, value any) bool {
		if w := value.(*window); !w.closing {
			w.updateEnvironment()
		}
		return true
	})
}

// GdkToplevelState's flags that hide a window: minimized, and suspended
// (on another workspace or covered, where the compositor says).
const toplevelMinimized, toplevelSuspended = 1 << 0, 1 << 16

// lifecycle is whether the app is in use: in the background while the
// surface is unmapped or its window minimized or suspended, active while
// one of the app's windows has the focus.
func (w *window) lifecycle() driver.Lifecycle {
	if w.surface == 0 || gtkWidgetGetMapped(w.surface) == 0 {
		return driver.LifecycleBackground
	}
	if top := gtkNativeGetSurface(gtkWidgetGetNative(w.surface)); top != 0 &&
		gdkToplevelGetState(top)&(toplevelMinimized|toplevelSuspended) != 0 {
		return driver.LifecycleBackground
	}
	active := false
	windowHandles.Range(func(_, v any) bool {
		if other := v.(*window); !other.closing && other.rootWindow() != 0 && gtkWindowIsActive(other.rootWindow()) != 0 {
			active = true
		}
		return !active
	})
	if active {
		return driver.LifecycleActive
	}
	return driver.LifecycleInactive
}

// rootWindow is the GtkWindow the surface shows in, the host's for an
// embedded one; 0 while it is in none.
func (w *window) rootWindow() uintptr {
	if w.win != 0 {
		return w.win
	}
	if w.surface == 0 {
		return 0
	}
	return gtkWidgetGetRoot(w.surface)
}

// mappedCallback follows the surface's map and unmap, and on the first
// map its window's focus and its toplevel's state, which exist only once
// it is in a window.
func mappedCallback(widget, data uintptr) {
	w := windowOf(data)
	if w == nil || w.closing {
		return
	}
	if gtkWidgetGetMapped(w.surface) != 0 && w.watchedRoot == 0 {
		if root := w.rootWindow(); root != 0 {
			w.watchedRoot = root
			gSignalConnect(root, "notify::is-active", cbActivity, w.handle)
			if top := gtkNativeGetSurface(gtkWidgetGetNative(w.surface)); top != 0 {
				gSignalConnect(top, "notify::state", cbActivity, w.handle)
			}
		}
	}
	w.updateEnvironment()
}

// activityCallback reads every window's environment again as one's focus
// or toplevel state changes, the app being active while any is.
func activityCallback(object, pspec, data uintptr) {
	windowHandles.Range(func(_, v any) bool {
		if w := v.(*window); !w.closing {
			w.updateEnvironment()
		}
		return true
	})
}

// updateEnvironment reads the environment again and reports it when it
// changed.
func (w *window) updateEnvironment() {
	env := w.environment()
	if env.Equal(w.env) {
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

// localeSettings is what the session sets apart from the language: the
// region of LC_TIME's formats, LC_MEASUREMENT's system, and GNOME's
// clock-format over the hours.
func localeSettings() driver.LocaleSettings {
	region := func(category string) string {
		for _, key := range []string{"LC_ALL", category, "LANG"} {
			if v := os.Getenv(key); v != "" {
				v, _, _ = strings.Cut(v, ".")
				v, _, _ = strings.Cut(v, "@")
				if _, r, ok := strings.Cut(v, "_"); ok && len(r) == 2 {
					return strings.ToUpper(r)
				}
				return ""
			}
		}
		return ""
	}
	s := driver.LocaleSettings{Region: region("LC_TIME")}
	switch region("LC_MEASUREMENT") {
	case "":
	case "US", "LR", "MM":
		s.MeasurementSystem = "us"
	case "GB":
		s.MeasurementSystem = "uk"
	default:
		s.MeasurementSystem = "metric"
	}
	switch gnomeClockFormat() {
	case "24h":
		s.HourCycle = "h23"
	case "12h":
		s.HourCycle = "h12"
	}
	return s
}

// gnomeClockFormat is GNOME's clock-format setting, "" where its schema
// is not installed.
func gnomeClockFormat() string {
	const schema = "org.gnome.desktop.interface"
	gnomeSettingsOnce.Do(func() {
		var lookup, source, settingsNew uintptr
		var err error
		if source, err = purego.Dlsym(gioLib, "g_settings_schema_source_get_default"); err != nil {
			return
		}
		if lookup, err = purego.Dlsym(gioLib, "g_settings_schema_source_lookup"); err != nil {
			return
		}
		if settingsNew, err = purego.Dlsym(gioLib, "g_settings_new"); err != nil {
			return
		}
		src, _, _ := purego.SyscallN(source)
		if src == 0 {
			return
		}
		if found, _, _ := purego.SyscallN(lookup, src, uintptr(unsafe.Pointer(cstr(schema))), 1); found == 0 {
			return
		}
		gnomeSettings, _, _ = purego.SyscallN(settingsNew, uintptr(unsafe.Pointer(cstr(schema))))
		gnomeGetString, _ = purego.Dlsym(gioLib, "g_settings_get_string")
	})
	if gnomeSettings == 0 || gnomeGetString == 0 {
		return ""
	}
	p, _, _ := purego.SyscallN(gnomeGetString, gnomeSettings, uintptr(unsafe.Pointer(cstr("clock-format"))))
	s := goString(p)
	if p != 0 {
		gFree(p)
	}
	return s
}

var (
	gnomeSettingsOnce             sync.Once
	gnomeSettings, gnomeGetString uintptr
)

// rightToLeft reports whether locale's language reads right to left.
func rightToLeft(locale string) bool {
	language, _, _ := strings.Cut(strings.ToLower(locale), "-")
	switch language {
	case "ar", "arc", "ckb", "dv", "fa", "he", "ks", "ku", "ps", "sd", "ug", "ur", "yi":
		return true
	}
	return false
}
