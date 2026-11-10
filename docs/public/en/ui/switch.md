---
title: Switch
description: A setting that is on or off, with base's track and thumb and ui's sizes.
group: Components
order: 64
---

`base.Switch` is a setting that takes effect at once; `ui.Switch` is the same control in Onevra's look. It holds no value: the app owns it and updates it in `OnChange`.

```go
notifications := core.NewSignal(cx, true)
ui.Switch{Checked: notifications.Get, OnChange: notifications.Set, Label: "Notifications"}
```

## The base switch

`base.Switch` is parts the app styles: a track, a thumb inside it, then its children as the label. A press reports the value it asks for, the opposite of `Checked`; the app sets the signal.

```go
base.Switch{
	Checked:  dark.Get,
	OnChange: dark.Set,
	Label:    "Dark appearance",
	Style:    core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8},
	TrackStyle: core.Style{Width: core.Pt(38), Height: core.Pt(22), CornerRadius: 11},
	ThumbStyle: core.Style{Width: core.Pt(18), Height: core.Pt(18), CornerRadius: 9},
}
```

`Disabled` stops presses and the focus. The track and thumb take `Style` and `StyleFunc` of their own, and their state styles can follow `Checked`.

## The styled switch

`ui.Switch` takes `Checked`, `OnChange`, `Label` and `Size` (`SizeDefault`, `SizeSmall` or `SizeLarge`). The knob slides and the track changes color on the platform's frame clock.

```go
ui.Switch{Checked: sync.Get, OnChange: sync.Set, Label: "Sync automatically", Size: ui.SizeSmall}
```

A switch is for a setting that applies at once. For a choice inside a form, use a [checkbox](checkbox.md); for one of several options, use `base.Radio` or a [select](select.md).

## Accessibility

The switch reports `RoleSwitch` and its on or off state. `Label` is what assistive technology reads, and the text beside a switch is usually its label. Space or a press toggles it when it has the keyboard focus.

## Platform

The control is drawn by the app, so it looks the same everywhere. Presses, focus and the screen reader are the platform's. A phone's switch is the same size as a desktop's by design; `SizeSmall` is for dense rows, not for a device.

## Related

- [Checkbox](checkbox.md): one or more choices.
- [Forms](../forms.md): settings in a form.
- [Accessibility](../accessibility.md): labels and focus.
- The gallery's Switch page, with its `base` and `ui` forms.
