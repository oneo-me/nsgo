//go:build darwin

package apple

import (
	"math"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var (
	// CGSize goes as its two fields, which arm64 and x86-64 pass as two
	// floats.
	cgLayerCreateWithContext func(ctx uintptr, w, h float64, aux uintptr) uintptr
	cgLayerGetContext        func(layer uintptr) uintptr
	cgContextDrawLayerInRect func(ctx uintptr, r CGRect, layer uintptr)
)

func loadCanvasLayer(cg uintptr) {
	purego.RegisterLibFunc(&cgLayerCreateWithContext, cg, "CGLayerCreateWithContext")
	registerSyscall(&cgLayerGetContext, cg, "CGLayerGetContext")
	purego.RegisterLibFunc(&cgContextDrawLayerInRect, cg, "CGContextDrawLayerInRect")
}

// CanvasLayer is what a drawing's static part (driver.CanvasSplit)
// paints, rasterized once into a CGLayer made for the contexts it is
// drawn into, as NSH's macOS Prefix: a frame then draws the layer and
// plays only the painting after it (PlayOver).
type CanvasLayer struct {
	ref uintptr
	// Drawing, Width and Height (in pixels) are what it was made for.
	Drawing       *driver.Drawing
	Width, Height int
}

// NewCanvasLayer rasterizes what ops paint before split.At into a layer
// made for ctx, w × h points at scale pixels each. It reports nil when an
// image the part draws has not loaded yet, which the layer would miss.
func NewCanvasLayer(ctx uintptr, d *driver.Drawing, ops []driver.DrawOp, split driver.CanvasSplit,
	w, h, scale float64, host CanvasHost) *CanvasLayer {
	pw, ph := int(math.Ceil(w*scale)), int(math.Ceil(h*scale))
	if pw <= 0 || ph <= 0 || split.At <= 0 {
		return nil
	}
	// The size is in the user space of the context it is made for, not
	// its device's: in pixels.
	ref := cgLayerCreateWithContext(ctx, float64(pw), float64(ph), 0)
	if ref == 0 {
		return nil
	}
	lc := cgLayerGetContext(ref)
	if lc == 0 {
		Release(ref)
		return nil
	}
	// Top down, in points; the layer's base space points up.
	cgContextTranslateCTM(lc, 0, float64(ph))
	cgContextScaleCTM(lc, scale, -scale)
	host.BaseUp = true
	p := newPlayer(lc, ops[:split.At], driver.CanvasSplit{}, host)
	p.run()
	if p.waited {
		Release(ref)
		return nil
	}
	return &CanvasLayer{ref: ref, Drawing: d, Width: pw, Height: ph}
}

// Draw draws the layer over ctx's top-down w × h points.
func (l *CanvasLayer) Draw(ctx uintptr, w, h float64) {
	cgContextSaveGState(ctx)
	// Layers draw bottom up.
	cgContextTranslateCTM(ctx, 0, h)
	cgContextScaleCTM(ctx, 1, -1)
	cgContextDrawLayerInRect(ctx, Rect(0, 0, w, h), l.ref)
	cgContextRestoreGState(ctx)
}

// Release lets the layer go; l may be nil.
func (l *CanvasLayer) Release() {
	if l != nil && l.ref != 0 {
		Release(l.ref)
		l.ref = 0
	}
}
