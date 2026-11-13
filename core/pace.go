package core

import (
	"math"
	"slices"
	"time"
)

// Views made over frames, as NSH's (performance.md): a list row bound in
// the overscan, or a long reply staged into a chat, would make its whole
// subtree in one batch, hundreds of native views nobody sees yet, which
// the platform creates and commits in one long frame. Lists pace their
// binds (list.go); this paces the views inside them, and the content of
// scroll views.
//
// Once a commit has made paceAfter views, a view about to be made in a
// scroll view or a list waits: it keeps its layout and frame, and its
// parent's native children leave it out. After the commit's views are
// sent, syncPaced makes the waiting views that show (their frame meets
// the viewport of every scroll view around them) at once, a level at a
// time down to their leaves, so nothing in sight is late; the rest come
// in later commits, nearest the viewport first, at most paceViews views
// or paceViewsTime a commit, and the tree asks for a display frame while
// some wait (paceLater). A command naming a waiting view makes it and the
// views around it at once.
//
// Nothing waits that may show away from its frame or lays out on its own
// (sticky views, followers, overlays and modals, transformed or animated
// views), nor what reads its frame (OnLayout) or takes the focus as it
// shows, nor anything outside scroll views and lists, whose content
// shows wherever its frame is.
const (
	// paceAfter is the views a commit makes before views out of sight
	// start to wait.
	paceAfter = 64
	// paceViews is the waiting views out of sight a commit makes at most.
	// Most of a view's cost is the platform's, about 0.2 ms a view on
	// macOS, so 32 keep a frame near 7 ms.
	paceViews = 32
	// paceViewsTime is the time a commit spends making waiting views out
	// of sight.
	paceViewsTime = 4 * time.Millisecond
)

// What a node's pace is.
const (
	paceWaiting uint8 = iota + 1 // in Tree.waiting, its view not made
	paceNow                      // made at the next commit, whatever it costs
)

// viewPace is what a tree keeps for pacing.
type viewPace struct {
	off bool // SetViewPacing(false)
	// made counts the views this commit made, paced the waiting views out
	// of sight, from started.
	made, paced int
	started     time.Time
	// all holds every view about to be made in a scroll view, while
	// syncPaced makes a level of waiting ones: their children are ranked
	// on their own.
	all bool
}

// SetViewPacing turns pacing views over frames on, as it starts, or off,
// as a test host does so its steps reach every view: what waits is made
// in the next commit.
func (t *Tree) SetViewPacing(on bool) {
	if t.views.off == !on {
		return
	}
	t.views.off = !on
	if !on {
		for _, n := range t.waiting {
			t.makeNow(n)
		}
	}
}

// resetPace starts a commit's budget: views waiting from an earlier one
// leave it only theirs, nothing more made first.
func (t *Tree) resetPace() {
	t.views.made = 0
	if len(t.waiting) > 0 {
		t.views.made = paceAfter
	}
	t.views.paced = 0
	t.views.started = time.Time{}
}

// left reports whether the commit may make another waiting view out of
// sight.
func (t *Tree) paceLeft() bool {
	return t.views.paced < paceViews && t.rt.clock.Now().Sub(t.views.started) < paceViewsTime
}

// holdView reports whether c, about to be made in p, waits, putting it in
// Tree.waiting.
func (t *Tree) holdView(p, c *node) bool {
	if t.views.off || c.pace == paceNow || !t.views.all && t.views.made < paceAfter || neverPaced(t, c) || !pacesIn(p) {
		return false
	}
	if c.pace != paceWaiting {
		c.pace = paceWaiting
		t.waiting = append(t.waiting, c)
	}
	return true
}

// neverPaced reports whether n is made with its parent wherever it is.
func neverPaced(t *Tree, n *node) bool {
	return t.sticky[n] || n.follow != nil || n.hoist != nil || n.isRoot || n.attach != nil ||
		n.h.layout != nil || n.input != nil && n.input.autoFocus || moves(n)
}

// moves reports whether n shows away from its frame: animated or
// transformed.
func moves(n *node) bool {
	return n.style.Animated != nil || n.anim != nil || n.style.Transform != (Transform{}) || len(n.style.Transforms) > 0
}

// pacesIn reports whether views made in p may wait: it is in a scroll view
// or a list, whose content shows only within its viewport.
func pacesIn(p *node) bool {
	for ; p != nil; p = p.nativeParent() {
		if p.scroll != nil || p.list != nil {
			return true
		}
		if p.isRoot {
			return false
		}
	}
	return false
}

// sight is how far waiting n's frame is from showing, in points: 0 when it
// meets the viewport of every scroll view around it, or when that cannot
// be told (it is empty, or a transform or a zoom is between it and the
// viewport), so it is made now; the most when it is hidden.
func sight(n *node) float32 {
	if n.hidden {
		return math.MaxFloat32
	}
	f := n.frame
	if f.Size.Width <= 0 || f.Size.Height <= 0 {
		return 0
	}
	r := [4]Points{f.Origin.X, f.Origin.Y, f.Origin.X + f.Size.Width, f.Origin.Y + f.Size.Height}
	var far Points
	for p := n.nativeParent(); p != nil; p = p.nativeParent() {
		if !p.created || moves(p) {
			// Past the viewports met so far, nothing moves it within them.
			return float32(far)
		}
		var off Point
		if sc := p.scroll; sc != nil {
			if sc.config.Zooms() {
				return float32(far)
			}
			off = sc.offset
			far = max(far, gap(r, [4]Points{off.X, off.Y, off.X + p.frame.Size.Width, off.Y + p.frame.Size.Height}))
		}
		dx, dy := p.frame.Origin.X-off.X, p.frame.Origin.Y-off.Y
		r = [4]Points{r[0] + dx, r[1] + dy, r[2] + dx, r[3] + dy}
		if p.isRoot {
			break
		}
	}
	return float32(far)
}

// gap is how far rect r is from rect v (left, top, right, bottom): 0 when
// they meet.
func gap(r, v [4]Points) Points {
	return max(v[0]-r[2], r[0]-v[2], v[1]-r[3], r[1]-v[3], 0)
}

// makeNow has the next commit make n and the views around it not made yet,
// whatever it costs.
func (t *Tree) makeNow(n *node) {
	for ; n != nil && !n.created && !n.deleted; n = n.nativeParent() {
		n.pace = paceNow
		t.mark(n)
		if p := n.nativeParent(); p != nil {
			t.reorder(p)
		}
	}
}

// makeCommanded makes the views the commit's commands name, which a
// backend must have.
func (t *Tree) makeCommanded() {
	if len(t.waiting) == 0 {
		return
	}
	at := func(id NodeID) {
		if int(id) < len(t.nodes) {
			if n := t.nodes[id]; n != nil && !n.created {
				t.makeNow(n)
			}
		}
	}
	for i := range t.commands {
		it := &t.commands[i]
		if it.ID != 0 {
			at(it.ID)
		}
		for _, c := range it.Sticky {
			at(c.Cell)
		}
	}
	if t.focus != nil && !t.focus.created {
		t.makeNow(t.focus)
	}
}

// syncPaced makes the waiting views that show, down to their leaves, then
// as many of the others as the budget allows, nearest first.
func (t *Tree) syncPaced() {
	type ranked struct {
		far float32
		n   *node
	}
	var rank []ranked
	for len(t.waiting) > 0 {
		rank = rank[:0]
		for _, n := range t.waiting {
			if n.created || n.deleted || n.pace != paceWaiting {
				continue
			}
			// Its parent not made, or gone: it waits again as its parent
			// makes it.
			if p := n.nativeParent(); p == nil || !p.created || p.deleted {
				n.pace = 0
				continue
			}
			far := float32(0)
			if !t.views.off {
				far = sight(n)
			}
			rank = append(rank, ranked{far, n})
		}
		t.waiting = t.waiting[:0]
		slices.SortStableFunc(rank, func(a, b ranked) int {
			switch {
			case a.far < b.far:
				return -1
			case a.far > b.far:
				return 1
			}
			return 0
		})
		shown, made := false, false
		for _, r := range rank {
			if r.far > 0 {
				if t.views.started.IsZero() {
					t.views.started = t.rt.clock.Now()
				}
				if !t.paceLeft() {
					t.waiting = append(t.waiting, r.n)
					continue
				}
				t.views.paced++
			} else {
				shown = true
			}
			r.n.pace = paceNow
			t.mark(r.n)
			t.reorder(r.n.nativeParent())
			made = true
		}
		if !made {
			break
		}
		// One level: the children of what it makes wait, ranked in the
		// next pass.
		t.views.all = true
		t.emit(t.view)
		t.views.all = false
		// Only what shows goes down a level in this commit; the rest waits
		// for the budget.
		if !shown {
			break
		}
	}
}

// madeChildren is kids without the views that wait.
func madeChildren(kids []*node) []*node {
	i := slices.IndexFunc(kids, func(c *node) bool { return !c.created })
	if i < 0 {
		return kids
	}
	out := slices.Clone(kids[:i])
	for _, c := range kids[i:] {
		if c.created {
			out = append(out, c)
		}
	}
	return out
}
