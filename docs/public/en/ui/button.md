---
title: Button
description: A push button, its variants and sizes, and the icon button.
group: Components
order: 60
---

`base.Button` is a pressable box with the button role and no look of its own. `ui.Button` is the same control in Onevra's look, with the variants a window needs.

```go
ui.Button{Text: "Save", OnPress: save}
ui.Button{Text: "Cancel", Variant: ui.ButtonSecondary, OnPress: cancel}
```

## The base button

`base.Button` takes its look from `Style` and its content from `Children`. `OnPress` runs on a completed press; `OnPressDown` and `OnPressUp` follow a press starting and ending, for feedback of the app's own.

```go
base.Button{
	Label:   "Save",
	OnPress: func(core.PressEvent) { save() },
	Style: core.Style{Background: core.RGB(0x007aff), Padding: core.Symmetric(8, 14), CornerRadius: 8, AlignSelf: core.AlignStart},
	Children: []core.Node{core.Text{Content: "Save", Style: core.Style{Color: core.RGB(0xffffff)}}},
}
```

`Disabled`, `Selected` and `Expanded` report a state to the app, the style and assistive technology. A toggle or the current item of a list uses `Selected`; a disclosure uses `Expanded`.

## Variants and sizes

`ui.Button.Variant` is `ButtonPrimary` (the default), `ButtonSecondary`, `ButtonOutline` or `ButtonGhost`. `Size` is `SizeDefault` (28 points), `SizeSmall` (24) or `SizeLarge` (32).

```go
ui.Button{Text: "Delete", Variant: ui.ButtonOutline, OnPress: remove}
ui.Button{Text: "Continue", Size: ui.SizeLarge, OnPress: next}
ui.Button{Text: "Save", Icon: lucide.Check, Flex: 1, OnPress: save}
```

`Color` fills the button with a color of the app's instead of a variant's. `Flex` gives it a share of a row; `TestID` names it for tests.

A button of an icon alone is `ui.IconButton`, with `Variant` `IconButtonGhost` or `IconButtonRaised`, a `Size` in points and a `Label` for assistive technology. A `ui.Button` with `Icon` and no `Text` is a square too, and needs `Label`.

```go
ui.IconButton{Icon: lucide.Trash, Label: "Delete", Variant: ui.IconButtonGhost, OnPress: remove}
```

## Content and states

`ui.Button` shows `Text`, an optional leading `Icon`, or both. The base form accepts any children, so an icon and a label of the app's own are a row of `core.View` and `core.Text`.

A disabled button takes neither presses nor the keyboard focus. Its look is the disabled state, not a color the app picks. `Style.Active` and `Style.Hover` add feedback of the app's own to the base form.

## Accessibility

The button reports `RoleButton`, its text as its name, and `Hint` as what pressing it does. `Label` replaces the text for assistive technology. An icon-only button without `Label` is unreadable, so name it.

Return and Space press a focused button on macOS, the web and Apple TV; a phone's screen reader presses it as a click. `NoFocus` takes a button out of the Tab stops when a row or a list reaches it another way.

## Platform

The size is the same on every device. A `base.Button` is the app's own view with the button role, not a platform button, so its look is exactly what the app styles; presses, focus and accessibility come from the platform.

## Related

- [Forms](../forms.md): buttons in a form and validation.
- [Styles and themes](../styles.md): the states and colors a base button takes.
- [Accessibility](../accessibility.md): roles, labels and focus.
- The gallery's Button page, with its `base` and `ui` forms.
