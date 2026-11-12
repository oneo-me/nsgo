//go:build js

package web

import (
	"strconv"
	"syscall/js"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The browser's history follows the app's navigation when the app is the
// page, as NSH's does: each screen above a stack's first and each modal
// shown is an entry (ItemHistory), so the back button, the mouse's back
// button and the back key go back in the app.
//
// Entries carry the depth and the page load they belong to. Entries the
// app takes away itself are gone back over with history.go, whose
// popstate is not a back; a back the app refuses gets its entry again.
// After a reload the earlier load's entries are passed over. An app in an
// element of a page, or in a frame, pushes nothing: its entries would be
// the page's; it takes the mouse's back button and the back key itself.
type history struct {
	owned   bool
	session string
	depth   int // the entries above the first the app has
	skip    int // popstates of the app's own history.go to ignore
}

// listenHistory starts following the history, owned when the app is the
// whole page of the top window.
func (s *surface) listenHistory(ls *listeners, whole bool) {
	win := js.Global().Get("window")
	h := &s.history
	h.owned = whole && win.Get("top").Equal(win)
	if !h.owned {
		ls.on(s.host, "mouseup", func(e js.Value) {
			if e.Get("button").Int() == 3 {
				e.Call("preventDefault")
				s.engine.Dispatch(driver.Event{Kind: driver.EventBack})
			}
		})
		ls.on(s.doc, "keydown", func(e js.Value) {
			if e.Get("key").String() == "BrowserBack" {
				e.Call("preventDefault")
				s.engine.Dispatch(driver.Event{Kind: driver.EventBack})
			}
		})
		return
	}
	h.session = strconv.FormatInt(time.Now().UnixNano(), 36)
	hist := win.Get("history")
	hist.Call("replaceState", h.state(0), "")
	ls.on(win, "popstate", func(e js.Value) { s.popState(e.Get("state")) })
}

func (h *history) state(depth int) map[string]any {
	return map[string]any{"nsgo": depth, "session": h.session}
}

// popState follows the browser moving through the history.
func (s *surface) popState(state js.Value) {
	h := &s.history
	hist := js.Global().Get("history")
	if h.skip > 0 {
		h.skip--
		return
	}
	depth := 0
	if state.Truthy() && state.Get("nsgo").Type() == js.TypeNumber {
		if state.Get("session").String() != h.session {
			// An entry of an earlier load of the page.
			hist.Call("back")
			return
		}
		depth = state.Get("nsgo").Int()
	}
	switch {
	case depth < h.depth:
		// Back, maybe several entries: one back each.
		for range h.depth - depth {
			s.engine.Dispatch(driver.Event{Kind: driver.EventBack})
		}
		h.depth = depth
	case depth > h.depth:
		// Forward to entries the app has gone back from: nothing there.
		h.skip++
		hist.Call("go", h.depth-depth)
	}
}

// setHistory answers ItemHistory: it pushes entries up to depth, or goes
// back over those the app took away.
func (s *surface) setHistory(depth int) {
	h := &s.history
	if !h.owned {
		return
	}
	hist := js.Global().Get("history")
	for h.depth < depth {
		h.depth++
		hist.Call("pushState", h.state(h.depth), "")
	}
	if h.depth > depth {
		h.skip++
		hist.Call("go", depth-h.depth)
		h.depth = depth
	}
}
