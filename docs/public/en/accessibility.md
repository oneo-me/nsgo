---
title: Accessibility
description: Labels, roles, values, actions and focus for screen readers and keyboards.
group: Core concepts
order: 61
---

## What a screen reader reads

A view's `Label` is what assistive technology says, `Hint` what activating it does, and `Role` what it is: `core.RoleButton`, `RoleLink`, `RoleHeader`, `RoleImage`, `RoleCheckbox`, `RoleSwitch`, `RoleRadio`, `RoleTab`, `RoleTextField`, `RoleMenu`, `RoleMenuItem`, `RoleDialog`, `RoleAlert`, `RoleAdjustable`, `RoleProgressBar`, `RoleTable` and the rest. A text without a label is read as its content.

```go
core.View{
	Role:  core.RoleButton,
	Label: "Add to cart",
	Hint:  "Adds the item and opens the cart",
	OnPress: add,
	Children: []core.Node{core.Icon{SVG: lucide.ShoppingCart}},
}
```

`AccessibilityValue` gives a range or a textual value: a slider's minimum, maximum and current number, or a text such as "3 items". `Accessible` makes a view one element, so a card's name and price are read together instead of as two stops, and without a label the view reads its descendants' text. `AccessibilityElementsHidden` hides decoration, such as a chevron or a separator, while `ImportantForAccessibility` hides a whole subtree.

```go
core.View{Accessible: true, Label: "Apples, $1.20 a kilo", Children: []core.Node{
	core.Text{Content: "Apples"},
	core.Text{Content: "$1.20 a kilo"},
}}
```

## Actions

`AccessibilityActions` lists what the screen reader offers on an element: the standard actions (`core.ActionActivate`, `ActionIncrement`, `ActionDecrement`, `ActionEscape`, `ActionMagicTap`) and the app's own, each with a `Name` and a `Label`. `OnAccessibilityAction` receives the name.

```go
core.View{
	Role: core.RoleAdjustable,
	Label: "Quantity",
	AccessibilityValue: func() core.AccessibilityValue {
		return core.AccessibilityValue{Max: 10, Now: float32(n.Get()), Text: fmt.Sprintf("%d items", n.Get())}
	},
	AccessibilityActions: []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}},
	OnAccessibilityAction: func(name string) {
		if name == core.ActionIncrement {
			n.Update(func(v int) int { return v + 1 })
		} else {
			n.Update(func(v int) int { return v - 1 })
		}
	},
	Children: quantityControls,
}
```

`Expanded` says whether a disclosure or a menu button is open, `Busy` that content is loading, and `Checked`, `Selected` and `Pressed` report a control's state. A control's parts can take its state through `PartStates`, so an indicator follows its checkbox without being a stop of its own.

## Live changes

`LiveRegion` has screen readers announce a view's label and the text inside it as they change: `core.LiveRegionPolite` waits for the user to be idle, `LiveRegionAssertive` interrupts. `cx.Announce` says one message once.

```go
core.Text{ContentFunc: func() string { return fmt.Sprintf("%d items in the basket", count.Get()) },
	LiveRegion: core.LiveRegionPolite}
```

## Focus and the keyboard

On macOS, the web and an Apple TV, views that take presses are Tab stops beside text fields; `NoFocus` keeps one out and `Focusable` makes a view without presses a stop. `Style.Focus` and `Style.FocusWithin` draw the focus ring, `OnFocus` and `OnBlur` hear it, and `Ref.Focus()` and `Ref.SetAccessibilityFocus()` move it. Return and Space press the focused view, and arrow keys move through tab lists, radio groups and menus.

`cx.Env().ScreenReader()` says whether one runs, `cx.Env().ReduceMotion()` whether the user asked for less motion, `cx.Env().FontScale()` the text size, and `cx.Env().RTL()` the layout direction. Texts follow the text size unless `NoFontScaling` is set, and a layout that is right to left mirrors its physical edges when it uses the logical ones.

## Next

- [Styles and themes](styles.md): focus and disabled states.
- [Forms](forms.md): labels and values for fields.
- [Canvas and shaders](canvas.md): exposing a drawing's parts.
