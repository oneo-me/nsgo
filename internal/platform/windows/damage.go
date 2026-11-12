//go:build windows

package windows

import (
	"math"
	"slices"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Partial frames, as NSH's: a frame draws only what changed since the
// back buffer was last drawn, and presents only what changed since the
// last frame.
//
//   - What changed: redrawView queues a view whose drawing changed, with
//     its subtree, and notes where the last frame painted that subtree
//     (view.tree). At the frame, measure walks the paths down to the
//     queued views and records where each draws now, and the queued
//     subtrees' new areas join the old ones. A view taken out of the tree
//     is queued before it leaves; one inserted, after. redrawAll draws
//     the whole window: for what applies to all of it (the environment,
//     the window's chrome, its size) and for any change not tied to a
//     view. A paint the system asks for with nothing queued draws
//     nothing, as DWM keeps what was presented.
//   - What is drawn: the swap chain is FLIP_SEQUENTIAL with two buffers,
//     so its back buffer holds the frame of two presents ago, and a frame
//     draws its own damage and the last frame's. The areas, in pixels,
//     are merged where that costs little until at most maxDamage are
//     left, each is cleared, and the tree is drawn once inside them. A
//     view whose subtree meets none is left out, and so is one whose
//     subtree shows nowhere (outside a clipping ancestor), in every
//     frame.
//   - What is presented: Present1 with the frame's own areas, so DWM
//     composes only those.
//
// Windows with web views, materials or native components' windows, a
// view drawn through effects (filters, a transform out of its plane) in
// the way of a change, a new or resized swap chain, and the frame after
// a whole one draw the whole window.

// maxDamage is the most areas a frame draws in.
const maxDamage = 24

// damage is what the next frame draws again.
type damage struct {
	whole bool
	// old are the areas, in the window's points, the queued views and
	// those taken out painted in the last frame.
	old    []rectF
	queued []*view
	// last is what the last frame changed, in pixels, all of it when
	// lastWhole.
	last      []rect
	lastWhole bool
}

// noArea is an empty area, which union and meets ignore.
var noArea = rectF{float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))}

// everywhere marks an area that is not known, such as a subtree drawn
// through effects: a change of it draws the whole window.
var everywhere = rectF{float32(math.Inf(-1)), float32(math.Inf(-1)), float32(math.Inf(1)), float32(math.Inf(1))}

func union(a, b rectF) rectF {
	return rectF{min(a.left, b.left), min(a.top, b.top), max(a.right, b.right), max(a.bottom, b.bottom)}
}

func unknown(r rectF) bool { return math.IsInf(float64(r.left), -1) }

// redrawAll has the next frame draw the whole window.
func (w *window) redrawAll() {
	w.damage.whole = true
	if w.hwnd != 0 {
		invalidateRect.Call(w.hwnd, 0, 0)
	}
}

// redrawPending has a frame draw what was queued.
func (w *window) redrawPending() {
	if w.hwnd != 0 {
		invalidateRect.Call(w.hwnd, 0, 0)
	}
}

// redrawView has the next frame draw v and its subtree again, where they
// were and where they are then.
func (w *window) redrawView(v *view) {
	d := &w.damage
	if v == nil || d.whole {
		w.redrawAll()
		return
	}
	if !v.queued {
		v.queued = true
		d.queued = append(d.queued, v)
		if !v.tree.empty() {
			d.old = append(d.old, v.tree)
		}
	}
	if w.hwnd != 0 {
		invalidateRect.Call(w.hwnd, 0, 0)
	}
}

// up returns the view v draws in.
func up(v *view) *view {
	if v.leftFrom != nil {
		return v.leftFrom
	}
	return v.parent
}

// measuring is a walk recording where views draw.
type measuring struct {
	w           *window
	perspective driver.Points
	of          driver.Size
}

// measure records where root's subtree draws now: all of it, or only
// the paths down to the queued views and their subtrees.
func (w *window) measure(all bool) {
	root := w.view(w.root)
	if root == nil {
		return
	}
	var client rect
	getClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
	k := float32(1 / w.scale)
	window := rectF{0, 0, float32(client.Right) * k, float32(client.Bottom) * k}
	m := measuring{w: w}
	f := frameOf(root)
	m.visit(root, identity, f.Origin.X, f.Origin.Y, window, all, false)
}

// visit measures v, placed as painter.draw places it, inside clip;
// effects says an ancestor draws through effects. Unless all, a view
// neither queued nor above a queued one keeps what it recorded.
func (m *measuring) visit(v *view, parent matrix, x, y driver.Points, clip rectF, all, effects bool) {
	if !all && !v.queued && !v.below {
		return
	}
	all = all || v.queued
	v.queued, v.below = false, false
	pl, ok := m.w.place(v, parent, x, y, m.perspective, m.of)
	if !ok {
		v.area, v.tree = noArea, noArea
		m.forget(v)
		return
	}
	s := pl.s
	effects = effects || pl.filters || !pl.flat
	size := frameOf(v).Size
	bounds := rectF{0, 0, float32(size.Width), float32(size.Height)}
	if effects {
		v.area = everywhere
	} else {
		v.area = mapBounds(pl.world, extent(s, bounds)).intersect(clip)
		if v.area.empty() {
			v.area = noArea
		}
	}
	inner := clip
	if s.Clip || v.kind == driver.KindScroll {
		inner = clip.intersect(mapBounds(pl.world, bounds))
	}
	perspective, of := m.perspective, m.of
	m.perspective, m.of = s.ChildPerspective, size
	tree := v.area
	world := pl.world
	for _, l := range v.leaving {
		c := l.v
		m.visit(c, world, c.frame.Origin.X-v.offset.X, c.frame.Origin.Y-v.offset.Y, inner, all, effects)
		tree = union(tree, c.tree)
	}
	for _, c := range v.children {
		f := frameOf(c)
		m.visit(c, world, f.Origin.X-v.offset.X, f.Origin.Y-v.offset.Y, inner, all, effects)
		tree = union(tree, c.tree)
	}
	m.perspective, m.of = perspective, of
	v.tree = tree
}

// forget clears what v's subtree recorded, which draws nothing now.
func (m *measuring) forget(v *view) {
	for _, c := range v.children {
		c.queued, c.below = false, false
		c.area, c.tree = noArea, noArea
		m.forget(c)
	}
	for _, l := range v.leaving {
		l.v.queued, l.v.below = false, false
		l.v.area, l.v.tree = noArea, noArea
		m.forget(l.v)
	}
}

// extent is what a view with style s paints of bounds, its own points,
// with its shadows and outline: conservative, as blurs fade out.
func extent(s *driver.ViewStyle, bounds rectF) rectF {
	r := bounds
	grow := func(dx, dy, by float32) {
		r = union(r, rectF{bounds.left + dx - by, bounds.top + dy - by, bounds.right + dx + by,
			bounds.bottom + dy + by})
	}
	if visible(s.ShadowColor) {
		grow(float32(s.ShadowOffset.X), float32(s.ShadowOffset.Y), 2*max(float32(s.ShadowRadius), 0))
	}
	if e := s.Effects; e != nil {
		for _, b := range e.Shadows {
			if !b.Inset {
				grow(float32(b.Offset.X), float32(b.Offset.Y), 1.5*max(float32(b.Blur), 0)+max(float32(b.Spread), 0))
			}
		}
	}
	if o := s.Outline; o.Visible() {
		grow(0, 0, max(float32(o.Offset), 0)+float32(o.Width))
	}
	// Anti-aliasing.
	return rectF{r.left - 1, r.top - 1, r.right + 1, r.bottom + 1}
}

// idle reports whether nothing was queued: a paint the system asked
// for, which DWM needs nothing new for, since it keeps what the flip
// model presented last.
func (d *damage) idle() bool { return !d.whole && len(d.queued) == 0 && len(d.old) == 0 }

// frameDamage returns what the frame changes, in pixels inside the
// client area, all of the window when changedAll, and what it draws:
// that and what the last frame changed, all of it when drawAll. whole
// says the frame draws all of the window anyway.
func (w *window) frameDamage(client rect, whole bool) (changed, drawn []rect, changedAll, drawAll bool) {
	d := &w.damage
	changedAll = whole || d.whole || d.idle()
	// The paths down to the queued views, where they are now, are
	// measured again.
	for _, v := range d.queued {
		for u := up(v); u != nil && !u.below; u = up(u) {
			u.below = true
		}
	}
	w.measure(changedAll)
	if !changedAll {
		areas := d.old
		for _, v := range d.queued {
			if !v.deleted || v.retired {
				areas = append(areas, v.tree)
			}
		}
		for _, a := range areas {
			if unknown(a) {
				changedAll = true
				break
			}
			if r := pixels(a, w.scale).intersect(client); !r.empty() {
				changed = append(changed, r)
			}
		}
	}
	for _, v := range d.queued {
		v.queued = false
	}
	clear(d.queued)
	d.queued, d.old, d.whole = d.queued[:0], d.old[:0], false
	drawAll = changedAll || d.lastWhole
	if changedAll {
		changed = nil
	} else {
		changed = merged(changed)
	}
	if !drawAll {
		drawn = merged(append(slices.Clone(changed), d.last...))
	}
	d.lastWhole, d.last = changedAll, append(d.last[:0], changed...)
	return changed, drawn, changedAll, drawAll
}

// pixels returns the whole pixels covering r, in points at scale.
func pixels(r rectF, scale float64) rect {
	return rect{int32(math.Floor(float64(r.left) * scale)), int32(math.Floor(float64(r.top) * scale)),
		int32(math.Ceil(float64(r.right) * scale)), int32(math.Ceil(float64(r.bottom) * scale))}
}

func (r rect) intersect(o rect) rect {
	return rect{max(r.Left, o.Left), max(r.Top, o.Top), min(r.Right, o.Right), min(r.Bottom, o.Bottom)}
}

func (r rect) union(o rect) rect {
	return rect{min(r.Left, o.Left), min(r.Top, o.Top), max(r.Right, o.Right), max(r.Bottom, o.Bottom)}
}

func (r rect) area() int64 {
	if r.empty() {
		return 0
	}
	return int64(r.Right-r.Left) * int64(r.Bottom-r.Top)
}

func (r rect) touches(o rect) bool {
	return r.Left <= o.Right && o.Left <= r.Right && r.Top <= o.Bottom && o.Top <= r.Bottom
}

// merged joins the rectangles of rs that touch, or whose union covers
// little more than they do, then the pairs whose union adds least, until
// at most maxDamage are left.
func merged(rs []rect) []rect {
	rs = slices.DeleteFunc(rs, rect.empty)
	for joined := true; joined; {
		joined = false
		for i := 0; i < len(rs) && !joined; i++ {
			for j := i + 1; j < len(rs); j++ {
				u := rs[i].union(rs[j])
				if rs[i].touches(rs[j]) || u.area() <= rs[i].area()+rs[j].area() {
					rs[i] = u
					rs = slices.Delete(rs, j, j+1)
					joined = true
					break
				}
			}
		}
	}
	for len(rs) > maxDamage {
		bi, bj, best := 0, 1, int64(math.MaxInt64)
		for i := range rs {
			for j := i + 1; j < len(rs); j++ {
				if cost := rs[i].union(rs[j]).area() - rs[i].area() - rs[j].area(); cost < best {
					bi, bj, best = i, j, cost
				}
			}
		}
		rs[bi] = rs[bi].union(rs[bj])
		rs = slices.Delete(rs, bj, bj+1)
	}
	return rs
}

// skips reports whether the pass leaves v and its subtree out: they show
// nowhere, or meet none of the areas a partial frame draws.
func (p *painter) skips(v *view) bool {
	t := v.tree
	if t == (rectF{}) || unknown(t) {
		// Not measured yet, or drawn through effects.
		return false
	}
	if t.empty() {
		return true
	}
	if p.areas == nil {
		return false
	}
	for _, a := range p.areas {
		if !a.intersect(t).empty() {
			return false
		}
	}
	return true
}

// pushAreas clips what the pass draws to areas, in pixels, after
// clearing them to clear; the clip stays until the pass ends.
func (p *painter) pushAreas(areas []rect, clear colorF) {
	k := 1 / p.scale
	p.areas = p.areas[:0]
	for _, a := range areas {
		p.areas = append(p.areas, rectF{float32(a.Left) * k, float32(a.Top) * k, float32(a.Right) * k,
			float32(a.Bottom) * k})
	}
	p.setTransform(identity)
	for _, r := range p.areas {
		p.dc.call(rtPushAxisAlignedClip, uintptr(unsafe.Pointer(&r)), d2dAntialiasAliased)
		p.dc.call(rtClear, uintptr(unsafe.Pointer(&clear)))
		p.dc.call(rtPopAxisAlignedClip)
	}
	if len(p.areas) == 1 {
		r := p.areas[0]
		p.dc.call(rtPushAxisAlignedClip, uintptr(unsafe.Pointer(&r)), d2dAntialiasAliased)
		p.areaClip = 1
		return
	}
	g := p.pathGeometry(d2dFillWinding, func(sink *comObject) {
		for _, r := range p.areas {
			addSegments(sink, driver.RoundedRect(nil, float64(r.left), float64(r.top), float64(r.right-r.left),
				float64(r.bottom-r.top), driver.Corners{}, false, false))
		}
	})
	if g == nil {
		return
	}
	p.pushLayerRaw(1, g, nil)
	g.release()
	p.areaClip = 2
}

// popAreas pops what pushAreas pushed.
func (p *painter) popAreas() {
	switch p.areaClip {
	case 1:
		p.dc.call(rtPopAxisAlignedClip)
	case 2:
		p.dc.call(rtPopLayer)
	}
	p.areaClip = 0
}
