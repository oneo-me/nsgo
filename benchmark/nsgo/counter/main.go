// Command counter is the counter case on NSGo: a count and two buttons.
package main

import (
	"strconv"
	"time"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/benchmark/internal/bench"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

func root(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	// The first frame after the first commit is ready.
	cx.RequestAnimationFrame(func(time.Time) { bench.Frame() })
	return core.View{
		Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 16},
		Children: []core.Node{
			core.Text{
				ContentFunc: func() string { return strconv.Itoa(count.Get()) },
				Style:       core.Style{FontSize: 56, FontWeight: core.FontWeightBold},
			},
			core.View{
				Style: core.Style{Direction: core.Row, Gap: 8},
				Children: []core.Node{
					ui.Button{Text: "−1", Variant: ui.ButtonSecondary, OnPress: func() { count.Update(func(n int) int { return n - 1 }) }},
					ui.Button{Text: "+1", OnPress: func() { count.Update(func(n int) int { return n + 1 }) }},
				},
			},
		},
	}
}

func main() { nsgo.Run(root, nsgo.WindowSize(360, 260)) }
