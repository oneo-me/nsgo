package main

import (
	"fmt"
	"math"

	"github.com/oneo-me/nsgo/core"
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
	colors := colorsOf(cx)
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
		AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(14), CornerRadius: radiusInset + 3, BorderCurve: core.BorderCurveContinuous, Opacity: core.Alpha(0.001),
		Animated: &core.AnimatedStyle{
			Opacity:    core.Interpolate(t, core.Interpolation{Input: []float64{0, 0.3, 1}, Output: []float64{0, 1, 1}}),
			TranslateY: lerp(t, -90, 0), Scale: lerp(t, 0.92, 1),
		}}, StyleFunc: filled(colors.light), Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			c := colors.up
			if message.Get().side == "sell" {
				c = colors.down
			}
			return core.Style{Width: core.Pt(34), Height: core.Pt(34), CornerRadius: 17, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: c()}
		}, Children: []core.Node{icon(iconCheck, 18, check)}},
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
			textf(func() string { return message.Get().title }, 15, semibold, colors.onLight),
			core.Text{ContentFunc: func() string { return message.Get().detail }, MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: colored(detailColor)},
		}},
	}}
	return banner{show: show, view: view}
}

// ticketLine is a line of the review: a label and a value.
func ticketLine(cx *core.Scope, lbl string, value func() string, strong bool) core.Node {
	colors := colorsOf(cx)
	size, weight := core.Points(14), regular
	if strong {
		size, weight = 17, semibold
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, MinHeight: core.Pt(24)}, Children: []core.Node{
		core.Text{Content: lbl, Style: core.Style{FlexGrow: 1, FontSize: 14}, StyleFunc: colored(colors.muted)},
		tabular(textf(value, size, weight, colors.text)),
	}}
}

// ticket is the order ticket, the content of a sheet: side, a limit
// price stepper, quantity chips and the estimated cost; reviewing expands
// the ticket and placing calls onPlaced. Nothing is sent anywhere.
func ticket(cx *core.Scope, open func() bool, s *stock, sd *core.Signal[side], onPlaced func(title, detail string, s side)) core.Node {
	colors := colorsOf(cx)
	tints := tintsOf(cx)
	fin := finOf(cx)
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
			return colors.up()
		}
		return colors.down()
	}
	verb := func() string {
		if sd.Get() == "buy" {
			return "Buy"
		}
		return "Sell"
	}
	stepper := func(name iconName, lbl string, by float64) core.Node {
		return press(pressProps{onPress: func() { nudge(by) }, label: lbl},
			core.View{StyleFunc: func() core.Style {
				return fin.pill().over(core.Style{Width: core.Pt(44), Height: core.Pt(44), CornerRadius: radiusControl, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, BorderWidth: 1})
			},
				Children: []core.Node{icon(name, 18, colors.text)}})
	}
	var chips []core.Node
	for _, q := range quantities {
		on := func() bool { return quantity.Get() == q }
		lbl := fmt.Sprintf("%.0f", q)
		if q >= 1000 {
			lbl = fmt.Sprintf("%.0fK", q/1000)
		}
		chips = append(chips, cell(press(pressProps{onPress: func() { quantity.Set(q) }, label: fmt.Sprintf("%.0f shares", q), selected: on},
			core.View{StyleFunc: func() core.Style {
				s := core.Style{Height: core.Pt(44), CornerRadius: radiusControl, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, BorderWidth: 1,
					Background: colors.raised(), BorderColor: colors.border()}
				if on() {
					s.Background, s.BorderColor = tints.accent(), colors.accent()
				}
				return s
			}, Children: []core.Node{core.Text{Content: lbl, StyleFunc: func() core.Style {
				c := colors.text
				if on() {
					c = colors.accent
				}
				return core.Style{FontSize: 14, FontWeight: semibold, Color: c()}
			}}}})))
	}
	return core.View{Style: core.Style{Gap: 16, Padding: core.Edges{Top: 4}}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
			core.Text{ContentFunc: func() string { return verb() + " " + s.code }, Style: core.Style{FlexGrow: 1, FontSize: 20, FontWeight: semibold}, StyleFunc: colored(colors.text)},
			tabular(text("simulated · nothing is sent", 12, regular, colors.faint)),
		}},
		segmented([]option[side]{{value: "buy", label: "Buy"}, {value: "sell", label: "Sell"}}, sd.Get, sd.Set, false),
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline}, Children: []core.Node{
				core.Text{Content: "Limit price", Style: core.Style{FlexGrow: 1, FontSize: 14}, StyleFunc: colored(colors.muted)},
				core.Text{ContentFunc: func() string { return "use last " + p(s.quote.Get().last) }, Role: core.RoleButton,
					OnPress: func(core.PressEvent) { limit.Set(s.quote.GetUntracked().last) }, Style: core.Style{FontSize: 13, FontVariant: core.FontTabularNums}, StyleFunc: colored(colors.accent)},
			}},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
				stepper(iconMinus, "Lower the price", -1),
				core.View{StyleFunc: func() core.Style {
					return core.Style{FlexGrow: 1, Height: core.Pt(44), CornerRadius: radiusControl, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
						Background: colors.inset(), BorderWidth: 1, BorderColor: colors.border()}
				}, Children: []core.Node{textf(func() string { return p(limit.Get()) }, 20, semibold, colors.text)}},
				stepper(iconPlus, "Raise the price", 1),
			}},
		}},
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline}, Children: []core.Node{
				core.Text{Content: "Quantity", Style: core.Style{FlexGrow: 1, FontSize: 14}, StyleFunc: colored(colors.muted)},
				textf(func() string { return grouped(quantity.Get(), 0) + " sh" }, 13, regular, colors.faint),
			}},
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: chips},
		}},
		core.View{Style: core.Style{Gap: 6, Padding: core.Edges{Top: 2}}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline}, Children: []core.Node{
				core.Text{ContentFunc: func() string {
					if sd.Get() == "buy" {
						return "Estimated cost"
					}
					return "Estimated proceeds"
				}, Style: core.Style{FlexGrow: 1, FontSize: 14}, StyleFunc: colored(colors.muted)},
				textf(func() string { return grouped(cost(), 2) }, 22, semibold, colors.text),
			}},
			core.View{Style: core.Style{Height: core.Pt(6), CornerRadius: 3, Overflow: core.Hidden}, StyleFunc: filled(colors.raised), Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					c := tone()
					if cost() > buyingPower {
						c = colors.danger()
					}
					return core.Style{Width: core.Pct(float32(math.Min(100, cost()/buyingPower*100))), Height: core.Pt(6), CornerRadius: 3, Background: c}
				}},
			}},
			textf(func() string {
				return fmt.Sprintf("%.1f%% of %s buying power", cost()/buyingPower*100, grouped(buyingPower, 0))
			}, 12, regular, colors.faint),
		}},
		core.Show{When: review.Get,
			Else: func(*core.Scope) core.Node {
				return core.Dynamic(func(*core.Scope) core.Node {
					v := verb()
					return button(buttonProps{label: "Review " + lower(v)}, func() { expand(true) })
				})
			},
			Then: func(*core.Scope) core.Node {
				return core.View{StyleFunc: func() core.Style {
					bg := tints.up
					if sd.Get() == "sell" {
						bg = tints.down
					}
					return core.Style{Gap: 12, Padding: core.All(14), CornerRadius: radiusInset, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, BorderColor: tone(), Background: bg()}
				}, Children: []core.Node{
					core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
						core.Text{ContentFunc: func() string { return fmt.Sprintf("%s %s %s", verb(), grouped(quantity.Get(), 0), s.code) },
							StyleFunc: func() core.Style {
								return core.Style{FlexGrow: 1, FontSize: 15, FontWeight: semibold, Color: tone()}
							}},
						text("LIMIT · DAY", 12, regular, colors.muted),
					}},
					core.View{Style: core.Style{Gap: 2}, Children: []core.Node{
						ticketLine(cx, "Limit", func() string { return p(limit.Get()) }, false),
						ticketLine(cx, "Fees (est.)", func() string { return grouped(fee(), 2) }, false),
						ticketLine(cx, "Total", func() string {
							if sd.Get() == "buy" {
								return grouped(cost()+fee(), 2)
							}
							return grouped(cost()-fee(), 2)
						}, true),
					}},
					core.View{Style: core.Style{Direction: core.Row, Gap: 10}, Children: []core.Node{
						button(buttonProps{label: "Edit", variant: secondary, flex: 1}, func() { expand(false) }),
						core.View{Style: core.Style{FlexGrow: 2, FlexShrink: 2, FlexBasis: core.Pt(0)}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
							v := verb()
							return button(buttonProps{label: "Place " + lower(v) + " order"}, func() {
								onPlaced(v+" order placed", fmt.Sprintf("%s %s at %s · simulated", grouped(quantity.GetUntracked(), 0), s.code, p(limit.GetUntracked())), sd.GetUntracked())
								expand(false)
							})
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
