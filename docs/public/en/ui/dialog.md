---
title: Dialog
description: A modal surface, a sheet and an alert.
group: Components
order: 41
---

`ui.Dialog` is a modal surface over a dimmed app, a sheet from the bottom on touch screens. `ui.Sheet` is a panel from the bottom, and `ui.AlertDialog` asks the user to confirm.

```go preview=dialog/basic height=360
open := core.NewSignal(cx, false)
ui.Dialog{
	Open:         open.Get,
	OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) },
	Title:        "Rename the file",
	Description:  "The new name shows everywhere.",
	Children:     []core.Node{ui.Input{Value: name.Get, OnChange: name.Set}},
	Footer: []core.Node{
		ui.Button{Text: "Cancel", Variant: ui.ButtonSecondary, OnPress: func() { open.Set(false) }},
		ui.Button{Text: "Save", OnPress: save},
	},
}
```

## Opening and closing

The app holds whether the dialog is open. `OnOpenChange` hears every change the dialog asks for, with a `base.Reason` that tells a press outside from Escape or back; `OnClose` runs once it has closed. `NoCloseOnEscape` and `NoCloseOnBackdropPress` keep Escape and a press outside from closing it.

`Title` and `Description` head the surface and `Footer` holds its buttons. `Bare` leaves the whole surface to `Children`, for a dialog of the app's own layout, and `Inset` sets its padding.

## Sheets

`ui.Sheet` is a panel at the bottom over a dimmed backdrop; a press on the backdrop, Escape or back calls `OnClose`.

```go preview=dialog/sheet height=320
ui.Sheet{Open: filters.Get, OnClose: func() { filters.Set(false) }, Children: filterControls}
```

## Alerts

`ui.AlertDialog` asks the user to confirm or acknowledge something. Each action closes it, and a press outside does not.

```go preview=dialog/alert height=320
ui.AlertDialog{
	Open:         confirm.Get,
	OnOpenChange: func(v bool, _ base.Reason) { confirm.Set(v) },
	Title:        "Delete the note?",
	Description:  "This cannot be undone.",
	Actions: []ui.AlertAction{
		{Text: "Cancel", Variant: ui.ButtonSecondary},
		{Text: "Delete", Color: colors.Danger, OnPress: remove},
	},
}
```

For the system's own alert or action sheet, use `cx.ShowAlert` ([Platform services](../services.md#alerts-and-the-clipboard)).

## Behavior

A modal blocks the app behind it, takes the keyboard focus and closes on Escape and back unless told not to. On touch screens a dialog is a sheet that follows a downward drag and closes past a threshold.

## Accessibility

The dialog reports `RoleDialog` and reads its title, or `Label` when the title is not enough. Focus moves into it when it opens and returns when it closes; the backdrop is not a stop. The footer's buttons keep their roles and labels.

## Platform

On Android and touch screens a dialog is a sheet from the bottom; on desktops it is a centered surface with a backdrop. The content is the same; only the presentation changes.

## Related

- [Navigation](../navigation.md): pages, tabs and modals together.
- [Menu](menu.md): anchored commands and context menus.
- [Accessibility](../accessibility.md): focus and dialog roles.
- The gallery's Dialog, Sheet and AlertDialog pages.
- [Unstyled components](../authoring/base.md#dialog): `base.Dialog` and `base.Sheet`, with iOS's presentations and motion of your own.
