# Services

Services are what the platform does for the app outside its views: alerts, the clipboard, opening links, the photo picker, the appearance, keyboard shortcuts, the status bar, the desktop window and the app's lifecycle. Read this before changing one, in `core` or in a backend.

## Requests and answers

A service is a command item in the batch. One that answers, such as a clipboard read, an opened URL or a pick, carries a number in `Count` and comes back as an event with it.

- `Tree.request` keeps the callback under the owner that asked, and the root runs it when that owner is gone, so a closed menu still hears its answer.
- Platforms answer on any goroutine through `Engine.Dispatch`; slow work, such as encoding a photo, runs off the main thread first.
- `nsgotest` answers from its model: `App.Clipboard`, `ClipboardImage` and `ClipboardFiles`, `OpenURL`, `PickImages`, and it records the appearance, quits and the context menu shown.

## Clipboard

`SetClipboard` and `SetClipboardImage` write; `ReadClipboard`, `ReadClipboardImage` and `ReadClipboardFiles` read; `HasClipboardText` and `HasClipboardImage` read only what the clipboard holds, so iOS and Android show no paste notice for them.

- An image is read as a PNG and its size in pixels, none over 48 MB, as NSH's. One beside copied files is their icon, not the user's image, and reads as none.
- Files are the `file:` URLs a file manager copied. Android and iOS rarely hold any, and the web has no access to them.

| Target | Writes an image as | Reads an image from |
| --- | --- | --- |
| macOS | PNG and TIFF | PNG, TIFF, JPEG, HEIC or GIF, converted by ImageIO |
| iOS | PNG | The same types, else `UIPasteboard.image` |
| Windows | `PNG` and a DIB over white | `PNG`, else the DIB through `x/image/bmp` |
| Linux | A `GdkTexture` | A texture, saved as PNG |
| Android | A PNG `ClipProvider` serves | An image URI, decoded and encoded as PNG |
| Web | The PNG browsers take | `image/png`, with the page's permission |

Android's clipboard holds images only as content URIs, so the runtime declares `ClipProvider` with an authority named after the app's package. It is unexported and grants URIs, so only the app pasting reads the image. NSH writes no images on Android.

## Opening URLs

`OpenURL` refuses what `CheckURL` refuses, as NSH's `check_url` does: a URL without a scheme, over 16 KiB, with spaces, a web address without a host, and `javascript:`, `data:`, `blob:`, `file:`, `about:` and `asset:`. An embedding host's handler (`Host.HandleOpenURL`, [embedding](embedding.md#the-host-channel)) takes a URL before the platform.

| Target | Opens with | Answers false when |
| --- | --- | --- |
| macOS | `NSWorkspace openURL:` | No app handles it |
| iOS | `UIApplication openURL:options:completionHandler:` | The completion says so |
| Android | `Intent.ACTION_VIEW` | No activity handles it |
| Windows | `ShellExecuteExW` without UI, after the scheme is found under `HKEY_CLASSES_ROOT` | The scheme is unregistered |
| Linux | `GtkUriLauncher`, through the desktop portal | Launching fails |
| Web | A new tab for web pages, a followed link for other schemes | A popup blocker keeps the tab from opening |

Windows checks the registry first because the shell asks the user for an app for an unknown scheme, even without UI.

## Photo picker

`PickImages` resolves `PickOptions` into a `PickRequest`, as NSH's `PickImagesOptions`: one image, or up to `Limit` of at most 100.

- `PickRequest.Encoding` decides each image: kept as it is, or decoded upright, scaled to `MaxDimension` and encoded as PNG (a PNG scaled) or JPEG.
- By default HEIC and other formats browsers do not show become JPEG; `PickCurrent` keeps them. A quality encodes JPEGs again.
- `FinishPick` trims to the limit, sniffs missing types and names files after their type (`IMG_0042.HEIC` as JPEG is `IMG_0042.jpg`).

| Target | Picker | Prepared by |
| --- | --- | --- |
| iOS | `PHPickerViewController`, ordered, no permission | ImageIO |
| macOS | An `NSOpenPanel` sheet for any image | ImageIO |
| Android | The Photo Picker from API 33 (30 with its extension), else the documents UI | `ImageDecoder` in `Services.java` |
| Windows | `GetOpenFileNameW`, from a message of its own | `internal/platform/picked` |
| Linux | `GtkFileDialog` | `internal/platform/picked` |
| Web | `<input type=file>`, which needs the press's user activation | `internal/platform/picked` |

`picked` decodes in Go, which reads no HEIC, so those pickers offer only the formats it reads unless `PickCurrent`. It turns JPEGs upright by their EXIF orientation and scales with Catmull-Rom.

## Appearance and quitting

`SetAppearance` forces light or dark, and `nsgo.Appearance` starts the app in it. The core keeps the system's appearance beside the forced one and reports the one shown as `Env.Dark`, so platforms may report either. A subtree forces its own with `SetSubtreeAppearance` ([themes](themes.md#scope)).

| Target | Native chrome that follows |
| --- | --- |
| macOS | `NSApp.appearance`: title bar, menus, alerts and hosted controls |
| iOS | The window's `overrideUserInterfaceStyle` |
| Android | System bar icons, alerts, the activity's theme, system headers and web views (forced dark from Android 10) |
| Windows | DWM's dark title bar and uxtheme's menu mode (ordinals 135 and 136); alerts are drawn in the appearance shown |
| Linux | `gtk-interface-color-scheme` from GTK 4.20, `gtk-application-prefer-dark-theme` before |
| Web | The page's `color-scheme` |

`Quit` ends the app. On desktops `nsgo.Run` returns once the engine stopped and storage was written, as NSH's `run_app`: macOS cancels `applicationShouldTerminate:` and stops AppKit's loop, posting an event to wake it. Android finishes its task; iOS apps never quit themselves. `mobile.ExitApp`, NSH's `BackHandler.exitApp`, does what Android's back does when nothing in the app takes it (`ItemBackUnhandled`): the task's root goes to the background, another activity finishes.

`nsgo.KeepAliveOnClose` keeps a macOS app running when its window closes: the window orders out, and the Dock's reopen shows it again. Windows and Linux end with the window, having no Dock to reopen from.

## Desktop windows

Package `desktop` is NSH's `nsh/desktop`: native controls, an app-drawn title bar, and the menus below. Its functions call the `Scope` methods of the same names, the core's part.

- `SetTitleBar` hides the system title bar and gives the app's bar its height, in which the platform centers its controls. `TitleBar` then reports the area they keep, and views with `Desktop.WindowDrag` drag the window.
- macOS centers the traffic lights in the bar, 16 points from the left, as Onevra does; Windows draws Onevra's rounded caption buttons; Linux reports the bar's height and the app draws its buttons.
- `SetWindowControls` hides those controls for the app's own, or moves the traffic lights, as MyGo's `TrafficLightPosition` does: the close button's top left from the window's, zero X or Y keeping the default.
- macOS 27's AppKit puts the traffic lights back whenever it lays the title bar out, without a notification, so NSGo's `NSWindow` subclass places them after each `layoutIfNeeded`. Hiding and showing them makes AppKit place them again once the app's position goes, as Electron's `window_buttons_proxy.mm` notes.
- `Maximized` reports a maximized or zoomed window, so an app-drawn maximize button can show restore.
- The platform decides a window drag on the main thread from the mounted props: the nearest view setting `WindowDrag` decides, and a nearer pressable, input or gesture view keeps the press.
- `nsgo.WindowMaterial` starts the window with a material behind the app's views and `SetWindowMaterial` changes it: an `NSVisualEffectView` on macOS, Mica, Acrylic or Mica Alt on Windows 11 22H2 and later. `ShowsMaterial` reports whether one shows; elsewhere the window stays opaque.
- Windows shows a material only in a window started with one: its content needs DirectComposition and `WS_EX_NOREDIRECTIONBITMAP`, which holds from the window's creation. Such a window changes its backdrop, or turns it off, any time.
- `SetWindowFrame` moves and sizes the window in points from the main screen's top left, keeping the top edge without a move.
- macOS converts the frame to its bottom-left one and animates it; Windows takes the window's pixels; Linux sets GTK's default size and leaves placement to the compositor.
- `SetWindowLevel` floats a window (`NSFloatingWindowLevel`, `HWND_TOPMOST`) or sinks it to the desktop's icons on macOS, on every space, or to the bottom of the z-order on Windows. `OrderFront` restores, raises and activates.

## Menus

`SetApplicationMenu` and `PopupMenu` send menus as data, handlers replaced by IDs that come back in `EventMenu`. Role items are the platform's own and are never reported. The default menu returns when the owner that set the menu goes.

- `SetMenuBar(MenuBarAutoHide)` hides the bar Windows and Linux draw until Alt is pressed and released alone, which is kept from Windows' menu mode. Alt with a mnemonic or F10 shows it for the menu they open; a press in the window or a closed menu hides it.
- `PopupMenu` shows one context menu at a time at a point in the root's points, and answers `EventMenuClosed`. Its items stay choosable until another menu replaces it, since GTK may activate an item after the popover closed.
- macOS tracks it with `popUpMenuPositioningItem:` from the main queue, which keeps applying batches; Windows with `TrackPopupMenuEx` from a message of its own, ended by `EndMenu`; Linux with a `GtkPopoverMenu` on the surface.

## Keys and the status bar

Shortcuts are scoped to their owner: the platform matches key presses against the table synchronously and reports only matches.

- An input method composing text keeps its keys on macOS and the web, so arrows still choose its candidates.
- Each `KeyBinding` says whether it fires while a text input or web view has the focus, and on a held key's auto-repeats; `driver.FindKey` decides for every Go backend, Android's Java the same way.
- By default (`KeyAuto`) a binding leaves an input the keys it uses (`EditsText`) and fires once while held; `OnKeyWith` asks for `KeyAlways`, `KeyNotEditing` and `Repeat`.
- GTK flags no repeats, so Linux counts a key pressed again before its release; UIKit reports a held key once, so iOS repeats a binding on a timer.
- iOS and Android match a hardware keyboard too. `KeyLabel` names a binding as the platform shows it (⇧⌘L, Ctrl+Shift+L), `Environment.AppleKeyboard` telling the web on Apple platforms.
- An alert's `Sheet` presents its actions as an action sheet, and `Cancelable` lets Escape, back and a press outside close it with -1 ([components](components.md#popups)).
- The status bar layers as NSH's `status_bar.rs`: `SetStatusBar` patches the app's default, and each screen's or modal's `StatusBar` is over it while shown. The latest wins field by field; iOS and Android apply the result.

## Where NSGo departs from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| `setFrame`'s `x` and `y` are the bottom left on macOS, with `anchorTop` | The top left from the main screen everywhere | One coordinate system; the top edge stays without an option |
| A bar height picks an `NSToolbar` style, and AppKit places the traffic lights | NSGo places them in the app's bar, where `SetWindowControls` says | Any bar height and position, as MyGo and Electron allow |
| No window material | `nsgo.WindowMaterial` and `SetWindowMaterial`, after MyGo's `Vibrancy` | Desktop apps show the system's materials behind sidebars |
| `openUrl` throws for a refused URL | `OpenURL` answers false; `CheckURL` says why | A callback has no exception to throw |
| No images written on Android | `ClipProvider` serves them | The clipboard holds images only as content URIs |
| Picked images become `File`s whose bytes stay native | `PickedFile` holds the bytes | Go apps hold bytes directly |

## Verification

`core` tests each request and answer; `examples/gallery` checks its System page with `nsgotest`. The Windows and Linux native tests drive the clipboard, frames, levels, the hiding menu bar, a context menu, the appearance and `Quit` in a window, and Windows' open dialog, drawn alerts, hidden caption buttons and backdrops. `internal/platform/apple` checks HEIC and scaled picks; `picked` checks orientation and scaling. macOS, iOS, Android and the web were checked by running each service.
