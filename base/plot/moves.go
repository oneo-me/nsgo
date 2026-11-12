package plot

import "github.com/nsgo-dev/nsgo/core"

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

// now is what it draws.
func (m *moves[P]) now() moveState[P] { return m.shown.Get() }

// left is how much of the move that runs is left, 0 at rest.
func (m *moves[P]) left() core.AnimatedNode {
	return core.Sub(core.Constant(1), m.progress)
}
