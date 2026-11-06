package driver

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// emojiPresentation is Unicode's Emoji_Presentation: what draws as a color
// emoji by default.
var emojiPresentation = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x231A, 0x231B, 1}, {0x23E9, 0x23EC, 1}, {0x23F0, 0x23F0, 1}, {0x23F3, 0x23F3, 1}, {0x25FD, 0x25FE, 1},
		{0x2614, 0x2615, 1}, {0x2648, 0x2653, 1}, {0x267F, 0x267F, 1}, {0x2693, 0x2693, 1}, {0x26A1, 0x26A1, 1},
		{0x26AA, 0x26AB, 1}, {0x26BD, 0x26BE, 1}, {0x26C4, 0x26C5, 1}, {0x26CE, 0x26CE, 1}, {0x26D4, 0x26D4, 1},
		{0x26EA, 0x26EA, 1}, {0x26F2, 0x26F3, 1}, {0x26F5, 0x26F5, 1}, {0x26FA, 0x26FA, 1}, {0x26FD, 0x26FD, 1},
		{0x2705, 0x2705, 1}, {0x270A, 0x270B, 1}, {0x2728, 0x2728, 1}, {0x274C, 0x274C, 1}, {0x274E, 0x274E, 1},
		{0x2753, 0x2755, 1}, {0x2757, 0x2757, 1}, {0x2795, 0x2797, 1}, {0x27B0, 0x27B0, 1}, {0x27BF, 0x27BF, 1},
		{0x2B1B, 0x2B1C, 1}, {0x2B50, 0x2B50, 1}, {0x2B55, 0x2B55, 1},
	},
	R32: []unicode.Range32{
		{0x1F004, 0x1F004, 1}, {0x1F0CF, 0x1F0CF, 1}, {0x1F18E, 0x1F18E, 1}, {0x1F191, 0x1F19A, 1}, {0x1F1E6, 0x1F1FF, 1},
		{0x1F201, 0x1F202, 1}, {0x1F21A, 0x1F21A, 1}, {0x1F22F, 0x1F22F, 1}, {0x1F232, 0x1F236, 1}, {0x1F238, 0x1F23A, 1},
		{0x1F250, 0x1F251, 1}, {0x1F300, 0x1F320, 1}, {0x1F32D, 0x1F335, 1}, {0x1F337, 0x1F37C, 1}, {0x1F37E, 0x1F393, 1},
		{0x1F3A0, 0x1F3CA, 1}, {0x1F3CF, 0x1F3D3, 1}, {0x1F3E0, 0x1F3F0, 1}, {0x1F3F4, 0x1F3F4, 1}, {0x1F3F8, 0x1F43E, 1},
		{0x1F440, 0x1F440, 1}, {0x1F442, 0x1F4FC, 1}, {0x1F4FF, 0x1F53D, 1}, {0x1F54B, 0x1F54E, 1}, {0x1F550, 0x1F567, 1},
		{0x1F57A, 0x1F57A, 1}, {0x1F595, 0x1F596, 1}, {0x1F5A4, 0x1F5A4, 1}, {0x1F5FB, 0x1F64F, 1}, {0x1F680, 0x1F6C5, 1},
		{0x1F6CC, 0x1F6CC, 1}, {0x1F6D0, 0x1F6D2, 1}, {0x1F6D5, 0x1F6D7, 1}, {0x1F6DC, 0x1F6DF, 1}, {0x1F6EB, 0x1F6EC, 1},
		{0x1F6F4, 0x1F6FC, 1}, {0x1F7E0, 0x1F7EB, 1}, {0x1F7F0, 0x1F7F0, 1}, {0x1F90C, 0x1F93A, 1}, {0x1F93C, 0x1F945, 1},
		{0x1F947, 0x1F9FF, 1}, {0x1FA70, 0x1FA7C, 1}, {0x1FA80, 0x1FA89, 1}, {0x1FA8F, 0x1FAC6, 1}, {0x1FACE, 0x1FADC, 1},
		{0x1FADF, 0x1FAE9, 1}, {0x1FAF0, 0x1FAF8, 1},
	},
}

const (
	textPresentation  = 0xFE0E
	emojiVariation    = 0xFE0F
	zeroWidthJoiner   = 0x200D
	keycap            = 0x20E3
	firstRegional     = 0x1F1E6
	lastRegional      = 0x1F1FF
	firstSkinTone     = 0x1F3FB
	lastSkinTone      = 0x1F3FF
	firstTag, lastTag = 0xE0020, 0xE007F
)

// emojiAt returns the length of the color emoji s starts with, 0 for none:
// a character of emoji presentation or one asked for with U+FE0F, with the
// modifiers, tags and joined characters that make one emoji of it, unless
// U+FE0E asks for text.
func emojiAt(s string) int {
	r, n := utf8.DecodeRuneInString(s)
	asked := false
	flag := r >= firstRegional && r <= lastRegional
	if flag {
		if next, m := utf8.DecodeRuneInString(s[n:]); next >= firstRegional && next <= lastRegional {
			n += m
		}
	}
	for n < len(s) {
		next, m := utf8.DecodeRuneInString(s[n:])
		switch {
		case next == textPresentation:
			return 0
		case next == emojiVariation:
			asked = true
		case next >= firstSkinTone && next <= lastSkinTone, next >= firstTag && next <= lastTag, next == keycap:
			asked = asked || next == keycap
		case next == zeroWidthJoiner:
			if _, j := utf8.DecodeRuneInString(s[n+m:]); j > 0 && n+m < len(s) {
				m += j
			}
		default:
			if asked || unicode.Is(emojiPresentation, r) {
				return n
			}
			return 0
		}
		n += m
	}
	if asked || unicode.Is(emojiPresentation, r) {
		return n
	}
	return 0
}

// UprightEmoji gives the color emoji of t's italic runs spans of their own
// that are not italic: emoji fonts have no italic face, and Chrome and
// Android would slant them where WebKit and CoreText keep them upright, as
// every platform does here.
func UprightEmoji(t *AttributedText) {
	if t.Text == "" {
		return
	}
	italic := t.Style.Font.Italic
	for i := range t.Spans {
		italic = italic || t.Spans[i].Style.Font.Italic
	}
	if !italic || isASCII(t.Text) {
		return
	}
	spans := t.Spans
	if len(spans) == 0 {
		spans = []TextSpan{{End: len(t.Text), Style: t.Style}}
	}
	var out []TextSpan
	for _, sp := range spans {
		if !sp.Style.Font.Italic {
			out = append(out, sp)
			continue
		}
		run := sp.Start
		for at := sp.Start; at < sp.End; {
			n := emojiAt(t.Text[at:sp.End])
			if n == 0 {
				_, size := utf8.DecodeRuneInString(t.Text[at:sp.End])
				at += size
				continue
			}
			if at > run {
				part := sp
				part.Start, part.End = run, at
				out = append(out, part)
			}
			part := sp
			part.Start, part.End = at, at+n
			part.Style.Font.Italic = false
			out = append(out, part)
			at += n
			run = at
		}
		if run < sp.End {
			part := sp
			part.Start = run
			out = append(out, part)
		}
	}
	if len(t.Spans) > 0 || len(out) > 1 || out[0].Style != t.Style {
		t.Spans = out
	}
}

func isASCII(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r >= utf8.RuneSelf }) < 0
}
