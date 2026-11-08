package tsx

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/oneo-me/nsgo/core"
)

// TypeScript declarations (NSH's declarations.rs): nsgo's own modules are
// written in types/, and the Go components registered for TSX are
// declared from their types, as their faces read them. The nsgo tool
// writes them to an app's .nsgo/types for editors and nsgo check.

//go:embed types/*.d.ts
var typesFS embed.FS

// registeredType is a registered component, for its declaration.
type registeredType struct {
	name string
	t    reflect.Type
	spec *faceSpec
}

var registeredTypes = map[string][]registeredType{}

// registeredFunc is a registered function, for its declaration.
type registeredFunc struct {
	name string
	t    reflect.Type
}

var registeredFuncs = map[string][]registeredFunc{}

// Declarations returns the TypeScript declarations of the modules TSX
// imports, by path under .nsgo/: types/nsgo/*.d.ts for NSGo's, a
// types/<module>.d.ts for each module registered (tsx.Register), and
// tsconfig.json, which an app's tsconfig.json extends.
func Declarations() map[string]string {
	out := map[string]string{}
	entries, _ := typesFS.ReadDir("types")
	for _, e := range entries {
		b, _ := typesFS.ReadFile("types/" + e.Name())
		out["types/nsgo/"+e.Name()] = string(b)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	modules := make([]string, 0, len(registeredTypes))
	for m := range registeredTypes {
		modules = append(modules, m)
	}
	for m := range registeredFuncs {
		if _, ok := registeredTypes[m]; !ok {
			modules = append(modules, m)
		}
	}
	for m := range registeredClasses {
		if !slices.Contains(modules, m) {
			modules = append(modules, m)
		}
	}
	modules = append(modules, catalogModules(modules)...)
	sort.Strings(modules)
	for _, m := range modules {
		var path, from string
		switch {
		case strings.HasPrefix(m, "nsgo/"):
			path, from = "types/"+m+".d.ts", "./"
			if n := strings.Count(m, "/") - 1; n > 0 {
				from = strings.Repeat("../", n)
			}
		default:
			path, from = "types/"+m+".d.ts", ""
		}
		d := newDeclarer()
		d.module = m
		for _, c := range registeredClasses[m] {
			d.class(c)
		}
		for _, c := range registeredTypes[m] {
			d.component(c)
		}
		funcs := registeredFuncs[m]
		sort.Slice(funcs, func(i, j int) bool { return funcs[i].name < funcs[j].name })
		for _, f := range funcs {
			d.function(f)
		}
		out[path] = out[path] + d.file(from)
		if c, ok := registeredCatalogs[m]; ok {
			out[path] += messagesDecl(c)
		}
	}
	out["types/nsgo/tokens.d.ts"] = TokenDeclarations(core.DefaultTheme())
	out["tsconfig.json"] = tsconfig
	return out
}

// TokenDeclarations declares the names token() reads in theme t, for
// .nsgo/types/nsgo/tokens.d.ts: its colors, as strings, and its tokens,
// lengths as numbers. The nsgo tool writes it from the app's theme.
func TokenDeclarations(t *core.Theme) string {
	kinds := map[string]string{}
	for _, p := range []*core.Palette{&t.Light, &t.Dark} {
		for _, n := range p.ColorNames() {
			kinds[n] = "string"
		}
	}
	for n, v := range t.Tokens {
		if _, ok := kinds[n]; ok {
			continue
		}
		kinds[n] = "string"
		if _, ok := cssLength(v); ok {
			kinds[n] = "number"
		}
	}
	names := make([]string, 0, len(kinds))
	for n := range kinds {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("// Written by nsgo from the app's theme: the names token() reads.\nimport \"nsgo\";\n\ndeclare module \"nsgo\" {\n  interface Tokens {\n")
	for _, n := range names {
		fmt.Fprintf(&b, "    %q: %s;\n", n, kinds[n])
	}
	b.WriteString("  }\n}\n")
	return b.String()
}

// WriteDeclarations writes Declarations under dir, an app's .nsgo.
func WriteDeclarations(dir string) error {
	for name, content := range Declarations() {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

const tsconfig = `// Written by nsgo, which keeps it current: the TypeScript settings of an
// NSGo app. The app's tsconfig.json extends it; set your own there.
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM"],
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "preserve",
    "jsxImportSource": "nsgo",
    "strict": true,
    "noEmit": true,
    "isolatedModules": true,
    "skipLibCheck": true,
    "types": [],
    "paths": {
      "nsgo": ["./types/nsgo/index.d.ts"],
      "nsgo/*": ["./types/nsgo/*.d.ts"],
      "*": ["./types/*.d.ts"]
    }
  },
  "files": ["./types/nsgo/globals.d.ts", "./types/nsgo/assets.d.ts", "./types/nsgo/tokens.d.ts"]
}
`

// declarer writes one module's declarations.
type declarer struct {
	b      strings.Builder
	named  map[reflect.Type]string
	queue  []reflect.Type
	extras strings.Builder
	// current is the component being declared, which names its ref, and
	// module the module declared.
	current, module string
	// async is set while an async call's parameters are declared.
	async bool
}

// typePrefixes name the types of the packages of a component as NSH
// does, after it: editor.Position is EditorPosition.
var typePrefixes = map[string]string{
	"github.com/oneo-me/nsgo/base/datatable": "DataTable",
	"github.com/oneo-me/nsgo/base/diff":      "Diff",
	"github.com/oneo-me/nsgo/base/dock":      "Dock",
	"github.com/oneo-me/nsgo/base/editor":    "Editor",
	"github.com/oneo-me/nsgo/base/plot":      "Plot",
	"github.com/oneo-me/nsgo/base/textview":  "TextView",
	"github.com/oneo-me/nsgo/base/toast":     "Toast",
	"github.com/oneo-me/nsgo/core":           "Core",
}

// typeRenames are the types NSH names otherwise, by the name they would
// have.
var typeRenames = map[string]string{"DockLayout": "DockAreaLayout", "Reason": "OpenChangeReason",
	"EditorSeverity": "EditorDiagnosticSeverity", "EditorRelatedInformation": "EditorDiagnosticRelatedInformation",
	"DiffExpansion": "DiffExpand", "DiffClasses": "DiffViewClasses", "DiffStyles": "DiffViewStyles",
	"CoreAlign": "Align", "CorePlacement": "Placement", "PlotCurve": "Curve",
	"EditorCodeDescription": "EditorDiagnosticCodeDescription", "ToastStatus": "ToastTransitionStatus",
	"AccessibilityValue": "PlotAccessibilityValue", "CoreAccessibilityValue": "PlotAccessibilityValue",
	"ToastSwipeDirection": "SwipeDirection", "PlotTooltipPlacement": "TooltipPlacement",
	"EditorTokenKind": "EditorToken", "TextViewOptions": "MarkdownOptions", "TextViewExtension": "MarkdownExtension",
	"TextViewCustom": "TextViewCustomNode"}

// inlineTypes are the types NSH declares in place rather than by name.
// "Size" keeps ui.Size from hiding the Size of a point.
var inlineTypes = map[string]bool{"Size": true, "TextViewColumnAlign": true, "TextViewTableCell": true, "TextViewCodeBlockAction": true}

// namedMembers are the members of unions declared by name in them, as
// NSH's are ({ type: "table" } & TextViewTable); the others are in place.
var namedMembers = map[string]bool{"TextViewCodeBlock": true, "TextViewTable": true, "TextViewCustomNode": true,
	"TextViewImage": true}

// mapNames are the maps by an enum's names NSH declares as interfaces of
// those names, by key type and value: TextView's classes and styles.
var mapNames = map[string]string{"TextViewPart string": "TextViewClasses",
	"TextViewPart StyleProp<TextStyle>": "TextViewStyles"}

// typeName is the name a named type is declared under.
func typeName(t reflect.Type) string {
	n := t.Name()
	if p := typePrefixes[t.PkgPath()]; p != "" && !strings.HasPrefix(n, p) {
		n = p + n
	}
	if r, ok := typeRenames[n]; ok {
		return r
	}
	return n
}

func newDeclarer() *declarer { return &declarer{named: map[reflect.Type]string{}} }

// file is the module's declarations, importing nsgo's types from from
// ("./" in nsgo/base, "" for nsgo).
func (d *declarer) file(from string) string {
	for len(d.queue) > 0 {
		t := d.queue[0]
		d.queue = d.queue[1:]
		d.structDecl(t)
	}
	types, index := "nsgo", "nsgo"
	if from != "" {
		types, index = from+"types", from+"index"
	}
	return "\n// Go components, declared from their types.\n" +
		fmt.Sprintf("import type { ComponentHostProps, JSX, StyleProp, TextStyle, ViewStyle, YNode, Point, Size, Bounds, Edges, ImageAsset } from %q;\n", types) +
		fmt.Sprintf("import type { AnimatedNode, AnimatedValue, Easing } from %q;\n\n", index) +
		d.extras.String() + d.b.String()
}

func (d *declarer) component(c registeredType) {
	fields := planFields(c.t, c.spec)
	d.current = c.name
	fmt.Fprintf(&d.b, "export interface %sProps extends ComponentHostProps {\n", c.name)
	seen := map[string]bool{"class": true, "style": true, "testID": true}
	byIndex := map[string]reflect.StructField{}
	for _, f := range reflect.VisibleFields(c.t) {
		byIndex[fmt.Sprint(f.Index)] = f
	}
	for _, f := range fields {
		sf := byIndex[fmt.Sprint(f.index)]
		if decls, ok := c.spec.decls[sf.Name]; ok {
			for _, t := range c.spec.types[sf.Name] {
				d.ts(t)
			}
			for _, l := range decls {
				name, _, _ := strings.Cut(l, "?")
				if !seen[name] {
					seen[name] = true
					fmt.Fprintf(&d.b, "  %s;\n", l)
				}
			}
			continue
		}
		if seen[f.prop] {
			continue
		}
		seen[f.prop] = true
		if sf.Type.Kind() == reflect.Slice && parts[sf.Type.Elem()] && !seen["children"] {
			// Its parts are its children.
			seen["children"] = true
			d.b.WriteString("  children?: JSX.Element;\n")
			continue
		}
		fmt.Fprintf(&d.b, "  %s?: %s;\n", f.prop, d.handlerType(c, sf))
	}
	d.b.WriteString("}\n")
	fmt.Fprintf(&d.b, "export declare const %s: (props: %sProps) => YNode;\n\n", c.name, c.name)
}

// handlerType is propType for a field of c, an event's taking the one
// object it passes.
func (d *declarer) handlerType(c registeredType, f reflect.StructField) string {
	if !isEvent(f.Name, f.Type) {
		return d.propType(f)
	}
	names := eventFields(c.spec, c.t, f.Name, f.Type)
	if names == nil {
		return d.propType(f)
	}
	fields := make([]string, len(names))
	for i, n := range names {
		fields[i] = n + ": " + d.ts(f.Type.In(i))
	}
	return "(e: { " + strings.Join(fields, "; ") + " }) => void"
}

// propType is the TypeScript type of a field's prop.
func (d *declarer) propType(f reflect.StructField) string {
	t := f.Type
	switch {
	case f.Name == "Children" && t == nodesType:
		return "JSX.Element"
	case strings.HasSuffix(strings.TrimSuffix(f.Name, "Func"), "Style") && (t == styleType || isGetter(t, styleType)):
		return "StyleProp<ViewStyle>"
	case f.Name == "Ref":
		if t.Kind() == reflect.Pointer && t != refType {
			n := d.refType(t)
			return n + " | ((ref: " + n + ") => void)"
		}
		return "YNode | ((node: YNode) => void)"
	case t.Kind() == reflect.Bool && strings.HasPrefix(f.Name, "No"):
		return "boolean"
	case t.Kind() == reflect.Func && t.NumIn() == 0 && t.NumOut() == 1 && t.Out(0) != nodeType:
		return d.ts(t.Out(0))
	case t.Kind() == reflect.Func && t.NumOut() == 1 && t.Out(0) == nodeType:
		return "(" + d.params(t, true) + ") => JSX.Element"
	case t.Kind() == reflect.Func:
		out := "void"
		if t.NumOut() == 1 {
			out = d.ts(t.Out(0))
		}
		return "(" + d.params(t, false) + ") => " + out
	}
	return d.ts(t)
}

// params declares a function's parameters, the scope left out; getters
// are what they get.
func (d *declarer) params(t reflect.Type, render bool) string {
	var ps []string
	for i := range t.NumIn() {
		in := t.In(i)
		if in == scopeType {
			continue
		}
		if render && in.Kind() == reflect.Func && in.NumIn() == 0 && in.NumOut() == 1 {
			in = in.Out(0)
		}
		ps = append(ps, fmt.Sprintf("a%d: %s", len(ps), d.ts(in)))
	}
	return strings.Join(ps, ", ")
}

// ts is the TypeScript type of a value of t.
func (d *declarer) ts(t reflect.Type) string {
	// Declarations runs with the registry locked.
	if c := classes[t]; c != nil {
		if c.module == d.module {
			return c.name
		}
		return fmt.Sprintf("import(%q).%s", c.module, c.name)
	}
	if s, ok := shapeOf(t); ok {
		if _, done := d.named[t]; !done {
			d.named[t] = s.name
			d.extras.WriteString(s.decl + "\n\n")
		}
		return s.name
	}
	switch t {
	case nodeType, nodesType:
		return "JSX.Element"
	case styleType:
		return "StyleProp<TextStyle>"
	case colorType:
		return "string"
	case lengthType:
		return "number | string"
	case durationType:
		return "number"
	case pointType:
		return "Point"
	case sizeType:
		return "Size"
	case boundsType:
		return "Bounds"
	case edgesType:
		return "number | Partial<Edges>"
	case imageType:
		return "string | ImageAsset"
	case valueType:
		return "AnimatedValue"
	case animNodeType:
		return "AnimatedNode"
	case easingType:
		return "Easing"
	case timingType:
		return "{ duration?: number; easing?: Easing; delay?: number }"
	case regexpType:
		return "string | RegExp"
	case pressType:
		return "{ position: Point; pagePosition: Point; clickCount: number }"
	case errorType:
		return "Error | null"
	}
	enumMu.RLock()
	names, isEnum := enums[t]
	members, isUnion := unions[t]
	enumMu.RUnlock()
	if isUnion {
		return d.union(t, members)
	}
	if isEnum {
		keys := make([]string, 0, len(names))
		for k := range names {
			keys = append(keys, fmt.Sprintf("%q", k))
		}
		sort.Strings(keys)
		union := strings.Join(keys, " | ")
		if t.Name() == "" || inlineTypes[typeName(t)] {
			return union
		}
		// A named enum is a type of its own the app may name.
		if n, ok := d.named[t]; ok {
			return n
		}
		n := typeName(t)
		d.named[t] = n
		fmt.Fprintf(&d.extras, "export type %s = %s;\n\n", n, union)
		return n
	}
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.String:
		// A named string type of a component's package is a type the app
		// may name: EditorCommand.
		if t.Name() != "" && typePrefixes[t.PkgPath()] != "" {
			if n, ok := d.named[t]; ok {
				return n
			}
			n := typeName(t)
			d.named[t] = n
			fmt.Fprintf(&d.extras, "export type %s = string;\n\n", n)
			return n
		}
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8,
		reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "(" + d.ts(t.Elem()) + ")[]"
	case reflect.Map:
		if name, ok := mapNames[typeName(t.Key())+" "+d.ts(t.Elem())]; ok {
			return d.mapInterface(t, name)
		}
		return "Partial<Record<" + d.ts(t.Key()) + ", " + d.ts(t.Elem()) + ">>"
	case reflect.Pointer:
		return d.ts(t.Elem()) + " | null"
	case reflect.Struct:
		if inlineTypes[typeName(t)] {
			return d.inlineStruct(t)
		}
		return d.structName(t)
	case reflect.Func:
		out := d.result(t)
		if d.async && out != "void" {
			// A function given to an async call may answer later.
			out += " | Promise<" + out + ">"
		}
		return "(" + d.params(t, false) + ") => " + out
	}
	return "unknown"
}

// function declares a registered function: its error result throws.
func (d *declarer) function(f registeredFunc) {
	fmt.Fprintf(&d.b, "export declare function %s%s;\n", f.name, d.signature(f.t, 0))
}

// signature is a Go call's parameters and result from its first'th
// parameter, as callGo gives them: an async call's context and a leading
// scope left out and its result a promise, its error result thrown.
func (d *declarer) signature(t reflect.Type, first int) string {
	async := isAsync(t, first)
	if async {
		first++
	}
	if first < t.NumIn() && t.In(first) == scopeType {
		first++
	}
	d.async = async
	defer func() { d.async = false }()
	var params []string
	for i := first; i < t.NumIn(); i++ {
		in := t.In(i)
		if t.IsVariadic() && i == t.NumIn()-1 {
			params = append(params, fmt.Sprintf("...arg%d: %s", i-first, d.ts(in)))
			continue
		}
		params = append(params, fmt.Sprintf("arg%d: %s", i-first, d.ts(in)))
	}
	result := d.result(t)
	if async {
		result = "Promise<" + result + ">"
	}
	return "(" + strings.Join(params, ", ") + "): " + result
}

// result is the TypeScript type of what a Go call returns.
func (d *declarer) result(t reflect.Type) string {
	outs := t.NumOut()
	if outs > 0 && t.Out(outs-1) == errorType {
		outs--
	}
	switch {
	case outs == 1:
		return d.ts(t.Out(0))
	case outs == 2 && t.Out(1).Kind() == reflect.Bool:
		// A false ok is null (results).
		return d.ts(t.Out(0)) + " | null"
	case outs > 1:
		return "unknown[]"
	}
	return "void"
}

// class declares a registered class.
func (d *declarer) class(c *classDef) {
	fmt.Fprintf(&d.b, "export declare class %s {\n", c.name)
	if c.ctor.IsValid() {
		sig := d.signature(c.ctor.Type(), 0)
		fmt.Fprintf(&d.b, "  constructor%s;\n", sig[:strings.LastIndex(sig, "):")+1])
	} else {
		d.b.WriteString("  private constructor();\n")
	}
	getters := make([]string, 0, len(c.getters))
	for g := range c.getters {
		getters = append(getters, g)
	}
	sort.Strings(getters)
	for _, g := range getters {
		name, get := lowerCamel(g), c.getters[g]
		fmt.Fprintf(&d.b, "  get %s(): %s;\n", name, d.result(get.Type))
		if s, ok := c.setters[g]; ok && s.Type.NumIn() == 2 {
			fmt.Fprintf(&d.b, "  set %s(value: %s);\n", name, d.ts(s.Type.In(1)))
		}
	}
	for _, m := range c.methods {
		fmt.Fprintf(&d.b, "  %s%s;\n", lowerCamel(m.Name), d.signature(m.Type, 1))
	}
	for _, s := range c.statics {
		fmt.Fprintf(&d.b, "  static %s%s;\n", s.name, d.signature(s.fn.Type(), 0))
	}
	d.b.WriteString("  /** Lets the Go value go now, closing it; the object throws after. */\n  dispose(): void;\n")
	d.b.WriteString("  [Symbol.dispose](): void;\n}\n")
}

// structName names a struct type's interface, declared once.
func (d *declarer) structName(t reflect.Type) string {
	if n, ok := d.named[t]; ok {
		return n
	}
	n := typeName(t)
	if t.Name() == "" || strings.Contains(n, "[") {
		n = "Value" + fmt.Sprint(len(d.named))
	}
	d.named[t] = n
	d.queue = append(d.queue, t)
	return n
}

func (d *declarer) structDecl(t reflect.Type) {
	// Built apart: its fields' types may declare others in extras.
	var b strings.Builder
	fmt.Fprintf(&b, "export interface %s {\n", d.named[t])
	for _, f := range d.fields(t) {
		fmt.Fprintf(&b, "  %s;\n", f)
	}
	b.WriteString("}\n\n")
	d.extras.WriteString(b.String())
}

// fields are the declarations of struct t's fields ("label?: string").
func (d *declarer) fields(t reflect.Type) []string {
	var out []string
	given := strings.HasSuffix(t.Name(), "Event") || strings.HasSuffix(t.Name(), "Request")
	for _, f := range reflect.VisibleFields(t) {
		if f.IsExported() && !f.Anonymous {
			opt := "?"
			if fieldRequired(t, f, given) {
				opt = ""
			}
			name, negated := fieldName(t, f)
			if negated {
				opt = "?"
			}
			ts := d.ts(f.Type)
			if f.Type.Kind() == reflect.Func && f.Type.NumIn() == 0 && f.Type.NumOut() == 1 && f.Type.Out(0) != nodeType {
				// A data object's func() T takes a plain T too.
				ts = d.ts(f.Type.Out(0)) + " | (" + ts + ")"
			}
			out = append(out, fmt.Sprintf("%s%s: %s", name, opt, ts))
		}
	}
	return out
}

// inlineStruct is struct t's object type in place.
func (d *declarer) inlineStruct(t reflect.Type) string {
	return "{ " + strings.Join(d.fields(t), "; ") + " }"
}

// union declares interface t, whose members carry their type tag, as a
// union of the members' objects.
func (d *declarer) union(t reflect.Type, members map[string]reflect.Type) string {
	if n, ok := d.named[t]; ok {
		return n
	}
	n := typeName(t)
	d.named[t] = n
	tags := make([]string, 0, len(members))
	for tag := range members {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	var b strings.Builder
	fmt.Fprintf(&b, "export type %s =\n", n)
	for i, tag := range tags {
		m := members[tag]
		for m.Kind() == reflect.Pointer {
			m = m.Elem()
		}
		head := fmt.Sprintf("{ type: %q }", tag)
		switch {
		case m.Kind() != reflect.Struct:
		case namedMembers[typeName(m)]:
			head = "(" + head + " & " + d.ts(m) + ")"
		case len(d.fields(m)) > 0:
			head = fmt.Sprintf("{ type: %q; %s }", tag, strings.Join(d.fields(m), "; "))
		}
		sep := ""
		if i == len(tags)-1 {
			sep = ";"
		}
		fmt.Fprintf(&b, "  | %s%s\n", head, sep)
	}
	b.WriteString("\n")
	d.extras.WriteString(b.String())
	return n
}

// mapInterface declares map t, by an enum's names, as an interface of
// those names.
func (d *declarer) mapInterface(t reflect.Type, name string) string {
	if n, ok := d.named[t]; ok {
		return n
	}
	d.named[t] = name
	enumMu.RLock()
	names := enums[t.Key()]
	enumMu.RUnlock()
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	value := d.ts(t.Elem())
	var b strings.Builder
	fmt.Fprintf(&b, "export interface %s {\n", name)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s?: %s;\n", k, value)
	}
	b.WriteString("}\n\n")
	d.extras.WriteString(b.String())
	return name
}

// fieldRequired reports whether struct t's field f is declared required:
// as Required and Optional say, else every field but pointers of what Go
// gives (events and requests), else by required.
func fieldRequired(t reflect.Type, f reflect.StructField, given bool) bool {
	fieldMu.RLock()
	set, ok := fieldMarks[t][f.Name]
	fieldMu.RUnlock()
	switch {
	case ok:
		return set
	case given:
		return f.Type.Kind() != reflect.Pointer
	}
	return required(f.Type)
}

var (
	fieldMu    sync.RWMutex
	fieldMarks = map[reflect.Type]map[string]bool{}
)

// Required declares the named fields of T required, which TypeScript
// asks an object to give: by default numbers and structs are, and every
// field of an event or a request Go hands JS (a type named …Event or
// …Request) but pointers.
func Required[T any](fields ...string) { markFields[T](true, fields) }

// Optional declares the named fields of T optional, an object may leave
// them out, its zero value read in their place.
func Optional[T any](fields ...string) { markFields[T](false, fields) }

func markFields[T any](required bool, fields []string) {
	fieldMu.Lock()
	defer fieldMu.Unlock()
	t := reflect.TypeFor[T]()
	if fieldMarks[t] == nil {
		fieldMarks[t] = map[string]bool{}
	}
	for _, f := range fields {
		fieldMarks[t][f] = required
	}
}

// required reports whether a struct's field of type t is declared
// required: numbers and structs, which the Go side always gives and an
// object rarely leaves out; strings, booleans, pointers, lists, maps,
// functions and enums an object may leave out, their zero value the
// default.
func required(t reflect.Type) bool {
	enumMu.RLock()
	_, isEnum := enums[t]
	enumMu.RUnlock()
	if isEnum {
		return false
	}
	switch t.Kind() {
	case reflect.Struct, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return t != colorType && t != durationType && t != styleType && t != pressType
	}
	return false
}

// leaveOut reports whether a trailing argument of type t may be left
// out: a flag, a choice, a pointer or a count.
func leaveOut(t reflect.Type) bool {
	enumMu.RLock()
	_, isEnum := enums[t]
	enumMu.RUnlock()
	switch t.Kind() {
	case reflect.Bool, reflect.Pointer, reflect.Int:
		return true
	}
	return isEnum
}

// refDecls declare ref methods whose arguments JS gives in a shape of
// their own, by the ref's type and the method's name: a group's ID that
// is null for none, which reads as 0.
var refDecls = map[reflect.Type]map[string]string{}

// refType declares a component's ref: its methods.
func (d *declarer) refType(t reflect.Type) string {
	if n, ok := d.named[t]; ok {
		return n
	}
	// NSH names a ref after its component: DiffViewRef.
	n := d.current + "Ref"
	d.named[t] = n
	var b strings.Builder
	fmt.Fprintf(&b, "export interface %s {\n  readonly node: YNode;\n", n)
	for i := range t.NumMethod() {
		m := t.Method(i)
		if decl, ok := refDecls[t][m.Name]; ok {
			fmt.Fprintf(&b, "  %s;\n", decl)
			continue
		}
		ft := m.Type
		var ps []string
		// Trailing flags, choices, pointers and counts may be left out, as
		// NSH's optional arguments are: they read as their zero value.
		optional := ft.NumIn()
		for optional > 2 && leaveOut(ft.In(optional-1)) {
			optional--
		}
		for j := 1; j < ft.NumIn(); j++ {
			opt := ""
			if j >= optional {
				opt = "?"
			}
			ps = append(ps, fmt.Sprintf("a%d%s: %s", j-1, opt, d.ts(ft.In(j))))
		}
		outs := ft.NumOut()
		res := "void"
		switch {
		case outs > 1 && ft.Out(outs-1).Kind() == reflect.Bool:
			if outs == 2 {
				res = "Promise<" + d.ts(ft.Out(0)) + " | null>"
			} else {
				res = "Promise<unknown | null>"
			}
		case outs == 1:
			res = "Promise<" + d.ts(ft.Out(0)) + ">"
		case outs > 1 && len(resultNames[m.Name]) == outs:
			// An object of the results, as results gives it.
			var fs []string
			for j, name := range resultNames[m.Name] {
				fs = append(fs, name+": "+d.ts(ft.Out(j)))
			}
			res = "Promise<{ " + strings.Join(fs, "; ") + " }>"
		case outs > 1:
			res = "Promise<unknown>"
		}
		fmt.Fprintf(&b, "  %s(%s): %s;\n", lowerCamel(m.Name), strings.Join(ps, ", "), res)
	}
	b.WriteString("}\n\n")
	d.extras.WriteString(b.String())
	return n
}
