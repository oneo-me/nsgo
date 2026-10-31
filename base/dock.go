package base

import (
	"reflect"
	"slices"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// DockArea is a dockable workspace, as gpui-base's and NSH's: a center
// and optional left, right and bottom docks, each a tree of splits and
// tab groups the user rearranges by dragging tabs between groups and
// docks, resizes with the handles between them and zooms into. Its
// layout is data the app passes and hears back whole on every edit, and
// persists in gpui-base's format (DockLayout's JSON). It draws nothing:
// the app styles every part and renders each panel and tab.
//
// An area narrower than 600 points (on phones, in narrow windows or
// columns) keeps the same layout and presents it compactly: the center
// shows one tab group at a time, its tab bar listing every panel of the
// center, a tab drag reorders within it, and the docks are drawers from
// their edges and a sheet at the bottom.
//
// A panel's content keeps its state while its group lives, across tab
// switches, resizes, zooms, docks closing and the compact presentation
// in the center; a panel moved to another group, or a dock's panel as
// the presentation changes, is rendered anew.
type DockArea struct {
	// Layout makes the area controlled: it shows what Layout returns and
	// reports edits with OnLayoutChange, which the app passes back.
	// Without it the area keeps its own, from DefaultLayout.
	Layout         func() DockLayout
	DefaultLayout  DockLayout
	OnLayoutChange func(DockLayout)
	// Locked rearranges nothing: no drags, drops or closes. Selecting,
	// resizing and toggling docks still work.
	Locked func() bool
	// Compact chooses the presentation; automatic by the area's width
	// when unset.
	Compact core.Toggle
	// Panels says what each panel allows, by key.
	Panels map[string]DockPanelOptions

	// RenderPanel renders a panel's content, by key.
	RenderPanel func(cx *core.Scope, key string) core.Node
	// RenderTab renders what a tab shows, the key as text without it; the
	// area makes it the tab, whose press selects and drag moves it.
	RenderTab func(cx *core.Scope, tab DockTab) core.Node
	// RenderDropIndicator renders what shows where a drag would land,
	// over what it would take; a view of DropIndicatorStyle without it.
	RenderDropIndicator func(cx *core.Scope, d DockDropIndicator) core.Node
	// RenderDock renders a dock's own chrome, before its layout.
	RenderDock func(cx *core.Scope, d DockInfo) core.Node
	// RenderEmpty renders what an empty center shows.
	RenderEmpty func(cx *core.Scope) core.Node
	// RenderDragPreview renders what follows the pointer as a tab is
	// dragged.
	RenderDragPreview func(cx *core.Scope, tab DockTab) core.Node

	// The parts' styles: each tab group's frame, its tab bar, each tab
	// (Selected for the displayed one), the content frame its panels show
	// in, the drop indicator, each handle's line and thumb (between the
	// groups of a split and at each dock's edge; a GroupStyle follows the
	// handle's hover and press), each dock's frame, and in the compact
	// presentation the side docks' drawers, the bottom dock's sheet and
	// the backdrop behind them. Each part's Func, when set, merges over
	// its style, again when what it reads changes, and its class goes
	// under both.
	Class, GroupClass, TabBarClass, TabClass, ContentClass                     string
	DropIndicatorClass, HandleClass, HandleThumbClass                          string
	DockClass, DrawerClass, SheetClass, BackdropClass                          string
	Style, GroupStyle, TabBarStyle, TabStyle, ContentStyle                     core.Style
	StyleFunc, GroupStyleFunc, TabBarStyleFunc, TabStyleFunc, ContentStyleFunc func() core.Style
	DropIndicatorStyle, HandleStyle, HandleThumbStyle                          core.Style
	DropIndicatorStyleFunc, HandleStyleFunc, HandleThumbStyleFunc              func() core.Style
	DockStyle, DrawerStyle, SheetStyle, BackdropStyle                          core.Style
	DockStyleFunc, DrawerStyleFunc, SheetStyleFunc, BackdropStyleFunc          func() core.Style
	// HandleLabel says what dragging a handle resizes, for assistive
	// technology.
	HandleLabel string

	// OnPanelActiveChange follows a panel becoming, or stopping being,
	// its group's displayed tab; a closed dock displays none, and a panel
	// that leaves hears OnPanelRemove instead.
	OnPanelActiveChange func(key string, active bool)
	// OnPanelRemove reports a panel that left the area for good.
	OnPanelRemove func(key string)
	// OnZoomChange reports the displayed panel of the group zoomed in, or
	// "" as it zooms out.
	OnZoomChange func(key string)
	// OnDockOpenChange reports a dock showing or going, in either
	// presentation, and as the area mounts the docks whose shown state
	// differs from the layout's.
	OnDockOpenChange func(p DockPlacement, open bool)
	// Ref, when set, is bound to the area's methods.
	Ref    *DockAreaRef
	TestID string
}

// DockPanelOptions are what a panel allows.
type DockPanelOptions struct {
	Closable bool
	NoZoom   bool
	// Hidden keeps the panel's tab out of its tab bar.
	Hidden bool
}

// DockTab is what RenderTab and RenderDragPreview get about a tab.
type DockTab struct {
	Key   string
	Group uint64
	// Selected reports whether it is its group's displayed tab, Zoomed
	// whether its group is zoomed in.
	Selected, Zoomed func() bool
	Closable         bool
	// Close closes the panel, where it may.
	Close func()
}

// DockInfo is what RenderDock gets about a dock.
type DockInfo struct {
	Placement DockPlacement
	Open      func() bool
	Toggle    func()
}

// DockDropIndicator is where a drag would land: into Group, or in a new
// group on Side of it.
type DockDropIndicator struct {
	Group uint64
	Side  core.Placement
}

// DockTarget is where DockAreaRef.MovePanel puts a panel: into tab
// group Group, or with Side in a new tab group on that side of container
// Group.
type DockTarget struct {
	Group uint64
	Side  core.Placement
	// Before is the tab it goes before; it goes last without one.
	Before string
	// NoActivate keeps the group's displayed tab.
	NoActivate bool
	// Size is the new group's length along a split, half of what it lands
	// beside when 0.
	Size core.Points
}

// DockAreaRef is a DockArea's methods, bound once it mounts. Each edit
// is reported with OnLayoutChange, as the user's are.
type DockAreaRef struct{ a *dockArea }

// Layout returns the layout the area shows.
func (r *DockAreaRef) Layout() DockLayout {
	if r.a == nil {
		return DockLayout{}
	}
	return core.Untracked(r.a.cx, r.a.layout).Clone()
}

// AddPanel adds panel key to the first tab group of region p, displayed;
// a dock missing is made, size across (200 when 0). A panel the area
// holds moves there.
func (r *DockAreaRef) AddPanel(key string, p DockPlacement, size core.Points) {
	if r.a != nil {
		r.a.add(key, p, size)
	}
}

// RemovePanel takes panel key out of the area, whatever its options say.
func (r *DockAreaRef) RemovePanel(key string) {
	if r.a != nil {
		r.a.remove(key)
	}
}

// ClosePanel takes panel key out of the area as its tab would: unless
// the area is locked, the panel is not closable, or it is the last panel
// of a group with nothing beside it.
func (r *DockAreaRef) ClosePanel(key string) {
	if r.a != nil {
		r.a.close(key)
	}
}

// MovePanel moves panel key to t; a panel the area does not hold is
// added there.
func (r *DockAreaRef) MovePanel(key string, t DockTarget) {
	if r.a != nil {
		r.a.move(key, t)
	}
}

// SplitAt puts panel key in a new tab group on side of container group.
func (r *DockAreaRef) SplitAt(group uint64, key string, side core.Placement) {
	if r.a != nil && side != core.PlaceDefault {
		r.a.move(key, DockTarget{Group: group, Side: side})
	}
}

// SelectPanel displays panel key in its group.
func (r *DockAreaRef) SelectPanel(key string) {
	if r.a != nil {
		r.a.activate(key)
	}
}

// SelectTab displays the index'th panel of tab group group.
func (r *DockAreaRef) SelectTab(group uint64, index int) {
	if r.a == nil {
		return
	}
	l := r.a.untracked()
	if g := findGroup(&l, group); g != nil && g.Tabs && index >= 0 && index < len(g.Panels) {
		r.a.activate(g.Panels[index])
	}
}

// SetDockOpen opens or closes dock p; at compact width it presents it,
// dismissing the other, or dismisses it.
func (r *DockAreaRef) SetDockOpen(p DockPlacement, open bool) {
	if r.a != nil {
		r.a.setDockOpen(p, open)
	}
}

// ToggleDock opens dock p when it is closed and closes it otherwise.
func (r *DockAreaRef) ToggleDock(p DockPlacement) {
	if r.a != nil {
		r.a.setDockOpen(p, !core.Untracked(r.a.cx, func() bool { return r.a.shown(p) }))
	}
}

// SetDockSize sets dock p's size across its edge, at least PanelMinSize.
func (r *DockAreaRef) SetDockSize(p DockPlacement, size core.Points) {
	if r.a == nil {
		return
	}
	r.a.edit(func(l *DockLayout) bool {
		d := l.Region(p)
		if d == nil || d.Size == max(size, PanelMinSize) {
			return false
		}
		d.Size = max(size, PanelMinSize)
		return true
	})
}

// ToggleZoom zooms tab group group in, so it fills the area, or out when
// it is zoomed; 0 zooms the zoomed group out.
func (r *DockAreaRef) ToggleZoom(group uint64) {
	if r.a != nil {
		r.a.toggleZoom(group)
	}
}

// ZoomedGroup returns the zoomed tab group's ID, 0 for none.
func (r *DockAreaRef) ZoomedGroup() uint64 {
	if r.a == nil {
		return 0
	}
	return r.a.zoom.GetUntracked()
}

// GroupOf returns the ID of the tab group holding panel key, 0 for none.
func (r *DockAreaRef) GroupOf(key string) uint64 {
	if r.a == nil {
		return 0
	}
	l := r.a.untracked()
	if _, g := regionOf(&l, key); g != nil {
		return g.ID
	}
	return 0
}

// The width below which an area is compact, Material 3's compact class;
// how far a pointer drags a tab before it moves; and how long a finger
// holds a tab before it drags.
const (
	dockCompactWidth core.Points = 600
	dockDragSlop     core.Points = 4
	dockSplitZone                = 0.35
)

// dockArea is a mounted DockArea.
type dockArea struct {
	cx *core.Scope
	p  DockArea
	// own is the layout of an area the app does not control; app the
	// app's layout installed.
	own  *core.Signal[DockLayout]
	app  *core.Memo[*DockLayout]
	next uint64
	ref  *core.Ref
	// size is the area's size as last laid out.
	size *core.Signal[core.Size]
	// zoom is the zoomed group, 0 none.
	zoom *core.Signal[uint64]
	// current is the compact presentation's center group; presented its
	// dock, DockCenter for none.
	current   *core.Signal[uint64]
	presented *core.Signal[DockPlacement]
	// live is a dock's size while its edge is dragged, 0 otherwise.
	live [4]*core.Signal[core.Points]
	// groups are the mounted tab groups, by ID, for a drag to measure.
	groups map[uint64]*dockGroup
	// held is each panel's content, by key, which the group showing the
	// panel places.
	held  map[string]*core.Holder
	drag  *core.Signal[*dockDrag]
	armed string
}

// dockGroup is a mounted tab group's measurable parts.
type dockGroup struct {
	region       DockPlacement
	bar, content *core.Ref
	tabs         map[string]*core.Ref
}

// dockDrag is a tab drag going on.
type dockDrag struct {
	panel  string
	region DockPlacement
	group  uint64
	at     core.Point
	target *dockTarget
	// bounds is what the drop indicator covers, in the surface.
	bounds core.Bounds
}

func layoutEqual(a, b *DockLayout) bool { return reflect.DeepEqual(a, b) }

// Render implements core.Node.
func (d DockArea) Render(cx *core.Scope) core.Node {
	a := &dockArea{cx: cx, p: d, ref: new(core.Ref), groups: map[uint64]*dockGroup{}, held: map[string]*core.Holder{},
		size: core.NewSignal(cx, core.Size{}), zoom: core.NewSignal(cx, uint64(0)),
		current: core.NewSignal(cx, uint64(0)), presented: core.NewSignal(cx, DockCenter),
		drag: core.NewSignalFunc[*dockDrag](cx, nil, func(a, b *dockDrag) bool { return a == b })}
	for i := range a.live {
		a.live[i] = core.NewSignal(cx, core.Points(0))
	}
	if d.Layout != nil {
		a.app = core.NewMemoFunc(cx, func() *DockLayout {
			l := installDock(d.Layout(), &a.next)
			return &l
		}, layoutEqual)
	} else {
		a.own = core.NewSignalFunc(cx, installDock(d.DefaultLayout, &a.next), func(x, y DockLayout) bool { return layoutEqual(&x, &y) })
	}
	if d.Ref != nil {
		d.Ref.a = a
	}
	a.follow()

	root := func() core.Style {
		s := core.Style{Direction: core.Row, Overflow: core.Hidden, MinWidth: core.Pt(0), MinHeight: core.Pt(0)}
		if d.StyleFunc != nil {
			s = s.Merge(d.StyleFunc())
		}
		return s
	}
	column := func() core.Style {
		if a.zoomedIn(DockCenter) {
			return core.Style{Display: core.DisplayNone}
		}
		return core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), MinHeight: core.Pt(0)}
	}
	full := func() bool { return !a.compact() }
	return core.View{Ref: a.ref, Class: d.Class, Style: d.Style, StyleFunc: root, TestID: d.TestID,
		OnLayout: func(b core.Bounds) {
			if b.Size != a.size.GetUntracked() {
				a.size.Set(b.Size)
			}
		},
		Children: []core.Node{
			core.Show{When: full, Then: func(cx *core.Scope) core.Node { return a.dock(cx, DockLeft) }},
			core.View{StyleFunc: column, Children: []core.Node{
				a.center(cx),
				core.Show{When: full, Then: func(cx *core.Scope) core.Node { return a.dock(cx, DockBottom) }},
			}},
			core.Show{When: full, Then: func(cx *core.Scope) core.Node { return a.dock(cx, DockRight) }},
			core.Show{When: a.compact, Then: func(cx *core.Scope) core.Node {
				return core.Group{a.drawer(cx, DockLeft), a.drawer(cx, DockRight), a.drawer(cx, DockBottom)}
			}},
			a.indicator(),
		}}
}

// layout returns the layout shown, tracked.
func (a *dockArea) layout() DockLayout {
	if a.app != nil {
		return *a.app.Get()
	}
	return a.own.Get()
}

func (a *dockArea) untracked() DockLayout { return core.Untracked(a.cx, a.layout) }

func (a *dockArea) locked() bool { return a.p.Locked != nil && a.p.Locked() }

func (a *dockArea) options(key string) DockPanelOptions { return a.p.Panels[key] }

// compact reports, tracked, whether the area presents compactly.
func (a *dockArea) compact() bool {
	switch a.p.Compact {
	case core.ToggleOn:
		return true
	case core.ToggleOff:
		return false
	}
	w := a.size.Get().Width
	if w == 0 {
		w = a.cx.Env().Window().Width
	}
	return w < dockCompactWidth
}

// edit applies fn to a copy of the layout and reports the result when it
// changed anything.
func (a *dockArea) edit(fn func(l *DockLayout) bool) {
	old := a.untracked()
	l := old.Clone()
	if !fn(&l) {
		return
	}
	for _, p := range regions {
		if t := l.tree(p); t != nil {
			t.normalize(p == DockCenter)
		}
	}
	if layoutEqual(&l, &old) {
		return
	}
	if a.own != nil {
		a.own.Set(l)
	}
	if a.p.OnLayoutChange != nil {
		a.p.OnLayoutChange(l.Clone())
	}
}

func (a *dockArea) alloc() uint64 {
	a.next++
	return a.next
}

// displayed returns the panels each open region's groups display.
func displayed(l *DockLayout) map[string]bool {
	out := map[string]bool{}
	for _, p := range regions {
		t := l.tree(p)
		if t == nil || p != DockCenter && l.Region(p).Closed {
			continue
		}
		t.walk(func(c *DockNode) {
			if c.Tabs && c.Active < len(c.Panels) {
				out[c.Panels[c.Active]] = true
			}
		})
	}
	return out
}

func allPanels(l *DockLayout) map[string]bool {
	out := map[string]bool{}
	for _, p := range regions {
		if t := l.tree(p); t != nil {
			for _, k := range t.panels() {
				out[k] = true
			}
		}
	}
	return out
}

// follow reports what changed between the layouts the area shows: the
// panels displayed, those that left, the zoom and the docks shown.
func (a *dockArea) follow() {
	cx := a.cx
	var was *DockLayout
	cx.Effect(func() {
		l := a.layout()
		cx.Untrack(func() {
			if z := a.zoom.GetUntracked(); z != 0 && findGroup(&l, z) == nil {
				a.setZoom(0)
			}
			present := allPanels(&l)
			for k, h := range a.held {
				if !present[k] {
					h.Dispose()
					delete(a.held, k)
				}
			}
			if was != nil {
				before, after := displayed(was), displayed(&l)
				if f := a.p.OnPanelActiveChange; f != nil {
					for _, k := range sortedKeys(before) {
						if !after[k] && present[k] {
							f(k, false)
						}
					}
					for _, k := range sortedKeys(after) {
						if !before[k] {
							f(k, true)
						}
					}
				}
				if f := a.p.OnPanelRemove; f != nil {
					for _, k := range sortedKeys(allPanels(was)) {
						if !present[k] {
							f(k)
						}
					}
				}
			}
			was = &l
		})
	})
	shown := map[DockPlacement]bool{}
	first := true
	cx.Effect(func() {
		now := map[DockPlacement]bool{}
		for _, p := range regions[1:] {
			now[p] = a.shown(p)
		}
		cx.Untrack(func() {
			l := a.untracked()
			for _, p := range []DockPlacement{DockLeft, DockRight, DockBottom} {
				before := shown[p]
				if first {
					r := l.Region(p)
					before = r != nil && !r.Closed
				}
				if now[p] != before && a.p.OnDockOpenChange != nil {
					a.p.OnDockOpenChange(p, now[p])
				}
			}
			shown, first = now, false
		})
	})
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func findGroup(l *DockLayout, id uint64) *DockNode {
	for _, p := range regions {
		if t := l.tree(p); t != nil {
			if n := t.find(id); n != nil {
				return n
			}
		}
	}
	return nil
}

// regionOf returns the region holding panel key.
func regionOf(l *DockLayout, key string) (DockPlacement, *DockNode) {
	for _, p := range regions {
		if t := l.tree(p); t != nil {
			if g := t.groupOf(key); g != nil {
				return p, g
			}
		}
	}
	return DockCenter, nil
}

// shown reports, tracked, whether dock p shows in the presentation.
func (a *dockArea) shown(p DockPlacement) bool {
	if a.compact() {
		return a.presented.Get() == p
	}
	l := a.layout()
	r := l.Region(p)
	return r != nil && !r.Closed
}

func (a *dockArea) setDockOpen(p DockPlacement, open bool) {
	if p == DockCenter {
		return
	}
	if core.Untracked(a.cx, a.compact) {
		switch {
		case open:
			a.presented.Set(p)
		case a.presented.GetUntracked() == p:
			a.presented.Set(DockCenter)
		}
		return
	}
	a.edit(func(l *DockLayout) bool {
		r := l.Region(p)
		if r == nil || r.Closed == !open {
			return false
		}
		r.Closed = !open
		return true
	})
}

func (a *dockArea) setZoom(id uint64) {
	if a.zoom.GetUntracked() == id {
		return
	}
	a.zoom.Set(id)
	if f := a.p.OnZoomChange; f != nil {
		key := ""
		l := a.untracked()
		if g := findGroup(&l, id); g != nil && g.Active < len(g.Panels) {
			key = g.Panels[g.Active]
		}
		f(key)
	}
}

func (a *dockArea) toggleZoom(group uint64) {
	if z := a.zoom.GetUntracked(); z != 0 && (group == 0 || group == z) {
		a.setZoom(0)
		return
	}
	l := a.untracked()
	g := findGroup(&l, group)
	if g == nil || !g.Tabs || g.Active >= len(g.Panels) || a.options(g.Panels[g.Active]).NoZoom {
		return
	}
	a.setZoom(g.ID)
}

// activate displays panel key in its group, and makes the group the
// compact presentation's current one.
func (a *dockArea) activate(key string) {
	l := a.untracked()
	region, g := regionOf(&l, key)
	if g == nil {
		return
	}
	if region == DockCenter {
		a.current.Set(g.ID)
	}
	id := g.ID
	a.edit(func(l *DockLayout) bool {
		g := l.tree(region).find(id)
		i := slices.Index(g.Panels, key)
		if i < 0 || g.Active == i {
			return false
		}
		g.Active = i
		return true
	})
}

// movable reports whether panel key may leave its group: not in a
// locked or zoomed area, and not as the last visible panel of a group
// with nothing beside it in its tree, which would empty the region.
func (a *dockArea) movable(key string) bool {
	if a.locked() || a.zoom.GetUntracked() != 0 {
		return false
	}
	l := a.untracked()
	region, g := regionOf(&l, key)
	if g == nil {
		return false
	}
	if l.tree(region).groups() > 1 {
		return true
	}
	visible := 0
	for _, k := range g.Panels {
		if !a.options(k).Hidden {
			visible++
		}
	}
	return visible > 1
}

func (a *dockArea) close(key string) {
	if !a.options(key).Closable || !a.movable(key) {
		return
	}
	a.edit(func(l *DockLayout) bool {
		region, _ := regionOf(l, key)
		return l.tree(region).detach(key)
	})
}

func (a *dockArea) remove(key string) {
	a.edit(func(l *DockLayout) bool {
		region, _ := regionOf(l, key)
		return l.tree(region).detach(key)
	})
}

// move puts panel key where t says, detaching it from where it is.
func (a *dockArea) move(key string, t DockTarget) {
	a.edit(func(l *DockLayout) bool {
		var to DockPlacement
		var n *DockNode
		for _, p := range regions {
			if tree := l.tree(p); tree != nil && n == nil {
				to, n = p, tree.find(t.Group)
			}
		}
		if n == nil || t.Side == core.PlaceDefault && !n.Tabs {
			return false
		}
		if from, g := regionOf(l, key); g != nil {
			l.tree(from).detach(key)
			if from != to {
				l.tree(from).normalize(from == DockCenter)
			}
		}
		target := dockTarget{region: to, node: t.Group, index: -1, activate: !t.NoActivate,
			split: t.Side != core.PlaceDefault, side: t.Side, size: t.Size}
		if !target.split && t.Before != "" {
			target.index = slices.Index(n.Panels, t.Before)
		}
		return l.tree(to).insert(key, target, a.alloc)
	})
}

func (a *dockArea) add(key string, p DockPlacement, size core.Points) {
	a.edit(func(l *DockLayout) bool {
		if from, g := regionOf(l, key); g != nil {
			l.tree(from).detach(key)
			if from != p {
				l.tree(from).normalize(from == DockCenter)
			}
		}
		t := l.tree(p)
		if t == nil {
			l.setRegion(p, &DockRegion{Root: DockNode{ID: a.alloc(), Tabs: true, Panels: []string{key}}, Size: size})
			return true
		}
		var g *DockNode
		t.walk(func(c *DockNode) {
			if g == nil && c.Tabs {
				g = c
			}
		})
		if g == nil {
			id := a.alloc()
			if t.Tabs {
				t.ID, t.Panels = id, []string{key}
				return true
			}
			t.Children = append(t.Children, DockNode{ID: id, Tabs: true, Panels: []string{key}})
			t.Sizes = append(t.Sizes, 0)
			return true
		}
		g.Panels = append(g.Panels, key)
		g.Active = len(g.Panels) - 1
		return true
	})
}

// focus returns the group the presentation narrows region to: the
// zoomed one, or at compact width the center's current one.
func (a *dockArea) focus(region DockPlacement) (uint64, bool) {
	if z := a.zoom.Get(); z != 0 {
		return z, true
	}
	if region != DockCenter || !a.compact() {
		return 0, false
	}
	l := a.layout()
	cur := a.current.Get()
	if l.Center.find(cur) == nil || !l.Center.find(cur).Tabs {
		cur = 0
		l.Center.walk(func(c *DockNode) {
			if cur == 0 && c.Tabs {
				cur = c.ID
			}
		})
	}
	return cur, cur != 0
}

// shows reports, tracked, whether container id of region shows: every
// one does but where the presentation narrows to one group, which shows
// with the containers around it.
func (a *dockArea) shows(region DockPlacement, id uint64) bool {
	g, ok := a.focus(region)
	if !ok {
		return true
	}
	l := a.layout()
	t := l.tree(region)
	if t == nil {
		return false
	}
	n := t.find(id)
	return n != nil && n.find(g) != nil
}

// zoomedIn reports, tracked, whether a zoom hides region p.
func (a *dockArea) zoomedIn(p DockPlacement) bool {
	z := a.zoom.Get()
	if z == 0 {
		return false
	}
	l := a.layout()
	t := l.tree(p)
	return t == nil || t.find(z) == nil
}

// dockShape is what a container's view is made from: its kind and
// children, never its sizes or displayed tab, which bind in place.
type dockShape struct {
	found    bool
	tabs     bool
	dir      Axis
	children []uint64
}

func (a *dockArea) shape(region DockPlacement, id uint64) dockShape {
	l := a.layout()
	t := l.tree(region)
	if t == nil {
		return dockShape{}
	}
	n := t.find(id)
	if n == nil {
		return dockShape{}
	}
	s := dockShape{found: true, tabs: n.Tabs, dir: n.Axis}
	for _, c := range n.Children {
		s.children = append(s.children, c.ID)
	}
	return s
}

// node is the view of container id in region, made again only when its
// shape changes.
func (a *dockArea) node(region DockPlacement, id uint64) core.Node {
	return core.Func(func(cx *core.Scope) core.Node {
		shape := core.NewMemoFunc(cx, func() dockShape { return a.shape(region, id) }, func(x, y dockShape) bool {
			return x.found == y.found && x.tabs == y.tabs && x.dir == y.dir && slices.Equal(x.children, y.children)
		})
		return core.Dynamic(func(cx *core.Scope) core.Node {
			s := shape.Get()
			switch {
			case !s.found:
				return nil
			case s.tabs:
				return a.group(cx, region, id)
			}
			return a.split(region, id, s)
		})
	})
}

// root is the view of region's tree, following its root's ID.
func (a *dockArea) root(region DockPlacement) core.Node {
	return core.Func(func(cx *core.Scope) core.Node {
		id := core.NewMemo(cx, func() uint64 {
			l := a.layout()
			if t := l.tree(region); t != nil {
				return t.ID
			}
			return 0
		})
		return core.Dynamic(func(*core.Scope) core.Node { return a.node(region, id.Get()) })
	})
}

func (a *dockArea) split(region DockPlacement, id uint64, s dockShape) core.Node {
	panels := make([]ResizablePanel, len(s.children))
	for i, child := range s.children {
		panels[i] = ResizablePanel{
			Size: func() core.Points {
				l := a.layout()
				if n := l.tree(region).find(id); n != nil && i < len(n.Sizes) {
					return n.Sizes[i]
				}
				return 0
			},
			Visible: func() bool { return a.shows(region, child) },
			Style:   core.Style{MinWidth: core.Pt(0), MinHeight: core.Pt(0)},
			Children: []core.Node{
				core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)},
					Children: []core.Node{a.node(region, child)}},
			},
		}
	}
	return ResizablePanelGroup{Axis: s.dir, Panels: panels,
		Style:       core.Style{FlexGrow: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)},
		HandleClass: a.p.HandleClass, HandleStyle: a.p.HandleStyle, HandleStyleFunc: a.p.HandleStyleFunc,
		HandleThumbClass: a.p.HandleThumbClass, HandleThumbStyle: a.p.HandleThumbStyle,
		HandleThumbStyleFunc: a.p.HandleThumbStyleFunc, HandleLabel: a.p.HandleLabel,
		OnResize: func(sizes []core.Points) {
			a.edit(func(l *DockLayout) bool {
				n := l.tree(region).find(id)
				if n == nil || len(sizes) != len(n.Children) || slices.Equal(n.Sizes, sizes) {
					return false
				}
				n.Sizes = slices.Clone(sizes)
				return true
			})
		}}
}

// group is a tab group: its tab bar, then its content frame holding a
// view for each of its panels, the displayed one shown.
func (a *dockArea) group(cx *core.Scope, region DockPlacement, id uint64) core.Node {
	g := &dockGroup{region: region, bar: new(core.Ref), content: new(core.Ref), tabs: map[string]*core.Ref{}}
	a.groups[id] = g
	cx.Cleanup(func() {
		if a.groups[id] == g {
			delete(a.groups, id)
		}
	})
	node := func() *DockNode {
		l := a.layout()
		if t := l.tree(region); t != nil {
			return t.find(id)
		}
		return nil
	}
	panels := core.NewMemoFunc(cx, func() []string {
		if n := node(); n != nil {
			return slices.Clone(n.Panels)
		}
		return nil
	}, slices.Equal)
	// The tabs: the group's visible panels, or at compact width every
	// panel of the center.
	tabs := core.NewMemoFunc(cx, func() []string {
		list := panels.Get()
		if region == DockCenter && a.compact() {
			l := a.layout()
			list = l.Center.panels()
		}
		var out []string
		for _, k := range list {
			if !a.options(k).Hidden {
				out = append(out, k)
			}
		}
		return out
	}, slices.Equal)
	shownPanel := func() string {
		n := node()
		if n == nil || n.Active >= len(n.Panels) {
			return ""
		}
		return n.Panels[n.Active]
	}
	bar := tabBar(part{a.p.TabBarClass, a.p.TabBarStyle, a.p.TabBarStyleFunc}, g.bar, core.Each(tabs.Get, func(k string) string { return k },
		func(cx *core.Scope, key func() string, _ func() int) core.Node { return a.tab(cx, g, id, key()) }))
	content := core.View{Ref: g.content, Class: a.p.ContentClass,
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), MinHeight: core.Pt(0),
			Overflow: core.Hidden}.Merge(a.p.ContentStyle),
		StyleFunc: a.p.ContentStyleFunc,
		Children: []core.Node{core.Each(panels.Get, func(k string) string { return k },
			func(cx *core.Scope, key func() string, _ func() int) core.Node {
				k := key()
				return core.View{
					StyleFunc: func() core.Style {
						if shownPanel() != k {
							return core.Style{Display: core.DisplayNone}
						}
						return core.Style{FlexGrow: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)}
					},
					Children: []core.Node{a.panel(k).Place()}}
			})}}
	return core.View{Class: a.p.GroupClass,
		Style:     core.Style{FlexGrow: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)}.Merge(a.p.GroupStyle),
		StyleFunc: a.p.GroupStyleFunc, Children: []core.Node{bar, content}}
}

// panel is panel key's content, kept while the layout lists the panel,
// so it moves with its state to whichever group shows it.
func (a *dockArea) panel(key string) *core.Holder {
	if h := a.held[key]; h != nil {
		return h
	}
	h := core.NewHolder(a.cx, func(cx *core.Scope) core.Node {
		if a.p.RenderPanel != nil {
			return a.p.RenderPanel(cx, key)
		}
		return nil
	})
	a.held[key] = h
	return h
}

// tabBar is a tab bar: a row with the tab list role.
func tabBar(p part, ref *core.Ref, tabs core.Node) core.Node {
	return core.View{Ref: ref, Role: core.RoleTabList, Class: p.class, Style: core.Style{Direction: core.Row}.Merge(p.style),
		StyleFunc: p.fn, Children: []core.Node{tabs}}
}

// tab is panel key's tab in group id's bar: a press displays it, a drag
// moves it (a pointer's after 4 points, a finger's after a long press).
func (a *dockArea) tab(cx *core.Scope, g *dockGroup, group uint64, key string) core.Node {
	ref := new(core.Ref)
	g.tabs[key] = ref
	cx.Cleanup(func() {
		if g.tabs[key] == ref {
			delete(g.tabs, key)
		}
	})
	selected := func() bool {
		l := a.layout()
		_, n := regionOf(&l, key)
		if n == nil || n.Active >= len(n.Panels) || n.Panels[n.Active] != key {
			return false
		}
		if g.region == DockCenter && a.compact() {
			cur, _ := a.focus(DockCenter)
			return n.ID == cur
		}
		return true
	}
	info := DockTab{Key: key, Group: group, Selected: selected,
		Zoomed:   func() bool { return a.zoom.Get() == group && group != 0 },
		Closable: a.options(key).Closable, Close: func() { a.close(key) }}
	var label core.Node = core.Text{Content: key}
	if a.p.RenderTab != nil {
		label = a.p.RenderTab(cx, info)
	}
	desktop := core.Untracked(cx, cx.Env().Desktop)
	return core.View{Ref: ref, Role: core.RoleTab, Label: key, Class: a.p.TabClass, Style: a.p.TabStyle, StyleFunc: a.p.TabStyleFunc, Selected: selected,
		OnPress: func(core.PressEvent) { a.activate(key) },
		OnLongPress: func(core.PressEvent) {
			if !desktop {
				a.armed = key
			}
		},
		Responder: &core.Responder{
			Claims: []core.TouchClaim{{OnMove: true, Ask: func(e core.ResponderEvent) bool {
				if !a.movable(key) {
					return false
				}
				if desktop {
					d := e.Gesture.Delta
					return d.X*d.X+d.Y*d.Y >= dockDragSlop*dockDragSlop
				}
				return a.armed == key
			}}},
			Keep:        true,
			OnGrant:     func(e core.ResponderEvent) { a.startDrag(key, e.PagePosition) },
			OnMove:      func(e core.ResponderEvent) { a.moveDrag(e.PagePosition) },
			OnRelease:   func(core.ResponderEvent) { a.endDrag(true) },
			OnTerminate: func(core.ResponderEvent) { a.endDrag(false) },
		},
		Children: []core.Node{label}}
}

func (a *dockArea) startDrag(key string, at core.Point) {
	a.armed = ""
	l := a.untracked()
	region, g := regionOf(&l, key)
	if g == nil {
		return
	}
	d := &dockDrag{panel: key, region: region, group: g.ID, at: at}
	a.resolve(d)
	a.drag.Set(d)
}

func (a *dockArea) moveDrag(at core.Point) {
	old := a.drag.GetUntracked()
	if old == nil {
		return
	}
	d := *old
	d.at = at
	a.resolve(&d)
	if (d.target == nil) != (old.target == nil) || d.bounds != old.bounds {
		if !core.Untracked(a.cx, a.cx.Env().ReduceMotion) {
			// The indicator eases to its new place.
			a.cx.AnimateLayout(core.LayoutAnimation{
				Create: &core.LayoutTransition{Curve: core.LayoutEaseInEaseOut, Duration: 120 * time.Millisecond},
				Update: &core.LayoutTransition{Curve: core.LayoutEaseInEaseOut, Duration: 120 * time.Millisecond},
				Delete: &core.LayoutTransition{Curve: core.LayoutEaseInEaseOut, Duration: 120 * time.Millisecond}})
		}
	}
	a.drag.Set(&d)
}

func (a *dockArea) endDrag(drop bool) {
	d := a.drag.GetUntracked()
	a.drag.Set(nil)
	if d == nil || !drop || d.target == nil {
		return
	}
	t := *d.target
	a.edit(func(l *DockLayout) bool {
		from, to := l.tree(d.region), l.tree(t.region)
		if from == nil || to == nil {
			return false
		}
		if !from.detach(d.panel) {
			return false
		}
		if from != to {
			from.normalize(d.region == DockCenter)
		}
		return to.insert(d.panel, t, a.alloc)
	})
	if t.region == DockCenter && !t.split {
		a.current.Set(t.node)
	}
}

func contains(b core.Bounds, p core.Point) bool {
	return p.X >= b.Origin.X && p.Y >= b.Origin.Y && p.X < b.Origin.X+b.Size.Width && p.Y < b.Origin.Y+b.Size.Height
}

// resolve finds where drag d would land at its point: between the tabs
// of a tab bar it is over, or in a content frame its middle merges into
// and its outer 35% on a side splits beside, except back onto its own
// group when that changes nothing. At compact width it only reorders.
func (a *dockArea) resolve(d *dockDrag) {
	d.target, d.bounds = nil, core.Bounds{}
	l := a.untracked()
	compact := core.Untracked(a.cx, a.compact)
	ids := make([]uint64, 0, len(a.groups))
	for id := range a.groups {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		g := a.groups[id]
		n := findGroup(&l, id)
		if n == nil {
			continue
		}
		bar, okBar := g.bar.Bounds()
		content, okContent := g.content.Bounds()
		if !okBar || !okContent || bar.Size.Width == 0 && content.Size.Width == 0 {
			continue
		}
		if contains(bar, d.at) {
			// The slot between the tabs the point is past, the dragged
			// one left out.
			index := 0
			order := n.Panels
			if compact && g.region == DockCenter {
				order = l.Center.panels()
			}
			var past string
			for _, k := range order {
				if k == d.panel {
					continue
				}
				r := g.tabs[k]
				if r == nil {
					continue
				}
				if b, ok := r.Bounds(); ok && d.at.X >= b.Origin.X+b.Size.Width/2 {
					index++
					past = k
				}
			}
			t := dockTarget{region: g.region, node: id, index: index, activate: true}
			if compact && g.region == DockCenter {
				// Before the panel after past, in its group; else after
				// the last.
				t = a.compactSlot(&l, d.panel, past)
			}
			d.target, d.bounds = &t, content
			return
		}
		if compact || !contains(content, d.at) {
			continue
		}
		side := splitSide(content, d.at)
		if side == core.PlaceDefault {
			if id == d.group {
				return
			}
			d.target, d.bounds = &dockTarget{region: g.region, node: id, index: -1, activate: true}, content
			return
		}
		if id == d.group && len(n.Panels) == 1 {
			return
		}
		d.target = &dockTarget{region: g.region, node: id, split: true, side: side}
		d.bounds = placeholder(content, side)
		return
	}
}

// compactSlot is where a drop after panel past lands in a compact
// center's strip, which lists every panel of the center in tree order.
func (a *dockArea) compactSlot(l *DockLayout, panel, past string) dockTarget {
	order := slices.DeleteFunc(l.Center.panels(), func(k string) bool { return k == panel })
	at := 0
	if past != "" {
		at = slices.Index(order, past) + 1
	}
	if at < len(order) {
		next := order[at]
		g := l.Center.groupOf(next)
		i := slices.Index(slices.DeleteFunc(slices.Clone(g.Panels), func(k string) bool { return k == panel }), next)
		return dockTarget{region: DockCenter, node: g.ID, index: i, activate: true}
	}
	g := l.Center.groupOf(order[len(order)-1])
	return dockTarget{region: DockCenter, node: g.ID, index: -1, activate: true}
}

// splitSide is the side zone of b point p is in, PlaceDefault for the
// middle, as NSH's split_placement_at.
func splitSide(b core.Bounds, p core.Point) core.Placement {
	x, y := p.X-b.Origin.X, p.Y-b.Origin.Y
	w, h := b.Size.Width, b.Size.Height
	switch {
	case x < w*dockSplitZone:
		return core.PlaceLeft
	case x > w*(1-dockSplitZone):
		return core.PlaceRight
	case y < h*dockSplitZone:
		return core.PlaceTop
	case y > h*(1-dockSplitZone):
		return core.PlaceBottom
	}
	return core.PlaceDefault
}

// placeholder is the half of b a drop on side takes, as NSH's
// DropPlaceholderBounds.
func placeholder(b core.Bounds, side core.Placement) core.Bounds {
	w, h := b.Size.Width/2, b.Size.Height/2
	switch side {
	case core.PlaceLeft:
		b.Size.Width = w
	case core.PlaceRight:
		b.Origin.X += w
		b.Size.Width = w
	case core.PlaceTop:
		b.Size.Height = h
	case core.PlaceBottom:
		b.Origin.Y += h
		b.Size.Height = h
	}
	return b
}

// indicator is the drop indicator and the drag preview: overlays over
// the root view that never take the pointer.
func (a *dockArea) indicator() core.Node {
	return core.Show{When: func() bool { return a.drag.Get() != nil }, Then: func(cx *core.Scope) core.Node {
		d := func() *dockDrag {
			if d := a.drag.Get(); d != nil {
				return d
			}
			return &dockDrag{}
		}
		at := func(b core.Bounds) core.Style {
			return core.Style{Position: core.Absolute, Left: core.Pt(b.Origin.X), Top: core.Pt(b.Origin.Y),
				Width: core.Pt(b.Size.Width), Height: core.Pt(b.Size.Height)}
		}
		drop := core.Show{When: func() bool { return d().target != nil }, Then: func(cx *core.Scope) core.Node {
			var mark core.Node = core.View{Class: a.p.DropIndicatorClass,
				Style: core.Style{FlexGrow: 1}.Merge(a.p.DropIndicatorStyle), StyleFunc: a.p.DropIndicatorStyleFunc}
			if f := a.p.RenderDropIndicator; f != nil {
				t := d().target
				mark = f(cx, DockDropIndicator{Group: t.node, Side: t.side})
			}
			return core.Overlay{StyleFunc: func() core.Style { return at(d().bounds) }, PointerEvents: core.PointerEventsNone,
				Children: []core.Node{mark}}
		}}
		nodes := core.Group{drop}
		if f := a.p.RenderDragPreview; f != nil {
			first := d()
			preview := f(cx, DockTab{Key: first.panel, Group: first.group,
				Selected: func() bool { return true }, Zoomed: func() bool { return false }})
			nodes = append(nodes, core.Overlay{PointerEvents: core.PointerEventsNone,
				StyleFunc: func() core.Style {
					p := d().at
					return core.Style{Position: core.Absolute, Left: core.Pt(p.X + 8), Top: core.Pt(p.Y + 8)}
				},
				Children: []core.Node{preview}})
		}
		return nodes
	}}
}

// center is the center region: its tree, or RenderEmpty without panels.
func (a *dockArea) center(cx *core.Scope) core.Node {
	empty := func() bool { l := a.layout(); return len(l.Center.panels()) == 0 }
	return core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)},
		Children: []core.Node{core.Show{When: empty,
			Then: func(cx *core.Scope) core.Node {
				if a.p.RenderEmpty != nil {
					return a.p.RenderEmpty(cx)
				}
				return nil
			},
			Else: func(*core.Scope) core.Node { return a.root(DockCenter) }}}}
}

// dockInfo is what RenderDock gets about dock p.
func (a *dockArea) dockInfo(p DockPlacement) DockInfo {
	return DockInfo{Placement: p, Open: func() bool { return a.shown(p) },
		Toggle: func() { a.setDockOpen(p, !core.Untracked(a.cx, func() bool { return a.shown(p) })) }}
}

// dockSize is how big dock p shows: what a drag gives it, else its size,
// within what the area leaves it: at least PanelMinSize, and room for a
// center of PanelMinSize beside the opposite dock.
func (a *dockArea) dockSize(p DockPlacement) core.Points {
	l := a.layout()
	r := l.Region(p)
	if r == nil {
		return 0
	}
	size := a.live[p].Get()
	if size == 0 {
		size = r.Size
	}
	if size <= 0 {
		size = dockDefaultSize
	}
	return a.clampDock(p, max(size, PanelMinSize))
}

func (a *dockArea) clampDock(p DockPlacement, size core.Points) core.Points {
	area := a.size.Get()
	var room core.Points
	switch p {
	case DockBottom:
		room = area.Height - PanelMinSize
	default:
		other := DockRight
		if p == DockRight {
			other = DockLeft
		}
		room = area.Width - PanelMinSize
		l := a.layout()
		if r := l.Region(other); r != nil && !r.Closed {
			s := a.live[other].Get()
			if s == 0 {
				s = max(r.Size, PanelMinSize)
				if r.Size <= 0 {
					s = dockDefaultSize
				}
			}
			room -= s
		}
	}
	if area == (core.Size{}) {
		return size
	}
	return min(max(size, PanelMinSize), max(room, PanelMinSize))
}

// dock is dock p in the full presentation: its frame at its size,
// clipping, with its chrome, its tree and the handle on its inner edge.
func (a *dockArea) dock(cx *core.Scope, p DockPlacement) core.Node {
	exists := func() bool { l := a.layout(); return l.Region(p) != nil }
	return core.Show{When: exists, Then: func(cx *core.Scope) core.Node {
		style := func() core.Style {
			l := a.layout()
			r := l.Region(p)
			if r == nil || r.Closed || a.zoomedIn(p) {
				return core.Style{Display: core.DisplayNone}
			}
			s := core.Style{Overflow: core.Hidden, FlexShrink: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)}
			if a.zoom.Get() != 0 {
				// Zoomed into this dock: it takes the area.
				s.FlexGrow, s.FlexBasis = 1, core.Pt(0)
				return s
			}
			size := core.Pt(a.dockSize(p))
			if p == DockBottom {
				s.Height = size
			} else {
				s.Width = size
			}
			return s
		}
		var start core.Points
		edge, dir := HandleTrailing, Horizontal
		grow := core.Points(1)
		switch p {
		case DockRight:
			edge, grow = HandleLeading, -1
		case DockBottom:
			edge, dir, grow = HandleLeading, Vertical, -1
		}
		handle := ResizeHandle{Axis: dir, Edge: edge, Class: a.p.HandleClass, Style: a.p.HandleStyle, StyleFunc: a.p.HandleStyleFunc,
			ThumbClass: a.p.HandleThumbClass, ThumbStyle: a.p.HandleThumbStyle, ThumbStyleFunc: a.p.HandleThumbStyleFunc,
			Label:       a.p.HandleLabel,
			OnDragStart: func() { start = core.Untracked(cx, func() core.Points { return a.dockSize(p) }) },
			OnDragMove:  func(d core.Points) { a.live[p].Set(max(start+grow*d, 1)) },
			OnDragEnd: func(d core.Points) {
				size := start + grow*d
				a.live[p].Set(0)
				a.edit(func(l *DockLayout) bool {
					r := l.Region(p)
					if r == nil {
						return false
					}
					if size < dockCollapse && !r.NoCollapse {
						// Closed, keeping its size.
						r.Closed = true
						return true
					}
					next := core.Untracked(cx, func() core.Points { return a.clampDock(p, size) })
					if next == r.Size {
						return false
					}
					r.Size = next
					return true
				})
			},
			Value: func() core.AccessibilityValue {
				return core.AccessibilityValue{Min: float32(PanelMinSize), Max: float32(a.clampDock(p, PanelMaxSize)),
					Now: float32(a.dockSize(p))}
			}}
		var chrome core.Node
		if a.p.RenderDock != nil {
			chrome = a.p.RenderDock(cx, a.dockInfo(p))
		}
		own := a.p.DockStyleFunc
		return core.View{Class: a.p.DockClass, Style: a.p.DockStyle, StyleFunc: func() core.Style {
			var s core.Style
			if own != nil {
				s = own()
			}
			return s.Merge(style())
		}, Children: []core.Node{
			chrome,
			core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)},
				Children: []core.Node{a.root(p)}},
			handle,
		}}
	}}
}

// drawer is dock p in the compact presentation: a drawer from its edge,
// or a sheet at the bottom, over a backdrop, presented one at a time.
func (a *dockArea) drawer(cx *core.Scope, p DockPlacement) core.Node {
	side, panel := core.PlaceLeft, part{a.p.DrawerClass, a.p.DrawerStyle, a.p.DrawerStyleFunc}
	switch p {
	case DockRight:
		side = core.PlaceRight
	case DockBottom:
		side, panel = core.PlaceBottom, part{a.p.SheetClass, a.p.SheetStyle, a.p.SheetStyleFunc}
	}
	m := newSheetMotion(cx, func() bool {
		l := a.layout()
		return a.presented.Get() == p && l.Region(p) != nil
	}, side)
	dismiss := func() {
		if a.presented.GetUntracked() == p {
			a.presented.Set(DockCenter)
		}
	}
	var chrome core.Node
	if a.p.RenderDock != nil {
		chrome = a.p.RenderDock(cx, a.dockInfo(p))
	}
	return sheetLayer{m: m, side: side, backdrop: part{a.p.BackdropClass, a.p.BackdropStyle, a.p.BackdropStyleFunc},
		backdropPress: dismiss, cancel: dismiss, panel: panel, role: core.RoleDialog,
		content: []core.Node{chrome, core.View{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)},
			Children: []core.Node{a.root(p)}}}}.node()
}
