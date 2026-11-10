---
title: 外观与主题
description: 从当前主题取颜色，并给组件提供变体和尺寸。
group: 编写组件
order: 72
---

带样式组件只负责选择样式：它包装一个无样式组件，放在自己的包里（比如 `chipsui`），行为留在被包装的组件中。它的颜色来自主题，所以无需重新构建就能跟随应用的主题、某个面板自己的主题，以及浅色或深色外观。

## 来自主题的颜色

在 `StyleFunc` 中读取颜色：它读取的内容变化时会重新运行，所以主题或外观变化时组件会原地更新样式。`cx.Palette()` 给出组件所在位置显示的调色板，`ui.ColorsOf(cx)` 给出 `ui` 使用的各个角色。

```go
// Toggle is chips.Toggle in the theme's colors.
type Toggle struct {
	On       func() bool
	OnChange func(bool)
	Label    string
}

func (t Toggle) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	on := func() bool { return t.On != nil && t.On() }
	return chips.Toggle{On: t.On, OnChange: t.OnChange, Label: t.Label,
		Class: "flex-row items-center gap-2 rounded-full px-3 py-1",
		StyleFunc: func() core.Style {
			if on() {
				return core.Style{Background: colors.Accent()}
			}
			return core.Style{Background: colors.Inset()}
		},
		DotClass: "size-2 rounded-full",
		DotStyleFunc: func() core.Style {
			if on() {
				return core.Style{Background: colors.OnAccent()}
			}
			return core.Style{Background: colors.Faint()}
		},
		Children: []core.Node{ui.Text(t.Label, 13, ui.Medium, colors.Text)},
	}
}
```

写死的颜色在深色模式和应用自己的主题下都会出错；每种颜色都应取自一个角色。

## 自己的角色

调色板中没有对应角色的颜色，以你自己起的名字放进 `Palette.Colors`，两种外观都要设置。应用可以在代码或主题文件的 `light` 和 `dark` 下覆盖它：

```go
theme := ui.DefaultTheme()
theme.Light.SetColor("chip", core.RGB(0xeef5ff))
theme.Dark.SetColor("chip", core.RGB(0x1b2a3d))
cx.SetTheme(theme)
```

组件用 `cx.Palette().Color("chip")` 读取，它同时告诉你主题里是否有这个角色，没有时组件可以退回到一个已有角色。渐变、阴影等效果不是颜色：把它们留在代码里，按 `cx.Dark()` 选择。

## 变体

有多种外观的组件接收 `Variant`，这是它自己的类型，零值是默认外观。它只列出组件会绘制的外观（包括颜色），而不是布尔字段或共用的颜色类型：

```go
// ToggleVariant is a toggle's look.
type ToggleVariant uint8

const (
	// ToggleSolid fills the pill with the accent when on.
	ToggleSolid ToggleVariant = iota
	// ToggleOutline draws its edge in the accent instead.
	ToggleOutline
)
```

## 尺寸

大多数组件只需要一种尺寸，这样控件无需应用选择就能对齐。需要多种尺寸的组件接收 `Size`，类型为 `ui.Size`（默认、小、大），与 `ui.Button` 的 28、24、32 点一致；`Size.Height()` 给出用于对齐其他控件的高度。在任何比例下都是同一形状的组件（比如头像）则以点数接收 `Size`。
