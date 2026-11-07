# TSX and dynamic delivery

NSGo runs TSX apps on NSH's design: they import `nsgo`, `nsgo/base`, `nsgo/mobile`, `nsgo/desktop` and `nsgo/test`, have NSH's props, events, style keys, Tailwind classes and behavior, and compile with `tsx/bundler`. Each part began as a Go port of its NSH counterpart. From 2026-11-08 TSX grows its own way (decided by the user): nothing checks it against NSH's apps, and an NSH app may need more than renamed imports. The [roadmap](../roadmap.md#tsx-and-dynamic-delivery) orders what is left.

An app embeds its sources and names its entry; the `nsgo` tool compiles it ahead for its builds, and `go run` and `go test` compile it as it starts:

```go
//go:embed *.tsx *.ts
var src embed.FS

func main() { nsgo.Run(compiler.App(src, "App.tsx")) }
```

`examples/tsx` is such an app: its modules, components, signals, inputs, lists and timers, styled with classes in light and dark, one column on phones and two from `md:`, a Go class and an async Go function of its own, and a timer module it loads with `import()`, with Go and TSX tests.

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
| `nsh-js` `web/` | `tsx/web.js`, `web.go`, `intl.js`, `intl.go` | Web APIs and `Intl` |
| `nsh-js` `sandbox.rs` | `tsx/sandbox.go` | Time budgets, stack depth, hardening |
| `nsh-js` `declarations.rs` | `tsx/declarations.go`, `tsx/types/*.d.ts` | `.nsgo/types` |
| `nsh-testing`'s `nsh/test` | `tsx/tsxtest` | `*.test.tsx` as Go tests |
| `nsh-core`'s `Op` | `core.Element` | Elements built one change at a time |
| `nsh-package` | — | Signed packages ([open question](../../ARCHITECTURE.md#open-questions)) |

Read the NSH file before changing its port, and keep the port's names and order where Go allows, so NSH's next change maps onto it.

## The engine

goja runs the JS: pure Go, so it builds for every target, `js/wasm` included. The alternatives fail a [principle](../../AGENTS.md#principles) or a target:

| Engine | Verdict |
| --- | --- |
| QuickJS through cgo, NSH's | cgo and C on every target |
| `modernc.org/quickjs` (QuickJS translated to Go) | Linux only |
| QuickJS in WebAssembly on wazero | iOS forbids generated code, so wazero interprets an interpreter there |
| The system's (JavaScriptCore, the browser's) | A bridge per platform; Android's runs only out of process |

What goja lacks, and the answer:

- **ES modules, async generators, `for await`, `import()`**: esbuild lowers them; imports of NSGo's modules become `require` of the modules the runtime makes.
- **Bytecode**: none to save, so bundles carry source, compiled as they load.
- **A heap limit**: JS allocates from Go's heap. Time (`Runtime.Interrupt`), stack (`SetMaxCallStackSize`, 10 000 calls in every app so a runaway recursion throws rather than ending the process) and NSH's quotas are enforceable; memory is not ([sandbox](#sandbox)).
- **A turn after the effects**: goja runs promise continuations as the outermost JS call returns, which may be an effect's, so they run untracked (`AsyncContextTracker`), as QuickJS's run after the effect in NSH.
- **`Intl` and locale methods**: goja has none (its `toLocaleString` reads a radix), so `intl.go` gives `Intl.NumberFormat`, `PluralRules` and `Collator` CLDR's data through `golang.org/x/text`, dates in English, and patches `Number`, `Date` and `String`'s locale methods.
- **Speed**: slower than QuickJS on pure JS, so NSH's first performance rule matters more: only the app's code is JS.

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

`bundler.CompileSplit` (`split.go`, NSH's `split.rs`) makes each app module an `import()` names a chunk of its own, with the modules it imports that the entry's script does not, and the tool splits a web release so: its page fetches a chunk as the import first runs, and every other package holds the whole app, `import()` resolving at once. goja's parser reads no `import()`, so the transform writes each as `_$load(spec, key)`, the same length, so the source map holds; a module's key is esbuild's name for it (`app:screen.tsx`). The modules of the entry's script and the chunks are defined once in the VM by key (`__nsgo_split`), so a chunk shares the entry's modules and their state, and a chunk imports them from the VM rather than holding them again. `tsx.SplitApp` takes the function that loads a chunk's script, which `compiler.App` fetches relative to the page (`compiler.BundledChunks`).

`compiler.App` runs the bundle the `nsgo` tool compiled ahead when the build has one ([tool](tool.md#tsx-apps)), else compiles the embedded sources as it starts. The tool's builds take the `nsgo_bundled` tag, under which the compiler's esbuild part is left out, so packages ship no compiler.

## The runtime

`tsx.App(name, bundle)` returns the app function `nsgo.Run` and `nsgotest.Start` take: it runs the bundle in a goja VM on the engine goroutine, and what `mount` renders is the app. JS reaches the graph through the root scope; `core` gives what JS creates to the innermost running owner, as NSH's graph does.

- A node is a JS object over a `core.Element`, with YNode's methods (`focus`, `scrollTo`, `scrollToIndex`, `measure`, the web view's and the video's) as Go methods the VM names in lower camel case; each reaches its view through a `core.Ref`. `measure` resolves after the turn's commit with `Ref.Frame` and `Ref.Bounds`.
- Host elements are made detached, their props set one at a time. Dynamic children are a render effect reconciling the nodes it inserted with Solid's `reconcileArrays`, as `dom.rs` does. Props decode into `core`'s element structs: an NSH prop sets the field of the same meaning; a prop `core` lacks is added to `core`, so Go apps gain it too; one not decoded is logged once. A prop set to `undefined` makes the element's props again from the others.
- A failure of what JS calls is an `Error` with a stable `code` (`core.ErrorCode`, NSH's `codes`): `core.Error`'s, the storage quota's, a host's and I/O's, with `permission` and `resource` where a capability refused it.
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

`nsgo/base` registers `base`'s components this way (`base.go`), with NSH's names where `base`'s differ and a few props shaped otherwise (`DataTable`'s `data`, `Plot`'s `data`, `x` and `y`, `DockArea`'s `panels`, a list by `id`, and `renderPanel`'s `{ id }`). `List` is written out (`baselist.go`), each's items as `<List>`'s and its choices the props `selected` or `defaultSelected`, `onSelectedChange` and `multiple`, since JS has no `Selection`. A component's host props, NSH's `ComponentHostProps`, are its fields: `class`, `style`, `testID`, and `onLayout` and `tooltip` on each component whose root is a view of its own, which take a `core.View`'s. `ResizablePanel` is a part: its call is a value its group collects from its children. The navigation (`createNavStack`, `NavStack`, `NavPage`, on `base.Nav`), `Dialog`, `AlertDialog`, `ToastManager` and `ToastStack` are written out. `TextView`, `Editor` and `DiffView` link packages other apps should not, so `tsx/textview`, `tsx/editor` and `tsx/diff` register them for an app that imports them.

### Modules

- `nsgo`: the reactive API, `mount`, `context`, `split`, `ErrorBoundary`, `resource`, the primitives, the environment's accessors, `token`, animation (`createAnimatedValue`, `AnimatedNode`, `AnimatedValue`, `Animated`, `Easing`, `LayoutAnimation`, `createMotion`, `motions`, on `core`'s animated nodes and animations), `PanResponder` and the responder props (on `core.Responder`, whose `OnTerminationRequest` asks), Canvas 2D (recording into the `core.Drawing` its `Draw` gets, animated arguments bound), `Keyboard`, `keymap`, `Clipboard`, `Storage`, `Picker`, `openUrl`, `announce`, `setAccessibilityFocus`, `nativeComponent` and `host`.
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

`tsx.Declarations` gives the TypeScript declarations of the modules TSX imports: NSGo's own, written in `tsx/types/`, and every module registered, declared from the components' and functions' types as their faces read them, under NSH's names: a ref after its component (`EditorRef`), the types of `editor`, `diff` and `textview` after theirs (`EditorPosition`, `DiffHunk`), enums and named string types as unions and aliases. A struct's numbers and structs are required and its other fields optional, but for the events and requests Go hands JS, whose fields are all there; `Required` and `Optional` mark the fields that differ. `assets.d.ts` declares asset imports, and `tokens.d.ts` the names `token` reads, from the app's theme. NSH's gallery checks against them with only the errors it has against NSH's own. `nsgo check` writes them to `.nsgo/types` with a `tsconfig.json` an app's extends, then runs TypeScript 7's checker, built from its Go sources with `go run` and cached ([tool](tool.md#checking)). The web APIs' types are TypeScript's DOM library's.

## Developing

Under `nsgo dev` the tool compiles TSX ahead for hot updates (`bundler.CompileHot`), and a save that changes only the bundle's files compiles it again and sends it over `/_nsgo/events`, with no Go build. `compiler.App`, built with the dev client, follows the server and runs the new bundle in the same VM (`tsx.Hot`). The hot transform and registry (`hot.go`) port NSH's (`tooling.md`, "Hot module replacement"), and `hot_test.go` ports `nsh-engine/tests/engine/hot.rs`:

- A hot bundle's modules go through a registry, which esbuild's module wrapper calls: a newer bundle runs again the modules whose code changed, and the importers of those that do not take their own updates, up to the entry. A module takes its own when all it exports are components. Importers keep a facade of a module's current exports.
- Top-level components, capitalized functions whose code has JSX, render through proxies that stay the same across updates, each a memo of its component's newest code. Its signature is its code and its module's code outside components, so an edit renders again the instances whose signature changed, and those of the modules run again because a module they import changed; their own state starts afresh, everything else keeps its.
- Top-level `context()` results keep their identity, taking the new default (`keepContext`), and top-level `const`s made by `signal()` or `createNavStack()` keep their value while their initializer's code is the same. A `mount` call made again with the same component and no props keeps what it mounted; from the first that differs, the module's earlier mounts are undone and made anew.
- A component that throws reports the error (`reportError`) and renders nothing until the fix; a failed compile shows over the running app.

## Sandbox

Limits and checks are Go's, never JS's, since a bundle holds only the app's code. `tsx.Sandboxed(name, bundle, limits)` runs a bundle within `tsx.Limits`, which the Go code running it sets: a time budget for loading and for each call into JS (NSH's 10 and 2 seconds in `DefaultLimits`), cut with `Runtime.Interrupt` as `ErrBudget`; a stack depth; and hardening, which deletes `eval`, has the `Function` constructors throw and freezes the built-in prototypes.

The open question proposes that a Go app and its Go components, native code, stay trusted, and only TSX modules are sandboxed, signed and held to capabilities.

Without a heap limit a module can still exhaust memory. That suits the app's own screens; a store of untrusted modules needs an engine with one, which the `tsx` API leaves room for. Capabilities, signed packages and a host's `tsx.View` wait for their [open question](../../ARCHITECTURE.md#open-questions).

## Delivering (planned)

- A host places `tsx.View` where a feature goes, given a verified package or a bundle of its own. The iOS and Android faces gain the same loading call.
- Packages follow `.nshpkg`: a header (app ID, version, build, capabilities, minimum runtime, signing time, certificate, file table with SHA-256s), the files (source, source map, assets) and an Ed25519 signature over both.
- Keys, certificates, revocation lists with sequence numbers, the trust store and rollback protection port as NSH has them, on `crypto/ed25519` and `crypto/sha256`. `nsgo keygen`, `certify`, `publish`, `verify` and `revoke` are NSH's commands.
- Capabilities are declared under `capabilities:` in `nsgo.yaml` and carried in the package, with NSH's keys: `network`, `webview`, `openUrl`, `clipboard`, `picker`, `host`, `statusBar`. Go checks them where each request is made: `fetch`, remote images, web views, `OpenURL`, host calls.
- A package runs on every later runtime, so once TSX ships nothing its modules expose is removed or renamed, as NSH's `compatibility.md` rules.

## Store policies

- Apple's guideline 2.5.2 forbids downloaded code that changes an app's features, while its developer agreement allows interpreted code that keeps the app's purpose.
- Apple's guideline 4.7 allows JavaScript mini apps under its rules: an index of them, and no native API exposed to them without Apple's permission.
- Google Play forbids downloaded executable code, except code run in a virtual machine or interpreter with indirect access to Android's APIs.
- Updating an app's own screens fits both stores. A store of third-party modules owes 4.7's rules, and capabilities keep its modules from what the host did not grant.

## Where this departs from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| QuickJS | goja | No C and no cgo, on every target |
| Oxc, `jsx.rs`, rolldown | esbuild and a port of `jsx.rs` on goja's parser, over CommonJS | Go libraries, no Node; goja's parser reads no ES modules |
| `nsh build` writes the bundle | The tool compiles ahead into the package through `-overlay`; `go run` and `go test` compile at start | Plain Go commands keep working; packages ship no compiler |
| Bytecode in packages | Source only | goja saves no bytecode |
| A heap limit | Time, stack and quotas | goja allocates from Go's heap |
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
