---
title: 菜单
description: 从触发器打开的菜单、上下文菜单及其条目。
group: 组件
order: 71
---

`base.Menu` 从触发器打开 `base.MenuItem`：桌面端在触发器旁，触摸屏上作为 sheet。`ui.Menu` 使用 `ui.Button` 作为触发器的同一形式；`base.ContextMenu` 在指针处打开同样的条目。

```go
open := core.NewSignal(cx, false)
base.Menu{
	Open:         open.Get,
	OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) },
	Trigger:      ui.Button{Text: "操作", Variant: ui.ButtonSecondary},
	Children: []core.Node{
		base.MenuItem{Label: "重命名", OnSelect: rename},
		base.MenuItem{Label: "复制", OnSelect: duplicate},
		base.MenuItem{Label: "删除", OnSelect: remove, Role: core.RoleMenuItem},
	},
}
```

## 条目

`base.MenuItem` 通过 `OnSelect` 报告选择，除非 `NoCloseOnSelect` 否则关闭菜单。`Disabled` 停用它，`Checked` 让它成为可勾选条目，`Indicator` 在内容前绘制应用的标记。`base.Submenu` 保存更多条目，在桌面端在条目旁打开，在触摸屏上则在 sheet 中替代周围的条目。

```go
base.MenuItem{Label: "显示侧边栏", Checked: sidebar.Get, OnSelect: func() { sidebar.Set(!sidebar.Get()) }}
base.Submenu{Label: "分享", Children: []core.Node{
	base.MenuItem{Label: "复制链接", OnSelect: copyLink},
	base.MenuItem{Label: "邮件", OnSelect: emailLink},
}}
```

`Role` 选择辅助技术听到的角色：菜单项、复选框或单选项。`NoCloseOnSelect` 让就地修改值的条目保持菜单打开。

## 带样式的菜单

`ui.Menu` 接受 `ui.Button` 触发器和同样的条目；`MaxHeight` 让条目超过高度后滚动。触发器自己的 `OnPress` 和 `Disabled` 属于菜单，因此不会运行。

```go
ui.Menu{Trigger: ui.Button{Text: "排序", Variant: ui.ButtonGhost, Icon: lucide.ArrowUpDown}, Children: []core.Node{
	base.MenuItem{Label: "名称", Checked: func() bool { return sort.Get() == "name" }, OnSelect: func() { sort.Set("name") }},
	base.MenuItem{Label: "日期", Checked: func() bool { return sort.Get() == "date" }, OnSelect: func() { sort.Set("date") }},
}}
```

## 上下文菜单

`base.ContextMenu` 在右键或触摸屏长按时，在指针处打开条目，作用于打开它的行或视图。

## 行为

Escape、返回和点击外部会关闭菜单，并通知 `OnCancel`。桌面端菜单是由 `Placement`、`Align`、`Offset` 和 `Margin` 放置的弹出层；触摸屏上是底部 sheet。键盘在条目间移动，打开子菜单并用 Return 选择。

## 无障碍

菜单报告 `RoleMenu`，每个条目报告 `RoleMenuItem`，或它采用的复选框、单选项角色。触发器内容不足以命名时用 `Label`。屏幕阅读器朗读条目的标签、勾选状态和是否禁用。

## 平台

触发器和条目由应用绘制；sheet、弹出层、焦点、Escape 和返回来自平台。桌面弹出层在触发器旁，手机上则用相同的条目显示 sheet。

## 相关

- [选择器](select.md)：从列表中选择一个值。
- [对话框](dialog.md)：模态表面。
- [标签页](tabs.md)：同时显示的分区。
- gallery 的 Menu、ContextMenu 和 Command 页面。
