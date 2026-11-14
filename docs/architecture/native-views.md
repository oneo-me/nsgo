# Hosted views

`WebView`, `Video` and `Native` host a platform view inside a view: the platform's web view and player, or one the app registered. Each follows NSH's design and behavior on every target; NSGo changes only the API's shape to Go's. Read this before changing these elements or a backend's host for them.

## Shared design

- The control fills a `KindWeb`, `KindVideo` or `KindNative` view, which keeps the decoration, frame and accessibility.
- One config item carries the whole state: `ItemWeb` with `WebConfig`, `ItemVideo` with `VideoConfig`; `ItemNative` carries a component's changed props.
- Commands ride their own items: `ItemWebCommand` for `Ref`'s history, scripts and messages, `ItemVideoSeek` for `Ref.Seek`, `ItemNativeCommand` for `Ref.Command`.
- Core resolves defaults before sending, so platforms see final values: a video's volume, rate, progress interval and resize mode (Contain).
- Platforms report an event only when the view's `EventMask` has it.
- NSGo keeps Go's zero values as defaults: `NoScroll` and `NoJavaScript` invert NSH's `scrollable` and `javaScriptEnabled`, and `window.nsgo` replaces `window.nsh`.
- NSH's navigation check against an app's `webview` capability is not ported; see [Open questions](../../ARCHITECTURE.md#open-questions).

## Web views

- Every page starts with `WebBridge`, defining `window.nsgo.postMessage(data)`, which reaches `OnMessage` as a string. `PostMessage` dispatches a `MessageEvent` on the page's window.
- `Ref.Evaluate` answers with a script's result as JSON, or its error, through that bridge rather than each platform's evaluation callback, so every backend with web views answers alike, Android's with no runtime rebuilt: `WebEvaluateScript` runs the script as an expression, awaits a promise, and posts the answer after a marker with a random key, which the core takes for the `Evaluate` waiting on it and never hands to `OnMessage`. A page leaving (`EventLoadStart`, reported while one waits) ends those waiting with an error. A page whose bridge is gone, such as a URL on the web, which gets none, never answers.
- A page loads again when the source, an injected script or its request headers change. Other settings apply to the live view.
- `WebView.Headers` adds fields to the initial URL request on Apple, Windows, Linux and Android; HTML sources and the web backend do not apply them yet.
- On Apple, a `file:` source uses `WKWebView`'s file loader and grants the page read access to its containing directory; links to files outside that directory are refused.
- `WebView.Incognito` uses WebKit's non-persistent data store on Apple, an ephemeral WebKitGTK network session on Linux and WebView2's in-private controller profile on Windows. Android disables its cache, form saving and DOM storage for an incognito view; the web backend has no per-frame storage switch.
- `Env.HasWebView` reports whether the target provides a web view host; it is false on tvOS, whose UIKit target has no WebKit host, and true on the other targets that expose `WebView`.
- `Scope.ServeScheme` registers app-owned URL schemes for web views under that scope, refusing those web views handle themselves, such as `ws` or `javascript`, which would end the app on Apple. Apple uses `WKURLSchemeHandler`, Windows answers WebView2 resource requests, and Linux registers a WebKitGTK URI scheme; the handler returns bytes and the native host infers their content type, and pages of other schemes cannot read them. Schemes are read when a native view is first made. WebView2 takes its schemes only as its environment starts, with the batch that makes a profile's first web view: a scheme served after that does not load on Windows, which the log says once. Android and the web backend do not provide this bridge yet.
- Load start and load each come with a navigation state change, carrying `WebNavigation`. Errors arrive as `*WebError` with the failing URL.
- A web view with `OnDownload` saves what its pages download, as NSH's macOS one does: core sends `WebConfig.DownloadDir`, the app's data directory's `downloads`, and Apple's host turns a main frame's response WebKit cannot show or marked an attachment, and a link asking for a download, into a `WKDownload`; Linux's WebKitGTK host handles its `download-started` signal; Windows' WebView2 host handles `DownloadStarting`; and Android's WebView host handles `DownloadListener`. These hosts save to `<id>/<file name>` under that directory and report their start, end and bytes or failure ([roadmap](../roadmap.md#finish-the-existing-platforms)).
- `Ref.SendInput` gives the page a click, typed text or a key as the user's own: macOS hands the `WKWebView` AppKit events, with each character's US key, so the page's events are trusted (`isTrusted`) and carry user activation, which `InjectJavaScript` cannot do. It takes the keyboard focus first. `Env.CanSendInput` reports this capability; other targets drop it, as NSH's do. TSX apps will need the `webview` capability's `input` for it ([roadmap](../roadmap.md#tsx-and-dynamic-delivery)).

| Target | Host |
| --- | --- |
| Apple | One `WKWebView` host in `internal/platform/apple`; URL loads use `Headers`, `Incognito` selects `WKWebsiteDataStore`'s non-persistent store, `ServeScheme` uses `WKURLSchemeHandler`, and `NoJavaScript` sets the live configuration's webpage preferences, which apply from the next page |
| Windows | A WebView2 composition controller in a DirectComposition visual, between layers of what is drawn after it; URL loads use `Headers` (`NavigateWithWebResourceRequest`), `Incognito` uses WebView2's per-controller in-private profile options, `ServeScheme` registers secure schemes with an authority on the environment (`ICoreWebView2EnvironmentOptions4`) and answers their resource requests, and the backend hit-tests and forwards input. The Evergreen runtime is found in the registry and its DLL called as WebView2's loader would, so nothing ships with the app |
| Linux | WebKitGTK 6.0 hosted in the view's widget, mapped once the view has a size, with URL loads using `Headers`, app-owned schemes registered on the network session, website data per app ID (or an ephemeral session for `Incognito`) and NSH's history keys and mouse buttons |
| Android | `android.webkit.WebView` with `window.nsgo` as a JavaScript interface; URL loads use `Headers`, `Incognito` disables cache, form saving and DOM storage, and scripts meant for the document's start run when the page starts, as android.webkit has no earlier hook |
| Web | An `<iframe>`. HTML loads as `srcdoc` in a sandbox with an origin of its own and gets the bridge over a `MessageChannel`; a URL gets only `PostMessage` and load events, as the browser hides the page |

## Videos

- A source is a URL, a file path or an `asset:` path resolved against the app's resources.
- `EventLoad` carries the duration and the picture's size in points; `EventProgress` comes every `ProgressInterval` while playing.
- `EventEnd` follows the platform: AVFoundation and Android report every end, then a repeating video starts over; a looping Media Foundation engine, `<video>` or GTK stream never ends.
- `Controls` shows the platform's controls. Windows has none, so it draws NSH's bar.
- `AudioSession` is NSH's `audioSession`. On iOS, while a video with one plays, the shared `AVAudioSession` is the playback category in the spoken-audio mode (mixing with others for `AudioSessionMixWithOthers`), and goes back to what it was, deactivated to let other apps play, once none plays (`apple/audiosession_ios.go`). On Android the sound is speech, and a playback video holds `AUDIOFOCUS_GAIN_TRANSIENT` while it plays, pausing for another app's focus and playing on when it returns; a change opens the source again. The desktops and the web ignore it.

| Target | Player |
| --- | --- |
| Apple | `AVPlayer` in an `AVPlayerLayer`, or `AVPlayerView` and `AVPlayerViewController` with controls; AVFoundation and AVKit load on the first video |
| Windows | Media Foundation's media engine in frame-server mode, drawing each frame as a Direct2D bitmap |
| Linux | `GtkMediaFile` (GStreamer) in a `GtkPicture` hosted in the view's widget, or a `GtkVideo` with controls; GTK streams have no playback rate |
| Android | `MediaPlayer` on a `TextureView`, with a `MediaController` for controls |
| Web | A `<video>` element |

## Native components

An app registers its own platform views by name, as NSH's embedding apps do (`nsh/docs/dev/embedding.md`): `nsgo.Component(name, factory)` hands `nsgo.Run`'s or `nsgo.NewView`'s backend a `core.NativeComponents` table, so a registry belongs to one surface, never to the process. An iOS or Android app embedding NSGo registers its own Swift or Java components with each view, which take the names Go's table lacks ([embedding](embedding.md)).

- `Native` names the component; its `Props` cross only when they change, a removed one as nil, and `Ref.Command` reaches the component's `Command`.
- The first `ItemNative` makes the component with a `NativeContext`, whose `Emit` reaches `OnEvent` and whose `SetSize` gives the element a natural size, as an image's; a width of 0 asks for its height alone, as a bar as wide as it is placed.
- A prop whose value is an `AnimatedNode` is bound rather than sent, as NSH 0.3.0's are: `NativeConfig.Bindings` names each bound prop's node and whether it outputs a color, and the backend's `Animator` (`BindNative`, `ChangedNatives`) hands the component each value that changed on the frame clock, through `driver.AnimatedPropSetter` or else `SetProp` with a `float64` or a `Color`. A plain value replacing a node is sent; a removed prop is sent as nil. Android binds them in Java as a canvas's arguments are and calls the view's setters, or a registered `NsgoComponent`'s `setAnimatedNumber` and `setAnimatedColor`, never an `AndroidPropMapper`; iOS hosts' components take the optional `setAnimatedNumber:value:` and `setAnimatedColor:argb:`.
- macOS and iOS register `driver.SwitchComponent` themselves, for `core.SystemSwitch`: NSSwitch and UISwitch, showing the user's choice at once and measuring themselves. Its parent centers it rather than stretching it, as a native view with a natural size keeps its aspect when stretched.
- iOS and Android register `driver.TabBarComponent` themselves, for `mobile.TabBar`; Android's measures itself 80 points tall plus the system navigation bar it reaches under.
- Factories run on the main thread and may return nil; an unknown name leaves an empty box and is logged once.
- Registering differs per platform only in what `View` returns, so an app registers a target's components in files built for it, as the gallery's `components_*.go` do.

| Target | `View` returns | Hosted as |
| --- | --- | --- |
| Apple | An `NSView` or `UIView` | A subview resizing with the view; on iOS placed at each frame, as views such as `UITabBar` size themselves |
| Windows | A window handle | A child window following its view after every frame, clipped with a window region to what its clipping ancestors leave visible, above what NSGo draws, as NSH's `NativeComponent::window` |
| Linux | A `GtkWidget` pointer | A widget hosted in the view's widget, as web views and videos are |
| Web | A DOM element (`js.Value`) | A child element filling the view |
| Android | The name of a `View` class | Built by reflection in the Java runtime, props set through its one-argument `set` methods and commands through its methods; the Go process cannot make views |

On Android the runtime also installs a listener on each `setOn…Listener` method the view's own class declares and reports its callbacks as events named after them, their arguments as a JSON array. A component may implement `android.PropMapper` to turn a prop into ordered setter calls, and `android.EventMapper` to receive those callbacks itself, as the gallery's `SeekBar` does.

## Known gaps

- Linux keeps presses, hovers and wheels from a page or a video's controls where the app does not show them, as NSH does; drags that begin there are not followed.
