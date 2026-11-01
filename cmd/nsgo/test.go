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
	"sort"
	"strings"
	"time"
)

// testOptions are nsgo test's flags.
type testOptions struct {
	run    string // the tests to run, as go test's -run
	junit  string // where to write a JUnit report
	trace  bool   // print each step's batches and commit times
	target string // the platform to run on; headless when ""
	device string // the simulator or device a mobile target runs on
	args   []string
}

// testEvent is a line of go test -json (test2json's format).
type testEvent struct {
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
}

// testResult is one test's outcome.
type testResult struct {
	Package, Name string
	Status        string // pass, fail or skip
	Elapsed       time.Duration
	Output        string
}

// testRun gathers what go test -json reports.
type testRun struct {
	results []*testResult
	running map[string]*testResult
	// packages hold what a package printed outside its tests, such as
	// build errors, and whether it failed.
	packages map[string]*testResult
	trace    bool
	out      io.Writer
}

// runTests runs the app's tests headless, or on a target, printing a line
// per test and failing when any failed.
func runTests(ctx context.Context, p *project, opts testOptions) error {
	if opts.target != "" {
		return runTargetTests(ctx, p, opts)
	}
	args := []string{"test", "-json"}
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
	cmd.Env = os.Environ()
	if opts.trace {
		cmd.Env = append(cmd.Env, "NSGO_TEST_TRACE=1")
	}
	return reportTests(cmd, opts)
}

// loadForTest reads the app's nsgo.yaml; a directory without one still
// has tests to run headless.
func loadForTest(dir string) (*project, error) {
	p, err := load(dir)
	if errors.Is(err, os.ErrNotExist) {
		if dir == "" {
			dir = "."
		}
		abs, absErr := filepath.Abs(dir)
		if absErr != nil {
			return nil, absErr
		}
		return &project{Dir: abs, Name: filepath.Base(abs)}, nil
	}
	return p, err
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

// print shows one finished test: a line, and a failure's output.
func (r *testRun) print(t *testResult) {
	mark := map[string]string{"pass": "✓", "fail": "✗", "skip": "-"}[t.Status]
	fmt.Fprintf(r.out, "%s %s (%s)\n", mark, t.Name, t.Elapsed.Round(time.Millisecond))
	if t.Status == "fail" && !strings.Contains(t.Name, "/") {
		for _, line := range strings.Split(strings.TrimRight(t.Output, "\n"), "\n") {
			if strings.HasPrefix(line, "=== ") || strings.HasPrefix(line, "--- ") {
				continue
			}
			fmt.Fprintf(r.out, "    %s\n", strings.TrimPrefix(line, "    "))
		}
	}
}

// finish prints the tally, writes the JUnit report and reports failure.
func (r *testRun) finish(opts testOptions, waitErr error) error {
	var passed, failed, skipped int
	for _, t := range r.results {
		if strings.Contains(t.Name, "/") {
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
			fmt.Fprintf(r.out, "✗ %s\n%s", name, indent(pkg.Output))
		}
	}
	fmt.Fprintf(r.out, "%d passed, %d failed, %d skipped\n", passed, failed, skipped)
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
		b.WriteString("    " + line + "\n")
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
