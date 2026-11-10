package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

func init() {
	add("slider/basic", func(cx *core.Scope) core.Node {
		tr := words(cx)
		volume := core.NewSignal(cx, 40.0)
		return form(ui.Slider{Value: volume.Get, OnChange: volume.Set, Min: 0, Max: 100, Step: 5, Label: tr("Volume", "音量")})
	})
	add("slider/range", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		price := core.NewSignal(cx, 20.0)
		ceiling := core.NewSignal(cx, 80.0)
		return form(
			ui.Slider{
				Value: price.Get, OnChange: price.Set,
				High: ceiling.Get, OnHighChange: ceiling.Set,
				Min: 0, Max: 100, Step: 1,
				Label: tr("Price", "价格"), LowLabel: tr("Minimum", "最低"), HighLabel: tr("Maximum", "最高"),
			},
			ui.Textf(func() string { return fmt.Sprintf("$%g – $%g", price.Get(), ceiling.Get()) }, 13, ui.Regular, colors.Muted),
		)
	})
	add("slider/format", func(cx *core.Scope) core.Node {
		tr := words(cx)
		colors := ui.ColorsOf(cx)
		speed := core.NewSignal(cx, 1.0)
		return form(
			ui.Slider{Value: speed.Get, OnChange: speed.Set, Min: 0.5, Max: 2, Step: 0.25, Label: tr("Speed", "速度"),
				ValueText: func(v float64) string { return fmt.Sprintf("%g×", v) }},
			ui.Textf(func() string { return fmt.Sprintf("%g×", speed.Get()) }, 13, ui.Regular, colors.Muted),
		)
	})
}
