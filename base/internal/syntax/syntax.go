// Package syntax is what code highlighting in base shares: the kinds of
// tokens, chroma's token types mapped onto them and NSH's palette,
// Allsum's, in light and dark.
package syntax

import (
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/nsgo-dev/nsgo/core"
)

// Kind is a kind of token in code, as NSH's palette names them.
type Kind uint8

// The kinds; Kinds counts them.
const (
	Plain Kind = iota
	Comment
	Keyword
	String
	Number
	Title
	Attr
	Built
	Deletion
	Bold
	Italic
	Kinds
)

func color(c uint32) core.Style { return core.Style{Color: core.RGB(c)} }

var (
	Light = [Kinds]core.Style{
		Comment: {Color: core.RGB(0x6e7781), FontStyle: core.FontStyleItalic},
		Keyword: color(0x8250df), String: color(0x116329), Number: color(0x953800), Title: color(0x0550ae),
		Attr: color(0x7d4e00), Built: color(0x0a7285), Deletion: color(0xcf222e),
		Bold: {FontWeight: core.FontWeightBold}, Italic: {FontStyle: core.FontStyleItalic},
	}
	Dark = [Kinds]core.Style{
		Comment: {Color: core.RGB(0x7d7a72), FontStyle: core.FontStyleItalic},
		Keyword: color(0xc4a7ff), String: color(0x9fd68a), Number: color(0xf5b27a), Title: color(0x82b8ff),
		Attr: color(0xf0c674), Built: color(0x6fd3d8), Deletion: color(0xf87171),
		Bold: {FontWeight: core.FontWeightBold}, Italic: {FontStyle: core.FontStyleItalic},
	}
)

// KindOf maps chroma's token types to the palette's Kinds.
func KindOf(t chroma.TokenType) Kind {
	switch t {
	case chroma.KeywordType, chroma.NameBuiltin, chroma.NameTag:
		return Built
	case chroma.KeywordConstant, chroma.NameConstant:
		return Number
	case chroma.OperatorWord, chroma.GenericHeading, chroma.GenericSubheading:
		return Keyword
	case chroma.NameFunction, chroma.NameClass, chroma.NameException, chroma.NameDecorator, chroma.NameNamespace:
		return Title
	case chroma.NameAttribute, chroma.NameProperty, chroma.NameBuiltinPseudo, chroma.NameVariable,
		chroma.NameVariableInstance, chroma.NameVariableGlobal, chroma.NameVariableClass, chroma.NameLabel:
		return Attr
	case chroma.GenericDeleted:
		return Deletion
	case chroma.GenericInserted:
		return String
	case chroma.GenericStrong:
		return Bold
	case chroma.GenericEmph:
		return Italic
	}
	switch t.Category() {
	case chroma.Comment:
		return Comment
	case chroma.Keyword:
		return Keyword
	case chroma.LiteralString:
		return String
	case chroma.LiteralNumber:
		return Number
	}
	if t.SubCategory() == chroma.LiteralString {
		return String
	}
	if t.SubCategory() == chroma.LiteralNumber {
		return Number
	}
	return Plain
}

// Tok is a run of a line of one kind, in bytes.
type Tok struct {
	Start, End int
	Kind       Kind
}

// Lexer is chroma's lexer for a language's name or a file's path,
// coalesced; nil for none.
func Lexer(name string) chroma.Lexer {
	l := lexers.Get(name)
	if l == nil {
		l = lexers.Match(name)
	}
	if l == nil {
		return nil
	}
	return chroma.Coalesce(l)
}

// LexLines lexes lines with lexer from its root state: the runs of each
// line that are not plain. It takes its time; run it off the engine
// goroutine.
func LexLines(lexer chroma.Lexer, lines []string) [][]Tok {
	out := make([][]Tok, len(lines))
	text := strings.Join(lines, "\n")
	it, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root"}, text)
	if err != nil {
		return out
	}
	line, col := 0, 0
	for t := it(); t != chroma.EOF; t = it() {
		k := KindOf(t.Type)
		v := t.Value
		for v != "" && line < len(out) {
			i := strings.IndexByte(v, '\n')
			part := v
			if i >= 0 {
				part = v[:i]
			}
			if k != Plain && part != "" {
				ts := out[line]
				if n := len(ts); n > 0 && ts[n-1].Kind == k && ts[n-1].End == col {
					ts[n-1].End += len(part)
				} else {
					out[line] = append(ts, Tok{Start: col, End: col + len(part), Kind: k})
				}
			}
			col += len(part)
			if i < 0 {
				break
			}
			line, col, v = line+1, 0, v[i+1:]
		}
	}
	return out
}

// Highlighter is an app's highlighting of a language, in place of
// chroma's, for the Editor and the DiffView alike. It runs off the engine
// goroutine.
type Highlighter interface {
	// Highlight returns the tokens of lines from to to (exclusive) of
	// lines, a snapshot of the whole text without line breaks: one slice
	// per line, its tokens in bytes of the line.
	Highlight(lines []string, from, to int) [][]Token
}

// Token is a run of a line an app's highlighter colors: from byte Start
// to End, as the kind Kind names (comment, keyword, string, number,
// title, attr, built, deletion).
type Token struct {
	Start, End int
	Kind       string
}

// KindNamed is the Kind a name gives, Plain for none: a decoration's or
// an app's token's.
func KindNamed(name string) Kind {
	switch name {
	case "comment":
		return Comment
	case "keyword":
		return Keyword
	case "string":
		return String
	case "number":
		return Number
	case "title", "function", "type":
		return Title
	case "attr", "property", "variable":
		return Attr
	case "built", "builtin":
		return Built
	case "deletion":
		return Deletion
	}
	return Plain
}

// HighlightLines runs h over lines from to to and returns their runs as
// Toks, sorted and clipped to each line.
func HighlightLines(h Highlighter, lines []string, from, to int) [][]Tok {
	got := h.Highlight(lines, from, to)
	out := make([][]Tok, to-from)
	for i := range out {
		if i >= len(got) {
			break
		}
		n := len(lines[from+i])
		for _, t := range got[i] {
			k := KindNamed(t.Kind)
			s, e := max(t.Start, 0), min(t.End, n)
			if k == Plain || s >= e {
				continue
			}
			out[i] = append(out[i], Tok{Start: s, End: e, Kind: k})
		}
		sort.Slice(out[i], func(a, b int) bool { return out[i][a].Start < out[i][b].Start })
	}
	return out
}
