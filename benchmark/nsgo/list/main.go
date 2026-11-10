// Command list is the list case on NSGo: 10,000 rows, scrolled a row
// further every display frame.
package main

import (
	"strconv"
	"time"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/benchmark/internal/bench"
	"github.com/oneo-me/nsgo/core"
)

var tones = [4]core.Color{core.RGB(0x007aff), core.RGB(0x34c759), core.RGB(0xff9500), core.RGB(0xaf52de)}

func root(cx *core.Scope) core.Node {
	list := &core.Ref{}
	top := 0
	var frame func(time.Time)
	frame = func(time.Time) {
		bench.Frame()
		top = (top + 1) % (bench.ListRows - 20)
		list.ScrollToIndex(top, false)
		cx.RequestAnimationFrame(frame)
	}
	cx.RequestAnimationFrame(frame)
	return core.List{
		Ref:           list,
		Count:         func() int { return bench.ListRows },
		EstimatedSize: bench.ListRowH,
		Style:         core.Style{FlexGrow: 1},
		Row: func(cx *core.Scope, row core.ListRow) core.Node {
			return core.View{
				Style: core.Style{Direction: core.Row, Height: core.Pt(bench.ListRowH), AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(0, 16)},
				Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						return core.Style{Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5, Background: tones[row.Index()%len(tones)]}
					}},
					core.Text{ContentFunc: func() string { return "Row " + strconv.Itoa(row.Index()) }, Style: core.Style{FlexGrow: 1, FontSize: 14}},
					core.Text{ContentFunc: func() string { return strconv.Itoa(row.Index() * 7 % 1000) }, Style: core.Style{FontSize: 13, Color: core.RGB(0x6b7280)}},
				},
			}
		},
	}
}

func main() { nsgo.Run(root, nsgo.WindowSize(480, 800)) }
