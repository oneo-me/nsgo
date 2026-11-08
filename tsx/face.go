package tsx

import (
	"github.com/oneo-me/nsgo/tsx/internal/js"
	"slices"

	"github.com/oneo-me/nsgo/core"
)

// Go components as elements (NSH's components.rs): a face makes a Go
// component of a node's props. What it reads of the props as it makes
// the component is tracked, so a change makes the component again; what
// the component reads through the functions the face gives it (StyleFunc,
// Open) it follows itself, and handlers look up the current function
// when they run.
//
// A face's node is a region element showing the component; its children
// go into a region element of their own, which the face places where the
// component takes children. A host tag's props (<Overlay>, <WebView>,
// <Video>) are signals set one at a time and the component is made once
// the node is first inserted; a component's (Canvas, Dialog, base's) are
// the props object compiled JSX passes, its getters tracked as they read
// signals.

// A face makes a Go component of props.
type face func(p *props) core.Node

// props are a face node's props.
type props struct {
	r *runtime
	n *node
	// cx is the scope the face makes the component under.
	cx *core.Scope
	// obj is a component's props object; a host tag's props are n's
	// signals.
	obj *js.Object
}

// newFaceNode makes a node showing what f makes of props obj, or of the
// props set on it when obj is nil.
func (r *runtime) newFaceNode(name string, f face, obj *js.Object) *node {
	n := &node{r: r, kind: kindFace, name: name, ref: &core.Ref{}, face: f,
		elem: core.NewElement(r.cx, nil), content: core.NewElement(r.cx, nil)}
	n.props = &props{r: r, n: n, obj: obj}
	if obj == nil {
		n.signals = map[string]*core.Signal[js.Value]{}
	}
	return n
}

// mountFace makes n's component, once.
func (r *runtime) mountFace(n *node) {
	if n.mounted {
		return
	}
	n.mounted = true
	p := n.props
	n.elem.Mount(core.Dynamic(func(cx *core.Scope) core.Node {
		p.cx = cx
		return n.face(p)
	}))
}

// signal is a host tag's prop name, made unset on first use.
func (n *node) signal(name string) *core.Signal[js.Value] {
	s, ok := n.signals[name]
	if !ok {
		s = core.NewSignalFunc[js.Value](n.r.cx, js.Undefined(), strictEquals)
		n.signals[name] = s
	}
	return s
}

// get reads prop name, tracked: the current platform's object's (ios,
// android, desktop, web) when the props lack it.
func (p *props) get(name string) js.Value {
	v := p.own(name)
	if defined(v) {
		return v
	}
	for _, k := range p.r.platformKeys() {
		if o, ok := p.own(k).(*js.Object); ok {
			if v := o.Get(name); defined(v) {
				return v
			}
		}
	}
	return v
}

func (p *props) own(name string) js.Value {
	if p.obj != nil {
		return p.obj.Get(name)
	}
	return p.n.signal(name).Get()
}

// peek reads prop name untracked.
func (p *props) peek(name string) js.Value {
	return core.Untracked(p.r.cx, func() js.Value { return p.get(name) })
}

// has reports whether prop name is set, untracked: a component reads
// what it needs of a prop as it renders, not to know that it is set. A
// key the JSX gave is set, whatever it reads now (sort={sort()} while
// sort() is undefined).
func (p *props) has(name string) bool {
	if p.obj != nil && slices.Contains(p.obj.Keys(), name) {
		return true
	}
	return core.Untracked(p.r.cx, func() bool { return defined(p.get(name)) })
}

func (p *props) str(name string) string { return stringOf(p.get(name)) }

func (p *props) bool(name string) bool {
	v := p.get(name)
	return v != nil && v.ToBoolean()
}

// boolOr reads a boolean that defaults to def.
func (p *props) boolOr(name string, def bool) bool {
	if v := p.get(name); defined(v) {
		return v.ToBoolean()
	}
	return def
}

func (p *props) float(name string) float64 {
	if v := p.get(name); defined(v) {
		return v.ToFloat()
	}
	return 0
}

func (p *props) int(name string) int { return int(p.float(name)) }

func (p *props) points(name string) core.Points { return core.Points(p.float(name)) }

// boolFunc follows a boolean prop; nil when it is absent as the face
// reads it.
func (p *props) boolFunc(name string) func() bool {
	if !p.has(name) {
		return nil
	}
	return func() bool { return p.bool(name) }
}

func (p *props) strFunc(name string) func() string {
	return func() string { return p.str(name) }
}

// style is the props' style and state styles, decoded.
func (p *props) style() core.Style {
	s := p.r.style(p.get("style"))
	for name, at := range stateProps {
		if v := p.get(name); defined(v) {
			st := p.r.style(v)
			*at(&s) = &st
		}
	}
	return s
}

// styleFunc follows the props' style.
func (p *props) styleFunc() func() core.Style { return p.style }

// class is the props' class and classList.
func (p *props) class() string {
	c, l := p.str("class"), p.r.classList(p.get("classList"))
	switch {
	case l == "":
		return c
	case c == "":
		return l
	}
	return c + " " + l
}

// classFunc follows the props' classes.
func (p *props) classFunc() func() string { return p.class }

// children are the node's children: for a host tag the region element
// compiled JSX inserts them in, for a component the children prop made
// where the component places them.
func (p *props) children() []core.Node {
	if p.obj == nil {
		return []core.Node{p.n.content}
	}
	return []core.Node{lazy{p.r, p.n, func() js.Value { return p.obj.Get("children") }}}
}

// slot is what prop name renders, as a node; nil when it is absent as
// the face reads it. A function is called, so the slot follows it.
func (p *props) slot(name string) core.Node {
	if !p.has(name) {
		return nil
	}
	return lazy{p.r, p.n, func() js.Value { return p.get(name) }}
}

// lazy is a region made as core renders it, under the scope it renders
// in: what it shows is made inside the Go component placing it, whose
// context that sees, as JSX children are made where they show. Its
// nodes' parent is owner, for the faces that look up the components
// around them.
type lazy struct {
	r     *runtime
	owner *node
	get   func() js.Value
}

// Render implements core.Node.
func (l lazy) Render(cx *core.Scope) core.Node {
	var e *core.Element
	cx.Run(func() {
		n := &node{r: l.r, kind: kindRegion, elem: core.NewElement(cx, nil), parent: l.owner}
		l.r.insertValue(n, l.r.accessor(l.get), nil)
		e = n.elem
	})
	return e
}

// region is a region element showing what get returns, as children.
func (r *runtime) region(get func() js.Value) *core.Element {
	n := &node{r: r, kind: kindRegion, elem: core.NewElement(r.cx, nil)}
	r.insertValue(n, r.accessor(get), nil)
	return n.elem
}

// faceTooltip is a <Tooltip> given as an element's tooltip: the hint
// overlay itself (core.HintTooltip), which its open state shows.
type faceTooltip struct{ n *node }

// Render implements core.Node.
func (f faceTooltip) Render(*core.Scope) core.Node { return f.n.elem }

// Hint implements core.HintTooltip.
func (f faceTooltip) Hint(cx *core.Scope, anchor *core.Ref) (core.Node, func(bool)) {
	f.n.props.cx = cx
	if h, ok := f.n.face(f.n.props).(core.HintTooltip); ok {
		return h.Hint(cx, anchor)
	}
	f.n.r.mountFace(f.n)
	return f.n.elem, func(bool) {}
}

// handler returns a caller of prop name's current function, nil when it
// is none as the face reads it.
func (p *props) handler(name string) func(args ...js.Value) js.Value {
	if !isFunction(p.get(name)) {
		return nil
	}
	return func(args ...js.Value) js.Value {
		if h := p.peek(name); isFunction(h) {
			return p.r.call(h, args...)
		}
		return js.Undefined()
	}
}

// on returns a handler without arguments.
func (p *props) on(name string) func() {
	h := p.handler(name)
	if h == nil {
		return nil
	}
	return func() { h() }
}

// onPress returns a handler of a press.
func (p *props) onPress(name string) func(core.PressEvent) {
	h := p.handler(name)
	if h == nil {
		return nil
	}
	return func(e core.PressEvent) { h(p.r.pressEvent(e)) }
}

// onLayout returns a handler of layouts.
func (p *props) onLayout() func(core.Bounds) {
	h := p.handler("onLayout")
	if h == nil {
		return nil
	}
	return func(b core.Bounds) { h(p.r.layoutEvent(b)) }
}

// onError returns a handler of an error, given { message }.
func (p *props) onError(name string) func(error) {
	h := p.handler(name)
	if h == nil {
		return nil
	}
	return func(err error) {
		o := p.r.vm.NewObject()
		_ = o.Set("message", err.Error())
		h(o)
	}
}

// platformKeys are the platform objects that apply here, from the most
// specific: ios, android, web, desktop.
func (r *runtime) platformKeys() []string {
	if r.platforms == nil {
		e := r.cx.Env()
		core.Untracked(r.cx, func() bool {
			switch os := e.OS(); os {
			case core.OSIOS, core.OSAndroid:
				r.platforms = []string{string(os)}
			case core.OSWeb:
				r.platforms = []string{"web"}
				if e.Desktop() {
					r.platforms = append(r.platforms, "desktop")
				}
			default:
				r.platforms = []string{"desktop"}
			}
			return true
		})
	}
	return r.platforms
}

// isPlatformProp reports whether a prop is a platform's object.
func isPlatformProp(name string) bool {
	return name == "ios" || name == "android" || name == "desktop" || name == "web"
}
