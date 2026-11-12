package main

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

func init() {
	add("button/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		l := newLog(cx)
		save, cancel := l.says(tr("Saved", "已保存")), l.says(tr("Canceled", "已取消"))
		return column(row(
			ui.Button{Text: tr("Save", "保存"), OnPress: save},
			ui.Button{Text: tr("Cancel", "取消"), Variant: ui.ButtonSecondary, OnPress: cancel},
		), l)
	})
	add("button/variants", func(cx *core.Scope) core.Node {
		tr := words(cx)
		l := newLog(cx)
		remove, next, save := l.says(tr("Deleted", "已删除")), l.says(tr("Continued", "已继续")), l.says(tr("Saved", "已保存"))
		return column(row(
			ui.Button{Text: tr("Delete", "删除"), Variant: ui.ButtonOutline, OnPress: remove},
			ui.Button{Text: tr("Continue", "继续"), Size: ui.SizeLarge, OnPress: next},
		), core.View{Style: core.Style{Direction: core.Row, Width: core.Pt(240)}, Children: []core.Node{
			ui.Button{Text: tr("Save", "保存"), Icon: lucide.Check, Flex: 1, OnPress: save},
		}}, l)
	})
	add("button/icon", func(cx *core.Scope) core.Node {
		tr := words(cx)
		l := newLog(cx)
		remove := l.says(tr("Deleted", "已删除"))
		return column(
			ui.IconButton{Icon: lucide.Trash, Label: tr("Delete", "删除"), Variant: ui.IconButtonGhost, OnPress: remove},
			l,
		)
	})
}
