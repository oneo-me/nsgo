# Testing

`nsgotest` runs an app in a Go test, headless by default and on a platform under `nsgo test -target`. It follows NSH's `nsh-testing` (`nsh/docs/dev/tooling.md`, "Headless runs" and "On a real platform"). Read this before changing `nsgotest`, `internal/testhost` or a backend's `testhost.go`.

## The model

Every batch goes to a model of the native views (`nsgotest/model.go`), which queries read and steps act on: frames, styles, props, texts with their spans, inputs, scroll offsets, canvas nodes as child views, and what the app asked the platform for. Headless, the app runs on `core.ApproxTextEngine` and a `core.ManualClock`; frames run only as the clock moves.

Selectors (`nsgotest/selector.go`) name what a user sees, as Playwright's: `id:`, `label:`, `text:` (a substring, or a field's text or its placeholder while empty), `role:<role> <name>`, `view:<node>`, and a bare string as a test ID, else a label, else a whole text. `label:`, `text:` and a role's name take `/regexp/flags`.

A view shows when it has an area that nothing hides or clips: `Style.Clip`, a scroll view the user cannot scroll and the window clip; a scrollable view shows all its content after its start; modals show over the window, and header views and the keyboard's bar where the platform puts them. Matches come by layer, the topmost modal's first.

| NSH | NSGo | Why |
| --- | --- | --- |
| `text:` matches a whole text | `text:` matches a substring; a bare string and `/^…$/` match whole | Playwright's `text=` matches substrings, and NSGo's tests already did |
| `role:button[Save]` | `role:button Save` | NSGo's form before the port; the name may hold spaces either way |
| Spans of a text are views of the model | `SpanStyle` and presses read a text's spans by their characters | Go tests name a text and its characters; no view per span is needed |

## Steps

Steps (`steps.go`, `pointer.go`) act as a user does and send what a platform sends, to the views that report it. A step whose selector matches no view, more than one it would act on, or a disabled one, fails the test. Before a step, alerts the last one left unanswered get their default answer: the cancel button, else dismissed when cancelable, else the step fails; `App.OnAlert` answers them instead.

- `Press`: a touch down and up when a view listens to touches, presses outside the overlays it is not in, then PressDown, PressUp and Press, on the match's middle (a text in a button is pressed on the text, also on targets); a canvas node presses the view under its middle, and a text's span takes `Index`. Presses of one view within 500 ms of clock time count up in `Clicks`, and a press takes the keyboard focus off pressable views.
- `LongPress`: PressDown, the view's `DelayLongPress` (500 ms) of clock time, LongPress and PressUp.
- `Type`, `Clear`, `Submit`, `KeyPress`: the field takes the focus, then a key and an edit per character within `MaxLength`; a line break in a single line is the return key, as `SubmitBehavior` resolves it.
- `Drag`: a finger lands, rests 100 ms, moves 10 points a frame through its path and lifts; the press under it is cancelled once it moves 8 points.
- Past the slop the innermost pan a drag activates takes it, else the scroll view it runs along that can move that way, unless the touch responder blocked native drags.
- `Scroll`: drags a scroll view by a distance, 10 points a frame. `ScrollTo`, `DragScroll`, `Pan`, `Pinch` and `Rotate` report a platform's own gestures.
- `Hover` and `HoverOut`: the views from the match out with hover events replace those hovered before.
- `Key`: a keymap binding; else, in a macOS or web environment, Tab and Shift+Tab move the keyboard focus between editable fields and pressable views not `NoFocus`, in tree order, and Return or Space press the pressable it is on; else the focused field's key, else Escape goes back.

Each step then takes frames while an animation that ends runs or a list binds the rows it paces ([lists](scrolling.md#lists)), for at most 2 seconds of clock time. Frames the app requests itself do not hold a step, since its own loops may never end; `ManualFrames` keeps steps from taking frames at all, for tests that look at animations on the way. `Advance` moves the clock a frame at a time; `FastForward` jumps it, running each due timer once.

## On a platform

`nsgo test -target <t>` builds the package's tests with the `nsgotest` tag as the target's package and runs each test in a process of its own (`cmd/nsgo/testtarget.go`). Each `Start` of a test opens another window on a desktop; a phone and a page show one app, so a test's second `Start` skips there. The test's `TestMain` calls `nsgotest.Main`, which gives the main thread to the backend's test host (`internal/testhost`) and runs the tests beside it.

`nsgotest.Start` then has the host open the app as its `Run` would, with the engine's `Observe`, `Event` and `Pin` options, the test's clock and a clipboard of its own: the model follows the batches the real engine sends and the scroll, focus and text events the platform reports, and the test's environment pins the appearance, locale, text size and reduced motion.

Steps resolve on the model, scroll the view into view a gesture at a time, and act through the host's native input; a step ends once no batch came and nothing animated for 100 ms, 5 seconds at most. Storage is a memory store in a temporary data directory, which the test reads through a graph of its own. Screenshots go to the tool as base64 lines in chunks, which it saves under `.nsgo/test/screenshots/<target>`.

| Target | Input | Screenshots | Quiet |
| --- | --- | --- | --- |
| macOS | Mouse and key `NSEvent`s posted to NSApp's queue, so monitors see them; wheels as trackpad phases to the view hit; hover through tracking-area handlers; Command edit keys as the Edit menu's actions | `cacheDisplayInRect:` | Animator, layout motions, ghosts |
| iOS simulator | IOHID digitizer events to `_enqueueHIDEvent:`; typing through `UIKeyboardImpl`; keys as HID keyboard events | `drawViewHierarchyInRect:` | Finite Core Animation, gliding scroll views |
| Web | Trusted CDP input from the tool, asked over the `nsgoTest` binding; macOS editing commands | `Page.captureScreenshot` | Animator, finite DOM animations |
| Windows | Posted mouse and wheel messages; keys at `editKey` and `shortcut` with the thread's keyboard state | `PrintWindow` | Transitions, moves, fades |
| Linux | The backend's pointer, wheel and key callbacks; the focused field's own signals | Cairo offscreen | Transitions, layout animations |
| Android | `TestAgent.java`: `MotionEvent`s and `KeyEvent`s on the UI thread, asked over the pipe | The window drawn to a bitmap | A signature of every view's place, scroll and opacity |

Alerts are answered at their buttons: an `NSAlert`'s, a `UIAlertController`'s label, the web's DOM button, Windows' drawn button, GTK's button, the `AlertDialog`'s. `ChooseMenu` and `ChoosePopup` choose the native menus' items: the main menu's on macOS, the menu bar's on Windows and Linux, and the context menu showing; phones and the web, which show no native menus, skip the test. `Advance` moves the test's clock without waiting, and the app's timers and frames follow it. What the test reports for a platform (`Load`, `Navigate`, `ImageSizes` and the like) and `ManualFrames` skip the test on a platform; `nsgotest.Target` names it, for tests that check only the headless model.

The test agent's Java ships in every APK, but the runtime takes a test from the launch intent only in a debuggable build, which `nsgo test` makes, so no other app sets the process's arguments or environment.

## TSX tests

A TSX app's `*.test.tsx` files import `nsgo/test` (`tsx/tsxtest`, NSH's `nsh/test`): `test`, `describe`, their `skip` and `only`, the hooks and `expect`, in a goja VM of their own, and `open(entry, options)`, which starts the app with `nsgotest` and gives an `App` acting as a user does: `press`, `type`, `scroll`, `drag`, `chooseMenuItem`, `waitFor`, `shows`, `texts`, `spanStyle`, the keyboard and the clock, and the app's services as `nsgotest` records and answers them: `storage`, `dialogs` and `onDialog`, `picks` and `onPick`, `menu`, `openedUrls`, `clipboard` and `requests`. Its options give storage, a clipboard, a size, a safe area, an appearance, a locale, reduced motion, a platform, a host and a network. The network answers by routes keyed `"<METHOD> <URL>"`, a `*` matching any text, through `tsx.WithFetcher`, and fails a request none lists; `"live"` is the real one. A route is a response or a function of the request: a request arrives on a goroutine of its own, so it posts the call to the test's goroutine (`nsgotest.App.Post`), which runs it as a step or wait settles, and polls a promise it returns at each settle after. Selectors are `nsgotest`'s, NSH's `role:button[Save]` read as `role:button Save`. `nsgo test` adds a Go test calling `tsxtest.Run` to the package, so they run with its Go tests, headless or on a platform.
