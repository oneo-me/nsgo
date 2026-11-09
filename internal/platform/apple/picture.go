//go:build darwin

package apple

import (
	"math"

	"github.com/ebitengine/purego/objc"
)

// Picture is what a shader effect or transition reads of views: a layer
// tree drawn with renderInContext: into a bitmap it keeps, made again
// only when its size changes, and the Metal texture the bitmap goes into.
// renderInContext: draws no Metal layer, video, web page, backdrop blur or
// glass.
type Picture struct {
	ctx  uintptr
	tex  objc.ID
	w, h int // pixels
}

// Take draws layer's tree into the picture: the rectangle of w by h
// points from (x, y) in layer's coordinates, at scale pixels a point,
// hiding hide's layer (0 for none) and lifting layer's mask while it
// draws. It reports whether it drew.
func (p *Picture) Take(layer objc.ID, x, y, w, h, scale float64, hide objc.ID) bool {
	pw, ph := int(math.Ceil(w*scale)), int(math.Ceil(h*scale))
	if pw <= 0 || ph <= 0 || layer == 0 || !loadMetal() {
		return false
	}
	pw, ph = min(pw, maxPictureSide), min(ph, maxPictureSide)
	if pw != p.w || ph != p.h || p.ctx == 0 {
		p.Release()
		bitmapsLoaded.Do(loadBitmaps)
		const premultipliedFirst, byteOrder32Little = 2, 2 << 12
		p.ctx = cgBitmapContextCreate(0, uintptr(pw), uintptr(ph), 8, uintptr(pw*4), srgb, premultipliedFirst|byteOrder32Little)
		if p.ctx == 0 {
			return false
		}
		desc := objc.ID(Class("MTLTextureDescriptor")).Send(
			sel("texture2DDescriptorWithPixelFormat:width:height:mipmapped:"), uint(mtlPixelFormatBGRA8Unorm), uint(pw), uint(ph), false)
		p.tex = mtlDevice.Send(sel("newTextureWithDescriptor:"), desc)
		p.w, p.h = pw, ph
	}
	ctx := p.ctx
	cgContextClearRect(ctx, Rect(0, 0, float64(pw), float64(ph)))
	cgContextSaveGState(ctx)
	// Layers draw with y down from the top, as the bitmap's rows go.
	cgContextTranslateCTM(ctx, 0, float64(ph))
	cgContextScaleCTM(ctx, scale, -scale)
	cgContextTranslateCTM(ctx, -x, -y)
	mask := layer.Send(sel("mask"))
	WithoutAnimation(func() {
		if mask != 0 {
			mask.Send(sel("retain"))
			layer.Send(sel("setMask:"), objc.ID(0))
		}
		if hide != 0 {
			hide.Send(sel("setHidden:"), true)
		}
		layer.Send(sel("renderInContext:"), ctx)
		if hide != 0 {
			hide.Send(sel("setHidden:"), false)
		}
		if mask != 0 {
			layer.Send(sel("setMask:"), mask)
			mask.Send(selRelease)
		}
	})
	cgContextRestoreGState(ctx)
	p.tex.Send(sel("replaceRegion:mipmapLevel:withBytes:bytesPerRow:"), mtlRegion{W: uint(pw), H: uint(ph), D: 1}, uint(0),
		cgBitmapContextGetData(ctx), uint(pw*4))
	return true
}

// maxPictureSide bounds a picture's side in pixels, as NSH's frames are.
const maxPictureSide = 4096

// Texture returns the picture's texture and its size in pixels, 0 before
// it was taken.
func (p *Picture) Texture() (objc.ID, int, int) { return p.tex, p.w, p.h }

// Release lets the picture's bitmap and texture go.
func (p *Picture) Release() {
	if p.ctx != 0 {
		Release(p.ctx)
		p.ctx = 0
	}
	if p.tex != 0 {
		p.tex.Send(selRelease)
		p.tex = 0
	}
	p.w, p.h = 0, 0
}
