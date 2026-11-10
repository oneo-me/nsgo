package main

import (
	"strings"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

func init() {
	add("input/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		name := core.NewSignal(cx, "")
		return form(
			ui.Input{
				Value:       name.Get,
				OnChange:    name.Set,
				Placeholder: tr("Your name", "你的名字"),
				TestID:      "name",
			},
			ui.Textf(func() string {
				if name.Get() == "" {
					return tr("Hello!", "你好！")
				}
				return tr("Hello, ", "你好，") + name.Get() + tr("!", "！")
			}, 13, ui.Regular, colors.Muted),
		)
	})
	add("input/submit", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		tasks := core.NewSignalFunc(cx, []string{}, nil)
		draft := core.NewSignal(cx, "")
		add := func() {
			if text := strings.TrimSpace(draft.Get()); text != "" {
				tasks.Update(func(list []string) []string { return append(list[:len(list):len(list)], text) })
			}
		}
		return form(
			ui.Input{Value: draft.Get, OnChange: draft.Set, Placeholder: tr("New task", "新任务"),
				OnSubmit: func(string) { add(); draft.Set("") }},
			ui.Textf(func() string { return strings.Join(tasks.Get(), " · ") }, 13, ui.Regular, colors.Muted),
		)
	})
	add("input/validation", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		email := core.NewSignal(cx, "")
		err := core.NewSignal(cx, "")
		return form(core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			ui.Input{Value: email.Get, OnChange: email.Set, Label: tr("Email", "邮箱"),
				OnSubmit: func(string) {
					if !strings.Contains(email.Get(), "@") {
						err.Set(tr("Enter an email address.", "请输入邮箱地址。"))
					} else {
						err.Set("")
					}
				}},
			ui.Textf(err.Get, 12, ui.Regular, colors.Danger),
		}})
	})
}
