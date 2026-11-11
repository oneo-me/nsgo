# Languages

An app says its messages, and `base`'s components theirs, in the language it shows. Each package owns its messages, written as catalogs in YAML; the `nsgo` tool checks them and compiles them to Go, and package `i18n` runs that code, formatting numbers with `golang.org/x/text` and dates by the Unicode CLDR's data (`internal/cldr`). NSH has none: its apps pick their strings from `locale()` themselves. Read this before changing `i18n`, `internal/cldr`, a package's `locales/` or the tool's `i18n*.go`.

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
    r: { type: double }   # String, int, double, num, DateTime or Object
due: "Due {d, date, medium} at {d, time, short}"
```

```yaml
# de.yaml
filesSelected: "{count, plural, =0 {Keine Dateien ausgewählt} one {# Datei ausgewählt} other {# Dateien ausgewählt}}"
greeting: "Hallo, {name}!"
github.com/oneo-me/nsgo/base/locales:   # supplies base's messages in German
  sortBy: "Nach {column} sortieren"
```

Messages are ICU MessageFormat: `{arg}`, `{n, number}` with `integer`, `percent` or a skeleton of the stems `percent`, `integer`, `.00`, `.0#` and `group-off` (`::.00 group-off`), `{d, date}` and `{d, time}` with `short`, `medium`, `long`, `full` or a skeleton of CLDR's date letters (`::yMMMd`, `::jm`), `plural` with `=N`, `offset:` and `#`, `selectordinal` and `select`, and apostrophes quoting braces. A parameter without a type is a `time.Time` when shown as a date or a time, a `float64` when shown as a percentage, an `int` when counted or shown as a number, else a `string`; a `DateTime` shown as `{d}` is its short date and time. Parameters come in the order of `placeholders`, then of the message.

A message is formatted in its catalog's language: a translation's numbers and dates in the translation's, and a message a translation lacks in the source's, as the source writes it.

`appName`, in the app's catalogs, is the app's name in each language, which its packages list it under ([the tool](#the-tool)); it takes no arguments.

A key that is an import path supplies that package's catalog in the file's language, under its keys, with its parameters. Apps supply languages NSGo does not translate to, or replace its words in one it does.

NSGo's own catalogs are `base/locales`, `base/textview/locales`, `base/terminal/locales`, `desktop/locales` and `updater/locales`, in English and Simplified Chinese. A component in another package gives that package catalogs of its own.

The gallery (`examples/gallery/locales`) says its interface in English and Simplified Chinese. What a demo shows as content stays as written: code, documents, conversations and names. A message keeps an English ID where code or tests key on it, as a demo's `name`, and says its `title` through a method expression (`locales.Messages.DemoStocks`). Right-to-left layout is tested with `ar-XB`, not an Arabic catalog ([pseudo locales](#pseudo-locales)).

## Generated code

The tool writes `catalog_gen.go` into the directory (`cmd/nsgo/i18n_gen.go`). A message without arguments is in a table per language, which one method looks up by a constant named after its key (`textDemoStocks`): the language's messages as one string and their lengths (`i18n.Texts`), which the linker writes as they are, where a string apiece or a map of them would cost a header and a fixup each, or code storing each as the program starts. One with arguments is compiled to Go per language: a concatenation, or for plurals and selects a `switch` writing to a `strings.Builder`. Adding a message changes only its own lines. An app links no YAML, catalog or message parser. The gallery's 1,900 messages in two languages add 1.0 MB to its macOS binary, of which the tables saved 200 KB, and embedding the YAML for a parser to read as the app runs would add 280 KB more (2026-11-08). Per key it writes:

- `Key(cx, args...)`, the message in the language the app shows under `cx`, which a binding (`ContentFunc`, `LabelFunc`) follows as the language changes;
- `Messages.Key(args...)`, from `In(locale)` or `For(cx)`, for a language of the caller's choice;
- an entry in `Translation`, which another package's code fills through `Supply(lang, t)` as it initializes: a message without arguments under its key in `Texts`, one with them as a field of its own, a function;
- its parameters' names in `Parameters`, for TSX ([in TSX](#in-tsx)).

A method looks a language up in what was supplied, then the catalog's own, then along `i18n.Parent`'s CLDR parents (`en-AU`, `en-001`, `en`), then in the source. `Source` and `Catalog` name the languages; the app's catalogs call `i18n.SetApp(Catalog)`.

The code is committed beside the catalogs, so `go build` and `go test` need no tool. The tool knows a package of catalogs by the header of its `catalog_gen.go`, and reads its source language from `Source`.

## Which language

`core.Env.Locale` is the locale the app runs in: the one `cx.SetLocale` forces, else the one an embedding host sets (`View.SetLocale`), else the user's. `core.Env.Language`, which `i18n.Language(cx)` reads, is the language the app shows: of the app catalog's languages, the one that suits the locale best by `x/text`'s matcher (`zh-Hant` for `zh-TW`, `de` for `de-CH`), with high confidence or not at all, else the source. `i18n.SetApp` gives the core that match (`core.SetLanguageMatch`), as the program initializes; an app without catalogs has no language, which `i18n` reads as English. Every other catalog matches the language among its own and the supplied ones, so an app in English alone shows `base` in English to a Chinese reader, as AppKit does.

`SetLocale` changes all of it at once, and `""` follows the host's or the user's again; the app keeps the choice in its storage, as the gallery's language setting does (`examples/gallery/language.go`).

The app lays out in the direction its language is written in (`core.Env.RTL`): right to left in Arabic or Hebrew whatever the system's, by the tag's script, else its language. Without catalogs it follows the locale it forces, else the system. The core sends the direction to the platform as `ItemDirection` on the first commit and when it changes, and each backend lays its native controls out in it:

| Backend | What mirrors |
| --- | --- |
| iOS | Every UIKit view of the surface, its navigation bars, tab bars, fields, alerts and native components, by `semanticContentAttribute`, which a standalone app's `UIView` appearance proxy gives the views UIKit makes later; the navigation's slides and edge swipe |
| Android | The view's tree by `setLayoutDirection`, headers and the back gesture's motion with it, alerts, and an `NsgoActivity`'s window |
| macOS | The surface's AppKit views, fields and scroll bars by `userInterfaceLayoutDirection`, and its menus; the title bar stays as the system lays it out |
| Windows | Menus, the bar's and context menus (`MFT_RIGHTORDER`, `TPM_LAYOUTRTL`); the title bar stays |
| Linux | GTK's default direction, which its menus and widgets follow; an embedded surface its own widgets |
| Web | Alerts; fields take the direction of their text |

An embedded surface leaves the host's views as they are.

Plural categories and numbers come from `golang.org/x/text`, which adds about 250 KB to an app using `base` (2026-11-02, `examples/embedded`); its data is older than CLDR 48, so French's `many` for millions is missing and French groups with no-break spaces where CLDR 48 has narrow ones.

Dates and times come from CLDR 48's data in `internal/cldr`: `go generate ./internal/cldr` writes `tables.go` from the cldr-json packages, pinned by their SHA-256, with each locale of CLDR's modern coverage keeping only what differs from its parent, deflated. A date is written by its style's pattern, or by a skeleton matched to the locale's `availableFormats`: the closest skeleton has the fields asked for, then fields of the same kind, then the nearest widths, and is fitted to the widths asked for; a skeleton of a date and a time the locale has no pattern for joins the two with its date-time pattern. Dates are in the Gregorian calendar, in the locale's digits, zones as offsets from GMT, and narrow no-break and thin spaces written as spaces, as browsers write them. `testdata/dates.json` holds what Chrome's `Intl` (ICU) writes for 29 locales, which the tests match but for two kinds of cases `known` names. The tables are linked only into an app that formats a date. `i18n.FormatDate` and `FormatNumberSkeleton` give apps the same formatting.

### Pseudo locales

`en-XA` and `ar-XB` test layouts before any translation exists, as Android's do. Every catalog matches them (`i18n.IsPseudo`), and says its source's messages transformed: `en-XA` accents their letters, lengthens them by a third and brackets them, `[Śóŕť ƀý Name ~~~]`, so text that stays plain is not in a catalog, cut text has too little room, and several brackets in a sentence are pieces a translation cannot reorder; `ar-XB` shows each word right to left and lays the app out so, so what does not mirror shows. What arguments write stays as written: the generated code marks it in the source's message (`i18n.Arg`) and `i18n.Pseudo` transforms the rest. `nsgo dev -lang en-XA` runs an app in one, as any other locale.

## In TSX

`tsx.RegisterMessages(module, locales.In, locales.Parameters)` makes a package of catalogs importable from TSX: `messages`, whose methods say each message in the language the app shows and follow it as it changes, `messagesIn(locale)` and `language()`. `.nsgo/types` declares them as an interface `Messages` with each message's parameters named and typed (`Date` for a `DateTime`):

```ts
import { messages } from "app/locales";
<Text>{messages.filesSelected(count())}</Text>
```

## The tool

- `nsgo dev` and `nsgo build` check the catalogs of the app's build and write the code of those in its main module: nsgo.yaml's `i18n.dir` (default `locales`, written in `i18n.source`, default `en`) and every package of catalogs the app compiles. `dev` does so before each build and watches the YAML.
- `nsgo test` does the same for `./...`, and `nsgo check` reports code out of date as an error without writing it.
- `nsgo i18n [-source L] [dir]` does it for one directory, a library's, as `//go:generate go tool nsgo i18n` in `base/locales` runs it; in an app's directory it does what `build` does.

Apple bundles list the catalogs' languages in `CFBundleLocalizations`, in which the system says what it adds to the app: the bundle's name in the menu bar, the dictation and emoji items of the Edit menu, panels, and UIKit's and AppKit's own words. It picks one as the app launches, by the user's languages, and no public API changes it later, so a standalone app's backend keeps the language `SetLocale` forces in the app's `AppleLanguages` default, as System Settings' language of an app does, and the system follows it from the next launch; following the user's again removes it, unless the user has set another since (`apple.PinLanguage`). Until then, macOS's app menu names the app in the language shown, by the bundle's `InfoPlist.strings`. Packages name the app by its catalogs' `appName`: a `CFBundleDisplayName` and an `InfoPlist.strings` per language on macOS and iOS, an `@string/app_name` per locale in the APK's resource table, which its label refers to, `name_localized` in the web manifest, and `Name[locale]` in Linux's desktop entry.

Errors stop them: a file not named by a canonical tag, a message that does not parse, a translation's key or argument the source lacks or uses as its type cannot be, two keys making one function name, a supplied package without catalogs or a key it lacks, and an `appName` with arguments. Warnings do not: a translation's missing messages, plural cases its language lacks or never says, unused placeholders, a package of the build without one of the app's languages, and an app that does not import its catalogs. `TestCatalogsUpToDate` checks NSGo's own code against its catalogs.

## Design choices

- YAML catalogs, nested by key, with descriptions and placeholders beside each message.
- Messages compiled to Go per language rather than catalogs parsed as the app starts: an app links no YAML or message parser, and a message costs a table lookup or a function's concatenation. Editing a message's text means writing the code again, which `nsgo dev`, `build` and `test` do themselves for the main module, and `go generate` for a library's catalogs.
- Supplying keyed by the package's import path, any package to any other.
- Functions take a `*core.Scope`, since several apps share a process when embedded, rather than reading a global language.
- `SetLocale` forces any locale and keeps nothing; the app keeps its choice.
