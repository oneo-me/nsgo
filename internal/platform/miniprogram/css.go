package miniprogram

import (
	"fmt"
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/webstyle"
)

// The render layer is a web view, so a view's style is its inline CSS,
// written as the web backend's classes are: background, border, corners,
// shadows, gradients, filters, transforms, transitions and clipping.

// css builds declarations.
type css struct{ strings.Builder }

// add writes name: value; for a value that is not empty.
func (c *css) add(name, value string) {
	if value != "" {
		c.WriteString(name)
		c.WriteByte(':')
		c.WriteString(value)
		c.WriteByte(';')
	}
}

// viewCSS is the CSS of v's style at its size, and what its props make of
// touches. A scroll view's overflow is the scroll view's own.
func viewCSS(v *view, parent *view) string {
	var c css
	vs := &v.style
	if vs.Hidden {
		c.add("display", "none")
	}
	c.add("background-color", webstyle.Fill(vs.Background))
	c.add("opacity", webstyle.Opacity(vs.Opacity))
	c.add("transform", webstyle.Transform(vs.Transform))
	if t := webstyle.Transition(vs.Transition); t != "" {
		c.add("transition", t)
	}
	c.add("border-radius", webstyle.Radius(vs))
	if o := vs.Outline; o.Visible() {
		c.add("outline", webstyle.Px(o.Width)+" "+webstyle.LineStyle(o.Style)+" "+webstyle.Color(o.Color))
		c.add("outline-offset", webstyle.Px(o.Offset))
	}
	c.add("box-shadow", webstyle.Shadow(vs))
	if vs.BlendMode != driver.BlendNormal {
		c.add("mix-blend-mode", vs.BlendMode.Name())
	}
	if o := vs.Origin; o != (driver.Point{}) || vs.OriginZ != 0 {
		c.add("transform-origin", fmt.Sprintf("calc(50%% + %s) calc(50%% + %s) %s",
			webstyle.Px(o.X), webstyle.Px(o.Y), webstyle.Px(vs.OriginZ)))
	}
	if vs.ChildPerspective > 0 {
		c.add("perspective", webstyle.Px(vs.ChildPerspective))
	}
	if vs.BackfaceHidden {
		c.add("backface-visibility", "hidden")
	}
	if e := vs.Effects; e != nil {
		if len(e.Gradients) > 0 {
			images := make([]string, len(e.Gradients))
			for i := range e.Gradients {
				images[i] = webstyle.Gradient(&e.Gradients[i], v.frame.Size)
			}
			c.add("background-image", strings.Join(images, ", "))
		}
		c.add("filter", webstyle.Filter(e.Filters))
		if f := webstyle.Filter(e.BackdropFilters); f != "" {
			c.add("backdrop-filter", f)
			c.add("-webkit-backdrop-filter", f)
		}
		if e.Mask != nil {
			m := webstyle.Gradient(e.Mask, v.frame.Size)
			c.add("mask-image", m)
			c.add("-webkit-mask-image", m)
		}
	}
	border, colors := v.border(), driver.AllColors(vs.BorderColor)
	if vs.Borders != (driver.Edges{}) {
		colors = vs.BorderColors
	}
	if border != (driver.Edges{}) {
		c.add("border-style", webstyle.LineStyle(vs.BorderStyle))
		if border == driver.All(border.Top) {
			c.add("border-width", webstyle.Px(border.Top))
		} else {
			c.add("border-width", webstyle.Px(border.Top)+" "+webstyle.Px(border.Right)+" "+
				webstyle.Px(border.Bottom)+" "+webstyle.Px(border.Left))
		}
		if colors == driver.AllColors(colors.Top) {
			c.add("border-color", webstyle.Color(colors.Top))
		} else {
			c.add("border-color", webstyle.Color(colors.Top)+" "+webstyle.Color(colors.Right)+" "+
				webstyle.Color(colors.Bottom)+" "+webstyle.Color(colors.Left))
		}
	}
	if v.kind != driver.KindScroll && vs.Clip {
		c.add("overflow", "hidden")
	}
	// Touches: a view that takes none passes them to what is under it;
	// box-none passes its own but not its children's, which CSS
	// inherits, so its children take theirs back.
	switch v.props.PointerEvents {
	case driver.PointerEventsNone, driver.PointerEventsBoxNone:
		c.add("pointer-events", "none")
	default:
		if parent != nil && parent.props.PointerEvents == driver.PointerEventsBoxNone {
			c.add("pointer-events", "auto")
		}
	}
	return c.String()
}

// animatedCSS is what v's animated values and sticky shift draw over its
// style, "" when nothing does.
func animatedCSS(v *view) string {
	vals := v.animated
	if vals.Set == 0 && v.shift == (driver.Point{}) {
		return ""
	}
	vs := vals.Apply(v.style)
	var c css
	transform := webstyle.Transform(vs.Transform)
	if v.shift != (driver.Point{}) {
		transform = strings.TrimSpace("translate(" + webstyle.Px(v.shift.X) + ", " + webstyle.Px(v.shift.Y) + ") " + transform)
	}
	if transform == "" {
		transform = "none"
	}
	c.add("transform", transform)
	if vals.Set == 0 {
		return c.String()
	}
	opacity := webstyle.Opacity(vs.Opacity)
	if opacity == "" {
		opacity = "1"
	}
	c.add("opacity", opacity)
	// A bound value moves at once; the view's transition is for its
	// style's changes.
	c.add("transition", "none")
	if bg := webstyle.Fill(vs.Background); bg != "" {
		c.add("background-color", bg)
	}
	if sh := webstyle.Shadow(&vs); sh != "" {
		c.add("box-shadow", sh)
	}
	if vals.Has(driver.PropBorderColor) {
		c.add("border-color", webstyle.Color(vs.BorderColor))
	}
	// A bound color fills every glyph, the runs' too, which inherit the
	// fill color over their own.
	if vals.Has(driver.PropTextColor) {
		c.add("-webkit-text-fill-color", webstyle.Color(vals.TextColor))
	}
	return c.String()
}

// layoutTransitionCSS moves a view's frame as tr says: CSS's transition of
// its box, a spring sampled as an ease-out.
func layoutTransitionCSS(tr *driver.LayoutTransition) string {
	easing := "ease-in-out"
	switch tr.Curve {
	case driver.LayoutLinear:
		easing = "linear"
	case driver.LayoutEaseIn:
		easing = "ease-in"
	case driver.LayoutEaseOut, driver.LayoutSpring:
		easing = "cubic-bezier(0.2, 0.8, 0.2, 1)"
	case driver.LayoutBezier:
		easing = webstyle.Easing(tr.Bezier)
	}
	timing := fmt.Sprintf(" %gms %s %gms", float64(tr.Duration)/1e6, easing, float64(tr.Delay)/1e6)
	return "left" + timing + ",top" + timing + ",width" + timing + ",height" + timing
}
