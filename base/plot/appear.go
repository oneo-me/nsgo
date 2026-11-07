package plot

import "github.com/oneo-me/nsgo/core"

// AppearScope remembers which plots inside it have finished appearing, as
// gpui-base's PlotAppearScope, so a plot made again after a gap, a row a
// list scrolled out of its window and back, shows its marks whole at once
// instead of drawing in again.
//
// A plot is remembered by its AppearGeneration once its appear has
// finished: give each row's plot its own. A new generation still replays
// the appear, and one taken away before its appear finished draws in again
// from the start; once a plot finishes a new generation, its last one is
// forgotten. The memory lasts while the scope lives, and a new ID forgets
// it. The innermost scope is the one a plot remembers in.
type AppearScope struct {
	// ID is what the scope remembers for, a comparable value: when it
	// changes the scope forgets, and the plots in it draw in afresh.
	ID        func() any
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
}

// appearKey is the innermost scope a plot is in.
var appearKey = core.NewKey[*appearMemorySet]("plot-appear", nil)

// appearMemorySet is the appears a scope has seen finish.
type appearMemorySet struct {
	finished map[any]bool
	// replaces is the generation each pending one takes the place of, a
	// plot moved to it from one it showed whole: when it finishes, in
	// whichever plot made again meanwhile, that one is forgotten.
	replaces map[any]any
	// epoch moves on when the scope forgets.
	epoch *core.Signal[int]
}

// Render implements core.Node.
func (a AppearScope) Render(cx *core.Scope) core.Node {
	scope := &appearMemorySet{finished: map[any]bool{}, replaces: map[any]any{}, epoch: core.NewSignal(cx, 0)}
	appearKey.Provide(cx, scope)
	if a.ID != nil {
		named, first := any(nil), true
		cx.Effect(func() {
			now := a.ID()
			if first {
				named, first = now, false
				return
			}
			if now == named {
				return
			}
			named = now
			clear(scope.finished)
			clear(scope.replaces)
			scope.epoch.Update(func(e int) int { return e + 1 })
		})
	}
	return core.View{Class: a.Class, Style: a.Style, StyleFunc: a.StyleFunc, Children: a.Children}
}

// appearMemory is a plot's part of the innermost AppearScope it is in.
type appearMemory struct {
	scope *appearMemorySet
	// held is the generation of this plot the scope holds as finished.
	held    any
	hasHeld bool
}

func newAppearMemory(cx *core.Scope) *appearMemory { return &appearMemory{scope: appearKey.Use(cx)} }

// isFinished reports whether the appear of generation has finished in the
// scope, so the plot shows whole at once; it subscribes to the scope
// forgetting.
func (m *appearMemory) isFinished(generation any) bool {
	if m.scope == nil {
		return false
	}
	m.scope.epoch.Get()
	finished := m.scope.finished[generation]
	if finished {
		m.held, m.hasHeld = generation, true
	} else if m.hasHeld && m.held != generation {
		m.scope.replaces[generation] = m.held
	}
	return finished
}

// finish has the scope remember generation in place of the plot's last one.
func (m *appearMemory) finish(generation any) {
	if m.scope == nil {
		return
	}
	if old, ok := m.scope.replaces[generation]; ok {
		delete(m.scope.replaces, generation)
		if old != generation {
			delete(m.scope.finished, old)
		}
	}
	if m.hasHeld && m.held != generation {
		delete(m.scope.finished, m.held)
	}
	m.held, m.hasHeld = generation, true
	m.scope.finished[generation] = true
}
