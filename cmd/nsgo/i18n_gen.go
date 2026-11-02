package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/token"
	"math"
	"path"
	"slices"
	"strconv"
	"strings"
)

// generate writes catalog_gen.go for c: a function per message, which
// says it in the language the app shows, and a method of Messages for a
// language of its choice. Messages with arguments are compiled to Go per
// language, those without are tables of text, so the app neither reads
// nor parses catalogs.
func (c *catalog) generate() ([]byte, error) {
	g := &goWriter{imports: map[string]string{
		"github.com/oneo-me/nsgo/core": "core",
		"github.com/oneo-me/nsgo/i18n": "i18n",
	}}
	langs := c.languages()
	src := c.files[c.source]

	var body bytes.Buffer
	g.out = &body
	g.p("// Source is the language the messages are written in.")
	g.p("const Source = %q", c.source)
	g.p("")
	g.p("// Catalog holds the messages' languages: %s.", describeLanguages(langs))
	g.p("var Catalog = i18n.NewCatalog(%s)", quoteList(langs))
	g.p("")
	if c.app {
		g.p("// The catalog's languages are the app's: those it shows.")
		g.p("func init() {")
		g.p("\ti18n.SetApp(Catalog)")
		g.p("}")
		g.p("")
	}
	if err := c.writeSupplies(g); err != nil {
		return nil, err
	}
	g.p("// Messages say the messages in one language.")
	g.p("type Messages struct{ lang string }")
	g.p("")
	g.p("// In returns the messages in the catalog's language that suits locale")
	g.p("// best, as zh-Hant suits zh-TW, else in the source language.")
	g.p("func In(locale string) Messages { return Messages{Catalog.Match(locale)} }")
	g.p("")
	g.p("// For returns the messages in the language the app shows under cx")
	g.p("// (i18n.Language); a binding that says them follows it as it changes.")
	g.p("func For(cx *core.Scope) Messages { return In(i18n.Language(cx)) }")
	g.p("")
	g.p("// Translation is a language's messages another package supplies; a nil")
	g.p("// field leaves the message to the catalog.")
	g.p("type Translation struct {")
	for _, key := range c.keys {
		g.p("\t%s func(%s) string", exportName(key), signature(c.params[key]))
	}
	g.p("}")
	g.p("")
	// Messages without arguments are constants: a table per language,
	// which one method looks up, rather than code per message.
	index := map[string]int{}
	var constants []string
	for _, key := range c.keys {
		if len(c.params[key]) == 0 {
			index[key] = len(constants)
			constants = append(constants, key)
		}
	}
	g.p("var supplied = map[string]Translation{}")
	g.p("")
	if len(constants) > 0 {
		g.p("// texts are the messages without arguments, by language, in the")
		g.p("// source's order; \"\" where a language lacks one.")
		g.p("var texts = map[string][]string{")
		for _, lang := range langs {
			g.p("\t%q: {", lang)
			for _, key := range constants {
				text := ""
				if m := c.files[lang].msgs[key]; m != nil {
					text = constantText(m.parts)
				}
				g.p("\t\t%s,", strconv.Quote(text))
			}
			g.p("\t},")
		}
		g.p("}")
		g.p("")
		g.p("// suppliedTexts are the messages without arguments other packages")
		g.p("// supply, as texts are.")
		g.p("var suppliedTexts = map[string][]string{}")
		g.p("")
		g.p("// text says the message without arguments at i in the language of msgs,")
		g.p("// else of the languages it falls back to, else of the source.")
		g.p("func (msgs Messages) text(i int) string {")
		g.p("\tfor lang := msgs.lang; ; lang = i18n.Parent(lang) {")
		g.p("\t\tif t := suppliedTexts[lang]; t != nil && t[i] != \"\" {")
		g.p("\t\t\treturn t[i]")
		g.p("\t\t}")
		g.p("\t\tif lang == \"\" {")
		g.p("\t\t\treturn texts[Source][i]")
		g.p("\t\t}")
		g.p("\t\tif t := texts[lang]; t != nil && t[i] != \"\" {")
		g.p("\t\t\treturn t[i]")
		g.p("\t\t}")
		g.p("\t}")
		g.p("}")
		g.p("")
	}
	g.p("// Supply adds the messages t has in lang to the catalog, over its own.")
	g.p("// nsgo writes the calls, in the package that supplies them, which runs")
	g.p("// them as it initializes.")
	g.p("func Supply(lang string, t Translation) {")
	g.p("\ts := supplied[lang]")
	if len(constants) > 0 {
		g.p("\tst := suppliedTexts[lang]")
		g.p("\tif st == nil {")
		g.p("\t\tst = make([]string, %d)", len(constants))
		g.p("\t\tsuppliedTexts[lang] = st")
		g.p("\t}")
	}
	for _, key := range c.keys {
		name := exportName(key)
		g.p("\tif t.%s != nil {", name)
		if i, ok := index[key]; ok {
			g.p("\t\tst[%d] = t.%s()", i, name)
		} else {
			g.p("\t\ts.%s = t.%s", name, name)
		}
		g.p("\t}")
	}
	g.p("\tsupplied[lang] = s")
	g.p("\tCatalog.Add(lang)")
	g.p("}")
	for _, key := range c.keys {
		m, params := src.msgs[key], c.params[key]
		name := exportName(key)
		g.p("")
		g.p("// %s says %s", name, strconv.Quote(m.text))
		g.p("// in the language the app shows under cx.")
		if m.desc != "" {
			g.p("//")
			for _, line := range strings.Split(strings.TrimSpace(m.desc), "\n") {
				g.p("// %s", line)
			}
		}
		cxParams := "cx *core.Scope"
		if len(params) > 0 {
			cxParams += ", " + signature(params)
		}
		g.p("func %s(%s) string { return For(cx).%s(%s) }", name, cxParams, name, arguments(params))
		g.p("")
		g.p("// %s says %s", name, strconv.Quote(m.text))
		g.p("// in the language of msgs.")
		if i, ok := index[key]; ok {
			g.p("func (msgs Messages) %s() string { return msgs.text(%d) }", name, i)
			continue
		}
		g.p("func (msgs Messages) %s(%s) string {", name, signature(params))
		g.p("\tfor lang := msgs.lang; ; lang = i18n.Parent(lang) {")
		g.p("\t\tif f := supplied[lang].%s; f != nil {", name)
		g.p("\t\t\treturn f(%s)", arguments(params))
		g.p("\t\t}")
		g.p("\t\tswitch lang {")
		for _, lang := range langs {
			tm := c.files[lang].msgs[key]
			if tm == nil {
				continue
			}
			if lang == c.source {
				g.p("\t\tcase \"\", %q:", lang)
			} else {
				g.p("\t\tcase %q:", lang)
			}
			g.say(tm.parts, lang, params, "\t\t\t")
		}
		g.p("\t\t}")
		g.p("\t}")
		g.p("}")
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "%s\n\npackage %s\n\nimport (\n", genHeader, c.pkg)
	paths := make([]string, 0, len(g.imports))
	for p := range g.imports {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		if name := g.imports[p]; name == path.Base(p) {
			fmt.Fprintf(&out, "\t%q\n", p)
		} else {
			fmt.Fprintf(&out, "\t%s %q\n", name, p)
		}
	}
	out.WriteString(")\n\n")
	out.Write(body.Bytes())
	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("%s: generated code does not parse: %w", c.dir, err)
	}
	return formatted, nil
}

// writeSupplies writes an init supplying the other packages' catalogs
// c's files supply, by import path and language.
func (c *catalog) writeSupplies(g *goWriter) error {
	type entry struct {
		lang string
		s    *supply
	}
	var all []entry
	for _, lang := range c.languages() {
		for _, s := range c.files[lang].supplies {
			all = append(all, entry{lang, s})
		}
	}
	if len(all) == 0 {
		return nil
	}
	slices.SortStableFunc(all, func(a, b entry) int { return strings.Compare(a.s.importPath, b.s.importPath) })
	g.p("// The messages this package supplies to others.")
	g.p("func init() {")
	for _, e := range all {
		t := e.s.target
		if t == nil {
			return fmt.Errorf("%s: %s is not resolved", c.dir, e.s.importPath)
		}
		pkg := g.use(e.s.importPath)
		g.p("\t%s.Supply(%q, %s.Translation{", pkg, e.lang, pkg)
		for _, key := range e.s.keys {
			params := t.params[key]
			g.p("\t\t%s: func(%s) string {", exportName(key), signature(params))
			g.say(e.s.msgs[key].parts, e.lang, params, "\t\t\t")
			g.p("\t\t},")
		}
		g.p("\t})")
	}
	g.p("}")
	g.p("")
	return nil
}

// goWriter writes Go code, recording the packages it uses.
type goWriter struct {
	out     *bytes.Buffer
	imports map[string]string // import path → name
}

func (g *goWriter) p(format string, args ...any) {
	fmt.Fprintf(g.out, format, args...)
	g.out.WriteByte('\n')
}

// use imports a package, named after its last two elements
// (baselocales), and returns its name.
func (g *goWriter) use(importPath string) string {
	if name, ok := g.imports[importPath]; ok {
		return name
	}
	elems := strings.Split(importPath, "/")
	if len(elems) > 2 {
		elems = elems[len(elems)-2:]
	}
	var b strings.Builder
	for _, c := range strings.ToLower(strings.Join(elems, "")) {
		if 'a' <= c && c <= 'z' || '0' <= c && c <= '9' && b.Len() > 0 {
			b.WriteRune(c)
		}
	}
	base, name := b.String(), b.String()
	taken := func(n string) bool {
		if token.IsKeyword(n) || reservedLocal[n] {
			return true
		}
		for _, used := range g.imports {
			if used == n {
				return true
			}
		}
		return false
	}
	for i := 2; name == "" || taken(name); i++ {
		name = base + strconv.Itoa(i)
	}
	g.imports[importPath] = name
	return name
}

// say writes the statements that return parts, a message in lang.
func (g *goWriter) say(parts []msgPart, lang string, params []param, indent string) {
	if !branches(parts) {
		var terms []string
		for _, part := range parts {
			terms = append(terms, g.term(part, lang, params, nil))
		}
		if len(terms) == 0 {
			terms = []string{`""`}
		}
		g.p("%sreturn %s", indent, strings.Join(terms, " + "))
		return
	}
	g.imports["strings"] = "strings"
	g.p("%svar b strings.Builder", indent)
	g.write(parts, lang, params, nil, indent)
	g.p("%sreturn b.String()", indent)
}

// counted is the number # says in a plural case: the argument less the
// offset.
type counted struct {
	expr   string
	offset float64
}

// write writes the statements adding parts to b.
func (g *goWriter) write(parts []msgPart, lang string, params []param, pound *counted, indent string) {
	for _, part := range parts {
		ch, ok := part.(msgChoice)
		if !ok {
			g.p("%sb.WriteString(%s)", indent, g.term(part, lang, params, pound))
			continue
		}
		p := findParam(params, ch.name)
		v := goName(p.name)
		if ch.kind == "select" {
			g.p("%sswitch %s {", indent, v)
			for _, c := range ch.cases {
				if c.key != "other" {
					g.p("%scase %q:", indent, c.key)
					g.write(c.parts, lang, params, pound, indent+"\t")
				}
			}
			g.p("%sdefault:", indent)
			g.write(caseParts(ch, "other"), lang, params, pound, indent+"\t")
			g.p("%s}", indent)
			continue
		}
		num := v
		if p.typ == "int" {
			num = "float64(" + v + ")"
		}
		inner := &counted{expr: num, offset: ch.offset}
		rule := "Cardinal"
		if ch.kind == "selectordinal" {
			rule = "Ordinal"
		}
		var exact []msgCase
		for _, c := range ch.cases {
			if strings.HasPrefix(c.key, "=") {
				exact = append(exact, c)
			}
		}
		in := indent
		if len(exact) > 0 {
			g.p("%sswitch {", indent)
			for _, c := range exact {
				n, _ := strconv.ParseFloat(c.key[1:], 64)
				lhs := v
				if p.typ == "int" && n != math.Trunc(n) {
					lhs = num
				}
				g.p("%scase %s == %s:", indent, lhs, strconv.FormatFloat(n, 'f', -1, 64))
				g.write(c.parts, lang, params, inner, indent+"\t")
			}
			g.p("%sdefault:", indent)
			in = indent + "\t"
		}
		g.p("%sswitch i18n.%s(%q, %s) {", in, rule, lang, inner.value())
		for _, c := range ch.cases {
			if c.key != "other" && !strings.HasPrefix(c.key, "=") {
				g.p("%scase i18n.%s:", in, strings.ToUpper(c.key[:1])+c.key[1:])
				g.write(c.parts, lang, params, inner, in+"\t")
			}
		}
		g.p("%sdefault:", in)
		g.write(caseParts(ch, "other"), lang, params, inner, in+"\t")
		g.p("%s}", in)
		if len(exact) > 0 {
			g.p("%s}", indent)
		}
	}
}

func (c *counted) value() string {
	if c.offset == 0 {
		return c.expr
	}
	return c.expr + " - " + strconv.FormatFloat(c.offset, 'f', -1, 64)
}

// term is the expression of a part that is not a choice.
func (g *goWriter) term(part msgPart, lang string, params []param, pound *counted) string {
	switch part := part.(type) {
	case msgText:
		return strconv.Quote(string(part))
	case msgPound:
		return fmt.Sprintf("i18n.FormatNumber(%q, %s)", lang, pound.value())
	case msgArg:
		p := findParam(params, part.name)
		v := goName(p.name)
		num := v
		if p.typ == "int" {
			num = "float64(" + v + ")"
		}
		switch {
		case part.style == "integer":
			return fmt.Sprintf("i18n.FormatInteger(%q, %s)", lang, num)
		case part.style == "percent":
			return fmt.Sprintf("i18n.FormatPercent(%q, %s)", lang, num)
		case p.typ == "int" || p.typ == "float64":
			return fmt.Sprintf("i18n.FormatNumber(%q, %s)", lang, num)
		case p.typ == "any":
			g.imports["fmt"] = "fmt"
			return "fmt.Sprint(" + v + ")"
		}
		return v
	}
	panic(fmt.Sprintf("nsgo: no term for %T", part))
}

func caseParts(ch msgChoice, key string) []msgPart {
	for _, c := range ch.cases {
		if c.key == key {
			return c.parts
		}
	}
	return nil
}

// constantText is a message without arguments: its text.
func constantText(parts []msgPart) string {
	var b strings.Builder
	for _, p := range parts {
		if t, ok := p.(msgText); ok {
			b.WriteString(string(t))
		}
	}
	return b.String()
}

// branches reports whether a message has a choice.
func branches(parts []msgPart) bool {
	return slices.ContainsFunc(parts, func(p msgPart) bool { _, ok := p.(msgChoice); return ok })
}

func findParam(params []param, name string) param {
	for _, p := range params {
		if p.name == name {
			return p
		}
	}
	panic("nsgo: message argument " + name + " has no parameter")
}

// reservedLocal are the names the generated functions use besides their
// parameters.
var reservedLocal = map[string]bool{
	"cx": true, "msgs": true, "lang": true, "b": true, "f": true, "s": true, "t": true,
	"supplied": true, "core": true, "i18n": true, "strings": true, "fmt": true, "math": true,
}

// predeclared are Go's predeclared identifiers, which a parameter must not
// hide.
var predeclared = map[string]bool{
	"any": true, "bool": true, "byte": true, "comparable": true, "complex64": true, "complex128": true,
	"error": true, "float32": true, "float64": true, "int": true, "int8": true, "int16": true, "int32": true,
	"int64": true, "rune": true, "string": true, "uint": true, "uint8": true, "uint16": true, "uint32": true,
	"uint64": true, "uintptr": true, "true": true, "false": true, "iota": true, "nil": true, "append": true,
	"cap": true, "clear": true, "close": true, "complex": true, "copy": true, "delete": true, "imag": true,
	"len": true, "make": true, "max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true,
}

// goName is a parameter's name in Go: arg0 for {0}, and type_ for a
// keyword or a name the code uses.
func goName(name string) string {
	if _, err := strconv.Atoi(name); err == nil {
		return "arg" + name
	}
	if token.IsKeyword(name) || predeclared[name] || reservedLocal[name] {
		return name + "_"
	}
	return name
}

func signature(params []param) string {
	var parts []string
	for _, p := range params {
		parts = append(parts, goName(p.name)+" "+p.typ)
	}
	return strings.Join(parts, ", ")
}

func arguments(params []param) string {
	var parts []string
	for _, p := range params {
		parts = append(parts, goName(p.name))
	}
	return strings.Join(parts, ", ")
}

func quoteList(items []string) string {
	var parts []string
	for _, s := range items {
		parts = append(parts, strconv.Quote(s))
	}
	return strings.Join(parts, ", ")
}

// describeLanguages says a catalog's languages for its comment.
func describeLanguages(langs []string) string {
	s := langs[0] + ", the source"
	if rest := langs[1:]; len(rest) > 0 {
		s += ", " + strings.Join(rest, ", ")
	}
	return s + ", and those other packages supply"
}
