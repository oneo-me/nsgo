# Roadmap

This plan lists what NSGo still lacks of NSH's 0.4.0 release (tag `v0.4.0`, `e626a8dd`), in the order to build it. Each item names its NSH reference and the result that finishes it. What exists is in [ARCHITECTURE.md](../ARCHITECTURE.md), and what NSGo leaves out on purpose in [NSGo and NSH](architecture/nsh.md#what-nsgo-leaves-out). Design questions that block an item are in [Open questions](../ARCHITECTURE.md#open-questions).

References are files in `nsh/docs/dev/` unless they give a path from `nsh/`, NSH's checkout beside the repository, with the section to read in brackets. [Following NSH's releases](architecture/nsh.md#following-nshs-releases) says how to rebuild this plan for NSH's next release.

## NSH's apps on NSGo

NSH's own apps and tests are the measure of alignment: each runs with its imports renamed from `nsh` to `nsgo`, and `go test ./tsx` runs its gallery, the gallery's tests, its examples with theirs, and its compat cases (`TestNSHGallery`, `TestNSHGalleryTests`, `TestNSHExamples`, `TestNSHCompat`). On 0.3.0 all of them passed but what needs the standard modules. Against 0.4.0 on 2026-11-06, 69 of the 173 compat cases, 12 of the 15 examples and 56 of the 353 gallery tests passed, and none of the gallery's 29 demos opened: its shell reads `process.env` as it starts, which waits for its [open question](../ARCHITECTURE.md#open-questions). The sections below are what the rest fail on, each closed by the cases it names. `.nsgo/types` then exports the type names 0.4.0's `.nsh/types` does.

## The 0.4.0 API: styles

Done when `props-text-lines`, `props-view-touch`, `props-media-image`, `props-media-video`, `props-input-config`, `core/steps`, `paint-borders`, `paint-current-color`, `paint-misc`, `paint-shadows`, `paint-tailwind`, `paint-text`, `state-expanded`, `state-focus-within` and `state-pressed` pass.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Props as style keys | `styles.md` (Principles, Extensions) | `numberOfLines`, `textOverflow`, `userSelect`, `pointerEvents` and `objectFit` are style keys and classes as CSS has them, inline winning over classes; transforms read CSS strings |
| States and variants | `styles.md` (variants) | `aria-expanded:`, `aria-pressed:`, `aria-busy:`, `aria-checked:`, `aria-selected:` and `data-<name>:` variants and their style props (`expandedStyle`, `pressedStyle`, `focusWithinStyle`); `focus-within:` follows any focus, popups' included, kept by the core per surface; parts show `data-*` attributes |

## The 0.4.0 API: the rest

Independent of each other, done in any order. Done when the `layout-*`, `core-theme`, `props-view-accessibility`, `props-list-follow`, `web-fetch`, `base-radio`, `base-tabs`, `base-switch` and `base-button` cases pass.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Roles and focus | `elements.md`, `api-naming.md` (keys of `RadioGroup` and `Tabs`) | The `listbox`, `option`, `group`, `region`, `tree` and `menuitemcheckbox` roles and the pressed state; `focusable` stops without presses; a group of radios or tabs is one Tab stop its arrows move through, swapped right to left; `hitSlop` on `Button`, `Checkbox`, `Radio`, `Switch`, `Toggle` and `Link` |
| Errors with codes | `api-naming.md`, `sandbox.md` (What a refusal does) | What JS calls fails one way with a stable `code` (`ERR_ACCESS_DENIED`, `ERR_FEATURE_UNAVAILABLE_ON_PLATFORM`, `ERR_NSH_QUOTA_EXCEEDED`), with `permission` and `resource` where a capability refused it |
| Fonts and files | `text.md`, `images.md` | Apps import WOFF and WOFF2 fonts; `fontFamily` lists fall back in order; italic runs keep color emoji upright; `fetch` reads the files the app ships (`asset:/`); `svg()` draws an SVG document as authored |

## Components of 0.4.0: controls

What `nsh/base` added in 0.4.0 (`components.md`, `nsh/crates/nsh-base/src`), each in `base` with no look and in `ui` with its look. These share the roving focus and keys of [the rest](#the-040-api-the-rest). Done when `base-slider`, `base-number-input`, `base-otp-input`, `base-toggle`, `base-toolbar`, `base-pagination`, `base-accordion`, `base-collapsible-reveal`, `base-link`, `base-avatar`, `base-tree`, `base-carousel` and `base-scrollbar` pass.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Value controls | `slider.rs`, `number_input.rs`, `otp_input.rs`, `toggle.rs` | `Slider` (a value or a range), `NumberInput`, `OtpInput` in one native field, `Toggle` and `ToggleGroup` |
| Structure and navigation | `components.md` (Carousels), `toolbar.rs`, `pagination.rs`, `accordion.rs`, `tree.rs` | `Accordion` and a `Collapsible` that reveals its height as it opens, `Toolbar`, `Pagination`, `Tree` in a virtual list, `Carousel` on a native scroll view that can loop, `Link`, `Avatar` and `AvatarGroup` |
| Scroll bars of the app's | `scrolling-and-lists.md`, `components.md` (Scroll bars) | `Scrollbar` names a scroll view or list and draws its bar in the app's look on every backend, `Scrollbar.setTheme` replaces every view's indicator, and the DataTable draws it above its rows |

## Components of 0.4.0: popups

On `base`'s popups ([components](architecture/components.md)). Done when `base-select`, `base-combobox`, `base-command`, `base-hover-card`, `base-date-picker`, `base-color-picker`, `base-menu-checked`, `base-popups`, `base-dialog` and `base-dialog-backdrop` pass, with the `browser/*-desktop` ones once the gallery's [open question](../ARCHITECTURE.md#open-questions) is answered.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Pickers | `components.md` (Popups), `select.rs`, `combobox.rs`, `date_picker.rs`, `calendar.rs`, `time_field.rs`, `color_picker` | `Select`, `Combobox` (one value or several, typed into its trigger), `DatePicker`, `Calendar` and `TimeField`, picked in one tap on phones, and `ColorPicker` (area, channel sliders, fields, swatches, mirrored right to left) |
| Palettes and cards | `components.md` (Popups), `command.rs`, `hover_card.rs` | `Command`'s palette filtering natively and `HoverCard`, open while the pointer rests or the focus is in it |
| Menus and dialogs | `components.md` (Popups), `menu.rs` | Menu items checked with the app's mark, a menu bar whose menus open on hover, popups and dialogs that report an open change, with its reason, before they open or close |

## Components of 0.4.0: plots

Independent of the other components. Done when `base-plot` passes and the gallery's Charts page shows each mark.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Marks | `components.md`, `nsh/crates/nsh-base/src/plot` | `Bar` (stacked, grouped, per-row bases), `Pie` and `Arc`, `PlotAxis`, `PlotLabel`, `RadialLine` and `RadialGrid`, `Sankey`, `Cell`, `Dot`, `Rule` and `Treemap` |
| Scales and motion | `nsh/crates/nsh-base/src/plot` (`scale.rs`, `tween.rs`, `appear.rs`) | An ordinal `colorScale`, reversed and second y scales, domains that glide (`motion.update`) and marks that appear once (`PlotAppearScope`), the hover moved by keys and screen readers |

## Components of 0.4.0: the rich editor and the DataTable

Each a project of its own.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Rich editor | `rich-editor.md` | `RichEditor`, a ProseMirror-style document with lists, mentions, tables (merged cells, resizing, reordering), a bubble menu, undo grouped by time and input methods composing in the line; the `base-rich-editor-*` cases pass |
| The DataTable of 0.4.0 | `data-table.md` | A body that takes no pointer under a hit layer of scroll views, rows and scrolling columns as slots reused for the rows in reach, a bounded extent past what 32-bit offsets hold, one `onSelectionChange` and Rust-made rows (`DataTableRows` from columns); `base-data-table` passes |
| Editor and diff | `editor.md`, `diff.md` | The Editor's gutter and overview ruler decorations, the DiffView's scroll position and change positions reported; `base-editor` and `base-diff` pass |

## Shaders and transitions

NSH 0.4.0's `Shader` draws a box with a WGSL `shade(uv)` (`shaders.md`); NSGo's shaders are Go ([shaders](architecture/shaders.md)).

| Item | NSH reference | Done when |
| --- | --- | --- |
| WGSL for TSX | `shaders.md` (The contract, Validation and limits) | A TSX `Shader` compiles its WGSL into `internal/shaderir`, with NSH's built-ins, uniforms and limits, so `core-shader` passes on every backend |
| Effects and transitions | `shaders.md` (Effects, transitions and pictures) | `ShaderEffect` draws a view's children through a shader, `ShaderTransition` changes them, and `ScreenEffect` and `ScreenTransition` cover the screen, picturing children only when they changed, as NSH's backends do |
| Pages and dialogs | `shaders.md` (Pages and dialogs) | Pages a stack pushes and pops and dialogs that open and close come and go through a shader, with `core-shader-effects` and `base-transition-shader` passing |

## TSX and dynamic delivery

What TSX still lacks of NSH's ([TSX and dynamic delivery](architecture/tsx.md)). Signed packages and capabilities wait for their [open question](../ARCHITECTURE.md#open-questions), and running TSX on the browser's engine for its own.

| Item | NSH reference | Done when |
| --- | --- | --- |
| TSX on the other targets | `tooling.md` (Dev server, `nsh test`) | `examples/tsx` updates in place on a save under `nsgo dev` on the web, Windows, Linux, the iOS simulator and Android, and passes its Go and TSX tests under `nsgo test -target` on the last four, as it does on macOS, and for its tests on the web, since 2026-11-04 |
| TSX on the browser's engine | `web.md` (The browser's JavaScript engine) | On the web a TSX app runs as ES modules on the page's own JavaScript engine, in an iframe realm per run, rather than on goja compiled to WebAssembly |
| Signed packages | `signed-packages.md` | `nsgo keygen`, `certify`, `publish` (signing a package again with another release key too), `verify` and `revoke` work; a Go, iOS and Android host verifies a package, runs it in `tsx.View`, and refuses tampered, revoked and older builds |
| Capabilities and limits | `sandbox.md` | Every request a capability names (`fetch`, remote images, web views and their trusted input, `openUrl`, the clipboard, the picker, speech, host calls) is refused with `ERR_ACCESS_DENIED` without it. Quotas cut a module's nodes, timers, host calls and requests, a host sets a view's limits, and `nsgo dev -limits` runs an app under them |

## Platform services of 0.4.0

| Item | NSH reference | Done when |
| --- | --- | --- |
| Speech | `sandbox.md` (`speech`), `nsh`'s `Speech` | `Speech.start` sessions recognize speech on macOS and iOS and report their transcript |
| The app's lifecycle | `nsh`'s `lifecycle()` (NSH `#747`) | `lifecycle()` reads, reactively, whether the app is `active`, `inactive` (visible without the focus) or in the `background`, which every backend reports |
| Clipboard images as URLs | `api-naming.md` (Clipboard) | `readImage` promises a URL and rejects what cannot be read, on every backend (`mod-clipboard` passes) |
| Web views of 0.4.0 | `macos.md` | macOS saves the downloads a web view's app opts into (`onDownload`, which no backend reports yet) to the app's data directory, and web views take trusted input (`sendInput`), which only a declared capability grants |

## Performance against NSH's baselines

NSH records what its hot paths cost (`data-table.md` (Performance), `performance.md`, `web.md`), and NSH's own build on this machine is the measure: each item runs one scenario on both galleries ([performance](architecture/performance.md#keeping-frames-cheap) says how). Against 0.3.0 the DataTable kept up with NSH's headless, in headless Chrome, with no task over 50 ms, and on macOS, its cells moving with the content in each frame on every backend.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Device pixels | `performance.md`, `geometry.md` | Layout rounds to device pixels rather than whole points, frames that stay on their pixels are not sent again, and a subtree that moved by whole pixels skips its frames, as NSH's `#557`, `#617` and `#637` do |
| 0.4.0's baselines | `data-table.md` (Performance), `performance.md` | The DataTable's scroll (a frame crossing no row 4 items in ~1.2 ms, a few rows ~35–50 in ~2 ms, no views made), its tick and the Animations page cost what NSH 0.4.0's build does on the same machine, headless, in headless Chrome and on macOS |
| The other platforms | `data-table.md` (Performance) | The DataTable's scroll, still and ticking, and a stack's slides cost on Windows, Linux, iOS and Android what NSH's do there (Pixel 10 Pro: ~870 views, commits at 3 and 7 ms at the median and 90th percentile, slides on whole pixels), measured on those machines and on NSH's builds for them |
| Android's new views | `nsh/AGENTS.md` (Performance rules, 4) | `NsgoView` starts a node in the default style with no props and no events, and `classes.dex` is rebuilt, so the Go side stops sending the three for every new view (`withDefaults`) |

## Motion

What components still lack to move as their parts come, go and travel (#19), which NSH lacks as well (`animation.md`, the table beside gpui-base's).

| Item | Reference | Done when |
| --- | --- | --- |
| Exits | NSH's gap for gpui-base's `Presence`; Motion's `AnimatePresence` | A region like `Show` keeps what it held after its condition turns false until an exit the app plays ends, so a sheet's or a toast's parts leave with their motion without the component tracking it |
| Shared elements | SwiftUI's `matchedGeometryEffect`, Motion's `layoutId` | Views a commit removes and inserts under one match ID count as one: the new view moves from the frame the old one showed, with the commit's layout animation |
| Resisting drags | `RubberBandClamp` | A node resists a value past its bounds on the frame clock, so a drag bound with `PanValues` slows at its ends without a commit per move |

## Application and windows

What Windows and Linux still lack of what macOS opens for an app ([application and windows](architecture/application.md)): there `OpenWindow`, `OpenPanel` and `NewTray` return nil, and the main view fills the one window. Avalonia's desktop lifetime is the reference.

| Item | Reference | Done when |
| --- | --- | --- |
| Surfaces on Windows and Linux | macOS's `own` surface | The backends run the app on a surface in no window and attach each one the app opens: windows, and panels that take the keyboard and hide as they lose it |
| Window limits on Linux | `XSetWMNormalHints` on X11, `xdg_toplevel.set_max_size` on Wayland; `GtkHeaderBar`'s decoration layout | GTK 4 reaches neither from its API: a maximum other than a fixed size holds, and `NoMinimize` and `NoMaximize` take the system title bar's buttons |
| Tray icons on Windows | `Shell_NotifyIcon` | An icon shows its image, its panel above or under it (`Shell_NotifyIconGetRect`) and its menu; a drawn icon is an element rasterized into the icon whenever it changes |
| Tray icons on Linux | StatusNotifierItem over D-Bus | KDE and GNOME with its AppIndicator extension show the icon and its menu; GNOME without one shows none |
| Global shortcuts on Windows and Linux | `RegisterHotKey`; `XGrabKey`, the GlobalShortcuts portal | A shortcut shows a panel from any app, on Wayland once the user grants it |
| One instance on Windows and Linux | A named mutex and pipe; a Unix socket | A second launch hands its arguments to the running app, which hears `OnReopen` |
| Launch at login on Windows and Linux | The `Run` registry key; an XDG autostart entry | `SetLaunchAtLogin` starts the app at login and `LaunchAtLogin` reads it |

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
| Links | `link-url`, OSC 8 | Command+click (Control+click elsewhere) opens hyperlinks and URLs and paths printed as text, with a hand over them |
| Clipboard protocols | `clipboard-read`, `clipboard-write` | Programs copy with OSC 52 and read the clipboard as the app allows, and the Kitty clipboard protocol offers pasted images |
| Keybinds, zoom, the context menu and secondary clicks | `keybind`, `right-click-action` | Ghostty's actions bind to keys over the defaults, zoom changes the font's size, and a secondary click shows Copy, Paste, Select All and Clear |
| Drawn box characters, blinking and contrast | `cursor-style-blink`, `minimum-contrast`, `window-padding-color` | Box drawing, blocks and Powerline's separators are drawn so lines join, the cursor blinks, and minimum contrast and padding colors apply |
| Search, images and shell integration | Find, the Kitty graphics protocol, `shell-integration` | A search highlights matches, programs show images, and OSC 133 prompts take Option+click to move the cursor |
| TSX | — | TSX apps import the terminal as `tsx/editor` registers `Editor` |

## Finish the existing platforms

These items wait for community contributions, and a change to one backend records the gaps it leaves here.

| Item | NSH reference | Done when |
| --- | --- | --- |
| The mouse on Android | `api-naming.md` (`onMouseEnter`) | `NsgoView` reports the mouse's enter and leave with where it is, its moves with the keys and the button held, and a move with a button down, which needs `classes.dex` rebuilt |
| Pasted HTML | `api-naming.md` (`PasteEvent`) | `onPaste`'s `html` holds the clipboard's HTML on Windows (`CF_HTML`), Linux (`text/html`) and Android (`ClipData.Item.getHtmlText`), as it does on macOS, iOS and the web |
| Folder picks on phones and the web | — | `PickDirectories` answers with folders the app reads on iOS (`UIDocumentPickerViewController` for `.folder`, kept in its security scope), Android (`ACTION_OPEN_DOCUMENT_TREE`, which grants a tree URI, not a path) and the web (`showDirectoryPicker`, or `<input webkitdirectory>`), which need a `Directory` that opens what it holds without a path, as `File.Open` does |
| Sharing on Windows, Linux and the web | — | `Share` shows Windows' share UI (`IDataTransferManagerInterop::ShowShareUIForWindow` with the files as `StorageFile`s), the desktop portal's or a Linux file manager's where one exists, and the web's `navigator.share` for text and files the app holds as bytes, rather than answering `NoShareSheet` |
| Modal reports | `elements.md` (`<Dialog>`) | Every backend reports a modal shown once it is presented and gone once it is dismissed (`EventModalShown`, `EventModalDismissed`), and the insets a screen keeps clear of (`EventScreenInsets`), so `Dialog`'s `OnOpen` and `OnClose` follow the platform instead of the animation's timer |
| Real file drags | `file-drop.md` | A real drag from Finder and from a Linux file manager is checked to reach a view, as one from Explorer is |
| Long presses on Android spans | `text.md` (nested texts) | A long press on a text's span reaches the span's `OnLongPress` on Android, as it does on the other backends: the runtime's Java reports the character under it, which needs `classes.dex` rebuilt |
| Ink past a text's box | `styles.md` (text) | A text's decoration lines and shadow that reach past its box are checked to draw on Android, Windows and Linux, as they do on Apple and the web |
| Android blurs | `styles.md` (`backdropFilter`) | From Android 12, a backdrop filter blurs what the view's parent drew under it through a `RenderNode` and a `RenderEffect`, and a scroll view's soft and hard edges ([scrolling](architecture/scrolling.md#edge-effects)) blur the content under its bars |
| Progressive edges before iOS 26 | — | Before iOS 26 a soft scroll edge blurs by a radius that grows toward the side through a `CABackdropLayer`'s variable blur, as macOS's does ([scrolling](architecture/scrolling.md#edge-effects)), rather than fading in an even blur |
| Web glass | — | Liquid Glass draws through the glass shader on the web where Chrome's HTML-in-Canvas draws a whole app, its `<canvas>` elements included, without hanging ([styles](architecture/styles.md)) |
| Web views under overlays on macOS | — | A `WKWebView` covered by an overlay is checked to take no hovers or cursor from its own tracking area, and stopped from taking them if it does, as inputs' editors are ([cursors](architecture/interaction.md#cursors)) |
| Native direction on devices | — | The app's direction ([languages](architecture/languages.md#which-language)) is checked to mirror the native controls on an iPhone, an Android device, Windows and Linux, where it was only built (2026-11-04), and on macOS, where headless tests ran |
