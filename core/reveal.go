package core

// ScrollAlignment is where Reveal puts a view along a scroll view's axis.
type ScrollAlignment uint8

const (
	// ScrollAlignDefault is the axis's own: ScrollAlignStart vertically,
	// ScrollAlignNearest horizontally.
	ScrollAlignDefault ScrollAlignment = iota
	ScrollAlignStart
	ScrollAlignCenter
	ScrollAlignEnd
	// ScrollAlignNearest scrolls the least that shows it, and not at all
	// while it shows whole.
	ScrollAlignNearest
)

// RevealOptions say where Reveal puts a view along vertical and
// horizontal scroll views, and whether the scrolls animate.
type RevealOptions struct {
	Vertical, Horizontal ScrollAlignment
	Animated             bool
}

// reveal is a Reveal waiting for its commit's layout.
type reveal struct {
	n    *node
	opts RevealOptions
}

// Reveal scrolls every scroll view around the element until it shows,
// innermost first, each clamped to its content: at the commit, with that
// commit's layout, so a view made in the same turn is revealed where it
// lands. The word is NSH's (`reveal`), as GPUI Kit's `reveal_item`; the
// alignments are physical, as NSH's.
func (r *Ref) Reveal(opts RevealOptions) {
	if r == nil || r.n == nil || r.n.deleted {
		return
	}
	r.t.reveals = append(r.t.reveals, reveal{r.n, opts})
	r.t.changed = true
}

// revealViews turns the reveals asked for into scrolls of the scroll
// views around each view, from this commit's layout.
func (t *Tree) revealViews() {
	for _, rv := range t.reveals {
		if !rv.n.deleted {
			t.revealNode(rv.n, rv.opts)
		}
	}
	clear(t.reveals)
	t.reveals = t.reveals[:0]
}

// revealNode scrolls the scroll views around n so it shows: the box it
// takes, in each one's content, goes where opts say, and on to the next
// one out as that scroll leaves it.
func (t *Tree) revealNode(n *node, opts RevealOptions) {
	box := Bounds{Size: n.frame.Size}
	for c := n; c.parent != nil; c = c.parent {
		box.Origin.X += c.frame.Origin.X
		box.Origin.Y += c.frame.Origin.Y
		p := c.parent
		sc := p.scroll
		if sc == nil || p.deleted {
			continue
		}
		// box is in p's content: scroll p so it shows, then follow it
		// into p's own box.
		to := sc.offset
		in := sc.config.Behavior.ContentInset
		size, content := p.frame.Size, sc.content
		if sc.config.Horizontal || sc.config.Zooms() {
			to.X = revealAt(box.Origin.X, box.Size.Width, sc.offset.X, size.Width, content.Width, in.Left, in.Right,
				orDefault(opts.Horizontal, ScrollAlignNearest))
		}
		if !sc.config.Horizontal || sc.config.Zooms() {
			to.Y = revealAt(box.Origin.Y, box.Size.Height, sc.offset.Y, size.Height, content.Height, in.Top, in.Bottom,
				orDefault(opts.Vertical, ScrollAlignStart))
		}
		if to != sc.offset {
			t.scrollTo(p, to, opts.Animated)
		}
		box.Origin.X -= to.X
		box.Origin.Y -= to.Y
	}
}

func orDefault(a, or ScrollAlignment) ScrollAlignment {
	if a == ScrollAlignDefault {
		return or
	}
	return a
}

// revealAt is the offset along one axis that puts the span at start,
// length long, where align says in a viewport size long scrolled to at,
// its content length long with insets before and after.
func revealAt(start, length, at, size, content, before, after Points, align ScrollAlignment) Points {
	var to Points
	switch align {
	case ScrollAlignStart:
		to = start
	case ScrollAlignCenter:
		to = start + length/2 - size/2
	case ScrollAlignEnd:
		to = start + length - size
	default:
		switch {
		case start >= at && start+length <= at+size:
			return at
		case start < at || length > size:
			to = start
		default:
			to = start + length - size
		}
	}
	return max(min(to, content-size+after), -before)
}
