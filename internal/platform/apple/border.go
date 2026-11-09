//go:build darwin

package apple

import (
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Borders whose sides differ, which a CALayer's own border cannot draw:
// the ring between the outer rounded rectangle and the inner one, inset
// by each side's width, is filled by a shape sublayer under the
// children, one per side color, each masked to its side's wedge as CSS
// splits the corners.

var (
	cgPathCreateMutable    func() uintptr
	cgPathMoveToPoint      func(path, m uintptr, x, y float64)
	cgPathAddLineToPoint   func(path, m uintptr, x, y float64)
	cgPathAddCurveToPoint  func(path, m uintptr, x1, y1, x2, y2, x, y float64)
	cgPathCloseSubpath     func(path uintptr)
	selSetPath, selSetFill = sel("setPath:"), sel("setFillColor:")
)

func loadBorders(cg uintptr) {
	reg := purego.RegisterLibFunc
	reg(&cgPathCreateMutable, cg, "CGPathCreateMutable")
	reg(&cgPathMoveToPoint, cg, "CGPathMoveToPoint")
	reg(&cgPathAddLineToPoint, cg, "CGPathAddLineToPoint")
	reg(&cgPathAddCurveToPoint, cg, "CGPathAddCurveToPoint")
	reg(&cgPathCloseSubpath, cg, "CGPathCloseSubpath")
}

// ring is the per-side border of one layer.
type ring struct {
	widths     driver.Edges
	colors     driver.EdgeColors
	corners    driver.ViewStyle // its corners: CornerRadius, Corners, ContinuousCorners
	continuous bool
	// sides are the shape layers: only [0] when every side has one color.
	sides [4]objc.ID
}

// rings are the layers drawing per-side borders.
var rings = map[objc.ID]*ring{}

// setBorders draws vs's per-side borders on layer, or removes them when
// its border is uniform. tr animates them when it lists borders.
func setBorders(layer objc.ID, vs *driver.ViewStyle, tr *driver.Transition) {
	r := rings[layer]
	if vs.Borders == (driver.Edges{}) {
		if r != nil {
			for i := range r.sides {
				r.remove(i)
			}
			delete(rings, layer)
		}
		return
	}
	if r == nil {
		r = &ring{}
		rings[layer] = r
	}
	r.widths, r.colors, r.continuous = vs.Borders, vs.BorderColors, vs.ContinuousCorners
	r.corners = driver.ViewStyle{CornerRadius: vs.CornerRadius, Corners: vs.Corners}
	if tr.Duration <= 0 || (tr.Properties != 0 && tr.Properties&driver.TransitionBorder == 0) {
		tr = nil
	}
	r.layout(layer, tr)
}

// LayoutBorders fits a layer's per-side borders to its bounds after its
// frame changed.
func LayoutBorders(layer objc.ID) {
	if r := rings[layer]; r != nil {
		r.layout(layer, nil)
	}
}

// ForgetBorders forgets the borders of a layer that is going away.
func ForgetBorders(layer objc.ID) { delete(rings, layer) }

// remove takes away side i's layer.
func (r *ring) remove(i int) {
	if r.sides[i] != 0 {
		r.sides[i].Send(sel("removeFromSuperlayer"))
		r.sides[i].Send(selRelease)
		r.sides[i] = 0
	}
}

func (r *ring) layout(layer objc.ID, tr *driver.Transition) {
	bounds := objc.Send[CGRect](layer, selBounds)
	w, h := bounds.Size.Width, bounds.Size.Height
	b := r.widths
	top, right, bottom, left := float64(b.Top), float64(b.Right), float64(b.Bottom), float64(b.Left)
	c := r.colors
	colors := [4]driver.Color{c.Top, c.Right, c.Bottom, c.Left}
	widths := [4]float64{top, right, bottom, left}
	// One layer draws a ring of one color around every side; sides
	// without a border would leave seams at their corners.
	one := c.Top == c.Right && c.Top == c.Bottom && c.Top == c.Left && min(top, right, bottom, left) > 0
	WithoutAnimation(func() {
		ring := r.ringPath(w, h)
		defer Release(ring)
		for i := range r.sides {
			show := widths[i] > 0 && colors[i] != 0
			if one {
				show = i == 0 && colors[0] != 0
			}
			if !show {
				r.remove(i)
				continue
			}
			side := r.sides[i]
			fresh := side == 0
			if fresh {
				side = objc.ID(Class("CAShapeLayer")).Send(selNew)
				side.Send(sel("setFillRule:"), NSString("even-odd"))
				layer.Send(sel("insertSublayer:atIndex:"), side, uint32(0))
				r.sides[i] = side
			}
			side.Send(selSetFrame, bounds)
			color := CGColor(colors[i])
			animate(side, "fillColor", tr, fresh, func() { side.Send(selSetFill, color) })
			Release(color)
			animate(side, "path", tr, fresh, func() { side.Send(selSetPath, ring) })
			mask := side.Send(sel("mask"))
			if one {
				if mask != 0 {
					side.Send(selSetMask, objc.ID(0))
				}
				continue
			}
			if mask == 0 {
				mask = objc.ID(Class("CAShapeLayer")).Send(selNew)
				side.Send(selSetMask, mask)
				mask.Send(selRelease)
			}
			mask.Send(selSetFrame, CGRect{Size: bounds.Size})
			wedge := r.wedgePath(i, w, h)
			animate(mask, "path", tr, fresh, func() { mask.Send(selSetPath, wedge) })
			Release(wedge)
		}
	})
}

// animate runs set, animating layer's key from the value shown when tr
// asks for it.
func animate(layer objc.ID, key string, tr *driver.Transition, fresh bool, set func()) {
	if tr == nil || fresh {
		set()
		return
	}
	shown := layer.Send(sel("presentationLayer"))
	if shown == 0 {
		shown = layer
	}
	from := shown.Send(selValueForKey, NSString(key))
	set()
	if from == 0 {
		return
	}
	anim := objc.ID(Class("CABasicAnimation")).Send(sel("animationWithKeyPath:"), NSString(key))
	anim.Send(sel("setFromValue:"), from)
	anim.Send(sel("setDuration:"), tr.Duration.Seconds())
	anim.Send(sel("setTimingFunction:"), timing(tr.Easing))
	if tr.Delay > 0 {
		anim.Send(sel("setBeginTime:"), caCurrentMediaTime()+tr.Delay.Seconds())
		anim.Send(sel("setFillMode:"), NSString("backwards"))
	}
	layer.Send(selAddAnimation, anim, NSString("nsgo."+key))
}

// ringPath returns a new path of the area between the outer edge and the
// inner edge, for an even-odd fill. The inner corners are elliptical
// where the sides around them differ, as CSS draws them.
func (r *ring) ringPath(w, h float64) uintptr {
	radii := r.corners.Radii(driver.Size{Width: driver.Points(w), Height: driver.Points(h)})
	segs := driver.RoundedRect(nil, 0, 0, w, h, radii, r.continuous, false)
	b := r.widths
	top, right, bottom, left := float64(b.Top), float64(b.Right), float64(b.Bottom), float64(b.Left)
	if iw, ih := w-left-right, h-top-bottom; iw > 0 && ih > 0 {
		segs = driver.EllipticRect(segs, left, top, iw, ih, driver.InnerRadii(radii, b), r.continuous, false)
	}
	return cgPath(segs)
}

// wedgePath returns a new path of side i's share of the ring (top, right,
// bottom, left): from its outer corners along the lines through the
// inner corners, past the rounded inner corners.
func (r *ring) wedgePath(i int, w, h float64) uintptr {
	b := r.widths
	top, right, bottom, left := float64(b.Top), float64(b.Right), float64(b.Bottom), float64(b.Left)
	radii := r.corners.Radii(driver.Size{Width: driver.Points(w), Height: driver.Points(h)}).Array()
	rad := float64(max(radii[0], radii[1], radii[2], radii[3]))
	if r.continuous {
		rad *= 1 + driver.Smoothing
	}
	reach := func(ox, oy, dx, dy float64) [2]float64 {
		m := max(abs(dx), abs(dy))
		if m == 0 {
			return [2]float64{ox, oy}
		}
		s := 1 + rad/m
		return [2]float64{ox + dx*s, oy + dy*s}
	}
	tl, tr := reach(0, 0, left, top), reach(w, 0, -right, top)
	br, bl := reach(w, h, -right, -bottom), reach(0, h, left, -bottom)
	var pts [4][2]float64
	switch i {
	case 0:
		pts = [4][2]float64{{0, 0}, {w, 0}, tr, tl}
	case 1:
		pts = [4][2]float64{{w, 0}, {w, h}, br, tr}
	case 2:
		pts = [4][2]float64{{w, h}, {0, h}, bl, br}
	default:
		pts = [4][2]float64{{0, h}, {0, 0}, tl, bl}
	}
	path := cgPathCreateMutable()
	for j, p := range pts {
		if j == 0 {
			cgPathMoveToPoint(path, 0, p[0], p[1])
		} else {
			cgPathAddLineToPoint(path, 0, p[0], p[1])
		}
	}
	cgPathCloseSubpath(path)
	return path
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
