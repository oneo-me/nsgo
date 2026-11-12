package main

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

func init() {
	add("checkbox/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		done := core.NewSignal(cx, false)
		return ui.Checkbox{Checked: done.Get, OnChange: done.Set, Text: tr("Remember me", "记住我")}
	})
	add("checkbox/indeterminate", func(cx *core.Scope) core.Node {
		tr := words(cx)
		files := []string{"notes.md", "photo.jpg", "budget.csv"}
		picked := core.NewSignalFunc(cx, map[string]bool{"notes.md": true}, nil)
		chosen := core.NewMemo(cx, func() int { return len(picked.Get()) })
		chooseAll := func(all bool) {
			next := map[string]bool{}
			if all {
				for _, f := range files {
					next[f] = true
				}
			}
			picked.Set(next)
		}
		children := []core.Node{ui.Checkbox{
			Checked:       func() bool { return chosen.Get() == len(files) },
			Indeterminate: func() bool { return chosen.Get() > 0 && chosen.Get() < len(files) },
			OnChange:      chooseAll,
			Text:          tr("Select all", "全选"),
		}}
		for _, f := range files {
			children = append(children, core.View{Style: core.Style{Padding: core.Edges{Left: 24}}, Children: []core.Node{
				ui.Checkbox{Checked: func() bool { return picked.Get()[f] }, Text: f, OnChange: func(on bool) {
					picked.Update(func(m map[string]bool) map[string]bool {
						next := map[string]bool{}
						for k, v := range m {
							next[k] = v
						}
						if on {
							next[f] = true
						} else {
							delete(next, f)
						}
						return next
					})
				}},
			}})
		}
		return core.View{Style: core.Style{Gap: 8}, Children: children}
	})
}
