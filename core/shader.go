package core

import (
	"fmt"
	"slices"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/shader"
	"github.com/oneo-me/nsgo/internal/shaderir"
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
type ShaderProgram struct {
	p        *driver.ShaderProgram
	uniforms map[string]uniformSlot
	lanes    int
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
func CompileShader(src string) (*ShaderProgram, error) {
	p, err := shader.CompileFragment([]byte(src))
	if err != nil {
		return nil, err
	}
	sp := &ShaderProgram{p: &driver.ShaderProgram{IR: p}, uniforms: map[string]uniformSlot{}}
	lane := 0
	for i, name := range p.UniformNames {
		if name == shader.RectSizeUniform || name == shader.ScaleUniform || name == shader.ImageSizesUniform {
			continue
		}
		t := p.Uniforms[i]
		n := t.DwordCount()
		if t.Main == shaderir.Array {
			t = t.Sub[0]
		}
		sp.uniforms[name] = uniformSlot{lane: lane, n: n, vec3: t.Main == shaderir.Vec3}
		lane += n
	}
	sp.lanes = lane
	return sp, nil
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
	// Images are what the program's image0At to image3At read, in order,
	// loaded as an Image loads its source.
	Images     []ImageSource
	ImagesFunc func() []ImageSource
	Style      Style
	StyleFunc  func() Style
	OnPress    func(PressEvent)
	OnLayout   func(Bounds)
	Ref        *Ref
	// Label names what the shader shows for assistive technology, which
	// skips an unlabeled shader as a decoration.
	Label  string
	TestID string
}

// Render implements Node.
func (s Shader) Render(*Scope) Node { return s }

type shaderNode struct {
	sent  *driver.Shader
	fresh bool
}

func (t *Tree) mountShader(cx *Scope, parent *node, s Shader) {
	n := t.newNode(driver.KindShader, parent, cx)
	t.bindStyle(cx, n, s.Style, s.StyleFunc)
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
	set := func(u Uniforms, images []ImageSource) {
		d := s.Program.values(s.Uniforms, u)
		for _, img := range images {
			d.Images = append(d.Images, t.pickScale(img))
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

// values returns what a shader view draws with base's uniforms and over's
// over them: a uniform over names replaces base's value whole.
func (p *ShaderProgram) values(base, over Uniforms) *driver.Shader {
	w := laneWriter{lanes: make([]float64, p.lanes), nodes: make([]uint32, p.lanes)}
	for _, u := range [2]Uniforms{base, over} {
		for name, v := range u {
			slot, ok := p.uniforms[name]
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
	for lane, node := range w.nodes {
		if node != 0 {
			d.Nodes = append(d.Nodes, driver.ShaderNode{Lane: lane, Node: node})
		}
	}
	return d
}

// laneWriter writes uniforms' values into their lanes, and the animated
// nodes that drive lanes.
type laneWriter struct {
	lanes   []float64
	nodes   []uint32
	at, end int
	name    string
	vec3    bool
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
		w.lane(0, x.animated().id)
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
