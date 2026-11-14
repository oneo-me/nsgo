package core

// NewSelector returns is, which reports whether source gives key, as Solid's createSelector does. A computation that calls is subscribes to its key alone, so when source moves from a to b only the readers of a and b run again, however many rows ask: a list's selected row. source is tracked; its value is compared with ==.
func NewSelector[K comparable](cx *Scope, source func() K) (is func(key K) bool) {
	return newSelector(cx, source).is
}

// selector keeps a signal for each key computations read, and drops it when none reads it.
type selector[K comparable] struct {
	rt      *runtime
	keys    map[K]*selectorKey
	current K
}

type selectorKey struct {
	on      *Signal[bool]
	readers int
}

func newSelector[K comparable](cx *Scope, source func() K) *selector[K] {
	s := &selector[K]{rt: cx.rt, keys: map[K]*selectorKey{}}
	started := false
	cx.renderEffect(func() {
		k := source()
		cx.Untrack(func() {
			if started && k == s.current {
				return
			}
			prev := s.current
			s.current = k
			if !started {
				started = true
				return
			}
			if e := s.keys[prev]; e != nil {
				e.on.Set(false)
			}
			if e := s.keys[k]; e != nil {
				e.on.Set(true)
			}
		})
	})
	return s
}

func (s *selector[K]) is(key K) bool {
	reader := s.rt.observer
	if reader == nil {
		return key == s.current
	}
	e := s.keys[key]
	if e == nil {
		e = &selectorKey{on: &Signal[bool]{rt: s.rt, value: key == s.current, equal: func(a, b bool) bool { return a == b }}}
		s.keys[key] = e
	}
	e.readers++
	// The reader's cleanups run as it runs again or goes, so a key nothing reads leaves the map.
	reader.addCleanup(func() {
		if e.readers--; e.readers == 0 && s.keys[key] == e {
			delete(s.keys, key)
		}
	})
	return e.on.Get()
}
