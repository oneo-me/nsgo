//go:build ios

package ios

import (
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// How scroll views scroll, as NSH's iOS backend does: UIScrollView's own
// paging, deceleration, bounces, insets, keyboard dismissal and
// UIRefreshControl, which cannot give way to an indicator of the app's
// own: then the backend detects the pull as a drag ends and keeps the
// indicator's room with the top inset. Snapping is decided by
// core.ScrollBehavior.SnapTarget as
// a drag ends; the drag and momentum events from the delegate; and a tap
// recognizer that dismisses the keyboard as KeyboardShouldPersistTaps
// says. A zoomable view moves its children into a zoom view UIKit
// scales; its bounds stay the unzoomed content size, so the core's frames
// stay valid.

// scrollState is what a scroll view keeps beside its UIScrollView.
type scrollState struct {
	// edges are its sides' effects before iOS 26, and bars the
	// interactions of the views over its sides from iOS 26.
	edges   [4]edgeView
	bars    [4]map[driver.NodeID]edgeBar
	config  driver.ScrollConfig
	content driver.Size // unzoomed
	zoom    objc.ID     // the zoom view, while it zooms
	refresh objc.ID     // the UIRefreshControl, while refreshable
	tap     objc.ID     // the keyboard's tap recognizer
	// room is the top inset kept for the app's own refresh indicator
	// while it refreshes.
	room driver.Points
	// keyboard is what the software keyboard covers of the view, added to
	// its bottom inset while an input in it has the focus (keyboard.go).
	keyboard driver.Points
	// applied counts the core's ItemScrollTo and ItemScrollBy items,
	// reported with each offset; sticky are its sticky cells.
	applied uint32
	sticky  []driver.StickyCell
}

// scrollBy moves the offset by d unclamped, which UIKit's drag and glide
// carry on from: the core keeps the visible content in place as rows
// above it change size.
func (v *view) scrollBy(d driver.Point) {
	o := objc.Send[apple.CGPoint](v.native, selContentOffset)
	v.native.Send(sel("setContentOffset:"), apple.CGPoint{X: o.X + float64(d.X), Y: o.Y + float64(d.Y)})
}

// pinSticky moves the scroll view's sticky cells for its offset, as
// NSH's StickyCells: each by clamp(offset - start, 0, end - start) from
// its frame.
func (v *view) pinSticky() {
	o := objc.Send[apple.CGPoint](v.native, selContentOffset)
	at := driver.Points(o.Y)
	if v.sc.config.Horizontal {
		at = driver.Points(o.X)
	}
	for _, c := range v.sc.sticky {
		if cv := v.surface.viewOf(c.Cell); cv != nil {
			cv.stick(v.surface.animator.StickyShift(c, at), v.sc.config.Horizontal)
		}
	}
}

// setSticky replaces the scroll view's sticky cells; cells no longer
// sticky go back to their frames.
func (v *view) setSticky(cells []driver.StickyCell) {
	for _, c := range v.sc.sticky {
		if cv := v.surface.viewOf(c.Cell); cv != nil {
			cv.stick(0, false)
		}
	}
	v.sc.sticky = cells
	bound := false
	for _, c := range cells {
		bound = bound || c.Node != 0
	}
	if bound {
		if v.surface.stickyBound == nil {
			v.surface.stickyBound = map[*view]bool{}
		}
		v.surface.stickyBound[v] = true
	} else {
		delete(v.surface.stickyBound, v)
	}
	v.pinSticky()
}

// repinBound pins again the sticky cells whose insets follow animated
// values, which may have moved.
func (s *surface) repinBound() {
	for v := range s.stickyBound {
		if v.sc != nil && s.views[v.id] == v {
			v.pinSticky()
		} else {
			delete(s.stickyBound, v)
		}
	}
}

// stick moves a sticky cell d along its list from its frame.
func (v *view) stick(d driver.Points, horizontal bool) {
	shift := driver.Point{Y: d}
	if horizontal {
		shift = driver.Point{X: d}
	}
	if shift != v.shift {
		v.shift = shift
		if v.framed {
			v.place(v.frame)
		}
	}
}

var (
	selRefreshed       = sel("nsgoRefreshed:")
	selDismissKeyboard = sel("nsgoDismissKeyboard:")
	selZoomScale       = sel("zoomScale")
)

// behaviorMethods are the scroll view's delegate methods and actions for
// its behavior.
func behaviorMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{Cmd: sel("scrollViewWillBeginDragging:"), Fn: scrollWillDrag},
		{Cmd: sel("scrollViewWillEndDragging:withVelocity:targetContentOffset:"), Fn: scrollWillEndDrag},
		{Cmd: sel("scrollViewDidEndDragging:willDecelerate:"), Fn: scrollDidEndDrag},
		{Cmd: sel("scrollViewDidEndDecelerating:"), Fn: scrollDidEndGlide},
		{Cmd: sel("viewForZoomingInScrollView:"), Fn: zoomView},
		{Cmd: sel("scrollViewDidZoom:"), Fn: scrollDidZoom},
		{Cmd: selRefreshed, Fn: scrollRefreshed},
		{Cmd: selDismissKeyboard, Fn: dismissKeyboardTap},
		{Cmd: sel("gestureRecognizer:shouldReceiveTouch:"), Fn: scrollTapReceives},
	}
}

// newScroll sets up scroll view v: its own delegate, no automatic insets
// (the app's ContentInset is the inset) and the keyboard's tap.
func newScroll(v *view) {
	v.native.Send(selSetDeleg, v.native)
	v.native.Send(sel("setContentInsetAdjustmentBehavior:"), 2) // never
	v.sc = &scrollState{}
	tap := objc.ID(objc.GetClass("UITapGestureRecognizer")).Send(selAlloc).
		Send(sel("initWithTarget:action:"), v.native, selDismissKeyboard)
	tap.Send(sel("setCancelsTouchesInView:"), false)
	tap.Send(selSetDeleg, v.native)
	v.native.Send(sel("addGestureRecognizer:"), tap)
	tap.Send(selRelease)
	v.sc.tap = tap
}

// children returns where v's children go: its zoom view while it zooms.
func (v *view) children() objc.ID {
	if v.sc != nil && v.sc.zoom != 0 {
		return v.sc.zoom
	}
	return v.native
}

// configureScroll applies a scroll view's configuration.
func (v *view) configureScroll(c *driver.ScrollConfig) {
	s := v.sc
	old := s.config
	s.config = *c
	b := &c.Behavior
	n := v.native
	n.Send(sel("setShowsVerticalScrollIndicator:"), !c.Horizontal && !c.HideIndicator)
	n.Send(sel("setShowsHorizontalScrollIndicator:"), (c.Horizontal || c.Zooms()) && !c.HideIndicator)
	// Native paging pages by the viewport; other snapping is decided as a
	// drag ends.
	n.Send(sel("setPagingEnabled:"), b.Paging && b.SnapToInterval <= 0 && len(b.SnapToOffsets) == 0)
	n.Send(sel("setDecelerationRate:"), float64(b.Deceleration()))
	n.Send(sel("setScrollEnabled:"), !b.NoScroll)
	n.Send(sel("setBounces:"), !b.NoBounces)
	n.Send(sel("setAlwaysBounceVertical:"), !b.NoBounces && !c.Horizontal)
	n.Send(sel("setAlwaysBounceHorizontal:"), !b.NoBounces && c.Horizontal)
	if in, was := b.ContentInset, old.Behavior.ContentInset; in != was {
		// A view resting at its start stays there, showing the inset.
		at := objc.Send[apple.CGPoint](n, selContentOffset)
		v.setInset(in)
		if at.Y <= float64(-was.Top-s.room)+0.5 {
			at.Y = float64(-in.Top - s.room)
		}
		if at.X <= float64(-was.Left)+0.5 {
			at.X = float64(-in.Left)
		}
		n.Send(sel("setContentOffset:"), at)
	}
	n.Send(sel("setKeyboardDismissMode:"), int(b.KeyboardDismissMode))
	own := b.RefreshIndicator > 0
	v.setRefreshable(c.Refreshable && !c.Horizontal && !own, c.Refreshing)
	v.setRoom(c.Refreshing && !c.Horizontal && own, false)
	// Zooming.
	n.Send(sel("setMinimumZoomScale:"), float64(c.MinZoomScale))
	n.Send(sel("setMaximumZoomScale:"), float64(c.MaxZoomScale))
	n.Send(sel("setBouncesZoom:"), !c.NoBouncesZoom)
	v.setZoomable(c.Zooms())
	if pinch := n.Send(sel("pinchGestureRecognizer")); pinch != 0 {
		pinch.Send(sel("setEnabled:"), !c.NoPinchZoom)
	}
	if s.zoom != 0 {
		if c.ZoomScale != old.ZoomScale {
			n.Send(sel("setZoomScale:animated:"), float64(c.ZoomScale), true)
		}
		v.layoutZoom()
	}
	v.placeEdges()
}

// setInset sets the view's content inset to in, with the refresh room
// above it.
func (v *view) setInset(in driver.Edges) {
	v.native.Send(sel("setContentInset:"), edgeInsets{Top: float64(in.Top + v.sc.room), Left: float64(in.Left),
		Bottom: float64(in.Bottom + v.sc.keyboard), Right: float64(in.Right)})
}

// setRoom keeps the room of the app's own refresh indicator above the
// content while on, scrolling to it from the top, or gives it back; a
// finger still on the view holds it where it is, unless it just lifted.
func (v *view) setRoom(on, lifted bool) {
	s := v.sc
	room := driver.Points(0)
	if on {
		room = s.config.Behavior.RefreshInset()
	}
	if room == s.room {
		return
	}
	n := v.native
	start := -objc.Send[edgeInsets](n, sel("adjustedContentInset")).Top + float64(s.room)
	at := objc.Send[apple.CGPoint](n, selContentOffset)
	s.room = room
	v.setInset(s.config.Behavior.ContentInset)
	if !lifted && objc.Send[bool](n, sel("isDragging")) {
		return // the finger holds it
	}
	switch {
	case room > 0 && at.Y <= start+0.5:
		// UIKit's bounce already aims at the old inset as a drag ends, so
		// the room is scrolled to on the next turn.
		apple.OnMain(func() {
			if v.sc != nil && v.sc.room > 0 {
				n.Send(sel("setContentOffset:animated:"), apple.CGPoint{X: at.X, Y: start - float64(v.sc.room)}, true)
			}
		})
	case room == 0 && at.Y < start:
		n.Send(sel("setContentOffset:animated:"), apple.CGPoint{X: at.X, Y: start}, true)
	}
}

// pulled is how far the view is pulled past its top, its refresh room
// included.
func (v *view) pulled() driver.Points {
	if v.sc.config.Horizontal {
		return 0
	}
	n := v.native
	start := -objc.Send[edgeInsets](n, sel("adjustedContentInset")).Top + float64(v.sc.room)
	return driver.Points(max(start-objc.Send[apple.CGPoint](n, selContentOffset).Y, 0))
}

// pulledToRefresh starts a refresh of the app's own indicator when a drag
// ends pulled far enough past the top: the room shows right away, and the
// next config says whether the app keeps it.
func (v *view) pulledToRefresh() {
	c := &v.sc.config
	if !c.Refreshable || c.Horizontal || c.Behavior.RefreshIndicator <= 0 || v.sc.room > 0 ||
		v.pulled() < c.Behavior.RefreshThreshold() {
		return
	}
	v.scrollEvent(driver.EventRefresh)
	v.setRoom(true, true)
}

// setRefreshable gives the view a UIRefreshControl, or takes it away,
// and has it show refreshing.
func (v *view) setRefreshable(on, refreshing bool) {
	s := v.sc
	switch {
	case on && s.refresh == 0:
		r := objc.ID(objc.GetClass("UIRefreshControl")).Send(selNew)
		const valueChanged = 1 << 12
		r.Send(sel("addTarget:action:forControlEvents:"), v.native, selRefreshed, uint(valueChanged))
		v.native.Send(sel("setRefreshControl:"), r)
		r.Send(selRelease)
		s.refresh = r
	case !on && s.refresh != 0:
		v.native.Send(sel("setRefreshControl:"), objc.ID(0))
		s.refresh = 0
	}
	if s.refresh == 0 {
		return
	}
	if now := objc.Send[bool](s.refresh, sel("isRefreshing")); refreshing && !now {
		s.refresh.Send(sel("beginRefreshing"))
	} else if !refreshing && now {
		s.refresh.Send(sel("endRefreshing"))
	}
}

// setContent sets the unzoomed size of the view's content.
func (v *view) setContent(size driver.Size) {
	v.sc.content = size
	if v.sc.zoom != 0 {
		v.layoutZoom()
		return
	}
	v.native.Send(selSetContentSize, apple.CGSize{Width: float64(size.Width), Height: float64(size.Height)})
}

// setZoomable gives the view a zoom view, moving its children into it,
// or takes it away, moving them back unzoomed.
func (v *view) setZoomable(on bool) {
	s := v.sc
	switch {
	case on && s.zoom == 0:
		z := objc.ID(objc.GetClass("UIView")).Send(selNew)
		for _, child := range subviews(v.native) {
			if views[child] != nil {
				z.Send(selAddSubview, child)
			}
		}
		v.native.Send(sel("insertSubview:atIndex:"), z, 0)
		z.Send(selRelease)
		s.zoom = z
		v.layoutZoom()
	case !on && s.zoom != 0:
		v.native.Send(sel("setZoomScale:"), 1.0)
		z := s.zoom
		s.zoom = 0
		for _, child := range subviews(z) {
			v.native.Send(selInsertBelow, child, z)
		}
		z.Send(selRemoveFromSuperview)
		v.setContent(s.content)
	}
}

// layoutZoom lays the zoom view out for the content at the current zoom:
// its bounds the unzoomed size, centered while smaller than the viewport
// when CenterContent.
func (v *view) layoutZoom() {
	s := v.sc
	z := objc.Send[float64](v.native, selZoomScale)
	w, h := float64(s.content.Width), float64(s.content.Height)
	// Bounds and center, never the frame: the view has a zoom transform.
	s.zoom.Send(sel("setBounds:"), apple.Rect(0, 0, w, h))
	seen := objc.Send[apple.CGRect](v.native, selBounds).Size
	in := s.config.Behavior.ContentInset
	var dx, dy float64
	if s.config.CenterContent {
		dx = max(seen.Width-float64(in.Left+in.Right)-w*z, 0)
		dy = max(seen.Height-float64(in.Top+in.Bottom)-h*z, 0)
	}
	s.zoom.Send(sel("setCenter:"), apple.CGPoint{X: (w*z + dx) / 2, Y: (h*z + dy) / 2})
	v.native.Send(selSetContentSize, apple.CGSize{Width: w * z, Height: h * z})
}

// zoomTo zooms the view to r.Scale around content point r.Center, or the
// middle of what shows.
func (v *view) zoomTo(r *driver.ZoomRequest, animated bool) {
	if v.sc == nil || v.sc.zoom == 0 {
		return
	}
	scale := min(max(float64(r.Scale), float64(v.sc.config.MinZoomScale)), float64(v.sc.config.MaxZoomScale))
	z := objc.Send[float64](v.native, selZoomScale)
	b := objc.Send[apple.CGRect](v.native, selBounds)
	cx, cy := (b.Origin.X+b.Size.Width/2)/z, (b.Origin.Y+b.Size.Height/2)/z
	if r.Center != nil {
		cx, cy = float64(r.Center.X), float64(r.Center.Y)
	}
	w, h := b.Size.Width/scale, b.Size.Height/scale
	v.native.Send(sel("zoomToRect:animated:"), apple.Rect(cx-w/2, cy-h/2, w, h), animated)
}

// subviews returns view's subviews.
func subviews(view objc.ID) []objc.ID {
	arr := view.Send(sel("subviews"))
	n := objc.Send[uint](arr, sel("count"))
	out := make([]objc.ID, 0, n)
	for i := range n {
		out = append(out, arr.Send(sel("objectAtIndex:"), i))
	}
	return out
}

// scrollEvent reports kind on scroll view v at its offset and zoom.
func (v *view) scrollEvent(kind driver.EventKind) {
	if kind == driver.EventScrollDragEnd || kind == driver.EventMomentumEnd {
		// At rest, unless a glide follows, which scrolls on.
		s := v.surface
		s.animator.Settle(v.id)
		if s.animator.Running() {
			s.startTicking()
		}
	}
	o := objc.Send[apple.CGPoint](v.native, selContentOffset)
	e := driver.Event{Kind: kind, Point: driver.Point{X: driver.Points(o.X), Y: driver.Points(o.Y)}, Count: v.sc.applied}
	if v.sc.zoom != 0 {
		e.Value = float32(objc.Send[float64](v.native, selZoomScale))
	}
	v.report(e)
}

func scrollView(self objc.ID) *view {
	if v := views[self]; v != nil && v.sc != nil {
		return v
	}
	return nil
}

func scrollWillDrag(self objc.ID, _ objc.SEL, _ objc.ID) {
	if v := scrollView(self); v != nil {
		v.surface.touchesTaken()
		// Caught while gliding: the glide ended.
		if objc.Send[bool](self, sel("isDecelerating")) {
			v.scrollEvent(driver.EventMomentumEnd)
		}
		v.scrollEvent(driver.EventScrollDragStart)
	}
}

// scrollWillEndDrag snaps a fling to SnapToInterval, SnapToOffsets or
// pages; UIKit's velocity is in points per millisecond.
func scrollWillEndDrag(self objc.ID, _ objc.SEL, _ objc.ID, velocity apple.CGPoint, target unsafe.Pointer) {
	if g := handoffs[self]; g != nil && g.consuming && target != nil {
		// The drag moved the view around it: no glide from the top.
		to := (*apple.CGPoint)(target)
		to.Y = -objc.Send[edgeInsets](self, sel("adjustedContentInset")).Top
		return
	}
	v := scrollView(self)
	if v == nil || !v.sc.config.Behavior.Snaps() || target == nil {
		return
	}
	c := &v.sc.config
	b := &c.Behavior
	if b.Paging && b.SnapToInterval <= 0 && len(b.SnapToOffsets) == 0 {
		return // UIKit pages itself
	}
	to := (*apple.CGPoint)(target)
	at := objc.Send[apple.CGPoint](self, selContentOffset)
	seen := objc.Send[apple.CGRect](self, selBounds).Size
	size := objc.Send[apple.CGSize](self, sel("contentSize"))
	pos, projected, speed, viewport, length := at.Y, to.Y, velocity.Y, seen.Height, size.Height+float64(b.ContentInset.Bottom)
	if c.Horizontal {
		pos, projected, speed, viewport, length = at.X, to.X, velocity.X, seen.Width, size.Width+float64(b.ContentInset.Right)
	}
	snap, ok := b.SnapTarget(driver.Points(pos), driver.Points(projected), float32(speed), driver.Points(viewport), driver.Points(length))
	if !ok {
		return
	}
	if c.Horizontal {
		to.X = float64(snap)
	} else {
		to.Y = float64(snap)
	}
}

func scrollDidEndDrag(self objc.ID, _ objc.SEL, _ objc.ID, decelerate bool) {
	if v := scrollView(self); v != nil {
		v.pulledToRefresh()
		v.scrollEvent(driver.EventScrollDragEnd)
		if decelerate {
			v.scrollEvent(driver.EventMomentumStart)
		}
	}
}

func scrollDidEndGlide(self objc.ID, _ objc.SEL, _ objc.ID) {
	if v := scrollView(self); v != nil {
		v.scrollEvent(driver.EventMomentumEnd)
	}
}

func zoomView(self objc.ID, _ objc.SEL, _ objc.ID) objc.ID {
	if v := scrollView(self); v != nil {
		return v.sc.zoom
	}
	return 0
}

// scrollDidZoom centers the content again and reports the zoom with the
// offset.
func scrollDidZoom(self objc.ID, _ objc.SEL, _ objc.ID) {
	v := scrollView(self)
	if v == nil || v.sc.zoom == 0 {
		return
	}
	if v.sc.config.CenterContent {
		z := objc.Send[float64](self, selZoomScale)
		seen := objc.Send[apple.CGRect](self, selBounds).Size
		in := v.sc.config.Behavior.ContentInset
		w, h := float64(v.sc.content.Width)*z, float64(v.sc.content.Height)*z
		dx := max(seen.Width-float64(in.Left+in.Right)-w, 0)
		dy := max(seen.Height-float64(in.Top+in.Bottom)-h, 0)
		v.sc.zoom.Send(sel("setCenter:"), apple.CGPoint{X: (w + dx) / 2, Y: (h + dy) / 2})
	}
	v.scrollEvent(driver.EventScroll)
}

func scrollRefreshed(self objc.ID, _ objc.SEL, _ objc.ID) {
	if v := scrollView(self); v != nil {
		v.scrollEvent(driver.EventRefresh)
	}
}

func dismissKeyboardTap(self objc.ID, _ objc.SEL, _ objc.ID) {
	if w := self.Send(sel("window")); w != 0 {
		w.Send(sel("endEditing:"), true)
	}
}

// scrollTapReceives lets the keyboard's tap see a touch as
// KeyboardShouldPersistTaps says: never on a field, and with "handled"
// only where nothing takes presses. Other recognizers receive every touch.
func scrollTapReceives(self objc.ID, cmd objc.SEL, r, touch objc.ID) bool {
	v := scrollView(self)
	if v == nil || r != v.sc.tap {
		if objc.Send[bool](objc.ID(objc.GetClass("UIScrollView")), sel("instancesRespondToSelector:"), cmd) {
			return apple.SendSuper[bool](self, cmd, r, touch)
		}
		return true
	}
	under := touch.Send(selView)
	switch v.sc.config.Behavior.KeyboardShouldPersistTaps {
	case driver.KeyboardTapsAlways:
		return false
	case driver.KeyboardTapsHandled:
		return !onInput(under, self) && !handlesTouches(under, self)
	}
	return !onInput(under, self)
}

// onInput reports whether view is in a text field inside scroll.
func onInput(view, scroll objc.ID) bool {
	for p := view; p != 0 && p != scroll; p = p.Send(selSuperview) {
		if v := views[p]; v != nil && v.input != nil {
			return true
		}
	}
	return false
}

// handlesTouches reports whether view or a view around it inside scroll
// takes presses.
func handlesTouches(view, scroll objc.ID) bool {
	for p := view; p != 0 && p != scroll; p = p.Send(selSuperview) {
		if v := views[p]; v != nil && (v.events.Has(driver.EventPress) || v.events.Has(driver.EventLongPress) ||
			v.events.Has(driver.EventPressDown) || v.events.Has(driver.EventPan)) {
			return true
		}
	}
	return false
}

// viewOf returns the view of id, or nil.
func (s *surface) viewOf(id driver.NodeID) *view {
	if int(id) < len(s.views) {
		return s.views[id]
	}
	return nil
}
