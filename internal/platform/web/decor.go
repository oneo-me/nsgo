//go:build js && wasm

package web

import (
	"fmt"
	"math"
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
)

// The browser draws what CSS has: per-corner radii, dashed borders,
// outlines, layered shadows, gradients, filters, backdrop filters, masks,
// blend modes and 3D transforms are CSS properties of the view's element.
// Gradients arrive resolved against the box in fractions, so their CSS is
// written again when the box changes size.

// decorCSS appends the CSS of vs's shape and effects on a box of size;
// what is at CSS's initial value it leaves out.
func decorCSS(c *cssText, vs *driver.ViewStyle, size driver.Size) {
	c.add("border-radius", radiusCSS(vs))
	if vs.ContinuousCorners {
		c.add("corner-shape", "squircle")
	}
	if o := vs.Outline; o.Visible() {
		c.add("outline", px(o.Width)+" "+lineStyle(o.Style)+" "+cssColor(o.Color))
		c.add("outline-offset", px(o.Offset))
	}
	c.add("box-shadow", shadowCSS(vs))
	if vs.BlendMode != driver.BlendNormal {
		c.add("mix-blend-mode", vs.BlendMode.Name())
	}
	if o := vs.Origin; o != (driver.Point{}) || vs.OriginZ != 0 {
		c.add("transform-origin", fmt.Sprintf("calc(50%% + %s) calc(50%% + %s) %s", px(o.X), px(o.Y), px(vs.OriginZ)))
	}
	if vs.ChildPerspective > 0 {
		c.add("perspective", px(vs.ChildPerspective))
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
			images[i] = gradientCSS(&e.Gradients[i], size)
		}
		c.add("background-image", strings.Join(images, ", "))
	}
	c.add("filter", filterCSS(e.Filters))
	if f := filterCSS(e.BackdropFilters); f != "" {
		c.add("backdrop-filter", f)
		c.add("-webkit-backdrop-filter", f)
	}
	if e.Mask != nil {
		m := gradientCSS(e.Mask, size)
		c.add("mask-image", m)
		c.add("-webkit-mask-image", m)
	}
}

// radiusCSS is the border-radius of vs, which the browser fits.
func radiusCSS(vs *driver.ViewStyle) string {
	if c := vs.Corners; c != (driver.Corners{}) {
		return fmt.Sprintf("%s %s %s %s", px(c.TopLeft), px(c.TopRight), px(c.BottomRight), px(c.BottomLeft))
	}
	if vs.CornerRadius == 0 {
		return ""
	}
	return px(vs.CornerRadius)
}

// lineStyle is the CSS of a border or outline style.
func lineStyle(s driver.BorderStyle) string {
	switch s {
	case driver.BorderDashed:
		return "dashed"
	case driver.BorderDotted:
		return "dotted"
	}
	return "solid"
}

// shadowCSS is the box-shadow of vs: its layers, then React Native's
// shadow under them.
func shadowCSS(vs *driver.ViewStyle) string {
	var layers []string
	if e := vs.Effects; e != nil {
		for _, sh := range e.Shadows {
			layer := fmt.Sprintf("%s %s %s %s %s", px(sh.Offset.X), px(sh.Offset.Y), px(sh.Blur), px(sh.Spread), cssColor(sh.Color))
			if sh.Inset {
				layer = "inset " + layer
			}
			layers = append(layers, layer)
		}
	}
	if vs.ShadowColor != 0 {
		// React Native's radius is the blur's standard deviation, half
		// CSS's blur radius.
		layers = append(layers, fmt.Sprintf("%s %s %s %s",
			px(vs.ShadowOffset.X), px(vs.ShadowOffset.Y), px(2*vs.ShadowRadius), cssColor(vs.ShadowColor)))
	}
	return strings.Join(layers, ", ")
}

// gradientCSS is the CSS gradient of p on a box of size.
func gradientCSS(p *driver.GradientPaint, size driver.Size) string {
	var b strings.Builder
	stops := func(scale func(float32) string) {
		for _, s := range p.Stops {
			fmt.Fprintf(&b, ", %s %s", cssColor(s.Color), scale(s.Offset))
		}
	}
	pct := func(f float32) string { return fmt.Sprintf("%g%%", f*100) }
	switch p.Kind {
	case driver.GradientRadial:
		fmt.Fprintf(&b, "radial-gradient(%s %s at %s %s", pct(p.RX), pct(p.RY), pct(p.CX), pct(p.CY))
		stops(pct)
	case driver.GradientConic:
		fmt.Fprintf(&b, "conic-gradient(from %gdeg at %s %s", p.Angle, pct(p.CX), pct(p.CY))
		stops(pct)
	default:
		// CSS's line runs through the center at the angle, as long as the
		// corners need; the stops go where they fall on it.
		w, h := float64(size.Width), float64(size.Height)
		x0, y0 := float64(p.X0)*w, float64(p.Y0)*h
		dx, dy := float64(p.X1)*w-x0, float64(p.Y1)*h-y0
		angle := math.Atan2(dx, -dy)
		s, c := math.Sincos(angle)
		line := math.Abs(w*s) + math.Abs(h*c)
		startX, startY := w/2-s*line/2, h/2+c*line/2
		fmt.Fprintf(&b, "linear-gradient(%gdeg", angle*180/math.Pi)
		stops(func(f float32) string {
			x, y := x0+dx*float64(f)-startX, y0+dy*float64(f)-startY
			return fmt.Sprintf("%gpx", x*s-y*c)
		})
	}
	b.WriteByte(')')
	return b.String()
}

// filterCSS is CSS's filter of filters, or "" for none.
func filterCSS(filters []driver.Filter) string {
	var parts []string
	for _, f := range filters {
		a := f.Amount
		switch f.Kind {
		case driver.FilterBlur:
			parts = append(parts, fmt.Sprintf("blur(%gpx)", a))
		case driver.FilterBrightness:
			parts = append(parts, fmt.Sprintf("brightness(%g)", a))
		case driver.FilterContrast:
			parts = append(parts, fmt.Sprintf("contrast(%g)", a))
		case driver.FilterGrayscale:
			parts = append(parts, fmt.Sprintf("grayscale(%g)", a))
		case driver.FilterHueRotate:
			parts = append(parts, fmt.Sprintf("hue-rotate(%gdeg)", a))
		case driver.FilterInvert:
			parts = append(parts, fmt.Sprintf("invert(%g)", a))
		case driver.FilterOpacity:
			parts = append(parts, fmt.Sprintf("opacity(%g)", a))
		case driver.FilterSaturate:
			parts = append(parts, fmt.Sprintf("saturate(%g)", a))
		case driver.FilterSepia:
			parts = append(parts, fmt.Sprintf("sepia(%g)", a))
		case driver.FilterDropShadow:
			sh := f.Shadow
			parts = append(parts, fmt.Sprintf("drop-shadow(%s %s %s %s)", px(sh.Offset.X), px(sh.Offset.Y), px(sh.Blur), cssColor(sh.Color)))
		}
	}
	return strings.Join(parts, " ")
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
