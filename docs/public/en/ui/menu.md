---
title: Menu
description: A menu from a trigger, a context menu and their items.
group: Components
order: 71
---

`base.Menu` opens `base.MenuItem`s from a trigger: beside it on desktops, as a sheet on touch screens. `ui.Menu` is the same with a `ui.Button` trigger; `base.ContextMenu` opens the same items at the pointer.

```go
open := core.NewSignal(cx, false)
base.Menu{
	Open:         open.Get,
	OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) },
	Trigger:      ui.Button{Text: "Actions", Variant: ui.ButtonSecondary},
	Children: []core.Node{
		base.MenuItem{Label: "Rename", OnSelect: rename},
		base.MenuItem{Label: "Duplicate", OnSelect: duplicate},
		base.MenuItem{Label: "Delete", OnSelect: remove, Role: core.RoleMenuItem},
	},
}
```

## Items

A `base.MenuItem` reports `OnSelect` and closes the menu unless `NoCloseOnSelect`. `Disabled` turns it off, `Checked` makes it a checkable item, and `Indicator` draws the app's mark before its content. A `base.Submenu` holds more items and opens beside the item or, on touch screens, in the sheet in place of the ones around it.

```go
base.MenuItem{Label: "Show sidebar", Checked: sidebar.Get, OnSelect: func() { sidebar.Set(!sidebar.Get()) }}
base.Submenu{Label: "Share", Children: []core.Node{
	base.MenuItem{Label: "Copy link", OnSelect: copyLink},
	base.MenuItem{Label: "Email", OnSelect: emailLink},
}}
```

`Role` chooses what assistive technology hears: a menu item, a checkbox or a radio item. `NoCloseOnSelect` keeps the menu open for a item that changes a value in place.

## The styled menu

`ui.Menu` takes a `ui.Button` trigger and the same items; `MaxHeight` scrolls the items past a height. The trigger's `OnPress` and `Disabled` are the menu's, so its own do not run.

```go
ui.Menu{Trigger: ui.Button{Text: "Sort", Variant: ui.ButtonGhost, Icon: lucide.ArrowUpDown}, Children: []core.Node{
	base.MenuItem{Label: "Name", Checked: func() bool { return sort.Get() == "name" }, OnSelect: func() { sort.Set("name") }},
	base.MenuItem{Label: "Date", Checked: func() bool { return sort.Get() == "date" }, OnSelect: func() { sort.Set("date") }},
}}
```

## Context menus

`base.ContextMenu` opens its items on a secondary click, or a long press on touch screens, at the pointer, for the row or the view it was opened on.

## Behavior

Escape, back and a press outside close a menu and reach `OnCancel`. On desktops the menu is a popup placed by `Placement`, `Align`, `Offset` and `Margin`; on touch screens it is a sheet from the bottom. The keyboard moves through items, opens submenus and chooses with Return.

## Accessibility

The menu reports `RoleMenu` and each item `RoleMenuItem`, or the checkbox or radio role it takes. `Label` names the trigger when its content does not. A screen reader reads an item's label, its checked state and whether it is disabled.

## Platform

The trigger and items are the app's; the sheet, popup, focus, Escape and back are the platform's. A desktop popup sits beside the trigger, where a phone shows a sheet with the same items.

## Related

- [Select](select.md): a value from a list.
- [Dialog](dialog.md): a modal surface.
- [Tabs](tabs.md): sections shown at once.
- The gallery's Menu, ContextMenu and Command pages.
