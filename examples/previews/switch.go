package main

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

func init() {
	add("switch/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		notifications := core.NewSignal(cx, true)
		return ui.Switch{Checked: notifications.Get, OnChange: notifications.Set, Label: tr("Notifications", "通知")}
	})
	add("switch/small", func(cx *core.Scope) core.Node {
		tr := words(cx)
		sync := core.NewSignal(cx, false)
		return ui.Switch{Checked: sync.Get, OnChange: sync.Set, Label: tr("Sync automatically", "自动同步"), Size: ui.SizeSmall}
	})
}
