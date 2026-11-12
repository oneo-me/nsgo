// Package bundler compiles TSX apps into the scripts the tsx package
// runs, as NSH's bundler does (nsh-bundler): esbuild strips TypeScript,
// lowers syntax goja lacks and bundles the app's modules into one script,
// and jsx.go compiles the JSX into calls of the renderer, as NSH's
// compiler does. Imports of NSGo's modules (nsgo, nsgo/*, nsgo:rt) stay,
// as require calls the runtime answers. No Node is needed.
//
// The nsgo tool links it, so it imports from NSGo only core and
// internal/driver, never the runtime (layering_test.go).
package bundler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/go-sourcemap/sourcemap"
	"go.yaml.in/yaml/v3"

	"github.com/nsgo-dev/nsgo/core"
)

// gojaLacks are the features goja does not run, which esbuild lowers.
var gojaLacks = map[string]bool{"async-generator": false, "for-await": false, "dynamic-import": false}

// Compile compiles the app whose entry is entry in fsys, and the modules
// it imports, into one script with its source map inline.
func Compile(fsys fs.FS, entry string) (string, error) { return build(fsys, entry, false) }

// CompileHot compiles the app as Compile does, for hot updates: a newer
// bundle run in the VM of an older one (tsx.Hot) runs again only the
// modules whose code changed and renders their components' instances
// again, so the rest of the app keeps its state.
func CompileHot(fsys fs.FS, entry string) (string, error) { return build(fsys, entry, true) }

func build(fsys fs.FS, entry string, hot bool) (string, error) {
	entry = path.Clean(entry)
	plugins := []api.Plugin{appPlugin(fsys, hot, nil)}
	entryPoint := entry
	if hot {
		// The entry is a module too, run again only when it changes.
		entryPoint = hotEntry
		plugins = append([]api.Plugin{hotPlugin(entry)}, plugins...)
	}
	// The entry's exports, a plugin's functions (tsx.Load), are a global
	// of the script's.
	globalName := "__nsgo_exports"
	if hot {
		globalName = ""
	}
	result := api.Build(api.BuildOptions{
		EntryPoints: []string{entryPoint},
		Bundle:      true,
		Write:       false,
		Format:      api.FormatIIFE,
		GlobalName:  globalName,
		Platform:    api.PlatformNeutral,
		Target:      api.ES2020,
		Supported:   gojaLacks,
		Sourcemap:   api.SourceMapInline,
		Outfile:     "bundle.js",
		Charset:     api.CharsetUTF8,
		LogLevel:    api.LogLevelSilent,
		Plugins:     plugins,
	})
	if len(result.Errors) > 0 {
		return "", messages(result.Errors)
	}
	setup, err := setupScript(fsys, entry)
	if err != nil {
		return "", err
	}
	for _, f := range result.OutputFiles {
		if strings.HasSuffix(f.Path, ".js") {
			code := string(f.Contents)
			if hot {
				if code, err = hotModules(code); err != nil {
					return "", err
				}
				setup = hotRuntime + setup
			}
			return setup + code, nil
		}
	}
	return "", errors.New("compiler: esbuild wrote no script")
}

// setupScript is what runs before the app: the theme file, the Tailwind
// theme and the default status bar nsgo.yaml beside the entry names,
// given to the runtime ("" without any). It is one line, so the source map's lines stay
// the app's.
func setupScript(fsys fs.FS, entry string) (string, error) {
	dir := path.Dir(entry)
	data, err := fs.ReadFile(fsys, path.Join(dir, "nsgo.yaml"))
	if err != nil {
		return "", nil
	}
	var config struct {
		Theme     string    `yaml:"theme"`
		Tailwind  yaml.Node `yaml:"tailwind"`
		StatusBar yaml.Node `yaml:"status_bar"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("nsgo.yaml: %w", err)
	}
	var opts []string
	if config.Theme != "" {
		theme, err := fs.ReadFile(fsys, path.Join(dir, config.Theme))
		if err != nil {
			return "", fmt.Errorf("nsgo.yaml: theme: %w", err)
		}
		if _, err := core.ParseTheme(theme); err != nil {
			return "", fmt.Errorf("nsgo.yaml: theme: %w", err)
		}
		opts = append(opts, "theme: "+strconv.Quote(string(theme)))
	}
	if !config.Tailwind.IsZero() {
		tw, err := yaml.Marshal(&config.Tailwind)
		if err != nil {
			return "", fmt.Errorf("nsgo.yaml: tailwind: %w", err)
		}
		// Checked here, so a mistake fails the build.
		if _, err := core.ParseTailwind(tw); err != nil {
			return "", fmt.Errorf("nsgo.yaml: %w", err)
		}
		opts = append(opts, "tailwind: "+strconv.Quote(string(tw)))
	}
	if !config.StatusBar.IsZero() {
		var sb struct {
			Foreground string `yaml:"foreground" json:"foreground,omitempty"`
			Hidden     *bool  `yaml:"hidden" json:"hidden,omitempty"`
		}
		if err := config.StatusBar.Decode(&sb); err != nil {
			return "", fmt.Errorf("nsgo.yaml: status_bar: %w", err)
		}
		switch sb.Foreground {
		case "", "auto", "light", "dark":
		default:
			return "", fmt.Errorf("nsgo.yaml: status_bar: foreground is auto, light or dark, not %q", sb.Foreground)
		}
		js, _ := json.Marshal(sb)
		opts = append(opts, "statusBar: "+string(js))
	}
	if len(opts) == 0 {
		return "", nil
	}
	return fmt.Sprintf("require(\"nsgo:rt\").setup({ %s });", strings.Join(opts, ", ")), nil
}

// isNative reports whether an import names one of NSGo's modules, which
// the runtime serves.
func isNative(p string) bool {
	return p == "nsgo" || strings.HasPrefix(p, "nsgo/") || strings.HasPrefix(p, "nsgo:")
}

// sources are the extensions a module's import may leave out, in order.
var sources = []string{".tsx", ".ts", ".jsx", ".js", ".mjs"}

// appPlugin resolves and loads the app's modules from fsys, compiling
// each one's JSX.
func appPlugin(fsys fs.FS, hot bool, split *splitter) api.Plugin {
	return api.Plugin{Name: "nsgo", Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: ".*"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			if isNative(args.Path) {
				return api.OnResolveResult{Path: args.Path, External: true}, nil
			}
			var p string
			switch {
			case args.Kind == api.ResolveEntryPoint:
				p = path.Clean(args.Path)
			case args.Path == "." || args.Path == ".." || strings.HasPrefix(args.Path, "./") || strings.HasPrefix(args.Path, "../"):
				// A directory ("..") imports its index.
				p = path.Join(path.Dir(args.Importer), args.Path)
			case !strings.HasPrefix(args.Path, "/"):
				// A bare specifier names a module the runtime serves:
				// NSGo's, or one an app registered (tsx.Register).
				return api.OnResolveResult{Path: args.Path, External: true}, nil
			default:
				return api.OnResolveResult{}, fmt.Errorf("cannot import %q: an app imports its own files by relative paths", args.Path)
			}
			if resolved, ok := resolve(fsys, p); ok {
				return api.OnResolveResult{Path: resolved, Namespace: "app"}, nil
			}
			return api.OnResolveResult{}, fmt.Errorf("cannot find %q", args.Path)
		})
		b.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "app"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			data, err := fs.ReadFile(fsys, args.Path)
			if err != nil {
				return api.OnLoadResult{}, err
			}
			src := string(data)
			if isAsset(args.Path) {
				code, err := assetModule(fsys, args.Path, data)
				if err != nil {
					return api.OnLoadResult{}, err
				}
				return api.OnLoadResult{Contents: &code, Loader: api.LoaderJS}, nil
			}
			if strings.HasSuffix(args.Path, ".json") {
				return api.OnLoadResult{Contents: &src, Loader: api.LoaderJSON}, nil
			}
			code, err := transform(args.Path, src, hot, split)
			if err != nil {
				var list []api.Message
				for _, e := range unwrap(err) {
					var ce *Error
					if errors.As(e, &ce) {
						list = append(list, api.Message{Text: ce.Message,
							Location: &api.Location{File: ce.File, Line: ce.Line, Column: ce.Column - 1}})
					} else {
						list = append(list, api.Message{Text: e.Error()})
					}
				}
				return api.OnLoadResult{Errors: list}, nil
			}
			return api.OnLoadResult{Contents: &code, Loader: api.LoaderJS}, nil
		})
	}}
}

// resolve finds the module p names: the file, or it with an extension,
// or its index.
func resolve(fsys fs.FS, p string) (string, bool) {
	var candidates []string
	if fi, err := fs.Stat(fsys, p); err == nil && !fi.IsDir() {
		return p, true
	}
	for _, ext := range append(sources, ".json") {
		candidates = append(candidates, p+ext)
	}
	for _, ext := range sources {
		candidates = append(candidates, path.Join(p, "index"+ext))
	}
	for _, c := range candidates {
		if fi, err := fs.Stat(fsys, c); err == nil && !fi.IsDir() {
			return c, true
		}
	}
	return "", false
}

// Transform compiles one module: its TypeScript stripped and its JSX
// compiled, as CommonJS with its source map inline.
func Transform(file, src string) (string, error) { return transform(file, src, false, nil) }

// transform is Transform, with the hot transform (hot.go) when hot, and
// the split one (split.go) when split is set.
func transform(file, src string, hot bool, split *splitter) (string, error) {
	supported := gojaLacks
	if split != nil {
		// import() stays, for splitImports.
		supported = map[string]bool{"async-generator": false, "for-await": false}
	}
	loader := api.LoaderTS
	switch path.Ext(file) {
	case ".tsx":
		loader = api.LoaderTSX
	case ".jsx":
		loader = api.LoaderJSX
	case ".js", ".mjs":
		loader = api.LoaderJS
	}
	r := api.Transform(src, api.TransformOptions{
		Loader:      loader,
		JSX:         api.JSXTransform,
		JSXFactory:  jsxFactory,
		JSXFragment: jsxFragment,
		Format:      api.FormatCommonJS,
		Target:      api.ES2020,
		Supported:   supported,
		Sourcemap:   api.SourceMapExternal,
		Sourcefile:  file,
		Charset:     api.CharsetUTF8,
		LogLevel:    api.LogLevelSilent,
		// A module's URL is its file's in the app, as NSH's compiler
		// resolves it.
		Define: map[string]string{"import.meta.url": strconv.Quote("file:///" + strings.TrimPrefix(file, "/"))},
	})
	if len(r.Errors) > 0 {
		return "", messages(r.Errors)
	}
	code := string(r.Code)
	var loads []insertion
	if split != nil {
		var err error
		if code, loads, err = split.splitImports(file, code); err != nil {
			return "", err
		}
	}
	if !hot && split == nil && !strings.Contains(code, jsxFactory) {
		return inline(code, string(r.Map)), nil
	}
	prev, err := sourcemap.Parse(file, r.Map)
	if err != nil {
		return "", fmt.Errorf("%s: %w", file, err)
	}
	pos := newPositions(code)
	original := newPositions(src)
	at := func(off int) (int, int) {
		l, c := pos.at(off)
		if _, _, ol, oc, ok := prev.Source(l+1, c); ok {
			// What it points at, past the spaces before it.
			l, c := original.at(skipSpace(src, original.offset(ol-1, oc)))
			return l + 1, c + 1
		}
		return l + 1, c + 1
	}
	f, err := lower(file, code, at, ambient(src))
	if err != nil {
		return "", err
	}
	if hot {
		ins, err := hotInsertions(file, code)
		if err != nil {
			return "", err
		}
		f = f.insert(ins)
	}
	f = f.insert(loads)
	out, m := f.mapped(pos, prev, file, src)
	return inline(out, m), nil
}

// skipSpace returns the offset of the first character from i on that is
// not a space.
func skipSpace(s string, i int) int {
	for i < len(s) && isSpace(s[i]) {
		i++
	}
	return i
}

// declareRE finds TypeScript's ambient declarations, which the transform
// removes but which name what the module's code uses.
var declareRE = regexp.MustCompile(`\bdeclare\s+(?:const|let|var|function|class)\s+([^;\n]*)`)

// ambient returns the names src declares with declare const, let, var,
// function and class.
func ambient(src string) []string {
	var names []string
	for _, m := range declareRE.FindAllStringSubmatch(src, -1) {
		depth := 0
		start := true
		for i, r := range m[1] {
			switch {
			case r == '(' || r == '<' || r == '{' || r == '[':
				depth++
			case r == ')' || r == '>' || r == '}' || r == ']':
				depth--
			case r == ',' && depth == 0:
				start = true
			case start && (r == '_' || r == '$' || unicode.IsLetter(r)):
				end := i
				for end < len(m[1]) && (m[1][end] == '_' || m[1][end] == '$' || unicode.IsLetter(rune(m[1][end])) || unicode.IsDigit(rune(m[1][end]))) {
					end++
				}
				names = append(names, m[1][i:end])
				start = false
			case start && !unicode.IsSpace(r):
				start = false
			}
		}
	}
	return names
}

// messages makes esbuild's messages one error.
func messages(ms []api.Message) error {
	errs := make([]error, len(ms))
	for i, m := range ms {
		if loc := m.Location; loc != nil {
			errs[i] = &Error{File: loc.File, Line: loc.Line, Column: loc.Column + 1, Message: m.Text}
		} else {
			errs[i] = errors.New(m.Text)
		}
	}
	return joinErrors(errs)
}

func joinErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	return errors.Join(errs...)
}

// unwrap returns the errors err joins, or err.
func unwrap(err error) []error {
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		return j.Unwrap()
	}
	return []error{err}
}
