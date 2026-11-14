# Mention completion

This is the design for issue [#46](https://github.com/nsgo-dev/nsgo/issues/46). It is not an implemented API. Mention completion belongs to the text field's behavior, while `base` and `ui` own the list's unstyled and styled forms; a chat composer only supplies its query and rows.

## State and field boundary

The proposed state is a `core.Suggest` owned by the app and passed to `core.Input` (and therefore to `base.Input`). It watches the field's value and UTF-16 caret, recognizes a trigger such as `@`, and exposes the query, rows, selected index and caret anchor. `Open`, `Accept` and `Close` change the state; the field remains the source of truth for its controlled value.

The state takes a provider-neutral row:

```go
type SuggestItem struct {
	ID, Label, Detail string
	Insert           string
	Disabled         bool
}
```

`Insert` replaces the trigger and query; when it is empty, `Label` is inserted. The state does not fetch a network resource, parse a chat protocol or decide which user a row represents. An app may memoize its `Items(query)` function or run it asynchronously through its own resource state.

The trigger is valid at the start of a line or after one of the allowed prefixes. A selection, a deleted trigger, a disallowed space or a caret that leaves the query closes the list. The query is the text between the trigger and the caret, counted in UTF-16 units so an emoji before the trigger does not shift the insertion point incorrectly.

## Keys and accessibility

While the suggestion list is open, a scoped key node consumes Up, Down, Page Up, Page Down, Return, Tab and Escape. It must be installed only while open, with `KeyAlways`, so a text field's normal caret and submit behavior returns when the list closes. Acceptance reports one controlled `OnChange` and moves the caret after the inserted text in the same batch.

The list is a `core.Overlay` anchored to the caret's page bounds. Its rows use option roles, the field reports an expanded state and the selected row is exposed to assistive technology. The overlay scrolls its rows and never changes the field's layout. A row's icon and visual style belong to the caller; `base` supplies behavior and `ui` supplies a look.

`Composer` maps its existing suggestion hook to the state and draws the rows. It does not own token rules or key capture, so another input can use the same completion behavior. The gallery's chat demo remains an example rather than a dependency of the component.

## Query timing and results

The initial proposal calls `Items` when the query changes and keeps the list open for an empty result so an app can show its own empty state. Results are ordered by the app. A result that arrives after the query has changed is discarded by a generation or query token; the field never shows rows for an older query.

The final API must choose whether `Items` is synchronous, an app-owned resource, or both. A synchronous callback must be bounded and run on the engine goroutine; a slow provider must update a signal and let the component render pending/error rows without blocking input.

## Verification

The implementation is complete only when these checks exist:

- `core` tests cover trigger prefixes, spaces, UTF-16 offsets, selection and caret changes, empty results, stale generations, acceptance and owner cleanup;
- `base` tests cover scoped keys, focus and accessibility roles while the list is open and the field's normal keys after it closes;
- `ui` or `ui/chat` tests cover the anchored overlay, scrolling rows, disabled items and an empty state; and
- an end-to-end composer test types `@re`, chooses a row with Down and Return, verifies one draft change and confirms the rows are gone.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Should `Items` run synchronously on every query change, or should the stable API be an app-owned async resource?
2. Should an empty result keep the overlay open for an app-provided empty state, or close it?
3. Should Tab accept a row as well as Return, and should either key submit when no row is selected?
4. Is one trigger and one allowed-prefix rule enough initially, or does a field need multiple triggers at once?
5. Does `SuggestItem.Insert` replace only the trigger and query, or support an explicit edit range for richer completions?
