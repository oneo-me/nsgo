# Themes

How an app, a component library and an embedding host give the UI its colors in both appearances, and how a subtree takes another theme or appearance. Read this before changing `core/theme.go` or the gallery's look.

## Model

A `core.Theme` is one look in both appearances: a `Palette` for light and one for dark, a corner radius and font families. Its roles are Onevra's (its `theme` package), and `DefaultTheme` holds Onevra's values:

| Role | What it colors |
| --- | --- |
| `Background` | The page |
| `Surface`, `SurfaceHover`, `SurfacePressed` | A card or control, under the pointer, held down |
| `Border` | Edges and separators |
| `Text`, `TextMuted` | Text and secondary text |
| `Accent`, `AccentHover`, `AccentPressed`, `AccentText` | The accent fill, its states, what is drawn on it |
| `Danger`, `Warning`, `Success` | Semantic colors |
| `Selection`, `Focus`, `Scrollbar` | Selected text, focus rings, scroll indicators |

`Palette.Colors` holds roles a library or app adds by name (`inset`, `info`), which `Palette.Color` and `SetColor` reach as they reach the fields, by their names in lower camel case. Gradients, shadows and other finishes are not colors and stay in the library's code, chosen by `Dark`.

## Scope

- `cx.SetTheme(t)` gives `cx` and what it builds from then on `t`; at the root it is the app's. A deeper call wins for its subtree, so a sidebar can carry another theme. Calling it again changes it.
- `cx.SetSubtreeAppearance(a)` forces light or dark for `cx` and what it builds from then on; `AppearanceSystem` follows the appearance above. `cx.SetAppearance` keeps forcing the app's, with its window chrome.
- `cx.Theme()`, `cx.Palette()` and `cx.Dark()` read the nearest theme, its palette in the appearance its subtree shows, and that appearance. They are tracked, so a `StyleFunc` that reads them follows every change without a rebuild.
- Both setters keep a signal in the scope's values, as `core.Key` does; a node made under a forced subtree binds the nearest one, so lookups cost nothing until an app forces one.
- `base` components read no theme. `Editor`, `DiffView` and highlighting follow `cx.Dark()`, so they match their subtree.

Native views follow a subtree's forced appearance through `ViewProps.Appearance`, which each node carries: `NSView.appearance` on macOS, `overrideUserInterfaceStyle` on iOS, `color-scheme` on the web. Windows, Linux and Android scope appearance per window only, so their native parts keep the app's.

## Theme files

`core.ParseTheme` reads a theme file over `DefaultTheme`, which an app embeds with `go:embed` and gives to `SetTheme`:

```yaml
radius: 8
light:
  background: "#fcfbf9"
  inset: "#f2efea" # a role of Palette.Colors
dark:
  background: "#211f1c"
```

Values are CSS colors. JSON is YAML, so a theme exported as JSON reads too.

It reads a shadcn/ui theme file too, as NSH's apps write theirs: the CSS variables of `cssVars`' `theme`, `light` and `dark` sections, where a value may name another (`var(--brand)`) and dark's go over light's. A variable that is a color is a role of the palettes, which classes name (`bg-muted-foreground`); the others are `Theme.Tokens` (`radius-card: 16px`), `radius`, `font-sans` and `font-mono` setting `Radius`, `Sans` and `Mono` too. `DefaultTheme` holds shadcn/ui's neutral tokens under their names, but those that name Onevra's roles (`background`, `border`, `accent`), so NSH's apps without a theme of their own still resolve them. A TSX app names its theme file in `nsgo.yaml`'s `theme`, and TSX's `token(name)` reads a color or a token of it.

## Tailwind

`Theme.Tailwind` extends Tailwind's default theme for the classes of the theme's subtree, as NSH's `nsh.json` `tailwind` does: `core.ParseTailwind` reads its colors (with shades and `DEFAULT`), spacing, rem, breakpoints, containers, font families, sizes and weights, tracking, leading, radii, shadows, inset and drop shadows, blurs, perspectives, aspect ratios and easings, an entry of a known name replacing the default's and an unknown key an error. Each class string is parsed once per Tailwind theme. A TSX app writes it under `nsgo.yaml`'s `tailwind`, which the compiler checks, so a mistake fails the build. Classes also read the theme's lengths as variables: `rounded-(--radius)`, `text-(length:--font-size)`, and `font-(family-name:--font-mono)` its families.

## Hosts

An embedding host's `core.Tokens` go over every theme's palette by the same role names: `Values` in both appearances, `Dark` over them where a subtree shows dark. A value that is no color is dropped.

## ui

`ui` styles `base` and `core`, as NSH's gallery does with `examples/nsh-gallery-ui`, in the look of Onevra's gallery, for apps and the gallery alike: flat bordered cards and controls, the controls 28 pt tall in 13 pt text on every device. It holds `ui.Theme`, Onevra's with the gallery's roles in `Palette.Colors` derived from Onevra's, its finishes and motion, and components written as structs with a `Render` method (`ui.Button{Text: …, Variant: ui.ButtonSecondary, OnPress: …}`): controls, popups, menus, panels, navigation, shells, lists and a tree, form fields, settings, images, and `ui/chat`, a conversation (`chat.FloatingConversation`, `chat.Turn`, `chat.Composer` and their chips, pickers and meters, ported from Onevra's `chat`). What only the gallery's demos share is in `examples/gallery/parts.go`. Each reads its colors where it is built, so a subtree's theme reaches it, and reads `ui.ColorsOf`, `ui.TintsOf` and `ui.FinOf` only inside a `StyleFunc`, a `Draw` or another function the engine runs again, as NSH's styles read its colors: a change of theme or appearance restyles every view in place, keeping scroll positions and state. Each color is a method (`colors.Text()`), and a method value (`colors.Text`) is a color to hand on; `colors.Now()` reads them all at once for a `Draw`, and `ui.Themed` holds a component's styles by part for its `StylesFunc` or a `TextView`'s `StyleOf`. `TestAppearanceRestylesInPlace` checks every demo against a fresh dark build, and the package's own every component.

Every `base` part style has a `…StyleFunc` (`DataTable`, `Editor` and `DiffView` a `StylesFunc`, `TextView` a `StyleOf`), as `core.Span` has, so an app restyles them in place too.

## Departures from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| shadcn's roles from `nsh.json`, as Tailwind tokens | Onevra's roles as typed fields of a Go value, plus named ones (shadcn/ui's among them), which classes name in kebab case | Go code reads the fields; the user picked Onevra's palette |
| One theme per engine | A theme and an appearance per subtree | Apps can theme a sidebar or panel apart |
| A theme file in `nsh.json` | `ParseTheme` over an embedded file | Go embeds files itself; the tool stays out of it |
| `nsh.json`'s `tailwind`, one per engine | `Theme.Tailwind`, per subtree; `nsgo.yaml`'s `tailwind` for TSX | It travels with the theme a subtree takes |
| `env()` variables of any edge on any side | Each side's own edge's | `PaddingEnv` holds one term per side ([styles](styles.md)) |
