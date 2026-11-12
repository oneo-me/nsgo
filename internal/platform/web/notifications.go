//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// System notifications through the page's Notification API, and the badge
// of an installed web app's icon through navigator.setAppBadge. A page
// whose browser has no Notification, or shows them only from a service
// worker, as Chrome on Android, answers unavailable or fails to show.

// shownNotifications are the notifications the page shows, by ID, which
// one of the same ID and CloseNotification close.
var shownNotifications = map[string]js.Value{}

func (s *surface) notify(it *driver.MountItem) {
	api := js.Global().Get("Notification")
	id := it.Count
	switch it.Kind {
	case driver.ItemNotificationPermission:
		answer := func(p js.Value) {
			s.engine.Dispatch(driver.Event{Kind: driver.EventNotification, Count: id, Index: notificationPermission(p)})
		}
		if !api.Truthy() {
			s.engine.Dispatch(driver.Event{Kind: driver.EventNotification, Count: id, Index: driver.NotificationsUnavailable})
			return
		}
		if !it.On || api.Get("permission").String() != "default" {
			answer(api.Get("permission"))
			return
		}
		then(api.Call("requestPermission"), answer, func(js.Value) { answer(api.Get("permission")) })
	case driver.ItemShowNotification:
		n := *it.Notification
		if !api.Truthy() || api.Get("permission").String() != "granted" {
			s.engine.Dispatch(driver.Event{Kind: driver.EventNotification, Count: id})
			return
		}
		if old, ok := shownNotifications[n.ID]; ok {
			old.Set("onclick", js.Null())
			old.Call("close")
		}
		shown := func() (note js.Value, ok bool) {
			// The constructor throws where only a service worker shows
			// them.
			defer func() { ok = recover() == nil }()
			return api.New(n.Title, map[string]any{"body": n.Body, "tag": n.ID, "silent": n.Silent}), true
		}
		note, ok := shown()
		if ok {
			shownNotifications[n.ID] = note
			var click js.Func
			click = js.FuncOf(func(js.Value, []js.Value) any {
				js.Global().Call("focus")
				note.Call("close")
				delete(shownNotifications, n.ID)
				click.Release()
				s.engine.Dispatch(driver.Event{Kind: driver.EventNotificationPress, Text: n.ID})
				return nil
			})
			note.Set("onclick", click)
		}
		s.engine.Dispatch(driver.Event{Kind: driver.EventNotification, Count: id, Finished: ok})
	case driver.ItemCloseNotification:
		if note, ok := shownNotifications[it.Value]; ok {
			note.Call("close")
			delete(shownNotifications, it.Value)
		}
	case driver.ItemBadge:
		nav := js.Global().Get("navigator")
		switch {
		case !nav.Get("setAppBadge").Truthy():
		case it.Count == 0:
			then(nav.Call("clearAppBadge"), func(js.Value) {}, func(js.Value) {})
		default:
			then(nav.Call("setAppBadge", it.Count), func(js.Value) {}, func(js.Value) {})
		}
	}
}

// notificationPermission is Notification.permission's value as the
// driver's.
func notificationPermission(p js.Value) int {
	switch p.String() {
	case "granted":
		return driver.NotificationsGranted
	case "denied":
		return driver.NotificationsDenied
	}
	return driver.NotificationsNotAsked
}
