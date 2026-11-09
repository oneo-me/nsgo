---
title: 测试
description: 像用户一样测试应用，可以无窗口运行，也可以在原生窗口中运行。
group: 指南
order: 30
---

## 一个测试

`nsgotest` 无需窗口即可启动应用。测试按文本或角色找到控件，并像用户一样操作它们：

```go
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("text:Press me")
	app.Find("text:Pressed 1 times")
}
```

当测试在某个目标上运行时，`TestMain` 会把主线程交给平台。

## 运行测试

```sh
go tool nsgo test
```

`test` 每个测试输出一行。`-run` 像 `go test` 一样筛选测试，`-junit report.xml` 写出 JUnit 报告，`-trace` 打印每一步的挂载项和提交耗时，`-bench` 改为运行基准测试，每个结果输出一行。`--` 之后的参数交给 `go test`。

传入没有 `nsgo.yaml` 的目录（例如库或模块根目录）时，`test` 会处理其下的所有包；目录也可以像 Go 的包模式那样以 `/...` 结尾。`go tool nsgo check` 以同样的方式选取并检查包。

## 在平台上运行

同一组测试可以在真实窗口中、用平台自己的输入运行：

```sh
go tool nsgo test -target macos
```

可用的目标有 `macos`、`windows`、`linux`、`web`（在 Chrome、Chromium 或 Edge 中）、`ios_simulator`、`tvos_simulator`（方向键、作为选择键的 Return 和作为 Menu 键的 Escape 模拟遥控器）和 `android`，或 `all`。每个测试在独立的进程中运行，`app.Screenshot("name")` 会把截图保存到 `.nsgo/test/screenshots`，`-fps` 会在 `macos`、`windows`、`linux` 和 `ios_simulator` 上每秒记录一次性能监视器的读数。`-env KEY=VALUE` 为测试设置环境变量，手机上的测试只能这样拿到变量。

要测量而不是检查时，用 `nsgotest.Measure()` 启动：应用运行在系统时钟上，`Advance` 会真实等待相应时长，各步骤只等待正在进行的动画停下。`app.Mark("name")` 标出此后的读数测的是什么，`app.Mark("-")` 结束标记；无论是否加 `-fps`，标记都会打印出来。

## TSX 测试

TSX 应用中用 `nsgo/test` 编写的 `*.test.tsx` 文件会随 Go 测试一起运行。

不写测试文件也能驱动 TSX 应用：传入它的入口，并从标准输入提供步骤，写法与 `*.test.tsx` 测试的函数体相同，`app` 即该应用：

```sh
go tool nsgo test App.tsx < steps.ts
```
