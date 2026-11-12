package core

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// ShaderProgram is a fragment shader written in Go's syntax: Ebitengine's
// Kage language, as MyGo's shaders are. The source is a Go file of package
// main whose function Fragment returns the color of a pixel, premultiplied:
//
//	package main
//
//	var Time float
//
//	func Fragment(dst vec4, pos vec2) vec4 {
//		v := 0.5 + 0.5*sin(pos.x/20+Time)
//		return vec4(v, v, 1, 1)
//	}
//
// dst is the pixel's center in pixels from the shader view's top left
// corner, and pos the same point in points. The language has the types
// bool, int, uint, float, vec2 to vec4, ivec2 to ivec4, uvec2 to uvec4,
// mat2 to mat4 and arrays of them, GLSL's built-in functions (sin, mix,
// smoothstep, dot, ...), functions of its own, if, and for loops.
// Exported variables at the package level are uniforms, which
// Shader.Uniforms sets. rectSize returns the view's size in points, and
// pixelScale the pixels a point. image0At(pos) to image3At(pos) read
// Shader.Images at a position in their pixels, premultiplied and
// transparent outside them, and image0Size() to image3Size() give their
// sizes in pixels.
//
// Each platform translates the program for its GPU once, the first time a
// view shows it.
//
// A program may be written in WGSL instead (CompileWGSL), as TSX apps
// write theirs and as effects and transitions need.
type ShaderProgram struct {
	p        *driver.ShaderProgram
	uniforms map[string]uniformSlot
	lanes    int
	// animated is whether a WGSL program reads nsh.time; pictures what
	// it samples.
	animated bool
	pictures driver.Pictures
	// uncompiled is a program of a build without the compilers
	// (shader_nocompile.go), which takes any uniform.
	uncompiled bool
}

// uniformSlot is where a uniform's lanes lie among the program's own,
// and whether it (or each of its elements) is a vec3, which a Color fills
// without its alpha.
type uniformSlot struct {
	lane, n int
	vec3    bool
}

// CompileShader compiles a shader's source, as ShaderProgram describes
// it. Its errors give the source's lines.
func CompileShader(src string) (*ShaderProgram, error) { return compileFragment(src) }

// CompileWGSL compiles a shader written in WGSL, as NSH's are: it defines
//
//	fn shade(uv: vec2f) -> vec4f
//
// the straight (not premultiplied) sRGB color of the point uv of the
// view's box, 0 to 1 from its top left corner, with whatever helpers and
// consts it needs. It reads nsh.size (the box in points), nsh.time
// (seconds since the shader appeared, standing still while paused),
// nsh.scale (pixels a point), nsh.progress (an effect's progress or how
// far a transition is, 0 to 1) and nsh.overflow (MaxSampleOffset). When
// it declares struct Uniforms, of f32, i32, u32, vec2f, vec3f, vec4f and
// array<vec4f, N> members in at most 4096 bytes, it reads them through
// uniforms, which Uniforms sets by name. sample_content, sample_backdrop,
// sample_from and sample_to read what an effect or a transition pictures
// at a point of the box, straight, transparent outside the picture.
//
// Its errors give the source's lines and columns. A source compiles once
// for the process, as an app sets the same one again.
func CompileWGSL(src string) (*ShaderProgram, error) {
	wgslCache.Lock()
	defer wgslCache.Unlock()
	if c, ok := wgslCache.programs[src]; ok {
		return c.p, c.err
	}
	p, err := compileWGSL(src)
	if len(wgslCache.programs) >= 64 {
		clear(wgslCache.programs)
	}
	wgslCache.programs[src] = compiledWGSL{p, err}
	return p, err
}

// wgslCache holds the programs compiled so far, by their source, up to
// 64, as NSH's cache does: apps set one source on many views, in list
// rows and on every hot reload.
var wgslCache = struct {
	sync.Mutex
	programs map[string]compiledWGSL
}{programs: map[string]compiledWGSL{}}

type compiledWGSL struct {
	p   *ShaderProgram
	err error
}

// MustCompileWGSL is CompileWGSL for a source that is part of the
// program: it panics when it does not compile.
func MustCompileWGSL(src string) *ShaderProgram {
	p, err := CompileWGSL(src)
	if err != nil {
		panic(err)
	}
	return p
}

// crossfade is the transitions' default program, compiled on first use:
// a package variable's initializer would link the compiler into every
// app.
func crossfade() *ShaderProgram {
	crossfadeProgram.Do(func() { crossfadeProgram.p = MustCompileWGSL(crossfadeSource) })
	return crossfadeProgram.p
}

var crossfadeProgram struct {
	sync.Once
	p *ShaderProgram
}

// MustCompileShader is CompileShader for a source that is part of the
// program, such as a package variable's: it panics when it does not
// compile.
func MustCompileShader(src string) *ShaderProgram {
	p, err := CompileShader(src)
	if err != nil {
		panic(err)
	}
	return p
}

// HasUniform reports whether the program has a uniform named name, which
// Uniforms may set.
func (p *ShaderProgram) HasUniform(name string) bool {
	_, ok := p.uniforms[name]
	return ok
}

// Uniforms are the values of a shader's uniform variables, by name. A
// value is a number (any of Go's integer or float kinds, or a bool), an
// AnimatedNode, a Color, or a slice of numbers or nodes or of any of these
// ([]any), which fill the uniform's lanes in turn: a vector's components,
// a matrix's columns, an array's elements. A Color fills four lanes, red,
// green, blue and alpha from 0 to 1, not premultiplied, or three for a
// vec3. Lanes left out are 0.
//
// An AnimatedNode's lane follows the node on the platform's frame clock,
// with no engine work per frame: a Loop of a Timing makes a clock, and a
// value Set with each pointer move puts a glow under the pointer.
type Uniforms map[string]any

// Shader fills its box with a fragment shader, which the GPU runs for each
// pixel: an animated background, a gradient no style describes. It draws
// again when its size, its uniforms or the animated nodes they hold
// change; a platform without a GPU path for shaders yet leaves it empty.
type Shader struct {
	Program      *ShaderProgram
	Uniforms     Uniforms
	UniformsFunc func() Uniforms
	// Paused stops a WGSL program's clock, nsh.time; PausedFunc, when
	// set, says so as the graph changes.
	Paused     bool
	PausedFunc func() bool
	// Images are what the program's image0At to image3At read, in order,
	// loaded as an Image loads its source.
	Images     []ImageSource
	ImagesFunc func() []ImageSource
	// Class is Tailwind CSS classes, as View's.
	Class     string
	Style     Style
	StyleFunc func() Style
	// Children draw over the shader.
	Children []Node
	OnPress  func(PressEvent)
	OnLayout func(Bounds)
	Ref      *Ref
	// Label names what the shader shows for assistive technology, which
	// skips an unlabeled shader as a decoration.
	Label  string
	TestID string
}

// Render implements Node.
func (s Shader) Render(*Scope) Node { return s }

func (s Shader) mountIn(t *Tree, cx *Scope, parent *node) { t.mountShader(cx, parent, s) }

type shaderNode struct {
	sent  *driver.Shader
	fresh bool
}

func (t *Tree) mountShader(cx *Scope, parent *node, s Shader) {
	n := t.newNode(driver.KindShader, parent, cx)
	t.setClasses(n, s.Class)
	t.bindStyle(cx, n, s.Style, s.StyleFunc)
	for _, c := range s.Children {
		t.mount(cx, n, c)
	}
	t.setRef(n, s.Ref)
	n.h = handlers{press: s.OnPress, layout: s.OnLayout}
	role := RoleNone
	if s.Label != "" {
		role = RoleImage
	}
	n.props = driver.ViewProps{Role: role, Label: s.Label, TestID: s.TestID}
	sn := &shaderNode{}
	n.shader = sn
	if s.Program == nil {
		return
	}
	clock := t.bindClock(cx, n, s.Program, s.Paused, s.PausedFunc)
	set := func(u Uniforms, images []ImageSource) {
		d := s.Program.values(s.Uniforms, u)
		s.Program.builtins(d, clock, nil, 0)
		for _, img := range images {
			img = t.pickScale(img)
			if t.rt.restricted {
				var refused string
				t.refuseImage(n, &img, &refused, nil)
			}
			d.Images = append(d.Images, img)
		}
		if old := sn.sent; old != nil && slices.Equal(old.Lanes, d.Lanes) && slices.Equal(old.Nodes, d.Nodes) &&
			slices.EqualFunc(old.Images, d.Images, ImageSource.Same) {
			return
		}
		sn.sent, sn.fresh = d, true
		t.touch(n)
	}
	if s.UniformsFunc == nil && s.ImagesFunc == nil {
		set(nil, s.Images)
		return
	}
	cx.renderEffect(func() {
		var u Uniforms
		if s.UniformsFunc != nil {
			u = s.UniformsFunc()
		}
		images := s.Images
		if s.ImagesFunc != nil {
			images = s.ImagesFunc()
		}
		set(u, images)
	})
}

// builtins writes a WGSL program's built-in lanes into d: the clock's
// node, the progress (a node or a number) and the overflow.
func (p *ShaderProgram) builtins(d *driver.Shader, clock *shaderClock, progress AnimatedNode, overflow float64) {
	if !p.p.WGSL {
		return
	}
	d.Pictures = p.pictures
	d.Lanes[2] = overflow
	d.Overflow = overflow
	var nodes []driver.ShaderNode
	if clock != nil {
		nodes = append(nodes, driver.ShaderNode{Lane: 0, Node: clock.v.id})
	}
	if progress != nil {
		if a := progress.animated(); a != nil {
			nodes = append(nodes, driver.ShaderNode{Lane: 1, Node: a.id})
		} else if c, ok := progress.(Constant); ok {
			d.Lanes[1] = float64(c)
		}
	}
	d.Nodes = append(nodes, d.Nodes...)
}

// shaderClock is a WGSL program's nsh.time: a value the platform moves
// one a second on its frame clock, standing still while paused. Each run
// goes an hour and the next starts where it ended.
type shaderClock struct {
	v       *AnimatedValue
	run     *AnimationRun
	elapsed float64 // the seconds run before since
	since   time.Time
	running bool
}

const clockSpan = time.Hour

// bindClock gives n the clock of a program reading nsh.time, running
// unless paused, or nil for one that does not.
func (t *Tree) bindClock(cx *Scope, n *node, p *ShaderProgram, paused bool, pausedFunc func() bool) *shaderClock {
	if p == nil || !p.animated {
		return nil
	}
	c := &shaderClock{v: NewAnimatedValue(cx, 0)}
	if pausedFunc == nil {
		c.set(cx, paused)
		return c
	}
	cx.renderEffect(func() { c.set(cx, pausedFunc()) })
	return c
}

func (c *shaderClock) set(cx *Scope, paused bool) {
	if paused != c.running {
		return
	}
	now := cx.tree().rt.clock.Now()
	if paused {
		c.elapsed += now.Sub(c.since).Seconds()
		c.running = false
		Untracked(cx, func() struct{} {
			c.run.Stop()
			c.v.Set(c.elapsed)
			return struct{}{}
		})
		return
	}
	c.since, c.running = now, true
	Untracked(cx, func() struct{} { c.loop(cx); return struct{}{} })
}

func (c *shaderClock) loop(cx *Scope) {
	from := c.elapsed
	c.v.Set(from)
	span := clockSpan.Seconds()
	c.run = cx.startAnimation(Timing(c.v, from+span, TimingConfig{Duration: clockSpan, Easing: Easing{Bezier: Linear}}), func(finished bool) {
		if finished && c.running {
			c.elapsed, c.since = from+span, cx.tree().rt.clock.Now()
			c.loop(cx)
		}
	}, true)
}

// values returns what a shader view draws with base's uniforms and over's
// over them: a uniform over names replaces base's value whole.
func (p *ShaderProgram) values(base, over Uniforms) *driver.Shader {
	w := laneWriter{lanes: make([]float64, p.lanes), nodes: make([]uint32, p.lanes), channels: make([]driver.Channel, p.lanes)}
	for _, u := range [2]Uniforms{base, over} {
		for name, v := range u {
			slot, ok := p.uniforms[name]
			if !ok && p.uncompiled {
				continue
			}
			if !ok {
				panic(fmt.Sprintf("core: the shader has no uniform %s", name))
			}
			clear(w.lanes[slot.lane : slot.lane+slot.n])
			clear(w.nodes[slot.lane : slot.lane+slot.n])
			w.at, w.end, w.name, w.vec3 = slot.lane, slot.lane+slot.n, name, slot.vec3
			w.put(v)
		}
	}
	d := &driver.Shader{Program: p.p, Lanes: w.lanes}
	if p.p.WGSL {
		// A uniform named like a built-in lane cannot write over it.
		clear(w.nodes[:builtinLanes])
	}
	for lane, node := range w.nodes {
		if node != 0 {
			d.Nodes = append(d.Nodes, driver.ShaderNode{Lane: lane, Node: node, Channel: w.channels[lane]})
		}
	}
	return d
}

// laneWriter writes uniforms' values into their lanes, and the animated
// nodes that drive lanes.
type laneWriter struct {
	lanes    []float64
	nodes    []uint32
	channels []driver.Channel
	at, end  int
	name     string
	vec3     bool
}

func (w *laneWriter) lane(x float64, node uint32) {
	if w.at >= w.end {
		panic(fmt.Sprintf("core: the shader's uniform %s has fewer lanes than its value", w.name))
	}
	w.lanes[w.at], w.nodes[w.at] = x, node
	w.at++
}

func (w *laneWriter) put(v any) {
	switch x := v.(type) {
	case float64:
		w.lane(x, 0)
	case float32:
		w.lane(float64(x), 0)
	case int:
		w.lane(float64(x), 0)
	case int32:
		w.lane(float64(x), 0)
	case int64:
		w.lane(float64(x), 0)
	case uint:
		w.lane(float64(x), 0)
	case uint32:
		w.lane(float64(x), 0)
	case uint64:
		w.lane(float64(x), 0)
	case bool:
		b := 0.0
		if x {
			b = 1
		}
		w.lane(b, 0)
	case Constant:
		w.lane(float64(x), 0)
	case Color:
		r, g, b, a := x.Components()
		w.lane(r, 0)
		w.lane(g, 0)
		w.lane(b, 0)
		if !w.vec3 {
			w.lane(a, 0)
		}
	case AnimatedNode:
		a := x.animated()
		if a == nil {
			w.lane(float64(x.(Constant)), 0)
			return
		}
		if !a.color {
			w.lane(0, a.id)
			return
		}
		// A color node fills a color's lanes, as a Color does.
		for c := driver.ChannelRed; c <= driver.ChannelAlpha; c++ {
			if c == driver.ChannelAlpha && w.vec3 {
				break
			}
			w.channels[min(w.at, len(w.channels)-1)] = c
			w.lane(0, a.id)
		}
	case []float64:
		for _, e := range x {
			w.lane(e, 0)
		}
	case []float32:
		for _, e := range x {
			w.lane(float64(e), 0)
		}
	case []int:
		for _, e := range x {
			w.lane(float64(e), 0)
		}
	case []AnimatedNode:
		for _, e := range x {
			w.put(e)
		}
	case []any:
		for _, e := range x {
			w.put(e)
		}
	case nil:
	default:
		panic(fmt.Sprintf("core: the shader's uniform %s is set to a %T, not numbers, nodes or a Color", w.name, v))
	}
}
