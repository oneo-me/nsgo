package diff

import (
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// HunkBounds is the box of change Index's rows: wrapped lines and
// annotations count, and a collapsed file's changes have none.
type HunkBounds struct {
	Index  int
	Bounds core.Bounds
}

// hunkBounds is every change's box across width.
func (v *dv) hunkBounds(width core.Points) []HunkBounds {
	var out []HunkBounds
	for i, r := range v.rows {
		if r.hunk < 0 {
			continue
		}
		top, bottom := core.Points(v.tops[i]), core.Points(v.tops[i+1])
		if n := len(out) - 1; n >= 0 && out[n].Index == r.hunk {
			out[n].Bounds.Size.Height = bottom - out[n].Bounds.Origin.Y
			continue
		}
		out = append(out, HunkBounds{Index: r.hunk, Bounds: core.Bounds{Origin: core.Point{Y: top}, Size: core.Size{Width: width, Height: bottom - top}}})
	}
	return out
}

// reportPosition reports the scroll position once a value of it changes,
// a relayout too, and the changes' boxes, which can move while the rows
// keep their height, from the first layout on.
func (v *dv) reportPosition() {
	if v.p.OnScroll != nil {
		var last core.ScrollEvent
		v.cx.Effect(func() {
			v.rev.Get()
			v.geo.Get()
			y, size := v.scrollY.Get(), v.viewport.Get()
			if size == (core.Size{}) {
				return
			}
			height := core.Points(v.height())
			now := core.ScrollEvent{Offset: core.Point{Y: y}, ContentSize: core.Size{Width: size.Width, Height: height}, Size: size, ZoomScale: 1}
			if now != last {
				last = now
				v.cx.Untrack(func() { v.p.OnScroll(now) })
			}
		})
	}
	if v.p.OnHunkBoundsChange != nil {
		width := core.NewMemo(v.cx, func() core.Points { return v.viewport.Get().Width })
		var last []HunkBounds
		seen := false
		v.cx.Effect(func() {
			v.rev.Get()
			v.geo.Get()
			w := width.Get()
			if w == 0 {
				return
			}
			now := v.hunkBounds(w)
			if !seen || !slices.Equal(now, last) {
				seen, last = true, now
				v.cx.Untrack(func() { v.p.OnHunkBoundsChange(now) })
			}
		})
	}
}

// ScrollOffset returns how far the rows are scrolled.
func (r *Ref) ScrollOffset() core.Points {
	if r.v == nil {
		return 0
	}
	return r.v.scrollY.GetUntracked()
}

// SetScrollOffset scrolls the rows to y at once, as far as they go, for a
// thumb that follows the pointer.
func (r *Ref) SetScrollOffset(y core.Points) {
	v := r.v
	if v == nil {
		return
	}
	y = max(min(y, core.Points(v.height())-v.viewport.GetUntracked().Height), 0)
	v.vs.ScrollTo(core.Point{Y: y}, false)
	v.scrollY.Set(y)
	v.scrolledTo(float32(y))
}

// HunkBounds returns each change's box across the viewport, in the rows'
// coordinates, for an overview ruler.
func (r *Ref) HunkBounds() []HunkBounds {
	if r.v == nil {
		return nil
	}
	return r.v.hunkBounds(r.v.viewport.GetUntracked().Width)
}
