//go:build js && wasm

package web

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// The page's host: nsgo.js, the loader, gives the app a bridge object in
// a global it names in the app's environment (NSGO_HOST), and the page's
// host answers through it. Values cross as JSON text.
//
//	params                  the launch parameters
//	call(id, name, data, post)
//	                        the app's call or post, which the loader
//	                        answers with reply
//	openURL(id, url)        a URL the app opens, answered with opened, or
//	                        absent when the host routes none
//	events                  what the page sent before the app ran
//	environment             the page's settings, as the native hosts'
//	                        (engine.HostEnvironment): appearance,
//	                        locale and fontScale
//
// call is absent when the page has no host to answer; the app's calls
// then fail. The app sets deliver(name, data), which hears each event the
// page sends, setEnvironment(data), which takes the page's settings from
// then on, and reply(id, ok, value) and opened(id, handled).

// pageHost is the app's end of the bridge.
type pageHost struct {
	bridge js.Value
	ch     *engine.Host
	mu     sync.Mutex
	next   int
	calls  map[int]*engine.HostCall
	urls   map[int]func(bool)
}

// connectHost returns the channel and the settings the bridge in global
// feeds; without one the app runs with no host. apply hears each change
// of the settings.
func connectHost(global string, apply func(engine.HostEnvironment)) engine.HostOptions {
	if global == "" {
		return engine.HostOptions{}
	}
	bridge := js.Global().Get(global)
	if bridge.Type() != js.TypeObject {
		log.Printf("nsgo: the page names a host bridge, %s, it does not have", global)
		return engine.HostOptions{}
	}
	h := &pageHost{bridge: bridge, ch: engine.NewHost(), calls: map[int]*engine.HostCall{}, urls: map[int]func(bool){}}
	if p := bridge.Get("params"); p.Type() == js.TypeString {
		var params map[string]any
		if err := json.Unmarshal([]byte(p.String()), &params); err != nil {
			log.Printf("nsgo: the host's parameters: %v", err)
		}
		h.ch.SetParams(params)
	}
	settings := &engine.HostSettings{}
	if e := bridge.Get("environment"); e.Type() == js.TypeString {
		settings.Environment = hostEnvironment(e.String())
		apply(settings.Environment)
	}
	h.set("setEnvironment", func(args []js.Value) {
		env := hostEnvironment(args[0].String())
		settings.SetEnvironment(env)
		apply(env)
	})
	if bridge.Get("call").Type() == js.TypeFunction {
		h.ch.HandleDefault(h.call)
	}
	if bridge.Get("openURL").Type() == js.TypeFunction {
		h.ch.HandleOpenURL(h.openURL)
	}
	h.set("reply", func(args []js.Value) {
		id, ok, value := args[0].Int(), args[1].Bool(), args[2].String()
		h.mu.Lock()
		c := h.calls[id]
		delete(h.calls, id)
		h.mu.Unlock()
		switch {
		case c == nil:
		case !ok:
			c.Reject(errors.New(value))
		default:
			var v any
			json.Unmarshal([]byte(value), &v)
			c.Resolve(v)
		}
	})
	h.set("opened", func(args []js.Value) {
		id, handled := args[0].Int(), args[1].Bool()
		h.mu.Lock()
		answer := h.urls[id]
		delete(h.urls, id)
		h.mu.Unlock()
		if answer != nil {
			answer(handled)
		}
	})
	h.set("deliver", func(args []js.Value) { h.deliver(args[0].String(), args[1].String()) })
	// What the page sent before the app ran comes first, in order.
	if q := bridge.Get("events"); q.Type() == js.TypeObject {
		for i := range q.Length() {
			ev := q.Index(i)
			h.deliver(ev.Index(0).String(), ev.Index(1).String())
		}
		bridge.Set("events", js.Null())
	}
	return engine.HostOptions{Channel: h.ch, Settings: settings}
}

// hostEnvironment reads the page's settings, JSON text.
func hostEnvironment(data string) engine.HostEnvironment {
	var page struct {
		Appearance string  `json:"appearance"`
		Locale     string  `json:"locale"`
		FontScale  float32 `json:"fontScale"`
	}
	if err := json.Unmarshal([]byte(data), &page); err != nil {
		log.Printf("nsgo: the page's settings: %v", err)
	}
	env := engine.HostEnvironment{Locale: page.Locale, FontScale: page.FontScale}
	switch page.Appearance {
	case "light":
		env.Appearance = driver.AppearanceLight
	case "dark":
		env.Appearance = driver.AppearanceDark
	}
	return env
}

// set gives the bridge fn as its method name, for the page's life.
func (h *pageHost) set(name string, fn func(args []js.Value)) {
	h.bridge.Set(name, js.FuncOf(func(_ js.Value, args []js.Value) any {
		fn(args)
		return nil
	}))
}

func (h *pageHost) deliver(name, data string) {
	var v any
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		log.Printf("nsgo: the page's event %q: %v", name, err)
		return
	}
	h.ch.Send(name, v)
}

// call hands the app's call or post to the page, which answers a call
// with reply.
func (h *pageHost) call(c *engine.HostCall) {
	data, err := json.Marshal(c.Data)
	if err != nil {
		c.Reject(err)
		return
	}
	h.mu.Lock()
	h.next++
	id := h.next
	if !c.IsPost() {
		h.calls[id] = c
	}
	h.mu.Unlock()
	h.bridge.Call("call", id, c.Name, string(data), c.IsPost())
}

// openURL offers the page's host a URL the app opens.
func (h *pageHost) openURL(url string, answer func(bool)) {
	h.mu.Lock()
	h.next++
	id := h.next
	h.urls[id] = answer
	h.mu.Unlock()
	h.bridge.Call("openURL", id, url)
}
