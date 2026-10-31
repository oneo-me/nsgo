# Embedding

NSGo apps run inside apps of their own: Go desktop programs make views with `nsgo.NewView`, and iOS and Android apps link the framework and library `nsgo build -embed` makes. NSH's design is in `nsh/docs/dev/embedding.md`. Read this before changing views, the host channel, `-embed` packages or the iOS and Android faces.

## Views

A view is one surface and one engine; `nsgo.Run` is one view in a window of its own, with its loop. A host makes as many as it likes, in one window or several, so backends keep a surface's state on the surface. What is the process's by nature stays process-wide: the standalone app's runner, window and controller, registries keyed by native object (views, widgets, menu targets), observers that report to every live surface, fonts, the image cache and file storage per data directory.

| Target | `Handle` / class | The engine starts | It stops |
| --- | --- | --- | --- |
| macOS | An `NSView` | In `viewDidMoveToWindow` | `View.Close` |
| Linux | A `GtkWidget`, its floating reference sunk | At GTK's first allocation | `View.Close` |
| Windows | A child window of `nsgo.Parent` | At its first `WM_SIZE` | `View.Close`, or the parent's destruction |
| iOS | `NSGoView`, a `UIView` | In `didMoveToWindow` | As it is deallocated, or `stop` |
| Android | `NsgoView`, a `FrameLayout`, starting a process of the app | At its first size | `stop`, or the process's end |

The engine waits for its place so that the host's settings made before are in the app's first frame: `engine.HostSettings` keeps them until it starts. Once the engine stops, the backend destroys the native components still showing, since a stopped app deletes nothing: Android's runtime calls `NsgoComponent.destroy` as the process ends, and the Go process closes its own components when the pipe closes.

The host owns the window around a view:

- Title, window commands, the menu bar and quitting are ignored. Dialogs and pickers belong to the top-level window.
- A forced appearance shows in the view: an `NSView`'s appearance, `overrideUserInterfaceStyle`, Windows' drawn fields, and on Linux, where GTK's settings are the process's, a CSS provider scoped to the view's widget name that draws its fields, scroll bars and menus in it.
- A view takes the keyboard while the focus is in it, which a press gives it, and keymaps fire only then. On Windows the host's loop translates keys before NSGo sees them, so a shortcut drops the character that follows.
- macOS fields get NSGo's field editor from their cell (`fieldEditorForView:`) rather than from the window's delegate, which is the host's.

On the web each app is a Go program of its own, and a page shows several. `nsgo build -target web` writes `nsgo.json` beside the app, naming its WebAssembly, `wasm_exec.js`, resources, host module and `nsgo.js`, the loader (`cmd/nsgo/web/nsgo.js`), relative to itself. The loader's `mount(element, url, options)` runs the app in the element, or the whole page for null, with `NSGO_MOUNT` (a CSS selector of the element), `NSGO_BASE`, `NSGO_RESOURCES` and `NSGO_HOST` in its environment; the app's own page mounts it so too. An app in an element of a page, or in a frame, leaves the browser's history and title to the page.

## The host channel

`core.Host` is the view's one object for the host's side, in Go for desktop hosts and behind the iOS and Android faces. The app side is `Scope.HostParams`, `OnHostEvent`, `CallHost` and `PostHost`.

- Parameters are read as the app starts and fixed for its life; later requests are events.
- `Send` never waits. While no app runs the channel keeps events for the next one; a stopping app leaves what it had not taken there.
- An app keeps the events of a name nobody hears until its first listener, which hears them once the turn that added it ends. Each keeps `HostInbox` (256), dropping the oldest.
- A call runs the handler of its name on the app's goroutine at once, which answers once, from any goroutine; posts reach it with nothing to answer.
- A call fails without a handler, after `HostCallTimeout` (30 seconds; `CallHostTimeout` with 0 waits for good), when the handler panics, or past 1024 waiting.
- `HandleOpenURL` sees the URLs `Scope.OpenURL` opens, once they pass `CheckURL`, before the platform: the host answers once, and false hands the URL to the platform.
- Values cross as JSON, encoded and decoded again even in Go, so a Go host sees what Swift and Java see: maps, slices, strings, `float64`, booleans and nil.

`HandleDefault` answers every name without a handler of its own, for a host that looks names up itself, as a page does. `nsgotest.Host` runs an app with a channel, so a test plays the host.

### On the web

The page is the host: `mount`'s `params` are the parameters, and its `host` object answers. `call(name, args)` returns the result or a promise of it, a throw or rejection failing the call with its message; posts go to `post(name, args)`, else to `call` with the answer dropped; `openURL(url)`, when present, returns or resolves with whether it took the URL. `player.hostEvent(name, payload)` sends an event, throwing at once for a payload that is not JSON.

Without a `host` option the loader imports `nsgo.json`'s host module, which `nsgo.yaml`'s `web.host` names inside the app's directory and the build copies, named by its content in a release, and awaits its `createHost(player)`; a result without `call` fails the mount. The module imports other code by absolute URL, since it is copied alone. `nsgo dev` serves it and builds again when it changes.

The loader keeps a bridge per app in the global `NSGO_HOST` names (`internal/platform/web/host.go`): the parameters, `call` and `openURL` with an ID that Go answers through `reply` and `opened`, and the events sent before the app ran, which Go takes as it starts, so its first listeners hear them. Values cross as JSON text.

## Settings

`engine.HostEnvironment` (a locale, a text size, light or dark) stands in for the platform's values: the engine applies it over each environment the platform reports, before a test's pin. The layout direction stays the system's, and an appearance the app forces still wins. Android writes the three into the environment its runtime reports instead.

The host's theme overrides are `core.Tokens`: CSS colors by role name, `Values` in both appearances and `Dark` over them, which go over every theme's palette ([themes](themes.md#hosts)). `OnError` hears each failure `RootOptions.Report` does, and `DataDir` moves the app's data and storage.

## iOS

`nsgo build -embed -target ios` builds a static library per platform with the `nsgoembed` tag and `-buildmode=c-archive` (devices; the simulator and Mac Catalyst, each arm64 and x86_64 joined with `lipo`), and makes `<module>.xcframework` of them, the module named after the app. `-dynamic` links each into a dynamic framework instead, which exports only `NSGoStart`, since iOS has no Go `c-shared`. A static library starts Go's runtime on a thread of its own, and only a function cgo exports waits for it, so the tool overlays one file on the app's main package: `NSGoStart`, the one C function, which runs the app's `main` on a goroutine until its `Run`, then loads UIKit's classes and registers NSGo's. Packages keep no cgo files; the overlay is generated.

`NSGo.h` declares `NSGoView` and `NSGoComponentContext` with `objc_runtime_visible`: Go registers them at run time, so they have no symbols, and Swift looks them up by name. The header only declares; its module map autolinks UIKit, Security and libresolv, which Go's runtime needs. The app's resources go beside the framework as `<name>Resources`, which the host copies into its bundle.

- `NSGoView` is registered on `UIView` with the surface's methods, not as a subclass of NSGo's surface class: `apple.Super` finds the superclass from the nearest of NSGo's classes, so one may not inherit another's.
- Foundation's JSON objects cross through `NSJSONSerialization`; the host's blocks are called through their function pointer (`apple.CallBlock`), since purego invokes only blocks it made, and copied while kept.
- The display link holds its view, so a view leaving its window lets the link go. It asks for the screen's top rate, which an iPhone grants only when the host's `Info.plist` sets `CADisableMinimumFrameDurationOnPhone`.
- purego returns no floats from methods, so the text size is a setter, `setFontScale:`.
- Catalyst is GOOS=darwin built with the `ios` tag, as gomobile builds it, so the iOS backend serves it. AppKit's threads drop the signal stack Go's preemption signal needs, so its slices link with `asyncpreemptoff=1`.

## Android

`nsgo build -embed -target android_arm64` (or `android_x64`) writes `<name>.aar`: the Java runtime as `classes.jar`, which `runtime/build.sh` builds beside `classes.dex`, the app's executable as `jni/<abi>/libnsgo.so`, its resources as assets and a library manifest asking for extracted native libraries and naming the clipboard's provider after the host's `applicationId`.

Each `NsgoView` starts a process of the app, so views are apart by nature. The parameters go in `NSGO_HOST_PARAMS` and the data directory in `NSGO_DATA_DIR`. Tokens, events, the host's handled names, replies and the URL handler's answers cross as `msgHost`; calls, posts, URLs and failures come back as `msgHostCall`, queued in Java until the process starts. A component name Go does not know goes to Java as `host:<name>`, which the view's registered `NsgoComponent.Factory` makes; its events carry one JSON value. A host package holds one app, since the runtime starts `libnsgo.so`.

## Departures from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| A runtime library loads apps' bundles (`AppView.load`) | The framework, library or Go program is the app; a view runs the one app built with it | Go apps are native code, compiled with the engine |
| A Swift module and a C ABI | A header declaring classes Go registers, and one cgo export | NSGo writes no C or Swift; purego reaches the runtime |
| A dropped `HostReply` rejects the call | An unanswered call fails at its timeout | Go has no destructors |
| One engine thread per Android view, over JNI | A process per view, over pipes | Android runs NSGo apps as processes |
| Native modules, `EngineContext`, limits, cache options, dev server sessions | Not ported | Go hosts call the app's Go directly and give each view its values |
| A web page's host has only `call`; `nsh test` runs the host module | The page's host also takes posts and URLs; tests play the host with `nsgotest.Host` | NSGo's channel has posts and the URL handler, and Go tests run without a JS engine |

Capabilities stay an [open question](../../ARCHITECTURE.md#open-questions): the host's handlers are its own code, which any app it embeds may call.
