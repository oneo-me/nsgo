package richeditor

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ContentMatch is a state of the automaton a content expression compiles
// to, ProseMirror's: the node types it may go on with, and whether the
// content may end here.
type ContentMatch struct {
	ValidEnd  bool
	next      []matchEdge
	wrapCache []wrapEntry
}

type matchEdge struct {
	typ  *NodeType
	next *ContentMatch
}

type wrapEntry struct {
	target *NodeType
	wraps  []*NodeType
}

var emptyMatch = &ContentMatch{ValidEnd: true}

// exprToken reads a content expression, ProseMirror's TokenStream.
type exprStream struct {
	tokens []string
	pos    int
	types  map[string]*NodeType
	expr   string
}

var exprTokens = regexp.MustCompile(`\w+|\W`)

func (s *exprStream) next() string {
	if s.pos < len(s.tokens) {
		return s.tokens[s.pos]
	}
	return ""
}

func (s *exprStream) eat(tok string) bool {
	if s.next() == tok {
		s.pos++
		return true
	}
	return false
}

// expr is a parsed expression: its kind and parts.
type expr struct {
	kind     string // "choice", "seq", "plus", "star", "opt", "range", "name"
	exprs    []*expr
	sub      *expr
	min, max int
	value    *NodeType
}

type exprError struct{ msg string }

func parseContentMatch(s string, types map[string]*NodeType) (m *ContentMatch, err error) {
	if strings.TrimSpace(s) == "" {
		return emptyMatch, nil
	}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(exprError)
			if !ok {
				panic(r)
			}
			err = fmt.Errorf("%s (in content expression %q)", e.msg, s)
		}
	}()
	var tokens []string
	for _, t := range exprTokens.FindAllString(s, -1) {
		if strings.TrimSpace(t) != "" {
			tokens = append(tokens, t)
		}
	}
	stream := &exprStream{tokens: tokens, types: types, expr: s}
	e := parseExpr(stream)
	if stream.next() != "" {
		panic(exprError{"unexpected trailing text"})
	}
	match := dfa(nfa(e))
	checkForDeadEnds(match)
	return match, nil
}

func parseExpr(s *exprStream) *expr {
	var exprs []*expr
	for {
		exprs = append(exprs, parseSeq(s))
		if !s.eat("|") {
			break
		}
	}
	if len(exprs) == 1 {
		return exprs[0]
	}
	return &expr{kind: "choice", exprs: exprs}
}

func parseSeq(s *exprStream) *expr {
	var exprs []*expr
	for {
		exprs = append(exprs, parseSubscript(s))
		if n := s.next(); n == "" || n == ")" || n == "|" {
			break
		}
	}
	if len(exprs) == 1 {
		return exprs[0]
	}
	return &expr{kind: "seq", exprs: exprs}
}

func parseSubscript(s *exprStream) *expr {
	e := parseAtom(s)
	for {
		switch {
		case s.eat("+"):
			e = &expr{kind: "plus", sub: e}
		case s.eat("*"):
			e = &expr{kind: "star", sub: e}
		case s.eat("?"):
			e = &expr{kind: "opt", sub: e}
		case s.eat("{"):
			e = parseRange(s, e)
		default:
			return e
		}
	}
}

func parseNum(s *exprStream) int {
	n, err := strconv.Atoi(s.next())
	if err != nil {
		panic(exprError{"expected number, got '" + s.next() + "'"})
	}
	s.pos++
	return n
}

func parseRange(s *exprStream, e *expr) *expr {
	lo := parseNum(s)
	hi := lo
	if s.eat(",") {
		hi = -1
		if s.next() != "}" {
			hi = parseNum(s)
		}
	}
	if !s.eat("}") {
		panic(exprError{"unclosed braced range"})
	}
	return &expr{kind: "range", min: lo, max: hi, sub: e}
}

func resolveName(s *exprStream, name string) []*NodeType {
	if t := s.types[name]; t != nil {
		return []*NodeType{t}
	}
	var out []*NodeType
	names := make([]string, 0, len(s.types))
	for n := range s.types {
		names = append(names, n)
	}
	// In the schema's order, so a group's default type is its first.
	var ordered []*NodeType
	for _, n := range names {
		ordered = append(ordered, s.types[n])
	}
	if len(ordered) > 0 {
		ordered = ordered[0].Schema.nodeOrder
	}
	for _, t := range ordered {
		if t.inGroup(name) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		panic(exprError{"no node type or group '" + name + "' found"})
	}
	return out
}

func parseAtom(s *exprStream) *expr {
	if s.eat("(") {
		e := parseExpr(s)
		if !s.eat(")") {
			panic(exprError{"missing closing paren"})
		}
		return e
	}
	n := s.next()
	if n == "" || !regexp.MustCompile(`^\w+$`).MatchString(n) {
		panic(exprError{"unexpected token '" + n + "'"})
	}
	var exprs []*expr
	for _, t := range resolveName(s, n) {
		exprs = append(exprs, &expr{kind: "name", value: t})
	}
	s.pos++
	if len(exprs) == 1 {
		return exprs[0]
	}
	return &expr{kind: "choice", exprs: exprs}
}

// NFA states are indexes; each holds edges with a term, nil for epsilon.
type nfaEdge struct {
	term *NodeType
	to   int
}

func nfa(e *expr) [][]nfaEdge {
	states := [][]nfaEdge{nil}
	node := func() int {
		states = append(states, nil)
		return len(states) - 1
	}
	edge := func(from, to int, term *NodeType) *nfaEdge {
		states[from] = append(states[from], nfaEdge{term, to})
		return nil
	}
	type loose struct{ from, at int }
	connect := func(edges []loose, to int) {
		for _, l := range edges {
			states[l.from][l.at].to = to
		}
	}
	dangling := func(from int, term *NodeType) loose {
		states[from] = append(states[from], nfaEdge{term, -1})
		return loose{from, len(states[from]) - 1}
	}
	var compile func(e *expr, from int) []loose
	compile = func(e *expr, from int) []loose {
		switch e.kind {
		case "choice":
			var out []loose
			for _, x := range e.exprs {
				out = append(out, compile(x, from)...)
			}
			return out
		case "seq":
			for i := 0; ; i++ {
				next := compile(e.exprs[i], from)
				if i == len(e.exprs)-1 {
					return next
				}
				from = node()
				connect(next, from)
			}
		case "star":
			loop := node()
			edge(from, loop, nil)
			connect(compile(e.sub, loop), loop)
			return []loose{dangling(loop, nil)}
		case "plus":
			loop := node()
			connect(compile(e.sub, from), loop)
			connect(compile(e.sub, loop), loop)
			return []loose{dangling(loop, nil)}
		case "opt":
			return append([]loose{dangling(from, nil)}, compile(e.sub, from)...)
		case "range":
			cur := from
			for range e.min {
				next := node()
				connect(compile(e.sub, cur), next)
				cur = next
			}
			if e.max == -1 {
				connect(compile(e.sub, cur), cur)
			} else {
				for i := e.min; i < e.max; i++ {
					next := node()
					edge(cur, next, nil)
					connect(compile(e.sub, cur), next)
					cur = next
				}
			}
			return []loose{dangling(cur, nil)}
		case "name":
			return []loose{dangling(from, e.value)}
		}
		panic("unknown expr")
	}
	connect(compile(e, 0), node())
	return states
}

func nullFrom(states [][]nfaEdge, node int) []int {
	var result []int
	var scan func(n int)
	scan = func(n int) {
		edges := states[n]
		if len(edges) == 1 && edges[0].term == nil {
			scan(edges[0].to)
			return
		}
		if at, ok := slices.BinarySearch(result, n); !ok {
			result = slices.Insert(result, at, n)
		}
		for _, e := range edges {
			if e.term == nil && !slices.Contains(result, e.to) {
				scan(e.to)
			}
		}
	}
	scan(node)
	return result
}

func dfa(states [][]nfaEdge) *ContentMatch {
	labeled := map[string]*ContentMatch{}
	key := func(s []int) string { return fmt.Sprint(s) }
	var explore func(set []int) *ContentMatch
	explore = func(set []int) *ContentMatch {
		type out struct {
			term *NodeType
			set  []int
		}
		var outs []out
		for _, n := range set {
			for _, e := range states[n] {
				if e.term == nil {
					continue
				}
				at := slices.IndexFunc(outs, func(o out) bool { return o.term == e.term })
				if at < 0 {
					outs = append(outs, out{e.term, nil})
					at = len(outs) - 1
				}
				for _, s := range nullFrom(states, e.to) {
					if !slices.Contains(outs[at].set, s) {
						outs[at].set = append(outs[at].set, s)
					}
				}
			}
		}
		state := &ContentMatch{ValidEnd: slices.Contains(set, len(states)-1)}
		labeled[key(set)] = state
		for _, o := range outs {
			slices.Sort(o.set)
			next := labeled[key(o.set)]
			if next == nil {
				next = explore(o.set)
			}
			state.next = append(state.next, matchEdge{o.term, next})
		}
		return state
	}
	return explore(nullFrom(states, 0))
}

func checkForDeadEnds(match *ContentMatch) {
	work := []*ContentMatch{match}
	seen := map[*ContentMatch]bool{match: true}
	for len(work) > 0 {
		state := work[0]
		work = work[1:]
		dead := !state.ValidEnd
		var nodes []string
		for _, e := range state.next {
			nodes = append(nodes, e.typ.Name)
			if dead && !(e.typ.IsText() || e.typ.hasRequiredAttrs()) {
				dead = false
			}
			if !seen[e.next] {
				seen[e.next] = true
				work = append(work, e.next)
			}
		}
		if dead {
			panic(exprError{"only non-generatable nodes (" + strings.Join(nodes, ", ") + ") in a required position"})
		}
	}
}

// MatchType is the state after a node of t, nil when t does not fit.
func (m *ContentMatch) MatchType(t *NodeType) *ContentMatch {
	for _, e := range m.next {
		if e.typ == t {
			return e.next
		}
	}
	return nil
}

// MatchFragment is the state after frag's children from start to end
// (the end of frag when negative), nil when one does not fit.
func (m *ContentMatch) MatchFragment(frag *Fragment, start, end int) *ContentMatch {
	if end < 0 {
		end = frag.ChildCount()
	}
	cur := m
	for i := start; cur != nil && i < end; i++ {
		cur = cur.MatchType(frag.Child(i).Type)
	}
	return cur
}

func (m *ContentMatch) inlineContent() bool {
	return len(m.next) != 0 && m.next[0].typ.IsInline()
}

func (m *ContentMatch) defaultType() *NodeType {
	for _, e := range m.next {
		if !(e.typ.IsText() || e.typ.hasRequiredAttrs()) {
			return e.typ
		}
	}
	return nil
}

func (m *ContentMatch) compatible(other *ContentMatch) bool {
	for _, a := range m.next {
		for _, b := range other.next {
			if a.typ == b.typ {
				return true
			}
		}
	}
	return false
}

// FillBefore is what must go before after, from index start, for it to fit
// here, ending in a valid end with toEnd; nil when nothing can.
func (m *ContentMatch) FillBefore(after *Fragment, toEnd bool, start int) *Fragment {
	seen := []*ContentMatch{m}
	var search func(match *ContentMatch, types []*NodeType) *Fragment
	search = func(match *ContentMatch, types []*NodeType) *Fragment {
		finished := match.MatchFragment(after, start, -1)
		if finished != nil && (!toEnd || finished.ValidEnd) {
			nodes := make([]*Node, len(types))
			for i, t := range types {
				nodes[i] = t.CreateAndFill(nil, nil, nil)
			}
			return FragmentFrom(nodes...)
		}
		for _, e := range match.next {
			if !(e.typ.IsText() || e.typ.hasRequiredAttrs()) && !slices.Contains(seen, e.next) {
				seen = append(seen, e.next)
				if found := search(e.next, append(slices.Clone(types), e.typ)); found != nil {
					return found
				}
			}
		}
		return nil
	}
	return search(m, nil)
}

// FindWrapping is the types to wrap a node of target in to fit here, an
// empty list for none needed; nil when none can.
func (m *ContentMatch) FindWrapping(target *NodeType) []*NodeType {
	for _, c := range m.wrapCache {
		if c.target == target {
			return c.wraps
		}
	}
	computed := m.computeWrapping(target)
	m.wrapCache = append(m.wrapCache, wrapEntry{target, computed})
	return computed
}

func (m *ContentMatch) computeWrapping(target *NodeType) []*NodeType {
	type active struct {
		match *ContentMatch
		typ   *NodeType
		via   *active
	}
	seen := map[string]bool{}
	work := []*active{{match: m}}
	for len(work) > 0 {
		cur := work[0]
		work = work[1:]
		if cur.match.MatchType(target) != nil {
			result := []*NodeType{}
			for o := cur; o.typ != nil; o = o.via {
				result = append(result, o.typ)
			}
			slices.Reverse(result)
			return result
		}
		for _, e := range cur.match.next {
			if !e.typ.IsLeaf() && !e.typ.hasRequiredAttrs() && !seen[e.typ.Name] && (cur.typ == nil || e.next.ValidEnd) {
				work = append(work, &active{match: e.typ.ContentMatch, typ: e.typ, via: cur})
				seen[e.typ.Name] = true
			}
		}
	}
	return nil
}

// EdgeCount and Edge list the types it goes on with.
func (m *ContentMatch) EdgeCount() int { return len(m.next) }

// Edge is its nth edge.
func (m *ContentMatch) Edge(n int) (*NodeType, *ContentMatch) { return m.next[n].typ, m.next[n].next }
