package imagedecode

import (
	"image"
	"math"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// affine is a 2D transform as SVG's matrix(a, b, c, d, e, f).
type affine struct{ a, b, c, d, e, f float64 }

var identity = affine{a: 1, d: 1}

// mul returns the transform that applies n, then m.
func (m affine) mul(n affine) affine {
	return affine{
		a: m.a*n.a + m.c*n.b,
		b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d,
		d: m.b*n.c + m.d*n.d,
		e: m.a*n.e + m.c*n.f + m.e,
		f: m.b*n.e + m.d*n.f + m.f,
	}
}

func (m affine) apply(x, y float64) point {
	return point{m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f}
}

func (m affine) det() float64 { return m.a*m.d - m.b*m.c }

func (m affine) invert() (affine, bool) {
	det := m.det()
	if det == 0 || math.IsNaN(det) || math.IsInf(det, 0) {
		return affine{}, false
	}
	return affine{
		a: m.d / det, b: -m.b / det, c: -m.c / det, d: m.a / det,
		e: (m.c*m.f - m.d*m.e) / det, f: (m.b*m.e - m.a*m.f) / det,
	}, true
}

// segment is a piece of the current path, in pixels.
type segment struct {
	kind uint8 // segMove, segLine, segCubic or segClose
	pts  [3]point
}

const (
	segMove uint8 = iota
	segLine
	segCubic
	segClose
)

// paintState is what DrawSave saves.
type paintState struct {
	ctm                  affine
	fill, stroke         driver.Color
	fillGrad, strokeGrad *driver.CanvasGradient
	width                float64
	cap                  driver.LineCap
	join                 driver.LineJoin
	alpha                float64
	clip                 *clipMask // nil clips nothing
}

// player plays a core.Drawing into premultiplied RGBA pixels, as the
// platforms' canvases play it. It draws no text.
type player struct {
	w, h  int
	pix   []byte
	st    paintState
	stack []paintState
	path  []segment
	start point // the current subpath's start
	cur   point
	has   bool // whether there is a current point
	r     *rasterizer
	over  bool // whether the work went over budget
	// clipPixels counts the pixels of the clip masks made.
	clipPixels int
}

// rasterize plays d into a w by h raster of premultiplied RGBA.
func rasterize(d *driver.Drawing, w, h int) ([]byte, error) {
	p := &player{
		w: w, h: h,
		pix: make([]byte, w*h*4),
		st:  paintState{ctm: identity, fill: driver.RGB(0), stroke: driver.RGB(0), width: 1, alpha: 1},
		r:   newRasterizer(w, h),
	}
	for i := range d.Ops {
		p.play(&d.Ops[i])
		if p.over {
			return nil, ErrTooComplex
		}
	}
	return p.pix, nil
}

func (p *player) play(op *driver.DrawOp) {
	x, y := float64(op.X), float64(op.Y)
	switch op.Kind {
	case driver.DrawSave:
		p.stack = append(p.stack, p.st)
	case driver.DrawRestore:
		if n := len(p.stack); n > 0 {
			p.st = p.stack[n-1]
			p.stack = p.stack[:n-1]
		}
	case driver.DrawTranslate:
		p.st.ctm = p.st.ctm.mul(affine{a: 1, d: 1, e: x, f: y})
	case driver.DrawScale:
		p.st.ctm = p.st.ctm.mul(affine{a: x, d: y})
	case driver.DrawRotate:
		sin, cos := math.Sincos(float64(op.A))
		p.st.ctm = p.st.ctm.mul(affine{a: cos, b: sin, c: -sin, d: cos})
	case driver.DrawBeginPath:
		p.path, p.has = p.path[:0], false
	case driver.DrawMoveTo:
		p.moveTo(p.st.ctm.apply(x, y))
	case driver.DrawLineTo:
		p.lineTo(p.st.ctm.apply(x, y))
	case driver.DrawQuadTo:
		q := p.st.ctm.apply(float64(op.X1), float64(op.Y1))
		if !p.has {
			p.moveTo(q)
		}
		e := p.st.ctm.apply(x, y)
		p.cubicTo(lerp(p.cur, q, 2.0/3), lerp(e, q, 2.0/3), e)
	case driver.DrawCubicTo:
		c1 := p.st.ctm.apply(float64(op.X1), float64(op.Y1))
		if !p.has {
			p.moveTo(c1)
		}
		p.cubicTo(c1, p.st.ctm.apply(float64(op.X2), float64(op.Y2)), p.st.ctm.apply(x, y))
	case driver.DrawArc:
		p.arc(op)
	case driver.DrawRect:
		w, h := float64(op.W), float64(op.H)
		m := p.st.ctm
		p.moveTo(m.apply(x, y))
		p.lineTo(m.apply(x+w, y))
		p.lineTo(m.apply(x+w, y+h))
		p.lineTo(m.apply(x, y+h))
		p.closePath()
		p.moveTo(m.apply(x, y))
	case driver.DrawRoundRect:
		w, h := float64(op.W), float64(op.H)
		r := max(min(float64(op.R), math.Abs(w)/2, math.Abs(h)/2), 0)
		p.moveTo(p.st.ctm.apply(x+r, y))
		p.lineTo(p.st.ctm.apply(x+w-r, y))
		p.ellipseArc(x+w-r, y+r, r, r, -math.Pi/2, math.Pi/2)
		p.lineTo(p.st.ctm.apply(x+w, y+h-r))
		p.ellipseArc(x+w-r, y+h-r, r, r, 0, math.Pi/2)
		p.lineTo(p.st.ctm.apply(x+r, y+h))
		p.ellipseArc(x+r, y+h-r, r, r, math.Pi/2, math.Pi/2)
		p.lineTo(p.st.ctm.apply(x, y+r))
		p.ellipseArc(x+r, y+r, r, r, math.Pi, math.Pi/2)
		p.closePath()
	case driver.DrawEllipse:
		rx, ry := float64(op.R), float64(op.RY)
		p.moveTo(p.st.ctm.apply(x+rx, y))
		p.ellipseArc(x, y, rx, ry, 0, 2*math.Pi)
		p.closePath()
	case driver.DrawClosePath:
		p.closePath()
	case driver.DrawFill:
		p.fill(op.EvenOdd)
	case driver.DrawStroke:
		p.stroke()
	case driver.DrawClip:
		p.clip(op.EvenOdd)
	case driver.DrawFillColor:
		p.st.fill, p.st.fillGrad = op.Color, nil
	case driver.DrawStrokeColor:
		p.st.stroke, p.st.strokeGrad = op.Color, nil
	case driver.DrawFillGradient:
		p.st.fillGrad = op.Gradient
	case driver.DrawStrokeGradient:
		p.st.strokeGrad = op.Gradient
	case driver.DrawLineWidth:
		p.st.width = float64(op.W)
	case driver.DrawLineCap:
		p.st.cap = op.Cap
	case driver.DrawLineJoin:
		p.st.join = op.Join
	case driver.DrawAlpha:
		p.st.alpha = min(max(float64(op.A), 0), 1)
	case driver.DrawClear:
		p.clear(x, y, float64(op.W), float64(op.H))
	}
}

func (p *player) moveTo(pt point) {
	p.path = append(p.path, segment{kind: segMove, pts: [3]point{pt}})
	p.start, p.cur, p.has = pt, pt, true
}

func (p *player) lineTo(pt point) {
	if !p.has {
		p.moveTo(pt)
		return
	}
	p.path = append(p.path, segment{kind: segLine, pts: [3]point{pt}})
	p.cur = pt
}

func (p *player) cubicTo(c1, c2, e point) {
	p.path = append(p.path, segment{kind: segCubic, pts: [3]point{c1, c2, e}})
	p.cur = e
}

// closePath closes the subpath; the next one starts where it started.
func (p *player) closePath() {
	if !p.has {
		return
	}
	p.path = append(p.path, segment{kind: segClose})
	p.path = append(p.path, segment{kind: segMove, pts: [3]point{p.start}})
	p.cur = p.start
}

// ellipseArc adds an arc of the ellipse around (cx, cy) in user space from
// angle a0 by sweep, as cubic curves of at most a quarter turn.
func (p *player) ellipseArc(cx, cy, rx, ry, a0, sweep float64) {
	n := max(int(math.Ceil(math.Abs(sweep)/(math.Pi/2)-1e-9)), 1)
	step := sweep / float64(n)
	k := 4.0 / 3.0 * math.Tan(step/4)
	m := p.st.ctm
	for i := range n {
		t0 := a0 + float64(i)*step
		t1 := t0 + step
		s0, c0 := math.Sincos(t0)
		s1, c1 := math.Sincos(t1)
		p.cubicTo(
			m.apply(cx+rx*(c0-k*s0), cy+ry*(s0+k*c0)),
			m.apply(cx+rx*(c1+k*s1), cy+ry*(s1-k*c1)),
			m.apply(cx+rx*c1, cy+ry*s1))
	}
}

// arc adds an arc as the web's arc() does: joined to the path by a line.
func (p *player) arc(op *driver.DrawOp) {
	x, y, r := float64(op.X), float64(op.Y), float64(op.R)
	a0, a1 := float64(op.A0), float64(op.A1)
	const full = 2 * math.Pi
	mod := func(v float64) float64 { return v - full*math.Floor(v/full) }
	var sweep float64
	switch {
	case !op.CCW && a1-a0 >= full:
		sweep = full
	case !op.CCW:
		sweep = mod(a1 - a0)
	case a0-a1 >= full:
		sweep = -full
	default:
		sweep = -mod(a0 - a1)
	}
	s := p.st.ctm.apply(x+r*math.Cos(a0), y+r*math.Sin(a0))
	if p.has {
		p.lineTo(s)
	} else {
		p.moveTo(s)
	}
	if sweep != 0 {
		p.ellipseArc(x, y, r, r, a0, sweep)
	}
}

// flatten returns the path's subpaths as polylines. Curves wholly outside
// the raster by more than margin become lines.
func (p *player) flatten(margin float64) []subpath {
	var subs []subpath
	var cur point
	minX, minY := -margin, -margin
	maxX, maxY := float64(p.w)+margin, float64(p.h)+margin
	for _, s := range p.path {
		switch s.kind {
		case segMove:
			subs = append(subs, subpath{pts: []point{s.pts[0]}})
			cur = s.pts[0]
			continue
		case segClose:
			if len(subs) > 0 {
				subs[len(subs)-1].closed = true
			}
			continue
		}
		if len(subs) == 0 {
			subs = append(subs, subpath{pts: []point{cur}})
		}
		sub := &subs[len(subs)-1]
		sub.segments++
		if s.kind == segLine {
			sub.pts = append(sub.pts, s.pts[0])
			cur = s.pts[0]
			continue
		}
		c1, c2, e := s.pts[0], s.pts[1], s.pts[2]
		n := 1
		if max(cur.x, c1.x, c2.x, e.x) >= minX && min(cur.x, c1.x, c2.x, e.x) <= maxX &&
			max(cur.y, c1.y, c2.y, e.y) >= minY && min(cur.y, c1.y, c2.y, e.y) <= maxY {
			// Wang's formula: segments enough to stay within tolerance.
			dd := max(cur.sub(c1.mul(2)).add(c2).length(), c1.sub(c2.mul(2)).add(e).length())
			n = int(min(math.Ceil(math.Sqrt(0.75*dd/tolerance)), 4096))
			n = max(n, 1)
		}
		p.r.points += n
		if p.r.points > maxRasterPoints {
			p.over = true
			return nil
		}
		for i := 1; i <= n; i++ {
			t := float64(i) / float64(n)
			u := 1 - t
			sub.pts = append(sub.pts, point{
				u*u*u*cur.x + 3*u*u*t*c1.x + 3*u*t*t*c2.x + t*t*t*e.x,
				u*u*u*cur.y + 3*u*u*t*c1.y + 3*u*t*t*c2.y + t*t*t*e.y,
			})
		}
		cur = e
	}
	return subs
}

func (p *player) fill(evenOdd bool) {
	src := p.source(p.st.fill, p.st.fillGrad)
	if src == nil {
		return
	}
	for _, sub := range p.flatten(1) {
		if len(sub.pts) > 2 {
			p.r.polygon(sub.pts)
		}
	}
	p.cover(evenOdd, src)
}

// lineWidth returns the stroke width in pixels: the line width scaled by
// the transform's mean scale.
func (p *player) lineWidth() float64 {
	return p.st.width * math.Sqrt(math.Abs(p.st.ctm.det()))
}

func (p *player) stroke() {
	width := p.lineWidth()
	if !(width > 0) || math.IsInf(width, 0) {
		return
	}
	src := p.source(p.st.stroke, p.st.strokeGrad)
	if src == nil {
		return
	}
	for _, sub := range p.flatten(width*miterLimit + 1) {
		p.r.stroke(sub, width/2, p.st.cap, p.st.join)
	}
	p.cover(false, src)
}

// cover fills what was added to the rasterizer with src, within the clip.
func (p *player) cover(evenOdd bool, src *source) {
	clip := p.st.clip
	// What an opaque solid color writes where it covers a pixel whole.
	var opaque [4]byte
	solid := src.grad == nil && src.color[3] >= 1
	if solid {
		for ch := range 4 {
			opaque[ch] = uint8(src.color[ch]*255 + 0.5)
		}
	}
	ok := p.r.fill(evenOdd, func(y, x0 int, cov []float32) {
		row := p.pix[(y*p.w+x0)*4:]
		for i, c := range cov {
			if c <= 0 {
				continue
			}
			if solid && c >= 1 && clip == nil {
				copy(row[i*4:i*4+4], opaque[:])
				continue
			}
			if clip != nil {
				c *= clip.at(x0+i, y)
				if c <= 0 {
					continue
				}
			}
			col, ok := src.at(x0+i, y)
			if !ok {
				continue
			}
			sa := col[3] * c
			if sa <= 0 {
				continue
			}
			px := row[i*4 : i*4+4 : i*4+4]
			for ch := range 4 {
				v := col[ch]*c*255 + float32(px[ch])*(1-sa)
				px[ch] = uint8(min(max(v+0.5, 0), 255))
			}
		}
	})
	if !ok {
		p.over = true
	}
}

// maxClipPixels bounds the pixels of the clip masks one drawing makes.
const maxClipPixels = 1 << 28

// clipMask is the coverage of a clip, from 0 to 255, in a rectangle of
// the raster; outside it nothing shows.
type clipMask struct {
	rect image.Rectangle
	a    []uint8
}

func (m *clipMask) at(x, y int) float32 {
	if !(image.Point{x, y}).In(m.rect) {
		return 0
	}
	return float32(m.a[(y-m.rect.Min.Y)*m.rect.Dx()+x-m.rect.Min.X]) * (1.0 / 255)
}

// clip intersects the clip with the current path.
func (p *player) clip(evenOdd bool) {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, sub := range p.flatten(1) {
		if len(sub.pts) < 3 {
			continue
		}
		p.r.polygon(sub.pts)
		for _, pt := range sub.pts {
			if pt.finite() {
				x0, y0, x1, y1 = min(x0, pt.x), min(y0, pt.y), max(x1, pt.x), max(y1, pt.y)
			}
		}
	}
	var bounds image.Rectangle
	if x0 <= x1 {
		bounds = image.Rect(int(math.Floor(max(x0, -1))), int(math.Floor(max(y0, -1))),
			int(math.Ceil(min(x1, float64(p.w+1)))), int(math.Ceil(min(y1, float64(p.h+1)))))
	}
	bounds = bounds.Intersect(image.Rect(0, 0, p.w, p.h))
	old := p.st.clip
	if old != nil {
		bounds = bounds.Intersect(old.rect)
	}
	mask := &clipMask{rect: bounds, a: make([]uint8, bounds.Dx()*bounds.Dy())}
	if p.clipPixels += len(mask.a); p.clipPixels > maxClipPixels {
		p.over = true
		return
	}
	if !p.r.fill(evenOdd, func(y, from int, cov []float32) {
		for i, c := range cov {
			x := from + i
			if !(image.Point{x, y}).In(bounds) {
				continue
			}
			if old != nil {
				c *= old.at(x, y)
			}
			mask.a[(y-bounds.Min.Y)*bounds.Dx()+x-bounds.Min.X] = uint8(c*255 + 0.5)
		}
	}) {
		p.over = true
	}
	p.st.clip = mask
}

// clear makes a rectangle transparent, within the clip.
func (p *player) clear(x, y, w, h float64) {
	m := p.st.ctm
	p.r.polygon([]point{m.apply(x, y), m.apply(x+w, y), m.apply(x+w, y+h), m.apply(x, y+h)})
	clip := p.st.clip
	if !p.r.fill(false, func(row, x0 int, cov []float32) {
		for i, c := range cov {
			if clip != nil {
				c *= clip.at(x0+i, row)
			}
			px := p.pix[(row*p.w+x0+i)*4:][:4]
			for ch := range 4 {
				px[ch] = uint8(float32(px[ch])*(1-c) + 0.5)
			}
		}
	}) {
		p.over = true
	}
}

// lutSize is how many colors a gradient's lookup table holds.
const lutSize = 1024

// source is a fill or stroke paint at the global alpha: one color, or a
// gradient mapped to pixels.
type source struct {
	color [4]float32 // premultiplied, from 0 to 1
	grad  *driver.CanvasGradient
	inv   affine // from pixels to gradient space
	lut   []([4]float32)
}

// source returns the paint of color or g, or nil when it paints nothing.
func (p *player) source(color driver.Color, g *driver.CanvasGradient) *source {
	alpha := float32(p.st.alpha)
	if g == nil || len(g.Stops) == 0 {
		c := premultiply(color, alpha)
		if c[3] <= 0 {
			return nil
		}
		return &source{color: c}
	}
	last := premultiply(g.Stops[len(g.Stops)-1].Color, alpha)
	m := affine{float64(g.Transform[0]), float64(g.Transform[1]), float64(g.Transform[2]),
		float64(g.Transform[3]), float64(g.Transform[4]), float64(g.Transform[5])}
	if g.Transform == [6]float32{} {
		m = identity
	}
	inv, ok := p.st.ctm.mul(m).invert()
	degenerate := g.X1 == g.X2 && g.Y1 == g.Y2
	if g.Radial {
		degenerate = g.R2 <= 0 && g.R1 <= 0
	}
	if !ok || degenerate || len(g.Stops) == 1 {
		return &source{color: last}
	}
	s := &source{grad: g, inv: inv, lut: make([][4]float32, lutSize)}
	stops := g.Stops
	j := 0
	for i := range s.lut {
		t := float32(i) / (lutSize - 1)
		for j < len(stops)-1 && stops[j+1].Offset < t {
			j++
		}
		var c [4]float32
		switch {
		case t <= stops[0].Offset:
			c = premultiply(stops[0].Color, alpha)
		case j >= len(stops)-1:
			c = last
		default:
			a, b := stops[j], stops[j+1]
			f := float32(0)
			if b.Offset > a.Offset {
				f = (t - a.Offset) / (b.Offset - a.Offset)
			}
			ca, cb := premultiply(a.Color, alpha), premultiply(b.Color, alpha)
			for ch := range 4 {
				c[ch] = ca[ch] + (cb[ch]-ca[ch])*f
			}
		}
		s.lut[i] = c
	}
	return s
}

// at returns the color of pixel (x, y); false where the paint shows
// nothing.
func (s *source) at(x, y int) ([4]float32, bool) {
	if s.grad == nil {
		return s.color, true
	}
	g := s.grad
	q := s.inv.apply(float64(x)+0.5, float64(y)+0.5)
	var t float64
	if !g.Radial {
		dx, dy := float64(g.X2-g.X1), float64(g.Y2-g.Y1)
		t = ((q.x-float64(g.X1))*dx + (q.y-float64(g.Y1))*dy) / (dx*dx + dy*dy)
	} else {
		var ok bool
		if t, ok = radialOffset(g, q); !ok {
			return [4]float32{}, false
		}
	}
	switch g.Spread {
	case driver.SpreadRepeat:
		t -= math.Floor(t)
	case driver.SpreadReflect:
		t = math.Abs(t - 2*math.Floor(t/2+0.5))
	default:
		t = min(max(t, 0), 1)
	}
	if math.IsNaN(t) {
		return [4]float32{}, false
	}
	return s.lut[int(t*(lutSize-1)+0.5)], true
}

// radialOffset solves for the largest t whose circle, interpolated from
// the focal circle to the outer one, passes through q, as the web's
// two-circle radial gradients do.
func radialOffset(g *driver.CanvasGradient, q point) (float64, bool) {
	cdx, cdy := float64(g.X2-g.X1), float64(g.Y2-g.Y1)
	r0, dr := float64(g.R1), float64(g.R2-g.R1)
	px, py := q.x-float64(g.X1), q.y-float64(g.Y1)
	a := cdx*cdx + cdy*cdy - dr*dr
	b := px*cdx + py*cdy + r0*dr
	c := px*px + py*py - r0*r0
	if math.Abs(a) < 1e-9 {
		if b == 0 {
			return 0, false
		}
		t := c / (2 * b)
		return t, r0+t*dr >= 0
	}
	disc := b*b - a*c
	if disc < 0 {
		return 0, false
	}
	sq := math.Sqrt(disc)
	t1, t2 := (b+sq)/a, (b-sq)/a
	if t1 < t2 {
		t1, t2 = t2, t1
	}
	if r0+t1*dr >= 0 {
		return t1, true
	}
	return t2, r0+t2*dr >= 0
}

// premultiply returns c's channels from 0 to 1, premultiplied, times
// alpha.
func premultiply(c driver.Color, alpha float32) [4]float32 {
	r, g, b, a := c.Components()
	fa := float32(a) * alpha
	return [4]float32{float32(r) * fa, float32(g) * fa, float32(b) * fa, fa}
}
