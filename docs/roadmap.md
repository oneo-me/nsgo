# Roadmap

This plan lists what NSGo may still take from NSH's `main` at `76a17753` (2026-11-10), 469 commits past its 0.4.0 release (tag `v0.4.0`, `e626a8dd`), in the order to build it, each with tests of NSGo's own: this once the plan follows `main` rather than a release (decided by the user, 2026-11-10). Each item names its NSH reference and the result that finishes it. What exists is in [ARCHITECTURE.md](../ARCHITECTURE.md), and what NSGo leaves out on purpose in [NSGo and NSH](architecture/nsh.md#what-nsgo-leaves-out). Design questions that block an item are in [Open questions](../ARCHITECTURE.md#open-questions).

References are files in `nsh/docs/dev/` unless they give a path from `nsh/`, NSH's checkout beside the repository, with the section to read in brackets, or NSH's pull requests (#931) where no page describes the work. [Reading NSH's releases](architecture/nsh.md#reading-nshs-releases) says how to rebuild this plan for NSH's next release.

## Performance against NSH's baselines

NSH records what its hot paths cost (`data-table.md` (Performance), `performance.md`, `web.md`), and NSH's own build on each machine is the measure: the gallery's scenarios (`examples/gallery/perf_test.go`) run the same steps as an `nsh test` file there ([performance](architecture/performance.md#keeping-frames-cheap) says how and lists the readings of 2026-11-09). Headless, in headless Chrome, on macOS for the DataTable and the stack, and in the iOS simulator, NSGo costs what NSH does or less. Those readings were taken against 0.4.0's build; `main` rewrote `performance.md` and made lists and scroll views cheaper (#1304, #1310, #1371), so each comparison is measured again against its build before an item here closes. What is left:

| Item | NSH reference | Done when |
| --- | --- | --- |
| Animated layers on macOS | `performance.md` (The performance monitor) | The gallery's Animations page costs on macOS what NSH's does (13–15% of a core against 8%, 1.1–1.5 ms against 0.5 ms a frame), measured at 120 Hz on a Mac whose screen is unlocked: with it locked both builds' display links stall (2026-11-09). NSGo's samples spend frames in AppKit's window layout pass (`_findAnySubviewNeedingAutoLayoutEngine`), which NSH's do not |
| GTK frames | `performance.md` (Linux) | The DataTable's scroll costs on Linux what NSH's does (5.4 ms a frame and 51% of a core against 2.2 ms and 32%), and its ticks and the stack's slides too (16% and 10% against 14% and 6%); most of a scrolling frame is GTK's own snapshot and rendering under its main loop (2026-11-09) |
| Windows and Android | `data-table.md` (Performance) | The DataTable's scroll and the stack's slides on Windows (41–44% and 28–29% against 30–32% and 20–22%, 2026-11-09) and the DataTable's scroll on Android (125% against 111% on the emulator) cost what NSH's do; and on a Pixel 10 Pro the table holds about NSH's ~870 views, commits at 3 and 7 ms at the median and 90th percentile, and slides on whole pixels |

## Services and components since 0.4.0

What NSH's `main` added after 0.4.0 that NSGo has not, in the order to build it.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Notifications and the badge | #931, #932, #933, #1140, #1373 | The app asks for permission, shows a system notification, silent if it asks, and sets its badge count on macOS and iOS (`UNUserNotificationCenter`), Android (`NotificationManager`, with `classes.dex` rebuilt), Windows (toasts and the taskbar badge) and Linux (`org.freedesktop.Notifications`), and `nsgotest` records them |
| Sounds and audio | `audio-design.md` (The API, The platforms) | `Sound` plays a short sound from a file or bytes and `AudioPlayer` plays, pauses, seeks and loops audio on every target, decoding off the main thread, and the gallery gives each kind of moment a sound |
| Haptics | #1328 | `mobile` plays impact, selection and notification feedback on iOS (`UIFeedbackGenerator`) and Android (`performHapticFeedback`, with `classes.dex` rebuilt), and does nothing elsewhere |
| The app's own information | #1381, #1226 | Go and TSX read the app's version, build and whether it is a development build, as `nsgo.yaml` gives them (`appInfo()`) |
| The user's locale settings | #976 | `Env` reports the first weekday, hour cycle and measurement system the user chose, apart from the language, and `DatePicker` and `i18n` follow them |
| Restarting the app | #1055 | `Restart` ends the app and starts it again on macOS, Windows and Linux, as `Quit` ends it |
| Script results from web views | `elements.md` (WebView evaluation) | A web view's `Evaluate` answers with the script's result as JSON, or its error, on every backend with web views |
| More of a web view's loads | #1038, #1028, #1233, #1496, #1439 | A load carries the app's request headers, `Incognito` keeps the website data in memory, the app serves URL schemes of its own to its pages, a page from the app's own files loads on macOS, and `Env` says where the platform shows no web view |
| Selections as Markdown and HTML | #1045 | A selectable area's selection reads as plain text, Markdown or HTML, as the text view parsed it |
| Hidden spans | #1438, #1463 | A span hidden in place keeps its room in no line, leaves copies and screen readers, and is skipped rather than drawn transparent |
| An emoji picker | `components.md` (Emoji pickers) | `base.EmojiPicker` searches every emoji by its names in the app's languages, its words through `base/locales`, with a styled form in `ui` |
| SVG on canvases | `canvas.md`, #1291 | A `Drawing` draws an SVG image at a size, its text drawn by the platform's text engine |
| Scroll handles on phones | #1401 | A long scroll view on iOS and Android shows a handle the user drags along either axis |
| Image memory | #1330 | The app sets how many encoded image bytes stay in memory, and every backend's cache keeps to it |
| A terminal backend | `tui.md` | An app runs in a terminal, its views drawn in cells and its keys, mouse and resizes read from it, under `nsgo dev` and `nsgo test`; `base/terminal` stays the terminal shown inside an app |
| Store releases | `tooling.md` (Releases), #1052, #1054 | `nsgo build` writes Android App Bundles with `targetSdkVersion` from `nsgo.yaml`, and a command uploads to TestFlight and Play's internal track, named apart from `nsgo publish`, which signs TSX packages |

## Application and windows

What Windows and Linux still lack of what macOS opens for an app ([application and windows](architecture/application.md)): there `OpenWindow`, `OpenPanel` and `NewTray` return nil, and the main view fills the one window. Avalonia's desktop lifetime is the reference for the model, and NSH's `main`, which opens windows, tray icons, global shortcuts and one instance on Windows and Linux too, for each platform's mechanism.

| Item | Reference | Done when |
| --- | --- | --- |
| Surfaces on Windows and Linux | macOS's `own` surface; NSH's `Window.open` (#989, #1011, #1137) | The backends run the app on a surface in no window and attach each one the app opens: windows, and panels that take the keyboard and hide as they lose it |
| Window limits on Linux | `XSetWMNormalHints` on X11, `xdg_toplevel.set_max_size` on Wayland; `GtkHeaderBar`'s decoration layout | GTK 4 reaches neither from its API: a maximum other than a fixed size holds, and `NoMinimize` and `NoMaximize` take the system title bar's buttons |
| Tray icons on Windows | `Shell_NotifyIcon`; #988 | An icon shows its image, its panel above or under it (`Shell_NotifyIconGetRect`) and its menu; a drawn icon is an element rasterized into the icon whenever it changes |
| Tray icons on Linux | StatusNotifierItem over D-Bus; #988, #1209 | KDE and GNOME with its AppIndicator extension show the icon and its menu; GNOME without one shows none |
| Global shortcuts on Windows and Linux | `RegisterHotKey`; `XGrabKey`, the GlobalShortcuts portal; #988 | A shortcut shows a panel from any app, on Wayland once the user grants it |
| One instance on Windows and Linux | A named mutex and pipe; a Unix socket; #1563 | A second launch hands its arguments to the running app, which hears `OnReopen`, and the files it names `OnOpenFiles` |
| File types on Windows, Linux and iOS | `HKCU\Software\Classes` and `SHChangeNotify`; a `.desktop` entry's `MimeType` with a shared MIME-info package; `CFBundleDocumentTypes` with `scene:openURLContexts:` | Their packages register `file_types` as the macOS app does, and opening such a file reaches `OnOpenFiles`, which on Windows and Linux now hears only the files named at launch |
| Launch at login on Windows and Linux | The `Run` registry key; an XDG autostart entry | `SetLaunchAtLogin` starts the app at login and `LaunchAtLogin` reads it |
| Displays | #1044 | `Env` reports each display's frame, visible frame and scale on macOS, Windows and Linux, so an app places a window on the screen it chooses |
| Tray titles and bounds | #1479, #1355, #1582 | A tray icon shows a title beside it in macOS's menu bar, and its click reports the icon's bounds, so an app places its own popup there |
| Transparent windows and the taskbar on Windows | #987 | A window opens transparent, showing what is behind it where the app draws nothing, and `SkipTaskbar` keeps it off the taskbar |

## Terminal

What `base/terminal` still lacks of a terminal app's features, Ghostty's the reference ([terminal](architecture/terminal.md)), in the order to build them.

| Item | Ghostty reference | Done when |
| --- | --- | --- |
| Bold and italic monospace text on Apple platforms | — | A monospace or named family's bold and italic draw in their faces: Apple's `buildFace` copies traits onto a descriptor that names a font, which keeps the regular face |
| Android | — | An Android app's remote session runs: libghostty-vt built for `aarch64-linux-android` (Ghostty's build takes the NDK's sysroot, which `tools/libghostty` would download) goes into the APK's native libraries, and the Go process, which has no cgo on arm64, loads it: purego loads libraries on Android only through cgo, so this needs a loader for bionic without cgo, checked on a device |
| iOS on a device | — | The gallery's `TestTerminalPhone` passes on an iPhone with its signed framework and software keyboard, as it does under `nsgo test -target ios_simulator` since 2026-11-05 |
| `-embed` frameworks and other packages | — | `-embed` XCFrameworks and AARs, and Windows and Linux packages, carry libghostty-vt beside the app, as macOS and iOS apps do |
| The web | — | A page runs a remote session over a WebSocket with libghostty-vt built for WebAssembly, which the browser runs, through the binding's calls |
| Mouse reports and the wheel on full-screen programs | `mouse-reporting`, `mouse-scroll-multiplier` | A program that takes the mouse gets presses, moves and the wheel as it asks, and the alternate screen gets the wheel as arrows |
| Paths as links | `link-url` | Command+click (Control+click elsewhere) opens a path printed as text, relative ones from the shell's directory, as it opens URLs and hyperlinks |
| Clipboard protocols | `clipboard-read`, `clipboard-write` | Programs copy with OSC 52 and read the clipboard as the app allows, and the Kitty clipboard protocol offers pasted images |
| Keybinds, zoom, the context menu and secondary clicks | `keybind`, `right-click-action` | Ghostty's actions bind to keys over the defaults, zoom changes the font's size, and a secondary click shows Copy, Paste, Select All and Clear |
| Drawn box characters, blinking and contrast | `cursor-style-blink`, `minimum-contrast`, `window-padding-color` | Box drawing, blocks and Powerline's separators are drawn so lines join, the cursor blinks, and minimum contrast and padding colors apply |
| Search, images and shell integration | Find, the Kitty graphics protocol, `shell-integration` | A search highlights matches, programs show images, and OSC 133 prompts take Option+click to move the cursor |

## Widgets

What package `widget` lacks of WidgetKit, and of widgets elsewhere ([tool](architecture/tool.md#widgets)).

| Item | Reference | Done when |
| --- | --- | --- |
| Widgets on devices Xcode signs | `signIOSApp`'s project | Xcode's project gets an app extension target signed automatically beside the app's, so `nsgo dev -target ios` and `-run` on an iPhone keep the widgets they now leave out |
| The app's own links | `CFBundleURLTypes`, `application:openURLs:` | `nsgo.yaml` declares URL schemes and a tap's `Entry.URL` or `Style.Link` of one reaches the running app, which now only opens |
| Interactive widgets | `Button(intent:)`, `appintentsmetadataprocessor` | A button in a widget runs a Go handler the app declares, as an `AppIntent` the tool compiles with its metadata, without opening the app |
| Widgets of sandboxed Mac apps | `com.apple.security.application-groups` | A macOS app in the App Sandbox, as the Mac App Store's, shares its timelines through `apple.app_group` and a provisioning profile, where it cannot write the home directory the extension reads |
| Widgets on Android | `AppWidgetProvider`, `RemoteViews` | The same timelines show in App Widgets drawn by the Java runtime, with `classes.dex` rebuilt |

## App Clips

What `app_clip` lacks of App Clips ([tool](architecture/tool.md#app-clips)).

| Item | Reference | Done when |
| --- | --- | --- |
| A clip on an iPhone | `startIOSDeviceClip` | `nsgo dev -target ios -clip <url>` signs, installs and opens the clip on an iPhone with a paid team, and its QR code opens it through Local Experiences; it is written but unchecked, as only a personal team was at hand |
| Clips in apps Xcode signs | `signIOSApp`'s project | Xcode's project gets an App Clip target embedded in the app's, so `nsgo dev -target ios` and `-run` on an iPhone keep the clip they now leave out |
| Getting the full app | `SKOverlay.AppClipConfiguration` | `mobile` shows the App Store's overlay offering the full app from a clip, through purego |
| Handing data to the full app | App groups | The clip and the app share `apple.app_group`'s container, so the app finds what the clip stored once installed |
| Links into the full app | `applinks:`, `CFBundleURLTypes` | The full app takes universal links and URL schemes into `mobile.OpenedURL`, as the clip takes its invocation |
| Opened links elsewhere | `Intent.getData`, `location.href` | Android's App Links and the web page's address reach `mobile.OpenedURL` as iOS's links do, which reads "" there now |

## Apple TV

tvOS runs on iOS's backend, its remote moving a focus ([keyboard focus](architecture/interaction.md#keyboard-focus)); what it still lacks:

| Item | NSH reference | Done when |
| --- | --- | --- |
| A real Apple TV | — | `nsgo dev -target tvos` and `nsgo build -target tvos -run` sign, install and run the gallery on a paired Apple TV, as they do on an iPhone; so far it ran in the simulator alone |
| Sizes for a room | — | Apps can read across a room: `ui`'s 28-point controls and 13-point text at 1920×1080 points read small from a couch, so `ui` or the app takes larger sizes there, never `core` or `base`, an [open question](../ARCHITECTURE.md#open-questions) until the user decides |
| The focus's look | — | A focused view lifts as tvOS's do, scaled with a shadow, or keeps `ui.FocusRing` by decision; the ring shows today |
| Device branches out of `base` | — | `base.Menu`, `Select`, `ContextMenu` and `base/dock.Area` change form by `Env.Desktop` or width no more, against the rule that `base` does nothing by device: `ui` or a field the app sets chooses, so a TV's popups show at their trigger rather than as touch sheets |
| Scrolling with the remote's surface | — | A scroll view with nothing focusable in it, such as a long text, scrolls as the remote's touch surface swipes, which only the focus moving scrolls now |
| What tvOS has not | — | Web views (no WebKit), the camera, the photo picker, speech, recording and the clipboard report `ErrUnsupported` or their platform's refusal on tvOS, each checked from the gallery, rather than failing as they reach a missing class |
| The terminal and embedding on tvOS | — | `tools/libghostty` builds libghostty-vt for tvOS and its simulator, and `-embed` builds a tvOS framework |
| tvOS tests' settling | — | A test step on the tvOS simulator settles once a dialog's transition ends, rather than after `settleWait`'s 5 seconds: an `nsgo.opacity` animation stays on its layer there |

## Apple Watch

watchOS runs in a SwiftUI host, its views nodes Go draws into ([platforms](../ARCHITECTURE.md#platforms)); what it still lacks:

| Item | NSH reference | Done when |
| --- | --- | --- |
| A real Apple Watch | — | `nsgo dev -target watchos` and `nsgo build -target watchos -run` sign, install and run the gallery on a paired Series 9 or later, reading `os_proc_available_memory` there, as so far it ran in the simulator alone |
| The App Store's package | — | `nsgo build -target watchos` writes what App Store Connect takes for a watch-only app, its iOS stub included, signed with `sign.profile` |
| A lighter engine | — | `core`'s node holds its static style shared rather than copied, and its own fields packed: about 6 KB a node now, two `Style`s of 1.3 KB among them, which a watch pays most for |
| The Digital Crown and native touches | — | A view that pans or turns a value takes the crown's rotation, and the test host posts touches SwiftUI sees, so tests reach its scrolls and text fields as a finger does |
| Leaving views animated | `animation.md` | A commit's layout animation takes views out with its Delete, which leave at once on a watch now |
| What watchOS has not | — | Web views, video, native components and shaders (no WebKit, AVKit views or Metal), the clipboard, pickers, files, sharing and speech report `ErrUnsupported`, each checked from the gallery; the views show empty boxes and the services answer unsupported now |

## Mini programs

Go apps run in a mini program through a runtime checked in a stand-in host alone ([mini programs](architecture/miniprogram.md)); what it still lacks:

| Item | NSH reference | Done when |
| --- | --- | --- |
| WeChat itself | `nsh-platform-miniprogram`'s README | `examples/embedded` runs in WeChat DevTools and on an iPhone and an Android phone: text lines where they were measured, taps, pans in and beside scroll views, text fields and the keyboard's height, modals, keyframes through `animate`, back through the page container, alerts, storage and photos, with what differs fixed |
| QQ, Alipay and Douyin | — | `examples/embedded` runs in each platform's developer tool and on a phone: its loader takes the module (none of the three documents WebAssembly for mini programs yet), and `api.js`, the markup QQ and Douyin read rewritten from WeChat's (`mut-bind`, handlers named by expressions, `page-container`, `animate`) and Alipay's render layer work as WeChat's do, or what differs is recorded |
| What Alipay lacks | — | Back goes through the app, keyframes run, the keyboard's height reaches the environment and scroll views report drags, on Alipay as on WeChat |
| Tests on the target | `testing.md` | `nsgo test -target miniprogram_wechat` runs an app's tests in DevTools through its automator, with native taps and screenshots, as the other targets' test hosts do |
| Frame rate | `performance.md` | A scroll with sticky headers and a pan holds 60 frames a second on a mid-range Android phone, with setData's paths per frame measured; what each costs decides whether bound values move in the render layer instead |
| Larger apps | — | An app of `ui`'s common components fits the platforms' 2 MB, by what the module links (encoding/json, CLDR's data, unused components) rather than by turning inlining off |
| What WeChat draws | — | The app's font files (`loadFontFace`), images tinted with `TintColor`, inline views aligned off the baseline, selection areas and videos (`<video>`) |
| What a mini program has not | — | Web views (`<web-view>` fills a page of its own), native components, shaders and TSX report `ErrUnsupported` or refuse to build, each checked |

## Installers

What the desktop installers ([tool](architecture/tool.md#installers)) still lack.

| Item | Reference | Done when |
| --- | --- | --- |
| Notarization checked | — | A Developer ID build is accepted by the notary service and opens from a download without Gatekeeper's warning; so far the hardened runtime's signature was checked with a development identity, and the submission as far as notarytool's credentials (2026-11-10) |
| Windows installers with Go and Xcode | — | `build` makes the Windows installer without makensis, which a Mac needs from Homebrew |
| Signed Windows installers | `signtool`, NSIS's `!uninstfinalize` | A certificate in `sign` signs the executable and the installer |
| Checked Linux packages | `pacman -U`, `dpkg -i` | The Debian package installs on Debian and Ubuntu and the Arch one with `pacman -U`, as `install.sh` runs them, which so far were only read by `bsdtar` and `pacman -Qip` (2026-11-10), and the Arch package carries `.MTREE` for `pacman -Qk` |
| Install scripts from GitHub | — | `install.sh` and `install.ps1` install from a published release; so far they installed from the releases directory served on each machine (2026-11-10) |
| The Windows installer's pages | — | Its directory, progress and finish pages, in Chinese among others, are checked on a screen; so far the installer ran silently alone (2026-11-10) |
| File types in installers | — | The Windows installer and the Linux packages register `file_types`, as the macOS app does ([application and windows](#application-and-windows)) |

## Updates

What updates ([updates](architecture/updates.md)) still lack, on the direct channel and the stores'.

| Item | Reference | Done when |
| --- | --- | --- |
| Updates installed end to end | `updater.md` (Flow) | The native tests install a newer release from a local appcast through Sparkle and WinSparkle and start it, a Developer ID signed and notarized macOS app and a Windows arm64 one update, Android reads an `https` appcast, and the App Store's page opens from an iPhone; so far each ran once by hand (2026-11-11, [updates](architecture/updates.md#verification)) |
| Install signs checked | — | A Flatpak and a TestFlight build check nothing, as an Android app installed as Google Play's, a Linux app with `SNAP` set and a development build on an iPhone did (2026-11-11) |
| Installing updates on Linux and Android | `updater.md` (Limits) | A direct Linux or Android app downloads the newer package the appcast names, checks its signature and installs it once the user confirms, Android's through `mobile.InstallPackage`, rather than opening the release's page |
| Store packages | [Store releases](#services-and-components-since-040) | `build -channel store` makes the Mac App Store's sandboxed package and MSIX for the Microsoft Store, without the updaters, as Store releases make Android App Bundles |
| Store updates in the app | Play In-App Updates, `iTunes lookup` | An app from Google Play offers Play's update flow, and one from the Mac App Store its page there, as iOS offers the App Store's |
| Package manager manifests | `brew` casks, `winget-pkgs` | Each direct release writes a Homebrew cask saying `auto_updates true` and a winget manifest for its setup, checked by `brew audit` and `winget validate` |
| Mini programs and the web | `wx.getUpdateManager`, service workers | A mini program answers `updater.Check` from its platform's `UpdateManager` and restarts into the new version, and a web app offers to reload once a newer build is deployed |

## Finish the existing platforms

These items wait for community contributions, and a change to one backend records the gaps it leaves here.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Shader effects on Android | `shaders.md` (Effects, transitions and pictures) | The Java runtime pictures an effect's or a transition's children and the surface into textures its GL thread reads, draws the shader over them and runs pages' and dialogs' `TransitionShader`, and shader lanes read a color node's channels, with `classes.dex` rebuilt; today its backend leaves effects out |
| Backdrops on Windows and Linux | `shaders.md` (Effects, transitions and pictures) | `sample_backdrop` reads what the window drew behind the view, as it does on macOS, iOS and the web, rather than transparency |
| The rest of WGSL | `shaders.md` (Validation and limits) | `var<private>` globals, structs of the app's own, non-square matrices and the bit-counting, packing, `modf` and `frexp` built-ins compile into `shaderir` |
| Backends linking only what an app shows | — | A backend's shaders, canvases, videos, web views and native components link only into apps that make one, as `core`'s elements do ([what an app links](architecture/layers.md#what-an-app-links)), and Android's DNS resolver only into apps with Go networking of their own |
| Effects on Linux checked by pixels | — | The gallery's `TestShaderEffects` screenshots show the effects under `nsgo test -target linux`; it passes there, but its captures of the Shader page come out blank, effects switched off too, while `HEAD`'s capture of that page did not (2026-11-08) |
| Effects on an iPhone | — | The gallery's `TestShaderEffectsPhone` passes under `nsgo test -target ios_simulator` and on a device; gallery tests there skip as they start (2026-11-08) |
| Transitions beside animated values | — | A view with `Style.Animated` bound still runs its `Transition` for the properties not bound on macOS, iOS and the web, as Windows and Linux mix them; `Toast` keeps its translation on a view of its own until then |
| Touchpad glides over hosted views on Linux | — | A touchpad scroll that reaches the surface through a hosted view's shield, whose events no scroll controller sees, glides on as the surface's own do |
| Roles, states and font lists on Android | `elements.md`, `text.md` | `NsgoView` describes the roles NSH 0.4.0 added (`listbox`, `option`, `combobox`, `group`, `region`, `navigation`, `tree`, `treeitem`, `toolbar`, `menuitemcheckbox`, `menuitemradio`) and the pressed and busy states, and a `fontFamily` list falls back through its families with `Typeface.CustomFallbackBuilder` rather than setting the first, which needs `classes.dex` rebuilt |
| Font lists on Windows | `text.md` | A character the first family of a `fontFamily` list lacks comes from the next that has it through an `IDWriteFontFallback`, rather than from the system's fallback, as elsewhere |
| Table headers on phones | — | A `DataTable`'s sort control takes a tap in the iOS simulator, as it does on desktops and the web; the gallery's TSX `TestDataTableSorts` sees none there (2026-11-08) and runs on desktops alone |
| Roving focus on the other backends | `elements.md` (Keyboard focus); #1521 | Windows and Linux move the focus between views as macOS and the web do, so `Focusable`, `Tabs`' and `RadioGroup`'s stops and their arrows, and a `Plot`'s keys, work there |
| The mouse on Android | `api-naming.md` (`onMouseEnter`) | `NsgoView` reports the mouse's enter and leave with where it is, its moves with the keys and the button held, and a move with a button down, which needs `classes.dex` rebuilt |
| Text selection reports on Android | NSGo's own (`text.md`) | `Selection.java` reports `EventSelectionChange` with the box of the texts selected to an area with `OnTextSelection`, as the other platforms do, with `classes.dex` rebuilt, so a `DiffView`'s frame follows the selection there |
| The system's text services on selections elsewhere | NSGo's own (`text.md`); #1516, #1517 | A selection offers what the platform's text views offer beside Copy, as a selectable text's does on iOS: Look Up, Translate and Services in a secondary click's menu on macOS, and the apps answering `ACTION_PROCESS_TEXT`, Translate among them, in Android's floating toolbar, with `classes.dex` rebuilt |
| Android's runtime of 2026-11-08 on a device | — | A DataTable scroll at a fractional density sends a few items between window shifts on a device, and slowed runs keep their rate there, with the `classes.dex` that starts each view in the default style the first batch carries and holds runs to `AtFrameRate`; the gallery's Android tests pass with it on an x86_64 emulator at 2.625 as before it (2026-11-08) |
| Rounded borders on Android | NSGo's own (`styles.md`) | `Box` stops the fill under the border with `ViewStyle.Rim`, as the other backends do, so the fill stops showing past a rounded border, with `classes.dex` rebuilt |
| Pasted HTML | `api-naming.md` (`PasteEvent`) | `onPaste`'s `html` holds the clipboard's HTML on Windows (`CF_HTML`), Linux (`text/html`) and Android (`ClipData.Item.getHtmlText`), as it does on macOS, iOS and the web |
| Sharing on Windows, Linux and the web | — | `Share` shows Windows' share UI (`IDataTransferManagerInterop::ShowShareUIForWindow` with the files as `StorageFile`s), the desktop portal's or a Linux file manager's where one exists, and the web's `navigator.share` for text and files the app holds as bytes, rather than answering `NoShareSheet` |
| Photos on the desktops | `services.md` (Taking a photo) | `TakePhoto` shows a camera on macOS (an `AVCaptureSession` sheet, or Continuity Camera's import from an iPhone), Windows (`CameraCaptureUI`) and Linux (the desktop portal's `Camera` with a PipeWire stream), rather than failing with `ErrNoCamera` |
| Photos in TSX tests | — | `nsgo/test` answers `Picker.takePhoto` as it answers `pickImages` (`onPick`), with an environment that has a camera |
| Recording on the other platforms | — | `microphone.Capture` records on Windows, Linux and Android (`AudioRecord` and `RECORD_AUDIO`, with `classes.dex` rebuilt), as it does on macOS and iOS |
| Recording beside video on iOS | — | A video that starts playing with an audio session of its own keeps a recording or a speech session going, which now loses the session to it, as `audiosession_ios.go`, `speech_ios.go` and `microphone` share the `AVAudioSession` without knowing of each other |
| Speech checked with a voice | `sandbox.md` (`speech`) | A session on a Mac and an iPhone, the permissions granted, reports the transcript of what is said; so far it was checked headless and as far as its refusal on a Mac whose session grants none (2026-11-08) |
| Speech on the other platforms | — | `StartSpeech` recognizes on Android (`SpeechRecognizer`, with `RECORD_AUDIO` and `classes.dex` rebuilt), Windows (`Windows.Media.SpeechRecognition`) and the web (`SpeechRecognition` where the browser has it), rather than ending `unavailable` |
| Web view downloads elsewhere | `macos.md` | A web view with `OnDownload` saves what its pages download on Windows (WebView2's `DownloadStarting`), Linux (WebKitGTK's `download-started`) and Android (`DownloadListener`, with `classes.dex` rebuilt), as Apple's do |
| Trusted web input elsewhere | — | `Ref.SendInput` reaches the page as trusted input on Windows (WebView2's `SendMouseInput` and key messages), Linux and iOS, which drop it now |
| The lifecycle of embedded views on Windows | `windows.md` (Environment) | A view embedded in another app's window follows that root window's minimize and restore, subclassed as NSH's `observe_root` does: it now reads them only as its own thread's activation changes |
| The lifecycle on a Mac's screen | — | `Env.Lifecycle` is checked to read active and inactive on a Mac whose screen shows the app, and background as it is hidden or minimized: in this machine's session, whose screen is locked, AppKit reports the app occluded (2026-11-08) |
| Levels and spin buttons | `elements.md` | `View.AccessibilityLevel` reaches the screen readers of macOS, iOS, Windows, Linux and Android as it does the web's `aria-level`, a `NumberInput`'s `RoleSpinButton` holds on the native text fields of iOS and Android, which take the role from their kind, and Android's `NsgoView` knows the role, with `classes.dex` rebuilt |
| Bars in multiline inputs | `components.md` (Scroll bars) | A multiline `Input` reports its text's offset and takes `ScrollTo` on every backend, so `scrollbar={theme}` draws a `Scrollbar` over its text |
| Modal reports | `elements.md` (`<Dialog>`) | Every backend reports a modal shown once it is presented and gone once it is dismissed (`EventModalShown`, `EventModalDismissed`), and the insets a screen keeps clear of (`EventScreenInsets`), so `Dialog`'s `OnOpen` and `OnClose` follow the platform instead of the animation's timer |
| Eyedroppers on the other desktops | `components.md` (`ColorEyeDropper`) | `Scope.PickScreenColor` works on Windows and Linux, where `ColorEyeDropper` is disabled, as NSH's is, or stays disabled by decision |
| Real file drags | `file-drop.md` | A real drag from Finder and from a Linux file manager is checked to reach a view, as one from Explorer is |
| Long presses on Android spans | `text.md` (nested texts) | A long press on a text's span reaches the span's `OnLongPress` on Android, as it does on the other backends: the runtime's Java reports the character under it, which needs `classes.dex` rebuilt |
| Ink past a text's box | `styles.md` (text) | A text's decoration lines and shadow that reach past its box are checked to draw on Android, Windows and Linux, as they do on Apple and the web |
| Android blurs | `styles.md` (`backdropFilter`) | From Android 12, a backdrop filter blurs what the view's parent drew under it through a `RenderNode` and a `RenderEffect`, and a scroll view's soft and hard edges ([scrolling](architecture/scrolling.md#edge-effects)) blur the content under its bars |
| Hit tests through transforms | — | A press, hover or wheel lands on the view a transform shows under the pointer on every backend and in the core's hit test, not where its frame lies, so what a flow diagram's node holds takes the pointer |
| Sharp zoom beyond Apple | — | A view scaled up draws its texts and canvases again at its scale on Android, as Apple's do in steps (`apple.RasterStep`); Windows, Linux and the web draw at the transform already |
| Wheels checked on the other platforms | — | `OnWheel` and a pinch's `Position` are checked with real input on iOS, Android, Windows, Linux and the web as on macOS (2026-11-08), and an Android touchpad's momentum glides on |
| System headers on Android | — | `ui.NavPage` shows the system's transparent header on Android too, as on iOS, once scroll edge effects draw there |
| Web glass | — | Liquid Glass draws through the glass shader on the web where Chrome's HTML-in-Canvas draws a whole app, its `<canvas>` elements included, without hanging ([styles](architecture/styles.md)) |
| Web views under overlays on macOS | — | A `WKWebView` covered by an overlay is checked to take no hovers or cursor from its own tracking area, and stopped from taking them if it does, as inputs' editors are ([cursors](architecture/interaction.md#cursors)) |
| The system's words in a forced language elsewhere | — | What Android, Windows and Linux say in the app themselves, such as text selection menus, file dialogs and GTK's widgets, follows the language `SetLocale` forces, as it does on macOS and iOS from the next launch ([languages](architecture/languages.md#the-tool)): Android 13's `LocaleManager.setApplicationLocales`, Windows' `SetThreadPreferredUILanguages` |
| Native direction on devices | — | The app's direction ([languages](architecture/languages.md#which-language)) is checked to mirror the native controls on an iPhone, an Android device, Windows and Linux, where it was only built (2026-11-04), and on macOS, where headless tests ran |

## TSX and dynamic delivery

What TSX still lacks of NSH's ([TSX and dynamic delivery](architecture/tsx.md)).

| Item | NSH reference | Done when |
| --- | --- | --- |
| A heap limit | `sandbox.md` (Limits) | A sandboxed module that keeps or allocates past `Limits.Memory` stops as one past its node quota does, on goja (no other engine, decided by the user, 2026-11-09); until then a package's root is trusted not to exhaust memory |
| A web view's navigations on the web | `sandbox.md` (Enforced in Rust: top-level web view navigations) | A sandboxed web view's page on the web navigates only to the hosts `webview` grants, as on the other targets |
| Updates in TSX | `updater.md` | TSX apps import `nsgo/updater`'s `checkForUpdate`, as Go apps call `updater.Check`, on the same platforms' updaters |
| OTA updates | `updater.md` (OTA updates) | A release changing only an app's TSX reaches installed apps, iOS and Android included, as a signed package `nsgo build` lists in the appcast: it runs on trial from the next start, becomes current once healthy and is dropped after a crash, never wider than the app (left for later by the user, 2026-11-10) |
