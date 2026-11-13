// Package websrc holds the scripts a page's realm runs: the realm's
// helpers and the sources of the runtime's programs. A release's page
// loads them as files of its own (Script and Bundle write them), so a
// Content-Security-Policy needs no 'unsafe-eval'; every other build
// compiles the same sources itself (docs/architecture/tsx.md).
package websrc

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
)

// A Source is a script as the runtime runs it: an expression, which the
// runtime calls for the program (RunProgram).
type Source struct {
	// Name is what the runtime looks the program up by.
	Name string
	// Source is the script.
	Source string
}

var (
	// Helper makes the realm's helpers, which its JavaScript reaches Go
	// through: the dispatch, release and throw of the engine's calls.
	Helper = Source{"helpers", helperJS}
	// Time patches Date to read the app's clock.
	Time = Source{"time.js", timeJS}
	// Web is the web APIs, as globals.
	Web = Source{"web.js", webJS}
	// Split and Merge are the component calls' props helpers.
	Split = Source{"split.js", splitJS}
	Merge = Source{"merge.js", mergeJS}
	// Harden is a sandbox's: it closes what makes code from strings.
	Harden = Source{"harden.js", hardenJS}
)

// Sources returns what a page ships for a TSX app's realm.
func Sources() []Source { return []Source{Helper, Time, Web, Split, Merge, Harden} }

const (
	// ProgramsGlobal is the global object a runtime file registers the
	// realm's programs on, by name.
	ProgramsGlobal = "__nsgoPrograms"
	// ScriptsGlobal is the global object an app's files register its
	// scripts on, by name: its bundles and their chunks.
	ScriptsGlobal = "__nsgoScripts"
)

// Runtime returns the file a page loads into the app's realm before the
// app starts.
func Runtime() string {
	var b strings.Builder
	b.WriteString("// Written by nsgo: the realm's programs, which the TSX runtime calls by\n")
	b.WriteString("// name. A page loads this before the app, so no policy needs 'unsafe-eval'.\n")
	b.WriteString("((p) => {\n")
	for _, s := range Sources() {
		fmt.Fprintf(&b, "p[%s] = () => (\n%s\n);\n", strconv.Quote(s.Name), s.Source)
	}
	fmt.Fprintf(&b, "})(globalThis[%s] ??= Object.create(null));\n", strconv.Quote(ProgramsGlobal))
	return b.String()
}

// Bundle returns the file that ships an app's script, a compiled bundle
// or one of its chunks, under name: the runtime runs it as it ran the
// script's source.
func Bundle(name, source string) string {
	source, comment := splitMap(source)
	var b strings.Builder
	fmt.Fprintf(&b, "// Written by nsgo: %s, for the page's realm.\n", name)
	fmt.Fprintf(&b, "((p) => { p[%s] = function () {\n", strconv.Quote(name))
	b.WriteString(source)
	b.WriteString("\n}; })(globalThis[")
	b.WriteString(strconv.Quote(ScriptsGlobal))
	b.WriteString("] ??= Object.create(null));\n")
	// A browser reads a source map from a file's last line alone.
	b.WriteString(comment)
	return b.String()
}

// splitMap takes the script's inline source map comment, if it has one,
// which stays the file's last line.
func splitMap(source string) (body, comment string) {
	body = strings.TrimRight(source, "\n")
	if i := strings.LastIndexByte(body, '\n'); strings.HasPrefix(body[i+1:], "//# sourceMappingURL=") {
		return body[:i], body[i+1:] + "\n"
	}
	return body, ""
}

//go:embed helper.js
var helperJS string

//go:embed time.js
var timeJS string

//go:embed web.js
var webJS string

//go:embed split.js
var splitJS string

//go:embed merge.js
var mergeJS string

//go:embed harden.js
var hardenJS string
