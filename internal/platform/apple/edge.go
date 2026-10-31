package apple

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

var selSetHidden = sel("setHidden:")

// ScrollEdge is what macOS, and iOS before 26, draw of a scroll edge
// effect over their blur, alike: the tint of the background the content
// scrolls over, a hard edge's hairline and, for iOS, the mask that fades
// a soft edge's blur in toward the side. Its layers are in the band's own
// space; the platform puts them over its blur.
type ScrollEdge struct {
	Mask, Tint, Line objc.ID
}

// Paint lays the layers out for edge e at side s, in a band of w by h
// points, y growing down. A hard edge's blur has no mask: it reports
// whether the blur takes Mask.
func (p *ScrollEdge) Paint(e *driver.ScrollEdge, s driver.Side, w, h float64) (masked bool) {
	if p.Mask == 0 {
		p.Mask = objc.ID(objc.GetClass("CAGradientLayer")).Send(selNew)
		p.Tint = objc.ID(objc.GetClass("CAGradientLayer")).Send(selNew)
		p.Line = objc.ID(objc.GetClass("CALayer")).Send(selNew)
	}
	x0, y0, x1, y1 := driver.EdgeLine(s)
	gradient := func(g objc.ID, stops ...driver.GradientStop) {
		g.Send(selSetFrame, CGRect{Size: CGSize{Width: w, Height: h}})
		paintGradient(g, &driver.GradientPaint{X0: x0, Y0: y0, X1: x1, Y1: y1, Stops: stops}, w, h)
	}
	soft := e.Drawn() == driver.EdgeEffectSoft
	if soft {
		gradient(p.Mask, driver.GradientStop{Color: driver.RGB(0)}, driver.GradientStop{Color: driver.RGBA(0, 0), Offset: 1})
		gradient(p.Tint, e.SoftTint(driver.EdgeTint)...)
	} else {
		tint := e.Tint.Faded(driver.EdgeTint)
		gradient(p.Tint, driver.GradientStop{Color: tint}, driver.GradientStop{Color: tint, Offset: 1})
	}
	line := driver.HairlineRect(s, driver.Size{Width: driver.Points(w), Height: driver.Points(h)})
	p.Line.Send(selSetFrame, CGRect{Origin: CGPoint{X: float64(line.Origin.X), Y: float64(line.Origin.Y)},
		Size: CGSize{Width: float64(line.Size.Width), Height: float64(line.Size.Height)}})
	c := CGColor(e.Line)
	p.Line.Send(selSetBackgroundColor, c)
	Release(c)
	p.Line.Send(selSetHidden, soft)
	return soft
}

// Release releases the layers.
func (p *ScrollEdge) Release() {
	for _, l := range []*objc.ID{&p.Mask, &p.Tint, &p.Line} {
		if *l != 0 {
			l.Send(selRemoveFromSuper)
			l.Send(selRelease)
			*l = 0
		}
	}
}

// FadeImage returns a new CGImage of a soft edge's fade at side s, 64
// pixels along the band and 1 across: opaque at the side, clear at the
// other end, for a variable blur to stretch over its band as its mask.
// The caller releases it.
func FadeImage(s driver.Side) uintptr {
	const n = 64
	pix := make([]byte, 4*n)
	for i := range n {
		// The image's pixels run from its top or left.
		a := (float64(i) + 0.5) / n
		if s == driver.SideTop || s == driver.SideLeft {
			a = 1 - a
		}
		v := byte(a*255 + 0.5)
		pix[4*i], pix[4*i+1], pix[4*i+2], pix[4*i+3] = v, v, v, v
	}
	w, h := 1, n
	if s == driver.SideLeft || s == driver.SideRight {
		w, h = n, 1
	}
	data := cfDataCreate(0, &pix[0], len(pix))
	defer Release(data)
	provider := cgDataProviderCreateWithData(data)
	defer Release(provider)
	const premultipliedLast = 1
	return cgImageCreate(w, h, 8, 32, w*4, srgb, premultipliedLast, provider, 0, true, 0)
}
