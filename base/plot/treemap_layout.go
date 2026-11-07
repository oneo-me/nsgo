package plot

import (
	"cmp"
	"math"
	"slices"
)

// The layout of a Treemap: the plot's rows made a tree by their ids and
// parents, as d3's stratify, the leaves' values summed up it, and each level
// tiled into its parent's box as d3's squarified treemap with its paddings.

// treeProblem is what the app's rows got wrong, told once while it stays.
type treeProblem struct {
	kind treeProblemKind
	row  int
	id   string
}

type treeProblemKind uint8

const (
	// problemDuplicate: a row has the id of an earlier row; it names no
	// parent.
	problemDuplicate treeProblemKind = iota
	// problemUnknownParent: a row names a parent no row has: it is a
	// top-level node.
	problemUnknownParent
	// problemCycle: a row is its own ancestor: neither it nor what is under
	// it is drawn.
	problemCycle
)

// treemapTree is the rows as a tree: each row's children, in the order the
// app wrote them, and its value, its leaves' sum for a group.
type treemapTree struct {
	children [][]int
	roots    []int
	value    []float64
	problems []treeProblem
}

// newTree is the tree of the rows, as d3's stratify: a row whose parent is
// empty, or names no row, is a top-level node. A leaf's value is its own
// when it is a finite number above 0, else 0; a group's is the sum of its
// children's, whatever its own.
func newTree(ids, parents []string, values []float64) *treemapTree {
	n := min(len(ids), len(parents), len(values))
	t := &treemapTree{children: make([][]int, n), value: make([]float64, n)}
	byID := make(map[string]int, n)
	for row, id := range ids[:n] {
		if _, ok := byID[id]; ok {
			t.problems = append(t.problems, treeProblem{problemDuplicate, row, id})
		} else {
			byID[id] = row
		}
	}
	parent := make([]int, n)
	for row, p := range parents[:n] {
		parent[row] = -1
		if p == "" {
			continue
		}
		if of, ok := byID[p]; ok {
			parent[row] = of
		} else {
			t.problems = append(t.problems, treeProblem{problemUnknownParent, row, p})
		}
	}
	// A row that is its own ancestor is lost, with what is under it.
	lost := make([]bool, n)
	state := make([]uint8, n) // 0: unseen, 1: on the path, 2: done
	for start := range n {
		var path []int
		cyclic := false
		for at := start; at >= 0; {
			if state[at] == 2 {
				cyclic = lost[at]
				break
			}
			if state[at] == 1 {
				cyclic = true
				t.problems = append(t.problems, treeProblem{kind: problemCycle, row: at})
				break
			}
			state[at] = 1
			path = append(path, at)
			at = parent[at]
		}
		for _, row := range path {
			state[row], lost[row] = 2, cyclic
		}
	}
	for row := range n {
		if lost[row] {
			continue
		}
		if parent[row] >= 0 {
			t.children[parent[row]] = append(t.children[parent[row]], row)
		} else {
			t.roots = append(t.roots, row)
		}
	}
	var sum func(row int) float64
	sum = func(row int) float64 {
		v := 0.0
		if len(t.children[row]) == 0 {
			if isFinite(values[row]) && values[row] > 0 {
				v = values[row]
			}
		} else {
			for _, c := range t.children[row] {
				v += sum(c)
			}
			// A sum past the largest float stays a number.
			v = min(v, math.MaxFloat64)
		}
		t.value[row] = v
		return v
	}
	for _, r := range t.roots {
		sum(r)
	}
	return t
}

func (t *treemapTree) isGroup(row int) bool { return row < len(t.children) && len(t.children[row]) > 0 }

// treemapOptions is how the layout tiles and pads: the paddings by level,
// the last for the deeper ones.
type treemapOptions struct {
	width, height                          float32
	ratio                                  float64
	sort                                   TreemapSort
	paddingInner, paddingOuter, paddingTop []float32
}

// atLevel is the value of level of levels, the last for the deeper ones.
func atLevel(levels []float32, level int) float32 {
	if len(levels) == 0 {
		return 0
	}
	return max(levels[min(level, len(levels)-1)], 0)
}

// treeNode is a node laid out: its box, its level, 0 for the top-level
// nodes, whether it has children laid out in it, the height of its band, and
// its place in the layout's nodes and that of the last node under it.
type treeNode struct {
	row, depth     int
	x0, y0, x1, y1 float32
	group          bool
	header         float32
	first, last    int
}

func (n treeNode) center() (float32, float32) { return (n.x0 + n.x1) / 2, (n.y0 + n.y1) / 2 }

func (n treeNode) distance(x, y float32) float32 {
	dx, dy := max(n.x0-x, x-n.x1, 0), max(n.y0-y, y-n.y1, 0)
	return float32(math.Hypot(float64(dx), float64(dy)))
}

// treeLayout is the nodes that take area, in pre-order, a group before what
// is under it and siblings in their tiling order, the place of each row's
// among them, the places of the top-level nodes, and the content box of each
// node by place: where the pointer finds its children.
type treeLayout struct {
	nodes   []treeNode
	ofRow   []int
	roots   []int
	content [][4]float32
}

func (l *treeLayout) of(row int) (treeNode, bool) {
	if row < 0 || row >= len(l.ofRow) || l.ofRow[row] < 0 {
		return treeNode{}, false
	}
	return l.nodes[l.ofRow[row]], true
}

// childrenOf is the children of the node at place, by place.
func (l *treeLayout) childrenOf(place int) []int {
	node := l.nodes[place]
	var out []int
	if node.group {
		for at := place + 1; at <= node.last; at = l.nodes[at].last + 1 {
			out = append(out, at)
		}
	}
	return out
}

func (l *treeLayout) nearest(places []int, x, y, reach float32) (int, bool) {
	found, best := -1, float32(math.Inf(1))
	for _, i := range places {
		if d := l.nodes[i].distance(x, y); d <= reach && d < best {
			found, best = i, d
		}
	}
	return found, found >= 0
}

// at is the row of the node the pointer at (x, y) is over: a leaf, or a
// group in its band or padding; across a gap, a node beside it, within reach
// at the top level.
func (l *treeLayout) at(x, y, reach float32) (int, bool) {
	at, ok := l.nearest(l.roots, x, y, reach)
	if !ok {
		return 0, false
	}
	for {
		node := l.nodes[at]
		if !node.group {
			return node.row, true
		}
		c := l.content[at]
		if x < c[0] || x > c[2] || y < c[1] || y > c[3] {
			return node.row, true
		}
		child, ok := l.nearest(l.childrenOf(at), x, y, float32(math.Inf(1)))
		if !ok {
			return node.row, true
		}
		at = child
	}
}

// step is the row of the leaf the keys step to from the node of row from:
// an arrow to a leaf on its side on the screen, Home and End to the first
// and last leaf; at an edge the hover stays.
func (l *treeLayout) step(from int, step keyStep) (int, bool) {
	var leaves []treeNode
	for _, n := range l.nodes {
		if !n.group {
			leaves = append(leaves, n)
		}
	}
	if len(leaves) == 0 {
		return 0, false
	}
	first, last := leaves[0].row, leaves[len(leaves)-1].row
	here, ok := l.of(from)
	if !ok {
		if step == stepPrevious || step == stepUp || step == stepLast {
			return last, true
		}
		return first, true
	}
	switch step {
	case stepFirst:
		return first, true
	case stepLast:
		return last, true
	}
	cx, cy := here.center()
	type place struct {
		row         int
		across      bool
		past, apart float32
	}
	abs := func(v float32) float32 { return float32(math.Abs(float64(v))) }
	var found []place
	for _, n := range leaves {
		if n.row == here.row {
			continue
		}
		nx, ny := n.center()
		acrossX := n.x0 < here.x1 && n.x1 > here.x0
		acrossY := n.y0 < here.y1 && n.y1 > here.y0
		switch {
		case step == stepNext && n.x0 >= here.x1-0.5:
			found = append(found, place{n.row, acrossY, n.x0 - here.x1, abs(ny - cy)})
		case step == stepPrevious && n.x1 <= here.x0+0.5:
			found = append(found, place{n.row, acrossY, here.x0 - n.x1, abs(ny - cy)})
		case step == stepDown && n.y0 >= here.y1-0.5:
			found = append(found, place{n.row, acrossX, n.y0 - here.y1, abs(nx - cx)})
		case step == stepUp && n.y1 <= here.y0+0.5:
			found = append(found, place{n.row, acrossX, here.y0 - n.y1, abs(nx - cx)})
		}
	}
	if len(found) == 0 {
		return here.row, true
	}
	rank := func(a bool) int {
		if a {
			return 0
		}
		return 1
	}
	return slices.MinFunc(found, func(a, b place) int {
		return cmp.Or(cmp.Compare(rank(a.across), rank(b.across)), cmp.Compare(a.past, b.past), cmp.Compare(a.apart, b.apart))
	}).row, true
}

// layOutTree lays t out in a box of options' size at the origin, as d3's
// squarified treemap does; a node with no value takes no area, and a level's
// bands go on all the siblings of a parent or none.
func layOutTree(t *treemapTree, o treemapOptions) *treeLayout {
	l := &treeLayout{ofRow: make([]int, len(t.value))}
	for i := range l.ofRow {
		l.ofRow[i] = -1
	}
	w, h := max(o.width, 0), max(o.height, 0)
	if w <= 0 || h <= 0 {
		return l
	}
	roots := orderedRows(t, t.roots, o.sort)
	p := atLevel(o.paddingInner, 0) / 2
	// The top level, as d3 lays out its root's children: the box grown by
	// half the gap, each tile shrunk by it, so no gap runs along the edges.
	tiles := squarify(rowValues(t, roots), [4]float32{-p, -p, w + p, h + p}, o.ratio)
	headers := bands(t, roots, tiles, p, atLevel(o.paddingTop, 0))
	for i, row := range roots {
		if at, ok := placeNode(t, o, l, row, 0, tiles[i], p, headers[i]); ok {
			l.roots = append(l.roots, at)
		}
	}
	return l
}

func rowValues(t *treemapTree, rows []int) []float64 {
	out := make([]float64, len(rows))
	for i, r := range rows {
		out[i] = t.value[r]
	}
	return out
}

// orderedRows is the rows that take area, in the order they are tiled.
func orderedRows(t *treemapTree, rows []int, sort TreemapSort) []int {
	out := make([]int, 0, len(rows))
	for _, r := range rows {
		if t.value[r] > 0 {
			out = append(out, r)
		}
	}
	if sort == TreemapByValue {
		// Stable: equal values keep the rows' order.
		slices.SortStableFunc(out, func(a, b int) int { return cmp.Compare(t.value[b], t.value[a]) })
	}
	return out
}

func round32(v float32) float32 { return float32(math.Round(float64(v))) }

// placeNode places row in its tile and what is under it; its place in the
// nodes, or false when it has no area.
func placeNode(t *treemapTree, o treemapOptions, l *treeLayout, row, depth int, tile [4]float32, p, header float32) (int, bool) {
	x0, y0, x1, y1 := tile[0]+p, tile[1]+p, tile[2]-p, tile[3]-p
	if x1 < x0 {
		x0 = (x0 + x1) / 2
		x1 = x0
	}
	if y1 < y0 {
		y0 = (y0 + y1) / 2
		y1 = y0
	}
	x0, y0, x1, y1 = round32(x0), round32(y0), round32(x1), round32(y1)
	if x1-x0 < 1 || y1-y0 < 1 {
		return 0, false
	}
	at := len(l.nodes)
	l.nodes = append(l.nodes, treeNode{row: row, depth: depth, x0: x0, y0: y0, x1: x1, y1: y1, first: at, last: at})
	l.content = append(l.content, [4]float32{x0, y0, x1, y1})
	l.ofRow[row] = at
	if children := orderedRows(t, t.children[row], o.sort); len(children) > 0 {
		outer := atLevel(o.paddingOuter, depth)
		inner := atLevel(o.paddingInner, depth+1) / 2
		top := min(y0+outer+header, y1)
		content := [4]float32{x0 + outer, top, max(x1-outer, x0+outer), max(y1-outer, top)}
		tiles := squarify(rowValues(t, children), [4]float32{content[0] - inner, content[1] - inner, content[2] + inner, content[3] + inner}, o.ratio)
		headers := bands(t, children, tiles, inner, atLevel(o.paddingTop, depth+1))
		for i, child := range children {
			placeNode(t, o, l, child, depth+1, tiles[i], inner, headers[i])
		}
		// A group whose children all rounded to nothing is drawn as a leaf.
		if len(l.nodes) > at+1 {
			l.nodes[at].group, l.nodes[at].header = true, header
			l.content[at] = content
		}
	}
	l.nodes[at].last = len(l.nodes) - 1
	return at, true
}

// bands is the height of the band at the top of each of rows, siblings tiled
// in tiles: on all of their groups or none.
func bands(t *treemapTree, rows []int, tiles [][4]float32, p, band float32) []float32 {
	fits := func(c [4]float32) bool { return c[2]-c[0]-2*p >= 2*band && c[3]-c[1]-2*p >= 2*band }
	banded := band > 0
	for i, r := range rows {
		banded = banded && (!t.isGroup(r) || fits(tiles[i]))
	}
	out := make([]float32, len(rows))
	for i, r := range rows {
		if banded && t.isGroup(r) {
			out[i] = band
		}
	}
	return out
}

// squarify is d3's treemapSquarify: the box cut into a tile per value, in
// order, near ratio. The values are above 0.
func squarify(values []float64, area [4]float32, ratio float64) [][4]float32 {
	n := len(values)
	out := make([][4]float32, n)
	for i := range out {
		out[i] = [4]float32{area[0], area[1], area[0], area[1]}
	}
	x0, y0, x1, y1 := float64(area[0]), float64(area[1]), float64(area[2]), float64(area[3])
	// Only the shares matter: divided by the largest, sums never overflow.
	unit := 0.0
	for _, v := range values {
		unit = max(unit, v)
	}
	if n == 0 || unit <= 0 || x1 <= x0 || y1 <= y0 {
		return out
	}
	value := func(i int) float64 { return values[i] / unit }
	ratio = max(ratio, 1)
	remaining := 0.0
	for i := range n {
		remaining += value(i)
	}
	for i0, i1 := 0, 0; i0 < n; i0 = i1 {
		dx, dy := x1-x0, y1-y0
		sum := value(i1)
		i1++
		low, high := sum, sum
		alpha := max(dy/dx, dx/dy) / (remaining * ratio)
		beta := sum * sum * alpha
		minRatio := max(high/beta, beta/low)
		for i1 < n {
			v := value(i1)
			sum += v
			low, high = min(low, v), max(high, v)
			beta = sum * sum * alpha
			next := max(high/beta, beta/low)
			if next > minRatio {
				sum -= v
				break
			}
			minRatio = next
			i1++
		}
		// The row: across the box's width when it is taller than wide, else
		// down its height; the last row takes what is left, so no sliver of
		// rounding stays.
		last := i1 >= n
		share := 1.0
		if !last {
			share = sum / remaining
		}
		if dx < dy {
			ry1 := y1
			if !last {
				ry1 = y0 + dy*share
			}
			x := x0
			for i := i0; i < i1; i++ {
				nx := x1
				if i+1 != i1 {
					nx = x + dx*value(i)/sum
				}
				out[i] = [4]float32{float32(x), float32(y0), float32(nx), float32(ry1)}
				x = nx
			}
			y0 = ry1
		} else {
			rx1 := x1
			if !last {
				rx1 = x0 + dx*share
			}
			y := y0
			for i := i0; i < i1; i++ {
				ny := y1
				if i+1 != i1 {
					ny = y + dy*value(i)/sum
				}
				out[i] = [4]float32{float32(x0), float32(y), float32(rx1), float32(ny)}
				y = ny
			}
			x0 = rx1
		}
		remaining -= sum
	}
	return out
}
