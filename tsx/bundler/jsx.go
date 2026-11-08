package bundler

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/parser"
)

// The JSX compiler (NSH's jsx.rs). esbuild has no hook between parsing and
// printing, so each module goes through its classic JSX transform first:
// `<View a={x()}>{y}</View>` is `__jsx(import_nsgo.View, { a: x() }, y)`
// in CommonJS. This pass reads that with goja's parser and writes each
// call as what NSH's compiler emits, the expressions copied as they are:
// host elements made in place (element, prop, bindProp, append, insert),
// components called with props whose changing parts are getters, If and
// Each as cond and each.

// jsxFactory and jsxFragment are what esbuild's transform calls.
const (
	jsxFactory  = "__jsx"
	jsxFragment = "__frag"
)

// compiled is what a primitive's tag compiles to.
type compiled struct {
	host      string // a host element made in place
	list      bool   // the renderer's list
	component bool   // a call of the export
}

// primitives are the primitives by (module, export), NSH's
// (jsx::PRIMITIVES) in NSGo's modules.
var primitives = map[[2]string]compiled{
	{"nsgo", "View"}:                      {host: "view"},
	{"nsgo", "Text"}:                      {host: "text"},
	{"nsgo", "Scroll"}:                    {host: "scroll"},
	{"nsgo", "Overlay"}:                   {host: "overlay"},
	{"nsgo", "Image"}:                     {host: "image"},
	{"nsgo", "WebView"}:                   {host: "webview"},
	{"nsgo", "Video"}:                     {host: "video"},
	{"nsgo", "Shader"}:                    {host: "shader"},
	{"nsgo", "ShaderEffect"}:              {host: "shadereffect"},
	{"nsgo", "ShaderTransition"}:          {host: "shadertransition"},
	{"nsgo", "ScreenEffect"}:              {host: "screeneffect"},
	{"nsgo", "ScreenTransition"}:          {host: "screentransition"},
	{"nsgo", "List"}:                      {list: true},
	{"nsgo", "Canvas"}:                    {component: true},
	{"nsgo/base", "Input"}:                {host: "input"},
	{"nsgo/base", "Dialog"}:               {component: true},
	{"nsgo/mobile", "InputAccessoryView"}: {component: true},
}

// helpers are the renderer's functions compiled code calls, imported as
// _$<name>, in NSH's order.
var helpers = []string{"list", "element", "text", "append", "insert", "prop", "bindProp", "spread",
	"component", "cond", "each", "merge", "ref"}

// hole prefixes generated names; apps do not use it.
const hole = "__nsgo_h"

// An Error is a compile error at a place in the app's source.
type Error struct {
	File         string
	Line, Column int // 1-based
	Message      string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Column, e.Message)
}

// lowering compiles one module's JSX.
type lowering struct {
	src string
	// modules are the variables holding a module's exports: `var
	// import_nsgo = require("nsgo")`.
	modules map[string]string
	binds   *bindings
	calls   map[int]*ast.CallExpression // the JSX calls by their start
	used    map[string]bool
	vars    int
	errs    []*Error
	// at places an offset of src in the app's source, for errors.
	at func(off int) (line, col int)
	// file is the module's path in the app.
	file string
}

// lower compiles the JSX of src, a module after esbuild's transform.
func lower(file, src string, at func(off int) (line, col int), ambient []string) (frag, error) {
	prog, err := parser.ParseFile(nil, file, src, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: the transformed module does not parse: %w", file, err)
	}
	l := &lowering{src: src, modules: map[string]string{}, calls: map[int]*ast.CallExpression{},
		used: map[string]bool{}, at: at, file: file}
	l.scan(prog)
	for _, name := range ambient {
		l.binds.declare(name, 0, true, true)
	}
	body := l.copyRange(0, len(src))
	if len(l.errs) > 0 {
		errs := make([]error, len(l.errs))
		for i, e := range l.errs {
			errs[i] = e
		}
		return nil, joinErrors(errs)
	}
	var out frag
	var names []string
	for _, h := range helpers {
		if l.used[h] {
			names = append(names, h+": _$"+h)
		}
	}
	if len(names) > 0 {
		out.gen("var { "+strings.Join(names, ", ")+" } = require(\"nsgo:rt\");\n", -1)
	}
	out.add(body)
	return out, nil
}

// scan finds the JSX calls, the module's bindings and the variables
// holding modules.
func (l *lowering) scan(prog *ast.Program) {
	l.binds = newBindings(prog, len(l.src))
	for _, st := range prog.Body {
		if v, ok := st.(*ast.VariableStatement); ok {
			for _, b := range v.List {
				l.module(b)
			}
		}
	}
	walk(prog, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpression); ok {
			if id, ok := c.Callee.(*ast.Identifier); ok && id.Name.String() == jsxFactory {
				l.calls[off(c.Idx0())] = c
			}
		}
		return true
	})
}

// module records `var import_nsgo = require("nsgo")`.
func (l *lowering) module(b *ast.Binding) {
	id, ok := b.Target.(*ast.Identifier)
	if !ok {
		return
	}
	if call, ok := b.Initializer.(*ast.CallExpression); ok {
		if callee, ok := call.Callee.(*ast.Identifier); ok && callee.Name.String() == "require" && len(call.ArgumentList) == 1 {
			if s, ok := call.ArgumentList[0].(*ast.StringLiteral); ok {
				l.modules[id.Name.String()] = s.Value.String()
			}
		}
	}
}

// off is a goja position as an offset into the source.
func off(i file.Idx) int { return int(i) - 1 }

func (l *lowering) errorAt(at int, format string, args ...any) {
	line, col := l.at(at)
	l.errs = append(l.errs, &Error{File: l.file, Line: line, Column: col, Message: fmt.Sprintf(format, args...)})
}

// copyRange copies src[start:end), each JSX call in it compiled.
func (l *lowering) copyRange(start, end int) frag {
	var out frag
	pos := start
	for _, s := range l.starts(start, end) {
		if s < pos {
			continue // inside a call already compiled
		}
		c := l.calls[s]
		// esbuild marks each call pure; what it becomes is no call to drop.
		text := strings.TrimSuffix(l.src[pos:s], "/* @__PURE__ */ ")
		if text != "" {
			out = append(out, chunk{text, pos, true})
		}
		out.add(l.element(c))
		pos = off(c.Idx1())
	}
	if pos < end {
		out = append(out, chunk{l.src[pos:end], pos, true})
	}
	return out
}

// starts returns the starts of the calls in [start, end), in order.
func (l *lowering) starts(start, end int) []int {
	var out []int
	for s, c := range l.calls {
		if s >= start && off(c.Idx1()) <= end {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}

// span is an expression and where its text is.
type span struct {
	e          ast.Expression
	start, end int
}

// copy copies an expression's text, its JSX compiled.
func (l *lowering) copy(s span) frag { return l.copyRange(s.start, s.end) }

// args returns the spans of a call's arguments: goja's positions leave
// out the parentheses around an expression, so each argument runs to the
// comma after it.
func (l *lowering) args(c *ast.CallExpression) []span {
	out := make([]span, len(c.ArgumentList))
	start := off(c.LeftParenthesis) + 1
	for i, a := range c.ArgumentList {
		end := off(c.RightParenthesis)
		next := end
		if i+1 < len(c.ArgumentList) {
			next = l.after(off(a.Idx1()), ',')
			end = next
		}
		out[i] = l.trim(span{a, start, end})
		start = next + 1
	}
	return out
}

// after returns the offset of the first delim from i on, past spaces,
// comments and closing parentheses.
func (l *lowering) after(i int, delim byte) int {
	for i < len(l.src) {
		switch c := l.src[i]; {
		case c == delim:
			return i
		case c == '/' && i+1 < len(l.src) && l.src[i+1] == '/':
			for i < len(l.src) && l.src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(l.src) && l.src[i+1] == '*':
			if j := strings.Index(l.src[i+2:], "*/"); j >= 0 {
				i += j + 4
				continue
			}
			return len(l.src)
		default:
			i++
		}
	}
	return len(l.src)
}

func (l *lowering) trim(s span) span {
	for s.start < s.end && isSpace(l.src[s.start]) {
		s.start++
	}
	for s.end > s.start && isSpace(l.src[s.end-1]) {
		s.end--
	}
	return s
}

func isSpace(c byte) bool { return c == ' ' || c == '\n' || c == '\t' || c == '\r' }

// prop is one prop of an element: its name and value, or a spread.
type prop struct {
	name   string
	key    string // the name as an object key: bare or quoted
	value  span
	spread bool
}

// props reads the props object esbuild wrote: `null` or `{ a: x, ...y }`.
func (l *lowering) props(s span) []prop {
	o, ok := s.e.(*ast.ObjectLiteral)
	if !ok {
		if _, null := s.e.(*ast.NullLiteral); null {
			return nil
		}
		return []prop{{spread: true, value: s}}
	}
	var out []prop
	start := off(o.LeftBrace) + 1
	for i, p := range o.Value {
		end := off(o.RightBrace)
		if i+1 < len(o.Value) {
			end = l.after(off(p.Idx1()), ',')
		}
		switch p := p.(type) {
		case *ast.SpreadElement:
			at := strings.Index(l.src[start:end], "...")
			out = append(out, prop{spread: true, value: l.trim(span{p.Expression, start + at + 3, end})})
		case *ast.PropertyKeyed:
			var name, key string
			switch k := p.Key.(type) {
			case *ast.StringLiteral:
				name, key = k.Value.String(), k.Literal
			case *ast.Identifier:
				name, key = k.Name.String(), k.Name.String()
			default:
				l.errorAt(off(p.Idx0()), "computed props are not supported")
				continue
			}
			colon := l.after(off(p.Key.Idx1()), ':')
			out = append(out, prop{name: name, key: key, value: l.trim(span{p.Value, colon + 1, end})})
		case *ast.PropertyShort:
			name := p.Name.Name.String()
			out = append(out, prop{name: name, key: name, value: span{&p.Name, off(p.Name.Idx0()), off(p.Name.Idx1())}})
		}
		start = end + 1
	}
	return out
}

// tagKind is what a tag names.
type tagKind uint8

const (
	tagNone tagKind = iota
	tagHost
	tagComponent
	tagFragment
	tagIf
	tagEach
)

// tag reads a JSX call's tag: a host element's core tag, or the callee
// of a component.
func (l *lowering) tag(s span) (tagKind, string, frag) {
	switch t := s.e.(type) {
	case *ast.Identifier:
		name := t.Name.String()
		if name == jsxFragment {
			return tagFragment, "", nil
		}
		if _, bound := l.binds.lookup(name, s.start); !bound {
			switch name {
			case "If":
				return tagIf, "", nil
			case "Each":
				return tagEach, "", nil
			}
			hint := ""
			if module := moduleOf(name); module != "" {
				hint = fmt.Sprintf(": add `import { %s } from %q`", name, module)
			}
			l.errorAt(s.start, "<%s> names nothing in scope%s", name, hint)
			return tagNone, "", nil
		}
	case *ast.StringLiteral:
		name := t.Value.String()
		l.errorAt(s.start, "lowercase tag <%s>: tags are PascalCase components: write <%s>%s", name, pascal(name), importHint(pascal(name)))
		return tagNone, "", nil
	case *ast.DotExpression:
		if id, ok := t.Left.(*ast.Identifier); ok {
			if module, ok := l.modules[id.Name.String()]; ok {
				switch p, ok := primitives[[2]string{module, t.Identifier.Name.String()}]; {
				case ok && p.host != "":
					return tagHost, p.host, nil
				case ok && p.list:
					return tagComponent, "", frag{chunk{l.helper("list"), s.start, false}}
				}
			}
		}
	}
	return tagComponent, "", l.copy(s)
}

// pascal is `web-view` as `WebView`: a lowercase tag as the name it
// likely meant.
func pascal(tag string) string {
	var b strings.Builder
	for part := range strings.FieldsFuncSeq(tag, func(r rune) bool { return r == '-' || r == '_' }) {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// importHint names the import a primitive's tag needs.
func importHint(name string) string {
	if module := moduleOf(name); module != "" {
		return fmt.Sprintf(" and `import { %s } from %q`", name, module)
	}
	if name == "If" || name == "Each" {
		return " (a global: no import)"
	}
	return ""
}

// moduleOf returns the module exporting primitive name.
func moduleOf(name string) string {
	for k := range primitives {
		if k[1] == name {
			return k[0]
		}
	}
	return ""
}

func (l *lowering) helper(name string) string {
	l.used[name] = true
	return "_$" + name
}

func (l *lowering) newVar() string {
	v := fmt.Sprintf("%sv%d", hole, l.vars)
	l.vars++
	return v
}

// element compiles a JSX call as an expression.
func (l *lowering) element(c *ast.CallExpression) frag {
	at := off(c.Idx0())
	args := l.args(c)
	if len(args) == 0 {
		return frag{chunk{"undefined", at, false}}
	}
	kind, core, callee := l.tag(args[0])
	var props []prop
	if len(args) > 1 {
		props = l.props(args[1])
	}
	var children []span
	if len(args) > 2 {
		children = args[2:]
	}
	var out frag
	switch kind {
	case tagHost:
		var body frag
		v := l.host(at, core, props, children, &body)
		out.gen("(() => { ", at)
		out.add(body)
		out.gen("return "+v+"; })()", at)
	case tagComponent:
		out = l.component(at, callee, props, children)
	case tagFragment:
		out.gen("[", at)
		out.add(join(l.children(children, true), ", ", at))
		out.gen("]", at)
	case tagIf:
		out = l.ifElement(at, props, children)
	case tagEach:
		out = l.eachElement(at, props, children)
	default:
		out.gen("undefined", at)
	}
	return out
}

// isJSX reports whether e is a JSX call.
func (l *lowering) isJSX(e ast.Expression) bool {
	c, ok := e.(*ast.CallExpression)
	return ok && l.calls[off(c.Idx0())] == c
}

// host compiles a host element: its statements go to body; it returns
// its variable.
func (l *lowering) host(at int, core string, props []prop, children []span, body *frag) string {
	v := l.newVar()
	body.gen(fmt.Sprintf("const %s = %s(%q); ", v, l.helper("element"), core), at)
	for _, p := range props {
		switch {
		case p.spread:
			body.gen(l.helper("spread")+"("+v+", ", p.value.start)
			body.add(l.copy(p.value))
			body.gen("); ", p.value.start)
		case p.name == "children" && len(children) > 0:
		case p.name == "ref":
			body.add(l.ref(p.value, v, true))
		default:
			var dynamic bool
			if isEvent(p.name) {
				dynamic = !isFunction(p.value.e) && !l.isName(p.value.e)
			} else {
				dynamic = l.isDynamic(p.value.e)
			}
			value := l.copy(p.value)
			if dynamic {
				body.gen(fmt.Sprintf("%s(%s, %s, ", l.helper("bindProp"), v, quote(p.name)), p.value.start)
				value = thunk(value, p.value.start)
			} else {
				body.gen(fmt.Sprintf("%s(%s, %s, ", l.helper("prop"), v, quote(p.name)), p.value.start)
			}
			body.add(value)
			body.gen("); ", p.value.start)
		}
	}
	if core != "text" {
		l.checkUnshownText(core, children)
	}
	l.hostChildren(v, children, body)
	return v
}

// checkUnshownText reports text written in a host element other than
// <Text>, which is never shown.
func (l *lowering) checkUnshownText(core string, children []span) {
	name := core
	for k, p := range primitives {
		if p.host == core {
			name = k[1]
		}
	}
	for _, c := range children {
		var text string
		switch e := c.e.(type) {
		case *ast.StringLiteral:
			text = e.Value.String()
		case *ast.NumberLiteral:
			text = e.Literal
		case *ast.TemplateLiteral:
			if len(e.Expressions) > 0 || e.Tag != nil {
				continue
			}
			for _, el := range e.Elements {
				text += el.Parsed.String()
			}
		default:
			continue
		}
		text = strings.TrimSpace(text)
		shown := []rune(text)
		more := ""
		if len(shown) > 24 {
			shown, more = shown[:24], "…"
		}
		l.errorAt(c.start, "text in <%s> is not shown: text shows in a <Text> (`<Text>%s%s</Text>`)", name, string(shown), more)
	}
}

// hostChildren compiles a host element's children: static ones made and
// appended in order, runs of dynamic ones inserted before the next static
// one.
func (l *lowering) hostChildren(v string, children []span, body *frag) {
	type item struct {
		static string // a static child's variable
		value  frag   // a dynamic child's value
	}
	var items []item
	for _, c := range children {
		if s, ok := c.e.(*ast.StringLiteral); ok {
			tv := l.newVar()
			body.gen(fmt.Sprintf("const %s = %s(%s); ", tv, l.helper("text"), quote(s.Value.String())), c.start)
			items = append(items, item{static: tv})
			continue
		}
		if l.isJSX(c.e) {
			call := c.e.(*ast.CallExpression)
			args := l.args(call)
			if len(args) > 0 {
				if kind, core, _ := l.peekTag(args[0]); kind == tagHost {
					var props []prop
					if len(args) > 1 {
						props = l.props(args[1])
					}
					var kids []span
					if len(args) > 2 {
						kids = args[2:]
					}
					items = append(items, item{static: l.host(off(call.Idx0()), core, props, kids, body)})
					continue
				}
			}
			items = append(items, item{value: l.element(call)})
			continue
		}
		items = append(items, item{value: l.value(c, true)})
	}
	for _, it := range items {
		if it.static != "" {
			body.gen(fmt.Sprintf("%s(%s, %s); ", l.helper("append"), v, it.static), -1)
		}
	}
	var run []frag
	for i, it := range items {
		if it.static != "" {
			continue
		}
		run = append(run, it.value)
		if i+1 < len(items) && items[i+1].static == "" {
			continue
		}
		marker := ""
		for _, n := range items[i+1:] {
			if n.static != "" {
				marker = n.static
				break
			}
		}
		body.gen(l.helper("insert")+"("+v+", ", -1)
		if len(run) == 1 {
			body.add(run[0])
		} else {
			body.gen("[", -1)
			body.add(join(run, ", ", -1))
			body.gen("]", -1)
		}
		if marker != "" {
			body.gen(", "+marker, -1)
		}
		body.gen("); ", -1)
		run = nil
	}
}

// peekTag is tag without reporting errors or copying: what a child is.
func (l *lowering) peekTag(s span) (tagKind, string, frag) {
	if d, ok := s.e.(*ast.DotExpression); ok {
		if id, ok := d.Left.(*ast.Identifier); ok {
			if module, ok := l.modules[id.Name.String()]; ok {
				if p, ok := primitives[[2]string{module, d.Identifier.Name.String()}]; ok && p.host != "" {
					return tagHost, p.host, nil
				}
			}
		}
	}
	return tagComponent, "", nil
}

// thunk writes f as an arrow function's body: `() => f`, an object
// literal in parentheses.
func thunk(f frag, at int) frag {
	var out frag
	if strings.HasPrefix(strings.TrimSpace(f.String()), "{") {
		out.gen("() => (", at)
		out.add(f)
		out.gen(")", at)
		return out
	}
	out.gen("() => ", at)
	out.add(f)
	return out
}

// value compiles an expression as a child's value: a function when it
// changes and the value is kept.
func (l *lowering) value(s span, keep bool) frag {
	if keep && l.isDynamic(s.e) && !isFunction(s.e) {
		return thunk(l.copy(s), s.start)
	}
	return l.copy(s)
}

// children compiles children as values, in order.
func (l *lowering) children(children []span, keep bool) []frag {
	out := make([]frag, len(children))
	for i, c := range children {
		out[i] = l.value(c, keep)
	}
	return out
}

func join(fs []frag, sep string, at int) frag {
	var out frag
	for i, f := range fs {
		if i > 0 {
			out.gen(sep, at)
		}
		out.add(f)
	}
	return out
}

// component compiles a component call with its props object.
func (l *lowering) component(at int, callee frag, props []prop, children []span) frag {
	var parts []frag
	var obj []frag
	flush := func() {
		var o frag
		if len(obj) == 0 {
			o.gen("{}", at)
		} else {
			o.gen("{ ", at)
			o.add(join(obj, ", ", at))
			o.gen(" }", at)
		}
		parts = append(parts, o)
		obj = nil
	}
	for _, p := range props {
		if p.spread {
			if len(obj) > 0 {
				flush()
			}
			parts = append(parts, l.copy(p.value))
			continue
		}
		var f frag
		if p.name == "ref" {
			if ref := l.ref(p.value, hole+"r", false); ref != nil {
				f.gen("ref("+hole+"r) { ", p.value.start)
				f.add(ref)
				f.gen("}", p.value.start)
				obj = append(obj, f)
				continue
			}
		}
		if l.isDynamic(p.value.e) {
			f.gen("get "+p.key+"() { return ", p.value.start)
			f.add(l.copy(p.value))
			f.gen("; }", p.value.start)
		} else {
			f.gen(p.key+": ", p.value.start)
			f.add(l.copy(p.value))
		}
		obj = append(obj, f)
	}
	// A lone child is read through the getter alone; among siblings an
	// expression that changes is a function of its own, which the insert
	// of the children runs alone.
	if values := l.children(children, len(children) > 1); len(values) > 0 {
		var f frag
		f.gen("get children() { return ", at)
		if len(values) == 1 {
			f.add(values[0])
		} else {
			f.gen("[", at)
			f.add(join(values, ", ", at))
			f.gen("]", at)
		}
		f.gen("; }", at)
		obj = append(obj, f)
	}
	if len(obj) > 0 || len(parts) == 0 {
		flush()
	}
	var out frag
	out.gen(l.helper("component")+"(", at)
	out.add(callee)
	out.gen(", ", at)
	if len(parts) == 1 {
		out.add(parts[0])
	} else {
		out.gen(l.helper("merge")+"(", at)
		out.add(join(parts, ", ", at))
		out.gen(")", at)
	}
	out.gen(")", at)
	return out
}

// ref compiles `ref={e}` for node: a signal is set to it and a function
// called with it, else a variable or a field is assigned it; a binding
// that cannot be assigned must hold a signal or a function. It returns
// nil for other expressions, which a host element sets as its ref prop.
func (l *lowering) ref(s span, node string, host bool) frag {
	var out frag
	switch e := s.e.(type) {
	case *ast.Identifier:
		name := e.Name.String()
		if d, ok := l.binds.lookup(name, s.start); !ok || d.fixed {
			out.gen(fmt.Sprintf("%s(%s, %s, %q); ", l.helper("ref"), name, node, name), s.start)
		} else {
			out.gen(fmt.Sprintf("%s(%s, %s) || (%s = %s); ", l.helper("ref"), name, node, name, node), s.start)
		}
		return out
	case *ast.DotExpression:
		if id, ok := e.Left.(*ast.Identifier); ok {
			if _, ok := l.modules[id.Name.String()]; ok {
				// An import: not assignable.
				out.gen(fmt.Sprintf("%s(", l.helper("ref")), s.start)
				out.add(l.copy(s))
				out.gen(fmt.Sprintf(", %s, %q); ", node, e.Identifier.Name.String()), s.start)
				return out
			}
		}
		dot := s.start + strings.LastIndexByte(l.src[s.start:off(e.Identifier.Idx0())], '.')
		left := l.trim(span{e.Left, s.start, dot})
		out.gen("(("+hole+"o, "+hole+"k) => "+l.helper("ref")+"("+hole+"o["+hole+"k], "+node+") || ("+hole+"o["+hole+"k] = "+node+"))(", s.start)
		out.add(l.copy(left))
		out.gen(", "+quote(e.Identifier.Name.String())+"); ", s.start)
		return out
	case *ast.BracketExpression:
		left := l.trim(span{e.Left, s.start, off(e.LeftBracket)})
		key := l.trim(span{e.Member, off(e.LeftBracket) + 1, off(e.RightBracket)})
		out.gen("(("+hole+"o, "+hole+"k) => "+l.helper("ref")+"("+hole+"o["+hole+"k], "+node+") || ("+hole+"o["+hole+"k] = "+node+"))(", s.start)
		out.add(l.copy(left))
		out.gen(", ", s.start)
		out.add(l.copy(key))
		out.gen("); ", s.start)
		return out
	}
	if !host {
		return nil
	}
	out.gen(fmt.Sprintf("%s(%s, \"ref\", ", l.helper("prop"), node), s.start)
	out.add(l.copy(s))
	out.gen("); ", s.start)
	return out
}

// flowProps reads the props of If or Each, each one allowed.
func (l *lowering) flowProps(at int, name string, props []prop, allowed ...string) map[string]frag {
	out := map[string]frag{}
	for _, p := range props {
		if p.spread {
			l.errorAt(p.value.start, "<%s> takes no spread props", name)
			continue
		}
		if !slices.Contains(allowed, p.name) {
			quoted := make([]string, len(allowed))
			for i, a := range allowed {
				quoted[i] = "`" + a + "`"
			}
			l.errorAt(p.value.start, "<%s> takes %s, not `%s`", name, strings.Join(quoted, ", "), p.name)
			continue
		}
		out[p.name] = l.copy(p.value)
	}
	return out
}

// ifElement compiles `<If when={c} else={…}>…</If>`.
func (l *lowering) ifElement(at int, props []prop, children []span) frag {
	attrs := l.flowProps(at, "If", props, "when", "else")
	when, ok := attrs["when"]
	var out frag
	if !ok {
		l.errorAt(at, "<If> needs `when`")
		out.gen("undefined", at)
		return out
	}
	cond := l.helper("cond")
	otherwise, hasElse := attrs["else"]
	// `{(user) => …}`: the child is called with an accessor of when's
	// value, read through it, once per read (Solid's <Show>).
	if len(children) == 1 && isFunction(children[0].e) {
		out.gen("(("+hole+"w) => "+cond+"("+hole+"w, () => (", at)
		out.add(l.copy(children[0]))
		out.gen(")("+hole+"w)", at)
		if hasElse {
			out.gen(", ", at)
			out.add(thunk(otherwise, at))
		}
		out.gen("))(", at)
		out.add(thunk(when, at))
		out.gen(")", at)
		return out
	}
	values := l.children(children, true)
	var then frag
	switch len(values) {
	case 0:
		then.gen("undefined", at)
	case 1:
		then = values[0]
	default:
		then.gen("[", at)
		then.add(join(values, ", ", at))
		then.gen("]", at)
	}
	out.gen(cond+"(", at)
	out.add(thunk(when, at))
	out.gen(", ", at)
	out.add(thunk(then, at))
	if hasElse {
		out.gen(", ", at)
		out.add(thunk(otherwise, at))
	}
	out.gen(")", at)
	return out
}

// eachElement compiles `<Each of={items} key={…}>{(item, index) => …}</Each>`.
func (l *lowering) eachElement(at int, props []prop, children []span) frag {
	attrs := l.flowProps(at, "Each", props, "of", "key")
	var out frag
	of, ok := attrs["of"]
	if !ok {
		l.errorAt(at, "<Each> needs `of`")
		out.gen("undefined", at)
		return out
	}
	if len(children) != 1 || l.isJSX(children[0].e) || isLiteral(children[0].e) {
		l.errorAt(at, "<Each> takes one child: a function rendering an item, `{(item, index) => …}`")
		out.gen("undefined", at)
		return out
	}
	out.gen(l.helper("each")+"(", at)
	out.add(thunk(of, at))
	out.gen(", ", at)
	out.add(l.copy(children[0]))
	if key, ok := attrs["key"]; ok {
		out.gen(", ", at)
		out.add(key)
	}
	out.gen(")", at)
	return out
}

// isDynamic reports whether an expression's value may change: it calls
// something or reads a member, outside nested functions. A member of a
// module (import_nsgo.View) is what NSH's source names directly, and
// does not count.
func (l *lowering) isDynamic(e ast.Expression) bool {
	dynamic := false
	walk(e, func(n ast.Node) bool {
		if dynamic {
			return false
		}
		switch n := n.(type) {
		case *ast.FunctionLiteral, *ast.ArrowFunctionLiteral:
			return false
		case *ast.CallExpression, *ast.NewExpression, *ast.BracketExpression, *ast.PrivateDotExpression:
			dynamic = true
		case *ast.DotExpression:
			if !l.isName(n) {
				dynamic = true
			}
		case *ast.TemplateLiteral:
			if n.Tag != nil {
				dynamic = true
			}
		}
		return !dynamic
	})
	return dynamic
}

// isName reports whether e names something: an identifier, or an import
// (import_nsgo.View).
func (l *lowering) isName(e ast.Expression) bool {
	switch e := e.(type) {
	case *ast.Identifier:
		return true
	case *ast.DotExpression:
		if id, ok := e.Left.(*ast.Identifier); ok {
			_, ok := l.modules[id.Name.String()]
			return ok
		}
	}
	return false
}

func isLiteral(e ast.Expression) bool {
	switch e.(type) {
	case *ast.StringLiteral, *ast.NumberLiteral, *ast.BooleanLiteral, *ast.NullLiteral, *ast.TemplateLiteral:
		return true
	}
	return false
}

func isFunction(e ast.Expression) bool {
	switch e.(type) {
	case *ast.FunctionLiteral, *ast.ArrowFunctionLiteral:
		return true
	}
	return false
}

// isEvent reports whether name is an event handler prop: onPress.
func isEvent(name string) bool {
	return len(name) > 2 && strings.HasPrefix(name, "on") && name[2] >= 'A' && name[2] <= 'Z'
}

// quote writes s as a JS string literal.
func quote(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}
