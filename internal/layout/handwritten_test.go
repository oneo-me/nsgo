package layout

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// These are Taffy's hand-written tests (tests/hand_written) for the features
// NSH enables.

// newLeaf and newNode build trees as TaffyTree's new_leaf and
// new_with_children do.
func newLeaf(s Style) *testNode { return &testNode{style: s} }

func newNode(s Style, children ...*testNode) *testNode {
	n := &testNode{style: s, children: children}
	for _, c := range children {
		c.parent = n
	}
	return n
}

// setStyle replaces a node's style and marks it dirty, as TaffyTree's
// set_style does.
func (n *testNode) setStyle(s Style) {
	n.style = s
	n.markDirty()
}

// markDirty is TaffyTree::mark_dirty: it clears the caches of the node and
// its ancestors, stopping at one already clear.
func (n *testNode) markDirty() {
	for m := n; m != nil && m.state.Invalidate(); m = m.parent {
	}
}

// layout returns the node's final layout with rounding on, as TaffyTree's
// layout does by default.
func (n *testNode) layout() Layout { return n.state.Rounded }

// computeLayout is TaffyTree::compute_layout with rounding enabled.
func computeLayout(root *testNode, available Size[AvailableSpace]) {
	Compute(root, available)
	Round(root, 1)
}

func pxSize(w, h float32) Size[Length] { return Size[Length]{Px(w), Px(h)} }

func definiteSpace(v float32) Size[AvailableSpace] {
	return Size[AvailableSpace]{Definite(v), Definite(v)}
}

var displays = []Display{DisplayBlock, DisplayFlex, DisplayGrid}

func displayName(d Display) string {
	return [...]string{"Flex", "Block", "FlowRoot", "Grid", "None"}[d]
}

func TestRoundingDoesntLeaveGaps(t *testing.T) {
	square := pxSize(100.3, 100.3)
	a := newLeaf(Style{Size: square})
	b := newLeaf(Style{Size: square})
	root := newNode(Style{Size: pxSize(963.3333, 1000), JustifyContent: contentOf(ContentCenter)}, a, b)
	computeLayout(root, maxContentSpaces)
	la, lb := a.layout(), b.layout()
	if la.Location.X+la.Size.Width != lb.Location.X {
		t.Errorf("a ends at %v, b starts at %v", la.Location.X+la.Size.Width, lb.Location.X)
	}
}

func TestMinMaxOverrides(t *testing.T) {
	for _, tc := range []struct {
		name          string
		min, max      Size[Length]
		width, height float32
	}{
		{"min overrides max", pxSize(100, 100), pxSize(10, 10), 100, 100},
		{"max overrides size", Size[Length]{}, pxSize(10, 10), 10, 10},
		{"min overrides size", pxSize(100, 100), Size[Length]{}, 100, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := newLeaf(Style{Size: pxSize(50, 50), MinSize: tc.min, MaxSize: tc.max})
			computeLayout(child, definiteSpace(100))
			if got := child.layout().Size; got != (Size[float32]{tc.width, tc.height}) {
				t.Errorf("size %v, want %vx%v", got, tc.width, tc.height)
			}
		})
	}
}

// TestNegativeAvailableSpace checks that margins exceeding the container
// never hand a leaf negative sizes. Taffy inspects every LayoutInput reaching
// the leaf's measure function; here the leaf's Measure sees the same known
// dimensions and the available space after its own insets, which are zero.
func TestNegativeAvailableSpace(t *testing.T) {
	for _, tc := range []struct {
		name      string
		display   Display
		direction FlexDirection
	}{
		{"block", DisplayBlock, Row},
		{"flex column", DisplayFlex, Column},
		{"grid", DisplayGrid, Row},
	} {
		t.Run(tc.name, func(t *testing.T) {
			leaf := newLeaf(Style{Margin: Rect[Length]{Left: Px(30), Right: Px(10)}})
			var negative []string
			leaf.onMeasure = func(known Size[Opt], available Size[AvailableSpace]) {
				neg := func(o Opt) bool { return o.Valid && o.Value < 0 }
				negSpace := func(a AvailableSpace) bool { return a.isDefinite() && a.Value < 0 }
				if neg(known.Width) || neg(known.Height) || negSpace(available.Width) || negSpace(available.Height) {
					negative = append(negative, fmt.Sprintf("known=%v available=%v", known, available))
				}
			}
			root := newNode(Style{
				Display:             tc.display,
				FlexDirection:       tc.direction,
				Size:                Size[Length]{Width: Px(20)},
				GridTemplateColumns: []GridTemplateComponent{{Track: Track(Px(20))}},
			}, leaf)
			computeLayout(root, maxContentSpaces)
			if len(negative) > 0 {
				t.Errorf("leaf measured with negative inputs: %v", negative)
			}
			if w := leaf.layout().Size.Width; w != 0 {
				t.Errorf("leaf width %v, want 0", w)
			}
		})
	}
}

const (
	overflowContainer float32 = 200
	overflowChild     float32 = 50
)

func containerWithAbsoluteChild(display Display, overflow Overflow, direction Direction, left, right Opt) Layout {
	inset := Rect[Length]{Left: Auto, Right: Auto, Top: Px(0), Bottom: Auto}
	if left.Valid {
		inset.Left = Px(left.Value)
	}
	if right.Valid {
		inset.Right = Px(right.Value)
	}
	child := newLeaf(Style{Position: PositionAbsolute, Inset: inset, Size: pxSize(overflowChild, overflowChild)})
	node := newNode(Style{
		Display:   display,
		Direction: direction,
		Position:  PositionRelative,
		Size:      pxSize(overflowContainer, overflowContainer),
		Overflow:  Point[Overflow]{overflow, overflow},
	}, child)
	computeLayout(node, maxContentSpaces)
	return node.layout()
}

func nestedOverflowLayout(display Display, childOverflow Point[Overflow]) Layout {
	grandchild := newLeaf(Style{
		Position: PositionAbsolute,
		Inset:    Rect[Length]{Left: Px(0), Right: Auto, Top: Px(overflowContainer), Bottom: Auto},
		Size:     pxSize(overflowChild, overflowChild),
	})
	child := newNode(Style{
		Display:  display,
		Position: PositionRelative,
		Size:     pxSize(overflowContainer, overflowContainer),
		Overflow: childOverflow,
	}, grandchild)
	node := newNode(Style{Display: display, Size: pxSize(overflowContainer, overflowContainer)}, child)
	computeLayout(node, maxContentSpaces)
	return node.layout()
}

func TestScrollableOverflow(t *testing.T) {
	for _, d := range displays {
		t.Run(displayName(d), func(t *testing.T) {
			check := func(what string, got, want float32) {
				t.Helper()
				if got != want {
					t.Errorf("%s: %v, want %v", what, got, want)
				}
			}

			// Wholly unreachable boxes are excluded for scroll containers: the
			// child spans x -100 to -50.
			l := containerWithAbsoluteChild(d, OverflowScroll, LTR, Some(-100), None)
			if l.ScrollableOverflow != (Rect[float32]{}) {
				t.Errorf("unreachable: overflow %v, want zero", l.ScrollableOverflow)
			}
			check("unreachable scroll width", l.ScrollWidth(), 0)

			// A box that is not a scroll container has no unreachable region:
			// start-side overflow is captured.
			l = containerWithAbsoluteChild(d, OverflowVisible, LTR, Some(-100), None)
			check("non-scroll left", l.ScrollableOverflow.Left, -100)

			// A child straddling the scroll origin contributes its whole border
			// box, but only the reachable extent counts towards the scroll width.
			l = containerWithAbsoluteChild(d, OverflowScroll, LTR, Some(-25), None)
			check("partial left", l.ScrollableOverflow.Left, -25)
			check("partial right", l.ScrollableOverflow.Right, 25)
			check("partial scroll width", l.ScrollWidth(), 0)

			// End-side overflow is captured and scrollable.
			l = containerWithAbsoluteChild(d, OverflowScroll, LTR, Some(overflowContainer), None)
			check("end right", l.ScrollableOverflow.Right, overflowContainer+overflowChild)
			check("end scroll width", l.ScrollWidth(), overflowChild)

			// A container with only an absolute child includes its own end
			// padding when it is a scroll container.
			abs := newLeaf(Style{
				Position: PositionAbsolute,
				Inset:    Rect[Length]{Left: Px(0), Right: Auto, Top: Px(0), Bottom: Auto},
				Size:     pxSize(5, 5),
			})
			node := newNode(Style{
				Display:  d,
				Position: PositionRelative,
				Overflow: Point[Overflow]{OverflowScroll, OverflowScroll},
				Padding:  Rect[Length]{Right: Px(20), Bottom: Px(30)},
				Size:     pxSize(overflowContainer, overflowContainer),
			}, abs)
			computeLayout(node, maxContentSpaces)
			check("padding right", node.layout().ScrollableOverflow.Right, 20)
			check("padding bottom", node.layout().ScrollableOverflow.Bottom, 30)

			// A scroll container in one axis clips both axes.
			l = nestedOverflowLayout(d, Point[Overflow]{OverflowHidden, OverflowVisible})
			check("hidden clips bottom", l.ScrollableOverflow.Bottom, overflowContainer)

			// Clip in one axis does not clip the other.
			l = nestedOverflowLayout(d, Point[Overflow]{OverflowClip, OverflowVisible})
			check("clip bottom", l.ScrollableOverflow.Bottom, overflowContainer+overflowChild)

			// In RTL the scroll origin is the top-right corner: a child past the
			// right edge is wholly unreachable.
			l = containerWithAbsoluteChild(d, OverflowScroll, RTL, None, Some(-100))
			if l.ScrollableOverflow != (Rect[float32]{}) {
				t.Errorf("rtl unreachable: overflow %v, want zero", l.ScrollableOverflow)
			}
			check("rtl unreachable scroll width", l.ScrollWidth(), 0)

			// In RTL, overflow past the left edge is reachable end-side overflow.
			l = containerWithAbsoluteChild(d, OverflowScroll, RTL, Some(-overflowChild), None)
			check("rtl end right", l.ScrollableOverflow.Right, overflowContainer+overflowChild)
			check("rtl end scroll width", l.ScrollWidth(), overflowChild)
		})
	}
}

func TestSafeAlignment(t *testing.T) {
	grid100 := func(extra Style) Style {
		extra.Display = DisplayGrid
		extra.Size = pxSize(100, 100)
		if extra.GridTemplateRows == nil {
			extra.GridTemplateRows = []GridTemplateComponent{{Track: Track(Px(100))}}
		}
		extra.GridTemplateColumns = []GridTemplateComponent{{Track: Track(Px(100))}}
		return extra
	}
	safeEnd := Align{AlignEnd, Safe}
	for _, tc := range []struct {
		name   string
		child  Style
		parent Style
		x      bool // check x rather than y
		want   float32
	}{
		{"grid safe align-self falls back to start", Style{Size: pxSize(50, 150), AlignSelf: safeEnd},
			grid100(Style{}), false, 0},
		{"grid safe align-self is end without overflow", Style{Size: pxSize(50, 40), AlignSelf: safeEnd},
			grid100(Style{}), false, 60},
		{"grid unsafe align-self keeps overflowing position", Style{Size: pxSize(50, 150), AlignSelf: alignOf(AlignEnd)},
			grid100(Style{}), false, -50},
		{"grid safe justify-self falls back to start", Style{Size: pxSize(150, 50), JustifySelf: safeEnd},
			grid100(Style{}), true, 0},
		{"grid safe justify-self rtl falls back to rtl start", Style{Size: pxSize(150, 50), JustifySelf: safeEnd},
			grid100(Style{Direction: RTL}), true, -50},
		{"flex safe align-self falls back to start", Style{Size: pxSize(50, 150), AlignSelf: safeEnd},
			Style{Size: pxSize(100, 100)}, false, 0},
		{"flex unsafe align-self keeps overflowing position", Style{Size: pxSize(50, 150), AlignSelf: alignOf(AlignEnd)},
			Style{Size: pxSize(100, 100)}, false, -50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := newLeaf(tc.child)
			root := newNode(tc.parent, child)
			computeLayout(root, maxContentSpaces)
			got := child.layout().Location.Y
			if tc.x {
				got = child.layout().Location.X
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("grid safe align-content falls back to start", func(t *testing.T) {
		a := newLeaf(Style{Size: pxSize(40, 80)})
		b := newLeaf(Style{Size: pxSize(40, 80)})
		root := newNode(Style{
			Display: DisplayGrid,
			Size:    pxSize(100, 100),
			GridTemplateRows: []GridTemplateComponent{
				{Track: Track(Px(80))}, {Track: Track(Px(80))},
			},
			GridTemplateColumns: []GridTemplateComponent{{Track: Track(Px(40))}},
			AlignContent:        AlignContent{ContentEnd, Safe},
		}, a, b)
		computeLayout(root, maxContentSpaces)
		if y := a.layout().Location.Y; y != 0 {
			t.Errorf("first track at %v, want 0", y)
		}
	})
	t.Run("flex safe justify-content falls back to start", func(t *testing.T) {
		a := newLeaf(Style{Size: pxSize(80, 50)})
		b := newLeaf(Style{Size: pxSize(80, 50)})
		root := newNode(Style{Size: pxSize(100, 100), JustifyContent: AlignContent{ContentEnd, Safe}}, a, b)
		computeLayout(root, maxContentSpaces)
		if x := a.layout().Location.X; x != 0 {
			t.Errorf("first item at %v, want 0", x)
		}
	})
	t.Run("flex safe align-content falls back to start on multi-line overflow", func(t *testing.T) {
		a := newLeaf(Style{Size: pxSize(60, 80)})
		b := newLeaf(Style{Size: pxSize(60, 80)})
		root := newNode(Style{FlexWrap: Wrap, Size: pxSize(100, 100), AlignContent: AlignContent{ContentEnd, Safe}}, a, b)
		computeLayout(root, maxContentSpaces)
		if y := a.layout().Location.Y; y != 0 {
			t.Errorf("first line at %v, want 0", y)
		}
	})
}

func TestOofHoisting(t *testing.T) {
	leafStyle := func(w, h float32) Style { return Style{Size: pxSize(w, h)} }
	pt := func(x, y float32) Point[float32] { return Point[float32]{x, y} }
	margin := func(left, top float32) Rect[Length] {
		return Rect[Length]{Left: Px(left), Top: Px(top), Right: Auto, Bottom: Auto}
	}
	inset := func(left, top float32) Rect[Length] {
		return Rect[Length]{Left: Px(left), Top: Px(top), Right: Auto, Bottom: Auto}
	}
	sameNodes := func(got []Node, want ...*testNode) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != Node(want[i]) {
				return false
			}
		}
		return true
	}

	t.Run("absolute skips static parent", func(t *testing.T) {
		for _, d := range displays {
			abs := newLeaf(Style{Position: PositionAbsolute, Inset: inset(10, 20), Size: pxSize(30, 30)})
			staticParent := newNode(Style{Display: d, Margin: margin(40, 40), Size: pxSize(50, 50)}, abs)
			cb := newNode(Style{Display: d, Position: PositionRelative, Size: pxSize(200, 200)}, staticParent)
			computeLayout(cb, maxContentSpaces)
			if l := abs.layout(); l.Location != pt(10, 20) || l.Size != (Size[float32]{30, 30}) {
				t.Errorf("%s: location %v size %v", displayName(d), l.Location, l.Size)
			}
		}
	})

	t.Run("hoisted children are in document order", func(t *testing.T) {
		for _, d := range displays {
			absStyle := Style{Position: PositionAbsolute, Size: pxSize(10, 10)}
			inflow := Style{Display: d, Size: pxSize(20, 20)}
			directA := newLeaf(absStyle)
			nestedB := newLeaf(absStyle)
			staticB := newNode(inflow, nestedB)
			nestedC := newLeaf(absStyle)
			staticC := newNode(inflow, nestedC)
			directD := newLeaf(absStyle)
			nestedE := newLeaf(absStyle)
			staticE := newNode(inflow, nestedE)
			cb := newNode(Style{
				Display:       d,
				Position:      PositionRelative,
				FlexWrap:      WrapReverse,
				FlexDirection: RowReverse,
				Size:          pxSize(50, 200),
			}, directA, staticB, staticC, directD, staticE)
			computeLayout(cb, maxContentSpaces)
			if !sameNodes(cb.state.Hoisted(), directA, nestedB, nestedC, directD, nestedE) {
				t.Errorf("%s: hoisted children out of document order", displayName(d))
			}
		}
	})

	t.Run("static position is containing block relative", func(t *testing.T) {
		for _, d := range displays {
			abs := newLeaf(Style{Position: PositionAbsolute, Size: pxSize(30, 30)})
			sibling := newLeaf(leafStyle(50, 50))
			staticParent := newNode(Style{Display: d, Margin: margin(40, 40), Size: pxSize(100, 100)}, sibling, abs)
			cb := newNode(Style{Display: d, Position: PositionRelative, Size: pxSize(200, 200)}, staticParent)
			computeLayout(cb, maxContentSpaces)
			// Block places it below the in-flow sibling; flex and grid as if it
			// were the sole item, at the content box's start.
			want := pt(40, 40)
			if d == DisplayBlock {
				want = pt(40, 90)
			}
			if l := abs.layout(); l.Location != want {
				t.Errorf("%s: location %v, want %v", displayName(d), l.Location, want)
			}
		}
	})

	t.Run("fixed hoists to root", func(t *testing.T) {
		fixed := newLeaf(Style{Position: PositionFixed, Inset: inset(5, 5), Size: pxSize(10, 10)})
		parent := newNode(Style{Position: PositionRelative, Margin: margin(50, 50), Size: pxSize(50, 50)}, fixed)
		root := newNode(leafStyle(200, 200), parent)
		computeLayout(root, maxContentSpaces)
		if l := fixed.layout(); l.Location != pt(5, 5) {
			t.Errorf("location %v", l.Location)
		}
	})

	t.Run("absolute within absolute", func(t *testing.T) {
		inner := newLeaf(Style{
			Position: PositionAbsolute,
			Inset:    Rect[Length]{Right: Px(0), Bottom: Px(0), Left: Auto, Top: Auto},
			Size:     pxSize(10, 10),
		})
		outer := newNode(Style{Position: PositionAbsolute, Inset: inset(20, 20), Size: pxSize(50, 50)}, inner)
		root := newNode(leafStyle(200, 200), outer)
		computeLayout(root, maxContentSpaces)
		if l := outer.layout(); l.Location != pt(20, 20) {
			t.Errorf("outer location %v", l.Location)
		}
		if l := inner.layout(); l.Location != pt(40, 40) {
			t.Errorf("inner location %v", l.Location)
		}
	})

	t.Run("fixed within absolute hoists to root", func(t *testing.T) {
		fixed := newLeaf(Style{Position: PositionFixed, Inset: inset(1, 2), Size: pxSize(10, 10)})
		abs := newNode(Style{Position: PositionAbsolute, Inset: inset(20, 20), Size: pxSize(50, 50)}, fixed)
		cb := newNode(Style{Position: PositionRelative, Margin: margin(30, 30), Size: pxSize(100, 100)}, abs)
		root := newNode(leafStyle(200, 200), cb)
		computeLayout(root, maxContentSpaces)
		if l := abs.layout(); l.Location != pt(20, 20) {
			t.Errorf("abs location %v", l.Location)
		}
		if l := fixed.layout(); l.Location != pt(1, 2) {
			t.Errorf("fixed location %v", l.Location)
		}
	})

	t.Run("hoisting survives cache hits", func(t *testing.T) {
		abs := newLeaf(Style{Position: PositionAbsolute, Inset: inset(10, 20), Size: pxSize(30, 30)})
		ancestor := newNode(leafStyle(50, 50), abs)
		for range 3 {
			ancestor = newNode(leafStyle(50, 50), ancestor)
		}
		cb := newNode(Style{Position: PositionRelative, Size: pxSize(200, 200)}, ancestor)
		check := func(step string, want Point[float32]) {
			t.Helper()
			if l := abs.layout(); l.Location != want {
				t.Errorf("%s: location %v, want %v", step, l.Location, want)
			}
		}
		computeLayout(cb, maxContentSpaces)
		check("first", pt(10, 20))
		computeLayout(cb, maxContentSpaces)
		check("cached", pt(10, 20))
		cb.markDirty()
		computeLayout(cb, maxContentSpaces)
		check("root dirty", pt(10, 20))
		s := abs.style
		s.Inset.Left = Px(15)
		abs.setStyle(s)
		computeLayout(cb, maxContentSpaces)
		check("restyled", pt(15, 20))
	})

	t.Run("cached root does not duplicate hoisted children", func(t *testing.T) {
		fixed := newLeaf(Style{Position: PositionFixed, Size: pxSize(30, 30)})
		inner := newNode(leafStyle(50, 50), fixed)
		root := newNode(leafStyle(200, 200), inner)
		for i, dirty := range []bool{false, false, true} {
			if dirty {
				root.markDirty()
			}
			computeLayout(root, maxContentSpaces)
			if !sameNodes(root.state.Hoisted(), fixed) {
				t.Errorf("pass %d: hoisted %d children, want the fixed box once", i, len(root.state.Hoisted()))
			}
		}
	})

	t.Run("rounding uses containing block offsets", func(t *testing.T) {
		abs := newLeaf(Style{Position: PositionAbsolute, Inset: inset(10.6, 10.6), Size: pxSize(10, 10)})
		staticParent := newNode(leafStyle(50, 50), abs)
		cb := newNode(Style{Position: PositionRelative, Margin: margin(20.3, 20.3), Size: pxSize(100, 100)},
			staticParent)
		root := newNode(leafStyle(200, 200), cb)
		computeLayout(root, maxContentSpaces)
		// cb rounds to 20; abs at a cumulative 20.3+10.6=30.9 rounds to 31,
		// which is 11 relative to cb.
		if l := cb.layout(); l.Location != pt(20, 20) {
			t.Errorf("cb location %v", l.Location)
		}
		if l := abs.layout(); l.Location != pt(11, 11) {
			t.Errorf("abs location %v", l.Location)
		}
	})

	t.Run("claim change relayouts with hot cache", func(t *testing.T) {
		abs := newLeaf(Style{Position: PositionAbsolute, Inset: inset(10, 20), Size: pxSize(30, 30)})
		middleStyle := Style{Margin: margin(40, 40), Size: pxSize(50, 50)}
		middle := newNode(middleStyle, abs)
		cb := newNode(Style{Position: PositionRelative, Size: pxSize(200, 200)}, middle)
		computeLayout(cb, maxContentSpaces)
		if l := abs.layout(); l.Location != pt(10, 20) {
			t.Errorf("static middle: abs location %v", l.Location)
		}
		positioned := middleStyle
		positioned.Position = PositionRelative
		middle.setStyle(positioned)
		computeLayout(cb, maxContentSpaces)
		if l := abs.layout(); l.Location != pt(10, 20) {
			t.Errorf("positioned middle: abs location %v", l.Location)
		}
		if l := middle.layout(); l.Location != pt(40, 40) {
			t.Errorf("positioned middle: middle location %v", l.Location)
		}
		middle.setStyle(middleStyle)
		computeLayout(cb, maxContentSpaces)
		if l := abs.layout(); l.Location != pt(10, 20) {
			t.Errorf("static again: abs location %v", l.Location)
		}
		if l := middle.layout(); l.Location != pt(40, 40) {
			t.Errorf("static again: middle location %v", l.Location)
		}
	})

	t.Run("fixed claim change relayouts with hot cache", func(t *testing.T) {
		fixed := newLeaf(Style{Position: PositionFixed, Inset: inset(5, 5), Size: pxSize(10, 10)})
		abs := newNode(Style{Position: PositionAbsolute, Inset: inset(50, 50), Size: pxSize(80, 80)}, fixed)
		root := newNode(leafStyle(200, 200), abs)
		computeLayout(root, maxContentSpaces)
		if l := fixed.layout(); l.Location != pt(5, 5) {
			t.Errorf("fixed location %v", l.Location)
		}
		if l := abs.layout(); l.Location != pt(50, 50) {
			t.Errorf("abs location %v", l.Location)
		}
		s := fixed.style
		s.Inset.Left = Px(7)
		fixed.setStyle(s)
		computeLayout(root, maxContentSpaces)
		if l := fixed.layout(); l.Location != pt(7, 5) {
			t.Errorf("restyled fixed location %v", l.Location)
		}
	})

	t.Run("content change inside hoisted subtree", func(t *testing.T) {
		inner := newLeaf(leafStyle(10, 10))
		abs := newNode(Style{Position: PositionAbsolute, Inset: inset(10, 10)}, inner)
		staticParent := newNode(leafStyle(50, 50), abs)
		cb := newNode(Style{Position: PositionRelative, Size: pxSize(200, 200)}, staticParent)
		computeLayout(cb, maxContentSpaces)
		if s := abs.layout().Size; s != (Size[float32]{10, 10}) {
			t.Errorf("size %v", s)
		}
		inner.setStyle(leafStyle(20, 30))
		computeLayout(cb, maxContentSpaces)
		if s := abs.layout().Size; s != (Size[float32]{20, 30}) {
			t.Errorf("resized: size %v", s)
		}
	})
}

// TestAdversarialStyles checks that degenerate or hostile styles terminate
// without panicking.
func TestAdversarialStyles(t *testing.T) {
	nan := float32(math.NaN())
	gridOf := func(s Style, children ...*testNode) *testNode {
		s.Display = DisplayGrid
		return newNode(s, children...)
	}

	t.Run("non-finite flex factor terminates", func(t *testing.T) {
		for _, f := range []float32{nan, inf32, negInf32} {
			child := newLeaf(Style{Size: pxSize(10, 10)})
			root := gridOf(Style{GridTemplateColumns: []GridTemplateComponent{{Track: MinMax(Auto, Fr(f))}}}, child)
			computeLayout(root, definiteSpace(100))
		}
	})

	t.Run("non-finite grid container size terminates", func(t *testing.T) {
		for _, w := range []float32{nan, inf32} {
			child := newLeaf(Style{})
			root := gridOf(Style{
				Size: Size[Length]{Px(w), Px(100)},
				GridTemplateColumns: []GridTemplateComponent{
					{Repeat: &Repetition{Count: 2, Tracks: []TrackSizing{Track(Fr(1))}}},
				},
			}, child)
			computeLayout(root, definiteSpace(100))
		}
	})

	t.Run("repetition without line names resolves lines", func(t *testing.T) {
		child := newLeaf(Style{
			Size:       pxSize(5, 5),
			GridColumn: Line[GridPlacement]{Start: GridPlacement{Kind: PlacementLine, Line: 1, Name: "c"}},
		})
		root := gridOf(Style{
			Size: pxSize(100, 100),
			GridTemplateColumns: []GridTemplateComponent{
				{Repeat: &Repetition{Count: 2, Tracks: []TrackSizing{Track(Px(10))}}},
				{Track: Track(Px(10))},
			},
			GridTemplateColumnNames: [][]string{nil, nil, {"c"}},
		}, child)
		computeLayout(root, definiteSpace(100))
		// "c" is the 4th line: the line after the three 10px columns.
		if x := child.layout().Location.X; x != 30 {
			t.Errorf("x = %v, want 30", x)
		}
	})

	t.Run("zero span and named span terminates", func(t *testing.T) {
		for _, dir := range []Direction{LTR, RTL} {
			child := newLeaf(Style{GridColumn: Line[GridPlacement]{
				Start: GridPlacement{Kind: PlacementSpan, Span: 1, Name: "does-not-exist"},
				End:   GridSpan(0),
			}})
			root := gridOf(Style{Direction: dir}, child)
			computeLayout(root, definiteSpace(100))
		}
	})

	t.Run("repetition with wrong line name set count panics", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil || !strings.Contains(fmt.Sprint(r), "line name sets") {
				t.Errorf("recovered %v, want a panic about line name sets", r)
			}
		}()
		child := newLeaf(Style{})
		root := gridOf(Style{
			GridTemplateColumns: []GridTemplateComponent{{Repeat: &Repetition{
				Count:     2,
				Tracks:    []TrackSizing{Track(Px(10))},
				LineNames: [][]string{{"a"}},
			}}},
			GridTemplateColumnNames: [][]string{nil, nil},
		}, child)
		computeLayout(root, definiteSpace(100))
	})

	t.Run("small item in max size occupied grid terminates", func(t *testing.T) {
		big := newLeaf(Style{
			GridRow:    Line[GridPlacement]{Start: GridSpan(10000)},
			GridColumn: Line[GridPlacement]{Start: GridSpan(10000)},
		})
		small := newLeaf(Style{
			GridRow:    Line[GridPlacement]{Start: GridSpan(10000)},
			GridColumn: Line[GridPlacement]{Start: GridSpan(1)},
		})
		root := gridOf(Style{Size: pxSize(100, 100)}, big, small)
		computeLayout(root, definiteSpace(100))
	})

	t.Run("extreme placements and spans terminate", func(t *testing.T) {
		var children []*testNode
		for _, p := range []Line[GridPlacement]{
			{Start: GridPlacement{}, End: GridSpan(2000)},
			{Start: GridLine(-2000), End: GridLine(2000)},
		} {
			children = append(children, newLeaf(Style{GridRow: p, GridColumn: p}))
		}
		root := gridOf(Style{Size: pxSize(100, 100)}, children...)
		computeLayout(root, definiteSpace(100))
	})
}
