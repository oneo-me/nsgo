// Package wgsl compiles the shaders TSX apps write in WGSL, as NSH's are,
// into package shaderir's programs, which every backend draws as it draws
// a Kage program (package shader).
//
// An app writes fn shade(uv: vec2f) -> vec4f, the straight sRGB color of
// the point uv of the view's box (0 to 1 from its top left corner), with
// helpers and consts of its own. It reads nsh.size (the box in points),
// nsh.time (seconds since the shader appeared), nsh.scale (pixels a
// point), nsh.progress (an effect's progress or a transition's, 0 to 1)
// and nsh.overflow (the points the shader draws beyond each side of its
// box), its own values through uniforms when it declares struct
// Uniforms, and pictures of other views with sample_content,
// sample_backdrop, sample_from and sample_to.
//
// The program's uniforms are the bridge's (package shader: the surface's
// size in points, its scale, and the pictures' sizes when it reads any),
// then __nsh, a vec3 of the time, the progress and the overflow
// (BuiltinLanes), then the members of struct Uniforms in turn. Its
// textures are the pictures, content, backdrop, from and to, up to the
// last it reads. The surface a platform draws covers the box and the
// overflow on each side.
package wgsl

import (
	"fmt"
	"strings"

	"github.com/oneo-me/nsgo/internal/shaderir"
)

// Limits on what a shader may be, as NSH's.
const (
	MaxSourceBytes  = 64 << 10
	MaxUniformBytes = 4096
)

// BuiltinLanes is how many lanes __nsh takes after the bridge's: the
// time, the progress and the overflow.
const BuiltinLanes = 3

// The pictures a program reads, a bit each and in the order of its
// textures.
const (
	Content Inputs = 1 << iota
	Backdrop
	From
	To
)

const maxPictures = 4

// Inputs is a set of pictures.
type Inputs uint8

// Program is a compiled shader.
type Program struct {
	IR *shaderir.Program
	// Uniforms are struct Uniforms' members, in their order.
	Uniforms []Uniform
	// Animated is whether shade reads nsh.time, so the shader draws every
	// frame.
	Animated bool
	// Inputs are the pictures shade reads, through the functions it
	// calls.
	Inputs Inputs
}

// Uniform is a member of struct Uniforms.
type Uniform struct {
	Name string
	// Type is its WGSL type (vec4<f32>).
	Type string
	// Lanes is how many lanes it takes; Vec3 is whether it is a vec3f,
	// which a color fills without its alpha.
	Lanes int
	Vec3  bool
}

// Error is why a shader did not compile, at a line and a column (in
// bytes) from 1 of the app's source, or 0 where it has no place there.
type Error struct {
	Line, Column int
	Message      string
	offset       int
}

func (e *Error) Error() string {
	if e.Line == 0 {
		return "shader: " + e.Message
	}
	return fmt.Sprintf("shader: %d:%d: %s", e.Line, e.Column, e.Message)
}

func errorAt(offset int, format string, args ...any) *Error {
	return &Error{offset: offset, Message: fmt.Sprintf(format, args...)}
}

// Compile checks src and writes it as a program.
func Compile(src string) (*Program, error) {
	if len(src) > MaxSourceBytes {
		return nil, &Error{Message: fmt.Sprintf("the source takes %d bytes, more than %d", len(src), MaxSourceBytes)}
	}
	full := src + "\n" + builtinSource()
	var p *Program
	err := catch(func() {
		m, err := parse(full)
		if err != nil {
			panic(bail{err.(*Error)})
		}
		p = compile(m, len(src))
	})
	if err != nil {
		e := err.(*Error)
		if e.offset < len(src) {
			e.Line, e.Column = position(src, e.offset)
		}
		return nil, e
	}
	p.IR.SourceID = shaderir.CalcSourceID([]byte(src))
	return p, nil
}

// position is the line and the column of offset in src, from 1.
func position(src string, offset int) (line, col int) {
	before := src[:offset]
	line = strings.Count(before, "\n") + 1
	return line, offset - strings.LastIndexByte(before, '\n')
}

func compile(m *module, user int) *Program {
	c := &checker{m: m, user: user, prog: &shaderir.Program{}}
	c.check()
	shade := c.global.names["shade"].fn
	time, textures := reach(shade)
	ir := c.prog
	ir.TextureCount = 0
	for i := range maxPictures {
		if textures&(1<<i) != 0 {
			ir.TextureCount = i + 1
		}
	}
	if ir.TextureCount == 0 {
		// No picture is read: the image sizes leave the bridge, and the
		// uniforms after them move down.
		ir.UniformNames = append(ir.UniformNames[:2], ir.UniformNames[3:]...)
		ir.Uniforms = append(ir.Uniforms[:2], ir.Uniforms[3:]...)
		renumber(ir, 2)
	} else {
		ir.Uniforms[2].Length = ir.TextureCount
	}
	return &Program{IR: ir, Uniforms: c.members, Animated: time, Inputs: Inputs(textures)}
}

// renumber moves every uniform after removed down one.
func renumber(p *shaderir.Program, removed int) {
	var walkExpr func(e *shaderir.Expr)
	walkExpr = func(e *shaderir.Expr) {
		if e.Type == shaderir.UniformVariable && e.Index > removed {
			e.Index--
		}
		for i := range e.Exprs {
			walkExpr(&e.Exprs[i])
		}
	}
	var walk func(b *shaderir.Block)
	walk = func(b *shaderir.Block) {
		if b == nil {
			return
		}
		for i := range b.Stmts {
			s := &b.Stmts[i]
			for j := range s.Exprs {
				walkExpr(&s.Exprs[j])
			}
			for _, sb := range s.Blocks {
				walk(sb)
			}
		}
	}
	for _, f := range p.Funcs {
		walk(f.Block)
	}
	walk(p.FragmentFunc.Block)
}

// builtinSource is the source after the app's: the pictures' samplers
// and the entry point, which clamps shade's color and premultiplies it. A
// picture covers the surface, the box and the overflow beyond each side,
// and is read between its pixels, transparent outside it.
func builtinSource() string {
	var b strings.Builder
	b.WriteString(`
fn __nsh_picture(uv: vec2f) -> vec2f {
	return (uv * nsh.size + nsh.overflow) / max(nsh.size + 2.0 * nsh.overflow, vec2f(1e-6));
}

fn __nsh_straight(c: vec4f) -> vec4f {
	if (c.a <= 0.0) {
		return vec4f(0.0);
	}
	return vec4f(c.rgb / c.a, c.a);
}
`)
	for i, name := range []string{"content", "backdrop", "from", "to"} {
		fmt.Fprintf(&b, `
fn sample_%[2]s(uv: vec2f) -> vec4f {
	let p = __nsh_picture(uv);
	if (any(p < vec2f(0.0)) || any(p > vec2f(1.0))) {
		return vec4f(0.0);
	}
	let size = __picture_size(%[1]d);
	let hi = max(size - 1.0, vec2f(0.0));
	let q = p * size - 0.5;
	let i = floor(q);
	let f = q - i;
	let a = clamp(i, vec2f(0.0), hi);
	let b = clamp(i + 1.0, vec2f(0.0), hi);
	let top = mix(__texel(%[1]d, a), __texel(%[1]d, vec2f(b.x, a.y)), f.x);
	let bottom = mix(__texel(%[1]d, vec2f(a.x, b.y)), __texel(%[1]d, b), f.x);
	return __nsh_straight(mix(top, bottom, f.y));
}
`, i, name)
	}
	b.WriteString(`
fn __nsh_fragment(pos: vec2f) -> vec4f {
	let uv = (pos - nsh.overflow) / max(nsh.size, vec2f(1e-6));
	let color = clamp(shade(uv), vec4f(0.0), vec4f(1.0));
	return vec4f(color.rgb * color.a, color.a);
}
`)
	return b.String()
}

// Crossfade is the transitions' default program: from sample_from to
// sample_to as nsh.progress goes from 0 to 1.
const Crossfade = `fn shade(uv: vec2f) -> vec4f {
	return mix(sample_from(uv), sample_to(uv), nsh.progress);
}
`
