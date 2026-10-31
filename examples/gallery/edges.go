package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"
)

// edgeMessages are the feed the edge effects demo scrolls.
var edgeMessages = []struct {
	from, line string
	tone       tone
}{
	{"Ada", "The engine notes are ready for review", toneAccent},
	{"Grace", "Compiler build passed on all six targets", toneUp},
	{"Linus", "Can we land the scheduler patch today?", toneInfo},
	{"Margaret", "Flight software checklist, revision 4", toneWarn},
	{"Alan", "Lunch at the usual place?", toneDanger},
	{"Barbara", "Slides for Thursday's talk", toneAccent},
	{"Dennis", "The new terminal font looks great", toneInfo},
	{"Katherine", "Trajectory numbers attached", toneUp},
	{"Ken", "Moving the meeting to 3 pm", toneWarn},
	{"Radia", "Spanning tree diagram, final", toneDanger},
}

// edgeEffects is a feed under a header and a toolbar, which its scroll
// view's edge effects set apart from the messages scrolling under them:
// iOS 26's own, elsewhere a blur that grows toward the edge, or frosts it.
func edgeEffects(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	effect := core.NewSignal(cx, core.EdgeEffectSoft)
	scroll := &core.Ref{}
	onTone := func() core.Color { return core.RGB(0xffffff) }
	var rows []core.Node
	for i := range 3 * len(edgeMessages) {
		m := edgeMessages[i%len(edgeMessages)]
		rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(12), CornerRadius: 14},
			StyleFunc: func() core.Style { return core.Style{Background: m.tone.color(cx)} },
			Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(36), Height: core.Pt(36), CornerRadius: 18, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
					StyleFunc: func() core.Style { return core.Style{Background: colors.surface()} },
					Children:  []core.Node{text(m.from[:1], 15, semibold, m.tone.shade(cx))}},
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
					text(m.from, 15, semibold, onTone),
					text(fmt.Sprintf("%s · %d", m.line, i+1), 13, regular, onTone),
				}},
			}})
	}
	bar := func(side core.Sides, children ...core.Node) core.Node {
		s := core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Direction: core.Row, AlignItems: core.AlignCenter,
			Gap: 10, Padding: core.Symmetric(10, 14)}
		if side == core.SideTop {
			s.Top = core.Pt(0)
		} else {
			s.Bottom, s.JustifyContent = core.Pt(0), core.JustifyCenter
		}
		return core.View{EdgeBar: core.EdgeBar{Scroll: scroll, Side: side}, Style: s, Children: children}
	}
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		core.View{Style: surfaceBox(cx, 380).Merge(core.Style{Overflow: core.Hidden}), StyleFunc: surfaceInk(cx), Children: []core.Node{
			// The scroll view is made again for each effect.
			core.Dynamic(func(*core.Scope) core.Node {
				e := effect.Get()
				return core.Scroll{Ref: scroll, TestID: "edge-scroll", EdgeEffects: core.EdgeEffects{Top: e, Bottom: e}, Style: core.Style{FlexGrow: 1},
					Children: []core.Node{core.View{Style: core.Style{Gap: 8, Padding: core.Edges{Top: 60, Right: 12, Bottom: 68, Left: 12}}, Children: rows}}}
			}),
			bar(core.SideTop,
				core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{text("Inbox", 22, semibold, colors.text)}},
				roundButton(cx, iconSearch, "Search", secondary, 36, func() {}),
			),
			bar(core.SideBottom,
				roundButton(cx, iconStar, "Starred", secondary, 40, func() {}),
				roundButton(cx, iconPlus, "New message", accentButton, 40, func() {}),
				roundButton(cx, iconBell, "Notifications", secondary, 40, func() {}),
			),
		}},
		segmented([]option[core.EdgeEffect]{{value: core.EdgeEffectSoft, label: "Soft"}, {value: core.EdgeEffectHard, label: "Hard"},
			{value: core.EdgeEffectHidden, label: "Hidden"}}, effect.Get, effect.Set, true),
	}}
}
