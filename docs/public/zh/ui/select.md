---
title: 选择器
description: 从列表中选一个值，桌面端是弹出列表，触摸屏上是 sheet。
group: 组件
order: 36
---

`ui.Select[T]` 从一组任意可比较类型的值中选一个。桌面端列表在触发器下方打开；触摸屏上它是一个 sheet。

```go
size := core.NewSignal(cx, "m")
ui.Select[string]{
	Options:  []ui.Option[string]{{Value: "s", Label: "小"}, {Value: "m", Label: "中"}, {Value: "l", Label: "大"}},
	Value:    size.Get,
	OnChange: size.Set,
	Label:    "尺寸",
}
```

## 选项与搜索

`Options` 由 `ui.Option[T]` 组成，每项有 `Value`、`Label`，以及可选的 `Icon`、`Disabled` 函数或应用自己的 `Content`。未选择时显示 `Placeholder`。`Searchable` 允许输入过滤列表，`OnClear` 显示清除按钮，`Disabled` 停用它。

```go
ui.Select[string]{
	Options:           languages,
	Value:             language.Get,
	OnChange:          language.Set,
	Searchable:        true,
	SearchPlaceholder: "搜索语言",
	OnClear:           func() { language.Set("") },
	Placeholder:       "选择语言",
}
```

## 行为

焦点在触发器上时，上、下、Return 和空格打开列表；方向键移动高亮，typeahead 按项标题跳转。选择一项会关闭列表并报告值。Escape、返回和点击外部会关闭列表而不改变值。`Empty` 是搜索无结果时显示的文字。

## 无障碍

触发器和列表带有选择器的角色；`Label` 命名字段，并报告选中项。Tab 到达触发器，Return 或空格打开它。

## 平台

在 macOS、Windows 和 Linux 上，列表是触发器旁的弹出层。在 iOS 和 Android 上它是 sheet，选中的行留在字段上。选项和值在每个目标上相同。

## 相关

- [表单](../forms.md)：表单中的选择器。
- [菜单](menu.md)：命令而不是值。
- [标签页](tabs.md)：同时显示的少量选项。
- gallery 的 Select 和 Combobox 页面。
- [无样式组件](../authoring/base.md#select)：`base.Select`，带分组和自定义触发器。
