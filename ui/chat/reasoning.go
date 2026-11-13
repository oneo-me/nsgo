package chat

import (
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
	"github.com/nsgo-dev/nsgo/ui/chat/locales"
)

// Reasoning is a model's reasoning before its answer. While it streams it
// stays open with what it has cost so far; once it ends it folds itself,
// unless the user opened it or is reading older content. A Turn shows it
// as its Reasoning, after the prompt and apart from the steps, so its text
// stays where the reader left it however the turn's work is folded.
type Reasoning struct {
	// Text is the reasoning, which the app streams; the block builds
	// nothing while it is empty.
	Text func() string
	// Status reports where the reasoning is: Completed folds the block and
	// stops its duration.
	Status func() Status
	// Elapsed is what the reasoning has cost so far. A running block reads
	// it again once a second; without it, ui/chat's labels give no time.
	Elapsed func() time.Duration
	// Label says the row while it runs ("Thinking for 12s") and Summary
	// once it ends ("Thought for 12s"): the app's words, through its
	// catalog. A nil Label or Summary reads ui/chat's own.
	Label, Summary func() string
	// Content replaces the default body, the Text as selectable prose.
	Content View
	// HoldOpen keeps the block open while the user reads older content, as
	// Turn's does.
	HoldOpen     func() bool
	Conversation *ConversationState
	TestID       string
}

// Render implements core.Node.
func (r Reasoning) Render(cx *core.Scope) core.Node {
	colors := tonesOf(cx)
	status := func() Status {
		if r.Status == nil {
			return Completed
		}
		return r.Status()
	}
	running := func() bool { return status() == Running }
	// reasoned reports whether there is anything to show yet.
	reasoned := func() bool {
		return r.Text != nil && r.Text() != "" || r.Content != nil
	}
	// The row counts a running block's seconds, so its label follows what
	// the block has cost; the count runs while there is reasoning that has
	// not ended, and starts again if it resumes.
	ticks := core.NewSignal(cx, 0)
	var stop func() bool
	var tick func()
	tick = func() {
		ticks.Update(func(n int) int { return n + 1 })
		stop = cx.AfterFunc(time.Second, tick)
	}
	cx.Effect(func() {
		on := reasoned() && running()
		cx.Untrack(func() {
			if stop != nil {
				stop()
				stop = nil
			}
			if on {
				stop = cx.AfterFunc(time.Second, tick)
			}
		})
	})
	seconds := func() int { return max(0, int(r.Elapsed().Seconds()+0.5)) }
	label := func() string {
		_ = ticks.Get()
		if running() {
			switch {
			case r.Label != nil:
				return r.Label()
			case r.Elapsed == nil:
				return locales.ThinkingUntimed(cx)
			}
			return locales.Thinking(cx, seconds())
		}
		switch {
		case r.Summary != nil:
			return r.Summary()
		case r.Elapsed == nil:
			return locales.Thoughts(cx)
		}
		return locales.ThoughtFor(cx, seconds())
	}
	// choice is the user's: 0 for none, 1 for open and 2 for closed.
	choice := core.NewSignal(cx, uint8(0))
	folded := false
	open := core.NewMemo(cx, func() bool {
		held := is(r.HoldOpen) || r.Conversation != nil && !r.Conversation.AtLatest()
		if c := choice.Get(); c != 0 {
			return c == 1
		}
		if running() {
			folded = false
			return true
		}
		if !held {
			folded = true
		}
		return !folded
	})
	toggle := func() {
		if open.GetUntracked() {
			choice.Set(2)
		} else {
			choice.Set(1)
		}
		r.Conversation.pause()
	}
	body := r.Content
	if body == nil {
		body = func(*core.Scope) core.Node {
			return core.Text{ContentFunc: r.Text, Selectable: core.SelectableOn, Style: core.Style{LineHeight: 20}}
		}
	}
	row := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Height: core.Pt(24)}, Children: []core.Node{
		core.Show{When: running, Then: func(cx *core.Scope) core.Node { return chatPulseDot(cx) }},
		base.Button{Expanded: open.Get, LabelFunc: label, OnPress: func(core.PressEvent) { toggle() },
			Style:     core.Style{Group: "chat-reasoning", Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(24), Gap: 5, MinWidth: core.Pt(0)},
			StyleFunc: func() core.Style { return core.Style{FocusVisible: ui.FocusRing(cx, 2)} },
			Children: []core.Node{
				core.Text{ContentFunc: label, MaxLines: 1, Style: core.Style{FontSize: 12.5, FontWeight: ui.Medium, MinWidth: core.Pt(0), FlexShrink: 1},
					StyleFunc: func() core.Style {
						return core.Style{Color: colors.chatSecondary(), GroupStyle: &core.GroupStyle{Name: "chat-reasoning",
							Hover: &core.Style{Color: colors.Text()}, Focus: &core.Style{Color: colors.Text()}}}
					}},
				ui.IconOf(chatDisclosure(open.Get), 10, colors.Muted),
			}},
	}}
	detail := core.Show{When: open.Get, Then: func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{MinWidth: core.Pt(0), Margin: core.Edges{Left: 3}, Padding: core.Edges{Left: 11, Bottom: 2},
			Borders: core.Edges{Left: 1}},
			StyleFunc: func() core.Style {
				return core.Style{BorderColors: core.EdgeColors{Left: colors.chatStrongBorder()}, Color: colors.chatSecondary()}
			},
			Children: []core.Node{core.Func(body)}}
	}}
	return core.Show{When: reasoned, Then: func(*core.Scope) core.Node {
		return core.View{TestID: r.TestID, Style: core.Style{MinWidth: core.Pt(0), FlexShrink: 0, Gap: 4},
			Children: []core.Node{row, detail}}
	}}
}
