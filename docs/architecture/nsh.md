# NSGo and NSH

NSGo ports the design of [Native Shell (NSH)](https://github.com/longbridge/nsh) to Go. The architecture, the names and the behavior apps see are NSH's; the language, the toolchain and a few platform mechanisms are Go's. This page says how the two relate as a whole, what NSGo adds and leaves out, and how it follows NSH's releases. Read it before porting NSH's work or departing from it.

Each subject records the departures of its own area ([by subject](#departures-by-subject)). This page holds only those that cut across subjects.

## What stays NSH's

- **The architecture.** One engine per surface owns the reactive graph, the shadow tree, styles, layout and text. Each commit sends one batch of mount items to a backend that mounts native views, and what animates runs on the platform's frame clock.
- **The backends.** UIKit, Android views, AppKit, Win32 drawn with Direct2D, GTK 4, and DOM elements in a browser.
- **The API.** Elements, props, events, style keys, Tailwind classes and `base`'s components keep NSH's names, with each API in NSH's home: `core`, `base`, `mobile` or `desktop`.
- **The algorithms.** Solid's reactive graph, Taffy 0.15's layout (`internal/layout`, checked against Taffy's fixtures) and NSH's scroll physics, list virtualization, dock layout and data table.
- **The rule that `base` has no look.** Components own behavior, state and accessibility, and the app styles every part.
- **TSX apps.** An NSH app runs on NSGo once its imports name `nsgo` in place of `nsh` ([TSX](tsx.md)).
- **Tests that act as a user.** Playwright-style selectors drive the app headless or with each platform's native input ([testing](testing.md)).

## What differs

### Language and runtime

| NSH | NSGo | Why |
| --- | --- | --- |
| A Rust core; apps in TSX, Rust components optional | A Go core; apps in Go, TSX on top of it | Go apps are native code compiled with the engine |
| QuickJS, a C library, runs every app | goja, written in Go, runs TSX apps only | No C and no cgo on any target |
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

## What NSGo adds

- GPU shaders written in Go, compiled for each backend ([shaders](shaders.md)).
- Icons as vector elements and the bundled Lucide pack, where NSH loads SVG files as images ([icons](icons.md)).
- Scroll edge effects under the app's bars, after iOS 26's ([scrolling](scrolling.md#edge-effects)).
- Window materials and window controls placed in the app's own title bar ([services](services.md#where-nsgo-departs-from-nsh)).
- A theme and an appearance per subtree, Onevra's palette by default ([themes](themes.md#departures-from-nsh)).
- Zooming scroll views on macOS, and AppKit's bounce where momentum reaches an edge ([scrolling](scrolling.md#scrolling)).
- Images on Android's clipboard, through a content provider ([services](services.md#where-nsgo-departs-from-nsh)).

## What NSGo leaves out

- The standard modules (`fs/promises`, `process`, `net`, `buffer`, ...) and WebSockets. Go apps use Go's libraries with `Scope.Post`, and TSX modules reach the host app instead.
- `EngineContext`, since a host gives each engine's app function its values directly.
- QuickJS's bytecode and heap limit: goja has neither ([TSX](tsx.md#the-engine)).
- The GPUI backend and the LayerFS trial.
- 32-bit Android (`armeabi-v7a`), since Go links android/arm only with cgo and an NDK.

## Following NSH's releases

NSGo tracks NSH's releases, never its `main`: `nsh/` is checked out at the latest release tag, and the [roadmap](../roadmap.md) lists what NSGo still lacks of that release. To move to a new release:

1. In `nsh/`, run `git fetch --tags --force` and check out the release's tag.
2. Read `git log <old tag>..<new tag>` there and the pages of `nsh/docs/dev/` it changed.
3. Compare the declarations NSH writes to `examples/gallery/.nsh/types/` when any `nsh` command runs on its gallery with those `tsx.WriteDeclarations` writes.
4. Run NSH's apps as oracles, imports renamed: its gallery (`TestNSHGallery`), its gallery's and examples' `*.test.tsx`, and the cases of `tests/compat/`.
5. Rewrite the roadmap from what these find, and update the subjects whose departures changed.

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
