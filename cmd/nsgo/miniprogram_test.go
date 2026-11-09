package main

import (
	"bytes"
	"encoding/json"
	"io"
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
	src, err := miniprogramRuntime.ReadFile("miniprogram/nsgo/tree.js")
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
	dir := t.TempDir()
	p := &project{Dir: t.TempDir(), Name: "Demo", ID: "dev.demo"}
	p.MiniProgram.CustomNavigation = true
	module := []byte("\x00asm module")
	var compressed bytes.Buffer
	w := brotli.NewWriter(&compressed)
	w.Write(module)
	w.Close()
	if err := writeMiniProgram(p, dir, compressed.Bytes()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app.js", "pages/index/index.wxml", "components/nsgo-node/index.wxml", "nsgo/runtime.js",
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
	if err := writeMiniProgram(p, dir, compressed.Bytes()); err != nil {
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
