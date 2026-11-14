package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type settings struct {
	Theme    string `json:"theme"`
	FontSize int    `json:"fontSize"`
}

func TestPersistedStoresAndFollowsItsEntry(t *testing.T) {
	cx := NewRoot(RootOptions{})
	def := settings{Theme: "light", FontSize: 13}
	p := NewPersisted(cx, "settings", def, PersistOptions{})
	var seen []settings
	cx.renderEffect(func() { seen = append(seen, p.Get()) })
	if p.Get() != def || p.Err() != nil {
		t.Fatalf("value = %+v %v", p.Get(), p.Err())
	}

	if err := p.Update(func(s settings) settings { s.Theme = "dark"; return s }); err != nil {
		t.Fatal(err)
	}
	if got := p.Get(); got.Theme != "dark" || got.FontSize != 13 {
		t.Fatalf("value = %+v", got)
	}
	raw, _ := cx.Storage().GetItem("settings")
	if raw != `{"version":0,"value":{"theme":"dark","fontSize":13}}` {
		t.Fatalf("stored %s", raw)
	}

	// Another writer of the entry reaches it, as another graph's change does.
	cx.Storage().SetItem("settings", `{"version":0,"value":{"theme":"sepia","fontSize":15}}`)
	if got := p.Get(); got.Theme != "sepia" || len(seen) != 3 {
		t.Fatalf("value = %+v, seen = %v", got, seen)
	}
	p.Reset()
	if p.Get() != def {
		t.Fatalf("a reset value = %+v", p.Get())
	}
}

func TestPersistedReadsTheDefaultForWhatItCannotRead(t *testing.T) {
	cx := NewRoot(RootOptions{})
	p := NewPersisted(cx, "n", 7, PersistOptions{Version: 2})
	for _, raw := range []string{`not json`, `{"version":2,"value":"seven"}`, `{"version":3,"value":1}`, `{"version":1,"value":1}`} {
		cx.Storage().SetItem("n", raw)
		if p.Get() != 7 || p.Err() == nil {
			t.Errorf("%s read %d %v, want the default and an error", raw, p.Get(), p.Err())
		}
	}
	// Setting a value clears the error.
	if err := p.Set(9); err != nil || p.Get() != 9 || p.Err() != nil {
		t.Fatalf("value = %d %v %v", p.Get(), p.Err(), err)
	}
}

func TestPersistedMigratesOlderVersions(t *testing.T) {
	cx := NewRoot(RootOptions{})
	// Version 1 kept the font size as a string.
	cx.Storage().SetItem("settings", `{"version":1,"value":{"theme":"dark","fontSize":"16"}}`)
	var versions []int
	migrate := func(version int, data json.RawMessage) (json.RawMessage, error) {
		versions = append(versions, version)
		var old struct {
			Theme    string `json:"theme"`
			FontSize string `json:"fontSize"`
		}
		if err := json.Unmarshal(data, &old); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"theme": old.Theme, "fontSize": len(old.FontSize) * 8})
	}
	p := NewPersisted(cx, "settings", settings{}, PersistOptions{Version: 2, Migrate: migrate})
	if got := p.Get(); got.Theme != "dark" || got.FontSize != 16 || p.Err() != nil {
		t.Fatalf("value = %+v %v", got, p.Err())
	}
	// The value decodes once per change, however often it is read.
	p.Get()
	p.Get()
	if len(versions) != 1 || versions[0] != 1 {
		t.Fatalf("migrated %v", versions)
	}

	failing := NewPersisted(cx, "settings", settings{Theme: "light"}, PersistOptions{Version: 2,
		Migrate: func(int, json.RawMessage) (json.RawMessage, error) { return nil, errors.New("unknown form") }})
	if failing.Get().Theme != "light" || failing.Err() == nil || !strings.Contains(failing.Err().Error(), "unknown form") {
		t.Fatalf("a failed migration read %+v %v", failing.Get(), failing.Err())
	}
}

func TestPersistedSetReportsAFullStorage(t *testing.T) {
	cx := NewRoot(RootOptions{})
	part := cx.Child()
	part.ProvideStorage("small", 64)
	p := NewPersisted(part, "big", "", PersistOptions{})
	var full *StorageFullError
	if err := p.Set(strings.Repeat("x", 100)); !errors.As(err, &full) {
		t.Fatalf("err = %v", err)
	}
	if p.Get() != "" {
		t.Fatalf("value = %q", p.Get())
	}
	if err := p.Set("ok"); err != nil || p.Get() != "ok" {
		t.Fatalf("value = %q %v", p.Get(), err)
	}
	// The partition holds it, apart from the app's own storage.
	if keys := cx.Storage().Keys(); len(keys) != 0 {
		t.Fatalf("the app's storage holds %v", keys)
	}
	if _, ok := cx.Storage().GetItem("big"); ok {
		t.Fatal("the app's storage reads the partition's entry")
	}
	bad := NewPersisted[any](cx, "bad", nil, PersistOptions{})
	if err := bad.Set(make(chan int)); err == nil {
		t.Fatal("an unencodable value stored")
	}
}
