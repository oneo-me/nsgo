---
title: 标签页
description: 标签列表、分段控件和系统标签栏。
group: 组件
order: 69
---

`ui.Tabs[T]` 从几个值中选一个，并在旁边显示内容；`base.Tabs` 是没有外观的标签列表，`base.Tab` 是其中一个标签。标签列表是一个 Tab 停靠点：方向键在其中移动。

```go
section := core.NewSignal(cx, "overview")
ui.Tabs[string]{
	Options: []ui.Option[string]{
		{Value: "overview", Label: "概览"},
		{Value: "activity", Label: "动态"},
		{Value: "settings", Label: "设置"},
	},
	Value:    section.Get,
	OnChange: section.Set,
	Label:    "分区",
}
```

## 变体与尺寸

`Variant` 是 `TabsSegmented`（默认，凸起的指示块在凹陷轨道中）或 `TabsUnderline`（选中标签下方的强调线）。`Size` 是 `SizeDefault`、`SizeSmall` 或 `SizeLarge`，`Equal` 让每个标签等宽，而不是按标签文字适应。

```go
ui.Tabs[string]{Variant: ui.TabsUnderline, Equal: true, Size: ui.SizeSmall, Options: tabs, Value: section.Get, OnChange: section.Set}
```

选项可以带 `Icon`、`Disabled` 函数、`Content` 节点和 `TestID`。`Content` 替代标签文字，并继承标签的文本样式。

## 基础标签页

`base.Tabs` 保存 `base.Tab`，由 `IndicatorStyle` 绘制指示器；应用保存当前选中的标签，并在列表旁显示对应内容。`base.Tab.OnPress` 在每次按压时运行，包括已选中的标签。

```go
base.Tabs{Label: "分区", IndicatorStyle: core.Style{Height: core.Pt(2), Background: core.RGB(0x007aff)}, Children: []core.Node{
	base.Tab{Label: "概览", Selected: func() bool { return section.Get() == "overview" },
		OnPress: func(core.PressEvent) { section.Set("overview") }, Children: []core.Node{core.Text{Content: "概览"}}},
	base.Tab{Label: "动态", Selected: func() bool { return section.Get() == "activity" },
		OnPress: func(core.PressEvent) { section.Set("activity") }, Children: []core.Node{core.Text{Content: "动态"}}},
}}
```

## 标签栏

`ui.TabShell` 在宽窗口把同一批标签放进侧边栏，在窄窗口放进底部栏。`mobile.TabBar` 是 iOS 和 Android 的系统标签栏，使用 `mobile.TabBarItem{Title, Icon}`、`Selected` 和 `OnChange`；其他平台上它不占空间，因此在那里显示 `ui.Tabs`。

## 无障碍

列表报告 `RoleTabList`，每个标签报告 `RoleTab`，选中的标签报告自己的状态。列表是一个 Tab 停靠点；左/上、右/下、Home 和 End 在启用的标签间移动，到达的标签会报告按压。`Label` 命名列表。

## 平台

iOS 和 Android 显示系统标签栏，带各自的外观和手势；macOS、Windows、Linux 和 Web 绘制 `base.Tabs` 和 `ui.Tabs`。应用的内容在每个目标上相同。

## 相关

- [导航](../navigation.md)：应用中的页面和标签栏。
- [菜单](menu.md)：命令而不是分区。
- [选择器](select.md)：在列表中选择多项。
- gallery 的 Tabs 和 NavStack 页面。
