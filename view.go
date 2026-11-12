package nsgo

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// View is an app in a native view of a host app's own window, run by the
// host's own event loop: an NSView on macOS, a GtkWidget on Linux, a
// child window on Windows. Go desktop apps embed NSGo apps with it; iOS
// and Android apps embed them with the frameworks `nsgo build -embed`
// makes. Each View is an engine of its own, so a process shows as many
// as it likes, in one window or several. Call its methods on the
// platform's main thread.
type View struct {
	p    platformView
	host *core.Host
	env  engine.HostEnvironment
}

// platformView is a backend's embedded view.
type platformView interface {
	Handle() uintptr
	SetHostEnvironment(engine.HostEnvironment)
	SetTokens(core.Tokens)
	Close()
}

// NewView makes a view showing app, for a host that runs the platform's
// event loop itself. The host puts Handle in its window and sizes it;
// the app starts once it is there, so settings made before are in its
// first frame. Component, Fonts, Appearance, Parent, DataDir, OnError and
// HostChannel apply; WindowMaterial, WindowSize and HideDock are
// Run's, as are the engine's command-line flags (--nsgo-fps). Close the
// view when the host is done with it.
func NewView(app App, opts ...Option) *View {
	o := options{appearance: map[string]core.Appearance{
		"light": core.AppearanceLight, "dark": core.AppearanceDark}[appearance]}
	for _, fn := range opts {
		fn(&o)
	}
	if o.host == nil {
		o.host = core.NewHost()
	}
	h := engine.HostOptions{Channel: o.host, Settings: &engine.HostSettings{}, OnError: o.onError, DataDir: o.dataDir}
	return &View{p: newView(app, o, h), host: o.host}
}

// Handle returns the native view: an NSView pointer on macOS, a
// GtkWidget pointer on Linux, the child window's HWND on Windows.
func (v *View) Handle() uintptr { return v.p.Handle() }

// Host returns the channel between the host and the app: the app's
// launch parameters, the events the host sends it and the functions it
// calls (core.Scope.CallHost), and the host's URL handler.
func (v *View) Host() *core.Host { return v.host }

// SetLocale has the app format for locale, a BCP 47 tag such as "de-CH",
// in place of the system's language; "" follows the system again. The
// layout direction stays the system's.
func (v *View) SetLocale(locale string) {
	v.env.Locale = locale
	v.p.SetHostEnvironment(v.env)
}

// SetFontScale sets the text size multiplier in place of the system's;
// 0 follows the system again.
func (v *View) SetFontScale(scale float32) {
	v.env.FontScale = scale
	v.p.SetHostEnvironment(v.env)
}

// SetAppearance shows the app light or dark in place of the system's
// appearance, the platform's own chrome in the view too;
// core.AppearanceSystem follows the system again. An appearance the app
// forces still wins.
func (v *View) SetAppearance(a core.Appearance) {
	v.env.Appearance = a
	v.p.SetHostEnvironment(v.env)
}

// SetTokens gives the app the values the host themes it with, which it
// reads with core.Env.Token.
func (v *View) SetTokens(t core.Tokens) { v.p.SetTokens(t) }

// Close stops the app, writing its storage, and destroys its native
// components and the view.
func (v *View) Close() {
	v.p.Close()
	engine.SyncStorage()
}

// Parent is the window NewView's view is made in: on Windows, where a
// view is a child window, the HWND of the host's window, which it needs.
// Other platforms ignore it.
func Parent(handle uintptr) Option {
	return func(o *options) { o.parent = handle }
}

// DataDir keeps a view's app data, its storage included, in dir rather
// than in the system's directory for the app's ID.
func DataDir(dir string) Option {
	return func(o *options) { o.dataDir = dir }
}

// OnError has fn hear each failure a view's app does not catch with a
// core.Catch, on the app's goroutine; it must not block.
func OnError(fn func(error)) Option {
	return func(o *options) { o.onError = fn }
}

// HostChannel has a view use h as its channel to the host, made before
// it, rather than a new one.
func HostChannel(h *core.Host) Option {
	return func(o *options) { o.host = h }
}
