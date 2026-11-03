package core

import (
	"log"
	"sync"
)

// A node's Tailwind classes (NSH's core/classes.rs): resolved for the
// environment when they are set and again when what they depend on
// changes, then merged under the node's own style.

// nodeClasses are a node's classes and what they resolved to.
type nodeClasses struct {
	list              *classList
	env               classEnv // what they were resolved for
	normal, important Style
	props             classProps
	// bound is set once the node follows the environment.
	bound bool
}

// setClasses gives n the classes in class, resolved now.
func (t *Tree) setClasses(n *node, class string) {
	if class == "" {
		// A node that follows the environment keeps following it, with
		// no classes, so classes set again do not follow it twice.
		if n.cls == nil || !n.cls.bound {
			n.cls = nil
			return
		}
		class = " "
	}
	var tw *tailwindTheme
	Untracked(n.owner, func() bool { tw = n.owner.Theme().tailwind(); return true })
	l := tw.classes(class)
	if n.cls != nil && n.cls.list == l {
		return
	}
	reportUnknown(l)
	c := &nodeClasses{list: l}
	if old := n.cls; old != nil {
		c.bound = old.bound
	}
	n.cls = c
	Untracked(n.owner, func() bool { c.env = t.classEnv(n); return true })
	t.resolveClasses(n)
	if (l.env || hasEnvSpans(n)) && !c.bound {
		c.bound = true
		t.followEnv(n)
	}
	if l.positional {
		t.positional = true
	}
	if l.spacing {
		t.classKids(n)
	}
}

// bindClasses gives n class and fn's classes after them, again when what
// fn reads changes.
func (t *Tree) bindClasses(cx *Scope, n *node, class string, fn func() string) {
	if fn == nil {
		t.setClasses(n, class)
		return
	}
	first := true
	cx.renderEffect(func() {
		t.setClasses(n, joinClasses(class, fn()))
		if first {
			first = false
			return
		}
		if t.restyle(n) {
			t.markLayout(n)
		}
		t.restyleGroup(n)
	})
}

// joinClasses returns a's classes then b's.
func joinClasses(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " " + b
}

// followSpanClasses has text n follow the environment for its spans'
// classes, when they depend on it.
func (t *Tree) followSpanClasses(n *node) {
	if !hasEnvSpans(n) {
		return
	}
	if n.cls == nil {
		n.cls = &nodeClasses{list: parseClasses(" ")}
		Untracked(n.owner, func() bool { n.cls.env = t.classEnv(n); return true })
	}
	if !n.cls.bound {
		n.cls.bound = true
		t.followEnv(n)
	}
}

// followEnv resolves n's classes again whenever the environment they
// were resolved for changes: the appearance, the window, the direction,
// the motion setting or the theme. The effect is the node's owner's, not
// that of a binding that set the classes, so it lasts as long as n.
func (t *Tree) followEnv(n *node) {
	first := true
	t.within(n.owner, func() {
		n.owner.renderEffect(func() {
			env := t.classEnv(n)
			if first {
				first = false
				return
			}
			c := n.cls
			if c == nil || n.deleted {
				return
			}
			c.env = env
			if n.created {
				c.env.rtl = n.rtl
			}
			if c.list.tw != env.tw {
				c.list = env.tw.classes(c.list.class)
				reportUnknown(c.list)
			}
			t.resolveClasses(n)
			t.resolveSpanClasses(n)
			if t.restyle(n) {
				t.markLayout(n)
			}
			t.restyleGroup(n)
		})
	})
}

// classEnv reads the environment n's classes resolve for, tracked.
func (t *Tree) classEnv(n *node) classEnv {
	o := n.owner
	e := o.Env()
	pal := o.Palette()
	return classEnv{dark: o.Dark(), window: e.Window(), rtl: e.RTL(), reduce: e.ReduceMotion(), os: e.OS(),
		palette: &pal, tw: o.Theme().tailwind(), theme: o.Theme()}
}

// resolveClasses resolves n's classes for their environment and n's
// place among its siblings.
func (t *Tree) resolveClasses(n *node) {
	c := n.cls
	env := c.env
	if c.list.positional {
		env.position = n.position()
	}
	c.normal, c.important, c.props = c.list.resolve(&env)
	if g := c.list.groups; len(g) > 0 {
		// An unnamed group takes a name, as only named groups are groups
		// to Style; GroupStyle's empty Name follows any.
		c.normal.Group = g[len(g)-1]
		if c.normal.Group == "" {
			c.normal.Group = "group"
		}
	}
}

// position returns n's place among the native children of its parent.
func (n *node) position() uint8 {
	p := n.nativeParent()
	if p == nil {
		return 0
	}
	kids := p.flatChildren()
	i := -1
	for j, k := range kids {
		if k == n {
			i = j
			break
		}
	}
	if i < 0 {
		return 0
	}
	var pos uint8
	if i == 0 {
		pos |= positionFirst
	}
	if i == len(kids)-1 {
		pos |= positionLast
	}
	if len(kids) == 1 {
		pos |= positionOnly
	}
	// odd: and even: count from 1, as CSS's nth-child.
	if i%2 == 0 {
		pos |= positionOdd
	} else {
		pos |= positionEven
	}
	return pos
}

// classKids restyles p's children that follow their place among their
// siblings or the spacing p gives them, as p's children changed.
func (t *Tree) classKids(p *node) {
	spacing := p.cls != nil && p.cls.list.spacing
	for _, c := range p.flatChildren() {
		positional := c.cls != nil && c.cls.list.positional
		if !spacing && !positional {
			continue
		}
		if positional {
			t.resolveClasses(c)
		}
		if t.restyle(c) {
			t.markLayout(c)
		}
	}
}

// withClasses merges n's classes under style, and its important classes
// and the spacing its parent gives it over.
func (n *node) withClasses(style Style) Style {
	if c := n.cls; c != nil {
		style = mergeStyles(mergeStyles(c.normal, style), c.important)
		if pe := c.props.pointerEvents; pe != 0 && n.props.PointerEvents == 0 {
			n.props.PointerEvents = pe
		}
	}
	if p := n.nativeParent(); p != nil && p.cls != nil && (p.cls.props.hasSpaceX || p.cls.props.hasSpaceY) {
		sp := p.cls.props
		kids := p.flatChildren()
		if len(kids) > 0 && kids[len(kids)-1] != n {
			m := &style.Margin
			if sp.hasSpaceX {
				side := &m.Right
				if sp.reverseX {
					side = &m.Left
				}
				if *side == 0 || sp.spaceImportant {
					*side = sp.spaceX
				}
			}
			if sp.hasSpaceY {
				side := &m.Bottom
				if sp.reverseY {
					side = &m.Top
				}
				if *side == 0 || sp.spaceImportant {
					*side = sp.spaceY
				}
			}
		}
	}
	return style
}

// mergeStyles is a.Merge(b), where edges and corners merge side by side
// and the styles of the states and of the group merge too, rather than
// b's replacing a's, as CSS's properties do.
func mergeStyles(a, b Style) Style {
	out := a.Merge(b)
	out.Margin, out.Padding, out.Borders = sides(a.Margin, b.Margin), sides(a.Padding, b.Padding), sides(a.Borders, b.Borders)
	for _, c := range []struct{ dst, x, y *Color }{
		{&out.BorderColors.Top, &a.BorderColors.Top, &b.BorderColors.Top},
		{&out.BorderColors.Right, &a.BorderColors.Right, &b.BorderColors.Right},
		{&out.BorderColors.Bottom, &a.BorderColors.Bottom, &b.BorderColors.Bottom},
		{&out.BorderColors.Left, &a.BorderColors.Left, &b.BorderColors.Left},
	} {
		*c.dst = cmpOr(*c.y, *c.x)
	}
	if b.CornerRadii != (Corners{}) && a.CornerRadii != (Corners{}) {
		out.CornerRadii = Corners{TopLeft: cmpOr(b.CornerRadii.TopLeft, a.CornerRadii.TopLeft),
			TopRight:    cmpOr(b.CornerRadii.TopRight, a.CornerRadii.TopRight),
			BottomRight: cmpOr(b.CornerRadii.BottomRight, a.CornerRadii.BottomRight),
			BottomLeft:  cmpOr(b.CornerRadii.BottomLeft, a.CornerRadii.BottomLeft)}
	}
	both := func(x, y *Style) *Style {
		if x == nil || y == nil {
			return nil
		}
		m := mergeStyles(*x, *y)
		return &m
	}
	pairs := []struct{ dst, x, y **Style }{
		{&out.Hover, &a.Hover, &b.Hover}, {&out.Focus, &a.Focus, &b.Focus}, {&out.Active, &a.Active, &b.Active},
		{&out.Selected, &a.Selected, &b.Selected}, {&out.Checked, &a.Checked, &b.Checked},
		{&out.Indeterminate, &a.Indeterminate, &b.Indeterminate}, {&out.Disabled, &a.Disabled, &b.Disabled},
	}
	for _, p := range pairs {
		if m := both(*p.x, *p.y); m != nil {
			*p.dst = m
		}
	}
	if a.GroupStyle != nil && b.GroupStyle != nil && a.GroupStyle.Name == b.GroupStyle.Name {
		g := *b.GroupStyle
		ga := a.GroupStyle
		for _, p := range []struct{ dst, x **Style }{{&g.Hover, &ga.Hover}, {&g.Focus, &ga.Focus},
			{&g.Active, &ga.Active}, {&g.Selected, &ga.Selected}, {&g.Checked, &ga.Checked},
			{&g.Indeterminate, &ga.Indeterminate}, {&g.Disabled, &ga.Disabled}} {
			switch {
			case *p.dst == nil:
				*p.dst = *p.x
			case *p.x != nil:
				m := mergeStyles(**p.x, **p.dst)
				*p.dst = &m
			}
		}
		out.GroupStyle = &g
	}
	return out
}

// sides is b's sides over a's, side by side.
func sides(a, b Edges) Edges {
	return Edges{Top: cmpOr(b.Top, a.Top), Right: cmpOr(b.Right, a.Right), Bottom: cmpOr(b.Bottom, a.Bottom),
		Left: cmpOr(b.Left, a.Left)}
}

// classResizeMode is the fit n's classes give an image or a video
// (object-contain), 0 without one: the element's own wins.
func (n *node) classResizeMode() ResizeMode {
	if n.cls == nil {
		return 0
	}
	return n.cls.props.resizeMode
}

// textLimits returns text n's line limit and how it ellipsizes: its own,
// else its classes'.
func (n *node) textLimits() (int, EllipsizeMode) {
	tn := n.text
	lines, mode := tn.maxLines, tn.ellipsize
	if c := n.cls; c != nil {
		if lines == 0 && c.props.lines > 0 {
			lines = c.props.lines
		}
		if mode == 0 {
			mode = c.props.ellipsize
		}
	}
	return lines, mode
}

// spanClasses are the classes of a text's spans.
type spanClasses struct {
	list *classList
	base Style // the span's own style
}

// spanStyle returns a span's style: its classes' under its own, resolved
// for text n's environment.
func (t *Tree) spanStyle(n *node, class string, style Style) (Style, *spanClasses) {
	if class == "" {
		return style, nil
	}
	var env classEnv
	Untracked(n.owner, func() bool { env = t.classEnv(n); return true })
	l := env.tw.classes(class)
	reportUnknown(l)
	sc := &spanClasses{list: l, base: style}
	return sc.resolve(&env), sc
}

func (sc *spanClasses) resolve(env *classEnv) Style {
	normal, important, _ := sc.list.resolve(env)
	return mergeStyles(mergeStyles(normal, sc.base), important)
}

// hasEnvSpans reports whether text n has spans whose classes follow the
// environment.
func hasEnvSpans(n *node) bool {
	if n.text == nil {
		return false
	}
	for _, sp := range n.text.spans {
		if sp.cls != nil && sp.cls.list.env {
			return true
		}
	}
	return false
}

// resolveSpanClasses resolves the classes of text n's spans again.
func (t *Tree) resolveSpanClasses(n *node) {
	if n.text == nil || n.cls == nil {
		return
	}
	env := n.cls.env
	changed := false
	for i := range n.text.spans {
		sp := &n.text.spans[i]
		if sp.cls != nil && sp.cls.list.tw != env.tw {
			sp.cls.list = env.tw.classes(sp.cls.list.class)
			reportUnknown(sp.cls.list)
			sp.style = sp.cls.resolve(&env)
			changed = true
		} else if sp.cls != nil && sp.cls.list.env {
			sp.style = sp.cls.resolve(&env)
			changed = true
		}
	}
	if changed {
		n.text.stale = true
		t.markResolve(n)
	}
}

var reported sync.Map // class → true

// reportUnknown logs each class that is not a known utility, once.
func reportUnknown(l *classList) {
	for _, u := range l.unknown {
		if _, seen := reported.LoadOrStore(u, true); !seen {
			log.Printf("core: unknown class %q", u)
		}
	}
}
