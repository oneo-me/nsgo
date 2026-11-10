---
title: Tabs
description: A tab list, a segmented control and the system tab bar.
group: Components
order: 40
---

`ui.Tabs[T]` chooses one of a few values, as a segmented control or a tab list; the app shows the chosen value's content beside it. A tab list is one Tab stop: the arrows move through the tabs.

```go
section := core.NewSignal(cx, "overview")
ui.Tabs[string]{
	Options: []ui.Option[string]{
		{Value: "overview", Label: "Overview"},
		{Value: "activity", Label: "Activity"},
		{Value: "settings", Label: "Settings"},
	},
	Value:    section.Get,
	OnChange: section.Set,
	Label:    "Sections",
}
```

## Variants and sizes

`Variant` is `TabsSegmented` (the default, a raised thumb in a recessed track) or `TabsUnderline` (an accent line under the selected tab). `Size` is `SizeDefault`, `SizeSmall` or `SizeLarge`, and `Equal` gives every tab the same width instead of fitting its label.

```go
ui.Tabs[string]{Variant: ui.TabsUnderline, Equal: true, Size: ui.SizeSmall, Options: tabs, Value: section.Get, OnChange: section.Set}
```

An option can carry an `Icon`, a `Disabled` function, a `Content` node and a `TestID`. `Content` replaces the label and inherits the tab's text style.

## Tab bars

`ui.TabShell` puts the same tabs in a sidebar on wide windows and a bar on narrow ones. `mobile.TabBar` is the system's tab bar on iOS and Android, with `mobile.TabBarItem{Title, Icon}`, `Selected` and `OnChange`; elsewhere it takes no room, so show `ui.Tabs` there.

## Accessibility

The list reports `RoleTabList`, each tab `RoleTab`, and the selected tab its state. The list is one Tab stop; Left and Up, Right and Down, Home and End move between enabled tabs, and the tab reached reports a press. `Label` names the list.

## Platform

iOS and Android show the system's tab bar with their own look and gestures; macOS, Windows, Linux and the web show `ui.Tabs`. The app's content is the same on every target.

## Related

- [Navigation](../navigation.md): pages and tab bars in an app.
- [Menu](menu.md): commands rather than sections.
- [Select](select.md): many options in a list.
- The gallery's Tabs and NavStack pages.
- [Unstyled components](../authoring/base.md#tabs): `base.Tabs`, for tabs of a look of your own.
