//go:build js && wasm

package web

import (
	"math"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
)

var (
	lineCaps  = map[driver.LineCap]string{driver.CapButt: "butt", driver.CapRound: "round", driver.CapSquare: "square"}
	lineJoins = map[driver.LineJoin]string{driver.JoinMiter: "miter", driver.JoinRound: "round", driver.JoinBevel: "bevel"}
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
	p := &canvasPlayer{s: s, v: v, ctx: ctx, ratio: ratio, size: math.Hypot(w, h), width: 1}
	ops := s.animator.CanvasOps(v.id, v.drawing)
	for i := range ops {
		p.play(&ops[i])
	}
}

// canvasPlayer plays a drawing into a 2D context the browser's canvas
// draws as NSGo's: the context already does what the ops say, so most
// map one to one.
type canvasPlayer struct {
	s     *surface
	v     *view
	ctx   js.Value
	ratio float64
	size  float64 // the canvas's diagonal, which repeating gradients cover
	// fill and stroke are the gradients painting, whose own transforms
	// apply as they paint, and width the line width; saved and widths
	// keep them for Restore.
	fill, stroke *driver.CanvasGradient
	saved        []*driver.CanvasGradient
	width        float64
	widths       []float64
}

func (p *canvasPlayer) play(op *driver.DrawOp) {
	ctx := p.ctx
	f := func(x float32) float64 { return float64(x) }
	switch op.Kind {
	case driver.DrawSave:
		ctx.Call("save")
		p.saved = append(p.saved, p.fill, p.stroke)
		p.widths = append(p.widths, p.width)
	case driver.DrawRestore:
		ctx.Call("restore")
		if n := len(p.saved); n >= 2 {
			p.fill, p.stroke = p.saved[n-2], p.saved[n-1]
			p.saved = p.saved[:n-2]
			p.width = p.widths[len(p.widths)-1]
			p.widths = p.widths[:len(p.widths)-1]
		}
	case driver.DrawTranslate:
		ctx.Call("translate", f(op.X), f(op.Y))
	case driver.DrawScale:
		ctx.Call("scale", f(op.X), f(op.Y))
	case driver.DrawRotate:
		ctx.Call("rotate", f(op.A))
	case driver.DrawTransform:
		ctx.Call("transform", f(op.X1), f(op.Y1), f(op.X2), f(op.Y2), f(op.X), f(op.Y))
	case driver.DrawSetTransform:
		r := p.ratio
		ctx.Call("setTransform", r*f(op.X1), r*f(op.Y1), r*f(op.X2), r*f(op.Y2), r*f(op.X), r*f(op.Y))
	case driver.DrawBeginPath:
		ctx.Call("beginPath")
	case driver.DrawMoveTo:
		ctx.Call("moveTo", f(op.X), f(op.Y))
	case driver.DrawLineTo:
		ctx.Call("lineTo", f(op.X), f(op.Y))
	case driver.DrawQuadTo:
		ctx.Call("quadraticCurveTo", f(op.X1), f(op.Y1), f(op.X), f(op.Y))
	case driver.DrawCubicTo:
		ctx.Call("bezierCurveTo", f(op.X1), f(op.Y1), f(op.X2), f(op.Y2), f(op.X), f(op.Y))
	case driver.DrawArc:
		ctx.Call("arc", f(op.X), f(op.Y), math.Max(f(op.R), 0), f(op.A0), f(op.A1), op.CCW)
	case driver.DrawRect:
		ctx.Call("rect", f(op.X), f(op.Y), f(op.W), f(op.H))
	case driver.DrawRoundRect:
		// arcTo rather than roundRect, which older browsers lack.
		x, y, w, h := f(op.X), f(op.Y), f(op.W), f(op.H)
		r := math.Max(math.Min(f(op.R), math.Min(math.Abs(w), math.Abs(h))/2), 0)
		ctx.Call("moveTo", x+r, y)
		ctx.Call("arcTo", x+w, y, x+w, y+h, r)
		ctx.Call("arcTo", x+w, y+h, x, y+h, r)
		ctx.Call("arcTo", x, y+h, x, y, r)
		ctx.Call("arcTo", x, y, x+w, y, r)
		ctx.Call("closePath")
	case driver.DrawEllipse:
		x, y := f(op.X), f(op.Y)
		ctx.Call("moveTo", x+math.Max(f(op.R), 0), y)
		ctx.Call("ellipse", x, y, math.Max(f(op.R), 0), math.Max(f(op.RY), 0), 0, 0, 2*math.Pi)
		ctx.Call("closePath")
	case driver.DrawClosePath:
		ctx.Call("closePath")
	case driver.DrawFill:
		p.paint(p.fill, func() { ctx.Call("fill", rule(op.EvenOdd)) }, false)
	case driver.DrawStroke:
		p.paint(p.stroke, func() { ctx.Call("stroke") }, true)
	case driver.DrawClip:
		ctx.Call("clip", rule(op.EvenOdd))
	case driver.DrawFillColor:
		p.fill = nil
		ctx.Set("fillStyle", cssColor(op.Color))
	case driver.DrawStrokeColor:
		p.stroke = nil
		ctx.Set("strokeStyle", cssColor(op.Color))
	case driver.DrawFillGradient:
		if g := p.gradient(op.Gradient); g.Truthy() {
			p.fill = op.Gradient
			ctx.Set("fillStyle", g)
		}
	case driver.DrawStrokeGradient:
		if g := p.gradient(op.Gradient); g.Truthy() {
			p.stroke = op.Gradient
			ctx.Set("strokeStyle", g)
		}
	case driver.DrawLineWidth:
		p.width = f(op.W)
		ctx.Set("lineWidth", f(op.W))
	case driver.DrawLineCap:
		ctx.Set("lineCap", lineCaps[op.Cap])
	case driver.DrawLineJoin:
		ctx.Set("lineJoin", lineJoins[op.Join])
	case driver.DrawMiterLimit:
		ctx.Set("miterLimit", f(op.W))
	case driver.DrawLineDash:
		var dash []any
		if op.Dash != nil {
			for _, l := range *op.Dash {
				dash = append(dash, float64(l))
			}
		}
		ctx.Call("setLineDash", dash)
	case driver.DrawLineDashOffset:
		ctx.Set("lineDashOffset", f(op.W))
	case driver.DrawAlpha:
		ctx.Set("globalAlpha", f(op.A))
	case driver.DrawComposite:
		ctx.Set("globalCompositeOperation", op.Composite.Name())
	case driver.DrawShadow:
		// Shadows ignore the transform: their lengths are the canvas's
		// pixels.
		ctx.Set("shadowColor", cssColor(op.Color))
		ctx.Set("shadowOffsetX", f(op.X)*p.ratio)
		ctx.Set("shadowOffsetY", f(op.Y)*p.ratio)
		ctx.Set("shadowBlur", f(op.R)*p.ratio)
	case driver.DrawFillText:
		ctx.Set("font", fontCSS(op.Font))
		x, y := f(op.X+op.X1), f(op.Y+op.Y1)
		p.paint(p.fill, func() { drawText(ctx, "fillText", op, x, y) }, false)
	case driver.DrawStrokeText:
		ctx.Set("font", fontCSS(op.Font))
		x, y := f(op.X+op.X1), f(op.Y+op.Y1)
		p.paint(p.stroke, func() { drawText(ctx, "strokeText", op, x, y) }, true)
	case driver.DrawClear:
		ctx.Call("clearRect", f(op.X), f(op.Y), f(op.W), f(op.H))
	case driver.DrawImage:
		p.image(op)
	}
}

func rule(evenOdd bool) string {
	if evenOdd {
		return "evenodd"
	}
	return "nonzero"
}

// paint runs draw with the gradient's own transform added for it, as
// SVG's gradientTransform says; a stroke keeps its line width.
func (p *canvasPlayer) paint(g *driver.CanvasGradient, draw func(), stroke bool) {
	if g == nil || g.Transform == ([6]float32{}) || !g.Radial && !g.Conic {
		draw()
		return
	}
	m := g.Matrix()
	ctx := p.ctx
	ctx.Call("save")
	ctx.Call("transform", float64(m[0]), float64(m[1]), float64(m[2]), float64(m[3]), float64(m[4]), float64(m[5]))
	if stroke {
		// The width in the gradient's space that strokes as wide.
		if det := math.Sqrt(math.Abs(float64(m[0]*m[3] - m[1]*m[2]))); det > 0 {
			ctx.Set("lineWidth", p.width/det)
		}
	}
	draw()
	ctx.Call("restore")
}

// gradient returns the browser's gradient for g. A linear gradient's own
// transform is folded into its points; the others take it as they paint.
// Repeating linear gradients repeat their stops across the canvas.
func (p *canvasPlayer) gradient(g *driver.CanvasGradient) js.Value {
	if g == nil || len(g.Stops) == 0 {
		return js.Null()
	}
	f := func(x float32) float64 { return float64(x) }
	stops := g.Stops
	var gr js.Value
	switch {
	case g.Conic:
		gr = p.ctx.Call("createConicGradient", f(g.Angle), f(g.X1), f(g.Y1))
	case g.Radial:
		gr = p.ctx.Call("createRadialGradient", f(g.X1), f(g.Y1), math.Max(f(g.R1), 0), f(g.X2), f(g.Y2),
			math.Max(f(g.R2), 0))
	default:
		x1, y1, x2, y2 := linearInUserSpace(g)
		if g.Spread != driver.SpreadPad {
			x1, y1, x2, y2, stops = p.repeat(g, x1, y1, x2, y2)
		}
		gr = p.ctx.Call("createLinearGradient", x1, y1, x2, y2)
	}
	for _, s := range stops {
		gr.Call("addColorStop", math.Min(math.Max(f(s.Offset), 0), 1), cssColor(s.Color))
	}
	return gr
}

// linearInUserSpace returns the points of the linear gradient that paints
// as g does through its transform: its start mapped, and its end along the
// direction of growing offsets, at the length that reaches offset 1.
func linearInUserSpace(g *driver.CanvasGradient) (x1, y1, x2, y2 float64) {
	m := g.Matrix()
	a, b, c, d, e, f := float64(m[0]), float64(m[1]), float64(m[2]), float64(m[3]), float64(m[4]), float64(m[5])
	gx1, gy1, gx2, gy2 := float64(g.X1), float64(g.Y1), float64(g.X2), float64(g.Y2)
	vx, vy := gx2-gx1, gy2-gy1
	l2 := vx*vx + vy*vy
	x1, y1 = a*gx1+c*gy1+e, b*gx1+d*gy1+f
	det := a*d - b*c
	if l2 == 0 || det == 0 {
		return x1, y1, a*gx2 + c*gy2 + e, b*gx2 + d*gy2 + f
	}
	// The offset grows along w = M⁻ᵀ v / |v|².
	wx := (d*vx - b*vy) / det / l2
	wy := (-c*vx + a*vy) / det / l2
	w2 := wx*wx + wy*wy
	return x1, y1, x1 + wx/w2, y1 + wy/w2
}

// repeat stretches a repeating or reflecting linear gradient over enough
// periods on each side to cover the canvas, its stops copied into each.
func (p *canvasPlayer) repeat(g *driver.CanvasGradient, x1, y1, x2, y2 float64) (float64, float64, float64, float64, []driver.GradientStop) {
	period := math.Hypot(x2-x1, y2-y1)
	if period <= 0 {
		return x1, y1, x2, y2, g.Stops
	}
	n := int(min(math.Ceil(p.size/period)+1, 32))
	total := 2*n + 1
	var stops []driver.GradientStop
	for k := -n; k <= n; k++ {
		flip := g.Spread == driver.SpreadReflect && k%2 != 0
		for i := range g.Stops {
			s := g.Stops[i]
			if flip {
				s = g.Stops[len(g.Stops)-1-i]
				s.Offset = 1 - s.Offset
			}
			s.Offset = (float32(k+n) + s.Offset) / float32(total)
			stops = append(stops, s)
		}
	}
	dx, dy := x2-x1, y2-y1
	return x1 - float64(n)*dx, y1 - float64(n)*dy, x2 + float64(n)*dx, y2 + float64(n)*dy, stops
}

// canvasImage is an image canvases draw, loaded once for the surface.
type canvasImage struct {
	img     js.Value
	density float64
	loaded  bool
	waiting map[*view]bool
}

// image draws op's image once the browser has it, and plays the canvas
// again when it loads.
func (p *canvasPlayer) image(op *driver.DrawOp) {
	if op.Image == nil {
		return
	}
	im := p.s.loadImage(*op.Image, p.v)
	if !im.loaded {
		return
	}
	f := func(x float32) float64 { return float64(x) }
	nw, nh := im.img.Get("naturalWidth").Float(), im.img.Get("naturalHeight").Float()
	if nw <= 0 || nh <= 0 {
		return
	}
	w, h := f(op.W), f(op.H)
	if w == 0 && h == 0 {
		w, h = nw/im.density, nh/im.density
	}
	if op.X2 == 0 && op.Y2 == 0 {
		p.ctx.Call("drawImage", im.img, f(op.X), f(op.Y), w, h)
		return
	}
	k := im.density
	p.ctx.Call("drawImage", im.img, f(op.X1)*k, f(op.Y1)*k, f(op.X2)*k, f(op.Y2)*k, f(op.X), f(op.Y), w, h)
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
	return style + strconv.Itoa(int(fnt.Weight)) + " " + px(fnt.Size) + " " + family
}

// drawText draws op's text at x, y with ctx's method. A canvas has no
// font-variant-numeric, so tabular figures are drawn one by one, each
// centered in the widest figure's advance, which is the width a font's
// tabular figures share.
func drawText(ctx js.Value, method string, op *driver.DrawOp, x, y float64) {
	if op.Font.Variant&driver.FontTabularNums == 0 || !strings.ContainsAny(op.Text, "0123456789") {
		ctx.Call(method, op.Text, x, y)
		return
	}
	var cell float64
	for d := '0'; d <= '9'; d++ {
		cell = max(cell, ctx.Call("measureText", string(d)).Get("width").Float())
	}
	rest := op.Text
	for rest != "" {
		i := strings.IndexAny(rest, "0123456789")
		if i != 0 {
			run := rest
			if i > 0 {
				run = rest[:i]
			}
			ctx.Call(method, run, x, y)
			x += ctx.Call("measureText", run).Get("width").Float()
			rest = rest[len(run):]
			continue
		}
		w := ctx.Call("measureText", rest[:1]).Get("width").Float()
		ctx.Call(method, rest[:1], x+(cell-w)/2, y)
		x, rest = x+cell, rest[1:]
	}
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
		setStyle(el, map[string]string{"position": "absolute", "left": px(n.Bounds.Origin.X),
			"top": px(n.Bounds.Origin.Y), "width": px(n.Bounds.Size.Width), "height": px(n.Bounds.Size.Height)})
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
