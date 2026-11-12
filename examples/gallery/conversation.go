package main

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
	"github.com/nsgo-dev/nsgo/ui/chat"
)

// conversationDemo is ui/chat as an app puts it together: the transcript with
// a turn in each state it can be in, a panel on the glass, and the composer
// with its attachments, choices and toolbar. Its controls keep their own state
// and nothing behind them runs.
func conversationDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	state := chat.NewConversationState(cx)
	wide := shell.desktop()
	draft := core.NewSignal(cx, "")
	writing := core.NewSignal(cx, true)
	listening := core.NewSignal(cx, false)
	photo, file := core.NewSignal(cx, true), core.NewSignal(cx, true)
	model, modelOpen := core.NewSignal(cx, chatModel("Swift")), core.NewSignal(cx, false)
	effort, tier := core.NewSignal(cx, "medium"), core.NewSignal(cx, "standard")
	branch, worktree := core.NewSignal(cx, "main"), core.NewSignal(cx, true)

	// script is what the AI Chat's scenario id says and calls.
	script := func(id string) (s scenario, say string, tool chatTool, think string) {
		s, _ = findScenario(id)
		for _, st := range s.steps {
			switch {
			case st.say != "":
				say = st.say
			case st.tool != nil:
				tool = *st.tool
			case st.think != "":
				think = st.think
			}
		}
		return s, say, tool, think
	}
	prompt := func(s scenario) chat.View {
		return func(*core.Scope) core.Node {
			var nodes core.Group
			if hasImage(s.image) {
				nodes = append(nodes, chat.AttachmentPreview{Label: tr.ChatPhoto(), Image: s.image})
			}
			if s.file != nil {
				nodes = append(nodes, chat.Attachment{Label: s.file.name, Detail: s.file.size})
			}
			return append(nodes, core.Text{Content: s.prompt, Selectable: core.SelectableOn, Style: core.Style{FontSize: 13, LineHeight: 19}})
		}
	}
	markdown := func(md string, streaming bool) chat.View {
		return func(*core.Scope) core.Node {
			return chatMarkdown(func() string { return md }, func() bool { return streaming })
		}
	}
	thought := func(seconds int, text string) chat.Activity {
		return chat.Activity{ID: "think", Icon: lucide.Brain, Action: tr.ChatThoughtFor(seconds), Status: chat.Completed,
			Content: func(*core.Scope) core.Node { return core.Text{Content: text, Selectable: core.SelectableOn} }}
	}
	activity := func(t chatTool, st toolState, status chat.Status) chat.Activity {
		t.state = st
		a := chat.Activity{ID: t.id, Icon: lucide.Terminal, Action: tr.ChatRunCode(), Detail: t.lang + tr.ChatSandboxed(), Status: status, MaxHeight: 320,
			Content: func(cx *core.Scope) core.Node { return toolBody(cx, func() chatTool { return t }) }}
		switch t.kind {
		case "search":
			a.Icon, a.Action, a.Detail = lucide.Globe, tr.ChatWebSearch(), t.query
		case "weather":
			a.Icon, a.Action, a.Detail = lucide.Sun, tr.ChatWeather(), t.city
		}
		return a
	}
	work := func(title string, activities ...chat.Activity) []chat.Step {
		return []chat.Step{{ID: "work", Title: title, Activities: activities}}
	}
	turn := func(status chat.Status, s scenario, steps []chat.Step, final string, footer chat.View) core.Node {
		t := chat.Turn{Status: func() chat.Status { return status }, Conversation: state, Prompt: prompt(s), Footer: footer,
			Steps:    func() []chat.Step { return steps },
			Summary:  func() string { return tr.ChatWorked(0, 12) },
			Working:  func() string { return tr.ChatWorking(tr.ChatRunning(), 0, 12) },
			HasFinal: func() bool { return final != "" }, ShowActions: func() bool { return status == chat.Completed },
			Actions: func(*core.Scope) core.Node {
				return core.Group{
					chat.IconAction{Label: tr.ChatCopy(), Icon: lucide.Copy, OnPress: func() {}},
					chat.IconAction{Label: tr.ChatRegenerate(), Icon: lucide.RefreshCw, OnPress: func() {}},
					chat.IconAction{Label: tr.ChatGoodReply(), Icon: lucide.ThumbsUp, OnPress: func() {}},
					chat.IconAction{Label: tr.ChatBadReply(), Icon: lucide.ThumbsDown, OnPress: func() {}},
				}
			}}
		if final != "" {
			t.Final = markdown(final, false)
		}
		return t
	}
	note := func(text string) chat.View {
		return func(*core.Scope) core.Node { return ui.Text(text, 13, ui.Regular, colors.Faint) }
	}

	guide, guideSay, _, _ := script("guide")
	vision, visionSay, _, _ := script("vision")
	reason, reasonSay, _, reasonThink := script("reason")
	weather, weatherSay, weatherTool, _ := script("weather")
	failing, failingSay, _, _ := script("error")
	offline, _, _, _ := script("offline")
	search, searchSay, searchTool, _ := script("search")
	files := scenario{prompt: "Draft the launch plan and what goes with it."}

	results := []chat.SummaryItem{
		{ID: "plan", Label: "launch-plan.md", Detail: "12 KB"},
		{ID: "checklist", Label: "checklist.md", Detail: "3 KB"},
		{ID: "announcement", Label: "announcement.md", Detail: "2 KB"},
		{ID: "budget", Label: "budget.csv", Detail: "8 KB"},
		{ID: "timeline", Label: "timeline.png", Detail: "164 KB"},
	}
	transcript := core.View{Selectable: true, Style: core.Style{Gap: 48, MinWidth: core.Pt(0)}, Children: []core.Node{
		dateSeparator(cx, func() string { return "Today" }),
		turn(chat.Completed, guide, nil, guideSay, nil),
		turn(chat.Completed, vision, nil, visionSay, nil),
		turn(chat.Completed, reason, work(tr.ChatRanTools(1), thought(5, reasonThink)), reasonSay, nil),
		turn(chat.Completed, weather, work(tr.ChatRanTools(1), activity(weatherTool, toolDone, chat.Completed)), weatherSay, nil),
		turn(chat.Completed, files, nil, "The launch plan is ready, with the checklist and the announcement beside it.", func(*core.Scope) core.Node {
			return chat.ResultSummary{Title: tr.ChatFilesWritten(len(results)), Description: tr.ChatFilesDescription(), MoreLabel: tr.ChatShowMore(), LessLabel: tr.ChatShowLess(),
				Items: results, Conversation: state}
		}),
		turn(chat.Failed, failing, nil, failingSay, func(cx *core.Scope) core.Node {
			return errorRow(cx, func() string { return "The model is overloaded right now (error 529)." }, func() {})
		}),
		turn(chat.Cancelled, reason, nil, "The ball costs", note(tr.ChatYouStoppedThisReply())),
		turn(chat.Waiting, offline, nil, "", note(tr.ChatWaitingForAConnection())),
		turn(chat.Running, search, append(work(tr.ChatWebSearch(), activity(searchTool, toolRunning, chat.Running)),
			chat.Step{ID: "reply", Message: markdown(searchSay, true)}), "", nil),
	}}

	attachments := func(*core.Scope) core.Node {
		return core.Group{
			core.Show{When: photo.Get, Then: func(*core.Scope) core.Node {
				return chat.AttachmentPreview{Label: tr.ChatPhoto(), RemoveLabel: tr.ChatRemoveFile(), Image: core.Src(chatPhoto), OnRemove: func() { photo.Set(false) }}
			}},
			core.Show{When: file.Get, Then: func(*core.Scope) core.Node {
				return chat.Attachment{Label: "trip-notes.pdf", Detail: "2.4 MB", RemoveLabel: tr.ChatRemoveFile(), OnRemove: func() { file.Set(false) }}
			}},
		}
	}
	modelRow := func(name chatModel) core.Node {
		return core.View{OnPress: func(core.PressEvent) { model.Set(name); modelOpen.Set(false) }, Role: core.RoleButton, Label: string(name), Selected: func() bool { return model.Get() == name },
			StyleFunc: func() core.Style {
				return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween, Padding: core.Symmetric(6, 8), CornerRadius: ui.RadiusControl,
					Hover: &core.Style{Background: colors.Pressed()}, Selected: &core.Style{Background: colors.Pressed()}}
			},
			Children: []core.Node{ui.Text(string(name), 13, ui.Regular, colors.Text), ui.Text(chatTokens(chatWindows[name]), 12, ui.Regular, colors.Muted)}}
	}
	accessory := func(*core.Scope) core.Node {
		label := tr.ChatChooseModel()
		levels := []chat.Option{{ID: "off", Label: tr.ChatEffortOff()}, {ID: "low", Label: tr.ChatEffortLow()},
			{ID: "medium", Label: tr.ChatEffortMedium()}, {ID: "high", Label: tr.ChatEffortHigh()}, {ID: "max", Label: tr.ChatEffortMax()}}
		tiers := []chat.Option{{ID: "standard", Label: tr.ChatTierStandard()}, {ID: "fast", Label: tr.ChatTierFast()}}
		nodes := core.Group{
			chat.ChipPopover{Chip: chat.Chip{TextFunc: func() string { return string(model.Get()) }, Icon: lucide.Sparkles, Caret: true, Yield: true}, Open: modelOpen.Get, OnOpenChange: modelOpen.Set,
				Label: label, Width: 240, TestID: "conversation-model", Children: []core.Node{core.View{Style: core.Style{Padding: core.All(8), Gap: 1}, Children: []core.Node{
					modelRow("Swift"), modelRow("Deep"), modelRow("Vision"),
				}}}},
			chat.EffortPicker{Label: tr.ChatEffort(), Icon: lucide.Brain, Levels: levels, Selected: effort.Get, OnChange: effort.Set, Off: true},
		}
		// A phone's row has no room for a third choice.
		if wide {
			nodes = append(nodes, chat.Picker{Icon: lucide.Zap, Options: tiers, Selected: tier.Get, OnChange: tier.Set, Width: 200, Label: func() string {
				if tier.Get() == "fast" {
					return tr.ChatTierFast()
				}
				return tr.ChatTierStandard()
			}})
		}
		return nodes
	}
	trailing := func(*core.Scope) core.Node {
		value := func() string { return "42.6k / 128k (33%)" }
		nodes := core.Group{chat.IconAction{Label: tr.ChatChooseAPhoto(), Icon: lucide.Paperclip, OnPress: func() { photo.Set(true); file.Set(true) }}}
		if wide {
			nodes = append(nodes, chat.IconAction{Label: tr.ChatVoiceInput(), Icon: lucide.Mic, Tint: colors.Danger, Active: listening.Get, OnPress: func() { listening.Set(!listening.GetUntracked()) }})
		}
		return append(nodes, chat.ContextMeter{Label: tr.ChatContextWindow(), Title: tr.ChatContextWindow(), Tooltip: value, Value: value, Fraction: func() float64 { return 0.33 }})
	}
	quota := chat.Quota{ID: "demo", Title: tr.ChatQuotaAccount(), RefreshLabel: tr.ChatQuotaRefresh(), OpenLabel: tr.ChatQuotaOpen(),
		Countdown: func() string { return "04:32" },
		Leading:   func(_ *core.Scope, size core.Points) core.Node { return ui.Icon(lucide.Gauge, size, colors.Muted) },
		Rows: func() []chat.QuotaRow {
			return []chat.QuotaRow{
				{Label: tr.ChatQuotaFiveHour(), Detail: tr.ChatQuotaFiveHourReset(), Value: tr.ChatQuotaLeft(63), Fraction: 0.63},
				{Label: tr.ChatQuotaWeekly(), Detail: tr.ChatQuotaWeeklyReset(), Value: tr.ChatQuotaLeft(18), Fraction: 0.18},
				{Label: tr.ChatQuotaCredits(), Value: "$12.40", Fraction: -1},
			}
		},
		OnRefresh: func() {}, OnOpen: func() {}}
	footer := func(*core.Scope) core.Node {
		branches := []chat.Option{{ID: "main", Label: "main"}, {ID: "feature/chat-ui", Label: "feature/chat-ui"}, {ID: "release/0.2026.10", Label: "release/0.2026.10"}}
		var nodes core.Group
		if wide {
			nodes = append(nodes, chat.Chip{Text: "MacBook Pro", Icon: lucide.Laptop})
		}
		nodes = append(nodes, chat.Chip{Text: "nsgo", Icon: lucide.Folder},
			chat.Picker{Label: branch.Get, Icon: lucide.GitBranch, Options: branches, Selected: branch.Get, OnChange: branch.Set,
				SearchLabel: tr.ChatSearchBranches(), EmptyText: tr.ChatNoResults(), Searchable: 2})
		if wide {
			nodes = append(nodes, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Margin: core.Edges{Left: 7}}, Children: []core.Node{
				ui.Switch{Checked: worktree.Get, OnChange: worktree.Set, Label: tr.ChatWorktree(), Size: ui.SizeSmall},
				ui.Text(tr.ChatWorktree(), 12.5, ui.Regular, colors.Muted),
			}})
		}
		nodes = append(nodes, core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0)}})
		if wide {
			nodes = append(nodes, core.Show{When: writing.Get, Then: func(*core.Scope) core.Node {
				return chat.Shimmer{Text: tr.ChatStreaming, Size: 12.5, Color: colors.Muted, Highlight: colors.Text}
			}})
		}
		first := func() chat.QuotaRow { return quota.Rows()[0] }
		return append(nodes, chat.QuotaSummary{Label: tr.ChatUsageLimits(), Tooltip: func() string { return quota.Title + "\n" + first().Label + " · " + first().Value },
			Row: first, Quotas: []chat.Quota{quota}})
	}
	composer := core.View{Style: core.Style{Gap: 6}, Children: []core.Node{
		chat.GlassPanel{TestID: "conversation-panel", Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			ui.Icon(lucide.CircleAlert, 14, colors.Danger), ui.Text(tr.ChatYourMessageSendsWhenThe(), 12.5, ui.Regular, colors.Danger),
		}}}},
		chat.Composer{Draft: draft.Get, OnDraftChange: draft.Set, Label: tr.ChatMessage(), Placeholder: tr.ChatPlaceholder(), SendLabel: tr.ChatSend(), StopLabel: tr.ChatStop(),
			Running: writing.Get, OnSend: func() { draft.Set(""); writing.Set(true) }, OnStop: func() { writing.Set(false) }, Glass: true, State: state,
			Accessory: accessory, Trailing: trailing, Footer: footer, Attachments: attachments, HasAttachments: func() bool { return photo.Get() || file.Get() }},
	}}

	return core.View{StyleFunc: func() core.Style { return fillPage.Merge(core.Style{Background: colors.Background()}) }, Children: []core.Node{
		core.View{Style: fillPage.Merge(core.Style{PaddingEnv: core.EnvEdges{Bottom: core.EnvKeyboard}}), Children: []core.Node{
			chat.FloatingConversation{State: state, JumpLabel: tr.ChatJumpToTheLatestMessage(), TestID: "conversation", Children: []core.Node{transcript}, Composer: []core.Node{composer}},
		}},
	}}
}
