package driver

import (
	"cmp"
	"math"
	"time"
)

// Animator evaluates the animation graph on a platform's main thread, as
// NSH's does: backends feed it each ItemAnimation, step it once a frame
// while it runs, and apply the values it gives each bound view. It is
// pure Go and touches no view, so every backend shares it.
//
// Drivers are closed-form functions of the time since they started, so
// the result does not depend on the frame rate or dropped frames. Not safe
// for concurrent use.
type Animator struct {
	nodes   map[uint32]*anode
	runs    map[uint32]*arun
	views   map[NodeID]*aview
	pans    map[NodeID][2]uint32
	scrolls map[NodeID][2]uint32
	// pinches and rotations are the values gestures on a view move, and
	// starts where a gesture's values were as it began: pan x, y, pinch
	// and rotation.
	pinches, rotations map[NodeID]uint32
	// pulls are the values scroll views' pulls past their tops set.
	pulls    map[NodeID]uint32
	starts   map[NodeID][4]float64
	report   func(Event)
	gen      uint64 // bumped when any value changes
	// changed are the views Changed reports next, and viewsStale says
	// every bound view may have: a value changing marks them all with a
	// flag, as setting each in the map every frame cost most of a frame
	// of the gallery's Animations page.
	changed    map[NodeID]bool
	viewsStale bool
	now      float64 // seconds of the last step
	finished []Event

	// shaders are the shader views with bound lanes; shadersStale says
	// values changed since ChangedShaders last looked.
	shaders      map[NodeID]*ashader
	shadersStale bool

	// canvases are the canvas views with bound drawing arguments, and
	// canvasesStale says values changed since ChangedCanvases looked.
	canvases      map[NodeID]*acanvas
	canvasesStale bool

	// natives are the native views with bound props, and nativesStale
	// says values changed since ChangedNatives looked.
	natives      map[NodeID]*anative
	nativesStale bool

	// runStarts are where the values each run moves were as it started,
	// for a reset; settling are the snapping diff clamps easing to a
	// bound.
	runStarts map[uint32][]valueStart
	settling  map[*anode]bool
}

// valueStart is a value as a run found it.
type valueStart struct {
	node  uint32
	value float64
}

// NewAnimator returns an animator that reports finished runs and reads
// through report, for the backend to dispatch to the engine.
func NewAnimator(report func(Event)) *Animator {
	return &Animator{nodes: map[uint32]*anode{}, runs: map[uint32]*arun{}, views: map[NodeID]*aview{},
		pans: map[NodeID][2]uint32{}, scrolls: map[NodeID][2]uint32{}, report: report, changed: map[NodeID]bool{},
		pinches: map[NodeID]uint32{}, rotations: map[NodeID]uint32{}, pulls: map[NodeID]uint32{}, starts: map[NodeID][4]float64{},
		shaders: map[NodeID]*ashader{}, canvases: map[NodeID]*acanvas{}, natives: map[NodeID]*anative{}, runStarts: map[uint32][]valueStart{},
		settling: map[*anode]bool{}}
}

// anode is a node of the graph.
type anode struct {
	kind   AnimationOpKind // the op that made it
	value  float64         // a value's
	offset float64
	vel    float64 // units per second, from the driver moving it
	driver *aleaf  // the animation moving a value
	op     AnimationOp
	// cache is the node's output in generation gen.
	gen   uint64
	num   float64
	color Color
	// diff-clamp state, and a snapping one's settling: from where to
	// which bound, since when
	last, lastIn  float64
	primed        bool
	settleFrom    float64
	settleTo      float64
	settleStarted float64
}

type aview struct {
	nodes [PropCount]uint32
	sent  AnimatedValues
}

// ashader is a shader view's bound lanes and their values ChangedShaders
// last reported.
type ashader struct {
	nodes []ShaderNode
	sent  []float64
}

// acanvas is a canvas view's bound arguments, the values ChangedCanvases
// last reported and the ops CanvasOps last played.
type acanvas struct {
	slots []DrawSlot
	nums  []float64
	cols  []Color
	sent  bool
	ops   []DrawOp
}

// anative is a native view's bound props and the values ChangedNatives
// last handed over, unset before the first.
type anative struct {
	bindings []NativeBinding
	nums     []float64
	cols     []Color
	sent     []bool
}

// AnimatedValues are the bound properties of a view, as the animator
// last evaluated them.
type AnimatedValues struct {
	// Set holds a bit for each bound AnimatedProp.
	Set                    uint16
	Opacity                float32
	TranslateX, TranslateY Points
	Scale, ScaleX, ScaleY  float32
	Rotate                 float32
	Background             Color
	BorderColor            Color
	ShadowColor            Color
	ShadowRadius           Points
	ShadowOpacity          float32
	// TextColor colors a text's whole content.
	TextColor        Color
	RotateX, RotateY float32
}

// Has reports whether p is bound.
func (v AnimatedValues) Has(p AnimatedProp) bool { return v.Set&(1<<p) != 0 }

// Apply returns vs with the bound properties in place of its own, for
// backends that draw from a ViewStyle.
func (v AnimatedValues) Apply(vs ViewStyle) ViewStyle {
	if v.Has(PropOpacity) {
		vs.Opacity = v.Opacity
	}
	t := &vs.Transform
	if v.Has(PropTranslateX) {
		t.TranslateX = v.TranslateX
	}
	if v.Has(PropTranslateY) {
		t.TranslateY = v.TranslateY
	}
	if v.Has(PropScale) {
		t.Scale = v.Scale
	}
	if v.Has(PropScaleX) {
		t.ScaleX = v.ScaleX
	}
	if v.Has(PropScaleY) {
		t.ScaleY = v.ScaleY
	}
	if v.Has(PropRotate) {
		t.Rotate = v.Rotate
	}
	if v.Has(PropBackground) {
		vs.Background = v.Background
	}
	if v.Has(PropBorderColor) {
		vs.BorderColor = v.BorderColor
		if vs.Borders != (Edges{}) {
			vs.BorderColors = AllColors(v.BorderColor)
		}
	}
	if v.Has(PropShadowColor) {
		vs.ShadowColor = v.ShadowColor
	}
	if v.Has(PropShadowRadius) {
		vs.ShadowRadius = v.ShadowRadius
	}
	if v.Has(PropShadowOpacity) {
		a := math.Round(float64(vs.ShadowColor&0xff) * float64(min(max(v.ShadowOpacity, 0), 1)))
		vs.ShadowColor = vs.ShadowColor&^0xff | Color(a)
	}
	if v.Has(PropRotateX) {
		t.RotateX = v.RotateX
	}
	if v.Has(PropRotateY) {
		t.RotateY = v.RotateY
	}
	return vs
}

// Apply applies a batch's changes to the graph.
func (a *Animator) Apply(ops []AnimationOp) {
	for i := range ops {
		a.apply(&ops[i])
	}
}

func (a *Animator) apply(o *AnimationOp) {
	switch o.Kind {
	case AnimCreateValue:
		a.nodes[o.Node] = &anode{kind: o.Kind, value: o.Value}
		a.touch()
	case AnimCreateInterpolate, AnimCreateMath, AnimCreateDiffClamp, AnimCreateRubberBand:
		a.nodes[o.Node] = &anode{kind: o.Kind, op: *o}
		a.touch()
	case AnimSet, AnimSetOffset, AnimFlattenOffset, AnimExtractOffset, AnimStopValue:
		n := a.value(o.Node)
		if n == nil {
			return
		}
		if o.Kind != AnimSetOffset {
			a.interrupt(n)
		}
		switch o.Kind {
		case AnimSet:
			n.value = o.Value
		case AnimSetOffset:
			n.offset = o.Value
		case AnimFlattenOffset:
			n.value += n.offset
			n.offset = 0
		case AnimExtractOffset:
			n.offset += n.value
			n.value = 0
		}
		a.touch()
	case AnimRead:
		v := 0.0
		if n := a.value(o.Node); n != nil {
			v = n.value + n.offset
		}
		a.report(Event{Kind: EventAnimation, Count: o.ID, Value: float32(v), Finished: true})
	case AnimStart:
		if o.Animation == nil {
			return
		}
		r := &arun{id: o.ID, root: a.build(o.Animation), forever: o.Endless || endless(o.Animation), rate: o.Animation.FrameRate}
		var starts []valueStart
		for _, id := range r.root.values(nil) {
			if n := a.value(id); n != nil {
				starts = append(starts, valueStart{id, n.value})
			}
		}
		a.runStarts[o.ID] = starts
		a.runs[o.ID] = r
		r.root.bind(r)
		r.root.start(a, -1)
	case AnimStop:
		if r := a.runs[o.ID]; r != nil {
			a.end(r, false)
		}
	case AnimReset:
		if r := a.runs[o.ID]; r != nil {
			a.end(r, false)
		}
		for _, st := range a.runStarts[o.ID] {
			if n := a.value(st.node); n != nil {
				a.interrupt(n)
				n.value, n.vel = st.value, 0
			}
		}
		a.touch()
	case AnimForgetRun:
		delete(a.runStarts, o.ID)
	case AnimBind:
		v := a.views[o.View]
		if v == nil {
			if o.Node == 0 {
				return
			}
			v = &aview{}
			a.views[o.View] = v
		}
		v.nodes[o.Prop] = o.Node
		a.changed[o.View] = true
		if v.nodes == ([PropCount]uint32{}) {
			delete(a.views, o.View)
		}
	case AnimBindPan:
		a.pans[o.View] = [2]uint32{o.X, o.Y}
	case AnimBindScroll:
		a.scrolls[o.View] = [2]uint32{o.X, o.Y}
	case AnimBindPinch:
		a.pinches[o.View] = o.Node
	case AnimBindRotation:
		a.rotations[o.View] = o.Node
	case AnimBindPull:
		a.pulls[o.View] = o.Node
	case AnimRelease:
		if n := a.nodes[o.Node]; n != nil {
			a.interrupt(n)
			delete(a.settling, n)
			delete(a.nodes, o.Node)
			a.touch()
		}
	}
}

// touch notes that values changed, so bound views are evaluated again.
func (a *Animator) touch() {
	a.gen++
	a.viewsStale = true
	a.shadersStale = true
	a.canvasesStale = true
	a.nativesStale = true
}

func (a *Animator) value(id uint32) *anode {
	if n := a.nodes[id]; n != nil && n.kind == AnimCreateValue {
		return n
	}
	return nil
}

// interrupt stops the animation moving n, ending its run unfinished.
func (a *Animator) interrupt(n *anode) {
	if l := n.driver; l != nil {
		n.driver = nil
		if r := l.run; r != nil && a.runs[r.id] == r {
			a.end(r, false)
		}
	}
}

// end finishes run r, reporting whether it ran to its end.
func (a *Animator) end(r *arun, finished bool) {
	delete(a.runs, r.id)
	r.root.stop(a)
	a.report(Event{Kind: EventAnimation, Count: r.id, Finished: finished})
}

// Running reports whether an animation runs, or a snapping diff clamp
// settles, so the backend keeps its frame callback.
func (a *Animator) Running() bool { return len(a.runs) > 0 || len(a.settling) > 0 }

// Ending reports whether something runs that ends on its own: an
// animation without an endless loop that is not marked Endless, or a
// settling clamp. Tests wait for
// it before they look.
func (a *Animator) Ending() bool {
	if len(a.settling) > 0 {
		return true
	}
	for _, r := range a.runs {
		if !r.forever {
			return true
		}
	}
	return false
}

// FrameRate returns the frames a second the running animations need, 0
// when one needs every frame the display has, for a backend to slow its
// frame clock.
func (a *Animator) FrameRate() float64 {
	if len(a.settling) > 0 {
		return 0
	}
	rate := 0.0
	for _, r := range a.runs {
		if r.rate == 0 {
			return 0
		}
		rate = max(rate, r.rate)
	}
	return rate
}

// Step advances the running animations to now, a monotonic time. A run
// with a frame rate moves only when its next frame is due, so a clock
// faster than the run changes nothing in between.
func (a *Animator) Step(now time.Duration) {
	a.now = now.Seconds()
	for _, r := range a.runs {
		if a.runs[r.id] != r {
			continue
		}
		if r.rate > 0 {
			if r.moved && a.now-r.last < 1/r.rate-frameSlack {
				continue
			}
			r.last, r.moved = a.now, true
		}
		if done, _ := r.root.step(a, a.now); done {
			a.end(r, true)
		}
	}
	for n := range a.settling {
		if n.settleStarted < 0 {
			// Settling starts with the first frame after the rest.
			n.settleStarted = a.now
		}
		p := (a.now - n.settleStarted) / settleDuration
		if p >= 1 {
			n.last = n.settleTo
			delete(a.settling, n)
		} else {
			// Easing out, as NSH's settle does.
			e := 1 - math.Pow(1-max(p, 0), 3)
			n.last = n.settleFrom + (n.settleTo-n.settleFrom)*e
		}
		a.touch()
	}
}

// frameSlack is how early a run's frame may land, in seconds: a display
// frame that lands a little before the rate's own would otherwise wait
// for the one after it, halving the rate.
const frameSlack = 0.002

// settleDuration is how long a snapping diff clamp takes to its bound,
// in seconds.
const settleDuration = 0.2

// Settle starts the snapping diff clamps that follow scroll view view's
// values toward their nearer bound, as the view comes to rest: the
// platform calls it where it reports a scroll's end. Max is the nearer
// bound only once the clamp's input reached it, so a header near the top
// of the content shows.
func (a *Animator) Settle(view NodeID) {
	ids, ok := a.scrolls[view]
	if !ok {
		return
	}
	for _, n := range a.nodes {
		if n.kind != AnimCreateDiffClamp || !n.op.Snap || !a.dependsOn(n.op.Input, ids) {
			continue
		}
		a.eval(n)
		lo, hi := n.op.Min, n.op.Max
		to := lo
		if n.lastIn >= hi && n.last-lo >= hi-n.last {
			to = hi
		}
		if n.last == to {
			continue
		}
		n.settleFrom, n.settleTo, n.settleStarted = n.last, to, -1
		a.settling[n] = true
	}
}

// dependsOn reports whether node id reads one of the values ids.
func (a *Animator) dependsOn(id uint32, ids [2]uint32) bool {
	if id == 0 {
		return false
	}
	if id == ids[0] || id == ids[1] {
		return true
	}
	n := a.nodes[id]
	if n == nil {
		return false
	}
	switch n.kind {
	case AnimCreateInterpolate, AnimCreateDiffClamp, AnimCreateRubberBand:
		return a.dependsOn(n.op.Input, ids)
	case AnimCreateMath:
		return a.dependsOn(n.op.A.Node, ids) || a.dependsOn(n.op.B.Node, ids)
	}
	return false
}

// Generation changes whenever a value may have changed, for backends
// that place what values drive, such as sticky cells, when it does.
func (a *Animator) Generation() uint64 { return a.gen }

// Value returns node's output now: a number, and a color for a color
// node; false when there is no such node.
func (a *Animator) Value(node uint32) (float64, Color, bool) {
	n := a.nodes[node]
	if n == nil {
		return 0, 0, false
	}
	num, col := a.eval(n)
	return num, col, true
}

// Changed calls fn with each bound view whose values may have changed
// since the last call, and their values.
func (a *Animator) Changed(fn func(view NodeID, v AnimatedValues)) {
	for id := range a.changed {
		delete(a.changed, id)
		vw := a.views[id]
		var vals AnimatedValues
		if vw != nil {
			vals = a.evalView(vw)
			if vals == vw.sent {
				continue
			}
			vw.sent = vals
		}
		fn(id, vals)
	}
	if !a.viewsStale {
		return
	}
	a.viewsStale = false
	for id, vw := range a.views {
		if vals := a.evalView(vw); vals != vw.sent {
			vw.sent = vals
			fn(id, vals)
		}
	}
}

// Touch has Changed report view's values again, as a backend needs after
// it drew the view's own style over them.
func (a *Animator) Touch(view NodeID) {
	if vw := a.views[view]; vw != nil {
		vw.sent = AnimatedValues{}
		a.changed[view] = true
	}
}

// Values returns view's bound values, and false when it has none.
func (a *Animator) Values(view NodeID) (AnimatedValues, bool) {
	vw := a.views[view]
	if vw == nil {
		return AnimatedValues{}, false
	}
	return a.evalView(vw), true
}

func (a *Animator) evalView(vw *aview) AnimatedValues {
	var v AnimatedValues
	for p, id := range vw.nodes {
		if id == 0 {
			continue
		}
		n := a.nodes[id]
		if n == nil {
			continue
		}
		num, col := a.eval(n)
		v.Set |= 1 << p
		switch AnimatedProp(p) {
		case PropOpacity:
			v.Opacity = float32(min(max(num, 0), 1))
		case PropTranslateX:
			v.TranslateX = Points(num)
		case PropTranslateY:
			v.TranslateY = Points(num)
		case PropScale:
			v.Scale = float32(num)
		case PropScaleX:
			v.ScaleX = float32(num)
		case PropScaleY:
			v.ScaleY = float32(num)
		case PropRotate:
			v.Rotate = float32(num)
		case PropBackground:
			v.Background = col
		case PropBorderColor:
			v.BorderColor = col
		case PropShadowColor:
			v.ShadowColor = col
		case PropShadowRadius:
			v.ShadowRadius = Points(max(num, 0))
		case PropShadowOpacity:
			v.ShadowOpacity = float32(min(max(num, 0), 1))
		case PropTextColor:
			v.TextColor = col
		case PropRotateX:
			v.RotateX = float32(num)
		case PropRotateY:
			v.RotateY = float32(num)
		}
	}
	return v
}

// Gesture moves the values bound to ev's gesture, a pan, pinch or
// rotation on ev.ID, before the platform reports it: from where they were
// as it began, so a gesture grabs a value an animation is moving where it
// is. A pan's start and end report the bound values in ev.Values. It
// reports whether a value moved, to draw.
func (a *Animator) Gesture(ev *Event) bool {
	view := ev.ID
	s := a.starts[view]
	current := func(id uint32, unset float64) float64 {
		if n := a.value(id); n != nil {
			a.interrupt(n)
			return n.value
		}
		return unset
	}
	set := func(id uint32, v float64) {
		if n := a.value(id); n != nil {
			n.value = v
		}
	}
	began := ev.Phase == GestureBegan
	switch ev.Kind {
	case EventPan:
		ids, ok := a.pans[view]
		if !ok {
			return false
		}
		if began {
			s[0], s[1] = current(ids[0], 0), current(ids[1], 0)
		}
		set(ids[0], s[0]+float64(ev.Point.X))
		set(ids[1], s[1]+float64(ev.Point.Y))
		if began || ev.Phase == GestureEnded || ev.Phase == GestureCancelled {
			ev.Values = Point{Points(current(ids[0], 0)), Points(current(ids[1], 0))}
		}
	case EventPinch:
		id, ok := a.pinches[view]
		if !ok {
			return false
		}
		if began {
			s[2] = current(id, 1)
		}
		set(id, s[2]*float64(ev.Value))
	case EventRotation:
		id, ok := a.rotations[view]
		if !ok {
			return false
		}
		if began {
			s[3] = current(id, 0)
		}
		set(id, s[3]+float64(ev.Value))
	default:
		return false
	}
	a.starts[view] = s
	a.touch()
	return true
}

// Scroll sets the values bound to scroll view view to its offset.
func (a *Animator) Scroll(view NodeID, offset Point) bool {
	return a.drive(a.scrolls, view, offset)
}

// Pull sets the value bound to scroll view view's pull to distance, how
// far it is pulled past its top.
func (a *Animator) Pull(view NodeID, distance Points) bool {
	n := a.value(a.pulls[view])
	if n == nil || n.value == float64(distance) {
		return false
	}
	a.interrupt(n)
	n.value = float64(distance)
	a.touch()
	return true
}

// Pulls reports whether a value follows view's pull.
func (a *Animator) Pulls(view NodeID) bool { return a.pulls[view] != 0 }

// PanBound reports whether drags on view move values.
func (a *Animator) PanBound(view NodeID) bool { _, ok := a.pans[view]; return ok }

func (a *Animator) drive(m map[NodeID][2]uint32, view NodeID, p Point) bool {
	ids, ok := m[view]
	if !ok {
		return false
	}
	for i, v := range [2]Points{p.X, p.Y} {
		if n := a.value(ids[i]); n != nil {
			a.interrupt(n)
			n.value = float64(v)
		}
	}
	// Scrolling again ends the settling where it is.
	for n := range a.settling {
		if a.dependsOn(n.op.Input, ids) {
			delete(a.settling, n)
		}
	}
	a.touch()
	return true
}

// BindShader has nodes drive lanes of shader view's uniforms, replacing
// what it bound before (Shader.Nodes). ChangedShaders reports the view
// next.
func (a *Animator) BindShader(view NodeID, nodes []ShaderNode) {
	if len(nodes) == 0 {
		delete(a.shaders, view)
		return
	}
	a.shaders[view] = &ashader{nodes: nodes}
	a.shadersStale = true
}

// ShaderLanes writes the values of view's bound lanes into lanes, which
// hold its Shader.Lanes; a lane beyond them is left out.
func (a *Animator) ShaderLanes(view NodeID, lanes []float64) {
	sh := a.shaders[view]
	if sh == nil {
		return
	}
	for _, b := range sh.nodes {
		if n := a.nodes[b.Node]; n != nil && b.Lane >= 0 && b.Lane < len(lanes) {
			lanes[b.Lane] = b.Channel.Of(a.eval(n))
		}
	}
}

// ChangedShaders calls fn with each shader view whose bound lanes
// changed since the last call, for the backend to draw again.
func (a *Animator) ChangedShaders(fn func(view NodeID)) {
	if !a.shadersStale {
		return
	}
	a.shadersStale = false
	for id, sh := range a.shaders {
		changed := len(sh.sent) != len(sh.nodes)
		if changed {
			sh.sent = make([]float64, len(sh.nodes))
		}
		for i, b := range sh.nodes {
			var v float64
			if n := a.nodes[b.Node]; n != nil {
				v = b.Channel.Of(a.eval(n))
			}
			if v != sh.sent[i] {
				sh.sent[i], changed = v, true
			}
		}
		if changed {
			fn(id)
		}
	}
}

// BindCanvas has nodes drive the arguments of canvas view's drawing that
// slots name, replacing what it bound before (Drawing.Slots).
// ChangedCanvases reports the view next.
func (a *Animator) BindCanvas(view NodeID, slots []DrawSlot) {
	if len(slots) == 0 {
		delete(a.canvases, view)
		return
	}
	a.canvases[view] = &acanvas{slots: slots, nums: make([]float64, len(slots)), cols: make([]Color, len(slots))}
	a.canvasesStale = true
}

// CanvasBound reports whether view's drawing has bound arguments.
func (a *Animator) CanvasBound(view NodeID) bool { return a.canvases[view] != nil }

// ChangedCanvases calls fn with each canvas view whose bound arguments
// changed since the last call, for the backend to draw again.
func (a *Animator) ChangedCanvases(fn func(view NodeID)) {
	if !a.canvasesStale {
		return
	}
	a.canvasesStale = false
	for id, c := range a.canvases {
		changed := !c.sent
		c.sent = true
		for i, s := range c.slots {
			var num float64
			var col Color
			if n := a.nodes[s.Node]; n != nil {
				num, col = a.eval(n)
			}
			if num != c.nums[i] || col != c.cols[i] {
				c.nums[i], c.cols[i], changed = num, col, true
			}
		}
		if changed {
			fn(id)
		}
	}
}

// CanvasOps returns d's ops with the arguments view binds set to their
// nodes' values, d.Ops itself when it binds none. The slice is reused by
// the next call for the view.
func (a *Animator) CanvasOps(view NodeID, d *Drawing) []DrawOp {
	c := a.canvases[view]
	if c == nil || d == nil {
		if d == nil {
			return nil
		}
		return d.Ops
	}
	c.ops = append(c.ops[:0], d.Ops...)
	for _, s := range c.slots {
		if s.Op < 0 || s.Op >= len(c.ops) {
			continue
		}
		var num float64
		var col Color
		if n := a.nodes[s.Node]; n != nil {
			num, col = a.eval(n)
		}
		c.ops[s.Op] = s.Set(c.ops[s.Op], num, col)
	}
	return c.ops
}

// BindNative has animated nodes drive props of native view's component,
// replacing what it bound before (NativeConfig.Bindings). ChangedNatives
// hands every bound value over next.
func (a *Animator) BindNative(view NodeID, bindings []NativeBinding) {
	if len(bindings) == 0 {
		delete(a.natives, view)
		return
	}
	n := len(bindings)
	a.natives[view] = &anative{bindings: bindings, nums: make([]float64, n), cols: make([]Color, n), sent: make([]bool, n)}
	a.nativesStale = true
}

// ChangedNatives calls fn with each bound prop of a native view whose
// value changed since it last handed it over: a number, or a color when
// the binding's node outputs one. Backends pass them to SetAnimatedProp.
func (a *Animator) ChangedNatives(fn func(view NodeID, b NativeBinding, num float64, col Color)) {
	if !a.nativesStale {
		return
	}
	a.nativesStale = false
	for id, nv := range a.natives {
		for i, b := range nv.bindings {
			var num float64
			var col Color
			if n := a.nodes[b.Node]; n != nil {
				num, col = a.eval(n)
			}
			if nv.sent[i] && num == nv.nums[i] && col == nv.cols[i] {
				continue
			}
			nv.sent[i], nv.nums[i], nv.cols[i] = true, num, col
			fn(id, b, num, col)
		}
	}
}

// Forget drops what is bound to a deleted view.
func (a *Animator) Forget(view NodeID) {
	delete(a.canvases, view)
	delete(a.shaders, view)
	delete(a.natives, view)
	delete(a.views, view)
	delete(a.pans, view)
	delete(a.scrolls, view)
	delete(a.pulls, view)
	delete(a.pinches, view)
	delete(a.rotations, view)
	delete(a.starts, view)
	delete(a.changed, view)
}

// eval returns n's output in the current generation.
func (a *Animator) eval(n *anode) (float64, Color) {
	if n.gen == a.gen && n.gen != 0 {
		return n.num, n.color
	}
	n.gen = a.gen
	switch n.kind {
	case AnimCreateValue:
		n.num = n.value + n.offset
	case AnimCreateInterpolate:
		in := 0.0
		if src := a.nodes[n.op.Input]; src != nil {
			in, _ = a.eval(src)
		}
		n.num, n.color = n.op.Interpolation.At(in)
	case AnimCreateMath:
		x, y := a.operand(n.op.A), a.operand(n.op.B)
		switch n.op.Math {
		case MathAdd:
			n.num = x + y
		case MathSub:
			n.num = x - y
		case MathMul:
			n.num = x * y
		case MathDiv:
			n.num = 0
			if y != 0 {
				n.num = x / y
			}
		case MathMod:
			n.num = 0
			if y != 0 {
				n.num = math.Mod(math.Mod(x, y)+y, y)
			}
		}
	case AnimCreateDiffClamp:
		in := 0.0
		if src := a.nodes[n.op.Input]; src != nil {
			in, _ = a.eval(src)
		}
		switch {
		case !n.primed:
			n.primed = true
			n.last = min(max(in, n.op.Min), n.op.Max)
		case a.settling[n]:
			// The settling moves it; Step sets last.
		default:
			n.last = min(max(n.last+in-n.lastIn, n.op.Min), n.op.Max)
		}
		n.lastIn = in
		n.num = n.last
	case AnimCreateRubberBand:
		in := 0.0
		if src := a.nodes[n.op.Input]; src != nil {
			in, _ = a.eval(src)
		}
		n.num = RubberBand(in, n.op.Min, n.op.Max, n.op.Value)
	}
	return n.num, n.color
}

func (a *Animator) operand(o Operand) float64 {
	if o.Node == 0 {
		return o.Const
	}
	if n := a.nodes[o.Node]; n != nil {
		v, _ := a.eval(n)
		return v
	}
	return 0
}

// At maps x through the interpolation: a number, and a color when it has
// Colors.
func (r *Interpolation) At(x float64) (float64, Color) {
	in := r.Input
	k := len(in)
	if k == 0 {
		return x, 0
	}
	if k == 1 {
		return r.output(0, 0, 0)
	}
	i := 0
	for i < k-2 && x > in[i+1] {
		i++
	}
	lo, hi := in[i], in[i+1]
	var t float64
	if hi != lo {
		t = (x - lo) / (hi - lo)
	}
	switch {
	case x < in[0]:
		switch cmp.Or(r.ExtrapolateLeft, r.Extrapolate) {
		case ExtrapolateIdentity:
			return x, 0
		case 0, ExtrapolateClamp:
			t = 0
		}
	case x > in[k-1]:
		switch cmp.Or(r.ExtrapolateRight, r.Extrapolate) {
		case ExtrapolateIdentity:
			return x, 0
		case 0, ExtrapolateClamp:
			t = 1
		}
	}
	if t >= 0 && t <= 1 {
		t = r.Easing.At(t)
	}
	return r.output(i, i+1, t)
}

func (r *Interpolation) output(i, j int, t float64) (float64, Color) {
	if len(r.Colors) > j {
		return t, lerpColor(r.Colors[i], r.Colors[j], t)
	}
	if len(r.Output) > j {
		return r.Output[i] + (r.Output[j]-r.Output[i])*t, 0
	}
	return 0, 0
}

func lerpColor(a, b Color, t float64) Color {
	t = min(max(t, 0), 1)
	var out Color
	for shift := 0; shift < 32; shift += 8 {
		x, y := float64(a>>shift&0xff), float64(b>>shift&0xff)
		out |= Color(math.Round(x+(y-x)*t)) << shift
	}
	return out
}

// At returns the eased progress at time fraction x from 0 to 1.
func (e Easing) At(x float64) float64 {
	switch e.Kind {
	case EasingBezier:
		return e.Bezier.At(x)
	case EasingSteps:
		return e.steps(x)
	}
	switch e.Mode {
	case EaseModeOut:
		return 1 - e.in(1-x)
	case EaseModeInOut:
		if x < 0.5 {
			return e.in(x*2) / 2
		}
		return 1 - e.in((1-x)*2)/2
	}
	return e.in(x)
}

// steps is CSS's steps(): Param steps jumping at Steps's positions.
func (e Easing) steps(x float64) float64 {
	n := max(math.Round(e.Param), 1)
	step := math.Floor(x * n)
	if e.Steps == JumpStart || e.Steps == JumpBoth {
		step++
	}
	jumps := n
	switch e.Steps {
	case JumpBoth:
		jumps++
	case JumpNone:
		jumps = max(n-1, 1)
	}
	if x >= 0 && step < 0 {
		step = 0
	}
	if x <= 1 && step > jumps {
		step = jumps
	}
	return step / jumps
}

// Reversed returns the easing that runs e backwards in time: its curve
// mirrored, so x → 1 - e(1 - x).
func (e Easing) Reversed() Easing {
	switch e.Kind {
	case EasingBezier:
		c := e.Bezier
		e.Bezier = Curve{1 - c.X2, 1 - c.Y2, 1 - c.X1, 1 - c.Y1}
		if c == Linear {
			e.Bezier = Linear
		}
	case EasingSteps:
		switch e.Steps {
		case JumpStart:
			e.Steps = JumpEnd
		case JumpEnd:
			e.Steps = JumpStart
		}
	default:
		switch e.Mode {
		case EaseModeIn:
			e.Mode = EaseModeOut
		case EaseModeOut:
			e.Mode = EaseModeIn
		}
	}
	return e
}

// in is the curve's "in" form, React Native's Easing functions.
func (e Easing) in(t float64) float64 {
	switch e.Kind {
	case EasingQuad:
		return t * t
	case EasingCubic:
		return t * t * t
	case EasingPoly:
		n := e.Param
		if n == 0 {
			n = 2
		}
		return math.Pow(t, n)
	case EasingSin:
		return 1 - math.Cos(t*math.Pi/2)
	case EasingCircle:
		return 1 - math.Sqrt(max(1-t*t, 0))
	case EasingExp:
		if t <= 0 {
			return 0
		}
		return math.Pow(2, 10*(t-1))
	case EasingElastic:
		b := e.Param
		if b == 0 {
			b = 1
		}
		p := b * math.Pi
		return 1 - math.Pow(math.Cos(t*math.Pi/2), 3)*math.Cos(t*p)
	case EasingBack:
		s := e.Param
		if s == 0 {
			s = 1.70158
		}
		return t * t * ((s+1)*t - s)
	case EasingBounce:
		return bounce(t)
	}
	return t
}

func bounce(t float64) float64 {
	switch {
	case t < 1/2.75:
		return 7.5625 * t * t
	case t < 2/2.75:
		t -= 1.5 / 2.75
		return 7.5625*t*t + 0.75
	case t < 2.5/2.75:
		t -= 2.25 / 2.75
		return 7.5625*t*t + 0.9375
	}
	t -= 2.625 / 2.75
	return 7.5625*t*t + 0.984375
}

// Runs and their steps.

type arun struct {
	id   uint32
	root astep
	// forever is set when a loop in it repeats without end, or its owner
	// starts it again when it ends.
	forever bool
	// rate is its frame rate, 0 for every step; last is when it last
	// moved, once moved.
	rate, last float64
	moved      bool
}

// endless reports whether an repeats forever: it holds a Loop of no
// count.
func endless(an *Animation) bool {
	if an.Kind == AnimationLoop && an.Iterations == 0 {
		return true
	}
	for _, s := range an.Steps {
		if endless(s) {
			return true
		}
	}
	return false
}

// astep is a node of a running animation's tree.
type astep interface {
	// start begins at time at, in seconds, or on the next step when at
	// is negative.
	start(a *Animator, at float64)
	// step advances to now and reports whether the step is done and when
	// it ended.
	step(a *Animator, now float64) (done bool, end float64)
	// stop releases the values it moves.
	stop(a *Animator)
	// bind gives the leaves their run, so an interruption ends it.
	bind(r *arun)
	// values lists the values the step moves, for loops to reset.
	values(out []uint32) []uint32
}

func (a *Animator) build(an *Animation) astep {
	switch an.Kind {
	case AnimationTiming, AnimationSpring, AnimationDecay:
		return &aleaf{spec: an}
	case AnimationDelay:
		return &adelay{d: an.Delay.Seconds()}
	case AnimationSequence:
		s := &aseq{}
		for _, c := range an.Steps {
			s.steps = append(s.steps, a.build(c))
		}
		return s
	case AnimationParallel, AnimationStagger:
		p := &apar{}
		for i, c := range an.Steps {
			var st astep = a.build(c)
			if an.Kind == AnimationStagger && i > 0 {
				st = &aseq{steps: []astep{&adelay{d: an.Delay.Seconds() * float64(i)}, st}}
			}
			p.steps = append(p.steps, st)
		}
		return p
	case AnimationLoop:
		l := &aloop{n: an.Iterations, noReset: an.NoReset}
		if len(an.Steps) > 0 {
			l.body = a.build(an.Steps[0])
		}
		return l
	}
	return &adelay{}
}

// aleaf moves one value.
type aleaf struct {
	spec    *Animation
	run     *arun
	node    *anode
	from    float64
	vel     float64 // initial velocity, units per second
	t0      float64
	started bool // has its start time
	// decay segments: a bounce starts a new one at a bound.
	segFrom, segVel, segT0 float64
	done                   bool
}

func (l *aleaf) bind(r *arun) { l.run = r }

func (l *aleaf) values(out []uint32) []uint32 { return append(out, l.spec.Value) }

func (l *aleaf) start(a *Animator, at float64) {
	n := a.value(l.spec.Value)
	l.node, l.done = n, false
	if n == nil {
		l.done = true
		return
	}
	if n.driver != nil && n.driver != l {
		a.interrupt(n)
	}
	n.driver = l
	l.from = n.value
	l.vel = n.vel
	switch l.spec.Kind {
	case AnimationSpring:
		if v := l.spec.Spring.Velocity; v != 0 {
			l.vel = v
		}
	case AnimationDecay:
		if v := l.spec.Decay.Velocity; v != 0 {
			l.vel = v
		}
	}
	l.t0, l.started = at, at >= 0
	l.segFrom, l.segVel, l.segT0 = l.from, l.vel, at
}

func (l *aleaf) stop(a *Animator) {
	if l.node != nil && l.node.driver == l {
		l.node.driver = nil
		l.node.vel = 0
	}
}

func (l *aleaf) step(a *Animator, now float64) (bool, float64) {
	n := l.node
	if l.done || n == nil {
		return true, now
	}
	if n.driver != l {
		// Something else took the value: the run was ended already.
		return true, now
	}
	if !l.started {
		l.t0, l.segT0, l.started = now, now, true
	}
	var v, vel float64
	var done bool
	var end float64
	switch l.spec.Kind {
	case AnimationTiming:
		v, done, end = l.timing(now)
	case AnimationSpring:
		v, vel, done = l.spring(now)
		end = now
	case AnimationDecay:
		v, vel, done = l.decay(now)
		end = now
	}
	n.value, n.vel = v, vel
	a.touch()
	if done {
		l.done = true
		n.driver = nil
		n.vel = 0
	}
	return done, end
}

func (l *aleaf) timing(now float64) (float64, bool, float64) {
	c := &l.spec.Timing
	d, delay := c.Duration.Seconds(), c.Delay.Seconds()
	end := l.t0 + delay + d
	// Steps of a composite start at exact ends; a nanosecond's slack
	// keeps float error from costing a frame.
	if now >= end-1e-9 {
		return l.spec.To, true, end
	}
	x := 1.0
	if d > 0 {
		x = (now - l.t0 - delay) / d
	}
	if x <= 0 {
		return l.from, false, end
	}
	return l.from + (l.spec.To-l.from)*c.Easing.At(x), false, end
}

// spring is React Native's closed-form damped spring.
func (l *aleaf) spring(now float64) (pos, vel float64, done bool) {
	c := &l.spec.Spring
	k, damp, m := c.Stiffness, c.Damping, c.Mass
	to := l.spec.To
	t := now - l.t0
	v0 := -l.vel
	zeta := damp / (2 * math.Sqrt(k*m))
	w0 := math.Sqrt(k / m)
	x0 := to - l.from
	if zeta < 1 {
		w1 := w0 * math.Sqrt(1-zeta*zeta)
		env := math.Exp(-zeta * w0 * t)
		sin, cos := math.Sin(w1*t), math.Cos(w1*t)
		pos = to - env*((v0+zeta*w0*x0)/w1*sin+x0*cos)
		vel = zeta*w0*env*(sin*(v0+zeta*w0*x0)/w1+x0*cos) - env*(cos*(v0+zeta*w0*x0)-w1*x0*sin)
	} else {
		env := math.Exp(-w0 * t)
		pos = to - env*(x0+(v0+w0*x0)*t)
		vel = env * (v0*(t*w0-1) + t*x0*w0*w0)
	}
	const rest = 0.001
	if math.Abs(vel) <= rest && math.Abs(to-pos) <= rest || t > 60 {
		return to, 0, true
	}
	return pos, vel, false
}

// decay slows the value exponentially, bouncing off or stopping at its
// bounds.
func (l *aleaf) decay(now float64) (pos, vel float64, done bool) {
	c := &l.spec.Decay
	k := 1 - c.Deceleration  // per millisecond
	const restSpeed = 0.0005 // units per millisecond
	for range 16 {
		t := (now - l.segT0) * 1000
		v := l.segVel / 1000
		pos = l.segFrom + v/k*(1-math.Exp(-k*t))
		speed := v * math.Exp(-k*t)
		if c.Clamp && (pos < c.Min || pos > c.Max) {
			bound := c.Max
			if pos < c.Min {
				bound = c.Min
			}
			// When it got there: x(t) = bound.
			f := 1 - (bound-l.segFrom)*k/v
			if f <= 0 || f >= 1 || c.Bounce <= 0 {
				return bound, 0, true
			}
			tb := -math.Log(f) / k
			hit := v * math.Exp(-k*tb)
			l.segFrom, l.segVel, l.segT0 = bound, -hit*c.Bounce*1000, l.segT0+tb/1000
			continue
		}
		if math.Abs(speed) < restSpeed {
			return pos, 0, true
		}
		return pos, speed * 1000, false
	}
	return pos, 0, true
}

// adelay waits.
type adelay struct {
	d, t0   float64
	started bool
}

func (d *adelay) start(a *Animator, at float64) { d.t0, d.started = at, at >= 0 }

func (d *adelay) stop(*Animator) {}

func (d *adelay) bind(*arun) {}

func (d *adelay) values(out []uint32) []uint32 { return out }

func (d *adelay) step(a *Animator, now float64) (bool, float64) {
	if !d.started {
		d.t0, d.started = now, true
	}
	end := d.t0 + d.d
	return now >= end-1e-9, end
}

// aseq runs its steps in turn, each from the exact end of the last.
type aseq struct {
	steps []astep
	i     int
}

func (s *aseq) start(a *Animator, at float64) {
	s.i = 0
	if len(s.steps) > 0 {
		s.steps[0].start(a, at)
	}
}

func (s *aseq) stop(a *Animator) {
	if s.i < len(s.steps) {
		s.steps[s.i].stop(a)
	}
}

func (s *aseq) bind(r *arun) {
	for _, st := range s.steps {
		st.bind(r)
	}
}

func (s *aseq) values(out []uint32) []uint32 {
	for _, st := range s.steps {
		out = st.values(out)
	}
	return out
}

func (s *aseq) step(a *Animator, now float64) (bool, float64) {
	end := now
	for s.i < len(s.steps) {
		done, e := s.steps[s.i].step(a, now)
		if !done {
			return false, now
		}
		end = e
		s.i++
		if s.i < len(s.steps) {
			s.steps[s.i].start(a, e)
		}
	}
	return true, end
}

// apar runs its steps at once.
type apar struct {
	steps []astep
	done  []bool
}

func (p *apar) start(a *Animator, at float64) {
	p.done = make([]bool, len(p.steps))
	for _, st := range p.steps {
		st.start(a, at)
	}
}

func (p *apar) stop(a *Animator) {
	for i, st := range p.steps {
		if !p.done[i] {
			st.stop(a)
		}
	}
}

func (p *apar) bind(r *arun) {
	for _, st := range p.steps {
		st.bind(r)
	}
}

func (p *apar) values(out []uint32) []uint32 {
	for _, st := range p.steps {
		out = st.values(out)
	}
	return out
}

func (p *apar) step(a *Animator, now float64) (bool, float64) {
	all, end := true, 0.0
	for i, st := range p.steps {
		if p.done[i] {
			continue
		}
		done, e := st.step(a, now)
		if done {
			p.done[i] = true
			end = max(end, e)
		} else {
			all = false
		}
	}
	if !all {
		return false, now
	}
	if end == 0 {
		end = now
	}
	return true, end
}

// aloop runs its body again, its values put back where they started.
type aloop struct {
	body    astep
	n, i    int
	noReset bool
	initial []float64
	ids     []uint32
}

func (l *aloop) startAt(a *Animator, at float64) {
	if !l.noReset {
		for k, id := range l.ids {
			if n := a.value(id); n != nil {
				n.value = l.initial[k]
			}
		}
		a.touch()
	}
	l.body.start(a, at)
}

func (l *aloop) start(a *Animator, at float64) {
	l.i = 0
	if l.body == nil {
		return
	}
	l.ids = l.body.values(l.ids[:0])
	l.initial = l.initial[:0]
	for _, id := range l.ids {
		v := 0.0
		if n := a.value(id); n != nil {
			v = n.value
		}
		l.initial = append(l.initial, v)
	}
	l.body.start(a, at)
}

func (l *aloop) stop(a *Animator) {
	if l.body != nil {
		l.body.stop(a)
	}
}

func (l *aloop) bind(r *arun) {
	if l.body != nil {
		l.body.bind(r)
	}
}

func (l *aloop) values(out []uint32) []uint32 {
	if l.body == nil {
		return out
	}
	return l.body.values(out)
}

func (l *aloop) step(a *Animator, now float64) (bool, float64) {
	if l.body == nil {
		return true, now
	}
	// A body that ends within this frame starts again at its end, up to
	// a bound so a zero-length body cannot spin.
	for range 64 {
		done, end := l.body.step(a, now)
		if !done {
			return false, now
		}
		l.i++
		if l.n > 0 && l.i >= l.n {
			return true, end
		}
		l.startAt(a, end)
		if end >= now {
			return false, now
		}
	}
	return false, now
}

// SampleRate is how many samples a second the samplers take: the frames
// of a 60 Hz display.
const SampleRate = 60

// maxSamples bounds what a sampler returns: 20 seconds.
const maxSamples = 20 * SampleRate

// SampleEasing returns e at n evenly spaced fractions from 0 to 1, both
// included, as the platforms evaluate it.
func SampleEasing(e Easing, n int) []float64 {
	n = min(max(n, 2), maxSamples)
	out := make([]float64, n)
	for i := range out {
		out[i] = e.At(float64(i) / float64(n-1))
	}
	return out
}

// SampleSpring returns a spring from from to to, as the platforms run
// it, at SampleRate until it rests: the drivers' own code, stepped like
// frames.
func SampleSpring(from, to float64, c SpringConfig) []float64 {
	return sample(&Animation{Kind: AnimationSpring, To: to, Spring: c}, from)
}

// SampleDecay returns a decay from from, as SampleSpring does.
func SampleDecay(from float64, c DecayConfig) []float64 {
	return sample(&Animation{Kind: AnimationDecay, Decay: c}, from)
}

func sample(spec *Animation, from float64) []float64 {
	n := &anode{kind: AnimCreateValue, value: from}
	l := &aleaf{spec: spec, node: n, from: from, vel: 0, started: true}
	switch spec.Kind {
	case AnimationSpring:
		l.vel = spec.Spring.Velocity
	case AnimationDecay:
		l.vel = spec.Decay.Velocity
	}
	l.segFrom, l.segVel = l.from, l.vel
	out := []float64{from}
	for i := 1; i < maxSamples; i++ {
		t := float64(i) / SampleRate
		var v float64
		var done bool
		if spec.Kind == AnimationSpring {
			v, _, done = l.spring(t)
		} else {
			v, _, done = l.decay(t)
		}
		out = append(out, v)
		if done {
			break
		}
	}
	return out
}
