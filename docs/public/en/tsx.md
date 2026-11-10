---
title: TSX apps
description: Write an app in TSX with Tailwind classes, compiled ahead with no Node.
group: Core concepts
order: 63
---

## A TSX app

A TSX app is a Go package that embeds its sources and runs its entry:

```go
//go:embed *.tsx nsgo.yaml
var src embed.FS

var Root = compiler.App(src, "App.tsx")

func main() { nsgo.Run(Root) }
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

`go tool nsgo init -template tsx` writes one, and `examples/tsx` is another. `nsgo dev` and `nsgo build` compile the TSX ahead of time, so packages ship no compiler; `go run` and `go test` compile it as the app starts. No Node is needed either way.

## Components and modules

TSX imports the primitives from `nsgo`, `base`'s components from `nsgo/base` and `ui`'s from `nsgo/ui`, once the app imports `tsx/ui` and sets `ui.DefaultTheme`; `examples/gallery-tsx` shows them.

## Go from TSX

Register Go for TSX to import:

```go
tsx.Register("app/native", map[string]core.Node{"Badge": Badge{}})
tsx.RegisterFunctions("app/native", map[string]any{
	"greet": func(name string) string { return "Hello, " + name },
})
```

`import { Badge, greet } from "app/native"` then takes `Badge`'s fields as props and calls `greet`. Functions taking a `context.Context` become promises, and `tsx.RegisterClass` makes Go types classes TSX creates with `new`.

## Signed packages

An app can run TSX it downloads, such as its own screens delivered anew or plugins, once it verifies the package. Declare what the package's code may reach in its `nsgo.yaml`; it is refused everything else:

```yaml
capabilities:
  network:
    hosts: [api.example.com, "*.cdn.example.com"]
  open_url: true # or { schemes: [https] }
  host: [search, "player.*"] # the app's handlers it may call
  # webview: { hosts: [...], input: true }, clipboard, picker, speech, status_bar
```

Keep a root key offline and certify release keys with it, then sign the app in that directory:

```sh
go tool nsgo keygen root
go tool nsgo keygen release
go tool nsgo certify -root root.key -key release.pub -apps 'com.example.*'
go tool nsgo publish -key release.key -cert release.cert plugin
go tool nsgo verify -root root.pub plugin/dist/Plugin-1.0.0.nsgopkg
```

The app trusts `root.pub`, verifies the package off the engine's goroutine and shows it in `tsx.View`, sandboxed beside its own views. The package reaches the app only through the host channel the view gives it, and keeps its own part of the app's storage:

```go
trust, err := pkg.OpenTrust(filepath.Join(cx.DataDir(), "trust"))
trust.AddRoot(root)
p, err := trust.Verify(data) // refuses tampered, revoked and older builds
channel := core.NewHost()
channel.Handle("search", func(c *core.HostCall) { c.Resolve(results(c.Data)) })
view := tsx.View{Package: p, Sandbox: tsx.Sandbox{Host: channel}}
core.Catch{Try: view.Render, Fallback: failed}
trust.Accept(p) // once it ran, older builds are refused
```

`Sandbox.Grant` gives it less than it declares, `Sandbox.Limits` other time budgets and quotas, and `Sandbox.Modules` Go functions and components beyond NSGo's own. A package whose entry exports functions is a plugin of functions: `tsx.Load(cx, p, tsx.Sandbox{})` runs it unseen and `Plugin.Call` calls them. `go tool nsgo dev -limits` runs an app as such a view would, and `go tool nsgo revoke` signs a list of revoked release keys and packages for `Trust.UpdateRevocations`. `examples/plugins` asks the user to grant a package's capabilities, installs it and runs it.

## Editors and tests

A `tsconfig.json` extending `.nsgo/tsconfig.json` gives editors the TSX's types, which `go tool nsgo dev` and `go tool nsgo check` write. `*.test.tsx` files beside the app, written with `nsgo/test`, run with its Go tests ([Testing](testing.md)).
