package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

// webTestPage runs the test binary in the page: its arguments and
// environment come in the query, its exit goes to the tool through the
// nsgoTest binding.
const webTestPage = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>html, body { margin: 0; height: 100%; }</style>
<script src="wasm_exec.js"></script>
</head>
<body>
<script>
const query = new URLSearchParams(location.search);
const go = new Go();
go.argv = ["test"].concat(JSON.parse(query.get("args") || "[]"));
go.env = JSON.parse(query.get("env") || "{}");
go.exit = (code) => nsgoTest(JSON.stringify({ op: "exit", code }));
WebAssembly.instantiateStreaming(fetch("app.wasm"), go.importObject)
  .then((r) => go.run(r.instance))
  .catch((e) => { console.error(String(e)); go.exit(2); });
</script>
</body>
</html>
`

// webTestRunner serves the test build on this machine and runs each test
// in a page of a headless browser, in a browser context of its own.
func webTestRunner(ctx context.Context, p *project, dir string, opts testOptions) (targetRunner, error) {
	if err := buildWeb(ctx, p, dir, true); err != nil {
		return targetRunner{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "test.html"), []byte(webTestPage), 0o644); err != nil {
		return targetRunner{}, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return targetRunner{}, err
	}
	srv := &http.Server{Handler: http.FileServer(http.Dir(dir))}
	go srv.Serve(ln)
	browser, err := startBrowser(ctx)
	if err != nil {
		srv.Close()
		return targetRunner{}, err
	}
	base := fmt.Sprintf("http://%s/test.html", ln.Addr())
	return targetRunner{
		run: func(ctx context.Context, args, env []string, out io.Writer) (int, error) {
			return runWebTest(ctx, browser, base, args, env, out)
		},
		close: func() {
			browser.close()
			srv.Close()
		},
	}, nil
}

// webOp is what the page asks the tool to do.
type webOp struct {
	ID   int     `json:"id"`
	Op   string  `json:"op"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	DX   float64 `json:"dx"`
	DY   float64 `json:"dy"`
	Hold int64   `json:"hold,omitempty"`
	Path []struct {
		X, Y float64
	} `json:"path,omitempty"`
	Text   string  `json:"text,omitempty"`
	Key    string  `json:"key,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
	Code   int     `json:"code,omitempty"`
}

// runWebTest runs one test's process: a page in a fresh context.
func runWebTest(ctx context.Context, b *cdp, base string, args, env []string, out io.Writer) (int, error) {
	raw, err := b.call("", "Target.createBrowserContext", map[string]any{"disposeOnDetach": true})
	if err != nil {
		return -1, err
	}
	var bctx struct{ BrowserContextID string }
	json.Unmarshal(raw, &bctx)
	defer b.call("", "Target.disposeBrowserContext", map[string]any{"browserContextId": bctx.BrowserContextID})
	raw, err = b.call("", "Target.createTarget", map[string]any{"url": "about:blank", "browserContextId": bctx.BrowserContextID})
	if err != nil {
		return -1, err
	}
	var target struct{ TargetID string }
	json.Unmarshal(raw, &target)
	raw, err = b.call("", "Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true})
	if err != nil {
		return -1, err
	}
	var attached struct{ SessionID string }
	json.Unmarshal(raw, &attached)
	session := attached.SessionID
	page := &webPage{b: b, session: session}

	exited := make(chan int, 1)
	var outMu sync.Mutex
	write := func(s string) {
		outMu.Lock()
		io.WriteString(out, s)
		outMu.Unlock()
	}
	b.mu.Lock()
	b.on = func(m cdpMessage) {
		if m.SessionID != session {
			return
		}
		switch m.Method {
		case "Runtime.consoleAPICalled":
			var ev struct {
				Args []struct {
					Value       any
					Description string
				}
			}
			json.Unmarshal(m.Params, &ev)
			var parts []string
			for _, a := range ev.Args {
				if s, ok := a.Value.(string); ok {
					parts = append(parts, s)
				} else if a.Description != "" {
					parts = append(parts, a.Description)
				} else {
					parts = append(parts, fmt.Sprint(a.Value))
				}
			}
			write(strings.Join(parts, " ") + "\n")
		case "Runtime.exceptionThrown":
			var ev struct {
				ExceptionDetails struct {
					Text      string
					Exception struct{ Description string }
				}
			}
			json.Unmarshal(m.Params, &ev)
			write("uncaught: " + ev.ExceptionDetails.Text + " " + ev.ExceptionDetails.Exception.Description + "\n")
		case "Runtime.bindingCalled":
			var ev struct{ Name, Payload string }
			json.Unmarshal(m.Params, &ev)
			var op webOp
			if ev.Name != "nsgoTest" || json.Unmarshal([]byte(ev.Payload), &op) != nil {
				return
			}
			if op.Op == "exit" {
				exited <- op.Code
				return
			}
			// Off the reading goroutine: an op calls the browser itself.
			go page.answer(op)
		}
	}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.on = nil
		b.mu.Unlock()
	}()
	for _, call := range []struct {
		method string
		params any
	}{
		{"Runtime.enable", nil},
		{"Page.enable", nil},
		{"Runtime.addBinding", map[string]any{"name": "nsgoTest"}},
		{"Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": webTestScale, "mobile": false}},
	} {
		if _, err := b.call(session, call.method, call.params); err != nil {
			return -1, err
		}
	}
	argData, _ := json.Marshal(args)
	envMap := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		envMap[k] = v
	}
	envData, _ := json.Marshal(envMap)
	u := base + "?args=" + url.QueryEscape(string(argData)) + "&env=" + url.QueryEscape(string(envData))
	if _, err := b.call(session, "Page.navigate", map[string]any{"url": u}); err != nil {
		return -1, err
	}
	select {
	case code := <-exited:
		return code, nil
	case <-ctx.Done():
		return -1, fmt.Errorf("the test took over %v", testTimeout)
	}
}

// webPage is a test's page and its session.
type webPage struct {
	b       *cdp
	session string
}

func (p *webPage) call(method string, params any) (json.RawMessage, error) {
	return p.b.call(p.session, method, params)
}

func (p *webPage) mouse(kind string, x, y float64, button string, buttons, clicks int) error {
	params := map[string]any{"type": kind, "x": x, "y": y, "button": button, "buttons": buttons, "clickCount": clicks}
	_, err := p.call("Input.dispatchMouseEvent", params)
	return err
}

// answer does op with the browser's trusted input and tells the page.
func (p *webPage) answer(op webOp) {
	var data any
	err := p.do(op, &data)
	r := map[string]any{"data": data}
	if err != nil {
		r["error"] = err.Error()
	}
	reply, _ := json.Marshal(r)
	p.call("Runtime.evaluate", map[string]any{"expression": fmt.Sprintf("nsgoTestReply(%d, %s)", op.ID, mustJSON(string(reply)))})
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// webTestScale is the device scale of the page tests run in.
const webTestScale = 2

func (p *webPage) do(op webOp, data *any) error {
	switch op.Op {
	case "viewport":
		_, err := p.call("Emulation.setDeviceMetricsOverride", map[string]any{
			"width": int(math.Round(op.Width)), "height": int(math.Round(op.Height)), "deviceScaleFactor": webTestScale, "mobile": false})
		return err
	case "press":
		if err := p.mouse("mouseMoved", op.X, op.Y, "none", 0, 0); err != nil {
			return err
		}
		if err := p.mouse("mousePressed", op.X, op.Y, "left", 1, 1); err != nil {
			return err
		}
		if op.Hold > 0 {
			time.Sleep(time.Duration(op.Hold+50) * time.Millisecond)
		}
		return p.mouse("mouseReleased", op.X, op.Y, "left", 0, 1)
	case "rightClick":
		if err := p.mouse("mousePressed", op.X, op.Y, "right", 2, 1); err != nil {
			return err
		}
		return p.mouse("mouseReleased", op.X, op.Y, "right", 0, 1)
	case "hover":
		return p.mouse("mouseMoved", op.X, op.Y, "none", 0, 0)
	case "drag":
		if len(op.Path) == 0 {
			return fmt.Errorf("an empty drag")
		}
		x, y := op.Path[0].X, op.Path[0].Y
		p.mouse("mouseMoved", x, y, "none", 0, 0)
		if err := p.mouse("mousePressed", x, y, "left", 1, 1); err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
		for _, to := range op.Path[1:] {
			n := int(math.Max(math.Abs(to.X-x), math.Abs(to.Y-y))/10) + 1
			for i := 1; i <= n; i++ {
				f := float64(i) / float64(n)
				p.mouse("mouseMoved", x+(to.X-x)*f, y+(to.Y-y)*f, "left", 1, 0)
				time.Sleep(16 * time.Millisecond)
			}
			x, y = to.X, to.Y
		}
		return p.mouse("mouseReleased", x, y, "left", 0, 1)
	case "scroll":
		p.mouse("mouseMoved", op.X, op.Y, "none", 0, 0)
		n := int(math.Max(math.Abs(op.DX), math.Abs(op.DY))/20) + 1
		// Deltas this small start as a touchpad's, which Chrome scrolls
		// itself, by what it is given here though it tells the page less:
		// its device pixels over the scale.
		for range n {
			if _, err := p.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseWheel", "x": op.X, "y": op.Y,
				"deltaX": op.DX / float64(n), "deltaY": op.DY / float64(n)}); err != nil {
				return err
			}
			time.Sleep(16 * time.Millisecond)
		}
		return nil
	case "type":
		for _, r := range op.Text {
			if r == '\n' {
				if err := p.key("enter"); err != nil {
					return err
				}
				continue
			}
			s := string(r)
			for _, kind := range []string{"keyDown", "keyUp"} {
				params := map[string]any{"type": kind, "key": s}
				if kind == "keyDown" {
					params["text"], params["unmodifiedText"] = s, s
				}
				if _, err := p.call("Input.dispatchKeyEvent", params); err != nil {
					return err
				}
			}
		}
		return nil
	case "compose":
		// Chrome's input method path: the page hears the composition
		// events an IME's marked text makes.
		n := len(utf16.Encode([]rune(op.Text)))
		_, err := p.call("Input.imeSetComposition", map[string]any{"text": op.Text, "selectionStart": n, "selectionEnd": n})
		return err
	case "commit":
		_, err := p.call("Input.insertText", map[string]any{"text": op.Text})
		return err
	case "key":
		return p.key(op.Key)
	case "screenshot":
		raw, err := p.call("Page.captureScreenshot", map[string]any{"format": "png"})
		if err != nil {
			return err
		}
		var shot struct{ Data string }
		json.Unmarshal(raw, &shot)
		*data = shot.Data
		return nil
	}
	return fmt.Errorf("unknown op %q", op.Op)
}

// cdpKeys are the DOM key, code and Windows key code of the keys tests
// name.
var cdpKeys = map[string]struct {
	key, code string
	vk        int
	text      string
}{
	"enter": {"Enter", "Enter", 13, "\r"}, "return": {"Enter", "Enter", 13, "\r"}, "backspace": {"Backspace", "Backspace", 8, ""},
	"escape": {"Escape", "Escape", 27, ""}, "tab": {"Tab", "Tab", 9, ""}, "space": {" ", "Space", 32, " "},
	"delete": {"Delete", "Delete", 46, ""}, "left": {"ArrowLeft", "ArrowLeft", 37, ""}, "up": {"ArrowUp", "ArrowUp", 38, ""},
	"right": {"ArrowRight", "ArrowRight", 39, ""}, "down": {"ArrowDown", "ArrowDown", 40, ""},
	"home": {"Home", "Home", 36, ""}, "end": {"End", "End", 35, ""}, "pageup": {"PageUp", "PageUp", 33, ""},
	"pagedown": {"PageDown", "PageDown", 34, ""},
}

// macCommands are the editing commands Chrome on macOS runs for keys, as
// Playwright sends them: without them its headless build edits nothing.
var macCommands = map[string]string{
	"backspace": "deleteBackward", "delete": "deleteForward", "left": "moveLeft", "right": "moveRight",
	"up": "moveUp", "down": "moveDown", "home": "moveToBeginningOfLine", "end": "moveToEndOfLine",
	"meta+a": "selectAll", "meta+c": "copy", "meta+x": "cut", "meta+v": "paste", "meta+z": "undo",
}

// key presses key ("Enter", "mod+a") with the browser's key events.
func (p *webPage) key(key string) error {
	parts := strings.Split(strings.ToLower(key), "+")
	name := parts[len(parts)-1]
	mods, prefix := 0, ""
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "mod":
			if runtime.GOOS == "darwin" {
				mods, prefix = mods|4, prefix+"meta+"
			} else {
				mods, prefix = mods|2, prefix+"ctrl+"
			}
		case "cmd", "meta", "command":
			mods, prefix = mods|4, prefix+"meta+"
		case "ctrl", "control":
			mods, prefix = mods|2, prefix+"ctrl+"
		case "alt", "option":
			mods, prefix = mods|1, prefix+"alt+"
		case "shift":
			mods, prefix = mods|8, prefix+"shift+"
		default:
			return fmt.Errorf("unknown modifier %q in %q", m, key)
		}
	}
	k, ok := cdpKeys[name]
	if !ok && len(name) == 1 {
		c := name[0]
		switch {
		case c >= 'a' && c <= 'z':
			k.key, k.code, k.vk, k.text = name, "Key"+strings.ToUpper(name), int(c-'a')+65, name
		case c >= '0' && c <= '9':
			k.key, k.code, k.vk, k.text = name, "Digit"+name, int(c), name
		default:
			k.key, k.text = name, name
		}
		ok = true
	}
	if !ok {
		return fmt.Errorf("unknown key %q", key)
	}
	down := map[string]any{"type": "rawKeyDown", "key": k.key, "code": k.code, "windowsVirtualKeyCode": k.vk, "modifiers": mods}
	if k.text != "" && mods&^8 == 0 {
		down["type"], down["text"], down["unmodifiedText"] = "keyDown", k.text, k.text
	}
	if runtime.GOOS == "darwin" {
		if c := macCommands[prefix+name]; c != "" {
			down["commands"] = []string{c}
		}
	}
	if _, err := p.call("Input.dispatchKeyEvent", down); err != nil {
		return err
	}
	_, err := p.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": k.key, "code": k.code,
		"windowsVirtualKeyCode": k.vk, "modifiers": mods})
	return err
}
