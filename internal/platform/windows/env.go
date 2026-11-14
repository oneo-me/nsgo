//go:build windows

package windows

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var (
	dwmapi = syscall.NewLazyDLL("dwmapi.dll")
	shcore = syscall.NewLazyDLL("shcore.dll")

	systemParametersInfoW    = user32.NewProc("SystemParametersInfoW")
	getUserDefaultLocaleName = kernel32.NewProc("GetUserDefaultLocaleName")
	getLocaleInfoEx          = kernel32.NewProc("GetLocaleInfoEx")
	dwmSetWindowAttribute    = dwmapi.NewProc("DwmSetWindowAttribute")
	enumDisplayMonitors      = user32.NewProc("EnumDisplayMonitors")
	getDpiForMonitor         = shcore.NewProc("GetDpiForMonitor")

	isWindowVisible          = user32.NewProc("IsWindowVisible")
	getForegroundWindow      = user32.NewProc("GetForegroundWindow")
	getWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
)

const (
	spiGetScreenReader        = 0x0046
	spiGetClientAreaAnimation = 0x1042
	localeIReadingLayout      = 0x0070
	localeIFirstDayOfWeek     = 0x100C // 0 Monday to 6 Sunday
	localeSTimeFormat         = 0x1003
	localeIMeasure            = 0x000D // 0 metric, 1 U.S.
	localeReturnNumber        = 0x20000000
	localeNameMaxLength       = 85

	dwmwaUseImmersiveDarkMode    = 20
	dwmwaUseImmersiveDarkModeOld = 19 // before Windows 10 20H1
)

type monitorInfoEx struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
	Device  [32]uint16
}

// environment reads what Windows says about where the app runs, with
// what the hidden title bar's window controls cover. There is no
// software keyboard, so its inset stays 0.
func (w *window) environment() driver.Environment {
	locale := userLocale()
	return driver.Environment{
		Dark:           regDWORD(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "AppsUseLightTheme", 1) == 0,
		FontScale:      float32(regDWORD(`Software\Microsoft\Accessibility`, "TextScaleFactor", 100)) / 100,
		ReduceMotion:   !systemFlag(spiGetClientAreaAnimation, true),
		ScreenReader:   systemFlag(spiGetScreenReader, false),
		RTL:            rightToLeft(locale),
		PixelRatio:     float32(w.scale),
		Locale:         locale,
		LocaleSettings: localeSettings(),
		Displays:       w.displays(),
		OS:             driver.OSWindows,
		OSVersion:      osVersion(),
		Idiom:          driver.IdiomDesktop,
		TitleBar:       w.titleBarArea(),
		WindowMaterial: w.material,
		Maximized:      w.zoomed(),
		FullScreen:     w.chrome.fullScreen,
		WindowFrame:    w.windowFrame(),
		Lifecycle:      w.lifecycle(),
	}
}

// displays reads the monitors in the virtual desktop. Coordinates are
// expressed from the primary monitor's top left, like WindowFrame.
func (w *window) displays() []driver.DisplayInfo {
	type found struct {
		monitor uintptr
		info    monitorInfoEx
	}
	var all []found
	callback := syscall.NewCallback(func(monitor, _, _, data uintptr) uintptr {
		var info monitorInfoEx
		info.Size = uint32(unsafe.Sizeof(info))
		if ok, _, _ := getMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
			all = append(all, found{monitor: monitor, info: info})
		}
		return 1
	})
	enumDisplayMonitors.Call(0, 0, callback, 0)
	if len(all) == 0 {
		return nil
	}
	primary := 0
	for i := range all {
		if all[i].info.Flags&1 != 0 {
			primary = i
			break
		}
	}
	base := all[primary].info.Monitor
	out := make([]driver.DisplayInfo, 0, len(all))
	for _, item := range all {
		scale := float32(1)
		var dpi, dpiY uint32
		if r, _, _ := getDpiForMonitor.Call(item.monitor, 0, uintptr(unsafe.Pointer(&dpi)), uintptr(unsafe.Pointer(&dpiY))); r == 0 && dpi > 0 {
			scale = float32(dpi) / 96
		}
		bounds := func(r rect) driver.Bounds {
			return driver.Bounds{Origin: driver.Point{X: driver.Points(float64(r.Left-base.Left) / float64(scale)), Y: driver.Points(float64(r.Top-base.Top) / float64(scale))}, Size: driver.Size{Width: driver.Points(float64(r.Right-r.Left) / float64(scale)), Height: driver.Points(float64(r.Bottom-r.Top) / float64(scale))}}
		}
		id := syscall.UTF16ToString(item.info.Device[:])
		if id == "" {
			id = fmt.Sprintf("%x", item.monitor)
		}
		out = append(out, driver.DisplayInfo{ID: id, Frame: bounds(item.info.Monitor), VisibleFrame: bounds(item.info.Work), Scale: scale, Main: item.info.Flags&1 != 0})
	}
	return out
}

// appActive is whether the app is the active one, as the last
// WM_ACTIVATEAPP said, which Windows sends per thread; before the first,
// whether the foreground window is the process's.
var appActive = sync.OnceValue(func() *bool {
	active := false
	if fg, _, _ := getForegroundWindow.Call(); fg != 0 {
		var pid uint32
		getWindowThreadProcessID.Call(fg, uintptr(unsafe.Pointer(&pid)))
		active = pid == uint32(syscall.Getpid())
	}
	return &active
})

// lifecycle is whether the app is in use: in the background while the
// window, an embedded view's root window, is minimized or hidden, active
// while the app is the active one.
func (w *window) lifecycle() driver.Lifecycle {
	root := w.owner()
	if visible, _, _ := isWindowVisible.Call(root); visible == 0 || isIconicWindow(root) {
		return driver.LifecycleBackground
	}
	if *appActive() {
		return driver.LifecycleActive
	}
	return driver.LifecycleInactive
}

// updateEnvironment reads the environment again, after Windows broadcast
// a settings change, and reports it when it changed.
func (w *window) updateEnvironment() {
	env := w.environment()
	if env.Equal(w.env) {
		return
	}
	dark := w.shownDark()
	w.env = env
	if w.shownDark() != dark {
		w.darkTitleBar(w.shownDark())
	}
	w.engine.SetEnvironment(env)
}

// darkTitleBar matches the title bar to the appearance, where DWM can.
func (w *window) darkTitleBar(dark bool) {
	if dwmSetWindowAttribute.Find() != nil {
		return
	}
	on := int32(boolArg(dark))
	if r, _, _ := dwmSetWindowAttribute.Call(w.hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&on)), 4); r != 0 {
		dwmSetWindowAttribute.Call(w.hwnd, dwmwaUseImmersiveDarkModeOld, uintptr(unsafe.Pointer(&on)), 4)
	}
}

// regDWORD reads a DWORD under HKEY_CURRENT_USER, or returns fallback.
func regDWORD(path, name string, fallback uint32) uint32 {
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, wide(path), 0, syscall.KEY_READ, &key) != nil {
		return fallback
	}
	defer syscall.RegCloseKey(key)
	var value, kind uint32
	size := uint32(unsafe.Sizeof(value))
	if syscall.RegQueryValueEx(key, wide(name), nil, &kind, (*byte)(unsafe.Pointer(&value)), &size) != nil ||
		kind != syscall.REG_DWORD {
		return fallback
	}
	return value
}

// systemFlag reads a BOOL system parameter, or returns fallback.
func systemFlag(action uintptr, fallback bool) bool {
	var on int32
	if r, _, _ := systemParametersInfoW.Call(action, 0, uintptr(unsafe.Pointer(&on)), 0); r == 0 {
		return fallback
	}
	return on != 0
}

// userLocale returns the user's locale as a BCP 47 tag, such as "en-US".
func userLocale() string {
	var buf [localeNameMaxLength]uint16
	if n, _, _ := getUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); n == 0 {
		return ""
	}
	// An alternate sort order follows an underscore, as in "de-DE_phoneb".
	name, _, _ := strings.Cut(syscall.UTF16ToString(buf[:]), "_")
	return name
}

// localeSettings reads the regional format with the user's changes:
// the first day of the week, the time format's hour letter and the
// measurement system.
func localeSettings() driver.LocaleSettings {
	var s driver.LocaleSettings
	number := func(kind uintptr) (uint32, bool) {
		var v uint32
		r, _, _ := getLocaleInfoEx.Call(0, kind|localeReturnNumber, uintptr(unsafe.Pointer(&v)), unsafe.Sizeof(v)/2)
		return v, r != 0
	}
	if first, ok := number(localeIFirstDayOfWeek); ok && first <= 6 {
		s.FirstDayOfWeek = int(first) + 1
	}
	var buf [80]uint16
	if r, _, _ := getLocaleInfoEx.Call(0, localeSTimeFormat, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r != 0 {
		// The hours' letter, outside quoted text.
		quoted := false
		for _, c := range syscall.UTF16ToString(buf[:]) {
			if c == '\'' {
				quoted = !quoted
				continue
			}
			if c == 'H' && !quoted {
				s.HourCycle = "h23"
				break
			}
			if c == 'h' && !quoted {
				s.HourCycle = "h12"
				break
			}
		}
	}
	if m, ok := number(localeIMeasure); ok {
		s.MeasurementSystem = map[uint32]string{0: "metric", 1: "us"}[m]
	}
	return s
}

// rightToLeft reports whether locale's script reads right to left.
func rightToLeft(locale string) bool {
	var layout uint32 // 0 left to right, 1 right to left, 2 and 3 vertical
	r, _, _ := getLocaleInfoEx.Call(uintptr(unsafe.Pointer(wide(locale))), localeIReadingLayout|localeReturnNumber,
		uintptr(unsafe.Pointer(&layout)), unsafe.Sizeof(layout)/2) // in UTF-16 units
	return r != 0 && layout == 1
}
