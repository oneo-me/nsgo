package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/base/textview"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

func init() {
	add("text/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		n := core.NewSignal(cx, 3)
		return column(
			ui.Text(tr("Settings", "设置"), 20, ui.Bold, colors.Text),
			ui.Textf(func() string { return fmt.Sprintf(tr("%d files", "%d 个文件"), n.Get()) }, 13, ui.Regular, colors.Muted),
			ui.Button{Text: tr("Add a file", "添加文件"), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: func() { n.Set(n.Get() + 1) }},
		)
	})
	add("text/spans", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		l := newLog(cx)
		openGuide := func(core.PressEvent) { l.text.Set(tr("Opened the guide", "已打开指南")) }
		return column(
			core.Text{Style: core.Style{FontSize: 15}, StyleFunc: func() core.Style { return core.Style{Color: colors.Text()} }, Spans: []core.Span{
				{Content: tr("Read the ", "开始前请阅读")},
				{Content: tr("guide", "指南"), Style: core.Style{TextDecoration: core.TextDecorationUnderline}, OnPress: openGuide},
				{Content: tr(" before you start.", "。")},
			}},
			l,
		)
	})
	add("text/document", func(cx *core.Scope) core.Node {
		tr := words(cx)
		open := func(url string) { cx.OpenURL(url, nil) }
		return core.View{Style: core.Style{Width: core.Pt(320), MaxWidth: core.Pct(100)}, Children: []core.Node{
			textview.TextView{Markdown: tr("## Notes\n\nA **native** document, with [a link](https://nsgo.dev/).", "## 笔记\n\n一份 **原生** 文档，带一个[链接](https://nsgo.dev/)。"), Styles: ui.TextViewStyles(cx),
				OnLinkPress: func(e textview.LinkPressEvent) { open(e.URL) }},
		}}
	})
}
