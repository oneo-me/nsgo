package layout

import "slices"

// Incremental layout, as NSH's layout tree does it over Taffy's
// algorithms. A change inside a relayout boundary (Boundary) lays out
// only the boundary's subtree, with what its parent gave it last time
// (Relayout); a leaf whose content changed but measures as before
// (MeasuresAsBefore) lays out nothing. A large page with values changing
// often then costs what changes, not what the page is.

// Boundary reports whether nothing inside n can change what its parent's
// layout takes from it, parent being the style of the node laying it
// out: n is an in-flow flex item with a size of fixed points both ways,
// so everything the flex algorithm takes from it follows from that size.
// Its flex basis is auto, its main size (a percentage basis against a
// parent of no definite size, or another length under a content-based
// minimum, would follow its content); it does not shrink (its automatic
// minimum, its content's, only matters to shrinking); growing takes the
// free space its parent and siblings leave; it is not aligned by its
// baseline. And its parent laid it out before. What it gives back that
// the size does not fix, its overflow, Relayout compares.
//
// An absolutely positioned n is one too when its size is its style's
// points and its place follows from its insets:
// its containing block takes nothing else from it, as what it overflows
// by is clipped or Relayout finds it the same.
func Boundary(n Node, parent *Style) bool {
	if !n.LayoutState().hasLast {
		return false
	}
	if s := n.Style(); s.Position == PositionAbsolute {
		return s.Display != DisplayNone && placedByInsets(s)
	}
	if parent.Display != DisplayFlex || parent.AlignItems.Keyword == AlignBaseline {
		return false
	}
	s := n.Style()
	return s.Display != DisplayNone && s.Position == PositionRelative &&
		s.Size.Width.Unit == UnitPx && s.Size.Height.Unit == UnitPx &&
		s.FlexShrink.Valid && s.FlexShrink.Value == 0 && s.FlexBasis.IsAuto() &&
		s.AlignSelf.Keyword != AlignBaseline
}

// placedByInsets reports whether an absolutely positioned box's size and
// place follow from its style alone: a size of points each way, which no
// limit changes, and an inset on each axis. A size its containing block
// stretches or a percentage would follow the block, which Relayout gives
// the size it had.
func placedByInsets(s *Style) bool {
	inset := func(l Length) bool { return l.Unit == UnitPx || l.Unit == UnitPercent }
	limit := func(l Length) bool { return l.Unit <= UnitAuto || l.Unit == UnitPx }
	in := s.Inset
	return s.Size.Width.Unit == UnitPx && s.Size.Height.Unit == UnitPx && s.AspectRatio == (Opt{}) &&
		(inset(in.Left) || inset(in.Right)) && (inset(in.Top) || inset(in.Bottom)) &&
		limit(s.MinSize.Width) && limit(s.MinSize.Height) && limit(s.MaxSize.Width) && limit(s.MaxSize.Height)
}

// Relayout lays boundary n out again with what its parent's last layout
// gave it, and reports whether it gave back what its parent takes from
// it as before: then its parent's layout stands, and its subtree needs
// only rounding (RoundFrom). Otherwise the caller lays its parent out
// again.
func Relayout(n Node) bool {
	st := n.LayoutState()
	if !st.hasLast {
		return false
	}
	passes.Add(1)
	before := st.lastOut
	after := computeNodeLayout(n, st.lastIn, nil)
	if !sameForParent(&after, &before) && !(clipsOverflow(n.Style()) && sameClipped(&after, &before)) {
		return false
	}
	// What its parent would have stored of it: its content's extent,
	// which its own scrolling follows.
	st.Unrounded.ScrollableOverflow = after.scrollableOverflow
	return true
}

// MeasuresAsBefore reports whether leaf n, whose content changed, gives
// every input layout asked of it since its cache was last cleared the
// answer it gave then: every layout above it was computed from those
// answers alone, so all of them stand and nothing needs laying out
// again. A leaf never asked (new, or invalidated already) measures as
// before; one asked too often to tell does not.
func MeasuresAsBefore(n Node) bool {
	st := n.LayoutState()
	if st.askedOver {
		return false
	}
	style := n.Style()
	for i := range st.asked {
		a := &st.asked[i]
		if out := computeLeafLayout(a.in, style, n); !sameOutput(&out, &a.out) {
			return false
		}
	}
	return true
}

// RoundFrom snaps the Unrounded layouts of n's subtree to whole pixels,
// as Round does for a whole tree, where origin is the unrounded position
// of the box n's location is relative to in its layout root.
func RoundFrom(n Node, origin Point[float32]) { roundLayout(n, origin.X, origin.Y) }

// sameOutput reports whether two outputs are the same.
func sameOutput(a, b *layoutOutput) bool {
	return a.scrollableOverflow == b.scrollableOverflow && sameForParent(a, b)
}

// clipsOverflow reports whether a box keeps its content's overflow from
// its parent on both axes (scrollableOverflowContribution).
func clipsOverflow(s *Style) bool {
	return s.Overflow.X != OverflowVisible && s.Overflow.Y != OverflowVisible
}

// sameClipped is sameForParent for a box whose overflow its parent never
// sees: all but the overflow.
func sameClipped(a, b *layoutOutput) bool {
	return a.size == b.size && a.baselines == b.baselines &&
		a.topMargin == b.topMargin && a.bottomMargin == b.bottomMargin &&
		a.marginsCanCollapseThrough == b.marginsCanCollapseThrough &&
		slices.EqualFunc(a.oofCandidates, b.oofCandidates, func(x, y oofCandidate) bool { return x == y })
}

// sameForParent reports whether two outputs give a parent the same: the
// same size, baselines, margins and out-of-flow boxes, and the same
// overflow past the box, which is all of the content's extent a parent
// scrolls to (scrollableOverflowContribution).
func sameForParent(a, b *layoutOutput) bool {
	past := func(o *layoutOutput) Rect[float32] {
		r := o.scrollableOverflow
		return Rect[float32]{Left: fmin(0, r.Left), Top: fmin(0, r.Top),
			Right: fmax(o.size.Width, r.Right), Bottom: fmax(o.size.Height, r.Bottom)}
	}
	return a.size == b.size && past(a) == past(b) && a.baselines == b.baselines &&
		a.topMargin == b.topMargin && a.bottomMargin == b.bottomMargin &&
		a.marginsCanCollapseThrough == b.marginsCanCollapseThrough &&
		slices.EqualFunc(a.oofCandidates, b.oofCandidates, func(x, y oofCandidate) bool { return x == y })
}
