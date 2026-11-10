---
title: Components and state
description: Elements, components, signals and styles.
group: Core concepts
order: 50
---

## Elements

The screen is a tree of elements, which are Go struct values. `core` has the primitives: `View`, `Text`, `Image`, `Scroll`, `Input` and more. `ui` has the controls an app uses, such as `Button`, `Select` and `DatePicker`, in a look that follows the theme. (`base` holds the same controls without a look, for [component authors](authoring/index.md).)

```go
core.View{
	Style: core.Style{Padding: core.All(24), Gap: 12},
	Children: []core.Node{
		core.Text{Content: "Hello"},
	},
}
```

## Components

A component is a function that takes a scope and returns an element. It runs once; what changes later is driven by signals.

```go
func Greeting(cx *core.Scope) core.Node {
	return core.Text{Content: "Hello from NSGo"}
}
```

A component with props is a type with a `Render` method:

```go
type Badge struct{ Text string }

func (b Badge) Render(*core.Scope) core.Node {
	return core.Text{Content: b.Text}
}
```

## Signals

A signal holds a value. Reading it inside a function an element calls, such as `ContentFunc`, subscribes that part of the screen, and only that part updates when the value changes.

```go
count := core.NewSignal(cx, 0)

core.Text{ContentFunc: func() string { return fmt.Sprintf("Pressed %d times", count.Get()) }}

ui.Button{Text: "Press me", OnPress: func() { count.Update(func(n int) int { return n + 1 }) }}
```

## Styles

`core.Style` follows CSS: flexbox and grid, paddings and margins, corners, borders, shadows, gradients, transforms and states such as `Active`. Elements also take Tailwind classes through `Class`:

```go
core.Text{Content: "Done", Class: "self-start rounded-full bg-zinc-100 px-3 py-1 text-sm"}
```

Themes give `ui`'s components their look in light and dark.

## Desktop apps

`nsgo.Run(Root)` shows one root view. A desktop app can open what it shows itself instead: `nsgo.Run(App)` runs `func App(app *core.Scope)` once, and it opens windows, panels, tray icons and global shortcuts with `desktop`, which share its signals. This works on macOS for now; `examples/tray` keeps out of the Dock and lives in the menu bar.
