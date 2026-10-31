# Text

Text in NSGo: how the core prepares, lays out and reports text, how fields edit it, and how selections run across texts. Read this before changing text styles, a `TextEngine`, `Input` or selection.

## Text and drawing

A platform's `TextEngine` runs on the engine goroutine in three phases. `Prepare` shapes the text once, `PreparedText.Measure` answers layout at any width, and `PreparedText.Layout` lays it out at its final size. The core releases prepared text when it changes or its node goes.

The `TextLayout` travels in `ItemText`; the main thread draws it and releases the one it replaces. Apple's engine keeps fonts, style attributes and frame paths in bounded process-wide caches, and a prepared text keeps its measured size per width.

The core keeps what texts measure per engine, as NSH's `MeasureCache` (`measure.go`): sizes keyed by a hash of what lays a text out, colors left out, and the width it wraps at, in two generations of 1024 entries. Identical texts, such as a list's rows or a value that comes back, measure once, and the words a min-content width measures are not prepared again. A text whose hash stays, a new color, lays nothing out ([commit](core.md#commit)).

macOS draws a text or canvas view into a content layer of its own, as NSH does: a plain `CALayer` below the view's sublayers, drawn by its delegate's `drawLayer:inContext:`. No view implements `drawRect:`, whose display pass recorded and replayed every changed text, and redrew a scroll view's texts at every scroll step. A content layer draws again when its content or size changes, and only while some of it can be seen; the rest wait until a batch, a scroll or a layout animation shows them.

Canvases that paint often draw on Core Animation's threads, and their animated arguments redraw only what they change ([canvas](canvas.md#animated-arguments)).

The core does what needs no engine, as NSH's does:

- `TextTransform` changes the case of each span's characters as they are joined.
- A text's own multiplier caps the user's text size (`NoFontScaling`, `MaxFontSizeMultiplier`).
- `TextAlignVertical` moves the content box down to the text's measured height.
- `AdjustsFontSizeToFit` bisects a scale, down to `MinimumFontScale`, at which the untruncated text fits the box and its line limit, as NSH's `Paragraph::fit`. Only the final layout uses that text; layout measures the text as written.
- `Text.OnTextLayout` runs after the commit with the lines a `LineLayout` reports: UTF-8 ranges, boxes in the content box, and the metrics of the font each line starts in.

The engines draw the rest:

- `EllipsizeMode`. CoreText truncates the last line's rest at its head, middle or tail, and Pango the same way. Clipping keeps the lines whole and cuts them at the box.
- DirectWrite and the web trim only tails, and Android's `TruncateAt` a head or middle on one line only. There `AttributedText.CutRest` replaces the lines past the last kept one with the most characters around an ellipsis that fit a line (`CutToFit`).
- `AttributedText.Origin` maps the attachments a cut text kept back to the app's indices.
- `TextAlignJustify` is each engine's justification; Android's needs no newer API than NSGo's minimum.
- `TextStyle.RTL` is the base direction: CoreText's paragraph writing direction, DirectWrite's reading direction, a Pango context's base direction without auto direction, Android's text direction heuristic and CSS's `direction`. Start and end alignments follow it ([styles](styles.md)).
- `TextDecorationStyle`, `TextDecorationColor`, `TextDecorationThickness` and `TextUnderlineOffset`, the offset putting an underline's top below the baseline, both auto (the font's) unless set and their percentages of the font size. CoreText underlines solid, double, dotted and dashed lines of the font's thickness and place in a color; Apple's backend draws the other underlines and every strikethrough itself, as Windows and Android draw the lines a font or paint cannot, Linux those Pango lacks (dotted, dashed, a thickness or offset, styled strikethroughs) and the web through CSS.
- Ink past a text's box, its lines and its shadow, is drawn, as a browser draws it: Apple's content layer grows to the layout's reach (`Layout.ContentFrame`), unless the view clips.
- `FontVariant` turns on OpenType features in `Font`: small capitals and oldstyle, lining, tabular or proportional figures, a node's replacing its parent's (`FontVariantNormal` undoes one), as NSH's `fontVariant`. CoreText fonts carry them as feature settings, DirectWrite as a typography per variant, Pango as a font-features attribute, Android as a paint's feature settings and the web as `font-variant-caps` and `font-variant-numeric`; inputs and canvases take them too. A browser's canvas has no numeric variants, so the web draws tabular figures one by one, each centered in the widest figure's advance, and other figure forms as the font's default.
- `AttributedText.Shadow`, the block's shadow: a Core Graphics shadow, a CSS `text-shadow`, Direct2D's shadow effect over the recorded glyphs, a GSK shadow node, Android's shadow layer.

The family `FontMonospace` is each platform's fixed-pitch font: CoreText's user fixed-pitch font, CSS's `ui-monospace` stack, Consolas on Windows, Pango's `monospace` on Linux, Android's `monospace`.

`nsgo.Fonts` registers the app's font files, paths under `resources/`, before the first view, and texts name them by family. `internal/platform/fontfile` reads each face's family, weight and slant from its `name` and `OS/2` tables for the engines that take a face without them:

| Platform | Registers with |
| --- | --- |
| macOS, iOS | `CTFontManagerRegisterGraphicsFont`, for the process |
| Web | A `FontFace` each in `document.fonts`, loaded before the engine starts |
| Windows | A DirectWrite font collection of the system's fonts and the app's, loaded from memory, which text formats take |
| Linux | fontconfig's `FcConfigAppFontAddFile`, before GTK starts |
| Android | `Typeface.createFromAsset`, kept by family; a style takes the nearest face and synthesizes the rest |

`Text.Spans` style byte ranges over the text's style; offsets are UTF-8 bytes, Go's own, and platforms convert them. `Text.SpansFunc` gives runs of text and style again when what it reads changes, prepared again only when they differ: an editor's row recolors as its highlighting arrives. `Scope.MeasureText` measures a text in a font as a canvas does, for a column fitted to its text or a monospaced font's advance. A press on a span with a handler comes back with the byte offset under the point.

- A span's `Background` and `CornerRadius` become its `TextBackground`: drawing only, a box per line from the font's ascent to its descent, rounded on every line, behind the selection and the glyphs. Hanging spaces at a line's end are not painted.
- A span's `Attachment` is a view inside the text, as NSH's inline attachments are. Its view is a layout root at its content size, laid out before the text is prepared, so a change inside it prepares the text again.
- The text holds an `AttachmentRune` in an attachment's place, and `AttributedText.Attachments` gives the engine its size and `VerticalAlign`.
- Every engine places an attachment by NSH's rule, `TextAttachment.Ascent` and `Extent` against the placeholder font's `FontExtent`, so a line grows where an attachment reaches past its font.
- After layout the core asks the `AttachmentLayout` where each attachment went and sends its view as a native child of the text view at that frame. Those cut off by the line limit are hidden.

A `Canvas` records its drawing on the engine into an immutable `Drawing` the platform plays ([canvas](canvas.md)).

## Hit testing

A final layout answers where its carets are, as NSH's `TextEngine` hit testing does (`internal/driver/hittest.go`): `HitTest` gives the `Caret` nearest a point, `CaretRect` a line-high box of no width, and `RangeRects` the boxes a range covers, a box per run of one direction. Points are in the content box and offsets UTF-16. A point past the end of a wrapped line gives its end with `Upstream` affinity, so the caret stays on that line.

- Every engine's layout implements `driver.TextHitTester` through `driver.LazyCarets`: on first use it reads each line's box and its clusters (the characters one caret stops around, with their edges and direction) into a `CaretLayout`, and plain Go answers from then on, on either thread. Releasing the layout keeps what was read.
- The engines read clusters from their own layouts: CoreText's glyph runs, Pango's cluster iterator on a context of hit testing's own (Pango objects belong to one thread at a time), DirectWrite's `HitTestTextPosition`, the browser's character ranges in the measuring element, and Android's carets by grapheme in Java, sent over the pipe. The headless engine gives each character its approximate width.
- macOS and iOS selections hit-test and highlight through it.

## Text inputs

`Input` follows NSH's `<Input>`: one line, or several with `Multiline` in each platform's multi-line control (NSTextView, UITextView, `<textarea>`, GtkTextView, a multi-line EditText, and Windows' drawn editor wrapping its lines).

- A multiline input is measured with its value, or its placeholder while empty, wrapped at its width, so it grows unless its style sizes it. `OnContentSizeChange` is the core's: after a layout that changed the value's size, one empty line's for an empty input.
- `InputConfig.Submit` resolves `SubmitBehavior` with `Multiline` as NSH's `InputConfig::submit()`, and backends read only it: single lines blur and submit by default, multiline inputs break the line.
- Shift, Option or Alt with Return always break a multiline input's line; a Return that commits a composition never submits.
- The platform answers `MaxLength` and Return natively, without the engine.
- `internal/platform/field` keeps what every Go backend reports the same way: edit counts, selections, keys, compositions, and a `Value` held while an input method composes. Android's `InputField` does the same in Java.
- Selections are UTF-16 offsets. `SelectedRange` and `Ref.SetSelection` send `ItemSelection` after the batch's value.
- `PlaceholderColor` and `SelectionColor` are inherited text styles; the caret takes the selection color, and `HideCaret` hides it.
- `OnCaretMove` reports the caret's bounds in the input while it has the focus, and the core adds them in the surface for `Overlay.AnchorBounds`. Each platform reads them from its own editor:

  | Platform | Caret bounds |
  | --- | --- |
  | macOS | AppKit's `firstRectForCharacterRange:` |
  | iOS | UIKit's caret rect, even with `HideCaret` |
  | Android | The EditText's `Layout` after it takes the text |
  | Linux | GTK's cursor extents and locations |
  | Windows | NSGo's own editor |
  | Web | A hidden copy of the field holding its text up to the caret |

- `OnPaste` reports what the user pasted, read as they paste: the text, local files as `file:` URLs, else an image's bytes. `PasteBehavior` `PasteReport` keeps the text out; images and files never go in.
- Pastes are caught where each platform pastes. macOS gives its fields a field editor of NSGo's own (the window's delegate hands it out) and its text views their own `paste:`; iOS overrides `paste:`, Android `onTextContextMenuItem`.
- Linux hears `paste-clipboard` and reads GDK's clipboard asynchronously, so its report comes after the text it inserts, as on the web for files, whose bytes are read first.
- What a platform lacks is a no-op there. Return keys, capitalization and the keyboard's appearance belong to software keyboards: iOS, Android, the web's `enterkeyhint` and `autocapitalize`; Linux passes GTK input hints. `IOS.KeyboardAppearance` is iOS's only.
- `AutoComplete` maps to iOS's `textContentType`, Android's autofill hints, the web's `autocomplete`, GTK's input purpose and macOS's content types for accounts, passwords and codes. macOS keeps AutoFill's guesses to masked fields without `AutoComplete`: a guess at a plain field can flash an empty panel over it. Windows has no autofill or autocorrection.

## Text selection

Selection follows NSH's: one selection runs across the texts of a selection area (a `View` with `Selectable`, NSH's selectable views), in document order, as a web page's does. A `Text` with `SelectableOn` outside any area is an area of its own; one with `SelectableOff` stays out. The highlight is the inherited `SelectionColor`, else the platform's.

- `ViewProps` carries `SelectionArea` (the view's `Selectable`), `Selectable` and `SelectionColor`, so a selection never waits for the engine.
- `SelectionModel` in `selection.go` is the platform-independent model the Go backends drive on their main thread; `internal/platform/textselect` holds what Windows and Linux share of it. Android's `Selection.java` follows it on the UI thread.
- A backend lists an area's texts as `SelectionTexts`, with content, frames, hit test and NodeIDs, then feeds it presses, drags and long presses and draws its `Ranges`.
- Offsets are UTF-16. `Text` joins texts on one row with a tab and others with a line break, leaving attachments out.
- macOS takes mouse presses in an area before its views see them and shows the I-beam over its texts; fields and gestures keep theirs. A press on a button or a link presses it unless it drags 10 points first, then selects, as NSH's does. The surface becomes first responder for Copy, Select All and Escape.
- Windows and Linux select as macOS does, with drags, double clicks for words, triple clicks for a text and Shift to extend, and answer Ctrl+C, Ctrl+A and Escape; each draws the highlight under the glyphs.
- iOS selects with a long press anywhere in an area, buttons and links too, but on fields, gestures and views with a long press of their own, then by words as the finger drags, and shows the edit menu; the view controller answers Copy and Select All. Handles drawn as UIKit's move the ends.
- Android selects a word with a long press, as its text views do, with the system's handles and a floating toolbar for Copy and Select all.
- The web uses the browser's own selection through `user-select`, and a `copy` handler joins texts as the model does.
