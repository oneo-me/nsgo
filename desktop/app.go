package desktop

import "github.com/oneo-me/nsgo/core"

// An app run with nsgo.Run's app function opens what it shows itself, as
// an Avalonia application does: windows, panels, tray icons and global
// shortcuts, on macOS. Each belongs to the owner that opened it and goes
// with it; the windows share the app's signals. Where the platform opens
// no windows of the app's, the functions that open one return nil, whose
// methods do nothing, and the app shows its main view
// (nsgo.SetMainView).

type (
	// Window is a window or a panel the app opened.
	Window = core.Window
	// WindowOptions are a window's title, size and key.
	WindowOptions = core.WindowOptions
	// PanelOptions are a panel's size and whether it hides as it loses
	// the focus.
	PanelOptions = core.PanelOptions
	// WindowClosing is a close the user asked for, which
	// Window.OnClosing may cancel.
	WindowClosing = core.WindowClosing
	// ShutdownMode says when the app ends by itself.
	ShutdownMode = core.ShutdownMode
	// Tray is a tray icon: in the macOS menu bar.
	Tray = core.Tray
	// TrayOptions are a tray icon's image or view, and what its clicks
	// show.
	TrayOptions = core.TrayOptions
)

const (
	ShutdownOnLastWindowClose = core.ShutdownOnLastWindowClose
	ShutdownOnMainWindowClose = core.ShutdownOnMainWindowClose
	ShutdownOnExplicit        = core.ShutdownOnExplicit
)

// OpenWindow opens a window showing content, which belongs to the
// current owner and closes when that owner goes. A window with
// opts.Key already open shows and takes the focus instead:
//
//	settings := desktop.OpenWindow(cx, desktop.WindowOptions{Title: "Settings", Key: "settings"}, Settings)
func OpenWindow(cx *core.Scope, opts WindowOptions, content func(cx *core.Scope) core.Node) *Window {
	return cx.OpenWindow(opts, content)
}

// OpenPanel opens a hidden panel showing content: a borderless window
// above the others, which takes the keyboard without activating the app,
// for Window.Toggle, a global shortcut or a tray icon to show, as a
// launcher or a tray icon's panel does.
func OpenPanel(cx *core.Scope, opts PanelOptions, content func(cx *core.Scope) core.Node) *Window {
	return cx.OpenPanel(opts, content)
}

// CurrentWindow returns the window cx is in, nil in the app's own scope.
func CurrentWindow(cx *core.Scope) *Window { return cx.CurrentWindow() }

// SetShutdownMode says when the app ends by itself: as its last window
// closes (the default), as its main window closes, or only by
// core.Scope.Quit, as a tray app does.
func SetShutdownMode(cx *core.Scope, mode ShutdownMode) { cx.SetShutdownMode(mode) }

// SetMainWindow makes w the main window: the one
// ShutdownOnMainWindowClose follows and reopening the app shows. The
// window nsgo.SetMainView opens is main.
func SetMainWindow(cx *core.Scope, w *Window) { cx.SetMainWindow(w) }

// MainWindow returns the main window, nil once it closed.
func MainWindow(cx *core.Scope) *Window { return cx.MainWindow() }

// OnReopen runs fn when the user opens the app again while it runs: by
// clicking it in the Dock or launching it a second time. Without a
// handler the main window shows and takes the focus.
func OnReopen(cx *core.Scope, fn func()) { cx.OnReopen(fn) }

// NewTray adds a tray icon, which the current owner keeps: an image, or a
// view that draws it and may change, a panel its primary click shows
// under it, and a menu its secondary click shows.
func NewTray(cx *core.Scope, opts TrayOptions) *Tray { return cx.NewTray(opts) }

// RegisterHotKey runs fn when key, in core.Scope.OnKey's form such as
// "alt+space", is pressed whichever app has the focus, while the current
// owner lives.
func RegisterHotKey(cx *core.Scope, key string, fn func()) { cx.RegisterHotKey(key, fn) }

// SetDockVisible shows the app in the Dock and the app switcher, or
// leaves it to its tray icon and windows, on macOS; nsgo.HideDock starts
// it hidden.
func SetDockVisible(cx *core.Scope, on bool) { cx.SetDockVisible(on) }

// SetLaunchAtLogin has the system start the app as the user logs in, or
// no more. macOS takes only an app in a bundle, as nsgo build makes it.
func SetLaunchAtLogin(cx *core.Scope, on bool) { cx.SetLaunchAtLogin(on) }

// LaunchAtLogin calls done with whether the system starts the app at
// login.
func LaunchAtLogin(cx *core.Scope, done func(on bool)) { cx.LaunchAtLogin(done) }
