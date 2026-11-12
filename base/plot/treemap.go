package plot

import (
	"log"
	"math"
	"slices"
	"strconv"

	"github.com/nsgo-dev/nsgo/base/internal/parts"
	"github.com/nsgo-dev/nsgo/core"
)

// TreemapSort is the order a Treemap's siblings are tiled in, from its top
// left corner.
type TreemapSort uint8

const (
	// TreemapByValue is largest first, equal values in the rows' order: the
	// tiles nearest to square.
	TreemapByValue TreemapSort = iota
	// TreemapByRow is the rows' order: the nodes keep their places as their
	// values change, at the cost of less square tiles.
	TreemapByRow
)

// TreemapLabelPlacement is where a Treemap's labels go in their leaves.
type TreemapLabelPlacement uint8

const (
	// TreemapLabelStart is at the leaf's top left.
	TreemapLabelStart TreemapLabelPlacement = iota
	// TreemapLabelCenter is in the leaf's middle, the lines centered.
	TreemapLabelCenter
)

// Treemap is a treemap of a Plot's rows, as d3's treemap of a stratify
// hierarchy: one rectangle per node, its rows made a tree by their ID and
// Parent, each leaf's area in proportion to its Value and each group the box
// of what is under it, squarified in order of value, with the gaps of
// PaddingInner, the room of PaddingOuter and the band of PaddingTop per
// level. A node's color is its row's of the plot's ColorScale: of its
// ColorValue along a ramp, of its Color category, or its Fill where the app
// sets one. Labels write each leaf's name and value and each group's name in
// its band, where they fit. The pointer finds the leaf under it (across a
// gap, the nearest; in a band or padding, the group), which the plot reports
// and marks with the hover box, the other nodes fading by HoverDim; the
// arrows step to the leaf beside on the screen, Home and End to the first
// and last, and screen readers through the rows. The plot's x and y scales
// are not used. With the plot's appear, the top-level nodes grow from their
// middles, one after another with Stagger; with Motion.Update, the nodes
// glide to their new boxes and colors, known by their IDs, new ones grow
// from their middles and those that are gone shrink to theirs.
type Treemap struct {
	// ID is the id of each row's node, its index without one: what Parent
	// names, and what a node is known by when the rows change.
	ID func() []string
	// Parent is the id of each row's parent: a row whose parent is empty, or
	// names no row, is a top-level node; without one, every row is. Rows that
	// others name are groups, the rest leaves; a row that is its own ancestor
	// is left out.
	Parent func() []string
	// Value is the size of each leaf. A group's is the sum of its children's.
	// A leaf whose value is 0, below 0 or not a number takes no area.
	Value func() []float64
	Sort  TreemapSort
	// Ratio is the aspect ratio the tiles approach, at least 1; the golden
	// ratio without it.
	Ratio float64
	// PaddingInner is the gap between siblings, PaddingOuter the room inside
	// each group around its children and PaddingTop the band at the top of
	// each group, GroupLabel written in it, by level from the top one down,
	// the last for those below. A level's band goes on all the siblings of a
	// parent or none of them, so the groups of a level read alike.
	PaddingInner, PaddingOuter, PaddingTop []core.Points
	// Corners rounds its rectangles' corners; square by default.
	Corners Corners
	// Color is the category of each node, which an ordinal ColorScale gives
	// a color: the node's Fill where nothing sets one.
	Color func() []string
	// ColorValue is the number of each node, which a ramp of the plot's
	// ColorScale makes its color, over the extent of the rows' values
	// without a domain.
	ColorValue func() []float64
	// LeafStyle is every leaf's paint, and LeafClassOf gives a leaf of a row
	// its own classes. The leaves that share classes share a canvas.
	LeafClass   string
	LeafClassOf func(row int) string
	LeafStyle   core.Style
	// GroupStyle and GroupClassOf are a group's: a rectangle under its
	// children, showing in the gaps, its padding and its band. Drawn only
	// with a class or a style.
	GroupClass   string
	GroupClassOf func(row int) string
	GroupStyle   core.Style
	// HoverDim is how much the nodes away from the one under the pointer,
	// and their labels, fade, as a share of their opacity: a leaf keeps its
	// own and its groups', a group its own and what is under it.
	HoverDim float32
	// Label is the name in each leaf, on one line, cut with an ellipsis
	// where the leaf is narrower.
	Label func() []string
	// ValueLabel is the value in each leaf, the line under its name, of the
	// leaf's row and its value; shown only where it fits whole.
	ValueLabel     func(row int, value float64) string
	LabelPlacement TreemapLabelPlacement
	// LabelStyle styles each leaf's label: a box as big as the leaf holding
	// its lines, its padding the room from the leaf's edges. A label is shown
	// only where its first line fits inside the box's padding. DarkLabelStyle
	// merges over it where the leaf's color is dark.
	LabelClass      string
	LabelStyle      core.Style
	DarkLabelClass  string
	DarkLabelStyle  core.Style
	ValueLabelClass string
	ValueLabelStyle core.Style
	// GroupLabel is the name of each group, written in its band, on one
	// line, cut with an ellipsis; GroupLabelStyle styles a box as big as the
	// band.
	GroupLabel      func() []string
	GroupLabelClass string
	GroupLabelStyle core.Style
	// Stagger is the share of the plot's appear over which its top-level
	// nodes start, in their tiling order.
	Stagger float32
	// HoverStyle styles a view as big as the node under the pointer or the
	// keys, over it; none without a class or style.
	HoverClass     string
	HoverStyle     core.Style
	HoverStyleFunc func() core.Style
	Class          string
	Style          core.Style
	StyleFunc      func() core.Style
	TestID         string
}

// treeSpot is a node as drawn: its row and id, what it is known by across
// changes, its box x0, y0, x1, y1, its color and classes, and where it is in
// the tree: root is the place of its top-level node among them, which the
// appear staggers by, first and last its span of the layout's pre-order.
type treeSpot struct {
	id          string
	row         int
	rect        [4]float32
	color       core.Color
	class       string
	group       bool
	depth, root int
	first, last int
	header      float32
}

// layer is the canvas it draws on.
func (s treeSpot) layer() treeLayer {
	l := treeLayer{group: s.group, class: s.class}
	if s.group {
		l.depth = s.depth
	}
	return l
}

// scaled is the same node shrunk to t of its size about its middle.
func (s treeSpot) scaled(t float32) treeSpot {
	cx, cy := (s.rect[0]+s.rect[2])/2, (s.rect[1]+s.rect[3])/2
	s.rect = [4]float32{cx + (s.rect[0]-cx)*t, cy + (s.rect[1]-cy)*t, cx + (s.rect[2]-cx)*t, cy + (s.rect[3]-cy)*t}
	return s
}

// treeLayer is a canvas's key: a part, a level and classes.
type treeLayer struct {
	group bool
	depth int
	class string
}

// treePose is the nodes of a treemap in its layout's pre-order.
type treePose []treeSpot

func (p treePose) same(o treePose) bool { return slices.Equal(p, o) }

func (p treePose) find(id string) (treeSpot, bool) {
	for _, s := range p {
		if s.id == id {
			return s, true
		}
	}
	return treeSpot{}, false
}

// lerp is the pose drawn t of the way from this one to to, its nodes known
// by their ids: new ones growing, gone ones shrinking.
func (p treePose) lerp(to treePose, t float32) treePose {
	mix := func(a, b float32) float32 { return a + (b-a)*t }
	out := make(treePose, 0, len(to))
	for _, b := range to {
		a, ok := p.find(b.id)
		if !ok {
			out = append(out, b.scaled(t))
			continue
		}
		s := b
		for i := range s.rect {
			s.rect[i] = mix(a.rect[i], b.rect[i])
		}
		if a.color != 0 && b.color != 0 {
			s.color = mixOKLab(a.color, b.color, t)
		}
		out = append(out, s)
	}
	for _, a := range p {
		if _, ok := to.find(a.id); !ok {
			out = append(out, a.scaled(1-t))
		}
	}
	return out
}

// treeLayers is the canvases to and from need: one per part, level and
// classes, groups under leaves.
func treeLayers(to, from treePose, groups bool) []treeLayer {
	var out []treeLayer
	for _, s := range slices.Concat(to, from) {
		if s.group && !groups {
			continue
		}
		if l := s.layer(); !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	slices.SortStableFunc(out, func(a, b treeLayer) int {
		switch {
		case a.group != b.group:
			if a.group {
				return -1
			}
			return 1
		}
		return a.depth - b.depth
	})
	return out
}

func warnTree(problems []treeProblem) {
	for _, p := range problems {
		switch p.kind {
		case problemDuplicate:
			log.Printf("base: Treemap: row %d's id %q is an earlier row's; nothing names it as a parent", p.row, p.id)
		case problemUnknownParent:
			log.Printf("base: Treemap: row %d's parent %q is no row's id; it is a top-level node", p.row, p.id)
		case problemCycle:
			log.Printf("base: Treemap: row %d is its own ancestor; it and what is under it are not drawn", p.row)
		}
	}
}

func levels(padding []core.Points) []float32 {
	out := make([]float32, len(padding))
	for i, p := range padding {
		out[i] = float32(p)
	}
	return out
}

// Render implements core.Node.
func (t Treemap) Render(cx *core.Scope) core.Node {
	own := part{Class: t.Class, Style: t.Style, Fn: t.StyleFunc}
	plot := plotOf(cx, "Treemap")
	if plot == nil {
		return markLayer(own, t.TestID)
	}
	ids := func() []string {
		if t.ID != nil {
			return t.ID()
		}
		out := make([]string, plot.rows())
		for i := range out {
			out[i] = strconv.Itoa(i)
		}
		return out
	}
	var warned []treeProblem
	tree := core.NewMemoFunc(cx, func() *treemapTree {
		if t.Value == nil {
			return nil
		}
		id := ids()
		parents := make([]string, len(id))
		if t.Parent != nil {
			parents = t.Parent()
		}
		tree := newTree(id, parents, t.Value())
		if !slices.Equal(warned, tree.problems) {
			warnTree(tree.problems)
			warned = tree.problems
		}
		return tree
	}, nil)
	ratio := t.Ratio
	if ratio == 0 {
		ratio = math.Phi
	}
	inner, outer, top := levels(t.PaddingInner), levels(t.PaddingOuter), levels(t.PaddingTop)
	laid := core.NewMemoFunc(cx, func() *treeLayout {
		if !plot.sized() {
			return nil
		}
		tr := tree.Get()
		if tr == nil {
			return nil
		}
		size := plot.size.Get()
		return layOutTree(tr, treemapOptions{float32(size.Width), float32(size.Height), ratio, t.Sort, inner, outer, top})
	}, nil)
	moves := newMoves(cx, plot, true, func() (treePose, bool) {
		l := laid.Get()
		if l == nil {
			return nil, false
		}
		id := ids()
		var categories, shades []core.Color
		if t.Color != nil {
			categories, _ = plot.categoryColors(t.Color())
		}
		if t.ColorValue != nil {
			numbers := t.ColorValue()
			shades, _ = plot.shades(numbers, extentOf(numbers))
		}
		colorAt := func(c []core.Color, row int) core.Color {
			if row < len(c) {
				return c[row]
			}
			return 0
		}
		root := 0
		out := make(treePose, len(l.nodes))
		for place, n := range l.nodes {
			if n.depth == 0 {
				root = max(slices.Index(l.roots, place), 0)
			}
			class, classOf := t.LeafClass, t.LeafClassOf
			if n.group {
				class, classOf = t.GroupClass, t.GroupClassOf
			}
			if classOf != nil {
				class = parts.JoinClasses(class, classOf(n.row))
			}
			color := colorAt(categories, n.row)
			if color == 0 {
				color = colorAt(shades, n.row)
			}
			name := strconv.Itoa(n.row)
			if n.row < len(id) {
				name = id[n.row]
			}
			out[place] = treeSpot{name, n.row, [4]float32{n.x0, n.y0, n.x1, n.y1}, color, class, n.group, n.depth, root, n.first, n.last, n.header}
		}
		return out, true
	})
	rootCount := core.NewMemo(cx, func() int {
		if l := laid.Get(); l != nil {
			return len(l.roots)
		}
		return 0
	})
	// The pointer finds the node under it; the keys step across the leaves
	// on the screen.
	plot.addHit(cx, &plotHit{
		at: func(at core.Point) (int, bool) {
			l := laid.GetUntracked()
			if l == nil {
				return 0, false
			}
			return l.at(float32(at.X), float32(at.Y), atLevel(inner, 0)+1)
		},
		center: func(row int) (core.Point, bool) {
			l := laid.GetUntracked()
			if l == nil {
				return core.Point{}, false
			}
			n, ok := l.of(row)
			x, y := n.center()
			return core.Point{X: core.Points(x), Y: core.Points(y)}, ok
		},
		grid: func(from int, step keyStep) (int, bool) {
			l := laid.GetUntracked()
			if l == nil {
				return 0, false
			}
			return l.step(from, step)
		},
	})
	// The hovered node's place in the layout, on the frame clock: the nodes
	// it holds or that hold it stay, the others fade.
	first, last := core.NewAnimatedValue(cx, -2), core.NewAnimatedValue(cx, -2)
	cx.Effect(func() {
		row := plot.hovered.Get()
		if row < 0 {
			return
		}
		n, ok := treeNode{}, false
		if l := laid.Get(); l != nil {
			n, ok = l.of(row)
		}
		if ok {
			first.Set(float64(n.first))
			last.Set(float64(n.last))
		} else {
			first.Set(-2)
			last.Set(-2)
		}
	})
	dim := min(max(t.HoverDim, 0), 1)
	if plot.noHover {
		dim = 0
	}
	// nested is 1 when the node from f to l is nested with the hovered one.
	opacity := func(f, l core.AnimatedNode) num {
		before := core.Interpolate(core.Sub(last, f), core.Interpolation{Input: []float64{-1, 0}, Output: []float64{0, 1}})
		after := core.Interpolate(core.Sub(l, first), core.Interpolation{Input: []float64{-1, 0}, Output: []float64{0, 1}})
		nested := core.Mul(before, after)
		off := core.Mul(core.Mul(core.Sub(core.Constant(1), nested), core.Constant(dim)), plot.fade)
		return nodeNum(core.Sub(core.Constant(1), off))
	}
	hasGroups := t.GroupClass != "" || t.GroupClassOf != nil || !t.GroupStyle.IsZero()
	canvases := core.NewMemoFunc(cx, func() []treeLayer {
		m := moves.now()
		if !m.has {
			return nil
		}
		var from treePose
		if m.moving {
			from = m.from
		}
		return treeLayers(m.to, from, hasGroups)
	}, slices.Equal)
	children := []core.Node{core.Each(canvases.Get, func(l treeLayer) treeLayer { return l },
		func(cx *core.Scope, item func() treeLayer, _ func() int) core.Node {
			l := item()
			style := t.LeafStyle
			if l.group {
				style = t.GroupStyle
			}
			return markCanvas(plot, room, part{Class: l.class, Style: style}, func(d *core.Drawing, plot *plotState, _ float32) {
				d.StrokeFromView(core.PaintNone).LineWidthFromView().LineDashFromView()
				if plot == nil {
					return
				}
				m := moves.now()
				if m.has {
					t.draw(d, plot, l, m, moves.progress, core.Untracked(cx, rootCount.Get), dim, opacity)
				}
			})
		})}
	children = append(children, t.labels(cx, plot, moves, tree, rootCount, dim, opacity)...)
	if hover := (part{Class: t.HoverClass, Style: t.HoverStyle, Fn: t.HoverStyleFunc}); !plot.noHover && hover.Set() {
		h := plot.hoverPlace(cx, func(row int) (core.Bounds, bool) {
			l := laid.Get()
			if l == nil {
				return core.Bounds{}, false
			}
			n, ok := l.of(row)
			return core.Bounds{Origin: core.Point{X: core.Points(n.x0), Y: core.Points(n.y0)},
				Size: core.Size{Width: core.Points(n.x1 - n.x0), Height: core.Points(n.y1 - n.y0)}}, ok
		})
		children = append(children, h.box(plot, hover.Class, hover.Style, hover.Fn))
	}
	return markLayer(own, t.TestID, children...)
}

// draw draws the nodes of layer of the pose shown, moving to their boxes and
// colors now, grown with the plot's appear and faded away from the hovered
// node.
func (t Treemap) draw(d *core.Drawing, plot *plotState, layer treeLayer, m moveState[treePose], progress core.AnimatedNode,
	roots int, dim float32, opacity func(f, l core.AnimatedNode) num) {
	var q core.AnimatedNode
	if m.moving {
		q = progress
	}
	appearing := plot.appearing.Get()
	paint := func(c core.Color) core.Paint { return paintOf(c, core.PaintCurrentColor) }
	one := func(spot treeSpot, gone bool) {
		if spot.layer() != layer {
			return
		}
		a, b := spot.rect, spot.rect
		was, had := treeSpot{}, false
		switch {
		case q != nil && gone:
			b = spot.scaled(0).rect
		case q != nil:
			if was, had = m.from.find(spot.id); had {
				a = was.rect
			} else {
				a = spot.scaled(0).rect
			}
		}
		x0, y0 := lerpNum(q, a[0], b[0]), lerpNum(q, a[1], b[1])
		x1, y1 := lerpNum(q, a[2], b[2]), lerpNum(q, a[3], b[3])
		x, y, w, h := x0, y0, x1.sub(x0), y1.sub(y0)
		if appearing && !gone {
			g := nodeNum(plot.grown(spot.root, roots, t.Stagger))
			left := g.mul(constNum(-0.5)).add(constNum(0.5))
			x, y, w, h = x.add(w.mul(left)), y.add(h.mul(left)), w.mul(g), h.mul(g)
		}
		alpha, dims := num{}, dim > 0 && !gone
		if dims {
			alpha = opacity(core.Constant(spot.first), core.Constant(spot.last))
		}
		// A color that changes fades to the new one.
		fades := q != nil && !gone && had && was.color != spot.color
		if fades {
			if dims {
				setAlpha(d, alpha)
			} else {
				d.SetAlpha(1)
			}
			d.FillFromView(paint(was.color)).BeginPath()
			rectPath(d, x, y, w, h, t.Corners)
			d.Fill()
		}
		switch {
		case dims && fades:
			setAlpha(d, alpha.mul(nodeNum(q)))
		case dims:
			setAlpha(d, alpha)
		case fades:
			setAlpha(d, nodeNum(q))
		default:
			d.SetAlpha(1)
		}
		d.FillFromView(paint(spot.color)).BeginPath()
		rectPath(d, x, y, w, h, t.Corners)
		d.Fill()
		d.Stroke()
	}
	for _, s := range m.to {
		one(s, false)
	}
	// Then those that are gone, shrinking away.
	if q != nil {
		for _, s := range m.from {
			if _, ok := m.to.find(s.id); !ok {
				one(s, true)
			}
		}
	}
}

// treeLabelKey is a label's key: its row, and whether it is a group's.
type treeLabelKey struct {
	row   int
	group bool
}

// labels is the labels of the nodes of the pose shown that have one: a leaf
// with a name, a group with a band and a name.
func (t Treemap) labels(cx *core.Scope, plot *plotState, moves *moves[treePose], tree *core.Memo[*treemapTree],
	rootCount *core.Memo[int], dim float32, opacity func(f, l core.AnimatedNode) num) []core.Node {
	if t.Label == nil && t.GroupLabel == nil {
		return nil
	}
	keys := func() []treeLabelKey {
		m := moves.now()
		var out []treeLabelKey
		for _, s := range m.to {
			if s.group && s.header > 0 && t.GroupLabel != nil && textAt(t.GroupLabel, s.row) != "" ||
				!s.group && t.Label != nil && textAt(t.Label, s.row) != "" {
				out = append(out, treeLabelKey{s.row, s.group})
			}
		}
		return out
	}
	return []core.Node{core.Each(keys, func(k treeLabelKey) treeLabelKey { return k },
		func(cx *core.Scope, item func() treeLabelKey, _ func() int) core.Node {
			return t.label(cx, plot, moves, tree, rootCount, item(), dim, opacity)
		})}
}

// label is the label of the node of key: a box as big as its leaf or its
// band, at its place, riding its node through Motion.Update, its lines shown
// where they fit.
func (t Treemap) label(cx *core.Scope, plot *plotState, moves *moves[treePose], tree *core.Memo[*treemapTree],
	rootCount *core.Memo[int], key treeLabelKey, dim float32, opacity func(f, l core.AnimatedNode) num) core.Node {
	row, group := key.row, key.group
	spot := func() (treeSpot, bool) {
		for _, s := range moves.now().to {
			if s.row == row && s.group == group {
				return s, true
			}
		}
		return treeSpot{}, false
	}
	band := func(s treeSpot) [4]float32 {
		if group {
			return [4]float32{s.rect[0], s.rect[1], s.rect[2], s.rect[1] + s.header}
		}
		return s.rect
	}
	// Its box at rest, and the one it holds while its node moves: no bigger
	// than its node is at any time of the move.
	type boxes struct {
		b            [4]float32
		w, h         float32
		awayX, awayY float32
		ok           bool
	}
	box := core.NewMemo(cx, func() boxes {
		at, ok := spot()
		if !ok {
			return boxes{}
		}
		b, a := band(at), band(at)
		if m := moves.now(); m.moving {
			if was, had := m.from.find(at.id); had {
				a = band(was)
			}
		}
		return boxes{b, min(b[2]-b[0], a[2]-a[0]), min(b[3]-b[1], a[3]-a[1]), a[0] - b[0], a[1] - b[1], true}
	})
	// How far it is from its place while its node moves there.
	awayX, awayY := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	cx.Effect(func() {
		b := box.Get()
		awayX.Set(float64(b.awayX))
		awayY.Set(float64(b.awayY))
	})
	left := moves.left()
	// Its node's span of the layout, on the frame clock, for the hover's
	// fade.
	spanFirst, spanLast := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	cx.Effect(func() {
		if s, ok := spot(); ok {
			spanFirst.Set(float64(s.first))
			spanLast.Set(float64(s.last))
		}
	})
	var fade core.AnimatedNode
	join := func(n core.AnimatedNode) {
		if fade == nil {
			fade = n
		} else {
			fade = core.Mul(fade, n)
		}
	}
	var s0 treeSpot
	core.Untracked(cx, func() struct{} {
		s0, _ = spot()
		return struct{}{}
	})
	if core.Untracked(cx, plot.appearing.Get) {
		join(plot.grown(s0.root, core.Untracked(cx, rootCount.Get), t.Stagger))
	}
	if dim > 0 {
		join(opacity(spanFirst, spanLast).n)
	}
	// A node new to the layout fades in as it grows.
	if m := core.Untracked(cx, func() moveState[treePose] { return moves.now() }); m.moving {
		if _, had := m.from.find(s0.id); !had {
			join(moves.progress)
		}
	}
	animated := &core.AnimatedStyle{TranslateX: core.Mul(left, awayX), TranslateY: core.Mul(left, awayY), Opacity: fade}
	dark := func() bool {
		s, ok := spot()
		return ok && s.color != 0 && isDark(s.color)
	}
	// What decides which lines show: the lines' sizes in the box.
	content, name, value := core.NewSignal(cx, core.Size{}), core.NewSignal(cx, core.Size{}), core.NewSignal(cx, core.Size{})
	nameFits := core.NewMemo(cx, func() bool {
		c, h := content.Get(), name.Get().Height
		return h > 0 && h <= c.Height+0.5 && c.Width >= 2*h
	})
	valueFits := core.NewMemo(cx, func() bool {
		c, n, v := content.Get(), name.Get(), value.Get()
		return nameFits.Get() && v.Height > 0 && n.Height+v.Height <= c.Height+0.5 && v.Width <= c.Width+0.5
	})
	center := !group && t.LabelPlacement == TreemapLabelCenter
	names := t.Label
	class, style := t.LabelClass, t.LabelStyle
	if group {
		names, class, style = t.GroupLabel, t.GroupLabelClass, t.GroupLabelStyle
	}
	lines := []core.Node{core.Text{ContentFunc: func() string { return textAt(names, row) }, MaxLines: 1,
		Style: core.Style{MaxWidth: core.Pct(100)}, OnLayout: measure(name),
		StyleFunc: func() core.Style {
			if nameFits.Get() {
				return core.Style{}
			}
			return core.Style{Opacity: core.Alpha(0)}
		}}}
	if !group && t.ValueLabel != nil {
		// A number is shown whole or not at all; where it does not fit, it
		// leaves the name its place.
		lines = append(lines, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignStart},
			StyleFunc: func() core.Style {
				if valueFits.Get() {
					return core.Style{}
				}
				return core.Style{Height: pts(0), Opacity: core.Alpha(0)}
			},
			Children: []core.Node{core.Text{ContentFunc: func() string {
				v := 0.0
				if tr := tree.Get(); tr != nil && row < len(tr.value) {
					v = tr.value[row]
				}
				return t.ValueLabel(row, v)
			}, OnLayout: measure(value), Class: t.ValueLabelClass, Style: t.ValueLabelStyle}}})
	}
	align, justify := core.AlignStart, core.JustifyStart
	if center {
		align, justify = core.AlignCenter, core.JustifyCenter
	}
	return core.View{
		Class: class,
		ClassFunc: func() string {
			if !group && dark() {
				return t.DarkLabelClass
			}
			return ""
		},
		Style: core.Style{Position: core.Absolute, Overflow: core.Hidden, Animated: animated}.Merge(style),
		StyleFunc: func() core.Style {
			b := box.Get()
			if !b.ok {
				return core.Style{Display: core.DisplayNone}
			}
			out := core.Style{Left: pts(b.b[0]), Top: pts(b.b[1]), Width: pts(max(b.w, 0)), Height: pts(max(b.h, 0))}
			if !group && dark() {
				out = out.Merge(t.DarkLabelStyle)
			}
			return out
		},
		PointerEvents: core.PointerEventsNone,
		Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1, Overflow: core.Hidden, AlignItems: align, JustifyContent: justify},
			OnLayout: measure(content), Children: lines}},
	}
}
