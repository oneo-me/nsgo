//go:build windows

package windows

import (
	"math"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Effects, transitions and navigation shaders (driver.Shader.Role) draw
// their host's children through Direct3D in their place: the painter
// draws the children into a Direct2D bitmap over a texture of the
// device's own (the box and the overflow around it), which the program
// reads as its picture with no copy, and draws the program's texture
// where the children would show. The children still take input, as the
// window hit-tests the views. A transition's first picture is drawn
// offscreen as it starts, before its batch is applied. A view's own
// effect hosts on it, a screen shader's on the root view. While an
// effect shows the window draws whole frames, as its pictures reach past
// the areas a change damages. The backdrop is not pictured: it reads as
// transparent.

// effect is a view's effect, transition or navigation shader.
type effect struct {
	shader *driver.Shader
	sv     *shaderView // the program's output
	// pics are content, backdrop, from and to; before the picture a
	// transition's start drew, which the Shader that follows places.
	pics      [4]pictureTex
	before    pictureTex
	hasBefore bool
}

// pictureTex is a picture of views: a texture the program reads and
// Direct2D's target bitmap over it, pw by ph pixels.
type pictureTex struct {
	tex, srv, bitmap *comObject
	pw, ph           uint32
	generation       uint64
}

func (pt *pictureTex) release() {
	releaseAll(&pt.bitmap, &pt.srv, &pt.tex)
	pt.pw, pt.ph = 0, 0
}

// make sizes the picture's texture and bitmap for pw by ph pixels.
func (pt *pictureTex) make(g *gfx, scale float32, pw, ph uint32) bool {
	if pt.tex != nil && pt.pw == pw && pt.ph == ph && pt.generation == gfxGeneration {
		return true
	}
	pt.release()
	desc := texture2DDesc{width: pw, height: ph, mipLevels: 1, arraySize: 1, format: dxgiFormatBGRA,
		sampleCount: 1, bindFlags: d3dBindRenderTarget | d3dBindShaderResource}
	if failed(g.d3d.call(d3dCreateTexture2D, uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&pt.tex)))) {
		return false
	}
	if failed(g.d3d.call(d3dCreateShaderResourceView, pt.tex.ptr(), 0, uintptr(unsafe.Pointer(&pt.srv)))) {
		pt.release()
		return false
	}
	surface := pt.tex.query(&iidDXGISurfaceForShader)
	if surface == nil {
		pt.release()
		return false
	}
	defer surface.release()
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaPremultiplied, dpiX: 96 * scale,
		dpiY: 96 * scale, options: d2dBitmapTarget}
	if failed(g.dc.call(dcCreateBitmapFromDxgiSurface, surface.ptr(), uintptr(unsafe.Pointer(&props)),
		uintptr(unsafe.Pointer(&pt.bitmap)))) {
		pt.release()
		return false
	}
	pt.pw, pt.ph, pt.generation = pw, ph, gfxGeneration
	return true
}

// setEffect gives v its effect, transition or navigation shader, or takes
// it away for nil.
func (w *window) setEffect(v *view, sh *driver.Shader) {
	if sh == nil || sh.Program == nil {
		w.endEffect(v)
		return
	}
	fx := v.fx
	if fx == nil {
		fx = &effect{}
		v.fx = fx
	}
	if fx.sv == nil {
		fx.sv = &shaderView{id: v.id}
	}
	was := fx.shader
	fx.shader = sh
	fx.sv.shader, fx.sv.dirty = sh, true
	w.animator.BindShader(v.id, sh.Nodes)
	if sh.Role.Transitions() {
		switch {
		case sh.Running && fx.hasBefore:
			side := 2
			if sh.Backwards {
				side = 3
			}
			fx.pics[side], fx.before = fx.before, fx.pics[side]
			fx.hasBefore = false
		case !sh.Running && was != nil && was.Running:
			fx.pics[2].release()
			fx.pics[3].release()
		}
	}
	if w.effects == nil {
		w.effects = map[*view]bool{}
	}
	w.effects[v] = true
	w.redrawAll()
}

// effectHost is the view v's shader draws over: v, or the root view for
// a screen shader.
func (w *window) effectHost(v *view) *view {
	if v.fx != nil && v.fx.shader != nil && v.fx.shader.Role.Covers() {
		return w.view(w.root)
	}
	return v
}

// effectOn returns the effect that draws over host's children, if one
// shows: its own, or a screen shader's on the root.
func (w *window) effectOn(host *view) *view {
	for v := range w.effects {
		if w.effectHost(v) == host && v.fx.shows() {
			return v
		}
	}
	return nil
}

func (fx *effect) shows() bool {
	return fx.shader != nil && (!fx.shader.Role.Transitions() || fx.shader.Running)
}

// startTransition draws what v's transition starts from offscreen,
// before the batch that starts it is applied.
func (w *window) startTransition(v *view) {
	if v.fx == nil {
		v.fx = &effect{}
	}
	fx := v.fx
	host := w.effectHost(v)
	if host == nil {
		return
	}
	g, err := devices()
	if err != nil {
		return
	}
	ov := float32(0)
	if fx.shader != nil {
		ov = float32(fx.shader.Overflow)
	}
	dc := g.dc
	scale := float32(w.scale)
	size := frameOf(host).Size
	pw := uint32(math.Ceil(float64((float32(size.Width) + 2*ov) * scale)))
	ph := uint32(math.Ceil(float64((float32(size.Height) + 2*ov) * scale)))
	if pw == 0 || ph == 0 || !fx.before.make(g, scale, pw, ph) {
		return
	}
	dc.call(dcSetTarget, fx.before.bitmap.ptr())
	dc.callF(rtSetDpi, 0b11, f32(96*scale), f32(96*scale))
	dc.call(rtBeginDraw)
	p := newPainter(w, g)
	clear := colorF{}
	p.setTransform(identity)
	dc.call(rtClear, uintptr(unsafe.Pointer(&clear)))
	if running := w.effectOn(host); running != nil && running.fx.sv.bitmap != nil {
		// One that runs ends here: its picture is the frame it drew last.
		dst := rectF{0, 0, float32(size.Width) + 2*ov, float32(size.Height) + 2*ov}
		dc.callF(rtDrawBitmap, 0b100, running.fx.sv.bitmap.ptr(), uintptr(unsafe.Pointer(&dst)), f32(1), d2dInterpolationLinear, 0)
	} else {
		p.drawChildren(host, translation(float64(ov), float64(ov)))
	}
	p.release()
	hr := dc.call(rtEndDraw, 0, 0)
	dc.call(dcSetTarget, 0)
	fx.hasBefore = hr == 0
}

// drawChildren draws v's children at world, as drawContent does.
func (p *painter) drawChildren(v *view, world matrix) {
	for _, c := range v.children {
		f := frameOf(c)
		p.draw(c, world, f.Origin.X-v.offset.X, f.Origin.Y-v.offset.Y)
	}
}

// drawEffect draws host's children through the effect of view e, at
// world: the live picture drawn offscreen, the program run over it, and
// its output in place of the children. It reports false where it cannot,
// and the children are drawn as they are.
func (p *painter) drawEffect(e, host *view, world matrix) bool {
	fx := e.fx
	sh := fx.shader
	g := p.g
	prog := g.program(sh.Program)
	if prog == nil {
		return false
	}
	ov := float32(sh.Overflow)
	size := frameOf(host).Size
	w, h := float32(size.Width)+2*ov, float32(size.Height)+2*ov
	pw, ph := uint32(math.Ceil(float64(w*p.scale))), uint32(math.Ceil(float64(h*p.scale)))
	if pw == 0 || ph == 0 {
		return false
	}
	live := -1
	switch {
	case sh.Role.Transitions():
		live = 3
		if sh.Backwards {
			live = 2
		}
		if sh.Pictures&(1<<live) == 0 {
			live = -1
		}
	case sh.Pictures&driver.PictureContent != 0:
		live = 0
	}
	if live >= 0 {
		pic := &fx.pics[live]
		if !pic.make(g, p.scale, pw, ph) {
			return false
		}
		p.flatten(func() {
			var target *comObject
			p.dc.call(dcGetTarget, uintptr(unsafe.Pointer(&target)))
			p.dc.call(dcSetTarget, pic.bitmap.ptr())
			pushed, split, m := p.pushed, p.split, p.m
			p.pushed, p.split = nil, nil
			p.setTransform(identity)
			clear := colorF{}
			p.dc.call(rtClear, uintptr(unsafe.Pointer(&clear)))
			p.drawChildren(host, translation(float64(ov), float64(ov)))
			for len(p.pushed) > 0 {
				p.pop()
			}
			p.pushed, p.split = pushed, split
			p.dc.call(dcSetTarget, target.ptr())
			target.release()
			p.m = m
		})
	}
	sv := fx.sv
	if sv.target == nil || sv.pw != pw || sv.ph != ph || sv.generation != gfxGeneration {
		sv.release()
		if !sv.makeTarget(p, pw, ph) {
			return false
		}
	}
	sv.pictures = [4]*pictureTex{&fx.pics[0], &fx.pics[1], &fx.pics[2], &fx.pics[3]}
	p.dc.call(rtFlush, 0, 0)
	sv.render(p, prog, w, h)
	sv.dirty = false
	p.setTransform(world)
	dst := rectF{-ov, -ov, w - ov, h - ov}
	p.dc.callF(rtDrawBitmap, 0b100, sv.bitmap.ptr(), uintptr(unsafe.Pointer(&dst)), f32(1), d2dInterpolationLinear, 0)
	return true
}

// navigating reports whether a navigation shader runs over host.
func (w *window) navigating(host *view) bool {
	return host != nil && host.fx != nil && host.fx.shader != nil && host.fx.shader.Role == driver.ShaderNav &&
		host.fx.shader.Running
}

func (w *window) endEffect(v *view) {
	fx := v.fx
	if fx == nil {
		return
	}
	v.fx = nil
	delete(w.effects, v)
	fx.sv.release()
	for i := range fx.pics {
		fx.pics[i].release()
	}
	fx.before.release()
	w.redrawAll()
}
