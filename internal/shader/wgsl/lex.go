package wgsl

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tokKind is what a token is.
type tokKind int

const (
	tEOF tokKind = iota
	tIdent
	tInt   // an integer literal: text holds it, without its suffix
	tFloat // a floating-point literal
	tPunct // an operator or a separator
)

// lexeme is one of the source's tokens, at a byte offset.
type lexeme struct {
	kind tokKind
	text string
	// suffix is a number's: "", "i", "u", "f" or "h".
	suffix string
	pos    int
}

// lex splits src into tokens, skipping blanks and comments; block comments
// nest, as WGSL's do.
func lex(src string) ([]lexeme, error) {
	var out []lexeme
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f':
			i++
			continue
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case strings.HasPrefix(src[i:], "/*"):
			start, depth := i, 0
			for i < len(src) {
				switch {
				case strings.HasPrefix(src[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(src[i:], "*/"):
					depth--
					i += 2
				default:
					i++
				}
				if depth == 0 {
					break // the comment ends
				}
			}
			if depth != 0 {
				return nil, errorAt(start, "the block comment is not closed")
			}
			continue
		case c >= '0' && c <= '9' || c == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9':
			t, n, err := lexNumber(src[i:])
			if err != nil {
				return nil, errorAt(i, "%s", err)
			}
			t.pos = i
			out = append(out, t)
			i += n
			continue
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= utf8.RuneSelf:
			start := i
			for i < len(src) {
				r, n := utf8.DecodeRuneInString(src[i:])
				if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					break
				}
				i += n
			}
			if i == start {
				return nil, errorAt(i, "unexpected character %q", src[i])
			}
			out = append(out, lexeme{kind: tIdent, text: src[start:i], pos: start})
			continue
		}
		if p := punct(src[i:]); p != "" {
			out = append(out, lexeme{kind: tPunct, text: p, pos: i})
			i += len(p)
			continue
		}
		r, _ := utf8.DecodeRuneInString(src[i:])
		return nil, errorAt(i, "unexpected character %q", r)
	}
	return append(out, lexeme{kind: tEOF, pos: len(src)}), nil
}

// puncts are WGSL's operators and separators, longest first. `>>` and
// `>=` are split where a template list closes (parser.closeTemplate).
var puncts = []string{
	"<<=", ">>=",
	"->", "&&", "||", "==", "!=", "<=", ">=", "<<", ">>", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "++", "--",
	"(", ")", "{", "}", "[", "]", "<", ">", ",", ";", ":", ".", "@", "=", "+", "-", "*", "/", "%", "&", "|", "^", "!", "~",
}

func punct(s string) string {
	for _, p := range puncts {
		if strings.HasPrefix(s, p) {
			return p
		}
	}
	return ""
}

// lexNumber reads the number s starts with: decimal or hexadecimal, an
// integer or a float, with WGSL's suffixes.
func lexNumber(s string) (lexeme, int, error) {
	i := 0
	isHex := strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")
	digit := func(c byte) bool { return c >= '0' && c <= '9' }
	if isHex {
		i = 2
		digit = func(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
	}
	float := false
	for i < len(s) && digit(s[i]) {
		i++
	}
	if i < len(s) && s[i] == '.' {
		float = true
		i++
		for i < len(s) && digit(s[i]) {
			i++
		}
	}
	exp := byte('e')
	if isHex {
		exp = 'p'
	}
	if i < len(s) && (s[i] == exp || s[i] == exp-'a'+'A') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			float = true
			i = j
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
		}
	}
	text := s[:i]
	suffix := ""
	if i < len(s) {
		switch s[i] {
		case 'i', 'u':
			if !float {
				suffix = string(s[i])
				i++
			}
		case 'f', 'h':
			suffix = string(s[i])
			i++
			float = true
		}
	}
	if i < len(s) && (s[i] == '_' || s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z' || s[i] >= '0' && s[i] <= '9') {
		return lexeme{}, 0, fmt.Errorf("the number %s is not valid", s[:i+1])
	}
	if len(text) > 1 && text[0] == '0' && !isHex && !float && digit(text[1]) {
		return lexeme{}, 0, fmt.Errorf("the integer %s has a leading zero", text)
	}
	kind := tInt
	if float {
		kind = tFloat
	}
	return lexeme{kind: kind, text: text, suffix: suffix}, i, nil
}
