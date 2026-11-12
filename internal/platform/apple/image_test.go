//go:build darwin

package apple

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestImageSizeReadsTheHeader(t *testing.T) {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 7, 3)))
	if got, err := imageSize(b.Bytes()); err != nil || got != (driver.Size{Width: 7, Height: 3}) {
		t.Fatalf("PNG size = %v, %v", got, err)
	}
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="10"/>`)
	if got, err := imageSize(svg); err != nil || got != (driver.Size{Width: 20, Height: 10}) {
		t.Fatalf("SVG size = %v, %v", got, err)
	}
	if _, err := imageSize([]byte("not an image")); err == nil {
		t.Fatal("no error for bytes that are no image")
	}
}

func TestDecodePointsPerPixel(t *testing.T) {
	view := CGSize{Width: 100, Height: 100}
	for _, c := range []struct {
		mode driver.ResizeMode
		w, h float64
		want float64
	}{
		{driver.Center, 400, 200, 0.5}, // a 2x file at its natural size
		{driver.ScaleDown, 400, 200, 0.25},
		{driver.ScaleDown, 100, 50, 0.5},
		{driver.Contain, 400, 200, 0.25},
		{driver.Cover, 400, 200, 0.5},
	} {
		r := request{mode: c.mode, view: view, scale: 2}
		if got := r.pointsPerPixel(c.w, c.h); got != c.want {
			t.Errorf("%v %gx%g: %g points per pixel, want %g", c.mode, c.w, c.h, got, c.want)
		}
	}
}
