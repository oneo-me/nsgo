package tsx_test

import (
	"os"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/tsx/compiler"
)

// start compiles an app of testdata and runs it headless.
func start(t *testing.T, entry string) *nsgotest.App {
	t.Helper()
	return nsgotest.Start(t, compiler.App(os.DirFS("testdata"), entry))
}

// TestControlFlow runs flow.tsx through its steps, reading the texts each
// leaves.
func TestControlFlow(t *testing.T) {
	app := start(t, "flow.tsx")
	steps := []struct {
		press string
		want  []string
	}{
		{"", []string{"toggle", "off", "reverse", "add", "a", "b", "c", "Count 0", "tap 0 times"}},
		{"id:toggle", []string{"toggle", "on", "reverse", "add", "a", "b", "c", "Count 0", "tap 0 times"}},
		{"id:reverse", []string{"toggle", "on", "reverse", "add", "c", "b", "a", "Count 0", "tap 0 times"}},
		{"id:add", []string{"toggle", "on", "reverse", "add", "c", "b", "a", "d3", "Count 0", "tap 0 times"}},
		{"id:row-b", []string{"toggle", "on", "reverse", "add", "c", "a", "d3", "Count 0", "tap 0 times"}},
		{"id:tap", []string{"toggle", "on", "reverse", "add", "c", "a", "d3", "Count 1", "tap 1 times"}},
		{"id:tap", []string{"toggle", "on", "reverse", "add", "c", "a", "d3", "Count 2", "tap 2 times", "many"}},
		{"id:toggle", []string{"toggle", "off", "reverse", "add", "c", "a", "d3", "Count 2", "tap 2 times", "many"}},
	}
	for _, s := range steps {
		if s.press != "" {
			app.Press(s.press)
		}
		if texts := app.Texts(); !slices.Equal(texts, s.want) {
			t.Fatalf("after %q: texts %q, want %q\n%s", s.press, texts, s.want, app.Dump())
		}
	}
}

// run compiles an App.tsx of source and runs it headless.
func run(t *testing.T, source string, opts ...nsgotest.Option) *nsgotest.App {
	t.Helper()
	fsys := fstest.MapFS{"App.tsx": {Data: []byte(source)}}
	return nsgotest.Start(t, compiler.App(fsys, "App.tsx"), opts...)
}

func TestClassesAndClassList(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
function App() {
	const on = signal(false);
	return <View testID="box" class="p-4 bg-white" classList={{ "bg-black": on() }} onPress={() => on.set(!on())}>
		<Text testID="t" class="text-lg">Hi <Text class="font-bold">there</Text></Text>
	</View>;
}
mount(App);`)
	white, black := core.RGB(0xffffff), core.RGB(0)
	if bg := app.Find("id:box").Style.Background; bg != white {
		t.Errorf("the box is %#x, want white", bg)
	}
	app.Press("id:box")
	if bg := app.Find("id:box").Style.Background; bg != black {
		t.Errorf("after the press the box is %#x, want black", bg)
	}
	if s := app.Find("id:t").TextStyle; s.Font.Size != 18 {
		t.Errorf("the text is %v points", s.Font.Size)
	}
	if v := app.Find("id:t"); len(v.Spans) != 2 || v.Spans[1].Style.Font.Weight != 700 {
		t.Errorf("the spans %+v", v.Spans)
	}
}

func TestErrorBoundaryShowsTheFallbackAndResets(t *testing.T) {
	app := run(t, `import { ErrorBoundary, mount, signal, Text, View } from "nsgo";
const broken = signal(false);
function check() {
	if (broken()) throw new Error("boom");
	return "fine";
}
function Fragile() {
	return <Text>{check()}</Text>;
}
function App() {
	return <View>
		<View testID="break" onPress={() => broken.set(true)}><Text>break</Text></View>
		<ErrorBoundary fallback={(e, reset) => <View testID="reset" onPress={() => { broken.set(false); reset(); }}><Text>{"caught " + e.message}</Text></View>}>
			{() => <Fragile />}
		</ErrorBoundary>
	</View>;
}
mount(App);`)
	if !slices.Contains(app.Texts(), "fine") {
		t.Fatalf("texts %q", app.Texts())
	}
	app.Press("id:break")
	if !slices.Contains(app.Texts(), "caught boom") {
		t.Fatalf("after the throw: %q", app.Texts())
	}
	app.Press("id:reset")
	if !slices.Contains(app.Texts(), "fine") {
		t.Errorf("after the reset: %q", app.Texts())
	}
}

func TestContextRefsSpreadsAndHostComponents(t *testing.T) {
	app := run(t, `import { context, mount, signal, Text, View } from "nsgo";
const Theme = context("light");
function Label() { return <Text testID="label">{"theme " + Theme()}</Text>; }
function App() {
	const node = signal();
	const props = { testID: "spread", class: "p-2" };
	const Tag = Math.random() < 2 ? View : Text;
	return <View>
		<Theme.Provider value="dark"><Label /></Theme.Provider>
		<View ref={node} {...props} />
		<Text testID="ref">{node() ? "has ref" : "no ref"}</Text>
		<Tag testID="dynamic" class="h-4" />
	</View>;
}
mount(App);`)
	texts := app.Texts()
	if !slices.Contains(texts, "theme dark") || !slices.Contains(texts, "has ref") {
		t.Errorf("texts %q", texts)
	}
	if v := app.Find("id:spread"); v.Frame.Size.Height != 16 {
		t.Errorf("the spread view is %v tall, want its p-2's 16", v.Frame.Size.Height)
	}
	if v := app.Find("id:dynamic"); v.Frame.Size.Height != 16 {
		t.Errorf("the dynamic tag is %v tall", v.Frame.Size.Height)
	}
}

func TestControlledInputScrollAndImage(t *testing.T) {
	app := run(t, `import { Image, mount, Scroll, signal, Text } from "nsgo";
import { Input } from "nsgo/base";
function App() {
	const name = signal("");
	return <Scroll testID="scroll" class="flex-1">
		<Input testID="name" value={name()} onChange={({ value }) => name.set(value.toUpperCase())} placeholder="Name" />
		<Text>{"hello " + name()}</Text>
		<Image testID="img" src={{ uri: "https://example.com/a.png", width: 20, height: 10 }} />
	</Scroll>;
}
mount(App);`)
	app.Type("id:name", "ada")
	if !slices.Contains(app.Texts(), "hello ADA") {
		t.Errorf("texts %q", app.Texts())
	}
	if v := app.Find("id:name"); v.Text != "ADA" || v.Input.Placeholder != "Name" {
		t.Errorf("the field shows %q, placeholder %q", v.Text, v.Input.Placeholder)
	}
	// Stretched across the scroll view, at the source's proportions.
	if v := app.Find("id:img"); v.Frame.Size.Height*2 != v.Frame.Size.Width {
		t.Errorf("the image is %v", v.Frame.Size)
	}
}

func TestTheEnvironment(t *testing.T) {
	app := run(t, `import { colorScheme, mount, platform, setColorScheme, Text, View, windowSize } from "nsgo";
function App() {
	return <View testID="toggle" onPress={() => setColorScheme("dark")}>
		<Text>{colorScheme() + " " + windowSize().width + " " + platform.select({ ios: "ios", default: "other" })}</Text>
	</View>;
}
mount(App);`, nsgotest.Environment(core.Environment{OS: core.OSIOS}))
	if texts := app.Texts(); !slices.Contains(texts, "light 390 ios") {
		t.Fatalf("texts %q", texts)
	}
	app.Press("id:toggle")
	if texts := app.Texts(); !slices.Contains(texts, "dark 390 ios") {
		t.Errorf("after setColorScheme: %q", texts)
	}
}

func TestTimers(t *testing.T) {
	app := run(t, `import { mount, signal, Text } from "nsgo";
function App() {
	const ticks = signal(0);
	const id = setInterval(() => ticks.update((n) => n + 1), 100);
	setTimeout(() => clearInterval(id), 350);
	return <Text>{"ticks " + ticks()}</Text>;
}
mount(App);`)
	app.Advance(time.Second)
	if texts := app.Texts(); !slices.Equal(texts, []string{"ticks 3"}) {
		t.Errorf("texts %q", texts)
	}
}

// TestTheReadmeExample runs README.md's TSX app.
func TestTheReadmeExample(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";

function App() {
  const count = signal(0);
  return (
    <View class="gap-3 p-6" onPress={() => count.update((n) => n + 1)}>
      <Text class="text-lg font-bold">Pressed {count()} times</Text>
    </View>
  );
}

mount(App);`)
	app.Press("text:Pressed 0 times")
	if texts := app.Texts(); !slices.Equal(texts, []string{"Pressed 1 times"}) {
		t.Errorf("texts %q", texts)
	}
}

func TestAsyncGeneratorsAreLowered(t *testing.T) {
	app := run(t, `import { mount, signal, Text } from "nsgo";
async function* count() { yield 1; yield 2; }
function App() {
	const total = signal(0);
	(async () => { for await (const n of count()) total.set(total() + n); })();
	return <Text testID="t">{() => "total " + total()}</Text>;
}
mount(App);`)
	app.WaitFor("text:total 3", time.Second)
}
