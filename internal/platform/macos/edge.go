//go:build darwin && !ios

package macos

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// A scroll view's edge effect is AppKit's within-window blur over the
// container, under the scroll bar, fading in toward the side when soft
// (apple.ScrollEdge), with the tint and a hard edge's hairline over it.
// AppKit has a scroll edge effect of its own from macOS 26.1, but only
// for NSScrollViews under a window's title bar accessories, which the
// app's scroll views are not.

// materialHeaderView is the NSVisualEffectMaterial of in-line headers and
// footers, which content scrolls under.
const materialHeaderView = 10

const viewAbove = 1 // NSWindowAbove

// edgeView is one side's effect: a holder the mouse goes through, with
// the blur and, over it, the tint.
type edgeView struct {
	holder, blur, tint objc.ID
	paint              apple.ScrollEdge
}

// placeEdges shows the scroll view's edge effects, at its size.
func (v *view) placeEdges() {
	s := v.sc
	for side := range s.edges {
		e := &s.config.Edges[side]
		ev := &s.edges[side]
		band := e.Band()
		if band <= 0 {
			if ev.holder != 0 {
				ev.release()
			}
			continue
		}
		if ev.holder == 0 {
			ev.holder = objc.ID(classOverlay).Send(selNew)
			ev.holder.Send(selSetWantsLayer, true)
			v.native.Send(selAddSubviewPositioned, ev.holder, viewAbove, v.content)
			ev.holder.Send(selRelease)
			ev.blur = objc.ID(classOverlay).Send(selNew)
			// Layer-hosting: the view takes a layer of its own first.
			ev.blur.Send(sel("setLayer:"), objc.ID(objc.GetClass("CALayer")).Send(sel("layer")))
			ev.blur.Send(selSetWantsLayer, true)
			ev.blur.Send(sel("setLayerUsesCoreImageFilters:"), true)
			ev.holder.Send(selAddSubview, ev.blur)
			ev.blur.Send(selRelease)
			ev.tint = objc.ID(classOverlay).Send(selNew)
			ev.tint.Send(selSetWantsLayer, true)
			ev.holder.Send(selAddSubview, ev.tint)
			ev.tint.Send(selRelease)
		}
		r := driver.BandRect(driver.Side(side), v.frame.Size, band)
		w, h := float64(r.Size.Width), float64(r.Size.Height)
		apple.WithoutAnimation(func() {
			ev.holder.Send(selSetFrame, apple.Rect(float64(r.Origin.X), float64(r.Origin.Y), w, h))
			ev.blur.Send(selSetFrame, apple.Rect(0, 0, w, h))
			ev.tint.Send(selSetFrame, apple.Rect(0, 0, w, h))
			fresh := ev.paint.Mask == 0
			mask := ev.paint.Paint(e, driver.Side(side), w, h)
			if fresh {
				layer := ev.tint.Send(selLayer)
				layer.Send(sel("addSublayer:"), ev.paint.Tint)
				layer.Send(sel("addSublayer:"), ev.paint.Line)
			}
			// TEMP: private CABackdropLayer + variableBlur feasibility test.
			bd := objc.ID(objc.GetClass("CABackdropLayer")).Send(sel("layer"))
			bd.Send(selSetFrame, apple.Rect(0, 0, w, h))
			f := objc.ID(objc.GetClass("CAFilter")).Send(sel("filterWithType:"), apple.NSString("variableBlur"))
			f.Send(sel("setValue:forKey:"), objc.ID(objc.GetClass("NSNumber")).Send(sel("numberWithDouble:"), 20.0), apple.NSString("inputRadius"))
			cg := apple.FadeImage(driver.Side(side))
			f.Send(sel("setValue:forKey:"), objc.ID(cg), apple.NSString("inputMaskImage"))
			f.Send(sel("setValue:forKey:"), objc.ID(objc.GetClass("NSNumber")).Send(sel("numberWithBool:"), true), apple.NSString("inputNormalizeEdges"))
			bd.Send(sel("setFilters:"), objc.ID(objc.GetClass("NSArray")).Send(sel("arrayWithObject:"), f))
			ev.blur.Send(selLayer).Send(sel("addSublayer:"), bd)
			_ = mask
		})
	}
}

// release takes the side's effect away.
func (ev *edgeView) release() {
	ev.paint.Release()
	ev.holder.Send(sel("removeFromSuperview"))
	*ev = edgeView{}
}

// fadeImages are the soft edges' masks, by side, made on first use.
var fadeImages [4]objc.ID

// fadeImage returns the soft edge's mask at side s, which AppKit
// stretches over the blur.
func fadeImage(s driver.Side) objc.ID {
	if fadeImages[s] == 0 {
		cg := apple.FadeImage(s)
		size := apple.CGSize{Width: 1, Height: 64}
		if s == driver.SideLeft || s == driver.SideRight {
			size = apple.CGSize{Width: 64, Height: 1}
		}
		img := objc.ID(objc.GetClass("NSImage")).Send(selAlloc).Send(sel("initWithCGImage:size:"), cg, size)
		apple.Release(cg)
		const resizingStretch = 1 // NSImageResizingModeStretch
		img.Send(sel("setResizingMode:"), resizingStretch)
		fadeImages[s] = img
	}
	return fadeImages[s]
}

// edgeFilters are a side's Core Image background filters: a blur whose
// radius grows toward a soft side along a gradient mask, or an even one.
func edgeFilters(e *driver.ScrollEdge, side driver.Side, w, h float64, soft bool) objc.ID {
	ci := objc.ID(objc.GetClass("CIFilter"))
	num := func(v float64) objc.ID { return objc.ID(objc.GetClass("NSNumber")).Send(sel("numberWithDouble:"), v) }
	list := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	if !soft {
		f := ci.Send(sel("filterWithName:"), apple.NSString("CIGaussianBlur"))
		f.Send(sel("setValue:forKey:"), num(float64(driver.HardEdgeBlur)), apple.NSString("inputRadius"))
		list.Send(sel("addObject:"), f)
		return list
	}
	// The mask's points, in Core Image's space: y grows up from the
	// band's bottom.
	x0, y0, x1, y1 := driver.EdgeLine(side)
	vec := objc.ID(objc.GetClass("CIVector"))
	color := objc.ID(objc.GetClass("CIColor"))
	g := ci.Send(sel("filterWithName:"), apple.NSString("CILinearGradient"))
	g.Send(sel("setValue:forKey:"), vec.Send(sel("vectorWithX:Y:"), float64(x0)*w, (1-float64(y0))*h), apple.NSString("inputPoint0"))
	g.Send(sel("setValue:forKey:"), vec.Send(sel("vectorWithX:Y:"), float64(x1)*w, (1-float64(y1))*h), apple.NSString("inputPoint1"))
	g.Send(sel("setValue:forKey:"), color.Send(sel("colorWithRed:green:blue:"), 1.0, 1.0, 1.0), apple.NSString("inputColor0"))
	g.Send(sel("setValue:forKey:"), color.Send(sel("colorWithRed:green:blue:"), 0.0, 0.0, 0.0), apple.NSString("inputColor1"))
	f := ci.Send(sel("filterWithName:"), apple.NSString("CIMaskedVariableBlur"))
	f.Send(sel("setValue:forKey:"), g.Send(sel("outputImage")), apple.NSString("inputMask"))
	f.Send(sel("setValue:forKey:"), num(float64(driver.SoftEdgeBlur)), apple.NSString("inputRadius"))
	list.Send(sel("addObject:"), f)
	return list
}
