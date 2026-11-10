---
title: Button
description: A push button, its variants and sizes, and the icon button.
group: Components
order: 31
---

`ui.Button` is a push button in `ui`'s look, with the variants and sizes a window needs. `OnPress` runs on a completed press.

```go preview=button/basic
ui.Button{Text: "Save", OnPress: save}
ui.Button{Text: "Cancel", Variant: ui.ButtonSecondary, OnPress: cancel}
```

## Variants and sizes

`ui.Button.Variant` is `ButtonPrimary` (the default), `ButtonSecondary`, `ButtonOutline` or `ButtonGhost`. `Size` is `SizeDefault` (28 points), `SizeSmall` (24) or `SizeLarge` (32).

```go preview=button/variants
ui.Button{Text: "Delete", Variant: ui.ButtonOutline, OnPress: remove}
ui.Button{Text: "Continue", Size: ui.SizeLarge, OnPress: next}
ui.Button{Text: "Save", Icon: lucide.Check, Flex: 1, OnPress: save}
```

`Color` fills the button with a color of the app's instead of a variant's, and `Tint` its icon. `Flex` gives it a share of a row; `TestID` names it for tests.

A button of an icon alone is `ui.IconButton`, with `Variant` `IconButtonGhost` or `IconButtonRaised`, a `Size` in points and a `Label` for assistive technology. A `ui.Button` with `Icon` and no `Text` is a square too, and needs `Label`.

```go preview=button/icon
ui.IconButton{Icon: lucide.Trash, Label: "Delete", Variant: ui.IconButtonGhost, OnPress: remove}
```

## Content and states

`ui.Button` shows `Text`, an optional leading `Icon`, or both.

```go
ui.Button{Text: "Saving…", Disabled: saving.Get, OnPress: save}
```

`Disabled` takes a function, so the button follows a signal: a disabled button takes neither presses nor the keyboard focus, and its look is the disabled state, not a color the app picks.

## Accessibility

The button reports `RoleButton`, its text as its name, and `Hint` as what pressing it does. `Label` replaces the text for assistive technology. An icon-only button without `Label` is unreadable, so name it.

Return and Space press a focused button on macOS, the web and Apple TV; a phone's screen reader presses it as a click.

## Platform

The size is the same on every device. The button is a view drawn the same way everywhere, not a platform button; presses, focus and accessibility come from the platform.

## Related

- [Forms](../forms.md): buttons in a form and validation.
- [Styles and themes](../styles.md): the theme a button takes its colors from.
- [Accessibility](../accessibility.md): roles, labels and focus.
- The gallery's Button page.
- [Unstyled components](../authoring/base.md#button): `base.Button`, for a button of a look of your own.
