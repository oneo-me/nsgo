package plot

import "github.com/oneo-me/nsgo/core"

// tween is the move of a plot's linear axis to a new domain (Motion.Update),
// as d3 transitions a scale's domain: the domain shown is interpolated from
// the old one to the new one on the platform's frame clock, and everything
// placed on the axis goes where the interpolated scale puts it, so every
// mark keeps its shape while the scale moves.
//
// The marks place their points on the new scale once; a place at on it
// shows at scale*at+offset, two nodes the domain's two ends drive: a
// scale's map of one domain is an affine map of its map of another. The
// marks' canvases record their paths through that transform while a move
// runs, and bars and bound views derive their places from the two nodes.
type tween struct {
	cx *core.Scope
	// lo and hi are the ends of the domain shown, in the units of basis;
	// toLo and toSpan the new domain's start and length in them, and start
	// and length the range's.
	lo, hi, toLo, toSpan, start, length *core.AnimatedValue
	scale, offset                       core.AnimatedNode
	moving                              *core.Signal[bool]
	// origin and unit measure the domains while moves follow each other,
	// so a domain of prices or times keeps its digits in the animator's
	// numbers.
	origin, unit float64
	last         linearMap
	hasLast      bool
}

func newTween(cx *core.Scope) *tween {
	v := func(x float64) *core.AnimatedValue { return core.NewAnimatedValue(cx, x) }
	t := &tween{cx: cx, lo: v(0), hi: v(1), toLo: v(0), toSpan: v(1), start: v(0), length: v(0),
		moving: core.NewSignal(cx, false), unit: 1}
	// The scale shown places v at start + (v-lo)/(hi-lo)*length; the new one
	// put it at at = start + (v-toLo)/toSpan*length: scale*at + offset.
	span := core.Sub(t.hi, t.lo)
	t.scale = core.Div(t.toSpan, span)
	t.offset = core.Add(core.Sub(t.start, core.Mul(t.start, t.scale)),
		core.Div(core.Mul(core.Sub(t.toLo, t.lo), t.length), span))
	return t
}

// shown is where a place on the new scale shows now.
func (t *tween) shown(at core.AnimatedNode) core.AnimatedNode {
	return core.Add(core.Mul(t.scale, at), t.offset)
}

// moved is how far a place on the new scale shows from it now: what moves
// a view laid out there.
func (t *tween) moved(at core.AnimatedNode) core.AnimatedNode {
	return core.Add(core.Mul(core.Sub(t.scale, core.Constant(1)), at), t.offset)
}

// stretched is the length a length on the new scale shows at now.
func (t *tween) stretched(length core.AnimatedNode) core.AnimatedNode {
	return core.Mul(t.scale, length)
}

// shownNum and stretchedNum are shown and stretched of a number of a
// drawing.
func (t *tween) shownNum(at num) num {
	return at.mul(nodeNum(t.scale)).add(nodeNum(t.offset))
}

func (t *tween) stretchedNum(length num) num { return length.mul(nodeNum(t.scale)) }

// follow has the axis place values by m now: from the last linear map, its
// domain moves to the new one over timing, from where it shows if a move
// runs. At once without a timing, with reduced motion, for a new range
// (nothing moves for the plot's size), a domain of one value at either end,
// or another kind of scale.
func (t *tween) follow(m axisMap, timing *core.TimingConfig) {
	to, isLinear := m.linear, m.kind == axisLinear
	from, hadFrom := t.last, t.hasLast
	t.last, t.hasLast = to, isLinear
	if !hadFrom || !isLinear {
		t.rest()
		return
	}
	if from == to {
		return
	}
	f, d, rng := from.ends(), to.ends(), to.rng
	movable := from.rng == rng && rng[0] != rng[1] && f[1] > f[0] && d[1] > d[0]
	if timing == nil || timing.Duration <= 0 || !movable || core.Untracked(t.cx, t.cx.Env().ReduceMotion) {
		t.rest()
		return
	}
	moving := t.moving.GetUntracked()
	if !moving {
		// From the old domain: 0 to 1 in its own units.
		t.origin, t.unit = f[0], f[1]-f[0]
		t.lo.Set(0)
		t.hi.Set(1)
	}
	lo, hi := (d[0]-t.origin)/t.unit, (d[1]-t.origin)/t.unit
	t.toLo.Set(lo)
	t.toSpan.Set(hi - lo)
	t.start.Set(float64(rng[0]))
	t.length.Set(float64(rng[1] - rng[0]))
	t.moving.Set(true)
	// Stopped by a newer move or a jump: that one ends it.
	t.cx.StartAnimation(core.Parallel(core.Timing(t.lo, lo, *timing), core.Timing(t.hi, hi, *timing)), func(finished bool) {
		if finished {
			t.rest()
		}
	})
}

// rest has places show where the new scale puts them.
func (t *tween) rest() {
	if !t.moving.GetUntracked() {
		return
	}
	t.moving.Set(false)
	t.lo.Stop()
	t.hi.Stop()
	t.lo.Set(0)
	t.hi.Set(1)
	t.toLo.Set(0)
	t.toSpan.Set(1)
	t.start.Set(0)
	t.length.Set(0)
}
