---
title: 部件与状态
description: 不持有应用拥有的状态，开放每个部件的样式，并告诉辅助技术组件是什么。
group: 编写组件
order: 71
---

无样式组件决定控件的行为，把外观留给使用者。本页按 `base` 构建控件的方式写一个 `Toggle`：一个可开可关的胶囊。

## 受控值

控件不持有应用拥有的值：应用以函数传入，并在回调中设置它。组件在显示它的地方读取，所以值变化时只有那一部分更新。

```go
// Toggle is a pill that is on or off. Its parts are the pill and a dot
// inside it, then its children, the label.
type Toggle struct {
	On       func() bool
	Disabled func() bool
	// OnChange receives the value a press asks for, the opposite of On's.
	OnChange func(on bool)

	Class        string
	Style        core.Style
	StyleFunc    func() core.Style
	DotClass     string
	DotStyle     core.Style
	DotStyleFunc func() core.Style
	Children     []core.Node
	Label        string
}
```

应用把值放在 signal 里：`chips.Toggle{On: bold.Get, OnChange: bold.Set, Label: "Bold"}`。

## 部件

组件绘制的每个部件都有以部件命名的 `Class`、`Style` 和 `StyleFunc`（`DotClass`、`DotStyle`、`DotStyleFunc`）。`Style` 是固定的，`StyleFunc` 跟随它读取的 signal，class 位于两者之下，应用用哪种写法都可以。

部件通过 `PartStates` 取得控件的状态，而不是自己上报，这样 `checked:` class 和 `Checked` 样式会作用于它，而辅助技术看到的仍是一个控件：

```go
func (t Toggle) Render(*core.Scope) core.Node {
	on := func() bool { return t.On != nil && t.On() }
	off := func() bool { return t.Disabled != nil && t.Disabled() }
	shows := func() core.PartStates { return core.PartStates{Checked: on(), Disabled: off()} }
	dot := core.View{Class: t.DotClass, Style: t.DotStyle, StyleFunc: t.DotStyleFunc, PartStates: shows}
	return core.View{
		Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc,
		Role: core.RoleSwitch, Label: t.Label, Checked: on, Disabled: t.Disabled,
		OnPress: func(core.PressEvent) {
			if t.OnChange != nil {
				t.OnChange(!on())
			}
		},
		Children: append([]core.Node{dot}, t.Children...),
	}
}
```

应用只用 class 就能给它样式：

```go
chips.Toggle{
	On: bold.Get, OnChange: bold.Set, Label: "Bold",
	Class:    "flex-row items-center gap-2 rounded-full border border-zinc-300 px-3 py-1 checked:bg-blue-50",
	DotClass: "size-2 rounded-full bg-zinc-300 checked:bg-blue-500",
	Children: []core.Node{core.Text{Content: "Bold"}},
}
```

## 无障碍

控件的根视图说明它是什么：`Role`；文字没有说清时给出 `Label`；激活后会发生什么写在 `Hint`；适用时再加上 `Checked`、`Selected`、`Disabled` 或 `Expanded`。部件本身不发声。用方向键在条目间移动的控件只占一个 Tab 停靠点，像标签列表那样。角色和操作的列表见[无障碍](../accessibility.md)。

## 要透传的字段

根视图是自己的视图的组件，要接收并传下该视图的 `OnLayout`、`Tooltip` 和 `TestID`，让应用能像对待任何视图一样测量、提示和测试它。回调是可选的：`OnChange` 为 nil 时，控件只显示值而不改变它。
