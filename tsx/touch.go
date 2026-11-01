package tsx

import (
	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
)

// Touches and the responder (NSH's touch.rs): a view's touch handlers
// hear core's TouchEvents, and its responder handlers make a
// core.Responder, which core negotiates as React Native does. Events
// carry the gesture core follows, so PanResponder's handlers get their
// gesture state from it.

// touchProps are the touch and responder props, which make OnTouch and
// the Responder of a view together.
var touchProps = map[string]bool{
	"onTouchStart": true, "onTouchMove": true, "onTouchEnd": true, "onTouchCancel": true,
	"onTouchStartCapture": true, "onTouchMoveCapture": true, "onTouchEndCapture": true,
	"onTouchCancelCapture": true, "onStartShouldSetResponder": true, "onMoveShouldSetResponder": true,
	"onStartShouldSetResponderCapture": true, "onMoveShouldSetResponderCapture": true,
	"onResponderGrant": true, "onResponderReject": true, "onResponderStart": true, "onResponderMove": true,
	"onResponderEnd": true, "onResponderRelease": true, "onResponderTerminate": true,
	"onResponderTerminationRequest": true,
}

// touches makes v's OnTouch and Responder from n's touch props.
func (r *runtime) touches(n *node, v *core.View) {
	get := func(name string) goja.Value {
		if h := n.values[name]; isFunction(h) {
			return h
		}
		return nil
	}
	phases := map[core.TouchPhase][2]string{
		core.TouchStart: {"onTouchStartCapture", "onTouchStart"}, core.TouchMove: {"onTouchMoveCapture", "onTouchMove"},
		core.TouchEnd: {"onTouchEndCapture", "onTouchEnd"}, core.TouchCancel: {"onTouchCancelCapture", "onTouchCancel"},
	}
	// Core runs the capture handlers from the outermost view in, then the
	// others out, until one stops it.
	hear := func(capture int) func(core.TouchEvent) {
		for _, names := range phases {
			if get(names[capture]) != nil {
				return func(e core.TouchEvent) {
					if h := get(phases[e.Phase][capture]); h != nil {
						ev := r.touchEvent(e)
						_ = ev.Set("stopPropagation", r.function(func(goja.FunctionCall) goja.Value {
							e.StopPropagation()
							return goja.Undefined()
						}))
						r.call(h, ev)
					}
				}
			}
		}
		return nil
	}
	v.OnTouchCapture, v.OnTouch = hear(0), hear(1)

	var resp core.Responder
	any := false
	claim := func(name string, c core.TouchClaim) {
		h := get(name)
		if h == nil {
			return
		}
		any = true
		c.Ask = func(e core.ResponderEvent) bool {
			ev := r.responderEvent(e)
			return r.call(h, ev, r.gestureState(e, 0)).ToBoolean()
		}
		resp.Claims = append(resp.Claims, c)
	}
	claim("onStartShouldSetResponderCapture", core.TouchClaim{Capture: true})
	claim("onMoveShouldSetResponderCapture", core.TouchClaim{Capture: true, OnMove: true})
	claim("onStartShouldSetResponder", core.TouchClaim{})
	claim("onMoveShouldSetResponder", core.TouchClaim{OnMove: true})
	on := func(name string) func(core.ResponderEvent) {
		h := get(name)
		if h == nil {
			return nil
		}
		any = true
		return func(e core.ResponderEvent) { r.call(h, r.responderEvent(e)) }
	}
	resp.OnReject = on("onResponderReject")
	// As in React Native, a grant blocks the scroll views around only when
	// onResponderGrant returns true.
	grant := get("onResponderGrant")
	any = any || grant != nil
	resp.BlockNative = func(e core.ResponderEvent) bool {
		if grant == nil {
			return false
		}
		res := r.call(grant, r.responderEvent(e))
		return isBool(res) && res.ToBoolean()
	}
	resp.OnStart, resp.OnMove, resp.OnEnd = on("onResponderStart"), on("onResponderMove"), on("onResponderEnd")
	resp.OnRelease, resp.OnTerminate = on("onResponderRelease"), on("onResponderTerminate")
	if h := get("onResponderTerminationRequest"); h != nil {
		any = true
		resp.OnTerminationRequest = func(e core.ResponderEvent) bool {
			res := r.call(h, r.responderEvent(e))
			return !isBool(res) || res.ToBoolean()
		}
	}
	if !any {
		return
	}
	// A view with a pan activation becomes the responder when its pan
	// activates, and is not asked.
	if v.PanActivation != (core.PanActivation{}) {
		resp.Claims = []core.TouchClaim{{OnPan: true}}
	}
	v.Responder = &resp
}

func isBool(v goja.Value) bool {
	_, ok := v.Export().(bool)
	return ok
}

// touchPoint is a finger as a TouchPoint.
func (r *runtime) touchPoint(id int, at, page core.Point, ms float64) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("identifier", id)
	_ = o.Set("locationX", float64(at.X))
	_ = o.Set("locationY", float64(at.Y))
	_ = o.Set("pageX", float64(page.X))
	_ = o.Set("pageY", float64(page.Y))
	_ = o.Set("target", 0)
	_ = o.Set("timestamp", ms)
	return o
}

// touchEvent is a TouchEvent as a GestureResponderEvent about its first
// changed finger.
func (r *runtime) touchEvent(e core.TouchEvent) *goja.Object {
	ms := float64(e.Time.Microseconds()) / 1000
	lifted := e.Phase == core.TouchEnd || e.Phase == core.TouchCancel
	var down, changed []any
	var about *goja.Object
	for _, t := range e.Touches {
		p := r.touchPoint(t.ID, t.Position, t.PagePosition, ms)
		if t.Changed {
			changed = append(changed, p)
			if about == nil {
				about = r.touchPoint(t.ID, t.Position, t.PagePosition, ms)
			}
		}
		if !lifted || !t.Changed {
			down = append(down, p)
		}
	}
	if about == nil {
		about = r.vm.NewObject()
	}
	return r.finishEvent(about, down, changed)
}

func (r *runtime) finishEvent(e *goja.Object, down, changed []any) *goja.Object {
	_ = e.Set("touches", r.vm.NewArray(down...))
	_ = e.Set("changedTouches", r.vm.NewArray(changed...))
	_ = e.Set("nativeEvent", e)
	_ = e.Set("stopPropagation", r.function(func(goja.FunctionCall) goja.Value { return goja.Undefined() }))
	return e
}

// responderKey holds a responder event's gesture on its JS event.
var responderKey = goja.NewSymbol("nsgo.responder")

// responderEvent is what a responder handler gets.
func (r *runtime) responderEvent(e core.ResponderEvent) *goja.Object {
	ms := float64(e.Time.Microseconds()) / 1000
	o := r.touchPoint(0, e.Position, e.PagePosition, ms)
	self := r.touchPoint(0, e.Position, e.PagePosition, ms)
	r.finishEvent(o, []any{self}, []any{self})
	_ = o.SetSymbol(responderKey, r.vm.ToValue(&e))
	return o
}

// gestureState is a ResponderGestureState of e's gesture.
func (r *runtime) gestureState(e core.ResponderEvent, stateID int) *goja.Object {
	g := e.Gesture
	o := r.vm.NewObject()
	_ = o.Set("moveX", float64(g.Position.X))
	_ = o.Set("moveY", float64(g.Position.Y))
	_ = o.Set("x0", float64(g.Origin.X))
	_ = o.Set("y0", float64(g.Origin.Y))
	_ = o.Set("dx", float64(g.Delta.X))
	_ = o.Set("dy", float64(g.Delta.Y))
	// Points per millisecond, as React Native's.
	_ = o.Set("vx", float64(g.Velocity.X)/1000)
	_ = o.Set("vy", float64(g.Velocity.Y)/1000)
	_ = o.Set("numberActiveTouches", g.Touches)
	_ = o.Set("stateID", stateID)
	return o
}

// panResponder is PanResponder: create(config) gives panHandlers, the
// responder props that call config's handlers with the gesture state.
func (r *runtime) panResponder() goja.Value {
	m := r.vm.NewObject()
	_ = m.Set("create", r.function(func(call goja.FunctionCall) goja.Value {
		config := arg(call, 0).ToObject(r.vm)
		stateID := 0
		gesture := func(e goja.Value) (goja.Value, goja.Value) {
			if o, ok := e.(*goja.Object); ok {
				if s := o.GetSymbol(responderKey); s != nil {
					if ev, ok := s.Export().(*core.ResponderEvent); ok {
						return e, r.gestureState(*ev, stateID)
					}
				}
			}
			return e, r.vm.NewObject()
		}
		// handler forwards a responder prop to config's callback, with def
		// as its answer when config has none.
		handler := func(name string, def goja.Value, after func()) goja.Value {
			return r.function(func(call goja.FunctionCall) goja.Value {
				e, gs := gesture(arg(call, 0))
				res := def
				if cb := config.Get(name); isFunction(cb) {
					res = r.call(cb, e, gs)
					if def != nil && !isBool(res) {
						res = def
					}
				}
				if after != nil {
					after()
				}
				if res == nil {
					return goja.Undefined()
				}
				return res
			})
		}
		h := r.vm.NewObject()
		no, yes := r.vm.ToValue(false), r.vm.ToValue(true)
		next := func() { stateID++ }
		for prop, cb := range map[string]string{
			"onStartShouldSetResponderCapture": "onStartShouldSetPanResponderCapture",
			"onStartShouldSetResponder":        "onStartShouldSetPanResponder",
			"onMoveShouldSetResponderCapture":  "onMoveShouldSetPanResponderCapture",
			"onMoveShouldSetResponder":         "onMoveShouldSetPanResponder",
		} {
			_ = h.Set(prop, handler(cb, no, nil))
		}
		// The grant blocks the scroll views around unless
		// onShouldBlockNativeResponder says otherwise.
		_ = h.Set("onResponderGrant", r.function(func(call goja.FunctionCall) goja.Value {
			e, gs := gesture(arg(call, 0))
			if cb := config.Get("onPanResponderGrant"); isFunction(cb) {
				r.call(cb, e, gs)
			}
			block := true
			if cb := config.Get("onShouldBlockNativeResponder"); isFunction(cb) {
				block = r.call(cb, e, gs).ToBoolean()
			}
			return r.vm.ToValue(block)
		}))
		_ = h.Set("onResponderReject", handler("onPanResponderReject", nil, nil))
		_ = h.Set("onResponderStart", handler("onPanResponderStart", nil, nil))
		_ = h.Set("onResponderMove", handler("onPanResponderMove", nil, nil))
		_ = h.Set("onResponderEnd", handler("onPanResponderEnd", nil, nil))
		_ = h.Set("onResponderRelease", handler("onPanResponderRelease", nil, next))
		_ = h.Set("onResponderTerminate", handler("onPanResponderTerminate", nil, next))
		_ = h.Set("onResponderTerminationRequest", handler("onPanResponderTerminationRequest", yes, nil))
		o := r.vm.NewObject()
		_ = o.Set("panHandlers", h)
		return o
	}))
	return m
}
