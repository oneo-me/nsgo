package main

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

func init() {
	add("select/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		size := core.NewSignal(cx, "m")
		return form(ui.Select[string]{
			Options:  []ui.Option[string]{{Value: "s", Label: tr("Small", "小")}, {Value: "m", Label: tr("Medium", "中")}, {Value: "l", Label: tr("Large", "大")}},
			Value:    size.Get,
			OnChange: size.Set,
			Label:    tr("Size", "尺寸"),
		})
	})
	add("select/searchable", func(cx *core.Scope) core.Node {
		tr := words(cx)
		var languages []ui.Option[string]
		for _, name := range []string{"Dansk", "Deutsch", "English", "Español", "Français", "Italiano", "Nederlands", "Polski", "Português", "Svenska", "Türkçe", "中文", "日本語", "한국어"} {
			languages = append(languages, ui.Option[string]{Value: name, Label: name})
		}
		language := core.NewSignal(cx, "")
		return form(ui.Select[string]{
			Options:           languages,
			Value:             language.Get,
			OnChange:          language.Set,
			Searchable:        true,
			SearchPlaceholder: tr("Search languages", "搜索语言"),
			OnClear:           func() { language.Set("") },
			Placeholder:       tr("Choose a language", "选择语言"),
		})
	})
}
