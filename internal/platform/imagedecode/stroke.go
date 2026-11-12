package imagedecode

import (
	"math"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// miterLimit is SVG's and the web canvas's default: longer miters are
// beveled.
const miterLimit = 4

// tolerance is how far, in pixels, flattened curves and arcs may stray
// from the true ones.
const tolerance = 0.05

// subpath is a flattened subpath in pixels.
type subpath struct {
	pts    []point
	closed bool
	// segments counts what drew it, so a subpath of zero length drawn by
	// a segment still gets caps, while a lone move gets none.
	segments int
}

// stroke adds the outline of sub stroked with half width hw as convex
// pieces: a quadrilateral per segment, a piece per join and per cap. Wound
// alike, they fill their union by the nonzero rule.
func (r *rasterizer) stroke(sub subpath, hw float64, lineCap driver.LineCap, join driver.LineJoin) {
	pts := r.tmp[:0]
	for _, p := range sub.pts {
		if !p.finite() {
			continue
		}
		if len(pts) == 0 || !p.near(pts[len(pts)-1]) {
			pts = append(pts, p)
		}
	}
	r.tmp = pts
	closed := sub.closed
	if closed && len(pts) > 1 && pts[0].near(pts[len(pts)-1]) {
		pts = pts[:len(pts)-1]
	}
	n := len(pts)
	if n == 0 {
		return
	}
	if n == 1 {
		// A segment of zero length shows its caps, as SVG has it.
		if sub.segments == 0 {
			return
		}
		switch lineCap {
		case driver.CapRound:
			r.circle(pts[0], hw)
		case driver.CapSquare:
			p := pts[0]
			r.convex(point{p.x - hw, p.y - hw}, point{p.x + hw, p.y - hw},
				point{p.x + hw, p.y + hw}, point{p.x - hw, p.y + hw})
		}
		return
	}
	segments := n - 1
	if closed {
		segments = n
	}
	for i := range segments {
		a, b := pts[i], pts[(i+1)%n]
		nrm := b.sub(a).unit().perp().mul(hw)
		r.convex(a.add(nrm), b.add(nrm), b.sub(nrm), a.sub(nrm))
	}
	first, last := 1, n-1
	if closed {
		first, last = 0, n
	}
	for i := first; i < last; i++ {
		p := pts[i]
		d0 := p.sub(pts[(i-1+n)%n]).unit()
		d1 := pts[(i+1)%n].sub(p).unit()
		r.join(p, d0, d1, hw, join)
	}
	if !closed {
		r.lineCap(pts[0], pts[0].sub(pts[1]).unit(), hw, lineCap)
		r.lineCap(pts[n-1], pts[n-1].sub(pts[n-2]).unit(), hw, lineCap)
	}
}

// join adds the join at p between a segment going d0 and one going d1.
func (r *rasterizer) join(p, d0, d1 point, hw float64, join driver.LineJoin) {
	cross, dot := d0.cross(d1), d0.dot(d1)
	if math.Abs(cross) < 1e-12 {
		if dot < 0 && join == driver.JoinRound {
			r.circle(p, hw) // turning back
		}
		return
	}
	// The outer side is the one the path turns away from.
	n0, n1 := d0.perp().mul(hw), d1.perp().mul(hw)
	if cross > 0 {
		n0, n1 = n0.mul(-1), n1.mul(-1)
	}
	// cos is the cosine of half the turn: the miter is hw / cos long.
	cos := math.Sqrt(max((1+dot)/2, 0))
	switch join {
	case driver.JoinMiter:
		if cos > 0 && 1/cos <= miterLimit {
			tip := n0.add(n1).unit().mul(hw / cos)
			r.convex(p, p.add(n0), p.add(tip), p.add(n1))
			return
		}
	case driver.JoinRound:
		if hw*(1-cos) > tolerance {
			r.sector(p, n0, n1, hw)
			return
		}
	}
	r.convex(p, p.add(n0), p.add(n1))
}

// lineCap adds the cap at p of a line ending in direction d.
func (r *rasterizer) lineCap(p, d point, hw float64, lineCap driver.LineCap) {
	switch lineCap {
	case driver.CapSquare:
		n, e := d.perp().mul(hw), d.mul(hw)
		r.convex(p.add(n), p.add(n).add(e), p.sub(n).add(e), p.sub(n))
	case driver.CapRound:
		n := d.perp().mul(hw)
		r.sector(p, n, d.mul(hw), hw)
		r.sector(p, d.mul(hw), n.mul(-1), hw)
	}
}

// arcStep returns the angle between points of a flattened arc of radius
// radius.
func arcStep(radius float64) float64 {
	if radius <= tolerance {
		return math.Pi / 4
	}
	return min(2*math.Acos(1-tolerance/radius), math.Pi/4)
}

// sector adds the pie slice around p from offset a to offset b, both
// radius long, the short way round.
func (r *rasterizer) sector(p, a, b point, radius float64) {
	a0 := math.Atan2(a.y, a.x)
	sweep := math.Atan2(a.cross(b), a.dot(b))
	steps := min(int(math.Ceil(math.Abs(sweep)/arcStep(radius))), 256)
	pts := make([]point, 0, steps+2)
	pts = append(pts, p, p.add(a))
	for i := 1; i < steps; i++ {
		s, c := math.Sincos(a0 + sweep*float64(i)/float64(steps))
		pts = append(pts, point{p.x + radius*c, p.y + radius*s})
	}
	pts = append(pts, p.add(b))
	r.convex(pts...)
}

// circle adds a disc around p.
func (r *rasterizer) circle(p point, radius float64) {
	steps := min(max(int(math.Ceil(2*math.Pi/arcStep(radius))), 8), 1024)
	pts := make([]point, steps)
	for i := range pts {
		s, c := math.Sincos(2 * math.Pi * float64(i) / float64(steps))
		pts[i] = point{p.x + radius*c, p.y + radius*s}
	}
	r.convex(pts...)
}
