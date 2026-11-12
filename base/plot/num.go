package plot

import (
	"math"

	"github.com/nsgo-dev/nsgo/core"
)

// num is a number of a drawing: a constant, or one the platform's frame
// clock moves.
type num struct {
	v float32
	n core.AnimatedNode
}

func constNum(v float32) num { return num{v: v} }

func nodeNum(n core.AnimatedNode) num { return num{n: n} }

func (a num) operand() core.AnimatedNode {
	if a.n != nil {
		return a.n
	}
	return core.Constant(a.v)
}

func (a num) add(b num) num {
	if a.n == nil && b.n == nil {
		return constNum(a.v + b.v)
	}
	return nodeNum(core.Add(a.operand(), b.operand()))
}

func (a num) sub(b num) num {
	if a.n == nil && b.n == nil {
		return constNum(a.v - b.v)
	}
	return nodeNum(core.Sub(a.operand(), b.operand()))
}

func (a num) mul(b num) num {
	if a.n == nil && b.n == nil {
		return constNum(a.v * b.v)
	}
	return nodeNum(core.Mul(a.operand(), b.operand()))
}

// atMost is a no greater than limit, for a of at least 0.
func (a num) atMost(limit float32) num {
	if a.n == nil {
		return constNum(min(a.v, limit))
	}
	l := float64(limit)
	return nodeNum(core.Interpolate(a.n, core.Interpolation{Input: []float64{0, l}, Output: []float64{0, l}}))
}

// lerpNum is from moving to to as t goes from 0 to 1, a constant where it
// does not move.
func lerpNum(t core.AnimatedNode, from, to float32) num {
	if t == nil || from == to {
		return constNum(to)
	}
	return constNum(to - from).mul(nodeNum(t)).add(constNum(from))
}

// bind binds the last operation's argument arg to n, if it moves.
func bind(d *core.Drawing, arg core.DrawArg, n num) {
	if n.n != nil {
		d.Bind(arg, n.n)
	}
}

func moveTo(d *core.Drawing, x, y num) {
	d.MoveTo(x.v, y.v)
	bind(d, core.ArgX, x)
	bind(d, core.ArgY, y)
}

func lineTo(d *core.Drawing, x, y num) {
	d.LineTo(x.v, y.v)
	bind(d, core.ArgX, x)
	bind(d, core.ArgY, y)
}

func cubicTo(d *core.Drawing, x1, y1, x2, y2, x, y num) {
	d.CubicTo(x1.v, y1.v, x2.v, y2.v, x.v, y.v)
	bind(d, core.ArgX1, x1)
	bind(d, core.ArgY1, y1)
	bind(d, core.ArgX2, x2)
	bind(d, core.ArgY2, y2)
	bind(d, core.ArgX, x)
	bind(d, core.ArgY, y)
}

func arcTo(d *core.Drawing, x, y, r, a0, a1 num, ccw bool) {
	d.Arc(x.v, y.v, r.v, a0.v, a1.v, ccw)
	bind(d, core.ArgX, x)
	bind(d, core.ArgY, y)
	bind(d, core.ArgR, r)
	bind(d, core.ArgA0, a0)
	bind(d, core.ArgA1, a1)
}

func setAlpha(d *core.Drawing, alpha num) {
	d.SetAlpha(alpha.v)
	bind(d, core.ArgA, alpha)
}

// Corners is the radius of each corner of a mark's rectangles; a radius
// larger than half a rectangle is made to fit it, as a canvas's roundRect
// makes it fit.
type Corners struct {
	TopLeft, TopRight, BottomRight, BottomLeft core.Points
}

// AllCorners is one radius for every corner.
func AllCorners(r core.Points) Corners { return Corners{r, r, r, r} }

func (c Corners) square() bool { return c == Corners{} }

// rectPath adds the rectangle x, y, w, h with corners c. A moving rectangle
// with corners draws its own arcs, each radius fitted to half the
// rectangle's sides, so they stay round as it moves.
func rectPath(d *core.Drawing, x, y, w, h num, c Corners) {
	moves := x.n != nil || y.n != nil || w.n != nil || h.n != nil
	if !moves {
		if c.square() {
			d.Rect(x.v, y.v, w.v, h.v)
		} else {
			d.RoundRectCorners(x.v, y.v, w.v, h.v, float32(c.TopLeft), float32(c.TopRight), float32(c.BottomRight), float32(c.BottomLeft))
		}
		return
	}
	if c.square() {
		d.Rect(x.v, y.v, w.v, h.v)
		bind(d, core.ArgX, x)
		bind(d, core.ArgY, y)
		bind(d, core.ArgW, w)
		bind(d, core.ArgH, h)
		return
	}
	half := func(r core.Points) num {
		if r <= 0 {
			return constNum(0)
		}
		// The least of r and half each side, sides of at least 0.
		return w.mul(constNum(0.5)).atMostOf(h.mul(constNum(0.5)), float32(r))
	}
	tl, tr, br, bl := half(c.TopLeft), half(c.TopRight), half(c.BottomRight), half(c.BottomLeft)
	right, bottom := x.add(w), y.add(h)
	const q = math.Pi / 2
	moveTo(d, x.add(tl), y)
	lineTo(d, right.sub(tr), y)
	arcTo(d, right.sub(tr), y.add(tr), tr, constNum(-q), constNum(0), false)
	lineTo(d, right, bottom.sub(br))
	arcTo(d, right.sub(br), bottom.sub(br), br, constNum(0), constNum(q), false)
	lineTo(d, x.add(bl), bottom)
	arcTo(d, x.add(bl), bottom.sub(bl), bl, constNum(q), constNum(2*q), false)
	lineTo(d, x, y.add(tl))
	arcTo(d, x.add(tl), y.add(tl), tl, constNum(2*q), constNum(3*q), false)
	d.ClosePath()
}

// atMostOf is the least of a, b and limit, for a and b of at least 0.
func (a num) atMostOf(b num, limit float32) num {
	a, b = a.atMost(limit), b.atMost(limit)
	if a.n == nil && b.n == nil {
		return constNum(min(a.v, b.v))
	}
	if a.n == nil {
		return b.atMost(a.v)
	}
	if b.n == nil {
		return a.atMost(b.v)
	}
	// (a + b - |a - b|) / 2, |x| an interpolation through 0, exact as
	// both are within limit.
	l := float64(limit)
	abs := core.Interpolate(a.sub(b).n, core.Interpolation{Input: []float64{-l, 0, l}, Output: []float64{l, 0, l}})
	return a.add(b).sub(nodeNum(abs)).mul(constNum(0.5))
}
