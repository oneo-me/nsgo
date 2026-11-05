package main

import (
	"fmt"
	"github.com/oneo-me/nsgo/icons/lucide"
	"math"

	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

type side string

// buyingPower is the simulated buying power.
const buyingPower = 250_000

var quantities = []float64{100, 500, 1000, 5000}

// banner is a confirmation that drops in from the top and leaves by
// itself.
type banner struct {
	show func(title, detail string, s side)
	view core.Node
}

func newBanner(cx *core.Scope) banner {
	colors := ui.ColorsOf(cx)
	t := core.NewAnimatedValue(cx, 0)
	type msg struct {
		title, detail string
		side          side
	}
	message := core.NewSignal(cx, msg{side: "buy"})
	show := func(title, detail string, s side) {
		message.Set(msg{title, detail, s})
		cx.StartAnimation(core.Sequence(
			core.Spring(t, 1, core.SpringConfig{Stiffness: 260, Damping: 22}),
			core.Delay(ms(2200)),
			core.Timing(t, 0, core.TimingConfig{Duration: ms(260), Easing: ease(core.EasingCubic, core.EaseModeIn)}),
		), nil)
	}
	check := func() core.Color {
		if cx.Dark() {
			return core.RGB(0x1f1d1a)
		}
		return core.RGB(0xffffff)
	}
	detailColor := func() core.Color {
		if cx.Dark() {
			return core.RGB(0x5a554e)
		}
		return core.RGB(0xa8a39a)
	}
	view := core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(10), Left: core.Pt(16), Right: core.Pt(16), Direction: core.Row,
		AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(14), CornerRadius: ui.RadiusInset + 3, BorderCurve: core.BorderCurveContinuous, Opacity: core.Alpha(0.001),
		Animated: &core.AnimatedStyle{
			Opacity:    core.Interpolate(t, core.Interpolation{Input: []float64{0, 0.3, 1}, Output: []float64{0, 1, 1}}),
			TranslateY: ui.Lerp(t, -90, 0), Scale: ui.Lerp(t, 0.92, 1),
		}}, StyleFunc: ui.Filled(colors.Light), Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			c := colors.Up
			if message.Get().side == "sell" {
				c = colors.Down
			}
			return core.Style{Width: core.Pt(34), Height: core.Pt(34), CornerRadius: 17, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: c()}
		}, Children: []core.Node{ui.Icon(lucide.Check, 18, check)}},
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
			ui.Textf(func() string { return message.Get().title }, 13, ui.Semibold, colors.OnLight),
			core.Text{ContentFunc: func() string { return message.Get().detail }, MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(detailColor)},
		}},
	}}
	return banner{show: show, view: view}
}

// ticketLine is a line of the review: a label and a value.
func ticketLine(cx *core.Scope, lbl string, value func() string, strong bool) core.Node {
	colors := ui.ColorsOf(cx)
	size, weight := core.Points(14), ui.Regular
	if strong {
		size, weight = 17, ui.Semibold
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, MinHeight: core.Pt(24)}, Children: []core.Node{
		core.Text{Content: lbl, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
		ui.Tabular(ui.Textf(value, size, weight, colors.Text)),
	}}
}

// ticket is the order ticket, the content of a sheet: side, a limit
// price stepper, quantity chips and the estimated cost; reviewing expands
// the ticket and placing calls onPlaced. Nothing is sent anywhere.
func ticket(cx *core.Scope, open func() bool, s *stock, sd *core.Signal[side], onPlaced func(title, detail string, s side)) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	fin := ui.FinOf(cx)
	limit := core.NewSignal(cx, core.Untracked(cx, s.quote.Get).last)
	quantity := core.NewSignal(cx, 500.0)
	review := core.NewSignal(cx, false)
	// Each opening starts from the last price, not reviewing.
	cx.Effect(func() {
		if !open() {
			return
		}
		cx.Untrack(func() {
			limit.Set(s.quote.GetUntracked().last)
			review.Set(false)
		})
	})
	p := func(v float64) string { return priceText(v, s) }
	cost := func() float64 { return limit.Get() * quantity.Get() }
	fee := func() float64 { return math.Max(1, cost()*0.0003) }
	expand := func(o bool) {
		cx.AnimateLayout(core.LayoutSpringPreset)
		review.Set(o)
	}
	nudge := func(steps float64) {
		limit.Set(math.Max(s.tick, math.Round(limit.GetUntracked()/s.tick+steps)*s.tick))
	}
	tone := func() core.Color {
		if sd.Get() == "buy" {
			return colors.Up()
		}
		return colors.Down()
	}
	verb := func() string {
		if sd.Get() == "buy" {
			return tr.StocksBuy()
		}
		return tr.StocksSell()
	}
	stepper := func(name func() *core.SVG, lbl string, by float64) core.Node {
		return ui.Press{OnPress: func() { nudge(by) }, Label: lbl, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				return fin.Pill().Over(core.Style{Width: core.Pt(44), Height: core.Pt(44), CornerRadius: ui.RadiusControl, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, BorderWidth: 1})
			},
				Children: []core.Node{ui.Icon(name, 18, colors.Text)}},
		}}
	}
	var chips []core.Node
	for _, q := range quantities {
		on := func() bool { return quantity.Get() == q }
		lbl := fmt.Sprintf("%.0f", q)
		if q >= 1000 {
			lbl = fmt.Sprintf("%.0fK", q/1000)
		}
		chips = append(chips, ui.Cell{Child: ui.Press{OnPress: func() { quantity.Set(q) }, Label: tr.StocksShares(int(q)), Selected: on, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				s := core.Style{Height: core.Pt(44), CornerRadius: ui.RadiusControl, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, BorderWidth: 1,
					Background: colors.Raised(), BorderColor: colors.Border()}
				if on() {
					s.Background, s.BorderColor = tints.Accent(), colors.Accent()
				}
				return s
			}, Children: []core.Node{core.Text{Content: lbl, StyleFunc: func() core.Style {
				c := colors.Text
				if on() {
					c = colors.Accent
				}
				return core.Style{FontSize: 13, FontWeight: ui.Semibold, Color: c()}
			}}}},
		}}})
	}
	return core.View{Style: core.Style{Gap: 16, Padding: core.Edges{Top: 4}}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
			core.Text{ContentFunc: func() string { return verb() + " " + s.code }, Style: core.Style{FlexGrow: 1, FontSize: 20, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)},
			ui.Tabular(ui.Text(tr.StocksSimulatedNothingIsSent(), 12, ui.Regular, colors.Faint)),
		}},
		ui.Segmented[side]{Options: []ui.Option[side]{{Value: "buy", Label: tr.StocksBuy()}, {Value: "sell", Label: tr.StocksSell()}}, Value: sd.Get, OnChange: sd.Set},
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline}, Children: []core.Node{
				core.Text{Content: tr.StocksLimitPrice(), Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
				core.Text{ContentFunc: func() string { return tr.StocksUseLast() + p(s.quote.Get().last) }, Role: core.RoleButton,
					OnPress: func(core.PressEvent) { limit.Set(s.quote.GetUntracked().last) }, Style: core.Style{FontSize: 13, FontVariant: core.FontTabularNums}, StyleFunc: ui.Colored(colors.Accent)},
			}},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
				stepper(lucide.Minus, tr.StocksLowerThePrice(), -1),
				core.View{StyleFunc: func() core.Style {
					return core.Style{FlexGrow: 1, Height: core.Pt(44), CornerRadius: ui.RadiusControl, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
						Background: colors.Inset(), BorderWidth: 1, BorderColor: colors.Border()}
				}, Children: []core.Node{ui.Textf(func() string { return p(limit.Get()) }, 20, ui.Semibold, colors.Text)}},
				stepper(lucide.Plus, tr.StocksRaiseThePrice(), 1),
			}},
		}},
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline}, Children: []core.Node{
				core.Text{Content: tr.StocksQuantity(), Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
				ui.Textf(func() string { return grouped(quantity.Get(), 0) + tr.StocksSh() }, 13, ui.Regular, colors.Faint),
			}},
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: chips},
		}},
		core.View{Style: core.Style{Gap: 6, Padding: core.Edges{Top: 2}}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline}, Children: []core.Node{
				core.Text{ContentFunc: func() string {
					if sd.Get() == "buy" {
						return tr.StocksEstimatedCost()
					}
					return tr.StocksEstimatedProceeds()
				}, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
				ui.Textf(func() string { return grouped(cost(), 2) }, 22, ui.Semibold, colors.Text),
			}},
			core.View{Style: core.Style{Height: core.Pt(6), CornerRadius: 3, Overflow: core.Hidden}, StyleFunc: ui.Filled(colors.Raised), Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					c := tone()
					if cost() > buyingPower {
						c = colors.Danger()
					}
					return core.Style{Width: core.Pct(float32(math.Min(100, cost()/buyingPower*100))), Height: core.Pt(6), CornerRadius: 3, Background: c}
				}},
			}},
			ui.Textf(func() string {
				return tr.StocksOfBuyingPower(fmt.Sprintf("%.1f%%", cost()/buyingPower*100), grouped(buyingPower, 0))
			}, 12, ui.Regular, colors.Faint),
		}},
		core.Show{When: review.Get,
			Else: func(*core.Scope) core.Node {
				return core.Dynamic(func(*core.Scope) core.Node {
					v := verb()
					return ui.Button{Text: tr.StocksReview() + lower(v), OnPress: func() { expand(true) }}
				})
			},
			Then: func(*core.Scope) core.Node {
				return core.View{StyleFunc: func() core.Style {
					bg := tints.Up
					if sd.Get() == "sell" {
						bg = tints.Down
					}
					return core.Style{Gap: 12, Padding: core.All(14), CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, BorderColor: tone(), Background: bg()}
				}, Children: []core.Node{
					core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
						core.Text{ContentFunc: func() string { return fmt.Sprintf("%s %s %s", verb(), grouped(quantity.Get(), 0), s.code) },
							StyleFunc: func() core.Style {
								return core.Style{FlexGrow: 1, FontSize: 13, FontWeight: ui.Semibold, Color: tone()}
							}},
						ui.Text(tr.StocksLimitDay(), 12, ui.Regular, colors.Muted),
					}},
					core.View{Style: core.Style{Gap: 2}, Children: []core.Node{
						ticketLine(cx, tr.StocksLimit(), func() string { return p(limit.Get()) }, false),
						ticketLine(cx, tr.StocksFeesEst(), func() string { return grouped(fee(), 2) }, false),
						ticketLine(cx, tr.StocksTotal2(), func() string {
							if sd.Get() == "buy" {
								return grouped(cost()+fee(), 2)
							}
							return grouped(cost()-fee(), 2)
						}, true),
					}},
					core.View{Style: core.Style{Direction: core.Row, Gap: 10}, Children: []core.Node{
						ui.Button{Text: tr.StocksEdit(), Variant: ui.ButtonSecondary, Flex: 1, OnPress: func() { expand(false) }},
						core.View{Style: core.Style{FlexGrow: 2, FlexShrink: 2, FlexBasis: core.Pt(0)}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
							v := verb()
							return ui.Button{Text: tr.StocksPlace() + lower(v) + tr.StocksOrder(), OnPress: func() {
								onPlaced(v+tr.StocksOrderPlaced(), tr.StocksPlacedDetail(grouped(quantity.GetUntracked(), 0), s.code, p(limit.GetUntracked())), sd.GetUntracked())
								expand(false)
							}}
						})}},
					}},
				}}
			}},
	}}
}

func lower(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]|0x20) + s[1:]
}
