//go:build linux && !android

package linux

import (
	"context"
	"log"
	"math"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

// cairoMatrix is cairo_matrix_t: it maps (x, y) to (xx*x + xy*y + x0,
// yx*x + yy*y + y0), the order of a canvas's a, b, c, d, e, f.
type cairoMatrix struct{ xx, yx, xy, yy, x0, y0 float64 }

func (m cairoMatrix) then(n cairoMatrix) cairoMatrix {
	return cairoMatrix{
		xx: m.xx*n.xx + m.xy*n.yx, yx: m.yx*n.xx + m.yy*n.yx,
		xy: m.xx*n.xy + m.xy*n.yy, yy: m.yx*n.xy + m.yy*n.yy,
		x0: m.xx*n.x0 + m.xy*n.y0 + m.x0, y0: m.yx*n.x0 + m.yy*n.y0 + m.y0,
	}
}

func (m cairoMatrix) invert() (cairoMatrix, bool) {
	det := m.xx*m.yy - m.yx*m.xy
	if det == 0 || math.IsNaN(det) {
		return cairoMatrix{}, false
	}
	return cairoMatrix{xx: m.yy / det, yx: -m.yx / det, xy: -m.xy / det, yy: m.xx / det,
		x0: (m.xy*m.y0 - m.yy*m.x0) / det, y0: (m.yx*m.x0 - m.xx*m.y0) / det}, true
}

var (
	cairoSetDash               func(cr uintptr, dashes *float64, n int32, offset float64)
	cairoSetMiterLimit         func(cr uintptr, limit float64)
	cairoTransform             func(cr uintptr, m *cairoMatrix)
	cairoSetMatrix             func(cr uintptr, m *cairoMatrix)
	cairoGetMatrix             func(cr uintptr, m *cairoMatrix)
	cairoCopyPath              func(cr uintptr) uintptr
	cairoAppendPath            func(cr, path uintptr)
	cairoPathDestroy           func(path uintptr)
	cairoStrokePreserve        func(cr uintptr)
	cairoSetSource             func(cr, pattern uintptr)
	cairoPatternCreateLinear   func(x0, y0, x1, y1 float64) uintptr
	cairoPatternCreateRadial   func(x0, y0, r0, x1, y1, r1 float64) uintptr
	cairoPatternCreateMesh     func() uintptr
	cairoMeshBeginPatch        func(p uintptr)
	cairoMeshEndPatch          func(p uintptr)
	cairoMeshMoveTo            func(p uintptr, x, y float64)
	cairoMeshLineTo            func(p uintptr, x, y float64)
	cairoMeshCurveTo           func(p uintptr, x1, y1, x2, y2, x, y float64)
	cairoMeshSetCornerColor    func(p uintptr, corner uint32, r, g, b, a float64)
	cairoPatternAddStop        func(p uintptr, offset, r, g, b, a float64)
	cairoPatternSetExtend      func(p uintptr, extend int32)
	cairoPatternSetMatrix      func(p uintptr, m *cairoMatrix)
	cairoPatternDestroy        func(p uintptr)
	cairoMaskSurface           func(cr, surface uintptr, x, y float64)
	cairoMask                  func(cr, pattern uintptr)
	cairoPopGroup              func(cr uintptr) uintptr
	cairoImageSurfaceCreateFor func(data unsafe.Pointer, format, w, h, stride int32) uintptr
	pangoCairoLayoutPath       func(cr, layout uintptr)
)

func bindCanvas(cairoLib, pangocairoLib uintptr) {
	reg := purego.RegisterLibFunc
	reg(&cairoSetDash, cairoLib, "cairo_set_dash")
	reg(&cairoSetMiterLimit, cairoLib, "cairo_set_miter_limit")
	reg(&cairoTransform, cairoLib, "cairo_transform")
	reg(&cairoSetMatrix, cairoLib, "cairo_set_matrix")
	reg(&cairoGetMatrix, cairoLib, "cairo_get_matrix")
	reg(&cairoCopyPath, cairoLib, "cairo_copy_path")
	reg(&cairoAppendPath, cairoLib, "cairo_append_path")
	reg(&cairoPathDestroy, cairoLib, "cairo_path_destroy")
	reg(&cairoStrokePreserve, cairoLib, "cairo_stroke_preserve")
	reg(&cairoSetSource, cairoLib, "cairo_set_source")
	reg(&cairoPatternCreateLinear, cairoLib, "cairo_pattern_create_linear")
	reg(&cairoPatternCreateRadial, cairoLib, "cairo_pattern_create_radial")
	reg(&cairoPatternCreateMesh, cairoLib, "cairo_pattern_create_mesh")
	reg(&cairoMeshBeginPatch, cairoLib, "cairo_mesh_pattern_begin_patch")
	reg(&cairoMeshEndPatch, cairoLib, "cairo_mesh_pattern_end_patch")
	reg(&cairoMeshMoveTo, cairoLib, "cairo_mesh_pattern_move_to")
	reg(&cairoMeshLineTo, cairoLib, "cairo_mesh_pattern_line_to")
	reg(&cairoMeshCurveTo, cairoLib, "cairo_mesh_pattern_curve_to")
	reg(&cairoMeshSetCornerColor, cairoLib, "cairo_mesh_pattern_set_corner_color_rgba")
	reg(&cairoPatternAddStop, cairoLib, "cairo_pattern_add_color_stop_rgba")
	reg(&cairoPatternSetExtend, cairoLib, "cairo_pattern_set_extend")
	reg(&cairoPatternSetMatrix, cairoLib, "cairo_pattern_set_matrix")
	reg(&cairoPatternDestroy, cairoLib, "cairo_pattern_destroy")
	reg(&cairoMaskSurface, cairoLib, "cairo_mask_surface")
	reg(&cairoMask, cairoLib, "cairo_mask")
	reg(&cairoPopGroup, cairoLib, "cairo_pop_group")
	reg(&cairoImageSurfaceCreateFor, cairoLib, "cairo_image_surface_create_for_data")
	reg(&pangoCairoLayoutPath, pangocairoLib, "pango_cairo_layout_path")
	bindRaster(cairoLib)
}

// Cairo's operators of each CompositeOp.
var cairoOperators = [...]int32{
	driver.CompositeSourceOver: 2, driver.CompositeSourceIn: 3, driver.CompositeSourceOut: 4,
	driver.CompositeSourceAtop: 5, driver.CompositeDestinationOver: 7, driver.CompositeDestinationIn: 8,
	driver.CompositeDestinationOut: 9, driver.CompositeDestinationAtop: 10, driver.CompositeLighter: 12,
	driver.CompositeCopy: 1, driver.CompositeXor: 11, driver.CompositeMultiply: 14, driver.CompositeScreen: 15,
	driver.CompositeOverlay: 16, driver.CompositeDarken: 17, driver.CompositeLighten: 18,
	driver.CompositeColorDodge: 19, driver.CompositeColorBurn: 20, driver.CompositeHardLight: 21,
	driver.CompositeSoftLight: 22, driver.CompositeDifference: 23, driver.CompositeExclusion: 24,
	driver.CompositeHue: 25, driver.CompositeSaturation: 26, driver.CompositeColor: 27,
	driver.CompositeLuminosity: 28,
}

// canvasState is what Save and Restore keep while a Drawing plays,
// besides Cairo's own state.
type canvasState struct {
	fill, stroke         driver.Color
	fillGrad, strokeGrad *driver.CanvasGradient
	width, miter         float64
	cap, join            int32
	dash                 []float64
	dashOffset           float64
	alpha                float64
	shadow               driver.DrawOp // DrawShadow, or zero for none
}

// canvasPlayer replays a drawing into a Cairo context, as the web's 2D
// context does.
type canvasPlayer struct {
	w     *window
	v     *view
	cr    uintptr
	base  cairoMatrix // the canvas's own coordinates
	size  rect
	st    canvasState
	stack []canvasState
}

// changedNatives hands components the animated props that changed.
func (w *window) changedNatives() {
	w.animator.ChangedNatives(func(id driver.NodeID, b driver.NativeBinding, num float64, col driver.Color) {
		if v := w.view(id); v != nil && v.native != nil {
			driver.SetAnimatedProp(v.native.comp, b, num, col)
		}
	})
}

// playCanvas replays v's drawing into cr at r, as the web's 2D context
// does.
func (w *window) playCanvas(v *view, cr uintptr, r rect) {
	w.playCanvasFrom(v, cr, r, w.animator.CanvasOps(v.id, v.drawing), driver.CanvasSplit{}, nil)
}

// playCanvasFrom replays ops into cr at r, leaving out what split skips
// (raster.go) and calling before, when set, with each op's index first.
func (w *window) playCanvasFrom(v *view, cr uintptr, r rect, ops []driver.DrawOp, split driver.CanvasSplit,
	before func(at int)) {
	cairoSave(cr)
	cairoRectangle(cr, r.x, r.y, r.w, r.h)
	cairoClip(cr)
	cairoTranslate(cr, r.x, r.y)
	p := &canvasPlayer{w: w, v: v, cr: cr, size: r,
		st: canvasState{fill: driver.RGB(0x000000), stroke: driver.RGB(0x000000), width: 1, miter: 10, alpha: 1}}
	cairoGetMatrix(cr, &p.base)
	cairoNewPath(cr)
	for i := range ops {
		if before != nil {
			before(i)
		}
		if split.Skips(i, &ops[i]) {
			continue
		}
		p.play(&ops[i])
	}
	for range p.stack {
		cairoRestore(cr)
	}
	cairoRestore(cr)
}

func (p *canvasPlayer) play(op *driver.DrawOp) {
	cr := p.cr
	f := func(v float32) float64 { return float64(v) }
	switch op.Kind {
	case driver.DrawSave:
		cairoSave(cr)
		p.stack = append(p.stack, p.st)
	case driver.DrawRestore:
		if n := len(p.stack); n > 0 {
			p.st = p.stack[n-1]
			p.stack = p.stack[:n-1]
			cairoRestore(cr)
		}
	case driver.DrawTranslate:
		cairoTranslate(cr, f(op.X), f(op.Y))
	case driver.DrawScale:
		if op.X != 0 && op.Y != 0 {
			cairoScale(cr, f(op.X), f(op.Y))
		}
	case driver.DrawRotate:
		cairoRotate(cr, f(op.A))
	case driver.DrawTransform:
		m := cairoMatrix{f(op.X1), f(op.Y1), f(op.X2), f(op.Y2), f(op.X), f(op.Y)}
		if _, ok := m.invert(); ok {
			cairoTransform(cr, &m)
		}
	case driver.DrawSetTransform:
		m := cairoMatrix{f(op.X1), f(op.Y1), f(op.X2), f(op.Y2), f(op.X), f(op.Y)}
		if _, ok := m.invert(); ok {
			m = p.base.then(m)
			cairoSetMatrix(cr, &m)
		}
	case driver.DrawBeginPath:
		cairoNewPath(cr)
	case driver.DrawMoveTo:
		cairoMoveTo(cr, f(op.X), f(op.Y))
	case driver.DrawLineTo:
		cairoLineTo(cr, f(op.X), f(op.Y))
	case driver.DrawQuadTo:
		var x0, y0 float64
		cairoGetCurrentPoint(cr, &x0, &y0)
		cairoCurveTo(cr, x0+2*(f(op.X1)-x0)/3, y0+2*(f(op.Y1)-y0)/3,
			f(op.X)+2*(f(op.X1)-f(op.X))/3, f(op.Y)+2*(f(op.Y1)-f(op.Y))/3, f(op.X), f(op.Y))
	case driver.DrawCubicTo:
		cairoCurveTo(cr, f(op.X1), f(op.Y1), f(op.X2), f(op.Y2), f(op.X), f(op.Y))
	case driver.DrawArc:
		r := max(f(op.R), 0)
		a0, a1 := f(op.A0), f(op.A1)
		// Cairo turns an arc past a full circle around again; the canvas
		// stops at one.
		if !op.CCW && a1-a0 > 2*math.Pi {
			a1 = a0 + 2*math.Pi
		} else if op.CCW && a0-a1 > 2*math.Pi {
			a1 = a0 - 2*math.Pi
		}
		if op.CCW {
			cairoArcNegative(cr, f(op.X), f(op.Y), r, a0, a1)
		} else {
			cairoArc(cr, f(op.X), f(op.Y), r, a0, a1)
		}
	case driver.DrawRect:
		cairoRectangle(cr, f(op.X), f(op.Y), f(op.W), f(op.H))
		cairoMoveTo(cr, f(op.X), f(op.Y))
	case driver.DrawRoundRect:
		roundRect(cr, rect{x: f(op.X), y: f(op.Y), w: f(op.W), h: f(op.H), r: f(op.R)})
	case driver.DrawEllipse:
		if op.R > 0 && op.RY > 0 {
			cairoNewSubPath(cr)
			cairoSave(cr)
			cairoTranslate(cr, f(op.X), f(op.Y))
			cairoScale(cr, f(op.R), f(op.RY))
			cairoArc(cr, 0, 0, 1, 0, 2*math.Pi)
			cairoRestore(cr)
			cairoClosePath(cr)
		}
	case driver.DrawClosePath:
		cairoClosePath(cr)
	case driver.DrawFill:
		p.shadowed(func(cr uintptr) {
			cairoSetFillRule(cr, fillRule(op.EvenOdd))
			p.source(cr, p.st.fill, p.st.fillGrad)
			cairoFillPreserve(cr)
			cairoSetFillRule(cr, 0)
		})
	case driver.DrawStroke:
		p.shadowed(func(cr uintptr) {
			p.lineStyle(cr)
			p.source(cr, p.st.stroke, p.st.strokeGrad)
			cairoStrokePreserve(cr)
		})
	case driver.DrawClip:
		path := cairoCopyPath(cr)
		cairoSetFillRule(cr, fillRule(op.EvenOdd))
		cairoClip(cr)
		cairoSetFillRule(cr, 0)
		cairoAppendPath(cr, path)
		cairoPathDestroy(path)
	case driver.DrawFillColor:
		p.st.fill, p.st.fillGrad = op.Color, nil
	case driver.DrawStrokeColor:
		p.st.stroke, p.st.strokeGrad = op.Color, nil
	case driver.DrawFillGradient:
		if op.Gradient != nil && len(op.Gradient.Stops) > 0 {
			p.st.fillGrad = op.Gradient
		}
	case driver.DrawStrokeGradient:
		if op.Gradient != nil && len(op.Gradient.Stops) > 0 {
			p.st.strokeGrad = op.Gradient
		}
	case driver.DrawLineWidth:
		p.st.width = f(op.W)
	case driver.DrawLineCap:
		p.st.cap = int32(op.Cap)
	case driver.DrawLineJoin:
		p.st.join = int32(op.Join)
	case driver.DrawMiterLimit:
		p.st.miter = f(op.W)
	case driver.DrawLineDash:
		p.st.dash = nil
		if op.Dash != nil {
			for _, l := range *op.Dash {
				p.st.dash = append(p.st.dash, float64(l))
			}
		}
	case driver.DrawLineDashOffset:
		p.st.dashOffset = f(op.W)
	case driver.DrawAlpha:
		p.st.alpha = min(max(f(op.A), 0), 1)
	case driver.DrawComposite:
		if int(op.Composite) < len(cairoOperators) {
			cairoSetOperator(cr, cairoOperators[op.Composite])
		}
	case driver.DrawShadow:
		p.st.shadow = *op
	case driver.DrawFillText, driver.DrawStrokeText:
		p.text(op)
	case driver.DrawImage:
		p.image(op)
	case driver.DrawClear:
		path := cairoCopyPath(cr)
		cairoSave(cr)
		cairoSetOperator(cr, operatorClear)
		cairoNewPath(cr)
		cairoRectangle(cr, f(op.X), f(op.Y), f(op.W), f(op.H))
		cairoFill(cr)
		cairoRestore(cr)
		cairoAppendPath(cr, path)
		cairoPathDestroy(path)
	}
}

func fillRule(evenOdd bool) int32 {
	if evenOdd {
		return 1
	}
	return 0
}

// lineStyle gives cr the stroke settings.
func (p *canvasPlayer) lineStyle(cr uintptr) {
	cairoSetLineWidth(cr, p.st.width)
	cairoSetLineCap(cr, p.st.cap)
	cairoSetLineJoin(cr, p.st.join)
	cairoSetMiterLimit(cr, p.st.miter)
	if len(p.st.dash) == 0 {
		cairoSetDash(cr, nil, 0, 0)
	} else {
		cairoSetDash(cr, &p.st.dash[0], int32(len(p.st.dash)), p.st.dashOffset)
	}
}

// source sets c, or g when not nil, as cr's source under the global
// alpha.
func (p *canvasPlayer) source(cr uintptr, c driver.Color, g *driver.CanvasGradient) {
	if g == nil {
		setColorAlpha(cr, c, p.st.alpha)
		return
	}
	pat := gradientPattern(g, p.st.alpha, math.Hypot(p.size.w, p.size.h))
	cairoSetSource(cr, pat)
	cairoPatternDestroy(pat)
}

// gradientPattern returns a Cairo pattern of g with alpha multiplied in,
// cover long enough to reach across the canvas for a conic one.
func gradientPattern(g *driver.CanvasGradient, alpha, cover float64) uintptr {
	f := func(v float32) float64 { return float64(v) }
	var pat uintptr
	switch {
	case g.Conic:
		pat = conicPattern(g, alpha, cover+math.Hypot(f(g.X1), f(g.Y1)))
	case g.Radial:
		pat = cairoPatternCreateRadial(f(g.X1), f(g.Y1), max(f(g.R1), 0), f(g.X2), f(g.Y2), max(f(g.R2), 0))
	default:
		pat = cairoPatternCreateLinear(f(g.X1), f(g.Y1), f(g.X2), f(g.Y2))
	}
	if !g.Conic {
		for _, s := range g.Stops {
			r, gg, b, a := s.Color.Components()
			cairoPatternAddStop(pat, min(max(float64(s.Offset), 0), 1), r, gg, b, a*alpha)
		}
		extend := int32(3) // pad
		switch g.Spread {
		case driver.SpreadRepeat:
			extend = 1
		case driver.SpreadReflect:
			extend = 2
		}
		cairoPatternSetExtend(pat, extend)
	}
	if m := g.Matrix(); m != [6]float32{1, 0, 0, 1, 0, 0} {
		// Cairo's pattern matrix maps user space into the pattern's.
		cm := cairoMatrix{f(m[0]), f(m[1]), f(m[2]), f(m[3]), f(m[4]), f(m[5])}
		if inv, ok := cm.invert(); ok {
			cairoPatternSetMatrix(pat, &inv)
		}
	}
	return pat
}

// conicPattern draws a conic gradient as a mesh of wedges reaching r
// from its center, split at its stops and every 1/64 of a turn.
func conicPattern(g *driver.CanvasGradient, alpha, r float64) uintptr {
	pat := cairoPatternCreateMesh()
	cx, cy := float64(g.X1), float64(g.Y1)
	var cuts []float64
	for i := range 64 {
		cuts = append(cuts, float64(i)/64)
	}
	for _, s := range g.Stops {
		cuts = append(cuts, min(max(float64(s.Offset), 0), 1))
	}
	cuts = append(cuts, 1)
	sortFloats(cuts)
	for i := 0; i+1 < len(cuts); i++ {
		t0, t1 := cuts[i], cuts[i+1]
		if t1-t0 < 1e-6 {
			continue
		}
		a0 := float64(g.Angle) + 2*math.Pi*t0
		a1 := float64(g.Angle) + 2*math.Pi*t1
		k := 4.0 / 3 * math.Tan((a1-a0)/4)
		x0, y0 := cx+r*math.Cos(a0), cy+r*math.Sin(a0)
		x1, y1 := cx+r*math.Cos(a1), cy+r*math.Sin(a1)
		c0, c1 := g.At(t0), g.At(t1)
		cairoMeshBeginPatch(pat)
		cairoMeshMoveTo(pat, cx, cy)
		cairoMeshLineTo(pat, x0, y0)
		cairoMeshCurveTo(pat, x0-k*r*math.Sin(a0), y0+k*r*math.Cos(a0), x1+k*r*math.Sin(a1), y1-k*r*math.Cos(a1), x1, y1)
		cairoMeshLineTo(pat, cx, cy)
		for corner, c := range []driver.Color{c0, c0, c1, c1} {
			r, gg, b, a := c.Components()
			cairoMeshSetCornerColor(pat, uint32(corner), r, gg, b, a*alpha)
		}
		cairoMeshEndPatch(pat)
	}
	return pat
}

func sortFloats(v []float64) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// shadowed draws with draw, which paints on the context it is given
// from its current path, first into a mask for the shadow when one is
// on, which it blurs and paints offset in the shadow's color.
func (p *canvasPlayer) shadowed(draw func(cr uintptr)) {
	sh := p.st.shadow
	if sh.Color&0xff == 0 || (sh.X == 0 && sh.Y == 0 && sh.R == 0) {
		draw(p.cr)
		return
	}
	scale := math.Max(float64(p.w.env.PixelRatio), 1)
	blur := float64(sh.R) / 2 * scale // the Gaussian's deviation
	pad := math.Ceil(3 * blur)
	w, h := int(math.Ceil(p.size.w*scale+2*pad)), int(math.Ceil(p.size.h*scale+2*pad))
	if w <= 0 || h <= 0 || w*h > 1<<24 {
		draw(p.cr)
		return
	}
	const formatA8 = 2
	stride := (w + 3) &^ 3
	buf := make([]byte, stride*h)
	surf := cairoImageSurfaceCreateFor(unsafe.Pointer(&buf[0]), formatA8, int32(w), int32(h), int32(stride))
	mask := cairoCreate(surf)
	// The mask's space: the canvas's points, scaled, inside the padding,
	// then the transform the drawing has now.
	var m cairoMatrix
	cairoGetMatrix(p.cr, &m)
	if inv, ok := p.base.invert(); ok {
		m = cairoMatrix{xx: scale, yy: scale, x0: pad, y0: pad}.then(inv.then(m))
	}
	cairoSetMatrix(mask, &m)
	path := cairoCopyPath(p.cr)
	cairoAppendPath(mask, path)
	cairoPathDestroy(path)
	draw(mask)
	cairoDestroy(mask)
	cairoSurfaceFlush(surf)
	boxBlur(buf, w, h, stride, blur)
	cairoSurfaceMarkDirty(surf)
	cairoSave(p.cr)
	inv := cairoMatrix{xx: 1 / scale, yy: 1 / scale, x0: (float64(sh.X)*scale - pad) / scale,
		y0: (float64(sh.Y)*scale - pad) / scale}
	full := p.base.then(inv)
	cairoSetMatrix(p.cr, &full)
	setColorAlpha(p.cr, sh.Color, 1)
	cairoMaskSurface(p.cr, surf, 0, 0)
	cairoRestore(p.cr)
	cairoSurfaceDestroy(surf)
	draw(p.cr)
}

// boxBlur blurs an alpha mask with three box blurs each way, close to a
// Gaussian of deviation sigma.
func boxBlur(buf []byte, w, h, stride int, sigma float64) {
	if sigma < 0.5 {
		return
	}
	// Three boxes of width sqrt(12σ²/3 + 1) make a Gaussian of σ.
	r := max(int(math.Round((math.Sqrt(4*sigma*sigma+1)-1)/2)), 1)
	line := make([]int, max(w, h))
	pass := func(get func(i int) byte, set func(i int, v byte), n int) {
		for i := range n {
			line[i] = int(get(i))
		}
		sum := 0
		for i := -r; i <= r; i++ {
			sum += line[min(max(i, 0), n-1)]
		}
		for i := range n {
			set(i, byte(sum/(2*r+1)))
			sum += line[min(i+r+1, n-1)] - line[max(i-r, 0)]
		}
	}
	for range 3 {
		for y := range h {
			row := buf[y*stride:]
			pass(func(i int) byte { return row[i] }, func(i int, v byte) { row[i] = v }, w)
		}
		for x := range w {
			pass(func(i int) byte { return buf[i*stride+x] }, func(i int, v byte) { buf[i*stride+x] = v }, h)
		}
	}
}

// text draws one line of text at the op's point, moved by its alignment,
// leaving the current path as it was.
func (p *canvasPlayer) text(op *driver.DrawOp) {
	if op.Text == "" {
		return
	}
	cr := p.cr
	l := canvasLayout(cr, op.Text, op.Font)
	defer gObjectUnref(l)
	baseline := float64(pangoLayoutGetBaseline(l)) / pangoScale
	x, y := float64(op.X+op.X1), float64(op.Y+op.Y1)-baseline
	saved := cairoCopyPath(cr)
	defer func() {
		cairoNewPath(cr)
		cairoAppendPath(cr, saved)
		cairoPathDestroy(saved)
	}()
	cairoNewPath(cr)
	stroke := op.Kind == driver.DrawStrokeText
	if !stroke && p.st.fillGrad == nil && p.st.shadow.Color&0xff == 0 {
		cairoMoveTo(cr, x, y)
		setColorAlpha(cr, p.st.fill, p.st.alpha)
		pangoCairoShowLayout(cr, l)
		return
	}
	cairoMoveTo(cr, x, y)
	pangoCairoLayoutPath(cr, l)
	p.shadowed(func(cr uintptr) {
		if stroke {
			p.lineStyle(cr)
			p.source(cr, p.st.stroke, p.st.strokeGrad)
			cairoStrokePreserve(cr)
			return
		}
		p.source(cr, p.st.fill, p.st.fillGrad)
		cairoFillPreserve(cr)
	})
}

// canvasLayout returns text in font as a Pango layout for cr.
func canvasLayout(cr uintptr, text string, font driver.Font) uintptr {
	if font.Size == 0 {
		font.Size = driver.DefaultFontSize
	}
	l := pangoCairoCreateLayout(cr)
	desc := pangoFontDescriptionNew()
	defer pangoFontDescriptionFree(desc)
	if font.Family != "" {
		pangoFontDescriptionSetFamily(desc, cstr(font.Family))
	}
	pangoFontDescriptionSetSize(desc, int32(float64(font.Size)*pangoScale+0.5))
	if font.Weight != 0 {
		pangoFontDescriptionSetWeight(desc, int32(font.Weight))
	}
	if font.Italic {
		pangoFontDescriptionSetStyle(desc, pangoStyleItalic)
	}
	pangoLayoutSetFontDescription(l, desc)
	pangoLayoutSetText(l, cstr(text), int32(len(text)))
	return l
}

// canvasImages are the images a window's canvases draw, by
// ImageSource.Key.
type canvasImages map[string]*imageSlot

// image draws op's image once it is loaded, and the canvas again when it
// loads.
func (p *canvasPlayer) image(op *driver.DrawOp) {
	if op.Image == nil {
		return
	}
	s := p.w.canvasImage(*op.Image, p.v)
	if s == nil || s.pic == nil {
		return
	}
	surf := s.surfaceOf(0)
	if surf == 0 {
		return
	}
	f := func(v float32) float64 { return float64(v) }
	nat := natural(s)
	w, h := f(op.W), f(op.H)
	if w == 0 && h == 0 {
		w, h = float64(nat.Width), float64(nat.Height)
	}
	sx, sy, sw, sh := 0.0, 0.0, float64(nat.Width), float64(nat.Height)
	if op.X2 != 0 || op.Y2 != 0 {
		sx, sy, sw, sh = f(op.X1), f(op.Y1), f(op.X2), f(op.Y2)
	}
	if sw <= 0 || sh <= 0 || nat.Width <= 0 {
		return
	}
	cr := p.cr
	saved := cairoCopyPath(cr)
	cairoNewPath(cr)
	cairoRectangle(cr, f(op.X), f(op.Y), w, h)
	p.shadowed(func(cr uintptr) {
		cairoSave(cr)
		cairoClip(cr)
		cairoTranslate(cr, f(op.X), f(op.Y))
		cairoScale(cr, w/sw, h/sh)
		cairoTranslate(cr, -sx, -sy)
		// The picture's pixels in the image's points.
		k := float64(nat.Width) / float64(s.pic.width)
		cairoScale(cr, k, k)
		cairoSetSourceSurface(cr, surf, 0, 0)
		cairoPaintWithAlpha(cr, p.st.alpha)
		cairoRestore(cr)
	})
	cairoNewPath(cr)
	cairoAppendPath(cr, saved)
	cairoPathDestroy(saved)
}

// canvasImage returns src's picture slot, loading it the first time and
// drawing v again when it loads.
func (w *window) canvasImage(src driver.ImageSource, v *view) *imageSlot {
	key := src.Key()
	if s := w.canvasImages[key]; s != nil {
		if s.pic == nil && s.load != nil {
			w.canvasWaiting[key] = appendView(w.canvasWaiting[key], v)
		}
		return s
	}
	if w.canvasImages == nil {
		w.canvasImages = canvasImages{}
		w.canvasWaiting = map[string][]*view{}
	}
	s := &imageSlot{src: src}
	w.canvasImages[key] = s
	w.canvasWaiting[key] = []*view{v}
	ctx, cancel := context.WithCancel(context.Background())
	ld := &imageLoad{cancel: cancel}
	s.load = ld
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
			for _, v := range w.canvasWaiting[key] {
				switch {
				case v.deleted:
				case v.shader != nil:
					gtkGLAreaQueueRender(v.shader.area)
				default:
					v.canvas.whole = true
					w.redrawCanvas(v)
				}
			}
			delete(w.canvasWaiting, key)
		})
	}()
	return s
}

func appendView(vs []*view, v *view) []*view {
	for _, o := range vs {
		if o == v {
			return vs
		}
	}
	return append(vs, v)
}

// setColorAlpha sets c as the source with alpha multiplied by k.
func setColorAlpha(cr uintptr, c driver.Color, k float64) {
	r, g, b, a := c.Components()
	cairoSetSourceRGBA(cr, r, g, b, a*k)
}
