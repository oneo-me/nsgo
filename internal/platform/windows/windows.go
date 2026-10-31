//go:build windows

// Package windows is NSGo's Win32 backend: one window that draws a
// retained tree of views with Direct2D into a flip-model swap chain on a
// Direct3D 11 device, text the engine laid out with DirectWrite, presses
// hit tested by the engine, inputs as editors of the backend's own drawn
// with the views (input.go), web views as WebView2 composition controllers
// in the window's DirectComposition tree, videos played by Media
// Foundation's media engine, images decoded by Go, native components'
// child windows following their views (component.go), files dropped
// through OLE (drop.go), and alerts as task dialogs. A window with a
// material behind its content draws through DirectComposition too, over
// the system backdrop DWM draws (material.go). It uses only the syscall
// package, so it builds without cgo.
//
// Transforms are Direct2D world transforms, translucent views draw in
// layers, and shapes and text are antialiased by Direct2D. Style
// transitions, animated values, layout animations and animated images
// tick on one frame clock, a timer that runs only while something
// animates (animate.go). Image blur and modal animations are not drawn,
// and there is no accessibility yet, which needs UI Automation
// providers.
package windows

import (
	"errors"
	"fmt"
	"log"
	"math"
	"runtime"
	"slices"
	"sync"
	"syscall"
	"time"
	"unsafe"

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

func init() {
	// The window belongs to the thread that creates it: main's.
	runtime.LockOSThread()
}

// Options configure the window.
type Options struct {
	Title         string
	Width, Height driver.Points
	// ID names the app's directories and storage (engine.AppDirs).
	ID string
	// Components are the native components Native elements name.
	Components driver.NativeComponents
	// Material is the material behind the window's content, where the app
	// draws nothing opaque (nsgo.WindowMaterial). It needs Windows 11
	// 22H2 or later, where DWM draws system backdrops; earlier Windows
	// keeps the window opaque.
	Material driver.WindowMaterial
	// Fonts are font files under the app's resources, registered before
	// the first view.
	Fonts []string
	// Appearance is the appearance the app starts in.
	Appearance driver.Appearance
	// Perf shows the performance monitor over the app (perf.go).
	Perf bool
	// Host is what an embedding host gives the view (NewView), and Parent
	// the HWND of the window it is a child window of.
	Host   engine.HostOptions
	Parent uintptr
	// test is what a test host opens the app with.
	test *testOpen
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	registerClassExW  = user32.NewProc("RegisterClassExW")
	createWindowExW   = user32.NewProc("CreateWindowExW")
	defWindowProcW    = user32.NewProc("DefWindowProcW")
	getMessageW       = user32.NewProc("GetMessageW")
	translateMessage  = user32.NewProc("TranslateMessage")
	dispatchMessageW  = user32.NewProc("DispatchMessageW")
	postMessageW      = user32.NewProc("PostMessageW")
	postQuitMessage   = user32.NewProc("PostQuitMessage")
	showWindow        = user32.NewProc("ShowWindow")
	setFocus          = user32.NewProc("SetFocus")
	getFocus          = user32.NewProc("GetFocus")
	getAncestor       = user32.NewProc("GetAncestor")
	destroyWindow     = user32.NewProc("DestroyWindow")
	beginPaint        = user32.NewProc("BeginPaint")
	endPaint          = user32.NewProc("EndPaint")
	invalidateRect    = user32.NewProc("InvalidateRect")
	getClientRect     = user32.NewProc("GetClientRect")
	adjustWindowRect  = user32.NewProc("AdjustWindowRect")
	screenToClient    = user32.NewProc("ScreenToClient")
	loadCursorW       = user32.NewProc("LoadCursorW")
	loadIconW         = user32.NewProc("LoadIconW")
	setWindowTextW    = user32.NewProc("SetWindowTextW")
	getWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
	setWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	getDpiForSystem   = user32.NewProc("GetDpiForSystem")
	setDpiAwareness   = user32.NewProc("SetProcessDpiAwarenessContext")

	getModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmDestroy       = 0x0002
	wmMove          = 0x0003
	wmSize          = 0x0005
	wmPaint         = 0x000F
	wmEraseBkgnd    = 0x0014
	wmSettingChange = 0x001A
	wmKeyDown       = 0x0100
	wmKeyUp         = 0x0101
	wmChar          = 0x0102
	wmSysKeyDown    = 0x0104
	wmCommand       = 0x0111
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmRButtonDown   = 0x0204
	wmMouseWheel    = 0x020A
	wmMouseHWheel   = 0x020E
	wmAppCommand    = 0x0319     // what DefWindowProc makes of XBUTTON1 and back keys
	wmApply         = 0x8000 + 1 // WM_APP + 1: batches are queued
	wmRun           = 0x8000 + 2 // functions are queued
	wmAlert         = 0x8000 + 3 // alerts are queued
	wmFrame         = 0x8000 + 4 // the app waits for a display frame

	wsOverlappedWindow = 0x00CF0000
	wsClipChildren     = 0x02000000
	wsVisible          = 0x10000000
	wsExNoRedirect     = 0x00200000           // no redirection bitmap, for a composition surface
	cwUseDefault       = ^uintptr(0x7FFFFFFF) // CW_USEDEFAULT as a uintptr
	gwlStyle           = ^uintptr(15)         // -16
	swShow             = 5
	sizeMinimized      = 1
	vkReturn           = 0x0D
	vkEscape           = 0x1B
	vkLeft             = 0x25
	appCommandBack     = 1 // APPCOMMAND_BROWSER_BACKWARD

	defaultFontName = "Segoe UI"
)

// wheelStep is how far one notch of a mouse wheel scrolls.
const wheelStep driver.Points = 48

type rect struct{ Left, Top, Right, Bottom int32 }

func (r rect) empty() bool { return r.Right <= r.Left || r.Bottom <= r.Top }

type point struct{ X, Y int32 }

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	_       uint32
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type paintStruct struct {
	Hdc       uintptr
	Erase     int32
	Paint     rect
	Restore   int32
	IncUpdate int32
	_         [32]byte
}

// view is the main thread's copy of one mounted view.
type view struct {
	id       driver.NodeID
	kind     driver.ViewKind
	parent   *view
	children []*view
	frame    driver.Bounds
	style    driver.ViewStyle
	events   driver.EventMask
	props    driver.ViewProps
	overflow driver.Bounds // where its children reach outside it (ItemOverflow)
	deleted  bool

	// A text view's layout and where it goes in the view, its content
	// and the UTF-16 range of it a selection highlights.
	text             drawnText
	content          driver.Bounds
	plain            string
	selStart, selEnd int

	// The shadow drawn under the background, and the box shadows, by
	// what each depends on; shadowsUsed are those the last pass drew.
	shadow      *shadow
	boxShadows  map[boxShadowKey]*boxShadow
	shadowsUsed map[boxShadowKey]bool
	// keyframeStart is when the keyframe animation of style began.
	keyframeStart time.Time

	// A scroll view's configuration, content size and offset.
	scroll driver.ScrollConfig
	size   driver.Size
	offset driver.Point
	// applied counts the core's ItemScrollTo and ItemScrollBy items,
	// reported with each offset; sticky are a scroll view's sticky cells,
	// and laid and shift a sticky cell's frame from the core and how far
	// it is moved from it.
	applied uint32
	sticky  []driver.StickyCell
	laid    driver.Bounds
	shift   driver.Point
	wheel   scrollbar.Wheel // a snapping view's wheel turns
	// glide is how a scroll view a finger let go comes to rest,
	// refreshing whether it keeps the refresh room and spinning whether
	// its spinner turns (touchscroll.go).
	glide                *scrollbar.Motion
	refreshing, spinning bool
	// The bar: when it last showed, its opacity, and whether its fade
	// runs on the frame clock.
	barShown   time.Time
	barAlpha   float32
	barTicking bool

	// An input's editor.
	field *textInput

	// An image view's configuration, source and placeholder.
	image               driver.ImageConfig
	source, placeholder imageSlot

	// A canvas's drawing, and the command lists it plays into.
	drawing     *driver.Drawing
	canvasCache canvasCache
	// shaderView is a shader view's Direct3D state.
	shaderView *shaderView
	// glass is the textures its Liquid Glass draws through.
	glass *glassView

	// A web view's WebView2 controller and portal, a video's media engine.
	web   *webView
	video *video

	// A native view's component.
	component *component

	// What animates: the style, after the first, and the scroll offset.
	styled     bool
	transition *transition
	scrolling  *scrolling
	// animated are the values bound to the view, Set 0 when none; drawn is
	// its style with them, as styleOf last made it.
	animated driver.AnimatedValues
	drawn    driver.ViewStyle
	// What a LayoutAnimation animates: the frame drawn, moving to frame,
	// and the view appearing or going.
	moving *moving
	fading *fading
	// leaving are the deleted children a LayoutAnimation keeps drawing
	// while they go, by where they were; leftFrom is the view a leaving
	// view is drawn in. A retired view is one of those or in one: its ID
	// is free and it only draws.
	leaving  []leaving
	leftFrom *view
	retired  bool

	// area and tree are where the last frame painted the view and its
	// subtree, in the window's points; queued says the next frame draws
	// them again, below that it measures a view under it (damage.go).
	area, tree    rectF
	queued, below bool
}

// detach takes v out of its parent's children.
func (v *view) detach() {
	if p := v.parent; p != nil {
		if i := slices.Index(p.children, v); i >= 0 {
			p.children = slices.Delete(p.children, i, i+1)
		}
		v.parent = nil
	}
}

type window struct {
	hwnd  uintptr
	title string
	// damage is what the next frame draws again (damage.go).
	damage damage
	// hud is the performance panel, when the app shows one (perf.go).
	hud *hud
	// embedded is set for a host's child window (NewView): the host owns
	// the top-level window, its chrome and the message loop. app and
	// opts start its engine once it first has a size; skipChar drops the
	// character of a key a shortcut took, which the host's loop
	// translated anyway.
	embedded bool
	app      engine.App
	opts     Options
	skipChar bool
	engine   *engine.Engine
	batches  queue.Batches
	scale    float64 // pixels per point

	// What other goroutines hand the main thread.
	mu    sync.Mutex
	tasks []func()

	// The main thread's.
	views    []*view // index is NodeID
	root     driver.NodeID
	env      driver.Environment // the last reported
	alerts   []driver.AlertConfig
	alerting bool
	// alertShown is the alert showing, for a test host to answer; alert
	// is how it is drawn, and alertRestore the field it took the focus
	// from (alert.go).
	alertShown   driver.AlertConfig
	alert        *shownAlert
	alertRestore *textInput
	// The photo picks and context menus waiting for a message of their
	// own, and whether one shows.
	picks         []pickRequest
	picking       bool
	popups        []popupRequest
	popupTracking bool
	// popupChoice is the item a test chose in the menu tracking, which
	// ends its tracking (testhost.go).
	popupChoice *uint32
	// popupShown are the items of the context menu tracking.
	popupShown []driver.MenuEntry
	// appearance is the one the app forces, or the system's; host the one
	// an embedding host sets in place of the system's.
	appearance, host driver.Appearance
	keys             []driver.KeyBinding // the keyboard shortcuts
	animating        []*view
	ticking          bool
	clock            *frameClock // made as it first ticks
	// frameWanted is set while the app waits for a display frame.
	frameWanted bool
	chrome      chrome
	webs        map[*webView]bool
	cacheDir    string                 // where web views keep their profile
	frames      []func(time.Time) bool // frame clients (onFrame)
	// animator runs the animated values on the frame clock, whose times
	// count from epoch.
	animator *driver.Animator
	epoch    time.Time
	// canvasImages are the images canvases draw, by ImageSource.Key.
	canvasImages map[string]*imageSlot
	// stickyBound are the scroll views with sticky cells whose insets
	// follow animated values.
	stickyBound map[*view]bool
	// components are the app's native components, hosts the views showing
	// them with a window of their own, and missing the unknown names
	// already logged.
	components driver.NativeComponents
	hosts      map[*component]bool
	missing    map[string]bool
	// drop is the window's file drop target while views take dropped
	// files: drops holds those views.
	drop  *dropTarget
	drops map[*view]bool

	// composition is the window's while it has web views or a material
	// behind its content, and composed the web views the last frame drew.
	composition *composition
	composed    []*webView
	// baseComposed draws the window's content through DirectComposition,
	// with its alpha, for a material behind it; backdrop is the DWM
	// system backdrop, backdropNone when the material could not be set.
	baseComposed bool
	backdrop     int32
	// material is the material DWM draws, WindowMaterialNone when the
	// window is opaque.
	material driver.WindowMaterial
	// The page holding the mouse while a button is held on it, the one
	// under it, those holding touch and pen contacts, and the video whose
	// control bar holds the mouse.
	rawWeb, hoverWeb *webView
	touchWebs        map[uint32]*webView
	barVideo         *video

	// The gestures in progress.
	pan                            *pan
	pinch, rotation, wheelPinching *twoFinger

	// The scroll views under the pointer, which show their bars; the bar
	// it drags; and whether it asked to hear when it leaves the window.
	bars     []*view
	barDrag  *barDrag
	tracking bool

	// The views under the pointer that follow hovers, and where the
	// pointer is while it is over the client area.
	hovered   hover.Tracker
	pointer   driver.Point
	pointerIn bool

	// The field with the keyboard focus, the one a modal took it from,
	// the one a press is selecting in, and whether the window lacks the
	// keyboard. blinking is set while the caret's timer runs; surrogate
	// holds the first half of a character typed in two WM_CHARs.
	focus, restore, selecting *textInput
	// textSelection is the selection of an area, and selected the texts
	// that highlight a part of it (selection.go).
	textSelection *textselect.Selection
	selected      []*view
	// touching is the touch that may drag a scroll view (touchscroll.go).
	touching *touchScroll
	// uia are the UI Automation elements made for the views (uia.go).
	uia                map[uint64]*uiaElement
	inactive, blinking bool
	surrogate          uint16
	// The last press, for counting double and triple clicks.
	clicks struct {
		time  uint32
		at    point
		count int
	}
	down driver.Event // the left button's last press: modifiers and count
	// mouse is finger 0 of the views' touches.
	mouse finger.Mouse
	// pressing is the left button's press on a view, as it happens.
	pressing *pressing

	swap *swapChain
}

// Window procedures are process-wide callbacks; they find their window
// here. Only the main thread uses these maps.
var windows = map[uintptr]*window{}

var wndProcCallback = syscall.NewCallback(wndProc)

func wide(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

// Run opens a window showing app and runs the message loop. Call it from
// main; it does not return until the window closes.
func Run(app engine.App, opts Options) {
	if opts.Width == 0 {
		opts.Width, opts.Height = 480, 640
	}
	const perMonitorAwareV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	setDpiAwareness.Call(perMonitorAwareV2)
	registerFonts(opts.Fonts)
	w := newWindow(opts)
	if opts.Material != driver.WindowMaterialNone {
		switch {
		case !systemBackdrops():
			log.Printf("nsgo: window materials need Windows 11 22H2 or later: the window stays opaque")
		case !compositionAvailable():
			log.Printf("nsgo: window materials need DirectComposition: the window stays opaque")
		default:
			w.baseComposed, w.backdrop = true, backdropFor(opts.Material)
		}
	}
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		w.scale = float64(dpi) / 96
	}

	instance, className := registerClass()

	// Size the window so its client area is the app's size.
	const style = wsOverlappedWindow | wsClipChildren
	var ex uintptr
	if w.baseComposed {
		ex = wsExNoRedirect
	}
	frame := rect{Right: w.px(opts.Width), Bottom: w.px(opts.Height)}
	adjustWindowRect.Call(uintptr(unsafe.Pointer(&frame)), style, 0)
	hwnd, _, _ := createWindowExW.Call(ex, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(wide(opts.Title))), style,
		cwUseDefault, cwUseDefault, uintptr(frame.Right-frame.Left), uintptr(frame.Bottom-frame.Top),
		0, 0, instance, 0)
	w.hwnd = hwnd
	windows[hwnd] = w
	// Nothing has the focus yet: typing must not open a composition.
	w.updateIME()
	w.configureGestures()
	if w.baseComposed {
		if w.applyMaterial() {
			w.material = opts.Material
		} else {
			w.backdrop = backdropNone
		}
	}
	w.env = w.environment()
	w.appearance = opts.Appearance
	w.darkTitleBar(w.shownDark())
	darkMenus(w.appearance)
	// The engine starts once the window exists, so Mount can post to it.
	w.startEngine(app, opts, driver.Size{Width: opts.Width, Height: opts.Height})
	showWindow.Call(hwnd, swShow)
	if opts.test != nil {
		opts.test.ready <- w
	}

	var m msg
	for {
		r, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			w.engine.Close()
			engine.SyncStorage()
			return
		}
		// Shortcuts take key presses before they become characters.
		if (m.Message == wmKeyDown || m.Message == wmSysKeyDown) && m.Hwnd == hwnd && w.shortcut(m.WParam, m.LParam) {
			continue
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// newWindow returns the state of a window showing an app.
func newWindow(opts Options) *window {
	w := &window{title: opts.Title, scale: 1, webs: map[*webView]bool{}, touchWebs: map[uint32]*webView{},
		epoch: time.Now(), components: opts.Components, hosts: map[*component]bool{}, missing: map[string]bool{},
		drops: map[*view]bool{}}
	w.animator = driver.NewAnimator(func(ev driver.Event) { w.engine.Dispatch(ev) })
	return w
}

// registerClass registers NSGo's window class, once.
func registerClass() (instance uintptr, className *uint16) {
	instance, _, _ = getModuleHandleW.Call(0)
	className = wide("NsgoWindow")
	if classRegistered {
		return instance, className
	}
	classRegistered = true
	cursor, _, _ := loadCursorW.Call(0, idcArrow)
	// nsgo links the app's icon as group 1; without one the system's
	// default stands. Windows picks the small icon from the same group.
	const appIcon = 1
	icon, _, _ := loadIconW.Call(instance, appIcon)
	wc := wndClassEx{
		WndProc:   wndProcCallback,
		Instance:  instance,
		Icon:      icon,
		Cursor:    cursor,
		ClassName: className,
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	return instance, className
}

var classRegistered bool

// startEngine starts app's engine at size.
func (w *window) startEngine(app engine.App, opts Options, size driver.Size) {
	eopts := engine.Options{
		Size:        size,
		Environment: w.env,
		Appearance:  opts.Appearance,
		Host:        opts.Host,
	}
	data, cache := engine.AppDirs(opts.ID)
	eopts.SetDirs(data, cache)
	if opts.test != nil {
		eopts.UseTest(opts.test.options, &opts.test.pin)
		cache = eopts.CacheDir
	}
	imageload.UseDiskCache(cache)
	w.cacheDir = cache
	if opts.Perf && opts.test == nil {
		eopts.Perf = w.startPerf()
	}
	w.engine = engine.Start(w, app, eopts)
}

// View is an app in a child window of an embedding host's window, which
// the host's own message loop runs (NewView).
type View struct{ w *window }

// NewView makes a child window of opts.Parent showing app, for a host
// that runs the message loop itself on this thread, with COM initialized
// (OleInitialize, for the clipboard and drops). The host places it
// (MoveWindow); its engine starts once it first has a size, so settings
// made before that are in the app's first frame. Several views may show
// at once.
func NewView(app engine.App, opts Options) (*View, error) {
	if opts.Parent == 0 {
		return nil, errors.New("a view on Windows is a child window: give the HWND of its parent")
	}
	registerFonts(opts.Fonts)
	if opts.Host.Settings == nil {
		opts.Host.Settings = &engine.HostSettings{}
	}
	w := newWindow(opts)
	w.embedded, w.app, w.opts = true, app, opts
	if dpi, _, _ := getDpiForWindow.Call(opts.Parent); dpi != 0 {
		w.scale = float64(dpi) / 96
	}
	instance, className := registerClass()
	const style = wsChild | wsVisible | wsClipChildren | wsClipSiblings
	hwnd, _, err := createWindowExW.Call(0, uintptr(unsafe.Pointer(className)), 0, style, 0, 0, 0, 0,
		opts.Parent, 0, instance, 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("no child window: %v", err)
	}
	w.hwnd = hwnd
	windows[hwnd] = w
	w.updateIME()
	w.configureGestures()
	w.env = w.environment()
	w.appearance = opts.Appearance
	return &View{w}, nil
}

// Handle returns the child window's HWND, which the host places.
func (v *View) Handle() uintptr { return v.w.hwnd }

// SetHostEnvironment sets what the host sets over the system's
// environment: its locale, text size and appearance, which the fields
// NSGo draws take too.
func (v *View) SetHostEnvironment(env engine.HostEnvironment) {
	v.w.opts.Host.Settings.SetEnvironment(env)
	v.w.host = env.Appearance
	v.w.redrawAll()
}

// SetTokens sets the values the host themes the app with.
func (v *View) SetTokens(t engine.Tokens) { v.w.opts.Host.Settings.SetTokens(t) }

// Close stops the app, destroys its native components and the child
// window.
func (v *View) Close() {
	if v.w.hwnd != 0 {
		destroyWindow.Call(v.w.hwnd)
	}
}

// testOpen is what a test host opens the app with: the test's options,
// the environment it pins, and where the window goes once it exists.
type testOpen struct {
	options testhost.Options
	pin     engine.TestPin
	ready   chan *window
}

// chromeItem applies an item of the window's own: its title, a window
// command or the menu bar.
func (w *window) chromeItem(it *driver.MountItem) {
	switch it.Kind {
	case driver.ItemTitle:
		w.title = it.Value
		setWindowTextW.Call(w.hwnd, uintptr(unsafe.Pointer(wide(it.Value))))
	case driver.ItemWindow:
		if it.Window != nil {
			w.windowCommand(it.Window)
		}
	case driver.ItemMenu:
		w.setMenu(it.Menu)
	}
}

// owner is the top-level window the app's dialogs and menus belong to:
// its own, or the host's.
func (w *window) owner() uintptr {
	if !w.embedded {
		return w.hwnd
	}
	const gaRoot = 2
	r, _, _ := getAncestor.Call(w.hwnd, gaRoot)
	return r
}

// Mount implements engine.Platform.
func (w *window) Mount(b *driver.MountBatch) {
	if w.batches.Push(b) {
		postMessageW.Call(w.hwnd, wmApply, 0, 0)
	}
}

// RequestFrame implements engine.Platform: the app waits for the next
// display frame. Called from the engine goroutine, so it posts to the
// window, whose frame clock runs on the UI thread.
func (w *window) RequestFrame() { postMessageW.Call(w.hwnd, wmFrame, 0, 0) }

// post runs fn on the main thread; any goroutine may call it.
func (w *window) post(fn func()) {
	w.mu.Lock()
	wake := len(w.tasks) == 0
	w.tasks = append(w.tasks, fn)
	w.mu.Unlock()
	if wake {
		postMessageW.Call(w.hwnd, wmRun, 0, 0)
	}
}

func (w *window) runTasks() {
	w.mu.Lock()
	fns := w.tasks
	w.tasks = nil
	w.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

func (w *window) px(p driver.Points) int32 { return int32(math.Round(float64(p) * w.scale)) }

func (w *window) point(x, y int32) driver.Point {
	return driver.Point{X: driver.Points(float64(x) / w.scale), Y: driver.Points(float64(y) / w.scale)}
}

func (w *window) view(id driver.NodeID) *view {
	if int(id) < len(w.views) {
		return w.views[id]
	}
	return nil
}

func (w *window) apply(b *driver.MountBatch) {
	var rescroll, loads []*view
	structure := false // views came or went, for UI Automation
	lb := w.layoutBatch(b)
	type focusChange struct {
		v     *view // nil dismisses the keyboard
		focus bool
	}
	var focus []focusChange
	for i := range b.Items {
		it := &b.Items[i]
		switch it.Kind {
		case driver.ItemCreate:
			structure = true
			w.create(it.ID, it.View)
			if lb != nil {
				lb.created[w.views[it.ID]] = true
			}
			continue
		case driver.ItemAnimation:
			w.applyAnimations(it.Animation)
			continue
		case driver.ItemAlert:
			// Shown from a message of its own: the dialog's message loop
			// must not run inside this one's batch.
			w.alerts = append(w.alerts, *it.Alert)
			postMessageW.Call(w.hwnd, wmAlert, 0, 0)
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
				w.setClipboardImage(data)
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
			w.setAppearance(it.Appearance)
			continue
		case driver.ItemQuit:
			if w.embedded {
				continue // the host's window stays
			}
			if w.popupTracking {
				endMenu.Call()
			}
			postMessageW.Call(w.hwnd, wmClose, 0, 0)
			continue
		case driver.ItemImageInfo:
			w.imageInfo(it)
			continue
		case driver.ItemKeymap:
			w.setKeymap(it.Keys)
			continue
		case driver.ItemTitle, driver.ItemWindow, driver.ItemMenu:
			// An embedding host owns its window and menu bar.
			if !w.embedded {
				w.chromeItem(it)
			}
			continue
		case driver.ItemAnnounce:
			w.uiaAnnounce(it.Value)
			continue
		case driver.ItemAccessibilityLabel:
			if v := w.view(it.ID); v != nil {
				v.props.Label = it.Value
				w.accessChanged(v, true)
			}
			continue
		case driver.ItemAccessibilityFocus:
			if v := w.view(it.ID); v != nil && v.field != nil {
				w.setFocus(v.field)
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
		case driver.ItemEvents, driver.ItemViewProps, driver.ItemNativeCommand, driver.ItemWebCommand,
			driver.ItemOverflow:
		default:
			// Where it was, where it goes, or what it draws.
			w.redrawView(v)
		}
		switch it.Kind {
		case driver.ItemInsert:
			if p := w.view(it.Parent); p != nil {
				v.detach()
				at := len(p.children)
				if before := w.view(it.Before); before != nil {
					if j := slices.Index(p.children, before); j >= 0 {
						at = j
					}
				}
				p.children = slices.Insert(p.children, at, v)
				v.parent = p
				if lb != nil && lb.created[v] {
					lb.inserted = append(lb.inserted, v)
				}
			}
		case driver.ItemRemove:
			v.detach()
		case driver.ItemDelete:
			w.animator.Forget(v.id)
			w.forgetElements(v)
			structure = true
			if !w.leave(v, lb) {
				w.destroy(v)
			}
		case driver.ItemOverflow:
			v.overflow = it.Frame
		case driver.ItemFrame:
			w.setFrame(v, it.Frame, lb)
			if v.kind == driver.KindScroll {
				rescroll = append(rescroll, v)
			}
		case driver.ItemViewStyle:
			w.setStyle(v, it.ViewStyle)
			if v.web != nil {
				v.web.restyle()
			}
			if v.kind == driver.KindImage {
				// Untinted, an animation plays again.
				w.play(v, &v.source)
				w.play(v, &v.placeholder)
			}
		case driver.ItemText:
			if v.text != nil {
				v.text.Release()
			}
			v.text, _ = it.Text.Layout.(drawnText)
			v.content = it.Text.Content
			if v.plain != it.Text.Text {
				v.plain = it.Text.Text
				w.accessChanged(v, true)
			}
		case driver.ItemInput:
			if v.field != nil && it.Input != nil {
				v.field.configure(it.Input)
			}
		case driver.ItemInputValue:
			if v.field != nil {
				v.field.setValue(it.Value, it.Count)
			}
		case driver.ItemSelection:
			if v.field != nil {
				v.field.selectRange(it.Selection)
			}
		case driver.ItemEvents:
			v.events = it.Events
			if v.field != nil {
				v.field.fs.Events = it.Events
			}
			w.dropEvents(v)
		case driver.ItemViewProps:
			if it.ViewProps != nil {
				v.props = *it.ViewProps
			}
		case driver.ItemScrollConfig:
			to := scrollbar.KeepStart(v.scroll.Behavior.ContentInset, it.Scroll.Behavior.ContentInset, v.offset)
			v.scroll = *it.Scroll
			if v.scroll.HideIndicator {
				v.barAlpha = 0
				w.redrawView(v)
			}
			if to != v.offset {
				w.scrollTo(v, to)
			}
			w.refreshChanged(v)
			rescroll = append(rescroll, v) // insets change the offsets
		case driver.ItemScrollContent:
			v.size = it.Size
			rescroll = append(rescroll, v)
		case driver.ItemFocus, driver.ItemBlur:
			focus = append(focus, focusChange{v, it.Kind == driver.ItemFocus})
		case driver.ItemImage:
			if w.setImage(v, it.Image) {
				loads = append(loads, v)
			}
		case driver.ItemModal:
			// A modal is a view; it appears without animation.
		case driver.ItemCanvas:
			v.drawing = it.Drawing
			w.animator.BindCanvas(v.id, it.Drawing.Slots)
		case driver.ItemShader:
			if it.Shader != nil {
				w.setShader(v, it.Shader)
			}
		case driver.ItemWeb:
			if v.kind == driver.KindWeb && it.Web != nil {
				if v.web == nil {
					v.web = w.newWeb(v)
				}
				v.web.configure(it.Web)
			}
		case driver.ItemWebCommand:
			if v.web != nil {
				v.web.command(it.WebCommand, it.Value)
			}
		case driver.ItemVideo:
			if v.kind == driver.KindVideo && it.Video != nil {
				if v.video == nil {
					v.video = &video{w: w, v: v}
				}
				v.video.configure(it.Video)
			}
		case driver.ItemVideoSeek:
			if v.video != nil {
				v.video.seek(it.Time)
			}
		case driver.ItemNative:
			if v.kind == driver.KindNative && it.Native != nil {
				w.setNative(v, it.Native)
			}
		case driver.ItemNativeCommand:
			if c := v.component; c != nil && c.c != nil {
				c.c.Command(it.Value, it.Data)
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
	if w.root != b.Root {
		w.redrawAll()
	}
	w.root = b.Root
	w.appear(lb)
	// Bindings change with the batch's views.
	w.animatedChanged()
	w.updateDrop()
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
	// Focus once the batch placed the fields.
	for _, f := range focus {
		switch {
		case f.v == nil:
			w.blurAll()
		case f.v.deleted || f.v.field == nil:
		case f.focus:
			if f.v.field.focusable() {
				w.setFocus(f.v.field)
			}
		case w.focusedField() == f.v.field:
			w.setFocus(nil)
		}
	}
	w.checkFocus()
	w.placeIME()
	// Views that moved, appeared or went under a still pointer.
	w.hoverViews()
	w.refreshSelection()
	if structure {
		w.structureChanged()
	}
}

func (w *window) create(id driver.NodeID, kind driver.ViewKind) {
	for len(w.views) <= int(id) {
		w.views = append(w.views, nil)
	}
	v := &view{id: id, kind: kind, style: driver.ViewStyle{Opacity: 1}}
	if kind == driver.KindInput {
		v.field = newTextInput(w, v)
	}
	w.views[id] = v
}

// destroy destroys v. Its children get their own deletes, unless they
// were retired with it: those go with it.
func (w *window) destroy(v *view) {
	v.detach()
	if p := v.leftFrom; p != nil {
		p.leaving = slices.DeleteFunc(p.leaving, func(l leaving) bool { return l.v == v })
		v.leftFrom = nil
	}
	for _, l := range v.leaving {
		l.v.leftFrom = nil
		w.destroy(l.v)
	}
	v.leaving = nil
	children := v.children
	v.children = nil
	for _, c := range children {
		c.parent = nil
		if c.retired {
			w.destroy(c)
		}
	}
	w.release(v)
	if v.field != nil {
		v.field.release() // drawn since it retired, if it did
	}
	if v.text != nil {
		v.text.Release()
		v.text = nil
	}
	v.shadow.release()
	v.shadow = nil
	v.shaderView.release()
	v.glass.release()
	v.shaderView = nil
	v.canvasCache.release()
	for _, sh := range v.boxShadows {
		sh.release()
	}
	v.boxShadows, v.shadowsUsed = nil, nil
	v.retired, v.moving, v.fading, v.drawing = false, nil, nil, nil
	v.source.reset(driver.ImageSource{})
	v.placeholder.reset(driver.ImageSource{})
	v.deleted = true
	w.hovered.Forget(v.id)
	if w.views[v.id] == v {
		w.views[v.id] = nil
	}
}

// retire frees v's ID, as a delete does, but keeps what it draws for a
// layout animation to draw it going: what takes input or plays goes at
// once.
func (w *window) retire(v *view) {
	w.release(v)
	v.retired, v.deleted = true, true
	v.scrolling = nil
	if w.views[v.id] == v {
		w.views[v.id] = nil
	}
}

// release lets go of what a deleted view takes input with, plays or
// hosts, once.
func (w *window) release(v *view) {
	if v.retired {
		return // it did when it retired
	}
	w.dropGone(v)
	if v.field != nil {
		if w.selecting == v.field {
			w.fieldUp()
		}
		v.field.release()
	}
	if v.web != nil {
		v.web.close()
		v.web = nil
	}
	if v.video != nil {
		v.video.close()
		v.video = nil
	}
	if v.component != nil {
		w.closeComponent(v.component)
		v.component = nil
	}
}

func boolArg(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

// scrollTo moves v's content to offset, clamped to the content, and
// reports whether it moved. The engine hit tests with the offset, so a
// move is always reported, whatever v's events.
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
	w.showBar(v)
	w.redrawView(v)
	// Values bound to the offset move in the frame that shows it.
	if w.animator.Scroll(v.id, offset) {
		w.animatedChanged()
	}
	if w.animator.Pulls(v.id) && w.animator.Pull(v.id, scrollbar.Pulled(&v.scroll, offset)) {
		w.animatedChanged()
	}
	w.pinSticky(v)
	// The engine sees the offset within the content and its insets, as a
	// touch pulls it past.
	w.engine.Dispatch(driver.Event{Kind: driver.EventScroll, ID: v.id, Point: scrollbar.Clamp(&v.scroll, v.frame.Size, v.size, offset), Count: v.applied})
}

// wheel scrolls the innermost scroll view under at that can still move
// in the wheel's direction. A wheel goes to the views along its axis, as
// a browser's does, so a vertical wheel scrolls the page around a
// horizontal row; horizontal views take a vertical wheel only where none
// under it is vertical. Tilting scrolls horizontal views only.
func (w *window) wheel(at driver.Point, notches driver.Points, tilt bool) {
	root := w.view(w.root)
	if root == nil {
		return
	}
	v, to := wheelTurn(scrollsAt(root, at, nil), notches, tilt)
	if v == nil {
		return
	}
	from := v.offset
	w.interruptScroll(v) // the wheel takes over
	if w.scrollTo(v, to) {
		w.wheelTurned(v, v.offset.X-from.X+v.offset.Y-from.Y)
	}
}

// wheelTurn returns the scroll view a wheel of notches at under turns,
// and where it goes: the innermost view along the wheel's axis that can
// move its way, else the innermost of any axis where none is along it.
func wheelTurn(under []*view, notches driver.Points, tilt bool) (*view, driver.Point) {
	along := false
	for _, v := range under {
		along = along || v.scroll.Horizontal == tilt
	}
	for i := len(under) - 1; i >= 0; i-- {
		v := under[i]
		if v.scroll.Behavior.NoScroll || along && v.scroll.Horizontal != tilt {
			continue
		}
		offset := v.offset
		switch {
		case tilt && v.scroll.Horizontal: // a tilt right is positive
			offset.X += notches * wheelStep
		case tilt:
			continue
		case v.scroll.Horizontal: // a turn away from the user is positive
			offset.X -= notches * wheelStep
		default:
			offset.Y -= notches * wheelStep
		}
		if to := scrollbar.Clamp(&v.scroll, v.frame.Size, v.size, offset); to != v.offset {
			return v, to
		}
	}
	return nil, driver.Point{}
}

// barDrag is the pointer dragging a scroll bar: origin is the scroll
// view's in the window.
type barDrag struct {
	v      *view
	origin driver.Point
	drag   scrollbar.Drag
}

// bar returns a scroll view's scroll bar.
func (v *view) bar() scrollbar.Bar {
	return scrollbar.Bar{Horizontal: v.scroll.Horizontal, Frame: v.frame.Size, Content: v.size, Offset: v.offset,
		Inset: v.scroll.Behavior.ContentInset}
}

// hoverBars shows the bars of the scroll views under at, the pointer.
func (w *window) hoverBars(at driver.Point) {
	var under []*view
	if root := w.view(w.root); root != nil {
		under = scrollsAt(root, at, nil)
	}
	if !slices.Equal(under, w.bars) {
		for _, v := range w.bars {
			if !slices.Contains(under, v) {
				w.redrawView(v)
			}
		}
		for _, v := range under {
			if !slices.Contains(w.bars, v) {
				w.redrawView(v)
			}
		}
		w.bars = under
	}
	// The pointer over a view keeps its bar up, as its move does.
	for _, v := range under {
		if !v.scroll.Behavior.NoScroll {
			w.showBar(v)
		}
	}
}

// showsBar reports whether v's bar is visible: its opacity is above zero.
func (w *window) showsBar(v *view) bool {
	return !v.scroll.HideIndicator && v.barAlpha > 0
}

// pressBar starts dragging the scroll bar under at, if there is one.
func (w *window) pressBar(at driver.Point) bool {
	root := w.view(w.root)
	if root == nil {
		return false
	}
	v, local := barAt(root, at)
	if v == nil {
		return false
	}
	w.barDrag = &barDrag{v: v, origin: driver.Point{X: at.X - local.X, Y: at.Y - local.Y}, drag: v.bar().PressIndicator(local)}
	w.showBar(v)
	w.interruptScroll(v)
	w.scrollEvent(v, driver.EventScrollDragStart)
	setCapture.Call(w.hwnd)
	w.dragBar(at)
	return true
}

// dragBar scrolls the dragged bar's view to follow the pointer at at.
func (w *window) dragBar(at driver.Point) {
	d := w.barDrag
	if d.v.deleted {
		w.barDrag = nil
		return
	}
	w.interruptScroll(d.v) // the bar takes over
	w.scrollTo(d.v, d.drag.Offset(driver.Point{X: at.X - d.origin.X, Y: at.Y - d.origin.Y}))
}

// barAt returns the innermost scroll view whose bar is under p, and p in
// that view; p is in the coordinates of v's parent's content.
func barAt(v *view, p driver.Point) (*view, driver.Point) {
	hit, at, _ := barIn(v, p)
	return hit, at
}

// barIn is barAt, which also reports whether anything under p takes the
// pointer, which a view that takes none passes to the views below it.
func barIn(v *view, p driver.Point) (*view, driver.Point, bool) {
	if v.style.Hidden || v.style.Opacity <= 0 {
		return nil, p, false
	}
	self, children := v.reach(p)
	if !self && !children {
		return nil, p, false
	}
	local := driver.Point{X: p.X - v.frame.Origin.X, Y: p.Y - v.frame.Origin.Y}
	took := self
	if children {
		inner := driver.Point{X: local.X + v.offset.X, Y: local.Y + v.offset.Y}
		for i := len(v.children) - 1; i >= 0; i-- {
			c := v.children[i]
			if !c.under(inner) {
				continue
			}
			hit, at, childTook := barIn(c, inner)
			if hit != nil {
				return hit, at, true
			}
			if childTook {
				took = true
				break
			}
		}
	}
	if self && v.kind == driver.KindScroll && !v.scroll.HideIndicator && !v.scroll.Behavior.NoScroll && v.bar().Contains(local) {
		return v, local, true
	}
	return nil, p, took
}

// scrollsAt appends the scroll views under p, outermost first; p is in
// the coordinates of v's parent's content.
func scrollsAt(v *view, p driver.Point, out []*view) []*view {
	out, _ = viewsAt(v, p, func(v *view) bool { return v.kind == driver.KindScroll }, out)
	return out
}

// viewsAt appends the views under p that match, outermost first: v and
// the child drawn on top at p, down to the innermost. It also reports
// whether anything under p takes the pointer: a view that takes none, or
// whose children take none, passes it to the views below it, so an
// overlay that lets presses through does not keep the wheel, a hover or
// touch from what it covers.
func viewsAt(v *view, p driver.Point, match func(*view) bool, out []*view) ([]*view, bool) {
	self, children := v.reach(p)
	if v.style.Hidden || v.style.Opacity <= 0 || !self && !children {
		return out, false
	}
	if self && match(v) {
		out = append(out, v)
	}
	took := self
	if children {
		local := driver.Point{X: p.X - v.frame.Origin.X + v.offset.X, Y: p.Y - v.frame.Origin.Y + v.offset.Y}
		for i := len(v.children) - 1; i >= 0; i-- {
			c := v.children[i]
			if !c.under(local) {
				continue
			}
			var childTook bool
			out, childTook = viewsAt(c, local, match, out)
			if childTook {
				return out, true
			}
		}
	}
	return out, took
}

// hoverViews reports the pointer entering and leaving the views that
// follow hovers, as NSH's views hear it from the system.
func (w *window) hoverViews() {
	var under []driver.NodeID
	if root := w.view(w.root); root != nil && w.pointerIn {
		views, _ := viewsAt(root, w.pointer, func(v *view) bool { return hover.Follows(v.events) }, nil)
		for _, v := range views {
			under = append(under, v.id)
		}
	}
	for _, e := range w.hovered.Move(under) {
		w.engine.Dispatch(e)
	}
}

// pointerMoved follows the mouse to client pixel x, y, asking to hear
// when it leaves the client area.
func (w *window) pointerMoved(x, y int32) {
	if !w.tracking {
		w.tracking = true
		t := trackMouse{Flags: tmeLeave, Track: w.hwnd}
		t.Size = uint32(unsafe.Sizeof(t))
		trackMouseEvent.Call(uintptr(unsafe.Pointer(&t)))
	}
	w.pointer, w.pointerIn = w.point(x, y), true
	w.hoverViews()
}

// touch reports the mouse as a finger, when ok.
func (w *window) touch(ev driver.Event, ok bool) {
	if ok {
		w.engine.Dispatch(ev)
	}
}

// listensTouches reports whether a view listens to touches.
func (w *window) listensTouches() bool {
	for _, v := range w.views {
		if v != nil && finger.Listens(v.events) {
			return true
		}
	}
	return false
}

// mouseMoved reports the mouse's move to the views under it that follow
// moves, innermost first: OnMouseMove always, OnHoverMove with no button
// down.
func (w *window) mouseMoved(button bool) {
	root := w.view(w.root)
	if root == nil || !w.pointerIn {
		return
	}
	under, _ := viewsAt(root, w.pointer, func(v *view) bool {
		return v.events.Has(driver.EventMouseMove) || !button && v.events.Has(driver.EventHoverMove)
	}, nil)
	if len(under) == 0 {
		return
	}
	mods := modifiers()
	for i := len(under) - 1; i >= 0; i-- {
		v := under[i]
		b, _ := w.surfaceBounds(v)
		local := driver.Point{X: w.pointer.X - b.Origin.X, Y: w.pointer.Y - b.Origin.Y}
		if !button && v.events.Has(driver.EventHoverMove) {
			w.engine.Dispatch(driver.Event{Kind: driver.EventHoverMove, ID: v.id, Point: local, Page: w.pointer, Modifiers: mods})
		}
		if v.events.Has(driver.EventMouseMove) {
			w.engine.Dispatch(driver.Event{Kind: driver.EventMouseMove, ID: v.id, Point: local})
		}
	}
}

// modifierKey reports a modifier key pressed or released while the mouse
// rests over the window as a hover move to where it is with the new
// modifiers, so a view hears Control come and go under a still mouse.
func (w *window) modifierKey(vk uintptr) {
	switch vk {
	case vkShift, vkControl, vkMenu, vkLWin, vkRWin:
	default:
		return
	}
	root := w.view(w.root)
	if root == nil || !w.pointerIn || keyDown(vkLButton) {
		return
	}
	under, _ := viewsAt(root, w.pointer, func(v *view) bool { return v.events.Has(driver.EventHoverMove) }, nil)
	mods := modifiers()
	for i := len(under) - 1; i >= 0; i-- {
		v := under[i]
		b, _ := w.surfaceBounds(v)
		local := driver.Point{X: w.pointer.X - b.Origin.X, Y: w.pointer.Y - b.Origin.Y}
		w.engine.Dispatch(driver.Event{Kind: driver.EventHoverMove, ID: v.id, Point: local, Page: w.pointer, Modifiers: mods})
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

// pressOutside reports a press at x, y in the client area to each view
// that asks for presses outside it and does not contain the point. It
// does not take the press: the caller handles it as before.
func (w *window) pressOutside(x, y int32) {
	at := w.point(x, y)
	for _, v := range w.views {
		if v == nil || !v.events.Has(driver.EventPressOutside) {
			continue
		}
		if b, shown := w.surfaceBounds(v); shown && !b.Contains(at) {
			w.engine.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: v.id, Point: at})
		}
	}
}

// surfaceBounds returns v's frame in the surface, through its ancestors'
// frames and scroll offsets, and whether v is in the root's tree.
func (w *window) surfaceBounds(v *view) (driver.Bounds, bool) {
	b, top := v.frame, v
	for p := v.parent; p != nil; p = p.parent {
		b.Origin.X += p.frame.Origin.X - p.offset.X
		b.Origin.Y += p.frame.Origin.Y - p.offset.Y
		top = p
	}
	return b, top == w.view(w.root)
}

func (w *window) back() { w.engine.Dispatch(driver.Event{Kind: driver.EventBack}) }

// repeated reports whether a key message's key was already down.
func repeated(lParam uintptr) bool { return lParam&(1<<30) != 0 }

func wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	w := windows[hwnd]
	// A host's child window has no app until it has a size.
	if w == nil || w.embedded && w.engine == nil && message != wmSize && message != wmDestroy {
		r, _, _ := defWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
		return r
	}
	// An alert takes the pointer and the keys while it shows.
	if w.alert != nil {
		switch message {
		case wmMouseMove, wmLButtonDown, wmLButtonUp:
			x, y := int16(lParam&0xffff), int16(lParam>>16&0xffff)
			w.alertMouse(message, w.point(int32(x), int32(y)))
			return 0
		case wmRButtonDown, wmRButtonUp, wmMButtonDown, wmMButtonUp, wmMouseWheel, wmChar:
			return 0
		case wmKeyDown:
			if w.alertKey(wParam) {
				return 0
			}
		}
	}
	if message == wmMouseMove {
		w.pointerMoved(int32(int16(lParam&0xffff)), int32(int16(lParam>>16&0xffff)))
		w.mouseMoved(wParam&(mkLButton|mkRButton|mkMButton) != 0)
	}
	switch message {
	case wmMouseMove, wmLButtonDown, wmLButtonUp, wmRButtonDown, wmRButtonUp, wmMButtonDown, wmMButtonUp,
		wmXButtonDown, wmXButtonUp:
		if w.hostedMouse(message, wParam, lParam) {
			return 0
		}
	case wmPointerDown, wmPointerUpdate, wmPointerUp, wmPointerLeave:
		if w.hostedPointer(message, wParam, lParam) || w.touchPointer(message, wParam, lParam) {
			return 0
		}
	case wmMouseLeave:
		w.mouseLeft()
		return 0
	case wmSetCursor:
		if lParam&0xffff == htClientArea && (w.hostedCursor() || w.fieldCursor() || w.viewCursor()) {
			return 1
		}
	case wmImeSetContext, wmImeStartComposition, wmImeComposition, wmImeEndComposition:
		if r, ok := w.ime(message, wParam, lParam); ok {
			return r
		}
	}
	switch message {
	case wmApply:
		done := w.measuring(true)
		for _, b := range w.batches.Drain() {
			if w.hud != nil && !b.Sent.IsZero() {
				w.hud.monitor.RecordLatency(time.Since(b.Sent))
			}
			w.apply(b)
		}
		done()
		return 0
	case wmRun:
		w.runTasks()
		return 0
	case wmFrame:
		w.frameWanted = true
		w.startClock()
		return 0
	case wmAlert:
		w.showAlert()
		return 0
	case wmPick:
		w.showPick()
		return 0
	case wmPopup:
		w.showPopup()
		return 0
	case wmKeyUp:
		w.modifierKey(wParam)
	case wmSysKeyUp:
		w.modifierKey(wParam)
		if w.menuBarKey(message, wParam, lParam) {
			return 0
		}
	case wmEnterMenuLoop, wmExitMenuLoop:
		w.menuLoop(message == wmEnterMenuLoop, wParam != 0)
	case wmSettingChange:
		// Theme ("ImmersiveColorSet"), locale and accessibility changes.
		w.updateEnvironment()
	case wmSize:
		if wParam != sizeMinimized {
			width, height := int32(lParam&0xffff), int32(lParam>>16&0xffff)
			size := w.point(width, height)
			switch {
			case w.engine != nil:
				w.engine.Resize(driver.Size{Width: size.X, Height: size.Y})
			case width > 0 && height > 0:
				w.startEngine(w.app, w.opts, driver.Size{Width: size.X, Height: size.Y})
			}
		}
		if w.engine == nil {
			return 0
		}
		// A maximize or restore changes the environment.
		w.updateEnvironment()
		return 0
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		w.paint()
		return 0
	case wmLButtonDown, wmRButtonDown:
		if w.embedded {
			// A child window takes the keyboard when pressed.
			if f, _, _ := getFocus.Call(); f != hwnd {
				setFocus.Call(hwnd)
			}
		}
		w.hideMenuBar()
		x, y := int16(lParam&0xffff), int16(lParam>>16&0xffff)
		if message == wmLButtonDown && w.pressBar(w.point(int32(x), int32(y))) {
			return 0
		}
		if message == wmLButtonDown {
			w.touch(w.mouse.Down(w.listensTouches(), w.point(int32(x), int32(y))))
		}
		w.pressOutside(int32(x), int32(y))
		if message == wmLButtonDown {
			at := w.point(int32(x), int32(y))
			clicks := w.countClicks(lParam)
			w.down = driver.Event{Modifiers: modifiers(), Count: uint32(clicks)}
			if w.fieldDown(at, clicks, wParam&mkShift != 0) {
				w.clearSelection()
				return 0
			}
			if w.selectPress(at, clicks, wParam&mkShift != 0) {
				return 0
			}
			w.pressDown(at)
			w.panDown(at)
		}
	case wmMouseMove:
		w.setHover(0) // off the caption buttons
		x, y := int16(lParam&0xffff), int16(lParam>>16&0xffff)
		at := w.point(int32(x), int32(y))
		w.touch(w.mouse.Move(at))
		if w.barDrag != nil {
			w.dragBar(at)
			return 0
		}
		if w.selecting != nil {
			w.fieldMove(at)
			return 0
		}
		if w.selectDrag(at) {
			return 0
		}
		w.hoverBars(at)
		if wParam&mkLButton != 0 {
			w.pressMoved(at)
		}
		if wParam&mkLButton != 0 && w.pan != nil {
			w.panMove(at)
		}
		return 0
	case wmLButtonUp:
		x, y := int16(lParam&0xffff), int16(lParam>>16&0xffff)
		at := w.point(int32(x), int32(y))
		w.touch(w.mouse.Up(at))
		if w.fieldUp() || w.selectUp() {
			return 0
		}
		if w.barDrag != nil {
			w.releaseBar()
			releaseCapture.Call()
			w.hoverBars(at)
			w.redrawAll()
			return 0
		}
		long := w.pressUp()
		if !w.panUp(at) && !long {
			w.press(at)
		}
		return 0
	case wmCaptureChanged:
		if lParam != hwnd && w.selecting != nil {
			w.selecting.drag = dragNone
			w.selecting = nil
		}
		if lParam != hwnd && w.textSelection != nil {
			w.textSelection.Dragging = false
		}
		w.releaseBar()
		w.captureLost(lParam)
		w.panLost()
		if lParam != hwnd {
			w.pressCancel()
		}
		if lParam != hwnd {
			w.touch(w.mouse.Cancel(w.pointer))
		}
		return 0
	case wmGesture:
		if w.gesture(lParam) {
			return 0
		}
	case wmNCCalcSize:
		if r, ok := w.ncCalcSize(wParam, lParam); ok && !w.embedded {
			return r
		}
	case wmNCHitTest:
		if !w.embedded {
			return w.ncHitTest(lParam)
		}
	case wmNCActivate:
		if !w.embedded {
			return w.ncActivate(wParam, lParam)
		}
	case wmNCMouseMove, wmNCMouseLeave, wmNCLButtonDown, wmNCLButtonUp, wmNCLButtonDblClk:
		if !w.embedded && w.ncMouse(message, wParam) {
			return 0
		}
	case wmContextMenu:
		if lParam&0xffffffff == contextFromKey&0xffffffff {
			w.fieldContextMenu(nil)
		} else {
			at := point{int32(int16(lParam & 0xffff)), int32(int16(lParam >> 16 & 0xffff))}
			screenToClient.Call(hwnd, uintptr(unsafe.Pointer(&at)))
			if p := w.point(at.X, at.Y); !w.fieldContextMenu(&p) {
				w.contextMenu(p)
			}
		}
		return 0
	case wmTick:
		w.ticked()
		return 0
	case wmTimer:
		switch wParam {
		case pinchTimer:
			w.wheelPinchEnded()
		case caretTimer:
			w.blink()
		case longPressTimer:
			w.longPressed()
		}
		return 0
	case wmKeyDown:
		if !repeated(lParam) {
			w.modifierKey(wParam)
		}
		// The host's loop translated the key before it came here; Run's
		// loop asks the shortcuts first.
		if w.embedded && w.shortcut(wParam, lParam) {
			w.skipChar = true
			return 0
		}
		// The focused field first, then going back (Escape leaves the
		// field instead), then Tab between the fields.
		if w.fieldKey(wParam) || w.selectKey(wParam) {
			return 0
		}
		if wParam == vkEscape && !repeated(lParam) {
			if w.focusedField() != nil {
				w.blurAll()
			} else {
				w.back()
			}
			return 0
		}
		if wParam == vkTab && !keyDown(vkControl) && !keyDown(vkMenu) && w.tab(keyDown(vkShift)) {
			return 0
		}
	case wmChar:
		if w.skipChar {
			w.skipChar = false
			return 0
		}
		w.fieldChar(uint16(wParam))
		return 0
	case wmSetFocus, wmKillFocus:
		w.windowFocus(message == wmSetFocus)
	case wmSysKeyDown:
		if !repeated(lParam) {
			w.modifierKey(wParam)
		}
		w.menuBarKey(message, wParam, lParam)
		// Alt+Left; DefWindowProc still sees it, or releasing Alt would
		// open the window menu.
		if wParam == vkLeft && !repeated(lParam) {
			w.back()
		}
	case wmAppCommand:
		if lParam>>16&0x0fff == appCommandBack {
			w.back()
			return 1
		}
	case wmMouseWheel, wmMouseHWheel:
		// Wheel messages carry screen coordinates.
		at := point{int32(int16(lParam & 0xffff)), int32(int16(lParam >> 16 & 0xffff))}
		screenToClient.Call(hwnd, uintptr(unsafe.Pointer(&at)))
		delta := int16(wParam >> 16 & 0xffff)
		if w.hostedWheel(at.X, at.Y, delta, message == wmMouseHWheel) {
			return 0
		}
		// A precision touchpad's pinch comes as turns with Control held.
		if message == wmMouseWheel && wParam&mkControl != 0 && w.wheelPinch(w.point(at.X, at.Y), delta) {
			return 0
		}
		w.wheel(w.point(at.X, at.Y), driver.Points(delta)/120, message == wmMouseHWheel)
		return 0
	case wmCommand:
		if lParam == 0 && wParam>>16&0xffff == 0 {
			w.menuCommand(uint16(wParam))
		}
		return 0
	case wmGetObject:
		if r, ok := w.getObject(wParam, lParam); ok {
			return r
		}
	case wmDestroy:
		// Clients let the window's providers go.
		if uiaReturnRawElementProvider.Find() == nil {
			uiaReturnRawElementProvider.Call(hwnd, 0, 0, 0)
		}
		for wv := range w.webs {
			wv.close()
		}
		w.revokeDrop()
		for c := range w.hosts {
			w.closeComponent(c)
		}
		if !w.embedded {
			postQuitMessage.Call(0)
			return 0
		}
		// The host's loop goes on: the view's app ends with its window,
		// and its timers.
		w.ticking = false
		if w.clock != nil {
			w.clock.close()
			w.clock = nil
		}
		if w.engine != nil {
			w.engine.Close()
		}
		w.stopPerf()
		delete(windows, hwnd)
		w.hwnd = 0
		return 0
	case wmMove:
		for wv := range w.webs {
			wv.moved()
		}
	}
	r, _, _ := defWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}
