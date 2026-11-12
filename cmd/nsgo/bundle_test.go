package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTSXAppsAreCompiledAhead(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", `package main

import (
	"embed"

	"github.com/nsgo-dev/nsgo"
	tsxc "github.com/nsgo-dev/nsgo/tsx/compiler"
)

//go:embed *.tsx
var src embed.FS

func main() { nsgo.Run(tsxc.App(src, "App.tsx")) }
`)
	write("main_test.go", `package main

var _ = tsxc.App(src, "Test.tsx")
`)
	write("App.tsx", `import { mount, Text } from "nsgo";
mount(() => <Text>Hi</Text>);
`)
	entries, err := tsxEntries(dir)
	if err != nil || !slices.Equal(entries, []string{"App.tsx"}) {
		t.Fatalf("entries %q, %v", entries, err)
	}
	p := &project{Dir: dir}
	file, err := p.bundleFile()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(file)
	if s := string(b); !strings.Contains(s, `compiler.Bundled("App.tsx", `) || !strings.Contains(s, "Hi") {
		t.Errorf("the bundle file:\n%s", s)
	}
	ov, err := p.overlay(map[string]string{"zz_nsgo_bundle.go": file})
	if err != nil {
		t.Fatal(err)
	}
	o, _ := os.ReadFile(ov)
	if !strings.Contains(string(o), filepath.Join(dir, "zz_nsgo_bundle.go")) {
		t.Errorf("the overlay %s", o)
	}
	none := &project{Dir: t.TempDir()}
	if f, err := none.bundleFile(); f != "" || err != nil {
		t.Errorf("an app without TSX has a bundle %q, %v", f, err)
	}
}
