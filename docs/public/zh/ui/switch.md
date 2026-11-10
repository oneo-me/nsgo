---
title: 开关
description: 打开或关闭的设置，带 base 的轨道与滑块和 ui 的尺寸。
group: 组件
order: 64
---

`base.Switch` 是立即生效的设置；`ui.Switch` 是同一个控件在 Onevra 外观下的形式。它不保存值：应用保存它，并在 `OnChange` 中更新。

```go
notifications := core.NewSignal(cx, true)
ui.Switch{Checked: notifications.Get, OnChange: notifications.Set, Label: "通知"}
```

## 基础开关

`base.Switch` 的部件由应用设置样式：轨道、轨道中的滑块，然后是作为标签的子元素。按压报告它请求的值，即 `Checked` 的反面；应用设置 signal。

```go
base.Switch{
	Checked:  dark.Get,
	OnChange: dark.Set,
	Label:    "深色外观",
	Style:    core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8},
	TrackStyle: core.Style{Width: core.Pt(38), Height: core.Pt(22), CornerRadius: 11},
	ThumbStyle: core.Style{Width: core.Pt(18), Height: core.Pt(18), CornerRadius: 9},
}
```

`Disabled` 阻止按压和焦点。轨道与滑块各自接受 `Style` 和 `StyleFunc`，它们的状态样式可以跟随 `Checked`。

## 带样式的开关

`ui.Switch` 接受 `Checked`、`OnChange`、`Label` 和 `Size`（`SizeDefault`、`SizeSmall` 或 `SizeLarge`）。滑块在平台的帧时钟上滑动，轨道改变颜色。

```go
ui.Switch{Checked: sync.Get, OnChange: sync.Set, Label: "自动同步", Size: ui.SizeSmall}
```

开关用于立即生效的设置。表单中的一个选择用[复选框](checkbox.md)；多个选项中的一个用 `base.Radio` 或[选择器](select.md)。

## 无障碍

开关报告 `RoleSwitch` 以及打开或关闭状态。`Label` 是辅助技术朗读的内容，开关旁的文字通常就是它的标签。获得键盘焦点时，空格或按压切换它。

## 平台

控件由应用绘制，因此在所有平台上外观相同。按压、焦点和屏幕阅读器来自平台。手机上的开关默认与桌面一样大；`SizeSmall` 是给密集行用的，不是按设备区分的。

## 相关

- [复选框](checkbox.md)：一个或多个选择。
- [表单](../forms.md)：表单中的设置。
- [无障碍](../accessibility.md)：标签和焦点。
- gallery 的 Switch 页面，包含 `base` 和 `ui` 两种形式。
