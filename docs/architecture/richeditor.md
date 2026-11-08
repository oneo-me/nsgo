# Rich editor

`base/richeditor` is NSH's `RichEditor` (`nsh/docs/dev/rich-editor.md`): a kernel after ProseMirror's, an extension layer after Tiptap's, and a native view. Read this before changing any of them. `ui/richeditor` is the view in `ui`'s colors, which apps show. It is a package of its own, as `base/diff` is, because its formats link goldmark and an HTML parser.

## Files

| File | What |
| --- | --- |
| `schema.go`, `content.go` | `Schema`, node and mark types, content expressions compiled to `ContentMatch` automata |
| `node.go`, `resolve.go` | `Node`, `Fragment`, `Slice`, `ResolvedPos`, `Node.Replace` after ProseMirror's `replace.ts` |
| `step.go`, `transform.go`, `fit.go` | Steps, `StepMap`, `Mapping`, `Transform` and its structure helpers, the fitter behind `ReplaceStepFit` |
| `selection.go`, `state.go`, `transaction_steps.go` | Text, node and all selections with bookmarks, `Transaction`, `Plugin`, `State` |
| `commands.go` | prosemirror-commands and prosemirror-schema-list |
| `history.go`, `keymap.go`, `inputrules.go` | Undo grouped by time and adjacency, keymaps, typed rules |
| `tablemap.go`, `tablecommands.go`, `tablepaste.go` | prosemirror-tables: `TableMap`, `CellSelection`, `FixTables`, `TableEditing` and the table commands, with moving rows and columns, setting sizes and pasting cells |
| `gapcursor.go` | prosemirror-gapcursor: `GapCursor`, `FindGapCursorFrom` and its plugin |
| `kit.go`, `link.go` | `Extension`, `Build`, `StarterKit`, `TableKit`, `Link`, `CharacterCount`, `Suggestion`, `Mention` |
| `format.go`, `markdown.go`, `html.go` | JSON, plain text, Markdown and HTML |
| `session.go` | `Session`: the one live editor a view and the app share |
| `view.go`, `geometry.go`, `input.go`, `tableview.go`, `blockdrag.go` | The view: blocks and spans, positions to points and back, typing, keys and presses, tables' grids and handles, blocks a handle drags |

## The kernel

Positions count as ProseMirror's do: a text its UTF-16 units, a leaf one, a node with content its two boundaries, stored as UTF-8 and converted by `byteAt`. A document's types are the schema's and the kernel knows none by name, a table's role being its spec's `Extra["tableRole"]`.

A `Transaction` is a `Transform`, and `State.ApplyTransaction` applies it and what plugins append atomically: filters may refuse it, the new document must pass `Node.Check`, and 64 rounds of appending end in a `StateError`. Asynchronous work builds a new transaction against the state current then.

A session belongs to the engine goroutine, as the reactive graph does, so it takes no lock; other goroutines reach it through `Scope.Post`.

## Extensions

`Build` resolves a list of `Extension`s as Tiptap does: the schema in priority order, the runtime with the list reversed then by priority, so an app's extension after the kit binds before it, under one keymap merging every extension's bindings. Commands are named (`toggleBold`, `insertTable`) with `Attrs` arguments, or plain `Command`s; `Chain` runs several on the states each leaves as one transaction.

Pasted cells follow prosemirror-tables: over a cell selection they repeat or are cut to fill it, other content filling each cell, and in a cell they land with their top left there, the table growing and the cells crossing the area's edges split (`PastedCells`, `ClipCells`, `InsertCells`). The gap cursor stands between closed blocks whose parent takes a textblock there, as before a table that starts the document; arrows and presses reach it, and typing, or an input method starting to compose (`Session.OpenGap`), fills it with that textblock.

## The view

- Blocks are views keyed by id (`block`): a block keeps its id while it is the same node, else the node at its index with its type, so a change rebuilds only what changed. A table's cells are its children, since rows have no view, placed in a grid at their map's slots.
- A textblock is one `Text` of spans, split at its marks and decorations. `textRun`s map document offsets to text offsets: a mention's `@name` is one position however long, and a composition and a widget decoration's view, a span's attachment, take none. A widget with `Side` below zero shows before a cursor at its place, the others and the composition after it.
- Geometry reads laid-out text through `Ref.TextCaretAt`, `TextCaretBox` and `TextRangeBoxes`, which every backend's `TextHitTester` answers, so carets and hit tests follow the app's fonts. A layout's report bumps `geo`, which redraws the selection and the caret.
- Input is the code `Editor`'s sink: an invisible `Input` at the caret. While an input method composes, its text shows in the line and the document waits; the commit is one transaction tagged with the composition's id, which the history groups.
- Keys go to the session's plugins by ProseMirror's names, `Mod` left for the view to resolve through `core`'s `mod`, then to caret movement. Moves keep a goal x by geometry.
- A drag on a table's band previews its size in the grid's tracks and commits once on release, and a grip's drag moves its line with `MoveTableColumn` or `MoveTableRow`, one undo step each.
- `BubbleMenu` is an `Overlay` anchored to the selection's box, shown while the app returns content.
- A handle the app places carries blocks through the session: `DragBlock` selects the block at a position, a defining parent standing for its first child (a list item for its paragraph), or takes the siblings a selection covers; `DragTo` moves a drop cursor before or after the innermost block at the pointer's height whose parent takes them, else beside the nearest, and `EndDrag` moves them with `MoveBlocks`, one undo step. The three take surface points, as `PageCoordsAtPos` and `PosAtPageCoords` answer. A selection dragged with Mod held across blocks covers them whole on release.
- The gap cursor is a line on the block after the gap, or under the one before; a press above, between or below the blocks of a container offers the plugins the gap's position.

## TSX

`tsx/richeditor` gives TSX sessions in `nsgo/base`, and `tsx/ui/richeditor` `nsgo/ui`'s styled `RichEditor`:

- `createRichEditor({ extensions, markdown | html | json, plugins, editable, openLink })` makes a `RichEditorSession`, of `StarterKit` and `TableKit` when it lists none. Its methods run and check commands, read the document in each format, the selection, attributes and suggestions, answer the geometry queries, drive a handle's drag and `on(event, f)` hears `revision`, `update`, `selectionUpdate`, `transaction`, `focus`, `blur` and `destroy`. `<RichEditor editor={editor} />` shows it.
- Commands are `RichCommand`s, data resolved in the editor that runs them: `command(name, args)` names one of its extensions', and `chain`, `first`, `toggleMark`, `setBlockType`, `wrapIn`, `insertText`, `insertContent`, `setTextSelection`, `setNodeSelection`, `moveBlocks`, `selectAll`, `undo` and others are the kernel's. A name or type the editor lacks throws a `TypeError`, a position outside the document a `RangeError`.
- `StarterKit`, `TableKit`, `Mention` and `CharacterCount` make the built-in extensions from Tiptap's options, `false` leaving a part out, and `configure` merges options deeply. `defineExtension` declares the app's: a node or mark named after it, and `addOptions`, `addGlobalAttributes`, `addKeyboardShortcuts`, `addCommands`, `addInputRules` and `addProseMirrorPlugins`, functions of its options run once as an editor is made. Bindings to names or types the schema lacks make `createRichEditor` throw.
- A plugin is an object of the app's hooks, called with the editor: `keys`, `handleKeyDown`, `handleTextInput`, `handleClick`, `handlePaste`, `decorations` (widgets are JSX), `filterTransaction`, and `appendTransaction`, whose command runs on the state the transactions left.

## Where NSGo departs from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| A default look from the theme's tokens | None in `base`: `Classes` and `Styles` give every part, and `ui/richeditor` the look | `base` has no look |
| `Arc<Mutex<…>>` sessions used from any thread | A session on the engine goroutine | The graph has the same rule; a lock would guard nothing else |
| Transforms return errors at each step | `Transform.Err` holds the first, after which it takes no more | Commands chain steps, and a failed transaction refuses to apply whole |
| Paste rules | Pasted text with Markdown's syntax reads as Markdown | One parser for the same marks |
| Keymaps resolve `Mod` | The view does, through `core`'s `mod` | The kernel stays free of the platform, and the web's platform is the page's |
| `ProseMirror` plugins after the keymap | Extensions' plugins before it | A suggestion's list and a cell selection take Enter and Backspace before they edit |
| JS hooks called only in the app's own calls, input posted to JS | Hooks called in place, on the engine goroutine | goja runs there anyway; posting would split one key's handling across turns |
| `StarterKit.configure(...)` on classes | `StarterKit(options)` and `configure` on what it returns | Go registers functions and classes, not classes standing for values |
