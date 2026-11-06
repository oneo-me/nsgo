# Services

Services are what the platform does for the app outside its views: alerts, the clipboard, opening links, the photo picker, file and folder picks and saves, opening and sharing the app's files, installing packages on Android, the appearance, keyboard shortcuts, the status bar, the desktop window and quitting. Read this before changing one, in `core` or in a backend; [application and windows](application.md) covers the windows, tray icons and global shortcuts an app opens. A service goes to the platform of the window the code asking is in.

## Requests and answers

A service is a command item in the batch. One that answers, such as a clipboard read, an opened URL or a pick, carries a number in `Count` and comes back as an event with it.

- `Tree.request` keeps the callback under the owner that asked, and the root runs it when that owner is gone, so a closed menu still hears its answer.
- Platforms answer on any goroutine through `Engine.Dispatch`; slow work, such as encoding a photo, runs off the main thread first.
- `nsgotest` answers from its model: `App.Clipboard`, `ClipboardImage` and `ClipboardFiles`, `OpenURL`, `PickImages`, `PickFiles`, `PickDirectories`, `SaveFile`, `OpenFile` and `CanInstallPackages`, and it records the shares, the packages installed, the install settings opened, the appearance, quits and the context menu shown.

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
| Android | A PNG `FileProvider` serves | An image URI, decoded and encoded as PNG |
| Web | The PNG browsers take | `image/png`, with the page's permission |

Android's clipboard holds images only as content URIs, which the runtime's `FileProvider` serves ([sharing files](#opening-and-sharing-files)). NSH writes no images on Android.

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

## Files

`PickFiles` and `SaveFile`, which NSH has not, move files of any type in and out of the app without passing their bytes through the platform's bridge. `PickFilesOptions.Types` offers media types (`application/pdf`), families (`image/*`) and extensions (`.zip`); a picker that filters only by one kind gets the other through `driver`'s table of common types, and offers every file when a type has no counterpart.

- A picked `File` is on disk: the user's own on desktops, a copy in the app's storage on iOS and Android (`Temporary`) that the app deletes. The web has no disk, so `File.Open` reads the file's `Blob` a slice at a time. `FinishFiles` fills names, sizes, modification times and types.
- `SaveFile` copies the file at `SaveOptions.Path`, or `Data` where it names none. Desktops write through a temporary file beside the destination (`SaveRequest.SaveTo`), so a write that fails leaves nothing where the user chose. iOS and Android save only files on disk, so `Data` and a name other than the file's are staged in a directory that is removed once the dialog answers (`SaveRequest.Stage`).
- `PickDirectories` picks folders apart from files, so a caller that wants files gets no folders to filter out. A `Directory` is the user's folder on disk, with its name and path.
- `done` tells a cancel from a failure: no files or folders, or `saved == false`, with a nil error, against the error the platform reported.

| Target | Picker | Folder picker | Save dialog |
| --- | --- | --- | --- |
| macOS | An `NSOpenPanel` sheet with `UTType`s | An `NSOpenPanel` sheet for directories | An `NSSavePanel` sheet, keeping the name's extension |
| iOS | `UIDocumentPickerViewController` copying into the app's `tmp` | None: an error ([roadmap](../roadmap.md#finish-the-existing-platforms)) | The same, exporting a copy of the staged file |
| Android | `ACTION_OPEN_DOCUMENT`; Java copies each document into the cache's `nsgo-files` | None: an error | `ACTION_CREATE_DOCUMENT`; Java copies the file in and deletes the document when that fails |
| Windows | `GetOpenFileNameW` | `IFileOpenDialog` with `FOS_PICKFOLDERS`, which comdlg32 has no peer of; Windows 11's Home, not on disk, opens the profile | `GetSaveFileNameW`, asking before replacing a file |
| Linux | `GtkFileDialog`, filtering by MIME type and suffix | `GtkFileDialog`'s folder selection | `GtkFileDialog`'s save |
| Web | `<input type=file>` | None: an error | `showSaveFilePicker`, whose writable commits whole or not at all; a download where the browser has none or the press's user activation is gone, which counts as saved |

## Opening and sharing files

`OpenFile` opens a file the app has in the app the system associates with its type, and `Share` shows the system's share sheet for files, text or both. Each `SharedFile` is a path, with the name the other app sees and its type, which the file's name and extension say by default. Files go by path or content URI, so their size costs the bridge nothing.

- `core` checks each file is a regular file before asking the platform, and fails a share of neither files nor text.
- `OpenFile` tells a type no app opens (`opened == false`, no error) from a failure. `Share` answers once the sheet shows, which says nothing of what the app chosen did with the files.
- Where a platform hands files on by path and shows their own names, a file whose `Name` differs is linked under it in a directory of the temporary one (`SharedFile.Stage`), which a later share removes after `ShareStageAge`. Desktops open a file itself, under its own name.
- `ShareOptions.Anchor` is the element a sheet shown as a popover points at, on macOS and iPads; it points at the root's center without one.

| Target | `OpenFile` | `Share` |
| --- | --- | --- |
| macOS | `NSWorkspace openURL:` once `URLForApplicationToOpenURL:` finds an app | `NSSharingServicePicker` |
| iOS | `UIDocumentInteractionController`'s Open In menu, which shows no apps when none opens the type | `UIActivityViewController` |
| Android | `Intent.ACTION_VIEW` | An `ACTION_SEND` or `ACTION_SEND_MULTIPLE` chooser, whose clip carries the grants and the preview |
| Windows | `ShellExecuteExW` without UI, false on `ERROR_NO_ASSOCIATION` | None: `NoShareSheet` ([roadmap](../roadmap.md#finish-the-existing-platforms)) |
| Linux | `GtkFileLauncher`, through the desktop portal, which may ask for an app; false on `G_IO_ERROR_NOT_SUPPORTED` or a dismissed chooser | None: `NoShareSheet` |
| Web | None: `NoFileOpener` | None: `NoShareSheet` |

Android hands files on as content URIs from the runtime's `FileProvider`, which the manifest declares unexported, granting URIs, with an authority named after the app's package. A URI names one of its roots, the app's data directory (which holds `DataDir` and `CacheDir`) or its external one, the path under it and the name the other app sees, with the type as a parameter; a file elsewhere fails, and the provider serves nothing outside its roots, whatever the path says. The intent grants the app chosen read access until its activity finishes, so the app keeps the file there until then and deletes it itself.

## Installing packages

`mobile.InstallPackage` hands an APK in the app's storage to Android's package installer, as `OpenFile` hands a file on: `ACTION_VIEW` of a `FileProvider` URI typed `application/vnd.android.package-archive`. The installer asks the user to confirm, and sends them to the setting first where they have not let the app install packages, so the answer says only that it started. `CanInstallPackages` is `canRequestPackageInstalls`, and `OpenInstallSettings` opens `ACTION_MANAGE_UNKNOWN_APP_SOURCES` for the app. Elsewhere `core` answers at once, without asking the platform.

The installer refuses an app whose manifest does not ask for `REQUEST_INSTALL_PACKAGES`, which stores review, so the runtime asks for nothing: an app lists it in `nsgo.yaml`'s `android.permissions` ([tool](tool.md)), an embedding host in its own manifest, and `InstallPackage` fails with that advice without it.

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

When the app ends by itself, and which of its windows a close keeps, is its shutdown mode's and its windows' `OnClosing` ([application and windows](application.md#lifecycle)).

## Desktop windows

Package `desktop` is NSH's `nsh/desktop`: native controls, an app-drawn title bar, and the menus below. Its functions call the `Scope` methods of the same names, the core's part.

- `SetTitleBar` hides the system title bar and gives the app's bar its height, in which the platform centers its controls. `TitleBar` then reports the area they keep, and views with `Desktop.WindowDrag` drag the window.
- macOS centers the traffic lights in the bar, 16 points from the left, as Onevra does; Windows draws Onevra's rounded caption buttons; Linux reports the bar's height and the app draws its buttons.
- `SetWindowControls` hides those controls for the app's own, or moves the traffic lights, as Electron's `trafficLightPosition` does: the close button's top left from the window's, zero X or Y keeping the default.
- macOS 27's AppKit puts the traffic lights back whenever it lays the title bar out, without a notification, so NSGo's `NSWindow` subclass places them after each `layoutIfNeeded`. Hiding and showing them makes AppKit place them again once the app's position goes, as Electron's `window_buttons_proxy.mm` notes.
- `Maximized` reports a maximized or zoomed window, so an app-drawn maximize button can show restore, and `FullScreen` one in full screen.
- The platform decides a window drag on the main thread from the mounted props: the nearest view setting `WindowDrag` decides, and a nearer pressable, input or gesture view keeps the press.
- `nsgo.WindowMaterial` starts the window with a material behind the app's views and `SetWindowMaterial` changes it: an `NSVisualEffectView` on macOS, Mica, Acrylic or Mica Alt on Windows 11 22H2 and later. `ShowsMaterial` reports whether one shows; elsewhere the window stays opaque.
- Windows shows a material only in a window started with one: its content needs DirectComposition and `WS_EX_NOREDIRECTIONBITMAP`, which holds from the window's creation. Such a window changes its backdrop, or turns it off, any time.
- `SetWindowFrame` moves and sizes the window in points from the main screen's top left, keeping the top edge without a move.
- macOS converts the frame to its bottom-left one and animates it; Windows takes the window's pixels; Linux sets GTK's default size and leaves placement to the compositor.
- `Frame` reports the frame `SetWindowFrame` takes, through the environment, so an app reads it as an `Env` signal. Windows keeps the last one through a minimize, which parks the window off-screen. Linux reports `gtk_window_get_default_size`, the size GTK keeps apart from maximizing, and no position.
- `nsgo.WindowFrame` and `WindowOptions.Frame` open a window at such a frame rather than moving it after it shows, so an app restores one without a flash. `WindowFrame.OnScreen` keeps a window whose top edge no screen shows from opening out of reach: it centers on the main screen on macOS, and on the nearest monitor on Windows.
- `SetWindowConstraints`, `WindowOptions.Constraints` and `nsgo.WindowConstraints` limit a window's content size, which it moves into, and say whether it resizes, minimizes and maximizes, the app's own `MinimizeWindow`, `ToggleMaximize` and `SetWindowFrame` sizes included. `NoResize` implies `NoMaximize`, but `SetWindowFrame` still sizes the window.
- macOS sets `contentMinSize`, `contentMaxSize` and the style mask, disables the zoom button and declines `windowShouldZoom:toFrame:`; AppKit limits only the user's resizes, so `SetWindowFrame` clamps. `NoMaximize` also sets `FullScreenNone`, the same button going full screen there. `isZoomed` reports a window at its maximum, so it cannot tell a maximized one.
- Windows answers `WM_GETMINMAXINFO`, without limits in full screen, and clears `WS_THICKFRAME`, `WS_MINIMIZEBOX` and `WS_MAXIMIZEBOX`; the caption buttons it draws dim and take no presses.
- GTK 4 sets no maximum size and takes no title bar buttons, so Linux sets the minimum as the surface's size request and fixes a window with `NoResize`, or a maximum equal to the minimum, by `gtk_window_set_resizable`; the app's commands still follow the constraints, and an app-drawn bar with them ([roadmap](../roadmap.md#application-and-windows)).
- `SetWindowLevel` floats a window (`NSFloatingWindowLevel`, `HWND_TOPMOST`) or sinks it to the desktop's icons on macOS, on every space, or to the bottom of the z-order on Windows. `OrderFront` restores, raises and activates.

## Menus

`SetApplicationMenu` and `PopupMenu` send menus as data, handlers replaced by IDs that come back in `EventMenu`. Role items are the platform's own and are never reported. The default menu returns when the owner that set the menu goes. An item's `When` resolves to `MenuEntry.NotInText`, as a key binding's does: Windows and Linux leave such a shortcut to a focused input in their key path, and macOS disables the item while a key press looks for its key equivalent with a field focused (`validateMenuItem:`), so AppKit hands the key to the field.

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
- The status bar layers as NSH's `status_bar.rs`: `SetStatusBar` patches the app's default, and each screen's or modal's `StatusBar` is over it while shown. The latest wins field by field; iOS and Android apply the result. A TSX app's default is `nsgo.yaml`'s `status_bar` (`foreground`, `hidden`), as NSH's `nsh.json` `statusBar`.

## Where NSGo departs from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| `setFrame`'s `x` and `y` are the bottom left on macOS, with `anchorTop` | The top left from the main screen everywhere | One coordinate system; the top edge stays without an option |
| A bar height picks an `NSToolbar` style, and AppKit places the traffic lights | NSGo places them in the app's bar, where `SetWindowControls` says | Any bar height and position, as Electron allows |
| No window material | `nsgo.WindowMaterial` and `SetWindowMaterial`, after Electron's `vibrancy` | Desktop apps show the system's materials behind sidebars |
| No window limits; a size only in `nsh.json` | `WindowConstraints`, after Electron's `minWidth` and `minimizable`, in content points | Settings and palette windows keep their size and buttons out of the user's hands |
| `openUrl` throws for a refused URL | `OpenURL` answers false; `CheckURL` says why | A callback has no exception to throw |
| No images written on Android | `FileProvider` serves them | The clipboard holds images only as content URIs |
| Picked images become `File`s whose bytes stay native | `PickedFile` holds the bytes | Go apps hold bytes directly |

## Verification

`core` tests each request and answer; `examples/gallery` checks its System page with `nsgotest`. The Windows and Linux native tests drive the clipboard, frames, levels, the hiding menu bar, a context menu, the appearance and `Quit` in a window, and Windows' drawn alerts, hidden caption buttons and backdrops. Both pick a file, save a copy and cancel a save in the system's dialogs, Linux's in a process of its own without the portal, and Windows' open dialog also picks an image. `internal/platform/apple` checks HEIC and scaled picks and the uniform types of file picks; `picked` checks orientation and scaling. `WindowConstraints` were checked on a real AppKit window (2026-11-06) and have only been built for Windows and Linux. macOS, iOS, Android and the web were checked by running each service, except file picks and saves on macOS and iOS, which have not run yet.
