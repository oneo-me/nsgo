// Package nsgo runs an NSGo app on the platform it is built for, either
// a root view, which shows in the app's main window:
//
//	func main() { nsgo.Run(Root) }
//
// or the app's function, which opens its windows, panels and tray icons
// itself (package desktop), as an Avalonia application does:
//
//	func main() { nsgo.Run(App) }
//
//	func App(app *core.Scope) {
//		desktop.SetShutdownMode(app, desktop.ShutdownOnExplicit)
//		desktop.NewTray(app, desktop.TrayOptions{Icon: icon, Panel: desktop.OpenPanel(app, desktop.PanelOptions{}, Panel)})
//	}
//
// Build tags pick the backend. The nsgo tool sets the app's name and ID
// from nsgo.yaml.
package nsgo

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/engine"
)

// Set by the nsgo tool with -ldflags -X.
var (
	name = "NSGo"
	id   = ""
	fps  = "0"
	// appearance is nsgo.yaml's: "light" or "dark" starts the app in it.
	appearance = ""
)

// App builds an app's root element once.
type App = engine.App

// Main is what Run runs: a root view, or the app's function.
type Main interface {
	func(cx *core.Scope) core.Node | func(app *core.Scope)
}

// Run runs main and the platform's event loop. Call it from main.
//
// A root view shows in the app's main window, or fills the app on
// phones and the web. The app's function runs once, in the app's own
// scope, and opens what the app shows: on macOS its windows, panels and
// tray icons (package desktop); everywhere the view SetMainView sets.
// Where the platform opens no windows of the app's (Windows, Linux,
// phones and the web) desktop.OpenWindow returns nil.
//
// On desktops Run returns once the app ends, by its shutdown mode
// (desktop.SetShutdownMode) or core.Scope.Quit, its storage written, so
// main can clean up; on iOS, Android and the web it never returns.
func Run[M Main](main M, opts ...Option) {
	o := options{monitor: fps == "1", width: 480, height: 640, appearance: map[string]core.Appearance{
		"light": core.AppearanceLight, "dark": core.AppearanceDark}[appearance]}
	for _, fn := range opts {
		fn(&o)
	}
	o.constraints = o.constraints.Normalized()
	size := o.constraints.Clamp(core.Size{Width: o.width, Height: o.height})
	o.width, o.height = size.Width, size.Height
	switch m := any(main).(type) {
	case func(cx *core.Scope) core.Node:
		run(devApp(m), nil, o)
	case func(app *core.Scope):
		run(nil, m, o)
	}
}

// SetMainView shows view as the app's main view: in its main window on
// desktops, which it opens, else filling the app. A later call replaces
// the view.
func SetMainView(cx *core.Scope, view func(cx *core.Scope) core.Node) {
	if view != nil {
		view = devApp(view)
	}
	cx.SetMainView(view)
}

// appOf returns the app's function of what Run runs: app, or one showing
// root as its main view.
func appOf(root App, app func(*core.Scope)) func(*core.Scope) {
	if app != nil {
		return app
	}
	return func(cx *core.Scope) { cx.SetMainView(root) }
}

// rootOf returns the root view of what Run runs on a platform that
// opens no windows: root, or app's main view.
func rootOf(root App, app func(*core.Scope)) App {
	if app != nil {
		return core.AppRoot(app)
	}
	return root
}

// Option configures Run and NewView.
type Option func(*options)

type options struct {
	components core.NativeComponents
	// fonts are the font files to register, under the app's resources.
	fonts []string
	// monitor shows the performance monitor over the app (macOS).
	monitor bool
	// material is the material behind the main window's content, and
	// width and height its size.
	material      core.WindowMaterial
	width, height core.Points
	// constraints limit the main window.
	constraints core.WindowConstraints
	// appearance is the appearance the app starts in.
	appearance core.Appearance
	// hideDock starts the app out of the Dock (macOS).
	hideDock bool
	// What NewView takes: the parent window, the data directory, the
	// error handler and the host channel.
	parent  uintptr
	dataDir string
	onError func(error)
	host    *core.Host
}

// Appearance starts the app in appearance whatever the system's, its
// window and native chrome included, as core.Scope.SetAppearance later
// changes it.
func Appearance(appearance core.Appearance) Option {
	return func(o *options) { o.appearance = appearance }
}

// WindowSize sizes the main window's content, in points: 480 by 640 by
// default. desktop.OpenWindow's windows take it where they set no size.
func WindowSize(width, height core.Points) Option {
	return func(o *options) { o.width, o.height = width, height }
}

// WindowConstraints limits the main window's content size and what the
// user may do to it: desktop.SetWindowConstraints says how, and changes
// them later. desktop.OpenWindow's windows take their own from
// WindowOptions.
func WindowConstraints(c core.WindowConstraints) Option {
	return func(o *options) { o.constraints = c }
}

// HideDock starts the app out of the Dock and the app switcher, on
// macOS, as a tray or background app runs: set before the app starts,
// its icon never shows there. desktop.SetDockVisible changes it later.
func HideDock() Option {
	return func(o *options) { o.hideDock = true }
}

// FPS shows (or hides) the performance monitor over the app: frame rate,
// frame and commit costs, CPU, GPU and memory. nsgo.yaml's dev.fps and
// build.fps, or the nsgo tool's -fps flag, turn it on.
func FPS(on bool) Option {
	return func(o *options) { o.monitor = on }
}

// WindowMaterial starts the main window with material behind its content,
// where the app draws nothing opaque: a system vibrancy material on
// macOS and a system backdrop on Windows 11 22H2 or later. Other targets
// ignore it. desktop.SetWindowMaterial changes it later, which on
// Windows needs a window started with one.
func WindowMaterial(material core.WindowMaterial) Option {
	return func(o *options) { o.material = material }
}

// Fonts registers the font files at paths, slash-separated under the
// app's resources directory (TrueType, OpenType or collections), before
// the first view: text names them by their families, as Font.Family or a
// Style's FontFamily.
func Fonts(paths ...string) Option {
	return func(o *options) { o.fonts = append(o.fonts, paths...) }
}

// Component registers a native component the app's Native elements name:
// factory makes the platform view for each one. Register the components
// of each platform in files built for it.
func Component(name string, factory core.NativeFactory) Option {
	return func(o *options) {
		if o.components == nil {
			o.components = core.NativeComponents{}
		}
		o.components[name] = factory
	}
}
