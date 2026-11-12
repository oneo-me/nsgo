//go:build windows

package windows

// Native components, as NSH's native.rs hosts a component's window: the
// app registers factories by name (Options.Components), and a Native
// element's view makes one with its first ItemNative. A component whose
// View is a window handle has that window made a child of the surface's,
// moved where its view was drawn after every frame, clipped with a window
// region to what its clipping ancestors (scroll views among them) and its
// own rounded corners leave visible, and hidden while its view is not
// drawn. Being a window, it sits above everything the window draws, web
// views included: the "airspace" of child windows. The view itself keeps
// its decoration, frame and presses; the component's window takes its
// own input from Windows.

import (
	"log"
	"math"
	"slices"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var (
	gdi32 = syscall.NewLazyDLL("gdi32.dll")

	createRectRgn      = gdi32.NewProc("CreateRectRgn")
	createRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")
	combineRgn         = gdi32.NewProc("CombineRgn")
	deleteObject       = gdi32.NewProc("DeleteObject")
	setWindowRgn       = user32.NewProc("SetWindowRgn")
	setParent          = user32.NewProc("SetParent")
)

const (
	wsChild        = 0x40000000
	wsClipSiblings = 0x04000000
	swHide         = 0
	swpShowWindow  = 0x0040
	rgnAnd         = 1
)

// component is a native view's component and the window it brought, if
// any.
type component struct {
	c      driver.NativeComponent // nil for an unknown name: an empty box
	hwnd   uintptr
	placed *componentPlacement // where the last frame put it; nil hidden
	// closed stops events the component emits after it went, when its
	// view's ID may name another view.
	closed atomic.Bool
}

// componentPlacement is where a frame put a component's window, in the
// window's client pixels: its whole rectangle, the part of it visible and
// the radius of its corners when its view clips.
type componentPlacement struct {
	r, visible rect
	radius     int32
}

// placedComponent is a component's window a frame reached.
type placedComponent struct {
	c  *component
	at componentPlacement
}

// setNative makes v's component with its first item, then sets the props
// that changed, in the order of their names.
func (w *window) setNative(v *view, c *driver.NativeConfig) {
	if c.Bind {
		w.animator.BindNative(v.id, c.Bindings)
	}
	if v.component == nil {
		v.component = w.makeComponent(v.id, c.Component)
	}
	nc := v.component.c
	if nc == nil {
		return
	}
	names := make([]string, 0, len(c.Props))
	for name := range c.Props {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		nc.SetProp(name, c.Props[name])
	}
	w.redrawAll()
}

// makeComponent makes the component name registers for view id; an
// unknown name makes an empty one, logged once.
func (w *window) makeComponent(id driver.NodeID, name string) *component {
	c := &component{}
	factory := w.components[name]
	if factory == nil {
		if !w.missing[name] {
			w.missing[name] = true
			log.Printf("nsgo: no native component is registered as %q: it shows an empty box", name)
		}
		return c
	}
	ctx := driver.NewNativeContext(func(event string, data any) {
		if !c.closed.Load() {
			w.engine.Dispatch(driver.Event{Kind: driver.EventNative, ID: id, Text: event, Data: data})
		}
	}, func(size driver.Size) {
		if !c.closed.Load() {
			w.engine.Dispatch(driver.Event{Kind: driver.EventNativeSize, ID: id, Size: size})
		}
	})
	c.c = factory(ctx)
	if c.c == nil {
		return c
	}
	if c.hwnd = windowHandle(c.c.View()); c.hwnd != 0 {
		w.adopt(c.hwnd)
		w.hosts[c] = true
	}
	return c
}

// windowHandle returns the window handle a component's View returned, or
// 0 when it is none.
func windowHandle(view any) uintptr {
	switch h := view.(type) {
	case uintptr:
		return h
	case syscall.Handle:
		return uintptr(h)
	case unsafe.Pointer:
		return uintptr(h)
	}
	return 0
}

// adopt makes hwnd a hidden child of the window; the next frame shows it
// where its view is drawn.
func (w *window) adopt(hwnd uintptr) {
	style, _, _ := getWindowLongPtrW.Call(hwnd, gwlStyle)
	style = style&^wsPopup | wsChild | wsClipSiblings
	setWindowLongPtrW.Call(hwnd, gwlStyle, style)
	if r, _, err := setParent.Call(hwnd, w.hwnd); r == 0 {
		log.Printf("nsgo: cannot host a native component's window: %v", err)
	}
	showWindow.Call(hwnd, swHide)
}

// closeComponent hides the component's window and closes the component,
// which destroys its window.
func (w *window) closeComponent(c *component) {
	c.closed.Store(true)
	if c.hwnd != 0 {
		showWindow.Call(c.hwnd, swHide)
		delete(w.hosts, c)
	}
	if c.c != nil {
		c.c.Close()
		c.c = nil
	}
	c.hwnd, c.placed = 0, nil
}

// placeComponent records that component c's window, width by height
// points at world, with corners of radius when its view clips, is drawn
// here.
func (p *painter) placeComponent(c *component, world matrix, width, height, radius float32) {
	s := float64(p.scale)
	px := func(v float32) int32 { return int32(math.Round(min(max(float64(v)*s, -1e8), 1e8))) }
	b := mapBounds(world, rectF{0, 0, width, height})
	r := rect{px(b.left), px(b.top), px(b.right), px(b.bottom)}
	clip := p.clipBounds()
	visible := rect{max(r.Left, px(clip.left)), max(r.Top, px(clip.top)),
		min(r.Right, px(clip.right)), min(r.Bottom, px(clip.bottom))}
	if r.empty() || visible.empty() {
		return
	}
	p.components = append(p.components, placedComponent{c, componentPlacement{r: r, visible: visible,
		radius: int32(math.Round(float64(radius) * s))}})
}

// placeComponents moves the components' windows where the frame just
// shown drew their views, and hides the others.
func (w *window) placeComponents(placed []placedComponent) {
	if len(w.hosts) == 0 {
		return
	}
	for c := range w.hosts {
		i := slices.IndexFunc(placed, func(pc placedComponent) bool { return pc.c == c })
		if i < 0 {
			c.place(nil)
		} else {
			c.place(&placed[i].at)
		}
	}
}

// place moves c's window to at, clipped to its visible part and its
// corners, or hides it when at is nil.
func (c *component) place(at *componentPlacement) {
	if at == nil {
		if c.placed != nil {
			showWindow.Call(c.hwnd, swHide)
			c.placed = nil
		}
		return
	}
	if c.placed != nil && *c.placed == *at {
		return
	}
	pl := *at
	c.placed = &pl
	r := pl.r
	width, height := r.Right-r.Left, r.Bottom-r.Top
	setWindowPos.Call(c.hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(width), uintptr(height),
		swpShowWindow|swpNoActivate|swpNoZOrder)
	if pl.visible == r && pl.radius == 0 {
		setWindowRgn.Call(c.hwnd, 0, 1)
		return
	}
	var region uintptr
	if pl.radius > 0 {
		region, _, _ = createRoundRectRgn.Call(0, 0, uintptr(width+1), uintptr(height+1),
			uintptr(pl.radius*2), uintptr(pl.radius*2))
	} else {
		region, _, _ = createRectRgn.Call(0, 0, uintptr(width), uintptr(height))
	}
	v := pl.visible
	cut, _, _ := createRectRgn.Call(uintptr(v.Left-r.Left), uintptr(v.Top-r.Top), uintptr(v.Right-r.Left),
		uintptr(v.Bottom-r.Top))
	combineRgn.Call(region, region, cut, rgnAnd)
	deleteObject.Call(cut)
	// The window owns the region once it is set.
	if ok, _, _ := setWindowRgn.Call(c.hwnd, region, 1); ok == 0 {
		deleteObject.Call(region)
	}
}
