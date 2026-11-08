---
title: Testing
description: Test an app the way a user uses it, headless or in a native window.
group: Guides
order: 30
---

## A test

`nsgotest` starts the app without a window. A test finds controls by their text or role and acts on them as a user would:

```go
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("text:Press me")
	app.Find("text:Pressed 1 times")
}
```

`TestMain` gives the platform the main thread when the tests run on a target.

## Run the tests

```sh
go tool nsgo test
```

`test` prints a line per test. `-run` picks tests as `go test`'s does, `-junit report.xml` writes a JUnit report and `-bench` runs benchmarks instead.

## Run on a platform

The same tests run in a real window with the platform's own input:

```sh
go tool nsgo test -target macos
```

The targets are `macos`, `windows`, `linux`, `web` (in Chrome, Chromium or Edge), `ios_simulator` and `android`. Each test runs in a process of its own, and `app.Screenshot("name")` saves screenshots in `.nsgo/test/screenshots`.

## TSX tests

A TSX app's `*.test.tsx` files, written with `nsgo/test`, run with its Go tests.
