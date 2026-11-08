# Interaction

Accessibility, pointer events, keyboard focus, touch areas, touches and the responder, gestures and file drops: what the core sends so platforms decide on the main thread. Read this before changing accessibility props, pointer events, focus, touches, gestures or drops.

## Accessibility

Elements carry NSH's accessibility props: `Role` (a menu item's checked kinds, `RoleListbox`, `RoleOption`, `RoleGroup`, `RoleRegion`, `RoleNavigation`, `RoleTree`, `RoleTreeItem` and `RoleToolbar` among them, each on a backend's nearest role, as WebKit maps ARIA's; Android reads those as plain views for now, [roadmap](../roadmap.md#finish-the-existing-platforms)), `Pressed` and `Busy`, `Label` (`LabelFunc` when it changes), `Hint`, `AccessibilityValue` (a range when `Max` is above `Min`, and `Text`), `Expanded`, `AccessibilityActions` with `OnAccessibilityAction`, `Accessible`, `ImportantForAccessibility`, `AccessibilityElementsHidden` and `LiveRegion`. `Ref.SetAccessibilityFocus` moves the screen reader, as an `ItemAccessibilityFocus` command.

- The props go in `ViewProps`, except a label changing alone on mounted props, which goes as `ItemAccessibilityLabel`.
- An `Accessible` view is one element. Without a label of its own, the core labels it with the text inside it, so every platform reads the same words, and the label follows the text.
- Standard actions (`activate`, `increment`, `decrement`, `longpress`, `expand`, `collapse`, `escape`, `magicTap`) take the screen reader's own gestures; others are listed as custom actions. A disabled view's actions are dropped.
- macOS sets NSAccessibility attributes: AXIncrement, AXDecrement and AXCancel are the standard actions, the others `NSAccessibilityCustomAction`s. iOS sets UIAccessibility properties: the adjustable trait's swipes, `accessibilityPerformEscape` and `accessibilityPerformMagicTap`, and `UIAccessibilityCustomAction`s; an escape no element takes goes back, closing the newest popup or screen. Both read an unlabelled element's texts, and announce live regions themselves when a label or a text inside one changes.
- The web maps the props to ARIA. A group hides its children with `aria-hidden`, and a focused view's arrow keys, Escape, Enter and Space perform its standard actions; ARIA has no custom actions, so those are not offered.
- Android sets `importantForAccessibility`, live regions and screen-reader focus on the view, and range info, hint, state description and actions in its `AccessibilityNodeInfo`. It has no escape or magic tap.
- Windows answers `WM_GETOBJECT` with a UI Automation provider per element, made on demand: a control type, the hint as help text and the Invoke, Toggle, ExpandCollapse, SelectionItem, RangeValue and Value patterns the role and state call for. Views that are no elements are flattened, and controls group the texts inside them as on iOS and Android. Live regions raise `LiveRegionChanged`, and announcements are notifications.
- Linux gives each view's widget a role from its construction (GTK keeps it), its label, hint, value and states through `GtkAccessible`, which AT-SPI exposes, and ranges through `GtkAccessibleRange`. Live regions and announcements go through `gtk_accessible_announce`; GTK has no custom actions.
- A canvas's nodes are elements of their own, children of the canvas ([canvas](canvas.md#nodes)).

## Pointer events

Presses, the mouse and secondary clicks carry where they happened, as NSH's `PressEvent`, `MouseEvent` and `ContextMenuEvent` do: `Position` in the view, `PagePosition` in its root view (the surface, or the modal around it) and the `Modifiers` held (`Platform` is Command on Apple platforms, the Windows or Super key elsewhere). A `PressEvent` also counts its `Clicks`.

- `OnPress`, `OnLongPress`, `OnPressDown` and `OnPressUp` take a `PressEvent` with where the press went down and the keys held then. A press from a screen reader or the keyboard is at the view's origin, with no keys and one click.
- A view that follows presses only for its `Active` style, or as a group, takes none inside a view that takes presses: a press on a button's face is the button's. A view with its own press handlers still takes them.
- `OnMouseEnter`, `OnMouseMove` and `OnMouseLeave` are the DOM's events and take a `MouseEvent`, which adds the `PressedButton` a drag holds; any of the three makes the view follow the mouse, and `OnMouseLeave` says where it was last. `OnMouseMove` hears every move over the view, with or without a button down; what only follows a resting mouse checks `PressedButton`, as the editor's hovers do. Each listening view under the pointer hears a move, innermost first, and the engine merges a view's queued moves. A modifier key pressed or released under a still mouse is a move with the new modifiers: macOS hears it in its event monitor's `flagsChanged`, Windows in the modifiers' `WM_KEYDOWN` and `WM_KEYUP`, Linux in the key controller's `modifiers` signal and the web in `keydown` and `keyup` of the modifier keys.
- macOS reads the `NSEvent`: `clickCount`, `modifierFlags`, and moves from a tracking area with `NSTrackingMouseMoved`. A tracking area holds the mouse even under an overlay, so a view is hovered only while the surface's hit test at the pointer lands in it, checked on each move and each batch. A drag goes only to the view it began in, so the surface reports drag moves to the views under the pointer itself.
- Windows counts clicks against the system's double-click time and distance, and reads the keys with `GetKeyState`; Linux takes GTK's press count and the controller's modifier state. Both route moves through the views they hit-test.
- The web counts clicks itself, since pointer events carry none: 500 ms and 4 points apart at most. Moves are mouse `pointermove`s.
- iOS reads the touch's `tapCount` and the event's `modifierFlags`, and hovers with a `UIHoverGestureRecognizer` (a pointer or a hovering Pencil). Android reports a mouse's moves from `setOnHoverListener` and its coming and going with no position, and no button with either ([roadmap](../roadmap.md#finish-the-existing-platforms)). It reads `MotionEvent`'s meta state, counts every press as one click, and sends the page position and keys after the rest of an event.

## Keyboard focus

On macOS and the web, views that take presses are Tab stops beside the text inputs, unless disabled, hidden, `NoFocus` (a backdrop that closes a popover) or under a newer modal. `View.Focusable`, when set, decides over both, following what it reads: it makes a stop of a view that takes no presses, whose keys come through the keymap, and a group's roving focus turns stops on and off with it; Return and Space press the focused one once, whatever the keymap binds, and a click presses without focusing. `Style.Focus` follows the keyboard focus, and `View.OnFocus` and `OnBlur` hear it come and go. The other backends move the focus between fields only.

- macOS puts the views in AppKit's key view loop (`acceptsFirstResponder`, `canBecomeKeyView`, the window recalculating it); the surface, which holds selections, hands Tab to the window's first key view. A modal remembers what had the focus as it came and gives it back as it goes, if the focus fell to the window; a field comes back with the caret at the end.
- The web makes them `tabindex=0`, reports focus while `:focus-visible`, prevents a pressable's `mousedown` from moving the focus, and makes what a modal covers `inert` until it goes, then gives the focus back.

The core keeps one focus per surface for `focus-within:` and `FocusWithin`: inputs and views that take presses report their focus whether or not they style it, and `focusReported` takes the view that gained it as the focus, whatever order a move arrives in; a blur of another view changes nothing, and the root is no focus. The next commit puts `stateFocusWithin` on the focus and the views around it and takes it off those it left (`updateFocusWithin`), walking the two chains and nothing else. The views around are parents in the core's tree, where an overlay is the child of the view the app put it in, so a popup's focus is within the view holding its trigger, as in a React portal.

## Touch areas

`View.PointerEvents` (`Auto`, `None`, `BoxNone`: only the children, `BoxOnly`: only the view), `HitSlop` and `DelayLongPress` go in `ViewProps`. Children placed outside their parent take touches where they are drawn: for each view whose children reach outside it, the core sends `ItemOverflow` with the area they cover, from layout frames and only when it changes. Clipping and scroll views have none, and transforms are not covered.

- `driver.Reach` decides for a point what it reaches of a view: the view itself by its bounds and slop, its children by its bounds and area. The core's own hit test, Windows' and Linux's walkers ask it on the way down.
- macOS and iOS override `hitTest:`: outside the bounds they search the subviews of a view with an area and never return the view itself, except in its slop; a press ends as a press in the slop too. The raw hit test macOS asks for the window drag, drops, selections and scroll bars returns the deepest view rather than its pressable ancestor, by the same pointer events, so a scroll bar under a layer that takes no pointer still takes the mouse.
- Android's `Group` hands a touch outside every child's bounds to the topmost child whose slop or area holds it, for the rest of the gesture; `None` declines the touch, `BoxNone` leaves it to the children and `BoxOnly` intercepts theirs.
- The web needs no area, since the browser finds children outside their parent itself. `pointer-events` classes carry `PointerEvents`, and a slop is a pseudo-element around the view.
- A slop works where the parent's bounds or area hold it, as in UIKit and React Native.
- Long presses wait 500 ms, or the view's `DelayLongPress`: a timer on macOS, iOS, the web and Linux, which no longer uses GTK's long press gesture, and on Android when the delay is the view's own, the system's long click then only taking the press. Windows reports press down, up and long presses from the left button itself and leaves the press to the engine's hit test at the release.

## Touches and the responder

`View.OnTouch` hears the fingers and `View.Responder` takes part in React Native's touch responder, as NSH's core runs them for Rust components (`core/touch.go`). Platforms send `EventTouch` with each finger's surface position while a view's mask has it; the core finds the view under a finger from its own frames and the listening views around it, which hear the touches from the finger outwards.

- The responder is negotiated as in React Native: captures from the outermost in, then from the finger outwards; with a responder, only the views around it are asked, and `Keep` refuses to hand the touches over, or `OnTerminationRequest`, asked each time another view claims them. A claim's `Ask` decides with the `PanGesture`; the responder stays pressed for its `Active` style until the touches end.
- `OnTouchCapture` hears the touches from the outermost view in before `OnTouch` hears them from the finger out, as the DOM's events capture and bubble; `TouchEvent.StopPropagation` ends the round.
- A responder whose claim does not `LetNative`, or whose `BlockNative` says so at the grant (TSX's `onResponderGrant` returning true, as React Native's), sends `ItemBlockNative`: iOS disables the pans of the scroll views around it, Android disallows their interception and the web cancels `touchmove`. The desktops' mouse drags scroll nothing, so they ignore it.
- A scroll view that starts dragging first cancels the touches: iOS from `scrollViewWillBeginDragging:`, Android when a scroll view takes the touch, the web through `pointercancel`.
- The desktops report the primary mouse button as finger 0 (`internal/platform/finger`); iOS tracks every touch with a gesture recognizer that never recognizes, Android in its root view's `dispatchTouchEvent` and the web with capturing pointer listeners.
- A `TouchClaim` with `OnPan` claims as the platform activates the view's `PanActivation`, so scroll views share the touch until then: the view becomes the responder exactly when its pan begins (`EventPan`'s first report), and its other claims are not asked.

## Cursors

`Style.Cursor` sets the pointer's look over a view. `Auto` leaves it to the nearest ancestor that names one, and the web shows a pointer over pressable views. The diagonal resizes are `CursorNwseResize` and `CursorNeswResize`, which CSS's corner names read as; macOS draws them with AppKit's frame cursors from version 15 and the open hand before it. macOS sets the cursor of the view under the pointer by the same hit test, inputs' I-beam too, since AppKit's cursor rects and text views' tracking areas would show through overlays: NSGo's fields and text views give them up, and a secure field's AppKit-made editor takes a subclass that does as it comes; Windows answers `WM_SETCURSOR` with that view, Linux sets GTK's named cursor, and the web sets CSS `cursor`. Touch platforms ignore it; resizable handles ask for the column or row resize cursor. `ui` names a cursor only for text fields and resize or scrub drags, so its pressables keep the platform's arrow. macOS gives a view its tracking areas only within what its parents show, so a handle's band lies inside its group.

## Gestures

`View.OnPan`, `OnPinch` and `OnRotation` follow gestures the platform recognizes; they report to the engine, which merges each gesture's moves per turn and never its first or last report.

- The platform decides on the main thread when a drag becomes a pan, from the view's `PanActivation`. Until it does, a scroll view around may still take the touch.
- An active pan takes the touch from presses and scroll views.
- A drag or wheel turn scrolls the innermost scroll view under it that can move its content that way, so an inner one passes on only at its edge. A touch or trackpad gesture keeps the scroll view it began with. Android still uses its scroll views' own handling.
- Desktop scroll bars show while their scroll view moves and while the pointer is over it, and can be dragged: the view's own indicator on macOS, `internal/platform/scrollbar`'s on Windows and Linux (Windows fades it as macOS does), and the browser's on the web.
- `View.PanValues` binds a drag to animated values: the platform moves them before it reports each move, so the view follows on the main thread, as NSH's bound values do. A drag through `OnPan` and a signal costs a commit per turn.
- A pinch reports its `Position` in the view: the fingers' middle, or the pointer over a trackpad, so a canvas zooms around it.

`View.OnWheel`, which NSH lacks, takes the wheel and touchpad scrolls over a view before the scroll views around it, for what pans and zooms itself, such as a flow diagram; a scroll view inside the view takes its own first. `WheelEvent.Delta` is how far the content moves, a notch about 100 points as browsers scroll, and `Touchpad` tells a touch surface's scroll from a wheel's turns. The engine adds queued turns of one view up rather than replacing them.

| Target | Takes | Touchpad | Glide after the fingers |
| --- | --- | --- | --- |
| macOS | `scrollWheel:` before the surface routes it to scroll views; a gesture and its momentum keep their view | A phase, which only touch surfaces report | The system's momentum, as more changes and an end |
| iOS | Two scroll-only `UIPanGestureRecognizer`s: continuous for a trackpad, discrete for a wheel | The continuous one | The view's own, from the end's `Velocity` |
| Web | The `wheel` event before the page's scroll handling, prevented | Pixels smaller than a notch | The browser's momentum, as more changes |
| Windows | `WM_MOUSEWHEEL` and `WM_MOUSEHWHEEL` before the scroll views | Turns of less than a notch | The system's, as more turns |
| Linux | The scroll controller before the scroll views; a touchpad keeps its view until `decelerate` | GTK's surface unit, or deltas of points before 4.8 | The view's own, from the end's `Velocity` |
| Android | `ACTION_SCROLL` generic motions on the view | `SOURCE_TOUCHPAD` | None yet |

## File drops

Files dragged from other apps follow NSH's `file-drop.md`. `View.OnFileDrop` receives `file:` URLs; `OnFileDragEnter` and `OnFileDragLeave` follow the drag.

- The destination is the innermost view under the pointer whose mask has `EventFileDrop`, unless it is disabled; the topmost modal bounds the search. The platform decides on the main thread from the mounted views and answers Copy or nothing.
- Leaving one destination for another reports a leave, then an enter. A deleted destination hears nothing more, since its ID may be reused.
- macOS takes `NSPasteboardTypeFileURL` on the surface, Windows registers an OLE `IDropTarget` written in Go while a view listens, and Linux adds a `GtkDropTarget` for `GdkFileList`. Browsers hide paths, so the web delivers each file as a `blob:` URL that `fetch` and `net/http` read, its name as the fragment.
