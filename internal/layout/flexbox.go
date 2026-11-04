package layout

import (
	"math"
	"sync"
)

// This file is the CSS flexbox layout algorithm
// (https://www.w3.org/TR/css-flexbox-1/), ported from Taffy's
// compute/flexbox.rs.

// flexItem is a flex item's state through the algorithm.
type flexItem struct {
	node Node

	// order is the item's index among its container's children.
	order uint32

	// size is the item's resolved preferred size.
	size Size[Opt]
	// sizeStyle is the item's size style, unresolved.
	sizeStyle   Size[Length]
	minSize     Size[Opt]
	maxSize     Size[Opt]
	aspectRatio Opt
	// alignSelf is the item's resolved cross-axis alignment.
	alignSelf Align

	overflow            Point[Overflow]
	contain             Contain
	scrollbarWidth      float32
	flexShrink          float32
	flexGrow            float32
	flexBasisIsDefinite bool

	// resolvedMinimumMainSize is the minimum main size, automatic minimum
	// sizes resolved.
	resolvedMinimumMainSize float32

	// relativeInset is the offset of a position: relative item, in flex
	// axes.
	relativeInset Size[float32]
	margin        Rect[float32]
	marginIsAuto  Rect[bool]
	padding       Rect[float32]
	border        Rect[float32]

	flexBasis      float32
	innerFlexBasis float32
	violation      float32
	frozen         bool

	// contentFlexFraction is the item's max-content flex fraction, for
	// intrinsic sizing.
	contentFlexFraction float32

	hypotheticalInnerSize Size[float32]
	hypotheticalOuterSize Size[float32]
	targetSize            Size[float32]
	outerTargetSize       Size[float32]

	baseline float32

	// offsetMain and offsetCross are the item's offsets from its line's
	// start on each axis.
	offsetMain  float32
	offsetCross float32

	// oofCandidates are out-of-flow candidates from the item's subtree,
	// relative to the container.
	oofCandidates []oofCandidate
}

func (it *flexItem) isScrollContainer() bool {
	return it.overflow.X.isScrollContainer() || it.overflow.Y.isScrollContainer()
}

func (it *flexItem) participatesInBaselineAlignment(dir FlexDirection) bool {
	return it.alignSelf == alignOf(AlignBaseline) && !it.marginIsAuto.crossStart(dir) && !it.marginIsAuto.crossEnd(dir)
}

// flexLine is a line of items: a contiguous run of the container's items.
type flexLine struct {
	items       []flexItem
	crossSize   float32
	offsetCross float32
}

// flexConstants are values reused throughout the algorithm.
type flexConstants struct {
	dir             FlexDirection
	layoutDirection Direction
	isRow           bool
	isColumn        bool
	isWrap          bool
	isWrapReverse   bool

	minSize Size[Opt]
	maxSize Size[Opt]
	border  Rect[float32]
	// contentBoxInset is padding, border and scrollbar gutters.
	contentBoxInset   Rect[float32]
	scrollbarGutter   Point[float32]
	isScrollContainer bool
	gap               Size[float32]
	alignItems        Align
	alignContent      AlignContent
	justifyContent    AlignContent

	nodeOuterSize Size[Opt]
	nodeInnerSize Size[Opt]
	// knownMainSizeIsDefinite says whether a known main size came from the
	// parent as definite, rather than from the container's own content.
	knownMainSizeIsDefinite           bool
	hasDefiniteMainSize               bool
	hasDefiniteCrossSize              bool
	crossAxisAvailableSpaceIsDefinite bool

	containerSize      Size[float32]
	innerContainerSize Size[float32]
}

// computeFlexboxLayout lays out a flex container.
func computeFlexboxLayout(n Node, in layoutInput) layoutOutput {
	known, parentSize := in.knownDimensions, in.parentSize
	style := n.Style()

	aspectRatio := style.AspectRatio
	padding := rectResolveOrZero(style.padding(), parentSize.Width)
	border := rectResolveOrZero(style.border(), parentSize.Width)
	paddingBorderSum := sizeAdd(rectSumAxes(padding), rectSumAxes(border))
	var boxSizingAdjustment Size[float32]
	if style.BoxSizing == ContentBox {
		boxSizingAdjustment = paddingBorderSum
	}

	minSize := optsAddF(optsAspectRatio(sizeMaybeResolve(style.MinSize, parentSize), aspectRatio), boxSizingAdjustment)
	maxSize := optsAddF(optsAspectRatio(sizeMaybeResolve(style.MaxSize, parentSize), aspectRatio), boxSizingAdjustment)
	var clampedStyleSize Size[Opt]
	if in.sizingMode == inherentSize {
		clampedStyleSize = optsClamp(
			optsAddF(optsAspectRatio(sizeMaybeResolve(style.Size, parentSize), aspectRatio), boxSizingAdjustment),
			minSize, maxSize)
	}

	// If both min and max on an axis are set and max <= min then this
	// determines the size on that axis.
	minMaxDefinite := minMaxDefiniteSize(minSize, maxSize)

	// The size of the container is floored by its padding and border.
	styledKnown := optsOr(known, optsMaxF(optsOr(minMaxDefinite, clampedStyleSize), paddingBorderSum))

	// Short-circuit layout if the container's size is fully determined and
	// only its size is asked for.
	if in.runMode == computeSize {
		if optsBoth(styledKnown) {
			return outputFromOuterSize(optsUnwrapOr(styledKnown, Size[float32]{}))
		}
		// The same if the width is known and only the width is requested.
		if in.axis == requestHorizontal && styledKnown.Width.Valid {
			return outputFromOuterSize(Size[float32]{Width: styledKnown.Width.Value})
		}
	}

	// Definiteness only applies to dimensions the parent passed in as known:
	// dimensions resolved from the node's own style are always definite.
	definite := Size[bool]{
		Width:  in.knownDimensionsAreDefinite.Width || !known.Width.Valid,
		Height: in.knownDimensionsAreDefinite.Height || !known.Height.Valid,
	}

	inner := in
	inner.knownDimensions = styledKnown
	inner.knownDimensionsAreDefinite = definite
	out := flexComputePreliminary(n, inner)

	// Layout containment suppresses the box's baseline for baseline
	// alignment.
	if style.Contain.suppressesBaseline() {
		out.baselines = baselines{}
	}
	return out
}

func flexComputePreliminary(n Node, in layoutInput) layoutOutput {
	known, parentSize, availableSpace := in.knownDimensions, in.parentSize, in.availableSpace
	style := n.Style()

	// Define some general constants needed for the rest of the algorithm.
	c := flexComputeConstants(style, known, in.knownDimensionsAreDefinite, parentSize, availableSpace)

	// 9. Flex Layout Algorithm

	// 9.1. Initial Setup

	// 1. Generate anonymous flex items as described in §4 Flex Items.
	buf := itemBuffers.Get().(*[]flexItem)
	items := flexGenerateItems(n, &c, (*buf)[:0])
	defer func() {
		clear(items)
		*buf = items[:0]
		itemBuffers.Put(buf)
	}()

	// 9.2. Line Length Determination

	// 2. Determine the available main and cross space for the flex items.
	available := flexDetermineAvailableSpace(known, availableSpace, &c)

	// 3. Determine the flex base size and hypothetical main size of each
	//    item.
	flexDetermineBaseSize(&c, available, items)

	// 4. Determine the main size of the flex container. This has already
	//    been done in flexComputeConstants: the inner size is
	//    c.nodeInnerSize.

	// 9.3. Main Size Determination

	// 5. Collect flex items into flex lines.
	lines := flexCollectLines(&c, available, items)

	// If the container's size is undefined, determine its main size, then
	// re-resolve gaps against the new size.
	if innerMain := c.nodeInnerSize.main(c.dir); innerMain.Valid {
		outerMain := innerMain.Value + rectMainSum(c.contentBoxInset, c.dir)
		c.innerContainerSize.setMain(c.dir, innerMain.Value)
		c.containerSize.setMain(c.dir, outerMain)
	} else {
		// Sets c.containerSize and c.innerContainerSize.
		flexDetermineContainerMainSize(available, lines, &c)
		c.nodeInnerSize.setMain(c.dir, Some(c.innerContainerSize.main(c.dir)))
		c.nodeOuterSize.setMain(c.dir, Some(c.containerSize.main(c.dir)))

		// Re-resolve percentage gaps.
		innerContainerMain := c.innerContainerSize.main(c.dir)
		newGap := style.gap().main(c.dir).maybeResolve(Some(innerContainerMain)).Or(0)
		c.gap.setMain(c.dir, newGap)
	}

	// If only the container's main size was requested we are done, as
	// nothing below changes it. This avoids measuring the cross size of
	// every item.
	if in.runMode == computeSize {
		mainIsRequested := (in.axis == requestHorizontal && c.dir.isRow()) ||
			(in.axis == requestVertical && c.dir.isColumn())
		if mainIsRequested {
			var size Size[float32]
			size.setMain(c.dir, c.containerSize.main(c.dir))
			return outputFromOuterSize(size)
		}
	}

	// 6. Resolve the flexible lengths of all the flex items to find their
	//    used main size.
	for i := range lines {
		flexResolveFlexibleLengths(&lines[i], &c)
	}

	// 9.4. Cross Size Determination

	// 7. Determine the hypothetical cross size of each item.
	for i := range lines {
		flexDetermineHypotheticalCrossSize(&lines[i], &c, available)
	}

	// Calculate child baselines. This only computes baselines when they are
	// needed.
	flexCalculateChildrenBaselines(known, available, lines, &c)

	// 8. Calculate the cross size of each flex line.
	flexCalculateCrossSize(lines, known, &c)

	// 9. Handle 'align-content: stretch'.
	flexHandleAlignContentStretch(lines, known, &c)

	// 10. Collapse visibility:collapse items. Not implemented, as
	//     visibility is not supported.

	// 11. Determine the used cross size of each flex item.
	flexDetermineUsedCrossSize(lines, &c)

	// 9.5. Main-Axis Alignment

	// 12. Distribute any remaining free space.
	flexDistributeRemainingFreeSpace(lines, &c)

	// 9.6. Cross-Axis Alignment

	// 13. Resolve cross-axis auto margins (also includes 14).
	flexResolveCrossAxisAutoMargins(lines, &c)

	// 15. Determine the flex container's used cross size.
	totalLineCrossSize := flexDetermineContainerCrossSize(lines, known, &c)

	// We have the container's size. If only its size is wanted, we are done.
	if in.runMode == computeSize {
		return outputFromOuterSize(c.containerSize)
	}

	// 16. Align all flex lines per align-content.
	flexAlignLinesPerAlignContent(lines, &c, totalLineCrossSize)

	// Do a final layout pass and gather the resulting layouts.
	inflowOverflow := flexFinalLayoutPass(lines, &c)

	// Collect out-of-flow candidates in document order (direct out-of-flow
	// children interleaved with candidates bubbled from in-flow children's
	// subtrees), for the out-of-flow pass that runs after this algorithm.
	candidates := flexCollectOofCandidates(n, &c, items)

	inset := c.border
	if c.layoutDirection.isRTL() {
		inset.Left += c.scrollbarGutter.X
	} else {
		inset.Right += c.scrollbarGutter.X
	}
	inset.Bottom += c.scrollbarGutter.Y
	areaSize := sizeSub(c.containerSize, rectSumAxes(inset))
	areaOffset := Point[float32]{inset.Left, inset.Top}

	for order, count := 0, n.ChildCount(); order < count; order++ {
		child := n.Child(order)
		if child.Style().Display == DisplayNone {
			child.LayoutState().Unrounded = Layout{Order: uint32(order)}
			performChildLayout(child, noneSize, noneSize, maxContentSpaces, contentSize, Line[bool]{})
		}
	}

	// 8.5. Flex Container Baselines: the flex container's first baseline
	// (https://www.w3.org/TR/css-flexbox-1/#flex-baselines). It comes from
	// the startmost flex line, by visual position: in a wrap-reverse
	// container the cross axis is flipped, so that is the last line in
	// flex-line order.
	var firstBaseline Opt
	if len(lines) > 0 {
		line := &lines[0]
		if c.isWrapReverse {
			line = &lines[len(lines)-1]
		}
		if len(line.items) > 0 {
			if c.isColumn {
				// A column container's baseline comes from the startmost item in
				// the line, the last one in flex order for reversed directions.
				item := &line.items[0]
				if c.dir.isReverse() {
					item = &line.items[len(line.items)-1]
				}
				firstBaseline = Some(item.baseline)
			} else {
				item := &line.items[0]
				for i := range line.items {
					if line.items[i].participatesInBaselineAlignment(c.dir) {
						item = &line.items[i]
						break
					}
				}
				firstBaseline = Some(item.baseline)
			}
		}
	}

	out := outputFromSizes(c.containerSize, inflowOverflow, baselines{first: firstBaseline})
	out.oofCandidates = candidates
	out.oofArea = oofPositioningArea{size: areaSize, offset: areaOffset}
	out.hasOofArea = true
	return out
}

// flexResolveNormalAlignment resolves normal, which behaves as stretch for
// flex items (https://www.w3.org/TR/css-align-3/#align-flex).
func flexResolveNormalAlignment(a Align) Align {
	if a.Keyword == AlignNormal || a.Keyword == AlignAuto {
		return alignOf(AlignStretch)
	}
	return a
}

func flexComputeConstants(style *Style, known Size[Opt], knownDefinite Size[bool], parentSize Size[Opt],
	availableSpace Size[AvailableSpace]) flexConstants {
	dir := style.FlexDirection
	wrap := style.FlexWrap

	aspectRatio := style.AspectRatio
	padding := rectResolveOrZero(style.padding(), parentSize.Width)
	border := rectResolveOrZero(style.border(), parentSize.Width)
	paddingBorderSum := sizeAdd(rectSumAxes(padding), rectSumAxes(border))
	var boxSizingAdjustment Size[float32]
	if style.BoxSizing == ContentBox {
		boxSizingAdjustment = paddingBorderSum
	}

	// Normal behaves as stretch for flex items. It is resolved once here so
	// that items whose align-self defers to it are resolved too.
	alignItems := flexResolveNormalAlignment(style.alignItems())
	// Normal behaves as stretch for flex containers. On the main axis
	// stretch behaves as flex-start, so a normal justify-content resolves
	// straight to flex-start
	// (https://www.w3.org/TR/css-align-3/#distribution-flex).
	alignContent := style.AlignContent
	if alignContent.Keyword == ContentNormal {
		alignContent = contentOf(ContentStretch)
	}
	justifyContent := style.JustifyContent
	if justifyContent.Keyword == ContentNormal {
		justifyContent = contentOf(ContentFlexStart)
	}
	layoutDirection := style.Direction

	// Scrollbar gutters are reserved on the axis opposite the scrolling one.
	scrollbarGutter := style.scrollbarGutter()
	isScrollContainer := style.Overflow.X.isScrollContainer() || style.Overflow.Y.isScrollContainer()
	contentBoxInset := rectAdd(padding, border)
	contentBoxInset.Bottom += scrollbarGutter.Y
	if layoutDirection.isRTL() {
		contentBoxInset.Left += scrollbarGutter.X
	} else {
		contentBoxInset.Right += scrollbarGutter.X
	}

	nodeOuterSize := known
	nodeInnerSize := optsSubF(nodeOuterSize, rectSumAxes(contentBoxInset))
	knownMainSizeIsDefinite := knownDefinite.main(dir)
	hasDefiniteMainSize := knownMainSizeIsDefinite && known.main(dir).Valid
	hasDefiniteCrossSize := knownDefinite.cross(dir) && known.cross(dir).Valid
	crossAvailableIsDefinite := hasDefiniteCrossSize || availableSpace.cross(dir).isDefinite()
	gap := sizeResolveOrZero(style.gap(), optsOr(nodeInnerSize, Size[Opt]{Some(0), Some(0)}))

	return flexConstants{
		dir:             dir,
		layoutDirection: layoutDirection,
		isRow:           dir.isRow(),
		isColumn:        dir.isColumn(),
		isWrap:          wrap.isMultiLine(),
		isWrapReverse:   wrap.isReverse(),
		minSize: optsAddF(optsAspectRatio(sizeMaybeResolve(style.MinSize, parentSize), aspectRatio),
			boxSizingAdjustment),
		maxSize: optsAddF(optsAspectRatio(sizeMaybeResolve(style.MaxSize, parentSize), aspectRatio),
			boxSizingAdjustment),
		border:                            border,
		gap:                               gap,
		contentBoxInset:                   contentBoxInset,
		scrollbarGutter:                   scrollbarGutter,
		isScrollContainer:                 isScrollContainer,
		alignItems:                        alignItems,
		alignContent:                      alignContent,
		justifyContent:                    justifyContent,
		nodeOuterSize:                     nodeOuterSize,
		nodeInnerSize:                     nodeInnerSize,
		knownMainSizeIsDefinite:           knownMainSizeIsDefinite,
		hasDefiniteMainSize:               hasDefiniteMainSize,
		hasDefiniteCrossSize:              hasDefiniteCrossSize,
		crossAxisAvailableSpaceIsDefinite: crossAvailableIsDefinite,
	}
}

// flexResolveRelativeInset resolves a position: relative item's inset to
// one offset on each flex axis, accounting for flex and writing direction.
func flexResolveRelativeInset(inset Rect[Opt], c *flexConstants) Size[float32] {
	dir := c.dir
	isRTLRow := dir.isRow() && c.layoutDirection.isRTL()
	isRTLColumn := dir.isColumn() && c.layoutDirection.isRTL()
	neg := func(o Opt) Opt { return Opt{-o.Value, o.Valid} }
	var main, cross float32
	if isRTLRow {
		main = inset.mainEnd(dir).OrOpt(neg(inset.mainStart(dir))).Or(0)
	} else {
		main = inset.mainStart(dir).OrOpt(neg(inset.mainEnd(dir))).Or(0)
	}
	if isRTLColumn {
		cross = neg(inset.crossEnd(dir)).OrOpt(inset.crossStart(dir)).Or(0)
	} else {
		cross = inset.crossStart(dir).OrOpt(neg(inset.crossEnd(dir))).Or(0)
	}
	var out Size[float32]
	out.setMain(dir, main)
	out.setCross(dir, cross)
	return out
}

// itemBuffers holds the flex items of passes that finished. A commit runs
// several passes over each container on a changed path, every display
// frame while something animates; nothing keeps an item past its pass.
var itemBuffers = sync.Pool{New: func() any { return new([]flexItem) }}

// flexGenerateItems generates anonymous flex items, appending them to
// items.
//
// 9.1. Initial Setup (https://www.w3.org/TR/css-flexbox-1/#box-manip):
// generate anonymous flex items as described in §4 Flex Items.
func flexGenerateItems(n Node, c *flexConstants, items []flexItem) []flexItem {
	// Percentage sizes of items resolve against the container's inner size,
	// but only if that size is definite. A known main size derived from the
	// container's own content is indefinite here.
	percentBasis := c.nodeInnerSize
	if !c.knownMainSizeIsDefinite {
		percentBasis.setMain(c.dir, None)
	}

	count := n.ChildCount()
	for index := 0; index < count; index++ {
		child := n.Child(index)
		cs := child.Style()
		if cs.Position.isOutOfFlow() || cs.Display == DisplayNone {
			continue
		}
		aspectRatio := cs.AspectRatio
		padding := rectResolveOrZero(cs.padding(), c.nodeInnerSize.Width)
		border := rectResolveOrZero(cs.border(), c.nodeInnerSize.Width)
		pbSum := rectSumAxes(rectAdd(padding, border))
		var boxSizingAdjustment Size[float32]
		if cs.BoxSizing == ContentBox {
			boxSizingAdjustment = pbSum
		}
		var relativeInset Size[float32]
		if cs.Position == PositionRelative {
			inset := Rect[Opt]{
				Left:   cs.Inset.Left.maybeResolve(c.nodeInnerSize.Width),
				Right:  cs.Inset.Right.maybeResolve(c.nodeInnerSize.Width),
				Top:    cs.Inset.Top.maybeResolve(c.nodeInnerSize.Height),
				Bottom: cs.Inset.Bottom.maybeResolve(c.nodeInnerSize.Height),
			}
			relativeInset = flexResolveRelativeInset(inset, c)
		}
		margin := cs.margin()
		items = append(items, flexItem{
			node:  child,
			order: uint32(index),
			size: optsAddF(optsAspectRatio(sizeMaybeResolve(cs.Size, percentBasis), aspectRatio),
				boxSizingAdjustment),
			sizeStyle:     cs.Size,
			minSize:       optsAddF(sizeMaybeResolve(cs.MinSize, percentBasis), boxSizingAdjustment),
			maxSize:       optsAddF(sizeMaybeResolve(cs.MaxSize, percentBasis), boxSizingAdjustment),
			aspectRatio:   aspectRatio,
			relativeInset: relativeInset,
			margin:        rectResolveOrZero(margin, c.nodeInnerSize.Width),
			marginIsAuto:  rectIsAuto(margin),
			padding:       padding,
			border:        border,
			alignSelf: flexResolveNormalAlignment(cs.alignSelfOr(c.alignItems)).
				resolveSelfRelative(cs.Direction, c.layoutDirection, c.isColumn),
			overflow:       cs.Overflow,
			contain:        cs.Contain,
			scrollbarWidth: cs.ScrollbarWidth,
			flexGrow:       cs.FlexGrow,
			flexShrink:     cs.flexShrink(),
		})
	}
	return items
}

// flexDetermineAvailableSpace determines the available main and cross
// space for the flex items.
//
// 9.2. Line Length Determination
// (https://www.w3.org/TR/css-flexbox-1/#algo-available): for each
// dimension, if that dimension of the flex container's content box is a
// definite size, use that; if that dimension of the flex container is being
// sized under a min or max-content constraint, the available space in that
// dimension is that constraint; otherwise, subtract the flex container's
// margin, border, and padding from the space available to the flex
// container in that dimension and use that value. This might result in an
// infinite value.
//
// The container's margins have already been subtracted from the outer
// available space by its parent.
func flexDetermineAvailableSpace(known Size[Opt], outer Size[AvailableSpace], c *flexConstants) Size[AvailableSpace] {
	// Min, max and preferred size styles are already applied to known.
	var s Size[AvailableSpace]
	if known.Width.Valid {
		s.Width = Definite(fmax(known.Width.Value-rectHSum(c.contentBoxInset), 0))
	} else {
		s.Width = outer.Width.subF(rectHSum(c.contentBoxInset)).maxF(0)
	}
	if known.Height.Valid {
		s.Height = Definite(fmax(known.Height.Value-rectVSum(c.contentBoxInset), 0))
	} else {
		s.Height = outer.Height.subF(rectVSum(c.contentBoxInset)).maxF(0)
	}
	return s
}

// flexDetermineBaseSize determines the flex base size and hypothetical main
// size of each item.
//
// 9.2. Line Length Determination
// (https://www.w3.org/TR/css-flexbox-1/#algo-main-item):
//
//   - A. If the item has a definite used flex basis, that's the flex base
//     size.
//   - B. If the flex item has an intrinsic aspect ratio, a used flex basis
//     of content, and a definite cross size, then the flex base size is
//     calculated from its inner cross size and the flex item's intrinsic
//     aspect ratio.
//   - C. If the used flex basis is content or depends on its available
//     space, and the flex container is being sized under a min-content or
//     max-content constraint (e.g. when performing automatic table layout
//     [CSS21]), size the item under that constraint. The flex base size is
//     the item's resulting main size.
//   - E. Otherwise, size the item into the available space using its used
//     flex basis in place of its main size, treating a value of content as
//     max-content. If a cross size is needed to determine the main size
//     (e.g. when the flex item's main size is in its block axis) and the flex
//     item's cross size is auto and not definite, in this calculation use
//     fit-content as the flex item's cross size. The flex base size is the
//     item's resulting main size.
//
// When determining the flex base size, the item's min and max main sizes are
// ignored (no clamping occurs). Furthermore, the sizing calculations that
// floor the content box size at zero when applying box-sizing are also
// ignored. (For example, an item with a specified size of zero, positive
// padding, and box-sizing: border-box will have an outer flex base size of
// zero—and hence a negative inner flex base size.)
func flexDetermineBaseSize(c *flexConstants, available Size[AvailableSpace], items []flexItem) {
	dir := c.dir
	for i := range items {
		child := &items[i]
		cs := child.node.Style()

		// Parent size for child sizing.
		crossParentSize := c.nodeInnerSize.cross(dir)
		childParentSize := fromCross(dir, crossParentSize)

		// Available space for child sizing, excluding the child's margins. Min
		// and max sizes transferred through the aspect ratio are taken into
		// account here (https://github.com/w3c/csswg-drafts/issues/10997).
		transferredMin := optsAspectRatio(child.minSize, child.aspectRatio)
		transferredMax := optsAspectRatio(child.maxSize, child.aspectRatio)
		childMinCross := transferredMin.cross(dir)
		childMaxCross := transferredMax.cross(dir)

		// Clamp the available space by the min and max sizes.
		var crossAvailable AvailableSpace
		switch av := available.cross(dir); av.Kind {
		case SpaceDefinite:
			crossAvailable = Definite(fclamp(fmax(crossParentSize.Or(av.Value)-rectCrossSum(child.margin, dir), 0),
				childMinCross, childMaxCross))
		case SpaceMinContent:
			if childMinCross.Valid {
				crossAvailable = Definite(childMinCross.Value)
			} else {
				crossAvailable = MinContentSpace
			}
		default:
			if childMaxCross.Valid {
				crossAvailable = Definite(childMaxCross.Value)
			} else {
				crossAvailable = MaxContentSpace
			}
		}

		// Known dimensions for child sizing.
		childCrossSizeIsDefinite := child.size.cross(dir).Valid
		childKnown := child.size.withMain(dir, None)
		// Clamp the definite cross size by the cross min and max sizes, so
		// that sizes transferred through an intrinsic aspect ratio (as for
		// replaced elements) are based on the used cross size.
		childKnown.setCross(dir, childKnown.cross(dir).clamp(transferredMin.cross(dir), transferredMax.cross(dir)))
		if child.alignSelf == alignOf(AlignStretch) && !child.marginIsAuto.crossStart(dir) &&
			!child.marginIsAuto.crossEnd(dir) && !childKnown.cross(dir).Valid {
			childKnown.setCross(dir, crossAvailable.intoOption())
			// The cross size of a stretched item is definite if the container
			// has a definite cross size
			// (https://www.w3.org/TR/css-flexbox-1/#definite-sizes).
			childCrossSizeIsDefinite = !c.isWrap && c.hasDefiniteCrossSize && crossParentSize.Valid
		}
		// A cross size that is a sizing keyword (min-content, max-content,
		// fit-content) is the item's size under that constraint, known
		// before its main size, as browsers size such items: a column's
		// item of min-content width wraps its text before its height is
		// measured.
		if !childKnown.cross(dir).Valid {
			sizeStyle := child.sizeStyle.cross(dir)
			if r, ok := resolveSizingKeyword(sizeStyle, crossAvailable.intoOption(), crossParentSize); ok && !sizeStyle.isStretch() {
				size := r.exact
				if r.measure {
					size = measureChildSize(child.node, childKnown, childParentSize,
						maxContentSpaces.withCross(dir, r.available), contentSize, dir.crossAxis(), Line[bool]{})
				}
				cross := Some(size).clamp(child.minSize.cross(dir), child.maxSize.cross(dir))
				childKnown.setCross(dir, cross)
				child.size.setCross(dir, cross)
				childCrossSizeIsDefinite = true
			}
		}

		// Percentage padding and border resolve against the container's
		// inline size (its width), whichever axis is the main axis.
		containerWidth := c.nodeInnerSize.Width
		var boxSizingAdjustment float32
		if cs.BoxSizing == ContentBox {
			p := rectResolveOrZero(cs.padding(), containerWidth)
			b := rectResolveOrZero(cs.border(), containerWidth)
			boxSizingAdjustment = rectSumAxes(rectAdd(p, b)).main(dir)
		}
		// Percentage flex bases resolve against the container's inner main
		// size, but only if it is definite. A known main size derived from
		// the container's own content is indefinite here.
		var percentMain Opt
		if c.knownMainSizeIsDefinite {
			percentMain = c.nodeInnerSize.main(dir)
		}
		basisStyle := cs.FlexBasis
		flexBasis := basisStyle.maybeResolve(percentMain).addF(boxSizingAdjustment)

		child.flexBasis = flexBaseSize(c, child, available, basisStyle, flexBasis, percentMain, childKnown,
			childParentSize, crossAvailable, childCrossSizeIsDefinite)

		// Floor the flex basis by the padding and border (flooring the inner
		// flex basis at zero). This seems to violate the spec, which says the
		// content box should not be floored at zero when calculating the flex
		// basis, but it matches both Chrome and Firefox.
		//
		// Spec: https://www.w3.org/TR/css-flexbox-1/#intrinsic-item-contributions
		// Spec: https://www.w3.org/TR/css-flexbox-1/#change-2016-max-contribution
		paddingBorderSum := rectMainSum(child.padding, dir) + rectMainSum(child.border, dir)
		child.flexBasis = fmax(child.flexBasis, paddingBorderSum)

		// The hypothetical main size is the item's flex base size clamped
		// according to its used min and max main sizes (and flooring the
		// content box size at zero).

		child.innerFlexBasis = child.flexBasis - rectMainSum(child.padding, dir) - rectMainSum(child.border, dir)

		pbAxes := rectSumAxes(rectAdd(child.padding, child.border))

		// The parent size on the main axis must not be set for this call, as
		// it resolves percentages, and a percentage size on an axis should not
		// contribute to a min-content contribution on that same axis. The
		// parent size and available space should be set as usual on the cross
		// axis so that wrapping content wraps correctly
		// (https://drafts.csswg.org/css-sizing-3/#min-percentage-contribution).
		automaticMin := Size[Opt]{child.overflow.X.automaticMinSize(), child.overflow.Y.automaticMinSize()}
		styleMinMain := optsOr(child.minSize, automaticMin).main(dir)

		if styleMinMain.Valid {
			child.resolvedMinimumMainSize = styleMinMain.Value
		} else {
			childAvailable := minContentSpaces.withCross(dir, crossAvailable)
			minContentMain := measureChildSize(child.node, childKnown, childParentSize, childAvailable, contentSize,
				dir.mainAxis(), Line[bool]{})

			// 4.5. Automatic Minimum Size of Flex Items
			// (https://www.w3.org/TR/css-flexbox-1/#min-size-auto)
			clamped := fminOpt(fminOpt(minContentMain, child.size.main(dir)), transferredMax.main(dir))
			child.resolvedMinimumMainSize = fmax(clamped, pbAxes.main(dir))
		}

		// Sizes transferred through the aspect ratio clamp the hypothetical
		// main size, but do not take part in resolving flexible lengths or
		// clamping the final size
		// (https://github.com/w3c/csswg-drafts/issues/10997).
		hypotheticalInnerMin := fmax(fmaxOpt(child.resolvedMinimumMainSize, transferredMin.main(dir)),
			pbAxes.main(dir))
		hypotheticalInner := fclamp(child.flexBasis, Some(hypotheticalInnerMin), transferredMax.main(dir))
		hypotheticalOuter := hypotheticalInner + rectMainSum(child.margin, dir)

		child.hypotheticalInnerSize.setMain(dir, hypotheticalInner)
		child.hypotheticalOuterSize.setMain(dir, hypotheticalOuter)
	}
}

// flexBaseSize computes an item's flex base size, before it is floored by
// its padding and border.
func flexBaseSize(c *flexConstants, child *flexItem, available Size[AvailableSpace], basisStyle Length,
	flexBasis, percentMain Opt, childKnown, childParentSize Size[Opt], crossAvailable AvailableSpace,
	childCrossSizeIsDefinite bool) float32 {
	dir := c.dir

	// A. If the item has a definite used flex basis, that's the flex base
	//    size.

	// B. If the flex item has an intrinsic aspect ratio, a used flex basis
	//    of content, and a definite cross size, then the flex base size is
	//    calculated from its inner cross size and the flex item's intrinsic
	//    aspect ratio.

	mainSize := child.size.main(dir)
	mainStretchSize := percentMain.subF(rectMainSum(child.margin, dir)).maxF(0)

	// A flex basis that is a sizing keyword (min-content, max-content,
	// fit-content, fit-content(...), stretch) is used in place of the main
	// size property: stretch resolves to an exact (definite) size while the
	// other keywords set the available space constraint the item is
	// measured under. A keyword that cannot resolve here behaves as content.
	var keywordAvailable AvailableSpace
	hasKeywordAvailable := false
	switch {
	case basisStyle.isContent():
		// A flex basis of content is an automatic size based on the item's
		// content: the item is measured, ignoring its main size property,
		// under the default constraint below.
	case basisStyle.isSizingKeyword():
		if r, ok := resolveSizingKeyword(basisStyle, mainStretchSize, percentMain); ok {
			if !r.measure {
				child.flexBasisIsDefinite = true
				return r.exact
			}
			keywordAvailable, hasKeywordAvailable = r.available, true
		}
	default:
		if flexBasis.Valid {
			child.flexBasisIsDefinite = true
			return flexBasis.Value
		}
		if basisStyle.IsAuto() && mainSize.Valid {
			child.flexBasisIsDefinite = true
			return mainSize.Value
		}

		// A main size that is a sizing keyword either resolves to an exact
		// size or sets the available space constraint the item is measured
		// under.
		if basisStyle.IsAuto() {
			if r, ok := resolveSizingKeyword(child.sizeStyle.main(dir), mainStretchSize, percentMain); ok {
				if !r.measure {
					child.flexBasisIsDefinite = true
					return r.exact
				}
				keywordAvailable, hasKeywordAvailable = r.available, true
			}
		}
	}

	// C. If the used flex basis is content or depends on its available
	//    space, and the flex container is being sized under a min-content or
	//    max-content constraint (e.g. when performing automatic table layout
	//    [CSS21]), size the item under that constraint. The flex base size is
	//    the item's resulting main size.

	// This is covered by the implementation of E below, which passes the
	// available space constraint through to the child size computation. It
	// may need a separate implementation if/when D is implemented.

	// D. Otherwise, if the used flex basis is content or depends on its
	//    available space, the available main size is infinite, and the flex
	//    item's inline axis is parallel to the main axis, lay the item out
	//    using the rules for a box in an orthogonal flow
	//    [CSS3-WRITING-MODES]. The flex base size is the item's max-content
	//    main size.

	// TODO if/when vertical writing modes are supported

	// If the item has an aspect ratio and a definite cross size then the
	// flex base size is that cross size transferred through the aspect ratio
	// (case B above), and is therefore definite.
	if childCrossSizeIsDefinite {
		if cross := childKnown.cross(dir); child.aspectRatio.Valid && cross.Valid {
			child.flexBasisIsDefinite = true
			if dir.isRow() {
				return cross.Value * child.aspectRatio.Value
			}
			return cross.Value / child.aspectRatio.Value
		}
	}

	// E. Otherwise, size the item into the available space using its used
	//    flex basis in place of its main size, treating a value of content
	//    as max-content. If a cross size is needed to determine the main size
	//    (e.g. when the flex item's main size is in its block axis) and the
	//    flex item's cross size is auto and not definite, in this calculation
	//    use fit-content as the flex item's cross size. The flex base size is
	//    the item's resulting main size.

	mainAvailable := keywordAvailable
	if !hasKeywordAvailable {
		// Definite available space becomes max-content.
		mainAvailable = MaxContentSpace
		if available.main(dir).isMinContent() {
			mainAvailable = MinContentSpace
		}
	}
	childAvailable := maxContentSpaces.withMain(dir, mainAvailable).withCross(dir, crossAvailable)

	return measureChildSize(child.node, childKnown, childParentSize, childAvailable, contentSize, dir.mainAxis(),
		Line[bool]{})
}

// flexCollectLines collects flex items into flex lines.
//
// 9.3. Main Size Determination
// (https://www.w3.org/TR/css-flexbox-1/#algo-line-break):
//
//   - If the flex container is single-line, collect all the flex items into
//     a single flex line.
//   - Otherwise, starting from the first uncollected item, collect
//     consecutive items one by one until the first time that the next
//     collected item would not fit into the flex container's inner main size
//     (or until a forced break is encountered). If the very first uncollected
//     item wouldn't fit, collect just it into the line. For this step, the
//     size of a flex item is its outer hypothetical main size (which can be
//     negative). Repeat until all flex items have been collected into flex
//     lines.
//
// Note that the "collect as many" line collects zero-sized flex items onto
// the end of the previous line even if the last non-zero item exactly
// "filled up" the line.
func flexCollectLines(c *flexConstants, available Size[AvailableSpace], items []flexItem) []flexLine {
	// Wrapping into multiple lines requires a definite main size. If the
	// container's known main size comes from its own content (and is thus
	// indefinite) then all items go into one line, matching how the
	// container was sized under a min or max-content constraint.
	if !c.isWrap || !c.knownMainSizeIsDefinite {
		return []flexLine{{items: items}}
	}

	var mainAvailable AvailableSpace
	switch maxMain := c.maxSize.main(c.dir); {
	case maxMain.Valid:
		avail := available.main(c.dir).intoOption().Or(maxMain.Value)
		// If the container's main size is not definite then it is at most
		// the max main size, so the max size (not the available space) is
		// the limit items wrap against.
		if !c.hasDefiniteMainSize {
			avail = fmin(avail, maxMain.Value)
		}
		mainAvailable = Definite(fmaxOpt(avail, c.minSize.main(c.dir)))
	case !c.dir.isRow() && !c.hasDefiniteMainSize && available.main(c.dir).isDefinite():
		// A column container's automatic main size is content-based, so
		// definite available space from an ancestor does not constrain where
		// its lines wrap. Automatic widths resolve against available space,
		// so rows do wrap against it.
		mainAvailable = MaxContentSpace
	default:
		mainAvailable = available.main(c.dir)
	}

	switch mainAvailable.Kind {
	case SpaceMaxContent:
		// Under a max-content constraint the items never wrap (at least for
		// now: future CSS may add forced wrap points).
		return []flexLine{{items: items}}
	case SpaceMinContent:
		// Under a min-content constraint, take every wrapping opportunity and
		// put each item in its own line.
		lines := make([]flexLine, len(items))
		for i := range items {
			lines[i] = flexLine{items: items[i : i+1]}
		}
		return lines
	}

	limit := mainAvailable.Value
	gap := c.gap.main(c.dir)
	lines := make([]flexLine, 0, 1)
	for len(items) > 0 {
		// Find the index of the first item of the next line, or the end if
		// all the remaining items fit on this line.
		var lineLength float32
		index := len(items)
		for i := range items {
			// Gaps only occur between items, so the first item on the line
			// adds none.
			var gapContribution float32
			if i != 0 {
				gapContribution = gap
			}
			lineLength += items[i].hypotheticalOuterSize.main(c.dir) + gapContribution
			if lineLength > limit && i != 0 {
				index = i
				break
			}
		}
		lines = append(lines, flexLine{items: items[:index]})
		items = items[index:]
	}
	return lines
}

// flexItemKnownDimensionDefiniteness says whether each of an item's known
// dimensions is definite when the item is laid out. A post-flexing main size
// is definite only if the container has a definite main size or the item's
// used flex basis is definite
// (https://www.w3.org/TR/css-flexbox-1/#definite-sizes).
func flexItemKnownDimensionDefiniteness(c *flexConstants, it *flexItem) Size[bool] {
	dir := c.dir
	mainIsDefinite := c.hasDefiniteMainSize || it.flexBasisIsDefinite

	// An item's cross size is definite if it is stretched (it stretches to
	// the flex line, whose size is known by the time the item's final layout
	// is performed), or if its cross size style resolved to a definite size.
	// A non-stretched item's cross width is also fit-content sized, and a
	// size resolved by fit-content sizing against definite available space
	// is itself definite (https://www.w3.org/TR/css-sizing-3/#definite). The
	// same does not apply to cross heights, which are content-based and so
	// indefinite when not stretched.
	hasCrossAutoMargins := it.marginIsAuto.crossStart(dir) || it.marginIsAuto.crossEnd(dir)
	crossSize := it.sizeStyle.cross(dir)
	isStretched := !hasCrossAutoMargins &&
		(crossSize.isStretch() || (it.alignSelf == alignOf(AlignStretch) && crossSize.IsAuto()))
	crossIsDefinite := isStretched || it.size.cross(dir).Valid ||
		(!dir.isRow() && c.crossAxisAvailableSpaceIsDefinite)

	return Size[bool]{true, true}.withMain(dir, mainIsDefinite).withCross(dir, crossIsDefinite)
}

// flexItemMainLength is an item's contribution to a line's length when
// sizing the container under definite or min-content available space.
func flexItemMainLength(c *flexConstants, child *flexItem) float32 {
	paddingBorderSum := rectMainSum(rectAdd(child.padding, child.border), c.dir)
	return fmax(fmaxOpt(child.flexBasis, child.minSize.main(c.dir))+rectMainSum(child.margin, c.dir),
		paddingBorderSum)
}

// flexLongestLine is the longest line's length: its items' lengths and
// gaps.
func flexLongestLine(c *flexConstants, lines []flexLine) float32 {
	gap := c.gap.main(c.dir)
	longest, found := float32(0), false
	for i := range lines {
		line := &lines[i]
		var total float32
		for j := range line.items {
			total += flexItemMainLength(c, &line.items[j])
		}
		length := total + flexSumAxisGaps(gap, len(line.items))
		if !found || flexTotalCmp(length, longest) >= 0 {
			longest, found = length, true
		}
	}
	return longest
}

// flexDetermineContainerMainSize determines the container's main size, when
// it is not already known.
func flexDetermineContainerMainSize(available Size[AvailableSpace], lines []flexLine, c *flexConstants) {
	dir := c.dir
	mainContentBoxInset := rectMainSum(c.contentBoxInset, dir)

	var outerMain float32
	if outer := c.nodeOuterSize.main(dir); outer.Valid {
		outerMain = outer.Value
	} else {
		switch av := available.main(dir); {
		case av.isDefinite():
			size := flexLongestLine(c, lines) + mainContentBoxInset
			if len(lines) > 1 {
				outerMain = fmax(size, av.Value)
			} else {
				outerMain = size
			}
		case av.isMinContent() && c.isWrap:
			outerMain = flexLongestLine(c, lines) + mainContentBoxInset
		default:
			outerMain = flexIntrinsicMainSize(available, lines, c) + mainContentBoxInset
		}
	}

	outerMain = fmax(fclamp(outerMain, c.minSize.main(dir), c.maxSize.main(dir)),
		mainContentBoxInset-c.scrollbarGutter.main(dir))

	innerMain := fmax(outerMain-mainContentBoxInset, 0)
	c.containerSize.setMain(dir, outerMain)
	c.innerContainerSize.setMain(dir, innerMain)
	c.nodeInnerSize.setMain(dir, Some(innerMain))
}

// flexIntrinsicMainSize is the container's min-content or max-content main
// size, without its padding and border: the largest sum of the item
// contributions on one line.
func flexIntrinsicMainSize(available Size[AvailableSpace], lines []flexLine, c *flexConstants) float32 {
	dir := c.dir
	var mainSize float32

	for li := range lines {
		line := &lines[li]
		for ii := range line.items {
			item := &line.items[ii]
			styleMin := item.minSize.main(dir)
			stylePreferred := item.size.main(dir)
			styleMax := item.maxSize.main(dir)
			marginMain := rectMainSum(item.margin, dir)

			// The spec is a bit unclear on this point (one reading is that the
			// max with the preferred size should not be included here), but for
			// row containers this matches both Chrome and Firefox as of 9th
			// March 2023. For column containers the flex base size alone is the
			// clamping basis, as the spec says: a column item's height must not
			// inflate the container's max-content size beyond the item's flex
			// basis.
			//
			// Spec: https://www.w3.org/TR/css-flexbox-1/#intrinsic-item-contributions
			// Spec modification: https://www.w3.org/TR/css-flexbox-1/#change-2016-max-contribution
			// Issue: https://github.com/w3c/csswg-drafts/issues/1435
			clampingBasis := Some(item.flexBasis)
			if c.isRow {
				clampingBasis = clampingBasis.max(stylePreferred)
			}
			var flexBasisMin, flexBasisMax Opt
			if item.flexShrink == 0 {
				flexBasisMin = clampingBasis
			}
			if item.flexGrow == 0 {
				flexBasisMax = clampingBasis
			}

			minMain := fmax(styleMin.max(flexBasisMin).OrOpt(flexBasisMin).Or(item.resolvedMinimumMainSize),
				item.resolvedMinimumMainSize)
			maxMain := styleMax.min(flexBasisMax).OrOpt(flexBasisMax).Or(inf32)

			var contribution float32
			switch {
			// If the clamping values are such that max <= min, then the
			// expensive content size measurement can be skipped, as the
			// clamping values override it anyway.
			case stylePreferred.Valid && (maxMain <= minMain || maxMain <= stylePreferred.Value):
				contribution = fmax(fmin(stylePreferred.Value, maxMain), minMain) + marginMain
			case maxMain <= minMain:
				contribution = minMain + marginMain
			// Else compute the min- or max-content size and apply the full
			// formula for the min- or max-content contribution.
			case item.isScrollContainer():
				contribution = item.flexBasis + marginMain
			case stylePreferred.Valid:
				// A definite preferred main size is the item's content
				// contribution (an inherent-size measurement would return it),
				// floored by its main padding and border, so the measurement is
				// skipped. Min and max clamping apply as for measured
				// contributions.
				itemPBMain := rectMainSum(item.padding, dir) + rectMainSum(item.border, dir)
				innerMain := fmax(stylePreferred.Value, itemPBMain)
				if c.isRow {
					contribution = fclamp(innerMain+marginMain, styleMin, styleMax)
				} else {
					contribution = fclamp(fmax(innerMain, item.flexBasis)+marginMain, styleMin, styleMax)
				}
			default:
				contribution = flexMeasureContribution(available, item, c, styleMin, styleMax)
			}
			diff := contribution - item.flexBasis
			switch {
			case diff > 0:
				item.contentFlexFraction = diff / fmax(1, item.flexGrow)
			case diff < 0:
				scaledShrinkFactor := fmax(1, item.flexShrink) * item.innerFlexBasis
				item.contentFlexFraction = diff / scaledShrinkFactor
			default:
				// diff is 0 here (and not NaN).
				item.contentFlexFraction = 0
			}
		}

		// The spec says to scale everything by the line's max flex fraction,
		// but neither Chrome nor Firefox do, so neither does this.

		// Add each item's flex base size to the product of its flex grow
		// factor (or scaled flex shrink factor, if the chosen max-content flex
		// fraction was negative) and the chosen max-content flex fraction,
		// then clamp that result by the max main size floored by the min main
		// size.
		//
		// The flex container's max-content size is the largest sum of these
		// sizes of all items within a single line.
		var sum float32
		for ii := range line.items {
			item := &line.items[ii]
			fraction := item.contentFlexFraction
			var contribution float32
			switch {
			case fraction > 0:
				contribution = fmax(1, item.flexGrow) * fraction
			case fraction < 0:
				scaledShrinkFactor := fmax(1, item.flexShrink) * item.innerFlexBasis
				if scaledShrinkFactor != 0 {
					contribution = scaledShrinkFactor * fraction
				}
			}
			size := item.flexBasis + contribution
			item.outerTargetSize.setMain(dir, size)
			item.targetSize.setMain(dir, size)
			sum += size
		}

		gapSum := flexSumAxisGaps(c.gap.main(dir), len(line.items))
		mainSize = fmax(mainSize, sum+gapSum)
	}
	return mainSize
}

// flexMeasureContribution measures an item's min- or max-content
// contribution to its container's main size.
func flexMeasureContribution(available Size[AvailableSpace], item *flexItem, c *flexConstants,
	styleMin, styleMax Opt) float32 {
	dir := c.dir
	// Parent size for child sizing.
	crossParentSize := c.nodeInnerSize.cross(dir)

	// Available space for child sizing, excluding the child's margins.
	crossAvailable := available.cross(dir)
	if crossAvailable.isDefinite() {
		crossAvailable.Value = fmax(crossParentSize.Or(crossAvailable.Value)-rectCrossSum(item.margin, dir), 0)
	}
	crossAvailable = crossAvailable.clampOpt(item.minSize.cross(dir), item.maxSize.cross(dir))

	childAvailable := available.withCross(dir, crossAvailable)

	// Known dimensions for child sizing. The definite cross size is clamped
	// by the cross min and max sizes, as flexDetermineBaseSize does: an item
	// measured wider than its max width wraps onto fewer lines and
	// under-reports its height.
	childKnown := item.size.withMain(dir, None)
	transferredMin := optsAspectRatio(item.minSize, item.aspectRatio)
	transferredMax := optsAspectRatio(item.maxSize, item.aspectRatio)
	childKnown.setCross(dir, childKnown.cross(dir).clamp(transferredMin.cross(dir), transferredMax.cross(dir)))
	if item.alignSelf == alignOf(AlignStretch) && !childKnown.cross(dir).Valid {
		childKnown.setCross(dir, crossAvailable.intoOption())
	}

	// Either the min- or the max-content size, depending on the constraint
	// sized under.
	measuredMain := measureChildSize(item.node, childKnown, c.nodeInnerSize, childAvailable, contentSize,
		dir.mainAxis(), Line[bool]{})

	// A known cross size is transferred through the item's aspect ratio and
	// floors the measured content size.
	var transferredMain Opt
	if cross := childKnown.cross(dir); item.aspectRatio.Valid && cross.Valid {
		if c.isRow {
			transferredMain = Some(cross.Value * item.aspectRatio.Value)
		} else {
			transferredMain = Some(cross.Value / item.aspectRatio.Value)
		}
	}
	innerMain := fmaxOpt(measuredMain, transferredMain)

	// This is asymmetrical between rows and columns. It may relate to
	// https://drafts.csswg.org/css-flexbox-1/#algo-main-container ("The
	// automatic block size of a block-level flex container is its
	// max-content size"): a flex basis defining a vertical size does not
	// shrink because it is on the block axis, where the automatic size is a
	// max-content size, whereas a horizontal one does shrink because the
	// automatic inline size is a min-content size. It was found by matching
	// WebKit and Firefox rather than by reading the spec (see the
	// flex_basis_unconstraint_row and flex_basis_uncontraint_column
	// fixtures).
	if c.isRow {
		return fclamp(innerMain+rectMainSum(item.margin, dir), styleMin, styleMax)
	}
	return fclamp(fmax(innerMain, item.flexBasis)+rectMainSum(item.margin, dir), styleMin, styleMax)
}

// flexResolveFlexibleLengths resolves the flexible lengths of the items in
// a line, setting the main component of each item's target size and outer
// target size.
//
// 9.7. Resolving Flexible Lengths
// (https://www.w3.org/TR/css-flexbox-1/#resolve-flexible-lengths).
func flexResolveFlexibleLengths(line *flexLine, c *flexConstants) {
	dir := c.dir
	totalMainAxisGap := flexSumAxisGaps(c.gap.main(dir), len(line.items))
	innerMain := c.nodeInnerSize.main(dir)

	// 1. Determine the used flex factor. Sum the outer hypothetical main
	//    sizes of all items on the line. If the sum is less than the flex
	//    container's inner main size, use the flex grow factor for the rest
	//    of this algorithm; otherwise, use the flex shrink factor.
	var totalHypotheticalOuter float32
	for i := range line.items {
		totalHypotheticalOuter += line.items[i].hypotheticalOuterSize.main(dir)
	}
	usedFlexFactor := totalMainAxisGap + totalHypotheticalOuter
	growing := usedFlexFactor < innerMain.Or(0)
	shrinking := usedFlexFactor > innerMain.Or(0)
	exactlySized := !growing && !shrinking

	// 2. Size inflexible items. Freeze, setting its target main size to its
	//    hypothetical main size
	//    - Any item that has a flex factor of zero
	//    - If using the flex grow factor: any item that has a flex base size
	//      greater than its hypothetical main size
	//    - If using the flex shrink factor: any item that has a flex base
	//      size smaller than its hypothetical main size
	for i := range line.items {
		child := &line.items[i]
		innerTarget := child.hypotheticalInnerSize.main(dir)
		child.targetSize.setMain(dir, innerTarget)

		if exactlySized || (child.flexGrow == 0 && child.flexShrink == 0) ||
			(growing && child.flexBasis > child.hypotheticalInnerSize.main(dir)) ||
			(shrinking && child.flexBasis < child.hypotheticalInnerSize.main(dir)) {
			child.frozen = true
			child.outerTargetSize.setMain(dir, innerTarget+rectMainSum(child.margin, dir))
		}
	}

	if exactlySized {
		return
	}

	// 3. Calculate initial free space. Sum the outer sizes of all items on
	//    the line, and subtract this from the flex container's inner main
	//    size. For frozen items, use their outer target main size; for other
	//    items, use their outer flex base size.
	usedSpace := func() float32 {
		var sum float32
		for i := range line.items {
			child := &line.items[i]
			if child.frozen {
				sum += child.outerTargetSize.main(dir)
			} else {
				sum += child.flexBasis + rectMainSum(child.margin, dir)
			}
		}
		return totalMainAxisGap + sum
	}
	initialFreeSpace := innerMain.subF(usedSpace()).Or(0)

	// 4. Loop
	for {
		// a. Check for flexible items. If all the flex items on the line are
		//    frozen, free space has been distributed; exit this loop.
		allFrozen := true
		for i := range line.items {
			if !line.items[i].frozen {
				allFrozen = false
				break
			}
		}
		if allFrozen {
			break
		}

		// b. Calculate the remaining free space as for initial free space,
		//    above. If the sum of the unfrozen flex items' flex factors is
		//    less than one, multiply the initial free space by this sum. If
		//    the magnitude of this value is less than the magnitude of the
		//    remaining free space, use this as the remaining free space.
		used := usedSpace()

		var sumFlexGrow, sumFlexShrink float32
		for i := range line.items {
			if child := &line.items[i]; !child.frozen {
				sumFlexGrow += child.flexGrow
				sumFlexShrink += child.flexShrink
			}
		}

		var freeSpace float32
		switch {
		case growing && sumFlexGrow < 1:
			freeSpace = fminOpt(initialFreeSpace*sumFlexGrow-totalMainAxisGap, innerMain.subF(used))
		case shrinking && sumFlexShrink < 1:
			freeSpace = fmaxOpt(initialFreeSpace*sumFlexShrink-totalMainAxisGap, innerMain.subF(used))
		default:
			freeSpace = innerMain.subF(used).Or(usedFlexFactor - used)
		}

		// c. Distribute free space proportional to the flex factors.
		//    - If the remaining free space is zero
		//        Do Nothing
		//    - If using the flex grow factor
		//        Find the ratio of the item's flex grow factor to the sum of
		//        the flex grow factors of all unfrozen items on the line. Set
		//        the item's target main size to its flex base size plus a
		//        fraction of the remaining free space proportional to the
		//        ratio.
		//    - If using the flex shrink factor
		//        For every unfrozen item on the line, multiply its flex shrink
		//        factor by its inner flex base size, and note this as its
		//        scaled flex shrink factor. Find the ratio of the item's scaled
		//        flex shrink factor to the sum of the scaled flex shrink
		//        factors of all unfrozen items on the line. Set the item's
		//        target main size to its flex base size minus a fraction of
		//        the absolute value of the remaining free space proportional to
		//        the ratio. Note this may result in a negative inner main size;
		//        it will be corrected in the next step.
		//    - Otherwise
		//        Do Nothing
		if flexIsNormal(freeSpace) {
			if growing && sumFlexGrow > 0 {
				for i := range line.items {
					if child := &line.items[i]; !child.frozen {
						child.targetSize.setMain(dir, child.flexBasis+freeSpace*(child.flexGrow/sumFlexGrow))
					}
				}
			} else if shrinking && sumFlexShrink > 0 {
				var sumScaledShrinkFactor float32
				for i := range line.items {
					if child := &line.items[i]; !child.frozen {
						sumScaledShrinkFactor += child.innerFlexBasis * child.flexShrink
					}
				}
				if sumScaledShrinkFactor > 0 {
					for i := range line.items {
						if child := &line.items[i]; !child.frozen {
							scaledShrinkFactor := child.innerFlexBasis * child.flexShrink
							child.targetSize.setMain(dir,
								child.flexBasis+freeSpace*(scaledShrinkFactor/sumScaledShrinkFactor))
						}
					}
				}
			}
		}

		// d. Fix min/max violations. Clamp each non-frozen item's target main
		//    size by its used min and max main sizes and floor its content-box
		//    size at zero. If the item's target main size was made smaller by
		//    this, it's a max violation. If the item's target main size was
		//    made larger by this, it's a min violation.
		var totalViolation float32
		for i := range line.items {
			child := &line.items[i]
			if child.frozen {
				continue
			}
			target := child.targetSize.main(dir)
			clamped := fmax(fclamp(target, Some(child.resolvedMinimumMainSize), child.maxSize.main(dir)), 0)
			child.violation = clamped - target
			child.targetSize.setMain(dir, clamped)
			child.outerTargetSize.setMain(dir, clamped+rectMainSum(child.margin, dir))
			totalViolation += child.violation
		}

		// e. Freeze over-flexed items. The total violation is the sum of the
		//    adjustments from the previous step ∑(clamped size - unclamped
		//    size). If the total violation is:
		//    - Zero
		//        Freeze all items.
		//    - Positive
		//        Freeze all the items with min violations.
		//    - Negative
		//        Freeze all the items with max violations.
		for i := range line.items {
			child := &line.items[i]
			if child.frozen {
				continue
			}
			switch {
			case totalViolation > 0:
				child.frozen = child.violation > 0
			case totalViolation < 0:
				child.frozen = child.violation < 0
			default:
				child.frozen = true
			}
		}

		// f. Return to the start of this loop.
	}
}

// flexDetermineHypotheticalCrossSize determines the hypothetical cross size
// of each item in a line.
//
// 9.4. Cross Size Determination
// (https://www.w3.org/TR/css-flexbox-1/#algo-cross-item): determine the
// hypothetical cross size of each item by performing layout with the used
// main size and the available space, treating auto as fit-content.
func flexDetermineHypotheticalCrossSize(line *flexLine, c *flexConstants, available Size[AvailableSpace]) {
	dir := c.dir
	for i := range line.items {
		child := &line.items[i]
		paddingBorderSum := rectCrossSum(rectAdd(child.padding, child.border), dir)

		// The available space passed to the child excludes its margins.
		childAvailableMain := Definite(fmax(c.containerSize.main(dir)-rectMainSum(child.margin, dir), 0))

		// Sizes transferred through the aspect ratio clamp the hypothetical
		// cross size (https://github.com/w3c/csswg-drafts/issues/10997).
		transferredMinCross := optsAspectRatio(child.minSize, child.aspectRatio).cross(dir)
		transferredMaxCross := optsAspectRatio(child.maxSize, child.aspectRatio).cross(dir)

		childCross := child.size.cross(dir).clamp(transferredMinCross, transferredMaxCross).maxF(paddingBorderSum)

		childAvailableCross := available.cross(dir)
		if childAvailableCross.isDefinite() {
			childAvailableCross.Value = fmax(childAvailableCross.Value-rectCrossSum(child.margin, dir), 0)
		}
		childAvailableCross = childAvailableCross.clampOpt(transferredMinCross, transferredMaxCross).
			maxF(paddingBorderSum)

		// A cross size that is a sizing keyword (min-content, max-content,
		// fit-content, fit-content(...)) sets the available space constraint
		// the item is measured under. The stretch keyword is not resolved
		// here: it stretches to the flex line, which
		// flexDetermineUsedCrossSize handles.
		crossStretchSize := c.nodeInnerSize.cross(dir).subF(rectCrossSum(child.margin, dir)).maxF(0)
		if r, ok := resolveSizingKeyword(child.sizeStyle.cross(dir), crossStretchSize, c.nodeInnerSize.cross(dir)); ok &&
			r.measure {
			childAvailableCross = r.available
		}

		var childInnerCross float32
		if childCross.Valid {
			childInnerCross = childCross.Value
		} else {
			var known Size[Opt]
			var avail Size[AvailableSpace]
			if c.isRow {
				known = Size[Opt]{Some(child.targetSize.Width), childCross}
				avail = Size[AvailableSpace]{childAvailableMain, childAvailableCross}
			} else {
				known = Size[Opt]{childCross, Some(child.targetSize.Height)}
				avail = Size[AvailableSpace]{childAvailableCross, childAvailableMain}
			}
			out := computeChildLayout(child.node, layoutInput{
				runMode:                    computeSize,
				sizingMode:                 contentSize,
				axis:                       requestAxis(dir.crossAxis()),
				knownDimensions:            known,
				knownDimensionsAreDefinite: flexItemKnownDimensionDefiniteness(c, child),
				parentSize:                 c.nodeInnerSize,
				availableSpace:             avail,
			})
			childInnerCross = fmax(fclamp(out.size.get(dir.crossAxis()), transferredMinCross, transferredMaxCross),
				paddingBorderSum)
		}
		childOuterCross := childInnerCross + rectCrossSum(child.margin, dir)

		child.hypotheticalInnerSize.setCross(dir, childInnerCross)
		child.hypotheticalOuterSize.setCross(dir, childOuterCross)
	}
}

// flexCalculateChildrenBaselines calculates the baselines of the items that
// take part in baseline alignment.
func flexCalculateChildrenBaselines(nodeSize Size[Opt], available Size[AvailableSpace], lines []flexLine,
	c *flexConstants) {
	// Only rows compute baselines, as baseline alignment is only supported
	// on a cross axis that is also the inline axis.
	// TODO: this may need revisiting if/when vertical writing modes are
	// supported.
	if !c.isRow {
		return
	}

	for li := range lines {
		line := &lines[li]
		// With one or zero items in baseline alignment, it is a no-op.
		count := 0
		for i := range line.items {
			if line.items[i].participatesInBaselineAlignment(c.dir) {
				count++
			}
		}
		if count <= 1 {
			continue
		}

		for i := range line.items {
			child := &line.items[i]
			// Only items in baseline alignment need their baselines.
			if !child.participatesInBaselineAlignment(c.dir) {
				continue
			}

			var known Size[Opt]
			var avail Size[AvailableSpace]
			if c.isRow {
				known = Size[Opt]{Some(child.targetSize.Width), Some(child.hypotheticalInnerSize.Height)}
				avail = Size[AvailableSpace]{Definite(c.containerSize.Width), available.Height.maybeSet(nodeSize.Height)}
			} else {
				known = Size[Opt]{Some(child.hypotheticalInnerSize.Width), Some(child.targetSize.Height)}
				avail = Size[AvailableSpace]{available.Width.maybeSet(nodeSize.Width), Definite(c.containerSize.Height)}
			}
			// The available space passed to the child excludes its margins.
			avail = spacesMaxF(spacesSubF(avail, rectSumAxes(child.margin)), Size[float32]{})
			out := computeChildLayout(child.node, layoutInput{
				runMode:                    performLayout,
				sizingMode:                 contentSize,
				axis:                       requestBoth,
				knownDimensions:            known,
				knownDimensionsAreDefinite: flexItemKnownDimensionDefiniteness(c, child),
				parentSize:                 c.nodeInnerSize,
				availableSpace:             avail,
			})

			height := out.size.Height
			// A scroll container's baseline comes from its content as if
			// scrolled to the initial position, clamped to its border box
			// (https://github.com/w3c/csswg-drafts/issues/7660).
			baseline := out.baselines.first.Or(height)
			if child.overflow.Y.isScrollContainer() {
				baseline = fmax(fmin(baseline, height), 0)
			}
			child.baseline = baseline + child.margin.Top
		}
	}
}

// flexCalculateCrossSize calculates the cross size of each flex line.
//
// 9.4. Cross Size Determination
// (https://www.w3.org/TR/css-flexbox-1/#algo-cross-line).
func flexCalculateCrossSize(lines []flexLine, nodeSize Size[Opt], c *flexConstants) {
	dir := c.dir
	crossPaddingBorder := rectCrossSum(c.contentBoxInset, dir)
	crossMin := c.minSize.cross(dir)
	crossMax := c.maxSize.cross(dir)

	// If the flex container is single-line and has a definite cross size,
	// the cross size of the flex line is the flex container's inner cross
	// size.
	if !c.isWrap && nodeSize.cross(dir).Valid {
		lines[0].crossSize = nodeSize.cross(dir).clamp(crossMin, crossMax).subF(crossPaddingBorder).maxF(0).Or(0)
		return
	}

	// Otherwise, for each flex line:
	//
	//    1. Collect all the flex items whose inline-axis is parallel to the
	//       main-axis, whose align-self is baseline, and whose cross-axis
	//       margins are both non-auto. Find the largest of the distances
	//       between each item's baseline and its hypothetical outer
	//       cross-start edge, and the largest of the distances between each
	//       item's baseline and its hypothetical outer cross-end edge, and sum
	//       these two values.
	//
	//    2. Among all the items not collected by the previous step, find the
	//       largest outer hypothetical cross size.
	//
	//    3. The used cross-size of the flex line is the largest of the
	//       numbers found in the previous two steps and zero.
	for li := range lines {
		line := &lines[li]
		var maxBaseline float32
		for i := range line.items {
			maxBaseline = fmax(maxBaseline, line.items[i].baseline)
		}
		var crossSize float32
		for i := range line.items {
			child := &line.items[i]
			var size float32
			if child.participatesInBaselineAlignment(dir) {
				size = maxBaseline - child.baseline + child.hypotheticalOuterSize.cross(dir)
			} else {
				size = child.hypotheticalOuterSize.cross(dir)
			}
			crossSize = fmax(crossSize, size)
		}
		line.crossSize = crossSize
	}

	// If the flex container is single-line, clamp the line's cross size to
	// the container's computed min and max cross sizes. If CSS 2.1's
	// definition of min/max-width/height applied more generally, this would
	// fall out automatically.
	if !c.isWrap {
		lines[0].crossSize = fclamp(lines[0].crossSize, crossMin.subF(crossPaddingBorder),
			crossMax.subF(crossPaddingBorder))
	}
}

// flexHandleAlignContentStretch handles align-content: stretch.
//
// 9.4. Cross Size Determination
// (https://www.w3.org/TR/css-flexbox-1/#algo-line-stretch): if the flex
// container has a definite cross size, align-content is stretch, and the sum
// of the flex lines' cross sizes is less than the flex container's inner
// cross size, increase the cross size of each flex line by equal amounts
// such that the sum of their cross sizes exactly equals the flex container's
// inner cross size.
func flexHandleAlignContentStretch(lines []flexLine, nodeSize Size[Opt], c *flexConstants) {
	if c.alignContent != contentOf(ContentStretch) {
		return
	}
	dir := c.dir
	crossPaddingBorder := rectCrossSum(c.contentBoxInset, dir)
	crossMin := c.minSize.cross(dir)
	crossMax := c.maxSize.cross(dir)
	containerMinInnerCross := nodeSize.cross(dir).OrOpt(crossMin).clamp(crossMin, crossMax).
		subF(crossPaddingBorder).maxF(0).Or(0)

	totalCrossAxisGap := flexSumAxisGaps(c.gap.cross(dir), len(lines))
	var linesTotalCross float32
	for i := range lines {
		linesTotalCross += lines[i].crossSize
	}
	linesTotalCross += totalCrossAxisGap

	if linesTotalCross < containerMinInnerCross {
		remaining := containerMinInnerCross - linesTotalCross
		addition := remaining / float32(len(lines))
		for i := range lines {
			lines[i].crossSize += addition
		}
	}
}

// flexDetermineUsedCrossSize determines the used cross size of each flex
// item.
//
// 9.4. Cross Size Determination
// (https://www.w3.org/TR/css-flexbox-1/#algo-stretch): if a flex item has
// align-self: stretch, its computed cross size property is auto, and neither
// of its cross-axis margins are auto, the used outer cross size is the used
// cross size of its flex line, clamped according to the item's used min and
// max cross sizes. Otherwise, the used cross size is the item's hypothetical
// cross size.
//
// If the flex item has align-self: stretch, redo layout for its contents,
// treating this used size as its definite cross size so that
// percentage-sized children can be resolved.
//
// Note that this step does not affect the main size of the flex item, even
// if it has an intrinsic aspect ratio.
func flexDetermineUsedCrossSize(lines []flexLine, c *flexConstants) {
	dir := c.dir
	for li := range lines {
		line := &lines[li]
		lineCrossSize := line.crossSize

		for i := range line.items {
			child := &line.items[i]
			cs := child.node.Style()
			// A cross size of stretch stretches to the flex line like
			// align-self: stretch, whatever the alignment.
			crossIsStretch := child.sizeStyle.cross(dir).isStretch()
			if !child.marginIsAuto.crossStart(dir) && !child.marginIsAuto.crossEnd(dir) &&
				(crossIsStretch || (child.alignSelf == alignOf(AlignStretch) && cs.Size.cross(dir).IsAuto())) {
				// This use of the max size is an exception to the rule that max
				// sizes transfer through the aspect ratio. Both Chrome and Firefox
				// agree on this, and the spec reads that way, although it seems
				// the spec should apply the aspect ratio here. Percentage padding
				// and border resolve against the container's inline size (its
				// width) on all four sides.
				padding := rectResolveOrZero(cs.padding(), c.nodeInnerSize.Width)
				border := rectResolveOrZero(cs.border(), c.nodeInnerSize.Width)
				var boxSizingAdjustment Size[float32]
				if cs.BoxSizing == ContentBox {
					boxSizingAdjustment = rectSumAxes(rectAdd(padding, border))
				}
				maxSizeIgnoringAspectRatio := optsAddF(sizeMaybeResolve(cs.MaxSize, c.nodeInnerSize),
					boxSizingAdjustment)

				child.targetSize.setCross(dir, fclamp(fmax(lineCrossSize-rectCrossSum(child.margin, dir), 0),
					child.minSize.cross(dir), maxSizeIgnoringAspectRatio.cross(dir)))
			} else {
				child.targetSize.setCross(dir, child.hypotheticalInnerSize.cross(dir))
			}

			child.outerTargetSize.setCross(dir, child.targetSize.cross(dir)+rectCrossSum(child.margin, dir))
		}
	}
}

// flexDistributeRemainingFreeSpace distributes any remaining free space.
//
// 9.5. Main-Axis Alignment
// (https://www.w3.org/TR/css-flexbox-1/#algo-main-align). For each flex
// line:
//
//  1. If the remaining free space is positive and at least one main-axis
//     margin on this line is auto, distribute the free space equally among
//     these margins. Otherwise, set all auto margins to zero.
//  2. Align the items along the main-axis per justify-content.
func flexDistributeRemainingFreeSpace(lines []flexLine, c *flexConstants) {
	dir := c.dir
	for li := range lines {
		line := &lines[li]
		totalMainAxisGap := flexSumAxisGaps(c.gap.main(dir), len(line.items))
		var used float32
		for i := range line.items {
			used += line.items[i].outerTargetSize.main(dir)
		}
		usedSpace := totalMainAxisGap + used
		freeSpace := c.innerContainerSize.main(dir) - usedSpace
		numAutoMargins := 0

		for i := range line.items {
			child := &line.items[i]
			if child.marginIsAuto.mainStart(dir) {
				numAutoMargins++
			}
			if child.marginIsAuto.mainEnd(dir) {
				numAutoMargins++
			}
		}

		if freeSpace > 0 && numAutoMargins > 0 {
			margin := freeSpace / float32(numAutoMargins)

			for i := range line.items {
				child := &line.items[i]
				if child.marginIsAuto.mainStart(dir) {
					if c.isRow {
						child.margin.Left = margin
					} else {
						child.margin.Top = margin
					}
				}
				if child.marginIsAuto.mainEnd(dir) {
					if c.isRow {
						child.margin.Right = margin
					} else {
						child.margin.Bottom = margin
					}
				}
			}

			// The auto margins absorbed all the free space, leaving none for
			// justify-content.
			freeSpace = 0
		}

		numItems := len(line.items)
		layoutReverse := dir.isReverse()
		gap := c.gap.main(dir)
		mode := applyAlignmentFallback(freeSpace, numItems, c.justifyContent)

		for i := range line.items {
			index := i
			if layoutReverse {
				index = numItems - 1 - i
			}
			line.items[index].offsetMain = computeAlignmentOffset(freeSpace, numItems, gap, mode, layoutReverse, i == 0)
		}
	}
}

// flexResolveCrossAxisAutoMargins resolves cross-axis auto margins.
//
// 9.6. Cross-Axis Alignment
// (https://www.w3.org/TR/css-flexbox-1/#algo-cross-margins). If a flex item
// has auto cross-axis margins:
//
//   - If its outer cross size (treating those auto margins as zero) is less
//     than the cross size of its flex line, distribute the difference in
//     those sizes equally to the auto margins.
//   - Otherwise, if the block-start or inline-start margin (whichever is in
//     the cross axis) is auto, set it to zero. Set the opposite margin so
//     that the outer cross size of the item equals the cross size of its
//     flex line.
func flexResolveCrossAxisAutoMargins(lines []flexLine, c *flexConstants) {
	dir := c.dir
	for li := range lines {
		line := &lines[li]
		lineCrossSize := line.crossSize
		var maxBaseline, maxBaselineToBottom float32
		for i := range line.items {
			maxBaseline = fmax(maxBaseline, line.items[i].baseline)
		}
		for i := range line.items {
			if child := &line.items[i]; child.participatesInBaselineAlignment(dir) {
				maxBaselineToBottom = fmax(maxBaselineToBottom, child.outerTargetSize.cross(dir)-child.baseline)
			}
		}

		for i := range line.items {
			child := &line.items[i]
			freeSpace := lineCrossSize - child.outerTargetSize.cross(dir)
			startAuto, endAuto := child.marginIsAuto.crossStart(dir), child.marginIsAuto.crossEnd(dir)

			switch {
			case startAuto && endAuto:
				if c.isRow {
					child.margin.Top = freeSpace / 2
					child.margin.Bottom = freeSpace / 2
				} else {
					child.margin.Left = freeSpace / 2
					child.margin.Right = freeSpace / 2
				}
			case startAuto:
				if c.isRow {
					child.margin.Top = freeSpace
				} else {
					child.margin.Left = freeSpace
				}
			case endAuto:
				if c.isRow {
					child.margin.Bottom = freeSpace
				} else {
					child.margin.Right = freeSpace
				}
			default:
				// 14. Align all flex items along the cross-axis.
				child.offsetCross = flexAlignItemAlongCrossAxis(child, freeSpace, maxBaseline, maxBaselineToBottom, c)
			}
		}
	}
}

// flexAlignItemAlongCrossAxis aligns an item along the cross axis per
// align-self, when neither of its cross-axis margins is auto.
//
// 9.6. Cross-Axis Alignment
// (https://www.w3.org/TR/css-flexbox-1/#algo-cross-align).
func flexAlignItemAlongCrossAxis(child *flexItem, freeSpace, maxBaseline, maxBaselineToBottom float32,
	c *flexConstants) float32 {
	crossAxisShouldReverse := c.isColumn && c.layoutDirection.isRTL()

	// If align-self is safe and the item would overflow its line, fall back
	// to logical start to avoid data loss (CSS Box Alignment 3 §4.3,
	// https://www.w3.org/TR/css-align-3/#overflow-values).
	keyword := child.alignSelf.Keyword
	if child.alignSelf.isSafe() && freeSpace < 0 {
		keyword = AlignStart
	}

	startOrFree := func(atFree bool) float32 {
		if atFree {
			return freeSpace
		}
		return 0
	}
	switch keyword {
	case AlignStart:
		return startOrFree(crossAxisShouldReverse)
	case AlignFlexStart:
		return startOrFree(c.isWrapReverse != crossAxisShouldReverse)
	case AlignEnd:
		return startOrFree(!crossAxisShouldReverse)
	case AlignFlexEnd:
		return startOrFree(c.isWrapReverse == crossAxisShouldReverse)
	case AlignCenter:
		return freeSpace / 2
	case AlignBaseline:
		if c.isRow {
			if c.isWrapReverse {
				// In a wrap-reverse container the cross axis is flipped, so the
				// baseline-aligned group aligns to the cross-start edge, the
				// bottom of the line.
				lineCrossSize := freeSpace + child.outerTargetSize.cross(c.dir)
				return lineCrossSize - maxBaselineToBottom - child.baseline
			}
			return maxBaseline - child.baseline
		}
		// Until vertical writing modes are supported, baseline alignment only
		// makes sense in rows, so columns treat it as flex-start.
		baselineColumnShouldReverse := crossAxisShouldReverse && !c.isWrap
		return startOrFree(c.isWrapReverse != baselineColumnShouldReverse)
	case AlignStretch:
		return startOrFree(c.isWrapReverse != crossAxisShouldReverse)
	}
	// Normal is resolved to stretch, and self-start and self-end to start
	// and end, when items are generated.
	panic("layout: unresolved flex item alignment")
}

// flexDetermineContainerCrossSize determines the flex container's used
// cross size, and returns the total cross size of its lines.
//
// 9.6. Cross-Axis Alignment
// (https://www.w3.org/TR/css-flexbox-1/#algo-cross-container):
//
//   - If the cross size property is a definite size, use that, clamped by
//     the used min and max cross sizes of the flex container.
//   - Otherwise, use the sum of the flex lines' cross sizes, clamped by the
//     used min and max cross sizes of the flex container.
func flexDetermineContainerCrossSize(lines []flexLine, nodeSize Size[Opt], c *flexConstants) float32 {
	dir := c.dir
	totalCrossAxisGap := flexSumAxisGaps(c.gap.cross(dir), len(lines))
	var totalLineCrossSize float32
	for i := range lines {
		totalLineCrossSize += lines[i].crossSize
	}

	paddingBorderSum := rectCrossSum(c.contentBoxInset, dir)
	crossScrollbarGutter := c.scrollbarGutter.cross(dir)
	outer := fmax(fclamp(nodeSize.cross(dir).Or(totalLineCrossSize+totalCrossAxisGap+paddingBorderSum),
		c.minSize.cross(dir), c.maxSize.cross(dir)), paddingBorderSum-crossScrollbarGutter)
	inner := fmax(outer-paddingBorderSum, 0)

	c.containerSize.setCross(dir, outer)
	c.innerContainerSize.setCross(dir, inner)
	return totalLineCrossSize
}

// flexAlignLinesPerAlignContent aligns all flex lines per align-content.
//
// 9.6. Cross-Axis Alignment
// (https://www.w3.org/TR/css-flexbox-1/#algo-line-align).
func flexAlignLinesPerAlignContent(lines []flexLine, c *flexConstants, totalCrossSize float32) {
	numLines := len(lines)
	gap := c.gap.cross(c.dir)
	totalCrossAxisGap := flexSumAxisGaps(gap, numLines)
	freeSpace := c.innerContainerSize.cross(c.dir) - totalCrossSize - totalCrossAxisGap

	mode := applyAlignmentFallback(freeSpace, numLines, c.alignContent)

	for i := range lines {
		index := i
		if c.isWrapReverse {
			index = numLines - 1 - i
		}
		lines[index].offsetCross = computeAlignmentOffset(freeSpace, numLines, gap, mode, c.isWrapReverse, i == 0)
	}
}

// flexCalculateItem lays out an item at its final size and position.
func flexCalculateItem(item *flexItem, totalOffsetMain *float32, totalOffsetCross, lineOffsetCross float32,
	totalOverflow *Rect[float32], border Rect[float32], c *flexConstants) {
	containerSize := c.containerSize
	direction := c.dir
	layoutDirection := c.layoutDirection
	out := computeChildLayout(item.node, layoutInput{
		runMode:                    performLayout,
		sizingMode:                 contentSize,
		axis:                       requestBoth,
		knownDimensions:            sizeSome(item.targetSize),
		knownDimensionsAreDefinite: flexItemKnownDimensionDefiniteness(c, item),
		parentSize:                 c.nodeInnerSize,
		// The available space passed to the child excludes its margins.
		availableSpace: spacesDefinite(sizeMax(sizeSub(containerSize, rectSumAxes(item.margin)), Size[float32]{})),
	})
	size := out.size

	isRTLRow := direction.isRow() && layoutDirection.isRTL()
	isRTLColumn := direction.isColumn() && layoutDirection.isRTL()
	mainRelativeInset := item.relativeInset.main(direction)
	crossRelativeInset := item.relativeInset.cross(direction)
	effectiveLineOffsetCross := lineOffsetCross
	if isRTLColumn {
		effectiveLineOffsetCross = 0
	}

	var offsetMain float32
	if isRTLRow {
		offsetMain = *totalOffsetMain - item.offsetMain - item.margin.mainEnd(direction) - mainRelativeInset - size.Width
	} else {
		offsetMain = *totalOffsetMain + item.offsetMain + item.margin.mainStart(direction) + mainRelativeInset
	}

	offsetCross := totalOffsetCross + item.offsetCross + effectiveLineOffsetCross + item.margin.crossStart(direction) +
		crossRelativeInset

	if direction.isRow() {
		baselineOffsetCross := totalOffsetCross + item.offsetCross + effectiveLineOffsetCross +
			item.margin.crossStart(direction)
		// A scroll container's baseline comes from its content as if scrolled
		// to the initial position, clamped to its border box
		// (https://github.com/w3c/csswg-drafts/issues/7660).
		innerBaseline := out.baselines.first.Or(size.Height)
		if item.overflow.Y.isScrollContainer() {
			innerBaseline = fmax(fmin(innerBaseline, size.Height), 0)
		}
		item.baseline = baselineOffsetCross + innerBaseline
	} else {
		baselineOffsetMain := *totalOffsetMain + item.offsetMain + item.margin.mainStart(direction)
		innerBaseline := out.baselines.first.Or(size.Height)
		item.baseline = baselineOffsetMain + innerBaseline
	}

	location := Point[float32]{offsetCross, offsetMain}
	if direction.isRow() {
		location = Point[float32]{offsetMain, offsetCross}
	}
	var scrollbarSize Size[float32]
	if item.overflow.Y == OverflowScroll {
		scrollbarSize.Width = item.scrollbarWidth
	}
	if item.overflow.X == OverflowScroll {
		scrollbarSize.Height = item.scrollbarWidth
	}

	item.node.LayoutState().Unrounded = Layout{
		Order:              item.order,
		Size:               size,
		ScrollableOverflow: out.scrollableOverflow,
		ScrollbarSize:      scrollbarSize,
		Location:           location,
		Padding:            item.padding,
		Border:             item.border,
		Margin:             item.margin,
	}

	// Keep the out-of-flow candidates from the item's subtree, moved from
	// item-relative to container-relative coordinates.
	item.oofCandidates = out.oofCandidates
	if len(item.oofCandidates) > 0 {
		translateCandidates(item.oofCandidates, location)
	}

	if isRTLRow {
		*totalOffsetMain -= item.offsetMain + rectMainSum(item.margin, direction) + size.main(direction)
	} else {
		*totalOffsetMain += item.offsetMain + rectMainSum(item.margin, direction) + size.main(direction)
	}

	contributionLocation := Point[float32]{location.X - border.Left, location.Y - border.Top}
	if layoutDirection.isRTL() {
		contributionLocation.X = containerSize.Width - (location.X + size.Width) - border.Right
	}
	*totalOverflow = rectUnion(*totalOverflow, scrollableOverflowContribution(contributionLocation, size,
		out.scrollableOverflow, item.overflow, item.contain, c.isScrollContainer))
}

// flexCalculateLayoutLine lays out the items of a line.
func flexCalculateLayoutLine(line *flexLine, totalOffsetCross *float32, overflow *Rect[float32],
	border Rect[float32], c *flexConstants) {
	containerSize := c.containerSize
	paddingBorder := c.contentBoxInset
	direction := c.dir
	layoutDirection := c.layoutDirection
	totalOffsetMain := paddingBorder.mainStart(direction)
	if layoutDirection.isRTL() && direction.isRow() {
		totalOffsetMain = containerSize.Width - paddingBorder.mainEnd(direction)
	}
	lineOffsetCross := line.offsetCross

	isRTLColumn := layoutDirection.isRTL() && direction.isColumn()
	if isRTLColumn {
		*totalOffsetCross -= lineOffsetCross + line.crossSize
	}

	n := len(line.items)
	for i := range line.items {
		index := i
		if direction.isReverse() {
			index = n - 1 - i
		}
		flexCalculateItem(&line.items[index], &totalOffsetMain, *totalOffsetCross, lineOffsetCross, overflow, border, c)
	}

	if !isRTLColumn {
		*totalOffsetCross += lineOffsetCross + line.crossSize
	}
}

// flexFinalLayoutPass lays out every item at its final size and position,
// and returns the in-flow scrollable overflow.
func flexFinalLayoutPass(lines []flexLine, c *flexConstants) Rect[float32] {
	totalOffsetCross := c.contentBoxInset.crossStart(c.dir)
	if c.isColumn && c.layoutDirection.isRTL() {
		totalOffsetCross = c.containerSize.Width - c.contentBoxInset.crossEnd(c.dir)
	}

	var overflow Rect[float32]
	n := len(lines)
	for i := range lines {
		index := i
		if c.isWrapReverse {
			index = n - 1 - i
		}
		flexCalculateLayoutLine(&lines[index], &totalOffsetCross, &overflow, c.border, c)
	}

	// A scroll container's own padding at the end of the content is part of
	// its scrollable overflow; other boxes don't extend their overflow by
	// their padding.
	if c.isScrollContainer {
		if c.layoutDirection.isRTL() {
			overflow.Right += c.contentBoxInset.Left - c.border.Left - c.scrollbarGutter.X
		} else {
			overflow.Right += c.contentBoxInset.Right - c.border.Right - c.scrollbarGutter.X
		}
		overflow.Bottom += c.contentBoxInset.Bottom - c.border.Bottom - c.scrollbarGutter.Y
	}
	return overflow
}

// flexCollectOofCandidates collects out-of-flow candidates in document
// order: direct out-of-flow children, with static positions per the flexbox
// alignment rules, interleaved with the candidates bubbled out of each
// in-flow item's subtree. They are sized and positioned by the out-of-flow
// pass at their containing block.
func flexCollectOofCandidates(n Node, c *flexConstants, items []flexItem) []oofCandidate {
	st := n.LayoutState()
	candidates := st.spare[:0]
	st.spare = nil // in use until computeOofLayout is done with it
	// Lines are contiguous runs of the items in document order (reversal
	// applies to positions, not storage), so the items are sorted by order.
	next := 0
	nextItem := func() *flexItem {
		for next < len(items) && len(items[next].oofCandidates) == 0 {
			next++
		}
		if next < len(items) {
			return &items[next]
		}
		return nil
	}

	dir := c.dir
	containerSize := c.containerSize
	inset := c.contentBoxInset

	mainIsRTL := c.isRow && c.layoutDirection.isRTL()
	crossIsRTL := !c.isRow && c.layoutDirection.isRTL()
	mainFlexStartReversed := dir.isReverse() != mainIsRTL
	crossFlexStartReversed := c.isWrapReverse != crossIsRTL

	// The static-position area on each axis is the container's content box.
	mainArea := Line[float32]{inset.mainStart(dir), containerSize.main(dir) - inset.mainEnd(dir)}
	crossArea := Line[float32]{inset.crossStart(dir), containerSize.cross(dir) - inset.crossEnd(dir)}

	for order, count := 0, n.ChildCount(); order < count; order++ {
		child := n.Child(order)
		cs := child.Style()
		position := cs.Position

		if cs.Display == DisplayNone {
			continue
		}
		// In-flow item: merge the candidates bubbled out of its subtree.
		if !position.isOutOfFlow() {
			if item := nextItem(); item != nil && item.order == uint32(order) {
				candidates = append(candidates, item.oofCandidates...)
				item.oofCandidates = nil
				next++
			}
			continue
		}

		alignSelf := flexResolveNormalAlignment(cs.alignSelfOr(c.alignItems)).
			resolveSelfRelative(cs.Direction, c.layoutDirection, c.isColumn)

		// Main-axis static position (justify-content).
		//
		// Stretch is invalid for justify-content in flexbox, so it is treated
		// as unset (flex-start). Normal is already resolved to flex-start.
		//
		// The safe overflow-position keyword is deliberately not applied here,
		// even when the item would overflow the main axis: Chrome does not
		// apply safe fallback to justify-content for absolutely positioned
		// flex items (only to the cross-axis align-self). Matching Chrome over
		// a strict reading of the spec keeps the fixtures passing.
		//
		// start and end are writing-mode relative (they flip for RTL but not
		// for reversed flex directions), whereas flex-start, flex-end and the
		// distributed keywords' fallbacks are flex-relative.
		mainKeyword := c.justifyContent.Keyword
		mainStartPosition := true
		switch mainKeyword {
		case ContentStart:
			mainStartPosition = !mainIsRTL
		case ContentEnd:
			mainStartPosition = mainIsRTL
		}
		var mainEdge axisStaticEdge
		switch mainKeyword {
		case ContentStart, ContentEnd:
			if mainStartPosition {
				mainEdge = edgeStart
			} else {
				mainEdge = edgeEnd
			}
		case ContentSpaceBetween, ContentNormal, ContentStretch, ContentFlexStart:
			if mainFlexStartReversed {
				mainEdge = edgeEnd
			} else {
				mainEdge = edgeStart
			}
		case ContentFlexEnd:
			if mainFlexStartReversed {
				mainEdge = edgeStart
			} else {
				mainEdge = edgeEnd
			}
		default:
			mainEdge = edgeCenter
		}

		// Cross-axis static position (align-self).
		//
		// start and end (and baseline, whose static-position fallback is
		// start) are writing-mode relative: they flip for RTL but not for
		// wrap-reverse. flex-start, flex-end and the stretch fallback are
		// flex-relative.
		crossEdgeFor := func(k AlignKeyword) axisStaticEdge {
			startPosition := true
			switch k {
			case AlignStart, AlignBaseline:
				startPosition = !crossIsRTL
			case AlignEnd:
				startPosition = crossIsRTL
			}
			switch k {
			case AlignStart, AlignEnd, AlignBaseline:
				// Stretch alignment does not apply to absolutely positioned
				// items: see "Example 3" at
				// https://www.w3.org/TR/css-flexbox-1/#abspos-items. Stretch
				// should be flex-start, not start, once both are supported.
				if startPosition {
					return edgeStart
				}
				return edgeEnd
			case AlignStretch, AlignFlexStart:
				if crossFlexStartReversed {
					return edgeEnd
				}
				return edgeStart
			case AlignFlexEnd:
				if crossFlexStartReversed {
					return edgeStart
				}
				return edgeEnd
			case AlignCenter:
				return edgeCenter
			}
			// Normal is resolved to stretch, and self-start and self-end to
			// start and end, where align-self is read above.
			panic("layout: unresolved flex item alignment")
		}

		cross := axisStaticPosition{
			area: crossArea,
			align: axisStaticAlign{
				keyword:  crossEdgeFor(alignSelf.Keyword),
				safety:   alignSelf.Safety,
				fallback: crossEdgeFor(resolveSelfAlignmentSafety(alignSelf, true)),
			},
		}
		main := axisStaticPosition{area: mainArea, align: staticAlignOf(mainEdge)}

		sp := Point[axisStaticPosition]{X: cross, Y: main}
		if c.isRow {
			sp = Point[axisStaticPosition]{X: main, Y: cross}
		}
		candidates = append(candidates, oofCandidate{node: child, order: uint32(order), position: position,
			staticPosition: sp})
	}
	return candidates
}

// flexSumAxisGaps is the total space taken by the gaps between numItems
// items or lines.
func flexSumAxisGaps(gap float32, numItems int) float32 {
	// Gaps only exist between items, so with fewer than two items there are
	// none.
	if numItems <= 1 {
		return 0
	}
	return gap * float32(numItems-1)
}

// flexIsNormal is Rust's f32::is_normal: neither zero, subnormal, infinite
// nor NaN.
func flexIsNormal(v float32) bool {
	exp := math.Float32bits(v) >> 23 & 0xFF
	return exp != 0 && exp != 0xFF
}

// flexTotalCmp is Rust's f32::total_cmp.
func flexTotalCmp(a, b float32) int {
	key := func(v float32) int32 {
		bits := int32(math.Float32bits(v))
		return bits ^ int32(uint32(bits>>31)>>1)
	}
	ka, kb := key(a), key(b)
	switch {
	case ka < kb:
		return -1
	case ka > kb:
		return 1
	}
	return 0
}
