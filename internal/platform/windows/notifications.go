//go:build windows

package windows

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// System notifications as Windows' toasts through WinRT's
// ToastNotificationManager, and the badge as the taskbar button's overlay
// icon. An app outside an MSIX names itself with an AppUserModelID, its
// ID, which the current user's registry gives the app's name and icon,
// as toasts need; a press reaches the app while it runs, through the
// toast's Activated event, or starts the app through the COM activator.

var (
	combase                   = syscall.NewLazyDLL("combase.dll")
	roGetActivationFactory    = combase.NewProc("RoGetActivationFactory")
	roActivateInstance        = combase.NewProc("RoActivateInstance")
	windowsCreateString       = combase.NewProc("WindowsCreateString")
	windowsDeleteString       = combase.NewProc("WindowsDeleteString")
	getCurrentPackageFullName = kernel32.NewProc("GetCurrentPackageFullName")
	advapi32                  = syscall.NewLazyDLL("advapi32.dll")
	regCreateKeyExW           = advapi32.NewProc("RegCreateKeyExW")
	regSetValueExW            = advapi32.NewProc("RegSetValueExW")

	createCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	deleteDC           = gdi32.NewProc("DeleteDC")
	selectObject       = gdi32.NewProc("SelectObject")
	getDIBits          = gdi32.NewProc("GetDIBits")
	getObjectW         = gdi32.NewProc("GetObjectW")
	createDIBSection   = gdi32.NewProc("CreateDIBSection")
	createBitmap       = gdi32.NewProc("CreateBitmap")
	createFontW        = gdi32.NewProc("CreateFontW")
	setBkMode          = gdi32.NewProc("SetBkMode")
	setTextColor       = gdi32.NewProc("SetTextColor")
	drawTextW          = user32.NewProc("DrawTextW")
	createIconIndirect = user32.NewProc("CreateIconIndirect")
	destroyIcon        = user32.NewProc("DestroyIcon")
	getIconInfo        = user32.NewProc("GetIconInfo")
	loadImageW         = user32.NewProc("LoadImageW")

	iidToastManagerStatics  = parseGUID("50ac103f-d235-4598-bbef-98fe4d1a3ad4")
	iidToastManagerStatics2 = parseGUID("7ab93c52-0e48-4750-ba9d-1a4113981847")
	iidToastFactory         = parseGUID("04124b20-82c6-4229-b109-fd9ed4662b53")
	iidToastNotification2   = parseGUID("9dfb9fd1-143a-490e-90bf-b9fba7132de7")
	iidXmlDocumentIO        = parseGUID("6cd0e74e-ee65-4489-9ebf-ca43e87ba637")
	iidXmlDocument          = parseGUID("f7f3a506-1e87-42d6-bcfb-b8c809fa5494")
	// TypedEventHandler<ToastNotification, IInspectable>, Activated's.
	iidToastActivated = parseGUID("ab54de2d-97d9-5528-b6ad-105afe156530")
	clsidTaskbarList  = parseGUID("56fdf344-fd6d-11d0-958a-006097c9a090")
	iidTaskbarList3   = parseGUID("ea1afb91-9e28-4b86-90e9-9e9f8a5eee6f")
)

// The methods called, by their vtable slots: IInspectable's six come
// before an interface's own.
const (
	toastStaticsCreateNotifier       = 6
	toastStaticsCreateNotifierWithID = 7
	toastStatics2History             = 6
	toastNotifierShow                = 6
	toastNotifierSetting             = 8
	toastFactoryCreate               = 6
	toastAddActivated                = 11
	toast2PutTag                     = 6
	toast2PutGroup                   = 8
	toastHistoryRemoveGroupedTagID   = 8
	toastHistoryRemoveGroupedTag     = 9
	xmlDocumentIOLoadXML             = 6
	taskbarHrInit                    = 3
	taskbarSetOverlayIcon            = 18
)

// comApartment makes the UI thread a single-threaded apartment, once, as
// drops and WebView2 do.
var comApartment = sync.OnceFunc(func() { oleInitialize.Call(0) })

// toastGroup is the group the app's toasts are in, so a close names one
// by its tag.
const toastGroup = "nsgo"

// toasts is the toast notifier and the toasts showing, by the app's ID,
// which the main thread keeps.
var toasts struct {
	once     sync.Once
	notifier *comObject
	aumid    string
	packaged bool
	shown    map[string]*comObject
}

// hstring makes a WinRT string of s, which deleteHString frees.
func hstring(s string) uintptr {
	u := utf16.Encode([]rune(s))
	var h uintptr
	if len(u) == 0 {
		return 0
	}
	windowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), uintptr(unsafe.Pointer(&h)))
	return h
}

func deleteHString(h uintptr) {
	if h != 0 {
		windowsDeleteString.Call(h)
	}
}

// activationFactory returns WinRT class name's factory as interface iid,
// nil where Windows has none.
func activationFactory(name string, iid *guid) *comObject {
	h := hstring(name)
	defer deleteHString(h)
	var f *comObject
	if hr, _, _ := roGetActivationFactory.Call(h, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&f))); failed(hr) {
		return nil
	}
	return f
}

// toastNotifier returns the app's toast notifier, nil where it has none:
// an MSIX's own, else one named by the app's ID, which it registers.
func (w *window) toastNotifier() *comObject {
	toasts.once.Do(func() {
		toasts.shown = map[string]*comObject{}
		var n uint32
		// APPMODEL_ERROR_NO_PACKAGE: the app is no package's.
		r, _, _ := getCurrentPackageFullName.Call(uintptr(unsafe.Pointer(&n)), 0)
		toasts.packaged = r != 15700
		statics := activationFactory("Windows.UI.Notifications.ToastNotificationManager", &iidToastManagerStatics)
		if statics == nil {
			return
		}
		defer statics.release()
		var notifier *comObject
		if toasts.packaged {
			if failed(statics.call(toastStaticsCreateNotifier, uintptr(unsafe.Pointer(&notifier)))) {
				return
			}
		} else {
			toasts.aumid = w.opts.ID
			if toasts.aumid == "" {
				toasts.aumid = "dev.nsgo.app"
			}
			registerAUMID(toasts.aumid, w.opts.Title)
			id := hstring(toasts.aumid)
			hr := statics.call(toastStaticsCreateNotifierWithID, id, uintptr(unsafe.Pointer(&notifier)))
			deleteHString(id)
			if failed(hr) {
				return
			}
		}
		toasts.notifier = notifier
	})
	return toasts.notifier
}

// registerAUMID gives the current user's AppUserModelID id the app's
// name and icon, which its toasts show.
func registerAUMID(id, name string) {
	var key syscall.Handle
	path := `Software\Classes\AppUserModelId\` + id
	if r, _, _ := regCreateKeyExW.Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(wide(path))), 0, 0, 0,
		syscall.KEY_WRITE, 0, uintptr(unsafe.Pointer(&key)), 0); r != 0 {
		return
	}
	defer syscall.RegCloseKey(key)
	set := func(name, value string) {
		v := syscall.StringToUTF16(value)
		regSetValueExW.Call(uintptr(key), uintptr(unsafe.Pointer(wide(name))), 0, syscall.REG_SZ,
			uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)*2))
	}
	if name == "" {
		name = filepath.Base(os.Args[0])
	}
	set("DisplayName", name)
	if icon := appIconPNG(id); icon != "" {
		set("IconUri", icon)
	}
}

// appIconPNG writes the executable's icon as a PNG in the app's cache
// directory, which toasts read, and returns its path, "" without one.
func appIconPNG(id string) string {
	_, dir := engine.AppDirs(id)
	if dir == "" {
		return ""
	}
	instance, _, _ := getModuleHandleW.Call(0)
	// LR_DEFAULTCOLOR at 256 by 256: the largest icon nsgo links.
	const imageIcon, appIcon = 1, 1
	icon, _, _ := loadImageW.Call(instance, appIcon, imageIcon, 256, 256, 0)
	if icon == 0 {
		return ""
	}
	defer destroyIcon.Call(icon)
	img := iconImage(icon)
	if img == nil {
		return ""
	}
	var buf bytes.Buffer
	if png.Encode(&buf, img) != nil {
		return ""
	}
	path := filepath.Join(dir, "notification-icon.png")
	if os.MkdirAll(dir, 0o755) != nil || os.WriteFile(path, buf.Bytes(), 0o644) != nil {
		return ""
	}
	return path
}

// bitmapInfo is BITMAPINFO with no color table.
type bitmapInfo struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
	Colors                       [1]uint32
}

// iconInfo is ICONINFO.
type iconInfo struct {
	Icon        int32
	HotX, HotY  uint32
	Mask, Color uintptr
}

// gdiBitmap is BITMAP.
type gdiBitmap struct {
	Type, Width, Height, WidthBytes int32
	Planes, BitsPixel               uint16
	Bits                            uintptr
}

// iconImage reads an icon's color bitmap, nil for one without alpha.
func iconImage(icon uintptr) *image.NRGBA {
	var info iconInfo
	if r, _, _ := getIconInfo.Call(icon, uintptr(unsafe.Pointer(&info))); r == 0 {
		return nil
	}
	defer deleteObject.Call(info.Mask)
	defer deleteObject.Call(info.Color)
	var bm gdiBitmap
	if info.Color == 0 || func() bool {
		r, _, _ := getObjectW.Call(info.Color, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm)))
		return r == 0
	}() {
		return nil
	}
	w, h := int(bm.Width), int(bm.Height)
	pix := make([]byte, w*h*4)
	bi := bitmapInfo{Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	bi.Size = uint32(unsafe.Sizeof(bi)) - 4
	dc, _, _ := createCompatibleDC.Call(0)
	defer deleteDC.Call(dc)
	if n, _, _ := getDIBits.Call(dc, info.Color, 0, uintptr(h), uintptr(unsafe.Pointer(&pix[0])),
		uintptr(unsafe.Pointer(&bi)), 0); n == 0 {
		return nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	alpha := false
	for i := 0; i < len(pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = pix[i+2], pix[i+1], pix[i], pix[i+3]
		alpha = alpha || pix[i+3] != 0
	}
	if !alpha {
		return nil
	}
	return img
}

// notify carries out a notification's item on the main thread, in the
// single-threaded apartment WinRT and the shell's objects need there.
func (w *window) notify(it *driver.MountItem) {
	comApartment()
	e, id := w.engine, it.Count
	switch it.Kind {
	case driver.ItemNotificationPermission:
		p := driver.NotificationsUnavailable
		if n := w.toastNotifier(); n != nil {
			// NotificationSetting: Enabled, then the ways it is turned off.
			var setting int32
			if !failed(n.call(toastNotifierSetting, uintptr(unsafe.Pointer(&setting)))) {
				p = driver.NotificationsDenied
				if setting == 0 {
					p = driver.NotificationsGranted
				}
			}
		}
		e.Dispatch(driver.Event{Kind: driver.EventNotification, Count: id, Index: p})
	case driver.ItemShowNotification:
		e.Dispatch(driver.Event{Kind: driver.EventNotification, Count: id, Finished: w.showToast(*it.Notification)})
	case driver.ItemCloseNotification:
		w.closeToast(it.Value)
	case driver.ItemBadge:
		w.setOverlayBadge(it.Count)
	}
}

// toastXML is a toast's content: its title, its body and, when silent,
// no sound.
func toastXML(n driver.Notification) string {
	esc := func(s string) string {
		var b bytes.Buffer
		_ = xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	s := `<toast launch="id=` + esc(n.ID) + `"><visual><binding template="ToastGeneric"><text>` + esc(n.Title) + `</text>`
	if n.Body != "" {
		s += `<text>` + esc(n.Body) + `</text>`
	}
	s += `</binding></visual>`
	if n.Silent {
		s += `<audio silent="true"/>`
	}
	return s + `</toast>`
}

// showToast shows n as a toast tagged with its ID, replacing the one of
// that tag, and reports whether the notifier took it.
func (w *window) showToast(n driver.Notification) bool {
	notifier := w.toastNotifier()
	if notifier == nil {
		return false
	}
	var inspectable *comObject
	class := hstring("Windows.Data.Xml.Dom.XmlDocument")
	hr, _, _ := roActivateInstance.Call(class, uintptr(unsafe.Pointer(&inspectable)))
	deleteHString(class)
	if failed(hr) {
		return false
	}
	defer inspectable.release()
	io := inspectable.query(&iidXmlDocumentIO)
	if io == nil {
		return false
	}
	content := hstring(toastXML(n))
	hr = io.call(xmlDocumentIOLoadXML, content)
	deleteHString(content)
	io.release()
	if failed(hr) {
		return false
	}
	doc := inspectable.query(&iidXmlDocument)
	factory := activationFactory("Windows.UI.Notifications.ToastNotification", &iidToastFactory)
	if doc == nil || factory == nil {
		doc.release()
		factory.release()
		return false
	}
	var toast *comObject
	hr = factory.call(toastFactoryCreate, doc.ptr(), uintptr(unsafe.Pointer(&toast)))
	factory.release()
	doc.release()
	if failed(hr) {
		return false
	}
	if t2 := toast.query(&iidToastNotification2); t2 != nil {
		tag, group := hstring(n.ID), hstring(toastGroup)
		t2.call(toast2PutTag, tag)
		t2.call(toast2PutGroup, group)
		deleteHString(tag)
		deleteHString(group)
		t2.release()
	}
	appID, e := n.ID, w.engine
	pressed := newHandler(&iidToastActivated, func(uintptr, unsafe.Pointer) {
		if toastPresses.first(appID, time.Now()) {
			e.Dispatch(driver.Event{Kind: driver.EventNotificationPress, Text: appID})
		}
		w.post(func() {
			if old := toasts.shown[appID]; old != nil {
				old.release()
				delete(toasts.shown, appID)
			}
			showWindow.Call(w.hwnd, swRestore)
			setForegroundWindow.Call(w.hwnd)
		})
	})
	pressed.agile = true
	var token eventToken
	toast.call(toastAddActivated, pressed.ptr(), uintptr(unsafe.Pointer(&token)))
	pressed.release()
	if failed(notifier.call(toastNotifierShow, toast.ptr())) {
		toast.release()
		return false
	}
	if old := toasts.shown[n.ID]; old != nil {
		old.release()
	}
	toasts.shown[n.ID] = toast
	return true
}

// closeToast removes the toast tagged id from the screen and the action
// center.
func (w *window) closeToast(id string) {
	if old := toasts.shown[id]; old != nil {
		old.release()
		delete(toasts.shown, id)
	}
	if w.toastNotifier() == nil {
		return
	}
	statics := activationFactory("Windows.UI.Notifications.ToastNotificationManager", &iidToastManagerStatics2)
	if statics == nil {
		return
	}
	defer statics.release()
	var history *comObject
	if failed(statics.call(toastStatics2History, uintptr(unsafe.Pointer(&history)))) {
		return
	}
	defer history.release()
	tag, group := hstring(id), hstring(toastGroup)
	defer deleteHString(tag)
	defer deleteHString(group)
	if toasts.packaged {
		history.call(toastHistoryRemoveGroupedTag, tag, group)
		return
	}
	aumid := hstring(toasts.aumid)
	history.call(toastHistoryRemoveGroupedTagID, tag, group, aumid)
	deleteHString(aumid)
}

// taskbar is the shell's taskbar list, made once on the main thread.
var taskbar = sync.OnceValue(func() *comObject {
	var list *comObject
	if hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidTaskbarList)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidTaskbarList3)), uintptr(unsafe.Pointer(&list))); failed(hr) {
		return nil
	}
	if failed(list.call(taskbarHrInit)) {
		list.release()
		return nil
	}
	return list
})

// setOverlayBadge shows count on the window's taskbar button, a red disc
// drawn at the small icon's size, or clears it for 0.
func (w *window) setOverlayBadge(count uint32) {
	list := taskbar()
	if list == nil {
		return
	}
	if count == 0 {
		list.call(taskbarSetOverlayIcon, w.hwnd, 0, 0)
		return
	}
	const smCxSmIcon = 49
	size, _, _ := getSystemMetrics.Call(smCxSmIcon)
	label := strconv.FormatUint(uint64(count), 10)
	if count > 99 {
		label = "99+"
	}
	icon := badgeIcon(int(max(size, 16)), label)
	if icon == 0 {
		return
	}
	desc := wide(label)
	list.call(taskbarSetOverlayIcon, w.hwnd, icon, uintptr(unsafe.Pointer(desc)))
	// The taskbar keeps a copy.
	destroyIcon.Call(icon)
}

// badgeIcon draws label in white on a red disc size pixels across, as an
// icon.
func badgeIcon(size int, label string) uintptr {
	bi := bitmapInfo{Width: int32(size), Height: -int32(size), Planes: 1, BitCount: 32}
	bi.Size = uint32(unsafe.Sizeof(bi)) - 4
	dc, _, _ := createCompatibleDC.Call(0)
	defer deleteDC.Call(dc)
	var bits unsafe.Pointer
	dib, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if dib == 0 {
		return 0
	}
	defer deleteObject.Call(dib)
	pix := unsafe.Slice((*byte)(bits), size*size*4)
	// The disc's coverage of each pixel, four samples a side.
	cover := make([]byte, size*size)
	r := float64(size) / 2
	for y := range size {
		for x := range size {
			in := 0
			for sy := range 4 {
				for sx := range 4 {
					dx, dy := float64(x)+(float64(sx)+0.5)/4-r, float64(y)+(float64(sy)+0.5)/4-r
					if math.Hypot(dx, dy) <= r {
						in++
					}
				}
			}
			cover[y*size+x] = byte(in * 255 / 16)
			i := (y*size + x) * 4
			// #e81123, Windows' badge red, in BGR.
			pix[i], pix[i+1], pix[i+2] = 0x23, 0x11, 0xe8
		}
	}
	old, _, _ := selectObject.Call(dc, dib)
	const fwBold, transparent, antialiased = 700, 1, 4
	height := -size * 3 / 4
	if len(label) > 1 {
		height = -size * 3 / 5
	}
	font, _, _ := createFontW.Call(uintptr(height), 0, 0, 0, fwBold, 0, 0, 0, 1, 0, 0, antialiased, 0,
		uintptr(unsafe.Pointer(wide("Segoe UI"))))
	oldFont, _, _ := selectObject.Call(dc, font)
	setBkMode.Call(dc, transparent)
	setTextColor.Call(dc, 0xffffff)
	text := syscall.StringToUTF16(label)
	box := rect{Right: int32(size), Bottom: int32(size)}
	const dtCenter, dtVCenter, dtSingleLine = 0x1, 0x4, 0x20
	drawTextW.Call(dc, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)-1), uintptr(unsafe.Pointer(&box)),
		dtCenter|dtVCenter|dtSingleLine)
	selectObject.Call(dc, oldFont)
	deleteObject.Call(font)
	selectObject.Call(dc, old)
	// GDI leaves its text's alpha at 0: the disc's coverage is every
	// pixel's, premultiplied.
	for i, a := range cover {
		p := pix[i*4 : i*4+4]
		p[0], p[1], p[2], p[3] = byte(int(p[0])*int(a)/255), byte(int(p[1])*int(a)/255), byte(int(p[2])*int(a)/255), a
	}
	mask, _, _ := createBitmap.Call(uintptr(size), uintptr(size), 1, 1, 0)
	defer deleteObject.Call(mask)
	info := iconInfo{Icon: 1, Mask: mask, Color: dib}
	icon, _, _ := createIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	return icon
}
