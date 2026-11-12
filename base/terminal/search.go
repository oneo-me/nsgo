package terminal

import (
	"time"

	"github.com/nsgo-dev/nsgo/base/terminal/internal/vt"
)

// SearchStatus is how a search of the terminal goes (Terminal.Search,
// Options.OnSearch).
type SearchStatus struct {
	// Text is what the search looks for: "" when none goes.
	Text string
	// Total counts the matches found so far, on the screen that shows: a
	// full-screen program's, or the main screen's with its scrollback.
	// Selected is the selected match's place among them, 1 the newest, as
	// a find bar's "1 of 3"; 0 when none is.
	Total, Selected int
	// Searching tells that the search has more of the scrollback to go
	// through: Total may grow.
	Searching bool
}

// search is the terminal's search, while one goes; t.mu guards it.
type search struct {
	s *vt.Search
	// pick selects the newest match once one is found, as a search
	// starts.
	pick   bool
	status SearchStatus
}

// searchBudget bounds what a frame spends going through the scrollback; a
// search of more goes on in the frames after it.
const searchBudget = 4 * time.Millisecond

// Search searches the screen and the scrollback for text, as Ghostty's
// find does, and highlights the matches (Theme.Search) as they are found,
// following the output that comes; the newest match is selected and
// shown, scrolling to it. Matching is exact but for the case of ASCII
// letters, which it ignores. Searching for the text searched already keeps
// the selected match; "" ends the search, as EndSearch. A view drives the
// search, which goes on in the frames that show the terminal.
func (t *Terminal) Search(text string) {
	if text == "" {
		t.EndSearch()
		return
	}
	t.mu.Lock()
	if t.term == nil {
		t.mu.Unlock()
		return
	}
	f := &t.find
	if f.s == nil {
		s, err := vt.NewSearch(t.term)
		if err != nil {
			t.mu.Unlock()
			return
		}
		f.s = s
	}
	if f.status.Text != text {
		f.s.SetText(text)
		f.status, f.pick = SearchStatus{Text: text, Searching: true}, true
	}
	t.mu.Unlock()
	t.redraw()
}

// SearchNext selects the next match of the search toward older output, up
// the screen and into the scrollback, wrapping around past the oldest, and
// unless it shows scrolls it to the middle of the view, clear of a find bar
// over an edge; SearchPrevious goes the other way.
func (t *Terminal) SearchNext()     { t.selectMatch(false) }
func (t *Terminal) SearchPrevious() { t.selectMatch(true) }

func (t *Terminal) selectMatch(back bool) {
	t.mu.Lock()
	if f := &t.find; f.s != nil && t.term != nil {
		// A match picked by hand is the one to keep.
		f.pick = false
		f.s.Select(back)
		f.show(t.term)
		f.update()
	}
	t.mu.Unlock()
	t.redraw()
}

// EndSearch ends the search, and its highlights.
func (t *Terminal) EndSearch() {
	t.mu.Lock()
	ended := t.find.s != nil
	t.endSearch()
	t.mu.Unlock()
	if ended {
		t.redraw()
	}
}

// endSearch frees the search; t.mu is held.
func (t *Terminal) endSearch() {
	if t.find.s != nil {
		t.find.s.Free()
	}
	t.find = search{}
}

// SearchStatus returns how the search goes.
func (t *Terminal) SearchStatus() SearchStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.find.status
}

// show scrolls the selected match to the middle of the viewport unless
// all of it shows, as browsers do, rather than to an edge, where an app's
// find bar may cover it; t.mu is held.
func (f *search) show(term *vt.Terminal) {
	m, ok := f.s.SelectedMatch()
	if !ok {
		return
	}
	a, b := m.Ends()
	_, rows := term.Size()
	_, y0, ok0 := term.ViewportPoint(a)
	_, y1, ok1 := term.ViewportPoint(b)
	if ok0 && ok1 && y0 < rows && y1 < rows {
		return
	}
	if y, ok := term.ScreenRow(a); ok {
		term.ScrollToRow(y - rows/2)
	}
}

// drive goes on with the search for a frame, and reports whether it has
// more to go through; t.mu is held.
func (f *search) drive(term *vt.Terminal) bool {
	if f.s == nil {
		return false
	}
	f.s.Feed()
	deadline := time.Now().Add(searchBudget)
	for st := f.s.Tick(); st != vt.SearchComplete; st = f.s.Tick() {
		if st == vt.SearchFeedRequired {
			f.s.Feed()
		}
		if time.Now().After(deadline) {
			break
		}
	}
	if f.pick && f.s.Total() > 0 {
		f.pick = false
		f.s.Select(false)
		f.show(term)
	}
	f.update()
	return f.status.Searching
}

// update reads the status of the search; t.mu is held.
func (f *search) update() {
	st := SearchStatus{Text: f.status.Text, Total: f.s.Total(), Searching: f.s.Status() != vt.SearchComplete}
	if i, ok := f.s.Selected(); ok {
		st.Selected = i + 1
	}
	f.status = st
}

// matchSpan is a match's cells in a row of the viewport, x0 to x1
// inclusive.
type matchSpan struct {
	y, x0, x1 int
	selected  bool
}

// viewportMatches returns the cells of the matches the viewport shows, in
// spans' memory, rows rows tall and cols wide; t.mu is held.
func (t *Terminal) viewportMatches(spans []matchSpan, buf []vt.Selection, cols, rows int) ([]matchSpan, []vt.Selection) {
	spans = spans[:0]
	f := &t.find
	if f.s == nil || f.status.Total == 0 {
		return spans, buf
	}
	// The matches as of the terminal now.
	f.s.Feed()
	buf = f.s.ViewportMatches(buf)
	// The cells of a match in the viewport: one that wraps in from above
	// it starts at its top left, and one that wraps on below it ends at
	// its bottom right.
	cells := func(m *vt.Selection) (c [4]int, shows bool) {
		a, b := m.Ends()
		x0, y0, ok0 := t.term.ViewportPoint(a)
		x1, y1, ok1 := t.term.ViewportPoint(b)
		switch {
		case !ok0 && !ok1:
			return c, false
		case !ok0:
			x0, y0 = 0, 0
		case !ok1:
			x1, y1 = cols-1, rows-1
		}
		return [4]int{x0, y0, x1, y1}, y0 < rows
	}
	var sel [4]int
	selOK := false
	if m, ok := f.s.SelectedMatch(); ok {
		sel, selOK = cells(&m)
	}
	for i := range buf {
		c, shows := cells(&buf[i])
		if !shows {
			continue
		}
		x0, y0, x1, y1 := c[0], c[1], c[2], c[3]
		selected := selOK && sel == c
		// A match that wraps takes the rows it goes through.
		for y := y0; y <= min(y1, rows-1); y++ {
			s := matchSpan{y, 0, cols - 1, selected}
			if y == y0 {
				s.x0 = x0
			}
			if y == y1 {
				s.x1 = x1
			}
			spans = append(spans, s)
		}
	}
	return spans, buf
}
