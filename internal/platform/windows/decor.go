//go:build windows

package windows

import (
	"math"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Direct2D draws what a style needs past a rounded rectangle from path
// geometries: corners of their own radii, continuous corners, dashed
// borders and outlines. Box shadows are their shapes recorded and blurred,
// as React Native's shadow is, masked to outside the box or inside its
// padding box; gradients are gradient brushes, and conic ones bitmaps
// drawn pixel by pixel; masks are layers with an opacity brush. A view
// with filters or turned out of its plane is recorded into a command list
// and drawn through Direct2D's effects: a color matrix, a blur, a shadow
// and a 3D transform. Nothing reads back what is behind a view, so
// backdrop filters and blend modes draw nothing here.

const (
	rtCreateGradientStopCollection = 9
	rtCreateLinearGradientBrush    = 10
	rtCreateRadialGradientBrush    = 11
	dcGetTarget                    = 75

	d2dPropertyMatrix4x4 = 16
	d2dPropertyMatrix5x4 = 17
	d2dGamma22           = 0
	d2dExtendClamp       = 0
	d2dDashCustom        = 5
)

var (
	clsidColorMatrix = parseGUID("921f03d6-641c-47df-852d-b4bb6153ae11")
	clsid3DTransform = parseGUID("e8467b04-ec61-4b8a-b5de-d4d73debea5a")
)

// gradientStop is D2D1_GRADIENT_STOP.
type gradientStop struct {
	position float32
	color    colorF
}

// box is the shape of a view: its size, fitted corners and whether they
// ease into the edges.
type box struct {
	w, h       float32
	radii      driver.Corners
	continuous bool
}

func newBox(s *driver.ViewStyle, w, h float32) box {
	return box{w: w, h: h, radii: s.Radii(driver.Size{Width: driver.Points(w), Height: driver.Points(h)}),
		continuous: s.ContinuousCorners && s.Rounded()}
}

// simple reports whether one radius rounds every corner, which Direct2D
// draws as a rounded rectangle.
func (b *box) simple() (float32, bool) {
	r := b.radii
	return float32(r.TopLeft), !b.continuous && r.Uniform()
}

// addSegments adds an outline's figures to an open sink.
func addSegments(sink *comObject, segs []driver.Segment) {
	pt := func(p [2]float64) pointF { return pointF{float32(p[0]), float32(p[1])} }
	for _, s := range segs {
		switch s.Kind {
		case driver.SegmentMove:
			sink.callPoint(sinkBeginFigure, float32(s.P[0][0]), float32(s.P[0][1]), d2dFigureFilled)
		case driver.SegmentLine:
			p := pt(s.P[0])
			sink.call(sinkAddLines, uintptr(unsafe.Pointer(&p)), 1)
		case driver.SegmentCubic:
			b := bezierSegment{pt(s.P[0]), pt(s.P[1]), pt(s.P[2])}
			sink.call(sinkAddBeziers, uintptr(unsafe.Pointer(&b)), 1)
		case driver.SegmentClose:
			sink.call(sinkEndFigure, d2dFigureClosed)
		}
	}
}

// geometry returns the box inset by d (outset when negative) as a
// geometry the caller releases, nil when Direct2D cannot make it.
func (p *painter) geometry(b *box, d float32) *comObject {
	dd := float64(d)
	segs := driver.RoundedRect(nil, dd, dd, float64(b.w)-2*dd, float64(b.h)-2*dd, b.radii.Inset(driver.Points(d)), b.continuous, false)
	return p.pathGeometry(d2dFillWinding, func(sink *comObject) { addSegments(sink, segs) })
}

// fillBox fills the box with c, or with brush when it is set.
func (p *painter) fillBox(b *box, c driver.Color, brush *comObject) {
	if b.w <= 0 || b.h <= 0 || brush == nil && !visible(c) {
		return
	}
	if brush == nil {
		brush = p.solid(c)
	}
	if r, ok := b.simple(); ok {
		r = max(min(r, b.w/2, b.h/2), 0)
		rr := roundedRect{rect: rectF{0, 0, b.w, b.h}, radiusX: r, radiusY: r}
		p.dc.call(rtFillRoundedRectangle, uintptr(unsafe.Pointer(&rr)), brush.ptr())
		return
	}
	if g := p.geometry(b, 0); g != nil {
		p.dc.call(rtFillGeometry, g.ptr(), brush.ptr(), 0)
		g.release()
	}
}

// fillRim fills the box inset by d with c, under its border.
func (p *painter) fillRim(b *box, c driver.Color, d driver.Points) {
	if g := p.geometry(b, float32(d)); g != nil {
		p.dc.call(rtFillGeometry, g.ptr(), p.solid(c).ptr(), 0)
		g.release()
	}
}

// pushClipBox clips what follows to the box.
func (p *painter) pushClipBox(b *box) {
	if r, ok := b.simple(); ok {
		p.pushClipRounded(rectF{0, 0, b.w, b.h}, r)
		return
	}
	g := p.geometry(b, 0)
	p.pushLayer(1, g)
	g.release()
}

// strokeBox draws a line of width in c along the box inset by inset,
// dashed as style says.
func (p *painter) strokeBox(b *box, inset, width float32, c driver.Color, style driver.BorderStyle) {
	if width <= 0 || !visible(c) {
		return
	}
	g := p.geometry(b, inset)
	if g == nil {
		return
	}
	p.dc.callF(rtDrawGeometry, 0b100, g.ptr(), p.solid(c).ptr(), f32(width), p.dashes(style))
	g.release()
}

// dashStyles are the stroke styles of dashed and dotted lines, made on
// the devices of generation dashed.
var (
	dashStyles [2]*comObject
	dashed     uint64
)

// dashes returns the stroke style of a border style, 0 for solid lines.
// Direct2D's dashes are in widths of the line.
func (p *painter) dashes(style driver.BorderStyle) uintptr {
	i := 0
	switch style {
	case driver.BorderDashed:
	case driver.BorderDotted:
		i = 1
	default:
		return 0
	}
	if dashed != gfxGeneration {
		releaseAll(&dashStyles[0], &dashStyles[1])
		dashed = gfxGeneration
	}
	if dashStyles[i] == nil {
		on, off, round, _ := style.Dashes(1)
		props := strokeStyleProperties{dashStyle: d2dDashCustom, miterLimit: 10}
		if round {
			props.dashCap = d2dCapRound
		}
		d := []float32{float32(on), float32(off)}
		p.g.factory.call(d2dFactoryCreateStrokeStyle, uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&d[0])), 2,
			uintptr(unsafe.Pointer(&dashStyles[i])))
	}
	return dashStyles[i].ptr()
}

// gradientBrush returns a brush painting gradient g over a w by h box,
// nil for a conic one or when Direct2D cannot make it; the caller
// releases it.
func (p *painter) gradientBrush(g *driver.GradientPaint, w, h float32) *comObject {
	if len(g.Stops) < 2 || g.Kind == driver.GradientConic {
		return nil
	}
	stops := make([]gradientStop, len(g.Stops))
	for i, s := range g.Stops {
		stops[i] = gradientStop{s.Offset, colorOf(s.Color)}
	}
	var collection, brush *comObject
	if failed(p.dc.call(rtCreateGradientStopCollection, uintptr(unsafe.Pointer(&stops[0])), uintptr(len(stops)),
		d2dGamma22, d2dExtendClamp, uintptr(unsafe.Pointer(&collection)))) {
		return nil
	}
	defer collection.release()
	if g.Kind == driver.GradientRadial {
		props := struct {
			center, offset pointF
			rx, ry         float32
		}{center: pointF{g.CX * w, g.CY * h}, rx: max(g.RX*w, 1e-3), ry: max(g.RY*h, 1e-3)}
		if failed(p.dc.call(rtCreateRadialGradientBrush, uintptr(unsafe.Pointer(&props)), 0, collection.ptr(),
			uintptr(unsafe.Pointer(&brush)))) {
			return nil
		}
		return brush
	}
	props := struct{ start, end pointF }{pointF{g.X0 * w, g.Y0 * h}, pointF{g.X1 * w, g.Y1 * h}}
	if failed(p.dc.call(rtCreateLinearGradientBrush, uintptr(unsafe.Pointer(&props)), 0, collection.ptr(),
		uintptr(unsafe.Pointer(&brush)))) {
		return nil
	}
	return brush
}

// drawGradients draws the gradients over the background, the first on
// top, in the box.
func (p *painter) drawGradients(b *box, paints []driver.GradientPaint) {
	for i := len(paints) - 1; i >= 0; i-- {
		g := &paints[i]
		if g.Kind == driver.GradientConic {
			p.drawConic(b, g)
			continue
		}
		if brush := p.gradientBrush(g, b.w, b.h); brush != nil {
			p.fillBox(b, 0, brush)
			brush.release()
		}
	}
}

// drawConic draws a conic gradient, which Direct2D has no brush for, as a
// bitmap of its pixels, in the box.
func (p *painter) drawConic(b *box, g *driver.GradientPaint) {
	pw, ph := int(math.Ceil(float64(b.w*p.scale))), int(math.Ceil(float64(b.h*p.scale)))
	if pw <= 0 || ph <= 0 || pw*ph > 4<<20 {
		return
	}
	pix := make([]byte, pw*ph*4)
	cx, cy := float64(g.CX)*float64(pw), float64(g.CY)*float64(ph)
	start := float64(g.Angle) * math.Pi / 180
	for y := range ph {
		for x := range pw {
			// Clockwise from up, as CSS's angles.
			a := math.Atan2(float64(x)+0.5-cx, -(float64(y) + 0.5 - cy))
			t := math.Mod(a-start+4*math.Pi, 2*math.Pi) / (2 * math.Pi)
			r, gg, bl, al := g.ColorAt(float32(t)).Components()
			i := (y*pw + x) * 4
			// Premultiplied BGRA.
			pix[i], pix[i+1], pix[i+2], pix[i+3] = byte(bl*al*255+0.5), byte(gg*al*255+0.5), byte(r*al*255+0.5), byte(al*255+0.5)
		}
	}
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaPremultiplied, dpiX: 96 * p.scale, dpiY: 96 * p.scale}
	var bmp *comObject
	if failed(p.dc.callSize(dcCreateBitmap, uint32(pw), uint32(ph), uintptr(unsafe.Pointer(&pix[0])), uintptr(pw*4),
		uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&bmp)))) {
		return
	}
	defer bmp.release()
	p.pushClipBox(b)
	dst := rectF{0, 0, b.w, b.h}
	p.dc.callF(rtDrawBitmap, 0b100, bmp.ptr(), uintptr(unsafe.Pointer(&dst)), f32(1), d2dInterpolationLinear, 0)
	p.pop()
}

// boxShadowKey is what one box shadow's drawing depends on.
type boxShadowKey struct {
	w, h, sigma, spread, scale float32
	radii                      driver.Corners
	continuous, inset          bool
	borders                    driver.Edges
	color                      driver.Color
}

// boxShadow is a box shadow's recording, blurred.
type boxShadow struct {
	key        boxShadowKey
	generation uint64
	list       *comObject
	blur       *comObject
	image      *comObject
}

func (sh *boxShadow) release() {
	if sh != nil {
		releaseAll(&sh.image, &sh.blur, &sh.list)
	}
}

// drawBoxShadows draws the box shadows of one kind, the first on top:
// outer ones under the box and outside it, inset ones over the background
// inside the padding box.
func (p *painter) drawBoxShadows(v *view, s *driver.ViewStyle, b *box, shadows []driver.BoxShadow, inset bool) {
	if b.w <= 0 || b.h <= 0 {
		return
	}
	widths := s.Borders
	if widths == (driver.Edges{}) {
		widths = driver.All(s.BorderWidth)
	}
	for i := len(shadows) - 1; i >= 0; i-- {
		sh := shadows[i]
		if sh.Inset != inset || !visible(sh.Color) {
			continue
		}
		key := boxShadowKey{w: b.w, h: b.h, sigma: float32(sh.Blur) / 2, spread: float32(sh.Spread), scale: p.scale,
			radii: b.radii, continuous: b.continuous, inset: inset, color: sh.Color}
		if inset {
			key.borders = widths
		}
		cached := v.boxShadows[key]
		if cached == nil || cached.generation != gfxGeneration {
			cached.release()
			cached = p.makeBoxShadow(key, float32(max(math.Abs(float64(sh.Offset.X)), math.Abs(float64(sh.Offset.Y)))))
			if cached == nil {
				continue
			}
			if v.boxShadows == nil {
				v.boxShadows = map[boxShadowKey]*boxShadow{}
			}
			v.boxShadows[key] = cached
		}
		if v.shadowsUsed == nil {
			v.shadowsUsed = map[boxShadowKey]bool{}
		}
		v.shadowsUsed[key] = true
		// The mask: outside the box, or the padding box.
		var mask *comObject
		if inset {
			inner := driver.InnerRadii(b.radii, widths)
			x, y := float64(widths.Left), float64(widths.Top)
			segs := driver.EllipticRect(nil, x, y, float64(b.w)-x-float64(widths.Right), float64(b.h)-y-float64(widths.Bottom),
				inner, b.continuous, false)
			mask = p.pathGeometry(d2dFillWinding, func(sink *comObject) { addSegments(sink, segs) })
		} else {
			far := float64(key.sigma*3+key.spread) + math.Abs(float64(sh.Offset.X)) + math.Abs(float64(sh.Offset.Y)) + 1
			segs := driver.RoundedRect(nil, -far, -far, float64(b.w)+2*far, float64(b.h)+2*far, driver.Corners{}, false, false)
			segs = driver.RoundedRect(segs, 0, 0, float64(b.w), float64(b.h), b.radii, b.continuous, false)
			mask = p.pathGeometry(d2dFillAlternate, func(sink *comObject) { addSegments(sink, segs) })
		}
		p.pushLayer(1, mask)
		mask.release()
		at := pointF{float32(sh.Offset.X), float32(sh.Offset.Y)}
		p.dc.call(dcDrawImage, cached.image.ptr(), uintptr(unsafe.Pointer(&at)), 0, d2dInterpolationLinear, 0)
		p.pop()
	}
}

// makeBoxShadow records key's shape, in its color, on the second context
// and blurs it. reach is how far the shadow moves, which an inset one's
// ring must cover.
func (p *painter) makeBoxShadow(key boxShadowKey, reach float32) *boxShadow {
	sh := &boxShadow{key: key, generation: gfxGeneration}
	var segs []driver.Segment
	s := float64(key.spread)
	w, h := float64(key.w), float64(key.h)
	box := box{w: key.w, h: key.h, radii: key.radii, continuous: key.continuous}
	fill := d2dFillWinding
	if key.inset {
		// A ring around the padding box shrunk by the spread casts the
		// shadow inwards.
		b := key.borders
		x, y := float64(b.Left), float64(b.Top)
		iw, ih := w-x-float64(b.Right), h-y-float64(b.Bottom)
		far := float64(key.sigma*3+reach) + s + 1
		segs = driver.RoundedRect(nil, x-far, y-far, iw+2*far, ih+2*far, driver.Corners{}, false, false)
		inner := driver.InnerRadii(box.radii, b)
		for i := range inner {
			for j := range inner[i] {
				if inner[i][j] > 0 {
					inner[i][j] = max(inner[i][j]-s, 0)
				}
			}
		}
		segs = driver.EllipticRect(segs, x+s, y+s, max(iw-2*s, 0), max(ih-2*s, 0), inner, box.continuous, false)
		fill = d2dFillAlternate
	} else {
		segs = driver.RoundedRect(nil, -s, -s, max(w+2*s, 0), max(h+2*s, 0), box.radii.Inset(-driver.Points(key.spread)),
			box.continuous, false)
	}
	geometry := p.pathGeometry(uintptr(fill), func(sink *comObject) { addSegments(sink, segs) })
	if geometry == nil {
		return nil
	}
	defer geometry.release()
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
		bake.call(rtFillGeometry, geometry.ptr(), brush.ptr(), 0)
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
	sigma := key.sigma
	sh.blur.call(effectSetValue, 0, d2dPropertyFloat, uintptr(unsafe.Pointer(&sigma)), 4)
	sh.blur.call(effectSetInput, 0, sh.list.ptr(), 1)
	sh.blur.call(effectGetOutput, uintptr(unsafe.Pointer(&sh.image)))
	if sh.image == nil {
		sh.release()
		return nil
	}
	return sh
}

// pruneShadows releases the box shadows v drew in an earlier pass but
// not in this one.
func (v *view) pruneShadows() {
	for k, sh := range v.boxShadows {
		if !v.shadowsUsed[k] {
			sh.release()
			delete(v.boxShadows, k)
		}
	}
	clear(v.shadowsUsed)
}

// pushMask makes what follows show as much as gradient g's alpha says,
// over a w by h box.
func (p *painter) pushMask(g *driver.GradientPaint, w, h float32) bool {
	brush := p.gradientBrush(g, w, h)
	if brush == nil {
		return false
	}
	p.pushLayerBrush(1, brush)
	brush.release()
	return true
}

// drawFiltered draws what record draws, recorded into a command list in
// the view's own coordinates, through filters and then local, at world.
func (p *painter) drawFiltered(world matrix, local driver.Matrix3D, filters []driver.Filter, record func()) {
	var list *comObject
	if failed(p.dc.call(dcCreateCommandList, uintptr(unsafe.Pointer(&list)))) {
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
	var effects []*comObject
	defer func() {
		for _, e := range effects {
			e.release()
		}
	}()
	image := list
	chain := func(clsid *guid, set func(e *comObject)) {
		var e *comObject
		if failed(p.dc.call(dcCreateEffect, uintptr(unsafe.Pointer(clsid)), uintptr(unsafe.Pointer(&e)))) {
			return
		}
		effects = append(effects, e)
		set(e)
		e.call(effectSetInput, 0, image.ptr(), 1)
		var out *comObject
		e.call(effectGetOutput, uintptr(unsafe.Pointer(&out)))
		if out != nil {
			effects = append(effects, out)
			image = out
		}
	}
	m, color, blur, shadows := driver.SplitFilters(filters)
	if color {
		// Rows of what each input gives the outputs, then the offsets.
		var rows [20]float32
		for in := range 5 {
			for out := range 4 {
				rows[in*4+out] = m[out*5+in]
			}
		}
		chain(&clsidColorMatrix, func(e *comObject) {
			e.call(effectSetValue, 0, d2dPropertyMatrix5x4, uintptr(unsafe.Pointer(&rows[0])), 80)
		})
	}
	if blur > 0 {
		sigma := float32(blur)
		chain(&clsidGaussianBlur, func(e *comObject) {
			e.call(effectSetValue, 0, d2dPropertyFloat, uintptr(unsafe.Pointer(&sigma)), 4)
		})
	}
	source := image
	var shadowImages []*comObject
	var offsets []pointF
	for _, sh := range shadows {
		image = source
		chain(&clsidShadow, func(e *comObject) {
			sigma := float32(sh.Blur) / 2
			e.call(effectSetValue, 0, d2dPropertyFloat, uintptr(unsafe.Pointer(&sigma)), 4)
			col := colorOf(sh.Color)
			e.call(effectSetValue, 1, d2dPropertyVector4, uintptr(unsafe.Pointer(&col)), 16)
		})
		if image != source {
			shadowImages = append(shadowImages, image)
			offsets = append(offsets, pointF{float32(sh.Offset.X), float32(sh.Offset.Y)})
		}
	}
	draw := func(img *comObject, at pointF) {
		if _, _, _, _, _, _, flat := local.Affine(); !flat {
			chain3D := img
			var e, out *comObject
			if !failed(p.dc.call(dcCreateEffect, uintptr(unsafe.Pointer(&clsid3DTransform)), uintptr(unsafe.Pointer(&e)))) {
				effects = append(effects, e)
				var mm [16]float32
				for i := range mm {
					mm[i] = float32(local[i])
				}
				// The effect works in pixels: the points' matrix between
				// scalings.
				s := float64(p.scale)
				px := driver.Matrix3D{s, 0, 0, 0, 0, s, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
				inv := driver.Matrix3D{1 / s, 0, 0, 0, 0, 1 / s, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
				full := px.Mul(local).Mul(inv)
				for i := range mm {
					mm[i] = float32(full[i])
				}
				e.call(effectSetValue, 2, d2dPropertyMatrix4x4, uintptr(unsafe.Pointer(&mm[0])), 64)
				e.call(effectSetInput, 0, chain3D.ptr(), 1)
				e.call(effectGetOutput, uintptr(unsafe.Pointer(&out)))
				if out != nil {
					effects = append(effects, out)
					img = out
				}
			}
		}
		p.dc.call(dcDrawImage, img.ptr(), uintptr(unsafe.Pointer(&at)), 0, d2dInterpolationLinear, 0)
	}
	p.setTransform(world)
	for i, img := range shadowImages {
		draw(img, offsets[i])
	}
	draw(source, pointF{})
}
