//go:build ios

// Package ios is NSGo's UIKit backend: one UIView per node, styled through
// its CALayer, with text the engine laid out with CoreText drawn on the
// main thread and images decoded off it. Alerts are UIAlertControllers.
// It calls UIKit through purego; Go itself needs cgo to link for iOS.
package ios

import (
	"runtime"
	"slices"
	"strings"
	"structs"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/engine"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
	"github.com/oneo-me/nsgo/internal/platform/queue"
	"github.com/oneo-me/nsgo/internal/testhost"
)

func init() {
	// UIKit runs on the process's main thread, which main starts on.
	runtime.LockOSThread()
}

// longPressDelay is how long a touch is held before it is a long press.
const longPressDelay = 500 * time.Millisecond

// view is the Go side of one native view.
type view struct {
	surface *surface
	id      driver.NodeID
	kind    driver.ViewKind
	native  objc.ID
	events  driver.EventMask
	// contentLayer draws a text's or a canvas's content.
	contentLayer objc.ID
	// axNodes are a canvas's named parts, its accessibility elements.
	axNodes []objc.ID
	// backdrop is the view's backdrop blur or Liquid Glass.
	backdrop objc.ID
	glass    bool

	pressing  atomic.Bool
	longFired atomic.Bool
	pressAt   driver.Event  // where the press began, and how
	overflow  driver.Bounds // where its children reach outside it (ItemOverflow)

	layout *apple.Layout // a text view's
	box    driver.Bounds

	sc    *scrollState // a scroll view's
	input *input       // an input's editor
	inset driver.Bounds

	image     *apple.ImageView
	modal     driver.ModalConfig // a modal's
	presented bool               // whether a modal was inserted yet
	sheet     *sheet             // a modal presented as a sheet
	bar       *navBar            // a screen's system header
	// headerWrap holds a header view in its screen's bar.
	headerWrap objc.ID
	drawing    *driver.Drawing    // a canvas's
	played     []driver.DrawOp    // the ops it last drew, its bound arguments' values in place
	shader     *apple.ShaderLayer // a shader view's (shader.go)
	web        *apple.Web         // a web view's
	video      *apple.Video       // a video's

	activation           driver.PanActivation
	handoff              bool    // its pan hands off from the scroll views inside it
	pan, pinch, rotation objc.ID // the recognizers of the gestures it follows
	hover                objc.ID // its UIHoverGestureRecognizer while it follows hovers

	// A text's content and selection, whether a view is an area, and the
	// long press that selects in it.
	plain            string
	selectable       driver.Selectable
	selColor         driver.Color
	selStart, selEnd int // UTF-16
	area             bool
	longPress        objc.ID

	// props are the view's last props; the rest what access.go last
	// gave UIKit.
	props               driver.ViewProps
	propsSet, textSet   bool
	axElement, axCustom bool

	// frame is where the view shows now; style its own style, which its
	// animated values and a layout animation draw over.
	frame    driver.Bounds
	framed   bool
	shift    driver.Point // a sticky cell's, from frame
	style    driver.ViewStyle
	animated driver.AnimatedValues

	component driver.NativeComponent // a native view's
	// componentView is the component's view, which fills the box: views
	// such as UITabBar size themselves, so it is placed at each frame
	// rather than autoresized.
	componentView  objc.ID
	componentTried bool
	accessory      driver.NodeID // an input's InputAccessoryView
}

type surface struct {
	// edgesDue are the scroll views whose bars get their edge
	// interactions at the end of the batch.
	edgesDue []*view
	// perf measures the surface while its panel, hud, shows; perfOn says
	// the app asked for them.
	perf   *engine.PerfMonitor
	hud    *hud
	perfOn bool
	// embedded is set for a host's NSGoView (embed.go): the host owns the
	// window and its view controllers. host is what it gives the app, and
	// embed the settings its view holds; stops end the surface's
	// observations as it closes.
	embedded bool
	host     engine.HostOptions
	embed    *embedState
	stops    []func()
	closed   bool
	// alerts are the alerts shown, by ID, kept for a test host to answer
	// them.
	alerts map[uint32]shownAlert
	// appearance is the one the app forces, or the system's.
	appearance driver.Appearance
	// stickyBound are the scroll views with sticky cells whose insets
	// follow animated values.
	stickyBound map[*view]bool
	app         engine.App
	id          string
	native      objc.ID
	engine      *engine.Engine
	batches     queue.Batches
	views       []*view // index is NodeID
	fingers     fingers // the touches reported
	env         driver.Environment

	keyboard driver.Points
	// keyboardScroll is the scroll view keeping the focused input clear
	// of the keyboard (keyboard.go).
	keyboardScroll   *view
	keyboardDuration time.Duration
	// keyboardStart is when the keyboard last started to move, on clock.
	keyboardStart time.Duration

	keys []driver.KeyBinding // the keymap's bindings
	// held is the binding a held key fires again, on a timer of its own:
	// UIKit reports a held key's press once.
	held        string
	heldStop    chan struct{}
	statusBar   int // the UIStatusBarStyle the controller asks for
	hideBar     bool
	barAnimated bool
	// headers are the navigation bar's heights; oriented the modals
	// that limit the orientations, in the order they were presented.
	headers  [2]driver.Points
	oriented []*view
	// tops is the screen each stack shows, as the core last said, slides
	// the stacks whose screens move, and changes the stacks a batch
	// changes the screens of (nav.go).
	tops    map[*view]*view
	slides  map[*view]*slide
	changes []stackChange

	// What a batch touched, finished once its frames are set.
	images, presenting []*view
	// shading are the shader views to draw once the batch or the frame
	// is applied.
	shading []*view

	selecting *selection

	// The frame clock and what it animates.
	animator *driver.Animator
	epoch    time.Time
	link     objc.ID // the CADisplayLink
	ticking  bool
	// frameWanted is set while the app waits for a display frame.
	frameWanted bool
	motions     map[*view]*motion
	ghosts      []*ghost
	components  driver.NativeComponents
	unknown     map[string]bool
	// accessories are the inputs each InputAccessoryView rides above.
	accessories map[driver.NodeID]*view
}

// edgeInsets is UIEdgeInsets.
type edgeInsets struct {
	_                        structs.HostLayout
	Top, Left, Bottom, Right float64
}

// views maps native objects to their Go side. Only the main thread uses it.
var views = map[objc.ID]*view{}

// runner is the surface a standalone app's scene shows (Run); UIKit makes
// the scene's delegate, so it cannot be handed one. Embedded surfaces are
// their hosts' views (NewView).
var runner *surface

var (
	sel = objc.RegisterName

	selAlloc, selNew                 = sel("alloc"), sel("new")
	selRelease                       = sel("release")
	selLayer                         = sel("layer")
	selSetFrame, selBounds           = sel("setFrame:"), sel("bounds")
	selAddSubview, selSuperview      = sel("addSubview:"), sel("superview")
	selInsertBelow                   = sel("insertSubview:belowSubview:")
	selRemoveFromSuperview           = sel("removeFromSuperview")
	selSetHidden                     = sel("setHidden:")
	selSetNeedsLayout                = sel("setNeedsLayout")
	selLocationInView                = sel("locationInView:")
	selAnyObject                     = sel("anyObject")
	selText, selSetText              = sel("text"), sel("setText:")
	selSetFont, selSetTextColor      = sel("setFont:"), sel("setTextColor:")
	selSetPlaceholder                = sel("setPlaceholder:")
	selSetTextAlignment              = sel("setTextAlignment:")
	selSetSecure                     = sel("setSecureTextEntry:")
	selSetKeyboardType               = sel("setKeyboardType:")
	selSetEnabled                    = sel("setEnabled:")
	selBecomeFirst, selResign        = sel("becomeFirstResponder"), sel("resignFirstResponder")
	selApply                         = sel("nsgoApply:")
	selChanged                       = sel("nsgoChanged:")
	selPerformOnMain                 = sel("performSelectorOnMainThread:withObject:waitUntilDone:")
	selSetIsAccessibilityElement     = sel("setIsAccessibilityElement:")
	selSetAccessibilityTraits        = sel("setAccessibilityTraits:")
	selSetAccessibilityValue         = sel("setAccessibilityValue:")
	selSetAccessibilityIdentifier    = sel("setAccessibilityIdentifier:")
	selSetAccessibilityContainerType = sel("setAccessibilityContainerType:")
	selIsEqualToString               = sel("isEqualToString:")
	selSetOpaque                     = sel("setOpaque:")
	selInitWithFrame, selSetDeleg    = sel("initWithFrame:"), sel("setDelegate:")
	selContentOffset                 = sel("contentOffset")
	selSetContentSize                = sel("setContentSize:")

	classView, classSurface, classInput, classScroll, classWatcher, classController objc.Class
	// surfaceMethods are the surface's, NsgoSurface's and NSGoView's.
	surfaceMethods []objc.MethodDef

	uiApplicationMain           func(argc int, argv unsafe.Pointer, principal, delegate uintptr) int
	uiReduceMotion, uiVoiceOver func() bool
	uiAccessibilityPost         func(notification uint32, argument objc.ID)
	announcement                uint32 // UIAccessibilityAnnouncementNotification
	traits                      = map[string]uint64{}
)

const uikitPath = "/System/Library/Frameworks/UIKit.framework/UIKit"

func loadUIKit() {
	uikit, err := purego.Dlopen(uikitPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&uiApplicationMain, uikit, "UIApplicationMain")
	purego.RegisterLibFunc(&uiReduceMotion, uikit, "UIAccessibilityIsReduceMotionEnabled")
	purego.RegisterLibFunc(&uiVoiceOver, uikit, "UIAccessibilityIsVoiceOverRunning")
	purego.RegisterLibFunc(&uiAccessibilityPost, uikit, "UIAccessibilityPostNotification")
	loadAccessibility(uikit)

	uiView := objc.GetClass("UIView")
	common := []objc.MethodDef{
		{Cmd: sel("hitTest:withEvent:"), Fn: hitTest},
		{Cmd: sel("touchesBegan:withEvent:"), Fn: touchesBegan},
		{Cmd: sel("touchesEnded:withEvent:"), Fn: touchesEnded},
		{Cmd: sel("touchesCancelled:withEvent:"), Fn: touchesCancelled},
	}
	common = append(common, accessMethods()...)
	common = append(common, gestureMethods()...)
	common = append(common, navMethods()...)
	registerGestures()
	registerTouches()
	registerNav()
	registerSelection()
	registerContent()
	registerCanvasNodes()
	classView = register("NsgoView", uiView, nil, common)
	surfaceMethods = append(common,
		objc.MethodDef{Cmd: sel("layoutSubviews"), Fn: surfaceLayout},
		objc.MethodDef{Cmd: selApply, Fn: surfaceApply},
		objc.MethodDef{Cmd: selTick, Fn: surfaceTick},
		objc.MethodDef{Cmd: sel("safeAreaInsetsDidChange"), Fn: func(self objc.ID, cmd objc.SEL) {
			apple.Super(self, cmd)
			if v := views[self]; v != nil {
				v.surface.refresh()
			}
		}},
		objc.MethodDef{Cmd: sel("traitCollectionDidChange:"), Fn: func(self objc.ID, cmd objc.SEL, previous objc.ID) {
			apple.Super(self, cmd, previous)
			if v := views[self]; v != nil {
				v.surface.refresh()
			}
		}},
	)
	classSurface = register("NsgoSurface", uiView, nil, surfaceMethods)
	classController = register("NsgoViewController", objc.GetClass("UIViewController"), nil, []objc.MethodDef{
		{Cmd: sel("preferredStatusBarStyle"), Fn: func(objc.ID, objc.SEL) int { return runner.statusBar }},
		{Cmd: sel("prefersStatusBarHidden"), Fn: func(objc.ID, objc.SEL) bool { return runner.hideBar }},
		{Cmd: sel("preferredStatusBarUpdateAnimation"), Fn: func(objc.ID, objc.SEL) int { return runner.barAnimation() }},
		{Cmd: sel("supportedInterfaceOrientations"), Fn: func(self objc.ID, cmd objc.SEL) uint {
			if m := runner.orientations(); m != 0 {
				return m
			}
			return apple.SendSuper[uint](self, cmd)
		}},
		{Cmd: sel("canBecomeFirstResponder"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("viewDidAppear:"), Fn: func(self objc.ID, cmd objc.SEL, animated bool) {
			apple.Super(self, cmd, animated)
			self.Send(selBecomeFirst)
		}},
		{Cmd: sel("pressesBegan:withEvent:"), Fn: pressesBegan},
		{Cmd: sel("pressesEnded:withEvent:"), Fn: pressesEnded},
		{Cmd: sel("pressesCancelled:withEvent:"), Fn: pressesEnded},
		{Cmd: sel("copy:"), Fn: controllerCopy},
		{Cmd: sel("selectAll:"), Fn: controllerSelectAll},
		{Cmd: sel("canPerformAction:withSender:"), Fn: controllerCanPerform},
	})
	// The watcher sees every touch going down in the surface, before its
	// views do, and lets it through.
	classWatcher = register("NsgoTouchWatcher", objc.GetClass("UIGestureRecognizer"), nil, []objc.MethodDef{
		{Cmd: sel("touchesBegan:withEvent:"), Fn: watcherBegan},
	})
	classInput = register("NsgoInput", uiView, []*objc.Protocol{objc.GetProtocol("UITextFieldDelegate"),
		objc.GetProtocol("UITextViewDelegate")}, append(common, inputMethods()...))
	registerInput()
	classScroll = register("NsgoScrollView", objc.GetClass("UIScrollView"),
		[]*objc.Protocol{objc.GetProtocol("UIScrollViewDelegate")},
		append(append(scrollMethods(), behaviorMethods()...), objc.MethodDef{Cmd: sel("scrollViewDidScroll:"), Fn: scrolled}))
	register("NsgoAppDelegate", objc.GetClass("UIResponder"),
		[]*objc.Protocol{objc.GetProtocol("UIApplicationDelegate")},
		[]objc.MethodDef{{Cmd: sel("application:didFinishLaunchingWithOptions:"),
			Fn: func(self objc.ID, _ objc.SEL, application, options objc.ID) bool { return true }}})
	// The SDK requires the scene life cycle; Info.plist names this class.
	register("NsgoSceneDelegate", objc.GetClass("UIResponder"),
		[]*objc.Protocol{objc.GetProtocol("UIWindowSceneDelegate")},
		[]objc.MethodDef{{Cmd: sel("scene:willConnectToSession:options:"), Fn: sceneWillConnect}})
}

func register(name string, super objc.Class, protocols []*objc.Protocol, methods []objc.MethodDef) objc.Class {
	return apple.RegisterClass(name, super, protocols, methods)
}

// Options configure Run.
type Options struct {
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
}

// Run shows app full screen and runs UIKit's event loop. Call it from
// main; it does not return.
func Run(root engine.App, opts Options) {
	runner = newSurface(root, opts)
	runUIKit(opts)
}

// newSurface is the surface the scene shows root in; a test host's has no
// root until the test opens one.
func newSurface(root engine.App, opts Options) *surface {
	s := &surface{app: root, id: opts.ID, epoch: time.Now(), motions: map[*view]*motion{}, tops: map[*view]*view{}, slides: map[*view]*slide{}, appearance: opts.Appearance,
		components: opts.Components, unknown: map[string]bool{}, accessories: map[driver.NodeID]*view{},
		perfOn: opts.Perf}
	s.animator = driver.NewAnimator(func(e driver.Event) { s.engine.Dispatch(e) })
	return s
}

// runUIKit runs UIKit's loop, which connects the scene, on this thread.
func runUIKit(opts Options) {
	loadUIKit()
	apple.RegisterFonts(opts.Fonts)
	argv := []*byte{&[]byte("nsgo\x00")[0], nil}
	uiApplicationMain(1, unsafe.Pointer(&argv[0]), 0, apple.CFString("NsgoAppDelegate"))
}

// window keeps a standalone app's key window alive; controller is its
// root view controller.
var window, controller objc.ID

func sceneWillConnect(self objc.ID, _ objc.SEL, scene, session, options objc.ID) {
	window = objc.ID(objc.GetClass("UIWindow")).Send(selAlloc).Send(sel("initWithWindowScene:"), scene)
	bounds := objc.Send[apple.CGRect](window, selBounds)
	controller = objc.ID(classController).Send(selNew)
	s := runner
	s.native = objc.ID(classSurface).Send(selAlloc).Send(selInitWithFrame, bounds)
	views[s.native] = &view{surface: s}
	controller.Send(sel("setView:"), s.native)
	window.Send(sel("setRootViewController:"), controller)
	setAppearance(s.appearance)
	window.Send(sel("makeKeyAndVisible"))
	s.watch()
	s.env = s.environment()
	if s.app == nil {
		// A test host's: the test opens its app.
		close(sceneReady)
		return
	}
	s.startEngine(nil)
}

// watch adds what follows the surface's touches: the watcher of presses
// outside views and the tracker of raw touches.
func (s *surface) watch() {
	watcher := objc.ID(classWatcher).Send(selAlloc).Send(sel("initWithTarget:action:"), objc.ID(0), objc.SEL(0))
	watcher.Send(sel("setCancelsTouchesInView:"), false)
	watcher.Send(sel("setDelaysTouchesEnded:"), false)
	s.native.Send(sel("addGestureRecognizer:"), watcher)
	watcher.Send(selRelease)
	s.addTracker()
	s.headers = headerHeights()
}

// sceneReady is closed once a test host's scene has its window.
var sceneReady chan struct{}

// startEngine starts s.app's engine at the surface's size; test, when
// set, is what a test host opens it with.
func (s *surface) startEngine(test *testOpen) {
	bounds := objc.Send[apple.CGRect](s.native, selBounds)
	opts := engine.Options{
		Size:        driver.Size{Width: driver.Points(bounds.Size.Width), Height: driver.Points(bounds.Size.Height)},
		Environment: s.env,
		Thread:      apple.RaiseThread,
		Appearance:  s.appearance,
		Host:        s.host,
	}
	opts.SetDirs(engine.AppDirs(s.id))
	if test != nil {
		size := opts.Size
		opts.UseTest(test.options, &test.pin)
		// A phone's screen is what it is.
		opts.Size = size
		s.alerts = map[uint32]shownAlert{}
	}
	imageload.UseDiskCache(opts.CacheDir)
	if s.perfOn && test == nil {
		s.perf = engine.NewPerfMonitor()
		opts.Perf = s.perf
		s.hud = newHud(s, s.perf)
		s.startTicking()
	}
	s.engine = engine.Start(s, s.app, opts)
	s.observe()
}

// testOpen is what a test host opens the app with: the test's options and
// the environment it pins.
type testOpen struct {
	options testhost.Options
	pin     engine.TestPin
}

// shownAlert is an alert a test may answer: its controller and actions.
type shownAlert struct {
	ctrl    objc.ID
	actions []driver.AlertAction
}

// environment returns what UIKit tells about where the app runs.
func (s *surface) environment() driver.Environment {
	in := objc.Send[edgeInsets](s.native, sel("safeAreaInsets"))
	traits := s.native.Send(sel("traitCollection"))
	scale := objc.Send[float64](traits, sel("displayScale"))
	if scale <= 0 {
		scale = objc.Send[float64](objc.ID(objc.GetClass("UIScreen")).Send(sel("mainScreen")), sel("scale"))
	}
	// The body text style is 17 points at the default text size.
	body := objc.ID(objc.GetClass("UIFont")).Send(sel("preferredFontForTextStyle:"),
		apple.Global(uikitPath, "UIFontTextStyleBody"))
	idiom := driver.IdiomPhone
	const pad, mac = 1, 5
	switch objc.Send[int](objc.ID(objc.GetClass("UIDevice")).Send(sel("currentDevice")), sel("userInterfaceIdiom")) {
	case pad:
		idiom = driver.IdiomTablet
	case mac:
		idiom = driver.IdiomMac
	}
	if onMac() {
		idiom = driver.IdiomMac
	}
	const dark = 2
	locale, rtl := apple.Locale()
	env := driver.Environment{
		SafeArea: driver.Edges{Top: driver.Points(in.Top), Right: driver.Points(in.Right),
			Bottom: driver.Points(in.Bottom), Left: driver.Points(in.Left)},
		KeyboardHeight:   s.keyboard,
		KeyboardDuration: s.keyboardDuration,
		Dark:             objc.Send[int](traits, sel("userInterfaceStyle")) == dark,
		FontScale:        float32(objc.Send[float64](body, sel("pointSize")) / 17),
		ReduceMotion:     uiReduceMotion(),
		ScreenReader:     uiVoiceOver(),
		RTL:              rtl,
		PixelRatio:       float32(scale),
		Locale:           locale,
		OS:               driver.OSIOS,
		OSVersion:        osVersion(),
		Idiom:            idiom,
	}
	env.HeaderHeight, env.LargeHeaderHeight = s.headers[0], s.headers[1]
	return env
}

// refresh reports the environment to the engine if it changed.
func (s *surface) refresh() {
	if s == nil || s.engine == nil {
		return
	}
	if env := s.environment(); env != s.env {
		s.env = env
		s.engine.SetEnvironment(env)
	}
}

// observe watches the notifications parts of the environment change with,
// until the surface closes.
func (s *surface) observe() {
	center := objc.ID(objc.GetClass("NSNotificationCenter")).Send(sel("defaultCenter"))
	refresh := func(objc.ID) { s.refresh() }
	on := func(name objc.ID, fn func(objc.ID)) { s.stops = append(s.stops, apple.Observe(center, name, fn)) }
	for _, name := range []string{"UIContentSizeCategoryDidChangeNotification",
		"UIAccessibilityReduceMotionStatusDidChangeNotification", "UIAccessibilityVoiceOverStatusDidChangeNotification"} {
		on(apple.Global(uikitPath, name), refresh)
	}
	on(apple.Global("/System/Library/Frameworks/Foundation.framework/Foundation", "NSCurrentLocaleDidChangeNotification"), refresh)
	on(apple.Global(uikitPath, "UIKeyboardWillChangeFrameNotification"), func(note objc.ID) { s.keyboardMoved(note, false) })
	on(apple.Global(uikitPath, "UIKeyboardWillHideNotification"), func(note objc.ID) { s.keyboardMoved(note, true) })
}

// uiWindow is the window the surface shows in, 0 while it is in none.
func (s *surface) uiWindow() objc.ID { return s.native.Send(sel("window")) }

// viewController is the view controller the surface's view belongs to:
// a standalone app's own, or the host's nearest above an embedded view.
func (s *surface) viewController() objc.ID {
	if !s.embedded {
		return controller
	}
	for r := s.native.Send(sel("nextResponder")); r != 0; r = r.Send(sel("nextResponder")) {
		if objc.Send[bool](r, sel("isKindOfClass:"), objc.GetClass("UIViewController")) {
			return r
		}
	}
	return 0
}

// topController returns the view controller on top of the surface's
// window's.
func (s *surface) topController() objc.ID {
	top := s.uiWindow().Send(sel("rootViewController"))
	for p := top.Send(sel("presentedViewController")); p != 0; p = top.Send(sel("presentedViewController")) {
		top = p
	}
	return top
}

// keyboardMoved follows the software keyboard: how much of the surface's
// bottom it will cover, and how long it takes to get there.
func (s *surface) keyboardMoved(note objc.ID, hiding bool) {
	info := note.Send(sel("userInfo"))
	get := func(key string) objc.ID { return info.Send(sel("objectForKey:"), apple.Global(uikitPath, key)) }
	seconds := objc.Send[float64](get("UIKeyboardAnimationDurationUserInfoKey"), sel("doubleValue"))
	s.keyboardDuration = time.Duration(seconds * float64(time.Second))
	s.keyboardStart = s.clock()
	s.keyboard = 0
	if frame := get("UIKeyboardFrameEndUserInfoKey"); !hiding && frame != 0 {
		end := objc.Send[apple.CGRect](frame, sel("CGRectValue"))
		space := objc.ID(objc.GetClass("UIScreen")).Send(sel("mainScreen")).Send(sel("coordinateSpace"))
		if w := s.uiWindow(); w != 0 {
			space = w.Send(sel("screen")).Send(sel("coordinateSpace"))
		}
		local := objc.Send[apple.CGRect](s.native, sel("convertRect:fromCoordinateSpace:"), end, space)
		h := objc.Send[apple.CGRect](s.native, selBounds).Size.Height - local.Origin.Y
		s.keyboard = driver.Points(max(h, 0))
	}
	s.refresh()
	s.keepFocusedClear()
}

// alert presents a over whatever the window shows; the action chosen goes
// to the engine as EventAlert.
func (s *surface) alert(a *driver.AlertConfig) {
	const styleSheet, styleAlert = 0, 1
	style := styleAlert
	if a.Sheet {
		style = styleSheet
	}
	ctrl := objc.ID(objc.GetClass("UIAlertController")).Send(sel("alertControllerWithTitle:message:preferredStyle:"),
		apple.NSString(a.Title), apple.NSString(a.Message), style)
	id := a.ID
	add := func(label string, style, index int) {
		handler := objc.NewBlock(func(_ objc.Block, _ objc.ID) {
			if s.alerts != nil {
				delete(s.alerts, id)
			}
			s.engine.Dispatch(driver.Event{Kind: driver.EventAlert, Count: id, Index: index})
		})
		action := objc.ID(objc.GetClass("UIAlertAction")).Send(sel("actionWithTitle:style:handler:"),
			apple.NSString(label), style, handler)
		handler.Release()
		ctrl.Send(sel("addAction:"), action)
	}
	const actionDefault, actionCancel, actionDestructive = 0, 1, 2
	cancel := false
	for i, act := range a.Actions {
		style := actionDefault
		switch act.Role {
		case driver.AlertCancel:
			// UIKit allows one cancel action.
			if !cancel {
				style, cancel = actionCancel, true
			}
		case driver.AlertDestructive:
			style = actionDestructive
		}
		add(act.Label, style, i)
	}
	if len(a.Actions) == 0 {
		add("OK", actionCancel, -1)
	}
	if pop := ctrl.Send(sel("popoverPresentationController")); pop != 0 {
		// iPad: an action sheet is a popover, anchored at the bottom
		// center without an arrow. It hides its cancel action and runs it
		// on a tap outside, so one that is never shown closes it.
		if !cancel {
			handler := objc.NewBlock(func(_ objc.Block, _ objc.ID) {
				s.engine.Dispatch(driver.Event{Kind: driver.EventAlert, Count: id, Index: -1})
			})
			action := objc.ID(objc.GetClass("UIAlertAction")).Send(sel("actionWithTitle:style:handler:"),
				objc.ID(0), actionCancel, handler)
			handler.Release()
			ctrl.Send(sel("addAction:"), action)
		}
		b := objc.Send[apple.CGRect](s.native, selBounds)
		pop.Send(sel("setSourceView:"), s.native)
		pop.Send(sel("setSourceRect:"), apple.Rect(b.Size.Width/2, b.Size.Height, 0, 0))
		pop.Send(sel("setPermittedArrowDirections:"), 0)
	}
	top := s.topController()
	if s.alerts != nil {
		s.alerts[id] = shownAlert{ctrl, a.Actions}
	}
	top.Send(sel("presentViewController:animated:completion:"), ctrl, true, objc.ID(0))
}

// pressOutside reports a touch going down to each view listening for
// presses outside it that holds neither the view touched nor the point.
func (s *surface) pressOutside(touch objc.ID) {
	hit := touch.Send(sel("view"))
	var at driver.Point
	found := false
	for _, v := range s.views {
		if v == nil || !v.events.Has(driver.EventPressOutside) || v.native.Send(sel("window")) == 0 {
			continue
		}
		if !found {
			p := objc.Send[apple.CGPoint](touch, selLocationInView, s.native)
			at, found = driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}, true
		}
		p := objc.Send[apple.CGPoint](touch, selLocationInView, v.native)
		b := objc.Send[apple.CGRect](v.native, selBounds)
		inside := p.X >= 0 && p.Y >= 0 && p.X < b.Size.Width && p.Y < b.Size.Height
		if inside || (hit != 0 && objc.Send[bool](hit, sel("isDescendantOfView:"), v.native)) {
			continue
		}
		s.engine.Dispatch(driver.Event{Kind: driver.EventPressOutside, ID: v.id, Point: at})
	}
}

func watcherBegan(self objc.ID, cmd objc.SEL, touches, event objc.ID) {
	apple.Super(self, cmd, touches, event)
	if v := views[self.Send(sel("view"))]; v != nil && v.surface.engine != nil {
		s := v.surface
		apple.ForEach(touches, s.pressOutside)
		apple.ForEach(touches, s.clearTouched)
	}
	// Failing at once keeps the watcher out of every other gesture.
	const failed = 5
	self.Send(sel("setState:"), failed)
}

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
			s.native.Send(sel("endEditing:"), true)
			continue
		case driver.ItemSetClipboard:
			objc.ID(objc.GetClass("UIPasteboard")).Send(sel("generalPasteboard")).Send(sel("setString:"), apple.NSString(it.Value))
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
		case driver.ItemAppearance:
			s.appearance = it.Appearance
			if s.embedded {
				s.embedStyle()
			} else {
				setAppearance(it.Appearance)
			}
			continue
		case driver.ItemQuit, driver.ItemPopupMenu:
			// iOS has no toasts, apps do not quit themselves, and the core
			// shows phones no native menus.
			continue
		case driver.ItemImageInfo:
			apple.ImageInfo(it, s.engine.Dispatch)
			continue
		case driver.ItemAnnounce:
			uiAccessibilityPost(announcement, apple.NSString(it.Value))
			continue
		case driver.ItemBlockNative:
			var v *view
			if int(it.ID) < len(s.views) {
				v = s.views[it.ID]
			}
			s.blockNative(v)
			continue
		case driver.ItemKeymap:
			s.keys = it.Keys
			continue
		case driver.ItemStatusBar:
			s.setStatusBar(it.StatusBar)
			continue
		case driver.ItemTitle, driver.ItemBackUnhandled, driver.ItemBackPreview, driver.ItemHistory, driver.ItemWindow, driver.ItemMenu:
			continue
		}
		v := s.views[it.ID]
		switch it.Kind {
		case driver.ItemDelete:
			if v.bar != nil {
				s.touchStack(s.stackOf(v))
			}
			s.delete(v, s.vanish(v, lb))
		case driver.ItemRemove:
			if v.bar != nil {
				s.touchStack(s.stackOf(v))
			}
			if v.sheet != nil {
				s.dismissSheet(v)
				v.presented = false
				s.updateOrientations()
			}
			v.native.Send(selRemoveFromSuperview)
		case driver.ItemInsert:
			if v.kind == driver.KindModal && v.isSheet() {
				// A sheet holds the modal's view, wherever the core puts it.
				if v.sheet == nil {
					v.presented = true
					s.presentSheet(v)
					s.updateOrientations()
				}
				break
			}
			if v.bar != nil {
				s.touchStack(s.stackOf(v))
				s.touchStack(s.views[it.Parent])
			}
			parent := s.views[it.Parent].children()
			if it.Before == 0 {
				parent.Send(selAddSubview, v.native)
			} else {
				parent.Send(selInsertBelow, v.native, s.views[it.Before].native)
			}
			if v.kind == driver.KindModal && !v.presented {
				v.presented = true
				s.presenting = append(s.presenting, v)
				s.updateOrientations()
			}
			if p := s.views[it.Parent]; p.bar != nil {
				// The header stays over the screen's content.
				parent.Send(sel("bringSubviewToFront:"), p.bar.bar)
			}
			s.appear(v, lb, it.Parent)
		case driver.ItemOverflow:
			v.overflow = it.Frame
		case driver.ItemFrame:
			s.setFrame(v, it.Frame, lb)
			if v.sc != nil && v.hasEdges() {
				v.placeEdges()
			}
			if v.bar != nil {
				s.layoutBar(v)
			}
			if v.headerWrap != 0 {
				s.sizeHeaderWrap(v)
			}
		case driver.ItemScreen:
			s.configureScreen(v, it.Screen)
		case driver.ItemViewStyle:
			if v.bar != nil && it.ViewStyle.Hidden != v.style.Hidden {
				s.touchStack(s.stackOf(v))
			}
			v.style = *it.ViewStyle
			if v.animatesStyle() {
				v.restyle()
			} else {
				apple.StyleLayer(v.native.Send(selLayer), it.ViewStyle, 0)
			}
			v.setBackdrop(it.ViewStyle)
			v.native.Send(selSetHidden, it.ViewStyle.Hidden)
			if v.kind == driver.KindScroll || v.kind == driver.KindVideo {
				// A scroll view shows only what it scrolls to, and a video
				// only its frame.
				v.native.Send(sel("setClipsToBounds:"), true)
			}
			if v.image != nil {
				v.image.SetStyle(it.ViewStyle)
			}
		case driver.ItemImage:
			v.image.Configure(it.Image)
			s.images = append(s.images, v)
		case driver.ItemModal:
			v.modal = *it.Modal
			if v.modal.Orientations != 0 {
				s.oriented = append(s.oriented, v)
			}
		case driver.ItemWeb:
			v.web.Configure(it.Web)
		case driver.ItemWebCommand:
			v.web.Command(it.WebCommand, it.Value)
		case driver.ItemVideo:
			v.video.Configure(it.Video)
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
			s.animator.BindCanvas(v.id, it.Drawing.Slots)
			v.setCanvasNodes(it.Drawing.Nodes)
			v.redraw()
		case driver.ItemShader:
			s.setShader(v, it.Shader)
		case driver.ItemScrollTo:
			size := objc.Send[apple.CGSize](v.native, sel("contentSize"))
			in := objc.Send[edgeInsets](v.native, sel("adjustedContentInset"))
			seen := objc.Send[apple.CGRect](v.native, selBounds).Size
			to := apple.CGPoint{
				X: min(max(float64(it.Point.X), -in.Left), max(size.Width-seen.Width+in.Right, -in.Left)),
				Y: min(max(float64(it.Point.Y), -in.Top), max(size.Height-seen.Height+in.Bottom, -in.Top)),
			}
			if v.sc != nil {
				v.sc.applied++
			}
			v.native.Send(sel("setContentOffset:animated:"), to, it.Animated)
		case driver.ItemScrollBy:
			if v.sc != nil {
				v.sc.applied++
				v.scrollBy(it.Point)
			}
		case driver.ItemStickyCells:
			if v.sc != nil {
				v.setSticky(it.Sticky)
			}
		case driver.ItemViewProps:
			v.setProps(it.ViewProps)
			p := it.ViewProps
			v.activation, v.handoff, v.area, v.selectable = p.PanActivation, p.PanScrollHandoff, p.SelectionArea, p.Selectable
			if v.selColor != p.SelectionColor {
				v.selColor = p.SelectionColor
				v.redraw()
			}
			v.setSelectable()
		case driver.ItemEvents:
			v.events = it.Events
			if v.input != nil {
				v.input.fs.Events = it.Events
			}
			v.setGestures(it.Events)
			v.updateAccessibility()
		case driver.ItemAccessibilityLabel:
			v.setLabel(it.Value)
		case driver.ItemAccessibilityFocus:
			v.focusAccessibility()
		case driver.ItemText:
			v.layout.Release()
			v.layout, _ = it.Text.Layout.(*apple.Layout)
			v.box = it.Text.Content
			v.sizeContent()
			v.textChanged(it.Text.Text)
			v.redraw()
		case driver.ItemInput:
			v.configure(it.Input)
			s.setAccessory(v, it.Input.Accessory)
		case driver.ItemInputValue:
			v.input.value(it.Value, it.Count)
		case driver.ItemSelection:
			if v.input != nil {
				v.input.setSelection(it.Selection)
			}
		case driver.ItemScrollConfig:
			if v.sc != nil {
				v.configureScroll(it.Scroll)
			}
		case driver.ItemScrollContent:
			if v.sc != nil {
				v.setContent(it.Size)
			}
		case driver.ItemZoomTo:
			v.zoomTo(it.Zoom, it.Animated)
		case driver.ItemFocus:
			if v.input != nil {
				v.input.field.Send(selBecomeFirst)
			}
		case driver.ItemBlur:
			if v.input != nil {
				v.input.field.Send(selResign)
			}
		}
	}
	s.attachEdgeBars()
	if root := s.views[b.Root]; root.native.Send(selSuperview) != s.native {
		s.native.Send(selAddSubview, root.native)
	}
	s.slideStacks()
	s.finish()
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
			apple.AnimateIn(v.native.Send(selLayer), v.modal.Animation, h, s.env.ReduceMotion)
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
		newScroll(v)
	case driver.KindInput:
		v.newInput()
	default:
		v.native = objc.ID(classView).Send(selNew)
	}
	v.native.Send(selSetOpaque, false)
	switch kind {
	case driver.KindText, driver.KindCanvas:
		v.newContent()
	case driver.KindImage:
		v.image = apple.NewImageView(v.native.Send(selLayer))
		v.image.Dispatch = v.report
	case driver.KindWeb:
		v.web = apple.NewWeb(v.native)
		v.web.Dispatch = v.report
	case driver.KindVideo:
		v.video = apple.NewVideo(v.native)
		v.video.Dispatch = v.report
		v.video.Parent = s.viewController()
		v.native.Send(sel("setClipsToBounds:"), true)
	case driver.KindShader:
		v.newShader()
	}
	views[v.native] = v
	return v
}

// delete releases v; kept leaves its native where it is, going away.
func (s *surface) delete(v *view, kept bool) {
	v.releaseCanvasNodes()
	if v.sc != nil {
		for i := range v.sc.edges {
			if ev := &v.sc.edges[i]; ev.holder != 0 {
				ev.paint.Release()
				ev.mask.Send(selRelease)
			}
		}
	}
	if v.sheet != nil {
		s.dismissSheet(v)
		defer s.updateOrientations()
	}
	if sl := s.slides[v]; sl != nil {
		s.endSlide(sl)
	}
	delete(s.tops, v)
	if b := v.bar; b != nil {
		delete(bars, b.bar)
		b.bar.Send(selRelease)
		b.item.Send(selRelease)
		b.back.Send(selRelease)
		if b.system != 0 {
			b.system.Send(selRelease)
		}
	}
	if v.headerWrap != 0 {
		v.headerWrap.Send(selRemoveFromSuperview)
		v.headerWrap.Send(selRelease)
	}
	if !kept && !s.inGhost(v.native) {
		v.native.Send(selRemoveFromSuperview)
	}
	s.animator.Forget(v.id)
	delete(s.motions, v)
	if v.component != nil {
		v.component.Close()
		v.component = nil
	}
	if in := s.accessories[v.id]; in != nil {
		s.setAccessory(in, 0)
	}
	if v.accessory != 0 {
		s.setAccessory(v, 0)
	}
	apple.ForgetTransform(v.native.Send(selLayer))
	apple.ForgetDecor(v.native.Send(selLayer))
	v.forgetContent()
	delete(gates, v.pan)
	v.layout.Release()
	if v.image != nil {
		v.image.Close()
	}
	if v.web != nil {
		v.web.Close()
	}
	if v.video != nil {
		v.video.Close()
	}
	if v.shader != nil {
		v.shader.Release()
	}
	delete(views, v.native)
	v.native.Send(selRelease)
	s.views[v.id] = nil
}

func (v *view) dispatch(kind driver.EventKind) {
	v.report(driver.Event{Kind: kind})
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

// pointer returns where touch is in v and in v's root view, the modal
// around it or the surface, and the modifier keys event holds.
func (v *view) pointer(kind driver.EventKind, touch, event objc.ID) driver.Event {
	root := v.surface.native
	for n := v.native; n != 0 && n != root; n = n.Send(selSuperview) {
		if m := views[n]; m != nil && m.native == n && m.kind == driver.KindModal {
			root = n
			break
		}
	}
	p := objc.Send[apple.CGPoint](touch, selLocationInView, v.native)
	page := objc.Send[apple.CGPoint](touch, selLocationInView, root)
	return driver.Event{Kind: kind, Point: driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)},
		Page:      driver.Point{X: driver.Points(page.X), Y: driver.Points(page.Y)},
		Modifiers: apple.Modifiers(objc.Send[uint](event, sel("modifierFlags")))}
}

// pressed reports kind about the press v follows, with where it began.
func (v *view) pressed(kind driver.EventKind) {
	e := v.pressAt
	e.Kind = kind
	v.report(e)
}

// hitTest is UIKit's hit test of self at p, in its own coordinates, by
// its pointer events and hit slop, and looking in its children where
// they reach outside it, as core.Reach says.
func hitTest(self objc.ID, cmd objc.SEL, p apple.CGPoint, event objc.ID) objc.ID {
	v := views[self]
	if v == nil || v.native != self {
		return apple.SendSuper[objc.ID](self, cmd, p, event)
	}
	b := objc.Send[apple.CGRect](self, selBounds)
	frame := driver.Bounds{Size: driver.Size{Width: driver.Points(b.Size.Width), Height: driver.Points(b.Size.Height)}}
	at := driver.Point{X: driver.Points(p.X - b.Origin.X), Y: driver.Points(p.Y - b.Origin.Y)}
	own, children := driver.Reach(frame, &v.props, v.overflow, at)
	if !frame.Contains(at) {
		if objc.Send[bool](self, sel("isHidden")) || !objc.Send[bool](self, sel("isUserInteractionEnabled")) {
			return 0
		}
		if children {
			subs := self.Send(sel("subviews"))
			for i := objc.Send[int](subs, sel("count")) - 1; i >= 0; i-- {
				sub := subs.Send(sel("objectAtIndex:"), i)
				local := objc.Send[apple.CGPoint](self, sel("convertPoint:toView:"), p, sub)
				if hit := sub.Send(cmd, local, event); hit != 0 {
					return hit
				}
			}
		}
		if own {
			return self
		}
		return 0
	}
	hit := apple.SendSuper[objc.ID](self, cmd, p, event)
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

// longPressDelay is how long a touch on v is held to be a long press.
func (v *view) longPressDelay() time.Duration {
	if d := v.props.DelayLongPress; d > 0 {
		return d
	}
	return longPressDelay
}

func (v *view) pressable() bool {
	return v.events.Has(driver.EventPress) || v.events.Has(driver.EventLongPress) ||
		v.events.Has(driver.EventPressDown) || v.events.Has(driver.EventPressUp)
}

// Touches a view does not handle go up the responder chain to a pressable
// ancestor.
func touchesBegan(self objc.ID, cmd objc.SEL, touches, event objc.ID) {
	v := views[self]
	// A secondary click of a pointer opens a context menu.
	const secondary = 1 << 1
	if v != nil && v.events.Has(driver.EventContextMenu) && objc.Send[uint](event, sel("buttonMask"))&secondary != 0 {
		v.report(v.pointer(driver.EventContextMenu, touches.Send(selAnyObject), event))
		return
	}
	if v == nil || !v.pressable() {
		apple.Super(self, cmd, touches, event)
		return
	}
	v.pressing.Store(true)
	v.longFired.Store(false)
	touch := touches.Send(selAnyObject)
	v.pressAt = v.pointer(driver.EventPressDown, touch, event)
	v.pressAt.Count = uint32(max(objc.Send[int](touch, sel("tapCount")), 1))
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

func touchesCancelled(self objc.ID, cmd objc.SEL, touches, event objc.ID) {
	v := views[self]
	if v == nil || !v.pressable() {
		apple.Super(self, cmd, touches, event)
		return
	}
	if v.pressing.Swap(false) {
		v.pressed(driver.EventPressUp)
	}
}

func touchesEnded(self objc.ID, cmd objc.SEL, touches, event objc.ID) {
	v := views[self]
	if v == nil || !v.pressable() {
		apple.Super(self, cmd, touches, event)
		return
	}
	p := objc.Send[apple.CGPoint](touches.Send(selAnyObject), selLocationInView, self)
	b := objc.Send[apple.CGRect](self, selBounds)
	pressed := v.pressing.Swap(false)
	v.pressed(driver.EventPressUp)
	if pressed && !v.longFired.Load() && v.within(p, b) {
		index := -1
		if v.layout != nil {
			box := v.box
			index = v.layout.IndexAt(p, apple.Rect(float64(box.Origin.X), float64(box.Origin.Y),
				float64(box.Size.Width), float64(box.Size.Height)))
		}
		e := v.pressAt
		e.Kind, e.Index = driver.EventPress, index
		v.report(e)
	}
}

// UIKeyboardHIDUsage codes of the keys core.NormalizeKey names.
var keyNames = map[int]string{
	40: "enter", 88: "enter", 41: "escape", 43: "tab", 44: "space", 42: "backspace", 76: "delete",
	82: "up", 81: "down", 80: "left", 79: "right", 74: "home", 77: "end", 75: "pageup", 78: "pagedown",
	58: "f1", 59: "f2", 60: "f3", 61: "f4", 62: "f5", 63: "f6", 64: "f7", 65: "f8", 66: "f9",
	67: "f10", 68: "f11", 69: "f12",
}

// pressesBegan takes hardware key presses the keymap binds, as EventKey,
// and passes the rest on.
func pressesBegan(self objc.ID, cmd objc.SEL, presses, event objc.ID) {
	app := pressSurface(self)
	binding, consumed := "", false
	if app != nil && app.engine != nil && len(app.keys) > 0 {
		apple.ForEach(presses, func(press objc.ID) {
			key := press.Send(sel("key"))
			if key == 0 || consumed {
				return
			}
			name, ok := keyNames[objc.Send[int](key, sel("keyCode"))]
			if !ok {
				name = strings.ToLower(apple.GoString(key.Send(sel("charactersIgnoringModifiers"))))
			}
			if name != "" {
				binding, consumed = driver.FindKey(app.keys, apple.KeyNames(name, objc.Send[uint](key, sel("modifierFlags"))),
					app.editing(), false)
			}
		})
	}
	if !consumed {
		apple.Super(self, cmd, presses, event)
		return
	}
	app.engine.Dispatch(driver.Event{Kind: driver.EventKey, Text: binding})
	if i := slices.IndexFunc(app.keys, func(b driver.KeyBinding) bool { return b.Key == binding }); i >= 0 && app.keys[i].Repeat {
		app.repeatKey(binding)
	}
}

// pressSurface is the surface hardware keys pressed in self go to: an
// embedded view's, or a standalone app's from its controller.
func pressSurface(self objc.ID) *surface {
	if v := views[self]; v != nil {
		return v.surface
	}
	return runner
}

// pressesEnded stops repeating a held key's binding.
func pressesEnded(self objc.ID, cmd objc.SEL, presses, event objc.ID) {
	if app := pressSurface(self); app != nil && app.heldStop != nil {
		close(app.heldStop)
		app.heldStop, app.held = nil, ""
	}
	apple.Super(self, cmd, presses, event)
}

// repeatKey fires binding again while its key is held, as a hardware
// keyboard's auto-repeat would: after 500 ms, then 30 times a second.
func (s *surface) repeatKey(binding string) {
	if s.heldStop != nil {
		close(s.heldStop)
	}
	stop := make(chan struct{})
	s.held, s.heldStop = binding, stop
	e := s.engine
	go func() {
		t := time.NewTimer(500 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				e.Dispatch(driver.Event{Kind: driver.EventKey, Text: binding})
				t.Reset(33 * time.Millisecond)
			}
		}
	}()
}

// editing reports whether a text input has the keyboard focus.
func (s *surface) editing() bool {
	for _, v := range s.views {
		if v != nil && v.input != nil && v.input.focused {
			return true
		}
	}
	return false
}

func surfaceLayout(self objc.ID, cmd objc.SEL) {
	apple.Super(self, cmd)
	if v := views[self]; v != nil && v.surface.engine != nil {
		b := objc.Send[apple.CGRect](self, selBounds)
		v.surface.engine.Resize(driver.Size{Width: driver.Points(b.Size.Width), Height: driver.Points(b.Size.Height)})
		if h := v.surface.hud; h != nil {
			h.layout()
		}
	}
}

func surfaceApply(self objc.ID, _ objc.SEL, _ objc.ID) {
	s := views[self].surface
	done := s.measuring(true)
	for _, b := range s.batches.Drain() {
		if s.perf != nil && !b.Sent.IsZero() {
			s.perf.RecordLatency(time.Since(b.Sent))
		}
		s.apply(b)
	}
	done()
	if s.hud != nil {
		s.native.Send(sel("bringSubviewToFront:"), s.hud.panel)
	}
	// A frame requested before the view was in a window starts the clock
	// here: a display link needs the view mounted.
	if s.frameWanted && !s.ticking {
		s.startTicking()
	}
}

func scrolled(self objc.ID, _ objc.SEL, _ objc.ID) {
	v := views[self]
	if v == nil {
		return
	}
	apple.FollowBounds(self.Send(selLayer))
	// Pinning it scrolls it again, which reports the pinned offset.
	if at := objc.Send[apple.CGPoint](self, selContentOffset); pinTop(self) && objc.Send[apple.CGPoint](self, selContentOffset) != at {
		return
	}
	o := objc.Send[apple.CGPoint](self, selContentOffset)
	offset := driver.Point{X: driver.Points(o.X), Y: driver.Points(o.Y)}
	scrolled := v.surface.animator.Scroll(v.id, offset)
	if v.sc != nil && v.surface.animator.Pulls(v.id) && v.surface.animator.Pull(v.id, v.pulled()) {
		scrolled = true
	}
	if scrolled {
		apple.WithoutAnimation(v.surface.applyAnimated)
	}
	if v.sc != nil && len(v.sc.sticky) > 0 {
		v.pinSticky()
	}
	if v.sc != nil && v.hasEdges() {
		v.followEdges()
	}
	v.surface.followLargeTitle(v, offset.Y)
	v.scrollEvent(driver.EventScroll)
}
