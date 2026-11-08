package editor

import (
	"slices"
	"sort"

	"github.com/oneo-me/nsgo/core"
)

// rulerWidth is the overview ruler's width unless its part gives one, and
// rulerMarkMin the least height of a mark on it, as Monaco's.
const (
	rulerWidth    = 14
	rulerMarkMin  = 6
	gutterStrip   = 3
	gutterEdgeBar = 2
)

// GutterMark is a Decoration's mark in the gutter's strip: a bar the
// strip's width beside each line of its range, or a line across the strip
// between two lines. Its look is Class and Style.
type GutterMark struct {
	Placement GutterPlacement
	Class     string
	Style     core.Style
}

// GutterPlacement is where a GutterMark is drawn.
type GutterPlacement uint8

const (
	// GutterLines is a bar beside every line the range covers, an empty
	// range covering its line: added or modified lines, a breakpoint.
	GutterLines GutterPlacement = iota
	// GutterBefore is a line across the strip at the boundary before the
	// range's start: the top of its line when it starts one, else the
	// bottom of its line, for lines deleted there.
	GutterBefore
)

// OverviewRulerMark is a Decoration's mark on the overview ruler, the strip
// along the editor's right edge that maps the whole text to its height: the
// range's rows scaled to the ruler, at least 6 points tall, in one of its
// lanes. Its look is Class and Style.
type OverviewRulerMark struct {
	Lane  OverviewRulerLane
	Class string
	Style core.Style
}

// OverviewRulerLane is the part of the ruler's width a mark takes, as
// Monaco's lanes.
type OverviewRulerLane uint8

const (
	LaneCenter OverviewRulerLane = iota
	LaneLeft
	LaneRight
	LaneFull
)

// gutterPlace is where a gutter mark is on a row.
type gutterPlace uint8

const (
	placeRow gutterPlace = iota
	placeTop
	placeBottom
)

// rowMark is a decoration's gutter mark on a row.
type rowMark struct {
	deco  *Decoration
	place gutterPlace
}

// rulerMark is a decoration's mark on the overview ruler and the display
// rows it covers, the last past its end.
type rulerMark struct {
	deco       *Decoration
	first, end int
}

// hasGutterMarks says a decoration has a gutter mark: the strip is reserved
// while one has, so the text does not move as they come and go.
func (e *ed) hasGutterMarks() bool {
	for i := range e.marks {
		if d := e.marks[i].deco; d != nil && d.Gutter != nil {
			return true
		}
	}
	return false
}

// stripWidth is the gutter's strip of marks, 0 without any.
func (e *ed) stripWidth() float32 {
	if !e.strip {
		return 0
	}
	if e.p.GutterDecorationsWidth > 0 {
		return float32(e.p.GutterDecorationsWidth)
	}
	return gutterStrip
}

// lastLine is the last line a mark covers: a range ending at a line's
// start does not cover that line.
func lastLine(m *mark) int {
	if m.to.line > m.from.line && m.to.col == 0 {
		return m.to.line - 1
	}
	return m.to.line
}

// reach is the most lines a mark spans, so a row looks only at the marks
// that can reach it.
func (e *ed) reach() int {
	e.textRev.Get()
	e.marksRev.Get()
	n := 0
	for i := range e.marks {
		n = max(n, e.marks[i].to.line-e.marks[i].from.line)
	}
	return n
}

// rowMarks are the gutter marks on display row. A row's lines are its
// fold's when folded, so a mark inside a fold marks the fold's row.
func (e *ed) rowMarks(row, reach int) []rowMark {
	s := e.st
	if row >= s.disp.count() {
		return nil
	}
	d := s.disp.row(row, s.r())
	lastOfLine := row+1 >= s.disp.count() || s.disp.row(row+1, s.r()).start == 0
	first := d.start == 0
	endLine := d.line + 1
	if lastOfLine {
		endLine = s.r().lines()
		if row+1 < s.disp.count() {
			endLine = s.disp.row(row+1, s.r()).line
		}
		endLine = max(endLine, d.line+1)
	}
	var out []rowMark
	from := sort.Search(len(e.marks), func(i int) bool { return e.marks[i].from.line >= d.line-reach })
	for i := from; i < len(e.marks) && e.marks[i].from.line < endLine; i++ {
		m := &e.marks[i]
		if m.deco == nil || m.deco.Gutter == nil {
			continue
		}
		place := placeRow
		if m.deco.Gutter.Placement == GutterBefore {
			at := m.from.line
			switch {
			case at < d.line:
				continue
			case at > d.line:
				// A boundary inside a fold: the folded row is marked.
			case m.from.col == 0 && first:
				place = placeTop
			case m.from.col != 0 && lastOfLine:
				place = placeBottom
			default:
				continue
			}
		} else if lastLine(m) < d.line {
			continue
		}
		out = append(out, rowMark{deco: m.deco, place: place})
	}
	return out
}

// rulerMarks are the overview ruler's marks.
func (e *ed) rulerMarks() []rulerMark {
	var out []rulerMark
	s := e.st
	for i := range e.marks {
		m := &e.marks[i]
		if m.deco == nil || m.deco.OverviewRuler == nil {
			continue
		}
		to := m.from.line
		if m.from != m.to {
			to = lastLine(m)
		}
		first, end := s.disp.lineRow(m.from.line), s.disp.count()
		if to+1 < s.r().lines() {
			end = s.disp.lineRow(to + 1)
		}
		out = append(out, rulerMark{deco: m.deco, first: first, end: max(end, first+1)})
	}
	return out
}

// rulerPlace is where a mark of rows rows is down a ruler track tall: its
// top and height, the rows' place in the rows' box, content tall or the
// ruler's height when shorter, scaled to the ruler and kept within it.
func rulerPlace(first, rows int, lineHeight, content, track float32) (top, height float32) {
	content = max(content, track, 1)
	scale := track / content
	height = max(lineHeight*float32(rows)*scale, rulerMarkMin)
	top = min(lineHeight*float32(first)*scale, max(track-height, 0))
	return top, height
}

// rulerLane is the share of the ruler's width before a mark in lane and
// the share it takes.
func rulerLane(lane OverviewRulerLane) (before, share float32) {
	const third = 100.0 / 3
	switch lane {
	case LaneLeft:
		return 0, third
	case LaneRight:
		return 2 * third, third
	case LaneFull:
		return 0, 100
	}
	return third, third
}

// gutterMarks draws row's marks in its strip.
func (e *ed) gutterMarks(cx *core.Scope, row func() int, reach func() int) core.Node {
	marks := core.NewMemoFunc(cx, func() []rowMark {
		e.textRev.Get()
		e.marksRev.Get()
		return e.rowMarks(row(), reach())
	}, slices.Equal[[]rowMark])
	return core.Each(marks.Get, func(m rowMark) rowMark { return m }, func(cx *core.Scope, m func() rowMark, _ func() int) core.Node {
		mark := m()
		width := core.Pt(core.Points(e.stripWidth()))
		place := core.Style{Position: core.Absolute, Left: core.Pt(0), Width: width}
		switch mark.place {
		case placeRow:
			place.Top, place.Bottom = core.Pt(0), core.Pt(0)
		case placeTop:
			place.Top, place.Height = core.Pt(-gutterEdgeBar/2), core.Pt(gutterEdgeBar)
		case placeBottom:
			place.Bottom, place.Height = core.Pt(-gutterEdgeBar/2), core.Pt(gutterEdgeBar)
		}
		return core.View{Class: mark.deco.Gutter.Class, Style: merge(place, mark.deco.Gutter.Style), PointerEvents: core.PointerEventsNone}
	})
}

// overviewRuler lies over the scroll view's right edge, taking no pointer.
// Its marks' rows change only with the text, the folds, the wrapping and
// the decorations; their places also with the viewport.
func (e *ed) overviewRuler() core.Node {
	marks := core.NewMemoFunc(e.cx, func() []rulerMark {
		e.textRev.Get()
		e.marksRev.Get()
		return e.rulerMarks()
	}, func(a, b []rulerMark) bool { return false })
	shown := core.NewMemo(e.cx, func() bool { return len(marks.Get()) > 0 })
	return core.Show{When: shown.Get, Then: func(cx *core.Scope) core.Node {
		return core.View{Class: e.p.Classes.OverviewRuler, PointerEvents: core.PointerEventsNone,
			Style:     core.Style{Position: core.Absolute, Top: core.Pt(0), Bottom: core.Pt(0), Right: core.Pt(0), Width: core.Pt(rulerWidth)},
			StyleFunc: func() core.Style { return e.styles().OverviewRuler },
			Children: []core.Node{core.Each(marks.Get, func(m rulerMark) *Decoration { return m.deco }, func(cx *core.Scope, m func() rulerMark, _ func() int) core.Node {
				deco := m().deco
				return core.View{Class: deco.OverviewRuler.Class, PointerEvents: core.PointerEventsNone,
					StyleFunc: func() core.Style {
						r := m()
						top, height := rulerPlace(r.first, r.end-r.first, e.lh, e.contentHeight(), float32(e.viewport.Get().Height))
						before, share := rulerLane(deco.OverviewRuler.Lane)
						return merge(core.Style{Position: core.Absolute, Left: core.Pct(before), Width: core.Pct(share), Top: core.Pt(core.Points(top)), Height: core.Pt(core.Points(height))},
							deco.OverviewRuler.Style)
					}}
			})},
		}
	}}
}
