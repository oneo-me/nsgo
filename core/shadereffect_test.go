package core

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

const waveWGSL = `struct Uniforms { tint: vec3f, speed: f32 }
fn shade(uv: vec2f) -> vec4f {
	return vec4f(uniforms.tint * (0.5 + 0.5 * sin(uv.x + nsh.time * uniforms.speed)), 1.0);
}`

const blurWGSL = `fn shade(uv: vec2f) -> vec4f {
	return (sample_content(uv - 0.01) + sample_content(uv + 0.01)) * 0.5 * nsh.progress;
}`

func shaderItemOf(b *driver.MountBatch, id NodeID) *driver.Shader {
	for _, it := range b.Items {
		if it.Kind == driver.ItemShader && it.ID == id {
			return it.Shader
		}
	}
	return nil
}

func countItems(b *driver.MountBatch, kind driver.ItemKind, id NodeID) int {
	n := 0
	for _, it := range b.Items {
		if it.Kind == kind && it.ID == id {
			n++
		}
	}
	return n
}

// finish reports the end of the runs batch b started that move value.
func finish(h *host, b *driver.MountBatch, value *AnimatedValue) {
	for _, it := range b.Items {
		for _, op := range it.Animation {
			if op.Kind == driver.AnimStart && op.Animation != nil && op.Animation.Value == value.id {
				h.tree.Dispatch(driver.Event{Kind: driver.EventAnimation, Count: op.ID, Finished: true})
			}
		}
	}
}

func TestCompileWGSLCaches(t *testing.T) {
	a, err := CompileWGSL(waveWGSL)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := CompileWGSL(waveWGSL); b != a {
		t.Error("the same source compiled twice")
	}
	if _, err := CompileWGSL("fn shade(uv: vec2f) -> vec4f { return 1.0; }"); err == nil {
		t.Error("a wrong source compiled")
	}
}

// A WGSL program's first lanes are the built-ins: its clock, which runs
// on the platform's frame clock until paused, the progress and the
// overflow.
func TestWGSLShaderLanes(t *testing.T) {
	prog := MustCompileWGSL(waveWGSL)
	var paused *Signal[bool]
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		paused = NewSignal(cx, false)
		return Shader{Program: prog, Style: Style{Height: Pt(40)}, TestID: "s",
			Uniforms: Uniforms{"tint": RGB(0xff0000), "speed": 2}, PausedFunc: paused.Get}
	})
	b := h.commit()
	sh := shaderItem(b)
	if sh == nil || !sh.Program.WGSL {
		t.Fatalf("no WGSL shader: %+v", sh)
	}
	if want := []float64{0, 0, 0, 1, 0, 0, 2}; !slices.Equal(sh.Lanes, want) {
		t.Errorf("lanes = %v, want %v", sh.Lanes, want)
	}
	if len(sh.Nodes) != 1 || sh.Nodes[0].Lane != 0 {
		t.Fatalf("nodes = %v, want the clock in lane 0", sh.Nodes)
	}
	clock := sh.Nodes[0].Node
	started := false
	for _, it := range b.Items {
		for _, op := range it.Animation {
			if op.Kind == driver.AnimStart && op.Animation.Value == clock && op.Animation.To == 3600 {
				started = true
			}
		}
	}
	if !started {
		t.Error("the clock does not run")
	}
	h.clock.Advance(1500 * time.Millisecond)
	paused.Set(true)
	b = h.commit()
	set := false
	for _, it := range b.Items {
		for _, op := range it.Animation {
			if op.Kind == driver.AnimSet && op.Node == clock && op.Value == 1.5 {
				set = true
			}
		}
	}
	if !set {
		t.Error("pausing does not hold the clock at the time it ran")
	}
}

// An effect is a view with children that sends its shader with its role
// and what it pictures, and says when its children changed.
func TestShaderEffect(t *testing.T) {
	prog := MustCompileWGSL(blurWGSL)
	var label *Signal[string]
	var progress *AnimatedValue
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		label = NewSignal(cx, "a")
		progress = NewAnimatedValue(cx, 0)
		return View{Children: []Node{
			ShaderEffect{Program: prog, ProgressNode: progress, MaxSampleOffset: 4, TestID: "fx", Children: []Node{
				Text{ContentFunc: label.Get},
			}},
			Text{Content: "outside", TestID: "out"},
		}}
	})
	b := h.commit()
	fx := h.find("fx")
	sh := shaderItemOf(b, fx.id)
	if sh == nil || sh.Role != driver.ShaderEffect || sh.Pictures != driver.PictureContent || sh.Overflow != 4 || sh.Lanes[2] != 4 {
		t.Fatalf("effect shader = %+v", sh)
	}
	if !slices.Contains(sh.Nodes, driver.ShaderNode{Lane: 1, Node: progress.id}) {
		t.Errorf("nodes = %v, want the progress in lane 1", sh.Nodes)
	}
	label.Set("b")
	if b := h.commit(); countItems(b, driver.ItemShaderInput, fx.id) != 1 {
		t.Error("a change in the children does not picture them again")
	}
}

// A transition runs as its value changes: its start comes first in the
// batch, its shader runs, and its end shows the children as they are.
func TestShaderTransition(t *testing.T) {
	var page *Signal[int]
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		page = NewSignal(cx, 0)
		return ShaderTransition{Value: func() any { return page.Get() }, Duration: 200 * time.Millisecond, TestID: "tr",
			Children: []Node{Text{ContentFunc: func() string { return []string{"one", "two"}[page.Get()] }}}}
	})
	b := h.commit()
	tr := h.find("tr")
	sh := shaderItemOf(b, tr.id)
	if sh == nil || sh.Role != driver.ShaderTransition || sh.Running || sh.Pictures != driver.PictureFrom|driver.PictureTo {
		t.Fatalf("idle transition = %+v", sh)
	}
	if countItems(b, driver.ItemShaderTransition, tr.id) != 0 {
		t.Error("the first value ran a transition")
	}
	page.Set(1)
	b = h.commit()
	if b.Items[0].Kind != driver.ItemShaderTransition || b.Items[0].ID != tr.id {
		t.Fatalf("the transition does not start first in its batch: %v", b.Items[0])
	}
	if sh := shaderItemOf(b, tr.id); sh == nil || !sh.Running || sh.Backwards {
		t.Fatalf("running transition = %+v", sh)
	}
	finish(h, b, tr.fx.value)
	if sh := shaderItemOf(h.commit(), tr.id); sh == nil || sh.Running {
		t.Errorf("the ended transition = %+v", sh)
	}
}

// Screen shaders take no space and picture the surface on any change.
func TestScreenEffect(t *testing.T) {
	prog := MustCompileWGSL(blurWGSL)
	var label *Signal[string]
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		label = NewSignal(cx, "a")
		return View{Children: []Node{Text{ContentFunc: label.Get}, ScreenEffect{Program: prog, Progress: 0.5}}}
	})
	b := h.commit()
	var fx *node
	for _, n := range h.tree.nodes {
		if n != nil && n.fx != nil {
			fx = n
		}
	}
	sh := shaderItemOf(b, fx.id)
	if sh == nil || sh.Role != driver.ShaderScreen || sh.Lanes[1] != 0.5 {
		t.Fatalf("screen shader = %+v", sh)
	}
	if r := fx.lstate.Rounded.Size; r.Width != 0 || r.Height != 0 {
		t.Errorf("the screen effect takes %v", r)
	}
	label.Set("b")
	if b := h.commit(); countItems(b, driver.ItemShaderInput, fx.id) != 1 {
		t.Error("a change does not picture the surface again")
	}
}

// A page's transitionShader runs over its stack as it is pushed, and
// backward as it is popped.
func TestNavShader(t *testing.T) {
	prog := MustCompileWGSL(`fn shade(uv: vec2f) -> vec4f { return select(sample_from(uv), sample_to(uv), uv.x < nsh.progress); }`)
	var pages *Signal[[]string]
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		pages = NewSignalFunc(cx, []string{"home"}, nil)
		return Stack{TestID: "stack", Children: []Node{Each(pages.Get, func(p string) string { return p },
			func(cx *Scope, p func() string, _ func() int) Node {
				return Screen{Title: p(), TransitionShader: &TransitionShader{Program: prog}, Children: []Node{Text{Content: p()}}}
			})}}
	})
	h.commit()
	stack := h.find("stack")
	pages.Set([]string{"home", "detail"})
	b := h.commit()
	if b.Items[0].Kind != driver.ItemShaderTransition || b.Items[0].ID != stack.id {
		t.Fatalf("a push does not start the shader first: %v", b.Items[0])
	}
	sh := shaderItemOf(b, stack.id)
	if sh == nil || sh.Role != driver.ShaderNav || !sh.Running || sh.Backwards {
		t.Fatalf("push shader = %+v", sh)
	}
	finish(h, b, stack.fx.value)
	h.commit()
	pages.Set([]string{"home"})
	b = h.commit()
	if sh := shaderItemOf(b, stack.id); sh == nil || !sh.Running || !sh.Backwards {
		t.Fatalf("pop shader = %+v", sh)
	}
}

// A dialog's transitionShader runs over the surface it covers as it comes
// and as it goes.
func TestModalShader(t *testing.T) {
	prog := MustCompileWGSL(wgslFade)
	var open *Signal[bool]
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		open = NewSignal(cx, false)
		return View{Children: []Node{Show{When: open.Get, Then: func(cx *Scope) Node {
			return Modal{TransitionShader: &TransitionShader{Program: prog, Duration: time.Second}, Children: []Node{Text{Content: "hi"}}}
		}}}}
	})
	h.commit()
	open.Set(true)
	b := h.commit()
	root := h.tree.view
	if b.Items[0].Kind != driver.ItemShaderTransition || b.Items[0].ID != root.id {
		t.Fatalf("presenting does not start the shader on the surface first: %v", b.Items[0])
	}
	if sh := shaderItemOf(b, root.id); sh == nil || !sh.Running || sh.Backwards {
		t.Fatalf("present shader = %+v", sh)
	}
	open.Set(false)
	if sh := shaderItemOf(h.commit(), root.id); sh == nil || !sh.Running || !sh.Backwards {
		t.Fatalf("dismiss shader = %+v", sh)
	}
}

const wgslFade = `fn shade(uv: vec2f) -> vec4f { return mix(sample_from(uv), sample_to(uv), nsh.progress); }`
