// Package webstyle writes NSGo's styles as CSS, for the backends that
// draw with a browser's engine: the web and WeChat mini programs, whose
// render layer is a web view. It is plain Go, so it is tested on any host.
//
// Lengths are CSS pixels, which are points; what is at CSS's initial value
// comes back empty, so callers leave the property out.
package webstyle

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Px is p in CSS pixels.
func Px(p driver.Points) string {
	var b [24]byte
	return string(append(strconv.AppendFloat(b[:0], float64(p), 'g', -1, 32), "px"...))
}

// Color is c as CSS.
func Color(c driver.Color) string {
	r, g, b, a := c.Components()
	return fmt.Sprintf("rgba(%d,%d,%d,%g)", int(r*255+0.5), int(g*255+0.5), int(b*255+0.5), a)
}

// Fill is the CSS of a background or border color, "" for none.
func Fill(c driver.Color) string {
	if c == 0 {
		return ""
	}
	return Color(c)
}

// Opacity is the CSS of an opacity, "" for opaque.
func Opacity(o float32) string {
	if o == 1 {
		return ""
	}
	return fmt.Sprint(o)
}

// Transform is t around the view's transform origin, in its order:
// perspective, translate, rotate, rotateX, rotateY, rotateZ, skew, then
// scale.
func Transform(t driver.Transform) string {
	if t == (driver.Transform{}) {
		return ""
	}
	sx, sy := t.Scales()
	var b strings.Builder
	if t.Perspective > 0 {
		fmt.Fprintf(&b, "perspective(%s) ", Px(t.Perspective))
	}
	fmt.Fprintf(&b, "translate(%s, %s) rotate(%gdeg)", Px(t.TranslateX), Px(t.TranslateY), t.Rotate)
	if !t.Flat() {
		fmt.Fprintf(&b, " rotateX(%gdeg) rotateY(%gdeg)", t.RotateX, t.RotateY)
	}
	if t.RotateZ != 0 {
		fmt.Fprintf(&b, " rotate(%gdeg)", t.RotateZ)
	}
	if t.SkewX != 0 || t.SkewY != 0 {
		fmt.Fprintf(&b, " skew(%gdeg, %gdeg)", t.SkewX, t.SkewY)
	}
	fmt.Fprintf(&b, " scale(%g, %g)", sx, sy)
	if t.Then != (driver.Matrix3D{}) {
		b.WriteString(" matrix3d(")
		for i, v := range t.Then {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%g", v)
		}
		b.WriteByte(')')
	}
	return b.String()
}

// transitionProperties are the CSS properties of each transition
// property.
var transitionProperties = []struct {
	property driver.TransitionProperty
	css      []string
}{
	{driver.TransitionOpacity, []string{"opacity"}},
	{driver.TransitionBackground, []string{"background-color"}},
	{driver.TransitionTransform, []string{"transform"}},
	{driver.TransitionBorder, []string{"border-width", "border-color", "border-radius", "outline-color", "outline-width", "outline-offset"}},
	{driver.TransitionShadow, []string{"box-shadow"}},
}

// Transition animates the listed properties from what the view shows to
// what the same style change sets.
func Transition(t driver.Transition) string {
	if t.Duration <= 0 {
		return ""
	}
	timing := fmt.Sprintf(" %gms %s %gms", float64(t.Duration)/1e6, Easing(t.Easing), float64(t.Delay)/1e6)
	var parts []string
	for _, p := range transitionProperties {
		if t.Properties == 0 || t.Properties&p.property != 0 {
			for _, name := range p.css {
				parts = append(parts, name+timing)
			}
		}
	}
	return strings.Join(parts, ", ")
}

// Easing is the CSS timing function of a curve, linear for none.
func Easing(c driver.Curve) string {
	if c == (driver.Curve{}) {
		return "linear"
	}
	return fmt.Sprintf("cubic-bezier(%g, %g, %g, %g)", c.X1, c.Y1, c.X2, c.Y2)
}

// Radius is the border-radius of vs, which the browser fits.
func Radius(vs *driver.ViewStyle) string {
	if c := vs.Corners; c != (driver.Corners{}) {
		return fmt.Sprintf("%s %s %s %s", Px(c.TopLeft), Px(c.TopRight), Px(c.BottomRight), Px(c.BottomLeft))
	}
	if vs.CornerRadius == 0 {
		return ""
	}
	return Px(vs.CornerRadius)
}

// LineStyle is the CSS of a border or outline style.
func LineStyle(s driver.BorderStyle) string {
	switch s {
	case driver.BorderDashed:
		return "dashed"
	case driver.BorderDotted:
		return "dotted"
	}
	return "solid"
}

// Shadow is the box-shadow of vs: its layers, then React Native's shadow
// under them.
func Shadow(vs *driver.ViewStyle) string {
	var layers []string
	if e := vs.Effects; e != nil {
		for _, sh := range e.Shadows {
			layer := fmt.Sprintf("%s %s %s %s %s", Px(sh.Offset.X), Px(sh.Offset.Y), Px(sh.Blur), Px(sh.Spread), Color(sh.Color))
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
			Px(vs.ShadowOffset.X), Px(vs.ShadowOffset.Y), Px(2*vs.ShadowRadius), Color(vs.ShadowColor)))
	}
	return strings.Join(layers, ", ")
}

// Gradient is the CSS gradient of p on a box of size.
func Gradient(p *driver.GradientPaint, size driver.Size) string {
	var b strings.Builder
	stops := func(scale func(float32) string) {
		for _, s := range p.Stops {
			fmt.Fprintf(&b, ", %s %s", Color(s.Color), scale(s.Offset))
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

// Filter is CSS's filter of filters, or "" for none.
func Filter(filters []driver.Filter) string {
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
			parts = append(parts, fmt.Sprintf("drop-shadow(%s %s %s %s)", Px(sh.Offset.X), Px(sh.Offset.Y), Px(sh.Blur), Color(sh.Color)))
		}
	}
	return strings.Join(parts, " ")
}

// TextShadow is the CSS of a text shadow.
func TextShadow(s driver.TextShadow) string {
	if s.Color == 0 {
		return "none"
	}
	return fmt.Sprintf("%s %s %s %s", Px(s.Offset.X), Px(s.Offset.Y), Px(s.Radius), Color(s.Color))
}

// DecorationStyles are the CSS of the decoration styles.
var DecorationStyles = map[driver.TextDecorationStyle]string{
	0: "solid", driver.DecorationSolid: "solid", driver.DecorationDouble: "double", driver.DecorationDotted: "dotted",
	driver.DecorationDashed: "dashed", driver.DecorationWavy: "wavy",
}

// Decoration is CSS's text-decoration-line of d, "none" for none.
func Decoration(d driver.TextDecoration) string {
	var lines []string
	if d&driver.TextDecorationUnderline != 0 {
		lines = append(lines, "underline")
	}
	if d&driver.TextDecorationLineThrough != 0 {
		lines = append(lines, "line-through")
	}
	if len(lines) == 0 {
		return "none"
	}
	return strings.Join(lines, " ")
}
