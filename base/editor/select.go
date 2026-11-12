package editor

import (
	"slices"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
)

// maxSelectionHistory is the most selections undoSelection goes back
// through.
const maxSelectionHistory = 100

// span is a range of the text, from to to.
func (s selection) span() span { return span{s.start(), s.end()} }

// contains reports whether a holds b.
func (a span) contains(b span) bool { return !b.from.less(a.from) && !a.to.less(b.to) }

// expand grows each selection to the smallest unit around it that is
// larger, without a syntax tree: the word, a string's contents, the
// string, a bracket pair's contents, the pair, the line's text, the whole
// lines, the text.
func (s *state) expand() {
	next := make([]selection, len(s.sels))
	for i, sel := range s.sels {
		cur := sel.span()
		best, found := cur, false
		for _, c := range s.units(cur) {
			if c != cur && c.contains(cur) && (!found || best.contains(c)) {
				best, found = c, true
			}
		}
		next[i] = selection{anchor: best.from, head: best.to, goal: -1}
	}
	s.setSels(next)
}

// units is what expand may grow cur to.
func (s *state) units(cur span) []span {
	var out []span
	l := s.line(cur.from.line)
	if cur.from.line == cur.to.line {
		if a, b := s.lang.wordAt(l, cur.from.col); a < b && b >= cur.to.col {
			out = append(out, span{pos{cur.from.line, a}, pos{cur.from.line, b}})
		}
		// Strings: the language's quotes, paired along the line.
		for _, p := range s.lang.pairs {
			if p[0] != p[1] {
				continue
			}
			for at, open := 0, -1; at < len(l); at++ {
				if !strings.HasPrefix(l[at:], p[0]) || at > 0 && l[at-1] == '\\' {
					continue
				}
				if open < 0 {
					open = at
					continue
				}
				if open < cur.from.col && cur.to.col <= at {
					out = append(out, span{pos{cur.from.line, open + 1}, pos{cur.from.line, at}},
						span{pos{cur.from.line, open}, pos{cur.from.line, at + len(p[1])}})
				}
				open = -1
			}
		}
		lead := len(leading(l))
		out = append(out, span{pos{cur.from.line, lead}, pos{cur.from.line, len(strings.TrimRight(l, " \t"))}})
	}
	// Bracket pairs, from the innermost out.
	from, to := cur.from, cur.to
	for range 100 {
		open, close, ok := s.enclosing(from, to)
		if !ok {
			break
		}
		inner := span{pos{open.line, open.col + 1}, close}
		out = append(out, inner, span{open, pos{close.line, close.col + 1}})
		from, to = open, pos{close.line, close.col + 1}
	}
	end := pos{cur.to.line + 1, 0}
	if end.line >= s.r().lines() {
		end = s.r().end()
	}
	out = append(out, span{pos{cur.from.line, 0}, end}, span{pos{}, s.r().end()})
	return out
}

// enclosing is the innermost pair of brackets around from to to: the
// opening bracket before from and its partner at or after to.
func (s *state) enclosing(from, to pos) (pos, pos, bool) {
	depth := map[string]int{}
	p := from
	for steps := 0; steps < 100_000; steps++ {
		if p == (pos{}) {
			return pos{}, pos{}, false
		}
		p = s.prevChar(p)
		l := s.line(p.line)
		if p.col >= len(l) {
			continue
		}
		ch := l[p.col : p.col+1]
		for _, b := range s.lang.brackets {
			if len(b[0]) != 1 || b[0] == b[1] {
				continue
			}
			switch ch {
			case b[1]:
				depth[b[0]]++
			case b[0]:
				if depth[b[0]] > 0 {
					depth[b[0]]--
					continue
				}
				if m, ok := s.scanBracket(p, b, 1); ok && !m.less(to) {
					return p, m, true
				}
			}
		}
	}
	return pos{}, pos{}, false
}

// blockSels is a block drag's selections: one per display row from the
// press's row to pt's, each from the press's x to pt's (what a short row
// has of them), pt's row primary.
func (e *ed) blockSels(pt core.Point) []selection {
	n := e.st.disp.count()
	rowOf := func(y core.Points) int { return min(max(int(float32(y)/e.lh), 0), n-1) }
	from, to := rowOf(e.blockAt.Y), rowOf(pt.Y)
	step := 1
	if to < from {
		step = -1
	}
	var out []selection
	for row := from; ; row += step {
		y := core.Points((float32(row) + 0.5) * e.lh)
		a, h := e.posAt(core.Point{X: e.blockAt.X, Y: y}), e.posAt(core.Point{X: pt.X, Y: y})
		out = append(out, selection{anchor: a, head: h, goal: -1})
		if row == to {
			break
		}
	}
	return out
}

// splitLines turns each selection over several lines into one per line.
func (s *state) splitLines() {
	var out []selection
	for _, sel := range s.sels {
		a, b := sel.start(), sel.end()
		if a.line == b.line {
			out = append(out, sel)
			continue
		}
		for line := a.line; line <= b.line; line++ {
			from, to := pos{line, 0}, pos{line, len(s.line(line))}
			if line == a.line {
				from = a
			}
			if line == b.line {
				to = b
			}
			if line == b.line && b.col == 0 {
				continue
			}
			out = append(out, selection{anchor: from, head: to, goal: -1})
		}
	}
	s.setSels(out)
}

// recordSelections keeps the selections a move left, for undoSelection.
func (e *ed) recordSelections() {
	if e.dragging {
		return // a drag records where it ends
	}
	sels := e.st.sels
	if e.selAt < len(e.selHist) && slices.Equal(e.selHist[e.selAt], sels) {
		return
	}
	e.selHist = append(e.selHist[:min(e.selAt+1, len(e.selHist))], slices.Clone(sels))
	if len(e.selHist) > maxSelectionHistory {
		e.selHist = e.selHist[1:]
	}
	e.selAt = len(e.selHist) - 1
}

// stepSelections goes back (-1) or forward (1) through the history.
func (e *ed) stepSelections(d int) {
	at := e.selAt + d
	if at < 0 || at >= len(e.selHist) {
		return
	}
	e.selAt = at
	e.st.setSels(slices.Clone(e.selHist[at]))
	e.restoring = true
	e.moved()
	e.restoring = false
}

// expandSelections grows the selections, remembering what they were for
// shrinkSelections.
func (e *ed) expandSelections() {
	before := slices.Clone(e.st.sels)
	if len(e.grown) == 0 || !slices.Equal(e.grown[len(e.grown)-1].after, before) {
		e.grown = nil
	}
	e.st.expand()
	if !slices.Equal(e.st.sels, before) {
		e.grown = append(e.grown, grownSels{before, slices.Clone(e.st.sels)})
	}
	e.keepGrown = true
	e.moved()
	e.keepGrown = false
}

// shrinkSelections retraces expandSelections while nothing else moved the
// selections.
func (e *ed) shrinkSelections() {
	n := len(e.grown)
	if n == 0 || !slices.Equal(e.grown[n-1].after, e.st.sels) {
		e.grown = nil
		return
	}
	e.st.setSels(slices.Clone(e.grown[n-1].before))
	e.grown = e.grown[:n-1]
	e.keepGrown = true
	e.moved()
	e.keepGrown = false
}

// grownSels is a step of expandSelections.
type grownSels struct{ before, after []selection }
