//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// A selection area (a view with SelectionArea) selects across its texts
// as NSH's does, drawn as UIKit draws a text view's selection, which
// UITextInteraction keeps to one UITextInput view (a selectable text
// outside an area selects with it: textinput.go). A long press selects
// the word under the finger, and the finger's drag goes on by words.
// Lifting the finger shows the edit menu over the selection: Copy and
// Select All, which the view controller answers as the first responder,
// hardware keyboards' Cmd-C and Cmd-A included. A tap on the selection
// shows or hides the menu, and one off it clears the selection, as does a
// touch outside the area, so the surface shows one selection. Two
// handles, a bar with a knob above the start and below the end, move the
// ends; the long press and the tap leave their touches to them, the
// scroll views around wait for their drags, and the menu comes back as
// one is let go.

// selection is the surface's selection: an area's model over its texts.
type selection struct {
	area   *view
	texts  []*view
	frames []driver.Bounds // the texts' content boxes in the area
	model  driver.SelectionModel
	menu   objc.ID // the area's UIEditMenuInteraction, iOS 16 on
	shown  bool    // the menu shows, as its delegate hears
	// shownAtTap is whether it showed as the tap's finger went down,
	// which dismisses it.
	shownAtTap bool
	// handles are the start's and the end's, in the area; grab is where
	// the finger holds the one it drags, dragging, from its caret's
	// middle.
	handles  [2]objc.ID
	grab     driver.Point
	dragging driver.SelectionHandle
	// told is the extent the area last reported.
	told driver.Bounds
}

// Len implements core.SelectionTexts.
func (s *selection) Len() int { return len(s.texts) }

// Content implements core.SelectionTexts.
func (s *selection) Content(i int) string { return s.texts[i].plain }

// Frame implements core.SelectionTexts.
func (s *selection) Frame(i int) driver.Bounds { return s.frames[i] }

// Key implements core.SelectionTexts.
func (s *selection) Key(i int) uint64 { return uint64(s.texts[i].id) }

// OffsetAt implements core.SelectionTexts.
func (s *selection) OffsetAt(i int, p driver.Point) int {
	v, f := s.texts[i], s.frames[i]
	at := apple.CGPoint{X: float64(p.X - f.Origin.X + v.box.Origin.X), Y: float64(p.Y - f.Origin.Y + v.box.Origin.Y)}
	return v.layout.OffsetAt(at, boxRect(v.box))
}

func boxRect(b driver.Bounds) apple.CGRect {
	return apple.Rect(float64(b.Origin.X), float64(b.Origin.Y), float64(b.Size.Width), float64(b.Size.Height))
}

// list finds the area's texts again, in document order, with their frames.
func (s *selection) list() {
	s.texts, s.frames = s.texts[:0], s.frames[:0]
	var walk func(n objc.ID)
	walk = func(n objc.ID) {
		if objc.Send[bool](n, sel("isHidden")) {
			return
		}
		v := views[n]
		if v != nil && v.native == n && v.kind == driver.KindText {
			if v.layout != nil && v.selectable != driver.SelectableOff {
				r := objc.Send[apple.CGRect](s.area.native, sel("convertRect:fromView:"), boxRect(v.box), n)
				s.texts = append(s.texts, v)
				s.frames = append(s.frames, driver.Bounds{
					Origin: driver.Point{X: driver.Points(r.Origin.X), Y: driver.Points(r.Origin.Y)},
					Size:   driver.Size{Width: driver.Points(r.Size.Width), Height: driver.Points(r.Size.Height)},
				})
			}
			return
		}
		for _, c := range apple.Subviews(n) {
			walk(c)
		}
	}
	walk(s.area.native)
}

// show has each text draw its part of the selection.
func (s *selection) show(old []*view) {
	for _, v := range old {
		v.setSelected(0, 0)
	}
	for _, r := range s.model.Ranges(s) {
		s.texts[r.Text].setSelected(r.Start, r.End)
	}
	s.tell(s.model.Extent(s))
	s.placeHandles()
}

// tell reports the selection's extent to the area when it moved.
func (s *selection) tell(b driver.Bounds) {
	if b != s.told {
		s.told = b
		s.area.report(driver.Event{Kind: driver.EventSelectionChange, Point: b.Origin, Size: b.Size})
	}
}

// Handles are drawn as UIKit's are on iOS 26: a 2-point bar and a
// 16-point knob with a faint shadow, in the selection's color, where
// NSH's knob is 11 points.
const (
	handleBar  = 2  // the bar's width, in points
	handleKnob = 16 // the knob's diameter
	handleSlop = 22 // how far around its bar a handle takes touches
)

// caret returns the caret at end h of the selection in the area: its x,
// its line's top and its height; false without a selection.
func (s *selection) caret(h driver.SelectionHandle) (x, top, height float64, ok bool) {
	start, end, ok := s.model.Bounds()
	if !ok || start.Text >= len(s.texts) || end.Text >= len(s.texts) {
		return 0, 0, 0, false
	}
	p := start
	if h == driver.HandleEnd {
		p = end
	}
	r, ok := s.texts[p.Text].layout.CaretRect(driver.Caret{Offset: p.Offset, Affinity: driver.Upstream * driver.Affinity(h)})
	if !ok {
		return 0, 0, 0, false
	}
	f := s.frames[p.Text]
	return float64(f.Origin.X + r.Origin.X), float64(f.Origin.Y + r.Origin.Y), float64(r.Size.Height), true
}

// placeHandles puts the handles at the ends, or takes them away while
// nothing is selected.
func (s *selection) placeHandles() {
	for h := range s.handles {
		x, top, height, ok := s.caret(driver.SelectionHandle(h))
		if !ok || s.model.IsEmpty() {
			if s.handles[h] != 0 {
				s.handles[h].Send(sel("removeFromSuperview"))
				s.handles[h] = 0
			}
			continue
		}
		if s.handles[h] == 0 {
			s.handles[h] = s.newHandle(h)
		}
		knob := s.handles[h].Send(sel("subviews")).Send(sel("lastObject"))
		bar := s.handles[h].Send(sel("subviews")).Send(sel("firstObject"))
		// The handle's view reaches handleSlop around the bar; the start's
		// knob sits on the bar's top, the end's under its bottom.
		s.handles[h].Send(selSetFrame, apple.Rect(x-handleSlop, top-handleSlop, 2*handleSlop, height+2*handleSlop))
		bar.Send(selSetFrame, apple.Rect(handleSlop-handleBar/2, handleSlop, handleBar, height))
		knobY := handleSlop - handleKnob + 1.0
		if h == int(driver.HandleEnd) {
			knobY = handleSlop + height - 1
		}
		knob.Send(selSetFrame, apple.Rect(handleSlop-handleKnob/2, knobY, handleKnob, handleKnob))
		s.area.native.Send(sel("bringSubviewToFront:"), s.handles[h])
	}
}

// newHandle makes handle h's view in the area: a bar and a knob in the
// handles' color, which a pan drags.
func (s *selection) newHandle(h int) objc.ID {
	view := func() objc.ID { return objc.ID(apple.Class("UIView")).Send(selNew) }
	handle := view()
	handle.Send(sel("setTag:"), h)
	color := s.handleColor()
	for _, round := range []bool{false, true} {
		part := view()
		part.Send(sel("setUserInteractionEnabled:"), false)
		part.Send(sel("setBackgroundColor:"), color)
		if round {
			layer := part.Send(sel("layer"))
			layer.Send(sel("setCornerRadius:"), float64(handleKnob)/2)
			layer.Send(sel("setShadowOpacity:"), float32(0.2))
			layer.Send(sel("setShadowRadius:"), 1.5)
			layer.Send(sel("setShadowOffset:"), apple.CGSize{Height: 0.5})
		}
		handle.Send(selAddSubview, part)
		part.Send(selRelease)
	}
	pan := objc.ID(apple.Class("UIPanGestureRecognizer")).Send(selAlloc).Send(
		sel("initWithTarget:action:"), selector, selHandleAction)
	pan.Send(selSetDeleg, selector)
	handle.Send(sel("addGestureRecognizer:"), pan)
	pan.Send(selRelease)
	s.area.native.Send(selAddSubview, handle)
	handle.Send(selRelease)
	return handle
}

// handleColor is the handles' color: the selection's color, opaque, as
// UIKit draws a text view's handles in the tint its highlight fades;
// else the area's tint.
func (s *selection) handleColor() objc.ID {
	for _, v := range s.texts {
		if v.selColor != 0 {
			return uiColor(v.selColor).Send(sel("colorWithAlphaComponent:"), 1.0)
		}
	}
	return s.area.native.Send(sel("tintColor"))
}

// selectHandle follows a handle's drag: its end follows the finger, and
// the menu comes back as it lifts. Dragged past the other end, the ends
// swap, and so do the handles.
func selectHandle(_ objc.ID, _ objc.SEL, r objc.ID) {
	sl := runner.selecting
	if sl == nil {
		return
	}
	p := sl.point(r)
	switch objc.Send[int](r, selState) {
	case stateBegan:
		h := driver.SelectionHandle(objc.Send[int](r.Send(selView), sel("tag")))
		x, top, height, ok := sl.caret(h)
		if !ok {
			return
		}
		sl.dragging = h
		sl.grab = driver.Point{X: p.X - driver.Points(x), Y: p.Y - driver.Points(top+height/2)}
		sl.hideMenu()
	case stateChanged:
		old := sl.texts
		sl.list()
		s0, e0, _ := sl.model.Bounds()
		sl.model.MoveHandle(sl, sl.dragging, driver.Point{X: p.X - sl.grab.X, Y: p.Y - sl.grab.Y})
		if s1, e1, ok := sl.model.Bounds(); ok {
			swapped := sl.dragging == driver.HandleStart && s1 == e0 && e1 != e0 ||
				sl.dragging == driver.HandleEnd && e1 == s0 && s1 != s0
			if swapped {
				sl.dragging = 1 - sl.dragging
				sl.handles[0], sl.handles[1] = sl.handles[1], sl.handles[0]
				for i, h := range sl.handles {
					if h != 0 {
						h.Send(sel("setTag:"), i)
					}
				}
			}
		}
		sl.show(old)
	case stateEnded, stateCancelled:
		if !sl.model.IsEmpty() {
			sl.showMenu()
		}
	}
}

// setSelected sets the UTF-16 range a text highlights.
func (v *view) setSelected(start, end int) {
	if v.surface.views[v.id] != v {
		return // deleted
	}
	if v.selStart != start || v.selEnd != end {
		v.selStart, v.selEnd = start, end
		v.redraw()
	}
}

var (
	selSelectAction = sel("nsgoSelect:")
	selHandleAction = sel("nsgoHandle:")
	selTapAction    = sel("nsgoSelectionTap:")
	classSelector   objc.Class
	// selector is the target and delegate of the areas' recognizers and
	// the handles', and the delegate of their menus.
	selector objc.ID
)

func registerSelection() {
	var protocols []*objc.Protocol
	for _, name := range []string{"UIGestureRecognizerDelegate", "UIEditMenuInteractionDelegate"} {
		if p := objc.GetProtocol(name); p != nil {
			protocols = append(protocols, p)
		}
	}
	classSelector = register("NsgoSelector", apple.Class("NSObject"), protocols, []objc.MethodDef{
		{Cmd: selSelectAction, Fn: selectLongPress},
		{Cmd: selHandleAction, Fn: selectHandle},
		{Cmd: selTapAction, Fn: selectTap},
		{Cmd: sel("gestureRecognizer:shouldReceiveTouch:"), Fn: selectReceives},
		{Cmd: sel("gestureRecognizerShouldBegin:"), Fn: selectShouldBegin},
		// The tap leaves the presses of the views in the area alone.
		{Cmd: sel("gestureRecognizer:shouldRecognizeSimultaneouslyWithGestureRecognizer:"), Fn: func(_ objc.ID, _ objc.SEL, r, _ objc.ID) bool {
			return isTap(r)
		}},
		// The scroll views around wait for a handle's drag to fail.
		{Cmd: sel("gestureRecognizer:shouldBeRequiredToFailByGestureRecognizer:"), Fn: func(_ objc.ID, _ objc.SEL, r, other objc.ID) bool {
			v := other.Send(selView)
			return onHandle(r.Send(selView)) && v != 0 && objc.Send[bool](v, sel("isKindOfClass:"), apple.Class("UIScrollView"))
		}},
		// The menu keeps clear of the selection.
		{Cmd: sel("editMenuInteraction:targetRectForConfiguration:"), Fn: func(objc.ID, objc.SEL, objc.ID, objc.ID) apple.CGRect {
			if sl := runner.selecting; sl != nil {
				if r, ok := sl.selectionRect(); ok {
					return r
				}
			}
			return apple.CGRect{}
		}},
		{Cmd: sel("editMenuInteraction:willPresentMenuForConfiguration:animator:"), Fn: func(_ objc.ID, _ objc.SEL, m, _, _ objc.ID) {
			if sl := runner.selecting; sl != nil && sl.menu == m {
				sl.shown = true
			}
		}},
		{Cmd: sel("editMenuInteraction:willDismissMenuForConfiguration:animator:"), Fn: func(_ objc.ID, _ objc.SEL, m, _, _ objc.ID) {
			if sl := runner.selecting; sl != nil && sl.menu == m {
				sl.shown = false
			}
		}},
	})
	selector = objc.ID(classSelector).Send(selNew)
}

// isTap reports whether r is an area's tap.
func isTap(r objc.ID) bool {
	v := views[r.Send(selView)]
	return v != nil && v.selTap == r
}

// onHandle reports whether n is a handle of the selection or in one.
func onHandle(n objc.ID) bool {
	sl := runner.selecting
	if sl == nil || n == 0 {
		return false
	}
	for _, h := range sl.handles {
		if h != 0 && (n == h || objc.Send[bool](n, sel("isDescendantOfView:"), h)) {
			return true
		}
	}
	return false
}

// selectShouldBegin begins an area's tap only while it holds a
// selection, and a handle's drag only while there is one.
func selectShouldBegin(_ objc.ID, _ objc.SEL, r objc.ID) bool {
	sl := runner.selecting
	switch {
	case isTap(r):
		return sl != nil && sl.area.native == r.Send(selView) && !sl.model.IsEmpty()
	case onHandle(r.Send(selView)):
		return sl != nil && !sl.model.IsEmpty()
	}
	return true
}

// setSelectable gives v a long press recognizer while it is an area, and
// takes it away after; a selectable text outside one selects on its own
// (textinput.go).
func (v *view) setSelectable() {
	want := v.area
	switch {
	case want && v.longPress == 0:
		v.longPress = objc.ID(apple.Class("UILongPressGestureRecognizer")).Send(selAlloc).Send(
			sel("initWithTarget:action:"), selector, selSelectAction)
		v.selTap = objc.ID(apple.Class("UITapGestureRecognizer")).Send(selAlloc).Send(
			sel("initWithTarget:action:"), selector, selTapAction)
		v.selTap.Send(sel("setCancelsTouchesInView:"), false)
		// A long press that selects is no tap.
		v.selTap.Send(sel("requireGestureRecognizerToFail:"), v.longPress)
		for _, r := range []objc.ID{v.longPress, v.selTap} {
			r.Send(selSetDeleg, selector)
			v.native.Send(sel("addGestureRecognizer:"), r)
			r.Send(selRelease)
		}
	case !want && v.longPress != 0:
		v.native.Send(sel("removeGestureRecognizer:"), v.longPress)
		v.native.Send(sel("removeGestureRecognizer:"), v.selTap)
		v.longPress, v.selTap = 0, 0
	}
}

// outermost returns the area a long press on v selects in: the outermost
// area around it, or v.
func outermost(v *view) *view {
	area := v
	for n := v.native.Send(selSuperview); n != 0; n = n.Send(selSuperview) {
		if a := views[n]; a != nil && a.native == n && a.area {
			area = a
		}
	}
	return area
}

// selectReceives leaves the touches on the handles to them, and keeps an
// area's long press and tap from touches a field, a page or a gesture
// inside the area takes, or a view's own long press; a long press on a
// button or a link selects, as NSH's does.
func selectReceives(_ objc.ID, _ objc.SEL, r, touch objc.ID) bool {
	stop := r.Send(selView)
	if onHandle(stop) {
		return true // a handle's drag
	}
	if onHandle(touch.Send(selView)) {
		return false
	}
	if sl := runner.selecting; sl != nil && isTap(r) {
		sl.shownAtTap = sl.shown
	}
	for n := touch.Send(selView); n != 0 && n != stop; n = n.Send(selSuperview) {
		if v := views[n]; v != nil && v.native == n && (v.kind == driver.KindInput || v.kind == driver.KindWeb || v.pan != 0 ||
			v.events.Has(driver.EventLongPress)) {
			return false
		}
	}
	return true
}

// selectLongPress selects the word under a long press and follows its
// drag by words, then shows the edit menu.
func selectLongPress(_ objc.ID, _ objc.SEL, r objc.ID) {
	v := views[r.Send(selView)]
	if v == nil {
		return
	}
	s := v.surface
	switch objc.Send[int](r, selState) {
	case stateBegan:
		if t := s.firstText(); t != nil {
			t.resignText()
		}
		area := outermost(v)
		if s.selecting == nil || s.selecting.area != area {
			s.clearSelection()
			s.selecting = &selection{area: area}
		}
		sl := s.selecting
		old := sl.texts
		sl.list()
		sl.model.SelectWordAt(sl, sl.point(r))
		sl.show(old)
	case stateChanged:
		if sl := s.selecting; sl != nil {
			old := sl.texts
			sl.list()
			sl.model.Drag(sl, sl.point(r))
			sl.show(old)
		}
	case stateEnded:
		if sl := s.selecting; sl != nil && !sl.model.IsEmpty() {
			sl.showMenu()
		}
	}
}

// point returns where recognizer r is in the area.
func (s *selection) point(r objc.ID) driver.Point {
	p := objc.Send[apple.CGPoint](r, selLocationInView, s.area.native)
	return driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}
}

// selectTap answers a tap in the area as a text view does: on the
// selection it shows or hides the menu, off it it clears the selection.
func selectTap(_ objc.ID, _ objc.SEL, r objc.ID) {
	sl := runner.selecting
	if sl == nil || objc.Send[int](r, selState) != stateEnded {
		return
	}
	sl.list()
	switch {
	case !sl.onSelection(sl.point(r)):
		sl.area.surface.clearSelection()
	case sl.shown || sl.shownAtTap:
		sl.hideMenu()
	default:
		sl.showMenu()
	}
}

// onSelection reports whether p, in the area, is on the highlight.
func (s *selection) onSelection(p driver.Point) bool {
	for _, r := range s.model.Ranges(s) {
		v, f := s.texts[r.Text], s.frames[r.Text]
		at := apple.CGPoint{X: float64(p.X - f.Origin.X + v.box.Origin.X), Y: float64(p.Y - f.Origin.Y + v.box.Origin.Y)}
		for _, b := range v.rangeRects(r.Start, r.End) {
			if at.X >= b.Origin.X && at.X <= b.Origin.X+b.Size.Width && at.Y >= b.Origin.Y && at.Y <= b.Origin.Y+b.Size.Height {
				return true
			}
		}
	}
	return false
}

// selectionRect returns the selection's extent in the area, from the
// start's caret to the end's.
func (s *selection) selectionRect() (apple.CGRect, bool) {
	if s.model.IsEmpty() {
		return apple.CGRect{}, false
	}
	x0, t0, h0, ok0 := s.caret(driver.HandleStart)
	x1, t1, h1, ok1 := s.caret(driver.HandleEnd)
	if !ok0 || !ok1 {
		return apple.CGRect{}, false
	}
	left, top := min(x0, x1), min(t0, t1)
	right, bottom := max(x0, x1), max(t0+h0, t1+h1)
	return apple.Rect(left, top, right-left, bottom-top), true
}

// showMenu shows the edit menu over the selection: UIEditMenuInteraction
// from iOS 16, its delegate keeping it clear of the selection, and
// UIMenuController before.
func (s *selection) showMenu() {
	r, ok := s.selectionRect()
	if !ok {
		return
	}
	if class := apple.Class("UIEditMenuInteraction"); class != 0 {
		if s.menu == 0 {
			s.menu = objc.ID(class).Send(selAlloc).Send(sel("initWithDelegate:"), selector)
			s.area.native.Send(sel("addInteraction:"), s.menu)
			s.menu.Send(selRelease)
		}
		at := apple.CGPoint{X: r.Origin.X + r.Size.Width/2, Y: r.Origin.Y}
		config := objc.ID(apple.Class("UIEditMenuConfiguration")).Send(
			sel("configurationWithIdentifier:sourcePoint:"), objc.ID(0), at)
		s.menu.Send(sel("presentEditMenuWithConfiguration:"), config)
		return
	}
	menu := objc.ID(apple.Class("UIMenuController")).Send(sel("sharedMenuController"))
	menu.Send(sel("showMenuFromView:rect:"), s.area.native, r)
}

// hideMenu hides the edit menu.
func (s *selection) hideMenu() {
	if s.menu != 0 {
		s.menu.Send(sel("dismissMenu"))
	} else if apple.Class("UIEditMenuInteraction") == 0 {
		objc.ID(apple.Class("UIMenuController")).Send(sel("sharedMenuController")).Send(sel("hideMenuFromView:"), s.area.native)
	}
	s.shown = false
}

// clearTouched clears the selection when a touch goes down outside its
// area.
func (s *surface) clearTouched(touch objc.ID) {
	sl := s.selecting
	if sl == nil {
		return
	}
	hit := touch.Send(selView)
	if hit != 0 && (hit == sl.area.native || objc.Send[bool](hit, sel("isDescendantOfView:"), sl.area.native)) {
		return
	}
	s.clearSelection()
}

// refreshSelection follows the area's texts after a batch: the selection
// keeps to its texts, and goes with its area.
func (s *surface) refreshSelection() {
	sl := s.selecting
	if sl == nil {
		return
	}
	if s.views[sl.area.id] != sl.area || sl.area.native.Send(sel("window")) == 0 {
		s.clearSelection()
		return
	}
	old := sl.texts
	sl.list()
	sl.model.TextsChanged(sl)
	sl.show(old)
}

func (s *surface) clearSelection() {
	sl := s.selecting
	if sl == nil {
		return
	}
	for _, v := range sl.texts {
		v.setSelected(0, 0)
	}
	sl.tell(driver.Bounds{})
	for _, h := range sl.handles {
		if h != 0 {
			h.Send(sel("removeFromSuperview"))
		}
	}
	if sl.menu != 0 && s.views[sl.area.id] == sl.area {
		sl.menu.Send(sel("dismissMenu"))
		sl.area.native.Send(sel("removeInteraction:"), sl.menu)
	}
	s.selecting = nil
}

// controllerCopy is the view controller's copy:, which copies the
// selection.
func controllerCopy(_ objc.ID, _ objc.SEL, _ objc.ID) {
	sl := runner.selecting
	if sl == nil || sl.model.IsEmpty() {
		return
	}
	sl.list()
	pb := objc.ID(apple.Class("UIPasteboard")).Send(sel("generalPasteboard"))
	pb.Send(sel("setString:"), apple.NSString(sl.model.Text(sl)))
}

// controllerSelectAll is the view controller's selectAll:, which selects
// the area and shows the menu again.
func controllerSelectAll(_ objc.ID, _ objc.SEL, _ objc.ID) {
	sl := runner.selecting
	if sl == nil {
		return
	}
	old := sl.texts
	sl.list()
	sl.model.SelectAll(sl)
	sl.show(old)
	sl.showMenu()
}

// controllerCanPerform offers Copy and Select All while there is a
// selection.
func controllerCanPerform(self objc.ID, cmd objc.SEL, action objc.SEL, sender objc.ID) bool {
	if action == sel("copy:") || action == sel("selectAll:") {
		return runner.selecting != nil && !runner.selecting.model.IsEmpty()
	}
	return apple.SendSuper[bool](self, cmd, action, sender)
}

// drawSelection fills the highlight behind a text's glyphs: its
// SelectionColor, else the tint at 20%.
func (v *view) drawSelection(ctx uintptr) {
	if v.selEnd <= v.selStart {
		return
	}
	var color uintptr
	if v.selColor != 0 {
		color = apple.CGColor(v.selColor)
		defer apple.Release(color)
	} else {
		tint := v.native.Send(sel("tintColor")).Send(sel("colorWithAlphaComponent:"), 0.2)
		color = objc.Send[uintptr](tint, sel("CGColor"))
	}
	apple.FillRects(ctx, v.layout.SelectionRects(v.selStart, v.selEnd, boxRect(v.box)), color)
}
