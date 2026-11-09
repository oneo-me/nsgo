package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/dop251/goja"
)

// runTree runs the runtime's tree.js on goja and returns a function that
// applies a batch of operations (JSON) and returns the page's setData
// object as JSON.
func runTree(t *testing.T) func(ops string) string {
	t.Helper()
	src, err := miniprogramRuntime.ReadFile("miniprogram/common/nsgo/tree.js")
	if err != nil {
		t.Fatal(err)
	}
	vm := goja.New()
	module := vm.NewObject()
	vm.Set("module", module)
	if _, err := vm.RunString(string(src)); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(`const tree = new module.exports.Tree()
function apply(ops) { tree.add(JSON.parse(ops)); return JSON.stringify(tree.take()) }`); err != nil {
		t.Fatal(err)
	}
	var apply func(string) string
	if err := vm.ExportTo(vm.Get("apply"), &apply); err != nil {
		t.Fatal(err)
	}
	return apply
}

func keys(t *testing.T, data string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		t.Fatal(err)
	}
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

func TestMiniProgramTreeSendsWhatChanged(t *testing.T) {
	apply := runTree(t)
	first := apply(`[["c",1,"view"],["root",1],["c",2,"view"],["c",3,"text"],["i",1,2,null],["i",2,3,null],
		["f",2,0,0,10,10],["s",3,"color:red;"]]`)
	if k := keys(t, first); k != "root" {
		t.Fatalf("first batch sends %s", k)
	}
	if !strings.Contains(first, `"s":"color:red;"`) {
		t.Fatalf("root = %s", first)
	}
	// A field changes by its path; a new child sends its parent's
	// children, which hold its fields.
	if k := keys(t, apply(`[["f",3,1,2,3,4]]`)); k != "root.c[0].c[0].f" {
		t.Fatalf("frame sends %s", k)
	}
	if k := keys(t, apply(`[["c",4,"view"],["i",2,4,3],["s",4,"x"],["s",3,"y"]]`)); k != "root.c[0].c" {
		t.Fatalf("insert sends %s", k)
	}
	// Modals show over the root, by their path there.
	if k := keys(t, apply(`[["c",5,"modal"],["i",3,5,null]]`)); k != "ov" {
		t.Fatalf("modal sends %s", k)
	}
	if k := keys(t, apply(`[["c",6,"view"],["i",5,6,null],["bk",0,true]]`)); k != "back ov[0].c" {
		t.Fatalf("modal child sends %s", k)
	}
	if k := keys(t, apply(`[["d",4]]`)); k != "root.c[0].c" {
		t.Fatalf("delete sends %s", k)
	}
}

func TestMiniProgramProject(t *testing.T) {
	wechat, _ := miniProgramPlatformOf("miniprogram_wechat")
	dir := t.TempDir()
	p := &project{Dir: t.TempDir(), Name: "Demo", ID: "dev.demo"}
	p.MiniProgram.CustomNavigation = true
	module := []byte("\x00asm module")
	var compressed bytes.Buffer
	w := brotli.NewWriter(&compressed)
	w.Write(module)
	w.Close()
	if err := writeMiniProgram(p, wechat, dir, compressed.Bytes()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app.js", "pages/index/index.wxml", "components/nsgo-node/index.wxml", "components/nsgo-node/index.wxs", "nsgo/runtime.js",
		"nsgo/api.js",
		"nsgo/tree.js", "nsgo/wasm_exec.js", "nsgo/config.js", "nsgo-app/index.js", "sitemap.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s", name)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "nsgo-app", "app.wasm.br"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(brotli.NewReader(bytes.NewReader(data))); !bytes.Equal(got, module) {
		t.Fatalf("module = %q", got)
	}
	var app struct {
		Window      map[string]string `json:"window"`
		SubPackages []struct {
			Root string `json:"root"`
		} `json:"subPackages"`
	}
	data, _ = os.ReadFile(filepath.Join(dir, "app.json"))
	if err := json.Unmarshal(data, &app); err != nil {
		t.Fatal(err)
	}
	if app.Window["navigationStyle"] != "custom" || len(app.SubPackages) != 1 || app.SubPackages[0].Root != miniprogramSubpackage {
		t.Fatalf("app.json = %s", data)
	}
	// An AppID DevTools wrote stays.
	os.WriteFile(filepath.Join(dir, "project.config.json"), []byte(`{"appid":"wx123"}`), 0o644)
	if err := writeMiniProgram(p, wechat, dir, compressed.Bytes()); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "project.config.json"))
	if !strings.Contains(string(data), `"appid": "wx123"`) {
		t.Fatalf("project.config.json = %s", data)
	}
	exec, _ := os.ReadFile(filepath.Join(dir, "nsgo", "wasm_exec.js"))
	if !strings.HasPrefix(string(exec), "// Go's wasm_exec.js") || !strings.Contains(string(exec), "module.exports = function (globalThis") {
		t.Fatalf("wasm_exec.js starts %q", exec[:min(len(exec), 200)])
	}
}

func TestMiniProgramPlatforms(t *testing.T) {
	for _, mp := range miniProgramPlatforms {
		t.Run(mp.name, func(t *testing.T) {
			dir := t.TempDir()
			p := &project{Dir: t.TempDir(), Name: "Demo", ID: "dev.demo"}
			p.MiniProgram.AppID = map[string]string{mp.name: "id-" + mp.name}
			if err := writeMiniProgram(p, mp, dir, []byte("module")); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"app.js", "app." + mp.style, "app.json", mp.project, "pages/index/index.js",
				"pages/index/index." + mp.markup, "pages/index/index." + mp.style, "components/nsgo-node/index.js",
				"components/nsgo-node/index." + mp.markup, "components/nsgo-node/index." + mp.style,
				"components/nsgo-node/index." + mp.script, "nsgo-app/index." + mp.markup} {
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Errorf("missing %s", name)
				}
			}
			config, _ := os.ReadFile(filepath.Join(dir, "nsgo", "config.js"))
			if !strings.Contains(string(config), `"platform": "`+mp.name+`"`) {
				t.Errorf("config.js = %s", config)
			}
			if mp.name != "wechat" {
				filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
					if ext := filepath.Ext(path); ext == ".wxml" || ext == ".wxss" || ext == ".wxs" {
						t.Errorf("%s holds WeChat's %s", mp.name, rel(path))
					}
					return nil
				})
				markup, _ := os.ReadFile(filepath.Join(dir, "components", "nsgo-node", "index."+mp.markup))
				if strings.Contains(string(markup), "wx:") || strings.Contains(string(markup), "<wxs") {
					t.Errorf("%s's markup keeps WeChat's directives", mp.name)
				}
			}
			project, _ := os.ReadFile(filepath.Join(dir, mp.project))
			if mp.name != "alipay" && !strings.Contains(string(project), `"appid": "id-`+mp.name+`"`) {
				t.Errorf("%s = %s", mp.project, project)
			}
		})
	}
}

// runAPI runs the runtime's api.js on goja over the platform's global
// object, written in JavaScript, and returns the VM with api, the result.
func runAPI(t *testing.T, platform, global string) *goja.Runtime {
	t.Helper()
	src, err := miniprogramRuntime.ReadFile("miniprogram/common/nsgo/api.js")
	if err != nil {
		t.Fatal(err)
	}
	vm := goja.New()
	module := vm.NewObject()
	vm.Set("module", module)
	if _, err := vm.RunString(string(src)); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString("const base = " + global + "\nconst api = module.exports.apiOf('" + platform + "', base)"); err != nil {
		t.Fatal(err)
	}
	return vm
}

func TestMiniProgramAlipayAPI(t *testing.T) {
	vm := runAPI(t, "alipay", `{
		store: {},
		getStorageSync(o) { return { data: this.store[o.key] } },
		setStorageSync(o) { this.store[o.key] = o.data },
		removeStorageSync(o) { delete this.store[o.key] },
		getStorageInfoSync() { return { keys: Object.keys(this.store) } },
		confirm(o) { o.success({ confirm: o.confirmButtonText === 'Yes' }) },
		alert(o) { o.success({}) },
		showActionSheet(o) { o.success({ index: o.items.length - 1 }) },
		getClipboard(o) { o.success({ text: 'copied' }) },
		chooseImage(o) { o.success({ apFilePaths: ['a.png', 'b.png'] }) },
	}`)
	for _, c := range []struct{ script, want string }{
		{"api.setStorageSync('k', 'v'); api.getStorageSync('k')", "v"},
		{"api.getStorageInfoSync().keys.join()", "k"},
		{"api.removeStorageSync('k'); String(api.getStorageSync('k'))", "undefined"},
		{"let r; api.showModal({ showCancel: true, confirmText: 'Yes', success: (x) => (r = x.confirm) }); String(r)", "true"},
		{"api.showModal({ showCancel: true, confirmText: 'No', success: (x) => (r = x.confirm) }); String(r)", "false"},
		{"api.showModal({ showCancel: false, confirmText: 'OK', success: (x) => (r = x.confirm) }); String(r)", "true"},
		{"api.showActionSheet({ itemList: ['a', 'b'], success: (x) => (r = x.tapIndex) }); String(r)", "1"},
		{"api.getClipboardData({ success: (x) => (r = x.data) }); r", "copied"},
		{"api.chooseMedia({ count: 2, success: (x) => (r = x.tempFiles.map((f) => f.tempFilePath).join()) }); r", "a.png,b.png"},
		{"typeof api.chooseMessageFile", "undefined"},
		{"typeof api.getStorageInfoSync", "function"},
	} {
		v, err := vm.RunString(c.script)
		if err != nil {
			t.Fatalf("%s: %v", c.script, err)
		}
		if got := v.String(); got != c.want {
			t.Errorf("%s = %q, want %q", c.script, got, c.want)
		}
	}
}

func TestMiniProgramAPIChoosesImages(t *testing.T) {
	vm := runAPI(t, "douyin", `{ chooseImage(o) { o.success({ tempFilePaths: ['x.jpg'] }) } }`)
	v, err := vm.RunString("let r; api.chooseMedia({ count: 1, success: (x) => (r = x.tempFiles[0].tempFilePath) }); r")
	if err != nil || v.String() != "x.jpg" {
		t.Fatalf("chooseMedia = %v, %v", v, err)
	}
	vm = runAPI(t, "wechat", `{ chooseMedia() {} }`)
	if v, _ := vm.RunString("api === base"); !v.ToBoolean() {
		t.Error("WeChat's APIs are not its own")
	}
}
