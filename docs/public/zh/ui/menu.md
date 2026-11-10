---
title: 菜单
description: 从触发器打开的菜单、上下文菜单及其条目。
group: 组件
order: 42
---

`ui.Menu` 从一个按钮打开 `ui.MenuItem`：桌面上显示在按钮旁边，触摸屏上显示为 sheet。`ui.ContextMenu` 在指针处打开同样的条目。

```go
ui.Menu{Trigger: ui.Button{Text: "操作", Variant: ui.ButtonSecondary}, Children: []core.Node{
	ui.MenuItem{Text: "重命名", Icon: lucide.Pencil, OnSelect: rename},
	ui.MenuItem{Text: "复制一份", Shortcut: "mod+d", OnSelect: duplicate},
	ui.MenuSeparator{},
	ui.MenuItem{Text: "删除", Variant: ui.MenuItemDanger, OnSelect: remove},
}}
```

菜单自己打开和关闭；`Open` 和 `OnOpenChange` 让应用保存它的状态。`Trigger` 是按钮的外观和内容，按压由菜单接管。`MaxHeight` 让条目超过一定高度后滚动。

## 条目

`ui.MenuItem` 显示 `Text`、可选的 `Icon` 和 `Shortcut`（写作 `"mod+c"`，桌面上按平台的方式显示），并报告 `OnSelect`，除非设置 `NoCloseOnSelect`，否则选择后菜单关闭。`Disabled` 禁用它，`Checked` 在旁边显示勾选，`Variant: ui.MenuItemDanger` 标记危险操作。`ui.MenuLabel` 作为一组的标题，`ui.MenuSeparator` 分隔各组。`ui.Submenu` 包含更多条目，在条目旁边打开，在触摸屏上则在 sheet 中替换周围的条目。

```go
ui.Menu{Trigger: ui.Button{Text: "显示", Variant: ui.ButtonGhost}, Children: []core.Node{
	ui.MenuLabel{Text: "排序方式"},
	ui.MenuItem{Text: "名称", Checked: func() bool { return sort.Get() == "name" }, OnSelect: func() { sort.Set("name") }},
	ui.MenuItem{Text: "日期", Checked: func() bool { return sort.Get() == "date" }, OnSelect: func() { sort.Set("date") }},
	ui.MenuSeparator{},
	ui.Submenu{Text: "分享", Children: []core.Node{
		ui.MenuItem{Text: "复制链接", OnSelect: copyLink},
		ui.MenuItem{Text: "邮件", OnSelect: emailLink},
	}},
}}
```

## 上下文菜单

`ui.ContextMenu` 在次级点击或触摸屏上的长按时，于指针处打开 `Trigger` 的条目。

```go
ui.ContextMenu{Trigger: fileRow, Children: []core.Node{
	ui.MenuItem{Text: "打开", OnSelect: open},
	ui.MenuItem{Text: "删除", Variant: ui.MenuItemDanger, OnSelect: remove},
}}
```

## 行为

Escape、返回和点击外部会关闭菜单。桌面上菜单是由 `Placement` 和 `Align` 定位的弹出层；触摸屏上是从底部弹出的 sheet。键盘可以在条目间移动、打开子菜单并用回车选择。

## 无障碍

菜单报告 `RoleMenu`，每个条目报告 `RoleMenuItem`。触发器的内容不足以说明时，用 `Label` 命名。读屏器读出条目的文字以及它是否被禁用。

## 平台

触发器和条目属于应用；sheet、弹出层、焦点、Escape 和返回属于平台。桌面弹出层位于触发器旁边，手机则用 sheet 显示同样的条目。

## 相关

- [选择器](select.md)：从列表中选择一个值。
- [对话框](dialog.md)：模态表面。
- [标签页](tabs.md)：同时显示的分区。
- gallery 的 Menu、ContextMenu 和 Command 页面。
- [无样式组件](../authoring/base.md#menu)：`base.Menu`，用于自己设计外观的菜单。
