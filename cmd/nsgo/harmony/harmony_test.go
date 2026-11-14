package harmony_test

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/cmd/nsgo/harmony"
)

// Package names the HAP for the app, keeps the host's entries and adds the
// web build under rawfile, the app's resources included.
func TestPackage(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "web")
	if err := os.MkdirAll(filepath.Join(site, "chunks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "chunks", "a.js"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An "asset:" image reads its bytes from here.
	if err := os.MkdirAll(filepath.Join(site, "resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "resources", "logo.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	hap, err := harmony.Package(dir, site, "Hello", "dev.oneo.nsgo.hello", "2.3.4", 7)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(hap)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	names := make([]string, len(zr.File))
	files := map[string][]byte{}
	for i, f := range zr.File {
		names[i] = f.Name
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		rc.Close()
		files[f.Name] = data
	}

	if names[0] != "module.json" {
		t.Errorf("first entry %q, want module.json", names[0])
	}
	for _, want := range []string{
		"resources/rawfile/web/index.html",
		"resources/rawfile/web/chunks/a.js",
		"resources/rawfile/web/resources/logo.png",
		"ets/modules.abc",
		"resources.index",
	} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s", want)
		}
	}
	module := string(files["module.json"])
	// The app's id is padded into an equally long bundle name so the
	// committed bytecode serves it without a rebuild.
	if !strings.Contains(module, "\"bundleName\":\"dev.oneo.nsgo.hello.nsgohost") {
		t.Errorf("module.json did not name the app: %s", module)
	}
	for _, want := range []string{
		"\"versionName\":\"2.3.4\"",
		"\"versionCode\":7",
		"\"label\":\"Hello\"",
	} {
		if !strings.Contains(module, want) {
			t.Errorf("module.json lacks %s", want)
		}
	}
	if strings.Contains(module, "{{") || strings.Contains(string(files["pack.info"]), "0.0.0") {
		t.Error("host values left unpatched")
	}
	if strings.Contains(string(files["ets/modules.abc"]), "com.nsgo.host.placeholder") {
		t.Error("the host's placeholder bundle survived in the bytecode")
	}
	if _, ok := files["build.sh"]; ok {
		t.Error("the host's build script shipped in the HAP")
	}
}

// Package serves a different app from the same committed host: its id is
// padded to the same length, so the bytecode's module names stay valid.
func TestPackageServesAnotherBundle(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "web")
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	hap, err := harmony.Package(dir, site, "Other", "com.example.other", "1.0.0", 1)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(hap)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "module.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "\"bundleName\":\"com.example.other.nsgohost") {
			t.Errorf("module.json did not name the app: %s", data)
		}
	}
}

// Package refuses an id too long to fit the bytecode's fixed-length name.
func TestPackageRefusesLongBundle(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "web")
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("a", 200)
	if _, err := harmony.Package(dir, site, "Long", long, "1.0.0", 1); err == nil {
		t.Fatal("Package accepted a bundle name too long for the host")
	}
}

// PackageWithHost keeps the app's own bundle: the host was built for it, so
// the bytecode is not touched and module.json and pack.info keep the id.
func TestPackageWithHost(t *testing.T) {
	dir := t.TempDir()
	hostDir := filepath.Join(dir, "host")
	if err := os.MkdirAll(filepath.Join(hostDir, "ets"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(hostDir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("module.json", `{"app":{"bundleName":"com.example.exact","versionName":"0.0.0","versionCode":1,"label":"$string:app_name"}}`)
	write("pack.info", `{"summary":{"app":{"bundleName":"com.example.exact","version":{"code":1,"name":"0.0.0"}}}}`)
	write("resources.index", "index")
	write(filepath.Join("ets", "modules.abc"), "abc:com.example.exact")
	site := filepath.Join(dir, "web")
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	hap, err := harmony.PackageWithHost(dir, site, hostDir, "Exact", "com.example.exact", "2.0.0", 5)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(hap)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = data
	}
	module := string(files["module.json"])
	if !strings.Contains(module, `"bundleName":"com.example.exact"`) {
		t.Errorf("module.json changed the bundle: %s", module)
	}
	if !strings.Contains(module, `"versionName":"2.0.0"`) || !strings.Contains(module, `"versionCode":5`) {
		t.Errorf("module.json lacks the app's version: %s", module)
	}
	if !strings.Contains(module, `"label":"Exact"`) {
		t.Errorf("module.json lacks the app's label: %s", module)
	}
	if got := string(files["ets/modules.abc"]); got != "abc:com.example.exact" {
		t.Errorf("bytecode changed: %q", got)
	}
	if !strings.Contains(string(files["pack.info"]), `"code":5,"name":"2.0.0"`) {
		t.Errorf("pack.info lacks the app's version: %s", files["pack.info"])
	}
}

// BundleOf returns the bundle the HAP installs as, padded without the SDK.
func TestBundleOf(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "web")
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	hap, err := harmony.Package(dir, site, "Hello", "dev.example.hello", "1.0.0", 1)
	if err != nil {
		t.Fatal(err)
	}
	want, err := harmony.BundleName("dev.example.hello")
	if err != nil {
		t.Fatal(err)
	}
	got, err := harmony.BundleOf(hap)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("BundleOf = %q, want %q", got, want)
	}
}
