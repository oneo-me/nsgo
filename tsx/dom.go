package tsx

import (
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/tsx/internal/js"

	"github.com/oneo-me/nsgo/core"
)

// The renderer compiled JSX runs on (NSH's dom.rs): the functions of
// nsgo:rt and the parts of nsgo built on them (mount).
//
// A node is a JS object over a core.Element; the renderer mirrors each
// node's children, which dynamic inserts reconcile against with Solid's
// reconcileArrays. A raw text node and the children of a <Text> stay
// here: core.Text takes spans, so the outermost <Text> gets its texts as
// spans and its other children as attachments.

// maxDepth is how deep nested arrays and functions are unwrapped.
const maxDepth = 64

type kind uint8

const (
	kindRegion kind = iota
	kindView
	kindText
	kindScroll
	kindInput
	kindImage
	kindFace // a Go component (face.go)
	kindRaw  // a raw text node
)

// node is a node of the renderer.
type node struct {
	r        *runtime
	kind     kind
	elem     *core.Element // nil for raw text
	parent   *node
	children []*node
	obj      *js.Object
	// The props of its kind; Style, Class and the states' styles come from
	// the fields below.
	view   core.View
	text   core.Text
	scroll core.Scroll
	input  core.Input
	image  core.Image
	raw    string
	// style is the inline style; class and classList the classes; states
	// the styles of states (activeStyle, ...) by prop name.
	style            core.Style
	class, classList string
	states           map[string]core.Style
	// ref names the view, for the node's methods; horizontal says a list's
	// axis.
	ref        *core.Ref
	horizontal bool
	// platform holds the keys each platform object (ios={{…}}) set, which
	// are cleared when it no longer has them.
	platform map[string][]string
	// values are a host element's props as set, to make them again when
	// one is unset.
	values map[string]js.Value

	// A face node's: what makes its component, its props, the region
	// element its children go in, and whether the component was made.
	name    string
	face    face
	props   *props
	signals map[string]*core.Signal[js.Value]
	content *core.Element
	mounted bool
	// A scroll view's content view's (contentStyle, contentClass).
	contentStyle core.Style
	contentClass string
}

// box is the element n's children go in.
func (n *node) box() *core.Element {
	if n.content != nil {
		return n.content
	}
	return n.elem
}

// value is n's JS object.
func (n *node) value() *js.Object {
	if n.obj == nil {
		n.obj = n.r.vm.ToValue(n).(*js.Object)
	}
	return n.obj
}

// nodeOf returns the node v is, or nil.
func (r *runtime) nodeOf(v js.Value) *node {
	if o, ok := v.(*js.Object); ok {
		if n, ok := o.Export().(*node); ok && n.r == r {
			return n
		}
	}
	return nil
}

func (r *runtime) mustNode(v js.Value) *node {
	n := r.nodeOf(v)
	if n == nil {
		panic(r.vm.NewTypeError("nsgo: not a node"))
	}
	return n
}

// element is `element(tag)`: a new element of a host tag.
func (r *runtime) element(call js.FunctionCall) js.Value {
	tag := arg(call, 0).String()
	return r.newElement(tag).value()
}

// newElement makes a node of a host tag.
func (r *runtime) newElement(tag string) *node {
	if f, ok := hostFaces[tag]; ok {
		return r.newFaceNode(tag, f, nil)
	}
	n := &node{r: r, ref: &core.Ref{}}
	switch tag {
	case "view":
		n.kind = kindView
		n.elem = core.NewElement(r.cx, n.view)
	case "text":
		n.kind = kindText
		n.elem = core.NewElement(r.cx, n.text)
	case "scroll":
		n.kind = kindScroll
		n.elem = core.NewElement(r.cx, n.scroll)
	case "input":
		n.kind = kindInput
		n.elem = core.NewElement(r.cx, n.input)
	case "image":
		n.kind = kindImage
		n.elem = core.NewElement(r.cx, n.image)
	default:
		panic(r.vm.NewTypeError("nsgo: no element %q yet", tag))
	}
	return n
}

// text is `text(value)`: a new raw text node.
func (r *runtime) textNode(call js.FunctionCall) js.Value {
	return r.newRaw(arg(call, 0).String()).value()
}

func (r *runtime) newRaw(s string) *node { return &node{r: r, kind: kindRaw, raw: s} }

// append is `append(parent, child)`: a static child at the end.
func (r *runtime) append(call js.FunctionCall) js.Value {
	p, child := r.mustNode(arg(call, 0)), arg(call, 1)
	if c := r.nodeOf(child); c != nil {
		r.insertBefore(p, c, nil)
	} else {
		r.insertValue(p, child, nil)
	}
	return js.Undefined()
}

// insert is `insert(parent, value, marker)`: children before marker, or
// at the end.
func (r *runtime) insert(call js.FunctionCall) js.Value {
	r.insertValue(r.mustNode(arg(call, 0)), arg(call, 1), r.nodeOf(arg(call, 2)))
	return js.Undefined()
}

// prop is `prop(node, name, value)`.
func (r *runtime) prop(call js.FunctionCall) js.Value {
	r.setProp(r.mustNode(arg(call, 0)), arg(call, 1).String(), arg(call, 2))
	return js.Undefined()
}

// bindProp is `bindProp(node, name, fn)`: the prop set to what fn returns,
// again when what it read changes.
func (r *runtime) bindProp(call js.FunctionCall) js.Value {
	n, name, fn := r.mustNode(arg(call, 0)), arg(call, 1).String(), arg(call, 2)
	r.cx.RenderEffect(func() { r.setProp(n, name, r.call(fn)) })
	return js.Undefined()
}

// component is `component(Comp, props)`: a component's call, untracked.
func (r *runtime) component(call js.FunctionCall) js.Value {
	comp, props := arg(call, 0), arg(call, 1)
	return core.Untracked(r.cx, func() js.Value { return r.call(comp, props) })
}

// insertBefore puts c among p's children before marker, or at the end.
func (r *runtime) insertBefore(p, c, marker *node) {
	if c == p {
		panic(r.vm.NewTypeError("nsgo: a node cannot go inside itself"))
	}
	if c.parent != nil {
		r.removeChild(c.parent, c)
	}
	i := len(p.children)
	if marker != nil {
		if j := slices.Index(p.children, marker); j >= 0 {
			i = j
		}
	}
	p.children = slices.Insert(p.children, i, c)
	c.parent = p
	switch {
	case p.kind == kindText:
		r.textChanged(p)
	case c.kind == kindRaw:
		r.warn("text is shown only inside <Text>")
	default:
		if c.kind == kindFace {
			r.mountFace(c)
		}
		p.box().Insert(c.elem, r.nextElement(p, i+1))
		if c.kind == kindText {
			r.textChanged(c)
		}
	}
}

// nextElement is the element of the first of p's children from i on that
// has one in core: what an insert at i goes before.
func (r *runtime) nextElement(p *node, i int) *core.Element {
	for _, c := range p.children[i:] {
		if c.kind != kindRaw {
			return c.elem
		}
	}
	return nil
}

// removeChild takes c out of p's children; it stays until its owner goes.
func (r *runtime) removeChild(p, c *node) {
	i := slices.Index(p.children, c)
	if i < 0 {
		return
	}
	p.children = slices.Delete(p.children, i, i+1)
	c.parent = nil
	if p.kind == kindText {
		r.textChanged(p)
	} else if c.elem != nil {
		p.box().Remove(c.elem)
	}
}

// nextSibling is the node after c among its parent's children.
func nextSibling(c *node) *node {
	if p := c.parent; p != nil {
		if i := slices.Index(p.children, c); i >= 0 && i+1 < len(p.children) {
			return p.children[i+1]
		}
	}
	return nil
}

// textChanged sets the outermost <Text> around n again, with the texts
// inside it as spans.
func (r *runtime) textChanged(n *node) {
	for n.parent != nil && n.parent.kind == kindText {
		n = n.parent
	}
	if n.kind != kindText {
		return
	}
	t := n.text
	t.Style, t.Class, t.Ref = n.fullStyle(), n.classes(), n.ref
	t.Spans = r.spans(n, nil, core.Style{}, "", nil, nil)
	n.elem.Set(t)
}

// spans appends the spans of text n's children, styled over style and
// class, pressed with the nearest press and long press around them.
func (r *runtime) spans(n *node, out []core.Span, style core.Style, class string, press, long func(core.PressEvent)) []core.Span {
	for _, c := range n.children {
		switch c.kind {
		case kindRaw:
			out = append(out, core.Span{Content: c.raw, Style: style, Class: class, OnPress: press, OnLongPress: long})
		case kindText:
			p, l := press, long
			if c.text.OnPress != nil {
				p = c.text.OnPress
			}
			if c.text.OnLongPress != nil {
				l = c.text.OnLongPress
			}
			out = r.spans(c, out, style.Merge(c.style), strings.TrimSpace(class+" "+c.classes()), p, l)
		default:
			out = append(out, core.Span{Attachment: c.elem})
		}
	}
	return out
}

// classes are n's class and classList.
func (n *node) classes() string {
	if n.classList == "" {
		return n.class
	}
	return strings.TrimSpace(n.class + " " + n.classList)
}

// An item is what a child resolves to: a node, or text.
type item struct {
	node *node
	text string
}

// insertValue inserts value into p before marker: once for a static
// value, kept up to date by a render effect for a dynamic one.
func (r *runtime) insertValue(p *node, value js.Value, marker *node) {
	if r.isDynamic(value, 0) {
		shown := new([]*node)
		if isFunction(value) {
			r.cx.RenderEffect(func() { r.insertRun(p, marker, value, shown) })
		} else {
			r.insertItems(p, marker, value, shown)
		}
		return
	}
	for _, n := range r.materialize(r.resolve(value, nil, 0), nil) {
		r.insertBefore(p, n, marker)
	}
}

// insertRun is a run of a dynamic insert of a function: it calls it, and
// the functions it returns, tracked. An array holding functions gets an
// effect of its own, made again only when this one runs again, so a
// change in one of its functions runs only that one.
func (r *runtime) insertRun(p, marker *node, value js.Value, shown *[]*node) {
	for depth := 0; isFunction(value); depth++ {
		if depth > maxDepth {
			panic(r.vm.NewTypeError("nsgo: children nested too deeply"))
		}
		value = r.call(value)
	}
	if r.isArray(value) && r.isDynamic(value, 0) {
		r.insertItems(p, marker, value, shown)
	} else {
		r.show(p, marker, value, shown)
	}
}

// insertItems inserts an array holding functions: each function gets a
// memo of its own, and one render effect shows them all.
func (r *runtime) insertItems(p, marker *node, items js.Value, shown *[]*node) {
	if memos := r.memoizeRun(items); memos != nil {
		items = memos
	}
	r.cx.RenderEffect(func() { r.show(p, marker, items, shown) })
}

// memoizeRun returns a copy of an array of children holding functions,
// with a memo for each function; nil otherwise.
func (r *runtime) memoizeRun(value js.Value) js.Value {
	items := r.arrayItems(value)
	functions := 0
	for _, v := range items {
		if isFunction(v) {
			functions++
		}
	}
	if len(items) < 2 || functions == 0 {
		return nil
	}
	out := make([]any, len(items))
	for i, v := range items {
		if isFunction(v) {
			out[i] = r.itemMemo(v)
		} else {
			out[i] = v
		}
	}
	return r.vm.NewArray(out...)
}

// itemMemo is a memo of fn, one function of an array of children; a
// function fn returns gets a memo of its own, under this one.
func (r *runtime) itemMemo(fn js.Value) js.Value {
	m := core.NewMemoFunc(r.cx, func() js.Value {
		v := r.call(fn)
		if isFunction(v) {
			return r.itemMemo(v)
		}
		return v
	}, strictEquals)
	return r.accessor(m.Get)
}

// show shows value in p before marker in place of the nodes shown holds,
// which then holds the new ones.
func (r *runtime) show(p, marker *node, value js.Value, shown *[]*node) {
	items := r.resolve(value, nil, 0)
	old := *shown
	var reuse *node
	if len(old) == 1 && len(items) == 1 && items[0].node == nil {
		reuse = old[0]
	}
	nodes := r.materialize(items, reuse)
	r.reconcile(p, old, nodes, marker)
	*shown = nodes
}

// resolve appends what value shows: functions called, arrays flattened,
// strings and numbers as text, nothing for null, undefined and booleans.
func (r *runtime) resolve(value js.Value, out []item, depth int) []item {
	if depth > maxDepth {
		panic(r.vm.NewTypeError("nsgo: children nested too deeply"))
	}
	if value == nil || js.IsUndefined(value) || js.IsNull(value) {
		return out
	}
	if n := r.nodeOf(value); n != nil {
		return append(out, item{node: n})
	}
	if o, ok := value.(*js.Object); ok {
		switch {
		case isFunction(o):
			return r.resolve(r.call(o), out, depth+1)
		case r.isArray(o):
			for _, v := range r.arrayItems(o) {
				out = r.resolve(v, out, depth+1)
			}
		default:
			r.warn("a child that is not an element, text, array or function is not rendered")
		}
		return out
	}
	if value.ExportType().Kind() == reflect.Bool {
		return out
	}
	return append(out, item{text: value.String()})
}

// materialize turns items into nodes: text becomes new raw text nodes,
// except a lone text over a lone raw text node reuse, which is updated.
func (r *runtime) materialize(items []item, reuse *node) []*node {
	if reuse != nil && reuse.kind == kindRaw && len(items) == 1 && items[0].node == nil {
		if reuse.raw != items[0].text {
			reuse.raw = items[0].text
			if reuse.parent != nil {
				r.textChanged(reuse.parent)
			}
		}
		return []*node{reuse}
	}
	nodes := make([]*node, len(items))
	for i, it := range items {
		if it.node != nil {
			nodes[i] = it.node
		} else {
			nodes[i] = r.newRaw(it.text)
		}
	}
	return nodes
}

// isDynamic reports whether value holds a function, directly or in nested
// arrays: its children change over time.
func (r *runtime) isDynamic(value js.Value, depth int) bool {
	if isFunction(value) {
		return true
	}
	if depth > maxDepth || !r.isArray(value) {
		return false
	}
	for _, v := range r.arrayItems(value) {
		if r.isDynamic(v, depth+1) {
			return true
		}
	}
	return false
}

func (r *runtime) isArray(v js.Value) bool {
	o, ok := v.(*js.Object)
	return ok && o.ClassName() == "Array"
}

// arrayItems returns the values of an array, none for other values.
// item is item i of array v, nil past its end or when v is no array.
func (r *runtime) item(v js.Value, i int) js.Value {
	if !r.isArray(v) || i < 0 {
		return nil
	}
	o := v.(*js.Object)
	if int64(i) >= o.Get("length").ToInteger() {
		return nil
	}
	return o.Get(strconv.Itoa(i))
}

func (r *runtime) arrayItems(v js.Value) []js.Value {
	if !r.isArray(v) {
		return nil
	}
	o := v.(*js.Object)
	n := int(o.Get("length").ToInteger())
	out := make([]js.Value, n)
	for i := range out {
		out[i] = o.Get(strconv.Itoa(i))
	}
	return out
}

// reconcile is Solid's reconcileArrays: it changes p's children from a to
// b (the nodes of one insert, before marker) with few moves.
func (r *runtime) reconcile(p *node, old, b []*node, marker *node) {
	a := slices.Clone(old)
	aStart, bStart := 0, 0
	aEnd, bEnd := len(a), len(b)
	after := marker
	if len(a) > 0 {
		if s := nextSibling(a[len(a)-1]); s != nil {
			after = s
		}
	}
	var index map[*node]int
	for aStart < aEnd || bStart < bEnd {
		if aStart < aEnd && bStart < bEnd && a[aStart] == b[bStart] {
			aStart++
			bStart++
			continue
		}
		for aEnd > aStart && bEnd > bStart && a[aEnd-1] == b[bEnd-1] {
			aEnd--
			bEnd--
		}
		switch {
		case aEnd == aStart:
			// Appends, before the first node of the placed tail.
			at := after
			if bEnd < len(b) {
				at = b[bEnd]
			}
			for ; bStart < bEnd; bStart++ {
				r.insertBefore(p, b[bStart], at)
			}
		case bEnd == bStart:
			// Removals.
			for ; aStart < aEnd; aStart++ {
				if _, again := index[a[aStart]]; !again {
					r.removeChild(p, a[aStart])
				}
			}
		case a[aStart] == b[bEnd-1] && b[bStart] == a[aEnd-1]:
			// A swap.
			aEnd--
			at := nextSibling(a[aEnd])
			firstNext := nextSibling(a[aStart])
			r.insertBefore(p, b[bStart], firstNext)
			bStart++
			aStart++
			bEnd--
			r.insertBefore(p, b[bEnd], at)
			a[aEnd] = b[bEnd]
		default:
			if index == nil {
				index = make(map[*node]int, bEnd-bStart)
				for i := bStart; i < bEnd; i++ {
					index[b[i]] = i
				}
			}
			i, ok := index[a[aStart]]
			switch {
			case ok && bStart < i && i < bEnd:
				k, sequence := aStart, 1
				for k++; k < aEnd && k < bEnd; k++ {
					if t, ok := index[a[k]]; !ok || t != i+sequence {
						break
					}
					sequence++
				}
				if sequence > i-bStart {
					at := a[aStart]
					for ; bStart < i; bStart++ {
						r.insertBefore(p, b[bStart], at)
					}
				} else {
					// A node that comes again later stays, to be moved
					// then rather than removed and inserted.
					r.insertBefore(p, b[bStart], a[aStart])
					if _, again := index[a[aStart]]; !again {
						r.removeChild(p, a[aStart])
					}
					bStart++
					aStart++
				}
			case ok:
				aStart++
			default:
				r.removeChild(p, a[aStart])
				aStart++
			}
		}
	}
}

// cond is `cond(when, then, otherwise)`: what then() renders while when()
// is truthy, else what otherwise() renders, each branch rendered once per
// change of truthiness, under the branch's memo.
func (r *runtime) cond(call js.FunctionCall) js.Value {
	when, then, otherwise := arg(call, 0), arg(call, 1), arg(call, 2)
	truthy := core.NewMemo(r.cx, func() bool { return r.call(when).ToBoolean() })
	branch := core.NewMemoFunc(r.cx, func() js.Value {
		b := otherwise
		if truthy.Get() {
			b = then
		}
		if !isFunction(b) {
			return js.Undefined()
		}
		return core.Untracked(r.cx, func() js.Value { return r.call(b) })
	}, strictEquals)
	return r.accessor(branch.Get)
}

// row is a row of each.
type row struct {
	key         any
	item, index *core.Signal[js.Value]
	scope       *core.Scope
	output      js.Value
}

// each is `each(list, render, key)`: render(item, index) for each item of
// list() (both accessors), one row per key (key(item, index), the index
// for key "index", or the item itself) kept while the key stays.
func (r *runtime) each(call js.FunctionCall) js.Value {
	list, render, keyFn := arg(call, 0), arg(call, 1), arg(call, 2)
	keyed := isFunction(keyFn)
	byIndex := !keyed && keyFn.ExportType() != nil && keyFn.ExportType().Kind() == reflect.String &&
		keyFn.String() == "index"
	// The rows go with the owner each is called under, not the memo.
	home := r.cx.Child()
	var rows []*row
	m := core.NewMemoFunc(r.cx, func() js.Value {
		items := r.arrayItems(r.call(list))
		var outputs []any
		home.Run(func() {
			byKey := map[any][]*row{}
			for i := len(rows) - 1; i >= 0; i-- {
				byKey[rows[i].key] = append(byKey[rows[i].key], rows[i])
			}
			next := make([]*row, 0, len(items))
			for i, it := range items {
				index := r.vm.ToValue(i)
				var kv js.Value
				switch {
				case byIndex:
					kv = index
				case keyed:
					kv = r.call(keyFn, it, index)
				default:
					kv = it
				}
				k := keyOf(kv)
				if old := byKey[k]; len(old) > 0 {
					w := old[len(old)-1]
					byKey[k] = old[:len(old)-1]
					w.item.Set(it)
					w.index.Set(index)
					next = append(next, w)
					continue
				}
				w := &row{key: k, item: core.NewSignalFunc(r.cx, it, strictEquals),
					index: core.NewSignalFunc(r.cx, index, strictEquals), scope: home.Child()}
				w.scope.Run(func() {
					w.output = r.call(render, r.accessor(w.item.Get), r.accessor(w.index.Get))
				})
				next = append(next, w)
			}
			for _, old := range byKey {
				for _, w := range old {
					w.scope.Dispose()
				}
			}
			rows = next
			outputs = make([]any, len(rows))
			for i, w := range rows {
				outputs[i] = w.output
			}
		})
		return r.vm.NewArray(outputs...)
	}, nil)
	return r.accessor(m.Get)
}

// keyOf is the key rows are matched by: objects by identity, other values
// by value, numbers as SameValueZero compares them.
func keyOf(v js.Value) any {
	if o, ok := v.(*js.Object); ok {
		return o
	}
	if v == nil || js.IsUndefined(v) {
		return nil
	}
	switch x := v.Export().(type) {
	case int64:
		return float64(x)
	case float64:
		if x == 0 {
			return 0.0
		}
		return x
	default:
		return x
	}
}

// mount is `mount(App, props)`: App rendered into the surface; it returns
// a function that removes it.
func (r *runtime) mount(call js.FunctionCall) js.Value {
	app, props := arg(call, 0), arg(call, 1)
	if _, ok := props.(*js.Object); !ok {
		props = r.vm.NewObject()
	}
	before := slices.Clone(r.root.children)
	owner := r.cx.Child()
	owner.Run(func() { r.insertValue(r.root, r.call(app, props), nil) })
	return r.function(func(js.FunctionCall) js.Value {
		owner.Dispose()
		for _, c := range slices.Clone(r.root.children) {
			if !slices.Contains(before, c) {
				r.removeChild(r.root, c)
			}
		}
		return js.Undefined()
	})
}

// merge is `merge(...sources)`: an object of the sources' own properties,
// later ones winning, getters kept, so props spread into a component stay
// live.
const mergeSource = `(function (...sources) {
	const out = {};
	for (const s of sources) {
		if (s != null) Object.defineProperties(out, Object.getOwnPropertyDescriptors(s));
	}
	return out;
})`

// installGlobals installs If and Each, which tags name without an import,
// for the compiler's component calls of them (const C = c ? If : View).
func (r *runtime) installGlobals() {
	vm := r.vm
	_ = vm.Set("If", r.function(func(call js.FunctionCall) js.Value {
		props := call.Argument(0).ToObject(vm)
		return r.cond(js.FunctionCall{Arguments: []js.Value{
			r.accessor(func() js.Value { return props.Get("when") }),
			r.accessor(func() js.Value { return props.Get("children") }),
			r.accessor(func() js.Value { return props.Get("else") }),
		}})
	}))
	_ = vm.Set("Each", r.function(func(call js.FunctionCall) js.Value {
		props := call.Argument(0).ToObject(vm)
		return r.each(js.FunctionCall{Arguments: []js.Value{
			r.accessor(func() js.Value { return props.Get("of") }),
			props.Get("children"),
			props.Get("key"),
		}})
	}))
}
