// Command list is the list case on MyGo: 10,000 rows, scrolled a row
// further every display frame.
package main

import (
	"log"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/nsgo-dev/nsgo/benchmark/internal/bench"
)

var tones = [4]ui.Color{ui.Hex("#007aff"), ui.Hex("#34c759"), ui.Hex("#ff9500"), ui.Hex("#af52de")}

type list struct {
	built bool
	state ui.ListState
	top   int
}

func (s *list) view(c *ui.Context) {
	// The first build comes before the first frame, which is ready.
	if s.built {
		bench.Frame()
	}
	s.built = true
	s.top = (s.top + 1) % (bench.ListRows - 20)
	s.state.ScrollTo(s.top, ui.Start)
	c.Invalidate()
	ui.List(c, &s.state, bench.ListRows, func(i int) {
		ui.Row(c).Height(bench.ListRowH).AlignItems(ui.Center).Gap(12).PaddingX(16).Children(func() {
			ui.Column(c).Width(10).Height(10).Radius(5).Background(tones[i%len(tones)])
			ui.Text(c, "Row "+strconv.Itoa(i)).Grow(1).FontSize(14)
			ui.Text(c, strconv.Itoa(i*7%1000)).FontSize(13).TextColor(ui.Hex("#6b7280"))
		})
	}).Fill()
}

func main() {
	s := &list{}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{Title: "List", Width: 480, Height: 800, Content: ui.View(s.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
