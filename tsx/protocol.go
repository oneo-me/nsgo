package tsx

import (
	"slices"

	"github.com/oneo-me/nsgo/tsx/internal/js"

	"github.com/oneo-me/nsgo/core"
)

// Props and style objects decoded into core's values (NSH's protocol.rs):
// each NSH prop sets the field of core's element that means the same.

// stateProps are the props styling a view in a state (NSH's activeStyle,
// ...), as core's Style fields of the states.
var stateProps = map[string]func(s *core.Style) **core.Style{
	"activeStyle":        func(s *core.Style) **core.Style { return &s.Active },
	"hoverStyle":         func(s *core.Style) **core.Style { return &s.Hover },
	"focusStyle":         func(s *core.Style) **core.Style { return &s.Focus },
	"disabledStyle":      func(s *core.Style) **core.Style { return &s.Disabled },
	"checkedStyle":       func(s *core.Style) **core.Style { return &s.Checked },
	"indeterminateStyle": func(s *core.Style) **core.Style { return &s.Indeterminate },
	"selectedStyle":      func(s *core.Style) **core.Style { return &s.Selected },
	"focusWithinStyle":   func(s *core.Style) **core.Style { return &s.FocusWithin },
	"expandedStyle":      func(s *core.Style) **core.Style { return &s.Expanded },
	"pressedStyle":       func(s *core.Style) **core.Style { return &s.Pressed },
}

// setProp sets prop name of n to v and gives n its props again.
func (r *runtime) setProp(n *node, name string, v js.Value) {
	switch {
	case n.kind == kindRaw || n.kind == kindRegion:
		panic(r.vm.NewTypeError("nsgo: a text node has no props"))
	case name == "children":
		return // a host element's children are nodes
	case name == "ref":
		r.applyRef(v, n, "")
		return
	case n.kind == kindFace:
		n.signal(name).Set(v)
		return
	case isPlatformProp(name):
		r.platformProp(n, name, v)
		return
	}
	if !defined(v) {
		// Unset: the props are made again from the others.
		if _, ok := n.values[name]; !ok {
			return
		}
		delete(n.values, name)
		n.view, n.text, n.scroll, n.input, n.image = core.View{}, core.Text{}, core.Scroll{}, core.Input{}, core.Image{}
		n.style, n.class, n.classList, n.states = core.Style{}, "", "", nil
		for k, v := range n.values {
			r.decodeProp(n, k, v)
		}
		r.apply(n)
		return
	}
	if n.values == nil {
		n.values = map[string]js.Value{}
	}
	n.values[name] = v
	if r.decodeProp(n, name, v) {
		r.apply(n)
	}
}

// platformProp sets the keys of a platform's object (ios={{…}}) as props
// where it applies, clearing those it no longer has.
func (r *runtime) platformProp(n *node, name string, v js.Value) {
	if !slices.Contains(r.platformKeys(), name) {
		return
	}
	var keys []string
	if o, ok := v.(*js.Object); ok {
		keys = o.Keys()
		for _, k := range keys {
			r.setProp(n, k, o.Get(k))
		}
	}
	for _, k := range n.platform[name] {
		if !slices.Contains(keys, k) {
			r.setProp(n, k, js.Undefined())
		}
	}
	if n.platform == nil {
		n.platform = map[string][]string{}
	}
	n.platform[name] = keys
}

// decodeProp decodes prop name into n's props, reporting whether n has it.
func (r *runtime) decodeProp(n *node, name string, v js.Value) bool {
	switch name {
	case "style":
		n.style = r.style(v)
		return true
	case "class":
		n.class = stringOf(v)
		return true
	case "classList":
		n.classList = r.classList(v)
		return true
	}
	if _, ok := stateProps[name]; ok {
		if n.states == nil {
			n.states = map[string]core.Style{}
		}
		n.states[name] = r.style(v)
		return true
	}
	var known bool
	switch n.kind {
	case kindView:
		known = r.viewProp(&n.view, name, v)
	case kindText:
		known = r.textProp(&n.text, name, v)
	case kindScroll:
		if name == "contentStyle" || name == "contentClass" {
			r.scrollContent(n, name, v)
			return true
		}
		known = r.scrollProp(&n.scroll, name, v)
		n.horizontal = n.scroll.Horizontal
	case kindInput:
		known = r.inputProp(&n.input, name, v)
	case kindImage:
		known = r.imageProp(&n.image, name, v)
	}
	if !known {
		r.warn("<%s> has no prop %q yet", kindNames[n.kind], name)
	}
	return known
}

// scrollContent sets a scroll view's contentStyle or contentClass: a view
// around its content, which scrolls with it and holds its children. The
// first one makes the view, moving the children made already into it.
func (r *runtime) scrollContent(n *node, name string, v js.Value) {
	if n.content == nil {
		n.content = r.newCoreElement(r.cx, core.View{})
		for _, c := range n.children {
			if c.kind != kindRaw {
				n.elem.Remove(c.elem)
				n.content.Insert(c.elem, nil)
			}
		}
		n.elem.Insert(n.content, nil)
	}
	if name == "contentStyle" {
		n.contentStyle = r.style(v)
	} else {
		n.contentClass = stringOf(v)
	}
	n.content.Set(core.View{Style: n.contentStyle, Class: n.contentClass})
}

// fullStyle is n's style with its states' styles.
func (n *node) fullStyle() core.Style {
	style := n.style
	for name, st := range n.states {
		st := st
		*stateProps[name](&style) = &st
	}
	return style
}

var kindNames = map[kind]string{kindView: "View", kindText: "Text", kindScroll: "Scroll", kindInput: "Input",
	kindImage: "Image"}

// apply gives n's element its props.
func (r *runtime) apply(n *node) {
	style := n.fullStyle()
	switch n.kind {
	case kindView:
		v := n.view
		v.Style, v.Class, v.Ref = style, n.classes(), n.ref
		r.touches(n, &v)
		r.gestures(n, &v)
		n.elem.Set(v)
	case kindText:
		r.textChanged(n)
	case kindScroll:
		v := n.scroll
		v.Style, v.Class, v.Ref = style, n.classes(), n.ref
		n.elem.Set(v)
	case kindInput:
		v := n.input
		v.Style, v.Class, v.Ref = style, n.classes(), n.ref
		n.elem.Set(v)
	case kindImage:
		v := n.image
		v.Style, v.Class, v.Ref = style, n.classes(), n.ref
		n.elem.Set(v)
	}
}
