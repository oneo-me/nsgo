//go:build windows

// Package windows is NSGo's Win32 backend: one window that draws a
// retained tree of views with Direct2D into a flip-model swap chain on a
// Direct3D 11 device, text the engine laid out with DirectWrite, presses
// hit tested by the engine, inputs as editors of the backend's own drawn
// with the views (input.go), web views as WebView2 composition controllers
// in the window's DirectComposition tree, videos played by Media
// Foundation's media engine, images decoded by Go, native components'
// child windows following their views (component.go), files dropped
// through OLE (drop.go), and alerts as task dialogs. It uses only the
// syscall package, so it builds without cgo.
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
	"math"
	"runtime"
	"slices"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/engine"
	"github.com/oneo-me/nsgo/platform/internal/hover"
	"github.com/oneo-me/nsgo/platform/internal/queue"
	"github.com/oneo-me/nsgo/platform/internal/scrollbar"
)

func init() {
	// The window belongs to the thread that creates it: main's.
	runtime.LockOSThread()
}

// Options configure the window.
type Options struct {
	Title         string
	Width, Height core.Points
	// ID names the app's directories and storage (engine.AppDirs).
	ID string
	// Components are the native components Native elements name.
	Components core.NativeComponents
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

	wsOverlappedWindow = 0x00CF0000
	wsClipChildren     = 0x02000000
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
const wheelStep core.Points = 48

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
	id       core.NodeID
	kind     core.ViewKind
	parent   *view
	children []*view
	frame    core.Bounds
	style    core.ViewStyle
	events   core.EventMask
	props    core.ViewProps
	deleted  bool

	// A text view's layout and where it goes in the view.
	text    drawnText
	content core.Bounds

	// The shadow drawn under the background.
	shadow *shadow

	// A scroll view's configuration, content size and offset.
	scroll core.ScrollConfig
	size   core.Size
	offset core.Point

	// An input's editor.
	field *textInput

	// An image view's configuration, source and placeholder.
	image               core.ImageConfig
	source, placeholder imageSlot

	// A canvas's drawing.
	drawing *core.Drawing

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
	animated core.AnimatedValues
	drawn    core.ViewStyle
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
	hwnd    uintptr
	title   string
	engine  *engine.Engine
	batches queue.Batches
	scale   float64 // pixels per point

	// What other goroutines hand the main thread.
	mu    sync.Mutex
	tasks []func()

	// The main thread's.
	views     []*view // index is NodeID
	root      core.NodeID
	env       core.Environment // the last reported
	alerts    []core.AlertConfig
	alerting  bool
	keys      map[string]bool // the keyboard shortcuts
	animating []*view
	ticking   bool
	chrome    chrome
	webs      map[*webView]bool
	cacheDir  string                 // where web views keep their profile
	frames    []func(time.Time) bool // frame clients (onFrame)
	// animator runs the animated values on the frame clock, whose times
	// count from epoch.
	animator *core.Animator
	epoch    time.Time
	// components are the app's native components, hosts the views showing
	// them with a window of their own, and missing the unknown names
	// already logged.
	components core.NativeComponents
	hosts      map[*component]bool
	missing    map[string]bool
	// drop is the window's file drop target while views take dropped
	// files: drops holds those views.
	drop  *dropTarget
	drops map[*view]bool

	// composition is the window's while it has web views, and composed
	// the web views the last frame drew.
	composition *composition
	composed    []*webView
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
	pointer   core.Point
	pointerIn bool

	// The field with the keyboard focus, the one a modal took it from,
	// the one a press is selecting in, and whether the window lacks the
	// keyboard. blinking is set while the caret's timer runs; surrogate
	// holds the first half of a character typed in two WM_CHARs.
	focus, restore, selecting *textInput
	inactive, blinking        bool
	surrogate                 uint16
	// The last press, for counting double and triple clicks.
	clicks struct {
		time  uint32
		at    point
		count int
	}

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
	w := &window{title: opts.Title, scale: 1, webs: map[*webView]bool{}, touchWebs: map[uint32]*webView{},
		epoch: time.Now(), components: opts.Components, hosts: map[*component]bool{}, missing: map[string]bool{},
		drops: map[*view]bool{}}
	w.animator = core.NewAnimator(func(ev core.Event) { w.engine.Dispatch(ev) })
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		w.scale = float64(dpi) / 96
	}

	instance, _, _ := getModuleHandleW.Call(0)
	const idcArrow = 32512
	cursor, _, _ := loadCursorW.Call(0, idcArrow)
	// nsgo links the app's icon as group 1; without one the system's
	// default stands. Windows picks the small icon from the same group.
	const appIcon = 1
	icon, _, _ := loadIconW.Call(instance, appIcon)
	className := wide("NsgoWindow")
	wc := wndClassEx{
		WndProc:   wndProcCallback,
		Instance:  instance,
		Icon:      icon,
		Cursor:    cursor,
		ClassName: className,
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Size the window so its client area is the app's size.
	const style = wsOverlappedWindow | wsClipChildren
	frame := rect{Right: w.px(opts.Width), Bottom: w.px(opts.Height)}
	adjustWindowRect.Call(uintptr(unsafe.Pointer(&frame)), style, 0)
	hwnd, _, _ := createWindowExW.Call(0, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(wide(opts.Title))), style,
		cwUseDefault, cwUseDefault, uintptr(frame.Right-frame.Left), uintptr(frame.Bottom-frame.Top),
		0, 0, instance, 0)
	w.hwnd = hwnd
	windows[hwnd] = w
	// Nothing has the focus yet: typing must not open a composition.
	w.updateIME()
	w.configureGestures()
	w.env = w.environment()
	w.darkTitleBar(w.env.Dark)
	// The engine starts once the window exists, so Mount can post to it.
	eopts := engine.Options{
		Size:        core.Size{Width: opts.Width, Height: opts.Height},
		Environment: w.env,
	}
	data, cache := engine.AppDirs(opts.ID)
	eopts.SetDirs(data, cache)
	w.cacheDir = cache
	w.engine = engine.Start(w, app, eopts)
	showWindow.Call(hwnd, swShow)

	var m msg
	for {
		r, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			w.engine.Close()
			engine.SyncStorage()
			return
		}
		// Shortcuts take key presses before they become characters.
		if (m.Message == wmKeyDown || m.Message == wmSysKeyDown) && m.Hwnd == hwnd && w.shortcut(m.WParam) {
			continue
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// Mount implements engine.Platform.
// Animate implements engine.Platform. This backend has no display link yet:
// apps drive their own frames with a timer, which costs the wakeups a
// platform-driven frame would not. See the macOS backend for the shape.
func (w *window) Animate(on bool) {}

func (w *window) Mount(b *core.MountBatch) {
	if w.batches.Push(b) {
		postMessageW.Call(w.hwnd, wmApply, 0, 0)
	}
}

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

func (w *window) px(p core.Points) int32 { return int32(math.Round(float64(p) * w.scale)) }

func (w *window) point(x, y int32) core.Point {
	return core.Point{X: core.Points(float64(x) / w.scale), Y: core.Points(float64(y) / w.scale)}
}

func (w *window) view(id core.NodeID) *view {
	if int(id) < len(w.views) {
		return w.views[id]
	}
	return nil
}

func (w *window) apply(b *core.MountBatch) {
	var rescroll, loads []*view
	lb := w.layoutBatch(b)
	type focusChange struct {
		v     *view // nil dismisses the keyboard
		focus bool
	}
	var focus []focusChange
	for i := range b.Items {
		it := &b.Items[i]
		switch it.Kind {
		case core.ItemCreate:
			w.create(it.ID, it.View)
			if lb != nil {
				lb.created[w.views[it.ID]] = true
			}
			continue
		case core.ItemAnimation:
			w.applyAnimations(it.Animation)
			continue
		case core.ItemAlert:
			// Shown from a message of its own: the dialog's message loop
			// must not run inside this one's batch.
			w.alerts = append(w.alerts, *it.Alert)
			postMessageW.Call(w.hwnd, wmAlert, 0, 0)
			continue
		case core.ItemDismissKeyboard:
			focus = append(focus, focusChange{})
			continue
		case core.ItemSetClipboard:
			w.setClipboard(it.Value)
			continue
		case core.ItemReadClipboard:
			w.engine.Dispatch(core.Event{Kind: core.EventClipboard, Count: it.Count, Text: w.clipboard()})
			continue
		case core.ItemKeymap:
			w.setKeymap(it.Keys)
			continue
		case core.ItemTitle:
			w.title = it.Value
			setWindowTextW.Call(w.hwnd, uintptr(unsafe.Pointer(wide(it.Value))))
			continue
		case core.ItemWindow:
			if it.Window != nil {
				w.windowCommand(it.Window)
			}
			continue
		case core.ItemMenu:
			w.setMenu(it.Menu)
			continue
		case core.ItemAnnounce, core.ItemStatusBar, core.ItemBackUnhandled:
			// No announcements without UI Automation; no status bar.
			continue
		}
		v := w.view(it.ID)
		if v == nil {
			continue
		}
		switch it.Kind {
		case core.ItemInsert:
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
		case core.ItemRemove:
			v.detach()
		case core.ItemDelete:
			w.animator.Forget(v.id)
			if !w.leave(v, lb) {
				w.destroy(v)
			}
		case core.ItemFrame:
			w.setFrame(v, it.Frame, lb)
			if v.kind == core.KindScroll {
				rescroll = append(rescroll, v)
			}
		case core.ItemViewStyle:
			w.setStyle(v, it.ViewStyle)
			if v.web != nil {
				v.web.restyle()
			}
			if v.kind == core.KindImage {
				// Untinted, an animation plays again.
				w.play(v, &v.source)
				w.play(v, &v.placeholder)
			}
		case core.ItemText:
			if v.text != nil {
				v.text.Release()
			}
			v.text, _ = it.Text.Layout.(drawnText)
			v.content = it.Text.Content
		case core.ItemInput:
			if v.field != nil && it.Input != nil {
				v.field.configure(it.Input)
			}
		case core.ItemInputValue:
			if v.field != nil {
				v.field.setValue(it.Value, it.Count)
			}
		case core.ItemEvents:
			v.events = it.Events
			w.dropEvents(v)
		case core.ItemViewProps:
			if it.ViewProps != nil {
				v.props = *it.ViewProps
			}
		case core.ItemScrollConfig:
			v.scroll = *it.Scroll
		case core.ItemScrollContent:
			v.size = it.Size
			rescroll = append(rescroll, v)
		case core.ItemFocus, core.ItemBlur:
			focus = append(focus, focusChange{v, it.Kind == core.ItemFocus})
		case core.ItemImage:
			if w.setImage(v, it.Image) {
				loads = append(loads, v)
			}
		case core.ItemModal:
			// A modal is a view; it appears without animation.
		case core.ItemCanvas:
			v.drawing = it.Drawing
		case core.ItemWeb:
			if v.kind == core.KindWeb && it.Web != nil {
				if v.web == nil {
					v.web = w.newWeb(v)
				}
				v.web.configure(it.Web)
			}
		case core.ItemWebCommand:
			if v.web != nil {
				v.web.command(it.WebCommand, it.Value)
			}
		case core.ItemVideo:
			if v.kind == core.KindVideo && it.Video != nil {
				if v.video == nil {
					v.video = &video{w: w, v: v}
				}
				v.video.configure(it.Video)
			}
		case core.ItemVideoSeek:
			if v.video != nil {
				v.video.seek(it.Time)
			}
		case core.ItemNative:
			if v.kind == core.KindNative && it.Native != nil {
				w.setNative(v, it.Native)
			}
		case core.ItemNativeCommand:
			if c := v.component; c != nil && c.c != nil {
				c.c.Command(it.Value, it.Data)
			}
		case core.ItemScrollTo:
			v.scrolling = nil
			if it.Animated {
				w.scrollAnimated(v, it.Point)
			} else {
				w.scrollTo(v, it.Point)
			}
		}
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
	invalidateRect.Call(w.hwnd, 0, 0)
}

func (w *window) create(id core.NodeID, kind core.ViewKind) {
	for len(w.views) <= int(id) {
		w.views = append(w.views, nil)
	}
	v := &view{id: id, kind: kind, style: core.ViewStyle{Opacity: 1}}
	if kind == core.KindInput {
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
	v.retired, v.moving, v.fading, v.drawing = false, nil, nil, nil
	v.source.reset(core.ImageSource{})
	v.placeholder.reset(core.ImageSource{})
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
func (w *window) scrollTo(v *view, offset core.Point) bool {
	offset.X = min(max(offset.X, 0), max(v.size.Width-v.frame.Size.Width, 0))
	offset.Y = min(max(offset.Y, 0), max(v.size.Height-v.frame.Size.Height, 0))
	if offset == v.offset {
		return false
	}
	v.offset = offset
	// Values bound to the offset move in the frame that shows it.
	if w.animator.Scroll(v.id, offset) {
		w.animatedChanged()
	}
	w.engine.Dispatch(core.Event{Kind: core.EventScroll, ID: v.id, Point: offset})
	return true
}

// wheel scrolls the innermost scroll view under at that can still move
// in the wheel's direction. Tilting scrolls horizontal views only.
func (w *window) wheel(at core.Point, notches core.Points, tilt bool) {
	root := w.view(w.root)
	if root == nil {
		return
	}
	under := scrollsAt(root, at, nil)
	for i := len(under) - 1; i >= 0; i-- {
		v := under[i]
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
		v.scrolling = nil // the wheel takes over
		if w.scrollTo(v, offset) {
			invalidateRect.Call(w.hwnd, 0, 0)
			return
		}
	}
}

// barDrag is the pointer dragging a scroll bar: origin is the scroll
// view's in the window.
type barDrag struct {
	v      *view
	origin core.Point
	drag   scrollbar.Drag
}

// bar returns a scroll view's scroll bar.
func (v *view) bar() scrollbar.Bar {
	return scrollbar.Bar{Horizontal: v.scroll.Horizontal, Frame: v.frame.Size, Content: v.size, Offset: v.offset}
}

// hoverBars shows the bars of the scroll views under at, the pointer.
func (w *window) hoverBars(at core.Point) {
	var under []*view
	if root := w.view(w.root); root != nil {
		under = scrollsAt(root, at, nil)
	}
	if !slices.Equal(under, w.bars) {
		w.bars = under
		invalidateRect.Call(w.hwnd, 0, 0)
	}
}

// showsBar reports whether v's scroll bar shows: while the pointer is
// over v or drags the bar.
func (w *window) showsBar(v *view) bool {
	return !v.scroll.HideIndicator && (w.barDrag != nil && w.barDrag.v == v || slices.Contains(w.bars, v))
}

// pressBar starts dragging the scroll bar under at, if there is one.
func (w *window) pressBar(at core.Point) bool {
	root := w.view(w.root)
	if root == nil {
		return false
	}
	v, local := barAt(root, at)
	if v == nil {
		return false
	}
	w.barDrag = &barDrag{v: v, origin: core.Point{X: at.X - local.X, Y: at.Y - local.Y}, drag: v.bar().Press(local)}
	setCapture.Call(w.hwnd)
	w.dragBar(at)
	return true
}

// dragBar scrolls the dragged bar's view to follow the pointer at at.
func (w *window) dragBar(at core.Point) {
	d := w.barDrag
	if d.v.deleted {
		w.barDrag = nil
		return
	}
	d.v.scrolling = nil // the bar takes over
	if w.scrollTo(d.v, d.drag.Offset(core.Point{X: at.X - d.origin.X, Y: at.Y - d.origin.Y})) {
		invalidateRect.Call(w.hwnd, 0, 0)
	}
}

// barAt returns the innermost scroll view whose bar is under p, and p in
// that view; p is in the coordinates of v's parent's content.
func barAt(v *view, p core.Point) (*view, core.Point) {
	if v.style.Hidden || v.style.Opacity <= 0 || !v.frame.Contains(p) {
		return nil, p
	}
	local := core.Point{X: p.X - v.frame.Origin.X, Y: p.Y - v.frame.Origin.Y}
	inner := core.Point{X: local.X + v.offset.X, Y: local.Y + v.offset.Y}
	for i := len(v.children) - 1; i >= 0; i-- {
		if c := v.children[i]; !c.style.Hidden && c.frame.Contains(inner) {
			if hit, at := barAt(c, inner); hit != nil {
				return hit, at
			}
			break
		}
	}
	if v.kind == core.KindScroll && !v.scroll.HideIndicator && v.bar().Contains(local) {
		return v, local
	}
	return nil, p
}

// scrollsAt appends the scroll views under p, outermost first; p is in
// the coordinates of v's parent's content.
func scrollsAt(v *view, p core.Point, out []*view) []*view {
	return viewsAt(v, p, func(v *view) bool { return v.kind == core.KindScroll }, out)
}

// viewsAt appends the views under p that match, outermost first: v and
// the child drawn on top at p, down to the innermost.
func viewsAt(v *view, p core.Point, match func(*view) bool, out []*view) []*view {
	if v.style.Hidden || v.style.Opacity <= 0 || !v.frame.Contains(p) {
		return out
	}
	if match(v) {
		out = append(out, v)
	}
	p = core.Point{X: p.X - v.frame.Origin.X + v.offset.X, Y: p.Y - v.frame.Origin.Y + v.offset.Y}
	for i := len(v.children) - 1; i >= 0; i-- {
		if c := v.children[i]; !c.style.Hidden && c.frame.Contains(p) {
			return viewsAt(c, p, match, out)
		}
	}
	return out
}

// hoverViews reports the pointer entering and leaving the views that
// follow hovers, as NSH's views hear it from the system.
func (w *window) hoverViews() {
	var under []core.NodeID
	if root := w.view(w.root); root != nil && w.pointerIn {
		for _, v := range viewsAt(root, w.pointer, func(v *view) bool { return hover.Follows(v.events) }, nil) {
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

// background is the color behind v: its own or its nearest ancestor's.
func background(v *view) core.Color {
	for ; v != nil; v = v.parent {
		if v.style.Background&0xff != 0 {
			return v.style.Background
		}
	}
	return core.RGB(0xffffff)
}

// pressOutside reports a press at x, y in the client area to each view
// that asks for presses outside it and does not contain the point. It
// does not take the press: the caller handles it as before.
func (w *window) pressOutside(x, y int32) {
	at := w.point(x, y)
	for _, v := range w.views {
		if v == nil || !v.events.Has(core.EventPressOutside) {
			continue
		}
		if b, shown := w.surfaceBounds(v); shown && !b.Contains(at) {
			w.engine.Dispatch(core.Event{Kind: core.EventPressOutside, ID: v.id, Point: at})
		}
	}
}

// surfaceBounds returns v's frame in the surface, through its ancestors'
// frames and scroll offsets, and whether v is in the root's tree.
func (w *window) surfaceBounds(v *view) (core.Bounds, bool) {
	b, top := v.frame, v
	for p := v.parent; p != nil; p = p.parent {
		b.Origin.X += p.frame.Origin.X - p.offset.X
		b.Origin.Y += p.frame.Origin.Y - p.offset.Y
		top = p
	}
	return b, top == w.view(w.root)
}

func (w *window) back() { w.engine.Dispatch(core.Event{Kind: core.EventBack}) }

// repeated reports whether a key message's key was already down.
func repeated(lParam uintptr) bool { return lParam&(1<<30) != 0 }

func wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	w := windows[hwnd]
	if w == nil {
		r, _, _ := defWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
		return r
	}
	if message == wmMouseMove {
		w.pointerMoved(int32(int16(lParam&0xffff)), int32(int16(lParam>>16&0xffff)))
	}
	switch message {
	case wmMouseMove, wmLButtonDown, wmLButtonUp, wmRButtonDown, wmRButtonUp, wmMButtonDown, wmMButtonUp,
		wmXButtonDown, wmXButtonUp:
		if w.hostedMouse(message, wParam, lParam) {
			return 0
		}
	case wmPointerDown, wmPointerUpdate, wmPointerUp, wmPointerLeave:
		if w.hostedPointer(message, wParam, lParam) {
			return 0
		}
	case wmMouseLeave:
		w.mouseLeft()
		return 0
	case wmSetCursor:
		if lParam&0xffff == htClientArea && (w.hostedCursor() || w.fieldCursor()) {
			return 1
		}
	case wmImeSetContext, wmImeStartComposition, wmImeComposition, wmImeEndComposition:
		if r, ok := w.ime(message, wParam, lParam); ok {
			return r
		}
	}
	switch message {
	case wmApply:
		for _, b := range w.batches.Drain() {
			w.apply(b)
		}
		return 0
	case wmRun:
		w.runTasks()
		return 0
	case wmAlert:
		w.showAlert()
		return 0
	case wmSettingChange:
		// Theme ("ImmersiveColorSet"), locale and accessibility changes.
		w.updateEnvironment()
	case wmSize:
		if wParam != sizeMinimized {
			width, height := int32(lParam&0xffff), int32(lParam>>16&0xffff)
			size := w.point(width, height)
			w.engine.Resize(core.Size{Width: size.X, Height: size.Y})
		}
		return 0
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		w.paint()
		return 0
	case wmLButtonDown, wmRButtonDown:
		x, y := int16(lParam&0xffff), int16(lParam>>16&0xffff)
		if message == wmLButtonDown && w.pressBar(w.point(int32(x), int32(y))) {
			return 0
		}
		w.pressOutside(int32(x), int32(y))
		if message == wmLButtonDown {
			at := w.point(int32(x), int32(y))
			if w.fieldDown(at, w.countClicks(lParam), wParam&mkShift != 0) {
				return 0
			}
			w.panDown(at)
		}
	case wmMouseMove:
		w.setHover(0) // off the caption buttons
		x, y := int16(lParam&0xffff), int16(lParam>>16&0xffff)
		at := w.point(int32(x), int32(y))
		if w.barDrag != nil {
			w.dragBar(at)
			return 0
		}
		if w.selecting != nil {
			w.fieldMove(at)
			return 0
		}
		w.hoverBars(at)
		if wParam&mkLButton != 0 && w.pan != nil {
			w.panMove(at)
		}
		return 0
	case wmLButtonUp:
		if w.fieldUp() {
			return 0
		}
		x, y := int16(lParam&0xffff), int16(lParam>>16&0xffff)
		at := w.point(int32(x), int32(y))
		if w.barDrag != nil {
			w.barDrag = nil
			releaseCapture.Call()
			w.hoverBars(at)
			invalidateRect.Call(w.hwnd, 0, 0)
			return 0
		}
		if !w.panUp(at) {
			w.press(at)
		}
		return 0
	case wmCaptureChanged:
		if lParam != hwnd && w.selecting != nil {
			w.selecting.drag = dragNone
			w.selecting = nil
		}
		w.barDrag = nil
		w.captureLost(lParam)
		w.panLost()
		return 0
	case wmGesture:
		if w.gesture(lParam) {
			return 0
		}
	case wmNCCalcSize:
		if r, ok := w.ncCalcSize(wParam, lParam); ok {
			return r
		}
	case wmNCHitTest:
		return w.ncHitTest(lParam)
	case wmNCActivate:
		return w.ncActivate(wParam, lParam)
	case wmNCMouseMove, wmNCMouseLeave, wmNCLButtonDown, wmNCLButtonUp, wmNCLButtonDblClk:
		if w.ncMouse(message, wParam) {
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
	case wmTimer:
		switch wParam {
		case animationTimer:
			w.tick()
		case pinchTimer:
			w.wheelPinchEnded()
		case caretTimer:
			w.blink()
		}
		return 0
	case wmKeyDown:
		// The focused field first, then going back (Escape leaves the
		// field instead), then Tab between the fields.
		if w.fieldKey(wParam) {
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
		w.fieldChar(uint16(wParam))
		return 0
	case wmSetFocus, wmKillFocus:
		w.windowFocus(message == wmSetFocus)
	case wmSysKeyDown:
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
		w.wheel(w.point(at.X, at.Y), core.Points(delta)/120, message == wmMouseHWheel)
		return 0
	case wmCommand:
		if lParam == 0 && wParam>>16&0xffff == 0 {
			w.menuCommand(uint16(wParam))
		}
		return 0
	case wmDestroy:
		for wv := range w.webs {
			wv.close()
		}
		w.revokeDrop()
		for c := range w.hosts {
			w.closeComponent(c)
		}
		postQuitMessage.Call(0)
		return 0
	case wmMove:
		for wv := range w.webs {
			wv.moved()
		}
	}
	r, _, _ := defWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}
