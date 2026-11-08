//go:build android

package android

import (
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/shaderir"
	"github.com/oneo-me/nsgo/internal/shaderir/glsl"
	"slices"
)

// Shaders draw with OpenGL ES 3.0 in the Java runtime's ShaderNode, a
// TextureView: Go translates each program into GLSL ES once and sends
// its source with the first item that draws it, with where each uniform
// lane lies in its std140 block; Java compiles it once for its context,
// writes the lanes, the animated ones each frame they change, and draws
// when they, the size or an image change.

// glProgram is a program translated for Java.
type glProgram struct {
	id     uint32
	source string // the fragment stage's GLSL ES, "" when it does not compile
	lanes  []int  // each lane's offset in the block
	types  []byte // and its type: a lane* constant
	size   int
	images int
	sent   bool
}

// What a lane holds, as ShaderNode reads it: a 32-bit float, or a 32-bit
// integer from the value's integer part.
const (
	laneFloat byte = iota
	laneInt
	laneUint
	laneBool
)

// shaderItem is what an ItemShader sends: its program, and whether the
// program's source goes with it.
type shaderItem struct {
	prog   *glProgram
	source bool
}

// glMain is the fragment stage around the program's function: the
// position counts down from the top, as the program's does.
const glMain = `
uniform float nsgoHeight;
out vec4 nsgoColor;

void main() {
	vec2 p = vec2(gl_FragCoord.x, nsgoHeight - gl_FragCoord.y);
	nsgoColor = nsgoShader(vec4(p, gl_FragCoord.z, 1.0), p / U1);
}
`

// trackShaders translates the programs a batch's shader items draw, on
// the engine goroutine, and loads their images.
func (s *surface) trackShaders(b *driver.MountBatch) []shaderItem {
	var out []shaderItem
	for i := range b.Items {
		it := &b.Items[i]
		if it.Kind != driver.ItemShader || it.Shader == nil {
			continue
		}
		p := s.programs[it.Shader.Program]
		if p == nil {
			p = translate(it.Shader.Program)
			s.nextProgram++
			p.id = s.nextProgram
			if s.programs == nil {
				s.programs = map[*driver.ShaderProgram]*glProgram{}
			}
			s.programs[it.Shader.Program] = p
		}
		out = append(out, shaderItem{prog: p, source: !p.sent})
		p.sent = true
		s.loadImages(it.Shader.Images)
	}
	return out
}

// translate turns prog into GLSL ES.
func translate(prog *driver.ShaderProgram) *glProgram {
	p := &glProgram{}
	if prog == nil {
		return p
	}
	ir := prog.IR
	p.source = "#version 300 es\nprecision highp float;\nprecision highp int;\n\n" +
		glsl.Fragment(ir, "nsgoShader", glsl.Options{FlipY: "true", Block: "NsgoUniforms"}) + glMain
	p.lanes, p.size = glsl.UniformLanes(ir)
	for _, t := range ir.LaneTypes() {
		k := laneFloat
		switch t {
		case shaderir.Int:
			k = laneInt
		case shaderir.Uint:
			k = laneUint
		case shaderir.Bool:
			k = laneBool
		}
		p.types = append(p.types, k)
	}
	p.images = ir.TextureCount
	return p
}

// writeShader writes an ItemShader as ShaderNode.set reads it.
func writeShader(w *writer, sh *driver.Shader, it shaderItem) {
	p := it.prog
	w.u32(p.id)
	w.flags(it.source)
	if it.source {
		w.str(p.source)
		w.u32(uint32(len(p.lanes)))
		for i, off := range p.lanes {
			w.u32(uint32(off))
			w.byte(p.types[i])
		}
		w.u32(uint32(p.size))
		w.u32(uint32(p.images))
	}
	w.u32(uint32(len(sh.Lanes)))
	for _, v := range sh.Lanes {
		w.f64(v)
	}
	w.u32(uint32(len(sh.Nodes)))
	for _, n := range sh.Nodes {
		w.u32(uint32(n.Lane))
		w.u32(n.Node)
	}
	w.u32(uint32(len(sh.Images)))
	for _, img := range sh.Images {
		w.str(img.Key())
	}
}

// withoutEffects leaves out of b what the Java runtime does not draw yet:
// effects, transitions and navigation shaders, and their starts and
// pictures, so their children show as they are and pages and dialogs
// come with their own motion.
func withoutEffects(b *driver.MountBatch) *driver.MountBatch {
	effect := func(it *driver.MountItem) bool {
		switch it.Kind {
		case driver.ItemShaderTransition, driver.ItemShaderInput:
			return true
		case driver.ItemShader:
			return it.Shader == nil || it.Shader.Role != driver.ShaderFill
		}
		return false
	}
	if !slices.ContainsFunc(b.Items, func(it driver.MountItem) bool { return effect(&it) }) {
		return b
	}
	out := *b
	out.Items = slices.DeleteFunc(slices.Clone(b.Items), func(it driver.MountItem) bool { return effect(&it) })
	return &out
}
