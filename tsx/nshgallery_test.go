package tsx_test

import (
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
)

// nshImports are the imports of NSH's modules, renamed to NSGo's.
var nshImports = regexp.MustCompile(`(from |import\()"nsh(/[^"]*)?"`)

// TestNSHGallery runs NSH's gallery (nsh/examples/gallery, with the
// styled UI it shares) with its imports renamed, opening every demo
// headless: none fails. It skips without nsh/ checked out.
func TestNSHGallery(t *testing.T) {
	examples := filepath.Join("..", "nsh", "examples")
	if _, err := os.Stat(filepath.Join(examples, "gallery", "App.tsx")); err != nil {
		t.Skip("nsh/ is not checked out")
	}
	fsys := fstest.MapFS{"app/nsgo.yaml": {Data: []byte("name: Gallery\ntheme: ../nsh-gallery-ui/theme.json\n")}}
	for _, dir := range []string{"gallery", "nsh-gallery-ui"} {
		root := filepath.Join(examples, dir)
		name := map[string]string{"gallery": "app", "nsh-gallery-ui": "nsh-gallery-ui"}[dir]
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				if d != nil && d.IsDir() && (d.Name() == ".nsh" || d.Name() == "tests" || d.Name() == "tools") {
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
