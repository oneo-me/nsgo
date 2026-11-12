// Package textselect is what the Windows and Linux backends share of a
// selection across the texts of an area (driver.SelectionModel): the
// texts as the model reads them, hit-tested through their layouts.
package textselect

import "github.com/nsgo-dev/nsgo/internal/driver"

// Text is one text of an area: its node, its content as ItemText gave it,
// its content box in the coordinates the selection is driven in, and its
// layout.
type Text struct {
	ID      driver.NodeID
	Content string
	Box     driver.Bounds
	Layout  driver.TextHitTester
}

// Selection is the selection of one area: the model over the area's
// texts, listed again before each use, and whether the pointer that
// started it is still down.
type Selection struct {
	Area     driver.NodeID
	Texts    []Text
	Model    driver.SelectionModel
	Dragging bool
	// Told is the extent the area last reported.
	Told driver.Bounds
}

// Tell returns the event reporting the selection's extent to its area, at
// origin in the coordinates of its texts' boxes; false when it stayed. A
// cleared selection reports none.
func (s *Selection) Tell(origin driver.Point, cleared bool) (driver.Event, bool) {
	var b driver.Bounds
	if !cleared {
		b = s.Model.Extent(s)
	}
	if b.Size.Height > 0 {
		b.Origin.X -= origin.X
		b.Origin.Y -= origin.Y
	}
	if b == s.Told {
		return driver.Event{}, false
	}
	s.Told = b
	return driver.Event{Kind: driver.EventSelectionChange, ID: s.Area, Point: b.Origin, Size: b.Size}, true
}

// Len implements driver.SelectionTexts.
func (s *Selection) Len() int { return len(s.Texts) }

// Content implements driver.SelectionTexts.
func (s *Selection) Content(i int) string { return s.Texts[i].Content }

// Frame implements driver.SelectionTexts.
func (s *Selection) Frame(i int) driver.Bounds { return s.Texts[i].Box }

// Key implements driver.SelectionTexts.
func (s *Selection) Key(i int) uint64 { return uint64(s.Texts[i].ID) }

// OffsetAt implements driver.SelectionTexts.
func (s *Selection) OffsetAt(i int, p driver.Point) int {
	t := s.Texts[i]
	return t.Layout.HitTest(driver.Point{X: p.X - t.Box.Origin.X, Y: p.Y - t.Box.Origin.Y}).Offset
}

// Ranges returns what each text, by its ID, has selected.
func (s *Selection) Ranges() map[driver.NodeID]driver.SelectionRange {
	out := map[driver.NodeID]driver.SelectionRange{}
	for _, r := range s.Model.Ranges(s) {
		out[s.Texts[r.Text].ID] = r
	}
	return out
}

// Taker is what a view under a press is to a selection.
type Taker uint8

const (
	// Passes lets the press go on to the views around.
	Passes Taker = iota
	// Keeps takes the press itself: a field, a page, a pressable view or
	// one that follows pans.
	Keeps
	// Area is a selection area: a view with SelectionArea.
	Area
	// SelectableText is a text with SelectableOn outside any area.
	SelectableText
)

// AreaAt returns the selection area a press goes to among the views under
// it, innermost first, as Taker classifies them: the outermost area
// around the view hit, or the selectable text hit, unless a view inside
// keeps the press. ok is false when none takes it.
func AreaAt[V any](under []V, taker func(V) Taker) (area V, ok bool) {
	taken := false
	for _, v := range under {
		switch taker(v) {
		case Keeps:
			if !ok {
				taken = true
			}
		case Area:
			if ok || !taken {
				area, ok = v, true
			}
		case SelectableText:
			if !taken && !ok {
				area, ok = v, true
			}
		}
	}
	return area, ok
}
