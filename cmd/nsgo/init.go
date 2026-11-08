package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
)

// templates are what nsgo init writes: a Go app, a TSX app, or a Go app
// with a TSX entry importing Go of its own (NSH's ts, rust and mixed).
var templates = []string{"go", "tsx", "mixed"}

// initApp writes a new app of template kind into dir: nsgo.yaml, a main
// package and tests; a TSX app's entry, its tests, tsconfig.json and
// .gitignore.
func initApp(dir, name, module, kind string) error {
	if !slices.Contains(templates, kind) {
		return fmt.Errorf("init: no template %q; use %s", kind, strings.Join(templates, ", "))
	}
	if name == "" {
		name = filepath.Base(dir)
	}
	if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9 _-]*$`).MatchString(name) {
		return fmt.Errorf("init: %q is not an app name; use letters, digits, spaces, - and _", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "nsgo.yaml")); err == nil {
		return fmt.Errorf("init: %s already has an nsgo.yaml", rel(dir))
	}
	id := "dev.nsgo." + strings.ToLower(regexp.MustCompile(`[^A-Za-z0-9]+`).ReplaceAllString(name, ""))
	slug := strings.Trim(strings.ToLower(regexp.MustCompile(`[^A-Za-z0-9]+`).ReplaceAllString(name, "-")), "-")
	data := struct{ Name, ID, Module, Slug string }{name, id, module, slug}
	files := map[string]*template.Template{"nsgo.yaml": initYAML, "app.go": initApp_, "app_test.go": initTest}
	if kind != "go" {
		files = map[string]*template.Template{"nsgo.yaml": initYAML, "main.go": initTSXMain, "App.tsx": initTSX,
			"app.test.tsx": initTSXTest, "tsconfig.json": initTSConfig, ".gitignore": initIgnore}
	}
	if kind == "mixed" {
		files["App.tsx"], files["native.go"] = initMixedTSX, initNative
	}
	if module != "" {
		files["go.mod"] = initMod
	}
	names := slices.Sorted(maps.Keys(files))
	for _, file := range names {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return fmt.Errorf("init: %s exists", rel(filepath.Join(dir, file)))
		}
	}
	for _, file := range names {
		t := files[file]
		f, err := os.OpenFile(filepath.Join(dir, file), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		err = t.Execute(f, data)
		f.Close()
		if err != nil {
			return err
		}
		done("create %s", rel(filepath.Join(dir, file)))
	}
	if module != "" {
		detail("next: cd %s && go get github.com/oneo-me/nsgo && go get -tool github.com/oneo-me/nsgo/cmd/nsgo && go tool nsgo dev", rel(dir))
	} else {
		detail("next: go tool nsgo dev %s", rel(dir))
	}
	if kind != "go" {
		detail("nsgo dev and nsgo check write the TSX's types to .nsgo/types for editors; check also type-checks it")
	}
	return nil
}

var initYAML = template.Must(template.New("yaml").Parse(`name: {{.Name}}
id: {{.ID}}
version: 0.1.0

# macOS and iOS use assets/AppIcon.icon, or assets/AppIcon.icns on macOS; the
# other targets, and Apple ones without those, use assets/Icon.png. To use
# other files, name them here:
# icon:
#   apple: assets/AppIcon.icon  # an Icon Composer .icon; macOS also takes .icns
#   png: assets/Icon.png        # a square PNG, 1024 pixels wide
# nsgo dev uses assets/AppIcon-dev.icon, AppIcon-dev.icns and Icon-dev.png
# where they exist, or the files dev.icon names.

dev:
  # native: this machine's backend; web: the browser; ios: a connected iPhone; ios_simulator; android: a connected device or emulator.
  target: native
  # true, the default, rebuilds and restarts on save; false runs once and exits with the app.
  hot_reload: true
  # true shows the performance monitor (macOS).
  fps: false

build:
  output: dist
  # true opens the output directory after building.
  open: false
  # true shows the performance monitor (macOS).
  fps: false
  targets: [macos_arm64, windows_x64, ios_simulator, android_arm64, web]
`))

var initMod = template.Must(template.New("mod").Parse(`module {{.Module}}

go 1.26.0
`))

var initApp_ = template.Must(template.New("app").Parse(`// Command {{.Name}} is an NSGo app.
package main

import (
	"fmt"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Root is the app's first screen.
func Root(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(24), PaddingEnv: core.EnvAll(core.EnvSafeArea), Gap: 16}, Children: []core.Node{
		core.Text{Content: "{{.Name}}", Style: core.Style{FontSize: 28, FontWeight: core.FontWeightBold}},
		core.Text{ContentFunc: func() string { return fmt.Sprintf("Pressed %d times", count.Get()) }},
		base.Button{OnPress: func(core.PressEvent) { count.Update(func(n int) int { return n + 1 }) }, Label: "Press me",
			Style: core.Style{Background: core.RGB(0x0969da), CornerRadius: 8, Padding: core.Symmetric(10, 16),
				AlignSelf: core.AlignStart, Active: &core.Style{Opacity: core.Alpha(0.8)}},
			Children: []core.Node{core.Text{Content: "Press me", Style: core.Style{Color: core.RGB(0xffffff)}}}},
	}}
}

func main() { nsgo.Run(Root) }
`))

var initTest = template.Must(template.New("test").Parse(`package main

import (
	"testing"

	"github.com/oneo-me/nsgo/nsgotest"
)

// TestMain gives the platform the main thread under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("role:button Press me")
	app.Find("text:Pressed 1 times")
}
`))

var initTSXMain = template.Must(template.New("main").Parse(`// Command {{.Name}} is an NSGo app written in TSX: App.tsx is its entry.
// nsgo dev and nsgo build compile it ahead; go run and go test compile it
// as it starts.
package main

import (
	"embed"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/tsx/compiler"
)

//go:embed *.tsx nsgo.yaml
var src embed.FS

// Root is the app: what App.tsx mounts.
var Root = compiler.App(src, "App.tsx")

func main() { nsgo.Run(Root) }
`))

var initTSX = template.Must(template.New("tsx").Parse(`import { mount, signal, Text, View } from "nsgo";

function App() {
  const count = signal(0);
  return (
    <View class="flex-1 gap-4 p-6">
      <Text class="text-3xl font-bold">{{.Name}}</Text>
      <Text>Pressed {count()} times</Text>
      <View
        role="button"
        accessibilityLabel="Press me"
        onPress={() => count.set(count() + 1)}
        class="self-start rounded-lg bg-blue-600 px-4 py-2 active:opacity-80"
      >
        <Text class="text-white">Press me</Text>
      </View>
    </View>
  );
}

mount(App);
`))

var initMixedTSX = template.Must(template.New("mixed").Parse(`import { mount, signal, Text, View } from "nsgo";
// What native.go registers: a component, a function and a class in Go.
import { Badge, greet, Tally } from "{{.Slug}}/native";

function App() {
  const tally = new Tally();
  const count = signal(tally.count);
  return (
    <View class="flex-1 gap-4 p-6">
      <Text class="text-3xl font-bold">{greet("{{.Name}}")}</Text>
      <Badge text={` + "`" + `Pressed ${count()} times` + "`" + `} />
      <View
        role="button"
        accessibilityLabel="Press me"
        onPress={() => count.set(tally.add(1))}
        class="self-start rounded-lg bg-blue-600 px-4 py-2 active:opacity-80"
      >
        <Text class="text-white">Press me</Text>
      </View>
    </View>
  );
}

mount(App);
`))

var initNative = template.Must(template.New("native").Parse(`package main

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx"
)

// Badge is a Go component App.tsx renders: its fields are its props, in
// lower camel case (Text is text).
type Badge struct {
	Text string
}

func (b Badge) Render(*core.Scope) core.Node {
	return core.Text{Content: b.Text, Class: "self-start rounded-full bg-zinc-100 px-3 py-1 text-sm"}
}

// Tally is a Go class App.tsx makes with new: its methods are the
// class's, and Count, a getter, its count property.
type Tally struct{ count int }

func (t *Tally) Add(by int) int { t.count += by; return t.count }
func (t *Tally) Count() int     { return t.count }

func init() {
	tsx.Register("{{.Slug}}/native", map[string]core.Node{"Badge": Badge{}})
	tsx.RegisterFunctions("{{.Slug}}/native", map[string]any{
		"greet": func(name string) string { return "Hello from Go, " + name },
	})
	tsx.RegisterClass("{{.Slug}}/native", "Tally", tsx.Class[Tally]{
		New:     func() *Tally { return &Tally{} },
		Getters: []string{"Count"},
	})
}
`))

var initTSXTest = template.Must(template.New("tsxtest").Parse(`// The app's TSX tests: nsgo test runs them headless, or on a platform
// with -target.
import { expect, open, test } from "nsgo/test";

test("counts presses", async () => {
  const app = await open("App.tsx");
  await app.press("role:button[Press me]");
  expect(app.shows("Pressed 1 times")).toBe(true);
});
`))

var initTSConfig = template.Must(template.New("tsconfig").Parse(`{
  "extends": "./.nsgo/tsconfig.json",
  "include": ["**/*.ts", "**/*.tsx"]
}
`))

var initIgnore = template.Must(template.New("ignore").Parse(`.nsgo/
dist/
`))
