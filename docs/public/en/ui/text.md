---
title: Text
description: Paragraphs, spans, links, fonts and documents.
group: Components
order: 61
---

`core.Text` is one paragraph typeset by the platform's text engine. `ui.Text` and `ui.Textf` are helpers for a line in `ui`'s type; the element is still a `core.Text`, so it takes `Style` and `Spans` too.

```go
ui.Text("Settings", 20, ui.Bold, colors.Text)
ui.Textf(func() string { return fmt.Sprintf("%d files", n.Get()) }, 13, ui.Regular, colors.Muted)
```

## Content

`Content` is fixed and `ContentFunc` follows a signal. `MaxLines` limits the lines and `EllipsizeMode` cuts the rest at its head, middle or tail; `Style.WhiteSpaceNoWrap` keeps a paragraph on one line. `TextAlign`, `TextTransform`, `LineHeight` and `LetterSpacing` style the paragraph, and `OnTextLayout` reports its lines after layout.

```go
core.Text{ContentFunc: func() string { return fmt.Sprintf("Pressed %d times", count.Get()) },
	MaxLines: 2, EllipsizeMode: core.EllipsizeTail, Style: core.Style{FontSize: 15}}
```

## Spans and links

`Spans` makes a text of runs, each with a style of its own. A span with `OnPress` is a link, and its `Background` and `CornerRadius` draw behind its characters on every line. `SpansFunc` gives the runs again when what it reads changes, for highlighted code or a changing sentence.

```go
core.Text{Style: core.Style{FontSize: 15}, Spans: []core.Span{
	{Content: "Read the "},
	{Content: "guide", Style: core.Style{TextDecoration: core.TextDecorationUnderline}, OnPress: openGuide},
	{Content: " before you start."},
}}
```

`Attachment` puts a view in a line, in place of the span's content: an icon, an image or a small button. It is laid out at its own size, wraps with the words and takes its own presses.

## Fonts and style

`FontFamily` names a family or a CSS list. `FontSize`, `FontWeight`, `FontStyle`, `FontVariant`, `LineHeight`, `LetterSpacing`, `TextDecoration` and `TextShadowColor` and its offset and radius style the text. `core.FontMonospace` is the platform's fixed-pitch family.

```go
core.Text{Content: "fn main() {}", Style: core.Style{FontFamily: core.FontMonospace, FontSize: 13}}
```

Register packaged fonts with `nsgo.Fonts` before the first view, or register a face while the app runs with `cx.RegisterFont`. A text prepared before a font arrives is prepared again.

## Documents

`base/textview.TextView` shows Markdown or HTML with the same primitives: headings, paragraphs, lists, tables, code, footnotes, alerts, images, formulas and links. `textview.Parse` reads Markdown and `textview.ParseHTML` HTML, and `Options.Streaming` repairs a document still arriving.

```go
textview.TextView{Markdown: "## Notes\n\nA **native** document.", Styles: ui.TextViewStyles(cx),
	OnLinkPress: func(e textview.LinkPressEvent) { open(e.URL) }}
```

## Selection and accessibility

`SelectableOn` lets the user select and copy a text. A `core.View` with `Selectable` makes its texts select together in document order. A `Role` such as `RoleHeader` tells assistive technology what the text is, and `Label` replaces it when the words are not the label.

## Platform

The platform's text engine lays the text out, so line breaks, fonts and input methods follow it. CJK line breaking, emoji and right-to-left text follow the system. The app can read the text size with `cx.Env().FontScale()`, and a text follows it unless `NoFontScaling` is set.

## Related

- [Text](../text.md): the full guide to text, spans, fonts and documents.
- [Images and icons](../images.md): inline images and icons.
- [Accessibility](../accessibility.md): headings, labels and selection.
- The gallery's Text and TextView pages.
