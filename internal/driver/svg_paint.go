package driver

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// parseSVGPaint reads a paint. It reports ok false for a paint it does not
// read, such as inherit or an unknown keyword, so the inherited one stays.
// A reference to an element is read whatever it names; what it names is
// looked up when a shape is painted, so it may come later in the document.
func parseSVGPaint(v string) (p svgPaint, ok bool, err error) {
	v = strings.TrimSpace(v)
	switch {
	case v == "none", strings.EqualFold(v, "transparent"):
		return svgPaint{kind: paintNone}, true, nil
	case strings.EqualFold(v, "currentColor"):
		return svgPaint{kind: paintCurrent}, true, nil
	case strings.HasPrefix(v, "url("):
		end := strings.IndexByte(v, ')')
		if end < 0 {
			return p, false, nil
		}
		id, _ := svgURL(v[:end+1])
		p = svgPaint{kind: paintURL, ref: id}
		if rest := strings.TrimSpace(v[end+1:]); rest != "" {
			if fb, ok, _ := parseSVGPaint(rest); ok && fb.kind != paintURL {
				p.fall, p.color = fb.kind, fb.color
			}
		}
		return p, true, nil
	}
	c, ok, err := parseSVGColor(v)
	if err != nil || !ok {
		return p, false, err
	}
	return fixedPaint(c), true, nil
}

// parseSVGColor reads a hex color, rgb(), rgba(), hsl(), hsla(), a CSS
// color name or transparent. A malformed hex color is an error; anything
// else it does not read reports ok false.
func parseSVGColor(v string) (c Color, ok bool, err error) {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "#") {
		c, err := parseSVGHex(v[1:])
		return c, err == nil, err
	}
	lower := strings.ToLower(v)
	if open := strings.IndexByte(lower, '('); open > 0 && strings.HasSuffix(lower, ")") && strings.TrimSpace(lower[:open]) == "color-mix" {
		return cssColorMix(lower[open+1 : len(lower)-1])
	}
	if open := strings.IndexByte(lower, '('); open > 0 && strings.HasSuffix(lower, ")") {
		args := strings.FieldsFunc(lower[open+1:len(lower)-1], func(r rune) bool {
			return r == ',' || r == '/' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
		})
		switch strings.TrimSpace(lower[:open]) {
		case "rgb", "rgba":
			return cssRGB(args)
		case "hsl", "hsla":
			return cssHSL(args)
		case "oklab":
			return cssOklab(args, ColorSpaceOklab)
		case "oklch":
			return cssOklab(args, ColorSpaceOklch)
		}
		return 0, false, nil
	}
	if lower == "transparent" {
		return 0, true, nil
	}
	if i := strings.Index(cssColorNames, " "+lower+":"); i >= 0 {
		hex := cssColorNames[i+len(lower)+2:]
		n, err := strconv.ParseUint(hex[:6], 16, 32)
		if err == nil {
			return RGB(uint32(n)), true, nil
		}
	}
	return 0, false, nil
}

func parseSVGHex(h string) (Color, error) {
	var r, g, b, a uint32
	a = 0xff
	switch len(h) {
	case 3, 4:
		v, err := strconv.ParseUint(h, 16, 16)
		if err != nil {
			return 0, err
		}
		n := func(shift uint) uint32 { return uint32(v>>shift&0xf) * 0x11 }
		r, g, b = n(8), n(4), n(0)
		if len(h) == 4 {
			r, g, b = n(12), n(8), n(4)
			a = n(0)
		}
	case 6, 8:
		v, err := strconv.ParseUint(h, 16, 64)
		if err != nil {
			return 0, err
		}
		if len(h) == 8 {
			r, g, b, a = uint32(v>>24&0xff), uint32(v>>16&0xff), uint32(v>>8&0xff), uint32(v&0xff)
		} else {
			v <<= 8
			r, g, b = uint32(v>>24&0xff), uint32(v>>16&0xff), uint32(v>>8&0xff)
		}
	default:
		return 0, fmt.Errorf("bad color %q", h)
	}
	return Color(r<<24 | g<<16 | b<<8 | a), nil
}

// cssChannel reads a channel: a number up to scale, or a percentage.
func cssChannel(v string, scale float64) (float64, bool) {
	pct := strings.HasSuffix(v, "%")
	if pct {
		v = v[:len(v)-1]
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n != n {
		return 0, false
	}
	if pct {
		n = n / 100 * scale
	}
	return min(max(n/scale, 0), 1), true
}

// cssAlpha reads the optional alpha argument at args[i].
func cssAlpha(args []string, i int) (float64, bool) {
	if len(args) <= i {
		return 1, true
	}
	return cssChannel(args[i], 1)
}

func cssRGB(args []string) (Color, bool, error) {
	if len(args) < 3 || len(args) > 4 {
		return 0, false, nil
	}
	var ch [4]float64
	for i := range 3 {
		n, ok := cssChannel(args[i], 255)
		if !ok {
			return 0, false, nil
		}
		ch[i] = n
	}
	a, ok := cssAlpha(args, 3)
	if !ok {
		return 0, false, nil
	}
	ch[3] = a
	return packColor(ch), true, nil
}

func cssHSL(args []string) (Color, bool, error) {
	if len(args) < 3 || len(args) > 4 {
		return 0, false, nil
	}
	hue := args[0]
	turns := 1.0 / 360
	for _, u := range []struct {
		suffix string
		turns  float64
	}{{"deg", 1.0 / 360}, {"grad", 1.0 / 400}, {"rad", 1 / (2 * math.Pi)}, {"turn", 1}} {
		if strings.HasSuffix(hue, u.suffix) {
			hue, turns = hue[:len(hue)-len(u.suffix)], u.turns
			break
		}
	}
	h, err := strconv.ParseFloat(hue, 64)
	if err != nil || h != h || math.IsInf(h, 0) {
		return 0, false, nil
	}
	h *= turns
	h -= math.Floor(h)
	s, ok1 := cssChannel(args[1], 100)
	l, ok2 := cssChannel(args[2], 100)
	a, ok3 := cssAlpha(args, 3)
	if !ok1 || !ok2 || !ok3 {
		return 0, false, nil
	}
	f := func(n float64) float64 {
		k := math.Mod(n+h*12, 12)
		return l - s*min(l, 1-l)*max(-1, min(k-3, 9-k, 1))
	}
	return packColor([4]float64{f(0), f(8), f(4), a}), true, nil
}

func packColor(ch [4]float64) Color {
	var c uint32
	for _, v := range ch {
		c = c<<8 | uint32(min(max(v, 0), 1)*255+0.5)
	}
	return Color(c)
}

// cssColorNames are CSS's named colors, each as " name:rrggbb".
const cssColorNames = " aliceblue:f0f8ff antiquewhite:faebd7 aqua:00ffff aquamarine:7fffd4" +
	" azure:f0ffff beige:f5f5dc bisque:ffe4c4 black:000000 blanchedalmond:ffebcd blue:0000ff" +
	" blueviolet:8a2be2 brown:a52a2a burlywood:deb887 cadetblue:5f9ea0 chartreuse:7fff00" +
	" chocolate:d2691e coral:ff7f50 cornflowerblue:6495ed cornsilk:fff8dc crimson:dc143c" +
	" cyan:00ffff darkblue:00008b darkcyan:008b8b darkgoldenrod:b8860b darkgray:a9a9a9" +
	" darkgreen:006400 darkgrey:a9a9a9 darkkhaki:bdb76b darkmagenta:8b008b darkolivegreen:556b2f" +
	" darkorange:ff8c00 darkorchid:9932cc darkred:8b0000 darksalmon:e9967a darkseagreen:8fbc8f" +
	" darkslateblue:483d8b darkslategray:2f4f4f darkslategrey:2f4f4f darkturquoise:00ced1" +
	" darkviolet:9400d3 deeppink:ff1493 deepskyblue:00bfff dimgray:696969 dimgrey:696969" +
	" dodgerblue:1e90ff firebrick:b22222 floralwhite:fffaf0 forestgreen:228b22 fuchsia:ff00ff" +
	" gainsboro:dcdcdc ghostwhite:f8f8ff gold:ffd700 goldenrod:daa520 gray:808080 green:008000" +
	" greenyellow:adff2f grey:808080 honeydew:f0fff0 hotpink:ff69b4 indianred:cd5c5c" +
	" indigo:4b0082 ivory:fffff0 khaki:f0e68c lavender:e6e6fa lavenderblush:fff0f5" +
	" lawngreen:7cfc00 lemonchiffon:fffacd lightblue:add8e6 lightcoral:f08080 lightcyan:e0ffff" +
	" lightgoldenrodyellow:fafad2 lightgray:d3d3d3 lightgreen:90ee90 lightgrey:d3d3d3" +
	" lightpink:ffb6c1 lightsalmon:ffa07a lightseagreen:20b2aa lightskyblue:87cefa" +
	" lightslategray:778899 lightslategrey:778899 lightsteelblue:b0c4de lightyellow:ffffe0" +
	" lime:00ff00 limegreen:32cd32 linen:faf0e6 magenta:ff00ff maroon:800000" +
	" mediumaquamarine:66cdaa mediumblue:0000cd mediumorchid:ba55d3 mediumpurple:9370db" +
	" mediumseagreen:3cb371 mediumslateblue:7b68ee mediumspringgreen:00fa9a" +
	" mediumturquoise:48d1cc mediumvioletred:c71585 midnightblue:191970 mintcream:f5fffa" +
	" mistyrose:ffe4e1 moccasin:ffe4b5 navajowhite:ffdead navy:000080 oldlace:fdf5e6" +
	" olive:808000 olivedrab:6b8e23 orange:ffa500 orangered:ff4500 orchid:da70d6" +
	" palegoldenrod:eee8aa palegreen:98fb98 paleturquoise:afeeee palevioletred:db7093" +
	" papayawhip:ffefd5 peachpuff:ffdab9 peru:cd853f pink:ffc0cb plum:dda0dd powderblue:b0e0e6" +
	" purple:800080 rebeccapurple:663399 red:ff0000 rosybrown:bc8f8f royalblue:4169e1" +
	" saddlebrown:8b4513 salmon:fa8072 sandybrown:f4a460 seagreen:2e8b57 seashell:fff5ee" +
	" sienna:a0522d silver:c0c0c0 skyblue:87ceeb slateblue:6a5acd slategray:708090" +
	" slategrey:708090 snow:fffafa springgreen:00ff7f steelblue:4682b4 tan:d2b48c teal:008080" +
	" thistle:d8bfd8 tomato:ff6347 turquoise:40e0d0 violet:ee82ee wheat:f5deb3 white:ffffff" +
	" whitesmoke:f5f5f5 yellow:ffff00 yellowgreen:9acd32"

// svgURL reads the id in a url(#id) reference.
func svgURL(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "url(") || !strings.HasSuffix(v, ")") {
		return "", false
	}
	v = strings.Trim(strings.TrimSpace(v[4:len(v)-1]), `"'`)
	if !strings.HasPrefix(v, "#") || len(v) == 1 {
		return "", false
	}
	return v[1:], true
}

// svgHref reads the id an element's href or xlink:href names in the
// document; references to other documents name nothing.
func svgHref(a map[string]string) string {
	v := strings.TrimSpace(a["href"])
	if !strings.HasPrefix(v, "#") {
		return ""
	}
	return v[1:]
}

// svgMatrix is an affine transform as SVG's matrix(a, b, c, d, e, f).
type svgMatrix struct{ a, b, c, d, e, f float64 }

var svgIdentity = svgMatrix{a: 1, d: 1}

func svgTranslate(x, y float32) svgMatrix {
	return svgMatrix{a: 1, d: 1, e: float64(x), f: float64(y)}
}

// mul returns the transform that applies n, then m.
func (m svgMatrix) mul(n svgMatrix) svgMatrix {
	return svgMatrix{
		a: m.a*n.a + m.c*n.b,
		b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d,
		d: m.b*n.c + m.d*n.d,
		e: m.a*n.e + m.c*n.f + m.e,
		f: m.b*n.e + m.d*n.f + m.f,
	}
}

func (m svgMatrix) apply(x, y float32) (float32, float32) {
	fx, fy := float64(x), float64(y)
	return float32(m.a*fx + m.c*fy + m.e), float32(m.b*fx + m.d*fy + m.f)
}

// parseSVGTransform reads a transform list. It reports false for one it
// cannot read, which leaves the element untransformed.
func parseSVGTransform(v string) (svgMatrix, bool) {
	m := svgIdentity
	s := v
	for {
		s = strings.TrimLeft(s, " \t\n\r\f,")
		if s == "" {
			return m, true
		}
		open := strings.IndexByte(s, '(')
		end := strings.IndexByte(s, ')')
		if open < 0 || end < open {
			return svgIdentity, false
		}
		args, err := svgNumbers(s[open+1:end], -1)
		if err != nil {
			return svgIdentity, false
		}
		n := len(args)
		var t svgMatrix
		switch strings.TrimSpace(s[:open]) {
		case "matrix":
			if n != 6 {
				return svgIdentity, false
			}
			t = svgMatrix{float64(args[0]), float64(args[1]), float64(args[2]),
				float64(args[3]), float64(args[4]), float64(args[5])}
		case "translate":
			if n != 1 && n != 2 {
				return svgIdentity, false
			}
			t = svgTranslate(args[0], 0)
			if n == 2 {
				t.f = float64(args[1])
			}
		case "scale":
			if n != 1 && n != 2 {
				return svgIdentity, false
			}
			t = svgMatrix{a: float64(args[0]), d: float64(args[0])}
			if n == 2 {
				t.d = float64(args[1])
			}
		case "rotate":
			if n != 1 && n != 3 {
				return svgIdentity, false
			}
			sin, cos := math.Sincos(float64(args[0]) * math.Pi / 180)
			t = svgMatrix{a: cos, b: sin, c: -sin, d: cos}
			if n == 3 {
				t = svgTranslate(args[1], args[2]).mul(t).mul(svgTranslate(-args[1], -args[2]))
			}
		case "skewX":
			if n != 1 {
				return svgIdentity, false
			}
			t = svgMatrix{a: 1, c: math.Tan(float64(args[0]) * math.Pi / 180), d: 1}
		case "skewY":
			if n != 1 {
				return svgIdentity, false
			}
			t = svgMatrix{a: 1, b: math.Tan(float64(args[0]) * math.Pi / 180), d: 1}
		default:
			return svgIdentity, false
		}
		m = m.mul(t)
		s = s[end+1:]
	}
}

// gradient resolves a linearGradient or radialGradient element for the
// shape in p.local, drawn through ctm. It returns a solid color instead
// for a gradient that paints one, and ok false for one that paints
// nothing: no stops, or bounding box units on a shape without area.
func (p *svgParser) gradient(el *svgNode, ctm svgMatrix) (g *CanvasGradient, solid Color, ok bool) {
	chain := []*svgNode{el}
	for cur := el; len(chain) < maxSVGUses; {
		next := p.ids[svgHref(cur.attrs)]
		if next == nil || next.name != "linearGradient" && next.name != "radialGradient" ||
			slices.Contains(chain, next) {
			break
		}
		chain = append(chain, next)
		cur = next
	}
	attr := func(name string) (string, bool) {
		for _, n := range chain {
			if v, ok := n.attrs[name]; ok {
				return strings.TrimSpace(v), true
			}
		}
		return "", false
	}
	var stops []GradientStop
	for _, n := range chain {
		last := float32(0)
		for _, c := range n.children {
			if c.name != "stop" {
				continue
			}
			off := svgOpacity(c.attrs["offset"])
			if _, ok := c.attrs["offset"]; !ok {
				off = 0
			}
			off = max(off, last)
			last = off
			color := RGB(0x000000)
			if v, ok := c.attrs["stop-color"]; ok {
				if c, ok, _ := parseSVGColor(v); ok {
					color = c
				}
			}
			if v, ok := c.attrs["stop-opacity"]; ok {
				color = svgAlpha(color, svgOpacity(v))
			}
			stops = append(stops, GradientStop{Offset: off, Color: color})
		}
		if len(stops) > 0 {
			break
		}
	}
	switch len(stops) {
	case 0:
		return nil, 0, false
	case 1:
		return nil, stops[0].Color, true
	}
	lastColor := stops[len(stops)-1].Color
	units, _ := attr("gradientUnits")
	bbox := units != "userSpaceOnUse"
	m := ctm
	if bbox {
		b, ok := svgBounds(p.local)
		if !ok || b.w <= 0 || b.h <= 0 {
			return nil, 0, false
		}
		m = m.mul(svgMatrix{a: float64(b.w), d: float64(b.h), e: float64(b.x), f: float64(b.y)})
	}
	if v, ok := attr("gradientTransform"); ok {
		if t, ok := parseSVGTransform(v); ok {
			m = m.mul(t)
		}
	}
	diag := float32(math.Sqrt(float64(p.vw*p.vw+p.vh*p.vh) / 2))
	coord := func(name, def string, ref float32) float32 {
		v, ok := attr(name)
		if !ok || v == "" {
			v = def
		}
		if bbox {
			ref = 1
		}
		n, err := svgLengthOf(v, ref)
		if err != nil {
			n, _ = svgLengthOf(def, ref)
		}
		return n
	}
	g = &CanvasGradient{Stops: stops}
	switch v, _ := attr("spreadMethod"); v {
	case "reflect":
		g.Spread = SpreadReflect
	case "repeat":
		g.Spread = SpreadRepeat
	}
	if el.name == "radialGradient" {
		cx, cy := coord("cx", "50%", p.vw), coord("cy", "50%", p.vh)
		r := coord("r", "50%", diag)
		fx, fy := cx, cy
		if _, ok := attr("fx"); ok {
			fx = coord("fx", "50%", p.vw)
		}
		if _, ok := attr("fy"); ok {
			fy = coord("fy", "50%", p.vh)
		}
		fr := max(coord("fr", "0%", diag), 0)
		if r <= 0 {
			return nil, lastColor, true
		}
		// A focal point outside the circle moves onto it, as SVG 1.1 has it.
		if dx, dy := fx-cx, fy-cy; dx*dx+dy*dy > r*r*0.998 {
			s := r * 0.999 / float32(math.Sqrt(float64(dx*dx+dy*dy)))
			fx, fy = cx+dx*s, cy+dy*s
		}
		g.Radial = true
		g.X1, g.Y1, g.R1 = fx, fy, min(fr, r)
		g.X2, g.Y2, g.R2 = cx, cy, r
	} else {
		g.X1, g.Y1 = coord("x1", "0%", p.vw), coord("y1", "0%", p.vh)
		g.X2, g.Y2 = coord("x2", "100%", p.vw), coord("y2", "0%", p.vh)
		if g.X1 == g.X2 && g.Y1 == g.Y2 {
			return nil, lastColor, true
		}
	}
	g.Transform = [6]float32{float32(m.a), float32(m.b), float32(m.c), float32(m.d), float32(m.e), float32(m.f)}
	return g, 0, true
}

// average returns the gradient's mean color over offsets 0 to 1, the
// fallback for players that paint no gradients.
func (g *CanvasGradient) average() Color {
	var sum [4]float64
	add := func(c0, c1 Color, w float64) {
		r0, g0, b0, a0 := c0.Components()
		r1, g1, b1, a1 := c1.Components()
		sum[0] += w * (r0*a0 + r1*a1) / 2
		sum[1] += w * (g0*a0 + g1*a1) / 2
		sum[2] += w * (b0*a0 + b1*a1) / 2
		sum[3] += w * (a0 + a1) / 2
	}
	s := g.Stops
	first, last := s[0], s[len(s)-1]
	add(first.Color, first.Color, float64(first.Offset))
	for i := 1; i < len(s); i++ {
		add(s[i-1].Color, s[i].Color, float64(s[i].Offset-s[i-1].Offset))
	}
	add(last.Color, last.Color, float64(1-last.Offset))
	if sum[3] <= 0 {
		return 0
	}
	return packColor([4]float64{sum[0] / sum[3], sum[1] / sum[3], sum[2] / sum[3], sum[3]})
}

// svgBounds returns the bounding box of path operations: their geometry's,
// not their control points'.
func svgBounds(ops []DrawOp) (svgBox, bool) {
	x0, y0 := math.Inf(1), math.Inf(1)
	x1, y1 := math.Inf(-1), math.Inf(-1)
	add := func(x, y float64) {
		x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
	}
	var cx, cy float64 // the current point
	for _, op := range ops {
		x, y := float64(op.X), float64(op.Y)
		switch op.Kind {
		case DrawMoveTo, DrawLineTo:
			add(x, y)
			cx, cy = x, y
		case DrawQuadTo:
			add(x, y)
			for _, t := range quadExtrema(cx, float64(op.X1), x) {
				add(quadAt(cx, float64(op.X1), x, t), quadAt(cy, float64(op.Y1), y, t))
			}
			for _, t := range quadExtrema(cy, float64(op.Y1), y) {
				add(quadAt(cx, float64(op.X1), x, t), quadAt(cy, float64(op.Y1), y, t))
			}
			cx, cy = x, y
		case DrawCubicTo:
			add(x, y)
			for _, t := range append(cubicExtrema(cx, float64(op.X1), float64(op.X2), x),
				cubicExtrema(cy, float64(op.Y1), float64(op.Y2), y)...) {
				add(cubicAt(cx, float64(op.X1), float64(op.X2), x, t),
					cubicAt(cy, float64(op.Y1), float64(op.Y2), y, t))
			}
			cx, cy = x, y
		case DrawRect, DrawRoundRect:
			add(x, y)
			add(x+float64(op.W), y+float64(op.H))
			cx, cy = x, y
		case DrawEllipse:
			add(x-float64(op.R), y-float64(op.RY))
			add(x+float64(op.R), y+float64(op.RY))
		}
	}
	if x0 > x1 {
		return svgBox{}, false
	}
	return svgBox{float32(x0), float32(y0), float32(x1 - x0), float32(y1 - y0)}, true
}

func quadAt(p0, p1, p2, t float64) float64 {
	u := 1 - t
	return u*u*p0 + 2*u*t*p1 + t*t*p2
}

func cubicAt(p0, p1, p2, p3, t float64) float64 {
	u := 1 - t
	return u*u*u*p0 + 3*u*u*t*p1 + 3*u*t*t*p2 + t*t*t*p3
}

// quadExtrema returns where in (0, 1) a quadratic curve's coordinate
// turns.
func quadExtrema(p0, p1, p2 float64) []float64 {
	d := p0 - 2*p1 + p2
	if d == 0 {
		return nil
	}
	if t := (p0 - p1) / d; t > 0 && t < 1 {
		return []float64{t}
	}
	return nil
}

// cubicExtrema returns where in (0, 1) a cubic curve's coordinate turns.
func cubicExtrema(p0, p1, p2, p3 float64) []float64 {
	d0, d1, d2 := p1-p0, p2-p1, p3-p2
	a, b, c := d0-2*d1+d2, 2*(d1-d0), d0
	var ts []float64
	keep := func(t float64) {
		if t > 0 && t < 1 {
			ts = append(ts, t)
		}
	}
	if math.Abs(a) < 1e-12 {
		if b != 0 {
			keep(-c / b)
		}
		return ts
	}
	disc := b*b - 4*a*c
	if disc < 0 {
		return nil
	}
	sq := math.Sqrt(disc)
	keep((-b + sq) / (2 * a))
	keep((-b - sq) / (2 * a))
	return ts
}

// cssRule is a rule of a style sheet with one simple selector: a type, a
// class or an id, or a type with classes and an id.
type cssRule struct {
	tag, id     string
	classes     []string
	specificity int
	decls       [][2]string
}

// parseCSSRules reads the rules of style sheets that ParseSVG applies:
// those with simple selectors. Rules with combinators, attribute selectors
// or pseudo-classes, and at-rules, are skipped.
func parseCSSRules(sheets []string) []cssRule {
	var rules []cssRule
	for _, sheet := range sheets {
		s := sheet
		for {
			i := strings.Index(s, "/*")
			if i < 0 {
				break
			}
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				s = s[:i]
				break
			}
			s = s[:i] + " " + s[i+2+j+2:]
		}
		for {
			s = strings.TrimSpace(s)
			if s == "" {
				break
			}
			if s[0] == '@' {
				s = skipCSSAtRule(s)
				continue
			}
			open := strings.IndexByte(s, '{')
			if open < 0 {
				break
			}
			end := strings.IndexByte(s[open:], '}')
			if end < 0 {
				break
			}
			decls := parseCSSDeclarations(s[open+1 : open+end])
			for _, sel := range strings.Split(s[:open], ",") {
				if r, ok := parseCSSSelector(strings.TrimSpace(sel)); ok {
					r.decls = decls
					rules = append(rules, r)
				}
			}
			s = s[open+end+1:]
		}
	}
	return rules
}

// skipCSSAtRule returns what follows the at-rule s starts with.
func skipCSSAtRule(s string) string {
	semi := strings.IndexByte(s, ';')
	open := strings.IndexByte(s, '{')
	if open < 0 || semi >= 0 && semi < open {
		if semi < 0 {
			return ""
		}
		return s[semi+1:]
	}
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return s[i+1:]
			}
		}
	}
	return ""
}

func parseCSSSelector(sel string) (cssRule, bool) {
	if sel == "" || strings.ContainsAny(sel, " \t\n>+~:[()") {
		return cssRule{}, false
	}
	var r cssRule
	i := strings.IndexAny(sel, ".#")
	if i < 0 {
		i = len(sel)
	}
	if tag := sel[:i]; tag != "*" {
		r.tag = tag
	}
	if r.tag != "" {
		r.specificity = 1
	}
	for rest := sel[i:]; rest != ""; {
		j := strings.IndexAny(rest[1:], ".#") + 1
		if j == 0 {
			j = len(rest)
		}
		name := rest[1:j]
		if name == "" {
			return cssRule{}, false
		}
		if rest[0] == '#' {
			r.id = name
			r.specificity += 100
		} else {
			r.classes = append(r.classes, name)
			r.specificity += 10
		}
		rest = rest[j:]
	}
	return r, true
}

func (r *cssRule) matches(n *svgNode) bool {
	if r.tag != "" && r.tag != n.name {
		return false
	}
	if r.id != "" && strings.TrimSpace(n.attrs["id"]) != r.id {
		return false
	}
	if len(r.classes) > 0 {
		classes := strings.Fields(n.attrs["class"])
		for _, c := range r.classes {
			if !slices.Contains(classes, c) {
				return false
			}
		}
	}
	return true
}

// applyCSSRules sets the declarations of the rules that match n on its
// attributes, the more specific ones last.
func applyCSSRules(n *svgNode, rules []cssRule) {
	var matched []*cssRule
	for i := range rules {
		if rules[i].matches(n) {
			matched = append(matched, &rules[i])
		}
	}
	slices.SortStableFunc(matched, func(a, b *cssRule) int { return a.specificity - b.specificity })
	for _, r := range matched {
		for _, d := range r.decls {
			n.attrs[d[0]] = d[1]
		}
	}
}

// parseCSSDeclarations reads "name: value" pairs separated by semicolons.
func parseCSSDeclarations(s string) [][2]string {
	var out [][2]string
	for _, decl := range strings.Split(s, ";") {
		name, value, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "!important"))
		if name != "" && value != "" {
			out = append(out, [2]string{name, value})
		}
	}
	return out
}

// ParseColor reads a CSS color: hex, rgb(), rgba(), hsl(), hsla(), a
// color name, transparent or currentColor, which is CurrentColor, as is
// a color-mix() of it a mark ResolveCurrent reads.
func ParseColor(s string) (Color, error) {
	c, ok, err := parseCSSColor(s)
	if err == nil && !ok {
		err = fmt.Errorf("driver: %q is no CSS color", s)
	}
	return c, err
}

// cssOklab reads oklab(L a b [/ alpha]) or oklch(L C H [/ alpha]): L a
// number from 0 to 1 or a percentage, a and b or C numbers (a percentage
// of 0.4), H in degrees.
func cssOklab(args []string, space ColorSpace) (Color, bool, error) {
	if len(args) != 3 && len(args) != 4 {
		return 0, false, nil
	}
	var v [3]float64
	for i := range 3 {
		a := strings.TrimSuffix(args[i], "deg")
		pct := strings.HasSuffix(a, "%")
		n, err := strconv.ParseFloat(strings.TrimSuffix(a, "%"), 64)
		if err != nil {
			if a == "none" {
				continue
			}
			return 0, false, nil
		}
		if pct {
			n /= 100
			if i > 0 {
				n *= 0.4
			}
		}
		v[i] = n
	}
	alpha := 1.0
	if len(args) == 4 {
		a, ok := cssChannel(args[3], 1)
		if !ok {
			return 0, false, nil
		}
		alpha = a
	}
	r, g, b := fromSpace(v, space)
	clamp := func(x float64) float64 { return math.Min(math.Max(x, 0), 1) }
	return packColor([4]float64{clamp(r), clamp(g), clamp(b), clamp(alpha)}), true, nil
}

// cssColorMix reads color-mix()'s arguments: "in <space>, <color> [p%],
// <color> [p%]", as CSS Color 5 mixes them.
func cssColorMix(args string) (Color, bool, error) {
	var parts []string
	depth, start := 0, 0
	for i, c := range args {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(args[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(args[start:]))
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "in ") {
		return 0, false, nil
	}
	method := strings.Fields(parts[0])
	space := map[string]ColorSpace{"srgb": ColorSpaceSRGB, "srgb-linear": ColorSpaceLinearSRGB,
		"oklab": ColorSpaceOklab, "oklch": ColorSpaceOklch, "xyz": ColorSpaceLinearSRGB,
		"xyz-d65": ColorSpaceLinearSRGB, "hsl": ColorSpaceSRGB, "hwb": ColorSpaceSRGB, "lab": ColorSpaceOklab,
		"lch": ColorSpaceOklch}
	sp, ok := space[method[1]]
	if !ok {
		return 0, false, nil
	}
	read := func(p string) (Color, float64, bool, bool) {
		pct, has := 0.0, false
		if i := strings.LastIndexByte(p, ' '); i > 0 && strings.HasSuffix(p, "%") && !strings.HasSuffix(p, ")") {
			n, err := strconv.ParseFloat(strings.TrimSuffix(p[i+1:], "%"), 64)
			if err != nil {
				return 0, 0, false, false
			}
			pct, has, p = n/100, true, strings.TrimSpace(p[:i])
		}
		c, ok, err := parseCSSColor(p)
		if err != nil || !ok {
			return 0, 0, false, false
		}
		return c, pct, has, true
	}
	c1, p1, has1, ok1 := read(parts[1])
	c2, p2, has2, ok2 := read(parts[2])
	if !ok1 || !ok2 {
		return 0, false, nil
	}
	switch {
	case !has1 && !has2:
		p1, p2 = 0.5, 0.5
	case !has2:
		p2 = 1 - p1
	case !has1:
		p1 = 1 - p2
	}
	sum := p1 + p2
	if sum <= 0 {
		return 0, false, nil
	}
	m := colorMix{a: c1, b: c2, t: float32(p2 / sum), scale: float32(min(sum, 1)), space: sp}
	if hasCurrent(c1) || hasCurrent(c2) {
		return m.mark(), true, nil
	}
	return m.color(), true, nil
}

// parseCSSColor is parseSVGColor with currentColor, which SVG's paints
// read themselves.
func parseCSSColor(s string) (Color, bool, error) {
	if strings.EqualFold(strings.TrimSpace(s), "currentColor") {
		return CurrentColor, true, nil
	}
	return parseSVGColor(s)
}

// CurrentColor, in a style's colors other than the text color itself, is
// the text color the view has: its own, else the one it inherits, else
// black, as CSS's currentColor. It never draws as itself.
const CurrentColor Color = 0x00000001

// A color-mix() of currentColor waits for the text color as a mark: a
// color with no alpha, which draws nothing and so is free, in the range
// of mixMark, holding its index in currentMixes.
const (
	mixMark Color = 0x7f000000
	mixMask Color = 0xff0000ff
)

// colorMix is a color-mix(): b at t over a, its alpha scaled by scale,
// as percentages that add to less than 100% do.
type colorMix struct {
	a, b     Color
	t, scale float32
	space    ColorSpace
}

func (m colorMix) color() Color {
	c := MixColors(m.a, m.b, m.t, m.space)
	if m.scale >= 1 {
		return c
	}
	r, g, b, a := c.Components()
	return packColor([4]float64{r, g, b, a * float64(m.scale)})
}

var currentMixes struct {
	sync.RWMutex
	list  []colorMix
	index map[colorMix]int
}

// mark returns m's mark, the same for the same mix. Apps that write
// more mixes than a mark holds get transparent for the rest.
func (m colorMix) mark() Color {
	mixes := &currentMixes
	mixes.Lock()
	defer mixes.Unlock()
	i, ok := mixes.index[m]
	if !ok {
		if len(mixes.list) >= 1<<16 {
			return 0
		}
		if mixes.index == nil {
			mixes.index = map[colorMix]int{}
		}
		i = len(mixes.list)
		mixes.list = append(mixes.list, m)
		mixes.index[m] = i
	}
	return mixMark | Color(i)<<8
}

func isCurrentMix(c Color) bool { return c&mixMask == mixMark }

func hasCurrent(c Color) bool { return c == CurrentColor || isCurrentMix(c) }

// ResolveCurrent returns c with CurrentColor, or the marks of its mixes,
// replaced by color.
func ResolveCurrent(c, color Color) Color {
	switch {
	case c == CurrentColor:
		return color
	case !isCurrentMix(c):
		return c
	}
	mixes := &currentMixes
	mixes.RLock()
	i := int(c >> 8 & 0xffff)
	if i >= len(mixes.list) {
		mixes.RUnlock()
		return c
	}
	m := mixes.list[i]
	mixes.RUnlock()
	m.a, m.b = ResolveCurrent(m.a, color), ResolveCurrent(m.b, color)
	return m.color()
}
