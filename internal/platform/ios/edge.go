//go:build ios

package ios

import (
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// A scroll view's edge effects are UIKit's own from iOS 26: the side's
// UIScrollEdgeEffect in the style the app asks for, shaped under its bars,
// each of which gets a UIScrollEdgeElementContainerInteraction once the
// batch made it. Before 26 a side's effect is a system blur over the
// content, fading in toward the side when soft (apple.ScrollEdge), in a
// holder that follows the offset over the content.

// nativeEdges reports whether UIKit has scroll edge effects.
var nativeEdges = sync.OnceValue(func() bool {
	return objc.Send[bool](objc.ID(objc.GetClass("UIScrollView")), sel("instancesRespondToSelector:"), sel("topEdgeEffect"))
})

// The UIScrollEdgeEffect of each side, and its UIRectEdge.
var (
	edgeEffectSels = [4]objc.SEL{sel("topEdgeEffect"), sel("rightEdgeEffect"), sel("bottomEdgeEffect"), sel("leftEdgeEffect")}
	rectEdges      = [4]uint{1, 8, 4, 2}
)

// edgeView is one side's effect before iOS 26: a holder that takes no
// touches, with the blur and, over it, the tint.
type edgeView struct {
	holder, blur, mask, tint objc.ID
	band                     driver.Bounds
	paint                    apple.ScrollEdge
}

// edgeBar is a bar's interaction with a side of a scroll view, and the
// element it shapes the effect by.
type edgeBar struct {
	bar                  *view
	interaction, element objc.ID
}

// placeEdges shows the scroll view's edge effects.
func (v *view) placeEdges() {
	s := v.sc
	if nativeEdges() {
		styles := objc.ID(objc.GetClass("UIScrollEdgeEffectStyle"))
		for side, e := range s.config.Edges {
			effect := v.native.Send(edgeEffectSels[side])
			effect.Send(selSetHidden, e.Effect == driver.EdgeEffectHidden)
			style := styles.Send(sel("automaticStyle"))
			switch e.Effect {
			case driver.EdgeEffectSoft:
				style = styles.Send(sel("softStyle"))
			case driver.EdgeEffectHard:
				style = styles.Send(sel("hardStyle"))
			}
			effect.Send(sel("setStyle:"), style)
		}
		// Bars a batch makes after the config get theirs at its end.
		v.surface.edgesDue = append(v.surface.edgesDue, v)
		return
	}
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
			ev.holder = objc.ID(objc.GetClass("UIView")).Send(selNew)
			ev.holder.Send(sel("setUserInteractionEnabled:"), false)
			// Above the content, which goes in the same view.
			ev.holder.Send(selLayer).Send(sel("setZPosition:"), 1000.0)
			effect := objc.ID(objc.GetClass("UIBlurEffect")).Send(sel("effectWithStyle:"), blurStyle(driver.BackdropThin))
			ev.blur = objc.ID(objc.GetClass("UIVisualEffectView")).Send(selAlloc).Send(sel("initWithEffect:"), effect)
			ev.holder.Send(selAddSubview, ev.blur)
			ev.blur.Send(selRelease)
			ev.mask = objc.ID(objc.GetClass("UIView")).Send(selNew)
			ev.tint = objc.ID(objc.GetClass("UIView")).Send(selNew)
			ev.holder.Send(selAddSubview, ev.tint)
			ev.tint.Send(selRelease)
			v.native.Send(selAddSubview, ev.holder)
			ev.holder.Send(selRelease)
		}
		ev.band = driver.BandRect(driver.Side(side), v.frame.Size, band)
		w, h := float64(ev.band.Size.Width), float64(ev.band.Size.Height)
		apple.WithoutAnimation(func() {
			ev.blur.Send(selSetFrame, apple.Rect(0, 0, w, h))
			ev.tint.Send(selSetFrame, apple.Rect(0, 0, w, h))
			ev.mask.Send(selSetFrame, apple.Rect(0, 0, w, h))
			fresh := ev.paint.Mask == 0
			masked := ev.paint.Paint(e, driver.Side(side), w, h)
			if fresh {
				ev.mask.Send(selLayer).Send(sel("addSublayer:"), ev.paint.Mask)
				ev.tint.Send(selLayer).Send(sel("addSublayer:"), ev.paint.Tint)
				ev.tint.Send(selLayer).Send(sel("addSublayer:"), ev.paint.Line)
			}
			// UIKit masks a blur only by a mask view.
			if masked {
				ev.blur.Send(sel("setMaskView:"), ev.mask)
			} else {
				ev.blur.Send(sel("setMaskView:"), objc.ID(0))
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
	ev.mask.Send(selRelease)
	ev.holder.Send(sel("removeFromSuperview"))
	*ev = edgeView{}
}

// attachEdgeBars gives the bars of the scroll views the batch configured
// their interaction with their side, and takes it from those that are no
// longer bars there.
func (s *surface) attachEdgeBars() {
	for _, v := range s.edgesDue {
		if v.sc == nil || int(v.id) >= len(s.views) || s.views[v.id] != v {
			continue
		}
		for side, e := range v.sc.config.Edges {
			had := v.sc.bars[side]
			kept := map[driver.NodeID]edgeBar{}
			for _, id := range e.Bars {
				if int(id) >= len(s.views) || s.views[id] == nil {
					continue
				}
				bar := s.views[id]
				if b, ok := had[id]; ok && b.bar == bar {
					kept[id] = b
					delete(had, id)
					continue
				}
				in := objc.ID(objc.GetClass("UIScrollEdgeElementContainerInteraction")).Send(selNew)
				in.Send(sel("setScrollView:"), v.native)
				in.Send(sel("setEdge:"), rectEdges[side])
				bar.native.Send(sel("addInteraction:"), in)
				in.Send(selRelease)
				// UIKit shapes the effect by the labels and images in the
				// bar, which the app's views draw into layers: an empty
				// label as large as the bar stands for them.
				el := objc.ID(objc.GetClass("UILabel")).Send(selNew)
				el.Send(selSetFrame, objc.Send[apple.CGRect](bar.native, selBounds))
				const flexible = 2 | 16 // width and height
				el.Send(sel("setAutoresizingMask:"), flexible)
				el.Send(sel("setUserInteractionEnabled:"), false)
				el.Send(sel("setIsAccessibilityElement:"), false)
				bar.native.Send(sel("insertSubview:atIndex:"), el, 0)
				el.Send(selRelease)
				kept[id] = edgeBar{bar: bar, interaction: in, element: el}
			}
			for id, b := range had {
				// A bar still in the tree is no longer one.
				if int(id) < len(s.views) && s.views[id] == b.bar {
					b.bar.native.Send(sel("removeInteraction:"), b.interaction)
					b.element.Send(sel("removeFromSuperview"))
				}
			}
			v.sc.bars[side] = kept
		}
	}
	clear(s.edgesDue)
	s.edgesDue = s.edgesDue[:0]
}
