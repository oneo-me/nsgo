// Package parts is what base's components share to style their parts.
package parts

import (
	"strings"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// FirstFrame is how long a popup or a toast waits for its first display
// frame before it counts as past it without one.
const FirstFrame = 50 * time.Millisecond

// HandleBand is the width of the band that takes a drag to resize.
const HandleBand core.Points = 3

// Part is the class and styles the app gives one of a component's parts.
type Part struct {
	Class string
	Style core.Style
	Fn    func() core.Style
}

// Of is the Part of class, style and fn.
func Of(class string, style core.Style, fn func() core.Style) Part {
	return Part{Class: class, Style: style, Fn: fn}
}

// Set reports whether the app styles the part.
func (p Part) Set() bool { return p.Class != "" || !p.Style.IsZero() || p.Fn != nil }

// JoinClasses returns a's classes then b's, which win over a's.
func JoinClasses(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " " + b
}

// Fill lays a view over the whole of its parent.
var Fill = core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)}

// Cover is an overlay over its whole root view; overlays are as large as
// their content otherwise.
var Cover = core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Width: core.Pct(100), Height: core.Pct(100)}

// Important returns class with every utility Important (opacity-0 is
// opacity-0!), as NSH's toast root does.
func Important(class string) string {
	tokens := strings.Fields(class)
	for i, t := range tokens {
		if u := t[strings.LastIndexByte(t, ':')+1:]; !strings.HasSuffix(t, "!") && !strings.HasPrefix(u, "!") {
			tokens[i] = t + "!"
		}
	}
	return strings.Join(tokens, " ")
}
