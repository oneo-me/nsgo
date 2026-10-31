//go:build windows

package windows

import (
	"log"
	"math"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Liquid Glass (ViewStyle.Glass) draws as MyGo's glass does
// (plugins/glass), which follows macOS 27's NSGlassEffectView: what the
// target holds under the view, its background included, blurred by
// Direct2D and copied into a texture as large as the view, then a pixel
// shader over it bends it near the edges as through a lens, maps its
// lightness to the glass's, and lights the rim. The painter draws the
// result clipped to the view's shape, under its children.

// glassHLSL is MyGo's glass effect (glass.hlsl) over a texture of the
// blurred backdrop the size of the box, with its prelude: the rounded
// rectangle's distance and bilinear reads of the backdrop.
const glassHLSL = `
cbuffer Glass : register(b0) {
	float4 rect;  // the box, in the texture's pixels
	float4 radii; // its corners' radii: top left, top right, bottom right, bottom left
	float4 p0;    // bezel, refraction, rim, rim width
	float4 p1;    // tint, straight
	float4 p2;    // low, high, curve, saturation
	float4 p3;    // the light's direction
};
Texture2D backdrop : register(t0);

float sdRoundRect(float2 p, float4 rect, float4 radii) {
	float2 h = rect.zw * 0.5;
	float2 q = p - rect.xy - h;
	float r = q.x < 0 ? (q.y < 0 ? radii.x : radii.w) : (q.y < 0 ? radii.y : radii.z);
	float2 a = abs(q) - h + r;
	return length(max(a, 0)) + min(max(a.x, a.y), 0) - r;
}

float3 backdropAt(int2 p) {
	int2 size = int2(rect.zw);
	return backdrop.Load(int3(clamp(p, int2(0, 0), size - 1), 0)).rgb;
}

float3 sampleBackdrop(float2 q) {
	float2 u = q - 0.5;
	float2 f = floor(u);
	float2 w = u - f;
	int2 p = int2(f);
	float3 top = lerp(backdropAt(p), backdropAt(p + int2(1, 0)), w.x);
	float3 bottom = lerp(backdropAt(p + int2(0, 1)), backdropAt(p + int2(1, 1)), w.x);
	return lerp(top, bottom, w.y);
}

float glassLens(float t, float bezel) {
	if (bezel <= 0 || t >= bezel) {
		return 0;
	}
	float u = 1 - max(t, 0) / bezel;
	float v = 1 - u * u * u * u;
	float a = sqrt(v * sqrt(v));
	float b = u * u * u;
	float n = sqrt(a * a + b * b);
	float s = b / n;
	float c = a / n;
	float st = s / 1.5;
	float ct = sqrt(1 - st * st);
	return (s * ct - c * st) / (c * ct + s * st) / 1.1180340;
}

float2 glassNormal(float2 p) {
	float2 h = rect.zw * 0.5;
	float2 q = p - rect.xy - h;
	float r = q.x < 0 ? (q.y < 0 ? radii.x : radii.w) : (q.y < 0 ? radii.y : radii.z);
	r = min(r * 1.5, min(h.x, h.y));
	float2 a = abs(q) - (h - r);
	float2 sq = float2(sign(q));
	if (a.x > 0 || a.y > 0) {
		float2 m = max(a, 0);
		return sq * m / sqrt(m.x * m.x + m.y * m.y);
	}
	return a.x > a.y ? float2(sq.x, 0) : float2(0, sq.y);
}

float3 glassTone(float3 c) {
	float l = 0.2126 * c.r + 0.7152 * c.g + 0.0722 * c.b;
	float t = p2.x + (p2.y - p2.x) * (1 - pow(max(1 - l, 0), p2.z));
	float k = (p2.y - p2.x) * p2.w;
	return saturate(t + (c - l) * k);
}

float4 nsgoVertex(uint id : SV_VertexID) : SV_Position {
	float2 uv = float2((id << 1) & 2, id & 2);
	return float4(uv * float2(2.0, -2.0) + float2(-1.0, 1.0), 0.0, 1.0);
}

float4 nsgoPixel(float4 position : SV_Position) : SV_Target {
	float2 p = position.xy;
	float t = max(-sdRoundRect(p, rect, radii), 0);
	float bezel = p0.x;
	float rimWidth = p0.w;
	float2 n = float2(0, 0);
	if (t < bezel || t < 3 * rimWidth) {
		n = glassNormal(p);
	}
	float2 q = p;
	if (t < bezel) {
		q -= n * (p0.y * glassLens(t, bezel));
	}
	float3 c = glassTone(sampleBackdrop(q));
	c += (p1.rgb - c) * p1.a;
	if (p0.z > 0 && t < 3 * rimWidth) {
		float r = t / rimWidth;
		float a = p0.z * exp(-r * r);
		float l = abs(n.x * p3.x + n.y * p3.y);
		c += (l - c) * a;
	}
	return float4(c, 1);
}
`

// glassProgram is the glass's shaders for the device of a generation.
type glassProgram struct {
	vs, ps, cb *comObject
	generation uint64
	failed     bool
}

var glassShaders glassProgram

// program returns the glass's shaders for g's device, or nil.
func (gp *glassProgram) program(g *gfx) *glassProgram {
	if gp.generation == gfxGeneration && (gp.ps != nil || gp.failed) {
		if gp.failed {
			return nil
		}
		return gp
	}
	releaseAll(&gp.vs, &gp.ps, &gp.cb)
	gp.generation, gp.failed = gfxGeneration, true
	vs, ok := compileHLSL(glassHLSL, "nsgoVertex", "vs_5_0")
	if !ok {
		return nil
	}
	ps, ok := compileHLSL(glassHLSL, "nsgoPixel", "ps_5_0")
	if !ok {
		return nil
	}
	if failed(g.d3d.call(d3dCreateVertexShader, uintptr(unsafe.Pointer(&vs[0])), uintptr(len(vs)), 0,
		uintptr(unsafe.Pointer(&gp.vs)))) ||
		failed(g.d3d.call(d3dCreatePixelShader, uintptr(unsafe.Pointer(&ps[0])), uintptr(len(ps)), 0,
			uintptr(unsafe.Pointer(&gp.ps)))) {
		log.Printf("nsgo: glass: Direct3D cannot make the shaders")
		return nil
	}
	desc := struct{ size, usage, bind, cpu, misc, stride uint32 }{size: uint32(unsafe.Sizeof(glassParams{})), bind: d3dBindConstantBuffer}
	if failed(g.d3d.call(d3dCreateBuffer, uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&gp.cb)))) {
		return nil
	}
	gp.failed = false
	return gp
}

// glassParams is the shader's constant buffer.
type glassParams struct {
	rect, radii, p0, p1, p2, p3 [4]float32
}

// glassTexture is a texture a view's glass draws through: one Direct2D
// draws into or reads, and Direct3D reads or draws into, pw by ph pixels.
type glassTexture struct {
	tex, view, bitmap *comObject // view is its resource or render target view
	pw, ph            uint32
	generation        uint64
}

func (t *glassTexture) release() { releaseAll(&t.bitmap, &t.view, &t.tex) }

// make makes t pw by ph for g's device, Direct2D drawing into it when
// target is set and reading it otherwise.
func (t *glassTexture) make(p *painter, pw, ph uint32, target bool) bool {
	if t.tex != nil && t.pw == pw && t.ph == ph && t.generation == gfxGeneration {
		return true
	}
	t.release()
	g := p.g
	desc := texture2DDesc{width: pw, height: ph, mipLevels: 1, arraySize: 1, format: dxgiFormatBGRA,
		sampleCount: 1, bindFlags: d3dBindRenderTarget | d3dBindShaderResource}
	if failed(g.d3d.call(d3dCreateTexture2D, uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&t.tex)))) {
		return false
	}
	method := d3dCreateRenderTargetView
	if target {
		method = d3dCreateShaderResourceView
	}
	if failed(g.d3d.call(method, t.tex.ptr(), 0, uintptr(unsafe.Pointer(&t.view)))) {
		t.release()
		return false
	}
	surface := t.tex.query(&iidDXGISurfaceForShader)
	if surface == nil {
		t.release()
		return false
	}
	defer surface.release()
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaPremultiplied, dpiX: 96 * p.scale, dpiY: 96 * p.scale}
	if target {
		props.options = 1 // D2D1_BITMAP_OPTIONS_TARGET
	}
	if failed(p.dc.call(dcCreateBitmapFromDxgiSurface, surface.ptr(), uintptr(unsafe.Pointer(&props)),
		uintptr(unsafe.Pointer(&t.bitmap)))) {
		t.release()
		return false
	}
	t.pw, t.ph, t.generation = pw, ph, gfxGeneration
	return true
}

// glassView is a glass view's textures: the blurred backdrop and the
// glass drawn from it.
type glassView struct {
	backdrop, glass glassTexture
}

func (gv *glassView) release() {
	if gv != nil {
		gv.backdrop.release()
		gv.glass.release()
	}
}

// drawGlass draws v's glass over box b at world, which keeps rectangles
// upright: false when it cannot, for its blur to draw instead.
func (p *painter) drawGlass(v *view, s *driver.ViewStyle, b *box, world matrix) bool {
	gp := glassShaders.program(p.g)
	pw, ph := uint32(math.Ceil(float64(b.w*p.scale))), uint32(math.Ceil(float64(b.h*p.scale)))
	if gp == nil || pw == 0 || ph == 0 || world.b != 0 || world.c != 0 {
		return false
	}
	copied := p.backdrop()
	if copied == nil {
		return false
	}
	defer copied.release()
	scale := p.scale
	m := min(b.w, b.h)
	bezel := min(36, m/2)
	params := glassParams{
		rect: [4]float32{0, 0, float32(pw), float32(ph)},
		radii: [4]float32{float32(b.radii.TopLeft) * scale, float32(b.radii.TopRight) * scale,
			float32(b.radii.BottomRight) * scale, float32(b.radii.BottomLeft) * scale},
		p0: [4]float32{bezel * scale, 1.6 * bezel * scale, 0.35, 1},
		p3: [4]float32{0, 1},
	}
	// The material, as MyGo measured macOS 27's: larger panes blur more.
	blur := min(max(m*0.035, 1), 10)
	switch {
	case s.Glass == driver.GlassClear:
		blur = 1
		params.p2 = [4]float32{0.125, 1.082, 1, 1}
	case p.w.shownDark():
		params.p2 = [4]float32{0.15, 0.51, 2, 2.2}
	default:
		params.p2 = [4]float32{0.541, 1, 1.2, 1}
	}
	if v.glass == nil {
		v.glass = &glassView{}
	}
	gv := v.glass
	if !gv.backdrop.make(p, pw, ph, true) || !gv.glass.make(p, pw, ph, false) {
		return false
	}
	image, release := p.blurBackdrop(copied, blur)
	if image == nil {
		return false
	}
	defer release()
	// The blurred backdrop under the box, into the texture the shader
	// reads.
	p.flatten(func() {
		var target *comObject
		p.dc.call(dcGetTarget, uintptr(unsafe.Pointer(&target)))
		p.dc.call(dcSetTarget, gv.backdrop.bitmap.ptr())
		m := p.m
		p.setTransform(translation(-world.e, -world.f))
		var clear colorF
		p.dc.call(rtClear, uintptr(unsafe.Pointer(&clear)))
		p.dc.call(dcDrawImage, image.ptr(), 0, 0, d2dInterpolationLinear, 0)
		p.dc.call(dcSetTarget, target.ptr())
		target.release()
		p.setTransform(m)
		// What Direct2D drew so far comes before Direct3D reads it.
		p.dc.call(rtFlush, 0, 0)
	})
	var ctx *comObject
	p.g.d3d.call(d3dGetImmediateContext, uintptr(unsafe.Pointer(&ctx)))
	if ctx == nil {
		return false
	}
	defer ctx.release()
	rtv := gv.glass.view.ptr()
	ctx.call(ctxOMSetRenderTargets, 1, uintptr(unsafe.Pointer(&rtv)), 0)
	vp := d3dViewport{w: float32(pw), h: float32(ph), maxDepth: 1}
	ctx.call(ctxRSSetViewports, 1, uintptr(unsafe.Pointer(&vp)))
	ctx.call(ctxIASetInputLayout, 0)
	ctx.call(ctxIASetPrimitiveTopology, d3dTriangleList)
	ctx.call(ctxVSSetShader, gp.vs.ptr(), 0, 0)
	ctx.call(ctxPSSetShader, gp.ps.ptr(), 0, 0)
	ctx.call(ctxUpdateSubresource, gp.cb.ptr(), 0, 0, uintptr(unsafe.Pointer(&params)), 0, 0)
	cb := gp.cb.ptr()
	ctx.call(ctxPSSetConstantBuffers, 0, 1, uintptr(unsafe.Pointer(&cb)))
	srv := gv.backdrop.view.ptr()
	ctx.call(ctxPSSetShaderResources, 0, 1, uintptr(unsafe.Pointer(&srv)))
	ctx.call(ctxDraw, 3, 0)
	// Unbound, the textures are Direct2D's again.
	ctx.call(ctxOMSetRenderTargets, 0, 0, 0)
	var none uintptr
	ctx.call(ctxPSSetShaderResources, 0, 1, uintptr(unsafe.Pointer(&none)))
	p.setTransform(world)
	p.pushClipBox(b)
	dst := rectF{0, 0, b.w, b.h}
	p.dc.callF(rtDrawBitmap, 0b100, gv.glass.bitmap.ptr(), uintptr(unsafe.Pointer(&dst)), f32(1), d2dInterpolationLinear, 0)
	p.pop()
	return true
}
