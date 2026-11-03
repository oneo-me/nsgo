package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

// fieldProps are a labelled input's: api names the property it shows,
// hint is a plain note, error turns it red, prefix goes before the text
// and next is the input return focuses.
type fieldProps struct {
	label, api, prefix string
	hint               func() string
	err                func() string
	next               *core.Ref
}

// formField is a labelled input: the border glows accent while focused
// (a ring fading in on the platform), an error turns it red, slides a
// message in (layout animation) and shakes the field.
func formField(p fieldProps, in base.Input) core.Node {
	return core.Func(func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		fin := ui.FinOf(cx)
		focus := core.NewAnimatedValue(cx, 0)
		shake := core.NewAnimatedValue(cx, 0)
		errText := p.err
		if errText == nil {
			errText = func() string { return "" }
		}
		shown := false
		cx.Effect(func() {
			has := errText() != ""
			if has == shown {
				return
			}
			shown = has
			cx.AnimateLayout(core.LayoutEaseInEaseOutPreset)
			if has {
				var steps []*core.Animation
				for _, x := range []float64{9, -8, 6, -4, 2, 0} {
					steps = append(steps, core.Timing(shake, x, core.TimingConfig{Duration: 45 * time.Millisecond}))
				}
				cx.StartAnimation(core.Sequence(steps...), nil)
			}
		})
		if p.next != nil {
			in.ReturnKeyType = core.ReturnKeyNext
			in.OnSubmit = func(string) { p.next.Focus() }
		} else if in.ReturnKeyType == 0 {
			in.ReturnKeyType = core.ReturnKeyDone
		}
		onFocus, onBlur := in.OnFocus, in.OnBlur
		in.OnFocus = func() {
			ui.Timing(cx, focus, 1, core.TimingConfig{Duration: 160 * time.Millisecond})
			if onFocus != nil {
				onFocus()
			}
		}
		in.OnBlur = func() {
			ui.Timing(cx, focus, 0, core.TimingConfig{Duration: 220 * time.Millisecond})
			if onBlur != nil {
				onBlur()
			}
		}
		fg := colors.Text
		if in.Readonly {
			fg = colors.Muted
		}
		box := core.Style{Height: core.Pt(46)}
		if in.Multiline {
			box = core.Style{MinHeight: core.Pt(46), Padding: core.Edges{Top: 13, Bottom: 13}}
		}
		own, ownFunc := in.Style, in.StyleFunc
		in.Style = core.Style{FlexGrow: 1, FlexShrink: 1, Padding: core.Symmetric(0, 14), FontSize: 15}.Merge(box)
		if in.Multiline {
			in.Style.Padding = core.Edges{Top: 13, Right: 14, Bottom: 13, Left: 14}
		}
		in.Style = in.Style.Merge(own)
		// The field's colors, under what the app's own StyleFunc gives.
		in.StyleFunc = func() core.Style {
			s := core.Style{Color: fg(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Accent()}
			if ownColor := own; ownColor.Color != 0 {
				s.Color = ownColor.Color
			}
			if ownFunc != nil {
				s = s.Merge(ownFunc())
			}
			return s
		}

		head := []core.Node{core.Text{Content: p.label, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13, FontWeight: ui.Medium}, StyleFunc: ui.Colored(colors.Muted)}}
		if p.api != "" {
			head = append(head, ui.API{Text: p.api})
		}
		if p.hint != nil {
			head = append(head, ui.Textf(p.hint, 13, ui.Regular, colors.Faint))
		}
		var well []core.Node
		if p.prefix != "" {
			well = append(well, core.Text{Content: p.prefix, Style: core.Style{Padding: core.Edges{Left: 14}, FontSize: 15}, StyleFunc: ui.Colored(colors.Muted)})
		}
		well = append(well, in, core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
			c := colors.Accent
			if errText() != "" {
				c = colors.Danger
			}
			// Around the field's border: its radius plus the 1 pt it
			// reaches out.
			return core.Style{Position: core.Absolute, Top: core.Pt(-1), Left: core.Pt(-1), Right: core.Pt(-1), Bottom: core.Pt(-1),
				CornerRadius: ui.RadiusControl + 1, BorderCurve: core.BorderCurveContinuous, BorderWidth: 2, BorderColor: c(),
				Animated: &core.AnimatedStyle{Opacity: focus}}
		}})
		align := core.AlignCenter
		if in.Multiline {
			align = core.AlignStart
		}
		readonly := in.Readonly
		return core.View{Style: core.Style{Gap: 6, Animated: &core.AnimatedStyle{TranslateX: shake}}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: head},
			core.View{StyleFunc: func() core.Style {
				border := colors.FieldBorder
				switch {
				case errText() != "":
					border = colors.Danger
				case readonly:
					border = colors.Line
				}
				s := core.Style{Direction: core.Row, AlignItems: align, CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous,
					BorderWidth: 1, BorderColor: border(), Background: colors.Field()}
				// Recessed while editable; a read-only field lies flat.
				if !readonly {
					s.Shadows = fin.Field().Shadows
				}
				return s
			}, Children: well},
			core.Show{When: func() bool { return errText() != "" }, Then: func(*core.Scope) core.Node {
				return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 7}, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pt(6), Height: core.Pt(6), CornerRadius: 3}, StyleFunc: ui.Filled(colors.Danger)},
					core.Text{ContentFunc: errText, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Danger)},
				}}
			}},
		}}
	})
}

// pair is two nodes side by side, equal widths.
func pair(a, b core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{ui.Cell{Child: a}, ui.Cell{Child: b}}}
}

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// strength is 0 to 4: length, mixed case, a digit, a symbol.
func strength(p string) int {
	if p == "" {
		return 0
	}
	var lower, upper, digit, symbol bool
	for _, r := range p {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case !unicode.IsSpace(r) && r != '_':
			symbol = true
		}
	}
	n := 0
	for _, ok := range []bool{len([]rune(p)) >= 8, lower && upper, digit, symbol} {
		if ok {
			n++
		}
	}
	return n
}

var (
	strengthTone = []ui.Tone{ui.ToneNeutral, ui.ToneDanger, ui.ToneWarn, ui.ToneInfo, ui.ToneUp}
)

// formDemo is an account form in a scroll view: the focused input stays
// above the keyboard (on the platform), return moves to the next one,
// dragging the form can hide the keyboard; leaving a field or submitting
// checks it.
func formDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	env := cx.Env()
	name, email, password := core.NewSignal(cx, ""), core.NewSignal(cx, ""), core.NewSignal(cx, "")
	code, shout, notes := core.NewSignal(cx, ""), core.NewSignal(cx, ""), core.NewSignal(cx, "")
	mode := core.NewSignal(cx, core.KeyboardDismissOnDrag)
	type status struct {
		t    ui.Tone
		text string
	}
	result := core.NewSignal(cx, status{})
	sheetOpen := core.NewSignal(cx, false)
	touched := core.NewSignalFunc(cx, map[string]bool{}, func(a, b map[string]bool) bool { return false })
	touch := func(key string) func() {
		return func() {
			touched.Update(func(t map[string]bool) map[string]bool {
				out := map[string]bool{key: true}
				for k, v := range t {
					out[k] = v
				}
				return out
			})
		}
	}
	// Inputs in return-key order.
	refs := make([]*core.Ref, 16)
	for i := range refs {
		refs[i] = &core.Ref{}
	}

	nameError := func() string {
		if touched.Get()["name"] && strings.TrimSpace(name.Get()) == "" {
			return tr.FormEnterYourName()
		}
		return ""
	}
	emailError := func() string {
		if !touched.Get()["email"] || emailPattern.MatchString(email.Get()) {
			return ""
		}
		if email.Get() != "" {
			return tr.FormThatAddressNeedsAnAnd()
		}
		return tr.FormEnterYourEMail()
	}
	passwordError := func() string {
		if touched.Get()["password"] && len([]rune(password.Get())) < 8 {
			return tr.FormUseAtLeast8Characters()
		}
		return ""
	}
	score := func() int { return strength(password.Get()) }

	submit := func() {
		touched.Set(map[string]bool{"name": true, "email": true, "password": true})
		issues := 0
		for _, e := range []string{nameError(), emailError(), passwordError()} {
			if e != "" {
				issues++
			}
		}
		cx.AnimateLayout(core.LayoutEaseInEaseOutPreset)
		switch issues {
		case 0:
			result.Set(status{ui.ToneUp, tr.FormAccountCreated(name.GetUntracked(), email.GetUntracked())})
		case 1:
			result.Set(status{ui.ToneDanger, tr.Form1FieldNeedsAttention()})
		default:
			result.Set(status{ui.ToneDanger, tr.FormFieldsNeedAttention(issues)})
		}
	}

	keyboard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		modeName := map[core.KeyboardDismissMode]string{core.KeyboardDismissNone: "none", core.KeyboardDismissOnDrag: "on-drag", core.KeyboardDismissInteractive: "interactive"}
		return ui.Card{Title: tr.FormKeyboard(), Description: "KeyboardDismissMode while dragging", Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
			h := env.KeyboardHeight()
			t := ui.ToneNeutral
			if h > 0 {
				t = ui.ToneAccent
			}
			return ui.Badge{Text: fmt.Sprintf("%d pt", int(h+0.5)), Tone: t}
		})}, Children: []core.Node{
			ui.Segmented[core.KeyboardDismissMode]{Options: []ui.Option[core.KeyboardDismissMode]{{Value: core.KeyboardDismissNone, Label: tr.FormNone()}, {Value: core.KeyboardDismissOnDrag, Label: tr.FormOnDrag()},
				{Value: core.KeyboardDismissInteractive, Label: tr.FormInteractive()}}, Value: mode.Get, OnChange: mode.Set},
		}, Footer: []core.Node{
			ui.Caption{Label: ui.Hint(cx, tr.FormDragTheFormWithThe(), tr.FormDesktopsHaveNoOnScreen()), Note: func() string { return modeName[mode.Get()] }},
		}}
	}

	account := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return ui.Section{Title: tr.FormAccount(), Note: "AutoComplete", Children: []core.Node{
			formField(fieldProps{label: tr.FormName(), next: refs[1], err: nameError},
				base.Input{Ref: refs[0], Value: name.Get, OnChange: name.Set, OnBlur: touch("name"), AutoCapitalize: core.AutoCapitalizeWords,
					AutoComplete: core.AutoCompleteName, Placeholder: "Ada Lovelace", TestID: "name"}),
			formField(fieldProps{label: tr.FormEMail(), api: "KeyboardEmail", next: refs[2], err: emailError},
				base.Input{Ref: refs[1], Value: email.Get, OnChange: email.Set, OnBlur: touch("email"), KeyboardType: core.KeyboardEmail,
					AutoCapitalize: core.AutoCapitalizeNone, NoAutoCorrect: true, AutoComplete: core.AutoCompleteEmail, Placeholder: "ada@example.com", TestID: "email"}),
			core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
				formField(fieldProps{label: tr.FormPassword(), api: "Masked", next: refs[3], err: passwordError},
					base.Input{Ref: refs[2], Value: password.Get, OnChange: func(v string) {
						if strength(v) != core.Untracked(cx, score) {
							cx.AnimateLayout(core.LayoutEaseInEaseOutPreset)
						}
						password.Set(v)
					}, OnBlur: touch("password"), Masked: true, AutoComplete: core.AutoCompleteNewPassword, Placeholder: tr.FormAtLeast8Characters(), TestID: "password"}),
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
					core.View{Style: fill, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
						s := score()
						return ui.Meter{Value: func() float64 { return float64(s) / 4 }, Tone: strengthTone[s]}
					})}},
					core.Text{ContentFunc: func() string {
						return []string{tr.FormEmpty(), tr.FormWeak(), tr.FormFair(), tr.FormGood(), tr.FormStrong()}[score()]
					}, StyleFunc: func() core.Style {
						c := colors.Faint()
						if s := score(); s > 0 {
							c = strengthTone[s].Color(cx)
						}
						return core.Style{Width: core.Pt(52), TextAlign: core.TextAlignEnd, FontSize: 13, FontWeight: ui.Medium, Color: c}
					}},
				}},
			}},
			formField(fieldProps{label: tr.FormPhone(), api: "KeyboardPhone", next: refs[4]},
				base.Input{Ref: refs[3], KeyboardType: core.KeyboardPhone, AutoComplete: core.AutoCompleteTel, Placeholder: "+1 555 0100"}),
			formField(fieldProps{label: tr.FormWebsite(), api: "KeyboardURL", next: refs[5]},
				base.Input{Ref: refs[4], KeyboardType: core.KeyboardURL, AutoCapitalize: core.AutoCapitalizeNone, NoAutoCorrect: true, Placeholder: "https://"}),
		}}
	}

	numbers := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Section{Title: tr.FormNumbers(), Note: "KeyboardType", Children: []core.Node{
			pair(
				formField(fieldProps{label: tr.FormAge(), hint: label(tr.Form3Digits()), next: refs[6]},
					base.Input{Ref: refs[5], KeyboardType: core.KeyboardNumber, MaxLength: 3, Placeholder: "36"}),
				formField(fieldProps{label: tr.FormAmount(), api: "KeyboardDecimal", prefix: "$", next: refs[7]},
					base.Input{Ref: refs[6], KeyboardType: core.KeyboardDecimal, Placeholder: "0.00"}),
			),
			formField(fieldProps{label: tr.FormOffset(), hint: label(tr.FormDarkKeyboardIos())},
				base.Input{IOS: core.IOSInputProps{KeyboardAppearance: core.KeyboardAppearanceDark}, Placeholder: "-12.5"}),
			formField(fieldProps{label: tr.FormVerificationCode(), hint: func() string { return fmt.Sprintf("%d/6", len(code.Get())) }, next: refs[8]},
				base.Input{Ref: refs[7], TestID: "code", Value: code.Get, OnChange: code.Set, KeyboardType: core.KeyboardNumber, MaxLength: 6, Placeholder: "······",
					Style: core.Style{LetterSpacing: 10, FontSize: 22, FontWeight: ui.Medium, TextAlign: core.TextAlignCenter}}),
		}}
	}

	shipping := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Section{Title: tr.FormShippingAddress(), Note: tr.FormScrollsIntoView(), Children: []core.Node{
			formField(fieldProps{label: tr.FormStreet(), next: refs[9]}, base.Input{Ref: refs[8], AutoComplete: core.AutoCompleteStreetAddress, Placeholder: "12 Analytical Way"}),
			pair(
				formField(fieldProps{label: tr.FormCity(), next: refs[10]}, base.Input{Ref: refs[9], AutoComplete: core.AutoCompleteCity, Placeholder: "London"}),
				formField(fieldProps{label: tr.FormPostalCode(), next: refs[11]}, base.Input{Ref: refs[10], AutoCapitalize: core.AutoCapitalizeCharacters,
					AutoComplete: core.AutoCompletePostalCode, Placeholder: "SW1A 1AA"}),
			),
			formField(fieldProps{label: tr.FormCountry(), next: refs[12]}, base.Input{Ref: refs[11], AutoComplete: core.AutoCompleteCountry, Placeholder: "United Kingdom"}),
		}}
	}

	selection := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Section{Title: tr.FormSelectionAndKeys(), Note: "OnSelectionChange", Children: []core.Node{selectionDemo()}}
	}

	textCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return ui.Section{Title: tr.FormText(), Note: tr.FormInputOptions(), Children: []core.Node{
			formField(fieldProps{label: tr.FormControlled(), hint: label(tr.FormUppercasedAsYouType()), next: refs[13]},
				base.Input{Ref: refs[12], TestID: "shout", Value: shout.Get, OnChange: func(t string) { shout.Set(strings.ToUpper(t)) }, Placeholder: tr.FormTypeInLowerCase()}),
			formField(fieldProps{label: tr.FormReadOnly(), api: "Readonly"}, base.Input{Value: label(tr.FormNotEditable()), Readonly: true}),
			formField(fieldProps{label: tr.FormCustomColors(), api: "PlaceholderColor, SelectionColor", next: refs[14]},
				base.Input{Ref: refs[13], Placeholder: tr.FormABluePlaceholderAnAmber(),
					StyleFunc: func() core.Style { return core.Style{PlaceholderColor: colors.Info(), SelectionColor: colors.Warn()} }}),
			core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
				formField(fieldProps{label: tr.FormNotes(), hint: func() string { return fmt.Sprintf("%d/280", len([]rune(notes.Get()))) }},
					base.Input{Ref: refs[14], Multiline: true, MaxLength: 280, Value: notes.Get, OnChange: notes.Set, Placeholder: tr.FormReturnAddsALine(),
						Style: core.Style{MinHeight: core.Pt(100)}}),
				ui.Meter{Value: func() float64 { return float64(len([]rune(notes.Get()))) / 280 }, Tone: ui.ToneAccent},
			}},
			autoGrow(),
			formField(fieldProps{label: tr.FormNoCaret(), api: "HideCaret"}, base.Input{HideCaret: true, DefaultValue: tr.FormNoCaretHere()}),
		}}
	}

	submitCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		tints := ui.TintsOf(cx)
		return ui.Card{Title: tr.FormCreateTheAccount(), Description: tr.FormSubmittingChecksEveryField(), Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
			r := result.Get()
			switch {
			case r.text == "":
				return core.Group{}
			case r.t == ui.ToneUp:
				return ui.Badge{Text: tr.FormSent(), Tone: ui.ToneUp}
			}
			return ui.Badge{Text: tr.FormCheck(), Tone: ui.ToneDanger}
		})}, Children: []core.Node{
			core.Show{When: func() bool { return result.Get().text != "" }, Then: func(*core.Scope) core.Node {
				return core.Dynamic(func(*core.Scope) core.Node {
					r := result.Get()
					fg, bg, name := colors.Danger, tints.Danger, ui.IconBell
					if r.t == ui.ToneUp {
						fg, bg, name = colors.Up, tints.Up, ui.IconCheck
					}
					return core.View{TestID: "form-status", Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.All(12),
						CornerRadius: ui.RadiusControl}, StyleFunc: ui.Filled(bg), Children: []core.Node{
						ui.Icon(name, 16, fg),
						core.Text{Content: r.text, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 14, LineHeight: 19}, StyleFunc: ui.Colored(fg)},
					}}
				})
			}},
			pair(ui.Button{Text: tr.FormSubmit(), OnPress: submit}, ui.Button{Text: tr.FormHideKeyboard(), Variant: ui.ButtonSecondary, OnPress: cx.DismissKeyboard}),
		}}
	}

	more := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Card{Title: tr.FormMoreKeyboards(), Description: tr.FormSheetsAndBarsThatRide(), Flush: true, Children: []core.Node{
			ui.LinkRow{Icon: ui.IconComponents, Title: tr.FormInputsInASheet(), Note: "Presentation: ModalPageSheet", OnPress: func() { sheetOpen.Set(true) }},
			ui.LinkRow{Divided: true, Icon: ui.IconText, Title: tr.FormAMessageBarAtThe(), Note: "PaddingEnv: EnvKeyboard", OnPress: func() { shell.push(messageBar) }},
		}}
	}

	var fields []core.Node
	for _, l := range []string{tr.FormFirst(), tr.FormSecond(), tr.FormThird(), tr.FormFourth(), tr.FormFifth(), tr.FormSixth(), tr.FormSeventh()} {
		fields = append(fields, formField(fieldProps{label: l}, base.Input{Placeholder: l + tr.FormField()}))
	}
	dialog := base.Dialog{Open: sheetOpen.Get, Presentation: core.ModalPageSheet, OnOpenChange: func(o bool, _ base.Reason) { sheetOpen.Set(o) },
		PopupStyle: core.Style{FlexGrow: 1, AlignSelf: core.AlignStretch}, Label: tr.FormASheet(),
		Children: []core.Node{core.View{Style: core.Style{PaddingEnv: core.EnvAll(core.EnvSafeArea), FlexGrow: 1}, StyleFunc: ui.Filled(colors.Background), Children: []core.Node{
			core.Scroll{Style: core.Style{FlexGrow: 1}, Behavior: core.ScrollBehavior{KeyboardDismissMode: core.KeyboardDismissInteractive}, Children: []core.Node{
				core.View{Style: core.Style{Padding: core.Edges{Top: 16, Right: 16, Bottom: 40, Left: 16}, Gap: 16}, Children: []core.Node{
					core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Edges{Top: 8, Left: 4}}, Children: []core.Node{
						core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
							core.Text{Content: tr.FormASheet(), Style: core.Style{FontSize: 24, FontWeight: ui.Bold, LetterSpacing: -0.4}, StyleFunc: ui.Colored(colors.Text)},
							ui.Text(tr.FormItsInputsStayAboveThe(), 13, ui.Regular, colors.Muted),
						}},
						ui.Button{Text: tr.FormDone(), Variant: ui.ButtonSecondary, OnPress: func() { sheetOpen.Set(false) }},
					}},
					ui.Inset{Children: fields},
				}},
			}},
		}}}}

	return demoPage(cx, pageProps{behaviorFunc: func() core.ScrollBehavior { return core.ScrollBehavior{KeyboardDismissMode: mode.Get()} }, after: []core.Node{dialog}},
		keyboard, account, numbers, shipping, selection, textCard, submitCard, more)
}

// autoGrow is a multiline input that grows with its text
// (OnContentSizeChange), up to 5 lines.
func autoGrow() core.Node {
	return core.Func(func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		height := core.NewSignal(cx, core.Points(0))
		size := core.NewSignal(cx, "–")
		return formField(fieldProps{label: tr.FormGrowsWithItsText(), hint: size.Get},
			base.Input{Multiline: true, Placeholder: tr.FormTypeSeveralLines(),
				OnContentSizeChange: func(s core.Size) {
					size.Set(fmt.Sprintf("%d×%d", int(s.Width+0.5), int(s.Height+0.5)))
					height.Set(min(s.Height, 110))
				},
				StyleFunc: func() core.Style { return core.Style{Height: core.Pt(max(height.Get(), 20) + 26)} }})
	})
}

type message struct {
	mine bool
	text string
}

// messageBar is an input pinned to the bottom, outside any scroll view:
// it rides on the keyboard.
func messageBar(cx *core.Scope) base.NavPage {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	env := cx.Env()
	messages := core.NewSignalFunc(cx, []message{
		{false, tr.FormTheBarBelowSitsOutside()},
		{false, tr.FormBottomPaddingOfEnvkeyboardLifts()},
	}, slices.Equal)
	draft := core.NewSignal(cx, "")
	send := func() {
		d := draft.GetUntracked()
		if strings.TrimSpace(d) == "" {
			return
		}
		messages.Update(func(m []message) []message { return append(slices.Clone(m), message{true, d}) })
		draft.Set("")
	}
	return screen(cx, tr.FormMessageBar(), nil, core.View{Style: core.Style{FlexGrow: 1, PaddingEnv: core.EnvEdges{Bottom: core.EnvKeyboard}}, StyleFunc: ui.Filled(colors.Background), Children: []core.Node{
		core.Scroll{Style: core.Style{FlexGrow: 1}, Behavior: core.ScrollBehavior{KeyboardDismissMode: core.KeyboardDismissInteractive}, Children: []core.Node{
			core.View{Style: core.Style{Padding: core.All(16), Gap: 8}, Children: []core.Node{
				core.Text{Content: tr.FormToday(), Style: core.Style{AlignSelf: core.AlignCenter, FontSize: 12, FontWeight: ui.Medium, Padding: core.Edges{Bottom: 8}}, StyleFunc: ui.Colored(colors.Faint)},
				core.Each(messages.Get, func(m message) message { return m }, func(cx *core.Scope, m func() message, _ func() int) core.Node {
					msg := core.Untracked(cx, m)
					return ui.Appear{Children: []core.Node{bubble(cx, msg)}}
				}),
			}},
		}},
		core.View{StyleFunc: func() core.Style {
			bottom := core.Points(8)
			// Above the home indicator, unless the keyboard covers it.
			if env.KeyboardHeight() == 0 {
				bottom += env.SafeArea().Bottom
			}
			return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Edges{Top: 8, Right: 16, Bottom: bottom, Left: 16},
				Background: colors.Surface(), Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Border()}}
		}, Children: []core.Node{
			base.Input{Value: draft.Get, OnChange: draft.Set, OnSubmit: func(string) { send() }, SubmitBehavior: core.SubmitBehaviorSubmit,
				Placeholder: tr.FormMessage(), ReturnKeyType: core.ReturnKeySend, TestID: "message",
				StyleFunc: func() core.Style {
					return fin.Inset().Over(core.Style{PlaceholderColor: colors.Faint(), SelectionColor: colors.Accent(), FlexGrow: 1, FlexShrink: 1, Height: core.Pt(44),
						Padding: core.Symmetric(0, 16), FontSize: 15, Color: colors.Text(), Background: colors.Inset(), BorderWidth: 1, BorderColor: colors.Border(), CornerRadius: 22})
				}},
			ui.RoundButton{Icon: ui.IconArrowUp, Label: tr.FormSend(), OnPress: send},
		}},
	}})
}

// bubble is a chat message: orange on the right for mine, raised on the
// left for theirs.
func bubble(cx *core.Scope, m message) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	fg := colors.Text
	if m.mine {
		fg = colors.OnAccent
	}
	return core.View{StyleFunc: func() core.Style {
		s := core.Style{AlignSelf: core.AlignStart, MaxWidth: core.Pct(80), Padding: core.Symmetric(10, 14), CornerRadii: core.Corners{TopLeft: 18, TopRight: 18, BottomRight: 18, BottomLeft: 6},
			BorderCurve: core.BorderCurveContinuous, Background: colors.Raised()}
		if m.mine {
			s = fin.Accent().Over(s)
			s.AlignSelf, s.Background, s.CornerRadii = core.AlignEnd, colors.AccentFill(), core.Corners{TopLeft: 18, TopRight: 18, BottomRight: 6, BottomLeft: 18}
		} else {
			s = fin.Raised().Over(s)
		}
		return s
	}, Children: []core.Node{core.Text{Content: m.text, Style: core.Style{FontSize: 15, LineHeight: 20}, StyleFunc: ui.Colored(fg)}}}
}

// selectionDemo is OnSelectionChange, OnKeyPress, SelectedRange and
// Ref.SetSelection.
func selectionDemo() core.Node {
	return core.Func(func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		fin := ui.FinOf(cx)
		value := core.NewSignal(cx, tr.FormSelectPartOfThisText())
		rng := core.NewSignal(cx, core.TextRange{})
		keys := core.NewSignalFunc(cx, []string(nil), slices.Equal)
		sel := core.NewSignal(cx, core.TextRange{})
		input := &core.Ref{}
		selected := func() string {
			// The range is in UTF-16 units; the demo's text is ASCII
			// until the user types otherwise.
			u := []rune(value.Get())
			r := rng.Get()
			if r.Start < 0 || r.End > len(u) || r.Start > r.End {
				return ""
			}
			return string(u[r.Start:r.End])
		}
		return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
			formField(fieldProps{label: tr.FormText(), hint: func() string { r := rng.Get(); return fmt.Sprintf("%d–%d", r.Start, r.End) }},
				base.Input{Ref: input, Value: value.Get, OnChange: value.Set, SelectedRange: sel.Get, OnSelectionChange: rng.Set, TestID: "selection",
					OnKeyPress: func(k string) {
						keys.Update(func(ks []string) []string {
							ks = append(slices.Clone(ks), k)
							return ks[max(0, len(ks)-8):]
						})
					}, AutoComplete: core.AutoCompleteOff}),
			ui.Caption{Label: tr.FormSelected(), Note: func() string { return fmt.Sprintf("%q", selected()) }},
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 6, MinHeight: core.Pt(28)}, Children: []core.Node{
				core.Dynamic(func(*core.Scope) core.Node {
					ks := keys.Get()
					if len(ks) == 0 {
						return ui.Text(tr.FormKeysYouPressShowHere(), 13, ui.Regular, colors.Faint)
					}
					var chips []core.Node
					for _, k := range ks {
						if k == " " {
							k = "␣"
						}
						chips = append(chips, core.View{StyleFunc: func() core.Style {
							return fin.Raised().Over(core.Style{MinWidth: core.Pt(28), Height: core.Pt(28), Padding: core.Symmetric(0, 8), CornerRadius: 7,
								AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border()})
						},
							Children: []core.Node{ui.Text(k, 13, ui.Regular, colors.Text)}})
					}
					return core.Group(chips)
				}),
			}},
			pair(
				ui.Button{Text: tr.FormSelectAll(), OnPress: func() { sel.Set(core.TextRange{Start: 0, End: core.UTF16Len(value.GetUntracked())}) }},
				ui.Button{Text: tr.FormCaretToStart(), Variant: ui.ButtonSecondary, OnPress: func() { input.SetSelection(0, 0) }},
			),
			formField(fieldProps{label: tr.FormOneTimeCode(), api: "AutoCompleteOneTimeCode"},
				base.Input{KeyboardType: core.KeyboardNumber, AutoComplete: core.AutoCompleteOneTimeCode, MaxLength: 6, Placeholder: "123456"}),
		}}
	})
}
