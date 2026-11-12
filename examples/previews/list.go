package main

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// item is a list demo's row.
type item struct{ ID int }

// title is an item's title in the app's language.
func title(tr func(en, zh string) string, it item) string {
	return fmt.Sprintf(tr("Item %d", "第 %d 项"), it.ID)
}

// items are a thousand rows, which a list builds only in view.
var items = func() []item {
	list := make([]item, 1000)
	for i := range list {
		list[i] = item{ID: i + 1}
	}
	return list
}()

func init() {
	add("list/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		return ui.List{
			Count:         func() int { return len(items) },
			Key:           func(i int) any { return items[i].ID },
			EstimatedSize: 44,
			Style:         core.Style{FlexGrow: 1},
			RowStyle:      core.Style{Padding: core.Symmetric(12, 16)},
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				item := items[row.Index()]
				return ui.Text(title(tr, item), 13, ui.Regular, colors.Text)
			},
		}
	})
	add("list/selection", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		opened := core.NewSignal(cx, tr("Double-click or press Return to open a row.", "双击或按回车打开一行。"))
		count := func() int { return len(items) }
		open := func(it item) { opened.Set(tr("Opened ", "已打开 ") + title(tr, it)) }
		rowContent := func(cx *core.Scope, it item) core.Node {
			return ui.Text(title(tr, it), 13, ui.Regular, colors.Text)
		}
		selected := ui.NewSelection(cx, true)
		return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			ui.List{
				Count: count, Key: func(i int) any { return items[i].ID },
				Selection:  selected,
				OnActivate: func(i int) { open(items[i]) },
				RowLabel:   func(i int) string { return title(tr, items[i]) },
				Row:        func(cx *core.Scope, row core.ListRow) core.Node { return rowContent(cx, items[row.Index()]) },
				Style:      core.Style{FlexGrow: 1},
				RowStyle:   core.Style{Padding: core.Symmetric(12, 16)},
			},
			core.View{Style: core.Style{Padding: core.Symmetric(8, 16)}, Children: []core.Node{
				ui.Textf(opened.Get, 13, ui.Regular, colors.Muted),
			}},
		}}
	})
}
