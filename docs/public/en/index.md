---
title: Introduction
description: What NSGo is, what it runs on and how it works.
group: Start
order: 1
---

NSGo is a cross-platform native UI framework written in Go. You write an app once, in Go or in TSX, and it runs on iOS, tvOS, watchOS, Android, macOS, Windows, Linux and the web, drawn by each platform's own views; Go apps also run in WeChat, QQ, Alipay and Douyin mini programs.

## What you write

An app is a Go main package. Its components return elements: views, text, images, inputs and the controls in `base` and `ui`. State lives in signals, and only what reads a signal updates when it changes.

```go
func Root(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	return core.View{Style: core.Style{Padding: core.All(24), Gap: 12}, Children: []core.Node{
		core.Text{ContentFunc: func() string { return fmt.Sprintf("Pressed %d times", count.Get()) }},
		base.Button{
			OnPress:  func(core.PressEvent) { count.Update(func(n int) int { return n + 1 }) },
			Children: []core.Node{core.Text{Content: "Press me"}},
		},
	}}
}
```

The same app can be written in TSX and styled with Tailwind classes ([TSX apps](tsx.md)).

## How it works

A Go engine owns the app: the reactive graph, styles, flexbox and grid layout, and text preparation. Each update produces one batch of changes, and a thin backend applies it to the platform's native views. Animation runs on the platform's frame clock, so nothing commits once a frame.

| Platform | Draws with |
| --- | --- |
| iOS | UIKit and CoreText |
| Android | Android Views |
| macOS | AppKit and CoreText |
| Windows | Direct2D and DirectWrite |
| Linux | GTK 4, Pango and Cairo |
| Web | WebAssembly and DOM elements |

Every target builds on a Mac with Go; iOS also needs Xcode. The `nsgo` tool creates, runs, tests and packages apps.

## Status

NSGo is early: it has no stable API or release yet, and what is missing is in the [roadmap](https://github.com/oneo-me/nsgo/blob/main/docs/roadmap.md). It ports the design of [Native Shell](https://github.com/longbridge/nsh) to Go.

## How to read the docs

Every page answers one question. A code example is a fragment of a component unless the page says otherwise: `cx` is the `*core.Scope` a component receives. `base` components have no look and take their styles from the app; `ui` components are the same controls in Onevra's look.

- [Your first app](getting-started.md): build a small todo app step by step.
- [Building UI](components.md): elements, components, signals, [layout](layout.md), [styles](styles.md), [text](text.md), [lists](lists.md), [navigation](navigation.md) and [forms](forms.md).
- [Component reference](ui.md): the most-used components, each with its `base` and `ui` forms.
- [Guides](tsx.md): [TSX apps](tsx.md), [testing](testing.md), [localization](localization.md), [building](building.md) and [embedding](embedding.md).
- [Contributing](contributing.md): build NSGo and the website from source.
- The [live demo](https://oneo-me.github.io/nsgo/gallery/loader.html): every component, in your browser.
