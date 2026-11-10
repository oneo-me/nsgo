---
title: 按钮
description: 普通按钮、变体与尺寸，以及图标按钮。
group: 组件
order: 31
---

`ui.Button` 是 `ui` 外观的按钮，带窗口需要的各种变体和尺寸。`OnPress` 在完整的按压后运行。

```go preview=button/basic
ui.Button{Text: "保存", OnPress: save}
ui.Button{Text: "取消", Variant: ui.ButtonSecondary, OnPress: cancel}
```

## 变体与尺寸

`ui.Button.Variant` 可以是 `ButtonPrimary`（默认）、`ButtonSecondary`、`ButtonOutline` 或 `ButtonGhost`。`Size` 可以是 `SizeDefault`（28 点）、`SizeSmall`（24）或 `SizeLarge`（32）。

```go preview=button/variants
ui.Button{Text: "删除", Variant: ui.ButtonOutline, OnPress: remove}
ui.Button{Text: "继续", Size: ui.SizeLarge, OnPress: next}
ui.Button{Text: "保存", Icon: lucide.Check, Flex: 1, OnPress: save}
```

`Color` 用应用自己的颜色代替变体的颜色填充按钮，`Tint` 设置图标的颜色。`Flex` 让它分得一行中的一份宽度；`TestID` 为测试命名。

只有图标的按钮用 `ui.IconButton`，`Variant` 为 `IconButtonGhost` 或 `IconButtonRaised`，`Size` 以点为单位，`Label` 供辅助技术读取。设置了 `Icon` 而没有 `Text` 的 `ui.Button` 也是方形的，同样需要 `Label`。

```go preview=button/icon
ui.IconButton{Icon: lucide.Trash, Label: "删除", Variant: ui.IconButtonGhost, OnPress: remove}
```

## 内容与状态

`ui.Button` 显示 `Text`、可选的前置 `Icon`，或两者都显示。

```go
ui.Button{Text: "正在保存…", Disabled: saving.Get, OnPress: save}
```

`Disabled` 接受一个函数，所以按钮可以跟随 signal：禁用的按钮既不接受按压也不获得键盘焦点，它的外观是禁用状态，而不是应用挑选的颜色。

## 无障碍

按钮报告 `RoleButton`，以文字作为名称，以 `Hint` 说明按下后会发生什么。`Label` 为辅助技术替换文字。只有图标而没有 `Label` 的按钮无法被读出，请给它命名。

在 macOS、Web 和 Apple TV 上，回车和空格会按下获得焦点的按钮；手机的读屏器把它当作点击来按下。

## 平台

按钮在每个设备上的尺寸相同。它是在各处以同样方式绘制的视图，而不是平台按钮；按压、焦点和无障碍来自平台。

## 相关

- [表单](../forms.md)：表单中的按钮和校验。
- [样式与主题](../styles.md)：按钮取用颜色的主题。
- [无障碍](../accessibility.md)：角色、标签和焦点。
- gallery 的 Button 页面。
- [无样式组件](../authoring/base.md#button)：`base.Button`，用于自己设计外观的按钮。
