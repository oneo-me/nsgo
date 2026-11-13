# TSX and dynamic delivery

NSGo runs TSX apps on NSH's design: they import `nsgo`, `nsgo/base`, `nsgo/ui`, `nsgo/mobile`, `nsgo/desktop` and `nsgo/test`, have NSH's props, events, style keys, Tailwind classes and behavior, and compile with `tsx/bundler`. Each part began as a Go port of its NSH counterpart. From 2026-11-08 TSX grows its own way (decided by the user): nothing checks it against NSH's apps, and an NSH app may need more than renamed imports. The [roadmap](../roadmap.md#tsx-and-dynamic-delivery) orders what is left.

An app embeds its sources and names its entry; the `nsgo` tool compiles it ahead for its builds, and `go run` and `go test` compile it as it starts:

```go
//go:embed *.tsx *.ts
var src embed.FS

func main() { nsgo.Run(compiler.App(src, "App.tsx")) }
```

`examples/tsx` is such an app: its modules, components, signals, inputs, lists and timers, styled with classes in light and dark, one column on phones and two from `md:`, a Go class and an async Go function of its own, and a timer module it loads with `import()`, with Go and TSX tests. `examples/gallery-tsx` is the gallery's Components in TSX, on `nsgo/ui`, `nsgo/ui/chat` and `nsgo/base`, with knobs that set their props: its apps (Stocks, AI Chat streaming scripted replies, Photos), the Conversation, the Terminal over a demo session of its own Go on phones, and the RichEditor among them, with tests in its wide and phone layouts on every target.

## The ports

| NSH | NSGo | Part |
| --- | --- | --- |
| `nsh-bundler`: Oxc, `jsx.rs`, rolldown, `hot.rs`, `split.rs` | `tsx/bundler`: esbuild, `jsx.go`, `hot.go`, `split.go` | TSX to one script; asset imports; hot and split bundles |
| `nsh-js` `reactive.rs`, `resource.rs` | `tsx/reactive.go`, `services.go` | `signal`, `computed`, `effect`, `batch`, `untracked`, `cleanup`, `resource` |
| `nsh-js` `dom.rs`, `list.rs` | `tsx/dom.go`, `components.go`, `face.go`, `elements.go` | `nsgo:rt`, what compiled JSX calls; the primitives |
| `nsh-js` `protocol.rs`, `nsh-core` `style.rs` | `tsx/protocol.go`, `hostprops.go`, `stylekeys.go`, `css.go` | Props and style objects decoded into `core`'s values |
| `nsh-js` `components.rs`, `navigation.rs` | `tsx/goface.go`, `base.go`, `dialog.go`, `navigation.go`, `toasts.go` | Go components in TSX: `nsgo/base`, an app's own |
| `nsh-js` `classes.rs`, `nsh-core` `call.rs` | `tsx/classes.go` | Go classes and functions in TSX, async ones as promises |
| `nsh-js` `hot.rs`, `chunks.rs` | `tsx/bundler/hot.go`, `split.go` (their runtimes), `components.go` | Hot updates; chunks loaded on demand |
| `nsh-js` `animated.rs`, `touch.rs`, `canvas.rs` | `tsx/animated.go`, `touch.go`, `canvas.go` | Animation, gestures and the responder, Canvas 2D |
| `nsh-js` `env.rs`, `platform_modules.rs`, `desktop.rs`, `host_channel.rs` | `tsx/env.go`, `platform.go`, `services.go`, `host.go` | The environment, services, `nsgo/mobile`, `nsgo/desktop`, `host` |
| `nsh-js` `web/` | `tsx/websrc/web.js`, `web.go`, `intl.js`, `intl.go` | Web APIs and `Intl` |
| `nsh-js` `sandbox.rs`, `nsh-core` `access.rs` | `tsx/sandbox.go`, `core/access.go` | Time budgets, stack depth, hardening, quotas; capabilities |
| `nsh-js` `declarations.rs` | `tsx/declarations.go`, `tsx/types/*.d.ts` | `.nsgo/types` |
| `nsh-testing`'s `nsh/test` | `tsx/tsxtest` | `*.test.tsx` as Go tests |
| `nsh-core`'s `Op` | `core.Element` | Elements built one change at a time |
| `nsh-package` | `tsx/pkg` | Signed packages, certificates, revocation lists and the trust store |
| `nsh-platform-web`'s `src/browser/`, `web/browser/` | `tsx/internal/js` | The page's engine on the web |

Read the NSH file before changing its port, and keep the port's names and order where Go allows, so NSH's next change maps onto it.

## The engines

goja runs the JS on every target but the web: pure Go, so it builds everywhere. On the web the page's own engine runs it, as NSH 0.4.0's does (decided by the user, 2026-11-08), which keeps goja out of the WebAssembly (13 MB less in `examples/tsx`'s package). `tsx/internal/js` is the one API both share, goja's: on goja its types under other names (`goja.go`), on the web the part `tsx` uses over `syscall/js` (`browser.go`), so the runtime is written once. Its tests (`js_test.go`) hold both to the same behavior.

The alternatives to goja fail a [principle](../../AGENTS.md#principles) or a target:

| Engine | Verdict |
| --- | --- |
| QuickJS through cgo, NSH's | cgo and C on every target |
| `modernc.org/quickjs` (QuickJS translated to Go) | Linux only |
| QuickJS in WebAssembly on wazero | iOS forbids generated code, so wazero interprets an interpreter there |
| The system's (JavaScriptCore) | A bridge per platform; Android's runs only out of process |

What goja lacks, and the answer:

- **ES modules, async generators, `for await`, `import()`**: esbuild lowers them; imports of NSGo's modules become `require` of the modules the runtime makes.
- **Bytecode**: none to save, so bundles carry source, compiled as they load.
- **A heap limit**: JS allocates from Go's heap. Time (`Runtime.Interrupt`), stack (`SetMaxCallStackSize`, 10 000 calls in every app so a runaway recursion throws rather than ending the process) and NSH's quotas are enforceable; memory is not ([sandbox](#sandbox)).
- **A turn after the effects**: goja runs promise continuations as the outermost JS call returns, which may be an effect's, so they run untracked (`AsyncContextTracker`), as QuickJS's run after the effect in NSH.
- **`Intl` and locale methods**: goja has none (its `toLocaleString` reads a radix), so `intl.go` gives `Intl.NumberFormat`, `PluralRules` and `Collator` CLDR's data through `golang.org/x/text`, dates in English, and patches `Number`, `Date` and `String`'s locale methods.
- **Speed**: slower than QuickJS on pure JS, so NSH's first performance rule matters more: only the app's code is JS.

### The page's engine

Each `js.Runtime` on the web is a realm of its own, a hidden iframe of the page's origin, removed with the app: a bundle runs in it as a script, and what the runtime installs (`require`, `nsgo:rt`, the web APIs, timers on the app's clock, `Date` reading it) is its globals, as on goja. The iframe is no sandbox: the app has the page's authority.

- A release's page makes that iframe and loads into it, before the app starts, the scripts its realm runs: `tsx/websrc`'s sources (the helpers, `web.js`, the props helpers, `Date`'s patch and the sandbox's hardening), which the tool writes as `nsgo-runtime.<hash>.js`, and the app's bundle and chunks, which it writes beside them. Each registers itself on the realm's `__nsgoPrograms` or `__nsgoScripts`, and the runtime calls what it finds there, so nothing is compiled from a string and a `Content-Security-Policy` needs no `'unsafe-eval'`. A build that ships none (a dev build, a test) has the runtime make its own iframe and compile the sources, which such a policy forbids.

- Primitives are copied into Go and objects are handles; Go functions are JS functions calling back through one `syscall/js` function by ID, Go values objects whose prototype holds their type's methods and fields, both let go of when JS collects them (`FinalizationRegistry`). Values export as goja's do (integers as `int64`, arrays as `[]any`), so the runtime reads them alike.
- What JS throws comes back as an `*Exception`, its stack read through the bundle's source map as goja reads it; a JS error a Go function panics with is thrown in JS; any other Go panic unwinds JS and goes on in the Go code that called it, even if JS caught it.
- Promise reactions and the browser's own callbacks run once Go gives the page its turn, after the engine's commit, not as the outermost call returns. What they call of Go runs under the app's root, untracked, and a commit follows (`engine_browser.go`). `nsgotest`'s headless runs give the page its turn as they settle on the web.
- The page's `Intl` is the browser's; `intl.go` is goja's alone.
- The page's engine cannot be interrupted, so `Sandboxed`'s time budgets do not hold on the web; its stack depth is the browser's, and its hardening applies.

## Compiling

`tsx/bundler` compiles with esbuild's Go API, so no Node is needed, and imports no runtime, so the tool that links it builds again only when `core` or the bundler changes:

- Each module goes through esbuild's transform: TypeScript stripped, syntax lowered to ES2020 and what goja lacks, JSX in its classic form (`__jsx(tag, props, ...children)`, with React's whitespace rules), as CommonJS, and `import.meta.url` its file's `file:` URL.
- `jsx.go`, a port of `jsx.rs`, reads that with goja's parser and writes each call as NSH's compiler does: host elements made in place (`element`, `prop`, `bindProp`, `append`, `insert`), components called with getters for what changes, `If` and `Each` as `cond` and `each`. Its source map is composed with esbuild's.
- Tags and refs resolve through the module's scopes (`scope.go`), TypeScript's `declare`d names among them, as Oxc's scopes resolve them for NSH.
- esbuild bundles the modules from an `fs.FS` into one script with its source map inline. A bare import is a module the runtime serves (NSGo's, or one an app registered) and stays a `require`; an app imports its own files by relative paths, a directory's (`".."`) its index, and JSON.
- An imported image is an `ImageAsset` (`{ url, size }`): its size in points read as it compiles (an `@2x` file at half its pixels), its bytes inlined as a `data:` URL the runtime keeps under the `url` it answers with, `asset:/` and its path, so it ships on every target, and images and `fetch` read it by that URL. Density variants beside it (`logo@2x.png`) add their `scales` and `variants`, `data:` URLs too, of which the core mounts the one for the screen's pixel ratio (`ImageSource.Variants`). An imported font is its family name, its bytes inlined and registered as its module runs, before the app mounts (`Scope.RegisterFont`). A WOFF or WOFF2 file is unpacked as it compiles (`internal/woff`, which only the bundler links), so no text engine meets one, though a font's bytes then ship uncompressed in the bundle; NSH unpacks them as it registers them, which would link Brotli into every app. WOFF2 collections are refused.
- `nsgo.yaml` beside the entry names the app's `theme` ([themes](themes.md#theme-files)) and extends Tailwind's under `tailwind` ([themes](themes.md#tailwind)); the bundle's first line hands both to the runtime (`nsgo:rt`'s `setup`). The tool reads both from the app's directory; compiled as it starts, the app embeds them too.
- NSH's compiler tests are its tests (`jsx_test.go`), with its errors: lowercase tags, unknown tags with the import to write, text outside `<Text>`.

### Split bundles

`bundler.CompileSplit` (`split.go`, NSH's `split.rs`) makes each app module an `import()` names a chunk of its own, with the modules it imports that the entry's script does not, and the tool splits a web release so: its page loads a chunk's script as the import first runs, and every other package holds the whole app, `import()` resolving at once. goja's parser reads no `import()`, so the transform writes each as `_$load(spec, key)`, the same length, so the source map holds; a module's key is esbuild's name for it (`app:screen.tsx`). The modules of the entry's script and the chunks are defined once in the VM by key (`__nsgo_split`), so a chunk shares the entry's modules and their state, and a chunk imports them from the VM rather than holding them again. `tsx.SplitApp` takes the function that loads a chunk's script, which `compiler.App` loads into the app's realm from its URL relative to the page (`compiler.BundledChunks`).

`compiler.App` runs the bundle the `nsgo` tool compiled ahead when the build has one ([tool](tool.md#tsx-apps)), else compiles the embedded sources as it starts. The tool's builds take the `nsgo_bundled` tag, under which the compiler's esbuild part is left out, so packages ship no compiler.

## The runtime

`tsx.App(name, bundle)` returns the app function `nsgo.Run` and `nsgotest.Start` take: it runs the bundle in a goja VM on the engine goroutine, and what `mount` renders is the app. JS reaches the graph through the root scope; `core` gives what JS creates to the innermost running owner, as NSH's graph does.

- A node is a JS object over a `core.Element`, with YNode's methods (`focus`, `scrollTo`, `scrollToIndex`, `measure`, the web view's and the video's) as Go methods the VM names in lower camel case; each reaches its view through a `core.Ref`. `measure` resolves after the turn's commit with `Ref.Frame` and `Ref.Bounds`.
- Host elements are made detached, their props set one at a time. Dynamic children are a render effect reconciling the nodes it inserted with Solid's `reconcileArrays`, as `dom.rs` does. Props decode into `core`'s element structs: an NSH prop sets the field of the same meaning; a prop `core` lacks is added to `core`, so Go apps gain it too; one not decoded is logged once. A prop set to `undefined` makes the element's props again from the others.
- A failure of what JS calls is an `Error`, or the built-in error a `tsx.JSError` names (`TypeError`, `RangeError`), with a stable `code` (`core.ErrorCode`, NSH's `codes`): `core.Error`'s, the storage quota's, a host's and I/O's, with `permission` and `resource` where a capability refused it.
- Style objects decode every NSH style key into `core.Style`: colors through `core.ParseColor` (`oklch()` and `color-mix()` included), shadows, gradients, filters and positions through `core`'s CSS parsers, `env()` and `calc(env() + …)` in paddings and margins, transform steps, transitions and keyframes. An animated node in `opacity`, a color or a transform step binds `Style.Animated`. A number that is not finite leaves its key unset.
- `ios`, `android`, `desktop` and `web` props set their keys where they apply (the web takes `desktop`'s too in the desktop idiom), clearing the keys an object no longer has.
- `core.Text` takes spans, so the runtime merges a `<Text>`'s raw text and nested `<Text>`s into its spans and makes other elements attachments.
- A node is released with the owner it was made under. NSH releases one when QuickJS collects it; Go's collector cannot see that only the tree's handler closures still hold it.
- An exception is a panic `core` recovers: an `ErrorBoundary` above shows its fallback, else the host reports it with the stack mapped to `.tsx` lines.
- goja runs promise jobs as the outermost call returns, so what Go settles later (an animation's end, a host's answer, `fetch`) resolves inside a call into JS (`inJS`). `setTimeout`, `setInterval`, `requestAnimationFrame` and `queueMicrotask` run on the engine goroutine through `Scope.AfterFunc` and `RequestAnimationFrame`.

### Go components

The primitives that are not `core.Element`s (`Overlay`, `WebView`, `Video`, `List`, `Canvas`, `Dialog`, `InputAccessoryView`) and every Go component are faces (`face.go`): a function making the Go value of a node's props, run in a `core.Dynamic`. What it reads of the props as it makes the value is tracked, so a change makes the component again; what it hands the component as a function (`StyleFunc`, `Open`) the component follows itself, and handlers look the current function up as they run. A host tag's props are signals set one at a time and its component is made as the node is first inserted; a component's are the props object compiled JSX passes, made as it is inserted too, so it knows the components around it. Its children and slots are regions made as `core` renders them (`lazy`), under the Go component's scope, so its context reaches them, as JSX children are made where they show.

`tsx.Register(module, components, options...)` makes Go components importable: each exported field is a prop, read by its type (`goface.go`):

- `func() T` follows the prop as it changes; a plain `T` is read as the component is made. `X` beside `XFunc` is one prop, `XFunc` following it.
- `func(...)` without results is an event, its arguments given to the handler as NSH's objects; `func(...) core.Node`, with or without a leading `*core.Scope`, renders what the prop's function returns, its getters read as the region renders.
- `core.Node` is a slot, `[]core.Node` named `Children` the children, `Class` and `Style` the element's classes and its style with the state styles, `XClass` and `XStyle` a part's, `Ref` a `*core.Ref` or the component's own ref, whose methods the `ref` prop gets, as NSH's: with the component's `node`, and what a method returns in a promise.
- Props are the fields in lower camel case; `Label` is `accessibilityLabel`, `Hint` `accessibilityHint`, `XLabel` `xAccessibilityLabel`, and a bool `NoX` the prop `x` negated, true unless set. `Rename` gives another name, and `Flag` a choice of names a bool field reads (`DiffView`'s `layout`, `Unified`). `Omit` leaves a field out, and `Preset` leaves it out set to a value, as `TextView`'s typography and the `Editor`'s look of diagnostics are NSH's in TSX.
- `Enum` names an integer type's values; `Union` names the members of an interface type by the type tag their objects carry (`{ type: "paragraph" }`); slices are arrays, `*regexp.Regexp` a pattern, and structs data objects whose keys are their fields in lower camel case. `Fields` names a struct's fields NSH names otherwise, and `Shape` gives a type a JS shape of its own both ways (an editor's context-menu event's `x` and `y`, a sort's `direction`, a dock layout's nodes typed `split` and `tabs`).

`tsx.RegisterFunctions(module, funcs)` makes Go functions importable the same way, as an embedding app's UI set exports its functions: arguments convert to their parameters, a non-nil error result throws. `tsx.RegisterStatics` hangs functions on a registered component, as NSH's `TextView.parse`.

`tsx.RegisterClass[T](module, name, tsx.Class[T]{New, Getters, Statics})` makes `*T` a class, NSH's `#[class]` (`rust-classes.md`): `New` is the constructor, `*T`'s exported methods its methods in lower camel case, `Getters` the methods read as properties with `SetX` as `X`'s setter, and `Statics` functions on the class. A JS object holds its `*T`, one object per instance while it lives (a table of weak pointers), and `dispose()` or `[Symbol.dispose]` lets it go, calling `Close` when `T` has one; the object throws after. A parameter `*T` takes an instance, a `TypeError` naming the member and the parameter otherwise, and a result `*T` gives one. Calls of classes and functions share one path (`callGo`), as NSH's `call.rs`:

- One whose Go function takes a `context.Context` first is async: its arguments convert in the JS call, it runs on a goroutine of its own and JS gets a promise, settled on the engine goroutine; the context ends with the app. Every other call runs in the JS call, as NSH's synchronous members do. A trailing error, or a panic, throws or rejects with an `Error`.
- A JS function a synchronous call gets runs in place when Go calls it. One an async call gets runs on the engine goroutine: a func without results is posted, NSH's `Callback::post`, and one with results waits for them, and for a promise the function returns to settle, NSH's `call_async`.
- `.nsgo/types` declares each class with its members, `Promise<T>` for an async call, and parameters as `argN`, since Go keeps no parameter names. `nativeModule(specifier)` gives a module's exports without an import, NSGo's own or a registered one, the object an import gives, and a `ReferenceError` where none is.

Every event passes one object, as NSH's do (`api-naming.md` (Payloads)). `tsx.Event` names its fields, as NSH's `#[args]` (`Event("OnOpenChange", "open", "reason")`). Without one, an event struct goes as it is, `Bounds` as `{ bounds }` and another value as `{ value }`; several arguments need the names, and `Register` panics without them, since Go keeps no parameter names.

`nsgo/base` registers `base`'s components this way (`base.go`), with NSH's names where `base`'s differ and a few props shaped otherwise (`DataTable`'s `data`, `nsgo/base/plot`'s `data` and what its marks read of it (`plot.go`), `DockArea`'s `panels`, a list by `id`, and `renderPanel`'s `{ id }`). `List` is written out (`baselist.go`), each's items as `<List>`'s and its choices the props `selected` or `defaultSelected`, `onSelectedChange` and `multiple`, since JS has no `Selection`. A component's host props, NSH's `ComponentHostProps`, are its fields: `class`, `style`, `testID`, and `onLayout` and `tooltip` on each component whose root is a view of its own, which take a `core.View`'s. `ResizablePanel` is a part: its call is a value its group collects from its children. The navigation (`createNavStack`, `NavStack`, `NavPage`, on `base.Nav`), `Dialog`, `AlertDialog`, `ToastManager` and `ToastStack` are written out. `TextView`, `Editor`, `DiffView`, `RichEditor` and `Terminal` link packages other apps should not, so `tsx/textview`, `tsx/editor`, `tsx/diff`, `tsx/richeditor` and `tsx/terminal` register them for an app that imports them; `Terminal` starts its session from its props (`command`, `conn`, `cursor`), a change of them starting another, while its view's (`theme`, `fontSize`, the key bar's) follow, and its ref copies, pastes and reads the title and directory; `tsx/richeditor` adds the rich editor's sessions, commands and extensions ([rich editor](richeditor.md#tsx)), and `tsx/ui/richeditor` its styled form in `nsgo/ui`.

### Styled components

`tsx/ui` gives TSX `ui`'s components as `nsgo/ui`, so a TSX app looks as a Go app does with no second implementation: an app imports it for its effect; a theme of its own needs `ui`'s roles, as `core.DefaultTheme` and `ui.WarmDuskTheme` have. Each takes its Go fields as props, by `tsx.Register`; generic components take string values, and `Input` and `Field`, defined on `base.Input`, take its fields. What Go holds in Go values TSX holds in props: a `List`'s and a `Tree`'s selection are lists of keys, a `Tree`'s items are data, and the ref of `Toasts` shows them. `TextView` and `DiffView` here are `ui`'s forms, which take `Options` from `tsx/textview` and `tsx/diff`. `tsx/ui/chat` gives `ui/chat` as `nsgo/ui/chat`, with `ConversationState` a class its transcript, turns and composer share (`new ConversationState()`: a Go constructor or function taking a leading `*core.Scope` gets the app's).

An icon prop takes a name `ui.Icons` gave or an SVG document. An app lists the Lucide icons it uses, as `lucide` keeps no table of them, so the others stay out of its binary. `tsx.Convert` gives a Go type a JS form that need not be an object, as these icons and dates are: `base.DateValue` is `DatePickerValue`'s strings and `base.TimeOfDay` a string, in data objects and events too. In a data object a `func() T` takes a plain `T`.

### Modules

- `nsgo`: the reactive API, `mount`, `context`, `split`, `ErrorBoundary`, `resource`, the primitives (the shader elements among them, whose WGSL `core.CompileWGSL` compiles: `shader.go`; a page's and a dialog's `transitionShader` too), the environment's accessors, `token`, animation (`createAnimatedValue`, `AnimatedNode`, `AnimatedValue`, `Animated`, `Easing`, `LayoutAnimation`, `createMotion`, `motions`, on `core`'s animated nodes and animations), `PanResponder` and the responder props (on `core.Responder`, whose `OnTerminationRequest` asks), Canvas 2D (recording into the `core.Drawing` its `Draw` gets, animated arguments bound), `Keyboard`, `keymap`, `Clipboard` (whose `readImage` rejects an image it cannot read), `Storage`, `Picker` (whose `takePhoto` is `core.TakePhoto`'s), `Speech` (on `core.StartSpeech`), `Notifications` (on core's notifications), `openUrl`, `announce`, `setAccessibilityFocus`, `nativeComponent` and `host`.
- `host` reaches the embedding app through the [host channel](embedding.md#the-host-channel): `call` (a promise of the handler's answer), `post`, `on` and `params`.
- The web APIs (`web.js` over Go's `net/http`, UTF-8, base64 and URL parsing): `fetch`, `Headers`, `Request`, `Response`, `URL`, `URLSearchParams`, `TextEncoder`, `TextDecoder`, `AbortController`, `Event`, `EventTarget`, `Blob`, `File`, `FormData`, readable, writable and transform streams, `structuredClone`, `atob`, `btoa` and `performance`. `fetch` reads `http`, `https`, `data:` and `file:` URLs.
- `nsgo/mobile` and `nsgo/desktop` are importable everywhere; where they do not apply their calls do nothing and their components render nothing.

## Core's part

`core.Element` is nsh-core's `Op` entry on `core`'s tree, the part of `core` only `tsx` uses:

- `NewElement` makes one detached from an element value (`View`, `Text`, `Scroll`, `Input`, `Image`), its props, whose functions it reads; `Set` replaces them, `Insert` and `Remove` place children, and `Release` frees it.
- An element is a `Node`: mounted where Go code puts a node, it shows there, so Go components take TSX children.
- A region element (`NewElement` with no value) lays its children out as its parent's, and `Mount` shows Go nodes in it, so TSX shows Go components and `ErrorBoundary` is a `Catch`.
- `Scope.RenderEffect` runs bindings as `core`'s own do, and `Scope.Run` runs code under an owner, as Solid's `runWithOwner`.

## Types and checking

`tsx.Declarations` gives the TypeScript declarations of the modules TSX imports: NSGo's own, written in `tsx/types/`, and every module registered, declared from the components' and functions' types as their faces read them, under NSH's names: a ref after its component (`EditorRef`), the types of `editor`, `diff` and `textview` after theirs (`EditorPosition`, `DiffHunk`), enums and named string types as unions and aliases. A struct's numbers and structs are required and its other fields optional, but for the events and requests Go hands JS, whose fields are all there; `Required` and `Optional` mark the fields that differ. `assets.d.ts` declares asset imports, and `tokens.d.ts` the names `token` reads, from the app's theme. NSH's gallery checks against them with only the errors it has against NSH's own. `nsgo dev` and `nsgo check` write them to `.nsgo/types` with a `tsconfig.json` an app's extends, and `check` then runs TypeScript 7's checker, built from its Go sources with `go run` and cached ([tool](tool.md#checking)). The web APIs' types are TypeScript's DOM library's.

## Developing

Under `nsgo dev` the tool compiles TSX ahead for hot updates (`bundler.CompileHot`), and a save that changes only the bundle's files compiles it again once they stop changing, reading again a file its writer still holds, as Windows refuses, and sends it over `/_nsgo/events`, with no Go build, on every target. `compiler.App`, built with the dev client, follows the server and runs the new bundle in the same VM (`tsx.Hot`). The hot transform and registry (`hot.go`) port NSH's (`tooling.md`, "Hot module replacement"), and `hot_test.go` ports `nsh-engine/tests/engine/hot.rs`:

- A hot bundle's modules go through a registry, which esbuild's module wrapper calls: a newer bundle runs again the modules whose code changed, and the importers of those that do not take their own updates, up to the entry. A module takes its own when all it exports are components. Importers keep a facade of a module's current exports.
- Top-level components, capitalized functions whose code has JSX, render through proxies that stay the same across updates, each a memo of its component's newest code. Its signature is its code and its module's code outside components, so an edit renders again the instances whose signature changed, and those of the modules run again because a module they import changed; their own state starts afresh, everything else keeps its.
- Top-level `context()` results keep their identity, taking the new default (`keepContext`), and top-level `const`s made by `signal()` or `createNavStack()` keep their value while their initializer's code is the same. A `mount` call made again with the same component and no props keeps what it mounted; from the first that differs, the module's earlier mounts are undone and made anew.
- A component that throws reports the error (`reportError`) and renders nothing until the fix; a failed compile shows over the running app.

## Sandbox

Limits and checks are Go's, never JS's, since a bundle holds only the app's code. Go code, an app's and its components', is trusted native code; only TSX modules are sandboxed, signed and held to capabilities (decided by the user, 2026-11-08). `tsx.Sandboxed(name, bundle, sandbox)` runs a bundle within a `tsx.Sandbox` (`sandbox.go`), which the Go code running it sets; `tsx.View` runs a package so.

- **Limits** (`tsx.Limits`, `DefaultLimits` NSH's where NSH has them): a time budget for loading and for each call into JS (10 and 2 seconds), cut with `Runtime.Interrupt` as `ErrBudget`, after which the VM goes on; a stack depth; hardening, which deletes `eval`, has the `Function` constructors throw and freezes the built-in prototypes; and quotas. Elements past `Nodes` (200 000) stop the VM: it is interrupted with `ERR_NSH_QUOTA_EXCEEDED`, which no `catch` stops, panics into the nearest `core.Catch`, and runs no JS after. Waiting timers past `Timers`, fetches past `Requests`, a body past `Body` and storage past `Storage` fail the call with the same code. `tsx.ParseLimits` reads them as `nsgo dev -limits` writes them.
- **Capabilities** (`pkg.Capabilities`, [signed packages](#signed-packages)): what a package does not declare is refused, and `Sandbox.Grant` narrows what it declares to what the host gives. Core holds them (`core.Access`, `Scope.ProvideAccess`, `core/access.go`) where requests are made, whoever makes them, so a Go component showing a package's data is held too: images, videos and canvas and shader images from the network need `network`, a file on the device is refused; a web view's source needs `webview`, and the platform refuses top-level navigations to other hosts as they start (`WebConfig.Navigable`: WebKit's navigation delegate on Apple platforms, WebView2's `NavigationStarting`, WebKitGTK's `decide-policy`, Android's `shouldOverrideUrlLoading`), with no downloads; `SendInput` needs `webview`'s `input`; `OpenURL`, `statusBar` and host calls, by handler name. A refused source shows nothing and reports `OnError`, once; a refused service answers as the system would with nothing there and logs once. The runtime checks what only it makes: `fetch`, every redirect hop included, which refuses `file:` URLs; `Clipboard`, `Picker`, `Speech.start` and `Notifications`, which `notifications` grants. Its calls fail with `ERR_ACCESS_DENIED`, `permission` and `resource`; `fetch` with a `TypeError` carrying them.
- **Imports.** A sandboxed bundle imports `SandboxModules`, NSGo's own, with the Go components registered into them; not the Go functions `RegisterFunctions` registers, components registered `Unsandboxed()` (`Terminal`, which starts programs), other modules or the embedding app's native components, unless `Sandbox.Modules` names them (`myapp`, `nsgo/base.Terminal`, `native:MapView`, `*`).
- **Host and storage.** `Sandbox.Host` is the only host its `host` module reaches (`Scope.ProvideHost`), so the Go app answers a plugin's calls through a `core.Host` of the plugin's own and none of the app's handlers; a package's storage is a partition of the app's under its app ID (`Scope.ProvideStorage`), which the app's own `Storage` neither lists nor clears.
- **Plugins of functions.** `tsx.Load` runs a package as `View` does, without showing it, and `Plugin.Call` calls what its entry module exports, the bundle's `__nsgo_exports` (`bundler.Compile`'s global name), resolving promises; arguments and results cross as JSON values.

Without a heap limit a module can still exhaust memory: the quotas bound what Go allocates for it, not what its JS does. That suits the app's own screens and packages its root signs; a store of untrusted modules needs a heap limit, which waits on the [roadmap](../roadmap.md#tsx-and-dynamic-delivery) while goja stays the engine, with no other engine taken in for it (decided by the user, 2026-11-09).

## Signed packages

`tsx/pkg` ports `nsh-package` on `crypto/ed25519` and `crypto/sha256` alone, so the `nsgo` tool links it; its package documentation holds the format, the trust model and how the format evolves. It is for code an app downloads: its own screens delivered anew, or plugins. Where packages come from, an update server or a store, is the app's; `examples/plugins` asks the user to grant a package's capabilities and installs it whole.

- **Keys.** An offline root certifies release keys for app IDs (`*`, `com.example.*` or an ID) and a time; packages carry their release key's certificate, so release keys rotate without app updates. Signatures are over a context per kind, a zero byte and the message's SHA-256.
- **Packages** (`.nsgopkg`): a JSON header (the app's ID, name, version and build, the minimum runtime, the signing time, the certificate, the capabilities, a file table with SHA-256s, `critical`), the files, and the release key's signature of all before it. A package holds one file, the bundle `bundler.Compile` writes, whose assets and source map are inline, so nothing is extracted. Fields and file kinds a runtime does not know are left unused and listed in `Info.Ignored`, and one `critical` names makes it refuse the package.
- **Capabilities** are `nsgo.yaml`'s `capabilities:`, NSH's keys in snake case (`network` with `hosts` and `http` rules, `webview` with `hosts` and `input`, `open_url`, `clipboard`, `picker`, `speech`, `status_bar`, `host`), read strictly by the tool and leniently by runtimes (`ParseCapabilities`): an unknown capability is not granted and an unknown key inside one drops it, so what a runtime does not know never widens access. The signer names `capabilities` in `critical`, so a runtime that does not know them refuses the package rather than granting all. A package without them is granted none.
- **Runtime versions.** `pkg.RuntimeVersion` is 0.1.0 until NSGo's first release. A package names the oldest runtime it runs on, by default the signing tool's. A package runs on every later runtime, so once TSX ships nothing its modules expose is removed or renamed, as NSH's `compatibility.md` rules.
- **Verifying.** `Trust.Verify` checks the certificate against the trusted roots, the release key and the package against the revocation list, the signature, the certificate covering the app and the signing time, never the current time, so a package outlives its certificate, the minimum runtime, the file table tiling the data and each hash, and the build against the highest accepted. `Accept` raises that floor once the host saw the package run, checking the revocations again. Revocation lists replace only older ones by sequence number. `OpenTrust(dir)` keeps both in `trust.json`, written whole and renamed, and on the web in the page's `localStorage` under a key of `dir`'s.
- **Running.** `tsx.View{Package: p}` runs a verified package where it is placed in a Go tree, in its [sandbox](#sandbox), so a host shows plugins beside its own views; a package that throws as it loads, or stops, panics into the nearest `core.Catch`. Verifying hashes the whole package, so hosts call it off the engine's goroutine. `examples/plugins` does all of this, with tests on every target.
- **Tooling.** `nsgo keygen`, `certify`, `publish`, `verify` and `revoke` ([tool](tool.md#signed-packages)).

## Store policies

- Apple's guideline 2.5.2 forbids downloaded code that changes an app's features, while its developer agreement allows interpreted code that keeps the app's purpose.
- Apple's guideline 4.7 allows JavaScript mini apps under its rules: an index of them, and no native API exposed to them without Apple's permission.
- Google Play forbids downloaded executable code, except code run in a virtual machine or interpreter with indirect access to Android's APIs.
- Updating an app's own screens fits both stores. A store of third-party modules owes 4.7's rules, and capabilities keep its modules from what the host did not grant.

## Where this departs from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| QuickJS | goja | No C and no cgo, on every target |
| The web's app as ES modules from blob URLs, with the browser's `fetch`, streams and URLs | The bundle as a script, with `web.js`'s APIs over Go's, as on goja | esbuild already links the app into one script, which then starts as it loads; `asset:` URLs, the test network and `WithFetcher` answer the same on every target |
| Oxc, `jsx.rs`, rolldown | esbuild and a port of `jsx.rs` on goja's parser, over CommonJS | Go libraries, no Node; goja's parser reads no ES modules |
| `nsh build` writes the bundle | The tool compiles ahead into the package through `-overlay`; `go run` and `go test` compile at start | Plain Go commands keep working; packages ship no compiler |
| Bytecode in packages | Source only | goja saves no bytecode |
| Assets in packages, extracted to a directory per package | One bundle with its assets inline, nothing extracted | The bundler already inlines assets; a package needs no files on disk |
| Swift and Kotlin trust APIs (`NshTrust`, `openPackage`) | `tsx/pkg` and `tsx.View` in the Go app on every target | An NSGo app is Go on iOS and Android too, embedded or not ([embedding](embedding.md)) |
| A heap limit | Time, stack and quotas | goja allocates from Go's heap |
| The clipboard, the picker and speech checked in the core too | In the runtime's modules only | A Go component's copy, paste, pickers and microphone act on the user's own input |
| Every module of the runtime | `SandboxModules`, others as `Sandbox.Modules` names them | Registered Go functions and components such as `Terminal` reach past the capabilities |
| A plugin's storage as the host decides | A partition of the app's under its app ID | One store, quota and backend on every target, the web's `localStorage` included |
| Top-level navigations refused on every platform | Not on the web | An `<iframe>` tells the page nothing of a cross-origin navigation before it happens |
| Nodes released when collected | Released with their owner | Go's collector cannot tell the tree's handler closures from the app's references |
| Rust components' JS face generated from specs | Go components read by reflection as they are made, declarations from their types | One registration call, no generator step; making a component is not a hot path |
| `#[class]`, `#[getter]`, `async fn`, `Callback<A, R>` | `RegisterClass` with getters by name, a leading `context.Context`, plain Go funcs | Go has no attributes or `async`; a context is how Go code that waits says so |
| Shared chunks of modules several chunks import | Each chunk holds the modules it imports beyond the entry's, defined once by key | esbuild splits only ES modules, which goja does not run; a module two chunks import costs bytes, not a second instance |
| Development bundles of one chunk per module, patched | The whole hot bundle again, its unchanged modules not run | A bundle compiles in milliseconds; only the modules' code is compared |
| Web APIs in Rust | Thin JS classes over Go natives (`web.js`) | Little code; only `fetch`, encoding and URLs need the system |
| `Intl` for every CLDR locale | Numbers, plurals and collation by CLDR; dates in English | `golang.org/x/text` has no date formats |
| `nsh.json` | `nsgo.yaml` | Configuration is YAML |
| `nsh check` downloads `tsc` from npm | `go run` builds it | Only Go is assumed |
| The modules `nsh`, `nsh/*`, `nsh:rt` | `nsgo`, `nsgo/*`, `nsgo:rt` | The product's name (decided by the user, 2026-10-28) |
