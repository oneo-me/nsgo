---
title: Localization
description: Translate an app's messages and follow the language it shows.
group: Guides
order: 45
---

## Messages

`locales/` beside `nsgo.yaml` holds the app's messages, a YAML file per language (`en.yaml`, `de.yaml`) in ICU MessageFormat:

```yaml
greeting: Hello, {name}!
appName:
  message: Hello
  description: The app's name, for translators.
```

A message is a string, or `message` with a `description` and `placeholders` for translators.

The package imports `locales/`. `dev`, `build` and `test` check the catalogs and write a Go function per message beside them, such as `locales.Greeting(cx, "Ada")`, which says it in the language the app shows and follows it as it changes; `go tool nsgo i18n` writes the functions alone. Messages format numbers and dates (`{d, date, medium}`), and `appName` names the app in each language in its packages.

## The language

`cx.SetLocale("de")` chooses the language, and the layout runs in the direction it is written in.

```sh
go tool nsgo dev -lang en-XA
```

runs the app in a pseudo locale that shows untranslated and cut text.

## NSGo's own words

NSGo's components say their words in the languages NSGo is translated to. For another language, a key that names a package, such as `github.com/oneo-me/nsgo/base/locales`, gives that package's messages in the file's language.
