package diff

import (
	"sort"
	"strings"
	"unicode"
)

// Rows are not of one height: a row is its lines (one, or with SoftWrap
// the wrapped lines of its line, in a split row the taller side's) times
// the line height, or a separator's height, plus the annotations under
// it. tops keeps every row's
// top, a prefix sum, so the rows at a height are a binary search.

// layoutRows computes every row's top from the wrapping and the
// annotations' heights.
func (v *dv) layoutRows() {
	n := len(v.rows)
	if cap(v.tops) < n+1 {
		v.tops = make([]float32, n+1)
	}
	v.tops = v.tops[:n+1]
	under := v.annotationsByRow()
	y := float32(0)
	for i, r := range v.rows {
		v.tops[i] = y
		h := v.rowHeight(r)
		if a, ok := under[i]; ok {
			h += a
		}
		y += h
	}
	v.tops[n] = y
}

// height is the rows' height.
func (v *dv) height() float32 {
	if len(v.tops) == 0 {
		return 0
	}
	return v.tops[len(v.tops)-1]
}

// rowAt is the row at height y, clamped to the rows.
func (v *dv) rowAt(y float32) int {
	n := len(v.rows)
	if n == 0 {
		return 0
	}
	i := sort.Search(n, func(i int) bool { return v.tops[i+1] > y })
	return min(i, n-1)
}

// rowHeight is row r's height, without its annotations.
func (v *dv) rowHeight(r row) float32 {
	if r.kind == rowSeparator || r.kind == rowFile {
		return v.sepH
	}
	return float32(v.lineCount(r)) * v.lh
}

// lineCount is how many lines row r takes: its wrapped lines with
// SoftWrap, else one.
func (v *dv) lineCount(r row) int {
	if !v.p.SoftWrap || v.wrapCols <= 0 || r.kind == rowSeparator || r.kind == rowFile {
		return 1
	}
	n := 1
	if r.old >= 0 && (!v.p.Unified || r.kind == rowDeleted) {
		n = max(n, len(v.wrapped(true, r.old)))
	}
	if r.new >= 0 && (!v.p.Unified || r.kind != rowDeleted) {
		n = max(n, len(v.wrapped(false, r.new)))
	}
	return n
}

// wrapped is where line's wrapped lines start, in bytes.
func (v *dv) wrapped(isOld bool, line int) []int {
	return wrapStarts(strings.TrimSuffix(v.lineText(isOld, line), "\n"), v.wrapCols)
}

// wrapStarts is where text's wrapped lines start at cols columns a line
// (tabs four): each broken after its last white space that fits, else at
// the edge, white space hanging past it.
func wrapStarts(text string, cols int) []int {
	starts := []int{0}
	if cols <= 0 {
		return starts
	}
	col, lastSpace := 0, -1
	start := 0
	for i, r := range text {
		w := 1
		if r == '\t' {
			w = 4
		}
		if unicode.IsSpace(r) {
			col += w
			lastSpace = i + len(string(r))
			continue
		}
		if col+w > cols && i > start {
			cut := i
			if lastSpace > start {
				cut = lastSpace
			}
			starts = append(starts, cut)
			start, lastSpace = cut, -1
			col = columns(text[cut:i])
		}
		col += w
	}
	return starts
}
