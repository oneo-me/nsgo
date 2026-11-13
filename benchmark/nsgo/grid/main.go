// Command grid is a stress case on NSGo alone: 600 cells, 30 by 20, whose
// numbers all change every display frame, so each frame changes 600 texts.
package main

import (
	"strconv"
	"time"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/benchmark/internal/bench"
	"github.com/nsgo-dev/nsgo/core"
)

const (
	cols, rows = 30, 20
	cellW      = 40
	cellH      = 20
)

var (
	ink   = core.RGB(0x111111)
	zebra = core.RGB(0xf6f7f9)
)

func root(cx *core.Scope) core.Node {
	// A row is one signal of its numbers: a tick sets every row once.
	seed := uint32(42)
	next := func() int {
		seed = seed*1664525 + 1013904223
		return int(seed>>16) % 10000
	}
	values := make([]*core.Signal[[cols]int], rows)
	for r := range values {
		var v [cols]int
		for c := range v {
			v[c] = next()
		}
		values[r] = core.NewSignal(cx, v)
	}
	var frame func(time.Time)
	frame = func(time.Time) {
		bench.Frame()
		for _, row := range values {
			var v [cols]int
			for c := range v {
				v[c] = next()
			}
			row.Set(v)
		}
		cx.RequestAnimationFrame(frame)
	}
	cx.RequestAnimationFrame(frame)

	lines := make([]core.Node, rows)
	for r, row := range values {
		cells := make([]core.Node, cols)
		for c := range cells {
			cells[c] = core.Text{
				ContentFunc: func() string { return strconv.Itoa(row.Get()[c]) },
				Style:       core.Style{Width: core.Pt(cellW), FontSize: 12, Color: ink, TextAlign: core.TextAlignEnd},
			}
		}
		bg := core.RGB(0xffffff)
		if r%2 == 1 {
			bg = zebra
		}
		lines[r] = core.View{
			Style:    core.Style{Direction: core.Row, Height: core.Pt(cellH), AlignItems: core.AlignCenter, Background: bg},
			Children: cells,
		}
	}
	return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(16)}, Children: lines}
}

func main() { nsgo.Run(root, nsgo.WindowSize(cols*cellW+32, rows*cellH+32)) }
