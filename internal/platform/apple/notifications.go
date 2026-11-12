//go:build darwin

package apple

import (
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// System notifications for macOS and iOS through UserNotifications'
// UNUserNotificationCenter, as NSH's notifications: its delegate shows
// them while the app is in front and reports their presses. The center
// serves only an app in a bundle, and raises for a bare executable, so
// one outside a bundle answers unavailable.

const notificationsPath = "/System/Library/Frameworks/UserNotifications.framework/UserNotifications"

// notifications holds what the delegate reports: the presses it heard
// before a surface listened, and where they go once one does.
var notifications struct {
	mu      sync.Mutex
	press   func(id string)
	pending []string
}

// notificationCenter returns the current center, or 0 where the process
// has none: outside an app bundle, or without the framework.
var notificationCenter = sync.OnceValue(func() objc.ID {
	bundle := objc.ID(Class("NSBundle")).Send(sel("mainBundle"))
	if bundle.Send(sel("bundleIdentifier")) == 0 || !strings.HasSuffix(GoString(bundle.Send(sel("bundlePath"))), ".app") {
		return 0
	}
	if _, err := purego.Dlopen(notificationsPath, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		return 0
	}
	c := Class("UNUserNotificationCenter")
	if c == 0 {
		return 0
	}
	return objc.ID(c).Send(sel("currentNotificationCenter"))
})

// WatchNotifications makes NSGo the center's delegate. Call it as the app
// starts, before it finishes launching, so the press that launched it is
// heard.
func WatchNotifications() {
	center := notificationCenter()
	if center == 0 {
		return
	}
	// UNNotificationPresentationOptions: banner, list, sound and badge,
	// as the notification asks.
	const present = 1<<4 | 1<<3 | 1<<1 | 1<<0
	delegate := objc.ID(RegisterClass("NsgoNotificationDelegate", Class("NSObject"),
		[]*objc.Protocol{objc.GetProtocol("UNUserNotificationCenterDelegate")}, []objc.MethodDef{
			{Cmd: sel("userNotificationCenter:willPresentNotification:withCompletionHandler:"),
				Fn: func(_ objc.ID, _ objc.SEL, _, _, done objc.ID) { CallBlock(done, present) }},
			{Cmd: sel("userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:"),
				Fn: func(_ objc.ID, _ objc.SEL, _, response, done objc.ID) {
					action := GoString(response.Send(sel("actionIdentifier")))
					if action == "com.apple.UNNotificationDefaultActionIdentifier" {
						notificationPressed(GoString(response.Send(sel("notification")).Send(sel("request")).Send(sel("identifier"))))
					}
					CallBlock(done)
				}},
		})).Send(sel("new"))
	center.Send(sel("setDelegate:"), delegate)
}

// SetNotificationPress sends the presses of notifications to press, those
// heard before it first.
func SetNotificationPress(press func(id string)) {
	notifications.mu.Lock()
	notifications.press = press
	pending := notifications.pending
	notifications.pending = nil
	notifications.mu.Unlock()
	for _, id := range pending {
		press(id)
	}
}

func notificationPressed(id string) {
	notifications.mu.Lock()
	press := notifications.press
	if press == nil {
		notifications.pending = append(notifications.pending, id)
	}
	notifications.mu.Unlock()
	if press != nil {
		press(id)
	}
}

// NotificationPermission answers with whether the app may show
// notifications (driver.NotificationsNotAsked, ...), asking the user
// first when ask is set and the user was not asked yet. answer runs on
// any goroutine.
func NotificationPermission(ask bool, answer func(int)) {
	center := notificationCenter()
	if center == 0 {
		answer(driver.NotificationsUnavailable)
		return
	}
	query := func() {
		read := objc.NewBlock(func(_ objc.Block, settings objc.ID) {
			// UNAuthorizationStatus: not determined, denied, authorized,
			// provisional and ephemeral.
			switch objc.Send[int](settings, sel("authorizationStatus")) {
			case 0:
				answer(driver.NotificationsNotAsked)
			case 1:
				answer(driver.NotificationsDenied)
			default:
				answer(driver.NotificationsGranted)
			}
		})
		center.Send(sel("getNotificationSettingsWithCompletionHandler:"), read)
		read.Release()
	}
	if !ask {
		query()
		return
	}
	// UNAuthorizationOptions: badge, sound and alert.
	const options = 1<<0 | 1<<1 | 1<<2
	asked := objc.NewBlock(func(_ objc.Block, _ bool, _ objc.ID) { query() })
	center.Send(sel("requestAuthorizationWithOptions:completionHandler:"), uint(options), asked)
	asked.Release()
}

// ShowNotification shows n, replacing a notification of its ID, and
// answers with whether the center took it: not without the permission.
// answer runs on any goroutine.
func ShowNotification(n driver.Notification, answer func(bool)) {
	center := notificationCenter()
	if center == 0 {
		answer(false)
		return
	}
	content := objc.ID(Class("UNMutableNotificationContent")).Send(sel("new"))
	content.Send(sel("setTitle:"), NSString(n.Title))
	content.Send(sel("setBody:"), NSString(n.Body))
	if !n.Silent {
		content.Send(sel("setSound:"), objc.ID(Class("UNNotificationSound")).Send(sel("defaultSound")))
	}
	request := objc.ID(Class("UNNotificationRequest")).Send(sel("requestWithIdentifier:content:trigger:"),
		NSString(n.ID), content, objc.ID(0))
	content.Send(sel("release"))
	added := objc.NewBlock(func(_ objc.Block, failure objc.ID) { answer(failure == 0) })
	center.Send(sel("addNotificationRequest:withCompletionHandler:"), request, added)
	added.Release()
}

// CloseNotification removes the notification called id, shown or still
// to show.
func CloseNotification(id string) {
	center := notificationCenter()
	if center == 0 {
		return
	}
	ids := objc.ID(Class("NSArray")).Send(sel("arrayWithObject:"), NSString(id))
	center.Send(sel("removeDeliveredNotificationsWithIdentifiers:"), ids)
	center.Send(sel("removePendingNotificationRequestsWithIdentifiers:"), ids)
}

// SetCenterBadge sets the badge through the center, as iOS 16 and later
// do, and reports whether it could.
func SetCenterBadge(count uint32) bool {
	center := notificationCenter()
	if center == 0 || !MsgBool(center, sel("respondsToSelector:"), uintptr(sel("setBadgeCount:withCompletionHandler:"))) {
		return false
	}
	center.Send(sel("setBadgeCount:withCompletionHandler:"), int(count), objc.ID(0))
	return true
}
