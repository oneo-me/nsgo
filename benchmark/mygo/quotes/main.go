// Command quotes is the quotes case on MyGo: a table of 40 quotes, ten of
// which change every display frame.
package main

import (
	"log"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/nsgo-dev/nsgo/benchmark/internal/bench"
)

var widths = [5]float32{80, 90, 80, 80, 100}

var (
	ink   = ui.Hex("#111111")
	muted = ui.Hex("#6b7280")
	up    = ui.Hex("#1a7f37")
	down  = ui.Hex("#cf222e")
	zebra = ui.Hex("#f6f7f9")
	rule  = ui.Hex("#e5e7eb")
	white = ui.Hex("#ffffff")
)

func tone(v float64) ui.Color {
	if v < 0 {
		return down
	}
	return up
}

type quotes struct {
	built  bool
	market *bench.Quotes
	ticks  int
}

// view builds the table every frame and asks for the next, ticking the
// market each time.
func (s *quotes) view(c *ui.Context) {
	// The first build comes before the first frame, which is ready.
	if s.built {
		bench.Frame()
	}
	s.built = true
	s.market.Tick()
	s.ticks++
	c.Invalidate()
	ui.Column(c).Fill().Background(white).Padding(16).Gap(bench.QuoteGap).Children(func() {
		ui.Textf(c, "Streaming quotes · tick %d", s.ticks).FontSize(18).FontWeight(600).TextColor(ink)
		ui.Row(c).Gap(12).Height(24).AlignItems(ui.Center).Border(1, rule).Children(func() {
			for i, label := range []string{"Symbol", "Price", "Change", "%", "Volume"} {
				align := ui.End
				if i == 0 {
					align = ui.Start
				}
				ui.Text(c, label).Width(widths[i]).FontSize(12).TextColor(muted).TextAlign(align)
			}
		})
		ui.Column(c).Gap(bench.QuoteGap).Children(func() {
			for i, q := range s.market.Rows {
				bg := white
				if i%2 == 1 {
					bg = zebra
				}
				ui.Row(c.Key(i)).Gap(12).Height(bench.QuoteRowH).AlignItems(ui.Center).Background(bg).Radius(4).Children(func() {
					ui.Text(c, q.Symbol).Width(widths[0]).FontSize(13).FontWeight(600).TextColor(ink)
					ui.Text(c, strconv.FormatFloat(q.Price, 'f', 2, 64)).Width(widths[1]).FontSize(13).TextColor(ink).TextAlign(ui.End)
					ui.Text(c, strconv.FormatFloat(q.Change(), 'f', 2, 64)).Width(widths[2]).FontSize(13).TextColor(tone(q.Change())).TextAlign(ui.End)
					ui.Text(c, strconv.FormatFloat(q.Percent(), 'f', 2, 64)+"%").Width(widths[3]).FontSize(13).TextColor(tone(q.Change())).TextAlign(ui.End)
					ui.Text(c, strconv.Itoa(q.Volume)).Width(widths[4]).FontSize(13).TextColor(muted).TextAlign(ui.End)
				})
			}
		})
	})
}

func main() {
	s := &quotes{market: bench.NewQuotes()}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{Title: "Quotes", Width: 560, Height: 860, Content: ui.View(s.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
