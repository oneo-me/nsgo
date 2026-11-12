package core

import (
	"strconv"
	"sync"

	"github.com/nsgo-dev/nsgo/internal/layout"
)

// GridTracks is a list of grid tracks in CSS syntax: lengths in px,
// percentages, fr, auto, min-content, max-content, fit-content(…),
// minmax(…, …), repeat(n | auto-fill | auto-fit, …) and [line names], as
// in "repeat(auto-fill, minmax(100px, 1fr))". A list layout cannot parse
// has no tracks. The zero value is unset: none.
type GridTracks string

// Valid reports whether layout can parse the tracks.
func (g GridTracks) Valid() bool {
	if g == "" {
		return true
	}
	_, err := parseTemplate(g)
	return err == nil
}

// gridTemplate is a parsed track list.
type gridTemplate struct {
	tracks []layout.GridTemplateComponent
	names  [][]string
	auto   []layout.TrackSizing
	err    error
}

// templates caches parsed track lists: styles hold the strings, and a
// layout reads them on every restyle.
var templates sync.Map // GridTracks → *gridTemplate

func parseTemplate(g GridTracks) (*gridTemplate, error) {
	if v, ok := templates.Load(g); ok {
		t := v.(*gridTemplate)
		return t, t.err
	}
	t := &gridTemplate{}
	t.tracks, t.names, t.err = layout.ParseGridTemplate(string(g))
	if t.err == nil {
		// Implicit tracks repeat a plain list of sizes.
		t.auto, _ = layout.ParseGridAutoTracks(string(g))
	}
	templates.Store(g, t)
	return t, t.err
}

// GridFlow is the CSS grid-auto-flow property: how a grid places items
// that do not say where they go.
type GridFlow uint8

const (
	FlowRow GridFlow = iota + 1
	FlowColumn
	FlowRowDense
	FlowColumnDense
)

// GridLines is where a grid item starts and ends on one axis. The zero
// value is auto: the grid places the item.
type GridLines struct{ Start, End GridPlacement }

// GridPlacement is one end of a grid item's row or column: a line, a span
// of tracks, or a named line. The zero value is auto.
type GridPlacement struct {
	line int16
	span uint16
	name string
}

// GridLine places an item at line n, counting from 1; negative lines count
// from the end of the explicit grid.
func GridLine(n int) GridPlacement { return GridPlacement{line: clampInt16(n)} }

// GridSpan makes an item span n tracks.
func GridSpan(n int) GridPlacement { return GridPlacement{span: uint16(min(max(n, 1), 0xFFFF))} }

// GridNamed places an item at the nth line called name (the first when n is
// 0); with a span it spans until the nth such line.
func GridNamed(name string, n int) GridPlacement {
	return GridPlacement{line: clampInt16(n), name: name}
}

// Span returns p spanning n named lines rather than placed at one.
func (p GridPlacement) Span(n int) GridPlacement {
	p.span, p.line = uint16(min(max(n, 1), 0xFFFF)), 0
	return p
}

func clampInt16(n int) int16 { return int16(min(max(n, -0x8000), 0x7FFF)) }

func (p GridPlacement) layout() layout.GridPlacement {
	switch {
	case p.span != 0:
		return layout.GridPlacement{Kind: layout.PlacementSpan, Span: p.span, Name: p.name}
	case p.line != 0 || p.name != "":
		return layout.GridPlacement{Kind: layout.PlacementLine, Line: p.line, Name: p.name}
	}
	return layout.GridPlacement{}
}

// String returns p in CSS syntax.
func (p GridPlacement) String() string {
	switch {
	case p.span != 0 && p.name != "":
		return "span " + strconv.Itoa(int(p.span)) + " " + p.name
	case p.span != 0:
		return "span " + strconv.Itoa(int(p.span))
	case p.name != "" && p.line != 0:
		return strconv.Itoa(int(p.line)) + " " + p.name
	case p.name != "":
		return p.name
	case p.line != 0:
		return strconv.Itoa(int(p.line))
	}
	return "auto"
}
