package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
)

// The miniprogram target writes a WeChat mini program project: the
// runtime (miniprogram/, embedded here), the app's module brotli
// compressed in a subpackage of its own, as WXWebAssembly loads only files
// of the code package and WeChat caps each package at 2 MB, and the files
// WeChat DevTools reads.

//go:embed miniprogram
var miniprogramRuntime embed.FS

// packageLimit is the most a mini program's package may hold, the module's
// subpackage included.
const packageLimit = 2 << 20

// touristAppID is the AppID WeChat DevTools previews projects without
// one of their own with.
const touristAppID = "touristappid"

// miniprogramSubpackage is the subpackage the module is in.
const miniprogramSubpackage = "nsgo-app"

// miniprogramTags are the build's tags: the backend, and no shader
// compiler, as the render layer draws no shader.
var miniprogramTags = []string{"nsgominiprogram", "nsgo_noshader"}

// buildMiniProgram writes the project to dir. The module is built with
// encoding/json's first implementation, which is smaller, and when it is
// still too large for its package, again without inlining.
func buildMiniProgram(ctx context.Context, p *project, dir string) error {
	// Stripped always, as anything larger does not load; and without the
	// dev client, as the logic layer makes no requests to a dev server.
	release, client := p.Release, p.Dev.Client
	p.Release, p.Dev.Client = true, false
	defer func() { p.Release, p.Dev.Client = release, client }()
	tmp, err := os.MkdirTemp("", "nsgo-miniprogram")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	wasm := filepath.Join(tmp, "app.wasm")
	env := []string{"GOOS=js", "GOARCH=wasm", "GOEXPERIMENT=" + experiments("nojsonv2")}
	// A mini program runs no code it did not ship, which the page's
	// engine would, and goja does not fit in its package.
	tsx, err := linkedDir(ctx, p, append(env, "GOFLAGS=-tags="+strings.Join(miniprogramTags, ",")), "github.com/oneo-me/nsgo/tsx")
	if err != nil {
		return err
	}
	if tsx != "" {
		return fmt.Errorf("mini programs run Go apps alone: this app links package tsx")
	}
	module, err := buildMiniProgramModule(ctx, p, wasm, env, nil)
	if err != nil {
		return err
	}
	if len(module) > packageLimit {
		stage("compiling Go without inlining")
		if module, err = buildMiniProgramModule(ctx, p, wasm, env, []string{"-gcflags=all=-l"}); err != nil {
			return err
		}
	}
	if len(module) > packageLimit {
		return fmt.Errorf("the module is %s compressed, past the %s a mini program's package holds; "+
			"leave out what the app does not show on phones, such as the editor, the terminal or TSX",
			formatSize(int64(len(module))), formatSize(packageLimit))
	}
	stage("writing the project")
	if err := writeMiniProgram(p, dir, module); err != nil {
		return err
	}
	if main := dirSize(dir) - dirSize(filepath.Join(dir, miniprogramSubpackage)); main > packageLimit {
		warn("the main package is %s, past WeChat's %s: move resources the app does not need out", formatSize(main), formatSize(packageLimit))
	}
	return nil
}

// experiments returns GOEXPERIMENT's value with extra added.
func experiments(extra string) string {
	if cur := os.Getenv("GOEXPERIMENT"); cur != "" {
		return cur + "," + extra
	}
	return extra
}

// buildMiniProgramModule builds the module to wasm and returns it
// compressed.
func buildMiniProgramModule(ctx context.Context, p *project, wasm string, env, flags []string) ([]byte, error) {
	if err := goBuildWith(ctx, p, wasm, env, buildExtra{tags: miniprogramTags, flags: flags}); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(wasm)
	if err != nil {
		return nil, err
	}
	stage("compressing the module")
	var out bytes.Buffer
	w := brotli.NewWriterLevel(&out, brotli.BestCompression)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// miniprogramConfig is nsgo/config.js: what the runtime starts the app
// with.
type miniprogramConfig struct {
	Wasm             string            `json:"wasm"`
	Subpackage       string            `json:"subpackage"`
	CustomNavigation bool              `json:"customNavigation"`
	Env              map[string]string `json:"env"`
}

// writeMiniProgram writes the runtime, module and project files to dir.
// A project written before keeps its AppID, which WeChat DevTools writes
// when it imports one with a test account.
func writeMiniProgram(p *project, dir string, module []byte) error {
	err := fs.WalkDir(miniprogramRuntime, "miniprogram", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := miniprogramRuntime.ReadFile(path)
		if err != nil {
			return err
		}
		return writeFile(filepath.Join(dir, strings.TrimPrefix(path, "miniprogram/")), data)
	})
	if err != nil {
		return err
	}
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return err
	}
	exec, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "nsgo", "wasm_exec.js"), miniprogramExec(exec)); err != nil {
		return err
	}
	cfg := miniprogramConfig{Wasm: "/" + miniprogramSubpackage + "/app.wasm.br", Subpackage: miniprogramSubpackage,
		CustomNavigation: p.MiniProgram.CustomNavigation, Env: map[string]string{}}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "nsgo", "config.js"), fmt.Appendf(nil, "module.exports = %s\n", data)); err != nil {
		return err
	}
	sub := filepath.Join(dir, miniprogramSubpackage)
	if err := writeFile(filepath.Join(sub, "app.wasm.br"), module); err != nil {
		return err
	}
	// A subpackage holds at least one page.
	for name, text := range map[string]string{
		"index.js": "Page({})\n", "index.json": "{}\n", "index.wxml": "<view />\n", "index.wxss": "",
	} {
		if err := writeFile(filepath.Join(sub, name), []byte(text)); err != nil {
			return err
		}
	}
	if err := copyResources(p, filepath.Join(dir, "resources")); err != nil {
		return err
	}
	window := map[string]any{"navigationBarTitleText": p.Name, "navigationBarBackgroundColor": "#ffffff",
		"navigationBarTextStyle": "black", "backgroundColor": "#ffffff"}
	if p.Appearance == "dark" {
		window["navigationBarBackgroundColor"], window["navigationBarTextStyle"], window["backgroundColor"] =
			"#000000", "white", "#000000"
	}
	if p.MiniProgram.CustomNavigation {
		window["navigationStyle"] = "custom"
	}
	app := map[string]any{
		"pages":           []string{"pages/index/index"},
		"subPackages":     []any{map[string]any{"root": miniprogramSubpackage, "name": miniprogramSubpackage, "pages": []string{"index"}}},
		"preloadRule":     map[string]any{"pages/index/index": map[string]any{"network": "all", "packages": []string{miniprogramSubpackage}}},
		"window":          window,
		"style":           "v2",
		"sitemapLocation": "sitemap.json",
		"lazyCodeLoading": "requiredComponents",
	}
	if p.Appearance == "" {
		app["darkmode"] = false
	}
	appid := p.MiniProgram.AppID
	if appid == "" {
		var kept struct {
			AppID string `json:"appid"`
		}
		if data, err := os.ReadFile(filepath.Join(dir, "project.config.json")); err == nil && json.Unmarshal(data, &kept) == nil &&
			kept.AppID != touristAppID {
			appid = kept.AppID
		}
	}
	if appid == "" {
		appid = touristAppID
	}
	project := map[string]any{
		"description": p.Name + " (" + p.ID + "), an NSGo app",
		"appid":       appid,
		"projectname": p.Name,
		"compileType": "miniprogram",
		"libVersion":  "3.0.0",
		// urlCheck is off: an app's images reach any host while it is
		// developed; a released mini program lists its domains.
		"setting":     map[string]any{"es6": true, "enhance": true, "minified": true, "urlCheck": false},
		"packOptions": map[string]any{"ignore": []any{}, "include": []any{}},
	}
	for name, v := range map[string]any{
		"app.json": app, "project.config.json": project,
		"sitemap.json": map[string]any{"rules": []any{map[string]any{"action": "allow", "page": "*"}}},
	} {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(dir, name), append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// miniprogramExec is Go's wasm_exec.js as a module the runtime calls with
// the global object Go runs in and what the mini program lacks: browsers'
// TextEncoder, performance and crypto, and WebAssembly, whose Instance it
// checks the module against while WXWebAssembly makes its own.
func miniprogramExec(src []byte) []byte {
	var b bytes.Buffer
	b.WriteString("// Go's wasm_exec.js (" + runtime.Version() + "), as a function of the globals it reaches.\n")
	b.WriteString("module.exports = function (globalThis, TextEncoder, TextDecoder, performance, crypto, WebAssembly) {\n")
	b.WriteString("const setTimeout = globalThis.setTimeout, clearTimeout = globalThis.clearTimeout;\n")
	b.WriteString("const fs = { writeSync: (fd, buf) => globalThis.fs.writeSync(fd, buf) };\n")
	b.Write(src)
	b.WriteString("\n};\n")
	return b.Bytes()
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// miniprogramSession writes the project into the dev cache and opens it in
// WeChat DevTools once, which compiles it again as each build changes it.
func miniprogramSession() session {
	var once sync.Once
	return session{
		build: func(ctx context.Context, p *project) error {
			return buildMiniProgram(ctx, p, p.cache("dev", "miniprogram"))
		},
		start: func(ctx context.Context, p *project) (app, error) {
			var err error
			once.Do(func() {
				dir := p.cache("dev", "miniprogram")
				note("%s", rel(dir))
				err = openMiniProgram(ctx, dir)
			})
			return app{stop: func() {}}, err
		},
	}
}

// devToolsCLI is WeChat DevTools' command line on this machine, "" when
// it is not installed.
func devToolsCLI() string {
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{"/Applications/wechatwebdevtools.app/Contents/MacOS/cli"}
	case "windows":
		paths = []string{`C:\Program Files (x86)\Tencent\微信web开发者工具\cli.bat`, `C:\Program Files\Tencent\微信web开发者工具\cli.bat`}
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// openMiniProgram opens the project in WeChat DevTools, whose service
// port must be on (Settings, Security).
func openMiniProgram(ctx context.Context, dir string) error {
	cli := devToolsCLI()
	if cli == "" {
		return fmt.Errorf("WeChat DevTools is not installed: get it at https://developers.weixin.qq.com/miniprogram/dev/devtools/download.html, then open %s", rel(dir))
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	return run(ctx, cli, "open", "--project", abs)
}
