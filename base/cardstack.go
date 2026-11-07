package base

import (
	"math"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// CardDecision is the side a card of a CardStack is sent to.
type CardDecision uint8

const (
	// CardLeft passes on a card.
	CardLeft CardDecision = iota + 1
	// CardRight keeps it.
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

// The stack's motion: a tilt at cardTiltDistance, what a drag or a fling
// must reach to send a card off, and the send's time.
const (
	cardTilt          = 15.0 // degrees
	cardTiltDistance  = 240.0
	cardThrowDistance = 100.0
	cardFlick         = 600.0 // points a second
	cardThrowDuration = 280 * time.Millisecond
)

// CardStack is a deck decided a card at a time: the top card follows a drag
// with a tilt, and a throw, its ref or assistive technology's actions send
// it off left or right while the cards behind rise. The app keeps the top
// card's Index and moves it on from OnDecide; moving it back brings a card
// back. Empty shows once every card is decided. The app sizes the stage and
// styles the cards.
type CardStack struct {
	Items []CardStackItem
	Index func() int
	// OnDecide receives the index of the card sent off and its side.
	OnDecide func(index int, d CardDecision)
	// LeftLabel and RightLabel name the sides' actions for assistive
	// technology.
	LeftLabel, RightLabel string
	Empty                 core.Node
	// Shown is how many cards show, 3 when 0; each behind peeks Peek points
	// below the one in front and is Shrink smaller.
	Shown  int
	Peek   core.Points
	Shrink float32
	// CardLook styles the card depth behind the top one, and Overlay
	// shows over the top card, x being its drag along the row.
	CardLook func(depth int) core.Style
	Overlay  func(x *core.AnimatedValue) core.Node
	// Ref's Decide sends the top card off, as a throw does.
	Ref       *CardStackRef
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Label     string
	TestID    string
	// Tooltip is its view's, as a core.View's.
	Tooltip core.Node
}

// CardStackRef is a CardStack's ref.
type CardStackRef struct {
	decide func(CardDecision)
	off    func() bool
}

// Decide sends the top card to d's side.
func (r *CardStackRef) Decide(d CardDecision) {
	if r.decide != nil {
		r.decide(d)
	}
}

// Off reports that no card can be sent now: none is left, or one is on its
// way.
func (r *CardStackRef) Off() bool { return r.off == nil || r.off() }

// Render implements core.Node.
func (s CardStack) Render(cx *core.Scope) core.Node {
	env := cx.Env()
	still := core.Untracked(cx, env.ReduceMotion)
	shownCards := s.Shown
	if shownCards <= 0 {
		shownCards = 3
	}
	x, y := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	width, height := core.NewSignal[core.Points](cx, 0), core.NewSignal[core.Points](cx, 0)
	busy := core.NewSignal(cx, false)
	index := func() int {
		if s.Index == nil {
			return 0
		}
		return min(max(s.Index(), 0), len(s.Items))
	}
	done := func() bool { return index() >= len(s.Items) }
	// decide sends the top card to d's side, from a drag at vx or the app,
	// then reports it and puts the drag back for the next card.
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
	if s.Ref != nil {
		s.Ref.decide = func(d CardDecision) { decide(d, 0) }
		s.Ref.off = func() bool { return done() || busy.Get() }
	}
	home := func() {
		cx.StartAnimation(core.Parallel(core.Spring(x, 0, core.SpringConfig{Stiffness: 260, Damping: 22}),
			core.Spring(y, 0, core.SpringConfig{Stiffness: 260, Damping: 22})), nil)
	}
	tilt := core.Interpolate(x, core.Interpolation{Input: []float64{-cardTiltDistance, 0, cardTiltDistance},
		Output: []float64{-cardTilt, 0, cardTilt}})
	visible := func() []CardStackItem {
		i := index()
		return s.Items[i:min(i+shownCards, len(s.Items))]
	}
	room := s.Peek * core.Points(shownCards-1)
	cards := core.Each(visible, func(item CardStackItem) string { return item.ID },
		func(cx *core.Scope, item func() CardStackItem, depth func() int) core.Node {
			card := core.Untracked(cx, item)
			return core.View{Label: card.Label,
				Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(room)},
				StyleFunc: func() core.Style {
					// A card behind shrinks about its center and moves down
					// so its bottom peeks below the one in front.
					d := core.Points(depth())
					st := core.Style{ZIndex: shownCards - int(d), Transform: core.Transform{
						TranslateY: d*s.Peek + d*core.Points(s.Shrink)*height.Get()/2, Scale: 1 - float32(d)*s.Shrink}}
					if !still {
						st.Transition = core.Transition{Duration: 220 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionTransform}
					}
					if int(d) == shownCards-1 {
						st.Opacity = core.Alpha(0.6)
					}
					return st
				},
				Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1, Overflow: core.Hidden},
					StyleFunc: func() core.Style {
						var st core.Style
						if s.CardLook != nil {
							st = s.CardLook(depth())
						}
						if depth() == 0 {
							st.Animated = &core.AnimatedStyle{TranslateX: x, TranslateY: y, Rotate: tilt}
						}
						return st
					},
					Children: []core.Node{card.Content, core.Show{When: func() bool { return depth() == 0 },
						Then: func(*core.Scope) core.Node {
							if s.Overlay == nil {
								return nil
							}
							return s.Overlay(x)
						}}}}}}
		})
	return core.View{Class: s.Class, Label: s.Label, TestID: s.TestID, Tooltip: s.Tooltip, Style: s.Style, StyleFunc: s.StyleFunc,
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
			height.Set(b.Size.Height - room)
		},
		Children: []core.Node{cards, core.Show{When: done, Then: func(*core.Scope) core.Node {
			return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				Children: []core.Node{s.Empty}}
		}}}}
}
