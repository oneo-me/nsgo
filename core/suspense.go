package core

// Suspense shows Fallback while a resource that Content reads loads its first value, as Solid's Suspense does, and Content once none does. Content stays mounted, hidden and out of layout meanwhile, so what it reads keeps it suspended until the resources are ready, and it shows at once then. A resource loading again, with a value already (ResourceRefreshing), keeps Content shown rather than going back to Fallback, as does one read outside a computation, as in a handler. A Suspense inside Content takes what is read under it.
type Suspense struct {
	Content  func(cx *Scope) Node
	Fallback func(cx *Scope) Node
}

// Render implements Node.
func (s Suspense) Render(*Scope) Node { return s }

func (s Suspense) mountIn(t *Tree, cx *Scope, parent *node) { t.mountSuspense(cx, parent, s) }

// suspenseKey gives Content's owners their Suspense.
var suspenseKey = NewKey[*suspense]("suspense", nil)

// suspender is a resource a Suspense waits for.
type suspender interface {
	// loadingFirst reports whether it loads with no value yet, subscribing the running computation.
	loadingFirst() bool
}

// suspense is a mounted Suspense: the resources computations under its Content read now, by how many, and whether any loads its first value.
type suspense struct {
	t       *Tree
	content *node // Content's region
	read    map[suspender]int
	// changed is bumped as a resource comes to be read or stops being read, so waiting follows the set.
	changed *Signal[uint64]
	waiting *Memo[bool]
}

func (t *Tree) mountSuspense(cx *Scope, parent *node, s Suspense) {
	if t.suspended == nil {
		t.suspended = map[*node]bool{}
	}
	b := &suspense{t: t, read: map[suspender]int{}, changed: NewSignal(cx, uint64(0))}
	b.waiting = NewMemo(cx, func() bool {
		b.changed.Get()
		for r := range b.read {
			if r.loadingFirst() {
				return true
			}
		}
		return false
	})
	b.content = t.newRegion(parent, cx)
	content := cx.Child()
	suspenseKey.Provide(content, b)
	t.within(content, func() { t.mount(content, b.content, s.Content(content)) })
	cx.renderEffect(func() {
		on := b.waiting.Get()
		if on {
			t.suspended[b.content] = true
		} else {
			delete(t.suspended, b.content)
		}
		// Content's own views hide and show; what is under them follows. They are those suspendedIn finds Content above, hoisted ones too, which a host places apart from the views around them.
		for _, n := range ownViews(nil, b.content.children) {
			if t.restyle(n) {
				t.markLayout(n)
			}
		}
	})
	if s.Fallback != nil {
		t.mount(cx, parent, Show{When: b.waiting.Get, Then: s.Fallback})
	}
	cx.Cleanup(func() { delete(t.suspended, b.content) })
}

// suspend has the Suspense around the computation running now wait for r, which it read, for as long as that run is the computation's last: a run again or the computation's end lets it go. owner is any scope of r's graph.
func suspend(r suspender, owner *Scope) {
	reader := owner.rt.observer
	if reader == nil {
		return
	}
	b := suspenseKey.Use(owner)
	if b == nil {
		return
	}
	if b.read[r]++; b.read[r] == 1 {
		b.bump()
	}
	reader.addCleanup(func() {
		if b.read[r]--; b.read[r] <= 0 {
			delete(b.read, r)
			b.bump()
		}
	})
}

func (b *suspense) bump() { b.changed.Update(func(n uint64) uint64 { return n + 1 }) }

// ownViews appends the views children holds, through its regions, including the overlays, modals and fixed views it hoists, which appendNative leaves out.
func ownViews(out []*node, children []*node) []*node {
	for _, c := range children {
		if c.kind == 0 {
			out = ownViews(out, c.children)
		} else {
			out = append(out, c)
		}
	}
	return out
}

// suspendedIn reports whether n is one of a waiting Suspense's Content's own views: the regions between it and the view it sits in hold one.
func (t *Tree) suspendedIn(n *node) bool {
	for p := n.parent; p != nil && p.kind == 0; p = p.parent {
		if t.suspended[p] {
			return true
		}
	}
	return false
}
