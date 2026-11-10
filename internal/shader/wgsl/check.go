package wgsl

import (
	"go/constant"
	"go/token"
	"strings"

	"github.com/oneo-me/nsgo/internal/shaderir"
)

// checker checks a module and writes it as the IR, one function at a
// time.
type checker struct {
	m *module
	// user is the length of the app's source, before the built-ins: what
	// lies past it may use the intrinsics and names starting with __.
	user   int
	prog   *shaderir.Program
	global *scope
	scope  *scope
	fns    []*fnInfo
	cur    *fnState
	// nsh is the IR uniform holding time, progress and overflow;
	// pictures how many textures the program reads.
	nsh      int
	pictures int
	uniforms map[string]uniformInfo
	// declared is whether the source declares struct Uniforms.
	declared bool
	members  []Uniform
}

type uniformInfo struct {
	index int
	t     ty
}

type symKind int

const (
	symLocal symKind = iota
	symConst
	symFn
	symType
	symBuiltin // nsh, uniforms
)

type symbol struct {
	kind    symKind
	t       ty
	local   int
	mutable bool
	// val is a const's value, or nil for one written again where it is
	// used (init, read in the module's scope).
	val  *value
	init expr
	fn   *fnInfo
	name string
}

type scope struct {
	parent *scope
	names  map[string]*symbol
}

func (s *scope) lookup(name string) *symbol {
	for ; s != nil; s = s.parent {
		if sym := s.names[name]; sym != nil {
			return sym
		}
	}
	return nil
}

func (c *checker) push() { c.scope = &scope{parent: c.scope, names: map[string]*symbol{}} }
func (c *checker) pop()  { c.scope = c.scope.parent }

func (c *checker) declare(name string, pos int, sym *symbol) {
	if strings.HasPrefix(name, "__") && pos < c.user {
		fail(pos, "a name starting with __ is reserved")
	}
	if _, ok := builtinTypes[name]; ok {
		fail(pos, "%s is a type", name)
	}
	if c.scope.names[name] != nil {
		fail(pos, "%s is declared twice", name)
	}
	if c.scope == c.global && builtin(name) != nil && pos < c.user {
		fail(pos, "%s is a built-in function", name)
	}
	sym.name = name
	c.scope.names[name] = sym
}

// fnInfo is a function's signature and what it reads, for the program's
// inputs and whether it animates.
type fnInfo struct {
	decl      *fnDecl
	index     int
	params    []ty
	ret       ty
	readsTime bool
	textures  uint8 // the pictures it samples, a bit each
	calls     []*fnInfo
}

// fnState is the function being written: its IR block, which holds all
// its locals (nested blocks hold none), and the loops and switches around
// the statement being written.
type fnState struct {
	info   *fnInfo
	params int
	top    *shaderir.Block
	blocks []*shaderir.Block
	flows  []*flow
}

// flowKind is what a break or a continue leaves, as the IR writes WGSL's
// loops and switches.
type flowKind int

const (
	// flowLoop is an IR loop that is the WGSL loop: break and continue
	// are the IR's.
	flowLoop flowKind = iota
	// flowBody is a one-pass IR loop around a loop's body that has a
	// continuing part: continue is the IR's break, and break sets brk and
	// breaks, so the loop around breaks too.
	flowBody
	// flowSwitch is a one-pass IR loop around a switch whose cases break:
	// break is the IR's, and continue sets cont and breaks, so what
	// follows the switch continues.
	flowSwitch
)

type flow struct {
	kind      flowKind
	brk, cont int
	contUsed  bool
}

func (c *checker) newLocal(t ty) int {
	f := c.cur
	f.top.LocalVars = append(f.top.LocalVars, t.ir())
	return f.params + len(f.top.LocalVars) - 1
}

// newCounter is a for loop's counter, which the loop declares.
func (c *checker) newCounter() int {
	f := c.cur
	f.top.LocalVars = append(f.top.LocalVars, shaderir.Type{})
	return f.params + len(f.top.LocalVars) - 1
}

func (c *checker) newBlock() *shaderir.Block {
	b := &shaderir.Block{}
	c.cur.blocks = append(c.cur.blocks, b)
	return b
}

func local(idx int, t ty) shaderir.Expr {
	return mk(shaderir.Expr{Type: shaderir.LocalVariable, Index: idx}, t)
}

func assign(lhs, rhs shaderir.Expr) shaderir.Stmt {
	return shaderir.Stmt{Type: shaderir.Assign, Exprs: []shaderir.Expr{lhs, rhs}}
}

// mk marks e of type t, as the writers read it: unsigned for u32.
func mk(e shaderir.Expr, t ty) shaderir.Expr {
	e.Unsigned = t.concrete().scalar() == kU32
	return e
}

// check checks the module and writes the program.
func (c *checker) check() {
	c.global = &scope{names: map[string]*symbol{}}
	c.scope = c.global
	c.uniforms = map[string]uniformInfo{}
	p := c.prog

	// The bridge's uniforms, which platforms set (package shader), then
	// the built-ins' and the app's.
	p.UniformNames = []string{"__rectSize", "__scale"}
	p.Uniforms = []shaderir.Type{{Main: shaderir.Vec2}, {Main: shaderir.Float}}
	p.Varyings = []shaderir.Type{{Main: shaderir.Vec2}}

	for _, a := range c.m.aliases {
		c.declare(a.name, a.pos, &symbol{kind: symType, t: c.resolveType(a.typ)})
	}
	for _, s := range c.m.structs {
		if s.name != "Uniforms" {
			fail(s.pos, "a shader declares no struct but Uniforms")
		}
		c.declared = true
		c.declare(s.name, s.pos, &symbol{kind: symType})
	}
	for _, g := range c.m.globals {
		switch {
		case g.kind == "var" && g.space == "private":
			fail(g.pos, "var<private> is not supported yet: declare the variable in the function")
		case g.kind == "var":
			if g.space == "uniform" || g.space == "storage" || g.space == "workgroup" || g.space == "" {
				fail(g.pos, "a shader declares no resources: it reads nsh and uniforms")
			}
			fail(g.pos, "the address space %s is not WGSL's", g.space)
		}
		if g.init == nil {
			fail(g.pos, "the const %s has no value", g.name)
		}
		c.declare(g.name, g.pos, c.constSymbol(g))
	}
	for _, f := range c.m.fns {
		for _, a := range f.attrs {
			switch a.name {
			case "vertex", "fragment", "compute":
				fail(a.pos, "a shader declares no entry points: it defines fn shade(uv: vec2f) -> vec4f")
			}
		}
		info := &fnInfo{decl: f, index: len(c.fns)}
		for _, prm := range f.params {
			t := c.resolveType(prm.typ)
			if t.k == kArray {
				fail(prm.pos, "a function takes no arrays")
			}
			info.params = append(info.params, t)
		}
		if f.ret != nil {
			info.ret = c.resolveType(f.ret)
			if info.ret.k == kArray {
				fail(f.ret.pos, "a function returns no arrays")
			}
		}
		c.fns = append(c.fns, info)
		c.declare(f.name, f.pos, &symbol{kind: symFn, fn: info})
	}

	c.uniformsOf()
	for _, a := range c.m.asserts {
		v := c.constant(a.x)
		if v.t.k != kBool {
			fail(a.pos, "const_assert takes a bool")
		}
		if !constant.BoolVal(v.c) {
			fail(a.pos, "the assertion fails")
		}
	}

	shade := c.global.names["shade"]
	if shade == nil || shade.kind != symFn {
		fail(c.user, "the shader defines no fn shade(uv: vec2f) -> vec4f")
	}
	if d := shade.fn; len(d.params) != 1 || !d.params[0].equal(vec(kF32, 2)) || !d.ret.equal(vec(kF32, 4)) {
		fail(d.decl.pos, "shade must be fn shade(uv: vec2f) -> vec4f")
	}

	for _, f := range c.fns {
		p.Funcs = append(p.Funcs, c.function(f))
	}
	c.recursion()

	entry := c.global.names["__nsh_fragment"].fn
	p.FragmentFunc.Block = &shaderir.Block{
		LocalVarIndexOffset: 1 + len(p.Varyings),
		Stmts: []shaderir.Stmt{{Type: shaderir.Return, Exprs: []shaderir.Expr{{
			Type: shaderir.Call,
			Exprs: []shaderir.Expr{
				{Type: shaderir.FunctionExpr, Index: entry.index},
				{Type: shaderir.LocalVariable, Index: 1},
			},
		}}}},
	}
}

// uniformsOf declares nsh and uniforms: the IR's uniforms after the
// bridge's, the pictures' sizes first where the program reads pictures,
// which the bridge's lanes then count (shader.BridgeLanes).
func (c *checker) uniformsOf() {
	// The textures a program reads are known only once its functions
	// are, so the image sizes go in now and leave when none is read.
	p := c.prog
	p.UniformNames = append(p.UniformNames, "__imageSizes")
	p.Uniforms = append(p.Uniforms, shaderir.Type{Main: shaderir.Array, Sub: []shaderir.Type{{Main: shaderir.Vec2}}, Length: maxPictures})
	c.nsh = len(p.Uniforms)
	p.UniformNames = append(p.UniformNames, "__nsh")
	p.Uniforms = append(p.Uniforms, shaderir.Type{Main: shaderir.Vec3})
	c.global.names["nsh"] = &symbol{kind: symBuiltin, name: "nsh"}
	if !c.declared {
		return
	}
	c.global.names["uniforms"] = &symbol{kind: symBuiltin, name: "uniforms"}
	var s *structDecl
	for _, d := range c.m.structs {
		s = d
	}
	offset := 0
	for _, mb := range s.members {
		t := c.resolveType(mb.typ)
		switch {
		case t.k == kF32, t.k == kI32, t.k == kU32:
		case t.k == kVec && t.elem == kF32:
		case t.k == kArray && t.arr.equal(vec(kF32, 4)):
		default:
			fail(mb.pos, "a uniform is f32, i32, u32, vec2f, vec3f, vec4f or array<vec4f, N>, not %s", t)
		}
		if _, ok := c.uniforms[mb.name]; ok {
			fail(mb.pos, "the uniform %s is declared twice", mb.name)
		}
		align, size := t.layout()
		offset = (offset+align-1)/align*align + size
		c.uniforms[mb.name] = uniformInfo{index: len(p.Uniforms), t: t}
		p.UniformNames = append(p.UniformNames, mb.name)
		p.Uniforms = append(p.Uniforms, t.ir())
		c.members = append(c.members, Uniform{Name: mb.name, Type: t.String(), Lanes: t.lanes(), Vec3: t.k == kVec && t.n == 3})
	}
	if size := (offset + 15) / 16 * 16; size > MaxUniformBytes {
		fail(s.pos, "the uniforms take %d bytes, more than %d", size, MaxUniformBytes)
	}
}

// constSymbol checks a const's value: a constant when it folds into one,
// else its expression, written again where it is used.
func (c *checker) constSymbol(d *declStmt) *symbol {
	if v, ok := c.tryConstant(d.init); ok {
		if d.typ != nil {
			v = c.convert(v, c.resolveType(d.typ), d.init.at())
		}
		return &symbol{kind: symConst, t: v.t, val: &v}
	}
	sym := &symbol{kind: symConst, init: d.init}
	if d.typ != nil {
		sym.t = c.resolveType(d.typ)
	}
	return sym
}

// tryConstant folds e into a constant, if it is one.
func (c *checker) tryConstant(e expr) (v value, ok bool) {
	err := catch(func() { v = c.constant(e) })
	return v, err == nil
}

// constant folds e, which must be a constant expression.
func (c *checker) constant(e expr) value {
	saved := c.cur
	c.cur = nil // no function: a constant writes no statements
	defer func() { c.cur = saved }()
	var out []shaderir.Stmt
	v := c.expr(e, &out)
	if v.c == nil && v.comps == nil {
		fail(e.at(), "the expression is not a constant")
	}
	return v
}

func (c *checker) resolveType(t *typeExpr) ty {
	if bt, ok := builtinTypes[t.name]; ok {
		if t.args != nil {
			fail(t.pos, "%s takes no template", t.name)
		}
		return bt
	}
	if sym := c.scope.lookup(t.name); sym != nil && sym.kind == symType && sym.t.k != kNone {
		return sym.t
	}
	switch t.name {
	case "vec2", "vec3", "vec4":
		if len(t.args) != 1 {
			fail(t.pos, "%s takes its components' type", t.name)
		}
		e := c.templateType(t.args[0], t.pos)
		if !e.isScalar() || e.abstract() {
			fail(t.pos, "a vector's components are bool, i32, u32 or f32")
		}
		return vec(e.k, int(t.name[3]-'0'))
	case "array":
		if len(t.args) != 2 {
			fail(t.pos, "array takes its elements' type and its length")
		}
		e := c.templateType(t.args[0], t.pos)
		n := c.templateLength(t.args[1], t.pos)
		return ty{k: kArray, n: n, arr: &e}
	case "f16":
		fail(t.pos, "f16 is not supported")
	case "Uniforms", "Nsh":
		fail(t.pos, "%s is a struct of the shader's inputs, not a type to use", t.name)
	case "ptr", "atomic", "sampler", "texture_2d":
		fail(t.pos, "a shader takes no %s", t.name)
	}
	if shape, ok := matShapes[t.name]; ok {
		if shape[0] != shape[1] {
			fail(t.pos, "only square matrices are supported")
		}
		if !strings.HasSuffix(t.name, "f") {
			if len(t.args) != 1 || !c.templateType(t.args[0], t.pos).equal(tF32) {
				fail(t.pos, "a matrix holds f32")
			}
		}
		return ty{k: kMat, elem: kF32, n: shape[0], rows: shape[1]}
	}
	fail(t.pos, "%s is not a type", t.name)
	return ty{}
}

func (c *checker) templateType(a any, pos int) ty {
	t, ok := a.(*typeExpr)
	if !ok {
		fail(pos, "the template takes a type")
	}
	return c.resolveType(t)
}

func (c *checker) templateLength(a any, pos int) int {
	var e expr
	switch x := a.(type) {
	case *typeExpr:
		e = &identExpr{name: x.name, pos: x.pos}
	case expr:
		e = x
	}
	v := c.constant(e)
	if !isInt(v.t.k) {
		fail(pos, "an array's length is an integer")
	}
	n, ok := constant.Int64Val(v.c)
	if !ok || n <= 0 || n > 1<<16 {
		fail(pos, "an array's length is from 1 to 65536")
	}
	return int(n)
}

// function writes f as an IR function.
func (c *checker) function(f *fnInfo) shaderir.Func {
	d := f.decl
	top := &shaderir.Block{LocalVarIndexOffset: len(f.params)}
	c.cur = &fnState{info: f, params: len(f.params), top: top}
	c.push()
	var in []shaderir.Type
	for i, prm := range d.params {
		c.declare(prm.name, prm.pos, &symbol{kind: symLocal, t: f.params[i], local: i})
		in = append(in, f.params[i].ir())
	}
	c.stmts(d.body.stmts, &top.Stmts)
	if f.ret.k != kNone && !returns(d.body.stmts) {
		fail(d.pos, "%s does not return a value on every path", d.name)
	}
	c.pop()
	for _, b := range c.cur.blocks {
		b.LocalVarIndexOffset = len(f.params) + len(top.LocalVars)
	}
	c.cur = nil
	return shaderir.Func{Index: f.index, InParams: in, Return: f.ret.ir(), Block: top}
}

// returns reports whether stmts end on every path, as WGSL's behaviors
// require of a function returning a value.
func returns(stmts []stmt) bool {
	if len(stmts) == 0 {
		return false
	}
	switch s := stmts[len(stmts)-1].(type) {
	case *returnStmt:
		return true
	case *branchStmt:
		return s.kind == "discard"
	case *block:
		return returns(s.stmts)
	case *ifStmt:
		if s.els == nil || !returns(s.then.stmts) {
			return false
		}
		return returns([]stmt{s.els})
	case *switchStmt:
		hasDefault := false
		for _, cs := range s.cases {
			hasDefault = hasDefault || cs.hasDefault
			if !returns(cs.body.stmts) {
				return false
			}
		}
		return hasDefault
	case *loopStmt:
		// A loop that never breaks ends only by returning.
		return s.breakIf == nil && !hasBranch(s.body.stmts, "break", false)
	}
	return false
}

// hasBranch reports whether stmts break or continue the loop or switch
// they are in, not one inside them: a continue in a switch is its loop's.
func hasBranch(stmts []stmt, kind string, inSwitch bool) bool {
	for _, st := range stmts {
		switch s := st.(type) {
		case *branchStmt:
			if s.kind == kind {
				return true
			}
		case *block:
			if hasBranch(s.stmts, kind, inSwitch) {
				return true
			}
		case *ifStmt:
			if hasBranch(s.then.stmts, kind, inSwitch) || s.els != nil && hasBranch([]stmt{s.els}, kind, inSwitch) {
				return true
			}
		case *switchStmt:
			if kind == "continue" {
				for _, cs := range s.cases {
					if hasBranch(cs.body.stmts, kind, true) {
						return true
					}
				}
			}
		}
	}
	return false
}

// recursion fails on a function that calls itself, which WGSL and the
// GPUs' languages forbid.
func (c *checker) recursion() {
	state := map[*fnInfo]int{}
	var visit func(f *fnInfo)
	visit = func(f *fnInfo) {
		switch state[f] {
		case 1:
			fail(f.decl.pos, "%s calls itself", f.decl.name)
		case 2:
			return
		}
		state[f] = 1
		for _, g := range f.calls {
			visit(g)
		}
		state[f] = 2
	}
	for _, f := range c.fns {
		visit(f)
	}
}

// reach returns what f and the functions it calls read: the time and the
// pictures.
func reach(f *fnInfo) (time bool, textures uint8) {
	seen := map[*fnInfo]bool{}
	var walk func(f *fnInfo)
	walk = func(f *fnInfo) {
		if seen[f] {
			return
		}
		seen[f] = true
		time = time || f.readsTime
		textures |= f.textures
		for _, g := range f.calls {
			walk(g)
		}
	}
	walk(f)
	return time, textures
}

func (c *checker) stmts(list []stmt, out *[]shaderir.Stmt) {
	for _, s := range list {
		c.stmt(s, out)
	}
}

func (c *checker) stmt(st stmt, out *[]shaderir.Stmt) {
	switch s := st.(type) {
	case *block:
		c.push()
		c.stmts(s.stmts, out)
		c.pop()
	case *declStmt:
		c.decl(s, out)
	case *assignStmt:
		c.assign(s, out)
	case *incStmt:
		lhs := c.ref(s.lhs, out)
		if !isInt(lhs.t.k) || !lhs.t.isScalar() {
			fail(s.pos, "%s takes an integer variable", s.op)
		}
		one := value{t: tAbsInt, c: constant.MakeInt64(1)}
		op := "+"
		if s.op == "--" {
			op = "-"
		}
		r := c.arith(op, lhs, one, s.pos, out)
		*out = append(*out, assign(lhs.e, c.ir(r)))
	case *callStmt:
		v := c.call(s.call, out, true)
		if v.t.k == kNone {
			*out = append(*out, shaderir.Stmt{Type: shaderir.ExprStmt, Exprs: []shaderir.Expr{v.e}})
		} else if v.e.Type == shaderir.Call && v.e.Exprs[0].Type == shaderir.FunctionExpr {
			*out = append(*out, shaderir.Stmt{Type: shaderir.ExprStmt, Exprs: []shaderir.Expr{v.e}})
		}
	case *ifStmt:
		c.ifStmt(s, out)
	case *returnStmt:
		f := c.cur.info
		if s.x == nil {
			if f.ret.k != kNone {
				fail(s.pos, "%s returns a %s", f.decl.name, f.ret)
			}
			*out = append(*out, shaderir.Stmt{Type: shaderir.Return})
			return
		}
		if f.ret.k == kNone {
			fail(s.pos, "%s returns no value", f.decl.name)
		}
		v := c.convert(c.expr(s.x, out), f.ret, s.x.at())
		*out = append(*out, shaderir.Stmt{Type: shaderir.Return, Exprs: []shaderir.Expr{c.ir(v)}})
	case *branchStmt:
		switch s.kind {
		case "break":
			c.emitBreak(s.pos, out)
		case "continue":
			c.emitContinue(s.pos, len(c.cur.flows)-1, out)
		case "discard":
			if c.cur.info.decl.name != "shade" {
				fail(s.pos, "discard is supported in shade only")
			}
			*out = append(*out, shaderir.Stmt{Type: shaderir.Discard})
		}
	case *forStmt:
		c.forStmt(s, out)
	case *whileStmt:
		c.push()
		c.loop(s.cond, s.body, nil, nil, out)
		c.pop()
	case *loopStmt:
		c.push()
		c.loop(nil, s.body, s.continuing, s.breakIf, out)
		c.pop()
	case *switchStmt:
		c.switchStmt(s, out)
	case *assertStmt:
		v := c.constant(s.x)
		if v.t.k != kBool || !constant.BoolVal(v.c) {
			fail(s.pos, "the assertion fails")
		}
	default:
		fail(st.at(), "the statement is not supported")
	}
}

func (c *checker) decl(s *declStmt, out *[]shaderir.Stmt) {
	switch s.kind {
	case "const":
		if s.init == nil {
			fail(s.pos, "the const %s has no value", s.name)
		}
		saved := c.scope
		sym := c.constSymbol(s)
		c.scope = saved
		if sym.val == nil {
			// A local const that does not fold reads the function's
			// scope where it is written.
			v := c.expr(s.init, out)
			if s.typ != nil {
				v = c.convert(v, sym.t, s.init.at())
			}
			v = c.concrete(v, s.init.at())
			idx := c.newLocal(v.t)
			*out = append(*out, assign(local(idx, v.t), c.ir(v)))
			c.declare(s.name, s.pos, &symbol{kind: symLocal, t: v.t, local: idx})
			return
		}
		c.declare(s.name, s.pos, sym)
	case "let":
		if s.init == nil {
			fail(s.pos, "let %s has no value", s.name)
		}
		v := c.expr(s.init, out)
		if s.typ != nil {
			v = c.convert(v, c.resolveType(s.typ), s.init.at())
		}
		v = c.concrete(v, s.init.at())
		if v.c != nil {
			// A constant stays one, of its type.
			c.declare(s.name, s.pos, &symbol{kind: symConst, t: v.t, val: &v})
			return
		}
		idx := c.newLocal(v.t)
		*out = append(*out, assign(local(idx, v.t), c.ir(v)))
		c.declare(s.name, s.pos, &symbol{kind: symLocal, t: v.t, local: idx})
	case "var":
		if s.space != "" && s.space != "function" {
			fail(s.pos, "a function's var is in the function's address space")
		}
		var t ty
		var v value
		if s.init != nil {
			v = c.expr(s.init, out)
		}
		switch {
		case s.typ != nil:
			t = c.resolveType(s.typ)
			if s.init != nil {
				v = c.convert(v, t, s.init.at())
			}
		case s.init != nil:
			v = c.concrete(v, s.init.at())
			t = v.t
		default:
			fail(s.pos, "var %s has neither a type nor a value", s.name)
		}
		if t.k == kNone {
			fail(s.pos, "var %s has no value", s.name)
		}
		idx := c.newLocal(t)
		if s.init != nil {
			*out = append(*out, assign(local(idx, t), c.ir(v)))
		} else {
			*out = append(*out, shaderir.Stmt{Type: shaderir.Init, InitIndex: idx})
		}
		c.declare(s.name, s.pos, &symbol{kind: symLocal, t: t, local: idx, mutable: true})
	}
}

func (c *checker) assign(s *assignStmt, out *[]shaderir.Stmt) {
	if s.lhs == nil {
		c.expr(s.rhs, out) // the phony assignment: checked, then dropped
		return
	}
	lhs := c.ref(s.lhs, out)
	rhs := c.expr(s.rhs, out)
	if s.op != "=" {
		op := strings.TrimSuffix(s.op, "=")
		read := lhs
		read.ref = false
		switch op {
		case "&", "|", "^", "<<", ">>":
			rhs = c.bitwise(op, read, rhs, s.pos, out)
		default:
			rhs = c.arith(op, read, rhs, s.pos, out)
		}
	}
	rhs = c.convert(rhs, lhs.t, s.rhs.at())
	r := c.ir(rhs)
	if lhs.fb {
		r = selectNumber(r, 1, 0)
	}
	*out = append(*out, assign(lhs.e, r))
}

// ref evaluates e as a variable to assign to.
func (c *checker) ref(e expr, out *[]shaderir.Stmt) value {
	root := e
	for {
		if m, ok := root.(*memberExpr); ok {
			root = m.x
		} else if i, ok := root.(*indexExpr); ok {
			root = i.x
		} else {
			break
		}
	}
	if id, ok := root.(*identExpr); ok {
		if sym := c.scope.lookup(id.name); sym != nil && sym.kind == symConst {
			fail(e.at(), "the value cannot be assigned: declare it with var")
		}
	}
	v := c.expr(e, out)
	if !v.ref {
		fail(e.at(), "the expression is not a variable")
	}
	if !v.mutable {
		fail(e.at(), "the value cannot be assigned: declare it with var")
	}
	return v
}

func (c *checker) cond(e expr, out *[]shaderir.Stmt) shaderir.Expr {
	v := c.expr(e, out)
	if v.t.k != kBool {
		fail(e.at(), "the condition is a %s, not a bool", v.t)
	}
	return c.ir(v)
}

func (c *checker) ifStmt(s *ifStmt, out *[]shaderir.Stmt) {
	cond := c.cond(s.cond, out)
	then := c.newBlock()
	c.push()
	c.stmts(s.then.stmts, &then.Stmts)
	c.pop()
	st := shaderir.Stmt{Type: shaderir.If, Exprs: []shaderir.Expr{cond}, Blocks: []*shaderir.Block{then}}
	if s.els != nil {
		els := c.newBlock()
		c.push()
		c.stmt(s.els, &els.Stmts)
		c.pop()
		st.Blocks = append(st.Blocks, els)
	}
	*out = append(*out, st)
}

func boolConst(b bool) shaderir.Expr {
	return shaderir.Expr{Type: shaderir.NumberExpr, Const: constant.MakeBool(b)}
}

func not(e shaderir.Expr) shaderir.Expr {
	return shaderir.Expr{Type: shaderir.Unary, Op: shaderir.NotOp, Exprs: []shaderir.Expr{e}}
}

// onePass is a loop of one pass around body, which a break leaves.
func (c *checker) onePass(body *shaderir.Block) shaderir.Stmt {
	return c.counted(body, shaderir.LessThanOp, 1)
}

// forever is a loop around body that only a break ends.
func (c *checker) forever(body *shaderir.Block) shaderir.Stmt {
	return c.counted(body, shaderir.GreaterThanEqualOp, 0)
}

func (c *checker) counted(body *shaderir.Block, op shaderir.Op, end int64) shaderir.Stmt {
	idx := c.newCounter()
	return shaderir.Stmt{
		Type:        shaderir.For,
		Blocks:      []*shaderir.Block{body},
		ForVarType:  shaderir.Type{Main: shaderir.Int},
		ForVarIndex: idx,
		ForInit:     constant.MakeInt64(0),
		ForEnd:      constant.MakeInt64(end),
		ForOp:       op,
		ForDelta:    constant.MakeInt64(1),
		Exprs: []shaderir.Expr{
			{Type: shaderir.NumberExpr, Const: constant.MakeInt64(0)},
			{Type: shaderir.NumberExpr, Const: constant.MakeInt64(end)},
		},
	}
}

func (c *checker) flag(out *[]shaderir.Stmt) int {
	idx := c.newLocal(tBool)
	*out = append(*out, assign(local(idx, tBool), boolConst(false)))
	return idx
}

func (c *checker) emitBreak(pos int, out *[]shaderir.Stmt) {
	flows := c.cur.flows
	if len(flows) == 0 {
		fail(pos, "break is not in a loop or a switch")
	}
	if f := flows[len(flows)-1]; f.kind == flowBody {
		*out = append(*out, assign(local(f.brk, tBool), boolConst(true)))
	}
	*out = append(*out, shaderir.Stmt{Type: shaderir.Break})
}

// emitContinue continues the loop of flows[:at+1].
func (c *checker) emitContinue(pos, at int, out *[]shaderir.Stmt) {
	flows := c.cur.flows
	for i := at; i >= 0; i-- {
		switch f := flows[i]; f.kind {
		case flowSwitch:
			if i < at {
				continue // only the innermost switch's break leaves it
			}
			f.contUsed = true
			*out = append(*out, assign(local(f.cont, tBool), boolConst(true)), shaderir.Stmt{Type: shaderir.Break})
			return
		case flowBody:
			*out = append(*out, shaderir.Stmt{Type: shaderir.Break})
			return
		case flowLoop:
			*out = append(*out, shaderir.Stmt{Type: shaderir.Continue})
			return
		}
	}
	fail(pos, "continue is not in a loop")
}

func (c *checker) forStmt(s *forStmt, out *[]shaderir.Stmt) {
	c.push()
	defer c.pop()
	if c.canonicalFor(s, out) {
		return
	}
	if s.init != nil {
		c.stmt(s.init, out)
	}
	var update *block
	if s.update != nil {
		update = &block{stmts: []stmt{s.update}, pos: s.update.at()}
	}
	c.loop(s.cond, s.body, update, nil, out)
}

// canonicalFor writes a for loop of a counter the body does not assign,
// stepping by a constant to a bound, as the IR's own loop, which every
// GPU language unrolls or runs at its best.
func (c *checker) canonicalFor(s *forStmt, out *[]shaderir.Stmt) bool {
	d, ok := s.init.(*declStmt)
	if !ok || d.kind != "var" || d.init == nil {
		return false
	}
	cond, ok := s.cond.(*binaryExpr)
	if !ok {
		return false
	}
	if x, ok := cond.x.(*identExpr); !ok || x.name != d.name {
		return false
	}
	var op shaderir.Op
	switch cond.op {
	case "<":
		op = shaderir.LessThanOp
	case "<=":
		op = shaderir.LessThanEqualOp
	case ">":
		op = shaderir.GreaterThanOp
	case ">=":
		op = shaderir.GreaterThanEqualOp
	case "!=":
		op = shaderir.NotEqualOp
	default:
		return false
	}
	var delta constant.Value
	switch u := s.update.(type) {
	case *incStmt:
		if x, ok := u.lhs.(*identExpr); !ok || x.name != d.name {
			return false
		}
		delta = constant.MakeInt64(1)
		if u.op == "--" {
			delta = constant.MakeInt64(-1)
		}
	case *assignStmt:
		if x, ok := u.lhs.(*identExpr); !ok || x.name != d.name || u.op != "+=" && u.op != "-=" {
			return false
		}
		v, ok := c.tryConstant(u.rhs)
		if !ok || v.c == nil || constant.Sign(v.c) == 0 {
			return false
		}
		delta = v.c
		if u.op == "-=" {
			delta = constant.UnaryOp(token.SUB, delta, 0)
		}
	default:
		return false
	}
	if assigns(s.body.stmts, d.name) {
		return false
	}
	// The bounds are written once, before the loop, unless they take
	// statements of their own.
	var pre []shaderir.Stmt
	init := c.expr(d.init, &pre)
	var t ty
	if d.typ != nil {
		t = c.resolveType(d.typ)
		init = c.convert(init, t, d.init.at())
	} else {
		init = c.concrete(init, d.init.at())
		t = init.t
	}
	if !t.isScalar() || !isNumeric(t.k) {
		return false
	}
	if isInt(t.k) {
		if delta.Kind() != constant.Int {
			return false
		}
	} else {
		delta = constant.ToFloat(delta)
	}
	idx := c.newCounter()
	c.declare(d.name, d.pos, &symbol{kind: symLocal, t: t, local: idx})
	var endPre []shaderir.Stmt
	end := c.convert(c.expr(cond.y, &endPre), t, cond.y.at())
	if len(pre) > 0 || len(endPre) > 0 {
		// Statements before a bound would run once, so the loop is
		// written in general.
		return c.retreat(s, out)
	}
	body := c.newBlock()
	c.cur.flows = append(c.cur.flows, &flow{kind: flowLoop})
	c.push()
	c.stmts(s.body.stmts, &body.Stmts)
	c.pop()
	c.cur.flows = c.cur.flows[:len(c.cur.flows)-1]
	*out = append(*out, shaderir.Stmt{
		Type:        shaderir.For,
		Blocks:      []*shaderir.Block{body},
		ForVarType:  t.ir(),
		ForVarIndex: idx,
		ForOp:       op,
		ForDelta:    delta,
		Exprs:       []shaderir.Expr{c.ir(init), c.ir(end)},
	})
	return true
}

// retreat writes a for loop canonicalFor began in general, in a scope of
// its own.
func (c *checker) retreat(s *forStmt, out *[]shaderir.Stmt) bool {
	c.pop()
	c.push()
	c.stmt(s.init, out)
	update := &block{stmts: []stmt{s.update}, pos: s.update.at()}
	c.loop(s.cond, s.body, update, nil, out)
	return true
}

// assigns reports whether stmts assign name.
func assigns(stmts []stmt, name string) bool {
	is := func(e expr) bool {
		for {
			switch x := e.(type) {
			case *identExpr:
				return x.name == name
			case *memberExpr:
				e = x.x
			case *indexExpr:
				e = x.x
			default:
				return false
			}
		}
	}
	for _, st := range stmts {
		switch s := st.(type) {
		case *assignStmt:
			if s.lhs != nil && is(s.lhs) {
				return true
			}
		case *incStmt:
			if is(s.lhs) {
				return true
			}
		case *declStmt:
			if s.name == name {
				return false // shadowed from here
			}
		case *block:
			if assigns(s.stmts, name) {
				return true
			}
		case *ifStmt:
			if assigns(s.then.stmts, name) || s.els != nil && assigns([]stmt{s.els}, name) {
				return true
			}
		case *forStmt:
			var head []stmt
			if s.init != nil {
				head = append(head, s.init)
			}
			if s.update != nil {
				head = append(head, s.update)
			}
			if assigns(append(head, s.body.stmts...), name) {
				return true
			}
		case *whileStmt:
			if assigns(s.body.stmts, name) {
				return true
			}
		case *loopStmt:
			if assigns(s.body.stmts, name) || s.continuing != nil && assigns(s.continuing.stmts, name) {
				return true
			}
		case *switchStmt:
			for _, cs := range s.cases {
				if assigns(cs.body.stmts, name) {
					return true
				}
			}
		}
	}
	return false
}

// loop writes a while loop, a loop or a for loop that is not canonical:
// an IR loop only breaks end, which checks cond first and runs update (a
// for loop's, or a loop's continuing part with its break if) last. A
// continue that must still run update leaves a one-pass loop around the
// body instead.
func (c *checker) loop(cond expr, body *block, update *block, breakIf expr, out *[]shaderir.Stmt) {
	outer := c.newBlock()
	if cond != nil {
		var pre []shaderir.Stmt
		x := c.cond(cond, &pre)
		outer.Stmts = append(outer.Stmts, pre...)
		brk := c.newBlock()
		brk.Stmts = []shaderir.Stmt{{Type: shaderir.Break}}
		outer.Stmts = append(outer.Stmts, shaderir.Stmt{Type: shaderir.If, Exprs: []shaderir.Expr{not(x)}, Blocks: []*shaderir.Block{brk}})
	}
	tail := update != nil || breakIf != nil
	c.cur.flows = append(c.cur.flows, &flow{kind: flowLoop})
	// The body's scope stays open for the continuing part, which reads
	// its declarations.
	c.push()
	if tail && hasBranch(body.stmts, "continue", false) {
		f := &flow{kind: flowBody}
		f.brk = c.flag(&outer.Stmts)
		inner := c.newBlock()
		c.cur.flows = append(c.cur.flows, f)
		c.stmts(body.stmts, &inner.Stmts)
		c.cur.flows = c.cur.flows[:len(c.cur.flows)-1]
		outer.Stmts = append(outer.Stmts, c.onePass(inner))
		brk := c.newBlock()
		brk.Stmts = []shaderir.Stmt{{Type: shaderir.Break}}
		outer.Stmts = append(outer.Stmts, shaderir.Stmt{Type: shaderir.If, Exprs: []shaderir.Expr{local(f.brk, tBool)}, Blocks: []*shaderir.Block{brk}})
	} else {
		c.stmts(body.stmts, &outer.Stmts)
	}
	if update != nil {
		c.stmts(update.stmts, &outer.Stmts)
	}
	if breakIf != nil {
		x := c.cond(breakIf, &outer.Stmts)
		brk := c.newBlock()
		brk.Stmts = []shaderir.Stmt{{Type: shaderir.Break}}
		outer.Stmts = append(outer.Stmts, shaderir.Stmt{Type: shaderir.If, Exprs: []shaderir.Expr{x}, Blocks: []*shaderir.Block{brk}})
	}
	c.pop()
	c.cur.flows = c.cur.flows[:len(c.cur.flows)-1]
	*out = append(*out, c.forever(outer))
}

func (c *checker) switchStmt(s *switchStmt, out *[]shaderir.Stmt) {
	x := c.concrete(c.expr(s.x, out), s.x.at())
	if !x.t.isScalar() || (x.t.k != kI32 && x.t.k != kU32) {
		fail(s.x.at(), "a switch takes an i32 or a u32, not a %s", x.t)
	}
	x = c.reuse(x, out)
	wrap := false
	for _, cs := range s.cases {
		wrap = wrap || hasBranch(cs.body.stmts, "break", false)
	}
	var f *flow
	if wrap {
		f = &flow{kind: flowSwitch}
		f.cont = c.flag(out)
		c.cur.flows = append(c.cur.flows, f)
	}
	// The cases as an if chain, the default last.
	type item struct {
		match shaderir.Expr
		body  *block
	}
	var items []item
	var deflt *block
	seen := map[string]bool{}
	for _, cs := range s.cases {
		if cs.hasDefault {
			if deflt != nil {
				fail(s.pos, "the switch has two defaults")
			}
			deflt = cs.body
		}
		var match shaderir.Expr
		for i, sel := range cs.sels {
			v := c.convert(c.constant(sel), x.t, sel.at())
			if key := v.c.ExactString(); seen[key] {
				fail(sel.at(), "the case %s is repeated", key)
			} else {
				seen[key] = true
			}
			eq := shaderir.Expr{Type: shaderir.Binary, Op: shaderir.EqualOp, Exprs: []shaderir.Expr{c.ir(x), c.ir(v)}}
			if i == 0 {
				match = eq
			} else {
				match = shaderir.Expr{Type: shaderir.Binary, Op: shaderir.OrOr, Exprs: []shaderir.Expr{match, eq}}
			}
		}
		if !cs.hasDefault {
			items = append(items, item{match, cs.body})
		}
	}
	var chain []shaderir.Stmt
	var build func(i int, dst *[]shaderir.Stmt)
	build = func(i int, dst *[]shaderir.Stmt) {
		if i == len(items) {
			if deflt != nil {
				c.push()
				c.stmts(deflt.stmts, dst)
				c.pop()
			}
			return
		}
		body := c.newBlock()
		c.push()
		c.stmts(items[i].body.stmts, &body.Stmts)
		c.pop()
		st := shaderir.Stmt{Type: shaderir.If, Exprs: []shaderir.Expr{items[i].match}, Blocks: []*shaderir.Block{body}}
		if i+1 < len(items) || deflt != nil {
			els := c.newBlock()
			build(i+1, &els.Stmts)
			st.Blocks = append(st.Blocks, els)
		}
		*dst = append(*dst, st)
	}
	build(0, &chain)
	if !wrap {
		*out = append(*out, chain...)
		return
	}
	c.cur.flows = c.cur.flows[:len(c.cur.flows)-1]
	inner := c.newBlock()
	inner.Stmts = chain
	*out = append(*out, c.onePass(inner))
	if f.contUsed {
		then := c.newBlock()
		c.emitContinue(s.pos, len(c.cur.flows)-1, &then.Stmts)
		*out = append(*out, shaderir.Stmt{Type: shaderir.If, Exprs: []shaderir.Expr{local(f.cont, tBool)}, Blocks: []*shaderir.Block{then}})
	}
}
