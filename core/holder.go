package core

import "slices"

// A Holder keeps content built while the places that show it come and
// go, so it moves between parts of the tree with its state (its signals,
// a field's text, a scroll offset, a web view's page) instead of being
// built again. It is NSH's render keys (core/renders.rs), where the core
// moves what JS rendered for a key into the last placeholder placing it.
type Holder struct {
	t       *Tree
	scope   *Scope
	content func(cx *Scope) Node
	// root is the content's region once built; places the places
	// mounted, oldest first: the newest shows the content.
	root   *node
	places []*node
}

// NewHolder returns a holder of what content builds, which it builds
// when a place first shows it. Unlike what code usually creates, the
// content belongs to cx itself rather than to the owner running, so it
// outlives the places that show it: until cx goes or Dispose.
func NewHolder(cx *Scope, content func(cx *Scope) Node) *Holder {
	h := &Holder{t: cx.tree(), content: content, scope: &Scope{}}
	cx.adopt(h.scope)
	h.scope.addCleanup(h.discard)
	return h
}

// Place returns a node that shows the content while it is mounted and
// no place mounted after it is: the content moves to the newest place,
// and back to the one before when that goes. With no place it stays
// built, out of the tree, its overlays hidden.
func (h *Holder) Place() Node { return holderPlace{h} }

// Dispose discards the content; places show nothing after it.
func (h *Holder) Dispose() { h.scope.Dispose() }

type holderPlace struct{ h *Holder }

// Render implements Node.
func (p holderPlace) Render(*Scope) Node { return p }

// mount implements regionMounter: the region is a place.
func (p holderPlace) mount(t *Tree, region *node, _ *Scope) {
	h := p.h
	if h.scope.disposed || h.t != t {
		return
	}
	region.place = h
	h.places = append(h.places, region)
	h.show()
}

// unplace forgets place n, which goes, before its children go with it.
func (h *Holder) unplace(n *node) {
	h.places = slices.DeleteFunc(h.places, func(p *node) bool { return p == n })
	if h.root != nil && h.root.parent == n {
		h.show()
	}
}

// show puts the content in the newest place, building it the first time.
func (h *Holder) show() {
	t := h.t
	var at *node
	if k := len(h.places); k > 0 {
		at = h.places[k-1]
	}
	if h.root == nil {
		if at == nil || h.scope.disposed {
			return
		}
		h.root = t.newRegion(at, h.scope)
		h.root.held = h
		t.within(h.scope, func() { t.mount(h.scope, h.root, h.content(h.scope)) })
		return
	}
	// A place inside the content cannot hold it.
	for p := at; p != nil; p = p.parent {
		if p == h.root {
			at = nil
			break
		}
	}
	if h.root.deleted || h.root.parent == at {
		return
	}
	h.detach()
	if at == nil {
		return
	}
	h.root.parent = at
	at.children = append(at.children, h.root)
	t.childrenChanged(at)
	// It inherits from its new place, and brings its marks.
	t.carry(h.root)
	t.markResolve(h.root)
	// Its overlays and modals go into the host of the new place; out of
	// the tree they stay hidden in the old one.
	t.rehost(h.root)
}

// detach takes the content out of the place showing it.
func (h *Holder) detach() {
	r := h.root
	if p := r.parent; p != nil {
		p.children = slices.DeleteFunc(p.children, func(c *node) bool { return c == r })
		r.parent = nil
		h.t.childrenChanged(p)
	}
}

// discard deletes the content as the holder's scope goes.
func (h *Holder) discard() {
	if h.root == nil || h.root.deleted {
		return
	}
	h.detach()
	h.t.discard(h.root)
}

// rehost moves the overlays and modals under n into the host their place
// in the tree has now.
func (t *Tree) rehost(n *node) {
	for _, c := range n.children {
		if hn := c.hoist; hn != nil && hn.host != nil && c.parent != nil {
			if host := t.hostOf(c.parent); host != hn.host && !t.detached(c) {
				old := hn.host
				old.hosted = slices.DeleteFunc(old.hosted, func(x *node) bool { return x == c })
				t.reorder(old)
				t.markLayout(old)
				hn.host = host
				host.hosted = append(host.hosted, c)
				t.reorder(host)
				t.markLayout(host)
			}
		}
		t.rehost(c)
	}
}

// detached reports whether n is in content no place shows.
func (t *Tree) detached(n *node) bool {
	for ; n != nil; n = n.parent {
		if n.parent == nil {
			return n.held != nil
		}
	}
	return false
}
