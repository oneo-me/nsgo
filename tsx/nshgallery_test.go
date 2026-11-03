package tsx_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/tsx/compiler"
	_ "github.com/oneo-me/nsgo/tsx/diff"
	_ "github.com/oneo-me/nsgo/tsx/editor"
	_ "github.com/oneo-me/nsgo/tsx/textview"
	"github.com/oneo-me/nsgo/tsx/tsxtest"
)

// nshImports are the imports of NSH's modules, renamed to NSGo's.
var nshImports = regexp.MustCompile(`(from |import\(|nativeModule[^(\n]*\()"nsh(/[^"]*)?"`)

// nshGallery is NSH's gallery (nsh/examples/gallery, with the styled UI
// it shares) with its imports renamed, at app/ and nsh-gallery-ui/, and
// its tests when tests is set. It skips the test without nsh/ checked
// out.
func nshGallery(t *testing.T, tests bool) fstest.MapFS {
	examples := filepath.Join("..", "nsh", "examples")
	if _, err := os.Stat(filepath.Join(examples, "gallery", "App.tsx")); err != nil {
		t.Skip("nsh/ is not checked out")
	}
	fsys := fstest.MapFS{
		"app/nsgo.yaml":                  {Data: []byte("name: Gallery\ntheme: ../nsh-gallery-ui/theme.json\n")},
		"nsh-gallery-ui/tests/nsgo.yaml": {Data: []byte("name: Gallery UI\ntheme: ../theme.json\n")},
	}
	for _, dir := range []string{"gallery", "nsh-gallery-ui"} {
		root := filepath.Join(examples, dir)
		name := map[string]string{"gallery": "app", "nsh-gallery-ui": "nsh-gallery-ui"}[dir]
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				if d != nil && d.IsDir() && (d.Name() == ".nsh" || d.Name() == "tools" || d.Name() == "tests" && !tests) {
					return filepath.SkipDir
				}
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			if strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".tsx") {
				data = nshImports.ReplaceAll(data, []byte(`${1}"nsgo${2}"`))
			}
			fsys[name+"/"+filepath.ToSlash(rel)] = &fstest.MapFile{Data: data}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return fsys
}

// TestNSHGallery runs NSH's gallery with its imports renamed, opening
// every demo headless: none fails.
func TestNSHGallery(t *testing.T) {
	fsys := nshGallery(t, false)
	// The demos, but AI Chat and Standard APIs, which run processes.
	for _, d := range []string{"Stocks", "Photos", "Lists", "Scrolling", "Collapsing header", "Navigation", "Form",
		"Resizable", "Dock", "Gestures", "Physics", "Animations", "Canvas", "Charts", "Styles", "Tailwind", "Text",
		"Text View", "Editor", "DataTable", "Diff", "Images", "Components", "Overlays", "Menus", "Controls",
		"Web and video", "System", "Storage"} {
		t.Run(d, func(t *testing.T) {
			app := nsgotest.Start(t, compiler.App(fsys, "app/App.tsx"))
			app.Press("text:" + d + " >> nth=0")
			app.Settle()
			if !app.Exists("text:Back") {
				t.Errorf("%s did not open: %q", d, app.Texts())
			}
		})
	}
}

// nshSkipped are the tests of NSH's gallery NSGo does not run, by file:
// what they say, which a skip stands in front of. The AI Chat demo spawns
// its model through the process module, one of the standard modules NSGo
// leaves out (docs/architecture/nsh.md), and the error boundary's shows
// QuickJS's wording of a TypeError, where goja words it otherwise.
var nshSkipped = map[string][]string{
	"app/tests/picker.test.tsx":         {`test("`},
	"app/tests/text-view.test.tsx":      {`test("a chat reply`},
	"app/tests/error-boundary.test.tsx": {`test("a failing binding`},
}

// TestNSHGalleryTests runs NSH's gallery's own tests (nsh/test, as
// nsgo/test) on it, headless.
func TestNSHGalleryTests(t *testing.T) {
	if testing.Short() {
		t.Skip("a minute of NSH's tests")
	}
	fsys := nshGallery(t, true)
	for file, starts := range nshSkipped {
		f := fsys[file]
		if f == nil {
			t.Fatalf("%s is gone from NSH's gallery", file)
		}
		for _, s := range starts {
			if !bytes.Contains(f.Data, []byte(s)) {
				t.Fatalf("%s has no %s", file, s)
			}
			f.Data = bytes.ReplaceAll(f.Data, []byte(s), []byte(strings.Replace(s, "test(", "test.skip(", 1)))
		}
	}
	tsxtest.Run(t, fsys)
}

// nshExample is NSH's example dir (nsh/examples/<dir>) with its imports
// renamed and its nsh.json's name and web host in an nsgo.yaml, as a
// port of it has; the module names its texts give are renamed too
// ("nsh/host:").
func nshExample(t *testing.T, dir string) fstest.MapFS {
	return nshApp(t, filepath.Join("..", "nsh", "examples", dir))
}

// nshApp is the NSH app at root, as nshExample has it.
func nshApp(t *testing.T, root string) fstest.MapFS {
	if _, err := os.Stat(filepath.Join(root, "App.tsx")); err != nil {
		t.Skip("nsh/ is not checked out")
	}
	fsys := fstest.MapFS{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		switch {
		case rel == "nsh.json":
			var config struct {
				Name      string
				Web       struct{ Host string }
				StatusBar map[string]any
				Theme     map[string]any
				Tailwind  map[string]any
			}
			if err := json.Unmarshal(data, &config); err != nil {
				return err
			}
			yaml := "name: " + config.Name + "\n"
			if config.Web.Host != "" {
				yaml += "web:\n  host: " + config.Web.Host + "\n"
			}
			if config.StatusBar != nil {
				sb, _ := json.Marshal(config.StatusBar) // JSON is YAML
				yaml += "status_bar: " + string(sb) + "\n"
			}
			if config.Tailwind != nil {
				tw, _ := json.Marshal(config.Tailwind)
				yaml += "tailwind: " + string(tw) + "\n"
			}
			if config.Theme != nil {
				theme, _ := json.Marshal(config.Theme)
				fsys["theme.yaml"] = &fstest.MapFile{Data: theme}
				yaml += "theme: theme.yaml\n"
			}
			fsys["nsgo.yaml"] = &fstest.MapFile{Data: []byte(yaml)}
		case strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".tsx"):
			data = nshImports.ReplaceAll(data, []byte(`${1}"nsgo${2}"`))
			data = bytes.ReplaceAll(data, []byte("nsh/host:"), []byte("nsgo/host:"))
		}
		fsys[filepath.ToSlash(rel)] = &fstest.MapFile{Data: data}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fsys
}

// TestNSHExamples opens NSH's examples headless and runs their tests:
// none fails.
func TestNSHExamples(t *testing.T) {
	for _, dir := range []string{"counter", "hopper", "desktop", "embedded", "host-echo"} {
		t.Run(dir, func(t *testing.T) {
			fsys := nshExample(t, dir)
			app := nsgotest.Start(t, compiler.App(fsys, "App.tsx"))
			if len(app.Texts()) == 0 {
				t.Error("it shows nothing")
			}
			if _, err := fs.Stat(fsys, "App.test.tsx"); err == nil {
				tsxtest.Run(t, fsys)
			}
		})
	}
}
