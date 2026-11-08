package flow

import (
	"math"
	"testing"
)

func cards(n int) []layoutNode {
	nodes := make([]layoutNode, n)
	for i := range nodes {
		nodes[i] = layoutNode{width: 120, height: 48}
	}
	return nodes
}

var testGaps = spacing{node: 40, rank: 72, edge: 16}

// checkLayout verifies that nodes keep apart within the bounds and that
// every route is orthogonal, runs from its source to its target and passes
// no other node.
func checkLayout(t *testing.T, nodes []layoutNode, edges []layoutEdge, r layoutResult) {
	t.Helper()
	for i := range nodes {
		for j := i + 1; j < len(nodes); j++ {
			if r.x[i] < r.x[j]+nodes[j].width && r.x[j] < r.x[i]+nodes[i].width &&
				r.y[i] < r.y[j]+nodes[j].height && r.y[j] < r.y[i]+nodes[i].height {
				t.Fatalf("nodes %d and %d overlap", i, j)
			}
		}
		if r.x[i] < -.01 || r.y[i] < -.01 || r.x[i]+nodes[i].width > r.width+.01 || r.y[i]+nodes[i].height > r.height+.01 {
			t.Fatalf("node %d leaves the bounds", i)
		}
	}
	for k, e := range edges {
		route := r.routes[k]
		if e.source == e.target {
			if route != nil {
				t.Fatalf("self-loop %d is routed", k)
			}
			continue
		}
		if len(route) < 2 {
			t.Fatalf("edge %d has route %v", k, route)
		}
		for j := 1; j < len(route); j++ {
			a, b := route[j-1], route[j]
			if math.Abs(a.x-b.x) > .01 && math.Abs(a.y-b.y) > .01 {
				t.Fatalf("edge %d bends off the axes: %v", k, route)
			}
		}
		for j := 1; j < len(route); j++ {
			a, b := route[j-1], route[j]
			for m := range nodes {
				if m == e.source || m == e.target {
					continue
				}
				if min(a.x, b.x) < r.x[m]+nodes[m].width && max(a.x, b.x) > r.x[m] &&
					min(a.y, b.y) < r.y[m]+nodes[m].height && max(a.y, b.y) > r.y[m] {
					t.Fatalf("edge %d passes node %d: %v", k, m, route)
				}
			}
		}
		ends := [2]int{e.source, e.target}
		points := [2]point{route[0], route[len(route)-1]}
		for side, n := range ends {
			p := points[side]
			if p.x < r.x[n]-.51 || p.x > r.x[n]+nodes[n].width+.51 || p.y < r.y[n]-.51 || p.y > r.y[n]+nodes[n].height+.51 {
				t.Fatalf("edge %d end %v is off node %d", k, p, n)
			}
		}
	}
}

func TestLayoutChain(t *testing.T) {
	nodes := cards(3)
	edges := []layoutEdge{{0, 1, false}, {1, 2, false}}
	r := layout(nodes, edges, TopToBottom, testGaps)
	checkLayout(t, nodes, edges, r)
	if !(r.y[0] < r.y[1] && r.y[1] < r.y[2]) {
		t.Fatalf("ranks go %v", r.y)
	}
	if r.x[0] != r.x[1] || r.x[1] != r.x[2] {
		t.Fatalf("a chain is not straight: %v", r.x)
	}
	for k, route := range r.routes {
		if len(route) != 2 {
			t.Fatalf("straight edge %d bends: %v", k, route)
		}
		if route[0].y != r.y[k]+48 || route[1].y != r.y[k+1] {
			t.Fatalf("edge %d does not join bottom to top: %v", k, route)
		}
	}
}

func TestLayoutLeftToRight(t *testing.T) {
	nodes := cards(3)
	edges := []layoutEdge{{0, 1, false}, {0, 2, false}}
	r := layout(nodes, edges, LeftToRight, testGaps)
	checkLayout(t, nodes, edges, r)
	if !(r.x[0] < r.x[1] && r.x[1] == r.x[2]) {
		t.Fatalf("ranks do not run across: %v", r.x)
	}
	if r.y[1] == r.y[2] {
		t.Fatal("siblings share a place")
	}
	// The parent centers between its children.
	if got, want := r.y[0]+24, (r.y[1]+r.y[2])/2+24; math.Abs(got-want) > .5 {
		t.Fatalf("parent at %v, children centered at %v", got, want)
	}
}

func TestLayoutAvoidsCrossings(t *testing.T) {
	// Data order invites a crossing: 0 → 3 and 1 → 2.
	nodes := cards(4)
	edges := []layoutEdge{{0, 3, false}, {1, 2, false}}
	r := layout(nodes, edges, TopToBottom, testGaps)
	checkLayout(t, nodes, edges, r)
	if (r.x[0] < r.x[1]) != (r.x[3] < r.x[2]) {
		t.Fatalf("edges cross: %v", r.x)
	}
}

func TestLayoutRanksSourcesBesideSuccessors(t *testing.T) {
	// 3 feeds only 2, which sits two ranks down: it moves down beside 1.
	nodes := cards(4)
	edges := []layoutEdge{{0, 1, false}, {1, 2, false}, {3, 2, false}}
	r := layout(nodes, edges, TopToBottom, testGaps)
	checkLayout(t, nodes, edges, r)
	if r.y[3] != r.y[1] {
		t.Fatalf("source at %v, beside %v expected", r.y[3], r.y[1])
	}
}

func TestLayoutLongEdgesAndCycles(t *testing.T) {
	nodes := cards(6)
	edges := []layoutEdge{
		{0, 1, false}, {1, 2, false}, {2, 3, false}, {0, 3, false}, // a long edge beside a chain
		{3, 1, false},                // closes a cycle
		{4, 4, false},                // a self-loop, ignored
		{0, 5, false}, {5, 3, false}, // a second path
		{9, 0, false}, // an unknown node, ignored
	}
	r := layout(nodes, edges[:len(edges)-1], TopToBottom, testGaps)
	checkLayout(t, nodes, edges[:len(edges)-1], r)
	if r.routes[5] != nil {
		t.Fatal("self-loop is routed")
	}
	// The reversed edge still runs from its source to its target.
	back := r.routes[4]
	if back[0].y < back[len(back)-1].y {
		t.Fatalf("edge closing the cycle points down: %v", back)
	}
	r = layout(nodes, edges, TopToBottom, testGaps)
	if r.routes[len(edges)-1] != nil {
		t.Fatal("edge to an unknown node is routed")
	}
}

func TestLayoutSeparatesOverlappingRuns(t *testing.T) {
	// Two sources cross toward each other's side between the same ranks;
	// their horizontal runs overlap, so they take separate tracks.
	nodes := cards(4)
	edges := []layoutEdge{{0, 2, false}, {0, 3, false}, {1, 2, false}, {1, 3, false}}
	r := layout(nodes, edges, TopToBottom, testGaps)
	checkLayout(t, nodes, edges, r)
	tracks := map[float64]bool{}
	for _, route := range r.routes {
		for j := 1; j < len(route); j++ {
			if route[j].y == route[j-1].y {
				tracks[route[j].y] = true
			}
		}
	}
	if len(tracks) != 2 {
		t.Fatalf("runs use %d tracks", len(tracks))
	}
}

// plan is the gallery's sample plan: eleven steps whose long and dashed
// edges converge on a review step, 8.
func plan() ([]layoutNode, []layoutEdge) {
	return cards(11), []layoutEdge{
		{0, 1, false}, {0, 2, false}, {0, 4, false}, {0, 8, false}, {1, 3, false}, {1, 4, false}, {3, 5, false}, {3, 6, true},
		{4, 7, false}, {2, 8, false}, {5, 8, true}, {7, 8, true}, {8, 9, false}, {8, 10, true},
	}
}

func TestLayoutBendsRarely(t *testing.T) {
	// A plan whose long edges pass dummies placed at different positions;
	// each edge jogs at most twice, so long edges run straight between.
	nodes, edges := plan()
	for _, direction := range []Direction{TopToBottom, LeftToRight} {
		r := layout(nodes, edges, direction, testGaps)
		checkLayout(t, nodes, edges, r)
		for k, route := range r.routes {
			if len(route) > 6 {
				t.Fatalf("edge %d jogs %d times: %v", k, len(route)/2-1, route)
			}
		}
	}
}

func TestLayoutMergesRunsIntoSharedEnds(t *testing.T) {
	// Runs meeting at the review step share a track: the two dashed
	// findings from the left and the solid edges from the right join it
	// in one place.
	nodes, edges := plan()
	r := layout(nodes, edges, TopToBottom, testGaps)
	last := func(k int) float64 {
		route := r.routes[k]
		for j := len(route) - 1; j > 0; j-- {
			if route[j].y == route[j-1].y && route[j].x != route[j-1].x {
				return route[j].y
			}
		}
		t.Fatalf("edge %d runs straight: %v", k, route)
		return 0
	}
	if last(10) != last(11) {
		t.Fatalf("dashed runs into the review take tracks %v and %v", last(10), last(11))
	}
	if last(3) != last(9) || last(3) != last(10) {
		t.Fatalf("solid runs at %v and %v, dashed at %v", last(3), last(9), last(10))
	}
}

func TestIsotonic(t *testing.T) {
	got := isotonic([]float64{10, 0, 50}, []float64{1, 1, 1}, []float64{20, 20})
	want := []float64{-5, 15, 50}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("isotonic = %v, want %v", got, want)
		}
	}
}

func TestLayoutLarge(t *testing.T) {
	const n = 300
	nodes := cards(n)
	var edges []layoutEdge
	for i := 1; i < n; i++ {
		edges = append(edges, layoutEdge{(i - 1) / 3, i, false})
		if i%11 == 0 {
			edges = append(edges, layoutEdge{i / 7, i, false})
		}
	}
	r := layout(nodes, edges, TopToBottom, testGaps)
	checkLayout(t, nodes, edges, r)
}
