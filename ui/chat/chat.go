package chat

import (
	"math"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"
)

// Status is a turn or activity's lifecycle, as the app reports it. A
// tool finishing does not finish its turn; only Completed folds it.
type Status uint8

const (
	Running Status = iota
	Waiting
	Completed
	Failed
	Cancelled
)

// View builds content the app owns: Markdown, attachments, confirmation
// controls or a bounded tool output. It follows the signals it reads.
type View func(cx *core.Scope) core.Node

// Activity is one step of work inside a Step, such as reading a
// file, running a command or thinking. ID is stable within its turn, so a
// tool's result updates its activity however results are ordered. Action
// is a short verb ("Read") and Detail its target.
type Activity struct {
	ID, Action, Detail string
	// Icon leads the row, lucide.Wrench when empty.
	Icon   func() *core.SVG
	Status Status
	// Content is the expanded detail; nil keeps the activity a single row.
	Content View
	// Mono presents Content as monospace output rather than prose.
	Mono bool
	// MaxHeight bounds the expanded detail, which then scrolls; 0 is 400.
	MaxHeight core.Points
}

// Step is intermediate prose (Message) or a group of activities under
// a Title the app summarizes, such as "Ran 2 commands".
type Step struct {
	ID         string
	Message    View
	Title      string
	Activities []Activity
}

// Turn is one request and its response: the prompt, the steps and the
// final response. A completed turn with a final response folds its steps
// behind a centered summary row and builds nothing hidden; a running turn
// ends with a pulsing working row. A choice the user makes survives
// updates. The expansion lives as long as the view does, so key turns by a
// stable ID.
type Turn struct {
	Status func() Status
	// Summary labels the completed turn's fold row ("Worked for 2m") and
	// Working the live indicator ("Working for 12s").
	Summary, Working func() string
	Prompt           View
	Steps            func() []Step
	// Final is the response, shown once HasFinal is true: the last step
	// is never taken for the answer.
	Final    View
	HasFinal func() bool
	// Footer holds results or errors and Actions response actions such
	// as copy, while ShowActions is true or nil. Neither folds with the
	// steps.
	Footer, Actions View
	ShowActions     func() bool
	// HoldOpen delays folding while the user reads older content, as being
	// above the latest turn of Conversation does on its own.
	HoldOpen     func() bool
	Conversation *ConversationState
	TestID       string
}

// Render implements core.Node.
func (t Turn) Render(cx *core.Scope) core.Node {
	final := func() bool { return is(t.HasFinal) }
	steps := func() []Step {
		if t.Steps == nil {
			return nil
		}
		return t.Steps()
	}
	hold := func() bool {
		return is(t.HoldOpen) || t.Conversation != nil && !t.Conversation.AtLatest()
	}
	// choice is the user's: 0 for none, 1 for open and 2 for closed.
	choice := core.NewSignal(cx, uint8(0))
	folded := false
	open := core.NewMemo(cx, func() bool {
		held := hold()
		if c := choice.Get(); c != 0 {
			return c == 1
		}
		if t.Status() != Completed || !final() {
			folded = false
			return true
		}
		if !held {
			folded = true
		}
		return !folded
	})
	foldable := func() bool { return t.Status() == Completed && final() && len(steps()) > 0 }
	var children []core.Node
	if t.Prompt != nil {
		children = append(children, Message{Outgoing: true, Children: []core.Node{Bubble{Children: []core.Node{core.Func(t.Prompt)}}}})
	}
	toggle := func() {
		o := open.GetUntracked()
		if o {
			choice.Set(2)
		} else {
			choice.Set(1)
		}
		t.Conversation.pause()
	}
	children = append(children,
		core.Show{When: foldable, Then: func(cx *core.Scope) core.Node { return chatFoldRow(cx, t.Summary, open.Get, toggle) }},
		core.Show{When: func() bool { return open.Get() && len(steps()) > 0 }, Then: func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{MinWidth: core.Pt(0), Gap: 16},
				Children: []core.Node{core.Each(steps, func(s Step) string { return s.ID }, func(cx *core.Scope, s func() Step, i func() int) core.Node {
					live := func() bool { return t.Status() == Running && i() == len(steps())-1 }
					return chatStep(cx, s, live, t.Conversation)
				})}}
		}},
		core.Show{When: func() bool { return t.Status() == Running }, Then: func(cx *core.Scope) core.Node { return chatWorkingRow(cx, t.Working) }},
	)
	if t.Final != nil {
		children = append(children, core.Show{When: final, Then: func(*core.Scope) core.Node {
			return Message{Children: []core.Node{core.Func(t.Final)}}
		}})
	}
	if t.Footer != nil {
		children = append(children, core.Func(t.Footer))
	}
	if t.Actions != nil {
		children = append(children, core.Show{When: func() bool { return t.ShowActions == nil || t.ShowActions() }, Then: func(*core.Scope) core.Node {
			return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 4}, Children: []core.Node{core.Func(t.Actions)}}
		}})
	}
	return core.View{TestID: t.TestID, Style: core.Style{MinWidth: core.Pt(0), FlexShrink: 0, Gap: 16}, Children: children}
}

// chatFoldRow is the divider-centered summary of a completed turn.
func chatFoldRow(cx *core.Scope, summary func() string, open func() bool, toggle func()) core.Node {
	colors := tonesOf(cx)
	divider := func() core.Node {
		return core.View{Style: core.Style{FlexGrow: 1, Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Border)}
	}
	label := func() string { return summary() }
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(24), Gap: 10}, Children: []core.Node{
		divider(),
		core.View{Role: core.RoleButton, Expanded: open, OnPress: func(core.PressEvent) { toggle() }, LabelFunc: label,
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(24), Padding: core.Symmetric(0, 2), Gap: 5,
				CornerRadius: ui.RadiusControl, Cursor: core.CursorPointer},
			StyleFunc: func() core.Style { return core.Style{Focus: ui.FocusRing(cx, 2)} },
			Children: []core.Node{
				ui.Textf(label, 13.5, ui.Medium, colors.Muted),
				ui.IconOf(chatDisclosure(open), 11.5, colors.Muted),
			}},
		divider(),
	}}
}

// chatDisclosure is the chevron of something open or closed.
func chatDisclosure(open func() bool) func() *core.SVG {
	return func() *core.SVG {
		if open() {
			return lucide.ChevronDown()
		}
		return lucide.ChevronRight()
	}
}

// chatWorkingRow shows pulsing dots and the live elapsed label.
func chatWorkingRow(cx *core.Scope, label func() string) core.Node {
	colors := tonesOf(cx)
	children := []core.Node{chatWaveDots(cx)}
	if label != nil {
		children = append(children, ui.Textf(label, 13.5, ui.Medium, colors.Muted))
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(22), Gap: 8}, Children: children}
}

// chatWave is count stops of a period of wave, for a keyframe loop.
func chatWave(count int, wave func(phase float64) float64) []core.KeyframeStop {
	values := make([]float64, count+1)
	for i := range values {
		values[i] = wave(float64(i) / float64(count))
	}
	return core.Stops(values...)
}

// chatWaveDots are three dots a wave of opacity crosses every 1.2 s.
func chatWaveDots(cx *core.Scope) core.Node {
	colors := tonesOf(cx)
	reduced := ui.ReduceMotion(cx)
	var dots []core.Node
	for i := range 3 {
		s := core.Style{Width: core.Pt(4), Height: core.Pt(4), CornerRadius: 2, Opacity: core.Alpha(0.35)}
		if !reduced {
			v := core.NewAnimatedValue(cx, 0.35)
			cx.StartAnimation(core.ValueKeyframes(v, core.KeyframeTiming{Duration: 1200 * time.Millisecond, Iterations: core.Forever},
				chatWave(24, func(p float64) float64 { return 0.35 + 0.65*max(0, math.Sin((p-float64(i)*0.15)*2*math.Pi)) })...), nil)
			s.Animated = &core.AnimatedStyle{Opacity: v}
		}
		dots = append(dots, core.View{Style: s, StyleFunc: ui.Filled(colors.Muted)})
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 2, Width: core.Pt(18), Height: core.Pt(6), FlexShrink: 0}, Children: dots}
}

// chatPulseDot is the accent's dot of a running activity, breathing once a
// second.
func chatPulseDot(cx *core.Scope) core.Node {
	s := core.Style{Width: core.Pt(5), Height: core.Pt(5), CornerRadius: 2.5, FlexShrink: 0}
	if !ui.ReduceMotion(cx) {
		v := core.NewAnimatedValue(cx, 1)
		cx.StartAnimation(core.ValueKeyframes(v, core.KeyframeTiming{Duration: time.Second, Iterations: core.Forever},
			chatWave(12, func(p float64) float64 { return 0.45 + 0.55*(1+math.Sin(p*2*math.Pi))/2 })...), nil)
		s.Animated = &core.AnimatedStyle{Opacity: v}
	}
	return core.View{Style: s, StyleFunc: ui.Filled(tonesOf(cx).Accent)}
}

// chatStep is a step: its prose, then its activity group.
func chatStep(cx *core.Scope, s func() Step, live func() bool, conversation *ConversationState) core.Node {
	return core.View{Style: core.Style{MinWidth: core.Pt(0)}, Children: []core.Node{
		core.Show{When: func() bool { return s().Message != nil }, Then: func(cx *core.Scope) core.Node { return core.Func(s().Message) }},
		core.Show{When: func() bool { return len(s().Activities) > 0 }, Then: func(cx *core.Scope) core.Node {
			return chatActivityGroup(cx, s, live, conversation)
		}},
	}}
}

// chatActivityGroup is a disclosure header over a guide-lined list of
// rows. It is open while live until the user chooses otherwise.
func chatActivityGroup(cx *core.Scope, s func() Step, live func() bool, conversation *ConversationState) core.Node {
	colors := tonesOf(cx)
	// choice is the user's: 0 for none, 1 for open and 2 for closed.
	choice := core.NewSignal(cx, uint8(0))
	open := func() bool {
		if c := choice.Get(); c != 0 {
			return c == 1
		}
		return live()
	}
	title := func() string { return s().Title }
	return core.View{Style: core.Style{MinWidth: core.Pt(0), Gap: 4}, Children: []core.Node{
		core.View{Role: core.RoleButton, Expanded: open, LabelFunc: title,
			OnPress: func(core.PressEvent) {
				if core.Untracked(cx, open) {
					choice.Set(2)
				} else {
					choice.Set(1)
				}
				conversation.pause()
			},
			Style:     core.Style{Group: "chat-group", Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(26), Gap: 6, MinWidth: core.Pt(0), Cursor: core.CursorPointer},
			StyleFunc: func() core.Style { return core.Style{Focus: ui.FocusRing(cx, 2)} },
			Children: []core.Node{
				core.Text{ContentFunc: title, MaxLines: 1, Style: core.Style{FontSize: 12.5, FontWeight: ui.Medium, MinWidth: core.Pt(0), FlexShrink: 1}, StyleFunc: func() core.Style {
					return core.Style{Color: colors.chatSecondary(), GroupStyle: &core.GroupStyle{Name: "chat-group", Hover: &core.Style{Color: colors.Text()}, Focus: &core.Style{Color: colors.Text()}}}
				}},
				ui.IconOf(chatDisclosure(open), 10, colors.Muted),
			}},
		core.Show{When: open, Then: func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{MinWidth: core.Pt(0), Margin: core.Edges{Left: 6}, Padding: core.Edges{Bottom: 2, Left: 12}, Borders: core.Edges{Left: 1}, Gap: 8},
				StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Left: colors.Border()}} },
				Children: []core.Node{core.Each(func() []Activity { return s().Activities }, func(a Activity) string { return a.ID },
					func(cx *core.Scope, a func() Activity, _ func() int) core.Node {
						return ActivityRow{Activity: a, Conversation: conversation}
					})}}
		}},
	}}
}

// ActivityRow is one activity as a compact card that can expand to its
// bounded detail. What the detail costs and how far it is cut is the app's.
type ActivityRow struct {
	Activity     func() Activity
	Conversation *ConversationState
}

// Render implements core.Node.
func (r ActivityRow) Render(cx *core.Scope) core.Node {
	colors := tonesOf(cx)
	a := r.Activity
	expanded := core.NewSignal(cx, false)
	detail := core.NewMemo(cx, func() bool { return a().Content != nil })
	open := func() bool { return detail.Get() && expanded.Get() }
	header := []core.Node{
		ui.IconOf(func() *core.SVG {
			if icon := a().Icon; icon != nil {
				return icon()
			}
			return lucide.Wrench()
		}, 12, colors.Muted),
		core.Text{ContentFunc: func() string { return a().Action }, MaxLines: 1, Style: core.Style{FontSize: 12.5, FontWeight: ui.Semibold, FlexShrink: 0}, StyleFunc: ui.Colored(colors.chatSecondary)},
		core.Text{ContentFunc: func() string { return a().Detail }, MaxLines: 1, Style: core.Style{FontSize: 12.5, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, StyleFunc: ui.Colored(colors.chatSecondary)},
		core.Show{When: func() bool { return a().Status == Failed }, Then: func(*core.Scope) core.Node { return ui.Icon(lucide.X, 10, colors.Danger) }},
		core.Show{When: func() bool { return a().Status == Running && !detail.Get() }, Then: func(cx *core.Scope) core.Node { return chatPulseDot(cx) }},
		core.Show{When: detail.Get, Then: func(*core.Scope) core.Node { return ui.IconOf(chatDisclosure(open), 10, colors.Muted) }},
	}
	shape := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(28), Padding: core.Symmetric(0, 8), Gap: 8, MinWidth: core.Pt(0)}
	return core.View{Style: core.Style{MinWidth: core.Pt(0), CornerRadius: 9, BorderWidth: 1, Overflow: core.Hidden}, StyleFunc: func() core.Style {
		return core.Style{BorderColor: colors.chatStrongBorder(), Background: ui.Mix(colors.Background(), colors.Text(), 0.035)}
	}, Children: []core.Node{
		core.Dynamic(func(cx *core.Scope) core.Node {
			if !detail.Get() {
				return core.View{Style: shape, Children: header}
			}
			return core.View{Role: core.RoleButton, Expanded: open, LabelFunc: func() string { return a().Action + " " + a().Detail },
				OnPress: func(core.PressEvent) {
					expanded.Update(func(v bool) bool { return !v })
					r.Conversation.pause()
				},
				Style: shape.Merge(core.Style{Cursor: core.CursorPointer}),
				StyleFunc: func() core.Style {
					return core.Style{Hover: &core.Style{Background: ui.Alpha(colors.Text(), 0.03)}, Focus: &core.Style{Background: ui.Alpha(colors.Text(), 0.03)}}
				},
				Children: header}
		}),
		core.Show{When: open, Then: func(cx *core.Scope) core.Node { return chatActivityDetail(cx, a) }},
	}}
}

// chatActivityDetail is an open activity's bounded viewport, which follows
// a running activity's output unless the user scrolled up.
func chatActivityDetail(cx *core.Scope, a func() Activity) core.Node {
	colors := tonesOf(cx)
	scroll := &core.Ref{}
	first := core.Untracked(cx, a)
	height := first.MaxHeight
	if height <= 0 {
		height = 400
	}
	body := core.Style{MinWidth: core.Pt(0), Padding: core.Symmetric(8, 12), Gap: 8, FontSize: ui.TypeBody.FontSize}
	if first.Mono {
		body.FontFamily, body.FontSize = ui.Mono, 12
	}
	following := true
	return core.Scroll{Ref: scroll, Style: core.Style{MaxHeight: core.Pt(height), Borders: core.Edges{Top: 1}, FlexShrink: 1},
		StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.chatStrongBorder()}} },
		OnScroll:  func(e core.ScrollEvent) { following = e.Offset.Y+e.Size.Height >= e.ContentSize.Height-2 },
		Children: []core.Node{core.View{Style: body, StyleFunc: ui.Colored(colors.chatSecondary),
			OnLayout: func(core.Bounds) {
				if following && core.Untracked(cx, a).Status == Running {
					scroll.ScrollToEnd(false)
				}
			},
			Children: []core.Node{core.Func(first.Content)}}}}
}

// tones are the theme's colors with the tones layered on them, as Waku's
// transcript has them.
type tones struct{ ui.Colors }

// tonesOf is ui.ColorsOf with those tones.
func tonesOf(cx *core.Scope) tones { return tones{ui.ColorsOf(cx)} }

func (c tones) chatSecondary() core.Color    { return ui.Mix(c.Text(), c.Muted(), 0.45) }
func (c tones) chatStrongBorder() core.Color { return ui.Mix(c.Border(), c.Text(), 0.08) }
func (c tones) chatOverlay() core.Color      { return ui.Alpha(c.Text(), 0.05) }
func (c tones) chatOverlayStrong() core.Color {
	return ui.Alpha(c.Text(), 0.09)
}

// Message lays out a message apart from its surface: an Outgoing one
// at the trailing edge, an incoming one across the width.
type Message struct {
	Outgoing bool
	Children []core.Node
}

// Render implements core.Node.
func (m Message) Render(*core.Scope) core.Node {
	s := core.Style{MinWidth: core.Pt(0), FlexShrink: 0}
	if m.Outgoing {
		s.AlignItems = core.AlignEnd
	}
	return core.View{Style: s, Children: m.Children}
}

// Bubble is the raised surface of an outgoing message.
type Bubble struct{ Children []core.Node }

// Render implements core.Node.
func (b Bubble) Render(cx *core.Scope) core.Node {
	colors := tonesOf(cx)
	return core.View{Style: core.Style{MinWidth: core.Pt(0), MaxWidth: core.Pt(540), Padding: core.Edges{Top: 9, Right: 14, Bottom: 8, Left: 14}, Gap: 7, CornerRadius: 12},
		StyleFunc: func() core.Style {
			return core.Style{Background: ui.Mix(colors.Background(), colors.Text(), 0.06), Color: colors.Text()}
		},
		Children: b.Children}
}

// is reads an optional state.
func is(fn func() bool) bool { return fn != nil && fn() }

// constant is a label that stays.
func constant(s string) func() string { return func() string { return s } }

// hasImage reports whether src shows anything.
func hasImage(src core.ImageSource) bool { return src.URI != "" || len(src.Data) > 0 }

// linearEasing runs an animation at one pace.
var linearEasing = core.BezierEasing(core.Linear)
