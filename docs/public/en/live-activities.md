---
title: Live Activities
description: Follow something as it happens on the Lock Screen and in the Dynamic Island, and as Live Updates on Android.
group: Platform features
order: 48
---

## Turn them on

A Live Activity follows something as it happens, a delivery or a timer: on the Lock Screen and in the Dynamic Island of iPhones from iOS 16.1, and as a Live Update on Android 16, at the top of the shade and in a chip of the status bar; earlier Androids show an ongoing notification. Turn them on in `nsgo.yaml`:

```yaml
live_activities:
  enabled: true
  # frequent_updates: true   # iOS: more updates than the system's budget, as the user allows
```

## Start, update and end

`widget.StartActivity` starts one of a kind, which names what it follows and is written as the kinds of widgets are, showing a `widget.ActivityContent`. `Activity.Update` replaces what it shows, `Activity.Alert` does too and alerts the user, and `Activity.End` ends it, with final content when not nil, and leaves the Lock Screen as the `Dismissal` says. `widget.Activities` lists the app's running activities, those of earlier runs of the app included, and `widget.ActivitiesEnabled` says whether the user and the platform allow them.

```go
a, err := widget.StartActivity("delivery", widget.ActivityContent{
	Title: "Your order", Body: "On its way", Symbol: "box.truck.fill",
	Timer: arrival, ShowProgress: true, Progress: 0.4,
	Actions: []widget.ActivityAction{{ID: "pause", Title: "Pause", Symbol: "pause.fill"}},
})
if err != nil {
	return
}
a.Update(widget.ActivityContent{Title: "Your order", Body: "Arrived", Status: "Now"})
a.End(nil, widget.DismissAt(time.Now().Add(2*time.Hour)))
```

`StartActivity` reports `widget.ErrNoActivities` in an app built without them, `widget.ErrActivitiesUnsupported` elsewhere than iOS 16.1 and Android, and `widget.ErrActivitiesDisabled` when the user turned them off; `Update` and `End` report `widget.ErrActivityEnded` once it ended. An activity's content is at most 4 KB, as ActivityKit and APNs take.

## What it shows

Both platforms show the title and body, the status or a timer the system ticks, a progress bar, up to three actions and the accent color. iOS draws them with the widget extension, and takes views of `widget`'s in their place for the Lock Screen and each part of the Dynamic Island: `LockScreen`, `Leading`, `Trailing`, `Center` and `Bottom` expanded, `CompactLeading` and `CompactTrailing` in the compact island, and `Minimal` as several activities share it. Android shows the fields alone, with the app's icon, as Live Updates take no views of the app.

```go
widget.ActivityContent{
	Title: "Order 412", Body: "On its way", Status: "8 min", Symbol: "box.truck.fill",
	Tint: widget.RGB(0x30d158), ShowProgress: true, Progress: 0.4,
	Trailing: widget.Text{Content: "8 min", Style: widget.Style{FontSize: 34, FontWeight: core.FontWeightBold}},
}
```

`URL` opens with the app when the activity is tapped, `Stale` is when iOS shows the content as out of date, and `Background` fills the activity on the Lock Screen.

## Presses

`widget.OnAction` runs the handler of an action's or a `widget.Button`'s press, with the activity and the action's ID. Set it in `main`, before `nsgo.Run`: on iOS the system starts the app in the background for a press, without its views, and holds the activity until the handler returns, so the handler updates it there; on Android a press opens the app. The handler runs on a goroutine of the system's, not the app's views', which it reaches through a `core.Scope`'s `Post`.

```go
func main() {
	widget.OnAction(func(p widget.ActivityPress) {
		if p.Action == "pause" {
			p.Activity.Update(paused)
		}
	})
	nsgo.Run(Root)
}
```

A view of the activity's does it too, on iOS 17 and later: `widget.Button{Action: "pause"}` runs the same handler without opening the app, and shows its children alone where a press only opens the app.

## Push updates

An app in the background updates its activities only while it runs, so one that follows what happens elsewhere starts them with `widget.ActivityOptions{Push: true}`, sends the token `widget.OnPushToken` hears to its server, and has it push APNs `liveactivity` updates whose `content-state` is `widget.ActivityState`'s answer for the content: no images go in a push. From iOS 17.2 the app gets a token to start activities by push too, with the attributes type `widget.ActivityAttributesType` and attributes of the kind and id. Pushes need the push notifications entitlement in the app's profile.

```go
state, err := widget.ActivityState(content)  // the push's content-state
```

## Limits

The system ends an activity after eight hours. `widget.DismissDefault` leaves an ended one on the Lock Screen for up to four hours on iOS and takes it away at once on Android, `widget.DismissNow` takes it away at once and `widget.DismissAt` at a time within four hours. Android asks for `POST_NOTIFICATIONS` and `POST_PROMOTED_NOTIFICATIONS` itself: ask the user with `cx.RequestNotificationPermission` first ([Notifications](notifications.md)). The gallery's Widgets demo and `examples/widgets` run one.

## Next

- [Widgets](widgets.md): the Home Screen and the Lock Screen's timelines, drawn by the same extension.
- [Notifications](notifications.md): the permission Android asks for first.
- [Building and shipping](building.md): the extension and the entitlements it needs.
