---
title: Lists and scrolling
description: Scroll views with native gestures, and lists that recycle rows for any number of items.
group: Core concepts
order: 55
---

## A scroll view

`core.Scroll` is the platform's own scroll view, so flings, bounces, the scroll bar, snapping and touch scrolling are native. Its children are laid out in a column, or a row with `Horizontal`, as long as they need.

```go
core.Scroll{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
	core.View{Style: core.Style{Padding: core.All(16), Gap: 12}, Children: rows},
}}
```

The scroll view fills its container and the padding goes on its content, so the scroll bar stays against the edge. It shrinks to fit its parent unless its style says otherwise. `HideIndicator` hides the platform's indicator, and `Bar` draws the app's own.

`Behavior` and `BehaviorFunc` choose how it scrolls: `Paging`, `SnapToInterval` and `SnapToOffsets` with `SnapToAlignment`, a `DecelerationRate`, `NoBounces`, `ContentInset` and the keyboard's rules. `MinZoomScale` and `MaxZoomScale` let pinches zoom, `OnRefresh` and `Refreshing` make a pull refresh it, and `OffsetValues` follows its offset on the main thread for effects that move with it.

```go
page := core.Scroll{
	OnScroll: func(e core.ScrollEvent) { y.Set(e.Offset.Y) },
	Behavior: core.ScrollBehavior{
		Paging:                    true,
		KeyboardDismissMode:       core.KeyboardDismissOnDrag,
		KeyboardShouldPersistTaps: core.KeyboardTapsHandled,
	},
	Children: []core.Node{...},
}
```

`OnScroll` reports the offset at most once a turn, or once per `ScrollEventThrottle`. `OnScrollDragStart`, `OnScrollDragEnd`, `OnMomentumStart`, `OnMomentumEnd` and `OnScrollEnd` bracket the user's gestures. `Ref` scrolls from code with `ScrollTo`, `ScrollToEnd`, `ZoomTo` and `ScrollToIndex` for a list.

## A list

`core.List` renders long data on a native scroll view while creating only the rows near the screen. `Count` says how many rows there are and `Row` builds one; `Key` gives each row an identity, so rows that stay keep their cells as rows come and go above them.

```go
core.List{
	Count:         func() int { return len(items) },
	Key:           func(i int) any { return items[i].ID },
	EstimatedSize: 56,
	Row: func(cx *core.Scope, r core.ListRow) core.Node {
		item := items[r.Index()]
		return core.View{Style: core.Style{Padding: core.All(12)}, Children: []core.Node{
			core.Text{Content: item.Title},
		}}
	},
}
```

`Header`, `Footer`, `Empty` and `Separator` hold the content around and between rows. `NumColumns` places rows side by side, `Gap` and `ColumnGap` separate them, and `Horizontal` scrolls across. `StickyHeaderIndices` pins those rows to the start of the viewport until the next one pushes them out, and `StickyHeaderOffset` keeps a header above them clear.

`OnViewableItemsChange` reports the rows in view by `ViewabilityConfig`'s thresholds, and `OnLoadMore` runs as the viewport nears the end, once per number of rows. `OnLoadMoreAtStart` does the same at the beginning, and `InitialScrollIndex` opens the list at a row. `Alignment: core.ListAlignEnd` rests a list at its end, as a chat does with its rows in reading order: it opens at its newest row, follows new rows while the user is at the end, and holds a short conversation at the bottom. `Inverted` puts row 0 at the end instead, and `MaintainVisibleContentPosition` follows the end of a growing list while the user is near it.

```go
core.List{Ref: list, Count: count, EstimatedSize: 64, Row: row, OnLoadMore: loadMore,
	StickyHeaderIndices: headers(cx), Footer: core.View{Style: core.Style{Height: core.Pt(24)}}}
```

`list.ScrollToIndexAt(index, 0.5, 0, true)` scrolls a row to the middle of the viewport, and `list.ScrollToEnd(true)` to the end. `list.ListPosition()` reads where the viewport starts, a row and how far into it, to keep a reading position: `list.ScrollToIndexAt(p.Index, 0, -p.Offset, false)` brings a list made anew back there. A list can also lie in one page with other content: set `Viewport: core.ListViewportParent`, and the scroll view around it scrolls the list with the rest.

## Tables

`base/datatable` shows a table of a million rows with virtualized rows and columns, a sticky header, columns fixed at the left (`Fixed`) or the right (`FixedRight`) while the others scroll under them, sorting, selecting, resizing and moving columns, and the app's own cells. `base.Table` is a plain table of the app's rows for smaller data, and `base/plot` draws charts.

## Next

- [Animation](animation.md): effects that follow a scroll view.
- [Text](text.md): rows of text that wrap.
- [Navigation](navigation.md): screens whose content scrolls.
