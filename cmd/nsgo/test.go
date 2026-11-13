package main

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// testOptions are nsgo test's flags.
type testOptions struct {
	run    string // the tests to run, as go test's -run
	bench  string // the benchmarks to run, as go test's -bench
	junit  string // where to write a JUnit report
	trace  bool   // print each step's batches and commit times
	target string // the platform to run on; headless when ""
	fps    bool   // log the performance monitor's readings on target
	device string // the simulator or device a mobile target runs on
	// env are the tests' KEY=VALUE variables, on every target: a
	// phone's tests see no others of this machine's.
	env  []string
	args []string
}

// testEvent is a line of go test -json (test2json's format).
type testEvent struct {
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
	// ImportPath names what a build-output event's compiler output is
	// of, which FailedBuild names on the package's fail event.
	ImportPath  string
	FailedBuild string
}

// testResult is one test's outcome.
type testResult struct {
	Package, Name string
	Status        string // pass, fail or skip
	Elapsed       time.Duration
	Output        string
	reported      int // how much of a benchmark's Output it reported
}

// testRun gathers what go test -json reports.
type testRun struct {
	results []*testResult
	running map[string]*testResult
	// packages hold what a package printed outside its tests, such as
	// build errors, and whether it failed.
	packages map[string]*testResult
	// builds hold the compiler's output by import path.
	builds map[string]string
	trace  bool
	out    io.Writer
}

// runTests runs the app's tests headless, or on a target, printing a line
// per test and failing when any failed.
func runTests(ctx context.Context, p *project, opts testOptions) error {
	if err := p.prepareCatalogs(ctx, "./...", false); err != nil {
		return err
	}
	if opts.fps && !slices.Contains([]string{"macos", "windows", "linux", "ios_simulator", "web", "android"}, opts.target) {
		return errors.New("test: -fps goes with -target macos, windows, linux, ios_simulator, web or android, whose monitors it reads")
	}
	if opts.bench != "" && (opts.target != "" || p.steps != nil) {
		return errors.New("test: -bench runs headless, on the package's tests")
	}
	if opts.target != "" {
		return runTargetTests(ctx, p, opts)
	}
	if p.steps != nil {
		opts.run = "^" + stepsTest + "$"
	}
	args := []string{"test", "-json"}
	if opts.bench != "" {
		// Benchmarks alone, unless -run names tests to run with them.
		args = append(args, "-bench", opts.bench)
		if opts.run == "" {
			opts.run = "^$"
		}
	}
	if opts.run != "" {
		args = append(args, "-run", opts.run)
	}
	if opts.trace {
		// Traces print as the tests run; -v keeps go test from hiding them.
		args = append(args, "-v", "-count=1")
	}
	tsxTest, err := p.tsxTestFile()
	if err != nil {
		return err
	}
	if tsxTest != "" {
		ov, err := p.overlay(map[string]string{"zz_nsgo_tsx_test.go": tsxTest})
		if err != nil {
			return err
		}
		args = append(args, "-overlay", ov)
	}
	args = append(args, opts.args...)
	if !hasPackage(opts.args) {
		args = append(args, "./...")
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = p.Dir
	cmd.Env = append(os.Environ(), opts.env...)
	if opts.trace {
		cmd.Env = append(cmd.Env, "NSGO_TEST_TRACE=1")
	}
	return reportTests(cmd, opts)
}

// loadForTest reads the app's nsgo.yaml; a directory without one still
// has tests to run headless. dir may end in /..., as go's patterns do,
// which names the same tree. Given an app's TSX entry, it reads its
// directory's, and the steps standard input holds, none from a terminal.
func loadForTest(dir string, stdin *os.File) (*project, error) {
	dir = treeOf(dir)
	if ext := filepath.Ext(dir); (ext == ".tsx" || ext == ".ts") && !strings.HasSuffix(dir, ".test"+ext) {
		if fi, err := os.Stat(dir); err != nil || fi.IsDir() {
			return nil, fmt.Errorf("test: no entry %s", dir)
		}
		p, err := loadForTest(filepath.Dir(dir), nil)
		if err != nil {
			return nil, err
		}
		s := &steps{entry: filepath.ToSlash(filepath.Base(dir))}
		if fi, err := stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
			b, err := io.ReadAll(stdin)
			if err != nil {
				return nil, fmt.Errorf("test: the steps of standard input: %w", err)
			}
			s.src = string(b)
		}
		p.steps = s
		return p, nil
	}
	return loadOrDir(dir)
}

// treeOf is the directory a go pattern ending in /... names, or dir.
func treeOf(dir string) string {
	if dir == "..." {
		return "."
	}
	if t, ok := strings.CutSuffix(filepath.ToSlash(dir), "/..."); ok {
		if t == "" {
			return "/"
		}
		return filepath.FromSlash(t)
	}
	return dir
}

// hasPackage reports whether go test's arguments name packages.
func hasPackage(args []string) bool {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return true
		}
	}
	return false
}

// reportTests runs cmd, which prints go test -json, and reports its
// tests.
func reportTests(cmd *exec.Cmd, opts testOptions) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	r := &testRun{running: map[string]*testResult{}, packages: map[string]*testResult{}, trace: opts.trace, out: os.Stderr}
	r.read(stdout)
	waitErr := cmd.Wait()
	return r.finish(opts, waitErr)
}

func (r *testRun) read(in io.Reader) {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var ev testEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			fmt.Fprintln(r.out, sc.Text())
			continue
		}
		r.event(ev)
	}
}

func (r *testRun) event(ev testEvent) {
	if ev.Action == "build-output" {
		if r.builds == nil {
			r.builds = map[string]string{}
		}
		r.builds[ev.ImportPath] += ev.Output
		return
	}
	if ev.Test == "" {
		pkg := r.packages[ev.Package]
		if pkg == nil {
			pkg = &testResult{Package: ev.Package}
			r.packages[ev.Package] = pkg
		}
		switch ev.Action {
		case "output":
			pkg.Output += ev.Output
		case "fail":
			pkg.Status = "fail"
			// A package that did not build shows why.
			pkg.Output += r.builds[ev.FailedBuild]
		case "pass", "skip":
			pkg.Status = ev.Action
		}
		return
	}
	key := ev.Package + "\x00" + ev.Test
	t := r.running[key]
	if t == nil {
		t = &testResult{Package: ev.Package, Name: ev.Test}
		r.running[key] = t
	}
	switch ev.Action {
	case "output":
		t.Output += ev.Output
		if strings.HasPrefix(ev.Test, "Benchmark") {
			r.benchmark(t)
		}
		if r.trace && !strings.HasPrefix(ev.Output, "=== ") && !strings.HasPrefix(ev.Output, "--- ") {
			fmt.Fprint(r.out, ev.Output)
		}
	case "pass", "fail", "skip":
		t.Status, t.Elapsed = ev.Action, time.Duration(ev.Elapsed*float64(time.Second))
		delete(r.running, key)
		r.results = append(r.results, t)
		r.print(t)
	}
}

// benchmark reports each result line a benchmark has printed whole as a
// passed test, since go test -json ends benchmarks with no event.
func (r *testRun) benchmark(t *testResult) {
	for {
		line, _, ok := strings.Cut(t.Output[t.reported:], "\n")
		if !ok {
			return
		}
		t.reported += len(line) + 1
		if !strings.Contains(line, " ns/op") {
			continue
		}
		res := &testResult{Package: t.Package, Name: t.Name, Status: "pass", Output: line}
		r.results = append(r.results, res)
		fmt.Fprintf(r.out, "%s✓ %s\n", margin, strings.Join(strings.Fields(line), " "))
	}
}

// print shows one finished test: a line, and a failure's output, a
// subtest's under its own line (go test gives each test its own).
func (r *testRun) print(t *testResult) {
	mark := map[string]string{"pass": "✓", "fail": "✗", "skip": "-"}[t.Status]
	fmt.Fprintf(r.out, "%s%s %s (%s)\n", margin, mark, t.Name, t.Elapsed.Round(time.Millisecond))
	if t.Status == "fail" {
		for _, line := range strings.Split(strings.TrimRight(t.Output, "\n"), "\n") {
			if strings.HasPrefix(line, "=== ") || strings.HasPrefix(line, "--- ") {
				continue
			}
			fmt.Fprintf(r.out, "%s    %s\n", margin, strings.TrimPrefix(line, "    "))
		}
	}
}

// finish prints the tally, writes the JUnit report and reports failure.
func (r *testRun) finish(opts testOptions, waitErr error) error {
	var passed, failed, skipped int
	for _, t := range r.results {
		// Subtests count with their test; a benchmark's every result
		// counts, its subbenchmarks' too.
		if strings.Contains(t.Name, "/") && !strings.HasPrefix(t.Name, "Benchmark") {
			continue
		}
		switch t.Status {
		case "pass":
			passed++
		case "fail":
			failed++
		case "skip":
			skipped++
		}
	}
	pkgFailed := false
	names := make([]string, 0, len(r.packages))
	for name := range r.packages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pkg := r.packages[name]
		// A package that failed with no failed test did not build or
		// crashed outside a test.
		if pkg.Status == "fail" && !r.anyFailed(name) {
			pkgFailed = true
			fmt.Fprintf(r.out, "%s✗ %s\n%s", margin, name, indent(pkg.Output))
		}
	}
	fmt.Fprintf(r.out, "%s%d passed, %d failed, %d skipped\n", margin, passed, failed, skipped)
	if opts.junit != "" {
		if err := writeJUnit(opts.junit, r.results, r.packages); err != nil {
			return err
		}
		note("%s", rel(opts.junit))
	}
	switch {
	case failed > 0:
		if failed == 1 {
			return errors.New("1 test failed")
		}
		return fmt.Errorf("%d tests failed", failed)
	case pkgFailed:
		return errors.New("tests did not build")
	case waitErr != nil && passed+skipped == 0:
		return waitErr
	}
	return nil
}

func (r *testRun) anyFailed(pkg string) bool {
	for _, t := range r.results {
		if t.Package == pkg && t.Status == "fail" {
			return true
		}
	}
	return false
}

func indent(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString(margin + "    " + line + "\n")
	}
	return b.String()
}

// The JUnit report's format, as CI systems read it.
type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Time     string       `xml:"time,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Skipped  int         `xml:"skipped,attr"`
	Time     string      `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Class   string        `xml:"classname,attr"`
	Name    string        `xml:"name,attr"`
	Time    string        `xml:"time,attr"`
	Failure *junitFailure `xml:"failure,omitempty"`
	Skipped *struct{}     `xml:"skipped,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

// writeJUnit writes results as a JUnit XML report at path, a suite per
// package; a package that did not build is a failed case of its own.
func writeJUnit(path string, results []*testResult, packages map[string]*testResult) error {
	bySuite := map[string]*junitSuite{}
	var order []string
	suite := func(pkg string) *junitSuite {
		s := bySuite[pkg]
		if s == nil {
			s = &junitSuite{Name: pkg}
			bySuite[pkg] = s
			order = append(order, pkg)
		}
		return s
	}
	var total, failures int
	var elapsed time.Duration
	add := func(t *testResult) {
		s := suite(t.Package)
		c := junitCase{Class: t.Package, Name: t.Name, Time: seconds(t.Elapsed)}
		switch t.Status {
		case "fail":
			c.Failure = &junitFailure{Message: firstLine(t.Output), Body: t.Output}
			s.Failures++
			failures++
		case "skip":
			c.Skipped = &struct{}{}
			s.Skipped++
		}
		s.Tests++
		total++
		s.Cases = append(s.Cases, c)
		elapsed += t.Elapsed
	}
	for _, t := range results {
		add(t)
	}
	for name, pkg := range packages {
		failed := false
		for _, t := range results {
			failed = failed || t.Package == name && t.Status == "fail"
		}
		if pkg.Status == "fail" && !failed {
			add(&testResult{Package: name, Name: "build", Status: "fail", Output: pkg.Output})
		}
	}
	sort.Strings(order)
	out := junitSuites{Tests: total, Failures: failures, Time: seconds(elapsed)}
	for _, name := range order {
		s := bySuite[name]
		var d time.Duration
		for _, c := range s.Cases {
			v, _ := time.ParseDuration(c.Time + "s")
			d += v
		}
		s.Time = seconds(d)
		out.Suites = append(out.Suites, *s)
	}
	data, err := xml.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(xml.Header), append(data, '\n')...), 0o644)
}

func seconds(d time.Duration) string { return fmt.Sprintf("%.3f", d.Seconds()) }

// firstLine is a failure's message: the first line it printed that is
// not go test's own.
func firstLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "=== ") && !strings.HasPrefix(line, "--- ") {
			return line
		}
	}
	return ""
}
