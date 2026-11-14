package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// project is an app's nsgo.yaml and where it lives.
type project struct {
	Dir string `yaml:"-"`
	// Release strips symbols and debug information for the smallest
	// binaries; nsgo build sets it, nsgo dev does not.
	Release bool `yaml:"-"`
	// idDefault records that nsgo.yaml gave no id, so it follows the name.
	idDefault bool
	// devURL is the dev server's address for targets that pass the app no
	// environment.
	devURL string
	// testBinary builds the package's tests with the nsgotest tag in
	// place of the app, for nsgo test -target.
	testBinary bool
	// bare records that the directory has no nsgo.yaml: a library, or a
	// module's root, that nsgo test and nsgo check take as a tree of
	// packages.
	bare bool
	// steps, when set, are what nsgo test <entry> runs in place of the
	// package's tests: the steps of standard input on the app at entry.
	steps *steps
	// split, set while a web release builds, holds its split bundles.
	split *splitBundles
	// clipParent is the app's ID when this is its App Clip's project
	// (clipProject), and parentClip the app's app_clip.
	clipParent string
	parentClip *appClip
	// released lists the files nsgo build put in the release directory
	// for the target it builds.
	released []string

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
	// Window is no longer nsgo.yaml's: Go code sizes windows.
	Window any `yaml:"window"`
	// Icon names the app's icons; resolveIcons fills in the defaults
	// under assets.
	Icon icons `yaml:"icon"`
	Dev  struct {
		Target string `yaml:"target"`
		// Icon replaces Icon in nsgo dev's builds, field by field.
		Icon icons `yaml:"icon"`
		// HotReload rebuilds and restarts the app when a watched file
		// changes; it defaults to true.
		HotReload *bool `yaml:"hot_reload"`
		// FPS is no longer nsgo.yaml's: apps take --nsgo-fps.
		FPS any `yaml:"fps"`
		// Client builds the dev client in, which connects to nsgo dev's
		// server: nsgo dev sets it. Trace has the client send the server
		// each commit: nsgo dev -trace sets it.
		Client bool `yaml:"-"`
		Trace  bool `yaml:"-"`
		// Highlight has the client outline what each commit changed: nsgo dev -highlight sets it.
		Highlight bool `yaml:"-"`
		// Devtools has the client serve DevTools: nsgo dev -devtools sets it.
		Devtools bool `yaml:"-"`
		// Limits runs a TSX app sandboxed, as tsx.View would, within
		// these limits (tsx.ParseLimits) and its capabilities: nsgo dev
		// -limits sets it.
		Limits string `yaml:"-"`
	} `yaml:"dev"`
	// Theme is a TSX app's theme file, relative to nsgo.yaml: a theme
	// core.ParseTheme reads, NSGo's or a shadcn/ui one, which the
	// compiler gives the app before it mounts.
	Theme string `yaml:"theme"`
	// Tailwind extends Tailwind's default theme for a TSX app's classes,
	// as core.ParseTailwind reads it; the compiler checks it.
	Tailwind yaml.Node `yaml:"tailwind"`
	// Capabilities are what a TSX app's code may reach once signed into a
	// package (nsgo publish), and under nsgo dev -limits.
	Capabilities yaml.Node `yaml:"capabilities"`
	// StatusBar is a TSX app's default status bar on iOS and Android,
	// under its pages' and dialogs': foreground (auto, light or dark) and
	// hidden.
	StatusBar yaml.Node `yaml:"status_bar"`
	// I18n is the app's catalogs: Dir, relative to nsgo.yaml, holds a
	// YAML file per language, Source's with every message. Their
	// languages are the ones the app shows.
	I18n struct {
		Dir    string `yaml:"dir"`
		Source string `yaml:"source"`
	} `yaml:"i18n"`
	// Web is the web target's: Host is a JS module, relative to
	// nsgo.yaml and inside its directory, whose createHost(player) gives
	// the app's page its host.
	Web struct {
		Host string `yaml:"host"`
	} `yaml:"web"`
	// MiniProgram is the mini program targets': AppID holds the mini
	// program's on each platform (wechat, qq, alipay, douyin), and
	// CustomNavigation hides the platform's navigation bar, so the app
	// fills the page and draws its own, clear of the capsule button.
	MiniProgram struct {
		AppID            map[string]string `yaml:"appid"`
		CustomNavigation bool              `yaml:"custom_navigation"`
	} `yaml:"miniprogram"`
	// Apple is the macOS and iOS targets': Info holds string entries for
	// their Info.plist, such as NSMicrophoneUsageDescription.
	Apple struct {
		Info map[string]string `yaml:"info"`
		// AppGroup is the app group an iOS app shares with its widgets,
		// group.<id> by default.
		AppGroup string `yaml:"app_group"`
	} `yaml:"apple"`
	// FileTypes are the kinds of documents the app opens, which its macOS
	// package registers (filetypes.go).
	FileTypes []fileType `yaml:"file_types"`
	// Widgets are the kinds of the app's widgets, which nsgo build puts
	// into its macOS and iOS packages as a widget extension (widgets.go).
	Widgets []widgetKind `yaml:"widgets"`
	// LiveActivities turns on the app's Live Activities, which the iOS
	// widget extension draws and Android shows as notifications
	// (widgets.go).
	LiveActivities liveActivities `yaml:"live_activities"`
	// AppClip is the app's App Clip, which nsgo build nests in its iOS
	// package (appclip.go).
	AppClip *appClip `yaml:"app_clip"`
	// Android is the Android target's: Permissions are the permissions
	// its manifest asks for beyond the runtime's, such as
	// REQUEST_INSTALL_PACKAGES, which a name without a dot is under
	// android.permission. TargetSDK replaces the runtime's
	// targetSdkVersion, for the level Google Play requires.
	Android struct {
		Permissions []string `yaml:"permissions"`
		TargetSDK   int      `yaml:"target_sdk"`
	} `yaml:"android"`
	// MacOS is the macOS target's disk image: DMGBackground, a PNG with
	// an optional @2x twin, is the picture behind its window, which takes
	// its size, else a built-in arrow; DMGIcon, an .icns or a square PNG,
	// is the volume's icon, else the app's. Category is the app's
	// LSApplicationCategoryType, such as public.app-category.productivity,
	// which the Mac App Store requires, and Entitlements are what a store
	// build's sandboxed app claims beyond its defaults (macstore.go).
	MacOS struct {
		DMGBackground string         `yaml:"dmg_background"`
		DMGIcon       string         `yaml:"dmg_icon"`
		Category      string         `yaml:"category"`
		Entitlements  map[string]any `yaml:"entitlements"`
	} `yaml:"macos"`
	// Windows is the Windows installer's: InstallerIcon and
	// UninstallerIcon, an .ico or a square PNG, are the icons of the
	// setup program and the uninstaller, else the app's. Store is the
	// Microsoft Store's package identity, as Partner Center lists it, which
	// a store build's MSIX carries (msix.go): the package's name, its
	// publisher (CN=...) and the publisher's display name.
	Windows struct {
		InstallerIcon   string `yaml:"installer_icon"`
		UninstallerIcon string `yaml:"uninstaller_icon"`
		Store           struct {
			Identity      string `yaml:"identity"`
			Publisher     string `yaml:"publisher"`
			PublisherName string `yaml:"publisher_name"`
		} `yaml:"store"`
	} `yaml:"windows"`
	// Linux is the Linux target's: Maintainer is who its Debian and Arch
	// packages name as theirs, "Name <email>", else the app's name.
	// Summary, License, an SPDX expression, and FlatpakRuntime, GNOME's
	// runtime version, are what a store build's AppStream metadata and
	// Flatpak manifest say (flatpak.go).
	Linux struct {
		Maintainer     string `yaml:"maintainer"`
		Summary        string `yaml:"summary"`
		License        string `yaml:"license"`
		FlatpakRuntime string `yaml:"flatpak_runtime"`
	} `yaml:"linux"`
	// Releases is where the app is published: GitHub, a repository
	// ("owner/name") whose releases, tagged TagPrefix (v by default) and
	// the version, hold build.output's releases directory, which the
	// install scripts download from (release.go); by default the one the
	// Git remote origin names.
	Releases struct {
		GitHub    string  `yaml:"github"`
		TagPrefix *string `yaml:"tag_prefix"`
	} `yaml:"release"`
	// Updates has the app update the way its platform does (updates.go).
	Updates updatesConfig `yaml:"updates"`
	// Mobile is the iOS and Android targets': ExcludeResources are paths in
	// resources/ their packages leave out, such as what only desktops run.
	Mobile struct {
		ExcludeResources []string `yaml:"exclude_resources"`
	} `yaml:"mobile"`
	Sign  signing `yaml:"sign"`
	Build struct {
		Output  string   `yaml:"output"`
		Targets []string `yaml:"targets"`
		// Open shows the output directory once the build finishes; it
		// defaults to false.
		Open *bool `yaml:"open"`
		// FPS is no longer nsgo.yaml's: apps take --nsgo-fps.
		FPS any `yaml:"fps"`
		// Installers packages the desktop targets for users to install
		// (installer.go): a disk image on macOS, an NSIS installer
		// on Windows, Debian and Arch packages on Linux; it defaults to
		// true.
		Installers *bool `yaml:"installers"`
		// Jobs is how many targets build at once; 0, the default, is
		// half the CPUs.
		Jobs int `yaml:"jobs"`
		// run starts the package built for this machine afterwards (-run),
		// on device (-device), an iPhone signed for team (-team);
		// embed packages the app for host apps of their own (-embed).
		run, embed bool
		// all records that the targets were given as all.
		all          bool
		dynamic      bool
		device, team string
	} `yaml:"build"`
}

// icons are an app's icon files.
type icons struct {
	// Apple is an Icon Composer .icon for macOS, iOS and an Apple Watch
	// without an icon of its own, or an .icns for macOS.
	Apple string `yaml:"apple"`
	// PNG is a square image for every other target, and for Apple targets
	// Apple does not cover.
	PNG string `yaml:"png"`
	// TVOS is the Apple TV's: Xcode's .brandassets with its layered icon
	// and top shelf images, or a wide image that fills them. Without it
	// the PNG layers as the app icon and top shelf images.
	TVOS string `yaml:"tvos"`
	// WatchOS is the Apple Watch's: an Icon Composer .icon drawn in
	// circles, or a square image. Without it Apple is used, then PNG.
	WatchOS string `yaml:"watchos"`
}

// openOutput reports whether nsgo build opens its output directory.
func (p *project) openOutput() bool {
	return p.Build.Open != nil && *p.Build.Open
}

// installers reports whether nsgo build packages desktop targets for
// users to install, unless build.installers turns it off.
func (p *project) installers() bool {
	return p.Build.Installers == nil || *p.Build.Installers
}

// hotReload reports whether nsgo dev rebuilds the app on changes, unless
// dev.hot_reload turns it off; without it, dev runs the app once and exits
// when the app does.
func (p *project) hotReload() bool {
	return p.Dev.HotReload == nil || *p.Dev.HotReload
}

// loadOrDir is load, or for a directory without nsgo.yaml a bare project
// named after it.
func loadOrDir(dir string) (*project, error) {
	p, err := load(dir)
	if !errors.Is(err, os.ErrNotExist) {
		return p, err
	}
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("no directory %s", dir)
	}
	return &project{Dir: abs, Name: filepath.Base(abs), bare: true}, nil
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
	if _, err := p.capabilities(); err != nil {
		return nil, err
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
	if p.Window != nil {
		return nil, fmt.Errorf("nsgo.yaml: window is set in Go now: nsgo.Run(Root, nsgo.WindowSize(width, height)), or desktop.WindowOptions")
	}
	if p.Dev.FPS != nil || p.Build.FPS != nil {
		return nil, fmt.Errorf("nsgo.yaml: fps is gone: start the app with --nsgo-fps, or run nsgo dev -fps")
	}
	if owner, name, ok := strings.Cut(p.Releases.GitHub, "/"); p.Releases.GitHub != "" && (!ok || owner == "" || name == "" || strings.Contains(name, "/")) {
		return nil, fmt.Errorf("nsgo.yaml: release.github %q is not owner/name", p.Releases.GitHub)
	}
	if p.Dev.Target == "" {
		p.Dev.Target = "native"
	}
	if p.Build.Output == "" {
		p.Build.Output = "dist"
	}
	p.Build.Output = p.resolve(p.Build.Output)
	for i, perm := range p.Android.Permissions {
		if !strings.Contains(perm, ".") {
			p.Android.Permissions[i] = "android.permission." + perm
		}
	}
	if p.I18n.Dir == "" {
		p.I18n.Dir = "locales"
	}
	p.I18n.Dir = p.resolve(p.I18n.Dir)
	if p.I18n.Source == "" {
		p.I18n.Source = "en"
	}
	if err := p.validateFileTypes(); err != nil {
		return nil, fmt.Errorf("nsgo.yaml: %w", err)
	}
	if err := p.validateWidgets(); err != nil {
		return nil, fmt.Errorf("nsgo.yaml: %w", err)
	}
	if err := p.validateAppClip(); err != nil {
		return nil, fmt.Errorf("nsgo.yaml: %w", err)
	}
	if err := p.validateUpdates(); err != nil {
		return nil, fmt.Errorf("nsgo.yaml: %w", err)
	}
	for _, path := range []*string{&p.Sign.Profile, &p.Sign.WidgetsProfile, &p.Sign.ClipProfile, &p.Sign.MacOSProfile, &p.Sign.Keystore} {
		if *path != "" {
			*path = p.resolve(*path)
		}
	}
	if len(p.Build.Targets) == 0 {
		p.Build.Targets = []string{"macos_" + goArchName(runtime.GOARCH)}
	}
	p.Build.all = slices.Contains(p.Build.Targets, "all")
	expanded, err := expandTargets(p.Build.Targets)
	if err != nil {
		return nil, fmt.Errorf("nsgo.yaml: %w", err)
	}
	p.Build.Targets = expanded
	if err := p.resolveIcons(); err != nil {
		return nil, fmt.Errorf("nsgo.yaml: %w", err)
	}
	if err := p.resolveInstallerFiles(); err != nil {
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

// corePath is package core's, which holds the app's identity (core.App).
const corePath = "github.com/nsgo-dev/nsgo/core"

// ldflags builds nsgo.yaml's name, ID, version and build number into
// core.App, and passes the rest of what nsgo.Run takes from it.
func (p *project) ldflags(extra ...string) string {
	const pkg = "github.com/nsgo-dev/nsgo"
	flags := fmt.Sprintf("-X '%s.appName=%s' -X '%s.appID=%s' -X '%s.appVersion=%s' -X '%s.appBuild=%s'",
		corePath, p.Name, corePath, p.ID, corePath, p.Version, corePath, p.BundleVersion())
	if p.Appearance != "" {
		flags += fmt.Sprintf(" -X %s.appearance=%s", pkg, p.Appearance)
	}
	if p.devURL != "" {
		flags += fmt.Sprintf(" -X '%s.devServer=%s' -X '%s.devServer=%s'", pkg, p.devURL, compilerPath, p.devURL)
	}
	if p.Dev.Client && p.Dev.Trace {
		flags += fmt.Sprintf(" -X %s.devTrace=1", pkg)
	}
	if p.Dev.Client && p.Dev.Highlight {
		flags += fmt.Sprintf(" -X %s.devHighlight=1", pkg)
	}
	if p.Dev.Client && p.Dev.Devtools {
		flags += fmt.Sprintf(" -X %s.devDevtools=1", pkg)
	}
	if p.Dev.Client && p.Dev.Limits != "" {
		flags += fmt.Sprintf(" -X '%s.devLimits=%s'", compilerPath, p.Dev.Limits)
		if caps, _ := p.capabilities(); caps != nil {
			data, _ := json.Marshal(caps)
			flags += fmt.Sprintf(" -X %s.devCapabilities=%s", compilerPath, base64.RawURLEncoding.EncodeToString(data))
		}
	}
	if p.Release {
		flags += fmt.Sprintf(" -X %s.appRelease=1 -s -w", corePath)
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
