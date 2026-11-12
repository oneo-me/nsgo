package core

// EnvInset is a value of the environment a padding or margin adds to its
// length, as CSS's env() in NSH's styles: the safe area's inset on the
// edge, or the software keyboard's height. The node follows it as it
// changes, the keyboard's moves animating with the keyboard natively.
type EnvInset uint8

const (
	EnvNone EnvInset = iota
	// EnvSafeArea is the inset on the edge of the root view the node
	// renders in: the surface's safe area, a dialog's, or a page's below
	// its header.
	EnvSafeArea
	// EnvKeyboard is the software keyboard's height over the surface, 0
	// while it is hidden or on a hardware keyboard. As a view's bottom
	// padding or margin it keeps the content above the keyboard.
	EnvKeyboard
)

// EnvEdges are the environment's terms on each edge of a padding or a
// margin. Right to left, a term follows its edge's swap and takes the
// inset of the edge it lands on.
type EnvEdges struct{ Top, Right, Bottom, Left EnvInset }

// EnvAll returns e on every edge.
func EnvAll(e EnvInset) EnvEdges { return EnvEdges{e, e, e, e} }

func (e EnvEdges) has(v EnvInset) bool {
	return e.Top == v || e.Right == v || e.Bottom == v || e.Left == v
}

// envValues are what a node's terms come to.
type envValues struct {
	insets   Edges
	keyboard Points // -1 before the first read
}

// envUses reports which of the environment's values s and its state
// styles add to a padding or margin.
func (s *Style) envUses() (safe, keyboard bool) {
	check := func(s *Style) {
		if s != nil {
			safe = safe || s.PaddingEnv.has(EnvSafeArea) || s.MarginEnv.has(EnvSafeArea)
			keyboard = keyboard || s.PaddingEnv.has(EnvKeyboard) || s.MarginEnv.has(EnvKeyboard)
		}
	}
	states := func(h, f, fv, a, sel, c, i, d *Style) {
		for _, st := range [...]*Style{h, f, fv, a, sel, c, i, d} {
			check(st)
		}
	}
	check(s)
	states(s.Hover, s.Focus, s.FocusVisible, s.Active, s.Selected, s.Checked, s.Indeterminate, s.Disabled)
	if g := s.GroupStyle; g != nil {
		states(g.Hover, g.Focus, g.FocusVisible, g.Active, g.Selected, g.Checked, g.Indeterminate, g.Disabled)
	}
	return
}

// readEnv reads, tracked, the values n's styles add, and animates a move
// of the keyboard with it.
func (t *Tree) readEnv(cx *Scope, n *node, styles ...*Style) {
	var safe, keyboard bool
	for _, s := range styles {
		if s != nil {
			sa, kb := s.envUses()
			safe, keyboard = safe || sa, keyboard || kb
		}
	}
	if !safe && !keyboard {
		n.env = nil
		return
	}
	if n.env == nil {
		n.env = &envValues{keyboard: -1}
	}
	if safe {
		n.env.insets = Insets(cx)
	}
	if keyboard {
		k := cx.Env().KeyboardHeight()
		if was := n.env.keyboard; was >= 0 && was != k {
			d := Untracked(cx, cx.Env().KeyboardDuration)
			cx.AnimateLayout(LayoutAnimation{Update: &LayoutTransition{Curve: LayoutKeyboard, Duration: d}})
		}
		n.env.keyboard = k
	}
}

// applyEnv adds what n's terms come to to its padding and margin. A
// padding with a term is not negative.
func (n *node) applyEnv() {
	s := &n.style
	if n.env == nil || s.PaddingEnv == (EnvEdges{}) && s.MarginEnv == (EnvEdges{}) {
		return
	}
	in := n.env.insets
	value := func(e EnvInset, own Points) Points {
		switch e {
		case EnvSafeArea:
			return own
		case EnvKeyboard:
			return max(n.env.keyboard, 0)
		}
		return 0
	}
	// Right to left, layout swaps left and right (layoutStyle), so a term
	// on the left takes the right edge's inset, unless the side is
	// physical.
	sides := func(keep Sides) (left, right Points) {
		left, right = in.Left, in.Right
		if n.rtl {
			if keep&SideLeft == 0 {
				left = in.Right
			}
			if keep&SideRight == 0 {
				right = in.Left
			}
		}
		return left, right
	}
	marginLeft, marginRight := sides(s.PhysicalEdges.Margin)
	padLeft, padRight := sides(s.PhysicalEdges.Padding)
	if n.scroll != nil && n.scroll.config.Horizontal {
		padLeft, padRight = in.Left, in.Right
	}
	pad := func(p *Points, e EnvInset, own Points) {
		if e != EnvNone {
			*p = max(*p+value(e, own), 0)
		}
	}
	pad(&s.Padding.Top, s.PaddingEnv.Top, in.Top)
	pad(&s.Padding.Right, s.PaddingEnv.Right, padRight)
	pad(&s.Padding.Bottom, s.PaddingEnv.Bottom, in.Bottom)
	pad(&s.Padding.Left, s.PaddingEnv.Left, padLeft)
	s.Margin.Top += value(s.MarginEnv.Top, in.Top)
	s.Margin.Right += value(s.MarginEnv.Right, marginRight)
	s.Margin.Bottom += value(s.MarginEnv.Bottom, in.Bottom)
	s.Margin.Left += value(s.MarginEnv.Left, marginLeft)
}
