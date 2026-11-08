package layout

import (
	"math"
	"sync/atomic"
)

// Compute lays out the tree under root in the available space, which is
// usually the viewport, and stores every node's [State.Unrounded] layout.
// Nodes whose cache still holds a result for their inputs are not laid out
// again. Call [Round] afterwards to snap the layouts to whole pixels.
//
// The root's margin box is placed at the origin of the initial containing
// block, which has the available size; out-of-flow boxes with no positioned
// ancestor are positioned against it.
func Compute(root Node, available Size[AvailableSpace]) {
	passes.Add(1)
	style := root.Style()
	position := style.Position
	direction := style.Direction

	icb := spacesOpts(available)

	// When the root's layout is served from the cache its layout algorithm
	// does not run, so the hoisted children it recorded on a previous run
	// (including those the root positioning pass below added) are still in
	// place and must not be added again.
	rootIsCached := false

	var layout Layout
	var candidates []oofCandidate
	if position.isOutOfFlow() && icb.Width.Valid && icb.Height.Valid {
		area := Size[float32]{icb.Width.Value, icb.Height.Value}
		// The static position of the root places its margin box at the origin
		// of the initial containing block.
		start, edge := float32(0), edgeStart
		if direction.isRTL() {
			start, edge = area.Width, edgeEnd
		}
		candidate := oofCandidate{
			node:     root,
			position: position,
			staticPosition: Point[axisStaticPosition]{
				X: axisStaticPosition{area: Line[float32]{start, start}, align: staticAlignOf(edge)},
				Y: staticPositionFromEdge(0, edgeStart),
			},
		}
		box := layoutOofBox(candidate, area, Point[float32]{}, direction,
			// The initial containing block is not a scroll container.
			Point[Overflow]{}, nil, func(in *layoutInput) {
				rootIsCached = root.LayoutState().cache.hasFinal && root.LayoutState().cache.hasFinalFor(in)
			})
		layout, candidates = box.layout, box.surfaced
	} else {
		layout, candidates = computeInFlowRootLayout(root, available, &rootIsCached)
	}

	// The initial containing block is the containing block for fixed boxes,
	// and for absolute boxes with no positioned ancestor.
	if len(candidates) == 0 {
		return
	}
	overflow := style.Overflow

	// The initial containing block has the size of the viewport (the
	// available space) and is anchored at the canvas origin. On an axis
	// where the available space is indefinite, fall back to the root's
	// padding box.
	areaInset := rectAdd(layout.Border, Rect[float32]{Right: layout.ScrollbarSize.Width, Bottom: layout.ScrollbarSize.Height})
	paddingBox := sizeSub(layout.Size, rectSumAxes(areaInset))
	var areaSize Size[float32]
	var areaOffset Point[float32]
	if icb.Width.Valid {
		areaSize.Width = fmax(icb.Width.Value-layout.ScrollbarSize.Width, 0)
		areaOffset.X = -layout.Location.X
	} else {
		areaSize.Width = paddingBox.Width
		areaOffset.X = areaInset.Left
	}
	if icb.Height.Valid {
		areaSize.Height = fmax(icb.Height.Value-layout.ScrollbarSize.Height, 0)
		areaOffset.Y = -layout.Location.Y
	} else {
		areaSize.Height = paddingBox.Height
		areaOffset.Y = areaInset.Top
	}

	var hoisted []Node
	var unclaimed []oofCandidate
	performOofLayout(root, candidates, areaSize, areaOffset, direction, claimAll, overflow, &hoisted, &unclaimed)
	if !rootIsCached {
		st := root.LayoutState()
		st.hoisted = append(st.hoisted, hoisted...)
	}
}

func computeInFlowRootLayout(root Node, available Size[AvailableSpace], rootIsCached *bool) (Layout, []oofCandidate) {
	style := root.Style()
	var known Size[Opt]

	// The available space passed to the root excludes the root's margins.
	availableWidth := available.Width.intoOption()
	margin := rectResolveOrZero(style.margin(), availableWidth)
	rootAvailable := spacesMaxF(spacesSubF(available, rectSumAxes(margin)), Size[float32]{})

	parentSize := spacesOpts(available)
	if style.isBlock() {
		aspectRatio := style.AspectRatio
		padding := rectResolveOrZero(style.padding(), parentSize.Width)
		border := rectResolveOrZero(style.border(), parentSize.Width)
		paddingBorder := rectSumAxes(rectAdd(padding, border))
		var boxSizingAdjustment Size[float32]
		if style.BoxSizing == ContentBox {
			boxSizingAdjustment = paddingBorder
		}

		minSize := optsAddF(optsAspectRatio(sizeMaybeResolve(style.MinSize, parentSize), aspectRatio), boxSizingAdjustment)
		maxSize := optsAddF(optsAspectRatio(sizeMaybeResolve(style.MaxSize, parentSize), aspectRatio), boxSizingAdjustment)
		clampedStyleSize := optsClamp(
			optsAddF(optsAspectRatio(sizeMaybeResolve(style.Size, parentSize), aspectRatio), boxSizingAdjustment),
			minSize, maxSize)

		// If both min and max on an axis are set and max <= min then they
		// determine the size on that axis.
		minMaxDefinite := minMaxDefiniteSize(minSize, maxSize)

		// Block nodes stretch to fill definite available width.
		availableBased := Size[Opt]{Width: rootAvailable.Width.intoOption()}

		known = optsMaxF(optsOr(optsOr(optsOr(known, minMaxDefinite), clampedStyleSize), availableBased), paddingBorder)
	}

	in := layoutInput{
		knownDimensions:            known,
		knownDimensionsAreDefinite: Size[bool]{true, true},
		parentSize:                 parentSize,
		availableSpace:             rootAvailable,
		sizingMode:                 inherentSize,
		axis:                       requestBoth,
		runMode:                    performLayout,
	}
	*rootIsCached = root.LayoutState().cache.hasFinalFor(&in)

	output := computeChildLayout(root, in)

	padding := rectResolveOrZero(style.padding(), availableWidth)
	border := rectResolveOrZero(style.border(), availableWidth)
	var scrollbar Size[float32]
	if style.Overflow.Y == OverflowScroll {
		scrollbar.Width = style.ScrollbarWidth
	}
	if style.Overflow.X == OverflowScroll {
		scrollbar.Height = style.ScrollbarWidth
	}
	rtl := style.Direction.isRTL()

	// The root's margin box is placed at the origin of the initial
	// containing block.
	location := Point[float32]{X: margin.Left, Y: margin.Top}
	if rtl && availableWidth.Valid {
		location.X = availableWidth.Value - output.size.Width - margin.Right
	}

	// A relatively positioned root is offset by its insets, resolved against
	// the initial containing block.
	if style.Position == PositionRelative {
		left := style.Inset.Left.maybeResolve(parentSize.Width)
		right := style.Inset.Right.maybeResolve(parentSize.Width)
		top := style.Inset.Top.maybeResolve(parentSize.Height)
		bottom := style.Inset.Bottom.maybeResolve(parentSize.Height)
		negRight := Opt{-right.Value, right.Valid}
		if rtl {
			location.X += negRight.OrOpt(left).Or(0)
		} else {
			location.X += left.OrOpt(negRight).Or(0)
		}
		location.Y += top.OrOpt(Opt{-bottom.Value, bottom.Valid}).Or(0)
	}

	layout := Layout{
		Location:           location,
		Size:               output.size,
		ScrollableOverflow: output.scrollableOverflow,
		ScrollbarSize:      scrollbar,
		Padding:            padding,
		Border:             border,
		Margin:             margin,
	}
	root.LayoutState().Unrounded = layout
	return layout, output.oofCandidates
}

// minMaxDefiniteSize returns, per axis, the min size when both min and max
// are set and max <= min.
func minMaxDefiniteSize(min, max Size[Opt]) Size[Opt] {
	var s Size[Opt]
	if min.Width.Valid && max.Width.Valid && max.Width.Value <= min.Width.Value {
		s.Width = min.Width
	}
	if min.Height.Valid && max.Height.Valid && max.Height.Value <= min.Height.Value {
		s.Height = min.Height
	}
	return s
}

// hasFinalFor reports whether the cache holds the final layout for in.
func (c *cache) hasFinalFor(in *layoutInput) bool {
	return in.runMode == performLayout && c.hasFinal && c.mode == cacheModeOf(in) && c.finalKey == newCacheKey(in)
}

// computeChildLayout lays out or measures a node: Taffy's
// compute_child_layout with the dispatch of its TaffyTree.
func computeChildLayout(n Node, in layoutInput) layoutOutput {
	return computeNodeLayout(n, in, nil)
}

// computeNodeLayout is computeChildLayout for a block child, which shares
// its parent's block formatting context when bctx is set.
func computeNodeLayout(n Node, in layoutInput, bctx *blockContext) layoutOutput {
	// An ancestor is display: none.
	if in.runMode == performHiddenLayout {
		n.LayoutState().hasLast = false
		return computeHiddenLayout(n)
	}

	st := n.LayoutState()
	out, ok := st.cache.get(&in)
	if !ok {
		out = computeUncachedLayout(n, st, in, bctx)
		if in.runMode == performLayout {
			st.performed = passes.Load()
		}
	}
	if in.runMode == performLayout {
		st.lastIn, st.lastOut, st.hasLast = in, out.clone(), true
	}
	return out
}

// computeUncachedLayout is computeNodeLayout on a cache miss.
func computeUncachedLayout(n Node, st *State, in layoutInput, bctx *blockContext) layoutOutput {
	style := n.Style()
	var out layoutOutput
	switch {
	case style.Display == DisplayNone:
		out = computeHiddenLayout(n)
	case n.ChildCount() == 0:
		out = computeLeafLayout(in, style, n)
		st.ask(&in, &out)
	case style.Display == DisplayBlock:
		out = computeBlockLayout(n, in, bctx)
	case style.Display == DisplayFlowRoot:
		out = computeBlockLayout(n, in, nil)
	case style.Display == DisplayFlex:
		out = computeFlexboxLayout(n, in)
	case style.Display == DisplayGrid:
		out = computeGridLayout(n, in)
	}

	// Lay out the out-of-flow candidates this node is the containing block
	// for; the rest bubble up in out.oofCandidates. This runs before the
	// result is cached, so that cached outputs hold the processed list, and
	// only in full layout passes: measure passes must not write layouts of
	// hoisted boxes, which a cached final pass would not rewrite.
	if in.runMode == performLayout {
		computeOofLayout(n, &out)
	}

	st.cache.store(&in, &out)
	return out
}

// computeHiddenLayout zeroes the layout of a display: none node and its
// subtree, and drops their caches.
func computeHiddenLayout(n Node) layoutOutput {
	st := n.LayoutState()
	st.Invalidate()
	st.Unrounded = Layout{}
	st.performed = passes.Load()
	for i, count := 0, n.ChildCount(); i < count; i++ {
		computeChildLayout(n.Child(i), hiddenInput)
	}
	return layoutOutput{}
}

// measureChildSize returns a child's size on one axis.
func measureChildSize(n Node, known, parentSize Size[Opt], available Size[AvailableSpace], mode sizingMode,
	axis AbsoluteAxis, collapsible Line[bool]) float32 {
	out := computeChildLayout(n, layoutInput{
		knownDimensions:               known,
		knownDimensionsAreDefinite:    Size[bool]{true, true},
		parentSize:                    parentSize,
		availableSpace:                available,
		sizingMode:                    mode,
		axis:                          requestAxis(axis),
		runMode:                       computeSize,
		verticalMarginsAreCollapsible: collapsible,
	})
	return out.size.get(axis)
}

// measureChildSizeBoth returns a child's size on both axes.
func measureChildSizeBoth(n Node, known, parentSize Size[Opt], available Size[AvailableSpace], mode sizingMode,
	collapsible Line[bool]) Size[float32] {
	return computeChildLayout(n, layoutInput{
		knownDimensions:               known,
		knownDimensionsAreDefinite:    Size[bool]{true, true},
		parentSize:                    parentSize,
		availableSpace:                available,
		sizingMode:                    mode,
		axis:                          requestBoth,
		runMode:                       computeSize,
		verticalMarginsAreCollapsible: collapsible,
	}).size
}

// performChildLayout lays out a child's subtree.
func performChildLayout(n Node, known, parentSize Size[Opt], available Size[AvailableSpace], mode sizingMode,
	collapsible Line[bool]) layoutOutput {
	return computeChildLayout(n, layoutInput{
		knownDimensions:               known,
		knownDimensionsAreDefinite:    Size[bool]{true, true},
		parentSize:                    parentSize,
		availableSpace:                available,
		sizingMode:                    mode,
		axis:                          requestBoth,
		runMode:                       performLayout,
		verticalMarginsAreCollapsible: collapsible,
	})
}

// Round snaps the Unrounded layouts of the tree under root to device
// pixels of ratio pixels per point (0 is 1), storing them as
// [State.Rounded]. It rounds edges' cumulative positions rather than each
// size, so that adjacent boxes stay adjacent.
func Round(root Node, ratio float32) { roundLayout(root, 0, 0, pixels(ratio)) }

// pixels is ratio, with 0 and less meaning one pixel per point.
func pixels(ratio float32) float64 {
	if ratio <= 0 {
		return 1
	}
	return float64(ratio)
}

// pixel is the device pixel Round puts v on, counted from the root's
// origin. v is a float32 sum of offsets, a hair off the position it
// stands for, and dividing by a fractional ratio is never exact: snapped
// to a 64th of a pixel first, an edge on a half pixel (a 28 point row at
// 2.625 is 73.5 pixels) rounds the same way wherever a scroll puts it.
func pixel(v float32, ratio float64) float64 {
	p := float64(v) * ratio
	return math.Floor(math.Round(p*64)/64 + 0.5)
}

// span is the distance in points from from to to, each rounded to a
// device pixel, computed from the whole pixels between them: the same
// pixels give the same float32 wherever they are, so a frame that stays
// on its pixels compares equal as a scroll moves the edges.
func span(from, to float32, ratio float64) float32 {
	return float32((pixel(to, ratio) - pixel(from, ratio)) / ratio)
}

// sameSnap reports whether children carried from a and from b snap to
// the same pixels relative to them: a and b are equal, or a whole number
// of device pixels apart, as a box scrolled by whole pixels is
// (pixel(x+k) is pixel(x)+k).
func sameSnap(a, b Point[float32], ratio float64) bool {
	return a == b || frac(a.X, ratio) == frac(b.X, ratio) && frac(a.Y, ratio) == frac(b.Y, ratio)
}

// frac is where in its device pixel v falls, in 64ths.
func frac(v float32, ratio float64) float64 {
	p := math.Round(float64(v) * ratio * 64)
	return p - 64*math.Floor(p/64)
}

// passes counts layout passes (Compute, Relayout) across the process'
// engines; a node keeps the pass that last laid its children out.
var passes atomic.Uint64

func roundLayout(n Node, cumulativeX, cumulativeY float32, ratio float64) {
	st := n.LayoutState()
	u := st.Unrounded
	l := u

	parentX, parentY := cumulativeX, cumulativeY
	cumulativeX += u.Location.X
	cumulativeY += u.Location.Y

	x, y := cumulativeX, cumulativeY
	right, bottom := x+u.Size.Width, y+u.Size.Height
	l.Location.X = span(parentX, x, ratio)
	l.Location.Y = span(parentY, y, ratio)
	l.Size.Width = span(x, right, ratio)
	l.Size.Height = span(y, bottom, ratio)
	l.ScrollbarSize.Width = span(0, u.ScrollbarSize.Width, ratio)
	l.ScrollbarSize.Height = span(0, u.ScrollbarSize.Height, ratio)
	l.Border.Left = span(x, x+u.Border.Left, ratio)
	l.Border.Right = span(right-u.Border.Right, right, ratio)
	l.Border.Top = span(y, y+u.Border.Top, ratio)
	l.Border.Bottom = span(bottom-u.Border.Bottom, bottom, ratio)
	l.Padding.Left = span(x, x+u.Padding.Left, ratio)
	l.Padding.Right = span(right-u.Padding.Right, right, ratio)
	l.Padding.Top = span(y, y+u.Padding.Top, ratio)
	l.Padding.Bottom = span(bottom-u.Padding.Bottom, bottom, ratio)

	o := u.ScrollableOverflow
	l.ScrollableOverflow.Left = span(x, x+o.Left, ratio)
	l.ScrollableOverflow.Right = span(x, x+o.Right, ratio)
	l.ScrollableOverflow.Top = span(y, y+o.Top, ratio)
	l.ScrollableOverflow.Bottom = span(y, y+o.Bottom, ratio)

	st.Rounded = l

	// The children's layouts and where they are carried from are those
	// they were snapped with: their rounding stands.
	origin := Point[float32]{cumulativeX, cumulativeY}
	if st.kidsRounded && st.kidsPerformed == st.performed && st.kidsRatio == ratio &&
		sameSnap(st.kidsOrigin, origin, ratio) {
		return
	}
	st.kidsRounded, st.kidsPerformed, st.kidsOrigin, st.kidsRatio = true, st.performed, origin, ratio
	st.rounds++

	// In-flow children are rounded relative to this node. Out-of-flow ones
	// are rounded through their containing block's hoisted list instead, so
	// that each node is visited once and relative to the box its location is
	// relative to.
	for i, count := 0, n.ChildCount(); i < count; i++ {
		child := n.Child(i)
		cs := child.Style()
		if !(cs.Position.isOutOfFlow() && cs.Display != DisplayNone) {
			roundLayout(child, cumulativeX, cumulativeY, ratio)
		}
	}
	for _, child := range st.hoisted {
		roundLayout(child, cumulativeX, cumulativeY, ratio)
	}
}
