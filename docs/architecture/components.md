# Components

`base` holds NSGo's built-in unstyled components, after NSH's `nsh-base` (`nsh/docs/dev/components.md`, `dock.md`): they own behavior, state and accessibility and take every style from the app. A component is a file of `base`, or a package under it when it is complex: `base/datatable`, `base/dock`, `base/plot` and `base/toast`. Read this before changing a component in `base`; [text view](textview.md) covers `base/textview`, and [editor](editor.md) `base/editor` and `base/diff`.

A component whose root is a view of its own takes that view's `OnLayout` and `Tooltip`, as NSH's components take their host's.

Each part a component styles takes Tailwind classes beside its style, as NSH's `class` and `popupClass` do: `Class` with `Style`, `PopupClass` with `PopupStyle`, `Classes` with `Styles`; `classes_test.go` holds every part to it. A part's class goes under its style and under what the component sets on the part itself (a sheet's place, a handle's 1-point line), as classes go under `Style` in `core`, where NSH puts an app's classes over the latter; important classes (`w-[3px]!`) go over both. Parts whose classes follow a state, a table's rows and cells and a toast's phases, take them through `core.View.ClassFunc`.

## Controls

`Button`, `Checkbox`, `Radio` in a `RadioGroup`, `Switch`, `Tab` in `Tabs`, `Collapsible` and `Progress` hold no value: the app passes it and sets it from `OnChange` or `OnPress`, as NSH's controlled props do.

`Tabs` and `RadioGroup` are one Tab stop (`base/roving.go`, as NSH's `roving.rs`): the item the focus is on, else the selected tab or checked radio, else the first enabled one is `Focusable`. Their arrows, bound in the keymap only while an item has the focus, focus the item they reach and activate it, swapped right to left, and `Tabs` adds Home and End. The items sort in tree order (`Ref.Before`), since an `Each` may mount one between others. `HitSlop` goes to the view of `Button`, `Checkbox`, `Radio` and `Switch`; `Toggle` and `Link` also take `HitSlopFunc`, which follows what it reads.

`Toolbar` and `ToggleGroup` are bars (`barItem`): groups of the same roving focus whose `Button`s, `Toggle`s and `Link`s join them, in a `ToolbarGroup` or a `ToggleGroup` inside a toolbar too, disabled with the bar. An arrow key only moves the focus, along a toolbar's `Axis`, and a toolbar's Tab comes back to the item it left where a toggle group's enters at the first pressed toggle. A `Menu`'s or `Popover`'s trigger is one of a bar's items, with what the app puts in it no item of its own, and with one of a toolbar's menus open the pointer over another's trigger moves there, as a menu bar does.

`Slider` and `NumberInput` share `numeric.go`, NSH's rules of a number in a range with a step and of one typed as text. A drag's value shows at once until the drag ends, without waiting for the app's `Value` to come back, and assistive technology hears it at the end unless a screen reader runs, so a drag sends no props item per move. A slider's `Value` is `SliderValue`, one number or a range, and a number input's `*float64`, nil for an empty field, the shapes TSX's `number | [number, number]` and `number | null` read as. NSH lets go of a drag's value once a TSX app has answered, a turn later; handlers run synchronously here, so the release does.

`OtpInput` is one transparent native field over cells that only show the code, which keeps typing, input methods, paste and the system's code autofill the platform's. Edits are read from what the field reports and apply to the code as typed, so a controlled `Value` a few keys behind is no conflict (`otpEchoes`).

`Accordion`'s and `Collapsible`'s `Reveal` animate the commit that opens or closes them with a critically damped layout spring, and the content, in a `Holder`, comes back as it was. `Avatar` lays its image out transparent over the fallback, so loading changes those two parts and nothing else, with an image node per source so a late event of the old one is not the new one's. `Pagination` makes its row once, a place for each item it can hold, so a page change makes no view. `Tree` flattens the items once per change of them and looks rows up by bisection; a move of the selection wakes the rows it leaves and reaches, spread over 32 signals (`treeFollow`). `Carousel` is a native scroll view snapping to where its slides report they are laid out, a looping one with a period of runway each side and each slide drawn in the period nearest the viewport's center by an animated translation.

`Scrollbar` watches its target with `Scope.WatchScroll` and scrolls it with `Ref.ScrollTo`; its built-in form is [scrolling](scrolling.md#bars-of-the-apps)'s.

`Tabs`' indicator is a view that follows the selected tab (`View.Follow`, `core/follow.go`, as NSH's `follow.rs`). After each layout the core spans the target's frame along the parent's main axis, inside the follower's margins there, and leaves the cross axis to the follower's layout, so a change of tab is one frame item with `FollowTransition`, and a layout that moves the tabs moves it in the same batch.

`Input` is the core's text field under a name in `base`, as NSH's `nsh/base` names its core's, so apps take it with the other controls; `NavPage` is `core.Screen` for a `Nav`'s pages the same way.

- A control's parts (a checkbox's indicator, a switch's track and thumb) follow its states through `View.PartStates`, which styles a view by states it does not report, so assistive technology hears them from the control alone. Content the app puts inside follows them with a `GroupStyle` under the control's `Style.Group`.
- A radio reports checked and selected both, as `gpui-base`'s does, since screen readers read one or the other.
- `Collapsible` makes its content the first time it opens and keeps it, hidden, while closed.
- `Progress` is one accessible element with a range from 0 to 100, and none while indeterminate.

| NSH | NSGo | Why |
| --- | --- | --- |
| A slot's element in a view of its own | The element in the component's row | Go's `core.Node` needs no holder, so `NumberInput`'s frames count from its row |
| `Scrollbar`'s `axis`, vertical by default | `Horizontal` | Go's zero value is the default |
| `Collapsible` and `AccordionPanel` keep their `reveal` as it changes | `Reveal` read as a `Collapsible` is made | Its two forms are different views |
| Items ordered by the core's `watch_order` | `Ref.Before` as they are read | One way to order, as `Tabs` has |

## What only NSGo has

`TagInput`, `Rating`, `HoldToConfirm`, `ImageCompare`, `Stepper` and `Sidebar` have no `base` form yet and live whole in `ui`, and the gallery marks them on their cards. `base.List` is NSGo's own: NSH leaves a flat list of choices to each app's `<List>`, so it holds what such a list needs over `core.List` (the selection, the keys, typeahead and the listbox's roles), and `ui.List` is only its look. `Selection` is state rather than a component: the rows a `List` chooses by press and keyboard, a multiple one with Shift and Mod, and a tree's the same way. `Tree` also takes `Folder` items whose children are read only as they expand, and `SelectOnly`, where a double press or `TreeRow.Toggle`, a chevron's, expands in place of the press. `Slider`, `NumberInput`, `Carousel` and `Progress` likewise take what the look draws around them: marks, a scrub handle, a commit as the field is left, the content's offset, the thumb's ref for a slider drawn by a shader.

## Popups

Popovers, tooltips, menus, context menus, dialogs and sheets share NSH's contract (`popup.go`): `Open` makes them controlled, `OnOpenChange` reports each request with its `Reason`, `OnCancel` every attempt to close, and `OnOpen` and `OnClose` follow what is shown, after a sheet's motion.

They present themselves by `Env.Desktop`, as NSH's do:

| | Desktops | Touch screens |
| --- | --- | --- |
| `Popover` | A press outside closes it and goes on to what is under it | A transparent backdrop overlay takes the press outside |
| `Menu` | A popup at its trigger; submenus cascade beside their item, on hover or press | A sheet at the bottom over `BackdropStyle`; a submenu's items replace its menu's, below its `Back` row, and Escape or back return a level |
| `ContextMenu` | Opens at the pointer (`AnchorBounds` at the click) | Opens as the menu's sheet after a finger rests 400 ms, watched with `OnTouch` so the trigger's own presses and scroll views go on; 10 points of movement cancel |
| `Tooltip` | After the pointer rests half a second | Only when the app opens it |

| `HoverCard` | Opens after the pointer rests `OpenDelay` (600 ms) or the focus comes to the trigger, stays while either is on the trigger or the card, closes `CloseDelay` (300 ms) after both left; Escape and a press outside close it at once | A press on the trigger opens and closes it, a press outside closes it and goes on |
| `Select`, `Combobox`, `DatePicker` | A popup at the trigger, as wide as it or at least that wide, never past the room on its side; a press outside closes it and goes on, leaving the focus where it went, any other close giving it back to the trigger | A sheet at the bottom over a backdrop, below the status bar at most; a search field takes the keyboard only when pressed |

Items reach their menu through a context key (`menuContext`), so the app arranges them freely. A `MenuItem` with `Checked` is a `RoleMenuItemCheckbox`, or the `Role` it gives; its `Indicator` is a part around the app's mark that takes the item's checked and disabled states. A sheet's hidden levels are out of layout and their content out of the tree, so screen readers and tests see one level.

- `Sheet` and a phone's menus are overlays, not modals: `sheet.Motion` (`internal/sheet`) ports NSH's `SheetMotion`. The panel's translation and the backdrop's opacity are animated nodes of one `fade` value and the panel's measured size, so they run on the platform's frame clock; the parts leave the tree when the motion out ends, or after its time where no end is reported.
- With `CloseOnSwipe` a bottom sheet's `PanValues` move a drag value; a release past 30% of its height or 800 points a second asks to close (`ReasonSwipe`), else it comes back. On iOS the panel's pan hands off from the scroll view the drag starts in (`View.PanScrollHandoff`): the drag scrolls it back to its top first, and what goes on past that moves the sheet while the scroll view stays pinned there. Elsewhere the scroll view keeps the drag.
- `View.Tooltip` is the short form: the core shows the node above the view, 4 points away, after half a second of hover on desktops. A `Tooltip` given there is the hint itself, as NSH's: it shows against the view when its own open state says, the core telling it of the pointer (`core.HintTooltip`), so an app opens it on touch screens too. Tooltips never take the pointer (`Overlay.PointerEvents`).
- An overlay popup (`Popover`, `Tooltip`, and a `Menu`'s or `ContextMenu`'s on desktops) comes and goes through `popupMotion` (`popup.go`), as Base UI's `data-starting-style` and `data-ending-style`: `StartingStyle` in its first frame (the next display frame, or 50 ms), so the app's `Transition` runs the entry, and `EndingStyle` as it closes, leaving the tree on the overlay's `OnTransitionEnd`; `OnClose` follows that. Both merge over the popup's style and their classes are important, as a toast's. Without an ending part, or under reduced motion, it goes at once. A desktop menu's submenus take its phases, and closing a menu closes each with its own exit. NSH waits out a transition by asking the core its length; `OnTransitionEnd` here says when it ended.
- `Dialog` stays a `core.Modal` the platform presents. Given `StartingStyle` and `EndingStyle`, or their classes, it comes and goes through `popupMotion` too, with the backdrop's phases following: the modal stays until the popup's `OnTransitionEnd`, and what is going takes no presses. `ui.Dialog` does on desktops, as Onevra's: a dim that fades and a surface that pops in and fades out, above the middle of the window. `AlertDialog` is the system's alert, or with `AlertPresentActionSheet` its action sheet: `UIAlertController`'s (a popover anchored at the bottom on iPad, with an unshown cancel action for a tap outside), a list dialog on Android, stacked buttons with the cancel one last on macOS, the web and Windows.

Popups are placed against what their trigger holds (`Overlay.AnchorContent`), the app's element inside the view a column stretches, as NSH places them against the element itself, and a press anywhere in that view is still no press outside.

## Pickers

`Select` and `Combobox` share `listbox.go`, as NSH's `listbox.rs`, the state machine `gpui-base` leaves to a styled layer: `listShown`, the rows for a query, with a query that holds the last one searching only its matches; the highlight's moves, typeahead, and where the highlight goes as the items change; rows of a `List` with `ListViewportParent`, so a popup is as tall as its rows up to its room and a list of thousands makes the rows in view. A move of the highlight wakes the two rows it leaves and reaches, spread over 32 signals. `RenderItem` gets what an item is, never its state: the row's `Selected` and `Focused` follow through `group-*` styles, so nothing draws again. A desktop chooses as the button goes down, which keeps a search field's focus. `dropdown` (`popup.go`) is the popup or sheet `Select`, `Combobox` and `DatePicker` open; a `Select`'s lays its selected row over the trigger (`Overlay.Cover`, shadcn/ui's `item-aligned`).

`Combobox` keeps the value as the user last left it, ahead of an app's that follows, so toggles in a row start from it and `OnConfirm` reports it on every close. `Command` is the same list in the page: its own state (`palette`, `command.go`) with separators, and a highlight that keeps its item as items change and goes to the first match as the query does. It departs from NSH's as Onevra's palette does, for a launcher's search: every word of a query must match, and exact labels rank before labels it starts, labels holding it and keywords (`FilterCommands`, which `Filter` replaces); a group gathers wherever its items are, showing where its best match ranks; items carry a description, an icon and leading and trailing content for the app's row; `Pending` and `Error` take the rows' place and suppress choosing; `ExpandOnSearch` holds the rows back until there is a query; and `InputSource` selects a keyboard input source for the search. Its keys unbind while an input method composes. `ui.CommandDialog` hosts `ui.Command` as Onevra's `CommandDialog` does.

`Calendar` is 42 cells made once per month, a turn of the months changing what they show, with one Tab stop whose keys turn the months; `DatePicker` holds it, with presets and, for one date with `TimePrecision`, a `TimeField`, which is one transparent native field over segments, as `OtpInput`. `ColorPicker` keeps a color in RGB, HSL, HSB and OKLCH (`colormodel.go`) so a gray keeps its hue, and its parts reach it through a context key. A color the app sets back that the picker reported, as itself or as its text read again, changes nothing, so rounding moves no thumb.

`ui`'s forms draw these parts in Onevra's look and hold no behavior of their own: `SearchField` and `Field` hand a form the input to draw, as `ui.Input`, and `ItemStyleFunc`, `ItemStyleOf` and the like follow a theme. `ui.Select` and `ui.Combobox` value a list's items by their index among `Options`, so a value may be any comparable type. Their words come from `base/locales`.

| NSH | NSGo | Why |
| --- | --- | --- |
| A trigger's role and presses bound on the app's element | A view around it, `Class` and `Style` the view's | Go's `core.Node` cannot be changed from outside, so frames count from the view |
| `value` as `null` or absent, `Date` an enum | `Value func() *string`, `DateValue` with `Range` and zero times unset | Go's zero values |
| `searchable`, `filterable`, `separator` | `Command.NoSearch`, `NoFilter`, `NoSeparator` | Go's zero value is the default |
| A `DateMatcher` of days, bounds or a function | `DisabledMatcher func(time.Time) bool`, made by `DaysOfWeek`, `DatesOutside` and `DatesBetween` | A Go function says all of them; TSX's forms read as NSH's |
| A JS matcher asked in the next turn | Asked as a day is read, once per date | Handlers run synchronously here |
| `HoverCard` delays of 0 and `Infinity` | Zero is the default, negative at once, `HoverNever` for good | Go's zero value is the default; TSX's read as NSH's |

## Toasts

`Toast`, `toast.Stack` and `toast.Manager` are NSH's, which are `gpui-base`'s three pieces with its names; `ui`'s `Toaster` and `Toasts` show them from anywhere in Onevra's look, at any of eight placements, with the stack turned around (`Stack.Reverse`), the swipe's direction and a limit, and the gallery's are those. NSH removed the system toast for them, and so `Scope.ShowToast` is gone.

- `ToastManager[K, V]` is plain data with no timer, generic over the app's ids and values: the app calls `Advance` on its own timer while toasts show, with `Scope.Now`, so a test's manual clock moves it. Its zero value has Sonner's motion, and `Push` takes the timeout directly, where NSH has `new(motion)` and `ToastOptions`.
- `ToastStack[T, K]` takes the toasts as `Toasts`, `Key` and `Toast`, an `Each`, in place of NSH's children each wrapped as an item. Each item is an absolute view placed in the layout at its distance from the edge, as hit tests go by frames and not transforms, so an expanded toast takes its presses where it shows; `TranslateY` plays the way there from where it was, with `ScaleX` and `Opacity`, by critically damped springs on the frame clock. The items' measured heights give the distances and the stack's height, so its hover area follows. Distances count from the line that stays put as the height changes, the anchored edge or a center anchor's middle, so `Reverse` and center anchors move nothing but what changed. An ending `Toast` tells its item (`leavingKey`) and takes no room from then on: it stays where it shows while the others close up, where NSH's toasts hold their place until they are removed, a step Onevra's stack does not have. A finger down (observed with `OnTouch`) expands it as hover does, and a zero-size overlay hears Escape and back while it holds toasts.
- `Toast` is the alert role, announces its `Label` as it mounts, and takes a swipe through the touch responder, claimed once the pointer moved 10 points along `SwipeDirection`. `StartingStyle` applies until the next display frame (or 50 ms), so the app's `Transition` runs the entry; `EndingStyle` and `SwipingStyle` merge over its style, and the phases' classes are made important, as NSH's are. The swipe's translation goes on a view around the root, whose `Transition` backends would otherwise drop; while ending, the root takes no presses.

## Charts

`base/plot` ports `nsh/base/plot` (`nsh/website/elements/plot.md`). A `Plot` holds the rows' columns, its scales and the hover; its marks (`Line`, `Area`, `Bar`, `Rule`, `Dot`, `Cell`, `Grid`, `Pie`, `Arc`, `RadialLine`, `RadialGrid`, `Sankey`, `Treemap`), `Axis` and `Label` draw what it holds. Each mark is a canvas filling the plot and the bleed around it its strokes and curves need, a canvas per slice, node, ribbon or class where each paints with styles of its own; it records once when its points, the plot's size or scales change, and paints with the paint properties it inherits ([canvas](canvas.md#view-paints)).

- Marks reach their plot through a context key and add themselves to it: their extent makes the domain of the axis their values go along when the scale gives none, and each `Line` and `Area` has a hover dot. Marks that find rows themselves (a slice, a dot, a cell, a node) add a hit test, and with one the plot follows what it finds rather than the row nearest along the axis.
- A band scale gives each row or category a band. Bars that share a `Stack` pile up from its first bar's base; the others stand side by side in a band.
- `ColorScale` colors categories (`OrdinalColors`, d3's implicit domain) and numbers along a ramp mixed in OKLab (`SequentialColors`, `DivergingColors`), as a mark's paint where nothing sets one.
- `Y2Scale` is a second y scale. While both y domains are automatic, they widen to the same number of nice steps, so one `Grid` serves both axes. A y `Axis`'s outside labels keep the scale's range inside the plot through insets; a label not laid out yet asks for the tallest one's room, as the steps depend on that room and the labels on the steps, which otherwise never settles (a departure from NSH).
- The hover never records: the pointer moves animated values and lands in the same frame, a step of the keys or a screen reader glides on `Motion.Pointer`, the parts fade on `Enter` and `Exit`, and the tooltip is clamped inside the plot by animated math from the plot's size and its own, as NSH's.
- The keys bind while the plot has the focus, Up and Down only over a grid and Escape only while a row is hovered, so they take nothing from the app around it.
- The appear's time runs linearly and each mark eases its own slice: lines clip to an edge `Bind` animates and record once more without the clip when it ends, bars and cells grow, a pie sweeps. `AppearScope` remembers the generations that finished, so a list's row made again shows whole.
- `Motion.Update` moves a linear domain (`tween`): places on the new scale show at `scale*at+offset`, two nodes the domain's ends drive, which paths record through (`Drawing.Transform` bound, undone before the stroke) and bound views derive from, so no mark records per frame. Marks with a pose of their own (`moves`) draw each number as a line from the old pose to the new one; a change while one runs reads how far it had gone and moves on from there. A slice's corners are not linear in its angles, so its numbers are sampled along the motion.
- A moving rectangle with corners records its own arcs, each radius fitted to the rectangle by animated math, as no op takes a rounded rectangle's arguments by slot.
- A `Sankey` lays out d3-sankey's algorithm with gpui-base's changes, a `Treemap` d3's squarified tiling over `stratify`.
- `Area`'s, `Bar`'s, `RadialLine`'s and `Sankey`'s background shows inside the shape through `FillBackgroundFromView`.

| NSH | NSGo | Why |
| --- | --- | --- |
| Rows of objects and accessors by key or function | `Y`, `X`, a mark's `Value` as functions returning `[]float64`, NaN skipping a row, and `Color`, `Label` as `[]string`; `Count` where no column says how many rows | Go reads fields with code, so the app gives the column |
| `PlotAxis`, `PlotLabel`, `PlotAppearScope` | `Axis`, `Label`, `AppearScope` | The package names them; TSX keeps NSH's |
| `Scale` objects with `reverse` | `PointScale()`, `LinearScale(domain...)`, `BandScale(Band)`, `.Reversed()` | Each kind has its own constructor |
| `base` and `y0` as a number or an accessor | `Base` and `Bases`, `Y0` and `Lows` | A field has one type |
| `scale="y2"`, `stack`, `curve` as props | Plain fields of the mark | A mark's axis does not change as it lives |
| `interactive`, `alignTicks` | `NoHover`, `NoAlignTicks` | Go's zero value is the default |
| `inactiveOpacity` | `HoverDim`, as a pie's: what they lose | Go's zero value dims nothing |
| `hoverDotClass` replaces `dotClass`, the styles each their own | `HoverDotStyle` merges over `DotStyle`, and `HoverDotClass` follows `DotClass` | The plot's shape and the mark's color combine |
| `data-dark:` classes on labels | `DarkLabelStyle` and `DarkLabelClass` | A text takes no part states; TSX keeps Go's |
| `curve` natural by default, linear on a radial line | `CurveDefault`, each mark's own | Go's zero value is one constant |
| Classes as a string or a function of the row | `SliceClass` for every slice, `SliceClassOf` and `SliceStyleOf` per row | A field has one type, and the app reads its own rows by index |

## Tables

`Table` and its parts (`TableHeader`, `TableBody`, `TableFooter`, `TableRow`, `TableHead`, `TableCell`, `TableCaption`) give the app's few rows a table's roles, `RoleTable` around `RoleRowGroup`, `RoleRow`, `RoleColumnHeader` and `RoleCell`, which each backend maps to its accessibility: AppKit's and WebKit's table roles, ARIA's, Android's role descriptions, and VoiceOver's containers with headers on iOS.

`DataTable` ports NSH's (`nsh/docs/dev/data-table.md`): the pure parts in `datatable/layout.go` (the scaled axis past 3 000 000 points, the columns' prefix sums, the selection's keys, the row window and slots), the controller in `datatable.go` and the layers in `datatable/view.go`.

- A vertical `Scroll` of a horizontal one of an empty box is the hit layer the platform scrolls; the body over it takes no pointer and places the rows and columns in view from the offsets the scroll events report, in whole points; the header above it is interactive. Desktops get a horizontal scroll bar band over the body's bottom and a vertical one over its right edge, each following the table's scroll view on its axis without echoing its moves back.
- Rows and scrolling columns are slots made once (`slotItem`): a scroll gives slots other rows, so no view is made while scrolling. The rows made are those in view and half a viewport each way, as many more on one side as the table's end cuts from the other (`rowWindow`), so the window keeps its length and moves with every row. The columns' window is NSH's (`window.Rows`, reaching one column and three), and only the cells of the columns in view and one each way read their texts; the others keep theirs, hidden. Rows are placed relative to a chunk of 1024, columns of 256, so places stay small at any offset.
- One effect sets the windows and chunk origins from the scroll offsets into signals: a scroll reaches the region boxes and the rows it brings, not every cell ([reactive graph](core.md#reactive-graph)). The region boxes are sized in points, so the rows moving inside one lay out that box alone ([commit](core.md#commit)).
- The regions also move with the scroll views on the main thread, in the frame the content does: each is translated by how far the scroll views went since the commit that placed it (`tableFollow`: the scroll views' `OffsetValues` through the scaled axis, less a base each commit sets with the places it lays out). The cells then keep up with a scroll however long a commit takes, and at rest the translation is what rounding left, so frames, hit testing and accessibility stay where the cells show. The translation stops at the rows made and the columns shown: a jump past them, a scroll bar dragged across a million rows, waits for the commit that brings their cells rather than showing none.
- A cell's text fills it and aligns with `TextAlign`, so a new text keeps its frame. A cell restyles for `PositiveStyle` and `NegativeStyle` only when its sign changes.
- Cells take presses and context menus from assistive technology and tests; a pointer reaches the hit layer. A title's drag moves its column unless it starts in the resize handle's band or a resize is under way.

| NSH | NSGo | Why |
| --- | --- | --- |
| `data`, `dataStart`, `rowCount` and objects by key | `RowCount` and `Text(row, column)` | Go reads its own data; the table asks for the cells it shows |
| A column's `resizable`, `movable` and `selectable` over the table's | `NoResize`, `NoMove`, `NoSelect` | Go's zero value is the default; a column can only opt out |
| `positiveClass` and `negativeClass` on numbers or signs | `PositiveClass`, `NegativeClass` and their styles on the text's sign or number | The table holds texts |
| Selection and targets as objects with optional fields | `DataTableSelection` and `-1` for none | One comparable value |
| The body moves with each commit, a frame or more after the content | The body also follows the scroll views on the main thread | The cells move with the content on every frame, not when a commit lands |
| Rows made a viewport and a half ahead, the window moving near its edges | Half a viewport each way, moving with every row | Following the content, a commit per row costs nothing to see; rows made ahead were thrown away by a fast scroll before they showed, and on the web held it at 20 fps |

## Resizable panels

`ResizablePanelGroup` ports `gpui-base`'s sizing (`resizeState`, tested as NSH tests it), filling its container unless the app sizes it: a drag grows the panel before its handle by taking from the panels after it, nearest first, as far as their minimums allow, and shrinking below a minimum takes from the panels before.

- Panels are a slice, not children registering through a context as NSH's do: Go lists them statically, and `Visible` hides one. `Axis` is NSH's: `Horizontal` side by side, `Vertical` stacked.
- Each panel's flex basis, grow, shrink and limits come from the size it wants: what a drag gave it, else `Size`. Unsized panels share the rest with a basis of 0; when every panel has a size they grow in proportion. Minimums shrink by one ratio when the group is too short for them.
- A handle is a 3-point band, centered on its 1-point line, between two panels (`handleBandNode`) whose negative margins give its thickness back, so it takes no room yet lies over the boundary inside the group, above its neighbours: macOS gives a view its cursor and tracks the pointer only within what its parents show. NSH's band is 9 points; 3 keep the target on the line, as desktop split views do, instead of taking clicks 4 points into the panels beside it. Touch screens add a 10-point slop each side. `ResizeHandle` is the same band on a container's edge, its line on the edge.
- Drags resize on a state made from the panels' laid out sizes as they start, and only their end, or a screen reader's 10-point step, reports `OnResize`. A press on the band, which no pan took, is a drag that did not move, as NSH's handle reports one from the pointer going down: two in a row fit a `DataTable`'s column, and reach `ResizeHandle.OnDoublePress`, which NSH lacks, for an app to restore a default size.
- Each handle follows the pointer's progression itself (`ResizeHandleState`): hover makes it hovered, a touch going down pressed, its native pan dragging, the pan's or touch's end idle, since a drag leaves the band within a point. `RenderHandle` renders inside the line in a `Dynamic` of that state, so a drag draws it three times, and `ResizeHandle` reports it with `OnStateChange`.

## Dock

`dock.Area` ports NSH's dock in layers: the layout data (`dock/layout.go`: `dock.Node` trees, normalization to a fixpoint, edits, the app's layouts installed with their IDs repaired), `gpui-base`'s persisted format (`dock/state.go`, `DockLayout`'s JSON) and the component (`dock.go`).

- Container IDs come from a counter per area above the largest ID its layout holds, not one per process.
- Each container renders from a memo of its shape (kind and children's IDs), so selecting a tab or resizing rebuilds nothing; a split whose children change is rendered again.
- Each panel's content is a `core.Holder` the area keeps by key while its layout lists the panel, as NSH's render keys: the group showing the panel places it, so a panel moved to another group or dock, or across the compact presentation, keeps its state. A drawer dismissed at compact width leaves its panels built, out of the tree.
- A tab drag claims the touches with the responder (`OnMove`, after 4 points of a pointer or a long press of a finger), measures the groups' tab bars and content frames with `Ref.Bounds`, and resolves the target as NSH's: a slot between tabs, a content frame's middle to merge, its outer 35% to split. The indicator and the drag preview are overlays that never take the pointer.
- Zoom and the compact presentation narrow the tree to one group by hiding the containers off its path, so the panels keep their content.
- Below 600 points the docks are `sheetMotion` drawers and a bottom sheet, presented one at a time, without changing the layout's `Closed`.

| NSH | NSGo | Why |
| --- | --- | --- |
| `DockAreaApi` through a ref | `DockAreaRef`, bound as the area mounts | Go has no refs to components |
| `movePanel`'s `index` and `activate`, `null` for no group | `DockTarget.Before`, the tab it goes before, and `NoActivate`; 0 for no group | Go's zero value is the default: last, displayed |
| Nodes typed `split` or `tabs`, a region's `open` and `collapsible`, `panels` a list of options by `id` | One `dock.Node` struct with `Tabs`, `DockRegion.Closed` and `NoCollapse`, `Panels` a map of `NoClose`, `NoZoom` and `Hidden` by key; `nsgo/base` gives NSH's shapes | Go has no unions, and its zero value allows everything, as a panel NSH's `panels` leaves out does |
| `groupOf` and `zoomedGroup` give `null` for none | `GroupOf` and `ZoomedGroup` return an ID and whether there is one | Go's comma-ok result |
