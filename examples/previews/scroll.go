package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

func init() {
	add("scroll/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		var rows []core.Node
		for i := range 30 {
			rows = append(rows, ui.Text(fmt.Sprintf(tr("Row %d", "第 %d 行"), i+1), 13, ui.Regular, colors.Text))
		}
		return core.Scroll{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			core.View{Style: core.Style{Padding: core.All(16), Gap: 12}, Children: rows},
		}}
	})
	add("scroll/paging", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		var pages []core.Node
		for i := range 4 {
			pages = append(pages, core.View{
				Style: core.Style{Width: core.Pct(100), Height: core.Pct(100), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				Children: []core.Node{
					ui.Text(fmt.Sprintf(tr("Page %d of 4", "第 %d 页，共 4 页"), i+1), 20, ui.Bold, colors.Text),
					ui.Text(tr("Swipe or scroll up for the next.", "滑动或向上滚动看下一页。"), 13, ui.Regular, colors.Muted),
				},
			})
		}
		return core.Scroll{
			Behavior: core.ScrollBehavior{
				Paging:                    true,
				KeyboardDismissMode:       core.KeyboardDismissOnDrag,
				KeyboardShouldPersistTaps: core.KeyboardTapsHandled,
			},
			Style:    core.Style{FlexGrow: 1},
			Children: pages,
		}
	})
}
