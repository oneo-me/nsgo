// Command runner builds the benchmark's programs, runs them one at a time
// and writes what they measured to results/<os>.json, which the website's
// Benchmarks page shows. Run it from benchmark/:
//
//	go run ./runner [-frameworks nsgo,mygo,nsh,gpui,gpui-fast] [-cases counter,quotes,list] [-runs 3] [-seconds 10]
//
// Each program prints "bench ready" on its first frame and, while a case
// keeps changing, "bench frames N" each second (internal/bench, cases/).
// The runner times the first line from the start, and reads the process'
// CPU time and memory from the system.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
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

var (
	frameworks = []string{"nsgo", "mygo", "nsh", "gpui", "gpui-fast"}
	cases      = []string{"counter", "quotes", "list"}
	// titles name the cases' windows, for the Go programs whose title is
	// set when they build.
	titles = map[string]string{"counter": "Counter", "quotes": "Quotes", "list": "List"}
)

// Results is results/<os>.json.
type Results struct {
	Date     string                         `json:"date"`
	Machine  Machine                        `json:"machine"`
	Runs     int                            `json:"runs"`
	Seconds  int                            `json:"seconds"`
	Versions map[string]string              `json:"versions"`
	Size     map[string]int64               `json:"size"`
	Cases    map[string]map[string]*Measure `json:"cases"`
	// CompositorAtRest is the compositor's CPU by case with none of the
	// programs open, which their Compositor leaves out.
	CompositorAtRest map[string]float64 `json:"compositorAtRest,omitempty"`
}

// Machine is what the results were measured on.
type Machine struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	CPU    string `json:"cpu"`
	Memory int64  `json:"memory"`
}

// Measure is a program's medians over its runs.
type Measure struct {
	// Startup is the milliseconds from starting the program to its first
	// frame.
	Startup float64 `json:"startup"`
	// Memory is the process' memory at the end, in bytes: its physical
	// footprint on macOS, proportional set size on Linux and working set on
	// Windows.
	Memory int64 `json:"memory"`
	// CPU is the process' CPU time per second of the measured window, in
	// percent of one core.
	CPU float64 `json:"cpu"`
	// Compositor is what the system compositor's CPU over the same window
	// rose by from its rest (Results.CompositorAtRest), in percent of one
	// core: WindowServer on macOS, the Desktop Window
	// Manager on Windows, the compositor and X server on Linux. Native
	// views leave drawing to it, where a GPU renderer draws in the process,
	// so the process' CPU alone favors native views. Unset where the
	// runner cannot read it.
	Compositor float64 `json:"compositor,omitempty"`
	// FPS is the frames the case changed per second, for the cases that
	// change every frame.
	FPS float64 `json:"fps,omitempty"`
	// Error is why the program could not be measured.
	Error string `json:"error,omitempty"`
}

func main() {
	fws := flag.String("frameworks", strings.Join(frameworks, ","), "the `frameworks` to measure")
	cs := flag.String("cases", strings.Join(cases, ","), "the `cases` to measure")
	runs := flag.Int("runs", 3, "runs of each program, interleaved; the median is kept")
	seconds := flag.Int("seconds", 10, "how long a run measures, after it settled")
	nsgoVersion := flag.String("nsgo-version", "", "NSGo's commit, where the checkout has no Git history")
	out := flag.String("out", filepath.Join("results", runtime.GOOS+".json"), "where to write the results")
	flag.Parse()

	r := &Results{
		Date:     time.Now().UTC().Format(time.DateOnly),
		Machine:  machine(),
		Runs:     *runs,
		Seconds:  *seconds,
		Versions: versions(*nsgoVersion),
		Size:     map[string]int64{},
		Cases:    map[string]map[string]*Measure{},

		CompositorAtRest: map[string]float64{},
	}
	bins := map[string]map[string]string{}
	for _, fw := range strings.Split(*fws, ",") {
		b, err := build(fw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", fw, err)
			continue
		}
		bins[fw] = b
		if info, err := os.Stat(b["counter"]); err == nil {
			r.Size[fw] = info.Size()
		}
	}

	for _, c := range strings.Split(*cs, ",") {
		samples := map[string][]*Measure{}
		// Interleaved, so what the machine does meanwhile falls on all.
		var baselines []float64
		for run := range *runs {
			// What the compositor spends with no program of the
			// benchmark's open, which each program's reading leaves out.
			baseline, ok := compositorLoad(time.Duration(*seconds) * time.Second)
			if ok {
				baselines = append(baselines, baseline)
				fmt.Fprintf(os.Stderr, "%s: compositor at rest %.1f%%\n", c, baseline)
			}
			for _, fw := range strings.Split(*fws, ",") {
				bin, ok := bins[fw][c]
				if !ok {
					continue
				}
				fmt.Fprintf(os.Stderr, "%s %s, run %d of %d\n", c, fw, run+1, *runs)
				m, err := measure(bin, c, time.Duration(*seconds)*time.Second)
				if err == nil && ok {
					m.Compositor = max(m.Compositor-baseline, 0)
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "  %v\n", err)
					m = &Measure{Error: err.Error()}
				} else {
					fmt.Fprintf(os.Stderr, "  start %.0f ms, %.1f MB, CPU %.1f%% (compositor %.1f%%), %.1f fps\n", m.Startup, float64(m.Memory)/1e6, m.CPU, m.Compositor, m.FPS)
				}
				samples[fw] = append(samples[fw], m)
				// A moment between programs, for the last one's window to go.
				time.Sleep(time.Second)
			}
		}
		if len(baselines) > 0 {
			r.CompositorAtRest[c] = round(median(baselines))
		}
		r.Cases[c] = map[string]*Measure{}
		for fw, ms := range samples {
			r.Cases[c][fw] = medians(ms)
		}
	}

	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		fatal(err)
	}
	fmt.Fprintln(os.Stderr, "wrote", *out)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "runner:", err)
	os.Exit(1)
}

// build builds a framework's programs as releases, and returns them by
// case.
func build(fw string) (map[string]string, error) {
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	bins := map[string]string{}
	switch fw {
	case "nsgo", "mygo":
		for _, c := range cases {
			out, err := filepath.Abs(filepath.Join("target", fw, c+exe))
			if err != nil {
				return nil, err
			}
			ldflags := "-s -w"
			if fw == "nsgo" {
				ldflags += " -X 'github.com/nsgo-dev/nsgo/core.appName=" + titles[c] + "'"
			}
			if err := run("", nil, "go", "build", "-trimpath", "-ldflags", ldflags, "-o", out, "./"+fw+"/"+c); err != nil {
				return nil, err
			}
			bins[c] = out
		}
	case "nsh", "gpui", "gpui-fast":
		dir, target, args := fw, "target", []string{"build", "--release"}
		if fw == "gpui-fast" {
			// The same programs on GPUI Fast, kept apart from upstream's.
			dir, target = "gpui", filepath.Join("target", "fast")
			args = append(args, "--features", "fast")
		}
		if err := run(dir, []string{"CARGO_TARGET_DIR=" + target}, "cargo", args...); err != nil {
			return nil, err
		}
		for _, c := range cases {
			out, err := filepath.Abs(filepath.Join(dir, target, "release", c+exe))
			if err != nil {
				return nil, err
			}
			bins[c] = out
		}
	default:
		return nil, fmt.Errorf("no framework %q", fw)
	}
	return bins, nil
}

func run(dir string, env []string, name string, args ...string) error {
	fmt.Fprintf(os.Stderr, "%s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	return cmd.Run()
}

// compositorLoad is the compositor's CPU over d, in percent of one core,
// and whether the runner can read it.
func compositorLoad(d time.Duration) (float64, bool) {
	c0, err := compositorCPU()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read the compositor: %v\n", err)
		return 0, false
	}
	t0 := time.Now()
	time.Sleep(d)
	c1, err := compositorCPU()
	if err != nil {
		return 0, false
	}
	return float64(c1-c0) / float64(time.Since(t0)) * 100, true
}

// settle is how long a program runs before the runner measures it.
const settle = 3 * time.Second

// measure runs a program once: its start, then, once it settled, its CPU
// over d, its frames and its memory.
func measure(bin, c string, d time.Duration) (*Measure, error) {
	cmd := exec.Command(bin)
	cmd.Dir = filepath.Dir(bin)
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	lines := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	m := &Measure{}
	select {
	case line, ok := <-lines:
		if !ok || line != "bench ready" {
			return nil, fmt.Errorf("the program printed %q before it was ready", line)
		}
		m.Startup = float64(time.Since(start).Microseconds()) / 1000
	case <-time.After(30 * time.Second):
		return nil, errors.New("the program was not ready in 30 s")
	}

	p, err := openProcess(cmd.Process.Pid)
	if err != nil {
		return nil, err
	}
	defer p.close()
	if !drain(lines, settle, nil) {
		return nil, errors.New("the program exited")
	}
	cpu0, err := p.cpu()
	if err != nil {
		return nil, err
	}
	comp0, compErr := compositorCPU()
	t0 := time.Now()
	var fps []float64
	if !drain(lines, d, &fps) {
		return nil, errors.New("the program exited")
	}
	cpu1, err := p.cpu()
	if err != nil {
		return nil, err
	}
	elapsed := float64(time.Since(t0))
	m.CPU = float64(cpu1-cpu0) / elapsed * 100
	if comp1, err := compositorCPU(); err == nil && compErr == nil {
		m.Compositor = float64(comp1-comp0) / elapsed * 100
	}
	if m.Memory, err = p.memory(); err != nil {
		return nil, err
	}
	if c != "counter" {
		if len(fps) == 0 {
			return nil, errors.New("the program reported no frames")
		}
		m.FPS = median(fps)
	}
	return m, nil
}

var framesLine = regexp.MustCompile(`^bench frames ([0-9.]+)$`)

// drain reads the program's lines for d, keeping its frames in fps, and
// reports whether it is still running.
func drain(lines <-chan string, d time.Duration, fps *[]float64) bool {
	end := time.After(d)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return false
			}
			if m := framesLine.FindStringSubmatch(line); m != nil && fps != nil {
				v, _ := strconv.ParseFloat(m[1], 64)
				*fps = append(*fps, v)
			}
		case <-end:
			return true
		}
	}
}

// medians is the median of each value over runs, or the first error when
// every run failed.
func medians(ms []*Measure) *Measure {
	var ok []*Measure
	for _, m := range ms {
		if m.Error == "" {
			ok = append(ok, m)
		}
	}
	if len(ok) == 0 {
		return ms[0]
	}
	of := func(f func(*Measure) float64) float64 {
		var vs []float64
		for _, m := range ok {
			vs = append(vs, f(m))
		}
		return median(vs)
	}
	return &Measure{
		Startup:    round(of(func(m *Measure) float64 { return m.Startup })),
		Memory:     int64(of(func(m *Measure) float64 { return float64(m.Memory) })),
		CPU:        round(of(func(m *Measure) float64 { return m.CPU })),
		Compositor: round(of(func(m *Measure) float64 { return m.Compositor })),
		FPS:        round(of(func(m *Measure) float64 { return m.FPS })),
	}
}

func median(vs []float64) float64 {
	s := slices.Sorted(slices.Values(vs))
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func round(v float64) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 1, 64), 64)
	return r
}

// versions are the frameworks' versions the programs build with: NSGo's
// commit, and the others' from go.mod and the Cargo.lock files.
func versions(nsgo string) map[string]string {
	v := map[string]string{"nsgo": nsgo}
	if out, err := exec.Command("git", "log", "-1", "--format=%h").Output(); nsgo == "" && err == nil {
		v["nsgo"] = strings.TrimSpace(string(out))
	}
	if b, err := os.ReadFile("go.mod"); err == nil {
		if m := regexp.MustCompile(`github.com/egoist/mygo (v\S+)`).FindSubmatch(b); m != nil {
			v["mygo"] = string(m[1])
		}
	}
	lock := func(file, pkg string) string {
		b, err := os.ReadFile(file)
		if err != nil {
			return ""
		}
		m := regexp.MustCompile(`name = "` + regexp.QuoteMeta(pkg) + `"\nversion = "([^"]+)"`).FindSubmatch(b)
		if m == nil {
			return ""
		}
		return string(m[1])
	}
	v["nsh"] = lock("nsh/Cargo.lock", "nsh-core")
	if kit := lock("gpui/Cargo.lock", "gpui-kit"); kit != "" {
		v["gpui"] = "gpui-kit " + kit + ", gpui-pre " + lock("gpui/Cargo.lock", "gpui-pre")
		v["gpui-fast"] = "gpui-kit " + kit + ", gpui-fast " + lock("gpui/Cargo.lock", "gpui-fast")
	}
	return v
}

// machine describes this machine.
func machine() Machine {
	m := Machine{OS: runtime.GOOS, Arch: runtime.GOARCH}
	m.CPU, m.Memory = cpuAndMemory()
	return m
}
