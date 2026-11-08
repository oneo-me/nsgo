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

`test` 每个测试输出一行。`-run` 像 `go test` 一样筛选测试，`-junit report.xml` 写出 JUnit 报告，`-bench` 改为运行基准测试。

## 在平台上运行

同一组测试可以在真实窗口中、用平台自己的输入运行：

```sh
go tool nsgo test -target macos
```

可用的目标有 `macos`、`windows`、`linux`、`web`（在 Chrome、Chromium 或 Edge 中）、`ios_simulator` 和 `android`。每个测试在独立的进程中运行，`app.Screenshot("name")` 会把截图保存到 `.nsgo/test/screenshots`。

## TSX 测试

TSX 应用中用 `nsgo/test` 编写的 `*.test.tsx` 文件会随 Go 测试一起运行。
