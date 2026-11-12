# NSGo and NSH

NSGo ports the design of [Native Shell (NSH)](https://github.com/longbridge/nsh) to Go. The architecture, the names and the behavior apps see are NSH's; the language, the toolchain and a few platform mechanisms are Go's. This page says how the two relate as a whole, what NSGo adds and leaves out, and how it reads NSH's releases. Read it before porting NSH's work or departing from it.

Each subject records the departures of its own area ([by subject](#departures-by-subject)). This page holds only those that cut across subjects.

## What stays NSH's

- **The architecture.** One engine per surface owns the reactive graph, the shadow tree, styles, layout and text. Each commit sends one batch of mount items to a backend that mounts native views, and what animates runs on the platform's frame clock.
- **The backends.** UIKit, Android views, AppKit, Win32 drawn with Direct2D, GTK 4, and DOM elements in a browser.
- **The API.** Elements, props, events, style keys, Tailwind classes and `base`'s components keep NSH's names, with each API in NSH's home: `core`, `base`, `mobile` or `desktop`.
- **The algorithms.** Solid's reactive graph, Taffy 0.15's layout (`internal/layout`, checked against Taffy's fixtures) and NSH's scroll physics, list virtualization, dock layout and data table.
- **The rule that `base` has no look.** Components own behavior, state and accessibility, and the app styles every part.
- **Tests that act as a user.** Playwright-style selectors drive the app headless or with each platform's native input, in tests of NSGo's own ([testing](testing.md)).

## What differs

### Language and runtime

| NSH | NSGo | Why |
| --- | --- | --- |
| A Rust core; apps in TSX, Rust components optional | A Go core; apps in Go, TSX on top of it | Go apps are native code compiled with the engine |
| QuickJS, a C library, runs every native app | goja, written in Go, runs TSX apps only, and the page's engine on the web as NSH's | No C and no cgo on any target |
| The `taffy` crate | A Go port of Taffy 0.15's algorithms | Go has no Taffy; its fixtures prove the port |

### Go's API shape

| NSH (Rust) | NSGo | Why |
| --- | --- | --- |
| Builders, `#[component]`, derive macros | Struct literals; components are types with `Render` | Go's idiom for many optional fields; no macros |
| `Reactive<T>` and `IntoReactive` | A `func() T` field beside the static one | A method value such as `sig.Get` already is the accessor |
| Copy handles in a generational arena | Pointers and the garbage collector | Go frees what nothing references |
| `TypeId` context keys | `Key[T]` values declared as package variables | Typed, comparable and cheap |
| A future executor on the engine thread | Goroutines, `Scope.Post` and `Scope.Context` | Go has them built in |
| `catch_unwind` | `recover`, reported as `PanicError` | The same, in Go terms |
| Options that default to `true` (`closeOnEscape`) | Negative names (`NoCloseOnEscape`) | The zero value must be the default |
| CSS strings for shadows, gradients, filters and fonts | Go values built by constructors (`LinearGradient`, `Blur`) | Styles are typed; `ParseColor` and the CSS parsers serve classes and TSX |

### Toolchain

| NSH | NSGo | Why |
| --- | --- | --- |
| A prebuilt `nsh` distribution, built with Rust | `cmd/nsgo`, which shells out to `go` and Xcode's tools ([tool](tool.md)) | Only Go and Xcode are assumed |
| `nsh.json` | `nsgo.yaml` | Hand-edited configuration is YAML |
| `nsh run`, `dev`, `package` and `build` | `nsgo dev`, and `nsgo build`, which writes each target's package | One command per step of the loop |
| `nsh check` downloads `tsc` from npm | `go run` builds TypeScript 7's checker | No Node or npm |
| Release bundles in bytecode, their source maps apart for `nsh symbolicate` | Source with its map inline, so stacks map as they are reported | goja saves no bytecode |
| Each desktop backend builds on its own OS with its toolchain | Every target builds from a Mac with Go and Xcode | x86_64 Android alone needs an NDK's clang |

### Platforms and embedding

| NSH | NSGo | Why |
| --- | --- | --- |
| Android runs `libnsh.so` in the app's process over JNI | A Go executable the Java runtime starts, over pipes ([platforms](../../ARCHITECTURE.md#platforms)) | Go builds Android shared libraries only with cgo and the NDK |
| A Swift module and a C ABI for iOS hosts | Classes Go registers, one cgo export the tool generates ([embedding](embedding.md)) | NSGo writes no C or Swift |
| A runtime library loads an app's bundle | The framework, library or program is the app | Go apps are compiled with the engine |
| The GPUI backend embeds apps in GPUI apps | Desktops embed with `nsgo.NewView` on their own backend | One backend per desktop to keep |
| `nsh/updater` swaps in the next version itself from a signed `latest.json`, and only links on phones | Sparkle on macOS and WinSparkle on Windows from appcasts `nsgo build` signs, the same appcast on Linux and Android, the App Store on iOS ([updates](updates.md)) | Each platform's own updater and windows, as the user decided (2026-11-10) |

### Consistency across platforms

NSGo departs from NSH and from MyGo in the same direction.

| NSH | MyGo | NSGo |
| --- | --- | --- |
| Native views, each showing as its platform draws it | Draws every view itself, alike everywhere | Native views, alike everywhere at MyGo's quality: a component one platform has natively, such as iOS's Liquid Glass, others imitate the same way |
| Components may change by device | — | `core` and `base` change nothing by device on their own; sizes and forms by device are the component author's, `ui`'s or the app's ([device differences](device-differences.md)) |

## What NSGo adds

- GPU shaders written in Go besides NSH's WGSL, both compiled for each backend by NSGo's own front ends rather than naga and wgpu ([shaders](shaders.md)).
- Icons as vector elements and the bundled Lucide pack, where NSH loads SVG files as images ([icons](icons.md)).
- Scroll edge effects under the app's bars, after iOS 26's ([scrolling](scrolling.md#edge-effects)).
- Apps that open their windows, panels and tray icons themselves, on one engine, with global shortcuts and shutdown modes, after Avalonia's ([application and windows](application.md#departures-from-nsh)).
- Window materials and window controls placed in the app's own title bar ([services](services.md#where-nsgo-departs-from-nsh)).
- A theme and an appearance per subtree, a neutral palette by default and a warm one as `ui.WarmDuskTheme` ([themes](themes.md#departures-from-nsh)).
- Zooming scroll views on macOS, and AppKit's bounce where momentum reaches an edge ([scrolling](scrolling.md#scrolling)).
- Images on Android's clipboard, through a content provider ([services](services.md#where-nsgo-departs-from-nsh)).
- File pickers and save dialogs for files of any type, streamed rather than passed through the platform's bridge ([services](services.md#files)).
- A terminal running programs with Ghostty's emulator ([terminal](terminal.md)).
- Widgets on macOS and iOS, timelines of views the app writes and a SwiftUI extension the tool compiles draws ([tool](tool.md#widgets)).
- Message catalogs each package owns, in YAML, compiled to Go by the tool, which an app supplies in more languages, where NSH leaves translations to apps ([languages](languages.md)).

## What NSGo leaves out

- The standard modules (`fs/promises`, `process`, `net`, `buffer`, ...) and WebSockets. Go apps use Go's libraries with `Scope.Post`, and TSX modules reach the host app instead.
- `EngineContext`, since a host gives each engine's app function its values directly.
- QuickJS's bytecode and heap limit: goja has neither ([TSX](tsx.md#the-engines)).
- The GPUI backend and the LayerFS trial.
- 32-bit Android (`armeabi-v7a`), since Go links android/arm only with cgo and an NDK.

## Reading NSH's releases

NSGo has grown from NSH and now goes its own way (decided by the user, 2026-11-08): TSX above all may depart from it, and no test runs NSH's apps, its compat cases or anything else from its checkout. Its releases stay a source of designs to port when they suit NSGo. NSH's checkout (`../nsh`, [contributing](../public/en/contributing.md#nsh)) is at the latest release tag, 0.5.0, and the [roadmap](../roadmap.md) lists what NSGo may still take from that release. The gallery goes its own way too: its index has Primitives and Components as groups of their own and no search (decided by the user, 2026-11-03), related components share a demo named after the first, such as `Button` with the toggles and `Checkbox` with `Radio` and `Switch` (2026-11-08), and it lists only the components NSGo has. To read a new release:

1. In NSH's checkout, run `git fetch --tags --force` and check out the release's tag.
2. Read `git log <old tag>..<new tag>` there and the pages of `nsh/docs/dev/` it changed.
3. Run the scenarios NSH's pages measure (`performance.md`, `data-table.md` and the other components' Performance sections, `web.md`) on NSH's build and on NSGo on one machine, each backend in turn, as [performance](performance.md#keeping-frames-cheap) does for the DataTable: matching the API leaves what a frame costs unchecked.
4. Rewrite the roadmap from what these find, and update the subjects whose departures changed.

## Departures by subject

| Subject | Where |
| --- | --- |
| Core | [Where Go departs from NSH](core.md#where-go-departs-from-nsh) |
| Styles | [Resolving](styles.md#resolving) |
| Themes | [Departures from NSH](themes.md#departures-from-nsh) |
| Scrolling | [Scrolling](scrolling.md#scrolling) |
| Services | [Where NSGo departs from NSH](services.md#where-nsgo-departs-from-nsh) |
| Hosted views | [Shared design](native-views.md#shared-design) |
| Canvas | [Where NSGo departs from NSH](canvas.md#where-nsgo-departs-from-nsh) |
| Animation | [Where Go departs from NSH](animation.md#where-go-departs-from-nsh) |
| Components | [Charts](components.md#charts), [Tables](components.md#tables), [Dock](components.md#dock) |
| Text view | [Parts](textview.md#parts) |
| Editor | [Where NSGo departs from NSH](editor.md#where-nsgo-departs-from-nsh) |
| Performance | [Where Go departs from NSH](performance.md#where-go-departs-from-nsh) |
| Embedding | [Departures from NSH](embedding.md#departures-from-nsh) |
| Testing | [The model](testing.md#the-model) |
| TSX | [Where this departs from NSH](tsx.md#where-this-departs-from-nsh) |
