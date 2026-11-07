//go:build darwin

package apple

import (
	"math"
	"runtime"
	"slices"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

// What a CALayer's own properties cannot draw goes into sublayers made on
// demand, as NSH's decor does: corners of different radii take a shape
// layer for the fill and a mask for clipping, while one radius, circular
// or continuous, is the layer's own corner radius and curve; dashed
// borders and outlines a stroked shape; each shadow a layer of its own
// with a shadow path; each gradient a CAGradientLayer. Fills, gradients
// and inset shadows sit below the view's content (a content layer's
// zPosition is -1) and its children, outer shadows below them, borders
// above the content and outlines above everything. A layer that clips its
// children clips these too, as it clips its own shadow.
//
// Masks are what the window server pays for on every frame a view moves,
// as NSH found: masked shadows on the gallery's cards and tiles dropped
// most frames of a scroll. So an opaque background or gradient, drawn
// above the outer shadows, hides them inside the box and they need no
// mask (an opaque background moves to a fill layer for that); otherwise
// they are masked to outside the box. Inset shadows go in a holder
// clipped to the padding box by its rounded bounds, masked only when its
// corners differ.

// Z are those orders scaled down, so a child perspective (a sublayer
// transform that projects by depth) leaves these layers their size.
func Z(order float64) float64 { return order * 1e-5 }

const (
	zShadow  = -1030
	zFill    = -1020
	zInset   = -1000
	zStroke  = 0
	zOutline = 1000
)

// decor is the extra layers of one view's layer.
type decor struct {
	vs driver.ViewStyle
	// shaped draws the fill and the clip with shapes: the corners differ
	// or ease into the edges.
	shaped bool
	fill   objc.ID
	// covered says an opaque fill or gradient hides the outer shadows
	// inside the box; cover is the fill layer of an unshaped one.
	covered bool
	cover   objc.ID
	// covers is the color cover shows; recolor is the transition that
	// fades it to a new one, while a new style draws.
	covers  driver.Color
	recolor *driver.Transition
	// insets holds the inset shadows, clipped to the padding box.
	insets objc.ID
	// clip and mask are the layer's masks, which d owns: the clip shape,
	// and a gradient mask over it.
	clip      objc.ID
	stroke    objc.ID
	outline   objc.ID
	shadows   []objc.ID
	gradients []objc.ID
	mask      objc.ID
	// keyframes is the animation the layer runs, made for a box of
	// keyframedAt.
	keyframes   driver.KeyframeAnimation
	keyframed   bool
	keyframedAt CGSize
	// at is the bounds origin the layers sit at: a scroll view's is its
	// offset, and the layers stay with the box, not the content.
	at CGPoint
}

// decors are the layers with decor.
var decors = map[objc.ID]*decor{}

var (
	selSetStrokeColor    = sel("setStrokeColor:")
	selSetLineWidth      = sel("setLineWidth:")
	selSetLineDash       = sel("setLineDashPattern:")
	selSetLineCap        = sel("setLineCap:")
	selSetZPos           = sel("setZPosition:")
	selPosition          = sel("position")
	selSetPosition       = sel("setPosition:")
	selSetShadowPath     = sel("setShadowPath:")
	selRemoveFromSuper   = sel("removeFromSuperlayer")
	selSuperlayer        = sel("superlayer")
	selInsertSublayer    = sel("insertSublayer:atIndex:")
	selAddSublayer       = sel("addSublayer:")
	selSetCornerCurve    = sel("setCornerCurve:")
	selSetMaskedCorners  = sel("setMaskedCorners:")
	selSetColors         = sel("setColors:")
	selSetLocations      = sel("setLocations:")
	selSetStartPoint     = sel("setStartPoint:")
	selSetEndPoint       = sel("setEndPoint:")
	selSetType           = sel("setType:")
	selAddObject         = sel("addObject:")
	selRemoveAnimation   = sel("removeAnimationForKey:")
	selSetDoubleSided    = sel("setDoubleSided:")
	selSetSublayerXform  = sel("setSublayerTransform:")
	selSetCompositing    = sel("setCompositingFilter:")
	selSetFilters        = sel("setFilters:")
	selFilterWithName    = sel("filterWithName:")
	selSetValueForKey    = sel("setValue:forKey:")
	selVectorWithXYZW    = sel("vectorWithX:Y:Z:W:")
	selAnimationWithPath = sel("animationWithKeyPath:")
)

// cgPath returns a new CGPath of segments; the caller releases it.
func cgPath(segments []driver.Segment) uintptr {
	path := cgPathCreateMutable()
	appendPath(path, segments)
	return path
}

func appendPath(path uintptr, segments []driver.Segment) {
	for _, s := range segments {
		switch s.Kind {
		case driver.SegmentMove:
			cgPathMoveToPoint(path, 0, s.P[0][0], s.P[0][1])
		case driver.SegmentLine:
			cgPathAddLineToPoint(path, 0, s.P[0][0], s.P[0][1])
		case driver.SegmentCubic:
			cgPathAddCurveToPoint(path, 0, s.P[0][0], s.P[0][1], s.P[1][0], s.P[1][1], s.P[2][0], s.P[2][1])
		case driver.SegmentClose:
			cgPathCloseSubpath(path)
		}
	}
}

// boxPath returns a new path of a box at x, y of size w, h with corners;
// the caller releases it.
func boxPath(x, y, w, h float64, corners driver.Corners, continuous bool) uintptr {
	return cgPath(driver.RoundedRect(nil, x, y, w, h, corners, continuous, false))
}

// newLayer returns a new layer of class, at z under host's other layers
// of higher z; host keeps it.
func newLayer(host objc.ID, class string, z float64) objc.ID {
	l := objc.ID(objc.GetClass(class)).Send(selNew)
	l.Send(selSetZPos, Z(z))
	host.Send(selInsertSublayer, l, uint32(0))
	l.Send(selRelease)
	return l
}

func drop(l *objc.ID) {
	if *l != 0 {
		l.Send(selRemoveFromSuper)
		*l = 0
	}
}

func dropAll(ls *[]objc.ID) {
	for i := range *ls {
		drop(&(*ls)[i])
	}
	*ls = (*ls)[:0]
}

// numbers returns a new autoreleased NSArray of NSNumbers.
func numbers(vs ...float64) objc.ID {
	a := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	for _, v := range vs {
		a.Send(selAddObject, objc.ID(objc.GetClass("NSNumber")).Send(selNumberWithDouble, v))
	}
	return a
}

// needsShape reports whether vs's corners need shape layers: radii that
// differ. One radius, circular or continuous, is the layer's own.
func needsShape(vs *driver.ViewStyle, radii driver.Corners) bool {
	if !vs.Rounded() {
		return false
	}
	var r driver.Points
	for _, c := range radii.Array() {
		if c > 0 {
			if r > 0 && c != r {
				return true
			}
			r = c
		}
	}
	return false
}

// applyDecor draws what vs needs beyond the layer's own properties, for
// the layer's bounds.
func applyDecor(layer objc.ID, vs *driver.ViewStyle) {
	d := decors[layer]
	e := vs.Effects
	if d == nil {
		plain := !vs.Rounded() && vs.BorderStyle == 0 && vs.Outline == (driver.Outline{}) && e == nil &&
			vs.Animation == (driver.KeyframeAnimation{}) && vs.ChildPerspective == 0 && !vs.BackfaceHidden &&
			vs.BlendMode == 0 && vs.Origin == (driver.Point{}) && vs.OriginZ == 0
		if plain {
			return
		}
		d = &decor{}
		decors[layer] = d
	}
	d.vs = *vs
	if tr := vs.Transition; tr.Duration > 0 && (tr.Properties == 0 || tr.Properties&driver.TransitionBackground != 0) {
		d.recolor = &tr
	}
	d.layout(layer)
	d.recolor = nil
	d.setKeyframes(layer, vs.Animation)
}

// LayoutDecor fits a layer's decoration to its bounds after its frame
// changed.
func LayoutDecor(layer objc.ID) {
	if d := decors[layer]; d != nil {
		d.layout(layer)
	}
	LayoutBorders(layer)
}

// FollowBounds keeps a layer's decoration on its box after its bounds
// origin moved, as a scroll view's does when it scrolls.
func FollowBounds(layer objc.ID) {
	b := objc.Send[CGRect](layer, selBounds)
	if d := decors[layer]; d != nil && d.at != b.Origin {
		WithoutAnimation(func() { d.follow(b.Origin) })
	}
	if r := rings[layer]; r != nil {
		WithoutAnimation(func() {
			for _, side := range r.sides {
				if side != 0 {
					side.Send(selSetFrame, b)
				}
			}
		})
	}
}

// ForgetDecor forgets the decoration of a layer that is going away.
func ForgetDecor(layer objc.ID) {
	if d := decors[layer]; d != nil {
		release(&d.clip)
		release(&d.mask)
	}
	delete(decors, layer)
	ForgetBorders(layer)
}

// release releases a layer d owns.
func release(l *objc.ID) {
	if *l != 0 {
		l.Send(selRelease)
		*l = 0
	}
}

// layout draws d for layer's bounds.
func (d *decor) layout(layer objc.ID) {
	vs := &d.vs
	b := objc.Send[CGRect](layer, selBounds)
	w, h := b.Size.Width, b.Size.Height
	size := driver.Size{Width: driver.Points(w), Height: driver.Points(h)}
	radii := vs.Radii(size)
	e := vs.Effects
	if e == nil {
		e = &driver.Effects{}
	}
	WithoutAnimation(func() {
		d.shaped = needsShape(vs, radii)
		d.covered = coversShadows(vs, e)
		d.layoutShape(layer, w, h, radii)
		d.layoutShadows(layer, e.Shadows, w, h, radii)
		d.layoutGradients(layer, e.Gradients, w, h, radii)
		d.layoutStroke(layer, w, h, radii)
		d.layoutOutline(layer, w, h, radii)
		d.layoutMask(layer, e.Mask, w, h)
		d.layoutFilters(layer, e.Filters)
		layer.Send(selSetDoubleSided, !vs.BackfaceHidden)
		layer.Send(selSetCompositing, blendFilter(vs.BlendMode))
		d.layoutPerspective(layer, w, h)
		// Laid out at the origin; then on the box.
		d.at = CGPoint{}
		d.follow(b.Origin)
	})
	if d.keyframed && d.keyframedAt != b.Size {
		// Its steps turn around the center of the old size.
		d.keyframed = false
		d.setKeyframes(layer, d.keyframes)
	}
}

// follow moves d's layers in the layer's coordinates from d.at to at.
func (d *decor) follow(at CGPoint) {
	dx, dy := at.X-d.at.X, at.Y-d.at.Y
	d.at = at
	if dx == 0 && dy == 0 {
		return
	}
	// The clip is in the gradient mask's coordinates when there is one.
	m := d.mask
	if m == 0 {
		m = d.clip
	}
	ls := append([]objc.ID{d.fill, d.cover, d.insets, d.stroke, d.outline, m}, d.gradients...)
	for _, l := range d.shadows {
		// Inset shadows in the holder move with it.
		if l.Send(selSuperlayer) != d.insets {
			ls = append(ls, l)
		}
	}
	for _, l := range ls {
		if l != 0 {
			p := objc.Send[CGPoint](l, selPosition)
			l.Send(selSetPosition, CGPoint{X: p.X + dx, Y: p.Y + dy})
		}
	}
}

// setCorners gives layer radius r on the masked corners, continuous ones
// where they fit: Core Animation's need about 1.53 times their radius of
// straight edge, and a circle or a pill is circular anyway, as in NSH.
func setCorners(layer objc.ID, r driver.Points, masked uint, continuous bool, w, h float64) {
	layer.Send(selSetCornerRadius, float64(r))
	if r > 0 {
		layer.Send(selSetMaskedCorners, masked)
	}
	curve := "circular"
	if continuous && float64(r)*1.528 <= min(w, h)/2 {
		curve = "continuous"
	}
	layer.Send(selSetCornerCurve, NSString(curve))
}

// cornersOf returns the radius radii share and the corners it rounds.
func cornersOf(radii driver.Corners) (driver.Points, uint) {
	var r driver.Points
	var masked uint
	for i, c := range radii.Array() {
		if c > 0 {
			r = c
			// Top left, top right, bottom right, bottom left, in the
			// layer's own corners (y down).
			masked |= [4]uint{1, 2, 8, 4}[i]
		}
	}
	return r, masked
}

// coversShadows reports whether an opaque background or gradient hides
// vs's outer shadows inside the box. A background a Core Animation
// transition animates stays the layer's own.
func coversShadows(vs *driver.ViewStyle, e *driver.Effects) bool {
	outer := false
	for _, sh := range e.Shadows {
		outer = outer || !sh.Inset
	}
	if !outer {
		return false
	}
	if opaqueGradient(e) {
		return true
	}
	tr := vs.Transition
	animated := tr.Duration > 0 && (tr.Properties == 0 || tr.Properties&driver.TransitionBackground != 0)
	return vs.Background&0xff == 0xff && !animated
}

// layoutShape gives layer its corners: its own corner radius where one
// radius covers them, else a fill shape and a clip mask.
func (d *decor) layoutShape(layer objc.ID, w, h float64, radii driver.Corners) {
	vs := &d.vs
	if !d.shaped {
		drop(&d.fill)
		if d.clip != 0 {
			if d.mask == 0 {
				layer.Send(selSetMask, objc.ID(0))
			}
			release(&d.clip)
		}
		r, masked := cornersOf(radii)
		setCorners(layer, r, masked, vs.ContinuousCorners, w, h)
		rim, inset, rimmed := vs.Rim()
		if rimmed || d.covered && (vs.Effects == nil || !opaqueGradient(vs.Effects)) {
			// Above the outer shadows, so they need no mask; under the
			// border, so its outer edge is its own.
			if d.cover == 0 {
				d.cover = newLayer(layer, "CALayer", zFill)
				d.paintCover(d.covers)
			}
			in := float64(inset)
			d.cover.Send(selSetFrame, CGRect{Origin: CGPoint{X: in, Y: in}, Size: CGSize{Width: w - 2*in, Height: h - 2*in}})
			setCorners(d.cover, max(r-inset, 0), masked, vs.ContinuousCorners, w-2*in, h-2*in)
			if d.covers != vs.Background {
				animate(d.cover, "backgroundColor", d.recolor, false, func() { d.paintCover(vs.Background) })
			}
			d.covers = vs.Background
			layer.Send(selSetBackgroundColor, uintptr(0))
			if rimmed {
				c := CGColor(rim)
				layer.Send(selSetBorderColor, c)
				Release(c)
			}
		} else {
			drop(&d.cover)
			d.covers = vs.Background
		}
		return
	}
	drop(&d.cover)
	d.covers = vs.Background
	// Shapes draw the corners: the layer's own background and border
	// would show square corners.
	layer.Send(selSetCornerRadius, 0.0)
	layer.Send(selSetBackgroundColor, uintptr(0))
	path := boxPath(0, 0, w, h, radii, vs.ContinuousCorners)
	defer Release(path)
	if vs.Background&0xff != 0 {
		if d.fill == 0 {
			d.fill = newLayer(layer, "CAShapeLayer", zFill)
		}
		d.fill.Send(selSetFrame, CGRect{Size: CGSize{Width: w, Height: h}})
		if _, inset, rimmed := vs.Rim(); rimmed {
			in := float64(inset)
			under := boxPath(in, in, w-2*in, h-2*in, radii.Inset(inset), vs.ContinuousCorners)
			defer Release(under)
			d.fill.Send(selSetPath, under)
		} else {
			d.fill.Send(selSetPath, path)
		}
		c := CGColor(vs.Background)
		d.fill.Send(selSetFill, c)
		Release(c)
	} else {
		drop(&d.fill)
	}
	if vs.Clip {
		layer.Send(selSetMasksToBounds, false)
		if d.clip == 0 {
			d.clip = objc.ID(objc.GetClass("CAShapeLayer")).Send(selNew)
		}
		d.clip.Send(selSetFrame, CGRect{Size: CGSize{Width: w, Height: h}})
		d.clip.Send(selSetPath, path)
		if d.mask == 0 {
			layer.Send(selSetMask, d.clip)
		}
	} else if d.clip != 0 {
		if d.mask == 0 {
			layer.Send(selSetMask, objc.ID(0))
		}
		release(&d.clip)
	}
}

// paintCover colors the cover layer.
func (d *decor) paintCover(color driver.Color) {
	c := CGColor(color)
	d.cover.Send(selSetBackgroundColor, c)
	Release(c)
}

// layoutShadows draws the box shadows: outer ones under the box, masked
// to outside it unless the box is covered, inset ones inside the padding
// box.
func (d *decor) layoutShadows(layer objc.ID, shadows []driver.BoxShadow, w, h float64, radii driver.Corners) {
	vs := &d.vs
	for len(d.shadows) > len(shadows) {
		drop(&d.shadows[len(d.shadows)-1])
		d.shadows = d.shadows[:len(d.shadows)-1]
	}
	widths := borderWidths(vs)
	x, y := float64(widths.Left), float64(widths.Top)
	iw, ih := w-x-float64(widths.Right), h-y-float64(widths.Bottom)
	inner := driver.InnerRadii(radii, widths)
	// The holder clips inset shadows by its rounded bounds where the
	// padding box's corners share a radius.
	r, masked, held := sharedRadius(inner)
	held = held && slices.ContainsFunc(shadows, func(sh driver.BoxShadow) bool { return sh.Inset })
	if held {
		if d.insets == 0 {
			d.insets = newLayer(layer, "CALayer", zInset)
		}
		d.insets.Send(selSetFrame, CGRect{Origin: CGPoint{X: x, Y: y}, Size: CGSize{Width: iw, Height: ih}})
		setCorners(d.insets, driver.Points(r), masked, vs.ContinuousCorners, iw, ih)
		d.insets.Send(selSetMasksToBounds, true)
	}
	for i, sh := range shadows {
		// CSS paints the first shadow on top.
		z := float64(zShadow - i)
		parent, frame := layer, CGRect{Size: CGSize{Width: w, Height: h}}
		if sh.Inset {
			z = float64(zInset - i)
			if held {
				// In the box's coordinates still.
				z, parent, frame.Origin = float64(-i), d.insets, CGPoint{X: -x, Y: -y}
			}
		}
		if i == len(d.shadows) {
			d.shadows = append(d.shadows, newLayer(parent, "CALayer", z))
		}
		l := d.shadows[i]
		if l.Send(selSuperlayer) != parent {
			parent.Send(selInsertSublayer, l, uint32(0))
		}
		l.Send(selSetZPos, Z(z))
		l.Send(selSetFrame, frame)
		c := CGColor(sh.Color)
		l.Send(selSetShadowColor, c)
		Release(c)
		l.Send(selSetShadowOpacity, float32(1))
		l.Send(selSetShadowRadius, float64(sh.Blur)/2)
		l.Send(selSetShadowOffset, CGSize{Width: float64(sh.Offset.X), Height: float64(sh.Offset.Y)})
		// Far enough out that the blur never reaches the edge. Masks draw
		// only inside their frames, so the outer mask's covers that far.
		far := float64(sh.Blur+abs32(sh.Spread)) + math.Abs(float64(sh.Offset.X)) + math.Abs(float64(sh.Offset.Y)) + 1
		maskFrame := CGRect{Size: CGSize{Width: w, Height: h}}
		var maskPath uintptr
		shadowPath := cgPathCreateMutable()
		if sh.Inset {
			// The padding box casts the shadow inwards from a ring around
			// a hole shrunk by the spread.
			if !held {
				maskPath = cgPathCreateMutable()
				appendPath(maskPath, driver.EllipticRect(nil, x, y, iw, ih, inner, vs.ContinuousCorners, false))
			}
			s := float64(sh.Spread)
			appendPath(shadowPath, driver.RoundedRect(nil, -far-w, -far-h, 3*w+2*far, 3*h+2*far, driver.Corners{}, false, false))
			hole := shrinkRadii(inner, s)
			appendPath(shadowPath, driver.EllipticRect(nil, x+s, y+s, max(iw-2*s, 0), max(ih-2*s, 0), hole, vs.ContinuousCorners, true))
		} else {
			// Under the box only where it is not: the box is a hole.
			if !d.covered {
				maskFrame = CGRect{Origin: CGPoint{X: -far, Y: -far}, Size: CGSize{Width: w + 2*far, Height: h + 2*far}}
				maskPath = cgPathCreateMutable()
				appendPath(maskPath, driver.RoundedRect(nil, 0, 0, w+2*far, h+2*far, driver.Corners{}, false, false))
				appendPath(maskPath, driver.RoundedRect(nil, far, far, w, h, radii, vs.ContinuousCorners, true))
			}
			s := float64(sh.Spread)
			appendPath(shadowPath, driver.RoundedRect(nil, -s, -s, max(w+2*s, 0), max(h+2*s, 0), radii.Inset(-sh.Spread),
				vs.ContinuousCorners, false))
		}
		l.Send(selSetShadowPath, shadowPath)
		Release(shadowPath)
		if maskPath == 0 {
			l.Send(selSetMask, objc.ID(0))
			continue
		}
		mask := l.Send(sel("mask"))
		if mask == 0 {
			mask = objc.ID(objc.GetClass("CAShapeLayer")).Send(selNew)
			l.Send(selSetMask, mask)
			mask.Send(selRelease)
		}
		mask.Send(selSetFrame, maskFrame)
		mask.Send(selSetPath, maskPath)
		Release(maskPath)
	}
	// The shadows have left the holder by now, so dropping it frees only
	// the holder.
	if !held {
		drop(&d.insets)
	}
}

// sharedRadius returns the radius elliptic radii share where they are
// circular, the corners it rounds, and whether they do share one.
func sharedRadius(radii [4][2]float64) (float64, uint, bool) {
	var r float64
	var masked uint
	for i, c := range radii {
		if c[0] != c[1] || c[0] > 0 && r > 0 && c[0] != r {
			return 0, 0, false
		}
		if c[0] > 0 {
			r = c[0]
			masked |= [4]uint{1, 2, 8, 4}[i]
		}
	}
	return r, masked, true
}

func abs32(p driver.Points) driver.Points {
	if p < 0 {
		return -p
	}
	return p
}

// shrinkRadii returns elliptic radii less d, never below 0.
func shrinkRadii(r [4][2]float64, d float64) [4][2]float64 {
	for i := range r {
		for j := range r[i] {
			if r[i][j] > 0 {
				r[i][j] = max(r[i][j]-d, 0)
			}
		}
	}
	return r
}

// borderWidths returns the width of each side of vs's border.
func borderWidths(vs *driver.ViewStyle) driver.Edges {
	if vs.Borders != (driver.Edges{}) {
		return vs.Borders
	}
	return driver.All(vs.BorderWidth)
}

// layoutGradients draws the gradients over the background, the first on
// top, in the box's shape.
func (d *decor) layoutGradients(layer objc.ID, paints []driver.GradientPaint, w, h float64, radii driver.Corners) {
	for len(d.gradients) > len(paints) {
		drop(&d.gradients[len(d.gradients)-1])
		d.gradients = d.gradients[:len(d.gradients)-1]
	}
	for i := range paints {
		z := float64(zFill + 10 - i)
		if i == len(d.gradients) {
			d.gradients = append(d.gradients, newLayer(layer, "CAGradientLayer", z))
		}
		g := d.gradients[i]
		g.Send(selSetFrame, CGRect{Size: CGSize{Width: w, Height: h}})
		paintGradient(g, &paints[i], w, h)
		d.shape(g, w, h, radii)
	}
}

// shape gives sublayer l the box's shape.
func (d *decor) shape(l objc.ID, w, h float64, radii driver.Corners) {
	if d.shaped {
		path := boxPath(0, 0, w, h, radii, d.vs.ContinuousCorners)
		m := l.Send(sel("mask"))
		if m == 0 {
			m = objc.ID(objc.GetClass("CAShapeLayer")).Send(selNew)
			l.Send(selSetMask, m)
			m.Send(selRelease)
		}
		m.Send(selSetFrame, CGRect{Size: CGSize{Width: w, Height: h}})
		m.Send(selSetPath, path)
		Release(path)
		return
	}
	r, masked := cornersOf(radii)
	setCorners(l, r, masked, d.vs.ContinuousCorners, w, h)
	l.Send(selSetMasksToBounds, r > 0)
}

// paintGradient makes gradient layer g draw p on a w by h box.
func paintGradient(g objc.ID, p *driver.GradientPaint, w, h float64) {
	colors := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	var at []float64
	for _, s := range p.Stops {
		c := CGColor(s.Color)
		colors.Send(selAddObject, c)
		Release(c)
		at = append(at, float64(s.Offset))
	}
	g.Send(selSetColors, colors)
	g.Send(selSetLocations, numbers(at...))
	switch p.Kind {
	case driver.GradientRadial:
		g.Send(selSetType, NSString("radial"))
		g.Send(selSetStartPoint, CGPoint{X: float64(p.CX), Y: float64(p.CY)})
		g.Send(selSetEndPoint, CGPoint{X: float64(p.CX + p.RX), Y: float64(p.CY + p.RY)})
	case driver.GradientConic:
		g.Send(selSetType, NSString("conic"))
		g.Send(selSetStartPoint, CGPoint{X: float64(p.CX), Y: float64(p.CY)})
		// The direction it starts in, the same in points on both axes.
		s, c := math.Sincos(float64(p.Angle) * math.Pi / 180)
		k := min(w, h) / 2
		g.Send(selSetEndPoint, CGPoint{X: float64(p.CX) + s*k/max(w, 1), Y: float64(p.CY) - c*k/max(h, 1)})
	default:
		g.Send(selSetType, NSString("axial"))
		g.Send(selSetStartPoint, CGPoint{X: float64(p.X0), Y: float64(p.Y0)})
		g.Send(selSetEndPoint, CGPoint{X: float64(p.X1), Y: float64(p.Y1)})
	}
}

// layoutStroke draws a dashed or dotted border, or a solid one around
// shaped corners, as a stroke over the content.
func (d *decor) layoutStroke(layer objc.ID, w, h float64, radii driver.Corners) {
	vs := &d.vs
	dashed := vs.BorderStyle == driver.BorderDashed || vs.BorderStyle == driver.BorderDotted
	width, color := vs.BorderWidth, vs.BorderColor
	if rim, _, rimmed := vs.Rim(); rimmed {
		color = rim
	}
	if vs.Borders != (driver.Edges{}) {
		// Sides that differ are rings (setBorders); dashes take the
		// widest side.
		if !dashed {
			drop(&d.stroke)
			return
		}
		b, c := vs.Borders, vs.BorderColors
		width, color = max(b.Top, b.Right, b.Bottom, b.Left), c.Top
	}
	if width <= 0 || color&0xff == 0 || (!dashed && !d.shaped) {
		drop(&d.stroke)
		if d.shaped || dashed {
			layer.Send(selSetBorderWidth, 0.0)
		}
		return
	}
	layer.Send(selSetBorderWidth, 0.0)
	if d.stroke == 0 {
		d.stroke = newLayer(layer, "CAShapeLayer", zStroke)
	}
	strokeShape(d.stroke, w, h, radii, vs.ContinuousCorners, width, width/2, color, vs.BorderStyle)
}

// strokeShape makes shape layer l stroke a line of width in color along
// the box inset by inset (outset when negative). The layer's frame holds
// the line.
func strokeShape(l objc.ID, w, h float64, radii driver.Corners, continuous bool, width, inset driver.Points,
	color driver.Color, style driver.BorderStyle) {
	in := float64(inset)
	pad := max(-in, 0) + float64(width)
	l.Send(selSetFrame, CGRect{Origin: CGPoint{X: -pad, Y: -pad}, Size: CGSize{Width: w + 2*pad, Height: h + 2*pad}})
	path := boxPath(pad+in, pad+in, w-2*in, h-2*in, radii.Inset(inset), continuous)
	l.Send(selSetPath, path)
	Release(path)
	l.Send(selSetFill, uintptr(0))
	c := CGColor(color)
	l.Send(selSetStrokeColor, c)
	Release(c)
	l.Send(selSetLineWidth, float64(width))
	if on, off, round, ok := style.Dashes(width); ok {
		l.Send(selSetLineDash, numbers(float64(on), float64(off)))
		cap := "butt"
		if round {
			cap = "round"
		}
		l.Send(selSetLineCap, NSString(cap))
	} else {
		l.Send(selSetLineDash, objc.ID(0))
		l.Send(selSetLineCap, NSString("butt"))
	}
}

// layoutOutline draws the outline outside the border box.
func (d *decor) layoutOutline(layer objc.ID, w, h float64, radii driver.Corners) {
	o := d.vs.Outline
	if !o.Visible() {
		drop(&d.outline)
		return
	}
	if d.outline == 0 {
		d.outline = newLayer(layer, "CAShapeLayer", zOutline)
	}
	strokeShape(d.outline, w, h, radii, d.vs.ContinuousCorners, o.Width, -(o.Offset + o.Width/2), o.Color, o.Style)
}

// layoutMask masks the layer by a gradient's alpha, within the clip shape
// when there is one.
func (d *decor) layoutMask(layer objc.ID, p *driver.GradientPaint, w, h float64) {
	if p == nil {
		if d.mask != 0 {
			layer.Send(selSetMask, d.clip)
			release(&d.mask)
		}
		return
	}
	if d.mask == 0 {
		d.mask = objc.ID(objc.GetClass("CAGradientLayer")).Send(selNew)
	}
	d.mask.Send(selSetFrame, CGRect{Size: CGSize{Width: w, Height: h}})
	paintGradient(d.mask, p, w, h)
	d.mask.Send(selSetMask, d.clip)
	layer.Send(selSetMask, d.mask)
}

// layoutFilters gives the layer Core Image filters: Core Animation draws
// them on macOS only, where the view must use Core Image filters
// (UsesFilters). A drop shadow is the layer's own shadow (layerShadow),
// without a path, so it follows what the layer draws.
func (d *decor) layoutFilters(layer objc.ID, filters []driver.Filter) {
	m, color, blur, _ := driver.SplitFilters(filters)
	if runtime.GOOS == "ios" {
		// UIKit's layers take no Core Image filters; opacity is what
		// they can do.
		if color && m[0] == 1 && m[6] == 1 && m[12] == 1 && m[4] == 0 && m[9] == 0 && m[14] == 0 {
			layer.Send(selSetOpacity, float32(d.vs.Opacity*m[18]))
		}
		return
	}
	if !color && blur == 0 {
		layer.Send(selSetFilters, objc.ID(0))
		return
	}
	list := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	ci := objc.ID(objc.GetClass("CIFilter"))
	vector := objc.ID(objc.GetClass("CIVector"))
	if color {
		f := ci.Send(selFilterWithName, NSString("CIColorMatrix"))
		for i, key := range []string{"inputRVector", "inputGVector", "inputBVector", "inputAVector"} {
			f.Send(selSetValueForKey, vector.Send(selVectorWithXYZW,
				float64(m[i*5]), float64(m[i*5+1]), float64(m[i*5+2]), float64(m[i*5+3])), NSString(key))
		}
		f.Send(selSetValueForKey, vector.Send(selVectorWithXYZW, float64(m[4]), float64(m[9]), float64(m[14]), float64(m[19])),
			NSString("inputBiasVector"))
		list.Send(selAddObject, f)
	}
	if blur > 0 {
		f := ci.Send(selFilterWithName, NSString("CIGaussianBlur"))
		f.Send(selSetValueForKey, objc.ID(objc.GetClass("NSNumber")).Send(selNumberWithDouble, float64(blur)),
			NSString("inputRadius"))
		list.Send(selAddObject, f)
	}
	layer.Send(selSetFilters, list)
}

// UsesFilters reports whether vs gives its layer Core Image filters.
func UsesFilters(vs *driver.ViewStyle) bool {
	if vs.Effects == nil || runtime.GOOS == "ios" {
		return false
	}
	_, color, blur, _ := driver.SplitFilters(vs.Effects.Filters)
	return color || blur > 0
}

// blendFilter is the compositing filter of a blend mode: a Core Image
// filter on macOS, its name on iOS; nil for normal.
func blendFilter(b driver.BlendMode) objc.ID {
	names := [...]string{"", "multiply", "screen", "overlay", "darken", "lighten", "colorDodge", "colorBurn",
		"hardLight", "softLight", "difference", "exclusion", "hue", "saturation", "color", "luminosity", "plusL", "plusD"}
	if b == 0 || int(b) >= len(names) {
		return 0
	}
	name := names[b]
	if runtime.GOOS == "ios" {
		if b == driver.BlendPlusLighter || b == driver.BlendPlusDarker {
			return NSString(name)
		}
		return NSString(name + "BlendMode")
	}
	ci := map[driver.BlendMode]string{driver.BlendPlusLighter: "CIAdditionCompositing",
		driver.BlendPlusDarker: "CILinearBurnBlendMode"}[b]
	if ci == "" {
		ci = "CI" + string(name[0]-'a'+'A') + name[1:] + "BlendMode"
	}
	return objc.ID(objc.GetClass("CIFilter")).Send(selFilterWithName, NSString(ci))
}

// layoutPerspective gives the children the view's perspective, seen from
// its center: CSS's perspective.
func (d *decor) layoutPerspective(layer objc.ID, w, h float64) {
	depth := d.vs.ChildPerspective
	if depth <= 0 {
		layer.Send(selSetSublayerXform, toCA(driver.Identity3D))
		return
	}
	// The sublayer transform acts around the layer's anchor point.
	anchor := objc.Send[CGPoint](layer, sel("anchorPoint"))
	cx, cy := (0.5-anchor.X)*w, (0.5-anchor.Y)*h
	m := driver.Translate3D(cx, cy, 0).Mul(driver.Perspective3D(float64(depth))).Mul(driver.Translate3D(-cx, -cy, 0))
	layer.Send(selSetSublayerXform, toCA(m))
}

// toCA converts a matrix to CATransform3D, which stores column vectors'
// matrices the same way.
func toCA(m driver.Matrix3D) transform3D {
	return transform3D{M11: m[0], M12: m[1], M13: m[2], M14: m[3], M21: m[4], M22: m[5], M23: m[6], M24: m[7],
		M31: m[8], M32: m[9], M33: m[10], M34: m[11], M41: m[12], M42: m[13], M43: m[14], M44: m[15]}
}

// setKeyframes runs a keyframe animation on the layer, additive on its
// transform and its opacity, so it goes on top of them. The transform's
// steps turn and scale around the center, wherever the anchor point is.
func (d *decor) setKeyframes(layer objc.ID, a driver.KeyframeAnimation) {
	if a == d.keyframes && (d.keyframed || a.Keyframes == 0) {
		return
	}
	d.keyframes = a
	for _, k := range []string{"transform", "opacity"} {
		layer.Send(selRemoveAnimation, NSString("nsgo.kf."+k))
	}
	dur, frames := a.Cycle()
	d.keyframed = dur > 0
	if dur <= 0 {
		return
	}
	b := objc.Send[CGRect](layer, selBounds)
	d.keyframedAt = b.Size
	anchor := objc.Send[CGPoint](layer, sel("anchorPoint"))
	cx, cy := (0.5-anchor.X)*b.Size.Width, (0.5-anchor.Y)*b.Size.Height
	frames = splitTurns(frames, math.Hypot(cx, cy))
	moves := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	var fades, times []float64
	timings := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	for i, k := range frames {
		t := driver.Transform{TranslateY: driver.Points(k.TranslateY) * driver.Points(b.Size.Height), Rotate: k.Rotate, Scale: k.Scale}
		m := driver.Translate3D(cx, cy, 0).Mul(t.Matrix(driver.Point{})).Mul(driver.Translate3D(-cx, -cy, 0))
		moves.Send(selAddObject, objc.ID(objc.GetClass("NSValue")).Send(sel("valueWithCATransform3D:"), toCA(m)))
		fades = append(fades, float64(k.Opacity)-1)
		times = append(times, float64(k.Offset))
		if i < len(frames)-1 {
			timings.Send(selAddObject, timing(k.Easing))
		}
	}
	for _, kv := range []struct {
		key    string
		values objc.ID
	}{{"transform", moves}, {"opacity", numbers(fades...)}} {
		anim := objc.ID(objc.GetClass("CAKeyframeAnimation")).Send(selAnimationWithPath, NSString(kv.key))
		anim.Send(sel("setValues:"), kv.values)
		anim.Send(sel("setKeyTimes:"), numbers(times...))
		anim.Send(sel("setTimingFunctions:"), timings)
		anim.Send(sel("setAdditive:"), true)
		anim.Send(sel("setDuration:"), dur.Seconds())
		repeat := float32(math.Inf(1))
		if a.Iterations > 0 {
			repeat = float32(a.Iterations)
		}
		anim.Send(sel("setRepeatCount:"), repeat)
		anim.Send(sel("setRemovedOnCompletion:"), a.Iterations > 0)
		if a.Delay > 0 {
			anim.Send(sel("setBeginTime:"), caCurrentMediaTime()+a.Delay.Seconds())
		}
		layer.Send(selAddAnimation, anim, NSString("nsgo.kf."+kv.key))
	}
}

// turnStep is the most degrees a step between two matrices may turn for
// Core Animation, which tweens matrices: a whole turn leaves one
// unchanged, so a step turns at most a quarter turn. It also tweens their
// translation in a straight line, so a center radius points away from
// the anchor (AppKit keeps it at the corner) cuts across the arc it turns
// along: steps stay short enough to keep it within a tenth of a point.
func turnStep(radius float64) float64 {
	if radius > 0.1 {
		return min(90, 2*math.Acos(1-0.1/radius)*180/math.Pi)
	}
	return 90
}

// splitTurns adds steps to linear steps that turn further than turnStep.
func splitTurns(frames []driver.Keyframe, radius float64) []driver.Keyframe {
	step := turnStep(radius)
	var out []driver.Keyframe
	for i, k := range frames {
		if i > 0 {
			prev := frames[i-1]
			turn := k.Rotate - prev.Rotate
			if n := int(math.Ceil(math.Abs(float64(turn)) / step)); n > 1 && prev.Easing == (driver.Curve{}) {
				for j := 1; j < n; j++ {
					f := float32(j) / float32(n)
					lerp := func(a, b float32) float32 { return a + (b-a)*f }
					out = append(out, driver.Keyframe{Offset: lerp(prev.Offset, k.Offset), Opacity: lerp(prev.Opacity, k.Opacity),
						Scale: lerp(prev.Scale, k.Scale), Rotate: lerp(prev.Rotate, k.Rotate), TranslateY: lerp(prev.TranslateY, k.TranslateY)})
				}
			}
		}
		out = append(out, k)
	}
	return out
}

// opaqueGradient reports whether e paints an opaque gradient, which
// covers the box.
func opaqueGradient(e *driver.Effects) bool {
	for _, g := range e.Gradients {
		opaque := len(g.Stops) > 0
		for _, st := range g.Stops {
			opaque = opaque && st.Color&0xff == 0xff
		}
		if opaque {
			return true
		}
	}
	return false
}
