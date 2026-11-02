package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
)

const lorem = "Native text, typeset once on the engine with the platform's text engine and drawn on the main thread without any layout work."

// bodyStyle is body text's type; bodyInk its color, for a StyleFunc.
var bodyStyle = core.Style{FontSize: 15, LineHeight: 22}

func bodyInk(cx *core.Scope) func() core.Style { return ui.Colored(ui.ColorsOf(cx).Text) }

// linkStyle is a link's look, for a span's StyleFunc.
func linkStyle(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Color: colors.Accent(), TextDecoration: core.TextDecorationUnderline, TextDecorationColor: ui.Alpha(colors.Accent(), 0.5)}
	}
}

// specimenRow is a labeled specimen: a quiet label (and the property it
// shows, in code) above the text.
func specimenRow(cx *core.Scope, lbl, codeText string, first bool, children ...core.Node) core.Node {
	colors := ui.ColorsOf(cx)
	s := core.Style{Gap: 8}
	if !first {
		s.Padding, s.Borders = core.Edges{Top: 12}, core.Edges{Top: 1}
	}
	head := []core.Node{core.Text{Content: lbl, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)}}
	if codeText != "" {
		head = append(head, core.Text{Content: codeText, MaxLines: 1, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)})
	}
	var line func() core.Style
	if !first {
		line = func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}} }
	}
	return core.View{Style: s, StyleFunc: line,
		Children: append([]core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: head}}, children...)}
}

func linksCard(cx *core.Scope) core.Node {
	tapped := core.NewSignal(cx, "nothing yet")
	set := func(s string) func(core.PressEvent) { return func(core.PressEvent) { tapped.Set(s) } }
	b := bodyStyle
	b.FontSize, b.LineHeight = 16, 25
	return ui.Card{Icon: ui.IconText, Title: "Spans and links", Description: "Spans with presses of their own", Children: []core.Node{
		core.Text{OnPress: set("the sentence (not a link)"), Style: b, StyleFunc: bodyInk(cx), TestID: "links", Spans: []core.Span{
			{Content: "By continuing you agree to the "},
			{Content: "Terms of service", StyleFunc: linkStyle(cx), OnPress: set("Terms of service")},
			{Content: " and the "},
			{Content: "Privacy policy", StyleFunc: linkStyle(cx), OnPress: set("Privacy policy")},
			{Content: ", which mention "},
			{Content: "bold ", Style: core.Style{FontWeight: ui.Bold}},
			{Content: "links", Style: core.Style{FontWeight: ui.Bold}, StyleFunc: linkStyle(cx), OnPress: set("a link inside bold text")},
			{Content: " too", Style: core.Style{FontWeight: ui.Bold}},
			{Content: "."},
		}},
		ui.Caption{Label: "Pressed", Note: tapped.Get},
	}}
}

func inlineCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	fin := ui.FinOf(cx)
	likes := core.NewSignal(cx, 3)
	b := bodyStyle
	b.FontSize, b.LineHeight = 16, 28
	return ui.Card{Icon: ui.IconImage, Title: "Inline views and images", Description: "They wrap with the line", Children: []core.Node{
		core.Text{Style: b, StyleFunc: bodyInk(cx), Spans: []core.Span{
			{Content: "Made with "},
			{Attachment: core.Image{Source: core.ImageSource{Data: logoPNG}, Style: core.Style{Width: core.Pt(18), Height: core.Pt(18), CornerRadius: 5}}},
			{Content: " NSGo. Status "},
			{Attachment: core.View{StyleFunc: func() core.Style {
				return core.Style{Background: tints.Up(), CornerRadius: 9, Padding: core.Symmetric(0, 8), Height: core.Pt(18), Direction: core.Row,
					AlignItems: core.AlignCenter, Gap: 4}
			}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(6), Height: core.Pt(6), CornerRadius: 3}, StyleFunc: ui.Filled(colors.Up)},
				core.Text{Content: "LIVE", Style: core.Style{FontSize: 11, FontWeight: ui.Bold, LetterSpacing: 0.5}, StyleFunc: ui.Colored(colors.Up)},
			}}},
			{Content: " and a button inside the sentence "},
			{Attachment: ui.Press{OnPress: func() { likes.Update(func(n int) int { return n + 1 }) }, Scale: 0.9, Label: "Like", HitSlop: 10, Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					return fin.Raised().Over(core.Style{Background: colors.Raised(), CornerRadius: 11, Padding: core.Symmetric(0, 9), Height: core.Pt(22),
						JustifyContent: core.JustifyCenter, BorderWidth: 1, BorderColor: colors.Border()})
				},
					Children: []core.Node{ui.Textf(func() string { return fmt.Sprintf("♥ %d", likes.Get()) }, 13, ui.Regular, colors.Accent)}},
			}}},
			{Content: " that takes presses of its own."},
		}},
	}}
}

func fontsCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	glow := func() core.Color {
		if cx.Dark() {
			return ui.Alpha(colors.Accent(), 0.6)
		}
		return ui.Alpha(colors.Accent(), 0.3)
	}
	drop := func() core.Color {
		if cx.Dark() {
			return ui.Black(0.9)
		}
		return ui.Paper(0.22)
	}
	return ui.Card{Icon: ui.IconStyles, Title: "Type", Description: "Custom fonts, weights, styles, spacing", Children: []core.Node{
		specimenRow(cx, "Custom fonts", "nsgo.Fonts", true,
			core.Text{Content: "Custom fonts", Style: core.Style{FontFamily: "Pacifico", FontSize: 34, LineHeight: 52}, StyleFunc: ui.Colored(colors.Accent)},
			core.Text{Spans: []core.Span{{Content: "Go Smallcaps "}, {Content: "and its italic", Style: core.Style{FontStyle: core.FontStyleItalic}}},
				TestID: "app-font", Style: core.Style{FontFamily: "Go Smallcaps", FontSize: 20}, StyleFunc: ui.Colored(colors.Text)},
		),
		specimenRow(cx, "Weight, style, spacing", "FontWeight, FontStyle", false,
			core.Text{Style: core.Style{FontSize: 17}, StyleFunc: ui.Colored(colors.Text), Spans: []core.Span{
				{Content: "Light ", Style: core.Style{FontWeight: core.FontWeightLight}},
				{Content: "Bold ", Style: core.Style{FontWeight: ui.Bold}},
				{Content: "Italic ", Style: core.Style{FontStyle: core.FontStyleItalic}},
				{Content: "Both ", Style: core.Style{FontWeight: ui.Bold, FontStyle: core.FontStyleItalic}},
				{Content: "Spaced", Style: core.Style{LetterSpacing: 4}, StyleFunc: ui.Colored(colors.Muted)},
			}}),
		specimenRow(cx, "Decorations", "TextDecoration", false,
			core.Text{Style: core.Style{FontSize: 17}, StyleFunc: ui.Colored(colors.Text), Spans: []core.Span{
				{Content: "Underline", Style: core.Style{TextDecoration: core.TextDecorationUnderline}},
				{Content: " "},
				{Content: "Strikethrough", StyleFunc: func() core.Style {
					return core.Style{TextDecoration: core.TextDecorationLineThrough, TextDecorationColor: colors.Danger(), Color: colors.Muted()}
				}},
				{Content: " "},
				{Content: "Wavy", StyleFunc: func() core.Style {
					return core.Style{TextDecoration: core.TextDecorationUnderline, TextDecorationStyle: core.DecorationWavy, TextDecorationColor: colors.Info(), Color: colors.Info()}
				}},
			}}),
		specimenRow(cx, "Transforms", "TextTransform", false,
			core.Text{Style: core.Style{FontSize: 17}, StyleFunc: ui.Colored(colors.Text), Spans: []core.Span{
				{Content: "uppercase ", Style: core.Style{TextTransform: core.TextTransformUppercase}},
				{Content: "capitalized words here", Style: core.Style{TextTransform: core.TextTransformCapitalize}, StyleFunc: ui.Colored(colors.Muted)},
			}}),
		specimenRow(cx, "Shadows", "TextShadow…", false,
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
				core.Text{Content: "Glow", Style: core.Style{FontSize: 26, FontWeight: core.FontWeight(800), TextShadowRadius: 5},
					StyleFunc: func() core.Style { return core.Style{Color: colors.Accent(), TextShadowColor: glow()} }},
				core.View{StyleFunc: func() core.Style {
					return fin.Raised().Over(core.Style{Height: core.Pt(48), Padding: core.Symmetric(0, 14), JustifyContent: core.JustifyCenter, CornerRadius: ui.RadiusControl,
						BorderCurve: core.BorderCurveContinuous, Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border()})
				}, Children: []core.Node{
					core.Text{Content: "Drop shadow", Style: core.Style{FontSize: 26, FontWeight: core.FontWeight(800),
						TextShadowOffset: core.Point{X: 2, Y: 3}, TextShadowRadius: 1.5}, StyleFunc: func() core.Style { return core.Style{Color: colors.Info(), TextShadowColor: drop()} }},
				}},
			}}),
	}}
}

func scalingCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	env := cx.Env()
	b := bodyStyle
	b.FontSize = 17
	return ui.Card{Icon: ui.IconSearch, Title: "System text size", Description: "Env.FontScale and its limits", Actions: []core.Node{ui.Textf(func() string {
		s := env.FontScale()
		if s == 0 {
			s = 1
		}
		return fmt.Sprintf("%.2f×", s)
	}, 15, ui.Medium, colors.Accent)}, Children: []core.Node{
		specimenRow(cx, "Default", "", true, core.Text{Content: "Scales with the system setting", Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Fixed", "NoFontScaling", false, core.Text{Content: "Fixed size", NoFontScaling: true, Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Capped", "MaxFontSizeMultiplier: 1.2", false, core.Text{Content: "At most 1.2×", MaxFontSizeMultiplier: 1.2, Style: b, StyleFunc: bodyInk(cx)}),
	}}
}

func truncationCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	b := bodyStyle
	centered := b
	centered.TextAlign = core.TextAlignCenter
	return ui.Card{Icon: ui.IconLists, Title: "Truncation", Description: "MaxLines and EllipsizeMode", Children: []core.Node{
		specimenRow(cx, "Two lines, tail", "MaxLines: 2", true, core.Text{Content: lorem, MaxLines: 2, Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "One line, head", "EllipsizeHead", false, core.Text{Content: lorem, MaxLines: 1, EllipsizeMode: core.EllipsizeHead, Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "One line, middle", "EllipsizeMiddle", false, core.Text{Content: lorem, MaxLines: 1, EllipsizeMode: core.EllipsizeMiddle, Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "One line, clip", "EllipsizeClip", false, core.Text{Content: lorem, MaxLines: 1, EllipsizeMode: core.EllipsizeClip, Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Centered, three lines", "TextAlignCenter", false, core.Text{Content: lorem + " " + lorem, MaxLines: 3, Style: centered, StyleFunc: ui.Colored(colors.Muted)}),
	}}
}

const (
	chineseText  = "原生文本在引擎线程上一次排版，在主线程上直接绘制。中文在任意两个汉字之间都可以换行，但句号、逗号和右括号不会出现在行首，左括号也不会留在行尾（就像这样）。"
	japaneseText = "日本語の文章は、ひらがな・カタカナ・漢字に Latin の単語や数字（2024年）を混ぜて組みます。句読点は行頭に来ません。"
	arabicText   = "النص الأصلي يُنضَّد مرة واحدة على خيط المحرك ثم يُرسم على الخيط الرئيسي دون أي عمل إضافي في التخطيط."
)

func cjkCard(cx *core.Scope) core.Node {
	b := bodyStyle
	return ui.Card{Icon: ui.IconText, Title: "Chinese and Japanese", Description: "Lines break between characters, never before closing punctuation", Children: []core.Node{
		specimenRow(cx, "Chinese paragraph", "", true, core.Text{Content: chineseText, Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Japanese: kana, kanji and Latin", "", false, core.Text{Content: japaneseText, Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Two lines, tail", "MaxLines: 2", false, core.Text{Content: chineseText, MaxLines: 2, Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "One line, middle", "EllipsizeMiddle", false, core.Text{Content: japaneseText, MaxLines: 1, EllipsizeMode: core.EllipsizeMiddle, Style: b, StyleFunc: bodyInk(cx)}),
	}}
}

func rtlTextCard(cx *core.Scope) core.Node {
	b := bodyStyle
	rtl := b
	rtl.WritingDirection = core.WritingRTL
	return ui.Card{Icon: ui.IconText, Title: "Right to left", Description: "Arabic and Hebrew, mixed with Latin text and numbers", Children: []core.Node{
		specimenRow(cx, "Arabic paragraph", "WritingRTL", true, core.Text{Content: arabicText, Style: rtl, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Arabic, English and numbers in one line", "", false, core.Text{Content: "تم شحن الطلب رقم 4521 عبر DHL في 12 مارس، والتسليم خلال 3 أيام.", Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Hebrew", "", false, core.Text{Content: "שלום! הגרסה 2.0 של NSGo זמינה עכשיו (גם ב-Linux).", Style: b, StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "One line, tail", "MaxLines: 1", false, core.Text{Content: arabicText, MaxLines: 1, Style: b, StyleFunc: bodyInk(cx)}),
	}}
}

func alignmentCard(cx *core.Scope) core.Node {
	const sentence = "Short, aligned in its box"
	const paragraph = "Justified text spreads the spaces of every line across the box, except on the last line of a paragraph, which stays at its start."
	aligned := func(a core.TextAlign) core.Style { s := bodyStyle; s.TextAlign = a; return s }
	return ui.Card{Icon: ui.IconLists, Title: "Alignment", Description: "TextAlign in left-to-right and right-to-left text", Children: []core.Node{
		specimenRow(cx, "Start", "TextAlignStart", true, core.Text{Content: sentence, Style: aligned(core.TextAlignStart), StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Center", "TextAlignCenter", false, core.Text{Content: sentence, Style: aligned(core.TextAlignCenter), StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "End", "TextAlignEnd", false, core.Text{Content: sentence, Style: aligned(core.TextAlignEnd), StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Justify", "TextAlignJustify", false, core.Text{Content: paragraph, Style: aligned(core.TextAlignJustify), StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Justify, Chinese", "TextAlignJustify", false, core.Text{Content: chineseText, Style: aligned(core.TextAlignJustify), StyleFunc: bodyInk(cx)}),
		specimenRow(cx, "Start, in Arabic", "TextAlignStart", false, core.Text{Content: arabicText, Style: aligned(core.TextAlignStart), StyleFunc: bodyInk(cx)}),
	}}
}

func selectionCard(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	b := bodyStyle
	ink := func() core.Style {
		return core.Style{Color: colors.Text(), SelectionColor: ui.Alpha(colors.Accent(), 0.35)}
	}
	return ui.Card{Icon: ui.IconText, Title: "Selection", Description: ui.Hint(cx, "Long press to select", "Drag, or double or triple click"), Actions: []core.Node{ui.Badge{Text: "NSGo"}}, Children: []core.Node{
		specimenRow(cx, "On its own", "SelectableOn", true, core.Text{Content: "This text selects on its own.", Selectable: core.SelectableOn, Style: b, StyleFunc: ink}),
		specimenRow(cx, "Across texts", "View.Selectable", false,
			core.View{Selectable: true, Style: core.Style{Gap: 8}, Children: []core.Node{
				core.Text{Content: "A selectable view selects across its texts, as a web page does.", Style: b, StyleFunc: ink},
				core.Text{Content: "Start in one, end in another; copy them as one.", Style: b, StyleFunc: ink},
				core.Text{Content: "This one stays out of the selection.", Selectable: core.SelectableOff, Style: b,
					StyleFunc: func() core.Style { return ink().Merge(core.Style{Color: colors.Faint()}) }},
			}}),
	}}
}

// textDemo is core.Text: spans and links, inline views, fonts, the
// system's text size, truncation, Chinese, Japanese and right-to-left
// text, alignment and selection.
func textDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{}, linksCard, inlineCard, fontsCard, scalingCard, truncationCard, cjkCard, rtlTextCard, alignmentCard, selectionCard)
}
