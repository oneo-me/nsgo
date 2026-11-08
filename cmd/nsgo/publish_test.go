package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/oneo-me/nsgo/tsx/pkg"
)

func TestPublishVerifyRevoke(t *testing.T) {
	dir := t.TempDir()
	at := func(name string) string { return filepath.Join(dir, name) }
	for _, k := range []string{"root", "release", "platform-root", "platform"} {
		if err := keygen(at(k)); err != nil {
			t.Fatal(err)
		}
	}
	if err := keygen(at("root")); err == nil {
		t.Fatal("keygen replaced a key")
	}
	if err := certify(certifyOptions{root: at("root.key"), key: at("release.pub"), apps: "com.example.*", days: 30}); err != nil {
		t.Fatal(err)
	}
	app := at("app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "nsgo.yaml"), []byte("name: Plugin\nid: com.example.plugin\nversion: 1.2.0\nbuild_number: 7\n"), 0o644)
	os.WriteFile(filepath.Join(app, "App.tsx"), []byte(`import { mount, Text } from "nsgo";
mount(() => <Text>plugin</Text>);`), 0o644)

	out := at("plugin.nsgopkg")
	signing := publishOptions{key: at("release.key"), cert: at("release.cert")}
	tooNew := signing
	tooNew.minRuntime, tooNew.out = "9.0.0", out
	if err := publish(tooNew, app); err == nil {
		t.Fatal("published for a newer runtime")
	}
	signing.out = out
	if err := publish(signing, app); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	info, err := pkg.Inspect(data)
	if err != nil {
		t.Fatal(err)
	}
	if info.App != (pkg.App{ID: "com.example.plugin", Name: "Plugin", Version: "1.2.0", Build: 7}) {
		t.Fatalf("got %+v", info.App)
	}
	if err := verifyPackage(verifyOptions{roots: at("root.pub")}, out); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackage(verifyOptions{}, out); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackage(verifyOptions{roots: at("platform-root.pub")}, out); !errors.Is(err, pkg.ErrUntrusted) {
		t.Fatalf("another root: got %v", err)
	}

	if err := revoke(revokeOptions{root: at("root.key"), sequence: 1, packages: out, out: at("revoked.json")}); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackage(verifyOptions{roots: at("root.pub"), revocations: at("revoked.json")}, out); !errors.Is(err, pkg.ErrRevoked) {
		t.Fatalf("a revoked package: got %v", err)
	}

	if err := certify(certifyOptions{root: at("platform-root.key"), key: at("platform.pub"), apps: "*", days: 1}); err != nil {
		t.Fatal(err)
	}
	if err := publish(publishOptions{key: at("platform.key"), cert: at("platform.cert")}, out); err == nil {
		t.Fatal("signed a package again without -out")
	}
	again := at("plugin-platform.nsgopkg")
	if err := publish(publishOptions{key: at("platform.key"), cert: at("platform.cert"), out: again}, out); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackage(verifyOptions{roots: at("platform-root.pub")}, again); err != nil {
		t.Fatal(err)
	}
}
