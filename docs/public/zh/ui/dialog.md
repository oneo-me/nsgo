---
title: 对话框
description: 模态表面、sheet 和警告框。
group: 组件
order: 41
---

`ui.Dialog` 是覆盖在变暗的应用上的模态表面，在触摸屏上是从底部弹出的 sheet。`ui.Sheet` 是从底部弹出的面板，`ui.AlertDialog` 请用户确认。

```go preview=dialog/basic height=360
open := core.NewSignal(cx, false)
ui.Dialog{
	Open:         open.Get,
	OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) },
	Title:        "重命名文件",
	Description:  "新名称会显示在所有地方。",
	Children:     []core.Node{ui.Input{Value: name.Get, OnChange: name.Set}},
	Footer: []core.Node{
		ui.Button{Text: "取消", Variant: ui.ButtonSecondary, OnPress: func() { open.Set(false) }},
		ui.Button{Text: "保存", OnPress: save},
	},
}
```

## 打开与关闭

对话框是否打开由应用保存。`OnOpenChange` 收到对话框请求的每次变化，附带一个 `base.Reason`，用来区分点击外部、Escape 和返回；`OnClose` 在关闭完成后运行。`NoCloseOnEscape` 和 `NoCloseOnBackdropPress` 让 Escape 和点击外部不关闭它。

`Title` 和 `Description` 位于表面顶部，`Footer` 放按钮。`Bare` 把整个表面交给 `Children`，用于应用自己布局的对话框，`Inset` 设置它的内边距。

## Sheet

`ui.Sheet` 是覆盖在变暗背景上的底部面板；点击背景、Escape 或返回会调用 `OnClose`。

```go preview=dialog/sheet height=320
ui.Sheet{Open: filters.Get, OnClose: func() { filters.Set(false) }, Children: filterControls}
```

## 警告框

`ui.AlertDialog` 请用户确认或知悉某件事。每个操作都会关闭它，点击外部则不会。

```go preview=dialog/alert height=320
ui.AlertDialog{
	Open:         confirm.Get,
	OnOpenChange: func(v bool, _ base.Reason) { confirm.Set(v) },
	Title:        "删除这条笔记？",
	Description:  "此操作无法撤销。",
	Actions: []ui.AlertAction{
		{Text: "取消", Variant: ui.ButtonSecondary},
		{Text: "删除", Color: colors.Danger, OnPress: remove},
	},
}
```

要显示系统自己的警告框或操作表，用 `cx.ShowAlert`（[平台服务](../services.md#警告框与剪贴板)）。

## 行为

模态框会阻挡后面的应用，获得键盘焦点，除非另有设置，否则按 Escape 或返回会关闭。在触摸屏上，对话框是一个 sheet，跟随向下拖动，超过阈值后关闭。

## 无障碍

对话框报告 `RoleDialog` 并读出标题，标题不够时读出 `Label`。打开时焦点移入，关闭时焦点返回；背景不是停靠点。底部按钮保留各自的角色和标签。

## 平台

在 Android 和触摸屏上，对话框是从底部弹出的 sheet；在桌面上是带背景的居中表面。内容相同，只是呈现方式不同。

## 相关

- [导航](../navigation.md)：页面、标签页和模态一起使用。
- [菜单](menu.md)：锚定命令和上下文菜单。
- [无障碍](../accessibility.md)：焦点和对话框角色。
- gallery 的 Dialog、Sheet 和 AlertDialog 页面。
- [无样式组件](../authoring/base.md#dialog)：`base.Dialog` 和 `base.Sheet`，带 iOS 的呈现方式和自定义动画。
