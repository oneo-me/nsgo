package tsx_test

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/tsx"
	"github.com/oneo-me/nsgo/tsx/bundler"
	"github.com/oneo-me/nsgo/tsx/pkg"
)

// signedPackage compiles source and signs it as build of com.example.plugin,
// returning it and a Trust of its root.
func signedPackage(t *testing.T, source string, build uint64) ([]byte, *pkg.Trust) {
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
	data, err := pkg.Build(pkg.Contents{App: pkg.App{ID: "com.example.plugin", Build: build}, Source: code}, release, cert)
	if err != nil {
		t.Fatal(err)
	}
	trust := pkg.NewTrust()
	trust.AddRoot(root.Public().(ed25519.PublicKey))
	return data, trust
}

func TestViewRunsAVerifiedPackage(t *testing.T) {
	data, trust := signedPackage(t, `import { mount, signal, Text } from "nsgo";
const n = signal(0);
let made = "no";
try { made = String(new Function("return 1")()); } catch (e) { made = "refused"; }
mount(() => <Text testID="count" onPress={() => n.set(n() + 1)}>{made + " " + n()}</Text>);`, 1)
	p, err := trust.Verify(data)
	if err != nil {
		t.Fatal(err)
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{core.Text{Content: "host", TestID: "host"}, tsx.View{Package: p}}}
	})
	if got := app.Find("id:count").Text; got != "refused 0" {
		t.Fatalf("the package shows %q\n%s", got, app.Dump())
	}
	app.Press("id:count")
	if got := app.Find("id:count").Text; got != "refused 1" {
		t.Fatalf("after a press it shows %q", got)
	}
	if app.Find("id:host").Text != "host" {
		t.Fatal("the host's own view is gone")
	}
	b := []byte(strings.Clone(string(data)))
	b[len(b)-100] ^= 1
	if _, err := trust.Verify(b); !errors.Is(err, pkg.ErrSignature) {
		t.Fatalf("a changed package: got %v", err)
	}
}

func TestViewThrowsUnderCatch(t *testing.T) {
	data, trust := signedPackage(t, `throw new Error("broken plugin");`, 1)
	p, err := trust.Verify(data)
	if err != nil {
		t.Fatal(err)
	}
	var failed error
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.Catch{Try: tsx.View{Package: p}.Render, Fallback: func(cx *core.Scope, err error, _ func()) core.Node {
			failed = err
			return core.Text{Content: "failed", TestID: "failed"}
		}}
	})
	if app.Find("id:failed").Text != "failed" || failed == nil || !strings.Contains(failed.Error(), "broken plugin") {
		t.Fatalf("got %v\n%s", failed, app.Dump())
	}
}
