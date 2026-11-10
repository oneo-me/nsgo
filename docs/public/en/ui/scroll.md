---
title: Scroll
description: The platform's scroll view, with paging, snapping, zoom and refresh.
group: Components
order: 39
---

`core.Scroll` is the platform's own scroll view. Its children are laid out in a column, or a row with `Horizontal`, and it fills its container while the content scrolls.

```go preview=scroll/basic height=240
core.Scroll{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
	core.View{Style: core.Style{Padding: core.All(16), Gap: 12}, Children: rows},
}}
```

The padding goes on the content, not the scroll view, so the scroll bar stays against the edge. The scroll view shrinks to fit its parent unless its style says otherwise.

## Behavior

`Behavior` chooses how it scrolls: `Paging`, `SnapToInterval` or `SnapToOffsets` with `SnapToAlignment`, a `DecelerationRate`, `NoBounces`, `ContentInset`, and the keyboard's rules.

```go preview=scroll/paging height=240
core.Scroll{
	Behavior: core.ScrollBehavior{
		Paging:                    true,
		KeyboardDismissMode:       core.KeyboardDismissOnDrag,
		KeyboardShouldPersistTaps: core.KeyboardTapsHandled,
	},
	Children: pages,
}
```

`HideIndicator` hides the platform's scroll bar and `Bar` draws the app's own. `MinZoomScale` and `MaxZoomScale` let pinches zoom; `NoPinchZoom` ignores them. `CenterContent` centers content smaller than the viewport.

## Refresh and events

`OnRefresh` makes the view refreshable; set `Refreshing` while the work runs. `PullValue` follows how far the content is pulled, for an indicator of the app's own, and `Behavior.RefreshIndicator` sets its height.

```go
refreshing := core.NewSignal(cx, false)
core.Scroll{
	Refreshing: refreshing.Get,
	OnRefresh:  func() { refreshing.Set(true); reload(cx) },
	Children:   content,
}
```

`OnScroll` reports the offset at most once a turn, or once per `ScrollEventThrottle`. `OnScrollDragStart`, `OnScrollDragEnd`, `OnMomentumStart`, `OnMomentumEnd` and `OnScrollEnd` bracket the user's gestures.

`OffsetValues` follows the offset on the main thread, for a header that collapses without a commit per frame. `Ref` scrolls from code with `ScrollTo`, `ScrollToEnd` and `ZoomTo`.

## Layout

`Horizontal` scrolls across. `Behavior.ContentInset` puts room around the content that scrolls with it. For a page that fills the safe area, use `PaddingEnv` on the content, not on the scroll view.

## Accessibility

`Label` names the scroll area and `Role` says what it is, such as a list box. The platform's scroll gestures, keyboard scrolling and screen reader scrolling are kept; `Busy` reports a view that is still loading.

## Platform

Every backend uses the platform's scroll view, so flings, bounces, scroll bars and touch scrolling are native. Edge effects differ: macOS and iOS rubber-band, Android stretches, Windows and Linux stop, and the web follows the browser. `EdgeEffects` and `EdgeBar` add bars over the content where the app wants them.

## Related

- [Lists and scrolling](../lists.md): the full guide to scrolling and lists.
- [List](list.md): rows that recycle.
- [Layout](../layout.md): paddings and the safe area.
- The gallery's Scroll and Lists pages.
