//go:build js

package web

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"image/png"
	"syscall/js"
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Run in a browser, which has the codecs: nsgo test -target web.
func TestPreparePicked(t *testing.T) {
	if !js.Global().Get("createImageBitmap").Truthy() {
		t.Skip("no browser codecs here: run with nsgo test -target web")
	}
	m := stdimage.NewNRGBA(stdimage.Rect(0, 0, 400, 200))
	for i := range m.Pix {
		m.Pix[i] = 0xff
	}
	m.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var b bytes.Buffer
	png.Encode(&b, m)
	files := []driver.PickedFile{{Name: "a.png", Data: b.Bytes()}, {Name: "b.png", Data: b.Bytes()}, {Name: "c.txt", Data: []byte("text")}}
	got := make(chan []driver.PickedFile, 1)
	preparePicked(driver.PickRequest{Limit: 3, MaxDimension: 100}, files, func(f []driver.PickedFile) { got <- f })
	out := <-got
	if f := out[0]; f.MIME != "image/png" || f.Size != (driver.Size{Width: 100, Height: 50}) || driver.SniffImage(f.Data) != "image/png" {
		t.Errorf("a scaled PNG = %s %v %s", f.MIME, f.Size, driver.SniffImage(f.Data))
	}
	if c, err := png.DecodeConfig(bytes.NewReader(out[0].Data)); err != nil || c.Width != 100 || c.Height != 50 {
		t.Errorf("the scaled PNG decodes to %+v, %v", c, err)
	}
	if f := out[2]; string(f.Data) != "text" {
		t.Errorf("a file that is no image = %+v", f)
	}
	var j bytes.Buffer
	jpeg.Encode(&j, m, nil)
	preparePicked(driver.PickRequest{Limit: 1, Quality: 0.5}, []driver.PickedFile{{Name: "d.jpeg", Data: j.Bytes()}},
		func(f []driver.PickedFile) { got <- f })
	if f := (<-got)[0]; f.MIME != "image/jpeg" || driver.SniffImage(f.Data) != "image/jpeg" || f.Name != "d.jpeg" ||
		f.Size != (driver.Size{Width: 400, Height: 200}) || bytes.Equal(f.Data, j.Bytes()) {
		t.Errorf("a JPEG encoded again = %s %s %v", f.MIME, f.Name, f.Size)
	}
}
