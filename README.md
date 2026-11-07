<p align="center">
  <img src="examples/gallery/assets/Icon.png" alt="NSGo" width="128">
</p>

<h1 align="center">NSGo</h1>

<p align="center">
  <strong>English</strong> |
  <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <a href="https://github.com/oneo-me"><img alt="GitHub" src="https://img.shields.io/badge/GitHub-oneo--me-181717?logo=github&logoColor=white"></a>
  <a href="https://oneo.me"><img alt="Blog" src="https://img.shields.io/badge/Blog-oneo.me-0969da"></a>
  <a href="https://x.com/oneo666"><img alt="X" src="https://img.shields.io/badge/X-%40oneo666-000000?logo=x&logoColor=white"></a>
  <a href="https://github.com/sponsors/oneo-me"><img alt="GitHub Sponsors" src="https://img.shields.io/badge/GitHub%20Sponsors-ea4aaa?logo=githubsponsors&logoColor=white"></a>
</p>

NSGo is a cross-platform native UI engine with a Go backend. App logic, the reactive graph and layout run in Go, and each platform draws with its own native UI.

NSGo follows the design of [Native Shell (NSH)](https://nsh.dev), which does the same in Rust. It exists for teams that want NSH's model without writing Rust.

<p align="center">
  <img src="examples/gallery/assets/screenshot.jpg" alt="The gallery's Stocks demo on macOS">
</p>

## Status

NSGo is early. It has NSH's reactive model, layout, text, images, canvas, shaders, animations, services, unstyled base components and styled `ui` components on macOS, iOS, Windows, Linux, the web and Android, and apps embed in iOS, Android and Go desktop apps of their own. There is no release or stable API yet. [ARCHITECTURE.md](ARCHITECTURE.md#platforms) tracks each target.

## Goals

- Write apps and components in Go, with signals, memos and effects.
- Lay out with flexbox and grid, and render with each platform's own views and text.
- Target iOS, Android, macOS, Windows, Linux and the web, as NSH does.
- Embed NSGo apps in existing native apps, or package them as standalone apps.

## Usage

An app is a Go main package that calls `nsgo.Run`. Components are functions or types that return elements; signals update what reads them:

```go
package main

import (
	"fmt"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

func Root(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	return core.View{Style: core.Style{Padding: core.All(24), Gap: 12}, Children: []core.Node{
		core.Text{ContentFunc: func() string { return fmt.Sprintf("Pressed %d times", count.Get()) }},
		base.Button{
			Label:    "Press me",
			OnPress:  func(core.PressEvent) { count.Update(func(n int) int { return n + 1 }) },
			Style:    core.Style{Padding: core.Symmetric(6, 14), CornerRadius: 8, Background: core.RGB(0x007aff), Color: core.RGB(0xffffff)},
			Children: []core.Node{core.Text{Content: "Press me"}},
		},
	}}
}

func main() { nsgo.Run(Root, nsgo.WindowSize(420, 560)) }
```

A desktop app can open what it shows itself instead, as an Avalonia application does: `nsgo.Run(App)` runs `func App(app *core.Scope)` once, and it opens windows, panels, tray icons and global shortcuts with `desktop`, which share its signals (on macOS for now). `examples/tray` keeps out of the Dock and lives in the menu bar.

Test it headless with `nsgotest`, no window needed; `go tool nsgo test -target macos` runs the same tests in a real window, with native input and screenshots (`app.Screenshot("name")`):

```go
func TestMain(m *testing.M) { nsgotest.Main(m) } // gives the platform the main thread on -target

func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("text:Press me")
	app.Find("text:Pressed 1 times")
}
```

An app can also be written in TSX, as an NSH app is with its imports naming `nsgo`, and styled with Tailwind CSS classes: the package embeds its sources, which the tool compiles ahead into its builds, and `go run` and `go test` compile as the app starts, with no Node. `examples/tsx` is one.

```go
//go:embed *.tsx
var src embed.FS

func main() { nsgo.Run(compiler.App(src, "App.tsx")) }
```

```tsx
import { mount, signal, Text, View } from "nsgo";

function App() {
  const count = signal(0);
  return (
    <View class="gap-3 p-6" onPress={() => count.update((n) => n + 1)}>
      <Text class="text-lg font-bold">Pressed {count()} times</Text>
    </View>
  );
}

mount(App);
```

TSX imports `base`'s components from `nsgo/base`, and an app's own Go components once it registers them: `tsx.Register("app/native", map[string]core.Node{"Badge": Badge{}})` makes `import { Badge } from "app/native"` take `Badge`'s fields as props; `tsx.RegisterFunctions` exports Go functions, those taking a `context.Context` as promises, and `tsx.RegisterClass` Go types TSX makes with `new`. A `tsconfig.json` extending `.nsgo/tsconfig.json` lets `go tool nsgo check` vet the app and type-check its TSX, and `*.test.tsx` files beside it, written with `nsgo/test`, run with its Go tests.

Beside the package, `nsgo.yaml` names the app and its targets:

```yaml
name: Hello
id: dev.oneo.nsgo.hello
version: 0.1.0
build_number: 1         # CFBundleVersion and Android's versionCode; else from version
# appearance: dark      # light or dark whatever the system's
# theme: theme.json     # a TSX app's theme: NSGo's or a shadcn/ui one
# i18n: { dir: locales, source: en }  # the app's catalogs and the language they are written in
icon:
  apple: assets/AppIcon.icon  # macOS and iOS, or an .icns for macOS
  png: assets/Icon.png        # every other target
dev:
  target: native        # native, web, ios, ios_simulator or android
  hot_reload: true      # false builds once, watching no files
  fps: false            # true shows the performance monitor (macOS, iOS, Windows, Linux); -fps overrides it
  # icon: { png: assets/Icon-dev.png }  # replaces icon's files in dev's builds; defaults to assets/*-dev.*
build:
  output: dist          # packages go to dist/<name>-<version>-<target>
  open: false           # true opens the output directory after building
  fps: false            # as dev.fps, for packages
  targets: [macos_arm64, macos_x64, windows_arm64, windows_x64, linux_arm64, linux_x64, ios, ios_simulator, android_arm64, android_x64, web]
# android: { permissions: [REQUEST_INSTALL_PACKAGES] }  # manifest permissions beyond the runtime's
# sign:                 # packages; passwords come from NSGO_KEYSTORE_PASSWORD and NSGO_KEY_PASSWORD
#   identity: "Apple Development: you@example.com (ABCDE12345)"  # signs the macOS app, else ad hoc
#   profile: assets/AppStore.mobileprovision  # signs the IPA
#   keystore: assets/release.p12              # signs APKs, with key_alias
```

Beside `nsgo.yaml`, `resources/` holds files the app ships, where `asset:` images load from and `nsgo.Fonts("fonts/Brand.ttf")` registers fonts from; `assets/` holds project files that do not ship, such as the icons.

`locales/` holds the app's messages, a YAML file per language (`en.yaml`, `de.yaml`) in ICU MessageFormat, which the package imports. `dev`, `build` and `test` check them and write a Go function per message beside them, such as `locales.Greeting(cx, "Ada")`, which says it in the language the app shows and follows it as it changes; `cx.SetLocale("de")` chooses one, and the layout runs in the direction it is written in; `nsgo dev -lang en-XA` runs the app in a pseudo locale that shows untranslated and cut text. Messages format numbers and dates (`{d, date, medium}`), and `appName` names the app in each language in its packages. `go tool nsgo i18n` writes the functions alone. A key that names a package, such as `github.com/oneo-me/nsgo/base/locales`, gives that package's messages in the file's language, for languages NSGo is not translated to.

Add the tool to your module once; `go tool nsgo` then runs it without installing:

```sh
go get -tool github.com/oneo-me/nsgo/cmd/nsgo
```

`go tool nsgo init` writes a new app: a Go app, or with `-template tsx` a TSX app and with `-template mixed` a Go app whose TSX entry imports a Go component, function and class of its own.

`go tool nsgo dev` runs the app and rebuilds and restarts it on each change, or with `dev.hot_reload: false` runs it once; a change of TSX alone updates the running app in place, keeping top-level signals, navigation stacks and the state of the components it did not change. A failed build shows over the running app; Command or Control+R in the app builds it again. `-trace` prints each of the running app's commits, its time and mount items, on every target.

`go tool nsgo build` writes a stripped release package per target: an `.app`, an `.ipa`, an `.apk`, Windows' and Linux's directories, or a web site, whose page fetches each TSX module an `import()` names when the import runs. `-run` starts the one built for this machine, or installs it on an iPhone, simulator or Android device, which `-device` picks.

`-embed` packages the app for an iOS or Android app of your own instead: an XCFramework (of dynamic frameworks with `-dynamic`) or an AAR whose `NSGoView` or `NsgoView` shows it. Go desktop programs show apps with `nsgo.NewView`, and web pages with the site's `nsgo.js`. The host is the page's, or the module `nsgo.yaml`'s `web.host` names. See `examples/embed-*`.

`go tool nsgo test` runs the app's tests, its `*.test.tsx` among them, with a line per test; `-run` filters them, `-junit report.xml` writes a JUnit report and `-trace` prints each step's mount items and commit times. `-target` runs them on `macos`, `windows`, `linux`, `web` (in Chrome, Chromium or Edge), `ios_simulator` or `android`, a process per test, saving screenshots in `.nsgo/test/screenshots`, and with `-fps` logs the performance monitor's readings each second on `macos`, `windows`, `linux` and `ios_simulator`. Given a TSX entry, `go tool nsgo test App.tsx < steps.ts` opens the app and runs the steps standard input holds, as a `*.test.tsx` test's body with `app` the app. `build.targets` and `-target` accept `all` for every platform that needs only Go and Xcode; add `android_x64` when an NDK is installed.

Flags replace the `nsgo.yaml` values they name, such as `-target`, `-version` or `-output`; `dev` and `build` take `-device` for an iPhone, simulator or Android device, `build` `-team` to sign the iPhone's package, and `dev` `-port` for its server. `-h` lists every flag.

On macOS, a scroll view's edge effects (`Scroll.EdgeEffects`) blur through Core Animation's private `CABackdropLayer` and `CAFilter`, as AppKit's own edge effects do, since no public API varies a blur across a view. Mac App Store review may reject apps that use private APIs: NSGo calls them only for scroll views with edge bars, but every macOS build contains their names.

## Development

Requirements: macOS with Xcode and Go 1.26 or later. Nothing else is needed to build every target for arm64, Android's APK included.

Android device and emulator development additionally needs `adb` from the Android SDK's platform-tools, and iOS device development needs an Apple ID in Xcode's Accounts, a trusted iPhone and Developer Mode on it.

The x86_64 emulator target, `android_x64`, also needs an Android NDK: Go links android/amd64 only with cgo. On Windows, Go 1.26 or later runs the native app and builds the Windows and web packages. On Linux, Go runs the native app once GTK 4.14 or newer, Pango and Cairo are installed, WebKitGTK 6.0 for web views and GStreamer plug-ins for videos.

Run the gallery from the repository root: NSH's gallery in Go, a demo of each element, `base` and `ui` component and service. A wide window shows the demos under tabs in a sidebar; a narrow one shows the tabs and the settings in a bottom bar, with demos pushed over them.

```sh
go tool nsgo dev examples/gallery                   # nsgo.yaml's dev target, exiting with the app
go tool nsgo dev -target web examples/gallery       # in the browser, serving until interrupted
go tool nsgo dev -target ios_simulator examples/gallery
go tool nsgo dev -target ios examples/gallery       # on the connected iPhone, signed by Xcode
go tool nsgo dev -target android examples/gallery   # on the connected device or emulator, over adb
go tool nsgo build examples/gallery                 # release packages in examples/gallery/dist
go tool nsgo build -target all examples/gallery     # every platform's release packages
go test ./...
bench/run.sh                                       # measure the quotes benchmark (bench/README.md)
```

NSGo uses NSH as its reference implementation, and aligns with its releases. Clone it beside the repository, as `../nsh`, and check out its latest release tag, never `main`:

```sh
git clone https://github.com/longbridge/nsh.git ../nsh
git -C ../nsh checkout "$(git -C ../nsh describe --tags --abbrev=0)"
```

`go test ./tsx` runs NSH's apps and compat cases from that checkout, or from `$NSH_DIR` when set, and skips them without one.

Read [ARCHITECTURE.md](ARCHITECTURE.md) and [AGENTS.md](AGENTS.md) before changing code.

The website at <https://oneo-me.github.io/nsgo/> is an Astro site with a React home page and a Markdown docs section; its Gallery links open the gallery's WebAssembly build in a new tab. Run it from `website/` with `bun install` and `bun run dev` at <http://localhost:4321/nsgo/>, or build it with `bun run build`; the [Website workflow](.github/workflows/website.yml) publishes `website/dist` to GitHub Pages. The website needs [Bun](https://bun.com) 1.4 or later.

## License

Licensed under the [Apache License, Version 2.0](LICENSE). NSGo is a port of NSH, and [NOTICE](NOTICE) records the attribution that port owes.

Unless you explicitly state otherwise, any contribution intentionally submitted for inclusion in NSGo by you, as defined in the Apache-2.0 license, shall be licensed under Apache-2.0, without any additional terms or conditions.
