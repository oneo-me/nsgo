package main

// The tool's terminal output follows NSH's CLI
// (nsh/crates/nsh-cli/src/ui.rs, nsh/docs/dev/tooling.md "Output"): one
// line per step, and the details only when a step fails.
//
//   - The tool's lines are indented by margin, so the app's own lines,
//     printed as they are, stand apart from them.
//   - ✓ a step that finished, → where to find something, • what a server
//     or device did, warn and error for problems; plain lines are the app's
//     own output.
//   - A step that takes a while shows a spinner in its line on a terminal,
//     with what it does at the moment (stage), replaced by its ✓ line, or
//     cleared when it fails and the error follows. Steps running at once,
//     such as nsgo build's targets, show a spinner line each.
//   - Colors on terminals unless NO_COLOR is set; redirected, only the
//     result lines appear.
//   - All of it is stderr, so stdout stays free for what a command prints.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// terminal says how much stderr can be decorated, decided once.
type terminal struct{ tty, color bool }

var terminalOf = sync.OnceValue(func() terminal {
	enableVT()
	info, err := os.Stderr.Stat()
	tty := err == nil && info.Mode()&os.ModeCharDevice != 0
	return terminal{tty: tty, color: tty && os.Getenv("NO_COLOR") == ""}
})

func paint(code, text string) string {
	if !terminalOf().color {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// dim is secondary text: timings, sizes, hints.
func dim(text string) string { return paint("2", text) }

func bold(text string) string   { return paint("1", text) }
func green(text string) string  { return paint("32", text) }
func cyan(text string) string   { return paint("36", text) }
func yellow(text string) string { return paint("33", text) }
func red(text string) string    { return paint("31", text) }

// link is url, clickable in terminals that support hyperlinks (OSC 8).
func link(url string) string {
	if !terminalOf().color {
		return url
	}
	return "\x1b]8;;" + url + "\x1b\\" + cyan(url) + "\x1b]8;;\x1b\\"
}

// formatSize is how large a package is, as its line says: 500 B, 3.2 KB
// or 1.4 MB.
func formatSize(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return fmt.Sprintf("%d B", n)
	case n < k*k:
		return fmt.Sprintf("%.1f KB", float64(n)/k)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(k*k))
	}
}

// formatDuration is how long a step took: 21ms, 1.2s or 1m 3s.
func formatDuration(d time.Duration) string {
	ms := d.Milliseconds()
	switch {
	case ms < 1000:
		return fmt.Sprintf("%dms", ms)
	case ms < 60_000:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return fmt.Sprintf("%dm %ds", ms/60_000, ms%60_000/1000)
	}
}

// toolVersion is the nsgo module's version as the tool was built: its
// release tag, the pseudo-version of a commit, or nothing when the build
// records none.
func toolVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	const module = "github.com/nsgo-dev/nsgo"
	if info.Main.Path == module {
		return builtVersion(info.Main.Version)
	}
	for _, dep := range info.Deps {
		if dep.Path != module {
			continue
		}
		if dep.Replace != nil {
			return builtVersion(dep.Replace.Version)
		}
		return builtVersion(dep.Version)
	}
	return ""
}

// builtVersion is a version worth showing; a checkout's "(devel)" is not.
func builtVersion(v string) string {
	if v == "(devel)" {
		return ""
	}
	return v
}

// margin indents the tool's lines.
const margin = "  "

// header titles a command's output: which NSGo built it, the app's name
// and version, then what it builds, set apart by blank lines.
func header(name, version, what string) {
	emit("")
	line(headerLine(toolVersion(), name, version, what))
	emit("")
}

func headerLine(tool, name, version, what string) string {
	line := bold("NSGo")
	if tool != "" {
		line += " " + dim(tool)
	}
	line += " " + dim("·") + " " + bold(name) + " " + dim(version)
	if what != "" {
		line += " " + dim("· "+what)
	}
	return line
}

// done prints "✓ message": a step that finished.
func done(format string, args ...any) {
	line(green("✓") + " " + fmt.Sprintf(format, args...))
}

// note prints "→ message": where to find something.
func note(format string, args ...any) {
	line(cyan("→") + " " + fmt.Sprintf(format, args...))
}

// event prints "• message": what a server or device did.
func event(format string, args ...any) {
	line(cyan("•") + " " + fmt.Sprintf(format, args...))
}

// detail prints a line under the one above, dimmed.
func detail(format string, args ...any) {
	line("  " + dim(fmt.Sprintf(format, args...)))
}

// warn prints "warn message": a problem that does not end the command.
func warn(format string, args ...any) {
	line(yellow("warn") + " " + fmt.Sprintf(format, args...))
}

// fail prints "error message": why the command ends.
func fail(format string, args ...any) {
	line(red("error") + " " + fmt.Sprintf(format, args...))
}

// plain prints a line as it is: an app's or a test's own output.
func plain(format string, args ...any) {
	emit(fmt.Sprintf(format, args...))
}

// report prints a command's error: "error message", then the lines it
// carries after the first, such as a tool's output, indented.
func report(err error) {
	message, rest, _ := strings.Cut(err.Error(), "\n")
	fail("%s", message)
	if rest = strings.TrimRight(rest, "\n"); rest == "" {
		return
	}
	for _, l := range strings.Split(rest, "\n") {
		line("  " + l)
	}
}

// screen is what is printed and the steps whose spinners are drawn below
// it, a line each.
type screen struct {
	sync.Mutex
	steps []*Step
	drawn int
	frame int
}

var out screen

// console is where a running app's output goes: each of its lines is
// printed above a step's spinner, so the app's log and the tool's lines
// never tear each other.
type console struct {
	mu  sync.Mutex
	buf []byte
}

var appConsole console

func (c *console) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf = append(c.buf, p...)
	for {
		i := bytes.IndexByte(c.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSuffix(string(c.buf[:i]), "\r")
		c.buf = c.buf[i+1:]
		emit(line)
	}
	return len(p), nil
}

// line writes one of the tool's lines, indented by margin.
func line(text string) { emit(margin + text) }

// emit writes a whole line above the running step's spinner.
func emit(text string) {
	out.Lock()
	defer out.Unlock()
	out.clear()
	fmt.Fprint(os.Stderr, text)
	if !strings.HasSuffix(text, "\n") {
		fmt.Fprintln(os.Stderr)
	}
	out.draw()
}

// clear removes the spinner lines, leaving the cursor at the start of the
// first. The caller holds out's lock.
func (o *screen) clear() {
	if o.drawn == 0 {
		return
	}
	fmt.Fprint(os.Stderr, "\r\x1b[2K"+strings.Repeat("\x1b[1A\x1b[2K", o.drawn-1))
	o.drawn = 0
}

// draw shows the running steps' spinners, a line each, or clears them when
// none runs. The caller holds out's lock.
func (o *screen) draw() {
	o.clear()
	if len(o.steps) == 0 {
		return
	}
	lines := make([]string, len(o.steps))
	for i, s := range o.steps {
		label := []rune(s.label)
		if len(label) > 64 {
			label = append(label[:63], '…')
		}
		line := fmt.Sprintf("%s%s %s", margin, cyan(spinnerFrames[o.frame%len(spinnerFrames)]), string(label))
		if elapsed := time.Since(s.start); elapsed >= time.Second {
			line += " " + dim(fmt.Sprintf("%ds", int(elapsed.Seconds())))
		}
		if s.stage != "" {
			line += " " + dim("· "+s.stage)
		}
		lines[i] = line
	}
	// The cursor stays at the end of the last line: the next line printed
	// replaces them.
	fmt.Fprint(os.Stderr, strings.Join(lines, "\n"))
	o.drawn = len(lines)
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var spinnerOnce sync.Once

// spin animates the spinner, once.
func spin() {
	spinnerOnce.Do(func() {
		go func() {
			tick := time.NewTicker(80 * time.Millisecond)
			defer tick.Stop()
			for range tick.C {
				out.Lock()
				if len(out.steps) > 0 {
					out.frame++
					out.draw()
				}
				out.Unlock()
			}
		}()
	})
}

// Step is a step that takes a while. On a terminal its spinner line is
// shown while it runs, and done replaces it with the step's ✓ line; fail
// clears it, since the error printed next says what went wrong.
type Step struct {
	label string
	stage string
	start time.Time
	// hold keeps the lines said in the step (detailIn, warnIn) until it
	// ends, when they print together above its own, so a step running
	// beside others keeps its lines apart from theirs.
	hold bool
	held []string
}

// begin starts the step label names, such as "building web".
func begin(format string, args ...any) *Step {
	s := &Step{label: fmt.Sprintf(format, args...), start: time.Now()}
	if terminalOf().tty {
		out.Lock()
		out.steps = append(out.steps, s)
		out.draw()
		out.Unlock()
		spin()
	}
	return s
}

type stepKey struct{}

// withStep returns ctx carrying s, the step what runs with ctx belongs to.
func withStep(ctx context.Context, s *Step) context.Context {
	return context.WithValue(ctx, stepKey{}, s)
}

// stepOf is the step ctx carries, or nil.
func stepOf(ctx context.Context) *Step {
	s, _ := ctx.Value(stepKey{}).(*Step)
	return s
}

// stage says what the step ctx carries does at the moment, such as
// "compiling Go", after its label in the spinner; without one, the last
// step begun. Without a terminal it does nothing.
func stage(ctx context.Context, format string, args ...any) {
	if !terminalOf().tty {
		return
	}
	out.Lock()
	defer out.Unlock()
	s := stepOf(ctx)
	if s == nil {
		if len(out.steps) == 0 {
			return
		}
		s = out.steps[len(out.steps)-1]
	}
	s.stage = fmt.Sprintf(format, args...)
	out.draw()
}

// detailIn is detail, said in the step ctx carries.
func detailIn(ctx context.Context, format string, args ...any) {
	lineIn(ctx, "  "+dim(fmt.Sprintf(format, args...)))
}

// warnIn is warn, said in the step ctx carries.
func warnIn(ctx context.Context, format string, args ...any) {
	lineIn(ctx, yellow("warn")+" "+fmt.Sprintf(format, args...))
}

// lineIn prints one of the tool's lines, or keeps it for later when the
// step ctx carries holds its lines.
func lineIn(ctx context.Context, text string) {
	if s := stepOf(ctx); s != nil && s.hold {
		out.Lock()
		s.held = append(s.held, text)
		out.Unlock()
		return
	}
	line(text)
}

// done ends the step with "✓ message (time, note)", without the note when
// it is empty, under the lines it held.
func (s *Step) done(message, extra string) {
	s.remove()
	elapsed := formatDuration(time.Since(s.start))
	if extra != "" {
		elapsed += ", " + extra
	}
	done("%s %s", message, dim("("+elapsed+")"))
}

// fail ends a step that failed, without a line of its own: only the lines
// it held print.
func (s *Step) fail() { s.remove() }

// remove takes the step's spinner away, if it had one, and prints the
// lines it held.
func (s *Step) remove() {
	out.Lock()
	held := s.held
	s.held = nil
	for i, step := range out.steps {
		if step == s {
			out.steps = append(out.steps[:i], out.steps[i+1:]...)
			out.draw()
			break
		}
	}
	out.Unlock()
	for _, l := range held {
		line(l)
	}
}
