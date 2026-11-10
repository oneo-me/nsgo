---
title: Words, tests and publishing
description: Translate what a component says, test it as a user would, and offer it to Go and TSX apps.
group: Writing components
order: 74
---

## Words

A component says its own words, such as a button's name for screen readers, through its package's catalogs rather than string literals. Put a `locales/` package in the library with the source language's catalog, and a file that generates the code:

```yaml
# locales/en.yaml
clear:
  message: Clear
  description: What screen readers say of the button that empties a chip input.
```

```go
// Package locales says the chips' words in the language the app shows.
package locales

//go:generate go tool nsgo i18n
```

`go generate ./locales` writes a function per message, which the component calls with its scope: `locales.Clear(cx)` says it in the language the app shows. Commit the generated `catalog_gen.go`, so apps build with Go alone. Add a file per language you translate to (`zh-Hans.yaml`); an app supplies others in its own catalogs, under a key naming your package's import path, as for [NSGo's own words](../localization.md#nsgos-own-words).

## Tests

`nsgotest` runs a component the way a user uses it, headless. Mount it in a small root and act on it by role and text:

```go
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestToggle(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		on := core.NewSignal(cx, false)
		return chips.Toggle{On: on.Get, OnChange: on.Set, Label: "Bold"}
	})
	app.Press("role:switch Bold")
	if !app.Find("role:switch Bold").Props.Checked {
		t.Fatal("the toggle did not turn on")
	}
}
```

From the library's root, `go tool nsgo test` takes every package, and `-target macos` or another target runs the same tests in a native window. Test both appearances and a pseudo locale (`-lang en-XA`) to catch hard-coded colors and words. [Testing](../testing.md) covers the rest.

## Publishing

A component library is a Go module: apps `go get` it and import its packages. Keep its unstyled and styled forms in separate packages when both exist, so an app with a look of its own imports no styles it does not use.

The site's [Components](https://oneo-me.github.io/nsgo/components/) page lists libraries others can use; add a document and a screenshot to `docs/public/components/` to offer yours.

TSX apps reach Go components once they are registered, in an `init` function:

```go
func init() {
	tsx.Register("example.com/chips", map[string]core.Node{"Toggle": Toggle{}})
}
```

`import { Toggle } from "example.com/chips"` then takes the fields as props ([TSX apps](../tsx.md#go-from-tsx)).
