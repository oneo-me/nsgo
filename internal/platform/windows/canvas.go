//go:build windows

package windows

import (
	"context"
	"log"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

// A canvas plays its drawing as the web's 2D context does: the player
// builds the path in the view's points, as each operation's transform
// maps it, and has Direct2D fill, stroke or clip to it. Shadows are what
// is drawn recorded on the second context and drawn again through the
// shadow effect. A drawing that clears or mixes with another operator
// than source-over plays into a bitmap of its own, so it mixes with what
// it drew rather than with the views behind it: each such operation is
// recorded and mixed with a copy of the bitmap by the composite or blend
// effect, the result copied over the bitmap inside the clip.

const (
	rtCreateBitmapBrush   = 7
	rtFlush               = 42
	dcSetPrimitiveBlend   = 78
	dcGetImageLocalBounds = 70
	brushSetOpacity       = 4
	brushSetTransform     = 5
	bitmapCopyFromBitmap  = 8
	layoutDraw            = 58
	fontFaceGlyphOutline  = 14

	d2dExtendWrap   = 1
	d2dExtendMirror = 2
	d2dPropertyEnum = 11
	d2dBlendCopy    = 1
	compositeCopy   = 10 // D2D1_COMPOSITE_MODE_SOURCE_COPY
	compositeIn     = 2  // SOURCE_IN
	compositeDstOut = 5  // DESTINATION_OUT
	compositePlus   = 9
)

var (
	clsidComposite = parseGUID("48fc9f51-f6ac-48f1-8b58-3b28ac46f76d")
	clsidBlend     = parseGUID("81c5b77b-13f8-4cdd-ad20-c890547ac65d")
)

// D2D1_COMPOSITE_MODE of each Porter-Duff CompositeOp.
var compositeModes = [...]uint32{
	driver.CompositeSourceOver: 0, driver.CompositeDestinationOver: 1, driver.CompositeSourceIn: 2,
	driver.CompositeDestinationIn: 3, driver.CompositeSourceOut: 4, driver.CompositeDestinationOut: 5,
	driver.CompositeSourceAtop: 6, driver.CompositeDestinationAtop: 7, driver.CompositeXor: 8,
	driver.CompositeLighter: 9, driver.CompositeCopy: 10,
}

// D2D1_BLEND_MODE of each blend CompositeOp, from CompositeMultiply.
var blendModes = [...]uint32{0, 1, 11, 2, 3, 9, 5, 13, 12, 18, 19, 20, 21, 22, 23}

// canvasState is what Save and Restore keep.
type canvasState struct {
	ctm                  matrix // the drawing's transform, to the view's points
	fill, stroke         driver.Color
	fillGrad, strokeGrad *driver.CanvasGradient
	width, miter         float32
	cap                  driver.LineCap
	join                 driver.LineJoin
	dash                 []float32
	dashOffset           float32
	alpha                float32
	composite            driver.CompositeOp
	shadow               driver.DrawOp // DrawShadow, zero for none
	clips                []clipShape   // the clips pushed since the canvas began
}

// clipShape is a clip the canvas pushed: its geometry in the view's
// points.
type clipShape struct{ g *comObject }

type segKind uint8

const (
	segMove segKind = iota
	segLine
	segCubic
	segClose
)

// segment is a piece of the path, in the view's points.
type segment struct {
	kind segKind
	pts  [3]pointF
}

// canvasPlayer plays a Drawing into the painter.
type canvasPlayer struct {
	p       *painter
	v       *view
	world   matrix // the view's, or the identity into a bitmap of its own
	width   float32
	height  float32
	st      canvasState
	stack   []canvasState
	path    []segment
	cur     pointF
	start   pointF
	has     bool
	strokes map[string]*comObject
	// off is the bitmap the canvas plays into when it mixes.
	off *comObject
	// split is what a list played over the static part leaves out
	// (canvascache.go).
	split driver.CanvasSplit
}

// changedCanvases draws again the canvases whose bound arguments changed.
func (w *window) changedCanvases() {
	w.animator.ChangedCanvases(func(id driver.NodeID) {
		if v := w.view(id); v != nil {
			v.canvasCache.argsChanged()
			w.redrawView(v)
		}
	})
}

// changedNatives hands components the animated props that changed.
func (w *window) changedNatives() {
	w.animator.ChangedNatives(func(id driver.NodeID, b driver.NativeBinding, num float64, col driver.Color) {
		if v := w.view(id); v != nil && v.component != nil {
			driver.SetAnimatedProp(v.component.c, b, num, col)
		}
	})
}

func (p *painter) newCanvasPlayer(v *view, world matrix, width, height float32) *canvasPlayer {
	return &canvasPlayer{p: p, v: v, world: world, width: width, height: height,
		st:      canvasState{ctm: identity, fill: 0x000000ff, stroke: 0x000000ff, width: 1, miter: 10, alpha: 1},
		strokes: map[string]*comObject{}}
}

func (c *canvasPlayer) playAll(ops []driver.DrawOp) {
	for i := range ops {
		if !c.split.Skips(i, &ops[i]) {
			c.play(&ops[i])
		}
	}
	for _, s := range append(c.stack, c.st) {
		for _, cl := range s.clips {
			c.p.pop()
			cl.g.release()
		}
	}
	for _, s := range c.strokes {
		s.release()
	}
	c.p.setTransform(c.world)
}

// canvasOffscreen plays ops into a bitmap of the canvas's own and draws
// it; false when there is no bitmap.
func (p *painter) canvasOffscreen(v *view, ops []driver.DrawOp, width, height float32) bool {
	pw, ph := uint32(math.Ceil(float64(width*p.scale))), uint32(math.Ceil(float64(height*p.scale)))
	if pw == 0 || ph == 0 || pw*ph > 1<<24 {
		return false
	}
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaPremultiplied, dpiX: 96 * p.scale,
		dpiY: 96 * p.scale, options: d2dBitmapTarget}
	var off *comObject
	if failed(p.dc.callSize(dcCreateBitmap, pw, ph, 0, 0, uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&off)))) {
		return false
	}
	defer off.release()
	world := p.m
	p.flatten(func() {
		var target *comObject
		p.dc.call(dcGetTarget, uintptr(unsafe.Pointer(&target)))
		p.dc.call(dcSetTarget, off.ptr())
		pushed, split := p.pushed, p.split
		p.pushed, p.split = nil, nil
		p.setTransform(identity)
		clear := colorF{}
		p.dc.call(rtClear, uintptr(unsafe.Pointer(&clear)))
		c := p.newCanvasPlayer(v, identity, width, height)
		c.off = off
		c.playAll(ops)
		for len(p.pushed) > 0 {
			p.pop()
		}
		p.pushed, p.split = pushed, split
		p.dc.call(dcSetTarget, target.ptr())
		target.release()
	})
	p.setTransform(world)
	dst := rectF{0, 0, width, height}
	p.dc.callF(rtDrawBitmap, 0b100, off.ptr(), uintptr(unsafe.Pointer(&dst)), f32(1), d2dInterpolationLinear, 0)
	return true
}

func (c *canvasPlayer) play(op *driver.DrawOp) {
	x, y := float64(op.X), float64(op.Y)
	f := func(v float32) float64 { return float64(v) }
	switch op.Kind {
	case driver.DrawSave:
		c.stack = append(c.stack, c.st)
		c.st.clips = nil
	case driver.DrawRestore:
		if n := len(c.stack); n > 0 {
			for range c.st.clips {
				c.p.pop()
			}
			for _, cl := range c.st.clips {
				cl.g.release()
			}
			c.st = c.stack[n-1]
			c.stack = c.stack[:n-1]
		}
	case driver.DrawTranslate:
		c.st.ctm = translation(x, y).then(c.st.ctm)
	case driver.DrawScale:
		c.st.ctm = scaling(x, y).then(c.st.ctm)
	case driver.DrawRotate:
		sin, cos := math.Sincos(float64(op.A))
		c.st.ctm = matrix{a: cos, b: sin, c: -sin, d: cos}.then(c.st.ctm)
	case driver.DrawTransform:
		c.st.ctm = matrix{f(op.X1), f(op.Y1), f(op.X2), f(op.Y2), x, y}.then(c.st.ctm)
	case driver.DrawSetTransform:
		c.st.ctm = matrix{f(op.X1), f(op.Y1), f(op.X2), f(op.Y2), x, y}
	case driver.DrawBeginPath:
		c.path, c.has = c.path[:0], false
	case driver.DrawMoveTo:
		c.moveTo(x, y)
	case driver.DrawLineTo:
		c.lineTo(x, y)
	case driver.DrawQuadTo:
		// The cubic with the same curve; affine maps keep it.
		if !c.has {
			c.moveTo(f(op.X1), f(op.Y1))
		}
		q := c.toView(f(op.X1), f(op.Y1))
		e := c.toView(x, y)
		c1 := pointF{c.cur.x + 2.0/3*(q.x-c.cur.x), c.cur.y + 2.0/3*(q.y-c.cur.y)}
		c2 := pointF{e.x + 2.0/3*(q.x-e.x), e.y + 2.0/3*(q.y-e.y)}
		c.cubic(c1, c2, e)
	case driver.DrawCubicTo:
		if !c.has {
			c.moveTo(f(op.X1), f(op.Y1))
		}
		c.cubic(c.toView(f(op.X1), f(op.Y1)), c.toView(f(op.X2), f(op.Y2)), c.toView(x, y))
	case driver.DrawArc:
		c.arc(op)
	case driver.DrawRect:
		w, h := f(op.W), f(op.H)
		c.moveTo(x, y)
		c.lineTo(x+w, y)
		c.lineTo(x+w, y+h)
		c.lineTo(x, y+h)
		c.closePath()
		c.moveTo(x, y)
	case driver.DrawRoundRect:
		w, h := f(op.W), f(op.H)
		r := max(min(f(op.R), math.Abs(w)/2, math.Abs(h)/2), 0)
		c.moveTo(x+r, y)
		c.lineTo(x+w-r, y)
		c.ellipseArc(x+w-r, y+r, r, r, -math.Pi/2, math.Pi/2)
		c.lineTo(x+w, y+h-r)
		c.ellipseArc(x+w-r, y+h-r, r, r, 0, math.Pi/2)
		c.lineTo(x+r, y+h)
		c.ellipseArc(x+r, y+h-r, r, r, math.Pi/2, math.Pi/2)
		c.lineTo(x, y+r)
		c.ellipseArc(x+r, y+r, r, r, math.Pi, math.Pi/2)
		c.closePath()
	case driver.DrawEllipse:
		rx, ry := f(op.R), f(op.RY)
		c.moveTo(x+rx, y)
		c.ellipseArc(x, y, rx, ry, 0, 2*math.Pi)
		c.closePath()
	case driver.DrawClosePath:
		c.closePath()
	case driver.DrawFill:
		c.fill(op.EvenOdd)
	case driver.DrawStroke:
		c.stroke()
	case driver.DrawClip:
		if g := c.geometry(op.EvenOdd); g != nil {
			c.p.pushLayer(1, g)
			c.st.clips = append(c.st.clips, clipShape{g})
		}
	case driver.DrawFillColor:
		c.st.fill, c.st.fillGrad = op.Color, nil
	case driver.DrawStrokeColor:
		c.st.stroke, c.st.strokeGrad = op.Color, nil
	case driver.DrawFillGradient:
		if op.Gradient != nil && len(op.Gradient.Stops) > 0 {
			c.st.fillGrad = op.Gradient
		}
	case driver.DrawStrokeGradient:
		if op.Gradient != nil && len(op.Gradient.Stops) > 0 {
			c.st.strokeGrad = op.Gradient
		}
	case driver.DrawLineWidth:
		c.st.width = op.W
	case driver.DrawLineCap:
		c.st.cap = op.Cap
	case driver.DrawLineJoin:
		c.st.join = op.Join
	case driver.DrawMiterLimit:
		c.st.miter = op.W
	case driver.DrawLineDash:
		c.st.dash = nil
		if op.Dash != nil {
			c.st.dash = *op.Dash
		}
	case driver.DrawLineDashOffset:
		c.st.dashOffset = op.W
	case driver.DrawAlpha:
		c.st.alpha = min(max(op.A, 0), 1)
	case driver.DrawComposite:
		c.st.composite = op.Composite
	case driver.DrawShadow:
		c.st.shadow = *op
	case driver.DrawFillText, driver.DrawStrokeText:
		c.text(op)
	case driver.DrawImage:
		c.image(op)
	case driver.DrawClear:
		c.clear(op)
	}
}

func (c *canvasPlayer) toView(x, y float64) pointF {
	px, py := c.st.ctm.apply(x, y)
	return pointF{float32(px), float32(py)}
}

func (c *canvasPlayer) moveTo(x, y float64) {
	c.cur = c.toView(x, y)
	c.start, c.has = c.cur, true
	c.path = append(c.path, segment{kind: segMove, pts: [3]pointF{c.cur}})
}

func (c *canvasPlayer) lineTo(x, y float64) {
	if !c.has {
		c.moveTo(x, y)
		return
	}
	c.cur = c.toView(x, y)
	c.path = append(c.path, segment{kind: segLine, pts: [3]pointF{c.cur}})
}

func (c *canvasPlayer) cubic(c1, c2, end pointF) {
	c.cur = end
	c.path = append(c.path, segment{kind: segCubic, pts: [3]pointF{c1, c2, end}})
}

func (c *canvasPlayer) closePath() {
	if c.has {
		c.path = append(c.path, segment{kind: segClose})
		c.cur = c.start
	}
}

// ellipseArc adds Bézier curves along an ellipse around (cx, cy) from
// angle a0 by sweep, from where the path is.
func (c *canvasPlayer) ellipseArc(cx, cy, rx, ry, a0, sweep float64) {
	n := max(int(math.Ceil(math.Abs(sweep)/(math.Pi/2)-1e-6)), 1)
	step := sweep / float64(n)
	k := 4.0 / 3 * math.Tan(step/4)
	a := a0
	for range n {
		sin0, cos0 := math.Sincos(a)
		sin1, cos1 := math.Sincos(a + step)
		c.cubic(
			c.toView(cx+rx*(cos0-k*sin0), cy+ry*(sin0+k*cos0)),
			c.toView(cx+rx*(cos1+k*sin1), cy+ry*(sin1-k*cos1)),
			c.toView(cx+rx*cos1, cy+ry*sin1))
		a += step
	}
}

// arc adds an arc as the web's arc() does: joined to the path by a line.
func (c *canvasPlayer) arc(op *driver.DrawOp) {
	x, y, r := float64(op.X), float64(op.Y), max(float64(op.R), 0)
	a0, a1 := float64(op.A0), float64(op.A1)
	const full = 2 * math.Pi
	mod := func(v float64) float64 { return v - full*math.Floor(v/full) }
	var sweep float64
	switch {
	case !op.CCW && a1-a0 >= full:
		sweep = full
	case !op.CCW:
		sweep = mod(a1 - a0)
	case a0-a1 >= full:
		sweep = -full
	default:
		sweep = -mod(a0 - a1)
	}
	sx, sy := x+r*math.Cos(a0), y+r*math.Sin(a0)
	if c.has {
		c.lineTo(sx, sy)
	} else {
		c.moveTo(sx, sy)
	}
	if sweep != 0 {
		c.ellipseArc(x, y, r, r, a0, sweep)
	}
}

// geometry returns the path as a Direct2D geometry filled by the nonzero
// rule, or the even-odd one, or nil when it is empty.
func (c *canvasPlayer) geometry(evenOdd bool) *comObject {
	if len(c.path) == 0 {
		return nil
	}
	mode := uintptr(d2dFillWinding)
	if evenOdd {
		mode = d2dFillAlternate
	}
	return c.p.pathGeometry(mode, func(sink *comObject) {
		open := false
		var start pointF
		for i := range c.path {
			s := &c.path[i]
			switch s.kind {
			case segMove:
				if open {
					sink.call(sinkEndFigure, d2dFigureOpen)
				}
				start = s.pts[0]
				sink.callPoint(sinkBeginFigure, start.x, start.y, d2dFigureFilled)
				open = true
			case segLine:
				if open {
					sink.call(sinkAddLines, uintptr(unsafe.Pointer(&s.pts[0])), 1)
				}
			case segCubic:
				if open {
					b := bezierSegment{s.pts[0], s.pts[1], s.pts[2]}
					sink.call(sinkAddBeziers, uintptr(unsafe.Pointer(&b)), 1)
				}
			case segClose:
				if open {
					sink.call(sinkEndFigure, d2dFigureClosed)
					open = false
					// What follows a close starts where the figure did.
					if i+1 < len(c.path) && c.path[i+1].kind != segMove {
						sink.callPoint(sinkBeginFigure, start.x, start.y, d2dFigureFilled)
						open = true
					}
				}
			}
		}
		if open {
			sink.call(sinkEndFigure, d2dFigureOpen)
		}
	})
}

// color applies the global alpha to col.
func (c *canvasPlayer) color(col driver.Color) driver.Color {
	a := math.Round(float64(col&0xff) * float64(c.st.alpha))
	return col&^0xff | driver.Color(a)
}

// brush returns a brush of col, or of g when not nil, under the global
// alpha, its space mapped to the view's points by to; the caller
// releases it. Nil paints nothing.
func (c *canvasPlayer) brush(col driver.Color, g *driver.CanvasGradient, to matrix) *comObject {
	props := struct {
		opacity   float32
		transform matrix3x2
	}{opacity: c.st.alpha, transform: identity.d2d()}
	if g == nil {
		col = c.color(col)
		if !visible(col) {
			return nil
		}
		cf := colorOf(col)
		var b *comObject
		c.p.dc.call(rtCreateSolidColorBrush, uintptr(unsafe.Pointer(&cf)), 0, uintptr(unsafe.Pointer(&b)))
		return b
	}
	m := g.Matrix()
	gm := matrix{float64(m[0]), float64(m[1]), float64(m[2]), float64(m[3]), float64(m[4]), float64(m[5])}
	props.transform = gm.then(to).d2d()
	if g.Conic {
		return c.conicBrush(g, gm.then(to), props.opacity)
	}
	stops := make([]gradientStop, len(g.Stops))
	r1, r2 := max(g.R1, 0), max(g.R2, 0)
	for i, s := range g.Stops {
		at := s.Offset
		if g.Radial && r2 > 0 {
			// Direct2D's radial gradients start at a point: the focal
			// circle moves the stops out.
			at = (r1 + s.Offset*(r2-r1)) / r2
		}
		stops[i] = gradientStop{min(max(at, 0), 1), colorOf(s.Color)}
	}
	extend := uintptr(d2dExtendClamp)
	switch g.Spread {
	case driver.SpreadRepeat:
		extend = d2dExtendWrap
	case driver.SpreadReflect:
		extend = d2dExtendMirror
	}
	var collection, b *comObject
	if failed(c.p.dc.call(rtCreateGradientStopCollection, uintptr(unsafe.Pointer(&stops[0])), uintptr(len(stops)),
		d2dGamma22, extend, uintptr(unsafe.Pointer(&collection)))) {
		return nil
	}
	defer collection.release()
	if g.Radial {
		radial := struct {
			center, offset pointF
			rx, ry         float32
		}{center: pointF{g.X2, g.Y2}, offset: pointF{g.X1 - g.X2, g.Y1 - g.Y2}, rx: max(r2, 1e-3), ry: max(r2, 1e-3)}
		c.p.dc.call(rtCreateRadialGradientBrush, uintptr(unsafe.Pointer(&radial)), uintptr(unsafe.Pointer(&props)),
			collection.ptr(), uintptr(unsafe.Pointer(&b)))
		return b
	}
	linear := struct{ start, end pointF }{pointF{g.X1, g.Y1}, pointF{g.X2, g.Y2}}
	c.p.dc.call(rtCreateLinearGradientBrush, uintptr(unsafe.Pointer(&linear)), uintptr(unsafe.Pointer(&props)),
		collection.ptr(), uintptr(unsafe.Pointer(&b)))
	return b
}

// conicBrush paints a conic gradient, which Direct2D has no brush for,
// as a bitmap of its pixels over the canvas, its space mapped to the
// view's points by to.
func (c *canvasPlayer) conicBrush(g *driver.CanvasGradient, to matrix, opacity float32) *comObject {
	scale := c.p.scale
	pw, ph := int(math.Ceil(float64(c.width*scale))), int(math.Ceil(float64(c.height*scale)))
	if pw <= 0 || ph <= 0 || pw*ph > 4<<20 {
		return nil
	}
	inv := to.invert()
	pix := make([]byte, pw*ph*4)
	for y := range ph {
		for x := range pw {
			gx, gy := inv.apply((float64(x)+0.5)/float64(scale), (float64(y)+0.5)/float64(scale))
			a := math.Atan2(gy-float64(g.Y1), gx-float64(g.X1)) - float64(g.Angle)
			t := math.Mod(a+4*math.Pi, 2*math.Pi) / (2 * math.Pi)
			r, gg, bl, al := g.At(t).Components()
			i := (y*pw + x) * 4
			pix[i], pix[i+1], pix[i+2], pix[i+3] = byte(bl*al*255+0.5), byte(gg*al*255+0.5), byte(r*al*255+0.5), byte(al*255+0.5)
		}
	}
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaPremultiplied, dpiX: 96, dpiY: 96}
	var bmp, b *comObject
	if failed(c.p.dc.callSize(dcCreateBitmap, uint32(pw), uint32(ph), uintptr(unsafe.Pointer(&pix[0])), uintptr(pw*4),
		uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&bmp)))) {
		return nil
	}
	defer bmp.release()
	brushProps := struct {
		opacity   float32
		transform matrix3x2
	}{opacity, scaling(1/float64(scale), 1/float64(scale)).d2d()}
	bitmapProps := struct{ x, y, interpolation uint32 }{d2dExtendClamp, d2dExtendClamp, d2dInterpolationLinear}
	c.p.dc.call(rtCreateBitmapBrush, bmp.ptr(), uintptr(unsafe.Pointer(&bitmapProps)), uintptr(unsafe.Pointer(&brushProps)),
		uintptr(unsafe.Pointer(&b)))
	return b
}

func (c *canvasPlayer) fill(evenOdd bool) {
	g := c.geometry(evenOdd)
	if g == nil {
		return
	}
	defer g.release()
	b := c.brush(c.st.fill, c.st.fillGrad, c.st.ctm)
	if b == nil {
		return
	}
	defer b.release()
	c.paint(func(dc *comObject, base matrix) {
		c.setTransform(dc, base)
		dc.call(rtFillGeometry, g.ptr(), b.ptr(), 0)
	})
}

func (c *canvasPlayer) stroke() {
	width := float32(float64(c.st.width) * c.st.ctm.stretch())
	if width <= 0 {
		return
	}
	g := c.geometry(false)
	if g == nil {
		return
	}
	defer g.release()
	b := c.brush(c.st.stroke, c.st.strokeGrad, c.st.ctm)
	if b == nil {
		return
	}
	defer b.release()
	style := c.strokeStyle()
	c.paint(func(dc *comObject, base matrix) {
		c.setTransform(dc, base)
		dc.callF(rtDrawGeometry, 0b100, g.ptr(), b.ptr(), f32(width), style.ptr())
	})
}

// setTransform sets dc's transform to base, the view's points.
func (c *canvasPlayer) setTransform(dc *comObject, base matrix) {
	if dc == c.p.dc {
		c.p.setTransform(base)
		return
	}
	t := base.d2d()
	dc.call(rtSetTransform, uintptr(unsafe.Pointer(&t)))
}

// strokeStyle returns the stroke style of the line's ends, joins, miter
// limit and dashes, made once per pass.
func (c *canvasPlayer) strokeStyle() *comObject {
	lineCap := uint32(d2dCapFlat)
	switch c.st.cap {
	case driver.CapRound:
		lineCap = d2dCapRound
	case driver.CapSquare:
		lineCap = d2dCapSquare
	}
	join := uint32(d2dJoinMiterOrBevel)
	switch c.st.join {
	case driver.JoinRound:
		join = d2dJoinRound
	case driver.JoinBevel:
		join = d2dJoinBevel
	}
	// Direct2D's dashes are in line widths.
	var dashes []float32
	w := max(c.st.width, 1e-3)
	for _, d := range c.st.dash {
		dashes = append(dashes, d/w)
	}
	var key strings.Builder
	for _, v := range []float32{float32(lineCap), float32(join), c.st.miter, c.st.dashOffset / w} {
		key.WriteString(strconv.FormatFloat(float64(v), 'g', -1, 32))
		key.WriteByte(',')
	}
	for _, d := range dashes {
		key.WriteString(strconv.FormatFloat(float64(d), 'g', -1, 32))
		key.WriteByte(' ')
	}
	if s, ok := c.strokes[key.String()]; ok {
		return s
	}
	props := strokeStyleProperties{startCap: lineCap, endCap: lineCap, dashCap: lineCap, lineJoin: join,
		miterLimit: max(c.st.miter, 1)}
	var dashPtr uintptr
	if len(dashes) > 0 && dashes[0]+dashes[len(dashes)-1] > 0 {
		props.dashStyle, props.dashOffset = d2dDashCustom, c.st.dashOffset/w
		dashPtr = uintptr(unsafe.Pointer(&dashes[0]))
	} else {
		dashes = nil
	}
	var s *comObject
	if failed(c.p.g.factory.call(d2dFactoryCreateStrokeStyle, uintptr(unsafe.Pointer(&props)), dashPtr,
		uintptr(len(dashes)), uintptr(unsafe.Pointer(&s)))) {
		s = nil
	}
	c.strokes[key.String()] = s
	return s
}

// paint runs draw, which draws a shape on the context it is given with
// the view's points as base: after its shadow, through the effect, and
// mixed by the operator into a canvas bitmap of its own.
func (c *canvasPlayer) paint(draw func(dc *comObject, base matrix)) {
	if sh := c.st.shadow; visible(sh.Color) && (sh.X != 0 || sh.Y != 0 || sh.R != 0) {
		if list := c.record(draw); list != nil {
			c.drawShadow(list, sh)
			list.release()
		}
	}
	if c.off != nil && c.st.composite != driver.CompositeSourceOver {
		if list := c.record(draw); list != nil {
			c.mix(list)
			list.release()
		}
		return
	}
	draw(c.p.dc, c.world)
	c.p.setTransform(c.world)
}

// record records draw on the second context, in the view's points.
func (c *canvasPlayer) record(draw func(dc *comObject, base matrix)) *comObject {
	bake := c.p.g.bake
	var list *comObject
	if failed(bake.call(dcCreateCommandList, uintptr(unsafe.Pointer(&list)))) {
		return nil
	}
	bake.call(dcSetTarget, list.ptr())
	bake.callF(rtSetDpi, 0b11, f32(96*c.p.scale), f32(96*c.p.scale))
	bake.call(rtBeginDraw)
	draw(bake, identity)
	hr := bake.call(rtEndDraw, 0, 0)
	bake.call(dcSetTarget, 0)
	if failed(hr) || failed(list.call(commandListClose)) {
		list.release()
		return nil
	}
	return list
}

// drawShadow draws what list recorded through the shadow effect, offset
// in the canvas's points whatever its transform.
func (c *canvasPlayer) drawShadow(list *comObject, sh driver.DrawOp) {
	var e, out *comObject
	if failed(c.p.dc.call(dcCreateEffect, uintptr(unsafe.Pointer(&clsidShadow)), uintptr(unsafe.Pointer(&e)))) {
		return
	}
	defer e.release()
	sigma := max(sh.R, 0) / 2
	e.call(effectSetValue, 0, d2dPropertyFloat, uintptr(unsafe.Pointer(&sigma)), 4)
	col := colorOf(sh.Color)
	e.call(effectSetValue, 1, d2dPropertyVector4, uintptr(unsafe.Pointer(&col)), 16)
	e.call(effectSetInput, 0, list.ptr(), 1)
	e.call(effectGetOutput, uintptr(unsafe.Pointer(&out)))
	if out == nil {
		return
	}
	defer out.release()
	c.p.setTransform(c.world)
	at := pointF{sh.X, sh.Y}
	c.p.dc.call(dcDrawImage, out.ptr(), uintptr(unsafe.Pointer(&at)), 0, d2dInterpolationLinear, 0)
}

// effect returns a new effect of clsid mixing source over destination
// by mode, or nil.
func (c *canvasPlayer) effect(clsid *guid, mode uint32, destination, source *comObject) (e, out *comObject) {
	if failed(c.p.dc.call(dcCreateEffect, uintptr(unsafe.Pointer(clsid)), uintptr(unsafe.Pointer(&e)))) {
		return nil, nil
	}
	e.call(effectSetValue, 0, d2dPropertyEnum, uintptr(unsafe.Pointer(&mode)), 4)
	e.call(effectSetInput, 0, destination.ptr(), 1)
	e.call(effectSetInput, 1, source.ptr(), 1)
	e.call(effectGetOutput, uintptr(unsafe.Pointer(&out)))
	if out == nil {
		e.release()
		return nil, nil
	}
	return e, out
}

// mix mixes what list recorded with the canvas's bitmap by the operator:
// the composite or blend effect of it over a copy of the bitmap, copied
// over the bitmap where the clips leave room.
func (c *canvasPlayer) mix(list *comObject) {
	p := c.p
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaPremultiplied, dpiX: 96 * p.scale,
		dpiY: 96 * p.scale}
	pw, ph := uint32(math.Ceil(float64(c.width*p.scale))), uint32(math.Ceil(float64(c.height*p.scale)))
	var dst *comObject
	if failed(p.dc.callSize(dcCreateBitmap, pw, ph, 0, 0, uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&dst)))) {
		return
	}
	defer dst.release()
	p.dc.call(rtFlush, 0, 0)
	dst.call(bitmapCopyFromBitmap, 0, c.off.ptr(), 0)
	var objects []*comObject
	defer func() {
		for _, o := range objects {
			o.release()
		}
	}()
	var e, out *comObject
	if b, ok := c.st.composite.Blend(); ok {
		e, out = c.effect(&clsidBlend, blendModes[b-driver.BlendMultiply], dst, list)
	} else {
		e, out = c.effect(&clsidComposite, compositeModes[c.st.composite], dst, list)
	}
	if out == nil {
		return
	}
	objects = append(objects, e, out)
	result := out
	if mask := c.clipMask(pw, ph); mask != nil {
		objects = append(objects, mask)
		e1, in := c.effect(&clsidComposite, compositeIn, mask, out)
		e2, rest := c.effect(&clsidComposite, compositeDstOut, dst, mask)
		objects = append(objects, e1, in, e2, rest)
		if in == nil || rest == nil {
			return
		}
		e3, sum := c.effect(&clsidComposite, compositePlus, rest, in)
		if sum == nil {
			return
		}
		objects = append(objects, e3, sum)
		result = sum
	}
	// A copy ignores layers' clips: pop them around it.
	pushed := p.pushed
	for i := len(pushed) - 1; i >= 0; i-- {
		if pushed[i].clip {
			p.dc.call(rtPopAxisAlignedClip)
		} else {
			p.dc.call(rtPopLayer)
		}
	}
	p.setTransform(identity)
	area := rectF{0, 0, c.width, c.height}
	p.dc.call(dcDrawImage, result.ptr(), 0, uintptr(unsafe.Pointer(&area)), d2dInterpolationLinear, compositeCopy)
	for _, q := range pushed {
		p.setTransform(q.m)
		if q.clip {
			r := q.rect
			p.dc.call(rtPushAxisAlignedClip, uintptr(unsafe.Pointer(&r)), d2dAntialiasAliased)
		} else {
			p.pushLayerRaw(q.opacity, q.mask, q.brush)
		}
	}
	p.setTransform(c.world)
}

// clipMask returns the area the canvas's clips leave, opaque, as a
// bitmap, or nil when nothing clips.
func (c *canvasPlayer) clipMask(pw, ph uint32) *comObject {
	var clips []clipShape
	for _, s := range c.stack {
		clips = append(clips, s.clips...)
	}
	clips = append(clips, c.st.clips...)
	if len(clips) == 0 {
		return nil
	}
	p := c.p
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaPremultiplied, dpiX: 96 * p.scale,
		dpiY: 96 * p.scale, options: d2dBitmapTarget}
	var mask *comObject
	if failed(p.dc.callSize(dcCreateBitmap, pw, ph, 0, 0, uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&mask)))) {
		return nil
	}
	bake := p.g.bake
	bake.call(dcSetTarget, mask.ptr())
	bake.callF(rtSetDpi, 0b11, f32(96*p.scale), f32(96*p.scale))
	bake.call(rtBeginDraw)
	t := identity.d2d()
	bake.call(rtSetTransform, uintptr(unsafe.Pointer(&t)))
	clear := colorF{}
	bake.call(rtClear, uintptr(unsafe.Pointer(&clear)))
	for _, cl := range clips {
		params := layerParameters1{contentBounds: infinite, maskAntialias: d2dAntialiasPerPrimitive,
			maskTransform: identity.d2d(), opacity: 1, mask: cl.g.ptr()}
		bake.call(dcPushLayer, uintptr(unsafe.Pointer(&params)), 0)
	}
	white := colorF{1, 1, 1, 1}
	var brush *comObject
	bake.call(rtCreateSolidColorBrush, uintptr(unsafe.Pointer(&white)), 0, uintptr(unsafe.Pointer(&brush)))
	if brush != nil {
		r := rectF{0, 0, c.width, c.height}
		bake.call(rtFillRectangle, uintptr(unsafe.Pointer(&r)), brush.ptr())
		brush.release()
	}
	for range clips {
		bake.call(rtPopLayer)
	}
	bake.call(rtEndDraw, 0, 0)
	bake.call(dcSetTarget, 0)
	return mask
}

// clear makes a rectangle of the canvas transparent: its own bitmap's
// pixels copied over by transparent ones.
func (c *canvasPlayer) clear(op *driver.DrawOp) {
	path, has, cur, start := c.path, c.has, c.cur, c.start
	c.path, c.has = nil, false
	c.play(&driver.DrawOp{Kind: driver.DrawRect, X: op.X, Y: op.Y, W: op.W, H: op.H})
	g := c.geometry(false)
	c.path, c.has, c.cur, c.start = path, has, cur, start
	if g == nil {
		return
	}
	defer g.release()
	p := c.p
	p.setTransform(c.world)
	if c.off == nil {
		// Without a bitmap of its own, clearing paints what is behind.
		p.fillGeometry(g, background(c.v)|0xff)
		return
	}
	p.dc.call(dcSetPrimitiveBlend, d2dBlendCopy)
	p.dc.call(rtFillGeometry, g.ptr(), p.solid(0).ptr(), 0)
	p.dc.call(dcSetPrimitiveBlend, 0)
}

// text draws op's text with its baseline at its point, moved by its
// alignment, in the drawing's transform: filled by DirectWrite in the
// fill paint, or stroked as the outlines of its glyphs.
func (c *canvasPlayer) text(op *driver.DrawOp) {
	if op.Text == "" {
		return
	}
	layout := plainLayout(op.Text, op.Font)
	if layout == nil {
		return
	}
	defer layout.release()
	var line lineMetrics
	var count uint32
	layout.call(layoutGetLineMetrics, uintptr(unsafe.Pointer(&line)), 1, uintptr(unsafe.Pointer(&count)))
	x, y := op.X+op.X1, op.Y+op.Y1-line.baseline
	at := translation(float64(x), float64(y)).then(c.st.ctm)
	if op.Kind == driver.DrawFillText {
		b := c.brush(c.st.fill, c.st.fillGrad, translation(-float64(x), -float64(y)))
		if b == nil {
			return
		}
		defer b.release()
		c.paint(func(dc *comObject, base matrix) {
			c.setTransform(dc, at.then(base))
			dc.callPoint(rtDrawTextLayout, 0, 0, layout.ptr(), b.ptr(), d2dDrawTextEnableColorFont)
		})
		return
	}
	runs := glyphOutlines(c.p.g.factory, layout, line.baseline)
	defer func() {
		for _, r := range runs {
			r.geometry.release()
		}
	}()
	b := c.brush(c.st.stroke, c.st.strokeGrad, translation(-float64(x), -float64(y)))
	if b == nil || len(runs) == 0 {
		return
	}
	defer b.release()
	style := c.strokeStyle()
	c.paint(func(dc *comObject, base matrix) {
		for _, r := range runs {
			c.setTransform(dc, translation(float64(r.x), float64(r.y)).then(at).then(base))
			dc.callF(rtDrawGeometry, 0b100, r.geometry.ptr(), b.ptr(), f32(max(c.st.width, 0)), style.ptr())
		}
	})
}

// plainLayout returns text in font on one line, or nil.
func plainLayout(text string, font driver.Font) *comObject {
	format := textFormat(font)
	if format == nil {
		return nil
	}
	defer format.release()
	units := utf16.Encode([]rune(text))
	if len(units) == 0 {
		return nil
	}
	format.call(formatSetWordWrapping, dwNoWrap)
	var layout *comObject
	if failed(dwriteFactory().callF(dwCreateTextLayout, 0b11000, uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)),
		format.ptr(), f32(unbounded), f32(unbounded), uintptr(unsafe.Pointer(&layout)))) {
		return nil
	}
	return layout
}

// image draws op's image once it has loaded, and the canvas again when
// it loads.
func (c *canvasPlayer) image(op *driver.DrawOp) {
	if op.Image == nil {
		return
	}
	s := c.p.w.canvasImage(*op.Image)
	if s == nil || s.pic == nil {
		return
	}
	bmp := s.bitmapOf(c.p.dc, 0)
	if bmp == nil {
		return
	}
	nat := natural(s)
	w, h := op.W, op.H
	if w == 0 && h == 0 {
		w, h = float32(nat.Width), float32(nat.Height)
	}
	k := float32(s.pic.width) / max(float32(nat.Width), 1e-3)
	src := rectF{0, 0, float32(s.pic.width), float32(s.pic.height)}
	if op.X2 != 0 || op.Y2 != 0 {
		src = rectF{op.X1 * k, op.Y1 * k, (op.X1 + op.X2) * k, (op.Y1 + op.Y2) * k}
	}
	dst := rectF{op.X, op.Y, op.X + w, op.Y + h}
	alpha := c.st.alpha
	c.paint(func(dc *comObject, base matrix) {
		c.setTransform(dc, c.st.ctm.then(base))
		dc.callF(rtDrawBitmap, 0b100, bmp.ptr(), uintptr(unsafe.Pointer(&dst)), f32(alpha), d2dInterpolationLinear,
			uintptr(unsafe.Pointer(&src)))
	})
}

// canvasImage returns src's slot, loading it the first time and painting
// the window again when it loads.
func (w *window) canvasImage(src driver.ImageSource) *imageSlot {
	key := src.Key()
	if s := w.canvasImages[key]; s != nil {
		return s
	}
	if w.canvasImages == nil {
		w.canvasImages = map[string]*imageSlot{}
	}
	s := &imageSlot{src: src}
	w.canvasImages[key] = s
	ctx, cancel := context.WithCancel(context.Background())
	s.load = &imageLoad{cancel: cancel}
	go func() {
		data, err := imageload.Load(ctx, src, nil)
		var pic *picture
		if err == nil {
			pic, err = decode(ctx, data)
		}
		w.post(func() {
			cancel()
			s.load = nil
			if err != nil {
				log.Printf("nsgo: canvas: image %s: %v", key, err)
				return
			}
			s.pic = pic
			// Shaders and canvases that may draw it draw again.
			for _, v := range w.views {
				if v != nil && v.shaderView != nil {
					v.shaderView.dirty = true
				}
				if v != nil && v.drawing != nil {
					v.canvasCache.release()
				}
			}
			w.redrawAll()
		})
	}()
	return s
}

var clsidFlood = parseGUID("61c23c20-ae69-4d8e-94cf-50078df638f2")

// drawTinted draws what record draws in color c alone, where it drew
// anything: a text's bound color over all of its glyphs. It records into
// a command list, as drawFiltered does, and draws a flood of c through
// the composite effect's source-in.
func (p *painter) drawTinted(world matrix, c driver.Color, record func()) {
	var list *comObject
	if failed(p.dc.call(dcCreateCommandList, uintptr(unsafe.Pointer(&list)))) {
		record()
		return
	}
	defer list.release()
	p.flatten(func() {
		var target *comObject
		p.dc.call(dcGetTarget, uintptr(unsafe.Pointer(&target)))
		p.dc.call(dcSetTarget, list.ptr())
		pushed, split, m := p.pushed, p.split, p.m
		p.pushed, p.split = nil, nil
		p.setTransform(identity)
		record()
		for len(p.pushed) > 0 {
			p.pop()
		}
		p.pushed, p.split = pushed, split
		p.dc.call(dcSetTarget, target.ptr())
		target.release()
		p.m = m
	})
	if failed(list.call(commandListClose)) {
		return
	}
	var flood, floodOut *comObject
	if failed(p.dc.call(dcCreateEffect, uintptr(unsafe.Pointer(&clsidFlood)), uintptr(unsafe.Pointer(&flood)))) {
		return
	}
	defer flood.release()
	col := colorOf(c)
	flood.call(effectSetValue, 0, d2dPropertyVector4, uintptr(unsafe.Pointer(&col)), 16)
	flood.call(effectGetOutput, uintptr(unsafe.Pointer(&floodOut)))
	if floodOut == nil {
		return
	}
	defer floodOut.release()
	c2 := &canvasPlayer{p: p}
	e, out := c2.effect(&clsidComposite, compositeIn, list, floodOut)
	if out == nil {
		return
	}
	defer e.release()
	defer out.release()
	p.setTransform(world)
	var bounds rectF
	p.dc.call(dcGetImageLocalBounds, list.ptr(), uintptr(unsafe.Pointer(&bounds)))
	p.dc.call(dcDrawImage, out.ptr(), 0, uintptr(unsafe.Pointer(&bounds)), d2dInterpolationLinear, 0)
}
