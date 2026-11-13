---
title: Widgets
description: Show the app's views on the Home Screen and the Lock Screen of iPhones and iPads, and on the Mac's desktop.
group: Platform features
order: 47
---

## A kind of widget

Widgets show a glance of the app whether it runs or not: on the Home Screen and the Lock Screen of iPhones and iPads, and on the Mac's desktop and in Notification Center. A kind is one widget the system's gallery offers, with the families it comes in:

```yaml
widgets:
  - kind: counter
    name: Counter                 # shown in the system's gallery of widgets
    description: Shows the count.
    families: [small, medium, circular]  # also large, extra_large, rectangular and inline
# apple: { app_group: group.dev.example.hello }  # the iOS app group, group.<id> by default
```

A widget extension draws it, which `build` and `dev` compile into the app with Xcode's Swift compiler ([Building and shipping](building.md#widgets-and-live-activities)). It runs none of the app's code, so a widget shows what the app wrote last, until it writes again.

## A timeline

`widget.Update` writes what the widgets of a kind show from now on, as a timeline of entries: the system shows the entry whose date passed last, then each later one at its date, without the app. A zero `Date` is now. Without entries, the widgets show the kind's name and description. An app in the foreground updates its widgets as often as it likes, and one in the background a few dozen times a day.

```go
err := widget.Update("counter", widget.Entry{
	View: widget.Column{Style: widget.Style{Gap: 4}, Children: []widget.View{
		widget.Text{Content: "Count", Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary}},
		widget.Text{Content: strconv.Itoa(n), Style: widget.Style{FontSize: 44, FontWeight: core.FontWeightBold}},
	}},
})
```

`Entry.Families` shows a family a view of its own in place of `Entry.View`, `Entry.Background` fills the widget behind it, past the margins the system keeps, and `Entry.URL` opens with the app when the widget is tapped. `widget.Update` reports `widget.ErrNoWidgets` in an app built without them, `widget.ErrUnsupported` elsewhere than macOS and iOS, and `widget.Available` says which before a write. It may run on any goroutine, before `nsgo.Run` too.

## Views

A widget's views are not core's elements: they lay out as SwiftUI does, a view as large as its content unless `Style` sizes it or makes it fill, and a container centering what it holds unless `AlignItems` and `Justify` say otherwise. `Text`, `Symbol` and `Image`, of bytes or of a file that `Update` reads, show content; `Column`, `Row`, `Stack`, `Spacer` and `Divider` arrange it; `Rectangle`, `Progress` and `Gauge` draw; and `Date` shows a time with the system's formats, which `Relative`, `Offset` and `Timer` keep counting by themselves, without an entry for each second.

```go
widget.Entry{
	View: widget.Column{Style: widget.Style{Gap: 4}, Children: []widget.View{
		widget.Text{Content: "Next hour in", Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary}},
		widget.Date{Time: hour, Format: widget.Timer, Style: widget.Style{TextStyle: widget.Title2, FontVariant: core.FontTabularNums}},
		widget.Progress{Value: 0.4, Style: widget.Style{Color: widget.Accent}},
	}},
}
```

`Style` takes a gap, padding, a size or fill, alignment, a background with rounded corners, the system's text styles or a font size, weight, design and variant, line limits, a `Link` that opens with the app, and `Accented`, which puts a view among those the system tints. `widget.Primary`, `Secondary` and `Accent` follow the appearance, and `RGB` and `RGBA` take another color in dark mode with `Dark`.

## Live Activities

The same extension draws the app's Live Activities on the Lock Screen and in the Dynamic Island ([Live Activities](live-activities.md)).

## Next

- [Live Activities](live-activities.md): what the app follows as it happens.
- [Notifications](notifications.md): the notifications and badges beside a widget.
- [Building and shipping](building.md): the app group the extension shares.
