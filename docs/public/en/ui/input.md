---
title: Input
description: The platform's text field, controlled or not, with focus and the software keyboard.
group: Components
order: 62
---

`base.Input` is the platform's text field, one line unless `Multiline`. `ui.Input` is the same field with Onevra's look, and `ui.Field` puts it in a labelled field's rim.

```go
name := core.NewSignal(cx, "")
ui.Input{Input: base.Input{
	Value:       name.Get,
	OnChange:    name.Set,
	Placeholder: "Your name",
	TestID:      "name",
}}
```

## Controlled and uncontrolled

With `Value`, the field is controlled: the app holds the value and updates it in `OnChange`. With `DefaultValue`, the field keeps its own. A controlled field can be cleared by setting its value to `""`.

```go
draft := core.NewSignal(cx, "")
base.Input{Value: draft.Get, OnChange: draft.Set, Placeholder: "New task",
	OnSubmit: func(string) { add(); draft.Set("") }}
```

## Fields

`Multiline` wraps and grows with its text unless its style sizes it. `Masked` hides the characters, `Readonly` and `Disabled` stop edits, and `MaxLength` caps the UTF-16 units the user enters. `Placeholder` can be a function with `PlaceholderFunc`.

```go
base.Input{Masked: true, Value: password.Get, OnChange: password.Set,
	KeyboardType: core.KeyboardDefault, AutoComplete: core.AutoCompletePassword}
```

`KeyboardType` picks a software keyboard (`KeyboardEmail`, `KeyboardNumber`, `KeyboardURL` and the rest), `ReturnKeyType` labels its return key, and `SubmitBehavior` says whether Return submits or breaks a multiline line. `AutoCapitalize`, `NoAutoCorrect` and `AutoComplete` tune the system's help.

## Focus and events

`Ref` focuses and blurs the field. `OnFocus` and `OnBlur` follow the focus, `OnSelectionChange` and `OnCaretMove` the selection and the caret, `OnKeyPress` each typed key, `OnPaste` what is pasted and `OnCompositionChange` an input method composing.

```go
email := &core.Ref{}
base.Input{Ref: email, Placeholder: "you@example.com"}
email.Focus()
```

The software keyboard covers the bottom of a phone. Put `core.EnvKeyboard` on the bottom padding or margin of the content so it stays above the keyboard; the move animates with it. `cx.DismissKeyboard()` closes the keyboard, and `cx.Env().KeyboardHeight()` reads its height.

## Validation

The app owns the rules. A signal holds the error, `OnBlur` or `OnSubmit` sets it, and a `StyleFunc` colors the field from it. The error text is a `core.Text` with `LiveRegionPolite` if it should be announced.

```go
err := core.NewSignal(cx, "")
base.Input{Value: email.Get, OnChange: email.Set,
	OnSubmit: func(string) { if !strings.Contains(email.Get(), "@") { err.Set("Enter an email address.") } },
	StyleFunc: func() core.Style {
		if err.Get() != "" { return core.Style{BorderWidth: 1, BorderColor: core.RGB(0xb4444c)} }
		return core.Style{}
	}}
```

## Accessibility

The field reports `RoleTextField` and reads its `Label`; `Placeholder` is not a label, so give a field with no visible label a `Label`. `Readonly` and `Disabled` are reported, and `OnSubmit` is the keyboard's action.

## Platform

iOS, Android, Windows and Linux use their native fields; the web uses an `<input>` or `<textarea>`. The software keyboard, autofill, input methods and text direction are the platform's. Focus and typing are tested the same way on every target.

## Related

- [Forms](../forms.md): fields, validation, choices and the keyboard.
- [Accessibility](../accessibility.md): labels and focus.
- [Layout](../layout.md): keeping content above the keyboard.
- The gallery's Input and Form pages.
