//go:build !nsgo_bundled

package compiler

import (
	"errors"
	"regexp"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// Hot bundles (NSH's hot.rs): esbuild's module wrapper (__commonJS)
// becomes the hot registry's, which runs a module only when its code
// changed since the bundle the VM ran before and hands importers a facade
// of its current exports, in which components are proxies. A proxy's
// instances render the component's newest code, so an edit renders the
// changed components again and the modules it did not change keep their
// state. mount replaces what it mounted before, for an edited entry.

const hotEntry = "nsgo:hot-entry"

// hotPlugin makes the entry a module of its own, required by hotEntry.
func hotPlugin(entry string) api.Plugin {
	return api.Plugin{Name: "nsgo-hot", Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: "^" + regexp.QuoteMeta(hotEntry) + "$"},
			func(api.OnResolveArgs) (api.OnResolveResult, error) {
				return api.OnResolveResult{Path: hotEntry, Namespace: "nsgo-hot"}, nil
			})
		b.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "nsgo-hot"}, func(api.OnLoadArgs) (api.OnLoadResult, error) {
			// Then the modules that changed and that nothing required.
			code := `require("./` + entry + `"); __nsgo_hot.flush();`
			return api.OnLoadResult{Contents: &code, Loader: api.LoaderJS}, nil
		})
	}}
}

// commonJS is esbuild's module wrapper.
var commonJS = regexp.MustCompile(`(?s)var __commonJS = \(cb, mod\) => function __require\d*\(\) \{.*?\n  \};\n`)

// hotModules has a bundle's modules go through the hot registry.
func hotModules(code string) (string, error) {
	loc := commonJS.FindStringIndex(code)
	if loc == nil {
		return "", errors.New("compiler: the bundle has no module wrapper to make hot")
	}
	// As many lines as the wrapper had, so the source map still holds.
	lines := strings.Count(code[loc[0]:loc[1]], "\n")
	return code[:loc[0]] + "var __commonJS = (cb) => (__nsgo_hot.define(cb), () => __nsgo_hot(cb));" +
		strings.Repeat("\n", lines) + code[loc[1]:], nil
}

// hotRuntime is the hot registry, one line before the bundle so its
// source map's lines stay the app's. It is made once per VM.
const hotRuntime = `(function (g) {` +
	`if (g.__nsgo_hot) return;` +
	`var registry = new Map(), nsgo = require("nsgo"), mount = nsgo.mount, unmount = null;` +
	`nsgo.mount = function (app, props) { if (unmount) unmount(); unmount = mount(app, props); return unmount; };` +
	`function proxy(rec, k) {` +
	`  var c = rec.proxies.get(k); if (c) return c.component;` +
	`  var impl = nsgo.signal(rec.exports[k], { equals: false });` +
	`  var component = function (props) { return function () { var f = impl(); return nsgo.untracked(function () { return f(props); }); }; };` +
	`  rec.proxies.set(k, { component: component, impl: impl }); return component;` +
	`}` +
	`function read(rec, k) {` +
	`  var v = rec.exports == null ? undefined : rec.exports[k];` +
	`  return typeof k === "string" && typeof v === "function" && /^[A-Z]/.test(k) ? proxy(rec, k) : v;` +
	`}` +
	`function facade(rec) {` +
	`  return new Proxy({}, {` +
	`    get: function (_, k) { return read(rec, k); },` +
	`    has: function (_, k) { return rec.exports != null && k in rec.exports; },` +
	`    ownKeys: function () { return rec.exports ? Reflect.ownKeys(rec.exports) : []; },` +
	`    getOwnPropertyDescriptor: function (_, k) {` +
	`      if (!rec.exports || !(k in rec.exports)) return undefined;` +
	`      return { enumerable: true, configurable: true, get: function () { return read(rec, k); } };` +
	`    }` +
	`  });` +
	`}` +
	`g.__nsgo_hot = function (cb) {` +
	`  var name = Object.getOwnPropertyNames(cb)[0], fn = cb[name], src = String(fn), rec = registry.get(name);` +
	`  if (rec && rec.src === src) return rec.facade;` +
	`  if (!rec) { rec = { proxies: new Map() }; rec.facade = facade(rec); registry.set(name, rec); }` +
	`  rec.src = src;` +
	`  var mod = { exports: {} }; fn(mod.exports, mod); rec.exports = mod.exports;` +
	`  rec.proxies.forEach(function (c, k) { c.impl.set(rec.exports[k]); });` +
	`  return rec.facade;` +
	`};` +
	`var defined = [];` +
	`g.__nsgo_hot.define = function (cb) { defined.push(cb); };` +
	`g.__nsgo_hot.flush = function () {` +
	`  var list = defined; defined = [];` +
	`  list.forEach(function (cb) { if (registry.has(Object.getOwnPropertyNames(cb)[0])) g.__nsgo_hot(cb); });` +
	`};` +
	`})(globalThis);`
