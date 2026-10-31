# Scrolling and lists

How scroll views behave and report, and how `List` windows, measures, anchors and pins its rows. Read this before changing `Scroll`, `List` or a backend's scroll view.

## Scrolling

A scroll view follows NSH's `scrolling-and-lists.md`: `ScrollBehavior` holds paging, snapping, deceleration, bounces, the content inset and the keyboard props of `Scroll` and `List`, and `ScrollConfig` carries it with refreshing and zooming in `ItemScrollConfig`.

- Offsets are UIKit's `contentOffset`: a `ContentInset` makes them negative at the start. `ContentOffset` is an `ItemScrollTo` sent after the first commit.
- `ScrollBehavior.SnapTarget` decides where a release snaps, as NSH's `snap_target`. iOS and macOS call it, Windows and Linux through `internal/platform/scrollbar`'s `Settle`; Android's `Scrolling.java` ports it, and the web marks `SnapPoints` for CSS scroll snapping.
- `OnScroll` runs at most once a turn, and at most once per `ScrollEventThrottle` when set (NSH's `scrollEventThrottle`): the core holds an offset that comes sooner and reports the latest when the interval has passed, on the graph's clock. The platforms report every move, which the core's hit tests need.
- Every scroll view reports its offset as it moves, so the core's hit tests and touches find what shows under a point; a scroll lays out again only while an overlay is placed beside an anchor.
- Platforms report drags, momentum and refreshes with the offset and the zoom scale (`Event.Value`); the core adds the content and viewport sizes for `ScrollEvent`. After `EventRefresh` the config goes out again, so a spinner the app did not keep stops.
- iOS uses UIScrollView's own props and UIRefreshControl, Android its scroll views with its own glides and pull, and the web CSS. macOS keeps [offsets of its own](#macos-scroll-views) and ignores the keyboard props.
- `ScrollBehavior.RefreshIndicator` is the height of an indicator the app draws itself, over the top of the scroll view, with styles bound to `PullValue`: the spinner is hidden, a release past that height refreshes, and refreshing keeps that room above the content (`RefreshThreshold`, `RefreshInset`; 64 and 56 points for the spinner). Platforms set the value bound with `AnimBindPull` to how far the content is pulled past its top, the room included, on their main thread. UIRefreshControl cannot give way to an indicator, so iOS then detects the pull as a drag ends and keeps the room with the top inset, as NSH's Mac idiom does.
- On iOS a scroll view's pan waits for the screen's swipe back to fail, so an edge drag goes back.
- Windows and Linux scroll with the wheel, the bar and touchscreen fingers (`scrollbar.TouchDrag`): a finger past the slop drags the innermost scroll view that moves its way, rubber-banding past the edges unless `NoBounces`, then glides by the deceleration, snaps or springs back, and a pull past the top refreshes with the spinner or the app's indicator. A wheel settles snapping views when it rests. Windows routes a wheel to the views along its axis, a vertical one to horizontal views only where none is vertical, as macOS does. They leave the keyboard props alone.
- Windows draws macOS's overlay indicator (`scrollbar.Bar.Indicator`): 6 points thick, rounded, 4 from the edge, fading 700 ms after the view or the pointer last moved and shrinking past the edges. Linux keeps its own bar, shown while the pointer is over the view.
- iOS and Android keep a focused field in a scroll view clear of the software keyboard: the keyboard's height joins the bottom inset while it shows, and the field scrolls into view with the keyboard's motion.
- Zooming (`MaxZoomScale` above `MinZoomScale`, `Ref.ZoomTo`) is iOS's zoom view, Android's `ZoomScroll`, a scaled content element on the web and the container's bounds on macOS; Windows and Linux ignore it, as NSH's desktops do. Mouse drags never scroll content.

### macOS scroll views

macOS follows NSH's `scroll.rs` (`internal/platform/macos/scroll.go`, `wheel.go`); the rubber band, the indicator's geometry and fade, the trackpad's resampling and revealing are `internal/platform/scrollbar`'s.

- A scroll view is a plain view whose container holds and clips its children. A moving offset is the container layer's `sublayerTransform`, one layer write a frame. The container's bounds origin takes it 140 ms after the last move, once the frame clock rests or the view has rested for a second, since `setBoundsOrigin:` walks the whole subtree.
- AppKit's hit testing, conversions and tracking areas follow the bounds, so presses, mouse moves, a scroll gesture's start, selections, a focused field and the test host commit first. Accessibility frames catch up at the commit.
- Trackpad gestures rubber-band past the edges unless `NoBounces` and spring back; the system's momentum follows and stops at the edges. Their events are shown in step with the display, one event interval back, so no frame moves twice or not at all.
- Snapping views snap from the release, with `DecelerationRate` projecting it, instead of taking the system's momentum. Mouse wheels move 10 points a line, and snapping views settle once the notches stop.
- A gesture and its momentum stay on the view they began on, and wheel notches while they come within 200 ms and the pointer stays over it. Views along the wheel's axis take it, and at the edges a gesture stretches the innermost view along it.
- The indicator shows while the view moves and while the pointer is over it (unless the system shows scroll bars only while scrolling), fades after 700 ms, shrinks past the edges, and the mouse drags it as a scroll bar. A press that stops a snap is no press.
- Pulling a refreshable view past its top shows the spinner's spokes as it goes. A field that gets the focus scrolls into view, and a selection dragged past the edge scrolls on the frame clock.
- Offsets are reported within the content, as NSH's, once a frame.
- A zooming view (`zoom.go`) departs from NSH's, which ignores zooming: the container's bounds hold the zoom, so AppKit scales the content and hit tests it where it shows, and a new zoom commits at once. It moves along both axes, without resampling; trackpad pinches zoom around the pointer, past the limits unless `NoBouncesZoom`, and spring back.

## Lists

`List` follows NSH's `<List>` and its `ListState` (`liststate.go`). It mounts only the rows within a viewport of the visible ones, in absolutely placed cells it reuses: a cell's content is built once, and its `ListRow` signals move it to another row. Item sizes start as an estimate and are corrected as cells are laid out; rows (`NumColumns` items, as long as the longest) sit in a Fenwick tree, so offsets and the row at an offset cost O(log n). The window is refilled after each layout, within the same commit.

- With `Key`, a change of the keys is a splice by their common start and end: kept rows keep their sizes and cells.
- A splice or a measurement wholly above the viewport keeps the visible rows in place with an `ItemScrollBy`, which platforms apply unclamped, carrying a drag, glide or animated scroll along.
- `Inverted` lays the content out from the end and keeps its distance from it; `MaintainVisibleContentPosition` follows the end of other lists within its threshold. The header, footer, empty view and separators (in the row's cell) are placed as rows are.
- Platforms count the `ItemScrollTo` and `ItemScrollBy` items each scroll view applied and report the count in `Event.Count`; the core replays the scrolls a report predates, as NSH's `ScrollState::issued` does.
- `Ref.ScrollToIndexAt`, `ScrollToEnd` and `InitialScrollIndex` resolve with the sizes known and pin their target. The list follows it with an `ItemScrollBy` or a new animated `ItemScrollTo` as rows are measured, until the viewport's are or the user scrolls away.
- Sticky rows (one column, not inverted) are computed in the core and sent as `ItemStickyCells`: each cell's travel in offsets, which macOS, iOS, Windows, Linux and Android apply on every scroll. Sticky cells move to the end of their list's children, so they draw above the others.
- The web puts each sticky cell in a track spanning its travel and lets CSS `position: sticky` pin it, clipping views with `overflow: clip` where the browser has it.
- A list's `StickyHeaderOffsetValue`, like a sticky view's `AnimatedStyle.StickyInset`, pins its headers an animated node's value from the start, as under a header that hides; the backend pins them again as the node changes, without a commit.
- `OnViewableItemsChange` reports rows by `ViewabilityConfig`'s thresholds, after `MinimumViewTime` on the graph's clock.
- `OnLoadMore` runs once per count as the viewport comes within `LoadMoreThreshold` viewports of the content's end, as the list opens too.
- `OnLoadMoreAtStart` is its counterpart at item 0's end, once per approach, and only after the user or a scroll command moved the list (`listState.moved`).
- With `OnLoadMoreAtStart`, an inverted list keeps its distance from the end only while row 0 shows. Scrolled away, a splice or a measurement before the viewport keeps the visible rows in place (`keepsVisibleRows`), so pages loaded there arrive out of sight.
- With `ListViewportParent` the list is a plain clipping view whose spacer gives it its full length in its parent's layout. It is virtualized against the nearest scroll view on its axis around it, else its root, modal, overlay or screen.
- Such a list's anchoring and scroll commands scroll that view, and its sticky rows go in that view's `ItemStickyCells`, the last stopping at the list's end. The scroll view props, `Inverted` and `MaintainVisibleContentPosition` are ignored; NSH warns of them, NSGo does not yet.

Binding is paced, as NSH's: a fast scroll or a jump asks for a screen of cells at once, and each bind builds or rebinds a cell's content.

- The viewport, twice the last scroll step ahead of it (none after a jump longer than the viewport) and the sticky rows bind in the commit that asks for them.
- The rest of the window waits and binds nearest the leading edge first, `paceCells` (4) a list per commit while the commit has run less than 2 ms, then on the next display frames (`Tree.Pacing`).
