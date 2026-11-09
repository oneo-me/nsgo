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

`test` prints a line per test. `-run` picks tests as `go test`'s does, `-junit report.xml` writes a JUnit report, `-trace` prints each step's mount items and commit times and `-bench` runs benchmarks instead, a line per result. Flags after `--` go to `go test`.

Given a directory without `nsgo.yaml`, such as a library or the module's root, `test` takes every package under it; a directory may also end in `/...`, as Go's patterns do. `go tool nsgo check` takes packages the same way and vets them.

## Run on a platform

The same tests run in a real window with the platform's own input:

```sh
go tool nsgo test -target macos
```

The targets are `macos`, `windows`, `linux`, `web` (in Chrome, Chromium or Edge), `ios_simulator`, `tvos_simulator` (the remote's arrows, Return as its select button and Escape as Menu) and `android`, or `all`. Each test runs in a process of its own, `app.Screenshot("name")` saves screenshots in `.nsgo/test/screenshots`, and `-fps` logs the performance monitor's readings each second on `macos`, `windows`, `linux` and `ios_simulator`. `-env KEY=VALUE` sets a variable of the tests' environment, which a phone's tests see no other way.

To measure rather than check, start with `nsgotest.Measure()`: the app runs on the system's clock, `Advance` waits as long as it moves, and steps wait only for what animates to rest. `app.Mark("name")` names what the readings from there on measure, and `app.Mark("-")` ends it; the marks print with or without `-fps`.

## TSX tests

A TSX app's `*.test.tsx` files, written with `nsgo/test`, run with its Go tests.

To script a TSX app without a test file, pass its entry and the steps on standard input, written as a `*.test.tsx` test's body with `app` the app:

```sh
go tool nsgo test App.tsx < steps.ts
```
