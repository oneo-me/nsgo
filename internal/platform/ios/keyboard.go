//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// The focused input is kept clear of the software keyboard by the
// innermost vertical scroll view around it, as NSH's: what the keyboard
// covers of the scroll view adds to its bottom inset, and the view
// scrolls the input above the keyboard, on the keyboard's own duration.

// keyboardMargin is how far above the keyboard an input is kept.
const keyboardMargin = 8

// keepFocusedClear follows the keyboard and the focus.
func (s *surface) keepFocusedClear() {
	var field *view
	if s.keyboard > 0 {
		for _, v := range s.views {
			if v != nil && v.input != nil && v.input.focused {
				field = v
				break
			}
		}
	}
	var scroll *view
	if field != nil {
		for n := field.native.Send(selSuperview); n != 0 && n != s.native; n = n.Send(selSuperview) {
			if v := views[n]; v != nil && v.native == n && v.sc != nil && !v.sc.config.Horizontal {
				scroll = v
				break
			}
		}
	}
	if old := s.keyboardScroll; old != nil && old != scroll && s.views[old.id] == old {
		old.sc.keyboard = 0
		old.setInset(old.sc.config.Behavior.ContentInset)
	}
	s.keyboardScroll = scroll
	if scroll == nil {
		return
	}
	bounds := objc.Send[apple.CGRect](scroll.native, selBounds)
	inSurface := objc.Send[apple.CGRect](s.native, sel("convertRect:fromView:"), bounds, scroll.native)
	top := objc.Send[apple.CGRect](s.native, selBounds).Size.Height - float64(s.keyboard)
	scroll.sc.keyboard = driver.Points(max(inSurface.Origin.Y+inSurface.Size.Height-top, 0))
	scroll.setInset(scroll.sc.config.Behavior.ContentInset)
	r := objc.Send[apple.CGRect](scroll.native, sel("convertRect:fromView:"), objc.Send[apple.CGRect](field.native, selBounds), field.native)
	r.Size.Height += keyboardMargin
	animations := objc.NewBlock(func(objc.Block) {
		scroll.native.Send(sel("scrollRectToVisible:animated:"), r, false)
	})
	objc.ID(apple.Class("UIView")).Send(sel("animateWithDuration:animations:"), s.keyboardDuration.Seconds(), animations)
	animations.Release()
}
