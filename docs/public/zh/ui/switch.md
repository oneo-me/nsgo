---
title: 开关
description: 打开或关闭的设置，有三种尺寸。
group: 组件
order: 35
---

`ui.Switch` 是立即生效的设置，采用 Onevra 外观。它不保存值：应用保存它，并在 `OnChange` 中更新。

```go
notifications := core.NewSignal(cx, true)
ui.Switch{Checked: notifications.Get, OnChange: notifications.Set, Label: "通知"}
```

## 尺寸

`ui.Switch` 接受 `Checked`、`OnChange`、`Label` 和 `Size`（`SizeDefault`、`SizeSmall` 或 `SizeLarge`）。旋钮的滑动和轨道颜色的变化在平台的帧时钟上运行。

```go
ui.Switch{Checked: sync.Get, OnChange: sync.Set, Label: "自动同步", Size: ui.SizeSmall}
```

开关用于立即生效的设置。表单中的一个选择用[复选框](checkbox.md)；多个选项中的一个用放在 `ui.RadioGroup` 中的 `ui.Radio`，或者[选择器](select.md)。

## 无障碍

开关报告 `RoleSwitch` 及其开或关的状态。`Label` 是辅助技术读出的内容，开关旁边的文字通常就是它的标签。获得键盘焦点时，空格或按压会切换它。

## 平台

控件由应用绘制，所以各处看起来一样。按压、焦点和读屏器来自平台。手机上的开关按设计与桌面上的尺寸相同；`SizeSmall` 用于紧凑的行，而不是某种设备。

## 相关

- [复选框](checkbox.md)：一个或多个选择。
- [表单](../forms.md)：表单中的设置。
- [无障碍](../accessibility.md)：标签和焦点。
- gallery 的 Switch 页面。
- [无样式组件](../authoring/base.md#switch)：`base.Switch`，用于自己设计外观的开关。
