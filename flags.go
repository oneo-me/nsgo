package nsgo

import (
	"errors"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/core"
)

// launchFlags are the engine's command-line flags, which any build of an
// app takes, with one dash or two:
//
//	--nsgo-fps               show the performance monitor
//	--nsgo-lang=zh-Hans      run in a locale, as core.Scope.SetLocale does
//	--nsgo-appearance=dark   start light, dark or following the system
//
// They override nsgo.yaml and Run's options. Phones and the web take them
// as the platform launches apps with values: the simulator's and
// devicectl's arguments, an Android intent's extras (--ez nsgo-fps true,
// --es nsgo-lang de) and the page's query (?nsgo-fps&nsgo-lang=de).
type launchFlags struct {
	fps        bool
	lang       string
	appearance core.Appearance
	// appearanceSet is whether --nsgo-appearance was given, as its
	// "system" is the zero Appearance.
	appearanceSet bool
}

// launch holds the flags the app was started with.
var launch launchFlags

// launchArgs are the program's arguments as it started, its launch flags
// among them, which a restart passes again.
var launchArgs []string

// The flags leave os.Args before main runs, so the app's own flag
// parsing never sees them.
func init() {
	if len(os.Args) > 1 {
		launchArgs = append([]string(nil), os.Args[1:]...)
	}
	os.Args = launch.parse(os.Args)
}

// parse takes the engine's flags out of args, a program's arguments, and
// returns the rest. A malformed or unknown one is logged and dropped.
// Arguments after "--" are the app's.
func (f *launchFlags) parse(args []string) []string {
	if len(args) == 0 {
		return args
	}
	rest := []string{args[0]}
	for i, arg := range args[1:] {
		if arg == "--" {
			return append(rest, args[1+i:]...)
		}
		name, ok := strings.CutPrefix(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "nsgo-")
		if !ok || !strings.HasPrefix(arg, "-") {
			rest = append(rest, arg)
			continue
		}
		if err := f.set(name); err != nil {
			log.Printf("nsgo: %s: %v", arg, err)
		}
	}
	return rest
}

// set applies one flag, name=value without its --nsgo- prefix.
func (f *launchFlags) set(flag string) error {
	name, value, hasValue := strings.Cut(flag, "=")
	switch name {
	case "fps":
		on := true
		if hasValue {
			var err error
			if on, err = strconv.ParseBool(value); err != nil {
				return err
			}
		}
		f.fps = on
	case "lang":
		if value == "" {
			return errors.New("needs a locale, as --nsgo-lang=zh-Hans")
		}
		f.lang = value
	case "appearance":
		a, ok := map[string]core.Appearance{"system": core.AppearanceSystem, "light": core.AppearanceLight, "dark": core.AppearanceDark}[value]
		if !ok {
			return errors.New("needs light, dark or system")
		}
		f.appearance, f.appearanceSet = a, true
	default:
		return errors.New("no such flag: --nsgo-fps, --nsgo-lang or --nsgo-appearance")
	}
	return nil
}

// apply puts the flags over Run's options.
func (f *launchFlags) apply(o *options) {
	o.monitor = f.fps
	if f.appearanceSet {
		o.appearance = f.appearance
	}
}

// localized returns root, run in the locale --nsgo-lang names. The flag
// has the last word as the app starts, over a choice the app restores,
// and the user's later choices have theirs.
func (f *launchFlags) localized(root App) App {
	if f.lang == "" {
		return root
	}
	return func(cx *core.Scope) core.Node {
		node := root(cx)
		cx.SetLocale(f.lang)
		return node
	}
}

// localizedApp is localized for the app's function.
func (f *launchFlags) localizedApp(app func(*core.Scope)) func(*core.Scope) {
	if f.lang == "" {
		return app
	}
	return func(cx *core.Scope) {
		app(cx)
		cx.SetLocale(f.lang)
	}
}
