//go:build linux && !android

package linux

import (
	"math"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Effects, transitions and navigation shaders (driver.Shader.Role) draw
// their host's children through a GtkGLArea of their own over them, the
// box and the overflow around it: as the host draws, its children's
// render nodes go into a node of their own instead of its snapshot, which
// the window's renderer renders into a texture of the box and the
// overflow, downloaded and handed to the area. The children still take
// input, as the window hit-tests the views. A transition's first picture
// is taken as it starts, before its batch is applied. A view's own effect
// hosts on it, a screen shader's on the root view; the host draws again
// whenever a frame moved anything or a batch changed what it pictures.
// The backdrop is not pictured: it reads as transparent.

// effect is a view's effect, transition or navigation shader.
type effect struct {
	shader *driver.Shader
	gl     *glShaderView // the area over the children
	host   *view
	signal []uint64
	// pics are content, backdrop, from and to; before the picture a
	// transition's start took, which the Shader that follows places.
	pics      [4]glPicture
	before    glPicture
	hasBefore bool
	// stale says the children changed since they were pictured.
	stale bool
}

// glPicture is a picture's pixels, RGBA premultiplied, and their texture
// in the area's context once uploaded.
type glPicture struct {
	pix   []byte
	w, h  int
	fresh bool
	tex   uint32
}

// texture uploads the picture if it changed and returns its texture.
func (p *glPicture) texture() glTexture {
	if p.w == 0 || p.h == 0 {
		return glTexture{}
	}
	if p.tex == 0 {
		glGenTextures(1, &p.tex)
		glBindTexture(glTexture2D, p.tex)
		glTexParameteri(glTexture2D, glMinFilter, glNearest)
		glTexParameteri(glTexture2D, glMagFilter, glNearest)
		glTexParameteri(glTexture2D, glWrapS, glClampToEdge)
		glTexParameteri(glTexture2D, glWrapT, glClampToEdge)
		p.fresh = true
	}
	glBindTexture(glTexture2D, p.tex)
	if p.fresh {
		p.fresh = false
		glTexImage2D(glTexture2D, 0, glRGBA, int32(p.w), int32(p.h), 0, glRGBA, glUnsignedByte, unsafe.Pointer(&p.pix[0]))
	}
	return glTexture{id: p.tex, w: float64(p.w), h: float64(p.h)}
}

var (
	gtkNativeGetRenderer     func(uintptr) uintptr
	gskRendererRenderTexture func(renderer, node uintptr, viewport *grapheneRect) uintptr
	gdkTextureDownload       func(texture uintptr, data unsafe.Pointer, stride uintptr)
	gdkTextureGetWidth       func(uintptr) int32
	gdkTextureGetHeight      func(uintptr) int32
	gtkSnapshotScale         func(uintptr, float32, float32)
	effectsBound             bool
)

func bindEffects() {
	if effectsBound {
		return
	}
	effectsBound = true
	purego.RegisterLibFunc(&gtkNativeGetRenderer, gtkLib, "gtk_native_get_renderer")
	purego.RegisterLibFunc(&gskRendererRenderTexture, gtkLib, "gsk_renderer_render_texture")
	purego.RegisterLibFunc(&gdkTextureDownload, gtkLib, "gdk_texture_download")
	purego.RegisterLibFunc(&gdkTextureGetWidth, gtkLib, "gdk_texture_get_width")
	purego.RegisterLibFunc(&gdkTextureGetHeight, gtkLib, "gdk_texture_get_height")
	purego.RegisterLibFunc(&gtkSnapshotScale, gtkLib, "gtk_snapshot_scale")
}

// setEffect gives v its effect, transition or navigation shader, or takes
// it away for nil.
func (w *window) setEffect(v *view, sh *driver.Shader) {
	if sh == nil || sh.Program == nil {
		w.endEffect(v)
		return
	}
	fx := v.fx
	if fx == nil {
		fx = &effect{}
		v.fx = fx
	}
	if fx.gl == nil {
		if !loadGL() {
			return
		}
		bindEffects()
		g := &glShaderView{area: gtkGLAreaNew()}
		if gtkGLAreaSetAllowedAPIs != nil {
			const both = 1 | 2 // GDK_GL_API_GL | GDK_GL_API_GLES
			gtkGLAreaSetAllowedAPIs(g.area, both)
		}
		gObjectRefSink(g.area)
		gtkWidgetSetCanTarget(g.area, 0)
		w.natives[g.area] = v
		fx.signal = append(fx.signal, gSignalConnect(g.area, "render", cbShaderRender, w.handle),
			gSignalConnect(g.area, "unrealize", cbShaderUnrealize, w.handle))
		fx.gl = g
	}
	was := fx.shader
	fx.shader = sh
	fx.gl.shader = sh
	w.animator.BindShader(v.id, sh.Nodes)
	if sh.Role.Transitions() {
		switch {
		case sh.Running && fx.hasBefore:
			side := 2
			if sh.Backwards {
				side = 3
			}
			fx.pics[side].pix, fx.pics[side].w, fx.pics[side].h = fx.before.pix, fx.before.w, fx.before.h
			fx.pics[side].fresh = true
			fx.hasBefore = false
		case !sh.Running && was != nil && was.Running:
			fx.pics[2].w, fx.pics[3].w = 0, 0
		}
	}
	if w.effects == nil {
		w.effects = map[*view]bool{}
	}
	w.effects[v] = true
	w.attachEffect(v)
	w.redrawEffect(v)
}

// effectHost is the view v's shader draws over: v, or the root view for
// a screen shader.
func (w *window) effectHost(v *view) *view {
	if v.fx != nil && v.fx.shader != nil && v.fx.shader.Role.Covers() {
		return w.view(w.root)
	}
	return v
}

// attachEffect puts v's area in its host, as the host changes.
func (w *window) attachEffect(v *view) {
	fx := v.fx
	host := w.effectHost(v)
	if host == nil || host.widget == 0 || fx.gl == nil || fx.host == host {
		return
	}
	if fx.host != nil && gtkWidgetGetParent(fx.gl.area) != 0 {
		gtkWidgetUnparent(fx.gl.area)
	}
	gtkWidgetSetParent(fx.gl.area, host.widget)
	fx.host = host
	host.fxArea = fx.gl.area
	gtkWidgetQueueAllocate(host.widget)
}

func (fx *effect) shows() bool {
	return fx.shader != nil && fx.gl != nil && (!fx.shader.Role.Transitions() || fx.shader.Running)
}

// effectOn returns the effect over host's children that shows, if any.
func (w *window) effectOn(host *view) *view {
	for v := range w.effects {
		if v.fx != nil && v.fx.host == host && v.fx.shows() {
			return v
		}
	}
	return nil
}

// redrawEffect draws v's effect again: its host's snapshot pictures the
// children and the area draws.
func (w *window) redrawEffect(v *view) {
	if fx := v.fx; fx != nil {
		fx.stale = true
	}
	if fx := v.fx; fx != nil && fx.host != nil {
		gtkWidgetQueueDraw(fx.host.widget)
		if fx.gl != nil {
			gtkGLAreaQueueRender(fx.gl.area)
		}
	}
}

// staleEffects draws every effect again, after a frame that moved views.
func (w *window) staleEffects() {
	for v := range w.effects {
		w.redrawEffect(v)
	}
}

// startTransition pictures what v's transition starts from, before the
// batch that starts it is applied.
func (w *window) startTransition(v *view) {
	if v.fx == nil {
		v.fx = &effect{}
	}
	fx := v.fx
	host := w.effectHost(v)
	if host == nil || host.widget == 0 {
		return
	}
	bindEffects()
	ov := 0.0
	if fx.shader != nil {
		ov = fx.shader.Overflow
	}
	fx.hasBefore = w.picture(&fx.before, host, ov, 0)
}

// picture renders host's children into pic, the box and ov points around
// it, the child widget skip (an effect's area) left out.
func (w *window) picture(pic *glPicture, host *view, ov float64, skip uintptr) bool {
	node, pw, ph := w.childrenNode(host, ov, skip)
	if node == 0 {
		return pw > 0
	}
	defer gskRenderNodeUnref(node)
	return w.render(pic, host, node, pw, ph)
}

// childrenNode snapshots host's children, scaled to pixels, into a node
// pw by ph pixels; 0 where there is nothing to draw.
func (w *window) childrenNode(host *view, ov float64, skip uintptr) (node uintptr, pw, ph int) {
	scale := float64(max(gtkWidgetGetScaleFactorG(host.widget), 1))
	size := host.shownFrame().Size
	pw = min(int(math.Ceil((float64(size.Width)+2*ov)*scale)), 4096)
	ph = min(int(math.Ceil((float64(size.Height)+2*ov)*scale)), 4096)
	if pw <= 0 || ph <= 0 {
		return 0, 0, 0
	}
	snap := gtkSnapshotNew()
	gtkSnapshotScale(snap, float32(scale), float32(scale))
	gtkSnapshotTranslate(snap, &graphenePoint{float32(ov), float32(ov)})
	for c := gtkWidgetGetFirstChild(host.widget); c != 0; c = gtkWidgetGetNextSibling(c) {
		if c != skip {
			gtkWidgetSnapshotChild(host.widget, c, snap)
		}
	}
	return gtkSnapshotFreeToNode(snap), pw, ph
}

// render renders node with host's window's renderer into pic.
func (w *window) render(pic *glPicture, host *view, node uintptr, pw, ph int) bool {
	native := gtkWidgetGetNative(host.widget)
	if native == 0 {
		return false
	}
	renderer := gtkNativeGetRenderer(native)
	if renderer == 0 {
		return false
	}
	tex := gskRendererRenderTexture(renderer, node, &grapheneRect{0, 0, float32(pw), float32(ph)})
	if tex == 0 {
		return false
	}
	defer gObjectUnref(tex)
	tw, th := int(gdkTextureGetWidth(tex)), int(gdkTextureGetHeight(tex))
	if cap(pic.pix) < tw*th*4 {
		pic.pix = make([]byte, tw*th*4)
	}
	pic.pix = pic.pix[:tw*th*4]
	gdkTextureDownload(tex, unsafe.Pointer(&pic.pix[0]), uintptr(tw*4))
	// Cairo's premultiplied BGRA, as GL's RGBA wants it.
	for i := 0; i+3 < len(pic.pix); i += 4 {
		pic.pix[i], pic.pix[i+2] = pic.pix[i+2], pic.pix[i]
	}
	pic.w, pic.h, pic.fresh = tw, th, true
	return true
}

// snapshotEffect pictures host's children for effect e in place of
// drawing them, and draws e's area over them; it reports false where e
// cannot draw, and the children are drawn as they are.
func (w *window) snapshotEffect(e, host *view, snap uintptr) bool {
	fx := e.fx
	sh := fx.shader
	if gtkGLAreaGetError(fx.gl.area) != 0 {
		return false
	}
	live := -1
	switch {
	case sh.Role.Transitions():
		live = 3
		if sh.Backwards {
			live = 2
		}
		if sh.Pictures&(1<<live) == 0 {
			live = -1
		}
	case sh.Pictures&driver.PictureContent != 0:
		live = 0
	}
	if live >= 0 && (fx.stale || sh.Role.Transitions()) {
		fx.stale = false
		// The children's nodes are taken as GTK draws them, and rendered
		// after the frame: the renderer is GTK's own during it.
		node, pw, ph := w.childrenNode(host, sh.Overflow, fx.gl.area)
		if node != 0 {
			pic := &fx.pics[live]
			w.post(func() {
				defer gskRenderNodeUnref(node)
				if e.fx == fx && fx.host == host && !host.deleted {
					w.render(pic, host, node, pw, ph)
					gtkGLAreaQueueRender(fx.gl.area)
				}
			})
		}
	}
	fx.gl.pictures = [4]*glPicture{&fx.pics[0], &fx.pics[1], &fx.pics[2], &fx.pics[3]}
	gtkGLAreaQueueRender(fx.gl.area)
	gtkWidgetSnapshotChild(host.widget, fx.gl.area, snap)
	return true
}

// effectArea is where the area of the effect over host's children goes:
// its box and the overflow around it.
func (w *window) effectArea(host *view) driver.Bounds {
	ov := driver.Points(0)
	for v := range w.effects {
		if v.fx != nil && v.fx.host == host && v.fx.shader != nil {
			ov = driver.Points(v.fx.shader.Overflow)
		}
	}
	size := host.shownFrame().Size
	return driver.Bounds{Origin: driver.Point{X: -ov, Y: -ov}, Size: driver.Size{Width: size.Width + 2*ov, Height: size.Height + 2*ov}}
}

// navigating reports whether a navigation shader runs over host.
func (w *window) navigating(host *view) bool {
	return host != nil && host.fx != nil && host.fx.shader != nil && host.fx.shader.Role == driver.ShaderNav &&
		host.fx.shader.Running && host.fx.gl != nil
}

func (w *window) endEffect(v *view) {
	fx := v.fx
	if fx == nil {
		return
	}
	v.fx = nil
	delete(w.effects, v)
	if fx.gl != nil {
		for _, id := range fx.signal {
			gSignalHandlerDisconnect(fx.gl.area, id)
		}
		delete(w.natives, fx.gl.area)
		if fx.host != nil {
			if fx.host.fxArea == fx.gl.area {
				fx.host.fxArea = 0
			}
			gtkWidgetQueueDraw(fx.host.widget)
		}
		if gtkWidgetGetParent(fx.gl.area) != 0 {
			gtkWidgetUnparent(fx.gl.area)
		}
		gObjectUnref(fx.gl.area)
	}
}
