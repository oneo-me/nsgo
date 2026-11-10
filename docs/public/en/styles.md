---
title: Styles and themes
description: Typed styles, Tailwind classes, state styles, and light and dark themes.
group: Core concepts
order: 52
---

## A style

`core.Style` is one Go value for a box's layout, look and text. Every field's zero value is unset, so styles compose: `Merge` replaces only what the later style sets, and a child inherits the text fields its parents set.

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

Styles take CSS's colors, corners, borders, outlines, shadows, gradients, filters, backdrop blurs, masks, blend modes, 3D transforms and transitions. `core.RGB`, `core.RGBA` and `core.ParseColor` make colors, `core.LinearGradient` and its kin make gradients, `core.Ring` makes Tailwind's ring, and `core.AllCorners` gives each corner its radius. `BorderCurveContinuous` rounds corners into their edges as iOS does.

```go
core.View{Style: core.Style{
	CornerRadii: core.Corners{TopLeft: 28, BottomRight: 28},
	BorderCurve: core.BorderCurveContinuous,
	Shadows:     []core.BoxShadow{core.Ring(1, core.RGBA(0x000000, 0.08)), ui.Shadow(0, 8, 24, -6, ui.Black(0.18))},
}}
```

## States

A view's look can change with its state. `Hover`, `Focus`, `FocusWithin`, `Active`, `Selected`, `Checked`, `Disabled` and the `aria-*` states are `*Style` values that merge over the rest while the state holds. An app sets `Selected`, `Checked`, `Disabled` and `Expanded` through functions, and the platform reports presses and focus.

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

A part of a component can take the states of the control around it without reporting them itself: `PartStates` gives `Checked`, `Selected`, `Disabled` and the rest, so a checkbox's mark follows the checkbox for screen readers too.

## Classes

`Class` on `View`, `Text`, `Span`, `Input`, `Image`, `Scroll` and `Canvas` takes Tailwind CSS 4 classes, and every part of a `base` component has a class beside its style. Utilities, variants and arbitrary values mean what they mean in Tailwind, on its default theme and palette.

```go
core.View{Class: "flex items-center gap-3 rounded-xl border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"}
```

A class merges under the style, so the style's fields win. Classes can follow what changes with `ClassFunc`, and a state variant (`hover:bg-zinc-100`, `checked:opacity-100`) becomes the style's state. `group` and `group-hover:` style children from a parent's state, `first:` and `last:` follow a child's place, and `aria-checked:`, `data-open:` and `focus-within:` follow the states a control shows.

The theme's `Tailwind` field extends the default theme with colors, breakpoints and utilities, and a color the palette lacks names a `Palette` role (`bg-accent`, `text-text-muted/70`) or a theme color, read from the node's theme in the appearance it shows.

## Themes

A `core.Theme` is one look in both appearances: a `Palette` for light and one for dark, a corner radius and font families. The palette's roles are `Background`, `Surface`, `Text`, `TextMuted`, `Border`, `Accent`, `AccentText`, `Danger`, `Warning`, `Success`, `Selection`, `Focus` and `Scrollbar`; `Colors` holds the ones a library adds, such as `inset` and `raised`. `core.DefaultTheme()`, which an app shows until it sets another, is the neutral theme with a blue accent and `ui`'s roles, and `ui.WarmDuskTheme()` a warm palette.

```go
theme := core.DefaultTheme()
theme.Light.Accent, theme.Dark.Accent = core.RGB(0xbd481b), core.RGB(0xed986e)
theme.Radius = 10
cx.SetTheme(theme)
```

`cx.SetTheme` gives a theme to the scope and everything built under it, so a sidebar or a panel can take a theme of its own; a nil theme takes the one above. `core.ParseTheme` reads a YAML or JSON theme file, or a shadcn/ui theme file's CSS variables, with role colors, a radius and font families. `core.DefaultTheme`, the theme an app shows until it sets one, and `ui.WarmDuskTheme` return themes ready to modify.

`cx.SetAppearance` forces the whole app light or dark, `cx.SetSubtreeAppearance` does it for a subtree, and `core.AppearanceSystem` follows the system again. `cx.Dark()` says which appearance a subtree shows, and `cx.Palette()` gives its colors with the embedding host's overrides applied.

## Colors in components

A component reads its colors where it is built and in its `StyleFunc`s, so a theme or appearance change follows in place, without building the component again. `ui.ColorsOf(cx)` names the roles `ui` uses, `ui.TintsOf(cx)` gives their translucent fills, and `ui.FinOf(cx)` gives `ui`'s layered finishes.

```go
colors := ui.ColorsOf(cx)
core.Text{Content: "Hello", StyleFunc: func() core.Style {
	return core.Style{Color: colors.Text(), FontSize: 15}
}}
```

`StyleFunc` runs again when what it read changes, so `cx.Dark()` and `cx.Palette()` inside it follow the theme. The same applies to `ClassFunc`, a span's `StyleFunc` and a component's `StyleFunc` fields.

## Next

- [Layout](layout.md): where boxes go.
- [Text](text.md): fonts, spans and decorations.
- [Animation](animation.md): transitions between styles.
