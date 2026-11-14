//go:build windows

package windows

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// Unpackaged desktop toasts are activated through a COM local server. The
// Start menu shortcut names the activator CLSID; the shell starts the app
// with LocalServer32 after it has exited and calls INotificationActivationCallback.

var (
	coRegisterClassObject = ole32.NewProc("CoRegisterClassObject")
	coRevokeClassObject   = ole32.NewProc("CoRevokeClassObject")

	iidClassFactory                = parseGUID("00000001-0000-0000-c000-000000000046")
	iidNotificationActivation      = parseGUID("53e31837-6600-4a81-9395-75cffe746f94")
	clsidShellLink                 = parseGUID("00021401-0000-0000-c000-000000000046")
	iidShellLink                   = parseGUID("000214f9-0000-0000-c000-000000000046")
	iidPersistFile                 = parseGUID("0000010b-0000-0000-c000-000000000046")
	iidPropertyStore               = parseGUID("886d8eeb-8cf2-4446-8d02-cdba1dbdcf99")
	pkeyAppUserModelID             = propertyKey{fmtid: parseGUID("9f4c2855-9f79-4b39-a8d0-e1d42de1d5f3"), pid: 5}
	pkeyAppUserModelToastActivator = propertyKey{fmtid: parseGUID("9f4c2855-9f79-4b39-a8d0-e1d42de1d5f3"), pid: 26}
)

const (
	clsctxLocalServer = 4
	regclsMultipleUse = 1

	// IClassFactory and IPersistFile methods after IUnknown.
	classFactoryCreateInstance = 3
	classFactoryLockServer     = 4
	persistFileLoad            = 5
	persistFileSave            = 6
	propertyStoreSetValue      = 6
	propertyStoreCommit        = 7
)

type propertyKey struct {
	fmtid guid
	pid   uint32
}

type propVariantString struct {
	vt       uint16
	reserved [3]uint16
	value    uintptr
}

// toastActivatorCLSID is stable for an app ID and does not need to be
// stored in the package. Version 5 UUID bits keep the value a valid CLSID.
func toastActivatorCLSID(id string) guid {
	if id == "" {
		id = "dev.nsgo.app"
	}
	h := sha1.Sum([]byte("nsgo-toast-activator:" + id))
	h[6] = h[6]&0x0f | 0x50
	h[8] = h[8]&0x3f | 0x80
	return guid{
		a: binary.BigEndian.Uint32(h[0:4]),
		b: binary.BigEndian.Uint16(h[4:6]),
		c: binary.BigEndian.Uint16(h[6:8]),
		d: [8]byte{h[8], h[9], h[10], h[11], h[12], h[13], h[14], h[15]},
	}
}

func formatGUID(g guid) string {
	return fmt.Sprintf("{%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x}",
		g.a, g.b, g.c, g.d[0], g.d[1], g.d[2], g.d[3], g.d[4], g.d[5], g.d[6], g.d[7])
}

func setUserRegistryString(path, name, value string) error {
	var key syscall.Handle
	r, _, _ := regCreateKeyExW.Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(wide(path))), 0, 0, 0,
		syscall.KEY_WRITE, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return syscall.Errno(r)
	}
	defer syscall.RegCloseKey(key)
	v := syscall.StringToUTF16(value)
	var valueName *uint16
	if name != "" {
		valueName = wide(name)
	}
	r, _, _ = regSetValueExW.Call(uintptr(key), uintptr(unsafe.Pointer(valueName)), 0, syscall.REG_SZ,
		uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)*2))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func registerToastActivator(id string) {
	if id == "" {
		id = "dev.nsgo.app"
	}
	// Packaged apps declare their activator in the package manifest and must
	// not create a per-user LocalServer32 registration.
	var n uint32
	if r, _, _ := getCurrentPackageFullName.Call(uintptr(unsafe.Pointer(&n)), 0); r != 15700 {
		return
	}
	clsid := toastActivatorCLSID(id)
	exe, err := os.Executable()
	if err != nil {
		return
	}
	command := fmt.Sprintf(`"%s" --toast-activation`, exe)
	path := `Software\Classes\CLSID\` + formatGUID(clsid) + `\LocalServer32`
	if err := setUserRegistryString(path, "", command); err != nil {
		return
	}
	factory := newToastClassFactory()
	var cookie uint32
	hr, _, _ := coRegisterClassObject.Call(uintptr(unsafe.Pointer(&clsid)), factory.ptr(), clsctxLocalServer, regclsMultipleUse,
		uintptr(unsafe.Pointer(&cookie)))
	if failed(hr) {
		factory.release()
		return
	}
	toastActivation.Lock()
	toastActivation.clsid, toastActivation.cookie, toastActivation.factory = clsid, cookie, factory
	toastActivation.Unlock()
}

func unregisterToastActivator() {
	toastActivation.Lock()
	cookie, factory := toastActivation.cookie, toastActivation.factory
	toastActivation.cookie, toastActivation.factory = 0, nil
	toastActivation.Unlock()
	if cookie != 0 {
		coRevokeClassObject.Call(uintptr(cookie))
	}
	if factory != nil {
		factory.release()
	}
}

type toastClassFactory struct {
	vtbl *[5]uintptr
	refs atomic.Int32
}

var toastClassFactoryVtbl = [5]uintptr{
	syscall.NewCallback(toastClassFactoryQueryInterface),
	syscall.NewCallback(toastClassFactoryAddRef),
	syscall.NewCallback(toastClassFactoryRelease),
	syscall.NewCallback(toastClassFactoryCreate),
	syscall.NewCallback(toastClassFactoryLock),
}

func newToastClassFactory() *toastClassFactory {
	f := &toastClassFactory{vtbl: &toastClassFactoryVtbl}
	f.refs.Store(1)
	return f
}
func (f *toastClassFactory) ptr() uintptr { return uintptr(unsafe.Pointer(f)) }
func (f *toastClassFactory) release()     { toastClassFactoryRelease(f) }
func toastClassFactoryQueryInterface(f *toastClassFactory, iid *guid, out *unsafe.Pointer) uintptr {
	if *iid != iidUnknown && *iid != iidClassFactory {
		*out = nil
		return eNoInterface
	}
	*out = unsafe.Pointer(f)
	toastClassFactoryAddRef(f)
	return sOK
}
func toastClassFactoryAddRef(f *toastClassFactory) uintptr { return uintptr(f.refs.Add(1)) }
func toastClassFactoryRelease(f *toastClassFactory) uintptr {
	n := f.refs.Add(-1)
	return uintptr(n)
}
func toastClassFactoryCreate(f *toastClassFactory, outer uintptr, iid *guid, out *unsafe.Pointer) uintptr {
	if outer != 0 {
		*out = nil
		return 0x80040110 // CLASS_E_NOAGGREGATION
	}
	c := newToastActivationCallback()
	hr := toastActivationCallbackQueryInterface(c, iid, out)
	toastActivationCallbackRelease(c)
	return hr
}
func toastClassFactoryLock(_ *toastClassFactory, lock uintptr) uintptr { return sOK }

type toastActivationCallback struct {
	vtbl *[4]uintptr
	refs atomic.Int32
}

var toastActivationCallbackVtbl = [4]uintptr{
	syscall.NewCallback(toastActivationCallbackQueryInterface),
	syscall.NewCallback(toastActivationCallbackAddRef),
	syscall.NewCallback(toastActivationCallbackRelease),
	syscall.NewCallback(toastActivationCallbackActivate),
}

var toastCallbacks struct {
	sync.Mutex
	items map[*toastActivationCallback]struct{}
}

func newToastActivationCallback() *toastActivationCallback {
	c := &toastActivationCallback{vtbl: &toastActivationCallbackVtbl}
	c.refs.Store(1)
	toastCallbacks.Lock()
	if toastCallbacks.items == nil {
		toastCallbacks.items = map[*toastActivationCallback]struct{}{}
	}
	toastCallbacks.items[c] = struct{}{}
	toastCallbacks.Unlock()
	return c
}
func toastActivationCallbackQueryInterface(c *toastActivationCallback, iid *guid, out *unsafe.Pointer) uintptr {
	if *iid != iidUnknown && *iid != iidNotificationActivation {
		*out = nil
		return eNoInterface
	}
	*out = unsafe.Pointer(c)
	toastActivationCallbackAddRef(c)
	return sOK
}
func toastActivationCallbackAddRef(c *toastActivationCallback) uintptr { return uintptr(c.refs.Add(1)) }
func toastActivationCallbackRelease(c *toastActivationCallback) uintptr {
	n := c.refs.Add(-1)
	if n == 0 {
		toastCallbacks.Lock()
		delete(toastCallbacks.items, c)
		toastCallbacks.Unlock()
	}
	return uintptr(n)
}
func toastActivationCallbackActivate(_ *toastActivationCallback, _ *uint16, args *uint16, _ uintptr, _ uint32) uintptr {
	if id := parseToastActivationArgs(wideString(args)); id != "" {
		dispatchToastActivation(id)
	}
	return sOK
}

func parseToastActivationArgs(args string) string {
	args = strings.TrimSpace(args)
	if strings.HasPrefix(args, "id=") {
		return strings.TrimPrefix(args, "id=")
	}
	return ""
}

var toastActivation struct {
	sync.Mutex
	window  *window
	engine  *engine.Engine
	pending []string
	clsid   guid
	cookie  uint32
	factory *toastClassFactory
}

func beginToastActivation(w *window) {
	toastActivation.Lock()
	toastActivation.window = w
	toastActivation.Unlock()
}

func attachToastActivation(w *window, e *engine.Engine) {
	toastActivation.Lock()
	toastActivation.window, toastActivation.engine = w, e
	pending := append([]string(nil), toastActivation.pending...)
	toastActivation.pending = nil
	toastActivation.Unlock()
	for _, id := range pending {
		dispatchToastActivation(id)
	}
}

func endToastActivation(w *window) {
	toastActivation.Lock()
	if toastActivation.window == w {
		toastActivation.window, toastActivation.engine = nil, nil
	}
	toastActivation.Unlock()
}

func dispatchToastActivation(id string) {
	toastActivation.Lock()
	e, w := toastActivation.engine, toastActivation.window
	if e == nil {
		toastActivation.pending = append(toastActivation.pending, id)
	}
	toastActivation.Unlock()
	if e != nil {
		e.Dispatch(driver.Event{Kind: driver.EventNotificationPress, Text: id})
	}
	if w != nil {
		w.post(func() {
			showWindow.Call(w.hwnd, swRestore)
			setForegroundWindow.Call(w.hwnd)
		})
	}
}

// registerToastShortcutCommand is called by the NSIS installer after it has
// created the Start menu link. It is intentionally a short-lived app mode.
func registerToastShortcutCommand(id string) bool {
	for i, arg := range os.Args {
		if arg == "--register-toast" && i+1 < len(os.Args) {
			setToastActivatorShortcut(os.Args[i+1], id, toastActivatorCLSID(id))
			return true
		}
	}
	return false
}

func setToastActivatorShortcut(path, id string, clsid guid) {
	if id == "" {
		id = "dev.nsgo.app"
	}
	var link *comObject
	hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidShellLink)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidShellLink)), uintptr(unsafe.Pointer(&link)))
	if failed(hr) || link == nil {
		return
	}
	defer link.release()
	persist := link.query(&iidPersistFile)
	store := link.query(&iidPropertyStore)
	if persist == nil || store == nil {
		persist.release()
		store.release()
		return
	}
	defer persist.release()
	defer store.release()
	file := wide(path)
	if failed(persist.call(persistFileLoad, uintptr(unsafe.Pointer(file)), 0)) {
		return
	}
	appID := syscall.StringToUTF16(id)
	appIDVariant := propVariantString{vt: 31, value: uintptr(unsafe.Pointer(&appID[0]))} // VT_LPWSTR
	if failed(store.call(propertyStoreSetValue, uintptr(unsafe.Pointer(&pkeyAppUserModelID)), uintptr(unsafe.Pointer(&appIDVariant)))) {
		return
	}
	clsidVariant := propVariantString{vt: 72, value: uintptr(unsafe.Pointer(&clsid))} // VT_CLSID
	if failed(store.call(propertyStoreSetValue, uintptr(unsafe.Pointer(&pkeyAppUserModelToastActivator)), uintptr(unsafe.Pointer(&clsidVariant)))) ||
		failed(store.call(propertyStoreCommit)) {
		return
	}
	persist.call(persistFileSave, uintptr(unsafe.Pointer(file)), 1)
}
