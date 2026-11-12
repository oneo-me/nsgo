//go:build linux && !android

package linux

import (
	"image"
	"testing"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvastest"
)

// rasterImage returns v's raster's pixels.
func rasterImage(r *canvasRaster) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, r.pw, r.ph))
	data, stride := cairoImageSurfaceGetData(r.surface), int(cairoImageSurfaceGetStride(r.surface))
	rows := unsafe.Slice((*byte)(cPointer(data)), stride*r.ph)
	for y := range r.ph {
		for x := range r.pw {
			p := rows[y*stride+x*4:]
			img.Pix[y*img.Stride+x*4] = p[2]
			img.Pix[y*img.Stride+x*4+1] = p[1]
			img.Pix[y*img.Stride+x*4+2] = p[0]
			img.Pix[y*img.Stride+x*4+3] = p[3]
		}
	}
	return img
}

// TestPartialRastersMatchFullOnes draws the split samples whole, moves
// their bound value and draws what it damaged over the kept static
// part, and compares that with drawing them whole at the new value.
func TestPartialRastersMatchFullOnes(t *testing.T) {
	if err := openLibs(); err != nil {
		t.Skip(err)
	}
	for _, s := range canvastest.Split(nil) {
		t.Run(s.Name, func(t *testing.T) {
			const scale = canvastest.Scale
			pw, ph := s.Width*scale, s.Height*scale
			draw := func(value float64, then func(w *window, v *view)) *image.RGBA {
				w := &window{animator: driver.NewAnimator(nil), env: driver.Environment{PixelRatio: scale}}
				w.animator.Apply([]driver.AnimationOp{{Kind: driver.AnimCreateValue, Node: 1, Value: value}})
				v := &view{id: 1}
				w.views = []*view{nil, v}
				w.setCanvas(v, s.Drawing)
				if v.canvas.split.At == 0 {
					t.Fatal("no static part is kept")
				}
				w.changedCanvases()
				w.rasterize(v, pw, ph, scale)
				v.canvas.whole = false
				if then != nil {
					then(w, v)
					w.rasterize(v, pw, ph, scale)
				}
				defer v.canvas.raster.release()
				if v.canvas.raster.static == 0 {
					t.Fatal("the static part's pixels were not kept")
				}
				return rasterImage(v.canvas.raster)
			}
			want := draw(60, nil)
			moved := draw(120, func(w *window, v *view) {
				w.animator.Apply([]driver.AnimationOp{{Kind: driver.AnimSet, Node: 1, Value: 60}})
				w.changedCanvases()
			})
			if n, at := canvastest.Differ(want, moved); n > 0 {
				t.Errorf("a partial redraw differs from a full one in %d pixels, the first at %v", n, at)
			}
			// All of it drawn over the static part.
			over := draw(60, func(w *window, v *view) {
				v.canvas.area = driver.Bounds{Size: driver.Size{Width: driver.Points(s.Width), Height: driver.Points(s.Height)}}
			})
			if n, at := canvastest.Differ(want, over); n > 0 {
				t.Errorf("drawing over the static part differs in %d pixels, the first at %v", n, at)
			}
		})
	}
}
