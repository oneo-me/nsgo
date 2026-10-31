package editor

import (
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// selection is a range from anchor to head, the caret; goal is the
// display column vertical moves keep (-1 for none).
type selection struct {
	anchor, head pos
	goal         int
}

func caret(p pos) selection { return selection{anchor: p, head: p, goal: -1} }

func (s selection) start() pos    { return minPos(s.anchor, s.head) }
func (s selection) end() pos      { return maxPos(s.anchor, s.head) }
func (s selection) empty() bool   { return s.anchor == s.head }
func (s selection) rev() bool     { return s.head.less(s.anchor) }
func (s selection) at(p pos) bool { return !p.less(s.start()) && !s.end().less(p) }

// state is what an editor holds outside the reactive graph: the text,
// the language, the selections (the last one primary) and the folds.
type state struct {
	buf  *buffer
	lang *language
	tab  int
	hard bool
	// autoClose and autoIndent follow the props.
	autoClose, autoIndent bool
	sels                  []selection
	disp                  *display
	wrap                  int
	// folded is every folded range, nested ones too; candidates the folds
	// that can start, computed when asked after an edit.
	folded     []foldRange
	candidates []foldRange
	candOK     bool
	appFolds   []foldRange
}

func newState(text string) *state {
	s := &state{buf: newBuffer(text), lang: &plainText, tab: 4, autoClose: true, autoIndent: true}
	s.sels = []selection{caret(pos{})}
	s.relayout()
	return s
}

func (s *state) r() *rope { return s.buf.r }

func (s *state) primary() selection { return s.sels[len(s.sels)-1] }

// relayout lays the text out again.
func (s *state) relayout() {
	s.disp = layout(s.r(), outermost(s.folded), s.wrap, s.tab)
}

// foldCandidates is where folds can start.
func (s *state) foldCandidates() []foldRange {
	if s.appFolds != nil {
		return s.appFolds
	}
	if !s.candOK {
		s.candidates, s.candOK = indentFolds(s.r(), s.tab), true
	}
	return s.candidates
}

// setSels sets the selections, sorted and merged, primary last.
func (s *state) setSels(sels []selection) {
	if len(sels) == 0 {
		sels = []selection{caret(pos{})}
	}
	primary := sels[len(sels)-1]
	for i := range sels {
		sels[i].anchor, sels[i].head = s.r().clamp(sels[i].anchor), s.r().clamp(sels[i].head)
	}
	sort.SliceStable(sels, func(i, j int) bool { return sels[i].start().less(sels[j].start()) })
	merged := sels[:0:0]
	for _, x := range sels {
		if n := len(merged); n > 0 {
			last := &merged[n-1]
			if x.start().less(last.end()) || x.start() == last.end() && (x.empty() || last.empty()) {
				start, end := minPos(last.start(), x.start()), maxPos(last.end(), x.end())
				if last.rev() {
					last.anchor, last.head = end, start
				} else {
					last.anchor, last.head = start, end
				}
				if x == primary {
					primary = *last
				}
				continue
			}
		}
		merged = append(merged, x)
	}
	// The primary one last.
	i := slices.IndexFunc(merged, func(x selection) bool { return x.at(primary.head) })
	if i >= 0 {
		p := merged[i]
		merged = append(slices.Delete(merged, i, i+1), p)
	}
	s.sels = merged
	s.unfoldAt()
}

// unfoldAt unfolds every fold hiding a caret.
func (s *state) unfoldAt() {
	changed := false
	for _, sel := range s.sels {
		for i := 0; i < len(s.folded); i++ {
			f := s.folded[i]
			if sel.head.line > f.start && sel.head.line <= f.end {
				s.folded = slices.Delete(s.folded, i, i+1)
				i--
				changed = true
			}
		}
	}
	if changed {
		s.relayout()
	}
}

// line is line i.
func (s *state) line(i int) string { return s.r().line(i) }

// prevChar is the place before p, across line breaks.
func (s *state) prevChar(p pos) pos {
	if p.col == 0 {
		if p.line == 0 {
			return p
		}
		return pos{p.line - 1, len(s.line(p.line - 1))}
	}
	_, n := utf8.DecodeLastRuneInString(s.line(p.line)[:p.col])
	return pos{p.line, p.col - n}
}

// nextChar is the place after p.
func (s *state) nextChar(p pos) pos {
	l := s.line(p.line)
	if p.col >= len(l) {
		if p.line+1 >= s.r().lines() {
			return p
		}
		return pos{p.line + 1, 0}
	}
	_, n := utf8.DecodeRuneInString(l[p.col:])
	return pos{p.line, p.col + n}
}

// charClass is a character's class for word moves: space, word or
// punctuation.
func (s *state) charClass(r rune) int {
	switch {
	case unicode.IsSpace(r):
		return 0
	case s.lang.wordChar(r):
		return 1
	}
	return 2
}

// wordLeft is the start of the word or run of punctuation before p.
func (s *state) wordLeft(p pos) pos {
	if p.col == 0 {
		return s.prevChar(p)
	}
	l := s.line(p.line)
	c := p.col
	for c > 0 {
		r, n := utf8.DecodeLastRuneInString(l[:c])
		if !unicode.IsSpace(r) {
			break
		}
		c -= n
	}
	if c == 0 {
		return pos{p.line, 0}
	}
	r, _ := utf8.DecodeLastRuneInString(l[:c])
	class := s.charClass(r)
	for c > 0 {
		r, n := utf8.DecodeLastRuneInString(l[:c])
		if s.charClass(r) != class {
			break
		}
		c -= n
	}
	return pos{p.line, c}
}

// wordRight is the end of the word or run of punctuation after p.
func (s *state) wordRight(p pos) pos {
	l := s.line(p.line)
	if p.col >= len(l) {
		return s.nextChar(p)
	}
	c := p.col
	for c < len(l) {
		r, n := utf8.DecodeRuneInString(l[c:])
		if !unicode.IsSpace(r) {
			break
		}
		c += n
	}
	if c == len(l) {
		return pos{p.line, c}
	}
	r, _ := utf8.DecodeRuneInString(l[c:])
	class := s.charClass(r)
	for c < len(l) {
		r, n := utf8.DecodeRuneInString(l[c:])
		if s.charClass(r) != class {
			break
		}
		c += n
	}
	return pos{p.line, c}
}

// subwordLeft and subwordRight stop at camelCase humps and snake_case
// parts too.
func (s *state) subwordLeft(p pos) pos {
	w := s.wordLeft(p)
	l := s.line(p.line)
	if w.line != p.line {
		return w
	}
	for c := p.col; c > w.col; {
		r, n := utf8.DecodeLastRuneInString(l[:c])
		c -= n
		if c < p.col-n && (r == '_' || unicode.IsUpper(r) && c > w.col) {
			if r == '_' {
				return pos{p.line, c + n}
			}
			return pos{p.line, c}
		}
	}
	return w
}

func (s *state) subwordRight(p pos) pos {
	w := s.wordRight(p)
	l := s.line(p.line)
	if w.line != p.line {
		return w
	}
	c := p.col
	if c < w.col {
		_, n := utf8.DecodeRuneInString(l[c:])
		c += n
	}
	for c < w.col {
		r, n := utf8.DecodeRuneInString(l[c:])
		if r == '_' || unicode.IsUpper(r) {
			return pos{p.line, c}
		}
		c += n
	}
	return w
}

// lineStart is the line's first non-blank character, or its start when
// the caret is there already: smart Home.
func (s *state) lineStart(p pos) pos {
	l := s.line(p.line)
	first := len(l) - len(strings.TrimLeft(l, " \t"))
	if p.col == first {
		return pos{p.line, 0}
	}
	return pos{p.line, first}
}

func (s *state) lineEnd(p pos) pos { return pos{p.line, len(s.line(p.line))} }

// vertical is p moved by n rows, at display column goal.
func (s *state) vertical(sel selection, n int) selection {
	p := sel.head
	row := s.disp.rowOf(p) + n
	goal := sel.goal
	if goal < 0 {
		goal = colAt(s.line(p.line), p.col, s.tab)
		if d := s.disp.row(s.disp.rowOf(p), s.r()); d.start > 0 {
			goal -= colAt(s.line(p.line), d.start, s.tab)
		}
	}
	switch {
	case row < 0:
		return selection{anchor: sel.anchor, head: pos{}, goal: goal}
	case row >= s.disp.count():
		return selection{anchor: sel.anchor, head: s.r().end(), goal: goal}
	}
	d := s.disp.row(row, s.r())
	l := s.line(d.line)
	base := colAt(l, d.start, s.tab)
	end := d.end
	if row+1 < s.disp.count() && s.disp.row(row+1, s.r()).line == d.line {
		// The end of a wrapped row is the next row's start.
		_, n := utf8.DecodeLastRuneInString(l[:end])
		end -= n
	}
	col := byteAtCol(l, d.start, max(end, d.start), base+goal, s.tab)
	return selection{anchor: sel.anchor, head: pos{d.line, col}, goal: goal}
}

// move moves every selection's head with f, extending or collapsing.
func (s *state) move(extend bool, f func(sel selection) selection) {
	out := make([]selection, len(s.sels))
	for i, sel := range s.sels {
		next := f(sel)
		if !extend {
			next.anchor = next.head
		}
		out[i] = next
	}
	s.setSels(out)
	s.buf.open = false
}

// moveBy moves every head by f.
func (s *state) moveBy(extend bool, f func(p pos) pos) {
	s.move(extend, func(sel selection) selection { return selection{anchor: sel.anchor, head: f(sel.head), goal: -1} })
}

// moveChar moves a character left or right; without extending, a
// selection collapses to its edge instead.
func (s *state) moveChar(extend, back bool) {
	s.move(extend, func(sel selection) selection {
		switch {
		case !extend && !sel.empty() && back:
			return caret(sel.start())
		case !extend && !sel.empty():
			return caret(sel.end())
		case back:
			return selection{anchor: sel.anchor, head: s.prevChar(sel.head), goal: -1}
		}
		return selection{anchor: sel.anchor, head: s.nextChar(sel.head), goal: -1}
	})
}

// plan is the edits of a command at each selection and where each
// selection goes: its head moved with the text, back bytes before it,
// or, with keep, from to to of its inserted text.
type plan struct {
	edits []edit
	back  []int
	keep  [][2]int
	kind  editKind
}

// run applies p's edits at every selection and places the selections.
func (s *state) run(p plan) []change {
	before := slices.Clone(s.sels)
	if len(p.edits) == 0 {
		return nil
	}
	order := make([]int, len(p.edits))
	for i := range order {
		order[i] = i
	}
	changes := s.buf.apply(slices.Clone(p.edits))
	if len(changes) == 0 {
		return nil
	}
	next := make([]selection, len(s.sels))
	for i, sel := range s.sels {
		e := p.edits[min(i, len(p.edits)-1)]
		if i < len(p.keep) && p.keep[i] != [2]int{} {
			at := mapThrough(changes, e.from, true)
			next[i] = selection{anchor: pos{at.line, at.col + p.keep[i][0]}, head: pos{at.line, at.col + p.keep[i][1]}, goal: -1}
			continue
		}
		head := mapThrough(changes, sel.head, false)
		if i < len(p.back) && p.back[i] > 0 {
			head = advance(mapThrough(changes, e.from, true), e.text[:len(e.text)-p.back[i]])
		}
		next[i] = caret(head)
	}
	s.afterEdit(changes)
	s.setSels(next)
	s.buf.record(changes, before, slices.Clone(s.sels), p.kind)
	return changes
}

// advance is where text ends when it starts at p.
func advance(p pos, text string) pos {
	if i := strings.LastIndexByte(text, '\n'); i >= 0 {
		return pos{p.line + strings.Count(text, "\n"), len(text) - i - 1}
	}
	return pos{p.line, p.col + len(text)}
}

// afterEdit carries the folds through changes and lays out again.
func (s *state) afterEdit(changes []change) {
	kept := s.folded[:0]
	for _, f := range s.folded {
		if g, ok := carryFold(f, changes); ok {
			kept = append(kept, g)
		}
	}
	s.folded = kept
	if s.appFolds != nil {
		var app []foldRange
		for _, f := range s.appFolds {
			if g, ok := carryFold(f, changes); ok {
				app = append(app, g)
			}
		}
		s.appFolds = app
	}
	s.candOK = false
	s.relayout()
}

// indentUnit is one level of indentation.
func (s *state) indentUnit() string {
	if s.hard {
		return "\t"
	}
	return strings.Repeat(" ", s.tab)
}

func leading(l string) string { return l[:len(l)-len(strings.TrimLeft(l, " \t"))] }

// insert types text at every selection: brackets and quotes close as they
// are typed, a closing one typed before itself steps over it, and an
// opening one wraps a selection.
func (s *state) insert(text string, typed bool) []change {
	var p plan
	p.kind = kindTyping
	for _, sel := range s.sels {
		e := edit{from: sel.start(), to: sel.end(), text: text}
		back, keep := 0, [2]int{}
		if typed && s.autoClose && utf8.RuneCountInString(text) == 1 {
			l := s.line(sel.head.line)
			next := ""
			if sel.empty() && sel.head.col < len(l) {
				r, _ := utf8.DecodeRuneInString(l[sel.head.col:])
				next = string(r)
			}
			for _, pair := range s.lang.pairs {
				switch {
				case sel.empty() && text == pair[1] && next == pair[1]:
					// Step over the closing one.
					e = edit{from: sel.head, to: pos{sel.head.line, sel.head.col + len(next)}, text: text}
				case text == pair[0] && !sel.empty() && sel.start().line == sel.end().line:
					inner := s.r().slice(sel.start(), sel.end())
					e.text = pair[0] + inner + pair[1]
					keep = [2]int{len(pair[0]), len(pair[0]) + len(inner)}
				case text == pair[0] && sel.empty() && (next == "" || strings.ContainsAny(next, " \t)]}>,;")) &&
					(pair[0] != pair[1] || !s.afterWord(sel.head)):
					e.text = pair[0] + pair[1]
					back = len(pair[1])
				default:
					continue
				}
				break
			}
		}
		p.edits = append(p.edits, e)
		p.back = append(p.back, back)
		p.keep = append(p.keep, keep)
	}
	return s.run(p)
}

// afterWord reports whether p follows a word character: a quote typed
// there is an apostrophe.
func (s *state) afterWord(p pos) bool {
	if p.col == 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s.line(p.line)[:p.col])
	return s.lang.wordChar(r)
}

// newline breaks the line at every selection, keeping its indentation,
// one level more after an opening bracket or the language's openers, and
// between a bracket pair putting the closing one on a line of its own.
func (s *state) newline() []change {
	var p plan
	for _, sel := range s.sels {
		l := s.line(sel.start().line)
		indent := ""
		text := "\n"
		back := 0
		if s.autoIndent {
			indent = leading(l)
			before := strings.TrimRight(l[:sel.start().col], " \t")
			after := strings.TrimLeft(l[min(sel.end().col, len(s.line(sel.end().line))):], " \t")
			opens := false
			for _, b := range s.lang.brackets {
				if strings.HasSuffix(before, b[0]) {
					opens = true
					if strings.HasPrefix(after, b[1]) && sel.end().line == sel.start().line {
						text = "\n" + indent + s.indentUnit() + "\n" + indent
						back = len("\n" + indent)
					}
				}
			}
			for _, o := range s.lang.indentAfter {
				if strings.HasSuffix(before, o) {
					opens = true
				}
			}
			if back == 0 {
				if opens {
					indent += s.indentUnit()
				}
				text = "\n" + indent
			}
		}
		p.edits = append(p.edits, edit{from: sel.start(), to: sel.end(), text: text})
		p.back = append(p.back, back)
	}
	return s.run(p)
}

// deleteBy deletes each selection, or from its caret to where f goes;
// backspace in a bracket pair deletes both, and in leading spaces back to
// the previous tab stop.
func (s *state) deleteBy(f func(p pos) pos, backspace bool) []change {
	var p plan
	p.kind = kindDeleting
	for _, sel := range s.sels {
		from, to := sel.start(), sel.end()
		if sel.empty() {
			to = f(sel.head)
			if to.less(from) {
				from, to = to, from
			}
			if backspace && from.line == to.line {
				l := s.line(from.line)
				if s.autoClose && to.col < len(l) {
					for _, pair := range s.lang.pairs {
						if l[from.col:to.col] == pair[0] && strings.HasPrefix(l[to.col:], pair[1]) {
							to.col += len(pair[1])
						}
					}
				}
				if lead := leading(l); !s.hard && to.col <= len(lead) && from.col < to.col && l[from.col:to.col] == " " {
					stop := (colAt(l, to.col, s.tab) - 1) / s.tab * s.tab
					for from.col > 0 && l[from.col-1] == ' ' && colAt(l, from.col, s.tab) > stop {
						from.col--
					}
				}
			}
		}
		p.edits = append(p.edits, edit{from: from, to: to})
	}
	return s.run(p)
}

// selectedLines is the lines each selection covers, merged.
func (s *state) selectedLines() [][2]int {
	var out [][2]int
	for _, sel := range s.sels {
		a, b := sel.start().line, sel.end().line
		if b > a && sel.end().col == 0 {
			b--
		}
		if n := len(out); n > 0 && a <= out[n-1][1]+1 {
			out[n-1][1] = max(out[n-1][1], b)
			continue
		}
		out = append(out, [2]int{a, b})
	}
	return out
}

// eachLine runs a command over each selected line: f returns the line's
// replacement edit, or false.
func (s *state) eachLine(f func(line int, text string) (edit, bool)) []change {
	var p plan
	for _, r := range s.selectedLines() {
		for line := r[0]; line <= r[1]; line++ {
			if e, ok := f(line, s.line(line)); ok {
				p.edits = append(p.edits, e)
			}
		}
	}
	if len(p.edits) == 0 {
		return nil
	}
	before := slices.Clone(s.sels)
	changes := s.buf.apply(p.edits)
	next := make([]selection, len(s.sels))
	for i, sel := range s.sels {
		next[i] = selection{anchor: mapThrough(changes, sel.anchor, sel.rev()), head: mapThrough(changes, sel.head, !sel.rev()), goal: -1}
		if sel.empty() {
			next[i].anchor = next[i].head
		}
	}
	s.afterEdit(changes)
	s.setSels(next)
	s.buf.record(changes, before, slices.Clone(s.sels), kindOther)
	return changes
}

// indent indents every selected line; a caret alone inserts spaces to the
// next stop (a tab with hard tabs).
func (s *state) indent() []change {
	if len(s.sels) == 1 && s.sels[0].empty() {
		sel := s.sels[0]
		if s.hard {
			return s.insert("\t", false)
		}
		col := colAt(s.line(sel.head.line), sel.head.col, s.tab)
		return s.insert(strings.Repeat(" ", s.tab-col%s.tab), false)
	}
	unit := s.indentUnit()
	return s.eachLine(func(line int, text string) (edit, bool) {
		if strings.TrimSpace(text) == "" {
			return edit{}, false
		}
		return edit{from: pos{line, 0}, to: pos{line, 0}, text: unit}, true
	})
}

// outdent takes a level of indentation off every selected line.
func (s *state) outdent() []change {
	return s.eachLine(func(line int, text string) (edit, bool) {
		n := 0
		for n < len(text) && n < s.tab && text[n] == ' ' {
			n++
		}
		if n == 0 && strings.HasPrefix(text, "\t") {
			n = 1
		}
		return edit{from: pos{line, 0}, to: pos{line, n}}, n > 0
	})
}

// toggleComment comments the selected lines out with the line comment at
// their least indentation, or takes it off when every one has it; a
// language without line comments takes its block comment.
func (s *state) toggleComment() []change {
	token := s.lang.line
	if token == "" {
		return s.toggleBlockComment()
	}
	lines := s.selectedLines()
	all, least := true, -1
	for _, r := range lines {
		for line := r[0]; line <= r[1]; line++ {
			t := s.line(line)
			if strings.TrimSpace(t) == "" {
				continue
			}
			if !strings.HasPrefix(strings.TrimLeft(t, " \t"), token) {
				all = false
			}
			if n := len(leading(t)); least < 0 || n < least {
				least = n
			}
		}
	}
	return s.eachLine(func(line int, text string) (edit, bool) {
		if strings.TrimSpace(text) == "" {
			return edit{}, false
		}
		if all {
			at := len(leading(text))
			end := at + len(token)
			if strings.HasPrefix(text[end:], " ") {
				end++
			}
			return edit{from: pos{line, at}, to: pos{line, end}}, true
		}
		at := min(least, len(leading(text)))
		return edit{from: pos{line, at}, to: pos{line, at}, text: token + " "}, true
	})
}

// toggleBlockComment wraps each selection in the block comment, or takes
// it off when the selection is wrapped.
func (s *state) toggleBlockComment() []change {
	open, close := s.lang.block[0], s.lang.block[1]
	if open == "" {
		return nil
	}
	var p plan
	for _, sel := range s.sels {
		text := s.r().slice(sel.start(), sel.end())
		if strings.HasPrefix(text, open) && strings.HasSuffix(text, close) && len(text) >= len(open)+len(close) {
			inner := strings.TrimSuffix(strings.TrimPrefix(text, open+" "), " "+close)
			if inner == text {
				inner = text[len(open) : len(text)-len(close)]
			}
			p.edits = append(p.edits, edit{from: sel.start(), to: sel.end(), text: inner})
			p.keep = append(p.keep, [2]int{0, len(inner)})
			continue
		}
		p.edits = append(p.edits, edit{from: sel.start(), to: sel.end(), text: open + " " + text + " " + close})
		p.keep = append(p.keep, [2]int{len(open) + 1, len(open) + 1 + len(text)})
	}
	return s.run(p)
}

// moveLines moves the selected lines up (-1) or down (1) past the line
// beside them.
func (s *state) moveLines(dir int) []change {
	lines := s.selectedLines()
	n := s.r().lines()
	if dir < 0 && lines[0][0] == 0 || dir > 0 && lines[len(lines)-1][1] >= n-1 {
		return nil
	}
	var p plan
	for _, r := range lines {
		block := s.r().slice(pos{r[0], 0}, pos{r[1], len(s.line(r[1]))})
		if dir < 0 {
			above := s.line(r[0] - 1)
			p.edits = append(p.edits, edit{from: pos{r[0] - 1, 0}, to: pos{r[1], len(s.line(r[1]))}, text: block + "\n" + above})
		} else {
			below := s.line(r[1] + 1)
			p.edits = append(p.edits, edit{from: pos{r[0], 0}, to: pos{r[1] + 1, len(below)}, text: below + "\n" + block})
		}
	}
	before := slices.Clone(s.sels)
	changes := s.buf.apply(p.edits)
	next := make([]selection, len(s.sels))
	for i, sel := range s.sels {
		next[i] = selection{anchor: pos{sel.anchor.line + dir, sel.anchor.col}, head: pos{sel.head.line + dir, sel.head.col}, goal: -1}
	}
	s.afterEdit(changes)
	s.setSels(next)
	s.buf.record(changes, before, slices.Clone(s.sels), kindOther)
	return changes
}

// duplicateLines copies the selected lines above (up) or below them.
func (s *state) duplicateLines(up bool) []change {
	var p plan
	shift := map[int]int{}
	for _, r := range s.selectedLines() {
		block := s.r().slice(pos{r[0], 0}, pos{r[1], len(s.line(r[1]))})
		p.edits = append(p.edits, edit{from: pos{r[1], len(s.line(r[1]))}, to: pos{r[1], len(s.line(r[1]))}, text: "\n" + block})
		if !up {
			for l := r[0]; l <= r[1]; l++ {
				shift[l] = r[1] - r[0] + 1
			}
		}
	}
	before := slices.Clone(s.sels)
	changes := s.buf.apply(p.edits)
	next := make([]selection, len(s.sels))
	for i, sel := range s.sels {
		a := mapThrough(changes, sel.anchor, true)
		h := mapThrough(changes, sel.head, true)
		d := shift[sel.start().line]
		next[i] = selection{anchor: pos{a.line + d, sel.anchor.col}, head: pos{h.line + d, sel.head.col}, goal: -1}
	}
	s.afterEdit(changes)
	s.setSels(next)
	s.buf.record(changes, before, slices.Clone(s.sels), kindOther)
	return changes
}

// deleteLines deletes the selected lines.
func (s *state) deleteLines() []change {
	var p plan
	n := s.r().lines()
	for _, r := range s.selectedLines() {
		switch {
		case r[1]+1 < n:
			p.edits = append(p.edits, edit{from: pos{r[0], 0}, to: pos{r[1] + 1, 0}})
		case r[0] > 0:
			p.edits = append(p.edits, edit{from: pos{r[0] - 1, len(s.line(r[0] - 1))}, to: pos{r[1], len(s.line(r[1]))}})
		default:
			p.edits = append(p.edits, edit{from: pos{r[0], 0}, to: pos{r[1], len(s.line(r[1]))}})
		}
	}
	p.kind = kindOther
	return s.run(p)
}

// joinLines joins each selection's lines, or a caret's line and the next,
// with a space.
func (s *state) joinLines() []change {
	var p plan
	for _, r := range s.selectedLines() {
		last := max(r[1], r[0]+1)
		if last >= s.r().lines() {
			continue
		}
		for line := r[0]; line < last; line++ {
			l := s.line(line)
			next := s.line(line + 1)
			trimmed := len(next) - len(strings.TrimLeft(next, " \t"))
			cut := len(strings.TrimRight(l, " \t"))
			p.edits = append(p.edits, edit{from: pos{line, cut}, to: pos{line + 1, trimmed}, text: " "})
		}
	}
	return s.run(p)
}

// transpose swaps the characters around each caret.
func (s *state) transpose() []change {
	var p plan
	for _, sel := range s.sels {
		h := sel.head
		l := s.line(h.line)
		if !sel.empty() || len(l) < 2 || h.col == 0 {
			p.edits = append(p.edits, edit{from: h, to: h})
			continue
		}
		if h.col >= len(l) {
			h = s.prevChar(h)
		}
		a := s.prevChar(h)
		b := s.nextChar(h)
		p.edits = append(p.edits, edit{from: a, to: b, text: l[h.col:b.col] + l[a.col:h.col]})
	}
	return s.run(p)
}

// transform replaces each selection (a caret: its word) with f of it.
func (s *state) transform(f func(string) string) []change {
	var p plan
	for _, sel := range s.sels {
		from, to := sel.start(), sel.end()
		if sel.empty() {
			a, b := s.lang.wordAt(s.line(from.line), from.col)
			from, to = pos{from.line, a}, pos{from.line, b}
		}
		text := f(s.r().slice(from, to))
		p.edits = append(p.edits, edit{from: from, to: to, text: text})
		p.keep = append(p.keep, [2]int{0, len(text)})
	}
	return s.run(p)
}

// sortLines sorts the selected lines, or the whole text for a caret.
func (s *state) sortLines(desc bool) []change {
	var p plan
	for _, r := range s.selectedLines() {
		if r[0] == r[1] {
			continue
		}
		lines := make([]string, 0, r[1]-r[0]+1)
		for l := r[0]; l <= r[1]; l++ {
			lines = append(lines, s.line(l))
		}
		sort.Strings(lines)
		if desc {
			slices.Reverse(lines)
		}
		p.edits = append(p.edits, edit{from: pos{r[0], 0}, to: pos{r[1], len(s.line(r[1]))}, text: strings.Join(lines, "\n")})
	}
	return s.run(p)
}

// trimTrailing removes the white space at the end of every line.
func (s *state) trimTrailing() []change {
	var edits []edit
	for line := range s.r().lines() {
		l := s.line(line)
		if t := strings.TrimRight(l, " \t"); len(t) < len(l) {
			edits = append(edits, edit{from: pos{line, len(t)}, to: pos{line, len(l)}})
		}
	}
	if len(edits) == 0 {
		return nil
	}
	before := slices.Clone(s.sels)
	changes := s.buf.apply(edits)
	next := make([]selection, len(s.sels))
	for i, sel := range s.sels {
		next[i] = selection{anchor: mapThrough(changes, sel.anchor, false), head: mapThrough(changes, sel.head, false), goal: -1}
	}
	s.afterEdit(changes)
	s.setSels(next)
	s.buf.record(changes, before, slices.Clone(s.sels), kindOther)
	return changes
}

// replaceText replaces the text with text as one edit that changes only
// what differs: the common prefix and suffix stay, and so do the carets
// and folds away from the change.
func (s *state) replaceText(text string) []change {
	text = normalize(text)
	old := s.r().text()
	if old == text {
		return nil
	}
	pre := 0
	for pre < len(old) && pre < len(text) && old[pre] == text[pre] {
		pre++
	}
	for pre > 0 && !startsRune(old[min(pre, len(old)-1)]) {
		pre--
	}
	suf := 0
	for suf < len(old)-pre && suf < len(text)-pre && old[len(old)-1-suf] == text[len(text)-1-suf] {
		suf++
	}
	for suf > 0 && len(old)-suf < len(old) && !startsRune(old[len(old)-suf]) {
		suf--
	}
	from, to := s.offsetPos(pre), s.offsetPos(len(old)-suf)
	before := slices.Clone(s.sels)
	changes := s.buf.apply([]edit{{from: from, to: to, text: text[pre : len(text)-suf]}})
	next := make([]selection, len(s.sels))
	for i, sel := range s.sels {
		next[i] = selection{anchor: mapThrough(changes, sel.anchor, false), head: mapThrough(changes, sel.head, false), goal: -1}
	}
	s.afterEdit(changes)
	s.setSels(next)
	s.buf.record(changes, before, slices.Clone(s.sels), kindOther)
	s.buf.open = false
	return changes
}

// offsetPos is the place of byte offset o of the text.
func (s *state) offsetPos(o int) pos {
	r := s.r()
	for line := range r.lines() {
		l := len(r.line(line))
		if o <= l {
			return pos{line, o}
		}
		o -= l + 1
	}
	return r.end()
}

// selectedText is the text of every selection, a line break between.
func (s *state) selectedText() string {
	parts := make([]string, len(s.sels))
	for i, sel := range s.sels {
		parts[i] = s.r().slice(sel.start(), sel.end())
	}
	return strings.Join(parts, "\n")
}

// copyText is what copying takes: the selections, or the lines of carets
// whole (lines then pastes them as lines).
func (s *state) copyText() (text string, lines bool) {
	empty := true
	for _, sel := range s.sels {
		if !sel.empty() {
			empty = false
		}
	}
	if !empty {
		return s.selectedText(), false
	}
	var parts []string
	for _, r := range s.selectedLines() {
		parts = append(parts, s.r().slice(pos{r[0], 0}, pos{r[1], len(s.line(r[1]))}))
	}
	return strings.Join(parts, "\n") + "\n", true
}

// paste types text at every selection: a line each when the text has as
// many lines as there are selections; whole lines copied from carets go
// in above the caret's line.
func (s *state) paste(text string, lines bool) []change {
	text = normalize(text)
	parts := strings.Split(text, "\n")
	var p plan
	for i, sel := range s.sels {
		t := text
		if len(s.sels) > 1 && len(parts) == len(s.sels) {
			t = parts[i]
		}
		if lines && sel.empty() {
			at := pos{sel.head.line, 0}
			p.edits = append(p.edits, edit{from: at, to: at, text: t})
			continue
		}
		p.edits = append(p.edits, edit{from: sel.start(), to: sel.end(), text: t})
	}
	return s.run(p)
}

// selectWord selects each caret's word; with every selection a word, the
// next occurrence of the primary one's text too.
func (s *state) selectNext() bool {
	allEmpty := true
	for _, sel := range s.sels {
		if !sel.empty() {
			allEmpty = false
		}
	}
	if allEmpty {
		next := make([]selection, len(s.sels))
		for i, sel := range s.sels {
			a, b := s.lang.wordAt(s.line(sel.head.line), sel.head.col)
			next[i] = selection{anchor: pos{sel.head.line, a}, head: pos{sel.head.line, b}, goal: -1}
		}
		s.setSels(next)
		return true
	}
	p := s.primary()
	needle := s.r().slice(p.start(), p.end())
	if at, ok := s.find(needle, p.end(), true); ok {
		end := s.offsetAfter(at, needle)
		s.setSels(append(slices.Clone(s.sels), selection{anchor: at, head: end, goal: -1}))
		return true
	}
	return false
}

// selectPrevious adds a selection at the occurrence of the primary
// selection's text before the first selection, wrapping; a caret selects
// its word as selectNext's does.
func (s *state) selectPrevious() bool {
	if s.primary().empty() {
		return s.selectNext()
	}
	p := s.primary()
	needle := s.r().slice(p.start(), p.end())
	if needle == "" || strings.Contains(needle, "\n") {
		return false
	}
	first := p.start()
	for _, x := range s.sels {
		first = minPos(first, x.start())
	}
	n := s.r().lines()
	for i := 0; i <= n; i++ {
		line := ((first.line-i)%n + n) % n
		l := s.line(line)
		if i == 0 {
			l = l[:min(first.col, len(l))]
		}
		if j := strings.LastIndex(l, needle); j >= 0 {
			at := pos{line, j}
			if slices.ContainsFunc(s.sels, func(x selection) bool { return x.start() == at }) {
				return false
			}
			s.setSels(append(slices.Clone(s.sels), selection{anchor: at, head: s.offsetAfter(at, needle), goal: -1}))
			return true
		}
	}
	return false
}

// paragraphEdge is the blank line before p's paragraph, or after it when
// down; the text's start or end without one.
func (s *state) paragraphEdge(p pos, down bool) pos {
	n := s.r().lines()
	blank := func(line int) bool { return strings.TrimSpace(s.line(line)) == "" }
	line := p.line
	step := -1
	if down {
		step = 1
	}
	// Past the blank lines the move starts on, then to the next one.
	for line += step; line >= 0 && line < n && blank(line); line += step {
	}
	for ; line >= 0 && line < n && !blank(line); line += step {
	}
	switch {
	case line < 0:
		return pos{}
	case line >= n:
		return s.r().end()
	}
	return pos{line, 0}
}

// titleCase capitalizes the first letter of each word of text and lowers
// the rest.
func titleCase(text string) string {
	out := []rune(strings.ToLower(text))
	start := true
	for i, r := range out {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if start {
				out[i] = unicode.ToTitle(r)
			}
			start = false
		} else {
			start = r != '\''
		}
	}
	return string(out)
}

// selectAllOccurrences selects every occurrence of the primary selection's
// text (a caret: its word).
func (s *state) selectAllOccurrences() {
	p := s.primary()
	if p.empty() {
		a, b := s.lang.wordAt(s.line(p.head.line), p.head.col)
		p = selection{anchor: pos{p.head.line, a}, head: pos{p.head.line, b}}
	}
	needle := s.r().slice(p.start(), p.end())
	if needle == "" || strings.Contains(needle, "\n") {
		return
	}
	var out []selection
	for line := range s.r().lines() {
		l := s.line(line)
		for at := 0; ; {
			i := strings.Index(l[at:], needle)
			if i < 0 {
				break
			}
			out = append(out, selection{anchor: pos{line, at + i}, head: pos{line, at + i + len(needle)}, goal: -1})
			at += i + len(needle)
		}
	}
	if len(out) > 0 {
		s.setSels(out)
	}
}

// find is the next occurrence of needle (a single line) from p, wrapping.
func (s *state) find(needle string, from pos, wrap bool) (pos, bool) {
	if needle == "" || strings.Contains(needle, "\n") {
		return pos{}, false
	}
	n := s.r().lines()
	for i := 0; i <= n; i++ {
		line := (from.line + i) % n
		l := s.line(line)
		start := 0
		if i == 0 {
			start = min(from.col, len(l))
		} else if line == from.line && i == n {
			l = l[:min(from.col, len(l))]
		}
		if j := strings.Index(l[start:], needle); j >= 0 {
			return pos{line, start + j}, true
		}
		if !wrap && line == n-1 {
			break
		}
	}
	return pos{}, false
}

func (s *state) offsetAfter(at pos, text string) pos { return pos{at.line, at.col + len(text)} }

// addCaret adds a caret a row above (-1) or below (1) the first or last
// selection, at its column.
func (s *state) addCaret(dir int) {
	edge := s.sels[0]
	if dir > 0 {
		edge = s.sels[len(s.sels)-1]
		for _, sel := range s.sels {
			if edge.head.less(sel.head) {
				edge = sel
			}
		}
	}
	moved := s.vertical(caret(edge.head), dir)
	if moved.head.line == edge.head.line {
		return
	}
	s.setSels(append(slices.Clone(s.sels), caret(moved.head)))
}

// selectLine selects each selection's whole lines, the next line too when
// it does already.
func (s *state) selectLine() {
	next := make([]selection, len(s.sels))
	for i, sel := range s.sels {
		end := pos{sel.end().line + 1, 0}
		if end.line >= s.r().lines() {
			end = s.r().end()
		}
		next[i] = selection{anchor: pos{sel.start().line, 0}, head: end, goal: -1}
	}
	s.setSels(next)
}

// matchingBracket is the bracket matching the one at or before p.
func (s *state) matchingBracket(p pos) (pos, pos, bool) {
	l := s.line(p.line)
	try := func(at int) (pos, pos, bool) {
		if at < 0 || at >= len(l) {
			return pos{}, pos{}, false
		}
		ch := l[at : at+1]
		for _, b := range s.lang.brackets {
			if len(b[0]) != 1 || b[0] == b[1] {
				continue
			}
			if ch == b[0] {
				if m, ok := s.scanBracket(pos{p.line, at}, b, 1); ok {
					return pos{p.line, at}, m, true
				}
			}
			if ch == b[1] {
				if m, ok := s.scanBracket(pos{p.line, at}, b, -1); ok {
					return pos{p.line, at}, m, true
				}
			}
		}
		return pos{}, pos{}, false
	}
	if a, b, ok := try(p.col); ok {
		return a, b, true
	}
	return try(p.col - 1)
}

// scanBracket finds the partner of the bracket at p, scanning in dir up to
// 5000 lines.
func (s *state) scanBracket(p pos, b [2]string, dir int) (pos, bool) {
	depth := 0
	n := s.r().lines()
	for line, steps := p.line, 0; line >= 0 && line < n && steps < 5000; line, steps = line+dir, steps+1 {
		l := s.line(line)
		start, end := 0, len(l)
		if line == p.line {
			if dir > 0 {
				start = p.col
			} else {
				end = p.col + 1
			}
		}
		for i := range end - start {
			at := start + i
			if dir < 0 {
				at = end - 1 - i
			}
			switch l[at : at+1] {
			case b[0]:
				depth += dir
			case b[1]:
				depth -= dir
			}
			if depth == 0 {
				return pos{line, at}, true
			}
		}
	}
	return pos{}, false
}
