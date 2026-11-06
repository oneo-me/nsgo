package chat

import (
	"strings"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

// Composer is a multiline editor card with send and stop and a row of
// the app's controls. Return sends on desktops and Shift+Return inserts a
// line; phones keep Return for lines. While Running, Stop shows, and a
// nonempty draft shows Send instead so the app can interrupt the run with
// the new message. Labels are the app's; it clears the draft in OnSend and
// owns cancelling in OnStop.
type Composer struct {
	Draft                             func() string
	OnDraftChange                     func(string)
	Label                             string
	Placeholder, SendLabel, StopLabel string
	// PlaceholderFunc replaces Placeholder and may change.
	PlaceholderFunc func() string
	Running         func() bool
	OnSend, OnStop  func()
	// Accessory holds controls in the card's bottom row, such as chips, and
	// shrinks; Trailing compact ones just before Send or Stop, which do not.
	Accessory, Trailing View
	// Attachments shows above the editor while HasAttachments is true.
	Attachments    View
	HasAttachments func() bool
	// Footer shows below the card.
	Footer View
	// OnDropFiles makes the card take files dropped from other apps. It
	// gets their URLs; reading them is the app's.
	OnDropFiles func(uris []string)
	// OnPaste hears what the user pastes, images and files included.
	OnPaste func(core.PasteEvent)
	// SendDisabled holds Send while attachments are prepared, and
	// StopDisabled keeps Stop shown but inactive. Editing stays on.
	SendDisabled, StopDisabled func() bool
	// Glass puts the card on glass, for a FloatingConversation; its
	// State measures it, for the edge's blur. The footer stays off the glass
	// so the card stays light.
	Glass  bool
	State  *ConversationState
	TestID string
}

// Render implements core.Node.
func (m Composer) Render(cx *core.Scope) core.Node {
	colors := tonesOf(cx)
	draft := func() string {
		if m.Draft == nil {
			return ""
		}
		return m.Draft()
	}
	empty := func() bool { return strings.TrimSpace(draft()) == "" }
	running := func() bool { return is(m.Running) }
	canSend := func() bool { return !empty() && !is(m.SendDisabled) }
	send := func() {
		if m.OnSend != nil && core.Untracked(cx, canSend) {
			m.OnSend()
		}
	}
	focused, dragging := core.NewSignal(cx, false), core.NewSignal(cx, false)
	height := core.NewSignal(cx, core.Points(chatEditorLine))
	submit := core.SubmitBehaviorNewline
	if cx.Env().Desktop() {
		submit = core.SubmitBehaviorSubmit
	}
	editor := base.Input{Value: draft, OnChange: m.OnDraftChange, Multiline: true, SubmitBehavior: submit, Label: m.Label, Placeholder: m.Placeholder, PlaceholderFunc: m.PlaceholderFunc, TestID: "composer",
		OnSubmit:            func(string) { send() },
		OnPaste:             m.OnPaste,
		OnFocus:             func() { focused.Set(true) },
		OnBlur:              func() { focused.Set(false) },
		OnContentSizeChange: func(s core.Size) { height.Set(min(10*chatEditorLine, max(chatEditorLine, s.Height))) },
		Style:               core.Style{Padding: core.Edges{Top: 2, Right: 14, Left: 14}, MinHeight: core.Pt(40), FontSize: 13.5, LineHeight: chatEditorLine, TextAlignVertical: core.TextAlignVerticalTop},
		StyleFunc: func() core.Style {
			return core.Style{Height: core.Pt(height.Get() + 2), Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: cx.Palette().Selection, CaretColor: colors.Accent()}
		}}
	var row []core.Node
	var accessory []core.Node
	if m.Accessory != nil {
		accessory = append(accessory, core.Func(m.Accessory))
	}
	row = append(row, core.View{Style: core.Style{Direction: core.Row, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), AlignItems: core.AlignCenter, Gap: 4}, Children: accessory})
	if m.Trailing != nil {
		row = append(row, core.View{Style: core.Style{Direction: core.Row, FlexShrink: 0, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{core.Func(m.Trailing)}})
	}
	row = append(row, core.Dynamic(func(cx *core.Scope) core.Node {
		if running() && empty() {
			return chatPill{Label: m.StopLabel, Disabled: m.StopDisabled, OnPress: m.OnStop, Background: func() core.Color {
				if is(m.StopDisabled) {
					return colors.chatOverlayStrong()
				}
				return ui.Alpha(colors.Danger(), 0.14)
			}, Foreground: func() core.Color {
				if is(m.StopDisabled) {
					return colors.Muted()
				}
				return colors.Danger()
			}}
		}
		return chatPill{Label: m.SendLabel, Disabled: func() bool { return !canSend() }, OnPress: send, Background: func() core.Color {
			if canSend() {
				return colors.AccentFill()
			}
			return colors.chatOverlayStrong()
		}, Foreground: func() core.Color {
			if canSend() {
				return colors.OnAccent()
			}
			return colors.Muted()
		}}
	}))
	var body []core.Node
	if m.Attachments != nil {
		body = append(body, core.Show{When: func() bool { return is(m.HasAttachments) }, Then: func(*core.Scope) core.Node {
			return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 6, Padding: core.Edges{Right: 10, Bottom: 6, Left: 10}}, Children: []core.Node{core.Func(m.Attachments)}}
		}})
	}
	body = append(body, editor, core.View{Style: core.Style{Direction: core.Row, MinWidth: core.Pt(0), Margin: core.Edges{Top: 8}, Padding: core.Symmetric(0, 10), AlignItems: core.AlignCenter, Gap: 8}, Children: row})
	card := core.View{Style: core.Style{MinWidth: core.Pt(0), Padding: core.Symmetric(10, 0), CornerRadius: 13, BorderWidth: 1, FlexShrink: 0},
		StyleFunc: func() core.Style {
			surface := colors.Surface()
			if dragging.Get() {
				surface = ui.Mix(surface, colors.Text(), 0.09)
			}
			border := colors.Border()
			switch {
			case dragging.Get():
				border = ui.Alpha(colors.Accent(), 0.7)
			case focused.Get():
				border = colors.chatStrongBorder()
			}
			s := core.Style{Background: surface, BorderColor: border, Shadows: []core.BoxShadow{ui.Shadow(0, 2, 8, 0, ui.Black(0.04))}}
			if m.Glass {
				s = s.Merge(chatGlass(ui.Alpha(surface, 0.6)))
				// A dragged file still lights the edge the glass otherwise draws.
				if !dragging.Get() {
					s.BorderColor = chatGlassEdge(cx, border)
				}
			}
			return s
		}, Children: body}
	if m.State != nil {
		// Only a single empty line with no attachments shows the card at rest.
		card.OnLayout = func(b core.Bounds) {
			if core.Untracked(cx, func() bool { return draft() == "" && !is(m.HasAttachments) }) {
				m.State.card.Set(b.Size.Height)
			}
		}
		if m.Footer != nil {
			m.State.footer.Set(28)
		} else {
			m.State.footer.Set(0)
		}
	}
	if m.OnDropFiles != nil {
		card.OnFileDragEnter = func() { dragging.Set(true) }
		card.OnFileDragLeave = func() { dragging.Set(false) }
		card.OnFileDrop = func(uris []string) { dragging.Set(false); m.OnDropFiles(uris) }
	}
	children := []core.Node{card}
	if m.Footer != nil {
		children = append(children, core.View{Style: core.Style{Direction: core.Row, MinWidth: core.Pt(0), Height: core.Pt(28), Padding: core.Edges{Top: 4, Right: 10, Left: 10}, AlignItems: core.AlignCenter, Gap: 2},
			Children: []core.Node{core.Func(m.Footer)}})
	}
	return core.View{TestID: m.TestID, Style: core.Style{MinWidth: core.Pt(0), FlexShrink: 0}, Children: children}
}

// chatEditorLine is a line of the composer's editor.
const chatEditorLine core.Points = 20

// chatPill is Send or Stop: a capsule of text.
type chatPill struct {
	Label                  string
	Disabled               func() bool
	OnPress                func()
	Background, Foreground func() core.Color
}

// Render implements core.Node.
func (p chatPill) Render(cx *core.Scope) core.Node {
	return base.Button{Label: p.Label, Disabled: p.Disabled, OnPress: func(core.PressEvent) {
		if p.OnPress != nil {
			p.OnPress()
		}
	},
		Style: core.Style{Height: core.Pt(26), Padding: core.Symmetric(0, 12), FlexShrink: 0, CornerRadius: 13, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			Disabled: &core.Style{Cursor: core.CursorDefault}},
		StyleFunc: func() core.Style {
			return core.Style{Background: p.Background(), Hover: &core.Style{Opacity: core.Alpha(0.9)}, Active: &core.Style{Opacity: core.Alpha(0.8)}, Focus: ui.FocusRing(cx, 2)}
		},
		Children: []core.Node{ui.Text(p.Label, 12.5, ui.Semibold, p.Foreground)}}
}

// IconAction is a named icon button with a tooltip, for a turn's
// actions or the composer's slots. Active marks a choice in Tint, such as
// a rating; Icon may follow it through IconFunc.
type IconAction struct {
	Label    string
	Icon     func() *core.SVG
	IconFunc func() *core.SVG
	Active   func() bool
	Tint     func() core.Color
	Disabled func() bool
	OnPress  func()
	TestID   string
}

// Render implements core.Node.
func (a IconAction) Render(cx *core.Scope) core.Node {
	colors := tonesOf(cx)
	name := a.IconFunc
	if name == nil {
		name = a.Icon
	}
	tint := func() core.Color {
		if is(a.Active) && a.Tint != nil {
			return a.Tint()
		}
		return colors.Muted()
	}
	button := base.Button{Label: a.Label, Selected: a.Active, Disabled: a.Disabled, TestID: a.TestID, OnPress: func(core.PressEvent) {
		if a.OnPress != nil {
			a.OnPress()
		}
	},
		Style: core.Style{Group: "chat-action", Width: core.Pt(26), Height: core.Pt(26), FlexShrink: 0, CornerRadius: 6, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			Disabled: ui.Dimmed()},
		StyleFunc: func() core.Style {
			return core.Style{Hover: &core.Style{Background: colors.chatOverlay()}, Focus: ui.FocusRing(cx, 2)}
		},
		Children: []core.Node{core.View{StyleFunc: func() core.Style {
			return core.Style{Color: tint(), GroupStyle: &core.GroupStyle{Name: "chat-action", Hover: &core.Style{Color: colors.Text()}}}
		}, Children: []core.Node{ui.IconOf(name, 14, ui.Inherited)}}}}
	return ui.Tooltip{Text: a.Label, Trigger: button}
}
