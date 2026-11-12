// Command plugins is a Go app that installs and runs signed TSX packages:
// it verifies greeter.nsgopkg against the root key rootKey, off the
// engine's goroutine, asks the user to grant what its capabilities
// declare, installs it in the app's data directory and shows it in
// tsx.View, which reaches the app only through a host channel of its
// own; or it shows why the package was refused. search.nsgopkg is a
// plugin of functions, which the app loads with tsx.Load and calls.
//
// The packages are the apps in plugin/ and search/, signed by a release
// key of a root whose secret was thrown away. To sign them again, print
// a root and a release key with go tool nsgo keygen, keep their secret
// keys out of the repository, set rootKey to the root's public key, then,
// in this directory:
//
//	NSGO_ROOT_KEY=<root secret> go tool nsgo certify -key <release public> -apps 'dev.oneo.nsgo.plugins.*' -out ~/keys/release.cert
//	export NSGO_RELEASE_KEY=<release secret>
//	go tool nsgo publish -cert ~/keys/release.cert -out greeter.nsgopkg plugin
//	go tool nsgo publish -cert ~/keys/release.cert -entry App.ts -out search.nsgopkg search
package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx"
	"github.com/nsgo-dev/nsgo/tsx/pkg"
)

var (
	//go:embed greeter.nsgopkg
	greeter []byte
	//go:embed search.nsgopkg
	search []byte
)

// rootKey is the public key of the root the packages' release key is
// certified by, as nsgo keygen prints it.
const rootKey = "dA1jQr/NOvprVZ26UmaY+CBd0Zdggq/VqVtIc0KK0x4="

func main() { nsgo.Run(Root, nsgo.WindowSize(480, 480)) }

// Root is the app: the packages, verified by a Trust kept in the app's
// data directory, or the page's storage on the web, so an older build is
// refused once a newer one ran.
func Root(cx *core.Scope) core.Node {
	root, err := pkg.DecodePublicKey(rootKey)
	if err != nil {
		panic(err)
	}
	// The web has no data directory: its trust is kept under the name.
	trust, installed := pkg.NewTrust(), ""
	if dir := cx.DataDir(); dir != "" || runtime.GOOS == "js" {
		if trust, err = pkg.OpenTrust(filepath.Join(dir, "trust")); err != nil {
			panic(err)
		}
		if dir != "" {
			installed = filepath.Join(dir, "plugins")
		}
	}
	trust.AddRoot(root)
	return core.View{Class: "flex-1 p-6 gap-4 bg-white dark:bg-neutral-900", Children: []core.Node{
		core.Text{Class: "text-sm font-semibold text-neutral-500", Content: "Plugin host"},
		Host(cx, greeter, trust, installed),
		Search(cx, search, trust),
	}}
}

// verified is what a Trust said of a package.
type verified struct {
	p   *pkg.Package
	err error
}

// verify verifies data off the engine's goroutine, into a signal.
func verify(cx *core.Scope, data []byte, trust *pkg.Trust) *core.Signal[verified] {
	s := core.NewSignalFunc(cx, verified{}, func(a, b verified) bool { return a == b })
	go func() {
		p, err := trust.Verify(data)
		cx.Post(func() { s.Set(verified{p, err}) })
	}()
	return s
}

// Host shows the package data once trust verifies it and the user grants
// what it declares, installing it in dir ("" keeps it in memory, as the
// web does), and accepts its build once it loaded.
func Host(cx *core.Scope, data []byte, trust *pkg.Trust, dir string) core.Node {
	result := verify(cx, data, trust)
	granted := core.NewSignal(cx, false)
	// The plugin's channel to the app: the handlers it may call, as its
	// capabilities and the user allow.
	channel := core.NewHost()
	channel.Handle("greet", func(c *core.HostCall) { c.Resolve("Hello from the host") })
	return core.Dynamic(func(cx *core.Scope) core.Node {
		r := result.Get()
		switch {
		case r.err != nil:
			return refused(r.err)
		case r.p == nil:
			return core.Text{TestID: "status", Class: "text-sm text-neutral-500", Content: "Verifying…"}
		case !granted.Get():
			return consent(r.p, func() {
				if err := install(dir, r.p, data); err != nil {
					result.Set(verified{err: err})
					return
				}
				granted.Set(true)
			})
		}
		return core.Catch{
			Try: func(cx *core.Scope) core.Node {
				n := tsx.View{Package: r.p, Sandbox: tsx.Sandbox{Host: channel}}.Render(cx)
				go trust.Accept(r.p)
				return n
			},
			Fallback: func(cx *core.Scope, err error, _ func()) core.Node { return refused(err) },
		}
	})
}

// consent asks the user to grant what p declares.
func consent(p *pkg.Package, install func()) core.Node {
	lines := []core.Node{core.Text{Class: "text-sm text-neutral-900 dark:text-white", Content: fmt.Sprintf("Install %s %s?", p.App.ID, p.App.Version)}}
	for _, l := range describe(p.Capabilities) {
		lines = append(lines, core.Text{Class: "text-sm text-neutral-600 dark:text-neutral-300", Content: "It may " + l})
	}
	lines = append(lines, core.View{TestID: "install", Class: "self-start px-3 py-2 rounded-lg bg-neutral-900 dark:bg-white",
		OnPress:  func(core.PressEvent) { install() },
		Children: []core.Node{core.Text{Class: "text-sm font-medium text-white dark:text-neutral-900", Content: "Install"}}})
	return core.View{TestID: "consent", Class: "gap-2", Children: lines}
}

// describe says what c grants, for people.
func describe(c pkg.Capabilities) []string {
	var out []string
	if c.Network != nil {
		out = append(out, "reach "+strings.Join(c.Network.Hosts, ", "))
	}
	if len(c.Host) > 0 {
		out = append(out, "call the app's "+strings.Join(c.Host, ", "))
	}
	if c.Clipboard {
		out = append(out, "read and write the clipboard")
	}
	if len(out) == 0 {
		out = append(out, "reach nothing outside itself")
	}
	return out
}

// install writes the package to dir whole, then renames it into place,
// so a crash never leaves half of one.
func install(dir string, p *pkg.Package, data []byte) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, p.App.ID+".nsgopkg")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Search loads the plugin of functions data once trust verifies it, and
// shows what its search finds.
func Search(cx *core.Scope, data []byte, trust *pkg.Trust) core.Node {
	result := verify(cx, data, trust)
	found := core.NewSignal(cx, "Searching…")
	return core.Dynamic(func(cx *core.Scope) core.Node {
		r := result.Get()
		if r.err != nil {
			return refused(r.err)
		}
		if r.p != nil {
			plugin, err := tsx.Load(cx, r.p, tsx.Sandbox{})
			if err != nil {
				return refused(err)
			}
			plugin.Call("search", []any{"ed"}, func(v any, err error) {
				if err != nil {
					found.Set(err.Error())
					return
				}
				found.Set(fmt.Sprint("Found ", v))
			})
		}
		return core.Text{TestID: "search", Class: "text-sm text-neutral-600 dark:text-neutral-300", ContentFunc: found.Get}
	})
}

func refused(err error) core.Node {
	return core.Text{TestID: "status", Class: "text-sm text-red-600", Content: "Refused: " + err.Error()}
}
