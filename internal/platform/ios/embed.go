//go:build ios

package ios

import (
	"errors"
	"log"
	"slices"
	"strings"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/internal/platform/imageload"
)

// Embedding: an iOS app of its own shows NSGo apps in NSGoViews, from the
// framework `nsgo build -embed` makes. The framework's NSGoStart runs the
// app's main, whose nsgo.Run hands this backend the app (Embed), then
// registers NSGoView, a UIView subclass of the surface's class, and
// NSGoComponentContext. The framework's header declares both visible to
// the Objective-C runtime only, so Swift reaches them by name. Each
// NSGoView is a surface of its own: its engine starts once it is in a
// window and stops as it is deallocated or stopped. Values cross as
// Foundation's JSON objects (apple.JSONObject); handlers and components
// are the host's blocks, which run on the main thread.

var (
	embedApp  engine.App
	embedOpts Options
	// contexts are the components' contexts, by the object the host's
	// component holds.
	contexts = map[objc.ID]*driver.NativeContext{}
	// classContext is NSGoComponentContext.
	classContext objc.Class
)

// Embed keeps the app every NSGoView shows and its options, from
// nsgo.Run in an embedded build.
func Embed(app engine.App, opts Options) { embedApp, embedOpts = app, opts }

// StartEmbedded loads UIKit's classes and registers NSGoView, on the main
// thread, once Embed has the app. resources is where the app's resources
// are in the host's bundle.
func StartEmbedded(resources string) {
	if resources != "" {
		imageload.ResourceRoot = resources
	}
	loadUIKit()
	apple.RegisterFonts(embedOpts.Fonts)
	classContext = register("NSGoComponentContext", apple.Class("NSObject"), nil, []objc.MethodDef{
		{Cmd: sel("emit:data:"), Fn: func(self objc.ID, _ objc.SEL, name, data objc.ID) {
			if cx := contexts[self]; cx != nil {
				cx.Emit(apple.GoString(name), apple.GoJSON(data))
			}
		}},
		{Cmd: sel("setSize:"), Fn: func(self objc.ID, _ objc.SEL, size apple.CGSize) {
			if cx := contexts[self]; cx != nil {
				cx.SetSize(driver.Size{Width: driver.Points(size.Width), Height: driver.Points(size.Height)})
			}
		}},
	})
	// Not a subclass of NsgoSurface: apple.Super finds the superclass from
	// the nearest of NSGo's classes, so one may not inherit another's.
	register("NSGoView", apple.Class("UIView"), nil, append(slices.Clone(surfaceMethods), embedMethods()...))
}

// embedState is what an NSGoView holds of the host's settings: the
// objects its properties return, and the blocks it calls.
type embedState struct {
	params, dataDir, locale objc.ID
	tokens, darkTokens      objc.ID
	onError, onOpenURL      objc.ID
	fontScale               float64
	// hostStyle is the overrideUserInterfaceStyle the host set, which an
	// appearance the app forces hides; handlers are the host's blocks,
	// factories its components'.
	hostStyle int
	handlers  map[string]objc.ID
	factories map[string]objc.ID
}

// embedded returns the surface of an NSGoView.
func embedded(self objc.ID) *surface {
	if v := views[self]; v != nil && v.surface.embedded {
		return v.surface
	}
	return nil
}

// keep replaces *slot with a copy of o, retained, releasing the old one.
func keep(slot *objc.ID, o objc.ID, copy bool) {
	if *slot != 0 {
		slot.Send(selRelease)
	}
	if o != 0 {
		if copy {
			o = o.Send(sel("copy"))
		} else {
			o = o.Send(sel("retain"))
		}
	}
	*slot = o
}

func embedMethods() []objc.MethodDef {
	getter := func(get func(*embedState) objc.ID) func(objc.ID, objc.SEL) objc.ID {
		return func(self objc.ID, _ objc.SEL) objc.ID {
			if s := embedded(self); s != nil {
				return get(s.embed)
			}
			return 0
		}
	}
	setter := func(slot func(*embedState) *objc.ID, apply func(*surface, objc.ID)) func(objc.ID, objc.SEL, objc.ID) {
		return func(self objc.ID, _ objc.SEL, o objc.ID) {
			s := embedded(self)
			if s == nil {
				return
			}
			keep(slot(s.embed), o, true)
			if apply != nil {
				apply(s, o)
			}
		}
	}
	return []objc.MethodDef{
		{Cmd: sel("initWithFrame:"), Fn: embedInit},
		// Hardware keys reach the view while a field in it has the focus.
		{Cmd: sel("pressesBegan:withEvent:"), Fn: pressesBegan},
		{Cmd: sel("pressesEnded:withEvent:"), Fn: pressesEnded},
		{Cmd: sel("pressesCancelled:withEvent:"), Fn: pressesEnded},
		{Cmd: sel("didMoveToWindow"), Fn: embedMoved},
		{Cmd: sel("dealloc"), Fn: func(self objc.ID, cmd objc.SEL) {
			if s := embedded(self); s != nil {
				s.close()
				delete(views, self)
			}
			apple.Super(self, cmd)
		}},
		{Cmd: sel("stop"), Fn: func(self objc.ID, _ objc.SEL) {
			if s := embedded(self); s != nil {
				s.close()
			}
		}},
		{Cmd: sel("setOverrideUserInterfaceStyle:"), Fn: func(self objc.ID, cmd objc.SEL, style int) {
			if s := embedded(self); s != nil {
				s.embed.hostStyle = style
				s.embedStyle()
				return
			}
			apple.Super(self, cmd, style)
		}},

		{Cmd: sel("params"), Fn: getter(func(e *embedState) objc.ID { return e.params })},
		{Cmd: sel("setParams:"), Fn: setter(func(e *embedState) *objc.ID { return &e.params }, func(s *surface, o objc.ID) {
			params, _ := apple.GoJSON(o).(map[string]any)
			if err := s.host.Channel.SetParams(params); err != nil {
				log.Printf("nsgo: NSGoView params: %v", err)
			}
		})},
		{Cmd: sel("dataDirectory"), Fn: getter(func(e *embedState) objc.ID { return e.dataDir })},
		{Cmd: sel("setDataDirectory:"), Fn: setter(func(e *embedState) *objc.ID { return &e.dataDir }, func(s *surface, o objc.ID) {
			s.host.DataDir = apple.GoString(o)
		})},
		{Cmd: sel("localeIdentifier"), Fn: getter(func(e *embedState) objc.ID { return e.locale })},
		{Cmd: sel("setLocaleIdentifier:"), Fn: setter(func(e *embedState) *objc.ID { return &e.locale }, func(s *surface, _ objc.ID) {
			s.applyHostEnvironment()
		})},
		{Cmd: sel("setFontScale:"), Fn: func(self objc.ID, _ objc.SEL, scale float64) {
			if s := embedded(self); s != nil {
				s.embed.fontScale = scale
				s.applyHostEnvironment()
			}
		}},
		{Cmd: sel("tokens"), Fn: getter(func(e *embedState) objc.ID { return e.tokens })},
		{Cmd: sel("setTokens:"), Fn: setter(func(e *embedState) *objc.ID { return &e.tokens }, (*surface).applyTokens)},
		{Cmd: sel("darkTokens"), Fn: getter(func(e *embedState) objc.ID { return e.darkTokens })},
		{Cmd: sel("setDarkTokens:"), Fn: setter(func(e *embedState) *objc.ID { return &e.darkTokens }, (*surface).applyTokens)},
		{Cmd: sel("onError"), Fn: getter(func(e *embedState) objc.ID { return e.onError })},
		{Cmd: sel("setOnError:"), Fn: setter(func(e *embedState) *objc.ID { return &e.onError }, nil)},
		{Cmd: sel("onOpenURL"), Fn: getter(func(e *embedState) objc.ID { return e.onOpenURL })},
		{Cmd: sel("setOnOpenURL:"), Fn: setter(func(e *embedState) *objc.ID { return &e.onOpenURL }, (*surface).routeURLs)},
		{Cmd: sel("preferredStatusBarStyle"), Fn: func(self objc.ID, _ objc.SEL) int {
			if s := embedded(self); s != nil {
				return s.statusBar
			}
			return 0
		}},
		{Cmd: sel("prefersStatusBarHidden"), Fn: func(self objc.ID, _ objc.SEL) bool {
			s := embedded(self)
			return s != nil && s.hideBar
		}},

		{Cmd: sel("send:data:"), Fn: func(self objc.ID, _ objc.SEL, name, data objc.ID) {
			if s := embedded(self); s != nil {
				if err := s.host.Channel.Send(apple.GoString(name), apple.GoJSON(data)); err != nil {
					log.Printf("nsgo: NSGoView send: %v", err)
				}
			}
		}},
		{Cmd: sel("handle:with:"), Fn: func(self objc.ID, _ objc.SEL, name, block objc.ID) {
			if s := embedded(self); s != nil {
				s.handle(apple.GoString(name), block)
			}
		}},
		{Cmd: sel("removeHandler:"), Fn: func(self objc.ID, _ objc.SEL, name objc.ID) {
			if s := embedded(self); s != nil {
				s.handle(apple.GoString(name), 0)
			}
		}},
		{Cmd: sel("registerComponent:factory:"), Fn: func(self objc.ID, _ objc.SEL, name, factory objc.ID) {
			s := embedded(self)
			if s == nil {
				return
			}
			n := apple.GoString(name)
			if strings.HasPrefix(n, "nsgo/") {
				log.Printf("nsgo: component names under nsgo/ are NSGo's own: %q is not registered", n)
				return
			}
			old := s.embed.factories[n]
			keep(&old, factory, true)
			s.embed.factories[n] = old
		}},
	}
}

// embedInit makes an NSGoView's surface around it, for the app Embed
// keeps.
func embedInit(self objc.ID, cmd objc.SEL, frame apple.CGRect) objc.ID {
	self = apple.SendSuper[objc.ID](self, cmd, frame)
	if self == 0 {
		return 0
	}
	if embedApp == nil {
		log.Print("nsgo: an NSGoView was made before NSGoStart: it stays empty")
		return self
	}
	s := newSurface(embedApp, embedOpts)
	s.embedded, s.native = true, self
	s.embed = &embedState{handlers: map[string]objc.ID{}, factories: map[string]objc.ID{}}
	s.host = engine.HostOptions{Channel: engine.NewHost(), Settings: &engine.HostSettings{}}
	s.host.OnError = func(err error) {
		apple.OnMain(func() {
			if s.embed.onError != 0 {
				apple.CallBlock(s.embed.onError, uintptr(apple.NSString(err.Error())))
			}
		})
	}
	views[self] = &view{surface: s}
	return self
}

// embedMoved starts an NSGoView's app once it is in a window, and stops
// its frame clock while it is in none: the display link holds the view.
func embedMoved(self objc.ID, cmd objc.SEL) {
	apple.Super(self, cmd)
	s := embedded(self)
	if s == nil || s.closed {
		return
	}
	if s.uiWindow() == 0 {
		s.dropLink()
		return
	}
	if s.engine == nil {
		s.watch()
		s.env = s.environment()
		s.startEngine(nil)
		return
	}
	s.refresh()
	if s.frameWanted.Load() || len(s.motions) > 0 {
		s.startTicking()
	}
}

// dropLink lets the display link go, which holds the view.
func (s *surface) dropLink() {
	if s.link != 0 {
		s.link.Send(sel("invalidate"))
		s.link.Send(selRelease)
		s.link = 0
	}
	s.ticking = false
	s.publishPace()
}

// close stops the surface's frame clock, its observations and its
// engine, then destroys the native components still showing.
func (s *surface) close() {
	if s.closed {
		return
	}
	s.closed = true
	s.dropLink()
	for _, stop := range s.stops {
		stop()
	}
	s.stops = nil
	if s.engine != nil {
		s.engine.Close()
		engine.SyncStorage()
	}
	for _, v := range s.views {
		if v != nil && v.component != nil {
			v.component.Close()
			v.component = nil
		}
	}
	// Its native objects go with it, and their addresses come back.
	for id, v := range views {
		if v.surface == s && id != s.native {
			delete(views, id)
		}
	}
}

// applyHostEnvironment sends the host's locale and text size.
func (s *surface) applyHostEnvironment() {
	locale := strings.ReplaceAll(apple.GoString(s.embed.locale), "_", "-")
	s.host.Settings.SetEnvironment(engine.HostEnvironment{Locale: locale, FontScale: float32(s.embed.fontScale)})
}

// applyTokens sends the host's tokens, as strings.
func (s *surface) applyTokens(objc.ID) {
	strs := func(o objc.ID) map[string]string {
		m, _ := apple.GoJSON(o).(map[string]any)
		if m == nil {
			return nil
		}
		out := make(map[string]string, len(m))
		for k, v := range m {
			if str, ok := v.(string); ok {
				out[k] = str
			}
		}
		return out
	}
	s.host.Settings.SetTokens(engine.Tokens{Values: strs(s.embed.tokens), Dark: strs(s.embed.darkTokens)})
}

// embedStyle shows an NSGoView in the appearance the app forces, else
// in the one its host set.
func (s *surface) embedStyle() {
	style := s.embed.hostStyle
	switch s.appearance {
	case driver.AppearanceLight:
		style = 1
	case driver.AppearanceDark:
		style = 2
	}
	apple.SendSuper[objc.ID](s.native, sel("setOverrideUserInterfaceStyle:"), style)
}

// handle has the host's block answer the app's calls of name, on the
// main thread: block(data, reply) calls reply(result, error) once, from
// any thread. A zero block removes the handler.
func (s *surface) handle(name string, block objc.ID) {
	old := s.embed.handlers[name]
	if block == 0 {
		if old != 0 {
			old.Send(selRelease)
		}
		delete(s.embed.handlers, name)
		s.host.Channel.RemoveHandler(name)
		return
	}
	keep(&old, block, true)
	s.embed.handlers[name] = old
	s.host.Channel.Handle(name, func(call *engine.HostCall) {
		apple.OnMain(func() {
			b := s.embed.handlers[name]
			if b == 0 {
				call.Reject(errors.New("nsgo: the host removed the handler"))
				return
			}
			reply := objc.NewBlock(func(_ objc.Block, result, failure objc.ID) {
				if failure != 0 {
					call.Reject(errors.New(apple.GoString(failure)))
				} else {
					call.Resolve(apple.GoJSON(result))
				}
			})
			apple.CallBlock(b, uintptr(apple.JSONObject(call.Data)), uintptr(reply))
			reply.Release()
		})
	})
}

// routeURLs has the host's onOpenURL block see the URLs the app opens.
func (s *surface) routeURLs(block objc.ID) {
	if block == 0 {
		s.host.Channel.HandleOpenURL(nil)
		return
	}
	s.host.Channel.HandleOpenURL(func(u string, answer func(bool)) {
		apple.OnMain(func() {
			b := s.embed.onOpenURL
			url := objc.ID(apple.Class("NSURL")).Send(sel("URLWithString:"), apple.NSString(u))
			answer(b != 0 && url != 0 && byte(apple.CallBlock(b, uintptr(url))) != 0)
		})
	})
}

// hostComponent is a component the host registered with its NSGoView: an
// object of its own conforming to NSGoComponent.
type hostComponent struct{ obj, cx objc.ID }

// hostFactory returns the factory of a component the host registered as
// name, or nil.
func (s *surface) hostFactory(name string) driver.NativeFactory {
	if !s.embedded || s.embed.factories[name] == 0 {
		return nil
	}
	return func(cx *driver.NativeContext) driver.NativeComponent {
		ctx := objc.ID(classContext).Send(selNew)
		contexts[ctx] = cx
		obj := objc.ID(apple.CallBlock(s.embed.factories[name], uintptr(ctx)))
		if obj == 0 {
			delete(contexts, ctx)
			ctx.Send(selRelease)
			return nil
		}
		return &hostComponent{obj: obj.Send(sel("retain")), cx: ctx}
	}
}

func (c *hostComponent) View() any { return c.obj.Send(sel("view")) }

func (c *hostComponent) SetProp(name string, value any) {
	c.obj.Send(sel("setProp:value:"), apple.NSString(name), apple.JSONObject(value))
}

// SetAnimatedNumber implements driver.AnimatedPropSetter.
func (c *hostComponent) SetAnimatedNumber(name string, v float64) {
	if objc.Send[bool](c.obj, sel("respondsToSelector:"), sel("setAnimatedNumber:value:")) {
		c.obj.Send(sel("setAnimatedNumber:value:"), apple.NSString(name), v)
		return
	}
	c.SetProp(name, v)
}

// SetAnimatedColor implements driver.AnimatedPropSetter, handing the host
// 0xAARRGGBB as NSH does.
func (c *hostComponent) SetAnimatedColor(name string, col driver.Color) {
	argb := uint32(col)>>8 | uint32(col)<<24
	if objc.Send[bool](c.obj, sel("respondsToSelector:"), sel("setAnimatedColor:argb:")) {
		c.obj.Send(sel("setAnimatedColor:argb:"), apple.NSString(name), argb)
		return
	}
	c.SetProp(name, float64(argb))
}

func (c *hostComponent) Command(name string, args any) {
	if objc.Send[bool](c.obj, sel("respondsToSelector:"), sel("command:args:")) {
		c.obj.Send(sel("command:args:"), apple.NSString(name), apple.JSONObject(args))
	}
}

func (c *hostComponent) Close() {
	if objc.Send[bool](c.obj, sel("respondsToSelector:"), sel("destroy")) {
		c.obj.Send(sel("destroy"))
	}
	delete(contexts, c.cx)
	c.cx.Send(selRelease)
	c.obj.Send(selRelease)
}
