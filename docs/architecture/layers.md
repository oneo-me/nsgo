# Package layers

NSGo's packages form layers, and each imports only downward: apps sit on components, components on `core`, and `core` on the protocol in `internal/driver`. Backends speak that protocol too, so `core` imports no platform. Read this before adding a package, changing what one imports or deciding where new code belongs.

[Code map](../../ARCHITECTURE.md#code-map) says what each package does; this document says how they stack and why.

## Layers

Each layer imports only the layers below it, and packages within a layer may import each other. The tree groups packages by layer, not by folder:

```text
nsgo
├── Apps
│   ├── examples/gallery, tray, tsx      Demos of what apps are made of
│   ├── examples/embedded, embed-*       An embeddable app and its hosts
│   └── bench/quotes                     Benchmarks
│
├── Entry points
│   ├── nsgo (root)                      Picks a backend and starts the app
│   ├── tsx                              Runs TSX apps on goja, or the page's engine
│   │   ├── internal/js                  The engines' one API
│   │   ├── bundler, compiler            Compiles and loads their bundles
│   │   ├── textview, editor, diff, terminal  Register base's optional components
│   │   └── tsxtest                      Runs *.test.tsx as Go tests
│   └── nsgotest                         Runs apps in tests, headless or on a target
│
├── Components
│   ├── ui                               Styled components on base
│   │   ├── chat                         A conversation
│   │   └── filepicker, diff, commitgraph, flow  A file picker and styled forms
│   ├── base                             Unstyled controls, popups, navigation
│   │   ├── datatable, dock, plot, toast Complex components
│   │   ├── commitgraph                  A Git history's revision graph
│   │   ├── flow                         A directed diagram
│   │   ├── textview, editor, diff       Complex components with heavier dependencies
│   │   ├── terminal                     A program in a pseudo-terminal
│   │   ├── locales                      Their catalogs
│   │   └── internal/                    What they share
│   ├── mobile                           What only phones have
│   ├── desktop                          What only desktops have
│   │   └── locales                      Its menus' words, which backends read
│   ├── icons/lucide                     The bundled icons
│   └── i18n                             The language the app shows
│       └── internal/cldr                Dates and pseudo locales by CLDR's data
│
├── Backends
│   └── internal/platform
│       ├── macos, ios                   AppKit and UIKit
│       ├── windows                      Win32 and Direct2D
│       ├── linux                        GTK 4
│       ├── android                      Java runtime over pipes
│       ├── web                          WebAssembly and the DOM
│       ├── apple                        Shared by macOS and iOS
│       ├── field, hover, finger         Shared input
│       ├── scrollbar, textselect        Shared scrolling and selection
│       ├── imagedecode, imageload       Shared images
│       ├── menutext                     Shared menu labels
│       ├── picked, fontfile             Shared picked photos and font files
│       └── queue, perfhud, canvastest   Shared batches, readings and test drawings
│
├── Engine
│   ├── internal/engine                  One app on its goroutine, a batch per commit
│   └── internal/testhost                Where nsgotest finds a backend's test host
│
├── Core
│   ├── core                             Elements, styles, reactive graph, shadow tree
│   ├── internal/layout                  Block, flexbox and grid layout
│   └── internal/shader, shader/wgsl     The shader compilers
│
├── Protocol
│   ├── internal/woff                    WOFF and WOFF2 fonts unpacked, for the bundler
│   ├── internal/driver                  Mount items, events, TextEngine, shared values
│   └── internal/shaderir                Shader IR, written as MSL, GLSL or HLSL
│
└── Build time, outside the layers
    ├── cmd/nsgo                         The nsgo tool
    ├── tools/lucidegen                  Generates icons/lucide
    └── tools/libghostty                 Builds libghostty-vt and its themes
```

What keeps each layer in its place:

| Layer | Never does |
| --- | --- |
| Apps | Import `internal/` |
| Entry points | Implement UI |
| Components | Style themselves or call the system |
| Backends | Decide what the UI contains |
| Engine | Touch main-thread-only UI objects |
| Core | Touch a native view or call the system |
| Protocol | Hold app logic |

The `nsgo` tool links no component or runtime, so changing them does not rebuild it.

## Dependency graph

A line means "imports". `layering_test.go` checks the graph on every target, and fails on an import no rule allows.

```text
                     apps (examples/*, bench/*)
         ┌───────────────────────┼────────────────────┐
         │                       │                    │
    nsgo (root)         tsx, tsx/compiler         nsgotest
         │                       │                    │
         │           ui/*  base/*  mobile  desktop    │
         │             icons/*  i18n                  │
         │                       │                    │
 internal/platform/<target>      │                    │
    │    │                       │                    │
    │  internal/engine           │                    │
    │    └───────────────────────┼────────────────────┘
    │                          core
    │                            │
    └───────────────────► internal/driver
```

- Apps may import any public package directly, `core` included. `core` and `nsgotest` alias the internal types their API reaches (`TestPublicAPINamesNoInternalType`).
- `internal/driver` imports from the module only `internal/shaderir`, for the shader programs it carries.
- `core` imports only `internal/driver`, `internal/layout` and the shader compilers (`internal/shader`, `internal/shader/wgsl`, `internal/shaderir`).
- Backends import `internal/driver`, `internal/engine`, `desktop/locales`, their shared packages and the shader writers, never the compilers, which only `core` links.
- Only a backend's test host, built with the `nsgotest` tag, imports `core`.
- `i18n` imports only `core` and `internal/cldr`; `base`'s packages import it for their catalogs.
- `ui` imports `base`, `core`, `mobile`, `desktop` and the icons; `base` never imports it.
- `tsx/tsxtest` is the one package that imports both `tsx` and `nsgotest`, and `tsx/ui` with the packages below it the TSX packages that import `ui`. `tsx/internal/js`, the engines' API, imports nothing of NSGo's, and on the web reaches the page through `syscall/js`.
- `base/terminal` is the one package of `base` that is not portable: it loads libghostty-vt through purego, runs programs on desktops and remote sessions there and on iOS, and elsewhere builds to report that it cannot (`unportable` in `layering_test.go`).

## Why core and backends do not meet

Backends reach `core` only through the engine and `core` imports no backend, so `core` builds and tests without a platform. Three packages join them:

- `internal/driver` is what they exchange: `core` writes a `MountBatch`, and a backend reports `driver.Event`s and implements `TextEngine`.
- `internal/engine` runs `core` and defines `engine.Platform`, which each backend implements.
- `nsgo` picks the backend by build tags (`run_windows.go`, `view_windows.go`) and hands it the app.

`nsgotest` is a backend of its own: headless, without the engine, applying each batch to a model of the native views. On a platform, it drives a backend's test host through `internal/testhost`.

`microphone` stands apart from the layers, as recording has nothing to do with views: it loads the system's audio libraries through purego, holds iOS's audio session of its own while it records, and elsewhere builds to report that it cannot.

Portable packages build unchanged for every target, with no purego, `syscall` or `unsafe`: `core`, `base` but for `base/terminal`, `ui`, `i18n`, `mobile`, `desktop`, `icons`, `tsx`, `nsgotest`, `internal/driver`, `internal/layout`, `internal/testhost`, the shader compilers and `internal/engine`. No package has cgo files or hand-written C.

## Backends

Each `internal/platform/<target>` maps the same protocol onto its system ([platforms](../../ARCHITECTURE.md#platforms)):

| Target | A view becomes |
| --- | --- |
| `macos`, `ios` | A layer-backed `NSView` or `UIView` per node |
| `windows` | An entry in a retained tree one window draws with Direct2D |
| `linux` | A GTK widget per view that draws itself with Cairo |
| `web` | An absolutely positioned element per view |
| `android` | An Android view per node, in the Java process the Go executable talks to over pipes |

The other packages under `internal/platform` hold what two or more backends share, such as `apple` for macOS and iOS or `hover` and `textselect` for Windows and Linux. Move code there once a second backend needs it, and add it to `platformShared` in `layering_test.go`.

## A button, top to bottom

This follows a `base.Button` from an app to the Windows backend; another backend replaces only the Windows parts.

1. The app calls `nsgo.Run(Root)`, and `Root` returns a `base.Button`.
2. `run_windows.go` is the only `run` a Windows build has, so `nsgo.Run` reaches `windows.Run`.
3. `windows.Run` creates the window, which is an `engine.Platform`, and calls `engine.Start` with the app.
4. On the engine goroutine, `Button.Render` returns a `core.View` with the app's styles, its role and its handlers.
5. `core` resolves styles, prepares text through the window's `TextEngine` (DirectWrite), lays out and commits a `driver.MountBatch`.
6. `window.Mount` queues the batch (`internal/platform/queue`) and posts `wmApply` to wake the UI thread without waiting.
7. The UI thread applies the items to its retained views and draws them with Direct2D.
8. A press returns as a `driver.Event` through `Engine.Dispatch`; `OnPress` runs on the engine and leads to the next commit.

## Where a change goes

| Change | Layer |
| --- | --- |
| A look, a variant or a styled control | `ui`, or the app for what is its own |
| An unstyled component | A file of `base`, or a package under it if it is complex |
| What only phones or only desktops have | `mobile` or `desktop` |
| A new element or service | `core` and `internal/driver`, then every backend |
| A drawing or input bug on one target | `internal/platform/<target>`, then check the other backends |
| A new package | Its layer, with a rule in `layers` in `layering_test.go` |
