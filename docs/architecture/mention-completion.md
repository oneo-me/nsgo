# Mention completion

This is the design for issue [#46](https://github.com/nsgo-dev/nsgo/issues/46). It is not an implemented API. The trigger and the query belong to the text field, in `core`; the list of rows is a `base` control, which owns its behavior, state and accessibility as [components](components.md) says, with its look in `ui`; a chat composer only supplies its rows.

## State and field boundary

The proposed state is a `core.Suggest` owned by the app and passed to `core.Input` (and therefore to `base.Input`, its alias). It watches the field's value and UTF-16 caret, recognizes a trigger such as `@`, and exposes whether it is open, the query and the caret's anchor, `CaretEvent.PageBounds`. `Accept(text)` replaces the trigger and query with text and `Close` closes it; the field remains the source of truth for its controlled value. It holds no rows.

Its rules are those of `base/richeditor`'s `Suggestion`, the precedent #46 names, on a plain field's text, and its options are `SuggestionOptions`' `Char` (as `Trigger`), `AllowSpaces` and `AllowedPrefixes`. The trigger is valid at the start of a line or after one of the allowed prefixes. A selection, a deleted trigger, a disallowed space or a caret that leaves the query closes the list. The query is the text between the trigger and the caret, counted in UTF-16 units so an emoji before the trigger does not shift the insertion point incorrectly.

Escape and `Close` dismiss the trigger they close, as `Suggestion`'s `dismissedFrom` does: the state keeps that trigger's offset, moved by edits before it, and stays closed on it while its match holds, however the query grows or the caret moves within it. It opens again only once the match ends or a trigger is typed elsewhere. `Accept` dismisses the same way, so an `Insert` that starts with the trigger, such as `@alice`, does not reopen the list on the text it inserted.

## The list

The proposed list is a `base.SuggestList`, given the state and the app's `Items(query)`, which shows the rows in a `core.Overlay` anchored to the caret while the state is open. It takes a provider-neutral row:

```go
type SuggestItem struct {
	ID, Label, Detail string
	Insert           string
	Disabled         bool
}
```

`Insert` replaces the trigger and query; when it is empty, `Label` is inserted. The list does not fetch a network resource, parse a chat protocol or decide which user a row represents. An app may memoize its `Items(query)` function or run it asynchronously through its own resource state.

It runs on the listbox `Select` and `Combobox` share (`base/listbox.go`), as a list beside a search field, so its rows, highlight, paging and announcements have one implementation; it shows the app's rows in the app's order and filters none. Like other `base` controls it takes the app's parts and has no look, and `ui` gives it one. An input other than Composer gets the same behavior by placing a `SuggestList` beside its field. The overlay scrolls its rows and never changes the field's layout. A row's icon and visual style belong to the caller.

## Keys and accessibility

While the list is open, Up, Down, Page Up and Page Down move the highlight, repeating as they are held, Return and Tab accept the highlighted row, and Escape closes the list. As the listbox's keys beside a search field, they are bound with `KeyAlways` inside a `Show` of the open state, so a text field's normal caret and submit behavior returns when the list closes. Acceptance reports one controlled `OnChange` and moves the caret after the inserted text in the same batch.

The rows take the option role, and the field takes the combobox role with `Expanded` following the state, as `base.Combobox`'s field does. The keyboard focus stays in the field and `core` has no active-descendant attribute, so a highlight the keyboard moves is announced with `Scope.Announce` while a screen reader runs, as the listbox does beside a search field.

`Composer` maps its existing suggestion hook to the list and places `ui`'s form of it beside its editor. It does not own token rules, rows' behavior or key capture, so another input can use the same completion behavior. The gallery's chat demo remains an example rather than a dependency of the component.

## Query timing and results

The initial proposal calls `Items` when the query changes and keeps the list open for an empty result so an app can show its own empty state. Results are ordered by the app. A result that arrives after the query has changed is discarded by a generation or query token; the list never shows rows for an older query.

The final API must choose whether `Items` is synchronous, an app-owned resource, or both. A synchronous callback must be bounded and run on the engine goroutine; a slow provider must update a signal and let the component render pending/error rows without blocking input.

## Verification

The implementation is complete only when these checks exist:

- `core` tests cover trigger prefixes, spaces, UTF-16 offsets, selection and caret changes, acceptance, a list that stays closed after Escape and after accepting an `Insert` that starts with the trigger, and owner cleanup;
- `base` tests cover the list's scoped keys, empty results, stale generations, the field's focus and the roles while the list is open, the highlight's announcement while a screen reader runs, and the field's normal keys after it closes;
- `ui` or `ui/chat` tests cover the anchored overlay, scrolling rows, disabled items and an empty state; and
- an end-to-end composer test types `@re`, chooses a row with Down and Return, verifies one draft change and confirms the rows are gone.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Should `Items` run synchronously on every query change, or should the stable API be an app-owned async resource?
2. Should an empty result keep the overlay open for an app-provided empty state, or close it?
3. Should Tab accept a row as well as Return, and should either key submit when no row is selected?
4. Is one trigger and one allowed-prefix rule enough initially, or does a field need multiple triggers at once?
5. Does `SuggestItem.Insert` replace only the trigger and query, or support an explicit edit range for richer completions?
