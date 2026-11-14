package core

import (
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// UpdateHighlights outlines for a moment each view a commit of its surface changed, as React DevTools' highlight updates and Flutter's repaint rainbow do, so work an update did not need shows. It draws over the surface's main layer, under the app's modals, and leaves its own views out. nsgo's dev client mounts it over the app under nsgo dev -highlight; it needs a host, an engine or a test driver, and shows nothing without one.
type UpdateHighlights struct{}

// Render implements Node.
func (h UpdateHighlights) Render(*Scope) Node { return h }

func (UpdateHighlights) mountIn(t *Tree, cx *Scope, parent *node) { t.mountHighlights(cx, parent) }

// highlightFor is how long a highlight shows.
const highlightFor = 500 * time.Millisecond

// highlightColor is the outlines' color, a blue seen on light and dark.
var highlightColor = RGB(0x2f9bff)

// highlights are a tree's UpdateHighlights: the boxes it shows, and its overlay, whose views it leaves out.
type highlights struct {
	owner *Scope
	root  *Ref
	boxes *Signal[[]highlightBox]
	next  uint64
}

type highlightBox struct {
	id     uint64
	bounds Bounds
}

func (t *Tree) mountHighlights(cx *Scope, parent *node) {
	if cx.rt.post == nil {
		return
	}
	h := &highlights{owner: cx.current(), root: new(Ref), boxes: NewSignalFunc[[]highlightBox](cx, nil, nil)}
	t.highlights = h
	cx.Cleanup(func() {
		if t.highlights == h {
			t.highlights = nil
		}
	})
	box := func(cx *Scope, b func() highlightBox, _ func() int) Node {
		return View{PointerEvents: PointerEventsNone, StyleFunc: func() Style {
			r := b().bounds
			return Style{Position: Absolute, Left: Pt(r.Origin.X), Top: Pt(r.Origin.Y), Width: Pt(r.Size.Width), Height: Pt(r.Size.Height),
				BorderWidth: 2, BorderColor: highlightColor, CornerRadius: 2}
		}}
	}
	t.mount(cx, parent, Overlay{Ref: h.root, PointerEvents: PointerEventsNone, TestID: "nsgo-update-highlights",
		Children: []Node{Each(h.boxes.Get, func(b highlightBox) uint64 { return b.id }, box)}})
}

// highlightKinds are the view-changing kinds of item: what a commit made, moved, restyled or filled anew.
var highlightKinds = map[driver.ItemKind]bool{
	driver.ItemCreate: true, driver.ItemFrame: true, driver.ItemViewStyle: true, driver.ItemText: true,
	driver.ItemInputValue: true, driver.ItemImage: true, driver.ItemCanvas: true, driver.ItemShader: true,
}

// noteChanges outlines the views items changed, other than the highlights' own, after the commit that sent them: it posts them, as a commit sets no signal.
func (t *Tree) noteChanges(items []driver.MountItem) {
	h := t.highlights
	var boxes []Bounds
	seen := map[NodeID]bool{}
	for i := range items {
		it := &items[i]
		if !highlightKinds[it.Kind] || seen[it.ID] || int(it.ID) >= len(t.nodes) {
			continue
		}
		seen[it.ID] = true
		n := t.nodes[it.ID]
		if n == nil || n.deleted || h.owns(n) {
			continue
		}
		if b := t.surfaceBounds(n); b.Size.Width > 0 && b.Size.Height > 0 {
			boxes = append(boxes, b)
		}
	}
	if len(boxes) > 0 {
		h.owner.Post(func() { h.show(boxes) })
	}
}

// owns reports whether n is one of the highlights' own views.
func (h *highlights) owns(n *node) bool {
	root := h.root.n
	for ; n != nil; n = n.parent {
		if n == root {
			return true
		}
	}
	return false
}

// show adds an outline for each of bounds, and takes them away after highlightFor.
func (h *highlights) show(bounds []Bounds) {
	first := h.next
	added := make([]highlightBox, len(bounds))
	for i, b := range bounds {
		h.next++
		added[i] = highlightBox{id: h.next, bounds: b}
	}
	h.boxes.Update(func(bs []highlightBox) []highlightBox { return append(slices.Clip(bs), added...) })
	h.owner.AfterFunc(highlightFor, func() {
		h.boxes.Update(func(bs []highlightBox) []highlightBox {
			kept := bs[:0:0]
			for _, b := range bs {
				if b.id <= first || b.id > first+uint64(len(added)) {
					kept = append(kept, b)
				}
			}
			return kept
		})
	})
}
