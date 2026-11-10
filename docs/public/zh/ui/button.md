---
title: 按钮
description: 普通按钮、变体与尺寸，以及图标按钮。
group: 组件
order: 60
---

`base.Button` 是一个带按钮角色、没有外观的可按压盒子。`ui.Button` 是同一个控件在 Onevra 外观下的形式，带窗口需要的各种变体。

```go
ui.Button{Text: "保存", OnPress: save}
ui.Button{Text: "取消", Variant: ui.ButtonSecondary, OnPress: cancel}
```

## 基础按钮

`base.Button` 从 `Style` 取得外观，从 `Children` 取得内容。`OnPress` 在完整的按压后运行；`OnPressDown` 和 `OnPressUp` 跟随按压的开始和结束，用于应用自己的反馈。

```go
base.Button{
	Label:   "保存",
	OnPress: func(core.PressEvent) { save() },
	Style: core.Style{Background: core.RGB(0x007aff), Padding: core.Symmetric(8, 14), CornerRadius: 8, AlignSelf: core.AlignStart},
	Children: []core.Node{core.Text{Content: "保存", Style: core.Style{Color: core.RGB(0xffffff)}}},
}
```

`Disabled`、`Selected` 和 `Expanded` 向应用、样式和辅助技术报告状态。开关按钮或列表的当前项使用 `Selected`；展开控件使用 `Expanded`。

## 变体与尺寸

`ui.Button.Variant` 是 `ButtonPrimary`（默认）、`ButtonSecondary`、`ButtonOutline` 或 `ButtonGhost`。`Size` 是 `SizeDefault`（28 点）、`SizeSmall`（24）或 `SizeLarge`（32）。

```go
ui.Button{Text: "删除", Variant: ui.ButtonOutline, OnPress: remove}
ui.Button{Text: "继续", Size: ui.SizeLarge, OnPress: next}
ui.Button{Text: "保存", Icon: lucide.Check, Flex: 1, OnPress: save}
```

`Color` 用应用的颜色填充按钮，替代变体的填充。`Flex` 让它在行中占据一份空间；`TestID` 为测试命名。

只有图标的按钮是 `ui.IconButton`，`Variant` 为 `IconButtonGhost` 或 `IconButtonRaised`，`Size` 以点为单位，并需要给辅助技术读的 `Label`。带 `Icon` 但没有 `Text` 的 `ui.Button` 也是方形，同样需要 `Label`。

```go
ui.IconButton{Icon: lucide.Trash, Label: "删除", Variant: ui.IconButtonGhost, OnPress: remove}
```

## 内容与状态

`ui.Button` 显示 `Text`、可选的前置 `Icon`，或两者。基础形式接受任意子元素，因此应用自己的图标和文字可以是一行 `core.View` 和 `core.Text`。

禁用的按钮不接受按压，也不进入键盘焦点。它的外观是禁用状态，而不是应用另选的一种颜色。`Style.Active` 和 `Style.Hover` 为基础形式添加应用自己的反馈。

## 无障碍

按钮报告 `RoleButton`，以文字为名称，`Hint` 说明按下它会做什么。`Label` 会替代文字供辅助技术读取。没有 `Label` 的图标按钮无法被读出，因此必须命名。

在 macOS、Web 和 Apple TV 上，Return 和空格按下获得焦点的按钮；手机的屏幕阅读器会把按压当作点击。`NoFocus` 在行或列表用其他方式到达按钮时，把它移出 Tab 停靠点。

## 平台

每个设备上的尺寸相同。`base.Button` 是应用自己的视图加按钮角色，不是平台按钮，因此外观完全由应用决定；按压、焦点和无障碍来自平台。

## 相关

- [表单](../forms.md)：表单中的按钮和校验。
- [样式与主题](../styles.md)：基础按钮采用的状态和颜色。
- [无障碍](../accessibility.md)：角色、标签和焦点。
- gallery 的 Button 页面，包含 `base` 和 `ui` 两种形式。
