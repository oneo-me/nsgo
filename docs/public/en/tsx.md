---
title: TSX apps
description: Write an app in TSX with Tailwind classes, compiled ahead with no Node.
group: Guides
order: 20
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

`go tool nsgo init -template tsx` writes one. `nsgo dev` and `nsgo build` compile the TSX ahead of time, so packages ship no compiler and no Node is needed.

## Components and modules

TSX imports the primitives from `nsgo`, `base`'s components from `nsgo/base` and `ui`'s from `nsgo/ui`, once the app imports `tsx/ui` and sets `ui.DefaultTheme`.

## Go from TSX

Register Go for TSX to import:

```go
tsx.Register("app/native", map[string]core.Node{"Badge": Badge{}})
tsx.RegisterFunctions("app/native", map[string]any{
	"greet": func(name string) string { return "Hello, " + name },
})
```

`import { Badge, greet } from "app/native"` then takes `Badge`'s fields as props and calls `greet`. Functions taking a `context.Context` become promises, and `tsx.RegisterClass` makes Go types classes TSX creates with `new`.

## Editors and tests

A `tsconfig.json` extending `.nsgo/tsconfig.json` gives editors the TSX's types, which `go tool nsgo dev` and `go tool nsgo check` write. `*.test.tsx` files beside the app, written with `nsgo/test`, run with its Go tests ([Testing](testing.md)).
