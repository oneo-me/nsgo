// Command quotes is the quotes case on NSGo: a table of 40 quotes, ten of
// which change every display frame.
package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/benchmark/internal/bench"
	"github.com/nsgo-dev/nsgo/core"
)

var widths = [5]core.Points{80, 90, 80, 80, 100}

var (
	ink   = core.RGB(0x111111)
	muted = core.RGB(0x6b7280)
	up    = core.RGB(0x1a7f37)
	down  = core.RGB(0xcf222e)
	zebra = core.RGB(0xf6f7f9)
	rule  = core.RGB(0xe5e7eb)
)

func tone(v float64) core.Color {
	if v < 0 {
		return down
	}
	return up
}

// cell is a column of a row, whose text follows the row's signal.
func cell(row *core.Signal[bench.Quote], col int, color func(bench.Quote) core.Color, text func(bench.Quote) string) core.Node {
	return core.Text{
		ContentFunc: func() string { return text(row.Get()) },
		StyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(widths[col]), FontSize: 13, Color: color(row.Get()), TextAlign: core.TextAlignEnd}
		},
	}
}

func header(label string, col int, align core.TextAlign) core.Node {
	return core.Text{Content: label, Style: core.Style{Width: core.Pt(widths[col]), FontSize: 12, Color: muted, TextAlign: align}}
}

func root(cx *core.Scope) core.Node {
	market := bench.NewQuotes()
	// A row is one signal: a tick sets the rows it changed, once each.
	rows := make([]*core.Signal[bench.Quote], len(market.Rows))
	for i, q := range market.Rows {
		rows[i] = core.NewSignal(cx, q)
	}
	ticks := core.NewSignal(cx, 0)
	var frame func(time.Time)
	frame = func(time.Time) {
		bench.Frame()
		for _, i := range market.Tick() {
			rows[i].Set(market.Rows[i])
		}
		ticks.Update(func(n int) int { return n + 1 })
		cx.RequestAnimationFrame(frame)
	}
	cx.RequestAnimationFrame(frame)

	table := make([]core.Node, len(rows))
	for i, row := range rows {
		bg := core.RGB(0xffffff)
		if i%2 == 1 {
			bg = zebra
		}
		table[i] = core.View{
			Style: core.Style{Direction: core.Row, Gap: 12, Height: core.Pt(bench.QuoteRowH), AlignItems: core.AlignCenter, Background: bg, CornerRadius: 4},
			Children: []core.Node{
				core.Text{Content: market.Rows[i].Symbol, Style: core.Style{Width: core.Pt(widths[0]), FontSize: 13, FontWeight: core.FontWeightSemibold, Color: ink}},
				cell(row, 1, func(bench.Quote) core.Color { return ink }, func(q bench.Quote) string { return strconv.FormatFloat(q.Price, 'f', 2, 64) }),
				cell(row, 2, func(q bench.Quote) core.Color { return tone(q.Change()) }, func(q bench.Quote) string { return strconv.FormatFloat(q.Change(), 'f', 2, 64) }),
				cell(row, 3, func(q bench.Quote) core.Color { return tone(q.Change()) }, func(q bench.Quote) string { return strconv.FormatFloat(q.Percent(), 'f', 2, 64) + "%" }),
				cell(row, 4, func(bench.Quote) core.Color { return muted }, func(q bench.Quote) string { return strconv.Itoa(q.Volume) }),
			},
		}
	}
	return core.View{
		Style: core.Style{FlexGrow: 1, Padding: core.All(16), Gap: bench.QuoteGap},
		Children: []core.Node{
			core.Text{
				ContentFunc: func() string { return fmt.Sprintf("Streaming quotes · tick %d", ticks.Get()) },
				Style:       core.Style{FontSize: 18, FontWeight: core.FontWeightSemibold, Color: ink},
			},
			core.View{
				Style: core.Style{Direction: core.Row, Gap: 12, Height: core.Pt(24), AlignItems: core.AlignCenter, BorderWidth: 1, BorderColor: rule},
				Children: []core.Node{
					header("Symbol", 0, core.TextAlignStart),
					header("Price", 1, core.TextAlignEnd),
					header("Change", 2, core.TextAlignEnd),
					header("%", 3, core.TextAlignEnd),
					header("Volume", 4, core.TextAlignEnd),
				},
			},
			core.View{Style: core.Style{Gap: bench.QuoteGap}, Children: table},
		},
	}
}

func main() { nsgo.Run(root, nsgo.WindowSize(560, 860)) }
