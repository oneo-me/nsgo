---
title: Checkbox
description: A check box, checked, unchecked or indeterminate.
group: Components
order: 63
---

`base.Checkbox` is a check box with no look; `ui.Checkbox` is the same control with Onevra's box and a label. It holds no value: the app says whether it is checked and updates that in `OnChange`.

```go
done := core.NewSignal(cx, false)
ui.Checkbox{Checked: done.Get, OnChange: done.Set, Text: "Remember me"}
```

## The base checkbox

`base.Checkbox` takes a style for the box, one for the indicator and the app's mark inside it. The app shows the mark from the value it owns.

```go
base.Checkbox{
	Checked:  done.Get,
	OnChange: done.Set,
	Label:    "Remember me",
	Style:          core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8},
	IndicatorStyle: core.Style{Width: core.Pt(18), Height: core.Pt(18), BorderWidth: 1, CornerRadius: 5},
	Indicator:      core.Icon{SVG: lucide.Check, Style: core.Style{FontSize: 14}},
	Children: []core.Node{core.Text{Content: "Remember me"}},
}
```

`Indeterminate` is neither checked nor unchecked, for a "select all" over a partial selection; a press then asks for checked. `Disabled` stops presses and the focus.

## Content

`ui.Checkbox` shows `Text` or the app's `Children` beside its box. `Label` replaces both for assistive technology, and `Hint` says what it is for. A checkbox in a row or a list usually lets the row handle the press too; give the box a `HitSlop` so the whole area is easy to press.

## States

The box follows the checked state for assistive technology and the theme. `Style`, `StyleFunc` and the state styles add the app's own look, and the indicator can take its state from the box without becoming a second stop.

## Accessibility

The checkbox reports `RoleCheckbox` and its checked or indeterminate state. Its name is `Text`, `Label` or the children's text. Space toggles it when it has the keyboard focus, and a screen reader announces the new state.

## Platform

The box is drawn by the app, not a platform control, so it looks the same everywhere. The press, focus ring and screen reader are the platform's. It is tested by role and label on every target.

## Related

- [Switch](switch.md): a setting that takes effect at once.
- [Forms](../forms.md): choices in a form.
- [List](list.md): checkboxes in rows.
- The gallery's Checkbox page, with its `base` and `ui` forms.
