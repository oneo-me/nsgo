package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// testTimeout bounds one test's run on a target.
const testTimeout = 3 * time.Minute

// targetRunner runs one test of the built package on a target: the test
// binary with args, its environment holding env, writing what it prints
// to out. It returns the process's exit code.
type targetRunner struct {
	run   func(ctx context.Context, args, env []string, out io.Writer) (int, error)
	close func()
}

// runTargetTests runs the app's tests on opts.target, each test in a
// process of its own, as the platform shows one app at a time: the test
// binary, built with the backend's test host, is the target's package.
func runTargetTests(ctx context.Context, p *project, opts testOptions) error {
	names, err := listTests(ctx, p, opts.run)
	if err != nil {
		return err
	}
	if p.steps != nil {
		names = []string{stepsTest}
	}
	if len(names) == 0 {
		plain("no tests match")
		return nil
	}
	p.testBinary = true
	progress := begin("building the tests for %s", opts.target)
	runner, err := targetFor(ctx, p, opts)
	if err != nil {
		progress.fail()
		return err
	}
	progress.done(fmt.Sprintf("built the tests for %s", opts.target), "")
	defer runner.close()
	shots := filepath.Join(p.Dir, ".nsgo", "test", "screenshots", opts.target)
	r := &testRun{running: map[string]*testResult{}, packages: map[string]*testResult{}, trace: opts.trace, out: os.Stderr}
	// A package that links no platform, as a library's, runs headless in
	// the target's runtime: the web's engine, say.
	env := slices.Clone(opts.env)
	if hosted, err := linksPlatform(ctx, p); err != nil {
		return err
	} else if hosted {
		env = append(env, "NSGO_TEST_TARGET="+opts.target)
	}
	if opts.trace {
		env = append(env, "NSGO_TEST_TRACE=1")
	}
	if opts.fps {
		env = append(env, "NSGO_TEST_FPS=1")
	}
	for _, name := range names {
		var out bytes.Buffer
		w := io.MultiWriter(&out, traceWriter{r.out, opts.trace, opts.fps})
		tctx, cancel := context.WithTimeout(ctx, testTimeout)
		started := time.Now()
		code, err := runner.run(tctx, []string{"-test.run", "^" + regexp.QuoteMeta(name) + "$", "-test.v"}, env, w)
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		text, saved := saveScreenshots(out.String(), shots)
		t := parseTestOutput(name, text, code, err)
		if t.Elapsed == 0 {
			t.Elapsed = time.Since(started)
		}
		t.Package = p.ID
		r.results = append(r.results, t)
		r.print(t)
		for _, s := range saved {
			fmt.Fprintf(r.out, "%s  → %s\n", margin, rel(s))
		}
	}
	return r.finish(opts, nil)
}

// traceWriter passes a test's trace lines on as they come: the batches
// with trace, the performance monitor's readings with fps, and the marks
// of what they measure (nsgotest's Mark) always, so a run without the
// monitor can be timed by them too.
type traceWriter struct {
	w          io.Writer
	trace, fps bool
}

func (t traceWriter) Write(b []byte) (int, error) {
	for _, line := range strings.SplitAfter(string(b), "\n") {
		switch {
		case t.trace && (strings.HasPrefix(line, "──") || strings.HasPrefix(line, "   ")):
			io.WriteString(t.w, line)
		case strings.Contains(line, "nsgo: perf ") && (t.fps || strings.Contains(line, "nsgo: perf mark ")):
			io.WriteString(t.w, margin+line[strings.Index(line, "nsgo: perf ")+len("nsgo: "):])
		}
	}
	return len(b), nil
}

// linksPlatform reports whether the package's tests link NSGo's root
// package, which links the platform's test host.
func linksPlatform(ctx context.Context, p *project) (bool, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-test", "-deps", "-f", "{{.ImportPath}}", ".")
	cmd.Dir = p.Dir
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("go list: %w", err)
	}
	return slices.Contains(strings.Fields(string(out)), "github.com/oneo-me/nsgo"), nil
}

// listTests lists the package's tests that run matches, as go test -list
// does on this machine.
func listTests(ctx context.Context, p *project, run string) ([]string, error) {
	if run == "" {
		run = "."
	}
	args := []string{"test", "-list", run}
	// The TSX tests are in a file the tool adds.
	if tsxTest, err := p.tsxTestFile(); err != nil {
		return nil, err
	} else if tsxTest != "" {
		ov, err := p.overlay(map[string]string{"zz_nsgo_tsx_test.go": tsxTest})
		if err != nil {
			return nil, err
		}
		args = append(args, "-overlay", ov)
	}
	cmd := exec.CommandContext(ctx, "go", append(args, ".")...)
	cmd.Dir = p.Dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go test -list: %w\n%s%s", err, out, stderr.Bytes())
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Test") || strings.HasPrefix(line, "Example") {
			names = append(names, strings.TrimSpace(line))
		}
	}
	return names, nil
}

// testLine matches go test -v's result line of a top-level test.
var testLine = regexp.MustCompile(`^--- (PASS|FAIL|SKIP): (\S+) \(([0-9.]+)s\)`)

// parseTestOutput turns one test process's output into its result; a
// process that printed no result for it crashed or timed out.
func parseTestOutput(name, out string, code int, runErr error) *testResult {
	t := &testResult{Name: name, Output: out}
	for _, line := range strings.Split(out, "\n") {
		if m := testLine.FindStringSubmatch(line); m != nil && m[2] == name {
			t.Status = strings.ToLower(m[1])
			s, _ := strconv.ParseFloat(m[3], 64)
			t.Elapsed = time.Duration(s * float64(time.Second))
		}
	}
	if t.Status == "" || t.Status == "pass" && code != 0 {
		t.Status = "fail"
		why := fmt.Sprintf("the test process exited with %d", code)
		if runErr != nil {
			why = runErr.Error()
		}
		t.Output += "\n" + why + "\n"
	}
	return t
}

// screenshotLine is how nsgotest hands the tool a screenshot: its name,
// then the PNG in base64, in chunks a device's log keeps whole.
var screenshotLine = regexp.MustCompile(`nsgo-screenshot (\S+) (\d+)/(\d+) (\S*)`)

// saveScreenshots saves the screenshots out carries in dir and returns
// out without them, and the files written.
func saveScreenshots(out, dir string) (string, []string) {
	var kept strings.Builder
	parts := map[string][]string{}
	var order []string
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 256<<20)
	for sc.Scan() {
		line := sc.Text()
		m := screenshotLine.FindStringSubmatch(line)
		if m == nil {
			kept.WriteString(line + "\n")
			continue
		}
		name := m[1]
		i, _ := strconv.Atoi(m[2])
		n, _ := strconv.Atoi(m[3])
		if parts[name] == nil {
			parts[name] = make([]string, n)
			order = append(order, name)
		}
		if i >= 1 && i <= len(parts[name]) {
			parts[name][i-1] = m[4]
		}
	}
	var saved []string
	for _, name := range order {
		data, err := base64.StdEncoding.DecodeString(strings.Join(parts[name], ""))
		if err != nil {
			kept.WriteString("screenshot " + name + ": " + err.Error() + "\n")
			continue
		}
		path := filepath.Join(dir, filepath.Base(name))
		if filepath.Ext(path) != ".png" {
			path += ".png"
		}
		os.MkdirAll(dir, 0o755)
		if err := os.WriteFile(path, data, 0o644); err == nil {
			saved = append(saved, path)
		}
	}
	return kept.String(), saved
}

// targetFor builds the tests as opts.target's package and returns how to
// run them there.
func targetFor(ctx context.Context, p *project, opts testOptions) (targetRunner, error) {
	dir := p.cache("test", opts.target)
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return targetRunner{}, err
	}
	local := func(bin string) targetRunner {
		return targetRunner{run: func(ctx context.Context, args, env []string, out io.Writer) (int, error) {
			return runLocal(ctx, bin, args, env, out)
		}, close: func() {}}
	}
	switch opts.target {
	case "macos":
		if runtime.GOOS != "darwin" {
			return targetRunner{}, fmt.Errorf("the macos target runs on a Mac")
		}
		app, err := buildMacApp(ctx, p, dir, runtime.GOARCH)
		if err != nil {
			return targetRunner{}, err
		}
		return local(filepath.Join(app, "Contents", "MacOS", p.Name)), nil
	case "windows":
		if runtime.GOOS != "windows" {
			return targetRunner{}, fmt.Errorf("the windows target runs on Windows")
		}
		if err := buildWindows(ctx, p, dir, runtime.GOARCH); err != nil {
			return targetRunner{}, err
		}
		return local(filepath.Join(dir, p.Name+".exe")), nil
	case "linux":
		if runtime.GOOS != "linux" {
			return targetRunner{}, fmt.Errorf("the linux target runs on Linux")
		}
		if err := buildLinux(ctx, p, dir, runtime.GOARCH); err != nil {
			return targetRunner{}, err
		}
		return local(filepath.Join(dir, p.Name)), nil
	case "web":
		return webTestRunner(ctx, p, dir, opts)
	case "ios_simulator":
		return iosTestRunner(ctx, p, iOS, dir, opts)
	case "tvos_simulator":
		return iosTestRunner(ctx, p, tvOS, dir, opts)
	case "watchos_simulator":
		return iosTestRunner(ctx, p, watchOS, dir, opts)
	case "android":
		return androidTestRunner(ctx, p, dir, opts)
	}
	return targetRunner{}, fmt.Errorf("unknown test target %q; use macos, windows, linux, web, ios_simulator, tvos_simulator, watchos_simulator or android", opts.target)
}

// runLocal runs bin on this machine.
func runLocal(ctx context.Context, bin string, args, env []string, out io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = filepath.Dir(bin)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), nil
	}
	if ctx.Err() != nil {
		return -1, fmt.Errorf("the test took over %v", testTimeout)
	}
	return 0, err
}
