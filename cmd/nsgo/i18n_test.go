package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMessage(t *testing.T) {
	for src, want := range map[string]string{
		"Hello, {name}!":                                 `[text "Hello, " arg name text "!"]`,
		"Use '{braces}' and don't":                       `[text "Use {braces} and don't"]`,
		"It''s {n, number, percent}":                     `[text "It's " arg n number percent]`,
		"{n, plural, =0 {none} one {# x} other {'#' #}}": `[plural n =0[text "none"] one[pound text " x"] other[text "# " pound]]`,
		"{g, select, female {She} other {They}}":         `[select g female[text "She"] other[text "They"]]`,
		"{n, plural, offset:1 other {#}}":                `[plural n other[pound]]`,
		"{d, date, short} {d, time} {d, date, ::yMMMd}":  `[arg d date short text " " arg d time text " " arg d date ::yMMMd]`,
		"{n, number, ::.00 group-off}":                   `[arg n number ::.00 group-off]`,
	} {
		parts, err := parseMessage(src)
		if err != nil {
			t.Errorf("%q: %v", src, err)
			continue
		}
		if got := dumpParts(parts); got != want {
			t.Errorf("%q:\n got %s\nwant %s", src, got, want)
		}
	}
	for src, want := range map[string]string{
		"{n, plural, one {x}}":                   "no other case",
		"{n, plural, lots {x} other {y}}":        "not a plural category",
		"{d, date, shortish}":                    "date style",
		"{d, date, ::yMMMQ}":                     "date skeleton",
		"{n, number, ::currency/EUR}":            "not supported",
		"{n":                                     "want",
		"a } b":                                  "unexpected }",
		"{n, plural, one {x} one {y} other {z}}": "twice",
	} {
		if _, err := parseMessage(src); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %v, want one saying %q", src, err, want)
		}
	}
}

func dumpParts(parts []msgPart) string {
	var out []string
	for _, p := range parts {
		switch p := p.(type) {
		case msgText:
			out = append(out, "text "+strconvQuote(string(p)))
		case msgPound:
			out = append(out, "pound")
		case msgArg:
			out = append(out, strings.TrimSpace("arg "+p.name+" "+p.kind+" "+p.style))
		case msgChoice:
			s := p.kind + " " + p.name
			for _, c := range p.cases {
				s += " " + c.key + dumpParts(c.parts)
			}
			out = append(out, s)
		}
	}
	return "[" + strings.Join(out, " ") + "]"
}

func strconvQuote(s string) string { return `"` + s + `"` }

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCatalogDiagnostics(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"en.yaml": `
files: "{count, plural, one {# file} few {# files} other {# files}}"
name: "{who}"
FooBar: x
fooBar: y
counted:
  message: "{n, plural, other {#}}"
  placeholders:
    n: {type: String}
unused:
  message: plain
  placeholders:
    extra: {}
when:
  message: "{d, date} {d, plural, other {#}}"
`,
		"de.yaml": `
files: "{count, plural, other {# Dateien}} {other}"
extra: "Mehr"
`,
		"EN-gb.yaml": `name: x`,
		"ru.yaml":    `files: "{count, plural, one {# файл} other {# файлов}}"`,
	})
	_, ds := loadCatalog(dir, "en")
	var errs, warns []string
	for _, d := range ds {
		if d.warn {
			warns = append(warns, d.String())
		} else {
			errs = append(errs, d.String())
		}
	}
	all := strings.Join(errs, "\n")
	for _, want := range []string{
		"not named after a canonical BCP 47 tag (en-GB.yaml)",
		"FooBar and fooBar make functions of the same name",
		"n is a string, which a number or plural cannot count",
		"extra is not a message of en.yaml",
		"{other} is not an argument",
		"d is both a date and a number or a choice",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("no error saying %q in\n%s", want, all)
		}
	}
	all = strings.Join(warns, "\n")
	for _, want := range []string{
		"en has no few plural case",
		"lacks ru's few case",
		"lacks ru's many case",
		"placeholder extra is not in the message",
		"lacks 6 of the source's messages",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("no warning saying %q in\n%s", want, all)
		}
	}
}

// TestCatalogsUpToDate checks the code of NSGo's own catalogs against
// what nsgo i18n writes.
func TestCatalogsUpToDate(t *testing.T) {
	for _, dir := range []string{"../../base/locales", "../../base/textview/locales"} {
		source, app, ok := generatedCatalog(dir)
		if !ok {
			t.Errorf("%s: no generated catalog", dir)
			continue
		}
		c, ds := loadCatalog(dir, source)
		c.app = app
		if err := ds.err(); err != nil {
			t.Fatal(err)
		}
		code, err := c.generate()
		if err != nil {
			t.Fatal(err)
		}
		old, _ := os.ReadFile(filepath.Join(dir, genFile))
		if string(old) != string(code) {
			t.Errorf("%s is out of date: run go generate ./...", filepath.Join(dir, genFile))
		}
	}
}

// TestCatalogApp writes an app's catalogs, which supply base's, and runs
// a test of the code in the app's module.
func TestCatalogApp(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a module")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.26.0\n\nrequire github.com/oneo-me/nsgo v0.0.0\n\nreplace github.com/oneo-me/nsgo => " + root + "\n",
		"go.sum": string(sum),
		"app.go": "package app\n\nimport _ \"example.com/app/locales\"\n",
		"locales/en.yaml": `
filesSelected:
  message: "{count, plural, =0 {No files selected} one {# file selected} other {# files selected}}"
  description: The status bar, while files are selected.
greeting: "Hello, {name}!"
replied: "{gender, select, female {She} male {He} other {They}} replied."
guests: "{n, plural, offset:1 =0 {Nobody} =1 {{host}} one {{host} and # other} other {{host} and # others}}"
place: "You came {n, selectordinal, one {#st} two {#nd} few {#rd} other {#th}}"
progress: "{done, number, percent} done"
quoted: "Use '{braces}' and don't"
positional: "{0} of {1}"
type: "{type} {string}"
object:
  message: "Item {item}"
  placeholders:
    item: {type: Object}
ratio:
  message: "Ratio {r}"
  placeholders:
    r: {type: double}
due: "Due {d, date, medium} at {d, time, short}"
month: "{d, date, ::yMMMM}"
sent:
  message: "Sent {at}"
  placeholders:
    at: {type: DateTime}
fixed:
  message: "{n, number, ::.00} or {n, number, ::group-off}"
  placeholders:
    n: {type: double}
`,
		"locales/de.yaml": `
filesSelected: "{count, plural, =0 {Keine Dateien ausgewählt} one {# Datei ausgewählt} other {# Dateien ausgewählt}}"
greeting: "Hallo, {name}!"
github.com/oneo-me/nsgo/base/locales:
  sortBy: "Nach {column} sortieren"
`,
		"locales/en-GB.yaml":  `greeting: "Hiya, {name}!"`,
		"locales/ar.yaml":     `due: "{d, date, medium}"`,
		"locales/en.yaml.txt": "",
		"app_test.go": `package app

import (
	"testing"
	"time"

	"example.com/app/locales"
	baselocales "github.com/oneo-me/nsgo/base/locales"
)

func TestMessages(t *testing.T) {
	en, de := locales.In("en-US"), locales.In("de-CH")
	day := time.Date(2026, 1, 5, 13, 7, 0, 0, time.UTC)
	for _, c := range []struct{ got, want string }{
		{en.FilesSelected(0), "No files selected"},
		{en.FilesSelected(1), "1 file selected"},
		{en.FilesSelected(1234), "1,234 files selected"},
		{de.FilesSelected(1234), "1.234 Dateien ausgewählt"},
		{de.Greeting("Ada"), "Hallo, Ada!"},
		{de.Replied("female"), "She replied."},
		{locales.In("en-AU").Greeting("Ada"), "Hiya, Ada!"},
		{locales.In("en-AU").FilesSelected(2), "2 files selected"},
		{locales.In("fr").Greeting("Ada"), "Hello, Ada!"},
		{en.Replied("male"), "He replied."},
		{en.Replied(""), "They replied."},
		{en.Guests(0, "Ada"), "Nobody"},
		{en.Guests(1, "Ada"), "Ada"},
		{en.Guests(2, "Ada"), "Ada and 1 other"},
		{en.Guests(3, "Ada"), "Ada and 2 others"},
		{en.Place(22), "You came 22nd"},
		{en.Place(11), "You came 11th"},
		{en.Progress(0.25), "25% done"},
		{en.Quoted(), "Use {braces} and don't"},
		{en.Positional("1", "2"), "1 of 2"},
		{en.Type("a", "b"), "a b"},
		{en.Object(true), "Item true"},
		{en.Ratio(1.5), "Ratio 1.5"},
		{en.Due(day), "Due Jan 5, 2026 at 1:07 PM"},
		{de.Due(day), "Due Jan 5, 2026 at 1:07 PM"},
		{locales.In("ar-EG").Due(day), "05\u200f/01\u200f/2026"},
		{en.Month(day), "January 2026"},
		{en.Sent(day), "Sent 1/5/26, 1:07 PM"},
		{en.Fixed(1234.5), "1,234.50 or 1234.5"},
		{locales.In("en-XA").Greeting("Ada"), "[Ĥéĺĺó, Ada! ~~~]"},
		{locales.In("en-XA").Quoted(), "[Úśé {ƀŕáćéś} áńď ďóń'ť ~~~~~~~~]"},
		{locales.In("ar-XB").Positional("1", "2"), "\u200f1\u200f \u200f\u202eof\u202c\u200f \u200f2\u200f"},
		{baselocales.In("en-XA").SortBy("Name"), "[Śóŕť ƀý Name ~~~]"},
		{baselocales.In("de-AT").SortBy("Name"), "Nach Name sortieren"},
		{baselocales.In("zh-CN").SortBy("名称"), "按名称排序"},
		{baselocales.In("fr").SortBy("Nom"), "Sort by Nom"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
	if got := locales.Catalog.Languages(); len(got) != 4 || got[0] != "en" {
		t.Errorf("languages %v", got)
	}
}
`,
	})
	// The module requires what NSGo's go.mod does once go adds it.
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOWORK", "off")
	p := &project{Dir: dir}
	p.I18n.Dir, p.I18n.Source = filepath.Join(dir, "locales"), "en"
	ctx := context.Background()
	if err := p.prepareCatalogs(ctx, "./...", false); err != nil {
		t.Fatal(err)
	}
	if err := p.prepareCatalogs(ctx, "./...", true); err != nil {
		t.Errorf("check after writing: %v", err)
	}
	code, _ := os.ReadFile(filepath.Join(dir, "locales", genFile))
	for _, want := range []string{"i18n.SetApp(Catalog)", `baselocales.Supply("de"`, "func Type(cx *core.Scope, type_ string, string_ string) string"} {
		if !strings.Contains(string(code), want) {
			t.Errorf("generated code lacks %q", want)
		}
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test: %v\n%s\n%s", err, out, code)
	}

	// A message changed makes the code out of date.
	writeFiles(t, dir, map[string]string{"locales/en-GB.yaml": `greeting: "Hello there, {name}!"`})
	if err := p.prepareCatalogs(ctx, "./...", true); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Errorf("check after a change: %v", err)
	}
}
