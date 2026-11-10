---
title: 样式与主题
description: 类型化样式、Tailwind 类名、状态样式，以及浅色和深色主题。
group: 核心概念
order: 52
---

## 样式

`core.Style` 用一个 Go 值描述盒子的布局、外观和文本。每个字段的零值都表示未设置，因此样式可以叠加：`Merge` 只替换后一个样式设置的字段，子元素继承父元素设置的文本字段。

```go
card := core.Style{
	Background: core.RGB(0xffffff),
	CornerRadius: 12,
	Padding: core.All(16),
	Gap: 12,
	BorderWidth: 1,
	BorderColor: core.RGBA(0x000000, 0.1),
}
```

样式支持 CSS 的颜色、圆角、边框、描边、阴影、渐变、滤镜、背景模糊、遮罩、混合模式、3D 变换和过渡。`core.RGB`、`core.RGBA` 和 `core.ParseColor` 生成颜色，`core.LinearGradient` 及其同类生成渐变，`core.Ring` 生成 Tailwind 的 ring，`core.AllCorners` 设置每个角。`BorderCurveContinuous` 让圆角像 iOS 一样平滑接入边。

```go
core.View{Style: core.Style{
	CornerRadii: core.Corners{TopLeft: 28, BottomRight: 28},
	BorderCurve: core.BorderCurveContinuous,
	Shadows:     []core.BoxShadow{core.Ring(1, core.RGBA(0x000000, 0.08)), ui.Shadow(0, 8, 24, -6, ui.Black(0.18))},
}}
```

## 状态

视图的外观可以随状态改变。`Hover`、`Focus`、`FocusWithin`、`Active`、`Selected`、`Checked`、`Disabled` 和 `aria-*` 状态都是 `*Style`，在状态成立时覆盖其余样式。应用通过函数设置 `Selected`、`Checked`、`Disabled` 和 `Expanded`，平台报告按压和焦点。

```go
core.View{
	OnPress: func(core.PressEvent) { open() },
	Style: core.Style{
		Background: core.RGB(0x007aff),
		Active:     &core.Style{Opacity: core.Alpha(0.85)},
		Hover:      &core.Style{Background: core.RGB(0x006be0)},
	},
	Disabled: func() bool { return saving },
}
```

组件的某个部件可以借用外层控件的状态而不自己报告：`PartStates` 提供 `Checked`、`Selected`、`Disabled` 等，因此复选框的勾选标记会跟随复选框，屏幕阅读器也只会读到控件本身。

## 类名

`View`、`Text`、`Span`、`Input`、`Image`、`Scroll` 和 `Canvas` 上的 `Class` 接受 Tailwind CSS 4 类名，`base` 组件的每个部件也都有与样式并列的类名。工具类、变体和任意值都与 Tailwind 一致，使用它的默认主题和调色板。

```go
core.View{Class: "flex items-center gap-3 rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"}
```

类名合并在样式之下，因此样式字段优先。`ClassFunc` 让类名跟随变化，状态变体（`hover:bg-zinc-100`、`checked:opacity-100`）会成为样式的状态。`group` 和 `group-hover:` 让子元素跟随父元素的状态，`first:` 和 `last:` 跟随子元素的位置，`aria-checked:`、`data-open:` 和 `focus-within:` 跟随控件展示的状态。

主题的 `Tailwind` 字段用颜色、断点和工具类扩展默认主题；调色板没有的颜色可以写成 `Palette` 的角色名（`bg-accent`、`text-text-muted/70`），或主题定义的颜色，从节点所在主题的当前外观读取。

## 主题

`core.Theme` 是同一种外观在浅色和深色下的定义：两套 `Palette`、一个圆角半径和字体系列。调色板的角色有 `Background`、`Surface`、`Text`、`TextMuted`、`Border`、`Accent`、`AccentText`、`Danger`、`Warning`、`Success`、`Selection`、`Focus` 和 `Scrollbar`；`Colors` 保存组件库添加的角色，例如 `inset` 和 `raised`。`core.DefaultTheme()` 是带蓝色强调色的中性主题，包含 `ui` 的角色，应用设置其他主题前显示的就是它；`ui.WarmDuskTheme()` 是暖色调色板。

```go
theme := core.DefaultTheme()
theme.Light.Accent, theme.Dark.Accent = core.RGB(0xbd481b), core.RGB(0xed986e)
theme.Radius = 10
cx.SetTheme(theme)
```

`cx.SetTheme` 把主题给当前 scope 及其之后构建的所有内容，因此侧边栏或面板可以有各自的主题；传入 nil 会恢复上层的主题。`core.ParseTheme` 读取 YAML 或 JSON 主题文件，或 shadcn/ui 主题文件的 CSS 变量，其中包含角色颜色、圆角和字体系列。

`cx.SetAppearance` 强制整个应用使用浅色或深色，`cx.SetSubtreeAppearance` 只作用于子树，`core.AppearanceSystem` 重新跟随系统。`cx.Dark()` 返回子树当前的外观，`cx.Palette()` 返回应用了嵌入宿主覆盖后的颜色。

## 组件中的颜色

组件在构建时和 `StyleFunc` 中读取颜色，因此主题或外观变化会原地生效，不需要重新构建组件。`ui.ColorsOf(cx)` 给出 `ui` 使用的角色，`ui.TintsOf(cx)` 给出它们的半透明填充，`ui.FinOf(cx)` 给出 `ui` 的层次质感。

```go
colors := ui.ColorsOf(cx)
core.Text{Content: "你好", StyleFunc: func() core.Style {
	return core.Style{Color: colors.Text(), FontSize: 15}
}}
```

`StyleFunc` 读取的值变化时会再次运行，因此其中的 `cx.Dark()` 和 `cx.Palette()` 会跟随主题。`ClassFunc`、span 的 `StyleFunc` 和组件的 `StyleFunc` 字段同理。

## 下一步

- [布局](layout.md)：盒子放在哪里。
- [文本](text.md)：字体、span 和装饰。
- [动画](animation.md)：样式之间的过渡。
