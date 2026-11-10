---
title: Checkbox
description: A check box, checked, unchecked or indeterminate.
group: Components
order: 34
---

`ui.Checkbox` is a check box in `ui`'s look, with a label beside it. It holds no value: the app says whether it is checked and updates that in `OnChange`.

```go preview=checkbox/basic
done := core.NewSignal(cx, false)
ui.Checkbox{Checked: done.Get, OnChange: done.Set, Text: "Remember me"}
```

## Content

`ui.Checkbox` shows `Text` or the app's `Children` beside its box. `Label` replaces both for assistive technology, and `Hint` says what it is for.

## States

`Indeterminate` is neither checked nor unchecked, for a "select all" over a partial selection; a press then asks for checked. `Disabled` stops presses and the focus.

```go preview=checkbox/indeterminate height=220
ui.Checkbox{
	Checked:       func() bool { return chosen.Get() == len(files) },
	Indeterminate: func() bool { return chosen.Get() > 0 && chosen.Get() < len(files) },
	OnChange:      chooseAll,
	Text:          "Select all",
}
```

## Accessibility

The checkbox reports `RoleCheckbox` and its checked or indeterminate state. Its name is `Text`, `Label` or the children's text. Space toggles it when it has the keyboard focus, and a screen reader announces the new state.

## Platform

The box is drawn by the app, not a platform control, so it looks the same everywhere. The press, focus ring and screen reader are the platform's. It is tested by role and label on every target.

## Related

- [Switch](switch.md): a setting that takes effect at once.
- [Forms](../forms.md): choices in a form.
- [List](list.md): checkboxes in rows.
- The gallery's Checkbox page.
- [Unstyled components](../authoring/base.md#checkbox): `base.Checkbox`, for a box of a look of your own.
