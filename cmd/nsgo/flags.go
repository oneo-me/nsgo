package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

// override is a flag that remembers whether the user gave it, so nsgo dev
// and nsgo build replace nsgo.yaml's values only with flags that were
// passed.
type override[T any] struct {
	value T
	set   bool
	parse func(string) (T, error)
}

func (o *override[T]) String() string { return fmt.Sprint(o.value) }

func (o *override[T]) Set(s string) error {
	v, err := o.parse(s)
	if err != nil {
		return err
	}
	o.value, o.set = v, true
	return nil
}

// replace stores the flag's value in dst when the user gave it.
func (o *override[T]) replace(dst *T) {
	if o.set {
		*dst = o.value
	}
}

// stringFlag and intFlag register a flag that replaces a value from
// nsgo.yaml.
func stringFlag(fs *flag.FlagSet, name, usage string) *override[string] {
	return bindOverride(fs, name, usage, func(s string) (string, error) { return s, nil })
}

func intFlag(fs *flag.FlagSet, name, usage string) *override[int] {
	return bindOverride(fs, name, usage, strconv.Atoi)
}

func bindOverride[T any](fs *flag.FlagSet, name, usage string, parse func(string) (T, error)) *override[T] {
	o := &override[T]{parse: parse}
	fs.Var(o, name, usage)
	return o
}

// boolOverride is an override[bool] that -flag alone sets to true.
type boolOverride struct{ override[bool] }

func (o *boolOverride) IsBoolFlag() bool { return true }

func boolFlag(fs *flag.FlagSet, name, usage string) *boolOverride {
	o := &boolOverride{override[bool]{parse: strconv.ParseBool}}
	fs.Var(o, name, usage)
	return o
}

// overrides are the app values nsgo.yaml gives, as flags shared by nsgo dev
// and nsgo build.
type overrides struct {
	name, id, version *override[string]
	width, height     *override[int]
	buildNumber       *override[int]
}

func bindOverrides(fs *flag.FlagSet) *overrides {
	return &overrides{
		name:        stringFlag(fs, "name", "the app `name` (default: nsgo.yaml's name)"),
		id:          stringFlag(fs, "id", "the bundle `id` (default: nsgo.yaml's id)"),
		version:     stringFlag(fs, "version", "the app `version` (default: nsgo.yaml's version)"),
		width:       intFlag(fs, "width", "the window `width` (default: nsgo.yaml's window.width)"),
		height:      intFlag(fs, "height", "the window `height` (default: nsgo.yaml's window.height)"),
		buildNumber: intFlag(fs, "build-number", "the build's `number`, CFBundleVersion and Android's versionCode (default: nsgo.yaml's build_number)"),
	}
}

func (o *overrides) apply(p *project) {
	o.name.replace(&p.Name)
	// A generated ID follows the name, as it does when nsgo.yaml is read.
	if p.idDefault {
		p.ID = generatedID(p.Name)
	}
	o.id.replace(&p.ID)
	o.version.replace(&p.Version)
	o.width.replace(&p.Window.Width)
	o.height.replace(&p.Window.Height)
	o.buildNumber.replace(&p.BuildNumber)
}

// devFlags are nsgo dev's flags; those that name an nsgo.yaml value replace
// it when the user gives them.
type devFlags struct {
	*overrides
	target    *override[string]
	hotReload *boolOverride
	fps       *boolOverride
	port      int
	open      bool
	device    string
	team      string
	release   bool
	trace     bool
}

func bindDevFlags(fs *flag.FlagSet) *devFlags {
	f := &devFlags{overrides: bindOverrides(fs)}
	f.target = stringFlag(fs, "target", "`target` to use: native, web, ios, ios_simulator or android (a device or emulator; default: nsgo.yaml's dev.target)")
	f.hotReload = boolFlag(fs, "hot-reload", "rebuild and restart on changes (default: nsgo.yaml's dev.hot_reload)")
	f.fps = boolFlag(fs, "fps", "show the performance monitor (default: nsgo.yaml's dev.fps)")
	fs.IntVar(&f.port, "port", 8080, "the dev server's `port`, which serves the web target; the next free one when taken")
	fs.BoolVar(&f.open, "open", true, "open the web target in the browser")
	fs.StringVar(&f.device, "device", "", "the `device` to run on: an iPhone's name, UDID, serial or ECID, a simulator's name or UDID, or an adb serial or emulator's AVD name (default: the only one connected, or the booted simulator)")
	fs.BoolVar(&f.release, "release", false, "build stripped release binaries, as nsgo build does, keeping dev's performance monitor")
	fs.BoolVar(&f.trace, "trace", false, "print each commit's time and mount items from the running app")
	fs.StringVar(&f.team, "team", "", "the Apple developer `team` the iOS target signs with (default: the team Xcode last used)")
	return f
}

func (f *devFlags) apply(p *project) {
	f.overrides.apply(p)
	f.target.replace(&p.Dev.Target)
	if f.hotReload.set {
		p.Dev.HotReload = &f.hotReload.value
	}
	f.fps.replace(&p.Dev.FPS)
}

// buildFlags are nsgo build's flags; those that name an nsgo.yaml value
// replace it when the user gives them.
type buildFlags struct {
	*overrides
	targets *override[string]
	output  *override[string]
	open    *boolOverride
	fps     *boolOverride
	run     bool
	embed   bool
	dynamic bool
	device  string
	team    string
	sign    signFlags
}

func bindBuildFlags(fs *flag.FlagSet) *buildFlags {
	f := &buildFlags{overrides: bindOverrides(fs)}
	f.targets = stringFlag(fs, "target", "comma-separated `targets`, or all for every platform Go and Xcode build (default: nsgo.yaml's build.targets)")
	f.output = stringFlag(fs, "output", "the `output` directory (default: nsgo.yaml's build.output)")
	f.open = boolFlag(fs, "open", "open the output directory (default: nsgo.yaml's build.open)")
	f.fps = boolFlag(fs, "fps", "show the performance monitor (default: nsgo.yaml's build.fps)")
	fs.BoolVar(&f.run, "run", false, "start the package built for this machine: the app, the iPhone's, simulator's or Android device's, or the site served")
	fs.StringVar(&f.device, "device", "", "the `device` -run starts the package on: an iPhone's name, UDID, serial or ECID, a simulator's name or UDID, or an adb serial or emulator's AVD name (default: the only one connected, or the booted simulator)")
	fs.StringVar(&f.team, "team", "", "the Apple developer `team` -run signs the iPhone's package with (default: the team Xcode last used)")
	fs.BoolVar(&f.embed, "embed", false, "package the app for iOS and Android apps of their own to embed: an XCFramework and an AAR")
	fs.BoolVar(&f.dynamic, "dynamic", false, "with -embed, make the XCFramework of dynamic frameworks, which the host embeds, in place of static libraries")
	f.sign.bind(fs)
	return f
}

func (f *buildFlags) apply(p *project) {
	f.overrides.apply(p)
	if f.targets.set {
		p.Build.Targets = strings.Split(f.targets.value, ",")
	}
	if f.output.set {
		p.Build.Output = p.resolve(f.output.value)
	}
	if f.open.set {
		p.Build.Open = &f.open.value
	}
	f.fps.replace(&p.Build.FPS)
	p.Build.run, p.Build.embed, p.Build.dynamic = f.run, f.embed, f.dynamic
	p.Build.device, p.Build.team = f.device, f.team
	f.sign.apply(p)
}
