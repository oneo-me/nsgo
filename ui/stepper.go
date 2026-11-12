package ui

import (
	"strconv"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// StepStatus is where a step of a Stepper stands.
type StepStatus uint8

const (
	StepUpcoming StepStatus = iota
	StepCurrent
	StepComplete
	// StepFailed is a step with an Error.
	StepFailed
)

// StepperStep is one step of a Stepper: a Label with a short Description
// under it. Error marks it failed: its marker turns into an alert and the
// message replaces the description.
type StepperStep struct {
	Label       string
	Description string
	Error       string
}

// Stepper is a stepper, where a person is in a flow of steps, for
// onboarding, checkout and setup: completed steps show a check, the
// current one a ring around its number and failed ones an alert, and the
// track between them fills up to the current step, one connector after
// another across a jump. Current is the step in progress, len(Steps)
// once every one is done; out-of-range values clamp. The app owns the
// steps, the flow and the content beside it.
//
// Steps run in a row, or down a column with Vertical. CurrentDescription
// shows only the current step's description, and Compact the markers
// alone, the current step's label under a row of them (CompleteLabel once
// all are done). With OnSelect, completed steps are buttons that return
// to them. Label names the group for assistive technology, and
// StatusText, when set, the status read after each step's label, such as
// "completed".
type Stepper struct {
	Steps              []StepperStep
	Current            func() int
	Vertical           bool
	CurrentDescription bool
	Compact            bool
	OnSelect           func(step int)
	Label              string
	CompleteLabel      string
	StatusText         func(StepStatus) string
	TestID             string
}

// Stepper's measures and motion: a 28-point marker with a 4-point ring,
// connectors 2 points thick, and a jump that fills or drains them 80 ms
// apart before the current step's ring grows.
const (
	stepMarker  core.Points = 28
	stepHalo    core.Points = 4
	stepTrack   core.Points = 2
	stepGap     core.Points = 8
	stepStagger             = 80 * time.Millisecond
	stepRing                = 120 * time.Millisecond
)

var (
	stepEase  = core.Curve{X1: 0.16, Y1: 1, X2: 0.3, Y2: 1}
	stepColor = core.Transition{Duration: 240 * time.Millisecond, Easing: core.Curve{X1: 0.22, Y1: 1, X2: 0.36, Y2: 1},
		Properties: core.TransitionBackground | core.TransitionBorder}
)

// Render implements core.Node.
func (s Stepper) Render(cx *core.Scope) core.Node {
	count := len(s.Steps)
	current := func() int {
		if s.Current == nil {
			return 0
		}
		return min(max(s.Current(), 0), count)
	}
	last := core.Untracked(cx, current)
	// travel is the step progress came from and the one it is at, so a
	// jump can stagger what it crosses.
	travel := core.NewMemo(cx, func() [2]int {
		to := current()
		from := last
		last = to
		return [2]int{from, to}
	})
	status := func(i int) StepStatus {
		switch c := current(); {
		case s.Steps[i].Error != "":
			return StepFailed
		case i < c:
			return StepComplete
		case i == c:
			return StepCurrent
		}
		return StepUpcoming
	}
	still := ReduceMotion(cx)
	delay := func(i int) time.Duration {
		t := travel.Get()
		from, to := t[0], t[1]
		switch {
		case still:
			return 0
		case to > from && i >= from && i < to:
			return time.Duration(i-from) * stepStagger
		case to < from && i >= to && i < from:
			return time.Duration(from-1-i) * stepStagger
		}
		return 0
	}
	ring := func() time.Duration {
		t := travel.Get()
		return time.Duration(max(0, t[1]-t[0]-1, t[0]-t[1]-1))*stepStagger + stepRing
	}
	head := core.Style{Gap: 12, MinWidth: core.Pt(0), CornerRadius: RadiusControl, AlignItems: core.AlignStart}
	if s.Vertical {
		head.Direction = core.Row
	}
	if s.OnSelect != nil {
		head.Group = "step"
	}
	var items []core.Node
	for i, step := range s.Steps {
		v := core.View{Style: head, Hint: step.Description, Children: s.head(cx, i, status, delay, ring),
			StyleFunc: func() core.Style {
				if s.OnSelect == nil {
					return core.Style{}
				}
				return core.Style{FocusVisible: FocusRing(cx, 4)}
			},
			LabelFunc: func() string {
				if s.StatusText == nil {
					return step.Label
				}
				return step.Label + ", " + s.StatusText(status(i))
			}}
		if s.TestID != "" {
			v.TestID = s.TestID + "-" + strconv.Itoa(i)
		}
		if s.OnSelect != nil {
			v.Role = core.RoleButton
			v.Disabled = func() bool { return i > current() }
			v.OnPress = func(core.PressEvent) {
				if i < core.Untracked(cx, current) {
					s.OnSelect(i)
				}
			}
		} else {
			v.Accessible = true
		}
		items = append(items, s.item(cx, i, status, delay, v))
	}
	list := core.Style{Direction: core.Row, AlignItems: core.AlignStart, MinWidth: core.Pt(0)}
	style := core.Style{AlignSelf: core.AlignStretch, MinWidth: core.Pt(0)}
	if s.Vertical {
		list, style = core.Style{MinWidth: core.Pt(0)}, core.Style{MinWidth: core.Pt(0)}
	}
	children := []core.Node{core.View{Style: list, Children: items}}
	if s.Compact && !s.Vertical {
		// Assistive technology hears the caption as it changes.
		children = append(children, core.View{Style: core.Style{Margin: core.Edges{Top: 12}, Gap: 2}, LiveRegion: core.LiveRegionPolite,
			Children: []core.Node{s.caption(cx, current)}})
	}
	return core.View{Style: style, Label: s.Label, TestID: s.TestID, Children: children}
}

// head is what step i shows: its marker and text.
func (s Stepper) head(cx *core.Scope, i int, status func(int) StepStatus, delay func(int) time.Duration, ringDelay func() time.Duration) []core.Node {
	colors := ColorsOf(cx)
	step := s.Steps[i]
	st := func() StepStatus { return status(i) }
	last := i == len(s.Steps)-1
	detail := func() string {
		if step.Error != "" {
			return step.Error
		}
		if s.CurrentDescription && st() != StepCurrent {
			return ""
		}
		return step.Description
	}
	head := []core.Node{s.marker(cx, i, status, delay, ringDelay)}
	if s.Compact {
		return head
	}
	text := core.View{Style: core.Style{MinWidth: core.Pt(0), Gap: 2}, Children: []core.Node{
		core.Text{Content: step.Label, Style: TypeLabel, StyleFunc: func() core.Style {
			c := colors.Muted()
			switch st() {
			case StepCurrent, StepFailed:
				c = colors.Text()
			case StepComplete:
				c = Mix(colors.Muted(), colors.Text(), 0.5)
			}
			return core.Style{Color: c}
		}},
		core.Text{ContentFunc: detail, Style: TypeCaption, StyleFunc: func() core.Style {
			s := core.Style{Color: colors.Muted()}
			if step.Error != "" {
				s.Color = colors.Danger()
			}
			if detail() == "" {
				s.Display = core.DisplayNone
			}
			return s
		}},
	}}
	switch {
	case s.Vertical:
		text.Style.FlexGrow, text.Style.FlexShrink, text.Style.Padding = 1, 1, core.Edges{Top: 4}
		if !last {
			text.Style.Padding.Bottom = 20
		}
	case !last:
		text.Style.Padding = core.Edges{Right: 16}
	}
	return append(head, text)
}

// item is step i's head and the connector to the next.
func (s Stepper) item(cx *core.Scope, i int, status func(int) StepStatus, delay func(int) time.Duration, head core.Node) core.Node {
	last := i == len(s.Steps)-1
	item := core.View{Style: core.Style{MinWidth: core.Pt(0)}, Children: []core.Node{head}}
	switch {
	case s.Vertical && !last && s.Compact:
		item.Style.Padding = core.Edges{Bottom: 24}
	case !s.Vertical && !last:
		item.Style.FlexGrow, item.Style.FlexShrink, item.Style.FlexBasis = 1, 1, core.Pt(0)
	case !s.Vertical:
		item.Style.FlexShrink = 0
	}
	if !last {
		item.Children = append(item.Children, s.connector(cx, i, status, delay))
	}
	return item
}

// connector is the track from step i's marker to the next one's, with
// the accent filling it once step i is complete.
func (s Stepper) connector(cx *core.Scope, i int, status func(int) StepStatus, delay func(int) time.Duration) core.Node {
	colors := ColorsOf(cx)
	var track, fill core.Style
	if s.Vertical {
		track = core.Style{Position: core.Absolute, Left: core.Pt((stepMarker - stepTrack) / 2), Width: core.Pt(stepTrack),
			Top: core.Pt(stepMarker + stepHalo + 2), Bottom: core.Pt(stepHalo + 2)}
		fill = core.Style{TransformOrigin: core.ObjectPosition{Y: core.PositionAxis{Offset: core.Pct(0)}}}
	} else {
		track = core.Style{Position: core.Absolute, Top: core.Pt((stepMarker - stepTrack) / 2), Height: core.Pt(stepTrack),
			Left: core.Pt(stepMarker + stepHalo + stepGap/2), Right: core.Pt(stepHalo + stepGap/2)}
		fill = core.Style{TransformOrigin: core.ObjectPosition{X: core.PositionAxis{Offset: core.Pct(0)}}}
	}
	track.CornerRadius, track.Overflow = stepTrack/2, core.Hidden
	fill.Position, fill.Left, fill.Right, fill.Top, fill.Bottom = core.Absolute, core.Pt(0), core.Pt(0), core.Pt(0), core.Pt(0)
	vertical := s.Vertical
	return core.View{PointerEvents: core.PointerEventsNone, Style: track, StyleFunc: Filled(colors.Border), Children: []core.Node{
		core.View{Style: fill, StyleFunc: func() core.Style {
			// A scale of 0 is unset, so an empty track scales to almost none.
			scale := float32(0.0001)
			if st := status(i); st == StepComplete || st == StepFailed && i < s.Current() {
				scale = 1
			}
			t := core.Transform{ScaleX: scale}
			if vertical {
				t = core.Transform{ScaleY: scale}
			}
			return core.Style{Background: colors.AccentFill(), Transform: t,
				Transition: core.Transition{Duration: 400 * time.Millisecond, Delay: delay(i), Easing: stepEase,
					Properties: core.TransitionTransform | core.TransitionBackground}}
		}},
	}}
}

// marker is step i's disc and glyph and, on the current step, the ring
// that grows out from behind it. The number, the check and the alert
// share one spot: the outgoing glyph shrinks away as the next pops in.
func (s Stepper) marker(cx *core.Scope, i int, status func(int) StepStatus, delay func(int) time.Duration, ringDelay func() time.Duration) core.Node {
	colors := ColorsOf(cx)
	shown := func(on bool, rest float32, d time.Duration) core.Style {
		if on {
			return core.Style{Opacity: core.Alpha(1), Transform: core.Transform{Scale: 1},
				Transition: core.Transition{Duration: 260 * time.Millisecond, Delay: d, Easing: stepEase, Properties: core.TransitionOpacity | core.TransitionTransform}}
		}
		return core.Style{Opacity: core.Alpha(0), Transform: core.Transform{Scale: rest},
			Transition: core.Transition{Duration: 160 * time.Millisecond, Easing: stepEase, Properties: core.TransitionOpacity | core.TransitionTransform}}
	}
	glyph := func(kind StepStatus) bool {
		st := status(i)
		if kind == StepCurrent {
			return st == StepCurrent || st == StepUpcoming
		}
		return st == kind
	}
	layer := core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0),
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
	ink := func() core.Color {
		switch status(i) {
		case StepCurrent:
			return colors.Text()
		case StepComplete:
			return colors.OnAccent()
		case StepFailed:
			return colors.Danger()
		}
		return colors.Muted()
	}
	number := Tabular(core.Text{Content: strconv.Itoa(i + 1), Style: core.Style{FontSize: 12, FontWeight: Medium}, StyleFunc: Colored(ink)})
	alert := core.Text{Content: "!", Style: core.Style{FontSize: 14, FontWeight: Bold}, StyleFunc: Colored(ink)}
	ring := core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(-stepHalo), Top: core.Pt(-stepHalo),
		Width: core.Pt(stepMarker + 2*stepHalo), Height: core.Pt(stepMarker + 2*stepHalo), CornerRadius: RadiusFull},
		StyleFunc: func() core.Style {
			st := status(i)
			c := Alpha(colors.Accent(), 0.18)
			if st == StepFailed {
				c = Alpha(colors.Danger(), 0.16)
			}
			on := st == StepCurrent || st == StepFailed && i == s.Current()
			return core.Style{Background: c}.Merge(shown(on, 0.6, ringDelay()))
		}}
	disc := core.View{Style: core.Style{Width: core.Pt(stepMarker), Height: core.Pt(stepMarker), CornerRadius: RadiusFull},
		StyleFunc: func() core.Style {
			bg, border, width := colors.Surface(), Mix(colors.Border(), colors.Text(), 0.25), core.Points(1)
			var hover *core.GroupStyle
			switch status(i) {
			case StepCurrent:
				border, width = colors.Accent(), 1.5
			case StepComplete:
				p := cx.Palette()
				bg, border = p.Accent, p.Accent
				if s.OnSelect != nil {
					hover = &core.GroupStyle{Hover: &core.Style{Background: p.AccentHover, BorderColor: p.AccentHover},
						Active: &core.Style{Background: p.AccentPressed, BorderColor: p.AccentPressed, Transform: core.Transform{Scale: 0.92}}}
				}
			case StepFailed:
				bg, border, width = Mix(colors.Surface(), colors.Danger(), 0.1), colors.Danger(), 1.5
			}
			return core.Style{Background: bg, BorderColor: border, BorderWidth: width, GroupStyle: hover, Transition: stepColor}
		},
		Children: []core.Node{
			core.View{Style: layer, StyleFunc: func() core.Style { return shown(glyph(StepCurrent), 0.5, delay(i)) }, Children: []core.Node{number}},
			core.View{Style: layer, StyleFunc: func() core.Style { return shown(glyph(StepComplete), 0.5, delay(i)) },
				Children: []core.Node{Icon(lucide.Check, 14, ink)}},
			core.View{Style: layer, StyleFunc: func() core.Style { return shown(glyph(StepFailed), 0.5, delay(i)) }, Children: []core.Node{alert}},
		}}
	return core.View{Style: core.Style{Width: core.Pt(stepMarker), Height: core.Pt(stepMarker), FlexShrink: 0}, Children: []core.Node{ring, disc}}
}

// caption names the current step under a compact row of markers, and
// its description or error under that.
func (s Stepper) caption(cx *core.Scope, current func() int) core.Node {
	colors := ColorsOf(cx)
	step := func() (StepperStep, bool) {
		if c := current(); c < len(s.Steps) {
			return s.Steps[c], true
		}
		return StepperStep{}, false
	}
	title := core.Text{Style: TypeLabel, StyleFunc: Colored(colors.Text), ContentFunc: func() string {
		if st, ok := step(); ok {
			return st.Label
		}
		return s.CompleteLabel
	}}
	detail := core.Text{Style: TypeCaption, ContentFunc: func() string {
		st, _ := step()
		if st.Error != "" {
			return st.Error
		}
		return st.Description
	}, StyleFunc: func() core.Style {
		st, _ := step()
		c := colors.Muted()
		if st.Error != "" {
			c = colors.Danger()
		}
		s := core.Style{Color: c}
		if st.Error == "" && st.Description == "" {
			s.Display = core.DisplayNone
		}
		return s
	}}
	return core.Group{title, detail}
}
