package chat

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"
)

// ContentWidth is the reading width of transcripts and the composer.
const ContentWidth core.Points = 720

// The composer card's height with one line of draft and no attachments,
// until a glass composer measures it, and how far the edge's progressive
// blur fades out beyond its bars.
const (
	chatRestingCard core.Points = 96
	chatEdgeFade    core.Points = 24
)

// ConversationState is a transcript's scrolling and its composer's
// measurements. Keep one per transcript, and share it with the turns and
// the glass composer in it. The transcript scrolls without virtualizing,
// which keeps selection across turns, so the app bounds its history.
type ConversationState struct {
	scroll                 *core.Ref
	following              *core.Signal[bool]
	composer, card, footer *core.Signal[core.Points]
	lastY                  core.Points
}

// NewConversationState makes a state at the latest turn.
func NewConversationState(cx *core.Scope) *ConversationState {
	return &ConversationState{scroll: &core.Ref{}, following: core.NewSignal(cx, true), composer: core.NewSignal(cx, core.Points(140)),
		card: core.NewSignal(cx, chatRestingCard), footer: core.NewSignal(cx, core.Points(0))}
}

// AtLatest reports whether the transcript follows its end.
func (s *ConversationState) AtLatest() bool { return s.following.Get() }

// JumpToLatest returns to the end and follows it again.
func (s *ConversationState) JumpToLatest() {
	s.following.Set(true)
	s.scroll.ScrollToEnd(false)
}

// ComposerHeight is the room the floating composer takes at the bottom,
// for what the app overlays on an empty conversation.
func (s *ConversationState) ComposerHeight() core.Points { return s.composer.Get() }

// pause stops following until the user returns to the end, so a disclosure
// opening under the pointer keeps its header in place.
func (s *ConversationState) pause() {
	if s != nil {
		s.following.Set(false)
	}
}

// scrolled follows the user: moving up leaves the end, and reaching it
// follows again. The content growing under a following view does not count.
func (s *ConversationState) scrolled(e core.ScrollEvent) {
	end := e.Offset.Y+e.Size.Height >= e.ContentSize.Height-2
	switch {
	case end:
		s.following.Set(true)
	case e.Offset.Y < s.lastY-1:
		s.following.Set(false)
	}
	s.lastY = e.Offset.Y
}

// grown keeps a following transcript at its end.
func (s *ConversationState) grown(core.Bounds) {
	if s.following.GetUntracked() {
		s.scroll.ScrollToEnd(false)
	}
}

// Conversation is a bounded scroll viewport with a jump button, which
// floats over it so showing it never changes the viewport's height. Give
// its parent a bounded height. Label its button with JumpLabel. It follows
// the end, and yields as the user scrolls up.
type Conversation struct {
	State     *ConversationState
	JumpLabel string
	Children  []core.Node
	TestID    string
}

// Render implements core.Node.
func (v Conversation) Render(cx *core.Scope) core.Node {
	s := v.State
	return core.View{TestID: v.TestID, Style: ui.FillRest, Children: []core.Node{
		core.Scroll{Ref: s.scroll, OnScroll: s.scrolled, Style: ui.FillRest, Children: []core.Node{
			chatTranscript(s, core.Edges{Top: 22, Right: 20, Bottom: 22, Left: 20}, v.Children, nil),
		}},
		core.Show{When: func() bool { return !s.AtLatest() }, Then: func(cx *core.Scope) core.Node {
			return chatJumpButton(cx, s, v.JumpLabel, func() core.Points { return ui.ButtonHeight/2 - 6 }, false)
		}},
	}}
}

// chatTranscript centers the turns in the reading width, 48 pt apart.
func chatTranscript(s *ConversationState, pad core.Edges, children []core.Node, clearance func() core.Style) core.Node {
	column := []core.Node{core.View{Style: core.Style{Width: core.Pct(100), MaxWidth: core.Pt(ContentWidth), MinWidth: core.Pt(0), Gap: 48}, Children: children}}
	if clearance != nil {
		column = append(column, core.View{Style: core.Style{FlexShrink: 0}, StyleFunc: clearance})
	}
	return core.View{OnLayout: s.grown, Style: core.Style{MinWidth: core.Pt(0), FlexShrink: 0, AlignItems: core.AlignCenter, Padding: pad}, Children: column}
}

// chatJumpButton is the jump to the latest turn: a 32-point circle, on
// glass over a floating conversation, bottom points up.
func chatJumpButton(cx *core.Scope, s *ConversationState, label string, bottom func() core.Points, glass bool) core.Node {
	colors := tonesOf(cx)
	button := base.Button{Label: label, OnPress: func(core.PressEvent) { s.JumpToLatest() },
		Style: core.Style{Group: "chat-jump", Width: core.Pt(32), Height: core.Pt(32), CornerRadius: 16, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Cursor: core.CursorPointer},
		StyleFunc: func() core.Style {
			st := core.Style{Focus: ui.FocusRing(cx, 2)}
			if glass {
				shadow := ui.Black(0.08)
				if cx.Dark() {
					shadow = ui.Black(0.22)
				}
				st = st.Merge(chatGlass(ui.Alpha(colors.Background(), 0.75)))
				st.Hover = &core.Style{Background: ui.Alpha(colors.Background(), 0.85)}
				st.Shadows = []core.BoxShadow{ui.Shadow(0, 1, 2, 0, shadow)}
				return st
			}
			st.Background = colors.Hover()
			return st
		},
		Children: []core.Node{ui.Icon(lucide.ArrowDown, 16, colors.Text)}}
	return core.View{PointerEvents: core.PointerEventsBoxNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), AlignItems: core.AlignCenter},
		StyleFunc: func() core.Style { return core.Style{Bottom: core.Pt(bottom())} },
		Children:  []core.Node{ui.Tooltip{Text: label, Trigger: button}}}
}

// chatGlass is the composer's surface: Liquid Glass where the system has
// it, else a blur of what is behind it, under tint.
func chatGlass(tint core.Color) core.Style {
	return core.Style{Background: tint, BackdropFilters: []core.Filter{core.Blur(24)}, GlassEffect: core.GlassRegular}
}

// chatGlassEdge is the border of a glass surface: none where the system's
// glass draws its own rim, keeping the surface's size, else rim.
func chatGlassEdge(cx *core.Scope, rim core.Color) core.Color {
	switch cx.Env().OS() {
	case core.OSMacOS, core.OSIOS, core.OSWindows:
		return core.Transparent
	}
	return rim
}

// FloatingConversation lets the transcript scroll behind the composer,
// leaving trailing room so the latest turn stays clear of it. The
// transcript blurs progressively from the bottom edge up to the middle of
// the composer card at rest, without following the card as it grows. Give
// its parent a bounded height; Composer typically builds a Composer
// with Glass set. It manages no title bar: TopInset clears a bar the host
// overlays, which the transcript scrolls under.
type FloatingConversation struct {
	State     *ConversationState
	JumpLabel string
	Children  []core.Node
	Composer  []core.Node
	TopInset  core.Points
	TestID    string
}

// Render implements core.Node.
func (v FloatingConversation) Render(cx *core.Scope) core.Node {
	s := v.State
	env := cx.Env()
	inset := max(0, v.TopInset)
	transparent := core.Style{Background: core.Transparent}
	// Only the scroll view clips: the composer's glass and shadow reach past
	// its margins.
	return core.View{TestID: v.TestID, Style: ui.FillRest, Children: []core.Node{
		// A transparent background keeps the edge a bare blur: its usual tint
		// would wash out the transcript.
		core.Scroll{Ref: s.scroll, OnScroll: s.scrolled, EdgeEffects: core.EdgeEffects{Bottom: core.EdgeEffectSoft}, TestID: "chat-transcript",
			Behavior: core.ScrollBehavior{KeyboardDismissMode: core.KeyboardDismissInteractive},
			Style:    ui.FillRest.Merge(transparent), Children: []core.Node{
				chatTranscript(s, core.Edges{Top: 22 + inset, Right: 20, Left: 20}, v.Children, func() core.Style {
					return core.Style{Height: core.Pt(s.composer.Get() + 22)}
				}),
			}},
		// The transcript blurs behind the footer and the card's lower half.
		core.View{EdgeBar: core.EdgeBar{Scroll: s.scroll, Side: core.SideBottom}, PointerEvents: core.PointerEventsNone,
			Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)},
			StyleFunc: func() core.Style {
				return core.Style{Height: core.Pt(max(0, 8+s.footer.Get()+s.card.Get()/2-chatEdgeFade))}
			}},
		core.Show{When: func() bool { return !s.AtLatest() }, Then: func(cx *core.Scope) core.Node {
			return chatJumpButton(cx, s, v.JumpLabel, func() core.Points { return s.composer.Get() + 12 }, true)
		}},
		core.View{PointerEvents: core.PointerEventsBoxNone, OnLayout: func(b core.Bounds) { s.composer.Set(b.Size.Height) },
			Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), AlignItems: core.AlignCenter},
			StyleFunc: func() core.Style {
				bottom := core.Points(8)
				if env.KeyboardHeight() == 0 {
					bottom += env.SafeArea().Bottom
				}
				return core.Style{Padding: core.Edges{Right: 20, Bottom: bottom, Left: 20}}
			},
			Children: []core.Node{core.View{Style: core.Style{Width: core.Pct(100), MaxWidth: core.Pt(ContentWidth), MinWidth: core.Pt(0)}, Children: v.Composer}}},
	}}
}

// GlassPanel is a compact card on the composer's glass, for status the
// host floats over a conversation, such as command output or errors. Its
// height follows its content.
type GlassPanel struct {
	Children []core.Node
	TestID   string
}

// Render implements core.Node.
func (p GlassPanel) Render(cx *core.Scope) core.Node {
	colors := tonesOf(cx)
	return core.View{TestID: p.TestID, Style: core.Style{MinWidth: core.Pt(0), Padding: core.Symmetric(8, 12), CornerRadius: 8, BorderWidth: 1},
		StyleFunc: func() core.Style {
			return chatGlass(ui.Alpha(colors.Surface(), 0.6)).Merge(core.Style{BorderColor: chatGlassEdge(cx, colors.Border())})
		}, Children: p.Children}
}
