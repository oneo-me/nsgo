---
title: Navigation
description: Native navigation stacks, system headers, modals, menus and tab bars.
group: Building UI
order: 17
---

## A stack

`base.Nav` keeps a stack of routes of one type: `Push` adds a page, `Pop` removes the top page, `PopToRoot` returns to the first and `Replace` swaps the top. Its `Stack` renders every route as a page, keeping the pages below mounted with their state while the top one shows.

```go
nav := base.NewNav(cx, "inbox")

func Inbox(cx *core.Scope) core.Node {
	return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, depth func() int) base.NavPage {
		if route == "inbox" {
			return base.NavPage{Title: "Inbox", Children: []core.Node{
				ui.Button{Text: "Open a message", OnPress: func() { nav.Push("message") }},
			}}
		}
		return base.NavPage{Title: "Message", Children: []core.Node{
			ui.Button{Text: "Back", Variant: ui.ButtonSecondary, OnPress: func() { nav.Pop(1) }},
		}}
	})
}
```

On iOS and Android the platform draws the system's header from the page's `Title` and options, with its back button and its transition; on macOS, Windows, Linux and the web the page draws `Header` when it sets one. `HeaderLargeTitle` asks for iOS's large title, `HeaderTransparent` lays content under the header, `HeaderHidden` removes it, and `HeaderLeft`, `HeaderTitle` and `HeaderRight` place the app's own views in it. `HeaderStyle` and `HeaderClass` color the header: `Background`, `Color` for the title and `TintColor` for its buttons.

`NoPopOnBack` keeps a page when back is pressed, `OnBackPress` hears the press and `NoPopOnSwipe` turns off iOS's swipe back. `cx.OnBackPress(func() bool)` gives the app a handler before the stack, for unsaved changes. `OnDismiss` reports that the user popped the page, and the stack removes it.

## Modals

`base.Dialog` shows a surface over a dimmed app, a sheet from the bottom on touch screens, and `base.Sheet` a panel at one edge. Each takes `Open`, `OnOpenChange`, its content and its motion, and closes on Escape and a press on its backdrop unless told not to.

```go
base.Dialog{
	Open:         dialog.Get,
	OnOpenChange: func(open bool, _ base.Reason) { dialog.Set(open) },
	Title:        "Rename",
	Description:  "The new name shows everywhere.",
	Children:     []core.Node{ui.Input{Input: base.Input{Value: name.Get, OnChange: name.Set}}},
	Footer: []core.Node{
		ui.Button{Text: "Cancel", Variant: ui.ButtonSecondary, OnPress: func() { dialog.Set(false) }},
		ui.Button{Text: "Save", OnPress: save},
	},
}
```

`base.ShowAlertDialog` shows the system's alert or action sheet with buttons by role (`core.AlertCancel`, `core.AlertDestructive`). `base.AlertDialog`, `base.Popover` and `base/toast` cover the rest of the system's surfaces. `ui.Dialog`, `ui.Sheet` and `ui.AlertDialog` are styled forms of the same.

## Menus

`base.Menu` opens `base.MenuItem`s from a trigger: beside it on desktops, as a sheet on touch screens. An item reports `OnSelect`, closes the menu unless `NoCloseOnSelect`, and can be checkable with `Checked`. `base.ContextMenu` does the same for a secondary click or a long press, and `base.Popover`, `base.HoverCard` and `base.Tooltip` show other anchored content.

```go
base.Menu{Trigger: ui.Button{Text: "Actions", Variant: ui.ButtonSecondary}, Children: []core.Node{
	base.MenuItem{Label: "Rename", OnSelect: rename},
	base.MenuItem{Label: "Duplicate", OnSelect: duplicate},
	base.MenuItem{Label: "Delete", OnSelect: delete, Role: core.RoleMenuItem},
}}
```

## Tabs

`ui.Tabs[T]` is a segmented control or a tab list, with an indicator that slides to the chosen option. `ui.Sidebar` holds sections and rows for a desktop sidebar, and `ui.TabShell` puts the same tabs in a sidebar on wide windows and a bar on narrow ones.

`mobile.TabBar` is the system's tab bar on iOS and Android, with its Liquid Glass or Material look; it takes `Items` of `mobile.TabBarItem{Title, Icon}`, the selected index and `OnChange`. Elsewhere it takes no room, so an app shows `ui.Tabs` there. `mobile.IsAvailable(cx)` says whether the bar shows.

```go
mobile.TabBar{
	Items: []mobile.TabBarItem{{Title: "Home", Icon: "house"}, {Title: "Search", Icon: "magnifyingglass"}},
	Selected: func() int { return tab.Get() },
	OnChange: func(i int) { tab.Set(i) },
}
```

## Next

- [Forms](forms.md): what a page of a stack holds.
- [Animation](animation.md): how pages and modals move.
- [Layout](layout.md): the safe area around a header.
