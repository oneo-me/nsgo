//go:build windows

package windows

import (
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/textselect"
)

// A selection area (a view with SelectionArea, or a selectable text
// outside one) selects with the mouse as on macOS: drag, double click a
// word, triple click a whole text, Shift-click to extend. While it holds
// a selection, Ctrl+C copies it, Ctrl+A selects the area and Escape
// clears it; a press anywhere else clears it, so the window shows one.
// The model is driven in surface points.

// defaultSelection is the highlight of a text without SelectionColor:
// Windows' accent blue, light enough to read the text through.
var defaultSelection = driver.RGBA(0x0078d7, 0.35)

// areaAt returns the selection area a press at p goes to, unless a field,
// a page, a pressable view or a pan inside it keeps the press.
func (w *window) areaAt(p driver.Point) *view {
	root := w.view(w.root)
	if root == nil {
		return nil
	}
	var under []*view
	for v := pick(root, p); v != nil; v = v.parent {
		under = append(under, v)
	}
	area, _ := textselect.AreaAt(under, func(v *view) textselect.Taker {
		switch {
		case v.kind == driver.KindInput || v.kind == driver.KindWeb || v.kind == driver.KindNative:
			return textselect.Keeps
		case v.events.Has(driver.EventPress) || v.events.Has(driver.EventLongPress) ||
			v.events.Has(driver.EventPressDown) || v.events.Has(driver.EventPan):
			return textselect.Keeps
		case v.props.SelectionArea:
			return textselect.Area
		case v.kind == driver.KindText && v.props.Selectable == driver.SelectableOn:
			return textselect.SelectableText
		}
		return textselect.Passes
	})
	return area
}

// listSelection finds the area's texts again, in document order, with
// their content boxes in the surface.
func (w *window) listSelection() {
	sl := w.textSelection
	sl.Texts = sl.Texts[:0]
	var walk func(v *view)
	walk = func(v *view) {
		if v.style.Hidden {
			return
		}
		if v.kind == driver.KindText {
			if l, ok := v.text.(driver.TextHitTester); ok && v.props.Selectable != driver.SelectableOff {
				b, _ := w.surfaceBounds(v)
				box := v.content
				box.Origin.X += b.Origin.X
				box.Origin.Y += b.Origin.Y
				sl.Texts = append(sl.Texts, textselect.Text{ID: v.id, Content: v.plain, Box: box, Layout: l})
			}
			return
		}
		for _, c := range v.children {
			walk(c)
		}
	}
	if area := w.view(sl.Area); area != nil {
		walk(area)
	}
}

// showSelection has each text draw its part of the selection.
func (w *window) showSelection() {
	ranges := map[driver.NodeID]driver.SelectionRange{}
	if w.textSelection != nil {
		ranges = w.textSelection.Ranges()
		w.tellSelection(w.textSelection, false)
	}
	for _, v := range w.selected {
		if _, ok := ranges[v.id]; !ok && !v.deleted && v.selEnd > v.selStart {
			v.selStart, v.selEnd = 0, 0
			w.redrawView(v)
		}
	}
	w.selected = w.selected[:0]
	for id, r := range ranges {
		if v := w.view(id); v != nil {
			if v.selStart != r.Start || v.selEnd != r.End {
				v.selStart, v.selEnd = r.Start, r.End
				w.redrawView(v)
			}
			w.selected = append(w.selected, v)
		}
	}
}

// tellSelection reports the selection's extent to its area.
func (w *window) tellSelection(sl *textselect.Selection, cleared bool) {
	area := w.view(sl.Area)
	if area == nil {
		return
	}
	b, _ := w.surfaceBounds(area)
	if ev, ok := sl.Tell(b.Origin, cleared); ok {
		w.report(area, ev)
	}
}

// selectPress starts or extends a selection on a left press at p, or
// clears the selection a press elsewhere leaves. It reports whether it
// took the press.
func (w *window) selectPress(p driver.Point, clicks int, shift bool) bool {
	area := w.areaAt(p)
	if area == nil {
		w.clearSelection()
		return false
	}
	if w.textSelection == nil || w.textSelection.Area != area.id {
		w.clearSelection()
		w.textSelection = &textselect.Selection{Area: area.id}
	}
	w.listSelection()
	w.textSelection.Model.Press(w.textSelection, p, clicks, shift)
	w.textSelection.Dragging = true
	w.showSelection()
	setCapture.Call(w.hwnd)
	return true
}

// selectDrag follows the drag of the press that started a selection.
func (w *window) selectDrag(p driver.Point) bool {
	sl := w.textSelection
	if sl == nil || !sl.Dragging {
		return false
	}
	w.listSelection()
	sl.Model.Drag(sl, p)
	w.showSelection()
	return true
}

// selectUp ends a selection's drag, reporting whether there was one.
func (w *window) selectUp() bool {
	if w.textSelection == nil || !w.textSelection.Dragging {
		return false
	}
	w.textSelection.Dragging = false
	releaseCapture.Call()
	return true
}

// selectKey answers a selection's keys: Ctrl+C, Ctrl+A and Escape.
func (w *window) selectKey(key uintptr) bool {
	sl := w.textSelection
	if sl == nil || w.focusedField() != nil || keyDown(vkMenu) || keyDown(vkShift) {
		return false
	}
	ctrl := keyDown(vkControl)
	switch {
	case ctrl && key == 'C':
		if !sl.Model.IsEmpty() {
			w.listSelection()
			w.setClipboard(sl.Model.Text(sl))
		}
	case ctrl && key == 'A':
		w.listSelection()
		sl.Model.SelectAll(sl)
		w.showSelection()
	case !ctrl && key == vkEscape && !sl.Model.IsEmpty():
		w.clearSelection()
	default:
		return false
	}
	return true
}

// refreshSelection follows the area's texts after a batch: the selection
// keeps to its texts, and goes with its area.
func (w *window) refreshSelection() {
	sl := w.textSelection
	if sl == nil {
		return
	}
	area := w.view(sl.Area)
	if area == nil || area.deleted {
		w.clearSelection()
		return
	}
	if _, shown := w.surfaceBounds(area); !shown {
		w.clearSelection()
		return
	}
	w.listSelection()
	sl.Model.TextsChanged(sl)
	w.showSelection()
}

func (w *window) clearSelection() {
	if w.textSelection == nil {
		return
	}
	w.tellSelection(w.textSelection, true)
	w.textSelection = nil
	w.showSelection()
}

// drawSelection fills the highlight of text v's selection behind its
// glyphs, its content box at x, y.
func (p *painter) drawSelection(v *view, x, y float32) {
	l, ok := v.text.(driver.TextHitTester)
	if !ok || v.selEnd <= v.selStart {
		return
	}
	c := v.props.SelectionColor
	if c == 0 {
		c = defaultSelection
	}
	for _, r := range l.RangeRects(v.selStart, v.selEnd, nil) {
		left, top := x+float32(r.Origin.X), y+float32(r.Origin.Y)
		p.fillRect(rectF{left, top, left + float32(r.Size.Width), top + float32(r.Size.Height)}, c)
	}
}
