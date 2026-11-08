package core

import (
	"math"
	"slices"

	"github.com/oneo-me/nsgo/internal/layout"
)

// Layout runs NSGo's port of Taffy (core/internal/layout): CSS flexbox,
// grid and block layout. Each shadow node with a native view is a layout
// node whose children are its native children, regions flattened and
// overlays and modals left out; those are layout roots of their own, sized
// against their host. Styles map onto CSS with React Native's defaults, as
// NSH maps its styles onto Taffy's.

// ceil rounds a text's size up to a device pixel of ratio pixels per
// point, as NSH's ceil_to_pixel does, so that rendering never truncates a
// glyph. A size that is a whole pixel but for float32 (56 / 2.625 * 2.625
// is 56.000004) stays that pixel; one really over it takes the next.
func ceil(p Points, ratio float32) Points {
	if ratio <= 0 {
		ratio = 1
	}
	v := float32(p)
	up := max(float32(math.Ceil(float64(v*ratio-1e-3))), 0) / ratio
	if up < v {
		up += 1 / ratio
	}
	return Points(up)
}

// borders returns the border widths layout uses.
func (s *Style) borders() Edges {
	if s.BorderStyle == BorderNone || s.BorderStyle == BorderHidden {
		return Edges{}
	}
	if s.Borders != (Edges{}) {
		return s.Borders
	}
	return All(s.BorderWidth)
}

// The layout.Node methods.

func (n *node) ChildCount() int { return len(n.flatChildren()) }

func (n *node) Child(i int) layout.Node { return n.flatChildren()[i] }

func (n *node) LayoutState() *layout.State { return &n.lstate }

func (n *node) Style() *layout.Style {
	if !n.lstyleOK {
		n.lstyle = n.layoutStyle()
		n.lstyleOK = true
	}
	return &n.lstyle
}

// Measure measures a leaf's content: its text, a field's line, an image's
// declared size. Views and canvases have none.
func (n *node) Measure(known layout.Size[layout.Opt], available layout.Size[layout.AvailableSpace]) layout.Size[float32] {
	var m Size
	switch {
	case n.text != nil:
		// available is the content box: a known width is the border box,
		// so measuring at it would ignore the text's own padding.
		// Unwrapped lines are as wide as they are, whatever the space.
		width := Auto
		switch {
		case n.text.noWrap:
		case available.Width.Kind == layout.SpaceDefinite:
			width = Points(available.Width.Value)
		case known.Width.Valid:
			width = Points(known.Width.Value)
		case available.Width.Kind == layout.SpaceMinContent:
			width = n.text.minContent()
		}
		if !width.IsAuto() {
			width = max(width, 0)
		}
		if n.text.prepared != nil {
			m = n.text.size(width)
		}
		m = Size{Width: ceil(m.Width, n.text.ratio), Height: ceil(m.Height, n.text.ratio)}
	case n.input != nil:
		m = n.input.measure(known, available)
	case n.image != nil:
		m = fitNatural(n.image.naturalSize(), known)
	case n.icon != nil:
		m = fitNatural(n.icon.naturalSize(), known)
	case n.native != nil:
		m = fitNatural(n.native.size, known)
	default:
		return layout.Size[float32]{}
	}
	return layout.Size[float32]{
		Width:  known.Width.Or(float32(m.Width)),
		Height: known.Height.Or(float32(m.Height)),
	}
}

// fitNatural fits a leaf's natural size to the sizes layout already knows.
func fitNatural(nat Size, known layout.Size[layout.Opt]) Size {
	w, h := known.Width, known.Height
	switch {
	case !w.Valid && !h.Valid:
		return nat
	// A size with one dimension 0 asks for the other alone, as a bar as
	// wide as it is placed.
	case !h.Valid && nat.Width <= 0:
		return Size{Height: nat.Height}
	case !w.Valid && nat.Height <= 0:
		return Size{Width: nat.Width}
	case !w.Valid && nat.Height > 0:
		return Size{Width: Points(h.Value) * nat.Width / nat.Height}
	case !h.Valid && nat.Width > 0:
		return Size{Height: Points(w.Value) * nat.Height / nat.Width}
	}
	return Size{}
}

// markLayout marks n and its ancestors for layout, dropping their cached
// results, up to the first relayout boundary on the way, n included,
// which the next layout lays out on its own. Regions are never laid out
// themselves, so they pass the mark on. Overlays and modals are placed
// by the root's layout, and a text sizes its attachments as it resolves,
// so the mark goes on from them.
func (t *Tree) markLayout(n *node) {
	stops := true
	for p := n; p != nil; p = p.parent {
		p.layoutDirty = true
		p.lstate.Invalidate()
		if p.hoist != nil {
			stops = false
		}
		if p.attach != nil && p.parent != nil {
			t.markResolve(p.parent)
		}
		if stops && t.boundary(p) {
			if !p.queued {
				p.queued = true
				t.boundaries = append(t.boundaries, p)
			}
			return
		}
	}
}

// boundary reports whether n is a relayout boundary (layout.Boundary):
// a view in its parent's flow whose size nothing inside can change. A
// view whose style changed is none: its own size may have.
func (t *Tree) boundary(n *node) bool {
	if n.kind == 0 || n.isRoot || !n.lstyleOK || n.deleted || n.hoist != nil || n.attach != nil || n.accessory != nil ||
		n.headerView {
		return false
	}
	p := n.nativeParent()
	return p != nil && p.lstyleOK && !p.deleted && layout.Boundary(n, &p.lstyle)
}

// relayoutBoundaries lays out again the boundaries a change inside
// marked, each with what its parent gave it last; one that gives back
// what it gave before leaves its parent's layout standing and only its
// subtree's frames change, another marks its parent, as a change of its
// own would. It reports whether it laid any out.
func (t *Tree) relayoutBoundaries() bool {
	relaid := false
	// The deepest first, popped from the end: an outer boundary laid out
	// from its caches leaves an inner one's subtree as it was, and its
	// frames would clear the inner one's mark unlaid.
	if len(t.boundaries) > 1 {
		depth := func(n *node) int {
			d := 0
			for p := n.parent; p != nil; p = p.parent {
				d++
			}
			return d
		}
		slices.SortStableFunc(t.boundaries, func(a, b *node) int { return depth(a) - depth(b) })
	}
	for len(t.boundaries) > 0 {
		b := t.boundaries[len(t.boundaries)-1]
		t.boundaries = t.boundaries[:len(t.boundaries)-1]
		b.queued = false
		if b.deleted || !b.layoutDirty {
			continue
		}
		// Moved or restyled since, it is no longer one on its own.
		origin, ok := t.layoutOrigin(b)
		if !ok || !t.boundary(b) || !layout.Relayout(b) {
			if p := b.nativeParent(); p != nil {
				t.markLayout(p)
			}
			continue
		}
		layout.RoundFrom(b, origin, t.ratio)
		t.syncFrames(b)
		relaid = true
	}
	return relaid
}

// layoutOrigin is where the box n's layout is relative to is in n's
// layout root, unrounded, as Round carries it down: the sum of its
// ancestors' locations up to the root. It reports false where an
// ancestor is out of flow in a parent that does not contain it.
func (t *Tree) layoutOrigin(n *node) (layout.Point[float32], bool) {
	var o layout.Point[float32]
	for a := n.nativeParent(); a != nil; a = a.nativeParent() {
		l := &a.lstate.Unrounded
		o.X += l.Location.X
		o.Y += l.Location.Y
		if a.isRoot {
			return o, true
		}
		if p := a.nativeParent(); a.style.Position == Absolute && (p == nil || p.style.Position == Static) {
			return o, false
		}
	}
	return o, false
}

// layoutStyle maps n's style onto a CSS style with React Native's
// defaults, as NSH does.
func (n *node) layoutStyle() layout.Style {
	s := &n.style
	var l layout.Style

	switch s.Display {
	case DisplayNone:
		l.Display = layout.DisplayNone
	case DisplayGrid:
		l.Display = layout.DisplayGrid
	case DisplayBlock:
		l.Display = layout.DisplayBlock
	}
	l.Position = layout.PositionRelative
	switch s.Position {
	case Absolute:
		l.Position = layout.PositionAbsolute
	case Static:
		l.Position = layout.PositionStatic
	case Fixed:
		// A fixed node is the layout root of its layer, placed in it
		// absolutely (fixedRoot).
		l.Position = layout.PositionAbsolute
	}
	direction := s.Direction
	if direction == 0 && s.RowByDefault {
		direction = Row
	}
	switch direction {
	case Row:
		l.FlexDirection = layout.Row
	case RowReverse:
		l.FlexDirection = layout.RowReverse
	case ColumnReverse:
		l.FlexDirection = layout.ColumnReverse
	default:
		l.FlexDirection = layout.Column
	}
	switch s.Wrap {
	case Wrap:
		l.FlexWrap = layout.Wrap
	case WrapReverse:
		l.FlexWrap = layout.WrapReverse
	}
	l.FlexGrow = s.FlexGrow
	l.FlexShrink = layout.Some(s.FlexShrink)
	l.FlexBasis = sizeLayout(s.FlexBasis)

	l.Size = layout.Size[layout.Length]{Width: sizeLayout(s.Width), Height: sizeLayout(s.Height)}
	// Yoga has no automatic minimum size: an unset minimum is 0.
	l.MinSize = layout.Size[layout.Length]{Width: lengthMinLayout(s.MinWidth), Height: lengthMinLayout(s.MinHeight)}
	l.MaxSize = layout.Size[layout.Length]{Width: sizeLayout(s.MaxWidth), Height: sizeLayout(s.MaxHeight)}
	if s.AspectRatio > 0 {
		l.AspectRatio = layout.Some(s.AspectRatio)
	}
	if s.BoxSizing == ContentBox {
		l.BoxSizing = layout.ContentBox
	}

	l.Inset = layout.Rect[layout.Length]{Left: lengthLayout(s.Left), Right: lengthLayout(s.Right),
		Top: lengthLayout(s.Top), Bottom: lengthLayout(s.Bottom)}
	if s.Position == Sticky {
		// Its insets are where it sticks, not offsets.
		l.Inset = layout.Rect[layout.Length]{}
	}
	l.Margin = edgesLayout(s.Margin, s.MarginPercent)
	for _, side := range [...]struct {
		bit Sides
		dst *layout.Length
	}{{SideTop, &l.Margin.Top}, {SideRight, &l.Margin.Right}, {SideBottom, &l.Margin.Bottom}, {SideLeft, &l.Margin.Left}} {
		if s.AutoMargin&side.bit != 0 {
			*side.dst = layout.Auto
		}
	}
	l.Padding = edgesLayout(s.Padding, s.PaddingPercent)
	if sc := n.screen; sc != nil && sc.pad > 0 {
		// The content goes below the system header.
		l.Padding.Top = layout.Px(float32(s.Padding.Top + sc.pad))
	}
	l.Border = edgesLayout(s.borders(), 0)
	rowGap, colGap := s.Gap, s.Gap
	if s.RowGap != 0 {
		rowGap = s.RowGap
	}
	if s.ColumnGap != 0 {
		colGap = s.ColumnGap
	}
	l.Gap = layout.Size[layout.Length]{Width: layout.Px(float32(colGap)), Height: layout.Px(float32(rowGap))}

	l.AlignItems = s.AlignItems.items()
	l.AlignSelf = s.AlignSelf.items()
	l.JustifyItems = s.JustifyItems.items()
	l.JustifySelf = s.JustifySelf.items()
	l.AlignContent = s.AlignContent.content()
	l.JustifyContent = s.JustifyContent.layout()
	// An unset align-content is React Native's flex-start outside grids,
	// where CSS's normal would stretch wrapped lines; grids keep normal, so
	// their auto tracks stretch.
	if l.Display != layout.DisplayGrid && l.AlignContent.Keyword == layout.ContentNormal {
		l.AlignContent.Keyword = layout.ContentFlexStart
	}

	// Hidden clips in layout as well as in paint: what a box clips away
	// must not reach an enclosing scroll view's content. Clip, not Hidden,
	// which would also make the box a scroll container with no automatic
	// minimum size.
	x, y := s.overflow()
	l.Overflow = layout.Point[layout.Overflow]{X: x.layout(), Y: y.layout()}

	if t := s.GridTemplateColumns; t != "" {
		if g, err := parseTemplate(t); err == nil {
			l.GridTemplateColumns, l.GridTemplateColumnNames = g.tracks, g.names
		}
	}
	if t := s.GridTemplateRows; t != "" {
		if g, err := parseTemplate(t); err == nil {
			l.GridTemplateRows, l.GridTemplateRowNames = g.tracks, g.names
		}
	}
	if t := s.GridAutoColumns; t != "" {
		if g, err := parseTemplate(t); err == nil {
			l.GridAutoColumns = g.auto
		}
	}
	if t := s.GridAutoRows; t != "" {
		if g, err := parseTemplate(t); err == nil {
			l.GridAutoRows = g.auto
		}
	}
	switch s.GridAutoFlow {
	case FlowColumn:
		l.GridAutoFlow = layout.FlowColumn
	case FlowRowDense:
		l.GridAutoFlow = layout.FlowRowDense
	case FlowColumnDense:
		l.GridAutoFlow = layout.FlowColumnDense
	}
	l.GridRow = layout.Line[layout.GridPlacement]{Start: s.GridRow.Start.layout(), End: s.GridRow.End.layout()}
	l.GridColumn = layout.Line[layout.GridPlacement]{Start: s.GridColumn.Start.layout(), End: s.GridColumn.End.layout()}

	// A scroll view's content may overflow along its axis; a horizontal one
	// lays its content out in a row.
	horizontalScroll := n.scroll != nil && n.scroll.config.Horizontal
	if n.scroll != nil {
		if horizontalScroll {
			l.FlexDirection = layout.Row
			l.Overflow.X = layout.OverflowScroll
		} else {
			l.Overflow.Y = layout.OverflowScroll
		}
	}

	// Right to left, left and right swap, as in React Native. A horizontal
	// scroll view lays its content out left to right with its own sides as
	// written, and its children's frames are mirrored.
	if n.rtl {
		l.Margin.Left, l.Margin.Right = mirroredOr(l.Margin.Left, l.Margin.Right, layout.Px(0), s.PhysicalEdges.Margin)
		l.Inset.Left, l.Inset.Right = mirroredOr(l.Inset.Left, l.Inset.Right, layout.Auto, s.PhysicalEdges.Inset)
		if !horizontalScroll {
			l.Direction = layout.RTL
			l.Padding.Left, l.Padding.Right = mirroredOr(l.Padding.Left, l.Padding.Right, layout.Px(0), s.PhysicalEdges.Padding)
			l.Border.Left, l.Border.Right = mirrored(l.Border.Left, l.Border.Right, n.style.PhysicalBorders.Widths)
		}
	}

	// Roots are placed by core: the root view and modals fill their host,
	// overlays size to their content.
	if n.isRoot {
		l.Position = layout.PositionRelative
		l.Inset = layout.Rect[layout.Length]{}
		if n.fill != nil {
			l.Size = layout.Size[layout.Length]{Width: layout.Px(float32(n.fill.Width)), Height: layout.Px(float32(n.fill.Height))}
			l.MinSize, l.MaxSize = layout.Size[layout.Length]{}, layout.Size[layout.Length]{}
		}
		if a := n.accessory; a != nil {
			l.Size.Width = layout.Px(float32(a.width))
			l.MinSize.Width, l.MaxSize.Width = layout.Auto, layout.Auto
		}
	}
	return l
}

func lengthLayout(l Length) layout.Length {
	if p, ok := l.Points(); ok {
		return layout.Px(float32(p))
	}
	if pct, ok := l.Percent(); ok {
		return layout.Percent(pct / 100)
	}
	return layout.Auto
}

// sizeLayout maps a size, which may be intrinsic.
func sizeLayout(l Length) layout.Length {
	switch l {
	case MinContent:
		return layout.MinContent
	case MaxContent:
		return layout.MaxContent
	case FitContent:
		return layout.FitContent
	}
	return lengthLayout(l)
}

// lengthMinLayout maps a minimum size: unset is 0, as in Yoga, and
// AutoMinimum CSS's automatic minimum.
func lengthMinLayout(l Length) layout.Length {
	if l.IsAuto() {
		return layout.Px(0)
	}
	return sizeLayout(l)
}

func edgesLayout(e Edges, percent Sides) layout.Rect[layout.Length] {
	length := func(p Points, side Sides) layout.Length {
		if percent&side != 0 {
			return layout.Percent(float32(p) / 100)
		}
		return layout.Px(float32(p))
	}
	return layout.Rect[layout.Length]{Left: length(e.Left, SideLeft), Right: length(e.Right, SideRight),
		Top: length(e.Top, SideTop), Bottom: length(e.Bottom, SideBottom)}
}

func (o Overflow) layout() layout.Overflow {
	switch o {
	case Hidden:
		return layout.OverflowClip
	case OverflowScroll:
		return layout.OverflowScroll
	}
	return layout.OverflowVisible
}

// items maps an align-items, align-self, justify-items or justify-self
// value; start and end are flex-start and flex-end.
func (a Align) items() layout.Align {
	var k layout.AlignKeyword
	switch a {
	case AlignStretch:
		k = layout.AlignStretch
	case AlignStart:
		k = layout.AlignFlexStart
	case AlignCenter:
		k = layout.AlignCenter
	case AlignEnd:
		k = layout.AlignFlexEnd
	case AlignBaseline:
		k = layout.AlignBaseline
	}
	return layout.Align{Keyword: k}
}

// content maps an align-content value.
func (a Align) content() layout.AlignContent {
	k := layout.ContentNormal
	switch a {
	case AlignStretch:
		k = layout.ContentStretch
	case AlignStart:
		k = layout.ContentFlexStart
	case AlignCenter:
		k = layout.ContentCenter
	case AlignEnd:
		k = layout.ContentFlexEnd
	case AlignSpaceBetween:
		k = layout.ContentSpaceBetween
	case AlignSpaceAround:
		k = layout.ContentSpaceAround
	case AlignSpaceEvenly:
		k = layout.ContentSpaceEvenly
	}
	return layout.AlignContent{Keyword: k}
}

func (j Justify) layout() layout.AlignContent {
	k := layout.ContentNormal
	switch j {
	case JustifyStart:
		k = layout.ContentFlexStart
	case JustifyCenter:
		k = layout.ContentCenter
	case JustifyEnd:
		k = layout.ContentFlexEnd
	case JustifySpaceBetween:
		k = layout.ContentSpaceBetween
	case JustifySpaceAround:
		k = layout.ContentSpaceAround
	case JustifySpaceEvenly:
		k = layout.ContentSpaceEvenly
	case JustifyStretch:
		k = layout.ContentStretch
	}
	return layout.AlignContent{Keyword: k}
}

// layout lays the root view out at the surface size, then the overlays
// and modals it hosts.
func (t *Tree) layout() {
	if t.relayoutBoundaries() && !t.view.layoutDirty && anchored(t.view) {
		t.placeHoisted(t.view) // anchors may have moved
	}
	if !t.view.layoutDirty {
		t.placeFollowers(false)
		t.layoutAccessories()
		t.layoutHeaderViews()
		return
	}
	t.layoutRoot(t.view, t.size, true)
	t.setFrame(t.view, Bounds{Size: t.size})
	t.syncFrames(t.view)
	t.placeFollowers(true)
	t.placeHoisted(t.view)
	t.layoutAccessories()
	t.layoutHeaderViews()
}

// layoutRoot lays root out in space, which it fills when fill is set.
func (t *Tree) layoutRoot(root *node, space Size, fill bool) {
	root.isRoot = true
	var want *Size
	if fill {
		want = &space
	}
	if (want == nil) != (root.fill == nil) || (want != nil && *want != *root.fill) {
		root.fill = want
		root.lstyleOK = false
		root.lstate.Invalidate()
	}
	layout.Compute(root, layout.Size[layout.AvailableSpace]{
		Width:  layout.Definite(float32(space.Width)),
		Height: layout.Definite(float32(space.Height)),
	})
	layout.Round(root, t.ratio)
}

// syncFrames sets the frames of n's children from their layouts, and
// what depends on them, through n's subtree. n's own frame is set.
func (t *Tree) syncFrames(n *node) {
	t.syncContent(n)
	n.layoutDirty = false
	n.synced = n.lstate.Rounds()
	mirror := n.rtl && n.scroll != nil && n.scroll.config.Horizontal
	for _, c := range n.flatChildren() {
		l := &c.lstate.Rounded
		f := Bounds{
			Origin: Point{X: Points(l.Location.X), Y: Points(l.Location.Y)},
			Size:   Size{Width: Points(l.Size.Width), Height: Points(l.Size.Height)},
		}
		if mirror {
			f.Origin.X = n.scroll.content.Width - f.Origin.X - f.Size.Width
		}
		if c.style.Position == Absolute {
			// Layout placed it in its containing block, past the static
			// views around it; its frame is in its parent's.
			for p := n; p.style.Position == Static && !p.isRoot && p.parent != nil; p = p.nativeParent() {
				f.Origin.X -= p.frame.Origin.X
				f.Origin.Y -= p.frame.Origin.Y
			}
		}
		// As NSH's frame walk does, a hidden subtree gets empty frames and
		// content boxes, whatever its layout kept.
		hidden := n.hidden || c.style.Display == DisplayNone
		shown := hidden != c.hidden
		if shown {
			c.hidden = hidden
			t.mark(c)
		}
		if c.hidden {
			f = Bounds{}
		}
		if c.follow != nil {
			f = t.laidFollower(c, f)
		}
		t.setFrame(c, f)
		// A positioned view is the containing block of what is absolute
		// inside it, so all of its subtree is rounded through it: when
		// layout rounded nothing under it since, its children's frames
		// relative to it stand, wherever it moved, as NSH's frame walk
		// skips a subtree moved by whole pixels. A static one's absolute
		// views are placed past it, by its ancestors' frames.
		if !shown && !c.layoutDirty && c.style.Position != Static && c.synced == c.lstate.Rounds() &&
			!t.syncContent(c) {
			continue
		}
		t.syncFrames(c)
	}
}

// syncContent sets what n's layout decides inside it: where its text goes
// and how far its content scrolls. It reports whether that changed.
func (t *Tree) syncContent(n *node) bool {
	l := &n.lstate.Rounded
	if n.hidden {
		l = &layout.Layout{}
	}
	switch {
	case n.text != nil:
		if b := contentBox(l); b != n.text.box {
			n.text.box = b
			t.mark(n)
			return true
		}
	case n.input != nil:
		if b := contentBox(l); b != n.input.config.Content {
			n.input.config.Content = b
			t.mark(n)
			return true
		}
	case n.scroll != nil:
		c := Size{Width: Points(l.Size.Width + l.ScrollWidth()), Height: Points(l.Size.Height + l.ScrollHeight())}
		if end := Points(l.ScrollableOverflow.Bottom); c != n.scroll.content || end != n.scroll.contentEnd {
			n.scroll.content, n.scroll.contentEnd = c, end
			t.mark(n)
			return true
		}
	}
	return false
}

// setFrame sets n's frame, marking n when it moved or resized, and
// reports whether it did.
func (t *Tree) setFrame(n *node, f Bounds) bool {
	if f == n.frame {
		return false
	}
	n.frame = f
	t.mark(n)
	return true
}

// contentBox is the box inside a layout's padding and border.
func contentBox(l *layout.Layout) Bounds {
	return Bounds{
		Origin: Point{X: Points(l.Border.Left + l.Padding.Left), Y: Points(l.Border.Top + l.Padding.Top)},
		Size: Size{
			Width:  Points(max(l.Size.Width-l.Border.Left-l.Border.Right-l.Padding.Left-l.Padding.Right, 0)),
			Height: Points(max(l.Size.Height-l.Border.Top-l.Border.Bottom-l.Padding.Top-l.Padding.Bottom, 0)),
		},
	}
}

// setRTL lays every node out again in the new direction.
func (t *Tree) setRTL(rtl bool) {
	t.rtl = rtl
	for _, n := range t.nodes {
		if n != nil {
			n.rtl = rtl
			if n.env != nil {
				t.restyle(n)
			}
		}
	}
	t.relayoutAll()
}

// relayoutAll drops every node's layout and lays the tree out again.
func (t *Tree) relayoutAll() {
	for _, n := range t.nodes {
		if n != nil {
			n.lstyleOK = false
			n.lstate.Invalidate()
		}
	}
	t.markLayout(t.view)
	t.markAll()
}
