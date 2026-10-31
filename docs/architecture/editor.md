# Editor and DiffView

`base/editor` is NSH's `Editor` (`nsh/docs/dev/editor.md`) and `base/diff` its `DiffView` (`nsh/docs/dev/diff.md`). Read this before changing either, or the highlighting they share in `base/internal/syntax`. Each is a package of its own because its highlighting links chroma's lexers.

## Editor

| File | What |
| --- | --- |
| `rope.go` | The text as lines in chunks of 512: a line is a binary search over the chunks, an edit splices the chunks it touches |
| `buffer.go` | Edits as change lists, carrying places through them, undo grouped as editors group it, UTF-16 columns |
| `state.go` | Selections (the primary one last, merged when they overlap), moves, and every editing command at every selection |
| `select.go` | Growing and shrinking selections, block drags, splitting into lines, the selection history |
| `display.go`, `fold.go` | Display rows: soft wrap at columns, folds by indentation or the app's, tabs and wide characters as columns |
| `highlight.go`, `search.go` | Highlighting jobs and their tokens by line; find and replace on `regexp`, line by line |
| `editor.go`, `view.go`, `api.go` | The controller (props, keys, the sink, presses and drags, completion, inline completion, definitions, hovers), the view's layers, the API and the ref |
| `ref.go` | The ref's language features: edits as one step, folds by nesting, offsets, boxes, scrolling, text presses |
| `guides.go` | Rulers, indent guides, whitespace marks and the inline completion's text |

- Places are a line and a byte column inside the editor; the API speaks UTF-16 columns, as language servers do. Lines end with `\n` only.
- An edit is one `buffer.apply` of every selection's edits, which returns the changes applied, the last edit first; selections, marks, folds and tokens move through them. Typing at one place and deleting merge into one undo step; a white space after a word, a move or a press starts another. 1000 steps are kept.
- `Value` replaces the text as one edit of what differs, so carets and folds away from the change stay, and the value `OnChange` just reported costs nothing.
- Without folds or soft wrap the display rows are the lines and nothing is laid out, so 200 000 lines open at once. Widths are columns of the font's `0`; past 20 000 lines only the edited lines widen the scroll width.
- The view is a vertical `Scroll` of the gutter and the text area (a horizontal `Scroll` without soft wrap). Only the rows of a `window.Rows` window are made, each a `Text` whose `SpansFunc` follows the revisions of the text, the highlighting and the marks. Selections, matches, brackets, the active line, the definition link and the carets are layers of boxes over the rows in view.
- A native `Input`, the sink, sits invisible at the primary caret, so input methods and soft keyboards work there; its changes become backspaces and typed text, its pastes `paste`. The keys are bound with `KeyAlways` while the sink has the focus and no composition runs; the completion list binds its own after them, so they win while it shows.
- Moves record the selections they leave, 100 at most, for `UndoSelection` and `RedoSelection`; an edit forgets them. `ExpandSelection` grows each selection to the smallest larger unit around it, without a syntax tree: the word, a string's contents, the string, a bracket pair's contents, the pair, the line's text, the whole lines, the text; `ShrinkSelection` retraces those steps while nothing else moved the selections.
- On desktops a press places the caret (two clicks a word, three a line, Shift extends, Alt adds a caret, Alt+Shift, or Control+Alt on Linux, drags a block of one selection per row between the press's x and the pointer's) and a responder claim on the first move drags. A drag never reveals the caret: within 16 points of the viewport's edges, or past them, a 16 ms timer scrolls 12 to 64 points a tick, faster the farther out, and extends the selection again. On touch screens a tap places it, a double tap or long press selects a word with handles a finger drags, and asks for the edit menu with `OnContextMenu`.

- Once typing pauses `InlineCompletionDelay` (one timer, whatever the keystrokes) with no completion list showing, `OnInlineCompletionRequest` asks; the answer shows in the `InlineCompletion` part from the caret, its further lines on the rows below, until Tab inserts it as one undo step or Escape, an edit, a move or the list clears it.
- `Ref.Edit` applies a language server's edits as one undo step, sorted, one overlapping an earlier one skipped; a caret goes after what is inserted at it and a selection grows over what is inserted at its ends. A fold's level for `FoldLevel` is one more than the folds around it; `FoldRange` folds lines wherever folds start.
- A `Box` is a rectangle of the rows' box twice: in the editor, its scrolls subtracted, and in the root view from the box's `Ref.Bounds`, as `CaretBounds`, `RangeBounds` and a hover's word give them. `OnTextPress` reports the character under a press after the caret moved: the one left of the nearest boundary when that boundary is right of the point, none past a row's end.
- A diagnostic carries a language server's `codeDescription` as `CodeDescription`, its URL, and its `relatedInformation` as `Related`, handed back with the hover for the app's card.
- `MinRows` and `MaxRows` set the root's height to its rows within them, with no room past the last line; otherwise `ScrollBeyondLastLine` rows (a screen by default) follow it. Indent guides merge each level's run of rows into one box; a blank line takes the lesser indentation around it.

Highlighting stays off the engine goroutine: a job lexes a snapshot of up to 2000 lines with chroma on a goroutine of its own and posts their tokens back. It starts at the last line at column 0 before the first line needing tokens, at most 200 lines back, where the lexer is most likely in its root state, and stops once three lines past the changed ones come out as they were. Jobs go first to the rows in view and 1000 lines past them. An edit moves the tokens of the lines it touches with the text, so colors do not flicker while the next job runs. The app's `Highlighter` replaces chroma in the same jobs: it gets a snapshot of every line and the range to give tokens for, which name the palette's kinds.

## DiffView

- `model.go`: lines keep their line breaks, so a missing final one is a change. The line diff is Myers' algorithm after the common prefix and suffix, bounded to 20 000 steps; within a change, deleted lines pair with the added lines most like them (a weighted longest common subsequence of similarity, at least 0.4, up to 64 lines a side; the rest in order), and a pair's changed words come from a Myers diff of its tokens, none when over 70 % changed.
- `patch.go` reads git's and `diff -u`'s unified diffs: renames, copies, new, deleted and binary files, missing final line breaks. A patch's file shows its hunks' lines only, under separators of their headers, numbered as in the file.
- `view.go` joins a patch's files into two texts, so rows, numbers and hunks count across files; each file's sides are lexed on a goroutine, by the app's `Highlighter` when it gives one. Each side is a gutter beside a horizontal `Scroll` of its text; the split sides keep each other's offset, a move under half a point ending the echo. Separators, file headers, hunk actions, annotations and the add control lie over the sides. The rows made are those in view and half a viewport each way, sliding with the view, so a scroll makes the few rows coming into it; they are made once the view has its size, so wrapping starts at its width.
- `geometry.go` keeps every row's top, a prefix sum of its lines (the taller side's wrapped lines with `SoftWrap`, columns as widths count them) and the annotations under it, so a window and a press find their rows by binary search.
- Separators and file headers are the app's with `RenderSeparator` and `RenderFileHeader`, whose controls call `Ref.Expand`, `ToggleFile`, `CollapseFile` and `ExpandFile`; a header then shows above a patch of one file too. A gap's expansion keeps the lines shown from its top and from its bottom: `ExpandDown` adds to the first, `ExpandUp` to the second.
- The change gone to (`GoToHunk`, `NextHunk`, `PreviousHunk`) is kept with the offset it scrolled to, clamped to the rows, until a scroll there and then away, or one farther than a viewport on the way: `CurrentHunk` and the next step go on from it while it lasts, as NSH's, so a change too near the end to reach the top still counts, and from the top of the view after.
- `review.go`: a gutter press selects its line on its side (unified: the number column pressed), Shift extends from the anchor, and on desktops a responder claim drags over the numbers; new texts clear the selection. Every annotation renders and reports its height from its layout; a split row's annotations stack on their side and the row takes the taller stack. The add control shows at the last selected line and at the hovered one, the hover ending 16 ms after the last view leaves so moving onto the control keeps it.

## Where NSGo departs from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| A default look from the theme's tokens | No look but the highlighting's palette: `Classes` and `Styles` give every part, a field each | `base` has no look; themes are the app's |
| `ropey` | Lines in chunks | Lines are what every part reads; chunks bound an edit's work |
| `register_language` and `register_highlighter`, kept per engine | A `Highlighter` prop: a function from a language's name to the app's highlighter | A prop needs no registry, and an engine holds no state beside its app |
| syntect's state kept every 16 lines | chroma from a line at column 0 before the change, until its tokens meet the old ones | chroma keeps no state to resume from |
| imara-diff's histogram with git's indent heuristic | Myers after the common prefix and suffix | One algorithm in Go, no dependency |
