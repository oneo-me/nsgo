//go:build linux && !android

package linux

import (
	"strings"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// System notifications through the desktop's notification server,
// org.freedesktop.Notifications on the session bus, which GNOME, KDE and
// the other desktops run: a notification of an app's ID replaces the
// server's one of that ID, and its default action is its press. The
// server needs no permission, so the app may show them where one runs.
// A Unity-compatible launcher gets a count through LauncherEntry.Update;
// desktops without that interface simply ignore the badge.

var (
	gVariantNewUint32              func(v uint32) uintptr
	gVariantNewInt32               func(v int32) uintptr
	gVariantNewArray               func(childType uintptr, children *uintptr, n uintptr) uintptr
	gVariantNewDictEntry           func(key, value uintptr) uintptr
	gVariantNewVariant             func(v uintptr) uintptr
	gVariantGetUint32              func(v uintptr) uint32
	gVariantTypeNew                func(t *byte) uintptr
	gVariantTypeFree               func(t uintptr)
	gDBusConnectionSignalSubscribe func(conn uintptr, sender, iface, member, path, arg0 *byte, flags uint32,
		callback, data, destroy uintptr) uint32
	bindNotifications = sync.OnceFunc(func() {
		bindInputSources()
		purego.RegisterLibFunc(&gVariantNewUint32, glibLib, "g_variant_new_uint32")
		purego.RegisterLibFunc(&gVariantNewInt32, glibLib, "g_variant_new_int32")
		purego.RegisterLibFunc(&gVariantNewArray, glibLib, "g_variant_new_array")
		purego.RegisterLibFunc(&gVariantNewDictEntry, glibLib, "g_variant_new_dict_entry")
		purego.RegisterLibFunc(&gVariantNewVariant, glibLib, "g_variant_new_variant")
		purego.RegisterLibFunc(&gVariantGetUint32, glibLib, "g_variant_get_uint32")
		purego.RegisterLibFunc(&gVariantTypeNew, glibLib, "g_variant_type_new")
		purego.RegisterLibFunc(&gVariantTypeFree, glibLib, "g_variant_type_free")
		purego.RegisterLibFunc(&gDBusConnectionSignalSubscribe, gioLib, "g_dbus_connection_signal_subscribe")
	})
)

const (
	notifyName  = "org.freedesktop.Notifications"
	notifyPath  = "/org/freedesktop/Notifications"
	notifyIface = "org.freedesktop.Notifications"
)

// notices maps the app's notification IDs to the server's and back, and
// keeps the window whose engine hears the presses: the first that showed
// one. The server's calls run off the main thread, its signals on it.
var notices struct {
	mu         sync.Mutex
	server     map[string]uint32
	app        map[uint32]string
	window     *window
	subscribed bool
}

// notify carries out a notification's item: the calls go to the server
// off the main thread, one item at a time and in order.
func (w *window) notify(it *driver.MountItem) {
	bindNotifications()
	notices.mu.Lock()
	if notices.window == nil || notices.window.closing {
		notices.window = w
	}
	if !notices.subscribed {
		notices.subscribed = true
		notices.server, notices.app = map[string]uint32{}, map[uint32]string{}
		if bus := sessionBus(); bus != 0 {
			gDBusConnectionSignalSubscribe(bus, cstr(notifyName), cstr(notifyIface), cstr("ActionInvoked"),
				cstr(notifyPath), nil, 0, cbNotificationAction, 0, 0)
			gDBusConnectionSignalSubscribe(bus, cstr(notifyName), cstr(notifyIface), cstr("NotificationClosed"),
				cstr(notifyPath), nil, 0, cbNotificationClosed, 0, 0)
		}
	}
	notices.mu.Unlock()
	item, e, appID := *it, w.engine, w.opts.ID
	notifyQueue() <- func() {
		bus := sessionBus()
		switch item.Kind {
		case driver.ItemNotificationPermission:
			p := driver.NotificationsUnavailable
			if bus != 0 {
				if reply := dbusCall(bus, notifyName, notifyPath, notifyIface, "GetServerInformation"); reply != 0 {
					gVariantUnref(reply)
					p = driver.NotificationsGranted
				}
			}
			e.Dispatch(driver.Event{Kind: driver.EventNotification, Count: item.Count, Index: p})
		case driver.ItemShowNotification:
			shown := bus != 0 && showNotice(bus, appID, *item.Notification)
			e.Dispatch(driver.Event{Kind: driver.EventNotification, Count: item.Count, Finished: shown})
		case driver.ItemCloseNotification:
			notices.mu.Lock()
			id, ok := notices.server[item.Value]
			notices.mu.Unlock()
			if ok && bus != 0 {
				gVariantUnrefNonZero(dbusCall(bus, notifyName, notifyPath, notifyIface, "CloseNotification", gVariantNewUint32(id)))
			}
		case driver.ItemBadge:
			setLauncherBadge(bus, appID, item.Count)
		}
	}
}

// setLauncherBadge updates the optional Unity launcher badge. The call is
// intentionally best effort: GNOME, KDE and other launchers do not expose
// this interface, and SetBadge remains a no-op there.
func setLauncherBadge(bus uintptr, appID string, count uint32) {
	if bus == 0 || appID == "" {
		return
	}
	pathID := strings.NewReplacer("/", "_", "-", "_").Replace(appID)
	path := "/com/canonical/Unity/LauncherEntry/" + pathID
	str := func(s string) uintptr { return gVariantNewString(cstr(s)) }
	entries := []uintptr{
		gVariantNewDictEntry(str("count"), gVariantNewVariant(gVariantNewInt32(int32(min(count, 1<<31-1))))),
		gVariantNewDictEntry(str("count-visible"), gVariantNewVariant(gVariantNewBoolean(boolInt(count != 0)))),
	}
	t := gVariantTypeNew(cstr("{sv}"))
	defer gVariantTypeFree(t)
	props := gVariantNewArray(t, &entries[0], uintptr(len(entries)))
	gVariantUnrefNonZero(dbusCall(bus, "com.canonical.Unity", path, "com.canonical.Unity.LauncherEntry", "Update", props))
}

// notifyQueue runs the server's calls in order, off the main thread.
var notifyQueue = sync.OnceValue(func() chan func() {
	ch := make(chan func(), 16)
	go func() {
		for fn := range ch {
			fn()
		}
	}()
	return ch
})

// showNotice has the server show n, replacing the one it showed for n's
// ID, and reports whether it did.
func showNotice(bus uintptr, appID string, n driver.Notification) bool {
	notices.mu.Lock()
	replaces := notices.server[n.ID]
	notices.mu.Unlock()
	str := func(s string) uintptr { return gVariantNewString(cstr(s)) }
	actions := []uintptr{str("default"), str("")}
	hints := []uintptr{gVariantNewDictEntry(str("desktop-entry"), gVariantNewVariant(str(appID)))}
	if n.Silent {
		hints = append(hints, gVariantNewDictEntry(str("suppress-sound"), gVariantNewVariant(gVariantNewBoolean(1))))
	}
	reply := dbusCall(bus, notifyName, notifyPath, notifyIface, "Notify",
		str(appID), gVariantNewUint32(replaces), str(appID), str(n.Title), str(n.Body),
		gVariantNewArray(0, &actions[0], uintptr(len(actions))),
		gVariantNewArray(0, &hints[0], uintptr(len(hints))),
		gVariantNewInt32(-1))
	if reply == 0 {
		return false
	}
	child := gVariantGetChildValue(reply, 0)
	id := gVariantGetUint32(child)
	gVariantUnref(child)
	gVariantUnref(reply)
	notices.mu.Lock()
	delete(notices.app, replaces)
	notices.server[n.ID], notices.app[id] = id, n.ID
	notices.mu.Unlock()
	return true
}

// noticeOf returns the app's ID of the server's notification a signal's
// parameters name first, "" when the app did not show it.
func noticeOf(params uintptr) (uint32, string) {
	child := gVariantGetChildValue(params, 0)
	id := gVariantGetUint32(child)
	gVariantUnref(child)
	notices.mu.Lock()
	defer notices.mu.Unlock()
	return id, notices.app[id]
}

// notificationActionCallback hears ActionInvoked: the default action is
// a press, which brings the window to the front.
func notificationActionCallback(conn, sender, path, iface, signal, params, data uintptr) {
	id, appID := noticeOf(params)
	if appID == "" || variantString(params, 1) != "default" {
		return
	}
	notices.mu.Lock()
	w := notices.window
	delete(notices.server, appID)
	delete(notices.app, id)
	notices.mu.Unlock()
	if w == nil || w.closing || w.engine == nil {
		return
	}
	gtkWindowPresent(w.win)
	w.engine.Dispatch(driver.Event{Kind: driver.EventNotificationPress, Text: appID})
}

// notificationClosedCallback hears NotificationClosed and forgets the
// notification.
func notificationClosedCallback(conn, sender, path, iface, signal, params, data uintptr) {
	id, appID := noticeOf(params)
	if appID == "" {
		return
	}
	notices.mu.Lock()
	if notices.server[appID] == id {
		delete(notices.server, appID)
	}
	delete(notices.app, id)
	notices.mu.Unlock()
}
