package core

import "slices"

// Views whose frame follows another view's (View.Follow), as NSH's
// core/follow.rs: along its parent's main axis a follower spans the
// frame of the view it follows, inside its own margins on that axis, and
// across it its own layout stands. The core places it after each layout,
// so a change of target is one frame item, and a layout that moves the
// target moves the follower in the same batch. No app code runs for it.

// following is what a follower holds.
type following struct {
	target     *Ref
	transition *LayoutTransition
	// laid is the frame layout gave it, and shown whether it gave one.
	laid  Bounds
	shown bool
	// at is the node it was last placed on, and moved the transition of
	// the frame the next batch sends.
	at    *node
	moved *LayoutTransition
}

// bindFollow makes n a follower of what v names.
func (t *Tree) bindFollow(cx *Scope, n *node, v View) {
	n.follow = &following{transition: v.FollowTransition}
	t.followers = append(t.followers, n)
	cx.renderEffect(func() {
		n.follow.target = v.Follow()
		t.followMoved = true
		t.changed = true
	})
}

// laidFollower keeps the frame layout gave follower c, which placeFollowers
// starts from, and returns the frame it stays at until then.
func (t *Tree) laidFollower(c *node, frame Bounds) Bounds {
	f := c.follow
	if f.laid != frame || f.shown == c.hidden {
		t.followMoved = true
	}
	f.laid, f.shown = frame, !c.hidden
	if c.hidden {
		// Shown again, it is placed at once rather than moved from here.
		f.at = nil
		return frame
	}
	return c.frame
}

// placeFollowers places the followers after a layout or when what they
// follow changed.
func (t *Tree) placeFollowers(relayout bool) {
	if len(t.followers) == 0 || !relayout && !t.followMoved {
		return
	}
	t.followMoved = false
	t.followers = slices.DeleteFunc(t.followers, func(n *node) bool { return n.deleted })
	for _, n := range t.followers {
		t.placeFollower(n)
	}
}

func (t *Tree) placeFollower(n *node) {
	f := n.follow
	if !f.shown {
		return
	}
	frame := f.laid
	parent := n.nativeParent()
	var at *node
	if target := f.target; target != nil && target.n != nil && !target.n.deleted && target.n != n {
		if span, ok := frameIn(target.n, parent); ok {
			at = target.n
			margin := n.style.Margin
			if row := parent.style.Direction == Row || parent.style.Direction == RowReverse; row {
				start, end := margin.Left, margin.Right
				if n.rtl {
					start, end = end, start
				}
				frame.Origin.X = span.Origin.X + start
				frame.Size.Width = max(span.Size.Width-start-end, 0)
			} else {
				frame.Origin.Y = span.Origin.Y + margin.Top
				frame.Size.Height = max(span.Size.Height-margin.Top-margin.Bottom, 0)
			}
		}
	}
	before := f.at
	f.at = at
	// To another element the move runs natively, its frame alone.
	if frame != n.frame && before != nil && at != nil && before != at {
		f.moved = f.transition
	}
	t.setFrame(n, frame)
}

// frameIn is n's frame in ancestor's coordinates: the frames of the views
// between them summed, none when ancestor is not around n or a root or a
// list on the way places what it holds on its own.
func frameIn(n, ancestor *node) (Bounds, bool) {
	if ancestor == nil {
		return Bounds{}, false
	}
	frame := n.frame
	for p := n.nativeParent(); p != ancestor; p = p.nativeParent() {
		if p == nil || p.isRoot || p.list != nil {
			return Bounds{}, false
		}
		frame.Origin.X += p.frame.Origin.X
		frame.Origin.Y += p.frame.Origin.Y
	}
	return frame, true
}
