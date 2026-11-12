package tsx_test

import (
	"crypto/ed25519"
	"errors"
	"io"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/tsx"
	"github.com/nsgo-dev/nsgo/tsx/bundler"
	"github.com/nsgo-dev/nsgo/tsx/pkg"
)

// verified compiles source and signs it with caps as com.example.plugin,
// returning it verified.
func verified(t *testing.T, source string, caps pkg.Capabilities) *pkg.Package {
	t.Helper()
	code, err := bundler.Compile(fstest.MapFS{"App.tsx": {Data: []byte(source)}}, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	root, _ := pkg.GenerateKey()
	release, _ := pkg.GenerateKey()
	cert, err := pkg.Certify(root, release.Public().(ed25519.PublicKey), []string{"com.example.*"}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	data, err := pkg.Build(pkg.Contents{App: pkg.App{ID: "com.example.plugin", Build: 1}, Source: code, Capabilities: caps}, release, cert)
	if err != nil {
		t.Fatal(err)
	}
	trust := pkg.NewTrust()
	trust.AddRoot(root.Public().(ed25519.PublicKey))
	p, err := trust.Verify(data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// network answers every request with its URL, and redirects /away to
// evil.io.
func network(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/away" {
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://evil.io/"}},
			Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	}
	if req.URL.Path == "/big" {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 100))), Request: req}, nil
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(req.URL.String())), Request: req}, nil
}

// tries is a plugin that runs each named attempt and shows what each
// gave, in order, as "name: result".
const tries = `import { mount, signal, Text, Clipboard, openUrl, Storage, host } from "nsgo";
const log = signal([]);
const say = (name, v) => log.set([...log(), name + ": " + v]);
const how = (e) => e.code ? e.code + " " + (e.permission ?? "") : e.name + " " + e.message;
const attempt = async (name, fn) => { try { say(name, await fn()); } catch (e) { say(name, how(e)); } };
(async () => {
	for (const [name, fn] of Object.entries(attempts)) await attempt(name, fn);
	say("done", 1);
})();
mount(() => <Text testID="log">{() => log().join("\n")}</Text>);
`

// results runs the attempts in a View of caps and returns what each gave.
func results(t *testing.T, attempts string, caps pkg.Capabilities, s tsx.Sandbox) map[string]string {
	t.Helper()
	p := verified(t, "const attempts = {"+attempts+"};\n"+tries, caps)
	s.Limits = &tsx.Limits{Load: 10 * time.Second, Call: 2 * time.Second, Body: 50, Requests: 2, Timers: 3, Storage: 64}
	app := nsgotest.Start(t, tsx.WithFetcher(func(cx *core.Scope) core.Node {
		return tsx.View{Package: p, Sandbox: s}
	}, network))
	app.WaitUntil(10*time.Second, func() bool { return strings.Contains(app.Find("id:log").Text, "done: 1") })
	out := map[string]string{}
	for _, line := range strings.Split(app.Find("id:log").Text, "\n") {
		k, v, _ := strings.Cut(line, ": ")
		out[k] = v
	}
	return out
}

func TestSandboxHoldsAPackageToItsCapabilities(t *testing.T) {
	got := results(t, `
	allowed: async () => (await fetch("https://api.example.com/a")).text(),
	other: () => fetch("https://evil.io/"),
	redirect: () => fetch("https://api.example.com/away"),
	file: () => fetch("file:///etc/hosts"),
	big: async () => (await fetch("https://api.example.com/big")).text(),
	data: async () => (await fetch("data:,hi")).text(),
	clipboard: () => Clipboard.readText(),
	openUrl: () => openUrl("https://example.com"),
	granted: () => host.call("greet", "Ada"),
	refused: () => host.call("secret"),
	timers: () => { for (let i = 0; i < 5; i++) setTimeout(() => {}, 100000); },
	storage: async () => { await Storage.setItem("k", "v"); return (await Storage.keys()).join(","); },
	full: () => Storage.setItem("big", "x".repeat(100)),
	`, pkg.Capabilities{Network: &pkg.Network{Hosts: []string{"api.example.com"}}, Host: []string{"greet"}}, tsx.Sandbox{Host: greeter()})
	want := map[string]string{
		"allowed":   "https://api.example.com/a",
		"other":     "ERR_ACCESS_DENIED network",
		"redirect":  "ERR_ACCESS_DENIED network",
		"file":      "ERR_ACCESS_DENIED network",
		"big":       "ERR_NSH_QUOTA_EXCEEDED ",
		"data":      "hi",
		"clipboard": "ERR_ACCESS_DENIED clipboard",
		"openUrl":   "ERR_ACCESS_DENIED openUrl",
		"granted":   "hi Ada",
		"refused":   "ERR_ACCESS_DENIED host",
		"timers":    "ERR_NSH_QUOTA_EXCEEDED ",
		"storage":   "k",
		"full":      "ERR_NSH_QUOTA_EXCEEDED ",
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s gave %q, want %q", k, got[k], w)
		}
	}
}

func greeter() *core.Host {
	h := core.NewHost()
	h.Handle("greet", func(c *core.HostCall) { c.Resolve("hi " + c.Data.(string)) })
	h.Handle("secret", func(c *core.HostCall) { c.Resolve("leaked") })
	return h
}

func TestGrantNarrowsCapabilities(t *testing.T) {
	got := results(t, `
	a: async () => (await fetch("https://a.com/")).text(),
	b: () => fetch("https://b.com/"),
	`, pkg.Capabilities{Network: &pkg.Network{Hosts: []string{"a.com", "b.com"}}},
		tsx.Sandbox{Grant: &pkg.Capabilities{Network: &pkg.Network{Hosts: []string{"a.com"}}}})
	if got["a"] != "https://a.com/" || got["b"] != "ERR_ACCESS_DENIED network" {
		t.Fatalf("got %v", got)
	}
}

func init() {
	tsx.Register("sandboxtest", map[string]core.Node{"Marker": core.Text{}})
	tsx.RegisterFunctions("nsgo/base", map[string]any{"sandboxSecret": func() string { return "secret" }})
}

func TestSandboxImportsOnlyWhatItIsGiven(t *testing.T) {
	attempts := `
	base: async () => typeof (await import("nsgo/base")).Button,
	fn: async () => typeof (await import("nsgo/base")).sandboxSecret,
	app: async () => typeof (await import("sandboxtest")).Marker,
	`
	got := results(t, attempts, pkg.Capabilities{}, tsx.Sandbox{})
	if got["base"] != "function" || got["fn"] != "undefined" || !strings.Contains(got["app"], "not given to this sandbox") {
		t.Fatalf("by default: %v", got)
	}
	got = results(t, attempts, pkg.Capabilities{}, tsx.Sandbox{Modules: []string{"nsgo/base.sandboxSecret", "sandboxtest"}})
	if got["fn"] != "function" || got["app"] != "function" {
		t.Fatalf("given: %v", got)
	}
}

func TestStopsAtTheNodeQuota(t *testing.T) {
	p := verified(t, `import { mount, signal, View, Text } from "nsgo";
const n = signal(1);
mount(() => <View>
	<Text testID="more" onPress={() => n.set(100)}>more</Text>
	{() => Array.from({ length: n() }, () => <View />)}
</View>);`, pkg.Capabilities{})
	var failed error
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.Catch{Try: tsx.View{Package: p, Sandbox: tsx.Sandbox{Limits: &tsx.Limits{Nodes: 50}}}.Render,
			Fallback: func(cx *core.Scope, err error, _ func()) core.Node {
				failed = err
				return core.Text{Content: "stopped", TestID: "stopped"}
			}}
	})
	app.Press("id:more")
	app.WaitFor("id:stopped", 5*time.Second)
	if core.ErrorCode(failed) != core.CodeQuotaExceeded {
		t.Fatalf("stopped with %v", failed)
	}
}

func TestLoadCallsWhatAPluginExports(t *testing.T) {
	p := verified(t, `export function add(a, b) { return a + b; }
export async function search(q) { const r = await fetch("https://api.example.com/" + q.term); return { url: await r.text() }; }
export async function fails() { throw new Error("no luck"); }
export const notAFunction = 1;`, pkg.Capabilities{Network: &pkg.Network{Hosts: []string{"api.example.com"}}})
	var plugin *tsx.Plugin
	var loadErr error
	app := nsgotest.Start(t, tsx.WithFetcher(func(cx *core.Scope) core.Node {
		plugin, loadErr = tsx.Load(cx, p, tsx.Sandbox{})
		return core.View{}
	}, network))
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	exports := plugin.Exports()
	slices.Sort(exports)
	if !slices.Equal(exports, []string{"add", "fails", "search"}) {
		t.Fatalf("exports %v", exports)
	}
	got := map[string]any{}
	call := func(name string, args ...any) {
		app.Post(func() {
			plugin.Call(name, args, func(v any, err error) {
				if err != nil {
					got[name] = err
				} else {
					got[name] = v
				}
			})
		})
	}
	call("add", 1, 2)
	call("search", map[string]any{"term": "cats"})
	call("fails")
	call("missing")
	app.WaitUntil(5*time.Second, func() bool { return len(got) == 4 })
	if got["add"] != int64(3) && got["add"] != float64(3) {
		t.Errorf("add gave %#v", got["add"])
	}
	if m, _ := got["search"].(map[string]any); m["url"] != "https://api.example.com/cats" {
		t.Errorf("search gave %#v", got["search"])
	}
	if err, _ := got["fails"].(error); err == nil || err.Error() != "no luck" {
		t.Errorf("fails gave %v", got["fails"])
	}
	if err, _ := got["missing"].(error); !errors.Is(err, tsx.ErrNoExport) {
		t.Errorf("missing gave %v", got["missing"])
	}
	if runtime.GOOS == "js" {
		return
	}
	app.Post(plugin.Close)
	var closed error
	app.Post(func() { plugin.Call("add", []any{1, 2}, func(_ any, err error) { closed = err }) })
	app.Settle()
	if closed == nil {
		t.Fatal("a closed plugin was called")
	}
}
