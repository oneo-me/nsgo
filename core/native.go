package core

import (
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Native shows a view of a component the app registered with its
// platform by name (nsgo.Component), filling the element's box. Props
// reach the component's SetProp when they change, events it emits reach
// OnEvent, and Ref.Command runs its commands. An unknown name shows an
// empty box.
type Native struct {
	// Component is the registered name.
	Component string
	// Props are the component's own props; PropsFunc, when set, replaces
	// them, again when what it reads changes. A prop that goes away is
	// set to nil. Values cross to Android as JSON. A prop whose value is
	// an AnimatedNode is bound to it instead: the platform's frame clock
	// hands the component its number or color on the frames it changes,
	// with no work for the engine (driver.AnimatedPropSetter).
	Props     map[string]any
	PropsFunc func() map[string]any
	Style     Style
	StyleFunc func() Style

	// OnEvent receives what the component emits.
	OnEvent  func(name string, data any)
	OnPress  func(PressEvent)
	OnLayout func(Bounds)
	Ref      *Ref
	Label    string
	TestID   string
}

// Render implements Node.
func (v Native) Render(*Scope) Node { return v }

func (v Native) mountIn(t *Tree, cx *Scope, parent *node) { t.mountNative(cx, parent, v) }

type nativeNode struct {
	component string
	props     map[string]any
	sent      map[string]any
	bound     []driver.NativeBinding // the bindings sent
	named     bool                   // the component's name was sent
	size      Size                   // what the component asked for
	event     func(name string, data any)
}

func (t *Tree) mountNative(cx *Scope, parent *node, v Native) {
	n := t.newNode(driver.KindNative, parent, cx)
	t.bindStyle(cx, n, v.Style, v.StyleFunc)
	t.setRef(n, v.Ref)
	n.h = handlers{press: v.OnPress, layout: v.OnLayout}
	n.props = driver.ViewProps{Label: v.Label, TestID: v.TestID}
	nn := &nativeNode{component: v.Component, props: v.Props, event: v.OnEvent}
	n.native = nn
	if v.PropsFunc != nil {
		cx.renderEffect(func() {
			nn.props = v.PropsFunc()
			t.touch(n)
		})
	}
}

// changes returns the props that differ from those sent, or nil, and
// records them as sent.
func (nn *nativeNode) changes() map[string]any {
	var out map[string]any
	set := func(k string, v any) {
		if out == nil {
			out = map[string]any{}
		}
		out[k] = v
	}
	for k, v := range nn.props {
		if old, ok := nn.sent[k]; !ok || !reflect.DeepEqual(old, v) {
			// A bound prop is not sent; a value replacing a node is.
			if !isBound(v) {
				set(k, v)
			}
		}
	}
	for k := range nn.sent {
		if _, ok := nn.props[k]; !ok {
			set(k, nil)
		}
	}
	if out != nil || !maps.EqualFunc(nn.props, nn.sent, func(a, b any) bool { return reflect.DeepEqual(a, b) }) {
		nn.sent = make(map[string]any, len(nn.props))
		for k, v := range nn.props {
			nn.sent[k] = v
		}
	}
	return out
}

// isBound reports whether a prop's value is a node of the animation
// graph, which drives the prop rather than being sent.
func isBound(v any) bool {
	a, ok := v.(AnimatedNode)
	return ok && a.animated() != nil
}

// bindings returns the props animated nodes drive, by name.
func (nn *nativeNode) bindings() []driver.NativeBinding {
	var out []driver.NativeBinding
	for k, v := range nn.props {
		if isBound(v) {
			n := v.(AnimatedNode).animated()
			out = append(out, driver.NativeBinding{Name: k, Node: n.id, Color: n.color})
		}
	}
	slices.SortFunc(out, func(a, b driver.NativeBinding) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// emitNative adds n's ItemNative when its component or props changed.
func (t *Tree) emitNative(n *node) {
	nn := n.native
	props := nn.changes()
	bindings := nn.bindings()
	bind := !slices.Equal(bindings, nn.bound)
	if nn.named && props == nil && !bind {
		return
	}
	nn.named = true
	nn.bound = bindings
	t.items = append(t.items, driver.MountItem{Kind: driver.ItemNative, ID: n.id,
		Native: &driver.NativeConfig{Component: nn.component, Props: props, Bind: bind, Bindings: bindings}})
}

// dispatchNative handles a native view's events.
func (t *Tree) dispatchNative(n *node, ev driver.Event) bool {
	nn := n.native
	switch ev.Kind {
	case driver.EventNative:
		if nn.event != nil {
			t.within(n.owner, func() { nn.event(ev.Text, ev.Data) })
		}
	case driver.EventNativeSize:
		if ev.Size != nn.size {
			nn.size = ev.Size
			t.relayout(n)
		}
	default:
		return false
	}
	return true
}

// Command has the component of the native view r names run name with
// args.
func (r *Ref) Command(name string, args any) {
	if r.n == nil || r.n.deleted || r.n.native == nil {
		return
	}
	r.t.commands = append(r.t.commands, driver.MountItem{Kind: driver.ItemNativeCommand, ID: r.n.id, Value: name, Data: args})
	r.t.changed = true
}
