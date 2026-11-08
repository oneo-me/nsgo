package nsgo

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// layers lists the module packages a package may import
// (docs/architecture/layers.md), both as paths inside the module, "" for
// its root. A pattern ending in "/" matches a package and the packages
// below it; the first rule whose package matches applies.
var layers = []struct {
	pkg     string
	imports []string
}{
	{"internal/layout", nil},
	{"microphone", nil},
	{"internal/woff", nil},
	{"internal/cldr/", nil},
	{"internal/cldr", nil},
	{"internal/driver", []string{"internal/shaderir"}},
	{"internal/shaderir/", []string{"internal/shaderir"}},
	{"internal/shader", []string{"internal/shaderir"}},
	{"internal/shader/wgsl", []string{"internal/shaderir"}},
	{"core", []string{"internal/driver", "internal/layout", "internal/shader", "internal/shader/wgsl", "internal/shaderir"}},
	{"internal/engine", []string{"core", "internal/driver", "internal/testhost"}},
	{"internal/testhost", []string{"core", "internal/driver"}},
	{"internal/platform/android/runtime", nil},
	{"internal/platform/apple", append([]string{"internal/driver"}, platformShared...)},
	{"internal/platform/canvastest", []string{"internal/driver"}},
	{"internal/platform/field", []string{"internal/driver"}},
	{"internal/platform/finger", []string{"internal/driver"}},
	{"internal/platform/fontfile", nil},
	{"internal/platform/hover", []string{"internal/driver"}},
	{"internal/platform/imagedecode", []string{"internal/driver"}},
	{"internal/platform/imageload", []string{"internal/driver"}},
	{"internal/platform/menutext", []string{"internal/driver", "desktop/locales"}},
	{"internal/platform/perfhud", []string{"internal/driver", "internal/engine", "internal/platform/apple"}},
	{"internal/platform/picked", []string{"internal/driver"}},
	{"internal/platform/queue", []string{"internal/driver"}},
	{"internal/platform/scrollbar", []string{"internal/driver"}},
	{"internal/platform/textselect", []string{"internal/driver"}},
	{"internal/platform/", append([]string{"internal/driver", "internal/engine", "internal/testhost", "core", "desktop/locales"}, platformShared...)},
	{"nsgotest", []string{"core", "internal/driver", "internal/testhost"}},
	{"", []string{"core", "internal/engine", "internal/platform/"}},
	// The code nsgo writes from catalogs runs on i18n.
	{"i18n", []string{"core", "internal/cldr"}},
	{"base/", []string{"core", "base/", "i18n"}},
	{"mobile", []string{"core"}},
	{"desktop/locales", []string{"core", "i18n"}},
	{"desktop", []string{"core"}},
	{"icons/", []string{"core"}},
	{"ui/", []string{"core", "base/", "ui/", "mobile", "desktop", "icons/"}},
	// The TSX test runner drives apps through nsgotest.
	{"tsx/tsxtest", []string{"core", "internal/driver", "nsgotest", "tsx", "tsx/"}},
	// The nsgo tool links the bundler, so it reaches no runtime.
	{"tsx/bundler", []string{"core", "internal/driver", "internal/woff"}},
	// The JS engines' API reaches goja or the page, nothing of NSGo's.
	{"tsx/internal/js", nil},
	// nsgo/ui gives TSX ui's components, for the apps that import it.
	{"tsx/ui", []string{"core", "base/", "ui/", "tsx", "tsx/"}},
	{"tsx/ui/", []string{"core", "base/", "ui/", "tsx", "tsx/"}},
	{"tsx/", []string{"core", "base/", "mobile", "desktop", "tsx", "tsx/bundler", "tsx/internal/js"}},
	// Apps use the public packages only, and the hosts that embed an
	// example app its package.
	{"examples/", []string{"", "core", "base/", "ui/", "mobile", "desktop", "desktop/locales", "i18n", "icons/", "nsgotest", "tsx/", "examples/"}},
	{"bench/", []string{"", "core", "base/", "mobile", "desktop", "icons/"}},
	// The nsgo tool links no component or runtime, so changing them does
	// not build it again; it writes TSX declarations by running the app.
	{"cmd/", []string{"core", "cmd/", "internal/platform/android/runtime", "tsx/bundler"}},
	{"tools/", []string{"core"}},
}

// platformShared is the code several backends share, with the shader
// writers but not the compiler, which only the core links.
var platformShared = []string{
	"internal/platform/apple", "internal/platform/canvastest", "internal/platform/field", "internal/platform/finger",
	"internal/platform/fontfile", "internal/platform/hover", "internal/platform/imagedecode", "internal/platform/imageload", "internal/platform/menutext", "internal/platform/perfhud", "internal/platform/picked", "internal/platform/queue", "internal/platform/scrollbar",
	"internal/platform/textselect", "internal/shaderir/",
}

// portable lists the packages that build unchanged for every target: no
// purego, syscall or unsafe.
var portable = []string{"core", "internal/driver", "internal/layout", "internal/shader", "internal/shader/wgsl", "internal/shaderir/", "internal/engine",
	"internal/testhost", "internal/cldr", "base/", "ui/", "i18n", "mobile", "desktop", "desktop/locales", "icons/", "nsgotest", "tsx"}

// unportable are the packages among portable's that reach the system: the
// terminal loads libghostty-vt and runs programs, on macOS, Linux and
// Windows, and builds elsewhere to report that it cannot.
var unportable = []string{"base/terminal/"}

// layerTargets are the GOOS/GOARCH pairs the nsgo tool builds for.
var layerTargets = [][2]string{
	{"darwin", "arm64"},
	{"ios", "arm64"},
	{"windows", "amd64"},
	{"windows", "arm64"},
	{"linux", "amd64"},
	{"android", "arm64"},
	{"js", "wasm"},
}

const modulePath = "github.com/oneo-me/nsgo"

func TestLayering(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("needs the go command")
	}
	for _, target := range layerTargets {
		t.Run(target[0]+"_"+target[1], func(t *testing.T) {
			t.Parallel()
			for _, p := range listPackages(t, target[0], target[1]) {
				checkLayer(t, p)
			}
		})
	}
}

type listedPackage struct {
	ImportPath string
	Imports    []string
	CgoFiles   []string
	Error      *struct{ Err string }
}

func listPackages(t *testing.T, goos, goarch string) []listedPackage {
	t.Helper()
	cmd := exec.Command("go", "list", "-e", "-json=ImportPath,Imports,CgoFiles,Error", "./...")
	// CGO_ENABLED=1 keeps cgo files in the listing, so a C import shows.
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=1", "GOFLAGS=")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, stderr.Bytes())
	}
	var pkgs []listedPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p listedPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if p.Error != nil {
			if !strings.Contains(p.Error.Err, "build constraints exclude all Go files") {
				t.Errorf("%s: %s", p.ImportPath, p.Error.Err)
			}
			continue
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

func checkLayer(t *testing.T, p listedPackage) {
	t.Helper()
	pkg, ok := inModule(p.ImportPath)
	if !ok {
		return
	}
	if len(p.CgoFiles) > 0 {
		t.Errorf("%s: cgo files %v; reach system APIs through purego or syscall", p.ImportPath, p.CgoFiles)
	}
	allowed, ruled := layerImports(pkg)
	isPortable := matchesAny(pkg, portable) && !matchesAny(pkg, unportable)
	for _, imp := range p.Imports {
		if dep, ok := inModule(imp); ok {
			if ruled && !matchesAny(dep, allowed) {
				t.Errorf("%s imports %s across layers", p.ImportPath, imp)
			}
		} else if isPortable && platformImport(imp) {
			t.Errorf("%s imports %s; it must build for every target unchanged", p.ImportPath, imp)
		}
	}
}

// inModule returns path inside the module, "" for its root.
func inModule(importPath string) (string, bool) {
	if importPath == modulePath {
		return "", true
	}
	rest, ok := strings.CutPrefix(importPath, modulePath+"/")
	return rest, ok
}

func layerImports(pkg string) ([]string, bool) {
	for _, l := range layers {
		if matches(pkg, l.pkg) {
			return l.imports, true
		}
	}
	return nil, false
}

func matches(pkg, pattern string) bool {
	if dir, ok := strings.CutSuffix(pattern, "/"); ok {
		return pkg == dir || strings.HasPrefix(pkg, pattern)
	}
	return pkg == pattern
}

func matchesAny(pkg string, patterns []string) bool {
	for _, pattern := range patterns {
		if matches(pkg, pattern) {
			return true
		}
	}
	return false
}

func platformImport(imp string) bool {
	switch imp {
	case "C", "unsafe", "syscall", "syscall/js":
		return true
	}
	return imp == "github.com/ebitengine/purego" || strings.HasPrefix(imp, "github.com/ebitengine/purego/")
}
