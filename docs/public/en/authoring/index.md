---
title: Writing components
description: Make components other people put in their apps, as NSGo's own base and ui are made.
group: Writing components
order: 70
---

These pages are for authors of components: a library of controls, a design system or one complex view that apps import as a Go package. They assume [Components and state](../components.md) and [Styles and themes](../styles.md).

## A component

A component is a type with a `Render` method, as in an app. Its fields are what the app sets; whatever the app does not set takes the zero value, so the zero value of each field should be the usual case.

```go
package chips

import "github.com/nsgo-dev/nsgo/core"

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

An app uses it as any element: `chips.Chip{Text: "New"}`.

## Behavior and look

NSGo keeps each control twice. `base` holds its behavior, state and accessibility with no look, and takes every style from the app part by part; `ui` puts a look on the `base` form, so behavior lives in one place. A library can follow the same split:

- An **unstyled** component owns presses, the keyboard, focus and what screen readers hear, and exposes each part's style, as `base.Switch` does. Apps and other libraries give it their look ([Parts and states](parts.md)).
- A **styled** component wraps an unstyled one and only chooses styles, from the theme in effect, as `ui.Switch` does ([Looks and themes](looks.md)).

A component with no behavior worth sharing, such as a badge, can be styled alone.

## A package

- Keep one component to a file, named after it (`chip.go`), with the types only it uses; what several share gets a file of its own.
- Give a complex component a package of its own, whose API drops the component's name, as `base/dock`'s `dock.Area`.
- Put the words a component says in the package's `locales/` ([Words, tests and publishing](shipping.md)).
- Do nothing implicitly by device: a size or form that changes on a phone or a desktop is a field the app sets.

## Next

- [Parts and states](parts.md): controlled values, styled parts and accessibility.
- [Looks and themes](looks.md): colors from the theme, variants and sizes.
- [Unstyled components](base.md): NSGo's own controls without a look, to style or build on.
- [Words, tests and publishing](shipping.md): translations, tests and TSX.
