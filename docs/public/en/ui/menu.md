---
title: Menu
description: A menu from a trigger, a context menu and their items.
group: Components
order: 42
---

`ui.Menu` opens `ui.MenuItem`s from a button: beside it on desktops, as a sheet on touch screens. `ui.ContextMenu` opens the same items at the pointer.

```go
ui.Menu{Trigger: ui.Button{Text: "Actions", Variant: ui.ButtonSecondary}, Children: []core.Node{
	ui.MenuItem{Text: "Rename", Icon: lucide.Pencil, OnSelect: rename},
	ui.MenuItem{Text: "Duplicate", Shortcut: "mod+d", OnSelect: duplicate},
	ui.MenuSeparator{},
	ui.MenuItem{Text: "Delete", Variant: ui.MenuItemDanger, OnSelect: remove},
}}
```

The menu opens and closes itself; `Open` and `OnOpenChange` let the app hold it. `Trigger` is the button's look and content, and the menu takes its presses. `MaxHeight` scrolls the items past a height.

## Items

A `ui.MenuItem` shows `Text`, an optional `Icon` and a `Shortcut` (`"mod+c"`, written the platform's way on desktops), and reports `OnSelect`, closing the menu unless `NoCloseOnSelect`. `Disabled` turns it off, `Checked` shows a check beside it, and `Variant: ui.MenuItemDanger` marks a destructive one. `ui.MenuLabel` heads a group and `ui.MenuSeparator` divides them. A `ui.Submenu` holds more items and opens beside the item or, on touch screens, in the sheet in place of the ones around it.

```go
ui.Menu{Trigger: ui.Button{Text: "View", Variant: ui.ButtonGhost}, Children: []core.Node{
	ui.MenuLabel{Text: "Sort by"},
	ui.MenuItem{Text: "Name", Checked: func() bool { return sort.Get() == "name" }, OnSelect: func() { sort.Set("name") }},
	ui.MenuItem{Text: "Date", Checked: func() bool { return sort.Get() == "date" }, OnSelect: func() { sort.Set("date") }},
	ui.MenuSeparator{},
	ui.Submenu{Text: "Share", Children: []core.Node{
		ui.MenuItem{Text: "Copy link", OnSelect: copyLink},
		ui.MenuItem{Text: "Email", OnSelect: emailLink},
	}},
}}
```

## Context menus

`ui.ContextMenu` opens its items on a secondary click, or a long press on touch screens, at the pointer over its `Trigger`.

```go
ui.ContextMenu{Trigger: fileRow, Children: []core.Node{
	ui.MenuItem{Text: "Open", OnSelect: open},
	ui.MenuItem{Text: "Delete", Variant: ui.MenuItemDanger, OnSelect: remove},
}}
```

## Behavior

Escape, back and a press outside close a menu. On desktops the menu is a popup placed by `Placement` and `Align`; on touch screens it is a sheet from the bottom. The keyboard moves through items, opens submenus and chooses with Return.

## Accessibility

The menu reports `RoleMenu` and each item `RoleMenuItem`. `Label` names the trigger when its content does not. A screen reader reads an item's text and whether it is disabled.

## Platform

The trigger and items are the app's; the sheet, popup, focus, Escape and back are the platform's. A desktop popup sits beside the trigger, where a phone shows a sheet with the same items.

## Related

- [Select](select.md): a value from a list.
- [Dialog](dialog.md): a modal surface.
- [Tabs](tabs.md): sections shown at once.
- The gallery's Menu, ContextMenu and Command pages.
- [Unstyled components](../authoring/base.md#menu): `base.Menu`, for a menu of a look of your own.
