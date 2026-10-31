package base

import (
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// A dock layout is data, as gpui-base's and NSH's: each region (the
// center, and an optional left, right and bottom dock) is a tree of
// DockNodes, splits of children along an axis and tab groups of panels
// shown one at a time. A panel always lives in a tab group, and is the
// app's key. Containers keep their ID across every edit, and every edit
// normalizes its tree before it returns.

// DockNode is one container of a dock layout: a tab group (Tabs) of
// Panels, Active the one displayed, or a split of Children along Axis
// with their slot Sizes (0 for none: the child shares what the others
// leave).
type DockNode struct {
	// ID names the container across edits. A layout the app writes may
	// leave it 0; the area assigns it, and reports layouts with it.
	ID       uint64
	Tabs     bool
	Panels   []string
	Active   int
	Axis     Axis
	Children []DockNode
	Sizes    []core.Points
}

// DockTabs returns a tab group of panels, the first displayed.
func DockTabs(panels ...string) DockNode { return DockNode{Tabs: true, Panels: panels} }

// DockSplit returns a split of children along axis, sharing it evenly.
func DockSplit(axis Axis, children ...DockNode) DockNode {
	return DockNode{Axis: axis, Children: children}
}

// DockRegion is a dock: its tree, its size across its edge (200 when 0,
// never below PanelMinSize) and whether it is closed. A collapsible dock
// dragged below 50 points closes, keeping its size; NoCollapse keeps it
// open.
type DockRegion struct {
	Root       DockNode
	Size       core.Points
	Closed     bool
	NoCollapse bool
}

// DockLayout is a whole DockArea's layout: the center and the docks.
type DockLayout struct {
	Center              DockNode
	Left, Right, Bottom *DockRegion
}

// DockPlacement is a region of a DockArea.
type DockPlacement uint8

const (
	DockCenter DockPlacement = iota
	DockLeft
	DockBottom
	DockRight
)

func (p DockPlacement) String() string { return [...]string{"center", "left", "bottom", "right"}[p] }

// dockDefaultSize is the size of a dock nothing sized, and dockCollapse
// the size below which a collapsible dock let go closes.
const (
	dockDefaultSize core.Points = 2 * PanelMinSize
	dockCollapse    core.Points = PanelMinSize / 2
)

// Region returns the dock at p, or nil; the center is no dock.
func (l *DockLayout) Region(p DockPlacement) *DockRegion {
	switch p {
	case DockLeft:
		return l.Left
	case DockRight:
		return l.Right
	case DockBottom:
		return l.Bottom
	}
	return nil
}

// tree returns region p's root.
func (l *DockLayout) tree(p DockPlacement) *DockNode {
	if p == DockCenter {
		return &l.Center
	}
	if r := l.Region(p); r != nil {
		return &r.Root
	}
	return nil
}

// regions are the placements a layout may hold, in render order.
var regions = [...]DockPlacement{DockCenter, DockLeft, DockRight, DockBottom}

// Clone returns a deep copy of l.
func (l DockLayout) Clone() DockLayout {
	out := DockLayout{Center: l.Center.clone()}
	for _, p := range regions[1:] {
		if r := l.Region(p); r != nil {
			c := *r
			c.Root = r.Root.clone()
			out.setRegion(p, &c)
		}
	}
	return out
}

func (l *DockLayout) setRegion(p DockPlacement, r *DockRegion) {
	switch p {
	case DockLeft:
		l.Left = r
	case DockRight:
		l.Right = r
	case DockBottom:
		l.Bottom = r
	}
}

func (n DockNode) clone() DockNode {
	n.Panels = slices.Clone(n.Panels)
	n.Sizes = slices.Clone(n.Sizes)
	if n.Children != nil {
		kids := make([]DockNode, len(n.Children))
		for i, c := range n.Children {
			kids[i] = c.clone()
		}
		n.Children = kids
	}
	return n
}

// walk calls fn on n and every container inside it, parents first.
func (n *DockNode) walk(fn func(*DockNode)) {
	fn(n)
	for i := range n.Children {
		n.Children[i].walk(fn)
	}
}

// find returns the container id in n, or nil.
func (n *DockNode) find(id uint64) *DockNode {
	var out *DockNode
	n.walk(func(c *DockNode) {
		if out == nil && c.ID == id {
			out = c
		}
	})
	return out
}

// parentOf returns the split holding container id in n and its index.
func (n *DockNode) parentOf(id uint64) (*DockNode, int) {
	for i := range n.Children {
		if n.Children[i].ID == id {
			return n, i
		}
		if p, at := n.Children[i].parentOf(id); p != nil {
			return p, at
		}
	}
	return nil, -1
}

// groupOf returns the tab group holding panel in n, or nil.
func (n *DockNode) groupOf(panel string) *DockNode {
	var out *DockNode
	n.walk(func(c *DockNode) {
		if out == nil && c.Tabs && slices.Contains(c.Panels, panel) {
			out = c
		}
	})
	return out
}

// panels returns the panels of n in tree order.
func (n *DockNode) panels() []string {
	var out []string
	n.walk(func(c *DockNode) { out = append(out, c.Panels...) })
	return out
}

// groups counts the tab groups of n.
func (n *DockNode) groups() int {
	count := 0
	n.walk(func(c *DockNode) {
		if c.Tabs {
			count++
		}
	})
	return count
}

// normalize collapses tree n to its canonical shape, as NSH's
// PaneTree::normalize: one pass bottom up, repeated until nothing
// changes. Empty containers go, a split with one child is replaced by it
// (which keeps its ID), a split inside one of the same axis is spliced
// in with its sizes scaled to the slot, and Active is clamped. The root
// stays a split when keepSplit. It reports whether it changed anything.
func (n *DockNode) normalize(keepSplit bool) bool {
	any := false
	for range 64 {
		changed := n.normalizeOnce()
		if !keepSplit && !n.Tabs && len(n.Children) == 1 {
			*n = n.Children[0]
			changed = true
		}
		if !changed {
			break
		}
		any = true
	}
	return any
}

func (n *DockNode) normalizeOnce() bool {
	changed := false
	if n.Tabs {
		if c := min(n.Active, max(len(n.Panels)-1, 0)); c != n.Active || n.Active < 0 {
			n.Active = max(c, 0)
			changed = true
		}
		return changed
	}
	if len(n.Sizes) != len(n.Children) {
		n.Sizes = fitSizes(n.Sizes, len(n.Children))
		changed = true
	}
	for i := range n.Children {
		if n.Children[i].normalizeOnce() {
			changed = true
		}
	}
	for i := 0; i < len(n.Children); {
		c := &n.Children[i]
		if c.Tabs && len(c.Panels) == 0 || !c.Tabs && len(c.Children) == 0 {
			n.Children = slices.Delete(n.Children, i, i+1)
			n.Sizes = slices.Delete(n.Sizes, i, i+1)
			changed = true
			continue
		}
		i++
	}
	for i := range n.Children {
		if c := &n.Children[i]; !c.Tabs && len(c.Children) == 1 {
			*c = c.Children[0]
			changed = true
		}
	}
	for i := 0; i < len(n.Children); {
		c := n.Children[i]
		if c.Tabs || c.Axis != n.Axis {
			i++
			continue
		}
		inner := distributeSlot(n.Sizes[i], c.Sizes)
		n.Children = slices.Replace(n.Children, i, i+1, c.Children...)
		n.Sizes = slices.Replace(n.Sizes, i, i+1, inner...)
		i += len(c.Children)
		changed = true
	}
	return changed
}

// fitSizes is sizes made count long.
func fitSizes(sizes []core.Points, count int) []core.Points {
	out := make([]core.Points, count)
	copy(out, sizes)
	return out
}

// distributeSlot scales inner sizes to fill slot, when every one is set.
func distributeSlot(slot core.Points, inner []core.Points) []core.Points {
	if slot <= 0 {
		return inner
	}
	var total core.Points
	for _, s := range inner {
		if s <= 0 {
			return inner
		}
		total += s
	}
	if total <= 0 {
		return inner
	}
	out := make([]core.Points, len(inner))
	for i, s := range inner {
		out[i] = s * slot / total
	}
	return out
}

// dockTarget is where a panel lands: into tab group Node at Index (-1
// last), or in a new tab group on Side of container Node.
type dockTarget struct {
	region   DockPlacement
	node     uint64
	index    int
	split    bool
	side     core.Placement
	activate bool
	// size is the new group's slot along a split, half of what it lands
	// beside when 0.
	size core.Points
}

// insert puts panel at t in tree n, allocating containers' IDs with
// next. It reports whether the panel landed.
func (n *DockNode) insert(panel string, t dockTarget, next func() uint64) bool {
	target := n.find(t.node)
	if target == nil {
		return false
	}
	if !t.split {
		if !target.Tabs {
			return false
		}
		at := t.index
		if at < 0 || at > len(target.Panels) {
			at = len(target.Panels)
		}
		target.Panels = slices.Insert(target.Panels, at, panel)
		switch {
		case t.activate:
			target.Active = at
		case at <= target.Active && len(target.Panels) > 1:
			target.Active++
		}
		return true
	}
	group := DockNode{ID: next(), Tabs: true, Panels: []string{panel}}
	before := t.side == core.PlaceLeft || t.side == core.PlaceTop
	dir := Horizontal
	if t.side == core.PlaceTop || t.side == core.PlaceBottom {
		dir = Vertical
	}
	if parent, i := n.parentOf(t.node); parent != nil && parent.Axis == dir {
		share, rest := parent.Sizes[i]/2, parent.Sizes[i]/2
		if t.size > 0 {
			share, rest = t.size, max(parent.Sizes[i]-t.size, 0)
		}
		parent.Sizes[i] = rest
		at := i + 1
		if before {
			at = i
		}
		parent.Children = slices.Insert(parent.Children, at, group)
		parent.Sizes = slices.Insert(parent.Sizes, at, share)
		return true
	}
	old := *target
	wrapper := DockNode{ID: next(), Axis: dir, Children: []DockNode{old, group}, Sizes: []core.Points{0, t.size}}
	if before {
		wrapper.Children, wrapper.Sizes = []DockNode{group, old}, []core.Points{t.size, 0}
	}
	*target = wrapper
	return true
}

// detach takes panel out of tree n, reporting whether it was there.
func (n *DockNode) detach(panel string) bool {
	g := n.groupOf(panel)
	if g == nil {
		return false
	}
	i := slices.Index(g.Panels, panel)
	g.Panels = slices.Delete(g.Panels, i, i+1)
	if i < g.Active {
		g.Active--
	}
	return true
}

// install repairs a layout the app wrote: IDs assigned where it left
// them out and replaced where repeated, a panel listed twice kept in its
// first place, sizes fitted to the children and every tree normalized.
// next allocates IDs above every ID it holds.
func installDock(l DockLayout, next *uint64) DockLayout {
	l = l.Clone()
	for _, p := range regions {
		if t := l.tree(p); t != nil {
			t.walk(func(c *DockNode) { *next = max(*next, c.ID) })
		}
	}
	seen := map[uint64]bool{}
	panels := map[string]bool{}
	for _, p := range regions {
		t := l.tree(p)
		if t == nil {
			continue
		}
		t.walk(func(c *DockNode) {
			if c.ID == 0 || seen[c.ID] {
				*next++
				c.ID = *next
			}
			seen[c.ID] = true
			if c.Tabs {
				kept := c.Panels[:0:0]
				for _, k := range c.Panels {
					if !panels[k] {
						panels[k] = true
						kept = append(kept, k)
					}
				}
				c.Panels = kept
				c.Children, c.Sizes = nil, nil
			}
		})
		t.normalize(p == DockCenter)
		if p == DockCenter && t.Tabs {
			// The center's root stays a split.
			*next++
			*t = DockNode{ID: *next, Children: []DockNode{*t}, Sizes: []core.Points{0}}
		}
	}
	return l
}
