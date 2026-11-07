//go:build darwin

package apple

import (
	"math"
	"strings"
	"structs"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

var (
	sel = objc.RegisterName

	selSetBackgroundColor = sel("setBackgroundColor:")
	selSetCornerRadius    = sel("setCornerRadius:")
	selSetBorderWidth     = sel("setBorderWidth:")
	selSetBorderColor     = sel("setBorderColor:")
	selSetOpacity         = sel("setOpacity:")
	selSetMasksToBounds   = sel("setMasksToBounds:")
	selSetShadowColor     = sel("setShadowColor:")
	selSetShadowOffset    = sel("setShadowOffset:")
	selSetShadowRadius    = sel("setShadowRadius:")
	selSetShadowOpacity   = sel("setShadowOpacity:")
	selSetShadow          = sel("setShadow:")
	selValueForKey        = sel("valueForKey:")
	selBounds             = sel("bounds")

	caCurrentMediaTime func() float64
)

// StyleLayer draws vs with a view's CALayer. An NSView whose layer it
// is takes the layer's shadow, as AppKit copies a view's shadow onto its
// layer. A style with a transition animates what it lists from the
// values shown; was is the style last drawn, nil for none.
func StyleLayer(layer objc.ID, was, vs *driver.ViewStyle, view objc.ID) {
	tr := vs.Transition
	var keys []string
	var from []objc.ID
	if tr.Duration > 0 {
		shown := layer.Send(sel("presentationLayer"))
		if shown == 0 {
			shown = layer
		}
		keys = transitionKeys(tr.Properties)
		for _, k := range keys {
			from = append(from, shown.Send(selValueForKey, NSString(k)))
		}
	}
	color := func(sel objc.SEL, c driver.Color) {
		if c == 0 {
			layer.Send(sel, uintptr(0))
			return
		}
		ref := CGColor(c)
		layer.Send(sel, ref)
		Release(ref)
	}
	color(selSetBackgroundColor, vs.Background)
	layer.Send(selSetCornerRadius, float64(vs.CornerRadius))
	layer.Send(selSetBorderWidth, float64(vs.BorderWidth))
	color(selSetBorderColor, vs.BorderColor)
	layer.Send(selSetOpacity, vs.Opacity)
	layer.Send(selSetMasksToBounds, vs.Clip)
	sh := layerShadow(vs)
	switch {
	case view != 0:
		setViewShadow(view, sh)
	case sh.Color == 0:
		layer.Send(selSetShadowOpacity, float32(0))
	default:
		color(selSetShadowColor, sh.Color)
		layer.Send(selSetShadowOffset, CGSize{Width: float64(sh.Offset.X), Height: float64(sh.Offset.Y)})
		layer.Send(selSetShadowRadius, float64(sh.Blur)/2)
		layer.Send(selSetShadowOpacity, float32(1))
	}
	setBorders(layer, vs, &tr)
	applyDecor(layer, vs)
	SetTransform(layer, vs.Transform)
	for i, k := range keys {
		to := layer.Send(selValueForKey, NSString(k))
		if from[i] == to || (from[i] != 0 && objc.Send[bool](from[i], sel("isEqual:"), to)) {
			continue
		}
		start := from[i]
		if start == 0 && strings.HasSuffix(k, "Color") {
			clear := CGColor(0)
			start = objc.ID(clear).Send(sel("autorelease"))
		}
		var anim objc.ID
		if k == "transform" {
			anim = moveAnimation(layer, was, vs)
		}
		if anim == 0 {
			anim = objc.ID(objc.GetClass("CABasicAnimation")).Send(sel("animationWithKeyPath:"), NSString(k))
			anim.Send(sel("setFromValue:"), start)
		}
		anim.Send(sel("setDuration:"), tr.Duration.Seconds())
		anim.Send(sel("setTimingFunction:"), timing(tr.Easing))
		if tr.Delay > 0 {
			anim.Send(sel("setBeginTime:"), caCurrentMediaTime()+tr.Delay.Seconds())
			anim.Send(sel("setFillMode:"), NSString("backwards"))
		}
		layer.Send(selAddAnimation, anim, NSString("nsgo."+k))
	}
}

// RestyleLayer draws vs with a layer that shows shown, the style
// StyleLayer last drew there; with none it draws vs at once. When the two
// differ only in opacity and transform, as an animation's frames do, it
// sets just those: drawing every property again remade its colors,
// shadow, filters and gradients each frame, and the window server drew
// them all again.
func RestyleLayer(layer objc.ID, shown, vs *driver.ViewStyle, view objc.ID) {
	if shown == nil && vs.Transition.Duration > 0 {
		// A view's first style shows at once, as CSS transitions none.
		first := *vs
		first.Transition = driver.Transition{}
		StyleLayer(layer, nil, &first, view)
		return
	}
	if shown == nil || vs.Transition.Duration > 0 || !onlyMoves(shown, vs) {
		StyleLayer(layer, shown, vs, view)
		return
	}
	if vs.Opacity != shown.Opacity {
		layer.Send(selSetOpacity, vs.Opacity)
	}
	if vs.Transform != shown.Transform {
		SetTransform(layer, vs.Transform)
	}
	if d := decors[layer]; d != nil {
		d.vs.Opacity, d.vs.Transform = vs.Opacity, vs.Transform
	}
}

// onlyMoves reports whether b differs from a in its opacity and transform
// alone. An opacity under filters is drawn with them on iOS (layoutFilters).
func onlyMoves(a, b *driver.ViewStyle) bool {
	x, y := *a, *b
	x.Transform, y.Transform = driver.Transform{}, driver.Transform{}
	if x.Effects != nil && len(x.Effects.Filters) > 0 {
		return x == y
	}
	x.Opacity, y.Opacity = 0, 0
	return x == y
}

// layerShadow returns the shadow a view's layer casts of what it draws:
// its widest drop shadow, as NSH's Apple backends draw, else its React
// Native shadow, whose radius is half a box shadow's blur.
func layerShadow(vs *driver.ViewStyle) driver.BoxShadow {
	if vs.Effects != nil {
		if _, _, _, shadows := driver.SplitFilters(vs.Effects.Filters); len(shadows) > 0 {
			sh := shadows[0]
			for _, s := range shadows[1:] {
				if s.Blur >= sh.Blur {
					sh = s
				}
			}
			return sh
		}
	}
	return driver.BoxShadow{Offset: vs.ShadowOffset, Blur: 2 * vs.ShadowRadius, Color: vs.ShadowColor}
}

// setViewShadow gives an NSView sh. AppKit owns the shadow of a view's
// layer and resets what is set on the layer to the view's shadow, which
// it copies there unchanged: the offset as the layer's, the blur radius
// as its shadow radius.
func setViewShadow(view objc.ID, sh driver.BoxShadow) {
	if sh.Color == 0 {
		view.Send(selSetShadow, objc.ID(0))
		return
	}
	ns := objc.ID(objc.GetClass("NSShadow")).Send(selNew)
	ref := CGColor(sh.Color)
	ns.Send(sel("setShadowColor:"), objc.ID(objc.GetClass("NSColor")).Send(sel("colorWithCGColor:"), ref))
	Release(ref)
	ns.Send(sel("setShadowOffset:"), CGSize{Width: float64(sh.Offset.X), Height: float64(sh.Offset.Y)})
	ns.Send(sel("setShadowBlurRadius:"), float64(sh.Blur)/2)
	view.Send(selSetShadow, ns)
	ns.Send(selRelease)
}

// transitionKeys returns the CALayer keys of the properties a transition
// animates.
func transitionKeys(p driver.TransitionProperty) []string {
	if p == 0 {
		p = ^driver.TransitionProperty(0)
	}
	var keys []string
	add := func(bit driver.TransitionProperty, ks ...string) {
		if p&bit != 0 {
			keys = append(keys, ks...)
		}
	}
	add(driver.TransitionOpacity, "opacity")
	add(driver.TransitionBackground, "backgroundColor")
	add(driver.TransitionTransform, "transform")
	add(driver.TransitionBorder, "borderWidth", "borderColor", "cornerRadius")
	add(driver.TransitionShadow, "shadowColor", "shadowOpacity", "shadowOffset", "shadowRadius")
	return keys
}

// timing returns the CAMediaTimingFunction of a curve.
func timing(c driver.Curve) objc.ID {
	fn := objc.ID(objc.GetClass("CAMediaTimingFunction"))
	if c == (driver.Curve{}) {
		return fn.Send(sel("functionWithName:"), NSString("linear"))
	}
	return fn.Send(sel("functionWithControlPoints::::"), c.X1, c.Y1, c.X2, c.Y2)
}

// transform3D is CATransform3D, which multiplies row vectors.
type transform3D struct {
	_                                                                              structs.HostLayout
	M11, M12, M13, M14, M21, M22, M23, M24, M31, M32, M33, M34, M41, M42, M43, M44 float64
}

// layerTransform is layer's transform for t around the transform origin
// offset from its center, and that origin's distance from the anchor
// point. It works from wherever the anchor point is, since AppKit keeps
// its layers' at their corner.
func layerTransform(layer objc.ID, t driver.Transform, offset driver.Point, z float64) (transform3D, float64) {
	size := objc.Send[CGRect](layer, selBounds).Size
	anchor := objc.Send[CGPoint](layer, sel("anchorPoint"))
	ax, ay := anchor.X*size.Width, anchor.Y*size.Height
	origin := driver.Point{X: driver.Points(size.Width/2) + offset.X, Y: driver.Points(size.Height/2) + offset.Y}
	m := driver.Translate3D(-ax, -ay, z).Mul(t.Matrix(origin)).Mul(driver.Translate3D(ax, ay, -z))
	return toCA(m), math.Hypot(float64(origin.X)-ax, float64(origin.Y)-ay)
}

// SetTransform sets a layer's transform to t around its transform
// origin: its center, moved by its style's Origin.
func SetTransform(layer objc.ID, t driver.Transform) {
	var offset driver.Point
	var z float64
	if d := decors[layer]; d != nil {
		offset, z = d.vs.Origin, float64(d.vs.OriginZ)
	}
	m, _ := layerTransform(layer, t, offset, z)
	if m == (transform3D{M11: 1, M22: 1, M33: 1, M44: 1}) {
		delete(held, layer)
	} else {
		held[layer] = m
	}
	setting = true
	layer.Send(sel("setTransform:"), m)
	setting = false
}

// move is a transform transition on a layer, from one style to another
// over times of the media clock.
type move struct {
	from, to        driver.ViewStyle
	begin, duration float64
	curve           driver.Curve
}

// moves are the layers' latest transform transitions.
var moves = map[objc.ID]*move{}

// moveAnimation returns the animation that moves layer's transform from
// what it shows to vs's and remembers it, or 0 if it cannot tell: the two
// differ in matrices of the app's, or nothing was drawn yet. It steps
// through the styles between, as Windows and Linux do. Between two
// matrices Core Animation moves the anchor point in a straight line, so
// a turn around any other point, as all of AppKit's layers make, would
// swing off that point on the way.
func moveAnimation(layer objc.ID, was, vs *driver.ViewStyle) objc.ID {
	if was == nil {
		delete(moves, layer)
		return 0
	}
	from, now := *was, caCurrentMediaTime()
	if m := moves[layer]; m != nil && now < m.begin+m.duration {
		from = driver.MixStyles(m.from, m.to, m.curve.At((now-m.begin)/m.duration), driver.TransitionTransform)
	}
	if from.Transform.Then != vs.Transform.Then {
		delete(moves, layer)
		return 0
	}
	tr := vs.Transition
	moves[layer] = &move{from: from, to: *vs, begin: now + tr.Delay.Seconds(), duration: tr.Duration.Seconds(), curve: tr.Easing}
	values := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	_, radius := layerTransform(layer, vs.Transform, vs.Origin, float64(vs.OriginZ))
	n := max(int(math.Ceil(turned(from.Transform, vs.Transform)/turnStep(radius))), 1)
	for i := range n + 1 {
		s := driver.MixStyles(from, *vs, float64(i)/float64(n), driver.TransitionTransform)
		m, _ := layerTransform(layer, s.Transform, s.Origin, float64(vs.OriginZ))
		values.Send(selAddObject, objc.ID(objc.GetClass("NSValue")).Send(sel("valueWithCATransform3D:"), m))
	}
	anim := objc.ID(objc.GetClass("CAKeyframeAnimation")).Send(selAnimationWithPath, NSString("transform"))
	anim.Send(sel("setValues:"), values)
	return anim
}

// turned returns the degrees t turns or slants through on the way to u.
func turned(t, u driver.Transform) float64 {
	var sum float64
	for _, d := range []float32{u.Rotate - t.Rotate, u.RotateX - t.RotateX, u.RotateY - t.RotateY, u.RotateZ - t.RotateZ,
		u.SkewX - t.SkewX, u.SkewY - t.SkewY} {
		sum += math.Abs(float64(d))
	}
	return sum
}

var (
	// held are the transforms SetTransform gave layers; setting says it
	// is setting one.
	held    = map[objc.ID]transform3D{}
	setting bool
)

// HeldTransformMethods are the methods of a CALayer subclass that keeps
// the transform SetTransform gave it: AppKit sets its views' layers back
// to its own transform whenever it syncs their geometry.
func HeldTransformMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{Cmd: sel("setTransform:"), Fn: func(self objc.ID, cmd objc.SEL, m transform3D) {
			if h, ok := held[self]; ok && !setting {
				m = h
			}
			Super(self, cmd, m)
		}},
		{Cmd: sel("setAffineTransform:"), Fn: func(self objc.ID, cmd objc.SEL, m affine) {
			if h, ok := held[self]; ok {
				Super(self, sel("setTransform:"), h)
				return
			}
			Super(self, cmd, m)
		}},
	}
}

// ForgetTransform forgets the transform SetTransform gave a layer that is
// going away.
func ForgetTransform(layer objc.ID) {
	delete(held, layer)
	delete(moves, layer)
}

// SetSublayerTranslation moves layer's sublayers by (dx, dy) points, as a
// scroll view shows an offset its bounds do not have yet.
func SetSublayerTranslation(layer objc.ID, dx, dy float64) {
	layer.Send(sel("setSublayerTransform:"), transform3D{M11: 1, M22: 1, M33: 1, M44: 1, M41: dx, M42: dy})
}

// RotateLayer turns layer by radians around its anchor point.
func RotateLayer(layer objc.ID, radians float64) {
	c, s := math.Cos(radians), math.Sin(radians)
	layer.Send(sel("setTransform:"), transform3D{M11: c, M12: s, M21: -s, M22: c, M33: 1, M44: 1})
}

// MediaTime returns Core Animation's clock: the time since the system
// started, on which display links and NSEvent timestamps count too.
func MediaTime() time.Duration { return time.Duration(caCurrentMediaTime() * float64(time.Second)) }
