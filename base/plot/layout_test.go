package plot

import (
	"math"
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/core"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func TestLinearAndPointScalesPlaceValues(t *testing.T) {
	y := newLinear([2]float64{100, 0}, [2]float32{200, 0})
	if at, _ := y.tick(25); at != 150 {
		t.Errorf("25 at %v, want 150", at)
	}
	if _, ok := y.tick(math.NaN()); ok {
		t.Error("NaN has a place")
	}
	if at, _ := newLinear([2]float64{5, 5}, [2]float32{0, 100}).tick(5); at != 50 {
		t.Errorf("one value at %v, want the middle", at)
	}
	m := axisMap{kind: axisPoint, len: 3, rng: [2]float32{0, 100}}
	if got := m.ticks(3, nil); !slices.Equal(got, []float32{0, 50, 100}) {
		t.Errorf("rows at %v", got)
	}
	if at, _ := (axisMap{kind: axisPoint, len: 1, rng: [2]float32{0, 100}}).tick(0); at != 50 {
		t.Errorf("one row at %v, want the middle", at)
	}
}

func TestBandsLayOutAsD3(t *testing.T) {
	plain := newBands(3, [2]float32{0, 90}, Band{}, false)
	if s, _ := plain.startOf(1); s != 30 || plain.width() != 30 {
		t.Errorf("band at %v wide %v", s, plain.width())
	}
	if c, _ := plain.tick(0.5); c != 30 {
		t.Errorf("between the bands at %v", c)
	}
	padded := newBands(6, [2]float32{0, 600}, Band{PaddingInner: 0.3, PaddingOuter: 0.15}, false)
	for i := range 6 {
		if c, _ := padded.center(i); !near(c, 100*float32(i)+50) {
			t.Errorf("band %d's middle at %v", i, c)
		}
	}
	capped := newBands(2, [2]float32{0, 200}, Band{MaxBandWidth: 30}, false)
	if s, _ := capped.startOf(1); s != 135 || capped.width() != 30 {
		t.Errorf("capped band at %v wide %v", s, capped.width())
	}
	count := func(rows int) bandMap {
		return newBands(rows, [2]float32{0, 100}, Band{BandCount: 4, PaddingInner: 0.4}, false)
	}
	a, _ := count(2).startOf(1)
	b, _ := count(4).startOf(1)
	if a != b {
		t.Errorf("BandCount moves a band: %v and %v", a, b)
	}
	reversed := newBands(3, [2]float32{0, 90}, Band{}, true)
	if s, _ := reversed.startOf(0); s != 60 {
		t.Errorf("reversed band at %v", s)
	}
	if c, _ := reversed.tick(2); c != 15 {
		t.Errorf("reversed tick at %v", c)
	}
	if !(axisMap{kind: axisBand, bands: reversed}).backward() {
		t.Error("a reversed band scale does not run back")
	}
	two := bandGroup(2, 30, 0)
	if s, _ := two.startOf(1); s != 15 || two.width() != 15 {
		t.Errorf("two bars share %v %v", s, two.width())
	}
	if one := bandGroup(1, 30, 0.5); one.width() != 30 {
		t.Errorf("one bar is %v wide, want the band", one.width())
	}
}

func TestAlignedScalesShareTheirSteps(t *testing.T) {
	volume, price, ok := align([2]float64{0, 280e6}, [2]float64{172.4, 191.8}, 5)
	if !ok || volume.domain != [2]float64{0, 300e6} || volume.step != 100e6 {
		t.Errorf("volume %+v", volume)
	}
	if price.domain != [2]float64{170, 200} || price.step != 10 {
		t.Errorf("price %+v", price)
	}
	profit, margin, _ := align([2]float64{-20, 100}, [2]float64{-0.05, 0.3}, 6)
	zero := func(a aligned) float64 { return -a.domain[0] / (a.domain[1] - a.domain[0]) }
	if math.Abs(zero(profit)-zero(margin)) > 1e-9 {
		t.Errorf("zeros apart: %+v %+v", profit, margin)
	}
	if _, _, ok := align([2]float64{5, 5}, [2]float64{0, 1}, 5); ok {
		t.Error("a single value has steps")
	}
}

func TestOrdinalColorsTakeTheirDomainAndMeetOthers(t *testing.T) {
	red, green, blue, gray := core.RGB(0xff0000), core.RGB(0x00ff00), core.RGB(0x0000ff), core.RGB(0x999999)
	var met []string
	s := OrdinalColors([]string{"a", "a", "b"}, red, green)
	if s.colorOf(&met, "b") != green || s.colorOf(&met, "a") != red || s.colorOf(&met, "") != 0 {
		t.Error("the domain's colors are off")
	}
	implicit := OrdinalColors([]string{"a"}, red, green, blue)
	got := []core.Color{implicit.colorOf(&met, "z"), implicit.colorOf(&met, "a"), implicit.colorOf(&met, "y"), implicit.colorOf(&met, "z")}
	if !slices.Equal(got, []core.Color{green, red, blue, green}) || !slices.Equal(met, []string{"z", "y"}) {
		t.Errorf("met %v colors %v", met, got)
	}
	if c := OrdinalColors(nil, red).Unknown(gray).colorOf(&met, "q"); c != gray {
		t.Errorf("unknown %08x", uint32(c))
	}
}

func TestRampsMixInOKLab(t *testing.T) {
	white, blue, gray := core.RGB(0xffffff), core.RGB(0x0000ff), core.RGB(0x999999)
	ramp := SequentialColors(white, blue).Unknown(gray)
	domain, _ := ramp.spread(plotExtent{0, 10, true})
	if ramp.shade(0, domain) != white || ramp.shade(99, domain) != blue || ramp.shade(math.NaN(), domain) != gray {
		t.Error("the ramp's ends are off")
	}
	if got, want := ramp.shade(5, domain), mixOKLab(white, blue, 0.5); got != want {
		t.Errorf("middle %08x, want %08x", uint32(got), uint32(want))
	}
	red, green := core.RGB(0xff0000), core.RGB(0x00ff00)
	div := DivergingColors(red, white, green)
	even, _ := div.spread(plotExtent{-2, 4, true})
	if even != [2]float64{-4, 4} {
		t.Errorf("domain %v, want even about 0", even)
	}
	if div.shade(0, even) != white || div.shade(-4, even) != red {
		t.Error("the pivot takes the middle")
	}
	if got := div.shade(5, [2]float64{-1, 10}); got != mixOKLab(white, green, 0.5) {
		t.Errorf("a lopsided domain moves the pivot: %08x", uint32(got))
	}
	if !isDark(blue) || isDark(white) {
		t.Error("dark is off")
	}
	// Mixing an end returns it, to the byte.
	if got := mixOKLab(red, blue, 0); got != red {
		t.Errorf("mix %08x", uint32(got))
	}
}

func TestD3Ticks(t *testing.T) {
	if got := linearTicks(0, 10, 5); !slices.Equal(got, []float64{0, 2, 4, 6, 8, 10}) {
		t.Errorf("ticks %v", got)
	}
	if got := linearTicks(0, 1, 5); !slices.Equal(got, []float64{0, 0.2, 0.4, 0.6, 0.8, 1}) {
		t.Errorf("ticks %v", got)
	}
	if got := linearTicks(3, 3, 5); !slices.Equal(got, []float64{3}) {
		t.Errorf("one value %v", got)
	}
	for _, c := range []struct {
		v    float64
		p    int
		want string
	}{{1500, 0, "1,500"}, {0.25, 2, "0.25"}, {-2, 0, "−2"}, {-0.001, 1, "0.0"}, {1234567.5, 1, "1,234,567.5"}} {
		if got := defaultTickText(c.v, c.p); got != c.want {
			t.Errorf("%v at %d: %q, want %q", c.v, c.p, got, c.want)
		}
	}
	if tickPrecision(0, 1, 5) != 1 || tickPrecision(0, 100, 5) != 0 {
		t.Error("precision is off")
	}
	if got := spread(10, 3); !slices.Equal(got, []int{0, 5, 9}) {
		t.Errorf("spread %v", got)
	}
	if got := labelInsets([]labelMeasure{{0, 10}, {1, 10}}, []labelMeasure{{0, 0}, {1, 1}}, 100); !near(got[0], 5) || !near(got[1], 5) {
		t.Errorf("insets %v", got)
	}
	// A label not laid out yet keeps the room the others ask for.
	if got := labelInsets([]labelMeasure{{0, 10}}, []labelMeasure{{0, 1}, {3, 0}}, 100); !near(got[0], 5) || !near(got[1], 5) {
		t.Errorf("insets %v", got)
	}
}

func values(v ...float64) func() []float64 { return func() []float64 { return v } }

func TestBarsStackAndStandSideBySide(t *testing.T) {
	l := layOutBars([]*barEntry{
		{key: 0, stack: "fruit", values: values(10, 15)},
		{key: 1, values: values(5, -3)},
		{key: 2, stack: "fruit", base: func() float64 { return 99 }, values: values(20, math.NaN())},
		{key: 3, stack: "fruit", values: values(30, 35)},
	})
	if l.slots != 2 {
		t.Fatalf("%d slots", l.slots)
	}
	place := func(k int) barPlace { p, _ := l.of(k); return p }
	if place(1).slot != 1 || place(3).slot != 0 {
		t.Errorf("slots %d %d", place(1).slot, place(3).slot)
	}
	if s := place(2).segments[0]; s != [2]float64{10, 30} {
		t.Errorf("the second piles at %v", s)
	}
	if !math.IsNaN(place(2).segments[1][0]) {
		t.Error("a gap has a segment")
	}
	if s := place(3).segments; s[0] != [2]float64{30, 60} || s[1] != [2]float64{15, 50} {
		t.Errorf("the last piles at %v", s)
	}
	// A stack of each y scale is its own, and a base per row makes ranges.
	l = layOutBars([]*barEntry{
		{key: 0, stack: "all", values: values(10)},
		{key: 1, stack: "all", scale: ScaleY2, values: values(1)},
		{key: 2, bases: values(100, math.NaN()), values: values(120, 50)},
	})
	if l.slots != 3 {
		t.Errorf("%d slots", l.slots)
	}
	bridge, _ := l.of(2)
	if bridge.segments[0] != [2]float64{100, 120} || !math.IsNaN(bridge.segments[1][0]) || bridge.hasFloor {
		t.Errorf("range %+v", bridge)
	}
}

func TestKeysStepThroughPlacedRows(t *testing.T) {
	xs := []float32{0, float32(math.NaN()), 50, 100}
	placed := func(i int) bool { return !math.IsNaN(float64(xs[i])) }
	for _, c := range []struct {
		from int
		step keyStep
		want int
	}{{-1, stepNext, 0}, {-1, stepPrevious, 3}, {0, stepNext, 2}, {2, stepPrevious, 0}, {3, stepNext, 3}, {0, stepPrevious, 0},
		{2, stepFirst, 0}, {0, stepLast, 3}, {9, stepNext, 3}, {9, stepPrevious, 3}} {
		if got, _ := stepRow(len(xs), placed, c.from, c.step); got != c.want {
			t.Errorf("from %d step %d: %d, want %d", c.from, c.step, got, c.want)
		}
	}
	if _, ok := stepRow(1, func(int) bool { return false }, 0, stepNext); ok {
		t.Error("no row placed finds one")
	}
}

func TestPieSlicesFollowTheirValues(t *testing.T) {
	s := pieArcs([]float64{1, 0, 3, math.NaN(), -2}, 0, tau)
	if len(s) != 2 || s[0].row != 0 || s[1].row != 2 || !near(s[0].end, tau/4) || !near(s[1].end, tau) {
		t.Fatalf("slices %+v", s)
	}
	r := pieRing{50, 50, 10, 50}
	if row, ok := r.sliceAt(s, 0, core.Point{X: 80, Y: 30}); !ok || row != 0 {
		t.Errorf("right of 12 o'clock finds %d %v", row, ok)
	}
	if _, ok := r.sliceAt(s, 0, core.Point{X: 50, Y: 50}); ok {
		t.Error("the hole has a slice")
	}
	if row, ok := r.sliceAt(s, 0, core.Point{X: 20, Y: 50}); !ok || row != 2 {
		t.Errorf("the left finds %d %v", row, ok)
	}
	// A new slice opens where the one before it ended.
	if a, b := sliceFrom(s[:1], 2, 0); a != s[0].end || b != a {
		t.Errorf("new slice from %v %v", a, b)
	}
	// d3's corners: at most half the ring.
	o, ok := newOutline(sectorShape{20, 60, 0, math.Pi}, 0, 100)
	if !ok || !near(o.outer.radius, 20) {
		t.Errorf("corner %+v", o.outer)
	}
	if _, ok := newOutline(sectorShape{0, 50, 0, 0.001}, 0.1, 0); ok {
		t.Error("a slice narrower than its pad draws")
	}
	if o, _ := newOutline(sectorShape{20, 60, 0, tau}, 0, 8); o.rounded() {
		t.Error("a whole turn has corners")
	}
}

func TestRadialAnglesAndOutlines(t *testing.T) {
	if a := spokeAngle(1, 4, 0, tau); !near(a, tau/4) {
		t.Errorf("a whole turn's step %v", a)
	}
	if a := spokeAngle(2, 3, 0, math.Pi); !near(a, math.Pi) {
		t.Errorf("a part's last %v", a)
	}
	f := radialPose{spokes: []radialSpoke{{0, 0, 10}, {1, tau / 3, 5}, {2, 2 * tau / 3, 10}}, domain: [2]float64{0, 10}, hasDomain: true,
		outer: radiusOf{points: 40}}.frame(core.Size{Width: 100, Height: 100})
	if p, ok := f.pointOf(0); !ok || !near(float32(p.Y), 10) {
		t.Errorf("the first row at %+v", p)
	}
	if row, ok := f.rowAt(core.Point{X: 50, Y: 20}); !ok || row != 0 {
		t.Errorf("found %d", row)
	}
	if _, ok := f.rowAt(core.Point{X: 0, Y: 0}); ok {
		t.Error("past the outer radius finds a row")
	}
	o := outlineOf(f, CurveLinear, true, false)
	if len(o.line) != 5 || o.line[4].kind != segClose {
		t.Errorf("polygon %+v", o.line)
	}
	if o := outlineOf(f, CurveLinear, false, true); o.area[len(o.area)-1].kind != segClose {
		t.Errorf("area %+v", o.area)
	}
	if g := f.grown(); g.vertices[0].radius != 0 {
		t.Errorf("grown %+v", g.vertices[0])
	}
}

func generator(w, h float32) sankeyGenerator {
	return sankeyGenerator{nodeWidth: 10, nodePadding: 8, iterations: 6, x1: w, y1: h}
}

func TestSankeyLaysFlowsInColumns(t *testing.T) {
	links := []SankeyLink{{0, 2, 5}, {1, 2, 15}, {2, 3, 20}}
	g, _, ok := generator(300, 200).layout(4, links)
	if !ok {
		t.Fatal("no layout")
	}
	if g.nodes[0].x0 != 0 || g.nodes[3].x1 != 300 {
		t.Errorf("columns %v %v", g.nodes[0].x0, g.nodes[3].x1)
	}
	if g.nodes[2].value != 20 {
		t.Errorf("throughput %v", g.nodes[2].value)
	}
	// Both sides of a node are covered by its ribbons.
	n := g.nodes[2]
	var in float32
	for _, l := range n.targetLinks {
		in += g.links[l].targetWidth
	}
	if !near(in, n.y1-n.y0) {
		t.Errorf("in %v of %v", in, n.y1-n.y0)
	}
	if _, _, ok := generator(300, 200).layout(2, []SankeyLink{{0, 1, 1}, {1, 0, 1}}); ok {
		t.Error("a cycle lays out")
	}
	if _, bad, ok := generator(300, 200).layout(1, []SankeyLink{{0, 5, 1}}); ok || bad != 5 {
		t.Errorf("a missing row: %d %v", bad, ok)
	}
	// Equal single nodes stagger, so their ribbons curve.
	g, _, _ = generator(300, 200).layout(5, []SankeyLink{{0, 1, 5}, {1, 2, 5}, {2, 3, 2}, {2, 4, 3}})
	if g.nodes[0].y0 == g.nodes[1].y0 {
		t.Error("flat columns do not stagger")
	}
	// Square roots keep nodes covered.
	s := generator(300, 200)
	s.scale = SankeySqrt
	if g, _, _ := s.layout(2, []SankeyLink{{0, 1, 16}}); g.links[0].value != 4 {
		t.Errorf("sqrt %v", g.links[0].value)
	}
	if got := throughputs(3, links[:2]); !slices.Equal(got, []float64{5, 15, 20}) {
		t.Errorf("throughput %v", got)
	}
}

func TestSankeyLinksKeepTheirKeys(t *testing.T) {
	g, _, _ := generator(300, 200).layout(3, []SankeyLink{{0, 1, 1}, {0, 1, 2}, {1, 2, 0}})
	keys := linkKeys(g)
	if !slices.Equal(keys, []linkKey{{0, 1, 0}, {0, 1, 1}}) {
		t.Errorf("keys %v", keys)
	}
	if l := findLink(g, linkKey{0, 1, 1}); l == nil || l.value != 2 {
		t.Errorf("found %+v", l)
	}
	p := poseOf(g, 1)
	if p.nodes[0].side != sideBefore || p.nodes[1].side != sideAbove || p.nodes[2].side != sideAfter {
		t.Errorf("sides %+v", p.nodes)
	}
	r := p.ribbons[0].ribbon
	if thin := r.thin(2); !near(thin.sBottom-thin.sTop, 2) {
		t.Errorf("thin %+v", thin)
	}
}

func TestTreeFromRows(t *testing.T) {
	ids := []string{"root", "a", "b", "a1", "a2", "lost", "lost2"}
	parents := []string{"", "root", "root", "a", "a", "lost2", "lost"}
	tree := newTree(ids, parents, []float64{99, 0, 4, 1, 3, 1, 1})
	if tree.value[0] != 8 || tree.value[1] != 4 {
		t.Errorf("sums %v", tree.value)
	}
	if !tree.isGroup(1) || tree.isGroup(2) {
		t.Error("groups are off")
	}
	if len(tree.problems) != 1 || tree.problems[0].kind != problemCycle {
		t.Errorf("problems %+v", tree.problems)
	}
	if len(tree.roots) != 1 {
		t.Errorf("roots %v", tree.roots)
	}
	other := newTree([]string{"a", "a", "b"}, []string{"", "", "gone"}, []float64{1, 1, 1})
	if len(other.problems) != 2 || other.problems[0].kind != problemDuplicate || other.problems[1].kind != problemUnknownParent {
		t.Errorf("problems %+v", other.problems)
	}
}

func TestSquarifyFillsItsBox(t *testing.T) {
	values := []float64{6, 6, 4, 3, 2, 2, 1}
	tiles := squarify(values, [4]float32{0, 0, 600, 400}, 1)
	var area float64
	for _, c := range tiles {
		area += float64(c[2]-c[0]) * float64(c[3]-c[1])
	}
	if math.Abs(area-240000) > 1 {
		t.Errorf("tiles cover %v", area)
	}
	for i, c := range tiles {
		want := values[i] / 24 * 240000
		if got := float64(c[2]-c[0]) * float64(c[3]-c[1]); math.Abs(got-want) > 1 {
			t.Errorf("tile %d: %v, want %v", i, got, want)
		}
	}
}

func TestTreeLayoutPadsAndFinds(t *testing.T) {
	tree := newTree([]string{"g", "h", "a", "b", "c"}, []string{"", "", "g", "g", "h"}, []float64{0, 0, 3, 1, 4})
	l := layOutTree(tree, treemapOptions{width: 200, height: 100, ratio: math.Phi, paddingInner: []float32{4}, paddingTop: []float32{10}})
	if len(l.nodes) != 5 || len(l.roots) != 2 {
		t.Fatalf("nodes %d roots %d", len(l.nodes), len(l.roots))
	}
	g, _ := l.of(0)
	if !g.group || g.header != 10 || g.last != g.first+2 {
		t.Errorf("group %+v", g)
	}
	// In its band the pointer finds the group, in its content a leaf.
	if row, ok := l.at((g.x0+g.x1)/2, g.y0+2, 3); !ok || row != 0 {
		t.Errorf("band finds %d", row)
	}
	a, _ := l.of(2)
	cx, cy := a.center()
	if row, ok := l.at(cx, cy, 3); !ok || row != 2 {
		t.Errorf("leaf finds %d", row)
	}
	if row, ok := l.step(2, stepFirst); !ok || row != l.nodes[1].row {
		t.Errorf("Home finds %d", row)
	}
	if row, ok := l.step(-1, stepLast); !ok || tree.isGroup(row) {
		t.Errorf("End finds %d", row)
	}
	// A value of nothing takes no area.
	none := newTree([]string{"a", "b"}, []string{"", ""}, []float64{0, 5})
	if l := layOutTree(none, treemapOptions{width: 100, height: 100, ratio: 1}); len(l.nodes) != 1 {
		t.Errorf("%d nodes", len(l.nodes))
	}
}

func TestPosesLerp(t *testing.T) {
	nan := math.NaN()
	a := dotPose{{0, 0, 4}, {nan, 0, 2}}
	b := dotPose{{10, 10, 8}}
	mid := a.lerp(b, 0.5)
	if mid[0].x != 5 || mid[0].r != 6 || mid[1].r != 1 {
		t.Errorf("dots %+v", mid)
	}
	p := treePose{{id: "x", rect: [4]float32{0, 0, 10, 10}}, {id: "gone", rect: [4]float32{0, 0, 10, 10}}}
	q := treePose{{id: "x", rect: [4]float32{10, 0, 20, 10}}, {id: "new", rect: [4]float32{0, 0, 10, 10}}}
	out := p.lerp(q, 0.5)
	if out[0].rect[0] != 5 || out[1].rect[0] != 2.5 || len(out) != 3 || out[2].rect[0] != 2.5 {
		t.Errorf("tree %+v", out)
	}
	c := cellPose{{0, 0, 1, 1, 0}}.lerp(cellPose{{1, 0, 1, 1, 0}}, 0.25)
	if c[0].x != 0.25 {
		t.Errorf("cell %+v", c)
	}
	pie := piePose{slices: []pieSlice{{0, 0, 1}}}.lerp(piePose{slices: []pieSlice{{0, 0, 2}, {1, 2, 3}}}, 0.5)
	if pie.slices[0].end != 1.5 || pie.slices[1].start != 1.5 {
		t.Errorf("pie %+v", pie.slices)
	}
}
