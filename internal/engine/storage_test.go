package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestAppDirs(t *testing.T) {
	env := map[string]string{"HOME": "/h", "APPDATA": `C:\r`, "LOCALAPPDATA": `C:\l`}
	get := func(k string) string { return env[k] }
	for _, c := range []struct{ goos, id, data, cache string }{
		{"darwin", "dev.app", "/h/Library/Application Support/dev.app", "/h/Library/Caches/dev.app"},
		{"ios", "", "/h/Library/Application Support/default", "/h/Library/Caches/default"},
		{"linux", "../x", "/h/.local/share/default", "/h/.cache/default"},
		{"windows", "dev.app", filepath.Join(`C:\r`, "dev.app"), filepath.Join(`C:\l`, "dev.app")},
		{"android", "dev.app", "", ""},
		{"js", "dev.app", "", ""},
	} {
		data, cache := appDirs(c.goos, c.id, get)
		if data != filepath.FromSlash(c.data) && data != c.data || cache != filepath.FromSlash(c.cache) && cache != c.cache {
			t.Errorf("%s %q: %q, %q; want %q, %q", c.goos, c.id, data, cache, c.data, c.cache)
		}
	}
	if data, _ := appDirs("linux", "a", func(string) string { return "" }); data != "" {
		t.Errorf("without a home: %q", data)
	}
}

// graphOn returns storage for a graph over store.
func graphOn(store *core.Store) core.Storage {
	return core.NewRoot(core.RootOptions{Post: func(func()) {}, Storage: store}).Storage()
}

func TestFileStorage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "app")
	store := OpenFileStorage(dir)
	if OpenFileStorage(dir) != store {
		t.Fatal("a second open made another store")
	}
	s := graphOn(store)
	s.SetItem("a", "1")
	s.SetItem("b", "2")
	s.RemoveItem("a")
	SyncStorage()
	data, err := os.ReadFile(filepath.Join(dir, "storage.json"))
	if err != nil || string(data) != `{"b":"2"}` {
		t.Fatalf("storage.json = %s, %v", data, err)
	}

	// A file that does not parse is set aside, not overwritten.
	other := filepath.Join(t.TempDir(), "app")
	os.MkdirAll(other, 0o755)
	os.WriteFile(filepath.Join(other, "storage.json"), []byte("{oops"), 0o644)
	s = graphOn(OpenFileStorage(other))
	if keys := s.Keys(); len(keys) != 0 {
		t.Fatalf("keys = %v", keys)
	}
	s.SetItem("c", "3")
	SyncStorage()
	if data, _ := os.ReadFile(filepath.Join(other, "storage.json.corrupt")); string(data) != "{oops" {
		t.Fatalf("set aside: %s", data)
	}
	if data, _ := os.ReadFile(filepath.Join(other, "storage.json")); string(data) != `{"c":"3"}` {
		t.Fatalf("storage.json = %s", data)
	}
}

func TestEngineMakesAppDirs(t *testing.T) {
	root := t.TempDir()
	opts := Options{Size: driver.Size{Width: 100, Height: 100}}
	opts.SetDirs(filepath.Join(root, "data"), filepath.Join(root, "cache"))
	var data string
	r := &recorder{mounted: make(chan struct{}, 8)}
	e := Start(r, func(cx *core.Scope) core.Node {
		data = cx.DataDir()
		return core.View{}
	}, opts)
	<-r.mounted
	e.Close()
	if data != opts.DataDir {
		t.Fatalf("DataDir = %q", data)
	}
	for _, d := range []string{opts.DataDir, opts.CacheDir} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Fatalf("%s: %v", d, err)
		}
	}
}
