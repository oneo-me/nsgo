//go:build linux && !android

package linux

import (
	"log"
	"strconv"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/shaderir"
	"github.com/nsgo-dev/nsgo/internal/shaderir/glsl"
)

// A shader view hosts a GtkGLArea, which GTK composes with the views
// around it, and draws its program there with OpenGL 3.3 or OpenGL ES
// 3.0, whichever GTK's context has, through libepoxy, which GTK loads:
// when the uniforms, the size, an image or the animated lanes change, as
// the area's render signal asks, never on a timer of its own. Each area
// has its own context, which compiles its program the first time it draws.

// glShaderView is a shader view's area and its GL objects, in the area's
// context.
type glShaderView struct {
	hosted
	area    uintptr
	shader  *driver.Shader
	program *driver.ShaderProgram // what prog was linked from
	failed  bool
	es      bool
	prog    uint32
	ubo     uint32
	vao     uint32
	height  int32 // the location of nsgoHeight
	offsets []int
	types   []shaderir.BasicType
	size    int
	count   int // the images the program reads
	lanes   []float64
	buf     []byte
	// textures are the images' textures, by ImageSource.Key, with their
	// sizes in pixels; pictures what a WGSL program reads instead, an
	// effect's (effect.go).
	textures map[string]glTexture
	pictures [4]*glPicture
}

type glTexture struct {
	id   uint32
	w, h float64
}

// The GL functions shaders call, from libepoxy's dispatch table.
var (
	glOnce               bool
	glOK                 bool
	glCreateShader       func(kind uint32) uint32
	glShaderSource       func(shader uint32, count int32, src **byte, length *int32)
	glCompileShader      func(shader uint32)
	glGetShaderiv        func(shader, name uint32, out *int32)
	glGetShaderInfoLog   func(shader uint32, size int32, length *int32, log *byte)
	glDeleteShader       func(shader uint32)
	glCreateProgram      func() uint32
	glAttachShader       func(prog, shader uint32)
	glLinkProgram        func(prog uint32)
	glGetProgramiv       func(prog, name uint32, out *int32)
	glGetProgramInfoLog  func(prog uint32, size int32, length *int32, log *byte)
	glDeleteProgram      func(prog uint32)
	glUseProgram         func(prog uint32)
	glGetUniformLocation func(prog uint32, name *byte) int32
	glUniform1f          func(loc int32, v float32)
	glUniform1i          func(loc int32, v int32)
	glGetUniformBlockIdx func(prog uint32, name *byte) uint32
	glUniformBlockBind   func(prog, index, binding uint32)
	glGenBuffers         func(n int32, out *uint32)
	glDeleteBuffers      func(n int32, ids *uint32)
	glBindBuffer         func(target, id uint32)
	glBufferData         func(target uint32, size int, data unsafe.Pointer, usage uint32)
	glBufferSubData      func(target uint32, offset, size int, data unsafe.Pointer)
	glBindBufferBase     func(target, index, id uint32)
	glGenVertexArrays    func(n int32, out *uint32)
	glDeleteVertexArrays func(n int32, ids *uint32)
	glBindVertexArray    func(id uint32)
	glViewport           func(x, y, w, h int32)
	glClearColor         func(r, g, b, a float32)
	glClear              func(mask uint32)
	glDrawArrays         func(mode uint32, first, count int32)
	glGenTextures        func(n int32, out *uint32)
	glDeleteTextures     func(n int32, ids *uint32)
	glBindTexture        func(target, id uint32)
	glActiveTexture      func(unit uint32)
	glTexImage2D         func(target uint32, level, internal, w, h, border int32, format, typ uint32, data unsafe.Pointer)
	glTexParameteri      func(target, name uint32, v int32)

	gtkGLAreaNew             func() uintptr
	gtkGLAreaQueueRender     func(uintptr)
	gtkGLAreaMakeCurrent     func(uintptr)
	gtkGLAreaGetError        func(uintptr) uintptr
	gtkGLAreaSetAllowedAPIs  func(uintptr, uint32)
	gdkGLContextGetUseES     func(uintptr) int32
	gtkWidgetGetScaleFactorG func(uintptr) int32
)

// GL's enumerations.
const (
	glFragmentShader = 0x8B30
	glVertexShader   = 0x8B31
	glCompileStatus  = 0x8B81
	glLinkStatus     = 0x8B82
	glUniformBuffer  = 0x8A11
	glDynamicDraw    = 0x88E8
	glColorBuffer    = 0x4000
	glTriangles      = 4
	glTexture2D      = 0x0DE1
	glTexture0       = 0x84C0
	glRGBA           = 0x1908
	glUnsignedByte   = 0x1401
	glMinFilter      = 0x2801
	glMagFilter      = 0x2800
	glNearest        = 0x2600
	glWrapS          = 0x2802
	glWrapT          = 0x2803
	glClampToEdge    = 0x812F
	glInvalidIndex   = 0xFFFFFFFF
)

// loadGL binds the GL functions through libepoxy and GTK's GL area,
// once; it reports whether they are there.
func loadGL() bool {
	if glOnce {
		return glOK
	}
	glOnce = true
	epoxy, err := purego.Dlopen("libepoxy.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		log.Printf("nsgo: shader: %v", err)
		return false
	}
	ok := true
	// Each of libepoxy's functions is a pointer variable it resolves on
	// the first call.
	fn := func(out any, name string) {
		addr, err := purego.Dlsym(epoxy, "epoxy_"+name)
		if err != nil {
			ok = false
			return
		}
		purego.RegisterFunc(out, *(*uintptr)(cPointer(addr)))
	}
	fn(&glCreateShader, "glCreateShader")
	fn(&glShaderSource, "glShaderSource")
	fn(&glCompileShader, "glCompileShader")
	fn(&glGetShaderiv, "glGetShaderiv")
	fn(&glGetShaderInfoLog, "glGetShaderInfoLog")
	fn(&glDeleteShader, "glDeleteShader")
	fn(&glCreateProgram, "glCreateProgram")
	fn(&glAttachShader, "glAttachShader")
	fn(&glLinkProgram, "glLinkProgram")
	fn(&glGetProgramiv, "glGetProgramiv")
	fn(&glGetProgramInfoLog, "glGetProgramInfoLog")
	fn(&glDeleteProgram, "glDeleteProgram")
	fn(&glUseProgram, "glUseProgram")
	fn(&glGetUniformLocation, "glGetUniformLocation")
	fn(&glUniform1f, "glUniform1f")
	fn(&glUniform1i, "glUniform1i")
	fn(&glGetUniformBlockIdx, "glGetUniformBlockIndex")
	fn(&glUniformBlockBind, "glUniformBlockBinding")
	fn(&glGenBuffers, "glGenBuffers")
	fn(&glDeleteBuffers, "glDeleteBuffers")
	fn(&glBindBuffer, "glBindBuffer")
	fn(&glBufferData, "glBufferData")
	fn(&glBufferSubData, "glBufferSubData")
	fn(&glBindBufferBase, "glBindBufferBase")
	fn(&glGenVertexArrays, "glGenVertexArrays")
	fn(&glDeleteVertexArrays, "glDeleteVertexArrays")
	fn(&glBindVertexArray, "glBindVertexArray")
	fn(&glViewport, "glViewport")
	fn(&glClearColor, "glClearColor")
	fn(&glClear, "glClear")
	fn(&glDrawArrays, "glDrawArrays")
	fn(&glGenTextures, "glGenTextures")
	fn(&glDeleteTextures, "glDeleteTextures")
	fn(&glBindTexture, "glBindTexture")
	fn(&glActiveTexture, "glActiveTexture")
	fn(&glTexImage2D, "glTexImage2D")
	fn(&glTexParameteri, "glTexParameteri")
	if !ok {
		log.Printf("nsgo: shader: libepoxy lacks GL functions")
		return false
	}
	purego.RegisterLibFunc(&gtkGLAreaNew, gtkLib, "gtk_gl_area_new")
	purego.RegisterLibFunc(&gtkGLAreaQueueRender, gtkLib, "gtk_gl_area_queue_render")
	purego.RegisterLibFunc(&gtkGLAreaMakeCurrent, gtkLib, "gtk_gl_area_make_current")
	purego.RegisterLibFunc(&gtkGLAreaGetError, gtkLib, "gtk_gl_area_get_error")
	if _, err := purego.Dlsym(gtkLib, "gtk_gl_area_set_allowed_apis"); err == nil {
		purego.RegisterLibFunc(&gtkGLAreaSetAllowedAPIs, gtkLib, "gtk_gl_area_set_allowed_apis")
	}
	purego.RegisterLibFunc(&gdkGLContextGetUseES, gtkLib, "gdk_gl_context_get_use_es")
	purego.RegisterLibFunc(&gtkWidgetGetScaleFactorG, gtkLib, "gtk_widget_get_scale_factor")
	glOK = true
	return true
}

// glVertexSource is a triangle covering the area.
const glVertexSource = `
void main() {
	vec2 uv = vec2(float((gl_VertexID << 1) & 2), float(gl_VertexID & 2));
	gl_Position = vec4(uv * 2.0 - 1.0, 0.0, 1.0);
}
`

// glMainSource is the fragment stage around the program's function: the
// position counts down from the top, as the program's does.
const glMainSource = `
uniform float nsgoHeight;
out vec4 nsgoColor;

void main() {
	vec2 p = vec2(gl_FragCoord.x, nsgoHeight - gl_FragCoord.y);
	nsgoColor = nsgoShader(vec4(p, gl_FragCoord.z, 1.0), p / U1);
}
`

// setShader gives v what it draws, making its area the first time, and
// binds its animated lanes.
func (w *window) setShader(v *view, sh *driver.Shader) {
	if v.shader == nil {
		if !loadGL() {
			return
		}
		g := &glShaderView{area: gtkGLAreaNew()}
		v.shader = g
		if gtkGLAreaSetAllowedAPIs != nil {
			const both = 1 | 2 // GDK_GL_API_GL | GDK_GL_API_GLES
			gtkGLAreaSetAllowedAPIs(g.area, both)
		}
		// Presses and hovers go to the view, not the area.
		w.host(v, &g.hosted, g.area, false)
		w.connect(v, g.area, "render", cbShaderRender)
		w.connect(v, g.area, "unrealize", cbShaderUnrealize)
	}
	v.shader.shader = sh
	w.animator.BindShader(v.id, sh.Nodes)
	gtkGLAreaQueueRender(v.shader.area)
}

// changedShaders draws again the shader views whose animated lanes
// changed.
func (w *window) changedShaders() {
	w.animator.ChangedShaders(func(id driver.NodeID) {
		if v := w.view(id); v != nil && v.fx != nil && v.fx.gl != nil {
			gtkGLAreaQueueRender(v.fx.gl.area)
		} else if v != nil && v.shader != nil {
			gtkGLAreaQueueRender(v.shader.area)
		}
	})
}

func shaderRenderCallback(area, context, data uintptr) uintptr {
	w, v := nativeOf(data, area)
	if v == nil {
		return 0
	}
	switch {
	case v.fx != nil && v.fx.gl != nil && v.fx.gl.area == area:
		w.drawGL(v.fx.gl, v.id, context)
	case v.shader != nil:
		w.drawShader(v, context)
	default:
		return 0
	}
	return 1
}

// shaderUnrealizeCallback deletes the area's GL objects with its context.
func shaderUnrealizeCallback(area, data uintptr) {
	_, v := nativeOf(data, area)
	if v == nil {
		return
	}
	gtkGLAreaMakeCurrent(area)
	if v.fx != nil && v.fx.gl != nil && v.fx.gl.area == area {
		v.fx.gl.deleteGL()
	} else if v.shader != nil {
		v.shader.deleteGL()
	}
}

func (g *glShaderView) deleteGL() {
	if gtkGLAreaGetError(g.area) != 0 {
		return
	}
	if g.prog != 0 {
		glDeleteProgram(g.prog)
	}
	if g.ubo != 0 {
		glDeleteBuffers(1, &g.ubo)
	}
	if g.vao != 0 {
		glDeleteVertexArrays(1, &g.vao)
	}
	for _, t := range g.textures {
		glDeleteTextures(1, &t.id)
	}
	for _, pic := range g.pictures {
		if pic != nil && pic.tex != 0 {
			glDeleteTextures(1, &pic.tex)
			pic.tex = 0
		}
	}
	g.prog, g.ubo, g.vao, g.program, g.textures = 0, 0, 0, nil, nil
}

// drawShader draws v's program in its area's context, made current.
func (w *window) drawShader(v *view, context uintptr) { w.drawGL(v.shader, v.id, context) }

// drawGL draws g's program in its area's context, made current, with
// view id's animated lanes.
func (w *window) drawGL(g *glShaderView, id driver.NodeID, context uintptr) {
	if g.shader == nil || gtkGLAreaGetError(g.area) != 0 {
		return
	}
	g.es = gdkGLContextGetUseES(context) != 0
	if g.program != g.shader.Program && !g.link(g.shader.Program) || g.failed {
		glClearColor(0, 0, 0, 0)
		glClear(glColorBuffer)
		return
	}
	scale := float64(max(gtkWidgetGetScaleFactorG(g.area), 1))
	width, height := float64(gtkWidgetGetWidth(g.area)), float64(gtkWidgetGetHeight(g.area))
	pw, ph := int32(width*scale), int32(height*scale)
	bridge := 3 + 2*g.count
	g.lanes = append(g.lanes[:0], make([]float64, bridge+len(g.shader.Lanes))...)
	copy(g.lanes[bridge:], g.shader.Lanes)
	w.animator.ShaderLanes(id, g.lanes[bridge:])
	g.lanes[0], g.lanes[1], g.lanes[2] = width, height, scale
	for i := range g.count {
		glActiveTexture(glTexture0 + uint32(i))
		var t glTexture
		switch {
		case g.shader.Program.WGSL:
			if pic := g.pictures[i]; pic != nil {
				t = pic.texture()
			}
		case i < len(g.shader.Images):
			t = w.shaderTexture(w.view(id), g.shader.Images[i])
		}
		glBindTexture(glTexture2D, t.id)
		g.lanes[3+2*i], g.lanes[4+2*i] = t.w, t.h
	}
	clear(g.buf)
	shaderir.PutLanes(g.buf, g.offsets, g.types, g.lanes, 4)

	glViewport(0, 0, pw, ph)
	glUseProgram(g.prog)
	glUniform1f(g.height, float32(ph))
	glBindBuffer(glUniformBuffer, g.ubo)
	if len(g.buf) > 0 {
		glBufferSubData(glUniformBuffer, 0, len(g.buf), unsafe.Pointer(&g.buf[0]))
	}
	glBindBufferBase(glUniformBuffer, 0, g.ubo)
	glClearColor(0, 0, 0, 0)
	glClear(glColorBuffer)
	glBindVertexArray(g.vao)
	glDrawArrays(glTriangles, 0, 3)
}

// shaderTexture returns image src's texture in v's context, loading the
// image the first time; a zero texture, which reads transparent, until
// it loads and the area draws again.
func (w *window) shaderTexture(v *view, src driver.ImageSource) glTexture {
	g := v.shader
	key := src.Key()
	if t, ok := g.textures[key]; ok {
		return t
	}
	s := w.canvasImage(src, v)
	if s == nil || s.pic == nil {
		return glTexture{}
	}
	pic := s.pic
	// Cairo's premultiplied BGRA, as GLES's RGBA wants it.
	rgba := make([]byte, len(pic.pix))
	for i := 0; i+3 < len(pic.pix); i += 4 {
		rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = pic.pix[i+2], pic.pix[i+1], pic.pix[i], pic.pix[i+3]
	}
	t := glTexture{w: float64(pic.width), h: float64(pic.height)}
	glGenTextures(1, &t.id)
	glBindTexture(glTexture2D, t.id)
	glTexImage2D(glTexture2D, 0, glRGBA, int32(pic.width), int32(pic.height), 0, glRGBA, glUnsignedByte,
		unsafe.Pointer(&rgba[0]))
	glTexParameteri(glTexture2D, glMinFilter, glNearest)
	glTexParameteri(glTexture2D, glMagFilter, glNearest)
	glTexParameteri(glTexture2D, glWrapS, glClampToEdge)
	glTexParameteri(glTexture2D, glWrapT, glClampToEdge)
	if g.textures == nil {
		g.textures = map[string]glTexture{}
	}
	g.textures[key] = t
	return t
}

// link translates p for the area's GL and links it, reporting whether it
// can draw.
func (g *glShaderView) link(p *driver.ShaderProgram) bool {
	g.deleteGL()
	g.program, g.failed = p, true
	ir := p.IR
	head := "#version 330 core\n"
	if g.es {
		head = "#version 300 es\nprecision highp float;\nprecision highp int;\n"
	}
	src := head + "\n" + glsl.Fragment(ir, "nsgoShader", glsl.Options{FlipY: "true", Block: "NsgoUniforms"}) +
		glMainSource
	compile := func(kind uint32, src string) uint32 {
		sh := glCreateShader(kind)
		cs := cstr(src)
		glShaderSource(sh, 1, &cs, nil)
		glCompileShader(sh)
		var ok int32
		glGetShaderiv(sh, glCompileStatus, &ok)
		if ok == 0 {
			buf := make([]byte, 4096)
			var n int32
			glGetShaderInfoLog(sh, int32(len(buf)), &n, &buf[0])
			log.Printf("nsgo: shader: GL: %s", buf[:max(n, 0)])
			glDeleteShader(sh)
			return 0
		}
		return sh
	}
	vs, fs := compile(glVertexShader, head+glVertexSource), compile(glFragmentShader, src)
	if vs == 0 || fs == 0 {
		return false
	}
	prog := glCreateProgram()
	glAttachShader(prog, vs)
	glAttachShader(prog, fs)
	glLinkProgram(prog)
	glDeleteShader(vs)
	glDeleteShader(fs)
	var ok int32
	glGetProgramiv(prog, glLinkStatus, &ok)
	if ok == 0 {
		buf := make([]byte, 4096)
		var n int32
		glGetProgramInfoLog(prog, int32(len(buf)), &n, &buf[0])
		log.Printf("nsgo: shader: GL: %s", buf[:max(n, 0)])
		glDeleteProgram(prog)
		return false
	}
	g.prog = prog
	g.height = glGetUniformLocation(prog, cstr("nsgoHeight"))
	g.offsets, g.size = glsl.UniformLanes(ir)
	g.types = ir.LaneTypes()
	g.count = ir.TextureCount
	g.buf = make([]byte, g.size)
	glUseProgram(prog)
	for i := range g.count {
		glUniform1i(glGetUniformLocation(prog, cstr("T"+strconv.Itoa(i))), int32(i))
	}
	if idx := glGetUniformBlockIdx(prog, cstr("NsgoUniforms")); idx != glInvalidIndex {
		glUniformBlockBind(prog, idx, 0)
	}
	glGenBuffers(1, &g.ubo)
	glBindBuffer(glUniformBuffer, g.ubo)
	glBufferData(glUniformBuffer, max(g.size, 16), nil, glDynamicDraw)
	glGenVertexArrays(1, &g.vao)
	g.failed = false
	return true
}
