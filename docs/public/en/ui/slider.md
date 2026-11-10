---
title: Slider
description: One value, or a range, along a track with the keyboard and the pointer.
group: Components
order: 66
---

`base.Slider` picks a value along a track and owns the drag, the step and the keyboard; `ui.Slider` is the same control with Onevra's track and knob. One value is a single knob; `High` makes a range of two.

```go
volume := core.NewSignal(cx, 40.0)
ui.Slider{Value: volume.Get, OnChange: volume.Set, Min: 0, Max: 100, Step: 5, Label: "Volume"}
```

## A range

Set `High` and `OnHighChange` to choose two values. `Value` is the low end. `LowLabel` and `HighLabel` name the knobs for assistive technology.

```go
price := core.NewSignal(cx, 20.0)
ceiling := core.NewSignal(cx, 80.0)
ui.Slider{
	Value: price.Get, OnChange: price.Set,
	High: ceiling.Get, OnHighChange: ceiling.Set,
	Min: 0, Max: 100, Step: 1,
	Label: "Price", LowLabel: "Minimum", HighLabel: "Maximum",
}
```

## The base slider

`base.Slider` takes a `base.SliderValue`: `base.SingleValue(v)` or `base.RangeValue(low, high)`. It styles the track, the indicator, the range and the thumbs separately, so a design of the app's own needs no `ui`.

```go
level := core.NewSignal(cx, 3.0)
base.Slider{
	Value: func() base.SliderValue { return base.SingleValue(level.Get()) },
	OnChange: func(v base.SliderValue) { level.Set(v.End) },
	Min: func() float64 { return 0 },
	Max: func() float64 { return 10 },
	Step: func() float64 { return 1 },
	Label: "Level",
	Style: core.Style{Height: core.Pt(28)},
	TrackStyle: core.Style{Height: core.Pt(4), CornerRadius: 2},
	ThumbStyle: core.Style{Width: core.Pt(18), Height: core.Pt(18), CornerRadius: 9},
}
```

`Axis` makes it vertical, and `ValueText` changes what assistive technology reads as the value. `Step` snaps; a step of 0 takes any value. `Marks` places the app's own ticks on the indicator.

## Behavior

A drag follows the pointer and reports `OnChange` while it moves; `OnChangeEnd` reports the value once it stops. The arrow keys move a focused knob by a step, Home and End go to the ends, and Page Up and Page Down move further. `ui.Slider` marks the steps under the track when `Step` is above 0, up to 60 marks.

## Accessibility

The slider reports `RoleAdjustable`, its `Label` and its value; a range has both knobs with their own labels. `ValueText` is read instead of the number. The keyboard and a screen reader's increment and decrement actions change it.

## Platform

The track is drawn by the app and the same size everywhere. Presses, focus and the screen reader are the platform's; a touch drag behaves as a pointer drag with the same step.

## Related

- [Forms](../forms.md): sliders in a form.
- [Accessibility](../accessibility.md): values and adjustable roles.
- The gallery's Slider, RangeSlider and Rating pages.
