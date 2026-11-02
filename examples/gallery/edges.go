package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

// edgeMessages are the feed the edge effects demo scrolls.
var edgeMessages = []struct {
	from, line string
	tone       ui.Tone
}{
	{"Ada", "The engine notes are ready for review", ui.ToneAccent},
	{"Grace", "Compiler build passed on all six targets", ui.ToneUp},
	{"Linus", "Can we land the scheduler patch today?", ui.ToneInfo},
	{"Margaret", "Flight software checklist, revision 4", ui.ToneWarn},
	{"Alan", "Lunch at the usual place?", ui.ToneDanger},
	{"Barbara", "Slides for Thursday's talk", ui.ToneAccent},
	{"Dennis", "The new terminal font looks great", ui.ToneInfo},
	{"Katherine", "Trajectory numbers attached", ui.ToneUp},
	{"Ken", "Moving the meeting to 3 pm", ui.ToneWarn},
	{"Radia", "Spanning tree diagram, final", ui.ToneDanger},
}

// edgeEffects is a feed under a header and a toolbar, which its scroll
// view's edge effects set apart from the messages scrolling under them:
// iOS 26's own, elsewhere a blur that grows toward the edge, or frosts it.
func edgeEffects(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	effect := core.NewSignal(cx, core.EdgeEffectSoft)
	scroll := &core.Ref{}
	onTone := func() core.Color { return core.RGB(0xffffff) }
	var rows []core.Node
	for i := range 3 * len(edgeMessages) {
		m := edgeMessages[i%len(edgeMessages)]
		rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(12), CornerRadius: 14},
			StyleFunc: func() core.Style { return core.Style{Background: m.tone.Color(cx)} },
			Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(36), Height: core.Pt(36), CornerRadius: 18, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
					StyleFunc: func() core.Style { return core.Style{Background: colors.Surface()} },
					Children:  []core.Node{ui.Text(m.from[:1], 15, ui.Semibold, m.tone.Shade(cx))}},
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
					ui.Text(m.from, 15, ui.Semibold, onTone),
					ui.Text(fmt.Sprintf("%s · %d", m.line, i+1), 13, ui.Regular, onTone),
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
				core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{ui.Text(tr.EdgesInbox(), 22, ui.Semibold, colors.Text)}},
				ui.RoundButton{Icon: ui.IconSearch, Label: tr.EdgesSearch(), Variant: ui.ButtonSecondary, Size: 36, OnPress: func() {}},
			),
			bar(core.SideBottom,
				ui.RoundButton{Icon: ui.IconStar, Label: tr.EdgesStarred(), Variant: ui.ButtonSecondary, Size: 40, OnPress: func() {}},
				ui.RoundButton{Icon: ui.IconPlus, Label: tr.EdgesNewMessage(), Variant: ui.ButtonAccent, Size: 40, OnPress: func() {}},
				ui.RoundButton{Icon: ui.IconBell, Label: tr.EdgesNotifications(), Variant: ui.ButtonSecondary, Size: 40, OnPress: func() {}},
			),
		}},
		ui.Segmented[core.EdgeEffect]{Options: []ui.Option[core.EdgeEffect]{{Value: core.EdgeEffectSoft, Label: tr.EdgesSoft()}, {Value: core.EdgeEffectHard, Label: tr.EdgesHard()},
			{Value: core.EdgeEffectHidden, Label: tr.EdgesHidden()}}, Value: effect.Get, OnChange: effect.Set, Small: true},
	}}
}
