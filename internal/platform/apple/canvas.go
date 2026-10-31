//go:build darwin

package apple

import (
	"context"
	"log"
	"math"
	"structs"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

var (
	cgContextRotateCTM           func(ctx uintptr, a float64)
	cgContextConcatCTM           func(ctx uintptr, m affine)
	cgContextGetCTM              func(ctx uintptr) affine
	cgContextBeginPath           func(ctx uintptr)
	cgContextMoveToPoint         func(ctx uintptr, x, y float64)
	cgContextAddLineToPoint      func(ctx uintptr, x, y float64)
	cgContextAddQuadCurve        func(ctx uintptr, cx, cy, x, y float64)
	cgContextAddCurve            func(ctx uintptr, x1, y1, x2, y2, x, y float64)
	cgContextAddArc              func(ctx uintptr, x, y, r, a0, a1 float64, clockwise int32)
	cgContextAddRect             func(ctx uintptr, r CGRect)
	cgContextAddEllipseInRect    func(ctx uintptr, r CGRect)
	cgContextAddPath             func(ctx, path uintptr)
	cgContextCopyPath            func(ctx uintptr) uintptr
	cgContextIsPathEmpty         func(ctx uintptr) bool
	cgContextClosePath           func(ctx uintptr)
	cgContextFillPath            func(ctx uintptr)
	cgContextEOFillPath          func(ctx uintptr)
	cgContextStrokePath          func(ctx uintptr)
	cgContextClip                func(ctx uintptr)
	cgContextEOClip              func(ctx uintptr)
	cgContextReplacePathStroked  func(ctx uintptr)
	cgContextSetFillColor        func(ctx, color uintptr)
	cgContextSetStrokeColor      func(ctx, color uintptr)
	cgContextSetLineWidth        func(ctx uintptr, w float64)
	cgContextSetLineCap          func(ctx uintptr, c int32)
	cgContextSetLineJoin         func(ctx uintptr, j int32)
	cgContextSetMiterLimit       func(ctx uintptr, l float64)
	cgContextSetAlpha            func(ctx uintptr, a float64)
	cgContextSetBlendMode        func(ctx uintptr, mode int32)
	cgContextClearRect           func(ctx uintptr, r CGRect)
	cgContextFillRect            func(ctx uintptr, r CGRect)
	cgContextSetTextMatrix       func(ctx uintptr, m affine)
	cgContextSetTextDrawingMode  func(ctx uintptr, mode int32)
	cgContextBeginTransparency   func(ctx, aux uintptr)
	cgContextEndTransparency     func(ctx uintptr)
	cgContextDrawImage           func(ctx uintptr, r CGRect, image uintptr)
	cgGradientCreate             func(space uintptr, components, locations *float64, count uintptr) uintptr
	cgImageCreateWithImageInRect func(image uintptr, r CGRect) uintptr
	cgPathCreateWithRoundedRect  func(r CGRect, rw, rh float64, transform uintptr) uintptr
)

// Runs of points and rectangles go by pointer, in one call. The
// gradients' CGPoints are their two fields, which arm64 and x86-64 pass
// as two floats, without the cost of a struct argument.
var (
	cgContextAddLines           func(ctx uintptr, points []CGPoint)
	cgContextAddRects           func(ctx uintptr, rects []CGRect)
	cgContextDrawLinearGradient func(ctx, gradient uintptr, x0, y0, x1, y1 float64, options uint32)
	cgContextDrawRadialGradient func(ctx, gradient uintptr, x0, y0, r0, x1, y1, r1 float64, options uint32)
	cgContextDrawConicGradient  func(ctx, gradient uintptr, x, y, angle float64)
)

// affine is CGAffineTransform.
type affine struct {
	_                structs.HostLayout
	A, B, C, D, X, Y float64
}

func (m affine) invert() affine {
	det := m.A*m.D - m.B*m.C
	if det == 0 {
		return affine{A: 1, D: 1}
	}
	return affine{A: m.D / det, B: -m.B / det, C: -m.C / det, D: m.A / det,
		X: (m.C*m.Y - m.D*m.X) / det, Y: (m.B*m.X - m.A*m.Y) / det}
}

func loadCanvas(cg uintptr) {
	reg := purego.RegisterLibFunc
	reg(&cgContextRotateCTM, cg, "CGContextRotateCTM")
	reg(&cgContextConcatCTM, cg, "CGContextConcatCTM")
	reg(&cgContextGetCTM, cg, "CGContextGetCTM")
	registerSyscall(&cgContextBeginPath, cg, "CGContextBeginPath")
	reg(&cgContextMoveToPoint, cg, "CGContextMoveToPoint")
	reg(&cgContextAddLineToPoint, cg, "CGContextAddLineToPoint")
	reg(&cgContextAddQuadCurve, cg, "CGContextAddQuadCurveToPoint")
	reg(&cgContextAddCurve, cg, "CGContextAddCurveToPoint")
	reg(&cgContextAddArc, cg, "CGContextAddArc")
	reg(&cgContextAddRect, cg, "CGContextAddRect")
	reg(&cgContextAddEllipseInRect, cg, "CGContextAddEllipseInRect")
	registerSyscall(&cgContextAddPath, cg, "CGContextAddPath")
	registerSyscall(&cgContextCopyPath, cg, "CGContextCopyPath")
	registerSyscall(&cgContextIsPathEmpty, cg, "CGContextIsPathEmpty")
	registerSyscall(&cgContextClosePath, cg, "CGContextClosePath")
	registerSyscall(&cgContextFillPath, cg, "CGContextFillPath")
	registerSyscall(&cgContextEOFillPath, cg, "CGContextEOFillPath")
	registerSyscall(&cgContextStrokePath, cg, "CGContextStrokePath")
	registerSyscall(&cgContextClip, cg, "CGContextClip")
	registerSyscall(&cgContextEOClip, cg, "CGContextEOClip")
	registerSyscall(&cgContextReplacePathStroked, cg, "CGContextReplacePathWithStrokedPath")
	registerSyscall(&cgContextSetFillColor, cg, "CGContextSetFillColorWithColor")
	registerSyscall(&cgContextSetStrokeColor, cg, "CGContextSetStrokeColorWithColor")
	reg(&cgContextSetLineWidth, cg, "CGContextSetLineWidth")
	registerSyscall(&cgContextSetLineCap, cg, "CGContextSetLineCap")
	registerSyscall(&cgContextSetLineJoin, cg, "CGContextSetLineJoin")
	reg(&cgContextSetMiterLimit, cg, "CGContextSetMiterLimit")
	reg(&cgContextSetAlpha, cg, "CGContextSetAlpha")
	registerSyscall(&cgContextSetBlendMode, cg, "CGContextSetBlendMode")
	reg(&cgContextClearRect, cg, "CGContextClearRect")
	reg(&cgContextFillRect, cg, "CGContextFillRect")
	reg(&cgContextSetTextMatrix, cg, "CGContextSetTextMatrix")
	registerSyscall(&cgContextSetTextDrawingMode, cg, "CGContextSetTextDrawingMode")
	registerSyscall(&cgContextBeginTransparency, cg, "CGContextBeginTransparencyLayer")
	registerSyscall(&cgContextEndTransparency, cg, "CGContextEndTransparencyLayer")
	addLines, addRects := symbol(cg, "CGContextAddLines"), symbol(cg, "CGContextAddRects")
	cgContextAddLines = func(ctx uintptr, points []CGPoint) {
		purego.SyscallN(addLines, ctx, uintptr(unsafe.Pointer(unsafe.SliceData(points))), uintptr(len(points)))
	}
	cgContextAddRects = func(ctx uintptr, rects []CGRect) {
		purego.SyscallN(addRects, ctx, uintptr(unsafe.Pointer(unsafe.SliceData(rects))), uintptr(len(rects)))
	}
	reg(&cgContextDrawImage, cg, "CGContextDrawImage")
	reg(&cgContextDrawLinearGradient, cg, "CGContextDrawLinearGradient")
	reg(&cgContextDrawRadialGradient, cg, "CGContextDrawRadialGradient")
	// macOS 14 and iOS 17 draw conic gradients; earlier systems draw
	// them as wedges.
	if _, err := purego.Dlsym(cg, "CGContextDrawConicGradient"); err == nil {
		reg(&cgContextDrawConicGradient, cg, "CGContextDrawConicGradient")
	}
	reg(&cgGradientCreate, cg, "CGGradientCreateWithColorComponents")
	reg(&cgImageCreateWithImageInRect, cg, "CGImageCreateWithImageInRect")
	reg(&cgPathCreateWithRoundedRect, cg, "CGPathCreateWithRoundedRect")
}

// DrawTopDown runs draw with ctx's state saved and its y axis pointing
// down over a layer height tall; flip turns a y-up context, which Core
// Animation gives a layer whose contents are not flipped, top down. Core
// Graphics offsets shadows in the context's base space, which points down
// when Core Animation flipped the context and up when flip did: draw
// learns which as baseUp.
func DrawTopDown(ctx uintptr, height float64, flip bool, draw func(baseUp bool)) {
	cgContextSaveGState(ctx)
	defer cgContextRestoreGState(ctx)
	if flip {
		cgContextTranslateCTM(ctx, 0, height)
		cgContextScaleCTM(ctx, 1, -1)
	}
	draw(flip)
}

// shadowOffset is a top-down offset in a base space that points up when
// baseUp is set.
func shadowOffset(x, y float64, baseUp bool) CGSize {
	if baseUp {
		y = -y
	}
	return CGSize{Width: x, Height: y}
}

// CGBlendMode of each CompositeOp.
var blendModes = [...]int32{
	driver.CompositeSourceOver:      0,
	driver.CompositeSourceIn:        18,
	driver.CompositeSourceOut:       19,
	driver.CompositeSourceAtop:      20,
	driver.CompositeDestinationOver: 21,
	driver.CompositeDestinationIn:   22,
	driver.CompositeDestinationOut:  23,
	driver.CompositeDestinationAtop: 24,
	driver.CompositeLighter:         27,
	driver.CompositeCopy:            17,
	driver.CompositeXor:             25,
	driver.CompositeMultiply:        1,
	driver.CompositeScreen:          2,
	driver.CompositeOverlay:         3,
	driver.CompositeDarken:          4,
	driver.CompositeLighten:         5,
	driver.CompositeColorDodge:      6,
	driver.CompositeColorBurn:       7,
	driver.CompositeHardLight:       9,
	driver.CompositeSoftLight:       8,
	driver.CompositeDifference:      10,
	driver.CompositeExclusion:       11,
	driver.CompositeHue:             12,
	driver.CompositeSaturation:      13,
	driver.CompositeColor:           14,
	driver.CompositeLuminosity:      15,
}

// CGTextDrawingMode values.
const (
	textFill   = 0
	textStroke = 1
)

// canvasState is what Save keeps besides Core Graphics' own state: the
// paints, which text and gradients need, and whether a shadow or an
// operator that clears outside the shape is on.
type canvasState struct {
	fill, stroke         driver.Color
	fillGrad, strokeGrad *driver.CanvasGradient
	shadow               bool
	composite            driver.CompositeOp
	// dash and phase are the dash pattern, which an offset sets again.
	dash  []float64
	phase float64
}

// CanvasHost is the view a drawing plays in: the screen's pixels a point,
// what draws it again once an image it waits for loads, which Owner
// names so it waits once, and whether the context's base space points up
// (DrawTopDown).
type CanvasHost struct {
	Ratio  float64
	Redraw func()
	Owner  any
	BaseUp bool
}

// Play draws ops into ctx, a view's context with its y axis pointing
// down, as a web canvas would: CoreGraphics adds path points through the
// transform current when they are added, as the canvas does, and a fill,
// stroke or clip keeps the path.
func Play(ctx uintptr, ops []driver.DrawOp, host CanvasHost) {
	PlayOver(ctx, ops, driver.CanvasSplit{}, host)
}

// PlayOver plays ops as Play does, but over their static part drawn
// already (a CanvasLayer): the ops before split.At that split skips set
// only the state.
func PlayOver(ctx uintptr, ops []driver.DrawOp, split driver.CanvasSplit, host CanvasHost) {
	if ctx == 0 {
		return
	}
	newPlayer(ctx, ops, split, host).run()
}

func newPlayer(ctx uintptr, ops []driver.DrawOp, split driver.CanvasSplit, host CanvasHost) *player {
	return &player{ctx: ctx, ops: ops, split: split, base: cgContextGetCTM(ctx), host: host,
		stack: []canvasState{{fill: 0x000000ff, stroke: 0x000000ff}}}
}

func (p *player) run() {
	ctx, ops := p.ctx, p.ops
	cgContextSaveGState(ctx)
	defer cgContextRestoreGState(ctx)
	defer p.releaseColors()
	cgContextBeginPath(ctx)
	p.color(cgContextSetFillColor, 0x000000ff)
	p.color(cgContextSetStrokeColor, 0x000000ff)
	for i := 0; i < len(ops); i++ {
		if p.split.Skips(i, &ops[i]) {
			continue
		}
		if n := p.lines(i); n > 0 {
			i += n - 1
			continue
		}
		p.play(i)
	}
	// What the drawing saved and never restored.
	for range len(p.stack) - 1 {
		cgContextRestoreGState(ctx)
	}
}

// A player calls Core Graphics as few times as it can, as a call can
// cost more than the op it plays draws: a run of lines in one call
// (lines), each color made once (cgColor), whether the path is empty
// asked only where the ops cannot tell (open), and the path the web's
// fill, stroke and clip keep copied only when a later op uses it (keep).
type player struct {
	ctx   uintptr
	ops   []driver.DrawOp
	split driver.CanvasSplit // what PlayOver leaves out
	// waited is set when an image the ops draw had not loaded.
	waited bool
	base   affine // the canvas's own coordinates
	host   CanvasHost
	stack  []canvasState
	path   pathState
	// colors are the CGColors made for the drawing, released at its end.
	colors map[driver.Color]uintptr
	// points and rects hold what one call adds.
	points []CGPoint
	rects  []CGRect
}

// pathState is what the player knows of the current path.
type pathState uint8

const (
	pathEmpty pathState = iota
	pathOpen
	// pathUnknown follows ops that may add nothing, such as an arc or a
	// rounded rectangle of no size: Core Graphics is asked.
	pathUnknown
)

// open reports whether the current path holds anything.
func (p *player) open() bool {
	if p.path == pathUnknown {
		p.path = pathEmpty
		if !cgContextIsPathEmpty(p.ctx) {
			p.path = pathOpen
		}
	}
	return p.path == pathOpen
}

func (p *player) st() *canvasState { return &p.stack[len(p.stack)-1] }

func (p *player) color(set func(ctx, color uintptr), c driver.Color) { set(p.ctx, p.cgColor(c)) }

// cgColor returns c's CGColor, made once for the drawing. The context
// keeps the colors it is given.
func (p *player) cgColor(c driver.Color) uintptr {
	ref, ok := p.colors[c]
	if !ok {
		if p.colors == nil {
			p.colors = map[driver.Color]uintptr{}
		}
		ref = CGColor(c)
		p.colors[c] = ref
	}
	return ref
}

func (p *player) releaseColors() {
	for _, ref := range p.colors {
		Release(ref)
	}
}

// keep runs fn, a call that uses up the path, at op at, and adds the path
// back when a later op builds on it or uses it.
func (p *player) keep(at int, fn func()) {
	if !p.open() {
		return
	}
	later := false
	for i := at + 1; i < len(p.ops) && p.ops[i].Kind != driver.DrawBeginPath; i++ {
		if p.ops[i].Kind.UsesPath() {
			later = true
			break
		}
	}
	if !later {
		fn()
		p.path = pathEmpty
		return
	}
	path := cgContextCopyPath(p.ctx)
	fn()
	cgContextAddPath(p.ctx, path)
	Release(path)
}

// lines adds the run of lines starting at op at in one call and returns
// how many ops it took, or 0: a MoveTo and the LineTos after it, or
// LineTos from one that starts the path, which moves there. Every
// MoveTo goes this way.
func (p *player) lines(at int) int {
	op := &p.ops[at]
	if op.Kind != driver.DrawMoveTo && (op.Kind != driver.DrawLineTo || p.open()) {
		return 0
	}
	pts := append(p.points[:0], CGPoint{X: float64(op.X), Y: float64(op.Y)})
	n := 1
	for ; at+n < len(p.ops) && p.ops[at+n].Kind == driver.DrawLineTo; n++ {
		o := &p.ops[at+n]
		pts = append(pts, CGPoint{X: float64(o.X), Y: float64(o.Y)})
	}
	cgContextAddLines(p.ctx, pts)
	p.points, p.path = pts, pathOpen
	return n
}

// moveTo starts a subpath at (x, y) without a registered call.
func (p *player) moveTo(x, y float64) {
	p.points = append(p.points[:0], CGPoint{X: x, Y: y})
	cgContextAddLines(p.ctx, p.points)
	p.path = pathOpen
}

// play plays op at; lines takes MoveTos and the LineTos that start a
// path.
func (p *player) play(at int) {
	op := &p.ops[at]
	ctx := p.ctx
	f := func(v float32) float64 { return float64(v) }
	switch op.Kind {
	case driver.DrawSave:
		cgContextSaveGState(ctx)
		p.stack = append(p.stack, *p.st())
	case driver.DrawRestore:
		if len(p.stack) > 1 {
			cgContextRestoreGState(ctx)
			p.stack = p.stack[:len(p.stack)-1]
		}
	case driver.DrawTranslate:
		cgContextTranslateCTM(ctx, f(op.X), f(op.Y))
	case driver.DrawScale:
		cgContextScaleCTM(ctx, f(op.X), f(op.Y))
	case driver.DrawRotate:
		cgContextRotateCTM(ctx, f(op.A))
	case driver.DrawTransform:
		cgContextConcatCTM(ctx, affine{A: f(op.X1), B: f(op.Y1), C: f(op.X2), D: f(op.Y2), X: f(op.X), Y: f(op.Y)})
	case driver.DrawSetTransform:
		cgContextConcatCTM(ctx, cgContextGetCTM(ctx).invert())
		cgContextConcatCTM(ctx, p.base)
		cgContextConcatCTM(ctx, affine{A: f(op.X1), B: f(op.Y1), C: f(op.X2), D: f(op.Y2), X: f(op.X), Y: f(op.Y)})
	case driver.DrawBeginPath:
		cgContextBeginPath(ctx)
		p.path = pathEmpty
	case driver.DrawLineTo:
		// The path holds points, or lines would have taken the op.
		cgContextAddLineToPoint(ctx, f(op.X), f(op.Y))
	case driver.DrawQuadTo:
		if !p.open() {
			p.moveTo(f(op.X1), f(op.Y1))
		}
		cgContextAddQuadCurve(ctx, f(op.X1), f(op.Y1), f(op.X), f(op.Y))
	case driver.DrawCubicTo:
		if !p.open() {
			p.moveTo(f(op.X1), f(op.Y1))
		}
		cgContextAddCurve(ctx, f(op.X1), f(op.Y1), f(op.X2), f(op.Y2), f(op.X), f(op.Y))
	case driver.DrawArc:
		// With y pointing down, CoreGraphics' clockwise is the
		// canvas's counterclockwise.
		var clockwise int32
		if op.CCW {
			clockwise = 1
		}
		cgContextAddArc(ctx, f(op.X), f(op.Y), max(f(op.R), 0), f(op.A0), f(op.A1), clockwise)
		p.path = pathUnknown
	case driver.DrawRect:
		p.rects = append(p.rects[:0], Rect(f(op.X), f(op.Y), f(op.W), f(op.H)))
		cgContextAddRects(ctx, p.rects)
		p.moveTo(f(op.X), f(op.Y))
	case driver.DrawRoundRect:
		w, h := max(f(op.W), 0), max(f(op.H), 0)
		r := min(max(f(op.R), 0), w/2, h/2)
		path := cgPathCreateWithRoundedRect(Rect(f(op.X), f(op.Y), w, h), r, r, 0)
		cgContextAddPath(ctx, path)
		Release(path)
		p.path = pathUnknown
	case driver.DrawEllipse:
		cgContextAddEllipseInRect(ctx, Rect(f(op.X-op.R), f(op.Y-op.RY), 2*f(op.R), 2*f(op.RY)))
		p.path = pathUnknown
	case driver.DrawClosePath:
		if p.open() {
			cgContextClosePath(ctx)
		}
	case driver.DrawFill:
		p.keep(at, func() { p.fillPath(op.EvenOdd) })
	case driver.DrawStroke:
		p.keep(at, p.strokePath)
	case driver.DrawClip:
		p.keep(at, func() {
			if op.EvenOdd {
				cgContextEOClip(ctx)
			} else {
				cgContextClip(ctx)
			}
		})
	case driver.DrawFillColor:
		p.st().fill, p.st().fillGrad = op.Color, nil
		p.color(cgContextSetFillColor, op.Color)
	case driver.DrawStrokeColor:
		p.st().stroke, p.st().strokeGrad = op.Color, nil
		p.color(cgContextSetStrokeColor, op.Color)
	case driver.DrawFillGradient:
		if op.Gradient != nil && len(op.Gradient.Stops) > 0 {
			p.st().fillGrad = op.Gradient
		}
	case driver.DrawStrokeGradient:
		if op.Gradient != nil && len(op.Gradient.Stops) > 0 {
			p.st().strokeGrad = op.Gradient
		}
	case driver.DrawLineWidth:
		cgContextSetLineWidth(ctx, f(op.W))
	case driver.DrawLineCap:
		cgContextSetLineCap(ctx, int32(op.Cap))
	case driver.DrawLineJoin:
		cgContextSetLineJoin(ctx, int32(op.Join))
	case driver.DrawMiterLimit:
		cgContextSetMiterLimit(ctx, f(op.W))
	case driver.DrawLineDash:
		var dash []float64
		if op.Dash != nil {
			for _, l := range *op.Dash {
				dash = append(dash, float64(l))
			}
		}
		p.setDash(dash, -1)
	case driver.DrawLineDashOffset:
		p.setDash(nil, f(op.W))
	case driver.DrawAlpha:
		cgContextSetAlpha(ctx, f(op.A))
	case driver.DrawComposite:
		if int(op.Composite) < len(blendModes) {
			p.st().composite = op.Composite
			cgContextSetBlendMode(ctx, blendModes[op.Composite])
		}
	case driver.DrawShadow:
		cgContextSetShadowWithColor(ctx, shadowOffset(f(op.X), f(op.Y), p.host.BaseUp), f(op.R), p.cgColor(op.Color))
		p.st().shadow = op.Color&0xff != 0
	case driver.DrawClear:
		cgContextClearRect(ctx, Rect(f(op.X), f(op.Y), f(op.W), f(op.H)))
	case driver.DrawFillText, driver.DrawStrokeText:
		p.text(op)
	case driver.DrawImage:
		p.image(op)
	}
}

// setDash sets the dash pattern to lengths, or its phase when phase is
// not negative.
func (p *player) setDash(lengths []float64, phase float64) {
	st := p.st()
	if phase >= 0 {
		st.phase = phase
	} else {
		st.dash = lengths
	}
	if len(st.dash) == 0 {
		cgContextSetLineDash(p.ctx, 0, nil, 0)
		return
	}
	cgContextSetLineDash(p.ctx, st.phase, &st.dash[0], uintptr(len(st.dash)))
}

// layered runs draw in a transparency layer when what it draws must be
// composited as a whole: under an operator that clears outside the
// shape, or a gradient under a shadow. The layer takes the shadow, alpha
// and operator; inside, drawing is plain.
func (p *player) layered(gradient bool, draw func()) {
	st := p.st()
	if !st.composite.Unbounded() && !(gradient && st.shadow) {
		draw()
		return
	}
	cgContextBeginTransparency(p.ctx, 0)
	cgContextSaveGState(p.ctx)
	cgContextSetBlendMode(p.ctx, 0)
	cgContextSetAlpha(p.ctx, 1)
	cgContextSetShadowWithColor(p.ctx, CGSize{}, 0, 0)
	draw()
	cgContextRestoreGState(p.ctx)
	cgContextEndTransparency(p.ctx)
}

func (p *player) fillPath(evenOdd bool) {
	g := p.st().fillGrad
	p.layered(g != nil, func() {
		if g == nil {
			if evenOdd {
				cgContextEOFillPath(p.ctx)
			} else {
				cgContextFillPath(p.ctx)
			}
			return
		}
		cgContextSaveGState(p.ctx)
		if evenOdd {
			cgContextEOClip(p.ctx)
		} else {
			cgContextClip(p.ctx)
		}
		p.gradient(g)
		cgContextRestoreGState(p.ctx)
	})
}

func (p *player) strokePath() {
	g := p.st().strokeGrad
	p.layered(g != nil, func() {
		if g == nil {
			cgContextStrokePath(p.ctx)
			return
		}
		cgContextSaveGState(p.ctx)
		cgContextReplacePathStroked(p.ctx)
		cgContextClip(p.ctx)
		p.gradient(g)
		cgContextRestoreGState(p.ctx)
	})
}

// Options of CGContextDrawLinearGradient and its radial one: the end
// colors continue past the ends.
const padEnds = 1 | 2

// gradient paints g over the clip, in its own transform.
func (p *player) gradient(g *driver.CanvasGradient) {
	m := g.Matrix()
	cgContextConcatCTM(p.ctx, affine{A: float64(m[0]), B: float64(m[1]), C: float64(m[2]), D: float64(m[3]),
		X: float64(m[4]), Y: float64(m[5])})
	stops := g.Stops
	x1, y1, x2, y2 := float64(g.X1), float64(g.Y1), float64(g.X2), float64(g.Y2)
	r1, r2 := max(float64(g.R1), 0), max(float64(g.R2), 0)
	if g.Spread != driver.SpreadPad && !g.Conic {
		// Repeats as more of the same stops, far enough to cover what
		// the canvas shows.
		const periods = 24
		stops = repeatStops(g, periods)
		dx, dy := x2-x1, y2-y1
		if g.Radial {
			r2 = r1 + (r2-r1)*float64(periods)
			x2, y2 = x1+dx*float64(periods), y1+dy*float64(periods)
		} else {
			x1, y1 = x1-dx*float64(periods/2), y1-dy*float64(periods/2)
			x2, y2 = x2+dx*float64(periods/2-1), y2+dy*float64(periods/2-1)
		}
	}
	grad := cgGradient(stops)
	if grad == 0 {
		return
	}
	defer Release(grad)
	switch {
	case g.Conic:
		if cgContextDrawConicGradient != nil {
			cgContextDrawConicGradient(p.ctx, grad, x1, y1, float64(g.Angle))
		} else {
			p.conicWedges(g)
		}
	case g.Radial:
		cgContextDrawRadialGradient(p.ctx, grad, x1, y1, r1, x2, y2, r2, padEnds)
	default:
		cgContextDrawLinearGradient(p.ctx, grad, x1, y1, x2, y2, padEnds)
	}
}

// repeatStops lays g's stops out over periods copies, mirrored every
// other one for a reflecting gradient, on offsets from 0 to 1.
func repeatStops(g *driver.CanvasGradient, periods int) []driver.GradientStop {
	var out []driver.GradientStop
	for k := range periods {
		flip := g.Spread == driver.SpreadReflect && k%2 == 1
		for i := range g.Stops {
			s := g.Stops[i]
			if flip {
				s = g.Stops[len(g.Stops)-1-i]
				s.Offset = 1 - s.Offset
			}
			s.Offset = (float32(k) + s.Offset) / float32(periods)
			out = append(out, s)
		}
	}
	return out
}

// conicWedges paints a conic gradient as thin wedges, a degree each, for
// systems without CGContextDrawConicGradient.
func (p *player) conicWedges(g *driver.CanvasGradient) {
	const n = 360
	cx, cy := float64(g.X1), float64(g.Y1)
	r := 1e5
	for i := range n {
		a0 := float64(g.Angle) + 2*math.Pi*float64(i)/n
		a1 := float64(g.Angle) + 2*math.Pi*float64(i+1)/n + 0.01
		p.color(cgContextSetFillColor, g.At((float64(i)+0.5)/n))
		cgContextBeginPath(p.ctx)
		cgContextMoveToPoint(p.ctx, cx, cy)
		cgContextAddArc(p.ctx, cx, cy, r, a0, a1, 0)
		cgContextClosePath(p.ctx)
		cgContextFillPath(p.ctx)
	}
}

// cgGradient returns a CGGradient of stops in sRGB, or 0 without stops.
func cgGradient(stops []driver.GradientStop) uintptr {
	if len(stops) == 0 {
		return 0
	}
	comps := make([]float64, 0, 4*len(stops))
	locs := make([]float64, 0, len(stops))
	for _, s := range stops {
		r, g, b, a := s.Color.Components()
		comps = append(comps, r, g, b, a)
		locs = append(locs, min(max(float64(s.Offset), 0), 1))
	}
	return cgGradientCreate(srgb, &comps[0], &locs[0], uintptr(len(stops)))
}

// text draws one line of text at the op's point, moved by its alignment:
// filled with the fill color, or a gradient through the glyphs, or
// stroked with the stroke settings.
func (p *player) text(op *driver.DrawOp) {
	if op.Text == "" {
		return
	}
	st := p.st()
	stroke := op.Kind == driver.DrawStrokeText
	color, grad := st.fill, st.fillGrad
	if stroke {
		color, grad = st.stroke, st.strokeGrad
	}
	str := attributed(&driver.AttributedText{Text: op.Text, Style: driver.TextStyle{Font: op.Font, Color: color}}, nil, nil)
	line := ctLineCreateWithAttrString(str)
	Release(str)
	defer Release(line)
	show := func() {
		// Glyphs draw upright in a context whose y axis points down.
		cgContextSetTextMatrix(p.ctx, affine{A: 1, D: -1})
		cgContextSetTextPosition(p.ctx, float64(op.X+op.X1), float64(op.Y+op.Y1))
		if stroke {
			cgContextSetTextDrawingMode(p.ctx, textStroke)
		}
		ctLineDraw(line, p.ctx)
		cgContextSetTextDrawingMode(p.ctx, textFill)
	}
	p.layered(grad != nil, func() {
		if grad == nil {
			show()
			return
		}
		// The glyphs drawn plain, then the gradient over them alone.
		cgContextBeginTransparency(p.ctx, 0)
		show()
		cgContextSaveGState(p.ctx)
		cgContextSetBlendMode(p.ctx, blendModes[driver.CompositeSourceIn])
		p.gradient(grad)
		cgContextRestoreGState(p.ctx)
		cgContextEndTransparency(p.ctx)
	})
}

// image draws op's image once it is loaded.
func (p *player) image(op *driver.DrawOp) {
	if op.Image == nil {
		return
	}
	img := canvasImage(*op.Image, p.host)
	if img == nil || img.image == 0 {
		p.waited = p.waited || img == nil || !img.failed
		return
	}
	f := func(v float32) float64 { return float64(v) }
	w, h := f(op.W), f(op.H)
	if w == 0 && h == 0 {
		w, h = float64(img.points.Width), float64(img.points.Height)
	}
	src := img.image
	if op.X2 != 0 || op.Y2 != 0 {
		// The part named in points of the image, in its pixels.
		k := img.width / max(float64(img.points.Width), 1e-9)
		src = cgImageCreateWithImageInRect(img.image, Rect(f(op.X1)*k, f(op.Y1)*k, f(op.X2)*k, f(op.Y2)*k))
		if src == 0 {
			return
		}
		defer Release(src)
	}
	p.layered(false, func() {
		cgContextSaveGState(p.ctx)
		// Images draw bottom up: flip them into the top-down space.
		cgContextTranslateCTM(p.ctx, f(op.X), f(op.Y)+h)
		cgContextScaleCTM(p.ctx, 1, -1)
		cgContextDrawImage(p.ctx, Rect(0, 0, w, h), src)
		cgContextRestoreGState(p.ctx)
	})
}

// canvasImages are the images canvases draw, decoded once each for the
// process at their full size, by ImageSource.Key; main thread only.
var canvasImages = map[string]*loadedImage{}

// loadedImage is an image canvases draw, or one loading.
type loadedImage struct {
	image         uintptr
	width, height float64 // pixels
	points        driver.Size
	failed        bool
	waiting       map[any]func()
	used          uint64
}

var canvasImageUse uint64

// maxCanvasImages bounds the images kept; the least recently drawn go.
const maxCanvasImages = 64

// canvasImage returns src's image, or nil while it loads, and has the
// host draw again once it has loaded.
func canvasImage(src driver.ImageSource, host CanvasHost) *loadedImage {
	key := src.Key()
	canvasImageUse++
	if im := canvasImages[key]; im != nil {
		im.used = canvasImageUse
		if im.image == 0 && !im.failed && host.Redraw != nil {
			im.waiting[host.Owner] = host.Redraw
		}
		return im
	}
	if len(canvasImages) >= maxCanvasImages {
		var oldest string
		var at uint64 = math.MaxUint64
		for k, im := range canvasImages {
			if im.image != 0 && im.used < at {
				oldest, at = k, im.used
			}
		}
		if im := canvasImages[oldest]; im != nil {
			Release(im.image)
			delete(canvasImages, oldest)
		}
	}
	im := &loadedImage{used: canvasImageUse, waiting: map[any]func(){}}
	if host.Redraw != nil {
		im.waiting[host.Owner] = host.Redraw
	}
	canvasImages[key] = im
	go func() {
		data, err := imageload.Load(context.Background(), src, nil)
		var d decoded
		if err == nil {
			d, err = decode(context.Background(), data, request{mode: driver.Stretch, ratio: max(host.Ratio, 1), scale: uriScale(src.URI)})
		}
		OnMain(func() {
			if err != nil || d.image == 0 {
				im.failed = true
				log.Printf("nsgo: canvas: image %s: %v", key, err)
			} else {
				im.image, im.width, im.height, im.points = d.image, d.width, d.height, d.points
				for _, f := range d.frames[min(len(d.frames), 1):] {
					Release(f)
				}
			}
			waiting := im.waiting
			im.waiting = nil
			for _, fn := range waiting {
				fn()
			}
		})
	}()
	return nil
}

// Tint paints c over what ctx holds in r, where it holds anything: a
// text's bound color over all of its glyphs.
func Tint(ctx uintptr, r CGRect, c driver.Color) {
	cgContextSaveGState(ctx)
	cgContextSetBlendMode(ctx, blendModes[driver.CompositeSourceIn])
	ref := CGColor(c)
	cgContextSetFillColor(ctx, ref)
	Release(ref)
	cgContextFillRect(ctx, r)
	cgContextRestoreGState(ctx)
}

// Damaged is the area of a canvas to draw again after its animated
// arguments played before now play after (driver.CanvasDamage), 2 points
// wider for anti-aliasing and rounded out to whole points; whole is set
// where only drawing all of it is right. An empty area draws nothing.
func Damaged(before, after []driver.DrawOp) (area CGRect, whole bool) {
	d, ok := driver.CanvasDamage(before, after)
	if !ok {
		return CGRect{}, true
	}
	if d.Size.Width <= 0 || d.Size.Height <= 0 {
		return CGRect{}, false
	}
	const aa = 2
	x0, y0 := math.Floor(float64(d.Origin.X)-aa), math.Floor(float64(d.Origin.Y)-aa)
	x1 := math.Ceil(float64(d.Origin.X+d.Size.Width) + aa)
	y1 := math.Ceil(float64(d.Origin.Y+d.Size.Height) + aa)
	return Rect(x0, y0, x1-x0, y1-y0), false
}
