package nsgo

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// unlinked lists what an app that shows one text must not link, by
// package, for each target ("" for all): what only an app's own use
// brings in, or what a backend leaves to the system
// (docs/architecture/layers.md, What an app links).
var unlinked = map[string][]string{
	"": {
		// Images download through the system's HTTP client (imageload.HTTP).
		"net/http", "crypto/tls", "crypto/x509",
		// Shaders compile when an app makes one: no package variable,
		// init or element case of its own reaches the compilers.
		"github.com/nsgo-dev/nsgo/internal/shader", "github.com/nsgo-dev/nsgo/internal/shader/wgsl", "go/parser",
	},
	// The page decodes and encodes images with the browser's codecs.
	"js": {"image/png", "image/jpeg", "image/gif", "golang.org/x/image/webp", "golang.org/x/image/draw"},
	// Only Android reads its resources from a zip, the APK.
	"darwin":  {"archive/zip"},
	"windows": {"archive/zip"},
	"linux":   {"archive/zip"},
}

func TestMinimalAppLinks(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("needs the go command")
	}
	for _, target := range [][2]string{{"darwin", "arm64"}, {"windows", "amd64"}, {"linux", "amd64"}, {"android", "arm64"}, {"js", "wasm"}} {
		t.Run(target[0]+"_"+target[1], func(t *testing.T) {
			t.Parallel()
			linked := linkedPackages(t, target[0], target[1])
			for _, pkg := range append(unlinked[""], unlinked[target[0]]...) {
				if linked[pkg] {
					t.Errorf("an app showing one text links %s", pkg)
				}
			}
		})
	}
}

// linkedPackages returns the packages the linker keeps in
// testdata/minimal for goos and goarch for what other packages reach:
// those its dependency dump has an edge into from another package's
// symbol. A package's own init and the tables it fills, and literals the
// linker shares by content, do not count.
func linkedPackages(t *testing.T, goos, goarch string) map[string]bool {
	t.Helper()
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-dumpdep", "-o", os.DevNull, "./testdata/minimal")
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0", "GOFLAGS=")
	var dump bytes.Buffer
	cmd.Stderr = &dump
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v\n%s", err, dump.Bytes())
	}
	linked := map[string]bool{}
	sc := bufio.NewScanner(&dump)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		from, to, ok := strings.Cut(sc.Text(), " -> ")
		if !ok {
			continue
		}
		to, _, _ = strings.Cut(to, " ")
		if pseudoSymbol(from) || pseudoSymbol(to) {
			continue
		}
		if pkg := symbolPackage(to); pkg != symbolPackage(from) {
			linked[pkg] = true
		}
	}
	return linked
}

// pseudoSymbol reports whether sym is no code of its package's: the
// runner of inits, an init's task, or data the compiler names after
// where it first appeared, such as a string literal or a frame's layout.
func pseudoSymbol(sym string) bool {
	if sym == "go:main.inittasks" || strings.HasSuffix(sym, "..inittask") {
		return true
	}
	for _, s := range []string{"..stmp_", ".stkobj", ".opendefer", ".arginfo", ".argliveinfo", "..gobytes.", "..dict."} {
		if strings.Contains(sym, s) {
			return true
		}
	}
	return false
}

// symbolPackage returns the package of a linker symbol, such as
// "net/http" of "net/http.(*Client).Do" or of "type:*net/http.Client".
func symbolPackage(sym string) string {
	for _, prefix := range []string{"type:.eq.", "type:.hash.", "type:", "go:itab.", "go:"} {
		if rest, ok := strings.CutPrefix(sym, prefix); ok {
			sym = rest
			break
		}
	}
	sym = strings.TrimLeft(sym, "*(")
	slash := strings.LastIndexByte(sym, '/')
	if dot := strings.IndexByte(sym[slash+1:], '.'); dot >= 0 {
		return sym[:slash+1+dot]
	}
	return sym
}
