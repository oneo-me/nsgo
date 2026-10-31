//go:build darwin

package apple

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/shader"
	"github.com/oneo-me/nsgo/internal/shaderir"
	"github.com/oneo-me/nsgo/internal/shaderir/msl"
)

// TestShadersReadImages draws a shader that reads an image into a texture
// with Metal and reads the pixels back: the image's left half red, its
// right half blue, stretched over the target and tinted by a uniform.
func TestShadersReadImages(t *testing.T) {
	if !loadMetal() {
		t.Skip("no Metal device")
	}
	ir, err := shader.CompileFragment([]byte(`package main

var Alpha float

func Fragment(dst vec4, pos vec2) vec4 {
	return image0At(pos*image0Size()/rectSize()) * Alpha
}
`))
	if err != nil {
		t.Fatal(err)
	}
	state, msg := metalPipelineState(msl.Fragment(ir, "nsgoShader") + metalStage(ir.TextureCount))
	if state == 0 {
		t.Fatal(msg)
	}
	// A 4 by 2 image: red on the left, blue on the right.
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for x := range 4 {
		for y := range 2 {
			c := color.RGBA{255, 0, 0, 255}
			if x >= 2 {
				c = color.RGBA{0, 0, 255, 255}
			}
			src.SetRGBA(x, y, c)
		}
	}
	var encoded bytes.Buffer
	png.Encode(&encoded, src)
	d, err := decode(context.Background(), encoded.Bytes(), request{ratio: 1, scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	tex := textureOf(d.image, 4, 2)
	if tex == 0 {
		t.Fatal("no texture")
	}
	const w, h = 8, 4
	desc := objc.ID(objc.GetClass("MTLTextureDescriptor")).Send(
		sel("texture2DDescriptorWithPixelFormat:width:height:mipmapped:"), uint(mtlPixelFormatBGRA8Unorm), uint(w), uint(h), false)
	const usageRenderTarget, usageShaderRead = 4, 1
	desc.Send(sel("setUsage:"), uint(usageRenderTarget|usageShaderRead))
	target := mtlDevice.Send(sel("newTextureWithDescriptor:"), desc)
	lanes := []float64{w, h, 1, 4, 2, 0.5}
	offsets, size := msl.UniformLanes(ir)
	buf := make([]byte, size)
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
	enc.Send(sel("setFragmentTexture:atIndex:"), tex, uint(0))
	enc.Send(sel("drawPrimitives:vertexStart:vertexCount:"), uint(mtlPrimitiveTypeTriangle), uint(0), uint(3))
	enc.Send(sel("endEncoding"))
	cb.Send(sel("commit"))
	cb.Send(sel("waitUntilCompleted"))
	pix := make([]byte, w*h*4)
	target.Send(sel("getBytes:bytesPerRow:fromRegion:mipmapLevel:"), unsafe.Pointer(&pix[0]), uint(w*4),
		mtlRegion{W: w, H: h, D: 1}, uint(0))
	// BGRA, premultiplied, at half alpha.
	left, right := pix[(1*w+1)*4:], pix[(1*w+6)*4:]
	if left[2] < 120 || left[0] > 10 || left[3] < 120 || left[3] > 135 {
		t.Errorf("left pixel %v, want half red", left[:4])
	}
	if right[0] < 120 || right[2] > 10 {
		t.Errorf("right pixel %v, want half blue", right[:4])
	}
}
