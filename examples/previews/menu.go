package main

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

func init() {
	add("menu/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		l := newLog(cx)
		rename, duplicate, remove := l.says(tr("Renamed", "已重命名")), l.says(tr("Duplicated", "已复制一份")), l.says(tr("Deleted", "已删除"))
		return column(
			ui.Menu{Trigger: ui.Button{Text: tr("Actions", "操作"), Variant: ui.ButtonSecondary}, Children: []core.Node{
				ui.MenuItem{Text: tr("Rename", "重命名"), Icon: lucide.Pencil, OnSelect: rename},
				ui.MenuItem{Text: tr("Duplicate", "复制一份"), Shortcut: "mod+d", OnSelect: duplicate},
				ui.MenuSeparator{},
				ui.MenuItem{Text: tr("Delete", "删除"), Variant: ui.MenuItemDanger, OnSelect: remove},
			}},
			l,
		)
	})
	add("menu/checked", func(cx *core.Scope) core.Node {
		tr := words(cx)
		l := newLog(cx)
		sort := core.NewSignal(cx, "name")
		copyLink, emailLink := l.says(tr("Link copied", "已复制链接")), l.says(tr("Link emailed", "已用邮件发送链接"))
		return column(
			ui.Menu{Trigger: ui.Button{Text: tr("View", "显示"), Variant: ui.ButtonGhost}, Children: []core.Node{
				ui.MenuLabel{Text: tr("Sort by", "排序方式")},
				ui.MenuItem{Text: tr("Name", "名称"), Checked: func() bool { return sort.Get() == "name" }, OnSelect: func() { sort.Set("name") }},
				ui.MenuItem{Text: tr("Date", "日期"), Checked: func() bool { return sort.Get() == "date" }, OnSelect: func() { sort.Set("date") }},
				ui.MenuSeparator{},
				ui.Submenu{Text: tr("Share", "分享"), Children: []core.Node{
					ui.MenuItem{Text: tr("Copy link", "复制链接"), OnSelect: copyLink},
					ui.MenuItem{Text: tr("Email", "邮件"), OnSelect: emailLink},
				}},
			}},
			l,
		)
	})
	add("menu/context", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		l := newLog(cx)
		open, remove := l.says(tr("Opened report.pdf", "已打开 report.pdf")), l.says(tr("Deleted report.pdf", "已删除 report.pdf"))
		fileRow := core.View{
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(10, 14), CornerRadius: 8, BorderWidth: 1},
			StyleFunc: func() core.Style {
				return core.Style{Background: colors.Surface(), BorderColor: colors.Border()}
			},
			Children: []core.Node{
				core.Icon{SVG: lucide.FileText(), Style: core.Style{Width: core.Pt(16), Height: core.Pt(16)}},
				ui.Text(tr("report.pdf · right-click or long-press", "report.pdf · 右键或长按"), 13, ui.Regular, colors.Text),
			},
		}
		return column(
			ui.ContextMenu{Trigger: fileRow, Children: []core.Node{
				ui.MenuItem{Text: tr("Open", "打开"), OnSelect: open},
				ui.MenuItem{Text: tr("Delete", "删除"), Variant: ui.MenuItemDanger, OnSelect: remove},
			}},
			l,
		)
	})
}
