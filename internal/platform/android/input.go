//go:build android

package android

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
)

// autofillHints are Android's autofill hints of each AutoComplete, as
// NSH's: "" turns autofill off and "-" leaves it to the system.
var autofillHints = map[driver.AutoComplete]string{
	driver.AutoCompleteDefault: "-", driver.AutoCompleteOff: "",
	driver.AutoCompleteUsername: "username", driver.AutoCompletePassword: "password",
	driver.AutoCompleteNewPassword: "newPassword", driver.AutoCompleteOneTimeCode: "smsOTPCode",
	driver.AutoCompleteEmail: "emailAddress", driver.AutoCompleteTel: "phone", driver.AutoCompleteName: "personName",
	driver.AutoCompleteGivenName: "personGivenName", driver.AutoCompleteFamilyName: "personFamilyName",
	driver.AutoCompleteMiddleName: "personMiddleName", driver.AutoCompleteNickname: "nickname",
	driver.AutoCompleteOrganization: "organization", driver.AutoCompleteStreetAddress: "postalAddress",
	driver.AutoCompleteAddressLine1: "streetAddress", driver.AutoCompleteAddressLine2: "extendedAddress",
	driver.AutoCompleteCity: "addressLocality", driver.AutoCompleteRegion: "addressRegion",
	driver.AutoCompletePostalCode: "postalCode", driver.AutoCompleteCountry: "addressCountry",
	driver.AutoCompleteCreditCardNumber: "creditCardNumber", driver.AutoCompleteCreditCardExpiration: "creditCardExpirationDate",
	driver.AutoCompleteCreditCardSecurityCode: "creditCardSecurityCode", driver.AutoCompleteCreditCardName: "personName",
	driver.AutoCompleteURL: "url", driver.AutoCompleteBirthdate: "birthDateFull",
}

// writeEditing appends ItemInput's editing props, which the runtime's
// InputField reads after the accessory: multiline, maxLength, what Return
// does (InputConfig.Submit), the return key, capitalization, flags (no
// autocorrection, no caret), the autofill hint, the placeholder and
// selection colors (RGBA, 0 for the platform's) and the PasteBehavior.
func writeEditing(w *writer, in *driver.InputConfig) {
	w.flags(in.Multiline)
	w.u32(uint32(max(in.MaxLength, 0)))
	w.byte(byte(in.Submit()))
	w.byte(byte(in.ReturnKey))
	w.byte(byte(in.AutoCapitalize))
	w.flags(in.NoAutoCorrect, in.HideCaret)
	hint, ok := autofillHints[in.AutoComplete]
	if !ok {
		hint = "-"
	}
	w.str(hint)
	w.u32(uint32(in.PlaceholderColor))
	w.u32(uint32(in.Caret()))
	w.byte(byte(in.PasteBehavior))
}

// readPaste reads the paste an EventPaste carries in its data: lines of
// "uri\t" and a file: URL, or "file\t", a MIME type, a tab and the
// bytes in base64.
func readPaste(text, data string) *driver.PasteEvent {
	p := &driver.PasteEvent{Text: text}
	for _, line := range strings.Split(data, "\n") {
		kind, rest, _ := strings.Cut(line, "\t")
		switch kind {
		case "uri":
			p.URIs = append(p.URIs, rest)
		case "file":
			mime, b64, _ := strings.Cut(rest, "\t")
			if b, err := base64.StdEncoding.DecodeString(b64); err == nil {
				p.Files = append(p.Files, driver.PastedFile{MIME: mime, Data: b})
			}
		}
	}
	return p
}

// readTouches reads the fingers of an EventTouch the runtime sends: each
// "id x y changed", in points, joined by ";".
func readTouches(s string) []driver.Touch {
	var out []driver.Touch
	for _, f := range strings.Split(s, ";") {
		p := strings.Fields(f)
		if len(p) != 4 {
			continue
		}
		id, _ := strconv.Atoi(p[0])
		x, _ := strconv.ParseFloat(p[1], 32)
		y, _ := strconv.ParseFloat(p[2], 32)
		out = append(out, driver.Touch{ID: id, PagePosition: driver.Point{X: driver.Points(x), Y: driver.Points(y)}, Changed: p[3] == "1"})
	}
	return out
}
