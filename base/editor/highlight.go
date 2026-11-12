package editor

import (
	"slices"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/nsgo-dev/nsgo/base/internal/syntax"
)

const (
	// jobLines is the most lines a job lexes.
	jobLines = 2000
	// lookBack is the farthest a job starts before the line it is for,
	// looking for a line that starts at column 0.
	lookBack = 200
	// converged is how many lines past the dirty ones must come out as
	// before for a job to stop early.
	converged = 3
	// ahead is how far past the rows in view jobs go before they rest.
	ahead = 1000
)

type tok = syntax.Tok

// highlighter colors the text with chroma, off the engine goroutine.
// Tokens stay on the engine goroutine by line: a job lexes a snapshot of
// the lines from a safe start (a line at column 0 before the first line
// needing tokens, the lexer's root state likely there) and stops where
// its tokens meet the ones the lines had, so typing in a function lexes
// a few lines. An edit shifts the tokens it moves with the text and marks
// the lines it touched until a job gives them theirs again, so colors do
// not flicker.
type highlighter struct {
	lexer chroma.Lexer
	// app is the app's highlighter, in place of the lexer.
	app    syntax.Highlighter
	tokens [][]tok
	fresh  []bool
	// running is a job out; version the text's version it lexes.
	running bool
	version int
}

func newHighlighter(lang *language, lines int) *highlighter {
	if lang.lexer == "" {
		return nil
	}
	l := lexers.Get(lang.lexer)
	if l == nil {
		return nil
	}
	return &highlighter{lexer: chroma.Coalesce(l), tokens: make([][]tok, lines), fresh: make([]bool, lines)}
}

// edited moves tokens with changes: the lines a change replaced become
// the new ones, the first keeping its tokens before the change, and
// marked for a job.
func (h *highlighter) edited(changes []change) {
	for _, c := range changes {
		old := c.oldEnd.line - c.from.line + 1
		added := c.newEnd.line - c.from.line + 1
		first := h.tokens[c.from.line]
		var kept []tok
		for _, t := range first {
			if t.End <= c.from.col {
				kept = append(kept, t)
			}
		}
		lines := make([][]tok, added)
		lines[0] = kept
		if added == 1 && old == 1 {
			// Text typed in a token takes its kind: the rest of the line
			// moves with the edit.
			shift := c.newEnd.col - c.oldEnd.col
			for _, t := range first {
				switch {
				case t.End <= c.from.col:
				case t.Start >= c.oldEnd.col:
					lines[0] = append(lines[0], tok{Start: t.Start + shift, End: t.End + shift, Kind: t.Kind})
				case t.Start <= c.from.col && c.oldEnd.col <= t.End:
					lines[0] = append(lines[0][:len(lines[0]):len(lines[0])], tok{Start: t.Start, End: t.End + shift, Kind: t.Kind})
				}
			}
		}
		h.tokens = slices.Replace(h.tokens, c.from.line, c.oldEnd.line+1, lines...)
		h.fresh = slices.Replace(h.fresh, c.from.line, c.oldEnd.line+1, make([]bool, added)...)
	}
}

// next is the job to run for rows in view from to to: the first line
// needing tokens in view or within ahead of it, else above it, from a
// safe start; ok is false when none needs one.
func (h *highlighter) next(r *rope, from, to int) (start, end int, ok bool) {
	n := len(h.fresh)
	target := -1
	for i := max(from-40, 0); i < min(to+ahead, n); i++ {
		if !h.fresh[i] {
			target = i
			break
		}
	}
	if target < 0 {
		for i := range min(from, n) {
			if !h.fresh[i] {
				target = i
				break
			}
		}
	}
	if target < 0 {
		return 0, 0, false
	}
	start = target
	for back := 0; start > 0 && back < lookBack; back++ {
		l := r.line(start)
		if l != "" && l[0] != ' ' && l[0] != '\t' && h.fresh[max(start-1, 0)] {
			break
		}
		start--
	}
	return start, min(start+jobLines, n), true
}

// take keeps a job's tokens for lines from start, until they meet the
// lines' own again past the lines needing them.
func (h *highlighter) take(start int, lines [][]tok) {
	same, passed := 0, false
	for i, ts := range lines {
		at := start + i
		if at >= len(h.tokens) {
			break
		}
		if h.fresh[at] && slices.Equal(h.tokens[at], ts) {
			same++
			if passed && same >= converged {
				return
			}
			continue
		}
		passed = passed || !h.fresh[at]
		same = 0
		h.tokens[at], h.fresh[at] = ts, true
	}
}

// line is the tokens of line i.
func (h *highlighter) line(i int) []tok {
	if h == nil || i >= len(h.tokens) {
		return nil
	}
	return h.tokens[i]
}
