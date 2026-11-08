package tsx

import (
	"github.com/oneo-me/nsgo/tsx/internal/js"

	"github.com/oneo-me/nsgo/core"
)

// speech is NSH's Speech: available() and start({ lang, interim }), whose
// session tells its start, results and end to the listeners on() adds, on
// core's StartSpeech.
func (r *runtime) speech() js.Value {
	o := r.vm.NewObject()
	_ = o.Set("available", r.function(func(js.FunctionCall) js.Value {
		p, resolve, _ := r.vm.NewPromise()
		_ = resolve(core.Untracked(r.cx, r.cx.Env().CanRecognizeSpeech))
		return r.vm.ToValue(p)
	}))
	_ = o.Set("start", r.function(func(call js.FunctionCall) js.Value {
		op := r.options(arg(call, 0))
		listeners := map[string][]*js.Value{}
		emit := func(name string, e js.Value) {
			for _, l := range append([]*js.Value(nil), listeners[name]...) {
				r.call(*l, e)
			}
		}
		session := r.vm.NewObject()
		s, err := r.cx.StartSpeech(core.SpeechOptions{
			Lang:    op.str("lang", ""),
			Interim: op.bool("interim", false),
			OnStart: func() { emit("start", r.vm.NewObject()) },
			OnResult: func(text string, final bool) {
				e := r.vm.NewObject()
				_ = e.Set("text", text)
				_ = e.Set("isFinal", final)
				emit("result", e)
			},
			OnEnd: func(text string, err error) {
				e := r.vm.NewObject()
				_ = e.Set("text", text)
				if err != nil {
					if code, ok := err.(core.SpeechError); ok {
						_ = e.Set("error", string(code))
					}
				}
				emit("end", e)
			},
		})
		if err != nil {
			panic(r.goError(&JSError{Name: "RangeError", Message: err.Error()}))
		}
		_ = session.Set("on", r.function(func(call js.FunctionCall) js.Value {
			name, l := arg(call, 0).String(), arg(call, 1)
			if _, ok := js.AssertFunction(l); !ok {
				panic(r.vm.NewTypeError("nsgo: a SpeechSession listener must be a function"))
			}
			p := &l
			listeners[name] = append(listeners[name], p)
			return r.function(func(js.FunctionCall) js.Value {
				for i, q := range listeners[name] {
					if q == p {
						listeners[name] = append(listeners[name][:i:i], listeners[name][i+1:]...)
						break
					}
				}
				return js.Undefined()
			})
		}))
		get := func(name string, v func() any) {
			_ = session.DefineAccessorProperty(name, r.function(func(js.FunctionCall) js.Value { return r.vm.ToValue(v()) }),
				nil, js.FLAG_FALSE, js.FLAG_TRUE)
		}
		get("text", func() any { return s.Text() })
		get("ended", func() any { return s.Ended() })
		_ = session.Set("stop", r.function(func(js.FunctionCall) js.Value {
			s.Stop()
			return js.Undefined()
		}))
		_ = session.Set("abort", r.function(func(js.FunctionCall) js.Value {
			s.Abort()
			return js.Undefined()
		}))
		return session
	}))
	return o
}
