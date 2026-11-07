package plot

import (
	"cmp"
	"math"
	"slices"
)

// The layout of a Sankey is gpui-base's generator, itself d3-sankey's,
// ported line for line. Nodes are indices; the extent is the plot's size.
// Its departures from d3 are gpui-base's: a link has a width at each end, so
// a node's sides are always covered even when its flows in and out differ or
// are square-rooted, each column is centered in the extent, and runs of
// equal single-node columns are staggered so the ribbons between them curve.

// staggerRatio is the vertical offset, as a fraction of node height, applied
// to stagger runs of equal-height single-node columns.
const staggerRatio = 0.15

// sankeyNode is a node with its computed layout.
type sankeyNode struct {
	index                int
	value                float64
	depth, height, layer int
	x0, x1, y0, y1       float32
	// sourceLinks and targetLinks are indices into the links, out and in.
	sourceLinks, targetLinks []int
}

// sankeyEdge is a link with its computed layout: y0 and y1 are the vertical
// centers of the ribbon at the source and target end, each with its own
// width; width is the nominal one the relaxation uses.
type sankeyEdge struct {
	index, source, target    int
	value                    float64
	y0, y1, width            float32
	sourceWidth, targetWidth float32
}

// sankeyGraph is the computed layout.
type sankeyGraph struct {
	nodes []sankeyNode
	links []sankeyEdge
}

func (g *sankeyGraph) layerCount() int {
	n := 0
	for i := range g.nodes {
		n = max(n, g.nodes[i].layer+1)
	}
	return n
}

// sankeyGenerator is the layout's options.
type sankeyGenerator struct {
	nodeWidth, nodePadding float32
	align                  SankeyAlign
	iterations             int
	scale                  SankeyValueScale
	x1, y1                 float32
}

// layout is the full layout of count nodes. ok is false if a link names a
// node out of range or the graph has a cycle: bad is the node named, or -1
// for a cycle.
func (s sankeyGenerator) layout(count int, links []SankeyLink) (g *sankeyGraph, bad int, ok bool) {
	for _, l := range links {
		if l.Source >= count || l.Source < 0 {
			return nil, l.Source, false
		}
		if l.Target >= count || l.Target < 0 {
			return nil, l.Target, false
		}
	}
	g = &sankeyGraph{nodes: make([]sankeyNode, count), links: make([]sankeyEdge, len(links))}
	for i := range g.nodes {
		g.nodes[i].index = i
	}
	for i, l := range links {
		value := l.Value
		if s.scale == SankeySqrt {
			value = math.Sqrt(max(value, 0))
		}
		g.links[i] = sankeyEdge{index: i, source: l.Source, target: l.Target, value: value}
	}
	if count == 0 {
		return g, 0, true
	}
	for i := range g.links {
		l := &g.links[i]
		g.nodes[l.source].sourceLinks = append(g.nodes[l.source].sourceLinks, i)
		g.nodes[l.target].targetLinks = append(g.nodes[l.target].targetLinks, i)
	}
	for i := range g.nodes {
		var out, in float64
		for _, l := range g.nodes[i].sourceLinks {
			out += g.links[l].value
		}
		for _, l := range g.nodes[i].targetLinks {
			in += g.links[l].value
		}
		g.nodes[i].value = max(out, in)
	}
	if !computeRanks(g) {
		return nil, -1, false
	}
	s.computeLayers(g)
	columns := make([][]int, g.layerCount())
	for i := range g.nodes {
		l := g.nodes[i].layer
		columns[l] = append(columns[l], i)
	}
	s.computeBreadths(g, columns)
	computeLinkBreadths(g)
	s.centerColumns(g)
	s.staggerFlatColumns(g)
	return g, 0, true
}

// computeRanks assigns the longest-path depth and height from one
// topological ordering; false means a cycle.
func computeRanks(g *sankeyGraph) bool {
	n := len(g.nodes)
	incoming := make([]int, n)
	var order []int
	for i := range g.nodes {
		incoming[i] = len(g.nodes[i].targetLinks)
		if incoming[i] == 0 {
			order = append(order, i)
		}
	}
	depths := make([]int, n)
	for visited := 0; visited < len(order); visited++ {
		index := order[visited]
		for _, l := range g.nodes[index].sourceLinks {
			target := g.links[l].target
			depths[target] = max(depths[target], depths[index]+1)
			incoming[target]--
			if incoming[target] == 0 {
				order = append(order, target)
			}
		}
	}
	if len(order) != n {
		return false
	}
	heights := make([]int, n)
	for _, index := range slices.Backward(order) {
		for _, l := range g.nodes[index].sourceLinks {
			heights[index] = max(heights[index], heights[g.links[l].target]+1)
		}
	}
	for i := range g.nodes {
		g.nodes[i].depth, g.nodes[i].height = depths[i], heights[i]
	}
	return true
}

func (s sankeyGenerator) alignLayer(g *sankeyGraph, index, n int) int {
	node := &g.nodes[index]
	switch s.align {
	case SankeyLeft:
		return node.depth
	case SankeyRight:
		return n - 1 - node.height
	case SankeyCenter:
		switch {
		case len(node.targetLinks) > 0:
			return node.depth
		case len(node.sourceLinks) > 0:
			least := math.MaxInt
			for _, l := range node.sourceLinks {
				least = min(least, g.nodes[g.links[l].target].depth)
			}
			return max(least-1, 0)
		}
		return 0
	}
	if len(node.sourceLinks) == 0 {
		return n - 1
	}
	return node.depth
}

func (s sankeyGenerator) computeLayers(g *sankeyGraph) {
	n := 0
	for i := range g.nodes {
		n = max(n, g.nodes[i].depth+1)
	}
	var kx float32
	if n > 1 {
		kx = (s.x1 - s.nodeWidth) / float32(n-1)
	}
	layers := make([]int, len(g.nodes))
	for i := range g.nodes {
		layers[i] = min(s.alignLayer(g, i, n), n-1)
	}
	for i, layer := range layers {
		node := &g.nodes[i]
		node.layer = layer
		node.x0 = float32(layer) * kx
		node.x1 = node.x0 + s.nodeWidth
	}
}

func (s sankeyGenerator) computeBreadths(g *sankeyGraph, columns [][]int) {
	longest := 0
	for _, c := range columns {
		longest = max(longest, len(c))
	}
	py := s.nodePadding
	if longest > 1 {
		py = min(py, s.y1/float32(longest-1))
	}
	s.initBreadths(g, columns, py)
	for i := range s.iterations {
		alpha := float32(math.Pow(0.99, float64(i)))
		beta := max(1-alpha, float32(i+1)/float32(s.iterations))
		s.relaxRightToLeft(g, columns, alpha, beta, py)
		s.relaxLeftToRight(g, columns, alpha, beta, py)
	}
}

func (s sankeyGenerator) initBreadths(g *sankeyGraph, columns [][]int, py float32) {
	// Scale factor between flow value and pixels, clamped to zero so heights
	// never invert in an over-crowded column.
	ky := float32(math.Inf(1))
	for _, column := range columns {
		sum := 0.0
		for _, i := range column {
			sum += g.nodes[i].value
		}
		if sum > 0 {
			ky = min(ky, (s.y1-float32(len(column)-1)*py)/float32(sum))
		}
	}
	if math.IsInf(float64(ky), 0) {
		ky = 0
	}
	ky = max(ky, 0)
	for _, column := range columns {
		y := float32(0)
		for _, i := range column {
			node := &g.nodes[i]
			height := float32(node.value) * ky
			node.y0, node.y1 = y, y+height
			y = node.y1 + py
		}
		// Distribute the leftover vertical space evenly; an over-crowded
		// column shifts nodes back up.
		leftover := (s.y1 - y + py) / float32(len(column)+1)
		for k, i := range column {
			dy := leftover * float32(k+1)
			g.nodes[i].y0 += dy
			g.nodes[i].y1 += dy
		}
	}
	for i := range g.links {
		g.links[i].width = float32(g.links[i].value) * ky
	}
	for _, column := range columns {
		for _, i := range column {
			sortSourceLinks(g, i)
			sortTargetLinks(g, i)
		}
	}
}

// relaxLeftToRight repositions each node based on its incoming links.
func (s sankeyGenerator) relaxLeftToRight(g *sankeyGraph, columns [][]int, alpha, beta, py float32) {
	for c := 1; c < len(columns); c++ {
		column := columns[c]
		for _, target := range column {
			var y, w float32
			for _, l := range g.nodes[target].targetLinks {
				link := &g.links[l]
				v := float32(link.value) * float32(g.nodes[target].layer-g.nodes[link.source].layer)
				y += targetTop(g, link.source, target, py) * v
				w += v
			}
			if w <= 0 {
				continue
			}
			dy := (y/w - g.nodes[target].y0) * alpha
			g.nodes[target].y0 += dy
			g.nodes[target].y1 += dy
			reorderLinks(g, target)
		}
		sortColumn(g, column)
		s.resolveCollisions(g, column, beta, py)
	}
}

// relaxRightToLeft repositions each node based on its outgoing links.
func (s sankeyGenerator) relaxRightToLeft(g *sankeyGraph, columns [][]int, alpha, beta, py float32) {
	for c := len(columns) - 2; c >= 0; c-- {
		column := columns[c]
		for _, source := range column {
			var y, w float32
			for _, l := range g.nodes[source].sourceLinks {
				link := &g.links[l]
				v := float32(link.value) * float32(g.nodes[link.target].layer-g.nodes[source].layer)
				y += sourceTop(g, source, link.target, py) * v
				w += v
			}
			if w <= 0 {
				continue
			}
			dy := (y/w - g.nodes[source].y0) * alpha
			g.nodes[source].y0 += dy
			g.nodes[source].y1 += dy
			reorderLinks(g, source)
		}
		sortColumn(g, column)
		s.resolveCollisions(g, column, beta, py)
	}
}

// resolveCollisions is d3's middle-out collision resolution: push nodes away
// from the middle node, then clamp the column against the extent's edges.
func (s sankeyGenerator) resolveCollisions(g *sankeyGraph, column []int, beta, py float32) {
	if len(column) == 0 {
		return
	}
	i := len(column) >> 1
	y0, y1 := g.nodes[column[i]].y0, g.nodes[column[i]].y1
	pushUp(g, column[:i], y0-py, beta, py)
	pushDown(g, column[i+1:], y1+py, beta, py)
	pushUp(g, column, s.y1, beta, py)
	pushDown(g, column, 0, beta, py)
}

func pushDown(g *sankeyGraph, column []int, y, alpha, py float32) {
	for _, i := range column {
		node := &g.nodes[i]
		if dy := (y - node.y0) * alpha; dy > 1e-6 {
			node.y0 += dy
			node.y1 += dy
		}
		y = node.y1 + py
	}
}

func pushUp(g *sankeyGraph, column []int, y, alpha, py float32) {
	for _, i := range slices.Backward(column) {
		node := &g.nodes[i]
		if dy := (node.y1 - y) * alpha; dy > 1e-6 {
			node.y0 -= dy
			node.y1 -= dy
		}
		y = node.y0 - py
	}
}

// sortSourceLinks sorts a node's outgoing links by the target node's y0,
// then link index.
func sortSourceLinks(g *sankeyGraph, index int) {
	slices.SortFunc(g.nodes[index].sourceLinks, func(a, b int) int {
		return cmp.Or(cmp.Compare(g.nodes[g.links[a].target].y0, g.nodes[g.links[b].target].y0), cmp.Compare(a, b))
	})
}

// sortTargetLinks sorts a node's incoming links by the source node's y0,
// then link index.
func sortTargetLinks(g *sankeyGraph, index int) {
	slices.SortFunc(g.nodes[index].targetLinks, func(a, b int) int {
		return cmp.Or(cmp.Compare(g.nodes[g.links[a].source].y0, g.nodes[g.links[b].source].y0), cmp.Compare(a, b))
	})
}

// reorderLinks re-sorts, after a node moved, the link lists of its
// neighbours on the opposite ends.
func reorderLinks(g *sankeyGraph, index int) {
	for _, l := range g.nodes[index].targetLinks {
		sortSourceLinks(g, g.links[l].source)
	}
	for _, l := range g.nodes[index].sourceLinks {
		sortTargetLinks(g, g.links[l].target)
	}
}

func sortColumn(g *sankeyGraph, column []int) {
	slices.SortStableFunc(column, func(a, b int) int { return cmp.Compare(g.nodes[a].y0, g.nodes[b].y0) })
}

// targetTop is the ideal y0 for target so that its ribbon from source lines
// up with the slot the ribbon occupies in the source's outgoing stack.
func targetTop(g *sankeyGraph, source, target int, py float32) float32 {
	node := &g.nodes[source]
	y := node.y0 - float32(max(len(node.sourceLinks)-1, 0))*py/2
	for _, l := range node.sourceLinks {
		if g.links[l].target == target {
			break
		}
		y += g.links[l].width + py
	}
	for _, l := range g.nodes[target].targetLinks {
		if g.links[l].source == source {
			break
		}
		y -= g.links[l].width
	}
	return y
}

// sourceTop is the ideal y0 for source so that its ribbon to target lines up
// with the slot the ribbon occupies in the target's incoming stack.
func sourceTop(g *sankeyGraph, source, target int, py float32) float32 {
	node := &g.nodes[target]
	y := node.y0 - float32(max(len(node.targetLinks)-1, 0))*py/2
	for _, l := range node.targetLinks {
		if g.links[l].source == source {
			break
		}
		y += g.links[l].width + py
	}
	for _, l := range g.nodes[source].sourceLinks {
		if g.links[l].target == target {
			break
		}
		y -= g.links[l].width
	}
	return y
}

// computeLinkBreadths assigns each link's centers and per-end widths by
// stacking the sorted link lists within each node: each side shares the
// node's height in proportion to the links' values, so both sides of a node
// are fully covered even when the graph is imbalanced.
func computeLinkBreadths(g *sankeyGraph) {
	for i := range g.nodes {
		node := &g.nodes[i]
		height := node.y1 - node.y0
		var out, in float64
		for _, l := range node.sourceLinks {
			out += g.links[l].value
		}
		for _, l := range node.targetLinks {
			in += g.links[l].value
		}
		y := node.y0
		for _, l := range node.sourceLinks {
			var w float32
			if out > 0 {
				w = float32(g.links[l].value/out) * height
			}
			g.links[l].sourceWidth = w
			g.links[l].y0 = y + w/2
			y += w
		}
		y = node.y0
		for _, l := range node.targetLinks {
			var w float32
			if in > 0 {
				w = float32(g.links[l].value/in) * height
			}
			g.links[l].targetWidth = w
			g.links[l].y1 = y + w/2
			y += w
		}
	}
}

// offsetLayers shifts every node and its attached link ends by its layer's
// offset.
func offsetLayers(g *sankeyGraph, offsets []float32) {
	for i := range g.nodes {
		dy := offsets[g.nodes[i].layer]
		g.nodes[i].y0 += dy
		g.nodes[i].y1 += dy
	}
	for i := range g.links {
		l := &g.links[i]
		l.y0 += offsets[g.nodes[l.source].layer]
		l.y1 += offsets[g.nodes[l.target].layer]
	}
}

// centerColumns vertically centers each column's stack of nodes within the
// extent, translating it, so a crowded column's small scale leaves the trunk
// balanced rather than high.
func (s sankeyGenerator) centerColumns(g *sankeyGraph) {
	layers := g.layerCount()
	lo, hi := make([]float32, layers), make([]float32, layers)
	for l := range lo {
		lo[l], hi[l] = float32(math.Inf(1)), float32(math.Inf(-1))
	}
	for i := range g.nodes {
		n := &g.nodes[i]
		lo[n.layer], hi[n.layer] = min(lo[n.layer], n.y0), max(hi[n.layer], n.y1)
	}
	offsets := make([]float32, layers)
	for l := range offsets {
		if !math.IsInf(float64(lo[l]), 0) && hi[l] > lo[l] {
			offsets[l] = (s.y1 - lo[l] - hi[l]) / 2
		}
	}
	offsetLayers(g, offsets)
}

// staggerFlatColumns nudges runs of adjacent single-node columns of
// near-equal height off the shared center line: centering aligns them
// exactly, so the ribbon between them is a flat rectangle; a small
// alternating stagger turns it into a gentle S-curve.
func (s sankeyGenerator) staggerFlatColumns(g *sankeyGraph) {
	layers := g.layerCount()
	if layers < 2 {
		return
	}
	count, single := make([]int, layers), make([]int, layers)
	for i := range g.nodes {
		count[g.nodes[i].layer]++
		single[g.nodes[i].layer] = i
	}
	heights := make([]float32, layers)
	for l := range heights {
		if count[l] == 1 {
			heights[l] = g.nodes[single[l]].y1 - g.nodes[single[l]].y0
		}
	}
	// Offset the odd columns of each flat run, so consecutive ribbons bend
	// down then back up.
	offsets := make([]float32, layers)
	run := 0
	for l := 1; l < layers; l++ {
		if count[l] == 1 && count[l-1] == 1 && math.Abs(float64(heights[l]-heights[l-1])) < 1e-3 {
			run++
			if run%2 == 1 {
				// Bound the nudge by the slack so the node stays in the extent.
				offsets[l] = min(heights[l]*staggerRatio, max(s.y1-heights[l], 0)/2)
			}
		} else {
			run = 0
		}
	}
	offsetLayers(g, offsets)
}

// sankeyRibbon is the outline of a link's ribbon, as d3-sankey's
// sankeyLinkHorizontal as a filled shape: from the source node's right edge
// to the target's left edge, its edges horizontal cubics with their control
// points at the horizontal middle, each end as wide as the link's width
// there, at least the least width.
type sankeyRibbon struct {
	sx, sTop, sBottom, tx, tTop, tBottom float32
}

func ribbonOf(g *sankeyGraph, l *sankeyEdge, least float32) sankeyRibbon {
	s, t := max(l.sourceWidth, least)/2, max(l.targetWidth, least)/2
	return sankeyRibbon{g.nodes[l.source].x1, l.y0 - s, l.y0 + s, g.nodes[l.target].x0, l.y1 - t, l.y1 + t}
}

func (r sankeyRibbon) mx() float32 { return (r.sx + r.tx) / 2 }

// thin is the same ribbon at its least width, each end around its middle.
func (r sankeyRibbon) thin(least float32) sankeyRibbon {
	half := max(least, 0) / 2
	s, t := (r.sTop+r.sBottom)/2, (r.tTop+r.tBottom)/2
	r.sTop, r.sBottom, r.tTop, r.tBottom = s-half, s+half, t-half, t+half
	return r
}
