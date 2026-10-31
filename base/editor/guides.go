package editor

import (
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/core"
)

// What the rows' box draws besides the text: rulers, indent guides,
// whitespace marks and the inline completion.

// rulerRects is a line at each of Rulers' columns, the box's height.
func (e *ed) rulerRects() []rect {
	if len(e.p.Rulers) == 0 || e.styles().Ruler.IsZero() && e.p.Classes.Ruler == "" {
		return nil
	}
	e.textRev.Get()
	h := e.contentHeight()
	out := make([]rect, 0, len(e.p.Rulers))
	for _, col := range e.p.Rulers {
		out = append(out, rect{pad + float32(col)*e.charW, 0, 1, h, &e.styles().Ruler, e.p.Classes.Ruler})
	}
	return out
}

// guideRects is a line per indentation level of the rows made, one box
// per run of rows at a level: a blank line takes the lesser indentation
// of the lines around it.
func (e *ed) guideRects() []rect {
	if e.p.NoIndentGuides || e.styles().IndentGuide.IsZero() && e.p.Classes.IndentGuide == "" {
		return nil
	}
	e.textRev.Get()
	s, end := e.windowRows()
	r := e.st.r()
	tab := max(e.st.tab, 1)
	var out []rect
	// open is the rect of each level still running, by level.
	var open []int
	for row := s; row < end; row++ {
		d := e.st.disp.row(row, r)
		levels := e.indentLevel(d.line) / tab
		y := float32(row) * e.lh
		for k := range levels {
			if k < len(open) && open[k] >= 0 && out[open[k]].y+out[open[k]].h == y {
				out[open[k]].h += e.lh
				continue
			}
			for len(open) <= k {
				open = append(open, -1)
			}
			open[k] = len(out)
			out = append(out, rect{pad + float32(k*tab)*e.charW, y, 1, e.lh, &e.styles().IndentGuide, e.p.Classes.IndentGuide})
		}
		for k := levels; k < len(open); k++ {
			open[k] = -1
		}
	}
	return out
}

// indentLevel is line's indentation in columns; a blank line's is the
// lesser of the lines around it, up to 100 lines away.
func (e *ed) indentLevel(line int) int {
	r := e.st.r()
	if n := indentOf(r.line(line), e.st.tab); n >= 0 {
		return n
	}
	near := func(step int) int {
		for i, l := 0, line+step; i < 100 && l >= 0 && l < r.lines(); i, l = i+1, l+step {
			if n := indentOf(r.line(l), e.st.tab); n >= 0 {
				return n
			}
		}
		return 0
	}
	return min(near(-1), near(1))
}

// glyph is a mark drawn over the rows: its place and text.
type glyph struct {
	x, y float32
	text string
}

// glyphs are the whitespace marks of the rows made: a · per space, a run
// at a time, and a → per tab, placed by the columns the row's text takes.
func (e *ed) glyphs() core.Node {
	if !e.p.ShowWhitespaces {
		return core.Group{}
	}
	list := core.NewMemoFunc(e.cx, func() []glyph {
		e.textRev.Get()
		s, end := e.windowRows()
		r := e.st.r()
		var out []glyph
		for row := s; row < end; row++ {
			d := e.st.disp.row(row, r)
			l := r.line(d.line)
			y := float32(row) * e.lh
			base := colAt(l, d.start, e.st.tab)
			col := base
			run, runCol := 0, 0
			flush := func() {
				if run > 0 {
					out = append(out, glyph{pad + float32(runCol-base)*e.charW, y, strings.Repeat("·", run)})
					run = 0
				}
			}
			for _, c := range l[d.start:min(d.end, len(l))] {
				switch c {
				case ' ':
					if run == 0 {
						runCol = col
					}
					run++
				case '\t':
					flush()
					out = append(out, glyph{pad + float32(col-base)*e.charW, y, "→"})
				default:
					flush()
				}
				col += runeCols(c, col, e.st.tab)
			}
			flush()
		}
		return out
	}, slices.Equal)
	return core.Each(func() []int {
		out := make([]int, len(list.Get()))
		for i := range out {
			out[i] = i
		}
		return out
	}, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
		g := func() glyph {
			all := list.Get()
			if i() >= len(all) {
				return glyph{}
			}
			return all[i()]
		}
		return core.Text{
			MaxLines:    1,
			Class:       e.p.Classes.Whitespace,
			ContentFunc: func() string { return g().text },
			StyleFunc: func() core.Style {
				x := g()
				return merge(core.Style{Position: core.Absolute, Left: core.Pt(core.Points(x.x)), Top: core.Pt(core.Points(x.y)),
					LineHeight: core.Points(e.lh), WhiteSpace: core.WhiteSpaceNoWrap}, e.styles().Whitespace)
			},
		}
	})
}

// inlineView is the inline completion at the primary caret: its first
// line from the caret over the rest of the row, its others on the rows
// below.
func (e *ed) inlineView() core.Node {
	return core.Show{When: func() bool { return e.inline.Get() != "" }, Then: func(cx *core.Scope) core.Node {
		return core.Dynamic(func(cx *core.Scope) core.Node {
			text := e.inline.Get()
			e.selRev.Get()
			e.textRev.Get()
			h := e.st.primary().head
			row := e.st.disp.rowOf(h)
			var lines []core.Node
			for i, l := range strings.Split(text, "\n") {
				x := float32(pad)
				if i == 0 {
					x = e.xOf(h)
				}
				lines = append(lines, core.Text{Content: strings.ReplaceAll(l, "\t", strings.Repeat(" ", e.st.tab)), MaxLines: 1, Class: e.p.Classes.InlineCompletion,
					Style: core.Style{Position: core.Absolute, Left: core.Pt(core.Points(x)), Top: core.Pt(core.Points(float32(row+i) * e.lh)),
						LineHeight: core.Points(e.lh), WhiteSpace: core.WhiteSpaceNoWrap},
					StyleFunc: func() core.Style { return e.styles().InlineCompletion }})
			}
			return core.Group(lines)
		})
	}}
}

// PresentInlineCompletion answers inline completion request id with text,
// shown at the caret until Tab inserts it, or Escape, an edit or a move
// clears it.
func (r *Ref) PresentInlineCompletion(id int, text string) {
	e := r.e
	if e == nil || e.inlineID != id || e.comp.shown && e.comp.id != 0 {
		return
	}
	e.inline.Set(normalize(text))
}

// HideInlineCompletion clears the inline completion.
func (r *Ref) HideInlineCompletion() {
	if r.e != nil {
		r.e.hideInline()
	}
}
