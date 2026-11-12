package main

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

func init() {
	add("dialog/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		l := newLog(cx)
		name := core.NewSignal(cx, "Notes.md")
		open := core.NewSignal(cx, false)
		save := func() {
			open.Set(false)
			l.text.Set(tr("Renamed to ", "已重命名为 ") + name.Get())
		}
		return column(
			ui.Button{Text: tr("Rename…", "重命名…"), Variant: ui.ButtonSecondary, OnPress: func() { open.Set(true) }},
			ui.Dialog{
				Open:         open.Get,
				OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) },
				Title:        tr("Rename the file", "重命名文件"),
				Description:  tr("The new name shows everywhere.", "新名称会显示在所有地方。"),
				Children:     []core.Node{ui.Input{Value: name.Get, OnChange: name.Set}},
				Footer: []core.Node{
					ui.Button{Text: tr("Cancel", "取消"), Variant: ui.ButtonSecondary, OnPress: func() { open.Set(false) }},
					ui.Button{Text: tr("Save", "保存"), OnPress: save},
				},
			},
			l,
		)
	})
	add("dialog/sheet", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		filters := core.NewSignal(cx, false)
		unread, starred := core.NewSignal(cx, true), core.NewSignal(cx, false)
		filterControls := []core.Node{
			ui.Text(tr("Filters", "筛选"), 17, ui.Bold, colors.Text),
			ui.Switch{Checked: unread.Get, OnChange: unread.Set, Label: tr("Unread only", "只看未读")},
			ui.Switch{Checked: starred.Get, OnChange: starred.Set, Label: tr("Starred only", "只看星标")},
		}
		return column(
			ui.Button{Text: tr("Filters", "筛选"), Variant: ui.ButtonSecondary, OnPress: func() { filters.Set(true) }},
			ui.Sheet{Open: filters.Get, OnClose: func() { filters.Set(false) }, Children: filterControls},
		)
	})
	add("dialog/alert", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		l := newLog(cx)
		confirm := core.NewSignal(cx, false)
		remove := l.says(tr("Deleted", "已删除"))
		return column(
			ui.Button{Text: tr("Delete…", "删除…"), Variant: ui.ButtonOutline, OnPress: func() { confirm.Set(true) }},
			ui.AlertDialog{
				Open:         confirm.Get,
				OnOpenChange: func(v bool, _ base.Reason) { confirm.Set(v) },
				Title:        tr("Delete the note?", "删除这条笔记？"),
				Description:  tr("This cannot be undone.", "此操作无法撤销。"),
				Actions: []ui.AlertAction{
					{Text: tr("Cancel", "取消"), Variant: ui.ButtonSecondary},
					{Text: tr("Delete", "删除"), Color: colors.Danger, OnPress: remove},
				},
			},
			l,
		)
	})
}
