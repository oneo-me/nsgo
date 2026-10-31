package main

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/base/editor"
	"github.com/oneo-me/nsgo/core"
)

// A language server for the Editor demo, simulated in the app, as NSH's
// gallery simulates it: what a TypeScript server answers (symbols, hover,
// definitions, completion, inline completion, a linter's diagnostics and
// their quick fixes, document colors), found with a few regular
// expressions over the text. It reads on a goroutine of its own, as a
// real server runs in a process of its own, and each answer comes back a
// little later through Scope.Post, so the demo handles answers that
// arrive after the text changed. Columns are bytes inside and UTF-16
// units in what it answers, as the editor's are.

// lspSymbol is a name declared in the file, a member of one of its types
// or a built-in's member.
type lspSymbol struct {
	name, kind string
	// signature is what hover shows; doc its JSDoc comment.
	signature, doc string
	// rng is where its name is declared; builtin ones have none.
	rng     editor.Range
	builtin bool
	// scope is the lines it is visible on (a function's parameters and
	// locals), everywhere without; body a function's first and last lines.
	scope, body *lineSpan
	// typ names its type: what members follow a "." after it (a
	// function's: what it returns).
	typ string
}

type lineSpan struct{ start, end int }

func (l *lineSpan) has(line int) bool { return l != nil && l.start <= line && line <= l.end }

// lspAction is a code action: edits to apply, or an editor command.
type lspAction struct {
	title   string
	kind    string // quickfix, refactor or source
	edits   []editor.TextEdit
	command editor.Command
}

// lspProblem is a diagnostic and the quick fixes that resolve it.
type lspProblem struct {
	diagnostic editor.Diagnostic
	fixes      []lspAction
}

// lspColor is a color literal of the text.
type lspColor struct {
	rng   editor.Range
	color core.Color
}

// lspHover is what hover shows: Markdown about the word at rng.
type lspHover struct {
	markdown string
	rng      editor.Range
}

// lspTarget is a definition: a name and where it is declared.
type lspTarget struct {
	name string
	rng  editor.Range
}

// lspType is a class or interface of the file and its members.
type lspType struct {
	symbol     *lspSymbol
	members    []*lspSymbol
	start, end int
}

// lspNamed is the symbols of one name, for lookup: the first without a
// scope, and those with one by where their scope starts, with the
// furthest end any of them up to each reaches.
type lspNamed struct {
	global *lspSymbol
	scoped []*lspSymbol
	reach  []int
}

// lspModel is what the server knows of one text.
type lspModel struct {
	lines []string
	// code is lines with strings and comments blanked out, byte for byte.
	code    []string
	symbols []*lspSymbol
	types   map[string]*lspType
	// typeNames are the types in the order the file declares them.
	typeNames []string
	// declarations are the lines that declare a method, whose names are
	// no calls.
	declarations map[int]bool
	byName       map[string]*lspNamed
}

var lspKeywords = strings.Fields(`as async await break case catch class const continue default delete do else export extends
	false finally for function if import in instanceof interface let new null of return static super switch this throw true try
	type typeof undefined var void while yield`)

var lspPrimitives = []string{"string", "number", "boolean", "unknown", "void", "never", "any"}

func builtin(kind, name, signature, doc, typ string) *lspSymbol {
	return &lspSymbol{name: name, kind: kind, signature: signature, doc: doc, typ: typ, builtin: true}
}

func method(name, signature, doc, typ string) *lspSymbol {
	return builtin("method", name, signature, doc, typ)
}

// lspGlobal is a global the server knows: what it is and its members.
type lspGlobal struct {
	signature, doc string
	members        []*lspSymbol
}

var lspGlobals = map[string]lspGlobal{
	"console": {"var console: Console", "The console of the runtime: messages for the developer.", []*lspSymbol{
		method("log", "(method) Console.log(...data: any[]): void", "Prints its arguments, separated by spaces.", ""),
		method("info", "(method) Console.info(...data: any[]): void", "Prints an informational message.", ""),
		method("warn", "(method) Console.warn(...data: any[]): void", "Prints a warning.", ""),
		method("error", "(method) Console.error(...data: any[]): void", "Prints an error.", ""),
		method("table", "(method) Console.table(data: any): void", "Prints an array or object as a table.", ""),
	}},
	"Math": {"var Math: Math", "Mathematical constants and functions.", []*lspSymbol{
		method("abs", "(method) Math.abs(x: number): number", "The absolute value of `x`.", "number"),
		method("ceil", "(method) Math.ceil(x: number): number", "The smallest integer at least `x`.", "number"),
		method("floor", "(method) Math.floor(x: number): number", "The largest integer at most `x`.", "number"),
		method("max", "(method) Math.max(...values: number[]): number", "The largest of the values.", "number"),
		method("min", "(method) Math.min(...values: number[]): number", "The smallest of the values.", "number"),
		method("random", "(method) Math.random(): number", "A pseudo-random number from 0 up to 1.", "number"),
		method("round", "(method) Math.round(x: number): number", "`x` rounded to the nearest integer.", "number"),
		method("sqrt", "(method) Math.sqrt(x: number): number", "The square root of `x`.", "number"),
		builtin("constant", "PI", "(property) Math.PI: number", "The ratio of a circle's circumference to its diameter.", "number"),
	}},
	"JSON": {"var JSON: JSON", "Converts values to and from JSON.", []*lspSymbol{
		method("parse", "(method) JSON.parse(text: string): any", "The value the JSON text describes.", ""),
		method("stringify", "(method) JSON.stringify(value: any, replacer?: null, space?: number): string", "The value as JSON text.", "string"),
	}},
}

// lspGlobalNames are the globals in a fixed order, for completion.
var lspGlobalNames = []string{"console", "Math", "JSON"}

var lspString = []*lspSymbol{
	builtin("field", "length", "(property) String.length: number", "The number of UTF-16 code units.", "number"),
	method("includes", "(method) String.includes(search: string): boolean", "Whether `search` is in the string.", "boolean"),
	method("padStart", "(method) String.padStart(length: number, fill?: string): string", "The string padded at its start to `length`.", "string"),
	method("split", "(method) String.split(separator: string): string[]", "The parts between the separators.", "string[]"),
	method("toLowerCase", "(method) String.toLowerCase(): string", "The string in lower case.", "string"),
	method("toUpperCase", "(method) String.toUpperCase(): string", "The string in upper case.", "string"),
	method("trim", "(method) String.trim(): string", "The string without the whitespace around it.", "string"),
}

var lspNumber = []*lspSymbol{
	method("toFixed", "(method) Number.toFixed(digits?: number): string", "The number with `digits` digits after the point.", "string"),
	method("toString", "(method) Number.toString(radix?: number): string", "The number as text.", "string"),
}

func lspArray(item string) []*lspSymbol {
	return []*lspSymbol{
		builtin("field", "length", "(property) Array<T>.length: number", "The number of elements.", "number"),
		method("filter", fmt.Sprintf("(method) Array<%s>.filter(predicate: (value: %[1]s) => boolean): %[1]s[]", item), "The elements the predicate keeps.", item+"[]"),
		method("find", fmt.Sprintf("(method) Array<%s>.find(predicate: (value: %[1]s) => boolean): %[1]s | undefined", item), "The first element the predicate takes.", item),
		method("forEach", fmt.Sprintf("(method) Array<%s>.forEach(callback: (value: %[1]s) => void): void", item), "Calls `callback` for each element.", ""),
		method("includes", fmt.Sprintf("(method) Array<%s>.includes(value: %[1]s): boolean", item), "Whether `value` is an element.", "boolean"),
		method("join", fmt.Sprintf("(method) Array<%s>.join(separator?: string): string", item), "The elements as text, between separators.", "string"),
		method("map", fmt.Sprintf("(method) Array<%s>.map<U>(callback: (value: %[1]s) => U): U[]", item), "The callback's results, in order.", ""),
		method("push", fmt.Sprintf("(method) Array<%s>.push(...items: %[1]s[]): number", item), "Adds the items at the end; the new length.", "number"),
		method("reduce", fmt.Sprintf("(method) Array<%s>.reduce<U>(callback: (sum: U, value: %[1]s) => U, initial: U): U", item), "Folds the elements into one value.", ""),
	}
}

var lspSnippets = []editor.CompletionItem{
	{Label: "forof", Kind: "snippet", Detail: "for … of loop", InsertText: "for (const item of items) {\n  \n}", Documentation: "Loops over the elements of an array."},
	{Label: "log", Kind: "snippet", Detail: "console.log()", InsertText: "console.log();", Documentation: "Prints to the console."},
	{Label: "fn", Kind: "snippet", Detail: "function declaration", InsertText: "function name() {\n  \n}", Documentation: "Declares a function."},
	{Label: "iife", Kind: "snippet", Detail: "immediately invoked function", InsertText: "(() => {\n  \n})();", Documentation: "Runs a function in place."},
}

func identChar(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// wordAt is the word around byte col of line, if it starts with no digit.
func wordAt(line string, col int) (word string, start, end int, ok bool) {
	start, end = col, col
	for start > 0 && start <= len(line) && identChar(line[start-1]) {
		start--
	}
	for end < len(line) && identChar(line[end]) {
		end++
	}
	if start == end || line[start] >= '0' && line[start] <= '9' {
		return "", 0, 0, false
	}
	return line[start:end], start, end, true
}

// col16 is byte b of line as a UTF-16 column; byteOf the reverse.
func col16(line string, b int) int {
	n := 0
	for i, r := range line {
		if i >= b {
			break
		}
		n += len(utf16.Encode([]rune{r}))
	}
	return n
}

func byteOf(line string, c int) int {
	n := 0
	for i, r := range line {
		if n >= c {
			return i
		}
		n += len(utf16.Encode([]rune{r}))
	}
	return len(line)
}

func (m *lspModel) span(line, start, end int) editor.Range {
	l := ""
	if line < len(m.lines) {
		l = m.lines[line]
	}
	return editor.Range{Start: editor.Position{Line: line, Column: col16(l, start)}, End: editor.Position{Line: line, Column: col16(l, end)}}
}

// closing is where what open opened (a quote, a template or a block
// comment) ends on text from from: just past its closing, or -1 when it
// runs on past the line (a quote ends with its line).
func closing(open, text string, from int) int {
	if open == "/*" {
		if c := strings.Index(text[from:], "*/"); c >= 0 {
			return from + c + 2
		}
		return -1
	}
	end := from
	for end < len(text) && text[end] != open[0] {
		if text[end] == '\\' {
			end += 2
		} else {
			end++
		}
	}
	if end < len(text) {
		return end + 1
	}
	if open == "`" {
		return -1
	}
	return len(text)
}

var codeStarts = regexp.MustCompile("//|/\\*|[\"'`]")

// mask is the lines with string literals and comments blanked out, so the
// rules look at code only; a template or a block comment left open runs
// on into the next lines.
func mask(lines []string) []string {
	out := make([]string, len(lines))
	blank := func(n int) string { return strings.Repeat(" ", max(n, 0)) }
	open := ""
	for line, text := range lines {
		var masked strings.Builder
		code := 0
		if open != "" {
			end := closing(open, text, 0)
			if end < 0 {
				out[line] = blank(len(text))
				continue
			}
			masked.WriteString(blank(end))
			code, open = end, ""
		}
		for code < len(text) {
			loc := codeStarts.FindStringIndex(text[code:])
			if loc == nil {
				break
			}
			at, tok := code+loc[0], text[code+loc[0]:code+loc[1]]
			masked.WriteString(text[code:at])
			end := len(text)
			if tok != "//" {
				end = closing(tok, text, at+len(tok))
			}
			if end < 0 {
				open, end = tok, len(text)
			}
			masked.WriteString(blank(end - at))
			code = end
		}
		if code == 0 {
			out[line] = text
		} else {
			out[line] = masked.String() + text[min(code, len(text)):]
		}
	}
	return out
}

// blockEnd is the line of the } closing the block opened on line start.
func blockEnd(code []string, start int) int {
	depth, opened := 0, false
	for line := start; line < len(code); line++ {
		for _, c := range code[line] {
			switch c {
			case '{':
				depth++
				opened = true
			case '}':
				depth--
			}
		}
		if opened && depth <= 0 {
			return line
		}
	}
	return len(code) - 1
}

var paramRe = regexp.MustCompile(`^(\s*)([A-Za-z_$][\w$]*)\??\s*(?::\s*(.+?))?\s*$`)

type lspParam struct {
	name, typ string
	col       int
}

// parameters are a function's name: type pairs.
func parameters(list string) []lspParam {
	var out []lspParam
	col := 0
	for _, part := range strings.Split(list, ",") {
		if m := paramRe.FindStringSubmatch(part); m != nil {
			typ := m[3]
			if typ == "" {
				typ = "any"
			}
			out = append(out, lspParam{m[2], typ, col + len(m[1])})
		}
		col += len(part) + 1
	}
	return out
}

var (
	typeRe     = regexp.MustCompile(`^(\s*(?:export\s+)?)(interface|class)\s+([A-Za-z_$][\w$]*)`)
	methodRe   = regexp.MustCompile(`^(\s+)([A-Za-z_$][\w$]*)\(([^)]*)\)\s*(?::\s*([^{]+?))?\s*\{`)
	fieldRe    = regexp.MustCompile(`^(\s+)([A-Za-z_$][\w$]*)(\??)\s*:\s*([^=;]+?)\s*(?:=.*)?;`)
	functionRe = regexp.MustCompile(`^(\s*(?:export\s+)?(?:async\s+)?function\s+)([A-Za-z_$][\w$]*)\s*\(([^)]*)\)\s*(?::\s*([^{]+?))?\s*\{`)
	docRe      = regexp.MustCompile(`^\s*/\*\*\s*(.*?)\s*\*/\s*$`)
	varRe      = regexp.MustCompile(`\b(const|let|var)\s+([A-Za-z_$][\w$]*)\s*(?::\s*([\w$\[\]<>|]+))?\s*=\s*(.*)`)
	newRe      = regexp.MustCompile(`^new\s+([A-Za-z_$][\w$]*)`)
	numberRe   = regexp.MustCompile(`^-?\d`)
	boolRe     = regexp.MustCompile(`^(true|false)\b`)
	callRe     = regexp.MustCompile(`^([A-Za-z_$][\w$]*)\s*\(`)
	constantRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`)
	exportRe   = regexp.MustCompile(`^export\s+`)
)

// analyze reads a text's declarations.
func analyze(text string) *lspModel {
	lines := strings.Split(text, "\n")
	code := mask(lines)
	m := &lspModel{lines: lines, code: code, types: map[string]*lspType{}, declarations: map[int]bool{}, byName: map[string]*lspNamed{}}
	var functions []lineSpan
	docAbove := func(line int) string {
		if line < 1 {
			return ""
		}
		if d := docRe.FindStringSubmatch(lines[line-1]); d != nil {
			return d[1]
		}
		return ""
	}
	addParams := func(params string, line, open int, scope lineSpan) {
		for _, p := range parameters(params) {
			at := open + p.col
			sc := scope
			m.symbols = append(m.symbols, &lspSymbol{name: p.name, kind: "parameter", signature: "(parameter) " + p.name + ": " + p.typ,
				rng: m.span(line, at, at+len(p.name)), scope: &sc, typ: p.typ})
		}
	}
	for line := range lines {
		source := lines[line]
		if t := typeRe.FindStringSubmatch(code[line]); t != nil {
			kind, name := t[2], t[3]
			col := len(t[1]) + len(kind) + 1
			end := blockEnd(code, line)
			signature := "class " + name
			if kind == "interface" {
				signature = exportRe.ReplaceAllString(strings.Join(lines[line:end+1], "\n"), "")
			}
			sym := &lspSymbol{name: name, kind: kind, signature: signature, doc: docAbove(line), rng: m.span(line, col, col+len(name))}
			m.symbols = append(m.symbols, sym)
			ty := &lspType{symbol: sym, start: line, end: end}
			for inner := line + 1; inner < end; inner++ {
				doc := docAbove(inner)
				if f := methodRe.FindStringSubmatch(code[inner]); f != nil {
					ret := f[4]
					if ret == "" {
						ret = "void"
					}
					open := len(f[1]) + len(f[2]) + 1
					params := lines[inner][open : open+len(f[3])]
					start := len(f[1])
					mEnd := blockEnd(code, inner)
					ty.members = append(ty.members, &lspSymbol{name: f[2], kind: "method", signature: fmt.Sprintf("(method) %s.%s(%s): %s", name, f[2], params, ret),
						doc: doc, rng: m.span(inner, start, start+len(f[2])), typ: ret, body: &lineSpan{inner, mEnd}})
					m.declarations[inner] = true
					functions = append(functions, lineSpan{inner, mEnd})
					addParams(params, inner, open, lineSpan{inner, mEnd})
				} else if f := fieldRe.FindStringSubmatch(lines[inner]); f != nil {
					start := len(f[1])
					ty.members = append(ty.members, &lspSymbol{name: f[2], kind: "field", signature: fmt.Sprintf("(property) %s.%s%s: %s", name, f[2], f[3], f[4]),
						doc: doc, rng: m.span(inner, start, start+len(f[2])), typ: f[4]})
				}
			}
			m.types[name] = ty
			m.typeNames = append(m.typeNames, name)
		} else if f := functionRe.FindStringSubmatch(code[line]); f != nil {
			name, col := f[2], len(f[1])
			ret := f[4]
			if ret == "" {
				ret = "void"
			}
			open := col + len(name) + 1
			params := source[open : open+len(f[3])]
			end := blockEnd(code, line)
			functions = append(functions, lineSpan{line, end})
			m.symbols = append(m.symbols, &lspSymbol{name: name, kind: "function", signature: fmt.Sprintf("function %s(%s): %s", name, params, ret),
				doc: docAbove(line), rng: m.span(line, col, col+len(name)), typ: ret, body: &lineSpan{line, end}})
			addParams(params, line, open, lineSpan{line, end})
		}
	}

	// Variables, once the functions (their scopes) are known.
	returns := map[string]string{}
	for _, s := range m.symbols {
		if _, ok := returns[s.name]; s.kind == "function" && !ok {
			returns[s.name] = s.typ
		}
	}
	// The innermost function around each line: the one starting last.
	scopes := make([]*lineSpan, len(lines))
	sorted := slices.Clone(functions)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].start < sorted[j].start })
	for i := range sorted {
		f := &sorted[i]
		for line := f.start; line <= f.end && line < len(lines); line++ {
			scopes[line] = f
		}
	}
	for line := range lines {
		for _, v := range varRe.FindAllStringSubmatchIndex(code[line], -1) {
			keyword, name := code[line][v[2]:v[3]], code[line][v[4]:v[5]]
			annotation := ""
			if v[6] >= 0 {
				annotation = code[line][v[6]:v[7]]
			}
			value := strings.TrimSpace(lines[line][v[8]:])
			typ, shown := annotation, ""
			if typ == "" {
				switch {
				case newRe.MatchString(value):
					typ = newRe.FindStringSubmatch(value)[1]
				case strings.ContainsAny(value[:min(1, len(value))], "\"'`"):
					typ = "string"
					if keyword == "const" {
						shown = strings.TrimSuffix(value, ";")
					}
				case numberRe.MatchString(value):
					typ = "number"
				case boolRe.MatchString(value):
					typ = "boolean"
				case callRe.MatchString(value):
					typ = returns[callRe.FindStringSubmatch(value)[1]]
				}
			}
			col := v[0] + strings.Index(code[line][v[0]:v[1]][len(keyword):], name) + len(keyword)
			kind := "variable"
			if keyword == "const" && constantRe.MatchString(name) {
				kind = "constant"
			}
			sig := shown
			if sig == "" {
				sig = typ
			}
			if sig == "" {
				sig = "any"
			}
			m.symbols = append(m.symbols, &lspSymbol{name: name, kind: kind, signature: keyword + " " + name + ": " + sig, doc: docAbove(line),
				rng: m.span(line, col, col+len(name)), scope: scopes[line], typ: typ})
		}
	}
	for _, s := range m.symbols {
		n := m.byName[s.name]
		if n == nil {
			n = &lspNamed{}
			m.byName[s.name] = n
		}
		if s.scope != nil {
			n.scoped = append(n.scoped, s)
		} else if n.global == nil {
			n.global = s
		}
	}
	for _, n := range m.byName {
		// Stable: of scopes starting on one line, the last declared wins.
		sort.SliceStable(n.scoped, func(i, j int) bool { return n.scoped[i].scope.start < n.scoped[j].scope.start })
		reach := -1
		for _, s := range n.scoped {
			reach = max(reach, s.scope.end)
			n.reach = append(n.reach, reach)
		}
	}
	return m
}

// lookup is the symbol name means on line: the innermost visible there.
func (m *lspModel) lookup(name string, line int) *lspSymbol {
	n := m.byName[name]
	if n == nil {
		return nil
	}
	low := sort.Search(len(n.scoped), func(i int) bool { return n.scoped[i].scope.start > line })
	for i := low - 1; i >= 0 && n.reach[i] >= line; i-- {
		if n.scoped[i].scope.end >= line {
			return n.scoped[i]
		}
	}
	return n.global
}

// membersOf is the members after a "." on a value of typ.
func (m *lspModel) membersOf(typ string) []*lspSymbol {
	t := strings.TrimSpace(typ)
	switch {
	case t == "":
		return nil
	case strings.HasSuffix(t, "[]"):
		return lspArray(strings.TrimSuffix(t, "[]"))
	case t == "string" || strings.ContainsAny(t[:1], "\"'`"):
		return lspString
	case t == "number":
		return lspNumber
	case strings.HasPrefix(t, "global:"):
		return lspGlobals[strings.TrimPrefix(t, "global:")].members
	}
	if ty := m.types[t]; ty != nil {
		return ty.members
	}
	return nil
}

func memberNamed(members []*lspSymbol, name string) *lspSymbol {
	for _, s := range members {
		if s.name == name {
			return s
		}
	}
	return nil
}

var receiverRe = regexp.MustCompile(`((?:[A-Za-z_$][\w$]*\s*\.\s*)*[A-Za-z_$][\w$]*)\s*$`)

// receiverType is the type of the expression before the "." at byte dot
// on line: this, a name, and members after it.
func (m *lspModel) receiverType(line, dot int) string {
	if line >= len(m.code) {
		return ""
	}
	r := receiverRe.FindStringSubmatch(m.code[line][:min(dot, len(m.code[line]))])
	if r == nil {
		return ""
	}
	words := strings.Split(r[1], ".")
	first := strings.TrimSpace(words[0])
	typ := ""
	switch {
	case first == "this":
		for _, name := range m.typeNames {
			if t := m.types[name]; t.symbol.kind == "class" && t.start <= line && line <= t.end {
				typ = name
			}
		}
	case m.lookup(first, line) != nil:
		typ = m.lookup(first, line).typ
	default:
		if _, ok := lspGlobals[first]; ok {
			typ = "global:" + first
		}
	}
	for _, w := range words[1:] {
		s := memberNamed(m.membersOf(typ), strings.TrimSpace(w))
		typ = ""
		if s != nil {
			typ = s.typ
		}
	}
	return typ
}

func typeName(t string) string { return strings.TrimPrefix(t, "global:") }

// closest is the name of names nearest word, if near enough to be a typo.
func closest(word string, names []string) string {
	distance := func(a, b []rune) int {
		row := make([]int, len(b)+1)
		for i := range row {
			row[i] = i
		}
		for i := 1; i <= len(a); i++ {
			prev := row[0]
			row[0] = i
			for j := 1; j <= len(b); j++ {
				tmp := row[j]
				cost := 1
				if a[i-1] == b[j-1] {
					cost = 0
				}
				row[j] = min(row[j]+1, row[j-1]+1, prev+cost)
				prev = tmp
			}
		}
		return row[len(b)]
	}
	best, bestD := "", max(1, utf8.RuneCountInString(word)/3)+1
	for _, n := range names {
		if d := distance([]rune(strings.ToLower(word)), []rune(strings.ToLower(n))); d < bestD {
			best, bestD = n, d
		}
	}
	return best
}

var (
	todoRe   = regexp.MustCompile(`//\s*((?:TODO|FIXME)\b:?\s*(.*))$`)
	varKwRe  = regexp.MustCompile(`\bvar\b`)
	accessRe = regexp.MustCompile(`\.\s*([A-Za-z_$][\w$]*)`)
	callAtRe = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)\s*\(`)
	wordRe   = regexp.MustCompile(`[\w$]+`)
)

// lint is the problems of the file and their fixes, in order.
func (m *lspModel) lint() []lspProblem {
	lines, code := m.lines, m.code
	byLine := make([][]lspProblem, len(lines))
	problem := func(rng editor.Range, severity editor.Severity, message, source, code string, fixes []lspAction, unnecessary bool) {
		d := editor.Diagnostic{Range: rng, Severity: severity, Message: message, Source: source, Code: code, Unnecessary: unnecessary}
		// ESLint's rules each have a page.
		if source == "eslint" && code != "" {
			d.CodeDescription = "https://eslint.org/docs/latest/rules/" + code
		}
		byLine[rng.Start.Line] = append(byLine[rng.Start.Line], lspProblem{d, fixes})
	}
	replace := func(title string, rng editor.Range, by string) lspAction {
		return lspAction{title: title, kind: "quickfix", edits: []editor.TextEdit{{Range: rng, Text: by}}}
	}
	var functions []string
	functionNames := func() []string {
		if functions == nil {
			seen := map[string]bool{}
			functions = []string{}
			for _, s := range m.symbols {
				if s.kind == "function" && !seen[s.name] {
					seen[s.name] = true
					functions = append(functions, s.name)
				}
			}
		}
		return functions
	}
	builtins := []string{"String", "Number", "Boolean", "Array", "Object", "Date", "Promise", "setTimeout", "parseInt", "parseFloat"}
	for line := range lines {
		// A TODO left in a comment.
		if t := todoRe.FindStringSubmatchIndex(lines[line]); t != nil && strings.TrimSpace(code[line][t[0]:]) == "" {
			start := t[2]
			before := len(strings.TrimRight(lines[line][:t[0]], " \t"))
			problem(m.span(line, start, len(lines[line])), editor.SeverityInfo, lines[line][t[2]:t[3]], "todo", "",
				[]lspAction{{title: "Remove the TODO comment", kind: "quickfix", edits: []editor.TextEdit{{Range: m.span(line, before, len(lines[line]))}}}}, false)
		}
		// var instead of let or const.
		for _, v := range varKwRe.FindAllStringIndex(code[line], -1) {
			rng := m.span(line, v[0], v[0]+3)
			problem(rng, editor.SeverityWarning, "Unexpected var, use let or const instead.", "eslint", "no-var",
				[]lspAction{replace("Replace 'var' with 'let'", rng, "let"), replace("Replace 'var' with 'const'", rng, "const")}, false)
		}
		// == and !=, which convert their operands.
		c := code[line]
		for i := 0; i+1 < len(c); i++ {
			if c[i+1] != '=' || c[i] != '=' && c[i] != '!' || i > 0 && strings.IndexByte("=!<>", c[i-1]) >= 0 || i+2 < len(c) && c[i+2] == '=' {
				continue
			}
			op := c[i : i+2]
			rng := m.span(line, i, i+2)
			problem(rng, editor.SeverityWarning, fmt.Sprintf("Expected '%s=' and instead saw '%s'.", op, op), "eslint", "eqeqeq",
				[]lspAction{replace(fmt.Sprintf("Replace '%s' with '%s='", op, op), rng, op+"=")}, false)
			i++
		}
		// Members that are not there, after this, a name of a known type
		// or a global.
		for _, a := range accessRe.FindAllStringSubmatchIndex(c, -1) {
			typ := m.receiverType(line, a[0])
			members := m.membersOf(typ)
			name := c[a[2]:a[3]]
			if typ == "" || len(members) == 0 || memberNamed(members, name) != nil {
				continue
			}
			rng := m.span(line, a[2], a[3])
			var names []string
			for _, s := range members {
				names = append(names, s.name)
			}
			guess := closest(name, names)
			msg := fmt.Sprintf("Property '%s' does not exist on type '%s'.", name, typeName(typ))
			var fixes []lspAction
			code := "2339"
			if guess != "" {
				msg += fmt.Sprintf(" Did you mean '%s'?", guess)
				fixes, code = []lspAction{replace(fmt.Sprintf("Change spelling to '%s'", guess), rng, guess)}, "2551"
			}
			problem(rng, editor.SeverityError, msg, "ts", code, fixes, false)
		}
		// Calls of names declared nowhere.
		if !m.declarations[line] {
			for _, k := range callAtRe.FindAllStringSubmatchIndex(c, -1) {
				before := strings.TrimRight(c[:k[2]], " \t")
				if len(before) > 0 && (identChar(before[len(before)-1]) || before[len(before)-1] == '.') {
					continue
				}
				name := c[k[2]:k[3]]
				_, global := lspGlobals[name]
				if slices.Contains(lspKeywords, name) || global || m.lookup(name, line) != nil || slices.Contains(builtins, name) {
					continue
				}
				rng := m.span(line, k[2], k[3])
				guess := closest(name, functionNames())
				msg := fmt.Sprintf("Cannot find name '%s'.", name)
				var fixes []lspAction
				if guess != "" {
					msg += fmt.Sprintf(" Did you mean '%s'?", guess)
					fixes = []lspAction{replace(fmt.Sprintf("Change spelling to '%s'", guess), rng, guess)}
				}
				problem(rng, editor.SeverityError, msg, "ts", "2304", fixes, false)
			}
		}
	}

	// Variables declared and never read: every name counted in one pass.
	uses := map[string]int{}
	for _, l := range lines {
		for _, w := range wordRe.FindAllStringIndex(l, -1) {
			if w[0] > 0 && l[w[0]-1] == '.' || l[w[0]] >= '0' && l[w[0]] <= '9' {
				continue
			}
			uses[l[w[0]:w[1]]]++
		}
	}
	for _, s := range m.symbols {
		if s.kind != "variable" && s.kind != "constant" || uses[s.name] > 1 {
			continue
		}
		line := s.rng.Start.Line
		next := editor.Position{Line: line + 1}
		if line+1 >= len(lines) {
			next = editor.Position{Line: line, Column: col16(lines[line], len(lines[line]))}
		}
		problem(s.rng, editor.SeverityWarning, fmt.Sprintf("'%s' is declared but its value is never read.", s.name), "ts", "6133",
			[]lspAction{{title: fmt.Sprintf("Remove unused declaration '%s'", s.name), kind: "quickfix",
				edits: []editor.TextEdit{{Range: editor.Range{Start: editor.Position{Line: line}, End: next}}}}}, true)
	}
	var out []lspProblem
	for _, here := range byLine {
		sort.SliceStable(here, func(i, j int) bool { return here[i].diagnostic.Range.Start.Column < here[j].diagnostic.Range.Start.Column })
		out = append(out, here...)
	}
	return out
}

// colors are the hex color literals of the file.
func (m *lspModel) colors() []lspColor {
	var out []lspColor
	hex := func(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
	for line, l := range m.lines {
		for i := 0; i < len(l); i++ {
			if l[i] != '#' {
				continue
			}
			n := 0
			for i+1+n < len(l) && hex(l[i+1+n]) {
				n++
			}
			if n != 3 && n != 6 || i+1+n < len(l) && identChar(l[i+1+n]) {
				continue
			}
			digits := strings.ToLower(l[i+1 : i+1+n])
			if n == 3 {
				digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
			}
			var v uint32
			fmt.Sscanf(digits, "%06x", &v)
			out = append(out, lspColor{m.span(line, i, i+1+n), core.RGB(v)})
			i += n
		}
	}
	return out
}

// symbolAt is the symbol at p, the range of its word, or the global
// named there.
func (m *lspModel) symbolAt(p editor.Position) (sym *lspSymbol, global string, rng editor.Range, ok bool) {
	if p.Line >= len(m.code) {
		return nil, "", rng, false
	}
	code := m.code[p.Line]
	word, start, end, found := wordAt(code, byteOf(m.lines[p.Line], p.Column))
	if !found || slices.Contains(lspKeywords, word) {
		return nil, "", rng, false
	}
	rng = m.span(p.Line, start, end)
	before := strings.TrimRight(code[:start], " \t")
	if strings.HasSuffix(before, ".") {
		return memberNamed(m.membersOf(m.receiverType(p.Line, len(before)-1)), word), "", rng, true
	}
	// A member's own declaration.
	for _, name := range m.typeNames {
		for _, s := range m.types[name].members {
			if s.rng.Start.Line == p.Line && s.rng.Start == rng.Start {
				return s, "", rng, true
			}
		}
	}
	if s := m.lookup(word, p.Line); s != nil {
		return s, "", rng, true
	}
	if _, ok := lspGlobals[word]; ok {
		return nil, word, rng, true
	}
	return nil, "", rng, false
}

func (m *lspModel) hover(p editor.Position) *lspHover {
	sym, global, rng, ok := m.symbolAt(p)
	switch {
	case !ok:
		return nil
	case global != "":
		g := lspGlobals[global]
		return &lspHover{"```typescript\n" + g.signature + "\n```\n\n" + g.doc, rng}
	case sym != nil:
		md := "```typescript\n" + sym.signature + "\n```"
		if sym.doc != "" {
			md += "\n\n" + sym.doc
		}
		return &lspHover{md, rng}
	}
	return nil
}

func (m *lspModel) definition(p editor.Position) *lspTarget {
	if sym, _, _, ok := m.symbolAt(p); ok && sym != nil && !sym.builtin {
		return &lspTarget{sym.name, sym.rng}
	}
	return nil
}

var (
	typeExpectedRe = regexp.MustCompile(`(?::|\bas|\bextends|\bimplements)\s*$`)
	newBeforeRe    = regexp.MustCompile(`\bnew\s+$`)
)

func completionKind(kind string) string {
	if kind == "parameter" {
		return "variable"
	}
	return kind
}

// completion is the items at a request's position, and the latency of
// the answer.
func (m *lspModel) completion(r editor.CompletionRequest) ([]editor.CompletionItem, time.Duration) {
	line := r.Position.Line
	if line >= len(m.code) {
		return nil, 30 * time.Millisecond
	}
	col := byteOf(m.lines[line], r.Position.Column)
	queryStart := max(col-len(r.Query), 0)
	code := m.code[line]
	before := code[:min(queryStart, len(code))]
	// Nothing in strings and comments: their characters are blanked out.
	if queryStart > 0 && queryStart-1 < len(code) && code[queryStart-1] == ' ' && m.lines[line][queryStart-1] != ' ' {
		return nil, 30 * time.Millisecond
	}
	var items []editor.CompletionItem
	member := r.Character == "." || strings.HasSuffix(strings.TrimRight(before, " \t"), ".")
	switch {
	case member:
		dot := len(strings.TrimRight(before, " \t")) - 1
		for _, s := range m.membersOf(m.receiverType(line, dot)) {
			items = append(items, editor.CompletionItem{Label: s.name, Kind: completionKind(s.kind), Detail: s.signature, Documentation: s.doc, SortText: "0" + s.name})
		}
	case typeExpectedRe.MatchString(before):
		for _, name := range m.typeNames {
			s := m.types[name].symbol
			detail := "interface " + s.name
			if s.kind == "class" {
				detail = s.signature
			}
			items = append(items, editor.CompletionItem{Label: s.name, Kind: completionKind(s.kind), Detail: detail, Documentation: s.doc, SortText: "0" + s.name})
		}
		for _, p := range lspPrimitives {
			items = append(items, editor.CompletionItem{Label: p, Kind: "keyword", SortText: "1" + p})
		}
	case newBeforeRe.MatchString(before):
		for _, name := range m.typeNames {
			if s := m.types[name].symbol; s.kind == "class" {
				items = append(items, editor.CompletionItem{Label: s.name, Kind: "class", Detail: s.signature, Documentation: s.doc})
			}
		}
	default:
		// The names visible here, the globals, the keywords and snippets.
		seen := map[string]int{}
		for _, s := range m.symbols {
			if s.scope != nil && !s.scope.has(line) || s.rng.Start.Line == line && s.rng.Start.Column == col16(m.lines[line], queryStart) ||
				m.lookup(s.name, line) != s {
				continue
			}
			order := "1"
			if s.scope != nil {
				order = "0"
			}
			it := editor.CompletionItem{Label: s.name, Kind: completionKind(s.kind), Detail: s.signature, Documentation: s.doc, SortText: order + s.name}
			if i, ok := seen[s.name]; ok {
				items[i] = it
				continue
			}
			seen[s.name] = len(items)
			items = append(items, it)
		}
		for _, name := range lspGlobalNames {
			g := lspGlobals[name]
			items = append(items, editor.CompletionItem{Label: name, Kind: "module", Detail: g.signature, Documentation: g.doc, SortText: "2" + name})
		}
		for _, k := range lspKeywords {
			items = append(items, editor.CompletionItem{Label: k, Kind: "keyword", SortText: "3" + k})
		}
		for _, s := range lspSnippets {
			s.SortText = "4" + s.Label
			items = append(items, s)
		}
	}
	// A first cut, as servers make; the editor ranks and filters the rest
	// as the query grows.
	if q := strings.ToLower(r.Query); q != "" {
		items = slices.DeleteFunc(items, func(it editor.CompletionItem) bool {
			f := it.FilterText
			if f == "" {
				f = it.Label
			}
			return !strings.HasPrefix(strings.ToLower(f), q[:1])
		})
	}
	if member {
		return items, 30 * time.Millisecond
	}
	return items, 60 * time.Millisecond
}

var inlineRules = []struct {
	re   *regexp.Regexp
	make func(m *lspModel, before string, enclosing *lspSymbol) string
}{
	{regexp.MustCompile(`^\s*function\s+[A-Za-z_$][\w$]*$`), func(*lspModel, string, *lspSymbol) string { return "(order: Order): number {\n  return 0;\n}" }},
	{regexp.MustCompile(`^\s*for\s?$`), func(_ *lspModel, before string, _ *lspSymbol) string { return space(before) + "(const item of order.items) {" }},
	{regexp.MustCompile(`^\s*if\s?$`), func(_ *lspModel, before string, _ *lspSymbol) string { return space(before) + `(order.coupon === "SAVE10") {` }},
	{regexp.MustCompile(`\bconsole\.$`), func(*lspModel, string, *lspSymbol) string { return "log(label);" }},
	{regexp.MustCompile(`^\s*receipt\.$`), func(*lspModel, string, *lspSymbol) string { return "add(label);" }},
	{regexp.MustCompile(`^\s*//\s*TODO$`), func(*lspModel, string, *lspSymbol) string { return ": " }},
	{regexp.MustCompile(`^\s*const total = $`), func(*lspModel, string, *lspSymbol) string { return "subtotal(order);" }},
	{regexp.MustCompile(`^\s*return\s?$`), func(_ *lspModel, before string, enclosing *lspSymbol) string {
		typ := ""
		if enclosing != nil {
			typ = enclosing.typ
		}
		value := map[string]string{"number": "0", "string": "`#${order.id}`", "boolean": "false"}[typ]
		if value == "" {
			return ""
		}
		return space(before) + value + ";"
	}},
}

// space is the space a suggestion starts with after before.
func space(before string) string {
	if strings.HasSuffix(before, " ") {
		return ""
	}
	return " "
}

// inline is the rest of the line to suggest after p, as ghost text.
func (m *lspModel) inline(p editor.Position) string {
	if p.Line >= len(m.lines) {
		return ""
	}
	text := m.lines[p.Line]
	at := byteOf(text, p.Column)
	before := text[:at]
	if strings.TrimSpace(text[at:]) != "" {
		return ""
	}
	// The function or method the caret is in: what it returns.
	var enclosing *lspSymbol
	consider := func(s *lspSymbol) {
		if s.body.has(p.Line) && (enclosing == nil || s.body.start > enclosing.body.start) {
			enclosing = s
		}
	}
	for _, s := range m.symbols {
		consider(s)
	}
	for _, name := range m.typeNames {
		for _, s := range m.types[name].members {
			consider(s)
		}
	}
	for _, r := range inlineRules {
		if r.re.MatchString(before) {
			return r.make(m, before, enclosing)
		}
	}
	return ""
}

// codeActions are the actions at rng: the fixes of the problems there (at
// the caret, else on its line), and the editor's own refactorings of the
// selection and the lines.
func codeActions(rng editor.Range, problems []lspProblem) []lspAction {
	before := func(a, b editor.Position) bool { return a.Line < b.Line || a.Line == b.Line && a.Column <= b.Column }
	var at, onLine []lspProblem
	for _, p := range problems {
		if before(p.diagnostic.Range.Start, rng.End) && before(rng.Start, p.diagnostic.Range.End) {
			at = append(at, p)
		}
		if p.diagnostic.Range.Start.Line <= rng.Start.Line && rng.Start.Line <= p.diagnostic.Range.End.Line {
			onLine = append(onLine, p)
		}
	}
	if len(at) > 0 {
		onLine = at
	}
	var out []lspAction
	for _, p := range onLine {
		out = append(out, p.fixes...)
	}
	if rng.Start != rng.End {
		out = append(out, lspAction{title: "Convert to Uppercase", kind: "refactor", command: editor.TransformToUppercase},
			lspAction{title: "Convert to Lowercase", kind: "refactor", command: editor.TransformToLowercase},
			lspAction{title: "Convert to Title Case", kind: "refactor", command: editor.TransformToTitleCase})
	}
	if rng.Start.Line != rng.End.Line {
		out = append(out, lspAction{title: "Sort Lines Ascending", kind: "refactor", command: editor.SortLinesAscending},
			lspAction{title: "Sort Lines Descending", kind: "refactor", command: editor.SortLinesDescending})
	}
	return append(out, lspAction{title: "Toggle Line Comment", kind: "source", command: editor.ToggleComment},
		lspAction{title: "Trim Trailing Whitespace", kind: "source", command: editor.TrimTrailingWhitespace})
}

// languageServer is the simulated server: one file, kept up to date by
// change, read once per text on the goroutine that first asks.
type languageServer struct {
	mu      sync.Mutex
	text    string
	version int
	model   *lspModel
	read    int // the version model reads
}

// change gives the server the file's new text (textDocument/didChange).
func (s *languageServer) change(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if text == s.text && s.version > 0 {
		return
	}
	s.text, s.version, s.model = text, s.version+1, nil
}

// current is what the server knows of the newest text.
func (s *languageServer) current() *lspModel {
	s.mu.Lock()
	if s.model != nil && s.read == s.version {
		m := s.model
		s.mu.Unlock()
		return m
	}
	text, version := s.text, s.version
	s.mu.Unlock()
	m := analyze(text)
	s.mu.Lock()
	if s.version == version {
		s.model, s.read = m, version
	}
	s.mu.Unlock()
	return m
}

// lspAsk asks the server on a goroutine of its own and hands the answer
// to the engine latency later, as a server's round trip does.
func lspAsk[T any](cx *core.Scope, s *languageServer, latency time.Duration, work func(m *lspModel) T, answer func(T)) {
	go func() {
		v := work(s.current())
		time.Sleep(latency)
		cx.Post(func() { answer(v) })
	}()
}
