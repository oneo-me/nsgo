//go:build windows

package windows

import (
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
)

// A scroll view's edge effects draw over its children, each side's band
// from one copy of what the target holds (backdrop): a soft band draws
// the copy blurred more toward the side, a level a time, each fading in
// along the band through a gradient opacity brush and blurring the copy
// by its whole blur (driver.BlurLevel.Total), under the tint fading out;
// a hard band draws it evenly blurred under the tint, with the hairline.

// drawEdges draws scroll view v's edge effects at world.
func (p *painter) drawEdges(v *view, world matrix) {
	var copied *comObject
	defer func() {
		if copied != nil {
			copied.release()
		}
	}()
	size := frameOf(v).Size
	for side := range v.scroll.Edges {
		e := &v.scroll.Edges[side]
		band := e.Band()
		if band <= 0 {
			continue
		}
		if copied == nil {
			if copied = p.backdrop(); copied == nil {
				return
			}
		}
		r := driver.BandRect(driver.Side(side), size, band)
		w, h := float32(r.Size.Width), float32(r.Size.Height)
		at := translation(float64(r.Origin.X), float64(r.Origin.Y)).then(world)
		x0, y0, x1, y1 := driver.EdgeLine(driver.Side(side))
		fade := func(from, to driver.Color, start, end float32) *driver.GradientPaint {
			return &driver.GradientPaint{X0: x0, Y0: y0, X1: x1, Y1: y1,
				Stops: []driver.GradientStop{{Color: from, Offset: start}, {Color: to, Offset: end}}}
		}
		blurred := func(sigma driver.Points, mask *driver.GradientPaint) {
			image, release := p.blurBackdrop(copied, float32(sigma))
			if image == nil {
				return
			}
			defer release()
			p.setTransform(at)
			p.pushClipRect(rectF{0, 0, w, h})
			masked := mask != nil && p.pushMask(mask, w, h)
			p.setTransform(identity)
			p.dc.call(dcDrawImage, image.ptr(), 0, 0, d2dInterpolationLinear, 0)
			if masked {
				p.pop()
			}
			p.pop()
		}
		if e.Drawn() == driver.EdgeEffectSoft {
			for _, l := range driver.SoftEdgeLevels() {
				blurred(l.Total, fade(driver.RGB(0), driver.RGBA(0, 0), l.Full, l.Gone))
			}
			p.setTransform(at)
			if brush := p.gradientBrush(fade(e.Tint.Faded(driver.SoftEdgeTint), e.Tint.Faded(0), 0, 1), w, h); brush != nil {
				rect := rectF{0, 0, w, h}
				p.dc.call(rtFillRectangle, uintptr(unsafe.Pointer(&rect)), brush.ptr())
				brush.release()
			}
			continue
		}
		blurred(driver.HardEdgeBlur, nil)
		p.setTransform(at)
		p.fillRect(rectF{0, 0, w, h}, e.Tint.Faded(driver.HardEdgeTint))
		line := driver.HairlineRect(driver.Side(side), r.Size)
		p.fillRect(rectF{float32(line.Origin.X), float32(line.Origin.Y),
			float32(line.Origin.X + line.Size.Width), float32(line.Origin.Y + line.Size.Height)}, e.Line)
	}
	p.setTransform(world)
}

// blurBackdrop returns copied blurred by sigma through Direct2D's
// Gaussian blur, and what releases it.
func (p *painter) blurBackdrop(copied *comObject, sigma float32) (*comObject, func()) {
	var effect, out *comObject
	if failed(p.dc.call(dcCreateEffect, uintptr(unsafe.Pointer(&clsidGaussianBlur)), uintptr(unsafe.Pointer(&effect)))) {
		return nil, nil
	}
	effect.call(effectSetValue, 0, d2dPropertyFloat, uintptr(unsafe.Pointer(&sigma)), 4)
	border := uint32(1) // D2D1_BORDER_MODE_HARD
	effect.call(effectSetValue, 2, d2dPropertyEnum, uintptr(unsafe.Pointer(&border)), 4)
	effect.call(effectSetInput, 0, copied.ptr(), 1)
	effect.call(effectGetOutput, uintptr(unsafe.Pointer(&out)))
	if out == nil {
		effect.release()
		return nil, nil
	}
	return out, func() {
		out.release()
		effect.release()
	}
}
