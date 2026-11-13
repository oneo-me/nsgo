# Performance

The performance monitor measures what a surface's frames and commits cost while it runs, for a panel over the app (`--nsgo-fps`). It follows NSH's `nsh_engine::perf` and macOS HUD. Read this before changing `internal/engine/perf.go`, a backend's `perf.go`, its frame clock or anything that runs every frame.

## Measuring

`engine.PerfMonitor` is shared by the two threads of a surface:

- The engine goroutine records every wake-up that produced a commit with `RecordCommit`: the app's code and the core's work, and the mount items the commit emitted. `RecordBusy` records how long each wake-up was busy.
- `engine.Options.Perf` installs a monitor; nil measures nothing and costs one check per wake-up.
- The main thread records its work with `RecordWork`: applying batches, stepping animations, handling input; `dirty` says whether the work changes what the screen shows.
- It closes a frame at every display refresh with `Vsync`. A refresh in which something changed is a frame the screen showed, and its cost is the main-thread work since the previous refresh; refreshes that changed nothing are not frames.
- A batch carries `MountBatch.Sent` while a monitor measures, so the platform can report how long it waited between the engine goroutine and the main thread with `RecordLatency`.
- The process' CPU, memory and (on macOS) GPU arrive with `SetProcess` from the platform's sampler.

`Snapshot` turns what was recorded into `PerfSnapshot`: rates over the last second, distributions over the last 120 frames and commits (mean, p50, p95, max), the share of frames over one refresh, the engine's busy share and the process' resources. The first 4 frames and commits are cold and are not counted.

## The panel

Any build shows one when started with `--nsgo-fps` (`flags.go`), and `nsgo dev -fps` starts it so; `nsgo.Run` turns the flag into the backend's monitor, and embedded views and target tests have none. Without the flag nothing is installed, so a build costs the same with the monitor's code as without it; with it, the frame clock runs every refresh, so measure with and read CPU without it. macOS, iOS, Windows and Linux show one; Android and the web accept the option and ignore it.

The panel sits at the top right of the surface (below the safe area on iOS), lets the pointer through, and is redrawn every 500 ms; on macOS it fades out while the pointer is over it. Its sizes, colors and trace are `engine`'s (`perfpanel.go`), and `internal/platform/perfhud` reads the process' CPU and memory for every backend:

- `FPS` counts frames shown over the last second; `max` is what the main thread could keep up with, capped at the display's rate.
- `TICK` is the gaps between refreshes and the 95th-percentile gap between shown frames.
- `MAIN` is main-thread time per frame, its 95th percentile, and `drop`, the share of frames that took longer than one refresh.
- `APP` and `CORE` are the app's and the core's time per commit, commits per second and mount items per commit.
- `HOP` is how long batches waited between the threads, and how busy the engine goroutine was.
- `CPU` is percent of one core, `GPU` this process' share (macOS only), `MEM` its physical footprint (Apple), working set (Windows) or resident set (Linux).
- The trace is the last 120 frames' main-thread cost against one refresh (the line): green within half of it, amber within it, red over.

While a monitor measures, the frame clock runs on every refresh, so an idle app still reports the display's tick rate. Its `FPS` reads low when the app is idle; that is not slowness. Measure release builds: a debug build's text and layout costs are not the app's.

## Adding a backend

A backend calls `RecordWork` around its main-thread work, `Vsync` from its frame clock with the refresh period, `RecordLatency` on the main thread when a batch with `Sent` applies, and `SetProcess` from a `perfhud.Sampler`. The panel draws `PerfSnapshot.Readings()` and `engine.PerfTrace`.

| Target | Frames close at | Main-thread work |
| --- | --- | --- |
| macOS, iOS | The display link, at its period | Batches, ticks and input |
| Windows | Each `DwmFlush`, at DWM's composition rate (`DwmGetCompositionTimingInfo`) | Batches, ticks and drawing a frame, less `Present`, which waits for the display |
| Linux | A tick callback of the surface's frame clock, at its refresh interval | Batches, ticks, and each frame from the clock's `layout` signal to `after-paint` |

Apple platforms read memory with `proc_pid_rusage`'s physical footprint; macOS reads GPU time from the `accumulatedGPUTime` of the process' accelerator clients in the IO registry (`internal/platform/apple/perf.go`), through purego.

## Keeping frames cheap

Every frame runs the commit on the engine goroutine and, on the main thread, the batch, the tick and the drawing of what changed. The quotes and list cases of `benchmark/` measure these paths against other frameworks ([benchmarks](../../benchmark/README.md)); compare their CPU, and the panel's `MAIN` and `CORE`, before and after changing them. The core's own benchmarks (`go test ./core -run '^$' -bench .`) time commits on the approximate text engine, as NSH's `bench_view_tick` and `bench_high_frequency` do:

| Benchmark | Per commit, M1 Max | Items |
| --- | --- | --- |
| `ViewTick`: 300 static rows and a counter | 4 µs (572 µs when commits walked the whole tree) | 1 |
| `Board/fixed`: 961 views, 40 values of fixed cells a tick | 52 µs (766 µs) | 40 |
| `Board/content`: the same in cells of their text's size | 51 µs (889 µs) | 40 |
| `Board/widening`: values that change width every tick | 486 µs (877 µs), nearly all of it flex layout | 280 |
| `ListScroll`: 1000 rows, 10 points a commit | 12 µs (121 µs) | 2.5 |

- A commit visits only what changed, lays out only what changes reach, and skips texts that measure as before ([commit](core.md#commit)). Text sizes are cached per engine ([text](text.md#text-and-drawing)) and lists pace their binds ([lists](scrolling.md#lists)).

- The engine's OS thread has the main thread's quality of service on Apple platforms (`engine.Options.Thread`, `apple.RaiseThread`), as NSH's does: at the default class macOS may run it on efficiency cores. Raised, a 40-row quote table ticking every frame commits about 15% faster, and steadily under load.
- The cores clock down between frames, so a commit in the app takes longer than the same work in a Go benchmark; measure a change both ways.
- A commit allocates only what it sends: compare a node's values on the stack and copy them into an item once they changed, and prepare a text from the node's own copy, which an interface call would otherwise move to the heap. Flex layout reuses its item buffers and each container its list of out-of-flow boxes (`State.spare`), which a table's absolutely placed cells allocated per row every layout. The reactive graph links a computation and a source with one slice of pairs a side, and effects keep their function without a wrapper: half the objects the collector marks of a table's cells.
- A new view's default style, empty props and empty event mask are not sent ([mount protocol](core.md#mount-protocol)): the gallery's start went from 4,605 items to 3,186, and opening the DataTable at 1200×800 from 13,763 to 11,676.
- macOS draws only content that changed and can be seen ([text and drawing](text.md#text-and-drawing)), and decides what can be seen from the batches' state rather than by asking AppKit. macOS and iOS send a content layer's frame only when it changes: a table's cells keep theirs as their texts change, and each `setFrame:` with its `CGRect` cost purego a reflected call.
- A view AppKit hit-tests answers from Go, a callback of 2,000–4,000 instructions, so a macOS scroll view's container asks only the children the point may reach, by the frames Go holds (`containerHitTest`): a pick over a scrolling list calls into Go half as often.
- Apple platforms clip and shade without masks where they can, as NSH does: the window server redraws every mask of a view on each frame it moves. One radius is the layer's own corners and curve, an opaque background moves to a fill layer above the outer shadows so they need no mask, and inset shadows are clipped by a holder's rounded bounds ([styles](styles.md#drawing)). Scrolling the gallery's Images or Styles page at 60 Hz in a 1200×800 window, Instruments' Animation Hitches measured the window server's renders at 6.6–6.8 ms at the 95th percentile and 1–2 expensive ones in about 700 frames, from 19.6–20.6 ms and 175 with masks.
- Apple platforms keep the style each layer was last drawn with, and a frame that only moves or fades a view sets just its transform and opacity (`apple.RestyleLayer`): drawing the whole style again remade its colors, shadow, filters and gradients every frame. On an M1 Max at 120 Hz in a 1200×800 window, the gallery's Animations page went from 36% to 18% of a core and the window server's GPU from 65% to 60%, against NSH's 13.5% and 62%, the three interleaved.
- The window server's readings follow everything on screen, so compare builds interleaved, against NSH and an idle round: any animation at 120 Hz costs it about half a core, NSH's as well, however small, and a window material about 7 more points of GPU while the window changes, as the gallery's Stocks page (63.5% with the material, 56.6% without, NSH 54%). So what loops as long as work lasts runs at `AtFrameRate` ([animated values](animation.md#animated-values)): on an M1 Max at 120 Hz, a conversation's working dots in a 1512×868 window with a material went from 61–68% of the GPU and 35% of a core in the window server to 16% and 13–15%, against 12% and 10% with nothing moving.
- Windows draws and presents only what changed ([partial frames](#partial-frames-on-windows)).
- On the web every JavaScript object and string a call passes or returns costs Go's WebAssembly a finalizer, whose cost grows with how many are live; at 33% of the main thread they held the DataTable's wheel scroll at 10 fps. Frames and plain texts are queued as numbers and bytes and applied by the page in one call per run of them, as NSH sends a batch (`dom.go`); the inline styles left go through `cssBox`, which sends only what changed, and property names are made once (`cssName`). A batch's scrolls wait for its other changes, so the page lays out once for them.
- Visual styles reach the page as classes, as NSH's do (`classes.go`): a view's look, what its events make of the pointer and selection, a text's paragraph and spans and a field's text are each a class named by the hash of its CSS and inserted once, so elements hold inline only their geometry and what animates, which goes inline over the class and leaves once nothing animates. After defining 16 classes, a view's slot takes a rule of its own, changed in place. An element's margins inside its parent's border are sent only when they change. In headless Chrome at 1200×800 the gallery's inline style went from 207 to 52 bytes an element, and the style text, rules and class names sent opening Text, List, Table, Tabs and the DataTable from 109, 81, 25, 17 and 768 KB to 57, 23, 9, 4 and 219 KB (the DataTable's 27,853 property writes to 160), every page's pixels as before in light and dark.
- The web measures a text of one style on one line with a canvas's `measureText`, cached by font and text, and its line's height once per style (`measure.go`), as NSH does: laying the measuring element out for each new text forced the page's layout. The element then keeps the text on one line, so a fraction of a point between the canvas and the page cannot wrap it. Spans, attachments, breaks, letter spacing, numeric and caps variants and non-Latin text are measured in the element.
- Where the page's policy forbids `new Function` (a Content-Security-Policy without `'unsafe-eval'`), `dom.go`'s helpers are missing and each operation goes through `syscall/js`, correct but slower.
- The gallery against NSH 0.4.0's release build of its own, the same steps on both: the scenarios of `examples/gallery/perf_test.go` (`NSGO_GALLERY_PERF=1 go tool nsgo test -target <t> -fps -run TestPerf`, `-env NSGO_GALLERY_PERF=1` on phones) against the same steps in an `nsh test` file, a DataTable ticking for 5 s then scrolled 6000 points down and back three times, the NavStack demo's deep stack pushed four pages deep and popped three times, and the Animations page left running. The monitors differ (NSGo's panel draws on Linux and ticks Windows' frame clock every refresh; NSH logs a line), so where they disagree the process' CPU without them decides, read between `Mark`'s lines. Rows of 2026-11-09, but headless (2026-11-08):

  | Where | NSGo | NSH |
  | --- | --- | --- |
  | Headless, 390×844, ticking, scrolls of 4 and 90 points: a frame crossing no row or ticking; one crossing rows (median, 90th percentile) | 8 items in 0.03, 0.08 ms; 33 items (at most 42) in 0.10, 0.17 ms | 7–8 items in 0.95–1.06, 1.8–2.0 ms; 26–33 items (at most 85) in 0.17–0.53, 1.3–1.9 ms |
  | Headless, 390×844: the Animations page; a press of its Release, Stagger, Sequence, Loop, Stop or Reset | 3,355 items in 6.7 ms; 1–28 items in at most 0.15 ms | 3,533 items in 6.9 ms; 2–24 items in at most 1.8 ms |
  | Headless Chrome, 1200×800 at 2×, ticking; a touchpad's scroll at 1000, 2000 and 6000 points a second (frames, main thread busy) | 22–24%; 60, 59 and 57 fps at 49, 58 and 94%, no task over 50 ms | 21–32%; 60, 60 and 60 fps at 55, 63 and 90% |
  | Headless Chrome, the Animations page | 60 fps, 23–26% | 60 fps, 23–29% |
  | macOS at 120 Hz, 1200×800, ticking; scrolling; the stack (fps, main thread a frame, CPU) | 0.8 ms, 32–34%; 115 fps, 1.2 ms, 58%; 108 fps, 0.65 ms, 16% | 1.4–1.6 ms, 27–29%; 100 fps, 3.1 ms, 84–92%; 86 fps, 1.4 ms, 18–21% |
  | macOS, the Animations page: main thread a frame; process CPU without the monitor | 1.1–1.5 ms; 13–15% | 0.5 ms; 8% |
  | Windows 11, 260 Hz, process CPU: ticking; scrolling; the stack | 10–15%; 29–44%; 25–31% | 7–14%; 29–32%; 18–24% |
  | Linux, Hyprland, 60 Hz: main thread a frame scrolling; process CPU ticking, scrolling, the stack | 5.4 ms; 16–17%, 51–52%, 10% | 2.2 ms; 14%, 32%, 6% |
  | iOS simulator, iPhone 18 Pro: ticking; scrolling; the stack (fps, main thread a frame and its p95, CPU); a push's batch | 0.27 ms, 4–6%; 59 fps, 0.4–0.5 ms, 15–19%; 0.4–0.6 ms, p95 2.0–3.2 ms, 10–13%; 5–6 ms | 0.11 ms, 12–19%; 52–54 fps, 0.15 ms, 20–21%; 0.45 ms, p95 0.9 ms, 9%; 4–7 ms |
  | Android emulator (x86-64), process CPU: ticking; scrolling; the stack | 70%; 125–127%; 46–48% | 82%; 111%; 57% |

  Against NSH 0.5.0's release build (2026-11-12), the same scenarios; NSH's test opens the NavStack demo by its card or its sidebar row, scrolls to "A deep stack" and backs out five times, as its "To the index" now leaves the demo:

  | Where | NSGo | NSH 0.5.0 |
  | --- | --- | --- |
  | Linux, Hyprland, 60 Hz: main thread a frame scrolling; process CPU ticking, scrolling, the stack (two rounds) | 5.5 ms; 16%, 51%, 10% | 1.6 ms; 9%, 22–23%, 7% |
  | Android emulator (x86-64), process CPU: ticking; scrolling | 69–70%; 118–126% | 68%; 110% |
  | macOS at 120 Hz, 1200×800, the monitors' process CPU, another workspace's iOS simulator busy meanwhile: ticking; scrolling; the stack (fps, main thread a frame, CPU) | 0.25 ms, 10–18%; 120 fps, 0.65 ms, 33–42%; 107–112 fps, 0.2 ms, 6–11% | 2.6 ms, 29–36%; 120 fps, 2.9 ms, 81%, down to 28–35 fps between scrolls; 86–90 fps, 1.9 ms, 22–29% |
  | macOS, the Animations page, as above: main thread a frame; CPU | 0.3–0.8 ms; 6–15% | 0.6 ms; 10–14% |

  Windows waits for a signed-in session, as SSH has no desktop to draw in, and the iOS simulator and headless Chrome for a quiet machine and NSH's web build.

  Measure what a user does: a touchpad's scroll (`Input.synthesizeScrollGesture` over CDP on the web, `App.Scroll` on a target, whose gesture moves 20 points a frame), ticking, at the size users run, and read whether the cells keep up as well as what a frame costs. A wheel's notches scroll by the page's own steps (`wheel.go`), as NSH's do, and a touchpad's scroll is the browser's. Run one measurement at a time, interleaved with NSH's, and check that nothing else runs: a single run moves by a fifth, and a headless browser left behind took 70% of a core and moved every reading.
- The browser fires a scroll's events in a frame's rendering steps, so a commit run there and applied at once lengthened the frame past the display's: the ticking DataTable scrolled at 6000 points a second drew every other frame. A scroll view's move is reported in a task of its own after the frame (`scrollSoon`), and the bound values that follow it have already moved, so what shows keeps up (31 to 57 fps).
- A view whose translation animated values set goes to a compositing layer of its own while they do (`will-change`): the DataTable's regions, moved under its scroll bar's sticky track every frame, made Chrome lay out its layers again each frame, 15 ms of the 16 (30 to 59 fps at 2000 points a second).
- Restyling compares a node's layout style field by field (`layout.Style.Equal`): `reflect.DeepEqual` took a tenth of the page's time in a fast scroll.
- The animation graph marks every bound view stale with a flag rather than setting each in a map every frame (`driver.Animator`).
- iOS slides a stack's screens themselves rather than a snapshot of the one it showed ([navigation](core.md#layers-and-navigation)), and places a view without a transform with one `setFrame:`: the snapshot waited 5–13 ms on the render server, and each message with a struct costs purego about 2 µs. In the simulator a push's batch went from 10–20 ms of the main thread to 5–6 ms, against NSH's 4–7 ms, and a pop's from 7–14 ms to 1–3 ms. NSH's lower p95 a frame is the frames its navigation controller's slides record; NSGo's slides are Core Animation's and record none.
- Apple platforms set an animated layer's transform through the implementation `setTransform:` has under NSGo's override, and register the messages that read its bounds once: each frame's message, the override's call back into Go and its message on passed the 128-byte matrix through reflection, and `objc.Send` registered a function each call. A view whose scale crosses a raster step redraws the content layers under it, found going down from it rather than up from every view. The macOS Animations page went from 16–20% of a core to 13–15%.
- Linux draws a text's glyphs as GSK text nodes (`gtk_snapshot_append_layout`), as NSH does, which the renderer draws from its glyph cache: a Cairo node of the text was rastered on the CPU whenever its view was drawn again. The DataTable's scroll went from 8.7 to 5.4 ms a frame and 86 to 60% of a core. Hidden canvases are looked at once a frame, before it draws, not after every view's allocation.
- Go's collector runs on the page's one thread, and once a cycle starts its idle marker finishes it before control returns to the page, so the marking landed in the task of the frame that started it: a fast scroll of the ticking DataTable, allocating 30 MB a second over a 45 MB heap, held 3–5 tasks of up to 110 ms in 6 s; NSH has no collector. The page collects in a task of its own instead (`collect.go`): the automatic trigger is at `GOGC=400`, and once the app has allocated what `GOGC=100` would collect at, `runtime.GC` runs when the page is idle, or within 500 ms. A collection there takes about 34 ms and no task goes over 50 ms; the heap peaks near three times what is live. `GOGC` in the page's environment keeps the runtime's pacing. Commits do not copy their items there (`CommitShared`), and the demo formats a row into one string, which with the work above took the scroll from about 30 to 20 MB a second.
- Canvases keep the static part under their animated arguments, and macOS, iOS and Linux draw only the area a change covers ([canvas](canvas.md#animated-arguments)). A canvas still plays all of the rest each time it draws. `go test ./internal/platform/apple -run '^$' -bench PlayChart` plays the Stocks demo's chart five hours into its session that way: 2,418 ops in 0.39 ms (93,018 ops in 44 ms when each `FillRect` recorded the line before it again and every op was a registered call). Record a path once ([recording](canvas.md#recording)) and cross into the platform as few times as an op allows ([players](canvas.md#players)).
- On an M1 Max at 60 Hz in a 1200×800 window, the gallery's Stocks demo costs 35–40% of a core, early in its session and five hours into it, and Animations 13%, as NSH's build of its gallery does on the same machine (its `examples/gallery/tools/perf.py`).
- Keeping canvases' static parts and drawing partial frames, the gallery's process CPU over 15 s, two interleaved runs each against the commit before:

  | Machine | Stocks | Canvas |
  | --- | --- | --- |
  | Windows 11, RTX 5080, 260 Hz | 95–98% → 34–36% | 84–85% → 25–29% |
  | Linux, Hyprland, Radeon 680M, 60 Hz | 70% → 61–62% | 36% → 23–24% |
- The main thread never waits for the GPU: Metal shader views take drawables and present on goroutines of their own ([shaders](shaders.md#platforms)).

## Partial frames on Windows

As NSH's, a frame draws only what changed since its back buffer was drawn and presents only what changed since the last frame (`internal/platform/windows/damage.go`):

- A change of a view queues it with `redrawView`, noting where the last frame painted its subtree; `redrawAll` draws everything, for the environment, the chrome, a size and what is not tied to a view. Applied items queue their views; a removed view is queued before it goes, an inserted one after.
- At the frame, a walk down the paths to the queued views records where each now paints (`measure`, placed as `painter.draw` places them, clipped by the ancestors that clip), and their subtrees' new areas join the old ones.
- The swap chain is `FLIP_SEQUENTIAL` with two buffers, so a frame draws its own areas and the last frame's, merged to at most 24, cleared and clipped to, and presents its own with `Present1`. Views whose subtree meets no area, or shows nowhere, are left out, in every frame.
- A window with web views, a material or native components' windows, a change under a filter or a 3D transform, a new or resized swap chain, and the frame after a whole one draw everything. A paint the system asks for with nothing queued draws nothing: DWM keeps what was presented.

`TestPartialFramesDrawWhatAWholeOneDoes` checks the window's pixels after partial frames against a whole frame.

## Frame clocks

macOS follows NSH's surface:

- While something animates on the frame clock (the animation graph, a layout animation, a scroll view moving or fading its indicator), batches wait for the start of the next frame. A frame the app asked for alone holds nothing back.
- The app's frame requests hop to the main thread only to start the clock or bring it to the full rate: while it ticks at that rate, the engine goroutine sets a flag the next tick takes (`RequestFrame`), as on iOS and watchOS. An app asking every frame sends no closure through purego's reflected callback.
- A scroll a tick reports on its way to the frame the app asked for is queued without waking the engine (`engine.DispatchBeforeFrame`) and runs in that frame's pass: a list the app scrolls every frame costs one engine pass and one commit a frame, not two.
- A scroll frame moves only a layer's translation, and AppKit learns the offset once the scroll rests ([macOS scroll views](scrolling.md#macos-scroll-views)). What is under a mouse that does not move is picked once the scroll settles as well: picking at each batch's end was 14% of the main thread's instructions on a list scrolled every frame, more with the pointer over the list (2026-11-13).
- A frame applies them for at most 4 ms (`frameBatches`), at least one whole batch, and carries the rest: a scroll's frames are not delayed by batches landing between them, and a burst spreads over frames.
- A resize applies what waited, then waits, at most 16 ms, for the commit at the new size (`engine.ResizeAndWait`) and applies it before AppKit draws the window: the one place the main thread waits for the engine.
- An engine that missed that deadline is not waited for again until its commit lands, so a busy app resizes as before.
- The display link does not fire while the window is occluded or minimized or the screen is locked: while frames are wanted and it has not ticked for 100 ms, a 60 Hz timer ticks in its place, so animations still end.

Every tick on macOS, iOS and watchOS passes the main goroutine through Go's scheduler (`apple.YieldFrame`). That goroutine lives in the platform's callbacks and never yields by itself, so Go's monitor thread, finding it on one P for 10 ms, takes the P and polls every 20 µs for a while: about 3,000 wakeups a second while an app animates. A yield a tick is enough for frames up to 9 ms apart; between slower ones, a 60 Hz display's or a slowed animation's, a timer yields every 8 ms and ends 100 ms after the last slow frame. On macOS the wakeups fall to about 400 a second, and a list scrolled every frame costs 6% fewer instructions at 120 frames a second and 9% at 60 (2026-11-13); iOS and watchOS are not measured.

Windows follows the compositor: a goroutine waits for each composition with `DwmFlush` and posts `wmTick`, one at a time, while something animates, and parks while nothing does. It sleeps a frame's 16 ms where `DwmFlush` fails, and ticks at most every 3 ms should it return at once.

purego prices each foreign call by what its Go signature asks of reflection (Apple M1 Max):

| Call | Cost |
| --- | --- |
| `purego.SyscallN`, `apple.Msg`, a function set by `registerSyscall` | Under 0.15 µs |
| A registered function or `ID.Send` with scalar arguments | 0.2–0.5 µs, and allocations for the collector |
| `objc.Send[T]`, which registers a function on every call | 0.4 µs |
| `objc.RegisterName`, `objc.GetClass`; `apple.Sel`, `apple.Class`, which keep what they found | 0.26 µs; 0.01 µs |
| A struct argument or result | About 2.5 µs |

Name selectors and classes with `apple.Sel` (each backend's `sel`) and `apple.Class`. On per-frame paths, send messages whose arguments and result are objects, integers or booleans with `apple.Msg`, and set C functions of pointers and integers with `registerSyscall` instead of `purego.RegisterLibFunc`. Send a point to super with `apple.SendSuperPoint`, registered once; the generic `apple.SendSuper` registers a function on every call. Pass runs by pointer, as `CGContextAddLines` takes a polyline's points and `CGContextAddRects` a rectangle, rather than a call for each. Declare a 16-byte struct argument (`CFRange`, `CGSize`, `CGPoint`) as its two fields, which arm64 and x86-64 both pass in two registers; larger structs pass differently on the two, so they stay structs. Keep immutable CoreFoundation objects, such as fonts, attributes and frame paths, in bounded caches.

## Where Go departs from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| `JS` per commit | `APP` per commit | Apps are Go |
| `ops` and mount items in a commit | Mount items | NSGo's commit does not count ops |
| Linux and Windows log one line of readings a second | Every native target shows macOS's panel | One monitor, read the same way on every target |
| Windows draws the whole window for a paint the system asks for with nothing queued | It draws nothing | The flip model's last frame is still on screen; PrintWindow's captures asked for such paints |
