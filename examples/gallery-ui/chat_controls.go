package ui

import (
	"github.com/oneo-me/nsgo/icons/lucide"
	"math"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// ChatAttachmentPreview is a 64-point image tile with an optional remove
// action, for rows of images alone; ChatAttachment keeps mixed rows at one
// height. Label names the image for assistive technology and its tooltip,
// and an empty RemoveLabel makes it read-only. Without Image it shows a
// placeholder: loading and decoding are the app's.
type ChatAttachmentPreview struct {
	Label, RemoveLabel string
	Image              core.ImageSource
	OnRemove           func()
}

// Render implements core.Node.
func (a ChatAttachmentPreview) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var inner core.Node
	if hasImage(a.Image) {
		inner = core.Image{Source: a.Image, ResizeMode: core.Cover, Label: a.Label, Style: core.Style{Width: core.Pt(62), Height: core.Pt(62)}}
	} else {
		inner = core.View{Label: a.Label, Style: core.Style{Width: core.Pt(62), Height: core.Pt(62), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{Icon(lucide.Image, 20, colors.Muted)}}
	}
	children := []core.Node{inner}
	if a.RemoveLabel != "" {
		children = append(children, base.Button{Label: a.RemoveLabel, OnPress: chatPress(a.OnRemove),
			Style: core.Style{Position: core.Absolute, Top: core.Pt(3), Right: core.Pt(3), Width: core.Pt(18), Height: core.Pt(18), CornerRadius: 5, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				Cursor: core.CursorPointer},
			StyleFunc: func() core.Style {
				return core.Style{Background: Alpha(colors.Background(), 0.9), Hover: &core.Style{Background: colors.Surface(), BorderWidth: 1, BorderColor: colors.Accent()}}
			},
			Children: []core.Node{Icon(lucide.X, 11, colors.Text)}})
	}
	return Tooltip{Text: a.Label, Trigger: core.View{Style: core.Style{Width: core.Pt(64), Height: core.Pt(64), FlexShrink: 0, CornerRadius: 8, Overflow: core.Hidden, BorderWidth: 1},
		StyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.chatStrongBorder(), Background: colors.Surface()}
		}, Children: children}}
}

// ChatAttachment is a compact file chip with an optional remove action, for
// the composer's Attachments or a prompt. Detail is short trailing text,
// such as a size. Image is a thumbnail the app prepared; without one, Icon
// replaces the file icon, such as lucide.Image for an image still on its way.
// An empty RemoveLabel makes it read-only. It reads no files.
type ChatAttachment struct {
	Label, Detail, RemoveLabel string
	Image                      core.ImageSource
	Icon                       func() *core.SVG
	OnRemove                   func()
}

// Render implements core.Node.
func (a ChatAttachment) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var children []core.Node
	if hasImage(a.Image) {
		children = append(children, core.Image{Source: a.Image, ResizeMode: core.Cover, Style: core.Style{Width: core.Pt(22), Height: core.Pt(22), CornerRadius: 4, FlexShrink: 0}})
	} else {
		name := a.Icon
		if name == nil {
			name = lucide.FileText
		}
		children = append(children, core.View{Style: core.Style{Width: core.Pt(22), Height: core.Pt(22), FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{Icon(name, 13, colors.Muted)}})
	}
	label := Text(a.Label, 12.5, Regular, colors.chatSecondary)
	label.MaxLines = 1
	label.Style.MinWidth, label.Style.FlexShrink = core.Pt(0), 1
	children = append(children, label)
	if a.Detail != "" {
		detail := Text(a.Detail, 11.5, Regular, colors.Muted)
		detail.Style.FlexShrink = 0
		children = append(children, detail)
	}
	if a.RemoveLabel == "" {
		children = append(children, core.View{Style: core.Style{Width: core.Pt(2)}})
	} else {
		children = append(children, base.Button{Label: a.RemoveLabel, OnPress: chatPress(a.OnRemove),
			Style: core.Style{Group: "chat-remove", Width: core.Pt(18), Height: core.Pt(18), FlexShrink: 0, CornerRadius: 4, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Cursor: core.CursorPointer},
			StyleFunc: func() core.Style {
				return core.Style{Hover: &core.Style{Background: colors.chatOverlayStrong()}, Focus: FocusRing(cx, 2)}
			},
			Children: []core.Node{Icon(lucide.X, 11, colors.Muted)}})
	}
	return core.View{Style: core.Style{Direction: core.Row, MinWidth: core.Pt(0), MaxWidth: core.Pt(220), Height: core.Pt(30), AlignItems: core.AlignCenter, Gap: 6, Padding: core.Symmetric(0, 4), CornerRadius: 7, BorderWidth: 1},
		StyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.chatStrongBorder(), Background: Mix(colors.Background(), colors.Text(), 0.035)}
		}, Children: children}
}

// chatPress is an optional action as a press handler.
func chatPress(fn func()) func(core.PressEvent) {
	return func(core.PressEvent) {
		if fn != nil {
			fn()
		}
	}
}

// ChatChip is a compact composer or footer control for a setting such as a
// model, reasoning level, project or branch. Its label shortens as the row
// tightens. A chip with OnPress is a button; without one it is a fixed
// value, a label rather than a disabled action.
type ChatChip struct {
	Text string
	// TextFunc replaces Text and may change.
	TextFunc func() string
	Icon     func() *core.SVG
	Image    core.ImageSource
	// Leading replaces Image, which replaces Icon: content at the size
	// it is given, such as a product mark in its own colors.
	Leading func(cx *core.Scope, size core.Points) core.Node
	// Caret marks a chip that opens a choice.
	Caret bool
	// Active keeps the chip lit while what it opens shows.
	Active  func() bool
	OnPress func()
	TestID  string
}

func (p ChatChip) text() func() string {
	if p.TextFunc != nil {
		return p.TextFunc
	}
	return constant(p.Text)
}

// face is the chip's look and content, for a button or a popup's trigger.
func (p ChatChip) face(cx *core.Scope) []core.Node {
	colors := ColorsOf(cx)
	var children []core.Node
	switch {
	case p.Leading != nil:
		children = append(children, p.Leading(cx, 14))
	case hasImage(p.Image):
		children = append(children, core.Image{Source: p.Image, ResizeMode: core.Contain, Style: core.Style{Width: core.Pt(14), Height: core.Pt(14), CornerRadius: 3, FlexShrink: 0}})
	case p.Icon != nil:
		children = append(children, core.View{Style: core.Style{FlexShrink: 0}, Children: []core.Node{Icon(p.Icon, 12, colors.Muted)}})
	}
	label := Textf(p.text(), 12.5, Regular, colors.chatSecondary)
	label.MaxLines = 1
	label.Style.MinWidth, label.Style.FlexShrink = core.Pt(0), 1
	children = append(children, label)
	if p.Caret {
		children = append(children, core.View{Style: core.Style{FlexShrink: 0}, Children: []core.Node{
			Icon(lucide.ChevronDown, 10.5, func() core.Color { return Alpha(colors.Muted(), 0.7) })}})
	}
	return children
}

// shape is the chip's size and how it fills.
func (p ChatChip) shape(cx *core.Scope) (core.Style, func() core.Style) {
	return core.Style{Direction: core.Row, Height: core.Pt(26), MinWidth: core.Pt(0), FlexShrink: 1, Padding: core.Symmetric(0, 7), CornerRadius: 6, AlignItems: core.AlignCenter, Gap: 6,
			Cursor: core.CursorPointer, Disabled: &core.Style{Opacity: core.Alpha(0.7)}},
		func() core.Style {
			return chatTriggerStates(cx, is(p.Active))
		}
}

// Render implements core.Node.
func (p ChatChip) Render(cx *core.Scope) core.Node {
	if p.OnPress == nil {
		return core.View{TestID: p.TestID, Label: p.Text, Style: core.Style{Direction: core.Row, Height: core.Pt(26), FlexShrink: 0, Padding: core.Symmetric(0, 7), AlignItems: core.AlignCenter, Gap: 6},
			Children: p.face(cx)}
	}
	style, states := p.shape(cx)
	return base.Button{TestID: p.TestID, Label: p.Text, OnPress: chatPress(p.OnPress), Style: style, StyleFunc: states, Children: p.face(cx)}
}

// ChatOption is one choice of a ChatPicker. ID is stable and nonempty.
// Group gathers consecutive options under a heading; Detail is trailing
// text such as a context size, and Description a muted second line. Leading
// replaces Image, which replaces Icon, and when any option has one the
// others keep its room.
type ChatOption struct {
	ID, Label, Detail, Description, Group string
	Icon                                  func() *core.SVG
	Image                                 core.ImageSource
	Leading                               func(cx *core.Scope, size core.Points) core.Node
}

// ChatPicker is a chip that opens a popover of grouped, optionally
// searchable options, with a check on the selection. Choosing closes it
// and calls OnChange, for the app to apply.
type ChatPicker struct {
	// Label is the chip's text and name.
	Label func() string
	Icon  func() *core.SVG
	Image core.ImageSource
	// Leading leads the chip as ChatChip's does.
	Leading  func(cx *core.Scope, size core.Points) core.Node
	Options  []ChatOption
	Selected func() string
	OnChange func(id string)
	// SearchLabel lets the user filter once there are more than Searchable
	// options, 8 when 0; EmptyText shows when nothing matches.
	SearchLabel, EmptyText string
	Searchable             int
	// Width is the popover's, 260 when 0.
	Width  core.Points
	TestID string
}

// Render implements core.Node.
func (p ChatPicker) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	open, query := core.NewSignal(cx, false), core.NewSignal(cx, "")
	width := p.Width
	if width <= 0 {
		width = 260
	}
	threshold := p.Searchable
	if threshold <= 0 {
		threshold = 8
	}
	leading := false
	for _, o := range p.Options {
		leading = leading || o.Leading != nil || hasImage(o.Image) || o.Icon != nil
	}
	chip := ChatChip{TextFunc: p.Label, Icon: p.Icon, Image: p.Image, Leading: p.Leading, Caret: true, Active: open.Get}
	style, states := chip.shape(cx)
	visible := func() []ChatOption {
		q := strings.ToLower(strings.TrimSpace(query.Get()))
		var out []ChatOption
		for _, o := range p.Options {
			if q == "" || strings.Contains(strings.ToLower(o.Label+" "+o.Group+" "+o.Detail+" "+o.ID), q) {
				out = append(out, o)
			}
		}
		return out
	}
	var body []core.Node
	if p.SearchLabel != "" && len(p.Options) > threshold {
		body = append(body, core.View{Style: core.Style{Padding: core.Edges{Top: 8, Right: 8, Left: 8}}, Children: []core.Node{
			Input{Input: base.Input{Value: query.Get, OnChange: query.Set, Placeholder: p.SearchLabel, Label: p.SearchLabel, AutoFocus: true}},
		}})
	}
	body = append(body, core.Dynamic(func(cx *core.Scope) core.Node {
		options := visible()
		if len(options) == 0 {
			empty := Text(p.EmptyText, 12.5, Regular, colors.Muted)
			empty.Style.Padding = core.All(12)
			return empty
		}
		var rows []core.Node
		group := ""
		for i, o := range options {
			if o.Group != "" && (i == 0 || o.Group != group) {
				head := Text(o.Group, 11.5, Medium, colors.Muted)
				head.Style.Padding = core.Edges{Top: 8, Right: 8, Bottom: 3, Left: 8}
				rows = append(rows, head)
			}
			group = o.Group
			rows = append(rows, chatPickerRow(cx, o, leading, func() bool { return p.Selected != nil && p.Selected() == o.ID }, func() {
				open.Set(false)
				if p.OnChange != nil {
					p.OnChange(o.ID)
				}
			}))
		}
		return core.Scroll{Style: core.Style{MaxHeight: core.Pt(348), FlexShrink: 1}, Children: []core.Node{core.View{Style: core.Style{Padding: core.All(4), Gap: 1}, Children: rows}}}
	}))
	return base.Popover{Open: open.Get, OnOpenChange: func(v bool, _ base.Reason) { open.Set(v); query.Set("") }, Placement: core.PlaceTop, Align: core.AlignStart, Offset: 4, Margin: 8,
		Style: style, StyleFunc: states, Trigger: core.Group(chip.face(cx)), Label: core.Untracked(cx, p.Label), TestID: p.TestID,
		PopupStyle:     core.Style{Width: core.Pt(width), MaxWidth: core.Pct(100)},
		PopupStyleFunc: chatPopup(cx), Children: body}
}

// chatPopup is what a chat control's popover is drawn on: glass under the
// page's color. It clips nothing, as a clip would take its shadow.
func chatPopup(cx *core.Scope) func() core.Style {
	return func() core.Style {
		colors := ColorsOf(cx)
		menu := FinOf(cx).Menu()
		return chatGlass(Alpha(colors.Background(), 0.92)).Merge(core.Style{CornerRadius: RadiusCard, BorderWidth: 1, BorderColor: chatGlassEdge(cx, menu.Border), Shadows: menu.Shadows, Color: colors.Text()})
	}
}

func chatPickerRow(cx *core.Scope, o ChatOption, leading bool, selected func() bool, choose func()) core.Node {
	colors := ColorsOf(cx)
	var children []core.Node
	switch {
	case o.Leading != nil:
		children = append(children, o.Leading(cx, 16))
	case hasImage(o.Image):
		children = append(children, core.Image{Source: o.Image, ResizeMode: core.Contain, Style: core.Style{Width: core.Pt(16), Height: core.Pt(16), CornerRadius: 3, FlexShrink: 0}})
	case o.Icon != nil:
		children = append(children, core.View{Style: core.Style{Width: core.Pt(16), Height: core.Pt(16), FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{Icon(o.Icon, 13, colors.chatSecondary)}})
	case leading:
		children = append(children, core.View{Style: core.Style{Width: core.Pt(16), Height: core.Pt(16), FlexShrink: 0}})
	}
	label := Text(o.Label, 13, Regular, colors.Text)
	label.MaxLines = 1
	text := []core.Node{label}
	if o.Description != "" {
		text = append(text, Text(o.Description, 11.5, Regular, colors.Muted))
	}
	children = append(children, core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 1}, Children: text})
	if o.Detail != "" {
		detail := Text(o.Detail, 11.5, Regular, colors.Muted)
		detail.Style.FlexShrink = 0
		children = append(children, detail)
	}
	children = append(children, core.View{Style: core.Style{Width: core.Pt(12), Height: core.Pt(12), FlexShrink: 0}, Children: []core.Node{
		core.Show{When: selected, Then: func(*core.Scope) core.Node { return Icon(lucide.Check, 12, colors.Accent) }},
	}})
	return base.Button{Label: o.Label, Selected: selected, OnPress: chatPress(choose),
		Style: core.Style{Direction: core.Row, MinWidth: core.Pt(0), Padding: core.Symmetric(6, 8), CornerRadius: 6, AlignItems: core.AlignCenter, Gap: 8, Cursor: core.CursorPointer},
		StyleFunc: func() core.Style {
			return core.Style{Hover: &core.Style{Background: colors.chatOverlay()}, Focus: &core.Style{Background: colors.chatOverlay()}}
		},
		Children: children}
}

// ChatContextMeter is a small ring gauge for the composer's Trailing, that
// opens a panel with the value and a bar. It fills with the accent, warns
// from 80% and alerts from 95%; the app measures use. Fraction is 0 to 1,
// or negative with nothing measured. Label is its name and Tooltip its
// hover text; Title and Value head the panel, such as "Context window" and
// "61k / 272k (22%)".
type ChatContextMeter struct {
	Label, Title   string
	Tooltip, Value func() string
	Fraction       func() float64
	TestID         string
}

// Render implements core.Node.
func (m ChatContextMeter) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	open := core.NewSignal(cx, false)
	fill := func() core.Color { return chatMeterColor(cx, m.Fraction()) }
	head := Text(m.Title, 12.5, Regular, colors.Text)
	head.MaxLines = 1
	head.Style.FlexGrow, head.Style.FlexShrink, head.Style.MinWidth = 1, 1, core.Pt(0)
	value := Textf(m.Value, 12.5, Regular, colors.Muted)
	value.Style.FlexShrink = 0
	trigger := base.Popover{Open: open.Get, OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) }, Placement: core.PlaceTop, Align: core.AlignEnd, Offset: 4, Margin: 8,
		Style:     core.Style{Height: core.Pt(20), Padding: core.Symmetric(0, 5), CornerRadius: 5, FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Cursor: core.CursorPointer},
		StyleFunc: chatMeterStates(cx, open.Get), Label: m.Label, TestID: m.TestID, PopupStyleFunc: chatPopup(cx),
		PopupStyle: core.Style{Width: core.Pt(320), MaxWidth: core.Pct(100), Padding: core.All(14), Gap: 7},
		Trigger:    chatGauge(cx, m.Fraction, fill),
		Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, MinWidth: core.Pt(0), AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{head, value}},
			chatBar(cx, m.Fraction, fill, core.Style{Height: core.Pt(6)}),
		}}
	if m.Tooltip != nil {
		trigger.Tooltip = chatTooltip(cx, m.Tooltip)
	}
	return trigger
}

// chatTooltip is a popup trigger's tooltip, whose text may change. It is
// the trigger's own, so the trigger stays as it opens.
func chatTooltip(cx *core.Scope, text func() string) core.Node {
	colors := ColorsOf(cx)
	return core.View{Style: core.Style{CornerRadius: radiusSmall, Padding: core.Symmetric(4, 8), MaxWidth: core.Pt(280)},
		StyleFunc: func() core.Style { return core.Style{Background: colors.Text(), Shadows: FinOf(cx).Raised().Shadows} },
		Children:  []core.Node{core.Text{ContentFunc: text, Style: typeCaption, StyleFunc: Colored(colors.Background)}}}
}

// chatMeterStates fills a meter's trigger as it opens and under the pointer.
func chatMeterStates(cx *core.Scope, open func() bool) func() core.Style {
	return func() core.Style { return chatTriggerStates(cx, open()) }
}

// chatTriggerStates fills a popup's trigger. Hover stays while it is open,
// as a view without one stops hearing the pointer leave and stays hovered.
func chatTriggerStates(cx *core.Scope, open bool) core.Style {
	colors := ColorsOf(cx)
	fill := colors.chatOverlay()
	s := core.Style{Focus: FocusRing(cx, 2)}
	if open {
		fill = colors.chatOverlayStrong()
		s.Background = fill
	}
	s.Hover = &core.Style{Background: fill}
	return s
}

func chatMeterColor(cx *core.Scope, fraction float64) core.Color {
	colors := ColorsOf(cx)
	switch {
	case fraction >= 0.95:
		return colors.Danger()
	case fraction >= 0.8:
		return colors.Warn()
	}
	return colors.Accent()
}

// chatBar is a track filled to fraction, negative for the track alone.
func chatBar(cx *core.Scope, fraction func() float64, fill func() core.Color, s core.Style) core.Node {
	colors := ColorsOf(cx)
	return core.View{Style: core.Style{FlexShrink: 0, Overflow: core.Hidden, CornerRadius: 3}.Merge(s), StyleFunc: Filled(colors.Border), Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pct(100), CornerRadius: 3}, StyleFunc: func() core.Style {
			return core.Style{Width: core.Pct(float32(100 * min(max(fraction(), 0), 1))), Background: fill()}
		}},
	}}
}

// chatGauge is a ring on the theme's border, clockwise from twelve o'clock.
func chatGauge(cx *core.Scope, fraction func() float64, fill func() core.Color) core.Node {
	colors := ColorsOf(cx)
	return core.Canvas{Style: core.Style{Width: core.Pt(13), Height: core.Pt(13), FlexShrink: 0}, Draw: func(d *core.Drawing, s core.Size) {
		const stroke = 2.5
		r := (float32(s.Width) - stroke) / 2
		x, y := float32(s.Width)/2, float32(s.Height)/2
		d.SetLineWidth(stroke)
		d.BeginPath().Arc(x, y, r, 0, 2*math.Pi, false).SetStrokeColor(colors.Border()).Stroke()
		if f := fraction(); f > 0 {
			start := float32(-math.Pi / 2)
			d.BeginPath().Arc(x, y, r, start, start+float32(min(f, 1))*2*math.Pi, false).SetStrokeColor(fill()).Stroke()
		}
	}}
}

// ChatQuotaRow is one account limit, such as Label "5-hour limit", Detail
// "Resets in 2h" and Value "63% left". Fraction is the share that remains,
// 0 to 1; a negative one omits the bar, as for a balance.
type ChatQuotaRow struct {
	Label, Detail, Value string
	Fraction             float64
}

// ChatQuota lists an account's limits with accent bars of what remains,
// warning from 20% left and alerting from 5%. The header ends in Countdown,
// such as the time to the next read ("04:59"), a refresh action when
// RefreshLabel is set, which Refreshing disables, and an open action when
// OnOpen's OpenLabel is. Note explains missing or stale rows, and Loading
// shows placeholders until rows are read. Reading usage is the app's.
type ChatQuota struct {
	ID, Title, Note         string
	Countdown               func() string
	RefreshLabel, OpenLabel string
	Refreshing, Loading     func() bool
	Leading                 func(cx *core.Scope, size core.Points) core.Node
	Trailing                ChatView
	Rows                    func() []ChatQuotaRow
	OnRefresh, OnOpen       func()
}

// Render implements core.Node.
func (q ChatQuota) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var head []core.Node
	if q.Leading != nil {
		head = append(head, q.Leading(cx, 13))
	}
	title := Text(q.Title, 12.5, Regular, colors.Muted)
	title.MaxLines = 1
	title.Style.FlexGrow, title.Style.FlexShrink, title.Style.MinWidth = 1, 1, core.Pt(0)
	head = append(head, title)
	if q.Countdown != nil {
		countdown := Tabular(Textf(q.Countdown, 11.5, Regular, colors.Muted))
		countdown.MaxLines = 1
		countdown.Style.FlexShrink = 0
		head = append(head, countdown)
	}
	if q.RefreshLabel != "" {
		head = append(head, chatQuotaAction(cx, q.RefreshLabel, lucide.RefreshCw, q.Refreshing, q.OnRefresh))
	}
	if q.OpenLabel != "" {
		head = append(head, chatQuotaAction(cx, q.OpenLabel, lucide.ExternalLink, nil, q.OnOpen))
	}
	if q.Trailing != nil {
		head = append(head, core.Func(q.Trailing))
	}
	rows := func() []ChatQuotaRow {
		if q.Rows == nil {
			return nil
		}
		return q.Rows()
	}
	children := []core.Node{
		core.View{Style: core.Style{Direction: core.Row, MinWidth: core.Pt(0), Height: core.Pt(20), AlignItems: core.AlignCenter, Gap: 6}, Children: head},
		core.Dynamic(func(cx *core.Scope) core.Node {
			var out []core.Node
			for _, row := range rows() {
				out = append(out, chatQuotaRow(cx, row))
			}
			if is(q.Loading) && len(out) == 0 {
				for range 2 {
					out = append(out, core.View{Style: core.Style{Height: core.Pt(6), CornerRadius: 3}, StyleFunc: Filled(colors.chatOverlay)})
				}
			}
			return core.View{Style: core.Style{MinWidth: core.Pt(0), Gap: 7}, Children: out}
		}),
	}
	if q.Note != "" {
		children = append(children, Text(q.Note, 12, Regular, colors.Muted))
	}
	return core.View{Style: core.Style{MinWidth: core.Pt(0), Gap: 7}, Children: children}
}

func chatQuotaRow(cx *core.Scope, row ChatQuotaRow) core.Node {
	colors := ColorsOf(cx)
	label := Text(row.Label, 12.5, Regular, colors.Text)
	label.MaxLines = 1
	label.Style.MinWidth, label.Style.FlexShrink = core.Pt(0), 1
	left := []core.Node{label}
	if row.Detail != "" {
		detail := Text(row.Detail, 11.5, Regular, colors.Muted)
		detail.Style.FlexShrink = 0
		left = append(left, detail)
	}
	value := Text(row.Value, 12.5, Regular, colors.chatSecondary)
	value.Style.FlexShrink = 0
	children := []core.Node{core.View{Style: core.Style{Direction: core.Row, MinWidth: core.Pt(0), AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), AlignItems: core.AlignCenter, Gap: 6}, Children: left},
		value,
	}}}
	if row.Fraction >= 0 {
		children = append(children, chatBar(cx, func() float64 { return row.Fraction }, func() core.Color { return chatQuotaColor(cx, row.Fraction) }, core.Style{Height: core.Pt(6)}))
	}
	return core.View{Style: core.Style{MinWidth: core.Pt(0), Gap: 7}, Children: children}
}

// chatQuotaAction is a 20-point icon button with a tooltip in the header.
func chatQuotaAction(cx *core.Scope, label string, icon func() *core.SVG, disabled func() bool, press func()) core.Node {
	colors := ColorsOf(cx)
	return Tooltip{Text: label, Trigger: base.Button{Label: label, Disabled: disabled, OnPress: chatPress(press),
		Style: core.Style{Width: core.Pt(20), Height: core.Pt(20), FlexShrink: 0, CornerRadius: 4, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Cursor: core.CursorPointer, Disabled: dimmed()},
		StyleFunc: func() core.Style {
			return core.Style{Hover: &core.Style{Background: colors.chatOverlay()}, Focus: FocusRing(cx, 2)}
		},
		Children: []core.Node{Icon(icon, 13, colors.Muted)}}}
}

func chatQuotaColor(cx *core.Scope, remaining float64) core.Color {
	colors := ColorsOf(cx)
	switch {
	case remaining <= 0.05:
		return colors.Danger()
	case remaining <= 0.2:
		return colors.Warn()
	}
	return colors.Accent()
}

// ChatQuotaSummary is a compact footer control showing Row's Detail, such
// as the time to reset, and a small bar of what remains, or Value for a
// balance. Pressing it opens a panel with every Quota. The app chooses the
// row, usually the limit that applies to the chosen model. Label names it
// and Tooltip is its hover text.
type ChatQuotaSummary struct {
	Label   string
	Tooltip func() string
	Row     func() ChatQuotaRow
	Quotas  []ChatQuota
	TestID  string
}

// Render implements core.Node.
func (q ChatQuotaSummary) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	open := core.NewSignal(cx, false)
	face := core.Dynamic(func(cx *core.Scope) core.Node {
		row := q.Row()
		if row.Fraction < 0 {
			return Text(row.Value, 11.5, Regular, colors.Muted)
		}
		var nodes []core.Node
		if row.Detail != "" {
			nodes = append(nodes, Text(row.Detail, 11.5, Regular, colors.Muted))
		}
		return append(core.Group(nodes), chatBar(cx, func() float64 { return row.Fraction }, colors.Accent, core.Style{Width: core.Pt(48), Height: core.Pt(4), CornerRadius: 2}))
	})
	var panel []core.Node
	for _, quota := range q.Quotas {
		panel = append(panel, quota)
	}
	trigger := base.Popover{Open: open.Get, OnOpenChange: func(v bool, _ base.Reason) { open.Set(v) }, Placement: core.PlaceTop, Align: core.AlignEnd, Offset: 4, Margin: 8,
		Style:     core.Style{Direction: core.Row, Height: core.Pt(20), Padding: core.Symmetric(0, 5), CornerRadius: 5, FlexShrink: 0, AlignItems: core.AlignCenter, Gap: 6, Cursor: core.CursorPointer},
		StyleFunc: chatMeterStates(cx, open.Get), Label: q.Label, TestID: q.TestID, Trigger: face, PopupStyleFunc: chatPopup(cx),
		PopupStyle: core.Style{Width: core.Pt(320), MaxWidth: core.Pct(100), Padding: core.All(14), Gap: 14}, Children: panel}
	if q.Tooltip != nil {
		trigger.Tooltip = chatTooltip(cx, q.Tooltip)
	}
	return trigger
}

// ChatSummaryItem is an artifact or result. Its ID is nonempty and unique
// within the summary; Metadata replaces Detail with badges or metrics the
// app builds.
type ChatSummaryItem struct {
	ID, Label, Detail string
	Metadata          ChatView
}

// ChatResultSummary is a bounded list of artifacts outside the execution
// fold: three, or up to 100 expanded. MoreLabel and LessLabel say the limits
// of the app's collection, and Action opens all of it in a view of the
// app's. Metadata replaces Description. It reads no files.
type ChatResultSummary struct {
	Title, Description, ActionLabel, MoreLabel, LessLabel string
	Items                                                 []ChatSummaryItem
	Metadata                                              ChatView
	OnAction                                              func()
	OnSelect                                              func(id string)
	Conversation                                          *ChatConversationState
}

// Render implements core.Node.
func (r ChatResultSummary) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	expanded := core.NewSignal(cx, false)
	title := Text(r.Title, 13, Bold, colors.Text)
	titles := []core.Node{title}
	switch {
	case r.Metadata != nil:
		titles = append(titles, core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{core.Func(r.Metadata)}})
	case r.Description != "":
		titles = append(titles, Text(r.Description, 13, Regular, colors.Muted))
	}
	head := []core.Node{
		Icon(lucide.Files, 20, colors.Muted),
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 4}, Children: titles},
	}
	if r.ActionLabel != "" {
		head = append(head, Button{Text: r.ActionLabel, Variant: ButtonGhost, OnPress: r.OnAction})
	}
	row := func(i ChatSummaryItem) core.Node {
		label := Text(i.Label, 13, Regular, colors.Text)
		label.MaxLines = 1
		label.Style.FlexGrow, label.Style.FlexShrink, label.Style.MinWidth = 1, 1, core.Pt(0)
		var trailing core.Node
		if i.Metadata != nil {
			trailing = core.View{Style: core.Style{Direction: core.Row, Gap: 8, FlexShrink: 0}, Children: []core.Node{core.Func(i.Metadata)}}
		} else {
			detail := Text(i.Detail, 13, Regular, colors.Muted)
			detail.Style.FlexShrink = 0
			trailing = detail
		}
		return base.Button{Label: i.Label, OnPress: func(core.PressEvent) {
			if r.OnSelect != nil {
				r.OnSelect(i.ID)
			}
		},
			Style:     core.Style{Direction: core.Row, MinWidth: core.Pt(0), Padding: core.Symmetric(8, 12), AlignItems: core.AlignCenter, Gap: 8, Cursor: core.CursorPointer},
			StyleFunc: func() core.Style { return core.Style{Hover: &core.Style{Background: colors.chatOverlay()}} },
			Children:  []core.Node{label, trailing}}
	}
	children := []core.Node{
		core.View{Style: core.Style{Direction: core.Row, MinWidth: core.Pt(0), AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(12)}, Children: head},
		core.Dynamic(func(*core.Scope) core.Node {
			limit := 3
			if expanded.Get() {
				limit = 100
			}
			var rows core.Group
			for _, i := range r.Items[:min(len(r.Items), limit)] {
				rows = append(rows, row(i))
			}
			return rows
		}),
	}
	if len(r.Items) > 3 {
		children = append(children, base.Button{OnPress: func(core.PressEvent) {
			expanded.Update(func(v bool) bool { return !v })
			r.Conversation.pause()
		},
			Style: core.Style{Height: core.Pt(ButtonHeight), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Borders: core.Edges{Top: 1}, Cursor: core.CursorPointer},
			StyleFunc: func() core.Style {
				return core.Style{BorderColors: core.EdgeColors{Top: colors.Border()}, Hover: &core.Style{Background: colors.chatOverlay()}}
			},
			Children: []core.Node{Textf(func() string {
				if expanded.Get() {
					return r.LessLabel
				}
				return r.MoreLabel
			}, 13, Regular, colors.Text)}})
	}
	return core.View{Style: core.Style{MinWidth: core.Pt(0), FlexShrink: 0, CornerRadius: 12, BorderWidth: 1, Overflow: core.Hidden},
		StyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.Border(), Background: Alpha(colors.Text(), 0.035)}
		}, Children: children}
}

// chatShimmerPeriod is how long the highlight takes to cross the text and
// rest.
const chatShimmerPeriod = 1800 * time.Millisecond

// ChatShimmer is a single line of text of Size in Color with a Highlight
// sweeping across it, marking ongoing work. It keeps its natural width
// unless its parent shrinks it, when it ends in an ellipsis. The sweep is a
// mask the platform moves, so it costs no commits. Reduced motion shows
// plain text.
type ChatShimmer struct {
	Text             func() string
	Size             core.Points
	Color, Highlight func() core.Color
	Style            core.Style
}

// Render implements core.Node.
func (s ChatShimmer) Render(cx *core.Scope) core.Node {
	color := s.Color
	base := core.Text{ContentFunc: s.Text, MaxLines: 1, Style: core.Style{FontSize: s.Size, FlexShrink: 1, MinWidth: core.Pt(0)}, StyleFunc: Colored(color)}
	if reduceMotion(cx) {
		base.Style = base.Style.Merge(s.Style)
		return base
	}
	width := core.NewSignal(cx, core.Points(0))
	phase := core.NewAnimatedValue(cx, 0)
	cx.StartAnimation(core.Loop(core.Timing(phase, 1, core.TimingConfig{Duration: chatShimmerPeriod, Easing: holdLinear}), 0), nil)
	// A band of the highlight's text slides across, while the text in it
	// slides back just as far, so only the band moves.
	band := func() core.Points { return max(width.Get()*0.25, s.Size*1.25) }
	sweep := core.Dynamic(func(cx *core.Scope) core.Node {
		w, b := width.Get(), band()
		if w <= 0 {
			return nil
		}
		x := Lerp(phase, float64(-2*b), float64(w+b))
		mask := core.LinearGradient(90, core.RGBA(0xffffff, 0), core.RGBA(0xffffff, 1), core.RGBA(0xffffff, 0))
		return core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Bottom: core.Pt(0), Width: core.Pt(2 * b), Overflow: core.Hidden,
			Mask: &mask, Animated: &core.AnimatedStyle{TranslateX: x}}, Children: []core.Node{
			core.Text{ContentFunc: s.Text, MaxLines: 1, Style: core.Style{FontSize: s.Size, Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Width: core.Pt(w),
				Animated: &core.AnimatedStyle{TranslateX: core.Mul(x, core.Constant(-1))}},
				StyleFunc: Colored(func() core.Color { return Mix(color(), s.Highlight(), 0.75) })},
		}}
	})
	return core.View{OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Style: core.Style{FlexShrink: 1, MinWidth: core.Pt(0), Overflow: core.Hidden}.Merge(s.Style),
		Children: []core.Node{base, sweep}}
}
