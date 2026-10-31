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
  params: { user: "Ada" },
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
// with that page's host: parameters, events, calls, posts and URLs.
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
}
