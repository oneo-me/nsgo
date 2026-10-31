package core

import "github.com/oneo-me/nsgo/internal/driver"

// EdgeEffect is how a scroll view's side sets the views over it, its
// bars (View.EdgeBar), apart from the content scrolling under them, as
// iOS 26's scroll edge effects do: Scroll.EdgeEffects.
//
// iOS 26 and later draw their own (UIScrollEdgeEffect), shaped under the
// bars. Elsewhere a soft edge blurs the content more toward the side, as
// far as the bars reach and a little past them, and fades it into the
// background it scrolls over; a hard edge frosts it evenly under the bars,
// with a hairline along them. Windows, Android and the web blur by a
// radius that grows toward the side; iOS before 26 and macOS by their
// system blur, fading in toward the side. Linux only fades the content
// into the background.
type EdgeEffect = driver.EdgeEffect

const (
	// EdgeEffectAutomatic is soft under the bars, and iOS 26's own under
	// its system bars.
	EdgeEffectAutomatic = driver.EdgeEffectAutomatic
	EdgeEffectSoft      = driver.EdgeEffectSoft
	EdgeEffectHard      = driver.EdgeEffectHard
	// EdgeEffectHidden draws none, under iOS's system bars too.
	EdgeEffectHidden = driver.EdgeEffectHidden
)

// EdgeEffects are a scroll view's edge effects, by side.
type EdgeEffects struct {
	Top, Right, Bottom, Left EdgeEffect
}

// EdgeBar makes a view a bar over a side of the scroll view Scroll refers
// to, which content scrolls under: a header or a toolbar placed over it.
// The scroll view's edge effect at that side reaches as far as its bars
// do, and iOS 26 shapes it under what they show
// (UIScrollEdgeElementContainerInteraction):
//
//	scroll := &core.Ref{}
//	core.View{Children: []core.Node{
//		core.Scroll{Ref: scroll, EdgeEffects: core.EdgeEffects{Top: core.EdgeEffectSoft}, ...},
//		core.View{EdgeBar: core.EdgeBar{Scroll: scroll, Side: core.SideTop},
//			Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0)}, ...},
//	}}
type EdgeBar struct {
	Scroll *Ref
	// Side is the one side it is over.
	Side Sides
}

// edgeSide returns the driver's side of one of s's sides.
func edgeSide(s Sides) (driver.Side, bool) {
	switch s {
	case SideTop:
		return driver.SideTop, true
	case SideRight:
		return driver.SideRight, true
	case SideBottom:
		return driver.SideBottom, true
	case SideLeft:
		return driver.SideLeft, true
	}
	return 0, false
}

// edgeLine is a hard edge's hairline, as macOS draws it: black at 10% in
// light mode, white at 7% in dark mode.
var edgeLine = [2]Color{RGBA(0x000000, 0.1), RGBA(0xffffff, 0.07)}

// markEdgeScrolls measures how far the bars reach into their scroll views
// and marks the scroll views whose edges changed, after layout.
func (t *Tree) markEdgeScrolls() {
	if len(t.edgeBars) == 0 && len(t.edgeScrolls) == 0 {
		return
	}
	measured := map[*node]*[4]driver.ScrollEdge{}
	var order []*node
	live := t.edgeBars[:0]
	for _, b := range t.edgeBars {
		if b.deleted {
			continue
		}
		live = append(live, b)
		s := b.edgeBar.Scroll.n
		side, ok := edgeSide(b.edgeBar.Side)
		if s == nil || s.deleted || s.scroll == nil || !ok {
			continue
		}
		sb, bb := t.surfaceBounds(s), t.surfaceBounds(b)
		var reach Points
		switch side {
		case driver.SideTop:
			reach = min(bb.Origin.Y+bb.Size.Height-sb.Origin.Y, sb.Size.Height)
		case driver.SideRight:
			reach = min(sb.Origin.X+sb.Size.Width-bb.Origin.X, sb.Size.Width)
		case driver.SideBottom:
			reach = min(sb.Origin.Y+sb.Size.Height-bb.Origin.Y, sb.Size.Height)
		case driver.SideLeft:
			reach = min(bb.Origin.X+bb.Size.Width-sb.Origin.X, sb.Size.Width)
		}
		edges := measured[s]
		if edges == nil {
			edges = &[4]driver.ScrollEdge{}
			measured[s] = edges
			order = append(order, s)
		}
		e := &edges[side]
		e.Extent = max(e.Extent, reach)
		e.Bars = append(e.Bars, b.id)
	}
	t.edgeBars = live
	line := edgeLine[0]
	if t.root.Env().dark.GetUntracked() {
		line = edgeLine[1]
	}
	for _, s := range t.edgeScrolls {
		if measured[s] == nil && !s.deleted {
			measured[s] = &[4]driver.ScrollEdge{}
			order = append(order, s)
		}
	}
	t.edgeScrolls = t.edgeScrolls[:0]
	for _, s := range order {
		edges := measured[s]
		bars := false
		for i := range edges {
			if e := &edges[i]; e.Extent > 0 {
				e.Tint, e.Line = t.scrolledOver(s), line
				bars = true
			} else {
				*e = driver.ScrollEdge{}
			}
		}
		if bars {
			t.edgeScrolls = append(t.edgeScrolls, s)
		}
		sc := s.scroll
		same := true
		for i := range edges {
			same = same && edges[i].Equal(&sc.edges[i])
		}
		if !same {
			sc.edges = *edges
			t.mark(s)
		}
	}
}

// scrolledOver returns the background scroll view n's content scrolls
// over: its own, or the nearest of its ancestors'.
func (t *Tree) scrolledOver(n *node) Color {
	for ; n != nil; n = n.nativeParent() {
		if c := n.style.Background; c != 0 {
			return c
		}
	}
	return 0
}

// scrollEdges returns the edges scroll view n sends: the effects it asks
// for, with what its bars measured.
func (sc *scrollNode) scrollEdges() [4]driver.ScrollEdge {
	edges := sc.edges
	for i, e := range [4]EdgeEffect{sc.effects.Top, sc.effects.Right, sc.effects.Bottom, sc.effects.Left} {
		edges[i].Effect = e
	}
	return edges
}
