package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// CardDecision is the side a card of a CardStack is sent to.
type CardDecision = base.CardDecision

const (
	// CardLeft passes on a card: its left, quiet action.
	CardLeft = base.CardLeft
	// CardRight keeps it: its right, main action.
	CardRight = base.CardRight
)

// CardStackItem is one card of a CardStack.
type CardStackItem = base.CardStackItem

// CardStack is base.CardStack on elevated cards, with a stamp naming the
// side a dragged card nears and buttons that send it off.
type CardStack struct {
	Items []CardStackItem
	Index func() int
	// OnDecide receives the index of the card sent off and its side.
	OnDecide func(index int, d CardDecision)
	// LeftLabel and RightLabel name the sides' actions, on the buttons,
	// the stamps and to assistive technology.
	LeftLabel, RightLabel string
	Empty                 core.Node
	// Label names the deck for assistive technology.
	Label  string
	TestID string
	// Style merges over the stage's, 320 points wide at most.
	Style core.Style
}

// The stack's geometry: each card behind peeks cardPeek below the one in
// front and is cardShrink smaller.
const (
	cardsShown          = 3
	cardPeek            = 10
	cardShrink          = 0.05
	cardStampFrom       = 16.0
	cardStampTo         = 96.0
	cardDefaultHeight   = 300
	cardDefaultMaxWidth = 320
)

// Render implements core.Node.
func (s CardStack) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	stamp := func(x *core.AnimatedValue, side CardDecision) core.Node {
		label, tint := s.RightLabel, colors.Up
		input, output := []float64{cardStampFrom, cardStampTo}, []float64{0, 1}
		pos := core.Style{Left: core.Pt(16), Transform: core.Transform{Rotate: -12}}
		if side == CardLeft {
			label, tint = s.LeftLabel, colors.Danger
			input, output = []float64{-cardStampTo, -cardStampFrom}, []float64{1, 0}
			pos = core.Style{Right: core.Pt(16), Transform: core.Transform{Rotate: 12}}
		}
		opacity := core.Interpolate(x, core.Interpolation{Input: input, Output: output, Extrapolate: core.ExtrapolateClamp})
		return core.View{PointerEvents: core.PointerEventsNone, Style: TypeTitle.Merge(core.Style{Position: core.Absolute,
			Top: core.Pt(16), Padding: core.Symmetric(2, 8), BorderWidth: 2, CornerRadius: RadiusControl,
			Animated: &core.AnimatedStyle{Opacity: opacity}}).Merge(pos),
			StyleFunc: func() core.Style {
				return core.Style{BorderColor: tint(), Color: tint(), Background: colors.Surface().Faded(0.85)}
			},
			Children: []core.Node{line(label, core.Style{}, 1)}}
	}
	ref := new(base.CardStackRef)
	stage := base.CardStack{Items: s.Items, Index: s.Index, OnDecide: s.OnDecide, LeftLabel: s.LeftLabel, RightLabel: s.RightLabel, Empty: s.Empty,
		Shown: cardsShown, Peek: cardPeek, Shrink: cardShrink, Ref: ref, Label: s.Label, TestID: s.TestID,
		CardLook: func(depth int) core.Style {
			st := CardStyle(cx).Merge(core.Style{Color: colors.Text(), Shadows: FinOf(cx).Raised().Shadows})
			if depth == 0 {
				st.Shadows = FinOf(cx).Menu().Shadows
			}
			return st
		},
		Overlay: func(x *core.AnimatedValue) core.Node { return core.Group{stamp(x, CardRight), stamp(x, CardLeft)} },
		Style: core.Style{AlignSelf: core.AlignCenter, Width: core.Pct(100), MaxWidth: core.Pt(cardDefaultMaxWidth),
			Height: core.Pt(cardDefaultHeight + cardPeek*(cardsShown-1))}.Merge(s.Style)}
	buttons := core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifyCenter, Gap: 12},
		Children: []core.Node{
			Button{Variant: ButtonGhost, Icon: lucide.X, Text: s.LeftLabel, Disabled: ref.Off, OnPress: func() { ref.Decide(CardLeft) }},
			Button{Variant: ButtonAccent, Icon: lucide.Check, Text: s.RightLabel, Disabled: ref.Off, OnPress: func() { ref.Decide(CardRight) }},
		}}
	return core.View{Style: core.Style{Gap: 16}, Children: []core.Node{stage, buttons}}
}
