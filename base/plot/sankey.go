package plot

import (
	"log"
	"math"
	"slices"

	"github.com/nsgo-dev/nsgo/core"
)

// SankeyAlign is which column a Sankey's node goes in, as d3-sankey's
// nodeAlign.
type SankeyAlign uint8

const (
	// SankeyJustify is its depth, and a node with no links out in the last
	// column.
	SankeyJustify SankeyAlign = iota
	// SankeyLeft is its depth: the longest path from a node with no links
	// in.
	SankeyLeft
	// SankeyRight is as far right as its height lets it.
	SankeyRight
	// SankeyCenter is its depth, or for a node with only links out, the
	// column before the nearest of its targets.
	SankeyCenter
)

// SankeyValueScale is how a Sankey's values map to its nodes' heights and
// its ribbons' widths.
type SankeyValueScale uint8

const (
	// SankeyLinear is in proportion to the value.
	SankeyLinear SankeyValueScale = iota
	// SankeySqrt is in proportion to its square root: a wide range of values
	// compressed, so a dominant flow does not dwarf the small ones.
	SankeySqrt
)

// SankeyLink is a flow of a Sankey: from the plot's row Source to its row
// Target, by their indices, Value its size. A value that is not a number, or
// below zero, carries nothing.
type SankeyLink struct {
	Source, Target int
	Value          float64
}

// Sankey is flows between a Plot's rows, as gpui-base's Sankey layout drawn
// as gpui-component's SankeyChart draws it: the rows are its nodes and Links
// the flows between them, laid out by d3-sankey's algorithm in the plot's
// size: a node per row, in columns by NodeAlign, as tall as the most that
// flows through it, and a ribbon per link from its source's right edge to
// its target's left edge, as wide at each end as its share of that node's
// side, at least MinLinkWidth. Each node and each ribbon is a canvas of its
// own, so each paints with its own classes and styles over the sankey's, as
// CSS inherits Fill and Stroke: Fill, the text color where none is set, and
// Stroke, none. The ribbons draw under the nodes, and both from the leading
// edge with the plot's appear; NodeLabel and ValueLabel label the nodes
// beside them, fading in with it. Over a sankey, the plot reports the node
// under the pointer, and the links not attached to it fade by HoverDim; the
// keys step through the nodes. When the app changes the flows, each node and
// ribbon moves from its old place to its new one with Motion.Update, a new
// node growing from its middle and a new ribbon from no width; links are
// told apart by their source and target, so reordering them moves nothing.
// Links that make a cycle or name a row the plot does not have log a warning
// and draw nothing. Its values take no part in the plot's scales.
type Sankey struct {
	Links func() []SankeyLink
	// NodeWidth is the width of each node, 24 without it; NodePadding the
	// gap between the nodes of a column, 8 without it, less in a column too
	// crowded for it.
	NodeWidth, NodePadding core.Points
	NodeAlign              SankeyAlign
	// Iterations is how many times the layout relaxes the nodes toward their
	// flows, 6 without it.
	Iterations int
	ValueScale SankeyValueScale
	// MinLinkWidth is the thinnest a ribbon draws, 1 without it, so a small
	// flow stays visible.
	MinLinkWidth core.Points
	// NodeCornerRadius rounds each node's corners.
	NodeCornerRadius core.Points
	// NodeStyle gives every node its Fill and Stroke, and NodeStyleOf and
	// NodeClassOf a node of a row its own, over them.
	NodeClass   string
	NodeClassOf func(row int) string
	NodeStyle   core.Style
	NodeStyleOf func(row int) core.Style
	// LinkStyle, LinkStyleOf and LinkClassOf are a ribbon's, by its index in
	// Links.
	LinkClass   string
	LinkClassOf func(link int) string
	LinkStyle   core.Style
	LinkStyleOf func(link int) core.Style
	// LinkBackgroundStyle's Background and Gradients fill a box from each
	// ribbon's source to its target, shown only inside the ribbon, under
	// its Fill, as an Area's; LinkBackgroundStyleOf gives each its own, its
	// source's color to its target's. A ribbon's Fill is then none where
	// none is set.
	LinkBackgroundClass   string
	LinkBackgroundClassOf func(link int) string
	LinkBackgroundStyle   core.Style
	LinkBackgroundStyleOf func(link int) core.Style
	// HoverDim is how much the links not attached to the node under the
	// pointer fade, as a share of their opacity, on the plot's pointer
	// spring and with the hover's fade.
	HoverDim float32
	// NodeLabel is the name beside each node, the last line of its label.
	NodeLabel func() []string
	// ValueLabel is the value above its name, of the node's row and its
	// throughput: the most that flows into or out of it, in the links'
	// values, whatever the ValueScale.
	ValueLabel func(row int, value float64) string
	// NodeLabelStyle styles each node's label: a box holding the value and
	// the name, left of a node in the first column, right of one in the last
	// and above one between them, kept between the plot's top and bottom
	// beside the node; its padding is the gap to the node.
	NodeLabelClass     string
	NodeLabelStyle     core.Style
	NodeLabelStyleFunc func() core.Style
	// ValueLabelStyle styles the value line, over the label's text styles.
	ValueLabelClass     string
	ValueLabelStyle     core.Style
	ValueLabelStyleFunc func() core.Style
	Class               string
	Style               core.Style
	StyleFunc           func() core.Style
	TestID              string
}

// linkKey is a link's key, kept when the app reorders Links: its source, its
// target, and how many links between the two come before it.
type linkKey struct{ source, target, nth int }

// linkKeys is the keys of g's links that carry something.
func linkKeys(g *sankeyGraph) []linkKey {
	seen := map[[2]int]int{}
	var keys []linkKey
	for i := range g.links {
		l := &g.links[i]
		pair := [2]int{l.source, l.target}
		if l.value > 0 {
			keys = append(keys, linkKey{l.source, l.target, seen[pair]})
		}
		seen[pair]++
	}
	return keys
}

// findLink is the link of g key names.
func findLink(g *sankeyGraph, key linkKey) *sankeyEdge {
	nth := 0
	for i := range g.links {
		l := &g.links[i]
		if l.source == key.source && l.target == key.target {
			if nth == key.nth {
				return l
			}
			nth++
		}
	}
	return nil
}

// throughputs is how much flows through each of count nodes: the most into
// or out of it, in the links' own values.
func throughputs(count int, links []SankeyLink) []float64 {
	in, out := make([]float64, count), make([]float64, count)
	for _, l := range links {
		if l.Source >= 0 && l.Source < count && l.Target >= 0 && l.Target < count {
			out[l.Source] += l.Value
			in[l.Target] += l.Value
		}
	}
	for i := range in {
		in[i] = max(in[i], out[i])
	}
	return in
}

// sankeySide is where a node's label goes.
type sankeySide uint8

const (
	sideBefore sankeySide = iota
	sideAfter
	sideAbove
)

// rect is a box, x0, y0, width and height.
type rect [4]float32

// anchor is the point a label on this side of r hangs from.
func (s sankeySide) anchor(r rect) (float32, float32) {
	switch s {
	case sideBefore:
		return r[0], r[1] + r[3]/2
	case sideAfter:
		return r[0] + r[2], r[1] + r[3]/2
	}
	return r[0] + r[2]/2, r[1]
}

// sankeyNodePose is a node as drawn: its rectangle, at least a point tall,
// and the side its label goes on.
type sankeyNodePose struct {
	rect rect
	side sankeySide
}

type keyedRibbon struct {
	key    linkKey
	ribbon sankeyRibbon
}

// sankeyPose is what a sankey draws: its nodes by row and its ribbons by
// link key, the thinnest a ribbon draws with them.
type sankeyPose struct {
	nodes   []sankeyNodePose
	ribbons []keyedRibbon
	least   float32
}

func poseOf(g *sankeyGraph, least float32) sankeyPose {
	last := max(g.layerCount()-1, 0)
	p := sankeyPose{nodes: make([]sankeyNodePose, len(g.nodes)), least: least}
	for i := range g.nodes {
		n := &g.nodes[i]
		side := sideAbove
		switch n.layer {
		case 0:
			side = sideBefore
		case last:
			side = sideAfter
		}
		p.nodes[i] = sankeyNodePose{rect{n.x0, n.y0, n.x1 - n.x0, max(n.y1-n.y0, 1)}, side}
	}
	for _, key := range linkKeys(g) {
		if l := findLink(g, key); l != nil {
			p.ribbons = append(p.ribbons, keyedRibbon{key, ribbonOf(g, l, least)})
		}
	}
	return p
}

func (p sankeyPose) same(o sankeyPose) bool {
	return p.least == o.least && slices.Equal(p.nodes, o.nodes) && slices.Equal(p.ribbons, o.ribbons)
}

func (p sankeyPose) ribbon(key linkKey) (sankeyRibbon, bool) {
	for _, r := range p.ribbons {
		if r.key == key {
			return r.ribbon, true
		}
	}
	return sankeyRibbon{}, false
}

// nodeFrom is where node i moves from when this pose moves to one where it
// is to: where it is here, or for a node new to it, to with no height at its
// middle.
func (p sankeyPose) nodeFrom(i int, to rect) rect {
	if i < len(p.nodes) {
		return p.nodes[i].rect
	}
	return rect{to[0], to[1] + to[3]/2, to[2], 0}
}

// ribbonFrom is where a ribbon moves from: where it is here, or for a link
// new to it, to at its least width.
func (p sankeyPose) ribbonFrom(key linkKey, to sankeyRibbon, least float32) sankeyRibbon {
	if r, ok := p.ribbon(key); ok {
		return r
	}
	return to.thin(least)
}

func (p sankeyPose) lerp(to sankeyPose, t float32) sankeyPose {
	at := func(a, b float32) float32 { return a + (b-a)*t }
	out := sankeyPose{nodes: make([]sankeyNodePose, len(to.nodes)), ribbons: make([]keyedRibbon, len(to.ribbons)), least: to.least}
	for i, n := range to.nodes {
		a, b := p.nodeFrom(i, n.rect), n.rect
		out.nodes[i] = sankeyNodePose{rect{at(a[0], b[0]), at(a[1], b[1]), at(a[2], b[2]), at(a[3], b[3])}, n.side}
	}
	for i, r := range to.ribbons {
		a, b := p.ribbonFrom(r.key, r.ribbon, to.least), r.ribbon
		out.ribbons[i] = keyedRibbon{r.key, sankeyRibbon{at(a.sx, b.sx), at(a.sTop, b.sTop), at(a.sBottom, b.sBottom),
			at(a.tx, b.tx), at(a.tTop, b.tTop), at(a.tBottom, b.tBottom)}}
	}
	return out
}

// traceRibbon records to's outline as a new path, moving from from while t
// runs.
func traceRibbon(d *core.Drawing, to, from sankeyRibbon, t core.AnimatedNode) {
	n := func(a, b float32) num { return lerpNum(t, a, b) }
	if t == nil {
		from = to
	}
	sx, sTop, sBottom := n(from.sx, to.sx), n(from.sTop, to.sTop), n(from.sBottom, to.sBottom)
	tx, tTop, tBottom := n(from.tx, to.tx), n(from.tTop, to.tTop), n(from.tBottom, to.tBottom)
	mx := n(from.mx(), to.mx())
	d.BeginPath()
	moveTo(d, sx, sTop)
	cubicTo(d, mx, sTop, mx, tTop, tx, tTop)
	lineTo(d, tx, tBottom)
	cubicTo(d, mx, tBottom, mx, sBottom, sx, sBottom)
	d.ClosePath()
}

func painted(d *core.Drawing, fill core.Paint) {
	d.FillFromView(fill).StrokeFromView(core.PaintNone).LineWidthFromView().LineDashFromView()
}

// sankeyProblem is what stops a sankey from drawing, told once while it
// stays.
type sankeyProblem struct {
	set bool
	bad int
}

// Render implements core.Node.
func (s Sankey) Render(cx *core.Scope) core.Node {
	own := part{Class: s.Class, Style: s.Style, Fn: s.StyleFunc}
	plot := plotOf(cx, "Sankey")
	if plot == nil {
		return markLayer(own, s.TestID)
	}
	orDefault := func(v, other core.Points) float32 {
		if v == 0 {
			return float32(other)
		}
		return float32(max(v, 0))
	}
	links := func() []SankeyLink {
		if s.Links == nil {
			return nil
		}
		out := slices.Clone(s.Links())
		for i := range out {
			if !isFinite(out[i].Value) || out[i].Value < 0 {
				out[i].Value = 0
			}
		}
		return out
	}
	var warned sankeyProblem
	graph := core.NewMemoFunc(cx, func() *sankeyGraph {
		if !plot.sized() {
			return nil
		}
		size := plot.size.Get()
		iterations := s.Iterations
		if iterations == 0 {
			iterations = 6
		}
		count := plot.rows()
		g, bad, ok := sankeyGenerator{orDefault(s.NodeWidth, 24), orDefault(s.NodePadding, 8), s.NodeAlign, iterations, s.ValueScale,
			float32(size.Width), float32(size.Height)}.layout(count, links())
		if ok {
			warned = sankeyProblem{}
			return g
		}
		if p := (sankeyProblem{true, bad}); warned != p {
			warned = p
			if bad < 0 {
				log.Printf("base: Sankey draws nothing: its links make a cycle")
			} else {
				log.Printf("base: Sankey draws nothing: a link names row %d, and the plot has %d", bad, count)
			}
		}
		return nil
	}, nil)
	least := orDefault(s.MinLinkWidth, 1)
	moves := newMoves(cx, plot, true, func() (sankeyPose, bool) {
		g := graph.Get()
		if g == nil {
			return sankeyPose{}, false
		}
		return poseOf(g, least), true
	})
	nodeRect := func(n *sankeyNode) rect { return rect{n.x0, n.y0, n.x1 - n.x0, max(n.y1-n.y0, 1)} }
	// The pointer over a node finds its row; the keys go to the middle of
	// each node with something through it.
	plot.addHit(cx, &plotHit{
		at: func(at core.Point) (int, bool) {
			g := graph.GetUntracked()
			if g == nil {
				return 0, false
			}
			x, y := float32(at.X), float32(at.Y)
			for i := range g.nodes {
				if r := nodeRect(&g.nodes[i]); x >= r[0] && x <= r[0]+r[2] && y >= r[1] && y <= r[1]+r[3] {
					return i, true
				}
			}
			return 0, false
		},
		center: func(row int) (core.Point, bool) {
			g := graph.GetUntracked()
			if g == nil || row >= len(g.nodes) || g.nodes[row].value <= 0 {
				return core.Point{}, false
			}
			n := &g.nodes[row]
			return core.Point{X: core.Points((n.x0 + n.x1) / 2), Y: core.Points((n.y0 + n.y1) / 2)}, true
		},
	})
	keys := func() []linkKey {
		if g := graph.Get(); g != nil {
			return linkKeys(g)
		}
		return nil
	}
	nodeRows := func() []int {
		g := graph.Get()
		if g == nil {
			return nil
		}
		out := make([]int, len(g.nodes))
		for i := range out {
			out[i] = i
		}
		return out
	}
	bleed := func() float32 { return strokeRoom + float32(math.Ceil(float64(least/2))) }
	ribbons := func(key linkKey) (to, from sankeyRibbon, t core.AnimatedNode, ok bool) {
		m := moves.now()
		if !m.has {
			return
		}
		if to, ok = m.to.ribbon(key); ok && m.moving {
			from, t = m.from.ribbonFrom(key, to, m.to.least), moves.progress
		}
		return
	}
	linkIndex := func(key linkKey) int {
		if g := graph.Get(); g != nil {
			if l := findLink(g, key); l != nil {
				return l.index
			}
		}
		return -1
	}
	dim := min(max(s.HoverDim, 0), 1)
	// A link's parts fade with the hover: how far it is away from the
	// hovered node follows the plot's pointer spring, kept while the hover
	// fades out.
	dimmed := func(cx *core.Scope, key linkKey) *core.AnimatedStyle {
		if dim <= 0 || plot.noHover {
			return nil
		}
		away := core.NewAnimatedValue(cx, 0)
		target := float32(0)
		cx.Effect(func() {
			node := plot.hovered.Get()
			if node < 0 {
				return
			}
			to := float32(1)
			if key.source == node || key.target == node {
				to = 0
			}
			if target != to {
				target = to
				plot.springTo(away, to, !plot.still())
			}
		})
		return &core.AnimatedStyle{Opacity: core.Sub(core.Constant(1), core.Mul(core.Mul(away, plot.fade), core.Constant(dim)))}
	}
	hasBackground := s.LinkBackgroundClass != "" || s.LinkBackgroundClassOf != nil || !s.LinkBackgroundStyle.IsZero() || s.LinkBackgroundStyleOf != nil
	linkPart := func(cx *core.Scope, key linkKey, className string, base core.Style, class func(int) string, fn func(int) core.Style) (part, func() string) {
		base.Animated = dimmed(cx, key)
		p := part{Class: className, Style: base}
		if fn != nil {
			p.Fn = func() core.Style {
				if i := linkIndex(key); i >= 0 {
					return fn(i)
				}
				return core.Style{}
			}
		}
		var classOf func() string
		if class != nil {
			classOf = func() string {
				if i := linkIndex(key); i >= 0 {
					return class(i)
				}
				return ""
			}
		}
		return p, classOf
	}
	id := func(k linkKey) linkKey { return k }
	var children []core.Node
	if hasBackground {
		children = append(children, core.Each(keys, id, func(cx *core.Scope, item func() linkKey, _ func() int) core.Node {
			key := item()
			p, class := linkPart(cx, key, s.LinkBackgroundClass, s.LinkBackgroundStyle, s.LinkBackgroundClassOf, s.LinkBackgroundStyleOf)
			canvas := markCanvas(plot, bleed, p, func(d *core.Drawing, plot *plotState, b float32) {
				if plot == nil {
					nothing(d)
					return
				}
				to, from, t, ok := ribbons(key)
				if !ok {
					nothing(d)
					return
				}
				plot.clip(d, b)
				traceRibbon(d, to, from, t)
				top, bottom := min(to.sTop, to.tTop), max(to.sBottom, to.tBottom)
				d.FillBackgroundFromView(core.Bounds{Origin: core.Point{X: core.Points(to.sx), Y: core.Points(top)},
					Size: core.Size{Width: core.Points(to.tx - to.sx), Height: core.Points(bottom - top)}})
			})
			canvas.ClassFunc = class
			return canvas
		}))
	}
	children = append(children, core.Each(keys, id, func(cx *core.Scope, item func() linkKey, _ func() int) core.Node {
		key := item()
		p, class := linkPart(cx, key, s.LinkClass, s.LinkStyle, s.LinkClassOf, s.LinkStyleOf)
		canvas := markCanvas(plot, bleed, p, func(d *core.Drawing, plot *plotState, b float32) {
			initial := core.PaintCurrentColor
			if hasBackground {
				initial = core.PaintNone
			}
			painted(d, initial)
			if plot == nil {
				return
			}
			if to, from, t, ok := ribbons(key); ok {
				plot.clip(d, b)
				traceRibbon(d, to, from, t)
				d.Fill()
				d.Stroke()
			}
		})
		canvas.ClassFunc = class
		return canvas
	}))
	children = append(children, core.Each(nodeRows, func(i int) int { return i }, func(cx *core.Scope, item func() int, _ func() int) core.Node {
		row := item()
		var fn func() core.Style
		if s.NodeStyleOf != nil {
			fn = func() core.Style { return s.NodeStyleOf(row) }
		}
		canvas := markCanvas(plot, room, part{Class: s.NodeClass, Style: s.NodeStyle, Fn: fn}, func(d *core.Drawing, plot *plotState, b float32) {
			painted(d, core.PaintCurrentColor)
			if plot == nil {
				return
			}
			m := moves.now()
			if !m.has || row >= len(m.to.nodes) {
				return
			}
			to := m.to.nodes[row].rect
			from, t := to, core.AnimatedNode(nil)
			if m.moving {
				from, t = m.from.nodeFrom(row, to), moves.progress
			}
			plot.clip(d, b)
			d.BeginPath()
			rectPath(d, lerpNum(t, from[0], to[0]), lerpNum(t, from[1], to[1]), lerpNum(t, from[2], to[2]), lerpNum(t, from[3], to[3]),
				AllCorners(s.NodeCornerRadius))
			d.Fill()
			d.Stroke()
		})
		if s.NodeClassOf != nil {
			canvas.ClassFunc = func() string { return s.NodeClassOf(row) }
		}
		return canvas
	}))
	if s.NodeLabel != nil || s.ValueLabel != nil {
		raw := core.NewMemoFunc(cx, func() []float64 { return throughputs(plot.rows(), links()) }, slices.Equal)
		children = append(children, core.Each(nodeRows, func(i int) int { return i }, func(cx *core.Scope, item func() int, _ func() int) core.Node {
			return s.nodeLabel(cx, plot, moves, raw, item())
		}))
	}
	return markLayer(own, s.TestID, children...)
}

// nodeLabel is row's label: its place beside the node, and the box of its
// lines. It rides its node through Motion.Update and fades in with the
// plot's appear. It takes no pointer.
func (s Sankey) nodeLabel(cx *core.Scope, plot *plotState, moves *moves[sankeyPose], raw *core.Memo[[]float64], row int) core.Node {
	type place struct {
		side sankeySide
		x, y float32
		ok   bool
	}
	at := core.NewMemo(cx, func() place {
		m := moves.now()
		if !m.has || row >= len(m.to.nodes) {
			return place{}
		}
		n := m.to.nodes[row]
		x, y := n.side.anchor(n.rect)
		return place{n.side, x, y, true}
	})
	// How far it is from its place while its node moves there.
	awayX, awayY := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	cx.Effect(func() {
		m := moves.now()
		var dx, dy float32
		if m.moving && row < len(m.to.nodes) {
			n := m.to.nodes[row]
			x, y := n.side.anchor(m.from.nodeFrom(row, n.rect))
			toX, toY := n.side.anchor(n.rect)
			dx, dy = x-toX, y-toY
		}
		awayX.Set(float64(dx))
		awayY.Set(float64(dy))
	})
	left := moves.left()
	height := core.NewSignal(cx, core.Size{})
	var lines []core.Node
	if s.ValueLabel != nil {
		lines = append(lines, core.Text{ContentFunc: func() string {
			v := 0.0
			if all := raw.Get(); row < len(all) {
				v = all[row]
			}
			return s.ValueLabel(row, v)
		}, Class: s.ValueLabelClass, Style: s.ValueLabelStyle, StyleFunc: s.ValueLabelStyleFunc})
	}
	if s.NodeLabel != nil {
		lines = append(lines, core.Text{ContentFunc: func() string { return textAt(s.NodeLabel, row) }})
	}
	content := core.View{Class: s.NodeLabelClass, Style: s.NodeLabelStyle, OnLayout: measure(height),
		StyleFunc: func() core.Style {
			align := core.AlignStart
			switch at.Get().side {
			case sideBefore:
				align = core.AlignEnd
			case sideAbove:
				align = core.AlignCenter
			}
			out := core.Style{AlignItems: align}
			if s.NodeLabelStyleFunc != nil {
				out = out.Merge(s.NodeLabelStyleFunc())
			}
			return out
		}, Children: lines}
	return core.View{
		Style: core.Style{Position: core.Absolute, Width: pts(0), Height: pts(0),
			Animated: &core.AnimatedStyle{TranslateX: core.Mul(left, awayX), TranslateY: core.Mul(left, awayY), Opacity: plot.appear.progress()}},
		StyleFunc: func() core.Style {
			p := at.Get()
			if !p.ok {
				return core.Style{}
			}
			y := p.y
			// Beside a node, its middle on the node's, kept inside the plot.
			if p.side != sideAbove {
				half := float32(height.Get().Height) / 2
				y = max(min(y, float32(plot.size.Get().Height)-half), half)
			}
			out := core.Style{Left: pts(p.x), Top: pts(y), Direction: core.Row}
			// Always a row, so the content takes its own width.
			switch p.side {
			case sideBefore:
				out.JustifyContent, out.AlignItems = core.JustifyEnd, core.AlignCenter
			case sideAfter:
				out.JustifyContent, out.AlignItems = core.JustifyStart, core.AlignCenter
			default:
				out.JustifyContent, out.AlignItems = core.JustifyCenter, core.AlignEnd
			}
			return out
		},
		PointerEvents: core.PointerEventsNone,
		Children:      []core.Node{content},
	}
}
