package flow

import (
	"math"
	"slices"
	"sort"
)

// The layout is a layered (Sugiyama) drawing in four steps: cycles are
// broken by reversing back edges, nodes are ranked so edges point down the
// ranks and stay short, ranks are ordered to reduce crossings, and nodes
// are placed across their rank as close to their neighbours as the gaps
// allow. Edges spanning several ranks pass through dummy nodes, which keep
// their path clear and become the bends of orthogonal routes.
//
// The algorithm works along two axes: breadth runs across a rank and depth
// down the ranks. Top to bottom maps them to x and y, left to right to y
// and x.

// portInset keeps an edge's end this far inside a node's side, clear of
// its rounded corners.
const portInset = 24

type point struct{ x, y float64 }

type layoutNode struct{ width, height float64 }

// layoutEdge joins two nodes; dashed edges keep off solid ones' tracks.
type layoutEdge struct {
	source, target int
	dashed         bool
}

type spacing struct {
	// node separates nodes across a rank, rank separates the ranks and edge
	// separates edges passing between nodes.
	node, rank, edge float64
}

type layoutResult struct {
	// x and y are each node's top left corner.
	x, y          []float64
	width, height float64
	// routes are each edge's points from its source to its target, nil for
	// edges the layout ignores.
	routes [][]point
}

// vertex is a node of the layered graph: a real node or a dummy that an
// edge spanning several ranks passes through.
type vertex struct {
	breadth, depth float64
	rank           int
	dummy          bool
	pos            float64
	index          int
	up, down       []int
}

func layout(nodes []layoutNode, edges []layoutEdge, direction Direction, gaps spacing) layoutResult {
	n := len(nodes)
	result := layoutResult{x: make([]float64, n), y: make([]float64, n), routes: make([][]point, len(edges))}
	if n == 0 {
		return result
	}
	across := func(nd layoutNode) (float64, float64) {
		if direction == LeftToRight {
			return nd.height, nd.width
		}
		return nd.width, nd.height
	}

	valid := make([]bool, len(edges))
	for i, e := range edges {
		valid[i] = e.source >= 0 && e.source < n && e.target >= 0 && e.target < n && e.source != e.target
	}
	reversed := breakCycles(n, edges, valid)
	from := func(i int) (int, int) {
		if reversed[i] {
			return edges[i].target, edges[i].source
		}
		return edges[i].source, edges[i].target
	}
	ranks := rank(n, edges, valid, from)

	vertices := make([]vertex, n)
	for i, nd := range nodes {
		b, d := across(nd)
		vertices[i] = vertex{breadth: b, depth: d, rank: ranks[i]}
	}
	link := func(a, b int) {
		vertices[a].down = append(vertices[a].down, b)
		vertices[b].up = append(vertices[b].up, a)
	}
	// Each edge's chain of vertices runs down the ranks, from the upper end.
	chains := make([][]int, len(edges))
	for i := range edges {
		if !valid[i] {
			continue
		}
		u, v := from(i)
		chain := []int{u}
		for r := ranks[u] + 1; r < ranks[v]; r++ {
			vertices = append(vertices, vertex{breadth: 0, rank: r, dummy: true})
			chain = append(chain, len(vertices)-1)
		}
		chain = append(chain, v)
		for j := 1; j < len(chain); j++ {
			link(chain[j-1], chain[j])
		}
		chains[i] = chain
	}

	layers := order(vertices)
	place(vertices, layers, gaps)

	// Ranks take the depth of their deepest node; nodes center in them.
	top := make([]float64, len(layers))
	depth := make([]float64, len(layers))
	for r, layer := range layers {
		for _, v := range layer {
			depth[r] = max(depth[r], vertices[v].depth)
		}
		if r > 0 {
			top[r] = top[r-1] + depth[r-1] + gaps.rank
		}
	}
	start := func(v int) float64 { return top[vertices[v].rank] + (depth[vertices[v].rank]-vertices[v].depth)/2 }
	minBreadth := math.Inf(1)
	for _, v := range vertices {
		minBreadth = min(minBreadth, v.pos-v.breadth/2)
	}
	for i := range vertices {
		vertices[i].pos -= minBreadth
	}
	world := func(b, d float64) point {
		if direction == LeftToRight {
			return point{d, b}
		}
		return point{b, d}
	}
	for i := range n {
		v := vertices[i]
		corner := world(v.pos-v.breadth/2, start(i))
		result.x[i], result.y[i] = corner.x, corner.y
		size := world(v.breadth, v.depth)
		result.width, result.height = max(result.width, corner.x+size.x), max(result.height, corner.y+size.y)
	}

	lines := straighten(vertices, layers, chains, gaps)
	tracks := assignTracks(vertices, chains, lines, edges, n)
	for i, chain := range chains {
		if chain == nil {
			continue
		}
		xs := lines[i]
		first, last := chain[0], chain[len(chain)-1]
		points := []point{{xs[0], start(first) + vertices[first].depth}}
		for j := 1; j < len(chain); j++ {
			if math.Abs(xs[j-1]-xs[j]) < .5 {
				continue
			}
			r := vertices[chain[j-1]].rank
			bottom := top[r] + depth[r]
			k := tracks[trackKey{i, j}]
			d := bottom + gaps.rank*float64(k.index+1)/float64(k.count+1)
			points = append(points, point{xs[j-1], d}, point{xs[j], d})
		}
		// Runs straighter than half a DIP stay straight, so the edge ends
		// in line with its last bend.
		points = append(points, point{points[len(points)-1].x, start(last)})
		points = simplify(points)
		for j := range points {
			points[j] = world(points[j].x, points[j].y)
		}
		if reversed[i] {
			slices.Reverse(points)
		}
		result.routes[i] = points
	}
	return result
}

// breakCycles reverses the edges a depth-first search finds pointing back
// to a node still on its path, which leaves the graph acyclic.
func breakCycles(n int, edges []layoutEdge, valid []bool) []bool {
	out := make([][]int, n)
	for i, e := range edges {
		if valid[i] {
			out[e.source] = append(out[e.source], i)
		}
	}
	reversed := make([]bool, len(edges))
	state := make([]uint8, n)
	type frame struct{ node, next int }
	for root := range n {
		if state[root] != 0 {
			continue
		}
		stack := []frame{{root, 0}}
		state[root] = 1
		for len(stack) > 0 {
			f := &stack[len(stack)-1]
			if f.next == len(out[f.node]) {
				state[f.node] = 2
				stack = stack[:len(stack)-1]
				continue
			}
			e := out[f.node][f.next]
			f.next++
			switch t := edges[e].target; state[t] {
			case 1:
				reversed[e] = true
			case 0:
				state[t] = 1
				stack = append(stack, frame{t, 0})
			}
		}
	}
	return reversed
}

// rank places each node below its predecessors by longest path, then moves
// nodes within the room their neighbours leave toward the median of the
// ranks that would make their edges shortest, which brings sources down
// beside their successors.
func rank(n int, edges []layoutEdge, valid []bool, from func(int) (int, int)) []int {
	preds, succs := make([][]int, n), make([][]int, n)
	indegree := make([]int, n)
	for i := range edges {
		if !valid[i] {
			continue
		}
		u, v := from(i)
		succs[u] = append(succs[u], v)
		preds[v] = append(preds[v], u)
		indegree[v]++
	}
	topo := make([]int, 0, n)
	for v := range n {
		if indegree[v] == 0 {
			topo = append(topo, v)
		}
	}
	for head := 0; head < len(topo); head++ {
		for _, w := range succs[topo[head]] {
			if indegree[w]--; indegree[w] == 0 {
				topo = append(topo, w)
			}
		}
	}
	ranks := make([]int, n)
	for _, v := range topo {
		for _, u := range preds[v] {
			ranks[v] = max(ranks[v], ranks[u]+1)
		}
	}
	for pass := range 4 {
		changed := false
		for k := range topo {
			v := topo[k]
			if pass%2 == 0 {
				v = topo[len(topo)-1-k]
			}
			if len(preds[v])+len(succs[v]) == 0 {
				continue
			}
			lo, hi := math.MinInt, math.MaxInt
			wants := make([]int, 0, len(preds[v])+len(succs[v]))
			for _, u := range preds[v] {
				lo = max(lo, ranks[u]+1)
				wants = append(wants, ranks[u]+1)
			}
			for _, w := range succs[v] {
				hi = min(hi, ranks[w]-1)
				wants = append(wants, ranks[w]-1)
			}
			sort.Ints(wants)
			low, high := wants[(len(wants)-1)/2], wants[len(wants)/2]
			want := min(max(ranks[v], low), high)
			want = min(max(want, lo), hi)
			if want != ranks[v] {
				ranks[v], changed = want, true
			}
		}
		if !changed {
			break
		}
	}
	lowest := math.MaxInt
	for _, r := range ranks {
		lowest = min(lowest, r)
	}
	for i := range ranks {
		ranks[i] -= lowest
	}
	return ranks
}

// order arranges each rank by the barycenters of its neighbours, sweeping
// down and up, and keeps the arrangement with the fewest crossings.
func order(vertices []vertex) [][]int {
	ranks := 0
	for _, v := range vertices {
		ranks = max(ranks, v.rank+1)
	}
	layers := make([][]int, ranks)
	for i, v := range vertices {
		layers[v.rank] = append(layers[v.rank], i)
	}
	reindex := func(layer []int) {
		for k, v := range layer {
			vertices[v].index = k
		}
	}
	for _, layer := range layers {
		reindex(layer)
	}
	best := cloneLayers(layers)
	bestCrossings := crossings(vertices, layers)
	sweep := func(r int, neighbours func(*vertex) []int) {
		layer := layers[r]
		centers := make(map[int]float64, len(layer))
		for _, v := range layer {
			adjacent := neighbours(&vertices[v])
			if len(adjacent) == 0 {
				centers[v] = float64(vertices[v].index)
				continue
			}
			sum := 0.0
			for _, u := range adjacent {
				sum += float64(vertices[u].index)
			}
			centers[v] = sum / float64(len(adjacent))
		}
		sort.SliceStable(layer, func(a, b int) bool { return centers[layer[a]] < centers[layer[b]] })
		reindex(layer)
	}
	for iteration := range 12 {
		if iteration%2 == 0 {
			for r := 1; r < ranks; r++ {
				sweep(r, func(v *vertex) []int { return v.up })
			}
		} else {
			for r := ranks - 2; r >= 0; r-- {
				sweep(r, func(v *vertex) []int { return v.down })
			}
		}
		if c := crossings(vertices, layers); c < bestCrossings {
			best, bestCrossings = cloneLayers(layers), c
			if c == 0 {
				break
			}
		}
	}
	for _, layer := range best {
		reindex(layer)
	}
	return best
}

func cloneLayers(layers [][]int) [][]int {
	out := make([][]int, len(layers))
	for i, l := range layers {
		out[i] = slices.Clone(l)
	}
	return out
}

// crossings counts the pairs of edges that cross between adjacent ranks.
func crossings(vertices []vertex, layers [][]int) int {
	total := 0
	type pair struct{ a, b int }
	var pairs []pair
	for _, layer := range layers {
		pairs = pairs[:0]
		for _, u := range layer {
			for _, v := range vertices[u].down {
				pairs = append(pairs, pair{vertices[u].index, vertices[v].index})
			}
		}
		for i := range pairs {
			for j := i + 1; j < len(pairs); j++ {
				p, q := pairs[i], pairs[j]
				if (p.a < q.a && p.b > q.b) || (p.a > q.a && p.b < q.b) {
					total++
				}
			}
		}
	}
	return total
}

// place sets each vertex's breadth position: alternate sweeps move each rank
// toward the mean position of its neighbours above or below, as close as
// its order and gaps allow. Dummies weigh more, which keeps long edges
// straight.
func place(vertices []vertex, layers [][]int, gaps spacing) {
	separation := func(a, b *vertex) float64 {
		gap := gaps.node
		switch {
		case a.dummy && b.dummy:
			gap = gaps.edge
		case a.dummy || b.dummy:
			gap = max(gaps.edge, gaps.node/2)
		}
		return (a.breadth+b.breadth)/2 + gap
	}
	for _, layer := range layers {
		x := 0.0
		for k, v := range layer {
			if k > 0 {
				x += separation(&vertices[layer[k-1]], &vertices[v])
			}
			vertices[v].pos = x
		}
		for _, v := range layer {
			vertices[v].pos -= x / 2
		}
	}
	align := func(layer []int, neighbours func(*vertex) []int) {
		if len(layer) == 0 {
			return
		}
		desired := make([]float64, len(layer))
		weight := make([]float64, len(layer))
		gap := make([]float64, len(layer))
		for k, v := range layer {
			vx := &vertices[v]
			adjacent := neighbours(vx)
			desired[k], weight[k] = vx.pos, .01
			if len(adjacent) > 0 {
				sum := 0.0
				for _, u := range adjacent {
					sum += vertices[u].pos
				}
				desired[k] = sum / float64(len(adjacent))
				weight[k] = float64(len(adjacent))
				if vx.dummy {
					weight[k] *= 8
				}
			}
			if k > 0 {
				gap[k-1] = separation(&vertices[layer[k-1]], vx)
			}
		}
		for k, x := range isotonic(desired, weight, gap) {
			vertices[layer[k]].pos = x
		}
	}
	up := func(v *vertex) []int { return v.up }
	down := func(v *vertex) []int { return v.down }
	both := func(v *vertex) []int { return append(slices.Clip(v.up), v.down...) }
	for iteration := range 8 {
		if iteration%2 == 0 {
			for r := 1; r < len(layers); r++ {
				align(layers[r], up)
			}
		} else {
			for r := len(layers) - 2; r >= 0; r-- {
				align(layers[r], down)
			}
		}
	}
	for range 2 {
		for _, layer := range layers {
			align(layer, both)
		}
	}
}

// isotonic returns the positions closest to desired, by weighted squared
// distance, that keep their order with at least gap[k] between positions k
// and k+1: pooling adjacent violators solves this exactly.
func isotonic(desired, weight, gap []float64) []float64 {
	n := len(desired)
	offset := make([]float64, n)
	for k := 1; k < n; k++ {
		offset[k] = offset[k-1] + gap[k-1]
	}
	type block struct {
		value, weight float64
		count         int
	}
	blocks := make([]block, 0, n)
	for k := range n {
		b := block{desired[k] - offset[k], weight[k], 1}
		for len(blocks) > 0 && blocks[len(blocks)-1].value > b.value {
			p := blocks[len(blocks)-1]
			blocks = blocks[:len(blocks)-1]
			w := p.weight + b.weight
			b = block{(p.value*p.weight + b.value*b.weight) / w, w, p.count + b.count}
		}
		blocks = append(blocks, b)
	}
	out := make([]float64, 0, n)
	for _, b := range blocks {
		for range b.count {
			out = append(out, b.value+offset[len(out)])
		}
	}
	return out
}

type trackKey struct{ edge, step int }

type track struct{ index, count int }

// straighten chooses where each edge crosses each rank of its chain so that
// it bends as rarely as possible. A dummy may move within the corridor its
// neighbours leave, and an edge may end anywhere along a node's side that
// no other edge uses; edges sharing a side meet at its middle, as branches
// of one line. A straight run touching a node settles as near its middle,
// and any other as near its vertices' placed positions, as its corridors
// allow, without bending to reach the middle.
func straighten(vertices []vertex, layers [][]int, chains [][]int, gaps spacing) [][]float64 {
	clearance := gaps.edge * .75
	// side bounds the lines through a beside its neighbour b, on b's side.
	side := func(a, b *vertex) float64 {
		if b.dummy {
			return (a.pos + b.pos) / 2
		}
		if b.pos < a.pos {
			return b.pos + b.breadth/2 + clearance
		}
		return b.pos - b.breadth/2 - clearance
	}
	corridor := func(v int, edges int) (float64, float64) {
		vx := &vertices[v]
		if !vx.dummy {
			if edges > 1 {
				return vx.pos, vx.pos
			}
			inset := min(vx.breadth/2, portInset)
			return vx.pos - vx.breadth/2 + inset, vx.pos + vx.breadth/2 - inset
		}
		lo, hi := math.Inf(-1), math.Inf(1)
		layer := layers[vx.rank]
		if k := vx.index; k > 0 {
			lo = side(vx, &vertices[layer[k-1]])
			if vertices[layer[k-1]].dummy {
				lo += gaps.edge / 4
			}
		}
		if k := vx.index; k+1 < len(layer) {
			hi = side(vx, &vertices[layer[k+1]])
			if vertices[layer[k+1]].dummy {
				hi -= gaps.edge / 4
			}
		}
		return min(lo, vx.pos), max(hi, vx.pos)
	}
	lines := make([][]float64, len(chains))
	for i, chain := range chains {
		if chain == nil {
			continue
		}
		xs := make([]float64, len(chain))
		begin := 0
		var lo, hi float64
		settle := func(end int) {
			// A run that leaves or reaches a node lines up with its
			// middle where its corridors allow, so lines meet nodes as
			// they meet the nodes around them; others follow their
			// dummies.
			sum, n := 0.0, 0
			for k, v := range chain[begin:end] {
				if j := begin + k; !vertices[v].dummy && (j == 0 || j == len(chain)-1) {
					sum, n = sum+vertices[v].pos, n+1
				}
			}
			if n == 0 {
				for _, v := range chain[begin:end] {
					sum += vertices[v].pos
				}
				n = end - begin
			}
			x := clamp(sum/float64(n), lo, hi)
			for k := begin; k < end; k++ {
				xs[k] = x
			}
		}
		for j, v := range chain {
			edges := 0
			switch j {
			case 0:
				edges = len(vertices[v].down)
			case len(chain) - 1:
				edges = len(vertices[v].up)
			}
			l, h := corridor(v, edges)
			if j > 0 && (l > hi || h < lo) {
				settle(j)
				begin = j
			}
			if j == begin {
				lo, hi = l, h
			} else {
				lo, hi = max(lo, l), min(hi, h)
			}
		}
		settle(len(chain))
		lines[i] = xs
	}
	return lines
}

// assignTracks spreads the cross-rank runs of edges between each pair of
// ranks over separate tracks wherever they would overlap. Runs that branch
// from a node's shared side share a track, as do runs of one style that
// meet at a node's shared side, as branches of one line, and runs of the
// two styles that reach that side from either hand.
func assignTracks(vertices []vertex, chains [][]int, lines [][]float64, edges []layoutEdge, real int) map[trackKey]track {
	type run struct {
		key    trackKey
		lo, hi float64
		owner  int
	}
	byRank := map[int][]run{}
	for i, chain := range chains {
		for j := 1; j < len(chain); j++ {
			a, b := chain[j-1], chain[j]
			xa, xb := lines[i][j-1], lines[i][j]
			if math.Abs(xa-xb) < .5 {
				continue
			}
			owner := 3*real + i
			switch {
			case a < real && len(vertices[a].down) > 1:
				owner = a
			case b < real && len(vertices[b].up) > 1:
				owner = real + 2*b
				if edges[i].dashed {
					owner++
				}
			}
			lo, hi := min(xa, xb), max(xa, xb)
			r := vertices[a].rank
			byRank[r] = append(byRank[r], run{trackKey{i, j}, lo, hi, owner})
		}
	}
	out := map[trackKey]track{}
	for _, runs := range byRank {
		type group struct {
			lo, hi float64
			runs   []trackKey
		}
		groups := map[int]*group{}
		var owners []int
		for _, r := range runs {
			g := groups[r.owner]
			if g == nil {
				g = &group{lo: r.lo, hi: r.hi}
				groups[r.owner] = g
				owners = append(owners, r.owner)
			}
			g.lo, g.hi = min(g.lo, r.lo), max(g.hi, r.hi)
			g.runs = append(g.runs, r.key)
		}
		sort.Slice(owners, func(a, b int) bool {
			ga, gb := groups[owners[a]], groups[owners[b]]
			if ga.lo != gb.lo {
				return ga.lo < gb.lo
			}
			return owners[a] < owners[b]
		})
		// into is the node whose shared side an owner's runs meet at, or
		// -1: groups of both styles meeting there from either side share a
		// track, so they join it in one place rather than one turning down
		// into the other.
		into := func(o int) int {
			if o >= real && o < 3*real {
				return (o - real) / 2
			}
			return -1
		}
		var ends []float64
		var lasts []int
		assigned := make(map[int]int, len(owners))
		for _, o := range owners {
			g := groups[o]
			k := slices.IndexFunc(ends, func(end float64) bool { return end < g.lo })
			if k < 0 && into(o) >= 0 {
				k = slices.IndexFunc(ends, func(end float64) bool { return math.Abs(end-g.lo) < .5 })
				if k >= 0 && into(lasts[k]) != into(o) {
					k = -1
				}
			}
			if k < 0 {
				k = len(ends)
				ends, lasts = append(ends, 0), append(lasts, 0)
			}
			ends[k], lasts[k] = g.hi, o
			assigned[o] = k
		}
		for _, o := range owners {
			for _, key := range groups[o].runs {
				out[key] = track{assigned[o], len(ends)}
			}
		}
	}
	return out
}

// simplify drops points that continue a straight run.
func simplify(points []point) []point {
	out := points[:0:0]
	for _, p := range points {
		if len(out) > 0 && math.Abs(out[len(out)-1].x-p.x) < .01 && math.Abs(out[len(out)-1].y-p.y) < .01 {
			continue
		}
		if len(out) >= 2 {
			a, b := out[len(out)-2], out[len(out)-1]
			if (math.Abs(a.x-b.x) < .01 && math.Abs(b.x-p.x) < .01) || (math.Abs(a.y-b.y) < .01 && math.Abs(b.y-p.y) < .01) {
				out[len(out)-1] = p
				continue
			}
		}
		out = append(out, p)
	}
	return out
}
