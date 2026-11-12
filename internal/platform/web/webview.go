//go:build js && wasm

package web

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// webFrame is a web view's <iframe>, as NSH's web backend has it. HTML
// loads as srcdoc in a sandbox with an origin of its own, so it never
// reaches the app's page, with a bridge script first: the first message
// carrying token hands the page a MessagePort, which
// window.nsgo.postMessage writes to and which runs injected scripts and
// history steps. A URL loads as it is (without its origin when it is the
// app's own); the browser hides what such a page does, so it only gets
// PostMessage and load events with the URL it was given.
type webFrame struct {
	frame  js.Value
	config driver.WebConfig
	loaded bool // configured at least once
	token  string
	port   js.Value // the page's end of the bridge, once it loaded
	portFn js.Func
	url    string
	title  string
	warned bool // that commands do not reach the page
}

// webBridge is the script an HTML source starts with.
func webBridge(token string) string {
	return `(() => {
  let port = null;
  const queue = [];
  window.nsgo = { postMessage(data) { data = String(data); port ? port.postMessage(data) : queue.push(data); } };
  addEventListener("message", (e) => {
    if (port || e.source !== parent || e.data !== ` + strconv.Quote(token) + ` || !e.ports[0]) return;
    e.stopImmediatePropagation();
    port = e.ports[0];
    port.onmessage = (m) => {
      const d = m.data;
      if (d && d.eval != null) { try { (0, eval)(String(d.eval)); } catch (err) { console.error(err); } }
      else if (d && d.go) history.go(d.go);
    };
    for (const data of queue.splice(0)) port.postMessage(data);
  });
})();`
}

func (s *surface) createWeb(v *view) {
	w := &webFrame{frame: s.element("iframe")}
	v.web = w
	w.frame.Call("setAttribute", "allowtransparency", "true")
	setStyle(w.frame, map[string]string{
		"width": "100%", "height": "100%", "border": "0", "background": "transparent", "border-radius": "inherit",
	})
	v.inner = w.frame
	v.el.Call("appendChild", w.frame)
	v.listeners.on(w.frame, "load", func(js.Value) { s.webLoaded(v) })
}

// sameFrame reports whether a and b load the same frame, comparing what
// NSH's web backend does: the user agent and swipes cannot apply to a
// frame.
func sameFrame(a, b *driver.WebConfig) bool {
	return a.Source == b.Source && a.InjectedJavaScript == b.InjectedJavaScript &&
		a.InjectedJavaScriptBeforeContent == b.InjectedJavaScriptBeforeContent &&
		a.NoScroll == b.NoScroll && a.NoJavaScript == b.NoJavaScript
}

func (s *surface) setWeb(v *view, c *driver.WebConfig) {
	w := v.web
	if w.loaded && sameFrame(c, &w.config) {
		return
	}
	w.loaded, w.config = true, *c
	w.closePort()
	w.token = ""
	src := c.Source
	if src.HTML != "" && !c.NoJavaScript {
		w.token = strconv.FormatUint(rand.Uint64(), 36)
	}
	sandbox := []string{"allow-forms", "allow-popups", "allow-popups-to-escape-sandbox", "allow-modals", "allow-downloads"}
	if !c.NoJavaScript {
		sandbox = append(sandbox, "allow-scripts")
	}
	if src.HTML == "" && src.Src != "" {
		// Pages of other sites keep their origin, so their storage works;
		// one of the app's own must not reach the app.
		url := js.Global().Get("URL")
		same := false
		if u, ok := newURL(url, src.Src); ok {
			same = u.Get("origin").String() == js.Global().Get("location").Get("origin").String()
		}
		if !same {
			sandbox = append(sandbox, "allow-same-origin")
		}
	}
	w.frame.Call("setAttribute", "sandbox", strings.Join(sandbox, " "))
	script := func(js string) string {
		return "<script>" + strings.ReplaceAll(js, "</script", `<\/script`) + "</script>"
	}
	if src.HTML != "" {
		var head strings.Builder
		if src.BaseURL != "" {
			head.WriteString(`<base href="` + strings.ReplaceAll(src.BaseURL, `"`, "&quot;") + `">`)
		}
		if w.token != "" {
			head.WriteString(script(webBridge(w.token)))
		}
		if c.NoScroll {
			head.WriteString("<style>html,body{overflow:hidden!important}</style>")
		}
		if c.InjectedJavaScriptBeforeContent != "" {
			head.WriteString(script(c.InjectedJavaScriptBeforeContent))
		}
		tail := ""
		if c.InjectedJavaScript != "" {
			tail = script(c.InjectedJavaScript)
		}
		// After a doctype, which must stay first.
		html, doctype := src.HTML, ""
		if trimmed := strings.TrimLeft(html, " \t\r\n"); len(trimmed) > 9 && strings.EqualFold(trimmed[:9], "<!doctype") {
			if end := strings.IndexByte(trimmed, '>'); end >= 0 {
				doctype, html = trimmed[:end+1], trimmed[end+1:]
			}
		}
		w.url = src.BaseURL
		if w.url == "" {
			w.url = "about:srcdoc"
		}
		doc := js.Global().Get("DOMParser").New().Call("parseFromString", src.HTML, "text/html")
		w.title = doc.Get("title").String()
		w.frame.Call("removeAttribute", "src")
		w.frame.Call("removeAttribute", "scrolling")
		w.frame.Set("srcdoc", doctype+head.String()+html+tail)
	} else {
		w.url, w.title = src.Src, ""
		w.frame.Call("removeAttribute", "srcdoc")
		if c.NoScroll {
			w.frame.Call("setAttribute", "scrolling", "no")
		} else {
			w.frame.Call("removeAttribute", "scrolling")
		}
		if src.Src != "" {
			w.frame.Set("src", src.Src)
		} else {
			w.frame.Call("removeAttribute", "src")
		}
	}
	if src.HTML != "" || src.Src != "" {
		// After the batch, which may carry the view's events after its
		// configuration.
		var start js.Func
		start = js.FuncOf(func(js.Value, []js.Value) any {
			start.Release()
			if s.view(v.id) == v {
				s.webNavigation(v, driver.EventLoadStart, true)
			}
			return nil
		})
		js.Global().Call("queueMicrotask", start)
	}
}

func newURL(ctor js.Value, s string) (u js.Value, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return ctor.New(s, js.Global().Get("location").Get("href")), true
}

// webNavigation reports the frame's state, as far as the browser shows
// it, with kind and as a navigation change.
func (s *surface) webNavigation(v *view, kind driver.EventKind, loading bool) {
	nav := driver.WebNavigation{URL: v.web.url, Title: v.web.title, Loading: loading}
	for _, k := range []driver.EventKind{kind, driver.EventNavigationStateChange} {
		n := nav
		s.send(v, driver.Event{Kind: k, Navigation: &n})
	}
}

// webLoaded hands a loaded HTML page its end of the bridge.
func (s *surface) webLoaded(v *view) {
	w := v.web
	if !w.frame.Call("hasAttribute", "src").Bool() && !w.frame.Call("hasAttribute", "srcdoc").Bool() {
		return
	}
	w.closePort()
	if w.token != "" {
		channel := js.Global().Get("MessageChannel").New()
		w.port = channel.Get("port1")
		w.portFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
			s.send(v, driver.Event{Kind: driver.EventMessage, Text: args[0].Get("data").String()})
			return nil
		})
		w.port.Set("onmessage", w.portFn)
		if win := w.frame.Get("contentWindow"); win.Truthy() {
			win.Call("postMessage", w.token, "*", []any{channel.Get("port2")})
		}
	}
	s.webNavigation(v, driver.EventLoad, false)
}

func (w *webFrame) closePort() {
	if w.port.Truthy() {
		w.port.Call("close")
		w.port.Set("onmessage", js.Null())
		w.portFn.Release()
		w.port = js.Undefined()
	}
}

func (s *surface) webCommand(v *view, c driver.WebCommand, value string) {
	w := v.web
	viaPort := func(name string, msg map[string]any) {
		if w.port.Truthy() {
			w.port.Call("postMessage", msg)
		} else if !w.warned {
			w.warned = true
			js.Global().Get("console").Call("warn", "nsgo: web view: "+name+
				" only reaches HTML sources (a page of another site is the browser's own)")
		}
	}
	switch c {
	case driver.WebReload:
		// The frame is another origin: it loads again rather than is told to.
		if w.frame.Call("hasAttribute", "srcdoc").Bool() {
			w.frame.Set("srcdoc", w.frame.Get("srcdoc"))
		} else if src := w.frame.Call("getAttribute", "src"); src.Truthy() {
			w.frame.Set("src", src)
		}
		s.webNavigation(v, driver.EventLoadStart, true)
	case driver.WebPostMessage:
		if win := w.frame.Get("contentWindow"); win.Truthy() {
			win.Call("postMessage", value, "*")
		}
	case driver.WebInjectJavaScript:
		viaPort("InjectJavaScript", map[string]any{"eval": value})
	case driver.WebGoBack:
		viaPort("GoBack", map[string]any{"go": -1})
	case driver.WebGoForward:
		viaPort("GoForward", map[string]any{"go": 1})
	}
	// StopLoading does nothing, as in NSH: the frame is another origin.
}
