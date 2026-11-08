# Testing

`nsgotest` runs an app in a Go test, headless by default and on a platform under `nsgo test -target`. It follows NSH's `nsh-testing` (`nsh/docs/dev/tooling.md`, "Headless runs" and "On a real platform"). Read this before changing `nsgotest`, `internal/testhost` or a backend's `testhost.go`.

## The model

Every batch goes to a model of the native views (`nsgotest/model.go`), which queries read and steps act on: frames, styles, props, texts with their spans, shadows and line limits, inputs, scroll offsets, the animated values gestures and scrolling drive (`Bound`, `AnimatedValue`), canvas nodes as child views, and what the app asked the platform for. Headless, the app runs on `core.ApproxTextEngine`, or the engine `TextEngine` gives, and a `core.ManualClock`; frames run only as the clock moves.

Selectors (`nsgotest/selector.go`) name what a user sees, as Playwright's: `id:`, `label:`, `text:` (a substring, or a field's text or its placeholder while empty; in quotes, `text:"Save"`, the whole text or a span's), `role:<role> <name>`, `view:<node>`, and a bare string as a test ID, else a label, else a whole text. `label:`, `text:` and a role's name take `/regexp/flags`.

A view shows when it has an area that nothing hides or clips: `Style.Clip`, a scroll view the user cannot scroll and the window clip; a scrollable view shows all its content after its start; modals show over the window, and header views and the keyboard's bar where the platform puts them. Matches come by layer, the topmost modal's first. `QueryMounted` and `FindMounted` match among every view mounted instead, shown or not, in tree order, for a test reading what the platform received of one.

| NSH | NSGo | Why |
| --- | --- | --- |
| `text:` matches a whole text | In Go tests `text:` matches a substring, and `text:"…"`, a bare string and `/^…$/` match whole; TSX tests have NSH's | Playwright's `text=` matches substrings, and NSGo's Go tests already did |
| `role:button[Save]` | `role:button Save` in Go tests, a substring of the name unless quoted; TSX tests have NSH's | NSGo's form before the port; the name may hold spaces either way |
| Spans of a text are views of the model | `SpanStyle` and presses read a text's spans by their characters | Go tests name a text and its characters; no view per span is needed |

## Steps

Steps (`steps.go`, `pointer.go`) act as a user does and send what a platform sends, to the views that report it. A step whose selector matches no view, more than one it would act on, or a disabled one, fails the test. Before a step, alerts the last one left unanswered get their default answer: the cancel button, else dismissed when cancelable, else the step fails; `App.OnAlert` answers them instead.

- `Press`: a touch down and up when a view listens to touches, presses outside the overlays it is not in, then PressDown, PressUp and Press, on the match's middle (a text in a button is pressed on the text, also on targets); a canvas node presses the view under its middle, and a text's span takes `Index`. Presses of one view within 500 ms of clock time count up in `Clicks`, and a press takes the keyboard focus off pressable views.
- `LongPress`: PressDown, the view's `DelayLongPress` (500 ms) of clock time, LongPress and PressUp, a finger resting at least 500 ms, as a person's does. On a platform it rests 200 ms longer, past what a scroll view holds its content's touches back, and the test's clock goes meanwhile for holds timed on the engine, as a context menu's.
- `Type`, `Clear`, `Submit`, `KeyPress`: the field takes the focus, then a key and an edit per character within `MaxLength`; a line break in a single line is the return key, as `SubmitBehavior` resolves it.
- `Compose` and `Commit`: an input method composes in the field, which holds the values the app sets meanwhile, and commits, a `Type` and the composition's end. On a platform they go through its text input client as an input method's do: AppKit's and UIKit's marked text, Chrome's `Input.imeSetComposition`, GTK's preedit signal, IMM's composition string, which needs an IME on Windows and skips the test without one, and Android's `InputConnection`.
- `Drag`: a finger lands, rests 100 ms, moves 10 points a frame through its path and lifts; the press under it is cancelled once it moves 8 points. `Touch` puts finger 1 down for a test to `Move`, `Lift`, `LiftAt` or `Cancel` a step at a time, a frame apart.
- Past the slop the innermost pan a drag activates takes it, else the scroll view it runs along that can move that way, unless the touch responder blocked native drags.
- `Scroll`: drags a scroll view by a distance, 10 points a frame. `ScrollTo`, `DragScroll`, `Pan`, `Pinch` and `Rotate` report a platform's own gestures.
- `Hover` and `HoverOut`: the views from the match out with hover events replace those hovered before.
- `Report` sends an event as the platform would, for one no step makes. `NoCaretMoves` keeps typing from reporting the caret, as NSH's headless model types.
- `Key`: a keymap binding, `mod` being Command in Apple environments and Control elsewhere; else, in a macOS or web environment, Tab and Shift+Tab move the keyboard focus between editable fields and pressable views not `NoFocus`, in tree order, and Return or Space press the pressable it is on; else the focused field takes a character (shifted as the US layout does), Backspace or Return; else Escape goes back. A key nothing takes does nothing, as on a device.

Each step then takes frames while an animation that ends runs or a list binds the rows it paces ([lists](scrolling.md#lists)), for at most 2 seconds of clock time. Frames the app requests itself do not hold a step, since its own loops may never end; `ManualFrames` keeps steps from taking frames at all, for tests that look at animations on the way. `Advance` moves the clock a frame at a time; `FastForward` jumps it, running each due timer once.

## On a platform

`nsgo test -target <t>` builds the package's tests with the `nsgotest` tag as the target's package and runs each test in a process of its own (`cmd/nsgo/testtarget.go`). A package whose tests link no backend runs them headless in the target's process, giving the page its turn as each settles on the web, where the promise reactions of a TSX app wait for it. Each `Start` of a test opens another window on a desktop; a phone and a page show one app, so a test's second `Start` skips there. The test's `TestMain` calls `nsgotest.Main`, which gives the main thread to the backend's test host (`internal/testhost`) and runs the tests beside it.

`nsgotest.Start` then has the host open the app as its `Run` would, with the engine's `Observe`, `Event` and `Pin` options, the test's clock and a clipboard of its own: the model follows the batches the real engine sends and the scroll, focus and text events the platform reports, and the test's environment pins the appearance, locale, text size and reduced motion.

Steps resolve on the model, scroll the view into view a gesture at a time, and act through the host's native input; a step ends once no batch came and nothing animated for 100 ms, 5 seconds at most, and `Start` first waits for the app's first batch, which comes late on Windows. Storage is a memory store in a temporary data directory, which the test reads through a graph of its own. Screenshots go to the tool as base64 lines in chunks, which it saves under `.nsgo/test/screenshots/<target>`.

| Target | Input | Screenshots | Quiet |
| --- | --- | --- | --- |
| macOS | Mouse and key `NSEvent`s posted to NSApp's queue, so monitors see them; wheels as trackpad phases to the view hit; hover through tracking-area handlers; Command edit keys as the Edit menu's actions | `cacheDisplayInRect:` | Animator, layout motions, ghosts |
| iOS simulator | IOHID digitizer events to `_enqueueHIDEvent:`; typing through `UIKeyboardImpl`; keys as HID keyboard events | `drawViewHierarchyInRect:` | Finite Core Animation, gliding scroll views |
| Web | Trusted CDP input from the tool, asked over the `nsgoTest` binding; macOS editing commands | `Page.captureScreenshot` | Animator, finite DOM animations |
| Windows | Posted mouse and wheel messages; keys at `editKey` and `shortcut` with the thread's keyboard state | `PrintWindow` | Transitions, moves, fades |
| Linux | The backend's pointer, wheel and key callbacks; the focused field's own signals | Cairo offscreen | Transitions, layout animations |
| Android | `TestAgent.java`: `MotionEvent`s and `KeyEvent`s on the UI thread, asked over the pipe | The window drawn to a bitmap | A signature of every view's place, scroll and opacity |

Alerts are answered at their buttons: an `NSAlert`'s, a `UIAlertController`'s label, the web's DOM button, Windows' drawn button, GTK's button, the `AlertDialog`'s. `ChooseMenu` and `ChoosePopup` choose the native menus' items: the main menu's on macOS, the menu bar's on Windows and Linux, and the context menu showing; phones and the web, which show no native menus, skip the test. `Advance` moves the test's clock a frame at a time, and after each step waits for the engine to run what the step's timers posted (`Surface.Sync`), so the timers they set, an interval's next run, fall due within the same advance; the app's frames follow the clock. What the test reports for a platform (`Load`, `Navigate`, `ImageSizes` and the like) and `ManualFrames` skip the test on a platform; `nsgotest.Target` names it, for tests that check only the headless model.

The test agent's Java ships in every APK, but the runtime takes a test from the launch intent only in a debuggable build, which `nsgo test` makes, so no other app sets the process's arguments or environment.

## TSX tests

A TSX app's `*.test.tsx` files import `nsgo/test` (`tsx/tsxtest`, NSH's `nsh/test`): `test`, `describe`, their `skip` and `only`, the hooks and `expect`, in a goja VM of their own with `atob` and `btoa`, and `open(entry, options)`, which starts the app with `nsgotest` and gives an `App` acting as a user does: `press`, `type`, `scroll`, `drag`, `chooseMenuItem`, `waitFor`, `shows`, `texts`, `spanStyle`, the keyboard and the clock, and the app's services as `nsgotest` records and answers them: `storage`, `dialogs` and `onDialog`, `picks` and `onPick`, `menu`, `openedUrls`, `clipboard`, `requests`, and `speech`, `hearSpeech` and `endSpeech`. Its options give storage, a clipboard, a size, a safe area, an appearance, a locale, reduced motion, a platform, speech recognition, a clock, a host and a network. `nsgo test` adds a Go test calling `tsxtest.Run` to the package, so they run with its Go tests, headless or on a platform; `tsxtest.RunSteps` runs a test body of its own on an entry, which `nsgo test <entry>` reads from standard input. `nsgotest.Errors` takes what the app reports failing instead of failing the test, as a host's overlay shows it while the app runs on.

`text:` and a role's name (`role:button[Save]`) are whole texts, and a span's; `waitFor` holds once any view matches; keys are named as keymaps name them (`"escape"`, `"mod+k"`). A step returns a promise that resolves once the app is quiet and rejects with why it could not be taken (a disabled view: `is disabled`), or with what the app threw meanwhile, which `expect(() => step).rejects.toThrow()` reads: nsgotest's failures panic in the app's `testing.TB` (`appTB`) and the step recovers them. A failure the app reports outside a step fails the test at its end.

Without `clock`, the app's time passes as the test waits, a frame at a time, as it does on NSH's system clock; `clock: {}` keeps it still but for `wait` and `clock.runFor`, and `clock.time` and `clock.setSystemTime` set what `Date` reads (`ManualClock.SetWallTime`), moving no timer. `performance.now()` and frames' times count in binary fractions of a millisecond, so a second of 16.667 ms frames is exactly 1000.

The network answers by routes keyed `"<METHOD> <URL>"`, a `*` matching any text, through `tsx.WithFetcher`, and fails a request none lists; `"live"` is the real one. A route is a response or a function of the request: a request arrives on a goroutine of its own, so it posts the call to the test's goroutine (`nsgotest.App.Post`), which runs it as a step or wait settles, and polls a promise it returns at each settle after.

`host` plays the embedding app: its params, and its handlers, a value or a function whose result, or the promise of it, answers; `app.host.send` sends the app an event, and `calls` and `posts` list what the app sent the names it handles. Without `host`, a headless run runs the page's host module nsgo.yaml's `web.host` names, as the page would: in a VM of its own with `console` and timers on the app's clock, its `createHost(player)` answering the app's calls and `player.hostEvent` sending it events, those sent before it starts reaching it as it does.
