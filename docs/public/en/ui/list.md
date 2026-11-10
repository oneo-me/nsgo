---
title: List
description: Rows of any number, built only in view, with selection and activation.
group: Components
order: 38
---

`ui.List` renders rows on a native scroll view and builds only the rows near the screen, with selection, activation and the keyboard.

```go preview=list/basic height=280
ui.List{
	Count:         func() int { return len(items) },
	Key:           func(i int) any { return items[i].ID },
	EstimatedSize: 44,
	Row: func(cx *core.Scope, row core.ListRow) core.Node {
		item := items[row.Index()]
		return ui.Text(item.Title, 13, ui.Regular, colors.Text)
	},
}
```

## Rows and identity

`Count` says how many rows there are and `Row` builds one. `Key` gives a row its identity: rows that stay keep their cells and their state as rows come and go above them. Use an ID rather than the index when rows can be added, removed or reordered; the index is a signal that follows the cell as it shows another row.

```go
ui.List{Count: count, Key: func(i int) any { return items[i].ID }, Row: func(cx *core.Scope, row core.ListRow) core.Node {
	return ui.Text(items[row.Index()].Title, 13, ui.Regular, colors.Text)
}}
```

`EstimatedSize` is a row's expected height, 50 points when 0. Rows may have different heights; the list measures the ones it builds and estimates the rest.

## Layout

`Header`, `Footer` and `Empty` hold content around the rows and while there are none. `Gap` separates rows, `Columns` lays them side by side, and the list's padding is the content's, so the scroll bar stays at the edge. Give the list a height or let it fill its parent.

## Selection and activation

`ui.List` takes a `Selection`, made with `ui.NewSelection(cx, multiple)`. Keys, labels, activation and the keyboard follow it: Up, Down, Home and End move the cursor, Shift extends, Mod+A chooses all, Space toggles in a multiple selection, and Enter or a double click activates a row.

```go preview=list/selection height=320
selected := ui.NewSelection(cx, true)
ui.List{
	Count: count, Key: func(i int) any { return items[i].ID },
	Selection: selected,
	OnActivate: func(i int) { open(items[i]) },
	RowLabel:   func(i int) string { return items[i].Title },
	Row: func(cx *core.Scope, row core.ListRow) core.Node { return rowContent(cx, items[row.Index()]) },
}
```

`TapSelects` makes a tap on a phone select rather than activate. `RowLabel` is what assistive technology reads and what typeahead matches.

## Loading and scrolling

`OnLoadMore` runs as the end comes near, once per number of rows. `Ref` scrolls with `ScrollToIndexAt`, `ScrollToEnd` and `ScrollToIndex`. `core.List`, the plain list under it, also has `OnLoadMoreAtStart`, `Inverted` for a chat, `StickyHeaderIndices`, `MaintainVisibleContentPosition`, `Viewport` and `ViewabilityConfig` for the low-level cases ([Lists and scrolling](../lists.md)).

## Accessibility

The list is one list or listbox, named by `Label`, with a row's text or `RowLabel` as its name. A selected list reports the cursor's row and announces it as the keyboard moves. A screen reader moving out of view asks the list to scroll there, which builds the rows it reaches.

## Platform

The native scroll view and the virtualization are the same on every target. Rows are recycled, so a row's state must follow its `Key`; keeping build state in the app rather than in a row is the usual rule.

## Related

- [Lists and scrolling](../lists.md): the full guide, including sticky headers and loading more.
- [Scroll](scroll.md): a plain scroll view.
- [DataTable](../lists.md#tables): a table of many rows and columns.
- The gallery's List, Table and DataTable pages.
