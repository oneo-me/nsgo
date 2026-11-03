package tsx

import (
	"reflect"
	"time"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// AlertDialog, ToastManager and ToastStack of nsgo/base: base's, over
// JS values. A ToastStack's toasts are its children, matched by node.

func init() {
	moduleTable["nsgo/base"] = append(moduleTable["nsgo/base"],
		export{"AlertDialog", (*runtime).alertDialog},
		export{"ToastManager", (*runtime).toastManagerClass},
		faceExport("ToastStack", toastStackFace),
	)
}

var alertRoles = map[string]core.AlertRole{"default": core.AlertDefault, "cancel": core.AlertCancel,
	"destructive": core.AlertDestructive}

// alertDialog is AlertDialog: show(options) resolves with the chosen
// action's id, else its label, or null when closed without one.
func (r *runtime) alertDialog() goja.Value {
	o := r.vm.NewObject()
	_ = o.Set("show", r.function(func(call goja.FunctionCall) goja.Value {
		opt := r.options(arg(call, 0))
		d := base.AlertDialog{Title: opt.str("title", ""), Description: opt.str("description", "")}
		if opt.str("presentation", "") == "actionSheet" {
			d.Presentation = base.AlertPresentActionSheet
		}
		toggle := func(k string) core.Toggle {
			switch {
			case !opt.has(k):
				return core.ToggleInherit
			case opt.bool(k, false):
				return core.ToggleOn
			}
			return core.ToggleOff
		}
		d.CloseOnEscape, d.CloseOnBackdropPress = toggle("closeOnEscape"), toggle("closeOnBackdropPress")
		ids := map[string]string{}
		for _, a := range r.arrayItems(opt.get("actions")) {
			ao := r.options(a)
			label := ao.str("label", "")
			d.Actions = append(d.Actions, core.AlertAction{Label: label, Role: alertRoles[ao.str("role", "default")]})
			if ao.has("id") {
				ids[label] = ao.str("id", "")
			}
		}
		if len(d.Actions) == 0 {
			p, _, reject := r.vm.NewPromise()
			_ = reject(r.vm.NewTypeError("AlertDialog.show needs at least one of its actions"))
			return r.vm.ToValue(p)
		}
		p, done := r.promise()
		base.ShowAlertDialog(r.cx, d, func(label string) {
			done(func() goja.Value {
				switch id, ok := ids[label]; {
				case label == "":
					return goja.Null()
				case ok:
					return r.vm.ToValue(id)
				}
				return r.vm.ToValue(label)
			})
		})
		return r.vm.ToValue(p)
	}))
	return o
}

// toastKey holds a JS ToastManager's base.ToastManager.
var toastKey = goja.NewSymbol("nsgo.toasts")

type toastManager = base.ToastManager[string, goja.Value]

func (r *runtime) toastMotion(v goja.Value) base.ToastMotion {
	o := r.options(v)
	return base.ToastMotion{Duration: millis(o.float("duration", 0)), ExitDuration: millis(o.float("exitDuration", 0)),
		CollapsedPeek: core.Points(o.float("collapsedPeek", 0)), ExpandedGap: core.Points(o.float("expandedGap", 0)),
		CollapsedScaleStep: float32(o.float("collapsedScaleStep", 0)), CollapsedVisible: int(o.float("collapsedVisible", 0))}
}

// toastManagerClass is the ToastManager class: base's manager on the
// app's clock.
func (r *runtime) toastManagerClass() goja.Value {
	class := r.vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		m := &toastManager{Motion: r.toastMotion(call.Argument(0))}
		_ = call.This.SetSymbol(toastKey, m)
		return nil
	}).(*goja.Object)
	proto := class.Get("prototype").(*goja.Object)
	method := func(name string, f func(m *toastManager, call goja.FunctionCall) goja.Value) {
		_ = proto.Set(name, r.function(func(call goja.FunctionCall) goja.Value {
			o, _ := call.This.(*goja.Object)
			var m *toastManager
			if o != nil {
				if s := o.GetSymbol(toastKey); s != nil {
					m, _ = s.Export().(*toastManager)
				}
			}
			if m == nil {
				panic(r.vm.NewTypeError("nsgo: %s of something that is not a ToastManager", name))
			}
			return f(m, call)
		}))
	}
	now := func() time.Time { return r.cx.Now() }
	entry := func(e base.ToastEntry[string, goja.Value]) *goja.Object {
		o := r.vm.NewObject()
		_ = o.Set("id", e.ID)
		_ = o.Set("value", e.Value)
		_ = o.Set("status", map[base.ToastStatus]string{base.ToastStarting: "starting", base.ToastPresent: "present",
			base.ToastEnding: "ending"}[e.Status])
		return o
	}
	entries := func(es []base.ToastEntry[string, goja.Value]) goja.Value {
		out := make([]any, len(es))
		for i, e := range es {
			out[i] = entry(e)
		}
		return r.vm.NewArray(out...)
	}
	// strings is a JS array of ids, empty without any.
	strings := func(ids []string) goja.Value {
		out := make([]any, len(ids))
		for i, id := range ids {
			out[i] = id
		}
		return r.vm.NewArray(out...)
	}
	method("push", func(m *toastManager, call goja.FunctionCall) goja.Value {
		timeout := millis(r.options(arg(call, 2)).float("timeout", 4000))
		old, ok := m.Push(arg(call, 0).String(), arg(call, 1), timeout, now())
		if !ok {
			return goja.Null()
		}
		return old
	})
	method("dismiss", func(m *toastManager, call goja.FunctionCall) goja.Value {
		return r.vm.ToValue(m.Dismiss(arg(call, 0).String(), now()))
	})
	method("dismissAll", func(m *toastManager, call goja.FunctionCall) goja.Value {
		return strings(m.DismissAll(now()))
	})
	method("advance", func(m *toastManager, call goja.FunctionCall) goja.Value {
		a := m.Advance(now(), arg(call, 0).ToBoolean())
		o := r.vm.NewObject()
		_ = o.Set("changed", a.Changed)
		_ = o.Set("presented", strings(a.Presented))
		_ = o.Set("ending", strings(a.Ending))
		_ = o.Set("removed", entries(a.Removed))
		return o
	})
	method("entries", func(m *toastManager, call goja.FunctionCall) goja.Value { return entries(m.Entries()) })
	method("visible", func(m *toastManager, call goja.FunctionCall) goja.Value {
		return entries(m.Visible(int(arg(call, 0).ToInteger())))
	})
	method("get", func(m *toastManager, call goja.FunctionCall) goja.Value {
		v, ok := m.Get(arg(call, 0).String())
		if !ok {
			return goja.Null()
		}
		return v
	})
	method("len", func(m *toastManager, call goja.FunctionCall) goja.Value { return r.vm.ToValue(m.Len()) })
	method("isEmpty", func(m *toastManager, call goja.FunctionCall) goja.Value { return r.vm.ToValue(m.Len() == 0) })
	method("dispose", func(m *toastManager, call goja.FunctionCall) goja.Value {
		m.DismissAll(now())
		return goja.Undefined()
	})
	return class
}

// toastStackFace lays out its children's toasts.
func toastStackFace(p *props) core.Node {
	r := p.r
	// The children made again only as what they read changes, as <Each>
	// keeps its rows: made at each read, they would be other nodes, which
	// the stack could not match.
	children := core.NewMemoFunc(r.cx, func() goja.Value { return p.get("children") }, nil)
	return base.ToastStack[goja.Value, any]{
		Toasts: func() []goja.Value {
			var out []goja.Value
			for _, it := range r.resolve(children.Get(), nil, 0) {
				if it.node != nil {
					out = append(out, it.node.value())
				}
			}
			return out
		},
		Key: func(v goja.Value) any { return keyOf(v) },
		Toast: func(cx *core.Scope, item func() goja.Value) core.Node {
			return r.region(func() goja.Value { return item() })
		},
		Motion:           r.toastMotion(p.get("motion")),
		Placement:        r.decode(p.get("placement"), reflect.TypeFor[base.Anchor]()).Interface().(base.Anchor),
		NoCloseOnEscape:  !p.boolOr("closeOnEscape", true),
		OnExpandedChange: func(e bool) { callIf(p, "onExpandedChange", r.vm.ToValue(e)) },
		OnCancel:         p.on("onCancel"),
		Class:            p.class(),
		StyleFunc:        p.style,
		TestID:           p.str("testID"),
	}
}

func callIf(p *props, name string, args ...goja.Value) {
	if h := p.handler(name); h != nil {
		h(args...)
	}
}
