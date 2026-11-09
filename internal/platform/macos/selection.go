//go:build darwin && !ios

package macos

import (
	"math"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// A selection area (a view with SelectionArea, or a selectable text
// outside one) selects with the mouse as a desktop does: drag, double
// click a word, triple click a whole text, Shift-click to extend. The
// surface takes the focus while an area has a selection, for Copy and
// Select All (the Edit menu, Cmd-C and Cmd-A) and Escape, which clears
// it; a press anywhere else clears it, so the window shows one selection.
// A drag past the edge of the scroll view around the area scrolls it on
// the frame clock, held still too, as NSH's does.

// selection is the surface's selection: an area's model over its texts.
type selection struct {
	area   *view
	texts  []*view
	frames []driver.Bounds // the texts' content boxes in the area
	model  driver.SelectionModel
	// down says the button pressed in the area is down, and dragging that
	// its drag selects; a press on a press target in the area (a button,
	// a link) goes to it as well, until the drag passes selectPressSlop
	// from where it went down, from.
	down, dragging bool
	press          *view
	from           apple.CGPoint
	// at is where the drag is, in the window.
	at apple.CGPoint
	// told is the extent the area last reported.
	told driver.Bounds
}

const (
	// selectionScroll is how far a drag past a scroll view's edge scrolls
	// it a 60 Hz frame, per point past the edge, and selectionScrollMax
	// the most.
	selectionScroll    = 0.4
	selectionScrollMax = 40
	// selectPressSlop is how far a press on a press target in an area
	// drags before it selects instead, as NSH's does.
	selectPressSlop = 10
)

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

// list finds the area's texts again, in document order, with their frames,
// which AppKit converts once the scroll offsets are committed.
func (s *selection) list() {
	s.area.surface.commitPending()
	s.texts, s.frames = s.texts[:0], s.frames[:0]
	var walk func(n objc.ID)
	walk = func(n objc.ID) {
		v := views[n]
		if v != nil && v.native == n && v.kind == driver.KindText {
			if v.layout != nil && v.selectable != driver.SelectableOff &&
				!objc.Send[bool](n, sel("isHiddenOrHasHiddenAncestor")) {
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

// point returns event's location in the area.
func (s *selection) point(event objc.ID) driver.Point {
	return s.pointAt(objc.Send[apple.CGPoint](event, selLocationInWindow))
}

// pointAt returns a point of the window in the area.
func (s *selection) pointAt(loc apple.CGPoint) driver.Point {
	p := objc.Send[apple.CGPoint](s.area.native, selConvertPoint, loc, objc.ID(0))
	return driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}
}

// drag extends the selection to where the drag is.
func (s *selection) drag() {
	old := s.texts
	s.list()
	s.model.Drag(s, s.pointAt(s.at))
	s.show(old)
}

// scroller returns the scroll view around the area, if any.
func (s *selection) scroller() *view {
	for n := s.area; n != nil; n = n.up {
		if n.surface.alive(n) {
			return n
		}
	}
	return nil
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
}

// tell reports the selection's extent to the area when it moved.
func (s *selection) tell(b driver.Bounds) {
	if b != s.told {
		s.told = b
		s.area.report(driver.Event{Kind: driver.EventSelectionChange, Point: b.Origin, Size: b.Size})
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

// areaAt returns the selection area a mouse press at event goes to, the
// outermost area around the view hit or the selectable text hit, and the
// press target it hit in the area, if any: a pressable view, or a text's
// link. A field or a gesture inside the area keeps the press.
func (s *surface) areaAt(event objc.ID) (area, press *view) {
	up := s.native.Send(selSuperview)
	if up == 0 {
		return nil, nil
	}
	at := objc.Send[apple.CGPoint](event, selLocationInWindow)
	p := objc.Send[apple.CGPoint](up, selConvertPoint, at, objc.ID(0))
	rawHit = true
	hit := s.native.Send(sel("hitTest:"), p)
	rawHit = false
	taken := false
	for n := hit; n != 0 && n != s.native; n = n.Send(selSuperview) {
		v := views[n]
		if v == nil || v.native != n && !v.isEditor(n) {
			continue
		}
		if area == nil && press == nil && !taken && v.native == n &&
			v.pressableAt(objc.Send[apple.CGPoint](n, selConvertPoint, at, objc.ID(0))) {
			press = v
		}
		switch {
		case v.isEditor(n) || v.kind == driver.KindInput || v.kind == driver.KindWeb:
			taken = true
		case area == nil && v.events.Has(driver.EventPan):
			taken = true
		case v.area && (area != nil || !taken):
			area = v
		case v.kind == driver.KindText && v.selectable == driver.SelectableOn && !taken:
			area = v
		}
	}
	return area, press
}

// selectPress starts or extends a selection on a mouse press, or clears
// the selection a press elsewhere leaves. It reports whether it took the
// press: one on a press target goes on to it.
func (s *surface) selectPress(event objc.ID) bool {
	area, press := s.areaAt(event)
	if area == nil {
		s.clearSelection()
		return false
	}
	if s.selecting == nil || s.selecting.area != area {
		s.clearSelection()
		s.selecting = &selection{area: area}
	}
	sl := s.selecting
	old := sl.texts
	sl.list()
	const shift = 1 << 17
	extend := objc.Send[uint](event, sel("modifierFlags"))&shift != 0
	sl.model.Press(sl, sl.point(event), objc.Send[int](event, sel("clickCount")), extend)
	sl.down, sl.dragging, sl.press = true, press == nil, press
	sl.from = objc.Send[apple.CGPoint](event, selLocationInWindow)
	sl.show(old)
	s.window.Send(selMakeFirstResponer, s.native)
	return press == nil
}

// selectDrag follows a drag of the press that started a selection; the
// frame clock scrolls the scroll view around the area while the drag is
// past its edge (autoscrollSelection).
func (s *surface) selectDrag(event objc.ID) bool {
	sl := s.selecting
	if sl == nil || !sl.down {
		return false
	}
	sl.at = objc.Send[apple.CGPoint](event, selLocationInWindow)
	if !sl.dragging {
		if math.Hypot(sl.at.X-sl.from.X, sl.at.Y-sl.from.Y) <= selectPressSlop {
			return false
		}
		// The drag selects: the press target's press is no press.
		sl.dragging = true
		if p := sl.press; p != nil && p.pressing.Swap(false) {
			p.longFired.Store(true)
			p.pressed(driver.EventPressUp)
		}
	}
	sl.drag()
	if sl.scroller() != nil {
		s.startTicking()
	}
	return true
}

// autoscrollSelection scrolls the scroll view around a selection's area
// for a frame of dt while the drag is past its edge, extending the
// selection, and reports whether it does.
func (s *surface) autoscrollSelection(dt time.Duration) bool {
	sl := s.selecting
	if sl == nil || !sl.dragging {
		return false
	}
	v := sl.scroller()
	if v == nil || v.sc.phase.kind != atRest {
		return false
	}
	p := objc.Send[apple.CGPoint](v.native, selConvertPoint, sl.at, objc.ID(0))
	along, length := p.Y, float64(v.frame.Size.Height)
	if v.sc.config.Horizontal {
		along, length = p.X, float64(v.frame.Size.Width)
	}
	var over float64
	switch {
	case along < 0:
		over = along
	case along > length:
		over = along - length
	default:
		return false
	}
	step := min(max(over*selectionScroll, -selectionScrollMax), selectionScrollMax) * dt.Seconds() * 60
	lo, hi := v.restLimits()
	to := min(max(v.pos()+driver.Points(step), lo), hi)
	if to == v.pos() {
		return false
	}
	v.moveTo(to)
	v.commit()
	sl.drag()
	return true
}

// selectUp ends a selection's drag; the release of a press that never
// dragged goes on to its press target.
func (s *surface) selectUp() bool {
	sl := s.selecting
	if sl == nil || !sl.down {
		return false
	}
	took := sl.dragging
	sl.down, sl.dragging, sl.press = false, false, nil
	return took
}

// selectKey answers the keys of a selection while the surface holds it:
// Cmd-C copies, Cmd-A selects the area and Escape clears. It reports
// whether it took the key. The Edit menu's Copy and Select All reach the
// same methods; keys go here first, so menus without them work too.
func (s *surface) selectKey(event objc.ID) bool {
	if s.selecting == nil || s.window.Send(sel("firstResponder")) != s.native {
		return false
	}
	const shift, control, option, command = 1 << 17, 1 << 18, 1 << 19, 1 << 20
	const escape = 53
	mods := objc.Send[uint](event, sel("modifierFlags")) & (shift | control | option | command)
	switch key := apple.GoString(event.Send(sel("charactersIgnoringModifiers"))); {
	case mods == command && key == "c":
		surfaceCopy(s.native, 0, 0)
	case mods == command && key == "a":
		surfaceSelectAll(s.native, 0, 0)
	case mods == 0 && objc.Send[uint16](event, sel("keyCode")) == escape && !s.selecting.model.IsEmpty():
		s.clearSelection()
	default:
		return false
	}
	return true
}

// refreshSelection follows the area's texts after a batch: the selection
// keeps to its texts, and goes with its area.
func (s *surface) refreshSelection() {
	sl := s.selecting
	if sl == nil {
		return
	}
	if s.views[sl.area.id] != sl.area || sl.area.native.Send(selWindow) == 0 {
		s.clearSelection()
		return
	}
	old := sl.texts
	sl.list()
	sl.model.TextsChanged(sl)
	sl.show(old)
}

func (s *surface) clearSelection() {
	if s.selecting == nil {
		return
	}
	for _, v := range s.selecting.texts {
		v.setSelected(0, 0)
	}
	s.selecting.tell(driver.Bounds{})
	s.selecting = nil
}

// surfaceCopy is the surface's copy:, which copies the selection.
func surfaceCopy(self objc.ID, _ objc.SEL, _ objc.ID) {
	s := views[self].surface
	if s.selecting == nil || s.selecting.model.IsEmpty() {
		return
	}
	s.selecting.list()
	pb := objc.ID(apple.Class("NSPasteboard")).Send(sel("generalPasteboard"))
	pb.Send(sel("clearContents"))
	pb.Send(sel("setString:forType:"), apple.NSString(s.selecting.model.Text(s.selecting)), apple.NSString(pasteboardText))
}

// surfaceSelectAll is the surface's selectAll:, which selects the area.
func surfaceSelectAll(self objc.ID, _ objc.SEL, _ objc.ID) {
	s := views[self].surface
	if s.selecting == nil {
		return
	}
	old := s.selecting.texts
	s.selecting.list()
	s.selecting.model.SelectAll(s.selecting)
	s.selecting.show(old)
}

// surfaceResign clears the selection as another view takes the focus.
func surfaceResign(self objc.ID, cmd objc.SEL) bool {
	views[self].surface.clearSelection()
	return true
}

// selectionColor returns the CGColor text v highlights in, the text's
// SelectionColor or the system's, and whether the caller releases it.
func (v *view) selectionColor() (color uintptr, owned bool) {
	if v.selColor != 0 {
		return apple.CGColor(v.selColor), true
	}
	ns := objc.ID(apple.Class("NSColor")).Send(sel("selectedTextBackgroundColor"))
	return objc.Send[uintptr](ns, sel("CGColor")), false
}

// drawSelection fills the highlight behind a text's glyphs.
func (v *view) drawSelection(ctx uintptr) {
	if v.selEnd <= v.selStart {
		return
	}
	color, owned := v.selectionColor()
	apple.FillRects(ctx, v.layout.SelectionRects(v.selStart, v.selEnd, boxRect(v.box)), color)
	if owned {
		apple.Release(color)
	}
}
