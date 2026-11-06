package driver

import "testing"

type extentTexts []Bounds

func (t extentTexts) Len() int                { return len(t) }
func (t extentTexts) Content(int) string      { return "text" }
func (t extentTexts) Frame(i int) Bounds      { return t[i] }
func (t extentTexts) OffsetAt(int, Point) int { return 2 }
func (t extentTexts) Key(i int) uint64        { return uint64(i) }

func TestExtentBoxesTheTextsSelected(t *testing.T) {
	line := func(y Points) Bounds { return Bounds{Origin: Point{X: 4, Y: y}, Size: Size{Width: 100, Height: 18}} }
	texts := extentTexts{line(0), line(18), line(36), line(54)}
	var m SelectionModel
	if b := m.Extent(texts); b != (Bounds{}) {
		t.Fatalf("no selection: %+v", b)
	}
	m.Press(texts, Point{X: 10, Y: 20}, 1, false)
	if b := m.Extent(texts); b != (Bounds{}) {
		t.Fatalf("a press selects nothing: %+v", b)
	}
	m.Drag(texts, Point{X: 10, Y: 40})
	if b := m.Extent(texts); b.Origin.Y != 18 || b.Size.Height != 36 {
		t.Fatalf("two lines: %+v", b)
	}
}
