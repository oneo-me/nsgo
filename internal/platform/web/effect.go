//go:build js && wasm

package web

import (
	"math"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/webstyle"
)

// Effects, transitions and navigation shaders (driver.Shader.Role) draw
// over their host's children with WebGL, from pictures painted from the
// DOM: a <canvas> over the children (nsgo-fx-out) draws the shader, and
// the host's class nsgo-fx puts its children at opacity 0, so they keep
// the pointer and the focus under it. A view's own effect hosts on it, a
// screen shader's on the root view.
//
// paintTree paints an element's children into a 2D canvas in points:
// backgrounds, rounded corners, uniform borders, images and canvases,
// text runs, clipping and opacity; no transforms, gradients or shadows,
// and images from other origins without CORS are left out so the picture
// stays readable. Pictures are painted again in the animation frame after
// a batch says they changed (ItemShaderInput) or the page scrolled in
// them, at most once a frame; a running transition's live side every
// frame. Where WebGL 2 is missing the children show as they are.

const effectCSS = `.nsgo-fx > :not(.nsgo-fx-out) { opacity: 0 !important; }
.nsgo-fx-out { position: absolute; pointer-events: none; z-index: 2147483647; }`

// effect is a view's effect, transition or navigation shader.
type effect struct {
	shader *driver.Shader
	gl     *glShader
	canvas js.Value // the canvas the shader draws on
	host   *view
	// pics are content, backdrop, from and to; before the picture an
	// ItemShaderTransition took, which the Shader that follows places.
	pics      [4]fxPicture
	before    fxPicture
	hasBefore bool
	stale     bool
	shown     bool
}

// fxPicture is a 2D canvas painted from the DOM and its texture in the
// effect's context.
type fxPicture struct {
	canvas js.Value
	tex    js.Value
	w, h   float64 // pixels
	fresh  bool    // painted since uploaded
}

func (s *surface) setEffect(v *view, sh *driver.Shader) {
	if sh == nil || sh.Program == nil {
		s.endEffect(v)
		return
	}
	if !s.effectStyled {
		s.effectStyled = true
		el := s.doc.Call("createElement", "style")
		el.Set("textContent", effectCSS)
		s.doc.Get("head").Call("appendChild", el)
	}
	fx := v.fx
	if fx == nil {
		fx = &effect{}
		v.fx = fx
	}
	if fx.gl == nil {
		fx.canvas = s.doc.Call("createElement", "canvas")
		fx.canvas.Set("className", "nsgo-fx-out")
		opts := map[string]any{"alpha": true, "premultipliedAlpha": true, "antialias": false, "depth": false, "stencil": false}
		fx.gl = &glShader{gl: fx.canvas.Call("getContext", "webgl2", opts)}
	}
	was := fx.shader
	fx.shader = sh
	fx.gl.shader = sh
	s.animator.BindShader(v.id, sh.Nodes)
	if sh.Role.Transitions() {
		switch {
		case sh.Running && fx.hasBefore:
			side := 2
			if sh.Backwards {
				side = 3
			}
			fx.pics[side], fx.before = fx.before, fx.pics[side]
			fx.pics[side].fresh = true
			fx.hasBefore = false
		case !sh.Running && was != nil && was.Running:
			fx.pics[2].release(fx.gl)
			fx.pics[3].release(fx.gl)
		}
	}
	fx.stale = true
	s.effects[v] = true
	s.startTicking()
}

// startTransition paints what v's transition starts from, before the
// batch that starts it is applied.
func (s *surface) startTransition(v *view) {
	if v.fx == nil {
		v.fx = &effect{}
	}
	fx := v.fx
	host := s.effectHost(v)
	if host == nil {
		return
	}
	ov := 0.0
	if fx.shader != nil {
		ov = fx.shader.Overflow
	}
	fx.hasBefore = s.paintPicture(&fx.before, host, ov, false)
}

func (s *surface) effectHost(v *view) *view {
	if v.fx != nil && v.fx.shader != nil && v.fx.shader.Role.Covers() {
		return s.root
	}
	return v
}

// paintPicture paints pic of what host's children show, the box and ov points
// around it; the backdrop is the root painted up to host.
func (s *surface) paintPicture(pic *fxPicture, host *view, ov float64, backdrop bool) bool {
	if host == nil || host.size.Width <= 0 || host.size.Height <= 0 {
		return false
	}
	ratio := float64(s.env.PixelRatio)
	if ratio <= 0 {
		ratio = 1
	}
	w, h := float64(host.size.Width)+2*ov, float64(host.size.Height)+2*ov
	pw, ph := math.Min(math.Ceil(w*ratio), 4096), math.Min(math.Ceil(h*ratio), 4096)
	if pic.canvas.IsUndefined() {
		pic.canvas = s.doc.Call("createElement", "canvas")
	}
	if pic.w != pw || pic.h != ph {
		pic.canvas.Set("width", pw)
		pic.canvas.Set("height", ph)
		pic.w, pic.h = pw, ph
	}
	ctx := pic.canvas.Call("getContext", "2d")
	ctx.Call("setTransform", 1, 0, 0, 1, 0, 0)
	ctx.Call("clearRect", 0, 0, pw, ph)
	ctx.Call("setTransform", pw/w, 0, 0, ph/h, 0, 0)
	r := host.el.Call("getBoundingClientRect")
	x0, y0 := r.Get("left").Float()-ov, r.Get("top").Float()-ov
	list := host.el.Get("classList")
	hidden := list.Call("contains", "nsgo-fx").Bool()
	if hidden {
		list.Call("remove", "nsgo-fx")
	}
	p := painter{ctx: ctx, x0: x0, y0: y0}
	if backdrop {
		p.stop = host.el
		p.element(s.root.el, 1)
	} else {
		for c := host.content.Get("firstChild"); !c.IsNull(); c = c.Get("nextSibling") {
			p.node(c, 1)
		}
	}
	if hidden {
		list.Call("add", "nsgo-fx")
	}
	pic.fresh = true
	return true
}

// painter paints elements into a 2D context, the page's point (x0, y0) at
// its origin; stop ends the painting where it comes, as a backdrop ends at
// its view.
type painter struct {
	ctx     js.Value
	x0, y0  float64
	stop    js.Value
	stopped bool
}

func (p *painter) node(n js.Value, alpha float64) {
	switch n.Get("nodeType").Int() {
	case 1:
		p.element(n, alpha)
	case 3:
		p.text(n, alpha)
	}
}

func (p *painter) element(el js.Value, alpha float64) {
	if p.stopped {
		return
	}
	if !p.stop.IsUndefined() && el.Equal(p.stop) {
		p.stopped = true
		return
	}
	if el.Get("classList").Call("contains", "nsgo-fx-out").Bool() {
		return
	}
	cs := js.Global().Call("getComputedStyle", el)
	if cs.Get("display").String() == "none" || cs.Get("visibility").String() == "hidden" {
		return
	}
	alpha *= parseNumber(cs.Get("opacity").String(), 1)
	if alpha <= 0 {
		return
	}
	r := el.Call("getBoundingClientRect")
	x, y := r.Get("left").Float()-p.x0, r.Get("top").Float()-p.y0
	w, h := r.Get("width").Float(), r.Get("height").Float()
	radius := parseNumber(cs.Get("borderTopLeftRadius").String(), 0)
	ctx := p.ctx
	ctx.Call("save")
	ctx.Set("globalAlpha", alpha)
	if bg := cs.Get("backgroundColor").String(); visibleColor(bg) {
		ctx.Set("fillStyle", bg)
		p.box(x, y, w, h, radius)
		ctx.Call("fill")
	}
	if bw := parseNumber(cs.Get("borderTopWidth").String(), 0); bw > 0 {
		if bc := cs.Get("borderTopColor").String(); visibleColor(bc) {
			ctx.Set("strokeStyle", bc)
			ctx.Set("lineWidth", bw)
			p.box(x+bw/2, y+bw/2, w-bw, h-bw, math.Max(radius-bw/2, 0))
			ctx.Call("stroke")
		}
	}
	switch el.Get("tagName").String() {
	case "IMG":
		if el.Get("complete").Bool() && el.Get("naturalWidth").Int() > 0 && sameOrigin(el) {
			ctx.Call("drawImage", el, x, y, w, h)
		}
	case "CANVAS":
		if el.Get("width").Int() > 0 && el.Get("height").Int() > 0 {
			ctx.Call("drawImage", el, x, y, w, h)
		}
	case "INPUT", "TEXTAREA":
		p.fieldText(el, cs, x, y, w, h)
	}
	if o := cs.Get("overflow").String(); o != "visible" && o != "" {
		p.box(x, y, w, h, radius)
		ctx.Call("clip")
	}
	for c := el.Get("firstChild"); !c.IsNull() && !p.stopped; c = c.Get("nextSibling") {
		p.node(c, alpha)
	}
	ctx.Call("restore")
}

// box makes a rounded rectangle the context's path.
func (p *painter) box(x, y, w, h, r float64) {
	ctx := p.ctx
	ctx.Call("beginPath")
	r = math.Min(r, math.Min(w, h)/2)
	if r <= 0 {
		ctx.Call("rect", x, y, w, h)
		return
	}
	ctx.Call("roundRect", x, y, w, h, r)
}

// text paints a text node line by line, as the page broke it.
func (p *painter) text(n js.Value, alpha float64) {
	s := n.Get("data").String()
	if strings.TrimSpace(s) == "" {
		return
	}
	parent := n.Get("parentElement")
	if parent.IsNull() {
		return
	}
	cs := js.Global().Call("getComputedStyle", parent)
	ctx := p.ctx
	ctx.Call("save")
	ctx.Set("globalAlpha", alpha)
	ctx.Set("font", cs.Get("font").String())
	ctx.Set("fillStyle", cs.Get("color").String())
	ctx.Set("textBaseline", "middle")
	rng := js.Global().Get("document").Call("createRange")
	rng.Call("selectNodeContents", n)
	rects := rng.Call("getClientRects")
	if rects.Length() <= 1 {
		if rects.Length() == 1 {
			r := rects.Index(0)
			ctx.Call("fillText", strings.TrimRight(s, "\n"), r.Get("left").Float()-p.x0, r.Get("top").Float()-p.y0+r.Get("height").Float()/2)
		}
		ctx.Call("restore")
		return
	}
	// Several lines: each run of UTF-16 units on one line goes alone.
	units := n.Get("length").Int()
	start, top, left := 0, math.NaN(), 0.0
	flush := func(end int, line js.Value) {
		if end <= start {
			return
		}
		ctx.Call("fillText", strings.TrimRight(n.Call("substringData", start, end-start).String(), "\n"), left-p.x0,
			line.Get("top").Float()-p.y0+line.Get("height").Float()/2)
	}
	var last js.Value
	for i := 0; i < units; i++ {
		rng.Call("setStart", n, i)
		rng.Call("setEnd", n, i+1)
		r := rng.Call("getBoundingClientRect")
		if r.Get("width").Float() == 0 && r.Get("height").Float() == 0 {
			continue
		}
		if t := r.Get("top").Float(); math.IsNaN(top) || math.Abs(t-top) > 1 {
			if !math.IsNaN(top) {
				flush(i, last)
			}
			start, top, left = i, t, r.Get("left").Float()
		}
		last = r
	}
	if !last.IsUndefined() {
		flush(units, last)
	}
	ctx.Call("restore")
}

// fieldText paints a text field's value, which no text node holds.
func (p *painter) fieldText(el, cs js.Value, x, y, w, h float64) {
	v := el.Get("value").String()
	if v == "" {
		v = el.Get("placeholder").String()
	}
	if v == "" {
		return
	}
	ctx := p.ctx
	ctx.Set("font", cs.Get("font").String())
	ctx.Set("fillStyle", cs.Get("color").String())
	ctx.Set("textBaseline", "middle")
	pad := parseNumber(cs.Get("paddingLeft").String(), 0)
	ctx.Call("fillText", v, x+pad, y+h/2, w)
}

func visibleColor(c string) bool {
	return c != "" && c != "transparent" && c != "rgba(0, 0, 0, 0)"
}

func parseNumber(s string, def float64) float64 {
	f, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "px"), 64)
	if err != nil {
		return def
	}
	return f
}

// sameOrigin reports whether an image can be painted without making the
// picture unreadable.
func sameOrigin(img js.Value) bool {
	src := img.Get("currentSrc").String()
	if strings.HasPrefix(src, "data:") || strings.HasPrefix(src, "blob:") || img.Get("crossOrigin").Truthy() {
		return true
	}
	return strings.HasPrefix(src, js.Global().Get("location").Get("origin").String())
}

// drawEffects paints what changed of the effects that show and draws
// them, in an animation frame.
func (s *surface) drawEffects() {
	for v := range s.effects {
		if s.view(v.id) != v || v.fx == nil {
			delete(s.effects, v)
			continue
		}
		s.drawEffect(v)
	}
}

func (s *surface) drawEffect(v *view) {
	fx := v.fx
	if fx.gl == nil || fx.shader == nil {
		return
	}
	host := s.effectHost(v)
	if host == nil {
		return
	}
	if fx.host != host {
		s.showEffect(fx, false)
		fx.host = host
	}
	sh := fx.shader
	shows := (!sh.Role.Transitions() || sh.Running) && !fx.gl.gl.IsNull()
	if !shows {
		s.showEffect(fx, false)
		return
	}
	ov := sh.Overflow
	if sh.Role.Transitions() {
		live := 3
		if sh.Backwards {
			live = 2
		}
		if sh.Pictures&(1<<live) != 0 {
			s.paintPicture(&fx.pics[live], host, ov, false)
		}
	} else if fx.stale {
		if sh.Pictures&driver.PictureContent != 0 {
			s.paintPicture(&fx.pics[0], host, ov, false)
		}
		if sh.Pictures&driver.PictureBackdrop != 0 && host != s.root {
			s.paintPicture(&fx.pics[1], host, ov, true)
		}
	}
	fx.stale = false
	w, h := float64(host.size.Width)+2*ov, float64(host.size.Height)+2*ov
	setStyle(fx.canvas, map[string]string{"left": webstyle.Px(driver.Points(-ov)), "top": webstyle.Px(driver.Points(-ov)), "width": webstyle.Px(driver.Points(w)), "height": webstyle.Px(driver.Points(h))})
	if !s.drawGL(fx.gl, fx.canvas, v.id, w, h, func(i int) (js.Value, float64, float64) {
		pic := &fx.pics[i]
		if pic.canvas.IsUndefined() || pic.w == 0 {
			return js.Undefined(), 0, 0
		}
		pic.upload(fx.gl)
		return pic.tex, pic.w, pic.h
	}) {
		s.showEffect(fx, false)
		return
	}
	s.showEffect(fx, true)
}

// showEffect puts fx's canvas over its host's children, hiding them, or
// takes it away.
func (s *surface) showEffect(fx *effect, on bool) {
	if fx.shown == on || fx.host == nil {
		return
	}
	fx.shown = on
	if on {
		fx.host.el.Call("appendChild", fx.canvas)
		fx.host.el.Get("classList").Call("add", "nsgo-fx")
	} else {
		fx.canvas.Call("remove")
		fx.host.el.Get("classList").Call("remove", "nsgo-fx")
	}
}

// upload puts the picture's pixels into its texture once it was painted
// anew.
func (p *fxPicture) upload(g *glShader) {
	gl := g.gl
	if p.tex.IsUndefined() {
		p.tex = gl.Call("createTexture")
		gl.Call("bindTexture", gl.Get("TEXTURE_2D"), p.tex)
		for _, k := range []string{"TEXTURE_MIN_FILTER", "TEXTURE_MAG_FILTER"} {
			gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get(k), gl.Get("NEAREST"))
		}
		for _, k := range []string{"TEXTURE_WRAP_S", "TEXTURE_WRAP_T"} {
			gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get(k), gl.Get("CLAMP_TO_EDGE"))
		}
		p.fresh = true
	}
	if !p.fresh {
		return
	}
	p.fresh = false
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), p.tex)
	gl.Call("pixelStorei", gl.Get("UNPACK_PREMULTIPLY_ALPHA_WEBGL"), true)
	gl.Call("texImage2D", gl.Get("TEXTURE_2D"), 0, gl.Get("RGBA"), gl.Get("RGBA"), gl.Get("UNSIGNED_BYTE"), p.canvas)
}

func (p *fxPicture) release(g *glShader) {
	if !p.tex.IsUndefined() && g != nil && !g.gl.IsNull() {
		g.gl.Call("deleteTexture", p.tex)
	}
	*p = fxPicture{}
}

// staleEffects marks the effects whose children an event changed, the
// page's scrolls among them, for their pictures again.
func (s *surface) staleEffects() {
	for v := range s.effects {
		if v.fx != nil {
			v.fx.stale = true
		}
	}
}

func (s *surface) navigating(host *view) bool {
	return host != nil && host.fx != nil && host.fx.shader != nil && host.fx.shader.Role == driver.ShaderNav &&
		host.fx.shader.Running && host.fx.gl != nil && !host.fx.gl.gl.IsNull()
}

func (s *surface) endEffect(v *view) {
	fx := v.fx
	if fx == nil {
		return
	}
	v.fx = nil
	delete(s.effects, v)
	s.showEffect(fx, false)
	for i := range fx.pics {
		fx.pics[i].release(fx.gl)
	}
	fx.before.release(fx.gl)
	if fx.gl != nil {
		fx.gl.release()
	}
}
