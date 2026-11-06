package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProfilesSignTheirBundleIDs(t *testing.T) {
	for _, c := range []struct {
		appID, id string
		allows    bool
		signed    string
	}{
		{"ABCDE12345.com.example.app", "com.example.app", true, "ABCDE12345.com.example.app"},
		{"ABCDE12345.com.example.app", "com.example.other", false, ""},
		{"ABCDE12345.*", "com.example.app", true, "ABCDE12345.com.example.app"},
		{"ABCDE12345.com.example.*", "com.example.app", true, "ABCDE12345.com.example.app"},
		{"ABCDE12345.com.other.*", "com.example.app", false, ""},
		{"", "com.example.app", false, ""},
	} {
		if got := profileAllows(c.appID, c.id); got != c.allows {
			t.Errorf("profileAllows(%q, %q) = %v", c.appID, c.id, got)
		}
		if c.allows {
			if got := signedAppID(c.appID, c.id); got != c.signed {
				t.Errorf("signedAppID(%q, %q) = %q, want %q", c.appID, c.id, got, c.signed)
			}
		}
	}
}

func TestPropertyListsRoundTrip(t *testing.T) {
	v := map[string]any{"application-identifier": "T.a.b", "get-task-allow": true,
		"keychain-access-groups": []any{"T.*"}, "n": int64(3), "d": []byte{1, 2, 3}}
	got, err := decodePlist(encodePlist(v))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, v) {
		t.Fatalf("round trip gave %#v", got)
	}
}

// A published site names its files by their content, keeps its page and
// manifest, and its page names them.
func TestWebReleasesNameFilesByContent(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644)
	}
	write("resources/fonts/a.ttf", "font")
	p := &project{Dir: dir, Name: "Site", ID: "dev.example.site"}
	got, err := copyHashedResources(p, filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	name, ok := got["fonts/a.ttf"]
	if !ok || !hashed.MatchString("/"+name) {
		t.Fatalf("resources went to %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "out", filepath.FromSlash(name))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out", resourcesDir)); !os.IsNotExist(err) {
		t.Fatal("the site still has a resources directory")
	}
	wasm := hashedName("app.wasm", []byte("wasm"))
	if !hashed.MatchString("/"+wasm) || hashedName("app.wasm", []byte("wasm2")) == wasm {
		t.Fatalf("hashed name %q", wasm)
	}
	if hashed.MatchString("/index.html") || hashed.MatchString("/manifest.webmanifest") {
		t.Fatal("the page or manifest would be cached for good")
	}
}

func TestSimulatorsArePickedByName(t *testing.T) {
	sims, err := parseSimulators([]byte(`{"devices": {
		"com.apple.CoreSimulator.SimRuntime.iOS-17-5": [{"udid": "A", "name": "iPhone 15", "state": "Shutdown"}],
		"com.apple.CoreSimulator.SimRuntime.iOS-18-2": [{"udid": "B", "name": "iPhone 16", "state": "Shutdown"},
			{"udid": "C", "name": "iPad Air", "state": "Booted"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	for wanted, udid := range map[string]string{"": "C", "iphone 15": "A", "B": "B", "iPad Air": "C"} {
		s, err := pickSimulator(sims, wanted)
		if err != nil || s.UDID != udid {
			t.Errorf("pick %q = %v, %v; want %s", wanted, s, err, udid)
		}
	}
	if _, err := pickSimulator(sims, "Vision"); err == nil {
		t.Error("an unknown name picked a simulator")
	}
}

func TestIdentityAloneLeavesIOSUnsigned(t *testing.T) {
	p := &project{Sign: signing{Identity: "Apple Development: someone"}}
	if err := signForStore(t.Context(), p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
