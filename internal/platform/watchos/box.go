package watchos

import (
	"github.com/oneo-me/nsgo/internal/driver"
)

// A view's look is drawn as a canvas is: boxOps records its shadows,
// background, gradients, inset shadows, border and outline as a Drawing's
// ops, which apple.Play draws with Core Graphics into the view's SwiftUI
// Canvas, in the order Linux's software painter draws them. It is plain
// Go, so its tests run on any machine.

// far is how far a shadow's shape is moved off the canvas, the shadow
// moved back by as much, so only the shadow draws.
const far = 1e5

// boxOps returns the ops that draw a w by h box styled s, under its
// content (back) and over it (front): the border and outline go over
// what the view shows, as UIKit draws a layer's border.
func boxOps(s *driver.ViewStyle, w, h float32) (back, front []driver.DrawOp) {
	if w <= 0 || h <= 0 {
		return nil, nil
	}
	size := driver.Size{Width: driver.Points(w), Height: driver.Points(h)}
	radii := s.Radii(size)
	continuous := s.ContinuousCorners && s.Rounded()
	e := s.Effects
	if e == nil {
		e = &driver.Effects{}
	}
	d := driver.NewDrawing(nil)
	filled := s.Background&0xff != 0 || len(e.Gradients) > 0
	if s.ShadowColor&0xff != 0 && filled && (s.ShadowRadius > 0 || s.ShadowOffset != driver.Point{}) {
		dropShadow(d, s.ShadowColor, s.ShadowOffset, s.ShadowRadius, 0, radii, continuous, w, h)
	}
	// The first shadow is on top.
	for i := len(e.Shadows) - 1; i >= 0; i-- {
		if b := e.Shadows[i]; !b.Inset && b.Color&0xff != 0 {
			dropShadow(d, b.Color, b.Offset, b.Blur/2, b.Spread, radii, continuous, w, h)
		}
	}
	border := *s
	if rim, inset, ok := s.Rim(); ok {
		d.SetFillColor(s.Background)
		d.FillPath(shape(radii, continuous, w, h, float32(inset)), driver.NonZero)
		border.BorderColor = rim
	} else if s.Background&0xff != 0 {
		d.SetFillColor(s.Background)
		d.FillPath(shape(radii, continuous, w, h, 0), driver.NonZero)
	}
	if len(e.Gradients) > 0 {
		d.Save()
		d.ClipPath(shape(radii, continuous, w, h, 0), driver.NonZero)
		for i := len(e.Gradients) - 1; i >= 0; i-- {
			d.SetFillGradient(canvasGradient(&e.Gradients[i], w, h))
			d.FillRect(0, 0, w, h)
		}
		d.Restore()
	}
	insetShadows(d, &border, e.Shadows, radii, w, h)
	back = d.Ops
	f := driver.NewDrawing(nil)
	drawBorder(f, &border, radii, continuous, w, h)
	if o := s.Outline; o.Visible() {
		stroke(f, shape(radii, continuous, w, h, -float32(o.Offset+o.Width/2)), o.Width, o.Color, o.Style)
	}
	return back, f.Ops
}

// shape is the box's outline inset by d, its corners inset with it;
// a negative d outsets it.
func shape(radii driver.Corners, continuous bool, w, h, d float32) *driver.Path {
	return segmentsPath(driver.RoundedRect(nil, float64(d), float64(d), float64(w-2*d), float64(h-2*d),
		radii.Inset(driver.Points(d)), continuous, false))
}

// segmentsPath is a path of segs.
func segmentsPath(segs []driver.Segment) *driver.Path {
	p := &driver.Path{}
	for _, s := range segs {
		pt := func(i int) (float32, float32) { return float32(s.P[i][0]), float32(s.P[i][1]) }
		switch s.Kind {
		case driver.SegmentMove:
			p.MoveTo(pt(0))
		case driver.SegmentLine:
			p.LineTo(pt(0))
		case driver.SegmentCubic:
			x1, y1 := pt(0)
			x2, y2 := pt(1)
			x, y := pt(2)
			p.CubicTo(x1, y1, x2, y2, x, y)
		case driver.SegmentClose:
			p.ClosePath()
		}
	}
	return p
}

// dropShadow draws a shadow of the box grown by spread, without the box:
// the box goes far off the canvas, its shadow offset back onto it.
func dropShadow(d *driver.Drawing, c driver.Color, off driver.Point, radius, spread driver.Points,
	radii driver.Corners, continuous bool, w, h float32) {
	d.Save()
	d.SetShadow(c, float32(off.X)+far, float32(off.Y), float32(radius)*2)
	d.Translate(-far, 0)
	d.SetFillColor(driver.RGBA(0x000000, 1))
	d.FillPath(shape(radii, continuous, w, h, -float32(spread)), driver.NonZero)
	d.Restore()
}

// insetShadows draws the inset shadows inside the padding box, the first
// on top: each is the shadow of what lies outside the box shrunk by its
// spread, clipped to the box.
func insetShadows(d *driver.Drawing, s *driver.ViewStyle, shadows []driver.BoxShadow, radii driver.Corners, w, h float32) {
	widths := s.Borders
	if widths == (driver.Edges{}) {
		widths = driver.All(s.BorderWidth)
	}
	inner := driver.InnerRadii(radii, widths)
	x, y := float64(widths.Left), float64(widths.Top)
	pw, ph := float64(w)-x-float64(widths.Right), float64(h)-y-float64(widths.Bottom)
	if pw <= 0 || ph <= 0 {
		return
	}
	for i := len(shadows) - 1; i >= 0; i-- {
		b := shadows[i]
		if !b.Inset || b.Color&0xff == 0 {
			continue
		}
		sp := float64(b.Spread)
		hole := inner
		for j := range hole {
			hole[j] = [2]float64{max(hole[j][0]-sp, 0), max(hole[j][1]-sp, 0)}
		}
		d.Save()
		d.ClipPath(segmentsPath(driver.EllipticRect(nil, x, y, pw, ph, inner, false, false)), driver.NonZero)
		d.SetShadow(b.Color, float32(b.Offset.X), float32(b.Offset.Y), float32(b.Blur))
		d.SetFillColor(b.Color)
		ring := segmentsPath(driver.EllipticRect(nil, x+sp, y+sp, max(pw-2*sp, 0), max(ph-2*sp, 0), hole, false, false))
		ring.Rect(-float32(b.Blur)-w, -float32(b.Blur)-h, 3*w+2*float32(b.Blur), 3*h+2*float32(b.Blur))
		d.FillPath(ring, driver.EvenOdd)
		d.Restore()
	}
}

// drawBorder draws the border inside the box: a stroked outline where
// every side is the same, or dashed, dotted or round, in the widest
// side's width and the top's color; else each side as the trapezoid it
// covers, meeting the next on the diagonals as CSS does.
func drawBorder(d *driver.Drawing, s *driver.ViewStyle, radii driver.Corners, continuous bool, w, h float32) {
	if s.Borders == (driver.Edges{}) {
		if s.BorderWidth <= 0 || s.BorderColor&0xff == 0 {
			return
		}
		b := float32(s.BorderWidth)
		stroke(d, shape(radii, continuous, w, h, b/2), s.BorderWidth, s.BorderColor, s.BorderStyle)
		return
	}
	width := func(p driver.Points, c driver.Color) float32 {
		if p <= 0 || c&0xff == 0 {
			return 0
		}
		return max(float32(p), 1)
	}
	b, c := s.Borders, s.BorderColors
	t, r, bo, l := width(b.Top, c.Top), width(b.Right, c.Right), width(b.Bottom, c.Bottom), width(b.Left, c.Left)
	if t == 0 && r == 0 && bo == 0 && l == 0 {
		return
	}
	same := t == r && t == bo && t == l && c.Top == c.Right && c.Top == c.Bottom && c.Top == c.Left
	if same || s.Rounded() || s.BorderStyle == driver.BorderDashed || s.BorderStyle == driver.BorderDotted {
		widest := max(t, r, bo, l)
		top := c.Top
		if top&0xff == 0 {
			top = max(c.Right, c.Bottom, c.Left)
		}
		stroke(d, shape(radii, continuous, w, h, widest/2), driver.Points(widest), top, s.BorderStyle)
		return
	}
	side := func(col driver.Color, pts ...float32) {
		if col&0xff == 0 {
			return
		}
		p := &driver.Path{}
		p.MoveTo(pts[0], pts[1])
		for i := 2; i < len(pts); i += 2 {
			p.LineTo(pts[i], pts[i+1])
		}
		p.ClosePath()
		d.SetFillColor(col)
		d.FillPath(p, driver.NonZero)
	}
	if t > 0 {
		side(c.Top, 0, 0, w, 0, w-r, t, l, t)
	}
	if r > 0 {
		side(c.Right, w, 0, w, h, w-r, h-bo, w-r, t)
	}
	if bo > 0 {
		side(c.Bottom, w, h, 0, h, l, h-bo, w-r, h-bo)
	}
	if l > 0 {
		side(c.Left, 0, h, 0, 0, l, t, l, h-bo)
	}
}

// stroke strokes p width wide in c, dashed as style says.
func stroke(d *driver.Drawing, p *driver.Path, width driver.Points, c driver.Color, style driver.BorderStyle) {
	d.Save()
	d.SetStrokeColor(c)
	d.SetLineWidth(float32(width))
	if on, off, round, ok := style.Dashes(width); ok {
		d.SetLineDash(float32(on), float32(off))
		if round {
			d.SetLineCap(driver.CapRound)
		}
	}
	d.StrokePath(p)
	d.Restore()
}

// canvasGradient is p, whose geometry is in fractions of the box, as a
// canvas gradient over a w by h box. A radial gradient's ellipse is a
// circle stretched by the transform.
func canvasGradient(p *driver.GradientPaint, w, h float32) *driver.CanvasGradient {
	g := &driver.CanvasGradient{Stops: p.Stops}
	switch p.Kind {
	case driver.GradientRadial:
		rx, ry := max(p.RX*w, 1e-3), max(p.RY*h, 1e-3)
		g.Radial = true
		g.X1, g.Y1, g.X2, g.Y2, g.R2 = 0, 0, 0, 0, rx
		// The unit circle of radius rx, scaled to ry vertically, around
		// the center.
		g.Transform = [6]float32{1, 0, 0, ry / rx, p.CX * w, p.CY * h}
	case driver.GradientConic:
		g.Conic = true
		g.X1, g.Y1 = p.CX*w, p.CY*h
		// Degrees clockwise from up, to radians from the x axis.
		g.Angle = (p.Angle - 90) * 3.14159265 / 180
	default:
		g.X1, g.Y1, g.X2, g.Y2 = p.X0*w, p.Y0*h, p.X1*w, p.Y1*h
	}
	return g
}

// inkOf is where a w by h box styled s draws, in its own coordinates:
// its bounds grown by its shadows and outline.
func inkOf(s *driver.ViewStyle, w, h float32) driver.Bounds {
	x0, y0, x1, y1 := float32(0), float32(0), w, h
	grow := func(dx, dy, r float32) {
		x0, y0 = min(x0, dx-r), min(y0, dy-r)
		x1, y1 = max(x1, w+dx+r), max(y1, h+dy+r)
	}
	if s.ShadowColor&0xff != 0 {
		grow(float32(s.ShadowOffset.X), float32(s.ShadowOffset.Y), 2*float32(s.ShadowRadius))
	}
	if e := s.Effects; e != nil {
		for _, b := range e.Shadows {
			if !b.Inset {
				grow(float32(b.Offset.X), float32(b.Offset.Y), float32(b.Blur+b.Spread))
			}
		}
	}
	if o := s.Outline; o.Visible() {
		grow(0, 0, float32(o.Offset+o.Width))
	}
	return driver.Bounds{Origin: driver.Point{X: driver.Points(x0), Y: driver.Points(y0)},
		Size: driver.Size{Width: driver.Points(x1 - x0), Height: driver.Points(y1 - y0)}}
}

// paints reports whether a box styled s draws anything of its own.
func paints(s *driver.ViewStyle) bool {
	if s.Background&0xff != 0 || s.ShadowColor&0xff != 0 && s.ShadowRadius > 0 || s.Outline.Visible() {
		return true
	}
	if s.BorderWidth > 0 && s.BorderColor&0xff != 0 || s.Borders != (driver.Edges{}) {
		return true
	}
	return s.Effects != nil && (len(s.Effects.Gradients) > 0 || len(s.Effects.Shadows) > 0)
}
