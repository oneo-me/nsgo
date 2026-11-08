package flow

import (
	"math"
	"time"

	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
)

const (
	defaultNodeGap   = 40
	defaultRankGap   = 72
	edgeGap          = 24
	fallbackWidth    = 200
	fallbackHeight   = 72
	planeRoom        = 1 << 14
	mutedNodeOpacity = .45
	// relayoutMotion is how long after a change of topology or spacing
	// new node sizes still move nodes rather than placing them.
	relayoutMotion = 600 * time.Millisecond
	ringGap        = 3
	ringWidth      = 2
	overlayInset   = 8
)

// nodeSpring is how nodes move to a new layout: critically damped,
// arriving in about half a second.
var nodeSpring = core.SpringConfig{Stiffness: 157.91, Damping: 25.13, Mass: 1}

type view struct {
	cx   *core.Scope
	s    *State
	spec View

	nodes []renderNode
	edges []renderEdge
	index map[string]int

	direction Direction
	gaps      spacing
	layoutKey uint64
	result    layoutResult
	// structureKey hashes the topology and spacing, which last changed at
	// structureAt.
	structureKey uint64
	structureAt  time.Time

	width, height float64
	camX, camY    *core.AnimatedValue
	zoom          *core.AnimatedValue
	cameraRun     *core.AnimationRun
	anims         map[string]*nodeAnim
	moveRun       *core.AnimationRun
	moves         int

	// Signals: tick runs prepare again, laid out follows the result,
	// moving the nodes' motion to a new layout.
	tick      *core.Signal[int]
	laidOut   *core.Signal[int]
	measured  *core.Signal[int]
	ready     *core.Signal[bool]
	moving    *core.Signal[bool]
	hovered   *core.Signal[string]
	panning   *core.Signal[bool]
	focused   string
	pinchFrom float64
	// lastPress is the node the last press reached, "" for empty space:
	// a double press opens or zooms only where both presses went.
	lastPress string
}

// nodeAnim is where a node shows, which the platform moves to its laid out
// place.
type nodeAnim struct {
	scope  *core.Scope
	x, y   *core.AnimatedValue
	placed bool
}

type renderNode struct {
	id, label string
	muted     bool
	fixedW    float64
	fixedH    float64
	// width and height are the measured size, valid once measured.
	width, height float64
	measured      bool
}

func (n *renderNode) size() (float64, float64) {
	if n.measured {
		return n.width, n.height
	}
	w, h := n.fixedW, n.fixedH
	if w <= 0 {
		w = fallbackWidth
	}
	if h <= 0 {
		h = fallbackHeight
	}
	return w, h
}

type renderEdge struct {
	source, target int
	label          string
	dashed         bool
	color          core.Color
	width          float64
}

// nodeItem is what a node's box shows of it.
type nodeItem struct {
	id, label     string
	muted         bool
	width, height core.Points
}

// Render implements core.Node.
func (spec View) Render(cx *core.Scope) core.Node {
	state := spec.State
	if state == nil {
		state = &State{}
	}
	v := &view{cx: cx, s: state, spec: spec, anims: map[string]*nodeAnim{},
		tick: core.NewSignal(cx, 0), laidOut: core.NewSignal(cx, 0), measured: core.NewSignal(cx, 0),
		ready: core.NewSignal(cx, false), moving: core.NewSignal(cx, false),
		hovered: core.NewSignal(cx, ""), panning: core.NewSignal(cx, false)}
	if state.sizes == nil {
		state.sizes = map[string]core.Size{}
	}
	if state.cam.zoom == 0 {
		state.cam.zoom = 1
	}
	v.camX, v.camY, v.zoom = core.NewAnimatedValue(cx, state.cam.x), core.NewAnimatedValue(cx, state.cam.y), core.NewAnimatedValue(cx, state.cam.zoom)
	state.v = v
	cx.Cleanup(func() {
		if state.v == v {
			state.v = nil
		}
	})
	data := func() Data {
		if spec.Data == nil {
			return Data{}
		}
		return spec.Data()
	}
	items := core.NewMemoFunc(cx, func() []nodeItem {
		d := data()
		seen := map[string]bool{}
		out := make([]nodeItem, 0, len(d.Nodes))
		for _, n := range d.Nodes {
			if n.ID == "" || seen[n.ID] {
				continue
			}
			seen[n.ID] = true
			label := n.Label
			if label == "" {
				label = n.ID
			}
			out = append(out, nodeItem{id: n.ID, label: label, muted: n.Muted, width: max(0, n.Width), height: max(0, n.Height)})
		}
		return out
	}, func(a, b []nodeItem) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	})
	cx.Effect(func() {
		d := data()
		direction := TopToBottom
		if spec.Direction != nil {
			direction = spec.Direction()
		}
		v.tick.Get()
		cx.Untrack(func() {
			v.update(d, direction)
			v.prepare(cx.Now())
		})
	})
	empty := func() bool { return len(items.Get()) == 0 }
	return core.View{Class: spec.Class, TestID: spec.TestID,
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0), Overflow: core.Hidden}.Merge(spec.Style),
		Children: []core.Node{core.Show{When: empty, Then: func(cx *core.Scope) core.Node {
			content := spec.Empty
			if content == nil {
				content = core.Text{Content: "—", StyleFunc: func() core.Style { return core.Style{Color: cx.Palette().TextMuted} }}
			}
			return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Padding: core.All(24)},
				Children: []core.Node{content}}
		}, Else: func(cx *core.Scope) core.Node { return v.diagram(cx, items) }}}}
}

func (v *view) reduceMotion() bool { return core.Untracked(v.cx, v.cx.Env().ReduceMotion) }

func (v *view) interactive() bool { return v.spec.OnSelect != nil || v.spec.OnOpen != nil }

func (v *view) selected() string {
	if v.spec.Selected == nil {
		return ""
	}
	return v.spec.Selected()
}

func (v *view) look() Look {
	var l Look
	if v.spec.LookFunc != nil {
		l = v.spec.LookFunc()
	}
	p := v.cx.Palette()
	zero := core.Color(0)
	if l.Line == zero {
		l.Line = p.Text.Faded(.35)
	}
	if l.Emphasis == zero {
		l.Emphasis = p.Text
	}
	if l.Label == zero {
		l.Label = p.TextMuted
	}
	if l.LabelBackground == zero {
		l.LabelBackground = p.Background
	}
	if l.LabelBorder == zero {
		l.LabelBorder = p.Text.Faded(.2)
	}
	return l
}

// diagram is the camera's plane of edges and nodes under the layer taking
// the pointer.
func (v *view) diagram(cx *core.Scope, items *core.Memo[[]nodeItem]) core.Node {
	label := func() string {
		if v.spec.LabelFunc != nil {
			return v.spec.LabelFunc()
		}
		if v.spec.Label != "" {
			return v.spec.Label
		}
		v.laidOut.Get()
		return locales.FlowLabel(cx, len(v.nodes), len(v.edges))
	}
	plane := core.View{PointerEvents: core.PointerEventsBoxNone,
		StyleFunc: func() core.Style {
			v.laidOut.Get()
			s := core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0),
				Width: core.Pt(core.Points(v.result.width) + planeRoom), Height: core.Pt(core.Points(v.result.height) + planeRoom),
				TransformOrigin: core.ObjectPosition{X: core.PositionAxis{Offset: core.Pt(0)}, Y: core.PositionAxis{Offset: core.Pt(0)}},
				Animated:        &core.AnimatedStyle{TranslateX: v.camX, TranslateY: v.camY, Scale: v.zoom}}
			if !v.ready.Get() {
				s.Opacity = core.Alpha(0)
			}
			return s
		},
		Children: []core.Node{v.edgeCanvas(),
			core.Each(items.Get, func(n nodeItem) string { return n.id }, func(cx *core.Scope, item func() nodeItem, _ func() int) core.Node {
				return v.nodeBox(cx, item)
			})}}
	var overlay core.Node = core.Group{}
	if v.spec.Overlay != nil {
		overlay = core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(overlayInset), Right: core.Pt(overlayInset)},
			Children: []core.Node{v.spec.Overlay}}
	}
	return core.View{Role: core.RoleGroup, LabelFunc: label,
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)},
		OnLayout: func(b core.Bounds) {
			v.resize(float64(b.Size.Width), float64(b.Size.Height))
			v.tick.Update(func(n int) int { return n + 1 })
		},
		Children: []core.Node{plane, v.pointerLayer(), overlay}}
}

// nodeBox places a node's content where the platform moves it, and takes
// the keyboard and assistive technology's presses.
func (v *view) nodeBox(cx *core.Scope, item func() nodeItem) core.Node {
	id := core.Untracked(cx, item).id
	a := v.anim(id)
	var content core.Node = core.Group{}
	if v.spec.Data != nil {
		for _, n := range core.Untracked(cx, v.spec.Data).Nodes {
			if n.ID == id {
				if n.Content != nil {
					content = n.Content
				}
				break
			}
		}
	}
	measured := func() bool {
		v.measured.Get()
		_, ok := v.s.sizes[id]
		return ok
	}
	var focusable func() bool
	if v.interactive() {
		focusable = func() bool { return true }
	}
	role := core.RoleNone
	if v.interactive() {
		role = core.RoleButton
	}
	return core.View{Class: v.spec.NodeClass, Role: role, Focusable: focusable, NoFocus: !v.interactive(),
		LabelFunc: func() string { return item().label },
		Selected:  func() bool { return v.selected() == id },
		OnPress:   v.pressHandler(id),
		OnFocus: func() {
			if v.focused != id {
				v.focused = id
				v.revealIfHidden(id)
			}
		},
		OnBlur: func() {
			if v.focused == id {
				v.focused = ""
			}
		},
		OnLayout: func(b core.Bounds) { v.measure(id, b.Size) },
		Style:    v.spec.NodeStyle,
		StyleFunc: func() core.Style {
			it := item()
			var s core.Style
			if v.spec.NodeStyleFunc != nil {
				s = v.spec.NodeStyleFunc()
			}
			s = s.Merge(core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0),
				Animated: &core.AnimatedStyle{TranslateX: a.x, TranslateY: a.y}})
			if it.width > 0 {
				s.Width = core.Pt(it.width)
			}
			if it.height > 0 {
				s.Height = core.Pt(it.height)
			}
			switch {
			case !measured():
				// Unmeasured nodes take their size before showing.
				s.Opacity = core.Alpha(0)
			case it.muted:
				s.Opacity = core.Alpha(mutedNodeOpacity)
			}
			return s
		},
		Children: []core.Node{content, core.Show{When: func() bool { return v.selected() == id }, Then: func(*core.Scope) core.Node {
			return core.View{Class: v.spec.RingClass, PointerEvents: core.PointerEventsNone,
				Style: core.Style{BorderWidth: ringWidth}.Merge(v.spec.RingStyle).Merge(core.Style{Position: core.Absolute,
					Left: core.Pt(-ringGap), Top: core.Pt(-ringGap), Right: core.Pt(-ringGap), Bottom: core.Pt(-ringGap)}),
				StyleFunc: func() core.Style {
					s := core.Style{}
					if v.spec.RingStyle.BorderColor == 0 {
						s.BorderColor = cx.Palette().Text
					}
					if v.spec.RingStyleFunc != nil {
						s = s.Merge(v.spec.RingStyleFunc())
					}
					return s
				}}
		}}}}
}

// pressHandler selects a node a press reaches and opens it on a double
// press.
func (v *view) pressHandler(id string) func(core.PressEvent) {
	if !v.interactive() {
		return nil
	}
	return func(e core.PressEvent) {
		again := v.lastPress == id
		v.lastPress = id
		if v.spec.OnSelect != nil {
			v.spec.OnSelect(id)
		}
		if e.Clicks == 2 && again && v.spec.OnOpen != nil {
			v.spec.OnOpen(id)
		}
	}
}

// pointerLayer takes the pointer over the whole diagram and finds the
// node under it itself, as the plane's nodes show where the camera's
// transform puts them: it pans when dragged, zooms on pinches and the
// wheel, and presses or hovers the node under the pointer.
func (v *view) pointerLayer() core.Node {
	press := func(e core.PressEvent) {
		if i := v.hit(e.Position); i >= 0 {
			if f := v.pressHandler(v.nodes[i].id); f != nil {
				f(e)
			}
			return
		}
		again := v.lastPress == ""
		v.lastPress = ""
		if e.Clicks >= 2 && again {
			factor := 2.0
			if e.Modifiers.Shift {
				factor = .5
			}
			v.zoomAt(float64(e.Position.X), float64(e.Position.Y), factor, true)
		} else if v.spec.OnSelect != nil && v.selected() != "" {
			v.spec.OnSelect("")
		}
	}
	hover := func(e core.MouseEvent) {
		id := ""
		if e.PressedButton == core.MouseButtonNone {
			if i := v.hit(e.Position); i >= 0 {
				id = v.nodes[i].id
			}
		}
		v.hovered.Set(id)
	}
	return core.View{NoFocus: true, ImportantForAccessibility: core.ImportantNo,
		OnPress: press, OnMouseMove: hover, OnMouseLeave: func(core.MouseEvent) { v.hovered.Set("") },
		PanValues: core.AnimatedXY{X: v.camX, Y: v.camY},
		OnPan: func(e core.PanEvent) {
			switch e.Phase {
			case core.GestureBegan:
				if v.cameraRun != nil {
					v.cameraRun.Stop()
				}
				v.panning.Set(true)
				v.s.userMoved = true
			case core.GestureEnded, core.GestureCancelled:
				v.panning.Set(false)
				v.s.cam.x, v.s.cam.y = float64(e.Values.X), float64(e.Values.Y)
				// A finger let go glides on; a mouse stops where it is.
				if !core.Untracked(v.cx, v.cx.Env().Desktop) {
					v.glide(float64(e.Velocity.X), float64(e.Velocity.Y))
				}
			}
		},
		OnPinch: func(e core.GestureEvent) {
			switch e.Phase {
			case core.GestureBegan:
				v.pinchFrom = 1
			case core.GestureChanged:
				if v.pinchFrom > 0 && e.Value > 0 {
					v.zoomAt(float64(e.Position.X), float64(e.Position.Y), float64(e.Value)/v.pinchFrom, false)
					v.pinchFrom = float64(e.Value)
				}
			}
		},
		OnWheel: v.wheel,
		StyleFunc: func() core.Style {
			s := core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Cursor: core.CursorGrab}
			switch {
			case v.panning.Get():
				s.Cursor = core.CursorGrabbing
			case v.hovered.Get() != "" && v.interactive():
				// Nodes are pressables, which keep the platform's arrow.
				s.Cursor = core.CursorDefault
			}
			return s
		}}
}

// anim returns the place of the node with id, made on first use.
func (v *view) anim(id string) *nodeAnim {
	if a, ok := v.anims[id]; ok {
		return a
	}
	sc := v.cx.Child()
	a := &nodeAnim{scope: sc, x: core.NewAnimatedValue(sc, 0), y: core.NewAnimatedValue(sc, 0)}
	v.anims[id] = a
	return a
}

// measure records a node's laid out size, laying the diagram out again
// when it changes.
func (v *view) measure(id string, size core.Size) {
	i, ok := v.index[id]
	if !ok {
		return
	}
	n := &v.nodes[i]
	if n.fixedW > 0 {
		size.Width = core.Points(n.fixedW)
	}
	if n.fixedH > 0 {
		size.Height = core.Points(n.fixedH)
	}
	if old, ok := v.s.sizes[id]; ok && math.Abs(float64(old.Width-size.Width)) <= .25 && math.Abs(float64(old.Height-size.Height)) <= .25 {
		return
	}
	v.s.sizes[id] = size
	n.width, n.height, n.measured = float64(size.Width), float64(size.Height), true
	v.measured.Update(func(n int) int { return n + 1 })
	v.tick.Update(func(n int) int { return n + 1 })
}

// update reconciles application data with the retained nodes, keeping
// measured sizes by ID; layout follows in prepare.
func (v *view) update(data Data, direction Direction) {
	v.direction = direction
	v.gaps = spacing{node: defaultNodeGap, rank: defaultRankGap, edge: edgeGap}
	if v.spec.NodeGap > 0 {
		v.gaps.node = float64(v.spec.NodeGap)
	}
	if v.spec.RankGap > 0 {
		v.gaps.rank = float64(v.spec.RankGap)
	}
	index := make(map[string]int, len(data.Nodes))
	nodes := v.nodes[:0:0]
	for _, n := range data.Nodes {
		if _, ok := index[n.ID]; ok || n.ID == "" {
			continue
		}
		index[n.ID] = len(nodes)
		rn := renderNode{id: n.ID, label: n.Label, muted: n.Muted, fixedW: float64(max(0, n.Width)), fixedH: float64(max(0, n.Height))}
		if old, ok := v.index[n.ID]; ok && old < len(v.nodes) {
			if o := v.nodes[old]; o.fixedW != rn.fixedW || o.fixedH != rn.fixedH {
				delete(v.s.sizes, n.ID)
			}
		}
		if size, ok := v.s.sizes[n.ID]; ok {
			rn.width, rn.height, rn.measured = float64(size.Width), float64(size.Height), true
		}
		if rn.label == "" {
			rn.label = n.ID
		}
		nodes = append(nodes, rn)
	}
	for id, a := range v.anims {
		if _, ok := index[id]; !ok {
			a.scope.Dispose()
			delete(v.anims, id)
		}
	}
	edges := make([]renderEdge, 0, len(data.Edges))
	for _, e := range data.Edges {
		s, ok1 := index[e.Source]
		d, ok2 := index[e.Target]
		if !ok1 || !ok2 || s == d {
			continue
		}
		edges = append(edges, renderEdge{source: s, target: d, label: e.Label, dashed: e.Dashed, color: e.Color, width: float64(e.Width)})
	}
	v.nodes, v.edges, v.index = nodes, edges, index
}

// prepare lays the diagram out again when its topology, node sizes or
// spacing change, and shows it once every node is measured and the view
// has a size. Nodes and the camera move to a new topology or spacing, and
// to sizes measured while they do; other new sizes, such as those of
// content settling after the diagram appears, take effect at once.
func (v *view) prepare(now time.Time) {
	var h, structure hasher
	structure.reset()
	structure.int(int(v.direction))
	structure.float(v.gaps.node)
	structure.float(v.gaps.rank)
	for _, n := range v.nodes {
		structure.string(n.id)
	}
	for _, e := range v.edges {
		structure.int(e.source)
		structure.int(e.target)
		dashed := byte(0)
		if e.dashed {
			dashed = 1
		}
		structure.byte(dashed)
	}
	h.sum = structure.sum
	all := true
	for i := range v.nodes {
		n := &v.nodes[i]
		w, ht := n.size()
		h.float(math.Round(w * 2))
		h.float(math.Round(ht * 2))
		all = all && n.measured
	}
	ready := v.ready.GetUntracked()
	if structure.sum != v.structureKey {
		v.structureKey = structure.sum
		if ready {
			v.structureAt = now
		}
	}
	changed := false
	if h.sum != v.layoutKey || len(v.result.x) != len(v.nodes) {
		v.layoutKey = h.sum
		snap := now.Sub(v.structureAt) > relayoutMotion
		nodes := make([]layoutNode, len(v.nodes))
		for i := range v.nodes {
			w, ht := v.nodes[i].size()
			nodes[i] = layoutNode{w, ht}
		}
		edges := make([]layoutEdge, len(v.edges))
		for i, e := range v.edges {
			edges[i] = layoutEdge{e.source, e.target, e.dashed}
		}
		v.result = layout(nodes, edges, v.direction, v.gaps)
		v.place(ready, snap)
		if ready && !v.s.userMoved {
			v.fit(snap)
		}
		changed = true
	}
	if !ready && all && v.width > 0 {
		v.ready.Set(true)
		if v.s.placed {
			// A diagram shown again keeps its camera.
			v.moveCamera(v.s.cam, false)
		} else {
			v.s.placed = true
			v.fit(true)
		}
	}
	if changed {
		v.laidOut.Update(func(n int) int { return n + 1 })
	}
}

// place moves each node to its laid out place: at once before the diagram
// shows, for a node not shown yet and when snapping, and otherwise along a
// spring on the platform while the edges follow.
func (v *view) place(ready, snap bool) {
	still := v.reduceMotion()
	var springs []*core.Animation
	for i, n := range v.nodes {
		a := v.anim(n.id)
		x, y := v.result.x[i], v.result.y[i]
		if !a.placed || !ready || !n.measured || snap || still {
			a.x.Set(x)
			a.y.Set(y)
			a.placed = a.placed || n.measured
			continue
		}
		springs = append(springs, core.Spring(a.x, x, nodeSpring), core.Spring(a.y, y, nodeSpring))
	}
	if len(springs) == 0 {
		if v.moveRun != nil {
			v.moveRun.Stop()
			v.moveRun = nil
		}
		v.moving.Set(false)
		return
	}
	v.moves++
	move := v.moves
	v.moving.Set(true)
	v.cx.Run(func() {
		v.moveRun = v.cx.StartAnimation(core.Parallel(springs...), func(bool) {
			if v.moves == move {
				v.moving.Set(false)
			}
		})
	})
}

// hasher is FNV-1a over the fields that decide whether the layout changed.
type hasher struct{ sum uint64 }

func (h *hasher) reset() { h.sum = 14695981039346656037 }

func (h *hasher) byte(b byte) {
	h.sum ^= uint64(b)
	h.sum *= 1099511628211
}

func (h *hasher) uint(v uint64) {
	for range 8 {
		h.byte(byte(v))
		v >>= 8
	}
}

func (h *hasher) string(s string) {
	for i := 0; i < len(s); i++ {
		h.byte(s[i])
	}
	h.byte(0)
}

func (h *hasher) int(v int)       { h.uint(uint64(v)) }
func (h *hasher) float(v float64) { h.uint(math.Float64bits(v)) }
