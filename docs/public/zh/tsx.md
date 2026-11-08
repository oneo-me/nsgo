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

## 签名包

应用可以运行它下载的 TSX，例如重新下发的自身页面或插件，前提是先验证包。根密钥离线保存，用它为发布密钥签发证书，然后在带 `nsgo.yaml` 的目录中签名应用：

```sh
go tool nsgo keygen root
go tool nsgo keygen release
go tool nsgo certify -root root.key -key release.pub -apps 'com.example.*'
go tool nsgo publish -key release.key -cert release.cert plugin
go tool nsgo verify -root root.pub plugin/dist/Plugin-1.0.0.nsgopkg
```

应用信任 `root.pub`，在引擎协程之外验证包，再用 `tsx.View` 在沙箱中把它显示在自己的视图旁：

```go
trust, err := pkg.OpenTrust(filepath.Join(cx.DataDir(), "trust"))
trust.AddRoot(root)
p, err := trust.Verify(data) // 拒绝被篡改、已吊销和更旧的构建
core.Catch{Try: tsx.View{Package: p}.Render, Fallback: failed}
trust.Accept(p)              // 运行之后，更旧的构建会被拒绝
```

`go tool nsgo revoke` 签署吊销发布密钥和包的列表，供 `Trust.UpdateRevocations` 使用。`examples/plugins` 就是这样的应用。

## 编辑器与测试

继承 `.nsgo/tsconfig.json` 的 `tsconfig.json` 为编辑器提供 TSX 的类型，这些类型由 `go tool nsgo dev` 和 `go tool nsgo check` 写出。应用旁边用 `nsgo/test` 编写的 `*.test.tsx` 文件会随 Go 测试一起运行（[测试](testing.md)）。
