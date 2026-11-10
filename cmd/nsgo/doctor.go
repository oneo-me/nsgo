package main

// nsgo doctor: what this machine has of the tools the targets need, then
// whether it builds each target and runs its apps (dev, test and build
// -run). It only looks; it installs, boots and starts nothing.

import (
	"context"
	"fmt"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// minGo is the oldest Go that builds NSGo apps, go.mod's go line.
const minGo = "go1.26"

// machine is what doctor found on this machine. An empty string is a
// tool it did not find.
type machine struct {
	goos, goarch string
	system       string // the system's name and version, "macOS 26.1"
	goVersion    string // go env GOVERSION
	xcode        string // Xcode's version
	sdks         map[string]bool
	simulators   map[string]int // available simulators by OS, "iOS"
	appleDevices map[string]int // paired devices by OS
	adb          string
	android      []androidDevice
	ndk          string
	browser      string
	linuxLibs    []string // what of GTK 4, Pango and Cairo is missing
	makensis     string
	miniTools    map[string]bool // mini program platforms whose tool opens projects
}

// androidDevice is one device adb lists, with its ABI.
type androidDevice struct{ serial, abi string }

// verdict is whether this machine builds or runs a target, and why not;
// elsewhere when no tool could make it, as for another system's apps.
type verdict struct {
	ok, elsewhere bool
	why           string
}

func yes() verdict                 { return verdict{ok: true} }
func no(why string) verdict        { return verdict{why: why} }
func elsewhere(why string) verdict { return verdict{elsewhere: true, why: why} }
func when(ok bool, why string) verdict {
	if ok {
		return yes()
	}
	return no(why)
}

func doctor(ctx context.Context) error {
	s := begin("looking at this machine")
	m := probe(ctx)
	s.remove()

	emit("")
	title := bold("NSGo")
	if v := toolVersion(); v != "" {
		title += " " + dim(v)
	}
	line(title + " " + dim("· "+m.system+" "+goArchName(m.goarch)))
	emit("")
	for _, t := range m.tools() {
		switch {
		case t.ok:
			done("%s", t.why)
		case t.elsewhere:
			line(dim("–") + " " + t.why)
		default:
			line(red("✗") + " " + t.why)
		}
	}
	emit("")

	width := 0
	for _, t := range targets {
		width = max(width, len(t))
	}
	line(dim(fmt.Sprintf("%-*s  build  run", width, "target")))
	mark := func(v verdict) string {
		switch {
		case v.ok:
			return green("✓")
		case v.elsewhere:
			return dim("–")
		}
		return red("✗")
	}
	for _, t := range targets {
		build, run := m.build(t), m.run(t)
		why := build.why
		if build.ok {
			why = run.why
		}
		if why != "" {
			why = "  " + dim(why)
		}
		line(fmt.Sprintf("%-*s  %s      %s", width, t, mark(build), mark(run)) + why)
	}
	emit("")
	if m.goVersion == "" || version.Compare(m.goVersion, minGo) < 0 {
		return fmt.Errorf("NSGo needs Go %s or later: https://go.dev/dl/", strings.TrimPrefix(minGo, "go"))
	}
	return nil
}

// tools are the lines saying what doctor found: ok for a tool found, else
// what lacking it costs, elsewhere when only some targets need it.
func (m *machine) tools() []verdict {
	var out []verdict
	switch {
	case m.goVersion == "":
		out = append(out, no("no Go on PATH: every target needs Go "+strings.TrimPrefix(minGo, "go")+" or later"))
	case version.Compare(m.goVersion, minGo) < 0:
		out = append(out, no(fmt.Sprintf("Go %s is too old: NSGo needs %s or later", strings.TrimPrefix(m.goVersion, "go"), strings.TrimPrefix(minGo, "go"))))
	default:
		out = append(out, yes().with("Go "+strings.TrimPrefix(m.goVersion, "go")))
	}
	if m.goos == "darwin" {
		if m.xcode != "" {
			out = append(out, yes().with(m.xcode+simulatorCounts(m.simulators)))
		} else {
			out = append(out, elsewhere("no Xcode: iOS, tvOS and watchOS need it (xcode-select -s /Applications/Xcode.app)"))
		}
	}
	if m.goos == "linux" {
		if len(m.linuxLibs) == 0 {
			out = append(out, yes().with("GTK 4, Pango and Cairo"))
		} else {
			out = append(out, elsewhere("no "+strings.Join(m.linuxLibs, ", ")+": Linux apps need GTK 4.14 or newer, Pango and Cairo"))
		}
	}
	if m.adb != "" {
		var serials []string
		for _, d := range m.android {
			serials = append(serials, d.serial+" "+dim("("+d.abi+")"))
		}
		out = append(out, yes().with("adb "+dim(m.adb)+", Android devices: "+list(serials)))
	} else {
		out = append(out, elsewhere("no adb: running on Android needs the Android SDK's platform-tools"))
	}
	if m.ndk != "" {
		out = append(out, yes().with("Android NDK "+dim(m.ndk)))
	} else {
		out = append(out, elsewhere("no Android NDK: only android_x64 needs one (sdkmanager \"ndk;27.2.12479018\")"))
	}
	if m.browser != "" {
		out = append(out, yes().with("browser "+dim(m.browser)))
	} else {
		out = append(out, elsewhere("no Chrome, Chromium or Edge: nsgo test -target web needs one, or NSGO_BROWSER"))
	}
	if m.makensis != "" {
		out = append(out, yes().with("makensis "+dim(m.makensis)))
	} else {
		out = append(out, elsewhere("no makensis: Windows packages get their installer's script, not the installer"))
	}
	return out
}

func (v verdict) with(why string) verdict {
	v.why = why
	return v
}

// simulatorCounts says how many simulators Xcode has for each OS.
func simulatorCounts(sims map[string]int) string {
	var parts []string
	for _, sys := range []*appleOS{iOS, tvOS, watchOS} {
		if n := sims[sys.name]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, sys.name))
		}
	}
	if len(parts) == 0 {
		return dim(", no simulators")
	}
	return dim(", simulators: " + strings.Join(parts, ", "))
}

// build says whether this machine builds target's package.
func (m *machine) build(target string) verdict {
	if _, _, ok := appleOSOf(target); ok && m.goos != "darwin" {
		return elsewhere("needs a Mac with Xcode")
	}
	if m.goVersion == "" || version.Compare(m.goVersion, minGo) < 0 {
		return no("needs Go " + strings.TrimPrefix(minGo, "go"))
	}
	if sys, sim, ok := appleOSOf(target); ok {
		switch {
		case m.xcode == "":
			return no("needs Xcode")
		case !m.sdks[sys.sdkName(sim)]:
			return no("Xcode has no " + sys.sdkName(sim) + " SDK (Xcode, Settings, Components)")
		}
		return yes()
	}
	if target == "android_x64" {
		return when(m.ndk != "", "needs an Android NDK")
	}
	return yes()
}

// run says whether this machine runs target's apps, with what is missing.
func (m *machine) run(target string) verdict {
	arch := goArchName(m.goarch)
	if sys, sim, ok := appleOSOf(target); ok {
		switch {
		case m.goos != "darwin":
			return elsewhere("needs a Mac with Xcode")
		case m.xcode == "":
			return no("needs Xcode")
		case sim:
			return when(m.simulators[sys.name] > 0, "add a "+sys.name+" simulator in Xcode")
		}
		return when(m.appleDevices[sys.name] > 0, "connect and pair an "+sys.device+" with Developer Mode on")
	}
	if desktop, a, ok := strings.Cut(target, "_"); ok && desktopSystems[desktop] != [2]string{} {
		switch sys := desktopSystems[desktop]; {
		case m.goos != sys[0] || a != arch:
			return elsewhere("runs on " + sys[1] + " " + a)
		case desktop == "linux" && len(m.linuxLibs) > 0:
			return no("needs GTK 4.14 or newer, Pango and Cairo")
		}
		return yes()
	}
	if a, ok := strings.CutPrefix(target, "android_"); ok {
		if m.adb == "" {
			return no("needs adb")
		}
		want, _ := androidABI(goArch(a))
		return when(slices.ContainsFunc(m.android, func(d androidDevice) bool { return d.abi == want }),
			"connect a "+want+" device or start an emulator")
	}
	if target == "web" {
		return yes()
	}
	if mp, ok := miniProgramPlatformOf(target); ok {
		// WeChat's tool runs on macOS and Windows, the others' only on macOS.
		if m.goos != "darwin" && (mp.name != "wechat" || m.goos != "windows") {
			return elsewhere(mp.title + "'s developer tool runs on macOS")
		}
		return when(m.miniTools[mp.name], "needs "+mp.title+"'s developer tool: "+mp.download)
	}
	return no("unknown target")
}

// desktopSystems are the desktop targets' GOOS and system names.
var desktopSystems = map[string][2]string{
	"macos":   {"darwin", "macOS"},
	"windows": {"windows", "Windows"},
	"linux":   {"linux", "Linux"},
}

// probe looks at the machine, the slow parts at once.
func probe(ctx context.Context) *machine {
	m := &machine{goos: runtime.GOOS, goarch: runtime.GOARCH, system: systemName(ctx),
		sdks: map[string]bool{}, simulators: map[string]int{}, appleDevices: map[string]int{}, miniTools: map[string]bool{}}
	var wg sync.WaitGroup
	wg.Go(func() { m.goVersion, _ = output(ctx, "go", "env", "GOVERSION") })
	if m.goos == "darwin" {
		wg.Go(func() { m.probeXcode(ctx) })
	}
	wg.Go(func() { m.probeAndroid(ctx) })
	if path, err := findBrowser(); err == nil {
		m.browser = path
	}
	if path, err := exec.LookPath("makensis"); err == nil {
		m.makensis = path
	}
	if m.goos == "linux" {
		m.linuxLibs = missingLinuxLibs()
	}
	for _, mp := range miniProgramPlatforms {
		m.miniTools[mp.name] = canOpenMiniProgram(mp)
	}
	wg.Wait()
	return m
}

func (m *machine) probeXcode(ctx context.Context) {
	out, err := output(ctx, "xcodebuild", "-version")
	if err != nil {
		return
	}
	m.xcode, _, _ = strings.Cut(out, "\n")
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, sys := range []*appleOS{iOS, tvOS, watchOS} {
		for _, sim := range []bool{false, true} {
			wg.Go(func() {
				_, err := output(ctx, "xcrun", "--sdk", sys.sdkName(sim), "--show-sdk-path")
				mu.Lock()
				m.sdks[sys.sdkName(sim)] = err == nil
				mu.Unlock()
			})
		}
		wg.Go(func() {
			sims, _ := simulators(ctx, sys)
			mu.Lock()
			m.simulators[sys.name] = len(sims)
			mu.Unlock()
		})
	}
	wg.Go(func() {
		devices, _ := iosDevices(ctx)
		mu.Lock()
		for _, d := range devices {
			if d.Paired {
				m.appleDevices[d.Platform]++
			}
		}
		mu.Unlock()
	})
	wg.Wait()
}

func (m *machine) probeAndroid(ctx context.Context) {
	m.ndk, _ = findNDK()
	adb, err := findAdb()
	if err != nil {
		return
	}
	m.adb = adb
	out, err := output(ctx, adb, "devices")
	if err != nil {
		return
	}
	for _, serial := range parseAdbDevices(out) {
		abi, _ := output(ctx, adb, "-s", serial, "shell", "getprop", "ro.product.cpu.abi")
		m.android = append(m.android, androidDevice{serial: serial, abi: abi})
	}
}

// parseAdbDevices reads the serials of the devices adb devices lists as
// ready, leaving out unauthorized and offline ones.
func parseAdbDevices(out string) []string {
	var serials []string
	for _, l := range strings.Split(out, "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[1] == "device" {
			serials = append(serials, f[0])
		}
	}
	return serials
}

// systemName is the machine's system and its version, as its user knows
// them.
func systemName(ctx context.Context) string {
	switch runtime.GOOS {
	case "darwin":
		if v, err := output(ctx, "sw_vers", "-productVersion"); err == nil {
			return "macOS " + v
		}
		return "macOS"
	case "windows":
		return "Windows"
	case "linux":
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			if name := osRelease(string(data)); name != "" {
				return name
			}
		}
		return "Linux"
	}
	return runtime.GOOS
}

// osRelease is os-release's PRETTY_NAME.
func osRelease(data string) string {
	for _, l := range strings.Split(data, "\n") {
		if v, ok := strings.CutPrefix(l, "PRETTY_NAME="); ok {
			if u, err := strconv.Unquote(v); err == nil {
				return u
			}
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}

// linuxLibs are the libraries every Linux app opens, by what the user
// knows them as, with the files the Linux backend tries.
var linuxLibs = []struct {
	name  string
	files []string
}{
	{"GTK 4", []string{"libgtk-4.so.1", "libgtk-4.so.0"}},
	{"Pango", []string{"libpango-1.0.so.0"}},
	{"PangoCairo", []string{"libpangocairo-1.0.so.0"}},
	{"Cairo", []string{"libcairo.so.2"}},
}

// missingLinuxLibs lists what of linuxLibs no library directory holds. A
// GTK older than 4.14 counts as missing.
func missingLinuxLibs() []string {
	dirs := filepath.SplitList(os.Getenv("LD_LIBRARY_PATH"))
	for _, root := range []string{"/usr/lib", "/usr/lib64", "/lib", "/lib64", "/usr/local/lib"} {
		dirs = append(dirs, root)
		multiarch, _ := filepath.Glob(filepath.Join(root, "*-linux-gnu*"))
		dirs = append(dirs, multiarch...)
	}
	var missing []string
	for _, lib := range linuxLibs {
		found := ""
		for _, dir := range dirs {
			for _, file := range lib.files {
				if _, err := os.Stat(filepath.Join(dir, file)); err == nil && found == "" {
					found = filepath.Join(dir, file)
				}
			}
		}
		switch {
		case found == "":
			missing = append(missing, lib.name)
		case lib.name == "GTK 4":
			if real, err := filepath.EvalSymlinks(found); err == nil && !gtkNewEnough(filepath.Base(real)) {
				missing = append(missing, "GTK 4.14")
			}
		}
	}
	return missing
}

// gtkNewEnough reads GTK's version from its library's real name,
// libgtk-4.so.1.1400.2 for 4.14.2, and reports whether it is 4.14 or
// newer; a name it cannot read passes.
func gtkNewEnough(name string) bool {
	_, rest, ok := strings.Cut(name, ".so.1.")
	if !ok {
		return true
	}
	minor, _, _ := strings.Cut(rest, ".")
	n, err := strconv.Atoi(minor)
	return err != nil || n/100 >= 14
}
