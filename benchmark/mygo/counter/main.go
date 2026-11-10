// Command counter is the counter case on MyGo: a count and two buttons.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/nsgo/benchmark/internal/bench"
)

type counter struct {
	n      int
	frames int
}

func (s *counter) view(c *ui.Context) {
	// The view builds every frame it draws: the frame after the first is
	// ready.
	if s.frames++; s.frames == 1 {
		c.Invalidate()
	} else if s.frames == 2 {
		bench.Frame()
	}
	ui.Column(c).Fill().Center().Gap(16).Children(func() {
		ui.Textf(c, "%d", s.n).FontSize(56).Bold()
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c.Key("decrement"), "−1").Clicked() {
				s.n--
			}
			if ui.PrimaryButton(c.Key("increment"), "+1").Clicked() {
				s.n++
			}
		})
	})
}

func main() {
	s := &counter{}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{Title: "Counter", Width: 360, Height: 260, Content: ui.View(s.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
