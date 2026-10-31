//go:build darwin && !ios

// Package macos is NSGo's AppKit backend: one flipped, layer-backed NSView
// per node, styled through its CALayer, with text the engine laid out
// with CoreText drawn on the main thread and images decoded off it, and
// an optional NSVisualEffectView material behind the window's content.
// Alerts are sheets on the window; Escape, Cmd-[, Option-Left and the
// mouse's back button go back. It calls AppKit through purego, so it
// builds without cgo.
package macos

import (
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/engine"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/finger"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
	"github.com/oneo-me/nsgo/internal/platform/queue"
	"github.com/oneo-me/nsgo/internal/testhost"
)

func init() {
	// AppKit runs on the process's main thread, which main starts on.
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
	// Material is the material behind the app's views, where they draw
	// nothing opaque (nsgo.WindowMaterial).
	Material driver.WindowMaterial
	// Perf shows the performance monitor over the app: frame rate, frame
	// and commit costs, CPU, GPU and memory.
	Perf bool
	// Fonts are font files under the app's resources, registered before
	// the first view.
	Fonts []string
	// Appearance is the appearance the app starts in.
	Appearance driver.Appearance
	// KeepAlive keeps the app running when its window closes: the window
	// hides, and the Dock shows it again.
	KeepAlive bool
	// Host is what an embedding host gives the view (NewView).
	Host engine.HostOptions
}

// longPressDelay is how long a press is held before it is a long press.
const longPressDelay = 500 * time.Millisecond

// view is the Go side of one native view.
type view struct {
	surface *surface
	id      driver.NodeID
	kind    driver.ViewKind
	native  objc.ID
	content objc.ID // where children go: native, or a scroll view's container
	events  driver.EventMask
	// mouseIn says the mouse is over the view, as its tracking area last
	// told.
	mouseIn bool
	// backdrop is the view's backdrop blur, Liquid Glass when glass.
	backdrop objc.ID
	glass    bool

	pressing  atomic.Bool
	longFired atomic.Bool
	pressAt   driver.Event  // where the press began, and how
	hover     objc.ID       // the tracking area of a view reporting hovers
	moves     bool          // whether it reports moves
	overflow  driver.Bounds // where its children reach outside it (ItemOverflow)
	// focusBefore is what had the keyboard focus as a modal came.
	focusBefore objc.ID

	layout *apple.Layout // a text view's
	box    driver.Bounds

	sc    *scrollState // a scroll view's
	input *input       // an input's editor
	inset driver.Bounds

	// contentLayer is what a text or canvas draws into (content.go);
	// stale says its content changed while it could not be seen.
	contentLayer objc.ID
	// axNodes are a canvas's named parts, its accessibility children.
	axNodes []objc.ID
	stale   bool
	// up is the view the batches inserted v in, nil while v is out of
	// the tree.
	up *view

	image     *apple.ImageView
	animation driver.ModalAnimation // a modal's
	presented bool                  // whether a modal was inserted yet
	transform driver.Transform      // AppKit drops it when the frame changes
	drawing   *driver.Drawing       // a canvas's
	played    []driver.DrawOp       // the ops it last drew, its bound arguments' values in place
	canvas    canvasPaint           // how it paints (content.go)
	shader    *apple.ShaderLayer    // a shader view's (shader.go)
	web       *apple.Web            // a web view's
	video     *apple.Video          // a video's

	// frame is where the view shows now; style its own style, which its
	// animated values and a layout animation draw over.
	frame    driver.Bounds
	framed   bool
	shift    driver.Point // a sticky cell's, from frame
	style    driver.ViewStyle
	animated driver.AnimatedValues

	component     driver.NativeComponent // a native view's
	componentView objc.ID
	disabled      bool

	// props are the view's last props; the rest what access.go last
	// gave AppKit.
	props                driver.ViewProps
	propsSet, textSet    bool
	axElement, axGrouped bool
	axCustom             bool

	windowDrag driver.WindowDrag
	activation driver.PanActivation

	// A text's content, links and selection, and whether a view is an
	// area.
	plain            string
	spans            []driver.TextSpan
	selectable       driver.Selectable
	selColor         driver.Color
	selStart, selEnd int // UTF-16
	area             bool
}

type surface struct {
	// stickyBound are the scroll views with sticky cells whose insets
	// follow animated values.
	stickyBound map[*view]bool
	native      objc.ID
	// window is the window native is in: its own, or for an embedded
	// surface the host's, 0 while it is in none.
	window objc.ID
	// embedded is set for a host's view (NewView): the host owns the
	// window, its chrome and the menu bar.
	embedded bool
	// The app and how an embedded surface starts it once it is in a
	// window; observed the window its observations follow.
	app      engine.App
	opts     Options
	observed objc.ID
	// stops end the surface's observations as it closes.
	stops  []func()
	closed bool
	// appearance is the one the app forces, host the one an embedding
	// host sets, which an embedded surface shows in.
	appearance, host driver.Appearance
	engine           *engine.Engine
	batches          queue.Batches
	views            []*view            // index is NodeID
	movers           map[*view]struct{} // views following mouse moves
	root             driver.NodeID      // the root view's, which the batches name
	// mouse is finger 0 of the views' touches; touched is the last
	// NSEvent reported, which AppKit passes up the views.
	mouse   finger.Mouse
	touched struct {
		event objc.ID
		kind  int
		time  float64
	}
	env driver.Environment
	// material is the material behind the app's views, effect the view
	// that shows it.
	material driver.WindowMaterial
	effect   objc.ID
	keys     []driver.KeyBinding // the keymap's bindings

	// What a batch touched, finished once its frames are set.
	images, presenting []*view
	modals             []*view // the modals shown, oldest first, for the keyboard focus
	fieldEditor        objc.ID // NSGo's field editor of the window, made as a field first edits
	// stale are the views whose content changed out of sight.
	stale []*view
	// shading are the shader views to draw once the batch or the frame
	// is applied.
	shading []*view

	// The frame clock and what it animates.
	animator *driver.Animator
	epoch    time.Time
	link     objc.ID // the frame clock's CADisplayLink, made once
	timer    objc.ID // or its NSTimer while ticking, before macOS 14
	ticking  bool
	// watcher checks that the link ticks, stall ticks in its place while
	// it does not; ticked is when a frame last ticked.
	watcher, stall objc.ID
	ticked         time.Time
	// waiting are the batches a frame did not get to; live the scroll
	// views on the frame clock, armed those whose offsets wait to be
	// committed to their bounds.
	waiting []*driver.MountBatch
	live    map[*view]bool
	armed   map[*view]struct{}
	// What the wheel and the trackpad scroll, the indicator the mouse
	// drags, and whether a press caught a snap.
	wheels wheels
	thumb  *thumbDrag
	caught bool
	// resized is the commit at a new size the main thread stopped waiting
	// for, until it lands.
	resized <-chan struct{}
	// frameWanted is set while the app waits for a display frame.
	frameWanted bool
	motions     map[*view]*motion
	ghosts      []*ghost
	components  driver.NativeComponents
	unknown     map[string]bool // component names warned about
	dragTarget  *view           // the view files are dragged over
	dragFiles   bool            // whether the drag carries files
	applying    bool            // whether batches are applying

	// The gestures in progress.
	pan             *panTrack
	pinch, rotation *gestureTrack
	selecting       *selection

	// The performance monitor and the panel that shows it.
	perf *engine.PerfMonitor
	hud  *hud
	// alerts are the alerts shown, by ID, kept for a test host to answer
	// them.
	alerts map[uint32]shownAlert

	chrome
}

// surfaces are the surfaces showing; runner is the one Run shows, which
// the Dock shows again and quitting closes. Only the main thread uses
// them.
var (
	surfaces = map[*surface]bool{}
	runner   *surface
)

// views maps native objects (views and fields) to their Go side.
// Only the main thread uses it.
var views = map[objc.ID]*view{}

var (
	sel = objc.RegisterName

	selAlloc, selInit, selNew       = sel("alloc"), sel("init"), sel("new")
	selRelease                      = sel("release")
	selSetWantsLayer, selLayer      = sel("setWantsLayer:"), sel("layer")
	selSetFrame, selBounds          = sel("setFrame:"), sel("bounds")
	selSetFrameSize                 = sel("setFrameSize:")
	selAddSubview, selSuperview     = sel("addSubview:"), sel("superview")
	selAddSubviewPositioned         = sel("addSubview:positioned:relativeTo:")
	selRemoveFromSuperview          = sel("removeFromSuperview")
	selSetHidden                    = sel("setHidden:")
	selSetNeedsDisplay              = sel("setNeedsDisplay:")
	selSetNeedsLayout               = sel("setNeedsLayout:")
	selConvertPoint                 = sel("convertPoint:fromView:")
	selLocationInWindow             = sel("locationInWindow")
	selStringValue, selSetString    = sel("stringValue"), sel("setStringValue:")
	selSetFont, selSetTextColor     = sel("setFont:"), sel("setTextColor:")
	selSetAlignment                 = sel("setAlignment:")
	selSetPlaceholder               = sel("setPlaceholderString:")
	selSetEditable, selSetEnabled   = sel("setEditable:"), sel("setEnabled:")
	selSetDelegate                  = sel("setDelegate:")
	selSetTarget, selSetAction      = sel("setTarget:"), sel("setAction:")
	selApply                        = sel("nsgoApply:")
	selPerformOnMain                = sel("performSelectorOnMainThread:withObject:waitUntilDone:")
	selWindow, selMakeFirstResponer = sel("window"), sel("makeFirstResponder:")
	selSetAccessibilityElement      = sel("setAccessibilityElement:")
	selSetAccessibilityRole         = sel("setAccessibilityRole:")
	selSetAccessibilitySubrole      = sel("setAccessibilitySubrole:")
	selSetAccessibilityEnabled      = sel("setAccessibilityEnabled:")
	selSetAccessibilitySelected     = sel("setAccessibilitySelected:")
	selSetAccessibilityValue        = sel("setAccessibilityValue:")
	selSetAccessibilityIdentifier   = sel("setAccessibilityIdentifier:")
	selIntrinsicContentSize         = sel("intrinsicContentSize")
	selCurrentContext, selCGContext = sel("currentContext"), sel("CGContext")
	selIsEqualToString              = sel("isEqualToString:")

	classView, classSurface, classInput objc.Class
	classScroll, classContainer         objc.Class
	classOverlay                        objc.Class
	classObserver, classMenuTarget      objc.Class
	classMaterial, classWindow          objc.Class
)

// loadAppKit loads AppKit and registers NSGo's classes, once.
func loadAppKit() {
	if classView != 0 {
		return
	}
	if _, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		panic(err)
	}
	nsView := objc.GetClass("NSView")
	common := []objc.MethodDef{
		{Cmd: sel("makeBackingLayer"), Fn: makeBackingLayer},
		{Cmd: sel("isFlipped"), Fn: func(self objc.ID, _ objc.SEL) bool { return true }},
		// AppKit clips views to their bounds as their frames change, and
		// with them the shadows and outlines drawn outside; a view clips
		// as its style says.
		{Cmd: sel("clipsToBounds"), Fn: func(self objc.ID, _ objc.SEL) bool {
			v := views[self]
			return v == nil || v.style.Clip || v.kind == driver.KindScroll
		}},
		{Cmd: sel("hitTest:"), Fn: hitTest},
		{Cmd: sel("mouseDown:"), Fn: mouseDown},
		{Cmd: sel("mouseUp:"), Fn: mouseUp},
		{Cmd: sel("mouseDragged:"), Fn: mouseDragged},
		{Cmd: sel("magnifyWithEvent:"), Fn: magnify},
		{Cmd: sel("rotateWithEvent:"), Fn: rotate},
		{Cmd: sel("rightMouseDown:"), Fn: rightMouseDown},
		{Cmd: sel("mouseEntered:"), Fn: mouseEntered},
		{Cmd: sel("mouseMoved:"), Fn: mouseMoved},
		{Cmd: sel("mouseExited:"), Fn: mouseExited},
		{Cmd: sel("resetCursorRects"), Fn: resetCursorRects},
		// A click on an inactive window is the app's, as NSH's views take
		// it: AppKit activates the window, then delivers the press.
		{Cmd: sel("acceptsFirstMouse:"), Fn: func(objc.ID, objc.SEL, objc.ID) bool { return true }},
		// The surface moves the window from its drag handles, not AppKit.
		{Cmd: sel("mouseDownCanMoveWindow"), Fn: func(objc.ID, objc.SEL) bool { return false }},
	}
	common = append(common, accessMethods()...)
	classView = register("NsgoView", nsView, nil, append(common, focusMethods()...))
	// NSGo's own windows place the traffic lights again after each layout
	// pass: macOS 27's AppKit puts them back whenever it lays the title
	// bar out, which no notification tells.
	classWindow = register("NsgoWindow", objc.GetClass("NSWindow"), nil, []objc.MethodDef{{
		Cmd: sel("layoutIfNeeded"),
		Fn: func(self objc.ID, cmd objc.SEL) {
			apple.Super(self, cmd)
			if v := views[self.Send(sel("contentView"))]; v != nil {
				v.surface.layoutTrafficLights()
			}
		},
	}})
	// The material sits behind the app's views and takes no press.
	classMaterial = register("NsgoMaterialView", objc.GetClass("NSVisualEffectView"), nil, []objc.MethodDef{{
		Cmd: sel("hitTest:"),
		Fn:  func(objc.ID, objc.SEL, apple.CGPoint) objc.ID { return 0 },
	}})
	refresh := func(self objc.ID, cmd objc.SEL) {
		apple.Super(self, cmd)
		if v := views[self]; v != nil {
			v.surface.refresh()
		}
	}
	classSurface = register("NsgoSurface", nsView, nil, append(common,
		objc.MethodDef{Cmd: sel("setFrameSize:"), Fn: surfaceSetFrameSize},
		objc.MethodDef{Cmd: sel("viewDidMoveToWindow"), Fn: surfaceMoved},
		objc.MethodDef{Cmd: selApply, Fn: surfaceApply},
		objc.MethodDef{Cmd: sel("viewDidChangeEffectiveAppearance"), Fn: refresh},
		objc.MethodDef{Cmd: sel("viewDidChangeBackingProperties"), Fn: refresh},
		objc.MethodDef{Cmd: sel("acceptsFirstResponder"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		// It holds selections, but Tab passes it by.
		objc.MethodDef{Cmd: sel("canBecomeKeyView"), Fn: func(objc.ID, objc.SEL) bool { return false }},
		objc.MethodDef{Cmd: sel("resignFirstResponder"), Fn: surfaceResign},
		// Keys reach the surface only while it holds a selection; it
		// takes them, as a text view would, rather than beep.
		objc.MethodDef{Cmd: sel("keyDown:"), Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
			// Tab still moves the keyboard focus. The surface is in no key
			// view loop, so the window starts it from its first key view.
			const tab, shift = 48, 1 << 17
			if v := views[self]; v != nil && objc.Send[uint16](event, sel("keyCode")) == tab {
				w := v.surface.window
				w.Send(sel("makeFirstResponder:"), objc.ID(0))

				if objc.Send[uint](event, sel("modifierFlags"))&shift != 0 {
					w.Send(sel("selectPreviousKeyView:"), objc.ID(0))
				} else {
					w.Send(sel("selectNextKeyView:"), objc.ID(0))
				}
			}
		}},
		objc.MethodDef{Cmd: sel("copy:"), Fn: surfaceCopy},
		objc.MethodDef{Cmd: sel("selectAll:"), Fn: surfaceSelectAll},
		objc.MethodDef{Cmd: selTick, Fn: surfaceTick},
		objc.MethodDef{Cmd: selWatch, Fn: surfaceWatch},
		objc.MethodDef{Cmd: selStallTick, Fn: surfaceStallTick},
		objc.MethodDef{Cmd: sel("draggingEntered:"), Fn: draggingEntered},
		objc.MethodDef{Cmd: sel("draggingUpdated:"), Fn: draggingUpdated},
		objc.MethodDef{Cmd: sel("draggingExited:"), Fn: draggingExited},
		objc.MethodDef{Cmd: sel("draggingEnded:"), Fn: draggingExited},
		objc.MethodDef{Cmd: sel("prepareForDragOperation:"), Fn: prepareForDrag},
		objc.MethodDef{Cmd: sel("performDragOperation:"), Fn: performDrag},
		objc.MethodDef{Cmd: sel("scrollWheel:"), Fn: scrollWheel},
	))
	classObserver = register("NsgoObserver", objc.GetClass("NSObject"), nil, []objc.MethodDef{{
		Cmd: sel("observeValueForKeyPath:ofObject:change:context:"),
		Fn: func(self objc.ID, _ objc.SEL, path, object, change objc.ID, ctx uintptr) {
			apple.OnMain(func() {
				for s := range surfaces {
					s.refresh()
				}
			})
		},
	}})
	// A scroll view is a view that takes the wheel events of the views in
	// it. Its container holds the children at its offset, and its overlay
	// the indicator and the refresh spinner, letting the mouse through.
	classScroll = register("NsgoScrollView", nsView, nil, append(append(common, focusMethods()...),
		objc.MethodDef{Cmd: sel("scrollWheel:"), Fn: scrollWheel}))
	flipped := objc.MethodDef{Cmd: sel("isFlipped"), Fn: func(objc.ID, objc.SEL) bool { return true }}
	classContainer = register("NsgoContainer", nsView, nil, []objc.MethodDef{flipped,
		{Cmd: sel("clipsToBounds"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("acceptsFirstMouse:"), Fn: func(objc.ID, objc.SEL, objc.ID) bool { return true }},
		{Cmd: sel("mouseDownCanMoveWindow"), Fn: func(objc.ID, objc.SEL) bool { return false }},
	})
	classOverlay = register("NsgoScrollOverlay", nsView, nil, []objc.MethodDef{flipped,
		{Cmd: sel("hitTest:"), Fn: func(objc.ID, objc.SEL, apple.CGPoint) objc.ID { return 0 }},
	})
	classInput = register("NsgoInput", nsView, nil, append(common,
		objc.MethodDef{Cmd: sel("layout"), Fn: inputLayout},
	))
	registerInput()
	registerContent()
	registerCanvasNodes()
	classMenuTarget = register("NsgoMenuTarget", objc.GetClass("NSObject"), nil, []objc.MethodDef{
		{Cmd: selMenuChosen, Fn: menuChosen},
	})
}

var (
	objectSetClass func(obj objc.ID, class objc.Class) objc.Class
	layerBase      objc.Class // the class of AppKit's backing layers
	classLayer     objc.Class // layerBase, holding transforms
)

// makeBackingLayer gives a view AppKit's own backing layer, made a
// subclass that keeps the transform a style gives it.
func makeBackingLayer(self objc.ID, cmd objc.SEL) objc.ID {
	layer := apple.SendSuper[objc.ID](self, cmd)
	if classLayer == 0 {
		loadSetClass()
		layerBase = layer.Class()
		classLayer = register("NsgoLayer", layerBase, nil, apple.HeldTransformMethods())
	}
	if layer.Class() == layerBase {
		objectSetClass(layer, classLayer)
	}
	return layer
}

// loadSetClass loads object_setClass, which gives AppKit's objects
// subclasses of NSGo's.
func loadSetClass() {
	if objectSetClass != nil {
		return
	}
	libobjc, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&objectSetClass, libobjc, "object_setClass")
}

func register(name string, super objc.Class, protocols []*objc.Protocol, methods []objc.MethodDef) objc.Class {
	return apple.RegisterClass(name, super, protocols, methods)
}

// Run opens a window showing app and runs the event loop until the app
// ends: its window closed, unless KeepAlive, or quit. Call it from main.
// It returns once the app's graph is gone and its storage written,
// leaving the process to the caller, as NSH's run_app does.
func Run(app engine.App, opts Options) {
	nsApp := startApp(opts)
	s := openWindow(app, opts, nil)
	runner = s
	// Activate once the run loop runs: a request before it can be lost,
	// and launch activation settles after the app has finished starting.
	apple.OnMain(func() {
		activateApp()
		s.window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	})
	nsApp.Send(sel("run"))
	s.close()
	runner = nil
	engine.SyncStorage()
	nsApp.Send(sel("setDelegate:"), objc.ID(0))
}

// View is an app in an NSView of an embedding host's window, which the
// host's own event loop runs (NewView).
type View struct{ s *surface }

// NewView makes an NSView showing app, for a host that runs AppKit's
// loop itself. Its engine starts once the view is in a window, so
// settings made before that are in the app's first frame. Call it on the
// main thread; several views may show at once, in one window or several.
func NewView(app engine.App, opts Options) *View {
	loadAppKit()
	apple.RegisterFonts(opts.Fonts)
	if opts.Host.Settings == nil {
		opts.Host.Settings = &engine.HostSettings{}
	}
	s := newSurface(opts, true)
	s.app, s.opts = app, opts
	return &View{s}
}

// Handle returns the NSView, which the host adds to its views and sizes.
func (v *View) Handle() uintptr { return uintptr(v.s.native) }

// SetHostEnvironment sets what the host sets over the system's
// environment: its locale, text size and appearance, which AppKit's own
// chrome in the view takes too.
func (v *View) SetHostEnvironment(env engine.HostEnvironment) {
	v.s.opts.Host.Settings.SetEnvironment(env)
	v.s.host = env.Appearance
	v.s.applyAppearance()
}

// SetTokens sets the values the host themes the app with.
func (v *View) SetTokens(t engine.Tokens) { v.s.opts.Host.Settings.SetTokens(t) }

// Close stops the app and frees the view, which the host removes from
// its window.
func (v *View) Close() {
	v.s.close()
	delete(views, v.s.native)
	v.s.native.Send(selRemoveFromSuperview)
	v.s.native.Send(selRelease)
}

// surfaceMoved starts an embedded surface's app once the view is in a
// window, and follows the window it moves to.
func surfaceMoved(self objc.ID, cmd objc.SEL) {
	apple.Super(self, cmd)
	v := views[self]
	if v == nil || v.surface == nil || !v.surface.embedded || v.surface.closed {
		return
	}
	s := v.surface
	s.window = self.Send(selWindow)
	if s.window == 0 {
		return
	}
	if s.engine == nil {
		s.start(s.app, s.opts, nil)
	}
	s.refresh()
}

// startApp makes the process a regular app with NSGo's menu and
// delegate, in opts' appearance, and returns NSApp.
func startApp(opts Options) objc.ID {
	loadAppKit()
	apple.RegisterFonts(opts.Fonts)
	nsApp := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	nsApp.Send(sel("setActivationPolicy:"), 0)
	defaultMenu = mainMenu()
	nsApp.Send(sel("setMainMenu:"), defaultMenu)
	keepAlive = opts.KeepAlive
	delegate := objc.ID(register("NsgoAppDelegate", objc.GetClass("NSObject"), nil, []objc.MethodDef{{
		Cmd: sel("applicationShouldTerminateAfterLastWindowClosed:"),
		Fn:  func(self objc.ID, _ objc.SEL, app objc.ID) bool { return false },
	}, {
		// Quitting returns from Run instead of ending the process.
		Cmd: sel("applicationShouldTerminate:"),
		Fn: func(self objc.ID, _ objc.SEL, app objc.ID) uint {
			finish()
			const terminateCancel = 0
			return terminateCancel
		},
	}, {
		// Clicking the Dock icon shows a window that closed while kept
		// alive.
		Cmd: sel("applicationShouldHandleReopen:hasVisibleWindows:"),
		Fn: func(self objc.ID, _ objc.SEL, app objc.ID, visible bool) bool {
			if runner != nil && !finished {
				runner.orderFront()
			}
			return false
		},
	}, {
		Cmd: sel("applicationWillTerminate:"),
		Fn:  func(self objc.ID, _ objc.SEL, note objc.ID) { engine.SyncStorage() },
	}})).Send(selNew)
	nsApp.Send(sel("setDelegate:"), delegate)
	setAppearance(opts.Appearance)
	return nsApp
}

// testWindow is what a test host opens a window with: the test's options
// and the environment it pins.
type testWindow struct {
	options testhost.Options
	pin     engine.TestPin
}

// shownAlert is an alert on screen and the action of each of its buttons.
type shownAlert struct {
	alert objc.ID
	order []int
}

// defaultMenu is the menu bar startApp makes, which a window shows until
// the app sets its own.
var defaultMenu objc.ID

// openWindow shows app in a new window at opts' size and starts its
// engine; test, when set, is what a test host opens it with.
func openWindow(app engine.App, opts Options, test *testWindow) *surface {
	if opts.Width == 0 {
		opts.Width, opts.Height = 480, 640
	}
	if test != nil && test.options.Size.Width > 0 {
		opts.Width, opts.Height = test.options.Size.Width, test.options.Size.Height
	}
	const titled, closable, miniaturizable, resizable = 1, 2, 4, 8
	window := objc.ID(classWindow).Send(selAlloc).Send(
		sel("initWithContentRect:styleMask:backing:defer:"),
		apple.Rect(0, 0, float64(opts.Width), float64(opts.Height)),
		uint(titled|closable|miniaturizable|resizable), uint(2), false)
	window.Send(sel("setTitle:"), apple.NSString(opts.Title))
	window.Send(sel("setReleasedWhenClosed:"), false)
	// Tab goes through the fields and the views that take presses.
	window.Send(sel("setAutorecalculatesKeyViewLoop:"), true)

	s := newSurface(opts, false)
	s.window = window
	s.setWindowDelegate()
	window.Send(sel("setContentView:"), s.native)
	s.setMaterial(opts.Material)
	s.start(app, opts, test)
	window.Send(sel("center"))
	if test == nil {
		window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	}
	return s
}

// newSurface makes a surface's view at opts' size; embedded is set for a
// host's view.
func newSurface(opts Options, embedded bool) *surface {
	s := &surface{epoch: time.Now(), motions: map[*view]*motion{}, movers: map[*view]struct{}{},
		components: opts.Components, embedded: embedded, appearance: opts.Appearance}
	s.defaultMenu, s.appName = defaultMenu, opts.Title
	s.native = objc.ID(classSurface).Send(selAlloc).Send(sel("initWithFrame:"),
		apple.Rect(0, 0, float64(opts.Width), float64(opts.Height)))
	s.native.Send(selSetWantsLayer, true)
	views[s.native] = &view{surface: s}
	s.animator = driver.NewAnimator(func(e driver.Event) { s.engine.Dispatch(e) })
	s.acceptFiles()
	surfaces[s] = true
	if embedded {
		s.applyAppearance()
	}
	return s
}

// start starts the surface's engine at its view's size; test, when set,
// is what a test host opens it with.
func (s *surface) start(app engine.App, opts Options, test *testWindow) {
	s.env = s.environment()
	var monitor *engine.PerfMonitor
	if opts.Perf {
		monitor = engine.NewPerfMonitor()
		s.perf = monitor
	}
	size := objc.Send[apple.CGRect](s.native, selBounds).Size
	eopts := engine.Options{Size: driver.Size{Width: driver.Points(size.Width), Height: driver.Points(size.Height)},
		Environment: s.env, Perf: monitor, Thread: apple.RaiseThread, Appearance: opts.Appearance, Host: opts.Host}
	eopts.SetDirs(engine.AppDirs(opts.ID))
	if test != nil {
		eopts.UseTest(test.options, &test.pin)
		s.alerts = map[uint32]shownAlert{}
	}
	imageload.UseDiskCache(eopts.CacheDir)
	s.engine = engine.Start(s, app, eopts)
	s.observe()
	if monitor != nil {
		s.hud = newHud(s, monitor)
		s.startTicking()
	}
}

// close stops the surface's frame clock, its observations and its
// engine, then destroys the native components still showing.
func (s *surface) close() {
	if s.closed {
		return
	}
	s.closed = true
	delete(surfaces, s)
	s.stopTicking()
	for _, stop := range s.stops {
		stop()
	}
	s.stops = nil
	if s.engine != nil {
		s.engine.Close()
	}
	for _, v := range s.views {
		if v != nil && v.component != nil {
			v.component.Close()
			v.component = nil
		}
	}
	s.closePopup()
	// Its native objects go with it, and their addresses come back.
	for id, v := range views {
		if v.surface == s && id != s.native {
			delete(views, id)
		}
	}
	delete(menuTargets, s.menuTarget)
}

// keepAlive hides the window instead of closing it; finished is set once
// the app ends.
var keepAlive, finished bool

// finish ends the app: it closes the window, stops AppKit's event loop
// and wakes it, so Run returns.
func finish() {
	if finished {
		return
	}
	finished = true
	if s := runner; s != nil && s.window != 0 {
		s.closePopup()
		// Close without asking the delegate again.
		s.window.Send(selSetDelegate, objc.ID(0))
		s.window.Send(sel("close"))
	}
	app := nsApp()
	app.Send(sel("stop:"), objc.ID(0))
	// stop: takes effect after the next event, and a close from the app
	// or a timer is none: post one.
	const applicationDefined = 15
	event := objc.ID(objc.GetClass("NSEvent")).Send(
		sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:"),
		uint(applicationDefined), apple.CGPoint{}, uint(0), float64(0), 0, objc.ID(0), int16(0), 0, 0)
	if event != 0 {
		app.Send(sel("postEvent:atStart:"), event, true)
	}
}

// activateApp makes the app active. activateIgnoringOtherApps: is
// deprecated but still the request the system honors soonest, and
// macOS 14's replacement, activate, is asynchronous.
func activateApp() {
	app := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	if objc.Send[bool](app, sel("respondsToSelector:"), sel("activateIgnoringOtherApps:")) {
		app.Send(sel("activateIgnoringOtherApps:"), true)
	} else {
		app.Send(sel("activate"))
	}
}

func mainMenu() objc.ID {
	menu := func(title string) objc.ID {
		return objc.ID(objc.GetClass("NSMenu")).Send(selAlloc).Send(sel("initWithTitle:"), apple.NSString(title))
	}
	item := func(m objc.ID, title, action, key string) {
		m.Send(sel("addItemWithTitle:action:keyEquivalent:"), apple.NSString(title), sel(action), apple.NSString(key))
	}
	submenu := func(bar, m objc.ID) {
		it := objc.ID(objc.GetClass("NSMenuItem")).Send(selNew)
		it.Send(sel("setSubmenu:"), m)
		bar.Send(sel("addItem:"), it)
	}
	bar := menu("")
	app := menu("")
	item(app, "Quit", "terminate:", "q")
	submenu(bar, app)
	edit := menu("Edit")
	item(edit, "Undo", "undo:", "z")
	item(edit, "Redo", "redo:", "Z")
	item(edit, "Cut", "cut:", "x")
	item(edit, "Copy", "copy:", "c")
	item(edit, "Paste", "paste:", "v")
	item(edit, "Select All", "selectAll:", "a")
	submenu(bar, edit)
	return bar
}

// environment returns what AppKit tells about where the app runs.
func (s *surface) environment() driver.Environment {
	ws := objc.ID(objc.GetClass("NSWorkspace")).Send(sel("sharedWorkspace"))
	locale, rtl := apple.Locale()
	appearance := apple.GoString(s.native.Send(sel("effectiveAppearance")).Send(sel("name")))
	return driver.Environment{
		Dark:           strings.Contains(appearance, "Dark"),
		FontScale:      1,
		ReduceMotion:   objc.Send[bool](ws, sel("accessibilityDisplayShouldReduceMotion")),
		ScreenReader:   objc.Send[bool](ws, sel("isVoiceOverEnabled")),
		RTL:            rtl,
		PixelRatio:     s.backingScale(),
		Locale:         locale,
		OS:             driver.OSMacOS,
		Idiom:          driver.IdiomDesktop,
		OSVersion:      osVersion(),
		TitleBar:       s.titleBarArea(),
		WindowMaterial: s.material,
		Maximized:      !s.embedded && objc.Send[bool](s.window, sel("isZoomed")),
	}
}

// backingScale is the window's pixels per point, or the main screen's
// while the surface is in none.
func (s *surface) backingScale() float32 {
	if s.window != 0 {
		return float32(objc.Send[float64](s.window, sel("backingScaleFactor")))
	}
	if screen := objc.ID(objc.GetClass("NSScreen")).Send(sel("mainScreen")); screen != 0 {
		return float32(objc.Send[float64](screen, sel("backingScaleFactor")))
	}
	return 2
}

// refresh lays the title bar out again and reports the environment to the
// engine if it changed.
func (s *surface) refresh() {
	if s == nil || s.engine == nil {
		return
	}
	s.layoutTrafficLights()
	if env := s.environment(); env != s.env {
		rescale := env.PixelRatio != s.env.PixelRatio
		s.env = env
		s.engine.SetEnvironment(env)
		if rescale {
			s.rescale()
		}
	}
}

// observe watches what the environment and the back and press-outside
// events come from: accessibility settings, the locale, and the app's
// key and mouse events.
func (s *surface) observe() {
	observeApp()
	center := objc.ID(objc.GetClass("NSNotificationCenter")).Send(sel("defaultCenter"))
	for _, name := range []string{"NSWindowDidResizeNotification", "NSWindowDidEnterFullScreenNotification",
		"NSWindowDidExitFullScreenNotification"} {
		s.stops = append(s.stops, apple.Observe(center, apple.Global(appKitPath, name), func(note objc.ID) {
			if note.Send(sel("object")) != s.window {
				return
			}
			s.refresh()
			if name != "NSWindowDidResizeNotification" {
				// AppKit lays the bar out again as it leaves full screen,
				// after the notification.
				apple.OnMain(func() { s.layoutTrafficLights(); s.refresh() })
			}
		}))
	}
	if !s.embedded {
		// The window's appearance lays its title bar out again too.
		w := s.window
		key := apple.NSString("effectiveAppearance")
		w.Send(sel("addObserver:forKeyPath:options:context:"), appObserver, key, uint(0), uintptr(0))
		s.stops = append(s.stops, func() { w.Send(sel("removeObserver:forKeyPath:"), appObserver, key) })
	}

	const keyDown, leftDown, leftUp, mouseMoved, leftDragged, rightDown, otherDown, wheel, flags = 1 << 10, 1 << 1, 1 << 2, 1 << 5, 1 << 6, 1 << 3, 1 << 25, 1 << 22, 1 << 12
	handler := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID { return s.filter(event) })
	nsEvent := objc.ID(objc.GetClass("NSEvent"))
	monitor := nsEvent.Send(sel("addLocalMonitorForEventsMatchingMask:handler:"),
		uint64(keyDown|leftDown|leftUp|mouseMoved|leftDragged|rightDown|otherDown|wheel|flags), handler).Send(sel("retain"))
	handler.Release()
	s.stops = append(s.stops, func() {
		nsEvent.Send(sel("removeMonitor:"), monitor)
		monitor.Send(selRelease)
	})
}

// appObserver hears the key-value changes every surface follows.
var appObserver objc.ID

// observeApp watches, once for every surface, what the environment
// comes from: accessibility settings and the locale.
func observeApp() {
	if appObserver != 0 {
		return
	}
	const foundation = "/System/Library/Frameworks/Foundation.framework/Foundation"
	refresh := func(objc.ID) {
		for s := range surfaces {
			s.refresh()
		}
	}
	ws := objc.ID(objc.GetClass("NSWorkspace")).Send(sel("sharedWorkspace"))
	apple.Observe(ws.Send(sel("notificationCenter")),
		apple.Global(appKitPath, "NSWorkspaceAccessibilityDisplayOptionsDidChangeNotification"), refresh)
	center := objc.ID(objc.GetClass("NSNotificationCenter")).Send(sel("defaultCenter"))
	apple.Observe(center, apple.Global(foundation, "NSCurrentLocaleDidChangeNotification"), refresh)
	appObserver = objc.ID(classObserver).Send(selNew)
	ws.Send(sel("addObserver:forKeyPath:options:context:"), appObserver, apple.NSString("voiceOverEnabled"), uint(0), uintptr(0))
}

// filter sees the app's key, mouse and wheel events before their views
// do, and measures them when a performance monitor does. It turns back
// keys and the back button into EventBack, which it takes, reports presses
// outside to the views that listen, and follows a selection's drag.
// Presses, scrolls and moves that look for their views commit the scroll
// offsets first, as AppKit's hit testing and tracking areas follow the
// bounds. Down presses are taken only by scroll views, to drag their
// indicators or catch a snap: AppKit activates the window and hands the
// others to the views that hit, which move the window or select text
// before their own handling.
func (s *surface) filter(event objc.ID) objc.ID {
	if s.perf == nil || event.Send(selWindow) != s.window {
		return s.filterEvent(event)
	}
	start := time.Now()
	out := s.filterEvent(event)
	const typeLeftDragged = 6
	dirty := objc.Send[uint](event, sel("type")) == typeLeftDragged
	s.perf.RecordWork(time.Since(start), dirty)
	return out
}

func (s *surface) filterEvent(event objc.ID) objc.ID {
	if s.window == 0 || event.Send(selWindow) != s.window || s.engine == nil {
		return event
	}
	const typeLeftDown, typeLeftUp, typeRightDown, typeMouseMoved, typeLeftDragged, typeKeyDown, typeFlagsChanged, typeScrollWheel, typeOtherDown = 1, 2, 3, 5, 6, 10, 12, 22, 25
	switch objc.Send[uint](event, sel("type")) {
	case typeFlagsChanged:
		s.modifiersChanged(event)
	case typeMouseMoved:
		// The hovers the move reports follow where the content shows.
		s.commitPending()
	case typeScrollWheel:
		if len(s.armed) > 0 && s.wheelStarts(event) {
			s.commitPending()
		}
	case typeKeyDown:
		if !s.hasKeys() {
			break
		}
		if binding, consumed := s.binding(event); consumed {
			if binding != "" {
				s.engine.Dispatch(driver.Event{Kind: driver.EventKey, Text: binding})
			}
			return 0
		}
		if s.selectKey(event) {
			return 0
		}
		if s.isBack(event) {
			s.engine.Dispatch(driver.Event{Kind: driver.EventBack})
			return 0
		}
	case typeLeftDown:
		s.commitPending()
		s.pressOutside(event)
		if s.pressScrolls(event) {
			return 0
		}
	case typeLeftDragged:
		if s.dragScrolls(event) || s.selectDrag(event) {
			return 0
		}
	case typeLeftUp:
		if s.releaseScrolls() || s.selectUp() {
			return 0
		}
	case typeRightDown:
		s.commitPending()
		s.pressOutside(event)
	case typeOtherDown:
		s.commitPending()
		const backButton = 3
		if objc.Send[int](event, sel("buttonNumber")) == backButton && s.under(event) {
			s.engine.Dispatch(driver.Event{Kind: driver.EventBack})
			return 0
		}
	}
	return event
}

// hasKeys reports whether the window's keys are the surface's: the
// keyboard focus is in its view, or nothing has it in a window of its
// own.
func (s *surface) hasKeys() bool {
	first := s.window.Send(sel("firstResponder"))
	if first == 0 || first == s.window {
		return !s.embedded
	}
	return objc.Send[bool](first, sel("isKindOfClass:"), objc.GetClass("NSView")) &&
		objc.Send[bool](first, sel("isDescendantOf:"), s.native)
}

// under reports whether a mouse event is over the surface's view.
func (s *surface) under(event objc.ID) bool {
	if !s.embedded {
		return true
	}
	loc := objc.Send[apple.CGPoint](event, selLocationInWindow)
	p := objc.Send[apple.CGPoint](s.native, selConvertPoint, loc, objc.ID(0))
	b := objc.Send[apple.CGRect](s.native, selBounds)
	return p.X >= 0 && p.Y >= 0 && p.X < b.Size.Width && p.Y < b.Size.Height
}

// AppKit key codes of the keys core.NormalizeKey names.
var keyNames = map[uint16]string{
	36: "enter", 76: "enter", 53: "escape", 48: "tab", 49: "space", 51: "backspace", 117: "delete",
	126: "up", 125: "down", 123: "left", 124: "right", 115: "home", 119: "end", 116: "pageup", 121: "pagedown",
	122: "f1", 120: "f2", 99: "f3", 118: "f4", 96: "f5", 97: "f6", 98: "f7", 100: "f8", 101: "f9",
	109: "f10", 103: "f11", 111: "f12",
}

// binding returns the keymap's binding a key event presses, as
// core.FindKey decides, and whether the event is consumed. It names the
// press both with "mod", Command here, and with "meta". Keys go to an
// input method composing text, as arrows choose its candidates.
func (s *surface) binding(event objc.ID) (string, bool) {
	if len(s.keys) == 0 || s.composing() || activates(event) && s.focusedPressable() {
		return "", false
	}
	name, ok := keyNames[objc.Send[uint16](event, sel("keyCode"))]
	if !ok {
		// The key's character without Shift or Option, in the current layout.
		name = strings.ToLower(apple.GoString(event.Send(sel("charactersByApplyingModifiers:"), uint(0))))
		if name == "" {
			return "", false
		}
	}
	return driver.FindKey(s.keys, apple.KeyNames(name, objc.Send[uint](event, sel("modifierFlags"))), s.editing(),
		objc.Send[bool](event, sel("isARepeat")))
}

// isBack reports whether a key event is Escape, Cmd-[ or Option-Left.
// Escape and Option-Left belong to a text field being edited.
func (s *surface) isBack(event objc.ID) bool {
	const shift, control, option, command = 1 << 17, 1 << 18, 1 << 19, 1 << 20
	const escape, leftArrow = 53, 123
	mods := objc.Send[uint](event, sel("modifierFlags")) & (shift | control | option | command)
	editing := s.editing()
	switch code := objc.Send[uint16](event, sel("keyCode")); {
	case code == escape && mods == 0, code == leftArrow && mods == option:
		return !editing
	case mods == command:
		return apple.GoString(event.Send(sel("charactersIgnoringModifiers"))) == "["
	}
	return false
}

// composing reports whether an input method is composing text in the
// focused text field.
func (s *surface) composing() bool {
	first := s.window.Send(sel("firstResponder"))
	return first != 0 && objc.Send[bool](first, sel("respondsToSelector:"), sel("hasMarkedText")) &&
		objc.Send[bool](first, sel("hasMarkedText"))
}

// editing reports whether keys go to a text field or a web page.
func (s *surface) editing() bool {
	first := s.window.Send(sel("firstResponder"))
	if objc.Send[bool](first, sel("isKindOfClass:"), objc.GetClass("NSText")) {
		return true
	}
	for n := first; n != 0 && objc.Send[bool](n, sel("isKindOfClass:"), objc.GetClass("NSView")); n = n.Send(selSuperview) {
		if v := views[n]; v != nil && v.kind == driver.KindWeb {
			return true
		}
	}
	return false
}

// pressOutside reports a mouse down to each view listening for presses
// outside it that neither holds the view hit nor the point.
func (s *surface) pressOutside(event objc.ID) {
	loc := objc.Send[apple.CGPoint](event, selLocationInWindow)
	var hit objc.ID
	at := driver.Point{}
	for _, v := range s.views {
		if v == nil || !v.events.Has(driver.EventPressOutside) || v.native.Send(selWindow) == 0 {
			continue
		}
		if hit == 0 {
			p := objc.Send[apple.CGPoint](s.native, selConvertPoint, loc, objc.ID(0))
			at = driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}
			up := objc.Send[apple.CGPoint](s.native.Send(selSuperview), selConvertPoint, loc, objc.ID(0))
			if hit = s.native.Send(sel("hitTest:"), up); hit == 0 {
				hit = s.native
			}
		}
		p := objc.Send[apple.CGPoint](v.native, selConvertPoint, loc, objc.ID(0))
		b := objc.Send[apple.CGRect](v.native, selBounds)
		inside := p.X >= 0 && p.Y >= 0 && p.X < b.Size.Width && p.Y < b.Size.Height
		if inside || objc.Send[bool](hit, sel("isDescendantOf:"), v.native) {
			continue
		}
		s.engine.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: v.id, Point: at})
	}
}

// alert shows a as a sheet on the window. The Cancel action answers
// Escape; the sheet's answer goes to the engine as EventAlert.
func (s *surface) alert(a *driver.AlertConfig) {
	alert := objc.ID(objc.GetClass("NSAlert")).Send(selNew)
	alert.Send(sel("setMessageText:"), apple.NSString(a.Title))
	alert.Send(sel("setInformativeText:"), apple.NSString(a.Message))
	// order[i] is the action of the alert's button i: an action sheet's
	// read in order with the cancel action last, as NSH stacks them.
	order := make([]int, 0, len(a.Actions))
	cancel := -1
	for i, act := range a.Actions {
		if a.Sheet && act.Role == driver.AlertCancel && cancel < 0 {
			cancel = i
			continue
		}
		order = append(order, i)
	}
	if cancel >= 0 {
		order = append(order, cancel)
	}
	escape := false
	for _, i := range order {
		act := a.Actions[i]
		button := alert.Send(sel("addButtonWithTitle:"), apple.NSString(act.Label))
		switch act.Role {
		case driver.AlertCancel:
			button.Send(sel("setKeyEquivalent:"), apple.NSString("\x1b"))
			escape = true
		case driver.AlertDestructive:
			button.Send(sel("setHasDestructiveAction:"), true)
		}
	}
	if a.Cancelable && !escape && len(order) > 0 {
		// Escape closes it without an action: a button that is never
		// seen takes the key.
		hidden := alert.Send(sel("addButtonWithTitle:"), apple.NSString(""))
		hidden.Send(sel("setKeyEquivalent:"), apple.NSString("\x1b"))
		hidden.Send(sel("setHidden:"), true)
	}
	id := a.ID
	done := objc.NewBlock(func(_ objc.Block, response int) {
		const firstButton = 1000
		index := response - firstButton
		if index < 0 || index >= len(order) {
			index = -1
		} else {
			index = order[index]
		}
		s.engine.Dispatch(driver.Event{Kind: driver.EventAlert, Count: id, Index: index})
		if s.alerts != nil {
			delete(s.alerts, id)
		}
		alert.Send(selRelease)
	})
	if s.alerts != nil {
		s.alerts[id] = shownAlert{alert, order}
	}
	alert.Send(sel("beginSheetModalForWindow:completionHandler:"), s.window, done)
	done.Release()
}

// pasteboardText is NSPasteboardTypeString.
const pasteboardText = "public.utf8-plain-text"

// Prepare implements core.TextEngine with CoreText.
func (s *surface) Prepare(t *driver.AttributedText) driver.PreparedText {
	return apple.TextEngine{}.Prepare(t)
}

// Mount implements engine.Platform.
func (s *surface) Mount(b *driver.MountBatch) {
	if s.batches.Push(b) {
		s.native.Send(selPerformOnMain, selApply, objc.ID(0), false)
	}
}

// RequestFrame implements engine.Platform: the app waits for the next
// display frame. Called from the engine goroutine, so it hops to the main
// thread, where the display link lives.
func (s *surface) RequestFrame() {
	apple.OnMain(func() {
		s.frameWanted = true
		s.startTicking()
	})
}

func (s *surface) apply(b *driver.MountBatch) {
	var lb *layoutBatch
	if b.LayoutAnimation != nil && !s.env.ReduceMotion {
		lb = &layoutBatch{anim: b.LayoutAnimation, created: map[driver.NodeID]bool{}, deleted: map[objc.ID]bool{}}
	}
	for i := range b.Items {
		it := &b.Items[i]
		switch it.Kind {
		case driver.ItemCreate:
			for len(s.views) <= int(it.ID) {
				s.views = append(s.views, nil)
			}
			s.views[it.ID] = s.create(it.View, it.ID)
			if lb != nil {
				lb.created[it.ID] = true
			}
			continue
		case driver.ItemAnimation:
			s.animator.Apply(it.Animation)
			continue
		case driver.ItemAlert:
			s.alert(it.Alert)
			continue
		case driver.ItemDismissKeyboard:
			if s.hasKeys() {
				s.window.Send(selMakeFirstResponer, objc.ID(0))
			}
			continue
		case driver.ItemSetClipboard:
			pb := objc.ID(objc.GetClass("NSPasteboard")).Send(sel("generalPasteboard"))
			pb.Send(sel("clearContents"))
			pb.Send(sel("setString:forType:"), apple.NSString(it.Value), apple.NSString(pasteboardText))
			continue
		case driver.ItemReadClipboard:
			s.readClipboard(it)
			continue
		case driver.ItemSetClipboardImage:
			if data, ok := it.Data.([]byte); ok {
				setClipboardImage(data)
			}
			continue
		case driver.ItemOpenURL:
			s.openURL(it.Value, it.Count)
			continue
		case driver.ItemPickImages:
			s.pickImages(*it.Pick, it.Count)
			continue
		case driver.ItemPopupMenu:
			s.popupMenu(it.Menu, it.Point, it.Count)
			continue
		case driver.ItemAppearance:
			s.appearance = it.Appearance
			s.applyAppearance()
			continue
		case driver.ItemQuit:
			if !s.embedded {
				apple.OnMain(finish)
			}
			continue
		case driver.ItemImageInfo:
			apple.ImageInfo(it, s.engine.Dispatch)
			continue
		case driver.ItemAnnounce:
			announce(it.Value)
			continue
		case driver.ItemKeymap:
			s.keys = it.Keys
			continue
		case driver.ItemTitle, driver.ItemWindow, driver.ItemMenu:
			// An embedding host owns its window and menu bar.
			if !s.embedded {
				s.chromeItem(it)
			}
			continue
		case driver.ItemStatusBar, driver.ItemBackUnhandled, driver.ItemBackPreview, driver.ItemHistory:
			continue
		}
		v := s.views[it.ID]
		switch it.Kind {
		case driver.ItemDelete:
			v.up = nil
			kept := s.vanish(v, lb)
			s.delete(v, kept)
			if v.kind == driver.KindModal {
				s.modalGone(v)
			}
		case driver.ItemRemove:
			v.up = nil
			v.native.Send(selRemoveFromSuperview)
		case driver.ItemInsert:
			v.up = s.views[it.Parent]
			parent := s.views[it.Parent].content
			if it.Before == 0 {
				parent.Send(selAddSubview, v.native)
			} else {
				const below = -1
				parent.Send(selAddSubviewPositioned, v.native, below, s.views[it.Before].native)
			}
			if v.kind == driver.KindText {
				invalidateTextCursor(v)
			} else if v.style.Cursor != driver.CursorAuto {
				invalidateCursor(v)
			}
			if v.kind == driver.KindModal && !v.presented {
				v.presented = true
				s.presenting = append(s.presenting, v)
				s.modalCame(v)
			}
			s.appear(v, lb, it.Parent)
		case driver.ItemFrame:
			s.setFrame(v, it.Frame, lb)
			if v.kind == driver.KindText {
				invalidateTextCursor(v)
			}
		case driver.ItemOverflow:
			v.overflow = it.Frame
		case driver.ItemViewStyle:
			cursor := v.style.Cursor
			v.style = *it.ViewStyle
			if cursor != v.style.Cursor {
				invalidateCursor(v)
			}
			if apple.UsesFilters(it.ViewStyle) {
				v.native.Send(sel("setLayerUsesCoreImageFilters:"), true)
			}
			if v.animatesStyle() {
				v.restyle()
			} else {
				apple.StyleLayer(v.native.Send(selLayer), it.ViewStyle, v.native)
				v.transform = it.ViewStyle.Transform
			}
			v.setBackdrop(it.ViewStyle)
			v.native.Send(selSetHidden, it.ViewStyle.Hidden)
			if v.image != nil {
				v.image.SetStyle(it.ViewStyle)
			}
		case driver.ItemImage:
			v.image.Configure(it.Image)
			s.images = append(s.images, v)
		case driver.ItemModal:
			v.animation = it.Modal.Animation
		case driver.ItemWeb:
			delete(views, v.web.View())
			v.web.Configure(it.Web)
			views[v.web.View()] = v
		case driver.ItemWebCommand:
			v.web.Command(it.WebCommand, it.Value)
		case driver.ItemVideo:
			delete(views, v.video.View())
			v.video.Configure(it.Video)
			if shown := v.video.View(); shown != 0 {
				views[shown] = v
			}
		case driver.ItemVideoSeek:
			v.video.Seek(it.Time)
		case driver.ItemNative:
			s.configureNative(v, it.Native)
		case driver.ItemNativeCommand:
			if v.component != nil {
				v.component.Command(it.Value, it.Data)
			}
		case driver.ItemCanvas:
			v.drawing = it.Drawing
			v.played = v.played[:0] // drawn whole first
			v.canvas.set(it.Drawing)
			s.animator.BindCanvas(v.id, it.Drawing.Slots)
			v.setCanvasNodes(it.Drawing.Nodes)
			v.redraw()
		case driver.ItemShader:
			s.setShader(v, it.Shader)
		case driver.ItemScrollTo:
			if v.sc != nil {
				v.scrollTo(it.Point, it.Animated)
			}
		case driver.ItemScrollBy:
			if v.sc != nil {
				v.scrollBy(it.Point)
			}
		case driver.ItemZoomTo:
			if v.sc != nil {
				v.zoomTo(it.Zoom, it.Animated)
			}
		case driver.ItemStickyCells:
			if v.sc != nil {
				v.setSticky(it.Sticky)
			}
		case driver.ItemViewProps:
			v.setProps(it.ViewProps)
			p := it.ViewProps
			v.windowDrag, v.activation, v.area, v.selectable = p.WindowDrag, p.PanActivation, p.SelectionArea, p.Selectable
			v.disabled = p.Disabled
			if v.kind == driver.KindText {
				invalidateTextCursor(v)
			}
			if v.selColor != p.SelectionColor {
				v.selColor = p.SelectionColor
				v.redraw()
			}
		case driver.ItemEvents:
			v.setEvents(it.Events)
			v.updateAccessibility()
			if v.kind == driver.KindText {
				invalidateTextCursor(v)
			} else {
				invalidateCursor(v)
			}
		case driver.ItemAccessibilityLabel:
			v.setLabel(it.Value)
		case driver.ItemAccessibilityFocus:
			v.focusAccessibility()
		case driver.ItemText:
			v.layout.Release()
			v.layout, _ = it.Text.Layout.(*apple.Layout)
			v.box = it.Text.Content
			v.sizeContent()
			v.spans = it.Text.Spans
			v.textChanged(it.Text.Text)
			v.redraw()
		case driver.ItemInput:
			v.configure(it.Input)
		case driver.ItemInputValue:
			v.input.value(it.Value, it.Count)
		case driver.ItemSelection:
			if v.input != nil {
				v.input.setSelection(it.Selection)
			}
		case driver.ItemScrollConfig:
			v.configureScroll(it.Scroll)
		case driver.ItemScrollContent:
			v.setContent(it.Size)
		case driver.ItemFocus:
			if v.input != nil {
				v.input.focus()
			}
		case driver.ItemBlur:
			if w := v.native.Send(selWindow); w != 0 {
				w.Send(selMakeFirstResponer, objc.ID(0))
			}
		}
	}
	s.root = b.Root
	if root := s.views[b.Root]; root.native.Send(selSuperview) != s.native {
		s.native.Send(selAddSubview, root.native)
	}
	s.finish()
	s.revisit()
	s.refreshSelection()
	apple.WithoutAnimation(func() {
		if lb != nil {
			s.stepMotions(s.clock())
		}
		s.applyAnimated()
	})
	s.drawShaders()
	if s.animator.Running() {
		s.startTicking()
	}
}

// finish starts what a batch asked for once its frames are set: image
// loads at the views' sizes and modals' entrances.
func (s *surface) finish() {
	s.startImages()
	for _, v := range s.presenting {
		if s.views[v.id] == v && v.native.Send(selSuperview) != 0 {
			h := objc.Send[apple.CGRect](v.native.Send(selSuperview), selBounds).Size.Height
			apple.AnimateIn(v.native.Send(selLayer), v.animation, h, s.env.ReduceMotion)
		}
	}
	clear(s.presenting)
	s.presenting = s.presenting[:0]
}

// startImages starts the image loads a batch or a frame's layout asked
// for, at the views' new sizes.
func (s *surface) startImages() {
	for _, v := range s.images {
		if s.views[v.id] == v {
			v.image.Start(float64(s.env.PixelRatio))
		}
	}
	clear(s.images)
	s.images = s.images[:0]
}

func (s *surface) create(kind driver.ViewKind, id driver.NodeID) *view {
	v := &view{surface: s, id: id, kind: kind}
	switch kind {
	case driver.KindScroll:
		v.native = objc.ID(classScroll).Send(selNew)
		v.native.Send(selSetWantsLayer, true)
		v.newScroll()
	case driver.KindInput:
		v.native = objc.ID(classInput).Send(selNew)
		v.newInput()
	default:
		v.native = objc.ID(classView).Send(selNew)
	}
	v.native.Send(selSetWantsLayer, true)
	if v.content == 0 {
		v.content = v.native
	}
	switch kind {
	case driver.KindImage:
		v.image = apple.NewImageView(v.native.Send(selLayer))
		v.image.Dispatch = v.report
	case driver.KindWeb:
		v.web = apple.NewWeb(v.native)
		v.web.Dispatch = v.report
	case driver.KindVideo:
		v.video = apple.NewVideo(v.native)
		v.video.Dispatch = v.report
	case driver.KindShader:
		v.newShader()
	}
	views[v.native] = v
	return v
}

// delete releases v; kept leaves its native where it is, going away.
func (s *surface) delete(v *view, kept bool) {
	if !kept && !s.inGhost(v.native) {
		v.native.Send(selRemoveFromSuperview)
	}
	s.animator.Forget(v.id)
	delete(s.motions, v)
	delete(s.live, v)
	if v.sc != nil {
		s.forgetScroll(v)
	}
	if s.dragTarget == v {
		s.dragTarget = nil
	}
	if v.component != nil {
		v.component.Close()
		v.component = nil
	}
	apple.ForgetTransform(v.native.Send(selLayer))
	apple.ForgetDecor(v.native.Send(selLayer))
	v.forgetContent()
	v.releaseCanvasNodes()
	if v.shader != nil {
		v.shader.Release()
	}
	v.layout.Release()
	if v.image != nil {
		v.image.Close()
	}
	if v.web != nil {
		delete(views, v.web.View())
		v.web.Close()
	}
	if v.video != nil {
		delete(views, v.video.View())
		v.video.Close()
	}
	if v.hover != 0 {
		v.hover.Send(selRelease)
	}
	delete(views, v.native)
	if in := v.input; in != nil {
		delete(views, in.field)
		delete(views, in.host)
	}
	v.native.Send(selRelease)
	delete(s.movers, v)
	s.views[v.id] = nil
}

func (v *view) setEvents(m driver.EventMask) {
	if v.input != nil {
		v.input.fs.Events = m
	}
	v.events = m
	moves := m.Has(driver.EventHoverMove) || m.Has(driver.EventMouseMove)
	wantHover := moves || m.Has(driver.EventHoverIn) || m.Has(driver.EventHoverOut)
	if moves {
		v.surface.movers[v] = struct{}{}
	} else {
		delete(v.surface.movers, v)
	}
	if v.kind == driver.KindScroll { // its tracking area reports hovers and moves
		return
	}
	if v.hover != 0 && (!wantHover || v.moves != moves) {
		v.native.Send(sel("removeTrackingArea:"), v.hover)
		v.hover.Send(selRelease)
		v.hover = 0
	}
	if wantHover && v.hover == 0 {
		v.hover, v.moves = trackingArea(v.native, moves), moves
	}
}

// trackingArea adds a tracking area over view's visible rect, reporting
// the pointer entering and leaving it and, if moves, moving within it.
func trackingArea(view objc.ID, moves bool) objc.ID {
	const enteredAndExited, mouseMoved, activeInActiveApp, inVisibleRect = 0x01, 0x02, 0x40, 0x200
	options := uint(enteredAndExited | activeInActiveApp | inVisibleRect)
	if moves {
		options |= mouseMoved
	}
	area := objc.ID(objc.GetClass("NSTrackingArea")).Send(selAlloc).Send(
		sel("initWithRect:options:owner:userInfo:"), apple.Rect(0, 0, 0, 0), options, view, objc.ID(0))
	view.Send(sel("addTrackingArea:"), area)
	return area
}

func (v *view) dispatch(kind driver.EventKind) {
	v.report(driver.Event{Kind: kind})
}

// pointer returns event's position in v and in its root view, and the
// modifier keys held, for an event of kind.
func (v *view) pointer(kind driver.EventKind, event objc.ID) driver.Event {
	loc := objc.Send[apple.CGPoint](event, selLocationInWindow)
	p := objc.Send[apple.CGPoint](v.native, selConvertPoint, loc, objc.ID(0))
	return driver.Event{Kind: kind, Point: driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}, Page: v.page(loc),
		Modifiers: apple.Modifiers(objc.Send[uint](event, sel("modifierFlags")))}
}

// page converts a window point to the root view v is in: the modal
// around it, or the surface.
func (v *view) page(loc apple.CGPoint) driver.Point {
	root := v.surface.native
	for n := v.native; n != 0 && n != root; n = n.Send(selSuperview) {
		if m := views[n]; m != nil && m.native == n && m.kind == driver.KindModal {
			root = n
			break
		}
	}
	p := objc.Send[apple.CGPoint](root, selConvertPoint, loc, objc.ID(0))
	return driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}
}

// pressed reports kind about the press v follows, with where it began.
func (v *view) pressed(kind driver.EventKind) {
	e := v.pressAt
	e.Kind = kind
	v.report(e)
}

// report sends e about v if v listens for its kind.
func (v *view) report(e driver.Event) {
	if v.events.Has(e.Kind) {
		e.ID = v.id
		// Values bound to the gesture move before it is reported, so they
		// are drawn in the frame that shows the move.
		if s := v.surface; s.animator.Gesture(&e) {
			apple.WithoutAnimation(s.applyAnimated)
		}
		v.surface.engine.Dispatch(e)
	}
}

func nsColor(c driver.Color) objc.ID {
	r, g, b, a := c.Components()
	return objc.ID(objc.GetClass("NSColor")).Send(sel("colorWithSRGBRed:green:blue:alpha:"), r, g, b, a)
}

// hitTest lets presses on text and plain containers through to the
// nearest pressable ancestor; text fields keep theirs.
func hitTest(self objc.ID, cmd objc.SEL, p apple.CGPoint) objc.ID {
	hit := reach(self, cmd, p)
	if rawHit {
		return hit
	}
	stop := self.Send(selSuperview)
	for n := hit; n != 0 && n != stop; n = n.Send(selSuperview) {
		if v := views[n]; v != nil {
			if v.isEditor(n) || v.kind == driver.KindScroll || v.kind == driver.KindWeb {
				return hit
			}
			if v.kind == driver.KindVideo && v.native != n {
				return hit // the player's controls
			}
			if (v.pressable() || v.events.Has(driver.EventPan)) && v.native == n {
				return n
			}
		}
	}
	return hit
}

// reach is AppKit's hit test of self at p, in its superview's
// coordinates, by its pointer events and hit slop, and looking in its
// children where they reach outside it, as core.Reach says.
func reach(self objc.ID, cmd objc.SEL, p apple.CGPoint) objc.ID {
	v := views[self]
	if v == nil || v.native != self {
		return apple.SendSuper[objc.ID](self, cmd, p)
	}
	f := objc.Send[apple.CGRect](self, sel("frame"))
	frame := driver.Bounds{Origin: driver.Point{X: driver.Points(f.Origin.X), Y: driver.Points(f.Origin.Y)},
		Size: driver.Size{Width: driver.Points(f.Size.Width), Height: driver.Points(f.Size.Height)}}
	at := driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}
	own, children := driver.Reach(frame, &v.props, v.overflow, at)
	if !frame.Contains(at) {
		if children {
			local := objc.Send[apple.CGPoint](self, selConvertPoint, p, self.Send(selSuperview))
			subs := self.Send(sel("subviews"))
			for i := objc.Send[int](subs, sel("count")) - 1; i >= 0; i-- {
				if hit := subs.Send(sel("objectAtIndex:"), i).Send(cmd, local); hit != 0 {
					return hit
				}
			}
		}
		if own {
			return self
		}
		return 0
	}
	hit := apple.SendSuper[objc.ID](self, cmd, p)
	switch {
	case !own && !children:
		return 0
	case !children && hit != 0: // box-only
		return self
	case !own && hit == self: // box-none
		return 0
	}
	return hit
}

// within reports whether p, in v's coordinates, is on v of bounds b or
// in its hit slop, where a press ends as a press.
func (v *view) within(p apple.CGPoint, b apple.CGRect) bool {
	s := v.props.HitSlop
	return p.X >= -float64(s.Left) && p.Y >= -float64(s.Top) && p.X < b.Size.Width+float64(s.Right) &&
		p.Y < b.Size.Height+float64(s.Bottom)
}

// longPressDelay is how long a press on v is held to be a long press.
func (v *view) longPressDelay() time.Duration {
	if d := v.props.DelayLongPress; d > 0 {
		return d
	}
	return longPressDelay
}

// touch reports event, a mouse event at a point of the window, as a
// finger with report, once however many views it reaches.
func (s *surface) touch(event objc.ID, report func(*surface, driver.Point) (driver.Event, bool)) {
	seen := s.touched
	seen.event, seen.kind, seen.time = event, objc.Send[int](event, sel("type")), objc.Send[float64](event, selTimestamp)
	if seen == s.touched {
		return
	}
	s.touched = seen
	loc := objc.Send[apple.CGPoint](event, selLocationInWindow)
	p := objc.Send[apple.CGPoint](s.native, selConvertPoint, loc, objc.ID(0))
	if ev, ok := report(s, driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}); ok {
		s.engine.Dispatch(ev)
	}
}

// listensTouches reports whether a view listens to touches.
func (s *surface) listensTouches() bool {
	for _, v := range s.views {
		if v != nil && finger.Listens(v.events) {
			return true
		}
	}
	return false
}

func (v *view) pressable() bool {
	return v.events.Has(driver.EventPress) || v.events.Has(driver.EventLongPress) ||
		v.events.Has(driver.EventPressDown) || v.events.Has(driver.EventPressUp)
}

// pressableAt reports whether the view takes a press at p, in its
// coordinates: a text with links only on one.
func (v *view) pressableAt(p apple.CGPoint) bool {
	return v.pressable() && (v.kind != driver.KindText || driver.PressesAt(v.spans, v.indexAt(p)))
}

func mouseDown(self objc.ID, cmd objc.SEL, event objc.ID) {
	v := views[self]
	// Control-click is a secondary click.
	const control = 1 << 18
	if v != nil && v.events.Has(driver.EventContextMenu) && objc.Send[uint](event, sel("modifierFlags"))&control != 0 {
		rightMouseDown(self, cmd, event)
		return
	}
	if v != nil {
		v.surface.touch(event, func(s *surface, p driver.Point) (driver.Event, bool) {
			return s.mouse.Down(s.listensTouches(), p)
		})
	}
	// AppKit has activated the window and handed the press to the view it
	// hit: a drag handle moves the window, and a selection area starts or
	// extends its selection, before the view sees it.
	if v != nil && v.surface.takePress(event) {
		return
	}
	if v != nil {
		v.surface.startPan(self, event)
	}
	if v == nil || !v.pressable() {
		if v == nil || v.surface.pan == nil {
			apple.Super(self, cmd, event)
		}
		return
	}
	v.pressing.Store(true)
	v.longFired.Store(false)
	v.pressAt = v.pointer(driver.EventPressDown, event)
	v.pressAt.Count = uint32(max(objc.Send[int](event, sel("clickCount")), 1))
	v.pressed(driver.EventPressDown)
	if v.events.Has(driver.EventLongPress) {
		e, long := v.surface.engine, v.pressAt
		long.Kind, long.ID = driver.EventLongPress, v.id
		time.AfterFunc(v.longPressDelay(), func() {
			if v.pressing.Load() && v.longFired.CompareAndSwap(false, true) {
				e.Dispatch(long)
			}
		})
	}
}

func mouseUp(self objc.ID, cmd objc.SEL, event objc.ID) {
	v := views[self]
	if v != nil {
		v.surface.touch(event, func(s *surface, p driver.Point) (driver.Event, bool) { return s.mouse.Up(p) })
	}
	panned := v != nil && v.surface.endPan(event)
	if v == nil || !v.pressable() {
		if !panned {
			apple.Super(self, cmd, event)
		}
		return
	}
	p := objc.Send[apple.CGPoint](self, selConvertPoint, objc.Send[apple.CGPoint](event, selLocationInWindow), objc.ID(0))
	b := objc.Send[apple.CGRect](self, selBounds)
	pressed := v.pressing.Swap(false)
	if !pressed {
		return // a pan took the press
	}
	v.pressed(driver.EventPressUp)
	if pressed && !v.longFired.Load() && v.within(p, b) {
		e := v.pressAt
		e.Kind, e.Index = driver.EventPress, v.indexAt(p)
		v.report(e)
	}
}

// indexAt returns the byte offset of the character at p in a text view,
// for presses on its links.
func (v *view) indexAt(p apple.CGPoint) int {
	if v.layout == nil {
		return -1
	}
	box := v.box
	return v.layout.IndexAt(p, apple.Rect(float64(box.Origin.X), float64(box.Origin.Y),
		float64(box.Size.Width), float64(box.Size.Height)))
}

// rightMouseDown reports a secondary click to a view listening for
// context menus, and passes it on otherwise.
func rightMouseDown(self objc.ID, cmd objc.SEL, event objc.ID) {
	v := views[self]
	if v == nil || !v.events.Has(driver.EventContextMenu) {
		apple.Super(self, cmd, event)
		return
	}
	v.report(v.pointer(driver.EventContextMenu, event))
}

// resetCursorRects gives the view its style's cursor, the I-beam over a
// text a press selects, or none to inherit its parent's.
func resetCursorRects(self objc.ID, cmd objc.SEL) {
	apple.Super(self, cmd)
	v := views[self]
	if v == nil {
		return
	}
	cursor := v.style.Cursor
	if cursor == driver.CursorAuto && v.selects() {
		cursor = driver.CursorText
	}
	if cursor != driver.CursorAuto {
		self.Send(sel("addCursorRect:cursor:"), objc.Send[apple.CGRect](self, selBounds), nsCursor(cursor))
	}
	// AppKit takes the cursor from the view hitTest: returns, which for a
	// text in a pressable view is the pressable: it shows their I-beams.
	if v.native == self && v.kind != driver.KindText && v.pressable() {
		v.textCursors(self)
	}
}

// textCursors adds the I-beams of the selecting texts in n to pressable
// v's cursor rects, down to the views hitTest: returns themselves.
func (v *view) textCursors(n objc.ID) {
	for _, c := range apple.Subviews(n) {
		w := views[c]
		if w == nil || w.native != c {
			v.textCursors(c)
			continue
		}
		switch {
		case w.pressable() || w.kind == driver.KindScroll || w.events.Has(driver.EventPan):
		case w.kind == driver.KindText:
			if w.style.Cursor == driver.CursorAuto && w.selects() {
				r := objc.Send[apple.CGRect](v.native, sel("convertRect:fromView:"), objc.Send[apple.CGRect](c, selBounds), c)
				v.native.Send(sel("addCursorRect:cursor:"), r, nsCursor(driver.CursorText))
			}
		default:
			v.textCursors(c)
		}
	}
}

// selects reports whether a press on text v selects, as areaAt finds:
// it is in an area, or selectable on its own, with no gesture between.
// Its ancestors are in place by the time AppKit asks for cursor rects.
func (v *view) selects() bool {
	if v.kind != driver.KindText || v.selectable == driver.SelectableOff {
		return false
	}
	for n := v.up; n != nil; n = n.up {
		if n.area {
			return true
		}
		if n.events.Has(driver.EventPan) {
			return false
		}
	}
	return v.selectable == driver.SelectableOn
}

// invalidateCursor makes the window ask for the view's cursor rects again.
func invalidateCursor(v *view) {
	if w := v.native.Send(selWindow); w != 0 {
		w.Send(sel("invalidateCursorRectsForView:"), v.native)
	}
}

// invalidateTextCursor makes the window ask again for a text's cursor
// rects and those of the pressable view showing its I-beam.
func invalidateTextCursor(v *view) {
	invalidateCursor(v)
	for n := v.up; n != nil; n = n.up {
		if n.pressable() {
			invalidateCursor(n)
			return
		}
	}
}

// nsCursor is the AppKit cursor of c. AppKit has no public wait, progress,
// help or zoom cursors: those show the arrow.
func nsCursor(c driver.Cursor) objc.ID {
	class := objc.ID(objc.GetClass("NSCursor"))
	pick := func(name string) objc.ID { return class.Send(sel(name)) }
	switch c {
	case driver.CursorPointer:
		return pick("pointingHandCursor")
	case driver.CursorText:
		return pick("IBeamCursor")
	case driver.CursorCrosshair:
		return pick("crosshairCursor")
	case driver.CursorMove, driver.CursorGrab:
		return pick("openHandCursor")
	case driver.CursorGrabbing:
		return pick("closedHandCursor")
	case driver.CursorNotAllowed:
		return pick("operationNotAllowedCursor")
	case driver.CursorColResize:
		return pick("resizeLeftRightCursor")
	case driver.CursorRowResize:
		return pick("resizeUpDownCursor")
	case driver.CursorNone:
		return invisibleCursor()
	}
	return pick("arrowCursor")
}

var invisibleNS objc.ID

// invisibleCursor is a cursor that draws nothing.
func invisibleCursor() objc.ID {
	if invisibleNS == 0 {
		image := objc.ID(objc.GetClass("NSImage")).Send(sel("alloc"))
		image = image.Send(sel("initWithSize:"), apple.CGSize{Width: 1, Height: 1})
		invisibleNS = objc.ID(objc.GetClass("NSCursor")).Send(sel("alloc"))
		invisibleNS = invisibleNS.Send(sel("initWithImage:hotSpot:"), image, apple.CGPoint{})
		image.Send(selRelease)
	}
	return invisibleNS
}

func mouseEntered(self objc.ID, _ objc.SEL, _ objc.ID) {
	if v := views[self]; v != nil {
		defer v.surface.measuring(false)()
		if v.native == self {
			v.mouseIn = true
		}
		if v.sc != nil && v.native == self {
			v.hovered()
		}
		v.dispatch(driver.EventHoverIn)
	}
}

// modifiersChanged reports a modifier key pressed or released while the
// mouse rests as a move to where it is with the new modifiers, to the
// views under it that hear hover moves: a link that shows while Command
// is down.
func (s *surface) modifiersChanged(event objc.ID) {
	mods := apple.Modifiers(objc.Send[uint](event, sel("modifierFlags")))
	loc := objc.Send[apple.CGPoint](s.window, sel("mouseLocationOutsideOfEventStream"))
	for v := range s.movers {
		if !v.mouseIn || !v.events.Has(driver.EventHoverMove) {
			continue
		}
		p := objc.Send[apple.CGPoint](v.native, selConvertPoint, loc, objc.ID(0))
		v.report(driver.Event{Kind: driver.EventHoverMove, Point: driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)},
			Page: v.page(loc), Modifiers: mods})
	}
}

// mouseMoved reports a move with no button down to the view whose
// tracking area it crossed. It goes no further up the responder chain,
// since the views around follow their own areas.
func mouseMoved(self objc.ID, _ objc.SEL, event objc.ID) {
	v := views[self]
	if v == nil || v.native != self {
		return
	}
	defer v.surface.measuring(false)()
	if v.sc != nil {
		v.hovered()
	}
	if v.events.Has(driver.EventHoverMove) {
		v.report(v.pointer(driver.EventHoverMove, event))
	}
	if v.events.Has(driver.EventMouseMove) {
		v.report(v.pointer(driver.EventMouseMove, event))
	}
}

// dragMoved reports a move with a button down, which AppKit sends only to
// the view the press began in, to every view under the pointer that
// follows mouse moves, innermost first.
func (s *surface) dragMoved(event objc.ID) {
	if len(s.movers) == 0 {
		return
	}
	loc := objc.Send[apple.CGPoint](event, selLocationInWindow)
	type under struct {
		v     *view
		depth int
		at    apple.CGPoint
	}
	var hit []under
	for v := range s.movers {
		if !v.events.Has(driver.EventMouseMove) || v.native.Send(selWindow) == 0 ||
			objc.Send[bool](v.native, sel("isHiddenOrHasHiddenAncestor")) {
			continue
		}
		p := objc.Send[apple.CGPoint](v.native, selConvertPoint, loc, objc.ID(0))
		b := objc.Send[apple.CGRect](v.native, selBounds)
		if p.X < 0 || p.Y < 0 || p.X >= b.Size.Width || p.Y >= b.Size.Height {
			continue
		}
		depth := 0
		for n := v.native; n != 0; n = n.Send(selSuperview) {
			depth++
		}
		hit = append(hit, under{v, depth, p})
	}
	slices.SortFunc(hit, func(a, b under) int { return b.depth - a.depth })
	for _, u := range hit {
		u.v.report(driver.Event{Kind: driver.EventMouseMove, Point: driver.Point{X: driver.Points(u.at.X), Y: driver.Points(u.at.Y)}})
	}
}

func mouseExited(self objc.ID, _ objc.SEL, _ objc.ID) {
	if v := views[self]; v != nil {
		defer v.surface.measuring(false)()
		if v.native == self {
			v.mouseIn = false
		}
		v.dispatch(driver.EventHoverOut)
	}
}

// resizeWait is the longest the main thread waits for the layout at a new
// size, the one place it waits for the engine, as NSH's macOS backend
// does: AppKit draws the window as soon as its frame changes, around the
// old content otherwise.
const resizeWait = 16 * time.Millisecond

func surfaceSetFrameSize(self objc.ID, cmd objc.SEL, size apple.CGSize) {
	v := views[self]
	var s *surface
	if v != nil {
		s = v.surface
	}
	if s != nil {
		// What waited goes first, as the resize's own commit follows it.
		// An animated resize, such as zooming from the title bar, steps the
		// frame in a loop that runs no run loop sources until it ends, so
		// nsgoApply: waits: each step applies what came in since.
		s.applyBatches(0, true)
	}
	if s != nil && s.perf != nil {
		defer s.measuring(false)()
	}
	apple.Super(self, cmd, size)
	if s != nil && s.engine != nil {
		s.resize(driver.Size{Width: driver.Points(size.Width), Height: driver.Points(size.Height)})
	}
	if s != nil && s.hud != nil {
		s.hud.layout()
	}
}

// resize tells the engine the surface's new size and waits, at most
// resizeWait, for the commit laid out at it, which it applies before
// AppKit draws. An engine that missed the deadline is not waited for
// again until that commit lands, so a busy app resizes as before.
func (s *surface) resize(size driver.Size) {
	if s.resized != nil {
		select {
		case <-s.resized:
			s.resized = nil
		default:
		}
	}
	done := s.engine.ResizeAndWait(size)
	if s.resized != nil || s.applying {
		return
	}
	t := time.NewTimer(resizeWait)
	defer t.Stop()
	select {
	case <-done:
		s.applyBatches(0, true)
	case <-t.C:
		s.resized = done
	}
}

func surfaceApply(self objc.ID, _ objc.SEL, _ objc.ID) {
	views[self].surface.applyQueued()
}

// applyQueued applies the batches the engine has sent as they land,
// unless something animates: then they wait for the start of the next
// frame, which applies them first.
func (s *surface) applyQueued() {
	if s.animating() {
		s.startTicking()
		return
	}
	s.applyBatches(0, true)
}

// applyBatches applies the batches waiting, in order, for at most budget
// (0: all of them) and at least one, unless a batch is being applied: one
// can resize the surface. alone records the work as the main thread's
// own, outside a frame's.
func (s *surface) applyBatches(budget time.Duration, alone bool) {
	if s.applying {
		return
	}
	s.waiting = append(s.waiting, s.batches.Drain()...)
	if len(s.waiting) == 0 {
		return
	}
	s.applying = true
	defer func() { s.applying = false }()
	start := time.Now()
	n := 0
	for n < len(s.waiting) {
		b := s.waiting[n]
		n++
		if s.perf != nil && !b.Sent.IsZero() {
			s.perf.RecordLatency(time.Since(b.Sent))
		}
		s.apply(b)
		if budget > 0 && time.Since(start) >= budget {
			break
		}
	}
	rest := copy(s.waiting, s.waiting[n:])
	clear(s.waiting[rest:])
	s.waiting = s.waiting[:rest]
	if s.perf != nil {
		if alone {
			s.perf.RecordWork(time.Since(start), true)
		}
		if s.hud != nil {
			s.hud.raise()
		}
	}
	// A frame requested before the view was in a window starts the clock
	// here, a display link needs the view mounted; the next frame applies
	// what is left.
	if s.frameWanted && !s.ticking || len(s.waiting) > 0 {
		s.startTicking()
	}
}
