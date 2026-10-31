//go:build linux && !android

package linux

import (
	"fmt"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Text inputs are GTK's own: a GtkEntry for one line, or a GtkTextView in
// a GtkScrolledWindow when multiline, hosted in the input's widget at its
// content box; the view draws the box. A text view has no placeholder, so
// a label under it shows one while the text is empty. The editors' signals
// answer maxLength (insert-text, cut and inserted again) and Return
// natively, and field.State turns them into NSH's events, holding a value
// the app sets while an input method preedits until the preedit ends.
// GTK counts characters, which the backend converts to UTF-16 units.
//
// Each editor's CSS is a display provider scoped to the editor's name,
// since a widget's own provider never reaches the nodes inside it: the
// selection, the placeholder, the caret.

// GtkTextIter, opaque and on the stack in C; generously sized.
type textIter [32]uintptr

var (
	gtkTextViewNew                 func() uintptr
	gtkTextViewGetBuffer           func(uintptr) uintptr
	gtkTextViewSetWrapMode         func(uintptr, int32)
	gtkTextViewSetEditable         func(uintptr, int32)
	gtkTextViewSetCursorVisible    func(uintptr, int32)
	gtkTextViewSetJustification    func(uintptr, int32)
	gtkTextViewSetAcceptsTab       func(uintptr, int32)
	gtkTextViewSetInputPurpose     func(uintptr, int32)
	gtkTextViewSetInputHints       func(uintptr, int32)
	gtkTextBufferSetText           func(uintptr, *byte, int32)
	gtkTextBufferGetText           func(uintptr, *textIter, *textIter, int32) uintptr
	gtkTextBufferGetBounds         func(uintptr, *textIter, *textIter)
	gtkTextBufferGetSelectionBound func(uintptr, *textIter, *textIter) int32
	gtkTextBufferGetIterAtOffset   func(uintptr, *textIter, int32)
	gtkTextBufferSelectRange       func(uintptr, *textIter, *textIter)
	gtkTextBufferInsert            func(uintptr, uintptr, *byte, int32)
	gtkTextBufferInsertAtCursor    func(uintptr, *byte, int32)
	gtkTextIterGetOffset           func(uintptr) int32
	gtkScrolledWindowNew           func() uintptr
	gtkScrolledWindowSetChild      func(uintptr, uintptr)
	gtkScrolledWindowSetPolicy     func(uintptr, int32, int32)
	gtkEditableGetSelectionBounds  func(uintptr, *int32, *int32) int32
	gtkEditableInsertText          func(uintptr, *byte, int32, uintptr)
	gtkEditableGetDelegate         func(uintptr) uintptr
	gtkEntrySetInputPurpose        func(uintptr, int32)
	gtkEntrySetInputHints          func(uintptr, int32)
	gtkWidgetSetName               func(uintptr, *byte)
	gtkLabelSetText                func(uintptr, *byte)
	gtkLabelSetXalign              func(uintptr, float32)
	gSignalStopEmissionByName      func(uintptr, *byte)
	gtkStyleContextAddForDisplay   func(uintptr, uintptr, uint32)
	gtkStyleContextRemoveDisplay   func(uintptr, uintptr)

	cbEntryInsert, cbEntryDelete, cbEntrySelection, cbPreedit                uintptr
	cbBufferChanged, cbBufferInsert, cbBufferDelete, cbBufferMark, cbAreaKey uintptr
)

func bindEditing() {
	for _, f := range []struct {
		fn   any
		name string
	}{
		{&gtkTextViewNew, "gtk_text_view_new"},
		{&gtkTextViewGetBuffer, "gtk_text_view_get_buffer"},
		{&gtkTextViewSetWrapMode, "gtk_text_view_set_wrap_mode"},
		{&gtkTextViewSetEditable, "gtk_text_view_set_editable"},
		{&gtkTextViewSetCursorVisible, "gtk_text_view_set_cursor_visible"},
		{&gtkTextViewSetJustification, "gtk_text_view_set_justification"},
		{&gtkTextViewSetAcceptsTab, "gtk_text_view_set_accepts_tab"},
		{&gtkTextViewSetInputPurpose, "gtk_text_view_set_input_purpose"},
		{&gtkTextViewSetInputHints, "gtk_text_view_set_input_hints"},
		{&gtkTextBufferSetText, "gtk_text_buffer_set_text"},
		{&gtkTextBufferGetText, "gtk_text_buffer_get_text"},
		{&gtkTextBufferGetBounds, "gtk_text_buffer_get_bounds"},
		{&gtkTextBufferGetSelectionBound, "gtk_text_buffer_get_selection_bounds"},
		{&gtkTextBufferGetIterAtOffset, "gtk_text_buffer_get_iter_at_offset"},
		{&gtkTextBufferSelectRange, "gtk_text_buffer_select_range"},
		{&gtkTextBufferInsert, "gtk_text_buffer_insert"},
		{&gtkTextBufferInsertAtCursor, "gtk_text_buffer_insert_at_cursor"},
		{&gtkTextIterGetOffset, "gtk_text_iter_get_offset"},
		{&gtkScrolledWindowNew, "gtk_scrolled_window_new"},
		{&gtkScrolledWindowSetChild, "gtk_scrolled_window_set_child"},
		{&gtkScrolledWindowSetPolicy, "gtk_scrolled_window_set_policy"},
		{&gtkEditableGetSelectionBounds, "gtk_editable_get_selection_bounds"},
		{&gtkEditableInsertText, "gtk_editable_insert_text"},
		{&gtkEditableGetDelegate, "gtk_editable_get_delegate"},
		{&gtkEntrySetInputPurpose, "gtk_entry_set_input_purpose"},
		{&gtkEntrySetInputHints, "gtk_entry_set_input_hints"},
		{&gtkWidgetSetName, "gtk_widget_set_name"},
		{&gtkLabelSetText, "gtk_label_set_text"},
		{&gtkLabelSetXalign, "gtk_label_set_xalign"},
		{&gtkStyleContextAddForDisplay, "gtk_style_context_add_provider_for_display"},
		{&gtkStyleContextRemoveDisplay, "gtk_style_context_remove_provider_for_display"},
	} {
		purego.RegisterLibFunc(f.fn, gtkLib, f.name)
	}
	purego.RegisterLibFunc(&gSignalStopEmissionByName, gobjectLib, "g_signal_stop_emission_by_name")
}

func init() {
	cbEntryInsert = newCallback(entryInsertCallback)
	cbEntryDelete = newCallback(entryDeleteCallback)
	cbEntrySelection = newCallback(entrySelectionCallback)
	cbPreedit = newCallback(preeditCallback)
	cbBufferChanged = newCallback(bufferChangedCallback)
	cbBufferInsert = newCallback(bufferInsertCallback)
	cbBufferDelete = newCallback(bufferDeleteCallback)
	cbBufferMark = newCallback(bufferMarkCallback)
	cbAreaKey = newCallback(areaKeyCallback)
}

// cssColor is c in CSS.
func cssColor(c driver.Color) string {
	r, g, b, a := c.Components()
	return fmt.Sprintf("rgba(%d,%d,%d,%.3f)", int(r*255), int(g*255), int(b*255), a)
}

// editorName is the widget name v's CSS selects.
func editorName(v *view) string { return fmt.Sprintf("nsgo-input-%d", v.id) }

// entryStyle is the CSS an editor takes: no look of GTK's own, the view
// draws its box, and the input's text style, caret, selection and
// placeholder colors.
func entryStyle(v *view) string {
	in := &v.input
	f := in.Style.Font
	name := "#" + editorName(v)
	all := name + ", " + name + " *"
	var b strings.Builder
	fmt.Fprintf(&b, "%s { padding: 0; margin: 0; min-width: 0; min-height: 0; border: none; border-radius: 0; box-shadow: none; background: none; background-color: transparent; outline: none; }\n", all)
	caret := in.Style.Color
	if in.SelectionColor != 0 {
		caret = in.SelectionColor
	}
	caretCSS := cssColor(caret)
	if in.HideCaret {
		caretCSS = "transparent"
	}
	fmt.Fprintf(&b, "%s { color: %s; caret-color: %s; }\n", all, cssColor(in.Style.Color), caretCSS)
	if f.Family != "" {
		fmt.Fprintf(&b, "%s { font-family: %q; }\n", all, f.Family)
	}
	if f.Size > 0 {
		fmt.Fprintf(&b, "%s { font-size: %.3fpt; }\n", all, float64(f.Size))
	}
	if f.Weight > 0 {
		fmt.Fprintf(&b, "%s { font-weight: %d; }\n", all, int(f.Weight))
	}
	if f.Italic {
		fmt.Fprintf(&b, "%s { font-style: italic; }\n", all)
	}
	if tags := f.Variant.Tags(); len(tags) > 0 {
		fmt.Fprintf(&b, "%s { font-feature-settings: \"%s\" 1; }\n", all, strings.Join(tags, "\" 1, \""))
	}
	if c := in.SelectionColor; c != 0 {
		fmt.Fprintf(&b, "%s selection { background-color: %s; color: %s; }\n", name, cssColor(c.Faded(0.3)), cssColor(in.Style.Color))
	}
	placeholder := in.Style.Color.Faded(0.5)
	if in.PlaceholderColor != 0 {
		placeholder = in.PlaceholderColor
	}
	fmt.Fprintf(&b, "%s placeholder, #%s-hint { color: %s; opacity: 1; }\n", name, editorName(v), cssColor(placeholder))
	return b.String()
}

// configure applies an input's look and behavior to its editor, making
// it on the first configuration and again when it becomes or stops being
// multiline.
func (w *window) configure(v *view, in *driver.InputConfig) {
	v.input = *in
	v.fs.Config = *in
	if v.kind != driver.KindInput {
		return
	}
	if v.entry != 0 && in.Multiline != (v.textView != 0) {
		text := w.editorText(v)
		focused := w.focus == v
		w.dropEditor(v)
		w.makeEditor(v, in.Multiline)
		w.setText(v, text)
		if focused {
			w.focusEntry(v)
		}
	} else if v.entry == 0 {
		w.makeEditor(v, in.Multiline)
	}
	gtkWidgetQueueAllocate(v.widget)
	editable := boolInt(!in.Readonly && !in.Disabled)
	purpose, hints := inputPurpose(in)
	if v.textView != 0 {
		tv := v.textView
		gtkTextViewSetEditable(tv, editable)
		gtkTextViewSetCursorVisible(tv, boolInt(!in.HideCaret && !in.Readonly))
		gtkTextViewSetInputPurpose(tv, purpose)
		gtkTextViewSetInputHints(tv, hints)
		justify, x := int32(0), float32(0)
		switch in.Style.Align {
		case driver.TextAlignCenter:
			justify, x = 2, 0.5
		case driver.TextAlignEnd:
			justify, x = 1, 1
		}
		gtkTextViewSetJustification(tv, justify)
		gtkLabelSetText(v.hint, cstr(in.Placeholder))
		gtkLabelSetXalign(v.hint, x)
		w.updateHint(v)
	} else {
		gtkEntrySetPlaceholderText(v.entry, cstr(in.Placeholder))
		gtkEntrySetVisibility(v.entry, boolInt(!in.Masked))
		gtkEditableSetEditable(v.entry, editable)
		gtkEntrySetInputPurpose(v.entry, purpose)
		gtkEntrySetInputHints(v.entry, hints)
		switch in.Style.Align {
		case driver.TextAlignCenter:
			gtkEntrySetAlignment(v.entry, 0.5)
		case driver.TextAlignEnd:
			gtkEntrySetAlignment(v.entry, 1)
		default:
			gtkEntrySetAlignment(v.entry, 0)
		}
	}
	gtkWidgetSetSensitive(v.entry, boolInt(!in.Disabled))
	w.styleEntry(v)
}

// GTK's input purposes and hints.
const (
	purposeFreeForm, purposeDigits, purposeNumber, purposePhone = 0, 2, 3, 4
	purposeURL, purposeEmail, purposeName, purposePassword      = 5, 6, 7, 8
	purposePIN                                                  = 9

	hintSpellcheck, hintNoSpellcheck           = 1 << 0, 1 << 1
	hintUppercaseChars, hintUppercaseWords     = 1 << 4, 1 << 5
	hintUppercaseSentences, hintWordCompletion = 1 << 6, 1 << 2
)

// inputPurpose is what an input holds and how it is typed, for GTK's
// input methods and on-screen keyboards.
func inputPurpose(in *driver.InputConfig) (purpose, hints int32) {
	switch in.AutoComplete {
	case driver.AutoCompleteEmail:
		purpose = purposeEmail
	case driver.AutoCompleteTel:
		purpose = purposePhone
	case driver.AutoCompleteURL:
		purpose = purposeURL
	case driver.AutoCompleteName, driver.AutoCompleteGivenName, driver.AutoCompleteFamilyName, driver.AutoCompleteMiddleName:
		purpose = purposeName
	case driver.AutoCompletePassword, driver.AutoCompleteNewPassword:
		purpose = purposePassword
	case driver.AutoCompleteOneTimeCode:
		purpose = purposePIN
	default:
		purpose = map[driver.KeyboardType]int32{driver.KeyboardEmail: purposeEmail, driver.KeyboardNumber: purposeDigits,
			driver.KeyboardDecimal: purposeNumber, driver.KeyboardPhone: purposePhone, driver.KeyboardURL: purposeURL}[in.KeyboardType]
	}
	if in.Masked && !in.Multiline {
		purpose = purposePassword
	}
	hints = hintSpellcheck | hintWordCompletion
	if in.NoAutoCorrect {
		hints = hintNoSpellcheck
	}
	switch in.AutoCapitalize {
	case driver.AutoCapitalizeSentences:
		hints |= hintUppercaseSentences
	case driver.AutoCapitalizeWords:
		hints |= hintUppercaseWords
	case driver.AutoCapitalizeCharacters:
		hints |= hintUppercaseChars
	}
	return purpose, hints
}

// makeEditor makes v's editor and connects its signals.
func (w *window) makeEditor(v *view, multiline bool) {
	v.fs.ID, v.fs.Dispatch = v.id, func(ev driver.Event) {
		if !w.closing && !v.deleted {
			w.engine.Dispatch(ev)
		}
	}
	if multiline {
		v.hint = gtkLabelNew(cstr(""))
		gObjectRefSink(v.hint)
		gtkLabelSetWrap(v.hint, 1)
		gtkWidgetSetName(v.hint, cstr(editorName(v)+"-hint"))
		gtkWidgetSetParent(v.hint, v.widget)
		v.entry = gtkScrolledWindowNew()
		gObjectRefSink(v.entry)
		gtkScrolledWindowSetPolicy(v.entry, 2, 1) // never across, as needed down
		v.textView = gtkTextViewNew()
		gtkTextViewSetWrapMode(v.textView, 3) // words, then characters
		gtkTextViewSetAcceptsTab(v.textView, 0)
		gtkScrolledWindowSetChild(v.entry, v.textView)
		v.buffer = gtkTextViewGetBuffer(v.textView)
		w.entries[v.textView] = v
		w.entries[v.buffer] = v
		gSignalConnect(v.buffer, "changed", cbBufferChanged, w.handle)
		gSignalConnect(v.buffer, "insert-text", cbBufferInsert, w.handle)
		gSignalConnect(v.buffer, "delete-range", cbBufferDelete, w.handle)
		gSignalConnect(v.buffer, "mark-set", cbBufferMark, w.handle)
		gSignalConnect(v.textView, "preedit-changed", cbPreedit, w.handle)
		gSignalConnect(v.textView, "notify::has-focus", cbEntryFocus, w.handle)
		gSignalConnect(v.textView, "paste-clipboard", cbPasteClipboard, w.handle)
		keys := gtkEventControllerKeyNew()
		gtkEventControllerSetPropagationPhase(keys, propagationPhaseCapture)
		gSignalConnect(keys, "key-pressed", cbAreaKey, w.handle)
		gtkWidgetAddController(v.textView, keys)
		w.entries[keys] = v
		gtkWidgetSetName(v.textView, cstr(editorName(v)))
	} else {
		v.entry = gtkEntryNew()
		gObjectRefSink(v.entry)
		gSignalConnect(v.entry, "changed", cbEntryChanged, w.handle)
		gSignalConnect(v.entry, "activate", cbEntryActivate, w.handle)
		gSignalConnect(v.entry, "insert-text", cbEntryInsert, w.handle)
		gSignalConnect(v.entry, "delete-text", cbEntryDelete, w.handle)
		gSignalConnect(v.entry, "notify::cursor-position", cbEntrySelection, w.handle)
		gSignalConnect(v.entry, "notify::selection-bound", cbEntrySelection, w.handle)
		// The entry's text widget, not the entry, takes the focus.
		if text := gtkEditableGetDelegate(v.entry); text != 0 {
			gSignalConnect(text, "notify::has-focus", cbEntryFocus, w.handle)
			gSignalConnect(text, "preedit-changed", cbPreedit, w.handle)
			gSignalConnect(text, "paste-clipboard", cbPasteClipboard, w.handle)
			w.entries[text] = v
		}
		gtkWidgetSetName(v.entry, cstr(editorName(v)))
	}
	// Hosted in the input's widget, after its children, at its content.
	gtkWidgetSetParent(v.entry, v.widget)
	w.entries[v.entry] = v
	if v.provider == 0 {
		v.provider = gtkCssProviderNew()
		gtkStyleContextAddForDisplay(gdkDisplayGetDefault(), v.provider, 800)
	}
}

// dropEditor unparents and releases v's editor.
func (w *window) dropEditor(v *view) {
	if w.focus == v {
		w.focus = nil
	}
	for k, e := range w.entries {
		if e == v {
			delete(w.entries, k)
		}
	}
	for _, p := range []*uintptr{&v.entry, &v.hint} {
		if *p != 0 {
			w.unparentWidget(*p)
			gObjectUnref(*p)
			*p = 0
		}
	}
	v.textView, v.buffer = 0, 0
}

// releaseEditor lets a deleted input's editor and CSS go.
func (w *window) releaseEditor(v *view) {
	if v.provider != 0 {
		gtkStyleContextRemoveDisplay(gdkDisplayGetDefault(), v.provider)
		gObjectUnref(v.provider)
		v.provider = 0
	}
	if v.entry != 0 {
		w.dropEditor(v)
	}
}

// styleEntry reloads the editor's CSS from its input configuration.
func (w *window) styleEntry(v *view) {
	if v.provider != 0 {
		gtkCssProviderLoadFromString(v.provider, cstr(entryStyle(v)))
	}
}

// editorText is the text v's editor holds.
func (w *window) editorText(v *view) string {
	if v.buffer != 0 {
		var start, end textIter
		gtkTextBufferGetBounds(v.buffer, &start, &end)
		p := gtkTextBufferGetText(v.buffer, &start, &end, 0)
		s := goString(p)
		gFree(p)
		return s
	}
	if v.entry != 0 {
		return goString(gtkEditableGetText(v.entry))
	}
	return ""
}

// setText sets the editor's text, reporting nothing.
func (w *window) setText(v *view, text string) {
	if w.editorText(v) == text {
		return
	}
	v.setting = true
	if v.buffer != 0 {
		gtkTextBufferSetText(v.buffer, cstr(text), -1)
	} else {
		gtkEditableSetText(v.entry, cstr(text))
	}
	v.setting = false
	w.updateHint(v)
}

// updateHint shows a text view's placeholder while its text is empty.
func (w *window) updateHint(v *view) {
	if v.hint != 0 {
		gtkWidgetSetVisible(v.hint, boolInt(v.input.Placeholder != "" && w.editorText(v) == ""))
	}
}

// setValue sets an input's text unless the user edited it after edit
// number count; during a preedit it waits for its end.
func (w *window) setValue(v *view, value string, count uint32) {
	if v.entry != 0 && v.fs.Value(value, count) {
		w.setText(v, value)
	}
}

// selection is the editor's selection in characters.
func (w *window) selection(v *view) (start, end int) {
	if v.buffer != 0 {
		var s, e textIter
		gtkTextBufferGetSelectionBound(v.buffer, &s, &e)
		return int(gtkTextIterGetOffset(uintptr(unsafe.Pointer(&s)))), int(gtkTextIterGetOffset(uintptr(unsafe.Pointer(&e))))
	}
	var s, e int32
	gtkEditableGetSelectionBounds(v.entry, &s, &e)
	return int(min(s, e)), int(max(s, e))
}

// unitsOf converts n characters of text to UTF-16 units, and charsOf
// back.
func unitsOf(text string, n int) int {
	i := 0
	for b := range text {
		if i == n {
			return driver.UTF16Units(text, b)
		}
		i++
	}
	return driver.UTF16Len(text)
}

func charsOf(text string, units int) int {
	return utf8.RuneCountInString(text[:driver.UTF16Offset(text, units)])
}

// selectRange is ItemSelection.
func (w *window) selectRange(v *view, r driver.TextRange) {
	if v.entry == 0 {
		return
	}
	text := w.editorText(v)
	start, end := charsOf(text, max(r.Start, 0)), charsOf(text, max(r.End, r.Start))
	v.setting = true
	if v.buffer != 0 {
		var s, e textIter
		gtkTextBufferGetIterAtOffset(v.buffer, &s, int32(end))
		gtkTextBufferGetIterAtOffset(v.buffer, &e, int32(start))
		gtkTextBufferSelectRange(v.buffer, &s, &e)
	} else {
		gtkEditableSelectRegion(v.entry, int32(start), int32(end))
	}
	v.setting = false
	v.fs.SetSelection(driver.TextRange{Start: unitsOf(text, start), End: unitsOf(text, end)})
}

// reportSelection reports the user's selection in UTF-16 units.
func (w *window) reportSelection(v *view) {
	if !v.setting {
		w.caretMoved(v)
	}
	if v.setting || v.entry == 0 || !v.fs.Events.Has(driver.EventSelectionChange) {
		return
	}
	text := w.editorText(v)
	s, e := w.selection(v)
	v.fs.Selected(driver.TextRange{Start: unitsOf(text, s), End: unitsOf(text, e)})
}

// edited reports the user's edit.
func (w *window) edited(v *view) {
	w.updateHint(v)
	if v.setting {
		return
	}
	v.fs.Edited(w.editorText(v))
	w.reportSelection(v)
}

// focusedEntry returns the input with keyboard focus, or nil.
func (w *window) focusedEntry() *view { return w.focus }

// focusEntry moves keyboard focus to v's editor, or away from the
// focused one when v is nil.
func (w *window) focusEntry(v *view) {
	if v != nil && v.entry != 0 {
		if v.textView != 0 {
			gtkWidgetGrabFocus(v.textView)
		} else {
			gtkWidgetGrabFocus(v.entry)
		}
		return
	}
	if top := w.toplevel(); top != 0 {
		gtkWindowSetFocus(top, 0)
	}
}

// editorOf returns the live input an editor's signal came from.
func editorOf(widget, data uintptr) (*window, *view) {
	w := windowOf(data)
	if w == nil || w.closing {
		return nil, nil
	}
	v := w.entries[widget]
	if v == nil || v.deleted {
		return nil, nil
	}
	return w, v
}

// entryChangedCallback reports a user's edit of a field.
func entryChangedCallback(entry uintptr, data uintptr) {
	if w, v := editorOf(entry, data); v != nil {
		w.edited(v)
	}
}

// entryActivateCallback is Return in a field.
func entryActivateCallback(entry uintptr, data uintptr) {
	if w, v := editorOf(entry, data); v != nil {
		if _, blur := v.fs.Return(w.editorText(v), false); blur {
			w.focusEntry(nil)
		}
	}
}

// insertion answers whether the user may insert text, reporting its key:
// a cut insertion is made here instead.
func (w *window) insertion(v *view, text string, insert func(kept string)) bool {
	if v.setting || v.clipping {
		return true
	}
	kept, ok, whole := v.fs.Insert(w.editorText(v), 0, text)
	if whole {
		return true
	}
	if ok && kept != "" {
		v.clipping = true
		insert(kept)
		v.clipping = false
	}
	return false
}

func entryInsertCallback(entry uintptr, text uintptr, length int32, position uintptr, data uintptr) {
	w, v := editorOf(entry, data)
	if v == nil {
		return
	}
	s := goString(text)
	if length >= 0 && int(length) < len(s) {
		s = s[:length]
	}
	if !w.insertion(v, s, func(kept string) { gtkEditableInsertText(entry, cstr(kept), -1, position) }) {
		gSignalStopEmissionByName(entry, cstr("insert-text"))
	}
}

func entryDeleteCallback(entry uintptr, start, end int32, data uintptr) {
	if _, v := editorOf(entry, data); v != nil && !v.setting && end != start {
		v.fs.Key("Backspace")
	}
}

func entrySelectionCallback(entry uintptr, pspec uintptr, data uintptr) {
	if w, v := editorOf(entry, data); v != nil {
		w.reportSelection(v)
	}
}

// preeditCallback follows an input method's preedit: its end applies a
// value the app set while it lasted.
func preeditCallback(widget uintptr, preedit uintptr, data uintptr) {
	w, v := editorOf(widget, data)
	if v == nil {
		return
	}
	text := goString(preedit)
	if value, apply := v.fs.Compose(text != "", text, w.editorText(v)); apply {
		w.setText(v, value)
	}
}

func bufferChangedCallback(buffer uintptr, data uintptr) {
	if w, v := editorOf(buffer, data); v != nil {
		w.edited(v)
	}
}

func bufferInsertCallback(buffer uintptr, iter uintptr, text uintptr, length int32, data uintptr) {
	w, v := editorOf(buffer, data)
	if v == nil {
		return
	}
	s := goString(text)
	if length >= 0 && int(length) < len(s) {
		s = s[:length]
	}
	if !w.insertion(v, s, func(kept string) { gtkTextBufferInsert(buffer, iter, cstr(kept), -1) }) {
		gSignalStopEmissionByName(buffer, cstr("insert-text"))
	}
}

func bufferDeleteCallback(buffer uintptr, start, end uintptr, data uintptr) {
	if _, v := editorOf(buffer, data); v != nil && !v.setting && gtkTextIterGetOffset(start) != gtkTextIterGetOffset(end) {
		v.fs.Key("Backspace")
	}
}

func bufferMarkCallback(buffer uintptr, iter uintptr, mark uintptr, data uintptr) {
	if w, v := editorOf(buffer, data); v != nil {
		w.reportSelection(v)
	}
}

// areaKeyCallback answers Return in a text view: it breaks the line
// unless SubmitBehavior submits, and Shift or Alt with it always break.
func areaKeyCallback(controller uintptr, keyval, keycode, state uint32, data uintptr) int32 {
	w, v := editorOf(controller, data)
	if v == nil || keyval != gdkReturn && keyval != gdkKPEnter || v.fs.Composing() {
		return 0
	}
	insert, blur := v.fs.Return(w.editorText(v), state&(gdkShift|gdkAlt) != 0)
	if insert {
		return 0
	}
	if blur {
		w.focusEntry(nil)
	}
	return 1
}

// entryFocusCallback reports an editor taking or losing keyboard focus.
func entryFocusCallback(entry uintptr, pspec uintptr, data uintptr) {
	w, v := editorOf(entry, data)
	if v == nil {
		return
	}
	if gtkWidgetHasFocus(entry) != 0 {
		w.focus = v
		if v.events.Has(driver.EventFocus) {
			w.engine.Dispatch(driver.Event{Kind: driver.EventFocus, ID: v.id})
		}
		w.caretMoved(v)
		return
	}
	if w.focus == v {
		w.focus = nil
		if value, apply := v.fs.Compose(false, "", w.editorText(v)); apply {
			w.setText(v, value)
		}
		if v.events.Has(driver.EventBlur) {
			w.engine.Dispatch(driver.Event{Kind: driver.EventBlur, ID: v.id})
		}
	}
}
