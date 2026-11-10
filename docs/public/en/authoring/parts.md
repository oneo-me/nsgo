---
title: Parts and states
description: Hold no state the app owns, style every part, and tell assistive technology what the component is.
group: Writing components
order: 71
---

An unstyled component decides how a control behaves and leaves how it looks to whoever uses it. This page builds `Toggle`, a pill that is on or off, the way `base` builds its controls.

## Controlled values

A control holds no value the app owns: the app passes it as a function and sets it from a callback. The component reads it where it is shown, so only that part updates when it changes.

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

An app keeps the value in a signal: `chips.Toggle{On: bold.Get, OnChange: bold.Set, Label: "Bold"}`.

## Parts

Each part a component draws takes a `Class`, a `Style` and a `StyleFunc`, named after the part (`DotClass`, `DotStyle`, `DotStyleFunc`). `Style` is fixed, `StyleFunc` follows the signals it reads, and the class goes under both, so an app can use whichever it writes.

A part takes the control's state through `PartStates` rather than reporting its own, so `checked:` classes and `Checked` styles apply to it while assistive technology sees one control:

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

An app styles it with classes alone:

```go
chips.Toggle{
	On: bold.Get, OnChange: bold.Set, Label: "Bold",
	Class:    "flex-row items-center gap-2 rounded-full border border-zinc-300 px-3 py-1 checked:bg-blue-50",
	DotClass: "size-2 rounded-full bg-zinc-300 checked:bg-blue-500",
	Children: []core.Node{core.Text{Content: "Bold"}},
}
```

## Accessibility

The root of a control says what it is: a `Role`, a `Label` when its text does not say it, a `Hint` for what activating it does, and `Checked`, `Selected`, `Disabled` or `Expanded` as they apply. Its parts stay silent. A control that moves between items with the arrows is one Tab stop, as a tab list is. [Accessibility](../accessibility.md) lists the roles and actions.

## What to pass through

A component whose root is a view of its own takes that view's `OnLayout`, `Tooltip` and `TestID` and hands them on, so apps measure, explain and test it as any view. Callbacks are optional: a nil `OnChange` makes a control that shows a value without changing it.
