//go:build nsgodev && js && wasm

package nsgo

import "syscall/js"

// pageOrigin is where the page running the app was served from: nsgo
// dev's server, for its builds.
func pageOrigin() string { return js.Global().Get("location").Get("origin").String() }
