package diff

import (
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// What the app's actions on lines take: the change under the pointer, or
// the lines the user selected, framed across the view.

// regionContext is how many unchanged lines a change's frame takes in on
// each side, which the pointer stays on the change over.
const regionContext = 2

// Region is what RenderRegionActions acts on: the changed lines of Old and
// New it holds, from 0, and whether the user selected them rather than the
// pointer being over a change.
type Region struct {
	Old, New []int
	Selected bool
}

// rowSpan is rows first to last, framed; first is -1 for none.
type rowSpan struct {
	first, last int
	selected    bool
}

// noRows is no rows selected by a drag.
var noRows = [2]int{-1, -1}

// activeRows are the rows framed: those a drag or the numbers selected,
// else the change the pointer is over (tracked).
func (v *dv) activeRows() rowSpan {
	v.rev.Get()
	v.selRev.Get()
	if d := v.rowSel.Get(); d[0] >= 0 {
		return rowSpan{min(d[0], d[1]), max(d[0], d[1]), true}
	}
	if s := v.sel; s != nil {
		a, b := v.rowOfLine(s.Side, s.Start), v.rowOfLine(s.Side, s.End)
		if a >= 0 && b >= 0 {
			return rowSpan{min(a, b), max(a, b), true}
		}
	}
	h := v.hovered.Get()
	if h.Start < 0 {
		return rowSpan{first: -1}
	}
	return v.changeRows(v.rowOfLine(h.Side, h.Start))
}

// changeRows are the rows of the change row i is in or next to, with the
// unchanged rows around it.
func (v *dv) changeRows(i int) rowSpan {
	if i = v.changeRow(i); i < 0 {
		return rowSpan{first: -1}
	}
	hunk := v.rows[i].hunk
	first, last := i, i
	for first > 0 && v.rows[first-1].hunk == hunk {
		first--
	}
	for last+1 < len(v.rows) && v.rows[last+1].hunk == hunk {
		last++
	}
	for k := 0; k < regionContext && first > 0 && v.rows[first-1].kind == rowEqual; k++ {
		first--
	}
	for k := 0; k < regionContext && last+1 < len(v.rows) && v.rows[last+1].kind == rowEqual; k++ {
		last++
	}
	return rowSpan{first: first, last: last}
}

// changeRow is row i if it is a change's, else the nearest change's row
// within regionContext unchanged rows, the one above first; -1 for none.
func (v *dv) changeRow(i int) int {
	if i < 0 || i >= len(v.rows) {
		return -1
	}
	up, down := i, i
	for k := 0; k <= regionContext; k++ {
		for _, at := range []int{up, down} {
			if at >= 0 && at < len(v.rows) && v.rows[at].hunk >= 0 {
				return at
			}
		}
		if up >= 0 && v.rows[up].kind != rowEqual {
			up = -1
		}
		if down < len(v.rows) && v.rows[down].kind != rowEqual {
			down = len(v.rows)
		}
		up, down = max(up-1, -1), min(down+1, len(v.rows))
	}
	return -1
}

// regionOf is the changed lines of rows s.
func (v *dv) regionOf(s rowSpan) Region {
	r := Region{Selected: s.selected}
	for _, x := range v.rows[s.first : s.last+1] {
		if x.hunk < 0 {
			continue
		}
		if x.old >= 0 {
			r.Old = append(r.Old, x.old)
		}
		if x.new >= 0 {
			r.New = append(r.New, x.new)
		}
	}
	return r
}

// region is the frame around the active rows and the app's actions at its
// top right, kept in view while the rows scroll under them.
func (v *dv) region() core.Node {
	if v.p.RenderRegionActions == nil {
		return core.Group{}
	}
	active := core.NewMemo(v.cx, v.activeRows)
	frame := core.View{
		PointerEvents: core.PointerEventsNone,
		Class:         v.p.Classes.Region,
		StyleFunc: func() core.Style {
			a := active.Get()
			v.geo.Get()
			if a.first < 0 || a.last+1 >= len(v.tops) {
				return core.Style{Display: core.DisplayNone}
			}
			return merge(core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(core.Points(v.tops[a.first])),
				Bottom: core.Pt(core.Points(v.height() - v.tops[a.last+1]))}, v.styles().Region)
		},
	}
	actions := core.Dynamic(func(cx *core.Scope) core.Node {
		a := active.Get()
		if a.first < 0 {
			return core.Group{}
		}
		region := core.Untracked(cx, func() Region { return v.regionOf(a) })
		if len(region.Old)+len(region.New) == 0 {
			return core.Group{}
		}
		return core.View{
			StyleFunc: func() core.Style {
				v.geo.Get()
				if a.last+1 >= len(v.tops) {
					return core.Style{Display: core.DisplayNone}
				}
				top := max(v.tops[a.first], min(float32(v.scrollY.Get()), v.tops[a.last+1]-v.lh))
				return core.Style{Position: core.Absolute, Right: core.Pt(8), Top: core.Pt(core.Points(top + 4))}
			},
			// The pointer on the actions stays on the lines under them,
			// and a press there leaves the selection's lines framed.
			OnMouseEnter: func(core.MouseEvent) {
				v.onActions = true
				if h := v.hovered.GetUntracked(); h.Start >= 0 {
					v.hover(h.Side, h.Start)
				}
			},
			OnMouseLeave: func(core.MouseEvent) {
				v.onActions = false
				if v.cleared {
					v.cleared = false
					v.rowSel.Set(noRows)
				}
				v.hover(New, -1)
			},
			Children: []core.Node{v.p.RenderRegionActions(cx, region)},
		}
	})
	return core.Group{frame, actions}
}

// textSelected frames the whole rows of the text selection in box, in the
// rows' coordinates; none for no selection.
func (v *dv) textSelected(box core.Bounds) {
	if box.Size.Height <= 0 {
		if v.cleared = v.onActions; !v.cleared {
			v.rowSel.Set(noRows)
		}
		return
	}
	v.cleared = false
	v.setSelection(nil, true)
	top := float32(box.Origin.Y)
	v.rowSel.Set([2]int{v.rowAt(top), v.rowAt(top + float32(box.Size.Height) - 1)})
}

// RevertRegion puts r's lines back as they are in the old text and reports
// the texts. It takes two texts' lines, not a patch's.
func (r *Ref) RevertRegion(x Region) {
	if v := r.v; v != nil && v.p.OnChange != nil && !slices.ContainsFunc(v.segs, func(s segment) bool { return s.patch }) {
		o, n := v.m.revertLines(x.Old, x.New)
		v.load(o, n, "")
		v.p.OnChange(Change{OldValue: o, NewValue: n})
	}
}

// AcceptRegion makes r's lines the old text's, as staging them does, and
// reports the texts. It takes two texts' lines, not a patch's.
func (r *Ref) AcceptRegion(x Region) {
	if v := r.v; v != nil && v.p.OnChange != nil && !slices.ContainsFunc(v.segs, func(s segment) bool { return s.patch }) {
		o, n := v.m.acceptLines(x.Old, x.New)
		v.load(o, n, "")
		v.p.OnChange(Change{OldValue: o, NewValue: n})
	}
}
