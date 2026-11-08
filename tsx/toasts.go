package tsx

import (
	"reflect"
	"time"

	"github.com/oneo-me/nsgo/tsx/internal/js"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/toast"
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
func (r *runtime) alertDialog() js.Value {
	o := r.vm.NewObject()
	_ = o.Set("show", r.function(func(call js.FunctionCall) js.Value {
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
			done(func() js.Value {
				switch id, ok := ids[label]; {
				case label == "":
					return js.Null()
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

// toastKey holds a JS ToastManager's toast.Manager.
var toastKey = js.NewSymbol("nsgo.toasts")

type toastManager = toast.Manager[string, js.Value]

func (r *runtime) toastMotion(v js.Value) toast.Motion {
	o := r.options(v)
	return toast.Motion{Duration: millis(o.float("duration", 0)), ExitDuration: millis(o.float("exitDuration", 0)),
		CollapsedPeek: core.Points(o.float("collapsedPeek", 0)), ExpandedGap: core.Points(o.float("expandedGap", 0)),
		CollapsedScaleStep: float32(o.float("collapsedScaleStep", 0)), CollapsedVisible: int(o.float("collapsedVisible", 0))}
}

// toastManagerClass is the ToastManager class: base's manager on the
// app's clock.
func (r *runtime) toastManagerClass() js.Value {
	class := r.vm.ToValue(func(call js.ConstructorCall) *js.Object {
		m := &toastManager{Motion: r.toastMotion(call.Argument(0))}
		_ = call.This.SetSymbol(toastKey, m)
		return nil
	}).(*js.Object)
	proto := class.Get("prototype").(*js.Object)
	method := func(name string, f func(m *toastManager, call js.FunctionCall) js.Value) {
		_ = proto.Set(name, r.function(func(call js.FunctionCall) js.Value {
			o, _ := call.This.(*js.Object)
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
	entry := func(e toast.Entry[string, js.Value]) *js.Object {
		o := r.vm.NewObject()
		_ = o.Set("id", e.ID)
		_ = o.Set("value", e.Value)
		_ = o.Set("status", map[toast.Status]string{toast.Starting: "starting", toast.Present: "present",
			toast.Ending: "ending"}[e.Status])
		return o
	}
	entries := func(es []toast.Entry[string, js.Value]) js.Value {
		out := make([]any, len(es))
		for i, e := range es {
			out[i] = entry(e)
		}
		return r.vm.NewArray(out...)
	}
	// strings is a JS array of ids, empty without any.
	strings := func(ids []string) js.Value {
		out := make([]any, len(ids))
		for i, id := range ids {
			out[i] = id
		}
		return r.vm.NewArray(out...)
	}
	method("push", func(m *toastManager, call js.FunctionCall) js.Value {
		timeout := millis(r.options(arg(call, 2)).float("timeout", 4000))
		old, ok := m.Push(arg(call, 0).String(), arg(call, 1), timeout, now())
		if !ok {
			return js.Null()
		}
		return old
	})
	method("dismiss", func(m *toastManager, call js.FunctionCall) js.Value {
		return r.vm.ToValue(m.Dismiss(arg(call, 0).String(), now()))
	})
	method("dismissAll", func(m *toastManager, call js.FunctionCall) js.Value {
		return strings(m.DismissAll(now()))
	})
	method("advance", func(m *toastManager, call js.FunctionCall) js.Value {
		a := m.Advance(now(), arg(call, 0).ToBoolean())
		o := r.vm.NewObject()
		_ = o.Set("changed", a.Changed)
		_ = o.Set("presented", strings(a.Presented))
		_ = o.Set("ending", strings(a.Ending))
		_ = o.Set("removed", entries(a.Removed))
		return o
	})
	method("entries", func(m *toastManager, call js.FunctionCall) js.Value { return entries(m.Entries()) })
	method("visible", func(m *toastManager, call js.FunctionCall) js.Value {
		return entries(m.Visible(int(arg(call, 0).ToInteger())))
	})
	method("get", func(m *toastManager, call js.FunctionCall) js.Value {
		v, ok := m.Get(arg(call, 0).String())
		if !ok {
			return js.Null()
		}
		return v
	})
	method("len", func(m *toastManager, call js.FunctionCall) js.Value { return r.vm.ToValue(m.Len()) })
	method("isEmpty", func(m *toastManager, call js.FunctionCall) js.Value { return r.vm.ToValue(m.Len() == 0) })
	method("dispose", func(m *toastManager, call js.FunctionCall) js.Value {
		m.DismissAll(now())
		return js.Undefined()
	})
	return class
}

// toastStackFace lays out its children's toasts.
func toastStackFace(p *props) core.Node {
	r := p.r
	// The children made again only as what they read changes, as <Each>
	// keeps its rows: made at each read, they would be other nodes, which
	// the stack could not match.
	children := core.NewMemoFunc(r.cx, func() js.Value { return p.get("children") }, nil)
	return toast.Stack[js.Value, any]{
		Toasts: func() []js.Value {
			var out []js.Value
			for _, it := range r.resolve(children.Get(), nil, 0) {
				if it.node != nil {
					out = append(out, it.node.value())
				}
			}
			return out
		},
		Key: func(v js.Value) any { return keyOf(v) },
		Toast: func(cx *core.Scope, item func() js.Value) core.Node {
			return r.region(func() js.Value { return item() })
		},
		Motion:          r.toastMotion(p.get("motion")),
		Placement:       r.decode(p.get("placement"), reflect.TypeFor[toast.Anchor]()).Interface().(toast.Anchor),
		NoCloseOnEscape: !p.boolOr("closeOnEscape", true),
		OnExpandedChange: func(expanded bool) {
			e := r.vm.NewObject()
			_ = e.Set("expanded", expanded)
			callIf(p, "onExpandedChange", e)
		},
		OnCancel:  p.on("onCancel"),
		Class:     p.class(),
		StyleFunc: p.style,
		TestID:    p.str("testID"),
	}
}

func callIf(p *props, name string, args ...js.Value) {
	if h := p.handler(name); h != nil {
		h(args...)
	}
}
