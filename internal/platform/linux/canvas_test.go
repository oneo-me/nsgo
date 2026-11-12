//go:build linux && !android

package linux

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvastest"
)

// TestCanvasSamples plays the shared samples into an image surface at
// canvastest.Scale, once their image loaded. NSGO_CANVAS_PNG names a
// directory to write them to.
func TestCanvasSamples(t *testing.T) {
	if err := openLibs(); err != nil {
		t.Skip(err)
	}
	logo := canvastest.Logo("../../..")
	w := &window{animator: driver.NewAnimator(nil), env: driver.Environment{PixelRatio: canvastest.Scale}}
	s := w.canvasImage(driver.ImageSource{Data: logo}, &view{})
	// The load lands on the window's task queue, which no loop runs here.
	for deadline := time.Now().Add(5 * time.Second); s.pic == nil; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the image did not load")
		}
		w.mu.Lock()
		tasks := w.tasks
		w.tasks = nil
		w.mu.Unlock()
		for _, fn := range tasks {
			fn()
		}
	}
	for _, s := range canvastest.Samples(w, logo) {
		t.Run(s.Name, func(t *testing.T) {
			for i := range s.Drawing.Ops {
				if op := &s.Drawing.Ops[i]; op.Kind == driver.DrawImage {
					op.Image = &driver.ImageSource{Data: logo}
				}
			}
			width, height := s.Width*canvastest.Scale, s.Height*canvastest.Scale
			surface := cairoImageSurfaceCreate(0, int32(width), int32(height))
			defer cairoSurfaceDestroy(surface)
			cr := cairoCreate(surface)
			cairoSetSourceRGBA(cr, 1, 1, 1, 1)
			cairoPaint(cr)
			cairoScale(cr, canvastest.Scale, canvastest.Scale)
			w.playCanvas(&view{drawing: s.Drawing}, cr, rect{w: float64(s.Width), h: float64(s.Height)})
			cairoDestroy(cr)
			cairoSurfaceFlush(surface)
			img := image.NewRGBA(image.Rect(0, 0, width, height))
			data, stride := cairoImageSurfaceGetData(surface), int(cairoImageSurfaceGetStride(surface))
			rows := unsafe.Slice((*byte)(cPointer(data)), stride*height)
			for y := range height {
				for x := range width {
					b := rows[y*stride+x*4:]
					// Premultiplied BGRA over white: alpha 0 reads white.
					img.Pix[y*img.Stride+x*4+0] = b[2] + 255 - b[3]
					img.Pix[y*img.Stride+x*4+1] = b[1] + 255 - b[3]
					img.Pix[y*img.Stride+x*4+2] = b[0] + 255 - b[3]
					img.Pix[y*img.Stride+x*4+3] = 255
				}
			}
			if dir := os.Getenv("NSGO_CANVAS_PNG"); dir != "" {
				if f, err := os.Create(dir + "/linux-" + s.Name + ".png"); err == nil {
					png.Encode(f, img)
					f.Close()
				}
			}
			for _, f := range s.Check(img) {
				t.Error(f)
			}
		})
	}
}
