---
title: 滑块
description: 沿轨道选择一个值或一个范围，支持键盘和指针。
group: 组件
order: 37
---

`ui.Slider` 是 Onevra 外观的滑块，沿轨道选值，并负责拖动、步进和键盘。单个值是单个旋钮；`High` 让它成为一个范围。

```go
volume := core.NewSignal(cx, 40.0)
ui.Slider{Value: volume.Get, OnChange: volume.Set, Min: 0, Max: 100, Step: 5, Label: "音量"}
```

## 范围

设置 `High` 和 `OnHighChange` 可以选择两个值。`Value` 是低端。`LowLabel` 和 `HighLabel` 为辅助技术命名两个旋钮。

```go
price := core.NewSignal(cx, 20.0)
ceiling := core.NewSignal(cx, 80.0)
ui.Slider{
	Value: price.Get, OnChange: price.Set,
	High: ceiling.Get, OnHighChange: ceiling.Set,
	Min: 0, Max: 100, Step: 1,
	Label: "价格", LowLabel: "最低", HighLabel: "最高",
}
```

## 步进与值

`Step` 让值吸附；步进为 0 时可以取任意值。`ValueText` 改变辅助技术读出的值，`Disabled` 停用它。

```go
ui.Slider{Value: speed.Get, OnChange: speed.Set, Min: 0.5, Max: 2, Step: 0.25, Label: "速度",
	ValueText: func(v float64) string { return fmt.Sprintf("%g×", v) }}
```

## 行为

拖动跟随指针，并在移动时报告 `OnChange`。方向键按一步移动获得焦点的旋钮，Home 和 End 到两端，Page Up 和 Page Down 移动更远。`Step` 大于 0 时，`ui.Slider` 在轨道下标记最多 60 个刻度。

## 无障碍

滑块报告 `RoleAdjustable`、`Label` 和值；范围有两个旋钮，各自带标签。`ValueText` 替代数字被读出。键盘和屏幕阅读器的增减操作可以改变它。

## 平台

轨道由应用绘制，在所有平台上尺寸相同。按压、焦点和屏幕阅读器来自平台；触摸拖动与指针拖动使用相同的步进。

## 相关

- [表单](../forms.md)：表单中的滑块。
- [无障碍](../accessibility.md)：值和可调节角色。
- gallery 的 Slider、RangeSlider 和 Rating 页面。
- [无样式组件](../authoring/base.md#slider)：`base.Slider`，可以垂直放置并带自定义刻度。
