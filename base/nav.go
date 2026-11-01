package base

import "github.com/oneo-me/nsgo/core"

// NavPage is one page of a Nav's stack, filling it. On iOS and Android the
// platform draws the system header, a navigation bar or a toolbar, from
// its title and header options; elsewhere its Header, when set, is drawn
// at the top.
type NavPage = core.Screen

// Nav is a navigation stack of routes of type R: pushing a route adds a
// screen, popping removes it. The same route pushed twice is two pages.
type Nav[R any] struct {
	entries *core.Signal[[]navEntry[R]]
	next    int
}

type navEntry[R any] struct {
	key   int
	route R
}

// NewNav returns a stack holding routes, the first at the bottom.
func NewNav[R any](cx *core.Scope, routes ...R) *Nav[R] {
	n := &Nav[R]{}
	n.entries = core.NewSignalFunc(cx, n.wrap(routes), nil)
	return n
}

func (n *Nav[R]) wrap(routes []R) []navEntry[R] {
	out := make([]navEntry[R], len(routes))
	for i, r := range routes {
		n.next++
		out[i] = navEntry[R]{n.next, r}
	}
	return out
}

// Routes returns the routes, bottom first.
func (n *Nav[R]) Routes() []R {
	es := n.entries.Get()
	out := make([]R, len(es))
	for i, e := range es {
		out[i] = e.route
	}
	return out
}

// Depth returns how many routes are above the first.
func (n *Nav[R]) Depth() int { return max(len(n.entries.Get())-1, 0) }

// Push adds route on top.
func (n *Nav[R]) Push(route R) {
	n.entries.Update(func(es []navEntry[R]) []navEntry[R] { return append(es, n.wrap([]R{route})...) })
}

// Pop removes up to count routes from the top, keeping the first.
func (n *Nav[R]) Pop(count int) {
	n.entries.Update(func(es []navEntry[R]) []navEntry[R] {
		return es[:max(len(es)-count, min(len(es), 1))]
	})
}

// PopToRoot keeps only the first route.
func (n *Nav[R]) PopToRoot() { n.Pop(len(n.entries.GetUntracked())) }

// Replace replaces the top route.
func (n *Nav[R]) Replace(route R) {
	n.entries.Update(func(es []navEntry[R]) []navEntry[R] {
		if len(es) == 0 {
			return n.wrap([]R{route})
		}
		return append(es[:len(es)-1:len(es)-1], n.wrap([]R{route})...)
	})
}

// SetRoutes replaces every route.
func (n *Nav[R]) SetRoutes(routes ...R) { n.entries.Set(n.wrap(routes)) }

// SetRoutesKeeping replaces every route, a route same says is one there
// already keeping its page, the first such one in order.
func (n *Nav[R]) SetRoutesKeeping(same func(a, b R) bool, routes ...R) {
	n.entries.Update(func(es []navEntry[R]) []navEntry[R] {
		used := make([]bool, len(es))
		out := make([]navEntry[R], 0, len(routes))
		for _, r := range routes {
			kept := false
			for i, e := range es {
				if !used[i] && same(e.route, r) {
					used[i], kept = true, true
					out = append(out, e)
					break
				}
			}
			if !kept {
				out = append(out, n.wrap([]R{r})...)
			}
		}
		return out
	})
}

// Remove takes away the routes match reports, with their pages.
func (n *Nav[R]) Remove(match func(R) bool) {
	n.entries.Update(func(es []navEntry[R]) []navEntry[R] {
		out := es[:0:0]
		for _, e := range es {
			if !match(e.route) {
				out = append(out, e)
			}
		}
		return out
	})
}

// Stack renders the routes as a stack of pages, one each. A page the
// user pops goes from the routes before its OnDismiss runs.
func (n *Nav[R]) Stack(style core.Style, page func(cx *core.Scope, route R, depth func() int) NavPage) core.Node {
	return core.Stack{Style: style, Children: []core.Node{
		core.Each(n.entries.Get, func(e navEntry[R]) int { return e.key },
			func(cx *core.Scope, e func() navEntry[R], depth func() int) core.Node {
				s := page(cx, e().route, depth)
				dismiss := s.OnDismiss
				key := e().key
				s.OnDismiss = func() {
					n.remove(key)
					if dismiss != nil {
						dismiss()
					}
				}
				return s
			}),
	}}
}

func (n *Nav[R]) remove(key int) {
	n.entries.Update(func(es []navEntry[R]) []navEntry[R] {
		out := es[:0:0]
		for _, e := range es {
			if e.key != key {
				out = append(out, e)
			}
		}
		return out
	})
}
