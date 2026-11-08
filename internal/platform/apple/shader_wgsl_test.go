//go:build darwin

package apple

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/shader/wgsl"
	"github.com/oneo-me/nsgo/internal/shaderir"
	"github.com/oneo-me/nsgo/internal/shaderir/msl"
)

// TestWGSLCompilesForMetal compiles the WGSL compiler's test shaders with
// Metal, which checks the MSL they become.
func TestWGSLCompilesForMetal(t *testing.T) {
	if !loadMetal() {
		t.Skip("no Metal device")
	}
	files, _ := filepath.Glob(filepath.Join("..", "..", "shader", "wgsl", "testdata", "*.wgsl"))
	if len(files) == 0 {
		t.Fatal("no shaders")
	}
	srcs := map[string]string{"crossfade": wgsl.Crossfade}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		srcs[filepath.Base(f)] = string(b)
	}
	for name, src := range srcs {
		p, err := wgsl.Compile(src)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if state, msg := metalPipelineState(msl.Fragment(p.IR, "nsgoShader") + metalStage(p.IR.TextureCount)); state == 0 {
			t.Errorf("%s: %s", name, msg)
		}
	}
}

// drawMetal draws ir into a w by h target with lanes and textures, and
// returns its pixels, BGRA premultiplied.
func drawMetal(t *testing.T, ir *shaderir.Program, lanes []float64, textures []objc.ID, w, h int) []byte {
	t.Helper()
	state, msg := metalPipelineState(msl.Fragment(ir, "nsgoShader") + metalStage(ir.TextureCount))
	if state == 0 {
		t.Fatal(msg)
	}
	desc := objc.ID(objc.GetClass("MTLTextureDescriptor")).Send(
		sel("texture2DDescriptorWithPixelFormat:width:height:mipmapped:"), uint(mtlPixelFormatBGRA8Unorm), uint(w), uint(h), false)
	const usageRenderTarget, usageShaderRead = 4, 1
	desc.Send(sel("setUsage:"), uint(usageRenderTarget|usageShaderRead))
	target := mtlDevice.Send(sel("newTextureWithDescriptor:"), desc)
	offsets, size := msl.UniformLanes(ir)
	buf := make([]byte, max(size, 16))
	shaderir.PutLanes(buf, offsets, ir.LaneTypes(), lanes, 1)
	pass := objc.ID(objc.GetClass("MTLRenderPassDescriptor")).Send(sel("renderPassDescriptor"))
	att := pass.Send(selColorAttachments).Send(selObjectAtIndexedSubscript, uint(0))
	att.Send(sel("setTexture:"), target)
	att.Send(sel("setLoadAction:"), uint(mtlLoadActionClear))
	att.Send(sel("setStoreAction:"), uint(mtlStoreActionStore))
	cb := mtlQueue.Send(sel("commandBuffer"))
	enc := cb.Send(sel("renderCommandEncoderWithDescriptor:"), pass)
	enc.Send(sel("setRenderPipelineState:"), state)
	enc.Send(sel("setFragmentBytes:length:atIndex:"), unsafe.Pointer(&buf[0]), uint(len(buf)), uint(0))
	for i, tex := range textures {
		enc.Send(sel("setFragmentTexture:atIndex:"), tex, uint(i))
	}
	enc.Send(sel("drawPrimitives:vertexStart:vertexCount:"), uint(mtlPrimitiveTypeTriangle), uint(0), uint(3))
	enc.Send(sel("endEncoding"))
	cb.Send(sel("commit"))
	cb.Send(sel("waitUntilCompleted"))
	pix := make([]byte, w*h*4)
	target.Send(sel("getBytes:bytesPerRow:fromRegion:mipmapLevel:"), unsafe.Pointer(&pix[0]), uint(w*4),
		mtlRegion{W: uint(w), H: uint(h), D: 1}, uint(0))
	return pix
}

// TestWGSLDraws draws WGSL programs: shade's straight color comes out
// premultiplied, uv runs over the box, and sample_from and sample_to read
// pictures between their pixels, mixed by nsh.progress.
func TestWGSLDraws(t *testing.T) {
	if !loadMetal() {
		t.Skip("no Metal device")
	}
	p, err := wgsl.Compile(`struct Uniforms { alpha: f32 }
fn shade(uv: vec2f) -> vec4f { return vec4f(uv.x, 0.0, 1.0, uniforms.alpha); }`)
	if err != nil {
		t.Fatal(err)
	}
	// The bridge's lanes, the built-ins' (time, progress, overflow), then alpha.
	pix := drawMetal(t, p.IR, []float64{8, 4, 1, 0, 0, 0, 0.5}, nil, 8, 4)
	right := pix[(1*8+7)*4:]
	if right[0] < 120 || right[0] > 135 || right[2] < 100 || right[3] < 120 || right[3] > 135 {
		t.Errorf("right pixel %v, want half blue and red (BGRA), premultiplied", right[:4])
	}

	cross, err := wgsl.Compile(wgsl.Crossfade)
	if err != nil {
		t.Fatal(err)
	}
	solid := func(b, g, r, a byte) objc.ID {
		desc := objc.ID(objc.GetClass("MTLTextureDescriptor")).Send(
			sel("texture2DDescriptorWithPixelFormat:width:height:mipmapped:"), uint(mtlPixelFormatBGRA8Unorm), uint(2), uint(2), false)
		tex := mtlDevice.Send(sel("newTextureWithDescriptor:"), desc)
		px := []byte{b, g, r, a, b, g, r, a, b, g, r, a, b, g, r, a}
		tex.Send(sel("replaceRegion:mipmapLevel:withBytes:bytesPerRow:"), mtlRegion{W: 2, H: 2, D: 1}, uint(0), unsafe.Pointer(&px[0]), uint(8))
		return tex
	}
	from, to := solid(0, 0, 255, 255), solid(255, 0, 0, 255) // red, blue
	// Four textures: content and backdrop unused, from and to.
	lanes := []float64{8, 4, 1, 0, 0, 0, 0, 2, 2, 2, 2, 0, 0.25, 0}
	pix = drawMetal(t, cross.IR, lanes, []objc.ID{blankTexture(), blankTexture(), from, to}, 8, 4)
	px := pix[(2*8+4)*4:]
	if px[2] < 180 || px[2] > 200 || px[0] < 55 || px[0] > 75 || px[3] != 255 {
		t.Errorf("a quarter through, pixel %v, want three quarters red (BGRA)", px[:4])
	}
}

// TestPictureTakesLayers pictures a layer tree as it shows, top row
// first, with the overflow around it and its mask lifted.
func TestPictureTakesLayers(t *testing.T) {
	if !loadMetal() {
		t.Skip("no Metal device")
	}
	root := objc.ID(objc.GetClass("CALayer")).Send(selNew)
	root.Send(sel("setGeometryFlipped:"), true)
	root.Send(sel("setFrame:"), Rect(0, 0, 20, 10))
	top := objc.ID(objc.GetClass("CALayer")).Send(selNew)
	top.Send(sel("setFrame:"), Rect(0, 0, 20, 5))
	top.Send(sel("setBackgroundColor:"), CGColor(driver.RGB(0xff0000)))
	root.Send(sel("addSublayer:"), top)
	root.Send(sel("setMask:"), objc.ID(objc.GetClass("CALayer")).Send(selNew))
	var p Picture
	defer p.Release()
	if !p.Take(root, -2, -2, 24, 14, 2, 0) {
		t.Fatal("no picture")
	}
	if p.w != 48 || p.h != 28 {
		t.Fatalf("picture %dx%d", p.w, p.h)
	}
	data := cgBitmapContextGetData(p.ctx)
	px := unsafe.Slice(*(**byte)(unsafe.Pointer(&data)), p.w*p.h*4)
	at := func(x, y int) []byte { return px[(y*p.w+x)*4 : (y*p.w+x)*4+4] }
	if c := at(24, 8); c[2] != 255 || c[3] != 255 {
		t.Errorf("the top half is %v, want red (BGRA)", c)
	}
	if c := at(24, 22); c[3] != 0 {
		t.Errorf("the bottom half is %v, want clear", c)
	}
	if c := at(1, 8); c[3] != 0 {
		t.Errorf("the overflow is %v, want clear", c)
	}
	if root.Send(sel("mask")) == 0 {
		t.Error("the mask was not put back")
	}
}
