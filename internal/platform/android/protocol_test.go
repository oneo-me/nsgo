package android

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// javaNames are the driver's names of the kinds Java names otherwise.
var javaNames = map[string]string{"EventHoverIn": "EventMouseEnter", "EventHoverOut": "EventMouseLeave", "EventHoverMove": "EventMouseMove"}

// TestJavaProtocolMatchesTheDriver holds Protocol.java's item and event
// kinds to the driver's of the same names, which the Java runtime reads
// the batches and writes the events by: a kind the driver gains shifts
// the ones after it.
func TestJavaProtocolMatchesTheDriver(t *testing.T) {
	fset := token.NewFileSet()
	dir := filepath.Join("..", "..", "driver")
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, f := range pkgs["driver"].Files {
		files = append(files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/nsgo-dev/nsgo/internal/driver", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	java, err := os.ReadFile(filepath.Join("java", "dev", "oneo", "nsgo", "Protocol.java"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, m := range regexp.MustCompile(`static final int (ITEM|EVENT)_(\w+) = (\d+);`).FindAllStringSubmatch(string(java), -1) {
		name := strings.ToUpper(m[1][:1]) + strings.ToLower(m[1][1:])
		for _, word := range strings.Split(m[2], "_") {
			if word == "URL" {
				name += word
				continue
			}
			name += word[:1] + strings.ToLower(word[1:])
		}
		if n, ok := javaNames[name]; ok {
			name = n
		}
		c, ok := pkg.Scope().Lookup(name).(*types.Const)
		if !ok {
			t.Errorf("Protocol.java's %s_%s is no driver constant %s", m[1], m[2], name)
			continue
		}
		want, _ := constant.Int64Val(c.Val())
		if got, _ := strconv.Atoi(m[3]); int64(got) != want {
			t.Errorf("Protocol.java's %s_%s is %d, the driver's %s %d", m[1], m[2], got, name, want)
		}
		checked++
	}
	if checked < 50 {
		t.Fatalf("checked only %d constants", checked)
	}
}
