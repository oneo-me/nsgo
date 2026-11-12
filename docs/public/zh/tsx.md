---
title: TSX 应用
description: 用 TSX 和 Tailwind 类名编写应用，提前编译，不需要 Node。
group: 核心概念
order: 63
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

TSX 从 `nsgo` 导入基本元素，从 `nsgo/base` 导入 `base` 的组件；应用导入 `tsx/ui` 后，还能从 `nsgo/ui` 导入 `ui` 的组件；`examples/gallery-tsx` 展示了这些组件。

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

应用可以运行它下载的 TSX，例如重新下发的自身页面或插件，前提是先验证包。在包的 `nsgo.yaml` 中声明它的代码可以访问什么，其余一律拒绝：

```yaml
capabilities:
  network:
    hosts: [api.example.com, "*.cdn.example.com"]
  open_url: true # 或 { schemes: [https] }
  host: [search, "player.*"] # 它可以调用的应用处理函数
  # webview: { hosts: [...], input: true }、clipboard、picker、speech、status_bar
```

根密钥离线保存，用它为发布密钥签发证书，然后在该目录中签名应用。`go tool nsgo keygen` 打印一对密钥，不写任何文件；根密钥和每个发布密钥各运行一次。公钥写在参数和应用代码里；私钥不要放进仓库，只通过 `NSGO_ROOT_KEY` 和 `NSGO_RELEASE_KEY` 交给工具：

```sh
go tool nsgo keygen # 根密钥一次，发布密钥一次
NSGO_ROOT_KEY=<根私钥> go tool nsgo certify -key <发布公钥> -apps 'com.example.*'
NSGO_RELEASE_KEY=<发布私钥> go tool nsgo publish -cert release.cert plugin
go tool nsgo verify -root <根公钥> plugin/dist/Plugin-1.0.0.nsgopkg
```

应用信任写在代码里的根公钥，在引擎协程之外验证包，再用 `tsx.View` 在沙箱中把它显示在自己的视图旁。包只能通过视图给它的 host 通道访问应用，并拥有应用存储中属于自己的一部分：

```go
trust, err := pkg.OpenTrust(filepath.Join(cx.DataDir(), "trust"))
trust.AddRoot(root)
p, err := trust.Verify(data) // 拒绝被篡改、已吊销和更旧的构建
channel := core.NewHost()
channel.Handle("search", func(c *core.HostCall) { c.Resolve(results(c.Data)) })
view := tsx.View{Package: p, Sandbox: tsx.Sandbox{Host: channel}}
core.Catch{Try: view.Render, Fallback: failed}
trust.Accept(p) // 运行之后，更旧的构建会被拒绝
```

`Sandbox.Grant` 让它得到比声明更少的权限，`Sandbox.Limits` 设置其他时间预算和配额，`Sandbox.Modules` 给它 NSGo 自带之外的 Go 函数和组件。入口导出函数的包是函数型插件：`tsx.Load(cx, p, tsx.Sandbox{})` 在后台运行它，`Plugin.Call` 调用这些函数。`go tool nsgo dev -limits` 像这样的视图一样运行应用，`go tool nsgo revoke` 签署吊销发布密钥和包的列表，供 `Trust.UpdateRevocations` 使用。`examples/plugins` 会请用户授予包声明的权限，再安装并运行它。

## 编辑器与测试

继承 `.nsgo/tsconfig.json` 的 `tsconfig.json` 为编辑器提供 TSX 的类型，这些类型由 `go tool nsgo dev` 和 `go tool nsgo check` 写出。应用旁边用 `nsgo/test` 编写的 `*.test.tsx` 文件会随 Go 测试一起运行（[测试](testing.md)）。
