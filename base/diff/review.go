package diff

import (
	"strings"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// What a reviewer does: selecting lines on the numbers, the app's
// annotations under lines and its control to add one.

// LineRange is lines Start to End (both included, from 0) of a side's
// text: the lines selected on the numbers, or those an annotation is
// added to.
type LineRange struct {
	Side       Side
	Start, End int
}

// Annotation is the app's content under a line (a review comment, a
// thread), drawn by RenderAnnotation across its side, as tall as it lays
// out. ID tells it apart from the others; Line is from 0 of Side's text.
// It shows while its line does.
type Annotation struct {
	ID   string
	Side Side
	Line int
}

// hoverLeave is how long after the pointer leaves a line its add control
// stays, so moving onto the control keeps it.
const hoverLeave = 16 * time.Millisecond

// lineOf is the line row r shows on side, -1 for none.
func (r row) lineOf(side Side) int {
	if r.kind == rowSeparator || r.kind == rowFile {
		return -1
	}
	if side == Old {
		return r.old
	}
	return r.new
}

// selectLine selects line on side from a press: extending from the
// anchor with shift.
func (v *dv) selectLine(side Side, line int, extend bool) {
	if line < 0 {
		return
	}
	if !extend || v.sel == nil || v.sel.Side != side {
		v.anchor = line
	}
	v.setSelection(&LineRange{Side: side, Start: min(v.anchor, line), End: max(v.anchor, line)}, true)
}

// setSelection sets the selected lines, reporting a change with report.
func (v *dv) setSelection(r *LineRange, report bool) {
	same := r == nil && v.sel == nil || r != nil && v.sel != nil && *r == *v.sel
	if same {
		return
	}
	if r != nil {
		c := *r
		v.sel = &c
	} else {
		v.sel = nil
	}
	v.selRev.Update(func(n int) int { return n + 1 })
	if report && v.p.OnLineSelectionChange != nil {
		v.p.OnLineSelectionChange(v.sel)
	}
}

// selected reports whether line on side is selected (tracked).
func (v *dv) selected(side Side, line int) bool {
	v.selRev.Get()
	return line >= 0 && v.sel != nil && v.sel.Side == side && v.sel.Start <= line && line <= v.sel.End
}

// gutterSide is the side a press on row r's gutter at x selects: the
// number column pressed in unified, else the side the row has.
func (v *dv) gutterSide(r row, side int, x float32) Side {
	if side >= 0 {
		return Side(side)
	}
	if r.kind == rowEqual && !v.p.NoLineNumbers {
		if x < v.numberWidth() {
			return Old
		}
		return New
	}
	if r.kind == rowDeleted {
		return Old
	}
	return New
}

// dragLines extends the selection to the line on its side at height y
// of the rows; rows without a line there keep the last one.
func (v *dv) dragLines(y float32) {
	if v.sel == nil {
		return
	}
	if line := v.rows[v.rowAt(y)].lineOf(v.sel.Side); line >= 0 {
		v.selectLine(v.sel.Side, line, true)
	}
}

// selectedText is the selected lines' text.
func (v *dv) selectedText() string {
	if v.sel == nil {
		return ""
	}
	lines := v.m.new
	if v.sel.Side == Old {
		lines = v.m.old
	}
	var b strings.Builder
	for i := max(v.sel.Start, 0); i <= v.sel.End && i < len(lines); i++ {
		b.WriteString(lines[i])
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// hover is the pointer over line on side (line -1: off the lines); the
// hover ends a moment after the last view reporting it leaves.
func (v *dv) hover(side Side, line int) {
	if v.hoverStop != nil {
		v.hoverStop()
		v.hoverStop = nil
	}
	if line >= 0 {
		v.hovered.Set(LineRange{Side: side, Start: line, End: line})
		return
	}
	v.hoverStop = v.cx.AfterFunc(hoverLeave, func() {
		v.hoverStop = nil
		v.hovered.Set(LineRange{Start: -1, End: -1})
	})
}

// annotationsByRow is the height the annotations add under each row: the
// taller side's stack in split, all of them unified.
func (v *dv) annotationsByRow() map[int]float32 {
	if len(v.annotations) == 0 {
		return nil
	}
	stacks := map[[2]int]float32{}
	out := map[int]float32{}
	for _, a := range v.annotations {
		row := v.rowOfLine(a.Side, a.Line)
		if row < 0 {
			continue
		}
		key := [2]int{row, int(a.Side)}
		if v.p.Unified {
			key[1] = 0
		}
		stacks[key] += v.annH[a.ID]
		out[row] = max(out[row], stacks[key])
	}
	return out
}

// rowOfLine is the row showing line on side, -1 when none does.
func (v *dv) rowOfLine(side Side, line int) int {
	m := v.newRow
	if side == Old {
		m = v.oldRow
	}
	if line < 0 || line >= len(m) {
		return -1
	}
	return m[line]
}

// annotationTop is where annotation i starts: under its row's lines,
// after those before it in its stack; ok is false while its line hides.
func (v *dv) annotationTop(i int) (float32, bool) {
	a := v.annotations[i]
	row := v.rowOfLine(a.Side, a.Line)
	if row < 0 || row >= len(v.rows) {
		return 0, false
	}
	y := v.tops[row] + float32(v.lineCount(v.rows[row]))*v.lh
	for _, b := range v.annotations[:i] {
		if v.rowOfLine(b.Side, b.Line) == row && (v.p.Unified || b.Side == a.Side) {
			y += v.annH[b.ID]
		}
	}
	return y, true
}

// annotationsView is every annotation, under its line across its side
// (the view unified), each reporting its height as it lays out.
func (v *dv) annotationsView() core.Node {
	if v.p.RenderAnnotation == nil {
		return core.Group{}
	}
	return core.Each(func() []string {
		v.geo.Get()
		ids := make([]string, len(v.annotations))
		for i, a := range v.annotations {
			ids[i] = a.ID
		}
		return ids
	}, func(id string) string { return id }, func(cx *core.Scope, id func() string, i func() int) core.Node {
		a := v.annotations[core.Untracked(cx, i)]
		return core.View{
			StyleFunc: func() core.Style {
				v.geo.Get()
				v.viewport.Get()
				at := i()
				if at >= len(v.annotations) {
					return core.Style{Display: core.DisplayNone}
				}
				top, ok := v.annotationTop(at)
				if !ok {
					return core.Style{Display: core.DisplayNone}
				}
				left, width := float32(0), v.viewWidth()
				if !v.p.Unified {
					width = v.halfWidth()
					if v.annotations[at].Side == New {
						left = width
					}
				}
				return merge(core.Style{Position: core.Absolute, Top: core.Pt(core.Points(top)), Left: core.Pt(core.Points(left)),
					Width: core.Pt(core.Points(width))}, v.styles().Annotation)
			},
			Class: v.p.Classes.Annotation,
			OnLayout: func(b core.Bounds) {
				key := id()
				if h := float32(b.Size.Height); v.annH[key] != h {
					v.annH[key] = h
					v.relayout()
				}
			},
			Children: []core.Node{v.p.RenderAnnotation(cx, a)},
		}
	})
}

// addControls are the app's controls to add an annotation: at the line
// the pointer is over and at the last selected line, the gutter's width
// and a line tall, the control at its right. A control gets the selected
// lines when its line is one of them, else its line.
func (v *dv) addControls() core.Node {
	if v.p.RenderAddAnnotation == nil {
		return core.Group{}
	}
	return core.Each(func() []LineRange {
		v.selRev.Get()
		h := v.hovered.Get()
		var out []LineRange
		if v.sel != nil {
			out = append(out, LineRange{Side: v.sel.Side, Start: v.sel.End, End: v.sel.End})
		}
		if h.Start >= 0 && (len(out) == 0 || out[0] != h) {
			out = append(out, h)
		}
		return out
	}, func(r LineRange) LineRange { return r }, func(cx *core.Scope, at func() LineRange, _ func() int) core.Node {
		place := core.Untracked(cx, at)
		rng := place
		if v.sel != nil && v.sel.Side == place.Side && v.sel.Start <= place.Start && place.Start <= v.sel.End {
			rng = *v.sel
		}
		return core.View{
			StyleFunc: func() core.Style {
				v.geo.Get()
				v.viewport.Get()
				row := v.rowOfLine(place.Side, place.Start)
				if row < 0 || row >= len(v.rows) {
					return core.Style{Display: core.DisplayNone}
				}
				left := float32(0)
				if !v.p.Unified && place.Side == New {
					left = v.halfWidth()
				}
				return merge(core.Style{Position: core.Absolute, Top: core.Pt(core.Points(v.tops[row])), Left: core.Pt(core.Points(left)),
					Width: core.Pt(core.Points(v.gutterWidth())), Height: core.Pt(core.Points(v.lh)), Direction: core.Row,
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyEnd}, v.styles().AddAnnotation)
			},
			Class:      v.p.Classes.AddAnnotation,
			OnHoverIn:  func() { v.hover(place.Side, place.Start) },
			OnHoverOut: func() { v.hover(place.Side, -1) },
			Children:   []core.Node{v.p.RenderAddAnnotation(cx, rng)},
		}
	})
}

// SelectedLines returns the lines selected on the numbers; ok is false
// for none.
func (r *Ref) SelectedLines() (LineRange, bool) {
	if r.v == nil || r.v.sel == nil {
		return LineRange{}, false
	}
	return *r.v.sel, true
}

// SetSelectedLines selects lines of a side, nil for none, without
// OnLineSelectionChange.
func (r *Ref) SetSelectedLines(lines *LineRange) {
	if r.v != nil {
		if lines != nil {
			r.v.anchor = lines.Start
		}
		r.v.setSelection(lines, false)
	}
}

// SelectedText returns the selected lines' text.
func (r *Ref) SelectedText() string {
	if r.v == nil {
		return ""
	}
	return r.v.selectedText()
}
