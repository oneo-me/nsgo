//go:build ios

package ios

import (
	"math"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// Nested scroll views: a drag scrolls the innermost scroll view that can
// move its content the way the finger goes. One at its edge that way lets
// its pan fail, so the scroll views around it take the drag, and those
// wait for the pans of the scroll views inside them to fail, so two never
// scroll together. A drag keeps the scroll view it began with.

var (
	selPanRecognizer = sel("panGestureRecognizer")
	selTranslation   = sel("translationInView:")
	selVelocity      = sel("velocityInView:")
	selIsKindOf      = sel("isKindOfClass:")
	selIsDescendant  = sel("isDescendantOfView:")
)

func scrollMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{Cmd: sel("gestureRecognizerShouldBegin:"), Fn: scrollPanShouldBegin},
		{Cmd: sel("gestureRecognizer:shouldRequireFailureOfGestureRecognizer:"), Fn: scrollPanWaits},
	}
}

// scrollPanShouldBegin lets a scroll view's pan begin unless the drag
// goes past its edge and a scroll view around it can move that way.
func scrollPanShouldBegin(self objc.ID, cmd objc.SEL, r objc.ID) bool {
	if r != self.Send(selPanRecognizer) {
		return apple.SendSuper[bool](self, cmd, r)
	}
	d := objc.Send[apple.CGPoint](r, selTranslation, self)
	if d == (apple.CGPoint{}) {
		d = objc.Send[apple.CGPoint](r, selVelocity, self)
	}
	if d != (apple.CGPoint{}) && !canScroll(self, d) {
		scrollClass := apple.Class("UIScrollView")
		for p := self.Send(selSuperview); p != 0; p = p.Send(selSuperview) {
			if objc.Send[bool](p, selIsKindOf, scrollClass) && canScroll(p, d) {
				return false
			}
		}
	}
	return apple.SendSuper[bool](self, cmd, r)
}

// scrollPanWaits makes a scroll view's pan wait for the pans of the
// scroll views inside it, and for the swipe back of the screen around it,
// which fails at once away from the screen's edge: an edge drag goes back
// instead of scrolling.
func scrollPanWaits(self objc.ID, cmd objc.SEL, r, other objc.ID) bool {
	if r == self.Send(selPanRecognizer) {
		inner := other.Send(selView)
		if inner != 0 && inner != self && objc.Send[bool](inner, selIsKindOf, apple.Class("UIScrollView")) &&
			inner.Send(selPanRecognizer) == other && objc.Send[bool](inner, selIsDescendant, self) {
			return true
		}
		if inner != 0 && objc.Send[bool](other, selIsKindOf, apple.Class("UIScreenEdgePanGestureRecognizer")) &&
			objc.Send[bool](self, selIsDescendant, inner) {
			return true
		}
	}
	if objc.Send[bool](objc.ID(apple.Class("UIScrollView")), sel("instancesRespondToSelector:"), cmd) {
		return apple.SendSuper[bool](self, cmd, r, other)
	}
	return false
}

// canScroll reports whether scroll view s can move its content the way a
// finger dragging by d goes, along d's main axis.
func canScroll(s objc.ID, d apple.CGPoint) bool {
	const slack = 0.5 // offsets land on fractions of a point
	at := objc.Send[apple.CGPoint](s, selContentOffset)
	size := objc.Send[apple.CGSize](s, sel("contentSize"))
	in := objc.Send[edgeInsets](s, sel("adjustedContentInset"))
	seen := objc.Send[apple.CGRect](s, selBounds).Size
	if math.Abs(d.X) > math.Abs(d.Y) {
		maxX := size.Width + in.Right - seen.Width
		return d.X > 0 && at.X > -in.Left+slack || d.X < 0 && at.X < maxX-slack
	}
	maxY := size.Height + in.Bottom - seen.Height
	return d.Y > 0 && at.Y > -in.Top+slack || d.Y < 0 && at.Y < maxY-slack
}
