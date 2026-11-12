package compiler

import (
	"io/fs"
	"path"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx"
)

// App returns the app whose TSX entry is entry in fsys, for nsgo.Run and
// nsgotest.Start. It runs the bundle the nsgo tool compiled ahead for
// entry when the build has one, so the app links no compiler; otherwise,
// under go run and go test, it compiles fsys's sources as it starts.
//
//	//go:embed *.tsx
//	var src embed.FS
//
//	func main() { nsgo.Run(compiler.App(src, "App.tsx")) }
func App(fsys fs.FS, entry string) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		code, ok := bundles[path.Clean(entry)]
		if !ok {
			var err error
			if code, err = compileAtStart(fsys, entry); err != nil {
				panic(err)
			}
		}
		if urls, ok := chunks[path.Clean(entry)]; ok {
			return tsx.SplitApp(entry, code, func(key string) (string, error) { return fetchChunk(urls[key]) })(cx)
		}
		return run(cx, entry, code)
	}
}

// bundles are the bundles compiled ahead, by entry.
var bundles = map[string]string{}

// Bundled gives App the script the nsgo tool compiled ahead for entry.
// The tool's builds call it from a file they add to the app's main
// package; apps do not.
func Bundled(entry, script string) { bundles[path.Clean(entry)] = script }

// chunks are the URLs of the chunks of split bundles, by entry and key.
var chunks = map[string]map[string]string{}

// BundledChunks says that entry's bundle is split (bundler.CompileSplit)
// and where its chunks are, by key: URLs relative to the web page, which
// App fetches as the app's import()s need them. The tool's web builds
// call it with Bundled.
func BundledChunks(entry string, urls map[string]string) { chunks[path.Clean(entry)] = urls }
