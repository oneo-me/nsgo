package core

import (
	"math"
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Overlays and modals stay where they are built in the shadow tree, for
// their owner, context, inherited text style and lifetime, but their
// native views go into their host: the nearest modal around them, or the
// root view. Each is a layout root of its own at its host's size.

type hoistNode struct {
	host    *node
	overlay *Overlay // nil for a modal or a fixed view
	modal   *Modal
	// fixed is a view hoisted while its position is Fixed.
	fixed  bool
	placed OverlayPosition
	// reported is a modal's size and insets when the platform presents
	// it at a size of its own (EventModalSize); dismissed says the
	// platform closed it, as when a sheet is swiped away.
	reported  *Signal[*modalSize]
	dismissed bool
	// anchorBounds is an overlay's AnchorBounds, in the surface.
	anchorBounds *Bounds
	// covering says an overlay's CoverShown holds, and covered is where
	// its Cover view was taken: its frame in the overlay, for the overlay
	// of size kept with the view at place in its parent.
	covering bool
	covered  *covered
	// tabAfter is a dialog overlay's place in the Tab order
	// (placeTabStops).
	tabAfter NodeID
	// held is the height a HoldToRoom overlay is held to, 0 for none.
	held Points
}

// hold holds overlay n to height, 0 for none, and lays it out again in
// space if that changed.
func (t *Tree) hold(n *node, height Points, space Size) {
	if n.hoist.held == height {
		return
	}
	n.hoist.held = height
	n.lstyleOK = false
	n.lstate.Invalidate()
	t.layoutRoot(n, space, false)
}

// placeTabStops gives each dialog overlay host hosts, and those inside
// its modals, its place in the Tab order: the native view right before
// it where the app put it, other overlays passed over, so a popover's
// content follows its trigger. Menus and lists of options keep their
// place: their own keys move the focus.
func (t *Tree) placeTabStops(host *node) {
	for _, n := range host.hosted {
		h := n.hoist
		if n.deleted {
			continue
		}
		if h.modal != nil {
			t.placeTabStops(n)
			continue
		}
		var after NodeID
		if h.overlay != nil && n.props.Role == RoleDialog {
			if p := nativeBefore(n); p != nil {
				after = p.id
			}
		}
		if after != h.tabAfter {
			h.tabAfter = after
			t.mark(n)
		}
	}
}

// nativeBefore is the native view right before n in depth-first order
// where the app put it: the last one inside the siblings before it,
// regions opened and hoisted views passed over, else the nearest native
// view around it.
func nativeBefore(n *node) *node {
	for c := n; c.parent != nil; c = c.parent {
		p := c.parent
		for j := slices.Index(p.children, c) - 1; j >= 0; j-- {
			if last := lastNative(p.children[j]); last != nil {
				return last
			}
		}
		if p.kind != 0 {
			return p
		}
	}
	return nil
}

// lastNative is the last native view in n's subtree in depth-first
// order, hoisted views passed over.
func lastNative(n *node) *node {
	if n.deleted || n.hoist != nil {
		return nil
	}
	for i := len(n.children) - 1; i >= 0; i-- {
		if l := lastNative(n.children[i]); l != nil {
			return l
		}
	}
	if n.kind != 0 {
		return n
	}
	return nil
}

// covered is where an overlay's Cover view is in it.
type covered struct {
	at    Bounds
	kept  Size
	place Point
}

// anchored reports whether an overlay host hosts, or one inside its
// modals, is placed beside an anchor, which a scroll may move.
func anchored(host *node) bool {
	for _, h := range host.hosted {
		if o := h.hoist.overlay; o != nil && (o.Anchor != nil || o.AnchorBounds != nil) || h.hoist.modal != nil && anchored(h) {
			return true
		}
	}
	return false
}

type modalSize struct {
	size   Size
	insets Edges
}

// hostOf returns the root view or the nearest modal around parent.
func (t *Tree) hostOf(parent *node) *node {
	for p := parent; p != nil; p = p.parent {
		if p.kind == driver.KindModal {
			return p
		}
	}
	return t.view
}

func (t *Tree) hoist(n *node, h *hoistNode) {
	h.host = t.hostOf(n.parent)
	n.hoist = h
	h.host.hosted = append(h.host.hosted, n)
	t.reorder(h.host)
	if p := n.parent; p != nil {
		t.childrenChanged(p)
	}
	// The root's layout places it, whatever boundary holds it.
	t.markLayout(n)
}

func (t *Tree) mountOverlay(cx *Scope, parent *node, o Overlay) {
	n := t.newNode(driver.KindView, parent, cx)
	h := &hoistNode{overlay: &o}
	t.hoist(n, h)
	t.bindClasses(cx, n, o.Class, o.ClassFunc)
	t.bindStyle(cx, n, o.Style, o.StyleFunc)
	t.setRef(n, o.Ref)
	if o.AnchorBounds != nil && o.Anchor == nil {
		cx.renderEffect(func() {
			b := o.AnchorBounds()
			h.anchorBounds = &b
			t.markLayout(t.view)
			t.changed = true
		})
	}
	if o.Cover != nil && o.CoverShown != nil {
		cx.renderEffect(func() {
			h.covering = o.CoverShown()
			t.markLayout(t.view)
			t.changed = true
		})
	}
	n.h = handlers{pressOutside: o.OnPressOutside, cancel: o.OnCancel, transitionEnd: o.OnTransitionEnd,
		mouseEnter: o.OnMouseEnter, mouseLeave: o.OnMouseLeave}
	n.props = driver.ViewProps{Role: o.Role, Label: o.Label, TestID: o.TestID, PointerEvents: o.PointerEvents}
	n.ownPointer = o.PointerEvents
	for _, c := range o.Children {
		t.mount(cx, n, c)
	}
}

func (t *Tree) mountModal(cx *Scope, parent *node, m Modal) {
	n := t.newNode(driver.KindModal, parent, cx)
	t.hoist(n, &hoistNode{modal: &m, reported: NewSignal[*modalSize](cx, nil)})
	t.setClasses(n, m.Class)
	t.bindStyle(cx, n, m.Style, m.StyleFunc)
	n.props = driver.ViewProps{Role: m.Role, Label: m.Label, TestID: m.TestID}
	if n.props.Role == RoleNone {
		n.props.Role = RoleDialog
	}
	t.bindStatusBar(cx, n, m.StatusBar, m.StatusBarFunc)
	if ts := m.TransitionShader; ts != nil {
		host := n.hoist.host
		t.navShader(host, ts, false)
		cx.Cleanup(func() { t.navShader(host, ts, true) })
	}
	// What a modal's content keeps clear of is the surface's safe area, or
	// what the platform reported of a modal it sized.
	inner := cx.Child()
	insets := func() Edges {
		if r := n.hoist.reported.Get(); r != nil {
			return r.insets
		}
		return t.modalArea(n, cx.Env().SafeArea(), cx.Env().OS()).insets
	}
	// Provided while inner runs, so it is inner's own.
	t.within(inner, func() {
		insetsKey.Provide(inner, insets)
		for _, c := range m.Children {
			t.mount(inner, n, c)
		}
	})
}

// placeHoisted lays out and places the overlays and modals host holds,
// and those inside them.
func (t *Tree) placeHoisted(host *node) {
	for _, n := range host.hosted {
		if n.deleted {
			continue
		}
		size := host.frame.Size
		if n.hoist.modal != nil {
			env := t.root.Env()
			var top Points
			if r := n.hoist.reported.GetUntracked(); r != nil {
				size = r.size
			} else {
				top = t.modalArea(n, env.safe.GetUntracked(), env.os.GetUntracked()).top
				size.Height = max(size.Height-top, 0)
			}
			t.layoutRoot(n, size, true)
			t.setFrame(n, Bounds{Origin: Point{Y: top}, Size: size})
		} else if n.hoist.fixed {
			t.layoutRoot(n, size, false)
			l := &n.lstate.Rounded
			box := Size{Width: Points(l.Size.Width), Height: Points(l.Size.Height)}
			t.setFrame(n, Bounds{Origin: t.onPixel(insetOrigin(&n.style, box, size, n.rtl)), Size: box})
		} else {
			t.placeOverlay(n, host, size)
		}
		// Its native view is its host's child, so a hidden shadow ancestor
		// hides it only through the core; so does a sheet swiped away.
		if hidden := hiddenIn(n) || n.hoist.dismissed; hidden != n.hidden {
			n.hidden = hidden
			t.mark(n)
		}
		if n.hidden {
			t.setFrame(n, Bounds{})
		}
		t.syncFrames(n)
		t.placeHoisted(n)
	}
}

// modalPlace is where a modal starts in its host, and what its content
// keeps clear of.
type modalPlace struct {
	top    Points
	insets Edges
}

// modalArea places modal n, unless the platform sized it, in the safe
// area safe on os: over its whole host, except that on Android the
// status bar stays above a modal over the surface that is not
// Android.StatusBarTranslucent, as above NSH's dialog windows. A modal in a
// modal keeps clear of its host's insets.
func (t *Tree) modalArea(n *node, safe Edges, os OS) modalPlace {
	if host := n.hoist.host; host.kind == driver.KindModal {
		return modalPlace{insets: t.modalInsets(host)}
	}
	if os == OSAndroid && !n.hoist.modal.Android.StatusBarTranslucent {
		top := safe.Top
		safe.Top = 0
		return modalPlace{top, safe}
	}
	return modalPlace{insets: safe}
}

// modalInsets returns what modal n's content keeps clear of, untracked.
func (t *Tree) modalInsets(n *node) Edges {
	if r := n.hoist.reported.GetUntracked(); r != nil {
		return r.insets
	}
	env := t.root.Env()
	return t.modalArea(n, env.safe.GetUntracked(), env.os.GetUntracked()).insets
}

// modalEvent handles what the platform reports of modal n: the size it
// presents it at, or a request to close it.
func (t *Tree) modalEvent(n *node, ev driver.Event) {
	h := n.hoist
	switch f := h.modal.OnShown; ev.Kind {
	case driver.EventModalShown:
		if f != nil {
			t.within(n.owner, f)
		}
		return
	case driver.EventModalDismissed:
		if f := h.modal.OnDismissed; f != nil {
			t.within(n.owner, f)
		}
		return
	}
	if ev.Kind == driver.EventModalSize {
		r := &modalSize{ev.Size, ev.Insets}
		if old := h.reported.GetUntracked(); old == nil || *old != *r {
			t.within(n.owner, func() { h.reported.Set(r) })
			t.relayout(n)
		}
		return
	}
	if ev.Finished && !h.dismissed {
		h.dismissed = true
		t.relayout(n)
	}
	if f := h.modal.OnRequestClose; f != nil {
		t.within(n.owner, func() { f(ev.Text) })
	}
}

// hiddenIn reports whether DisplayNone hides n or a shadow ancestor, or
// n is in a holder's content that no place shows.
func hiddenIn(n *node) bool {
	for ; n != nil; n = n.parent {
		if n.style.Display == DisplayNone || n.parent == nil && n.held != nil {
			return true
		}
	}
	return false
}

// placeOverlay sizes overlay n to its content within its host and places
// it beside its anchor, or by its insets in its host without one.
func (t *Tree) placeOverlay(n *node, host *node, hostSize Size) {
	o := n.hoist.overlay
	s := n.style
	w, h := hostSize.Width, hostSize.Height
	t.layoutRoot(n, hostSize, false)
	l := &n.lstate.Rounded
	size := Size{Width: Points(l.Size.Width), Height: Points(l.Size.Height)}
	if held := n.hoist.held; held > 0 && size.Height >= held {
		// Held to its last room: its side is picked at its natural size.
		t.hold(n, 0, hostSize)
		size = Size{Width: Points(l.Size.Width), Height: Points(l.Size.Height)}
	}
	var origin Point
	anchor, ok := t.anchorRect(o, host)
	if !ok && n.hoist.anchorBounds != nil {
		anchor, ok = *n.hoist.anchorBounds, true
		if host != t.view {
			hb := t.surfaceBounds(host)
			anchor.Origin.X -= hb.Origin.X
			anchor.Origin.Y -= hb.Origin.Y
		}
	}
	if !ok {
		// Without an anchor, an overlay is placed as an absolute child of
		// its host.
		origin = t.onPixel(insetOrigin(&s, size, hostSize, n.rtl))
	} else {
		safe := t.root.Env().safe.GetUntracked()
		if host.kind == driver.KindModal {
			safe = Edges{}
		}
		area := Bounds{
			Origin: Point{X: safe.Left + o.Margin, Y: safe.Top + o.Margin},
			Size: Size{Width: max(w-safe.Horizontal()-2*o.Margin, 0),
				Height: max(h-safe.Vertical()-2*o.Margin, 0)},
		}
		rtl := t.root.Env().rtl.GetUntracked()
		var side Placement
		want := o.Placement
		if want == PlaceDefault {
			want = PlaceBottom
		}
		var room Size
		if at, ok := t.covering(n, size); ok {
			// Its start on the anchor's, the view's middle on the anchor's,
			// inside the area as far as it fits.
			keep := func(v, lo, hi, length Points) Points { return max(min(v, hi-length), lo) }
			origin = Point{X: keep(anchor.Origin.X, area.Origin.X, area.Origin.X+area.Size.Width, size.Width),
				Y: keep(anchor.Origin.Y+(anchor.Size.Height-at.Size.Height)/2-at.Origin.Y, area.Origin.Y,
					area.Origin.Y+area.Size.Height, size.Height)}
			side, room = want, area.Size
		} else {
			origin, side, room = place(anchor, size, area, want, o.Align, o.Offset, rtl)
			if o.HoldToRoom {
				var held Points
				if size.Height > room.Height {
					held = room.Height
				}
				if held != n.hoist.held {
					t.hold(n, held, hostSize)
					size = Size{Width: Points(l.Size.Width), Height: Points(l.Size.Height)}
					origin, side, room = place(anchor, size, area, side, o.Align, o.Offset, rtl)
				}
			}
		}
		origin = t.onPixel(origin)
		pos := OverlayPosition{Placement: side, Bounds: Bounds{Origin: origin, Size: size}, Available: room}
		if pos != n.hoist.placed && o.OnPosition != nil {
			f := o.OnPosition
			t.after(n, func() { f(pos) })
		}
		n.hoist.placed = pos
	}
	t.setFrame(n, Bounds{Origin: origin, Size: size})
}

// onPixel snaps a root's origin to device pixels as layout snaps every
// other frame: a pointer's position or a centered overlay's holds
// fractions, and a box off the pixel grid leaves its edges blurred, its
// border and fill apart.
func (t *Tree) onPixel(p Point) Point {
	r := float64(t.ratio)
	if r <= 0 {
		r = 1
	}
	snap := func(v Points) Points { return Points(math.Floor(float64(v)*r+0.5) / r) }
	return Point{X: snap(p.X), Y: snap(p.Y)}
}

// covering is where overlay n's Cover view is in it: taken once both are
// laid out and kept while n keeps its size and the view its place in its
// parent, so a scroll of its content moves nothing.
func (t *Tree) covering(n *node, size Size) (Bounds, bool) {
	h := n.hoist
	ref := h.overlay.Cover
	if ref == nil || ref.n == nil || ref.n.deleted || !h.covering {
		return Bounds{}, false
	}
	inner := ref.n
	if c := h.covered; c != nil && c.kept == size && c.place == inner.frame.Origin {
		return c.at, true
	}
	at := inner.frame
	for p := inner.nativeParent(); ; p = p.nativeParent() {
		if p == nil {
			return Bounds{}, false
		}
		if p.scroll != nil {
			at.Origin.X -= p.scroll.offset.X
			at.Origin.Y -= p.scroll.offset.Y
		}
		if p == n {
			break
		}
		at.Origin.X += p.frame.Origin.X
		at.Origin.Y += p.frame.Origin.Y
	}
	h.covered = &covered{at: at, kept: size, place: inner.frame.Origin}
	return at, true
}

// insetOrigin places a box of size in a host of hostSize by s's insets,
// as an absolute child of the host.
func insetOrigin(s *Style, size, hostSize Size, rtl bool) Point {
	w, h := hostSize.Width, hostSize.Height
	left, right := s.Left.Resolve(w), s.Right.Resolve(w)
	if rtl {
		left, right = right, left
	}
	tp, b := s.Top.Resolve(h), s.Bottom.Resolve(h)
	var origin Point
	switch {
	case !left.IsAuto():
		origin.X = left
	case !right.IsAuto():
		origin.X = w - right - size.Width
	}
	switch {
	case !tp.IsAuto():
		origin.Y = tp
	case !b.IsAuto():
		origin.Y = h - b - size.Height
	}
	return origin
}

// syncPosition hoists n into its host while its position is Fixed, puts
// it back in its parent when it no longer is, and keeps the sticky views
// the tree pins.
func (t *Tree) syncPosition(n *node) {
	if n.kind == 0 || n.attach != nil || n.accessory != nil || n.headerView || n.isStack {
		return
	}
	host := n.parent != nil && n.parent.scroll != nil && n.parent.scroll.barHost == n
	if sticky := n.style.Position == Sticky || host; sticky != t.sticky[n] {
		if sticky {
			if t.sticky == nil {
				t.sticky = map[*node]bool{}
			}
			t.sticky[n] = true
		} else {
			delete(t.sticky, n)
		}
	}
	fixed := n.style.Position == Fixed
	switch {
	case fixed && n.hoist == nil && n.parent != nil:
		t.hoist(n, &hoistNode{fixed: true})
		t.relayout(n)
	case !fixed && n.hoist != nil && n.hoist.fixed:
		host := n.hoist.host
		host.hosted = slices.DeleteFunc(host.hosted, func(h *node) bool { return h == n })
		t.reorder(host)
		t.markLayout(host)
		n.hoist, n.isRoot, n.fill, n.lstyleOK = nil, false, nil, false
		n.lstate.Invalidate()
		t.childrenChanged(n.parent)
	}
}

// anchorRect returns ref's bounds in host's coordinates.
func (t *Tree) anchorRect(o *Overlay, host *node) (Bounds, bool) {
	ref := o.Anchor
	if ref == nil || ref.n == nil || ref.n.deleted {
		return Bounds{}, false
	}
	b := t.surfaceBounds(ref.n)
	if o.AnchorContent {
		b = t.contentBounds(ref.n)
	}
	if host != t.view {
		hb := t.surfaceBounds(host)
		b.Origin.X -= hb.Origin.X
		b.Origin.Y -= hb.Origin.Y
	}
	return b, true
}

// contentBounds is the box around the views n holds in the surface, its
// own frame while it holds none.
func (t *Tree) contentBounds(n *node) Bounds {
	var box Bounds
	found := false
	var walk func(*node)
	walk = func(p *node) {
		for _, c := range p.children {
			switch {
			case c.hoist != nil:
			case c.kind == 0:
				walk(c)
			default:
				b := t.surfaceBounds(c)
				if !found {
					box, found = b, true
					continue
				}
				x1, y1 := max(box.Origin.X+box.Size.Width, b.Origin.X+b.Size.Width), max(box.Origin.Y+box.Size.Height, b.Origin.Y+b.Size.Height)
				box.Origin = Point{X: min(box.Origin.X, b.Origin.X), Y: min(box.Origin.Y, b.Origin.Y)}
				box.Size = Size{Width: x1 - box.Origin.X, Height: y1 - box.Origin.Y}
			}
		}
	}
	walk(n)
	if !found {
		return t.surfaceBounds(n)
	}
	return box
}

// surfaceBounds returns n's frame in surface points: its native
// ancestors' frames added, their scroll offsets taken off.
func (t *Tree) surfaceBounds(n *node) Bounds {
	b := n.frame
	for p := n.nativeParent(); p != nil; p = p.nativeParent() {
		b.Origin.X += p.frame.Origin.X
		b.Origin.Y += p.frame.Origin.Y
		if p.scroll != nil {
			b.Origin.X -= p.scroll.offset.X
			b.Origin.Y -= p.scroll.offset.Y
		}
	}
	return b
}

// place puts an overlay of size beside anchor inside area, as NSH does:
// on the side asked unless it does not fit there and the opposite side
// fits or has more room, aligned along that side, then clamped into area.
// In right-to-left layouts start and end swap for top and bottom.
func place(anchor Bounds, size Size, area Bounds, side Placement, align Align, offset Points, rtl bool) (Point, Placement, Size) {
	aLeft, aTop := anchor.Origin.X, anchor.Origin.Y
	aRight, aBottom := aLeft+anchor.Size.Width, aTop+anchor.Size.Height
	lo, hi := area.Origin, Point{X: area.Origin.X + area.Size.Width, Y: area.Origin.Y + area.Size.Height}
	roomOn := func(p Placement) Points {
		switch p {
		case PlaceTop:
			return aTop - offset - lo.Y
		case PlaceBottom:
			return hi.Y - aBottom - offset
		case PlaceLeft:
			return aLeft - offset - lo.X
		}
		return hi.X - aRight - offset
	}
	need := func(p Placement) Points {
		if p == PlaceTop || p == PlaceBottom {
			return size.Height
		}
		return size.Width
	}
	opposite := map[Placement]Placement{PlaceTop: PlaceBottom, PlaceBottom: PlaceTop, PlaceLeft: PlaceRight, PlaceRight: PlaceLeft}[side]
	if roomOn(side) < need(side) && (roomOn(opposite) >= need(opposite) || roomOn(opposite) > roomOn(side)) {
		side = opposite
	}
	along := func(a0, a1, length Points, swap bool) Points {
		start, end := a0, a1-length
		if swap {
			start, end = end, start
		}
		switch align {
		case AlignStart:
			return start
		case AlignEnd:
			return end
		}
		return a0 + (a1-a0-length)/2
	}
	var p Point
	switch side {
	case PlaceTop:
		p = Point{X: along(aLeft, aRight, size.Width, rtl), Y: aTop - offset - size.Height}
	case PlaceBottom:
		p = Point{X: along(aLeft, aRight, size.Width, rtl), Y: aBottom + offset}
	case PlaceLeft:
		p = Point{X: aLeft - offset - size.Width, Y: along(aTop, aBottom, size.Height, false)}
	case PlaceRight:
		p = Point{X: aRight + offset, Y: along(aTop, aBottom, size.Height, false)}
	}
	p.X = max(min(p.X, hi.X-size.Width), lo.X)
	p.Y = max(min(p.Y, hi.Y-size.Height), lo.Y)
	room := Size{Width: area.Size.Width, Height: max(roomOn(side), 0)}
	if side == PlaceLeft || side == PlaceRight {
		room = Size{Width: max(roomOn(side), 0), Height: area.Size.Height}
	}
	return p, side, room
}

// back routes Escape or back: the topmost overlay listening for cancel,
// else the topmost modal, else the innermost stack that can pop.
func (t *Tree) back() {
	host := t.view
	for {
		top := topModal(host)
		if top == nil {
			break
		}
		host = top
	}
	if o := topOverlay(host); o != nil {
		t.within(o.owner, o.h.cancel)
		return
	}
	if host.kind == driver.KindModal {
		if stack := t.innerStack(host); stack != nil && t.pop(stack) {
			return
		}
		if f := host.hoist.modal.OnRequestClose; f != nil {
			t.within(host.owner, func() { f("cancel") })
		}
		return
	}
	for i := len(t.backs) - 1; i >= 0; i-- {
		b := t.backs[i]
		if b.owner.disposed {
			continue
		}
		taken := false
		t.within(b.owner, func() { taken = b.fn() })
		if taken {
			return
		}
	}
	if stack := t.innerStack(host); stack != nil && t.pop(stack) {
		return
	}
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemBackUnhandled})
	t.changed = true
}

// backPreview returns the screen back pops, as back routes it, when
// nothing takes it first: no overlay listening for cancel, no modal's
// OnRequestClose, no Scope.OnBackPress handler and no NoPopOnBack.
func (t *Tree) backPreview() driver.NodeID {
	host := t.view
	for top := topModal(host); top != nil; top = topModal(host) {
		host = top
	}
	if topOverlay(host) != nil {
		return 0
	}
	if host.kind != driver.KindModal {
		for _, b := range t.backs {
			if !b.owner.disposed {
				return 0
			}
		}
	}
	stack := t.innerStack(host)
	if stack == nil {
		return 0
	}
	screens := stack.screens()
	top := screens[len(screens)-1]
	if s := top.screen.screen; s.NoPopOnBack || s.NoPopOnSwipe {
		return 0
	}
	t.peek(stack, screens[len(screens)-2])
	return top.id
}

// peek lays screen below out under stack's top one for a back gesture, or
// stops when below is nil.
func (t *Tree) peek(stack, below *node) {
	if t.peeking != nil && t.peeking != below {
		t.peeking.screen.peek = false
		if s := t.peeking.screen.stack; s != nil {
			t.showTop(s)
		}
	}
	t.peeking = below
	if below != nil {
		below.screen.peek = true
		t.showTop(stack)
	}
}

func topModal(host *node) *node {
	for i := len(host.hosted) - 1; i >= 0; i-- {
		if n := host.hosted[i]; !n.deleted && n.kind == driver.KindModal && !n.hoist.dismissed && !hiddenIn(n) {
			return n
		}
	}
	return nil
}

func topOverlay(host *node) *node {
	for i := len(host.hosted) - 1; i >= 0; i-- {
		if n := host.hosted[i]; !n.deleted && n.kind != driver.KindModal && n.h.cancel != nil && !hiddenIn(n) {
			return n
		}
	}
	return nil
}

// pressOutside reports a press outside overlay n unless it landed on its
// anchor or an overlay above it.
func (t *Tree) pressOutside(n *node, at Point) {
	if n.h.pressOutside == nil || hiddenIn(n) {
		return
	}
	// Its anchor's whole box, what a press on it reaches.
	if a, ok := t.anchorRect(&Overlay{Anchor: n.hoist.overlay.Anchor}, t.view); ok && a.Contains(at) {
		return
	}
	host := n.hoist.host
	above := false
	for _, o := range host.hosted {
		if o == n {
			above = true
			continue
		}
		if above && !o.deleted && !o.hidden && t.surfaceBounds(o).Contains(at) {
			return
		}
	}
	t.within(n.owner, func() { n.h.pressOutside(at) })
}
