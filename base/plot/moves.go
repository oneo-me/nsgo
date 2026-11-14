package plot

import (
	"math"

	"github.com/nsgo-dev/nsgo/core"
)

// pose is what a mark's moves go between: its shape as the app gives it.
type pose[P any] interface {
	// lerp is the pose drawn t of the way from this one to to.
	lerp(to P, t float32) P
	same(other P) bool
}

// moveState is the pose a mark moves to and, while a move runs, the one it
// moves from.
type moveState[P any] struct {
	to, from    P
	has, moving bool
}

// moves is a mark's moves from one pose to the next with Motion.Update, as
// d3's transitions of a mark's attributes: the pose it draws, the one it
// moves from while a move runs, and how far that has gone, on the
// platform's frame clock. A change while a move runs moves on from where
// the mark is drawn then: the move stops, the platform tells how far it
// had gone, and the next one starts from the pose drawn there. The changes
// of one turn make one move, and a pose laid out in points for a new size
// shows at once.
type moves[P pose[P]] struct {
	cx       *core.Scope
	shown    *core.Signal[moveState[P]]
	progress *core.AnimatedValue
}

// newMoves follows what read returns, moving to each new pose. sized says
// its poses are laid out for the plot's size, so a new size moves nothing.
func newMoves[P pose[P]](cx *core.Scope, plot *plotState, sized bool, read func() (P, bool)) *moves[P] {
	m := &moves[P]{cx: cx, shown: core.NewSignalFunc(cx, moveState[P]{}, nil), progress: core.NewAnimatedValue(cx, 1)}
	var last P
	hasLast := false
	var lastSize core.Size
	changes := 0
	settle := func(to P, ok bool) {
		size := plot.size.GetUntracked()
		before, had, was := last, hasLast, lastSize
		last, hasLast, lastSize = to, ok, size
		if !ok {
			if m.shown.GetUntracked().has {
				m.shown.Set(moveState[P]{})
			}
			return
		}
		if !had {
			m.shown.Set(moveState[P]{to: to, has: true})
			return
		}
		if before.same(to) {
			return
		}
		changes++
		change := changes
		timing := plot.updateTiming()
		if timing == nil || sized && was != size {
			m.shown.Set(moveState[P]{to: to, has: true})
			return
		}
		now := m.shown.GetUntracked()
		if !now.has || !now.moving {
			at := before
			if now.has {
				at = now.to
			}
			m.run(at, to, *timing)
			return
		}
		m.progress.Stop()
		m.progress.Read(cx, func(t float64) {
			// A newer change asks again.
			if changes != change {
				return
			}
			now := m.shown.GetUntracked()
			m.run(now.from.lerp(now.to, float32(t)), to, *timing)
		})
	}
	first := true
	cx.Effect(func() {
		to, ok := read()
		// Decided once the turn's changes are all in; what comes first, or
		// goes, shows at once.
		if first || !ok || !hasLast {
			first = false
			settle(to, ok)
			return
		}
		cx.Post(func() {
			var to P
			var ok bool
			core.Untracked(cx, func() struct{} {
				to, ok = read()
				return struct{}{}
			})
			settle(to, ok)
		})
	})
	return m
}

func (m *moves[P]) run(from, to P, timing core.TimingConfig) {
	m.shown.Set(moveState[P]{to: to, from: from, has: true, moving: true})
	m.progress.Set(0)
	// Stopped by a newer change: that one ends it.
	m.cx.StartAnimation(core.Timing(m.progress, 1, timing), func(finished bool) {
		if finished {
			m.shown.Update(func(s moveState[P]) moveState[P] {
				s.moving = false
				return s
			})
		}
	})
}

// between is a place moving from from to to with the move that runs.
func (m *moves[P]) between(from, to float32) num { return lerpNum(m.progress, from, to) }

// now is what it draws.
func (m *moves[P]) now() moveState[P] { return m.shown.Get() }

// left is how much of the move that runs is left, 0 at rest.
func (m *moves[P]) left() core.AnimatedNode {
	return core.Sub(core.Constant(1), m.progress)
}

// valuePose is the values a mark places along its scales, a few per row in
// the plot's units, what Motion.Update moves between, so that a new size
// moves nothing and a new domain moves the mark with the scales. Rows are
// matched by key where keys holds one per row (a row's x on a linear
// scale), else by index; NaN is no value.
type valuePose struct {
	keys []float64
	rows [][]float64
}

func (p valuePose) same(o valuePose) bool {
	if len(p.rows) != len(o.rows) || len(p.keys) != len(o.keys) {
		return false
	}
	eq := func(a, b float64) bool { return a == b || math.IsNaN(a) && math.IsNaN(b) }
	for i, k := range p.keys {
		if !eq(k, o.keys[i]) {
			return false
		}
	}
	for i, r := range p.rows {
		if len(r) != len(o.rows[i]) {
			return false
		}
		for j, v := range r {
			if !eq(v, o.rows[i][j]) {
				return false
			}
		}
	}
	return true
}

// match is the row of p that row of to moves from, -1 for none: the row
// of its key, or of its index where rows have no keys and as many rows.
func (p valuePose) match(to valuePose) func(row int) int {
	if to.keys == nil || p.keys == nil {
		if to.keys != nil || p.keys != nil || len(p.rows) != len(to.rows) {
			return func(int) int { return -1 }
		}
		return func(row int) int { return row }
	}
	at := make(map[float64]int, len(p.keys))
	for i, k := range p.keys {
		if !math.IsNaN(k) {
			at[k] = i
		}
	}
	return func(row int) int {
		if row < len(to.keys) {
			if i, ok := at[to.keys[row]]; ok {
				return i
			}
		}
		return -1
	}
}

func (p valuePose) lerp(to valuePose, t float32) valuePose {
	from := p.match(to)
	f := float64(t)
	out := valuePose{keys: to.keys, rows: make([][]float64, len(to.rows))}
	for row, r := range to.rows {
		values := append([]float64(nil), r...)
		if i := from(row); i >= 0 && len(p.rows[i]) == len(r) {
			for j, b := range r {
				if a := p.rows[i][j]; isFinite(a) && isFinite(b) {
					values[j] = a + (b-a)*f
				}
			}
		}
		out.rows[row] = values
	}
	return out
}
