//go:build windows

package windows

// Drawing the window's view tree with Direct2D, as NSH's render.rs: each
// frame draws the whole tree into the swap chain's back buffer and
// presents it. Each view draws, in this order:
//
//  1. its shadow and background,
//  2. its content, clipped to its bounds: text, image, canvas,
//  3. its children, clipped to its rounded bounds when it clips,
//  4. its scroll indicator,
//  5. its border, over the children,
//
// all inside an opacity layer when it is translucent. Coordinates are
// points: the context's DPI is the window's scale times 96. Transforms are
// 3x2 world transforms.
//
// A window with web views draws the same single pass, split at each web
// view (compose.go): reaching one records where it goes (compose), and
// the next thing drawn first moves the context to the next layer above it
// (realize: the clips and layers pushed so far are popped, the target
// changes, and they are pushed again). Without a composition, realize is
// one test of a nil pointer.

import (
	"math"
	"slices"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
)

// matrix is an affine transform: x' = a x + c y + e, y' = b x + d y + f,
// laid out as D2D1_MATRIX_3X2_F.
type matrix struct{ a, b, c, d, e, f float64 }

var identity = matrix{a: 1, d: 1}

func translation(x, y float64) matrix { return matrix{a: 1, d: 1, e: x, f: y} }

func scaling(x, y float64) matrix { return matrix{a: x, d: y} }

func (m matrix) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

// then returns m followed by n.
func (m matrix) then(n matrix) matrix {
	return matrix{
		a: m.a*n.a + m.b*n.c, b: m.a*n.b + m.b*n.d,
		c: m.c*n.a + m.d*n.c, d: m.c*n.b + m.d*n.d,
		e: m.e*n.a + m.f*n.c + n.e, f: m.e*n.b + m.f*n.d + n.f,
	}
}

func (m matrix) invert() matrix {
	det := m.a*m.d - m.b*m.c
	if det == 0 {
		return identity
	}
	return matrix{
		a: m.d / det, b: -m.b / det, c: -m.c / det, d: m.a / det,
		e: (m.c*m.f - m.d*m.e) / det, f: (m.b*m.e - m.a*m.f) / det,
	}
}

// axisAligned reports whether m only moves and grows, keeping rectangles
// rectangles the right way round.
func (m matrix) axisAligned() bool { return m.b == 0 && m.c == 0 && m.a > 0 && m.d > 0 }

// stretch returns how much m scales lengths, on average.
func (m matrix) stretch() float64 { return math.Sqrt(math.Abs(m.a*m.d - m.b*m.c)) }

func (m matrix) d2d() matrix3x2 {
	return matrix3x2{float32(m.a), float32(m.b), float32(m.c), float32(m.d), float32(m.e), float32(m.f)}
}

// bound returns the box around r mapped by m, rounded out.
func (m matrix) bound(r rect) rect {
	x0, y0 := math.Inf(1), math.Inf(1)
	x1, y1 := math.Inf(-1), math.Inf(-1)
	for _, c := range [4][2]int32{{r.Left, r.Top}, {r.Right, r.Top}, {r.Left, r.Bottom}, {r.Right, r.Bottom}} {
		x, y := m.apply(float64(c[0]), float64(c[1]))
		x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
	}
	return rect{int32(math.Floor(x0)), int32(math.Floor(y0)), int32(math.Ceil(x1)), int32(math.Ceil(y1))}
}

func colorOf(c driver.Color) colorF {
	return colorF{float32(c>>24&0xff) / 255, float32(c>>16&0xff) / 255, float32(c>>8&0xff) / 255, float32(c&0xff) / 255}
}

func visible(c driver.Color) bool { return c&0xff != 0 }

// infinite bounds a layer's content without limiting it.
var infinite = rectF{-1e7, -1e7, 1e7, 1e7}

// pushed is a layer or clip the painter pushed, with what it needs to
// push it again.
type pushed struct {
	clip    bool // an axis-aligned clip, else a layer
	m       matrix
	rect    rectF      // the clip's
	opacity float32    // the layer's
	mask    *comObject // the layer's geometric mask, held
	brush   *comObject // the layer's opacity brush, held
}

// painter is one frame's drawing pass.
type painter struct {
	w      *window
	g      *gfx
	dc     *comObject
	scale  float32 // pixels per point
	m      matrix  // the world transform: the drawn view's points to the target's DIPs
	brush  *comObject
	pushed []pushed
	// alpha is the opacity of what is drawn now: its views', multiplied.
	alpha float32
	// composed are the web views reached in this pass, in paint order,
	// with where they go.
	composed []composed
	// split is the pass's layers when the window has a composition.
	split *split
	// components are the native components' windows the pass reached,
	// with where they go.
	components []placedComponent
	// perspective is the depth the views drawn now are seen from, at the
	// center of perspectiveOf, their parent's size; 0 is none.
	perspective   driver.Points
	perspectiveOf driver.Size
	// areas are what a partial frame draws, in the window's points, nil
	// for all of it; areaClip says how pushAreas clipped to them
	// (damage.go).
	areas    []rectF
	areaClip int
}

// composed is a web view a pass reached.
type composed struct {
	web *webView
	at  placement
}

// split is the layers a window's pass draws into (realize).
type split struct {
	c             *composition
	width, height uint32 // the window's, in pixels
	// layers holds, for each layer started, how many web views are below
	// it.
	layers []int
	// pending is set when a web view was reached: the next drawing
	// starts a layer.
	pending bool
	// ended is the first failure of ending a target's drawing.
	ended uintptr
}

func newPainter(w *window, g *gfx) *painter {
	p := &painter{w: w, g: g, dc: g.dc, scale: float32(w.scale), m: identity, alpha: 1}
	black := colorF{a: 1}
	g.dc.call(rtCreateSolidColorBrush, uintptr(unsafe.Pointer(&black)), 0, uintptr(unsafe.Pointer(&p.brush)))
	return p
}

func (p *painter) release() {
	for len(p.pushed) > 0 {
		p.pop()
	}
	p.popAreas()
	releaseAll(&p.brush)
}

// solid returns the pass's brush in color c.
func (p *painter) solid(c driver.Color) *comObject {
	col := colorOf(c)
	p.brush.call(brushSetColor, uintptr(unsafe.Pointer(&col)))
	return p.brush
}

// setTransform makes m the world transform.
func (p *painter) setTransform(m matrix) {
	p.m = m
	t := m.d2d()
	p.dc.call(rtSetTransform, uintptr(unsafe.Pointer(&t)))
}

// pushLayer draws what follows at opacity, inside mask (in the current
// coordinates) unless it is nil.
func (p *painter) pushLayer(opacity float32, mask *comObject) {
	p.pushLayerRaw(opacity, mask, nil)
	mask.addRef()
	p.pushed = append(p.pushed, pushed{m: p.m, opacity: opacity, mask: mask})
}

// pushLayerBrush draws what follows at opacity, as much as brush's alpha
// says.
func (p *painter) pushLayerBrush(opacity float32, brush *comObject) {
	p.pushLayerRaw(opacity, nil, brush)
	brush.addRef()
	p.pushed = append(p.pushed, pushed{m: p.m, opacity: opacity, brush: brush})
}

func (p *painter) pushLayerRaw(opacity float32, mask, brush *comObject) {
	params := layerParameters1{contentBounds: infinite, maskAntialias: d2dAntialiasPerPrimitive,
		maskTransform: identity.d2d(), opacity: opacity}
	if mask != nil {
		params.mask = mask.ptr()
	}
	if brush != nil {
		params.opacityBrush = brush.ptr()
	}
	p.dc.call(dcPushLayer, uintptr(unsafe.Pointer(&params)), 0)
}

// pushClipRect clips what follows to r, in the current coordinates:
// exactly while the transform keeps rectangles upright, else through a
// geometry.
func (p *painter) pushClipRect(r rectF) {
	if p.m.b == 0 && p.m.c == 0 {
		// Aliased, as a pixel-aligned layer's bounds clip.
		p.dc.call(rtPushAxisAlignedClip, uintptr(unsafe.Pointer(&r)), d2dAntialiasAliased)
		p.pushed = append(p.pushed, pushed{clip: true, m: p.m, rect: r})
		return
	}
	g := p.roundedGeometry(r, 0)
	p.pushLayer(1, g)
	g.release()
}

// pushClipRounded clips what follows to r with its corners rounded by
// radius.
func (p *painter) pushClipRounded(r rectF, radius float32) {
	if radius <= 0 {
		p.pushClipRect(r)
		return
	}
	g := p.roundedGeometry(r, radius)
	p.pushLayer(1, g)
	g.release()
}

// pop pops the last push.
func (p *painter) pop() {
	n := len(p.pushed)
	if n == 0 {
		return
	}
	top := p.pushed[n-1]
	p.pushed = p.pushed[:n-1]
	if top.clip {
		p.dc.call(rtPopAxisAlignedClip)
		return
	}
	p.dc.call(rtPopLayer)
	top.mask.release()
	top.brush.release()
}

// flatten pops every layer and clip pushed so far, runs between, and
// pushes them again as they were: exact for clips, while a translucent
// layer splits in two.
func (p *painter) flatten(between func()) {
	for i := len(p.pushed) - 1; i >= 0; i-- {
		if p.pushed[i].clip {
			p.dc.call(rtPopAxisAlignedClip)
		} else {
			p.dc.call(rtPopLayer)
		}
	}
	between()
	current := p.m
	for _, q := range p.pushed {
		p.setTransform(q.m)
		if q.clip {
			r := q.rect
			p.dc.call(rtPushAxisAlignedClip, uintptr(unsafe.Pointer(&r)), d2dAntialiasAliased)
		} else {
			p.pushLayerRaw(q.opacity, q.mask, q.brush)
		}
	}
	p.setTransform(current)
}

// clipBounds returns what the clips pushed so far leave visible, in the
// target's DIPs.
func (p *painter) clipBounds() rectF {
	r := infinite
	for _, q := range p.pushed {
		switch {
		case q.clip:
			r = r.intersect(mapBounds(q.m, q.rect))
		case q.mask != nil:
			var b rectF
			t := q.m.d2d()
			if !failed(q.mask.call(geometryGetBounds, uintptr(unsafe.Pointer(&t)), uintptr(unsafe.Pointer(&b)))) {
				r = r.intersect(b)
			}
		}
	}
	return r
}

// compose records that web view wv, width by height points at world with
// corners of radius, is drawn here: what follows goes into the next
// layer.
func (p *painter) compose(wv *webView, world matrix, width, height, radius float32) {
	s := float64(p.scale)
	// The page's pixels are the view's points times the scale: the linear
	// part stays, the translation is in pixels.
	m := world
	m.e, m.f = m.e*s, m.f*s
	if world.a == 1 && world.d == 1 && world.b == 0 && world.c == 0 {
		// Only moved: on whole pixels, so that the page stays sharp.
		m.e, m.f = math.Round(m.e), math.Round(m.f)
	}
	c := p.clipBounds()
	sc := float32(s)
	clip := rectF{max(c.left*sc, 0), max(c.top*sc, 0), min(c.right*sc, 1e7), min(c.bottom*sc, 1e7)}
	w, h := int32(math.Round(float64(width)*s)), int32(math.Round(float64(height)*s))
	if w <= 0 || h <= 0 || mapBounds(m, rectF{0, 0, float32(w), float32(h)}).intersect(clip).empty() {
		return
	}
	p.composed = append(p.composed, composed{wv, placement{m: m, width: w, height: h, clip: clip,
		radius: radius * sc, alpha: p.alpha}})
	if p.split != nil {
		p.split.pending = true
	}
}

// realize is called before something is drawn: after a web view, the pass
// goes on in the next layer, made as needed, with its clips and layers
// pushed again there.
func (p *painter) realize() {
	s := p.split
	if s == nil || !s.pending {
		return
	}
	s.pending = false
	target, err := s.c.layerTarget(p.g, len(s.layers), s.width, s.height, p.scale)
	if err != nil {
		// Drawn below the web view instead.
		return
	}
	s.layers = append(s.layers, len(p.composed))
	p.flatten(func() {
		if hr := p.dc.call(rtEndDraw, 0, 0); failed(hr) && s.ended == 0 {
			s.ended = hr
		}
		p.dc.call(dcSetTarget, target.ptr())
		p.dc.call(rtBeginDraw)
		t := identity.d2d()
		p.dc.call(rtSetTransform, uintptr(unsafe.Pointer(&t)))
		clear := colorF{}
		p.dc.call(rtClear, uintptr(unsafe.Pointer(&clear)))
	})
}

// roundedGeometry returns r with corners of radius as a geometry the
// caller releases; nil when Direct2D cannot make it, which a layer takes
// as no mask.
func (p *painter) roundedGeometry(r rectF, radius float32) *comObject {
	radius = max(min(radius, (r.right-r.left)/2, (r.bottom-r.top)/2), 0)
	rr := roundedRect{rect: r, radiusX: radius, radiusY: radius}
	var g *comObject
	if failed(p.g.factory.call(d2dFactoryCreateRoundedRectangleGeometry, uintptr(unsafe.Pointer(&rr)),
		uintptr(unsafe.Pointer(&g)))) {
		return nil
	}
	return g
}

func (p *painter) fillRect(r rectF, c driver.Color) {
	if visible(c) && !r.empty() {
		p.dc.call(rtFillRectangle, uintptr(unsafe.Pointer(&r)), p.solid(c).ptr())
	}
}

// fillRounded fills r with corners of radius.
func (p *painter) fillRounded(r rectF, radius float32, c driver.Color) {
	if !visible(c) || r.empty() {
		return
	}
	radius = max(min(radius, (r.right-r.left)/2, (r.bottom-r.top)/2), 0)
	if radius <= 0 {
		p.fillRect(r, c)
		return
	}
	rr := roundedRect{rect: r, radiusX: radius, radiusY: radius}
	p.dc.call(rtFillRoundedRectangle, uintptr(unsafe.Pointer(&rr)), p.solid(c).ptr())
}

func (p *painter) fillGeometry(g *comObject, c driver.Color) {
	if g != nil && visible(c) {
		p.dc.call(rtFillGeometry, g.ptr(), p.solid(c).ptr(), 0)
	}
}

// paint answers WM_PAINT.
func (w *window) paint() {
	var ps paintStruct
	beginPaint.Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	defer endPaint.Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	w.frame()
}

// frame draws the whole tree into the swap chain and presents it.
func (w *window) frame() {
	var client rect
	getClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
	if client.empty() {
		return
	}
	g, err := devices()
	if err != nil {
		return
	}
	if w.baseComposed && w.composition == nil {
		if _, err := w.compose(); err != nil {
			return
		}
	}
	if w.swap != nil && w.swap.stale() {
		w.swap.release()
		w.swap = nil
	}
	if w.swap == nil {
		if w.swap, err = newSwapChain(g, w.hwnd, uint32(client.Right), uint32(client.Bottom), w.baseComposed); err != nil {
			return
		}
	}
	if w.baseComposed {
		w.composition.setBase(w.swap.swap)
	}
	// A new or resized chain's buffers hold nothing drawn.
	fresh := w.swap.width != uint32(max(client.Right, 1)) || w.swap.height != uint32(max(client.Bottom, 1)) ||
		!w.swap.drawn
	if err := w.swap.resize(uint32(client.Right), uint32(client.Bottom)); err != nil {
		w.lost()
		return
	}
	if w.damage.idle() && !fresh && w.composition == nil && !w.swap.composed && len(w.hosts) == 0 {
		// The system's paint: DWM has what was presented.
		return
	}
	scale := float32(w.scale)
	target, err := w.swap.targetBitmap(g, scale)
	if err != nil {
		w.lost()
		return
	}
	// Presenting waits for the display, which is not the frame's work.
	measured := w.measuring(true)
	dc := g.dc
	dc.call(dcSetTarget, target.ptr())
	dc.callF(rtSetDpi, 0b11, f32(96*scale), f32(96*scale))
	dc.call(rtBeginDraw)
	p := newPainter(w, g)
	// A window with web views or a material has a composition: the base
	// target is the window's own swap chain, and web views split the pass
	// into layers above it.
	c := w.composition
	if c != nil {
		p.split = &split{c: c, width: uint32(client.Right), height: uint32(client.Bottom)}
	}
	clear := colorF{1, 1, 1, 1}
	if w.baseComposed && w.backdrop != backdropNone {
		clear = colorF{}
	}
	whole := fresh || c != nil || w.swap.composed || len(w.hosts) > 0
	changed, drawn, changedAll, all := w.frameDamage(client, whole)
	if all {
		p.setTransform(identity)
		dc.call(rtClear, uintptr(unsafe.Pointer(&clear)))
	} else {
		p.pushAreas(drawn, clear)
	}
	if root := w.view(w.root); root != nil {
		f := frameOf(root)
		p.draw(root, identity, f.Origin.X, f.Origin.Y)
	}
	p.drawAlert(driver.Size{Width: driver.Points(float64(client.Right) / w.scale), Height: driver.Points(float64(client.Bottom) / w.scale)})
	p.drawCaptionButtons(client.Right)
	p.drawPerf()
	p.release()
	hr := dc.call(rtEndDraw, 0, 0)
	dc.call(dcSetTarget, 0)
	var layers []int
	if p.split != nil {
		layers = p.split.layers
		if hr == 0 {
			hr = p.split.ended
		}
	}
	measured()
	if hr == d2dErrRecreateTarget {
		w.lost()
		return
	}
	if c != nil {
		if hr := c.present(len(layers)); failed(hr) {
			w.lost()
			return
		}
	}
	if hr := w.swap.present(changed, changedAll); hr == dxgiErrDeviceRemoved || hr == dxgiErrDeviceReset {
		w.lost()
		return
	}
	w.composedPlaced(p.composed, layers)
	w.placeComponents(p.components)
}

// composedPlaced tells the web views the frame just shown drew where they
// go, and those it no longer drew, and stacks them with the layers drawn,
// layers[i] being how many web views are below layer i.
func (w *window) composedPlaced(drawn []composed, layers []int) {
	old := w.composed
	w.composed = w.composed[:0:0]
	for _, c := range drawn {
		w.composed = append(w.composed, c.web)
	}
	if len(old) == 0 && len(drawn) == 0 && w.composition == nil {
		return
	}
	for _, wv := range old {
		if !slices.Contains(w.composed, wv) {
			wv.composeAt(nil)
		}
	}
	visuals := make([]*comObject, 0, len(drawn))
	for _, c := range drawn {
		c.web.composeAt(&c.at)
		if c.web.portal != nil {
			visuals = append(visuals, c.web.portal.outer)
		}
	}
	if w.composition != nil {
		w.composition.arrange(visuals, layers)
	}
}

// lost drops the swap chains and the devices, and draws again on new
// ones.
func (w *window) lost() {
	if w.swap != nil {
		w.swap.release()
		w.swap = nil
	}
	if w.composition != nil {
		w.composition.deviceLost()
	}
	deviceLost()
	w.redrawAll()
}

// placing is where a view draws in a pass: its style, its opacity and
// its transform. world is the world transform of its points when flat,
// else its parent's moved to its origin, under local.
type placing struct {
	s             *driver.ViewStyle
	opacity       float32
	local         driver.Matrix3D
	world         matrix
	flat, filters bool
}

// place returns where v draws; parent is the world transform of v's
// parent's coordinates, x, y where v's origin goes in them, and
// perspective and of the depth its parent's children are seen from at
// the center of that size. It reports false when v draws nothing.
func (w *window) place(v *view, parent matrix, x, y, perspective driver.Points, of driver.Size) (placing, bool) {
	s := w.styleOf(v)
	opacity := s.Opacity
	size := frameOf(v).Size
	local := s.Matrix(size)
	if k, ok := v.keyframe(); ok {
		local = k.Matrix(size).Mul(local)
		opacity *= k.Opacity
	}
	if perspective > 0 {
		local = driver.ChildPerspective(perspective, of, driver.Point{X: x, Y: y}).Mul(local)
	}
	if fd := v.fading; fd != nil {
		// Appearing or going: faded, or scaled about its center first.
		if fd.tr.Property == driver.LayoutScale {
			if fd.shown <= 0 {
				return placing{}, false
			}
			cx, cy := float64(size.Width)/2, float64(size.Height)/2
			local = local.Mul(driver.Translate3D(cx, cy, 0)).Mul(driver.Matrix3D{fd.shown, 0, 0, 0, 0, fd.shown, 0, 0,
				0, 0, 1, 0, 0, 0, 0, 1}).Mul(driver.Translate3D(-cx, -cy, 0))
		} else {
			opacity *= float32(min(max(fd.shown, 0), 1))
		}
	}
	if s.Hidden || opacity <= 0 || s.BackfaceHidden && facingAway(local) {
		return placing{}, false
	}
	at := translation(float64(x), float64(y)).then(parent)
	pl := placing{s: s, opacity: opacity, local: local, world: at,
		filters: s.Effects != nil && len(s.Effects.Filters) > 0}
	a, b, c, d, e, f, flat := local.Affine()
	if pl.flat = flat; flat {
		pl.world = matrix{a, b, c, d, e, f}.then(at)
		pl.local = driver.Identity3D
	}
	return pl, true
}

// draw paints v and its subtree; parent is the world transform of v's
// parent's coordinates, and x, y where v's origin goes in them. A view
// with filters or turned out of its plane is drawn through effects.
func (p *painter) draw(v *view, parent matrix, x, y driver.Points) {
	if p.skips(v) {
		return
	}
	pl, ok := p.w.place(v, parent, x, y, p.perspective, p.perspectiveOf)
	if !ok {
		return
	}
	s, opacity, world := pl.s, pl.opacity, pl.world
	if pl.filters || !pl.flat {
		p.realize()
		p.setTransform(world)
		if opacity < 1 {
			p.pushLayer(opacity, nil)
		}
		alpha := p.alpha
		p.alpha *= opacity
		var list []driver.Filter
		if pl.filters {
			list = s.Effects.Filters
		}
		p.drawFiltered(world, pl.local, list, func() { p.drawContent(v, s, identity) })
		p.alpha = alpha
		if opacity < 1 {
			p.pop()
		}
		return
	}
	// Drawn above a web view reached before it, if any.
	p.realize()
	p.setTransform(world)
	if opacity < 1 {
		p.pushLayer(opacity, nil)
	}
	alpha := p.alpha
	p.alpha *= opacity
	size := frameOf(v).Size
	area := rectF{float32(v.overflow.Origin.X), float32(v.overflow.Origin.Y),
		float32(v.overflow.Origin.X + v.overflow.Size.Width), float32(v.overflow.Origin.Y + v.overflow.Size.Height)}
	if area.empty() {
		area = rectF{0, 0, float32(size.Width), float32(size.Height)}
	}
	if s.BlendMode == driver.BlendNormal || !p.drawBlended(s.BlendMode, world, area, func() { p.drawContent(v, s, world) }) {
		p.drawContent(v, s, world)
	}
	p.alpha = alpha
	if opacity < 1 {
		p.pop()
	}
}

// facingAway reports whether m turns a box's back to the viewer.
func facingAway(m driver.Matrix3D) bool {
	x0, y0 := m.Apply(0, 0)
	x1, y1 := m.Apply(1, 0)
	x2, y2 := m.Apply(0, 1)
	return (x1-x0)*(y2-y0)-(y1-y0)*(x2-x0) < 0
}

// drawContent paints v at world, the transform already set: its shadows
// and background, gradients and inset shadows, its content and children
// (clipped to its shape when it clips), then its border and outline.
func (p *painter) drawContent(v *view, s *driver.ViewStyle, world matrix) {
	size := frameOf(v).Size
	width, height := float32(size.Width), float32(size.Height)
	b := newBox(s, width, height)
	radius, simple := b.simple()
	radius = max(min(radius, width/2, height/2), 0)
	bounds := rectF{0, 0, width, height}
	e := s.Effects
	if e == nil {
		e = &driver.Effects{}
	}
	masked := e.Mask != nil && p.pushMask(e.Mask, width, height)
	filled := visible(s.Background) || len(e.Gradients) > 0
	if filled && visible(s.ShadowColor) && simple && width > 0 && height > 0 {
		// React Native's shadow, under a fill only, as iOS draws a shadow
		// of a layer with a background.
		p.drawShadow(v, s, width, height, radius)
	}
	p.drawBoxShadows(v, s, &b, e.Shadows, false)
	// Glass reads what is behind it, its background included, in place
	// of its blur.
	glass := s.Glass != driver.GlassNone
	if !glass {
		p.drawBackdrop(e, &b, world)
	}
	p.fillBox(&b, s.Background, nil)
	p.drawGradients(&b, e.Gradients)
	if glass && !p.drawGlass(v, s, &b, world) {
		p.drawBackdrop(e, &b, world)
	}
	p.drawBoxShadows(v, s, &b, e.Shadows, true)
	v.pruneShadows()
	// An image is clipped to its corners without Clip, as its layer clips
	// it on Apple and as Linux and the web clip it.
	clip := s.Clip || v.kind == driver.KindScroll || v.kind == driver.KindImage
	if clip {
		p.pushClipBox(&b)
	} else {
		// The view's own content stays inside it.
		p.pushClipRect(bounds)
	}
	switch v.kind {
	case driver.KindImage:
		bw := p.w.borderEdges(s)
		p.drawImage(v, s.TintColor, rectF{bw[3], bw[0], width - bw[1], height - bw[2]})
	case driver.KindCanvas:
		if v.drawing != nil {
			p.canvas(v, width, height)
		}
	case driver.KindShader:
		p.drawShader(v, width, height)
	case driver.KindWeb:
		if wv := v.web; wv != nil && wv.composed() {
			corners := float32(0)
			if s.Clip {
				corners = radius
			}
			p.compose(wv, world, width, height, corners)
		} else if wv != nil && wv.failed {
			p.drawWebPlaceholder(width, height)
		}
	case driver.KindVideo:
		if v.video != nil {
			p.drawVideo(v.video, width, height)
		}
	case driver.KindNative:
		if c := v.component; c != nil && c.hwnd != 0 {
			corners := float32(0)
			if s.Clip {
				corners = radius
			}
			p.placeComponent(c, world, width, height, corners)
		}
	case driver.KindInput:
		if v.field != nil {
			v.field.draw(p, world)
		}
	}
	if v.text != nil {
		x, y := float32(v.content.Origin.X), float32(v.content.Origin.Y)
		p.drawSelection(v, x, y)
		if v.animated.Has(driver.PropTextColor) {
			p.drawTinted(world, v.animated.TextColor, func() { v.text.draw(p, x, y) })
		} else {
			v.text.draw(p, x, y)
		}
	}
	if !clip {
		p.pop()
	}
	// The children, with those going drawn where they were among them,
	// in the view's perspective.
	perspective, of := p.perspective, p.perspectiveOf
	p.perspective, p.perspectiveOf = s.ChildPerspective, size
	next := 0
	drawLeaving := func(upTo int) {
		for ; next < len(v.leaving) && v.leaving[next].at <= upTo; next++ {
			c := v.leaving[next].v
			p.draw(c, world, c.frame.Origin.X-v.offset.X, c.frame.Origin.Y-v.offset.Y)
		}
	}
	for i, c := range v.children {
		drawLeaving(i)
		f := frameOf(c)
		p.draw(c, world, f.Origin.X-v.offset.X, f.Origin.Y-v.offset.Y)
	}
	drawLeaving(math.MaxInt)
	p.perspective, p.perspectiveOf = perspective, of
	indicator := v.kind == driver.KindScroll && p.w.showsBar(v)
	if indicator || p.w.borderEdges(s) != ([4]float32{}) || s.Outline.Visible() {
		// A scroll indicator, a border over a web view.
		p.realize()
	}
	p.setTransform(world)
	if v.kind == driver.KindScroll {
		p.drawEdges(v, world)
		p.drawSpinner(v, world)
	}
	if indicator {
		p.drawIndicator(v)
	}
	if clip {
		p.pop()
	}
	if dashed := s.BorderStyle == driver.BorderDashed || s.BorderStyle == driver.BorderDotted; dashed || !simple {
		p.drawShapedBorder(s, &b)
	} else {
		p.drawBorder(s, width, height, radius)
	}
	if o := s.Outline; o.Visible() {
		p.strokeBox(&b, -(float32(o.Offset) + float32(o.Width)/2), float32(o.Width), o.Color, o.Style)
	}
	if masked {
		p.pop()
	}
}

// drawShapedBorder draws a dashed or dotted border, or one around corners
// a rounded rectangle cannot draw: a stroke along the middle of a uniform
// border, else the ring between the box and its padding box, cut to each
// side's part where their colors differ. Dashes take the widest side.
func (p *painter) drawShapedBorder(s *driver.ViewStyle, b *box) {
	widths := p.w.borderEdges(s)
	if widths == ([4]float32{}) || b.w <= 0 || b.h <= 0 {
		return
	}
	colors := [4]driver.Color{s.BorderColor, s.BorderColor, s.BorderColor, s.BorderColor}
	if s.Borders != (driver.Edges{}) {
		c := s.BorderColors
		colors = [4]driver.Color{c.Top, c.Right, c.Bottom, c.Left}
	}
	uniform := widths[0] == widths[1] && widths[1] == widths[2] && widths[2] == widths[3] &&
		colors[0] == colors[1] && colors[1] == colors[2] && colors[2] == colors[3]
	if dashed := s.BorderStyle == driver.BorderDashed || s.BorderStyle == driver.BorderDotted; dashed || uniform {
		w := max(widths[0], widths[1], widths[2], widths[3])
		p.strokeBox(b, w/2, w, colors[0], s.BorderStyle)
		return
	}
	edges := driver.Edges{Top: driver.Points(widths[0]), Right: driver.Points(widths[1]), Bottom: driver.Points(widths[2]),
		Left: driver.Points(widths[3])}
	segs := driver.RoundedRect(nil, 0, 0, float64(b.w), float64(b.h), b.radii, b.continuous, false)
	x, y := float64(edges.Left), float64(edges.Top)
	if iw, ih := float64(b.w)-x-float64(edges.Right), float64(b.h)-y-float64(edges.Bottom); iw > 0 && ih > 0 {
		segs = driver.EllipticRect(segs, x, y, iw, ih, driver.InnerRadii(b.radii, edges), b.continuous, false)
	}
	ring := p.pathGeometry(d2dFillAlternate, func(sink *comObject) { addSegments(sink, segs) })
	if ring == nil {
		return
	}
	defer ring.release()
	for i, quad := range sideQuads(b.w, b.h, widths) {
		if widths[i] <= 0 || !visible(colors[i]) {
			continue
		}
		clip := p.polygon(quad[:])
		if clip == nil {
			continue
		}
		p.pushLayer(1, clip)
		clip.release()
		p.fillGeometry(ring, colors[i])
		p.pop()
	}
}

// drawIndicator paints a scroll bar's thumb while the scroll view moves,
// while the pointer is over it, or while the thumb is dragged.
func (p *painter) drawIndicator(v *view) {
	thumb, _, ok := v.bar().Indicator()
	if !ok || !p.w.showsBar(v) {
		return
	}
	bar := rectF{float32(thumb.Origin.X), float32(thumb.Origin.Y),
		float32(thumb.Origin.X + thumb.Size.Width), float32(thumb.Origin.Y + thumb.Size.Height)}
	radius := min(thumb.Size.Width, thumb.Size.Height) / 2
	c := driver.RGBA(0x000000, 0.45*v.barAlpha)
	if p.w.shownDark() {
		c = driver.RGBA(0xffffff, 0.55*v.barAlpha)
	}
	p.fillRounded(bar, float32(radius), c)
}

// shadowKey is what a shadow's drawing depends on.
type shadowKey struct {
	width, height, radius, sigma, scale float32
	color                               driver.Color
}

// shadow is a view's shadow: its shape filled in its color, recorded,
// blurred by an effect.
type shadow struct {
	key        shadowKey
	generation uint64
	list       *comObject // ID2D1CommandList
	blur       *comObject // ID2D1Effect
	image      *comObject // the image drawn: the blur's output, or the list
}

func (sh *shadow) release() {
	if sh != nil {
		releaseAll(&sh.image, &sh.blur, &sh.list)
	}
}

// drawShadow draws v's shadow: React Native's shadowRadius is the blur's
// standard deviation, as Core Animation's shadowRadius is half CSS's blur.
func (p *painter) drawShadow(v *view, s *driver.ViewStyle, width, height, radius float32) {
	key := shadowKey{width, height, radius, max(float32(s.ShadowRadius), 0), p.scale, s.ShadowColor}
	if v.shadow == nil || v.shadow.key != key || v.shadow.generation != gfxGeneration {
		v.shadow.release()
		v.shadow = p.makeShadow(key)
		if v.shadow == nil {
			return
		}
	}
	at := pointF{float32(s.ShadowOffset.X), float32(s.ShadowOffset.Y)}
	p.dc.call(dcDrawImage, v.shadow.image.ptr(), uintptr(unsafe.Pointer(&at)), 0, d2dInterpolationLinear, 0)
}

// makeShadow records key's shape on the second context and blurs it.
func (p *painter) makeShadow(key shadowKey) *shadow {
	sh := &shadow{key: key, generation: gfxGeneration}
	bake := p.g.bake
	if failed(bake.call(dcCreateCommandList, uintptr(unsafe.Pointer(&sh.list)))) {
		return nil
	}
	bake.call(dcSetTarget, sh.list.ptr())
	bake.callF(rtSetDpi, 0b11, f32(96*key.scale), f32(96*key.scale))
	bake.call(rtBeginDraw)
	t := identity.d2d()
	bake.call(rtSetTransform, uintptr(unsafe.Pointer(&t)))
	col := colorOf(key.color)
	var brush *comObject
	bake.call(rtCreateSolidColorBrush, uintptr(unsafe.Pointer(&col)), 0, uintptr(unsafe.Pointer(&brush)))
	if brush != nil {
		rr := roundedRect{rect: rectF{0, 0, key.width, key.height}, radiusX: key.radius, radiusY: key.radius}
		bake.call(rtFillRoundedRectangle, uintptr(unsafe.Pointer(&rr)), brush.ptr())
		brush.release()
	}
	hr := bake.call(rtEndDraw, 0, 0)
	bake.call(dcSetTarget, 0)
	if failed(hr) || failed(sh.list.call(commandListClose)) {
		sh.release()
		return nil
	}
	if key.sigma <= 0 {
		sh.list.addRef()
		sh.image = sh.list
		return sh
	}
	if failed(p.dc.call(dcCreateEffect, uintptr(unsafe.Pointer(&clsidGaussianBlur)), uintptr(unsafe.Pointer(&sh.blur)))) {
		sh.release()
		return nil
	}
	const standardDeviation = 0
	sigma := key.sigma
	sh.blur.call(effectSetValue, standardDeviation, d2dPropertyFloat, uintptr(unsafe.Pointer(&sigma)), 4)
	sh.blur.call(effectSetInput, 0, sh.list.ptr(), 1)
	sh.blur.call(effectGetOutput, uintptr(unsafe.Pointer(&sh.image)))
	if sh.image == nil {
		sh.release()
		return nil
	}
	return sh
}

// borderEdges returns the width of each side's drawn border in points,
// top, right, bottom, left: a side without a visible color has none, and
// one that has is at least a pixel wide.
func (w *window) borderEdges(s *driver.ViewStyle) [4]float32 {
	side := func(width driver.Points, c driver.Color) float32 {
		if width <= 0 || !visible(c) {
			return 0
		}
		return max(float32(width), float32(1/w.scale))
	}
	if s.Borders == (driver.Edges{}) {
		d := side(s.BorderWidth, s.BorderColor)
		return [4]float32{d, d, d, d}
	}
	b, c := s.Borders, s.BorderColors
	return [4]float32{side(b.Top, c.Top), side(b.Right, c.Right), side(b.Bottom, c.Bottom), side(b.Left, c.Left)}
}

// drawBorder paints a view's border inside its bounds, over its
// children, as NSH's draw_borders does with solid borders.
func (p *painter) drawBorder(s *driver.ViewStyle, width, height, radius float32) {
	widths := p.w.borderEdges(s)
	if widths == ([4]float32{}) || width <= 0 || height <= 0 {
		return
	}
	colors := [4]driver.Color{s.BorderColor, s.BorderColor, s.BorderColor, s.BorderColor}
	if s.Borders != (driver.Edges{}) {
		c := s.BorderColors
		colors = [4]driver.Color{c.Top, c.Right, c.Bottom, c.Left}
	}
	uniform := widths[0] == widths[1] && widths[1] == widths[2] && widths[2] == widths[3] &&
		colors[0] == colors[1] && colors[1] == colors[2] && colors[2] == colors[3]
	if bw := widths[0]; uniform && (radius <= 0 || radius >= bw) {
		// Stroking the middle line is exact while the inner corners stay
		// round, or square.
		half := bw / 2
		line := rectF{half, half, max(width-half, half), max(height-half, half)}
		if radius <= 0 {
			p.dc.callF(rtDrawRectangle, 0b100, uintptr(unsafe.Pointer(&line)), p.solid(colors[0]).ptr(), f32(bw), 0)
		} else {
			rr := roundedRect{rect: line, radiusX: radius - half, radiusY: radius - half}
			p.dc.callF(rtDrawRoundedRectangle, 0b100, uintptr(unsafe.Pointer(&rr)), p.solid(colors[0]).ptr(), f32(bw), 0)
		}
		return
	}
	// The ring between the border box and the padding box.
	outer := corners{{radius, radius}, {radius, radius}, {radius, radius}, {radius, radius}}
	figures := []roundedFigure{{rectF{0, 0, width, height}, outer}}
	if inner, c := paddingBox(width, height, radius, widths); !inner.empty() {
		figures = append(figures, roundedFigure{inner, c})
	}
	ring := p.path(figures)
	if ring == nil {
		return
	}
	defer ring.release()
	first, same := driver.Color(0), true
	for i := range 4 {
		if widths[i] > 0 {
			if first == 0 {
				first = colors[i]
			} else if colors[i] != first {
				same = false
			}
		}
	}
	if same {
		p.fillGeometry(ring, first)
		return
	}
	// Sides that differ: the ring cut to each side's part of the box.
	for i, quad := range sideQuads(width, height, widths) {
		if widths[i] <= 0 || !visible(colors[i]) {
			continue
		}
		clip := p.polygon(quad[:])
		if clip == nil {
			continue
		}
		p.pushLayer(1, clip)
		clip.release()
		p.fillGeometry(ring, colors[i])
		p.pop()
	}
}

// corners are the horizontal and vertical radii of a box's corners,
// clockwise from the top left.
type corners [4][2]float32

type roundedFigure struct {
	r rectF
	c corners
}

// paddingBox returns the box inside borders of widths (top, right,
// bottom, left) of a width by height box with corners of radius, and its
// corners: the outer ones less the borders on each axis.
func paddingBox(width, height, radius float32, widths [4]float32) (rectF, corners) {
	top, right, bottom, left := widths[0], widths[1], widths[2], widths[3]
	r := rectF{left, top, max(width-right, left), max(height-bottom, top)}
	c := func(x, y float32) [2]float32 { return [2]float32{max(radius-x, 0), max(radius-y, 0)} }
	return r, corners{c(left, top), c(right, top), c(right, bottom), c(left, bottom)}
}

// fitCorners scales c down as CSS does where adjacent corners overlap on
// a width by height box.
func fitCorners(width, height float32, c corners) corners {
	f := float32(1)
	for _, side := range [4][2]float32{
		{c[0][0] + c[1][0], width}, {c[1][1] + c[2][1], height},
		{c[2][0] + c[3][0], width}, {c[3][1] + c[0][1], height},
	} {
		if side[0] > 0 {
			f = min(f, side[1]/side[0])
		}
	}
	if f < 1 {
		for i := range c {
			c[i][0] *= f
			c[i][1] *= f
		}
	}
	return c
}

// sideQuads returns each side's part of a width by height box with border
// widths, clockwise from the top: corners split along the lines from the
// outer corners through the inner ones, as CSS joins differing borders.
func sideQuads(width, height float32, widths [4]float32) [4][4]pointF {
	t, r, b, l := widths[0], widths[1], widths[2], widths[3]
	end := func(cx, cy, dx, dy float32) pointF {
		if dx == 0 && dy == 0 {
			dx, dy = 1, 1
			if cx != 0 {
				dx = -1
			}
			if cy != 0 {
				dy = -1
			}
		}
		k := float32(math.Inf(1))
		if dx != 0 {
			k = width / 2 / float32(math.Abs(float64(dx)))
		}
		if dy != 0 {
			k = min(k, height/2/float32(math.Abs(float64(dy))))
		}
		return pointF{cx + dx*k, cy + dy*k}
	}
	tl := end(0, 0, l, t)
	tr := end(width, 0, -r, t)
	br := end(width, height, -r, -b)
	bl := end(0, height, l, -b)
	return [4][4]pointF{
		{{0, 0}, {width, 0}, tr, tl},
		{{width, 0}, {width, height}, br, tr},
		{{width, height}, {0, height}, bl, br},
		{{0, height}, {0, 0}, tl, bl},
	}
}

// pathGeometry opens a path geometry; fill returns it, closed, or nil.
func (p *painter) pathGeometry(mode uintptr, fill func(sink *comObject)) *comObject {
	var path, sink *comObject
	if failed(p.g.factory.call(d2dFactoryCreatePathGeometry, uintptr(unsafe.Pointer(&path)))) {
		return nil
	}
	if failed(path.call(pathOpen, uintptr(unsafe.Pointer(&sink)))) {
		path.release()
		return nil
	}
	sink.call(sinkSetFillMode, mode)
	fill(sink)
	hr := sink.call(sinkClose)
	sink.release()
	if failed(hr) {
		path.release()
		return nil
	}
	return path
}

// path returns rounded rectangles as one geometry filled alternately: one
// inside another makes a ring.
func (p *painter) path(figures []roundedFigure) *comObject {
	return p.pathGeometry(d2dFillAlternate, func(sink *comObject) {
		for _, f := range figures {
			addRounded(sink, f.r, f.c)
		}
	})
}

// polygon returns a closed polygon.
func (p *painter) polygon(pts []pointF) *comObject {
	return p.pathGeometry(d2dFillAlternate, func(sink *comObject) {
		sink.callPoint(sinkBeginFigure, pts[0].x, pts[0].y, d2dFigureFilled)
		sink.call(sinkAddLines, uintptr(unsafe.Pointer(&pts[1])), uintptr(len(pts)-1))
		sink.call(sinkEndFigure, d2dFigureClosed)
	})
}

// addRounded adds r with corners c to an open sink as a closed figure.
func addRounded(sink *comObject, r rectF, c corners) {
	x, y, w, h := r.left, r.top, r.right-r.left, r.bottom-r.top
	c = fitCorners(w, h, c)
	round := func(k [2]float32) bool { return k[0] > 0 && k[1] > 0 }
	for i := range c {
		if !round(c[i]) {
			c[i] = [2]float32{}
		}
	}
	line := func(px, py float32) {
		pt := pointF{px, py}
		sink.call(sinkAddLines, uintptr(unsafe.Pointer(&pt)), 1)
	}
	arc := func(px, py float32, k [2]float32) {
		if round(k) {
			a := arcSegment{point: pointF{px, py}, width: k[0], height: k[1], sweep: d2dSweepClockwise}
			sink.call(sinkAddArc, uintptr(unsafe.Pointer(&a)))
		}
	}
	sink.callPoint(sinkBeginFigure, x+c[0][0], y, d2dFigureFilled)
	line(x+w-c[1][0], y)
	arc(x+w, y+c[1][1], c[1])
	line(x+w, y+h-c[2][1])
	arc(x+w-c[2][0], y+h, c[2])
	line(x+c[3][0], y+h)
	arc(x, y+h-c[3][1], c[3])
	line(x, y+c[0][1])
	arc(x+c[0][0], y, c[0])
	sink.call(sinkEndFigure, d2dFigureClosed)
}
