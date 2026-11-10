---
title: Looks and themes
description: Take colors from the theme in effect, and give a component variants and sizes.
group: Writing components
order: 72
---

A styled component chooses styles and nothing else: it wraps an unstyled one, in a package of its own such as `chipsui`, and keeps its behavior there. Its colors come from the theme, so it follows the app's theme, a panel's own theme and the light or dark appearance without being built again.

## Colors from the theme

Read colors inside a `StyleFunc`: it runs again when what it read changes, so a change of theme or appearance restyles the component in place. `cx.Palette()` gives the palette shown where the component is, and `ui.ColorsOf(cx)` names the roles `ui` uses.

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

Hard-coded colors break in dark mode and under an app's theme; take every color from a role.

## Roles of your own

A color the palette has no role for goes in `Palette.Colors` under a name of yours, set in both appearances. Apps override it in code or in their theme file, under `light` and `dark`:

```go
theme := core.DefaultTheme()
theme.Light.SetColor("chip", core.RGB(0xeef5ff))
theme.Dark.SetColor("chip", core.RGB(0x1b2a3d))
cx.SetTheme(theme)
```

A component reads it with `cx.Palette().Color("chip")`, which reports whether the theme has it, so the component can fall back to a role when it does not. Gradients, shadows and other finishes are not colors: keep them in code, chosen by `cx.Dark()`.

## Variants

A component with several looks takes `Variant`, a type of its own whose zero value is the default look. It names only looks the component draws, its color included, rather than boolean fields or a shared color type:

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

## Sizes

Most components need one size, which keeps controls lined up without an app choosing. One that needs more takes `Size`, a `ui.Size` of default, small or large, so it matches `ui.Button`'s 28, 24 and 32 points; `Size.Height()` gives the height to line other controls up with. A component that is one shape at any scale, such as an avatar, takes `Size` in points instead.
