---
title: Layout
description: Flexbox, grid, sizes and positioning, with the safe area and the keyboard in the layout.
group: Core concepts
order: 51
---

## Boxes

Every element is a box laid out by the engine. The defaults are React Native's: a column flexbox, children that keep their size unless they grow or shrink, items stretched across the cross axis, and sizes that include padding and borders.

```go
core.View{
	Style: core.Style{Direction: core.Row, Gap: 12, Padding: core.All(16), AlignItems: core.AlignCenter},
	Children: []core.Node{
		core.Text{Content: "One"},
		core.Text{Content: "Two"},
	},
}
```

`Column`, `Row`, `ColumnReverse` and `RowReverse` set the main axis; `Wrap` and `WrapReverse` let children flow onto more lines. `JustifyContent` places them along that axis and `AlignItems`, or `AlignSelf` on one child, across it. `Gap`, `RowGap` and `ColumnGap` separate them, and the `style` padding and margin use `core.All`, `core.Symmetric` or `core.Edges`.

## Sizes

`Width`, `Height`, `MinWidth`, `MaxWidth` and `FlexBasis` take `core.Pt` for points or `core.Pct` for a percentage of the parent. `FlexGrow` shares the free space and `FlexShrink` lets a box give some back; a box with neither keeps its size. `AspectRatio` is width divided by height. `AutoMargin` takes the free space on the sides it names, so one side pushes a box to the other.

```go
core.View{Style: core.Style{Direction: core.Row, Gap: 16}, Children: []core.Node{
	core.View{Style: core.Style{Width: core.Pt(220), FlexShrink: 0}},
	core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0)}},
}}
```

A box's content gives it its size until a width or height says otherwise. `core.MinContent`, `core.MaxContent` and `core.FitContent` take the sizes CSS names: a text's min-content width is its widest word. `MinWidth: core.Pt(0)` lets a text shrink and wrap inside a row, where its natural width would keep the row wide.

A screen that fills the window takes `FlexGrow: 1` and `FlexShrink: 1`, as does each box between it and a list or scroll view that should scroll. With `FlexGrow` alone, a box grows to its content, so a long list stretches it past the window instead of scrolling.

## Grid

`DisplayGrid` lays children out in CSS grid. `GridTemplateColumns` and `GridTemplateRows` take CSS track syntax, and `GridRow` and `GridColumn` place an item by line or span.

```go
core.View{Style: core.Style{
	Display: core.DisplayGrid,
	GridTemplateColumns: "repeat(3, 1fr)",
	GridAutoRows: "44px",
	Gap: 12,
}, Children: []core.Node{
	core.View{Style: core.Style{GridColumn: core.GridLines{Start: core.GridSpan(2)}}},
}}
```

`repeat(auto-fill, minmax(160px, 1fr))` makes as many columns as fit. `GridFlow` chooses the order the grid fills, and `JustifyItems`, `JustifySelf` and `AlignItems` place items in their cells.

## Position

`Relative` is the default and keeps a box in the flow; `Top`, `Right`, `Bottom` and `Left` offset it. `Absolute` takes it out of the flow and places it in its nearest positioned ancestor. `Fixed` places it in the root view or its modal, whatever scrolls, and `Sticky` pins it in the nearest scroll view once it reaches its inset. `ZIndex` stacks boxes among their siblings.

```go
core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(8), Right: core.Pt(8)}}
```

`Static` keeps a box in the flow and ignores its insets, so absolutely positioned children find an ancestor above it. `Overflow` clips children to a box, and `DisplayNone` takes a box out of layout.

## Direction and the environment

`LayoutDirection` lays a subtree out left to right or right to left, over the app's direction: `DirectionRTL` and `DirectionLTR` set it, and the default inherits. Left and right in margins, paddings, borders and insets swap with the direction, so they are starts and ends as in React Native. `PhysicalEdges` and `PhysicalBorders` keep named sides where they are. `WritingDirection` orders runs of mixed-direction text, and `WritingAuto` takes it from the first strong letter.

`PaddingEnv` and `MarginEnv` add the environment to a side. `core.EnvSafeArea` is the safe area's inset on that edge and `core.EnvKeyboard` the software keyboard's height, so content stays above the keyboard while its move animates with it.

```go
core.View{Style: core.Style{
	Padding: core.All(16),
	PaddingEnv: core.EnvEdges{Top: core.EnvSafeArea, Bottom: core.EnvSafeArea},
}}
```

`core.EnvAll(core.EnvSafeArea)` names every edge, and `cx.Env().SafeArea()` and `cx.Env().KeyboardHeight()` read the values in code.

## Responsive classes

`Class` takes Tailwind CSS classes: `flex`, `flex-col`, `flex-wrap`, `items-center`, `justify-between`, `gap-3`, `p-4`, `w-full`, `grid`, `grid-cols-3`, `col-span-2` and the rest. Breakpoints follow the window, and the display's appearance, the direction and reduced motion follow the environment:

```go
core.View{Class: "flex flex-col gap-3 p-4 md:flex-row md:gap-6 dark:bg-zinc-900 rtl:flex-row-reverse"}
```

`sm:`, `md:`, `lg:`, `xl:` and `2xl:` apply from 40, 48, 64, 80 and 96 rems wide; `max-md:` applies below one. `portrait:`, `dark:`, `rtl:`, `motion-reduce:`, `ios:`, `android:` and `web:` follow what the environment says. Classes merge under a `Style`, and a state's class (`hover:`) becomes a state style, so the two can be mixed.

## Next

- [Styles and themes](styles.md): colors, corners, shadows and the look of a subtree.
- [Text](text.md): how text takes its size and wraps.
- [Lists and scrolling](lists.md): content that scrolls and rows that recycle.
