---
title: Notifications
description: Ask to show notifications, show and close them, count them on the app's icon and hear presses.
group: Platform features
order: 45
---

## Permission

`cx.RequestNotificationPermission` asks the user to let the app show notifications, with the system's prompt the first time, and answers with a `core.NotificationPermission`: `NotificationsGranted`, `NotificationsDenied` or `NotificationsUnavailable` where the platform or the build shows none. Ask as the user does something that needs notifications: iOS and Android ask only once, and browsers refuse a prompt the user did not cause. `cx.NotificationPermission` reads the answer without asking.

```go
cx.RequestNotificationPermission(func(p core.NotificationPermission) {
	if p == core.NotificationsGranted {
		cx.ShowNotification(core.Notification{ID: "chat:42", Title: "Ada", Body: "Lunch?"}, nil)
	}
})
```

## Showing one

`cx.ShowNotification` shows a notification, also while the app is in front, and replaces one of the same `ID`; `Silent` leaves out the system's sound and vibration. It answers `done` with whether the system took the notification, false without the permission, and returns `core.ErrNotification`, calling no `done`, for one without an ID or a title. `cx.CloseNotification` removes the one an ID names.

```go
err := cx.ShowNotification(core.Notification{ID: "sync", Title: "Synced", Body: "12 notes"}, func(shown bool) {})
cx.CloseNotification("sync")
```

## Presses

`cx.OnNotificationPress` runs its function with the `ID` of a notification the user pressed, while the component that set it lives, and brings the app to the front. A press that started the app reaches the first handler registered. Keep in `ID` what the press opens (`"chat:42"`), as a press can start the app anew. On Windows it does so for an app its installer put in the Start menu; one run otherwise hears presses only while it runs.

```go
cx.OnNotificationPress(func(id string) { openChat(strings.TrimPrefix(id, "chat:")) })
```

An Android app asking from Android 13 lists `android.permission.POST_NOTIFICATIONS` in `nsgo.yaml`'s `android.permissions`.

## Badge

`cx.SetBadge` puts a count on the app's icon: the Dock's on macOS, the Home Screen's on iOS, which needs the notification permission, the taskbar button's on Windows, Unity-compatible Linux launchers and an installed web app's. A count of 0 or less clears it. Launchers without a Linux badge interface and Android have no count to set; Android launchers mark an app whose notifications show.

```go
cx.SetBadge(len(unread.Get()))
```

## Next

- [Haptics](haptics.md): the feedback the device plays under the user's finger.
- [Platform services](services.md): alerts, the clipboard, files, storage and networking.
- [Live Activities](live-activities.md): what the app follows, on the Lock Screen and in the Dynamic Island.
- [Building and shipping](building.md): the permissions a package needs.
