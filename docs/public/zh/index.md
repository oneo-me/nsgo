---
title: 简介
description: NSGo 是什么、运行在哪些平台上，以及它如何工作。
order: 1
---

NSGo 是一个用 Go 编写的跨平台原生 UI 框架。用 Go 或 TSX 写一次应用，它就能运行在 iOS、tvOS、watchOS、Android、macOS、Windows、Linux 和 Web 上，由每个平台自己的视图绘制；Go 应用还能运行在微信小程序中。

## 你要写什么

应用是一个 Go main 包。组件返回元素：视图、文本、图片、输入框，以及 `ui` 中的控件。状态存放在 signal 里，signal 变化时只有读取它的地方会更新。

```go
func Root(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	return core.View{Style: core.Style{Padding: core.All(24), Gap: 12}, Children: []core.Node{
		core.Text{ContentFunc: func() string { return fmt.Sprintf("Pressed %d times", count.Get()) }},
		ui.Button{Text: "Press me", OnPress: func() { count.Update(func(n int) int { return n + 1 }) }},
	}}
}
```

同一个应用也可以用 TSX 编写，并用 Tailwind 类名设置样式（[TSX 应用](tsx.md)）。

## 工作原理

Go 引擎掌管应用：响应式图、样式、flexbox 与 grid 布局，以及文本准备。每次更新产生一批变化，由一层很薄的后端应用到平台的原生视图上。动画运行在平台的帧时钟上，不需要每帧提交。

| 平台 | 绘制方式 |
| --- | --- |
| iOS | UIKit 与 CoreText |
| Android | Android View |
| macOS | AppKit 与 CoreText |
| Windows | Direct2D 与 DirectWrite |
| Linux | GTK 4、Pango 与 Cairo |
| Web | WebAssembly 与 DOM 元素 |

每个目标都能在装有 Go 的 Mac 上构建，iOS 另需 Xcode。`nsgo` 工具负责创建、运行、测试和打包应用。

## 现状

NSGo 还处于早期：尚无稳定 API，也还没有发布版本，缺少的部分记录在[路线图](https://github.com/oneo-me/nsgo/blob/main/docs/roadmap.md)中。它把 [Native Shell](https://github.com/longbridge/nsh) 的设计移植到了 Go。

## 如何阅读这些文档

每个页面只回答一个问题。除非页面另有说明，代码示例都是组件中的片段：`cx` 是组件收到的 `*core.Scope`。应用使用带外观的 `ui` 组件；`base` 是同一批控件的无外观形式，供组件作者使用。

- **入门**：先完成[环境准备](getting-started.md)，再一步一步做示例应用：[待办应用](tutorials/todo.md)、[笔记应用](tutorials/notes.md)和[GitHub 热门仓库](tutorials/repos.md)。
- **工具链**：[运行与调试](debugging.md)、[测试](testing.md)、[构建与发布](building.md)和[嵌入](embedding.md)。
- **组件**：[最常用的组件](ui.md)，以及它们的变体、状态和无障碍。
- **核心概念**：[组件与状态](components.md)、[布局](layout.md)、[样式](styles.md)、[文本](text.md)、[列表](lists.md)、[导航](navigation.md)、[表单](forms.md)、[本地化](localization.md)、[TSX](tsx.md) 等。
- **编写组件**：[给别人的应用用的组件](authoring/index.md)，无样式和带样式两种形式。
- **参与项目**：[从源码构建 NSGo](contributing.md)、了解代码结构并提交改动。
- [在线体验](https://oneo-me.github.io/nsgo/gallery/loader.html)：在浏览器中查看全部组件。
