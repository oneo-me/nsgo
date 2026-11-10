---
title: 选择器
description: 从列表中选一个值，桌面端是弹出列表，触摸屏上是 sheet。
group: 组件
order: 65
---

`ui.Select[T]` 从一组值中选一个；`base.Select` 是同一个控件没有外观的形式，值为字符串。桌面端列表在触发器下方打开；触摸屏上它是一个 sheet。

```go
size := core.NewSignal(cx, "m")
ui.Select[string]{
	Options:  []ui.Option[string]{{Value: "s", Label: "小"}, {Value: "m", Label: "中"}, {Value: "l", Label: "大"}},
	Value:    size.Get,
	OnChange: size.Set,
	Label:    "尺寸",
}
```

## 基础选择器

`base.Select` 接受 `base.SelectItem` 和 `*string` 值，因此应用可以用 `nil` 表示“未选择”。触发器、弹出层、行和分组的外观都由应用设置。

```go
choice := core.NewSignal[*string](cx, nil)
base.Select{
	Items: func() []base.SelectItem {
		return []base.SelectItem{{Value: "s", Title: "小"}, {Value: "m", Title: "中"}}
	},
	Value:      choice.Get,
	OnChange:   choice.Set,
	Label:      "尺寸",
	Style:      core.Style{Height: core.Pt(28), Padding: core.Symmetric(4, 10), BorderWidth: 1, CornerRadius: 6},
	PopupStyle: core.Style{Padding: core.All(4), CornerRadius: 8},
	ItemStyle:  core.Style{Padding: core.Symmetric(4, 8), CornerRadius: 4},
}
```

## 带样式的选择器

`ui.Select[T]` 接受 `ui.Option[T]` 组成的 `Options`、`Value`、`OnChange`、`Placeholder` 和 `Label`。`Searchable` 允许输入过滤列表，`OnClear` 显示清除按钮，`Disabled` 停用它。

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

`base.Select` 还会把相邻的同 `Group` 项放在一个表头下，按标题进行 typeahead，并允许应用用 `SearchField` 绘制搜索框。

## 行为

焦点在触发器上时，上、下、Return 和空格打开列表；方向键移动高亮，typeahead 按项标题跳转。选择一项会关闭列表并报告值。Escape、返回和点击外部取消，`OnOpenChange` 和 `OnCancel` 会收到。

`Cleanable` 在基础形式上显示清除按钮。`Empty` 是搜索无结果时显示的内容。

## 无障碍

触发器和列表带有选择器的角色；`Label` 命名字段，并报告选中项。应用自己的触发器如果没有文字，需要给出可见名称。Tab 到达触发器，Return 或空格打开它。

## 平台

在 macOS、Windows 和 Linux 上，列表是触发器旁的弹出层。在 iOS 和 Android 上它是 sheet，选中的行留在字段上。选项和值在每个目标上相同。

## 相关

- [表单](../forms.md)：表单中的选择器。
- [菜单](menu.md)：命令而不是值。
- [标签页](tabs.md)：同时显示的少量选项。
- gallery 的 Select 和 Combobox 页面。
