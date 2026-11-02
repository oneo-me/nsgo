package main

import (
	"fmt"
	"strconv"
	"strings"
)

// A message in ICU MessageFormat, as ARB files, Android and Format.JS
// write them: text, {arg}, {arg, number[, integer|percent]}, and plural,
// selectordinal and select with their cases, # and =N.

// msgPart is one part of a message: msgText, msgArg, msgPound or
// msgChoice.
type msgPart any

type msgText string

// msgArg is {name} or {name, number[, style]}.
type msgArg struct {
	name, kind, style string
}

// msgPound is # in a plural case: the number counted.
type msgPound struct{}

// msgChoice is {name, plural|selectordinal|select, ...}.
type msgChoice struct {
	name, kind string
	offset     float64
	cases      []msgCase
}

// msgCase is a choice's case: "=0", "one", "female" or "other".
type msgCase struct {
	key   string
	parts []msgPart
}

type msgParser struct {
	src string
	pos int
}

// parseMessage parses an ICU message.
func parseMessage(src string) ([]msgPart, error) {
	p := &msgParser{src: src}
	parts, err := p.parts(false)
	if err != nil {
		return nil, err
	}
	if p.pos < len(src) {
		return nil, p.errorf("unexpected }")
	}
	return parts, nil
}

func (p *msgParser) errorf(format string, args ...any) error {
	return fmt.Errorf("at %d: %s", p.pos+1, fmt.Sprintf(format, args...))
}

// parts reads text and arguments up to an unmatched } or the end; in a
// plural case # is the number.
func (p *msgParser) parts(inPlural bool) ([]msgPart, error) {
	var parts []msgPart
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			parts = append(parts, msgText(text.String()))
			text.Reset()
		}
	}
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == '\'':
			p.quoted(&text, inPlural)
		case c == '{':
			flush()
			part, err := p.argument()
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case c == '}':
			flush()
			return parts, nil
		case c == '#' && inPlural:
			flush()
			parts = append(parts, msgPound{})
			p.pos++
		default:
			text.WriteByte(c)
			p.pos++
		}
	}
	flush()
	return parts, nil
}

// quoted reads an apostrophe: two say one, and one before {, } or a
// plural's # quotes up to the next apostrophe; any other is itself.
func (p *msgParser) quoted(text *strings.Builder, inPlural bool) {
	p.pos++
	if p.pos < len(p.src) && p.src[p.pos] == '\'' {
		text.WriteByte('\'')
		p.pos++
		return
	}
	if p.pos >= len(p.src) || !(p.src[p.pos] == '{' || p.src[p.pos] == '}' || p.src[p.pos] == '#' && inPlural) {
		text.WriteByte('\'')
		return
	}
	for p.pos < len(p.src) {
		if p.src[p.pos] == '\'' {
			if p.pos+1 < len(p.src) && p.src[p.pos+1] == '\'' {
				text.WriteByte('\'')
				p.pos += 2
				continue
			}
			p.pos++
			return
		}
		text.WriteByte(p.src[p.pos])
		p.pos++
	}
}

func (p *msgParser) space() {
	for p.pos < len(p.src) && strings.IndexByte(" \t\n\r", p.src[p.pos]) >= 0 {
		p.pos++
	}
}

// word reads a name, a keyword or a case's selector.
func (p *msgParser) word() string {
	p.space()
	start := p.pos
	for p.pos < len(p.src) && strings.IndexByte("{}, \t\n\r'#", p.src[p.pos]) < 0 {
		p.pos++
	}
	return p.src[start:p.pos]
}

func (p *msgParser) expect(c byte) error {
	p.space()
	if p.pos >= len(p.src) || p.src[p.pos] != c {
		return p.errorf("want %q", c)
	}
	p.pos++
	return nil
}

func (p *msgParser) argument() (msgPart, error) {
	p.pos++ // {
	name := p.word()
	if !validArgName(name) {
		return nil, p.errorf("bad argument name %q", name)
	}
	p.space()
	if p.pos < len(p.src) && p.src[p.pos] == '}' {
		p.pos++
		return msgArg{name: name}, nil
	}
	if err := p.expect(','); err != nil {
		return nil, err
	}
	kind := p.word()
	switch kind {
	case "number":
		style := ""
		p.space()
		if p.pos < len(p.src) && p.src[p.pos] == ',' {
			p.pos++
			p.space()
			start := p.pos
			for p.pos < len(p.src) && p.src[p.pos] != '}' {
				p.pos++
			}
			style = strings.TrimSpace(p.src[start:p.pos])
			if style != "integer" && style != "percent" {
				return nil, p.errorf("number style %q is not supported: use integer or percent", style)
			}
		}
		return msgArg{name: name, kind: kind, style: style}, p.expect('}')
	case "plural", "selectordinal", "select":
		return p.choice(name, kind)
	case "date", "time":
		return nil, p.errorf("%s arguments are not supported yet", kind)
	}
	return nil, p.errorf("unknown argument type %q", kind)
}

func (p *msgParser) choice(name, kind string) (msgPart, error) {
	if err := p.expect(','); err != nil {
		return nil, err
	}
	ch := msgChoice{name: name, kind: kind}
	seen := map[string]bool{}
	for {
		p.space()
		if p.pos < len(p.src) && p.src[p.pos] == '}' {
			p.pos++
			break
		}
		key := p.word()
		if key == "" {
			return nil, p.errorf("want a case")
		}
		if off, ok := strings.CutPrefix(key, "offset:"); ok && kind == "plural" && len(ch.cases) == 0 {
			if off == "" {
				off = p.word()
			}
			n, err := strconv.ParseFloat(off, 64)
			if err != nil {
				return nil, p.errorf("bad offset %q", off)
			}
			ch.offset = n
			continue
		}
		if err := checkCase(kind, key); err != nil {
			return nil, p.errorf("%v", err)
		}
		if seen[key] {
			return nil, p.errorf("case %s twice", key)
		}
		seen[key] = true
		if err := p.expect('{'); err != nil {
			return nil, err
		}
		parts, err := p.parts(kind != "select")
		if err != nil {
			return nil, err
		}
		if err := p.expect('}'); err != nil {
			return nil, err
		}
		ch.cases = append(ch.cases, msgCase{key: key, parts: parts})
	}
	if !seen["other"] {
		return nil, p.errorf("{%s, %s} has no other case", name, kind)
	}
	return ch, nil
}

// pluralKeys are CLDR's plural categories.
var pluralKeys = []string{"zero", "one", "two", "few", "many", "other"}

func checkCase(kind, key string) error {
	if kind == "select" {
		if !validArgName(key) {
			return fmt.Errorf("bad case %q", key)
		}
		return nil
	}
	if n, ok := strings.CutPrefix(key, "="); ok {
		if _, err := strconv.ParseFloat(n, 64); err != nil {
			return fmt.Errorf("bad case %q", key)
		}
		return nil
	}
	for _, k := range pluralKeys {
		if key == k {
			return nil
		}
	}
	return fmt.Errorf("case %q is not a plural category (%s) or =N", key, strings.Join(pluralKeys, ", "))
}

// validArgName reports whether s names an argument: an identifier, or a
// position ("0").
func validArgName(s string) bool {
	if _, err := strconv.ParseUint(s, 10, 32); err == nil {
		return true
	}
	return validIdent(s)
}

// validIdent reports whether s is a letter or _, then letters, digits
// and _.
func validIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if !(c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || i > 0 && '0' <= c && c <= '9') {
			return false
		}
	}
	return true
}

// argUse is how a message uses an argument.
type argUse uint8

const (
	useText    argUse = 1 << iota // {name}
	useNumber                     // {name, number} or counted by plural or selectordinal
	useSelect                     // {name, select}
	usePercent                    // {name, number, percent}, a fraction
)

// messageArgs returns the message's arguments in the order they first
// appear, with how it uses each.
func messageArgs(parts []msgPart) ([]string, map[string]argUse) {
	var order []string
	uses := map[string]argUse{}
	var walk func([]msgPart)
	add := func(name string, u argUse) {
		if _, ok := uses[name]; !ok {
			order = append(order, name)
		}
		uses[name] |= u
	}
	walk = func(parts []msgPart) {
		for _, part := range parts {
			switch part := part.(type) {
			case msgArg:
				if part.style == "percent" {
					add(part.name, useNumber|usePercent)
				} else if part.kind == "number" {
					add(part.name, useNumber)
				} else {
					add(part.name, useText)
				}
			case msgChoice:
				if part.kind == "select" {
					add(part.name, useSelect)
				} else {
					add(part.name, useNumber)
				}
				for _, c := range part.cases {
					walk(c.parts)
				}
			}
		}
	}
	walk(parts)
	return order, uses
}

// pluralCases returns the categories each of a message's choices of kind
// (plural or selectordinal) has a case for.
func pluralCases(parts []msgPart, kind string) [][]string {
	var out [][]string
	var walk func([]msgPart)
	walk = func(parts []msgPart) {
		for _, part := range parts {
			if ch, ok := part.(msgChoice); ok {
				if ch.kind == kind {
					var keys []string
					for _, c := range ch.cases {
						if !strings.HasPrefix(c.key, "=") {
							keys = append(keys, c.key)
						}
					}
					out = append(out, keys)
				}
				for _, c := range ch.cases {
					walk(c.parts)
				}
			}
		}
	}
	walk(parts)
	return out
}
