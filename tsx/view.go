package tsx

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx/pkg"
)

// View runs the app of a package a pkg.Trust verified where it is placed
// in a Go app, sandboxed: the app's own screens delivered anew, or a
// plugin. A package that throws as it loads panics, as any component
// does; place it under a core.Catch to show something else.
//
//	p, err := trust.Verify(data) // off the engine's goroutine
//	...
//	core.Catch{Try: tsx.View{Package: p}.Render, Fallback: failed}
type View struct {
	Package *pkg.Package
	// Limits are what the package may use, DefaultLimits when nil.
	Limits *Limits
}

// Render implements core.Node.
func (v View) Render(cx *core.Scope) core.Node {
	if v.Package == nil {
		panic("tsx: a View needs a package")
	}
	limits := DefaultLimits
	if v.Limits != nil {
		limits = *v.Limits
	}
	return Sandboxed(v.Package.App.ID, v.Package.Source(), limits)(cx)
}
