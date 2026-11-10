---
title: Slider
description: One value, or a range, along a track with the keyboard and the pointer.
group: Components
order: 37
---

`ui.Slider` picks a value along a track, with the drag, the step and the keyboard. One value is a single knob; `High` makes a range of two.

```go preview=slider/basic
volume := core.NewSignal(cx, 40.0)
ui.Slider{Value: volume.Get, OnChange: volume.Set, Min: 0, Max: 100, Step: 5, Label: "Volume"}
```

## A range

Set `High` and `OnHighChange` to choose two values. `Value` is the low end. `LowLabel` and `HighLabel` name the knobs for assistive technology.

```go preview=slider/range
price := core.NewSignal(cx, 20.0)
ceiling := core.NewSignal(cx, 80.0)
ui.Slider{
	Value: price.Get, OnChange: price.Set,
	High: ceiling.Get, OnHighChange: ceiling.Set,
	Min: 0, Max: 100, Step: 1,
	Label: "Price", LowLabel: "Minimum", HighLabel: "Maximum",
}
```

## Steps and values

`Step` snaps the value; a step of 0 takes any value. `ValueText` changes what assistive technology reads as the value, and `Disabled` stops it.

```go preview=slider/format
ui.Slider{Value: speed.Get, OnChange: speed.Set, Min: 0.5, Max: 2, Step: 0.25, Label: "Speed",
	ValueText: func(v float64) string { return fmt.Sprintf("%g×", v) }}
```

## Behavior

A drag follows the pointer and reports `OnChange` while it moves. The arrow keys move a focused knob by a step, Home and End go to the ends, and Page Up and Page Down move further. `ui.Slider` marks the steps under the track when `Step` is above 0, up to 60 marks.

## Accessibility

The slider reports `RoleAdjustable`, its `Label` and its value; a range has both knobs with their own labels. `ValueText` is read instead of the number. The keyboard and a screen reader's increment and decrement actions change it.

## Platform

The track is drawn by the app and the same size everywhere. Presses, focus and the screen reader are the platform's; a touch drag behaves as a pointer drag with the same step.

## Related

- [Forms](../forms.md): sliders in a form.
- [Accessibility](../accessibility.md): values and adjustable roles.
- The gallery's Slider, RangeSlider and Rating pages.
- [Unstyled components](../authoring/base.md#slider): `base.Slider`, vertical and with marks of your own.
