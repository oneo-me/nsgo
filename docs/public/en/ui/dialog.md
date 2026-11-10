---
title: Dialog
description: A modal surface, a sheet and the system's alert.
group: Components
order: 70
---

`ui.Dialog` is a modal surface over a dimmed app, a sheet from the bottom on touch screens. `base.Dialog` is the same layer without a look, and `base.Sheet` is a panel at one edge.

```go
open := core.NewSignal(cx, false)
ui.Dialog{
	Open:         open.Get,
	OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) },
	Title:        "Rename the file",
	Description:  "The new name shows everywhere.",
	Children:     []core.Node{ui.Input{Input: base.Input{Value: name.Get, OnChange: name.Set}}},
	Footer: []core.Node{
		ui.Button{Text: "Cancel", Variant: ui.ButtonSecondary, OnPress: func() { open.Set(false) }},
		ui.Button{Text: "Save", OnPress: save},
	},
}
```

## The base dialog

`base.Dialog` takes its backdrop, layer and popup styles, an `Animation` or a `TransitionShader`, and the app's children. `Open`, `DefaultOpen` and `OnOpenChange` control it; `OnOpen`, `OnClose` and `OnCancel` report what happened.

`NoCloseOnEscape` and `NoCloseOnBackdropPress` keep Escape and a press outside from closing it; `NoCloseOnSwipe` does the same for iOS's swipe. `Presentation` chooses the iOS presentation, and `StatusBar` changes the bar while it is open.

## Sheets and alerts

`base.Sheet` is a panel at an edge, with a backdrop and the app's content; with `CloseOnSwipe` a bottom sheet follows a downward drag and closes past a threshold. `base.ShowAlertDialog` shows the system's alert or action sheet with buttons by role.

```go
base.ShowAlertDialog(cx, base.AlertDialog{
	Title: "Delete the note?",
	Actions: []core.AlertAction{
		{Label: "Cancel", Role: core.AlertCancel},
		{Label: "Delete", Role: core.AlertDestructive},
	},
}, func(label string) { if label == "Delete" { remove() } })
```

`base.AlertDialog` is the content of that alert, with the platform drawing it. `base.Popover` and `base.Menu` are anchored surfaces rather than modals.

## Behavior

A modal blocks the app behind it, takes the keyboard focus and closes on Escape and back unless told not to. A sheet follows a downward drag and closes past a threshold. `OnOpenChange` receives a `base.Reason`, so the app can tell a press outside from a button.

## Accessibility

The dialog reports `RoleDialog` and reads its title, or `Label` when the title is not enough. Focus moves into it when it opens and returns when it closes; the backdrop is not a stop. The footer's buttons keep their roles and labels.

## Platform

On iOS a dialog can be a full screen, page sheet or form sheet, and a sheet follows the system's swipe. On Android and touch screens it is a sheet from the bottom; on desktops it is a centered surface with a backdrop. The content is the same; only the presentation changes.

## Related

- [Navigation](../navigation.md): pages, tabs and modals together.
- [Menu](menu.md): anchored commands and context menus.
- [Accessibility](../accessibility.md): focus and dialog roles.
- The gallery's Dialog, Sheet and AlertDialog pages.
