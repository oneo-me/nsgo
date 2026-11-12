package tsx

import (
	"math"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/mobile"
)

// nsgo's Notifications and nsgo/mobile's Haptics (NSH's notifications.rs
// and haptics.rs) on core's API: the notifications held to the
// notifications capability, every call but on's rejecting without it, on
// throwing.

func (r *runtime) notifications() js.Value {
	o := r.vm.NewObject()
	set := func(name string, f func(call js.FunctionCall) js.Value) {
		_ = o.Set(name, r.function(func(call js.FunctionCall) js.Value {
			if p := r.refused(core.AccessNotifications); p != nil {
				return p
			}
			return f(call)
		}))
	}
	permission := func(ask bool) func(js.FunctionCall) js.Value {
		return func(js.FunctionCall) js.Value {
			p, done := r.promise()
			answer := func(perm core.NotificationPermission) {
				done(func() js.Value { return r.vm.ToValue(perm.String()) })
			}
			if ask {
				r.cx.RequestNotificationPermission(answer)
			} else {
				r.cx.NotificationPermission(answer)
			}
			return r.vm.ToValue(p)
		}
	}
	set("permission", permission(false))
	set("requestPermission", permission(true))
	set("show", func(call js.FunctionCall) js.Value {
		opt := r.options(arg(call, 0))
		n := core.Notification{ID: opt.str("id", ""), Title: opt.str("title", ""), Body: opt.str("body", ""),
			Silent: opt.bool("silent", false)}
		p, resolve, reject := r.vm.NewPromise()
		err := r.cx.ShowNotification(n, func(shown bool) { r.inJS(func() { _ = resolve(shown) }) })
		if err != nil {
			_ = reject(r.goError(&JSError{Name: "RangeError", Message: "Notifications.show: `id` and `title` must not be empty"}))
		}
		return r.vm.ToValue(p)
	})
	set("close", func(call js.FunctionCall) js.Value {
		r.cx.CloseNotification(arg(call, 0).String())
		p, done := r.promise()
		done(js.Undefined)
		return r.vm.ToValue(p)
	})
	set("setBadge", func(call js.FunctionCall) js.Value {
		count := arg(call, 0).ToFloat()
		p, resolve, reject := r.vm.NewPromise()
		if !(count >= 0 && count == math.Trunc(count) && count <= math.MaxInt32) {
			_ = reject(r.goError(&JSError{Name: "RangeError",
				Message: "Notifications.setBadge: count must be a whole number from 0"}))
			return r.vm.ToValue(p)
		}
		r.cx.SetBadge(int(count))
		_ = resolve(js.Undefined())
		return r.vm.ToValue(p)
	})
	_ = o.Set("on", r.function(func(call js.FunctionCall) js.Value {
		if err := r.checkAccess(core.AccessRequest{Permission: core.AccessNotifications}); err != nil {
			panic(r.goError(err))
		}
		if ev := arg(call, 0).String(); ev != "press" {
			panic(r.goError(&JSError{Name: "TypeError", Message: "Notifications.on: no event " + ev}))
		}
		fn := arg(call, 1)
		owner := r.cx.Child()
		owner.Run(func() {
			owner.OnNotificationPress(func(id string) {
				e := r.vm.NewObject()
				_ = e.Set("id", id)
				r.call(fn, e)
			})
		})
		return r.function(func(js.FunctionCall) js.Value {
			owner.Dispose()
			return js.Undefined()
		})
	}))
	return o
}

// hapticNames are Haptics' styles and kinds by their names.
var (
	hapticImpacts = map[string]core.Haptic{"light": core.HapticLight, "medium": core.HapticMedium,
		"heavy": core.HapticHeavy, "soft": core.HapticSoft, "rigid": core.HapticRigid}
	hapticOutcomes = map[string]core.Haptic{"success": core.HapticSuccess, "warning": core.HapticWarning,
		"error": core.HapticError}
)

func (r *runtime) haptics() js.Value {
	o := r.vm.NewObject()
	play := func(h core.Haptic) js.Value {
		mobile.PlayHaptic(r.cx, h)
		return js.Undefined()
	}
	named := func(call js.FunctionCall, names map[string]core.Haptic, def core.Haptic) core.Haptic {
		if v := arg(call, 0); defined(v) {
			if h, ok := names[v.String()]; ok {
				return h
			}
			panic(r.goError(&JSError{Name: "RangeError", Message: "Haptics: no style " + v.String()}))
		}
		return def
	}
	_ = o.Set("selection", r.function(func(js.FunctionCall) js.Value { return play(core.HapticSelection) }))
	_ = o.Set("impact", r.function(func(call js.FunctionCall) js.Value {
		return play(named(call, hapticImpacts, core.HapticMedium))
	}))
	_ = o.Set("notification", r.function(func(call js.FunctionCall) js.Value {
		return play(named(call, hapticOutcomes, core.HapticSuccess))
	}))
	return o
}
