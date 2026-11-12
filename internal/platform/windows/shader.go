//go:build windows

package windows

import (
	"log"
	"math"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/shaderir"
	"github.com/nsgo-dev/nsgo/internal/shaderir/hlsl"
)

// A shader view draws with Direct3D 11 on the device Direct2D draws
// with: its program becomes HLSL, which D3DCompile compiles once per
// program, and a triangle covering a texture of the view's own runs it.
// The painter draws the texture as a bitmap, after drawing it again when
// the uniforms, the size, an image or the animated lanes changed, never
// on a timer of its own.

var (
	d3dcompiler = syscall.NewLazyDLL("d3dcompiler_47.dll")
	d3dCompile  = d3dcompiler.NewProc("D3DCompile")

	iidDXGISurfaceForShader = iidDXGISurface
)

// Vtable indexes of ID3D11Device, ID3D11DeviceContext and ID3DBlob.
const (
	d3dCreateBuffer             = 3
	d3dCreateShaderResourceView = 7
	d3dCreateRenderTargetView   = 9
	d3dCreateVertexShader       = 12
	d3dCreatePixelShader        = 15
	d3dGetImmediateContext      = 40

	ctxPSSetShaderResources   = 8
	ctxPSSetShader            = 9
	ctxVSSetShader            = 11
	ctxDraw                   = 13
	ctxPSSetConstantBuffers   = 16
	ctxIASetInputLayout       = 17
	ctxIASetPrimitiveTopology = 24
	ctxOMSetRenderTargets     = 33
	ctxRSSetViewports         = 44
	ctxUpdateSubresource      = 48
	ctxClearRenderTargetView  = 50

	blobGetBufferPointer = 3
	blobGetBufferSize    = 4

	d3dBindConstantBuffer = 4
	d3dBindShaderResource = 8
	d3dBindRenderTarget   = 32
	d3dTriangleList       = 4
)

// d3dViewport is D3D11_VIEWPORT.
type d3dViewport struct{ x, y, w, h, minDepth, maxDepth float32 }

// shaderStage is the program's fragment function as the pixel stage,
// and a triangle covering the target.
const shaderStage = `
float4 nsgoVertex(uint id : SV_VertexID) : SV_Position {
	float2 uv = float2((id << 1) & 2, id & 2);
	return float4(uv * float2(2.0, -2.0) + float2(-1.0, 1.0), 0.0, 1.0);
}

float4 nsgoPixel(float4 position : SV_Position) : SV_Target {
	return nsgoShader(position, position.xy / U1);
}
`

// d3dProgram is a program compiled for the device of a generation.
type d3dProgram struct {
	vs, ps, cb *comObject
	offsets    []int
	types      []shaderir.BasicType
	size       int
	textures   int
	generation uint64
	failed     bool
}

// d3dPrograms are the programs compiled so far, by the program, which
// apps keep for their lives.
var d3dPrograms = map[*driver.ShaderProgram]*d3dProgram{}

// shaderView is a shader view's Direct3D state.
type shaderView struct {
	id     driver.NodeID
	shader *driver.Shader
	// target is the texture it draws into, rtv its render target view
	// and bitmap Direct2D's view of it, pw by ph pixels, made in
	// generation.
	target, rtv, bitmap *comObject
	pw, ph              uint32
	generation          uint64
	dirty               bool
	lanes               []float64
	buf                 []byte
	// images are the images' textures, by ImageSource.Key; pictures what
	// a WGSL program reads instead, an effect's (effect.go).
	images   map[string]*d3dImage
	pictures [4]*pictureTex
}

// d3dImage is an image's texture and its view, w by h pixels.
type d3dImage struct {
	tex, srv   *comObject
	w, h       float64
	generation uint64
}

func (sv *shaderView) release() {
	if sv == nil {
		return
	}
	releaseAll(&sv.bitmap, &sv.rtv, &sv.target)
	for _, im := range sv.images {
		releaseAll(&im.srv, &im.tex)
	}
	sv.images = nil
}

// setShader gives v what it draws and binds its animated lanes.
func (w *window) setShader(v *view, sh *driver.Shader) {
	if v.shaderView == nil {
		v.shaderView = &shaderView{id: v.id}
	}
	v.shaderView.shader, v.shaderView.dirty = sh, true
	w.animator.BindShader(v.id, sh.Nodes)
	w.redrawView(v)
}

// changedShaders marks the shader views whose animated lanes changed.
func (w *window) changedShaders() {
	w.animator.ChangedShaders(func(id driver.NodeID) {
		if v := w.view(id); v != nil && v.fx != nil && v.fx.sv != nil {
			v.fx.sv.dirty = true
			w.redrawAll()
		} else if v != nil && v.shaderView != nil {
			v.shaderView.dirty = true
			w.redrawView(v)
		}
	})
}

// program returns prog compiled for the device of g, or nil.
func (g *gfx) program(prog *driver.ShaderProgram) *d3dProgram {
	p := d3dPrograms[prog]
	if p != nil && p.generation == gfxGeneration {
		if p.failed {
			return nil
		}
		return p
	}
	if p != nil {
		releaseAll(&p.vs, &p.ps, &p.cb)
	}
	p = &d3dProgram{generation: gfxGeneration, failed: true}
	d3dPrograms[prog] = p
	ir := prog.IR
	src := hlsl.Fragment(ir, "nsgoShader", 0, 0) + shaderStage
	vs, ok := compileHLSL(src, "nsgoVertex", "vs_5_0")
	if !ok {
		return nil
	}
	ps, ok := compileHLSL(src, "nsgoPixel", "ps_5_0")
	if !ok {
		return nil
	}
	if failed(g.d3d.call(d3dCreateVertexShader, uintptr(unsafe.Pointer(&vs[0])), uintptr(len(vs)), 0,
		uintptr(unsafe.Pointer(&p.vs)))) ||
		failed(g.d3d.call(d3dCreatePixelShader, uintptr(unsafe.Pointer(&ps[0])), uintptr(len(ps)), 0,
			uintptr(unsafe.Pointer(&p.ps)))) {
		log.Printf("nsgo: shader: Direct3D cannot make the shaders")
		return nil
	}
	p.offsets, p.size = hlsl.UniformLanes(ir)
	p.types = ir.LaneTypes()
	p.textures = ir.TextureCount
	p.size = (max(p.size, 16) + 15) &^ 15
	desc := struct{ size, usage, bind, cpu, misc, stride uint32 }{size: uint32(p.size), bind: d3dBindConstantBuffer}
	if failed(g.d3d.call(d3dCreateBuffer, uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&p.cb)))) {
		return nil
	}
	p.failed = false
	return p
}

// compileHLSL compiles entry of src for target, or logs the compiler's
// errors.
func compileHLSL(src, entry, target string) ([]byte, bool) {
	if err := d3dCompile.Find(); err != nil {
		log.Printf("nsgo: shader: %v", err)
		return nil, false
	}
	code := []byte(src)
	name, e, t := append([]byte("nsgo"), 0), append([]byte(entry), 0), append([]byte(target), 0)
	var blob, errs *comObject
	hr, _, _ := d3dCompile.Call(uintptr(unsafe.Pointer(&code[0])), uintptr(len(code)), uintptr(unsafe.Pointer(&name[0])),
		0, 0, uintptr(unsafe.Pointer(&e[0])), uintptr(unsafe.Pointer(&t[0])), 0, 0,
		uintptr(unsafe.Pointer(&blob)), uintptr(unsafe.Pointer(&errs)))
	if errs != nil {
		if failed(hr) {
			log.Printf("nsgo: shader: HLSL: %s", blobString(errs))
		}
		errs.release()
	}
	if failed(hr) || blob == nil {
		return nil, false
	}
	defer blob.release()
	return []byte(blobString(blob)), true
}

func blobString(b *comObject) string {
	ptr, _, _ := syscall.SyscallN(b.vtbl[blobGetBufferPointer], b.ptr())
	n, _, _ := syscall.SyscallN(b.vtbl[blobGetBufferSize], b.ptr())
	if ptr == 0 || n == 0 {
		return ""
	}
	// The blob's own memory, which outlives this copy.
	return string(unsafe.Slice(*(**byte)(unsafe.Pointer(&ptr)), n))
}

// drawShader draws v's shader over its width by height points: again
// into its texture when it changed, then the texture.
func (p *painter) drawShader(v *view, width, height float32) {
	sv := v.shaderView
	if sv == nil || sv.shader == nil || sv.shader.Program == nil {
		return
	}
	g := p.g
	prog := g.program(sv.shader.Program)
	pw, ph := uint32(math.Ceil(float64(width*p.scale))), uint32(math.Ceil(float64(height*p.scale)))
	if prog == nil || pw == 0 || ph == 0 {
		return
	}
	if sv.target == nil || sv.pw != pw || sv.ph != ph || sv.generation != gfxGeneration {
		sv.release()
		if !sv.makeTarget(p, pw, ph) {
			return
		}
		sv.dirty = true
	}
	if sv.dirty {
		// What Direct2D recorded so far draws before Direct3D's.
		p.dc.call(rtFlush, 0, 0)
		sv.render(p, prog, width, height)
		sv.dirty = false
	}
	dst := rectF{0, 0, width, height}
	p.dc.callF(rtDrawBitmap, 0b100, sv.bitmap.ptr(), uintptr(unsafe.Pointer(&dst)), f32(1), d2dInterpolationLinear, 0)
}

// makeTarget makes the texture the view draws into, pw by ph, and
// Direct2D's bitmap of it.
func (sv *shaderView) makeTarget(p *painter, pw, ph uint32) bool {
	g := p.g
	desc := texture2DDesc{width: pw, height: ph, mipLevels: 1, arraySize: 1, format: dxgiFormatBGRA,
		sampleCount: 1, bindFlags: d3dBindRenderTarget | d3dBindShaderResource}
	if failed(g.d3d.call(d3dCreateTexture2D, uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&sv.target)))) {
		return false
	}
	if failed(g.d3d.call(d3dCreateRenderTargetView, sv.target.ptr(), 0, uintptr(unsafe.Pointer(&sv.rtv)))) {
		sv.release()
		return false
	}
	surface := sv.target.query(&iidDXGISurfaceForShader)
	if surface == nil {
		sv.release()
		return false
	}
	defer surface.release()
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaPremultiplied, dpiX: 96 * p.scale,
		dpiY: 96 * p.scale}
	if failed(p.dc.call(dcCreateBitmapFromDxgiSurface, surface.ptr(), uintptr(unsafe.Pointer(&props)),
		uintptr(unsafe.Pointer(&sv.bitmap)))) {
		sv.release()
		return false
	}
	sv.pw, sv.ph, sv.generation = pw, ph, gfxGeneration
	return true
}

// render runs prog over the view's texture with its uniforms and images.
func (sv *shaderView) render(p *painter, prog *d3dProgram, width, height float32) {
	g, w := p.g, p.w
	bridge := 3 + 2*prog.textures
	sv.lanes = append(sv.lanes[:0], make([]float64, bridge+len(sv.shader.Lanes))...)
	copy(sv.lanes[bridge:], sv.shader.Lanes)
	w.animator.ShaderLanes(sv.id, sv.lanes[bridge:])
	sv.lanes[0], sv.lanes[1], sv.lanes[2] = float64(width), float64(height), float64(p.scale)
	srvs := make([]uintptr, prog.textures)
	for i := range prog.textures {
		if sv.shader.Program.WGSL {
			if pic := sv.pictures[i]; pic != nil && pic.srv != nil {
				srvs[i] = pic.srv.ptr()
				sv.lanes[3+2*i], sv.lanes[4+2*i] = float64(pic.pw), float64(pic.ph)
			}
			continue
		}
		if i < len(sv.shader.Images) {
			if im := sv.image(w, g, sv.shader.Images[i]); im != nil {
				srvs[i] = im.srv.ptr()
				sv.lanes[3+2*i], sv.lanes[4+2*i] = im.w, im.h
			}
		}
	}
	if cap(sv.buf) < prog.size {
		sv.buf = make([]byte, prog.size)
	}
	sv.buf = sv.buf[:prog.size]
	clear(sv.buf)
	shaderir.PutLanes(sv.buf, prog.offsets, prog.types, sv.lanes, 4)

	var ctx *comObject
	g.d3d.call(d3dGetImmediateContext, uintptr(unsafe.Pointer(&ctx)))
	if ctx == nil {
		return
	}
	defer ctx.release()
	rtv := sv.rtv.ptr()
	ctx.call(ctxOMSetRenderTargets, 1, uintptr(unsafe.Pointer(&rtv)), 0)
	vp := d3dViewport{w: float32(sv.pw), h: float32(sv.ph), maxDepth: 1}
	ctx.call(ctxRSSetViewports, 1, uintptr(unsafe.Pointer(&vp)))
	var clear [4]float32
	ctx.call(ctxClearRenderTargetView, rtv, uintptr(unsafe.Pointer(&clear[0])))
	ctx.call(ctxIASetInputLayout, 0)
	ctx.call(ctxIASetPrimitiveTopology, d3dTriangleList)
	ctx.call(ctxVSSetShader, prog.vs.ptr(), 0, 0)
	ctx.call(ctxPSSetShader, prog.ps.ptr(), 0, 0)
	ctx.call(ctxUpdateSubresource, prog.cb.ptr(), 0, 0, uintptr(unsafe.Pointer(&sv.buf[0])), 0, 0)
	cb := prog.cb.ptr()
	ctx.call(ctxPSSetConstantBuffers, 0, 1, uintptr(unsafe.Pointer(&cb)))
	if len(srvs) > 0 {
		ctx.call(ctxPSSetShaderResources, 0, uintptr(len(srvs)), uintptr(unsafe.Pointer(&srvs[0])))
	}
	ctx.call(ctxDraw, 3, 0)
	// Unbound, the texture is Direct2D's to read.
	ctx.call(ctxOMSetRenderTargets, 0, 0, 0)
	if len(srvs) > 0 {
		clear := make([]uintptr, len(srvs))
		ctx.call(ctxPSSetShaderResources, 0, uintptr(len(clear)), uintptr(unsafe.Pointer(&clear[0])))
	}
}

// image returns src's texture for the view, loading the image the first
// time; nil until it has loaded, when the view draws again.
func (sv *shaderView) image(w *window, g *gfx, src driver.ImageSource) *d3dImage {
	key := src.Key()
	if im := sv.images[key]; im != nil && im.generation == gfxGeneration {
		return im
	}
	s := w.canvasImage(src)
	if s == nil || s.pic == nil {
		return nil
	}
	pic := s.pic
	desc := texture2DDesc{width: uint32(pic.width), height: uint32(pic.height), mipLevels: 1, arraySize: 1,
		format: dxgiFormatBGRA, sampleCount: 1, bindFlags: d3dBindShaderResource}
	pix := pic.frames[0].Pix
	data := struct {
		mem         uintptr
		pitch, size uint32
	}{uintptr(unsafe.Pointer(&pix[0])), uint32(pic.width * 4), 0}
	im := &d3dImage{w: float64(pic.width), h: float64(pic.height), generation: gfxGeneration}
	if failed(g.d3d.call(d3dCreateTexture2D, uintptr(unsafe.Pointer(&desc)), uintptr(unsafe.Pointer(&data)),
		uintptr(unsafe.Pointer(&im.tex)))) {
		return nil
	}
	if failed(g.d3d.call(d3dCreateShaderResourceView, im.tex.ptr(), 0, uintptr(unsafe.Pointer(&im.srv)))) {
		im.tex.release()
		return nil
	}
	if sv.images == nil {
		sv.images = map[string]*d3dImage{}
	}
	sv.images[key] = im
	return im
}
