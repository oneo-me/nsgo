package main

import (
	"strings"

	"github.com/oneo-me/nsgo/base/richeditor"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
	uirich "github.com/oneo-me/nsgo/ui/richeditor"
)

const richDoc = "# Release notes\n\n" +
	"Type **Markdown** as you go: `#` and a space makes a heading, `-` a list, `1.` a numbered one, `>` a quote, and `@` mentions someone.\n\n" +
	"- Lists split on Enter\n- Tab indents an item, Shift-Tab outdents it\n  - Nested items keep their place\n\n" +
	"- [x] Undo groups what you type by time\n- [ ] Input methods compose in the line\n\n" +
	"> Select text for the bubble menu, or use Mod-B and Mod-I.\n\n" +
	"## Details\n\n" +
	"| Key | In a table |\n| --- | --- |\n| Tab | The next cell, a new row after the last |\n| Shift-Tab | The previous cell |\n\n" +
	"Drag a column's or row's border to resize it, and the grips on the table's edges to select and move rows and columns.\n\n" +
	"---\n\n```\nsession, _ := richeditor.NewSession(richeditor.SessionConfig{Markdown: text})\n```\n"

var richPeople = []string{"Ada Lovelace", "Alan Turing", "Grace Hopper", "Katherine Johnson", "Linus Torvalds", "Margaret Hamilton"}

// richKeys are the keys the mention list takes while it shows.
var richKeys = []string{"ArrowUp", "ArrowDown", "Enter", "Tab"}

func richEditorDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	exts := append(richeditor.StarterKit(richeditor.StarterOptions{Link: richeditor.LinkOptions{Open: func(href string) { cx.OpenURL(href, nil) }}}),
		richeditor.TableKit(richeditor.TableOptions{}),
		richeditor.Mention(richeditor.MentionOptions{Suggestion: richeditor.SuggestionOptions{CaptureKeys: richKeys}}),
		richeditor.CharacterCount(richeditor.CharacterCountOptions{}))
	session, err := richeditor.NewSession(richeditor.SessionConfig{Extensions: exts, Markdown: richDoc})
	if err != nil {
		return core.Text{Content: err.Error()}
	}
	rev := core.NewSignal(cx, 0)
	stop := session.Subscribe(func(*richeditor.State, []*richeditor.Transaction) { rev.Update(func(n int) int { return n + 1 }) })
	cx.Cleanup(stop)
	ref := &richeditor.Ref{}
	picked := core.NewSignal(cx, 0)
	mode := core.NewSignal(cx, "rich")

	active := func(name string, attrs richeditor.Attrs) func() bool {
		return func() bool {
			rev.Get()
			return session.IsActive(name, attrs)
		}
	}
	button := func(icon func() *core.SVG, label, command string, args richeditor.Attrs, on func() bool) core.Node {
		return ui.IconButton{Icon: icon, Label: label, Selected: on, Size: 28,
			Disabled: func() bool { rev.Get(); return !session.CanNamed(command, args) },
			OnPress: func() {
				session.RunNamed(command, args)
				session.Focus()
			}}
	}
	mark := func(icon func() *core.SVG, label, name, command string) core.Node {
		return button(icon, label, command, nil, active(name, nil))
	}
	marks := func() []core.Node {
		return []core.Node{
			mark(lucide.Bold, tr.RichBold(), "bold", "toggleBold"),
			mark(lucide.Italic, tr.RichItalic(), "italic", "toggleItalic"),
			mark(lucide.Strikethrough, tr.RichStrike(), "strike", "toggleStrike"),
			mark(lucide.Code, tr.RichCode(), "code", "toggleCode"),
			mark(lucide.Highlighter, tr.RichHighlight(), "highlight", "toggleHighlight"),
		}
	}
	bar := ui.Toolbar{Label: tr.RichToolbar(), Children: append(marks(),
		button(lucide.Heading1, tr.RichHeading(1), "toggleHeading", richeditor.Attrs{"level": 1}, active("heading", richeditor.Attrs{"level": 1})),
		button(lucide.Heading2, tr.RichHeading(2), "toggleHeading", richeditor.Attrs{"level": 2}, active("heading", richeditor.Attrs{"level": 2})),
		button(lucide.List, tr.RichBulletList(), "toggleBulletList", nil, active("bulletList", nil)),
		button(lucide.ListOrdered, tr.RichOrderedList(), "toggleOrderedList", nil, active("orderedList", nil)),
		button(lucide.ListChecks, tr.RichTaskList(), "toggleTaskList", nil, active("taskList", nil)),
		button(lucide.TextQuote, tr.RichQuote(), "toggleBlockquote", nil, active("blockquote", nil)),
		button(lucide.Table, tr.RichTable(), "insertTable", richeditor.Attrs{"rows": 3, "cols": 3, "withHeaderRow": true}, nil),
		button(lucide.TableCellsMerge, tr.RichMergeCells(), "mergeCells", nil, nil),
		button(lucide.Undo2, tr.RichUndo(), "undo", nil, nil),
		button(lucide.Redo2, tr.RichRedo(), "redo", nil, nil),
	)}

	suggestion := func() *richeditor.SuggestionState {
		rev.Get()
		return richeditor.SuggestionStateOf(session.State(), "mention")
	}
	matches := func() []string {
		s := suggestion()
		var out []string
		if s == nil || !s.Active {
			return nil
		}
		for _, p := range richPeople {
			if strings.Contains(strings.ToLower(p), strings.ToLower(s.Query)) {
				out = append(out, p)
			}
		}
		return out
	}
	insert := func(name string) {
		session.RunNamed("insertMention", richeditor.Attrs{"id": name, "label": name})
		picked.Set(0)
	}
	// What the list captured reaches it through the plugin's state.
	keys := 0
	cx.Effect(func() {
		s := suggestion()
		if s == nil || s.KeyCount == keys {
			return
		}
		keys = s.KeyCount
		cx.Untrack(func() {
			list := matches()
			if len(list) == 0 {
				return
			}
			switch s.LastKey {
			case "ArrowDown":
				picked.Set((picked.Get() + 1) % len(list))
			case "ArrowUp":
				picked.Set((picked.Get() + len(list) - 1) % len(list))
			default:
				insert(list[min(picked.Get(), len(list)-1)])
			}
		})
	})
	mentions := core.Show{When: func() bool { return len(matches()) > 0 }, Then: func(cx *core.Scope) core.Node {
		return core.Overlay{Placement: core.PlaceBottom, Offset: 4, TestID: "rich-mentions",
			AnchorBounds: func() core.Bounds {
				if s := suggestion(); s != nil {
					return ref.PageBounds(s.From)
				}
				return core.Bounds{}
			},
			StyleFunc: func() core.Style {
				return core.Style{Background: colors.Surface(), BorderWidth: 1, BorderColor: colors.Border(), CornerRadius: ui.RadiusControl, Padding: core.All(4), Width: core.Pt(220)}
			},
			Children: []core.Node{core.Each(matches, func(s string) string { return s }, func(cx *core.Scope, name func() string, i func() int) core.Node {
				return core.View{Role: core.RoleMenuItem, NoFocus: true, OnPress: func(core.PressEvent) { insert(name()) },
					StyleFunc: func() core.Style {
						st := core.Style{Padding: core.Symmetric(4, 8), CornerRadius: ui.RadiusControl - 2}
						if i() == picked.Get() {
							st.Background = ui.Alpha(colors.Accent(), 0.15)
						}
						return st
					}, Children: []core.Node{core.Text{ContentFunc: name, Style: ui.TypeBody}}}
			})}}
	}}
	editor := uirich.View{Editor: session, Ref: ref, TestID: "rich", Label: tr.DemoRichEditor(), Placeholder: tr.RichPlaceholder(), Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)},
		BubbleMenuPlacement: core.PlaceTop, BubbleMenuOffset: 6,
		BubbleMenu: func() core.Node {
			rev.Get()
			sel := session.State().Selection
			if _, text := sel.(*richeditor.TextSelection); !text || sel.Empty() {
				return nil
			}
			return ui.Toolbar{Label: tr.RichToolbar(), Children: marks()}
		}}
	source := core.Show{When: func() bool { return mode.Get() == "markdown" }, Then: func(*core.Scope) core.Node {
		return core.Scroll{Style: core.Style{FlexBasis: core.Pt(180), FlexShrink: 0, BorderWidth: 1}, StyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.Border(), CornerRadius: ui.RadiusControl}
		}, Children: []core.Node{core.Text{Selectable: core.SelectableOn, Style: ui.TypeCode.Merge(core.Style{Padding: core.All(12)}),
			StyleFunc: ui.Colored(colors.Text), ContentFunc: func() string { rev.Get(); return session.Markdown() }}}}
	}}
	status := core.Text{Style: ui.TypeBody, StyleFunc: ui.Colored(colors.Muted), ContentFunc: func() string {
		rev.Get()
		return tr.RichCount(richeditor.Words(session.State()), richeditor.Characters(session.State(), false))
	}}
	return stagePage(cx, core.View{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Gap: 8}, Children: []core.Node{
		bar,
		core.View{StyleFunc: func() core.Style {
			return core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), CornerRadius: ui.RadiusInset, BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Surface(), Overflow: core.Hidden}
		}, Children: []core.Node{editor}},
		mentions, source,
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
			status,
			ui.Button{Text: tr.RichMarkdown(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: func() {
				if mode.GetUntracked() == "rich" {
					mode.Set("markdown")
				} else {
					mode.Set("rich")
				}
			}},
		}},
	}})
}
