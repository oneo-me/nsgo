// Package desktop holds what only desktops have: the window, its title
// bar and the menus, as NSH's nsh/desktop. It builds on every platform,
// so one app runs everywhere: on phones and tablets its functions do
// nothing, and IsAvailable says whether they act.
//
// A title bar of the app's own calls SetTitleBar once, marks its
// background with core.View's Desktop.WindowDrag and keeps clear of the edges
// TitleBar reports; SetWindowControls moves or hides the system's
// controls in it. A window over a material leaves transparent what
// should show it while ShowsMaterial.
package desktop

import "github.com/nsgo-dev/nsgo/core"

type (
	// TitleBarStyle is whether the system draws the window's title bar.
	TitleBarStyle = core.TitleBarStyle
	// WindowFrame is where a window moves and how large it becomes, in
	// points from the main screen's top left.
	WindowFrame = core.WindowFrame
	// WindowLevel is where a window stacks among the others.
	WindowLevel = core.WindowLevel
	// WindowConstraints limit a window's size and what the user may do
	// to it.
	WindowConstraints = core.WindowConstraints
	// MenuBarMode says when Windows and Linux show the menu bar.
	MenuBarMode = core.MenuBarMode
	// WindowControls is how a hidden title bar shows the system's window
	// controls: hidden for the app to draw its own, or macOS's traffic
	// lights moved.
	WindowControls = core.WindowControls
	// WindowMaterial is a translucent material behind the window's
	// content: one of macOS's vibrancy materials, or of Windows 11's
	// system backdrops, each platform showing its closest.
	WindowMaterial = core.WindowMaterial
)

const (
	TitleBarDefault = core.TitleBarDefault
	TitleBarHidden  = core.TitleBarHidden
)

const (
	WindowLevelNormal   = core.WindowLevelNormal
	WindowLevelFloating = core.WindowLevelFloating
	WindowLevelDesktop  = core.WindowLevelDesktop
)

const (
	MenuBarShown    = core.MenuBarShown
	MenuBarAutoHide = core.MenuBarAutoHide
)

const (
	WindowMaterialNone = core.WindowMaterialNone
	// The materials of window areas.
	WindowMaterialWindow      = core.WindowMaterialWindow
	WindowMaterialContent     = core.WindowMaterialContent
	WindowMaterialSidebar     = core.WindowMaterialSidebar
	WindowMaterialHeader      = core.WindowMaterialHeader
	WindowMaterialTitlebar    = core.WindowMaterialTitlebar
	WindowMaterialUnderWindow = core.WindowMaterialUnderWindow
	WindowMaterialUnderPage   = core.WindowMaterialUnderPage
	// The materials of transient surfaces, Acrylic on Windows.
	WindowMaterialMenu         = core.WindowMaterialMenu
	WindowMaterialPopover      = core.WindowMaterialPopover
	WindowMaterialHUD          = core.WindowMaterialHUD
	WindowMaterialSheet        = core.WindowMaterialSheet
	WindowMaterialTooltip      = core.WindowMaterialTooltip
	WindowMaterialSelection    = core.WindowMaterialSelection
	WindowMaterialFullScreenUI = core.WindowMaterialFullScreenUI
	// Windows 11's system backdrops: Mica, Acrylic and Mica Alt.
	WindowMaterialMica    = core.WindowMaterialMica
	WindowMaterialAcrylic = core.WindowMaterialAcrylic
	WindowMaterialTabbed  = core.WindowMaterialTabbed
)

// IsAvailable reports whether the app runs on macOS, Windows or Linux,
// where this package's functions act.
func IsAvailable(cx *core.Scope) bool {
	switch cx.Env().OS() {
	case core.OSMacOS, core.OSWindows, core.OSLinux:
		return true
	}
	return false
}

// SetTitle sets the window's title on desktops and the page's on the web.
func SetTitle(cx *core.Scope, title string) { cx.SetTitle(title) }

// SetTitleBar hides the system's title bar so the app can draw its own
// height points tall (0: the platform's height), or restores it. The
// platform centers its window controls in that height. TitleBar then
// reports the area they keep.
func SetTitleBar(cx *core.Scope, style TitleBarStyle, height core.Points) {
	cx.SetTitleBar(style, height)
}

// TitleBar returns what a hidden title bar covers: Top is its height,
// Left and Right the widths the system's window controls take at its
// ends. It is zero while the system draws the title bar, in full screen
// and off desktops.
func TitleBar(cx *core.Scope) core.Edges { return cx.Env().TitleBar() }

// SetWindowControls hides the system's window controls in a hidden
// title bar, or moves macOS's traffic lights in it, from any time on:
//
//	desktop.SetWindowControls(cx, desktop.WindowControls{TrafficLights: core.Point{X: 20, Y: 18}})
//
// Offset moves them from there, as a transition does each frame. TitleBar
// follows where they are. The zero WindowControls restores the
// platform's.
func SetWindowControls(cx *core.Scope, c WindowControls) { cx.SetWindowControls(c) }

// SetWindowMaterial puts m behind the window's content, or makes the
// window opaque again with WindowMaterialNone; nsgo.WindowMaterial
// starts the window with one. macOS changes it any time; Windows 11 22H2
// and later only in a window started with a material, others staying
// opaque.
func SetWindowMaterial(cx *core.Scope, m WindowMaterial) { cx.SetWindowMaterial(m) }

// ShowsMaterial reports whether a material shows behind the window's
// content, where the app leaves it transparent. Where it is false, give
// those parts a background of their own.
func ShowsMaterial(cx *core.Scope) bool {
	return cx.Env().WindowMaterial() != core.WindowMaterialNone
}

// Maximized reports whether the window is maximized or zoomed, so a
// maximize button of the app's own can show restore.
func Maximized(cx *core.Scope) bool { return cx.Env().Maximized() }

// FullScreen reports whether the window is in full screen.
func FullScreen(cx *core.Scope) bool { return cx.Env().FullScreen() }

// Frame returns where the window is and how large, as SetWindowFrame and
// nsgo.WindowFrame take them, changing as it moves and resizes. While
// the window is maximized or in full screen it is that frame, so an app
// remembering a window keeps the one it last had otherwise. Linux knows
// only the size GTK keeps, which ignores those states, and Move says no
// position came; zero without a window.
func Frame(cx *core.Scope) WindowFrame { return cx.Env().WindowFrame() }

// MinimizeWindow minimizes the window.
func MinimizeWindow(cx *core.Scope) { cx.MinimizeWindow() }

// ToggleMaximize maximizes (zooms) the window, or restores it.
func ToggleMaximize(cx *core.Scope) { cx.ToggleMaximize() }

// ToggleFullScreen enters or leaves full screen.
func ToggleFullScreen(cx *core.Scope) { cx.ToggleFullScreen() }

// CloseWindow closes the window as its close button does: the window's
// OnClosing handlers may keep it, and the app ends by its shutdown mode.
func CloseWindow(cx *core.Scope) { cx.CloseWindow() }

// SetWindowFrame moves and sizes the window: f's top left corner is in
// points from the main screen's top left on every platform, and fields
// it leaves out keep their values, so a window given only a new height
// keeps its top edge. macOS animates the change when f asks. Linux sets
// the size GTK keeps for the window, and leaves its position to the
// compositor.
func SetWindowFrame(cx *core.Scope, f WindowFrame) { cx.SetWindowFrame(f) }

// SetWindowLevel puts the window at level among the others, on macOS
// and Windows; Linux leaves stacking to the compositor.
func SetWindowLevel(cx *core.Scope, level WindowLevel) { cx.SetWindowLevel(level) }

// SetWindowConstraints limits the window's content size and says whether
// it resizes, minimizes and maximizes; the zero WindowConstraints limits
// nothing. The window moves into its new sizes, and MinimizeWindow and
// ToggleMaximize do nothing where it may not, as its buttons:
//
//	desktop.SetWindowConstraints(cx, desktop.WindowConstraints{MinWidth: 480, MinHeight: 320, NoMaximize: true})
//
// A window sets them as it opens with WindowOptions.Constraints, and the
// main window with nsgo.WindowConstraints. On Linux, GTK 4 has neither
// maximum sizes nor a way to take the title bar's buttons, so only
// NoResize, or a maximum equal to the minimum, limits a window from
// above, and the user's buttons stay.
func SetWindowConstraints(cx *core.Scope, c WindowConstraints) { cx.SetWindowConstraints(c) }

// OrderFront shows the window, restoring it when minimized or hidden,
// brings it to the front of its level and activates the app.
func OrderFront(cx *core.Scope) { cx.OrderFront() }

// SetMenuBar says when Windows and Linux show the menu bar the window
// draws for the application menu; macOS's is the system's and always
// shows.
func SetMenuBar(cx *core.Scope, mode MenuBarMode) { cx.SetMenuBar(mode) }
