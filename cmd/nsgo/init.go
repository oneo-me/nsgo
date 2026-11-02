package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// initApp writes a new app into dir: nsgo.yaml, a main package with a
// component and a headless test of it.
func initApp(dir, name, module string) error {
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
	data := struct{ Name, ID, Module string }{name, id, module}
	files := map[string]*template.Template{"nsgo.yaml": initYAML, "app.go": initApp_, "app_test.go": initTest}
	if module != "" {
		files["go.mod"] = initMod
	}
	for file, t := range files {
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
	return nil
}

var initYAML = template.Must(template.New("yaml").Parse(`name: {{.Name}}
id: {{.ID}}
version: 0.1.0

window:
  width: 420
  height: 640

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
