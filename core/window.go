package core

import (
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The windows, panels and tray icons an app opens itself, its global
// shortcuts and its lifetime: the core's part of package desktop and of
// nsgo.SetMainView. One graph runs every window: each is a tree of its
// own, a surface the platform attaches, so the windows share signals.
// Where the platform opens no windows (RootOptions.Windows is nil:
// phones, the web, a host's view, a test) the app has one surface, the
// first tree, and SetMainView shows its view there.

// ShutdownMode says when a desktop app ends by itself: the core's part of
// desktop.SetShutdownMode, which apps use. Scope.Quit ends it in any
// mode.
type ShutdownMode uint8

const (
	// ShutdownOnLastWindowClose ends the app as its last window closes;
	// panels and tray icons do not count.
	ShutdownOnLastWindowClose ShutdownMode = iota
	// ShutdownOnMainWindowClose ends it as its main window closes.
	ShutdownOnMainWindowClose
	// ShutdownOnExplicit ends it only by Scope.Quit, as a tray app does.
	ShutdownOnExplicit
)

// WindowOptions are a window's: the core's part of desktop.WindowOptions,
// which apps use.
type WindowOptions struct {
	// Title is the window's; empty is the app's name.
	Title string
	// Width and Height are the content's size in points; zero is the
	// main window's (nsgo.WindowSize).
	Width, Height Points
	// Frame, when it moves, opens the window there at its size instead of
	// centered, as an app restoring a window does: Width and Height give
	// way to it, and a window no screen would show opens centered.
	Frame WindowFrame
	// Key, when set, finds a window opened with the same key and still
	// open: OpenWindow shows and focuses it instead of opening another.
	Key string
	// Hidden opens the window without showing it.
	Hidden bool
	// Material is the material behind the content (WindowMaterial).
	Material WindowMaterial
	// Constraints limit the window's size, which Width and Height keep
	// to, and what the user may do to it.
	Constraints WindowConstraints
}

// PanelOptions are a panel's: the core's part of desktop.PanelOptions,
// which apps use.
type PanelOptions struct {
	// Width and Height are the content's size in points.
	Width, Height Points
	// HideOnBlur hides the panel as it loses the focus, as a tray icon's
	// panel or a launcher does.
	HideOnBlur bool
	// Material is the material behind the content (WindowMaterial).
	Material WindowMaterial
}

// Window is a window or a panel the app opened: the core's part of
// desktop.Window, which apps use. Its methods do nothing on a nil
// Window, which OpenWindow returns where the platform opens no windows,
// or once it closed.
type Window struct {
	t       *Tree
	scope   *Scope
	kind    driver.SurfaceKind
	key     string
	visible *Signal[bool]
	closed  bool
	closing []*closingHandler
}

type closingHandler struct {
	owner *Scope
	fn    func(*WindowClosing)
}

// WindowClosing is a close the user asked for, as with the window's
// close button: the core's part of desktop.WindowClosing, which apps use.
type WindowClosing struct{ canceled bool }

// Cancel keeps the window open, as an app that hides it instead does.
func (c *WindowClosing) Cancel() { c.canceled = true }

// OpenWindow is the core's part of desktop.OpenWindow, which apps use: it
// opens a window showing content, which belongs to the current owner and
// closes when that owner goes. It returns nil where the platform opens no
// windows.
func (cx *Scope) OpenWindow(opts WindowOptions, content func(cx *Scope) Node) *Window {
	rt := cx.rt
	if rt.windows == nil {
		return nil
	}
	if opts.Key != "" {
		if w := rt.keyed[opts.Key]; w != nil && !w.closed {
			w.Focus()
			return w
		}
	}
	if opts.Width <= 0 || opts.Height <= 0 {
		opts.Width, opts.Height = rt.windows.Width, rt.windows.Height
	}
	if opts.Frame.Move && opts.Frame.Width > 0 && opts.Frame.Height > 0 {
		opts.Width, opts.Height = opts.Frame.Width, opts.Frame.Height
	} else {
		opts.Frame = WindowFrame{}
	}
	limits := opts.Constraints.Normalized()
	size := limits.Clamp(Size{Width: opts.Width, Height: opts.Height})
	w := rt.openSurface(cx.current(), driver.SurfaceConfig{Kind: driver.SurfaceWindow, Title: opts.Title,
		Width: size.Width, Height: size.Height, Frame: opts.Frame, Hidden: opts.Hidden, Material: opts.Material, Constraints: limits}, content)
	if opts.Key != "" {
		w.key = opts.Key
		if rt.keyed == nil {
			rt.keyed = map[string]*Window{}
		}
		rt.keyed[opts.Key] = w
	}
	return w
}

// OpenPanel is the core's part of desktop.OpenPanel, which apps use: it
// opens a hidden panel showing content, for Show, Toggle or a tray icon
// to show. It returns nil where the platform opens no windows.
func (cx *Scope) OpenPanel(opts PanelOptions, content func(cx *Scope) Node) *Window {
	rt := cx.rt
	if rt.windows == nil {
		return nil
	}
	if opts.Width <= 0 || opts.Height <= 0 {
		opts.Width, opts.Height = 360, 420
	}
	return rt.openSurface(cx.current(), driver.SurfaceConfig{Kind: driver.SurfacePanel, Width: opts.Width,
		Height: opts.Height, Hidden: true, HideOnBlur: opts.HideOnBlur, Material: opts.Material}, content)
}

// openSurface opens a surface of cfg's kind showing content under owner:
// a tree of its own, which the platform attaches as ItemOpenSurface asks.
func (rt *runtime) openSurface(owner *Scope, cfg driver.SurfaceConfig, content func(cx *Scope) Node) *Window {
	app := rt.tree
	s := &Scope{}
	owner.adopt(s)
	t := newTree(s, app.text)
	rt.surfaceID++
	t.id, cfg.ID = rt.surfaceID, rt.surfaceID
	w := &Window{t: t, scope: s, kind: cfg.Kind, visible: NewSignal(s, !cfg.Hidden)}
	t.window = w
	rt.surfaces = append(rt.surfaces, t)
	// It starts in the app's environment until its platform reports its own.
	env := app.reported
	env.TitleBar, env.Maximized, env.FullScreen, env.WindowFrame, env.WindowMaterial = Edges{}, false, false, WindowFrame{}, cfg.Material
	t.SetEnvironment(env)
	t.Resize(Size{Width: cfg.Width, Height: cfg.Height})
	app.commands = append(app.commands, driver.MountItem{Kind: driver.ItemOpenSurface, Surface: &cfg})
	app.changed = true
	s.addCleanup(w.gone)
	if content != nil {
		t.Mount(content)
	}
	return w
}

// gone ends the window as its scope goes: the engine sends its platform
// WindowDestroy, and the app may end with it.
func (w *Window) gone() {
	if w.closed {
		return
	}
	w.closed = true
	t := w.t
	t.closed = true
	rt := t.rt
	if w.key != "" && rt.keyed[w.key] == w {
		delete(rt.keyed, w.key)
	}
	if w.kind != driver.SurfaceWindow || rt.root.disposed {
		return
	}
	main := rt.main == w
	if main {
		rt.main = nil
	}
	switch rt.shutdown {
	case ShutdownOnLastWindowClose:
		for _, o := range rt.surfaces {
			if o.window.kind == driver.SurfaceWindow && !o.closed {
				return
			}
		}
		rt.quit(false)
	case ShutdownOnMainWindowClose:
		if main {
			rt.quit(false)
		}
	}
}

// command sends c to the window's platform.
func (w *Window) command(c WindowCommand) {
	t := w.t
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemWindow, Window: &c})
	t.changed = true
}

// Show shows the window without taking the focus from another app.
func (w *Window) Show() {
	if w == nil || w.closed {
		return
	}
	w.visible.Set(true)
	w.command(WindowCommand{Action: driver.WindowShow})
}

// showAt shows a panel under tray icon id.
func (w *Window) showAt(tray uint32) {
	w.visible.Set(true)
	w.command(WindowCommand{Action: driver.WindowShow, Anchor: tray})
}

// Hide hides the window, keeping it and what it shows.
func (w *Window) Hide() {
	if w == nil || w.closed {
		return
	}
	w.visible.Set(false)
	w.command(WindowCommand{Action: driver.WindowHide})
}

// Toggle hides the window if it shows, else shows and focuses it.
func (w *Window) Toggle() {
	if w == nil || w.closed {
		return
	}
	if w.visible.GetUntracked() {
		w.Hide()
	} else {
		w.Focus()
	}
}

// Focus shows the window in front of the others and activates the app.
func (w *Window) Focus() {
	if w == nil || w.closed {
		return
	}
	w.visible.Set(true)
	w.command(WindowCommand{Action: driver.WindowOrderFront})
}

// Close closes the window for good, as its owner going does, without
// asking OnClosing.
func (w *Window) Close() {
	if w == nil || w.closed {
		return
	}
	w.scope.Dispose()
}

// Visible reports whether the window shows, tracked.
func (w *Window) Visible() bool { return w != nil && !w.closed && w.visible.Get() }

// Closed reports whether the window is closed.
func (w *Window) Closed() bool { return w == nil || w.closed }

// OnClosing runs fn when the user asks to close the window, newest first,
// while the current owner of cx lives; fn may cancel the close.
func (w *Window) OnClosing(cx *Scope, fn func(*WindowClosing)) {
	if w == nil || w.closed {
		return
	}
	h := &closingHandler{owner: cx.current(), fn: fn}
	w.closing = append(w.closing, h)
	h.owner.addCleanup(func() { w.closing = slices.DeleteFunc(w.closing, func(o *closingHandler) bool { return o == h }) })
}

// requestClose asks the OnClosing handlers, then closes the window
// unless one canceled.
func (w *Window) requestClose() {
	c := &WindowClosing{}
	for i := len(w.closing) - 1; i >= 0; i-- {
		h := w.closing[i]
		w.t.within(h.owner, func() { h.fn(c) })
	}
	if !c.canceled {
		w.t.within(w.t.root, w.Close)
	}
}

// CurrentWindow is the core's part of desktop.CurrentWindow, which apps
// use: the window cx is in, nil in the app's own scope or where the
// platform opens no windows.
func (cx *Scope) CurrentWindow() *Window {
	if t := cx.surf; t != nil && t.window != nil && !t.window.closed {
		return t.window
	}
	return nil
}

// SetShutdownMode is the core's part of desktop.SetShutdownMode, which
// apps use.
func (cx *Scope) SetShutdownMode(m ShutdownMode) { cx.rt.shutdown = m }

// SetMainWindow is the core's part of desktop.SetMainWindow, which apps
// use: the window ShutdownOnMainWindowClose follows and the Dock shows
// again.
func (cx *Scope) SetMainWindow(w *Window) { cx.rt.main = w }

// MainWindow is the core's part of desktop.MainWindow, which apps use.
func (cx *Scope) MainWindow() *Window {
	if w := cx.rt.main; w != nil && !w.closed {
		return w
	}
	return nil
}

// SetMainView is the core's part of nsgo.SetMainView, which apps use: it
// shows view in the main window, which it opens where the platform opens
// windows, or in the app's one surface elsewhere. A later call replaces
// the view.
func (cx *Scope) SetMainView(view func(cx *Scope) Node) {
	rt := cx.rt
	rt.mainViewSignal().Set(view)
	if rt.windows == nil || view == nil || rt.ownMain != nil && !rt.ownMain.closed {
		return
	}
	rt.ownMain = rt.root.OpenWindow(*rt.windows, rt.showMainView)
	rt.main = rt.ownMain
}

func (rt *runtime) mainViewSignal() *Signal[func(cx *Scope) Node] {
	if rt.mainView == nil {
		rt.mainView = NewSignalFunc[func(cx *Scope) Node](rt.root, nil, func(a, b func(cx *Scope) Node) bool { return false })
	}
	return rt.mainView
}

// showMainView shows the view SetMainView set.
func (rt *runtime) showMainView(*Scope) Node {
	return Dynamic(func(cx *Scope) Node {
		if v := rt.mainViewSignal().Get(); v != nil {
			return Func(v)
		}
		return nil
	})
}

// AppRoot is the root view nsgo.Run mounts for app, which opens its views
// itself: app runs once, and the root shows what SetMainView sets, unless
// the platform opens windows, where it stays empty.
func AppRoot(app func(cx *Scope)) func(cx *Scope) Node {
	return func(cx *Scope) Node {
		app(cx)
		if cx.rt.windows != nil {
			return nil
		}
		return cx.rt.showMainView(cx)
	}
}

// quit asks the platform to end the app, saying whether it restarts.
func (rt *runtime) quit(restart bool) {
	if t := rt.tree; t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemQuit, On: restart})
		t.changed = true
	}
}

// app sends item to the app's own surface, the first.
func (rt *runtime) app(item driver.MountItem) {
	if t := rt.tree; t != nil {
		t.commands = append(t.commands, item)
		t.changed = true
	}
}

type reopenHandler struct {
	owner *Scope
	fn    func()
}

// OnReopen is the core's part of desktop.OnReopen, which apps use: fn
// runs, while the current owner lives, when the user opens the app again
// while it runs, from the Dock or by launching it a second time. With no
// handler the main window shows and takes the focus.
func (cx *Scope) OnReopen(fn func()) {
	rt := cx.rt
	h := &reopenHandler{owner: cx.current(), fn: fn}
	rt.reopens = append(rt.reopens, h)
	h.owner.addCleanup(func() { rt.reopens = slices.DeleteFunc(rt.reopens, func(o *reopenHandler) bool { return o == h }) })
}

func (rt *runtime) reopen() {
	if len(rt.reopens) == 0 {
		if rt.windows == nil {
			// Where the platform opens no windows of the app's (Windows and Linux hand a second launch here), the app's one surface is its main window: bring its window to the front, restored if minimized.
			rt.root.OrderFront()
			return
		}
		rt.main.Focus()
		return
	}
	for _, h := range slices.Clone(rt.reopens) {
		rt.tree.within(h.owner, h.fn)
	}
}

// TrayOptions are a tray icon's: the core's part of desktop.TrayOptions,
// which apps use.
type TrayOptions struct {
	// Icon is the icon as PNG; Template draws it in the menu bar's
	// colors on macOS, as its own icons are.
	Icon     []byte
	Template bool
	// View draws the icon instead, Width points wide (22 by default) and
	// as tall as the menu bar, so it can change and move: on macOS.
	View  func(cx *Scope) Node
	Width Points
	// Title is text beside the icon, on macOS; Tooltip shows on hover.
	Title, Tooltip string
	// Panel shows under the icon on a primary click, and hides on the
	// next.
	Panel *Window
	// OnClick runs on a primary click without a Panel.
	OnClick func()
	// Menu returns the menu a secondary click shows, and a primary click
	// without a Panel or OnClick.
	Menu func() []MenuItem
}

// Tray is a tray icon: the core's part of desktop.Tray, which apps use.
// Its methods do nothing on a nil Tray, which NewTray returns where the
// platform opens no windows.
type Tray struct {
	rt      *runtime
	owner   *Scope
	opts    TrayOptions
	cfg     driver.TrayConfig
	view    *Window
	removed bool
}

// NewTray is the core's part of desktop.NewTray, which apps use: it adds
// a tray icon, the macOS menu bar's or the Windows notification area's,
// which the current owner keeps.
func (cx *Scope) NewTray(opts TrayOptions) *Tray {
	rt := cx.rt
	if rt.windows == nil {
		return nil
	}
	owner := cx.current()
	rt.trayID++
	tr := &Tray{rt: rt, owner: owner, opts: opts, cfg: driver.TrayConfig{ID: rt.trayID, Icon: opts.Icon,
		Template: opts.Template, Title: opts.Title, Tooltip: opts.Tooltip}}
	if opts.View != nil {
		// The view's surface is the next opened.
		tr.cfg.View = rt.surfaceID + 1
	}
	if rt.trays == nil {
		rt.trays = map[uint32]*Tray{}
	}
	rt.trays[tr.cfg.ID] = tr
	tr.send()
	if opts.View != nil {
		width := opts.Width
		if width <= 0 {
			width = 22
		}
		tr.view = rt.openSurface(owner, driver.SurfaceConfig{Kind: driver.SurfaceTrayIcon, Width: width, Height: 22,
			Tray: tr.cfg.ID}, opts.View)
	}
	owner.addCleanup(tr.Remove)
	return tr
}

func (tr *Tray) send() {
	cfg := tr.cfg
	tr.rt.app(driver.MountItem{Kind: driver.ItemTray, Tray: &cfg})
}

// SetIcon shows png as the icon, in the menu bar's colors when template.
func (tr *Tray) SetIcon(png []byte, template bool) {
	if tr == nil || tr.removed {
		return
	}
	tr.cfg.Icon, tr.cfg.Template = png, template
	tr.send()
}

// SetTitle sets the text beside the icon, on macOS.
func (tr *Tray) SetTitle(title string) {
	if tr == nil || tr.removed {
		return
	}
	tr.cfg.Title = title
	tr.send()
}

// SetTooltip sets what shows as the pointer rests on the icon.
func (tr *Tray) SetTooltip(tooltip string) {
	if tr == nil || tr.removed {
		return
	}
	tr.cfg.Tooltip = tooltip
	tr.send()
}

// Remove takes the icon away, as its owner going does.
func (tr *Tray) Remove() {
	if tr == nil || tr.removed {
		return
	}
	tr.removed = true
	delete(tr.rt.trays, tr.cfg.ID)
	if !tr.rt.root.disposed {
		tr.cfg.Removed = true
		tr.send()
	}
	tr.view.Close()
}

// clicked handles a click on the icon: button 0 is the primary.
func (tr *Tray) clicked(button int) {
	o := tr.opts
	switch {
	case button == 0 && o.Panel != nil && !o.Panel.closed:
		if o.Panel.visible.GetUntracked() {
			o.Panel.Hide()
		} else {
			o.Panel.showAt(tr.cfg.ID)
		}
	case button == 0 && o.OnClick != nil:
		tr.rt.tree.within(tr.owner, o.OnClick)
	case o.Menu != nil:
		tr.showMenu()
	}
}

// showMenu shows the icon's menu, which reports as context menus do.
func (tr *Tray) showMenu() {
	t := tr.rt.tree
	var items []MenuItem
	t.within(tr.owner, func() { items = tr.opts.Menu() })
	t.popupClosed(t.popup.id)
	t.alertID++
	t.popup = popupMenu{id: t.alertID, actions: map[uint32]menuAction{}, owner: tr.owner}
	entries := t.menuEntries(items, tr.owner, t.popup.actions)
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemTrayMenu, Menu: entries, Count: t.popup.id,
		Tray: &driver.TrayConfig{ID: tr.cfg.ID}})
	t.changed = true
}

type hotKey struct {
	driver.KeyBinding
	owner *Scope
	fn    func()
}

// RegisterHotKey is the core's part of desktop.RegisterHotKey, which apps
// use: fn runs when key, in OnKey's form, is pressed whichever app has
// the focus, while the current owner lives.
func (cx *Scope) RegisterHotKey(key string, fn func()) {
	rt := cx.rt
	if rt.windows == nil {
		return
	}
	rt.hotKeyID++
	h := &hotKey{KeyBinding: driver.KeyBinding{Key: driver.NormalizeKey(key), ID: rt.hotKeyID}, owner: cx.current(), fn: fn}
	rt.hotKeys = append(rt.hotKeys, h)
	rt.sendHotKeys()
	h.owner.addCleanup(func() {
		rt.hotKeys = slices.DeleteFunc(rt.hotKeys, func(o *hotKey) bool { return o == h })
		if !rt.root.disposed {
			rt.sendHotKeys()
		}
	})
}

func (rt *runtime) sendHotKeys() {
	keys := make([]driver.KeyBinding, len(rt.hotKeys))
	for i, h := range rt.hotKeys {
		keys[i] = h.KeyBinding
	}
	rt.app(driver.MountItem{Kind: driver.ItemHotKeys, Keys: keys})
}

func (rt *runtime) hotKey(id uint32) {
	for _, h := range rt.hotKeys {
		if h.ID == id {
			rt.tree.within(h.owner, h.fn)
			return
		}
	}
}

// SetDockVisible is the core's part of desktop.SetDockVisible, which apps
// use: on macOS it shows the app in the Dock and the app switcher, or
// leaves it to its tray icon and windows.
func (cx *Scope) SetDockVisible(on bool) {
	if cx.rt.windows != nil {
		cx.rt.app(driver.MountItem{Kind: driver.ItemDock, On: on})
	}
}

// SetLaunchAtLogin is the core's part of desktop.SetLaunchAtLogin, which
// apps use: the system starts the app as the user logs in, or no more.
func (cx *Scope) SetLaunchAtLogin(on bool) {
	if cx.rt.windows != nil {
		cx.rt.app(driver.MountItem{Kind: driver.ItemSetLaunchAtLogin, On: on})
	}
}

// LaunchAtLogin is the core's part of desktop.LaunchAtLogin, which apps
// use: done hears whether the system starts the app at login.
func (cx *Scope) LaunchAtLogin(done func(on bool)) {
	t := cx.rt.tree
	if cx.rt.windows == nil || t == nil {
		if done != nil {
			done(false)
		}
		return
	}
	t.request(cx, driver.MountItem{Kind: driver.ItemLaunchAtLogin}, func(ev driver.Event) {
		if done != nil {
			done(ev.Finished)
		}
	})
}

// surfaceEvent handles an event about the tree's surface or the app
// rather than a view, and reports whether ev was one.
type openFilesHandler struct {
	owner *Scope
	fn    func(paths []string)
}

// OnOpenFiles is the core's part of desktop.OnOpenFiles, which apps use:
// fn runs, while the current owner lives, with the paths of files the
// system asks the app to open. Files that came before any handler, as
// those the app launched to open, go to the first one registered.
func (cx *Scope) OnOpenFiles(fn func(paths []string)) {
	rt := cx.rt
	h := &openFilesHandler{owner: cx.current(), fn: fn}
	rt.opens = append(rt.opens, h)
	h.owner.addCleanup(func() { rt.opens = slices.DeleteFunc(rt.opens, func(o *openFilesHandler) bool { return o == h }) })
	if paths := rt.openPending; len(paths) > 0 {
		rt.openPending = nil
		// Not while the caller builds its view: the handler may well
		// change what it shows.
		h.owner.Post(func() { h.fn(paths) })
	}
}

func (rt *runtime) openFiles(paths []string) {
	if len(paths) == 0 {
		return
	}
	if len(rt.opens) == 0 {
		rt.openPending = append(rt.openPending, paths...)
		return
	}
	for _, h := range slices.Clone(rt.opens) {
		rt.tree.within(h.owner, func() { h.fn(slices.Clone(paths)) })
	}
}

func (t *Tree) surfaceEvent(ev driver.Event) bool {
	rt := t.rt
	switch ev.Kind {
	case driver.EventWindowClose:
		if w := t.window; w != nil && !w.closed {
			w.requestClose()
		}
	case driver.EventWindowHidden:
		if w := t.window; w != nil && !w.closed {
			t.within(w.scope, func() { w.visible.Set(false) })
		}
	case driver.EventReopen:
		t.within(t.root, rt.reopen)
	case driver.EventOpenFiles:
		paths, _ := ev.Data.([]string)
		t.within(t.root, func() { rt.openFiles(paths) })
	case driver.EventTray:
		if tr := rt.trays[ev.Count]; tr != nil {
			t.within(tr.owner, func() { tr.clicked(ev.Index) })
		}
	case driver.EventHotKey:
		rt.hotKey(ev.Count)
	case driver.EventLaunchAtLogin, driver.EventNotification, driver.EventBiometric:
		t.answer(ev)
	case driver.EventNotificationPress:
		t.within(t.root, func() { rt.notificationPressed(ev.Text) })
	default:
		return false
	}
	return true
}

// ID numbers an opened surface's tree, which ItemOpenSurface names; the
// first tree's is 0.
func (t *Tree) ID() uint32 { return t.id }

// Surfaces returns the trees of the windows, panels and tray views the
// app opened, which the engine commits to the platforms that attach them,
// the closed ones until Forget.
func (t *Tree) Surfaces() []*Tree { return t.rt.surfaces }

// Closed reports whether an opened surface closed: the platform destroys
// its window, and the engine forgets it.
func (t *Tree) Closed() bool { return t.closed }

// Forget drops a closed surface's tree.
func (t *Tree) Forget() {
	t.rt.surfaces = slices.DeleteFunc(t.rt.surfaces, func(o *Tree) bool { return o == t })
}
