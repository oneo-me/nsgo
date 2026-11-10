---
title: 编写组件
description: 编写给别人放进应用里的组件，方式与 NSGo 自己的 base 和 ui 相同。
group: 编写组件
order: 70
---

这些页面写给组件作者：一套控件库、一个设计系统，或者一个由应用以 Go 包引入的复杂视图。阅读前请先了解[组件与状态](../components.md)和[样式与主题](../styles.md)。

## 一个组件

组件是带 `Render` 方法的类型，和应用里的写法一样。它的字段由应用设置；应用没有设置的字段取零值，所以每个字段的零值都应当是最常见的情况。

```go
package chips

import "github.com/oneo-me/nsgo/core"

// Chip is a short label in a pill.
type Chip struct {
	Text string
}

// Render implements core.Node.
func (c Chip) Render(*core.Scope) core.Node {
	return core.View{Class: "self-start rounded-full px-3 py-1", Children: []core.Node{
		core.Text{Content: c.Text, Class: "text-sm"},
	}}
}
```

应用像使用其他元素一样使用它：`chips.Chip{Text: "New"}`。

## 行为与外观

NSGo 的每个控件都有两份。`base` 负责行为、状态和无障碍，没有外观，每个部件的样式都来自应用；`ui` 给 `base` 的形式加上外观，行为只在一处实现。组件库也可以这样划分：

- **无样式**组件负责按压、键盘、焦点和读屏器读出的内容，并开放每个部件的样式，像 `base.Switch` 那样。应用和其他组件库给它外观（[部件与状态](parts.md)）。
- **带样式**组件包装一个无样式组件，只负责选择样式，样式取自当前主题，像 `ui.Switch` 那样（[外观与主题](looks.md)）。

没有值得共享的行为的组件，比如徽标，可以只有带样式的形式。

## 一个包

- 一个文件放一个组件，以组件命名（`chip.go`），只放它自己用到的类型；多个组件共用的部分单独成文件。
- 复杂组件单独成包，API 中省去组件名，像 `base/dock` 的 `dock.Area`。
- 组件要说的话放在包的 `locales/` 中（[文案、测试与发布](shipping.md)）。
- 不要按设备隐式改变任何东西：在手机或桌面上不同的尺寸或形态，应该是应用设置的字段。

## 下一步

- [部件与状态](parts.md)：受控值、可设置样式的部件和无障碍。
- [外观与主题](looks.md)：来自主题的颜色、变体和尺寸。
- [无样式组件](base.md)：NSGo 自己的无外观控件，可以设置样式或在其上构建。
- [文案、测试与发布](shipping.md)：翻译、测试和 TSX。
