package bundler

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dop251/goja"
)

// The tests of NSH's compiler (nsh-bundler's jsx.rs), on this one's
// output: imports are CommonJS (import_nsgo.Canvas), and esbuild writes
// undefined as void 0.

func compile(t *testing.T, src string) string {
	t.Helper()
	code, err := Transform("App.tsx", src)
	if err != nil {
		t.Fatal(err)
	}
	code = code[:strings.Index(code, "//# sourceMappingURL")]
	return strings.Join(strings.Fields(code), " ")
}

func compileError(t *testing.T, src string) string {
	t.Helper()
	_, err := Transform("App.tsx", src)
	if err == nil {
		t.Fatalf("%s compiled", src)
	}
	return err.Error()
}

func contains(t *testing.T, code string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(code, p) {
			t.Errorf("no %s in\n%s", p, code)
		}
	}
}

func TestHostElementsStaticAndDynamicProps(t *testing.T) {
	code := compile(t, `import { View } from "nsgo";
const n = 1; let f = (x: number) => x;
export const a = <View testID="a" accessible pointerEvents={n} accessibilityLabel={f(n)} onPress={f} onLongPress={n > 0 ? f : undefined} />;`)
	contains(t, code,
		`_$prop(__nsgo_hv0, "testID", "a");`,
		`_$prop(__nsgo_hv0, "accessible", true);`,
		`_$prop(__nsgo_hv0, "pointerEvents", n);`,
		`_$bindProp(__nsgo_hv0, "accessibilityLabel", () => f(n));`,
		`_$prop(__nsgo_hv0, "onPress", f);`,
		`_$bindProp(__nsgo_hv0, "onLongPress", () => n > 0 ? f : void 0);`,
		`var { element: _$element, prop: _$prop, bindProp: _$bindProp } = require("nsgo:rt");`)
}

func TestPrimitivesCompileInPlace(t *testing.T) {
	code := compile(t, `import { Canvas, Image, List, Text, Video, View, WebView } from "nsgo";
export const a = <View><Text>t</Text><Image src="x" /><WebView src="about:blank" /><Video src="v" /></View>;
export const b = <List each={[1]}>{(i) => <Text>{i()}</Text>}</List>;
export const c = <Canvas draw={() => {}} />;`)
	for _, tag := range []string{"view", "text", "image", "webview", "video"} {
		contains(t, code, `_$element("`+tag+`")`)
	}
	contains(t, code, "_$component(_$list, {", "_$component(import_nsgo.Canvas, {")
}

func TestBindingsInScopeWinOverPrimitivesAndGlobals(t *testing.T) {
	code := compile(t, `import { View, Text } from "nsgo";
export function Outer() {
	const Text = (p: { children?: unknown }) => p.children;
	const If = (p: { when: boolean }) => p.when;
	return <View><Text>mine</Text><If when /></View>;
}
export const primitive = <Text>core</Text>;
export const C = Math.random() > 0.5 ? View : Text;
export const d = <C />;`)
	contains(t, code,
		// esbuild names the local Text apart from the import.
		`_$component(Text2, { get children() { return "mine"; } })`,
		"_$component(If, { when: true })",
		`_$element("text")`,
		"_$component(C, {})")
}

func TestLowercaseTagsSayWhatToWrite(t *testing.T) {
	for src, want := range map[string]string{
		"export const a = <view />;":                          "lowercase tag <view>: tags are PascalCase components: write <View> and `import { View } from \"nsgo\"`",
		"export const a = <web-view />;":                      "write <WebView> and `import { WebView } from \"nsgo\"`",
		"export const a = <each of={[]}>{() => null}</each>;": "write <Each> (a global: no import)",
	} {
		if err := compileError(t, src); !strings.Contains(err, want) {
			t.Errorf("%s: %s, want %s", src, err, want)
		}
	}
	if err := compileError(t, "export const a = <view />;"); !strings.HasPrefix(err, "App.tsx:1:19: ") {
		t.Errorf("the error is not at the tag: %s", err)
	}
}

func TestUnknownTagsSayWhatToImport(t *testing.T) {
	if err := compileError(t, "export const a = <Text>x</Text>;"); !strings.Contains(err, "<Text> names nothing in scope: add `import { Text } from \"nsgo\"`") {
		t.Error(err)
	}
	if err := compileError(t, "export const a = <Frobnicate />;"); !strings.Contains(err, "<Frobnicate> names nothing in scope") {
		t.Error(err)
	}
}

func TestComponentPropsChildrenAndSpreads(t *testing.T) {
	code := compile(t, `import { Text } from "nsgo";
declare const Card: any, s: () => number, rest: object;
export const a = <Card title={s()} n={1} {...rest} footer={<Text>f</Text>}>one {s()}</Card>;`)
	contains(t, code,
		"_$merge({ get title() { return s(); }, n: 1 }, rest, {",
		"get footer() { return (() => {",
		`get children() { return ["one ", () => s()]; }`)
	code = compile(t, `declare const Card: any, s: () => number;
export const a = <Card>{s()}</Card>;`)
	contains(t, code, "get children() { return s(); }")
}

func TestRefsAssignVariablesOrCallFunctions(t *testing.T) {
	code := compile(t, `import { View } from "nsgo";
let el: unknown; export const a = <View ref={el} />; export const b = <View ref={(n) => (el = n)} />;`)
	contains(t, code,
		"_$ref(el, __nsgo_hv0) || (el = __nsgo_hv0);",
		`_$prop(__nsgo_hv1, "ref", (n) => el = n);`)
}

func TestRefsOfConstsFieldsAndAssertions(t *testing.T) {
	code := compile(t, `import { View, signal } from "nsgo";
import { shared } from "./refs";
const s = signal<unknown>(); const refs: { a?: unknown, [k: string]: unknown } = {};
declare const k: () => string; let el: unknown;
export const a = <View ref={s} />;
export const b = <View ref={shared} />;
export const c = <View ref={refs.a} />;
export const d = <View ref={refs[k()]} />;
export const e = <View ref={el!} />;`)
	contains(t, code,
		`_$ref(s, __nsgo_hv0, "s");`,
		`_$ref(import_refs.shared, __nsgo_hv1, "shared");`,
		`((__nsgo_ho, __nsgo_hk) => _$ref(__nsgo_ho[__nsgo_hk], __nsgo_hv2) || (__nsgo_ho[__nsgo_hk] = __nsgo_hv2))(refs, "a");`,
		"|| (__nsgo_ho[__nsgo_hk] = __nsgo_hv3))(refs, k());",
		"_$ref(el, __nsgo_hv4) || (el = __nsgo_hv4);")
}

func TestTextOutsideTextIsAnError(t *testing.T) {
	for src, want := range map[string]string{
		"<View>Hello world <Text>shown</Text></View>":                    "text in <View> is not shown: text shows in a <Text> (`<Text>Hello world</Text>`)",
		`<Scroll>{"a string"}</Scroll>`:                                  "text in <Scroll> is not shown: text shows in a <Text> (`<Text>a string</Text>`)",
		"<View>{42}</View>":                                              "text in <View> is not shown: text shows in a <Text> (`<Text>42</Text>`)",
		"<View>a sentence far longer than what the message shows</View>": "(`<Text>a sentence far longer th…</Text>`)",
	} {
		err := compileError(t, `import { Scroll, Text, View } from "nsgo"; export const a = `+src+";")
		if !strings.Contains(err, want) {
			t.Errorf("%s: %s", src, err)
		}
	}
	// Text in a <Text>, whitespace, expressions and components' children stay.
	compile(t, `import { Text, View } from "nsgo"; declare const Card: any, name: string;
export const a = <View>
  <Text>Hi {name}</Text>
  {name}
  <Card>words</Card>
</View>;`)
}

func TestAnIfWithAFunctionChildHandsItAnAccessorOfWhen(t *testing.T) {
	code := compile(t, `import { Text } from "nsgo";
declare const user: () => { name: string } | null;
export const a = <If when={user()} else={<Text>out</Text>}>
  {(u) => <Text>{u().name}</Text>}
</If>;`)
	contains(t, code,
		"((__nsgo_hw) => _$cond(__nsgo_hw, () => ((u) => (() => {",
		"})())(__nsgo_hw), () => (() => {",
		"))(() => user());")
}

func TestControlFlow(t *testing.T) {
	code := compile(t, `import { View, Text } from "nsgo";
declare const on: () => boolean, items: () => { id: string }[];
export const a = <View>
	<If when={on()} else={<Text>off</Text>}>on</If>
	<Each of={items()} key={(i) => i.id}>{(item) => <Text>{item().id}</Text>}</Each>
</View>;`)
	contains(t, code,
		`_$cond(() => on(), () => "on", () => (() => {`,
		"_$each(() => items(), (item) => (() => {",
		"(i) => i.id)")
	if err := compileError(t, `import { Text } from "nsgo"; export const a = <Each of={[]}><Text /></Each>;`); !strings.Contains(err, "<Each> takes one child") {
		t.Error(err)
	}
	if err := compileError(t, "export const a = <If>x</If>;"); !strings.Contains(err, "<If> needs `when`") {
		t.Error(err)
	}
}

func TestTextLikeReact(t *testing.T) {
	code := compile(t, `import { Text } from "nsgo";
export const a = <Text>
   hello
   world  &amp; &#65;&#x42;
  </Text>;`)
	contains(t, code, `_$text("hello world & AB")`)
}

func TestErrorsPointAtTheSource(t *testing.T) {
	err := compileError(t, "import { View } from \"nsgo\";\n\nexport const a = <View>\n  hi\n</View>;")
	if !strings.HasPrefix(err, "App.tsx:4:") {
		t.Errorf("the error is not on line 4: %s", err)
	}
}

func TestStacksPointAtTheSource(t *testing.T) {
	fsys := fstest.MapFS{"App.tsx": {Data: []byte(`import { View } from "nsgo";

function boom(): string {
  throw new Error("bad");
}
export const a = <View testID={boom()} />;
`)}}
	code, err := Compile(fsys, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	vm := goja.New()
	rt := vm.NewObject()
	for _, name := range []string{"element", "prop"} {
		_ = rt.Set(name, func(goja.FunctionCall) goja.Value { return vm.NewObject() })
	}
	_ = rt.Set("bindProp", func(c goja.FunctionCall) goja.Value {
		f, _ := goja.AssertFunction(c.Argument(2))
		if _, err := f(goja.Undefined()); err != nil {
			panic(err)
		}
		return goja.Undefined()
	})
	_ = vm.Set("require", func(name string) goja.Value {
		if name == "nsgo:rt" {
			return rt
		}
		return vm.NewObject()
	})
	_, err = vm.RunScript("bundle.js", code)
	if err == nil || !strings.Contains(err.Error(), "bad") || !strings.Contains(err.Error(), "App.tsx:4:") {
		t.Errorf("the error %v is not at App.tsx:4", err)
	}
}
