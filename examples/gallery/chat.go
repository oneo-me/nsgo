package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
	"github.com/oneo-me/nsgo/ui/chat"
)

var featured = []struct {
	id   string
	icon func() *core.SVG
	note string
}{{"guide", lucide.TextAlignStart, "Markdown, a table, code"}, {"reason", lucide.Sparkles, "Reasoning first"}, {"search", lucide.Search, "A tool call"}, {"vision", lucide.Image, "A photo and a file"}}

// chatRun is the reply being written.
type chatRun struct {
	id           string
	model        chatModel
	script       string
	cancelled    bool
	md, reason   string
	thinkStart   time.Time
	thinkSeconds int
}

// chatWindows are the fictional models' context windows.
var chatWindows = map[chatModel]int{"Swift": 128_000, "Deep": 1_000_000, "Vision": 200_000}

// chatTokens writes a count of tokens short.
func chatTokens(n int) string {
	switch {
	case n >= 999_500:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%.0fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// chatDemo is a chat assistant on ui's conversation components, backed by
// the local Claude or Codex CLI on desktops and by scripted streaming
// scenarios on the web and phones. The desktop layout adds a list of
// conversations on the left.
func chatDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	wide := shell.desktop()
	ids := 0
	nextID := func() string { ids++; return fmt.Sprintf("m%d", ids) }

	messages := core.NewSignalFunc(cx, []msg(nil), func(a, b []msg) bool { return false })
	model := core.NewSignal(cx, chatModel("Swift"))
	native := desktop.IsAvailable(cx)
	provider := core.NewSignal(cx, "Codex")
	effort := core.NewSignal(cx, "medium")
	branch := core.NewSignal(cx, "main")
	worktree := core.NewSignal(cx, true)
	var cancelLocal func()
	draft := core.NewSignal(cx, "")
	attachment := core.NewSignalFunc(cx, (*chatAttachment)(nil), func(a, b *chatAttachment) bool { return a == b })
	bannerOn := core.NewSignal(cx, false)
	bannerKind := core.NewSignal(cx, "offline")
	// copied is the copied message, for a moment.
	copied := core.NewSignal(cx, "")
	conversation := chat.NewConversationState(cx)

	// The reply being written; finished replies are plain data in
	// messages.
	liveID := core.NewSignal(cx, "")
	liveReason, liveMd := core.NewSignal(cx, ""), core.NewSignal(cx, "")
	thinking := core.NewSignal(cx, false)
	thinkSeconds := core.NewSignal(cx, 0)
	liveTools := core.NewSignalFunc(cx, []chatTool(nil), func(a, b []chatTool) bool { return false })
	now := core.NewSignal(cx, cx.Now())

	var run *chatRun

	after := func(r *chatRun, d time.Duration, fn func()) {
		later(cx, d, func() {
			if !r.cancelled {
				fn()
			}
		})
	}
	// stream writes text at about rate tokens a second, with jitter.
	stream := func(r *chatRun, text string, rate float64, intoReason bool, then func()) {
		tokens := streamTokens(text)
		var next func(i int)
		next = func(i int) {
			if i >= len(tokens) {
				then()
				return
			}
			after(r, time.Duration(1000/rate*(0.55+rand.Float64()*0.9))*time.Millisecond, func() {
				if intoReason {
					r.reason += tokens[i]
					liveReason.Set(r.reason)
				} else {
					r.md += tokens[i]
					liveMd.Set(r.md)
				}
				next(i + 1)
			})
		}
		next(0)
	}
	setTool := func(id string, st toolState) {
		liveTools.Update(func(ts []chatTool) []chatTool {
			ts = slices.Clone(ts)
			for i := range ts {
				if ts[i].id == id {
					ts[i].state = st
				}
			}
			return ts
		})
	}
	find := func(id string) msg {
		for _, m := range messages.GetUntracked() {
			if m.id == id {
				return m
			}
		}
		return msg{}
	}
	replace := func(id string, m msg) {
		messages.Update(func(ms []msg) []msg {
			ms = slices.Clone(ms)
			for i := range ms {
				if ms[i].id == id {
					ms[i] = m
				}
			}
			return ms
		})
	}
	// freeze turns the reply into its message's and ends the run.
	freeze := func(r *chatRun, status, errText string) {
		if run != r {
			return
		}
		seconds := r.thinkSeconds
		if seconds == 0 {
			seconds = max(1, int(cx.Now().Sub(r.thinkStart).Seconds()+0.5))
		}
		final := find(r.id)
		final.md, final.status, final.err, final.ended = r.md, status, errText, cx.Now()
		if r.reason != "" {
			final.reasoning = &reasoning{text: r.reason, seconds: seconds}
		}
		for _, t := range liveTools.GetUntracked() {
			if t.state != toolDone {
				t.state = toolCancelled
			}
			final.tools = append(final.tools, t)
		}
		run = nil
		cx.Batch(func() {
			replace(r.id, final)
			liveID.Set("")
		})
	}

	// reply runs a scenario's reply into the turn at; again starts that
	// turn afresh, under a new ID, for a retry or a regeneration.
	var reply func(s scenario, at string, again bool)
	reply = func(s scenario, at string, again bool) {
		script := s.id
		if native {
			script = "local:" + provider.GetUntracked()
		}
		id := at
		if again {
			id = nextID()
		}
		start := cx.Now()
		r := &chatRun{id: id, model: s.model, script: script, thinkStart: start}
		run = r
		old := find(at)
		cx.Batch(func() {
			liveReason.Set("")
			thinking.Set(false)
			thinkSeconds.Set(0)
			liveTools.Set(nil)
			liveMd.Set("")
			liveID.Set(r.id)
			now.Set(start)
			replace(at, msg{kind: "turn", id: id, text: old.text, image: old.image, file: old.file, model: s.model, script: script, started: start})
		})
		if native {
			var parts []string
			for _, m := range messages.GetUntracked() {
				if m.kind != "turn" {
					continue
				}
				parts = append(parts, "User: "+m.text)
				if m.id == r.id {
					break
				}
				parts = append(parts, "Assistant: "+m.md)
			}
			prompt := "You are a helpful chat assistant. Answer the final user message using the conversation below. Reply directly in Markdown. Do not use tools or modify files.\n\n" + strings.Join(parts, "\n\n")
			cancelLocal = localReply(cx, provider.GetUntracked(), prompt, func(chunk string) {
				if run != r || r.cancelled {
					return
				}
				r.md += chunk
				liveMd.Set(r.md)
			}, func(errText string) {
				if run != r || r.cancelled {
					return
				}
				cancelLocal = nil
				if errText != "" {
					freeze(r, "error", errText)
				} else {
					freeze(r, "done", "")
				}
			})
			return
		}
		var step func(i int)
		step = func(i int) {
			if i >= len(s.steps) {
				freeze(r, "done", "")
				return
			}
			st := s.steps[i]
			switch {
			case st.think != "":
				r.thinkStart = cx.Now()
				now.Set(r.thinkStart)
				thinking.Set(true)
				stream(r, st.think, 75, true, func() {
					r.thinkSeconds = max(1, int(cx.Now().Sub(r.thinkStart).Seconds()+0.5))
					thinkSeconds.Set(r.thinkSeconds)
					thinking.Set(false)
					after(r, 350*time.Millisecond, func() { step(i + 1) })
				})
			case st.tool != nil:
				t := *st.tool
				t.state = toolPending
				liveTools.Update(func(ts []chatTool) []chatTool { return append(slices.Clone(ts), t) })
				after(r, 450*time.Millisecond, func() {
					setTool(t.id, toolRunning)
					after(r, time.Duration(st.ms)*time.Millisecond, func() {
						setTool(t.id, toolDone)
						after(r, 400*time.Millisecond, func() { step(i + 1) })
					})
				})
			case st.say != "":
				stream(r, st.say, 40, false, func() { step(i + 1) })
			case st.fail != "":
				after(r, 900*time.Millisecond, func() { freeze(r, "error", st.fail) })
			default:
				step(i + 1)
			}
		}
		after(r, 450*time.Millisecond, func() { step(0) })
	}

	stop := func() {
		if run == nil {
			return
		}
		run.cancelled = true
		if cancelLocal != nil {
			cancelLocal()
			cancelLocal = nil
		}
		freeze(run, "stopped", "")
	}
	cx.Cleanup(func() {
		if run != nil {
			run.cancelled = true
		}
		if cancelLocal != nil {
			cancelLocal()
		}
	})

	// submit sends the user's message for a scenario, then its reply. A
	// new message interrupts one being written, as it does an agent.
	submit := func(s scenario, text string, image core.ImageSource, file *fileRef) {
		stop()
		offline := 0
		if !native {
			for _, st := range s.steps {
				if st.offline > 0 {
					offline = st.offline
				}
			}
		}
		user := msg{kind: "turn", id: nextID(), text: text, image: image, file: file, model: s.model, started: cx.Now(), waiting: offline > 0}
		cx.Batch(func() {
			messages.Update(func(ms []msg) []msg { return append(slices.Clone(ms), user) })
			draft.Set("")
			attachment.Set(nil)
			model.Set(s.model)
		})
		conversation.JumpToLatest()
		var steps []chatStep
		for _, st := range s.steps {
			if st.offline == 0 {
				steps = append(steps, st)
			}
		}
		s.steps = steps
		if offline == 0 {
			reply(s, user.id, false)
			return
		}
		bannerKind.Set("offline")
		bannerOn.Set(true)
		later(cx, time.Duration(offline)*time.Millisecond, func() {
			bannerKind.Set("online")
			later(cx, 900*time.Millisecond, func() {
				bannerOn.Set(false)
				reply(s, user.id, false)
			})
		})
	}
	send := func() {
		text := strings.TrimSpace(draft.GetUntracked())
		a := attachment.GetUntracked()
		if text == "" && a == nil {
			return
		}
		if a != nil {
			v, _ := findScenario("vision")
			if m := model.GetUntracked(); m != "Swift" {
				v.model = m
			}
			if text == "" {
				text = v.prompt
			}
			submit(v, text, a.image, &a.file)
			return
		}
		submit(freeform(text, model.GetUntracked()), text, core.ImageSource{}, nil)
	}
	play := func(id string) {
		if id == "history" {
			messages.Update(func(ms []msg) []msg { return append(chatHistory(nextID), ms...) })
			conversation.JumpToLatest()
			return
		}
		s, _ := findScenario(id)
		submit(s, s.prompt, s.image, s.file)
	}
	update := func(id string, change func(m msg) msg) {
		replace(id, change(find(id)))
	}
	regenerate := func(m msg) {
		if run != nil {
			return
		}
		conversation.JumpToLatest()
		s, ok := findScenario(m.script)
		switch {
		case m.script == "retry":
			s = retried
		case !ok:
			s = freeform(m.text, m.model)
		}
		s.model = m.model
		var steps []chatStep
		for _, st := range s.steps {
			if st.offline == 0 && st.fail == "" {
				steps = append(steps, st)
			}
		}
		s.steps = steps
		reply(s, m.id, true)
	}
	retry := func(m msg) {
		if run != nil {
			return
		}
		conversation.JumpToLatest()
		if native {
			regenerate(m)
		} else {
			reply(retried, m.id, true)
		}
	}
	copyText := func(key, text string) {
		cx.SetClipboard(text)
		copied.Set(key)
		later(cx, 1600*time.Millisecond, func() {
			if copied.GetUntracked() == key {
				copied.Set("")
			}
		})
	}
	// choosePhoto attaches a photo from the system's picker.
	choosePhoto := func() {
		cx.PickImages(core.PickOptions{MaxDimension: 2048, Quality: 0.85}, func(files []core.PickedFile) {
			if len(files) == 0 {
				return
			}
			f := files[0]
			size := fmt.Sprintf("%d KB", max(1, (len(f.Data)+512)/1024))
			if len(f.Data) >= 1024*1024 {
				size = fmt.Sprintf("%.1f MB", float64(len(f.Data))/1024/1024)
			}
			attachment.Set(&chatAttachment{image: core.ImageSource{Data: f.Data, Size: f.Size}, file: fileRef{f.Name, size}})
		})
	}
	// dropped attaches a file dropped on the composer, named but not read.
	dropped := func(uris []string) {
		if len(uris) == 0 {
			return
		}
		name := uris[0]
		if u, err := url.Parse(name); err == nil {
			name = path.Base(u.Path)
		}
		attachment.Set(&chatAttachment{file: fileRef{name, fileKind(name)}})
	}

	bannerValue := core.NewAnimatedValue(cx, 0)
	cx.Effect(func() {
		to := 0.0
		if bannerOn.Get() {
			to = 1
		}
		ui.Spring(cx, bannerValue, to, ui.SettleSpring)
	})
	streaming := func() bool { return liveID.Get() != "" }

	turnRow := func(cx *core.Scope, m func() msg) core.Node {
		live := func() bool { return liveID.Get() == m().id }
		reasonText := func() string {
			if live() {
				return liveReason.Get()
			}
			if r := m().reasoning; r != nil {
				return r.text
			}
			return ""
		}
		reasoned := core.NewMemo(cx, func() bool { return reasonText() != "" })
		thinkingLabel := func() string {
			if live() {
				if thinking.Get() {
					return tr.ChatThinking(max(0, int(now.Get().Sub(run.thinkStart).Seconds())))
				}
				return tr.ChatThoughtFor(max(1, thinkSeconds.Get()))
			}
			if r := m().reasoning; r != nil {
				return tr.ChatThoughtFor(r.seconds)
			}
			return ""
		}
		tools := func() []chatTool {
			if live() {
				return liveTools.Get()
			}
			return m().tools
		}
		tool := func(id string) func() chatTool {
			return func() chatTool {
				for _, t := range tools() {
					if t.id == id {
						return t
					}
				}
				return chatTool{}
			}
		}
		md := func() string {
			if live() {
				return liveMd.Get()
			}
			return m().md
		}
		written := core.NewMemo(cx, func() bool { return md() != "" })
		status := core.NewMemo(cx, func() chat.Status {
			switch {
			case live():
				return chat.Running
			case m().waiting:
				return chat.Waiting
			case m().status == "error":
				return chat.Failed
			case m().status == "stopped":
				return chat.Cancelled
			case m().status == "done":
				return chat.Completed
			}
			return chat.Running
		})
		elapsed := func() (minutes, seconds int) {
			end := m().ended
			if live() || end.IsZero() {
				end = now.Get()
			}
			s := max(0, int(end.Sub(m().started).Seconds()))
			return s / 60, s % 60
		}
		statusLabel := func() string {
			switch status.Get() {
			case chat.Waiting:
				return tr.ChatWaitingForAConnection()
			case chat.Failed:
				return tr.ChatFailed()
			case chat.Cancelled:
				return tr.ChatStopped()
			}
			return tr.ChatRunning()
		}
		steps := func() []chat.Step {
			var activities []chat.Activity
			if reasoned.Get() {
				a := chat.Activity{ID: "think", Icon: lucide.Brain, Action: thinkingLabel(), Status: chat.Completed,
					Content: func(*core.Scope) core.Node { return core.Text{ContentFunc: reasonText, Selectable: core.SelectableOn} }}
				if live() && thinking.Get() {
					a.Status = chat.Running
				}
				activities = append(activities, a)
			}
			for _, t := range tools() {
				a := chat.Activity{ID: t.id, Icon: lucide.Terminal, Action: tr.ChatRunCode(), Detail: t.lang + tr.ChatSandboxed(), MaxHeight: 320}
				switch t.kind {
				case "search":
					a.Icon, a.Action, a.Detail = lucide.Globe, tr.ChatWebSearch(), t.query
				case "weather":
					a.Icon, a.Action, a.Detail = lucide.Sun, tr.ChatWeather(), t.city
				}
				switch t.state {
				case toolDone:
					a.Status = chat.Completed
				case toolCancelled:
					a.Status = chat.Cancelled
				default:
					a.Status = chat.Running
				}
				id := t.id
				a.Content = func(cx *core.Scope) core.Node { return toolBody(cx, tool(id)) }
				activities = append(activities, a)
			}
			var out []chat.Step
			if n := len(activities); n > 0 {
				title := tr.ChatRanTools(n)
				if last := activities[n-1]; live() && !written.Get() {
					// A live group names its latest activity.
					title = strings.TrimSpace(last.Action + " " + last.Detail)
				}
				out = append(out, chat.Step{ID: "work", Title: title, Activities: activities})
			}
			if live() && written.Get() {
				// The reply streams after the work and becomes the final
				// response once complete.
				out = append(out, chat.Step{ID: "reply", Message: func(*core.Scope) core.Node { return chatMarkdown(md, func() bool { return true }) }})
			}
			return out
		}
		return chat.Turn{Status: status.Get, Steps: steps, Conversation: conversation,
			Summary: func() string { minutes, seconds := elapsed(); return tr.ChatWorked(minutes, seconds) },
			Working: func() string { minutes, seconds := elapsed(); return tr.ChatWorking(statusLabel(), minutes, seconds) },
			Prompt: func(cx *core.Scope) core.Node {
				first := core.Untracked(cx, m)
				var nodes core.Group
				if hasImage(first.image) {
					nodes = append(nodes, chat.AttachmentPreview{Label: tr.ChatPhoto(), Image: first.image})
				}
				if first.file != nil {
					nodes = append(nodes, chat.Attachment{Label: first.file.name, Detail: first.file.size})
				}
				return append(nodes, core.Text{Content: first.text, Selectable: core.SelectableOn, Style: core.Style{FontSize: 13, LineHeight: 19}})
			},
			Final: func(*core.Scope) core.Node { return chatMarkdown(md, func() bool { return false }) }, HasFinal: func() bool { return !live() && written.Get() },
			Footer: func(cx *core.Scope) core.Node {
				return core.Group{
					core.Show{When: func() bool { return m().waiting }, Then: func(cx *core.Scope) core.Node {
						return ui.HStack{Gap: 6, Children: []core.Node{ui.Spinner{Size: 12}, ui.Text(tr.ChatWaitingForAConnection(), 12, ui.Regular, colors.Muted)}}
					}},
					core.Show{When: func() bool { return status.Get() == chat.Failed }, Then: func(cx *core.Scope) core.Node {
						return errorRow(cx, func() string {
							if e := m().err; e != "" {
								return e
							}
							return tr.ChatSomethingWentWrong()
						}, func() { retry(core.Untracked(cx, m)) })
					}},
					core.Show{When: func() bool { return status.Get() == chat.Cancelled }, Then: func(cx *core.Scope) core.Node {
						return ui.Text(tr.ChatYouStoppedThisReply(), 13, ui.Regular, colors.Faint)
					}},
				}
			},
			ShowActions: func() bool { return status.Get() == chat.Completed || status.Get() == chat.Cancelled },
			Actions: func(cx *core.Scope) core.Node {
				rate := func(to string) func() {
					return func() {
						update(core.Untracked(cx, m).id, func(x msg) msg {
							if x.rating == to {
								x.rating = ""
							} else {
								x.rating = to
							}
							return x
						})
					}
				}
				return core.Group{
					chat.IconAction{Label: tr.ChatCopy(), IconFunc: func() *core.SVG {
						if copied.Get() == m().id {
							return lucide.Check()
						}
						return lucide.Copy()
					}, Active: func() bool { return copied.Get() == m().id }, Tint: colors.Up, OnPress: func() { mm := core.Untracked(cx, m); copyText(mm.id, mm.md) }},
					chat.IconAction{Label: tr.ChatRegenerate(), Icon: lucide.RefreshCw, OnPress: func() { regenerate(core.Untracked(cx, m)) }},
					chat.IconAction{Label: tr.ChatGoodReply(), Icon: lucide.ThumbsUp, Active: func() bool { return m().rating == "up" }, Tint: colors.Accent, OnPress: rate("up")},
					chat.IconAction{Label: tr.ChatBadReply(), Icon: lucide.ThumbsDown, Active: func() bool { return m().rating == "down" }, Tint: colors.Danger, OnPress: rate("down")},
				}
			}}
	}

	emptyText := tr.ChatASimulatedAssistantEveryReply()
	if native {
		emptyText = tr.ChatChatWithYourLocalClaude()
	}
	var featuredTiles []core.Node
	for i, f := range featured {
		s, _ := findScenario(f.id)
		featuredTiles = append(featuredTiles, core.View{Style: core.Style{Width: core.Pct(47), FlexGrow: 1}, Children: []core.Node{
			Appear{Index: i + 1, Children: []core.Node{ui.Press{OnPress: func() { play(s.id) }, Label: s.chip, Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					return fin.Card().Over(core.Style{MinHeight: core.Pt(104), Padding: core.All(12), Gap: 8, CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous,
						Background: colors.Surface(), BorderWidth: 1, BorderColor: colors.Border()})
				}, Children: []core.Node{
					ui.Icon(f.icon, 18, colors.Accent), ui.Text(s.chip, 13, ui.Semibold, colors.Text), ui.Text(f.note, 12, ui.Regular, colors.Muted),
				}},
			}}}},
		}})
	}
	type chip struct{ id, label string }
	var chips []chip
	for _, s := range scenarios {
		chips = append(chips, chip{s.id, s.chip})
	}
	chips = append(chips, chip{"history", tr.ChatLoadALongConversation()})

	// The empty conversation shows over the transcript, above the composer.
	empty := func(*core.Scope) core.Node {
		var nodes []core.Node
		for _, c := range chips {
			nodes = append(nodes, ui.Press{OnPress: func() { play(c.id) }, HitSlop: 4, Label: c.label, Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					return fin.Pill().Over(core.Style{Height: core.Pt(36), Padding: core.Symmetric(0, 14), CornerRadius: 18, JustifyContent: core.JustifyCenter, BorderWidth: 1})
				}, Children: []core.Node{ui.Text(c.label, 13, ui.Medium, colors.Text)}},
			}})
		}
		return core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), AlignItems: core.AlignCenter},
			StyleFunc: func() core.Style { return core.Style{Bottom: core.Pt(conversation.ComposerHeight())} },
			Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1, Width: core.Pct(100), MaxWidth: core.Pt(chat.ContentWidth), MinHeight: core.Pt(0)}, Children: []core.Node{
				core.View{Style: core.Style{FlexGrow: 1, JustifyContent: core.JustifyCenter, Padding: core.Symmetric(24, 20), Gap: 24, MinHeight: core.Pt(0)}, Children: []core.Node{
					Appear{Children: []core.Node{core.View{Style: core.Style{AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
						IconTile{Icon: lucide.Sparkles, Size: 56},
						ui.Text(tr.ChatHowCanIHelp(), 22, ui.Semibold, colors.Text),
						core.Text{Content: emptyText, Style: core.Style{FontSize: 13, LineHeight: 21, TextAlign: core.TextAlignCenter}, StyleFunc: ui.Colored(colors.Muted)},
					}}}},
					core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 12}, Children: featuredTiles},
				}},
				core.Scroll{Horizontal: true, HideIndicator: true, Behavior: core.ScrollBehavior{KeyboardShouldPersistTaps: core.KeyboardTapsHandled}, Style: core.Style{MinHeight: core.Pt(52)},
					Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, Gap: 8, Padding: core.Symmetric(8, 20)}, Children: nodes}}},
			}}}}
	}

	accessory := func(cx *core.Scope) core.Node {
		var models chat.Picker
		if native {
			models = chat.Picker{Label: provider.Get, Icon: lucide.Sparkles, Selected: provider.Get, OnChange: func(id string) {
				if run == nil {
					provider.Set(id)
				}
			}, Options: []chat.Option{
				{ID: "Claude", Label: "Claude", Group: tr.ChatProviderLocal(), Icon: lucide.Sparkles},
				{ID: "Codex", Label: "Codex", Group: tr.ChatProviderLocal(), Icon: lucide.Sparkles},
			}}
		} else {
			var options []chat.Option
			for _, name := range []chatModel{"Swift", "Deep", "Vision"} {
				options = append(options, chat.Option{ID: string(name), Label: string(name), Detail: chatTokens(chatWindows[name]), Group: tr.ChatProviderRemote(), Icon: lucide.Sparkles})
			}
			models = chat.Picker{Label: func() string { return string(model.Get()) }, Icon: lucide.Sparkles, Options: options,
				Selected: func() string { return string(model.Get()) }, OnChange: func(id string) { model.Set(chatModel(id)) },
				SearchLabel: tr.ChatSearchModels(), EmptyText: tr.ChatNoResults(), Searchable: 2, Width: 280, TestID: "chat-model"}
		}
		levels := []chat.Option{{ID: "off", Label: tr.ChatEffortOff()}, {ID: "low", Label: tr.ChatEffortLow()},
			{ID: "medium", Label: tr.ChatEffortMedium()}, {ID: "high", Label: tr.ChatEffortHigh()}, {ID: "max", Label: tr.ChatEffortMax()}}
		tooltip := func() string {
			for _, o := range levels {
				if o.ID == effort.Get() {
					return tr.ChatEffort() + " · " + o.Label
				}
			}
			return tr.ChatEffort()
		}
		return core.Group{models, chat.EffortPicker{Label: tr.ChatEffort(), Tooltip: tooltip, Icon: lucide.Brain, Levels: levels, Selected: effort.Get, OnChange: effort.Set, Off: true, TestID: "chat-effort"}}
	}
	trailing := func(cx *core.Scope) core.Node {
		used := func() int {
			n := 18_400 + 6_200*len(messages.Get())
			if streaming() {
				n += len(liveMd.Get()) / 4
			}
			return n
		}
		fraction := func() float64 { return min(1, float64(used())/float64(chatWindows[model.Get()])) }
		value := func() string {
			return fmt.Sprintf("%s / %s (%d%%)", chatTokens(used()), chatTokens(chatWindows[model.Get()]), int(math.Round(fraction()*100)))
		}
		return core.Group{
			chat.IconAction{Label: tr.ChatChooseAPhoto(), Icon: lucide.Paperclip, OnPress: choosePhoto, TestID: "chat-attach"},
			chat.ContextMeter{Label: tr.ChatContextWindow(), Title: tr.ChatContextWindow(), Tooltip: value, Value: value, Fraction: fraction, TestID: "chat-context"},
		}
	}
	// A sample account: its countdown runs and its refresh only waits.
	const quotaEvery = 5 * time.Minute
	quotaNext := core.NewSignal(cx, cx.Now().Add(quotaEvery))
	refreshing := core.NewSignal(cx, false)
	clock := every(cx, time.Second, func() {
		if quotaNext.GetUntracked().Before(cx.Now()) {
			quotaNext.Set(cx.Now().Add(quotaEvery))
		}
		now.Set(cx.Now())
	})
	cx.Cleanup(clock)
	quota := chat.Quota{ID: "demo", Title: tr.ChatQuotaAccount(), RefreshLabel: tr.ChatQuotaRefresh(), OpenLabel: tr.ChatQuotaOpen(), Refreshing: refreshing.Get,
		Countdown: func() string {
			if refreshing.Get() {
				return tr.ChatQuotaRefreshing()
			}
			seconds := max(0, int((quotaNext.Get().Sub(now.Get())+time.Second-1)/time.Second))
			return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
		},
		Leading: func(_ *core.Scope, size core.Points) core.Node { return ui.Icon(lucide.Gauge, size, colors.Muted) },
		Rows: func() []chat.QuotaRow {
			return []chat.QuotaRow{
				{Label: tr.ChatQuotaFiveHour(), Detail: tr.ChatQuotaFiveHourReset(), Value: tr.ChatQuotaLeft(63), Fraction: 0.63},
				{Label: tr.ChatQuotaWeekly(), Detail: tr.ChatQuotaWeeklyReset(), Value: tr.ChatQuotaLeft(18), Fraction: 0.18},
				{Label: tr.ChatQuotaCredits(), Value: "$12.40", Fraction: -1},
			}
		},
		OnRefresh: func() {
			refreshing.Set(true)
			later(cx, 1200*time.Millisecond, func() {
				quotaNext.Set(cx.Now().Add(quotaEvery))
				refreshing.Set(false)
			})
		},
		OnOpen: func() { cx.OpenURL("https://github.com/oneo-me/nsgo", nil) }}
	footer := func(cx *core.Scope) core.Node {
		var replay []chat.Option
		for _, c := range chips {
			replay = append(replay, chat.Option{ID: c.id, Label: c.label, Icon: lucide.RefreshCw})
		}
		branches := []chat.Option{{ID: "main", Label: "main"}, {ID: "feature/chat-ui", Label: "feature/chat-ui"}, {ID: "release/0.2026.10", Label: "release/0.2026.10"}}
		nodes := core.Group{
			chat.Chip{Text: "nsgo", Icon: lucide.Folder},
			chat.Picker{Label: branch.Get, Icon: lucide.GitBranch, Options: branches, Selected: branch.Get, OnChange: branch.Set,
				SearchLabel: tr.ChatSearchBranches(), EmptyText: tr.ChatNoResults(), Searchable: 2, TestID: "chat-branch"},
			chat.Picker{Label: tr.ChatReplay, Icon: lucide.RefreshCw, Options: replay, OnChange: play, TestID: "chat-replay"},
		}
		if wide {
			nodes = append(nodes, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Margin: core.Edges{Left: 7}}, Children: []core.Node{
				ui.Switch{Checked: worktree.Get, OnChange: worktree.Set, Label: tr.ChatWorktree(), Small: true},
				ui.Text(tr.ChatWorktree(), 12.5, ui.Regular, func() core.Color { return ui.Mix(colors.Text(), colors.Muted(), 0.45) }),
			}})
		}
		nodes = append(nodes, core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0)}})
		if wide {
			nodes = append(nodes, core.Show{When: streaming, Then: func(*core.Scope) core.Node {
				return chat.Shimmer{Text: tr.ChatStreaming, Size: 12.5, Color: colors.Muted, Highlight: colors.Text}
			}})
		}
		first := func() chat.QuotaRow { return quota.Rows()[0] }
		return append(nodes, chat.QuotaSummary{Label: tr.ChatUsageLimits(), Tooltip: func() string { return quota.Title + "\n" + first().Label + " · " + first().Value },
			Row: first, Quotas: []chat.Quota{quota}, TestID: "chat-quota"})
	}
	attachments := func(cx *core.Scope) core.Node {
		return core.Dynamic(func(cx *core.Scope) core.Node {
			a := attachment.Get()
			if a == nil {
				return nil
			}
			return chat.Attachment{Label: a.file.name, Detail: a.file.size, Image: a.image, RemoveLabel: tr.ChatRemoveFile(), OnRemove: func() { attachment.Set(nil) }}
		})
	}

	pane := core.View{StyleFunc: func() core.Style { return fillPage.Merge(core.Style{Background: colors.Background()}) }, Children: []core.Node{
		core.View{Style: fillPage.Merge(core.Style{PaddingEnv: core.EnvEdges{Bottom: core.EnvKeyboard}}), Children: []core.Node{
			chat.FloatingConversation{State: conversation, JumpLabel: tr.ChatJumpToTheLatestMessage(), TestID: "messages",
				// One selection across the turns.
				Children: []core.Node{core.View{Selectable: true, Style: core.Style{Gap: 48, MinWidth: core.Pt(0)}, Children: []core.Node{
					core.Each(messages.Get, func(m msg) string { return m.id }, func(cx *core.Scope, m func() msg, _ func() int) core.Node {
						if core.Untracked(cx, m).kind == "date" {
							return dateSeparator(cx, func() string { return m().label })
						}
						return core.Func(func(cx *core.Scope) core.Node { return turnRow(cx, m) })
					}),
				}}},
				Composer: []core.Node{chat.Composer{Draft: draft.Get, OnDraftChange: draft.Set, Label: tr.ChatMessage(), Placeholder: tr.ChatPlaceholder(),
					SendLabel: tr.ChatSend(), StopLabel: tr.ChatStop(), Running: streaming, OnSend: send, OnStop: stop, Glass: true, State: conversation,
					Accessory: accessory, Trailing: trailing, Footer: footer, Attachments: attachments, HasAttachments: func() bool { return attachment.Get() != nil },
					OnDropFiles: dropped}},
			},
			core.Show{When: func() bool { return len(messages.Get()) == 0 }, Then: empty},
			// The connection banner.
			core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(8), Left: core.Pt(16), Right: core.Pt(16), Opacity: core.Alpha(0.001),
				Animated: &core.AnimatedStyle{Opacity: bannerValue, TranslateY: ui.Lerp(bannerValue, -64, 0)}}, Children: []core.Node{
				core.Dynamic(func(*core.Scope) core.Node {
					off := bannerKind.Get() == "offline"
					t, name, title, detail := ui.ToneUp, lucide.Check, tr.ChatBackOnline(), tr.ChatSendingYourMessage()
					if off {
						t, name, title, detail = ui.ToneWarn, lucide.WifiOff, tr.ChatYouReOffline(), tr.ChatYourMessageSendsWhenThe()
					}
					return core.View{StyleFunc: func() core.Style {
						return fin.Menu().Over(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, MinHeight: core.Pt(48), Padding: core.Symmetric(0, 14),
							CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, BorderColor: ui.Alpha(t.Color(cx), 0.35)})
					}, Children: []core.Node{
						core.View{Style: core.Style{Width: core.Pt(28), Height: core.Pt(28), CornerRadius: 14, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
							StyleFunc: ui.Filled(func() core.Color { return t.Tint(cx) }), Children: []core.Node{ui.Icon(name, 15, t.Shade(cx))}},
						core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 1}, Children: []core.Node{ui.Text(title, 13, ui.Semibold, colors.Text), ui.Text(detail, 13, ui.Regular, colors.Muted)}},
					}}
				}),
			}},
		}},
	}}
	if !wide {
		return pane
	}

	// Desktop: the scenarios as conversations, each opened afresh.
	current := core.NewSignal(cx, "")
	newChat := func() {
		stop()
		cx.Batch(func() {
			messages.Set(nil)
			attachment.Set(nil)
			current.Set("")
		})
	}
	openChat := func(id string) {
		newChat()
		current.Set(id)
		play(id)
	}
	var convs []core.Node
	for _, c := range chips {
		sel := func() bool { return current.Get() == c.id }
		detail := tr.ChatWeeksOfMessagesLoadedAt()
		if s, ok := findScenario(c.id); ok {
			detail = s.prompt
		}
		convs = append(convs, core.View{OnPress: func(core.PressEvent) { openChat(c.id) }, Role: core.RoleButton, Label: c.label, Selected: sel,
			StyleFunc: func() core.Style {
				return core.Style{Padding: core.Symmetric(9, 12), CornerRadius: ui.RadiusControl, Gap: 2, Selected: &core.Style{Background: colors.Pressed()}}
			},
			Children: []core.Node{
				core.Text{Content: c.label, MaxLines: 1, StyleFunc: func() core.Style {
					w := ui.Regular
					if sel() {
						w = ui.Semibold
					}
					return core.Style{FontSize: 13, FontWeight: w, Color: colors.Text()}
				}},
				core.Text{Content: detail, MaxLines: 1, Style: core.Style{FontSize: 12}, StyleFunc: ui.Colored(colors.Muted)},
			}})
	}
	return core.View{StyleFunc: func() core.Style {
		return fillPage.Merge(core.Style{Direction: core.Row, Background: colors.Background()})
	}, Children: []core.Node{
		core.Scroll{StyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(280), MinWidth: core.Pt(280), Borders: core.Edges{Right: 1}, BorderColors: core.EdgeColors{Right: colors.Line()}}
		}, Children: []core.Node{
			core.View{Style: core.Style{Padding: core.All(12), Gap: 2}, Children: append([]core.Node{
				ui.Press{OnPress: newChat, Label: tr.ChatNewChat(), Children: []core.Node{core.View{StyleFunc: func() core.Style {
					return fin.Raised().Over(core.Style{Height: core.Pt(40), CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous,
						Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 8, Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border()})
				},
					Children: []core.Node{ui.Icon(lucide.Plus, 15, colors.Text), ui.Text(tr.ChatNewChat(), 13, ui.Semibold, colors.Text)}}}},
				core.Text{Content: tr.ChatConversations(), Style: core.Style{FontSize: 11, FontWeight: ui.Semibold, LetterSpacing: 0.6, Padding: core.Edges{Top: 18, Right: 12, Bottom: 6, Left: 12}}, StyleFunc: ui.Colored(colors.Faint)},
			}, convs...)},
		}},
		pane,
	}}
}

// hasImage reports whether src is an image.
func hasImage(src core.ImageSource) bool { return src.URI != "" || len(src.Data) > 0 }
