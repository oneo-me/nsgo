package bundler

import (
	"errors"
	"fmt"
	"hash/fnv"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/token"
	"github.com/evanw/esbuild/pkg/api"
)

// Hot bundles (NSH's nsh-bundler hot.rs and nsh-js hot.rs): esbuild's
// module wrapper (__commonJS) defines each module with the hot registry
// instead, which runs a module once and hands importers a facade of its
// current exports. A newer bundle run in the same VM runs again the
// modules whose code changed, and the importers of those that do not take
// their own updates, up to the entry: a module takes its own when all it
// exports are components. The hot transform below has components render
// through proxies that stay the same, so an edit renders again only the
// instances whose code changed, and keeps top-level signals, navigators,
// contexts and mounts.

const hotEntry = "nsgo:hot-entry"

// hotPlugin makes the entry a module of its own, which hotEntry starts.
func hotPlugin(entry string) api.Plugin {
	return api.Plugin{Name: "nsgo-hot", Setup: func(b api.PluginBuild) {
		b.OnResolve(api.OnResolveOptions{Filter: "^" + regexp.QuoteMeta(hotEntry) + "$"},
			func(api.OnResolveArgs) (api.OnResolveResult, error) {
				return api.OnResolveResult{Path: hotEntry, Namespace: "nsgo-hot"}, nil
			})
		b.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "nsgo-hot"}, func(api.OnLoadArgs) (api.OnLoadResult, error) {
			code := `__nsgo_hot.start(() => require("./` + entry + `"));`
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
	return code[:loc[0]] + "var __commonJS = (cb) => __nsgo_hot.define(cb);" +
		strings.Repeat("\n", lines) + code[loc[1]:], nil
}

// hotRuntime is the hot registry, one line before the bundle so its
// source map's lines stay the app's. It is made once per VM.
var hotRuntime = strings.ReplaceAll(hotRuntimeSource, "\n", " ")

const hotRuntimeSource = `(function (g) {
if (g.__nsgo_hot) return;
var nsgo = require("nsgo"), rt = require("nsgo:rt"), baseMount = nsgo.mount;
var factories = new Map(), modules = new Map(), importers = new Map();
var components = new Map(), proxies = new WeakSet(), contexts = new Map(), kept = new Map();
var renders = new Map(), pending = new Map();
var defined = [], current = null, entry = null, forced = false, stale = [];
function undo(list) { list.forEach(function (r) { try { r.dispose(); } catch (e) { reportError(e); } }); }
function facade(m) {
  function read(k) { return m.mod.exports == null ? undefined : m.mod.exports[k]; }
  return new Proxy({}, {
    get: function (_, k) { return read(k); },
    has: function (_, k) { return m.mod.exports != null && k in m.mod.exports; },
    ownKeys: function () { return m.mod.exports ? Reflect.ownKeys(m.mod.exports) : []; },
    getOwnPropertyDescriptor: function (_, k) {
      if (!m.mod.exports || !(k in m.mod.exports)) return undefined;
      var d = Object.getOwnPropertyDescriptor(m.mod.exports, k);
      return { enumerable: !d || d.enumerable, configurable: true, get: function () { return read(k); } };
    }
  });
}
function run(id) {
  var m = modules.get(id), fn = factories.get(id), mod = { exports: {} };
  if (!m.mod) m.mod = mod;
  m.src = String(fn);
  m.components = 0;
  pending.set(id, renders.get(id) || []);
  renders.set(id, []);
  var prev = current;
  current = id;
  try { fn(mod.exports, mod); } finally {
    current = prev;
    var left = pending.get(id);
    pending.delete(id);
    undo(left);
  }
  m.mod = mod;
  m.accepts = m.components > 0 && Object.keys(mod.exports).every(function (k) { return proxies.has(mod.exports[k]); });
}
function affected(changed) {
  var rerun = new Set(), queue = changed.slice();
  while (queue.length) {
    var id = queue.pop();
    if (rerun.has(id)) continue;
    rerun.add(id);
    if (id === entry || modules.get(id).accepts) continue;
    (importers.get(id) || new Set()).forEach(function (from) { queue.push(from); });
  }
  var seen = new Set(), order = [];
  function visit(id) {
    if (seen.has(id)) return;
    seen.add(id);
    rerun.forEach(function (dep) { if (importers.has(dep) && importers.get(dep).has(id)) visit(dep); });
    order.push(id);
  }
  rerun.forEach(visit);
  return order;
}
var hot = g.__nsgo_hot = {};
hot.define = function (cb) {
  var id = Object.getOwnPropertyNames(cb)[0];
  factories.set(id, cb[id]);
  defined.push(id);
  return function () { return hot.require(id); };
};
hot.require = function (id) {
  if (current === null && entry === null) entry = id;
  if (current !== null) {
    if (!importers.has(id)) importers.set(id, new Set());
    importers.get(id).add(current);
  }
  var m = modules.get(id);
  if (!m) {
    m = { mod: null, components: 0, accepts: false };
    m.facade = facade(m);
    modules.set(id, m);
    run(id);
  }
  return m.facade;
};
hot.start = function (load) {
  var ids = defined;
  defined = [];
  if (entry === null) { load(); return; }
  var changed = ids.filter(function (id) { var m = modules.get(id); return m && m.src !== String(factories.get(id)); });
  var order = affected(changed);
  try {
    order.forEach(function (id) { forced = changed.indexOf(id) < 0; run(id); });
  } finally {
    forced = false;
    var list = stale;
    stale = [];
    nsgo.batch(function () { list.forEach(function (c) { c.version.set(c.version() + 1); }); });
  }
};
hot.component = function (module, name, fn, sig) {
  var key = module + ":" + name, c = components.get(key), m = current === null ? null : modules.get(current);
  if (m) m.components++;
  if (c) {
    if (c.sig !== sig || forced) stale.push(c);
    c.fn = fn;
    c.sig = sig;
    return c.proxy;
  }
  c = { fn: fn, sig: sig, version: nsgo.signal(0) };
  c.proxy = function (props) {
    return nsgo.computed(function () {
      c.version();
      var f = c.fn;
      try { return nsgo.untracked(function () { return f(props); }); } catch (e) { reportError(e); return undefined; }
    });
  };
  Object.defineProperty(c.proxy, "name", { value: name });
  proxies.add(c.proxy);
  components.set(key, c);
  return c.proxy;
};
hot.context = function (module, name, made) {
  var key = module + ":" + name, k = contexts.get(key);
  if (k) return rt.keepContext(k, made);
  contexts.set(key, made);
  return made;
};
hot.keep = function (module, name, sig, made) {
  var key = module + ":" + name, k = kept.get(key);
  if (k && k.sig === sig) return k.value;
  kept.set(key, { sig: sig, value: made });
  return made;
};
nsgo.mount = function (app, props) {
  var id = current, p = id === null ? null : pending.get(id);
  if (!p) return baseMount(app, props);
  if (props === undefined && p.length && p[0].app === app) {
    var r = p.shift();
    renders.get(id).push(r);
    return r.dispose;
  }
  undo(p.splice(0));
  var d = baseMount(app, props);
  renders.get(id).push({ app: app, dispose: d });
  return d;
};
})(globalThis);`

// The hot transform (NSH's nsh-bundler hot.rs), on a module after
// esbuild's: each top-level component, a capitalized function whose code
// has JSX, is registered with __nsgo_hot.component, which gives a proxy
// that stays the same across updates; its signature is its code and the
// module's code outside components, so an edit of either renders its
// instances again. A function given to a top-level mount is a component
// named mount#n. Top-level context() results go through
// __nsgo_hot.context, which keeps their identity, and top-level consts
// made by signal() or createNavStack() through __nsgo_hot.keep, which
// keeps their value while their initializer's code is the same.

// keptCalls are the calls whose top-level consts keep their value.
var keptCalls = map[string]bool{"signal": true, "createNavStack": true}

// insertion is text the hot transform inserts at an offset of a module.
type insertion struct {
	at   int
	text string
}

// hotInsertions returns what the hot transform inserts into src, module
// key after esbuild's transform, in order.
func hotInsertions(key, src string) ([]insertion, error) {
	prog, err := parser.ParseFile(nil, key, src, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: the transformed module does not parse: %w", key, err)
	}
	hasJSX := func(n ast.Node) bool {
		found := false
		walk(n, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpression); ok {
				if id, ok := c.Callee.(*ast.Identifier); ok && id.Name.String() == jsxFactory {
					found = true
				}
			}
			return !found
		})
		return found
	}
	type target struct {
		kind       string // component, context, keep
		name       string
		start, end int
		// after is set for a function declaration, assigned its proxy
		// after it.
		after bool
	}
	var targets []target
	mounts := 0
	for _, st := range prog.Body {
		switch st := st.(type) {
		case *ast.FunctionDeclaration:
			f := st.Function
			if f.Name != nil && isComponentName(f.Name.Name.String()) && hasJSX(f) {
				targets = append(targets, target{"component", f.Name.Name.String(), off(f.Idx0()), off(f.Idx1()), true})
			}
		case *ast.LexicalDeclaration, *ast.VariableStatement:
			list, isConst := []*ast.Binding(nil), false
			if l, ok := st.(*ast.LexicalDeclaration); ok {
				list, isConst = l.List, l.Token == token.CONST
			} else {
				list = st.(*ast.VariableStatement).List
			}
			for _, b := range list {
				id, ok := b.Target.(*ast.Identifier)
				if !ok || b.Initializer == nil {
					continue
				}
				name := id.Name.String()
				// The initializer starts after the =: goja's positions
				// leave out the parentheses of a callee such as
				// (0, import_nsgo.signal).
				start := skipSpace(src, strings.IndexByte(src[off(id.Idx1()):], '=')+off(id.Idx1())+1)
				end := off(b.Initializer.Idx1())
				switch init := b.Initializer.(type) {
				case *ast.ArrowFunctionLiteral, *ast.FunctionLiteral:
					if isComponentName(name) && hasJSX(init) {
						targets = append(targets, target{"component", name, start, end, false})
					}
				case *ast.CallExpression:
					switch c := calleeName(init.Callee); {
					case c == "context":
						targets = append(targets, target{"context", name, start, end, false})
					case isConst && keptCalls[c]:
						targets = append(targets, target{"keep", name, start, end, false})
					}
				}
			}
		case *ast.ExpressionStatement:
			call, ok := st.Expression.(*ast.CallExpression)
			if !ok || calleeName(call.Callee) != "mount" || len(call.ArgumentList) == 0 {
				continue
			}
			switch arg := call.ArgumentList[0].(type) {
			case *ast.ArrowFunctionLiteral, *ast.FunctionLiteral:
				if hasJSX(arg) {
					targets = append(targets, target{"component", fmt.Sprintf("mount#%d", mounts), off(arg.Idx0()), off(arg.Idx1()), false})
					mounts++
				}
			}
		}
	}
	// Signatures: a component's code and the module's outside components.
	rest := fnv.New64a()
	at := 0
	for _, t := range targets {
		if t.kind == "component" {
			rest.Write([]byte(src[at:t.start]))
			at = t.end
		}
	}
	rest.Write([]byte(src[at:]))
	restSum := rest.Sum64()
	signature := func(code string, withRest bool) string {
		h := fnv.New64a()
		h.Write([]byte(code))
		if withRest {
			fmt.Fprintf(h, "%x", restSum)
		}
		return fmt.Sprintf("%016x", h.Sum64())
	}
	var out []insertion
	q := strconv.Quote
	for _, t := range targets {
		code := src[t.start:t.end]
		switch {
		case t.after:
			out = append(out, insertion{t.end, fmt.Sprintf(";%s = __nsgo_hot.component(%s, %s, %s, %s);",
				t.name, q(key), q(t.name), t.name, q(signature(code, true)))})
		case t.kind == "component":
			out = append(out, insertion{t.start, fmt.Sprintf("__nsgo_hot.component(%s, %s, ", q(key), q(t.name))},
				insertion{t.end, ", " + q(signature(code, true)) + ")"})
		case t.kind == "context":
			out = append(out, insertion{t.start, fmt.Sprintf("__nsgo_hot.context(%s, %s, ", q(key), q(t.name))},
				insertion{t.end, ")"})
		case t.kind == "keep":
			out = append(out, insertion{t.start, fmt.Sprintf("__nsgo_hot.keep(%s, %s, %s, ", q(key), q(t.name), q(signature(code, false)))},
				insertion{t.end, ")"})
		}
	}
	slices.SortStableFunc(out, func(a, b insertion) int { return a.at - b.at })
	return out, nil
}

func isComponentName(name string) bool { return name != "" && name[0] >= 'A' && name[0] <= 'Z' }

// calleeName is the name a call calls: f, or the f of import_x.f and
// (0, import_x.f), as esbuild writes an imported function's call.
func calleeName(e ast.Expression) string {
	switch c := e.(type) {
	case *ast.Identifier:
		return c.Name.String()
	case *ast.DotExpression:
		return c.Identifier.Name.String()
	case *ast.SequenceExpression:
		if len(c.Sequence) > 0 {
			return calleeName(c.Sequence[len(c.Sequence)-1])
		}
	}
	return ""
}

// insert puts ins into f, a module's compiled code, at their offsets of
// its source: inside the text copied from there, or before what follows.
func (f frag) insert(ins []insertion) frag {
	if len(ins) == 0 {
		return f
	}
	var out frag
	i := 0
	for _, c := range f {
		if !c.copied {
			for i < len(ins) && c.src >= 0 && ins[i].at < c.src {
				out.gen(ins[i].text, ins[i].at)
				i++
			}
			out = append(out, c)
			continue
		}
		pos := 0
		for i < len(ins) && ins[i].at < c.src+len(c.text) {
			if cut := ins[i].at - c.src; cut > pos {
				out = append(out, chunk{c.text[pos:cut], c.src + pos, true})
				pos = cut
			}
			out.gen(ins[i].text, ins[i].at)
			i++
		}
		if pos < len(c.text) {
			out = append(out, chunk{c.text[pos:], c.src + pos, true})
		}
	}
	for ; i < len(ins); i++ {
		out.gen(ins[i].text, ins[i].at)
	}
	return out
}
