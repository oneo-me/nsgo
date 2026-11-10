---
title: 对话框
description: 模态表面、sheet 和系统警告框。
group: 组件
order: 70
---

`ui.Dialog` 是变暗的应用上方的模态表面，在触摸屏上从底部出现成一个 sheet。`base.Dialog` 是同一个层没有外观的形式，`base.Sheet` 则是从某一侧出现。

```go
open := core.NewSignal(cx, false)
ui.Dialog{
	Open:         open.Get,
	OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) },
	Title:        "重命名文件",
	Description:  "新名称会显示在所有地方。",
	Children:     []core.Node{ui.Input{Input: base.Input{Value: name.Get, OnChange: name.Set}}},
	Footer: []core.Node{
		ui.Button{Text: "取消", Variant: ui.ButtonSecondary, OnPress: func() { open.Set(false) }},
		ui.Button{Text: "保存", OnPress: save},
	},
}
```

## 基础对话框

`base.Dialog` 接受背景、层和弹出层的样式，一个 `Animation` 或 `TransitionShader`，以及应用的子元素。`Open`、`DefaultOpen` 和 `OnOpenChange` 控制它；`OnOpen`、`OnClose` 和 `OnCancel` 报告发生的事情。

`NoCloseOnEscape` 和 `NoCloseOnBackdropPress` 阻止 Escape 或点击外部关闭；`NoCloseOnSwipe` 对 iOS 的滑动关闭做同样的事。`Presentation` 选择 iOS 的呈现方式，`StatusBar` 在打开时改变状态栏。

## Sheet 与警告框

`base.Sheet` 是某一侧的面板，带背景和应用的内容；设置 `CloseOnSwipe` 后，底部 sheet 会跟随向下拖动，并在超过阈值时关闭。`base.ShowAlertDialog` 显示系统的警告框或操作表，按钮按角色区分。

```go
base.ShowAlertDialog(cx, base.AlertDialog{
	Title: "删除这条笔记？",
	Actions: []core.AlertAction{
		{Label: "取消", Role: core.AlertCancel},
		{Label: "删除", Role: core.AlertDestructive},
	},
}, func(label string) { if label == "删除" { remove() } })
```

`base.AlertDialog` 是那个警告框的内容，由平台绘制。`base.Popover` 和 `base.Menu` 是锚定表面，不是模态。

## 行为

模态会挡住后面的应用，取得键盘焦点，除非另有设置，Escape 和返回会关闭它。sheet 会跟随向下拖动，超过阈值后关闭。`OnOpenChange` 收到 `base.Reason`，应用可以区分点击外部和按钮。

## 无障碍

对话框报告 `RoleDialog`，朗读标题，标题不够时用 `Label`。打开时焦点进入其中，关闭时返回；背景不是停靠点。底部的按钮保留自己的角色和标签。

## 平台

在 iOS 上，对话框可以是全屏、page sheet 或 form sheet，sheet 跟随系统的滑动。在 Android 和触摸屏上它是底部 sheet；在桌面上它是带背景的居中表面。内容相同，只有呈现方式变化。

## 相关

- [导航](../navigation.md)：页面、标签页和模态一起使用。
- [菜单](menu.md)：锚定命令和上下文菜单。
- [无障碍](../accessibility.md)：焦点和对话框角色。
- gallery 的 Dialog、Sheet 和 AlertDialog 页面。
