package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"gopkg.in/yaml.v3"
)

// project is an app's nsgo.yaml and where it lives.
type project struct {
	Dir string `yaml:"-"`
	// Release strips symbols and debug information for the smallest
	// binaries; nsgo build sets it, nsgo dev does not.
	Release bool `yaml:"-"`
	// Perf shows the performance monitor: nsgo dev sets it from Dev.FPS
	// and nsgo build from Build.FPS.
	Perf bool `yaml:"-"`
	// idDefault records that nsgo.yaml gave no id, so it follows the name.
	idDefault bool
	// devURL is the dev server's address for targets that pass the app no
	// environment.
	devURL string
	// testBinary builds the package's tests with the nsgotest tag in
	// place of the app, for nsgo test -target.
	testBinary bool

	Name    string `yaml:"name"`
	ID      string `yaml:"id"`
	Version string `yaml:"version"`
	// BuildNumber numbers builds of a version, as stores want each upload
	// numbered: CFBundleVersion and Android's versionCode. Without it
	// they come from Version.
	BuildNumber int `yaml:"build_number"`
	// Appearance starts the app light or dark whatever the system's, its
	// launch screen and window included; empty follows the system.
	Appearance string `yaml:"appearance"`
	Window     struct {
		Width  int `yaml:"width"`
		Height int `yaml:"height"`
	} `yaml:"window"`
	// Icon names the app's icons; resolveIcons fills in the defaults
	// under assets.
	Icon icons `yaml:"icon"`
	Dev  struct {
		Target string `yaml:"target"`
		// Icon replaces Icon in nsgo dev's builds, field by field.
		Icon icons `yaml:"icon"`
		// HotReload rebuilds and restarts the app when a watched file
		// changes; it defaults to false.
		HotReload *bool `yaml:"hot_reload"`
		// FPS shows the performance monitor over dev's builds; it
		// defaults to false.
		FPS bool `yaml:"fps"`
		// Client builds the dev client in, which connects to nsgo dev's
		// server: nsgo dev sets it. Trace has the client send the server
		// each commit: nsgo dev -trace sets it.
		Client bool `yaml:"-"`
		Trace  bool `yaml:"-"`
	} `yaml:"dev"`
	// Web is the web target's: Host is a JS module, relative to
	// nsgo.yaml and inside its directory, whose createHost(player) gives
	// the app's page its host.
	Web struct {
		Host string `yaml:"host"`
	} `yaml:"web"`
	Sign  signing `yaml:"sign"`
	Build struct {
		Output  string   `yaml:"output"`
		Targets []string `yaml:"targets"`
		// Open shows the output directory once the build finishes; it
		// defaults to false.
		Open *bool `yaml:"open"`
		// FPS shows the performance monitor over the packages; it
		// defaults to false.
		FPS bool `yaml:"fps"`
		// run starts the package built for this machine afterwards (-run),
		// on device (-device), an iPhone signed for team (-team);
		// embed packages the app for host apps of their own (-embed).
		run, embed   bool
		dynamic      bool
		device, team string
	} `yaml:"build"`
}

// icons are an app's icon files.
type icons struct {
	// Apple is an Icon Composer .icon for macOS and iOS, or an .icns for
	// macOS.
	Apple string `yaml:"apple"`
	// PNG is a square image for every other target, and for Apple targets
	// Apple does not cover.
	PNG string `yaml:"png"`
}

// openOutput reports whether nsgo build opens its output directory.
func (p *project) openOutput() bool {
	return p.Build.Open != nil && *p.Build.Open
}

// hotReload reports whether nsgo dev rebuilds the app on changes; without
// it, dev runs the app once and exits when the app does.
func (p *project) hotReload() bool {
	return p.Dev.HotReload != nil && *p.Dev.HotReload
}

func load(dir string) (*project, error) {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(abs, "nsgo.yaml"))
	if err != nil {
		return nil, err
	}
	p := &project{Dir: abs}
	if err := yaml.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("nsgo.yaml: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("nsgo.yaml: name is required")
	}
	if p.ID == "" {
		p.ID = generatedID(p.Name)
		p.idDefault = true
	}
	if p.Version == "" {
		p.Version = "0.0.0"
	}
	switch p.Appearance {
	case "", "light", "dark":
	default:
		return nil, fmt.Errorf("nsgo.yaml: appearance is %q; use light, dark or leave it out", p.Appearance)
	}
	if p.Window.Width == 0 || p.Window.Height == 0 {
		p.Window.Width, p.Window.Height = 480, 640
	}
	if p.Dev.Target == "" {
		p.Dev.Target = "native"
	}
	if p.Build.Output == "" {
		p.Build.Output = "dist"
	}
	p.Build.Output = p.resolve(p.Build.Output)
	for _, path := range []*string{&p.Sign.Profile, &p.Sign.Keystore} {
		if *path != "" {
			*path = p.resolve(*path)
		}
	}
	if len(p.Build.Targets) == 0 {
		p.Build.Targets = []string{"macos_" + goArchName(runtime.GOARCH)}
	}
	expanded, err := expandTargets(p.Build.Targets)
	if err != nil {
		return nil, fmt.Errorf("nsgo.yaml: %w", err)
	}
	p.Build.Targets = expanded
	if err := p.resolveIcons(); err != nil {
		return nil, fmt.Errorf("nsgo.yaml: %w", err)
	}
	return p, nil
}

// generatedID is the bundle ID of a project whose nsgo.yaml gives none.
func generatedID(name string) string {
	return "dev.nsgo." + name
}

// resolve makes a path from nsgo.yaml or a flag absolute against the app's
// directory.
func (p *project) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(p.Dir, path)
}

// cache is the project's directory for development builds.
func (p *project) cache(parts ...string) string {
	return filepath.Join(append([]string{p.Dir, ".nsgo"}, parts...)...)
}

// ldflags passes nsgo.yaml's name, ID and window size, and the performance
// monitor, to nsgo.Run.
func (p *project) ldflags(extra ...string) string {
	const pkg = "github.com/oneo-me/nsgo"
	flags := fmt.Sprintf("-X '%s.name=%s' -X '%s.id=%s' -X %s.width=%d -X %s.height=%d",
		pkg, p.Name, pkg, p.ID, pkg, p.Window.Width, pkg, p.Window.Height)
	if p.Perf {
		flags += fmt.Sprintf(" -X '%s.fps=1'", pkg)
	}
	if p.Appearance != "" {
		flags += fmt.Sprintf(" -X %s.appearance=%s", pkg, p.Appearance)
	}
	if p.devURL != "" {
		flags += fmt.Sprintf(" -X '%s.devServer=%s'", pkg, p.devURL)
	}
	if p.Dev.Client && p.Dev.Trace {
		flags += fmt.Sprintf(" -X %s.devTrace=1", pkg)
	}
	if p.Release {
		flags += " -s -w"
	}
	for _, e := range extra {
		flags += " " + e
	}
	return flags
}

// BundleVersion is the build's number: BuildNumber, else Version.
func (p *project) BundleVersion() string {
	if p.BuildNumber > 0 {
		return strconv.Itoa(p.BuildNumber)
	}
	return p.Version
}

// goArchName maps Go's amd64 to x64, the name packages use.
func goArchName(arch string) string {
	if arch == "amd64" {
		return "x64"
	}
	return arch
}
