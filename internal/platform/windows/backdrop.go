//go:build windows

package windows

import (
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Backdrop filters and blend modes read what lies behind a view out of
// the pass's target, as NSH's Windows backend does: the target is copied
// once the layers pushed so far are flattened into it, then a backdrop
// filter draws the copy through its effects under the view, clipped to
// the view's box, and a blend mode mixes the view, recorded into a
// command list, with the copy through Direct2D's blend effect. Inside a
// view drawn offscreen (filtered or turned out of its plane), whose
// target is a command list, there is no backdrop: the blend is normal and
// the backdrop unfiltered.

var iidD2D1Bitmap = parseGUID("a2296057-ea42-4099-983b-539fb6505426")

// backdrop returns a copy of what the target holds, or nil when it holds
// no pixels to copy.
func (p *painter) backdrop() *comObject {
	var target *comObject
	p.dc.call(dcGetTarget, uintptr(unsafe.Pointer(&target)))
	if target == nil {
		return nil
	}
	defer target.release()
	bitmap := target.query(&iidD2D1Bitmap)
	if bitmap == nil {
		return nil
	}
	bitmap.release()
	sw := p.w.swap
	if sw == nil {
		return nil
	}
	// A copy takes the target's format, its alpha mode included.
	props := targetProperties(p.scale, sw.composed)
	props.options = 0
	var copied *comObject
	if failed(p.dc.callSize(dcCreateBitmap, max(sw.width, 1), max(sw.height, 1), 0, 0, uintptr(unsafe.Pointer(&props)),
		uintptr(unsafe.Pointer(&copied)))) {
		return nil
	}
	// The layers pushed so far hold what they drew until they pop.
	p.flatten(func() {
		p.dc.call(rtFlush, 0, 0)
		copied.call(bitmapCopyFromBitmap, 0, target.ptr(), 0)
	})
	return copied
}

// drawBackdrop draws what is behind the view through its backdrop
// filters, clipped to box b; world places the view.
func (p *painter) drawBackdrop(e *driver.Effects, b *box, world matrix) {
	if len(e.BackdropFilters) == 0 {
		return
	}
	copied := p.backdrop()
	if copied == nil {
		return
	}
	defer copied.release()
	var objects []*comObject
	defer func() {
		for _, o := range objects {
			o.release()
		}
	}()
	image := copied
	chain := func(clsid *guid, set func(e *comObject)) {
		var e, out *comObject
		if failed(p.dc.call(dcCreateEffect, uintptr(unsafe.Pointer(clsid)), uintptr(unsafe.Pointer(&e)))) {
			return
		}
		objects = append(objects, e)
		set(e)
		e.call(effectSetInput, 0, image.ptr(), 1)
		e.call(effectGetOutput, uintptr(unsafe.Pointer(&out)))
		if out != nil {
			objects = append(objects, out)
			image = out
		}
	}
	m, color, blur, _ := driver.SplitFilters(e.BackdropFilters)
	if blur > 0 {
		sigma := float32(blur)
		chain(&clsidGaussianBlur, func(e *comObject) {
			e.call(effectSetValue, 0, d2dPropertyFloat, uintptr(unsafe.Pointer(&sigma)), 4)
			// The edges blur with what is beyond the window as clamped.
			border := uint32(1) // D2D1_BORDER_MODE_HARD
			e.call(effectSetValue, 2, d2dPropertyEnum, uintptr(unsafe.Pointer(&border)), 4)
		})
	}
	if color {
		var rows [20]float32
		for in := range 5 {
			for out := range 4 {
				rows[in*4+out] = m[out*5+in]
			}
		}
		chain(&clsidColorMatrix, func(e *comObject) {
			e.call(effectSetValue, 0, d2dPropertyMatrix5x4, uintptr(unsafe.Pointer(&rows[0])), 80)
		})
	}
	p.setTransform(world)
	p.pushClipBox(b)
	p.setTransform(identity)
	p.dc.call(dcDrawImage, image.ptr(), 0, 0, d2dInterpolationLinear, 0)
	p.pop()
	p.setTransform(world)
}

// drawBlended draws what record draws mixed with what is behind it by
// mode, within area of the view at world; false when the target holds no
// backdrop, for the caller to draw it as it is.
func (p *painter) drawBlended(mode driver.BlendMode, world matrix, area rectF, record func()) bool {
	copied := p.backdrop()
	if copied == nil {
		return false
	}
	defer copied.release()
	var list *comObject
	if failed(p.dc.call(dcCreateCommandList, uintptr(unsafe.Pointer(&list)))) {
		return false
	}
	defer list.release()
	p.flatten(func() {
		var target *comObject
		p.dc.call(dcGetTarget, uintptr(unsafe.Pointer(&target)))
		p.dc.call(dcSetTarget, list.ptr())
		pushed, split, m := p.pushed, p.split, p.m
		p.pushed, p.split = nil, nil
		p.setTransform(world)
		record()
		for len(p.pushed) > 0 {
			p.pop()
		}
		p.pushed, p.split = pushed, split
		p.dc.call(dcSetTarget, target.ptr())
		target.release()
		p.m = m
	})
	if failed(list.call(commandListClose)) {
		return true
	}
	clsid, value := &clsidBlend, uint32(0)
	switch mode {
	case driver.BlendPlusLighter:
		clsid, value = &clsidComposite, compositeModes[driver.CompositeLighter]
	case driver.BlendPlusDarker:
		value = d2dBlendLinearBurn
	default:
		value = blendModes[mode-driver.BlendMultiply]
	}
	var e, out *comObject
	if failed(p.dc.call(dcCreateEffect, uintptr(unsafe.Pointer(clsid)), uintptr(unsafe.Pointer(&e)))) {
		return true
	}
	defer e.release()
	e.call(effectSetValue, 0, d2dPropertyEnum, uintptr(unsafe.Pointer(&value)), 4)
	e.call(effectSetInput, 0, copied.ptr(), 1)
	e.call(effectSetInput, 1, list.ptr(), 1)
	e.call(effectGetOutput, uintptr(unsafe.Pointer(&out)))
	if out == nil {
		return true
	}
	defer out.release()
	// The mix replaces what is behind within the view's area.
	p.setTransform(world)
	p.pushClipRect(area)
	p.setTransform(identity)
	p.dc.call(dcDrawImage, out.ptr(), 0, 0, d2dInterpolationLinear, compositeCopy)
	p.pop()
	p.setTransform(world)
	return true
}
