package main

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/tsx/pkg"
)

func TestCapabilitiesAreReadStrictly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: P\ncapabilities:\n  camera: true\n"), 0o644)
	if _, err := load(dir); err == nil || !errors.Is(err, pkg.ErrFormat) {
		t.Fatalf("an unknown capability: %v", err)
	}
}

// keygen prints a key pair and writes nothing.
func TestKeygenPrintsAKeyPair(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = keygen()
	w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	var public, secret string
	if _, err := fmt.Sscanf(string(out), "public: %s\nsecret: %s\n", &public, &secret); err != nil {
		t.Fatalf("keygen printed %q: %v", out, err)
	}
	pub, err := pkg.DecodePublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	key, err := pkg.DecodeSecretKey(secret)
	if err != nil || !key.Public().(ed25519.PublicKey).Equal(pub) {
		t.Fatalf("the secret key %q is not the public one's: %v", secret, err)
	}
	if files, _ := os.ReadDir(dir); len(files) != 0 {
		t.Errorf("keygen wrote %v", files)
	}
}

func TestPublishVerifyRevoke(t *testing.T) {
	dir := t.TempDir()
	at := func(name string) string { return filepath.Join(dir, name) }
	// Keys as nsgo keygen prints them: the secret ones in the environment,
	// the public ones in flags.
	secret, public := map[string]string{}, map[string]string{}
	for _, k := range []string{"root", "release", "platform-root", "platform"} {
		key, err := pkg.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		secret[k], public[k] = pkg.EncodeSecretKey(key), pkg.EncodePublicKey(key.Public().(ed25519.PublicKey))
	}
	t.Setenv("NSGO_ROOT_KEY", "")
	if err := certify(certifyOptions{key: public["release"], apps: "com.example.*", days: 30, out: at("release.cert")}); err == nil || !strings.Contains(err.Error(), "NSGO_ROOT_KEY") {
		t.Fatalf("certified without a root: %v", err)
	}
	t.Setenv("NSGO_ROOT_KEY", secret["root"])
	if err := certify(certifyOptions{key: at("release.pub"), apps: "com.example.*", days: 30, out: at("release.cert")}); err == nil {
		t.Fatal("certified a key file's name")
	}
	if err := certify(certifyOptions{key: public["release"], apps: "com.example.*", days: 30, out: at("release.cert")}); err != nil {
		t.Fatal(err)
	}
	app := at("app")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "nsgo.yaml"), []byte(`name: Plugin
id: com.example.plugin
version: 1.2.0
build_number: 7
capabilities:
  network:
    http:
      - host: api.example.com
        methods: [GET]
        path_prefixes: [/v1]
  open_url: true
  host: [search]
`), 0o644)
	os.WriteFile(filepath.Join(app, "App.tsx"), []byte(`import { mount, Text } from "nsgo";
mount(() => <Text>plugin</Text>);`), 0o644)

	out := at("plugin.nsgopkg")
	t.Setenv("NSGO_RELEASE_KEY", secret["release"])
	signing := publishOptions{cert: at("release.cert")}
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
	c := info.Capabilities
	if !c.Allows("network", "https://api.example.com/v1/x", "GET") || c.Allows("network", "https://api.example.com/v2", "GET") ||
		!c.Allows("openUrl", "mailto:a@b.c", "") || !c.Allows("host", "search", "") || c.Clipboard {
		t.Fatalf("capabilities %+v", c)
	}
	if err := verifyPackage(verifyOptions{roots: public["root"]}, out); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackage(verifyOptions{}, out); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackage(verifyOptions{roots: public["platform-root"]}, out); !errors.Is(err, pkg.ErrUntrusted) {
		t.Fatalf("another root: got %v", err)
	}

	if err := revoke(revokeOptions{sequence: 1, packages: out, out: at("revoked.json")}); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackage(verifyOptions{roots: public["root"], revocations: at("revoked.json")}, out); !errors.Is(err, pkg.ErrRevoked) {
		t.Fatalf("a revoked package: got %v", err)
	}

	t.Setenv("NSGO_ROOT_KEY", secret["platform-root"])
	if err := certify(certifyOptions{key: public["platform"], apps: "*", days: 1, out: at("platform.cert")}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NSGO_RELEASE_KEY", secret["platform"])
	if err := publish(publishOptions{cert: at("platform.cert")}, out); err == nil {
		t.Fatal("signed a package again without -out")
	}
	again := at("plugin-platform.nsgopkg")
	if err := publish(publishOptions{cert: at("platform.cert"), out: again}, out); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackage(verifyOptions{roots: public["platform-root"]}, again); err != nil {
		t.Fatal(err)
	}
}
