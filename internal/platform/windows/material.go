//go:build windows

package windows

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A window's material is a DWM system backdrop, drawn behind the window's
// content where the content is transparent. Windows 11 22H2 (build 22621)
// and later draw Mica, Acrylic and Mica Alt; earlier Windows keeps the
// window opaque. The window draws through DirectComposition when it starts
// with a material, since only a surface with alpha shows what DWM draws
// behind it, and WS_EX_NOREDIRECTIONBITMAP, which that needs, holds only
// from the window's creation: such a window changes its backdrop any
// time, and one started without stays opaque.

var (
	ntdll                        = syscall.NewLazyDLL("ntdll.dll")
	rtlGetVersion                = ntdll.NewProc("RtlGetVersion")
	dwmExtendFrameIntoClientArea = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
)

const (
	dwmwaSystemBackdropType = 38

	backdropNone    = 1 // DWMSBT_NONE
	backdropMica    = 2 // DWMSBT_MAINWINDOW
	backdropAcrylic = 3 // DWMSBT_TRANSIENTWINDOW
	backdropMicaAlt = 4 // DWMSBT_TABBEDWINDOW

	backdropBuild = 22621
)

// margins is MARGINS: how far the DWM frame extends into the client area.
type margins struct{ Left, Right, Top, Bottom int32 }

// osVersionInfo is RTL_OSVERSIONINFOW.
type osVersionInfo struct {
	Size, Major, Minor, Build, PlatformID uint32
	CSDVersion                            [128]uint16
}

// windowsBuild returns the build of Windows 10 or 11, the maximum on a
// later major version. RtlGetVersion tells the version whatever the
// executable's manifest declares.
var windowsBuild = sync.OnceValue(func() uint32 {
	var v osVersionInfo
	v.Size = uint32(unsafe.Sizeof(v))
	rtlGetVersion.Call(uintptr(unsafe.Pointer(&v)))
	switch {
	case v.Major > 10:
		return ^uint32(0)
	case v.Major < 10:
		return 0
	}
	return v.Build
})

// systemBackdrops reports Windows 11 22H2 or later, where DWM draws the
// materials of DWMWA_SYSTEMBACKDROP_TYPE.
func systemBackdrops() bool { return windowsBuild() >= backdropBuild }

// compositionAvailable reports whether DirectComposition can start. A
// material window shows its content through it, over the backdrop.
func compositionAvailable() bool {
	if dcompositionCreateDevice2.Find() != nil {
		return false
	}
	var device *comObject
	hr, _, _ := dcompositionCreateDevice2.Call(0, uintptr(unsafe.Pointer(&iidDCompDesktopDevice)),
		uintptr(unsafe.Pointer(&device)))
	if failed(hr) || device == nil {
		return false
	}
	device.release()
	return true
}

// backdropFor returns the system backdrop closest to m: Mica and its Alt
// for window materials, Acrylic for transient surfaces.
func backdropFor(m driver.WindowMaterial) int32 {
	switch m {
	case driver.WindowMaterialNone:
		return backdropNone
	case driver.WindowMaterialMica:
		return backdropMica
	case driver.WindowMaterialTabbed:
		return backdropMicaAlt
	case driver.WindowMaterialAcrylic, driver.WindowMaterialMenu, driver.WindowMaterialPopover,
		driver.WindowMaterialHUD, driver.WindowMaterialSheet, driver.WindowMaterialTooltip,
		driver.WindowMaterialSelection, driver.WindowMaterialFullScreenUI:
		return backdropAcrylic
	}
	return backdropMica
}

// applyMaterial makes DWM draw the window's backdrop. It reports whether
// it did; the caller keeps the window opaque when it did not.
func (w *window) applyMaterial() bool {
	if w.backdrop == 0 || w.backdrop == backdropNone {
		return false
	}
	m := margins{-1, -1, -1, -1}
	if hr, _, _ := dwmExtendFrameIntoClientArea.Call(w.hwnd, uintptr(unsafe.Pointer(&m))); failed(hr) {
		return false
	}
	backdrop := w.backdrop
	if hr, _, _ := dwmSetWindowAttribute.Call(w.hwnd, dwmwaSystemBackdropType,
		uintptr(unsafe.Pointer(&backdrop)), 4); failed(hr) {
		return false
	}
	return true
}

// setMaterial changes the backdrop of a window started with a material,
// WindowMaterialNone making it opaque; other windows stay opaque.
func (w *window) setMaterial(m driver.WindowMaterial) {
	if !w.baseComposed || m == w.material {
		return
	}
	w.backdrop, w.material = backdropFor(m), driver.WindowMaterialNone
	switch {
	case m == driver.WindowMaterialNone:
		backdrop := w.backdrop
		dwmSetWindowAttribute.Call(w.hwnd, dwmwaSystemBackdropType, uintptr(unsafe.Pointer(&backdrop)), 4)
	case w.applyMaterial():
		w.material = m
	default:
		w.backdrop = backdropNone
	}
	w.updateEnvironment()
	w.redrawAll()
}
