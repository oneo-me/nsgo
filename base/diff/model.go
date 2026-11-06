package diff

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// splitLines splits text after each line break, which stays part of its
// line: "a\nb\n" and "a\nb" both have two lines, and their last ones
// differ, so a missing final line break is a change, as git shows it.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	var out []string
	for len(text) > 0 {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			out = append(out, text)
			break
		}
		out = append(out, text[:i+1])
		text = text[i+1:]
	}
	return out
}

// op is an edit script's step: lines equal, deleted from a or added from
// b.
type op uint8

const (
	opEqual op = iota
	opDelete
	opInsert
)

// myers is the shortest edit script from a to b, Myers' O(ND) algorithm
// over the middle the common prefix and suffix leave; past maxD steps the
// rest is one change.
func myers[T comparable](a, b []T, maxD int) []op {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	out := make([]op, 0, len(a)+len(b))
	for range pre {
		out = append(out, opEqual)
	}
	out = append(out, middle(a[pre:len(a)-suf], b[pre:len(b)-suf], maxD)...)
	for range suf {
		out = append(out, opEqual)
	}
	return out
}

func middle[T comparable](a, b []T, maxD int) []op {
	n, m := len(a), len(b)
	if n == 0 || m == 0 {
		out := make([]op, 0, n+m)
		for range n {
			out = append(out, opDelete)
		}
		for range m {
			out = append(out, opInsert)
		}
		return out
	}
	off := n + m
	v := make([]int, 2*off+2)
	var trace [][]int
	found := false
	for d := 0; d <= n+m && d <= maxD; d++ {
		trace = append(trace, append([]int(nil), v...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || k != d && v[off+k-1] < v[off+k+1] {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1
			}
			v[off+k] = x
			if x >= n && y >= m {
				found = true
				break
			}
		}
		if found {
			trace = append(trace, v)
			break
		}
	}
	if !found {
		out := make([]op, 0, n+m)
		for range n {
			out = append(out, opDelete)
		}
		for range m {
			out = append(out, opInsert)
		}
		return out
	}
	// Back through the trace.
	var rev []op
	x, y := n, m
	for d := len(trace) - 2; d >= 0; d-- {
		vd := trace[d]
		k := x - y
		var prevK int
		if k == -d || k != d && vd[off+k-1] < vd[off+k+1] {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := vd[off+prevK]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			rev = append(rev, opEqual)
			x, y = x-1, y-1
		}
		if d > 0 {
			if x == prevX {
				rev = append(rev, opInsert)
			} else {
				rev = append(rev, opDelete)
			}
		}
		x, y = prevX, prevY
	}
	for x > 0 && y > 0 {
		rev = append(rev, opEqual)
		x, y = x-1, y-1
	}
	out := make([]op, len(rev))
	for i, o := range rev {
		out[len(rev)-1-i] = o
	}
	return out
}

// hunk is a change: old lines oldStart to oldEnd replaced by new lines
// newStart to newEnd, and which of them pair as one line changed.
type hunk struct {
	oldStart, oldEnd, newStart, newEnd int
	// pairs are old and new lines shown side by side, in order; lines
	// in no pair stand alone.
	pairs [][2]int
}

// model is the diff of two texts.
type model struct {
	old, new []string
	hunks    []hunk
}

// maxSteps bounds the line diff's work.
const maxSteps = 20_000

func newModel(oldText, newText string) *model {
	m := &model{old: splitLines(oldText), new: splitLines(newText)}
	ops := myers(m.old, m.new, maxSteps)
	i, j := 0, 0
	for k := 0; k < len(ops); {
		if ops[k] == opEqual {
			i, j, k = i+1, j+1, k+1
			continue
		}
		h := hunk{oldStart: i, newStart: j}
		for k < len(ops) && ops[k] != opEqual {
			if ops[k] == opDelete {
				i++
			} else {
				j++
			}
			k++
		}
		h.oldEnd, h.newEnd = i, j
		h.pairs = pair(m.old[h.oldStart:h.oldEnd], m.new[h.newStart:h.newEnd], h.oldStart, h.newStart)
		m.hunks = append(m.hunks, h)
	}
	return m
}

// pair pairs each deleted line with the added line most like it, in
// order: a weighted longest common subsequence over a similarity from 0 to
// 1, pairs less alike than 0.4 left out, then the lines between two pairs
// paired in order as far as both sides have lines. Past 64 lines a side,
// in order.
func pair(old, new []string, oldAt, newAt int) [][2]int {
	var kept [][2]int
	if len(old) <= 64 && len(new) <= 64 {
		n, m := len(old), len(new)
		score := make([][]float64, n+1)
		for i := range score {
			score[i] = make([]float64, m+1)
		}
		sim := func(i, j int) float64 { return similarity(old[i], new[j]) }
		for i := n - 1; i >= 0; i-- {
			for j := m - 1; j >= 0; j-- {
				best := max(score[i+1][j], score[i][j+1])
				if s := sim(i, j); s >= 0.4 {
					best = max(best, score[i+1][j+1]+s)
				}
				score[i][j] = best
			}
		}
		for i, j := 0, 0; i < n && j < m; {
			s := sim(i, j)
			switch {
			case s >= 0.4 && score[i][j] == score[i+1][j+1]+s:
				kept = append(kept, [2]int{i, j})
				i, j = i+1, j+1
			case score[i][j] == score[i+1][j]:
				i++
			default:
				j++
			}
		}
	}
	// In order between the pairs found.
	var out [][2]int
	pi, pj := 0, 0
	fill := func(i, j int) {
		for pi < i && pj < j {
			out = append(out, [2]int{oldAt + pi, newAt + pj})
			pi, pj = pi+1, pj+1
		}
	}
	for _, k := range kept {
		fill(k[0], k[1])
		out = append(out, [2]int{oldAt + k[0], newAt + k[1]})
		pi, pj = k[0]+1, k[1]+1
	}
	fill(len(old), len(new))
	return out
}

// similarity is how alike two lines are: the better of their common
// prefix and suffix over the longer, indentation aside, and the Dice
// coefficient of their words.
func similarity(a, b string) float64 {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == b {
		return 1
	}
	if a == "" || b == "" {
		return 0
	}
	longer := float64(max(len(a), len(b)))
	pre, suf := 0, 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	edge := float64(pre+suf) / longer
	wa, wb := words(a), words(b)
	count := map[string]int{}
	for _, w := range wa {
		count[w]++
	}
	common := 0
	for _, w := range wb {
		if count[w] > 0 {
			count[w]--
			common++
		}
	}
	dice := 0.0
	if len(wa)+len(wb) > 0 {
		dice = 2 * float64(common) / float64(len(wa)+len(wb))
	}
	return max(edge, dice)
}

func words(s string) []string {
	var out []string
	for _, t := range tokens(s, false) {
		if strings.TrimSpace(t) != "" {
			out = append(out, t)
		}
	}
	return out
}

// tokens splits s into words (letter, digit and _ runs; a CJK character
// alone), white space runs and other characters alone; with chars, into
// characters.
func tokens(s string, chars bool) []string {
	var out []string
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		j := i + n
		if !chars {
			switch {
			case isWord(r) && !cjk(r):
				for j < len(s) {
					r2, n2 := utf8.DecodeRuneInString(s[j:])
					if !isWord(r2) || cjk(r2) {
						break
					}
					j += n2
				}
			case unicode.IsSpace(r):
				for j < len(s) {
					r2, n2 := utf8.DecodeRuneInString(s[j:])
					if !unicode.IsSpace(r2) {
						break
					}
					j += n2
				}
			}
		}
		out = append(out, s[i:j])
		i = j
	}
	return out
}

func isWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func cjk(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

// Inline is what is highlighted within changed lines.
type Inline uint8

const (
	// InlineWordAlt marks the words that changed, joined when one
	// character apart, so a changed phrase reads as one.
	InlineWordAlt Inline = iota
	// InlineWord marks each word that changed.
	InlineWord
	// InlineChar marks each character that changed.
	InlineChar
	// InlineNone marks nothing: the lines' tints alone.
	InlineNone
)

// inline is the byte ranges that changed in a pair's old and new lines;
// none when more than 70% of either changed or a line is over 10 000
// bytes.
func inline(a, b string, mode Inline) (old, new [][2]int) {
	a, b = strings.TrimSuffix(a, "\n"), strings.TrimSuffix(b, "\n")
	if mode == InlineNone || len(a) > 10_000 || len(b) > 10_000 || a == b {
		return nil, nil
	}
	ta, tb := tokens(a, mode == InlineChar), tokens(b, mode == InlineChar)
	ops := myers(ta, tb, 2000)
	i, j, pa, pb := 0, 0, 0, 0
	for _, o := range ops {
		switch o {
		case opEqual:
			pa, pb = pa+len(ta[i]), pb+len(tb[j])
			i, j = i+1, j+1
		case opDelete:
			old = appendRange(old, pa, pa+len(ta[i]), mode == InlineWordAlt)
			pa += len(ta[i])
			i++
		case opInsert:
			new = appendRange(new, pb, pb+len(tb[j]), mode == InlineWordAlt)
			pb += len(tb[j])
			j++
		}
	}
	if changed(old) > len(a)*7/10 || changed(new) > len(b)*7/10 {
		return nil, nil
	}
	return old, new
}

// appendRange adds a range, joined to the last when they touch, or are a
// character apart with join.
func appendRange(rs [][2]int, a, b int, join bool) [][2]int {
	if n := len(rs); n > 0 && (rs[n-1][1] == a || join && a-rs[n-1][1] <= 1) {
		rs[n-1][1] = b
		return rs
	}
	return append(rs, [2]int{a, b})
}

func changed(rs [][2]int) int {
	n := 0
	for _, r := range rs {
		n += r[1] - r[0]
	}
	return n
}

// revert puts hunk h's old lines back into the new text.
func (m *model) revert(h hunk) (string, string) {
	lines := append(append(append([]string(nil), m.new[:h.newStart]...), m.old[h.oldStart:h.oldEnd]...), m.new[h.newEnd:]...)
	return strings.Join(m.old, ""), strings.Join(lines, "")
}

// walk calls fn with the hunk's lines in the order rows show them side by
// side: a line alone, its other side -1, or a pair.
func (h hunk) walk(fn func(old, new int)) {
	oi, ni := h.oldStart, h.newStart
	for _, pr := range append(h.pairs, [2]int{h.oldEnd, h.newEnd}) {
		for ; oi < pr[0]; oi++ {
			fn(oi, -1)
		}
		for ; ni < pr[1]; ni++ {
			fn(-1, ni)
		}
		if pr[0] < h.oldEnd && pr[1] < h.newEnd {
			fn(pr[0], pr[1])
			oi, ni = pr[0]+1, pr[1]+1
		}
	}
}

// revertLines puts the old and new lines given (sorted, from 0) back as
// the old text has them, the changes' other lines staying changes.
func (m *model) revertLines(old, new []int) (string, string) {
	var lines []string
	at := 0
	for _, h := range m.hunks {
		lines = append(lines, m.new[at:h.newStart]...)
		h.walk(func(o, n int) {
			if o >= 0 && has(old, o) {
				lines = append(lines, m.old[o])
			}
			if n >= 0 && !has(new, n) {
				lines = append(lines, m.new[n])
			}
		})
		at = h.newEnd
	}
	return strings.Join(m.old, ""), strings.Join(append(lines, m.new[at:]...), "")
}

// acceptLines makes the old and new lines given (sorted, from 0) the old
// text's, the changes' other lines staying changes.
func (m *model) acceptLines(old, new []int) (string, string) {
	var lines []string
	at := 0
	for _, h := range m.hunks {
		lines = append(lines, m.old[at:h.oldStart]...)
		h.walk(func(o, n int) {
			if o >= 0 && !has(old, o) {
				lines = append(lines, m.old[o])
			}
			if n >= 0 && has(new, n) {
				lines = append(lines, m.new[n])
			}
		})
		at = h.oldEnd
	}
	return strings.Join(append(lines, m.old[at:]...), ""), strings.Join(m.new, "")
}

// has reports whether sorted lines holds line.
func has(lines []int, line int) bool {
	_, ok := slices.BinarySearch(lines, line)
	return ok
}

// accept makes hunk h's new lines the old text's.
func (m *model) accept(h hunk) (string, string) {
	lines := append(append(append([]string(nil), m.old[:h.oldStart]...), m.new[h.newStart:h.newEnd]...), m.old[h.oldEnd:]...)
	return strings.Join(lines, ""), strings.Join(m.new, "")
}
