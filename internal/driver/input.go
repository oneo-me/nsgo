package driver

import (
	"unicode/utf16"
)

// TextRange is a selection in a text field, in UTF-16 units as the
// platforms' text APIs count; Start equals End for a caret.
type TextRange struct {
	Start, End int
}

// SubmitBehavior is what Return does in a text field.
type SubmitBehavior uint8

const (
	// SubmitBehaviorDefault inserts a line break in a multiline field,
	// and blurs and submits in a single line.
	SubmitBehaviorDefault SubmitBehavior = iota
	// SubmitBehaviorSubmit reports OnSubmit and keeps the focus.
	SubmitBehaviorSubmit
	// SubmitBehaviorBlurAndSubmit reports OnSubmit and takes the focus
	// away.
	SubmitBehaviorBlurAndSubmit
	// SubmitBehaviorNewline inserts a line break in a multiline field; a
	// single line blurs and submits.
	SubmitBehaviorNewline
)

// Submit is what Return does, as SubmitBehavior and Multiline say
// together, as NSH's InputConfig::submit: never SubmitBehaviorDefault,
// and SubmitBehaviorNewline only when multiline. Platforms read only
// this. A multiline field that submits still takes a line break from
// Shift, Option or Alt with Return on hardware keyboards, and a Return
// that commits an input method's composition never submits.
func (c *InputConfig) Submit() SubmitBehavior {
	switch b := c.SubmitBehavior; {
	case b == SubmitBehaviorNewline && !c.Multiline:
		return SubmitBehaviorBlurAndSubmit
	case b != SubmitBehaviorDefault:
		return b
	case c.Multiline:
		return SubmitBehaviorNewline
	}
	return SubmitBehaviorBlurAndSubmit
}

// ReturnKeyType labels the software keyboard's return key.
type ReturnKeyType uint8

const (
	ReturnKeyDefault ReturnKeyType = iota
	ReturnKeyDone
	ReturnKeyGo
	ReturnKeyNext
	ReturnKeySearch
	ReturnKeySend
)

// AutoCapitalize is a field's automatic capitalization; sentences are
// the default.
type AutoCapitalize uint8

const (
	AutoCapitalizeSentences AutoCapitalize = iota
	AutoCapitalizeNone
	AutoCapitalizeWords
	AutoCapitalizeCharacters
)

// KeyboardAppearance is the look of iOS's keyboard; the default follows
// the app's appearance.
type KeyboardAppearance uint8

// AutoComplete says what a field holds, for the system's autofill and
// suggestions: React Native's autoComplete, which iOS maps to its
// textContentType.
type AutoComplete uint8

const (
	// AutoCompleteDefault lets the platform decide.
	AutoCompleteDefault AutoComplete = iota
	AutoCompleteOff
	AutoCompleteUsername
	// AutoCompletePassword is an existing password.
	AutoCompletePassword
	// AutoCompleteNewPassword is a password being chosen.
	AutoCompleteNewPassword
	// AutoCompleteOneTimeCode is a code from a text message or e-mail.
	AutoCompleteOneTimeCode
	AutoCompleteEmail
	AutoCompleteTel
	AutoCompleteName
	AutoCompleteGivenName
	AutoCompleteFamilyName
	AutoCompleteMiddleName
	AutoCompleteNickname
	AutoCompleteOrganization
	// AutoCompleteStreetAddress is every line of a street address.
	AutoCompleteStreetAddress
	AutoCompleteAddressLine1
	AutoCompleteAddressLine2
	AutoCompleteCity
	// AutoCompleteRegion is a state or region.
	AutoCompleteRegion
	AutoCompletePostalCode
	AutoCompleteCountry
	AutoCompleteCreditCardNumber
	AutoCompleteCreditCardExpiration
	AutoCompleteCreditCardSecurityCode
	AutoCompleteCreditCardName
	AutoCompleteURL
	AutoCompleteBirthdate
)

var autoCompleteNames = [...]string{"", "off", "username", "current-password", "new-password", "one-time-code",
	"email", "tel", "name", "given-name", "family-name", "additional-name", "nickname", "organization",
	"street-address", "address-line1", "address-line2", "address-level2", "address-level1", "postal-code",
	"country-name", "cc-number", "cc-exp", "cc-csc", "cc-name", "url", "bday"}

// String returns the HTML autocomplete token, empty for the default.
func (a AutoComplete) String() string {
	if int(a) < len(autoCompleteNames) {
		return autoCompleteNames[a]
	}
	return ""
}

// UTF16Len is the length of s in UTF-16 units.
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// ClipUTF16 cuts s to at most max UTF-16 units, never inside a
// character; max 0 or less keeps all of it.
func ClipUTF16(s string, max int) string {
	if max <= 0 {
		return s
	}
	n := 0
	for i, r := range s {
		if n += utf16.RuneLen(r); n > max {
			return s[:i]
		}
	}
	return s
}

// UTF16Offset converts a UTF-16 offset in s to a byte offset, clamped
// to s.
func UTF16Offset(s string, units int) int {
	n := 0
	for i, r := range s {
		if n >= units {
			return i
		}
		n += utf16.RuneLen(r)
	}
	return len(s)
}

// UTF16Units converts a byte offset in s to UTF-16 units.
func UTF16Units(s string, bytes int) int {
	return UTF16Len(s[:min(max(bytes, 0), len(s))])
}

// LimitEdit is what a field with maxLength keeps of an edit replacing
// replaced units of text with insert: insert cut to the room left, as
// the platforms' own limits do. ok is false when nothing of a non-empty
// insert fits.
func LimitEdit(text string, replaced int, insert string, maxLength int) (kept string, ok bool) {
	if maxLength <= 0 {
		return insert, true
	}
	room := maxLength - (UTF16Len(text) - replaced)
	kept = ClipUTF16(insert, max(room, 0))
	if room <= 0 {
		kept = ""
	}
	return kept, kept != "" || insert == ""
}

// KeyOf is what EventKeyPress reports for an edit inserting text: the
// text, or "Enter" for a line break.
func KeyOf(text string) string {
	if text == "\n" || text == "\r" || text == "\r\n" {
		return "Enter"
	}
	return text
}

// Caret is the color of the caret: CaretColor, or else SelectionColor.
func (c *InputConfig) Caret() Color {
	if c.CaretColor != 0 {
		return c.CaretColor
	}
	return c.SelectionColor
}
