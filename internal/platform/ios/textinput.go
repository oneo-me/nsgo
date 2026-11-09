//go:build ios

package ios

import (
	"unicode/utf16"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// A selectable text outside any area selects as UIKit's text views do,
// as NSH's does: its view adopts UITextInput (non-editable) and a
// UITextInteraction does the rest, the long press that selects a word,
// the loupe, the handles, the highlight in the view's tint (its
// SelectionColor, opaque) and the edit menu (Copy, Select All, Look Up,
// Translate, Share). Positions are UTF-16 offsets into the text, the ones
// its layout uses. Texts in an area have no selection of their own: the
// area selects across them (selection.go).

// textSelection is a selectable text's own selection.
type textSelection struct {
	interaction objc.ID // its UITextInteraction
	start, end  int     // the selected UTF-16 range, while has
	has         bool
	// delegate is UIKit's observer of the selection (not retained, as
	// UIKit's own text views keep it weakly); tokenizer is made on first
	// use, until the view resigns.
	delegate, tokenizer objc.ID
}

var (
	classText, classTextPosition, classTextRange, classSelectionRect objc.Class

	// positions, ranges and selectionRects are what the classes above
	// hold, by object, until they are freed.
	positions      = map[objc.ID]int{}
	ranges         = map[objc.ID][2]int{}
	selectionRects = map[objc.ID]selectionRect{}

	emptyInputView objc.ID
)

type selectionRect struct {
	rect       apple.CGRect
	rtl        bool
	start, end bool
}

// registerTextInput registers the text view's class, with the views'
// common methods, and the classes it gives UIKit.
func registerTextInput(common []objc.MethodDef) {
	freed := func(forget func(objc.ID)) objc.MethodDef {
		return objc.MethodDef{Cmd: sel("dealloc"), Fn: func(self objc.ID, cmd objc.SEL) {
			forget(self)
			apple.Super(self, cmd)
		}}
	}
	if apple.Class("UITextPosition") == 0 || apple.Class("UITextRange") == 0 || apple.Class("UITextSelectionRect") == 0 {
		classText = register("NsgoText", apple.Class("UIView"), nil, common)
		return
	}
	classTextPosition = register("NsgoTextPosition", apple.Class("UITextPosition"), nil, []objc.MethodDef{
		freed(func(self objc.ID) { delete(positions, self) }),
	})
	classTextRange = register("NsgoTextRange", apple.Class("UITextRange"), nil, []objc.MethodDef{
		freed(func(self objc.ID) { delete(ranges, self) }),
		{Cmd: sel("start"), Fn: func(self objc.ID, _ objc.SEL) objc.ID { return textPosition(ranges[self][0]) }},
		{Cmd: sel("end"), Fn: func(self objc.ID, _ objc.SEL) objc.ID { return textPosition(ranges[self][1]) }},
		{Cmd: sel("isEmpty"), Fn: func(self objc.ID, _ objc.SEL) bool { r := ranges[self]; return r[0] == r[1] }},
	})
	classSelectionRect = register("NsgoSelectionRect", apple.Class("UITextSelectionRect"), nil, []objc.MethodDef{
		freed(func(self objc.ID) { delete(selectionRects, self) }),
		{Cmd: sel("rect"), Fn: func(self objc.ID, _ objc.SEL) apple.CGRect { return selectionRects[self].rect }},
		{Cmd: sel("writingDirection"), Fn: func(self objc.ID, _ objc.SEL) int {
			if selectionRects[self].rtl {
				return 1 // NSWritingDirectionRightToLeft
			}
			return 0
		}},
		{Cmd: sel("containsStart"), Fn: func(self objc.ID, _ objc.SEL) bool { return selectionRects[self].start }},
		{Cmd: sel("containsEnd"), Fn: func(self objc.ID, _ objc.SEL) bool { return selectionRects[self].end }},
		{Cmd: sel("isVertical"), Fn: func(objc.ID, objc.SEL) bool { return false }},
	})
	var protocols []*objc.Protocol
	if p := objc.GetProtocol("UITextInput"); p != nil {
		protocols = append(protocols, p)
	}
	methods := append(append([]objc.MethodDef{}, common...), textInputMethods()...)
	classText = register("NsgoText", apple.Class("UIView"), protocols, methods)
}

// textPosition returns an autoreleased position at offset.
func textPosition(offset int) objc.ID {
	p := objc.ID(classTextPosition).Send(selNew)
	positions[p] = offset
	return p.Send(sel("autorelease"))
}

// textRange returns an autoreleased range between two offsets.
func textRange(a, b int) objc.ID {
	r := objc.ID(classTextRange).Send(selNew)
	ranges[r] = [2]int{min(a, b), max(a, b)}
	return r.Send(sel("autorelease"))
}

// textOf returns the text view self, with its own selection on.
func textOf(self objc.ID) *view {
	if v := views[self]; v != nil && v.native == self && v.tsel != nil {
		return v
	}
	return nil
}

// units is v's text in UTF-16, as UIKit counts.
func (v *view) units() []uint16 { return utf16.Encode([]rune(v.plain)) }

// clampUnits keeps offset in v's text.
func (v *view) clampUnits(offset int) int { return max(0, min(offset, len(v.units()))) }

// changeSelection sets v's selection from NSGo (not UIKit), telling
// UIKit's observer.
func (v *view) changeSelection(start, end int, has bool) {
	t := v.tsel
	if t == nil || t.has == has && (!has || t.start == start && t.end == end) {
		return
	}
	d := t.delegate
	if d != 0 {
		d.Send(sel("selectionWillChange:"), v.native)
	}
	t.start, t.end, t.has = start, end, has
	if d != 0 {
		d.Send(sel("selectionDidChange:"), v.native)
	}
}

// caretRect returns the caret's rect at offset in v; upstream puts an
// offset where a line wraps at the end of the line before. UIKit's caret
// is 2 points wide.
func (v *view) caretRect(offset int, upstream bool) apple.CGRect {
	a := driver.Downstream
	if upstream {
		a = driver.Upstream
	}
	if v.layout != nil {
		if r, ok := v.layout.CaretRect(driver.Caret{Offset: offset, Affinity: a}); ok {
			return apple.Rect(float64(v.box.Origin.X+r.Origin.X), float64(v.box.Origin.Y+r.Origin.Y), 2, float64(r.Size.Height))
		}
	}
	return apple.Rect(float64(v.box.Origin.X), float64(v.box.Origin.Y), 2, float64(v.box.Size.Height))
}

// closestOffset returns the caret offset nearest p, in v.
func (v *view) closestOffset(p apple.CGPoint) int {
	if v.layout == nil {
		return 0
	}
	return v.clampUnits(v.layout.OffsetAt(p, boxRect(v.box)))
}

// rangeRects returns the boxes of UTF-16 range a to b in v, a box per
// line's run.
func (v *view) rangeRects(a, b int) []apple.CGRect {
	if v.layout == nil {
		return nil
	}
	return v.layout.SelectionRects(a, b, boxRect(v.box))
}

func textInputMethods() []objc.MethodDef {
	none := func(objc.ID, objc.SEL) objc.ID { return 0 }
	pos := func(p objc.ID) int { return positions[p] }
	// UITextLayoutDirection is right 2, left 3, up 4 and down 5;
	// UITextStorageDirection, which UIKit may pass too, forward 0 and
	// backward 1.
	back := func(direction int) bool { return direction == 1 || direction == 3 || direction == 4 }
	return []objc.MethodDef{
		{Cmd: sel("canBecomeFirstResponder"), Fn: func(self objc.ID, _ objc.SEL) bool { return textOf(self) != nil }},
		{Cmd: sel("becomeFirstResponder"), Fn: func(self objc.ID, cmd objc.SEL) bool {
			ok := apple.SendSuper[bool](self, cmd)
			if v := textOf(self); ok && v != nil {
				// The surface shows one selection.
				v.surface.clearSelection()
			}
			return ok
		}},
		{Cmd: sel("resignFirstResponder"), Fn: func(self objc.ID, cmd objc.SEL) bool {
			ok := apple.SendSuper[bool](self, cmd)
			if v := textOf(self); ok && v != nil {
				v.changeSelection(0, 0, false)
				if v.tsel.tokenizer != 0 {
					v.tsel.tokenizer.Send(selRelease)
					v.tsel.tokenizer = 0
				}
			}
			return ok
		}},
		{Cmd: sel("canPerformAction:withSender:"), Fn: func(self objc.ID, cmd objc.SEL, action objc.SEL, sender objc.ID) bool {
			v := textOf(self)
			switch {
			case v == nil:
				return apple.SendSuper[bool](self, cmd, action, sender)
			case action == sel("cut:") || action == sel("paste:") || action == sel("delete:") ||
				action == sel("toggleBoldface:") || action == sel("toggleItalics:") || action == sel("toggleUnderline:"):
				return false // nothing edits the text
			case action == sel("copy:"):
				return v.tsel.has && v.tsel.start < v.tsel.end
			case action == sel("selectAll:"):
				n := len(v.units())
				return n > 0 && !(v.tsel.has && v.tsel.start == 0 && v.tsel.end == n)
			}
			// The system's own: Look Up, Translate, Share.
			return apple.SendSuper[bool](self, cmd, action, sender)
		}},
		{Cmd: sel("copy:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
			if v := textOf(self); v != nil && v.tsel.has && v.tsel.start < v.tsel.end {
				u := v.units()
				text := string(utf16.Decode(u[v.clampUnits(v.tsel.start):v.clampUnits(v.tsel.end)]))
				objc.ID(apple.Class("UIPasteboard")).Send(sel("generalPasteboard")).Send(sel("setString:"), apple.NSString(text))
			}
		}},
		{Cmd: sel("selectAll:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
			if v := textOf(self); v != nil {
				if n := len(v.units()); n > 0 {
					v.changeSelection(0, n, true)
				}
			}
		}},
		// No keyboard comes up for a text that is only selected.
		{Cmd: sel("inputView"), Fn: func(objc.ID, objc.SEL) objc.ID {
			if emptyInputView == 0 {
				emptyInputView = objc.ID(apple.Class("UIView")).Send(selNew)
			}
			return emptyInputView
		}},
		{Cmd: sel("isEditable"), Fn: func(objc.ID, objc.SEL) bool { return false }},

		// UIKeyInput, which edits nothing.
		{Cmd: sel("hasText"), Fn: func(self objc.ID, _ objc.SEL) bool { v := views[self]; return v != nil && v.plain != "" }},
		{Cmd: sel("insertText:"), Fn: func(objc.ID, objc.SEL, objc.ID) {}},
		{Cmd: sel("deleteBackward"), Fn: func(objc.ID, objc.SEL) {}},

		// UITextInput: the text and its selection.
		{Cmd: sel("textInRange:"), Fn: func(self objc.ID, _ objc.SEL, r objc.ID) objc.ID {
			v := views[self]
			if v == nil {
				return 0
			}
			u, o := v.units(), ranges[r]
			a, b := v.clampUnits(o[0]), v.clampUnits(o[1])
			return apple.NSString(string(utf16.Decode(u[a:max(a, b)])))
		}},
		{Cmd: sel("replaceRange:withText:"), Fn: func(objc.ID, objc.SEL, objc.ID, objc.ID) {}},
		{Cmd: sel("selectedTextRange"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if v := textOf(self); v != nil && v.tsel.has {
				return textRange(v.tsel.start, v.tsel.end)
			}
			return 0
		}},
		{Cmd: sel("setSelectedTextRange:"), Fn: func(self objc.ID, _ objc.SEL, r objc.ID) {
			v := textOf(self)
			if v == nil {
				return
			}
			o, ok := ranges[r]
			if r == 0 || !ok || o[0] >= o[1] {
				v.tsel.has = false
				return
			}
			v.tsel.start, v.tsel.end, v.tsel.has = v.clampUnits(o[0]), v.clampUnits(o[1]), true
		}},
		{Cmd: sel("markedTextRange"), Fn: none},
		{Cmd: sel("markedTextStyle"), Fn: none},
		{Cmd: sel("setMarkedTextStyle:"), Fn: func(objc.ID, objc.SEL, objc.ID) {}},
		{Cmd: sel("setMarkedText:selectedRange:"), Fn: func(objc.ID, objc.SEL, objc.ID, apple.NSRange) {}},
		{Cmd: sel("unmarkText"), Fn: func(objc.ID, objc.SEL) {}},
		{Cmd: sel("inputDelegate"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if v := textOf(self); v != nil {
				return v.tsel.delegate
			}
			return 0
		}},
		{Cmd: sel("setInputDelegate:"), Fn: func(self objc.ID, _ objc.SEL, d objc.ID) {
			if v := textOf(self); v != nil {
				v.tsel.delegate = d
			}
		}},
		{Cmd: sel("tokenizer"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			v := textOf(self)
			if v == nil {
				return 0
			}
			if v.tsel.tokenizer == 0 {
				v.tsel.tokenizer = objc.ID(apple.Class("UITextInputStringTokenizer")).Send(selAlloc).
					Send(sel("initWithTextInput:"), self)
			}
			return v.tsel.tokenizer
		}},
		{Cmd: sel("textInputView"), Fn: func(self objc.ID, _ objc.SEL) objc.ID { return self }},

		// Positions.
		{Cmd: sel("beginningOfDocument"), Fn: func(objc.ID, objc.SEL) objc.ID { return textPosition(0) }},
		{Cmd: sel("endOfDocument"), Fn: func(self objc.ID, _ objc.SEL) objc.ID {
			if v := views[self]; v != nil {
				return textPosition(len(v.units()))
			}
			return textPosition(0)
		}},
		{Cmd: sel("textRangeFromPosition:toPosition:"), Fn: func(_ objc.ID, _ objc.SEL, a, b objc.ID) objc.ID {
			return textRange(pos(a), pos(b))
		}},
		{Cmd: sel("positionFromPosition:offset:"), Fn: func(self objc.ID, _ objc.SEL, p objc.ID, by int) objc.ID {
			v := views[self]
			if to := pos(p) + by; v != nil && to >= 0 && to <= len(v.units()) {
				return textPosition(to)
			}
			return 0
		}},
		{Cmd: sel("positionFromPosition:inDirection:offset:"), Fn: func(self objc.ID, _ objc.SEL, p objc.ID, direction, by int) objc.ID {
			v := views[self]
			if v == nil {
				return 0
			}
			at, n := pos(p), len(v.units())
			switch direction {
			case 0, 1, 2, 3: // characters
				if back(direction) {
					by = -by
				}
				if to := at + by; to >= 0 && to <= n {
					return textPosition(to)
				}
				return 0
			}
			// Up and down: lines, keeping the x.
			c := v.caretRect(at, false)
			if direction == 4 {
				by = -by
			}
			y := c.Origin.Y + c.Size.Height/2 + float64(by)*c.Size.Height
			switch b := boxRect(v.box); {
			case y < b.Origin.Y:
				return textPosition(0)
			case y > b.Origin.Y+b.Size.Height:
				return textPosition(n)
			}
			return textPosition(v.closestOffset(apple.CGPoint{X: c.Origin.X, Y: y}))
		}},
		{Cmd: sel("comparePosition:toPosition:"), Fn: func(_ objc.ID, _ objc.SEL, a, b objc.ID) int {
			switch pa, pb := pos(a), pos(b); {
			case pa < pb:
				return -1 // NSOrderedAscending
			case pa > pb:
				return 1
			}
			return 0
		}},
		{Cmd: sel("offsetFromPosition:toPosition:"), Fn: func(_ objc.ID, _ objc.SEL, a, b objc.ID) int { return pos(b) - pos(a) }},
		{Cmd: sel("positionWithinRange:farthestInDirection:"), Fn: func(_ objc.ID, _ objc.SEL, r objc.ID, direction int) objc.ID {
			if back(direction) {
				return textPosition(ranges[r][0])
			}
			return textPosition(ranges[r][1])
		}},
		{Cmd: sel("characterRangeByExtendingPosition:inDirection:"), Fn: func(self objc.ID, _ objc.SEL, p objc.ID, direction int) objc.ID {
			v := views[self]
			if v == nil {
				return 0
			}
			at := pos(p)
			if back(direction) {
				return textRange(at, max(at-1, 0))
			}
			return textRange(at, v.clampUnits(at+1))
		}},
		{Cmd: sel("baseWritingDirectionForPosition:inDirection:"), Fn: func(objc.ID, objc.SEL, objc.ID, int) int { return -1 }}, // natural
		{Cmd: sel("setBaseWritingDirection:forRange:"), Fn: func(objc.ID, objc.SEL, int, objc.ID) {}},

		// Geometry, in the view.
		{Cmd: sel("firstRectForRange:"), Fn: func(self objc.ID, _ objc.SEL, r objc.ID) apple.CGRect {
			if v := views[self]; v != nil {
				if rects := v.rangeRects(ranges[r][0], ranges[r][1]); len(rects) > 0 {
					return rects[0]
				}
			}
			return apple.CGRect{}
		}},
		{Cmd: sel("caretRectForPosition:"), Fn: func(self objc.ID, _ objc.SEL, p objc.ID) apple.CGRect {
			if v := views[self]; v != nil {
				return v.caretRect(pos(p), false)
			}
			return apple.CGRect{}
		}},
		{Cmd: sel("selectionRectsForRange:"), Fn: func(self objc.ID, _ objc.SEL, r objc.ID) objc.ID {
			array := objc.ID(apple.Class("NSMutableArray")).Send(selNew).Send(sel("autorelease"))
			v := views[self]
			if v == nil {
				return array
			}
			rects := v.rangeRects(ranges[r][0], ranges[r][1])
			for i, rect := range rects {
				sr := objc.ID(classSelectionRect).Send(selNew)
				selectionRects[sr] = selectionRect{rect: rect, start: i == 0, end: i == len(rects)-1}
				array.Send(sel("addObject:"), sr)
				sr.Send(selRelease)
			}
			return array
		}},
		{Cmd: sel("closestPositionToPoint:"), Fn: func(self objc.ID, _ objc.SEL, p apple.CGPoint) objc.ID {
			if v := views[self]; v != nil {
				return textPosition(v.closestOffset(p))
			}
			return textPosition(0)
		}},
		{Cmd: sel("closestPositionToPoint:withinRange:"), Fn: func(self objc.ID, _ objc.SEL, p apple.CGPoint, r objc.ID) objc.ID {
			at := 0
			if v := views[self]; v != nil {
				at = v.closestOffset(p)
			}
			o := ranges[r]
			return textPosition(min(max(at, o[0]), o[1]))
		}},
		{Cmd: sel("characterRangeAtPoint:"), Fn: func(self objc.ID, _ objc.SEL, p apple.CGPoint) objc.ID {
			v := views[self]
			if v == nil {
				return 0
			}
			at := v.closestOffset(p)
			return textRange(at, v.clampUnits(at+1))
		}},
	}
}

// updateOwnSelection gives v, a text, its own selection while it is
// selectable outside any area, and takes it away after.
func (v *view) updateOwnSelection() {
	interaction := apple.Class("UITextInteraction") // none on tvOS
	on := interaction != 0 && classTextPosition != 0 && v.kind == driver.KindText && v.selectable == driver.SelectableOn &&
		v.native.Send(sel("window")) != 0 && !v.inArea()
	switch {
	case on && v.tsel == nil:
		v.tsel = &textSelection{}
		v.tsel.interaction = objc.ID(interaction).Send(sel("textInteractionForMode:"), 1) // non-editable
		v.tsel.interaction.Send(sel("setTextInput:"), v.native)
		v.native.Send(sel("addInteraction:"), v.tsel.interaction)
	case !on && v.tsel != nil:
		v.resignText()
		v.native.Send(sel("removeInteraction:"), v.tsel.interaction)
		if v.tsel.tokenizer != 0 {
			v.tsel.tokenizer.Send(selRelease)
		}
		v.tsel = nil
	}
	if v.tsel != nil {
		// UIKit draws the handles in the tint and fades it for the
		// highlight.
		var tint objc.ID
		if v.selColor != 0 {
			tint = uiColor(v.selColor).Send(sel("colorWithAlphaComponent:"), 1.0)
		}
		v.native.Send(sel("setTintColor:"), tint)
	}
}

// inArea reports whether v is inside a selection area, which selects for
// it.
func (v *view) inArea() bool {
	for p := v.parent(); p != nil; p = p.parent() {
		if p.area {
			return true
		}
	}
	return false
}

// reachesEnds reports whether p, in v but outside its bounds, is near an
// end of its selection, where UIKit draws a handle whose knob stands out
// of the text: the handles' gestures are the view's, so it takes the
// touch, as within handleSlop of an area's handle.
func (v *view) reachesEnds(p apple.CGPoint) bool {
	t := v.tsel
	if t == nil || !t.has || t.start >= t.end {
		return false
	}
	for _, c := range []apple.CGRect{v.caretRect(t.start, false), v.caretRect(t.end, true)} {
		if p.X >= c.Origin.X-handleSlop && p.X <= c.Origin.X+c.Size.Width+handleSlop &&
			p.Y >= c.Origin.Y-handleSlop && p.Y <= c.Origin.Y+c.Size.Height+handleSlop {
			return true
		}
	}
	return false
}

// textReplaced follows a new text: a selection's offsets mean nothing in
// it.
func (v *view) textReplaced() {
	if v.tsel != nil && v.tsel.has {
		v.resignText()
	}
}

// resignText ends v's own selection, giving the first responder back to
// the view controller, which answers the keys and an area's Copy.
func (v *view) resignText() {
	if !objc.Send[bool](v.native, sel("isFirstResponder")) {
		return
	}
	v.native.Send(selResign)
	if c := v.surface.viewController(); c != 0 && !v.surface.embedded {
		c.Send(selBecomeFirst)
	}
}

// resignTouched resigns the selecting text when a touch goes down outside
// it, as an area's selection clears.
func (s *surface) resignTouched(touch objc.ID) {
	first := s.firstText()
	if first == nil {
		return
	}
	hit := touch.Send(selView)
	if hit != 0 && (hit == first.native || objc.Send[bool](hit, sel("isDescendantOfView:"), first.native)) {
		return
	}
	first.resignText()
}

// firstText returns the text that is first responder with its own
// selection, or nil.
func (s *surface) firstText() *view {
	for _, v := range s.ownTexts {
		if v.tsel != nil && v.tsel.has && objc.Send[bool](v.native, sel("isFirstResponder")) {
			return v
		}
	}
	return nil
}
