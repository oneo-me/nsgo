package vt

import (
	"runtime"
	"unsafe"
)

// Search options and data (GhosttySearchOption, GhosttySearchData).
const (
	searchOptNeedle       = 0
	searchOptSelectNext   = 1
	searchOptSelectPrev   = 2
	searchOptSelectScroll = 3

	searchScrollNone = 1

	searchDataStatus          = 0
	searchDataTotal           = 2
	searchDataSelectedIndex   = 3
	searchDataSelectedMatch   = 4
	searchDataViewportMatches = 6
)

// SearchStatus is how far a search went (GhosttySearchStatus).
type SearchStatus int32

const (
	// SearchRunning: Tick makes progress.
	SearchRunning SearchStatus = iota
	// SearchFeedRequired: Feed must read the terminal first.
	SearchFeedRequired
	// SearchComplete: the search caught up with the terminal as of the
	// last Feed.
	SearchComplete
)

// Search searches a terminal, its scrollback included, for text, as
// Ghostty's find does: matches follow the screen as it changes, and the
// screens as programs switch them. Matching is exact but for ASCII letters,
// whose case it ignores.
type Search struct {
	h uintptr
}

// selectionBuffer is a GhosttySelectionBuffer.
type selectionBuffer struct {
	ptr      *Selection
	cap, len uintptr
}

// NewSearch makes a search of t, which looks for nothing until SetText.
// Selecting a match scrolls nothing: the caller shows it where it likes.
func NewSearch(t *Terminal) (*Search, error) {
	s := &Search{}
	if err := result(call(fnSearchNew, 0, uintptr(unsafe.Pointer(&s.h)), t.h)); err != nil {
		return nil, err
	}
	none := int32(searchScrollNone)
	call(fnSearchSet, s.h, searchOptSelectScroll, uintptr(unsafe.Pointer(&none)))
	return s, nil
}

// Free frees the search, before or after its terminal.
func (s *Search) Free() {
	if s.h != 0 {
		call(fnSearchFree, s.h)
		s.h = 0
	}
}

// SetText searches for text from scratch, or keeps the matches when it is
// the text searched already; "" searches for nothing.
func (s *Search) SetText(text string) {
	b := []byte(text)
	cs := cBytes(b)
	call(fnSearchSet, s.h, searchOptNeedle, uintptr(unsafe.Pointer(&cs)))
	runtime.KeepAlive(b)
}

// Feed reads what changed of the terminal, a bounded amount at a time.
func (s *Search) Feed() { call(fnSearchFeed, s.h) }

// Tick makes a bounded amount of progress, without the terminal.
func (s *Search) Tick() SearchStatus {
	st := SearchComplete
	call(fnSearchTick, s.h, uintptr(unsafe.Pointer(&st)))
	return st
}

// Status returns how far the search went.
func (s *Search) Status() SearchStatus {
	st := SearchComplete
	call(fnSearchGet, s.h, searchDataStatus, uintptr(unsafe.Pointer(&st)))
	return st
}

// Total returns how many matches the search found so far.
func (s *Search) Total() int {
	var n uintptr
	call(fnSearchGet, s.h, searchDataTotal, uintptr(unsafe.Pointer(&n)))
	return int(n)
}

// Selected returns the index of the selected match, 0 the newest, and
// reports whether one is.
func (s *Search) Selected() (int, bool) {
	var i uintptr
	return int(i), ok(call(fnSearchGet, s.h, searchDataSelectedIndex, uintptr(unsafe.Pointer(&i))))
}

// SelectedMatch returns the selected match.
func (s *Search) SelectedMatch() (Selection, bool) {
	m := Selection{size: unsafe.Sizeof(Selection{})}
	return m, ok(call(fnSearchGet, s.h, searchDataSelectedMatch, uintptr(unsafe.Pointer(&m))))
}

// Select selects the next match toward older output, or toward newer
// output when back, wrapping around. It reports whether there was a match.
func (s *Search) Select(back bool) bool {
	opt := uintptr(searchOptSelectNext)
	if back {
		opt = searchOptSelectPrev
	}
	return ok(call(fnSearchSet, s.h, opt, 0))
}

// ViewportMatches returns the matches on the pages the viewport shows, as
// of the last Feed, in buf's memory when it has room.
func (s *Search) ViewportMatches(buf []Selection) []Selection {
	for {
		buf = buf[:cap(buf)]
		for i := range buf {
			buf[i].size = unsafe.Sizeof(Selection{})
		}
		b := selectionBuffer{cap: uintptr(len(buf))}
		if len(buf) > 0 {
			b.ptr = &buf[0]
		}
		r := Result(int32(call(fnSearchGet, s.h, searchDataViewportMatches, uintptr(unsafe.Pointer(&b)))))
		switch {
		case r == success:
			return buf[:b.len]
		case r == OutOfSpace && int(b.len) > len(buf):
			buf = make([]Selection, b.len)
		default:
			return buf[:0]
		}
	}
}

// ViewportPoint returns the column and the row of the viewport of a cell,
// and reports whether the viewport shows it.
func (t *Terminal) ViewportPoint(ref *GridRef) (x, y int, shown bool) {
	return t.pointOf(ref, pointViewport)
}

// ScreenRow returns the row of a cell among those of the screen and its
// scrollback, 0 the oldest, as ScrollToRow takes it.
func (t *Terminal) ScreenRow(ref *GridRef) (int, bool) {
	_, y, found := t.pointOf(ref, pointScreen)
	return y, found
}

// GhosttyPointTag's coordinate systems.
const (
	pointViewport = 1
	pointScreen   = 2
)

func (t *Terminal) pointOf(ref *GridRef, tag uintptr) (x, y int, found bool) {
	var p struct {
		x uint16
		_ uint16
		y uint32
	}
	if !ok(call(fnPointFromGridRef, t.h, uintptr(unsafe.Pointer(ref)), tag, uintptr(unsafe.Pointer(&p)))) {
		return 0, 0, false
	}
	return int(p.x), int(p.y), true
}

// Ends returns the cells a selection starts and ends at.
func (s *Selection) Ends() (start, end *GridRef) { return &s.start, &s.end }
