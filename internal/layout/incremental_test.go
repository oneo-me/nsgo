package layout

import "testing"

// word is a leaf whose content is width points wide and 10 tall, which
// counts its measurements.
func word(width *float32) *testNode {
	n := newLeaf(Style{FlexShrink: Some(0)})
	n.content = func(Size[Opt], Size[AvailableSpace]) Size[float32] { return Size[float32]{*width, 10} }
	return n
}

// cellRow is a row of a cell holding a word, and a second cell after it.
func cellRow(cell Style, width *float32) (root, c, leaf, next *testNode) {
	leaf = word(width)
	c = newNode(cell, leaf)
	next = newLeaf(Style{Size: pxSize(20, 20), FlexShrink: Some(0)})
	root = newNode(Style{Display: DisplayFlex, Size: pxSize(300, 100)}, c, next)
	computeLayout(root, definiteSpace(300))
	return root, c, leaf, next
}

var fixedCell = Style{Display: DisplayFlex, Position: PositionRelative, Size: pxSize(60, 30), FlexShrink: Some(0)}

func TestABoundaryRelaysOutAlone(t *testing.T) {
	width := float32(20)
	root, cell, leaf, _ := cellRow(fixedCell, &width)
	if !Boundary(cell, root.Style()) {
		t.Fatal("a fixed cell in a row is no boundary")
	}
	width = 40
	leaf.state.Invalidate()
	cell.state.Invalidate()
	if !Relayout(cell) {
		t.Fatal("a fixed cell gave back another output for a wider word")
	}
	RoundFrom(cell, root.state.Unrounded.Location, 1)
	if w := leaf.layout().Size.Width; w != 40 {
		t.Errorf("the word is %v wide, want 40", w)
	}
}

func TestWhatIsNoBoundary(t *testing.T) {
	width := float32(20)
	for name, s := range map[string]Style{
		"content-sized": {Display: DisplayFlex, FlexShrink: Some(0)},
		"shrinking":     {Display: DisplayFlex, Size: pxSize(60, 30)},
		"a percentage":  {Display: DisplayFlex, Size: Size[Length]{Percent(0.5), Px(30)}, FlexShrink: Some(0)},
		"a flex basis":  {Display: DisplayFlex, Size: pxSize(60, 30), FlexShrink: Some(0), FlexBasis: Px(10)},
		"by baseline":   {Display: DisplayFlex, Size: pxSize(60, 30), FlexShrink: Some(0), AlignSelf: Align{Keyword: AlignBaseline}},
		"out of flow":   {Display: DisplayFlex, Size: pxSize(60, 30), FlexShrink: Some(0), Position: PositionAbsolute},
	} {
		s.Position = max(s.Position, PositionRelative)
		root, cell, _, _ := cellRow(s, &width)
		if Boundary(cell, root.Style()) {
			t.Errorf("a cell %s is a boundary", name)
		}
	}
	if Boundary(newLeaf(fixedCell), &Style{Display: DisplayFlex}) {
		t.Error("a cell never laid out is a boundary")
	}
}

func TestABoundaryWhoseOverflowChangesTellsItsParent(t *testing.T) {
	width := float32(20)
	_, cell, leaf, _ := cellRow(Style{Display: DisplayFlex, Size: pxSize(60, 30), FlexShrink: Some(0), Position: PositionRelative}, &width)
	width = 200
	leaf.state.Invalidate()
	cell.state.Invalidate()
	if Relayout(cell) {
		t.Error("a cell its word now overflows gave back what it gave before")
	}
}

func TestALeafThatMeasuresAsBefore(t *testing.T) {
	width := float32(20)
	_, _, leaf, _ := cellRow(Style{Display: DisplayFlex, FlexShrink: Some(0)}, &width)
	if !MeasuresAsBefore(leaf) {
		t.Error("an unchanged word does not measure as before")
	}
	width = 30
	if MeasuresAsBefore(leaf) {
		t.Error("a wider word measures as before")
	}
	leaf.state.Invalidate()
	if !MeasuresAsBefore(leaf) {
		t.Error("an invalidated leaf, which nothing was computed from, does not measure as before")
	}
}
