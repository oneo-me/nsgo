---
title: 复选框
description: 选中、未选中或不确定的复选控件。
group: 组件
order: 63
---

`base.Checkbox` 是没有外观的复选框；`ui.Checkbox` 是同一个控件在 Onevra 外观下的形式，带复选框和标签。它不保存值：应用说明它是否选中，并在 `OnChange` 中更新。

```go
done := core.NewSignal(cx, false)
ui.Checkbox{Checked: done.Get, OnChange: done.Set, Text: "记住我"}
```

## 基础复选框

`base.Checkbox` 接受盒子的样式、指示器的样式，以及应用放在指示器里的标记。标记由应用根据自己保存的值显示。

```go
base.Checkbox{
	Checked:  done.Get,
	OnChange: done.Set,
	Label:    "记住我",
	Style:          core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8},
	IndicatorStyle: core.Style{Width: core.Pt(18), Height: core.Pt(18), BorderWidth: 1, CornerRadius: 5},
	Indicator:      core.Icon{SVG: lucide.Check, Style: core.Style{FontSize: 14}},
	Children: []core.Node{core.Text{Content: "记住我"}},
}
```

`Indeterminate` 表示既非选中也非未选中，用于部分选择时的“全选”；此时按压会请求选中。`Disabled` 阻止按压和焦点。

## 内容

`ui.Checkbox` 在框旁显示 `Text` 或应用的 `Children`。`Label` 会替代两者供辅助技术读取，`Hint` 说明它的用途。复选框放在行或列表中时，通常也由行处理按压；给框一个 `HitSlop` 可以让整个区域更容易按到。

## 状态

框的选中状态会同时反映给辅助技术和主题。`Style`、`StyleFunc` 和状态样式添加应用自己的外观，指示器可以跟随框的状态，而不会成为第二个停靠点。

## 无障碍

复选框报告 `RoleCheckbox` 以及选中或不确定状态。它的名称是 `Text`、`Label` 或子元素的文字。获得键盘焦点时，空格切换它，屏幕阅读器会朗读新状态。

## 平台

框由应用绘制，不是平台控件，因此在所有平台上外观相同。按压、焦点环和屏幕阅读器来自平台。每个目标都可以按角色和标签测试它。

## 相关

- [开关](switch.md)：立即生效的设置。
- [表单](../forms.md)：表单中的选择。
- [列表](list.md)：行中的复选框。
- gallery 的 Checkbox 页面，包含 `base` 和 `ui` 两种形式。
