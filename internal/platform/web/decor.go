//go:build js && wasm

package web

import (
	"fmt"
	"math"
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/webstyle"
)

// The browser draws what CSS has: per-corner radii, dashed borders,
// outlines, layered shadows, gradients, filters, backdrop filters, masks,
// blend modes and 3D transforms are CSS properties of the view's element.
// Gradients arrive resolved against the box in fractions, so their CSS is
// written again when the box changes size.

// decorCSS appends the CSS of vs's shape and effects on a box of size;
// what is at CSS's initial value it leaves out.
func decorCSS(c *cssText, vs *driver.ViewStyle, size driver.Size) {
	c.add("border-radius", webstyle.Radius(vs))
	if vs.ContinuousCorners {
		c.add("corner-shape", "squircle")
	}
	if o := vs.Outline; o.Visible() {
		c.add("outline", webstyle.Px(o.Width)+" "+webstyle.LineStyle(o.Style)+" "+webstyle.Color(o.Color))
		c.add("outline-offset", webstyle.Px(o.Offset))
	}
	c.add("box-shadow", webstyle.Shadow(vs))
	if vs.BlendMode != driver.BlendNormal {
		c.add("mix-blend-mode", vs.BlendMode.Name())
	}
	if o := vs.Origin; o != (driver.Point{}) || vs.OriginZ != 0 {
		c.add("transform-origin", fmt.Sprintf("calc(50%% + %s) calc(50%% + %s) %s", webstyle.Px(o.X), webstyle.Px(o.Y), webstyle.Px(vs.OriginZ)))
	}
	if vs.ChildPerspective > 0 {
		c.add("perspective", webstyle.Px(vs.ChildPerspective))
	}
	if vs.BackfaceHidden {
		c.add("backface-visibility", "hidden")
	}
	e := vs.Effects
	if e == nil {
		return
	}
	if len(e.Gradients) > 0 {
		images := make([]string, len(e.Gradients))
		for i := range e.Gradients {
			images[i] = webstyle.Gradient(&e.Gradients[i], size)
		}
		c.add("background-image", strings.Join(images, ", "))
	}
	c.add("filter", webstyle.Filter(e.Filters))
	if f := webstyle.Filter(e.BackdropFilters); f != "" {
		c.add("backdrop-filter", f)
		c.add("-webkit-backdrop-filter", f)
	}
	if e.Mask != nil {
		m := webstyle.Gradient(e.Mask, size)
		c.add("mask-image", m)
		c.add("-webkit-mask-image", m)
	}
}

// redrawGradients writes v's gradients again for its new size.
func (s *surface) redrawGradients(v *view) {
	if e := v.style.Effects; e != nil && (len(e.Gradients) > 0 || e.Mask != nil) {
		s.setLook(v)
	}
}

// setKeyframes runs v's keyframe animation with the Web Animations API,
// on the individual transform properties and an additive opacity, so it
// goes on top of the view's own transform and opacity.
func (s *surface) setKeyframes(v *view, a driver.KeyframeAnimation) {
	if a == v.keyframes {
		return
	}
	v.keyframes = a
	for _, anim := range v.running {
		anim.Call("cancel")
	}
	v.running = v.running[:0]
	d, frames := a.Cycle()
	if d <= 0 {
		return
	}
	iterations := any(math.Inf(1))
	if a.Iterations > 0 {
		iterations = a.Iterations
	}
	timing := map[string]any{"duration": float64(d) / 1e6, "delay": float64(a.Delay) / 1e6, "iterations": iterations}
	var moves, fades []any
	for _, k := range frames {
		ease := "linear"
		if c := k.Easing; c != (driver.Curve{}) {
			ease = fmt.Sprintf("cubic-bezier(%g, %g, %g, %g)", c.X1, c.Y1, c.X2, c.Y2)
		}
		moves = append(moves, map[string]any{"offset": k.Offset, "easing": ease,
			"rotate": fmt.Sprintf("%gdeg", k.Rotate), "scale": fmt.Sprint(k.Scale),
			"translate": fmt.Sprintf("0 %g%%", k.TranslateY*100)})
		fades = append(fades, map[string]any{"offset": k.Offset, "easing": ease, "opacity": k.Opacity - 1})
	}
	v.running = append(v.running, v.el.Call("animate", moves, timing))
	fade := map[string]any{"composite": "add"}
	for k, x := range timing {
		fade[k] = x
	}
	v.running = append(v.running, v.el.Call("animate", fades, fade))
}

// stopKeyframes cancels v's keyframe animation.
func (v *view) stopKeyframes() {
	for _, anim := range v.running {
		anim.Call("cancel")
	}
	v.running = nil
	v.keyframes = driver.KeyframeAnimation{}
}
