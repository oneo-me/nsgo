# Services

Services are what the platform does for the app outside its views: alerts, the clipboard, opening links, the photo picker, taking a photo, file and folder picks and saves, opening and sharing the app's files, installing packages on Android, speech recognition, biometrics, notifications and the badge, haptics, the app's lifecycle, the appearance, keyboard shortcuts, the status bar, the desktop window and quitting. Read this before changing one, in `core` or in a backend; [application and windows](application.md) covers the windows, tray icons and global shortcuts an app opens. A service goes to the platform of the window the code asking is in.

## Requests and answers

A service is a command item in the batch. One that answers, such as a clipboard read, an opened URL or a pick, carries a number in `Count` and comes back as an event with it.

- `Tree.request` keeps the callback under the owner that asked, and the root runs it when that owner is gone, so a closed menu still hears its answer.
- Platforms answer on any goroutine through `Engine.Dispatch`; slow work, such as encoding a photo, runs off the main thread first.
- `nsgotest` answers from its model: `App.Clipboard`, `ClipboardImage` and `ClipboardFiles`, `OpenURL`, `PickImages`, `TakePhoto` (recording `Photos`), `PickFiles`, `PickDirectories`, `SaveFile`, `OpenFile` and `CanInstallPackages`, and it records the shares, the packages installed, the install settings opened, the appearance, quits and the context menu shown. A speech session it runs hears what `HearSpeech` says and ends with `EndSpeech`.

## Clipboard

`SetClipboard` and `SetClipboardImage` write; `ReadClipboard`, `ReadClipboardImage` and `ReadClipboardFiles` read; `HasClipboardText` and `HasClipboardImage` read only what the clipboard holds, so iOS and Android show no paste notice for them.

- An image is read as a PNG and its size in pixels. One whose PNG is over 48 MB fails with `CodeQuotaExceeded`, as NSH's, and one the platform cannot read or convert fails with why (`driver.ReadClipboardImage`), so `readImage` rejects both and resolves null only for none. One beside copied files is their icon, not the user's image, and reads as none.
- Files are the `file:` URLs a file manager copied. Android and iOS rarely hold any, and the web has no access to them.

| Target | Writes an image as | Reads an image from |
| --- | --- | --- |
| macOS | PNG and TIFF | PNG, TIFF, JPEG, HEIC or GIF, converted by ImageIO |
| iOS | PNG | The same types, else `UIPasteboard.image` |
| Windows | `PNG` and a DIB over white | `PNG`, else the DIB through `x/image/bmp` |
| Linux | A `GdkTexture` | A texture, saved as PNG |
| Android | A PNG `FileProvider` serves | An image URI, decoded and encoded as PNG |
| Web | The PNG browsers take | `image/png`, with the page's permission: a refused read fails |

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
| Web | `<input type=file>`, which needs the press's user activation | The browser's codecs, through `createImageBitmap` and a canvas (`web/picked.go`) |

`picked` decodes in Go, which reads no HEIC, so those pickers offer only the formats it reads unless `PickCurrent`. It turns JPEGs upright by their EXIF orientation and scales with Catmull-Rom. The web does the same with the browser's codecs, so pages link no Go image codec.

## Taking a photo

`TakePhoto` shows the system's camera for one photo, which NSH has not: a `PickRequest` of one whose JPEG is always encoded again, upright and at the quality asked for (0.9 by default), scaled to `MaxDimension`, and answered by `EventPick` as a pick is. `done` gets nil for a cancel and an error for a failure, such as a refused permission. `Env.CanTakePhoto` says the platform shows a camera; elsewhere `TakePhoto` fails with `ErrNoCamera` without asking it.

| Target | Camera | Permission |
| --- | --- | --- |
| iOS | `UIImagePickerController`, where `isSourceTypeAvailable` says the device has a camera, which the simulator has not | `AVCaptureDevice`'s, asked once; the Info.plist declares `NSCameraUsageDescription` (`nsgo.yaml`'s `apple.info`), without which a photo fails, since the system would end the app as it asks |
| Android | `ACTION_IMAGE_CAPTURE`, where the device has `FEATURE_CAMERA_ANY`; the camera app writes into a file of the cache's `nsgo-camera`, the only files `FileProvider` opens for writing, which Java prepares and deletes | None, unless the manifest asks for `CAMERA`, which the camera app then needs granted: Java asks for it first (`permissionResult`) |
| Web | `<input type=file capture=environment>` on touch screens (`pointer: coarse`), which mobile browsers open as the camera; prepared with the browser's codecs | The browser's |

The desktops show no camera ([roadmap](../roadmap.md#finish-the-existing-platforms)).

## Files

`PickFiles` and `SaveFile`, which NSH has not, move files of any type in and out of the app without passing their bytes through the platform's bridge. `PickFilesOptions.Types` offers media types (`application/pdf`), families (`image/*`) and extensions (`.zip`); a picker that filters only by one kind gets the other through `driver`'s table of common types, and offers every file when a type has no counterpart.

- A picked `File` is on disk: the user's own on desktops, a copy in the app's storage on iOS and Android (`Temporary`) that the app deletes. The web has no disk, so `File.Open` reads the file's `Blob` a slice at a time. `FinishFiles` fills names, sizes, modification times and types.
- `SaveFile` copies the file at `SaveOptions.Path`, or `Data` where it names none. Desktops write through a temporary file beside the destination (`SaveRequest.SaveTo`), so a write that fails leaves nothing where the user chose, and report the path they chose, which the app saves to directly from then on. iOS and Android save only files on disk, so `Data` and a name other than the file's are staged in a directory that is removed once the dialog answers (`SaveRequest.Stage`).
- `PickScreenColor` lets the user pick a color anywhere on the screen, with `NSColorSampler` on macOS and the browser's `EyeDropper`, where `Env.CanPickScreenColor` says so.
- `PickDirectories` picks folders apart from files, so a caller that wants files gets no folders to filter out. A `Directory` is an `fs.FS` of the user's folder: on desktops and iOS it reads the folder on disk at `Path`, which iOS keeps in its security scope while the app runs; Android and the web have no path, so it reads through `Directory.FS`, waiting for the platform, and the app reads it off the engine's goroutine. `driver.FolderFS` makes that `FS` of a platform's listings and reads, finding a name through the listings it remembers and listing a folder afresh whenever it is read.
- `done` tells a cancel from a failure: no files or folders, or `saved == false`, with a nil error, against the error the platform reported.

| Target | Picker | Folder picker | Save dialog |
| --- | --- | --- | --- |
| macOS | An `NSOpenPanel` sheet with `UTType`s | An `NSOpenPanel` sheet for directories | An `NSSavePanel` sheet, keeping the name's extension |
| iOS | `UIDocumentPickerViewController` copying into the app's `tmp` | The same for `public.folder`, opening the folders where they are | The same, exporting a copy of the staged file |
| Android | `ACTION_OPEN_DOCUMENT`; Java copies each document into the cache's `nsgo-files` | `ACTION_OPEN_DOCUMENT_TREE`, one folder, whose tree Java lists and reads a megabyte at a time for Go (`msgFolder`); the grant lasts until the device restarts | `ACTION_CREATE_DOCUMENT`; Java copies the file in and deletes the document when that fails |
| Windows | `GetOpenFileNameW` | `IFileOpenDialog` with `FOS_PICKFOLDERS`, which comdlg32 has no peer of; Windows 11's Home, not on disk, opens the profile | `GetSaveFileNameW`, asking before replacing a file |
| Linux | `GtkFileDialog`, filtering by MIME type and suffix | `GtkFileDialog`'s folder selection | `GtkFileDialog`'s save |
| Web | `<input type=file>` | One folder: `showDirectoryPicker`, whose handles read it as the app opens it, else `<input webkitdirectory>`, which lists it whole and leaves out the folders holding no files | `showSaveFilePicker`, whose writable commits whole or not at all; a download where the browser has none or the press's user activation is gone, which counts as saved |

## Opening and sharing files

`OpenFile` opens a file the app has in the app the system associates with its type, and `Share` shows the system's share sheet for files, text or both. Each `SharedFile` is a path, with the name the other app sees and its type, which the file's name and extension say by default. Files go by path or content URI, so their size costs the bridge nothing.

- `core` checks each file is a regular file before asking the platform, and fails a share of neither files nor text. `OpenFile` also takes a folder on macOS, Windows and Linux, whose openers hand it to the file manager; phones have no file manager an app can open a folder of its own in, so `core` fails it there.
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

## Speech

`StartSpeech` recognizes what the user says, NSH's `Speech.start`: one session per engine, which a new one and its scope's end abort. It reports the whole transcript so far, before it is final with `Interim`, and ends once with the last one and a `SpeechError` named as the Web Speech API's (`not-allowed`, `no-speech`, `audio-capture`, `network`, `language-not-supported`, `unavailable`, `failed`). `Stop` has the recognizer finish what it heard; `Abort` ends it at once. A `Lang` of "" is the app's locale.

- macOS and iOS (`apple/speech.go`) feed the Speech framework's recognizer from `AVAudioEngine`'s input, asking for speech recognition and the microphone once each. The app's Info.plist declares `NSMicrophoneUsageDescription` and `NSSpeechRecognitionUsageDescription` (`nsgo.yaml`'s `apple.info`): without them a session ends `not-allowed`, since the system would end the app as it asks. iOS holds the audio session in the record category while it listens.
- `Env.CanRecognizeSpeech` says the platform recognizes speech; elsewhere a session ends `unavailable` without asking it ([roadmap](../roadmap.md#finish-the-existing-platforms)).

## Biometrics

`BiometricAvailability` answers with the enrolled kind (`BiometricFace`, `BiometricFingerprint` or `BiometricNone`) without prompting. `AuthenticateBiometric` shows the system's biometric prompt with the app's reason, which must not be empty (an empty one answers `BiometricFailure` without a prompt, as `LocalAuthentication` throws on it), and answers `BiometricSuccess`, `BiometricFailure`, `BiometricCancelled` or `BiometricUnavailable`; it never falls back to a device passcode. Code whose `Access` does not grant `AccessBiometrics` gets unavailable.

| Target | Availability and authentication | App configuration |
| --- | --- | --- |
| macOS, iOS | `LAContext` from `LocalAuthentication` (`apple/biometric.go`) | Face ID needs `NSFaceIDUsageDescription` in `nsgo.yaml`'s `apple.info` |
| Android | `BiometricManager`/`BiometricPrompt` from API 28; as Android does not say which kind is enrolled, an enrolled fingerprint answers `BiometricFingerprint` and otherwise face hardware answers `BiometricFace` | The app lists `USE_BIOMETRIC` in `android.permissions` |
| Other targets | Unavailable | None |

`nsgotest` exposes `BiometricAvailability`, `BiometricAnswer` and `AuthenticationReasons` so tests can fake enrollment and prompt results.

## Notifications and the badge

`NotificationPermission` answers whether the app may show notifications without asking, `RequestNotificationPermission` asks with the system's prompt the first time, as NSH's `Notifications`: not asked (`"default"`), granted, denied, or unavailable where the platform or build shows none. `ShowNotification` shows one, replacing one of its ID, also while the app is in front, and answers whether the system took it; `CloseNotification` removes it; `OnNotificationPress` hears a press, the one that started the app kept for the first handler. `SetBadge` sets the count on the app's icon. Code its `Access` does not grant `AccessNotifications` gets unavailable and nothing shown.

| Target | Notifications | Permission | Press | Badge |
| --- | --- | --- | --- | --- |
| macOS, iOS | `UNUserNotificationCenter` (`apple/notifications.go`), its delegate set before launching ends, showing them in front | The center's; a bare executable outside an app bundle has none | The delegate's default action | The Dock tile's label; iOS's needs the permission, `setBadgeCount:` from iOS 16 |
| Android | `NotificationManager` in `Notifications.java`, a channel with the system's sound and a silent one | `POST_NOTIFICATIONS` from API 33 where the manifest asks for it (`android.permissions`), else the app's setting | A `PendingIntent` bringing the activity back with the ID, kept until the first batch | None: the count goes with notifications shown after it |
| Windows | Toasts through WinRT's `ToastNotificationManager`, tagged with the ID; an app outside an MSIX registers its ID as an AppUserModelID in the current user's registry with its name and icon | The notifier's setting: granted or denied | The toast's `Activated`, while the app runs | The taskbar button's overlay icon, a red disc drawn with GDI |
| Linux | `org.freedesktop.Notifications` over GDBus, with `desktop-entry` and `suppress-sound` hints | Granted where a server answers, else unavailable | The `default` action's `ActionInvoked`, presenting the window | None |
| Web | The page's `Notification` | The page's | Its `onclick`, focusing the window | `navigator.setAppBadge` for an installed web app |

`nsgotest` keeps the notifications shown, the badge and the permission (`NotificationPermission`, `NotificationAnswer`), and `PressNotification` presses one.

## Haptics

`mobile.PlayHaptic`, NSH's `Haptics`, plays a selection, an impact of five weights or a task's outcome: UIKit's feedback generators on iOS, and on Android the view's nearest `HapticFeedbackConstants`, `CONFIRM`, `REJECT` and `SEGMENT_TICK` where its API has them, as the user's touch feedback setting allows. The core sends nothing elsewhere.

## The app's lifecycle

`Env.Lifecycle`, NSH's `lifecycle()`, says whether the app is in use: `active`, `inactive` (shown without the focus) or `background` (none of it shows). Each backend reports it with the environment, as it changes.

| Target | Background while | Active while |
| --- | --- | --- |
| macOS | The app is hidden, or `occlusionState` says none of it shows (minimized, covered, the screen locked) | It is the active app |
| iOS | `applicationState` is background | It is active: not under a call, the app switcher or Control Center |
| Android | `NsgoView`'s window is not visible | The window has the focus |
| Windows | The window, an embedded view's root window, is minimized or hidden | The app is active (`WM_ACTIVATEAPP`) |
| Linux | The surface is unmapped, or its toplevel minimized or suspended | One of the app's windows `is-active` |
| Web | The document is hidden | The document has the focus |

`core.App`, NSH's `appInfo()` and TSX's too, is the app's ID, name, version and build number as `nsgo.yaml` gives them, which the tool builds into `core` with `-X`, and whether it is a development build: any but `nsgo build`'s. It is the binary's, not an engine's, so every view a host embeds reads the host build's.

## Displays and locale settings

`Env.Displays` is the desktop's list of `DisplayInfo` values. `Frame` is a display's whole area and `VisibleFrame` the area left for windows after the menu bar, Dock or taskbar; both use the same top-left, y-down screen coordinates as `desktop.SetWindowFrame`. `Scale` is device pixels per point and `Main` marks the primary display. On Windows a display's top left is its pixel offset from the primary display divided by the smallest display scale, and points within a display are its own scale's, so displays with different scales never overlap. The list changes when the platform reports a display arrangement change; on Linux a change only to an X11 work area, as a panel resized, shows at the next environment change. Phones and the web leave it empty; a view embedded in a desktop host reports the host machine's displays.

| Target | Display source | Locale settings source |
| --- | --- | --- |
| macOS | `NSScreen.screens`, converted from AppKit's bottom-left coordinates | `NSLocale` and its calendar and date format preferences |
| Windows | `EnumDisplayMonitors`, monitor work areas and per-monitor DPI | `GetLocaleInfoEx` for first weekday, hour cycle and measurement |
| Linux | GDK's monitor model; X11 work areas, the whole monitor on Wayland, which reports none | the session's locale categories and GTK settings |
| iOS, Android, Web | no desktop displays | platform or browser locale settings where available |

`LocaleSettings` reports `FirstDayOfWeek`, `HourCycle` and `MeasurementSystem` independently of `Env.Locale`, so a localized app can follow the user's calendar, clock and measurement choices. A zero field means the platform did not report that choice.

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

`desktop.Restart`, NSH's `Application.restart()`, quits as `Quit` does with `ItemQuit`'s `On` set, and `nsgo.Run`, once the app has ended, starts the program again with the arguments it started with, its launch flags among them. The two never run side by side: on macOS and Linux a shell waits up to 10 seconds for the old process to exit, then starts the new one, a macOS bundle through `open -n` as the user opens it; on Windows the new process waits for the one `NSGO_RESTART_AFTER` names before `Run` starts. A release process started by a restart carries `NSGO_RESTART_GUARD`, so an app that restarts again within 10 seconds of its restart, as one restarting itself at launch does, logs one refusal instead of looping, while a restart the user asks for later still runs. Under `nsgo dev`, which ends with the app it started, it only quits.

When the app ends by itself, and which of its windows a close keeps, is its shutdown mode's and its windows' `OnClosing` ([application and windows](application.md#lifecycle)).

`OnQuit` handlers clean up as the app ends, as NSH's `Application.on("quit")` and GPUI's `on_app_quit`: `Engine.Close` runs them on the engine goroutine, oldest first, with one context that ends `QuitTimeout` (200 ms) after they start, before the graph is disposed, and waits for them no longer than that and 100 ms. They run however the engine closes: `Quit`, the shutdown mode, AppKit's terminate, the window closing on Windows and Linux, the Java runtime ending on Android, iOS's `applicationWillTerminate:` and a host removing an embedded view. The web and watchOS have no such moment, as the system may end either unheard: `Engine.Hide` runs the handlers as a page hides (`pagehide`) or a watchOS app goes to the background, keeps them and the graph in case the app shows again, as a page from the back-forward cache does, and a page hidden for good closes the engine.

## Desktop windows

Package `desktop` is NSH's `nsh/desktop`: native controls, an app-drawn title bar, and the menus below. Its functions call the `Scope` methods of the same names, the core's part.

- `SetTitleBar` hides the system title bar and gives the app's bar its height, in which the platform centers its controls. `TitleBar` then reports the area they keep, and views with `Desktop.WindowDrag` drag the window.
- macOS centers the traffic lights in the bar, 16 points from the left; Windows draws rounded caption buttons of NSGo's; Linux reports the bar's height and the app draws its buttons.
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

`SetApplicationMenu` and `PopupMenu` send menus as data, handlers replaced by IDs that come back in `EventMenu`. Role items are the platform's own and are never reported. The backends say them, and macOS's default menu, in the language the app shows, which `ItemDirection` carries, from the catalogs of `desktop/locales`: an item without a label of its own, as the bar is rebuilt when the language changes. macOS's full screen item takes its own action, since AppKit's titles it in the language of the bundle. An app says its menus' titles and items itself, setting the menu again from an `Effect` as the language changes. The default menu returns when the owner that set the menu goes. An item's `When` resolves to `MenuEntry.NotInText`, as a key binding's does: Windows and Linux leave such a shortcut to a focused input in their key path, and macOS disables the item while a key press looks for its key equivalent with a field focused (`validateMenuItem:`), so AppKit hands the key to the field.

- `SetMenuBar(MenuBarAutoHide)` hides the bar Windows and Linux draw until Alt is pressed and released alone, which is kept from Windows' menu mode. Alt with a mnemonic or F10 shows it for the menu they open; a press in the window or a closed menu hides it.
- `PopupMenu` shows one context menu at a time at a point in the root's points, and answers `EventMenuClosed`. Its items stay choosable until another menu replaces it, since GTK may activate an item after the popover closed.
- macOS tracks it with `popUpMenuPositioningItem:` from the main queue, which keeps applying batches; Windows with `TrackPopupMenuEx` from a message of its own, ended by `EndMenu`; Linux with a `GtkPopoverMenu` on the surface.

## Keys and the status bar

Shortcuts are scoped to their owner: the platform matches key presses against the table synchronously and reports only matches.

- An input method composing text keeps its keys on macOS and the web, so arrows still choose its candidates.
- `Scope.SelectInputSource`, `base.Command`'s `InputSource`, selects an enabled keyboard input source by the platform's ID, and `RestoreInputSource` goes back unless the user chose another since; phones and the web ignore both, and `nsgotest` records them in `App.InputSource`.
- macOS selects through Text Input Sources (`com.apple.keylayout.ABC`), which need no field's input context, so restoring works after the field is gone. Windows activates a keyboard layout for the process by its KLID (`00000409`), unloading again one the user has not enabled. Linux asks the input method framework over D-Bus, off the main thread: Fcitx 5's input methods of the current group (`keyboard-us`), else IBus's engines (`xkb:us::eng`) on the bus its daemon names.
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
| No camera | `TakePhoto` through the system's camera UI | Apps take a photo without a camera view of their own |

## Verification

`core` tests each request and answer, speech sessions included; `examples/gallery` checks its System page with `nsgotest`. The Windows and Linux native tests switch input sources and back and follow the lifecycle through a minimize or a hide; Linux's switch ran with a second Fcitx 5 input method enabled (2026-11-08). Speech was checked on macOS only as far as its refusal without permission, which this machine's session gives: no transcript has been recognized yet. The Windows and Linux native tests drive the clipboard, frames, levels, the hiding menu bar, a context menu, the appearance and `Quit` in a window, and Windows' drawn alerts, hidden caption buttons and backdrops. Both pick a file, save a copy and cancel a save in the system's dialogs, Linux's in a process of its own without the portal, and Windows' open dialog also picks an image. `internal/platform/apple` checks HEIC and scaled picks and the uniform types of file picks; `picked` checks orientation and scaling, and `TestPreparePicked` the web's scaling and encoding, run as WebAssembly in Chrome. `WindowConstraints` were checked on a real AppKit window (2026-11-06) and have only been built for Windows and Linux. macOS, iOS, Android and the web were checked by running each service, except file picks and saves on macOS and iOS, which have not run yet.
