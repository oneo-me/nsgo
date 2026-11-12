package core

import (
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Icon draws an SVG — an icon, a logo, an illustration — in the text color
// it inherits, so a theme colors it as it colors text: its shapes scale to
// the icon's size and stay sharp.
//
// An icon is as high as the inherited font size, keeping the SVG's aspect
// ratio, unless Style sets a size. Its drawing is fitted and centered in
// the icon's box, whose aspect ratio a non-square box does not change.
//
//	core.Icon{SVG: lucide.Search(), Style: core.Style{FontSize: 20}, Label: "Search"}
//
// An icon is a decoration assistive technology does not see unless Label
// names it.
type Icon struct {
	SVG       *SVG
	SVGFunc   func() *SVG
	Style     Style
	StyleFunc func() Style
	OnPress   func(PressEvent)
	OnLayout  func(Bounds)
	Ref       *Ref
	Label     string
	TestID    string
	// documentSize measures it at its document's size (SVGView).
	documentSize bool
}

// Render implements Node.
func (i Icon) Render(*Scope) Node { return i }

func (i Icon) mountIn(t *Tree, cx *Scope, parent *node) { t.mountIcon(cx, parent, i) }

// SVGView draws an SVG document as authored, as an <svg> written inline in
// a web page: in its own colors, sized by its width and height, else its
// viewBox, where Style sets no size. Its shapes are vectors, sharp at any
// size, and currentColor is the text color it inherits.
type SVGView struct {
	SVG       *SVG
	SVGFunc   func() *SVG
	Style     Style
	StyleFunc func() Style
	OnPress   func(PressEvent)
	OnLayout  func(Bounds)
	Ref       *Ref
	Label     string
	TestID    string
}

// Render implements Node.
func (v SVGView) Render(*Scope) Node {
	return Icon{SVG: v.SVG, SVGFunc: v.SVGFunc, Style: v.Style, StyleFunc: v.StyleFunc, OnPress: v.OnPress,
		OnLayout: v.OnLayout, Ref: v.Ref, Label: v.Label, TestID: v.TestID, documentSize: true}
}

// iconNode is an icon's drawing state: the document, the color it inherits
// and the text it inherits it from.
type iconNode struct {
	svg   *Signal[*SVG]
	color *Signal[Color]
	text  driver.TextStyle
	// documentSize is Icon's.
	documentSize bool
}

// naturalSize is what an icon measures without a style size: as high as its
// text, as wide as the SVG's aspect ratio makes it.
func (in *iconNode) naturalSize() Size {
	if in.documentSize {
		w, h := in.svg.GetUntracked().Size()
		return Size{Width: w, Height: h}
	}
	h := in.text.Font.Size
	if h <= 0 {
		h = DefaultFontSize
	}
	w := h
	if sw, sh := in.svg.GetUntracked().Size(); sh > 0 && sw > 0 {
		w = h * sw / sh
	}
	return Size{Width: w, Height: h}
}

func (t *Tree) mountIcon(cx *Scope, parent *node, v Icon) {
	// Icons draw through the canvas protocol: the engine records the shapes
	// and a platform plays them, as canvases do.
	n := t.newNode(driver.KindCanvas, parent, cx)
	t.bindStyle(cx, n, v.Style, v.StyleFunc)
	t.setRef(n, v.Ref)
	n.h = handlers{press: v.OnPress, layout: v.OnLayout}
	// An icon is a decoration until Label names it, so assistive technology
	// skips an unlabeled one instead of announcing an empty image.
	role := RoleNone
	if v.Label != "" {
		role = RoleImage
	}
	n.props = driver.ViewProps{Role: role, Label: v.Label, TestID: v.TestID}
	in := &iconNode{svg: NewSignal(cx, v.SVG), color: NewSignal(cx, Color(0)), documentSize: v.documentSize}
	cn := &canvasNode{size: NewSignal(cx, Size{})}
	n.icon, n.canvas = in, cn
	t.canvases = append(t.canvases, n)
	if v.SVGFunc != nil {
		cx.renderEffect(func() {
			svg := v.SVGFunc()
			if svg == in.svg.Get() {
				return
			}
			oldW, oldH := in.svg.GetUntracked().Size()
			in.svg.Set(svg)
			if w, h := svg.Size(); w != oldW || h != oldH {
				t.relayout(n)
			}
		})
	}
	cx.renderEffect(func() {
		size := cn.size.Get()
		color := in.color.Get()
		svg := in.svg.Get()
		if size == (Size{}) {
			return
		}
		d := &Drawing{}
		svg.Draw(d, size, color)
		cn.drawing, cn.fresh = d, true
		t.touch(n)
	})
}

// MustParseSVG is ParseSVG for documents that are part of the program, such
// as embedded icons: it panics when one is in error.
func MustParseSVG(data []byte) *SVG {
	s, err := ParseSVG(data)
	if err != nil {
		panic(err)
	}
	return s
}
