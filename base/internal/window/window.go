// Package window is the rows a virtualized view of base makes: a table's,
// an editor's.
package window

// Rows is the rows a virtualized view makes: those in view and a
// reach around them, moved only when the viewport nears the window's edge,
// so scrolling a little changes no rows and a scroll finds the rows of the
// next frames made already.
type Rows struct {
	// margin is rows kept made past the viewport's edges, at least; ahead
	// rows made ahead of it when the window moves.
	margin, ahead     int
	start, end, first int
}

// New is a window keeping margin rows made past the viewport's edges and
// making ahead rows ahead of it when it moves, at least.
func New(margin, ahead int) *Rows { return &Rows{margin: margin, ahead: ahead} }

// Span is the rows to make with visible rows in view of count: the last
// window while it covers them with half a viewport to spare, else the
// visible rows, a viewport and a half more ahead the way they scrolled and
// half a viewport behind, or as many on one side as the other end leaves.
func (w *Rows) Span(vStart, vEnd, count int) (int, int) {
	page := vEnd - vStart
	// Before the viewport has a size, the rows ahead stand in, and the
	// window waits for the first real view: kept, it would be too short
	// for it and move on the first scroll.
	if page <= 0 {
		return min(vStart, count), min(vStart+w.ahead, count)
	}
	margin := max(page/2, w.margin)
	needStart, needEnd := max(vStart-margin, 0), min(vEnd+margin, count)
	if w.start <= needStart && needEnd <= w.end && w.end <= count && w.end-w.start <= page*4+w.ahead*3 {
		return w.start, w.end
	}
	ahead, behind := max(page*3/2, w.ahead), max(page/2, w.ahead/2)
	before, after := behind, ahead
	if vStart < w.first {
		before, after = ahead, behind
	}
	// Cut at one end, the window takes the rows on the other: it keeps
	// its length, so the slots made for it at the top are those it needs
	// further down, and none is made on the first scroll.
	start, end := vStart-before, vEnd+after
	if start < 0 {
		end -= start
		start = 0
	}
	if end > count {
		start = max(start-(end-count), 0)
		end = count
	}
	w.start, w.end, w.first = start, end, vStart
	return w.start, w.end
}
