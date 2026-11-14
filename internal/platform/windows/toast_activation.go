//go:build windows

package windows

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// Unpackaged desktop toasts are activated through a COM local server. The Start menu shortcut names the activator CLSID; the shell starts the app with LocalServer32 after it has exited and calls INotificationActivationCallback. The installer and LocalServer32 start the app with the --nsgo- launch flags, which leave os.Args before main (flags.go): --nsgo-register-toast and --nsgo-unregister-toast arrive as Options, and --nsgo-toast-activation only takes away the -Embedding COM appends.

var (
	coRegisterClassObject = ole32.NewProc("CoRegisterClassObject")
	coRevokeClassObject   = ole32.NewProc("CoRevokeClassObject")
	regDeleteTreeW        = advapi32.NewProc("RegDeleteTreeW")

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
	stgmReadWrite              = 2 // STGM_READWRITE
	propertyStoreSetValue      = 6
	propertyStoreCommit        = 7
)

type propertyKey struct {
	fmtid guid
	pid   uint32
}

// propVariant is a PROPVARIANT holding a pointer (propidlbase.h): vt and three reserved words, then a union whose largest members, a count and a pointer such as BLOB's, make it 16 bytes on 386 and 24 on amd64 and arm64. value is an unsafe.Pointer so the collector sees what it points to.
type propVariant struct {
	vt       uint16
	reserved [3]uint16
	value    unsafe.Pointer
	_        uintptr
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

func deleteUserRegistryTree(path string) {
	regDeleteTreeW.Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(wide(path))))
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
	command := fmt.Sprintf(`"%s" --nsgo-toast-activation`, exe)
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
	if id, press := parseToastActivationArgs(wideString(args)); id != "" && toastPresses.first(press, time.Now()) {
		dispatchToastActivation(id)
	}
	return sOK
}

// toastPressWindow is how long a press of a toast is held against another delivery of it.
const toastPressWindow = 5 * time.Second

// toastPresses lets one press of a toast reach the app once: while the app runs, a press arrives both through the toast's Activated event and through the COM activator, in either order, as Microsoft's DesktopToasts sample notes. It goes by the toast's launch arguments, which a nonce makes new for each toast shown, so a press of a toast that replaced a pressed one under the same ID gets through.
var toastPresses toastPressDedup

// toastPressDedup holds when each toast, by its launch arguments, was last pressed.
type toastPressDedup struct {
	sync.Mutex
	at map[string]time.Time
}

// first reports whether a press of the toast with launch arguments press at now is the first delivery of it, not one within toastPressWindow of an earlier press, and records it.
func (d *toastPressDedup) first(press string, now time.Time) bool {
	d.Lock()
	defer d.Unlock()
	for k, at := range d.at {
		if now.Sub(at) >= toastPressWindow {
			delete(d.at, k)
		}
	}
	if _, ok := d.at[press]; ok {
		return false
	}
	if d.at == nil {
		d.at = map[string]time.Time{}
	}
	d.at[press] = now
	return true
}

// toastNonce counts the toasts shown, from the time the process first showed one, so a toast of an earlier run of the app, pressed after it ended, does not share its nonce with one this run shows.
var toastNonce struct {
	sync.Once
	n atomic.Uint64
}

// toastLaunch is the launch arguments of a toast shown with id: a nonce new for each toast, then the ID, which runs to the end and may hold any character.
func toastLaunch(id string) string {
	toastNonce.Do(func() { toastNonce.n.Store(uint64(time.Now().UnixNano())) })
	return "n=" + strconv.FormatUint(toastNonce.n.Add(1), 36) + ";id=" + id
}

// parseToastActivationArgs returns the ID of the toast launch arguments args name and the arguments the press is known by. It also takes the bare id=<ID> of a toast an earlier version of the app showed.
func parseToastActivationArgs(args string) (id, press string) {
	press = strings.TrimSpace(args)
	rest := press
	if nonce, after, ok := strings.Cut(rest, ";"); ok && strings.HasPrefix(nonce, "n=") && strings.HasPrefix(after, "id=") {
		rest = after
	}
	if !strings.HasPrefix(rest, "id=") {
		return "", ""
	}
	return strings.TrimPrefix(rest, "id="), press
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

// unregisterToast removes the current user's activator registration and AppUserModelID of the app, as the uninstaller runs it with --nsgo-unregister-toast.
func unregisterToast(id string) {
	if id == "" {
		id = "dev.nsgo.app"
	}
	deleteUserRegistryTree(`Software\Classes\CLSID\` + formatGUID(toastActivatorCLSID(id)))
	deleteUserRegistryTree(`Software\Classes\AppUserModelId\` + id)
}

// setToastActivatorShortcut stamps the shortcut at path with the app's AppUserModelID and toast activator, as the installer runs it with --nsgo-register-toast once it made the Start menu shortcut.
func setToastActivatorShortcut(path, id string) {
	if id == "" {
		id = "dev.nsgo.app"
	}
	clsid := toastActivatorCLSID(id)
	var link *comObject
	hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidShellLink)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidShellLink)), uintptr(unsafe.Pointer(&link)))
	if failed(hr) || link == nil {
		log.Printf("nsgo: toast shortcut %s: CoCreateInstance(ShellLink): %#x", path, uint32(hr))
		return
	}
	defer link.release()
	persist := link.query(&iidPersistFile)
	store := link.query(&iidPropertyStore)
	if persist == nil || store == nil {
		persist.release()
		store.release()
		log.Printf("nsgo: toast shortcut %s: the shell link has no IPersistFile or IPropertyStore", path)
		return
	}
	defer persist.release()
	defer store.release()
	file := wide(path)
	// Loaded with STGM_READ, the link's property store refuses SetValue and Commit with STG_E_ACCESSDENIED, and Save then writes the shortcut back unchanged.
	if hr := persist.call(persistFileLoad, uintptr(unsafe.Pointer(file)), stgmReadWrite); failed(hr) {
		log.Printf("nsgo: toast shortcut %s: IPersistFile::Load: %#x", path, uint32(hr))
		return
	}
	appID := syscall.StringToUTF16(id)
	appIDVariant := propVariant{vt: 31, value: unsafe.Pointer(&appID[0])} // VT_LPWSTR
	clsidVariant := propVariant{vt: 72, value: unsafe.Pointer(&clsid)}    // VT_CLSID
	step, hr := "SetValue(AppUserModel.ID)", store.call(propertyStoreSetValue, uintptr(unsafe.Pointer(&pkeyAppUserModelID)), uintptr(unsafe.Pointer(&appIDVariant)))
	if !failed(hr) {
		step, hr = "SetValue(AppUserModel.ToastActivatorCLSID)", store.call(propertyStoreSetValue, uintptr(unsafe.Pointer(&pkeyAppUserModelToastActivator)), uintptr(unsafe.Pointer(&clsidVariant)))
	}
	if !failed(hr) {
		step, hr = "Commit", store.call(propertyStoreCommit)
	}
	if !failed(hr) {
		step, hr = "IPersistFile::Save", persist.call(persistFileSave, uintptr(unsafe.Pointer(file)), 1)
	}
	// The variants, and appID and clsid they point at, outlive the calls that read them.
	runtime.KeepAlive(&appIDVariant)
	runtime.KeepAlive(&clsidVariant)
	runtime.KeepAlive(appID)
	runtime.KeepAlive(&clsid)
	if failed(hr) {
		log.Printf("nsgo: toast shortcut %s: %s: %#x", path, step, uint32(hr))
	}
}
