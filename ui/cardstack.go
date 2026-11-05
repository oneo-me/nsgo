package ui

import (
	"math"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// CardDecision is the side a card of a CardStack is sent to.
type CardDecision uint8

const (
	// CardLeft passes on a card: its left, quiet action.
	CardLeft CardDecision = iota + 1
	// CardRight keeps it: its right, main action.
	CardRight
)

// CardStackItem is one card of a CardStack.
type CardStackItem struct {
	// ID keeps the card's view while it rises through the stack.
	ID string
	// Label names the card for assistive technology.
	Label   string
	Content core.Node
}

// CardStack is a deck decided a card at a time: the top card follows a
// drag with a tilt and a stamp naming the side it nears, and a throw or
// the buttons send it off left or right while the cards behind rise.
// The app keeps the top card's Index and moves it on from OnDecide;
// moving it back brings a card back. Empty shows once every card is
// decided.
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
	cardTilt            = 15.0 // degrees at cardTiltDistance
	cardTiltDistance    = 240.0
	cardThrowDistance   = 100.0
	cardFlick           = 600.0 // points a second
	cardStampFrom       = 16.0
	cardStampTo         = 96.0
	cardThrowDuration   = 280 * time.Millisecond
	cardDefaultHeight   = 300
	cardDefaultMaxWidth = 320
)

// Render implements core.Node.
func (s CardStack) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	still := ReduceMotion(cx)
	x, y := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	width := core.NewSignal(cx, core.Points(cardDefaultMaxWidth))
	height := core.NewSignal(cx, core.Points(cardDefaultHeight))
	busy := core.NewSignal(cx, false)
	index := func() int {
		if s.Index == nil {
			return 0
		}
		return min(max(s.Index(), 0), len(s.Items))
	}
	done := func() bool { return index() >= len(s.Items) }
	// decide sends the top card to d's side, from a drag at vx or a
	// button, then reports it and puts the drag back for the next card.
	decide := func(d CardDecision, vx float64) {
		i := core.Untracked(cx, index)
		if i >= len(s.Items) || busy.GetUntracked() {
			return
		}
		report := func() {
			busy.Set(false)
			cx.Batch(func() {
				x.Set(0)
				y.Set(0)
				if s.OnDecide != nil {
					s.OnDecide(i, d)
				}
			})
		}
		if still {
			report()
			return
		}
		busy.Set(true)
		to := float64(width.GetUntracked()) + 160
		if d == CardLeft {
			to = -to
		}
		duration := cardThrowDuration
		if speed := math.Abs(vx); speed > cardFlick {
			duration = max(duration*time.Duration(cardFlick)/time.Duration(speed), 140*time.Millisecond)
		}
		cx.StartAnimation(core.Timing(x, to, core.TimingConfig{Duration: duration, Easing: core.BezierEasing(core.EaseOut)}),
			func(bool) { report() })
	}
	home := func() {
		cx.StartAnimation(core.Parallel(core.Spring(x, 0, core.SpringConfig{Stiffness: 260, Damping: 22}),
			core.Spring(y, 0, core.SpringConfig{Stiffness: 260, Damping: 22})), nil)
	}
	tilt := core.Interpolate(x, core.Interpolation{Input: []float64{-cardTiltDistance, 0, cardTiltDistance},
		Output: []float64{-cardTilt, 0, cardTilt}})
	stamp := func(side CardDecision) core.Node {
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
	visible := func() []CardStackItem {
		i := index()
		return s.Items[i:min(i+cardsShown, len(s.Items))]
	}
	cards := core.Each(visible, func(item CardStackItem) string { return item.ID },
		func(cx *core.Scope, item func() CardStackItem, depth func() int) core.Node {
			card := core.Untracked(cx, item)
			return core.View{Label: card.Label,
				Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0),
					Bottom: core.Pt(cardPeek * (cardsShown - 1))},
				StyleFunc: func() core.Style {
					// A card behind shrinks about its center and moves down
					// so its bottom peeks cardPeek below the one in front.
					d := core.Points(depth())
					s := core.Style{ZIndex: cardsShown - int(d), Transform: core.Transform{
						TranslateY: d*cardPeek + d*cardShrink*height.Get()/2, Scale: 1 - float32(d)*cardShrink}}
					if !still {
						s.Transition = core.Transition{Duration: 220 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionTransform}
					}
					if d == cardsShown-1 {
						s.Opacity = core.Alpha(0.6)
					}
					return s
				},
				Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1, Overflow: core.Hidden},
					StyleFunc: func() core.Style {
						s := CardStyle(cx).Merge(core.Style{Color: colors.Text(), Shadows: FinOf(cx).Raised().Shadows})
						if depth() == 0 {
							s.Shadows = FinOf(cx).Menu().Shadows
							s.Animated = &core.AnimatedStyle{TranslateX: x, TranslateY: y, Rotate: tilt}
						}
						return s
					},
					Children: []core.Node{card.Content, core.Show{When: func() bool { return depth() == 0 },
						Then: func(cx *core.Scope) core.Node {
							return core.Group{stamp(CardRight), stamp(CardLeft)}
						}}}}}}
		})
	stage := core.View{Label: s.Label, TestID: s.TestID,
		AccessibilityActions: []core.AccessibilityAction{{Name: "left", Label: s.LeftLabel}, {Name: "right", Label: s.RightLabel}},
		OnAccessibilityAction: func(name string) {
			if name == "left" {
				decide(CardLeft, 0)
			} else if name == "right" {
				decide(CardRight, 0)
			}
		},
		PanValues: core.AnimatedXY{X: x, Y: y}, PanActivation: core.PanActivation{ActiveOffsetX: 6, FailOffsetY: 24},
		OnPan: func(e core.PanEvent) {
			if e.Phase != core.GestureEnded && e.Phase != core.GestureCancelled {
				return
			}
			dx, vx := float64(e.Values.X), float64(e.Velocity.X)
			switch {
			case e.Phase == core.GestureCancelled || core.Untracked(cx, done):
				home()
			case dx > cardThrowDistance && vx > -cardFlick, vx > cardFlick && dx > 0:
				decide(CardRight, vx)
			case dx < -cardThrowDistance && vx < cardFlick, vx < -cardFlick && dx < 0:
				decide(CardLeft, vx)
			default:
				home()
			}
		},
		OnLayout: func(b core.Bounds) {
			width.Set(b.Size.Width)
			height.Set(b.Size.Height - cardPeek*(cardsShown-1))
		},
		Style: core.Style{AlignSelf: core.AlignCenter, Width: core.Pct(100), MaxWidth: core.Pt(cardDefaultMaxWidth),
			Height: core.Pt(cardDefaultHeight + cardPeek*(cardsShown-1))}.Merge(s.Style),
		Children: []core.Node{cards, core.Show{When: done, Then: func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				Children: []core.Node{s.Empty}}
		}}}}
	off := func() bool { return done() || busy.Get() }
	buttons := core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifyCenter, Gap: 12},
		Children: []core.Node{
			Button{Variant: ButtonGhost, Icon: lucide.X, Text: s.LeftLabel, Disabled: off, OnPress: func() { decide(CardLeft, 0) }},
			Button{Variant: ButtonAccent, Icon: lucide.Check, Text: s.RightLabel, Disabled: off, OnPress: func() { decide(CardRight, 0) }},
		}}
	return core.View{Style: core.Style{Gap: 16}, Children: []core.Node{stage, buttons}}
}
