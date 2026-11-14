package core

import "time"

// History is a value with the steps it took, to undo and redo: a document, a canvas or a form the app's Edit menu and keys walk back through. Only its own Set and Update record steps. Its value, CanUndo and CanRedo are signals.
//
// Like a signal's, its methods belong to the graph's goroutine.
type History[T any] struct {
	owner *Scope
	equal func(a, b T) bool
	opts  HistoryOptions
	value *Signal[T]
	// past holds the values steps started from, oldest first; future the values undone, the next redo last.
	past, future []T
	steps        *Signal[uint64] // bumped as past or future change, for CanUndo and CanRedo
	group        int             // the depth of Group calls running
	grouped      bool            // whether the running group has recorded its step
	lastAt       time.Time       // when the latest step was last changed, for Merge
	broken       bool            // the next change starts a step of its own
}

// HistoryOptions are how a History records its steps.
type HistoryOptions struct {
	// Limit is the most steps it keeps, dropping the oldest first; 0 keeps 100, and a negative limit every step.
	Limit int
	// Merge makes a change within this long of the latest step's last part of that step, as typing groups into one undo; 0 makes each change a step.
	Merge time.Duration
}

// NewHistory returns a history starting at v; setting a value equal to the current one records nothing.
func NewHistory[T comparable](cx *Scope, v T, opts HistoryOptions) *History[T] {
	return NewHistoryFunc(cx, v, func(a, b T) bool { return a == b }, opts)
}

// NewHistoryFunc returns a history as NewHistory does that compares values with equal; with a nil equal every Set records a step, as slices and maps need.
func NewHistoryFunc[T any](cx *Scope, v T, equal func(a, b T) bool, opts HistoryOptions) *History[T] {
	if opts.Limit == 0 {
		opts.Limit = 100
	}
	return &History[T]{owner: cx.current(), equal: equal, opts: opts,
		value: NewSignalFunc(cx, v, equal), steps: NewSignal(cx, uint64(0)), broken: true}
}

// Get returns the value, subscribing the running computation.
func (h *History[T]) Get() T { return h.value.Get() }

// GetUntracked returns the value without subscribing.
func (h *History[T]) GetUntracked() T { return h.value.GetUntracked() }

// Set makes v the value as a step that Undo goes back from, merged into the latest step within Merge of it or within a running Group, and drops the steps undone.
func (h *History[T]) Set(v T) {
	cur := h.value.GetUntracked()
	if h.equal != nil && h.equal(cur, v) {
		return
	}
	now := h.owner.rt.clock.Now()
	merge := h.group > 0 && h.grouped ||
		h.group == 0 && !h.broken && h.opts.Merge > 0 && now.Sub(h.lastAt) < h.opts.Merge
	if !merge {
		h.past = append(h.past, cur)
		if h.opts.Limit > 0 && len(h.past) > h.opts.Limit {
			h.past = append(h.past[:0:0], h.past[len(h.past)-h.opts.Limit:]...)
		}
	}
	if h.group > 0 {
		h.grouped = true
	}
	h.broken = false
	h.lastAt = now
	h.future = nil
	h.commit(v)
}

// Update sets what fn returns for the current value, as Set does.
func (h *History[T]) Update(fn func(T) T) { h.Set(fn(h.value.GetUntracked())) }

// Group runs fn, making every change it sets one step, as a paste or a replace-all is.
func (h *History[T]) Group(fn func()) {
	if h.group == 0 {
		h.grouped = false
	}
	h.group++
	defer func() {
		if h.group--; h.group == 0 {
			h.broken = true
		}
	}()
	fn()
}

// Break ends the latest step, so the next change starts a step of its own however soon it comes: a word typed, a selection moved.
func (h *History[T]) Break() { h.broken = true }

// Undo goes back to the value the latest step started from, and reports whether there was one.
func (h *History[T]) Undo() bool {
	if len(h.past) == 0 {
		return false
	}
	v := h.past[len(h.past)-1]
	h.past = h.past[:len(h.past)-1]
	h.future = append(h.future, h.value.GetUntracked())
	h.broken = true
	h.commit(v)
	return true
}

// Redo goes forward to the value the latest Undo left, and reports whether there was one.
func (h *History[T]) Redo() bool {
	if len(h.future) == 0 {
		return false
	}
	v := h.future[len(h.future)-1]
	h.future = h.future[:len(h.future)-1]
	h.past = append(h.past, h.value.GetUntracked())
	h.broken = true
	h.commit(v)
	return true
}

// CanUndo reports whether Undo has a step to go back, subscribing the running computation.
func (h *History[T]) CanUndo() bool {
	h.steps.Get()
	return len(h.past) > 0
}

// CanRedo reports whether Redo has a step to go forward, subscribing the running computation.
func (h *History[T]) CanRedo() bool {
	h.steps.Get()
	return len(h.future) > 0
}

// Clear forgets every step, keeping the value: a document just saved or opened.
func (h *History[T]) Clear() {
	h.past, h.future = nil, nil
	h.broken = true
	h.bump()
}

// commit sets the value and the steps in one batch, so a computation reading both runs once.
func (h *History[T]) commit(v T) {
	h.owner.Batch(func() {
		h.value.Set(v)
		h.bump()
	})
}

func (h *History[T]) bump() { h.steps.Update(func(n uint64) uint64 { return n + 1 }) }
