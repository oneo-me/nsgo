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

// Without X11 grabs, Linux global shortcuts use the XDG GlobalShortcuts portal. The portal is the compositor-owned path on Wayland and also works on desktops that expose it on X11; it asks the user before the shortcuts become active.
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
	gVariantTakeRef                  func(value uintptr) uintptr
	gDBusAddressGetForBusSync        func(busType int32, cancellable uintptr, err *uintptr) uintptr
	gDBusConnectionGetUniqueName     func(conn uintptr) uintptr
	gDBusConnectionSignalUnsubscribe func(conn uintptr, subscription uint32)
	bindGlobalHotKeys                = sync.OnceFunc(func() {
		bindNotifications()
		purego.RegisterLibFunc(&gVariantParse, glibLib, "g_variant_parse")
		purego.RegisterLibFunc(&gVariantNewObjectPath, glibLib, "g_variant_new_object_path")
		purego.RegisterLibFunc(&gVariantLookupValue, glibLib, "g_variant_lookup_value")
		purego.RegisterLibFunc(&gVariantTakeRef, glibLib, "g_variant_take_ref")
		purego.RegisterLibFunc(&gDBusAddressGetForBusSync, gioLib, "g_dbus_address_get_for_bus_sync")
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

// globalHotKeys is owned by the GTK main thread. Portal response and activation signals are dispatched on that same thread by GLib; the portal's methods are called on portalQueue's thread, which posts what follows back.
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

// portalQueue runs the portal's calls in order, off the main thread: the bus starts the portal as the first call reaches it, which can take seconds.
var portalQueue = sync.OnceValue(func() chan func() {
	ch := make(chan func(), 16)
	go func() {
		for fn := range ch {
			fn()
		}
	}()
	return ch
})

var (
	portalBusOnce sync.Once
	portalBusConn uintptr
)

// portalBus is a session bus connection of the shortcuts' own, 0 without one, on portalQueue's thread. The portal knows an unsandboxed app by the ID it registers (Registry.Register), and takes it only before the connection's first portal call; GTK calls the portal's Settings on the shared connection as it starts on Wayland, so the shortcuts use a connection that has made no other call.
func portalBus(appID string) uintptr {
	portalBusOnce.Do(func() {
		const busSession, authClient, messageBus = 2, 1 << 0, 1 << 3
		var err uintptr
		address := gDBusAddressGetForBusSync(busSession, 0, &err)
		if err != 0 {
			gErrorFree(err)
			return
		}
		bus := gDBusConnectionNewForAddressSync(cstr(goString(address)), authClient|messageBus, 0, 0, &err)
		gFree(address)
		if err != 0 {
			gErrorFree(err)
			return
		}
		// A sandboxed app, or a portal without the registry, refuses the registration; the portal then finds the app's ID itself, as it does for a Flatpak.
		if appID != "" {
			gVariantUnrefNonZero(portalCall(bus, globalShortcutPath, globalRegistryIface, "Register",
				gVariantNewString(cstr(appID)), portalVariant("a{sv}", "{}")))
		}
		portalBusConn = bus
	})
	return portalBusConn
}

// portalCall calls a method of the portal and returns its reply, 0 when it failed. It takes args, floating or not, and waits as long as GDBus does by default, as the portal may be starting.
func portalCall(bus uintptr, path, iface, method string, args ...uintptr) uintptr {
	ok := true
	for i, arg := range args {
		if arg == 0 {
			ok = false
			continue
		}
		args[i] = gVariantTakeRef(arg)
	}
	defer func() {
		for _, arg := range args {
			gVariantUnrefNonZero(arg)
		}
	}()
	if !ok {
		return 0
	}
	var params uintptr
	if len(args) > 0 {
		params = gVariantNewTuple(&args[0], uintptr(len(args)))
	} else {
		params = gVariantNewTuple(nil, 0)
	}
	var err uintptr
	reply := gDBusConnectionCallSync(bus, cstr(globalShortcutName), cstr(path), cstr(iface), cstr(method), params, 0, 0, -1, 0, &err)
	if err != 0 {
		gErrorFree(err)
		return 0
	}
	return reply
}

// applyHotKeys replaces the app's global shortcuts. A portal request may show a compositor-owned permission or configuration prompt; activation is reported later by the portal's Activated signal.
func (w *window) applyHotKeys(keys []driver.KeyBinding) {
	w.closeHotKeys()
	if len(keys) == 0 || w.engine == nil {
		return
	}
	if w.applyX11HotKeys(keys) {
		return
	}
	state := &globalHotKeys{ids: map[string]uint32{}}
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
	bindGlobalHotKeys()
	appID := w.appID
	portalQueue() <- func() {
		bus := portalBus(appID)
		if bus == 0 {
			return
		}
		w.post(func() {
			if w.hotKeys != state {
				return
			}
			state.bus = bus
			token := portalToken("nsgo_create")
			if !w.awaitHotKeyResponse(state, globalHotKeyCreating, token) {
				w.closeHotKeys()
				return
			}
			w.callHotKeyPortal(state, "CreateSession", func() []uintptr { return []uintptr{portalCreateOptions(token)} })
		})
	}
}

// awaitHotKeyResponse subscribes to the Response of the request token names before the call that makes it, so the subscription wins a fast reply.
func (w *window) awaitHotKeyResponse(state *globalHotKeys, stage globalHotKeyStage, token string) bool {
	state.stage, state.request = stage, portalRequestPath(state.bus, token)
	if state.request == "" {
		return false
	}
	state.requestSub = subscribeGlobalHotKeyResponse(state, w)
	return state.requestSub != 0
}

// callHotKeyPortal calls a GlobalShortcuts method for state on portalQueue's thread, with the arguments args makes there. A call that fails ends state's shortcuts, and a request at a path other than the one awaited is awaited instead.
func (w *window) callHotKeyPortal(state *globalHotKeys, method string, args func() []uintptr) {
	bus, awaited := state.bus, state.request
	portalQueue() <- func() {
		reply := portalCall(bus, globalShortcutPath, globalShortcutIface, method, args()...)
		request := ""
		if reply != 0 {
			request = portalTupleText(reply, 0)
			gVariantUnref(reply)
		}
		if request == awaited {
			return
		}
		w.post(func() {
			if w.hotKeys != state || state.request != awaited {
				return
			}
			if state.requestSub != 0 {
				gDBusConnectionSignalUnsubscribe(state.bus, state.requestSub)
				state.requestSub = 0
			}
			if request == "" {
				w.closeHotKeys()
				return
			}
			state.request = request
			if state.requestSub = subscribeGlobalHotKeyResponse(state, w); state.requestSub == 0 {
				w.closeHotKeys()
			}
		})
	}
}

func subscribeGlobalHotKeyResponse(state *globalHotKeys, w *window) uint32 {
	return gDBusConnectionSignalSubscribe(state.bus, cstr(globalShortcutName), cstr(globalRequestIface), cstr("Response"),
		cstr(state.request), nil, 0, cbGlobalHotKeyResponse, w.handle, 0)
}

func (w *window) closeHotKeys() {
	state := w.hotKeys
	if state == nil {
		return
	}
	w.hotKeys = nil
	if state.x11 != nil {
		closeX11HotKeys(state.x11)
		return
	}
	if state.requestSub != 0 {
		gDBusConnectionSignalUnsubscribe(state.bus, state.requestSub)
	}
	if state.activatedSub != 0 {
		gDBusConnectionSignalUnsubscribe(state.bus, state.activatedSub)
	}
	bus, path, iface := state.bus, state.session, globalSessionIface
	if path == "" {
		path, iface = state.request, globalRequestIface
	}
	if bus == 0 || path == "" {
		return
	}
	portalQueue() <- func() { gVariantUnrefNonZero(portalCall(bus, path, iface, "Close")) }
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
		if !w.awaitHotKeyResponse(state, globalHotKeyBinding, token) {
			w.closeHotKeys()
			return
		}
		specs := state.specs
		w.callHotKeyPortal(state, "BindShortcuts", func() []uintptr {
			return []uintptr{gVariantNewObjectPath(cstr(session)), portalShortcuts(specs), gVariantNewString(cstr("")), portalOptions(token)}
		})
	case globalHotKeyBinding:
		// The portal's successful response means the session is active. The Activated signal carries each shortcut ID from here on.
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

// portalRequestPath is the object path xdg-desktop-portal derives from the connection's unique name and the request's handle token. Knowing it before the method call lets the response subscription win a fast portal reply.
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

// portalVariant parses text as a value of typeName, 0 when it is not one. g_variant_parse returns a reference the caller owns, never a floating one, which portalCall takes.
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
