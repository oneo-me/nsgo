//go:build js && wasm

// Package miniprogram is NSGo's mini program backend, for WeChat, QQ,
// Alipay and Douyin alike: the engine
// and the app run in WebAssembly in the mini program's logic layer, each
// batch becomes operations its runtime applies to a tree of plain objects
// and hands the render layer with setData, and one recursive component
// draws each view at its frame. Text is measured on an offscreen canvas
// and broken into lines here (text.go), canvases are drawn on one and
// shown as pictures, and taps, touches, scrolls and edits come back as
// events (mount.go, touch.go).
//
// The runtime (cmd/nsgo/miniprogram) starts the module with Go's
// wasm_exec.js in a global object of its own, nsgoMP being the bridge:
// what it calls in Go, Go sets on it, and what Go calls it provides,
// the platform's APIs (api) among them, as WeChat's whichever platform it
// is.
package miniprogram

import (
	"math"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/canvas2d"
)

// Options configure the app.
type Options struct {
	// ID names the app's entries in the mini program's storage.
	ID string
	// Appearance is the appearance the app starts in.
	Appearance driver.Appearance
}

// surface is the app's page: the engine's platform.
type surface struct {
	bridge js.Value
	// api is the platform's APIs, with WeChat's names and options.
	api     js.Value
	m       *mounter
	text    *TextEngine
	engine  *engine.Engine
	env     driver.Environment
	started time.Time
	// canvas is the offscreen canvas drawings are drawn on, one at a
	// time, and images what they draw, by source.
	canvas js.Value
	images map[string]*canvasImage
	// files are images of bytes written for <image> to load, by key.
	files map[string]string
	// appearance is the one the app forces, AppearanceSystem for none.
	appearance driver.Appearance
}

// Run runs app in the mini program's page. Call it from main; it does
// not return.
func Run(app engine.App, opts Options) {
	g := js.Global()
	bridge := g.Get("nsgoMP")
	s := &surface{bridge: bridge, api: bridge.Get("api"), started: time.Now(),
		images: map[string]*canvasImage{}, files: map[string]string{}}
	s.text = NewTextEngine(measurer{s.bridge})
	s.m = newMounter(s)
	s.m.frame = func() { s.engine.Frame() }
	s.appearance = opts.Appearance
	s.env = s.environment()
	s.m.reduce = s.env.ReduceMotion
	id := opts.ID
	if id == "" {
		id = "default"
	}
	s.engine = engine.Start(s, app, engine.Options{
		Size: s.size(), Environment: s.env, Appearance: opts.Appearance,
		Storage: engine.NewStorage(&storage{api: s.api, prefix: "nsgo:" + id + ":"}),
	})
	s.bridge.Set("event", js.FuncOf(func(_ js.Value, args []js.Value) any {
		s.event(args)
		return nil
	}))
	s.bridge.Call("ready")
	select {}
}

// measurer measures text on the runtime's offscreen canvas.
type measurer struct{ bridge js.Value }

func (m measurer) Width(font, text string) float64 {
	return m.bridge.Call("measure", font, text).Float()
}

// Prepare implements engine.Platform.
func (s *surface) Prepare(t *driver.AttributedText) driver.PreparedText { return s.text.Prepare(t) }

// MountsAtOnce implements engine.SharedMounter: Mount applies a batch
// before it returns, as the engine shares the logic layer's one thread.
func (s *surface) MountsAtOnce() {}

// Mount implements engine.Platform.
func (s *surface) Mount(b *driver.MountBatch) { driver.Apply(b, s.m.mount) }

// RequestFrame implements engine.Platform.
func (s *surface) RequestFrame() { s.m.requestFrame() }

func (s *surface) submit(ops []byte) { s.bridge.Call("submit", string(ops)) }

func (s *surface) dispatch(ev driver.Event) { s.engine.Dispatch(ev) }

func (s *surface) tick() { s.bridge.Call("tick") }

// device is what the runtime reports about the page and the device.
func (s *surface) device() js.Value { return s.bridge.Call("device") }

func (s *surface) size() driver.Size {
	d := s.device()
	return driver.Size{Width: driver.Points(d.Get("width").Float()), Height: driver.Points(d.Get("height").Float())}
}

// environment reads the environment from the runtime's device.
func (s *surface) environment() driver.Environment {
	d := s.device()
	num := func(name string) driver.Points { return driver.Points(d.Get(name).Float()) }
	env := s.env
	env.SafeArea = driver.Edges{Top: num("safeTop"), Right: num("safeRight"), Bottom: num("safeBottom"), Left: num("safeLeft")}
	env.Dark = d.Get("dark").Truthy()
	env.PixelRatio = float32(d.Get("pixelRatio").Float())
	env.FontScale = float32(d.Get("fontScale").Float())
	env.Locale = strings.ReplaceAll(d.Get("language").String(), "_", "-")
	env.OS = driver.OS(d.Get("os").String())
	env.OSVersion = osVersion(d.Get("osVersion").String())
	env.Idiom = driver.IdiomPhone
	if d.Get("tablet").Truthy() {
		env.Idiom = driver.IdiomTablet
	}
	return env
}

// osVersion reads "17.4.1" as 17.4.
func osVersion(v string) driver.OSVersion {
	var out driver.OSVersion
	parts := strings.SplitN(v, ".", 3)
	for i, p := range parts[:min(len(parts), 2)] {
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		if i == 0 {
			out.Major = n
		} else {
			out.Minor = n
		}
	}
	return out
}

// changed sends the environment and size again: the window resized, the
// theme or the keyboard changed.
func (s *surface) changed() {
	env := s.environment()
	if env != s.env {
		s.env = env
		s.m.reduce = env.ReduceMotion
		s.engine.SetEnvironment(env)
	}
	s.engine.Resize(s.size())
}

// event handles what the runtime reports: its name, the view's ID, then
// what the event carries.
func (s *surface) event(args []js.Value) {
	name := args[0].String()
	arg := func(i int) js.Value {
		if i < len(args) {
			return args[i]
		}
		return js.Undefined()
	}
	id := driver.NodeID(arg(1).Int())
	pt := func(i int) driver.Point {
		return driver.Point{X: driver.Points(arg(i).Float()), Y: driver.Points(arg(i + 1).Float())}
	}
	m := s.m
	switch name {
	case "tap":
		m.press(id, driver.EventPress, pt(2))
	case "longpress":
		m.press(id, driver.EventLongPress, pt(2))
	case "pressin":
		m.press(id, driver.EventPressDown, pt(2))
	case "pressout":
		m.press(id, driver.EventPressUp, pt(2))
	case "touch":
		phase := []driver.GesturePhase{driver.GestureBegan, driver.GestureChanged, driver.GestureEnded,
			driver.GestureCancelled}[arg(3).Int()]
		m.touch(id, arg(2).Int(), phase, fingers(arg(4)), fingers(arg(5)),
			time.Duration(arg(6).Float()*float64(time.Millisecond)))
	case "scroll":
		m.scrolled(id, pt(2))
	case "dragstart":
		m.scrollPhase(id, driver.EventScrollDragStart)
	case "dragend":
		m.scrollPhase(id, driver.EventScrollDragEnd)
	case "scrollend":
		m.scrollPhase(id, driver.EventScrollEnd)
	case "refresh":
		if v := m.view(id); v != nil {
			m.send(v, driver.Event{Kind: driver.EventRefresh})
		}
	case "input":
		m.edited(id, arg(2).String())
	case "confirm":
		m.field(id, driver.EventSubmitEditing, arg(2).String())
	case "focus":
		m.field(id, driver.EventFocus, "")
	case "blur":
		m.field(id, driver.EventBlur, "")
	case "keyboard":
		s.env.KeyboardHeight = driver.Points(arg(2).Float())
		s.env.KeyboardDuration = time.Duration(arg(3).Float() * float64(time.Second))
		s.engine.SetEnvironment(s.env)
	case "imgload":
		if v := m.view(id); v != nil {
			m.send(v, driver.Event{Kind: driver.EventLoad, Size: driver.Size{Width: driver.Points(arg(2).Float()),
				Height: driver.Points(arg(3).Float())}})
			m.send(v, driver.Event{Kind: driver.EventLoadEnd})
		}
	case "imgerror":
		if v := m.view(id); v != nil {
			m.send(v, driver.Event{Kind: driver.EventError, Text: arg(2).String()})
			m.send(v, driver.Event{Kind: driver.EventLoadEnd})
		}
	case "back":
		s.engine.Dispatch(driver.Event{Kind: driver.EventBack})
	case "frame":
		m.tick(time.Since(s.started))
	case "device":
		s.changed()
	case "lifecycle":
		s.env.Lifecycle = driver.LifecycleActive
		if !arg(2).Truthy() {
			s.env.Lifecycle = driver.LifecycleBackground
		}
		s.engine.SetEnvironment(s.env)
	case "alert":
		s.engine.Dispatch(driver.Event{Kind: driver.EventAlert, Count: uint32(arg(2).Int()), Index: arg(3).Int()})
	}
}

// fingers reads the runtime's fingers: [id, pageX, pageY] each.
func fingers(list js.Value) []finger {
	if list.Type() != js.TypeObject {
		return nil
	}
	out := make([]finger, list.Length())
	for i := range out {
		f := list.Index(i)
		out[i] = finger{id: f.Index(0).Int(), page: driver.Point{X: driver.Points(f.Index(1).Float()), Y: driver.Points(f.Index(2).Float())}}
	}
	return out
}

// paint draws ops on the offscreen canvas and returns the picture as a
// data URL, "" while an image they draw loads.
func (s *surface) paint(id driver.NodeID, ops []driver.DrawOp, size driver.Size) string {
	ratio := float64(s.env.PixelRatio)
	if ratio <= 0 {
		ratio = 2
	}
	w, h := float64(size.Width), float64(size.Height)
	if !s.canvas.Truthy() {
		s.canvas = s.api.Call("createOffscreenCanvas", map[string]any{"type": "2d", "width": 1, "height": 1})
	}
	c := s.canvas
	// Setting the size clears the canvas and resets its context.
	c.Set("width", int(w*ratio+0.999))
	c.Set("height", int(h*ratio+0.999))
	ctx := c.Call("getContext", "2d")
	ctx.Call("scale", ratio, ratio)
	waiting := false
	p := canvas2d.NewPlayer(ctx, ratio, math.Hypot(w, h))
	p.Font = canvasFont
	p.Image = func(src driver.ImageSource) (js.Value, float64, bool) {
		im := s.canvasImage(src, id)
		if !im.loaded {
			waiting = waiting || !im.failed
			return js.Undefined(), 1, false
		}
		return im.img, float64(src.Density()), true
	}
	for i := range ops {
		p.Play(&ops[i])
	}
	if waiting {
		return ""
	}
	return c.Call("toDataURL", "image/png").String()
}

// canvasImage is an image canvases draw, loaded once.
type canvasImage struct {
	img            js.Value
	loaded, failed bool
}

// canvasImage returns src's image, loading it the first time; the
// canvases waiting for it paint again once it loads.
func (s *surface) canvasImage(src driver.ImageSource, id driver.NodeID) *canvasImage {
	key := src.Key()
	if im := s.images[key]; im != nil {
		return im
	}
	if !s.canvas.Truthy() {
		s.canvas = s.api.Call("createOffscreenCanvas", map[string]any{"type": "2d", "width": 1, "height": 1})
	}
	im := &canvasImage{img: s.canvas.Call("createImage")}
	s.images[key] = im
	var onload, onerror js.Func
	done := func(ok bool) {
		onload.Release()
		onerror.Release()
		im.loaded, im.failed = ok, !ok
		s.m.repaintWaiting()
	}
	onload = js.FuncOf(func(js.Value, []js.Value) any { done(true); return nil })
	onerror = js.FuncOf(func(js.Value, []js.Value) any { done(false); return nil })
	im.img.Set("onload", onload)
	im.img.Set("onerror", onerror)
	im.img.Set("src", s.imageURL(src))
	return im
}

// imageURL is where an <image> loads src: the package's resources for
// asset: paths, a file written in the user's data for bytes, and URLs as
// they are.
func (s *surface) imageURL(src driver.ImageSource) string {
	if len(src.Data) > 0 {
		key := src.Key()
		if path, ok := s.files[key]; ok {
			return path
		}
		ext := ".png"
		switch driver.SniffImage(src.Data) {
		case "image/jpeg":
			ext = ".jpg"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		case "image/svg+xml":
			ext = ".svg"
		}
		path := s.api.Get("env").Get("USER_DATA_PATH").String() + "/nsgo-image-" + strconv.Itoa(len(s.files)) + ext
		data := js.Global().Get("Uint8Array").New(len(src.Data))
		js.CopyBytesToJS(data, src.Data)
		s.api.Call("getFileSystemManager").Call("writeFileSync", path, data.Get("buffer"))
		s.files[key] = path
		return path
	}
	if path, ok := strings.CutPrefix(src.URI, "asset:"); ok {
		return "/resources/" + strings.TrimPrefix(path, "/")
	}
	return src.URI
}
