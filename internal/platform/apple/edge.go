package apple

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

var selSetHidden = sel("setHidden:")

// ScrollEdge is what macOS and iOS draw of a scroll edge effect over their
// blur, alike: the tint of the background the content scrolls over and a
// hard edge's hairline. Its layers are in the band's own space; the platform
// puts them over its blur.
type ScrollEdge struct {
	Tint, Line objc.ID
}

// Paint lays the layers out for edge e at side s, in a band of w by h
// points, y growing down.
func (p *ScrollEdge) Paint(e *driver.ScrollEdge, s driver.Side, w, h float64) {
	if p.Tint == 0 {
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
}

// Release releases the layers.
func (p *ScrollEdge) Release() {
	for _, l := range []*objc.ID{&p.Tint, &p.Line} {
		if *l != 0 {
			l.Send(selRemoveFromSuper)
			l.Send(selRelease)
			*l = 0
		}
	}
}

// variableBlurSpread is how much wider Core Animation's variable blur
// spreads than its Gaussian blur, whose radius is the standard deviation,
// of the same radius: measured on macOS 27.
const variableBlurSpread = 2

// EdgeBlur returns a CABackdropLayer's filters for effect d at side s: a blur
// growing toward a soft side, or an even one.
func EdgeBlur(s driver.Side, d driver.EdgeEffect) objc.ID {
	number := func(f float64) objc.ID {
		return objc.ID(objc.GetClass("NSNumber")).Send(sel("numberWithDouble:"), f)
	}
	filter := objc.ID(objc.GetClass("CAFilter"))
	set := func(f objc.ID, key string, value objc.ID) {
		f.Send(sel("setValue:forKey:"), value, NSString(key))
	}
	var f objc.ID
	if d == driver.EdgeEffectSoft {
		f = filter.Send(sel("filterWithType:"), NSString("variableBlur"))
		set(f, "inputRadius", number(float64(driver.SoftEdgeBlur/variableBlurSpread)))
		set(f, "inputMaskImage", fadeImage(s))
	} else {
		f = filter.Send(sel("filterWithType:"), NSString("gaussianBlur"))
		set(f, "inputRadius", number(float64(driver.HardEdgeBlur)))
	}
	// What is past the band's edges blurs in as if it went on, as
	// AppKit's edges have it, rather than as clear.
	set(f, "inputNormalizeEdges", objc.ID(objc.GetClass("NSNumber")).Send(sel("numberWithBool:"), true))
	return objc.ID(objc.GetClass("NSArray")).Send(sel("arrayWithObject:"), f)
}

// fadeImages are the soft edges' masks, by side, made on first use.
var fadeImages [4]objc.ID

// fadeImage returns the soft edge's mask at side s, which the variable
// blur stretches over the band.
func fadeImage(s driver.Side) objc.ID {
	if fadeImages[s] == 0 {
		fadeImages[s] = objc.ID(FadeImage(s))
	}
	return fadeImages[s]
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
