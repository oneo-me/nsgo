---
title: Text
description: Show text with spans, fonts, decorations, selection and documents.
group: Building UI
order: 13
---

## A text

`core.Text` is one paragraph, typeset by the platform's own text engine on the engine side and drawn by the main thread. Its size comes from its font and its width, and it wraps inside its box.

```go
core.Text{Content: "Hello from NSGo", Style: core.Style{
	FontSize:   15,
	LineHeight: 22,
	FontWeight: core.FontWeightSemibold,
}}
```

`Content` is fixed and `ContentFunc` follows a signal, so a text that changes reads it inside the function. `MaxLines` limits the lines shown, `EllipsizeMode` cuts the rest at its head, middle or tail, and `WhiteSpaceNoWrap` keeps a paragraph on one line. `AdjustsFontSizeToFit` shrinks the text until it fits its box and its line limit.

Text fields inherit as CSS does: a text inside a view with `FontSize`, `Color` or `LineHeight` takes them unless it sets its own. `TextAlign`, `TextTransform`, `LetterSpacing`, `TextDecoration`, `TextDecorationStyle`, `TextDecorationColor` and `TextShadowColor`, `TextShadowOffset` and `TextShadowRadius` style the lines and the glyphs.

## Runs and links

`Spans` makes a text of runs, each with a style of its own. A span with `OnPress` is a link, and a press on another span is the text's own `OnPress`.

```go
core.Text{Style: core.Style{FontSize: 15}, Spans: []core.Span{
	{Content: "By continuing you agree to the "},
	{Content: "terms", Style: core.Style{TextDecoration: core.TextDecorationUnderline}, OnPress: openTerms},
	{Content: " and the "},
	{Content: "privacy policy", Style: core.Style{TextDecoration: core.TextDecorationUnderline}, OnPress: openPrivacy},
	{Content: "."},
}}
```

`SpansFunc` gives spans again when what it reads changes, and only the parts that differ are prepared again, so a code editor's rows recolor as highlighting arrives. A span's `Background` and `CornerRadius` draw a box behind its characters on every line.

`Attachment` puts a view inside a text, in place of the span's content. It is laid out at its own size, wraps with the words around it and takes its own presses, so an icon, an image or a small button can sit in a line.

```go
core.Text{Style: core.Style{FontSize: 17, LineHeight: 24}, Spans: []core.Span{
	{Content: "Search with "},
	{Attachment: core.Icon{SVG: lucide.Search, Style: core.Style{FontSize: 17}}},
	{Content: " NSGo"},
}}
```

## Fonts

`FontFamily` names a family or a CSS list (`Inter, "Noto Sans SC", sans-serif`), each used for the characters those before it lack. `FontWeight`, `FontStyle`, `FontVariant`, `LineHeight`, `LineHeightMultiple`, `LetterSpacing` and `LetterSpacingEm` set the face and the spacing, and `core.FontMonospace` is the platform's fixed-pitch family.

Package the app's fonts under `resources/` and register them before the first view, or register a font's bytes while the app runs:

```go
func main() { nsgo.Run(Root, nsgo.Fonts("fonts/Inter-Regular.ttf", "fonts/Inter-Bold.ttf")) }

cx.RegisterFont(data) // a TrueType or OpenType face
```

A text laid out before a font arrives is prepared again. `cx.Env().FontScale()` is the user's text size, which texts follow unless `NoFontScaling` is set; `MaxFontSizeMultiplier` caps it.

## Selection and measurement

`SelectableOn` lets the user select and copy a text, and a `View` with `Selectable` makes its texts select together, in document order, as a web page's do. `SelectionColor` colors the highlight. `cx.MeasureText` measures a string in a font, for a column fitted to its text, and `OnTextLayout` reports the lines after each layout.

## Documents

`base/textview` shows Markdown or HTML with the same primitives: headings, paragraphs, lists, tables, code, footnotes, alerts, images, formulas and links.

```go
textview.TextView{
	Markdown: "## NSGo\n\nNative views from **Go**.",
	Styles:   ui.TextViewStyles(cx),
	OnLinkPress: func(e textview.LinkPressEvent) {
		if strings.HasPrefix(e.URL, "#") {
			return
		}
		cx.OpenURL(e.URL, nil)
	},
}
```

`textview.Parse` reads Markdown and `textview.ParseHTML` reads HTML into blocks the app can also build. `Options.Streaming` repairs a document that arrives in pieces, and `StreamingCaret` draws a caret at its end, for a reply written as it streams.

## Next

- [Images and icons](images.md): pictures and SVGs, in a text too.
- [Styles and themes](styles.md): colors and text fields that inherit.
- [Layout](layout.md): how a text takes its width.
