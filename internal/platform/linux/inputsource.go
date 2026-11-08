//go:build linux && !android

package linux

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
)

// Input sources are the input method framework's: Fcitx 5's input methods
// on the session bus ("keyboard-us", "pinyin"), else IBus's engines on its
// own bus ("xkb:us::eng", "libpinyin"). Both are reached over GDBus, off
// the main thread, one request at a time and in order.

var (
	gBusGetSync                      func(busType int32, cancellable uintptr, err *uintptr) uintptr
	gDBusConnectionNewForAddressSync func(address *byte, flags uint32, observer, cancellable uintptr, err *uintptr) uintptr
	gDBusConnectionCallSync          func(conn uintptr, name, path, iface, method *byte, params, replyType uintptr, flags uint32, timeout int32, cancellable uintptr, err *uintptr) uintptr
	gVariantNewTuple                 func(children *uintptr, n uintptr) uintptr
	gVariantGetChildValue            func(v uintptr, i uintptr) uintptr
	gVariantNChildren                func(v uintptr) uintptr
	gVariantGetString                func(v uintptr, length uintptr) uintptr
	gVariantGetVariant               func(v uintptr) uintptr
	gVariantGetTypeString            func(v uintptr) uintptr
	bindInputSources                 = sync.OnceFunc(func() {
		purego.RegisterLibFunc(&gBusGetSync, gioLib, "g_bus_get_sync")
		purego.RegisterLibFunc(&gDBusConnectionNewForAddressSync, gioLib, "g_dbus_connection_new_for_address_sync")
		purego.RegisterLibFunc(&gDBusConnectionCallSync, gioLib, "g_dbus_connection_call_sync")
		purego.RegisterLibFunc(&gVariantNewTuple, glibLib, "g_variant_new_tuple")
		purego.RegisterLibFunc(&gVariantGetChildValue, glibLib, "g_variant_get_child_value")
		purego.RegisterLibFunc(&gVariantNChildren, glibLib, "g_variant_n_children")
		purego.RegisterLibFunc(&gVariantGetString, glibLib, "g_variant_get_string")
		purego.RegisterLibFunc(&gVariantGetVariant, glibLib, "g_variant_get_variant")
		purego.RegisterLibFunc(&gVariantGetTypeString, glibLib, "g_variant_get_type_string")
	})
)

// inputSources takes the requests of setInputSource in order.
var inputSources = sync.OnceValue(func() chan string {
	ch := make(chan string, 16)
	go func() {
		var was, set string
		for id := range ch {
			im := inputMethods()
			if im == nil {
				continue
			}
			if id != "" {
				if current := im.current(); current != id && im.choose(id) {
					was, set = current, id
				}
				continue
			}
			if set != "" && im.current() == set {
				im.choose(was)
			}
			was, set = "", ""
		}
	}()
	return ch
})

// setInputSource selects the input source id, or with none goes back to
// the one selected before, unless the user chose another since.
func setInputSource(id string) { inputSources() <- id }

// inputMethod is a framework's current input source and a way to choose
// one, which reports whether it took.
type inputMethod struct {
	current func() string
	choose  func(id string) bool
}

// inputMethods finds the framework the session runs, nil for none.
func inputMethods() *inputMethod {
	bindInputSources()
	if bus := sessionBus(); bus != 0 {
		const name, path, iface = "org.fcitx.Fcitx5", "/controller", "org.fcitx.Fcitx.Controller1"
		current := func() string {
			reply := dbusCall(bus, name, path, iface, "CurrentInputMethod")
			defer gVariantUnrefNonZero(reply)
			return variantString(reply, 0)
		}
		if reply := dbusCall(bus, name, path, iface, "CurrentInputMethod"); reply != 0 {
			gVariantUnref(reply)
			return &inputMethod{current: current, choose: func(id string) bool {
				// Fcitx switches only to an input method of the current
				// group, which says whether it is enabled.
				gVariantUnrefNonZero(dbusCall(bus, name, path, iface, "SetCurrentIM", gVariantNewString(cstr(id))))
				return current() == id
			}}
		}
	}
	if bus := ibusBus(); bus != 0 {
		const name, path, iface = "org.freedesktop.IBus", "/org/freedesktop/IBus", "org.freedesktop.IBus"
		current := func() string {
			reply := dbusCall(bus, name, path, "org.freedesktop.DBus.Properties", "Get",
				gVariantNewString(cstr(iface)), gVariantNewString(cstr("GlobalEngine")))
			defer gVariantUnrefNonZero(reply)
			if reply == 0 {
				return ""
			}
			// (v) holding an IBusEngineDesc: (sa{sv}s...), its name third.
			desc := gVariantGetChildValue(reply, 0)
			defer gVariantUnref(desc)
			for strings.HasPrefix(goString(gVariantGetTypeString(desc)), "v") {
				inner := gVariantGetVariant(desc)
				gVariantUnref(desc)
				desc = inner
			}
			if gVariantNChildren(desc) < 3 {
				return ""
			}
			return variantString(desc, 2)
		}
		return &inputMethod{current: current, choose: func(id string) bool {
			reply := dbusCall(bus, name, path, iface, "SetGlobalEngine", gVariantNewString(cstr(id)))
			gVariantUnrefNonZero(reply)
			return reply != 0
		}}
	}
	return nil
}

// sessionBus is the session's message bus, 0 without one.
var sessionBus = sync.OnceValue(func() uintptr {
	const busSession = 2
	var err uintptr
	bus := gBusGetSync(busSession, 0, &err)
	if err != 0 {
		gErrorFree(err)
	}
	return bus
})

// ibusBus is IBus's own bus, at the address its daemon wrote for this
// display, 0 when it runs none.
var ibusBus = sync.OnceValue(func() uintptr {
	address := os.Getenv("IBUS_ADDRESS")
	if address == "" {
		address = ibusAddress()
	}
	if address == "" {
		return 0
	}
	const authClient, messageBus = 1 << 0, 1 << 3
	var err uintptr
	bus := gDBusConnectionNewForAddressSync(cstr(address), authClient|messageBus, 0, 0, &err)
	if err != 0 {
		gErrorFree(err)
		return 0
	}
	return bus
})

// ibusAddress reads the address from the file IBus names after the
// machine and the display: <machine-id>-unix-<display>, the display's
// number on X11 and its name on Wayland.
func ibusAddress() string {
	id, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		if id, err = os.ReadFile("/var/lib/dbus/machine-id"); err != nil {
			return ""
		}
	}
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		display = os.Getenv("DISPLAY")
		if _, after, ok := strings.Cut(display, ":"); ok {
			display, _, _ = strings.Cut(after, ".")
		}
	}
	if display == "" {
		return ""
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		home, _ := os.UserHomeDir()
		config = filepath.Join(home, ".config")
	}
	f, err := os.Open(filepath.Join(config, "ibus", "bus", strings.TrimSpace(string(id))+"-unix-"+display))
	if err != nil {
		return ""
	}
	defer f.Close()
	for s := bufio.NewScanner(f); s.Scan(); {
		if address, ok := strings.CutPrefix(s.Text(), "IBUS_ADDRESS="); ok {
			return address
		}
	}
	return ""
}

// dbusCall calls a method with string arguments, which it takes, and
// returns its reply, 0 when it failed.
func dbusCall(bus uintptr, name, path, iface, method string, args ...uintptr) uintptr {
	var params uintptr
	if len(args) > 0 {
		params = gVariantNewTuple(&args[0], uintptr(len(args)))
	} else {
		params = gVariantNewTuple(nil, 0)
	}
	var err uintptr
	reply := gDBusConnectionCallSync(bus, cstr(name), cstr(path), cstr(iface), cstr(method), params, 0, 0, 1000, 0, &err)
	if err != 0 {
		gErrorFree(err)
		return 0
	}
	return reply
}

// variantString is the string child i of a tuple, "" without one.
func variantString(tuple uintptr, i int) string {
	if tuple == 0 {
		return ""
	}
	child := gVariantGetChildValue(tuple, uintptr(i))
	defer gVariantUnref(child)
	if t := goString(gVariantGetTypeString(child)); t != "s" {
		return ""
	}
	return goString(gVariantGetString(child, 0))
}

func gVariantUnrefNonZero(v uintptr) {
	if v != 0 {
		gVariantUnref(v)
	}
}
