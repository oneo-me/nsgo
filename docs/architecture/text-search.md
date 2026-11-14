# Text-level search

This is the shared design for issue [#47](https://github.com/nsgo-dev/nsgo/issues/47). It is not an implemented API. Chat transcript search will consume this capability; the matching rules and highlights belong to `base/textview`, so another document viewer can use them without importing `ui/chat`.

## Finding in a document

The document exposes matches in the text a `TextView` renders:

```go
type TextRef struct {
	Block    []int // the block's path, outermost first
	Footnote int   // -1 when the block is not in a footnote
}

type Match struct {
	Text     TextRef
	From, To int // byte offsets in the text TextView shows
}

type FindOptions struct {
	CaseSensitive bool
	WholeWord     bool
}

func (d Document) Find(query string, opts FindOptions) []Match
```

`Find` searches paragraphs, headings, code and math TeX, including the children of quotes, lists, details and custom blocks. It excludes image alt text and hidden content. A match never crosses a text value or block, so a query containing a line break does not join two paragraphs. Regex is deliberately outside the first contract; the options can grow later without making plain search ambiguous.

The returned block path is stable for a document revision, and an empty query returns no matches. Implementations cap work and result count so an untrusted transcript cannot allocate without bound. A caller that needs a match after an edit reruns `Find` against the new document rather than applying byte offsets to stale text.

## Highlights

`TextView` gains a search state and an optional stable key. The state carries the query, matches, and selected match; a view registers its matches in render order. The renderer splits spans at match boundaries and adds `PartSearch` to every match and `PartSearchSelected` to the selected one. `base` supplies no colors: the app's `Styles` or `StyleOf` chooses them, and code-block highlighting merges with search parts.

Search highlights are not a text selection. A selectable view keeps its selection and copy behavior, while a search can mark a different range. A custom renderer receives the match metadata and may draw its own highlight; it must still report the match so a shared search can count and navigate it.

## One search across views

An app creates a search state under a scope and shares it with several `TextView`s. Each view registers a stable key and replaces only that key's matches when its document changes. The state keeps the keys' order and exposes:

```go
func NewSearch(cx *core.Scope) *Search
func (s *Search) SetQuery(string)
func (s *Search) Query() string
func (s *Search) Open() bool
func (s *Search) Close()
func (s *Search) Count() int
func (s *Search) Selected() int // one-based, zero when there is none
func (s *Search) Next()
func (s *Search) Previous()
func (s *Search) SelectedBounds() (core.Bounds, bool)
```

`SelectedBounds` comes from the selected text range's boxes, not from the block frame. A transcript owns scrolling, so it calls its own reveal operation with those bounds and keeps the selected match clear of a find bar. A view that is removed unregisters its key and cannot leave stale matches.

## Chat integration

`ui/chat` then supplies a find bar with a field, count, previous/next and close actions. `Conversation` provides one search state to the text views for its turns, opens it from Mod+F (Ctrl+F elsewhere), focuses the field, and reveals the selected bounds. Its localized words are “Find”, “Previous”, “Next”, “Close”, “No results” and “%d of %d”. It does not implement matching, span splitting or cross-view counting.

## Verification

The implementation is complete only when these checks exist:

- `base/textview` tests cover a three-block document, case and whole-word options, a match at an inline span boundary, code and math text, hidden content, stable block paths and the result cap;
- `TextView` tests cover search parts, selected parts, syntax-highlight merging, custom renderers, stale registrations and selection remaining independent;
- shared-search tests cover registration order, replacement, removal, next/previous wrapping and selected bounds across three views; and
- `ui/chat` tests type a query over three turns, show the count, navigate and reveal the selected match, then close without changing the transcript selection.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Should cross-block matches remain unsupported initially, or should a document-level plain-text view join blocks?
2. Should `ProvideSearch` automatically include every descendant `TextView`, or must each view opt in with a key?
3. Is a synchronous `Document.Find` acceptable for large transcripts, or should the API expose an asynchronous index from the start?
4. Should regular expressions and accent/Unicode-insensitive matching be later options, or belong to a separate search package?
5. Does `SelectedBounds` leave scrolling to the owning transcript, or should `TextView` reveal its own match?
