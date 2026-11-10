---
title: Select
description: One value from a list, as a popup on desktops and a sheet on touch screens.
group: Components
order: 36
---

`ui.Select[T]` chooses one of a list of values of any comparable type. On desktops the list opens under the trigger; on touch screens it is a sheet.

```go preview=select/basic height=280
size := core.NewSignal(cx, "m")
ui.Select[string]{
	Options: []ui.Option[string]{{Value: "s", Label: "Small"}, {Value: "m", Label: "Medium"}, {Value: "l", Label: "Large"}},
	Value:   size.Get,
	OnChange: size.Set,
	Label:   "Size",
}
```

## Options and search

`Options` are `ui.Option[T]`s, each with a `Value`, a `Label`, and an optional `Icon`, `Disabled` function or `Content` of the app's own. `Placeholder` shows while no option is chosen. `Searchable` filters the list by typing, `OnClear` shows a clear button, and `Disabled` stops it.

```go preview=select/searchable height=400
ui.Select[string]{
	Options:  languages,
	Value:    language.Get,
	OnChange: language.Set,
	Searchable: true,
	SearchPlaceholder: "Search languages",
	OnClear:  func() { language.Set("") },
	Placeholder: "Choose a language",
}
```

## Behavior

Up, Down, Return and Space open the list from the focused trigger; arrows move the highlight and typeahead jumps by the items' titles. Choosing an item closes the list and reports its value. Escape, back and a press outside close it without a change. `Empty` is the text shown while search finds nothing.

## Accessibility

The trigger and list carry the select's roles; `Label` names the field, and the selected item is reported. The keyboard reaches the trigger with Tab and opens it with Return or Space.

## Platform

On macOS, Windows and Linux the list is a popup beside the trigger. On iOS and Android it is a sheet, and the selected row stays on the field. The same options and values work on every target.

## Related

- [Forms](../forms.md): selects in a form.
- [Menu](menu.md): commands rather than a value.
- [Tabs](tabs.md): a few options shown at once.
- The gallery's Select and Combobox pages.
- [Unstyled components](../authoring/base.md#select): `base.Select`, with groups and a trigger of your own.
