---
title: Switch
description: A setting that is on or off, in three sizes.
group: Components
order: 35
---

`ui.Switch` is a setting that takes effect at once. It holds no value: the app owns it and updates it in `OnChange`.

```go preview=switch/basic
notifications := core.NewSignal(cx, true)
ui.Switch{Checked: notifications.Get, OnChange: notifications.Set, Label: "Notifications"}
```

## Sizes

`ui.Switch` takes `Checked`, `OnChange`, `Label` and `Size` (`SizeDefault`, `SizeSmall` or `SizeLarge`). The knob slides and the track changes color on the platform's frame clock.

```go preview=switch/small
ui.Switch{Checked: sync.Get, OnChange: sync.Set, Label: "Sync automatically", Size: ui.SizeSmall}
```

A switch is for a setting that applies at once. For a choice inside a form, use a [checkbox](checkbox.md); for one of several options, use `ui.Radio`s in a `ui.RadioGroup` or a [select](select.md).

## Accessibility

The switch reports `RoleSwitch` and its on or off state. `Label` is what assistive technology reads, and the text beside a switch is usually its label. Space or a press toggles it when it has the keyboard focus.

## Platform

The control is drawn by the app, so it looks the same everywhere. Presses, focus and the screen reader are the platform's. A phone's switch is the same size as a desktop's by design; `SizeSmall` is for dense rows, not for a device.

## Related

- [Checkbox](checkbox.md): one or more choices.
- [Forms](../forms.md): settings in a form.
- [Accessibility](../accessibility.md): labels and focus.
- The gallery's Switch page.
- [Unstyled components](../authoring/base.md#switch): `base.Switch`, for a switch of a look of your own.
