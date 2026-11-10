---
title: Select
description: One value from a list, as a popup on desktops and a sheet on touch screens.
group: Components
order: 65
---

`ui.Select[T]` chooses one of a list of values; `base.Select` is the same control without a look, with strings as its values. On desktops the list opens under the trigger; on touch screens it is a sheet.

```go
size := core.NewSignal(cx, "m")
ui.Select[string]{
	Options: []ui.Option[string]{{Value: "s", Label: "Small"}, {Value: "m", Label: "Medium"}, {Value: "l", Label: "Large"}},
	Value:   size.Get,
	OnChange: size.Set,
	Label:   "Size",
}
```

## The base select

`base.Select` takes `base.SelectItem`s and a `*string` value, so the app can hold “none” as `nil`. The app styles the trigger, the popup, its rows and its groups.

```go
choice := core.NewSignal[*string](cx, nil)
base.Select{
	Items: func() []base.SelectItem {
		return []base.SelectItem{{Value: "s", Title: "Small"}, {Value: "m", Title: "Medium"}}
	},
	Value: choice.Get,
	OnChange: choice.Set,
	Label: "Size",
	Style: core.Style{Height: core.Pt(28), Padding: core.Symmetric(4, 10), BorderWidth: 1, CornerRadius: 6},
	PopupStyle: core.Style{Padding: core.All(4), CornerRadius: 8},
	ItemStyle: core.Style{Padding: core.Symmetric(4, 8), CornerRadius: 4},
}
```

## The styled select

`ui.Select[T]` takes `Options` of `ui.Option[T]`, a `Value`, an `OnChange`, a `Placeholder` and a `Label`. `Searchable` filters the list by typing, `OnClear` shows a clear button, and `Disabled` stops it.

```go
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

`base.Select` also groups adjacent items with the same `Group` under one header, supports typeahead by title, and lets the app draw a search field with `SearchField`.

## Behavior

Up, Down, Return and Space open the list from the focused trigger; arrows move the highlight and typeahead jumps by the items' titles. Choosing an item closes the list and reports its value. Escape, back and a press outside cancel, which `OnOpenChange` and `OnCancel` hear.

`Cleanable` shows the clear button on the base form. `Empty` is the content shown while search finds nothing.

## Accessibility

The trigger and list carry the select's roles; `Label` names the field, and the selected item is reported. Give the app's own trigger a visible name if it shows no text. The keyboard reaches the trigger with Tab and opens it with Return or Space.

## Platform

On macOS, Windows and Linux the list is a popup beside the trigger. On iOS and Android it is a sheet, and the selected row stays on the field. The same options and values work on every target.

## Related

- [Forms](../forms.md): selects in a form.
- [Menu](menu.md): commands rather than a value.
- [Tabs](tabs.md): a few options shown at once.
- The gallery's Select and Combobox pages.
