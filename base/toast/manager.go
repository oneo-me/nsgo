package toast

import (
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// Toasts follow NSH's nsh/base, which has gpui-base's three pieces with
// their names and semantics: Manager keeps the toasts and their
// lifecycle, Stack lays them out, and Toast is each one's root. The
// look and the content are the app's, and so is showing a toast from
// anywhere: an app builds its own toast(...) on these, as the gallery
// does.

// Status is a toast's phase in its lifecycle.
type Status uint8

const (
	// Starting is a toast entering, for the motion's Duration.
	Starting Status = iota
	// Present is a toast shown, counting its timeout down.
	Present
	// Ending is a toast leaving, for the motion's ExitDuration,
	// before it is removed.
	Ending
)

// Motion are the tokens of a Manager and a Stack; a zero
// field keeps Sonner's, which SonnerMotion returns.
type Motion struct {
	// Duration is how long a toast enters, and the stack's springs'
	// response.
	Duration time.Duration
	// ExitDuration is how long a toast leaves.
	ExitDuration time.Duration
	// CollapsedPeek is how far apart collapsed toasts overlap, and
	// ExpandedGap the space between expanded ones.
	CollapsedPeek, ExpandedGap core.Points
	// CollapsedScaleStep is how much narrower each collapsed toast behind
	// the newest is, as a fraction of its width.
	CollapsedScaleStep float32
	// CollapsedVisible is how many collapsed toasts show.
	CollapsedVisible int
}

// SonnerMotion returns Sonner's motion.
func SonnerMotion() Motion {
	return Motion{Duration: 400 * time.Millisecond, ExitDuration: 200 * time.Millisecond,
		CollapsedPeek: 14, ExpandedGap: 14, CollapsedScaleStep: 0.05, CollapsedVisible: 3}
}

// resolved returns m with Sonner's values for its zero fields.
func (m Motion) resolved() Motion {
	s := SonnerMotion()
	if m.Duration == 0 {
		m.Duration = s.Duration
	}
	if m.ExitDuration == 0 {
		m.ExitDuration = s.ExitDuration
	}
	if m.CollapsedPeek == 0 {
		m.CollapsedPeek = s.CollapsedPeek
	}
	if m.ExpandedGap == 0 {
		m.ExpandedGap = s.ExpandedGap
	}
	if m.CollapsedScaleStep == 0 {
		m.CollapsedScaleStep = s.CollapsedScaleStep
	}
	if m.CollapsedVisible == 0 {
		m.CollapsedVisible = s.CollapsedVisible
	}
	return m
}

// Entry is a toast a Manager keeps.
type Entry[K comparable, V any] struct {
	ID     K
	Value  V
	Status Status
}

// Advance is what an Advance changed: the toasts it presented, the
// ones whose timeout ran out, and the ones it removed.
type Advance[K comparable, V any] struct {
	Changed   bool
	Presented []K
	Ending    []K
	Removed   []Entry[K, V]
}

type managedToast[K comparable, V any] struct {
	Entry[K, V]
	timeout   time.Duration // left; none when 0 and not timed
	timed     bool
	elapsed   time.Duration // in the transition
	lastCheck time.Time
}

// Manager keeps toasts in order with their lifecycle, as gpui-base's:
// each one starts, is present once the motion's Duration has passed,
// counts its timeout down while present and not paused, and stays ending
// for the ExitDuration before it is removed. It renders nothing and runs
// no timer: the app pushes toasts, renders Visible with a Stack and
// calls Advance on a timer while any is shown, passing whether the stack
// is expanded. Times are the app's clock's, Scope.Now, so a test's manual
// clock moves them. The zero value has no toasts and Sonner's motion.
type Manager[K comparable, V any] struct {
	Motion  Motion
	entries []managedToast[K, V]
}

// Push adds the newest toast, replacing one with the same id, whose value
// it returns. A timeout of 0 keeps it until it is dismissed.
func (m *Manager[K, V]) Push(id K, value V, timeout time.Duration, now time.Time) (replaced V, ok bool) {
	for i, e := range m.entries {
		if e.ID == id {
			replaced, ok = e.Value, true
			m.entries = append(m.entries[:i], m.entries[i+1:]...)
			break
		}
	}
	m.entries = append(m.entries, managedToast[K, V]{Entry: Entry[K, V]{ID: id, Value: value},
		timeout: timeout, timed: timeout > 0, lastCheck: now})
	return replaced, ok
}

// Dismiss begins a toast's exit and reports whether it was not leaving
// already.
func (m *Manager[K, V]) Dismiss(id K, now time.Time) bool {
	for i := range m.entries {
		if e := &m.entries[i]; e.ID == id {
			if e.Status == Ending {
				return false
			}
			e.end(now)
			return true
		}
	}
	return false
}

// DismissAll begins every toast's exit and returns the ids of those that
// were not leaving already.
func (m *Manager[K, V]) DismissAll(now time.Time) []K {
	var ids []K
	for i := range m.entries {
		if e := &m.entries[i]; e.Status != Ending {
			e.end(now)
			ids = append(ids, e.ID)
		}
	}
	return ids
}

func (e *managedToast[K, V]) end(now time.Time) {
	e.Status, e.elapsed, e.lastCheck = Ending, 0, now
}

// Advance moves the lifecycle on to now; paused holds present toasts'
// timeouts, as while the stack is expanded.
func (m *Manager[K, V]) Advance(now time.Time, paused bool) Advance[K, V] {
	motion := m.Motion.resolved()
	var a Advance[K, V]
	for i := range m.entries {
		e := &m.entries[i]
		delta := max(now.Sub(e.lastCheck), 0)
		e.lastCheck = now
		switch e.Status {
		case Starting:
			if e.elapsed += delta; e.elapsed >= motion.Duration {
				e.Status, e.elapsed = Present, 0
				a.Presented = append(a.Presented, e.ID)
				a.Changed = true
			}
		case Present:
			if !paused && e.timed {
				if e.timeout -= delta; e.timeout <= 0 {
					e.end(now)
					a.Ending = append(a.Ending, e.ID)
					a.Changed = true
				}
			}
		case Ending:
			e.elapsed += delta
		}
	}
	kept := m.entries[:0]
	for _, e := range m.entries {
		if e.Status == Ending && e.elapsed >= motion.ExitDuration {
			a.Removed = append(a.Removed, e.Entry)
			a.Changed = true
		} else {
			kept = append(kept, e)
		}
	}
	clear(m.entries[len(kept):])
	m.entries = kept
	return a
}

// Entries returns every toast, oldest first.
func (m *Manager[K, V]) Entries() []Entry[K, V] {
	out := make([]Entry[K, V], len(m.entries))
	for i, e := range m.entries {
		out[i] = e.Entry
	}
	return out
}

// Visible returns the newest limit toasts that are not ending and every
// ending one, oldest first: what a Stack shows.
func (m *Manager[K, V]) Visible(limit int) []Entry[K, V] {
	active := 0
	for _, e := range m.entries {
		if e.Status != Ending {
			active++
		}
	}
	skip := max(active-limit, 0)
	var out []Entry[K, V]
	for _, e := range m.entries {
		if e.Status != Ending {
			if skip > 0 {
				skip--
				continue
			}
		}
		out = append(out, e.Entry)
	}
	return out
}

// Get returns the value of the toast with id.
func (m *Manager[K, V]) Get(id K) (v V, ok bool) {
	for _, e := range m.entries {
		if e.ID == id {
			return e.Value, true
		}
	}
	return v, false
}

// Len returns how many toasts it keeps.
func (m *Manager[K, V]) Len() int { return len(m.entries) }
