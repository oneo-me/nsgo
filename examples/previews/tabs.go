package main

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// sections are the sections the tab demos switch between.
func sections(tr func(en, zh string) string) []ui.Option[string] {
	return []ui.Option[string]{
		{Value: "overview", Label: tr("Overview", "概览")},
		{Value: "activity", Label: tr("Activity", "动态")},
		{Value: "settings", Label: tr("Settings", "设置")},
	}
}

// shown is the section a tab demo shows below its tabs.
func shown(cx *core.Scope, section *core.Signal[string]) core.Node {
	tr := words(cx)
	colors := ui.ColorsOf(cx)
	return ui.Textf(func() string {
		for _, t := range sections(tr) {
			if t.Value == section.Get() {
				return t.Label + tr(" shows here.", "显示在这里。")
			}
		}
		return ""
	}, 13, ui.Regular, colors.Muted)
}

func init() {
	add("tabs/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		section := core.NewSignal(cx, "overview")
		return column(ui.Tabs[string]{
			Options: []ui.Option[string]{
				{Value: "overview", Label: tr("Overview", "概览")},
				{Value: "activity", Label: tr("Activity", "动态")},
				{Value: "settings", Label: tr("Settings", "设置")},
			},
			Value:    section.Get,
			OnChange: section.Set,
			Label:    tr("Sections", "分区"),
		}, shown(cx, section))
	})
	add("tabs/underline", func(cx *core.Scope) core.Node {
		tabs := sections(words(cx))
		section := core.NewSignal(cx, "overview")
		return form(
			ui.Tabs[string]{Variant: ui.TabsUnderline, Equal: true, Size: ui.SizeSmall, Options: tabs, Value: section.Get, OnChange: section.Set},
			shown(cx, section),
		)
	})
}
