//go:build js && wasm

package web

import (
	"math"
	"strconv"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/canvas2d"
	"github.com/oneo-me/nsgo/internal/platform/webstyle"
)

// setDrawing keeps a canvas view's drawing, binds its animated arguments
// and plays it.
func (s *surface) setDrawing(v *view, d *driver.Drawing) {
	v.drawing = d
	s.animator.BindCanvas(v.id, d.Slots)
	s.paint(v)
	if d != nil {
		s.describeNodes(v, d.Nodes)
	}
}

// changedCanvases plays again the canvases whose bound arguments changed.
func (s *surface) changedCanvases() {
	s.animator.ChangedCanvases(func(id driver.NodeID) {
		if v := s.view(id); v != nil && v.kind == driver.KindCanvas {
			s.paint(v)
		}
	})
}

// changedNatives hands components the animated props that changed.
func (s *surface) changedNatives() {
	s.animator.ChangedNatives(func(id driver.NodeID, b driver.NativeBinding, num float64, col driver.Color) {
		if v := s.view(id); v != nil {
			driver.SetAnimatedProp(v.component, b, num, col)
		}
	})
}

// paint plays a canvas view's drawing into its <canvas>, whose pixels
// follow the pixel ratio. The drawing's origin is inside the border.
func (s *surface) paint(v *view) {
	if v.drawing == nil {
		return
	}
	ratio := float64(s.env.PixelRatio)
	if ratio <= 0 {
		ratio = 1
	}
	w := math.Max(float64(v.size.Width-v.border.Left-v.border.Right), 0)
	h := math.Max(float64(v.size.Height-v.border.Top-v.border.Bottom), 0)
	// Setting the size clears the canvas and resets its context.
	v.inner.Set("width", math.Ceil(w*ratio))
	v.inner.Set("height", math.Ceil(h*ratio))
	ctx := v.inner.Call("getContext", "2d")
	ctx.Call("scale", ratio, ratio)
	p := canvas2d.NewPlayer(ctx, ratio, math.Hypot(w, h))
	p.Font = fontCSS
	p.Image = func(src driver.ImageSource) (js.Value, float64, bool) {
		im := s.loadImage(src, v)
		return im.img, im.density, im.loaded
	}
	ops := s.animator.CanvasOps(v.id, v.drawing)
	for i := range ops {
		p.Play(&ops[i])
	}
}

// canvasImage is an image canvases draw, loaded once for the surface.
type canvasImage struct {
	img     js.Value
	density float64
	loaded  bool
	waiting map[*view]bool
}

// loadImage returns src's image, loading it the first time, and has v
// draw again once it loads: a canvas plays again, a shader draws.
func (s *surface) loadImage(src driver.ImageSource, v *view) *canvasImage {
	key := src.Key()
	im := s.canvasImages[key]
	if im == nil {
		if s.canvasImages == nil {
			s.canvasImages = map[string]*canvasImage{}
		}
		im = &canvasImage{img: js.Global().Get("Image").New(), density: float64(src.Density()),
			waiting: map[*view]bool{}}
		s.canvasImages[key] = im
		var onload js.Func
		onload = js.FuncOf(func(js.Value, []js.Value) any {
			onload.Release()
			im.loaded = true
			for v := range im.waiting {
				switch {
				case s.view(v.id) != v:
				case v.kind == driver.KindShader:
					s.shadeLater(v)
				default:
					s.paint(v)
				}
			}
			im.waiting = nil
			return nil
		})
		im.img.Call("addEventListener", "load", onload)
		url, _ := imageURL(src)
		im.img.Set("src", url)
	}
	if !im.loaded && im.waiting != nil {
		im.waiting[v] = true
	}
	return im
}

// fontCSS is the CSS font shorthand of fnt.
func fontCSS(fnt driver.Font) string {
	family := cssFamily(fnt.Family)
	style := ""
	if fnt.Italic {
		style = "italic "
	}
	if fnt.Variant&driver.FontSmallCaps != 0 {
		style += "small-caps "
	}
	return style + strconv.Itoa(int(fnt.Weight)) + " " + webstyle.Px(fnt.Size) + " " + family
}

// describeNodes gives a canvas's named parts to assistive technology: an
// element each inside the <canvas>, which browsers expose as its
// children at their bounds and never draw.
func (s *surface) describeNodes(v *view, nodes []driver.CanvasNode) {
	v.inner.Set("textContent", "")
	for i := range nodes {
		n := &nodes[i]
		el := s.doc.Call("createElement", "div")
		if role := roles[n.Role]; role != "" {
			el.Call("setAttribute", "role", role)
		}
		if n.Label != "" {
			el.Call("setAttribute", "aria-label", n.Label)
			el.Set("textContent", n.Label)
		}
		if n.TestID != "" {
			el.Call("setAttribute", "data-testid", n.TestID)
		}
		if n.Selected {
			el.Call("setAttribute", "aria-selected", "true")
		}
		el.Call("setAttribute", "data-canvas-node", "")
		el.Set("nsgoCenter", []any{float64(n.Bounds.Origin.X + n.Bounds.Size.Width/2),
			float64(n.Bounds.Origin.Y + n.Bounds.Size.Height/2)})
		setStyle(el, map[string]string{"position": "absolute", "left": webstyle.Px(n.Bounds.Origin.X),
			"top": webstyle.Px(n.Bounds.Origin.Y), "width": webstyle.Px(n.Bounds.Size.Width), "height": webstyle.Px(n.Bounds.Size.Height)})
		v.inner.Call("appendChild", el)
	}
}

// canvasNodePress presses at the center of the canvas node el is, as
// assistive technology activates one: the view under that point that
// takes presses gets the press, as a pointer there would give it.
func (s *surface) canvasNodePress(el js.Value) bool {
	if !el.Truthy() || el.Get("nsgoCenter").IsUndefined() {
		return false
	}
	center := el.Get("nsgoCenter")
	box := el.Get("parentElement").Call("getBoundingClientRect")
	x := box.Get("left").Float() + center.Index(0).Float()
	y := box.Get("top").Float() + center.Index(1).Float()
	v := s.target(s.doc.Call("elementFromPoint", x, y))
	if v == nil {
		return true
	}
	at := v.el.Call("getBoundingClientRect")
	host := s.host.Call("getBoundingClientRect")
	ev := driver.Event{Count: 1,
		Point: driver.Point{X: driver.Points(x - at.Get("left").Float()), Y: driver.Points(y - at.Get("top").Float())},
		Page:  driver.Point{X: driver.Points(x - host.Get("left").Float()), Y: driver.Points(y - host.Get("top").Float())}}
	for _, k := range []driver.EventKind{driver.EventPressDown, driver.EventPressUp, driver.EventPress} {
		ev.Kind = k
		s.send(v, ev)
	}
	return true
}
