---
title: TSX 应用
description: 用 TSX 和 Tailwind 类名编写应用，提前编译，不需要 Node。
group: 指南
order: 20
---

## TSX 应用

TSX 应用是一个嵌入自身源码并运行入口的 Go 包：

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

`go tool nsgo init -template tsx` 会写出这样一个应用，`examples/tsx` 也是一个例子。`nsgo dev` 和 `nsgo build` 会提前编译 TSX，所以发布的包里不带编译器；`go run` 和 `go test` 则在应用启动时编译。两种方式都不需要 Node。

## 组件与模块

TSX 从 `nsgo` 导入基本元素，从 `nsgo/base` 导入 `base` 的组件；应用导入 `tsx/ui` 并设置 `ui.DefaultTheme` 后，还能从 `nsgo/ui` 导入 `ui` 的组件；`examples/gallery-tsx` 展示了这些组件。

## 在 TSX 中使用 Go

注册 Go 代码供 TSX 导入：

```go
tsx.Register("app/native", map[string]core.Node{"Badge": Badge{}})
tsx.RegisterFunctions("app/native", map[string]any{
	"greet": func(name string) string { return "Hello, " + name },
})
```

之后 `import { Badge, greet } from "app/native"` 就会把 `Badge` 的字段当作属性，并能调用 `greet`。接收 `context.Context` 的函数会变成 promise，`tsx.RegisterClass` 则把 Go 类型变成 TSX 可以用 `new` 创建的类。

## 编辑器与测试

继承 `.nsgo/tsconfig.json` 的 `tsconfig.json` 为编辑器提供 TSX 的类型，这些类型由 `go tool nsgo dev` 和 `go tool nsgo check` 写出。应用旁边用 `nsgo/test` 编写的 `*.test.tsx` 文件会随 Go 测试一起运行（[测试](testing.md)）。
