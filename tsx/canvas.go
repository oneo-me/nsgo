package tsx

import (
	"math"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/core"
)

// Canvas (NSH's canvas.rs): CanvasRenderingContext2D records into the
// core.Drawing core's Canvas gives its Draw, Path2D builds a core.Path
// and CanvasGradient a core.CanvasGradient. Arguments that are animated
// nodes bind the operation's argument, so they animate without drawing
// again.

// canvasKey holds the Go side of a context, a path or a gradient.
var canvasKey = js.NewSymbol("nsgo.canvas")

// canvasFace is a <Canvas>'s.
func canvasFace(p *props) core.Node {
	r := p.r
	c := core.Canvas{
		Class:     p.class(),
		StyleFunc: p.styleFunc(),
		OnPress:   p.onPress("onPress"),
		OnLayout:  p.onLayout(),
		Ref:       p.n.ref,
		Label:     p.str("accessibilityLabel"),
		TestID:    p.str("testID"),
	}
	ctx := r.newContext()
	c.Draw = func(d *core.Drawing, size core.Size) {
		draw := p.get("draw")
		if !isFunction(draw) {
			return
		}
		ctx.begin(d, size)
		r.call(draw, ctx.obj, r.size(size))
	}
	return c
}

// canvas is `Canvas(props)`.
func (r *runtime) canvas(call js.FunctionCall) js.Value {
	obj := arg(call, 0).ToObject(r.vm)
	n := r.newFaceNode("Canvas", canvasFace, obj)
	r.applyRef(obj.Get("ref"), n, "")
	r.mountFace(n)
	return n.value()
}

// context2D is a CanvasRenderingContext2D's state the drawing does not
// keep: paints, the font, the shadow and the dash.
type context2D struct {
	r       *runtime
	obj     *js.Object
	d       *core.Drawing
	size    core.Size
	st      ctxState
	stack   []ctxState
	created bool
}

type ctxState struct {
	fill, stroke   js.Value
	lineWidth      float64
	lineCap        string
	lineJoin       string
	miterLimit     float64
	dash           []float32
	dashOffset     float64
	alpha          float64
	composite      string
	shadowColor    string
	shadowBlur     float64
	shadowX        float64
	shadowY        float64
	font           string
	fontVariant    string
	textAlign      string
	textBaseline   string
	parsedFont     core.Font
	parsedFontFrom string
}

func defaultState(r *runtime) ctxState {
	return ctxState{fill: r.vm.ToValue("#000"), stroke: r.vm.ToValue("#000"), lineWidth: 1, lineCap: "butt",
		lineJoin: "miter", miterLimit: 10, alpha: 1, composite: "source-over", shadowColor: "rgba(0, 0, 0, 0)",
		font: "10px sans-serif", fontVariant: "normal", textAlign: "start", textBaseline: "alphabetic"}
}

// begin points the context at a new drawing, its state reset.
func (c *context2D) begin(d *core.Drawing, size core.Size) {
	c.d, c.size, c.stack = d, size, nil
	c.st = defaultState(c.r)
}

// num reads a CanvasNumber, binding arg of the last operation when it is
// an animated node; bind runs after the operation is recorded.
func (c *context2D) num(v js.Value) (float32, core.AnimatedNode) {
	if n := c.r.animatedNodeOf(v); n != nil {
		return 0, n
	}
	return float32(v.ToFloat()), nil
}

func (c *context2D) newContextObject() *js.Object {
	r := c.r
	o := r.vm.NewObject()
	if cls := r.canvasClasses.context; cls != nil {
		_ = o.SetPrototype(cls.Get("prototype").(*js.Object))
	}
	_ = o.SetSymbol(canvasKey, c)
	method := func(name string, f func(call js.FunctionCall) js.Value) {
		_ = o.Set(name, r.function(f))
	}
	// op records an operation whose numeric arguments are args, bound to
	// the animated nodes among them by slots.
	op := func(name string, slots []core.DrawArg, record func(d *core.Drawing, x []float32)) {
		method(name, func(call js.FunctionCall) js.Value {
			x := make([]float32, len(slots))
			var nodes []core.AnimatedNode
			for i := range slots {
				v, n := c.num(arg(call, i))
				x[i] = v
				if n != nil {
					nodes = append(nodes, n)
				} else {
					nodes = append(nodes, nil)
				}
			}
			record(c.d, x)
			for i, n := range nodes {
				if n != nil {
					c.d.Bind(slots[i], n)
				}
			}
			return js.Undefined()
		})
	}
	xy := []core.DrawArg{core.ArgX, core.ArgY}
	rect := []core.DrawArg{core.ArgX, core.ArgY, core.ArgW, core.ArgH}
	op("moveTo", xy, func(d *core.Drawing, x []float32) { d.MoveTo(x[0], x[1]) })
	op("lineTo", xy, func(d *core.Drawing, x []float32) { d.LineTo(x[0], x[1]) })
	op("quadraticCurveTo", []core.DrawArg{core.ArgX1, core.ArgY1, core.ArgX, core.ArgY},
		func(d *core.Drawing, x []float32) { d.QuadTo(x[0], x[1], x[2], x[3]) })
	op("bezierCurveTo", []core.DrawArg{core.ArgX1, core.ArgY1, core.ArgX2, core.ArgY2, core.ArgX, core.ArgY},
		func(d *core.Drawing, x []float32) { d.CubicTo(x[0], x[1], x[2], x[3], x[4], x[5]) })
	method("arc", func(call js.FunctionCall) js.Value {
		slots := []core.DrawArg{core.ArgX, core.ArgY, core.ArgR, core.ArgA0, core.ArgA1}
		x := make([]float32, 5)
		nodes := make([]core.AnimatedNode, 5)
		for i := range x {
			x[i], nodes[i] = c.num(arg(call, i))
		}
		c.d.Arc(x[0], x[1], x[2], x[3], x[4], arg(call, 5).ToBoolean())
		for i, n := range nodes {
			if n != nil {
				c.d.Bind(slots[i], n)
			}
		}
		return js.Undefined()
	})
	method("arcTo", func(call js.FunctionCall) js.Value {
		x := make([]float32, 5)
		for i := range x {
			x[i], _ = c.num(arg(call, i))
		}
		c.d.ArcTo(x[0], x[1], x[2], x[3], x[4])
		return js.Undefined()
	})
	method("ellipse", func(call js.FunctionCall) js.Value {
		x := make([]float32, 7)
		for i := range x {
			x[i], _ = c.num(arg(call, i))
		}
		c.d.EllipseArc(x[0], x[1], x[2], x[3], x[4], x[5], x[6], arg(call, 7).ToBoolean())
		return js.Undefined()
	})
	op("rect", rect, func(d *core.Drawing, x []float32) { d.Rect(x[0], x[1], x[2], x[3]) })
	method("roundRect", func(call js.FunctionCall) js.Value {
		x := make([]float32, 4)
		for i := range x {
			x[i], _ = c.num(arg(call, i))
		}
		tl, tr, br, bl := c.radii(arg(call, 4))
		c.d.RoundRectCorners(x[0], x[1], x[2], x[3], tl, tr, br, bl)
		return js.Undefined()
	})
	method("closePath", func(js.FunctionCall) js.Value {
		c.d.ClosePath()
		return js.Undefined()
	})
	op("translate", xy, func(d *core.Drawing, x []float32) { d.Translate(x[0], x[1]) })
	op("scale", xy, func(d *core.Drawing, x []float32) { d.Scale(x[0], x[1]) })
	op("rotate", []core.DrawArg{core.ArgA}, func(d *core.Drawing, x []float32) { d.Rotate(x[0]) })
	six := []core.DrawArg{core.ArgX1, core.ArgY1, core.ArgX2, core.ArgY2, core.ArgX, core.ArgY}
	op("transform", six, func(d *core.Drawing, x []float32) { d.Transform(x[0], x[1], x[2], x[3], x[4], x[5]) })
	op("setTransform", six, func(d *core.Drawing, x []float32) { d.SetTransform(x[0], x[1], x[2], x[3], x[4], x[5]) })
	method("resetTransform", func(js.FunctionCall) js.Value {
		c.d.ResetTransform()
		return js.Undefined()
	})
	method("beginPath", func(js.FunctionCall) js.Value {
		c.d.BeginPath()
		return js.Undefined()
	})
	op("fillRect", rect, func(d *core.Drawing, x []float32) {
		c.applyFill()
		d.FillRect(x[0], x[1], x[2], x[3])
	})
	op("strokeRect", rect, func(d *core.Drawing, x []float32) {
		c.applyStroke()
		d.StrokeRect(x[0], x[1], x[2], x[3])
	})
	op("clearRect", rect, func(d *core.Drawing, x []float32) { d.Clear(x[0], x[1], x[2], x[3]) })
	method("save", func(js.FunctionCall) js.Value {
		c.stack = append(c.stack, c.st)
		c.st.dash = append([]float32(nil), c.st.dash...)
		c.d.Save()
		return js.Undefined()
	})
	method("restore", func(js.FunctionCall) js.Value {
		if n := len(c.stack); n > 0 {
			c.st, c.stack = c.stack[n-1], c.stack[:n-1]
			c.d.Restore()
		}
		return js.Undefined()
	})
	method("setLineDash", func(call js.FunctionCall) js.Value {
		c.st.dash = nil
		for _, v := range r.arrayItems(arg(call, 0)) {
			c.st.dash = append(c.st.dash, float32(v.ToFloat()))
		}
		if len(c.st.dash)%2 == 1 {
			c.st.dash = append(c.st.dash, c.st.dash...)
		}
		c.d.SetLineDash(c.st.dash...)
		return js.Undefined()
	})
	method("getLineDash", func(js.FunctionCall) js.Value {
		out := make([]any, len(c.st.dash))
		for i, d := range c.st.dash {
			out[i] = js32(d)
		}
		return r.vm.NewArray(out...)
	})
	method("fill", func(call js.FunctionCall) js.Value {
		path, rule := c.pathAndRule(call)
		c.applyFill()
		switch {
		case path != nil:
			c.d.FillPath(path, rule)
		case rule == core.EvenOdd:
			c.d.FillEvenOdd()
		default:
			c.d.Fill()
		}
		return js.Undefined()
	})
	method("stroke", func(call js.FunctionCall) js.Value {
		c.applyStroke()
		if path := r.pathOf(arg(call, 0)); path != nil {
			c.d.StrokePath(path)
		} else {
			c.d.Stroke()
		}
		return js.Undefined()
	})
	method("clip", func(call js.FunctionCall) js.Value {
		path, rule := c.pathAndRule(call)
		switch {
		case path != nil:
			c.d.ClipPath(path, rule)
		case rule == core.EvenOdd:
			c.d.ClipEvenOdd()
		default:
			c.d.Clip()
		}
		return js.Undefined()
	})
	text := func(stroke bool) func(call js.FunctionCall) js.Value {
		return func(call js.FunctionCall) js.Value {
			s := arg(call, 0).String()
			x, nx := c.num(arg(call, 1))
			y, ny := c.num(arg(call, 2))
			c.applyText()
			if stroke {
				c.applyStroke()
				c.d.StrokeText(s, x, y, c.font())
			} else {
				c.applyFill()
				c.d.FillText(s, x, y, c.font())
			}
			if nx != nil {
				c.d.Bind(core.ArgX, nx)
			}
			if ny != nil {
				c.d.Bind(core.ArgY, ny)
			}
			return js.Undefined()
		}
	}
	method("fillText", text(false))
	method("strokeText", text(true))
	method("measureText", func(call js.FunctionCall) js.Value {
		m := c.d.TextMetrics(arg(call, 0).String(), c.font())
		out := r.vm.NewObject()
		_ = out.Set("width", js32(m.Width))
		_ = out.Set("fontBoundingBoxAscent", js32(m.Ascent))
		_ = out.Set("fontBoundingBoxDescent", js32(m.Descent))
		return out
	})
	method("isPointInPath", func(call js.FunctionCall) js.Value {
		if path := r.pathOf(arg(call, 0)); path != nil {
			return r.vm.ToValue(c.d.PathContains(path, float32(arg(call, 1).ToFloat()), float32(arg(call, 2).ToFloat()),
				fillRule(arg(call, 3))))
		}
		return r.vm.ToValue(c.d.IsPointInPath(float32(arg(call, 0).ToFloat()), float32(arg(call, 1).ToFloat()),
			fillRule(arg(call, 2))))
	})
	method("isPointInStroke", func(call js.FunctionCall) js.Value {
		if path := r.pathOf(arg(call, 0)); path != nil {
			return r.vm.ToValue(c.d.StrokeContains(path, float32(arg(call, 1).ToFloat()), float32(arg(call, 2).ToFloat())))
		}
		return r.vm.ToValue(c.d.IsPointInStroke(float32(arg(call, 0).ToFloat()), float32(arg(call, 1).ToFloat())))
	})
	method("createLinearGradient", func(call js.FunctionCall) js.Value {
		f := floats(call, 4)
		return r.newGradient(core.CanvasLinear(f[0], f[1], f[2], f[3]))
	})
	method("createRadialGradient", func(call js.FunctionCall) js.Value {
		f := floats(call, 6)
		return r.newGradient(core.CanvasRadial(f[0], f[1], f[2], f[3], f[4], f[5]))
	})
	method("createConicGradient", func(call js.FunctionCall) js.Value {
		f := floats(call, 3)
		return r.newGradient(core.CanvasConic(f[0], f[1], f[2]))
	})
	method("drawImage", func(call js.FunctionCall) js.Value {
		src := r.imageSource(arg(call, 0))
		f := make([]float32, len(call.Arguments)-1)
		for i := range f {
			f[i], _ = c.num(arg(call, i+1))
		}
		switch len(f) {
		case 2:
			c.d.DrawImage(src, f[0], f[1])
		case 4:
			c.d.DrawImageIn(src, f[0], f[1], f[2], f[3])
		case 8:
			c.d.DrawImageFrom(src, f[0], f[1], f[2], f[3], f[4], f[5], f[6], f[7])
		}
		return js.Undefined()
	})

	prop := func(name string, get func() js.Value, set func(v js.Value)) {
		_ = o.DefineAccessorProperty(name, r.function(func(js.FunctionCall) js.Value { return get() }),
			r.function(func(call js.FunctionCall) js.Value {
				set(arg(call, 0))
				return js.Undefined()
			}), js.FLAG_FALSE, js.FLAG_TRUE)
	}
	str := func(p *string, apply func()) (func() js.Value, func(js.Value)) {
		return func() js.Value { return r.vm.ToValue(*p) }, func(v js.Value) {
			*p = v.String()
			if apply != nil {
				apply()
			}
		}
	}
	num := func(p *float64, arg core.DrawArg, apply func(x float32)) (func() js.Value, func(js.Value)) {
		return func() js.Value { return r.vm.ToValue(*p) }, func(v js.Value) {
			x, n := c.num(v)
			*p = js32(x)
			apply(x)
			if n != nil {
				c.d.Bind(arg, n)
			}
		}
	}
	prop("fillStyle", func() js.Value { return c.st.fill }, func(v js.Value) { c.st.fill = v })
	prop("strokeStyle", func() js.Value { return c.st.stroke }, func(v js.Value) { c.st.stroke = v })
	g, s := num(&c.st.lineWidth, core.ArgW, func(x float32) { c.d.SetLineWidth(x) })
	prop("lineWidth", g, s)
	g, s = num(&c.st.dashOffset, core.ArgW, func(x float32) { c.d.SetLineDashOffset(x) })
	prop("lineDashOffset", g, s)
	g, s = num(&c.st.alpha, core.ArgA, func(x float32) { c.d.SetAlpha(x) })
	prop("globalAlpha", g, s)
	g, s = num(&c.st.miterLimit, core.ArgW, func(x float32) { c.d.SetMiterLimit(x) })
	prop("miterLimit", g, s)
	g, s = str(&c.st.lineCap, func() {
		c.d.SetLineCap(map[string]core.LineCap{"butt": core.CapButt, "round": core.CapRound, "square": core.CapSquare}[c.st.lineCap])
	})
	prop("lineCap", g, s)
	g, s = str(&c.st.lineJoin, func() {
		c.d.SetLineJoin(map[string]core.LineJoin{"miter": core.JoinMiter, "round": core.JoinRound, "bevel": core.JoinBevel}[c.st.lineJoin])
	})
	prop("lineJoin", g, s)
	g, s = str(&c.st.composite, func() {
		if op, ok := compositeOps[c.st.composite]; ok {
			c.d.SetComposite(op)
		}
	})
	prop("globalCompositeOperation", g, s)
	shadow := func() {
		col, _ := core.ParseColor(c.st.shadowColor)
		c.d.SetShadow(col, float32(c.st.shadowX), float32(c.st.shadowY), float32(c.st.shadowBlur))
	}
	g, s = str(&c.st.shadowColor, shadow)
	prop("shadowColor", g, s)
	for name, p := range map[string]*float64{"shadowBlur": &c.st.shadowBlur, "shadowOffsetX": &c.st.shadowX,
		"shadowOffsetY": &c.st.shadowY} {
		prop(name, func() js.Value { return r.vm.ToValue(*p) }, func(v js.Value) {
			*p = v.ToFloat()
			shadow()
		})
	}
	g, s = str(&c.st.font, nil)
	prop("font", g, s)
	g, s = str(&c.st.fontVariant, nil)
	prop("fontVariantNumeric", g, s)
	g, s = str(&c.st.textAlign, nil)
	prop("textAlign", g, s)
	g, s = str(&c.st.textBaseline, nil)
	prop("textBaseline", g, s)
	canvas := r.vm.NewObject()
	_ = canvas.DefineAccessorProperty("width", r.function(func(js.FunctionCall) js.Value {
		return r.vm.ToValue(js32(c.size.Width))
	}), nil, js.FLAG_FALSE, js.FLAG_TRUE)
	_ = canvas.DefineAccessorProperty("height", r.function(func(js.FunctionCall) js.Value {
		return r.vm.ToValue(js32(c.size.Height))
	}), nil, js.FLAG_FALSE, js.FLAG_TRUE)
	_ = o.Set("canvas", canvas)
	return o
}

func (r *runtime) newContext() *context2D {
	c := &context2D{r: r}
	c.st = defaultState(r)
	c.obj = c.newContextObject()
	return c
}

func floats(call js.FunctionCall, n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = float32(arg(call, i).ToFloat())
	}
	return out
}

// radii reads roundRect's radii: one, or 2 to 4 as CSS's border-radius.
func (c *context2D) radii(v js.Value) (tl, tr, br, bl float32) {
	items := c.r.arrayItems(v)
	if items == nil {
		if !defined(v) {
			return
		}
		x := float32(v.ToFloat())
		return x, x, x, x
	}
	f := make([]float32, len(items))
	for i, it := range items {
		f[i] = float32(it.ToFloat())
	}
	switch len(f) {
	case 1:
		return f[0], f[0], f[0], f[0]
	case 2:
		return f[0], f[1], f[0], f[1]
	case 3:
		return f[0], f[1], f[2], f[1]
	case 4:
		return f[0], f[1], f[2], f[3]
	}
	return
}

func fillRule(v js.Value) core.FillRule {
	if stringOf(v) == "evenodd" {
		return core.EvenOdd
	}
	return core.NonZero
}

// pathAndRule reads fill's and clip's arguments: [path], [rule].
func (c *context2D) pathAndRule(call js.FunctionCall) (*core.Path, core.FillRule) {
	if p := c.r.pathOf(arg(call, 0)); p != nil {
		return p, fillRule(arg(call, 1))
	}
	return nil, fillRule(arg(call, 0))
}

// paint sets fill or stroke to v: a color, a gradient or an animated
// color.
func (c *context2D) paint(v js.Value, fill bool) {
	if g := c.r.gradientOf(v); g != nil {
		if fill {
			c.d.SetFillGradient(g)
		} else {
			c.d.SetStrokeGradient(g)
		}
		return
	}
	n := c.r.animatedNodeOf(v)
	var col core.Color
	if n == nil {
		col, _ = core.ParseColor(stringOf(v))
	}
	if fill {
		c.d.SetFillColor(col)
	} else {
		c.d.SetStrokeColor(col)
	}
	if n != nil {
		c.d.Bind(core.ArgColor, n)
	}
}

func (c *context2D) applyFill()   { c.paint(c.st.fill, true) }
func (c *context2D) applyStroke() { c.paint(c.st.stroke, false) }

func (c *context2D) applyText() {
	c.d.SetTextAlign(map[string]core.TextAlign{"start": core.TextAlignStart, "left": core.TextAlignStart,
		"center": core.TextAlignCenter, "end": core.TextAlignEnd, "right": core.TextAlignEnd}[c.st.textAlign])
	c.d.SetTextBaseline(map[string]core.TextBaseline{"alphabetic": core.BaselineAlphabetic, "top": core.BaselineTop,
		"hanging": core.BaselineHanging, "middle": core.BaselineMiddle, "ideographic": core.BaselineIdeographic,
		"bottom": core.BaselineBottom}[c.st.textBaseline])
}

// font is the context's font, parsed once per value.
func (c *context2D) font() core.Font {
	key := c.st.font + "|" + c.st.fontVariant
	if c.st.parsedFontFrom != key {
		c.st.parsedFont, c.st.parsedFontFrom = parseFont(c.st.font), key
		if v, ok := fontVariants[c.st.fontVariant]; ok {
			c.st.parsedFont.Variant = v
		}
	}
	return c.st.parsedFont
}

// parseFont reads CSS's font shorthand: [style] [variant] [weight]
// size[/line-height] family.
func parseFont(s string) core.Font {
	f := core.Font{Size: 10, Family: "sans-serif"}
	w := words(s)
	for i, word := range w {
		switch {
		case word == "italic" || word == "oblique":
			f.Italic = true
		case word == "bold":
			f.Weight = core.FontWeightBold
		case word == "normal" || word == "small-caps":
		case strings.HasSuffix(word, "px") || strings.HasSuffix(word, "pt") || strings.HasSuffix(word, "rem") ||
			strings.Contains(word, "/"):
			size, _, _ := strings.Cut(word, "/")
			if l, ok := cssLength(size); ok {
				f.Size = l
			}
			if i+1 < len(w) {
				f.Family = strings.Trim(strings.Join(w[i+1:], " "), `"'`)
			}
			return f
		default:
			if n, err := strconv.Atoi(word); err == nil {
				f.Weight = core.FontWeight(n)
			}
		}
	}
	return f
}

var compositeOps = map[string]core.CompositeOp{"source-over": core.CompositeSourceOver,
	"source-in": core.CompositeSourceIn, "source-out": core.CompositeSourceOut, "source-atop": core.CompositeSourceAtop,
	"destination-over": core.CompositeDestinationOver, "destination-in": core.CompositeDestinationIn,
	"destination-out": core.CompositeDestinationOut, "destination-atop": core.CompositeDestinationAtop,
	"lighter": core.CompositeLighter, "copy": core.CompositeCopy, "xor": core.CompositeXor,
	"multiply": core.CompositeMultiply, "screen": core.CompositeScreen, "overlay": core.CompositeOverlay,
	"darken": core.CompositeDarken, "lighten": core.CompositeLighten, "color-dodge": core.CompositeColorDodge,
	"color-burn": core.CompositeColorBurn, "hard-light": core.CompositeHardLight,
	"soft-light": core.CompositeSoftLight, "difference": core.CompositeDifference,
	"exclusion": core.CompositeExclusion, "hue": core.CompositeHue, "saturation": core.CompositeSaturation,
	"color": core.CompositeColor, "luminosity": core.CompositeLuminosity}

// canvasClasses are Path2D, CanvasRenderingContext2D and CanvasGradient.
type canvasClasses struct {
	path, gradient, context *js.Object
}

func (r *runtime) installCanvas() {
	vm := r.vm
	// A context comes from a Canvas's draw; its class is for instanceof.
	r.canvasClasses.context = vm.ToValue(func(js.ConstructorCall) *js.Object {
		panic(vm.NewTypeError("nsgo: a CanvasRenderingContext2D is a Canvas's draw's"))
	}).(*js.Object)
	path := vm.ToValue(func(call js.ConstructorCall) *js.Object {
		p := &core.Path{}
		switch a := call.Argument(0); {
		case r.pathOf(a) != nil:
			p.AddPath(r.pathOf(a))
		case defined(a):
			parsed, err := core.ParsePath(a.String())
			if err != nil {
				panic(vm.NewTypeError("nsgo: Path2D: %v", err))
			}
			p = parsed
		}
		_ = call.This.SetSymbol(canvasKey, p)
		return nil
	}).(*js.Object)
	proto := path.Get("prototype").(*js.Object)
	pm := func(name string, n int, f func(p *core.Path, x []float32, call js.FunctionCall)) {
		_ = proto.Set(name, r.function(func(call js.FunctionCall) js.Value {
			p := r.pathOf(call.This)
			if p == nil {
				panic(vm.NewTypeError("nsgo: %s of something that is not a Path2D", name))
			}
			f(p, floats(call, n), call)
			return js.Undefined()
		}))
	}
	pm("moveTo", 2, func(p *core.Path, x []float32, _ js.FunctionCall) { p.MoveTo(x[0], x[1]) })
	pm("lineTo", 2, func(p *core.Path, x []float32, _ js.FunctionCall) { p.LineTo(x[0], x[1]) })
	pm("quadraticCurveTo", 4, func(p *core.Path, x []float32, _ js.FunctionCall) { p.QuadTo(x[0], x[1], x[2], x[3]) })
	pm("bezierCurveTo", 6, func(p *core.Path, x []float32, _ js.FunctionCall) {
		p.CubicTo(x[0], x[1], x[2], x[3], x[4], x[5])
	})
	pm("arc", 5, func(p *core.Path, x []float32, call js.FunctionCall) {
		p.Arc(x[0], x[1], x[2], x[3], x[4], arg(call, 5).ToBoolean())
	})
	pm("arcTo", 5, func(p *core.Path, x []float32, _ js.FunctionCall) { p.ArcTo(x[0], x[1], x[2], x[3], x[4]) })
	pm("ellipse", 7, func(p *core.Path, x []float32, call js.FunctionCall) {
		p.EllipseArc(x[0], x[1], x[2], x[3], x[4], x[5], x[6], arg(call, 7).ToBoolean())
	})
	pm("rect", 4, func(p *core.Path, x []float32, _ js.FunctionCall) { p.Rect(x[0], x[1], x[2], x[3]) })
	pm("roundRect", 4, func(p *core.Path, x []float32, call js.FunctionCall) {
		var c context2D
		c.r = r
		tl, tr, br, bl := c.radii(arg(call, 4))
		p.RoundRectCorners(x[0], x[1], x[2], x[3], tl, tr, br, bl)
	})
	pm("closePath", 0, func(p *core.Path, _ []float32, _ js.FunctionCall) { p.ClosePath() })
	pm("addPath", 0, func(p *core.Path, _ []float32, call js.FunctionCall) {
		if q := r.pathOf(arg(call, 0)); q != nil {
			p.AddPath(q)
		}
	})
	r.canvasClasses.path = path

	grad := vm.ToValue(func(call js.ConstructorCall) *js.Object {
		panic(vm.NewTypeError("nsgo: CanvasGradient is made by a context's create…Gradient"))
	}).(*js.Object)
	gproto := grad.Get("prototype").(*js.Object)
	_ = gproto.Set("addColorStop", r.function(func(call js.FunctionCall) js.Value {
		g := r.gradientOf(call.This)
		if g == nil {
			panic(vm.NewTypeError("nsgo: addColorStop of something that is not a CanvasGradient"))
		}
		off := arg(call, 0).ToFloat()
		if off < 0 || off > 1 || math.IsNaN(off) {
			panic(vm.NewTypeError("nsgo: the offset %v is outside 0 to 1", off))
		}
		col, err := core.ParseColor(arg(call, 1).String())
		if err != nil {
			panic(vm.NewTypeError("nsgo: %v", err))
		}
		g.Stops = append(g.Stops, core.GradientStop{Offset: float32(off), Color: col})
		return js.Undefined()
	}))
	r.canvasClasses.gradient = grad
}

func (r *runtime) newGradient(g *core.CanvasGradient) *js.Object {
	o := r.vm.NewObject()
	_ = o.SetPrototype(r.canvasClasses.gradient.Get("prototype").(*js.Object))
	_ = o.SetSymbol(canvasKey, g)
	return o
}

func (r *runtime) symbolValue(v js.Value) any {
	o, ok := v.(*js.Object)
	if !ok {
		return nil
	}
	s := o.GetSymbol(canvasKey)
	if s == nil {
		return nil
	}
	return s.Export()
}

func (r *runtime) pathOf(v js.Value) *core.Path {
	p, _ := r.symbolValue(v).(*core.Path)
	return p
}

func (r *runtime) gradientOf(v js.Value) *core.CanvasGradient {
	g, _ := r.symbolValue(v).(*core.CanvasGradient)
	return g
}

// canvasExports are nsgo's canvas classes.
var canvasExports = []export{
	fn("Canvas", (*runtime).canvas),
	{"Path2D", func(r *runtime) js.Value { return r.canvasClasses.path }},
	{"CanvasGradient", func(r *runtime) js.Value { return r.canvasClasses.gradient }},
	{"CanvasRenderingContext2D", func(r *runtime) js.Value { return r.canvasClasses.context }},
}
