//go:build linux && !android

package linux

import (
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// layoutAnim is a view's part of a commit's LayoutAnimation, as NSH's
// transition.rs runs it: the frame moving from where the view showed to
// the one the commit gave it, and the view appearing or leaving. Moving
// changes only where the view is drawn (shownFrame); appearing and
// leaving multiply its opacity or scale it about its center, over its
// style.
type layoutAnim struct {
	move      *driver.LayoutTransition // nil while the frame stands
	moveStart time.Time
	from      driver.Bounds
	shown     driver.Bounds

	fade      *driver.LayoutTransition // nil once the view is in
	fadeStart time.Time
	out       bool    // leaving: the view is dying
	shownPart float64 // how much of the view shows, 0 to 1
}

// layoutBatch is what applying a commit with a LayoutAnimation keeps.
type layoutBatch struct {
	anim    *driver.LayoutAnimation
	at      time.Time
	created map[driver.NodeID]bool // the views the commit made
}

// newLayoutBatch returns the batch's layout animation, or nil when it
// has none or the user turned animation effects off.
func (w *window) newLayoutBatch(b *driver.MountBatch) *layoutBatch {
	if b.LayoutAnimation == nil || w.env.ReduceMotion {
		return nil
	}
	return &layoutBatch{anim: b.LayoutAnimation, at: time.Now(), created: map[driver.NodeID]bool{}}
}

// shownFrame is the frame v is drawn at: mid animation, or its own.
func (v *view) shownFrame() driver.Bounds {
	if l := v.layout; l != nil && l.move != nil {
		return l.shown
	}
	return v.frame
}

// layoutOpacity is what appearing or leaving multiplies v's opacity by.
func (v *view) layoutOpacity() float32 {
	if l := v.layout; l != nil && l.fade != nil && l.fade.Property == driver.LayoutOpacity {
		return float32(min(max(l.shownPart, 0), 1))
	}
	return 1
}

// layoutScale is how much appearing or leaving scales v.
func (v *view) layoutScale() float32 {
	if l := v.layout; l != nil && l.fade != nil && l.fade.Property == driver.LayoutScale {
		// GSK cannot invert a zero scale, which picking needs.
		return float32(max(l.shownPart, 1e-3))
	}
	return 1
}

// Modals come in as on the web and macOS: sliding up from below the
// surface over 300 ms or fading in over 200 ms, easing out; with reduced
// motion what slides fades.
var (
	modalSlide = driver.LayoutTransition{Curve: driver.LayoutEaseOut, Duration: 300 * time.Millisecond}
	modalFade  = driver.LayoutTransition{Curve: driver.LayoutEaseOut, Property: driver.LayoutOpacity, Duration: 200 * time.Millisecond}
)

// present brings modal v in.
func (w *window) present(v *view) {
	anim := v.modal
	if w.navigating(w.view(w.root)) {
		return // the dialog's shader draws its coming
	}
	if anim == driver.ModalSlide && w.env.ReduceMotion {
		anim = driver.ModalFade
	}
	now := time.Now()
	switch anim {
	case driver.ModalSlide:
		l := w.layoutOf(v)
		from := v.frame
		from.Origin.Y += max(v.frame.Size.Height, 1)
		l.move, l.moveStart, l.from, l.shown = &modalSlide, now, from, from
	case driver.ModalFade:
		l := w.layoutOf(v)
		l.fade, l.fadeStart, l.out, l.shownPart = &modalFade, now, false, 0
	default:
		return
	}
	w.moved(v)
}

// animatesLayout reports whether a commit's layout animation may move v:
// modals come and go on their own.
func animatesLayout(v *view) bool { return v.kind != driver.KindModal }

// framed applies the item's frame to v, from the frame it shows now to
// it with lb's Update when it existed before the commit, or with the
// item's own transition, else at once.
func (w *window) framed(v *view, it *driver.MountItem, lb *layoutBatch) {
	old := v.shownFrame()
	frame := it.Frame
	v.laid = frame
	frame.Origin.X += v.shift.X
	frame.Origin.Y += v.shift.Y
	v.frame = frame
	tr, start := it.Transition, time.Now()
	if lb != nil && lb.anim.Update != nil && !lb.created[v.id] {
		tr, start = lb.anim.Update, lb.at
	}
	if tr == nil || !animatesLayout(v) || old == frame {
		if v.layout != nil && v.layout.move != nil {
			v.layout.move = nil
			w.dropLayout(v)
		}
		return
	}
	l := w.layoutOf(v)
	l.move, l.moveStart, l.from, l.shown = tr, start, old, old
}

// inserted starts v appearing when the commit made it and inserted it
// into a view that was there before.
func (w *window) inserted(v, parent *view, lb *layoutBatch) {
	if lb == nil || lb.anim.Create == nil || !lb.created[v.id] || lb.created[parent.id] || !animatesLayout(v) {
		return
	}
	l := w.layoutOf(v)
	l.fade, l.fadeStart, l.out, l.shownPart = lb.anim.Create, lb.at, false, 0
	w.moved(v)
}

// contentAt returns where v's content shows in the window: its frame mid
// animation, carried through its parents', less the scroll offsets.
func contentAt(v *view) driver.Point {
	var at driver.Point
	for p := v; p != nil; p = p.parent {
		o := p.shownFrame().Origin
		at.X += o.X - p.offset.X
		at.Y += o.Y - p.offset.Y
	}
	return at
}

// match moves v, which the commit made, from where old, which it
// deletes, shows, with lb's Update rather than appearing, and hides old
// at once (driver.ItemMatch).
func (w *window) match(v, old *view, lb *layoutBatch) {
	if lb == nil || lb.anim.Update == nil || old == nil || v.parent == nil || old.parent == nil || old.style.Hidden {
		return
	}
	was, in := contentAt(old.parent), contentAt(v.parent)
	o := old.shownFrame()
	from := driver.Bounds{Origin: driver.Point{X: was.X + o.Origin.X - in.X, Y: was.Y + o.Origin.Y - in.Y}, Size: o.Size}
	if l := v.layout; l != nil && l.fade != nil && !l.out {
		l.fade, l.shownPart = nil, 1
	}
	old.style.Hidden = true
	w.showWidget(old)
	w.moved(old)
	l := w.layoutOf(v)
	l.move, l.moveStart, l.from, l.shown = lb.anim.Update, lb.at, from, from
	if v.widget != 0 {
		gtkWidgetQueueAllocate(v.widget)
	}
	w.moved(v)
}

// retiring reports whether v, deleted by the commit, leaves animated:
// when lb has a Delete and v is the top of the subtree deleted, still
// shown in a live parent.
func (w *window) retiring(v *view, lb *layoutBatch) bool {
	if lb == nil || lb.anim.Delete == nil || !animatesLayout(v) {
		return false
	}
	p := v.parent
	return p != nil && !p.deleted && v.widget != 0 && p.widget != 0 && gtkWidgetGetParent(v.widget) == p.widget
}

// retire deletes v but leaves its widget where it is, fading or shrinking
// out with lb's Delete; the subtree's own deletions keep their views in
// it (retireInner). The ID is free at once: the outgoing drawing belongs
// to the view, not to the ID.
func (w *window) retire(v *view, lb *layoutBatch) {
	v.detach()
	w.retireInner(v)
	gtkWidgetSetCanTarget(v.widget, 0)
	l := w.layoutOf(v)
	l.fade, l.fadeStart, l.out = lb.anim.Delete, lb.at, true
}

// retireInner deletes v, a view leaving or inside one, keeping what it
// draws until the view leaving goes.
func (w *window) retireInner(v *view) {
	w.forgetDestination(v)
	w.hovered.Forget(v.id)
	v.dying, v.deleted = true, true
	if w.views[v.id] == v {
		w.views[v.id] = nil
	}
	if w.focus == v {
		w.focus = nil
		gtkWindowSetFocus(w.toplevel(), 0)
	}
	if v.video != nil {
		w.stopProgress(v.video)
	}
}

// bury destroys a view that finished leaving, with the views inside it.
func (w *window) bury(v *view) {
	for _, c := range append([]*view(nil), v.children...) {
		if c.dying {
			w.bury(c)
		}
	}
	w.delete(v)
}

// layoutOf returns v's layout animation, made and ticking.
func (w *window) layoutOf(v *view) *layoutAnim {
	if v.layout == nil {
		v.layout = &layoutAnim{shownPart: 1}
		w.layouts = append(w.layouts, v)
		w.startClock()
	}
	return v.layout
}

// dropLayout forgets v's layout animation once nothing of it runs.
func (w *window) dropLayout(v *view) {
	if l := v.layout; l != nil && l.move == nil && l.fade == nil {
		v.layout = nil
	}
}

// stepLayout moves the layout animations to now, and reports whether any
// still runs.
func (w *window) stepLayout(now time.Time) bool {
	live := w.layouts[:0]
	for _, v := range w.layouts {
		l := v.layout
		if l == nil {
			continue
		}
		if l.move != nil {
			p, done := l.move.At(now.Sub(l.moveStart))
			old := l.shown
			if done {
				l.move = nil
			} else {
				l.shown = driver.LerpBounds(l.from, v.frame, p)
			}
			if v.shownFrame().Size != old.Size && v.widget != 0 {
				gtkWidgetQueueAllocate(v.widget)
			}
		}
		if l.fade != nil {
			p, done := l.fade.At(now.Sub(l.fadeStart))
			if l.out {
				l.shownPart = 1 - p
				if now.Sub(l.fadeStart) >= l.fade.End() {
					w.bury(v)
					continue
				}
			} else {
				l.shownPart = p
				if done {
					l.fade = nil
				}
			}
		}
		w.moved(v)
		if l.move == nil && l.fade == nil {
			v.layout = nil
			continue
		}
		live = append(live, v)
	}
	clear(w.layouts[len(live):])
	w.layouts = live
	return len(live) > 0
}
