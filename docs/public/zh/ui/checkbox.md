---
title: 复选框
description: 选中、未选中或不确定的复选控件。
group: 组件
order: 34
---

`ui.Checkbox` 是 `ui` 外观的复选框，旁边带标签。它不保存值：应用说明它是否选中，并在 `OnChange` 中更新。

```go preview=checkbox/basic
done := core.NewSignal(cx, false)
ui.Checkbox{Checked: done.Get, OnChange: done.Set, Text: "记住我"}
```

## 内容

`ui.Checkbox` 在框旁显示 `Text` 或应用的 `Children`。`Label` 为辅助技术替换两者，`Hint` 说明它的用途。

## 状态

`Indeterminate` 既非选中也非未选中，用于部分选中时的“全选”；此时按下会请求选中。`Disabled` 阻止按压和焦点。

```go preview=checkbox/indeterminate height=220
ui.Checkbox{
	Checked:       func() bool { return chosen.Get() == len(files) },
	Indeterminate: func() bool { return chosen.Get() > 0 && chosen.Get() < len(files) },
	OnChange:      chooseAll,
	Text:          "全选",
}
```

## 无障碍

复选框报告 `RoleCheckbox` 及其选中或不确定状态。它的名称是 `Text`、`Label` 或子元素的文字。获得键盘焦点时空格切换它，读屏器会读出新状态。

## 平台

框由应用绘制而不是平台控件，所以各处看起来一样。按压、焦点环和读屏器来自平台。它在每个目标上都按角色和标签测试。

## 相关

- [开关](switch.md)：立即生效的设置。
- [表单](../forms.md)：表单中的选择。
- [列表](list.md)：行中的复选框。
- gallery 的 Checkbox 页面。
- [无样式组件](../authoring/base.md#checkbox)：`base.Checkbox`，用于自己设计外观的复选框。
