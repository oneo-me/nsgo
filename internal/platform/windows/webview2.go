//go:build windows

package windows

// WebView2 through COM over syscall, and the loader's job done in Go.
// WebView2Loader.dll only finds the installed Evergreen runtime and calls
// its EmbeddedBrowserWebView.dll, so NSGo does the same and ships no DLL:
// the runtime's version is in EdgeUpdate's registry keys, and the client
// DLL's CreateWebViewEnvironmentWithOptionsInternal makes the environment.
// The vtable indexes below follow WebView2.idl (SDK 1.0.2903.40); each
// interface's methods come after IUnknown's three.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	ole32             = syscall.NewLazyDLL("ole32.dll")
	coInitializeEx    = ole32.NewProc("CoInitializeEx")
	coTaskMemFree     = ole32.NewProc("CoTaskMemFree")
	shlwapi           = syscall.NewLazyDLL("shlwapi.dll")
	shCreateMemStream = shlwapi.NewProc("SHCreateMemStream")

	setCursor             = user32.NewProc("SetCursor")
	isChild               = user32.NewProc("IsChild")
	getDoubleClickTime    = user32.NewProc("GetDoubleClickTime")
	getSystemMetrics      = user32.NewProc("GetSystemMetrics")
	getPointerType        = user32.NewProc("GetPointerType")
	getPointerInfo        = user32.NewProc("GetPointerInfo")
	getPointerTouchInfo   = user32.NewProc("GetPointerTouchInfo")
	getPointerPenInfo     = user32.NewProc("GetPointerPenInfo")
	getPointerDeviceRects = user32.NewProc("GetPointerDeviceRects")
)

var (
	iidUnknown                        = parseGUID("00000000-0000-0000-c000-000000000046")
	iidAgileObject                    = parseGUID("94ea2b94-e9cc-49e0-c0ff-ee64ca8f5b90")
	iidEnvironment                    = parseGUID("b96d755e-0319-4e92-a296-23436f46a1fc")
	iidEnvironment3                   = parseGUID("80a22ae3-be7c-4ce2-afe1-5a50056cdeeb")
	iidController                     = parseGUID("4d00c0d1-9434-4eb6-8078-8697a560334f")
	iidController2                    = parseGUID("c979903e-d4ca-4228-92eb-47ee3fa96eab")
	iidController3                    = parseGUID("f9614724-5d2b-41dc-aef7-73d62b51543b")
	iidSettings2                      = parseGUID("ee9a0f68-f46c-4e32-ac23-ef8cac224d2a")
	iidSettings6                      = parseGUID("11cb3acd-9bc8-43b8-83bf-f40753714f87")
	iidEnvironmentCompleted           = parseGUID("4e8a3389-c9d8-4bd2-b6b5-124fee6cc14d")
	iidCompositionControllerCompleted = parseGUID("02fab84b-1428-4fb7-ad45-1b2e64736184")
	iidNavigationStarting             = parseGUID("9adbe429-f36d-432b-9ddc-f8881fbd76e3")
	iidNavigationCompleted            = parseGUID("d33a35bf-1c49-4f98-93ab-006e0533fe1c")
	iidWebMessageReceived             = parseGUID("57213f19-00e6-49fa-8e07-898ea01ecbd2")
	iidWebResourceRequested           = parseGUID("ab00b74c-15f1-4646-80e8-e76341d25d71")
	iidWeb4                           = parseGUID("20d02d59-6df2-42dc-bd06-f98a694b1302")
	iidDownloadStarting               = parseGUID("efedc989-c396-41ca-83f7-07f845a55724")
	iidBytesReceivedChanged           = parseGUID("828e8ab6-d94c-4264-9cef-5217170d6251")
	iidDownloadStateChanged           = parseGUID("81336594-7ede-4ba9-bf71-acf0a95b58dd")
	iidCursorChanged                  = parseGUID("9da43ccc-26e1-4dad-b56c-d8961c94c571")
	iidFocusChanged                   = parseGUID("05ea24bd-6452-4926-9014-4b82b498135d")
	iidMoveFocusRequested             = parseGUID("69035451-6dc7-4cb8-9bce-b2bd70ad289f")
	iidExecuteScriptCompleted         = parseGUID("49511172-cc67-4bca-9923-137112f4c4cc")
	iidAddScriptCompleted             = parseGUID("b99369f3-9b11-47b5-bc6f-8e7895fcea17")
)

// Vtable indexes.
const (
	comQueryInterface = 0
	comAddRef         = 1
	comRelease        = 2

	// ICoreWebView2Environment, then ICoreWebView2Environment3
	envCreateWebResourceResponse   = 4
	envCreateCompositionController = 9
	envCreatePointerInfo           = 10

	// ICoreWebView2CompositionController
	compPutRootVisualTarget = 4
	compSendMouseInput      = 5
	compSendPointerInput    = 6
	compGetCursor           = 7
	compAddCursorChanged    = 9

	// ICoreWebView2Controller, then ICoreWebView2Controller2 and 3
	controllerPutIsVisible                       = 4
	controllerPutBounds                          = 6
	controllerMoveFocus                          = 12
	controllerAddMoveFocusRequested              = 13
	controllerAddGotFocus                        = 15
	controllerAddLostFocus                       = 17
	controllerNotifyParentPositionChanged        = 23
	controllerClose                              = 24
	controllerGetCoreWebView2                    = 25
	controllerPutDefaultBackgroundColor          = 27
	controllerPutRasterizationScale              = 29
	controllerPutShouldDetectMonitorScaleChanges = 31

	// ICoreWebView2
	webGetSettings                   = 3
	webGetSource                     = 4
	webNavigate                      = 5
	webNavigateToString              = 6
	webAddNavigationStarting         = 7
	webAddNavigationCompleted        = 15
	webAddScriptOnDocumentCreated    = 27
	webRemoveScriptOnDocumentCreated = 28
	webExecuteScript                 = 29
	webReload                        = 31
	webAddWebMessageReceived         = 34
	webGetCanGoBack                  = 38
	webGetCanGoForward               = 39
	webGoBack                        = 40
	webGoForward                     = 41
	webStop                          = 43
	webGetDocumentTitle              = 48
	webAddWebResourceRequested       = 55
	webAddWebResourceRequestedFilter = 57
	// ICoreWebView2_4 (after ICoreWebView2_2 and ICoreWebView2_3).
	webAddDownloadStarting = 75

	// ICoreWebView2DownloadStartingEventArgs.
	downloadArgsGetOperation  = 3
	downloadArgsPutCancel     = 5
	downloadArgsGetResultPath = 6
	downloadArgsPutResultPath = 7
	downloadArgsPutHandled    = 9

	// ICoreWebView2DownloadOperation.
	downloadAddBytesChanged = 3
	downloadAddStateChanged = 7
	downloadGetURI          = 9
	downloadGetBytes        = 13
	downloadGetState        = 16
	downloadGetInterrupt    = 17
	downloadCancel          = 18

	// ICoreWebView2Settings, then ICoreWebView2Settings2 and 6
	settingsPutIsScriptEnabled          = 4
	settingsPutIsStatusBarEnabled       = 10
	settingsGetUserAgent                = 21
	settingsPutUserAgent                = 22
	settingsPutIsSwipeNavigationEnabled = 32

	// ICoreWebView2NavigationStartingEventArgs
	navigationStartingGetUri    = 3
	navigationStartingPutCancel = 8

	// ICoreWebView2NavigationCompletedEventArgs
	navigationGetIsSuccess      = 3
	navigationGetWebErrorStatus = 4

	// ICoreWebView2WebMessageReceivedEventArgs
	messageGetJSON      = 4
	messageTryGetString = 5

	// ICoreWebView2MoveFocusRequestedEventArgs
	moveFocusPutHandled = 5

	// ICoreWebView2WebResourceRequestedEventArgs, ICoreWebView2WebResourceRequest
	resourceArgsGetRequest  = 3
	resourceArgsPutResponse = 5
	requestGetUri           = 3

	// ICoreWebView2PointerInfo
	pointerPutPointerKind         = 4
	pointerPutPointerId           = 6
	pointerPutFrameId             = 8
	pointerPutPointerFlags        = 10
	pointerPutPointerDeviceRect   = 12
	pointerPutDisplayRect         = 14
	pointerPutPixelLocation       = 16
	pointerPutHimetricLocation    = 18
	pointerPutPixelLocationRaw    = 20
	pointerPutHimetricLocationRaw = 22
	pointerPutTime                = 24
	pointerPutHistoryCount        = 26
	pointerPutInputData           = 28
	pointerPutKeyStates           = 30
	pointerPutPerformanceCount    = 32
	pointerPutButtonChangeKind    = 34
	pointerPutPenFlags            = 36
	pointerPutPenMask             = 38
	pointerPutPenPressure         = 40
	pointerPutPenRotation         = 42
	pointerPutPenTiltX            = 44
	pointerPutPenTiltY            = 46
	pointerPutTouchFlags          = 48
	pointerPutTouchMask           = 50
	pointerPutTouchContact        = 52
	pointerPutTouchContactRaw     = 54
	pointerPutTouchOrientation    = 56
	pointerPutTouchPressure       = 58
)

// COREWEBVIEW2_POINTER_EVENT_KIND
const (
	pointerEventUpdate = 581
	pointerEventDown   = 582
	pointerEventUp     = 583
	pointerEventLeave  = 586
)

// Pointer messages and types.
const (
	wmPointerUpdate = 0x0245
	wmPointerDown   = 0x0246
	wmPointerUp     = 0x0247
	wmPointerLeave  = 0x024A
	ptTouch         = 2
	ptPen           = 3

	smCXDoubleClk = 36
	smCYDoubleClk = 37
)

// pointerInfo is POINTER_INFO, padded to its size on every architecture.
type pointerInfo struct {
	pointerType, pointerID, frameID, pointerFlags uint32
	sourceDevice, hwndTarget                      uintptr
	pixelLocation, himetricLocation               point
	pixelLocationRaw, himetricLocationRaw         point
	time, historyCount                            uint32
	inputData                                     int32
	keyStates                                     uint32
	performanceCount                              uint64
	buttonChangeType                              uint32
	_                                             uint32
}

// pointerTouchInfo is POINTER_TOUCH_INFO.
type pointerTouchInfo struct {
	pointerInfo
	touchFlags, touchMask uint32
	contact, contactRaw   rect
	orientation, pressure uint32
}

// pointerPenInfo is POINTER_PEN_INFO.
type pointerPenInfo struct {
	pointerInfo
	penFlags, penMask, pressure, rotation uint32
	tiltX, tiltY                          int32
}

const (
	sOK             = 0
	eNoInterface    = 0x80004002
	rpcEChangedMode = 0x80010106
)

// getString calls a getter returning a string the caller frees.
func (o *comObject) getString(method int) string {
	var p *uint16
	if failed(o.call(method, uintptr(unsafe.Pointer(&p)))) {
		return ""
	}
	return takeString(p)
}

func (o *comObject) getBool(method int) bool {
	var b int32
	o.call(method, uintptr(unsafe.Pointer(&b)))
	return b != 0
}

// takeString copies and frees a string COM allocated.
func takeString(p *uint16) string {
	if p == nil {
		return ""
	}
	s := wideString(p)
	coTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	return s
}

// wideString copies a NUL-terminated UTF-16 string.
func wideString(p *uint16) string {
	if p == nil {
		return ""
	}
	n := 0
	for at := unsafe.Pointer(p); *(*uint16)(at) != 0; at = unsafe.Add(at, 2) {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

// comHandler is a COM callback object: IUnknown and one method. A
// WebView2 handler's Invoke takes two arguments, an HRESULT or sender and
// a result or event's arguments; a notify object's method takes three
// words. Go keeps it alive while COM holds a reference.
type comHandler struct {
	vtbl *[4]uintptr
	refs int32
	iid  *guid
	// agile says it may be called from any thread (IAgileObject), as a
	// WinRT event's handler raised on a worker thread is.
	agile  bool
	invoke func(a uintptr, b unsafe.Pointer)
	notify func(a, b, c uintptr)
}

var (
	handlerVtbl = [4]uintptr{
		syscall.NewCallback(handlerQueryInterface),
		syscall.NewCallback(handlerAddRef),
		syscall.NewCallback(handlerRelease),
		syscall.NewCallback(handlerInvoke),
	}
	// notifyVtbl is a callback object's whose method takes three words,
	// such as IMFMediaEngineNotify's EventNotify.
	notifyVtbl = [4]uintptr{
		handlerVtbl[0], handlerVtbl[1], handlerVtbl[2],
		syscall.NewCallback(handlerNotify),
	}
	handlersMu sync.Mutex
	handlers   = map[*comHandler]struct{}{}
)

// newHandler returns a handler of interface iid holding one reference,
// the caller's, which it gives up with release.
func newHandler(iid *guid, invoke func(a uintptr, b unsafe.Pointer)) *comHandler {
	h := &comHandler{vtbl: &handlerVtbl, refs: 1, iid: iid, invoke: invoke}
	handlersMu.Lock()
	handlers[h] = struct{}{}
	handlersMu.Unlock()
	return h
}

// newNotify returns a callback object of interface iid whose one method
// takes three words, holding the caller's reference. It may be called on
// any thread.
func newNotify(iid *guid, notify func(a, b, c uintptr)) *comHandler {
	h := &comHandler{vtbl: &notifyVtbl, refs: 1, iid: iid, notify: notify}
	handlersMu.Lock()
	handlers[h] = struct{}{}
	handlersMu.Unlock()
	return h
}

func (h *comHandler) ptr() uintptr { return uintptr(unsafe.Pointer(h)) }

func (h *comHandler) release() { handlerRelease(h) }

func handlerQueryInterface(h *comHandler, iid *guid, out *unsafe.Pointer) uintptr {
	if *iid == iidUnknown || *iid == *h.iid || h.agile && *iid == iidAgileObject {
		*out = unsafe.Pointer(h)
		handlerAddRef(h)
		return sOK
	}
	*out = nil
	return eNoInterface
}

func handlerAddRef(h *comHandler) uintptr {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	h.refs++
	return uintptr(h.refs)
}

func handlerRelease(h *comHandler) uintptr {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	if h.refs--; h.refs == 0 {
		delete(handlers, h)
	}
	return uintptr(h.refs)
}

func handlerNotify(h *comHandler, a, b, c uintptr) uintptr {
	h.notify(a, b, c)
	return sOK
}

func handlerInvoke(h *comHandler, a uintptr, b unsafe.Pointer) uintptr {
	if h.invoke != nil {
		h.invoke(a, b)
	}
	return sOK
}

// eventToken is an EventRegistrationToken.
type eventToken struct{ value int64 }

// subscribe registers handler h with add method, giving up the caller's
// reference: WebView2 keeps its own until the web view closes.
func (o *comObject) subscribe(method int, h *comHandler) {
	var token eventToken
	o.call(method, h.ptr(), uintptr(unsafe.Pointer(&token)))
	h.release()
}

var errNoWebView2 = errors.New("the WebView2 runtime is not installed")

// evergreenClient is the WebView2 Evergreen runtime's EdgeUpdate client.
const evergreenClient = `{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`

// findWebView2 returns the path of the installed Evergreen runtime's
// EmbeddedBrowserWebView.dll, looking where WebView2Loader does: the
// machine's install (32-bit registry view first, where EdgeUpdate
// writes), then the user's.
func findWebView2() (string, error) {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64", "386": "x86"}[runtime.GOARCH]
	if arch == "" {
		return "", errNoWebView2
	}
	roots := []struct {
		key  syscall.Handle
		view uint32
	}{
		{syscall.HKEY_LOCAL_MACHINE, syscall.KEY_WOW64_32KEY},
		{syscall.HKEY_LOCAL_MACHINE, syscall.KEY_WOW64_64KEY},
		{syscall.HKEY_CURRENT_USER, 0},
	}
	for _, root := range roots {
		var dirs []string
		client := `SOFTWARE\Microsoft\EdgeUpdate\Clients\` + evergreenClient
		pv := regString(root.key, root.view, client, "pv")
		if pv == "" || pv == "0.0.0.0" {
			continue
		}
		if location := regString(root.key, root.view, client, "location"); location != "" {
			dirs = append(dirs, filepath.Join(location, pv))
		}
		state := `SOFTWARE\Microsoft\EdgeUpdate\ClientState\` + evergreenClient
		if dir := regString(root.key, root.view, state, "EBWebView"); dir != "" {
			dirs = append(dirs, dir)
		}
		for _, dir := range dirs {
			dll := filepath.Join(dir, "EBWebView", arch, "EmbeddedBrowserWebView.dll")
			if _, err := os.Stat(dll); err == nil {
				return dll, nil
			}
		}
	}
	return "", errNoWebView2
}

// regString reads a string value, "" when there is none.
func regString(root syscall.Handle, view uint32, path, name string) string {
	var key syscall.Handle
	if syscall.RegOpenKeyEx(root, wide(path), 0, syscall.KEY_READ|view, &key) != nil {
		return ""
	}
	defer syscall.RegCloseKey(key)
	var kind, size uint32
	if syscall.RegQueryValueEx(key, wide(name), nil, &kind, nil, &size) != nil ||
		(kind != syscall.REG_SZ && kind != syscall.REG_EXPAND_SZ) || size < 2 {
		return ""
	}
	buf := make([]uint16, size/2)
	if syscall.RegQueryValueEx(key, wide(name), nil, &kind, (*byte)(unsafe.Pointer(&buf[0])), &size) != nil {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// webEnvironment is a WebView2 environment: one browser process for the
// web views of a profile folder, made on the main thread, which it
// belongs to.
type webEnvironment struct {
	env     *comObject
	err     error
	done    bool
	waiting []func(*comObject, error)
}

// webEnvironments are the UI threads', by thread and profile folder.
var (
	webEnvironmentsMu sync.Mutex
	webEnvironments   = map[webEnvironmentKey]*webEnvironment{}
)

type webEnvironmentKey struct {
	thread uintptr
	folder string
}

var getCurrentThreadID = kernel32.NewProc("GetCurrentThreadId")

// withWebEnvironment calls fn on the calling UI thread once its
// environment of profile folder is made, or cannot be; w posts the
// failure to start.
func withWebEnvironment(w *window, folder string, fn func(*comObject, error)) {
	thread, _, _ := getCurrentThreadID.Call()
	key := webEnvironmentKey{thread, folder}
	webEnvironmentsMu.Lock()
	e := webEnvironments[key]
	fresh := e == nil
	if fresh {
		e = &webEnvironment{}
		webEnvironments[key] = e
	}
	webEnvironmentsMu.Unlock()
	if fresh {
		e.waiting = append(e.waiting, fn)
		if err := startWebEnvironment(e, folder); err != nil {
			// Reported from a message of its own: the view's events may come
			// later in the batch being applied.
			w.post(func() { e.finish(nil, err) })
		}
		return
	}
	if e.done {
		fn(e.env, e.err)
		return
	}
	e.waiting = append(e.waiting, fn)
}

func (e *webEnvironment) finish(env *comObject, err error) {
	e.env, e.err, e.done = env, err, true
	waiting := e.waiting
	e.waiting = nil
	for _, fn := range waiting {
		fn(env, err)
	}
}

// initCOM makes the main thread a single-threaded apartment, as WebView2
// and Media Foundation's media engine need.
func initCOM() error {
	const coinitApartmentThreaded = 2
	if hr, _, _ := coInitializeEx.Call(0, coinitApartmentThreaded); hr == rpcEChangedMode {
		return errors.New("WebView2 needs a single-threaded COM apartment on the UI thread")
	}
	return nil
}

func startWebEnvironment(e *webEnvironment, folder string) error {
	if err := initCOM(); err != nil {
		return err
	}
	path, err := findWebView2()
	if err != nil {
		return err
	}
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return err
	}
	create, err := dll.FindProc("CreateWebViewEnvironmentWithOptionsInternal")
	if err != nil {
		return err
	}
	var h *comHandler
	h = newHandler(&iidEnvironmentCompleted, func(hr uintptr, result unsafe.Pointer) {
		defer h.release()
		env := (*comObject)(result)
		if failed(hr) || env == nil {
			e.finish(nil, hresultError("WebView2 could not start", hr))
			return
		}
		// As WebView2Loader does, keep the environment through its own
		// interface.
		if env = env.query(&iidEnvironment); env == nil {
			e.finish(nil, errors.New("WebView2 gave no environment"))
			return
		}
		e.finish(env, nil)
	})
	const checkRunningInstance, installedRuntime = 1, 0
	hr, _, _ := create.Call(checkRunningInstance, installedRuntime, uintptr(unsafe.Pointer(wide(folder))), 0, h.ptr())
	if failed(hr) {
		h.release()
		return hresultError("WebView2 could not start", hr)
	}
	return nil
}
