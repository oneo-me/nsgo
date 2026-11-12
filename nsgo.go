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
// Build tags pick the backend. The nsgo tool builds the app's identity
// into it (core.App), and its updates where nsgo.yaml configures them
// (package updater), from nsgo.yaml. The engine's own command-line
// flags, such as --nsgo-fps, leave os.Args before main runs (launchFlags).
package nsgo

import (
	"log"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/engine"
)

// Set by the nsgo tool with -ldflags -X.
var (
	// appearance is nsgo.yaml's: "light" or "dark" starts the app in it.
	appearance = ""
	// updateFeed is this target's appcast, updateKey the public key its
	// downloads are signed with, and updateAutomatic "1" when the app
	// checks on its own (updates.go).
	updateFeed, updateKey, updateAutomatic = "", "", ""
)

// name and id are the app's name, "NSGo" where the tool set none, and ID.
var name, id = appName(), core.App().ID

func appName() string {
	if n := core.App().Name; n != "" {
		return n
	}
	return "NSGo"
}

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
// (desktop.SetShutdownMode), core.Scope.Quit or desktop.Restart, its
// storage written, so main can clean up; on iOS, Android and the web it
// never returns. After desktop.Restart it has started the app again,
// which waits for this process to exit.
func Run[M Main](main M, opts ...Option) {
	awaitRestarted()
	o := options{width: 480, height: 640, appearance: map[string]core.Appearance{
		"light": core.AppearanceLight, "dark": core.AppearanceDark}[appearance]}
	for _, fn := range opts {
		fn(&o)
	}
	launch.apply(&o)
	o.constraints = o.constraints.Normalized()
	if o.frame.Width > 0 && o.frame.Height > 0 {
		o.width, o.height = o.frame.Width, o.frame.Height
	}
	size := o.constraints.Clamp(core.Size{Width: o.width, Height: o.height})
	o.width, o.height = size.Width, size.Height
	switch m := any(main).(type) {
	case func(cx *core.Scope) core.Node:
		run(devApp(launch.localized(m)), nil, o)
	case func(app *core.Scope):
		run(nil, launch.localizedApp(m), o)
	}
	// nsgo dev ends with the app it started, so a restart there only quits.
	if core.Restarting() && !devBuild {
		if err := restart(launchArgs); err != nil {
			log.Printf("nsgo: restarting: %v", err)
		}
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
	// monitor shows the performance monitor over the app: --nsgo-fps.
	monitor bool
	// material is the material behind the main window's content, and
	// width and height its size.
	material      core.WindowMaterial
	width, height core.Points
	// frame is where the main window opens, when it moves.
	frame core.WindowFrame
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

// WindowFrame opens the main window with frame f, as desktop.Frame
// reported it, instead of centered at WindowSize: an app restores its
// window with it. A window no screen would show, as on a screen that
// went away, opens centered on the main screen. Linux takes only the size.
func WindowFrame(f core.WindowFrame) Option {
	return func(o *options) { o.frame = f }
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
