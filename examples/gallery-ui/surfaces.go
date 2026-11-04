package ui

import (
	"math"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Card is a demo's container: a header on the page of a bold Title, a
// Description and Actions at its end, then a flat bordered surface holding
// the Children and an optional Footer.
type Card struct {
	Title       string
	Description string
	// Actions end the header (a switch, a small button, a value).
	Actions  []core.Node
	Children []core.Node
	// Footer follows the children on its own line: a status or a caption.
	Footer []core.Node
	// Flush lays the children edge to edge, without the surface's padding
	// and gaps (rows, a full-bleed stage).
	Flush bool
	// OnPress makes the surface a button named Label, else Title, which
	// takes the presses of what it holds.
	OnPress func()
	Label   string
	TestID  string
	// Style merges over the surface's.
	Style core.Style
}

// CardStyle is a card's surface, flat as Onevra's are, read in a
// StyleFunc.
func CardStyle(cx *core.Scope) core.Style {
	colors := ColorsOf(cx)
	return core.Style{Background: colors.Surface(), CornerRadius: RadiusCard, BorderCurve: core.BorderCurveContinuous,
		BorderWidth: 1, BorderColor: colors.Border()}
}

// Render implements core.Node.
func (c Card) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var head core.Node
	if c.Title != "" {
		title := Text(c.Title, 13, Bold, colors.Text)
		title.Role = core.RoleHeader
		titles := []core.Node{title}
		if c.Description != "" {
			description := Text(c.Description, 12, Regular, colors.Muted)
			description.MaxLines = 2
			titles = append(titles, description)
		}
		nodes := append([]core.Node{core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: titles}}, c.Actions...)
		head = core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd, Gap: 10, Padding: core.Symmetric(0, 2)}, Children: nodes}
	}
	label := c.Label
	if label == "" {
		label = c.Title
	}
	return panel(cx, panelProps{testID: c.TestID, head: head, flush: c.Flush, children: c.Children, footer: c.Footer, style: c.Style,
		onPress: c.OnPress, label: label})
}

type panelProps struct {
	testID   string
	head     core.Node
	flush    bool
	children []core.Node
	footer   []core.Node
	style    core.Style
	onPress  func()
	label    string
}

// panel is a card: the head on the page, then the surface with the body
// and the footer after a hairline. A flush body clips to the surface's
// inner corners.
func panel(cx *core.Scope, p panelProps) core.Node {
	colors := ColorsOf(cx)
	const inner = RadiusCard - 1
	var parts []core.Node
	if len(p.children) > 0 {
		body := core.Style{Gap: 12, Padding: core.All(14)}
		if p.flush {
			r := core.Corners{TopLeft: inner, TopRight: inner}
			if len(p.footer) == 0 {
				r.BottomLeft, r.BottomRight = inner, inner
			}
			body = core.Style{Overflow: core.Hidden, CornerRadii: r, BorderCurve: core.BorderCurveContinuous}
		}
		parts = append(parts, core.View{Style: body, Children: p.children})
	}
	if len(p.footer) > 0 {
		s := core.Style{Gap: 6, Padding: core.Symmetric(10, 14)}
		if len(parts) > 0 {
			s.Borders = core.Edges{Top: 1}
		}
		parts = append(parts, core.View{Style: s,
			StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}} }, Children: p.footer})
	}
	surface := core.View{StyleFunc: func() core.Style { return CardStyle(cx).Merge(p.style) }, Children: parts}
	if p.onPress != nil {
		surface.Role, surface.Label, surface.OnPress = core.RoleButton, p.label, func(core.PressEvent) { p.onPress() }
		surface.Style = core.Style{Cursor: core.CursorPointer, Transition: quick}
		surface.StyleFunc = func() core.Style {
			colors := ColorsOf(cx)
			return CardStyle(cx).Merge(core.Style{Hover: &core.Style{BorderColor: colors.FieldBorder()},
				Active: &core.Style{Background: colors.Raised()}, Focus: FocusRing(cx, 2)}).Merge(p.style)
		}
	}
	if p.head == nil {
		surface.TestID = p.testID
		return surface
	}
	return core.View{TestID: p.testID, Style: core.Style{Gap: 8}, Children: []core.Node{p.head, surface}}
}

// Inset is the recessed surface demos set apart. Zero Gap and Padding
// take 10 pt gaps and 12 pt padding, unless Exact takes them as given.
type Inset struct {
	Gap, Padding core.Points
	Exact        bool
	Children     []core.Node
}

// Render implements core.Node.
func (in Inset) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	gap, padding := in.Gap, in.Padding
	if !in.Exact {
		if gap == 0 {
			gap = 10
		}
		if padding == 0 {
			padding = 12
		}
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Background: colors.Inset(), CornerRadius: RadiusInset, BorderCurve: core.BorderCurveContinuous,
			Padding: core.All(padding), Gap: gap}
	}, Children: in.Children}
}

// Sheet is a panel at the bottom over a dimmed backdrop while Open
// (base.Sheet in the gallery's look); a press on the backdrop, Escape or
// the back button calls OnClose.
type Sheet struct {
	Open     func() bool
	OnClose  func()
	Children []core.Node
}

// Render implements core.Node.
func (s Sheet) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	env := cx.Env()
	return base.Sheet{
		Open: s.Open,
		OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				s.OnClose()
			}
		},
		BackdropStyle: core.Style{Background: Black(0.4)},
		PopupStyleFunc: func() core.Style {
			return core.Style{Gap: 12, Padding: core.Edges{Top: 8, Right: 16, Bottom: env.SafeArea().Bottom + 16, Left: 16},
				CornerRadii: core.Corners{TopLeft: 16, TopRight: 16}, Background: colors.Surface()}
		},
		Children: append([]core.Node{core.View{Style: core.Style{Width: core.Pt(36), Height: core.Pt(5), CornerRadius: 3, AlignSelf: core.AlignCenter},
			StyleFunc: Filled(colors.Border)}}, s.Children...),
	}
}

// edgeFade is how far content fades at an edge it scrolls past.
const edgeFade core.Points = 24

// ScrollView scrolls its Children (a column, or a row when Horizontal)
// and fades them into EdgeColor at each edge more of them lies past, so
// the reader sees there is more. TopBar and BottomBar float the app's
// controls over a vertical view's edges on a FloatingBar, and the
// content starts and ends clear of them. The app sizes it, as a
// core.Scroll.
type ScrollView struct {
	Horizontal bool
	Children   []core.Node
	TopBar     []core.Node
	BottomBar  []core.Node
	// TopBarLabel and BottomBarLabel name the bars for assistive
	// technology.
	TopBarLabel, BottomBarLabel string
	// EdgeColor is what the content scrolls over: the page's background
	// when nil.
	EdgeColor func() core.Color
	OnScroll  func(core.ScrollEvent)
	Ref       *core.Ref
	TestID    string
	// Style merges over the frame's; ContentStyle over the content's,
	// such as its padding and gap.
	Style        core.Style
	ContentStyle core.Style
}

// Render implements core.Node.
func (v ScrollView) Render(cx *core.Scope) core.Node {
	color := v.EdgeColor
	if color == nil {
		color = ColorsOf(cx).Background
	}
	offset := core.NewAnimatedValue(cx, 0)
	viewport, content := core.NewSignal(cx, core.Points(0)), core.NewSignal(cx, core.Points(0))
	along := func(s core.Size) core.Points {
		if v.Horizontal {
			return s.Width
		}
		return s.Height
	}
	values := core.AnimatedXY{Y: offset}
	if v.Horizontal {
		values = core.AnimatedXY{X: offset}
	}
	bars := !v.Horizontal
	const bar = ButtonHeight + 8
	var pad core.Edges
	var top, bottom core.Points
	if bars && v.TopBar != nil {
		top = 8 + bar
		pad.Top = top + 8
	}
	if bars && v.BottomBar != nil {
		bottom = 8 + bar
		pad.Bottom = bottom + 8
	}
	inner := core.Style{Padding: pad}
	if v.Horizontal {
		inner.Direction = core.Row
	}
	scroll := core.Scroll{Horizontal: v.Horizontal, OffsetValues: values, Ref: v.Ref, TestID: v.TestID, OnScroll: v.OnScroll,
		Style:    core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0)},
		OnLayout: func(b core.Bounds) { viewport.Set(along(b.Size)) },
		Children: []core.Node{core.View{Style: inner.Merge(v.ContentStyle), Children: v.Children,
			OnLayout: func(b core.Bounds) { content.Set(along(b.Size)) }}}}
	// fade is the band at one edge, shown as far as the content goes on
	// past it.
	fade := func(end bool, extent core.Points) core.Node {
		s := core.Style{Position: core.Absolute}
		size := core.Pt(extent + edgeFade)
		angle := float32(180)
		switch {
		case v.Horizontal && !end:
			s.Top, s.Bottom, s.Left, s.Width, angle = core.Pt(0), core.Pt(0), core.Pt(0), size, 90
		case v.Horizontal:
			s.Top, s.Bottom, s.Right, s.Width, angle = core.Pt(0), core.Pt(0), core.Pt(0), size, 270
		case !end:
			s.Left, s.Right, s.Top, s.Height = core.Pt(0), core.Pt(0), core.Pt(0), size
		default:
			s.Left, s.Right, s.Bottom, s.Height, angle = core.Pt(0), core.Pt(0), core.Pt(0), size, 0
		}
		return core.View{PointerEvents: core.PointerEventsNone, Style: s, StyleFunc: func() core.Style {
			room := float64(content.Get() - viewport.Get())
			if room <= 0 {
				return core.Style{Opacity: core.Alpha(0)}
			}
			input, output := []float64{0, float64(edgeFade)}, []float64{0, 1}
			if end {
				input, output = []float64{room - float64(edgeFade), room}, []float64{1, 0}
			}
			c := color()
			return core.Style{Gradients: []core.Gradient{{Kind: core.GradientLinear, Angle: angle, Stops: []core.ColorStop{
				{Color: c, At: core.Pt(0)}, {Color: c.Faded(0.9), At: core.Pt(extent)}, {Color: c.Faded(0), At: core.Pct(100)}}}},
				Animated: &core.AnimatedStyle{Opacity: core.Interpolate(offset,
					core.Interpolation{Input: input, Output: output, Extrapolate: core.ExtrapolateClamp})}}
		}}
	}
	children := []core.Node{scroll, fade(false, top), fade(true, bottom)}
	float := func(items []core.Node, label string, end bool) core.Node {
		s := core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 8)}
		if end {
			s.Bottom = core.Pt(8)
		} else {
			s.Top = core.Pt(8)
		}
		return core.View{PointerEvents: core.PointerEventsBoxNone, Style: s, Children: []core.Node{FloatingBar{Children: items, Label: label}}}
	}
	if bars && v.TopBar != nil {
		children = append(children, float(v.TopBar, v.TopBarLabel, false))
	}
	if bars && v.BottomBar != nil {
		children = append(children, float(v.BottomBar, v.BottomBarLabel, true))
	}
	return core.View{Style: core.Style{FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0), Overflow: core.Hidden}.Merge(v.Style),
		Children: children}
}

// FloatingBar is a capsule of controls floating over content, such as
// icon buttons, on the glass of a bar.
type FloatingBar struct {
	Children []core.Node
	// Label names the bar for assistive technology.
	Label  string
	TestID string
	Style  core.Style
}

// Render implements core.Node.
func (b FloatingBar) Render(cx *core.Scope) core.Node {
	return core.View{TestID: b.TestID, Label: b.Label,
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, Padding: core.All(4), MaxWidth: core.Pct(100),
			CornerRadius: radiusFull, BorderWidth: 1},
		StyleFunc: func() core.Style {
			return FinOf(cx).Glass().Over(core.Style{Color: ColorsOf(cx).Text()}).Merge(b.Style)
		}, Children: b.Children}
}

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
	still := reduceMotion(cx)
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
		return core.View{PointerEvents: core.PointerEventsNone, Style: typeTitle.Merge(core.Style{Position: core.Absolute,
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
			Button{Variant: ButtonGhost, Icon: IconClose, Text: s.LeftLabel, Disabled: off, OnPress: func() { decide(CardLeft, 0) }},
			Button{Variant: ButtonAccent, Icon: IconCheck, Text: s.RightLabel, Disabled: off, OnPress: func() { decide(CardRight, 0) }},
		}}
	return core.View{Style: core.Style{Gap: 16}, Children: []core.Node{stage, buttons}}
}
