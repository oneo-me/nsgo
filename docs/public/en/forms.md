---
title: Forms
description: Text fields, focus, the software keyboard, switches, pickers and validation.
group: Building UI
order: 18
---

## Text fields

`base.Input` is the platform's text field. With `Value` it is controlled: the app holds the value and updates it in `OnChange`; with `DefaultValue` it keeps its own. `ui.Input` and `ui.Field` wrap it in `ui`'s look.

```go
name, email := core.NewSignal(cx, ""), &core.Ref{}

ui.Input{Input: base.Input{
	Value:         name.Get,
	OnChange:      name.Set,
	Placeholder:   "Your name",
	ReturnKeyType: core.ReturnKeyNext,
	OnSubmit:      func(string) { email.Focus() },
}}
```

`Masked` hides the characters, `Readonly` and `Disabled` stop edits, and `Multiline` wraps the text and grows with it unless its style sizes it. `KeyboardType` picks the software keyboard (`core.KeyboardEmail`, `core.KeyboardNumber`, `core.KeyboardURL` and the rest), `AutoComplete` tells the system what the field holds for autofill, and `MaxLength` caps the UTF-16 units the user enters. `AutoCapitalize` and `NoAutoCorrect` tune the keyboard's behavior.

`OnSelectionChange` and `OnCaretMove` report the selection and where the caret is, for a menu that opens at it. `OnKeyPress` hears each key, `OnPaste` what the user pastes, and `OnCompositionChange` follows an input method.

## Focus and the keyboard

`Ref` focuses a field and blurs it, and `cx.DismissKeyboard()` closes the software keyboard. `OnFocus` and `OnBlur` follow the focus, and `View.OnFocus`/`OnBlur` do the same for any focusable view.

```go
email := &core.Ref{}
base.Input{Ref: email, Placeholder: "you@example.com"}
email.Focus()
```

Software keyboards cover the bottom of a phone. `core.EnvKeyboard` as a bottom padding or margin keeps content above it, and its move is animated with the keyboard:

```go
core.View{Style: core.Style{
	Padding:    core.All(16),
	PaddingEnv: core.EnvEdges{Bottom: core.EnvKeyboard},
	Gap:        12,
}, Children: form}
```

A scroll view around a form can dismiss the keyboard when dragged and decide what a tap does while the keyboard shows:

```go
core.Scroll{Behavior: core.ScrollBehavior{
	KeyboardDismissMode:       core.KeyboardDismissOnDrag,
	KeyboardShouldPersistTaps: core.KeyboardTapsHandled,
}, Children: []core.Node{form}}
```

`cx.Env().KeyboardHeight()` and `cx.Env().KeyboardDuration()` read the keyboard's size and timing in code, and `cx.Env().SafeArea()` the insets around the screen.

## Choices

`base.Switch` is a setting that takes effect at once, `base.Checkbox` is checked, unchecked or indeterminate, and `base.Radio` is one of a `base.RadioGroup`. `base.Select` opens a list of `base.SelectItem`s from a trigger on desktops and as a sheet on touch screens. The `ui` forms are `ui.Switch`, `ui.Checkbox`, `ui.Tabs` and `ui.Select[T]`.

```go
notifications := core.NewSignal(cx, true)
ui.Switch{Checked: notifications.Get, OnChange: notifications.Set, Label: "Notifications"}

size := core.NewSignal(cx, "m")
ui.Select[string]{
	Options:  []ui.Option[string]{{Value: "s", Label: "Small"}, {Value: "m", Label: "Medium"}, {Value: "l", Label: "Large"}},
	Value:    size.Get,
	OnChange: size.Set,
	Label:    "Size",
}
```

`base.NumberInput` steps and scrubs a number with bounds, `base.DatePicker` and `base.TimeField` edit dates and times, `base.ColorPicker` edits a color, `base.OtpInput` takes a one-time code, and `ui.TagInput` edits a list of tags. `ui.Input`, `ui.NumberInput`, `ui.DatePicker` and `ui.ColorPicker` are the styled forms.

## Validation

The app owns the rules: a signal holds the error, `OnBlur` or `OnSubmit` sets it, and the field's look follows it. `Style.Active`, `Style.Focus` and `Style.Disabled` color the state, and a `core.Show` adds a message.

```go
address := core.NewSignal(cx, "")
err := core.NewSignal(cx, "")
base.Input{
	Value:    address.Get,
	OnChange: address.Set,
	OnSubmit: func(string) {
		if !strings.Contains(address.Get(), "@") {
			err.Set("Enter an email address.")
		}
	},
	StyleFunc: func() core.Style {
		c := core.RGB(0xd1d5db)
		if err.Get() != "" {
			c = core.RGB(0xb4444c)
		}
		return core.Style{BorderWidth: 1, BorderColor: c}
	},
}
```

`cx.AnimateLayout(core.LayoutEaseInEaseOutPreset)` animates a message that appears, and `base.Input.OnSubmit` with `ReturnKeyNext` moves the keyboard to the next field.

## Next

- [Navigation](navigation.md): where a form lives.
- [Text](text.md): how a field's text is styled.
- [Accessibility](accessibility.md): labels and values screen readers need.
