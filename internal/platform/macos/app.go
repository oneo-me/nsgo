//go:build darwin && !ios

package macos

import (
	"log"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/engine"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

// The app's own surface and the surfaces it opens. Run starts the app on
// a surface in no window, own, which shows no view: the app opens its
// windows, panels and tray views (driver.ItemOpenSurface), and own makes
// each a surface of its own, attached to the engine, as a window, an
// NSPanel or a view in a status item's button.

// own is the app's own surface while Run runs; perf is the performance
// monitor its first window shows.
var (
	own     *surface
	ownPerf *engine.PerfMonitor
)

// NSApplication's activation policies.
const policyRegular, policyAccessory = 0, 1

// startOwn starts app's engine on the app's own surface.
func startOwn(app engine.App, opts Options) *surface {
	o := opts
	o.Width, o.Height = 0, 0
	s := newSurface(o, false)
	s.opts = opts
	own = s
	s.env = s.environment()
	if opts.Perf {
		ownPerf = engine.NewPerfMonitor()
	}
	eopts := engine.Options{Environment: s.env, Perf: ownPerf, Thread: apple.RaiseThread,
		Appearance: opts.Appearance, Host: opts.Host,
		Windows: &engine.WindowOptions{Title: opts.Title, Width: opts.Width, Height: opts.Height, Frame: opts.Frame,
			Material: opts.Material, Constraints: opts.Constraints}}
	eopts.SetDirs(engine.AppDirs(opts.ID))
	imageload.UseDiskCache(eopts.CacheDir)
	s.engine = engine.Start(s, app, eopts)
	observeApp()
	return s
}

// windowSurface returns the surface whose window is w.
func windowSurface(w objc.ID) *surface {
	for s := range surfaces {
		if s.window == w && !s.embedded {
			return s
		}
	}
	return nil
}

// classPanel is the class of panels: borderless windows that take the
// keyboard without activating the app.
var classPanel objc.Class

// Window style masks and levels.
const (
	styleTitled, styleClosable, styleMiniaturizable, styleResizable = 1, 2, 4, 8
	styleNonactivating                                              = 1 << 7
	levelStatus                                                     = 25
	joinsAllSpaces, fullScreenAuxiliary                             = 1, 1 << 8
)

// openSurface opens the surface cfg asks for and attaches it to the
// app's engine.
func (o *surface) openSurface(cfg driver.SurfaceConfig) {
	opts := o.opts
	opts.Width, opts.Height, opts.Material = cfg.Width, cfg.Height, cfg.Material
	if cfg.Kind == driver.SurfaceTrayIcon {
		s := newSurface(opts, true)
		s.id, s.kind, s.tray = cfg.ID, cfg.Kind, cfg.Tray
		s.attach(o.engine)
		if tr := trays[cfg.Tray]; tr != nil {
			tr.show(s)
		}
		return
	}
	var window objc.ID
	if cfg.Kind == driver.SurfacePanel {
		window = newPanel(cfg)
	} else {
		window = objc.ID(classWindow).Send(selAlloc).Send(sel("initWithContentRect:styleMask:backing:defer:"),
			apple.Rect(0, 0, float64(cfg.Width), float64(cfg.Height)),
			uint(styleTitled|styleClosable|styleMiniaturizable|styleResizable), uint(2), false)
		title := cfg.Title
		if title == "" {
			title = opts.Title
		}
		window.Send(sel("setTitle:"), apple.NSString(title))
		// Tab goes through the fields and the views that take presses.
		window.Send(sel("setAutorecalculatesKeyViewLoop:"), true)
	}
	window.Send(sel("setReleasedWhenClosed:"), false)
	s := newSurface(opts, false)
	s.id, s.kind, s.hideOnBlur = cfg.ID, cfg.Kind, cfg.HideOnBlur
	s.window = window
	s.setWindowDelegate()
	window.Send(sel("setContentView:"), s.native)
	if cfg.Kind == driver.SurfacePanel {
		// A panel's content is clipped to its rounded corners.
		s.native.Send(selLayer).Send(sel("setCornerRadius:"), float64(12))
		s.native.Send(selLayer).Send(sel("setMasksToBounds:"), true)
	}
	s.setMaterial(cfg.Material)
	s.attach(o.engine)
	if cfg.Kind == driver.SurfaceWindow {
		s.setConstraints(cfg.Constraints)
		if cfg.Frame.Move {
			s.openFrame = cfg.Frame.OnScreen(usableScreens())
			s.setWindowFrame(s.openFrame)
		} else {
			window.Send(sel("center"))
		}
	}
	if !cfg.Hidden {
		if cfg.Kind == driver.SurfacePanel {
			s.showPanel(0)
		} else {
			s.orderFront()
		}
	}
}

// newPanel returns a panel of cfg's size: titled for its rounded corners
// and shadow, with no title bar or controls, above the windows of every
// space.
func newPanel(cfg driver.SurfaceConfig) objc.ID {
	if classPanel == 0 {
		yes := func(objc.ID, objc.SEL) bool { return true }
		classPanel = register("NsgoPanel", objc.GetClass("NSPanel"), nil, []objc.MethodDef{
			{Cmd: sel("canBecomeKeyWindow"), Fn: yes},
		})
	}
	p := objc.ID(classPanel).Send(selAlloc).Send(sel("initWithContentRect:styleMask:backing:defer:"),
		apple.Rect(0, 0, float64(cfg.Width), float64(cfg.Height)),
		uint(styleTitled|styleNonactivating|maskFullSize), uint(2), false)
	p.Send(sel("setTitlebarAppearsTransparent:"), true)
	p.Send(sel("setTitleVisibility:"), titleHidden)
	for i := 0; i < 3; i++ {
		if b := p.Send(selStandardButton, i); b != 0 {
			b.Send(selSetHidden, true)
		}
	}
	p.Send(sel("setMovable:"), false)
	p.Send(sel("setFloatingPanel:"), true)
	p.Send(sel("setHidesOnDeactivate:"), false)
	p.Send(sel("setLevel:"), levelStatus)
	p.Send(sel("setCollectionBehavior:"), uint(joinsAllSpaces|fullScreenAuxiliary))
	return p
}

// attach attaches an opened surface to the app's engine, as start starts
// one: at its view's size, in its environment.
func (s *surface) attach(app *engine.Engine) {
	s.env = s.environment()
	s.engine = app.Attach(s.id, s)
	size := objc.Send[apple.CGRect](s.native, selBounds).Size
	s.engine.Resize(driver.Size{Width: driver.Points(size.Width), Height: driver.Points(size.Height)})
	s.engine.SetEnvironment(s.env)
	if s.kind == driver.SurfaceTrayIcon {
		observeApp()
		return
	}
	s.observe()
	if s.hideOnBlur {
		center := objc.ID(objc.GetClass("NSNotificationCenter")).Send(sel("defaultCenter"))
		s.stops = append(s.stops, apple.Observe(center, apple.Global(appKitPath, "NSWindowDidResignKeyNotification"),
			func(note objc.ID) {
				if note.Send(sel("object")) == s.window && objc.Send[bool](s.window, sel("isVisible")) {
					s.hidePanel(true)
				}
			}))
	}
	if ownPerf != nil && s.kind == driver.SurfaceWindow && !ownHud {
		ownHud = true
		s.perf = ownPerf
		s.hud = newHud(s, ownPerf)
		s.startTicking()
	}
}

// ownHud is set once a window shows the performance monitor.
var ownHud bool

// show shows an opened surface's window: a panel under its anchor, a
// window without activating the app.
func (s *surface) show(anchor uint32) {
	if s.kind == driver.SurfacePanel {
		s.showPanel(anchor)
		return
	}
	s.window.Send(sel("orderFront:"), objc.ID(0))
}

// showPanel shows a panel and gives it the keyboard: under tray icon
// anchor, else in the upper middle of the screen with the mouse, as
// Spotlight shows.
func (s *surface) showPanel(anchor uint32) {
	w := s.window
	frame := objc.Send[apple.CGRect](w, sel("frame"))
	var screen objc.ID
	if tr := trays[anchor]; tr != nil && tr.item != 0 {
		bar := tr.item.Send(sel("button")).Send(selWindow)
		icon := objc.Send[apple.CGRect](bar, sel("frame"))
		screen = bar.Send(sel("screen"))
		frame.Origin.X = icon.Origin.X + icon.Size.Width/2 - frame.Size.Width/2
		frame.Origin.Y = icon.Origin.Y - frame.Size.Height - 6
	} else {
		mouse := objc.Send[apple.CGPoint](objc.ID(objc.GetClass("NSEvent")), sel("mouseLocation"))
		screens := objc.ID(objc.GetClass("NSScreen")).Send(sel("screens"))
		for i := range objc.Send[int](screens, sel("count")) {
			sc := screens.Send(sel("objectAtIndex:"), i)
			r := objc.Send[apple.CGRect](sc, sel("frame"))
			if mouse.X >= r.Origin.X && mouse.X < r.Origin.X+r.Size.Width && mouse.Y >= r.Origin.Y &&
				mouse.Y < r.Origin.Y+r.Size.Height {
				screen = sc
				break
			}
		}
		if screen == 0 {
			screen = objc.ID(objc.GetClass("NSScreen")).Send(sel("mainScreen"))
		}
		r := objc.Send[apple.CGRect](screen, sel("frame"))
		frame.Origin.X = r.Origin.X + (r.Size.Width-frame.Size.Width)/2
		frame.Origin.Y = r.Origin.Y + r.Size.Height*2/3 - frame.Size.Height/2
	}
	if screen != 0 {
		// Inside the screen's visible part.
		v := objc.Send[apple.CGRect](screen, sel("visibleFrame"))
		frame.Origin.X = max(v.Origin.X+8, min(frame.Origin.X, v.Origin.X+v.Size.Width-frame.Size.Width-8))
		frame.Origin.Y = max(v.Origin.Y+8, min(frame.Origin.Y, v.Origin.Y+v.Size.Height-frame.Size.Height))
	}
	s.anchor = anchor
	w.Send(sel("setFrameOrigin:"), frame.Origin)
	w.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
}

// hidePanel hides a panel; blurred says it lost the focus, which the app
// hears.
func (s *surface) hidePanel(blurred bool) {
	s.window.Send(sel("orderOut:"), objc.ID(0))
	if !blurred {
		return
	}
	if tr := trays[s.anchor]; tr != nil {
		tr.blurred = time.Now()
	}
	s.engine.Dispatch(driver.Event{Kind: driver.EventWindowHidden})
}

// destroy closes an opened surface whose tree is gone.
func (s *surface) destroy() {
	if s.closed {
		return
	}
	window := s.window
	if s.embedded {
		s.native.Send(selRemoveFromSuperview)
	} else if window != 0 {
		s.closePopup()
		window.Send(selSetDelegate, objc.ID(0))
		window.Send(sel("orderOut:"), objc.ID(0))
	}
	s.close()
	delete(views, s.native)
	s.native.Send(selRelease)
	if !s.embedded && window != 0 {
		// Closed once the batch that destroyed it has returned.
		apple.OnMain(func() {
			window.Send(sel("close"))
			window.Send(selRelease)
		})
	}
}

// setDock shows the app in the Dock and the app switcher, or not.
func setDock(on bool) {
	policy := policyAccessory
	if on {
		policy = policyRegular
	}
	app := nsApp()
	if objc.Send[int](app, sel("activationPolicy")) == policy {
		return
	}
	app.Send(sel("setActivationPolicy:"), policy)
	if on {
		// The menu bar becomes the app's once it activates again.
		activateApp()
	}
}

// smAppService is ServiceManagement's SMAppService, loaded once; nil
// before macOS 13.
func smAppService() objc.ID {
	if cls := objc.GetClass("SMAppService"); cls != 0 {
		return objc.ID(cls)
	}
	if _, err := purego.Dlopen("/System/Library/Frameworks/ServiceManagement.framework/ServiceManagement",
		purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		return 0
	}
	return objc.ID(objc.GetClass("SMAppService"))
}

// setLaunchAtLogin registers the app as a login item, or unregisters it.
// The system takes only apps in a bundle.
func setLaunchAtLogin(on bool) {
	cls := smAppService()
	if cls == 0 {
		return
	}
	service := cls.Send(sel("mainAppService"))
	var err objc.ID
	method := "unregisterAndReturnError:"
	if on {
		method = "registerAndReturnError:"
	}
	if !objc.Send[bool](service, sel(method), unsafe.Pointer(&err)) && err != 0 {
		log.Printf("nsgo: launch at login: %s", apple.GoString(err.Send(sel("localizedDescription"))))
	}
}

// launchesAtLogin reports whether the app is an enabled login item.
func launchesAtLogin() bool {
	cls := smAppService()
	if cls == 0 {
		return false
	}
	const enabled = 1
	return objc.Send[int](cls.Send(sel("mainAppService")), sel("status")) == enabled
}
