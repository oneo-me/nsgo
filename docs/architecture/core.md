# Core

`core` turns Go components into one `MountBatch` per commit. Components run once; signals drive bindings, regions rebuild only what changed, and the commit lays the tree out and sends the differences. The mount protocol, the events and the values `core` shares with backends live in `internal/driver`; `core` aliases those its app API reaches in `shared.go`, and `TestPublicAPINamesNoInternalType` fails when one is missing.

Read this before changing `core`, `internal/engine` or the mount protocol a backend implements; [text](text.md), [scrolling and lists](scrolling.md), [interaction](interaction.md) and [services](services.md) cover those parts of `core`.

## Reactive graph

The graph follows NSH's (Solid 1.9's scheduling): signals, memos, render effects for element bindings and user effects for the app, run in that order after each batch.

- Every computation and scope is an owner. Disposing an owner disposes what it created and runs its cleanups, last first, as `testing.T.Cleanup` does.
- Go has no goroutine-local state, so code finds its graph through `cx *Scope`. While the graph runs code, what that code creates belongs to the innermost running owner, whichever `cx` it captured. Handlers run under the owner their element was built in.
- The graph belongs to the engine goroutine and never locks. Other goroutines reach it with `Scope.Post`. `Scope.Context` is cancelled when the owner goes, so work started for a screen stops with it.
- A reader and each source it read keep each other's index in their lists, so a computation that runs again unlinks in constant time however many read the same source: a table's thousand cells reading one signal.
- Timers go through the `Clock` interface. Tests inject a `ManualClock`.
- A panic in app code is recovered and sent to the nearest `Catch` above the owner it happened under. With no `Catch`, the host reports it and the app keeps running.

## Elements and components

Host elements (`View`, `Text`, `Scroll`, `List`, `Image`, `Canvas`, `Overlay`, ...) are structs with exported fields whose zero values are defaults, as in `http.Server`. Apps take `Input`, `Screen`, `InputAccessoryView` and `Modal` through `base.Input`, `base.NavPage`, `mobile.InputAccessoryView` and `base.Dialog`, as NSH's apps do. [Hosted views](native-views.md), [images](images.md), [canvas](canvas.md) and [shaders](shaders.md) cover their elements.

- A field that can change takes a function (`StyleFunc`, `ContentFunc`, `Value`, `Disabled`), bound by one render effect per part.
- Styles compose with `Style.Merge`, and text fields inherit down the tree as in CSS.
- State styles merge over the rest in NSH's order: `Checked`, `Indeterminate` and `Selected`, which the app sets, then `Hover`, `Focus` and `Active`, then `Disabled` over all; a `GroupStyle` follows its group's states first ([styles](styles.md)).
- The engine tracks `Hover`, `Focus` and `Active` from the events it asks the platform to report. Windows and Linux hit-test hovers themselves (`internal/platform/hover`).
- `PartStates` styles a view in states it does not report, as a control's part, NSH's `project_checked`.
- Props only one platform has sit in a field named after it, as NSH's `ios`, `android` and `desktop` objects: `View.IOS.BackdropMaterial`, `View.Android.Ripple`, `View.Desktop.WindowDrag`.
- What two platforms have stays top-level, so `InputAccessoryViewID` and `LiveRegion`, iOS's and Android's in NSH, are top-level because NSGo implements them on more.
- A `Ref`, usable at its zero value, names a mounted element for anchoring, focus, scrolling and its bounds.

A component is any type whose `Render(cx) Node` returns its nodes, like `http.Handler`. Its fields are its props. `Render` runs once under a scope of its own, so `Key.Provide` and cleanups stay inside it. `Func` turns a function into a component.

## Regions

`Show`, `Each`, `Dynamic` and `Catch` mount as regions. A region is a node with no native view, laid out as part of its parent's children, and owned by one render effect that rebuilds its span.

- Branches and rows belong to the region's owner, not to its effect, so they outlive the effect's runs and go only when replaced.
- `Each` matches rows by key. A kept row gets its item and index through signals, and only the rows that moved are reinserted.
- An item of plain values (numbers, strings, arrays and structs of them) notifies its row only when it changes; one holding pointers, slices or interfaces notifies on every read, as what it points at may have changed.
- Go methods cannot take type parameters, so `Each` is a generic function rather than a method or struct.
- Region nodes are never laid out. Layout marks pass through them, so a change inside a branch or row still reaches the root.
- A `Holder` is NSH's render keys as a value: content built once, owned by the scope given to `NewHolder` rather than the running owner, and shown in the newest place mounted with `Place`.
- A held content moves to its place with its native views, keeping its state; with no place it stays built out of the tree, its overlays hidden.

## Layers and navigation

`Overlay` and `Modal` stay in the shadow tree where they are built, for their owner, context, inherited text and lifetime. Their native views are hoisted into their host: the nearest modal around them, or the root view. Each is a layout root of its own at the host's size, placed after the main layout.

- An overlay with an anchor goes beside it as NSH's `place` does: the side asked unless only the opposite side fits or has more room, aligned along it, then clamped into the safe area.
- A modal provides the insets its content keeps clear of; a screen does too, without the top when it has a header. `Insets` reads them.
- `EventBack` (Escape, the back button) goes to the topmost overlay that listens for cancel, else the topmost modal's `OnRequestClose`, else the `Scope.OnBackPress` handlers newest first, else the innermost stack that can pop.
- A back gesture that begins (Android's predictive back, iOS's swipe) sends `EventBackStarted`; the core answers `ItemBackPreview` with the screen a back would pop and shows the one below. `EventBackCancelled` hides it again.
- `ItemHistory` counts the backs the app can take: screens above each stack's first and modals shown. The web keeps that many browser history entries when the app is the whole page, so the browser's back goes back in the app.
- `OnBackPress` stays in `core`, where NSH's apps take it from `nsh/mobile`, because it also hears Escape and the browser's back.
- A popped screen reports `OnDismiss` and stays hidden until the app removes it; the last screen never pops.
- Where the environment has `HeaderHeight` (iOS, Android), screens get NSH's system header: the core pads the content below the bar and the status bar, sends `ItemScreen`, and lays `HeaderLeft`, `HeaderRight` and `HeaderTitle` out as roots the platform puts in the bar. `Screen.IOS.HeaderRightSystemItem` with a `HeaderRight` asks iOS for UIKit's own bar button in its place, whose press iOS reports on `HeaderRight`'s first view that takes presses, as NSH's `headerRightSystemItem`.
- `Header` is then not mounted; elsewhere it is the screen's first child.
- iOS gives each screen a `UINavigationBar` of its own with a back item, Android a `Toolbar`; their back buttons and iOS's leading-edge swipe send `EventBack` with the screen's ID.
- iOS slides the screens and their bars as `UINavigationController` pushes and pops: 0.35 s on the keyboard's curve, the screen below moving 30% of the width under a dimming view.
- A hidden screen has no layout, so a batch that changes a stack first snapshots the screen it shows, and the slide moves that snapshot; the swipe back moves the screens themselves.
- Each platform measures its bar with what it holds and reports it (`EventScreenHeader`), which replaces the environment's estimate.
- A transparent header and a large title lay the content under them: each vertical scroll view whose top they cover gets that part as `ScrollConfig.HeaderInset`, added to its top content inset.
- iOS collapses a large title into the bar once its scroll view scrolled half the title's row under it, and brings it back above that, as UIKit's navigation controller does.
- `mobile.TabBar` is the system's tab bar, a native component iOS (`UITabBar`) and Android (a Material 3 navigation bar drawn by the runtime's `TabBar.java`) register themselves; elsewhere it shows nothing.
- iOS presents a modal with `ModalPageSheet` or `ModalFormSheet` as a sheet view controller, which reports its size (`EventModalSize`) and its swipes (`EventRequestClose`, "swipe"); a swiped sheet stays hidden until the app removes it.
- `IOS.SupportedOrientations` goes to the controllers' `supportedInterfaceOrientations`. On Android a modal over the surface starts below the status bar unless `Android.StatusBarTranslucent`.
- `EventPressOutside` reports presses outside an overlay, ignoring those on its anchor or on overlays above it.
- A view's `Tooltip` is a hint overlay above it that the core shows after half a second of hover on desktops; `Overlay.PointerEvents` keeps it from taking the pointer.
- A `mobile.InputAccessoryView` is a layout root as wide as the surface and as tall as its content, which no view holds: its views are sent before the tree's, and an input's `InputConfig.Accessory` names it.
- iOS makes it the field's `inputAccessoryView`; Android shows it above the keyboard while the field has focus; other platforms leave it unmounted.

## Commit

`Tree.Commit` runs up to eight rounds. `OnLayout` handlers' changes go into the same batch. A commit costs what changed, not what the tree is: each change marks its node, `mark` for its parts and `markResolve` for its styles or content, and the walks below reach the marked nodes only, through the views they are emitted in (`dirtyKids`) and their shadow parents (`resolveKids`). What every node reads (the environment, the layout direction, the font scale, an accessory's ID) marks them all.

1. **Resolve.** Inherit text styles into the marked nodes, and into every node under one whose inherited fields changed; prepare text whose content or style changed.
2. **Layout.** CSS block, flexbox and grid from `internal/layout` over each view's native children; overlays and modals are layout roots of their own. Results stay cached until a node changes, and frames are rounded to whole points as Taffy does.
3. **Emit.** Compare each marked view's parts with what was sent: view style, props, events, text, input, scroll; layout marks the views it moved. Then sync children against the sibling order last sent, reinserting only the views outside the longest run already in order.

Layout is incremental, as NSH's layout tree:

- A change stops at a relayout boundary (`layout.Boundary`): an in-flow flex item of fixed points both ways that does not shrink, has an auto basis and is not aligned by its baseline.
- A boundary is laid out again on its own with what its parent gave it last, and its parent only when its size, baselines or overflow changed (`layout.Relayout`).
- The deepest boundaries go first: an outer one laid out from its caches would leave an inner one's subtree as it was and clear its mark.
- A restyled view is no boundary, and overlays and modals always mark the root's layout, which places them.
- A text whose content changed lays nothing out when it measures as before for every input layout asked of it since its cache was cleared (`layout.MeasuresAsBefore`), or when only its colors changed.
- Texts with attachments always lay out. A restyle lays a view out only when the layout style it maps to changed.

Tests keep the marks honest: with `driver.CheckCommits`, which `nsgotest` and the core's tests set, each commit is followed by one that lays every node out anew and compares every node, and panics on what the first missed.

`layout.go` maps styles with React Native's defaults, as NSH does:

- Column direction, no shrinking and no automatic minimum sizes.
- `flex-start` content alignment outside grids.
- Left and right swapped where the subtree lays out right to left.

## Animation

Style `Transition`s, animated values and layout animations run on the platform, with no engine work per frame and nothing for the engine's garbage collector to pause; [animation](animation.md) covers them. `Scope.RequestAnimationFrame` waits for the display's frames, and `Scope.Animate` steps app values on them for what a platform cannot animate itself; every step commits.

## Storage and directories

`Scope.Storage` follows NSH's `nsh/storage`: strings by key with a 5 MB quota, held in memory by a `Store` and loaded once from the platform's `StorageBackend`.

- Engines given the same `Store` share its entries and quota. Each graph keeps a signal per key it read and one for the key set; its own changes bump them at once, and other graphs' changes arrive through `Post`.
- The store hands each change to the backend under its lock, so every graph and the backend see one order.
- Native targets keep `storage.json` in the data directory: `engine.OpenFileStorage` returns one store per file in the process and rewrites the file on a goroutine of its own.
- A file that does not parse is set aside as `storage.json.corrupt`, and one that cannot be read leaves changes in memory. Platforms call `engine.SyncStorage` as the process ends.
- The web keeps entries in `localStorage` under `nsgo:<app id>:`.
- `Scope.DataDir` and `Scope.CacheDir` are named by the app's ID, which the tool passes from `nsgo.yaml`: `engine.AppDirs` on Apple platforms, Windows and Linux, the files and cache directories the runtime passes on Android, and none on the web. The engine makes them as it starts.

## Mount protocol

The protocol is defined in `mount.go` and `events.go`. Backends apply items in order on the main thread, in one hop per batch.

- IDs are dense and reused after `ItemDelete`, possibly later in the same batch.
- `ItemInsert` places a view before a sibling, not at an index, so views a platform adds itself cannot shift positions.
- A view that changes parents, or comes back after `ItemRemove`, is inserted after the round's other inserts, once its new parent is in place.
- Backends move such a view in one step, so scroll offsets and iframes' pages stay: the web with `moveBefore`, parking removed views in a hidden element.
- Each round's deletes come last.
- A text field counts the user's edits. `ItemInputValue` carries the edit count the engine had seen, and the field ignores a value older than its last edit.
- Platforms report only the events in a view's mask, and never wait for the engine. The engine queue is unbounded, merges scroll events per view and keeps only the last resize.

## Where Go departs from NSH

| NSH (Rust) | NSGo | Why |
| --- | --- | --- |
| Builders, `#[component]`, derive macros | Struct literals; components are types with `Render` | Go's idiom for many optional fields; no macros |
| `Reactive<T>` and `IntoReactive` | A `func() T` field beside the static one | A method value such as `sig.Get` already is the accessor |
| Copy handles in a generational arena | Pointers and the garbage collector | Go frees what nothing references |
| `TypeId` context keys | `Key[T]` values declared as package variables | Typed, comparable and cheap |
| A future executor on the engine thread | Goroutines, `Scope.Post` and `Scope.Context` | Go has them built in |
| `catch_unwind` | `recover`, reported as `PanicError` | The same, in Go terms |
| `MountItem` enum | One struct with a `Kind` and pointers to large parts | No allocation per item kind; easy to encode across a pipe |
| `accessibilityActions` as a list in the props | `ViewProps.Actions` points at the element's list, never changed | `ViewProps` stays comparable, so the core diffs it with `!=` |
| Options with defaults of `true` (`closeOnEscape`) | Negative names (`NoCloseOnEscape`) | The zero value must be the default |
| Per-platform image loading and caching | One Go loader, `internal/platform/imageload` | Written once; platforms only decode and draw |

## TSX

`core.Element` is nsh-core's operations for the TSX runtime: an element made detached from an element value, its props replaced, its children placed one at a time, and released ([TSX](tsx.md#cores-part)).
