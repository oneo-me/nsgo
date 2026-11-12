//go:build js && wasm

package web

import (
	"math"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/webstyle"
)

// Sticky list cells stick with the browser's position: sticky, as NSH's
// web backend's do (op_sticky): they move with its scrolling, in step
// with the compositor, where a shift set from scroll events would trail
// it by a frame. Each cell sits in a track spanning its range of travel,
// from its frame to the offset End plus its size, out of which the next
// sticky cell pushes it. Views clip with overflow: clip where the
// browser has it, which lets a cell in a list inside the scroll view
// reach the scroll view.

// stickyCSS lifts a track above the list's other cells; with no breadth
// across it takes no pointer events.
const stickyCSS = `.nsgo-track { position: absolute; z-index: 5; }
.nsgo-track > * { position: sticky !important; margin-right: auto; }
.nsgo-sticky { isolation: isolate; }`

// outer is the element v's parent holds: its track while it sticks.
func (v *view) outer() js.Value {
	if v.track.Truthy() {
		return v.track
	}
	return v.el
}

// setSticky replaces scroll view v's sticky cells.
func (s *surface) setSticky(v *view, cells []driver.StickyCell) {
	if !s.stickyCSS {
		s.stickyCSS = true
		el := s.doc.Call("createElement", "style")
		el.Set("textContent", stickyCSS)
		s.doc.Get("head").Call("appendChild", el)
	}
	sc := v.sc
	ranges := make(map[driver.NodeID]driver.StickyCell, len(cells))
	for _, c := range cells {
		ranges[c.Cell] = c
	}
	v.el.Get("classList").Call("toggle", "nsgo-sticky", len(cells) > 0)
	// Cells no longer sticky go back to their frames.
	for id, cell := range sc.tracks {
		if _, ok := ranges[id]; ok && s.view(id) == cell {
			continue
		}
		delete(sc.tracks, id)
		cell.unstick()
	}
	if sc.tracks == nil {
		sc.tracks = map[driver.NodeID]*view{}
	}
	for id, r := range ranges {
		cell := s.view(id)
		if cell == nil {
			continue
		}
		if !cell.track.Truthy() {
			if cell.layoutAnim.Truthy() {
				cell.layoutAnim.Call("cancel")
				cell.layoutAnim = js.Undefined()
			}
			track := s.element("div")
			track.Get("classList").Call("add", "nsgo-track")
			host := cell.el.Get("parentNode")
			if host.IsNull() || host.IsUndefined() {
				host = v.content
			}
			cell.trackPage = !host.Equal(v.content)
			if cell.trackPage {
				host.Get("classList").Call("add", "nsgo-sticky")
			}
			host.Call("insertBefore", track, cell.el)
			track.Call("appendChild", cell.el)
			cell.track = track
			sc.tracks[id] = cell
		}
		cell.stickyIn, cell.sticky = v, r
		if r.Node != 0 {
			if s.stickyBound == nil {
				s.stickyBound = map[*view]bool{}
			}
			s.stickyBound[cell] = true
		} else {
			delete(s.stickyBound, cell)
		}
		s.placeTrack(cell)
	}
}

// replaceBound places again the sticky cells whose insets follow
// animated values, which may have moved.
func (s *surface) replaceBound() {
	for cell := range s.stickyBound {
		if s.view(cell.id) != cell || !cell.track.Truthy() || cell.sticky.Node == 0 {
			delete(s.stickyBound, cell)
			continue
		}
		s.placeTrack(cell)
	}
}

// unstick puts a sticky cell back among its list's cells at its frame.
func (v *view) unstick() {
	track := v.track
	v.track, v.stickyIn = js.Undefined(), nil
	if track.Get("parentNode").Truthy() && v.el.Get("parentNode").Equal(track) {
		track.Call("replaceWith", v.el)
	} else {
		track.Call("remove")
	}
	v.placedOK = false
	v.place(v.frame)
}

// trackOrigin is where the views around a page list's track put it in
// its scroll view's content.
func (s *surface) trackOrigin(v *view) driver.Points {
	list := v.stickyIn
	var at driver.Points
	for n := v.track.Get("parentNode"); n.Truthy() && !n.Equal(list.content) && !n.Equal(list.el); n = n.Get("parentNode") {
		id := n.Get("nsgoID")
		if id.IsUndefined() {
			continue
		}
		if p := s.view(driver.NodeID(id.Int())); p != nil {
			if list.sc.config.Horizontal {
				at += p.frame.Origin.X
			} else {
				at += p.frame.Origin.Y
			}
		}
	}
	return at
}

// placeTrack places a sticky cell's track along its range of travel and
// the cell in it: it sticks the distance from the range's start to its
// frame from the scroll view's start.
func (s *surface) placeTrack(cell *view) {
	if !cell.track.Truthy() || cell.stickyIn == nil {
		return
	}
	f, r, list := cell.frame, cell.sticky, cell.stickyIn
	horizontal := list.sc.config.Horizontal
	at, size, content := f.Origin.Y, f.Size.Height, list.scroll.Height
	if horizontal {
		at, size, content = f.Origin.X, f.Size.Width, list.scroll.Width
	}
	var origin driver.Points
	travel := r.End - r.Start
	if cell.trackPage {
		origin = s.trackOrigin(cell)
	} else {
		travel = min(travel, content-at-size)
	}
	length := size + max(travel, 0)
	if math.IsInf(float64(length), 0) || length != length {
		length = size
	}
	w, h := driver.Points(0), length
	if horizontal {
		w, h = length, 0
	}
	setStyle(cell.track, map[string]string{"left": webstyle.Px(f.Origin.X), "top": webstyle.Px(f.Origin.Y), "width": webstyle.Px(w), "height": webstyle.Px(h)})
	start := r.Start
	if r.Node != 0 {
		// The inset follows its animated value.
		if v, _, ok := s.animator.Value(r.Node); ok {
			start += r.Inset - driver.Points(v)
		}
	}
	inset := webstyle.Px(at + origin - start)
	style := map[string]string{"left": "auto", "top": inset, "width": webstyle.Px(f.Size.Width), "height": webstyle.Px(f.Size.Height)}
	if horizontal {
		style["left"], style["top"] = inset, "auto"
	}
	setStyle(cell.el, style)
	cell.placedOK = false
}
