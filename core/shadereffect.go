package core

import (
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Effects and transitions draw pictures the platform takes of views
// through a WGSL program (CompileWGSL): a view's children (ShaderEffect,
// ShaderTransition), the whole surface (ScreenEffect, ScreenTransition)
// and a change of pages or a dialog's coming and going
// (TransitionShader). The core says which pictures changed in each
// batch (ItemShaderInput), and starts a transition first in its batch
// (ItemShaderTransition), so the platform pictures what showed before.
// A platform without a GPU, or one that draws no pictures yet, shows
// the children as they are.

// ShaderEffect draws its children through a WGSL program, which reads
// what they draw with sample_content(uv): over them and in their place,
// while they still take touches, the pointer and focus. Without a
// Program the children show as they are.
type ShaderEffect struct {
	Program      *ShaderProgram
	Uniforms     Uniforms
	UniformsFunc func() Uniforms
	// Progress is nsh.progress; ProgressFunc, when set, follows the graph,
	// and ProgressNode, when set, moves it on the platform's frame clock.
	Progress     float64
	ProgressFunc func() float64
	ProgressNode AnimatedNode
	// MaxSampleOffset is how far beyond each side of the box, in points,
	// the shader draws and its pictures reach (nsh.overflow).
	MaxSampleOffset float64
	// Paused stops nsh.time; PausedFunc follows the graph.
	Paused     bool
	PausedFunc func() bool
	Class      string
	Style      Style
	StyleFunc  func() Style
	Children   []Node
	OnLayout   func(Bounds)
	Ref        *Ref
	TestID     string
}

// Render implements Node.
func (e ShaderEffect) Render(*Scope) Node { return e }

// ShaderTransition shows its children as they are, and draws them
// through a WGSL program as they change: when Value's result changes
// (not its first), sample_from(uv) is a picture of what they showed,
// sample_to(uv) what they show, and nsh.progress goes from 0 to 1 over
// Duration. Without a Program it cross-fades, unless NoFallback.
type ShaderTransition struct {
	Program      *ShaderProgram
	NoFallback   bool
	Uniforms     Uniforms
	UniformsFunc func() Uniforms
	// Value runs the transition when its result changes; compare by ==.
	Value func() any
	// Duration is how long a transition runs, 300 ms when 0.
	Duration        time.Duration
	MaxSampleOffset float64
	Paused          bool
	PausedFunc      func() bool
	Class           string
	Style           Style
	StyleFunc       func() Style
	Children        []Node
	OnLayout        func(Bounds)
	Ref             *Ref
	TestID          string
}

// Render implements Node.
func (e ShaderTransition) Render(*Scope) Node { return e }

// ScreenEffect draws the whole surface through a WGSL program, which
// reads it with sample_content(uv). It takes no space and has no
// children; dialogs a platform presents in windows of their own are
// outside it.
type ScreenEffect struct {
	Program         *ShaderProgram
	Uniforms        Uniforms
	UniformsFunc    func() Uniforms
	Progress        float64
	ProgressFunc    func() float64
	ProgressNode    AnimatedNode
	MaxSampleOffset float64
	Paused          bool
	PausedFunc      func() bool
}

// Render implements Node.
func (e ScreenEffect) Render(*Scope) Node { return e }

// ScreenTransition draws the whole surface through a WGSL program as it
// changes, when Value's result changes, as ShaderTransition does its
// children.
type ScreenTransition struct {
	Program         *ShaderProgram
	NoFallback      bool
	Uniforms        Uniforms
	UniformsFunc    func() Uniforms
	Value           func() any
	Duration        time.Duration
	MaxSampleOffset float64
	Paused          bool
	PausedFunc      func() bool
}

// Render implements Node.
func (e ScreenTransition) Render(*Scope) Node { return e }

// TransitionShader is a WGSL program a page (Screen) or a dialog (Modal)
// comes and goes through instead of its Animation: sample_from(uv) is the
// lower side (the page under it, the surface without the dialog),
// sample_to(uv) the upper, and nsh.progress goes from 0 to 1 as it comes
// and from 1 to 0 as it goes, over Duration (350 ms when 0). When the
// user asks for less motion, or where the platform cannot draw it, the
// page or the dialog takes its own Animation.
type TransitionShader struct {
	Program  *ShaderProgram
	Uniforms Uniforms
	Duration time.Duration
}

// fxNode is a node's effect, transition or navigation shader: what it
// last sent, and a transition's run.
type fxNode struct {
	role    driver.ShaderRole
	program *ShaderProgram
	clock   *shaderClock
	// values makes the shader's lanes with the current uniforms, the
	// progress given.
	values   func(progress AnimatedNode) *driver.Shader
	progress AnimatedNode
	overflow float64
	sent     *driver.Shader
	fresh    bool
	// A transition's run: its progress, 0 to 1, and whether it runs.
	value             *AnimatedValue
	run               *AnimationRun
	runs              int
	running, backward bool
}

// pictures reports whether the platform pictures views for n's shader.
func (fx *fxNode) pictures() bool { return fx != nil && fx.sent != nil && fx.sent.Pictures != 0 }

// update sends the shader again with what it has now.
func (t *Tree) updateFx(n *node) {
	fx := n.fx
	if fx.values == nil {
		if fx.sent != nil {
			fx.sent, fx.fresh = nil, true
			t.touch(n)
		}
		return
	}
	progress := fx.progress
	if fx.role.Transitions() {
		progress = fx.value
	}
	d := fx.values(progress)
	d.Role = fx.role
	d.Running, d.Backwards = fx.running, fx.backward
	fx.sent, fx.fresh = d, true
	t.touch(n)
}

// bindFx gives n an effect or a transition of role.
func (t *Tree) bindFx(cx *Scope, n *node, role driver.ShaderRole, p *ShaderProgram, base Uniforms, over func() Uniforms,
	progress AnimatedNode, progressFunc func() float64, overflow float64, paused bool, pausedFunc func() bool) {
	if p != nil && !p.p.WGSL {
		panic("core: effects and transitions take WGSL programs (CompileWGSL)")
	}
	fx := &fxNode{role: role, program: p, progress: progress, overflow: overflow}
	n.fx = fx
	t.pictured = append(t.pictured, n)
	if p == nil {
		return
	}
	fx.clock = t.bindClock(cx, n, p, paused, pausedFunc)
	if role.Transitions() {
		fx.value = NewAnimatedValue(cx, 0)
	}
	var u Uniforms
	fx.values = func(progress AnimatedNode) *driver.Shader {
		d := p.values(base, u)
		p.builtins(d, fx.clock, progress, overflow)
		return d
	}
	if over == nil && progressFunc == nil {
		t.updateFx(n)
		return
	}
	cx.renderEffect(func() {
		if over != nil {
			u = over()
		}
		if progressFunc != nil {
			fx.progress = Constant(progressFunc())
		}
		t.updateFx(n)
	})
}

func (t *Tree) mountShaderEffect(cx *Scope, parent *node, e ShaderEffect) {
	n := t.newNode(driver.KindView, parent, cx)
	t.setClasses(n, e.Class)
	t.bindStyle(cx, n, e.Style, e.StyleFunc)
	t.setRef(n, e.Ref)
	n.h = handlers{layout: e.OnLayout}
	n.props = driver.ViewProps{TestID: e.TestID}
	t.bindFx(cx, n, driver.ShaderEffect, e.Program, e.Uniforms, e.UniformsFunc, progressOf(e.Progress, e.ProgressNode), e.ProgressFunc, e.MaxSampleOffset, e.Paused, e.PausedFunc)
	for _, c := range e.Children {
		t.mount(cx, n, c)
	}
}

func (t *Tree) mountShaderTransition(cx *Scope, parent *node, e ShaderTransition) {
	n := t.newNode(driver.KindView, parent, cx)
	t.setClasses(n, e.Class)
	t.bindStyle(cx, n, e.Style, e.StyleFunc)
	t.setRef(n, e.Ref)
	n.h = handlers{layout: e.OnLayout}
	n.props = driver.ViewProps{TestID: e.TestID}
	p := e.Program
	if p == nil && !e.NoFallback {
		p = crossfade()
	}
	t.bindFx(cx, n, driver.ShaderTransition, p, e.Uniforms, e.UniformsFunc, nil, nil, e.MaxSampleOffset, e.Paused, e.PausedFunc)
	t.bindTransitionValue(cx, n, e.Value, e.Duration)
	for _, c := range e.Children {
		t.mount(cx, n, c)
	}
}

// progressOf is an effect's progress: its node, else its number.
func progressOf(x float64, n AnimatedNode) AnimatedNode {
	if n != nil {
		return n
	}
	return Constant(x)
}

// screenStyle places a screen shader's view: no space, out of the flow.
var screenStyle = Style{Position: Absolute, Width: Pt(0), Height: Pt(0)}

func (t *Tree) mountScreenEffect(cx *Scope, parent *node, e ScreenEffect) {
	n := t.newNode(driver.KindView, parent, cx)
	t.bindStyle(cx, n, screenStyle, nil)
	t.bindFx(cx, n, driver.ShaderScreen, e.Program, e.Uniforms, e.UniformsFunc, progressOf(e.Progress, e.ProgressNode), e.ProgressFunc, e.MaxSampleOffset, e.Paused, e.PausedFunc)
}

func (t *Tree) mountScreenTransition(cx *Scope, parent *node, e ScreenTransition) {
	n := t.newNode(driver.KindView, parent, cx)
	t.bindStyle(cx, n, screenStyle, nil)
	p := e.Program
	if p == nil && !e.NoFallback {
		p = crossfade()
	}
	t.bindFx(cx, n, driver.ShaderScreenTransition, p, e.Uniforms, e.UniformsFunc, nil, nil, e.MaxSampleOffset, e.Paused, e.PausedFunc)
	t.bindTransitionValue(cx, n, e.Value, e.Duration)
}

// bindTransitionValue runs n's transition each time value's result
// changes after its first.
func (t *Tree) bindTransitionValue(cx *Scope, n *node, value func() any, d time.Duration) {
	if value == nil {
		return
	}
	if d == 0 {
		d = 300 * time.Millisecond
	}
	first, last := true, any(nil)
	cx.renderEffect(func() {
		v := value()
		if first {
			first, last = false, v
			return
		}
		if v == last {
			return
		}
		last = v
		Untracked(cx, func() struct{} {
			t.runTransition(cx, n, d, false)
			return struct{}{}
		})
	})
}

// runTransition starts n's transition, ending one that runs: its
// progress goes from 0 to 1, or from 1 to 0 backward, over d, and the
// platform pictures n as it shows before the batch.
func (t *Tree) runTransition(cx *Scope, n *node, d time.Duration, backward bool) {
	fx := n.fx
	if fx == nil || fx.values == nil || n.deleted {
		return
	}
	if fx.run != nil {
		fx.run.Stop()
	}
	from, to := 0.0, 1.0
	if backward {
		from, to = 1, 0
	}
	fx.runs++
	run := fx.runs
	fx.running, fx.backward = true, backward
	fx.value.Set(from)
	fx.run = cx.StartAnimation(Timing(fx.value, to, TimingConfig{Duration: d, Easing: Easing{Bezier: Linear}}), func(bool) {
		if fx.runs != run || n.deleted {
			return
		}
		fx.running, fx.run = false, nil
		t.updateFx(n)
	})
	t.updateFx(n)
	for _, id := range t.shaderStarts {
		if id == n.id {
			return
		}
	}
	t.shaderStarts = append(t.shaderStarts, n.id)
}

// navShader runs ts over host, a stack as a page comes or goes, the
// surface as a dialog does: backward as it goes. It reports whether it
// runs, which less motion and a program that is not WGSL keep it from.
func (t *Tree) navShader(host *node, ts *TransitionShader, backward bool) bool {
	if ts == nil || ts.Program == nil || !ts.Program.p.WGSL || host == nil || host.deleted {
		return false
	}
	if t.root.Env().reduce.GetUntracked() {
		return false
	}
	cx := host.owner
	fx := host.fx
	if fx == nil {
		fx = &fxNode{role: driver.ShaderNav}
		host.fx = fx
		t.pictured = append(t.pictured, host)
		t.within(cx, func() { fx.value = NewAnimatedValue(cx, 0) })
	}
	p, u := ts.Program, ts.Uniforms
	fx.program = p
	fx.values = func(progress AnimatedNode) *driver.Shader {
		d := p.values(u, nil)
		p.builtins(d, nil, progress, 0)
		return d
	}
	d := ts.Duration
	if d == 0 {
		d = 350 * time.Millisecond
	}
	t.within(cx, func() { t.runTransition(cx, host, d, backward) })
	return true
}

// navigate runs the shader of the page that comes onto stack n or leaves
// it, as its top screen changes.
func (t *Tree) navigate(n *node, top *node) {
	if t.navTops == nil {
		t.navTops = map[*node]*node{}
	}
	last, seen := t.navTops[n]
	t.navTops[n] = top
	if !seen || last == top || last == nil || top == nil {
		return
	}
	// A push puts top above last; anything else takes last away.
	pushed := false
	for _, c := range n.flatChildren() {
		if c == last {
			pushed = true
		}
		if c == top {
			break
		}
	}
	pushed = pushed && !last.screen.popped && !last.deleted
	if pushed {
		t.navShader(n, top.screen.screen.TransitionShader, false)
	} else {
		t.navShader(n, last.screen.screen.TransitionShader, true)
	}
}

// emitFx sends n's shader when it changed.
func (t *Tree) emitFx(n *node) {
	if fx := n.fx; fx != nil && fx.fresh {
		fx.fresh = false
		t.items = append(t.items, driver.MountItem{Kind: driver.ItemShader, ID: n.id, Shader: fx.sent})
	}
}

// markShaderInputs says which pictures the batch changed: an effect's or
// a transition's children, where an item of the batch is, and a screen
// shader's or a backdrop's surface, when anything changed.
func (t *Tree) markShaderInputs() {
	if len(t.pictured) == 0 {
		return
	}
	live := t.pictured[:0]
	for _, n := range t.pictured {
		if !n.deleted {
			live = append(live, n)
		}
	}
	clear(t.pictured[len(live):])
	t.pictured = live
	marked := map[*node]bool{}
	any := false
	for _, it := range t.items {
		switch it.Kind {
		case driver.ItemShader, driver.ItemShaderInput, driver.ItemShaderTransition, driver.ItemAnimation:
			continue
		}
		if it.ID == 0 || int(it.ID) >= len(t.nodes) {
			continue
		}
		any = true
		for p := t.nodes[it.ID]; p != nil; p = p.parent {
			if p.fx != nil && !marked[p] {
				marked[p] = true
			} else if p.fx != nil {
				break
			}
		}
	}
	if !any {
		return
	}
	for _, n := range t.pictured {
		fx := n.fx
		if !fx.pictures() {
			continue
		}
		if marked[n] || fx.role.Covers() || n == t.view || fx.sent.Pictures&driver.PictureBackdrop != 0 {
			t.items = append(t.items, driver.MountItem{Kind: driver.ItemShaderInput, ID: n.id})
		}
	}
}
