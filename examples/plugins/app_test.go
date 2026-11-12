package main

import (
	"bytes"
	"crypto/ed25519"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

//go:embed plugin/App.tsx
var pluginSrc embed.FS

const wait = 10 * time.Second

func TestInstallsAndRunsThePackages(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.WaitFor("id:install", wait)
	if texts := strings.Join(app.Texts(), "\n"); !strings.Contains(texts, "It may call the app's greet") {
		t.Fatalf("the consent says %q", texts)
	}
	app.Press("id:install")
	app.WaitUntil(wait, func() bool { return app.Exists("id:greeting") && app.Find("id:greeting").Text == "Hello from the host" })
	app.Press("id:wave")
	app.Press("id:wave")
	if got := app.Find("id:waves").Text; got != "Waved 2 times" {
		t.Fatalf("the package shows %q\n%s", got, app.Dump())
	}
	// The waves are in the plugin's part of the storage, not the app's.
	if keys := app.Storage().Keys(); len(keys) != 0 {
		t.Fatalf("the app's storage lists %v", keys)
	}
	app.WaitUntil(wait, func() bool { return app.Find("id:search").Text == "Found [signed sandboxed]" })
}

func TestInstallsWhole(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("the web has no files: it installs in memory")
	}
	dir := t.TempDir()
	p, err := pkg.Inspect(greeter)
	if err != nil {
		t.Fatal(err)
	}
	if err := install(dir, &pkg.Package{Info: *p}, greeter); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "dev.oneo.nsgo.plugins.greeter.nsgopkg")); err != nil || !bytes.Equal(b, greeter) {
		t.Fatalf("installed %d bytes, %v", len(b), err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dev.oneo.nsgo.plugins.greeter.nsgopkg.tmp")); err == nil {
		t.Fatal("the temporary file stayed")
	}
}

// refusal starts a host of data and returns why it refused it.
func refusal(t *testing.T, data []byte, trust *pkg.Trust) string {
	t.Helper()
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node { return Host(cx, data, trust, "") })
	app.WaitUntil(wait, func() bool { return strings.HasPrefix(app.Find("id:status").Text, "Refused") })
	return app.Find("id:status").Text
}

func TestRefusesATamperedPackage(t *testing.T) {
	data := bytes.Clone(greeter)
	data[len(data)-100] ^= 1
	trust := pkg.NewTrust()
	root, _ := pkg.DecodePublicKey(rootKey)
	trust.AddRoot(root)
	if got := refusal(t, data, trust); !strings.Contains(got, "signature") {
		t.Fatal(got)
	}
}

// signer signs builds of the plugin with a root of a test's own.
type signer struct {
	root, release ed25519.PrivateKey
	cert          *pkg.Certificate
	source        string
}

func newSigner(t *testing.T) *signer {
	t.Helper()
	src, _ := fs.Sub(pluginSrc, "plugin")
	code, err := bundler.Compile(src, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	s := &signer{source: code}
	s.root, _ = pkg.GenerateKey()
	s.release, _ = pkg.GenerateKey()
	s.cert, err = pkg.Certify(s.root, s.release.Public().(ed25519.PublicKey), []string{"dev.oneo.nsgo.plugins.*"}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (s *signer) build(t *testing.T, build uint64) []byte {
	t.Helper()
	data, err := pkg.Build(pkg.Contents{App: pkg.App{ID: "dev.oneo.nsgo.plugins.greeter", Build: build}, Source: s.source}, s.release, s.cert)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (s *signer) trust() *pkg.Trust {
	t := pkg.NewTrust()
	t.AddRoot(s.root.Public().(ed25519.PublicKey))
	return t
}

func TestRefusesARevokedPackage(t *testing.T) {
	s := newSigner(t)
	data := s.build(t, 1)
	trust := s.trust()
	p, err := trust.Verify(data)
	if err != nil {
		t.Fatal(err)
	}
	list, err := pkg.Revoke(s.root, 1, []string{p.KeyID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := trust.UpdateRevocations(list); err != nil {
		t.Fatal(err)
	}
	if got := refusal(t, data, trust); !strings.Contains(got, "revoked") {
		t.Fatal(got)
	}
}

func TestRefusesAnOlderBuild(t *testing.T) {
	s := newSigner(t)
	// The web has no files to keep a Trust's state in.
	trust := pkg.NewTrust()
	if runtime.GOOS != "js" {
		var err error
		if trust, err = pkg.OpenTrust(t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	trust.AddRoot(s.root.Public().(ed25519.PublicKey))
	newer, older := s.build(t, 2), s.build(t, 1)
	// One app, as phones and the web show one: a press swaps the newer
	// build for the older.
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		swapped := core.NewSignal(cx, false)
		return core.View{Class: "flex-1", Children: []core.Node{
			core.View{TestID: "older", Class: "h-10", OnPress: func(core.PressEvent) { swapped.Set(true) }},
			core.Dynamic(func(cx *core.Scope) core.Node {
				if swapped.Get() {
					return Host(cx, older, trust, "")
				}
				return Host(cx, newer, trust, "")
			}),
		}}
	})
	app.WaitFor("id:install", wait)
	app.Press("id:install")
	app.WaitFor("id:waves", wait)
	app.WaitUntil(wait, func() bool { b, _ := trust.AcceptedBuild("dev.oneo.nsgo.plugins.greeter"); return b == 2 })
	app.Press("id:older")
	app.WaitUntil(wait, func() bool { return strings.HasPrefix(app.Find("id:status").Text, "Refused") })
	if got := app.Find("id:status").Text; !strings.Contains(got, "older") {
		t.Fatal(got)
	}
}

// TestWebViewStaysOnItsHosts has a package's web view, which may show
// example.com alone, try to leave its page: the platform refuses it as it
// starts.
func TestWebViewStaysOnItsHosts(t *testing.T) {
	switch nsgotest.Target() {
	case "":
		t.Skip("headless, no page navigates")
	case "web":
		t.Skip("an <iframe>'s navigations are not checked on the web (roadmap)")
	}
	s := newSigner(t)
	code, err := bundler.Compile(fstest.MapFS{"App.tsx": {Data: []byte(`import { mount, signal, Text, View, WebView } from "nsgo";
const refused = signal("");
const page = "<p>away</p><script>setTimeout(() => { location.href = 'https://evil.invalid/'; }, 300);</script>";
mount(() => <View style={{ flex: 1 }}>
	<Text testID="refused">{refused}</Text>
	<WebView style={{ height: 200 }} html={page} baseUrl="https://example.com/" onError={(e) => refused.set(e.message)} />
</View>);`)}}, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	data, err := pkg.Build(pkg.Contents{App: pkg.App{ID: "dev.oneo.nsgo.plugins.web", Build: 1}, Source: code,
		Capabilities: pkg.Capabilities{WebView: &pkg.WebView{Hosts: []string{"example.com"}}}}, s.release, s.cert)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.trust().Verify(data)
	if err != nil {
		t.Fatal(err)
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node { return tsx.View{Package: p} })
	app.WaitUntil(20*time.Second, func() bool { return strings.Contains(app.Find("id:refused").Text, "evil.invalid") })
}
