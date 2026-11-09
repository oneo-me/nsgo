//go:build js && wasm

// Package canvas2d plays NSGo's drawings into a canvas's 2D context over
// syscall/js: a browser's <canvas> on the web, and a WeChat mini
// program's offscreen canvas, which has the same context.
package canvas2d

import (
	"math"
	"strings"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/webstyle"
)

var (
	lineCaps  = map[driver.LineCap]string{driver.CapButt: "butt", driver.CapRound: "round", driver.CapSquare: "square"}
	lineJoins = map[driver.LineJoin]string{driver.JoinMiter: "miter", driver.JoinRound: "round", driver.JoinBevel: "bevel"}
)

// Player plays a drawing into a 2D context, which draws as NSGo's: the
// context already does what the ops say, so most map one to one.
type Player struct {
	Ctx js.Value
	// Ratio is the canvas's pixels per point, and Size its diagonal in
	// points, which repeating gradients cover.
	Ratio, Size float64
	// Font is the context's font shorthand of a font.
	Font func(driver.Font) string
	// Image returns the loaded image an op draws and its pixels per
	// point; false while it loads, when the op draws nothing.
	Image func(driver.ImageSource) (img js.Value, density float64, ok bool)
	// fill and stroke are the gradients painting, whose own transforms
	// apply as they paint, and width the line width; saved and widths
	// keep them for Restore.
	fill, stroke *driver.CanvasGradient
	saved        []*driver.CanvasGradient
	width        float64
	widths       []float64
}

// NewPlayer returns a player into ctx, whose canvas has ratio pixels per
// point and a diagonal of size points.
func NewPlayer(ctx js.Value, ratio, size float64) *Player {
	return &Player{Ctx: ctx, Ratio: ratio, Size: size, width: 1}
}

// Play plays op.
func (p *Player) Play(op *driver.DrawOp) {
	ctx := p.Ctx
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
		r := p.Ratio
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
		ctx.Set("fillStyle", webstyle.Color(op.Color))
	case driver.DrawStrokeColor:
		p.stroke = nil
		ctx.Set("strokeStyle", webstyle.Color(op.Color))
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
		ctx.Set("shadowColor", webstyle.Color(op.Color))
		ctx.Set("shadowOffsetX", f(op.X)*p.Ratio)
		ctx.Set("shadowOffsetY", f(op.Y)*p.Ratio)
		ctx.Set("shadowBlur", f(op.R)*p.Ratio)
	case driver.DrawFillText:
		ctx.Set("font", p.Font(op.Font))
		x, y := f(op.X+op.X1), f(op.Y+op.Y1)
		p.paint(p.fill, func() { drawText(ctx, "fillText", op, x, y) }, false)
	case driver.DrawStrokeText:
		ctx.Set("font", p.Font(op.Font))
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
func (p *Player) paint(g *driver.CanvasGradient, draw func(), stroke bool) {
	if g == nil || g.Transform == ([6]float32{}) || !g.Radial && !g.Conic {
		draw()
		return
	}
	m := g.Matrix()
	ctx := p.Ctx
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
func (p *Player) gradient(g *driver.CanvasGradient) js.Value {
	if g == nil || len(g.Stops) == 0 {
		return js.Null()
	}
	f := func(x float32) float64 { return float64(x) }
	stops := g.Stops
	var gr js.Value
	switch {
	case g.Conic:
		gr = p.Ctx.Call("createConicGradient", f(g.Angle), f(g.X1), f(g.Y1))
	case g.Radial:
		gr = p.Ctx.Call("createRadialGradient", f(g.X1), f(g.Y1), math.Max(f(g.R1), 0), f(g.X2), f(g.Y2),
			math.Max(f(g.R2), 0))
	default:
		x1, y1, x2, y2 := linearInUserSpace(g)
		if g.Spread != driver.SpreadPad {
			x1, y1, x2, y2, stops = p.repeat(g, x1, y1, x2, y2)
		}
		gr = p.Ctx.Call("createLinearGradient", x1, y1, x2, y2)
	}
	for _, s := range stops {
		gr.Call("addColorStop", math.Min(math.Max(f(s.Offset), 0), 1), webstyle.Color(s.Color))
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
func (p *Player) repeat(g *driver.CanvasGradient, x1, y1, x2, y2 float64) (float64, float64, float64, float64, []driver.GradientStop) {
	period := math.Hypot(x2-x1, y2-y1)
	if period <= 0 {
		return x1, y1, x2, y2, g.Stops
	}
	n := int(min(math.Ceil(p.Size/period)+1, 32))
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

// image draws op's image once it has loaded.
func (p *Player) image(op *driver.DrawOp) {
	if op.Image == nil {
		return
	}
	img, density, ok := p.Image(*op.Image)
	if !ok {
		return
	}
	f := func(x float32) float64 { return float64(x) }
	// A page's images have natural sizes; a mini program's canvas images
	// only their own.
	nw, nh := img.Get("width").Float(), img.Get("height").Float()
	if n := img.Get("naturalWidth"); n.Type() == js.TypeNumber {
		nw, nh = n.Float(), img.Get("naturalHeight").Float()
	}
	if nw <= 0 || nh <= 0 {
		return
	}
	w, h := f(op.W), f(op.H)
	if w == 0 && h == 0 {
		w, h = nw/density, nh/density
	}
	if op.X2 == 0 && op.Y2 == 0 {
		p.Ctx.Call("drawImage", img, f(op.X), f(op.Y), w, h)
		return
	}
	k := density
	p.Ctx.Call("drawImage", img, f(op.X1)*k, f(op.Y1)*k, f(op.X2)*k, f(op.Y2)*k, f(op.X), f(op.Y), w, h)
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
