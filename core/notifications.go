package core

import (
	"errors"
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// NotificationPermission is whether the app may show notifications.
type NotificationPermission uint8

const (
	// NotificationsNotAsked is the user not asked yet:
	// RequestNotificationPermission shows the system's prompt.
	NotificationsNotAsked NotificationPermission = iota
	// NotificationsGranted lets the app show them.
	NotificationsGranted
	// NotificationsDenied keeps the app from showing them; only the
	// system's settings change it.
	NotificationsDenied
	// NotificationsUnavailable is a platform or a build that shows none,
	// such as a macOS app run outside its bundle, or code its Access does
	// not grant AccessNotifications.
	NotificationsUnavailable
)

// String returns NSH's name of p: "default", "granted", "denied" or
// "unavailable".
func (p NotificationPermission) String() string {
	switch p {
	case NotificationsGranted:
		return "granted"
	case NotificationsDenied:
		return "denied"
	case NotificationsUnavailable:
		return "unavailable"
	}
	return "default"
}

// Notification is a system notification (ShowNotification).
type Notification struct {
	// ID is the app's name for it: showing another with the same ID
	// replaces it, CloseNotification removes it and a press reports it.
	// Keep in it what the press opens ("chat:42"), as a press can start
	// the app anew.
	ID string
	// Title and Body are its text.
	Title, Body string
	// Silent shows it without the system's sound or vibration.
	Silent bool
}

// ErrNotification is ShowNotification's error for a notification
// without an ID or a title.
var ErrNotification = errors.New("nsgo: a notification needs an ID and a title")

// NotificationPermission answers done with whether the app may show
// notifications, without asking the user.
func (cx *Scope) NotificationPermission(done func(NotificationPermission)) {
	cx.notificationPermission(false, done)
}

// RequestNotificationPermission asks the user to let the app show
// notifications, with the system's prompt the first time, and answers
// done with the permission. Ask as the user does something that needs
// them: browsers refuse a prompt the user did not cause, and iOS and
// Android ask only once. An iOS app's badge needs it too.
func (cx *Scope) RequestNotificationPermission(done func(NotificationPermission)) {
	cx.notificationPermission(true, done)
}

func (cx *Scope) notificationPermission(ask bool, done func(NotificationPermission)) {
	t := cx.tree()
	if t == nil || cx.denied(AccessRequest{Permission: AccessNotifications}) {
		if done != nil {
			done(NotificationsUnavailable)
		}
		return
	}
	t.request(cx, driver.MountItem{Kind: driver.ItemNotificationPermission, On: ask}, func(ev driver.Event) {
		if done != nil {
			done(NotificationPermission(ev.Index))
		}
	})
}

// ShowNotification shows n, also while the app is in front, and answers
// done, when not nil, with whether the system took it: false without the
// permission. It returns ErrNotification, calling no done, for n without
// an ID or a title.
func (cx *Scope) ShowNotification(n Notification, done func(shown bool)) error {
	if n.ID == "" || n.Title == "" {
		return ErrNotification
	}
	answer := func(shown bool) {
		if done != nil {
			done(shown)
		}
	}
	t := cx.tree()
	if t == nil || cx.denied(AccessRequest{Permission: AccessNotifications}) {
		answer(false)
		return nil
	}
	dn := driver.Notification(n)
	t.request(cx, driver.MountItem{Kind: driver.ItemShowNotification, Notification: &dn},
		func(ev driver.Event) { answer(ev.Finished) })
	return nil
}

// CloseNotification removes the notification called id, if it still
// shows.
func (cx *Scope) CloseNotification(id string) {
	if t := cx.tree(); t != nil && id != "" && !cx.denied(AccessRequest{Permission: AccessNotifications}) {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemCloseNotification, Value: id})
		t.changed = true
	}
}

// SetBadge shows count on the app's icon, and 0 or less clears it: the
// Dock's on macOS, the Home Screen's on iOS, which needs the
// notification permission, the taskbar button's on Windows and an
// installed web app's. Android has no count to set: launchers mark an app
// whose notifications show, and count goes with those shown after it.
// Elsewhere nothing happens.
func (cx *Scope) SetBadge(count int) {
	t := cx.tree()
	if t == nil || cx.denied(AccessRequest{Permission: AccessNotifications}) {
		return
	}
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemBadge, Count: uint32(min(max(count, 0), 1<<31-1))})
	t.changed = true
}

type notificationPressHandler struct {
	owner *Scope
	fn    func(id string)
}

// OnNotificationPress runs fn, while the current owner lives, with the ID
// of a notification the user pressed, which brings the app to the front.
// Presses that came before any handler, as the one that started the app,
// go to the first one registered.
func (cx *Scope) OnNotificationPress(fn func(id string)) {
	if cx.denied(AccessRequest{Permission: AccessNotifications}) {
		return
	}
	rt := cx.rt
	h := &notificationPressHandler{owner: cx.current(), fn: fn}
	rt.notificationPresses = append(rt.notificationPresses, h)
	h.owner.addCleanup(func() {
		rt.notificationPresses = slices.DeleteFunc(rt.notificationPresses, func(o *notificationPressHandler) bool { return o == h })
	})
	if ids := rt.pressesPending; len(ids) > 0 {
		rt.pressesPending = nil
		// Not while the caller builds its view: the handler may well
		// change what it shows.
		h.owner.Post(func() {
			for _, id := range ids {
				h.fn(id)
			}
		})
	}
}

// notificationPressed hands the press of notification id to the
// handlers, or keeps it for the first one.
func (rt *runtime) notificationPressed(id string) {
	if len(rt.notificationPresses) == 0 {
		rt.pressesPending = append(rt.pressesPending, id)
		return
	}
	for _, h := range slices.Clone(rt.notificationPresses) {
		rt.tree.within(h.owner, func() { h.fn(id) })
	}
}
