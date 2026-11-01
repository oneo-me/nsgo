package layout

import "slices"

// Out-of-flow (absolutely and fixed positioned) boxes are hoisted to their
// containing block: container algorithms report them as candidates with a
// static position, candidates bubble up the tree until a node claims them,
// and that node sizes and positions them against its padding box and
// records them as its hoisted children.

// resolveStaticOffset is the offset of an out-of-flow box's border box from
// its static position, given its final size and margins: its margin box is
// aligned within the static-position area, falling back when a safe
// alignment overflows.
func resolveStaticOffset(sp Point[axisStaticPosition], size Size[float32], margin Rect[float32]) Point[float32] {
	return Point[float32]{
		X: resolveStaticOffsetAxis(sp.X, size.Width, Line[float32]{margin.Left, margin.Right}),
		Y: resolveStaticOffsetAxis(sp.Y, size.Height, Line[float32]{margin.Top, margin.Bottom}),
	}
}

func resolveStaticOffsetAxis(sp axisStaticPosition, size float32, margin Line[float32]) float32 {
	marginBox := size + margin.Start + margin.End
	keyword := sp.align.keyword
	if sp.align.safety == Safe && marginBox > sp.area.End-sp.area.Start {
		keyword = sp.align.fallback
	}
	return alignInLine(sp.area, marginBox, keyword) + margin.Start
}

// alignInLine positions a box of size within container, aligned to edge.
func alignInLine(container Line[float32], size float32, edge axisStaticEdge) float32 {
	switch edge {
	case edgeEnd:
		return container.End - size
	case edgeCenter:
		return (container.Start + container.End - size) / 2
	}
	return container.Start
}

// oofAlignment is an out-of-flow box's self-alignment on one axis, relative
// to the containing block's writing mode.
type oofAlignment uint8

const (
	// oofNormal (normal or auto) stretch-fits non-replaced, non-table boxes
	// and aligns them to the start.
	oofNormal oofAlignment = iota
	oofStretch
	oofStart
	oofCenter
	oofEnd
)

// oofAlignmentOf converts an align-self or justify-self whose self-start and
// self-end are resolved.
func oofAlignmentOf(a Align) oofAlignment {
	switch a.Keyword {
	case AlignNormal, AlignAuto:
		return oofNormal
	case AlignStretch:
		return oofStretch
	case AlignCenter:
		return oofCenter
	case AlignEnd, AlignFlexEnd, AlignSelfEnd:
		return oofEnd
	}
	// The fallback alignment of baseline is start.
	return oofStart
}

// physicalEdge is the edge of the containing block the margin box aligns to
// once sized. Normal and stretch behave as start.
func (a oofAlignment) physicalEdge(reversed bool) axisStaticEdge {
	switch {
	case a == oofCenter:
		return edgeCenter
	case (a == oofEnd) != reversed:
		return edgeEnd
	}
	return edgeStart
}

// oofAxis holds the inputs of the absolute positioning layout model on one
// axis (https://www.w3.org/TR/css-position-3/#abspos-layout). Coordinates
// are physical and relative to the start of the containing block's
// inset-resolution area.
type oofAxis struct {
	cbSize         float32
	inset          Line[Opt] // absent is auto
	margin         Line[Opt] // absent is auto
	staticPosition axisStaticPosition
	alignment      oofAlignment
	safety         AlignmentSafety
	// isInline says whether this is the containing block's inline axis.
	isInline bool
	// reversed says whether the containing block's logical start is this
	// axis's physical end (the inline axis of an RTL containing block).
	reversed bool
	// isScrollAxis says whether the containing block scrolls on this axis.
	isScrollAxis bool
}

func (a *oofAxis) startEdge() axisStaticEdge {
	if a.reversed {
		return edgeEnd
	}
	return edgeStart
}

func (a *oofAxis) endEdge() axisStaticEdge {
	if a.reversed {
		return edgeStart
	}
	return edgeEnd
}

// insetModifiedContainingBlock is the IMCB on this axis
// (https://www.w3.org/TR/css-position-3/#resolving-insets).
func (a *oofAxis) insetModifiedContainingBlock() Line[float32] {
	var imcb Line[float32]
	var weaker axisStaticEdge
	switch {
	case a.inset.Start.Valid && a.inset.End.Valid:
		imcb, weaker = Line[float32]{a.inset.Start.Value, a.cbSize - a.inset.End.Value}, a.endEdge()
	case a.inset.Start.Valid:
		// A lone auto inset resolves to zero and is the weaker inset.
		imcb, weaker = Line[float32]{a.inset.Start.Value, a.cbSize}, edgeEnd
	case a.inset.End.Valid:
		imcb, weaker = Line[float32]{0, a.cbSize - a.inset.End.Value}, edgeStart
	default:
		// Both insets are auto: resolve them from the static position
		// rectangle and the alignment within it.
		sp := a.staticPosition
		switch sp.align.keyword {
		case edgeStart:
			imcb = Line[float32]{sp.area.Start, a.cbSize}
		case edgeEnd:
			imcb = Line[float32]{0, sp.area.End}
		default:
			// Center the IMCB on the static position rectangle's center,
			// bounded by the nearer edge of the containing block.
			center := (sp.area.Start + sp.area.End) / 2
			startDistance := center
			endDistance := a.cbSize - center
			if abs32(startDistance) <= abs32(endDistance) {
				imcb = Line[float32]{0, 2 * startDistance}
			} else {
				imcb = Line[float32]{a.cbSize - 2*endDistance, a.cbSize}
			}
		}
		weaker = a.endEdge()
	}

	// Overconstrained insets: the weaker inset is reduced to bring the IMCB
	// size up to zero.
	if imcb.End < imcb.Start {
		if weaker == edgeStart {
			imcb.Start = imcb.End
		} else {
			imcb.End = imcb.Start
		}
	}
	return imcb
}

// stretchFitSize is the IMCB minus the non-auto margins
// (https://www.w3.org/TR/css-position-3/#abspos-auto-size).
func (a *oofAxis) stretchFitSize() float32 {
	imcb := a.insetModifiedContainingBlock()
	return fmax(imcb.End-imcb.Start-(a.margin.Start.Or(0)+a.margin.End.Or(0)), 0)
}

// isStretchSized says whether an auto size on this axis is the stretch-fit
// size rather than fit-content.
func (a *oofAxis) isStretchSized(isReplaced, isTable bool) bool {
	if !a.inset.Start.Valid || !a.inset.End.Valid {
		return false
	}
	switch a.alignment {
	case oofStretch:
		return true
	case oofNormal:
		return !isReplaced && !isTable
	}
	return false
}

// resolveMargins resolves auto margins given the box's final size
// (https://www.w3.org/TR/css-position-3/#abspos-margins).
func (a *oofAxis) resolveMargins(size float32) Line[float32] {
	bothInsets := a.inset.Start.Valid && a.inset.End.Valid
	imcb := a.insetModifiedContainingBlock()
	free := imcb.End - imcb.Start - size
	switch {
	case a.margin.Start.Valid && a.margin.End.Valid:
		return Line[float32]{a.margin.Start.Value, a.margin.End.Value}
	case !bothInsets:
		// If either inset is auto then auto margins resolve to zero.
		return Line[float32]{a.margin.Start.Or(0), a.margin.End.Or(0)}
	case a.margin.Start.Valid:
		return Line[float32]{a.margin.Start.Value, free - a.margin.Start.Value}
	case a.margin.End.Valid:
		return Line[float32]{free - a.margin.End.Value, a.margin.End.Value}
	}
	// On the inline axis, negative free space goes entirely to the end
	// margin.
	if free < 0 && a.isInline {
		if a.reversed {
			return Line[float32]{free, 0}
		}
		return Line[float32]{0, free}
	}
	return Line[float32]{free / 2, free / 2}
}

// position is the physical start of the border box given its final size
// and margins (https://www.w3.org/TR/css-position-3/#abspos-alignment).
func (a *oofAxis) position(size float32, margin Line[float32]) float32 {
	imcb := a.insetModifiedContainingBlock()
	switch {
	case !a.inset.Start.Valid && !a.inset.End.Valid:
		// Static alignment, with safe overflow relative to the IMCB.
		sp := a.staticPosition
		marginBox := size + margin.Start + margin.End
		var offset float32
		if sp.align.safety == Safe && marginBox > imcb.End-imcb.Start {
			offset = alignInLine(imcb, marginBox, a.startEdge())
		} else {
			offset = alignInLine(sp.area, marginBox, sp.align.keyword)
		}
		return offset + margin.Start
	case !a.inset.End.Valid:
		// One auto inset: align the margin box to the stronger inset.
		return imcb.Start + margin.Start
	case !a.inset.Start.Valid:
		return imcb.End - size - margin.End
	}

	// Auto margins have absorbed all the free space.
	if !a.margin.Start.Valid || !a.margin.End.Valid {
		return imcb.Start + margin.Start
	}

	marginBox := size + margin.Start + margin.End
	imcbSize := imcb.End - imcb.Start
	edge := a.alignment.physicalEdge(a.reversed)
	var start float32
	switch a.safety {
	case Unsafe:
		start = alignInLine(imcb, marginBox, edge)
	case Safe:
		if marginBox > imcbSize {
			edge = a.startEdge()
		}
		start = alignInLine(imcb, marginBox, edge)
	default:
		// The default overflow alignment of absolutely positioned boxes
		// (https://www.w3.org/TR/css-align-3/#auto-safety-position).
		if marginBox <= imcbSize || a.alignment == oofNormal {
			// 1. It fits in the IMCB: align as specified.
			start = alignInLine(imcb, marginBox, edge)
			break
		}
		// The overflow limit rect bounds the IMCB and the containing block,
		// extended to infinity in the scrollable direction of a scroll
		// container.
		limit := Line[float32]{fmin(imcb.Start, 0), fmax(imcb.End, a.cbSize)}
		if a.isScrollAxis {
			if a.reversed {
				limit.Start = negInf32
			} else {
				limit.End = inf32
			}
		}
		if marginBox <= limit.End-limit.Start {
			// 2. It fits in the limit rect: cover the IMCB and align as
			//    specified as far as possible without overflowing the limit.
			start = fmax(fmin(alignInLine(imcb, marginBox, edge), limit.End-marginBox), limit.Start)
		} else {
			// 3. Start-align in the limit rect.
			start = alignInLine(limit, marginBox, a.startEdge())
		}
	}
	return start + margin.Start
}

// computeOofLayout runs the out-of-flow pass of node after its layout
// algorithm produced out: it lays out the candidates node is the containing
// block for, records them as its hoisted children, leaves the rest in
// out.oofCandidates to bubble up, and adds the claimed boxes' scrollable
// overflow. It does nothing for outputs without a positioning area (leaves,
// measurements).
func computeOofLayout(n Node, out *layoutOutput) {
	if !out.hasOofArea {
		return
	}
	style := n.Style()
	direction := style.Direction
	claims := style.isContainingBlock()

	candidates := out.oofCandidates
	out.oofCandidates = nil
	st := n.LayoutState()
	// Always reset the hoisted list, even to empty, so that lists recorded
	// by previous layouts do not persist.
	hoisted := st.hoisted[:0]
	var unclaimed []oofCandidate
	overflowRect := performOofLayout(n, candidates, out.oofArea.size, out.oofArea.offset, direction, claims,
		style.Overflow, &hoisted, &unclaimed)
	st.hoisted = hoisted
	out.oofCandidates = unclaimed
	out.scrollableOverflow = rectUnion(out.scrollableOverflow, overflowRect)
}

// oofBoxLayout is the result of layoutOofBox.
type oofBoxLayout struct {
	// layout is the box's final layout, relative to the containing block's
	// border box.
	layout Layout
	// surfaced are out-of-flow candidates from the box's subtree, relative
	// to the box's border box.
	surfaced []oofCandidate
	overflow Point[Overflow]
	contain  Contain
}

// layoutOofBox sizes and positions one out-of-flow box against its
// containing block and stores its layout.
//
// areaSize and areaOffset describe the containing block's inset-resolution
// area (its border box minus borders and scrollbar gutters) relative to its
// border box. containerOverflow is the containing block's overflow, which
// decides whether an overflowing box may be aligned into its scrollable
// area. If gridOwner is a grid container, the box is positioned against the
// grid area its placement properties name. beforeLayout, if set, sees the
// inputs of the final layout pass before it runs.
func layoutOofBox(candidate oofCandidate, areaSize Size[float32], areaOffset Point[float32], direction Direction,
	containerOverflow Point[Overflow], gridOwner Node, beforeLayout func(*layoutInput)) oofBoxLayout {
	node := candidate.node
	cs := node.Style()

	// A grid container positions the box against the grid area of its
	// placement properties (which falls back to the passed area for auto
	// placement).
	if gridOwner != nil {
		if info := gridOwner.LayoutState().grid; info != nil {
			areaRect := Rect[float32]{
				Left:   areaOffset.X,
				Right:  areaOffset.X + areaSize.Width,
				Top:    areaOffset.Y,
				Bottom: areaOffset.Y + areaSize.Height,
			}
			area := info.resolveAbsoluteGridArea(cs.GridRow, cs.GridColumn, direction, areaRect)
			areaSize = Size[float32]{area.Right - area.Left, area.Bottom - area.Top}
			areaOffset = Point[float32]{area.Left, area.Top}
		}
	}
	areaWidth, areaHeight := areaSize.Width, areaSize.Height
	areaOpts := sizeSome(areaSize)

	aspectRatio := cs.AspectRatio
	styleMargin := cs.margin()
	margin := Rect[Opt]{
		Left:   styleMargin.Left.resolveToOption(areaWidth),
		Right:  styleMargin.Right.resolveToOption(areaWidth),
		Top:    styleMargin.Top.resolveToOption(areaWidth),
		Bottom: styleMargin.Bottom.resolveToOption(areaWidth),
	}
	padding := rectResolveOrZero(cs.padding(), Some(areaWidth))
	border := rectResolveOrZero(cs.border(), Some(areaWidth))
	paddingBorderSum := rectSumAxes(rectAdd(padding, border))
	var boxSizingAdjustment Size[float32]
	if cs.BoxSizing == ContentBox {
		boxSizingAdjustment = paddingBorderSum
	}

	// Resolve the insets.
	left := cs.Inset.Left.maybeResolve(Some(areaWidth))
	right := cs.Inset.Right.maybeResolve(Some(areaWidth))
	top := cs.Inset.Top.maybeResolve(Some(areaHeight))
	bottom := cs.Inset.Bottom.maybeResolve(Some(areaHeight))

	// Known dimensions from the size styles.
	sizeStyle := cs.Size
	styleSize := optsAddF(optsAspectRatio(sizeMaybeResolve(sizeStyle, areaOpts), aspectRatio), boxSizingAdjustment)
	minSize := optsMaxF(optsOr(
		optsAddF(optsAspectRatio(sizeMaybeResolve(cs.MinSize, areaOpts), aspectRatio), boxSizingAdjustment),
		sizeSome(paddingBorderSum)), paddingBorderSum)
	maxSize := optsAddF(optsAspectRatio(sizeMaybeResolve(cs.MaxSize, areaOpts), aspectRatio), boxSizingAdjustment)
	known := optsClamp(styleSize, minSize, maxSize)

	isReplaced := cs.isReplaced()
	isTable := cs.ItemIsTable
	itemDirection := cs.Direction
	// Taffy supports only the horizontal-tb writing mode, so only the inline
	// axis can be reversed. Auto behaves as normal for absolutely positioned
	// boxes: it does not defer to the containing block's *-items.
	justifySelf := cs.justifySelfOr(alignOf(AlignNormal)).resolveSelfRelative(itemDirection, direction, true)
	alignSelf := cs.alignSelfOr(alignOf(AlignNormal)).resolveSelfRelative(itemDirection, direction, false)
	overflow := cs.Overflow
	scrollbarWidth := cs.ScrollbarWidth
	contain := cs.Contain

	// Static positions are relative to the containing block's border box:
	// make them relative to the inset-resolution area.
	sp := candidate.staticPosition
	sp.X.area.Start -= areaOffset.X
	sp.X.area.End -= areaOffset.X
	sp.Y.area.Start -= areaOffset.Y
	sp.Y.area.End -= areaOffset.Y

	horizontal := oofAxis{
		cbSize:         areaWidth,
		inset:          Line[Opt]{left, right},
		margin:         Line[Opt]{margin.Left, margin.Right},
		staticPosition: sp.X,
		alignment:      oofAlignmentOf(justifySelf),
		safety:         justifySelf.Safety,
		isInline:       true,
		reversed:       direction.isRTL(),
		isScrollAxis:   containerOverflow.X.isScrollContainer(),
	}
	vertical := oofAxis{
		cbSize:         areaHeight,
		inset:          Line[Opt]{top, bottom},
		margin:         Line[Opt]{margin.Top, margin.Bottom},
		staticPosition: sp.Y,
		alignment:      oofAlignmentOf(alignSelf),
		safety:         alignSelf.Safety,
		isScrollAxis:   containerOverflow.Y.isScrollContainer(),
	}
	stretchFit := Size[float32]{horizontal.stretchFitSize(), vertical.stretchFitSize()}

	// The space available to the border box is its stretch-fit size. A
	// table's never exceeds the containing block's minus the margins
	// (https://www.w3.org/TR/css-tables-3/#abspos).
	availableSize := stretchFit
	if isTable {
		nonAuto := rectUnwrapOr(margin, 0)
		availableSize = sizeMin(availableSize, Size[float32]{
			Width:  fmax(areaWidth-rectHSum(nonAuto), 0),
			Height: fmax(areaHeight-rectVSum(nonAuto), 0),
		})
	}
	available := spacesDefinite(sizeClamp(availableSize, minSize, maxSize))

	// Resolve sizing keywords (min-content, max-content, fit-content and
	// stretch) in the size styles. An explicitly sized axis takes precedence
	// over the automatic size below.
	if sizeStyle.Width.isSizingKeyword() || sizeStyle.Height.isSizingKeyword() {
		resolveAbsoluteSizingKeywords(node, &known, sizeStyle, areaSize, Rect[Opt]{left, right, top, bottom},
			margin, contentSize)
		known = optsClamp(optsAspectRatio(known, aspectRatio), minSize, maxSize)
	}

	// Resolve automatic sizes (https://www.w3.org/TR/css-position-3/#abspos-auto-size).
	//
	// An auto size is the stretch-fit size if the self-alignment on that
	// axis is stretch (or normal, for a non-replaced, non-table box) and
	// neither inset is auto; otherwise it is the fit-content size, measured
	// below. Stretch-fit sizes are resolved first so that the other axis can
	// follow through the aspect ratio. The block axis depends on the ratio,
	// so a stretched inline size transfers through it rather than also
	// stretching the block size; explicit stretch on both axes stretches
	// both and ignores the ratio.
	stretchWidth := !known.Width.Valid && horizontal.isStretchSized(isReplaced, isTable)
	stretchHeight := !known.Height.Valid && vertical.isStretchSized(isReplaced, isTable)
	bothExplicitStretch := horizontal.alignment == oofStretch && vertical.alignment == oofStretch
	stretchAspectRatio := aspectRatio
	if stretchWidth && stretchHeight && bothExplicitStretch {
		stretchAspectRatio = None
	}
	if stretchWidth {
		if isTable {
			// A table cannot be stretched below its content: stretch-fit is a
			// minimum.
			minSize.Width = Some(fmax(minSize.Width.Or(0), stretchFit.Width))
		} else {
			known.Width = Some(stretchFit.Width)
			known = optsClamp(optsAspectRatio(known, stretchAspectRatio), minSize, maxSize)
		}
	}
	if !known.Height.Valid && stretchHeight {
		if isTable {
			minSize.Height = Some(fmax(minSize.Height.Or(0), stretchFit.Height))
		} else {
			known.Height = Some(stretchFit.Height)
			known = optsClamp(optsAspectRatio(known, stretchAspectRatio), minSize, maxSize)
		}
	}

	var finalSize Size[float32]
	if optsBoth(known) {
		finalSize = optsUnwrapOr(known, Size[float32]{})
	} else {
		measured := measureChildSizeBoth(node, known, areaOpts, available, contentSize, Line[bool]{})
		finalSize = optsUnwrapOr(known, measured)
	}
	finalSize = sizeClamp(finalSize, minSize, maxSize)

	in := layoutInput{
		knownDimensions:            sizeSome(finalSize),
		knownDimensionsAreDefinite: Size[bool]{true, true},
		parentSize:                 areaOpts,
		availableSpace:             available,
		sizingMode:                 contentSize,
		axis:                       requestBoth,
		runMode:                    performLayout,
	}
	if beforeLayout != nil {
		// A copy of its own, so in does not move to the heap for every box.
		edited := in
		beforeLayout(&edited)
		in = edited
	}
	out := computeChildLayout(node, in)

	// Resolve auto margins (https://www.w3.org/TR/css-position-3/#abspos-margins).
	hMargin := horizontal.resolveMargins(finalSize.Width)
	vMargin := vertical.resolveMargins(finalSize.Height)
	resolvedMargin := Rect[float32]{Left: hMargin.Start, Right: hMargin.End, Top: vMargin.Start, Bottom: vMargin.End}

	// Align the margin box within the IMCB
	// (https://www.w3.org/TR/css-position-3/#abspos-alignment).
	location := Point[float32]{
		X: horizontal.position(finalSize.Width, hMargin) + areaOffset.X,
		Y: vertical.position(finalSize.Height, vMargin) + areaOffset.Y,
	}

	// Scrollbars take space on the axis opposite the scrolling one.
	var scrollbar Size[float32]
	if overflow.Y == OverflowScroll {
		scrollbar.Width = scrollbarWidth
	}
	if overflow.X == OverflowScroll {
		scrollbar.Height = scrollbarWidth
	}

	layout := Layout{
		Order:              candidate.order,
		Size:               finalSize,
		ScrollableOverflow: out.scrollableOverflow,
		ScrollbarSize:      scrollbar,
		Location:           location,
		Padding:            padding,
		Border:             border,
		Margin:             resolvedMargin,
	}
	node.LayoutState().Unrounded = layout

	return oofBoxLayout{layout: layout, surfaced: out.oofCandidates, overflow: overflow, contain: contain}
}

// performOofLayout lays out the candidates that node is the containing
// block for and collects the rest into unclaimed.
//
// candidates are node's merged, document-ordered candidates, relative to its
// border box; areaSize and areaOffset describe its inset-resolution area.
// Laying out a claimed box may surface candidates from its subtree (a fixed
// descendant of an absolute box): claimed ones join the work list and the
// rest go to unclaimed. Claimed boxes are appended to hoisted in order. It
// returns the claimed boxes' scrollable overflow.
func performOofLayout(n Node, candidates []oofCandidate, areaSize Size[float32], areaOffset Point[float32],
	direction Direction, claims containingBlockClaims, containerOverflow Point[Overflow], hoisted *[]Node,
	unclaimed *[]oofCandidate) Rect[float32] {
	var overflowRect Rect[float32]
	if len(candidates) == 0 {
		return overflowRect
	}
	isScrollContainer := containerOverflow.X.isScrollContainer() || containerOverflow.Y.isScrollContainer()

	// Split the candidates into a work list of claimed ones and the
	// unclaimed rest. Claimed candidates surfaced while laying out a claimed
	// box join the work list (as Blink re-sweeps).
	// Usually every candidate is claimed and none surfaces: the work list
	// is then candidates itself, copied only to grow.
	worklist, owned := candidates, false
	for _, c := range candidates {
		if !claims.forPosition(c.position) {
			worklist, owned = nil, true
			break
		}
	}
	if owned {
		for _, c := range candidates {
			if claims.forPosition(c.position) {
				worklist = append(worklist, c)
			} else {
				*unclaimed = append(*unclaimed, c)
			}
		}
	}

	for i := 0; i < len(worklist); i++ {
		candidate := worklist[i]
		*hoisted = append(*hoisted, candidate.node)

		box := layoutOofBox(candidate, areaSize, areaOffset, direction, containerOverflow, n, nil)
		location := box.layout.Location
		size := box.layout.Size

		// Re-sweep candidates surfaced from the box's subtree, relative to
		// this node's border box.
		if len(box.surfaced) > 0 {
			if !owned {
				worklist, owned = slices.Clone(worklist), true
			}
			translateCandidates(box.surfaced, location)
			for _, c := range box.surfaced {
				if claims.forPosition(c.position) {
					worklist = append(worklist, c)
				} else {
					*unclaimed = append(*unclaimed, c)
				}
			}
		}

		// The location is measured from the scroll origin: the inline start,
		// which is the right side in RTL.
		relative := Point[float32]{location.X - areaOffset.X, location.Y - areaOffset.Y}
		if direction.isRTL() {
			relative.X = areaSize.Width - (location.X - areaOffset.X) - size.Width
		}
		overflowRect = rectUnion(overflowRect, scrollableOverflowContribution(relative, size,
			box.layout.ScrollableOverflow, box.overflow, box.contain, isScrollContainer))
	}
	return overflowRect
}
