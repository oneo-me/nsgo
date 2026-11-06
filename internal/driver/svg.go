package driver

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// SVG is a parsed SVG document: vector shapes that stay sharp at any size.
// An Icon draws one in the color it inherits, and the image decoders draw
// SVG files with it.
//
// ParseSVG reads static drawings, icons, logos and illustrations: path,
// rect, circle, ellipse, line, polyline and polygon inside svg, g, a,
// switch and use elements, with fills, strokes, stroke widths, caps and
// joins, fill and clip rules, opacities, transforms, linear and radial
// gradients and clip paths. A color may be none, currentColor, a hex
// color, rgb(), rgba(), hsl(), hsla() or a CSS color name. Groups pass
// these to their children, and presentation attributes, the style
// attribute and simple style sheets (type, class and id selectors) set
// them.
//
// Text, images, patterns, masks, filters, markers, dashes and animations
// are not read: an SVG is a drawing, not a page. ParseSVG reads only the
// document, never a file or URL it names.
type SVG struct {
	// box is the viewBox, or the document's width and height when it has
	// none. It is the user space the operations are in.
	box svgBox
	// width and height are the document's intrinsic size.
	width, height Points
	// ops draw the document in its viewBox; a Color of 0 is currentColor.
	ops []DrawOp
}

// svgBox is a viewBox in user space.
type svgBox struct{ x, y, w, h float32 }

// Bounds on what one document may take to parse, since SVG files come from
// the network: nesting deeper than maxSVGDepth fails, and so does a
// document whose use elements draw more than maxSVGVisits elements or that
// records more than maxSVGOps operations.
const (
	maxSVGDepth  = 256
	maxSVGUses   = 16
	maxSVGVisits = 1 << 20
	maxSVGOps    = 1 << 21
)

var errSVGTooComplex = errors.New("svg: too complex to draw")

// ParseSVG parses an SVG document.
func ParseSVG(data []byte) (*SVG, error) {
	root, err := parseSVGTree(data)
	if err != nil {
		return nil, err
	}
	s := &SVG{}
	p := &svgParser{ids: map[string]*svgNode{}}
	p.index(root)
	if err := p.root(root, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Size returns the document's intrinsic size: its width and height, one of
// them and the viewBox's aspect ratio, or its viewBox's size.
func (s *SVG) Size() (w, h Points) {
	if s == nil {
		return 0, 0
	}
	return s.width, s.height
}

// Draw records the document's shapes into d, fitted and centered in box and
// painted in color: every currentColor becomes color, and a zero color is
// black. A non-uniform box keeps the document's aspect ratio.
func (s *SVG) Draw(d *Drawing, box Size, color Color) {
	if s == nil || d == nil {
		return
	}
	if color == 0 {
		color = RGB(0x000000)
	}
	vw, vh := s.box.w, s.box.h
	if vw <= 0 || vh <= 0 {
		return
	}
	scale := min(float32(box.Width)/vw, float32(box.Height)/vh)
	if scale <= 0 {
		return
	}
	ox := (float32(box.Width) - vw*scale) / 2
	oy := (float32(box.Height) - vh*scale) / 2
	d.Save().Translate(ox, oy).Scale(scale, scale).Translate(-s.box.x, -s.box.y)
	for _, op := range s.ops {
		switch op.Kind {
		case DrawFillColor, DrawStrokeColor:
			if op.Color == 0 {
				op.Color = color
			}
		}
		d.Ops = append(d.Ops, op)
	}
	d.Restore()
}

// svgNode is an element of the document.
type svgNode struct {
	// name is the element's local name, empty for elements outside the
	// SVG namespace, which are never drawn.
	name     string
	attrs    map[string]string
	children []*svgNode
	text     string // a style element's sheet
}

// parseSVGTree reads the document's elements, with each element's style
// sheet rules and style attribute folded into its attributes.
func parseSVGTree(data []byte) (*svgNode, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	// Exporters declare entities (Illustrator's xmlns="&ns_svg;") and
	// encodings Go does not read. Entities stay as written, never expanded,
	// and documents in other encodings read as their ASCII.
	dec.Strict = false
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	var root *svgNode
	var stack []*svgNode
	var sheets []string
	for root == nil || len(stack) > 0 {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) && root == nil {
			return nil, errors.New("svg: no svg element")
		}
		if err != nil {
			return nil, fmt.Errorf("svg: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			n := &svgNode{name: tok.Name.Local, attrs: svgAttrs(tok)}
			if space := tok.Name.Space; space != "" && space != "http://www.w3.org/2000/svg" && !strings.HasPrefix(space, "&") {
				n.name = ""
			}
			if root == nil {
				if tok.Name.Local != "svg" {
					return nil, fmt.Errorf("svg: root element is %q, not svg", tok.Name.Local)
				}
				root = n
			} else {
				if len(stack) >= maxSVGDepth {
					return nil, errors.New("svg: elements nested too deeply")
				}
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			n := stack[len(stack)-1]
			if n.name == "style" {
				sheets = append(sheets, n.text)
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 && stack[len(stack)-1].name == "style" {
				stack[len(stack)-1].text += string(tok)
			}
		}
	}
	rules := parseCSSRules(sheets)
	var style func(n *svgNode)
	style = func(n *svgNode) {
		applyCSSRules(n, rules)
		if v, ok := n.attrs["style"]; ok {
			for _, d := range parseCSSDeclarations(v) {
				n.attrs[d[0]] = d[1]
			}
		}
		for _, c := range n.children {
			style(c)
		}
	}
	style(root)
	return root, nil
}

// svgAttrs reads an element's attributes, keeping their case. Of the
// namespaced ones, only xlink:href is read, as href.
func svgAttrs(el xml.StartElement) map[string]string {
	a := make(map[string]string, len(el.Attr))
	for _, attr := range el.Attr {
		if attr.Name.Space != "" {
			if attr.Name.Local == "href" && attr.Name.Space != "xmlns" {
				if _, ok := a["href"]; !ok {
					a["href"] = attr.Value
				}
			}
			continue
		}
		a[attr.Name.Local] = attr.Value
	}
	return a
}

// svgPaint is a fill or a stroke: none, the current color, a fixed color or
// a reference to a gradient.
type svgPaint struct {
	kind  svgPaintKind
	color Color
	// ref is a paintURL's element id; fall and color are what paints
	// when it names no gradient.
	ref  string
	fall svgPaintKind
}

type svgPaintKind uint8

const (
	paintNone svgPaintKind = iota
	paintCurrent
	paintFixed
	paintURL
)

func fixedPaint(c Color) svgPaint { return svgPaint{kind: paintFixed, color: c} }

// svgStyle is what shapes inherit from the elements around them, with the
// transform and group opacity they are drawn with.
type svgStyle struct {
	fill, stroke svgPaint
	// color is what currentColor stands for: paintCurrent for the color
	// the document is drawn in, or a fixed color the color attribute set.
	color                      svgPaint
	fillOpacity, strokeOpacity float32
	width                      float32
	cap                        LineCap
	join                       LineJoin
	dash                       []float32
	dashOffset                 float32
	evenOdd, clipEvenOdd       bool
	hidden                     bool
	ctm                        svgMatrix
	opacity                    float32
}

func defaultSVGStyle() svgStyle {
	return svgStyle{
		fill:          fixedPaint(RGB(0x000000)),
		color:         svgPaint{kind: paintCurrent},
		fillOpacity:   1,
		strokeOpacity: 1,
		width:         1,
		ctm:           svgIdentity,
		opacity:       1,
	}
}

type svgParser struct {
	ids map[string]*svgNode
	// vw and vh are the viewBox's size, which percentages are of.
	vw, vh float32
	ops    []DrawOp
	// local is the shape being read, in its own coordinates.
	local []DrawOp
	// using holds the elements use elements are drawing, which a use
	// inside them may not draw again.
	using  map[*svgNode]bool
	visits int
}

func (p *svgParser) index(n *svgNode) {
	if id := strings.TrimSpace(n.attrs["id"]); id != "" {
		if _, ok := p.ids[id]; !ok {
			p.ids[id] = n
		}
	}
	for _, c := range n.children {
		p.index(c)
	}
}

// root reads the svg element's size and draws its children into s.
func (p *svgParser) root(el *svgNode, s *SVG) error {
	attrs := el.attrs
	hasBox := false
	if v, ok := attrs["viewBox"]; ok {
		f, err := svgNumbers(v, 4)
		if err != nil {
			return fmt.Errorf("svg: viewBox: %w", err)
		}
		s.box = svgBox{f[0], f[1], f[2], f[3]}
		if s.box.w < 0 || s.box.h < 0 {
			return errors.New("svg: negative viewBox size")
		}
		hasBox = s.box.w > 0 && s.box.h > 0
	}
	var w, h float32
	_, hasW := attrs["width"]
	_, hasH := attrs["height"]
	if hasW {
		n, err := svgLengthOf(attrs["width"], s.box.w)
		if err != nil {
			return fmt.Errorf("svg: width: %w", err)
		}
		w = n
	}
	if hasH {
		n, err := svgLengthOf(attrs["height"], s.box.h)
		if err != nil {
			return fmt.Errorf("svg: height: %w", err)
		}
		h = n
	}
	switch {
	case hasW && hasH:
	case hasW && hasBox:
		h = w * s.box.h / s.box.w
	case hasH && hasBox:
		w = h * s.box.w / s.box.h
	}
	if (w <= 0 || h <= 0) && hasBox {
		w, h = s.box.w, s.box.h
	}
	if w <= 0 || h <= 0 || math.IsInf(float64(w), 0) || math.IsInf(float64(h), 0) {
		return errors.New("svg: no size")
	}
	s.width, s.height = Points(w), Points(h)
	if !hasBox {
		s.box = svgBox{w: w, h: h}
	}
	p.vw, p.vh = s.box.w, s.box.h
	style := defaultSVGStyle()
	if err := p.element(el, style, true); err != nil {
		return err
	}
	s.ops = p.ops
	return nil
}

// drawable reports whether elements named name draw where they are.
func drawable(name string) bool {
	switch name {
	case "svg", "g", "a", "switch", "use",
		"path", "rect", "circle", "ellipse", "line", "polyline", "polygon":
		return true
	}
	return false
}

// element draws one element and its children.
func (p *svgParser) element(el *svgNode, style svgStyle, root bool) error {
	if !drawable(el.name) {
		return nil
	}
	if p.visits++; p.visits > maxSVGVisits || len(p.ops) > maxSVGOps {
		return errSVGTooComplex
	}
	a := el.attrs
	if strings.TrimSpace(a["display"]) == "none" {
		return nil
	}
	if err := style.apply(a); err != nil {
		return err
	}
	if v, ok := a["transform"]; ok {
		if m, ok := parseSVGTransform(v); ok {
			style.ctm = style.ctm.mul(m)
		}
	}
	if v, ok := a["opacity"]; ok {
		style.opacity *= svgOpacity(v)
	}
	if style.opacity <= 0 {
		return nil
	}
	if !root && el.name == "svg" {
		x, err := p.length(a, "x")
		if err != nil {
			return err
		}
		y, err := p.length(a, "y")
		if err != nil {
			return err
		}
		style.ctm = style.ctm.mul(svgTranslate(x, y))
	}
	if v, ok := a["clip-path"]; ok {
		clipped, visible, err := p.clip(v, style)
		if err != nil || !visible {
			return err
		}
		if clipped {
			defer p.op(DrawOp{Kind: DrawRestore})
		}
	}
	switch el.name {
	case "svg", "g", "a":
		for _, c := range el.children {
			if err := p.element(c, style, false); err != nil {
				return err
			}
		}
		return nil
	case "switch":
		// Every test a child could ask for passes: the first child draws.
		for _, c := range el.children {
			if drawable(c.name) {
				return p.element(c, style, false)
			}
		}
		return nil
	case "use":
		return p.use(el, style)
	}
	p.local = p.local[:0]
	if err := p.shape(el); err != nil {
		return err
	}
	return p.paint(style)
}

// shape reads a shape element's geometry into p.local.
func (p *svgParser) shape(el *svgNode) error {
	a := el.attrs
	switch el.name {
	case "path":
		if d, ok := a["d"]; ok {
			if err := p.path(d); err != nil {
				return fmt.Errorf("svg: path: %w", err)
			}
		}
	case "rect":
		return p.rect(a)
	case "circle":
		return p.circle(a)
	case "ellipse":
		return p.ellipse(a)
	case "line":
		return p.line(a)
	case "polyline", "polygon":
		return p.polygon(a, el.name == "polygon")
	}
	return nil
}

// use draws the element a use element refers to, moved by its x and y.
func (p *svgParser) use(el *svgNode, style svgStyle) error {
	target := p.ids[svgHref(el.attrs)]
	if target == nil || p.using[target] || len(p.using) >= maxSVGUses {
		return nil
	}
	x, err := p.length(el.attrs, "x")
	if err != nil {
		return err
	}
	y, err := p.length(el.attrs, "y")
	if err != nil {
		return err
	}
	style.ctm = style.ctm.mul(svgTranslate(x, y))
	if p.using == nil {
		p.using = map[*svgNode]bool{}
	}
	p.using[target] = true
	defer delete(p.using, target)
	if target.name != "symbol" {
		return p.element(target, style, false)
	}
	// A symbol draws as a group; its viewBox is not read.
	if strings.TrimSpace(target.attrs["display"]) == "none" {
		return nil
	}
	if err := style.apply(target.attrs); err != nil {
		return err
	}
	if v, ok := target.attrs["opacity"]; ok {
		style.opacity *= svgOpacity(v)
	}
	for _, c := range target.children {
		if err := p.element(c, style, false); err != nil {
			return err
		}
	}
	return nil
}

// clip starts clipping to the clip path v names, in a saved state the
// caller restores when clipped. A clip path without shapes hides what it
// clips: visible is false. Clip paths in objectBoundingBox units are not
// read.
func (p *svgParser) clip(v string, style svgStyle) (clipped, visible bool, err error) {
	id, ok := svgURL(v)
	if !ok {
		return false, true, nil
	}
	cp := p.ids[id]
	if cp == nil || cp.name != "clipPath" || strings.TrimSpace(cp.attrs["clipPathUnits"]) == "objectBoundingBox" {
		return false, true, nil
	}
	base := defaultSVGStyle()
	base.ctm = style.ctm
	if t, ok := cp.attrs["transform"]; ok {
		if m, ok := parseSVGTransform(t); ok {
			base.ctm = base.ctm.mul(m)
		}
	}
	if err := base.apply(cp.attrs); err != nil {
		return false, false, err
	}
	shapes := 0
	evenOdd := false
	for _, c := range cp.children {
		child := c
		cs := base
		if child.name == "use" {
			target := p.ids[svgHref(child.attrs)]
			if target == nil {
				continue
			}
			x, err := p.length(child.attrs, "x")
			if err != nil {
				return false, false, err
			}
			y, err := p.length(child.attrs, "y")
			if err != nil {
				return false, false, err
			}
			if err := cs.apply(child.attrs); err != nil {
				return false, false, err
			}
			if t, ok := child.attrs["transform"]; ok {
				if m, ok := parseSVGTransform(t); ok {
					cs.ctm = cs.ctm.mul(m)
				}
			}
			cs.ctm = cs.ctm.mul(svgTranslate(x, y))
			child = target
		}
		switch child.name {
		case "path", "rect", "circle", "ellipse", "line", "polyline", "polygon":
		default:
			continue
		}
		if strings.TrimSpace(child.attrs["display"]) == "none" {
			continue
		}
		if err := cs.apply(child.attrs); err != nil {
			return false, false, err
		}
		if t, ok := child.attrs["transform"]; ok {
			if m, ok := parseSVGTransform(t); ok {
				cs.ctm = cs.ctm.mul(m)
			}
		}
		p.local = p.local[:0]
		if err := p.shape(child); err != nil {
			return false, false, err
		}
		if len(p.local) == 0 {
			continue
		}
		if shapes == 0 {
			p.op(DrawOp{Kind: DrawSave})
		}
		p.emitPath(cs.ctm, shapes == 0)
		shapes++
		evenOdd = cs.clipEvenOdd
	}
	if shapes == 0 {
		return false, false, nil
	}
	// Several shapes clip to their union, which nonzero comes closest to.
	p.op(DrawOp{Kind: DrawClip, EvenOdd: evenOdd && shapes == 1})
	return true, true, nil
}

// paint records the shape in p.local with the fill and the stroke style
// asks for.
func (p *svgParser) paint(style svgStyle) error {
	if len(p.local) == 0 || style.hidden {
		return nil
	}
	fill, stroke := style.fill, style.stroke
	if style.width <= 0 {
		stroke = svgPaint{}
	}
	if fill.kind == paintNone && stroke.kind == paintNone {
		return nil
	}
	start := len(p.ops)
	p.emitPath(style.ctm, true)
	painted := false
	if ok, restore := p.setPaint(fill, style, style.fillOpacity, true); ok {
		p.op(DrawOp{Kind: DrawFill, EvenOdd: style.evenOdd})
		if restore {
			p.op(DrawOp{Kind: DrawRestore})
		}
		painted = true
	}
	if ok, restore := p.setPaint(stroke, style, style.strokeOpacity, false); ok {
		det := style.ctm.a*style.ctm.d - style.ctm.b*style.ctm.c
		width := style.width
		if style.ctm != svgIdentity {
			width = float32(float64(width) * math.Sqrt(math.Abs(det)))
		}
		p.op(DrawOp{Kind: DrawLineWidth, W: width})
		p.op(DrawOp{Kind: DrawLineCap, Cap: style.cap})
		p.op(DrawOp{Kind: DrawLineJoin, Join: style.join})
		dashed := len(style.dash) > 0
		if dashed {
			scale := width / style.width
			d := make(LineDash, len(style.dash))
			for i, l := range style.dash {
				d[i] = l * scale
			}
			p.op(DrawOp{Kind: DrawLineDash, Dash: &d})
			p.op(DrawOp{Kind: DrawLineDashOffset, W: style.dashOffset * scale})
		}
		p.op(DrawOp{Kind: DrawStroke})
		if dashed {
			p.op(DrawOp{Kind: DrawLineDash})
			p.op(DrawOp{Kind: DrawLineDashOffset})
		}
		if restore {
			p.op(DrawOp{Kind: DrawRestore})
		}
		painted = true
	}
	if !painted {
		p.ops = p.ops[:start]
	}
	return nil
}

// setPaint records the fill or stroke paint, at alpha times the group
// opacity. It reports false when nothing would show, and restore when it
// saved the state to lower the global alpha for currentColor, which the
// caller restores after painting.
func (p *svgParser) setPaint(paint svgPaint, style svgStyle, alpha float32, fill bool) (ok, restore bool) {
	alpha *= style.opacity
	colorKind, gradientKind := DrawStrokeColor, DrawStrokeGradient
	if fill {
		colorKind, gradientKind = DrawFillColor, DrawFillGradient
	}
	if alpha <= 0 {
		return false, false
	}
	if paint.kind == paintURL {
		if el := p.ids[paint.ref]; el != nil && (el.name == "linearGradient" || el.name == "radialGradient") {
			g, solid, ok := p.gradient(el, style.ctm)
			if !ok {
				return false, false
			}
			if g == nil {
				paint = fixedPaint(solid)
			} else {
				for i := range g.Stops {
					g.Stops[i].Color = svgAlpha(g.Stops[i].Color, alpha)
				}
				fallback := g.average()
				if fallback == 0 {
					fallback = 1 // transparent but not currentColor
				}
				p.op(DrawOp{Kind: colorKind, Color: fallback})
				p.op(DrawOp{Kind: gradientKind, Gradient: g})
				return true, false
			}
		} else {
			paint = svgPaint{kind: paint.fall, color: paint.color}
		}
	}
	if paint.kind == paintCurrent && style.color.kind == paintFixed {
		paint = style.color
	}
	switch paint.kind {
	case paintCurrent:
		if alpha < 1 {
			p.op(DrawOp{Kind: DrawSave})
			p.op(DrawOp{Kind: DrawAlpha, A: alpha})
			restore = true
		}
		p.op(DrawOp{Kind: colorKind})
		return true, restore
	case paintFixed:
		c := svgAlpha(paint.color, alpha)
		if c&0xff == 0 {
			return false, false
		}
		p.op(DrawOp{Kind: colorKind, Color: c})
		return true, false
	}
	return false, false
}

// svgAlpha multiplies c's alpha by alpha.
func svgAlpha(c Color, alpha float32) Color {
	if alpha >= 1 {
		return c
	}
	a := float32(c&0xff) * alpha
	return c&^0xff | Color(uint32(a+0.5))
}

func (p *svgParser) op(o DrawOp) { p.ops = append(p.ops, o) }

// begin starts a shape's path.
func (p *svgParser) begin() { p.local = append(p.local, DrawOp{Kind: DrawBeginPath}) }

// lop adds an operation to the shape being read.
func (p *svgParser) lop(o DrawOp) { p.local = append(p.local, o) }

// emitPath records the shape in p.local through m, starting a new path
// when begin is set. Rectangles and ellipses stay what they are under
// transforms that keep them axis-aligned and become curves under others.
func (p *svgParser) emitPath(m svgMatrix, begin bool) {
	identity := m == svgIdentity
	aligned := m.b == 0 && m.c == 0
	sx, sy := float32(math.Abs(m.a)), float32(math.Abs(m.d))
	for _, op := range p.local {
		if identity {
			if op.Kind != DrawBeginPath || begin {
				p.op(op)
			}
			continue
		}
		switch op.Kind {
		case DrawBeginPath:
			if begin {
				p.op(op)
			}
		case DrawMoveTo, DrawLineTo:
			op.X, op.Y = m.apply(op.X, op.Y)
			p.op(op)
		case DrawQuadTo:
			op.X1, op.Y1 = m.apply(op.X1, op.Y1)
			op.X, op.Y = m.apply(op.X, op.Y)
			p.op(op)
		case DrawCubicTo:
			op.X1, op.Y1 = m.apply(op.X1, op.Y1)
			op.X2, op.Y2 = m.apply(op.X2, op.Y2)
			op.X, op.Y = m.apply(op.X, op.Y)
			p.op(op)
		case DrawRect, DrawRoundRect:
			if aligned && (op.Kind == DrawRect || sx == sy) {
				x0, y0 := m.apply(op.X, op.Y)
				op.X, op.Y = min(x0, x0+float32(m.a)*op.W), min(y0, y0+float32(m.d)*op.H)
				op.W, op.H, op.R = op.W*sx, op.H*sy, op.R*sx
				p.op(op)
				continue
			}
			var path []DrawOp
			if op.Kind == DrawRect {
				path = []DrawOp{{Kind: DrawMoveTo, X: op.X, Y: op.Y},
					{Kind: DrawLineTo, X: op.X + op.W, Y: op.Y},
					{Kind: DrawLineTo, X: op.X + op.W, Y: op.Y + op.H},
					{Kind: DrawLineTo, X: op.X, Y: op.Y + op.H},
					{Kind: DrawClosePath}}
			} else {
				path = roundRectPath(nil, op.X, op.Y, op.W, op.H, op.R, op.R)
			}
			p.emitOps(path, m)
		case DrawEllipse:
			if aligned {
				op.X, op.Y = m.apply(op.X, op.Y)
				op.R, op.RY = op.R*sx, op.RY*sy
				p.op(op)
				continue
			}
			path := []DrawOp{{Kind: DrawMoveTo, X: op.X + op.R, Y: op.Y}}
			path = appendCorner(path, op.X, op.Y, op.R, op.RY, 0, 360)
			path = append(path, DrawOp{Kind: DrawClosePath})
			p.emitOps(path, m)
		default:
			p.op(op)
		}
	}
}

// emitOps records path operations through m.
func (p *svgParser) emitOps(ops []DrawOp, m svgMatrix) {
	saved := p.local
	p.local = ops
	p.emitPath(m, false)
	p.local = saved
}

// apply reads the presentation attributes on an element.
func (s *svgStyle) apply(a map[string]string) error {
	for name, value := range a {
		value = strings.TrimSpace(value)
		if value == "inherit" {
			continue
		}
		switch name {
		case "fill":
			v, ok, err := parseSVGPaint(value)
			if err != nil {
				return fmt.Errorf("svg: fill: %w", err)
			}
			if ok {
				s.fill = v
			}
		case "stroke":
			v, ok, err := parseSVGPaint(value)
			if err != nil {
				return fmt.Errorf("svg: stroke: %w", err)
			}
			if ok {
				s.stroke = v
			}
		case "color":
			if v, ok, _ := parseSVGPaint(value); ok && (v.kind == paintFixed || v.kind == paintCurrent) {
				s.color = v
			}
		case "fill-opacity":
			s.fillOpacity = svgOpacity(value)
		case "stroke-opacity":
			s.strokeOpacity = svgOpacity(value)
		case "stroke-width":
			n, err := svgLength(value)
			if err != nil {
				return fmt.Errorf("svg: stroke-width: %w", err)
			}
			s.width = n
		case "stroke-dasharray":
			s.dash = svgDashes(value)
		case "stroke-dashoffset":
			if n, err := svgLength(value); err == nil {
				s.dashOffset = n
			}
		case "stroke-linecap":
			switch value {
			case "butt":
				s.cap = CapButt
			case "round":
				s.cap = CapRound
			case "square":
				s.cap = CapSquare
			}
		case "stroke-linejoin":
			switch value {
			case "miter", "miter-clip", "arcs":
				s.join = JoinMiter
			case "round":
				s.join = JoinRound
			case "bevel":
				s.join = JoinBevel
			}
		case "fill-rule":
			s.evenOdd = value == "evenodd"
		case "clip-rule":
			s.clipEvenOdd = value == "evenodd"
		case "visibility":
			s.hidden = value == "hidden" || value == "collapse"
		}
	}
	return nil
}

// svgDashes reads stroke-dasharray, an odd list repeated, none for a
// solid line.
func svgDashes(v string) []float32 {
	if strings.TrimSpace(v) == "none" {
		return nil
	}
	var dash []float32
	total := float32(0)
	for _, f := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
		n, err := svgLength(f)
		if err != nil || n < 0 {
			return nil
		}
		dash = append(dash, n)
		total += n
	}
	if total == 0 {
		return nil
	}
	if len(dash)%2 != 0 {
		dash = append(dash, dash...)
	}
	return dash
}

// svgOpacity reads an opacity, a number or a percentage from 0 to 1; one
// it cannot read is opaque.
func svgOpacity(v string) float32 {
	v = strings.TrimSpace(v)
	scale := float32(1)
	if strings.HasSuffix(v, "%") {
		v, scale = strings.TrimSpace(v[:len(v)-1]), 0.01
	}
	n, err := strconv.ParseFloat(v, 32)
	if err != nil || n != n {
		return 1
	}
	return min(max(float32(n)*scale, 0), 1)
}

// length reads a coordinate or length attribute, resolving percentages
// against the viewBox: its width for x coordinates and widths, its height
// for y coordinates and heights, and its normalized diagonal otherwise.
func (p *svgParser) length(a map[string]string, name string) (float32, error) {
	v, ok := a[name]
	if !ok || strings.TrimSpace(v) == "" {
		return 0, nil
	}
	var ref float32
	switch name {
	case "x", "x1", "x2", "cx", "rx", "width", "fx":
		ref = p.vw
	case "y", "y1", "y2", "cy", "ry", "height", "fy":
		ref = p.vh
	default:
		ref = float32(math.Sqrt(float64(p.vw*p.vw+p.vh*p.vh) / 2))
	}
	n, err := svgLengthOf(v, ref)
	if err != nil {
		return 0, fmt.Errorf("svg: %s: %w", name, err)
	}
	return n, nil
}

// svgLength reads a length in user units. Absolute units convert at 96
// per inch, em and ex take a 16-unit font, and a percentage reads as its
// number.
func svgLength(v string) (float32, error) { return svgLengthOf(v, 100) }

// svgLengthOf reads a length as svgLength does, with percentages of ref.
func svgLengthOf(v string, ref float32) (float32, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, nil
	}
	sc := svgScanner{s: v}
	n, ok := sc.number()
	if !ok {
		return 0, fmt.Errorf("bad number %q", v)
	}
	switch strings.TrimSpace(v[sc.i:]) {
	case "%":
		n *= ref / 100
	case "pt":
		n *= 96.0 / 72
	case "pc":
		n *= 16
	case "in":
		n *= 96
	case "cm":
		n *= 96 / 2.54
	case "mm":
		n *= 96 / 25.4
	case "em", "rem":
		n *= 16
	case "ex":
		n *= 8
	}
	return n, nil
}

// svgNumbers reads n numbers separated by space or commas, or every number
// when n is negative.
func svgNumbers(v string, n int) ([]float32, error) {
	sc := svgScanner{s: v}
	out := make([]float32, 0, max(n, 0))
	for n < 0 || len(out) < n {
		sc.skip()
		if sc.eof() {
			break
		}
		f, ok := sc.number()
		if !ok {
			return nil, fmt.Errorf("want %d numbers in %q", n, v)
		}
		out = append(out, f)
	}
	if n >= 0 && len(out) != n {
		return nil, fmt.Errorf("want %d numbers in %q", n, v)
	}
	return out, nil
}

// svgScanner reads numbers, flags and commands from an attribute value.
type svgScanner struct {
	s string
	i int
}

func (s *svgScanner) eof() bool { return s.i >= len(s.s) }

func (s *svgScanner) peek() byte {
	if s.eof() {
		return 0
	}
	return s.s[s.i]
}

func (s *svgScanner) next() byte {
	b := s.s[s.i]
	s.i++
	return b
}

func (s *svgScanner) skip() {
	for !s.eof() {
		switch s.s[s.i] {
		case ' ', '\t', '\n', '\r', '\f', ',':
			s.i++
		default:
			return
		}
	}
}

// numberStart reports whether a number starts here, after separators.
func (s *svgScanner) numberStart() bool {
	s.skip()
	if s.eof() {
		return false
	}
	b := s.s[s.i]
	return b >= '0' && b <= '9' || b == '.' || b == '+' || b == '-'
}

func (s *svgScanner) number() (float32, bool) {
	s.skip()
	start := s.i
	if s.numberStart() {
		if b := s.s[s.i]; b == '+' || b == '-' {
			s.i++
		}
	}
	digits := false
	for !s.eof() && s.s[s.i] >= '0' && s.s[s.i] <= '9' {
		s.i++
		digits = true
	}
	if !s.eof() && s.s[s.i] == '.' {
		s.i++
		for !s.eof() && s.s[s.i] >= '0' && s.s[s.i] <= '9' {
			s.i++
			digits = true
		}
	}
	if !digits {
		s.i = start
		return 0, false
	}
	if !s.eof() && (s.s[s.i] == 'e' || s.s[s.i] == 'E') {
		j := s.i + 1
		if j < len(s.s) && (s.s[j] == '+' || s.s[j] == '-') {
			j++
		}
		if j < len(s.s) && s.s[j] >= '0' && s.s[j] <= '9' {
			for j < len(s.s) && s.s[j] >= '0' && s.s[j] <= '9' {
				j++
			}
			s.i = j
		}
	}
	f, err := strconv.ParseFloat(s.s[start:s.i], 32)
	if err != nil {
		s.i = start
		return 0, false
	}
	return float32(f), true
}

// flag reads a single 0 or 1, which may run into the next number.
func (s *svgScanner) flag() (bool, bool) {
	s.skip()
	if s.eof() {
		return false, false
	}
	switch s.s[s.i] {
	case '0':
		s.i++
		return false, true
	case '1':
		s.i++
		return true, true
	}
	return false, false
}

func isPathCommand(b byte) bool {
	switch b {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's',
		'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
		return true
	}
	return false
}
