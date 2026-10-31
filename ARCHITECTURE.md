# NSGo architecture

NSGo is a Go port of the design of [Native Shell (NSH)](https://github.com/longbridge/nsh). A Go engine owns the reactive graph, the shadow tree, styles, layout and text. Each commit produces one batch of mount items, and a thin platform backend applies it to native views and runs what animates on its own frame clock. Sections below say what exists; the [roadmap](docs/roadmap.md) orders what is missing.

## Reference implementation

NSH, checked out in `nsh/`, is a reference, not a constraint ([principles](AGENTS.md#principles)): port its design rather than its code, and record each deviation in the owning subject.

## Code map

Apps import the public packages; `internal/` holds the implementation, which only this module can import ([boundaries](#boundaries)).

| Package | NSH counterpart | Responsibility |
| --- | --- | --- |
| `nsgo` (module root) | `run_app`, `AppView` | `nsgo.Run`, and `nsgo.NewView` in a host's window ([embedding](docs/architecture/embedding.md)): picks the backend by build tags |
| `core` | `nsh-core` | The app's API: elements, styles, Tailwind classes, themes, the reactive graph and regions, services; the shadow tree, layout and text preparation |
| `base` | `nsh-base` | Unstyled components: controls, popups, navigation, panels, a dock, charts and tables ([components](docs/architecture/components.md)) |
| `mobile`, `desktop` | `nsh/mobile`, `nsh/desktop` | What only phones have (the tab bar) and only desktops (the window, menus) |
| `base/textview` | `nsh-base`'s `text_view` | `TextView`: Markdown, HTML or the app's blocks ([text view](docs/architecture/textview.md)) |
| `base/textview/highlight` | `nsh-base`'s `highlight` feature | Code highlighting for `TextView` with chroma |
| `base/editor`, `base/diff` | `nsh-base`'s `editor`, `diff` | `Editor`, a code editor, and `DiffView`, the changes between texts or of a patch ([editor](docs/architecture/editor.md)) |
| `base/internal/*` | `nsh-base`'s `syntax`, `window` | What `base`'s packages share: highlighting and the rows a virtualized view makes |
| `tsx` | `nsh-js` | TSX apps on goja: the `nsgo` module's reactive API and the renderer of compiled JSX on `core.Element` ([TSX](docs/architecture/tsx.md)) |
| `tsx/compiler` | `nsh-bundler` | TSX to one script with esbuild and a port of NSH's JSX compiler; `compiler.App` compiles an app's embedded sources as it starts |
| `icons/lucide` | — (NSH loads SVG as images) | The bundled Lucide pack, an accessor per icon ([icons](docs/architecture/icons.md)) |
| `nsgotest` | `nsh-testing` | App tests, headless or on a platform ([testing](docs/architecture/testing.md)) |
| `internal/testhost` | `nsh-testing`'s agent | A backend's test host for `nsgotest`: the main thread, native input, screenshots |
| `internal/driver` | `nsh-core`'s mount protocol | The mount protocol, events and `TextEngine`; what every backend runs, such as the `Animator` |
| `internal/layout` | Taffy | Block, flexbox and grid layout, checked against Taffy's fixtures |
| `internal/shader`, `internal/shaderir` | — (MyGo's fork of Ebitengine's Kage) | The shader compiler: Go's syntax checked into an IR, written as MSL, GLSL or HLSL ([shaders](docs/architecture/shaders.md)) |
| `internal/engine` | `nsh-engine` | The engine goroutine: a mailbox, a batch per commit, storage files, the performance monitor |
| `internal/platform/<target>` | `nsh-platform-*` | Native view mounting, text engine and input for one target |
| `internal/platform/apple` | Shared Apple code | What macOS and iOS share: layers, Metal, CoreText, web views, video |
| `internal/platform/*` | Platforms' shared code | What backends share: images ([images](docs/architecture/images.md)), picked photos, process readings |
| `examples/gallery` | `examples/gallery` | NSH's gallery in Go, with headless tests; the website runs its web build |
| `examples/embedded`, `examples/embed-*` | `examples/embed-*` | An app to embed, and desktop, iOS and Android apps and a web page embedding it |
| `examples/tsx` | — | An app written in TSX and styled with Tailwind classes, with tests |
| `cmd/nsgo` | `nsh-cli` | `nsgo init`, `dev`, `test` and `build` from an app's `nsgo.yaml` ([tool](docs/architecture/tool.md)); `apk` packages and signs Android apps in Go |
| `tools/lucidegen` | — | Regenerates `icons/lucide` from a pinned Lucide checkout |

## Platforms

Every target builds on macOS with Go; iOS also needs Xcode, and x86_64 Android an NDK's clang.

| Target | Package | Reaches the system through | Status |
| --- | --- | --- | --- |
| macOS | `internal/platform/macos` | purego and the Objective-C runtime; `CGO_ENABLED=0` | Runs |
| iOS | `internal/platform/ios` | The same; cgo only to link with Xcode's linker, and for an embedded framework's one export | Runs in the simulator and on devices |
| Web | `internal/platform/web` | `syscall/js` in WebAssembly; DOM elements | Runs |
| Windows | `internal/platform/windows` | `syscall` and COM: Direct2D, DirectWrite, WebView2, Media Foundation | Runs |
| Linux | `internal/platform/linux` | GTK 4, Pango and Cairo loaded with purego; WebKitGTK and GStreamer | Runs |
| Android | `internal/platform/android` | A Go executable the prebuilt Java runtime starts and talks to over pipes | Runs on arm64 devices and x86_64 emulators |

Android runs the app as a second process because Go builds Android shared libraries only with cgo and the NDK, and executables without cgo for arm64 only. iOS forbids child processes, so it links in-process with Xcode.

## Boundaries

Packages import only downward; `layering_test.go` checks it on every target:

```text
                    apps (examples/*, bench/*)
         ┌───────────────┬───────┴───────┬───────────────┐
    nsgo (root)       nsgotest base/* mobile desktop  icons/* tsx
         │               │               │               │
 internal/platform/*     │               │               │
         │               │               │               │
  internal/engine        │               │               │
         └───────────────┴───────┬───────┴───────────────┘
                               core
                                 │
                          internal/driver
```

- Apps import `nsgo`, `core`, `base`, `mobile`, `desktop`, `icons`, `tsx` and `nsgotest` only, native components included; `core` and `nsgotest` alias the internal types their API reaches (`TestPublicAPINamesNoInternalType`).
- `internal/driver` is the protocol and imports from the module only `internal/shaderir`, for the shader programs it carries. Backends import it and `internal/engine`, never `core`, and never the shader compiler (`internal/shader`), which only `core` links.
- `core` imports only `internal/driver`, `internal/layout` and the shader compiler (`internal/shader`, `internal/shaderir`).
- `internal/engine` runs one app on `core` and defines `engine.Platform`, which backends implement. `nsgotest` is the headless backend, without the engine, and on a platform drives a backend's test host.
- `core`, `base`, `mobile`, `desktop`, `icons`, `tsx`, `nsgotest`, `internal/driver`, `internal/layout`, `internal/testhost`, the shader compiler and `internal/engine` build unchanged for every target: no purego, `syscall` or `unsafe`. No package has cgo files or hand-written C.
- `base` holds unstyled components; apps or a component library style them. A component gets a package under `base` only for dependencies other apps should not link, as `base/textview` links goldmark.
- Each API has one home: `core` elements and the environment, `base` controls and navigation, `mobile` and `desktop` what only phones or desktops have. A part `core` keeps for another package's API names it in its comment.
- The main thread never waits for the engine, but up to 16 ms on macOS for a resized window's layout; the engine never touches main-thread-only UI objects.
- What animates runs on the platform's frame clock, which stops when idle: the engine sends transitions and animated values once, never a commit per frame ([animation](docs/architecture/animation.md)).
- The reactive graph belongs to the engine goroutine and never locks; other goroutines reach it through `Scope.Post`.
- Text is prepared and laid out on the engine side through a platform `TextEngine`; the main thread only draws the result.
- One engine is one app with no global state, so a host app embeds several.
- Layout (`internal/layout`) ports Taffy 0.15's algorithms step by step and passes every Taffy fixture that applies to NSH's features.

## Tool

`nsgo` is the only build entry point; it shells out to `go`, and to Xcode's `xcrun`, `xcodebuild`, `codesign` and `simctl` for Apple targets ([tool](docs/architecture/tool.md)).

- Targets name an OS and an architecture (`macos_arm64`, `windows_x64`), so each package holds one binary.
- Hot reload watches the directories of `go list -deps`, NSGo's included, and keeps the last good build running on a failure.

## Core flows

**Render.** Components run once and bind signals to element parts; regions (`Show`, `Each`, `Dynamic`, `Catch`) rebuild only their span. A commit resolves styles, prepares text, lays out and sends what changed as one `MountBatch` ([core](docs/architecture/core.md)).

**Mount.** The platform receives the batch with a single hop to its main thread and applies the items in order. Services answer with events ([services](docs/architecture/services.md)).

**Input.** Native gestures become engine events, queued without blocking; their handlers run on the engine and lead to the next commit.

## Open questions

Record a design question here, and confirm the answer with the user before choosing it.

- **Signed packages and capabilities.** Port both for TSX modules only, Go code staying trusted, as [TSX](docs/architecture/tsx.md#sandbox) proposes?

## Verification

`core`, `internal/driver` and `internal/engine` have headless tests; apps test themselves with `nsgotest`, as `examples/gallery` does, and `nsgo test -target` runs them on each backend with its native input. The Windows and Linux backends' tests open a real window and check input, services and pixels.

## Subjects

| Document | Read before changing |
| --- | --- |
| [Core](docs/architecture/core.md) | The graph, elements, regions, layers, commit, storage or the mount protocol |
| [Styles](docs/architecture/styles.md) | `Style` or how a backend draws a view |
| [Themes](docs/architecture/themes.md) | Themes, theme files or the gallery's look |
| [Text](docs/architecture/text.md) | Text, a `TextEngine`, `Input` or selection |
| [Scrolling](docs/architecture/scrolling.md) | `Scroll`, `List` or a scroll view |
| [Interaction](docs/architecture/interaction.md) | Accessibility, pointers, focus, touches, gestures or drops |
| [Services](docs/architecture/services.md) | A service, a menu, the desktop window or the app's lifecycle |
| [Hosted views](docs/architecture/native-views.md) | `WebView`, `Video` or `Native` components |
| [Images](docs/architecture/images.md) | How backends load, cache, decode or play images |
| [Canvas](docs/architecture/canvas.md) | `Canvas`, `Drawing` or a backend's canvas player |
| [Animation](docs/architecture/animation.md) | Animated values, layout animations or a backend's frame clock |
| [Shaders](docs/architecture/shaders.md) | `Shader`, the shader compiler or a backend's shader view |
| [Components](docs/architecture/components.md) | A component in `base`: controls, popups, resizable panels or the dock |
| [Text view](docs/architecture/textview.md) | `TextView`, its parsers, parts or highlighting |
| [Editor](docs/architecture/editor.md) | `Editor`, `DiffView` or their highlighting |
| [Icons](docs/architecture/icons.md) | SVG parsing, `core.Icon` or the Lucide pack |
| [Performance](docs/architecture/performance.md) | The performance monitor, a frame clock or a per-frame path |
| [Embedding](docs/architecture/embedding.md) | `nsgo.NewView`, the host channel or `-embed` packages |
| [Testing](docs/architecture/testing.md) | `nsgotest`, its steps or a backend's test host |
| [Tool](docs/architecture/tool.md) | `nsgo`: the dev server, tests on targets, packages or signing |
| [TSX and dynamic delivery](docs/architecture/tsx.md) | TSX, signed packages or the sandbox |
