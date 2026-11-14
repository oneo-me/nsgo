package core

import (
	goruntime "runtime"
	"strconv"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/devtools"
)

// devHost is a host under DevTools, its surfaces unregistered when the test ends.
func devHost(t *testing.T, app func(cx *Scope) Node) *host {
	t.Helper()
	devtools.Enabled = true
	t.Cleanup(func() {
		devtools.Enabled = false
		for _, s := range devtools.Surfaces() {
			devtools.Unregister(s)
		}
	})
	return newHost(t, Size{Width: 200, Height: 200}, app)
}

// What DevTools recorded of a list's rows and a branch goes with them, however long the list or the Show that made them stays.
func TestDevToolsForgetRowsAndBranchesThatGo(t *testing.T) {
	var items *Signal[[]int]
	var shown *Signal[bool]
	h := devHost(t, func(cx *Scope) Node {
		items = NewSignalFunc[[]int](cx, nil, nil)
		shown = NewSignal(cx, false)
		return Group{
			Each(items.Get, func(i int) int { return i }, func(cx *Scope, item func() int, _ func() int) Node {
				return Text{ContentFunc: func() string { return strconv.Itoa(item()) }}
			}),
			Show{When: shown.Get, Then: func(cx *Scope) Node {
				NewSignal(cx, "branch")
				return View{}
			}},
		}
	})
	h.commit()
	d := h.root.rt.dev
	values, owners := len(d.byID), len(d.values)
	items.Set([]int{1, 2, 3})
	h.commit()
	if got := len(d.byID); got != values+6 {
		t.Fatalf("3 rows recorded %d signals, want 6, an item and an index each", got-values)
	}
	items.Set([]int{2})
	h.commit()
	items.Set(nil)
	h.commit()
	for range 3 {
		shown.Set(true)
		h.commit()
		shown.Set(false)
		h.commit()
	}
	if len(d.byID) != values || len(d.values) != owners {
		t.Fatalf("DevTools hold %d signals of %d owners after the rows and branches went, want %d of %d", len(d.byID), len(d.values), values, owners)
	}
}

// A signal its code dropped, as a collection's deleted item, leaves DevTools' state, though its owner stays.
func TestDevToolsDropSignalsNothingHolds(t *testing.T) {
	var app *Scope
	h := devHost(t, func(cx *Scope) Node {
		app = cx
		return View{}
	})
	h.commit()
	d := h.root.rt.dev
	listed := func() []string {
		var out []string
		var walk func(o devtools.Owner)
		walk = func(o devtools.Owner) {
			for _, v := range o.Values {
				out = append(out, v.Text)
			}
			for _, c := range o.Children {
				walk(c)
			}
		}
		walk((&devSurface{h.tree}).Owners())
		return out
	}
	before, values := len(listed()), len(d.byID)
	kept := NewSignal(app, -1)
	for i := range 10 {
		NewSignal(app, i)
	}
	goruntime.GC()
	after := listed()
	if len(after) != before+1 || after[len(after)-1] != "-1" || len(d.byID) != values+1 {
		t.Fatalf("DevTools list %v and hold %d signals more, want only the one kept more", after, len(d.byID)-values)
	}
	goruntime.KeepAlive(kept)
}

// Under nsgo dev -devtools without -highlight, the overlay turns outlines off before the first commit's are drawn: the first screen does not flash.
func TestDevToolsHighlightsOffFromTheFirstCommit(t *testing.T) {
	h := devHost(t, func(cx *Scope) Node {
		cx.Post(func() {
			for _, s := range devtools.Surfaces() {
				s.Highlight(false)
			}
		})
		return Group{View{Style: Style{Width: Pt(50), Height: Pt(50)}}, UpdateHighlights{}}
	})
	h.commit()
	h.drain()
	if n := len(h.tree.highlights.boxes.GetUntracked()); n != 0 {
		t.Fatalf("the first commit outlined %d views with outlines off", n)
	}
}
