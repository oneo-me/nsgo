//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/field"
)

// Text inputs follow NSH's: a UITextField for one line, or a UITextView
// when multiline, at the content box inside the input's view, which
// draws the decoration and is the editor's delegate. A text view has no
// placeholder, so a label under it shows one while the text is empty.
// The delegate answers the editor's questions natively (maxLength, what
// Return does) and field.State turns what it hears into NSH's events,
// holding a value the app sets while an input method composes (the
// editor's marked text) until the composition ends.

type input struct {
	fs        field.State
	field     objc.ID // the UITextField or UITextView
	label     objc.ID // a text view's placeholder
	bar       objc.ID // the InputAccessoryView's view
	multiline bool
	focused   bool
	// applying is a change made here, clipping an insertion cut to
	// maxLength, lineBreak a line break typed with a modifier.
	applying, clipping, lineBreak bool
}

var classField, classTextView objc.Class

// inputMethods are the delegate methods of an input's view.
func inputMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{Cmd: sel("layoutSubviews"), Fn: inputLayout},
		{Cmd: selChanged, Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { inputAt(self).changed() }},
		{Cmd: sel("textFieldShouldReturn:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) bool {
			in := inputAt(self)
			if in == nil {
				return false
			}
			if _, blur := in.fs.Return(in.text(), false); blur {
				in.field.Send(selResign)
			}
			return false
		}},
		{Cmd: sel("textFieldShouldBeginEditing:"), Fn: inputShouldBegin},
		{Cmd: sel("textViewShouldBeginEditing:"), Fn: inputShouldBegin},
		{Cmd: sel("textFieldDidBeginEditing:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { inputAt(self).focusChanged(true) }},
		{Cmd: sel("textViewDidBeginEditing:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { inputAt(self).focusChanged(true) }},
		{Cmd: sel("textFieldDidEndEditing:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { inputAt(self).focusChanged(false) }},
		{Cmd: sel("textViewDidEndEditing:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { inputAt(self).focusChanged(false) }},
		{Cmd: sel("textFieldDidChangeSelection:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { inputAt(self).selectionMoved() }},
		{Cmd: sel("textViewDidChangeSelection:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { inputAt(self).selectionMoved() }},
		{Cmd: sel("textViewDidChange:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) { inputAt(self).changed() }},
		{Cmd: sel("textField:shouldChangeCharactersInRange:replacementString:"), Fn: inputShouldChange},
		{Cmd: sel("textView:shouldChangeTextInRange:replacementText:"), Fn: inputShouldChange},
	}
}

// registerInput registers the editor classes.
func registerInput() {
	caretRect := objc.MethodDef{Cmd: sel("caretRectForPosition:"), Fn: func(self objc.ID, cmd objc.SEL, pos objc.ID) apple.CGRect {
		if in := inputOf(self); in != nil && in.fs.Config.HideCaret {
			return apple.CGRect{}
		}
		return apple.SendSuper[apple.CGRect](self, cmd, pos)
	}}
	deleteBackward := objc.MethodDef{Cmd: sel("deleteBackward"), Fn: func(self objc.ID, cmd objc.SEL) {
		in := inputOf(self)
		empty := in != nil && in.text() == ""
		apple.Super(self, cmd)
		// Deleting in an empty field changes nothing, so no delegate call
		// reports it.
		if empty {
			in.fs.Key("Backspace")
		}
	}}
	classField = register("NsgoField", objc.GetClass("UITextField"), nil,
		append([]objc.MethodDef{caretRect, deleteBackward}, pasteMethods()...))
	classTextView = register("NsgoTextView", objc.GetClass("UITextView"), nil, append(pasteMethods(), caretRect, deleteBackward,
		// Shift and Option with Return break the line of a text view whose
		// Return submits, as chat apps do on hardware keyboards.
		objc.MethodDef{Cmd: sel("keyCommands"), Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
			inherited := apple.SendSuper[objc.ID](self, cmd)
			in := inputOf(self)
			if in == nil || in.fs.Config.Submit() == driver.SubmitBehaviorNewline {
				return inherited
			}
			commands := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
			if inherited != 0 {
				commands.Send(sel("addObjectsFromArray:"), inherited)
			}
			for _, mods := range []int{1 << 17, 1 << 19} { // Shift, Option
				c := objc.ID(objc.GetClass("UIKeyCommand")).Send(sel("keyCommandWithInput:modifierFlags:action:"),
					apple.NSString("\r"), mods, sel("nsgoInsertLineBreak:"))
				c.Send(sel("setWantsPriorityOverSystemBehavior:"), true)
				commands.Send(sel("addObject:"), c)
			}
			return commands
		}},
		objc.MethodDef{Cmd: sel("nsgoInsertLineBreak:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
			if in := inputOf(self); in != nil {
				in.lineBreak = true
				self.Send(sel("insertText:"), apple.NSString("\n"))
				in.lineBreak = false
			}
		}},
	))
}

// inputOf returns the input an editor belongs to; inputAt the one of an
// input's view.
func inputOf(editor objc.ID) *input {
	if v := views[editor]; v != nil {
		return v.input
	}
	return nil
}

func inputAt(native objc.ID) *input { return inputOf(native) }

func (v *view) newInput() {
	v.native = objc.ID(classInput).Send(selNew)
	v.input = &input{fs: field.State{ID: v.id, Dispatch: v.surface.engine.Dispatch}}
	v.setEditor(false)
}

// setEditor replaces an input's editor, keeping its text.
func (v *view) setEditor(multiline bool) {
	in := v.input
	text := ""
	if in.field != 0 {
		text = in.text()
		in.field.Send(selRemoveFromSuperview)
		delete(views, in.field)
		if in.label != 0 {
			in.label.Send(selRemoveFromSuperview)
			in.label = 0
		}
	}
	if multiline {
		tv := objc.ID(classTextView).Send(selNew)
		tv.Send(sel("setBackgroundColor:"), objc.ID(objc.GetClass("UIColor")).Send(sel("clearColor")))
		tv.Send(sel("setTextContainerInset:"), edgeInsets{})
		tv.Send(sel("textContainer")).Send(sel("setLineFragmentPadding:"), 0.0)
		tv.Send(selSetDeleg, v.native)
		label := objc.ID(objc.GetClass("UILabel")).Send(selNew)
		label.Send(sel("setNumberOfLines:"), 0)
		label.Send(selSetHidden, true)
		v.native.Send(selAddSubview, label)
		label.Send(selRelease)
		in.field, in.label = tv, label
	} else {
		f := objc.ID(classField).Send(selNew)
		f.Send(selSetDeleg, v.native)
		const editingChanged = 1 << 17
		f.Send(sel("addTarget:action:forControlEvents:"), v.native, selChanged, uint(editingChanged))
		in.field = f
	}
	in.multiline = multiline
	v.native.Send(selAddSubview, in.field)
	in.field.Send(selRelease)
	views[in.field] = v
	if in.bar != 0 {
		in.field.Send(sel("setInputAccessoryView:"), in.bar)
	}
	if text != "" {
		in.replaceText(text)
	}
}

func (in *input) text() string { return apple.GoString(in.field.Send(selText)) }

// markedText is what an input method composes in the editor, if it does.
func (in *input) markedText() (string, bool) {
	r := in.field.Send(sel("markedTextRange"))
	if r == 0 {
		return "", false
	}
	t := apple.GoString(in.field.Send(sel("textInRange:"), r))
	return t, t != ""
}

// selection is the editor's selection in UTF-16 units.
func (in *input) selection() (driver.TextRange, bool) {
	f := in.field
	r := f.Send(sel("selectedTextRange"))
	if r == 0 {
		return driver.TextRange{}, false
	}
	begin := f.Send(sel("beginningOfDocument"))
	offset := func(p objc.ID) int {
		return max(objc.Send[int](f, sel("offsetFromPosition:toPosition:"), begin, p), 0)
	}
	return driver.TextRange{Start: offset(r.Send(sel("start"))), End: offset(r.Send(sel("end")))}, true
}

// selectRange selects r, clamped to the text.
func (in *input) selectRange(r driver.TextRange) {
	f := in.field
	begin := f.Send(sel("beginningOfDocument"))
	n := objc.Send[int](f, sel("offsetFromPosition:toPosition:"), begin, f.Send(sel("endOfDocument")))
	start := min(max(r.Start, 0), n)
	end := min(max(r.End, start), n)
	from := f.Send(sel("positionFromPosition:offset:"), begin, start)
	to := f.Send(sel("positionFromPosition:offset:"), begin, end)
	if from == 0 || to == 0 {
		return
	}
	f.Send(sel("setSelectedTextRange:"), f.Send(sel("textRangeFromPosition:toPosition:"), from, to))
}

// replaceText sets the text without reporting it, keeping the caret
// where it was, at the end if it was there.
func (in *input) replaceText(text string) {
	old := driver.UTF16Len(in.text())
	caret, ok := in.selection()
	in.applying = true
	in.field.Send(selSetText, apple.NSString(text))
	if ok && in.focused {
		n := driver.UTF16Len(text)
		at := min(caret.End, n)
		if caret.End >= old {
			at = n
		}
		in.selectRange(driver.TextRange{Start: at, End: at})
	}
	in.applying = false
	in.updatePlaceholder()
}

func (in *input) updatePlaceholder() {
	if in.label != 0 {
		in.label.Send(selSetHidden, in.fs.Config.Placeholder == "" || in.text() != "")
	}
}

// changed reports the user's edit, a composition starting or changing
// before the text holding it and one ending after the text it committed.
func (in *input) changed() {
	if in == nil {
		return
	}
	in.updatePlaceholder()
	if in.applying {
		return
	}
	marked, composing := in.markedText()
	if composing {
		in.compose(true, marked)
	}
	in.fs.Edited(in.text())
	if !composing {
		in.compose(false, "")
	}
	in.selectionMoved()
}

func (in *input) compose(composing bool, marked string) {
	if value, apply := in.fs.Compose(composing, marked, in.text()); apply {
		in.replaceText(value)
	}
}

// selectionMoved reports the user's selection, and a composition the
// input method ended by unmarking, which leaves the text as it was.
func (in *input) selectionMoved() {
	if in == nil || in.applying || !in.focused {
		return
	}
	if in.text() == in.fs.Known() {
		marked, composing := in.markedText()
		in.compose(composing, marked)
	}
	if r, ok := in.selection(); ok && in.fs.Events.Has(driver.EventSelectionChange) {
		in.fs.Selected(r)
	}
	in.caretMoved()
}

func (in *input) focusChanged(focused bool) {
	if in == nil || in.focused == focused {
		return
	}
	in.focused = focused
	v := views[in.field]
	if focused {
		v.dispatch(driver.EventFocus)
		in.selectionMoved()
		v.surface.keepFocusedClear()
	} else {
		in.compose(false, "")
		v.dispatch(driver.EventBlur)
	}
}

// shouldChange answers whether the editor may replace r with repl: Return
// in a text view does what SubmitBehavior says, and an insertion is cut
// to maxLength, the cut edit made here.
func (in *input) shouldChange(r apple.NSRange, repl string) bool {
	if in.applying || in.clipping {
		return true
	}
	if in.multiline && repl == "\n" && !in.lineBreak {
		if _, composing := in.markedText(); !composing {
			insert, blur := in.fs.Return(in.text(), false)
			if !insert {
				if blur {
					in.field.Send(selResign)
				}
				return false
			}
		}
	}
	kept, ok, whole := in.fs.Insert(in.text(), int(r.Length), repl)
	if whole {
		return true
	}
	if ok && kept != "" {
		f := in.field
		begin := f.Send(sel("beginningOfDocument"))
		from := f.Send(sel("positionFromPosition:offset:"), begin, int(r.Location))
		to := f.Send(sel("positionFromPosition:offset:"), begin, int(r.Location+r.Length))
		if from != 0 && to != 0 {
			in.clipping = true
			f.Send(sel("replaceRange:withText:"), f.Send(sel("textRangeFromPosition:toPosition:"), from, to), apple.NSString(kept))
			in.clipping = false
			in.changed()
		}
	}
	return false
}

func inputShouldChange(self objc.ID, _ objc.SEL, _ objc.ID, r apple.NSRange, repl objc.ID) bool {
	in := inputAt(self)
	return in == nil || in.shouldChange(r, apple.GoString(repl))
}

func inputShouldBegin(self objc.ID, _ objc.SEL, _ objc.ID) bool {
	in := inputAt(self)
	return in == nil || !in.fs.Config.Readonly
}

// value is ItemInputValue.
func (in *input) value(text string, count uint32) {
	if in.fs.Value(text, count) && in.text() != text {
		in.replaceText(text)
	}
}

// setSelection is ItemSelection.
func (in *input) setSelection(r driver.TextRange) {
	in.fs.SetSelection(r)
	in.applying = true
	in.selectRange(r)
	in.applying = false
}

// setBar makes bar the view above the editor's keyboard.
func (in *input) setBar(bar objc.ID) {
	in.bar = bar
	in.field.Send(sel("setInputAccessoryView:"), bar)
	if objc.Send[bool](in.field, sel("isFirstResponder")) {
		in.field.Send(sel("reloadInputViews"))
	}
}

// UIKit's text alignments.
const uiAlignLeft, uiAlignCenter, uiAlignRight = 0, 1, 2

// UIKeyboardType of each core keyboard type.
var keyboards = map[driver.KeyboardType]int{
	driver.KeyboardDefault: 0, driver.KeyboardURL: 3, driver.KeyboardNumber: 4,
	driver.KeyboardPhone: 5, driver.KeyboardEmail: 7, driver.KeyboardDecimal: 8,
}

// UIReturnKeyType of each return key.
var returnKeys = map[driver.ReturnKeyType]int{
	driver.ReturnKeyDefault: 0, driver.ReturnKeyGo: 1, driver.ReturnKeySearch: 6, driver.ReturnKeySend: 7,
	driver.ReturnKeyDone: 9, driver.ReturnKeyNext: 4,
}

// UITextAutocapitalizationType of each capitalization.
var capitalizations = map[driver.AutoCapitalize]int{
	driver.AutoCapitalizeNone: 0, driver.AutoCapitalizeWords: 1, driver.AutoCapitalizeSentences: 2,
	driver.AutoCapitalizeCharacters: 3,
}

// contentTypes are UIKit's UITextContentType constants.
var contentTypes = map[driver.AutoComplete]string{
	driver.AutoCompleteUsername: "UITextContentTypeUsername", driver.AutoCompletePassword: "UITextContentTypePassword",
	driver.AutoCompleteNewPassword: "UITextContentTypeNewPassword", driver.AutoCompleteOneTimeCode: "UITextContentTypeOneTimeCode",
	driver.AutoCompleteEmail: "UITextContentTypeEmailAddress", driver.AutoCompleteTel: "UITextContentTypeTelephoneNumber",
	driver.AutoCompleteName: "UITextContentTypeName", driver.AutoCompleteGivenName: "UITextContentTypeGivenName",
	driver.AutoCompleteFamilyName: "UITextContentTypeFamilyName", driver.AutoCompleteMiddleName: "UITextContentTypeMiddleName",
	driver.AutoCompleteNickname: "UITextContentTypeNickname", driver.AutoCompleteOrganization: "UITextContentTypeOrganizationName",
	driver.AutoCompleteStreetAddress: "UITextContentTypeFullStreetAddress", driver.AutoCompleteAddressLine1: "UITextContentTypeStreetAddressLine1",
	driver.AutoCompleteAddressLine2: "UITextContentTypeStreetAddressLine2", driver.AutoCompleteCity: "UITextContentTypeAddressCity",
	driver.AutoCompleteRegion: "UITextContentTypeAddressState", driver.AutoCompletePostalCode: "UITextContentTypePostalCode",
	driver.AutoCompleteCountry: "UITextContentTypeCountryName", driver.AutoCompleteCreditCardNumber: "UITextContentTypeCreditCardNumber",
	driver.AutoCompleteCreditCardExpiration:   "UITextContentTypeCreditCardExpiration",
	driver.AutoCompleteCreditCardSecurityCode: "UITextContentTypeCreditCardSecurityCode",
	driver.AutoCompleteCreditCardName:         "UITextContentTypeCreditCardName", driver.AutoCompleteURL: "UITextContentTypeURL",
	driver.AutoCompleteBirthdate: "UITextContentTypeBirthdate",
}

func uiColor(c driver.Color) objc.ID {
	r, g, b, a := c.Components()
	return objc.ID(objc.GetClass("UIColor")).Send(sel("colorWithRed:green:blue:alpha:"), r, g, b, a)
}

// configure is ItemInput.
func (v *view) configure(c *driver.InputConfig) {
	in := v.input
	if c.Multiline != in.multiline {
		focused := in.focused
		if focused {
			in.field.Send(selResign)
		}
		v.setEditor(c.Multiline)
		if focused {
			in.field.Send(selBecomeFirst)
		}
	}
	in.fs.Config = *c
	f := in.field
	font := apple.Font(c.Style.Font)
	f.Send(selSetFont, objc.ID(font))
	f.Send(selSetTextColor, uiColor(c.Style.Color))
	align := uiAlignLeft
	switch c.Style.Align {
	case driver.TextAlignCenter:
		align = uiAlignCenter
	case driver.TextAlignEnd:
		align = uiAlignRight
	}
	f.Send(selSetTextAlignment, align)
	placeholder := c.Style.Color.Faded(0.5)
	if c.PlaceholderColor != 0 {
		placeholder = c.PlaceholderColor
	}
	if in.multiline {
		in.label.Send(selSetFont, objc.ID(font))
		in.label.Send(selSetTextColor, uiColor(placeholder))
		in.label.Send(selSetTextAlignment, align)
		in.label.Send(selSetText, apple.NSString(c.Placeholder))
		f.Send(sel("setEditable:"), !c.Disabled)
		f.Send(sel("setSelectable:"), !c.Disabled)
	} else {
		attrs := objc.ID(objc.GetClass("NSMutableDictionary")).Send(sel("dictionary"))
		attrs.Send(sel("setObject:forKey:"), uiColor(placeholder), apple.Global(uikitPath, "NSForegroundColorAttributeName"))
		attrs.Send(sel("setObject:forKey:"), objc.ID(font), apple.Global(uikitPath, "NSFontAttributeName"))
		str := objc.ID(objc.GetClass("NSAttributedString")).Send(selAlloc).Send(
			sel("initWithString:attributes:"), apple.NSString(c.Placeholder), attrs)
		f.Send(sel("setAttributedPlaceholder:"), str)
		str.Send(selRelease)
		f.Send(selSetEnabled, !c.Disabled)
	}
	apple.Release(font)
	f.Send(selSetSecure, c.Masked && !c.Multiline)
	f.Send(selSetKeyboardType, keyboards[c.KeyboardType])
	f.Send(sel("setReturnKeyType:"), returnKeys[c.ReturnKey])
	f.Send(sel("setAutocapitalizationType:"), capitalizations[c.AutoCapitalize])
	correction := 0 // default
	if c.NoAutoCorrect {
		correction = 1
	}
	f.Send(sel("setAutocorrectionType:"), correction)
	f.Send(sel("setKeyboardAppearance:"), int(c.KeyboardAppearance))
	var contentType objc.ID
	if name := contentTypes[c.AutoComplete]; name != "" {
		contentType = apple.Global(uikitPath, name)
	} else if c.AutoComplete == driver.AutoCompleteOff {
		contentType = apple.NSString("")
	}
	f.Send(sel("setTextContentType:"), contentType)
	if c.Caret() != 0 {
		f.Send(sel("setTintColor:"), uiColor(c.Caret()))
	} else {
		f.Send(sel("setTintColor:"), objc.ID(0))
	}
	in.updatePlaceholder()
	v.inset = c.Content
	v.native.Send(selSetNeedsLayout)
}

func inputLayout(self objc.ID, cmd objc.SEL) {
	apple.Super(self, cmd)
	v := views[self]
	in, box := v.input, v.inset
	frame := apple.Rect(float64(box.Origin.X), float64(box.Origin.Y), float64(box.Size.Width), float64(box.Size.Height))
	in.field.Send(selSetFrame, frame)
	if in.label != 0 {
		fit := objc.Send[apple.CGSize](in.label, sel("sizeThatFits:"), apple.CGSize{Width: frame.Size.Width, Height: 1e7})
		frame.Size.Height = min(fit.Height, frame.Size.Height)
		in.label.Send(selSetFrame, frame)
	}
}
