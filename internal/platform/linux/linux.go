//go:build linux && !android

// Package linux is NSGo's GTK 4 backend, on NSH's Linux design: every
// view is a widget of its own (NsgoView, a GtkWidget subclass) that its
// parent places at the core's frame and that draws itself with GtkSnapshot
// and Cairo, text the engine measured and laid out with Pango included.
// Inputs, web views, videos and the app's native components are native
// GtkEntry, WebKitWebView, GtkPicture and app widgets hosted in their
// view's widget, so they stack, clip, fade and transform with the views
// around them. GTK delivers the pointer, wheel and keyboard events and
// file drops to the surface; the backend hit tests them and the engine
// keeps the interaction state. Transitions, animated values, layout
// animations and animated images run on one frame clock that ticks only
// while something animates.
//
// The backend is the one the roadmap asks for: GTK 4 through purego, no
// cgo and no hand-written C. It loads its libraries at run time, so an
// app built on macOS needs only Go and Xcode, and an app built for Linux
// needs GTK 4.14 or newer, Pango and Cairo where it runs. Web views also
// need WebKitGTK 6.0, which loads on the first one; without it a web view
// shows a placeholder, as NSH's does. Videos play through GTK's
// GStreamer media backend.
//
// The application menu is a GTK menu bar above the surface. The backend
// does not select text, expose accessibility, draw window controls of its
// own or animate modals.
package linux

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/engine"
	"github.com/oneo-me/nsgo/internal/platform/finger"
	"github.com/oneo-me/nsgo/internal/platform/hover"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
	"github.com/oneo-me/nsgo/internal/platform/queue"
	"github.com/oneo-me/nsgo/internal/platform/scrollbar"
	"github.com/oneo-me/nsgo/internal/platform/textselect"
	"github.com/oneo-me/nsgo/internal/testhost"
)

// Options configure the window.
type Options struct {
	Title         string
	Width, Height driver.Points
	// ID names the app's directories and storage (engine.AppDirs).
	ID string
	// Components are the native components Native elements name.
	Components driver.NativeComponents
	// Fonts are font files under the app's resources, registered before
	// the first view.
	Fonts []string
	// Appearance is the appearance the app starts in.
	Appearance driver.Appearance
	// Perf shows the performance monitor over the app (perf.go).
	Perf bool
	// Host is what an embedding host gives the view (NewView).
	Host engine.HostOptions
	// test is what a test host opens the app with.
	test *testOpen
}

// testOpen is what a test host opens the app with: the test's options and
// the environment it pins.
type testOpen struct {
	options testhost.Options
	pin     engine.TestPin
}

// window is one GTK window and the tree it shows. The engine's goroutine
// reaches its main-thread half only through Mount and post.
type window struct {
	win     uintptr // GtkWindow, 0 for an embedded surface
	surface uintptr // the NsgoView holding the root view
	// embedded is set for a host's widget (NewView): the host owns the
	// window, its chrome and the main loop. app and opts start its engine
	// once GTK first allocates it.
	embedded bool
	app      engine.App
	opts     Options
	box      uintptr // the window content: menu bar and surface
	menuBar  uintptr
	loop     uintptr // GMainLoop

	env driver.Environment

	engine  *engine.Engine
	batches queue.Batches
	chrome  chrome

	// The engine goroutine hands these to the main thread.
	mu    sync.Mutex
	tasks []func()

	// The main thread's. views is indexed by NodeID.
	views     []*view
	root      driver.NodeID
	entries   map[uintptr]*view
	natives   map[uintptr]*view // by the objects of hosted widgets
	appID     string
	keys      []driver.KeyBinding
	held      map[uint32]bool // the keys down, by keycode
	animating []*view
	ticking   bool
	tickID    uint32
	// frameWanted is set while the app waits for a display frame.
	frameWanted bool
	animator    *driver.Animator
	// canvasImages are the images canvases draw, and canvasWaiting the
	// canvases to draw again when one loads.
	canvasImages  canvasImages
	canvasWaiting map[string][]*view
	// hiddenCanvases wait out of sight to be drawn (raster.go).
	hiddenCanvases []*view
	// hud is the performance panel, when the app shows one (perf.go).
	hud *hud
	// stickyBound are the scroll views with sticky cells whose insets
	// follow animated values.
	stickyBound map[*view]bool
	epoch       time.Time // the animator's clock starts here
	layouts     []*view   // views a layout animation moves, shows or retires
	playing     []*view   // image views whose animated images play
	fading      []*view   // image views whose sources fade in

	components driver.NativeComponents
	drop       fileDrop

	lastPointer   driver.Point
	down          driver.Event  // the last press: modifiers and click count
	mouse         finger.Mouse  // finger 0 of the views' touches
	paste         *pasting      // the paste whose clipboard is being read
	pointerIn     bool          // the pointer is over the surface, at lastPointer
	hovered       hover.Tracker // the views under it that follow hovers
	shielding     bool          // a press kept from a hosted control goes on
	panning       *pan
	bars          []*view  // the scroll views under the pointer, which show their bars
	barDrag       *barDrag // the bar the pointer drags
	pinching      *twoFinger
	rotating      *twoFinger
	closing       bool
	lastWidth     driver.Points
	lastHeight    driver.Points
	started       bool
	handle        uintptr
	measureMap    [2]uintptr // the engine thread's Pango contexts, left to right and right to left
	hitMap        [2]uintptr // hit testing's, used under hitMu from either thread
	hitMu         sync.Mutex
	extents       map[driver.Font]driver.FontExtent // the engine thread's font metrics
	capHeights    map[driver.Font]driver.Points     // and capital heights
	lineMetrics   map[driver.Font]lineMetrics       // and decoration lines
	pressConsumed bool
	// selecting is the selection of an area, and selected the texts
	// that highlight a part of it (selection.go).
	selecting *textselect.Selection
	selected  []*view
	// forced is the appearance an embedded view's app forces, scoped the
	// one its controls show in, by schemeProvider under the surface's
	// name (scheme.go).
	forced, scoped driver.Appearance
	schemeProvider uintptr
	surfaceName    string
	// touching is the touch that may drag a scroll view (touchscroll.go).
	touching *touchScroll
	// roles are the accessible roles of the views the batch being applied
	// creates (access.go).
	roles        map[driver.NodeID]int32
	longPress    uint64 // counts long presses waited for, to cancel the last
	focus        *view  // the entry with keyboard focus
	pinchTimeout uint32
	alerts       []driver.AlertConfig
	alerting     bool
	alertingID   uint32
	alertShown   driver.AlertConfig // the alert showing, for a test host
	menuCounter  uint32
	actionGroup  uintptr
	actionRefs   map[uintptr]menuRef
	// building names the action group, prefix and refs the menu being
	// built registers its actions in, when not the menu bar's.
	building menuBuild
	// popover is the context menu shown, popoverID its request and
	// popupRefs its items.
	popover   uintptr
	popoverID uint32
	popupRefs map[uintptr]menuRef
	// appearance is the one the app forces; preferDark the session's
	// gtk-application-prefer-dark-theme, read once it is first forced.
	appearance                  driver.Appearance
	preferDark, preferDarkKnown bool
}

// menuBuild is where a menu's actions go.
type menuBuild struct {
	group  uintptr
	prefix string
	refs   map[uintptr]menuRef
}

// Run opens a window showing app and runs GTK's main loop. Call it from
// main; it returns when the window closes.
func Run(app engine.App, opts Options) {
	w, err := start(app, opts)
	if err != nil {
		panic("nsgo: " + err.Error())
	}
	w.run()
	w.stop()
}

// start opens the window and starts the engine, without running the main
// loop yet. Run continues with it; tests drive the window from their own
// GTK thread.
func start(app engine.App, opts Options) (*window, error) {
	if opts.Width == 0 {
		opts.Width, opts.Height = 480, 640
	}
	if err := openLibs(); err != nil {
		return nil, fmt.Errorf("the Linux backend needs GTK 4, Pango and Cairo: %w", err)
	}
	runtime.LockOSThread()
	registerFonts(opts.Fonts)
	preferGLOnNvidia()
	if gtkInitCheck() == 0 {
		return nil, errors.New("no display; run the app in a desktop session or with xvfb-run")
	}
	if err := registerViewType(); err != nil {
		return nil, fmt.Errorf("the Linux backend needs GTK 4: %w", err)
	}
	w := newWindow(opts, false)
	w.createWindow(opts)
	w.env = w.environment()

	w.setAppearance(opts.Appearance)
	w.startEngine(app, opts, driver.Size{Width: opts.Width, Height: opts.Height})
	w.loop = gMainLoopNew(0, 0)
	w.setupSettings()
	return w, nil
}

// newWindow returns the state of a window, or of a host's widget when
// embedded.
func newWindow(opts Options, embedded bool) *window {
	w := &window{
		entries: map[uintptr]*view{},
		natives: map[uintptr]*view{},
		appID:   opts.ID,

		actionRefs: map[uintptr]menuRef{},
		env:        driver.Environment{OS: driver.OSLinux, Idiom: driver.IdiomDesktop},
		components: opts.Components,
		epoch:      time.Now(),
		embedded:   embedded,
	}
	w.animator = driver.NewAnimator(w.animatorReport)
	w.handle = registerWindow(w)
	w.chrome.keys = map[string]menuRef{}
	return w
}

// startEngine starts app's engine at size.
func (w *window) startEngine(app engine.App, opts Options, size driver.Size) {
	eopts := engine.Options{
		Size:        size,
		Environment: w.env,
		Appearance:  opts.Appearance,
		Host:        opts.Host,
	}
	eopts.SetDirs(engine.AppDirs(opts.ID))
	if opts.test != nil {
		eopts.UseTest(opts.test.options, &opts.test.pin)
	}
	imageload.UseDiskCache(eopts.CacheDir)
	if opts.Perf && opts.test == nil {
		eopts.Perf = w.startPerf()
	}
	w.engine = engine.Start(w, app, eopts)
}

// View is an app in a GtkWidget of an embedding host's window, which the
// host's own main loop runs (NewView).
type View struct{ w *window }

// preferGLOnNvidia has GSK draw with its GL renderer instead of its
// Vulkan one (GTK's default since 4.16) under NVIDIA's driver, as NSH
// does, unless GSK_RENDERER chooses: there GSK's Vulkan frames cost the
// main thread several times as much (NSH measured ~9.5 ms of paint per
// frame at 75 fps against ~1 ms at 120 fps with GL). A host embedding a
// view chooses for itself.
func preferGLOnNvidia() {
	if _, set := os.LookupEnv("GSK_RENDERER"); set {
		return
	}
	if _, err := os.Stat("/proc/driver/nvidia/version"); err == nil {
		os.Setenv("GSK_RENDERER", "ngl")
	}
}

// NewView makes a GtkWidget showing app, for a host that has initialized
// GTK and runs its main loop itself. Its engine starts once GTK first
// allocates the widget, so settings made before that are in the app's
// first frame. Call it on GTK's thread; several views may show at once.
func NewView(app engine.App, opts Options) (*View, error) {
	if err := openLibs(); err != nil {
		return nil, fmt.Errorf("the Linux backend needs GTK 4, Pango and Cairo: %w", err)
	}
	runtime.LockOSThread()
	registerFonts(opts.Fonts)
	if gtkInitCheck() == 0 {
		return nil, errors.New("no display")
	}
	if err := registerViewType(); err != nil {
		return nil, fmt.Errorf("the Linux backend needs GTK 4: %w", err)
	}
	if opts.Host.Settings == nil {
		opts.Host.Settings = &engine.HostSettings{}
	}
	w := newWindow(opts, true)
	w.app, w.opts = app, opts
	w.surface = w.newViewWidget(nil)
	gtkWidgetSetHexpand(w.surface, 1)
	gtkWidgetSetVexpand(w.surface, 1)
	// Keys reach the surface while the focus is in it, which a press
	// gives it.
	gtkWidgetSetFocusable(w.surface, 1)
	w.setupInputs()
	w.setupSettings()
	w.forced = opts.Appearance
	w.scopeAppearance(w.shownAppearance())
	return &View{w}, nil
}

// Handle returns the GtkWidget, which the host adds to its widgets.
func (v *View) Handle() uintptr { return v.w.surface }

// SetHostEnvironment sets what the host sets over the session's
// environment: its locale, text size and appearance.
func (v *View) SetHostEnvironment(env engine.HostEnvironment) {
	v.w.opts.Host.Settings.SetEnvironment(env)
	v.w.scopeAppearance(v.w.shownAppearance())
}

// SetTokens sets the values the host themes the app with.
func (v *View) SetTokens(t engine.Tokens) { v.w.opts.Host.Settings.SetTokens(t) }

// Close stops the app, destroys its native components and lets the
// widget go, which the host removes from its parent.
func (v *View) Close() {
	w := v.w
	w.scopeAppearance(driver.AppearanceSystem)
	w.close()
	w.closeNatives()
	gObjectUnref(w.surface)
}

// closeNatives destroys the native components still showing, once the
// app has stopped.
func (w *window) closeNatives() {
	for _, x := range w.views {
		if x != nil && x.native != nil {
			w.closeNative(x)
		}
	}
}

// toplevel is the window the surface is in: its own, or the host's.
func (w *window) toplevel() uintptr {
	if w.win != 0 {
		return w.win
	}
	return gtkWidgetGetRoot(w.surface)
}

// run presents the window and runs GTK's main loop until it closes.
func (w *window) run() {
	gtkWindowPresent(w.win)
	gMainLoopRun(w.loop)
}

// stop ends the engine, destroys the native components still showing
// and frees the loop after run returns.
func (w *window) stop() {
	w.engine.Close()
	w.closeNatives()
	engine.SyncStorage()
	for i, m := range w.measureMap {
		if m != 0 {
			gObjectUnref(m)
			w.measureMap[i] = 0
		}
	}
	w.hitMu.Lock()
	for i, m := range w.hitMap {
		if m != 0 {
			gObjectUnref(m)
			w.hitMap[i] = 0
		}
	}
	w.hitMu.Unlock()
	if w.loop != 0 {
		gMainLoopUnref(w.loop)
		w.loop = 0
	}
	// The handle stays registered: GTK may still call a callback of a
	// window whose loop has stopped as its widgets are finalized.
}

// createWindow makes the window, its menu bar box and the surface.
func (w *window) createWindow(opts Options) {
	w.win = gtkWindowNew()
	title := opts.Title
	gtkWindowSetTitle(w.win, cstr(title))
	gtkWindowSetDefaultSize(w.win, int32(opts.Width), int32(opts.Height))
	w.lastWidth, w.lastHeight = opts.Width, opts.Height
	if opts.test != nil {
		// A fixed size has tiling compositors float a test's window at the
		// test's size instead of resizing it as another opens.
		gtkWindowSetResizable(w.win, 0)
	}

	w.box = gtkBoxNew(1, 0) // vertical
	w.surface = w.newViewWidget(nil)
	gtkWidgetSetHexpand(w.surface, 1)
	gtkWidgetSetVexpand(w.surface, 1)
	gtkBoxAppend(w.box, w.surface)
	gtkWindowSetChild(w.win, w.box)
	w.setupInputs()
	gSignalConnect(w.win, "close-request", cbCloseRequest, w.handle)
	for _, name := range []string{"maximized", "fullscreen"} {
		gSignalConnect(w.win, "notify::"+name, cbWindowState, w.handle)
	}
}

// setupSettings rereads the environment whenever GTK's settings change.
func (w *window) setupSettings() {
	if settings := gtkSettingsGetDefault(); settings != 0 {
		for _, name := range []string{"gtk-theme-name", "gtk-application-prefer-dark-theme", "gtk-xft-dpi", "gtk-enable-animations"} {
			gSignalConnect(settings, "notify::"+name, cbSettingsChanged, w.handle)
		}
	}
}

// Mount implements engine.Platform: it queues the batch and wakes the
// main loop once.
func (w *window) Mount(b *driver.MountBatch) {
	if w.batches.Push(b) {
		gIdleApply(cbApply, w.handle)
	}
}

// RequestFrame implements engine.Platform: the app waits for the next
// display frame. Called from the engine goroutine, so it hops to the main
// loop, whose frame clock runs there.
func (w *window) RequestFrame() {
	w.post(func() {
		w.frameWanted = true
		w.startClock()
	})
}

// post runs fn on the main thread; any goroutine may call it.
func (w *window) post(fn func()) {
	w.mu.Lock()
	wake := len(w.tasks) == 0
	w.tasks = append(w.tasks, fn)
	w.mu.Unlock()
	if wake {
		gIdleApply(cbRun, w.handle)
	}
}

// gIdleApply schedules cb(handle) on the main loop; the callback
// returns G_SOURCE_REMOVE.
func gIdleApply(cb uintptr, handle uintptr) {
	gIdleAdd(cb, handle)
}

func runCallback(data uintptr) int32 {
	if w := windowOf(data); w != nil {
		w.runTasks()
	}
	return 0
}

func applyCallback(data uintptr) int32 {
	w := windowOf(data)
	if w == nil {
		return 0
	}
	done := w.measuring(true)
	for _, b := range w.batches.Drain() {
		if w.hud != nil && !b.Sent.IsZero() {
			w.hud.monitor.RecordLatency(time.Since(b.Sent))
		}
		w.apply(b)
	}
	done()
	return 0
}

func closeRequestCallback(win uintptr, data uintptr) int32 {
	if w := windowOf(data); w != nil {
		w.close()
	}
	return 1 // the window closes; Run returns after the loop ends
}

// runTasks runs what other goroutines posted, in order.
func (w *window) runTasks() {
	w.mu.Lock()
	fns := w.tasks
	w.tasks = nil
	w.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// chromeItem applies an item of the window's own: its title, a window
// command or the menu bar.
func (w *window) chromeItem(it *driver.MountItem) {
	switch it.Kind {
	case driver.ItemTitle:
		gtkWindowSetTitle(w.win, cstr(it.Value))
	case driver.ItemWindow:
		if it.Window != nil {
			w.windowCommand(it.Window)
		}
	case driver.ItemMenu:
		w.setMenu(it.Menu)
	}
}

// close ends the app: the engine stops, and the main loop returns.
func (w *window) close() {
	if w.closing {
		return
	}
	w.closing = true
	if w.tickID != 0 {
		gSourceRemove(w.tickID)
		w.tickID = 0
	}
	if w.engine != nil {
		w.engine.Close()
	}
	w.stopPerf()
	if w.loop != 0 {
		gMainLoopQuit(w.loop)
	}
}

func (w *window) view(id driver.NodeID) *view {
	if id > 0 && int(id) < len(w.views) {
		return w.views[id]
	}
	return nil
}

// apply applies one commit's items.
func (w *window) apply(b *driver.MountBatch) {
	var rescroll, loads []*view
	type focusChange struct {
		v     *view
		focus bool
	}
	var focus []focusChange
	lb := w.newLayoutBatch(b)
	w.prepareRoles(b)
	for i := range b.Items {
		it := &b.Items[i]
		switch it.Kind {
		case driver.ItemCreate:
			w.create(it.ID, it.View)
			if lb != nil {
				lb.created[it.ID] = true
			}
			continue
		case driver.ItemAnimation:
			w.animations(it.Animation)
			continue
		case driver.ItemAlert:
			w.showAlert(it.Alert)
			continue
		case driver.ItemDismissKeyboard:
			focus = append(focus, focusChange{})
			continue
		case driver.ItemSetClipboard:
			w.setClipboard(it.Value)
			continue
		case driver.ItemReadClipboard:
			w.readClipboard(it)
			continue
		case driver.ItemSetClipboardImage:
			if data, ok := it.Data.([]byte); ok {
				setClipboardImage(data)
			}
			continue
		case driver.ItemOpenURL:
			w.openURL(it.Value, it.Count)
			continue
		case driver.ItemPickImages:
			w.pickImages(*it.Pick, it.Count)
			continue
		case driver.ItemPopupMenu:
			w.popupMenu(it.Menu, it.Point, it.Count)
			continue
		case driver.ItemAppearance:
			// The settings are the process's, which a host's widget
			// leaves to the host: its controls follow in its scope.
			if w.embedded {
				w.forced = it.Appearance
				w.scopeAppearance(w.shownAppearance())
			} else {
				w.setAppearance(it.Appearance)
			}
			continue
		case driver.ItemQuit:
			if !w.embedded {
				w.close()
			}
			continue
		case driver.ItemImageInfo:
			w.imageInfo(it)
			continue
		case driver.ItemKeymap:
			w.setKeymap(it.Keys)
			continue
		case driver.ItemTitle, driver.ItemWindow, driver.ItemMenu:
			// An embedding host owns its window and menus.
			if !w.embedded {
				w.chromeItem(it)
			}
			continue
		case driver.ItemAnnounce:
			w.announce(it.Value, false)
			continue
		case driver.ItemAccessibilityLabel:
			if v := w.view(it.ID); v != nil {
				v.props.Label = it.Value
				w.accessChanged(v)
			}
			continue
		case driver.ItemAccessibilityFocus:
			if v := w.view(it.ID); v != nil && v.widget != 0 {
				gtkWidgetGrabFocus(v.widget)
			}
			continue
		case driver.ItemStatusBar, driver.ItemBackUnhandled, driver.ItemBackPreview, driver.ItemHistory:
			// No status bar; Escape is back.
			continue
		}
		v := w.view(it.ID)
		if v == nil {
			continue
		}
		switch it.Kind {
		case driver.ItemInsert:
			if p := w.view(it.Parent); p != nil {
				v.detach()
				at := len(p.children)
				before := w.view(it.Before)
				if before != nil {
					if j := slices.Index(p.children, before); j >= 0 {
						at = j
					} else {
						before = nil
					}
				}
				p.children = slices.Insert(p.children, at, v)
				v.parent = p
				w.insertWidget(p, v, before)
				w.inserted(v, p, lb)
				if v.kind == driver.KindModal && !v.presented {
					v.presented = true
					w.present(v)
				}
			}
		case driver.ItemRemove:
			v.detach()
			w.unparentWidget(v.widget)
		case driver.ItemDelete:
			w.animator.Forget(v.id)
			switch {
			case v.parent != nil && v.parent.dying:
				w.retireInner(v)
			case w.retiring(v, lb):
				w.retire(v, lb)
			default:
				w.delete(v)
			}
		case driver.ItemOverflow:
			v.overflow = it.Frame
		case driver.ItemFrame:
			old := v.shownFrame().Size
			w.framed(v, it.Frame, lb)
			resized := v.shownFrame().Size != old
			w.moved(v)
			if resized && v.widget != 0 {
				gtkWidgetQueueAllocate(v.widget)
			}
			if v.kind == driver.KindScroll {
				rescroll = append(rescroll, v)
			}
		case driver.ItemViewStyle:
			hidden := v.style.Hidden
			w.setStyle(v, it.ViewStyle)
			if v.style.Hidden != hidden {
				w.accessChanged(v)
			}
			w.showWidget(v)
			w.moved(v)
			if v.entry != 0 {
				w.styleEntry(v)
			}
			w.webBackground(v)
			if v.kind == driver.KindImage {
				w.playImage(v) // again once it is no longer tinted
			}
		case driver.ItemText:
			if v.text != nil {
				v.text.Release()
			}
			v.text = it.Text.Layout
			v.content = it.Text.Content
			v.plain = it.Text.Text
			w.accessChanged(v)
			w.changed(v)
		case driver.ItemInput:
			w.configure(v, it.Input)
		case driver.ItemInputValue:
			w.setValue(v, it.Value, it.Count)
		case driver.ItemSelection:
			w.selectRange(v, it.Selection)
		case driver.ItemEvents:
			w.setEvents(v, it.Events)
			w.accessChanged(v)
		case driver.ItemViewProps:
			if it.ViewProps != nil {
				v.props = *it.ViewProps
				if v.props.Disabled && w.drop.over == v {
					w.setDropOver(nil)
				}
				w.accessChanged(v)
			}
		case driver.ItemScrollConfig:
			to := scrollbar.KeepStart(v.scroll.Behavior.ContentInset, it.Scroll.Behavior.ContentInset, v.offset)
			v.scroll = *it.Scroll
			if to != v.offset {
				w.scrollTo(v, to)
			}
			w.refreshChanged(v)
			rescroll = append(rescroll, v) // insets change the offsets
		case driver.ItemScrollContent:
			v.size = it.Size
			w.changed(v)
			rescroll = append(rescroll, v)
		case driver.ItemFocus, driver.ItemBlur:
			focus = append(focus, focusChange{v, it.Kind == driver.ItemFocus})
		case driver.ItemImage:
			if w.setImage(v, it.Image) {
				loads = append(loads, v)
			}
			w.changed(v)
		case driver.ItemModal:
			// A modal is a view: it shows without a separate window, and
			// comes in as it is first inserted (present).
			v.modal = it.Modal.Animation
		case driver.ItemCanvas:
			w.setCanvas(v, it.Drawing)
			w.setCanvasParts(v, it.Drawing.Nodes)
		case driver.ItemShader:
			if it.Shader != nil {
				w.setShader(v, it.Shader)
			}
		case driver.ItemWeb:
			w.configureWeb(v, it.Web)
		case driver.ItemWebCommand:
			w.webCommand(v, it.WebCommand, it.Value)
		case driver.ItemVideo:
			w.configureVideo(v, it.Video)
		case driver.ItemVideoSeek:
			if v.video != nil {
				v.video.seek(it.Time)
			}
		case driver.ItemNative:
			w.configureNative(v, it.Native)
		case driver.ItemNativeCommand:
			if v.native != nil && v.native.comp != nil {
				v.native.comp.Command(it.Value, it.Data)
			}
		case driver.ItemScrollBy:
			v.applied++
			w.scrollBy(v, it.Point)
		case driver.ItemStickyCells:
			w.setSticky(v, it.Sticky)
		case driver.ItemScrollTo:
			v.applied++
			w.interruptScroll(v)
			if it.Animated {
				w.scrollAnimated(v, it.Point)
			} else {
				w.scrollTo(v, it.Point)
			}
		}
	}
	w.setRoot(b.Root)
	// Bindings change with the batch, and its views exist now.
	w.showAnimated()
	w.updateDropTarget()
	for _, v := range rescroll {
		if !v.deleted {
			w.scrollTo(v, v.offset)
		}
	}
	for _, v := range loads {
		if !v.deleted {
			w.load(v, &v.placeholder, false)
			w.load(v, &v.source, true)
		}
	}
	for _, f := range focus {
		switch {
		case f.v == nil:
			if w.focusedEntry() != nil {
				w.focusEntry(nil)
			}
		case f.v.deleted || f.v.entry == 0:
		case f.focus:
			w.focusEntry(f.v)
		default:
			if w.focusedEntry() == f.v {
				w.focusEntry(nil)
			}
		}
	}
	// Views that moved, appeared or went under a still pointer.
	w.hoverViews()
	w.refreshSelection()
}

func (w *window) create(id driver.NodeID, kind driver.ViewKind) {
	for len(w.views) <= int(id) {
		w.views = append(w.views, nil)
	}
	v := &view{id: id, kind: kind, style: driver.ViewStyle{Opacity: 1}}
	v.widget = w.newViewWidget(v)
	w.views[id] = v
}

// setRoot makes the view id the surface's child.
func (w *window) setRoot(id driver.NodeID) {
	if old := w.view(w.root); old != nil && w.root != id && old.widget != 0 && gtkWidgetGetParent(old.widget) == w.surface {
		w.unparentWidget(old.widget)
	}
	w.root = id
	if root := w.view(id); root != nil && root.widget != 0 && gtkWidgetGetParent(root.widget) != w.surface {
		if gtkWidgetGetParent(root.widget) != 0 {
			gtkWidgetUnparent(root.widget)
		}
		gtkWidgetSetParent(root.widget, w.surface)
		gtkWidgetQueueAllocate(w.surface)
	}
	gtkWidgetQueueDraw(w.surface)
}

// delete drops v and its subtree. The engine deletes the children too,
// so they are detached but their own items release what they hold.
func (w *window) delete(v *view) {
	w.forgetDestination(v)
	w.hovered.Forget(v.id)
	v.detach()
	for _, c := range v.children {
		c.parent = nil
	}
	v.children = nil
	v.releaseCanvasParts()
	if v.text != nil {
		v.text.Release()
		v.text = nil
	}
	w.releaseEditor(v)
	if v.web != nil {
		w.closeWeb(v)
	}
	if v.video != nil {
		w.closeVideo(v)
	}
	if v.native != nil {
		w.closeNative(v)
	}
	if v.shader != nil {
		w.unhost(v, &v.shader.hosted)
		v.shader = nil
	}
	v.source.reset(driver.ImageSource{})
	v.placeholder.reset(driver.ImageSource{})
	w.dropWidget(v)
	v.canvas.raster.release()
	v.canvas.raster = nil
	v.deleted, v.dying, v.layout, v.animated = true, false, nil, nil
	if w.views[v.id] == v {
		w.views[v.id] = nil
	}
}

// windowCommand carries out a Window item.
func (w *window) windowCommand(c *driver.WindowCommand) {
	switch c.Action {
	case driver.WindowSetTitleBar:
		w.chrome.titleBar = c.TitleBar
		w.chrome.height = c.Height
		gtkWindowSetDecorated(w.win, boolInt(c.TitleBar == driver.TitleBarDefault))
		w.showMenuBar()
		w.updateEnvironment()
	case driver.WindowMinimize:
		gtkWindowMinimize(w.win)
	case driver.WindowToggleMaximize:
		if gtkWindowIsMaximized(w.win) != 0 {
			gtkWindowUnmaximize(w.win)
		} else {
			gtkWindowMaximize(w.win)
		}
	case driver.WindowToggleFullScreen:
		if w.chrome.fullScreen {
			gtkWindowUnfullscreen(w.win)
			w.chrome.fullScreen = false
		} else {
			gtkWindowFullscreen(w.win)
			w.chrome.fullScreen = true
		}
		w.showMenuBar()
		w.updateEnvironment()
	case driver.WindowClose:
		w.close()
	case driver.WindowSetFrame:
		w.setFrame(c.Frame)
	case driver.WindowOrderFront:
		gtkWindowPresent(w.win)
	case driver.WindowSetMenuBar:
		w.chrome.menuBar, w.chrome.revealed = c.MenuBar, false
		w.showMenuBar()
	case driver.WindowSetLevel:
		// Stacking is the compositor's.
	}
}

func boolInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// scrollTo moves v's content to offset, clamped to its content, and
// reports the move.
func (w *window) scrollTo(v *view, offset driver.Point) bool {
	offset = scrollbar.Clamp(&v.scroll, v.frame.Size, v.size, offset)
	if offset == v.offset {
		return false
	}
	w.moveOffset(v, offset)
	return true
}

// moveOffset puts v's content at offset, unclamped, and reports it.
func (w *window) moveOffset(v *view, offset driver.Point) {
	v.offset = offset
	if v.widget != 0 {
		gtkWidgetQueueAllocate(v.widget)
		gtkWidgetQueueDraw(v.widget)
	}
	if w.animator.Scroll(v.id, offset) {
		w.showAnimated()
	}
	if w.animator.Pulls(v.id) && w.animator.Pull(v.id, scrollbar.Pulled(&v.scroll, offset)) {
		w.showAnimated()
	}
	w.pinSticky(v)
	// The engine sees the offset within the content and its insets, as a
	// touch pulls it past.
	w.engine.Dispatch(driver.Event{Kind: driver.EventScroll, ID: v.id, Point: scrollbar.Clamp(&v.scroll, v.frame.Size, v.size, offset), Count: v.applied})
}

// wheel scrolls the innermost scroll view under at that can move in the
// wheel's direction. dx and dy are GTK's scroll deltas in points.
func (w *window) wheel(at driver.Point, dx, dy float64) {
	root := w.view(w.root)
	if root == nil {
		return
	}
	under := scrollsAt(root, at, nil)
	for i := len(under) - 1; i >= 0; i-- {
		v := under[i]
		if v.scroll.Behavior.NoScroll {
			continue
		}
		offset := v.offset
		if v.scroll.Horizontal {
			if dx == 0 {
				continue
			}
			offset.X += driver.Points(dx)
		} else {
			if dy == 0 {
				continue
			}
			offset.Y += driver.Points(dy)
		}
		from := v.offset
		w.interruptScroll(v) // the wheel takes over
		if w.scrollTo(v, offset) {
			w.wheelTurned(v, v.offset.X-from.X+v.offset.Y-from.Y)
			return
		}
	}
}

// background is the color behind v: its own or its nearest ancestor's.
func background(v *view) driver.Color {
	for ; v != nil; v = v.parent {
		if v.style.Background&0xff != 0 {
			return v.style.Background
		}
	}
	return driver.RGB(0xffffff)
}

// frameAt returns v's origin in the window's points.
func (w *window) frameAt(v *view) driver.Point {
	var x, y driver.Points
	for p := v; p != nil; p = p.parent {
		x += p.frame.Origin.X - p.offset.X
		y += p.frame.Origin.Y - p.offset.Y
	}
	return driver.Point{X: x, Y: y}
}

// surfaceBounds returns v's frame in the surface and whether v is in the
// root's tree.
func (w *window) surfaceBounds(v *view) (driver.Bounds, bool) {
	b, top := v.frame, v
	for p := v.parent; p != nil; p = p.parent {
		b.Origin.X += p.frame.Origin.X - p.offset.X
		b.Origin.Y += p.frame.Origin.Y - p.offset.Y
		top = p
	}
	return b, top == w.view(w.root)
}

// back reports Escape as the back event when nothing focused took it.
func (w *window) back() {
	w.engine.Dispatch(driver.Event{Kind: driver.EventBack})
}

// setKeymap replaces the app's keyboard shortcuts.
func (w *window) setKeymap(keys []driver.KeyBinding) { w.keys = keys }
