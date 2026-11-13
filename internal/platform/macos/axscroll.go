//go:build darwin && !ios

package macos

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// A scroll view is an AXScrollArea, and its bar on its axis an
// AXScrollBar, as AppKit's scroll views have: the view draws its own
// indicator, so AppKit has no scroller to report. The bar is an
// NSAccessibilityElement made when a client first asks for it
// (accessibilityVerticalScrollBar, accessibilityHorizontalScrollBar or
// the scroll area's children), whose value is where the view is
// scrolled to, 0 at the content's start and 1 at its end, posted with
// the scroll reports. Nothing is made or posted while no client asks.

// axBarWidth is how wide the bar's frame is along the view's edge.
const axBarWidth = 10

func axScrollMethods() []objc.MethodDef {
	bar := func(horizontal bool) func(objc.ID, objc.SEL) objc.ID {
		return func(self objc.ID, _ objc.SEL) objc.ID {
			v := views[self]
			if v == nil || v.sc == nil || v.sc.config.Horizontal != horizontal {
				return 0
			}
			return v.axScrollBar()
		}
	}
	return []objc.MethodDef{
		{Cmd: sel("accessibilityVerticalScrollBar"), Fn: bar(false)},
		{Cmd: sel("accessibilityHorizontalScrollBar"), Fn: bar(true)},
	}
}

// axScrollBar is v's bar element, made the first time.
func (v *view) axScrollBar() objc.ID {
	if v.axBar == 0 {
		el := objc.ID(apple.Class("NSAccessibilityElement")).Send(selNew)
		el.Send(selSetAccessibilityRole, apple.NSString("AXScrollBar"))
		el.Send(sel("setAccessibilityParent:"), v.native)
		// NSAccessibilityOrientationVertical is 1, Horizontal 2.
		orientation := 1
		if v.sc.config.Horizontal {
			orientation = 2
		}
		el.Send(sel("setAccessibilityOrientation:"), orientation)
		v.axBar = el
	}
	v.updateAXBar(false)
	return v.axBar
}

// axScrolled is where v is scrolled to along its axis: 0 at the
// content's start, 1 at its end, 0 when it does not scroll.
func (v *view) axScrolled() float64 {
	s := v.sc
	at, length, size := s.at.Y, s.content.Height, v.frame.Size.Height
	if s.config.Horizontal {
		at, length, size = s.at.X, s.content.Width, v.frame.Size.Width
	}
	if length <= size {
		return 0
	}
	return min(max(float64(at/(length-size)), 0), 1)
}

// updateAXBar brings the bar's value and frame up to date, posting the
// value's change when post is set.
func (v *view) updateAXBar(post bool) {
	el := v.axBar
	if el == 0 {
		return
	}
	el.Send(selSetAccessibilityValue, nsNumber(v.axScrolled()))
	w, h := float64(v.frame.Size.Width), float64(v.frame.Size.Height)
	frame := apple.Rect(w-axBarWidth, 0, axBarWidth, h)
	if v.sc.config.Horizontal {
		frame = apple.Rect(0, h-axBarWidth, w, axBarWidth)
	}
	el.Send(sel("setAccessibilityFrameInParentSpace:"), frame)
	if post {
		axPost()(el, apple.Global(appKitPath, "NSAccessibilityValueChangedNotification"), 0)
	}
}

// releaseAXBar lets a deleted scroll view's bar go.
func (v *view) releaseAXBar() {
	if v.axBar != 0 {
		v.axBar.Send(selRelease)
		v.axBar = 0
	}
}

// axScrollRole is a scroll view's role, when the app gives none.
func (v *view) axScrollRole() ([2]string, bool) {
	if v.kind == driver.KindScroll && v.props.Role == driver.RoleNone && v.props.Accessible != driver.AccessibleNo {
		return [2]string{"AXScrollArea", ""}, true
	}
	return [2]string{}, false
}
