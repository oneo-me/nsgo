//go:build ios

package ios

import (
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// A scroll view's edge effects are drawn as on macOS: a Core Animation
// backdrop layer over the content, with the tint and a hard edge's hairline
// over it (apple.ScrollEdge), in a holder that follows the offset. A soft
// edge's blur is Core Animation's variable blur. UIScrollEdgeEffect is not
// used, even where UIKit has it: its reach does not follow the bars, or a
// header that is not a navigation controller's, so it stops short of a tall
// bar's end. CABackdropLayer and CAFilter are private; UIKit's blurs cannot
// vary across a view.

// nativeEdges reports whether UIKit has scroll edge effects to hide.
var nativeEdges = sync.OnceValue(func() bool {
	return objc.Send[bool](objc.ID(objc.GetClass("UIScrollView")), sel("instancesRespondToSelector:"), sel("topEdgeEffect"))
})

// edgeEffectSels are the UIScrollEdgeEffects of each side.
var edgeEffectSels = [4]objc.SEL{sel("topEdgeEffect"), sel("rightEdgeEffect"), sel("bottomEdgeEffect"), sel("leftEdgeEffect")}

// edgeView is one side's effect: a holder that takes no touches, whose layer
// holds the backdrop and, over it, the tint and hairline.
type edgeView struct {
	holder, backdrop objc.ID
	// blur is the effect the backdrop's filter draws, soft or hard.
	blur  driver.EdgeEffect
	band  driver.Bounds
	paint apple.ScrollEdge
}

// placeEdges shows the scroll view's edge effects.
func (v *view) placeEdges() {
	s := v.sc
	for side := range s.edges {
		if nativeEdges() {
			v.native.Send(edgeEffectSels[side]).Send(selSetHidden, true)
		}
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
			ev.holder = objc.ID(objc.GetClass("UIView")).Send(selNew)
			ev.holder.Send(sel("setUserInteractionEnabled:"), false)
			// Above the content, which goes in the same view.
			ev.holder.Send(selLayer).Send(sel("setZPosition:"), 1000.0)
			ev.backdrop = objc.ID(objc.GetClass("CABackdropLayer")).Send(selNew)
			ev.holder.Send(selLayer).Send(sel("addSublayer:"), ev.backdrop)
			ev.backdrop.Send(selRelease)
			v.native.Send(selAddSubview, ev.holder)
			ev.holder.Send(selRelease)
		}
		ev.band = driver.BandRect(driver.Side(side), v.frame.Size, band)
		w, h := float64(ev.band.Size.Width), float64(ev.band.Size.Height)
		apple.WithoutAnimation(func() {
			ev.backdrop.Send(selSetFrame, apple.Rect(0, 0, w, h))
			if d := e.Drawn(); d != ev.blur {
				ev.backdrop.Send(sel("setFilters:"), apple.EdgeBlur(driver.Side(side), d))
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
	v.followEdges()
}

// followEdges keeps the effects over the part of the content that shows,
// as it scrolls.
func (v *view) followEdges() {
	s := v.sc
	at := objc.Send[apple.CGPoint](v.native, selContentOffset)
	apple.WithoutAnimation(func() {
		for i := range s.edges {
			if ev := &s.edges[i]; ev.holder != 0 {
				b := ev.band
				ev.holder.Send(selSetFrame, apple.Rect(at.X+float64(b.Origin.X), at.Y+float64(b.Origin.Y),
					float64(b.Size.Width), float64(b.Size.Height)))
			}
		}
	})
}

// hasEdges reports whether the scroll view draws effects of its own that
// follow its offset.
func (v *view) hasEdges() bool {
	for i := range v.sc.edges {
		if v.sc.edges[i].holder != 0 {
			return true
		}
	}
	return false
}

// release takes the side's effect away.
func (ev *edgeView) release() {
	ev.paint.Release()
	ev.holder.Send(sel("removeFromSuperview"))
	*ev = edgeView{}
}
