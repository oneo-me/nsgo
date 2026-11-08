// Command plugins is a Go app that runs a signed TSX package: it verifies
// plugin.nsgopkg against the root key in root.pub, off the engine's
// goroutine, then shows it in tsx.View, or why it was refused.
//
// The package is the app in plugin/, signed by a release key of a root
// whose secret was thrown away. To sign it again, make keys of your own
// outside the repository, then, in this directory:
//
//	go tool nsgo certify -root ~/keys/root.key -key ~/keys/release.pub -apps 'dev.oneo.nsgo.plugins.*'
//	go tool nsgo publish -key ~/keys/release.key -cert ~/keys/release.cert -out plugin.nsgopkg plugin
//	cp ~/keys/root.pub .
package main

import (
	_ "embed"
	"path/filepath"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx"
	"github.com/oneo-me/nsgo/tsx/pkg"
)

var (
	//go:embed plugin.nsgopkg
	plugin []byte
	//go:embed root.pub
	rootKey string
)

func main() { nsgo.Run(Root, nsgo.WindowSize(480, 360)) }

// Root is the app: the package, verified by a Trust kept in the app's
// data directory, so an older build is refused once a newer one ran.
func Root(cx *core.Scope) core.Node {
	root, err := pkg.DecodePublicKey(rootKey)
	if err != nil {
		panic(err)
	}
	trust := pkg.NewTrust()
	if dir := cx.DataDir(); dir != "" {
		if trust, err = pkg.OpenTrust(filepath.Join(dir, "trust")); err != nil {
			panic(err)
		}
	}
	trust.AddRoot(root)
	return Host(cx, plugin, trust)
}

// Host shows the package data once trust verifies it, and accepts its
// build once it loaded.
func Host(cx *core.Scope, data []byte, trust *pkg.Trust) core.Node {
	type result struct {
		p   *pkg.Package
		err error
	}
	verified := core.NewSignalFunc(cx, result{}, func(a, b result) bool { return a == b })
	go func() {
		p, err := trust.Verify(data)
		cx.Post(func() { verified.Set(result{p, err}) })
	}()
	return core.View{Class: "flex-1 p-6 gap-4 bg-white dark:bg-neutral-900", Children: []core.Node{
		core.Text{Class: "text-sm font-semibold text-neutral-500", Content: "Plugin host"},
		core.Dynamic(func(cx *core.Scope) core.Node {
			r := verified.Get()
			switch {
			case r.err != nil:
				return refused(r.err)
			case r.p == nil:
				return core.Text{TestID: "status", Class: "text-sm text-neutral-500", Content: "Verifying…"}
			}
			return core.Catch{
				Try: func(cx *core.Scope) core.Node {
					n := tsx.View{Package: r.p}.Render(cx)
					go trust.Accept(r.p)
					return n
				},
				Fallback: func(cx *core.Scope, err error, _ func()) core.Node { return refused(err) },
			}
		}),
	}}
}

func refused(err error) core.Node {
	return core.Text{TestID: "status", Class: "text-sm text-red-600", Content: "Refused: " + err.Error()}
}
