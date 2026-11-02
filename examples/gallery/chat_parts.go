package main

import (
	"fmt"
	"strings"

	"github.com/oneo-me/nsgo/base/textview"
	"github.com/oneo-me/nsgo/base/textview/highlight"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
)

// spinner is a ring that spins on the platform: the Spin keyframes.
func spinner(cx *core.Scope, size core.Points, tint func() core.Color) core.Node {
	colors := ui.ColorsOf(cx)
	if size == 0 {
		size = 14
	}
	if tint == nil {
		tint = colors.Accent
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2, Borders: core.All(2),
			BorderColors: core.EdgeColors{Top: tint(), Right: colors.Border(), Bottom: colors.Border(), Left: colors.Border()},
			Animation:    core.KeyframeAnimation{Keyframes: core.KeyframesSpin, Duration: ms(800)}}
	}}
}

// caret is the streaming caret: an inline block that pulses on the
// platform.
func caret(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{Style: core.Style{Width: core.Pt(8), Height: core.Pt(16), Margin: core.Edges{Left: 2}, CornerRadius: 2,
		Animation: core.KeyframeAnimation{Keyframes: core.KeyframesPulse, Duration: ms(900)}}, StyleFunc: ui.Filled(colors.Accent)}
}

// actionButton is a small icon button under a reply, a 44 pt target.
func actionButton(cx *core.Scope, name func() ui.IconName, lbl string, active func() bool, t ui.Tone, onPress func()) core.Node {
	colors := ui.ColorsOf(cx)
	return ui.Press{OnPress: onPress, Scale: 0.86, Label: lbl, Selected: active, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(36), Height: core.Pt(36), CornerRadius: 18, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Margin: core.All(4)}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				bg := transparent
				if active() {
					bg = t.Tint(cx)
				}
				return core.Style{Position: core.Absolute, Width: core.Pt(32), Height: core.Pt(32), CornerRadius: 16, Background: bg, Transition: core.Transition{Duration: ms(180)}}
			}},
			ui.IconOf(name, 17, func() core.Color {
				if active() {
					return t.Color(cx)
				}
				return colors.Muted()
			}),
		}},
	}}
}

// dateSeparator is a row between days.
func dateSeparator(cx *core.Scope, lbl func() string) core.Node {
	colors := ui.ColorsOf(cx)
	line := core.View{Style: core.Style{FlexGrow: 1, Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(12, 16)}, Children: []core.Node{
		line, ui.Textf(lbl, 12, ui.Medium, colors.Faint), line,
	}}
}

// fileKind is the extension of a file's name, "file" without one.
func fileKind(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 && i < len(name)-1 {
		return name[i+1:]
	}
	return "file"
}

// fileChip is a file as a chip: an icon, the name and the size.
func fileChip(cx *core.Scope, f fileRef, onRemove func()) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	right := core.Points(14)
	if onRemove != nil {
		right = 4
	}
	children := []core.Node{
		core.View{Style: core.Style{Width: core.Pt(30), Height: core.Pt(30), CornerRadius: 8, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: ui.Filled(func() core.Color { return ui.ToneDanger.Tint(cx) }), Children: []core.Node{ui.Icon(ui.IconFileText, 16, colors.Danger)}},
		core.View{Style: core.Style{Gap: 1}, Children: []core.Node{
			core.Text{Content: f.name, MaxLines: 1, Style: core.Style{FontSize: 14, MaxWidth: core.Pt(170)}, StyleFunc: ui.Colored(colors.Text)},
			ui.Text(strings.ToUpper(fileKind(f.name))+" · "+f.size, 12, ui.Regular, colors.Muted),
		}},
	}
	if onRemove != nil {
		children = append(children, ui.Press{OnPress: onRemove, Label: "Remove file", Scale: 0.85, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(40), Height: core.Pt(44), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{ui.Icon(ui.IconClose, 14, colors.Muted)}},
		}})
	}
	return core.View{StyleFunc: func() core.Style {
		return fin.Raised().Over(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Height: core.Pt(48), Padding: core.Edges{Left: 10, Right: right},
			CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border(), AlignSelf: core.AlignEnd})
	}, Children: children}
}

// attachedPhoto is an attached image, rounded.
func attachedPhoto(cx *core.Scope, src core.ImageSource, w, h core.Points) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Width: core.Pt(w), Height: core.Pt(h), CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous, Overflow: core.Hidden,
			Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border()}
	}, Children: []core.Node{
		core.Image{Source: src, ResizeMode: core.Cover, Style: core.Style{Width: core.Pt(w), Height: core.Pt(h)}},
	}}
}

var toolStateLabel = map[toolState]string{toolPending: "Queued", toolRunning: "Running", toolDone: "Done", toolCancelled: "Stopped"}
var toolStateTone = map[toolState]ui.Tone{toolPending: ui.ToneNeutral, toolRunning: ui.ToneInfo, toolDone: ui.ToneUp, toolCancelled: ui.ToneWarn}

// toolCard is a tool call: a header with its state, then its result once
// done.
func toolCard(cx *core.Scope, t func() chatTool) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	first := t()
	name, title, detail := ui.IconTerminal, "Run code", first.lang+", sandboxed"
	switch first.kind {
	case "search":
		name, title, detail = ui.IconGlobe, "Web search", first.query
	case "weather":
		name, title, detail = ui.IconSun, "Weather", first.city
	}
	return core.View{StyleFunc: func() core.Style {
		b := colors.Border()
		if t().state == toolRunning {
			b = ui.Alpha(colors.Info(), 0.4)
			if cx.Dark() {
				b = core.RGB(0x33405a)
			}
		}
		return fin.Card().Over(core.Style{Background: colors.Surface(), CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, BorderColor: b,
			Transition: core.Transition{Duration: ms(250)}, Overflow: core.Hidden})
	}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(12)}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				return fin.Raised().Over(core.Style{Width: core.Pt(32), Height: core.Pt(32), CornerRadius: 9, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
					Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border()})
			}, Children: []core.Node{ui.Icon(name, 16, colors.Text)}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 1}, Children: []core.Node{
				ui.Text(title, 14, ui.Semibold, colors.Text),
				core.Text{Content: detail, MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
			}},
			core.Dynamic(func(*core.Scope) core.Node {
				st := t().state
				if st == toolRunning || st == toolPending {
					tint := colors.Info
					if st == toolPending {
						tint = colors.Faint
					}
					return ui.HStack{Gap: 8, Children: []core.Node{spinner(cx, 14, tint), ui.Text(toolStateLabel[st], 13, ui.Regular, colors.Muted)}}
				}
				return ui.Badge{Text: toolStateLabel[st], Tone: toolStateTone[st]}
			}),
		}},
		toolBody(cx, t),
	}}
}

func toolBody(cx *core.Scope, t func() chatTool) core.Node {
	colors := ui.ColorsOf(cx)
	top := core.Style{Borders: core.Edges{Top: 1}}
	topLine := func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}} }
	done := func() bool { return t().state == toolDone }
	switch t().kind {
	case "search":
		return core.Show{When: done, Then: func(*core.Scope) core.Node {
			var rows []core.Node
			for i, s := range t().sources {
				rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Height: core.Pt(48), Padding: core.Symmetric(0, 12)}, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pt(24), Height: core.Pt(24), CornerRadius: 6, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
						StyleFunc: ui.Filled(func() core.Color { return s.tone.Tint(cx) }),
						Children:  []core.Node{ui.Text(strings.ToUpper(s.site[:1]), 12, ui.Bold, s.tone.Shade(cx))}},
					core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 1}, Children: []core.Node{
						core.Text{Content: s.title, MaxLines: 1, Style: core.Style{FontSize: 14}, StyleFunc: ui.Colored(colors.Text)},
						ui.Text(s.site, 12, ui.Regular, colors.Faint),
					}},
					ui.Text(itoa(i+1), 12, ui.Regular, colors.Faint),
				}})
			}
			return core.View{Style: top.Merge(core.Style{Padding: core.Symmetric(4, 0)}), StyleFunc: topLine, Children: rows}
		}}
	case "weather":
		return core.Show{When: done, Then: func(*core.Scope) core.Node {
			w := t()
			var bars []core.Node
			for i, temp := range w.hours {
				c := colors.Raised
				if i == 0 {
					c = colors.Warn
				}
				bars = append(bars, core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), AlignItems: core.AlignCenter, Gap: 4}, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pct(100), Height: core.Pt(core.Points(6 + (temp-w.low)*3)), CornerRadius: 3}, StyleFunc: ui.Filled(c)},
					ui.Text(fmt.Sprintf("%dh", (17+i)%24), 11, ui.Regular, colors.Faint),
				}})
			}
			return core.View{Style: top.Merge(core.Style{Padding: core.All(12), Gap: 12}), StyleFunc: topLine, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd, Gap: 12}, Children: []core.Node{
					core.Text{Content: fmt.Sprintf("%d°", w.temp), Style: core.Style{FontSize: 40, FontWeight: ui.Semibold, LetterSpacing: -1}, StyleFunc: ui.Colored(colors.Text)},
					core.View{Style: core.Style{FlexGrow: 1, Padding: core.Edges{Bottom: 7}, Gap: 2}, Children: []core.Node{
						ui.Text(w.condition, 15, ui.Regular, colors.Text),
						ui.Text(fmt.Sprintf("H %d°  L %d°", w.high, w.low), 13, ui.Regular, colors.Muted),
					}},
				}},
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd, Gap: 6, Height: core.Pt(44)}, Children: bars},
			}}
		}}
	}
	codeColor := func() core.Color {
		if cx.Dark() {
			return core.RGB(0xcfc9bf)
		}
		return colors.Code()
	}
	return core.View{Style: top, StyleFunc: topLine, Children: []core.Node{
		core.View{Style: core.Style{Padding: core.All(12)}, StyleFunc: ui.Filled(colors.Inset), Children: []core.Node{
			core.Text{Content: t().code, Selectable: core.SelectableOn, Style: core.Style{FontFamily: ui.Mono, FontSize: 12, LineHeight: 18}, StyleFunc: ui.Colored(codeColor)},
		}},
		core.Show{When: done, Then: func(*core.Scope) core.Node {
			c := t()
			return core.View{StyleFunc: func() core.Style {
				return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(0, 12), Height: core.Pt(44),
					Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Line()}}
			}, Children: []core.Node{
				ui.Text("Output", 13, ui.Regular, colors.Muted),
				core.Text{Content: c.output, Selectable: core.SelectableOn, Style: core.Style{FlexGrow: 1, FontFamily: ui.Mono, FontSize: 14}, StyleFunc: ui.Colored(colors.Up)},
				ui.Text(fmt.Sprintf("exit 0 · %d ms", c.ms), 12, ui.Regular, colors.Faint),
			}}
		}},
	}}
}

// errorRow is the reply's error: what went wrong and a Retry button.
func errorRow(cx *core.Scope, message func() string, onRetry func()) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(12), CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous,
			Background: ui.ToneDanger.Tint(cx), BorderWidth: 1, BorderColor: ui.Alpha(colors.Danger(), 0.32)}
	}, Children: []core.Node{
		ui.Icon(ui.IconAlert, 18, colors.Danger),
		core.Text{ContentFunc: message, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 14, LineHeight: 19}, StyleFunc: ui.Colored(colors.Text)},
		ui.Press{OnPress: onRetry, Label: "Retry", Children: []core.Node{core.View{StyleFunc: func() core.Style {
			return fin.Light().Over(core.Style{Height: core.Pt(36), Padding: core.Symmetric(0, 14), CornerRadius: 18,
				Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Background: colors.Light()})
		}, Children: []core.Node{
			ui.Icon(ui.IconRegenerate, 14, colors.OnLight), ui.Text("Retry", 13, ui.Semibold, colors.OnLight),
		}}}},
	}}
}

// avatar is the assistant's: a small tile with the spark.
func avatar(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return fin.Raised().Over(core.Style{Width: core.Pt(26), Height: core.Pt(26), CornerRadius: 8, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border()})
	}, Children: []core.Node{ui.Icon(ui.IconSpark, 14, colors.Accent)}}
}

// chatStyles is a reply's typography at a chat's scale: tighter than a
// document's.
func chatStyles(cx *core.Scope) func(textview.Part) core.Style {
	return partStyles(cx, func() textview.Styles {
		s := readingParts(cx)
		s[textview.PartRoot] = core.Style{Gap: 12, FontSize: 15, LineHeight: 22, Color: ui.ColorsOf(cx).Text()}
		s[textview.PartHeading1] = core.Style{FontSize: 20, LineHeight: 26}
		s[textview.PartHeading2] = core.Style{FontSize: 17, LineHeight: 22}
		s[textview.PartHeading3] = core.Style{FontSize: 15, LineHeight: 22}
		return s
	})
}

// chatMarkdown is a reply's Markdown. While it streams, syntax its end
// leaves open shows as if closed and only the block being written renders
// again, the caret at its end. Each block selects, code blocks carry a
// copy action, and a press on an image opens the viewer.
func chatMarkdown(md func() string, streaming func() bool) core.Node {
	return core.Func(func(cx *core.Scope) core.Node {
		viewing := core.NewSignal(cx, textview.Image{})
		// The viewer is made on the first press, and kept for its fade out.
		used := core.NewSignal(cx, false)
		hl := highlight.New(cx)
		return core.View{Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node {
				on := streaming()
				var c core.Node
				if on {
					c = caret(cx)
				}
				return textview.TextView{MarkdownFunc: md, Options: textview.Options{Streaming: on}, StreamingCaret: c, Selectable: true, NoCodeBlockScrollbar: true,
					StyleOf: chatStyles(cx), Highlighter: hl, CodeBlockActions: []textview.CodeAction{{ID: textview.CopyAction, Label: "Copy code"}}, RenderCodeBlockAction: copyAction,
					OnImagePress: func(im textview.Image) { used.Set(true); viewing.Set(im) }}
			}),
			core.Show{When: used.Get, Then: func(cx *core.Scope) core.Node {
				return imageViewer(cx, viewing.Get, func() { viewing.Set(textview.Image{}) })
			}},
		}}
	})
}
