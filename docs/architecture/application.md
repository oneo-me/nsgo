# Application and windows

An app opens what it shows itself, as an Avalonia application does: windows, panels, tray icons and global shortcuts, on one engine whose windows share its signals. Read this before changing `nsgo.Run`, `nsgo.SetMainView`, `desktop`'s windows, tray icons, shortcuts or shutdown, or how the engine attaches surfaces. macOS opens them; the [roadmap](../roadmap.md#application-and-windows) orders Windows and Linux.

## Model

`nsgo.Run` takes a root view or the app's function. The function runs once, in the app's own scope, and decides what to open:

```go
func main() { nsgo.Run(App, nsgo.HideDock()) }

func App(app *core.Scope) {
	desktop.SetShutdownMode(app, desktop.ShutdownOnExplicit)
	panel := desktop.OpenPanel(app, desktop.PanelOptions{HideOnBlur: true}, Panel)
	desktop.NewTray(app, desktop.TrayOptions{View: TrayIcon, Panel: panel, Menu: trayMenu})
	launcher := desktop.OpenPanel(app, desktop.PanelOptions{Width: 480, Height: 64, HideOnBlur: true}, Launcher)
	desktop.RegisterHotKey(app, "ctrl+alt+space", launcher.Toggle)
}
```

| App | Opens |
| --- | --- |
| Desktop app | `nsgo.Run(Root)`: a main window, the app ending as it closes; others, such as settings, with `OpenWindow` |
| Tray app | A tray icon whose primary click shows a panel and secondary click a menu; an optional welcome window; `ShutdownOnExplicit` and `nsgo.HideDock` |
| Background app | Global shortcuts that show a panel, as Raycast's; an optional welcome window; `ShutdownOnExplicit` |
| Phone or web app | The view `nsgo.SetMainView` sets, or the root view; `desktop`'s functions do nothing |

`examples/tray` is a tray app with all of them.

## Surfaces

The engine runs one graph with a tree per surface (`core.Tree`). Where the platform opens windows (`core.RootOptions.Windows`, the main window's options), the first tree is the app's own surface: it shows no view, and the platform gives it no window.

- `OpenWindow`, `OpenPanel` and a drawn tray icon make a tree under a scope of the opener's owner, so the window closes when that owner goes. Every scope under it knows its tree (`Scope.surf`), so services, key bindings and `desktop`'s window functions act on the window the code is in, and the app's own scope on the first tree.
- Each tree has its own `Env`: size, scale, safe area and title bar. What the app forces, its locale, appearance and host tokens, is one signal shared by every window's.
- On desktops, `Env.Displays` reports each display's whole and visible frame, scale and primary flag in the same coordinates as `desktop.SetWindowFrame`, so a saved frame can be placed on a chosen display.
- Opening sends `ItemOpenSurface` to the app's surface. The platform makes the window and attaches it: `engine.Engine.Attach(id, platform)` returns the surface's `Engine`, which reports its size, environment and events as the first surface's does.
- The engine commits every attached tree after each wake-up, the first first, and asks a window's platform for display frames, since the app's surface has no display. A tree waits uncommitted until it is attached.
- A closed window's tree ends: the engine sends its platform one batch, `WindowDestroy`, and forgets it, or sends it to a platform that attaches after the close.
- Where the platform opens no windows (phones, the web, `nsgo.NewView` and test hosts), the first tree is the one surface: `OpenWindow` and `OpenPanel` return nil, while supported desktop backends still process `NewTray`, and `core.AppRoot` shows the main view in it.

## Lifecycle

- `SetShutdownMode`: `ShutdownOnLastWindowClose` (the default), `ShutdownOnMainWindowClose` or `ShutdownOnExplicit`, which only `Scope.Quit` ends. Panels and tray views are not windows here.
- `OnQuit` handlers clean up as the app ends, however it does, with 200 ms for their work ([services](services.md#appearance-and-quitting)).
- A close the user asks for is `EventWindowClose`: the window's `OnClosing` handlers run newest first and may cancel it, as a window that hides instead does. `Window.Close` and its owner going close it without asking.
- `nsgo.SetMainView` sets the main view: it opens the main window at `nsgo.WindowSize` and makes it main, and a later call replaces the view in it. `nsgo.Run(Root)` is `SetMainView(Root)`.
- `OnReopen` hears a click on the Dock icon and a second launch (`EventReopen`); without a handler the main window shows and takes the focus, on Windows and Linux the app's one window, restored if minimized. Windows' named mutex and pipe and Linux's Unix socket, beside its lock in `$XDG_RUNTIME_DIR` (in a Flatpak, `$XDG_RUNTIME_DIR/app/$FLATPAK_ID`, which the app's instances share) or else in a private directory of the user's in the temporary one, hand a second launch to the running process before it creates a second engine, which dispatches `OnReopen` and its file arguments to `OnOpenFiles`; on Windows the second launch lets the running process take the foreground before it hands over.
- `OnOpenFiles` hears files the system opens with the app (`EventOpenFiles`): on macOS `application:openURLs:` for the types `file_types` registers, on Windows and Linux the files named on the command line. Files come before the app's first view builds when they launched it, so the core keeps those until a handler registers and posts them to it.
- `nsgo.HideDock` starts the app as an accessory app, before AppKit's loop runs, so its icon never shows in the Dock; `SetDockVisible` changes the activation policy later, as apps do while their settings show. macOS runs one instance of a bundle and reopens it.
- `SetLaunchAtLogin` and `LaunchAtLogin` register and read `SMAppService.mainAppService`, which takes only apps in a bundle.

## macOS

`Run` starts the engine on a surface in no window (`own`), which applies the app-level items: it opens surfaces, adds tray icons, registers shortcuts, sets the activation policy and the login item.

- A window is an `NsgoWindow` centered at its content size. AppKit's close asks the delegate, which reports `EventWindowClose` and declines; `WindowDestroy` orders the window out and closes it once the batch returned.
- A panel is an `NsgoPanel`: an `NSPanel` that can become key without activating the app, titled for the system's rounded corners and shadow, with a transparent title bar and no controls, at the status level on every space. It shows under its tray icon's button, clamped to the screen's visible frame, else in the upper middle of the screen with the mouse, as Spotlight. With `HideOnBlur` it hides as it resigns key and reports `EventWindowHidden`.
- A tray icon is an `NSStatusItem`: its button reports left and right clicks, a Control-click as secondary. The click that hides a panel by taking its focus does not show it again. Its menu is the status item's own, set for one `performClick:` and reported closed as a context menu.
- A drawn icon is a surface in the button: an embedded surface whose hit test lets clicks through to the button, the item as wide as the view.
- Global shortcuts are Carbon's `RegisterEventHotKey` on the application's event target, which needs no accessibility permission; a key names its ANSI virtual key code.
- An alert asked from the app's own surface, which has no window to hold a sheet, runs app-modal.

## Linux

The Linux backend attaches opened window surfaces to GTK's main loop and registers StatusNotifierItem icons on the session bus when a compatible watcher is present, reporting primary and secondary activation. Panels use borderless GTK windows, take keyboard focus and honor `HideOnBlur`; tray anchoring, non-activating behavior, drawn tray views and tray menus remain in the surfaces slice of the roadmap.

## Configuration

`nsgo.yaml` keeps what the package needs before any code runs: the name, the ID, the version and the launch screen's appearance. Windows are sized and limited in Go, `nsgo.WindowSize` and `nsgo.WindowConstraints` for the main window and `WindowOptions` for others; `nsgo.HideDock` and the shutdown mode are Go's too. A `window` in `nsgo.yaml` fails the tool, naming `nsgo.WindowSize`.

## Departures from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| `run_app` shows one `AppView` in one window; more windows are a host's, an engine each | One engine with a surface per window, panel and tray view, which the app opens as Avalonia's apps do | Go code is trusted and in one process, so windows share signals instead of crossing a host channel |
| `keep_alive_on_close`, macOS only | `SetShutdownMode` and a cancelable `OnClosing` | Tray and background apps outlive their windows |
| No tray, panels, global shortcuts, Dock policy or launch at login | `desktop` has them | Desktop apps need them |
| `nsh.json`'s window size | `nsgo.WindowSize`, `nsgo.WindowConstraints` and `WindowOptions` | An app opens windows of different sizes, or none |

## Verification

`core/window_test.go` opens windows, panels and tray icons on a platform that opens windows, headless: shared signals, shutdown modes, canceled closes, reopening, keys and owners, tray clicks and menus, shortcuts, the Dock and the login item, and the main view where no windows open. `internal/engine` attaches an opened window and destroys it. On macOS `examples/tray` was run: its welcome window, its panel under the icon after a click, a close that keeps the app, and the gallery ending with its window.
