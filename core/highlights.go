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

// inspectColor is the fill and outline of the view DevTools inspect.
var inspectColor = RGBA(0x2f9bff, 0.25)

// highlights are a tree's UpdateHighlights: the boxes it shows, and its overlay, whose views it leaves out. DevTools also draw the view they inspect there, and take presses there while they pick one.
type highlights struct {
	owner *Scope
	root  *Ref
	boxes *Signal[[]highlightBox]
	next  uint64
	// updates is whether commits are outlined: DevTools turn it off and on.
	updates bool
	inspect *Signal[Bounds]
	picking *Signal[bool]
	// hover and picked hear the views the pointer is over and the one pressed while picking.
	hover, picked func(id uint32)
}

type highlightBox struct {
	id     uint64
	bounds Bounds
}

func (t *Tree) mountHighlights(cx *Scope, parent *node) {
	if cx.rt.post == nil {
		return
	}
	// Its own scope holds what it runs, which DevTools leave out of the app's computations.
	cx = cx.Child()
	h := &highlights{owner: cx, root: new(Ref), boxes: NewSignalFunc[[]highlightBox](cx, nil, nil), updates: true,
		inspect: NewSignal(cx, Bounds{}), picking: NewSignal(cx, false)}
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
	inspected := View{PointerEvents: PointerEventsNone, StyleFunc: func() Style {
		r := h.inspect.Get()
		if r.Size.Width <= 0 || r.Size.Height <= 0 {
			return Style{Display: DisplayNone}
		}
		return Style{Position: Absolute, Left: Pt(r.Origin.X), Top: Pt(r.Origin.Y), Width: Pt(r.Size.Width), Height: Pt(r.Size.Height),
			Background: inspectColor, BorderWidth: 1, BorderColor: highlightColor}
	}}
	// While DevTools pick, a view over the whole surface takes the pointer: what it is over and what it presses is the app's view under it.
	catcher := Show{When: h.picking.Get, Then: func(cx *Scope) Node {
		return View{TestID: "nsgo-devtools-pick", Style: Style{Position: Absolute, Left: Pt(0), Top: Pt(0), Width: Pct(100), Height: Pct(100)},
			OnMouseMove: func(e MouseEvent) { t.pickAt(e.PagePosition, false) },
			OnPressDown: func(e PressEvent) { t.pickAt(e.PagePosition, true) },
			OnPress:     func(PressEvent) {}}
	}}
	t.mount(cx, parent, Overlay{Ref: h.root, PointerEvents: PointerEventsBoxNone, TestID: "nsgo-update-highlights",
		Style:    Style{Width: Pct(100), Height: Pct(100)},
		Children: []Node{Each(h.boxes.Get, func(b highlightBox) uint64 { return b.id }, box), inspected, catcher}})
}

// pickAt finds the app's innermost view at p, past DevTools' own, outlines it and reports it: as picked when pressed, which ends picking.
func (t *Tree) pickAt(p Point, pressed bool) {
	h := t.highlights
	if h == nil {
		return
	}
	id, _ := hitWith(t.view, p, func(n *node) bool { return !h.owns(n) })
	n := t.node(id)
	if n == nil {
		return
	}
	h.inspect.Set(t.surfaceBounds(n))
	if pressed {
		h.picking.Set(false)
		if h.picked != nil {
			h.picked(uint32(id))
		}
	} else if h.hover != nil {
		h.hover(uint32(id))
	}
}

// highlightKinds are the view-changing kinds of item: what a commit made, moved, restyled or filled anew.
var highlightKinds = map[driver.ItemKind]bool{
	driver.ItemCreate: true, driver.ItemFrame: true, driver.ItemViewStyle: true, driver.ItemText: true,
	driver.ItemInputValue: true, driver.ItemImage: true, driver.ItemCanvas: true, driver.ItemShader: true,
}

// noteChanges outlines the views items changed, other than the highlights' own, after the commit that sent them: it posts them, as a commit sets no signal.
func (t *Tree) noteChanges(items []driver.MountItem) {
	h := t.highlights
	if !h.updates {
		return
	}
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
