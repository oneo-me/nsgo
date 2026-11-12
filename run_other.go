//go:build !darwin && !windows && !android && !linux && !(js && wasm)

package nsgo

import "github.com/nsgo-dev/nsgo/core"

func run(App, func(*core.Scope), options) {
	panic("nsgo: no backend for this platform yet")
}
