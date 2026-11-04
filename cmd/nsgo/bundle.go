package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/tsx/bundler"
)

// A TSX app's bundles: the tool compiles each entry its main package
// names (compiler.App(src, "App.tsx")) ahead, and its builds add them to
// the package with a file calling compiler.Bundled, built with the
// nsgo_bundled tag, which leaves the compiler out.

const compilerPath = "github.com/oneo-me/nsgo/tsx/compiler"

// tsxEntries are the entries the main package in dir passes to
// compiler.App as string literals, relative to dir.
func tsxEntries(dir string) ([]string, error) {
	fset := token.NewFileSet()
	matches, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	seen := map[string]bool{}
	var out []string
	for _, file := range matches {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		name := ""
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == compilerPath {
				name = "compiler"
				if imp.Name != nil {
					name = imp.Name.Name
				}
			}
		}
		if name == "" || name == "_" {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "App" {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); !ok || id.Name != name {
				return true
			}
			if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if entry, err := strconv.Unquote(lit.Value); err == nil && !seen[entry] {
					seen[entry] = true
					out = append(out, entry)
				}
			}
			return true
		})
	}
	sort.Strings(out)
	return out, nil
}

// compileBundles compiles the app's TSX entries, by entry; hot ones for
// nsgo dev, which updates them in place (bundler.CompileHot).
func compileBundles(dir string, hot bool) (map[string]string, error) {
	entries, err := tsxEntries(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, e := range entries {
		compile := bundler.Compile
		if hot {
			compile = bundler.CompileHot
		}
		code, err := compile(os.DirFS(dir), e)
		if err != nil {
			return nil, fmt.Errorf("compiling %s: %w", e, err)
		}
		out[e] = code
	}
	return out, nil
}

// splitBundles are a web release's bundles (bundler.CompileSplit): each
// entry's script, and its chunks' URLs by key.
type splitBundles struct {
	main   map[string]string
	chunks map[string]map[string]string
}

// compileSplit compiles the app's TSX entries split, giving each chunk
// to write, which returns its URL relative to the page.
func compileSplit(dir string, write func(name string, data []byte) (string, error)) (*splitBundles, error) {
	entries, err := tsxEntries(dir)
	if err != nil {
		return nil, err
	}
	s := &splitBundles{main: map[string]string{}, chunks: map[string]map[string]string{}}
	for _, e := range entries {
		main, chunks, err := bundler.CompileSplit(os.DirFS(dir), e)
		if err != nil {
			return nil, fmt.Errorf("compiling %s: %w", e, err)
		}
		s.main[e] = main
		urls := map[string]string{}
		for key, code := range chunks {
			name := strings.TrimSuffix(strings.TrimPrefix(key, "app:"), filepath.Ext(key))
			if urls[key], err = write("chunks/"+strings.ReplaceAll(name, "/", "-")+".js", []byte(code)); err != nil {
				return nil, err
			}
		}
		s.chunks[e] = urls
	}
	return s, nil
}

// bundleFile compiles the app's TSX and writes the file that gives its
// bundles to compiler.App, returning its path; "" for an app without
// TSX. A web release's are split.
func (p *project) bundleFile() (string, error) {
	var bundles map[string]string
	var chunks map[string]map[string]string
	var err error
	if p.split != nil {
		bundles, chunks = p.split.main, p.split.chunks
	} else {
		bundles, err = compileBundles(p.Dir, p.Dev.Client && !p.Release)
	}
	if err != nil || len(bundles) == 0 {
		return "", err
	}
	entries := make([]string, 0, len(bundles))
	for e := range bundles {
		entries = append(entries, e)
	}
	sort.Strings(entries)
	var b strings.Builder
	b.WriteString("// Written by nsgo: the app's TSX, compiled ahead.\n\npackage main\n\n")
	fmt.Fprintf(&b, "import %q\n\nfunc init() {\n", compilerPath)
	for _, e := range entries {
		fmt.Fprintf(&b, "\tcompiler.Bundled(%q, %s)\n", e, strconv.Quote(bundles[e]))
		if urls := chunks[e]; len(urls) > 0 {
			fmt.Fprintf(&b, "\tcompiler.BundledChunks(%q, map[string]string{\n", e)
			for _, k := range slices.Sorted(maps.Keys(urls)) {
				fmt.Fprintf(&b, "\t\t%q: %q,\n", k, urls[k])
			}
			b.WriteString("\t})\n")
		}
	}
	b.WriteString("}\n")
	return p.cacheFile("bundle", ".go", b.String())
}

// cacheFile writes content to .nsgo under a name of its own content, so
// builds of several targets at once share it.
func (p *project) cacheFile(prefix, ext, content string) (string, error) {
	sum := sha256.Sum256([]byte(content))
	path := p.cache(prefix + "-" + hex.EncodeToString(sum[:8]) + ext)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), prefix+"-*.tmp")
	if err != nil {
		return "", err
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()
	return path, os.Rename(tmp.Name(), path)
}

// overlay writes go's -overlay file adding files, by name, to the main
// package. go names the package's files by its directory as $PWD spells
// it or by its real path (macOS's /tmp is /private/tmp), so the overlay
// names both.
func (p *project) overlay(files map[string]string) (string, error) {
	abs, err := filepath.Abs(p.Dir)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	replace := map[string]string{}
	for name, src := range files {
		replace[filepath.Join(abs, name)] = src
		replace[filepath.Join(real, name)] = src
	}
	b, _ := json.Marshal(map[string]any{"Replace": replace})
	return p.cacheFile("overlay", ".json", string(b))
}

// steps are nsgo test <entry>'s: the entry, relative to the package, and
// the steps standard input held.
type steps struct{ entry, src string }

// stepsTest is the Go test running p.steps.
const stepsTest = "TestSteps"

// tsxTestFile writes the Go test that runs the app's *.test.tsx files
// (tsxtest.Run), or the steps of nsgo test <entry>, returning its path;
// "" when it has none. Headless, the test reads the app's files from its
// directory; built for a target, which runs it where they are not, it
// carries them.
func (p *project) tsxTestFile() (string, error) {
	var found bool
	files := map[string][]byte{}
	err := filepath.WalkDir(p.Dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != p.Dir && (strings.HasPrefix(name, ".") || name == "node_modules" || name == p.Build.Output) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".test.tsx") || strings.HasSuffix(name, ".test.ts") {
			found = true
		}
		if p.testBinary && !strings.HasSuffix(name, ".go") && tsxFile(name) {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			r, _ := filepath.Rel(p.Dir, path)
			files[filepath.ToSlash(r)] = b
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if !found && p.steps == nil {
		return "", nil
	}
	name, err := packageName(p.Dir)
	if err != nil {
		return "", err
	}
	fsys, imports := `os.DirFS(".")`, `"os"`
	if p.testBinary {
		var b strings.Builder
		b.WriteString("fstest.MapFS{\n")
		keys := slices.Sorted(maps.Keys(files))
		for _, k := range keys {
			fmt.Fprintf(&b, "\t%q: {Data: []byte(%q)},\n", k, files[k])
		}
		b.WriteString("}")
		fsys, imports = b.String(), `"testing/fstest"`
	}
	test := "func TestTSX(t *testing.T) { tsxtest.Run(t, appFiles) }"
	if s := p.steps; s != nil {
		test = fmt.Sprintf("func %s(t *testing.T) { tsxtest.RunSteps(t, appFiles, %q, %q) }", stepsTest, s.entry, s.src)
	}
	return p.cacheFile("tsxtest", "_test.go", fmt.Sprintf(`// Written by nsgo: the app's TSX tests.

package %s

import (
	%s
	"testing"

	"github.com/oneo-me/nsgo/tsx/tsxtest"
)

// appFiles are the files the tests compile the app from.
var appFiles = %s

%s
`, name, imports, fsys, test))
}

// tsxFile reports whether a TSX app compiles from a file of this name:
// its modules, data, configuration and assets.
func tsxFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".tsx", ".ts", ".jsx", ".js", ".mjs", ".json", ".yaml", ".yml",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ttf", ".otf", ".woff", ".woff2", ".ttc":
		return true
	}
	return false
}

// packageName is the name of the package in dir.
func packageName(dir string) (string, error) {
	fset := token.NewFileSet()
	matches, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for _, file := range matches {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, parser.PackageClauseOnly)
		if err != nil {
			return "", err
		}
		return f.Name.Name, nil
	}
	return "", fmt.Errorf("%s has no Go package", dir)
}
