package main

import (
	"github.com/oneo-me/nsgo/icons/lucide"
	"strings"
	"time"
	"unicode"

	"github.com/oneo-me/nsgo/base/textview"
	"github.com/oneo-me/nsgo/base/textview/highlight"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// readingStyles is the gallery's reading typography for every part of a
// TextView, base having none, as its StyleOf.
func readingStyles(cx *core.Scope) func(textview.Part) core.Style {
	return partStyles(cx, func() textview.Styles { return readingParts(cx) })
}

// partStyles is a TextView's StyleOf from styles by part, built again
// when what build reads (the palette) changes.
func partStyles(cx *core.Scope, build func() textview.Styles) func(textview.Part) core.Style {
	styles := ui.Themed(cx, build)
	return func(p textview.Part) core.Style { return styles()[p] }
}

// readingParts are the reading typography's styles by part, read where
// colorsOf's colors are.
func readingParts(cx *core.Scope) textview.Styles {
	colors := ui.ColorsOf(cx).Now()
	codeFont := core.Style{FontFamily: ui.Mono, FontSize: 13}
	alert := func(c core.Color) core.Style {
		return core.Style{Gap: 4, Padding: core.Symmetric(8, 12), Borders: core.Edges{Left: 3}, BorderColors: core.EdgeColors{Left: c}, CornerRadius: 4, Background: ui.Alpha(c, 0.08)}
	}
	return textview.Styles{
		textview.PartRoot:              {Gap: 12, Color: colors.Text, FontSize: 13, LineHeight: 23},
		textview.PartHeading:           {FontWeight: ui.Semibold, Margin: core.Edges{Top: 6}, Color: colors.Text},
		textview.PartHeading1:          {FontSize: 26, LineHeight: 32, LetterSpacing: -0.4},
		textview.PartHeading2:          {FontSize: 20, LineHeight: 26, Padding: core.Edges{Bottom: 6}, Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.Line}},
		textview.PartHeading3:          {FontSize: 15, LineHeight: 23},
		textview.PartHeading4:          {FontSize: 13, LineHeight: 21},
		textview.PartLink:              {Color: colors.Accent, TextDecoration: core.TextDecorationUnderline, TextDecorationColor: ui.Alpha(colors.Accent, 0.4)},
		textview.PartStrong:            {FontWeight: ui.Semibold},
		textview.PartEmphasis:          {FontStyle: core.FontStyleItalic},
		textview.PartStrikethrough:     {TextDecoration: core.TextDecorationLineThrough, Color: colors.Muted},
		textview.PartUnderline:         {TextDecoration: core.TextDecorationUnderline},
		textview.PartMark:              {Background: ui.Alpha(colors.Warn, 0.25)},
		textview.PartSmall:             {FontSize: 12, Color: colors.Muted},
		textview.PartSuperscript:       {FontSize: 11},
		textview.PartSubscript:         {FontSize: 11},
		textview.PartCode:              codeFont.Merge(core.Style{Background: colors.Raised, CornerRadius: 4, Color: colors.Code}),
		textview.PartKeyboard:          codeFont.Merge(core.Style{FontSize: 12, Background: colors.Raised, CornerRadius: 4, Color: colors.Text}),
		textview.PartBlockquote:        {Gap: 4, Padding: core.Edges{Left: 12}, Borders: core.Edges{Left: 3}, BorderColors: core.EdgeColors{Left: colors.Border}, Color: colors.Muted},
		textview.PartAlert:             alert(colors.Info),
		textview.PartAlertNote:         alert(colors.Info),
		textview.PartAlertTip:          alert(colors.Up),
		textview.PartAlertImportant:    alert(core.RGB(0x8b5cf6)),
		textview.PartAlertWarning:      alert(colors.Warn),
		textview.PartAlertCaution:      alert(colors.Danger),
		textview.PartAlertTitle:        {FontWeight: ui.Semibold},
		textview.PartAlertContent:      {Gap: 4},
		textview.PartList:              {Gap: 4},
		textview.PartListItem:          {Direction: core.Row, Gap: 8},
		textview.PartListMarker:        {Color: colors.Muted, MinWidth: core.Pt(14)},
		textview.PartTaskMarker:        {Width: core.Pt(16), Height: core.Pt(16), CornerRadius: 4, BorderWidth: 1, BorderColor: colors.FieldBorder, Background: colors.Field, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Margin: core.Edges{Top: 3}},
		textview.PartTaskMarkerChecked: {Background: colors.AccentFill, BorderColor: colors.AccentFill, Color: colors.OnAccent, FontSize: 11},
		textview.PartTaskDone:          {Color: colors.Muted},
		textview.PartListItemContent:   {FlexGrow: 1, FlexShrink: 1, Gap: 4},
		textview.PartCodeBlock:         {Background: colors.Inset, CornerRadius: ui.RadiusControl, BorderWidth: 1, BorderColor: colors.Line},
		textview.PartCodeBlockScroll:   {Padding: core.Symmetric(10, 12)},
		textview.PartCodeBlockCode:     codeFont.Merge(core.Style{LineHeight: 19, Color: colors.Code}),
		textview.PartCodeBlockHeader:   {Padding: core.All(4)},
		textview.PartCodeBlockAction:   {Padding: core.Symmetric(2, 8), CornerRadius: 6, Background: colors.Raised, BorderWidth: 1, BorderColor: colors.Border, FontSize: 12, Color: colors.Muted, Hover: &core.Style{Color: colors.Text}},
		textview.PartImage:             {CornerRadius: ui.RadiusControl},
		textview.PartTable:             {Gap: 6},
		textview.PartTableActions:      {Direction: core.Row, JustifyContent: core.JustifyEnd},
		textview.PartTableScroll:       {CornerRadius: 8, BorderWidth: 1, BorderColor: colors.Line},
		textview.PartTableRow:          {Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.Line}},
		textview.PartTableHead:         {Background: colors.Raised},
		textview.PartTableCell:         {Padding: core.Symmetric(6, 10)},
		textview.PartTableHeaderCell:   {FontWeight: ui.Semibold},
		textview.PartRule:              {Height: core.Pt(1), Background: colors.Line, Margin: core.Symmetric(4, 0)},
		textview.PartMath:              codeFont.Merge(core.Style{Color: core.RGB(0x8b5cf6)}),
		textview.PartMathBlock:         codeFont.Merge(core.Style{Color: core.RGB(0x8b5cf6), Background: core.RGBA(0x8b5cf6, 0.1), CornerRadius: 8, Padding: core.Symmetric(8, 12), AlignItems: core.AlignCenter}),
		textview.PartDetails:           {Gap: 6, Padding: core.All(10), CornerRadius: ui.RadiusControl, BorderWidth: 1, BorderColor: colors.Line},
		textview.PartSummary:           {FontWeight: ui.Semibold, Direction: core.Row, Gap: 6},
		textview.PartSummaryMarker:     {Color: colors.Muted},
		textview.PartFootnotes:         {Gap: 4, Padding: core.Edges{Top: 8}, Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Line}, FontSize: 13, Color: colors.Muted},
		textview.PartFootnote:          {Direction: core.Row, Gap: 6},
		textview.PartFootnoteLabel:     {Color: colors.Faint},
		textview.PartFootnoteReference: {Color: colors.Accent, FontSize: 11},
	}
}

// copyAction labels a code block's copy button, Copied while it is done.
func copyAction(cx *core.Scope, a textview.CodeBlockAction) core.Node {
	tr := locales.For(cx)
	return core.Text{ContentFunc: func() string {
		if a.Done() {
			return tr.TextviewCopied()
		}
		return tr.TextviewCopyCode()
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
	tr := locales.For(cx)
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
	return Card{Title: tr.TextviewStreaming(), Description: tr.TextviewAReplyAsAnAi(), Children: []core.Node{
		core.Dynamic(func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			colors := ui.ColorsOf(cx)
			on := streaming.Get()
			var caret core.Node
			if on {
				caret = core.View{Style: core.Style{Margin: core.Edges{Left: 2}, Width: core.Pt(8), Height: core.Pt(16), CornerRadius: 2,
					Animation: core.KeyframeAnimation{Keyframes: core.KeyframesPulse}}, StyleFunc: ui.Filled(colors.Accent)}
			}
			return textview.TextView{MarkdownFunc: streamed.Get, StyleOf: readingStyles(cx), Options: textview.Options{Streaming: on}, StreamingCaret: caret,
				Selectable: true, Highlighter: highlight.New(cx), CodeBlockActions: []textview.CodeAction{{ID: textview.CopyAction, Label: tr.TextviewCopyCode()}}, RenderCodeBlockAction: copyAction, TestID: "streamed"}
		}),
		core.Dynamic(func(*core.Scope) core.Node {
			l := tr.TextviewStreamIt()
			if streaming.Get() {
				l = tr.TextviewStreaming2()
			}
			return ui.Button{Text: l, Icon: lucide.Play, Variant: ui.ButtonSecondary, Disabled: streaming.Get, OnPress: play}
		}),
	}}
}

func markdownCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	pressed := core.NewSignal(cx, tr.TextviewNothingYet())
	viewing := core.NewSignal(cx, textview.Image{})
	return Card{Title: tr.TextviewMarkdown(), Description: tr.TextviewCommonmarkAndGithubSTables(), Children: []core.Node{
		textview.TextView{Markdown: tvMarkdown, StyleOf: readingStyles(cx), Selectable: true, TestID: "document", Highlighter: highlight.New(cx),
			CodeBlockActions: []textview.CodeAction{{ID: textview.CopyAction, Label: tr.TextviewCopyCode()}}, RenderCodeBlockAction: copyAction,
			OnLinkPress: func(e textview.LinkPressEvent) { pressed.Set(e.URL) }, OnImagePress: viewing.Set},
		Caption{Label: tr.TextviewLinkPressed(), Note: pressed.Get},
		imageViewer(cx, viewing.Get, func() { viewing.Set(textview.Image{}) }),
	}}
}

func htmlCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	viewing := core.NewSignal(cx, textview.Image{})
	return Card{Title: tr.TextviewHtml(), Description: tr.TextviewAnArticlePageSectionsFigures(), Children: []core.Node{
		textview.TextView{HTML: tvHTML, StyleOf: readingStyles(cx), Selectable: true, Highlighter: highlight.New(cx), OnImagePress: viewing.Set, TestID: "html"},
		imageViewer(cx, viewing.Get, func() { viewing.Set(textview.Image{}) }),
	}}
}

// chartBlock is a tiny bar chart of a ```chart block's lines ("Mon 12").
func chartBlock(cx *core.Scope, source string) core.Node {
	colors := ui.ColorsOf(cx)
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
			core.View{Style: core.Style{Width: core.Pct(100), CornerRadius: 6, Height: core.Pt(core.Points(b.value / most * 64))}, StyleFunc: ui.Filled(colors.Accent)},
			ui.Text(b.label, 11, ui.Regular, colors.Muted),
		}})
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Height: core.Pt(112), Direction: core.Row, AlignItems: core.AlignEnd, Gap: 8, CornerRadius: 12, BorderWidth: 1, BorderColor: colors.Border(),
			Background: colors.Inset(), Padding: core.All(12)}
	}, Children: cols}
}

func extensionsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
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
	return Card{Title: tr.TextviewExtensions(), Description: tr.TextviewMentionsAChartBlockA(), Children: []core.Node{
		textview.TextView{Markdown: tvExtended, StyleOf: readingStyles(cx),
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
						StyleFunc: ui.Filled(func() core.Color { return ui.Alpha(sky(), 0.15) }), Children: []core.Node{ui.Text("@"+node.Attributes["1"], 13, ui.Medium, sky)}}
				case "chart":
					return chartBlock(cx, node.Text)
				case "callout":
					return core.View{Style: core.Style{Direction: core.Row, Gap: 12, CornerRadius: 12, Padding: core.All(12)},
						StyleFunc: ui.Filled(func() core.Color { return ui.Alpha(emerald(), 0.1) }), Children: []core.Node{
							ui.Text("✦", 13, ui.Regular, emerald),
							core.View{Style: fill, Children: []core.Node{textview.TextView{Document: textview.Document{Blocks: node.Children}, StyleOf: readingStyles(cx)}}},
						}}
				}
				lbl := node.Attributes["label"]
				if lbl == "" {
					lbl = node.Name
				}
				return core.View{Style: core.Style{Gap: 4, CornerRadius: 12, BorderWidth: 1, Padding: core.All(12)},
					StyleFunc: func() core.Style { return core.Style{BorderColor: ui.Alpha(sky(), 0.4)} }, Children: []core.Node{
						core.Text{Content: lbl, Style: core.Style{FontSize: 12, FontWeight: ui.Semibold, TextTransform: core.TextTransformUppercase, LetterSpacing: 0.6}, StyleFunc: ui.Colored(sky)},
						textview.TextView{Document: textview.Document{Blocks: node.Children}, StyleOf: readingStyles(cx)},
					}}
			}},
	}}
}

const (
	tvQuestion = "Which **platforms** does NSGo run on?"
	tvAnswer   = "Every one with a native backend:\n\n- iOS and Android: UIKit and Android views\n- macOS, Windows and Linux: AppKit, Direct2D and GTK 4\n- the web: DOM elements"
)

// selectionAreaCard is two documents and a plain text that select as one,
// around a label that stays out.
func selectionAreaCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return Card{Title: tr.TextviewSelectableView(), Description: tr.TextviewOneSelectionAcrossTextsAnd(), Children: []core.Node{
		core.View{Selectable: true, Style: core.Style{Gap: 12}, Children: []core.Node{
			textview.TextView{Markdown: tvQuestion, StyleOf: readingStyles(cx)},
			textview.TextView{Markdown: tvAnswer, StyleOf: readingStyles(cx)},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
				core.Text{Content: tr.TextviewNotSelectable(), Selectable: core.SelectableOff, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Faint)},
				core.Text{Content: tr.TextviewAPlainTextJoinsThe(), Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
			}},
		}},
		Caption{Label: Hint(cx, tr.TextviewLongPressThenMoveThe(), tr.TextviewDragFromTheQuestionInto())},
	}}
}

// textViewDemo is base/textview's TextView over the whole page, one sample
// at a time: Markdown and HTML documents, the app's own syntax, a
// streaming reply and selection across documents.
func textViewDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	cards := []cardFunc{markdownCard, streamingCard, selectionAreaCard, extensionsCard, htmlCard}
	sample := core.NewSignal(cx, 0)
	return stagePage(cx, core.Dynamic(func(*core.Scope) core.Node {
		card := cards[sample.Get()]
		return core.Func(func(cx *core.Scope) core.Node { return demoPage(cx, pageProps{flush: true, still: true}, card) })
	}), Section{Title: tr.TerminalOptions(), Note: "textview.TextView", Children: []core.Node{
		choiceKnob("Markdown", sample, options([]int{0, 1, 2, 3, 4}, tr.TextviewMarkdown(), tr.TextviewStreaming(), tr.TextviewSelectableView(), tr.TextviewExtensions(), tr.TextviewHtml())...),
	}})
}
