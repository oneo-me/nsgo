package main

import (
	"fmt"
	"strings"

	"github.com/oneo-me/nsgo/base/textview"
	"github.com/oneo-me/nsgo/base/textview/highlight"
	"github.com/oneo-me/nsgo/core"
)

// spinner is a ring that spins on the platform: the Spin keyframes.
func spinner(cx *core.Scope, size core.Points, tint func() core.Color) core.Node {
	colors := colorsOf(cx)
	if size == 0 {
		size = 14
	}
	if tint == nil {
		tint = colors.accent
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2, Borders: core.All(2),
			BorderColors: core.EdgeColors{Top: tint(), Right: colors.border(), Bottom: colors.border(), Left: colors.border()},
			Animation:    core.KeyframeAnimation{Keyframes: core.KeyframesSpin, Duration: ms(800)}}
	}}
}

// caret is the streaming caret: an inline block that pulses on the
// platform.
func caret(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	return core.View{Style: core.Style{Width: core.Pt(8), Height: core.Pt(16), Margin: core.Edges{Left: 2}, CornerRadius: 2,
		Animation: core.KeyframeAnimation{Keyframes: core.KeyframesPulse, Duration: ms(900)}}, StyleFunc: filled(colors.accent)}
}

// actionButton is a small icon button under a reply, a 44 pt target.
func actionButton(cx *core.Scope, name func() iconName, lbl string, active func() bool, t tone, onPress func()) core.Node {
	colors := colorsOf(cx)
	return press(pressProps{onPress: onPress, scale: 0.86, label: lbl, selected: active},
		core.View{Style: core.Style{Width: core.Pt(36), Height: core.Pt(36), CornerRadius: 18, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Margin: core.All(4)}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				bg := transparent
				if active() {
					bg = t.tint(cx)
				}
				return core.Style{Position: core.Absolute, Width: core.Pt(32), Height: core.Pt(32), CornerRadius: 16, Background: bg, Transition: core.Transition{Duration: ms(180)}}
			}},
			iconOf(name, 17, func() core.Color {
				if active() {
					return t.color(cx)
				}
				return colors.muted()
			}),
		}})
}

// dateSeparator is a row between days.
func dateSeparator(cx *core.Scope, lbl func() string) core.Node {
	colors := colorsOf(cx)
	line := core.View{Style: core.Style{FlexGrow: 1, Height: core.Pt(1)}, StyleFunc: filled(colors.line)}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(12, 16)}, Children: []core.Node{
		line, textf(lbl, 12, medium, colors.faint), line,
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
	colors := colorsOf(cx)
	fin := finOf(cx)
	right := core.Points(14)
	if onRemove != nil {
		right = 4
	}
	children := []core.Node{
		core.View{Style: core.Style{Width: core.Pt(30), Height: core.Pt(30), CornerRadius: 8, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: filled(func() core.Color { return toneDanger.tint(cx) }), Children: []core.Node{icon(glyphFileText, 16, colors.danger)}},
		core.View{Style: core.Style{Gap: 1}, Children: []core.Node{
			core.Text{Content: f.name, MaxLines: 1, Style: core.Style{FontSize: 14, MaxWidth: core.Pt(170)}, StyleFunc: colored(colors.text)},
			text(strings.ToUpper(fileKind(f.name))+" · "+f.size, 12, regular, colors.muted),
		}},
	}
	if onRemove != nil {
		children = append(children, press(pressProps{onPress: onRemove, label: "Remove file", scale: 0.85},
			core.View{Style: core.Style{Width: core.Pt(40), Height: core.Pt(44), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(iconClose, 14, colors.muted)}}))
	}
	return core.View{StyleFunc: func() core.Style {
		return fin.raised().over(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Height: core.Pt(48), Padding: core.Edges{Left: 10, Right: right},
			CornerRadius: radiusControl, BorderCurve: core.BorderCurveContinuous, Background: colors.raised(), BorderWidth: 1, BorderColor: colors.border(), AlignSelf: core.AlignEnd})
	}, Children: children}
}

// attachedPhoto is an attached image, rounded.
func attachedPhoto(cx *core.Scope, src core.ImageSource, w, h core.Points) core.Node {
	colors := colorsOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Width: core.Pt(w), Height: core.Pt(h), CornerRadius: radiusInset, BorderCurve: core.BorderCurveContinuous, Overflow: core.Hidden,
			Background: colors.raised(), BorderWidth: 1, BorderColor: colors.border()}
	}, Children: []core.Node{
		core.Image{Source: src, ResizeMode: core.Cover, Style: core.Style{Width: core.Pt(w), Height: core.Pt(h)}},
	}}
}

var toolStateLabel = map[toolState]string{toolPending: "Queued", toolRunning: "Running", toolDone: "Done", toolCancelled: "Stopped"}
var toolStateTone = map[toolState]tone{toolPending: toneNeutral, toolRunning: toneInfo, toolDone: toneUp, toolCancelled: toneWarn}

// toolCard is a tool call: a header with its state, then its result once
// done.
func toolCard(cx *core.Scope, t func() chatTool) core.Node {
	colors := colorsOf(cx)
	fin := finOf(cx)
	first := t()
	name, title, detail := iconTerminal, "Run code", first.lang+", sandboxed"
	switch first.kind {
	case "search":
		name, title, detail = glyphGlobe, "Web search", first.query
	case "weather":
		name, title, detail = iconSun, "Weather", first.city
	}
	return core.View{StyleFunc: func() core.Style {
		b := colors.border()
		if t().state == toolRunning {
			b = alpha(colors.info(), 0.4)
			if cx.Dark() {
				b = core.RGB(0x33405a)
			}
		}
		return fin.card().over(core.Style{Background: colors.surface(), CornerRadius: radiusInset, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, BorderColor: b,
			Transition: core.Transition{Duration: ms(250)}, Overflow: core.Hidden})
	}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(12)}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				return fin.raised().over(core.Style{Width: core.Pt(32), Height: core.Pt(32), CornerRadius: 9, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
					Background: colors.raised(), BorderWidth: 1, BorderColor: colors.border()})
			}, Children: []core.Node{icon(name, 16, colors.text)}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 1}, Children: []core.Node{
				text(title, 14, semibold, colors.text),
				core.Text{Content: detail, MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: colored(colors.muted)},
			}},
			core.Dynamic(func(*core.Scope) core.Node {
				st := t().state
				if st == toolRunning || st == toolPending {
					tint := colors.info
					if st == toolPending {
						tint = colors.faint
					}
					return hrow(8, spinner(cx, 14, tint), text(toolStateLabel[st], 13, regular, colors.muted))
				}
				return badge(cx, toolStateLabel[st], toolStateTone[st])
			}),
		}},
		toolBody(cx, t),
	}}
}

func toolBody(cx *core.Scope, t func() chatTool) core.Node {
	colors := colorsOf(cx)
	top := core.Style{Borders: core.Edges{Top: 1}}
	topLine := func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.line()}} }
	done := func() bool { return t().state == toolDone }
	switch t().kind {
	case "search":
		return core.Show{When: done, Then: func(*core.Scope) core.Node {
			var rows []core.Node
			for i, s := range t().sources {
				rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Height: core.Pt(48), Padding: core.Symmetric(0, 12)}, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pt(24), Height: core.Pt(24), CornerRadius: 6, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
						StyleFunc: filled(func() core.Color { return s.tone.tint(cx) }),
						Children:  []core.Node{text(strings.ToUpper(s.site[:1]), 12, bold, s.tone.shade(cx))}},
					core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 1}, Children: []core.Node{
						core.Text{Content: s.title, MaxLines: 1, Style: core.Style{FontSize: 14}, StyleFunc: colored(colors.text)},
						text(s.site, 12, regular, colors.faint),
					}},
					text(itoa(i+1), 12, regular, colors.faint),
				}})
			}
			return core.View{Style: top.Merge(core.Style{Padding: core.Symmetric(4, 0)}), StyleFunc: topLine, Children: rows}
		}}
	case "weather":
		return core.Show{When: done, Then: func(*core.Scope) core.Node {
			w := t()
			var bars []core.Node
			for i, temp := range w.hours {
				c := colors.raised
				if i == 0 {
					c = colors.warn
				}
				bars = append(bars, core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), AlignItems: core.AlignCenter, Gap: 4}, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pct(100), Height: core.Pt(core.Points(6 + (temp-w.low)*3)), CornerRadius: 3}, StyleFunc: filled(c)},
					text(fmt.Sprintf("%dh", (17+i)%24), 11, regular, colors.faint),
				}})
			}
			return core.View{Style: top.Merge(core.Style{Padding: core.All(12), Gap: 12}), StyleFunc: topLine, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd, Gap: 12}, Children: []core.Node{
					core.Text{Content: fmt.Sprintf("%d°", w.temp), Style: core.Style{FontSize: 40, FontWeight: semibold, LetterSpacing: -1}, StyleFunc: colored(colors.text)},
					core.View{Style: core.Style{FlexGrow: 1, Padding: core.Edges{Bottom: 7}, Gap: 2}, Children: []core.Node{
						text(w.condition, 15, regular, colors.text),
						text(fmt.Sprintf("H %d°  L %d°", w.high, w.low), 13, regular, colors.muted),
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
		return colors.code()
	}
	return core.View{Style: top, StyleFunc: topLine, Children: []core.Node{
		core.View{Style: core.Style{Padding: core.All(12)}, StyleFunc: filled(colors.inset), Children: []core.Node{
			core.Text{Content: t().code, Selectable: core.SelectableOn, Style: core.Style{FontFamily: mono, FontSize: 12, LineHeight: 18}, StyleFunc: colored(codeColor)},
		}},
		core.Show{When: done, Then: func(*core.Scope) core.Node {
			c := t()
			return core.View{StyleFunc: func() core.Style {
				return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(0, 12), Height: core.Pt(44),
					Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.line()}}
			}, Children: []core.Node{
				text("Output", 13, regular, colors.muted),
				core.Text{Content: c.output, Selectable: core.SelectableOn, Style: core.Style{FlexGrow: 1, FontFamily: mono, FontSize: 14}, StyleFunc: colored(colors.up)},
				text(fmt.Sprintf("exit 0 · %d ms", c.ms), 12, regular, colors.faint),
			}}
		}},
	}}
}

// errorRow is the reply's error: what went wrong and a Retry button.
func errorRow(cx *core.Scope, message func() string, onRetry func()) core.Node {
	colors := colorsOf(cx)
	fin := finOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.All(12), CornerRadius: radiusInset, BorderCurve: core.BorderCurveContinuous,
			Background: toneDanger.tint(cx), BorderWidth: 1, BorderColor: alpha(colors.danger(), 0.32)}
	}, Children: []core.Node{
		icon(glyphAlert, 18, colors.danger),
		core.Text{ContentFunc: message, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 14, LineHeight: 19}, StyleFunc: colored(colors.text)},
		press(pressProps{onPress: onRetry, label: "Retry"}, core.View{StyleFunc: func() core.Style {
			return fin.light().over(core.Style{Height: core.Pt(36), Padding: core.Symmetric(0, 14), CornerRadius: 18,
				Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Background: colors.light()})
		}, Children: []core.Node{
			icon(glyphRegenerate, 14, colors.onLight), text("Retry", 13, semibold, colors.onLight),
		}}),
	}}
}

// avatar is the assistant's: a small tile with the spark.
func avatar(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	fin := finOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return fin.raised().over(core.Style{Width: core.Pt(26), Height: core.Pt(26), CornerRadius: 8, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			Background: colors.raised(), BorderWidth: 1, BorderColor: colors.border()})
	}, Children: []core.Node{icon(iconSpark, 14, colors.accent)}}
}

// chatStyles is a reply's typography at a chat's scale: tighter than a
// document's.
func chatStyles(cx *core.Scope) func(textview.Part) core.Style {
	return partStyles(cx, func() textview.Styles {
		s := readingParts(cx)
		s[textview.PartRoot] = core.Style{Gap: 12, FontSize: 15, LineHeight: 22, Color: colorsOf(cx).text()}
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
					StyleFunc: chatStyles(cx), Highlighter: hl, CodeBlockActions: []string{textview.CopyAction}, RenderCodeBlockAction: copyAction,
					OnImagePress: func(im textview.Image) { used.Set(true); viewing.Set(im) }}
			}),
			core.Show{When: used.Get, Then: func(cx *core.Scope) core.Node {
				return imageViewer(cx, viewing.Get, func() { viewing.Set(textview.Image{}) })
			}},
		}}
	})
}
