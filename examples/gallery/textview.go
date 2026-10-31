package main

import (
	"strings"
	"time"
	"unicode"

	"github.com/oneo-me/nsgo/base/textview"
	"github.com/oneo-me/nsgo/base/textview/highlight"
	"github.com/oneo-me/nsgo/core"
)

// readingStyles is the gallery's reading typography for every part of a
// TextView, base having none, as its StyleFunc.
func readingStyles(cx *core.Scope) func(textview.Part) core.Style {
	return partStyles(cx, func() textview.Styles { return readingParts(cx) })
}

// partStyles is a TextView's StyleFunc from styles by part, built again
// when what build reads (the palette) changes.
func partStyles(cx *core.Scope, build func() textview.Styles) func(textview.Part) core.Style {
	styles := themed(cx, build)
	return func(p textview.Part) core.Style { return styles()[p] }
}

// readingParts are the reading typography's styles by part, read where
// colorsOf's colors are.
func readingParts(cx *core.Scope) textview.Styles {
	colors := colorsOf(cx).now()
	codeFont := core.Style{FontFamily: mono, FontSize: 13}
	alert := func(c core.Color) core.Style {
		return core.Style{Gap: 4, Padding: core.Symmetric(8, 12), Borders: core.Edges{Left: 3}, BorderColors: core.EdgeColors{Left: c}, CornerRadius: 4, Background: alpha(c, 0.08)}
	}
	return textview.Styles{
		textview.PartRoot:              {Gap: 12, Color: colors.text, FontSize: 15, LineHeight: 23},
		textview.PartHeading:           {FontWeight: semibold, Margin: core.Edges{Top: 6}, Color: colors.text},
		textview.PartHeading1:          {FontSize: 26, LineHeight: 32, LetterSpacing: -0.4},
		textview.PartHeading2:          {FontSize: 20, LineHeight: 26, Padding: core.Edges{Bottom: 6}, Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.line}},
		textview.PartHeading3:          {FontSize: 17, LineHeight: 23},
		textview.PartHeading4:          {FontSize: 15, LineHeight: 21},
		textview.PartLink:              {Color: colors.accent, TextDecoration: core.TextDecorationUnderline, TextDecorationColor: alpha(colors.accent, 0.4)},
		textview.PartStrong:            {FontWeight: semibold},
		textview.PartEmphasis:          {FontStyle: core.FontStyleItalic},
		textview.PartStrikethrough:     {TextDecoration: core.TextDecorationLineThrough, Color: colors.muted},
		textview.PartUnderline:         {TextDecoration: core.TextDecorationUnderline},
		textview.PartMark:              {Background: alpha(colors.warn, 0.25)},
		textview.PartSmall:             {FontSize: 12, Color: colors.muted},
		textview.PartSuperscript:       {FontSize: 11},
		textview.PartSubscript:         {FontSize: 11},
		textview.PartCode:              codeFont.Merge(core.Style{Background: colors.raised, CornerRadius: 4, Color: colors.code}),
		textview.PartKeyboard:          codeFont.Merge(core.Style{FontSize: 12, Background: colors.raised, CornerRadius: 4, Color: colors.text}),
		textview.PartBlockquote:        {Gap: 4, Padding: core.Edges{Left: 12}, Borders: core.Edges{Left: 3}, BorderColors: core.EdgeColors{Left: colors.border}, Color: colors.muted},
		textview.PartAlert:             alert(colors.info),
		textview.PartAlertNote:         alert(colors.info),
		textview.PartAlertTip:          alert(colors.up),
		textview.PartAlertImportant:    alert(core.RGB(0x8b5cf6)),
		textview.PartAlertWarning:      alert(colors.warn),
		textview.PartAlertCaution:      alert(colors.danger),
		textview.PartAlertTitle:        {FontWeight: semibold},
		textview.PartAlertContent:      {Gap: 4},
		textview.PartList:              {Gap: 4},
		textview.PartListItem:          {Direction: core.Row, Gap: 8},
		textview.PartListMarker:        {Color: colors.muted, MinWidth: core.Pt(14)},
		textview.PartTaskMarker:        {Width: core.Pt(16), Height: core.Pt(16), CornerRadius: 4, BorderWidth: 1, BorderColor: colors.fieldBorder, Background: colors.field, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Margin: core.Edges{Top: 3}},
		textview.PartTaskMarkerChecked: {Background: colors.accentFill, BorderColor: colors.accentFill, Color: colors.onAccent, FontSize: 11},
		textview.PartTaskDone:          {Color: colors.muted},
		textview.PartListItemContent:   {FlexGrow: 1, FlexShrink: 1, Gap: 4},
		textview.PartCodeBlock:         {Background: colors.inset, CornerRadius: radiusControl, BorderWidth: 1, BorderColor: colors.line},
		textview.PartCodeBlockScroll:   {Padding: core.Symmetric(10, 12)},
		textview.PartCodeBlockCode:     codeFont.Merge(core.Style{LineHeight: 19, Color: colors.code}),
		textview.PartCodeBlockHeader:   {Padding: core.All(4)},
		textview.PartCodeBlockAction:   {Padding: core.Symmetric(2, 8), CornerRadius: 6, Background: colors.raised, BorderWidth: 1, BorderColor: colors.border, FontSize: 12, Color: colors.muted, Hover: &core.Style{Color: colors.text}},
		textview.PartImage:             {CornerRadius: radiusControl},
		textview.PartTable:             {Gap: 6},
		textview.PartTableActions:      {Direction: core.Row, JustifyContent: core.JustifyEnd},
		textview.PartTableScroll:       {CornerRadius: 8, BorderWidth: 1, BorderColor: colors.line},
		textview.PartTableRow:          {Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.line}},
		textview.PartTableHead:         {Background: colors.raised},
		textview.PartTableCell:         {Padding: core.Symmetric(6, 10)},
		textview.PartTableHeaderCell:   {FontWeight: semibold},
		textview.PartRule:              {Height: core.Pt(1), Background: colors.line, Margin: core.Symmetric(4, 0)},
		textview.PartMath:              codeFont.Merge(core.Style{Color: core.RGB(0x8b5cf6)}),
		textview.PartMathBlock:         codeFont.Merge(core.Style{Color: core.RGB(0x8b5cf6), Background: core.RGBA(0x8b5cf6, 0.1), CornerRadius: 8, Padding: core.Symmetric(8, 12), AlignItems: core.AlignCenter}),
		textview.PartDetails:           {Gap: 6, Padding: core.All(10), CornerRadius: radiusControl, BorderWidth: 1, BorderColor: colors.line},
		textview.PartSummary:           {FontWeight: semibold, Direction: core.Row, Gap: 6},
		textview.PartSummaryMarker:     {Color: colors.muted},
		textview.PartFootnotes:         {Gap: 4, Padding: core.Edges{Top: 8}, Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.line}, FontSize: 13, Color: colors.muted},
		textview.PartFootnote:          {Direction: core.Row, Gap: 6},
		textview.PartFootnoteLabel:     {Color: colors.faint},
		textview.PartFootnoteReference: {Color: colors.accent, FontSize: 11},
	}
}

// copyAction labels a code block's copy button, Copied while it is done.
func copyAction(cx *core.Scope, a textview.CodeBlockAction) core.Node {
	return core.Text{ContentFunc: func() string {
		if a.Done() {
			return "Copied"
		}
		return "Copy code"
	}}
}

// streamTokens splits a reply into the chunks a model would send: words
// of up to four characters with the space before them.
func streamTokens(s string) []string {
	var out []string
	r := []rune(s)
	for i := 0; i < len(r); {
		j := i
		for j < len(r) && unicode.IsSpace(r[j]) {
			j++
		}
		k := j
		for k < len(r) && !unicode.IsSpace(r[k]) && k-j < 4 {
			k++
		}
		if k == i {
			k = i + 1
		}
		out = append(out, string(r[i:k]))
		i = k
	}
	return out
}

func streamingCard(cx *core.Scope) core.Node {
	streamed := core.NewSignal(cx, "")
	streaming := core.NewSignal(cx, false)
	var stop func()
	halt := func() {
		if stop != nil {
			stop()
			stop = nil
		}
		streaming.Set(false)
	}
	cx.Cleanup(halt)
	play := func() {
		halt()
		tokens := streamTokens(tvReply)
		i := 0
		streamed.Set("")
		streaming.Set(true)
		stop = every(cx, 40*time.Millisecond, func() {
			if i >= len(tokens) {
				halt()
				return
			}
			streamed.Set(streamed.GetUntracked() + tokens[i])
			i++
		})
	}
	return card(cx, cardProps{icon: iconChat, title: "Streaming", subtitle: "A reply as an AI model writes it, with a caret"},
		core.Dynamic(func(cx *core.Scope) core.Node {
			colors := colorsOf(cx)
			on := streaming.Get()
			var caret core.Node
			if on {
				caret = core.View{Style: core.Style{Margin: core.Edges{Left: 2}, Width: core.Pt(8), Height: core.Pt(16), CornerRadius: 2,
					Animation: core.KeyframeAnimation{Keyframes: core.KeyframesPulse}}, StyleFunc: filled(colors.accent)}
			}
			return textview.TextView{MarkdownFunc: streamed.Get, StyleFunc: readingStyles(cx), Options: textview.Options{Streaming: on}, StreamingCaret: caret,
				Selectable: true, Highlighter: highlight.New(cx), CodeBlockActions: []string{textview.CopyAction}, RenderCodeBlockAction: copyAction, TestID: "streamed"}
		}),
		core.Dynamic(func(*core.Scope) core.Node {
			l := "Stream it"
			if streaming.Get() {
				l = "Streaming…"
			}
			return button(buttonProps{label: l, icon: iconPlay, variant: secondary, disabled: streaming.Get}, play)
		}),
	)
}

func markdownCard(cx *core.Scope) core.Node {
	pressed := core.NewSignal(cx, "nothing yet")
	viewing := core.NewSignal(cx, textview.Image{})
	return card(cx, cardProps{icon: iconText, title: "Markdown", subtitle: "CommonMark and GitHub's: tables, tasks, alerts, footnotes, math, HTML"},
		textview.TextView{Markdown: tvMarkdown, StyleFunc: readingStyles(cx), Selectable: true, TestID: "document", Highlighter: highlight.New(cx),
			CodeBlockActions: []string{textview.CopyAction}, RenderCodeBlockAction: copyAction,
			OnLinkPress: pressed.Set, OnImagePress: viewing.Set},
		caption(cx, "Link pressed", pressed.Get),
		imageViewer(cx, viewing.Get, func() { viewing.Set(textview.Image{}) }),
	)
}

func htmlCard(cx *core.Scope) core.Node {
	viewing := core.NewSignal(cx, textview.Image{})
	return card(cx, cardProps{icon: iconText, title: "HTML", subtitle: "An article page: sections, figures, lists, tables, code"},
		textview.TextView{HTML: tvHTML, StyleFunc: readingStyles(cx), Selectable: true, Highlighter: highlight.New(cx), OnImagePress: viewing.Set, TestID: "html"},
		imageViewer(cx, viewing.Get, func() { viewing.Set(textview.Image{}) }),
	)
}

// chartBlock is a tiny bar chart of a ```chart block's lines ("Mon 12").
func chartBlock(cx *core.Scope, source string) core.Node {
	colors := colorsOf(cx)
	type bar struct {
		label string
		value float64
	}
	var bars []bar
	most := 1.0
	for _, line := range strings.Split(source, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		var v float64
		for _, c := range f[1] {
			if c >= '0' && c <= '9' {
				v = v*10 + float64(c-'0')
			}
		}
		bars = append(bars, bar{f[0], v})
		most = max(most, v)
	}
	var cols []core.Node
	for _, b := range bars {
		cols = append(cols, core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), AlignItems: core.AlignCenter, Gap: 4}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pct(100), CornerRadius: 6, Height: core.Pt(core.Points(b.value / most * 64))}, StyleFunc: filled(colors.accent)},
			text(b.label, 11, regular, colors.muted),
		}})
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Height: core.Pt(112), Direction: core.Row, AlignItems: core.AlignEnd, Gap: 8, CornerRadius: 12, BorderWidth: 1, BorderColor: colors.border(),
			Background: colors.inset(), Padding: core.All(12)}
	}, Children: cols}
}

func extensionsCard(cx *core.Scope) core.Node {
	sky := func() core.Color {
		if cx.Dark() {
			return core.RGB(0x7dd3fc)
		}
		return core.RGB(0x0284c7)
	}
	emerald := func() core.Color {
		if cx.Dark() {
			return core.RGB(0x34d399)
		}
		return core.RGB(0x059669)
	}
	return card(cx, cardProps{icon: iconComponents, title: "Extensions", subtitle: "Mentions, a chart block, a <Callout> tag, a :::note container"},
		textview.TextView{Markdown: tvExtended, StyleFunc: readingStyles(cx),
			Extensions: []textview.Extension{
				{Name: "mention", Pattern: mentionPattern},
				{Name: "chart", Language: "chart"},
				{Name: "callout", Tag: "callout"},
				{Name: "note", Container: "note"},
			},
			RenderCustom: func(cx *core.Scope, node textview.Custom) core.Node {
				switch node.Name {
				case "mention":
					return core.View{Style: core.Style{CornerRadius: 10, Padding: core.Symmetric(0, 8), Transform: core.Transform{TranslateY: 4}},
						StyleFunc: filled(func() core.Color { return alpha(sky(), 0.15) }), Children: []core.Node{text("@"+node.Attributes["1"], 14, medium, sky)}}
				case "chart":
					return chartBlock(cx, node.Text)
				case "callout":
					return core.View{Style: core.Style{Direction: core.Row, Gap: 12, CornerRadius: 12, Padding: core.All(12)},
						StyleFunc: filled(func() core.Color { return alpha(emerald(), 0.1) }), Children: []core.Node{
							text("✦", 15, regular, emerald),
							core.View{Style: fill, Children: []core.Node{textview.TextView{Document: textview.Document{Blocks: node.Children}, StyleFunc: readingStyles(cx)}}},
						}}
				}
				lbl := node.Attributes["label"]
				if lbl == "" {
					lbl = node.Name
				}
				return core.View{Style: core.Style{Gap: 4, CornerRadius: 12, BorderWidth: 1, Padding: core.All(12)},
					StyleFunc: func() core.Style { return core.Style{BorderColor: alpha(sky(), 0.4)} }, Children: []core.Node{
						core.Text{Content: lbl, Style: core.Style{FontSize: 12, FontWeight: semibold, TextTransform: core.TextTransformUppercase, LetterSpacing: 0.6}, StyleFunc: colored(sky)},
						textview.TextView{Document: textview.Document{Blocks: node.Children}, StyleFunc: readingStyles(cx)},
					}}
			}},
	)
}

const (
	tvQuestion = "Which **platforms** does NSGo run on?"
	tvAnswer   = "Every one with a native backend:\n\n- iOS and Android: UIKit and Android views\n- macOS, Windows and Linux: AppKit, Direct2D and GTK 4\n- the web: DOM elements"
)

// selectionAreaCard is two documents and a plain text that select as one,
// around a label that stays out.
func selectionAreaCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	return card(cx, cardProps{icon: iconText, title: "Selectable view", subtitle: "One selection across texts and TextViews"},
		core.View{Selectable: true, Style: core.Style{Gap: 12}, Children: []core.Node{
			textview.TextView{Markdown: tvQuestion, StyleFunc: readingStyles(cx)},
			textview.TextView{Markdown: tvAnswer, StyleFunc: readingStyles(cx)},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
				core.Text{Content: "Not selectable", Selectable: core.SelectableOff, Style: core.Style{FontSize: 13}, StyleFunc: colored(colors.faint)},
				core.Text{Content: "A plain Text joins the selection too.", Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 15}, StyleFunc: colored(colors.text)},
			}},
		}},
		caption(cx, hint(cx, "Long press, then move the handles from the question into the answer", "Drag from the question into the answer"), nil),
	)
}

// textViewDemo is base/textview's TextView: Markdown and HTML documents,
// the app's own syntax, a streaming reply and selection across documents.
func textViewDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{}, streamingCard, markdownCard, selectionAreaCard, extensionsCard, htmlCard)
}
