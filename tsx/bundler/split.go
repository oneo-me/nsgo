package bundler

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// Split bundles (NSH's native-pipeline.md, "Split bundles"): CompileSplit
// makes each app module an import() names a chunk of its own, which the
// runtime loads when the import first runs, as the web fetches it. A
// chunk holds its module and what it imports that the entry's script
// does not; modules either holds are defined once in the VM, by their
// keys (app:<path>), so a chunk shares the entry's state.
//
// goja's parser reads no import(), so the split transform writes each as
// _$load(spec, key), the same length: `import("./Stats")` is
// `_$load("./Stats", "app:Stats.tsx")`.

// CompileSplit compiles the app whose entry is entry in fsys as Compile
// does, but for its import()s of its own modules: main is the entry's
// script, and chunks the scripts the runtime loads for them, by key.
func CompileSplit(fsys fs.FS, entry string) (main string, chunks map[string]string, err error) {
	entry = path.Clean(entry)
	s := &splitter{fsys: fsys}
	main, inMain, err := s.build(entry, nil)
	if err != nil {
		return "", nil, err
	}
	setup, err := setupScript(fsys, entry)
	if err != nil {
		return "", nil, err
	}
	main = splitRuntime + setup + main
	chunks = map[string]string{}
	for len(s.roots) > 0 {
		root := s.roots[0]
		s.roots = s.roots[1:]
		if _, done := chunks[moduleKey(root)]; done || inMain[root] {
			continue
		}
		code, _, err := s.build(root, inMain)
		if err != nil {
			return "", nil, err
		}
		chunks[moduleKey(root)] = code
	}
	return main, chunks, nil
}

// moduleKey is a module's key in a bundle: esbuild's name for it.
func moduleKey(file string) string { return "app:" + file }

// splitter is one split compile: the chunks' roots found so far.
type splitter struct {
	fsys  fs.FS
	roots []string
}

const splitEntry = "nsgo:split-entry"

// build bundles the modules root imports, root among them, but those of
// shared, which it requires from the VM by key: the entry's script when
// shared is nil, a chunk's otherwise. It returns the script and the
// modules it holds.
func (s *splitter) build(root string, shared map[string]bool) (string, map[string]bool, error) {
	entryCode := `require("./` + root + `");`
	plugins := []api.Plugin{{Name: "nsgo-split", Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: "^" + regexp.QuoteMeta(splitEntry) + "$"},
			func(api.OnResolveArgs) (api.OnResolveResult, error) {
				return api.OnResolveResult{Path: splitEntry, Namespace: "nsgo-split"}, nil
			})
		b.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "nsgo-split"}, func(api.OnLoadArgs) (api.OnLoadResult, error) {
			return api.OnLoadResult{Contents: &entryCode, Loader: api.LoaderJS}, nil
		})
		if shared != nil {
			// What the entry's script holds is the VM's already.
			b.OnResolve(api.OnResolveOptions{Filter: `^\.\.?(/|$)`, Namespace: "app"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
				if p, ok := resolve(s.fsys, path.Join(path.Dir(args.Importer), args.Path)); ok && shared[p] {
					return api.OnResolveResult{Path: sharedPrefix + moduleKey(p), External: true}, nil
				}
				return api.OnResolveResult{}, nil
			})
		}
	}}, appPlugin(s.fsys, false, s)}
	result := api.Build(api.BuildOptions{
		EntryPoints: []string{splitEntry},
		Bundle:      true,
		Write:       false,
		Format:      api.FormatIIFE,
		Platform:    api.PlatformNeutral,
		Target:      api.ES2020,
		Supported:   gojaLacks,
		Sourcemap:   api.SourceMapInline,
		Outfile:     strings.TrimSuffix(path.Base(root), path.Ext(root)) + ".js",
		Charset:     api.CharsetUTF8,
		LogLevel:    api.LogLevelSilent,
		Metafile:    true,
		Plugins:     plugins,
	})
	if len(result.Errors) > 0 {
		return "", nil, messages(result.Errors)
	}
	held := map[string]bool{}
	for _, m := range regexp.MustCompile(`"app:([^"]+)"`).FindAllStringSubmatch(result.Metafile, -1) {
		held[m[1]] = true
	}
	for _, f := range result.OutputFiles {
		if !strings.HasSuffix(f.Path, ".js") {
			continue
		}
		loc := commonJS.FindStringIndex(string(f.Contents))
		if loc == nil {
			return "", nil, errors.New("compiler: the bundle has no module wrapper to split")
		}
		code := string(f.Contents)
		lines := strings.Count(code[loc[0]:loc[1]], "\n")
		code = code[:loc[0]] + "var __commonJS = (cb) => __nsgo_split.define(cb);" + strings.Repeat("\n", lines) + code[loc[1]:]
		code = strings.ReplaceAll(code, `__require("`+sharedPrefix, `__nsgo_split.require("`)
		return strings.ReplaceAll(code, `require("`+sharedPrefix, `__nsgo_split.require("`), held, nil
	}
	return "", nil, errors.New("compiler: esbuild wrote no script")
}

// sharedPrefix marks a chunk's import of a module the VM holds.
const sharedPrefix = "nsgo-shared:"

// dynamicImport finds a module's import() calls after esbuild's
// transform, and the string literal it names, if any.
var dynamicImport = regexp.MustCompile(`\bimport\(\s*("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')?`)

// splitImports writes the import()s of code, file's module after
// esbuild's transform, as _$load calls, in place, and returns what to
// insert into them: the key of the app module each names, which it
// records as a chunk's root.
func (s *splitter) splitImports(file, code string) (string, []insertion, error) {
	var ins []insertion
	var errs []error
	out := []byte(code)
	for _, m := range dynamicImport.FindAllStringSubmatchIndex(code, -1) {
		copy(out[m[0]:], "_$load(")
		if m[2] < 0 {
			continue
		}
		spec, err := strconv.Unquote(strings.ReplaceAll(code[m[2]:m[3]], "'", `"`))
		if err != nil || !(strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../")) {
			continue
		}
		p, ok := resolve(s.fsys, path.Join(path.Dir(file), spec))
		if !ok {
			errs = append(errs, fmt.Errorf("%s: cannot find %q", file, spec))
			continue
		}
		if !slices.Contains(s.roots, p) {
			s.roots = append(s.roots, p)
		}
		ins = append(ins, insertion{m[3], ", " + strconv.Quote(moduleKey(p))})
	}
	if len(errs) > 0 {
		return "", nil, joinErrors(errs)
	}
	return string(out), ins, nil
}

// splitRuntime is what defines a split app's modules and loads its
// chunks, one line before the entry's script so its source map holds.
var splitRuntime = strings.ReplaceAll(`(function (g) {
if (g.__nsgo_split) return;
var factories = new Map(), modules = new Map(), chunks = new Map();
var split = g.__nsgo_split = {};
split.define = function (cb) {
  var key = Object.getOwnPropertyNames(cb)[0];
  if (!factories.has(key)) factories.set(key, cb[key]);
  return function () { return split.require(key); };
};
split.require = function (key) {
  var m = modules.get(key);
  if (m) return m.exports;
  m = { exports: {} };
  modules.set(key, m);
  try { factories.get(key)(m.exports, m); } catch (e) { modules.delete(key); throw e; }
  return m.exports;
};
g._$load = function (spec, key) {
  if (key === undefined) return Promise.resolve().then(function () { return require(spec); });
  if (factories.has(key)) return Promise.resolve().then(function () { return split.require(key); });
  var p = chunks.get(key);
  if (!p) {
    p = require("nsgo:rt").loadChunk(key);
    chunks.set(key, p);
    p.catch(function () { chunks.delete(key); });
  }
  return p.then(function () { return split.require(key); });
};
})(globalThis);`, "\n", " ")
