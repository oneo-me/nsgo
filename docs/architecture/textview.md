# Text view

`base/textview` is NSH's `TextView` (`nsh/docs/dev/text-view.md`): a document of Markdown, HTML or the app's own blocks, shown with `Text`, `View`, `Scroll` and `Image`. Read this before changing the document model, the parser, the parts or highlighting.

## Layers

| Package | What |
| --- | --- |
| `base/textview` | `Document` and its blocks and inlines, `Parse`, `ParseHTML`, `Extension`, and the unstyled `TextView`, styled by `Part` |
| `base/textview/highlight` | A `Highlighter` from chroma's lexers in NSH's light and dark palette, following the system's appearance |

Highlighting is its own package because chroma's lexers add about 4.5 MB to a binary, against about 0.3 MB for goldmark. Apps opt in with `Highlighter: highlight.New(cx)`.

`highlight.New` keeps the last 64 blocks' tokens for the process and measures each language's lexing speed: a block is colored from the cache, lexed on the spot when its language should take under 2 ms, and otherwise shown plain, or with the colors of the block it grew from, and lexed on a goroutine whose result colors it again. This keeps lexing off the engine goroutine, as NSH's highlighter does with its blocking thread.

## Parsing

`Parse` reads Markdown with goldmark's CommonMark parser and its GFM and footnote extensions, then turns goldmark's tree into `Document` values. The model is plain Go values, so apps build documents themselves and compare them with `reflect.DeepEqual`.

- Alerts are blockquotes whose first line is `[!NOTE]`, `[!TIP]`, `[!IMPORTANT]`, `[!WARNING]` or `[!CAUTION]`.
- Heading anchors are GitHub's slugs, made unique per document.
- Formulas are always on, as NSH's default: `$…$` and `$$…$$` in text, a `$$` fence on lines of its own and ```` ```math ```` fences read as `Math`.
- A pair of single dollars stays prose unless its TeX, one space dropped inside each end, neither starts nor ends with a space and no digit follows it, so prices stay text.
- TeX's own `\(…\)` and `\[…\]` are rewritten as dollars before parsing, outside code, as NSH's `latex.rs` does.
- Escapes and character references in text resolve as goldmark's HTML output resolves them.
- HTML inside reads as `ParseHTML` reads it.
- An HTML block that opens an element a later block closes (`<details>`, Markdown, `</details>`) holds the Markdown between them. So does a formatting tag in a paragraph (`<b>x *y*</b>`), as gpui-kit pairs them.
- Other paired tags read as one HTML fragment from the source between them.
- `ParseWith` takes NSH's `MarkdownOptions` as `Options`, their defaults as zero values: `NoMath`, `NoHTML` and `NoAlerts` turn formulas, HTML and alerts off, `Frontmatter` makes YAML front matter a `Custom` block "frontmatter", and `MDX` makes every element a `Custom` node named by its tag and `{expressions}` ones named "expression".
- `RepairStreaming`, which `Options.Streaming` applies before parsing, ports NSH's streaming repair.
- It closes what the last block leaves open (emphasis, strikethrough, inline code, `$` and `$$` formulas, `\(` and `\[`) and shows unfinished links as their text.
- It drops unfinished images, footnote references and HTML tags, and completes a table's delimiter row. A text ending inside a `$$` block needs nothing.
- An `Extension` makes `Custom` nodes from a regular expression in text (`Pattern`), a fenced code language (`Language`), a `:::name` container (`Container`) or an HTML element (`Tag`).
- Containers are a goldmark block parser, so their content parses as the document does. A container inside another closes with fewer colons.

## HTML

`ParseHTML` ports NSH's `html/`, with `golang.org/x/net/html` in place of html5ever: both build the tree a browser builds, so unclosed and misplaced tags read as a browser shows them. The reader walks the body in block or inline context; runs of inline content between blocks become paragraphs, and blocks met inside inline content are lines of their own. Whitespace collapses as a browser collapses it, except in `<pre>`.

- What a reader does not show is skipped: metadata, scripts, styles, form controls, embedded media, and elements `hidden`, `aria-hidden="true"` or styled `display: none`, such as KaTeX's HTML beside its MathML.
- GitHub's own markup reads back as what it writes: alert `<div>`s, task list boxes, `language-*` classes of code, and `<math>` with a TeX annotation as `Math`.
- `<details>` is `Details`; `<u>`, `<mark>`, `<sup>`, `<sub>`, `<small>` and `<kbd>` have inline nodes of their own. A `<mark>` keeps the color its `color` attribute or `background-color` asks for.
- Depth is bounded at 256 and `colspan` at 64, since the input is untrusted.

## Rendering

- `TextView` parses in one memo, then shows the blocks as rows of an `Each` keyed by position and kind. Each row keeps its block in a memo compared deeply, so a document that grows at its end rebuilds only its last block.
- A row is a `Dynamic`: the part styles and the highlighter it reads, such as the system's appearance, rebuild it.
- Paragraphs, headings, cells and code are each one `Text` with spans; links are spans with a press handler that calls `OnLinkPress`, then with `OpenLinks` opens a URL with a scheme through `Scope.OpenURL`. An image in no link reports its press with `OnImagePress`.
- Images show as blocks when a paragraph holds only images. In a line of text an image is an attachment: the size it asks for, else three quarters of a line tall.
- `RenderMath` shows formulas: blocks in place, inline ones as attachments, whose own view's `VerticalAlign` places them. Without it `Math` shows its TeX in `PartMath` or `PartMathBlock`.
- An inline custom node shows through `CustomSpan` as a span, else through `RenderCustom` as an attachment, else as its content.
- `Selectable` makes the root a selection area, so one selection runs across the blocks.
- `StreamingCaret` is placed as an attachment after the last text of the last block, into quotes, lists and custom content, after a code block's code or in a table's last cell, else on a line after the last block. The last row is built again when another follows it.
- `CodeBlockActions` are buttons in `PartCodeBlockHeader`; `CopyAction` copies the code and reports itself done for 1.6 seconds through `CodeBlockAction.Done`. `RenderTableActions` shows content above each table. `NoTableScroll`, `NoTableScrollbar` and `NoCodeBlockScrollbar` are NSH's scroll switches.
- `Provide` gives every `TextView` below a scope `Plugin`s: an extension, and what renders its nodes before `RenderCustom`, as NSH's `MarkdownPlugins` context does.
- A disclosure's summary toggles it on press; its blocks are built each time it opens.
- Tables are one CSS grid in a horizontal `Scroll`. Each column is `minmax(floor, auto)`, the floor estimated from its text as NSH does. The grid is the scroll view's width and at least the floors' sum, so narrow views wrap cells, then scroll.

## Parts

A node with several parts merges `StyleFunc`'s styles for each, then the app's `Styles` for each, later parts winning, so a typography passed as `StyleFunc` gives way to `Styles`. `Classes` go under both, each part's in the same order; an inline part's span takes the classes of the parts around it too.

Where NSGo departs from NSH:

| NSH | NSGo | Why |
| --- | --- | --- |
| `classes` and `styles` objects by part's name | `Classes` and `Styles` maps and `StyleFunc` by `Part` | A part is a constant, so a typography is one function of it |
| Typography in `nsh/base`'s `TextView` | Typography from the app | `base` has no look |
| `links="report"` or `"open"` | `OpenLinks` | Go's zero value is the default |
| Alert icon | Alert title text, and `PartAlertContent` around its blocks | The title takes the alert's color while the blocks keep the text's |
| `renderCustom` for inline nodes | `RenderCustom` as an attachment, or `CustomSpan` returning a span | A span keeps a link's press and the text's selection |
| `renderMath` with a `TextViewMath` | `RenderMath` with the `Math` and whether it is display | Go functions take arguments, not a props object |
| syntect and its feature flag | chroma in `base/textview/highlight` | Pure Go; linked only when used |

## Limits

Each span draws its own background, so nested inline content inside a mark or code (`<mark>x **y**</mark>`) shows a box per span, where NSH joins them into one. Superscripts and subscripts are smaller but sit on the baseline, since spans have no baseline offset. A disclosure's summary reports no expanded state to assistive technology. Anchors are reported, not scrolled to. Chroma's lexers cannot resume, so a growing block is lexed whole each time.
