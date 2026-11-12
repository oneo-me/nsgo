package flow

import (
	"math"

	"github.com/nsgo-dev/nsgo/core"
)

const (
	defaultEdgeWidth = 1.5
	mutedEdgeOpacity = .35
	cornerRadius     = 12
	cornerSteps      = 6
	arrowLength      = 6
	arrowHalfWidth   = 3.5
	dashLength       = 4
	dashGap          = 5
	labelSize        = 11
	labelPadX        = 6
	labelPadY        = 2
	// canvasBleed is the room around the diagram its strokes and labels
	// may reach into.
	canvasBleed = 32
)

var labelFont = core.Font{Size: labelSize, Weight: 500}

// edgeCanvas draws the edges beneath the nodes, emphasizing those of the
// hovered and selected nodes. While nodes move to a new layout each point
// of a route follows the end it is nearer along the route: its arguments
// are bound to the nodes' places, so the platform moves the edges with
// the nodes on its own frame clock.
func (v *view) edgeCanvas() core.Node {
	return core.Canvas{
		StyleFunc: func() core.Style {
			v.laidOut.Get()
			return core.Style{Position: core.Absolute, Left: core.Pt(-canvasBleed), Top: core.Pt(-canvasBleed),
				Width: core.Pt(core.Points(v.result.width) + 2*canvasBleed), Height: core.Pt(core.Points(v.result.height) + 2*canvasBleed)}
		},
		Draw: func(d *core.Drawing, _ core.Size) {
			v.laidOut.Get()
			v.measured.Get()
			look := v.look()
			emphasized := map[int]bool{}
			for _, id := range [...]string{v.hovered.Get(), v.selected()} {
				if i, ok := v.index[id]; ok && id != "" {
					emphasized[i] = true
				}
			}
			moving := v.moving.Get()
			d.Translate(canvasBleed, canvasBleed)
			d.SetLineCap(core.CapRound).SetLineJoin(core.JoinRound)
			var routes []*route
			for i, e := range v.edges {
				routes = append(routes, v.route(i, e, moving))
			}
			for pass := range 2 {
				for i, e := range v.edges {
					r := routes[i]
					if r == nil || (emphasized[e.source] || emphasized[e.target]) != (pass == 1) {
						continue
					}
					color, width := e.color, e.width
					if width <= 0 {
						width = defaultEdgeWidth
					}
					if pass == 1 {
						width += .5
						if color == 0 {
							color = look.Emphasis
						}
					}
					if color == 0 {
						color = look.Line
					}
					if v.nodes[e.source].muted || v.nodes[e.target].muted {
						color = color.Faded(mutedEdgeOpacity)
					}
					r.draw(d, float32(width), color, e.dashed)
				}
			}
			for i, e := range v.edges {
				if r := routes[i]; r != nil && e.label != "" {
					alpha := float32(1)
					if v.nodes[e.source].muted || v.nodes[e.target].muted {
						alpha = mutedEdgeOpacity
					}
					r.label(d, e.label, look, alpha)
				}
			}
		}}
}

// route is an edge's laid out points and, while its ends move, the nodes
// that follow them: a point at weight w along the route shows at
// itself + s + w·(t - s), where s and t are how far its source and target
// show from their laid out places.
type route struct {
	points  []point
	weights []float64
	s       [2]core.AnimatedNode
	diff    [2]core.AnimatedNode
	t       [2]core.AnimatedNode
}

func (v *view) route(i int, e renderEdge, moving bool) *route {
	if i >= len(v.result.routes) || !v.nodes[e.source].measured || !v.nodes[e.target].measured {
		return nil
	}
	laid := v.result.routes[i]
	if len(laid) < 2 {
		return nil
	}
	r := &route{points: laid, weights: make([]float64, len(laid))}
	total := 0.0
	for k := 1; k < len(laid); k++ {
		total += math.Hypot(laid[k].x-laid[k-1].x, laid[k].y-laid[k-1].y)
	}
	along := 0.0
	for k := range laid {
		if k > 0 {
			along += math.Hypot(laid[k].x-laid[k-1].x, laid[k].y-laid[k-1].y)
		}
		if total > 0 {
			r.weights[k] = along / total
		}
	}
	if moving {
		sa, ta := v.anim(v.nodes[e.source].id), v.anim(v.nodes[e.target].id)
		r.s = [2]core.AnimatedNode{core.Sub(sa.x, core.Constant(v.result.x[e.source])), core.Sub(sa.y, core.Constant(v.result.y[e.source]))}
		r.t = [2]core.AnimatedNode{core.Sub(ta.x, core.Constant(v.result.x[e.target])), core.Sub(ta.y, core.Constant(v.result.y[e.target]))}
		r.diff = [2]core.AnimatedNode{core.Sub(r.t[0], r.s[0]), core.Sub(r.t[1], r.s[1])}
	}
	return r
}

// at binds the last operation's arguments ax and ay to where p, at weight
// w along the route, shows under the source's shift.
func (r *route) at(d *core.Drawing, ax, ay core.DrawArg, p point, w float64) {
	if r.diff[0] == nil || w == 0 {
		return
	}
	for axis, arg := range [2]core.DrawArg{ax, ay} {
		c := p.x
		if axis == 1 {
			c = p.y
		}
		d.Bind(arg, core.Interpolate(r.diff[axis], core.Interpolation{Input: []float64{0, 1}, Output: []float64{c, c + w},
			Extrapolate: core.ExtrapolateExtend}))
	}
}

// shifted runs draw under the translation of the point at weight w.
func (r *route) shifted(d *core.Drawing, w float64, draw func()) {
	d.Save()
	if r.s[0] != nil {
		d.Translate(0, 0)
		for axis, arg := range [2]core.DrawArg{core.ArgX, core.ArgY} {
			n := r.s[axis]
			if w > 0 {
				n = core.Add(n, core.Mul(r.diff[axis], core.Constant(w)))
			}
			d.Bind(arg, n)
		}
	}
	draw()
	d.Restore()
}

func (r *route) moveTo(d *core.Drawing, p point, w float64) {
	d.MoveTo(float32(p.x), float32(p.y))
	r.at(d, core.ArgX, core.ArgY, p, w)
}

func (r *route) lineTo(d *core.Drawing, p point, w float64) {
	d.LineTo(float32(p.x), float32(p.y))
	r.at(d, core.ArgX, core.ArgY, p, w)
}

func (r *route) quadTo(d *core.Drawing, c point, cw float64, p point, w float64) {
	d.QuadTo(float32(c.x), float32(c.y), float32(p.x), float32(p.y))
	r.at(d, core.ArgX1, core.ArgY1, c, cw)
	r.at(d, core.ArgX, core.ArgY, p, w)
}

// draw strokes the route with rounded bends, ending in an arrowhead.
func (r *route) draw(d *core.Drawing, width float32, c core.Color, dashed bool) {
	points := r.points
	n := len(points)
	end, before := points[n-1], points[n-2]
	dx, dy := end.x-before.x, end.y-before.y
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	ux, uy := dx/length, dy/length
	head := math.Min(arrowLength, length)
	line := append([]point(nil), points...)
	line[n-1] = point{end.x - ux*head*.8, end.y - uy*head*.8}
	weights := append([]float64(nil), r.weights...)
	r.shifted(d, 0, func() {
		d.SetStrokeColor(c).SetLineWidth(width)
		if dashed {
			r.dashes(d, line, weights)
		} else {
			d.BeginPath()
			r.moveTo(d, line[0], weights[0])
			corners(line, weights, func(a, control, b point, wa, wc, wb float64) {
				r.lineTo(d, a, wa)
				r.quadTo(d, control, wc, b, wb)
			})
			r.lineTo(d, line[n-1], weights[n-1])
			d.Stroke()
		}
		d.SetLineDash()
	})
	// The arrowhead moves with the target; the stroke's round joins soften
	// its points.
	r.shifted(d, 1, func() {
		d.SetFillColor(c).SetStrokeColor(c).SetLineWidth(1)
		d.BeginPath().MoveTo(float32(end.x), float32(end.y)).
			LineTo(float32(end.x-ux*head-uy*arrowHalfWidth), float32(end.y-uy*head+ux*arrowHalfWidth)).
			LineTo(float32(end.x-ux*head+uy*arrowHalfWidth), float32(end.y-uy*head-ux*arrowHalfWidth)).
			ClosePath().Fill().Stroke()
	})
}

// dashes strokes a dashed route a piece at a time: a straight piece along
// an axis anchors the pattern to the plane, so edges sharing a track dash
// in step whichever way they run, and a bend continues it.
func (r *route) dashes(d *core.Drawing, line []point, weights []float64) {
	const period = dashLength + dashGap
	d.SetLineDash(dashLength, dashGap)
	phase := 0.0
	piece := func(a point, wa float64, b point, wb float64) {
		length := math.Hypot(b.x-a.x, b.y-a.y)
		if length == 0 {
			return
		}
		ux, uy := (b.x-a.x)/length, (b.y-a.y)/length
		if math.Abs(ux) > .999 || math.Abs(uy) > .999 {
			// Along the axis a dash covers [k·period, k·period+dashLength]
			// of the plane's coordinate, entered from either end.
			anchor := ux*a.x + uy*a.y
			if ux < 0 || uy < 0 {
				anchor += dashLength
			}
			phase = math.Mod(math.Mod(anchor, period)+period, period)
		}
		d.SetLineDashOffset(float32(phase))
		d.BeginPath()
		r.moveTo(d, a, wa)
		r.lineTo(d, b, wb)
		d.Stroke()
		phase = math.Mod(phase+length, period)
	}
	from, fw := line[0], weights[0]
	corners(line, weights, func(a, control, b point, wa, wc, wb float64) {
		piece(from, fw, a, wa)
		// The bend continues the pattern over its length.
		length, prev := 0.0, a
		for step := 1; step <= cornerSteps; step++ {
			t := float64(step) / cornerSteps
			u := 1 - t
			q := point{u*u*a.x + 2*u*t*control.x + t*t*b.x, u*u*a.y + 2*u*t*control.y + t*t*b.y}
			length += math.Hypot(q.x-prev.x, q.y-prev.y)
			prev = q
		}
		d.SetLineDashOffset(float32(phase))
		d.BeginPath()
		r.moveTo(d, a, wa)
		r.quadTo(d, control, wc, b, wb)
		d.Stroke()
		phase = math.Mod(phase+length, period)
		from, fw = b, wb
	})
	piece(from, fw, line[len(line)-1], weights[len(line)-1])
}

// label draws an edge's label in a pill on the middle of the route's
// longest segment.
func (r *route) label(d *core.Drawing, text string, look Look, alpha float32) {
	points := r.points
	best, length := 1, -1.0
	for k := 1; k < len(points); k++ {
		if l := math.Hypot(points[k].x-points[k-1].x, points[k].y-points[k-1].y); l > length {
			best, length = k, l
		}
	}
	a, b := points[best-1], points[best]
	x, y := (a.x+b.x)/2, (a.y+b.y)/2
	w := (r.weights[best-1] + r.weights[best]) / 2
	m := d.TextMetrics(text, labelFont)
	tw, th := float64(m.Width), float64(m.LineHeight)
	box := struct{ x, y, w, h float32 }{float32(x - tw/2 - labelPadX), float32(y - th/2 - labelPadY), float32(tw + 2*labelPadX), float32(th + 2*labelPadY)}
	fade := func(c core.Color) core.Color { return c.Faded(alpha) }
	r.shifted(d, w, func() {
		d.BeginPath().RoundRect(box.x, box.y, box.w, box.h, box.h/2)
		d.SetFillColor(fade(look.LabelBackground)).Fill()
		d.SetStrokeColor(fade(look.LabelBorder)).SetLineWidth(1).Stroke()
		d.SetFillColor(fade(look.Label)).SetTextBaseline(core.BaselineTop).
			FillText(text, box.x+labelPadX, box.y+labelPadY, labelFont)
	})
}

// corners calls bend for each interior point of a route with where its
// curve leaves the incoming segment, the point itself as control and where
// it joins the outgoing one, with their weights along the route. Bends
// take at most half of either segment, so short jogs become smooth steps.
func corners(points []point, weights []float64, bend func(a, control, b point, wa, wc, wb float64)) {
	for k := 1; k+1 < len(points); k++ {
		prev, at, next := points[k-1], points[k], points[k+1]
		in, out := math.Hypot(at.x-prev.x, at.y-prev.y), math.Hypot(next.x-at.x, next.y-at.y)
		if in == 0 || out == 0 {
			continue
		}
		r := math.Min(cornerRadius, math.Min(in, out)/2)
		wa := weights[k] - (weights[k]-weights[k-1])*r/in
		wb := weights[k] + (weights[k+1]-weights[k])*r/out
		bend(point{at.x - (at.x-prev.x)/in*r, at.y - (at.y-prev.y)/in*r}, at,
			point{at.x + (next.x-at.x)/out*r, at.y + (next.y-at.y)/out*r}, wa, weights[k], wb)
	}
}
