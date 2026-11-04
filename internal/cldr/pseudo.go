package cldr

import (
	"strings"
	"unicode"
)

// The pseudo locales, after Android's: en-XA accents and lengthens the
// source language's messages, ar-XB shows them right to left.
const (
	PseudoAccents = "en-XA"
	PseudoBidi    = "ar-XB"
)

// Marks around what an argument wrote in a message Pseudolocalize
// transforms: private use characters, which messages do not hold.
const (
	ArgStart = ""
	ArgEnd   = ""
)

// accents are the letters en-XA writes for ASCII's.
var accents = strings.NewReplacer(
	"a", "á", "b", "ƀ", "c", "ć", "d", "ď", "e", "é", "f", "ƒ", "g", "ğ", "h", "ĥ", "i", "í",
	"j", "ĵ", "k", "ķ", "l", "ĺ", "m", "ḿ", "n", "ń", "o", "ó", "p", "ṕ", "q", "ʠ", "r", "ŕ",
	"s", "ś", "t", "ť", "u", "ú", "v", "ṽ", "w", "ẃ", "x", "ẍ", "y", "ý", "z", "ź",
	"A", "Á", "B", "Ɓ", "C", "Ć", "D", "Ď", "E", "É", "F", "Ƒ", "G", "Ğ", "H", "Ĥ", "I", "Í",
	"J", "Ĵ", "K", "Ķ", "L", "Ĺ", "M", "Ḿ", "N", "Ń", "O", "Ó", "P", "Ṕ", "Q", "Ǫ", "R", "Ŕ",
	"S", "Ś", "T", "Ť", "U", "Ú", "V", "Ṽ", "W", "Ẃ", "X", "Ẍ", "Y", "Ý", "Z", "Ź",
)

// Pseudolocalize transforms s, a message in the source language whose
// arguments' text is between ArgStart and ArgEnd, as the pseudo locale
// tag shows it, keeping what the arguments wrote. Any other tag only
// drops the marks.
func Pseudolocalize(tag, s string) string {
	var out strings.Builder
	letters := 0
	for s != "" {
		text, rest, marked := strings.Cut(s, ArgStart)
		switch tag {
		case PseudoAccents:
			letters += len([]rune(text))
			out.WriteString(accents.Replace(text))
		case PseudoBidi:
			out.WriteString(mirrorWords(text))
		default:
			out.WriteString(text)
		}
		if !marked {
			break
		}
		arg, after, _ := strings.Cut(rest, ArgEnd)
		if tag == PseudoBidi {
			arg = "‏" + arg + "‏"
		}
		out.WriteString(arg)
		s = after
	}
	if tag != PseudoAccents {
		return out.String()
	}
	// Translations run longer than English: a third more, in words.
	pad := strings.Repeat("~", (letters+2)/3)
	if pad != "" {
		pad = " " + pad
	}
	return "[" + out.String() + pad + "]"
}

// mirrorWords sets each word of s right to left: an override reverses
// its letters and marks keep it in a right-to-left run.
func mirrorWords(s string) string {
	var out strings.Builder
	word := false
	for _, r := range s {
		space := unicode.IsSpace(r)
		switch {
		case !space && !word:
			out.WriteString("‏‮")
			word = true
		case space && word:
			out.WriteString("‬‏")
			word = false
		}
		out.WriteRune(r)
	}
	if word {
		out.WriteString("‬‏")
	}
	return out.String()
}
