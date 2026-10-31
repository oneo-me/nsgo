//go:build darwin && !ios

package macos

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// A scroll view's edge effect is drawn as AppKit draws its own
// (NSScrollPocket): a Core Animation backdrop layer over the container,
// under the scroll bar, blurring what is under it, with the tint and a
// hard edge's hairline over it (apple.ScrollEdge). A soft edge's blur is
// Core Animation's variable blur, whose radius follows a mask image's
// alpha, from the side to nothing at the band's end; a hard edge's is an
// even Gaussian blur. CABackdropLayer and CAFilter are private, but
// NSVisualEffectView's blur cannot vary across it. AppKit's own edge
// effect, from macOS 26.1, reaches only NSScrollViews under a window's
// title bar accessories, which the app's scroll views are not.

const viewAbove = 1 // NSWindowAbove

// variableBlurSpread is how much wider Core Animation's variable blur
// spreads than its Gaussian blur, whose radius is the standard deviation,
// of the same radius: measured on macOS 27.
const variableBlurSpread = 2

// edgeView is one side's effect: a holder the mouse goes through, whose
// layer holds the backdrop and, over it, the tint and hairline.
type edgeView struct {
	holder, backdrop objc.ID
	// blur is the effect the backdrop's filter draws, soft or hard.
	blur  driver.EdgeEffect
	paint apple.ScrollEdge
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
			ev.backdrop = objc.ID(objc.GetClass("CABackdropLayer")).Send(selNew)
			ev.holder.Send(selLayer).Send(sel("addSublayer:"), ev.backdrop)
			ev.backdrop.Send(selRelease)
		}
		r := driver.BandRect(driver.Side(side), v.frame.Size, band)
		w, h := float64(r.Size.Width), float64(r.Size.Height)
		apple.WithoutAnimation(func() {
			ev.holder.Send(selSetFrame, apple.Rect(float64(r.Origin.X), float64(r.Origin.Y), w, h))
			ev.backdrop.Send(selSetFrame, apple.Rect(0, 0, w, h))
			if d := e.Drawn(); d != ev.blur {
				ev.backdrop.Send(sel("setFilters:"), edgeBlur(driver.Side(side), d))
				ev.blur = d
			}
			fresh := ev.paint.Tint == 0
			ev.paint.Paint(e, driver.Side(side), w, h)
			if fresh {
				layer := ev.holder.Send(selLayer)
				layer.Send(sel("addSublayer:"), ev.paint.Tint)
				layer.Send(sel("addSublayer:"), ev.paint.Line)
			}
		})
	}
}

// release takes the side's effect away.
func (ev *edgeView) release() {
	ev.paint.Release()
	ev.holder.Send(sel("removeFromSuperview"))
	*ev = edgeView{}
}

// edgeBlur returns the backdrop's filters for effect d at side s: a blur
// growing toward a soft side, or an even one.
func edgeBlur(s driver.Side, d driver.EdgeEffect) objc.ID {
	filter := objc.ID(objc.GetClass("CAFilter"))
	set := func(f objc.ID, key string, value objc.ID) {
		f.Send(sel("setValue:forKey:"), value, apple.NSString(key))
	}
	var f objc.ID
	if d == driver.EdgeEffectSoft {
		f = filter.Send(sel("filterWithType:"), apple.NSString("variableBlur"))
		set(f, "inputRadius", nsNumber(float64(driver.SoftEdgeBlur/variableBlurSpread)))
		set(f, "inputMaskImage", fadeImage(s))
	} else {
		f = filter.Send(sel("filterWithType:"), apple.NSString("gaussianBlur"))
		set(f, "inputRadius", nsNumber(float64(driver.HardEdgeBlur)))
	}
	// What is past the band's edges blurs in as if it went on, as
	// AppKit's edges have it, rather than as clear.
	set(f, "inputNormalizeEdges", objc.ID(objc.GetClass("NSNumber")).Send(sel("numberWithBool:"), true))
	return objc.ID(objc.GetClass("NSArray")).Send(sel("arrayWithObject:"), f)
}

// fadeImages are the soft edges' masks, by side, made on first use.
var fadeImages [4]objc.ID

// fadeImage returns the soft edge's mask at side s, which the variable
// blur stretches over the band.
func fadeImage(s driver.Side) objc.ID {
	if fadeImages[s] == 0 {
		fadeImages[s] = objc.ID(apple.FadeImage(s))
	}
	return fadeImages[s]
}
