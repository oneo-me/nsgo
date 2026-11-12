package tsx_test

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/tsx"
	"github.com/nsgo-dev/nsgo/tsx/bundler"
)

// TestHotUpdatesKeepState edits a component: it renders again with the
// new code, while a counter in another module keeps its count and the
// app is not mounted again.
func TestHotUpdatesKeepState(t *testing.T) {
	files := fstest.MapFS{
		"App.tsx": {Data: []byte(`import { mount, View } from "nsgo";
import { Counter } from "./counter";
import { Greeting } from "./greeting";
mount(() => <View><Counter /><Greeting /></View>);`)},
		"counter.tsx": {Data: []byte(`import { signal, Text } from "nsgo";
const count = signal(0);
export function Counter() { return <Text testID="count" onPress={() => count.set(count() + 1)}>{() => "count " + count()}</Text>; }`)},
		"greeting.tsx": {Data: []byte(`import { Text } from "nsgo";
export function Greeting() { return <Text>Hello</Text>; }`)},
	}
	first, err := bundler.CompileHot(files, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	app, update := tsx.Hot("App.tsx", first)
	a := nsgotest.Start(t, app)
	a.Press("id:count")
	a.Press("id:count")
	if !a.Exists("text:count 2") || !a.Exists("text:Hello") {
		t.Fatalf("texts %q", a.Texts())
	}
	files["greeting.tsx"] = &fstest.MapFile{Data: []byte(`import { Text } from "nsgo";
export function Greeting() { return <Text>Hello again</Text>; }`)}
	next, err := bundler.CompileHot(files, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	update(next)
	a.Settle()
	if !a.Exists("text:Hello again") || !a.Exists("text:count 2") || len(a.Query("text:count 2")) != 1 {
		t.Errorf("after the edit %q", a.Texts())
	}
	// An edited entry mounts the app again, once; the counter's module,
	// unchanged, keeps its count.
	files["App.tsx"] = &fstest.MapFile{Data: []byte(`import { mount, Text, View } from "nsgo";
import { Counter } from "./counter";
import { Greeting } from "./greeting";
mount(() => <View><Text>Title</Text><Counter /><Greeting /></View>);`)}
	next, err = bundler.CompileHot(files, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	update(next)
	a.Settle()
	if got := a.Texts(); len(got) != 3 || got[0] != "Title" || got[1] != "count 2" {
		t.Errorf("after editing the entry %q", got)
	}
}

// hotApp runs files' hot bundle and returns what applies an edit of them.
func hotApp(t *testing.T, files fstest.MapFS, opts ...nsgotest.Option) (*nsgotest.App, func(name, src string)) {
	t.Helper()
	first, err := bundler.CompileHot(files, "App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	app, update := tsx.Hot("App.tsx", first)
	a := nsgotest.Start(t, app, opts...)
	return a, func(name, src string) {
		t.Helper()
		files[name] = &fstest.MapFile{Data: []byte(src)}
		next, err := bundler.CompileHot(files, "App.tsx")
		if err != nil {
			t.Fatal(err)
		}
		a.Post(func() { update(next) })
		a.Settle()
	}
}

// sorted is the app's texts, sorted, as NSH's hot tests compare them.
func sorted(a *nsgotest.App) string {
	texts := slices.Clone(a.Texts())
	slices.Sort(texts)
	return strings.Join(texts, ", ")
}

const hotAppSrc = `import { Image, mount, signal, Text, View } from "nsgo";
import { Counter } from "./Counter";
import dot from "./dot.png";

function App() {
  const n = signal(0);
  return <View><Image src={dot} /><Text onPress={() => n.set(n() + 1)}>app {n()}</Text><Counter /></View>;
}

mount(App);`

const hotCounterSrc = `import { signal, Text } from "nsgo";
import { greeting } from "./words";

export function Counter() {
  const c = signal(0);
  return <Text onPress={() => c.set(c() + 1)}>{greeting} {c()}</Text>;
}`

// dotPNG is a 1x1 PNG: asset modules are part of hot bundles too.
var dotPNG = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89, 0, 0, 0, 0x0d, 0x49,
	0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0, 1, 0, 0, 5, 0, 1, 0x0d, 0x0a, 0x2d, 0xb4, 0, 0,
	0, 0, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}

// TestHotEditsKeepUnrelatedState is NSH's
// edits_replace_modules_and_keep_unrelated_state (nsh-engine's hot.rs).
func TestHotEditsKeepUnrelatedState(t *testing.T) {
	var reported []error
	a, edit := hotApp(t, fstest.MapFS{
		"App.tsx":     {Data: []byte(hotAppSrc)},
		"Counter.tsx": {Data: []byte(hotCounterSrc)},
		"words.ts":    {Data: []byte(`export const greeting = "counter";`)},
		"dot.png":     {Data: dotPNG},
	}, nsgotest.Errors(func(err error) { reported = append(reported, err) }))
	check := func(want string) {
		t.Helper()
		if got := sorted(a); got != want {
			t.Fatalf("texts %q, want %q", got, want)
		}
	}
	check("app 0, counter 0")
	a.Press("text:app 0")
	a.Press("text:app 1")
	a.Press("text:counter 0")
	check("app 2, counter 1")

	// A component edit: only that component renders again.
	counter := strings.Replace(hotCounterSrc, "{greeting} {c()}", "{greeting}: {c()}", 1)
	edit("Counter.tsx", counter)
	check("app 2, counter: 0")

	// A plain module edit reaches the components that import it.
	a.Press("text:counter: 0")
	edit("words.ts", `export const greeting = "taps";`)
	check("app 2, taps: 0")

	// An edit of the root component renders it again, and what it
	// renders: their own state starts afresh.
	edit("App.tsx", strings.Replace(hotAppSrc, "app {n()}", "app! {n()}", 1))
	check("app! 0, taps: 0")

	// A component that throws reports the error and renders nothing; the
	// fix renders it again.
	edit("Counter.tsx", strings.Replace(counter, "const c = signal(0);",
		"const c = signal(0);\n  if (c() === 0) throw new Error(\"broken counter\");", 1))
	check("app! 0")
	if len(reported) != 1 || !strings.Contains(reported[0].Error(), "broken counter") {
		t.Errorf("reported %v", reported)
	}
	edit("Counter.tsx", counter)
	check("app! 0, taps: 0")
}

const hotNavSrc = `import { mount, signal, Text, View } from "nsgo";
import { NavPage, NavStack, createNavStack } from "nsgo/base";

type Route = { name: string };
const nav = createNavStack<Route>([{ name: "home" }]);
const taps = signal(0);

function Label(props: { route: string }) {
  return <Text>label {props.route}</Text>;
}

function Local() {
  const c = signal(0);
  return <Text onPress={() => c.set(c() + 1)}>local {c()}</Text>;
}

function App() {
  return (
    <View style={{ flex: 1 }}>
    <Local />
    <NavStack state={nav} style={{ flex: 1 }}>
      {(route) => (
        <NavPage title={route.name}>
          <View>
            <Text onPress={() => nav.push({ name: ` + "`page${nav.routes().length}`" + ` })}>push {route.name}</Text>
            <Text onPress={() => taps.set(taps() + 1)}>taps {taps()}</Text>
            <Label route={route.name} />
          </View>
        </NavPage>
      )}
    </NavStack>
    </View>
  );
}

MOUNT`

// TestHotEntryEditsKeepNavigationAndState is NSH's
// entry_edits_keep_navigation_and_state: edits of the entry keep the
// mounted tree, the top-level signals and the navigator's screens, with
// mount(App) and with an inline function.
func TestHotEntryEditsKeepNavigationAndState(t *testing.T) {
	for _, mount := range []string{"mount(App);", "mount(() => <App />);"} {
		t.Run(mount, func(t *testing.T) {
			src := strings.Replace(hotNavSrc, "MOUNT", mount, 1)
			a, edit := hotApp(t, fstest.MapFS{"App.tsx": {Data: []byte(src)}})
			check := func(want string) {
				t.Helper()
				if got := sorted(a); got != want {
					t.Fatalf("texts %q, want %q", got, want)
				}
			}
			check("label home, local 0, push home, taps 0")
			a.Press("text:taps 0")
			a.Press("text:push home")
			a.Advance(time.Second)
			a.Press("text:local 0")
			// The screen below is hidden, where NSH's tree keeps it.
			state := "label page1, local 1, push page1, taps 1"
			check(state)

			// An edit of another component of the entry: only it renders
			// again.
			src = strings.Replace(src, "label {props.route}", "label: {props.route}", 1)
			edit("App.tsx", src)
			labelled := strings.ReplaceAll(state, "label ", "label: ")
			check(labelled)

			// An edit of the root component renders the tree again (its
			// components' own state starts afresh), from the kept
			// navigator and signal: the pushed screen stays.
			src = strings.Replace(src, "<NavStack state={nav} style={{ flex: 1 }}>", "<NavStack state={nav} style={{ flex: 1, gap: 1 }}>", 1)
			edit("App.tsx", src)
			check(strings.Replace(labelled, "local 1", "local 0", 1))

			// A changed initializer starts afresh.
			edit("App.tsx", strings.ReplaceAll(src, "signal(0)", "signal(10)"))
			if got := sorted(a); !strings.Contains(got, "taps 10") || !strings.Contains(got, "push page1") {
				t.Errorf("texts %q", got)
			}
		})
	}
}

// TestHotContextsKeepTheirIdentity edits the module making a context: its
// provider and its readers in other modules stay connected, and the
// context takes the new default.
func TestHotContextsKeepTheirIdentity(t *testing.T) {
	a, edit := hotApp(t, fstest.MapFS{
		"App.tsx": {Data: []byte(`import { mount, View } from "nsgo";
import { Theme } from "./theme";
import { Badge, Plain } from "./badge";
mount(() => <View><Theme.Provider value="dark"><Badge /></Theme.Provider><Plain /></View>);`)},
		"theme.ts": {Data: []byte(`import { context } from "nsgo";
export const Theme = context("light");
export const version = 1;`)},
		"badge.tsx": {Data: []byte(`import { Text } from "nsgo";
import { Theme } from "./theme";
export function Badge() { return <Text>badge {Theme()}</Text>; }
export function Plain() { return <Text>plain {Theme()}</Text>; }`)},
	})
	if got := sorted(a); got != "badge dark, plain light" {
		t.Fatalf("texts %q", got)
	}
	edit("theme.ts", `import { context } from "nsgo";
export const Theme = context("sepia");
export const version = 2;`)
	if got := sorted(a); got != "badge dark, plain sepia" {
		t.Errorf("after the edit %q", got)
	}
}
