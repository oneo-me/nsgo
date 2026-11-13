package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeOf(t *testing.T) {
	for in, want := range map[string]string{
		"":              "",
		".":             ".",
		"...":           ".",
		"./...":         ".",
		"./core/...":    filepath.FromSlash("./core"),
		"examples/tsx":  "examples/tsx",
		"/abs/pkg/...":  filepath.FromSlash("/abs/pkg"),
		"/...":          "/",
		"App.tsx":       "App.tsx",
		"core/x...y":    "core/x...y",
		"core/...extra": "core/...extra",
	} {
		if got := treeOf(in); got != want {
			t.Errorf("treeOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadForTestTakesBareTrees(t *testing.T) {
	dir := t.TempDir()
	p, err := loadForTest(filepath.Join(dir, "..."), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !p.bare || p.Dir != dir {
		t.Fatalf("loadForTest = %+v, want a bare project at %s", p, dir)
	}
	if _, err := loadForTest(filepath.Join(dir, "missing"), nil); err == nil {
		t.Fatal("loadForTest took a missing directory")
	}
	if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: App\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, err = loadForTest(dir, nil); err != nil || p.bare || p.Name != "App" {
		t.Fatalf("loadForTest = %+v, %v, want the app", p, err)
	}
}

// TestBenchmarkResults feeds the events go test -json prints for
// benchmarks, which end with no event of their own, one result split
// over two lines of output.
func TestBenchmarkResults(t *testing.T) {
	var out bytes.Buffer
	r := &testRun{running: map[string]*testResult{}, packages: map[string]*testResult{}, out: &out}
	r.read(strings.NewReader(`{"Action":"run","Package":"p","Test":"BenchmarkTick"}
{"Action":"output","Package":"p","Test":"BenchmarkTick","Output":"=== RUN   BenchmarkTick\n"}
{"Action":"output","Package":"p","Test":"BenchmarkTick","Output":"BenchmarkTick\n"}
{"Action":"output","Package":"p","Test":"BenchmarkTick","Output":"BenchmarkTick-10   \t"}
{"Action":"output","Package":"p","Test":"BenchmarkTick","Output":"  100\t  77375 ns/op\t  26 allocs/op\n"}
{"Action":"output","Package":"p","Test":"BenchmarkBoard/fixed","Output":"BenchmarkBoard/fixed-10 \t 1\t 291583 ns/op\n"}
{"Action":"output","Package":"p","Output":"PASS\n"}
{"Action":"pass","Package":"p","Elapsed":0.2}
`))
	if err := r.finish(testOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	want := margin + "✓ BenchmarkTick-10 100 77375 ns/op 26 allocs/op\n" +
		margin + "✓ BenchmarkBoard/fixed-10 1 291583 ns/op\n" +
		margin + "2 passed, 0 failed, 0 skipped\n"
	if out.String() != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestTSXTestsSkipOwnTrees(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"app/nsgo.yaml", "app/app.test.tsx", "lib/go.mod", "lib/lib.test.tsx", "testdata/x.test.tsx"} {
		path := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := &project{Dir: dir, bare: true}
	if file, err := p.tsxTestFile(); err != nil || file != "" {
		t.Fatalf("tsxTestFile = %q, %v, want none", file, err)
	}
}

func TestIsTestName(t *testing.T) {
	for name, want := range map[string]bool{
		"Test": true, "TestFeel": true, "Test_x": true, "Test2": true,
		"Testing": false, "testFeel": false, "BenchmarkFeel": false,
	} {
		if got := isTestName(name); got != want {
			t.Errorf("isTestName(%q) = %v", name, got)
		}
	}
}
