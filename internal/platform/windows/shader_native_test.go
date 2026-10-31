//go:build windows

package windows

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/shader"
)

// TestShadersReadImages draws a shader that reads an image with Direct3D
// into a view's texture, which the painter draws, and reads the pixels
// back: the image's left half red, its right half blue, stretched over
// the view at half alpha.
func TestShadersReadImages(t *testing.T) {
	g, err := devices()
	if err != nil {
		t.Skip(err)
	}
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
	pic, err := decode(context.Background(), encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	img := driver.ImageSource{Data: encoded.Bytes()}
	w := &window{scale: 1, animator: driver.NewAnimator(nil)}
	w.canvasImages = map[string]*imageSlot{img.Key(): {src: img, pic: pic}}
	ir, err := shader.CompileFragment([]byte(`package main

var Alpha float

func Fragment(dst vec4, pos vec2) vec4 {
	return image0At(pos*image0Size()/rectSize()) * Alpha
}
`))
	if err != nil {
		t.Fatal(err)
	}
	prog := &driver.ShaderProgram{IR: ir}
	v := &view{id: 1}
	w.setShader(v, &driver.Shader{Program: prog, Lanes: []float64{0.5}, Images: []driver.ImageSource{img}})

	const pw, ph = 8, 4
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaPremultiplied, dpiX: 96, dpiY: 96,
		options: d2dBitmapTarget}
	var target, readable *comObject
	if failed(g.dc.callSize(dcCreateBitmap, pw, ph, 0, 0, uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&target)))) {
		t.Fatal("no target bitmap")
	}
	defer target.release()
	props.options = d2dBitmapCPURead | d2dBitmapCannotDraw
	if failed(g.dc.callSize(dcCreateBitmap, pw, ph, 0, 0, uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&readable)))) {
		t.Fatal("no readable bitmap")
	}
	defer readable.release()
	g.dc.call(dcSetTarget, target.ptr())
	g.dc.callF(rtSetDpi, 0b11, f32(96), f32(96))
	g.dc.call(rtBeginDraw)
	clear := colorF{}
	g.dc.call(rtClear, uintptr(unsafe.Pointer(&clear)))
	p := newPainter(w, g)
	p.setTransform(identity)
	p.drawShader(v, pw, ph)
	p.release()
	if hr := g.dc.call(rtEndDraw, 0, 0); failed(hr) {
		t.Fatal(hresultError("EndDraw", hr))
	}
	g.dc.call(dcSetTarget, 0)
	v.shaderView.release()
	readable.call(bitmapCopyFromBitmap, 0, target.ptr(), 0)
	var mapped struct {
		pitch uint32
		bits  *byte
	}
	if failed(readable.call(bitmapMap, d2dMapRead, uintptr(unsafe.Pointer(&mapped)))) {
		t.Fatal("cannot map the pixels")
	}
	pix := unsafe.Slice(mapped.bits, int(mapped.pitch)*ph)
	left, right := pix[1*int(mapped.pitch)+1*4:], pix[1*int(mapped.pitch)+6*4:]
	readable.call(bitmapUnmap)
	if left[2] < 120 || left[0] > 10 || left[3] < 120 || left[3] > 135 {
		t.Errorf("left pixel %v, want half red", left[:4])
	}
	if right[0] < 120 || right[2] > 10 {
		t.Errorf("right pixel %v, want half blue", right[:4])
	}
}
