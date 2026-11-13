package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// hostPage embeds examples/embedded through the loader, playing its host.
const hostPage = `<!doctype html>
<html><body style="margin:0">
<div id="app" style="width:400px;height:500px"></div>
<script type="module">
const config = await (await fetch("nsgo.json")).json();
const { mount } = await import("./" + config.loader);
window.posts = [];
window.player = await mount(document.getElementById("app"), "nsgo.json", {
  config,
  module: WebAssembly.compileStreaming(fetch(config.wasm)),
  params: { user: "Ada" },
  appearance: "dark",
  host: {
    call(name) {
      if (name === "time") return Promise.resolve("12:00");
      throw new Error("no host function " + name);
    },
    post(name, args) { window.posts.push([name, args]); },
    openURL(url) { return url.startsWith("https://example.com/"); },
  },
});
</script>
</body></html>
`

// TestWebHostChannel runs examples/embedded in a headless browser, in its
// own page with nsgo.yaml's host module and in another page's element
// with that page's host: parameters, events, calls, posts, URLs, the
// page's settings and a module the page compiled.
func TestWebHostChannel(t *testing.T) {
	if testing.Short() {
		t.Skip("builds an app for the web")
	}
	if _, err := findBrowser(); err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	p, err := load("../../examples/embedded")
	if err != nil {
		t.Fatal(err)
	}
	p.Release = true
	dir := t.TempDir()
	if err := buildWeb(ctx, p, dir, false); err != nil {
		t.Fatal(err)
	}
	var app webApp
	data, _ := os.ReadFile(filepath.Join(dir, "nsgo.json"))
	if err := json.Unmarshal(data, &app); err != nil || !hashed.MatchString("/"+app.Loader) || !hashed.MatchString("/"+app.Host) {
		t.Fatalf("nsgo.json = %s", data)
	}
	os.WriteFile(filepath.Join(dir, "host.html"), []byte(hostPage), 0o644)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.FileServer(http.Dir(dir))}
	go srv.Serve(ln)
	defer srv.Close()
	b, err := startBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()

	open := func(page string) *webPage {
		raw, err := b.call("", "Target.createTarget", map[string]any{"url": fmt.Sprintf("http://%s/%s", ln.Addr(), page)})
		if err != nil {
			t.Fatal(err)
		}
		var target struct{ TargetID string }
		json.Unmarshal(raw, &target)
		raw, err = b.call("", "Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true})
		if err != nil {
			t.Fatal(err)
		}
		var attached struct{ SessionID string }
		json.Unmarshal(raw, &attached)
		return &webPage{b: b, session: attached.SessionID}
	}
	eval := func(pg *webPage, expr string) any {
		raw, err := pg.call("Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true, "awaitPromise": true})
		if err != nil {
			t.Fatal(err)
		}
		var res struct {
			Result struct{ Value any }
		}
		json.Unmarshal(raw, &res)
		return res.Result.Value
	}
	waitText := func(pg *webPage, text string) {
		t.Helper()
		for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
			if eval(pg, fmt.Sprintf("document.body.innerText.includes(%q)", text)) == true {
				return
			}
		}
		t.Fatalf("the page never showed %q; it shows %q", text, eval(pg, "document.body.innerText"))
	}
	press := func(pg *webPage, id string) {
		t.Helper()
		at, _ := eval(pg, fmt.Sprintf(`(() => { const r = document.querySelector('[data-testid=%q]').getBoundingClientRect();
			return [r.x + r.width / 2, r.y + r.height / 2]; })()`, id)).([]any)
		if len(at) != 2 {
			t.Fatalf("no %s to press", id)
		}
		x, y := at[0].(float64), at[1].(float64)
		for _, kind := range []string{"mousePressed", "mouseReleased"} {
			if err := pg.mouse(kind, x, y, "left", 1, 1); err != nil {
				t.Fatal(err)
			}
		}
	}

	own := open("index.html")
	waitText(own, "1 messages · Hello from the page")
	press(own, "ask")
	waitText(own, "The host says")
	press(own, "open")
	waitText(own, "opened: true")

	pg := open("host.html")
	waitText(pg, "Hello, Ada")
	eval(pg, `player.hostEvent("message", { text: "Order filled" })`)
	waitText(pg, "1 messages · Order filled")
	press(pg, "ask")
	waitText(pg, "The host says 12:00")
	press(pg, "tell")
	for deadline := time.Now().Add(10 * time.Second); eval(pg, "JSON.stringify(posts)") != `[["tapped",{"count":1}]]`; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("posts = %v", eval(pg, "JSON.stringify(posts)"))
		}
	}
	if eval(pg, `(() => { try { player.hostEvent("x", { n: 1n }); return false } catch { return true } })()`) != true {
		t.Fatal("a payload that is not JSON did not throw")
	}
	scheme := `document.getElementById("app").style.colorScheme`
	if got := eval(pg, scheme); got != "dark" {
		t.Fatalf("the element's color scheme = %v, want the host's dark", got)
	}
	eval(pg, `player.setAppearance("light")`)
	if got := eval(pg, scheme); got != "light" {
		t.Fatalf("after setAppearance, the element's color scheme = %v, want light", got)
	}
}

// strictPolicy is a hardened page's: no 'unsafe-eval', and none of the
// page's own scripts inline. The style the app sets on elements is inline.
const strictPolicy = "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; " +
	"style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'"

// TestWebStrictCSP runs examples/tsx in a headless browser under
// strictPolicy: the app runs from the files buildWeb wrote, the realm's
// scripts and its bundle among them, so nothing is compiled from a string
// (docs/architecture/tsx.md).
func TestWebStrictCSP(t *testing.T) {
	if testing.Short() {
		t.Skip("builds an app for the web")
	}
	if _, err := findBrowser(); err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	p, err := load("../../examples/tsx")
	if err != nil {
		t.Fatal(err)
	}
	p.Release = true
	dir := t.TempDir()
	if err := buildWeb(ctx, p, dir, false); err != nil {
		t.Fatal(err)
	}
	var app webApp
	data, err := os.ReadFile(filepath.Join(dir, "nsgo.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &app); err != nil || app.Runtime == "" || len(app.Scripts) == 0 {
		t.Fatalf("nsgo.json = %s", data)
	}
	files := http.FileServer(http.Dir(dir))
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", strictPolicy)
		files.ServeHTTP(w, r)
	})}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go srv.Serve(ln)
	b, err := startBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	raw, err := b.call("", "Target.createTarget", map[string]any{"url": fmt.Sprintf("http://%s/index.html", ln.Addr())})
	if err != nil {
		t.Fatal(err)
	}
	var target struct{ TargetID string }
	json.Unmarshal(raw, &target)
	raw, err = b.call("", "Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true})
	if err != nil {
		t.Fatal(err)
	}
	var attached struct{ SessionID string }
	json.Unmarshal(raw, &attached)
	pg := &webPage{b: b, session: attached.SessionID}
	eval := func(expr string) any {
		t.Helper()
		raw, err := pg.call("Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true, "awaitPromise": true})
		if err != nil {
			t.Fatal(err)
		}
		var res struct{ Result struct{ Value any } }
		json.Unmarshal(raw, &res)
		return res.Result.Value
	}
	// The app's heading, and the card of the chunk its import() names.
	for _, text := range []string{"Today", "A 25 minute session"} {
		for deadline := time.Now().Add(60 * time.Second); ; time.Sleep(100 * time.Millisecond) {
			if eval(fmt.Sprintf("document.body.innerText.includes(%q)", text)) == true {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the page never showed %q; it shows %q", text, eval("document.body.innerText"))
			}
		}
	}
	// The policy is the page's: a script the page makes inline does not run,
	// as does nothing the runtime would compile from a string.
	inline := `(() => { const s = document.createElement("script"); s.textContent = "globalThis.__inline = 1"; document.body.append(s); return globalThis.__inline === 1 })()`
	if eval(inline) != false {
		t.Fatalf("the page ran an inline script under %s", strictPolicy)
	}
	// The realm is the loader's, and it holds what the page shipped: the
	// runtime ran those rather than their sources.
	programs := `Array.from(Object.keys(document.querySelector("iframe").contentWindow.__nsgoPrograms ?? {})).sort().join()`
	if got := eval(programs); got != "harden.js,helpers,merge.js,split.js,time.js,web.js" {
		t.Fatalf("the realm's programs = %v", got)
	}
	// The app's bundle, and the chunk its import() names, each a file the
	// loader added to the realm: the chunk's script element ran it.
	scripts := `Array.from(Object.keys(document.querySelector("iframe").contentWindow.__nsgoScripts ?? {})).sort().join()`
	if got := eval(scripts); got != "App.tsx,focus.tsx" {
		t.Fatalf("the realm's scripts = %v", got)
	}
}
