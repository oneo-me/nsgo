# Languages

An app says its messages, and `base`'s components theirs, in the language it shows. Each package owns its messages, written as catalogs in YAML; the `nsgo` tool checks them and compiles them to Go, and package `i18n` runs that code. The design follows MyGo's (`docs/i18n.md` in MyGo), with YAML for its JSON and static Go for its embedded catalogs. NSH has none: its apps pick their strings from `locale()` themselves. Read this before changing `i18n`, a package's `locales/` or the tool's `i18n*.go`.

## Catalogs

A package of catalogs is a directory, named `locales` by convention, holding a YAML file per language named by its canonical BCP 47 tag (`en.yaml`, `zh-Hans.yaml`, `pt-BR.yaml`). The source language's file has every message, and may describe it and type its placeholders; translations give a key's message alone:

```yaml
# en.yaml
filesSelected:
  message: "{count, plural, =0 {No files selected} one {# file selected} other {# files selected}}"
  description: The status bar, while files are selected.
greeting: "Hello, {name}!"
ratio:
  message: "Ratio {r}"
  placeholders:
    r: { type: double }   # String, int, double, num or Object
```

```yaml
# de.yaml
filesSelected: "{count, plural, =0 {Keine Dateien ausgewählt} one {# Datei ausgewählt} other {# Dateien ausgewählt}}"
greeting: "Hallo, {name}!"
github.com/oneo-me/nsgo/base/locales:   # supplies base's messages in German
  sortBy: "Nach {column} sortieren"
```

Messages are ICU MessageFormat: `{arg}`, `{n, number}` with `integer` or `percent`, `plural` with `=N`, `offset:` and `#`, `selectordinal` and `select`, and apostrophes quoting braces. A parameter without a type is a `float64` when shown as a percentage, an `int` when counted or shown as a number, else a `string`; parameters come in the order of `placeholders`, then of the message.

A key that is an import path supplies that package's catalog in the file's language, under its keys, with its parameters. Apps supply languages NSGo does not translate to, or replace its words in one it does.

NSGo's own catalogs are `base/locales` and `base/textview/locales`, in English, Simplified and Traditional Chinese. A component in another package gives that package catalogs of its own.

The gallery (`examples/gallery/locales`) says its interface in English, Simplified and Traditional Chinese and Arabic, and supplies Arabic to `base`'s catalogs. What a demo shows as content stays as written: code, documents, conversations and names. A message keeps an English ID where code or tests key on it, as a demo's `name`, and says its `title` through a method expression (`locales.Messages.DemoStocks`); Traditional Chinese was converted from Simplified with OpenCC's `s2twp`, then corrected by hand.

## Generated code

The tool writes `catalog_gen.go` into the directory (`cmd/nsgo/i18n_gen.go`). A message without arguments is a string in a table per language, which one method looks up by a constant named after its key (`textDemoStocks`), so adding a message changes only its own lines; one with arguments is compiled to Go per language: a concatenation, or for plurals and selects a `switch` writing to a `strings.Builder`. An app links no YAML, catalog or message parser. The gallery's 1,900 messages in four languages add 1.3 MB to it (2026-11-02). Per key it writes:

- `Key(cx, args...)`, the message in the language the app shows under `cx`, which a binding (`ContentFunc`, `LabelFunc`) follows as the language changes;
- `Messages.Key(args...)`, from `In(locale)` or `For(cx)`, for a language of the caller's choice;
- a field of `Translation`, a function another package's code fills through `Supply(lang, t)` as it initializes.

A method looks a language up in what was supplied, then the catalog's own, then along `i18n.Parent`'s CLDR parents (`en-AU`, `en-001`, `en`), then in the source. `Source` and `Catalog` name the languages; the app's catalogs call `i18n.SetApp(Catalog)`.

The code is committed beside the catalogs, so `go build` and `go test` need no tool. The tool knows a package of catalogs by the header of its `catalog_gen.go`, and reads its source language from `Source`.

## Which language

`core.Env.Locale` is the locale the app runs in: the one `cx.SetLocale` forces, else the one an embedding host sets (`View.SetLocale`), else the user's. `core.Env.Language`, which `i18n.Language(cx)` reads, is the language the app shows: of the app catalog's languages, the one that suits the locale best by `x/text`'s matcher (`zh-Hant` for `zh-TW`, `de` for `de-CH`), with high confidence or not at all, else the source. `i18n.SetApp` gives the core that match (`core.SetLanguageMatch`), as the program initializes; an app without catalogs has no language, which `i18n` reads as English. Every other catalog matches the language among its own and the supplied ones, so an app in English alone shows `base` in English to a Chinese reader, as AppKit does.

`SetLocale` changes all of it at once, and `""` follows the host's or the user's again; the app keeps the choice in its storage, as the gallery's language setting does (`examples/gallery/language.go`).

The app lays out in the direction its language is written in (`core.Env.RTL`): right to left in Arabic or Hebrew whatever the system's, by the tag's script, else its language. Without catalogs it follows the locale it forces, else the system. The core sends the direction to the platform as `ItemDirection` on the first commit and when it changes; iOS's navigation slides and swipes back in it. Native controls and Android's back gesture still mirror as the system does ([roadmap](../roadmap.md#languages)).

Plural categories and numbers come from `golang.org/x/text`, which adds about 250 KB to an app using `base` (2026-11-02, `examples/embedded`). Its plural data is older than CLDR 48: French's `many` for millions is missing.

## The tool

- `nsgo dev` and `nsgo build` check the catalogs of the app's build and write the code of those in its main module: nsgo.yaml's `i18n.dir` (default `locales`, written in `i18n.source`, default `en`) and every package of catalogs the app compiles. `dev` does so before each build and watches the YAML.
- `nsgo test` does the same for `./...`, and `nsgo check` reports code out of date as an error without writing it.
- `nsgo i18n [-source L] [dir]` does it for one directory, a library's, as `//go:generate go tool nsgo i18n` in `base/locales` runs it; in an app's directory it does what `build` does.

Errors stop them: a file not named by a canonical tag, a message that does not parse, a translation's key or argument the source lacks or uses as its type cannot be, two keys making one function name, a supplied package without catalogs or a key it lacks. Warnings do not: a translation's missing messages, plural cases its language lacks or never says, unused placeholders, a package of the build without one of the app's languages, and an app that does not import its catalogs. `TestCatalogsUpToDate` checks NSGo's own code against its catalogs.

## Where this departs from MyGo

- YAML catalogs, nested by key, where MyGo's are ARB JSON with `@key` descriptions.
- Messages compiled to Go per language, where MyGo embeds the catalogs and parses them as an app starts.
- Supplying keyed by the package's import path, any package to any other, where MyGo's apps override `mygo.<key>`.
- Functions take a `*core.Scope`, since several apps share a process when embedded, where MyGo's read a global language.
- `x/text` for plurals and numbers, where MyGo ships CLDR 48's data.
- `SetLocale` forces any locale and keeps nothing, where MyGo's `App.SetLocale` picks one of the app's languages and keeps it for the next launches.
