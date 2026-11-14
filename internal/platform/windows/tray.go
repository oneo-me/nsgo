//go:build windows

package windows

import (
	"bytes"
	"image/png"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Windows notification-area icons are owned by the shell. The shell sends
// their button messages back to the window through trayCallbackMessage.
// Custom PNGs become private HICONs; the executable's icon is a shared
// fallback and is therefore never destroyed here.

var (
	shellNotifyIcon        = shell32.NewProc("Shell_NotifyIconW")
	shellNotifyIconGetRect = shell32.NewProc("Shell_NotifyIconGetRect")
)

const (
	nimAdd            = 0x00000000
	nimModify         = 0x00000001
	nimDelete         = 0x00000002
	nimSetVersion     = 0x00000004
	nifMessage        = 0x00000001
	nifIcon           = 0x00000002
	nifTip            = 0x00000004
	notifyIconVersion = 4

	trayCallbackMessage = 0x8000 + 9
	wmNull              = 0x0000
	ninSelect           = 0x0400
	ninKeySelect        = 0x0401
)

// notifyIconData is NOTIFYICONDATAW. The fields after Tip are retained so
// cbSize matches the current Win32 structure on both 32- and 64-bit Windows.
type notifyIconData struct {
	Size        uint32
	Hwnd        uintptr
	ID          uint32
	Flags       uint32
	Callback    uint32
	Icon        uintptr
	Tip         [128]uint16
	State       uint32
	StateMask   uint32
	Info        [256]uint16
	Version     uint32
	InfoTitle   [64]uint16
	InfoFlags   uint32
	Guid        guid
	BalloonIcon uintptr
}

// notifyIconIdentifier identifies an icon for Shell_NotifyIconGetRect.
type notifyIconIdentifier struct {
	Size uint32
	Hwnd uintptr
	ID   uint32
	Guid guid
}

type trayIcon struct {
	cfg     driver.TrayConfig
	icon    uintptr
	private bool
	added   bool
}

func (w *window) applyTray(cfg driver.TrayConfig) {
	tr := w.trays[cfg.ID]
	if cfg.Removed {
		if tr != nil {
			w.removeTray(tr)
		}
		return
	}
	if tr == nil {
		tr = &trayIcon{}
		w.trays[cfg.ID] = tr
	}
	oldCfg, oldIcon, oldPrivate := tr.cfg, tr.icon, tr.private
	tr.cfg = cfg
	tr.icon, tr.private = trayHICON(cfg.Icon)
	data := w.trayData(cfg, tr.icon)
	if tr.icon != 0 {
		data.Flags |= nifIcon
	}
	data.Size = uint32(unsafe.Sizeof(data))
	op := nimModify
	if !tr.added {
		op = nimAdd
	}
	if ok, _, _ := shellNotifyIcon.Call(uintptr(op), uintptr(unsafe.Pointer(&data))); ok == 0 {
		if op == nimAdd {
			delete(w.trays, cfg.ID)
		}
		if tr.private {
			destroyIcon.Call(tr.icon)
		}
		tr.cfg, tr.icon, tr.private = oldCfg, oldIcon, oldPrivate
		return
	}
	if oldPrivate && oldIcon != 0 {
		destroyIcon.Call(oldIcon)
	}
	if op == nimAdd {
		tr.added = true
		version := data
		version.Version = notifyIconVersion
		version.Size = uint32(unsafe.Sizeof(version))
		shellNotifyIcon.Call(uintptr(nimSetVersion), uintptr(unsafe.Pointer(&version)))
	}
}

func (w *window) trayData(cfg driver.TrayConfig, icon uintptr) notifyIconData {
	d := notifyIconData{Hwnd: w.hwnd, ID: cfg.ID, Callback: trayCallbackMessage, Icon: icon}
	d.Flags = nifMessage | nifTip
	if tip := syscall.StringToUTF16(cfg.Tooltip); len(tip) > 0 {
		copy(d.Tip[:], tip[:min(len(tip), len(d.Tip)-1)])
	}
	return d
}

func (w *window) removeTray(tr *trayIcon) {
	d := w.trayData(tr.cfg, tr.icon)
	d.Size = uint32(unsafe.Sizeof(d))
	if tr.added {
		shellNotifyIcon.Call(uintptr(nimDelete), uintptr(unsafe.Pointer(&d)))
	}
	if tr.private && tr.icon != 0 {
		destroyIcon.Call(tr.icon)
	}
	tr.added = false
	delete(w.trays, tr.cfg.ID)
}

func (w *window) removeTrays() {
	for _, tr := range w.trays {
		w.removeTray(tr)
	}
}

// trayRect returns the notification area's screen rectangle. The shell can
// fail this call while Explorer is restarting or before the icon is visible.
func (w *window) trayRect(id uint32) (rect, bool) {
	root := w
	if root.hostWindow != nil {
		root = root.hostWindow
	}
	tr := root.trays[id]
	if tr == nil || !tr.added || root.hwnd == 0 {
		return rect{}, false
	}
	identifier := notifyIconIdentifier{Size: uint32(unsafe.Sizeof(notifyIconIdentifier{})), Hwnd: root.hwnd, ID: id}
	var bounds rect
	if ok, _, _ := shellNotifyIconGetRect.Call(uintptr(unsafe.Pointer(&identifier)), uintptr(unsafe.Pointer(&bounds))); ok != 0 && !bounds.empty() {
		return bounds, true
	}
	return rect{}, false
}

func (w *window) trayMessage(id, message uintptr) {
	tr := w.trays[uint32(id)]
	if tr == nil || w.engine == nil {
		return
	}
	index := uint32(0)
	switch uint32(message) {
	case wmRButtonUp, wmContextMenu:
		index = 1
	case wmLButtonUp, wmMButtonUp, ninSelect, ninKeySelect:
		// primary button (and keyboard activation)
	default:
		return
	}
	w.engine.Dispatch(driver.Event{Kind: driver.EventTray, Count: uint32(id), Index: int(index)})
}

func trayHICON(data []byte) (uintptr, bool) {
	if len(data) > 0 {
		if icon := iconFromPNG(data); icon != 0 {
			return icon, true
		}
	}
	instance, _, _ := getModuleHandleW.Call(0)
	const appIcon = 1
	icon, _, _ := loadIconW.Call(instance, appIcon)
	return icon, false
}

func iconFromPNG(data []byte) uintptr {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Empty() {
		return 0
	}
	const size = 32
	bi := bitmapInfo{Width: size, Height: -size, Planes: 1, BitCount: 32}
	bi.Size = uint32(unsafe.Sizeof(bi)) - 4
	dc, _, _ := createCompatibleDC.Call(0)
	if dc == 0 {
		return 0
	}
	defer deleteDC.Call(dc)
	var bits unsafe.Pointer
	dib, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if dib == 0 || bits == nil {
		return 0
	}
	defer deleteObject.Call(dib)
	pix := unsafe.Slice((*byte)(bits), size*size*4)
	b := img.Bounds()
	for y := range size {
		sy := b.Min.Y + y*b.Dy()/size
		for x := range size {
			sx := b.Min.X + x*b.Dx()/size
			r, g, bl, a := img.At(sx, sy).RGBA()
			i := (y*size + x) * 4
			pix[i] = byte(bl / 0x100)
			pix[i+1] = byte(g / 0x100)
			pix[i+2] = byte(r / 0x100)
			pix[i+3] = byte(a / 0x100)
		}
	}
	mask, _, _ := createBitmap.Call(size, size, 1, 1, 0)
	if mask == 0 {
		return 0
	}
	defer deleteObject.Call(mask)
	info := iconInfo{Icon: 1, Mask: mask, Color: dib}
	icon, _, _ := createIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	return icon
}
