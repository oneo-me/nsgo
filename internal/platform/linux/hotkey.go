//go:build linux && !android

package linux

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var portalTokenID atomic.Uint64

var (
	gdkX11DisplayGetXDisplay   func(display uintptr) uintptr
	gdkX11DisplayErrorTrapPush func(display uintptr)
	gdkX11DisplayErrorTrapPop  func(display uintptr) int32
	x11DefaultRootWindow       func(display uintptr) uintptr
	x11StringToKeysym          func(name *byte) uintptr
	x11KeysymToKeycode         func(display, keysym uintptr) uint8
	x11GrabKey                 func(display uintptr, keycode int32, modifiers uint32, root uintptr, ownerEvents, pointerMode, keyboardMode int32) int32
	x11UngrabKey               func(display uintptr, keycode int32, modifiers uint32, root uintptr) int32
	x11HotKeysReady            bool
	bindX11HotKeys             = sync.OnceFunc(func() {
		if os.Getenv("WAYLAND_DISPLAY") != "" || strings.Contains(strings.ToLower(os.Getenv("GDK_BACKEND")), "wayland") || os.Getenv("DISPLAY") == "" {
			return
		}
		for _, symbol := range []string{"gdk_x11_display_get_xdisplay", "gdk_x11_display_error_trap_push", "gdk_x11_display_error_trap_pop"} {
			if _, err := purego.Dlsym(gtkLib, symbol); err != nil {
				return
			}
		}
		lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			if lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
				return
			}
		}
		for _, symbol := range []string{"XDefaultRootWindow", "XStringToKeysym", "XKeysymToKeycode", "XGrabKey", "XUngrabKey"} {
			if _, err := purego.Dlsym(lib, symbol); err != nil {
				return
			}
		}
		purego.RegisterLibFunc(&gdkX11DisplayGetXDisplay, gtkLib, "gdk_x11_display_get_xdisplay")
		purego.RegisterLibFunc(&gdkX11DisplayErrorTrapPush, gtkLib, "gdk_x11_display_error_trap_push")
		purego.RegisterLibFunc(&gdkX11DisplayErrorTrapPop, gtkLib, "gdk_x11_display_error_trap_pop")
		purego.RegisterLibFunc(&x11DefaultRootWindow, lib, "XDefaultRootWindow")
		purego.RegisterLibFunc(&x11StringToKeysym, lib, "XStringToKeysym")
		purego.RegisterLibFunc(&x11KeysymToKeycode, lib, "XKeysymToKeycode")
		purego.RegisterLibFunc(&x11GrabKey, lib, "XGrabKey")
		purego.RegisterLibFunc(&x11UngrabKey, lib, "XUngrabKey")
		x11HotKeysReady = true
	})
)

// Linux global shortcuts use the XDG GlobalShortcuts portal. The portal is
// the compositor-owned path on Wayland and also works on desktops that expose
// it on X11; it asks the user before the shortcuts become active.
const (
	globalShortcutName  = "org.freedesktop.portal.Desktop"
	globalShortcutPath  = "/org/freedesktop/portal/desktop"
	globalShortcutIface = "org.freedesktop.portal.GlobalShortcuts"
	globalRegistryIface = "org.freedesktop.host.portal.Registry"
	globalRequestIface  = "org.freedesktop.portal.Request"
	globalSessionIface  = "org.freedesktop.portal.Session"
)

var (
	gVariantParse                    func(typ uintptr, text, limit, end *byte, err *uintptr) uintptr
	gVariantNewObjectPath            func(path *byte) uintptr
	gVariantLookupValue              func(value uintptr, key *byte, expectedType uintptr) uintptr
	gDBusConnectionGetUniqueName     func(conn uintptr) uintptr
	gDBusConnectionSignalUnsubscribe func(conn uintptr, subscription uint32)
	bindGlobalHotKeys                = sync.OnceFunc(func() {
		bindNotifications()
		purego.RegisterLibFunc(&gVariantParse, glibLib, "g_variant_parse")
		purego.RegisterLibFunc(&gVariantNewObjectPath, glibLib, "g_variant_new_object_path")
		purego.RegisterLibFunc(&gVariantLookupValue, glibLib, "g_variant_lookup_value")
		purego.RegisterLibFunc(&gDBusConnectionGetUniqueName, gioLib, "g_dbus_connection_get_unique_name")
		purego.RegisterLibFunc(&gDBusConnectionSignalUnsubscribe, gioLib, "g_dbus_connection_signal_unsubscribe")
	})
)

type globalHotKeyStage uint8

const (
	globalHotKeyCreating globalHotKeyStage = iota + 1
	globalHotKeyBinding
)

type globalHotKeySpec struct {
	id      string
	key     uint32
	trigger string
}

type x11Grab struct {
	keycode   uint8
	modifiers uint32
}

type x11HotKey struct {
	keycode   uint8
	modifiers uint32
}

type x11HotKeys struct {
	gdk      uintptr // the GdkDisplay
	display  uintptr // its Xlib Display
	root     uintptr
	handler  uint64 // the xevent handler's ID
	grabs    []x11Grab
	bindings map[x11HotKey]uint32
}

// globalHotKeys is owned by the GTK main thread. Portal response and
// activation signals are dispatched on that same thread by GLib.
type globalHotKeys struct {
	bus          uintptr
	session      string
	request      string
	stage        globalHotKeyStage
	requestSub   uint32
	activatedSub uint32
	ids          map[string]uint32
	specs        []globalHotKeySpec
	x11          *x11HotKeys
}

const (
	x11KeyPress      = 2
	x11GrabModeAsync = 1
	x11ShiftMask     = 1 << 0
	x11LockMask      = 1 << 1
	x11ControlMask   = 1 << 2
	x11Mod1Mask      = 1 << 3
	x11Mod2Mask      = 1 << 4 // NumLock on the standard XKB map
	x11Mod4Mask      = 1 << 6 // Super/Logo on the standard XKB map
	// x11HotKeyMask is what a combination is matched on: a key event's state also carries Lock and NumLock, the pointer's buttons, Mod3 and Mod5, and XKB's layout group in bits 13 and 14, none of which a passive grab tells apart.
	x11HotKeyMask = x11ShiftMask | x11ControlMask | x11Mod1Mask | x11Mod4Mask
)

// applyX11HotKeys uses passive grabs on the X11 root window. GTK 4 has no display filters; its GdkX11Display emits xevent for every X event before translating it, and the handler takes the grabbed key presses while the focused app keeps its normal event stream for every other key.
func (w *window) applyX11HotKeys(keys []driver.KeyBinding) bool {
	bindX11HotKeys()
	if !x11HotKeysReady {
		return false
	}
	gdk := gdkDisplayGetDefault()
	if gdk == 0 {
		return false
	}
	display := gdkX11DisplayGetXDisplay(gdk)
	if display == 0 {
		return false
	}
	x := &x11HotKeys{gdk: gdk, display: display, root: x11DefaultRootWindow(display), bindings: map[x11HotKey]uint32{}}
	for _, key := range keys {
		trigger, ok := portalTrigger(key.Key)
		if !ok {
			continue
		}
		parts := strings.Split(trigger, "+")
		name := parts[len(parts)-1]
		keysym := x11StringToKeysym(cstr(name))
		if keysym == 0 {
			continue
		}
		keycode := x11KeysymToKeycode(display, keysym)
		if keycode == 0 {
			continue
		}
		mods, ok := x11TriggerModifiers(parts[:len(parts)-1])
		if !ok {
			continue
		}
		combo := x11HotKey{keycode: keycode, modifiers: mods}
		if _, exists := x.bindings[combo]; exists {
			continue
		}
		grabs := make([]x11Grab, 0, 4)
		for _, extra := range []uint32{0, x11LockMask, x11Mod2Mask, x11LockMask | x11Mod2Mask} {
			grabs = append(grabs, x11Grab{keycode: keycode, modifiers: mods | extra})
		}
		// A combination another client grabbed, such as the window manager's, fails with an asynchronous BadAccess, which GTK ends the process for outside an error trap. Popping the trap waits for the server's answer. Without owner events every grabbed press goes to the root window as a core event, even while the app's own window, whose keys GTK takes through XInput 2, has the focus.
		gdkX11DisplayErrorTrapPush(gdk)
		for _, grab := range grabs {
			x11GrabKey(display, int32(grab.keycode), grab.modifiers, x.root, 0, x11GrabModeAsync, x11GrabModeAsync)
		}
		if gdkX11DisplayErrorTrapPop(gdk) != 0 {
			x.ungrab(grabs)
			continue
		}
		x.grabs = append(x.grabs, grabs...)
		x.bindings[combo] = key.ID
	}
	if len(x.grabs) == 0 {
		return false
	}
	x.handler = gSignalConnect(gdk, "xevent", cbX11HotKeyEvent, w.handle)
	w.hotKeys = &globalHotKeys{x11: x}
	return true
}

// ungrab releases grabs, those that failed included: XUngrabKey leaves other clients' grabs alone.
func (x *x11HotKeys) ungrab(grabs []x11Grab) {
	gdkX11DisplayErrorTrapPush(x.gdk)
	for _, grab := range grabs {
		x11UngrabKey(x.display, int32(grab.keycode), grab.modifiers, x.root)
	}
	_ = gdkX11DisplayErrorTrapPop(x.gdk)
}

func closeX11HotKeys(x *x11HotKeys) {
	if x.handler != 0 {
		gSignalHandlerDisconnect(x.gdk, x.handler)
	}
	x.ungrab(x.grabs)
}

func x11TriggerModifiers(parts []string) (uint32, bool) {
	var mods uint32
	for _, part := range parts {
		switch part {
		case "CTRL":
			mods |= x11ControlMask
		case "ALT":
			mods |= x11Mod1Mask
		case "SHIFT":
			mods |= x11ShiftMask
		case "LOGO":
			mods |= x11Mod4Mask
		default:
			return 0, false
		}
	}
	return mods, true
}

// x11HotKeyEvent is GdkX11Display's xevent handler, gboolean (*)(GdkX11Display *, const XEvent *, gpointer): TRUE takes the event from GTK.
func x11HotKeyEvent(display, xevent, data uintptr) int32 {
	if xevent == 0 {
		return 0
	}
	w := windowOf(data)
	if w == nil || w.closing || w.engine == nil || w.hotKeys == nil || w.hotKeys.x11 == nil {
		return 0
	}
	id, ok := w.hotKeys.x11.match(cPointer(xevent))
	if !ok {
		return 0
	}
	w.engine.Dispatch(driver.Event{Kind: driver.EventHotKey, Count: id})
	return 1
}

// match returns the shortcut a key press, an XKeyEvent, is: its state at 80 and its keycode at 84 on LP64.
func (x *x11HotKeys) match(ev unsafe.Pointer) (uint32, bool) {
	if *(*int32)(ev) != x11KeyPress {
		return 0, false
	}
	keycode := *(*uint32)(unsafe.Add(ev, 84))
	if keycode > 255 {
		return 0, false
	}
	state := *(*uint32)(unsafe.Add(ev, 80)) & x11HotKeyMask
	id, ok := x.bindings[x11HotKey{keycode: uint8(keycode), modifiers: state}]
	return id, ok
}

// applyHotKeys replaces the app's global shortcuts. A portal request may
// show a compositor-owned permission or configuration prompt; activation is
// reported later by the portal's Activated signal.
func (w *window) applyHotKeys(keys []driver.KeyBinding) {
	w.closeHotKeys()
	if len(keys) == 0 || w.engine == nil {
		return
	}
	if w.applyX11HotKeys(keys) {
		return
	}
	bindGlobalHotKeys()
	bus := sessionBus()
	if bus == 0 {
		return
	}
	// Recent xdg-desktop-portal versions require an application ID before
	// creating a global-shortcuts session. Sandboxed apps are already known
	// to the portal, so an unavailable registry or a rejected registration
	// does not prevent the normal session attempt below.
	registerPortalApp(bus, w.appID)

	state := &globalHotKeys{bus: bus, ids: map[string]uint32{}}
	for _, key := range keys {
		trigger, ok := portalTrigger(key.Key)
		if !ok {
			continue
		}
		id := strconv.FormatUint(uint64(key.ID), 10)
		state.specs = append(state.specs, globalHotKeySpec{id: id, key: key.ID, trigger: trigger})
		state.ids[id] = key.ID
	}
	if len(state.specs) == 0 {
		return
	}
	w.hotKeys = state

	token := portalToken("nsgo_create")
	state.stage, state.request = globalHotKeyCreating, portalRequestPath(bus, token)
	if state.request != "" {
		state.requestSub = subscribeGlobalHotKeyResponse(state, w)
		if state.requestSub == 0 {
			w.closeHotKeys()
			return
		}
	}
	request := dbusCall(bus, globalShortcutName, globalShortcutPath, globalShortcutIface, "CreateSession",
		portalCreateOptions(token))
	if request == 0 {
		w.closeHotKeys()
		return
	}
	requestPath := portalTupleText(request, 0)
	gVariantUnref(request)
	if requestPath == "" {
		w.closeHotKeys()
		return
	}
	if state.request == "" {
		state.stage, state.request = globalHotKeyCreating, requestPath
		state.requestSub = subscribeGlobalHotKeyResponse(state, w)
		if state.requestSub == 0 {
			w.closeHotKeys()
		}
	}
}

func subscribeGlobalHotKeyResponse(state *globalHotKeys, w *window) uint32 {
	return gDBusConnectionSignalSubscribe(state.bus, cstr(globalShortcutName), cstr(globalRequestIface), cstr("Response"),
		cstr(state.request), nil, 0, cbGlobalHotKeyResponse, w.handle, 0)
}

func registerPortalApp(bus uintptr, appID string) {
	if appID == "" {
		return
	}
	options := portalVariant("a{sv}", "{}")
	if options == 0 {
		return
	}
	gVariantUnrefNonZero(dbusCall(bus, globalShortcutName, globalShortcutPath, globalRegistryIface, "Register",
		gVariantNewString(cstr(appID)), options))
}

func (w *window) closeHotKeys() {
	state := w.hotKeys
	if state == nil {
		return
	}
	w.hotKeys = nil
	if state.requestSub != 0 {
		gDBusConnectionSignalUnsubscribe(state.bus, state.requestSub)
	}
	if state.activatedSub != 0 {
		gDBusConnectionSignalUnsubscribe(state.bus, state.activatedSub)
	}
	if state.x11 != nil {
		closeX11HotKeys(state.x11)
	}
	if state.session != "" {
		gVariantUnrefNonZero(dbusCall(state.bus, globalShortcutName, state.session, globalSessionIface, "Close"))
	} else if state.request != "" {
		gVariantUnrefNonZero(dbusCall(state.bus, globalShortcutName, state.request, globalRequestIface, "Close"))
	}
}

func globalHotKeyResponse(conn, sender, path, iface, signal, params, data uintptr) {
	w := windowOf(data)
	state := func() *globalHotKeys {
		if w == nil || w.hotKeys == nil || goString(path) != w.hotKeys.request {
			return nil
		}
		return w.hotKeys
	}()
	if state == nil {
		return
	}
	if state.requestSub != 0 {
		gDBusConnectionSignalUnsubscribe(state.bus, state.requestSub)
		state.requestSub = 0
	}
	if gVariantNChildren(params) < 2 || portalTupleUint32(params, 0) != 0 {
		w.closeHotKeys()
		return
	}
	results := gVariantGetChildValue(params, 1)
	defer gVariantUnref(results)
	switch state.stage {
	case globalHotKeyCreating:
		session := portalLookupText(results, "session_handle")
		if session == "" {
			w.closeHotKeys()
			return
		}
		state.session = session
		// The portal emits Activated on its own object, naming the session in the first argument.
		state.activatedSub = gDBusConnectionSignalSubscribe(state.bus, cstr(globalShortcutName), cstr(globalShortcutIface), cstr("Activated"),
			cstr(globalShortcutPath), nil, 0, cbGlobalHotKeyActivated, w.handle, 0)
		if state.activatedSub == 0 {
			w.closeHotKeys()
			return
		}
		token := portalToken("nsgo_bind")
		state.stage, state.request = globalHotKeyBinding, portalRequestPath(state.bus, token)
		if state.request != "" {
			state.requestSub = subscribeGlobalHotKeyResponse(state, w)
			if state.requestSub == 0 {
				w.closeHotKeys()
				return
			}
		}
		request := dbusCall(state.bus, globalShortcutName, globalShortcutPath, globalShortcutIface, "BindShortcuts",
			gVariantNewObjectPath(cstr(session)), portalShortcuts(state.specs), gVariantNewString(cstr("")),
			portalOptions(token))
		if request == 0 {
			w.closeHotKeys()
			return
		}
		requestPath := portalTupleText(request, 0)
		gVariantUnref(request)
		if requestPath == "" {
			w.closeHotKeys()
			return
		}
		if state.request == "" {
			state.stage, state.request = globalHotKeyBinding, requestPath
			state.requestSub = subscribeGlobalHotKeyResponse(state, w)
			if state.requestSub == 0 {
				w.closeHotKeys()
			}
		}
	case globalHotKeyBinding:
		// The portal's successful response means the session is active. The
		// Activated signal carries each shortcut ID from here on.
	}
}

// globalHotKeyActivated hears the portal's Activated(o session_handle, s shortcut_id, t timestamp, a{sv} options).
func globalHotKeyActivated(conn, sender, path, iface, signal, params, data uintptr) {
	w := windowOf(data)
	if w == nil || w.closing || w.engine == nil || w.hotKeys == nil || w.hotKeys.session == "" || gVariantNChildren(params) < 2 {
		return
	}
	if portalTupleText(params, 0) != w.hotKeys.session {
		return
	}
	id := portalTupleText(params, 1)
	if key, ok := w.hotKeys.ids[id]; ok {
		w.engine.Dispatch(driver.Event{Kind: driver.EventHotKey, Count: key})
	}
}

func portalOptions(token string) uintptr {
	return portalVariant("a{sv}", "{'handle_token': <"+portalQuote(token)+">}")
}

func portalCreateOptions(token string) uintptr {
	return portalVariant("a{sv}", "{'handle_token': <"+portalQuote(token)+">, 'session_handle_token': <"+portalQuote(token)+">}")
}

func portalToken(prefix string) string {
	return prefix + "_" + strconv.FormatUint(portalTokenID.Add(1), 10)
}

// portalRequestPath is the object path xdg-desktop-portal derives from the
// connection's unique name and the request's handle token. Knowing it before
// the method call lets the response subscription win a fast portal reply.
func portalRequestPath(bus uintptr, token string) string {
	name := goString(gDBusConnectionGetUniqueName(bus))
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(strings.TrimPrefix(name, ":"), ".", "_")
	return globalShortcutPath + "/request/" + name + "/" + token
}

func portalShortcuts(specs []globalHotKeySpec) uintptr {
	var b strings.Builder
	b.WriteByte('[')
	for i, spec := range specs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("(")
		b.WriteString(portalQuote(spec.id))
		b.WriteString(", {'description': <'NSGo global shortcut'>, 'preferred_trigger': <")
		b.WriteString(portalQuote(spec.trigger))
		b.WriteString(">})")
	}
	b.WriteByte(']')
	return portalVariant("a(sa{sv})", b.String())
}

func portalVariant(typeName, text string) uintptr {
	typ := gVariantTypeNew(cstr(typeName))
	defer gVariantTypeFree(typ)
	var err uintptr
	value := gVariantParse(typ, cstr(text), nil, nil, &err)
	if err != 0 {
		gErrorFree(err)
		return 0
	}
	return value
}

func portalQuote(s string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r").Replace(s) + "'"
}

func portalTupleUint32(tuple uintptr, index int) uint32 {
	value := gVariantGetChildValue(tuple, uintptr(index))
	defer gVariantUnref(value)
	return gVariantGetUint32(value)
}

func portalTupleText(tuple uintptr, index int) string {
	value := gVariantGetChildValue(tuple, uintptr(index))
	defer gVariantUnref(value)
	return portalValueText(value)
}

func portalLookupText(dict uintptr, key string) string {
	value := gVariantLookupValue(dict, cstr(key), 0)
	if value == 0 {
		return ""
	}
	defer gVariantUnref(value)
	return portalValueText(value)
}

func portalValueText(value uintptr) string {
	original := value
	for value != 0 && strings.HasPrefix(goString(gVariantGetTypeString(value)), "v") {
		inner := gVariantGetVariant(value)
		if value != original {
			gVariantUnref(value)
		}
		value = inner
	}
	if value == 0 {
		return ""
	}
	typ := goString(gVariantGetTypeString(value))
	if typ != "s" && typ != "o" {
		if value != original {
			gVariantUnref(value)
		}
		return ""
	}
	text := goString(gVariantGetString(value, 0))
	if value != original {
		gVariantUnref(value)
	}
	return text
}

// portalTrigger converts a normalized NSGo shortcut into the XDG shortcuts
// specification used by GlobalShortcuts. "mod" means Control on Linux.
func portalTrigger(key string) (string, bool) {
	parts := strings.Split(key, "+")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return "", false
	}
	mods := make([]string, 0, len(parts)-1)
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "mod", "ctrl":
			mods = append(mods, "CTRL")
		case "alt":
			mods = append(mods, "ALT")
		case "shift":
			mods = append(mods, "SHIFT")
		case "meta":
			mods = append(mods, "LOGO")
		default:
			return "", false
		}
	}
	name, ok := portalKeyName(parts[len(parts)-1])
	if !ok {
		return "", false
	}
	return strings.Join(append(mods, name), "+"), true
}

func portalKeyName(name string) (string, bool) {
	if len(name) == 1 {
		c := name[0]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			return string(c), true
		}
	}
	if strings.HasPrefix(name, "f") {
		if n, err := strconv.Atoi(name[1:]); err == nil && n >= 1 && n <= 35 {
			return "F" + strconv.Itoa(n), true
		}
	}
	if key, ok := portalSpecialKeys[name]; ok {
		return key, true
	}
	return "", false
}

var portalSpecialKeys = map[string]string{
	"backspace": "BackSpace", "tab": "Tab", "enter": "Return", "escape": "Escape", "space": "space",
	"insert": "Insert", "delete": "Delete", "home": "Home", "end": "End", "pageup": "Page_Up", "pagedown": "Page_Down",
	"left": "Left", "up": "Up", "right": "Right", "down": "Down",
	";": "semicolon", "=": "equal", ",": "comma", "-": "minus", "minus": "minus", ".": "period", "/": "slash",
	"`": "grave", "[": "bracketleft", "\\": "backslash", "]": "bracketright", "'": "apostrophe",
	"+": "plus", "plus": "plus",
}
