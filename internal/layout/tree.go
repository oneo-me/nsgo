package layout

// Node is a box layout reads and writes. The caller implements it over its
// own tree; layout never keeps a Node beyond the call that received it
// except in the State of its containing block (see [State.Hoisted]).
type Node interface {
	// Style returns the node's style. Layout only reads it.
	Style() *Style
	// ChildCount returns the number of children, out-of-flow ones included.
	ChildCount() int
	// Child returns the child at index i.
	Child(i int) Node
	// Measure returns the size of a leaf's content box, given the
	// dimensions already known and the space available for the rest. Layout
	// calls it only for nodes without children; a node with no content
	// returns a zero size, or the known dimensions.
	Measure(known Size[Opt], available Size[AvailableSpace]) Size[float32]
	// LayoutState returns the storage layout keeps for the node. It must
	// return the same State every time.
	LayoutState() *State
}

// State is what layout keeps for a node: its results, its cache and the
// out-of-flow boxes it positions. The zero State is ready to use.
type State struct {
	// Unrounded is the layout Compute last stored.
	Unrounded Layout
	// Rounded is Unrounded snapped to whole pixels by Round.
	Rounded Layout

	cache   cache
	hoisted []Node
	// spare is the buffer of the out-of-flow candidates its last layout
	// collected, which the next one collects into: a table's absolutely
	// placed cells allocated a list per row each layout.
	spare []oofCandidate
	// grid is the detailed grid information of a grid container, which
	// positions its out-of-flow children against grid areas.
	grid *gridInfo

	// lastIn and lastOut are what the parent's last full layout gave the
	// node and what it gave back, while hasLast: what Relayout lays it out
	// with again and compares.
	lastIn  layoutInput
	lastOut layoutOutput
	hasLast bool
	// asked are a leaf's answers to the inputs layout asked it since its
	// cache was last cleared, each once: what every layout above it was
	// computed from (MeasuresAsBefore). askedOver is set once there were
	// more than maxAsked.
	asked     []answer
	askedOver bool

	// performed is the pass (passes) that last laid the node's children
	// out, writing their Unrounded layouts. Round snaps the children again
	// only when that pass is not the one their Rounded layouts came of
	// (kidsPerformed), when the node's own position moved off its pixels
	// (kidsOrigin), or when the pixels changed (kidsRatio): a tree laid
	// out from its caches but for one path rounds that path.
	performed     uint64
	kidsPerformed uint64
	kidsOrigin    Point[float32]
	kidsRatio     float64
	kidsRounded   bool
	// rounds counts the times Round snapped the children.
	rounds uint64
}

// Rounds is how many times Round has snapped the children: what walks
// the rounded tree need not go under a node whose count it saw, but for
// what is rounded through another containing block.
func (s *State) Rounds() uint64 { return s.rounds }

// answer is what a leaf gave back for an input.
type answer struct {
	in  layoutInput
	out layoutOutput
}

// maxAsked is how many answers a leaf keeps: layout asks a few, its sizes
// in measuring passes and its final one; more come of layouts at many
// sizes (a window resized), and are not worth measuring again.
const maxAsked = 8

// Invalidate drops the node's cached results, as it must be when the node's
// style, children or content change; its ancestors must be invalidated too.
// It reports whether there was anything to drop, so that a caller walking up
// the tree may stop at an ancestor already invalidated.
func (s *State) Invalidate() bool {
	s.asked, s.askedOver = s.asked[:0], false
	return s.cache.clear()
}

// ask records a leaf's answer to in.
func (s *State) ask(in *layoutInput, out *layoutOutput) {
	if s.askedOver {
		return
	}
	for i := range s.asked {
		if s.asked[i].in == *in {
			return
		}
	}
	if len(s.asked) == maxAsked {
		s.asked, s.askedOver = s.asked[:0], true
		return
	}
	s.asked = append(s.asked, answer{*in, *out})
}

// Hoisted returns the out-of-flow (absolutely or fixed positioned) boxes for
// which the node was the containing block in the last layout. Their layouts
// are relative to this node rather than to their parent.
func (s *State) Hoisted() []Node { return s.hoisted }

// Layout is a node's computed layout.
type Layout struct {
	// Order is the node's paint order among its siblings.
	Order uint32
	// Location is the position of the border box's top-left corner,
	// relative to the parent's border box, or for an out-of-flow box to its
	// containing block's.
	Location Point[float32]
	// Size is the border box size.
	Size Size[float32]
	// ScrollableOverflow is the extent of the content's scrollable overflow,
	// as edge coordinates relative to the padding box's start.
	ScrollableOverflow Rect[float32]
	// ScrollbarSize is the space taken by scrollbars: the width of a
	// vertical one and the height of a horizontal one.
	ScrollbarSize Size[float32]
	Border        Rect[float32]
	Padding       Rect[float32]
	Margin        Rect[float32]
}

// ContentBoxSize is the size inside the padding and border.
func (l *Layout) ContentBoxSize() Size[float32] {
	return Size[float32]{
		Width:  l.Size.Width - l.Padding.Left - l.Padding.Right - l.Border.Left - l.Border.Right,
		Height: l.Size.Height - l.Padding.Top - l.Padding.Bottom - l.Border.Top - l.Border.Bottom,
	}
}

// ScrollWidth is how far the content can scroll horizontally.
func (l *Layout) ScrollWidth() float32 {
	return fmax(0, l.ScrollableOverflow.Right+fmin(l.ScrollbarSize.Width, l.Size.Width)-l.Size.Width+
		l.Border.Left+l.Border.Right)
}

// ScrollHeight is how far the content can scroll vertically.
func (l *Layout) ScrollHeight() float32 {
	return fmax(0, l.ScrollableOverflow.Bottom+fmin(l.ScrollbarSize.Height, l.Size.Height)-l.Size.Height+
		l.Border.Top+l.Border.Bottom)
}

// runMode says what a layout call computes.
type runMode uint8

const (
	// performLayout computes the node's size and lays out its subtree.
	performLayout runMode = iota
	// computeSize computes only the node's size.
	computeSize
	// performHiddenLayout zeroes the layouts of a display: none subtree.
	performHiddenLayout
)

// sizingMode says whether a node's own size styles apply.
type sizingMode uint8

const (
	// contentSize ignores the node's size styles: it is sized by content.
	contentSize sizingMode = iota
	// inherentSize applies the node's size styles.
	inherentSize
)

// requestedAxis is the axis whose size a computeSize call needs.
type requestedAxis uint8

const (
	requestHorizontal requestedAxis = iota
	requestVertical
	requestBoth
)

func requestAxis(axis AbsoluteAxis) requestedAxis {
	if axis == Horizontal {
		return requestHorizontal
	}
	return requestVertical
}

// layoutInput is Taffy's LayoutInput: what a parent asks of a child.
type layoutInput struct {
	runMode    runMode
	sizingMode sizingMode
	axis       requestedAxis

	knownDimensions            Size[Opt]
	knownDimensionsAreDefinite Size[bool]
	parentSize                 Size[Opt]
	availableSpace             Size[AvailableSpace]
	// verticalMarginsAreCollapsible says whether the node's top and bottom
	// margins may collapse with its children's (block layout).
	verticalMarginsAreCollapsible Line[bool]
}

var hiddenInput = layoutInput{
	runMode:                    performHiddenLayout,
	knownDimensionsAreDefinite: Size[bool]{true, true},
	availableSpace:             maxContentSpaces,
	sizingMode:                 inherentSize,
	axis:                       requestBoth,
}

// collapsibleMarginSet tracks the largest positive and the most negative of
// a set of adjoining margins.
type collapsibleMarginSet struct {
	positive, negative float32
}

func marginSetFrom(margin float32) collapsibleMarginSet {
	if margin >= 0 {
		return collapsibleMarginSet{positive: margin}
	}
	return collapsibleMarginSet{negative: margin}
}

func (m collapsibleMarginSet) collapseWithMargin(margin float32) collapsibleMarginSet {
	if margin >= 0 {
		m.positive = fmax(m.positive, margin)
	} else {
		m.negative = fmin(m.negative, margin)
	}
	return m
}

func (m collapsibleMarginSet) collapseWithSet(o collapsibleMarginSet) collapsibleMarginSet {
	m.positive = fmax(m.positive, o.positive)
	m.negative = fmin(m.negative, o.negative)
	return m
}

func (m collapsibleMarginSet) resolve() float32 { return m.positive + m.negative }

// baselines are a box's first and last baselines.
type baselines struct {
	first, last Opt
}

// layoutOutput is Taffy's LayoutOutput: what a child reports to its parent.
type layoutOutput struct {
	size                      Size[float32]
	scrollableOverflow        Rect[float32]
	baselines                 baselines
	topMargin                 collapsibleMarginSet
	bottomMargin              collapsibleMarginSet
	marginsCanCollapseThrough bool
	// oofCandidates are out-of-flow boxes in the subtree whose containing
	// block is further up, in document order.
	oofCandidates []oofCandidate
	// oofArea is set by container algorithms in performLayout runs, when
	// hasOofArea is: the area against which the node positions the
	// out-of-flow boxes it contains.
	oofArea    oofPositioningArea
	hasOofArea bool
}

func outputFromSizes(size Size[float32], overflow Rect[float32], b baselines) layoutOutput {
	return layoutOutput{size: size, scrollableOverflow: overflow, baselines: b}
}

func outputFromOuterSize(size Size[float32]) layoutOutput { return layoutOutput{size: size} }

// clone copies o so that its candidate list is not shared.
func (o layoutOutput) clone() layoutOutput {
	if len(o.oofCandidates) > 0 {
		o.oofCandidates = append([]oofCandidate(nil), o.oofCandidates...)
	}
	return o
}

// oofPositioningArea is a containing block's inset-resolution area: its
// size, and its offset from the border box.
type oofPositioningArea struct {
	size   Size[float32]
	offset Point[float32]
}

// axisStaticEdge is an edge (or the center) an out-of-flow box aligns to.
type axisStaticEdge uint8

const (
	edgeStart axisStaticEdge = iota
	edgeCenter
	edgeEnd
)

// axisStaticAlign is how an out-of-flow box aligns within its static
// position area: by keyword, falling back when safe alignment overflows.
type axisStaticAlign struct {
	keyword  axisStaticEdge
	safety   AlignmentSafety
	fallback axisStaticEdge
}

func staticAlignOf(edge axisStaticEdge) axisStaticAlign {
	return axisStaticAlign{keyword: edge, fallback: edge}
}

// axisStaticPosition is an out-of-flow box's static position on one axis:
// the area it would occupy in flow, and how it aligns there.
type axisStaticPosition struct {
	area  Line[float32]
	align axisStaticAlign
}

func staticPositionFromEdge(anchor float32, edge axisStaticEdge) axisStaticPosition {
	return axisStaticPosition{area: Line[float32]{anchor, anchor}, align: staticAlignOf(edge)}
}

// staticPositionFromAlignment records an alignment whose self-start and
// self-end are already resolved.
func staticPositionFromAlignment(alignment Align, area Line[float32], axisIsRTL bool) axisStaticPosition {
	edgeFor := func(k AlignKeyword) axisStaticEdge {
		startPosition := !(k == AlignEnd || k == AlignFlexEnd) != axisIsRTL
		switch {
		case k == AlignCenter:
			return edgeCenter
		case startPosition:
			return edgeStart
		}
		return edgeEnd
	}
	fallback := alignment.Keyword
	if alignment.Safety == Safe {
		fallback = AlignStart
	}
	return axisStaticPosition{
		area: area,
		align: axisStaticAlign{
			keyword:  edgeFor(alignment.Keyword),
			safety:   alignment.Safety,
			fallback: edgeFor(fallback),
		},
	}
}

// oofCandidate is an out-of-flow box waiting for its containing block. Its
// static position is relative to the border box of the node holding it.
type oofCandidate struct {
	node           Node
	order          uint32
	position       Position
	staticPosition Point[axisStaticPosition]
}

// translateCandidates moves candidates' static positions by offset.
func translateCandidates(cs []oofCandidate, offset Point[float32]) {
	for i := range cs {
		sp := &cs[i].staticPosition
		sp.X.area.Start += offset.X
		sp.X.area.End += offset.X
		sp.Y.area.Start += offset.Y
		sp.Y.area.End += offset.Y
	}
}
