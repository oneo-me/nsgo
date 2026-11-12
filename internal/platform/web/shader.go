//go:build js && wasm

package web

import (
	"log"
	"math"
	"strconv"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/shaderir"
	"github.com/nsgo-dev/nsgo/internal/shaderir/glsl"
)

// A shader view draws with WebGL 2 into a <canvas> of its own, whose
// pixels follow the pixel ratio: when its uniforms, its size or the
// animated nodes its lanes follow change, in the next animation frame, so
// at most once a frame however many batches arrive. Each view has its own
// context, which compiles its program the first time it draws it; browsers
// keep a few dozen contexts at most.

// glShader is a shader view's WebGL state.
type glShader struct {
	gl      js.Value // WebGL2RenderingContext, null where the browser has none
	shader  *driver.Shader
	program *driver.ShaderProgram // what prog was linked from
	prog    js.Value
	ubo     js.Value
	height  js.Value // the uniform location of the drawable's height
	offsets []int
	types   []shaderir.BasicType
	lanes   []float64
	buf     []byte
	bytes   js.Value // a Uint8Array as large as buf
	failed  bool
	// textures are the images' textures in this context, by
	// ImageSource.Key, and textureCount how many the program reads.
	textures     map[string]glTexture
	textureCount int
}

// glTexture is an image's texture and its size in pixels.
type glTexture struct {
	tex  js.Value
	w, h float64
}

const glVertex = `#version 300 es
void main() {
	vec2 uv = vec2(float((gl_VertexID << 1) & 2), float(gl_VertexID & 2));
	gl_Position = vec4(uv * 2.0 - 1.0, 0.0, 1.0);
}
`

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

func (s *surface) createShader(v *view) {
	v.inner = s.element("canvas")
	setStyle(v.inner, map[string]string{"width": "100%", "height": "100%", "display": "block"})
	v.el.Call("appendChild", v.inner)
	opts := map[string]any{"alpha": true, "premultipliedAlpha": true, "antialias": false, "depth": false, "stencil": false}
	v.gl = &glShader{gl: v.inner.Call("getContext", "webgl2", opts)}
}

// setShader gives v what it draws and binds its animated lanes.
func (s *surface) setShader(v *view, sh *driver.Shader) {
	v.gl.shader = sh
	s.animator.BindShader(v.id, sh.Nodes)
	s.shadeLater(v)
}

// texture returns image src's texture in g's context, made the first
// time it is drawn after the browser loaded it; ok is false until then,
// and v draws again once it loads.
func (s *surface) texture(v *view, src driver.ImageSource) (glTexture, bool) {
	g := v.gl
	key := src.Key()
	if t, ok := g.textures[key]; ok {
		return t, true
	}
	im := s.loadImage(src, v)
	if !im.loaded {
		return glTexture{}, false
	}
	gl := g.gl
	t := glTexture{tex: gl.Call("createTexture"), w: im.img.Get("naturalWidth").Float(), h: im.img.Get("naturalHeight").Float()}
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), t.tex)
	// Shaders read premultiplied colors, the image's top row first.
	gl.Call("pixelStorei", gl.Get("UNPACK_PREMULTIPLY_ALPHA_WEBGL"), true)
	gl.Call("texImage2D", gl.Get("TEXTURE_2D"), 0, gl.Get("RGBA"), gl.Get("RGBA"), gl.Get("UNSIGNED_BYTE"), im.img)
	for _, p := range []string{"TEXTURE_MIN_FILTER", "TEXTURE_MAG_FILTER"} {
		gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get(p), gl.Get("NEAREST"))
	}
	for _, p := range []string{"TEXTURE_WRAP_S", "TEXTURE_WRAP_T"} {
		gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get(p), gl.Get("CLAMP_TO_EDGE"))
	}
	if g.textures == nil {
		g.textures = map[string]glTexture{}
	}
	g.textures[key] = t
	return t, true
}

// shadeLater draws v in the next animation frame.
func (s *surface) shadeLater(v *view) {
	s.queueShader(v)
	s.startTicking()
}

func (s *surface) queueShader(v *view) {
	for _, w := range s.shading {
		if w == v {
			return
		}
	}
	s.shading = append(s.shading, v)
}

// changedShaders queues the shader views whose animated lanes changed,
// for the frame to draw.
func (s *surface) changedShaders() {
	s.animator.ChangedShaders(func(id driver.NodeID) {
		if v := s.view(id); v != nil && v.gl != nil && v.fx == nil {
			s.queueShader(v)
		}
	})
}

// drawShaders draws the shader views waiting to, in an animation frame.
func (s *surface) drawShaders() {
	for _, v := range s.shading {
		if s.view(v.id) == v && v.gl != nil {
			s.drawShader(v)
		}
	}
	clear(s.shading)
	s.shading = s.shading[:0]
	s.drawEffects()
}

func (s *surface) drawShader(v *view) {
	g := v.gl
	w := math.Max(float64(v.size.Width-v.border.Left-v.border.Right), 0)
	h := math.Max(float64(v.size.Height-v.border.Top-v.border.Bottom), 0)
	s.drawGL(g, v.inner, v.id, w, h, func(i int) (js.Value, float64, float64) {
		if i < len(g.shader.Images) {
			if t, ok := s.texture(v, g.shader.Images[i]); ok {
				return t.tex, t.w, t.h
			}
		}
		return js.Undefined(), 0, 0
	})
}

// drawGL draws g's shader on canvas, w by h points, its animated lanes
// those of view id's, with the textures textures gives: undefined for one
// that reads as transparent. It reports whether it drew.
func (s *surface) drawGL(g *glShader, canvas js.Value, id driver.NodeID, w, h float64, textures func(i int) (js.Value, float64, float64)) bool {
	gl := g.gl
	if gl.IsNull() || g.shader == nil || g.failed && g.program == g.shader.Program {
		return false
	}
	if g.program != g.shader.Program && !g.link(g.shader.Program) {
		return false
	}
	ratio := float64(s.env.PixelRatio)
	if ratio <= 0 {
		ratio = 1
	}
	pw, ph := math.Ceil(w*ratio), math.Ceil(h*ratio)
	if pw == 0 || ph == 0 {
		return false
	}
	if canvas.Get("width").Float() != pw || canvas.Get("height").Float() != ph {
		canvas.Set("width", pw)
		canvas.Set("height", ph)
	}

	bridge := 3 + 2*g.textureCount
	n := bridge + len(g.shader.Lanes)
	g.lanes = append(g.lanes[:0], make([]float64, n)...)
	copy(g.lanes[bridge:], g.shader.Lanes)
	s.animator.ShaderLanes(id, g.lanes[bridge:])
	g.lanes[0], g.lanes[1], g.lanes[2] = w, h, ratio
	for i := range g.textureCount {
		gl.Call("activeTexture", gl.Get("TEXTURE0").Int()+i)
		tex, tw, th := textures(i)
		if tex.IsUndefined() {
			// What is not loaded reads as transparent: no texture.
			gl.Call("bindTexture", gl.Get("TEXTURE_2D"), js.Null())
			continue
		}
		gl.Call("bindTexture", gl.Get("TEXTURE_2D"), tex)
		g.lanes[3+2*i], g.lanes[4+2*i] = tw, th
	}
	clear(g.buf)
	shaderir.PutLanes(g.buf, g.offsets, g.types, g.lanes, 4)
	js.CopyBytesToJS(g.bytes, g.buf)

	gl.Call("viewport", 0, 0, pw, ph)
	gl.Call("useProgram", g.prog)
	gl.Call("uniform1f", g.height, ph)
	gl.Call("bindBufferBase", gl.Get("UNIFORM_BUFFER"), 0, g.ubo)
	gl.Call("bufferSubData", gl.Get("UNIFORM_BUFFER"), 0, g.bytes)
	gl.Call("clearColor", 0, 0, 0, 0)
	gl.Call("clear", gl.Get("COLOR_BUFFER_BIT"))
	gl.Call("drawArrays", gl.Get("TRIANGLES"), 0, 3)
	return true
}

// link translates p for WebGL and links it, reporting whether it can draw.
func (g *glShader) link(p *driver.ShaderProgram) bool {
	gl := g.gl
	g.program, g.failed = p, true
	if !g.prog.IsUndefined() {
		gl.Call("deleteProgram", g.prog)
		g.prog = js.Undefined()
	}
	ir := p.IR
	src := "#version 300 es\nprecision highp float;\nprecision highp int;\n\n" +
		glsl.Fragment(ir, "nsgoShader", glsl.Options{FlipY: "true", Block: "NsgoUniforms"}) + glMain
	compile := func(kind, src string) js.Value {
		sh := gl.Call("createShader", gl.Get(kind))
		gl.Call("shaderSource", sh, src)
		gl.Call("compileShader", sh)
		if !gl.Call("getShaderParameter", sh, gl.Get("COMPILE_STATUS")).Truthy() {
			log.Printf("nsgo: shader: WebGL: %s", gl.Call("getShaderInfoLog", sh).String())
			gl.Call("deleteShader", sh)
			return js.Null()
		}
		return sh
	}
	vs, fs := compile("VERTEX_SHADER", glVertex), compile("FRAGMENT_SHADER", src)
	if vs.IsNull() || fs.IsNull() {
		return false
	}
	prog := gl.Call("createProgram")
	gl.Call("attachShader", prog, vs)
	gl.Call("attachShader", prog, fs)
	gl.Call("linkProgram", prog)
	gl.Call("deleteShader", vs)
	gl.Call("deleteShader", fs)
	if !gl.Call("getProgramParameter", prog, gl.Get("LINK_STATUS")).Truthy() {
		log.Printf("nsgo: shader: WebGL: %s", gl.Call("getProgramInfoLog", prog).String())
		gl.Call("deleteProgram", prog)
		return false
	}
	g.prog = prog
	g.height = gl.Call("getUniformLocation", prog, "nsgoHeight")
	g.textureCount = ir.TextureCount
	gl.Call("useProgram", prog)
	for i := range ir.TextureCount {
		gl.Call("uniform1i", gl.Call("getUniformLocation", prog, "T"+strconv.Itoa(i)), i)
	}
	var size int
	g.offsets, size = glsl.UniformLanes(ir)
	g.types = ir.LaneTypes()
	g.buf = make([]byte, size)
	g.bytes = js.Global().Get("Uint8Array").New(size)
	gl.Call("uniformBlockBinding", prog, gl.Call("getUniformBlockIndex", prog, "NsgoUniforms"), 0)
	if g.ubo.IsUndefined() {
		g.ubo = gl.Call("createBuffer")
	}
	gl.Call("bindBuffer", gl.Get("UNIFORM_BUFFER"), g.ubo)
	gl.Call("bufferData", gl.Get("UNIFORM_BUFFER"), size, gl.Get("DYNAMIC_DRAW"))
	g.failed = false
	return true
}

// release gives the view's context back to the browser, which keeps few.
func (g *glShader) release() {
	if g.gl.IsNull() {
		return
	}
	if lose := g.gl.Call("getExtension", "WEBGL_lose_context"); !lose.IsNull() {
		lose.Call("loseContext")
	}
}
