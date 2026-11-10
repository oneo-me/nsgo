---
title: Embedding
description: Put an NSGo app inside an iOS, Android, desktop or web app of your own.
group: Toolchain
order: 23
---

## Build an embeddable app

An embeddable app is a normal app package with a `Root` and a `main`, built with `-embed` for the host's target. Each view runs the app once and is an engine of its own, so a host shows as many views as it likes, each with its own state.

```sh
go tool nsgo build -embed -target ios examples/embedded
go tool nsgo build -embed -target android_arm64 examples/embedded
go tool nsgo build -target web examples/embedded
```

The iOS build writes `<Name>.xcframework` and `<Name>Resources`; the Android build writes `<Name>.aar`; the web build writes a directory with `nsgo.json` and the loader.

## iOS

Add the framework to the app target as a static library (do not embed it) and the resources folder as a folder reference, then `import <Name>`. Call `NSGoStart()` once as the app launches, before the first view.

```swift
import Embedded
import UIKit

@main
final class AppDelegate: UIResponder, UIApplicationDelegate {
	func application(_ app: UIApplication, didFinishLaunchingWithOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
		NSGoStart()
		return true
	}
}

let view = NSGoView(frame: .zero)
view.params = ["user": "Ada"]              // cx.HostParams()
view.dataDirectory = documents.path        // where storage lives
view.localeIdentifier = "de-CH"            // in place of the system's
view.setFontScale(1.3)
view.overrideUserInterfaceStyle = .dark
view.tokens = ["accent": "#cf222e"]        // over the app's theme
view.handle("time") { _, reply in reply(Date().description, nil) }
view.send("message", ["text": "Hello"])    // cx.OnHostEvent("message", …)
view.onOpenURL = { url in router.open(url) }
view.onError = { message in log(message) }
view.register(component: "host.badge") { context in Badge(context) }
```

`NSGoView` starts the app once it is in a window and stops it when it is deallocated or sent `stop()`. Set the launch values before it is in a window; the rest may change at any time. Handlers, components and callbacks run on the main thread, and a reply may come later from any thread. A view controller that forwards the status bar returns the view's `preferredStatusBarStyle` and `prefersStatusBarHidden`.

## Android

Add the AAR to the app module and package native libraries extracted, so the runtime finds them:

```kotlin
implementation(files("libs/Embedded.aar"))

android { packaging { jniLibs { useLegacyPackaging = true } } }
```

Make `dev.oneo.nsgo.NsgoView`s in code or in a layout. Each starts the app in a process of its own as it first gets a size, and `stop()` ends it. Forward the activity's results to every view, and handle configuration changes in the activity so the apps are not restarted.

```java
NsgoView app = new NsgoView(context);
app.setParams(Map.of("user", "Ada"));
app.setStorageDirectory(dir);
app.setLocale(Locale.forLanguageTag("de-CH"));
app.setFontScale(1.3f);
app.setAppearance(NsgoView.APPEARANCE_DARK);
app.setTokens(Map.of("accent", "#cf222e"), null);
app.handle("time", (data, reply) -> reply.resolve(now()));
app.handle("tapped", (data, reply) -> status.setText("tapped " + ((Map<?, ?>) data).get("count")));
app.setOpenUrlHandler(url -> router.open(url));
app.setErrorListener(message -> log(message));
app.registerComponent("host.badge", Badge::new);
```

```java
@Override
protected void onActivityResult(int request, int result, Intent data) {
	super.onActivityResult(request, result, data);
	for (NsgoView app : apps) app.activityResult(request, result, data);
}
```

## Desktop

A Go host runs the platform's event loop and puts each view's `Handle` in its window. `nsgo.NewView` starts the app once the handle is in a window, so settings made before are in its first frame.

```go
channel := core.NewHost()
channel.SetParams(map[string]any{"user": "Ada"})
channel.Handle("time", func(c *core.HostCall) { c.Resolve(time.Now().Format("15:04:05")) })
channel.HandleOpenURL(func(url string, answer func(bool)) {
	answer(strings.HasPrefix(url, "https://example.com/orders/"))
})

view := nsgo.NewView(app.Root, nsgo.HostChannel(channel), nsgo.DataDir(dataDir))
defer view.Close()

view.SetTokens(core.Tokens{Values: map[string]string{"accent": "#cf222e"}})
view.SetLocale("de-CH")
view.SetFontScale(1.3)
view.SetAppearance(core.AppearanceDark)
handle := view.Handle() // an NSView, GtkWidget or child window
```

`view.Host()` sends events to the app and receives its calls, and `nsgo.Component` registers a native component for the host's platform.

## Web

The web build is a page of its own, but a page can mount it in an element and be its host. `mount(element, url, options)` returns a player, and `nsgo.json` names the loader, the wasm and the resources relative to itself.

```html
<div id="app"></div>
<script type="module">
const url = new URL("./Embedded-0.1.0-web/nsgo.json", location.href);
const config = await (await fetch(url)).json();
const { mount } = await import(new URL(config.loader, url));

const player = await mount(document.getElementById("app"), url.href, {
	config,
	params: { user: "Ada" },
	host: {
		call(name) {
			if (name === "time") return new Date().toLocaleTimeString();
			throw new Error(`no host function ${name}`);
		},
		post(name, data) { console.log(name, data); },
		openURL(url) { return url.startsWith("https://example.com/orders/"); },
	},
});
player.hostEvent("message", { text: "Hello from the page" });
</script>
```

Without a `host` option the loader imports the module `nsgo.yaml`'s `web.host` names and awaits its `createHost(player)`. An app in an element or a frame leaves the browser's history and title to the page.

`appearance` (`"light"`, `"dark"` or `"system"`), `locale` and `fontScale` stand in for the browser's, as a native host's settings do, and the player's `setAppearance`, `setLocale` and `setFontScale` change them later. `module`, a compiled `WebAssembly.Module` or a promise of one, spares fetching and compiling the wasm, so a page mounting the app many times, or serving the wasm compressed, compiles it once.

```yaml
web:
  host: host.js
```

## The host channel

`core.Host` is the one contract every host implements: launch parameters, events the host sends, and the host's functions the app calls. Values cross as JSON does, so maps become `map[string]any`, numbers `float64`, and the same code works in Swift, Java, Go and JavaScript.

- `HostParams` reads the launch parameters once, at start.
- `cx.OnHostEvent` hears `Host.Send` and the page's `hostEvent`.
- `cx.CallHost` calls a named handler and resolves with its answer; `cx.PostHost` calls one without an answer.
- `Host.HandleOpenURL` sees `cx.OpenURL` before the platform, and its answer decides whether the platform opens it too.
- `Host.SetParams`, `Send`, `Handle` and `HandleDefault` are safe from any goroutine.

On iOS the host sets `params`, installs handlers with `handle`, sends events with `send` and answers `onOpenURL`. On Android it does the same with `setParams`, `handle`, `send` and `setOpenUrlHandler`. On desktop it uses `core.Host`'s methods, and on the web the page's `host` object answers.

## Native components

`core.Native` shows a view the host makes: its `Props` reach the host component's `SetProp`, what the component emits reaches `OnEvent`, and a prop whose value is an animated node moves on the frame clock without work for the engine.

```go
core.Native{
	Component: "host.badge",
	PropsFunc: func() map[string]any { return map[string]any{"count": count.Get()} },
	OnEvent:   func(name string, data any) { if name == "tap" { count.Set(0) } },
	Style:     core.Style{MinWidth: core.Pt(120), MinHeight: core.Pt(28)},
}
```

Register the component in the host: `app.register(component:)` and an `NSGoComponent` on iOS, `registerComponent` and an `NsgoComponent` on Android, `nsgo.Component` on desktop. The web has no native components, so an unknown name shows an empty box there.

## Settings

`tokens` overrides the app's theme by role name (`accent`, `inset`, `raised`), in both appearances or in one; `cx.Palette()` sees them. The host also sets the locale, the text size, the appearance and the data directory, and hears the app's failures. `nsgo build -embed` writes no windows, menus or tray icons: those belong to the host.

`examples/embed-ios`, `examples/embed-android`, `examples/embed-desktop` and `examples/embed-web` are the same app with each kind of host.

## Next

- [Platform services](services.md): what the app asks the platform for.
- [Building and shipping](building.md): the packages `-embed` makes, and signing them.
- [Example: A todo app](tutorials/todo.md): the app a host embeds.
