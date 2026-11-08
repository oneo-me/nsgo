package wgsl

import "slices"

// The syntax tree: what parse reads, with each node's byte offset in the
// source, which check turns into the IR.

type expr interface{ at() int }

type (
	identExpr struct {
		name string
		pos  int
	}
	litExpr   struct{ tok lexeme }
	unaryExpr struct {
		op  string
		x   expr
		pos int
	}
	binaryExpr struct {
		op   string
		x, y expr
		pos  int
	}
	// callExpr calls a function or a type's constructor: fn names it,
	// with the template a type or bitcast takes.
	callExpr struct {
		fn   *typeExpr
		args []expr
	}
	memberExpr struct {
		x    expr
		name string
		pos  int
	}
	indexExpr struct {
		x, i expr
		pos  int
	}
)

func (e *identExpr) at() int  { return e.pos }
func (e *litExpr) at() int    { return e.tok.pos }
func (e *unaryExpr) at() int  { return e.pos }
func (e *binaryExpr) at() int { return e.pos }
func (e *callExpr) at() int   { return e.fn.pos }
func (e *memberExpr) at() int { return e.pos }
func (e *indexExpr) at() int  { return e.pos }

// typeExpr names a type, with its template's types and expressions
// (vec3<f32>, array<vec4f, 4>).
type typeExpr struct {
	name string
	args []any // *typeExpr or expr
	pos  int
}

type stmt interface{ at() int }

type (
	declStmt struct {
		kind string // let, var or const
		// space is a module-scope var's address space (var<private>).
		space string
		name  string
		typ   *typeExpr
		init  expr
		pos   int
	}
	assignStmt struct {
		lhs expr // nil for _ =
		op  string
		rhs expr
		pos int
	}
	incStmt struct {
		lhs expr
		op  string
		pos int
	}
	callStmt struct{ call *callExpr }
	ifStmt   struct {
		cond expr
		then *block
		els  stmt // *ifStmt, *block or nil
		pos  int
	}
	forStmt struct {
		init   stmt
		cond   expr
		update stmt
		body   *block
		pos    int
	}
	whileStmt struct {
		cond expr
		body *block
		pos  int
	}
	loopStmt struct {
		body       *block
		continuing *block
		breakIf    expr
		pos        int
	}
	switchStmt struct {
		x     expr
		cases []switchCase
		pos   int
	}
	returnStmt struct {
		x   expr
		pos int
	}
	branchStmt struct {
		kind string // break, continue or discard
		pos  int
	}
	block struct {
		stmts []stmt
		pos   int
	}
	assertStmt struct {
		x   expr
		pos int
	}
)

type switchCase struct {
	sels       []expr
	hasDefault bool
	body       *block
}

func (s *declStmt) at() int   { return s.pos }
func (s *assignStmt) at() int { return s.pos }
func (s *incStmt) at() int    { return s.pos }
func (s *callStmt) at() int   { return s.call.at() }
func (s *ifStmt) at() int     { return s.pos }
func (s *forStmt) at() int    { return s.pos }
func (s *whileStmt) at() int  { return s.pos }
func (s *loopStmt) at() int   { return s.pos }
func (s *switchStmt) at() int { return s.pos }
func (s *returnStmt) at() int { return s.pos }
func (s *branchStmt) at() int { return s.pos }
func (s *block) at() int      { return s.pos }
func (s *assertStmt) at() int { return s.pos }

type (
	param struct {
		name string
		typ  *typeExpr
		pos  int
	}
	fnDecl struct {
		name   string
		params []param
		ret    *typeExpr
		body   *block
		attrs  []attr
		pos    int
	}
	member struct {
		name string
		typ  *typeExpr
		pos  int
	}
	structDecl struct {
		name    string
		members []member
		pos     int
	}
	aliasDecl struct {
		name string
		typ  *typeExpr
		pos  int
	}
	attr struct {
		name string
		pos  int
	}
	module struct {
		fns     []*fnDecl
		structs []*structDecl
		globals []*declStmt
		aliases []*aliasDecl
		asserts []*assertStmt
		// order is the declarations as they come, for names declared once.
		order []any
	}
)

type parser struct {
	toks []lexeme
	i    int
}

func parse(src string) (*module, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	m := &module{}
	err = catch(func() {
		for p.peek().kind != tEOF {
			p.topLevel(m)
		}
	})
	return m, err
}

// bail stops parsing with an error at pos, which parse returns.
type bail struct{ err *Error }

func catch(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(bail)
			if !ok {
				panic(r)
			}
			err = b.err
		}
	}()
	f()
	return nil
}

func fail(pos int, format string, args ...any) {
	panic(bail{errorAt(pos, format, args...)})
}

func (p *parser) peek() lexeme { return p.toks[p.i] }

func (p *parser) peekAt(n int) lexeme {
	if p.i+n < len(p.toks) {
		return p.toks[p.i+n]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) next() lexeme {
	t := p.toks[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

func (p *parser) is(text string) bool {
	t := p.peek()
	return (t.kind == tPunct || t.kind == tIdent) && t.text == text
}

func (p *parser) accept(text string) bool {
	if p.is(text) {
		p.i++
		return true
	}
	return false
}

func (p *parser) expect(text string) lexeme {
	t := p.peek()
	if !p.accept(text) {
		fail(t.pos, "expected %s, found %s", quote(text), describe(t))
	}
	return t
}

func (p *parser) ident() lexeme {
	t := p.next()
	if t.kind != tIdent {
		fail(t.pos, "expected a name, found %s", describe(t))
	}
	if reserved[t.text] {
		fail(t.pos, "%s is a reserved word", t.text)
	}
	return t
}

func quote(s string) string { return "'" + s + "'" }

func describe(t lexeme) string {
	switch t.kind {
	case tEOF:
		return "the end of the source"
	case tIdent, tPunct:
		return quote(t.text)
	}
	return "the number " + t.text + t.suffix
}

var reserved = map[string]bool{
	"let": true, "var": true, "const": true, "fn": true, "struct": true, "return": true, "if": true, "else": true,
	"for": true, "while": true, "loop": true, "break": true, "continue": true, "continuing": true, "switch": true,
	"case": true, "default": true, "discard": true, "true": true, "false": true, "alias": true, "override": true,
	"enable": true, "requires": true, "diagnostic": true, "const_assert": true,
}

func (p *parser) attrs() []attr {
	var out []attr
	for p.is("@") {
		at := p.next()
		name := p.next()
		if name.kind != tIdent {
			fail(name.pos, "expected an attribute's name, found %s", describe(name))
		}
		out = append(out, attr{name: name.text, pos: at.pos})
		if p.accept("(") {
			for depth := 1; depth > 0; {
				t := p.next()
				switch {
				case t.kind == tEOF:
					fail(t.pos, "the attribute's arguments are not closed")
				case t.text == "(" && t.kind == tPunct:
					depth++
				case t.text == ")" && t.kind == tPunct:
					depth--
				}
			}
		}
	}
	return out
}

func (p *parser) topLevel(m *module) {
	if p.accept(";") {
		return
	}
	attrs := p.attrs()
	t := p.peek()
	switch {
	case p.is("fn"):
		f := p.fn()
		f.attrs = attrs
		m.fns = append(m.fns, f)
		m.order = append(m.order, f)
		return
	case p.is("enable"), p.is("requires"), p.is("diagnostic"):
		fail(t.pos, "a shader takes no %s directive", t.text)
	}
	if len(attrs) > 0 {
		a := attrs[0]
		switch a.name {
		case "group", "binding":
			fail(a.pos, "a shader declares no resources: it reads nsh and uniforms")
		}
		fail(a.pos, "the attribute @%s does not apply here", a.name)
	}
	switch {
	case p.is("struct"):
		s := p.structDecl()
		m.structs = append(m.structs, s)
		m.order = append(m.order, s)
	case p.is("alias"):
		p.next()
		name := p.ident()
		p.expect("=")
		a := &aliasDecl{name: name.text, typ: p.typ(), pos: name.pos}
		p.expect(";")
		m.aliases = append(m.aliases, a)
		m.order = append(m.order, a)
	case p.is("const_assert"):
		p.next()
		a := &assertStmt{x: p.expr(), pos: t.pos}
		p.expect(";")
		m.asserts = append(m.asserts, a)
	case p.is("const"), p.is("var"), p.is("override"), p.is("let"):
		if p.is("override") {
			fail(t.pos, "a shader declares no overrides: set its values through uniforms")
		}
		if p.is("let") {
			fail(t.pos, "let declares values in functions only; declare a const")
		}
		d := p.decl()
		p.expect(";")
		m.globals = append(m.globals, d)
		m.order = append(m.order, d)
	default:
		fail(t.pos, "expected a declaration, found %s", describe(t))
	}
}

func (p *parser) fn() *fnDecl {
	p.expect("fn")
	name := p.ident()
	f := &fnDecl{name: name.text, pos: name.pos}
	p.expect("(")
	for !p.is(")") {
		p.attrs()
		pn := p.ident()
		p.expect(":")
		f.params = append(f.params, param{name: pn.text, typ: p.typ(), pos: pn.pos})
		if !p.accept(",") {
			break
		}
	}
	p.expect(")")
	if p.accept("->") {
		p.attrs()
		f.ret = p.typ()
	}
	f.body = p.block()
	return f
}

func (p *parser) structDecl() *structDecl {
	p.expect("struct")
	name := p.ident()
	s := &structDecl{name: name.text, pos: name.pos}
	p.expect("{")
	for !p.is("}") {
		p.attrs()
		mn := p.ident()
		p.expect(":")
		s.members = append(s.members, member{name: mn.text, typ: p.typ(), pos: mn.pos})
		if !p.accept(",") {
			break
		}
	}
	p.expect("}")
	p.accept(";")
	return s
}

// templateNames take a template list after their name in an expression.
var templateNames = map[string]bool{
	"vec2": true, "vec3": true, "vec4": true, "array": true, "bitcast": true, "ptr": true, "atomic": true,
	"mat2x2": true, "mat2x3": true, "mat2x4": true, "mat3x2": true, "mat3x3": true, "mat3x4": true,
	"mat4x2": true, "mat4x3": true, "mat4x4": true, "texture_2d": true, "sampler": false,
}

func (p *parser) typ() *typeExpr {
	name := p.ident()
	t := &typeExpr{name: name.text, pos: name.pos}
	if p.is("<") {
		t.args = p.template()
	}
	return t
}

// template reads a template list: types, or expressions where a type
// cannot be (an array's length).
func (p *parser) template() []any {
	p.expect("<")
	var args []any
	for {
		if t := p.peek(); t.kind == tIdent && !reserved[t.text] && !p.isCallAfterIdent() && t.text != "true" && t.text != "false" {
			args = append(args, p.typ())
		} else {
			args = append(args, p.binary(precShift))
		}
		if !p.accept(",") {
			break
		}
		if p.isTemplateEnd() {
			break
		}
	}
	p.closeTemplate()
	return args
}

// isCallAfterIdent reports whether the identifier at the cursor is called
// (a function in an array's length), not a type.
func (p *parser) isCallAfterIdent() bool { return p.peekAt(1).text == "(" }

func (p *parser) isTemplateEnd() bool {
	t := p.peek()
	return t.kind == tPunct && (t.text == ">" || t.text == ">>" || t.text == ">=" || t.text == ">>=")
}

// closeTemplate takes the '>' that ends a template list, leaving the rest
// of a '>>' or '>=' the lexer read as one token.
func (p *parser) closeTemplate() {
	t := p.peek()
	if t.kind != tPunct || t.text == "" || t.text[0] != '>' {
		fail(t.pos, "expected '>' closing the template, found %s", describe(t))
	}
	if t.text == ">" {
		p.i++
		return
	}
	rest := lexeme{kind: tPunct, text: t.text[1:], pos: t.pos + 1}
	p.toks = slices.Insert(p.toks, p.i+1, rest)
	p.i++
}

func (p *parser) block() *block {
	t := p.expect("{")
	b := &block{pos: t.pos}
	for !p.is("}") {
		if p.peek().kind == tEOF {
			fail(p.peek().pos, "the block is not closed")
		}
		if s := p.stmt(); s != nil {
			b.stmts = append(b.stmts, s)
		}
	}
	p.expect("}")
	return b
}

func (p *parser) stmt() stmt {
	p.attrs()
	t := p.peek()
	switch {
	case p.accept(";"):
		return nil
	case p.is("{"):
		return p.block()
	case p.is("return"):
		p.next()
		s := &returnStmt{pos: t.pos}
		if !p.is(";") {
			s.x = p.expr()
		}
		p.expect(";")
		return s
	case p.is("if"):
		return p.ifStmt()
	case p.is("for"):
		p.next()
		s := &forStmt{pos: t.pos}
		p.expect("(")
		if !p.is(";") {
			s.init = p.simple()
		}
		p.expect(";")
		if !p.is(";") {
			s.cond = p.expr()
		}
		p.expect(";")
		if !p.is(")") {
			s.update = p.simple()
		}
		p.expect(")")
		s.body = p.block()
		return s
	case p.is("while"):
		p.next()
		s := &whileStmt{pos: t.pos, cond: p.expr()}
		s.body = p.block()
		return s
	case p.is("loop"):
		p.next()
		return p.loop(t.pos)
	case p.is("switch"):
		return p.switchStmt()
	case p.is("break"), p.is("continue"), p.is("discard"):
		p.next()
		if t.text == "break" && p.is("if") {
			fail(t.pos, "break if ends a continuing block only")
		}
		p.expect(";")
		return &branchStmt{kind: t.text, pos: t.pos}
	case p.is("const_assert"):
		p.next()
		s := &assertStmt{x: p.expr(), pos: t.pos}
		p.expect(";")
		return s
	case p.is("continuing"):
		fail(t.pos, "continuing comes last in a loop's body only")
	}
	s := p.simple()
	p.expect(";")
	return s
}

// simple reads a declaration, an assignment, an increment or a call: a
// statement a for loop's header takes.
func (p *parser) simple() stmt {
	t := p.peek()
	switch {
	case p.is("let"), p.is("var"), p.is("const"):
		return p.decl()
	case p.is("_"):
		p.next()
		p.expect("=")
		return &assignStmt{op: "=", rhs: p.expr(), pos: t.pos}
	}
	lhs := p.unary()
	op := p.peek()
	if op.kind == tPunct {
		switch op.text {
		case "=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=":
			p.next()
			return &assignStmt{lhs: lhs, op: op.text, rhs: p.expr(), pos: t.pos}
		case "++", "--":
			p.next()
			return &incStmt{lhs: lhs, op: op.text, pos: t.pos}
		}
	}
	if c, ok := lhs.(*callExpr); ok {
		return &callStmt{call: c}
	}
	fail(t.pos, "the expression is evaluated but not used")
	return nil
}

func (p *parser) decl() *declStmt {
	t := p.next()
	d := &declStmt{kind: t.text, pos: t.pos}
	if t.text == "var" && p.is("<") {
		p.next()
		d.space = p.ident().text
		if p.accept(",") {
			p.ident()
		}
		p.closeTemplate()
	}
	name := p.ident()
	d.name, d.pos = name.text, name.pos
	if p.accept(":") {
		d.typ = p.typ()
	}
	if p.accept("=") {
		d.init = p.expr()
	}
	return d
}

func (p *parser) ifStmt() *ifStmt {
	t := p.expect("if")
	s := &ifStmt{cond: p.expr(), pos: t.pos}
	s.then = p.block()
	if p.accept("else") {
		if p.is("if") {
			s.els = p.ifStmt()
		} else {
			s.els = p.block()
		}
	}
	return s
}

func (p *parser) loop(pos int) *loopStmt {
	open := p.expect("{")
	s := &loopStmt{pos: pos, body: &block{pos: open.pos}}
	for !p.is("}") {
		if p.peek().kind == tEOF {
			fail(p.peek().pos, "the loop is not closed")
		}
		if p.is("continuing") {
			p.next()
			c := p.expect("{")
			s.continuing = &block{pos: c.pos}
			for !p.is("}") {
				if p.peek().kind == tEOF {
					fail(p.peek().pos, "the continuing block is not closed")
				}
				if p.is("break") && p.peekAt(1).text == "if" {
					p.next()
					p.next()
					s.breakIf = p.expr()
					p.expect(";")
					if !p.is("}") {
						fail(p.peek().pos, "break if ends its continuing block")
					}
					break
				}
				if st := p.stmt(); st != nil {
					s.continuing.stmts = append(s.continuing.stmts, st)
				}
			}
			p.expect("}")
			if !p.is("}") {
				fail(p.peek().pos, "continuing comes last in a loop's body")
			}
			break
		}
		if st := p.stmt(); st != nil {
			s.body.stmts = append(s.body.stmts, st)
		}
	}
	p.expect("}")
	return s
}

func (p *parser) switchStmt() *switchStmt {
	t := p.expect("switch")
	s := &switchStmt{x: p.expr(), pos: t.pos}
	p.attrs()
	p.expect("{")
	for !p.is("}") {
		var c switchCase
		switch {
		case p.accept("default"):
			c.hasDefault = true
		case p.accept("case"):
			for {
				if p.accept("default") {
					c.hasDefault = true
				} else {
					c.sels = append(c.sels, p.expr())
				}
				if !p.accept(",") || p.is(":") || p.is("{") {
					break
				}
			}
		default:
			fail(p.peek().pos, "expected case or default, found %s", describe(p.peek()))
		}
		p.accept(":")
		c.body = p.block()
		s.cases = append(s.cases, c)
	}
	p.expect("}")
	return s
}

// Operator precedence, loosest first.
const (
	precOrOr = iota
	precAndAnd
	precOr
	precXor
	precAnd
	precRelational
	precShift
	precAdditive
	precMultiplicative
)

var binaryPrec = map[string]int{
	"||": precOrOr, "&&": precAndAnd, "|": precOr, "^": precXor, "&": precAnd,
	"==": precRelational, "!=": precRelational, "<": precRelational, "<=": precRelational, ">": precRelational, ">=": precRelational,
	"<<": precShift, ">>": precShift, "+": precAdditive, "-": precAdditive, "*": precMultiplicative, "/": precMultiplicative, "%": precMultiplicative,
}

func (p *parser) expr() expr { return p.binary(precOrOr) }

func (p *parser) binary(min int) expr {
	x := p.unary()
	for {
		t := p.peek()
		prec, ok := binaryPrec[t.text]
		if t.kind != tPunct || !ok || prec < min {
			return x
		}
		p.next()
		y := p.binary(prec + 1)
		x = &binaryExpr{op: t.text, x: x, y: y, pos: t.pos}
	}
}

func (p *parser) unary() expr {
	t := p.peek()
	if t.kind == tPunct {
		switch t.text {
		case "-", "!", "~":
			p.next()
			return &unaryExpr{op: t.text, x: p.unary(), pos: t.pos}
		case "&", "*":
			fail(t.pos, "a shader takes no pointers")
		}
	}
	return p.postfix(p.primary())
}

func (p *parser) primary() expr {
	t := p.peek()
	switch t.kind {
	case tInt, tFloat:
		p.next()
		return &litExpr{tok: t}
	case tIdent:
		if t.text == "true" || t.text == "false" {
			p.next()
			return &litExpr{tok: t}
		}
		p.next()
		fn := &typeExpr{name: t.text, pos: t.pos}
		if templateNames[t.text] && p.is("<") {
			fn.args = p.template()
		}
		if p.is("(") {
			return &callExpr{fn: fn, args: p.args()}
		}
		if fn.args != nil {
			fail(t.pos, "the type %s is not called", t.text)
		}
		if reserved[t.text] {
			fail(t.pos, "expected an expression, found %s", describe(t))
		}
		return &identExpr{name: t.text, pos: t.pos}
	case tPunct:
		if t.text == "(" {
			p.next()
			x := p.expr()
			p.expect(")")
			return x
		}
	}
	fail(t.pos, "expected an expression, found %s", describe(t))
	return nil
}

func (p *parser) args() []expr {
	p.expect("(")
	var args []expr
	for !p.is(")") {
		args = append(args, p.expr())
		if !p.accept(",") {
			break
		}
	}
	p.expect(")")
	return args
}

func (p *parser) postfix(x expr) expr {
	for {
		t := p.peek()
		switch {
		case p.accept("."):
			name := p.next()
			if name.kind != tIdent {
				fail(name.pos, "expected a member's name, found %s", describe(name))
			}
			x = &memberExpr{x: x, name: name.text, pos: name.pos}
		case p.accept("["):
			i := p.expr()
			p.expect("]")
			x = &indexExpr{x: x, i: i, pos: t.pos}
		default:
			return x
		}
	}
}
